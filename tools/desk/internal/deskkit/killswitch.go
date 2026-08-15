package deskkit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	disabledEnv = "DESK_TOOLS_DISABLED"
	loopEnv     = "DESK_LOOP"
)

// dirOverride, when non-empty, replaces the real ~/.config/assay directory.
// It is a TEST HOOK only (white-box tests set it to a temp dir). It is deliberately
// NOT wired to any env var or flag: the kill-switch file AND the audit log live in
// this directory, so an operator-relocatable path would let a caller move — and thus
// bypass — the kill switch. Production always resolves it from $HOME.
var dirOverride string

// heartbeatStaleDuration is the age after which a HEARTBEAT file is considered stale
// and treated as a STOP-ALL. Exported so deskboard can report it and tests can verify.
var HeartbeatStaleDuration = 24 * time.Hour

func deskDir() (string, error) {
	if dirOverride != "" {
		return dirOverride, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "assay"), nil
}

// StateDir returns the desk-tools state directory (~/.config/assay), the same
// directory the kill switch, audit log, and loop flags live in. It is exported so
// desk tools can place their OWN state — read-tool snapshots (deskquiet),
// loop-heartbeat files — alongside the shared state they already depend on, reusing
// the one resolution path (and its dirOverride test hook) rather than a second one.
// The directory may not exist yet; callers MkdirAll their own subpaths. A missing
// HOME is Unverifiable — the tool cannot place state it cannot locate.
func StateDir() (string, error) {
	return deskDir()
}

func disabledFilePath() (string, error) {
	dir, err := deskDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "DISABLED"), nil
}

// killSwitchState reports whether the kill switch is armed and a human-readable
// reason. env DESK_TOOLS_DISABLED=1 arms it; otherwise the presence of the DISABLED
// file arms it (its first line is the reason). A DISABLED file that exists but cannot
// be read is an error (fail closed) — the caller must NOT proceed.
func killSwitchState() (armed bool, reason string, err error) {
	if os.Getenv(disabledEnv) == "1" {
		return true, disabledEnv + "=1", nil
	}
	path, perr := disabledFilePath()
	if perr != nil {
		return false, "", perr
	}
	b, rerr := os.ReadFile(path)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return false, "", nil
		}
		return false, "", rerr
	}
	reason = firstLine(string(b))
	if reason == "" {
		reason = "kill switch file present"
	}
	return true, reason, nil
}

// stopFlagState checks the loop stop-flags: STOP (all loops), then STOP.<loop> (per-loop
// when DESK_LOOP is set). Returns armed=true and the flag filename as the reason.
// A flag file that exists but cannot be read is an error (fail closed).
//
// Precedence is preserved exactly as documented (DISABLED > STOP > STOP.<name>): STOP is
// resolved BEFORE the loop name is even looked at, so an all-loops halt still wins over
// an unrecognised loop name — a mis-spelled DESK_LOOP can never mask a STOP.
//
// An unrecognised DESK_LOOP is Unverifiable (exit 6), NOT "no flag held": the tool cannot
// establish which flag file would speak for this loop, so it cannot report clean. See
// loopnames.go for why exit 6 rather than exit 5, and for how the blast radius is bounded
// to the mis-named session alone.
func stopFlagState() (armed bool, reason string, err error) {
	dir, derr := deskDir()
	if derr != nil {
		return false, "", derr
	}

	// Check STOP (all-loops halt).
	stopAll, rsn, serr := flagFileState(dir, "STOP")
	if serr != nil {
		return false, "", serr
	}
	if stopAll {
		return true, rsn, nil
	}

	// Check STOP.<loop> when DESK_LOOP is set.
	loopName := os.Getenv(loopEnv)
	if loopName == "" {
		return false, "", nil
	}

	flagNames, known := LoopFlagNames(loopName)
	if !known {
		return false, "", Unverifiable(fmt.Sprintf(
			"per-loop stop flag unverifiable: %s=%q names no known loop, so no STOP.<name> "+
				"file can be checked for it — this is could-not-check, NOT 'no stop flag held'. "+
				"Known loop names: %s. Fix the spelling, or register the loop in "+
				"tools/desk/internal/deskkit/loopnames.go",
			loopEnv, loopName, strings.Join(KnownLoopNames(), ", ")), nil)
	}

	// Any flag in the loop's name class arms it, so a rename cannot orphan a held flag.
	for _, name := range flagNames {
		flagArmed, flagReason, ferr := flagFileState(dir, "STOP."+name)
		if ferr != nil {
			return false, "", ferr
		}
		if flagArmed {
			return true, flagReason, nil
		}
	}
	return false, "", nil
}

// heartbeatState checks the HEARTBEAT dead-man lease. If the file exists and its mtime
// is older than HeartbeatStaleDuration, it is treated as STOP-ALL. Absent file = feature
// off. A file that exists but cannot be stat'd is an error (fail closed).
func heartbeatState() (armed bool, reason string, err error) {
	dir, derr := deskDir()
	if derr != nil {
		return false, "", derr
	}
	path := filepath.Join(dir, "HEARTBEAT")
	fi, serr := os.Stat(path)
	if serr != nil {
		if os.IsNotExist(serr) {
			return false, "", nil // feature off
		}
		return false, "", serr // unreadable → fail closed
	}
	age := time.Since(fi.ModTime())
	if age > HeartbeatStaleDuration {
		return true, fmt.Sprintf("HEARTBEAT stale (age %s > %s)", age.Truncate(time.Second), HeartbeatStaleDuration), nil
	}
	return false, "", nil
}

// flagFileState checks whether the named flag file exists. If so, armed=true with the
// flag filename as reason. Absent → not armed. Unreadable → error (fail closed).
func flagFileState(dir, name string) (armed bool, reason string, err error) {
	path := filepath.Join(dir, name)
	b, rerr := os.ReadFile(path)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return false, "", nil
		}
		return false, "", rerr
	}
	rsn := strings.TrimSpace(string(b))
	if rsn != "" {
		rsn = name + ": " + rsn
	} else {
		rsn = name + " (empty reason)"
	}
	rsn = firstLine(rsn)
	return true, rsn, nil
}

// ActiveStopFlag represents an active stop flag found in the desk-tools directory.
// It is used by deskboard to banner visible stop conditions.
type ActiveStopFlag struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
	Stale  bool   `json:"stale,omitempty"` // true only for stale HEARTBEAT
	// Inert marks a STOP.<name> flag whose <name> matches no known loop (see
	// loopnames.go). Such a flag halts NOTHING — it is a human believing they hold a
	// stop that no loop will ever read. That belief is the human-side half of the same
	// silent failure the Guard fix closes, so the board must say so rather than list
	// the file alongside flags that really are armed.
	Inert bool `json:"inert,omitempty"`
}

// ActiveStopFlags returns the list of currently active stop flags (STOP, STOP.<name>,
// stale HEARTBEAT). DISABLED is NOT included — it is the broader kill switch and has
// its own banner. This is a best-effort diagnostic for deskboard; a read error on the
// flag dir returns nil (the board run itself would have hit that error via Guard()).
func ActiveStopFlags() []ActiveStopFlag {
	dir, err := deskDir()
	if err != nil {
		return nil
	}
	var flags []ActiveStopFlag

	// STOP
	if r, ok := readFlagReason(dir, "STOP"); ok {
		flags = append(flags, ActiveStopFlag{Name: "STOP", Reason: r})
	}

	// STOP.<loop> — enumerate by reading directory
	entries, rerr := os.ReadDir(dir)
	if rerr == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			n := e.Name()
			if strings.HasPrefix(n, "STOP.") && n != "STOP" {
				if r, ok := readFlagReason(dir, n); ok {
					loop := strings.TrimPrefix(n, "STOP.")
					inert := !IsKnownLoopName(loop)
					if inert {
						r = r + " — INERT: " + loop + " matches no known loop, so this flag halts nothing"
					}
					flags = append(flags, ActiveStopFlag{Name: n, Reason: r, Inert: inert})
				}
			}
		}
	}

	// HEARTBEAT staleness
	armed, reason, err := heartbeatState()
	if err == nil && armed {
		flags = append(flags, ActiveStopFlag{Name: "HEARTBEAT", Reason: reason, Stale: true})
	}

	return flags
}

// readFlagReason reads the first line of the flag file as its reason, or an empty-string
// fallback. ok=false means file does not exist or is unreadable.
func readFlagReason(dir, name string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", false
	}
	rsn := firstLine(strings.TrimSpace(string(b)))
	if rsn == "" {
		rsn = "present (no reason)"
	}
	return rsn, true
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// Guard is the MANDATORY first call of every desk tool. It checks all
// stop conditions in precedence order BEFORE any other action — including reads:
//  1. DISABLED (kill switch)
//  2. STOP (all-loops halt)
//  3. STOP.<loop> (per-loop halt when DESK_LOOP set)
//  4. HEARTBEAT staleness (dead-man lease >24h)
//
// Behaviour on armed: append an audit line (result=disabled) and return a *DeskError
// mapping to exit 3 with detail naming the exact flag. On unverifiable state → exit 6
// (fail-closed). On disarm transition → record it once.
func Guard() error {
	return guard(toolName())
}

func guard(tool string) error {
	// 1. DISABLED (kill switch — broadest precedence)
	armed, reason, err := killSwitchState()
	if err != nil {
		return Unverifiable("kill-switch state unverifiable (DISABLED file unreadable / HOME missing)", err)
	}
	if armed {
		writeDisabledAudit(tool, reason)
		return Disabled("refused: desk tools disabled (result=disabled) — " + reason)
	}

	// 2. STOP (all-loops halt)
	armed, reason, err = stopFlagState()
	if err != nil {
		// An already-typed refusal states its own cause precisely (e.g. an
		// unrecognised DESK_LOOP); re-wrapping it would bury that under a
		// "flag dir unreadable" that is not what happened.
		if IsUnverifiable(err) {
			return err
		}
		return Unverifiable("stop-flag state unverifiable (flag dir unreadable)", err)
	}
	if armed {
		writeDisabledAudit(tool, reason)
		return Disabled("refused: stop flag active (result=disabled) — " + reason)
	}

	// 3. HEARTBEAT staleness (dead-man lease)
	armed, reason, err = heartbeatState()
	if err != nil {
		return Unverifiable("heartbeat state unverifiable (flag dir unreadable)", err)
	}
	if armed {
		writeDisabledAudit(tool, reason)
		return Disabled("refused: heartbeat stale (result=disabled) — " + reason)
	}

	// Disarm transition recording (DISABLED transitions only).
	if lastResultWas(ResultDisabled) {
		_ = Log(Entry{Tool: tool, Verb: "guard", Result: ResultOK, Detail: "kill-switch disarmed (DISABLED cleared)"})
	}
	return nil
}

func writeDisabledAudit(tool, reason string) {
	if err := Log(Entry{Tool: tool, Verb: "guard", Result: ResultDisabled, Detail: reason}); err != nil {
		// The kill switch still wins even if we cannot record it; make the failure
		// loud rather than swallow it.
		fmt.Fprintf(os.Stderr, "desk-tools: WARNING: could not write disabled audit line: %v\n", err)
	}
}

// lastResultWas reports whether the most recent audit line has the given result.
// Transition detection is best-effort: a corrupt/unreadable file returns false here
// (the outward-write flow surfaces corruption via LoadEntries/AllowWrite as exit 6).
func lastResultWas(result string) bool {
	entries, err := LoadEntries()
	if err != nil || len(entries) == 0 {
		return false
	}
	return entries[len(entries)-1].Result == result
}

// toolName derives the tool name for audit lines from the running binary
// (/opt/desk-tools/bin/<tool>). Under `go test` this is the test binary name, which
// is fine for the audit field.
func toolName() string {
	if len(os.Args) == 0 {
		return "desk"
	}
	return filepath.Base(os.Args[0])
}
