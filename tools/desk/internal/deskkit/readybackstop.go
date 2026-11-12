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
// The verdict is GitHub's mergeStateStatus (PullRequest.MergeStateStatus):
//
//   - BLOCKED, DIRTY: the forge disagrees. Convert to draft ONCE, post one comment naming the
//     state and the head, and fail with a Refused error. A failed conversion is never retried;
//     it is an Unverifiable error saying the change is still ready.
//   - CLEAN, HAS_HOOKS, UNSTABLE, BEHIND: the change stays ready (ReadyBackstopPasses).
//     UNSTABLE (a non-required check is failing) and BEHIND (the base moved on) are still
//     mergeable or merely need updating, and the ruling names only BLOCKED and DIRTY.
//   - UNKNOWN (and DRAFT, the flip not yet visible): the forge has not settled. Re-read on
//     an interval, bounded by a deadline (30s in production).
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

// readyBackstopReverts lists the merge states that return a flipped change to draft.
var readyBackstopReverts = []string{"BLOCKED", "DIRTY"}

// readyBackstopSettling lists the states the forge reports while it has not decided yet.
var readyBackstopSettling = []string{"UNKNOWN", "DRAFT"}

// BackstopVerdict is the backstop's outcome class.
type BackstopVerdict string

const (
	// BackstopPass: the forge's state is in ReadyBackstopPasses; nothing was written.
	BackstopPass BackstopVerdict = "pass"
	// BackstopReverted: the forge reported BLOCKED or DIRTY and the change is a draft again.
	BackstopReverted BackstopVerdict = "reverted"
	// BackstopRevertFailed: the forge reported BLOCKED or DIRTY and ConvertToDraft failed.
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

	// ReadState re-reads the change and returns its merge state (any case).
	ReadState func() (string, error)
	// ConvertToDraft returns the change to draft. Called at most once.
	ConvertToDraft func() error
	// Comment posts one comment on the change.
	Comment func(body string) error

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
	// Reads is how many times ReadState was called.
	Reads int
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
		st, err := b.ReadState()
		out.Reads++
		out.State, readErr = strings.ToUpper(strings.TrimSpace(st)), err
		if err != nil {
			out.State = ""
		}
		if err == nil && !backstopIn(readyBackstopSettling, out.State) {
			break
		}
		if now().Add(b.Interval).After(deadline) {
			break
		}
		sleep(b.Interval)
	}

	switch {
	case readErr == nil && backstopIn(ReadyBackstopPasses, out.State):
		out.Verdict = BackstopPass
		return out

	case readErr == nil && backstopIn(readyBackstopReverts, out.State):
		if err := b.ConvertToDraft(); err != nil {
			out.Verdict = BackstopRevertFailed
			out.Err = Unverifiable(fmt.Sprintf(
				"the ready flip on %s was NOT reverted: the forge reports merge state %s at %s, and "+
					"converting the change back to draft failed (%s). The change is READY but not mergeable "+
					"per the forge; return it to draft by hand. Nothing was retried.",
				change, out.State, b.HeadSHA, firstLineOf(err.Error())), err)
			return out
		}
		out.Verdict = BackstopReverted
		msg := fmt.Sprintf("the ready flip on %s was reverted because the forge disagrees: its merge "+
			"state is %s at %s, so the change is a draft again", change, out.State, b.HeadSHA)
		if cerr := b.Comment(readyBackstopRevertBody(out.State, b.HeadSHA)); cerr != nil {
			msg += fmt.Sprintf(" (the explaining comment could not be posted: %s)", firstLineOf(cerr.Error()))
		}
		out.Err = Refused(msg)
		return out
	}

	out.Verdict = BackstopUnchecked
	why := "the merge state was still " + out.State + " when the " + b.Deadline.String() + " bound ran out"
	switch {
	case readErr != nil:
		why = "the merge state could not be read (" + firstLineOf(readErr.Error()) + ")"
	case out.State == "":
		why = "the forge served no merge state for the change"
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

func readyBackstopRevertBody(state, head string) string {
	return fmt.Sprintf("Returned to draft by the post-flip check: right after this change was marked ready, "+
		"the forge reported its merge state as `%s` at head `%s`.\n\n"+
		"Ready means mergeable per the forge, so the flip was reverted. The ready verb's own checks had "+
		"passed, which means they and the forge's merge verdict disagree; the merge state names what the "+
		"forge still requires.", state, head)
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
