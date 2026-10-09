package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// The machine-wide cell defaults file: `<cells root>/defaults.env`, in cell.env's own grammar,
// read by every verb that loads a cell. It exists so a setting every cell on a machine should
// share (CELL_GO_CACHE=on is the motivating one) is written once rather than once per cell.
//
// Order, lowest to highest: cellctl's compiled default, the process environment, this file, the
// cell's own cell.env. So a cell overrides the machine, and the machine overrides the shell —
// the same relation cell.env has always had to the shell, one step further out.
//
// Three rules keep the file from becoming a second, quieter place to configure a cell:
//
//   - Absent means absent. No file changes nothing, prints nothing and is never an error.
//   - A file that is there is read strictly. One that cannot be read, a line that is not an
//     assignment, or a key in cellDefaultsRefused stops the verb and names the file.
//   - It is never written by cellctl. `cellctl set` writes the cell's own cell.env only.
//
// The file applies to every cell kind. It adds no path into a scrubbed or container launch that
// the process environment did not already have: both kinds compose their child environment from
// a fixed list of keys (scrubbedEnvKeys, containerRun), each read through the same Env this file
// is overlaid onto, and the per-kind refusals of host model policy still hold whichever layer
// set the key.
const cellDefaultsFile = "defaults.env"

// The layers an effective value can come from, as Env.Source reports them.
const (
	layerProcess  = "process environment"
	layerDefaults = "defaults file"
	layerCellEnv  = "cell.env"
	layerCompiled = "compiled default"
	layerUnset    = "unset"
)

// cellDefaultsRefused is every key the defaults file may not set, with the reason the refusal
// prints. Each one names or scopes ONE cell, or binds a resource one cell holds; a machine-wide
// value would be wrong for every cell but one, or would move every cell at once.
var cellDefaultsRefused = []struct{ key, why string }{
	// What a cell is.
	{"CELL", "names one cell"},
	{"CELL_KIND", "is one cell's kind, with that kind's own preconditions"},
	{"CELL_KIND_OVERRIDE_INTERNAL", "carries one invocation's --kind and would re-kind every cell on every run"},
	{"ROLES", "is one cell's set of role windows"},
	// What a cell is scoped to.
	{"CELL_REPO", "is the one checkout a cell's role worktrees are created from"},
	{"CELL_REPO_SLUG", "is the one repository a scrubbed cell is scoped to"},
	{"CELL_ROOTS", "is one cell's stream-root map, the scope its desks read"},
	{"CELLS_CONFIG", "is one cell's own cells.yaml slice"},
	{"CELL_COMMS_CONFIG", "is a comms manifest that names the single cell it belongs to"},
	// Where a cell lives, and what it holds while running.
	{"CELLS_ROOT", "locates this file and every cell, and is resolved before the file is read"},
	{"DESKD", "decides whether one cell runs its own deskd, which a container or scrubbed cell refuses outright"},
	{"DESKD_ADDR", "is the address one cell's deskd listens on"},
	{"DESKD_INDEX", "is one cell's deskd index"},
	{"TMUX_SESSION", "is one cell's session name, which `down` stops"},
	{"CELL_GO_CACHE_ROOT", "is a cache root marked for exactly one cell; a second cell is refused it"},
	{"CELL_CONTAINER_CONFIG", "binds one cell to its container definition"},
	{"CELL_CONTAINER_LAUNCHER", "binds one cell to its container launcher"},
	// A cell's forge binding: which forge its credentials are minted for and sent to.
	{"CELL_FORGE", "is one cell's forge"},
	{"GITHUB_HOST", "is the forge host one cell's credentials are sent to"},
	{"FORGE_API_BASE", "is the forge endpoint one cell's credentials are sent to"},
	{"GITLAB_API_BASE", "is the forge endpoint one cell's credentials are sent to"},
	{"GITLAB_GROUP", "is the group one cell reads"},
	{"GITLAB_TOKEN_STORE", "is one cell's token store"},
	{"DESKD_GITLAB_TOKEN_FILE", "is one cell's deskd read token"},
	{"DESKD_APP_PEM", "is one cell's deskd read key"},
	{"DESKD_APP_ID_VAR", "names the App id one cell's deskd mints with"},
	{"ORGS", "is the set of organisations one cell mints tokens for"},
}

func cellDefaultsRefusal(key string) (string, bool) {
	for _, r := range cellDefaultsRefused {
		if r.key == key {
			return r.why, true
		}
	}
	return "", false
}

// cellDefaultsPath is the defaults file loadCell reads: defaults.env in the cells root.
func cellDefaultsPath(e *Env) string {
	p := filepath.Join(cellsRoot(e), cellDefaultsFile)
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// cellDefaultsFor is the same file, reached from a cell.env's own path — which is all `set`'s
// readers are handed. A cell.env that lives under the cells root, at any depth (a cell may be
// named `team/demo`), resolves to the root's one file, exactly as loadCell does. A cell.env
// anywhere else is not a cell loadCell could load from this environment; it resolves to the file
// beside its own cell directory, so the answer never depends on a cells root it is not in.
func cellDefaultsFor(envfile string) string {
	if abs, err := filepath.Abs(envfile); err == nil {
		envfile = abs
	}
	inRoot := cellDefaultsPath(newEnvFromProcess())
	if rel, err := filepath.Rel(filepath.Dir(inRoot), envfile); err == nil && filepath.IsLocal(rel) {
		return inRoot
	}
	return filepath.Join(filepath.Dir(filepath.Dir(envfile)), cellDefaultsFile)
}

// overlayCellDefaults overlays the defaults file at path onto e and returns the keys it set.
// read is false, with no error, when there is no file — the one case that changes nothing.
//
// The file is opened the way the other shared file in the cells root is (readPolicySource):
// without blocking, and only when it is a regular file, so a FIFO or a directory left at the
// path is a refusal rather than a hang or an empty read. A symlink is followed, as it is for
// cell.env and providers.json; one whose target is missing is a file that is present and cannot
// be read, so it refuses instead of silently dropping the machine's defaults.
func overlayCellDefaults(e *Env, path string) (keys []string, read bool, err error) {
	raw, err := readPolicySource(path)
	if err != nil {
		if os.IsNotExist(err) {
			if _, lerr := os.Lstat(path); lerr != nil {
				return nil, false, nil
			}
		}
		return nil, false, fmt.Errorf("cannot read cell defaults file %s: %v", path, err)
	}
	keys, err = overlayEnvLines(runtime.GOOS, e, raw, layerDefaults, func(line int, key string) error {
		if key == "" {
			return fmt.Errorf("cell defaults file %s: line %d is not a KEY=VALUE assignment (a comment starts with #)", path, line)
		}
		if why, refused := cellDefaultsRefusal(key); refused {
			return fmt.Errorf("cell defaults file %s: line %d sets %s, which %s — it cannot be a machine-wide default; set it in the cell's own cell.env", path, line, key, why)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return keys, true, nil
}

// goCacheSupply says which layer supplied CELL_GO_CACHE, for `check`'s managed-cache row. An
// empty value is the same as no value everywhere the key is read, so it is reported as unset.
func (c *Cell) goCacheSupply() string {
	v := c.Env.Get("CELL_GO_CACHE")
	if v == "" {
		return "CELL_GO_CACHE unset"
	}
	return fmt.Sprintf("CELL_GO_CACHE=%s from %s", v, c.Env.Source("CELL_GO_CACHE"))
}

// checkCellDefaults is `check`'s account of the machine-wide layer: whether a defaults file was
// read, and what the managed Go cache resolves to and which layer decided it.
//
// It prints nothing at all for a cell with no defaults file and no CELL_GO_CACHE anywhere — the
// same rule the other opt-in rows follow — so a machine that uses neither sees the `check` it
// always saw. Neither row can MISS: an unusable cache setting is a warn here and a refusal at
// launch, which is where it was refused before this row existed.
func (c *Cell) checkCellDefaults(k *checker) {
	if !c.DefaultsRead && c.Env.Get("CELL_GO_CACHE") == "" {
		return
	}
	if c.DefaultsRead {
		keys := "it sets no keys"
		if n := len(c.DefaultsKeys); n > 0 {
			keys = fmt.Sprintf("%d key(s): %s", n, strings.Join(c.DefaultsKeys, " "))
		}
		k.chk(true, "cell defaults: read %s (%s; this cell's cell.env overrides each)", c.DefaultsPath, keys)
	} else {
		k.na("cell defaults: none read — no %s", c.DefaultsPath)
	}
	supply := c.goCacheSupply()
	if c.Kind == "container" {
		k.na("managed Go cache: not applied to a container cell — its own runtime configures its caches (%s)", supply)
		return
	}
	p, err := c.cachePolicy()
	switch {
	case err != nil:
		k.warn("managed Go cache: unusable — %v (%s); a launch refuses on it", err, supply)
	case p != nil:
		k.chk(true, "managed Go cache: on (%s)", supply)
	default:
		k.na("managed Go cache: off (%s)", supply)
	}
}
