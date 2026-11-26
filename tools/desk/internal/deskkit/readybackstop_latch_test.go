package deskkit

import (
	"errors"
	"strings"
	"testing"
)

// The re-flip latch (#7096): the backstop polls a transient mergeable=UNKNOWN instead of
// reverting on it, records why it reverted in the marker, and the not-reverted condition clears
// a revert once a fresh read shows that recorded cause no longer holds.

// runSeq runs a non-draft BLOCKED read whose successive Requirements reads answer seq, the last
// repeating.
func (r *backstopRig) runSeq(seq ...*MergeRequirements) BackstopOutcome {
	r.states = []string{"blocked"}
	b := r.backstop()
	b.Requirements = func() (*MergeRequirements, error) {
		i := min(r.reqReads, len(seq)-1)
		r.reqReads++
		return seq[i], nil
	}
	return b.Run()
}

func withMergeable(v string) *MergeRequirements {
	m := metRequirements()
	m.Mergeable = v
	return m
}

// An UNKNOWN conflict verdict that settles within the bound is judged on the settled read:
// MERGEABLE leaves the change ready (the viewer-relative could-not-check), CONFLICTING reverts
// recording the mergeable cause.
//
// regression: #7096
func TestBackstopMergeableUnknownSettles(t *testing.T) {
	r := &backstopRig{}
	out := r.runSeq(withMergeable("UNKNOWN"), withMergeable("UNKNOWN"), withMergeable("MERGEABLE"))
	if out.Verdict != BackstopUnchecked || r.converts != 0 || r.reqReads != 3 {
		t.Fatalf("%+v converts=%d reqReads=%d, want could-not-check after 3 reads, no revert", out, r.converts, r.reqReads)
	}
	if len(r.comments) != 1 || HasReadyBackstopRevert(r.comments[0], "abc123") {
		t.Errorf("comments = %q, want ONE could-not-check comment and no revert marker", r.comments)
	}

	r = &backstopRig{}
	out = r.runSeq(withMergeable("UNKNOWN"), withMergeable("CONFLICTING"))
	if out.Verdict != BackstopReverted || r.converts != 1 || r.reqReads != 2 {
		t.Fatalf("%+v converts=%d reqReads=%d, want a revert on the settled CONFLICTING read", out, r.converts, r.reqReads)
	}
	if got := strings.Join(out.Causes, ","); got != RevertCauseMergeable {
		t.Errorf("causes = %q, want %q", got, RevertCauseMergeable)
	}
	if !strings.Contains(r.comments[0], ReadyBackstopRevertMarkerFor("abc123", []string{RevertCauseMergeable})) {
		t.Errorf("revert comment = %q, want the marker recording cause mergeable", r.comments[0])
	}
}

// An UNKNOWN that never settles is could-not-check: the change stays ready and no marker is
// written, so nothing latches. With another requirement unmet the change reverts for that one
// alone, and the marker records only it.
//
// regression: #7096
func TestBackstopMergeableUnknownNeverSettles(t *testing.T) {
	r := &backstopRig{}
	out := r.runSeq(withMergeable("UNKNOWN"))
	if out.Verdict != BackstopUnchecked || out.Err != nil || r.converts != 0 {
		t.Fatalf("%+v converts=%d, want could-not-check and no revert", out, r.converts)
	}
	// 30s bound, 3s interval, the first read at zero: 11 reads.
	if r.reqReads != 11 {
		t.Errorf("requirement reads = %d, want 11 across the 30s bound", r.reqReads)
	}
	if len(r.comments) != 1 || HasReadyBackstopRevert(r.comments[0], "abc123") ||
		!strings.Contains(r.comments[0], "mergeable UNKNOWN") {
		t.Errorf("comments = %q, want ONE could-not-check comment naming mergeable UNKNOWN, no marker", r.comments)
	}

	m := withMergeable("UNKNOWN")
	m.ReviewDecision = "CHANGES_REQUESTED"
	r = &backstopRig{}
	out = r.runSeq(m)
	if out.Verdict != BackstopReverted || r.converts != 1 {
		t.Fatalf("%+v converts=%d, want a revert for the review decision", out, r.converts)
	}
	if got := strings.Join(out.Causes, ","); got != RevertCauseReview {
		t.Errorf("causes = %q, want only %q: an unsettled verdict is never a cause", got, RevertCauseReview)
	}
	if strings.Contains(r.comments[0], "has not computed") {
		t.Errorf("revert comment names the unsettled verdict as unmet: %q", r.comments[0])
	}
}

// Every revert records its causes in the marker, one word per cause, in first-seen order.
//
// regression: #7096
func TestBackstopRevertRecordsCause(t *testing.T) {
	r := &backstopRig{states: []string{"DIRTY"}}
	r.run()
	if !strings.Contains(r.comments[0], "<!-- assay:ready-backstop-revert head=abc123 cause=mergeable -->") {
		t.Errorf("DIRTY revert = %q, want the marker recording cause mergeable", r.comments[0])
	}
	m := metRequirements()
	m.ReviewDecision, m.Required[0].State, m.Complete = "CHANGES_REQUESTED", "FAILURE", false
	r = &backstopRig{states: []string{"blocked"}, req: m}
	r.run()
	if !strings.Contains(r.comments[0], "<!-- assay:ready-backstop-revert head=abc123 cause=review,checks,coverage -->") {
		t.Errorf("BLOCKED revert = %q, want the marker recording review,checks,coverage", r.comments[0])
	}
	r = &backstopRig{states: []string{"blocked"}, reqErr: errors.New("graphql 502")}
	r.run()
	if !strings.Contains(r.comments[0], "cause=unreadable -->") {
		t.Errorf("unreadable revert = %q, want the marker recording cause unreadable", r.comments[0])
	}
}

// The marker parses as a line of its own only, in its two forms; anything else names no head.
func TestRevertMarkerParse(t *testing.T) {
	for line, want := range map[string]struct {
		ok, recorded bool
		causes       string
	}{
		"<!-- assay:ready-backstop-revert head=abc123 -->":                     {ok: true},
		"  <!-- assay:ready-backstop-revert head=abc123 cause=review -->  ":    {ok: true, recorded: true, causes: "review"},
		"<!-- assay:ready-backstop-revert head=abc123 cause=review,checks -->": {ok: true, recorded: true, causes: "review,checks"},
		"<!-- assay:ready-backstop-revert head=abc123 cause= -->":              {ok: true, recorded: true, causes: ""},
		"<!-- assay:ready-backstop-revert head=abc123 extra=1 -->":             {},
		"<!-- assay:ready-backstop-revert head=abc123 cause=a b -->":           {},
		"<!-- assay:ready-backstop-revert head= -->":                           {},
		"see <!-- assay:ready-backstop-revert head=abc123 --> here":            {},
	} {
		head, causes, recorded, ok := parseReadyBackstopRevert(line)
		if ok != want.ok || recorded != want.recorded || (ok && (head != "abc123" || strings.Join(causes, ",") != want.causes)) {
			t.Errorf("%q: head=%q causes=%q recorded=%v ok=%v, want %+v", line, head, causes, recorded, ok, want)
		}
	}
	if !HasReadyBackstopRevert(ReadyBackstopRevertMarkerFor("ABC123", []string{"review"}), "abc123") {
		t.Error("the marker written for a head does not match it")
	}
}

// latchComment is a trusted revert comment for head carrying marker.
func latchComment(t *testing.T, id int64, marker string) Comment {
	t.Helper()
	c := Comment{DatabaseID: id, Body: "Returned to draft by the post-flip check: right after this change was marked " +
		"ready, the forge reported its merge state as `DIRTY` at head `abc123`.\n\n" + marker}
	c.Author.Login = "assay-reviewer-app[bot]"
	return c
}

func latchRoster(t *testing.T) {
	t.Helper()
	withRoster(t, forgeRoster("reviewer=github:assay-reviewer-app:300000004", nil))
}

// latchHold runs NotRevertedHold with a reread answering req/err, and counts the reads.
func latchHold(comments []Comment, req *MergeRequirements, err error) (string, bool, int) {
	reads := 0
	reason, held := NotRevertedHold(comments, 7, "abc123", "abc123", func() (*MergeRequirements, error) {
		reads++
		return req, err
	})
	return reason, held, reads
}

// A recorded cause that a fresh read shows cleared no longer holds the head: the #7310 shape,
// reverted on CHANGES_REQUESTED and since re-approved at the same head.
//
// regression: #7096
func TestNotRevertedCauseCleared(t *testing.T) {
	latchRoster(t)
	cs := []Comment{latchComment(t, 1, ReadyBackstopRevertMarkerFor("abc123", []string{RevertCauseReview}))}
	reason, held, reads := latchHold(cs, metRequirements(), nil)
	if held || reads != 1 {
		t.Fatalf("held=%v reads=%d reason=%q, want released after ONE fresh read", held, reads, reason)
	}
	if !strings.Contains(reason, "cause review, which no longer holds") {
		t.Errorf("note = %q, want it to name the cleared cause", reason)
	}
	// Only the recorded cause is re-judged: a check still running holds nothing here (the
	// verb's own checks-green condition and the next backstop judge it).
	m := metRequirements()
	m.Required[0].State = "IN_PROGRESS"
	if _, held, _ := latchHold(cs, m, nil); held {
		t.Error("a requirement outside the recorded cause held the head")
	}
}

// A recorded cause that still holds keeps refusing, so a flip/revert loop at one head stays
// impossible.
//
// regression: #7096
func TestNotRevertedCauseStillHolds(t *testing.T) {
	latchRoster(t)
	for cause, edit := range map[string]func(*MergeRequirements){
		RevertCauseReview:    func(m *MergeRequirements) { m.ReviewDecision = "CHANGES_REQUESTED" },
		RevertCauseMergeable: func(m *MergeRequirements) { m.Mergeable = "UNKNOWN" },
		RevertCauseChecks:    func(m *MergeRequirements) { m.Required[0].State = "FAILURE" },
		RevertCauseRules:     func(m *MergeRequirements) { m.Unevaluated = []string{"a ruleset rule of type MERGE_QUEUE"} },
		RevertCauseCoverage:  func(m *MergeRequirements) { m.Complete = false },
	} {
		m := metRequirements()
		edit(m)
		cs := []Comment{latchComment(t, 1, ReadyBackstopRevertMarkerFor("abc123", []string{cause}))}
		reason, held, _ := latchHold(cs, m, nil)
		if !held || !strings.Contains(reason, "the cause it recorded still holds") || !strings.Contains(reason, "comment 1") {
			t.Errorf("%s: held=%v reason=%q, want held naming the cause and the comment", cause, held, reason)
		}
	}
	// Two reverts at the head: both must clear.
	cs := []Comment{
		latchComment(t, 1, ReadyBackstopRevertMarkerFor("abc123", []string{RevertCauseReview})),
		latchComment(t, 2, ReadyBackstopRevertMarkerFor("abc123", []string{RevertCauseChecks})),
	}
	m := metRequirements()
	m.Required[0].State = "FAILURE"
	if reason, held, _ := latchHold(cs, m, nil); !held || !strings.Contains(reason, "comment 2") {
		t.Errorf("held=%v reason=%q, want the second revert's cause to hold", held, reason)
	}
}

// A marker written before causes were recorded, or one recording an unreadable read, clears
// only when a fresh read shows every requirement met.
//
// regression: #7096
func TestNotRevertedLegacyMarker(t *testing.T) {
	latchRoster(t)
	for name, marker := range map[string]string{
		"legacy":     ReadyBackstopRevertMarker("abc123"),
		"unreadable": ReadyBackstopRevertMarkerFor("abc123", []string{RevertCauseUnreadable}),
	} {
		cs := []Comment{latchComment(t, 1, marker)}
		if reason, held, _ := latchHold(cs, metRequirements(), nil); held {
			t.Errorf("%s: held with every requirement met: %s", name, reason)
		}
		m := metRequirements()
		m.Required[0].State = "IN_PROGRESS"
		reason, held, _ := latchHold(cs, m, nil)
		if !held || !strings.Contains(reason, "every merge requirement met") || !strings.Contains(reason, "build-test is IN_PROGRESS") {
			t.Errorf("%s: held=%v reason=%q, want held naming what is unmet", name, held, reason)
		}
	}
}

// A fresh read that fails, returns nothing, describes another head, or is not served keeps the
// revert holding: the cause cannot be shown cleared.
//
// regression: #7096
func TestNotRevertedReadFailsClosed(t *testing.T) {
	latchRoster(t)
	cs := []Comment{latchComment(t, 1, ReadyBackstopRevertMarkerFor("abc123", []string{RevertCauseReview}))}
	other := metRequirements()
	other.HeadSHA = "fff999"
	for name, c := range map[string]struct {
		req *MergeRequirements
		err error
	}{
		"error":      {nil, errors.New("graphql 502")},
		"nil read":   {nil, nil},
		"other head": {other, nil},
		"no head":    {&MergeRequirements{Mergeable: "MERGEABLE", ReviewDecision: "APPROVED", Complete: true}, nil},
	} {
		if reason, held, _ := latchHold(cs, c.req, c.err); !held || !strings.Contains(reason, "still holds") {
			t.Errorf("%s: held=%v reason=%q, want held", name, held, reason)
		}
	}
	if reason, held := NotRevertedHold(cs, 7, "abc123", "abc123", nil); !held || !strings.Contains(reason, "serves no") {
		t.Errorf("no reread: held=%v reason=%q, want held", held, reason)
	}
	// A cause this release does not recognise, or an empty cause list, cannot be shown cleared.
	for _, marker := range []string{
		ReadyBackstopRevertMarkerFor("abc123", []string{"future-cause"}),
		"<!-- assay:ready-backstop-revert head=abc123 cause= -->",
	} {
		if reason, held, _ := latchHold([]Comment{latchComment(t, 1, marker)}, metRequirements(), nil); !held ||
			!strings.Contains(reason, "does not recognise") {
			t.Errorf("%q: held=%v reason=%q, want held on an unrecognised cause", marker, held, reason)
		}
	}
}

// At ReadyBackstopRevertCap reverts at one head the condition holds whatever a read shows, and
// does not read; with no trusted revert at the head it neither holds nor reads.
//
// regression: #7096
func TestNotRevertedCapAndNoRead(t *testing.T) {
	latchRoster(t)
	var cs []Comment
	for i := 0; i < ReadyBackstopRevertCap; i++ {
		cs = append(cs, latchComment(t, int64(i+1), ReadyBackstopRevertMarkerFor("abc123", []string{RevertCauseReview})))
	}
	if reason, held, reads := latchHold(cs, metRequirements(), nil); !held || reads != 0 || !strings.Contains(reason, "3 times") {
		t.Errorf("held=%v reads=%d reason=%q, want held at the cap without a read", held, reads, reason)
	}
	if _, held, _ := latchHold(cs[:ReadyBackstopRevertCap-1], metRequirements(), nil); held {
		t.Error("below the cap, a cleared cause still held")
	}
	stranger := latchComment(t, 9, ReadyBackstopRevertMarkerFor("abc123", []string{RevertCauseReview}))
	stranger.Author.Login = "drive-by-user"
	for _, set := range [][]Comment{nil, {stranger},
		{latchComment(t, 1, ReadyBackstopRevertMarkerFor("def456", []string{RevertCauseReview}))}} {
		if reason, held, reads := latchHold(set, nil, errors.New("must not read")); held || reason != "" || reads != 0 {
			t.Errorf("held=%v reason=%q reads=%d, want no hold and no read", held, reason, reads)
		}
	}
}
