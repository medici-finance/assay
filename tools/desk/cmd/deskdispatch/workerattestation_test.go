package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var workerRun = strings.Repeat("c", 64)

func workerReceiptFixture(model string) deskkit.WorkerReceipt {
	return deskkit.WorkerReceipt{Issue: 41, URL: "https://example.invalid/attestation/41", Binding: deskkit.WorkerBinding{
		Version: 1, Run: workerRun, Repo: allowedRepo, Item: "example--01", Kit: "worker", Branch: "feat/x",
		Base: strings.Repeat("a", 40), Brief: "spec.md", BriefSHA256: strings.Repeat("b", 64), Model: model, Tier: "strong",
	}}
}

// stubWorkerAttest replaces step 5's worker attestation and records what it was asked to bind.
func stubWorkerAttest(t *testing.T, err error) *[]dispatchOpts {
	t.Helper()
	var calls []dispatchOpts
	old := attestWorkerDispatchFn
	attestWorkerDispatchFn = func(o dispatchOpts, plan dispatchPlan, repo, home string) (deskkit.WorkerReceipt, error) {
		calls = append(calls, o)
		if err != nil {
			return deskkit.WorkerReceipt{}, err
		}
		return workerReceiptFixture(o.model), nil
	}
	t.Cleanup(func() { attestWorkerDispatchFn = old })
	return &calls
}

func attestArgs(root, prompt string, extra ...string) []string {
	return append([]string{"example--01", "--root", root, "--repo", allowedRepo, "--kit", "worker", "--model", "gpt-6.1-sol",
		"--tier", "strong", "--brief", "spec.md", "--attest-worker", "--prompt-file", prompt}, extra...)
}

func TestAttestWorkerEmitsTheBoundPromptAfterTheRecord(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies(t.TempDir())
	calls := stubWorkerAttest(t, nil)
	prompt := filepath.Join(t.TempDir(), "prompt.md")
	if rc := run(attestArgs(root, prompt)); rc != 0 {
		t.Fatalf("attested dispatch rc=%d", rc)
	}
	if len(*calls) != 1 || (*calls)[0].model != "gpt-6.1-sol" {
		t.Fatalf("attestation not made exactly once with the dispatch selection: %+v", *calls)
	}
	p, err := os.ReadFile(emittedPathFor(prompt))
	if err != nil {
		t.Fatal(err)
	}
	r := workerReceiptFixture("gpt-6.1-sol")
	for _, want := range []string{"## Pre-work worker attestation", r.PRBinding(), "--attestation 41"} {
		if !strings.Contains(string(p), want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Contains(string(p), "## Pending model stamp") {
		t.Error("an attested run was still handed the pending-stamp section")
	}
}

func TestAttestWorkerFailureEmitsNoPromptAndReleasesTheClaim(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies(t.TempDir())
	stubWorkerAttest(t, deskkit.Unverifiable("fixture: stamp did not read back", nil))
	prompt := filepath.Join(t.TempDir(), "prompt.md")
	if rc := run(attestArgs(root, prompt)); rc != deskkit.ExitUnverifiable {
		t.Fatalf("failed attestation rc=%d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if _, err := os.Stat(emittedPathFor(prompt)); err == nil {
		t.Fatal("a failed worker attestation emitted a prompt")
	}
	if !s.ran("dispatch-claim.sh release") {
		t.Fatal("a failed worker attestation leaked the claim")
	}
}

func TestAttestWorkerPreconditionsRefuseBeforeTheClaim(t *testing.T) {
	for name, edit := range map[string]func([]string) []string{
		"review kit": func(a []string) []string { return replaceArg(a, "--kit", "review") },
		"with --pr":  func(a []string) []string { return append(a, "--pr", "77") },
		"no model":   func(a []string) []string { return dropArg(a, "--model") },
		"no source":  func(a []string) []string { return dropArg(a, "--brief") },
	} {
		t.Run(name, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			plantScripts(t, root)
			s.replies = happyReplies(t.TempDir())
			calls := stubWorkerAttest(t, nil)
			prompt := filepath.Join(t.TempDir(), "prompt.md")
			if rc := run(edit(attestArgs(root, prompt))); rc != deskkit.ExitRefused {
				t.Fatalf("rc=%d, want %d", rc, deskkit.ExitRefused)
			}
			if s.ran("dispatch-claim.sh acquire") || len(*calls) != 0 {
				t.Fatalf("a refused precondition reached durable state: %v", s.calls)
			}
		})
	}
}

func TestAttestWorkerDryRunWritesNothing(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	calls := stubWorkerAttest(t, nil)
	prompt := filepath.Join(t.TempDir(), "prompt.md")
	out := captureStdout(t, func() {
		if rc := run(attestArgs(root, prompt, "--dry-run")); rc != 0 {
			t.Errorf("dry-run rc=%d", rc)
		}
	})
	if len(s.calls) != 0 || len(*calls) != 0 {
		t.Fatalf("dry run wrote: calls=%v attest=%d", s.calls, len(*calls))
	}
	if !strings.Contains(out, "--attest-worker: step") {
		t.Fatalf("dry-run plan does not name the attestation:\n%s", out)
	}
}

func replaceArg(a []string, flag, v string) []string {
	out := append([]string(nil), a...)
	for i := range out {
		if out[i] == flag {
			out[i+1] = v
		}
	}
	return out
}

func dropArg(a []string, flag string) []string {
	var out []string
	for i := 0; i < len(a); i++ {
		if a[i] == flag {
			i++
			continue
		}
		out = append(out, a[i])
	}
	return out
}

// stubWorkerRead replaces the stamp-only record read and counts its calls.
func stubWorkerRead(t *testing.T, r deskkit.WorkerReceipt, err error) *int {
	t.Helper()
	n := 0
	old := readWorkerAttestationFn
	readWorkerAttestationFn = func(deskkit.Forge, deskkit.ForgeRepo, int) (deskkit.WorkerReceipt, error) {
		n++
		return r, err
	}
	t.Cleanup(func() { readWorkerAttestationFn = old })
	return &n
}

func TestStampOnlyBindsToTheWorkerAttestation(t *testing.T) {
	line := "Worker-Attestation: " + allowedRepo + "#41 run=" + workerRun
	for name, tc := range map[string]struct {
		body    string
		extra   []string
		record  string // the attested model
		readErr error
		rc      int
		reads   int
	}{
		"matching record":            {body: "Summary\n\n" + line, record: "gpt-6.1-sol", rc: 0, reads: 1},
		"by flag only":               {extra: []string{"--attestation", "41"}, record: "gpt-6.1-sol", rc: 0, reads: 1},
		"model disagrees":            {body: line, record: "gpt-6-astra", rc: deskkit.ExitRefused, reads: 1},
		"flag disagrees with change": {body: line, extra: []string{"--attestation", "42"}, record: "gpt-6.1-sol", rc: deskkit.ExitRefused},
		"record unreadable":          {body: line, readErr: deskkit.Unverifiable("fixture: 500", nil), rc: deskkit.ExitUnverifiable, reads: 1},
		"duplicate line":             {body: line + "\n" + line, record: "gpt-6.1-sol", rc: deskkit.ExitRefused},
		"no attestation named":       {body: "Summary", record: "gpt-6.1-sol", rc: 0},
	} {
		t.Run(name, func(t *testing.T) {
			s := &stub{}
			s.install(t)
			t.Setenv("DESK_LOOP", "the-desk")
			gh := installGHStamp(t)
			gh.prBody = tc.body
			reads := stubWorkerRead(t, workerReceiptFixture(tc.record), tc.readErr)
			if rc := run(append(onlyArgs("gpt-6.1-sol"), tc.extra...)); rc != tc.rc {
				t.Fatalf("rc=%d, want %d", rc, tc.rc)
			}
			if *reads != tc.reads {
				t.Fatalf("record reads=%d, want %d", *reads, tc.reads)
			}
			if tc.rc != 0 && gh.writes != 0 {
				t.Fatalf("a refused binding wrote %d stamp labels", gh.writes)
			}
		})
	}
	t.Run("branch disagrees", func(t *testing.T) {
		s := &stub{}
		s.install(t)
		gh := installGHStamp(t)
		gh.prBody, gh.headRef = line, "feat/elsewhere"
		stubWorkerRead(t, workerReceiptFixture("gpt-6.1-sol"), nil)
		if rc := run(onlyArgs("gpt-6.1-sol")); rc != deskkit.ExitRefused || gh.writes != 0 {
			t.Fatalf("change on another branch stamped: rc=%d writes=%d", rc, gh.writes)
		}
	})
	t.Run("review kit refuses --attestation", func(t *testing.T) {
		s := &stub{}
		s.install(t)
		t.Setenv("DESK_LOOP", "pr-review-desk")
		gh := installGHStamp(t)
		if rc := run(append(onlyArgs("gpt-6.1-sol"), "--kit", "review", "--attestation", "41")); rc != deskkit.ExitRefused || gh.writes != 0 {
			t.Fatalf("review stamp bound to a worker attestation: rc=%d writes=%d", rc, gh.writes)
		}
	})
}
