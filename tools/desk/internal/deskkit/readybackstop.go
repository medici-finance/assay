package deskkit

import (
	"fmt"
	"strings"
	"time"
)

// The post-flip backstop (medici-finance/assay#6242).
//
// THE INVARIANT: ready means mergeable per the forge. The ready verbs (cmd/deskflip and
// `deskpost ready`) decide "green" and "approved" from their own reading of the forge's
// records, and every way that reading has diverged from the forge's own merge verdict has
// been a separate bug. The backstop closes the class: right after a successful ready
// mutation it reads the forge's merge verdict for the change and, when the forge disagrees,
// returns the change to draft. It is the ONE caller of Forge.ConvertToDraft, admitted for
// this use only by the ruling
// https://github.com/medici-finance/assay/issues/6242#issuecomment-6812384155
//
// The verdict is GitHub's mergeStateStatus (PullRequest.MergeStateStatus), read TOGETHER with
// the change's draft flag (PullRequest.Draft):
//
//   - The change still reads as a draft, whatever its merge state: the flip is not yet visible
//     to the read. GitHub reports a draft as BLOCKED, so a lagging read right after the flip
//     says BLOCKED for a change the forge may well call mergeable once the flip shows. This is
//     settling, never a revert (medici-finance/assay#6253 review F1).
//   - DIRTY with the change NOT a draft: the forge disagrees. Convert to draft ONCE, post one
//     comment naming the state and the head, and fail with a Refused error. A failed conversion
//     is never retried; it is an Unverifiable error saying the change is still ready.
//   - BLOCKED with the change NOT a draft: judged by the requirements (medici-finance/assay#6508).
//     GitHub computes mergeStateStatus for the identity that reads it, and a role App that may
//     not merge reads BLOCKED on a change the maintainer's merge box calls ready to merge. So a
//     BLOCKED read alone proves nothing; the backstop reads the merge requirements that say the
//     same thing to every reader (MergeRequirements: no merge conflict, the review decision, the
//     required checks at the head). Any requirement unmet reverts, exactly as DIRTY does, and so
//     does a requirement read that fails, or a base-branch rule the read sees but does not
//     evaluate (MergeRequirements.Unevaluated): it never fails open. Only when every requirement
//     is met and no such rule is seen does the change stay ready, with one could-not-check
//     comment, since a rule the read cannot see may still hold the merge.
//   - CLEAN, HAS_HOOKS, UNSTABLE, BEHIND: the change stays ready (ReadyBackstopPasses).
//     UNSTABLE (a non-required check is failing) and BEHIND (the base moved on) are still
//     mergeable or merely need updating.
//   - UNKNOWN (and DRAFT, the flip not yet visible), or the draft flag still set: the forge
//     has not settled. Re-read on an interval, bounded by a deadline (30s in production).
//   - Still unsettled at the deadline, an empty state (a backend that serves none), an
//     unrecognised state, or a failed read: could-not-check. The change stays ready, one
//     comment says the verdict could not be checked, and the caller prints the notice.

// ReadyBackstopDeadline and ReadyBackstopInterval are the production bound and re-read
// interval for an unsettled merge state. The ruling sets the bound: 30s in total.
const (
	ReadyBackstopDeadline = 30 * time.Second
	ReadyBackstopInterval = 3 * time.Second
)

// ReadyBackstopPasses lists the merge states that leave a flipped change ready.
var ReadyBackstopPasses = []string{"CLEAN", "HAS_HOOKS", "UNSTABLE", "BEHIND"}

// readyBackstopReverts lists the merge states that return a flipped change to draft whoever
// reads them (#6508).
var readyBackstopReverts = []string{"DIRTY"}

// readyBackstopViewerRelative lists the merge states GitHub computes for the reading identity.
// One of them reverts when a merge requirement is unmet or cannot be read, and is otherwise
// could-not-check (#6508).
var readyBackstopViewerRelative = []string{"BLOCKED"}

// readyBackstopSettling lists the states the forge reports while it has not decided yet. A
// read whose draft flag is still set is settling too, whatever its state (backstopSettling).
var readyBackstopSettling = []string{"UNKNOWN", "DRAFT"}

// backstopSettling reports whether one read is unsettled: the change still reads as a draft,
// or the forge has not decided its merge state.
func backstopSettling(state string, draft bool) bool {
	return draft || backstopIn(readyBackstopSettling, state)
}

// BackstopVerdict is the backstop's outcome class.
type BackstopVerdict string

const (
	// BackstopPass: the forge's state is in ReadyBackstopPasses; nothing was written.
	BackstopPass BackstopVerdict = "pass"
	// BackstopReverted: the forge reported DIRTY, or BLOCKED with a requirement unmet or
	// unreadable, and the change is a draft again.
	BackstopReverted BackstopVerdict = "reverted"
	// BackstopRevertFailed: the change was due a revert and ConvertToDraft failed.
	BackstopRevertFailed BackstopVerdict = "revert-failed"
	// BackstopUnchecked: no settled verdict could be read; the change stays ready.
	BackstopUnchecked BackstopVerdict = "could-not-check"
)

// ReadyBackstop is one post-flip check. The verb supplies the three forge operations as
// closures so the same logic serves both verbs' backends (the deskkit.Forge of deskflip and
// the postBackend of deskpost ready).
type ReadyBackstop struct {
	// Repo (owner/name), Number and HeadSHA name the change in messages and the comment.
	Repo    string
	Number  int
	HeadSHA string

	// ReadState re-reads the change and returns its merge state (any case) and its draft
	// flag. A read that still shows the change as a draft is settling, whatever the state.
	ReadState func() (state string, draft bool, err error)
	// ConvertToDraft returns the change to draft. Called at most once.
	ConvertToDraft func() error
	// Comment posts one comment on the change.
	Comment func(body string) error
	// Requirements reads the change's merge requirements (MergeRequirements). It is read only
	// after a non-draft BLOCKED read (#6508). A nil closure or a failed read counts as unmet, so
	// the change returns to draft.
	Requirements func() (*MergeRequirements, error)

	// Deadline bounds the re-reads of an unsettled state, from the first read; Interval is
	// the wait between reads. Zero Deadline means one read. Now and Sleep default to the
	// wall clock; tests inject them.
	Deadline time.Duration
	Interval time.Duration
	Now      func() time.Time
	Sleep    func(time.Duration)
}

// BackstopOutcome is what Run found and did.
type BackstopOutcome struct {
	Verdict BackstopVerdict
	// State is the last merge state read, upper-cased ("" when none was read).
	State string
	// Draft is the last read's draft flag.
	Draft bool
	// Reads is how many times ReadState was called.
	Reads int
	// Unmet lists the merge requirements a BLOCKED read was reverted for (MergeRequirements.Unmet,
	// or the failed read).
	Unmet []string
	// Notice is the could-not-verify line for BackstopUnchecked, and a warning when a comment
	// could not be posted. The caller prints it on stderr.
	Notice string
	// Err is non-nil for BackstopReverted (Refused) and BackstopRevertFailed (Unverifiable).
	Err error
}

// Run performs the backstop. Call it only after a ready mutation that succeeded.
func (b ReadyBackstop) Run() BackstopOutcome {
	now, sleep := b.Now, b.Sleep
	if now == nil {
		now = time.Now
	}
	if sleep == nil {
		sleep = time.Sleep
	}
	change := fmt.Sprintf("%s#%d", b.Repo, b.Number)
	deadline := now().Add(b.Deadline)

	var out BackstopOutcome
	var readErr error
	for {
		st, draft, err := b.ReadState()
		out.Reads++
		out.State, out.Draft, readErr = strings.ToUpper(strings.TrimSpace(st)), draft, err
		if err != nil {
			out.State, out.Draft = "", false
		}
		if err == nil && !backstopSettling(out.State, out.Draft) {
			break
		}
		if now().Add(b.Interval).After(deadline) {
			break
		}
		sleep(b.Interval)
	}

	switch {
	case readErr == nil && out.Draft:
		// Still a draft at the deadline: never a revert, whatever the state (F1).
	case readErr == nil && backstopIn(ReadyBackstopPasses, out.State):
		out.Verdict = BackstopPass
		return out

	case readErr == nil && backstopIn(readyBackstopViewerRelative, out.State):
		// GitHub computed this state for the reader: judge it by the requirements (#6508).
		if out.Unmet = b.unmetRequirements(); len(out.Unmet) > 0 {
			return b.revert(out, change)
		}
	case readErr == nil && backstopIn(readyBackstopReverts, out.State):
		return b.revert(out, change)
	}

	out.Verdict = BackstopUnchecked
	why := "the merge state was still " + out.State + " when the " + b.Deadline.String() + " bound ran out"
	switch {
	case readErr != nil:
		why = "the merge state could not be read (" + firstLineOf(readErr.Error()) + ")"
	case out.Draft:
		why = "the change still read as a draft (merge state " + out.State + ") when the " +
			b.Deadline.String() + " bound ran out"
	case out.State == "":
		why = "the forge served no merge state for the change"
	case backstopIn(readyBackstopViewerRelative, out.State):
		why = "the forge reported " + out.State + ", which GitHub computes for the reading identity, and every " +
			"merge requirement the check reads is met (no merge conflict, the review decision, the required " +
			"checks at the head); a rule outside those may still hold the merge"
	case !backstopIn(readyBackstopSettling, out.State):
		why = "the forge reported a merge state this check does not recognise (" + out.State + ")"
	}
	out.Notice = fmt.Sprintf("could-not-verify: %s stays ready, but its merge verdict could not be checked "+
		"after the flip: %s.", change, why)
	if cerr := b.Comment(readyBackstopUncheckedBody(why, b.HeadSHA)); cerr != nil {
		out.Notice += fmt.Sprintf(" The could-not-check comment could not be posted: %s.", firstLineOf(cerr.Error()))
	}
	return out
}

// unmetRequirements reads the requirements for a BLOCKED read. A nil closure or a failed read
// is one unmet line: an unreadable requirement reverts, never passes (#6508).
func (b ReadyBackstop) unmetRequirements() []string {
	if b.Requirements == nil {
		return []string{"the merge requirements could not be read (this backend serves no requirement read)"}
	}
	req, err := b.Requirements()
	if err != nil {
		return []string{"the merge requirements could not be read (" + firstLineOf(err.Error()) + ")"}
	}
	if req != nil && req.HeadSHA != "" && b.HeadSHA != "" && !strings.EqualFold(req.HeadSHA, b.HeadSHA) {
		return []string{fmt.Sprintf("the merge requirements were read for head %s, not the flipped head", req.HeadSHA)}
	}
	return req.Unmet()
}

// revert returns the change to draft once and posts the one explaining comment.
func (b ReadyBackstop) revert(out BackstopOutcome, change string) BackstopOutcome {
	because := ""
	if len(out.Unmet) > 0 {
		because = " (" + strings.Join(out.Unmet, "; ") + ")"
	}
	if err := b.ConvertToDraft(); err != nil {
		out.Verdict = BackstopRevertFailed
		out.Err = Unverifiable(fmt.Sprintf(
			"the ready flip on %s was NOT reverted: the forge reports merge state %s at %s%s, and "+
				"converting the change back to draft failed (%s). The change is READY but not mergeable "+
				"per the forge; return it to draft by hand. Nothing was retried.",
			change, out.State, b.HeadSHA, because, firstLineOf(err.Error())), err)
		return out
	}
	out.Verdict = BackstopReverted
	msg := fmt.Sprintf("the ready flip on %s was reverted because the forge disagrees: its merge "+
		"state is %s at %s%s, so the change is a draft again", change, out.State, b.HeadSHA, because)
	if cerr := b.Comment(readyBackstopRevertBody(out.State, b.HeadSHA, out.Unmet)); cerr != nil {
		msg += fmt.Sprintf(" (the explaining comment could not be posted: %s)", firstLineOf(cerr.Error()))
	}
	out.Err = Refused(msg)
	return out
}

// readyBackstopUnmetLead starts each line of a revert comment that names an unmet requirement.
// FindReadyBackstopRevert keys on it: a BLOCKED revert that names one holds its head.
const readyBackstopUnmetLead = "Requirement not met: "

func readyBackstopRevertBody(state, head string, unmet []string) string {
	reqs := ""
	for _, u := range unmet {
		reqs += readyBackstopUnmetLead + u + ".\n"
	}
	if reqs != "" {
		reqs += "\n"
	}
	return fmt.Sprintf("Returned to draft by the post-flip check: right after this change was marked ready, "+
		"the forge reported its merge state as `%s` at head `%s`.\n\n%s"+
		"Ready means mergeable per the forge, so the flip was reverted. The ready verb's own checks had "+
		"passed, which means they and the forge's merge verdict disagree; the merge state names what the "+
		"forge still requires. Neither deskflip nor deskpost ready will flip this head again; a new head clears that."+
		"\n\n%s", state, head, reqs, ReadyBackstopRevertMarker(head))
}

// ReadyBackstopRevertMarker is the hidden line the backstop's revert comment carries for head:
// the stable key the pre-flip not-reverted check of BOTH flip verbs matches (NotRevertedHold), so a
// head the backstop already returned to draft is never flipped again on the next tick
// (medici-finance/assay#6253 review F2, medici-finance/assay#6254). It names the head, so a new
// head never matches.
func ReadyBackstopRevertMarker(head string) string {
	return "<!-- assay:ready-backstop-revert head=" + strings.ToLower(strings.TrimSpace(head)) + " -->"
}

// HasReadyBackstopRevert reports whether body carries the revert marker for head as a line of
// its own. It matches the exact marker line, never free text, and an empty head never matches.
func HasReadyBackstopRevert(body, head string) bool {
	if strings.TrimSpace(head) == "" {
		return false
	}
	want := ReadyBackstopRevertMarker(head)
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

// FindReadyBackstopRevert returns the first comment that carries the revert marker for head
// and was written by a trusted identity (TrustedAuthor: the backstop posts as a role App). A
// marker from any other author is ignored, so a stranger cannot hold a change out of ready by
// planting one. A revert comment that records a viewer-relative state and names no unmet
// requirement (a BLOCKED revert an earlier release posted on the state alone, #6508) is ignored
// too: that read was never evidence against the head, and the next flip's backstop judges the
// head's requirements again.
func FindReadyBackstopRevert(comments []Comment, head string) (Comment, bool) {
	for _, c := range comments {
		if TrustedAuthor(c.Author.Login) && HasReadyBackstopRevert(c.Body, head) && !stateOnlyRevert(c.Body) {
			return c, true
		}
	}
	return Comment{}, false
}

// NotRevertedHold is the shared pre-flip not-reverted condition: the reason a flip of head must
// stop because the post-flip backstop already returned it to draft, or "" when no trusted revert
// comment names this head. deskflip and deskpost ready both call it, so the two verbs that perform
// the ready mutation cannot disagree about a reverted head (medici-finance/assay#6254).
//
// REFUSE-ONLY. A match can only hold a flip; no result admits one, so a comment never satisfies a
// gate through it. A marker from an untrusted author, or for another head, matches nothing.
//
// shown is the head as the calling verb prints it (each verb abbreviates a SHA its own way); the
// match itself always uses the full head.
func NotRevertedHold(comments []Comment, pr int, head, shown string) (string, bool) {
	c, ok := FindReadyBackstopRevert(comments, head)
	if !ok {
		return "", false
	}
	ref := c.URL
	if ref == "" {
		ref = fmt.Sprintf("comment %d", c.DatabaseID)
	}
	return fmt.Sprintf("the post-flip check already returned PR #%d to draft at %s (%s): the forge "+
		"reported it not mergeable right after a flip at this head, so flipping it again would be "+
		"reverted again. A new head clears this condition.", pr, shown, ref), true
}

// stateOnlyRevert reports whether a revert comment records a viewer-relative state without
// naming any unmet requirement.
func stateOnlyRevert(body string) bool {
	if !backstopIn(readyBackstopViewerRelative, revertedState(body)) {
		return false
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), readyBackstopUnmetLead) {
			return false
		}
	}
	return true
}

// revertedState returns the merge state a revert comment records (the `X` in "reported its merge
// state as `X`"), upper-cased, or "" when the body names none.
func revertedState(body string) string {
	const lead = "reported its merge state as `"
	i := strings.Index(body, lead)
	if i < 0 {
		return ""
	}
	rest := body[i+len(lead):]
	j := strings.IndexByte(rest, '`')
	if j < 0 {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(rest[:j]))
}

func readyBackstopUncheckedBody(why, head string) string {
	return fmt.Sprintf("Could not check the forge's merge verdict after this change was marked ready, at head "+
		"`%s`: %s.\n\nThe change stays ready. Confirm by hand that the forge reports it mergeable.", head, why)
}

func backstopIn(set []string, s string) bool {
	for _, v := range set {
		if v == s {
			return true
		}
	}
	return false
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
