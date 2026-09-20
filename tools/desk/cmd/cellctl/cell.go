package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// rolesDefault, houseVerbs, deskToolsBin and realConfigHome are the host-layout constants the
// port carries over from the oracle UNCHANGED (tools/cellctl/cellctl:173-179).
const rolesDefault = "the-desk pr-review-desk verify-desk intake-desk worker-desk"

// houseVerbs is the desk-verb set a house (and scrubbed) cell's `check` proves is installed: the
// boot/roster pair every role needs, the board/dispatch/worktree verbs the loops call, and the
// write verbs the roles post with.
const houseVerbs = "deskboot deskroster deskwt deskboard deskdispatch deskpr deskfile deskpost desktoken"

// The value sets a per-run --kind / --cockpit / --harness flag (and the matching cell.env key)
// may take. One list each, read by the validators, the flag parsers and `show`, so a new value
// is added in exactly one place.
var (
	kindValues    = []string{"k8s", "house", "container", "scrubbed"}
	cockpitValues = []string{"auto", "tmux", "herdr", "orca"}
	harnessValues = []string{"claude", "codex"}
	knownRoles    = []string{"intake-desk", "worker-desk", "pr-review-desk", "verify-desk", "the-desk"}
)

func valueIn(v string, set []string) bool {
	for _, w := range set {
		if v == w {
			return true
		}
	}
	return false
}

func joinPipe(set []string) string { return strings.Join(set, "|") }

// Env is the variable environment a verb resolves against: the process environment with the
// cell.env assignments overlaid, exactly as bash's `set -a; source cell.env; set +a` leaves it.
// Presence is tracked separately from value because the oracle distinguishes the two forms
// deliberately — `${TIER_MODEL_TOP_CLAUDE-fable}` honours a key SET TO EMPTY (the shape a
// fixture uses to reproduce the no-pin-no-tier-match case) while `${CELL_ROOTS:-}` does not.
type Env struct {
	vals map[string]string
	set  map[string]bool
}

func newEnvFromProcess() *Env {
	e := &Env{vals: map[string]string{}, set: map[string]bool{}}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			e.vals[kv[:i]] = kv[i+1:]
			e.set[kv[:i]] = true
		}
	}
	return e
}

// Get is `${KEY:-}` — the value, or "" when unset OR set to the empty string.
func (e *Env) Get(k string) string { return e.vals[k] }

// IsSet is the `-` (not `:-`) test: true when the key exists at all, empty value included.
func (e *Env) IsSet(k string) bool { return e.set[k] }

// GetOr is `${KEY:-default}`.
func (e *Env) GetOr(k, def string) string {
	if v := e.vals[k]; v != "" {
		return v
	}
	return def
}

// GetOrSet is `${KEY-default}`: the default applies only when the key is entirely absent.
func (e *Env) GetOrSet(k, def string) string {
	if e.set[k] {
		return e.vals[k]
	}
	return def
}

func (e *Env) Put(k, v string) {
	e.vals[k] = v
	e.set[k] = true
}

// parseCellEnv overlays one cell.env file onto e. The oracle SOURCEs the file, so this has to
// honour the shell forms `cellctl new` itself writes — bare values, double-quoted values
// (ROLES="a b c"), and the %q-quoted single-quoted values a container cell carries — plus
// comments and a leading `export`. It deliberately does NOT execute anything: the file is
// operator-writable, and `cellctl set`/`show` in the oracle already refuse to source it for
// exactly that reason.
func parseCellEnv(e *Env, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		s := strings.TrimLeft(line, " \t")
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		s = strings.TrimPrefix(s, "export ")
		i := strings.IndexByte(s, '=')
		if i <= 0 {
			continue
		}
		key := s[:i]
		if !validEnvKeyShape(key) {
			continue
		}
		e.Put(key, unquoteShellValue(s[i+1:], e))
	}
	return nil
}

func validEnvKeyShape(k string) bool {
	if k == "" {
		return false
	}
	for i := 0; i < len(k); i++ {
		c := k[i]
		ok := c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9' && i > 0)
		if !ok {
			return false
		}
	}
	return true
}

// unquoteShellValue reduces one right-hand side to the value bash would assign. Single quotes
// are literal; double quotes honour \\, \", \$ and \` and expand $VAR/${VAR} against what has
// been assigned so far; an unquoted value expands the same way. A trailing inline comment is
// NOT stripped — bash does not strip one in an assignment either.
func unquoteShellValue(s string, e *Env) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		switch s[i] {
		case '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				out.WriteString(s[i+1:])
				return out.String()
			}
			out.WriteString(s[i+1 : i+1+j])
			i += j + 2
		case '"':
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("\\\"$`", s[i+1]) >= 0 {
					out.WriteByte(s[i+1])
					i += 2
					continue
				}
				if s[i] == '$' {
					n, v := expandVar(s[i:], e)
					out.WriteString(v)
					i += n
					continue
				}
				out.WriteByte(s[i])
				i++
			}
			i++
		case '\\':
			if i+1 < len(s) {
				out.WriteByte(s[i+1])
				i += 2
			} else {
				i++
			}
		case '$':
			n, v := expandVar(s[i:], e)
			out.WriteString(v)
			i += n
		default:
			out.WriteByte(s[i])
			i++
		}
	}
	return out.String()
}

// expandVar consumes one $VAR or ${VAR} reference from the head of s and returns how many bytes
// it consumed plus the value. An unrecognised shape consumes the single '$' and yields it back,
// which is what bash does for a lone dollar.
func expandVar(s string, e *Env) (int, string) {
	if len(s) < 2 {
		return 1, "$"
	}
	if s[1] == '{' {
		j := strings.IndexByte(s, '}')
		if j < 0 {
			return 1, "$"
		}
		name := s[2:j]
		if !validEnvKeyShape(name) {
			return 1, "$"
		}
		return j + 1, e.Get(name)
	}
	j := 1
	for j < len(s) {
		c := s[j]
		if c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9' && j > 1) {
			j++
			continue
		}
		break
	}
	if j == 1 {
		return 1, "$"
	}
	return j, e.Get(s[1:j])
}

// Cell is everything `load_cell` leaves in scope: the resolved directory, the kind/forge
// defaults it asserts, and the variable environment every later step reads.
type Cell struct {
	Env *Env

	Name       string
	Dir        string
	Home       string
	Config     string
	CellsCfg   string
	DeskdAddr  string
	DeskdIndex string

	Kind         string
	KindOverride string
	Deskd        string
	Forge        string
	ForgeAPIBase string
	GitHubHost   string

	Roles   []string
	Harness string
	Session string
	Repo    string
}

// cellsRoot is $CELLS_ROOT with the oracle's own default.
func cellsRoot(e *Env) string {
	if v := e.Get("CELLS_ROOT"); v != "" {
		return v
	}
	xdg := e.Get("XDG_DATA_HOME")
	if xdg == "" {
		xdg = filepath.Join(e.Get("HOME"), ".local", "share")
	}
	return filepath.Join(xdg, "assay", "cells")
}

func deskToolsBin(e *Env) string { return e.GetOr("DESK_TOOLS_BIN", "/opt/desk-tools/bin") }

// realConfigHome is the OPERATOR's config home — the one holding the App private keys a k8s or
// house cell symlinks to. A scrubbed cell never reads it, by construction.
func realConfigHome(e *Env) string {
	if v := e.Get("ASSAY_CONFIG_HOME"); v != "" {
		return v
	}
	return filepath.Join(e.Get("HOME"), ".config", "assay")
}

func cellDir(e *Env, name string) string {
	d := filepath.Join(cellsRoot(e), name)
	if st, err := os.Stat(filepath.Join(d, "cell.env")); err != nil || st.IsDir() {
		die("no cell '%s' under %s (cell.env missing)", name, cellsRoot(e))
	}
	return d
}

// loadCell is the port of `load_cell`: read cell.env into the environment, then assert every
// per-kind and per-forge precondition and fill every compiled default. A refusal here is the
// same refusal, with the same text, as the oracle's.
func loadCell(name string) *Cell {
	e := newEnvFromProcess()
	c := &Cell{Env: e}
	d := cellDir(e, name)
	abs, err := filepath.Abs(d)
	if err != nil {
		die("cannot resolve cell directory: %v", err)
	}
	c.Dir = abs
	if err := parseCellEnv(e, filepath.Join(c.Dir, "cell.env")); err != nil {
		die("cannot read %s/cell.env: %v", c.Dir, err)
	}
	c.Name = e.GetOr("CELL", name)
	e.Put("CELL", c.Name)
	c.Home = filepath.Join(c.Dir, "home")
	// The cell config-home is a FIXED relative path: the desk binaries resolve their config as
	// $HOME/.config/assay, and the shims are what point $HOME at the cell.
	c.Config = filepath.Join(c.Home, ".config", "assay")
	c.CellsCfg = e.GetOr("CELLS_CONFIG", filepath.Join(c.Dir, "cells-"+c.Name+".yaml"))
	c.DeskdAddr = e.GetOr("DESKD_ADDR", "127.0.0.1:8787")
	c.DeskdIndex = e.GetOr("DESKD_INDEX", filepath.Join(c.Dir, "index", "index.db"))

	c.Kind = e.GetOr("CELL_KIND", "k8s")
	if ko := e.Get("CELL_KIND_OVERRIDE_INTERNAL"); ko != "" {
		c.Kind = ko
		c.KindOverride = ko
	}
	switch c.Kind {
	case "k8s":
		c.Deskd = e.GetOr("DESKD", "1")
	case "container":
		if e.GetOr("DESKD", "0") != "0" {
			die("container cells do not run host deskd")
		}
		c.Deskd = "0"
		l := e.Get("CELL_CONTAINER_LAUNCHER")
		if !strings.HasPrefix(l, "/") || !isExecFile(l) {
			die("container cell needs an absolute executable CELL_CONTAINER_LAUNCHER")
		}
	case "house":
		c.Deskd = e.GetOr("DESKD", "0")
		if e.Get("CELL_ROOTS") == "" {
			die("cell.env: CELL_ROOTS (the <owner>/<repo>=<abs path>,... stream-root map) is not set — a house cell needs it")
		}
	case "scrubbed":
		if e.GetOr("DESKD", "0") != "0" {
			die("scrubbed cells do not run host deskd")
		}
		c.Deskd = "0"
		if e.Get("CELL_REPO") == "" {
			die("cell.env: CELL_REPO (the checkout the harness runs against) is not set")
		}
		if !isGitCheckout(e.Get("CELL_REPO")) {
			die("cell.env: CELL_REPO is not a git checkout: %s", e.Get("CELL_REPO"))
		}
		if e.Get("CELL_REPO_SLUG") == "" {
			die("cell.env: CELL_REPO_SLUG (<owner>/<repo>) is not set — a scrubbed cell is scoped to one repo")
		}
	default:
		die("cell.env: CELL_KIND=%s is not a known kind (k8s|house|container|scrubbed)", c.Kind)
	}

	// Forge awareness. The endpoint is DERIVED from the forge, never a hardcoded host, so no
	// verb below spells a host literal.
	c.Forge = e.GetOr("CELL_FORGE", "github")
	c.GitHubHost = e.GetOr("GITHUB_HOST", "github.com")
	e.Put("GITHUB_HOST", c.GitHubHost)
	switch c.Forge {
	case "github":
		c.ForgeAPIBase = e.GetOr("FORGE_API_BASE", "https://api."+c.GitHubHost)
	case "gitlab":
		base := e.GetOr("FORGE_API_BASE", e.GetOr("GITLAB_API_BASE", "https://gitlab.com/api/v4"))
		c.ForgeAPIBase = base
		e.Put("GITLAB_API_BASE", e.GetOr("GITLAB_API_BASE", base))
		store := e.GetOr("GITLAB_TOKEN_STORE", c.Config)
		e.Put("GITLAB_TOKEN_STORE", store)
		e.Put("DESKD_GITLAB_TOKEN_FILE", e.GetOr("DESKD_GITLAB_TOKEN_FILE", filepath.Join(store, "gitlab-deskd.token")))
	default:
		die("cell.env: CELL_FORGE=%s is not a known forge (github|gitlab)", c.Forge)
	}
	e.Put("FORGE_API_BASE", c.ForgeAPIBase)

	rolesStr := e.Get("ROLES")
	if rolesStr == "" {
		if c.Kind == "container" {
			rolesStr = "the-desk"
		} else {
			rolesStr = rolesDefault
		}
	}
	c.Roles = strings.Fields(rolesStr)
	e.Put("ROLES", rolesStr)

	e.Put("DESK_MODEL_DEFAULT", e.GetOr("DESK_MODEL_DEFAULT", "sonnet"))

	c.Harness = e.GetOr("CELL_HARNESS", "claude")
	if !valueIn(c.Harness, harnessValues) {
		die("cell.env: CELL_HARNESS=%s is not a known harness (claude|codex)", c.Harness)
	}

	// Model TIER map compiled defaults (#986). `-` (not `:-`) on purpose: cell.env can set one
	// of these to the EMPTY string to deliberately REMOVE an entry, which is the shape a
	// fixture uses to reproduce the no-pin-no-tier-match case `check` must surface as a MISS.
	for k, def := range map[string]string{
		"TIER_MODEL_TOP_CLAUDE":  "fable",
		"TIER_MODEL_MID_CLAUDE":  "sonnet",
		"TIER_MODEL_FAST_CLAUDE": "haiku",
		"TIER_MODEL_TOP_CODEX":   "gpt-5.6-terra",
		"TIER_MODEL_MID_CODEX":   "gpt-5.6-terra",
		"TIER_MODEL_FAST_CODEX":  "gpt-5.6-terra",
	} {
		e.Put(k, e.GetOrSet(k, def))
	}

	c.Session = e.GetOr("TMUX_SESSION", c.Name+"-cell")
	c.Repo = e.Get("CELL_REPO")
	if c.Repo == "" {
		die("cell.env: CELL_REPO (the checkout roles worktree from) is not set")
	}
	return c
}

func isExecFile(p string) bool {
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return false
	}
	return st.Mode()&0o111 != 0
}

func isGitCheckout(p string) bool {
	cmd := exec.Command("git", "-C", p, "rev-parse", "--git-dir")
	cmd.Stdout, cmd.Stderr = nil, nil
	return cmd.Run() == nil
}

// prescanKindOverride applies a `--kind <k>` given anywhere in a verb's arguments BEFORE
// loadCell runs: loadCell is where the kind's own preconditions are asserted and where every
// kind-dependent default is set, so the override has to be in force by then. An unknown value is
// refused here, before anything is loaded.
func prescanKindOverride(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] != "--kind" {
			continue
		}
		if i+1 >= len(args) || args[i+1] == "" {
			die("--kind needs a value (%s)", joinPipe(kindValues))
		}
		if !valueIn(args[i+1], kindValues) {
			die("--kind must be one of %s, got '%s'", joinPipe(kindValues), args[i+1])
		}
		return args[i+1]
	}
	return ""
}

// loadCellWithKind is loadCell plus a per-run kind override. Only THIS process may set it — a
// value inherited from a launching shell (a nested cellctl inside a booted window) must never
// re-kind a cell silently, which is why the override travels as a parameter and not as an
// environment variable the child could inherit.
func loadCellWithKind(name, kindOverride string) *Cell {
	if kindOverride != "" {
		os.Setenv("CELL_KIND_OVERRIDE_INTERNAL", kindOverride)
		defer os.Unsetenv("CELL_KIND_OVERRIDE_INTERNAL")
	}
	return loadCell(name)
}

// rootsValid checks the SHAPE of a CELL_ROOTS value — every entry `<owner>/<repo>=<abs path>` —
// and prints the first malformed entry on stderr. Existence of the paths is `check`'s job.
func rootsValid(v string) bool {
	for _, entry := range strings.Fields(strings.ReplaceAll(v, ",", " ")) {
		eq := strings.IndexByte(entry, '=')
		if eq < 0 {
			fmt.Fprintf(os.Stderr, "malformed CELL_ROOTS entry '%s' (want <owner>/<repo>=<abs path>)\n", entry)
			return false
		}
		name, path := entry[:eq], entry[eq+1:]
		parts := strings.Split(name, "/")
		bad := len(parts) != 2 || parts[0] == "" || parts[1] == "" ||
			strings.ContainsAny(name, " \t=") || !strings.HasPrefix(path, "/") || strings.Contains(path, ",")
		if bad {
			fmt.Fprintf(os.Stderr, "malformed CELL_ROOTS entry '%s' (want <owner>/<repo>=<abs path>)\n", entry)
			return false
		}
	}
	return true
}

// rootEntries splits a CELL_ROOTS value into its `<name>`, `<path>` pairs, in order.
func rootEntries(v string) [][2]string {
	var out [][2]string
	for _, entry := range strings.Fields(strings.ReplaceAll(v, ",", " ")) {
		eq := strings.IndexByte(entry, '=')
		if eq < 0 {
			out = append(out, [2]string{entry, ""})
			continue
		}
		out = append(out, [2]string{entry[:eq], entry[eq+1:]})
	}
	return out
}

func sortedKeys[T any](m map[string]T) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// ghConfigRelPath is the GitHub CLI's config DIRECTORY, relative to a home — a path segment the
// cell links and points GH_CONFIG_DIR at, never a command this program runs. This package
// invokes no forge CLI at all; it is spelled as the whole relative path, in one place, so the
// bare binary name never appears as a call argument where the forge-CLI ban would have to decide
// whether a directory component is an invocation.
const ghConfigRelPath = ".config/gh"
