package deskkit

import (
	"fmt"
	"os"
	"os/user"
	"strings"
)

// The LOGGED OVERRIDE for the shared secret scan.
//
// WHAT THIS IS NOT. It is not a softening of the refusal. Policy is unchanged and stated
// in one line: EXIT 5 IS A STOP. A worker that hits a scan refusal still stops; the
// tool-wiring rule that exit 5 is never a fallback trigger is unchanged; nothing here is
// reachable by accident, by an environment variable, or by a retry.
//
// WHAT IT IS. Before this existed, a hard block with no override had exactly two outcomes
// once the scan was wrong, and the house has now paid for both:
//
//   - the PR strands. A fix sat unpushed in TWO abandoned worktrees
//     across two sessions and several days, because rewriting the flagged text would have
//     desynced a recorded Evidence row from what was actually run.
//   - or the worker leaves the sanctioned transport. A worker pushed
//     with raw `git push` + `gh pr create`, which discards not only the scan but every
//     other guarantee the wrapper provides — and leaves NO record that a scan was skipped.
//
// The second is strictly worse than the first, and it is invisible in the artifact. So the
// override's whole purpose is to convert an invisible bypass into an AUDITED one: same
// refusal, same stop, but a documented way through that costs minutes and leaves a row
// naming the tool, the surface digest, the stated reason and the identity that stated it.
// An unlogged bypass remains what it always was — outside the tools, unrecorded, and
// against policy.
//
// THE REFUSAL MUST ADVERTISE IT. A path nobody knows about is a path nobody uses; a
// worker did not route around the scan out of malice, but because the refusal said
// "no override flag by design". OverrideHint is appended to the scan refusals so the
// message itself carries the way out.

// ScanOverrideFlag is the flag name every refusal verb uses for the logged override. One
// name across the whole tool family: a worker that learns it on deskpr can use it on
// deskreply without re-reading a manual.
const ScanOverrideFlag = "force-scan-override"

// ScanOverrideVerb is the audit `verb` of the override row. It is deliberately DISTINCT
// from the verb of the write it accompanies, so `grep scan_override` over the audit log
// enumerates every bypass ever taken without having to reason about which tool wrote it.
const ScanOverrideVerb = "scan_override"

// minOverrideReason is the shortest reason that can be a reason. The point of the field is
// that a human reviewing the audit log later can tell WHY the scan was overridden; "x",
// "-" and "fp" cannot do that, and a field that accepts them trains callers to fill it
// with nothing. Deliberately modest: this is a legibility floor, not a quality bar.
const minOverrideReason = 12

// ScanOverride is one authorised, audited bypass of the shared secret scan.
type ScanOverride struct {
	// Tool is the invoking binary ("deskpr", "deskreply", …).
	Tool string
	// Verb is the write the override is attached to ("create", "update", "reply").
	Verb string
	// Repo is owner/name, when the verb knows it.
	Repo string
	// PR is the pull request the write targets, when the verb knows it.
	PR *int
	// Reason is the operator's stated justification, verbatim.
	Reason string
	// Surface is the name of the surface the scan refused ("PR body", "branch diff …").
	Surface string
	// Content is the exact bytes that were refused. Only its DIGEST is ever written: the
	// audit log is replayed into terminals and agent context, so writing the content
	// itself would make the audit trail the leak.
	Content []byte
	// Refusal is the refusal message being overridden, recorded so the row says what was
	// waved through and not merely that something was.
	Refusal string
}

// ValidateScanOverride checks the operator-supplied half of an override BEFORE any work is
// done, so a malformed one refuses at the same exit code as everything else rather than
// halfway through a push.
func ValidateScanOverride(reason string) error {
	r := strings.TrimSpace(reason)
	if r == "" {
		return Refused("refused: --" + ScanOverrideFlag + " requires a reason argument")
	}
	if len(r) < minOverrideReason {
		return Refused(fmt.Sprintf(
			"refused: --%s reason is %d characters; at least %d are required — the audit row "+
				"has to say WHY the scan was wrong, in words a reviewer can act on",
			ScanOverrideFlag, len(r), minOverrideReason))
	}
	if StripControl(r) != r {
		return Refused("refused: --" + ScanOverrideFlag + " reason contains control characters")
	}
	return nil
}

// LogScanOverride writes the audit row for an authorised bypass and echoes it to stderr.
//
// It is called ONLY on the path where a scan refusal was overridden, and it is called
// BEFORE the write it authorises. That ordering is the whole point: if the write then
// fails, the row still records that the scan was waved through, so the log can never
// under-report bypasses. Belt-and-braces on the same reasoning as the write flow's
// audit-every-path rule.
//
// A failure to WRITE the row is fatal to the override. An override whose audit row did not
// land is an unlogged bypass, which is the thing this exists to prevent — so the error is
// returned and the caller refuses rather than proceeding quietly.
func LogScanOverride(o ScanOverride) error {
	if err := ValidateScanOverride(o.Reason); err != nil {
		return err
	}
	detail := fmt.Sprintf("scan override on %q by %s: %s | refusal: %s",
		o.Surface, OverrideIdentity(), strings.TrimSpace(o.Reason), StripControl(o.Refusal))
	if err := Log(Entry{
		Tool:       o.Tool,
		Verb:       ScanOverrideVerb,
		Result:     ResultOK,
		Detail:     StripControl(detail),
		Repo:       o.Repo,
		PR:         o.PR,
		BodyDigest: Sha256Hex(o.Content),
		ArgsDigest: ArgsDigest(os.Args[1:]),
	}); err != nil {
		return Unverifiable("cannot record the scan-override audit row — refusing rather "+
			"than taking an UNLOGGED bypass", err)
	}
	fmt.Fprintf(os.Stderr,
		"scan-override RECORDED: tool=%s verb=%s surface=%q digest=%s identity=%s reason=%q\n",
		o.Tool, o.Verb, o.Surface, Sha256Hex(o.Content)[:12], OverrideIdentity(),
		strings.TrimSpace(o.Reason))
	return nil
}

// OverrideIdentity is the best identity the process can establish for the audit row: the
// desk session tag and the OS user. Both are SELF-REPORTED — this is forensics, not
// authentication, the same posture SessionTag documents. What makes the row useful is not
// that the identity is unforgeable but that a bypass leaves one at all.
func OverrideIdentity() string {
	who := "unknown"
	if u, err := user.Current(); err == nil && u.Username != "" {
		who = u.Username
	}
	return who + "/" + SessionTag()
}

// impersonationMarker is the fragment of impersonationRefusal's message that identifies it.
// The impersonation guard is NOT a secret scan and is NOT overridable here: it refuses a
// claim made in a configured human's voice, and "the operator asserts it is fine" is
// precisely the assertion it exists to disbelieve. A false positive there is a rewording
// away — unlike a recorded Evidence command or a shipped lockfile, the two
// cases that motivated the override at all.
const impersonationMarker = "reads as a RULING/DECISION claim attributed to"

// HandleScanRefusal is the ONE decision point every refusal verb routes its secret-scan
// result through. Given the scan's verdict and whatever the operator passed to
// --force-scan-override, it returns:
//
//   - nil                     the scan was clean, or the refusal was overridden AND the
//     audit row landed;
//   - the refusal, ANNOTATED  no override was supplied. The message now carries
//     OverrideHint, so the refusal advertises the audited way
//     through instead of ending, as an earlier refusal did, at "no override
//     flag by design";
//   - a different refusal     the override was supplied but is not usable here (bad
//     reason, non-overridable guard, audit write failed).
//
// Verbs call this INSTEAD of returning the scan error directly. Centralising it is what
// makes "every refusal verb advertises the override, spells it the same way, and cannot
// take an unlogged one" a property of the code rather than of four call sites remembering.
func HandleScanRefusal(o ScanOverride, scanErr error) error {
	if scanErr == nil {
		return nil
	}
	reason := strings.TrimSpace(o.Reason)
	if !IsRefused(scanErr) {
		// Not a compiled-in refusal (an unverifiable, a rate limit) — nothing to override.
		return scanErr
	}
	if strings.Contains(scanErr.Error(), impersonationMarker) {
		if reason != "" {
			return Refused("refused: --" + ScanOverrideFlag + " does not apply to the " +
				"impersonation guard — that refusal is about WHOSE VOICE the text is in, " +
				"not about a credential shape, and rewording it is always available. " +
				"Original refusal: " + scanErr.Error())
		}
		return scanErr
	}
	if reason == "" {
		return Refused(scanErr.Error() + OverrideHint())
	}
	o.Refusal = scanErr.Error()
	return LogScanOverride(o)
}

// OverrideHint is the sentence the scan refusals append so the refusal itself advertises
// the audited way through. It is a single leading space plus one sentence, so callers can
// concatenate it onto an existing message without reformatting.
func OverrideHint() string {
	return " If this is a false positive, re-run with --" + ScanOverrideFlag +
		" \"<why>\" — the write proceeds and an audit row records the tool, the surface " +
		"digest, your reason and your identity. Exit 5 without that flag remains a STOP: " +
		"never route around it with raw git/gh."
}
