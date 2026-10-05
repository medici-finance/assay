package main

import (
	"encoding/json"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func onlyArgs(model string) []string {
	return []string{"--stamp-only", "--repo", allowedRepo, "--pr", "77", "--model", model, "--tier", "strong"}
}

func TestStampOnlyRoundTrip(t *testing.T) {
	for _, model := range []string{"gpt-6-astra", "gpt-6.1-sol", "gpt-6-1-sol"} {
		t.Run(model, func(t *testing.T) {
			s := &stub{}
			s.install(t)
			t.Setenv("DESK_LOOP", "the-desk")
			gh := installGHStamp(t)
			if rc := run(onlyArgs(model)); rc != 0 {
				t.Fatalf("stamp-only rc=%d, want 0", rc)
			}
			got, state := deskkit.AttestedModelStampOf(deskkit.StampTimeline{Present: gh.labels, Events: ghEvents(gh)}, deskkit.IsStampAuthorityLogin)
			if state != deskkit.ModelStamped || got.Model != model || got.Tier != "strong" {
				t.Fatalf("readback=%+v %v", got, state)
			}
			n := len(gh.requests)
			if rc := run(onlyArgs(model)); rc != 0 {
				t.Fatalf("idempotent rc=%d", rc)
			}
			for _, r := range gh.requests[n:] {
				if r.Method != "GET" {
					t.Fatalf("idempotent write: %+v", r)
				}
			}
			if len(s.calls) != 0 {
				t.Fatalf("stamp-only launched child processes: %v", s.calls)
			}
		})
	}
}

func ghEvents(s *ghStampServer) []deskkit.LabelEvent {
	var out []deskkit.LabelEvent
	for _, e := range s.timeline {
		out = append(out, deskkit.LabelEvent{Name: e.Label, AppliedBy: e.Actor, Removed: e.Event == "unlabeled"})
	}
	return out
}

func TestStampOnlyRejectsArgs(t *testing.T) {
	for _, args := range [][]string{
		{"--stamp-only"},
		{"--stamp-only", "--repo", allowedRepo, "--pr", "77", "--tier", "strong"},
		{"--stamp-only", "--repo", allowedRepo, "--pr", "77", "--model", "gpt-6.1-sol"},
		append(onlyArgs("gpt-6.1-sol"), "--branch", "extra"),
		append(onlyArgs("gpt-6.1-sol"), "--pr", "0"),
		onlyArgs("Not A Model"),
		append(onlyArgs("gpt-6.1-sol"), "--tier", "unknown"),
		append(onlyArgs("gpt-6.1-sol"), "extra"),
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			s := &stub{}
			s.install(t)
			t.Setenv("DESK_LOOP", "the-desk")
			gh := installGHStamp(t)
			if rc := run(args); rc != deskkit.ExitRefused {
				t.Fatalf("rc=%d, want 5", rc)
			}
			if len(s.calls)+len(gh.requests)+len(gh.minted) != 0 {
				t.Fatal("invalid arguments reached external work")
			}
		})
	}
}

func TestStampOnlyCaller(t *testing.T) {
	for _, loop := range []string{"worker-desk", "verify-desk", "intake-desk", "", "pr-review-desk"} {
		t.Run(loop, func(t *testing.T) {
			s := &stub{}
			s.install(t)
			t.Setenv("DESK_LOOP", loop)
			gh := installGHStamp(t)
			if rc := run(onlyArgs("gpt-6-astra")); rc != deskkit.ExitRefused {
				t.Fatalf("rc=%d want 5", rc)
			}
			if len(s.calls)+len(gh.requests)+len(gh.minted) != 0 {
				t.Fatal("wrong caller reached external work")
			}
		})
	}
}

func TestStampOnlyFailure(t *testing.T) {
	for _, mode := range []string{"read", "timeline", "write", "drop", "after", "actor", "closed"} {
		t.Run(mode, func(t *testing.T) {
			s := &stub{}
			s.install(t)
			t.Setenv("DESK_LOOP", "the-desk")
			gh := installGHStamp(t)
			switch mode {
			case "read":
				gh.failPR = true
			case "timeline":
				gh.failTL = true
			case "write":
				gh.failWrite = true
			case "drop":
				gh.dropWrites = true
			case "after":
				gh.failAfter = true
			case "actor":
				gh.wrongActor = true
			case "closed":
				gh.state = "closed"
			}
			want := deskkit.ExitUnverifiable
			if mode == "closed" {
				want = deskkit.ExitRefused
			}
			out := captureStdout(t, func() {
				if rc := run(onlyArgs("gpt-6.1-sol")); rc != want {
					t.Fatalf("rc=%d want %d", rc, want)
				}
			})
			if strings.Contains(out, "OK:") {
				t.Fatalf("false success: %s", out)
			}
			if len(s.calls) != 0 {
				t.Fatalf("child calls: %v", s.calls)
			}
		})
	}
}

func TestStampOnlyRepair(t *testing.T) {
	for _, labels := range [][]string{
		{"dispatched-tier:strong"},
		{"dispatched-model:old", "dispatched-tier:any", "dispatched-tier:strong"},
		{"dispatched-model:gpt-6.1-sol", "dispatched-tier:strong"},
		{"dispatched-tier:unknown"},
	} {
		t.Run(strings.Join(labels, ","), func(t *testing.T) {
			s := &stub{}
			s.install(t)
			t.Setenv("DESK_LOOP", "the-desk")
			gh := installGHStamp(t)
			gh.labels = append([]string{"unrelated"}, labels...)
			// Absent standing events are repaired through the existing removal semantics.
			out := captureStdout(t, func() {
				if rc := run(onlyArgs("gpt-6.1-sol")); rc != 0 {
					t.Fatalf("rc=%d", rc)
				}
			})
			if !strings.Contains(out, "applied and verified") || !strings.Contains(out, "RE-STAMPED") {
				t.Fatalf("receipt: %s", out)
			}
			if !labelsPresent(gh.labels, []string{"unrelated"}) {
				t.Fatal("unrelated label removed")
			}
			stamp, state := deskkit.AttestedModelStampOf(deskkit.StampTimeline{Present: gh.labels, Events: ghEvents(gh)}, deskkit.IsStampAuthorityLogin)
			if state != deskkit.ModelStamped || stamp.Model != "gpt-6.1-sol" || stamp.Tier != "strong" {
				t.Fatalf("stamp=%+v state=%v", stamp, state)
			}
		})
	}
}

func TestStampOnlyDryRun(t *testing.T) {
	s := &stub{}
	home, _ := s.install(t)
	t.Setenv("DESK_LOOP", "the-desk")
	gh := installGHStamp(t)
	gh.labels = []string{"dispatched-tier:any"}
	out := captureStdout(t, func() {
		if rc := run(append(onlyArgs("gpt-6-astra"), "--dry-run")); rc != 0 {
			t.Fatalf("rc=%d", rc)
		}
	})
	if _, err := os.Stat(filepath.Join(home, ".config", "assay", "audit.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("dry run appended audit: %v", err)
	}
	if !strings.Contains(out, "PLAN:") || strings.Contains(out, "OK:") {
		t.Fatalf("receipt: %s", out)
	}
	for _, r := range gh.requests {
		if r.Method != "GET" {
			t.Fatalf("dry run mutation: %+v", r)
		}
	}
	if len(s.calls) != 0 {
		t.Fatalf("child calls: %v", s.calls)
	}
}

func TestStampOnlyReview(t *testing.T) {
	s := &stub{}
	s.install(t)
	t.Setenv("DESK_LOOP", "pr-review-desk")
	gh := installGHStamp(t)
	if rc := run(append(onlyArgs("gpt-6-astra"), "--kit", "review")); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	for _, m := range gh.minted {
		if m.role != deskkit.ReviewDispatcherRole {
			t.Fatalf("wrong identity: %+v", m)
		}
	}
	if len(s.calls) != 0 {
		t.Fatalf("child calls: %v", s.calls)
	}
}

func TestStampOnlyGitLab(t *testing.T) {
	s := &stub{}
	home, _ := s.install(t)
	t.Setenv("DESK_LOOP", "the-desk")
	gl := installGLStamp(t, home)
	args := []string{"--stamp-only", "--repo", glProject, "--pr", "7", "--model", "gpt-6.1-sol", "--tier", "strong"}
	out := captureStdout(t, func() {
		if rc := run(args); rc != 0 {
			t.Fatalf("rc=%d", rc)
		}
	})
	if !strings.Contains(out, "applied and verified") || !strings.Contains(out, "gitlab") {
		t.Fatalf("receipt: %s", out)
	}
	n := len(gl.requests)
	if rc := run(args); rc != 0 {
		t.Fatalf("no-op rc=%d", rc)
	}
	for _, r := range gl.requests[n:] {
		if r.Method != "GET" {
			t.Fatalf("idempotent mutation: %+v", r)
		}
	}
	if len(s.calls) != 0 {
		t.Fatalf("child calls: %v", s.calls)
	}
}

func TestStampReadbackBothPaths(t *testing.T) {
	for _, entry := range []string{"dispatch", "stamp-only"} {
		t.Run(entry, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			plantScripts(t, root)
			s.replies = happyReplies("/private/tmp/worker-home")
			t.Setenv("DESK_LOOP", "the-desk")
			gh := installGHStamp(t)
			gh.dropWrites = true
			args := onlyArgs("gpt-6.1-sol")
			if entry == "dispatch" {
				args = stampArgs(t, root)
			}
			if rc := run(args); rc != deskkit.ExitUnverifiable {
				t.Fatalf("unobserved stamp reported rc=%d", rc)
			}
		})
	}
}

func TestStampOnlyAudit(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(itoa(map[bool]int{false: 0, true: 1}[fail]), func(t *testing.T) {
			s := &stub{}
			home, _ := s.install(t)
			t.Setenv("DESK_LOOP", "the-desk")
			gh := installGHStamp(t)
			gh.dropWrites = fail
			run(onlyArgs("gpt-6.1-sol"))
			raw, err := os.ReadFile(filepath.Join(home, ".config", "assay", "audit.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			var row deskkit.Entry
			if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &row); err != nil {
				t.Fatal(err)
			}
			want := deskkit.ResultOK
			if fail {
				want = deskkit.ResultUnverifiable
			}
			if row.Verb != "stamp-only" || row.Repo != allowedRepo || row.PR == nil || *row.PR != 77 || row.Result != want {
				t.Fatalf("audit=%+v", row)
			}
			if !fail && !strings.Contains(row.Detail, "applied and verified") {
				t.Fatalf("receipt omitted: %+v", row)
			}
		})
	}
}

func TestPendingStampCommand(t *testing.T) {
	o := dispatchOpts{item: "item", model: "gpt-6.1-sol", tier: "strong", kit: "worker-objective", root: "."}
	line, err := stepStamp(o, allowedRepo)
	if err != nil {
		t.Fatal(err)
	}
	want := "deskdispatch --stamp-only --repo " + allowedRepo + " --pr <N> --model gpt-6.1-sol --tier strong --kit worker-objective"
	if !strings.Contains(line, want) {
		t.Fatalf("pending receipt: %s", line)
	}
	prompt, err := assemblePrompt(o, dispatchPlan{repo: allowedRepo}, "/example/worktree")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, want) || !strings.Contains(prompt, "Do not run this command as the worker") {
		t.Fatal("pending command/custody missing from worker assignment")
	}
}
