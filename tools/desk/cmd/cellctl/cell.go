package main

import (
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	harnessValues = []string{"claude", "codex", "cursor"}
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
	// src names the LAYER that last assigned each key (the layer* constants in defaults.go), so
	// `check` and `show` can say where an effective value came from. It is bookkeeping only:
	// no lookup reads it, so it can never change what a key resolves to.
	src map[string]string
}

func newEnvFromProcess() *Env {
	e := &Env{vals: map[string]string{}, set: map[string]bool{}, src: map[string]string{}}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			key := kv[:i]
			if runtime.GOOS == "windows" {
				key = strings.ToUpper(key)
			}
			e.vals[key] = kv[i+1:]
			e.set[key] = true
			e.src[key] = layerProcess
		}
	}
	if runtime.GOOS == "windows" && e.Get("HOME") == "" {
		e.Put("HOME", e.Get("USERPROFILE"))
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

// Put is an assignment cellctl itself makes — a compiled default, a derived value. Re-putting
// the value a key already holds (the `e.Put(k, e.GetOr(k, def))` shape loadCell uses to fill
// defaults) keeps the layer that supplied it; anything else is cellctl's own.
func (e *Env) Put(k, v string) {
	layer := layerCompiled
	if e.set[k] && e.vals[k] == v && e.src[k] != "" {
		layer = e.src[k]
	}
	e.putFrom(k, v, layer)
}

// putFrom assigns k and records the layer the assignment came from.
func (e *Env) putFrom(k, v, layer string) {
	e.vals[k] = v
	e.set[k] = true
	if e.src == nil {
		e.src = map[string]string{}
	}
	e.src[k] = layer
}

// Source is the layer that supplied k's effective value, or layerUnset when nothing set it.
func (e *Env) Source(k string) string {
	if !e.set[k] {
		return layerUnset
	}
	if l := e.src[k]; l != "" {
		return l
	}
	return layerCompiled
}

// parseCellEnv overlays one cell.env file onto e. The oracle SOURCEs the file, so this has to
// honour the shell forms `cellctl new` itself writes — bare values, double-quoted values
// (ROLES="a b c"), and the %q-quoted single-quoted values a container cell carries — plus
// comments and a leading `export`. It deliberately does NOT execute anything: the file is
// operator-writable, and `cellctl set`/`show` in the oracle already refuse to source it for
// exactly that reason.
func parseCellEnv(e *Env, path string) error {
	return parseCellEnvFor(runtime.GOOS, e, path)
}

// parseCellEnvFor is parseCellEnv for an explicit goos, so the Windows reading of a `\` in a
// path (cellenvpath.go) is table-tested on any host.
func parseCellEnvFor(goos string, e *Env, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = overlayEnvLines(goos, e, raw, layerCellEnv, nil, false)
	return err
}

// overlayEnvLines is the ONE reader of the cell.env grammar: it overlays every `KEY=VALUE`
// assignment in raw onto e, in order, recording layer as each key's source, and returns the keys
// it assigned in first-seen order. cell.env and the machine-wide defaults file (defaults.go) both
// go through it, so a value means the same thing in either file.
//
// vet is the strictness switch. nil is cell.env's long-standing reading: a line that is not an
// assignment is skipped, as it always was. A non-nil vet is called with the 1-based line number
// and the key of every assignment ("" for a line that is neither blank, a comment, nor an
// assignment) BEFORE anything on that line is assigned, and its error stops the read.
//
// emptySetsNothing is the machine-wide file's rule (defaults.go): an assignment whose value is
// empty assigns nothing and is not among the keys returned, so the layer below stands. cell.env
// passes false — an empty value there IS an assignment, as it always was.
func overlayEnvLines(goos string, e *Env, raw []byte, layer string, vet func(line int, key string) error, emptySetsNothing bool) ([]string, error) {
	var keys []string
	seen := map[string]bool{}
	for n, line := range strings.Split(string(raw), "\n") {
		s := strings.TrimLeft(line, " \t")
		// A whitespace-only line (a CRLF file's blank line) carries no `=`, so it was always
		// skipped; saying so here keeps the strict reading from calling it malformed.
		if strings.TrimSpace(s) == "" || strings.HasPrefix(s, "#") {
			continue
		}
		s = strings.TrimPrefix(s, "export ")
		i := strings.IndexByte(s, '=')
		key := ""
		if i > 0 && validEnvKeyShape(s[:i]) {
			key = s[:i]
		}
		if vet != nil {
			if err := vet(n+1, key); err != nil {
				return keys, err
			}
		}
		if key == "" {
			continue
		}
		value := unquoteShellValueFor(goos, s[i+1:], e)
		if emptySetsNothing && emptyEnvValue(value) {
			continue
		}
		e.putFrom(key, value, layer)
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// emptyEnvValue is the test the machine-wide file's empty-value rule applies to a value AFTER the
// grammar has reduced it — quotes removed, `$VAR` references expanded — so `KEY=`, a pair of
// quotes of either kind with nothing between them, and `KEY=$UNSET` are all empty. A value that
// is only the carriage return a CRLF line ending leaves behind is empty too: to the person who
// saved the file, that line has no value. Anything else, a single space included, is a value.
func emptyEnvValue(v string) bool { return v == "" || v == "\r" }

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
//
// One deliberate departure, on Windows only: an unquoted `\` before a byte bashQuote never
// escapes there (a letter, a digit, `#%+-./:=@_~` mid-value, a non-ASCII byte) or at the end of
// the value is a path separator and is kept, so `CELL_REPO=C:\src\x` loads as written instead
// of as `C:srcx`, while every bashQuote-written value still loads as quoted
// (cellEnvEscapableFor). Off Windows the rule is bash's.
func unquoteShellValue(s string, e *Env) string {
	return unquoteShellValueFor(runtime.GOOS, s, e)
}

func unquoteShellValueFor(goos, s string, e *Env) string {
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
			if goos == "windows" && (i+1 >= len(s) || !cellEnvEscapableFor(goos, s[i+1], i == 0)) {
				out.WriteByte('\\')
				i++
				continue
			}
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
	Cadence      *cadenceOptions
	cadenceLease *cellcadence.Lease
	Env          *Env

	Name string
	Dir  string
	// Root is the cells root this load resolved, absolute, and Ref the cell's path under it — the
	// name it was loaded by (`demo`, or `team/demo` for a nested cell). The cell directory's
	// parent is the cells root only for a cell one level down, and the root is also where the
	// machine-wide defaults file and the shared provider catalog are read from.
	//
	// The pair comes from reenter, and these are the places that hand it to a later cellctl:
	// the model-policy hook command (--cells-root and the cell directory), the container
	// console, the comms pane, a cadence-supervised role pane, and the environment of both
	// scratch launches (CELLS_ROOT and ASSAY_SCRATCH_CELL). An ordinary role pane carries Ref
	// and not Root: it is started by this process and takes the cells root from the environment
	// it inherits. Two commands carry neither — the deskd stand pane and a scheduled run's
	// precheck name the cell by Name, its CELL= value, and take the cells root from their
	// environment; so do `up`'s own in-process hand-offs to `desk`. For a cell one level under
	// the root whose CELL= is its directory name, all of these are the same cell.
	Root string
	Ref  string

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

	// The machine-wide defaults file (defaults.go): the path looked at, whether a file was there
	// and read, and the keys it set, in file order.
	DefaultsPath string
	DefaultsRead bool
	DefaultsKeys []string
}

// cellsRoot is $CELLS_ROOT with the oracle's own default.
func cellsRoot(e *Env) string {
	if v := e.Get("CELLS_ROOT"); v != "" {
		return v
	}
	xdg := e.Get("XDG_DATA_HOME")
	if xdg == "" {
		xdg = filepath.Join(hostHome(e), ".local", "share")
	}
	return filepath.Join(xdg, "assay", "cells")
}

// cellNameLocal reports whether name names a directory UNDER the cells root: a relative path
// that stays inside the root and is not the root itself. `demo` and `team/demo` are; an absolute
// path, `.`, `..`, `../other/x` and `team/..` are not. The test is on the name as written,
// cleaned — it follows no link, so a cell directory that is a symlink to somewhere else is still
// named by a local name and still loads.
//
// A cell is refused under any other name (cellNameNotLocal), by `new`, by the loader and by
// `set`. That is what lets a cell be addressed by ONE pair — the cells root and its path under
// it — everywhere a later cellctl process is told to load it again: a cell directory outside
// the root would need a second address (its own parent, with a different defaults file and a
// different provider catalog beside it), and every re-entry would have to choose between them.
func cellNameLocal(name string) bool {
	return filepath.IsLocal(name) && filepath.Clean(name) != "."
}

// cellNameNotLocal is the refusal for a name cellNameLocal rejects: the name, then the root.
const cellNameNotLocal = "cell name '%s' is not a path under the cells root %s — a cell is named by its directory under the cells root (demo, team/demo), never by an absolute path, by the root itself, or by a path that leaves the root through '..'"

// cellAddress is the (cells root, name) pair that loads the cell named name again: the root the
// load resolved, made absolute, and the name cleaned — its path under that root. name is one
// cellNameLocal accepted, so the pair names the directory the load read, and no other.
func cellAddress(root, name string) (string, string) {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return root, filepath.Clean(name)
}

// reenter is the (cells root, name) pair that loads this cell again; the Root field says which
// commands are built from it and which are not. A Cell that loadCell did not build — only a
// test's own literal is one — carries no root and is addressed the way every cell was before
// the root was carried: by its directory's parent and its name, the same pair for a cell one
// level under its root.
func (c *Cell) reenter() (root, name string) {
	if c.Root == "" {
		return filepath.Dir(c.Dir), c.Name
	}
	return c.Root, c.Ref
}

func deskToolsBin(e *Env) string {
	if runtime.GOOS == "windows" {
		return e.GetOr("DESK_TOOLS_BIN", filepath.Join(e.Get("LOCALAPPDATA"), "Assay", "bin"))
	}
	return e.GetOr("DESK_TOOLS_BIN", "/opt/desk-tools/bin")
}

// realConfigHome is the OPERATOR's config home — the one holding the App private keys a k8s or
// house cell symlinks to. A scrubbed cell never reads it, by construction.
func realConfigHome(e *Env) string {
	return mustResolve(configHomeFor(runtime.GOOS, e))
}

// cellDir is the directory of the cell named name, which must be a name under the cells root
// (cellNameLocal) with a cell.env in it.
func cellDir(e *Env, name string) string {
	if !cellNameLocal(name) {
		die(cellNameNotLocal, name, cellsRoot(e))
	}
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
	c.Root, c.Ref = cellAddress(cellsRoot(e), name)
	// The machine-wide defaults go on BEFORE the cell's own file, so cell.env overrides them key
	// by key; they go on AFTER the process environment, as cell.env does. A file that is not
	// there sets nothing; one that is there and unusable stops here, before any verb acts.
	c.DefaultsPath = cellDefaultsPath(e)
	if c.DefaultsKeys, c.DefaultsRead, err = overlayCellDefaults(e, c.DefaultsPath); err != nil {
		die("%v", err)
	}
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
		if cfg := e.Get("CELL_CONTAINER_CONFIG"); cfg != "" {
			if l != "" {
				die("set only CELL_CONTAINER_CONFIG or CELL_CONTAINER_LAUNCHER, not both")
			}
			if !filepath.IsAbs(cfg) || !isRegular(cfg) {
				die("CELL_CONTAINER_CONFIG must be an absolute configuration file")
			}
		} else if err := cellPathCheck(runtime.GOOS, l); err != nil {
			die("container launcher: %v", err)
		} else if !isExecFile(l) {
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
		die("cell.env: CELL_HARNESS=%s is not a known harness (%s)", c.Harness, joinPipe(harnessValues))
	}
	if c.Harness == "cursor" && c.Kind != "house" {
		die("cursor currently requires a house cell; %s is unsupported", c.Kind)
	}

	// Model TIER map compiled defaults (#986). `-` (not `:-`) on purpose: cell.env can set one
	// of these to the EMPTY string to deliberately REMOVE an entry, which is the shape a
	// fixture uses to reproduce the no-pin-no-tier-match case `check` must surface as a MISS.
	for k, def := range tierModelDefaults {
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
	if runtime.GOOS == "windows" && filepath.Ext(p) == "" {
		p += ".exe"
	}
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Ext(p), ".exe")
	}
	return st.Mode()&0o111 != 0
}

func isGitCheckout(p string) bool {
	cmd := exec.Command("git", "-C", p, "rev-parse", "--git-dir")
	cmd.Stdout, cmd.Stderr = nil, nil
	return cmd.Run() == nil
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
		if err := cellPathCheck(runtime.GOOS, path); err != nil {
			fmt.Fprintf(os.Stderr, "malformed CELL_ROOTS path: %v\n", err)
			return false
		}
		parts := strings.Split(name, "/")
		bad := len(parts) != 2 || parts[0] == "" || parts[1] == "" ||
			strings.ContainsAny(name, " \t=") || cellPathCheck(runtime.GOOS, path) != nil || strings.Contains(path, ",")
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
