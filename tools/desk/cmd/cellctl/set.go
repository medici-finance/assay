package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cellEnvKnownKeys is the fixed part of the cell.env key allowlist `cellctl set` recognises
// without --force. The DESK_MODEL_<role>, CODEX_MODEL_<role>, TIER_MODEL_<TIER>_<HARNESS> and
// CELL_PROVIDER_<NAME>_{BASE_URL,TOKEN_ENV,MODEL} families are matched by shape below — a typo'd
// role/tier/provider name is refused rather than silently scaffolding a variable nothing reads.
var cellEnvKnownKeys = strings.Fields(`CELL CELL_KIND CELL_CONTAINER_LAUNCHER CELL_ROOTS CELL_COCKPIT DESKD CELL_FORGE CELL_REPO CELLS_CONFIG
FORGE_API_BASE DESKD_ADDR DESKD_INDEX DESKD_APP_PEM DESKD_APP_ID_VAR ORGS GITLAB_GROUP
GITLAB_API_BASE GITLAB_TOKEN_STORE DESKD_GITLAB_TOKEN_FILE ROLES DESK_MODEL_DEFAULT CODEX_MODEL_default
CELL_HARNESS TMUX_SESSION CELL_PROVIDER CELL_REPO_SLUG CELL_PATH
TIER_MODEL_TOP_CLAUDE TIER_MODEL_MID_CLAUDE TIER_MODEL_FAST_CLAUDE
TIER_MODEL_TOP_CODEX TIER_MODEL_MID_CODEX TIER_MODEL_FAST_CODEX`)

func knownCellEnvKey(k string) bool {
	if valueIn(k, cellEnvKnownKeys) {
		return true
	}
	for _, prefix := range []string{"DESK_MODEL_", "CODEX_MODEL_"} {
		if strings.HasPrefix(k, prefix) {
			r := strings.TrimPrefix(k, prefix)
			for _, w := range strings.Fields(rolesDefault) {
				if r == underscore(w) {
					return true
				}
			}
		}
	}
	if strings.HasPrefix(k, "CELL_PROVIDER_") {
		for _, suf := range []string{"_BASE_URL", "_TOKEN_ENV", "_MODEL"} {
			if strings.HasSuffix(k, suf) && len(k) > len("CELL_PROVIDER_")+len(suf) {
				return true
			}
		}
	}
	return false
}

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
	case "CELL_HARNESS":
		if !valueIn(value, harnessValues) {
			die("set: CELL_HARNESS must be claude or codex, got '%s'", value)
		}
	case "CELL_KIND":
		if !valueIn(value, kindValues) {
			die("set: CELL_KIND must be one of %s, got '%s'", joinPipe(kindValues), value)
		}
	case "CELL_COCKPIT":
		if !valueIn(value, cockpitValues) {
			die("set: CELL_COCKPIT must be one of %s, got '%s'", joinPipe(cockpitValues), value)
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
func validateKindChange(envfile, kind string, kvs []string) {
	need := ""
	switch kind {
	case "container":
		need = "CELL_CONTAINER_LAUNCHER"
	case "house":
		need = "CELL_ROOTS"
	case "scrubbed":
		need = "CELL_REPO_SLUG"
	default:
		return
	}
	v := ""
	for _, kv := range kvs {
		if k, val, ok := splitKV(kv); ok && k == need {
			v = val
		}
	}
	if v == "" {
		v, _ = envFileValue(envfile, need)
	}
	// A line `cellctl new` wrote with %q may be shell-quoted (CELL_ROOTS='' on a container cell
	// is the empty string once sourced, not two characters) — strip one matching pair of quotes
	// so the file-level read agrees with what loadCell would see.
	if len(v) >= 2 && ((v[0] == '\'' && v[len(v)-1] == '\'') || (v[0] == '"' && v[len(v)-1] == '"')) {
		v = v[1 : len(v)-1]
	}
	if v == "" {
		die("set: CELL_KIND=%s needs %s, which is neither set in %s nor given in this call — a %s cell cannot load without it (set both in one call, or %s first); nothing written", kind, need, envfile, kind, need)
	}
	if kind == "container" && (!strings.HasPrefix(v, "/") || !isExecFile(v)) {
		die("set: CELL_KIND=container needs an absolute executable CELL_CONTAINER_LAUNCHER, got '%s'; nothing written", v)
	}
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
}

// setEnvKey is the single-key rewrite/append, after validateEnvKey has already passed. The
// rewrite is line-for-line: an existing `KEY=...` line (not a `#`-commented one) is replaced in
// place so comments and ordering are untouched; a key with no active line is appended.
func setEnvKey(envfile, key, value string, force bool) {
	validateEnvKey(key, value, force)
	raw, err := os.ReadFile(envfile)
	if err != nil {
		die("set: cannot read %s: %v", envfile, err)
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
				lines[i] = key + "=" + value
				replaced = true
			}
		}
	} else {
		lines = append(lines, key+"="+value)
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
	shown := before
	if shown == "" {
		shown = "<unset>"
	}
	fmt.Printf("[set] %s: %s -> %s\n", key, shown, value)
}

// activeHarnessOf is the LAST `CELL_HARNESS=` line in a cell.env, or `claude` when the file
// carries none. File-level, not loadCell, on purpose: `set` edits the file directly and never
// sources it (sourcing an operator-writable cell.env as part of `set` would execute arbitrary
// shell in it), so this reads the one line it needs by hand.
func activeHarnessOf(envfile string) string {
	if v, ok := envFileValue(envfile, "CELL_HARNESS"); ok && v != "" {
		return v
	}
	return "claude"
}

const setUsage = "cellctl set <cell> KEY=VALUE [KEY=VALUE...] [--force]  |  cellctl set <cell> <role> [--harness claude|codex] --model <m>  |  cellctl set <cell> [--kind <k>] [--cockpit <c>] [--harness <h>] [--provider <p>]"

func cmdSet(cell string, args []string) {
	e := newEnvFromProcess()
	d := cellDir(e, cell)
	abs, err := filepath.Abs(d)
	if err != nil {
		die("set: cannot resolve cell directory: %v", err)
	}
	envfile := filepath.Join(abs, "cell.env")

	force := false
	role, harness, model, kind, cockpit, provider := "", "", "", "", "", ""
	var kvs, sugar []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--force":
			force = true
		case "--harness":
			harness = needFlagValue(args, &i, "--harness needs a value (claude|codex)")
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
			die("set: --harness must be claude or codex, got '%s'", harness)
		}
		active := harness
		if active == "" {
			active = activeHarnessOf(envfile)
		}
		mvar := "DESK_MODEL_" + underscore(role)
		if active == "codex" {
			mvar = "CODEX_MODEL_" + underscore(role)
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
	applyEnvKVs(e, envfile, force, kvs)
}
