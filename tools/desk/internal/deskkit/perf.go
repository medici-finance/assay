package deskkit

// perf.go — the opt-in, LOCAL-ONLY per-invocation performance record.
//
// The audit ledger (audit.go) records WHAT a verb did; nothing recorded how long it took or
// what it cost. This file adds that as a SEPARATE dataset rather than as a field on the
// ledger's Entry: the ledger is load-bearing state (ratelimit.go counts its rows,
// idempotent.go reads them), so its schema stays exactly as it is, and the perf history is
// the opposite in every respect — opt-in, pruned to 7 UTC days on write, and read by no
// control at all.
//
// The contract, each clause enforced by a test rather than promised:
//
//   - OFF BY DEFAULT. PerfEnabled is ASSAY_TELEMETRY == "1" exactly. Off, PerfRun calls the
//     verb and returns, and nothing is created: no directory, no day file, no install id.
//   - NO FREE TEXT. PerfRecord has six string fields and each is a constant, a compiled-in
//     roster member, a build stamp or random hex. tool, verb, role and result each pass
//     through a mapper whose return is one of its own constants, never any part of its input.
//   - LOCAL ONLY. Nothing here opens a socket; there is no sender and no endpoint. The forge
//     round-trip COUNTER lives beside the transport seam (forgetransport.go) so this file
//     needs no network import at all.
//   - NEVER LOAD-BEARING. perfAppend returns nothing. A failure to resolve, create, open or
//     write is swallowed: the record never changes a verb's exit code, never prints, never
//     blocks, and takes no lock (one O_APPEND write of one short line).
//   - NEVER THE LEDGER. Every path joins perfDir() — a perf/ SUBDIRECTORY of the state dir —
//     so no writer, pruner or reader here can name audit.jsonl or one of its day segments.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// PerfSchema is the on-disk record version. Adding ANY field bumps it (perf-v2), updates the
// field table in docs/telemetry.md, and adds a case to the no-identifier test — all three in
// the same change.
const PerfSchema = "perf-v1"

// perfEnvVar is the one opt-in. It is the SAME variable statusgen/telemetry.go reads.
const perfEnvVar = "ASSAY_TELEMETRY"

// perfRetentionDays is the history window: today plus the six prior UTC days.
const perfRetentionDays = 7

// perfOther / perfRoleNone are the mappers' fallbacks — constants, never the input.
const (
	perfOther    = "other"
	perfRoleNone = "none"
	perfStepPfx  = "step:"
	perfUnpinned = "unpinned"
)

// PerfRecord is one perf-v1 line. Exactly these fields and no others; cpuMs alone is
// omitted when unmeasurable, so "not measured" and "used no CPU" stay distinguishable.
type PerfRecord struct {
	Schema      string `json:"schema"`
	TS          string `json:"ts"`
	Tool        string `json:"tool"`
	Verb        string `json:"verb"`
	Role        string `json:"role"`
	WallMs      int64  `json:"wallMs"`
	CPUMs       *int64 `json:"cpuMs,omitempty"`
	Exit        int    `json:"exit"`
	Result      string `json:"result"`
	ChildProcs  int64  `json:"childProcs"`
	ChildWallMs int64  `json:"childWallMs"`
	ForgeCalls  int64  `json:"forgeCalls"`
	SourceSHA   string `json:"sourceSHA"`
	Install     string `json:"install"`
}

// PerfEnabled reports whether local perf logging is armed: ASSAY_TELEMETRY compared to "1"
// EXACTLY — not a truthiness test. It must not drift from statusgen/telemetry.go's
// telemetryArmed, whose environment half makes the identical comparison, so one opt-in
// means the same thing to both tools.
//
// There is deliberately no second switch for this LOCAL file. docs/telemetry.md's two-switch
// rule exists so that no inherited environment can cause a SEND; this log sends nothing —
// it is a file on the user's own disk. What the single switch still buys is consent: a user
// who never opted in gets no new directory, file or identifier. Any future REMOTE sink for
// these records is bound by the two-switch rule in full.
func PerfEnabled() bool { return os.Getenv(perfEnvVar) == "1" }

// The in-process counters PerfRun reads. They are incremented unconditionally (an atomic add
// is cheaper than the branch that would skip it) and reset at PerfRun entry; with the switch
// off they are never read and never leave memory.
var (
	perfChildProcs  atomic.Int64
	perfChildWallNs atomic.Int64
	perfForgeCalls  atomic.Int64
)

// perfCountChild records one child started through deskkit.Run and its wall time.
func perfCountChild(d time.Duration) {
	perfChildProcs.Add(1)
	perfChildWallNs.Add(int64(d))
}

// perfCountForgeCall records one HTTP round-trip through the counting forge transport.
func perfCountForgeCall() { perfForgeCalls.Add(1) }

// perfResetCounters zeroes all three counters (PerfRun entry).
func perfResetCounters() {
	perfChildProcs.Store(0)
	perfChildWallNs.Store(0)
	perfForgeCalls.Store(0)
}

// PerfCounters returns the current child count, summed child wall time and forge round-trips.
func PerfCounters() (children int64, childWall time.Duration, forgeCalls int64) {
	return perfChildProcs.Load(), time.Duration(perfChildWallNs.Load()), perfForgeCalls.Load()
}

// perfNow is the clock (a test hook; production is time.Now).
var perfNow = time.Now

// PerfRun is the ONE wrapper a verb's main() puts around its run function:
//
//	os.Exit(deskkit.PerfRun("<tool>", verbs, os.Args[1:], run))
//
// It must wrap run rather than defer inside it, because os.Exit runs no deferred function.
// Off, it calls run and returns. Armed, it times run, builds the record (verb resolved by
// matching args[0] against verbs — the verb's own registered subcommand set — and "other"
// for anything else), appends it best-effort, and returns run's exit code UNCHANGED.
func PerfRun(tool string, verbs []string, args []string, run func([]string) int) int {
	if !PerfEnabled() {
		return run(args)
	}
	perfResetCounters()
	began := time.Now()
	code := run(args)
	wall := time.Since(began)

	rec := newPerfRecord(tool, perfVerb(verbs, args), wall, code)
	children, childWall, forge := PerfCounters()
	rec.ChildProcs = children
	rec.ChildWallMs = childWall.Milliseconds()
	rec.ForgeCalls = forge
	if ms, ok := processCPUMillis(); ok {
		rec.CPUMs = &ms
	}
	perfAppend(rec)
	return code
}

// perfBootSteps is the compiled roster of boot-step names PerfStep accepts — deskboot's own
// step constants, pinned equal to its step list by that package's tests. A step outside it is
// recorded as "step:other", so a caller can never route text into the verb field.
var perfBootSteps = []string{
	"loop-identity",
	"worktree-prune",
	"worktree-lock",
	"roster-set",
	"roster-preflight",
	"token-mint",
	"board-fetch",
	"worktree-current",
}

// PerfBootSteps returns a copy of the step-name roster, in boot order.
func PerfBootSteps() []string { return append([]string(nil), perfBootSteps...) }

// PerfStep writes one boot-step record: verb "step:<name>", wallMs the step's duration,
// exit the step's exit code. Child and forge counts are not attributed per step (they stay
// 0) and cpuMs is omitted — a process-wide CPU reading is not a per-step measurement.
func PerfStep(tool, step string, d time.Duration, exit int) {
	if !PerfEnabled() {
		return
	}
	perfAppend(newPerfRecord(tool, perfStepVerb(step), d, exit))
}

// perfStepVerb returns "step:" plus the ROSTER's spelling of step, or "step:other".
func perfStepVerb(step string) string {
	for _, s := range perfBootSteps {
		if s == step {
			return perfStepPfx + s
		}
	}
	return perfStepPfx + perfOther
}

// newPerfRecord builds a record through the four mappers. verb must already be a roster
// value (perfVerb / the step roster); every other string field is mapped here.
func newPerfRecord(tool, verb string, wall time.Duration, exit int) PerfRecord {
	return PerfRecord{
		Schema:    PerfSchema,
		TS:        perfNow().UTC().Truncate(time.Second).Format(time.RFC3339),
		Tool:      perfTool(tool),
		Verb:      verb,
		Role:      perfRole(os.Getenv("DESK_LOOP")),
		WallMs:    wall.Milliseconds(),
		Exit:      exit,
		Result:    resultForExit(exit),
		SourceSHA: perfSourceSHA(),
	}
}

// perfTool maps a tool name onto the canonical tool roster, or "other".
func perfTool(raw string) string {
	if c, ok := CanonicalToolKey(raw); ok {
		if _, member := canonicalToolKeys[c]; member {
			return c
		}
	}
	return perfOther
}

// perfVerb returns the element of verbs that args[0] names, or "other". It returns the
// ROSTER's string, never args[0] itself.
func perfVerb(verbs, args []string) string {
	if len(args) == 0 {
		return perfOther
	}
	for _, v := range verbs {
		if v != "" && args[0] == v {
			return v
		}
	}
	return perfOther
}

// perfRole maps $DESK_LOOP onto the loop roster, or "none" when unset or unknown.
func perfRole(raw string) string {
	if c, ok := CanonicalLoopName(strings.TrimSpace(raw)); ok {
		if _, member := canonicalLoopNames[c]; member {
			return c
		}
	}
	return perfRoleNone
}

// resultForExit is the fixed exit→result map onto the audit Result* constants. It returns
// one of its own constants, never its input; it is never read from the audit log.
func resultForExit(exit int) string {
	switch exit {
	case ExitOK:
		return ResultOK
	case ExitDisabled:
		return ResultDisabled
	case ExitRefused:
		return ResultRefused
	case ExitUnverifiable:
		return ResultUnverifiable
	default:
		return ResultUnverifiable
	}
}

// shaPattern is what a build stamp looks like; anything else records as "unpinned".
var shaPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// perfSourceSHA is Version()'s build stamp, constrained to hex so even a mis-set ldflag
// cannot carry text into the record.
func perfSourceSHA() string {
	s, _ := Version()
	if shaPattern.MatchString(s) {
		return s
	}
	return perfUnpinned
}

// perfDir is <state dir>/perf. Every perf path joins it, so none can name audit.jsonl.
func perfDir() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "perf"), nil
}

// perfDayFile matches a day file name EXACTLY and captures its date. The pruner and the
// reader touch nothing else, and both parse the date out of the MATCHED name.
var perfDayFile = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})\.jsonl$`)

// perfDayOf returns the UTC day a day-file name records, ok=false for any other name.
func perfDayOf(name string) (time.Time, bool) {
	m := perfDayFile.FindStringSubmatch(name)
	if m == nil {
		return time.Time{}, false
	}
	day, err := time.Parse("2006-01-02", m[1])
	if err != nil {
		return time.Time{}, false
	}
	return day, true
}

// perfAppend writes rec as one line to today's day file, best-effort. It returns nothing:
// the perf log is never allowed to change what the verb does.
func perfAppend(rec PerfRecord) {
	_ = perfWrite(rec)
}

// perfWrite is perfAppend's body with the error kept, for the tests that need to see WHY a
// write did not land. Production callers go through perfAppend, which discards it.
func perfWrite(rec PerfRecord) error {
	dir, err := perfDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	perfPruneOnce(dir)
	id, err := perfInstallID(dir)
	if err != nil {
		return err
	}
	rec.Install = id
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	day := perfNow().UTC().Format("2006-01-02") + ".jsonl"
	f, err := os.OpenFile(filepath.Join(dir, day), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(line)
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

// perfPrunedDir records the directory this process last pruned, so the pruner runs on the
// first append of a process (per directory, which keeps tests that swap the state dir honest).
var (
	perfPruneMu   sync.Mutex
	perfPrunedDir string
)

func perfPruneOnce(dir string) {
	perfPruneMu.Lock()
	defer perfPruneMu.Unlock()
	if perfPrunedDir == dir {
		return
	}
	perfPrunedDir = dir
	perfPrune(dir)
}

// perfPrune removes day files older than today-6 (UTC). It lists ONLY dir, matches names
// against perfDayFile EXACTLY, parses the date out of the matched name, and removes nothing
// else: install-id and any name a future change adds survive. A concurrent pruner's ENOENT
// is ignored.
func perfPrune(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := perfCutoff()
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() {
			continue
		}
		day, ok := perfDayOf(name)
		if !ok {
			continue
		}
		if day.Before(cutoff) {
			if rerr := os.Remove(filepath.Join(dir, name)); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
				continue
			}
		}
	}
}

// perfCutoff is the oldest UTC day kept: today minus (retention-1) days, at midnight.
func perfCutoff() time.Time {
	now := perfNow().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -(perfRetentionDays - 1))
}

// installIDPattern is the only shape an install id may have: 32 lowercase hex characters.
var installIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// perfInstallID reads <dir>/install-id, minting it (16 bytes of crypto/rand, hex) when
// missing or malformed. It is an identifier of the INSTALL, never derived from any machine
// or user property; deleting the file simply mints a new one.
func perfInstallID(dir string) (string, error) {
	p := filepath.Join(dir, "install-id")
	if b, err := os.ReadFile(p); err == nil {
		if id := strings.TrimSpace(string(b)); installIDPattern.MatchString(id) {
			return id, nil
		}
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(raw[:])
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		// Either a concurrent first write won the race (use its id) or the file is malformed
		// (replace it).
		if b, rerr := os.ReadFile(p); rerr == nil {
			if got := strings.TrimSpace(string(b)); installIDPattern.MatchString(got) {
				return got, nil
			}
		}
		if werr := os.WriteFile(p, []byte(id+"\n"), 0o600); werr != nil {
			return "", werr
		}
		return id, nil
	}
	if err != nil {
		return "", err
	}
	_, werr := f.WriteString(id + "\n")
	cerr := f.Close()
	if werr != nil {
		return "", werr
	}
	if cerr != nil {
		return "", cerr
	}
	return id, nil
}
