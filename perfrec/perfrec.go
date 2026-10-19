// Package perfrec is the one writer of the opt-in, LOCAL-ONLY per-invocation performance
// record. Every tool that records one — the desk tools through their shared kit, statusgen
// directly — calls this package, so there is exactly one implementation of the schemas, the
// day-file layout, the 7-day retention and the install id.
//
// Two schemas share the day files (schema.go): the legacy perf-v1 line New builds, whose
// wallMs starts wherever its caller started a timer and whose cpuMs is process-lifetime
// CPU; and perf-v2, built only by a Session (session.go), whose wall and CPU are deltas over
// ONE declared boundary and whose counters carry explicit measured/unavailable coverage. A
// reader never mixes the two meanings: Record's JSON codec refuses a line whose fields do not
// belong to the schema it names.
//
// The contract, each clause enforced by a test rather than promised:
//
//   - OFF BY DEFAULT. Enabled is ASSAY_TELEMETRY == "1" exactly. Off, nothing is created: no
//     directory, no day file, no install id.
//   - NO FREE TEXT. Record has six string fields and each is a constant, a roster member
//     the caller compiled in, a build stamp constrained to hex, or random hex. The mappers
//     here return one of their own constants or a roster member, never any part of their input.
//   - LOCAL ONLY. The package imports nothing that can open a socket or start a process
//     (TestClosureCarriesNothingOnline); there is no sender and no endpoint.
//   - NEVER LOAD-BEARING. Append returns nothing. A failure to resolve, create, open or write
//     is swallowed: the record never changes a caller's exit code and never prints.
//   - NEVER THE LEDGER. Every path joins Dir — a perf/ SUBDIRECTORY of the state dir — so no
//     writer or pruner here can name an audit ledger file.
package perfrec

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
	"time"
)

// Schema is the LEGACY on-disk record version New builds. Its field set and meanings are
// frozen: a changed boundary or field is a new schema (SchemaV2 is the first), with its own
// field table in docs/telemetry.md and its own case in the no-identifier tests.
const Schema = "perf-v1"

// EnvVar is the one opt-in. It is the SAME variable statusgen's network telemetry reads.
const EnvVar = "ASSAY_TELEMETRY"

// RetentionDays is the history window: today plus the six prior UTC days.
const RetentionDays = 7

// Other and RoleNone are the mappers' fallbacks — constants, never the input. Unpinned is
// the source stamp of a build that carries no hex stamp.
const (
	Other    = "other"
	RoleNone = "none"
	Unpinned = "unpinned"
)

// The four result values. They equal the desk audit ledger's Result* strings; the desk kit pins
// that equality with a test, since this module cannot import it.
const (
	ResultOK           = "ok"
	ResultDisabled     = "disabled"
	ResultRefused      = "refused"
	ResultUnverifiable = "unverifiable"
)

// The exit codes ResultForExit knows. They equal the desk kit's Exit* constants (pinned by a
// test on that side).
const (
	exitOK           = 0
	exitDisabled     = 3
	exitRefused      = 5
	exitUnverifiable = 6
)

// Record is one line of the perf history, in either schema. Schema decides which fields the
// line carries and what they mean; the JSON codec (schema.go) writes exactly that schema's
// field set and refuses a record that carries another schema's fields.
//
// perf-v1 (legacy): every field down to Install except the v2-only block. cpuMs alone is
// omitted when unmeasurable, so "not measured" and "used no CPU" stay distinguishable;
// wallMs is whatever the caller timed and cpuMs is process-lifetime CPU.
//
// perf-v2: the shared fields plus the v2-only block, and none of the three legacy
// counters (their meanings live on as counters with explicit coverage). wallMs and cpuMs
// are deltas over Boundary; CPU names the CPU coverage state, and cpuMs is present only when
// it is CPUMeasured.
type Record struct {
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

	// perf-v2 only.
	Boundary     string            `json:"boundary,omitempty"`
	CPU          string            `json:"cpu,omitempty"`
	Phases       []PhaseStat       `json:"phases,omitempty"`
	PhaseOverlap bool              `json:"phaseOverlap,omitempty"`
	Counters     map[Counter]int64 `json:"counters,omitempty"`
	Unavailable  []Counter         `json:"unavailable,omitempty"`
}

// Enabled reports whether local perf logging is armed: ASSAY_TELEMETRY compared to "1"
// EXACTLY — not a truthiness test. statusgen's network telemetry makes the identical
// comparison for its environment half, so one opt-in means the same thing to every tool.
//
// There is deliberately no second switch for this LOCAL file. docs/telemetry.md's two-switch
// rule exists so that no inherited environment can cause a SEND; this log sends nothing —
// it is a file on the user's own disk. What the single switch still buys is consent: a user
// who never opted in gets no new directory, file or identifier. Any future REMOTE sink for
// these records is bound by the two-switch rule in full.
func Enabled() bool { return os.Getenv(EnvVar) == "1" }

// StateDir returns the default state directory, ~/.config/assay — the directory the desk
// tools keep their kill switch, audit log and loop flags in. Perf records live in its perf/
// subdirectory.
func StateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "assay"), nil
}

// Dir is <stateDir>/perf. Every perf path joins it, so none can name an audit ledger file.
func Dir(stateDir string) string { return filepath.Join(stateDir, "perf") }

// ResultForExit is the fixed exit→result map. It returns one of this package's constants,
// never its input.
func ResultForExit(exit int) string {
	switch exit {
	case exitOK:
		return ResultOK
	case exitDisabled:
		return ResultDisabled
	case exitRefused:
		return ResultRefused
	case exitUnverifiable:
		return ResultUnverifiable
	default:
		return ResultUnverifiable
	}
}

// Verb returns the element of roster that args[0] names, or "other". It returns the ROSTER's
// string, never args[0] itself.
func Verb(roster, args []string) string {
	if len(args) == 0 {
		return Other
	}
	for _, v := range roster {
		if v != "" && args[0] == v {
			return v
		}
	}
	return Other
}

// ShaPattern is what a build stamp looks like; anything else records as "unpinned".
var ShaPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// SourceSHA returns stamp when it is a hex build stamp, "unpinned" otherwise, so even a
// mis-set ldflag cannot carry text into the record.
func SourceSHA(stamp string) string {
	if ShaPattern.MatchString(stamp) {
		return stamp
	}
	return Unpinned
}

// New builds a LEGACY perf-v1 record (wall as the caller timed it; the caller may set cpuMs
// to process-lifetime CPU and the three legacy counters). tool, verb and role must already be
// roster values (the caller maps them); the timestamp, result and schema are fixed here. sha
// is passed through SourceSHA. New measurements use Begin/Finish (session.go) instead.
func New(tool, verb, role string, wall time.Duration, exit int, sha string, now time.Time) Record {
	return Record{
		Schema:    Schema,
		TS:        now.UTC().Truncate(time.Second).Format(time.RFC3339),
		Tool:      tool,
		Verb:      verb,
		Role:      role,
		WallMs:    wall.Milliseconds(),
		Exit:      exit,
		Result:    ResultForExit(exit),
		SourceSHA: SourceSHA(sha),
	}
}

// dayFile matches a day file name EXACTLY and captures its date. The pruner and the reader
// touch nothing else, and both parse the date out of the MATCHED name.
var dayFile = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})\.jsonl$`)

// DayOf returns the UTC day a day-file name records, ok=false for any other name.
func DayOf(name string) (time.Time, bool) {
	m := dayFile.FindStringSubmatch(name)
	if m == nil {
		return time.Time{}, false
	}
	day, err := time.Parse("2006-01-02", m[1])
	if err != nil {
		return time.Time{}, false
	}
	return day, true
}

// Cutoff is the oldest UTC day kept at now: today minus (retention-1) days, at midnight.
func Cutoff(now time.Time) time.Time {
	n := now.UTC()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -(RetentionDays - 1))
}

// Prune removes day files in dir older than Cutoff(now). It lists ONLY dir, matches names
// against the day-file pattern EXACTLY, parses the date out of the matched name, and removes
// nothing else: install-id and any name a future change adds survive. A concurrent pruner's
// ENOENT is ignored.
func Prune(dir string, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := Cutoff(now)
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() {
			continue
		}
		day, ok := DayOf(name)
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

// InstallIDPattern is the only shape an install id may have: 32 lowercase hex characters.
var InstallIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// InstallID reads <dir>/install-id, minting it (16 bytes of crypto/rand, hex) when missing or
// malformed. It is an identifier of the INSTALL, never derived from any machine or user
// property; deleting the file simply mints a new one.
func InstallID(dir string) (string, error) {
	p := filepath.Join(dir, "install-id")
	if b, err := os.ReadFile(p); err == nil {
		if id := strings.TrimSpace(string(b)); InstallIDPattern.MatchString(id) {
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
			if got := strings.TrimSpace(string(b)); InstallIDPattern.MatchString(got) {
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

// Writer appends records and remembers, per process, which directory it has already pruned
// (so the pruner runs on the first append into a directory, not on every one). The zero
// value is ready. Now is the clock; nil means time.Now (tests swap it).
type Writer struct {
	Now func() time.Time

	mu        sync.Mutex
	prunedDir string
}

func (w *Writer) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

// Reset forgets the directory last pruned, so the next write prunes again. Tests only.
func (w *Writer) Reset() {
	w.mu.Lock()
	w.prunedDir = ""
	w.mu.Unlock()
}

func (w *Writer) pruneOnce(dir string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.prunedDir == dir {
		return
	}
	w.prunedDir = dir
	Prune(dir, w.now())
}

// Append writes rec as one line to today's day file under stateDir, best-effort. It returns
// nothing: the perf log is never allowed to change what the tool does.
func (w *Writer) Append(stateDir string, rec Record) { _ = w.Write(stateDir, rec) }

// Write is Append's body with the error kept, for the tests that need to see WHY a write did
// not land. Production callers go through Append, which discards it.
func (w *Writer) Write(stateDir string, rec Record) error {
	dir := Dir(stateDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	w.pruneOnce(dir)
	id, err := InstallID(dir)
	if err != nil {
		return err
	}
	rec.Install = id
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	day := w.now().UTC().Format("2006-01-02") + ".jsonl"
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

// Default is the process-wide writer the tools share, on the real clock.
var Default = &Writer{}

// Append writes rec under the default state dir via the process-wide writer, best-effort.
// Not enabled → nothing happens. Callers that need a different state dir use Writer.Append.
func Append(rec Record) {
	if !Enabled() {
		return
	}
	dir, err := StateDir()
	if err != nil {
		return
	}
	Default.Append(dir, rec)
}
