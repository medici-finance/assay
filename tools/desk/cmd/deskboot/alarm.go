package main

import (
	"fmt"
	"os"
	"path/filepath"
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
// files ONE `to:desk` issue quoting the red line.
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
// filesystem write so "exactly one payload per role per day" is a checked property.

// fileRedPreflightAlarm is the seam production binds to doFileRedPreflightAlarm. It files (or,
// on a repeat within the day, no-ops against its own marker) the to:desk alarm for a red
// envelope and reports whether a payload was produced this call. It is a package var ONLY so a
// future test can inject a recording fake; the shipped path is exercised through execCommand.
var fileRedPreflightAlarm = doFileRedPreflightAlarm

// doFileRedPreflightAlarm is the production alarm filer. It returns (true, nil) when it
// produced a filing this call, (false, nil) when the marker shows this role was already
// alarmed today, and (false, err) when it could not file at all.
func doFileRedPreflightAlarm(o bootOpts, tokenRole, summary string) (bool, error) {
	marker, err := alarmMarkerPath(tokenRole)
	if err != nil {
		return false, err
	}
	// Deduped per role per day: a marker present means this role's envelope was already
	// alarmed today, so a re-booting loop does not re-file the same issue every tick.
	if _, statErr := os.Stat(marker); statErr == nil {
		return false, nil
	}

	repo, err := o.resolveRepo()
	if err != nil {
		return false, fmt.Errorf("cannot resolve a repo to file the alarm in: %w", err)
	}

	day := time.Now().Format("2006-01-02")
	title := fmt.Sprintf("deskboot: red operating-envelope preflight (role %s, %s)", tokenRole, day)
	body := fmt.Sprintf(
		"deskboot's operating-envelope preflight came back RED for role `%s` and the boot stopped "+
			"(could-not-run for the whole pass — nothing was claimed). A red envelope otherwise "+
			"reaches no one but the window it printed to, so deskboot files this one alarm, addressed "+
			"to the desk, deduped by a marker per role per day.\n\n"+
			"The roster's own verdict, verbatim:\n\n```\n%s\n```\n\n"+
			"Each failing check names its own remediation in the line above. Fix the envelope, then "+
			"re-run deskboot for role `%s`.\n",
		tokenRole, summary, tokenRole)

	bodyFile, err := os.CreateTemp("", "deskboot-preflight-alarm-*.md")
	if err != nil {
		return false, fmt.Errorf("cannot create the alarm body file: %w", err)
	}
	defer os.Remove(bodyFile.Name())
	if _, err := bodyFile.WriteString(body); err != nil {
		bodyFile.Close()
		return false, fmt.Errorf("cannot write the alarm body file: %w", err)
	}
	if err := bodyFile.Close(); err != nil {
		return false, fmt.Errorf("cannot close the alarm body file: %w", err)
	}

	r := runCmd("", "deskfile", "new", "-R", repo, "--title", title, "--body-file", bodyFile.Name(),
		"--to", "desk", "--raised-by", "desk", "--label", "help wanted")
	if r.err != nil {
		// deskfile's OWN title-keyed dedupe found an existing filing for this title and came
		// back exit 5 ("already filed") — that IS the idempotent no-op this alarm wants, so
		// treat it as filed and drop the marker so this session stops re-trying too.
		if deskkit.ExitCodeOf(r.err) != deskkit.ExitRefused {
			return false, fmt.Errorf("deskfile new failed: %s", firstLine(nonEmpty(r.stderr, r.stdout)))
		}
	}

	// Filing succeeded (or deduped at deskfile): drop the marker so a second red today files
	// no second payload. A marker-write failure does not undo the filing — the payload is out,
	// so report success and let the next tick's deskfile dedupe catch a re-file.
	if werr := writeAlarmMarker(marker, title); werr != nil {
		fmt.Fprintf(os.Stderr, "deskboot: WARNING: alarm filed but its dedupe marker could not be written (%v) — "+
			"a re-boot today may re-file; deskfile's own title dedupe still applies\n", werr)
	}
	return true, nil
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
