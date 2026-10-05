package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestRosterMutationsPreserveForeignFields(t *testing.T) {
	for _, mode := range []string{"set", "drop", "prune"} {
		t.Run(mode, func(t *testing.T) {
			home := rosterSetup(t)
			t.Setenv("DESK_SESSION", "shared")
			writeTestBeacon(t, home, Beacon{Session: "shared", OpenWork: []WorkEntry{{Repo: "tracker", PR: 7, What: "old"}}})
			path, _ := beaconPath("shared")
			data, _ := os.ReadFile(path)
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			fields["future"] = json.RawMessage(`{"owner":"another writer"}`)
			fields["resource"] = json.RawMessage(`{"tokens":{"value":17}}`)
			data, _ = json.Marshal(fields)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch mode {
			case "set":
				err = cmdSet([]string{"--role", "worker-desk"})
			case "drop":
				err = cmdDrop([]string{"--repo", "tracker", "--pr", "7"})
			case "prune":
				t.Setenv("FAKEGH_PR_MERGED_7", "1")
				err = cmdList()
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(path)
			if err != nil {
				t.Fatalf("foreign-owned beacon was removed: %v", err)
			}
			var got map[string]json.RawMessage
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"future", "resource"} {
				var before, after interface{}
				_ = json.Unmarshal(fields[key], &before)
				if err := json.Unmarshal(got[key], &after); err != nil {
					t.Fatalf("lost %s: %s", key, data)
				}
				b, _ := json.Marshal(before)
				a, _ := json.Marshal(after)
				if string(a) != string(b) {
					t.Fatalf("changed %s: %s", key, data)
				}
			}
		})
	}
}

type racingPruneForge struct {
	fakeRosterForge
	duringRead func()
}

func (f *racingPruneForge) GetPullRequest(_ deskkit.ForgeRepo, n int) (*deskkit.PullRequest, error) {
	if n == 7 {
		f.duringRead()
		return &deskkit.PullRequest{Number: n, State: "closed", Merged: true}, nil
	}
	return &deskkit.PullRequest{Number: n, State: "open"}, nil
}

func TestListPrunePreservesConcurrentChanges(t *testing.T) {
	for _, mode := range []string{"added-work", "changed-work", "ack", "resource"} {
		t.Run(mode, func(t *testing.T) {
			home := rosterSetup(t)
			t.Setenv("DESK_SESSION", "shared")
			writeTestBeacon(t, home, Beacon{Session: "shared", OpenWork: []WorkEntry{{Repo: "tracker", PR: 7, What: "old"}}})
			orig := forgeFor
			t.Cleanup(func() { forgeFor = orig })
			f := &racingPruneForge{duringRead: func() {
				var err error
				switch mode {
				case "added-work":
					err = cmdSet([]string{"--repo", "tracker", "--pr", "8", "--what", "new"})
				case "changed-work":
					err = cmdSet([]string{"--repo", "tracker", "--pr", "7", "--what", "changed"})
				case "ack":
					_, err = deskkit.AppendAck("shared", deskkit.AckRecord{Role: "worker-desk", Restatement: "keep"})
				case "resource":
					_, err = deskkit.MergeResourceVitals("shared", deskkit.ResourceVitals{Tokens: deskkit.MeasuredInt(42)})
				}
				if err != nil {
					t.Fatal(err)
				}
			}}
			forgeFor = func(string) (deskkit.Forge, deskkit.ForgeRepo, error) { return f, deskkit.ForgeRepo{}, nil }
			if err := cmdList(); err != nil {
				t.Fatal(err)
			}
			b := readTestBeacon(t, home, "shared")
			if b == nil {
				t.Fatal("prune deleted data written during forge read")
			}
			switch mode {
			case "added-work":
				if len(b.OpenWork) != 1 || b.OpenWork[0].PR != 8 {
					t.Fatalf("lost new work: %+v", b)
				}
			case "changed-work":
				if len(b.OpenWork) != 1 || b.OpenWork[0].What != "changed" {
					t.Fatalf("pruned changed work: %+v", b)
				}
			case "ack":
				if !b.hasAcks() || len(b.OpenWork) != 0 {
					t.Fatalf("lost ack or failed prune: %+v", b)
				}
			case "resource":
				if len(b.Resource) == 0 || len(b.OpenWork) != 0 {
					t.Fatalf("lost resource or failed prune: %+v", b)
				}
			}
		})
	}
}

func TestRosterMalformedBeaconPreserved(t *testing.T) {
	for _, input := range []string{"", "null", "[]", `{"session":`, `{"open_work":"invalid"}`} {
		for _, mode := range []string{"set", "drop"} {
			t.Run(mode+"/"+input, func(t *testing.T) {
				home := rosterSetup(t)
				t.Setenv("DESK_SESSION", "shared")
				writeTestBeacon(t, home, Beacon{Session: "shared"})
				path, _ := beaconPath("shared")
				if err := os.WriteFile(path, []byte(input), 0600); err != nil {
					t.Fatal(err)
				}
				var err error
				if mode == "set" {
					err = cmdSet([]string{"--role", "worker-desk"})
				} else {
					err = cmdDrop([]string{"--repo", "tracker", "--pr", "7"})
				}
				if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
					t.Fatalf("want unverifiable, got %v", err)
				}
				got, err := os.ReadFile(path)
				if err != nil || string(got) != input {
					t.Fatalf("malformed input changed: %q, %v", got, err)
				}
			})
		}
	}
}

func TestListPruneReportsConcurrentCorruption(t *testing.T) {
	home := rosterSetup(t)
	writeTestBeacon(t, home, Beacon{Session: "shared", OpenWork: []WorkEntry{{Repo: "tracker", PR: 7, What: "old"}}})
	path, _ := beaconPath("shared")
	orig := forgeFor
	t.Cleanup(func() { forgeFor = orig })
	f := &racingPruneForge{duringRead: func() {
		if err := os.WriteFile(path, []byte(`{"partial":`), 0600); err != nil {
			t.Fatal(err)
		}
	}}
	forgeFor = func(string) (deskkit.Forge, deskkit.ForgeRepo, error) { return f, deskkit.ForgeRepo{}, nil }
	var listErr error
	out := captureStdout(t, func() { listErr = cmdList() })
	if deskkit.ExitCodeOf(listErr) != deskkit.ExitUnverifiable {
		t.Fatalf("want unverifiable prune, got %v", listErr)
	}
	if !strings.Contains(out, "shared") || !strings.Contains(out, "MERGED") || !strings.Contains(out, "old") {
		t.Fatalf("valid snapshot missing after failed prune: %q", out)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != `{"partial":` {
		t.Fatalf("corrupt input replaced: %q, %v", got, err)
	}
}

func TestListRetainsValidRowsWhenBeaconCorrupt(t *testing.T) {
	for _, mode := range []string{"duplicate", "directory"} {
		t.Run(mode, func(t *testing.T) {
			home := rosterSetup(t)
			writeTestBeacon(t, home, Beacon{Session: "valid", Role: "worker-desk"})
			path, _ := beaconPath("broken")
			const bad = `{"role":"worker-desk","role":"verify-desk"}`
			if mode == "directory" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("FAKEGH_LIST_PRS", "99:true:possibly owned")
			var listErr error
			out := captureStdout(t, func() { listErr = cmdList() })
			if deskkit.ExitCodeOf(listErr) != deskkit.ExitUnverifiable || !strings.Contains(listErr.Error(), "broken.json") {
				t.Fatalf("corruption hidden: %v", listErr)
			}
			if !strings.Contains(out, "valid") || !strings.Contains(out, "standing sessions") || strings.Contains(out, "(no registered work)") {
				t.Fatalf("misleading/incomplete output: %q", out)
			}
			if strings.Contains(out, "unclaimed (no session registered)") || !strings.Contains(out, "ownership unverified") || strings.Contains(out, "(none)") || !strings.Contains(out, "(unknown)") {
				t.Fatalf("unverified ownership reported as absent: %q", out)
			}
			if mode == "directory" {
				info, err := os.Stat(path)
				if err != nil || !info.IsDir() {
					t.Fatalf("directory changed: %v %v", info, err)
				}
			} else {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != bad {
					t.Fatalf("corrupt data changed: %q %v", data, err)
				}
			}
		})
	}
}
