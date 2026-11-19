package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The authoring-stamp ready condition (#6855) on the App-identity flip verb: `deskpost ready`
// performs the same mutation as deskflip, so it must clear the same condition. Each case is
// TestReadySuccess's green fixture plus a strong dispatch stamp; only the Worker-Attestation
// line and the authored-* pair vary.

const postAttestRun = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

// postAuthoringCond is the condition's contract name, spelled out so a rename is reviewed.
const postAuthoringCond = "authoring-stamp"

const postFixLine = "deskdispatch --stamp-only --repo " + exampleRepo +
	" --pr 1 --attestation 41 --model opus-4.8 --tier strong --kit worker"

// postRecord serves a validated record #41 selecting opus-4.8 / strong and counts reads.
func postRecord(t *testing.T) *int {
	t.Helper()
	reads := 0
	old := readWorkerRecord
	readWorkerRecord = func(fr deskkit.ForgeRepo, n int) (deskkit.WorkerReceipt, error) {
		reads++
		if fr.Slug() != exampleRepo || n != 41 {
			t.Errorf("record read for %s#%d, want %s#41", fr.Slug(), n, exampleRepo)
		}
		return deskkit.WorkerReceipt{Issue: 41, Binding: deskkit.WorkerBinding{Version: 1, Run: postAttestRun,
			Repo: exampleRepo, Kit: "worker", Model: "opus-4.8", Tier: "strong"}}, nil
	}
	t.Cleanup(func() { readWorkerRecord = old })
	return &reads
}

func postAttested(t *testing.T, line bool, authored ...string) *fakeGH {
	t.Helper()
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	if line {
		f.prBody = "Summary.\n\n" + deskkit.WorkerAttestationLinePrefix + " " + exampleRepo + "#41 run=" + postAttestRun + "\n"
	}
	d := deskDispatcherLogin(t)
	ev := strongStampBy(d)
	for _, l := range authored {
		ev = append(ev, deskkit.LabelEvent{Name: l, AppliedBy: d})
	}
	f.stamp(ev...)
	return f
}

func errText(t *testing.T) string {
	t.Helper()
	b, ok := stderr.(interface{ String() string })
	if !ok {
		t.Fatal("stderr is not the harness buffer")
	}
	return b.String()
}

// regression: #6855
func TestReadyAuthoringUnstamped(t *testing.T) {
	f := postAttested(t, true)
	reads := postRecord(t)
	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused || f.flips != 0 {
		t.Fatalf("unstamped attested PR: exit = %d flips = %d, want %d and no flip:\n%s",
			code, f.flips, deskkit.ExitRefused, errText(t))
	}
	out := errText(t)
	if *reads == 0 || !strings.Contains(out, postAuthoringCond) || !strings.Contains(out, "UNSTAMPED") ||
		!strings.Contains(out, postFixLine) {
		t.Errorf("the refusal must name %s, say UNSTAMPED and print %q:\n%s", postAuthoringCond, postFixLine, out)
	}
}

// regression: #6855
func TestReadyAuthoringMismatch(t *testing.T) {
	f := postAttested(t, true, deskkit.AuthoredModelPrefix+"haiku-3", deskkit.AuthoredTierPrefix+"strong")
	postRecord(t)
	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused || f.flips != 0 {
		t.Fatalf("mismatched authoring stamp: exit = %d flips = %d, want %d and no flip:\n%s",
			code, f.flips, deskkit.ExitRefused, errText(t))
	}
	if out := errText(t); !strings.Contains(out, "MISMATCHED") || !strings.Contains(out, postFixLine) {
		t.Errorf("the refusal must say MISMATCHED and print %q:\n%s", postFixLine, out)
	}
}

// regression: #6855
func TestReadyAuthoringStamped(t *testing.T) {
	f := postAttested(t, true, deskkit.AuthoredModelPrefix+"opus-4.8", deskkit.AuthoredTierPrefix+"strong")
	reads := postRecord(t)
	if code := run(readyArgs(exampleRepo)); code != 0 || f.flips != 1 {
		t.Fatalf("matching authoring stamp: exit = %d flips = %d, want a flip:\n%s", code, f.flips, errText(t))
	}
	if out := errText(t); *reads != 1 || !strings.Contains(out, postAuthoringCond+" OK") {
		t.Errorf("the record must be read exactly once (read %d) and the decision named:\n%s", *reads, out)
	}
}

// regression: #6855
func TestReadyAuthoringNoLine(t *testing.T) {
	f := postAttested(t, false)
	reads := postRecord(t)
	if code := run(readyArgs(exampleRepo)); code != 0 || f.flips != 1 {
		t.Fatalf("no attestation line: exit = %d flips = %d, want a flip:\n%s", code, f.flips, errText(t))
	}
	if out := errText(t); *reads != 0 || !strings.Contains(out, postAuthoringCond+" OK: no "+deskkit.WorkerAttestationLinePrefix) {
		t.Errorf("no line must read no record (read %d) and say the condition does not apply:\n%s", *reads, out)
	}
}

// The pre-flip re-read re-runs the condition: a Worker-Attestation line added between the reads
// moves no head, so a first-read-only check would flip an unstamped attested PR.
func TestReadyAuthoringReRead(t *testing.T) {
	f := postAttested(t, false)
	postRecord(t)
	f.onSecondPullRead = func() {
		f.prBody = "Summary.\n\n" + deskkit.WorkerAttestationLinePrefix + " " + exampleRepo + "#41 run=" + postAttestRun + "\n"
	}
	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused || f.flips != 0 {
		t.Fatalf("a line added before the re-read: exit = %d flips = %d, want %d and no flip:\n%s",
			code, f.flips, deskkit.ExitRefused, errText(t))
	}
	if out := errText(t); !strings.Contains(out, "UNSTAMPED") {
		t.Errorf("the re-read refusal must say UNSTAMPED:\n%s", out)
	}
}
