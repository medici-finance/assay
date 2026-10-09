package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Cheap-default reset: a periodic demand lever on model pins.
//
// A model pin drifts to the most expensive setting and stays there unless something pulls it
// back. The pull-back needs two facts the cell.env line itself does not carry: WHEN the pin was
// set, and WHETHER anyone decided it (a flag or a ruling) or it is just a default riding along.
// Those two facts live in a pin record beside cell.env (model-pins.json). A DEFAULT pin older
// than the TTL is repinned to the cheap default (the harness's MID tier entry); an EXPLICIT pin
// is never touched. A pin with no record has unknown provenance and is treated as EXPLICIT.

const (
	pinKindDefault  = "default"
	pinKindExplicit = "explicit"

	// modelPinsFile is the pin record, in the cell directory next to cell.env.
	modelPinsFile = "model-pins.json"

	// modelTTLKey is the cell.env key holding the TTL in days; modelTTLDefaultDays is the
	// house default (weekly). 0 turns the reset off for that cell.
	modelTTLKey         = "CELL_MODEL_TTL_DAYS"
	modelTTLDefaultDays = 7
)

// pinRecord is one pin's provenance. Value is what cell.env held when the record was written, so
// a later cell.env that disagrees with it is recognisable as a hand edit.
type pinRecord struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	At    string `json:"at"`
}

type pinFile struct {
	Pins map[string]pinRecord `json:"pins"`
}

// isModelPinKey is true for a per-harness model pin key: DESK_MODEL_<role|DEFAULT>,
// CODEX_MODEL_<role|default>, CURSOR_MODEL_<role|default>.
func isModelPinKey(k string) bool {
	for _, p := range []string{"DESK_MODEL_", "CODEX_MODEL_", "CURSOR_MODEL_"} {
		if strings.HasPrefix(k, p) && len(k) > len(p) {
			return true
		}
	}
	return false
}

// pinRole is the role a pin key names, for the notice: DESK_MODEL_worker_desk -> worker-desk,
// and the harness-wide default key -> "default".
func pinRole(k string) string {
	for _, p := range []string{"DESK_MODEL_", "CODEX_MODEL_", "CURSOR_MODEL_"} {
		if strings.HasPrefix(k, p) {
			r := strings.TrimPrefix(k, p)
			if strings.EqualFold(r, "default") {
				return "default"
			}
			return strings.ReplaceAll(r, "_", "-")
		}
	}
	return k
}

// pinHarness is the harness namespace a pin key belongs to.
func pinHarness(k string) string {
	switch {
	case strings.HasPrefix(k, "CODEX_MODEL_"):
		return "codex"
	case strings.HasPrefix(k, "CURSOR_MODEL_"):
		return "cursor"
	}
	return "claude"
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
	if err := os.WriteFile(filepath.Join(dir, modelPinsFile), append(raw, '\n'), 0o600); err != nil {
		die("model pins: cannot write %s: %v", filepath.Join(dir, modelPinsFile), err)
	}
}

// recordPins stamps the pin record for every model pin key in kvs after a `set`. A plain set
// records DEFAULT; an existing EXPLICIT record is sticky (a later plain set updates its value
// but never downgrades it), so only an `--explicit` set or a deliberate edit of the record
// changes who owns a pin.
func recordPins(dir string, kvs []string, kind string, now time.Time) {
	var pf pinFile
	loaded := false
	for _, kv := range kvs {
		k, v, ok := splitKV(kv)
		if !ok || !isModelPinKey(k) {
			continue
		}
		if !loaded {
			pf, loaded = loadPinFile(dir), true
		}
		k2 := kind
		if old, had := pf.Pins[k]; had && old.Kind == pinKindExplicit {
			k2 = pinKindExplicit
		}
		pf.Pins[k] = pinRecord{Kind: k2, Value: v, At: now.UTC().Format(time.RFC3339)}
	}
	if loaded {
		savePinFile(dir, pf)
	}
}

// parseModelTTL reads a TTL as Nd (days), or any time.ParseDuration form, or a bare number of
// days. Zero means "off".
func parseModelTTL(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty TTL")
	}
	days := strings.TrimSuffix(s, "d")
	if n, err := strconv.ParseFloat(days, 64); err == nil {
		if n < 0 {
			return 0, fmt.Errorf("TTL must not be negative: %q", s)
		}
		return time.Duration(n * float64(24*time.Hour)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("TTL %q is not Nd (days), a Go duration, or a number of days", s)
	}
	return d, nil
}

// cellModelTTL is the cell's TTL: cell.env CELL_MODEL_TTL_DAYS, else the weekly default. A
// malformed value is a refusal, not a silent default.
func cellModelTTL(e *Env) time.Duration {
	v := e.Get(modelTTLKey)
	if v == "" {
		return modelTTLDefaultDays * 24 * time.Hour
	}
	d, err := parseModelTTL(v)
	if err != nil {
		die("cell.env: %s: %v", modelTTLKey, err)
	}
	return d
}

// cheapDefault is the model a DEFAULT pin is pulled back to: the MID tier entry for the pin's
// harness, already configured per cell (TIER_MODEL_MID_<HARNESS>).
func cheapDefault(e *Env, key string) string {
	return e.Get("TIER_MODEL_MID_" + strings.ToUpper(pinHarness(key)))
}

// resetAgedDefaultPins repins every DEFAULT pin older than ttl to the cheap default and returns
// one notice line per pin it moved. EXPLICIT pins and pins with no record are never touched; the
// the-desk pin is never touched either (the methodology pins the coordinator to the top tier).
// A DEFAULT pin whose cell.env value no longer matches its record was hand-edited: its clock
// restarts instead of being repinned on the old age. write=false computes and applies the
// result to the in-memory env only (the dry-run plan), leaving cell.env and the record alone.
func (c *Cell) resetAgedDefaultPins(ttl time.Duration, now time.Time, write bool) []string {
	if ttl <= 0 {
		return nil
	}
	envfile := filepath.Join(c.Dir, "cell.env")
	pf := loadPinFile(c.Dir)
	// What cell.env itself says, read through the loader (not the launch env, which a flag or the
	// process environment may already have overlaid).
	fileEnv := effectiveCellEnv(envfile, nil)
	keys := make([]string, 0, len(pf.Pins))
	for k := range pf.Pins {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var notices []string
	dirty := false
	for _, k := range keys {
		rec := pf.Pins[k]
		if rec.Kind != pinKindDefault || !isModelPinKey(k) || pinRole(k) == "the-desk" {
			continue
		}
		if !fileEnv.IsSet(k) {
			continue
		}
		cur := fileEnv.Get(k)
		at, err := time.Parse(time.RFC3339, rec.At)
		if err != nil {
			continue // an unreadable stamp is unknown provenance: leave the pin alone
		}
		if cur != rec.Value {
			pf.Pins[k] = pinRecord{Kind: pinKindDefault, Value: cur, At: now.UTC().Format(time.RFC3339)}
			dirty = true
			continue
		}
		if now.Sub(at) < ttl {
			continue
		}
		cheap := cheapDefault(c.Env, k)
		if cheap == "" || cheap == cur {
			continue
		}
		ageDays := int(now.Sub(at) / (24 * time.Hour))
		notices = append(notices, fmt.Sprintf("[model-reset] %s: %s -> %s (default pin %dd old, ttl %s)",
			pinRole(k), cur, cheap, ageDays, fmtTTL(ttl)))
		c.Env.Put(k, cheap)
		if write {
			writeEnvKey(envfile, k, cheap, false)
			pf.Pins[k] = pinRecord{Kind: pinKindDefault, Value: cheap, At: now.UTC().Format(time.RFC3339)}
			dirty = true
		}
	}
	if write && dirty {
		savePinFile(c.Dir, pf)
	}
	return notices
}

func fmtTTL(ttl time.Duration) string {
	if ttl%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", int(ttl/(24*time.Hour)))
	}
	return ttl.String()
}

// bootModelReset is the launch-time hook: reset aged DEFAULT pins and print the one-line
// notices. A dry run plans the repin without writing it.
func (c *Cell) bootModelReset(now time.Time) {
	write := c.Env.Get("DRY_RUN") != "1"
	for _, n := range c.resetAgedDefaultPins(cellModelTTL(c.Env), now, write) {
		if !write {
			n += " [dry-run: not written]"
		}
		fmt.Println(n)
	}
}

// cmdModels is `cellctl models reset <cell> [--model-ttl <Nd|duration>]`: the explicit form of
// the boot-time reset.
func cmdModels(args []string) {
	if len(args) == 0 || args[0] != "reset" {
		die("models: usage: cellctl models reset <cell> [--model-ttl <Nd>]")
	}
	cell := needCell(args[1:])
	var ttlFlag string
	rest := args[2:]
	for i := 0; i < len(rest); i++ {
		switch a := rest[i]; a {
		case "--model-ttl":
			ttlFlag = needFlagValue(rest, &i, "--model-ttl needs a value (Nd, e.g. 7d)")
		default:
			die("models reset: unknown argument %s", a)
		}
	}
	c := loadCell(cell)
	ttl := cellModelTTL(c.Env)
	if ttlFlag != "" {
		d, err := parseModelTTL(ttlFlag)
		if err != nil {
			die("models reset: --model-ttl: %v", err)
		}
		ttl = d
	}
	notices := c.resetAgedDefaultPins(ttl, time.Now(), c.Env.Get("DRY_RUN") != "1")
	for _, n := range notices {
		fmt.Println(n)
	}
	if len(notices) == 0 {
		fmt.Printf("[model-reset] nothing to repin (ttl %s)\n", fmtTTL(ttl))
	}
}
