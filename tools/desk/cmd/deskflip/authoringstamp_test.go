package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The authoring-stamp condition (#6855), wired through the verb. Each case is the fully
// flippable greenPR with a strong dispatch stamp, so the ONLY variable is the change's
// Worker-Attestation line and its authored-* pair.

// authoringCond is the condition's contract name, spelled out so a rename is a reviewed edit.
const authoringCond = "authoring-stamp"

const attestRun = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// attestedRecord serves a validated worker record #41 selecting opus-4.8 / strong, and counts
// the reads so a case can show whether the condition consulted it.
func attestedRecord(t *testing.T) *int {
	t.Helper()
	reads := 0
	old := readWorkerRecordFn
	readWorkerRecordFn = func(_ deskkit.Forge, fr deskkit.ForgeRepo, n int) (deskkit.WorkerReceipt, error) {
		reads++
		if fr.Slug() != privateCIRepo || n != 41 {
			t.Errorf("record read for %s#%d, want %s#41", fr.Slug(), n, privateCIRepo)
		}
		return deskkit.WorkerReceipt{Issue: 41, Binding: deskkit.WorkerBinding{Version: 1, Run: attestRun,
			Repo: privateCIRepo, Kit: "worker", Model: "opus-4.8", Tier: "strong"}}, nil
	}
	t.Cleanup(func() { readWorkerRecordFn = old })
	return &reads
}

func attestedStub(t *testing.T, authored ...string) *stub {
	t.Helper()
	s := newStub()
	s.reviews = nil
	s.pr.Body = "Summary.\n\n" + deskkit.WorkerAttestationLinePrefix + " " + privateCIRepo + "#41 run=" + attestRun + "\n"
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.labelEvents = strongStamp(t)
	for _, l := range authored {
		s.labelEvents = append(s.labelEvents, deskkit.LabelEvent{Name: l, AppliedBy: dispatcherLogin(t)})
	}
	return s
}

const wantFixLine = "deskdispatch --stamp-only --repo " + privateCIRepo +
	" --pr 7 --attestation 41 --model opus-4.8 --tier strong --kit worker"

// regression: #6855
func TestAuthoringUnstampedRefuses(t *testing.T) {
	s := attestedStub(t)
	reads := attestedRecord(t)
	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitRefused || s.flipped() {
		t.Fatalf("an unstamped attested PR: rc = %d flipped = %t, want %d and no flip:\n%s",
			rc, s.flipped(), deskkit.ExitRefused, out)
	}
	if *reads == 0 || !strings.Contains(out, "condition "+authoringCond) ||
		!strings.Contains(out, "UNSTAMPED") || !strings.Contains(out, wantFixLine) {
		t.Errorf("the refusal must name %s, say UNSTAMPED and print %q:\n%s",
			authoringCond, wantFixLine, out)
	}
}

// regression: #6855
func TestAuthoringMismatchRefuses(t *testing.T) {
	s := attestedStub(t, deskkit.AuthoredModelPrefix+"haiku-3", deskkit.AuthoredTierPrefix+"strong")
	attestedRecord(t)
	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitRefused || s.flipped() {
		t.Fatalf("a mismatched authoring stamp: rc = %d flipped = %t, want %d and no flip:\n%s",
			rc, s.flipped(), deskkit.ExitRefused, out)
	}
	if !strings.Contains(out, "MISMATCHED") || !strings.Contains(out, wantFixLine) {
		t.Errorf("the refusal must say MISMATCHED and print %q:\n%s", wantFixLine, out)
	}
}

// regression: #6855
func TestAuthoringStampedFlips(t *testing.T) {
	s := attestedStub(t, deskkit.AuthoredModelPrefix+"opus-4.8", deskkit.AuthoredTierPrefix+"strong")
	reads := attestedRecord(t)
	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitOK || !s.flipped() {
		t.Fatalf("a matching authoring stamp: rc = %d flipped = %t, want a flip:\n%s", rc, s.flipped(), out)
	}
	if *reads != 1 || !strings.Contains(out, authoringCond+" OK") {
		t.Errorf("the record must be read once (got %d) and the decision named:\n%s", *reads, out)
	}
}

// regression: #6855
func TestAuthoringNoLineUnaffected(t *testing.T) {
	s := newStub()
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.labelEvents = strongStamp(t)
	reads := attestedRecord(t)
	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitOK || !s.flipped() {
		t.Fatalf("a PR with no attestation line: rc = %d flipped = %t, want a flip:\n%s", rc, s.flipped(), out)
	}
	if *reads != 0 || !strings.Contains(out, authoringCond+" OK: no "+deskkit.WorkerAttestationLinePrefix) {
		t.Errorf("no line must read no record (got %d) and say the condition does not apply:\n%s", *reads, out)
	}
}
