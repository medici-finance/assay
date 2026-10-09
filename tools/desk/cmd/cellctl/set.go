package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// knownCellEnvKey reports a key `cellctl set` writes without --force: a lever in the key
// registry (envkeys.go), machine-wide or per-cell. The per-role model pins are listed there for
// the roles cellctl knows and a provider's three variables are matched by shape, so a typo'd
// role, tier or provider variable is refused rather than silently scaffolding a variable
// nothing reads. A key cellctl reads that is not a lever (a per-run switch, an ambient host
// variable) still needs --force: it does not belong in a file.
func knownCellEnvKey(k string) bool { return envKeyIsLever(k) }

// validateEnvKey is everything that can be checked WITHOUT touching the file: the key is a
// shell-identifier shape, is a known cell.env key unless force, and (for DESK_MODEL_the_desk
// specifically) is not an Opus pin. Split out of setEnvKey so a multi-key `set` can validate
// every KEY=VALUE before the backup or the first write — a refusal on key 2 of 3 must leave
// cell.env exactly as it found it.
func validateEnvKey(key, value string, force bool) {
	if !validEnvKeyShape(key) {
		die("set: '%s' is not a valid KEY (letters, digits, underscore; must not start with a digit)", key)
	}
	if !force && !knownCellEnvKey(key) {
		die("set: '%s' is not a known cell.env key — pass --force to set it anyway", key)
	}
	if key == "DESK_MODEL_the_desk" {
		refuseOpusForTheDesk(value)
	}
	// Value checks on the same footing as the Opus rule above — never bypassable by --force,
	// which only widens the KEY allowlist. A persisted value that is not one load_cell /
	// cockpit_want accepts would only surface as a refusal at the NEXT boot; `set` catches it
	// here instead.
	switch key {
	case "CELL_CADENCE":
		resolveCadence("house", value, "")
	case "CELL_TICK_BUDGET":
		resolveCadence("house", "30m", value)
	case "CELL_HARNESS":
		if !valueIn(value, harnessValues) {
			die("set: CELL_HARNESS must be one of %s, got '%s'", joinPipe(harnessValues), value)
		}
	case "CELL_KIND":
		if !valueIn(value, kindValues) {
			die("set: CELL_KIND must be one of %s, got '%s'", joinPipe(kindValues), value)
		}
	case "CELL_COCKPIT":
		if !valueIn(value, cockpitValues) {
			die("set: CELL_COCKPIT must be one of %s, got '%s'", joinPipe(cockpitValues), value)
		}
	case deskkit.EnvRepairAdmission:
		// The dispatch-boundary repair-admission gate is a strict on/off opt-in
		// (deskkit.RepairAdmissionEnabled enables ONLY on the literal "on"). A malformed value
		// is refused here, on the SAME footing as the Opus/harness/kind rules above — never
		// bypassable by --force, which only widens the KEY allowlist, never the value rule for a
		// known key.
		if value != "on" && value != "off" {
			die("set: %s must be 'on' or 'off', got '%s'", deskkit.EnvRepairAdmission, value)
		}
	}
}

// envFileValue is the LAST active `KEY=` line's value in a cell.env, or ("", false) when the
// file carries no such line. File-level on purpose: `set`/`show` must never SOURCE an
// operator-writable file to answer "what does cell.env say", and this is also how `show` tells a
// cell.env value from a compiled default.
func envFileValue(envfile, key string) (string, bool) {
	f, err := os.Open(envfile)
	if err != nil {
		return "", false
	}
	defer f.Close()
	val, found := "", false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, key+"=") {
			val, found = strings.TrimPrefix(line, key+"="), true
		}
	}
	return val, found
}

// validateKindChange asserts the target kind's own precondition against the EFFECTIVE value of
// the key it needs — given in this same call's KEY=VALUE list, else the file's current line — so
// `set <cell> CELL_KIND=house CELL_ROOTS=…` in one call passes while `CELL_KIND=house` alone on
// a cell with no CELL_ROOTS refuses, naming the missing key, before a backup or an edit is made.
//
// The effective value is read through the ONE cell.env loader (effectiveCellEnv), never a
// file-level re-parse of its own: a second reader that strips quotes or backslashes its own way
// passes or refuses a value loadCell then reads differently (a Windows `C:\…` path, a %q-escaped
// launcher). TestCellEnvValueReaders keeps it that way.
func validateKindChange(envfile, kind string, kvs []string) {
	need := ""
	e := effectiveCellEnv(envfile, kvs)
	switch kind {
	case "container":
		if cfg := e.Get("CELL_CONTAINER_CONFIG"); cfg != "" {
			if e.Get("CELL_CONTAINER_LAUNCHER") != "" {
				die("set: clear CELL_CONTAINER_LAUNCHER when selecting native container configuration")
			}
			if !filepath.IsAbs(cfg) || !isRegular(cfg) {
				die("set: CELL_CONTAINER_CONFIG must be an absolute configuration file")
			}
			return
		}
		need = "CELL_CONTAINER_LAUNCHER"
	case "house":
		need = "CELL_ROOTS"
	case "scrubbed":
		need = "CELL_REPO_SLUG"
	default:
		return
	}
	// A line `cellctl new` wrote with %q may be shell-quoted (CELL_ROOTS='' on a container cell
	// is the empty string once sourced, not two characters); the loader already reads it so.
	v := e.Get(need)
	if v == "" {
		die("set: CELL_KIND=%s needs %s, which is neither set in %s nor given in this call — a %s cell cannot load without it (set both in one call, or %s first); nothing written", kind, need, envfile, kind, need)
	}
	if kind == "container" {
		if err := cellPathCheck(runtime.GOOS, v); err != nil {
			die("set: container launcher: %v; nothing written", err)
		}
	}
	if kind == "container" && !isExecFile(v) {
		die("set: CELL_KIND=container needs an absolute executable CELL_CONTAINER_LAUNCHER, got '%s'; nothing written", v)
	}
}

// effectiveCellEnv is what the two files will say to loadCell once this call's KEY=VALUE pairs
// are written: the machine-wide defaults file, then the cell's file read through the loader,
// then each pair overlaid exactly as the loader will read the raw `KEY=VALUE` line setEnvKey
// writes for it.
//
// The defaults file is a layer this function READS and never a file `set` writes: every write
// goes through setEnvKey on the cell's own cell.env. Reading it here is what keeps a pair `set`
// validates from being judged against a different cell than the one loadCell will then build.
//
// Every line is read over the process environment, as loadCell reads it, so a `$VAR` in either
// file or in a pair expands to what a launch from this shell expands it to. Read over nothing,
// `$HOME/launcher` became `/launcher` and `set` judged a path no cell boots with.
//
// The result still holds only the keys the two files and the pairs assign. That part is
// deliberate: `set` judges what the files say, so a variable exported in the one shell that
// happens to run `set` neither satisfies a precondition (a launcher, a stream-root map) nor
// hides a missing line the next launch, from another shell, would trip on.
func effectiveCellEnv(envfile string, kvs []string) *Env {
	boot := newEnvFromProcess()
	if _, _, err := overlayCellDefaults(boot, cellDefaultsFor(envfile)); err != nil {
		die("set: %v; nothing written", err)
	}
	if err := parseCellEnv(boot, envfile); err != nil {
		die("set: %v", err)
	}
	for _, kv := range kvs {
		if k, v, ok := splitKV(kv); ok {
			if k != "CELL_COMMS_CONFIG" {
				v = unquoteShellValue(v, boot)
			}
			boot.putFrom(k, v, layerCellEnv)
		}
	}
	e := &Env{vals: map[string]string{}, set: map[string]bool{}}
	for k, layer := range boot.src {
		if layer == layerDefaults || layer == layerCellEnv {
			e.putFrom(k, boot.vals[k], layer)
		}
	}
	return e
}

func splitKV(kv string) (string, string, bool) {
	i := strings.IndexByte(kv, '=')
	if i < 0 {
		return "", "", false
	}
	return kv[:i], kv[i+1:], true
}

// applyEnvKVs is the ONE write path for every persisted change — `cellctl set` (both forms) and
// `desk`/`up --set` alike. Pass 1 validates every pair and touches nothing on a refusal; pass 2
// writes exactly one backup, then applies each pair in order.
func applyEnvKVs(e *Env, envfile string, force bool, kvs []string) {
	applyEnvKVsKind(e, envfile, force, kvs, pinKindDefault)
}

// applyEnvKVsKind is applyEnvKVs with the provenance a model pin in kvs is recorded under: a
// pin set through `set` is a DEFAULT (the cheap-default reset may repin it once it ages past the
// TTL) unless the caller declares it EXPLICIT (`set --explicit`, for a pin a brief or ruling set).
func applyEnvKVsKind(e *Env, envfile string, force bool, kvs []string, pinKind string) {
	if len(kvs) == 0 {
		die("set: at least one KEY=VALUE is required (cellctl set <cell> KEY=VALUE [...])")
	}
	for _, kv := range kvs {
		key, value, _ := splitKV(kv)
		validateEnvKey(key, value, force)
		if key == "CELL_KIND" {
			validateKindChange(envfile, value, kvs)
		}
	}
	// Judge Cursor against the final transaction, so a multi-key switch to a
	// house cell is accepted atomically and an unsupported combination writes nothing.
	final := envWithFileForCursor(envfile, kvs)
	if final.Get("CELL_HARNESS") == "cursor" {
		if err := cursorConfigurationError(final.GetOr("CELL_KIND", "k8s"), final.Get("CELL_PROVIDER"), final.Get("CELL_MODEL_POLICY") != ""); err != nil {
			die("set: %v; nothing written", err)
		}
	}
	backup := envfile + ".bak-" + time.Now().UTC().Format("20060102T150405Z")
	raw, err := os.ReadFile(envfile)
	if err != nil {
		die("set: cannot read %s: %v", envfile, err)
	}
	if err := os.WriteFile(backup, raw, 0o600); err != nil {
		die("set: cannot write %s: %v", backup, err)
	}
	fmt.Printf("[set] backup written: %s\n", backup)
	for _, kv := range kvs {
		key, value, _ := splitKV(kv)
		setEnvKey(envfile, key, value, force)
	}
	recordPins(filepath.Dir(envfile), kvs, pinKind, time.Now())
}

func envWithFileForCursor(envfile string, kvs []string) *Env {
	return effectiveCellEnv(envfile, kvs)
}

// setEnvKey is the single-key rewrite/append, after validateEnvKey has already passed. The
// rewrite is line-for-line: an existing `KEY=...` line (not a `#`-commented one) is replaced in
// place so comments and ordering are untouched; a key with no active line is appended.
func setEnvKey(envfile, key, value string, force bool) {
	before := writeEnvKey(envfile, key, value, force)
	shown := before
	if shown == "" {
		shown = "<unset>"
	}
	fmt.Printf("[set] %s: %s -> %s\n", key, shown, value)
}

// writeEnvKey is setEnvKey's write, silent, returning the value the key held before ("" when it
// had no active line). The boot-time model reset writes through it so that its one notice line
// is the only thing a launch prints.
func writeEnvKey(envfile, key, value string, force bool) string {
	validateEnvKey(key, value, force)
	raw, err := os.ReadFile(envfile)
	if err != nil {
		die("set: cannot read %s: %v", envfile, err)
	}
	// The comms argument is a literal manifest path. Encode it in the existing
	// cell.env grammar so apostrophes, dollars and backslashes survive reload.
	stored := value
	if key == "CELL_COMMS_CONFIG" {
		stored = bashQuote(value)
	}
	trailingNewline := strings.HasSuffix(string(raw), "\n")
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	before, has := "", false
	for _, l := range lines {
		if strings.HasPrefix(l, key+"=") {
			if !has {
				before = strings.TrimPrefix(l, key+"=")
			}
			has = true
		}
	}
	if has {
		replaced := false
		for i, l := range lines {
			if !replaced && strings.HasPrefix(l, key+"=") {
				lines[i] = key + "=" + stored
				replaced = true
			}
		}
	} else {
		lines = append(lines, key+"="+stored)
	}
	out := strings.Join(lines, "\n")
	if trailingNewline || !has {
		out += "\n"
	}
	st, serr := os.Stat(envfile)
	mode := os.FileMode(0o600)
	if serr == nil {
		mode = st.Mode().Perm()
	}
	if err := os.WriteFile(envfile, []byte(out), mode); err != nil {
		die("set: cannot write %s: %v", envfile, err)
	}
	return before
}

// activeHarnessOf is the LAST `CELL_HARNESS=` line in a cell.env, or `claude` when the file
// carries none. File-level, not loadCell, on purpose: `set` edits the file directly and never
// sources it (sourcing an operator-writable cell.env as part of `set` would execute arbitrary
// shell in it), so this reads the one line it needs by hand.
func activeHarnessOf(envfile string) string {
	return envWithFileForCursor(envfile, nil).GetOr("CELL_HARNESS", "claude")
}

const setUsage = "cellctl set <cell> KEY=VALUE [KEY=VALUE...] [--force] [--explicit]  |  cellctl set <cell> <role> [--harness claude|codex|cursor] --model <m>  |  cellctl set <cell> [--kind <k>] [--cockpit <c>] [--harness <h>] [--provider <p>]"

func cmdSet(cell string, args []string) {
	e := newEnvFromProcess()
	d := cellDir(e, cell)
	abs, err := filepath.Abs(d)
	if err != nil {
		die("set: cannot resolve cell directory: %v", err)
	}
	envfile := filepath.Join(abs, "cell.env")

	force, explicit := false, false
	role, harness, model, kind, cockpit, provider := "", "", "", "", "", ""
	var kvs, sugar []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--force":
			force = true
		case "--explicit":
			explicit = true
		case "--harness":
			harness = needFlagValue(args, &i, "--harness needs a value ("+joinPipe(harnessValues)+")")
		case "--model":
			model = needFlagValue(args, &i, "--model needs a value")
		case "--kind":
			kind = needFlagValue(args, &i, "--kind needs a value ("+joinPipe(kindValues)+")")
		case "--cockpit":
			cockpit = needFlagValue(args, &i, "--cockpit needs a value ("+joinPipe(cockpitValues)+")")
		case "--provider":
			provider = needFlagValue(args, &i, "--provider needs a value (kimi|glm, or a name with CELL_PROVIDER_<NAME>_BASE_URL/_TOKEN_ENV in cell.env)")
		default:
			switch {
			case strings.HasPrefix(a, "--"):
				die("set: unknown flag %s", a)
			case strings.Contains(a, "="):
				kvs = append(kvs, a)
			case valueIn(a, knownRoles):
				if role != "" {
					die("set: '%s' — a role was already given ('%s'); only one role-sugar call at a time", a, role)
				}
				role = a
			default:
				die("set: '%s' is not KEY=VALUE (or a role name, with --model and optionally --harness)", a)
			}
		}
	}
	if kind != "" {
		sugar = append(sugar, "CELL_KIND="+kind)
	}
	if cockpit != "" {
		sugar = append(sugar, "CELL_COCKPIT="+cockpit)
	}
	if provider != "" {
		sugar = append(sugar, "CELL_PROVIDER="+provider)
	}
	if role != "" {
		// Role-sugar form: computes the model-pin KEY itself from the ACTIVE harness (the flag
		// given here, else the cell's own CELL_HARNESS) so a codex call writes
		// CODEX_MODEL_<role>, never DESK_MODEL_<role>. With a role, --harness SELECTS the
		// namespace and is not itself persisted.
		if model == "" {
			die("set: '%s' needs --model <m> (role-sugar form: cellctl set <cell> <role> [--harness claude|codex] --model <m>)", role)
		}
		if len(kvs) != 0 {
			die("set: the role-sugar form and KEY=VALUE pairs cannot be combined in one call")
		}
		h := harness
		if h == "" {
			h = "claude"
		}
		if !valueIn(h, harnessValues) {
			die("set: --harness must be one of %s, got '%s'", joinPipe(harnessValues), harness)
		}
		active := harness
		if active == "" {
			active = activeHarnessOf(envfile)
		}
		mvar := "DESK_MODEL_" + underscore(role)
		if active == "codex" {
			mvar = "CODEX_MODEL_" + underscore(role)
		} else if active == "cursor" {
			mvar = "CURSOR_MODEL_" + underscore(role)
		}
		kvs = []string{mvar + "=" + model}
	} else {
		if model != "" {
			die("set: --model needs a role (cellctl set <cell> <role> --model <m>); the harness-wide default is the KEY=VALUE form (DESK_MODEL_DEFAULT / CODEX_MODEL_default)")
		}
		if harness != "" {
			sugar = append(sugar, "CELL_HARNESS="+harness)
		}
	}
	kvs = append(kvs, sugar...)
	pinKind := pinKindDefault
	if explicit {
		pinKind = pinKindExplicit
	}
	applyEnvKVsKind(e, envfile, force, kvs, pinKind)
}
