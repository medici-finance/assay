package deskkit

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// backstopRig drives ReadyBackstop with a scripted forge and a fake clock that advances only
// when the backstop sleeps, so the 30s bound runs instantly (#6242).
type backstopRig struct {
	states   []string
	readErr  error
	convErr  error
	reads    int
	converts int
	comments []string
	now      time.Time
}

func (r *backstopRig) run() BackstopOutcome {
	r.now = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return ReadyBackstop{
		Repo: "o/r", Number: 7, HeadSHA: "abc123",
		ReadState: func() (string, error) {
			i := min(r.reads, len(r.states)-1)
			r.reads++
			return r.states[i], r.readErr
		},
		ConvertToDraft: func() error { r.converts++; return r.convErr },
		Comment:        func(b string) error { r.comments = append(r.comments, b); return nil },
		Deadline:       ReadyBackstopDeadline, Interval: ReadyBackstopInterval,
		Now:   func() time.Time { return r.now },
		Sleep: func(d time.Duration) { r.now = r.now.Add(d) },
	}.Run()
}

// regression: #6242
func TestBackstopRevertStates(t *testing.T) {
	for _, st := range []string{"blocked", "DIRTY"} {
		r := &backstopRig{states: []string{st}}
		out := r.run()
		up := strings.ToUpper(st)
		if out.Verdict != BackstopReverted || !IsRefused(out.Err) || r.converts != 1 {
			t.Fatalf("%s: verdict=%s err=%v converts=%d, want reverted/Refused/1", st, out.Verdict, out.Err, r.converts)
		}
		if len(r.comments) != 1 || !strings.Contains(r.comments[0], up) || !strings.Contains(r.comments[0], "abc123") {
			t.Errorf("%s: comments = %q, want ONE naming the state and head", st, r.comments)
		}
		if !strings.Contains(out.Err.Error(), "reverted because the forge disagrees") {
			t.Errorf("%s: err = %v", st, out.Err)
		}
	}
}

// regression: #6242
func TestBackstopPassStates(t *testing.T) {
	for _, st := range ReadyBackstopPasses {
		r := &backstopRig{states: []string{strings.ToLower(st)}}
		out := r.run()
		if out.Verdict != BackstopPass || out.Err != nil || r.converts != 0 || len(r.comments) != 0 || r.reads != 1 {
			t.Errorf("%s: %+v converts=%d comments=%d reads=%d, want a silent pass", st, out, r.converts, len(r.comments), r.reads)
		}
	}
}

// regression: #6242
func TestBackstopUnknownDeadline(t *testing.T) {
	r := &backstopRig{states: []string{"UNKNOWN"}}
	out := r.run()
	if out.Verdict != BackstopUnchecked || out.Err != nil || r.converts != 0 {
		t.Fatalf("%+v converts=%d, want could-not-check, no error, no revert", out, r.converts)
	}
	if len(r.comments) != 1 || !strings.Contains(out.Notice, "could-not-verify") {
		t.Errorf("comments=%q notice=%q", r.comments, out.Notice)
	}
	// 30s at 3s: reads at 0,3,...,30 — the last lands ON the bound, and the next would pass it.
	if r.reads != 11 {
		t.Errorf("reads = %d, want 11 within the 30s bound", r.reads)
	}
}

// regression: #6242
func TestBackstopUnknownThenClean(t *testing.T) {
	r := &backstopRig{states: []string{"UNKNOWN", "DRAFT", "CLEAN"}}
	out := r.run()
	if out.Verdict != BackstopPass || r.reads != 3 || len(r.comments) != 0 {
		t.Errorf("%+v reads=%d comments=%d, want pass after 3 reads, no comment", out, r.reads, len(r.comments))
	}
}

// regression: #6242
func TestBackstopConvertFails(t *testing.T) {
	r := &backstopRig{states: []string{"BLOCKED"}, convErr: errors.New("boom")}
	out := r.run()
	if out.Verdict != BackstopRevertFailed || IsRefused(out.Err) || out.Err == nil || r.converts != 1 {
		t.Fatalf("%+v converts=%d, want revert-failed, Unverifiable, exactly one attempt", out, r.converts)
	}
	if !strings.Contains(out.Err.Error(), "NOT reverted") || len(r.comments) != 0 {
		t.Errorf("err=%v comments=%q", out.Err, r.comments)
	}
}

// A failed read, an empty state and an unrecognised state are could-not-check, never a revert.
func TestBackstopUnreadable(t *testing.T) {
	for name, r := range map[string]*backstopRig{
		"read error": {states: []string{"CLEAN"}, readErr: errors.New("503")},
		"empty":      {states: []string{""}},
		"unknown-to": {states: []string{"SOMETHING_NEW"}},
	} {
		out := r.run()
		if out.Verdict != BackstopUnchecked || r.converts != 0 || len(r.comments) != 1 {
			t.Errorf("%s: %+v converts=%d comments=%d", name, out, r.converts, len(r.comments))
		}
	}
}
