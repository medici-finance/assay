package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Cheap-default reset: a periodic demand lever on model pins.
//
// A model pin drifts to the most expensive setting and stays there unless something pulls it
// back. The pull-back needs two facts the cell.env line itself does not carry: WHEN the pin was
// set, and WHETHER anyone decided it or it is a default riding along. Those two facts live in a
// pin record beside cell.env (model-pins.json).
//
// A pin an operator sets is EXPLICIT: plain `cellctl set`, the role form, and `desk`/`up --set`
// all record EXPLICIT, and the reset never touches an explicit pin. Only a pin set with
// `cellctl set ... --default` is recorded DEFAULT. A pin with no record has unknown provenance and
// is treated as EXPLICIT. A DEFAULT pin older than the TTL is repinned to the cheap default (the
// harness's MID tier entry in the cell's own files) only when that LOWERS it.

const (
	pinKindDefault  = "default"
	pinKindExplicit = "explicit"

	// modelPinsFile is the pin record, in the cell directory next to cell.env; modelPinsLock is
	// the mkdir lock every write of cell.env-plus-record holds.
	modelPinsFile = "model-pins.json"
	modelPinsLock = ".model-pins.lock"

	// modelTTLKey is the cell.env key holding the TTL in days; modelTTLDefaultDays is the
	// default (weekly). 0 turns the reset off for that cell.
	modelTTLKey         = "CELL_MODEL_TTL_DAYS"
	modelTTLDefaultDays = 7
)

// pinRecord is one pin's provenance. Value is what cell.env held when the record was written, so
// a later cell.env that disagrees with it is recognisable as a hand edit. Prev is the value a
// reset moved the pin from, kept so the repin stays reviewable after the boot's output is gone.
type pinRecord struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	At    string `json:"at"`
	Prev  string `json:"prev,omitempty"`
}

type pinFile struct {
	Pins map[string]pinRecord `json:"pins"`
}

// pinSpaces are the three harness namespaces a model pin lives in.
var pinSpaces = []struct{ prefix, def, harness string }{
	{"DESK_MODEL_", "DESK_MODEL_DEFAULT", "claude"},
	{"CODEX_MODEL_", "CODEX_MODEL_default", "codex"},
	{"CURSOR_MODEL_", "CURSOR_MODEL_default", "cursor"},
}

// modelPinKey names the harness and role a model pin key belongs to. The set is exact, the
// per-role pin of every known role plus the harness default, never a prefix match:
// DESK_MODEL_OVERRIDE and the floor-override name share the prefix and are not pins.
func modelPinKey(k string) (harness, role string, ok bool) {
	for _, s := range pinSpaces {
		if k == s.def {
			return s.harness, "default", true
		}
		for _, r := range knownRoles {
			if k == s.prefix+underscore(r) {
				return s.harness, r, true
			}
		}
	}
	return "", "", false
}

func isModelPinKey(k string) bool {
	_, _, ok := modelPinKey(k)
	return ok
}

func loadPinFile(dir string) pinFile {
	pf := pinFile{Pins: map[string]pinRecord{}}
	raw, err := os.ReadFile(filepath.Join(dir, modelPinsFile))
	if err != nil {
		return pf
	}
	if err := json.Unmarshal(raw, &pf); err != nil || pf.Pins == nil {
		// An unreadable record must not make every pin look DEFAULT: start empty, which reads as
		// "provenance unknown, treat as explicit".
		return pinFile{Pins: map[string]pinRecord{}}
	}
	return pf
}

func savePinFile(dir string, pf pinFile) {
	raw, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		die("model pins: cannot encode record: %v", err)
	}
	if err := writeFileAtomic(filepath.Join(dir, modelPinsFile), append(raw, '\n'), 0o600); err != nil {
		die("model pins: cannot write %s: %v", filepath.Join(dir, modelPinsFile), err)
	}
}

// pinLockTries is how many 50ms waits a held pin lock gets (3s) before the caller is told.
var pinLockTries = 60

// lockModelPins serialises the writers of cell.env and its pin record (a `set`, a boot reset,
// `models reset`), so windows booting together cannot interleave a read-modify-write. A lock
// whose holder is dead is taken over; one held past the wait is an error the caller reports.
func lockModelPins(dir string) (func(), error) {
	lockdir := filepath.Join(dir, modelPinsLock)
	for i := 0; i < pinLockTries; i++ {
		err := os.Mkdir(lockdir, 0o700)
		if err == nil {
			writePid(lockdir, strconv.Itoa(os.Getpid()))
			return func() { _ = os.RemoveAll(lockdir) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if held := readPid(lockdir); held != "" && !pidAlive(held) {
			_ = os.RemoveAll(lockdir)
			continue
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, fmt.Errorf("%s is held by another cellctl (pid %q); remove it if no cellctl is running", lockdir, readPid(lockdir))
}

// recordPins stamps the pin record for every model pin key in kvs after a `set` has written
// them. The value recorded is the one the loader reads back (a quoted line is unquoted), so the
// next reset does not mistake the quoting for a hand edit. An empty value is not a pin: its
// record is dropped.
func recordPins(envfile string, kvs []string, kind string, now time.Time) {
	var keys []string
	for _, kv := range kvs {
		if k, _, ok := splitKV(kv); ok && isModelPinKey(k) {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return
	}
	dir := filepath.Dir(envfile)
	pf := loadPinFile(dir)
	fileEnv := effectiveCellEnv(envfile, nil)
	for _, k := range keys {
		if v := fileEnv.Get(k); v == "" {
			delete(pf.Pins, k)
		} else {
			pf.Pins[k] = pinRecord{Kind: kind, Value: v, At: now.UTC().Format(time.RFC3339)}
		}
	}
	savePinFile(dir, pf)
}

// ttlDays is the days form of a TTL: digits, an optional fraction, an optional d. Five digits
// bound it well inside a time.Duration, and NaN, Inf, exponents and underscores never match.
var ttlDays = regexp.MustCompile(`^[0-9]{1,5}(\.[0-9]{1,6})?d?$`)

// parseModelTTL reads a TTL as Nd or N (days) or a Go duration (36h). Zero means "off".
func parseModelTTL(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty TTL")
	}
	if ttlDays.MatchString(s) {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "d"), 64)
		if err != nil {
			return 0, fmt.Errorf("TTL %q: %v", s, err)
		}
		return time.Duration(n * float64(24*time.Hour)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("TTL %q is not Nd (days, e.g. 7d), a number of days, or a non-negative Go duration (e.g. 36h)", s)
	}
	return d, nil
}

// cellModelTTLErr is the cell's TTL: CELL_MODEL_TTL_DAYS, else the weekly default.
func cellModelTTLErr(e *Env) (time.Duration, error) {
	v := e.Get(modelTTLKey)
	if v == "" {
		return modelTTLDefaultDays * 24 * time.Hour, nil
	}
	d, err := parseModelTTL(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %v", modelTTLKey, err)
	}
	return d, nil
}

// cellModelTTL is cellModelTTLErr with a malformed value as a refusal, not a silent default.
func cellModelTTL(e *Env) time.Duration {
	d, err := cellModelTTLErr(e)
	if err != nil {
		die("cell.env: %v (`cellctl set` refuses it; fix the line or pass --model-ttl)", err)
	}
	return d
}

// tierEntry is one tier-map entry from the cell's own files, else the compiled map. Never the
// launch environment: a variable exported in the invoking shell must not be persisted.
func tierEntry(fileEnv *Env, level, harness string) string {
	k := "TIER_MODEL_" + level + "_" + strings.ToUpper(harness)
	if v := fileEnv.Get(k); v != "" {
		return v
	}
	return tierModelDefaults[k]
}

// modelRank orders a model for the reset's one question, "is this above the cheap default":
// 3 top, 2 mid, 1 fast, by the cell's tier map, then (claude only) by the model family named in
// the value. 0 is unknown, and an unknown model is never repinned.
func modelRank(fileEnv *Env, harness, v string) int {
	for i, level := range []string{"TOP", "MID", "FAST"} {
		if t := tierEntry(fileEnv, level, harness); t != "" && t == v {
			return 3 - i
		}
	}
	if harness == "claude" {
		b := policyBase(v)
		switch {
		case strings.Contains(b, "haiku"):
			return 1
		case strings.Contains(b, "sonnet"):
			return 2
		case strings.Contains(b, "opus"), strings.Contains(b, "fable"):
			return 3
		}
	}
	return 0
}

// pinMove is one repin the reset makes (or, read-only, would make).
type pinMove struct {
	Key, Role, From, To string
	Age, TTL            time.Duration
}

func (m pinMove) notice() string {
	return fmt.Sprintf("[model-reset] %s: %s -> %s (default pin %s old, ttl %s)", m.Role, m.From, m.To, fmtAge(m.Age, m.TTL), fmtTTL(m.TTL))
}

// planModelReset decides which DEFAULT pins the reset moves, from the cell's own files and the
// pin record; it writes nothing. A pin is moved only when ALL of these hold:
//   - its record says DEFAULT and its key is a real model pin;
//   - it is not the-desk's, and it is not a harness default the-desk resolves through (no
//     per-role the-desk pin on that harness): the coordinator's resolved model never moves;
//   - it is not a claude pin on a provider cell (the provider's endpoint rejects tier names);
//   - cell.env still carries a non-empty line for it, equal to the recorded value (a different
//     value is a hand edit: the clock restarts), and the record is older than the TTL;
//   - the cheap default ranks known and strictly below the current value.
//
// The returned record has stale entries dropped and hand-edited clocks restarted; dirty says
// it changed.
func (c *Cell) planModelReset(ttl time.Duration, now time.Time, provider string) ([]pinMove, pinFile, bool) {
	pf := loadPinFile(c.Dir)
	if ttl <= 0 || len(pf.Pins) == 0 {
		return nil, pf, false
	}
	envfile := filepath.Join(c.Dir, "cell.env")
	fileEnv := effectiveCellEnv(envfile, nil)
	onProvider := provider != "" || fileEnv.Get("CELL_PROVIDER") != ""
	keys := make([]string, 0, len(pf.Pins))
	for k := range pf.Pins {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var moves []pinMove
	dirty := false
	for _, k := range keys {
		rec := pf.Pins[k]
		harness, role, ok := modelPinKey(k)
		if !ok || rec.Kind != pinKindDefault {
			continue
		}
		cur := fileEnv.Get(k)
		if fileEnv.Source(k) != layerCellEnv || cur == "" {
			delete(pf.Pins, k) // the line is gone or empty: nothing is pinned any more
			dirty = true
			continue
		}
		if role == "the-desk" || (role == "default" && c.theDeskFallsThrough(fileEnv, harness)) {
			continue
		}
		if harness == "claude" && onProvider {
			continue
		}
		at, err := time.Parse(time.RFC3339, rec.At)
		if err != nil {
			continue // an unreadable stamp is unknown provenance: leave the pin alone
		}
		if cur != rec.Value {
			pf.Pins[k] = pinRecord{Kind: pinKindDefault, Value: cur, At: now.UTC().Format(time.RFC3339)}
			dirty = true
			continue
		}
		age := now.Sub(at)
		if age <= ttl {
			continue
		}
		to := tierEntry(fileEnv, "MID", harness)
		if to == "" || to == cur {
			continue
		}
		if rt := modelRank(fileEnv, harness, to); rt == 0 || modelRank(fileEnv, harness, cur) <= rt {
			continue
		}
		moves = append(moves, pinMove{Key: k, Role: role, From: cur, To: to, Age: age, TTL: ttl})
	}
	return moves, pf, dirty
}

// theDeskFallsThrough is true when the-desk has no per-role pin on harness, in the cell's files
// or in the launch env, so it resolves through that harness's default.
func (c *Cell) theDeskFallsThrough(fileEnv *Env, harness string) bool {
	for _, s := range pinSpaces {
		if s.harness == harness {
			k := s.prefix + underscore("the-desk")
			return fileEnv.Get(k) == "" || c.Env.Get(k) == ""
		}
	}
	return true
}

// resetAgedDefaultPins applies the plan. The in-memory env always takes the repin (so a dry-run
// plan matches the launch); write=false stops there. A live run validates every repin before
// touching anything, then writes one cell.env backup, one atomic cell.env rewrite and the record,
// under the pin lock. It returns the moves and the backup path ("" when nothing was written).
func (c *Cell) resetAgedDefaultPins(ttl time.Duration, now time.Time, write bool, provider string) ([]pinMove, string, error) {
	if write && ttl > 0 {
		unlock, err := lockModelPins(c.Dir)
		if err != nil {
			return nil, "", err
		}
		defer unlock()
	}
	moves, pf, dirty := c.planModelReset(ttl, now, provider)
	for _, m := range moves {
		if c.Env.Get(m.Key) == m.From {
			c.Env.Put(m.Key, m.To)
		}
	}
	if !write {
		return moves, "", nil
	}
	backup := ""
	if len(moves) > 0 {
		envfile := filepath.Join(c.Dir, "cell.env")
		kvs := make([]string, 0, len(moves))
		for _, m := range moves {
			validateEnvKey(m.Key, m.To, false)
			kvs = append(kvs, m.Key+"="+m.To)
		}
		backup = backupCellEnv(envfile)
		writeEnvKeys(envfile, kvs)
		for _, m := range moves {
			pf.Pins[m.Key] = pinRecord{Kind: pinKindDefault, Value: m.To, At: now.UTC().Format(time.RFC3339), Prev: m.From}
		}
		dirty = true
	}
	if dirty {
		savePinFile(c.Dir, pf)
	}
	return moves, backup, nil
}

func fmtTTL(ttl time.Duration) string {
	if ttl%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", int(ttl/(24*time.Hour)))
	}
	return ttl.String()
}

// fmtAge prints the age in the TTL's own unit: whole days beside a days TTL, else a duration.
func fmtAge(age, ttl time.Duration) string {
	if ttl%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", int(age/(24*time.Hour)))
	}
	return age.Truncate(time.Minute).String()
}

// dryRunMark is appended to every reset line a DRY_RUN=1 run prints.
const dryRunMark = " [dry-run: not written]"

// bootModelReset is the launch-time hook: reset aged DEFAULT pins and print one line per pin
// moved. A dry run plans the repin without writing it. A held lock skips this boot's reset.
func (c *Cell) bootModelReset(now time.Time, provider string) {
	write := c.Env.Get("DRY_RUN") != "1"
	moves, _, err := c.resetAgedDefaultPins(cellModelTTL(c.Env), now, write, provider)
	if err != nil {
		fmt.Printf("[model-reset] skipped this boot: %v\n", err)
		return
	}
	for _, m := range moves {
		n := m.notice()
		if !write {
			n += dryRunMark
		}
		fmt.Println(n)
	}
}

// pendingModelReset is the read-only reset `show` and `check` apply before they resolve, so
// what they report is what the next boot runs.
func (c *Cell) pendingModelReset(provider string) ([]pinMove, error) {
	ttl, err := cellModelTTLErr(c.Env)
	if err != nil {
		return nil, err
	}
	moves, _, _ := c.resetAgedDefaultPins(ttl, time.Now(), false, provider)
	return moves, nil
}

const modelsUsage = "cellctl models reset <cell> [--model-ttl <Nd|duration>]"

// cmdModels is `cellctl models reset <cell> [--model-ttl <Nd|duration>]`: the explicit form of
// the boot-time reset. --model-ttl replaces the cell's TTL for this run, so it also recovers a
// cell whose CELL_MODEL_TTL_DAYS line is malformed.
func cmdModels(args []string) {
	if len(args) == 0 || args[0] != "reset" {
		die("models: usage: %s", modelsUsage)
	}
	cell := needCell(args[1:])
	var ttlFlag string
	rest := args[2:]
	for i := 0; i < len(rest); i++ {
		switch a := rest[i]; a {
		case "--model-ttl":
			ttlFlag = needFlagValue(rest, &i, "--model-ttl needs a value (Nd, e.g. 7d)")
		default:
			die("models reset: unknown argument %s (usage: %s)", a, modelsUsage)
		}
	}
	c := loadCell(cell)
	var ttl time.Duration
	if ttlFlag != "" {
		d, err := parseModelTTL(ttlFlag)
		if err != nil {
			die("models reset: --model-ttl: %v", err)
		}
		ttl = d
	} else {
		ttl = cellModelTTL(c.Env)
	}
	write := c.Env.Get("DRY_RUN") != "1"
	moves, backup, err := c.resetAgedDefaultPins(ttl, time.Now(), write, c.Env.Get("CELL_PROVIDER"))
	if err != nil {
		die("models reset: %v; nothing written", err)
	}
	mark := ""
	if !write {
		mark = dryRunMark
	}
	if backup != "" {
		fmt.Printf("[model-reset] backup written: %s\n", backup)
	}
	for _, m := range moves {
		fmt.Println(m.notice() + mark)
	}
	if len(moves) == 0 {
		fmt.Printf("[model-reset] nothing to repin (ttl %s)%s\n", fmtTTL(ttl), mark)
	}
}
