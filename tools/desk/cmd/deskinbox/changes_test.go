package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: class issue-projection
func TestChangesDeliveryAndWithdrawal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	const repo = "example/repo"
	f := &fakeForge{issues: []deskkit.IssueSummary{
		{Number: 7, Labels: []string{"VeRiFy-GaTe"}},
		{Number: 8, Labels: []string{"other"}},
	}}
	withForge(t, map[string]*fakeForge{repo: f})
	current := inboxAction{State: "prepare", Owner: "desk", URL: "https://example.org/issue/7"}
	save := readChangeAction
	t.Cleanup(func() { readChangeAction = save })
	readChangeAction = func(_ string, n int, _ ...deskkit.RequestObserver) (inboxAction, error) {
		if n != 7 {
			t.Fatalf("queried unrelated issue %d", n)
		}
		return current, nil
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	call := func(at time.Time, args ...string) (changeBatch, int) {
		t.Helper()
		var out, diag bytes.Buffer
		code := run(append([]string{"changes", repo}, args...), &out, &diag, at)
		var b changeBatch
		if out.Len() > 0 {
			if err := json.Unmarshal(out.Bytes(), &b); err != nil {
				t.Fatalf("%s: %v", out.String(), err)
			}
		}
		return b, code
	}
	poll := func(want string, at time.Time, args ...string) changeBatch {
		t.Helper()
		b, code := call(at, args...)
		if code != 0 || (want == "" && (len(b.Events) != 0 || b.Receipt != "")) ||
			(want != "" && (len(b.Events) != 1 || b.Events[0].ID != repo+"#7" || b.Events[0].Kind != want || b.Receipt == "")) {
			t.Fatalf("want %q, got %+v (exit %d)", want, b, code)
		}
		return b
	}
	ack := func(receipt string, want int) {
		t.Helper()
		if _, code := call(now, "--ack", receipt); code != want {
			t.Fatalf("ack exit=%d want %d", code, want)
		}
	}
	if _, code := call(now, "--ack="); code != deskkit.ExitRefused {
		t.Fatalf("empty receipt exit=%d", code)
	}
	poll("", now)
	current.State, current.Owner, current.Act, current.Evidence = "ready", "human", "approve the revision", "proof-A"
	first := poll("ready", now)
	if retry := poll("ready", now); first.Receipt != retry.Receipt {
		t.Fatal("retry altered receipt")
	}
	// Another consumer and a different repo scope do not share our cursor.
	for _, args := range [][]string{{"--consumer", "another"}, {"example/other"}} {
		if poll("ready", now, args...).Receipt == first.Receipt {
			t.Fatal("receipt reused across scopes")
		}
	}
	current.State = "unknown"
	if _, code := call(now); code != deskkit.ExitUnverifiable {
		t.Fatalf("incomplete detail exit=%d", code)
	}
	current.State = "ready"
	if poll("ready", now).Receipt != first.Receipt {
		t.Fatal("incomplete observation mutated receipt")
	}
	ack(first.Receipt, 0)
	current.Title = "unrelated title edit"
	poll("", now.Add(5*time.Minute))
	poll("", now.Add(5*time.Minute), "--remind-after", "1h")
	reminder := poll("reminder", now.Add(time.Hour), "--remind-after", "1h")
	now = now.Add(time.Hour)
	ack(reminder.Receipt, 0)
	poll("", now, "--remind-after", "1h")

	current.Evidence = "proof-B"
	newProof := poll("changed", now)
	if retry := poll("changed", now); retry.Receipt != newProof.Receipt {
		t.Fatal("retry of the changed offer replaced its receipt")
	}
	// A delivery might have shown proof-B even though no ack arrived.
	// Re-offer proof-A so the consumer can replace that superseded action.
	current.Evidence = "proof-A"
	reverted := poll("changed", now)
	if reverted.Receipt == newProof.Receipt || reverted.Events[0].Action.Evidence != "proof-A" {
		t.Fatalf("reverted offer did not supersede the pending change: %+v", reverted)
	}
	ack(newProof.Receipt, deskkit.ExitRefused)
	current.State = "stale"
	withdrawn := poll("withdrawn", now)
	if withdrawn.Events[0].Action != nil {
		t.Fatal("withdrawal advertised stale act")
	}
	ack(reverted.Receipt, deskkit.ExitRefused)
	ack(withdrawn.Receipt, 0)
	ack(withdrawn.Receipt, 0)
	poll("", now)
	current.State = "ready"
	second := poll("ready", now)
	f.err = errors.New("listing unavailable")
	if _, code := call(now); code != deskkit.ExitUnverifiable {
		t.Fatalf("partial discovery exit=%d", code)
	}
	f.err = nil
	if poll("ready", now).Receipt != second.Receipt {
		t.Fatal("partial discovery invalidated receipt")
	}
	// An unacknowledged first request may have reached the human. Withdraw it as well.
	f.issues = nil
	poll("withdrawn", now)
}

type failedChangeWriter struct{}

func (failedChangeWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

// regression: class issue-projection
func TestChangesBrokenPipeReplaysAndContendedCursorIsUntouched(t *testing.T) {
	dir := t.TempDir()
	live := map[string]changeAction{"example/repo#1": {Owner: "human", Act: "Review"}}
	now := time.Now()
	var receipt string
	err := withChangesState(dir, func(path string, s *changeState) error {
		if err := withChangesState(dir, func(string, *changeState) error { t.Fatal("acquired a contended cursor"); return nil }); !errors.Is(err, deskkit.ErrLockBusy) {
			t.Fatalf("second reader %v", err)
		}
		err := deliverChanges(failedChangeWriter{}, path, s, live, now, 0)
		receipt = s.Receipt
		return err
	})
	if !errors.Is(err, io.ErrClosedPipe) || receipt == "" {
		t.Fatalf("stage receipt %q: %v", receipt, err)
	}
	err = withChangesState(dir, func(path string, s *changeState) error {
		var b bytes.Buffer
		if err := deliverChanges(&b, path, s, live, now, 0); err != nil {
			return err
		}
		var batch changeBatch
		if err := json.Unmarshal(b.Bytes(), &batch); err != nil {
			return err
		}
		if batch.Receipt != receipt || len(batch.Events) != 1 {
			t.Fatalf("retry: %+v", batch)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "cursor.json")
	if err := os.WriteFile(path, []byte("null"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := withChangesState(dir, func(string, *changeState) error { t.Fatal("used corrupt cursor"); return nil }); err == nil {
		t.Fatal("accepted corrupt cursor")
	}
}

// regression: class issue-projection
func TestChangesKeepsUrgentActsFirstAfterWithdrawals(t *testing.T) {
	state := changeState{Version: 1, Delivered: map[string]deliveredAction{"withdrawn": {Digest: "old"}}}
	live := map[string]changeAction{
		"question":     {rank: 2, createdAt: "2021", Act: "question"},
		"urgent newer": {rank: 0, createdAt: "2024", Act: "urgent"},
		"urgent older": {rank: 0, createdAt: "2023", Act: "urgent"},
	}
	var out bytes.Buffer
	if err := deliverChanges(&out, filepath.Join(t.TempDir(), "cursor.json"), &state, live, time.Now(), 0); err != nil {
		t.Fatal(err)
	}
	var result changeBatch
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 4 {
		t.Fatalf("events=%+v", result.Events)
	}
	for i, want := range []string{"withdrawn", "urgent older", "urgent newer", "question"} {
		if got := result.Events[i].ID; got != want {
			t.Fatalf("event %d: got %q want %q", i, got, want)
		}
	}
}

// regression: class verification-provenance
func TestChangesWithLandedRunsheet(t *testing.T) {
	f := readyDecisionFixture(t)
	const repo = "example-org/tracker"
	f.issue.Labels = []string{"help wanted"}
	target := "https://github.com/" + repo + "/blob/" + strings.Repeat("b", 40) + "/docs/run.md"
	f.issue.Body = "[Runsheet](" + target + ")"
	f.Forge = &fakeForge{issues: []deskkit.IssueSummary{{Number: 7, Labels: f.issue.Labels}}}
	f.pinnedFile = &deskkit.FileContent{Exists: true, Kind: deskkit.FileKindRegular, Content: []byte(repo + "#7\n! perform-reviewed-task\nWhy: only the account holder can approve.\nWhat the session did: prepared the approval.\nResume: read the receipt.")}
	f.branchFile = &deskkit.FileContent{Exists: false}
	poll := func(want string) changeBatch {
		t.Helper()
		var out, diag bytes.Buffer
		if code := run([]string{"changes", repo}, &out, &diag, time.Now()); code != 0 {
			t.Fatalf("changes exit %d: %s", code, diag.String())
		}
		var batch changeBatch
		if err := json.Unmarshal(out.Bytes(), &batch); err != nil {
			t.Fatal(err)
		}
		if want == "" && len(batch.Events) != 0 || want != "" && (len(batch.Events) != 1 || batch.Events[0].Kind != want) {
			t.Fatalf("wanted %q, got %+v", want, batch)
		}
		return batch
	}
	poll("")
	f.branchFile = f.pinnedFile
	ready := poll("ready")
	line := target + "#L2"
	if a := ready.Events[0].Action; a == nil || a.Evidence != line || !strings.Contains(a.Act, line) {
		t.Fatalf("missing verified evidence and act: %+v", a)
	}
	event, err := json.Marshal(ready)
	if err != nil || bytes.Contains(event, []byte("perform-reviewed-task")) {
		t.Fatalf("delivery copied the command out of the verified runsheet: %s (%v)", event, err)
	}
	if code := run([]string{"changes", repo, "--ack", ready.Receipt}, io.Discard, io.Discard, time.Now()); code != 0 {
		t.Fatalf("ack exit %d", code)
	}
	poll("")
	f.branchFile = &deskkit.FileContent{Exists: true, Kind: deskkit.FileKindRegular, Content: []byte("superseded instructions")}
	poll("withdrawn")
}
