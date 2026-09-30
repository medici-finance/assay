package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// alarm.go — the red-preflight ALARM (the credfence loop of this stream's balancing-loops brief).
//
// The operating-envelope preflight (boot step 5) has always STOPPED the boot on a red
// envelope and printed the roster's own summary. What it did NOT do was reach anyone but the
// window it printed to: a red envelope on a headless loop, a pod, or a session no human is
// watching went by unheard, tick after tick. This is the shape of a whole failure class — a
// sensor with no actuator — so a red preflight is now ALARMED as well as printed: deskboot
// files ONE `to:desk` issue naming each failing check and its state. The check DETAILS stay
// local (deskkit.PreflightAlarmBody): the issue may land in a public repository.
//
// Two properties are load-bearing:
//
//   - IT NEVER CHANGES THE VERDICT. A red preflight is could-not-run for the whole pass,
//     exactly as before; the alarm is a side effect on the way to the SAME refusal. A filing
//     failure is a stderr warning, never a reason the boot reports something other than red.
//   - IT IS DEDUPED PER ROLE PER DAY. A marker under the state directory records that this
//     role was alarmed today, so a loop that boots, refuses, and re-boots on a five-minute
//     supervisor does not file the same envelope issue every tick. The marker is the desk's
//     own dedupe; deskfile's title-keyed dedupe is a second, independent layer behind it.
//
// The filing goes through the same execCommand seam every other child process does, so a test
// asserts the `deskfile new --to desk` argv without a real forge, and the marker is a real
// filesystem write so "exactly one payload per role per day" is a checked property. The
// body's acceptance by deskfile's blocker-evidence gate is checked on deskfile's side, where
// the gate lives (deskfile's TestPreflightAlarmBodyPassesBlockerEvidenceGate).

// alarmOutcome is what one alarm attempt did. It is reported, never inferred: the boot's
// refusal text says which of these happened, so "deskboot filed an alarm" is printed only when
// a filing was actually accepted.
type alarmOutcome int

const (
	// alarmNotFiled — no filing was accepted (the error says why). Never writes the marker.
	alarmNotFiled alarmOutcome = iota
	// alarmFiled — deskfile accepted a new filing this call.
	alarmFiled
	// alarmDuplicate — deskfile's title dedupe found an equivalent filing already open.
	alarmDuplicate
	// alarmAlreadyToday — this role's marker shows it was alarmed today; nothing was sent.
	alarmAlreadyToday
)

// fileRedPreflightAlarm is the seam production binds to doFileRedPreflightAlarm. It is a
// package var ONLY so a future test can inject a recording fake; the shipped path is
// exercised through execCommand.
var fileRedPreflightAlarm = doFileRedPreflightAlarm

// doFileRedPreflightAlarm is the production alarm filer. rosterOutput is the red roster
// run's output; ONLY its closed-vocabulary check names and states reach the issue
// (deskkit.PreflightAlarmBody) — never a check's detail or remediation, which name local
// specifics and may be filed into a public repository.
//
// The marker is written ONLY when an equivalent issue is known to exist afterwards: a filing
// deskfile accepted, or deskfile's title-dedupe refusal (deskkit.DedupeRefusalPrefix). Any
// other refusal — the blocker-evidence gate, the filing budget, an unbound role, a scan — is
// an error and leaves no marker, so the next red boot tries again instead of a day-long
// silence that reads as "alarmed".
func doFileRedPreflightAlarm(o bootOpts, tokenRole, rosterOutput string) (alarmOutcome, error) {
	marker, err := alarmMarkerPath(tokenRole)
	if err != nil {
		return alarmNotFiled, err
	}
	// Deduped per role per day: a marker present means this role's envelope was already
	// alarmed today, so a re-booting loop does not re-file the same issue every tick.
	if _, statErr := os.Stat(marker); statErr == nil {
		return alarmAlreadyToday, nil
	}

	repo, err := o.resolveRepo()
	if err != nil {
		return alarmNotFiled, fmt.Errorf("cannot resolve a repo to file the alarm in: %w", err)
	}

	title := deskkit.PreflightAlarmTitle(tokenRole, time.Now().Format("2006-01-02"))
	body := deskkit.PreflightAlarmBody(tokenRole, rosterOutput)

	bodyFile, err := os.CreateTemp("", "deskboot-preflight-alarm-*.md")
	if err != nil {
		return alarmNotFiled, fmt.Errorf("cannot create the alarm body file: %w", err)
	}
	defer os.Remove(bodyFile.Name())
	if _, err := bodyFile.WriteString(body); err != nil {
		bodyFile.Close()
		return alarmNotFiled, fmt.Errorf("cannot write the alarm body file: %w", err)
	}
	if err := bodyFile.Close(); err != nil {
		return alarmNotFiled, fmt.Errorf("cannot close the alarm body file: %w", err)
	}

	// Raised BY the booting role (its provenance), addressed TO the desk (its inbox). The
	// write itself runs under the session's own minted identity; --raised-by is attribution,
	// so it names the role that actually hit the red envelope.
	r := runCmd("", "deskfile", "new", "-R", repo, "--title", title, "--body-file", bodyFile.Name(),
		"--to", "desk", "--raised-by", tokenRole, "--label", deskkit.PreflightAlarmLabel)
	outcome := alarmFiled
	if r.err != nil {
		// The CHILD's exit status, read off the process error. deskkit.ExitCodeOf reads a
		// DeskError and maps every other error — an *exec.ExitError included — to 6, so it
		// can never see deskfile's exit 5.
		code := childExitCode(r.err)
		out := r.stderr + "\n" + r.stdout
		if code != deskkit.ExitRefused || !strings.Contains(out, deskkit.DedupeRefusalPrefix) {
			return alarmNotFiled, fmt.Errorf("deskfile new did not file the alarm (exit %d): %s",
				code, firstLine(refusalLine(r.stderr, r.stdout)))
		}
		// deskfile's OWN title-keyed dedupe found an existing filing for this title — that
		// IS the idempotent no-op this alarm wants.
		outcome = alarmDuplicate
	}

	// An equivalent issue now exists: drop the marker so a second red today files no second
	// payload. A marker-write failure does not undo the filing — the payload is out, so
	// report it and let the next tick's deskfile dedupe catch a re-file.
	if werr := writeAlarmMarker(marker, title); werr != nil {
		fmt.Fprintf(os.Stderr, "deskboot: WARNING: alarm filed but its dedupe marker could not be written (%v) — "+
			"a re-boot today may re-file; deskfile's own title dedupe still applies\n", werr)
	}
	return outcome, nil
}

// alarmSentence is the one sentence the red-preflight refusal carries about the alarm. It
// states what happened on THIS boot, so an operator never reads "filed" when nothing was.
func alarmSentence(outcome alarmOutcome, err error) string {
	switch {
	case err != nil:
		return "deskboot could NOT file the to:desk alarm (see the WARNING above), so this red envelope " +
			"is heard only by this window — raise it with the desk by hand."
	case outcome == alarmFiled:
		return "deskboot filed ONE to:desk alarm naming each failing check (deduped per role per day) so a " +
			"red envelope is not heard only by this window."
	case outcome == alarmDuplicate:
		return "An equivalent to:desk alarm is already open (deskfile's title dedupe), so none was filed again."
	case outcome == alarmAlreadyToday:
		return "This role's to:desk alarm was already filed today (deduped per role per day), so none was filed again."
	default:
		return "No to:desk alarm was filed."
	}
}

// childExitCode is a finished child process's exit status, or -1 when the error is not an
// exit status at all (the binary could not be started).
func childExitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// refusalLine picks the line of a child's output that carries its verdict: the first line
// holding "refused" (deskfile prints its config banner first), else the first non-empty
// stream.
func refusalLine(stderr, stdout string) string {
	for _, s := range []string{stderr, stdout} {
		for _, ln := range strings.Split(s, "\n") {
			if strings.Contains(ln, "refused") {
				return strings.TrimSpace(ln)
			}
		}
	}
	return nonEmpty(stderr, stdout)
}

// alarmMarkerPath is the per-role, per-day dedupe marker: <StateDir>/deskboot-alarm.<role>.<YYYYMMDD>.
// Keyed on the token ROLE (not the loop) and the calendar day, so exactly one alarm per role
// per day survives a loop that re-boots on a short supervisor interval.
func alarmMarkerPath(tokenRole string) (string, error) {
	dir, err := deskkit.StateDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve the state directory for the alarm marker: %w", err)
	}
	name := fmt.Sprintf("deskboot-alarm.%s.%s", tokenRole, time.Now().Format("20060102"))
	return filepath.Join(dir, name), nil
}

// writeAlarmMarker records that the role was alarmed today. The marker's CONTENT is the issue
// title it filed, so an operator reading the state dir can see what was alarmed, not just that
// something was.
func writeAlarmMarker(marker, title string) error {
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(title+"\n"), 0o600)
}

// nonEmpty returns the first non-empty of its arguments, or "" — so a diagnosis prefers a
// tool's stderr but falls back to its stdout rather than reporting nothing.
func nonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
