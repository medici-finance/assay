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
	drafts   []bool // the draft flag per read, the last repeating; nil reads "not a draft"
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
		ReadState: func() (string, bool, error) {
			i := min(r.reads, len(r.states)-1)
			draft := false
			if len(r.drafts) > 0 {
				draft = r.drafts[min(r.reads, len(r.drafts)-1)]
			}
			r.reads++
			return r.states[i], draft, r.readErr
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

// GitHub reports a draft as BLOCKED: a read that still shows the change as a draft is settling,
// so (BLOCKED, draft) then (CLEAN, not draft) stays ready with no comment (#6253 review F1).
//
// regression: #6242
func TestBackstopDraftBlockedSettles(t *testing.T) {
	r := &backstopRig{states: []string{"BLOCKED", "CLEAN"}, drafts: []bool{true, false}}
	out := r.run()
	if out.Verdict != BackstopPass || out.Err != nil || r.converts != 0 || len(r.comments) != 0 || r.reads != 2 {
		t.Errorf("%+v reads=%d converts=%d comments=%d, want a silent pass after 2 reads", out, r.reads, r.converts, len(r.comments))
	}
}

// (BLOCKED, draft) through the whole bound is could-not-check, like UNKNOWN, never a revert.
//
// regression: #6242
func TestBackstopDraftAtDeadline(t *testing.T) {
	r := &backstopRig{states: []string{"BLOCKED"}, drafts: []bool{true}}
	out := r.run()
	if out.Verdict != BackstopUnchecked || out.Err != nil || r.converts != 0 || r.reads != 11 {
		t.Fatalf("%+v reads=%d converts=%d, want could-not-check after 11 reads, no revert", out, r.reads, r.converts)
	}
	if len(r.comments) != 1 || !strings.Contains(r.comments[0], "still read as a draft") {
		t.Errorf("comments = %q, want ONE could-not-check comment naming the draft read", r.comments)
	}
}

// The revert comment carries the hidden marker for its head, matched as an exact line.
//
// regression: #6242
func TestBackstopRevertMarker(t *testing.T) {
	r := &backstopRig{states: []string{"DIRTY"}}
	r.run()
	if len(r.comments) != 1 || !HasReadyBackstopRevert(r.comments[0], "abc123") {
		t.Fatalf("comments = %q, want ONE carrying the marker for abc123", r.comments)
	}
	if HasReadyBackstopRevert(r.comments[0], "def456") || HasReadyBackstopRevert(r.comments[0], "") {
		t.Error("the marker matched another head or an empty head")
	}
	// Only deskflip reads the marker; deskpost ready never reads comments (its anti-forgery
	// control), so the comment must name deskflip alone as the verb that will not re-flip
	// (review F4).
	body := r.comments[0]
	if !strings.Contains(body, "deskflip will not flip this head again; a new head clears that.") ||
		strings.Contains(body, "ready verbs") || strings.Contains(body, "deskpost") {
		t.Errorf("revert comment = %q, want it to name only deskflip as refusing a re-flip", body)
	}
	inline := "see " + ReadyBackstopRevertMarker("abc123") + " here"
	if HasReadyBackstopRevert(inline, "abc123") {
		t.Error("a marker inside free text matched; only an exact line may")
	}
}
