package main

import (
	"errors"
	"fmt"
	"io/fs"
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
//   - Absent means absent. No file is never an error and sets no key: every value is what the
//     three other layers make it. (`check` gained one pair of rows with this file, and prints
//     them for a cell with no file only when CELL_GO_CACHE is set — see checkCellDefaults.)
//   - A file that is there is read strictly. One that cannot be read, a line that is not an
//     assignment, or a key in cellDefaultsRefused stops the verb and names the file.
//   - cellctl writes it once and never again. `cellctl new` and `cellctl defaults init` create
//     it from the template when there is none — every settable key commented out, so the new
//     file sets nothing — and nothing ever rewrites, appends to or replaces a file that is
//     there. `cellctl set` writes the cell's own cell.env only.
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
// prints. It is drawn from the key registry (envkeys.go), and it is every key cellctl reads that
// is not a machine-wide lever: the per-cell keys, each of which names or scopes ONE cell or binds
// a resource one cell holds, so a machine-wide value would be wrong for every cell but one; the
// switches that belong to one run of one command; and the locations and host variables cellctl
// follows from the launching shell — the cells root this very file is found in among them — which
// a file that outranks the shell must not be able to move.
var cellDefaultsRefused = envKeyRefusals()

// cellDefaultsRefusal is why the defaults file refuses key, and where the key goes instead.
func cellDefaultsRefusal(key string) (why, instead string, refused bool) {
	for _, r := range cellDefaultsRefused {
		if r.key != key {
			continue
		}
		k, _ := envKeyLookup(key)
		switch {
		case k.class == envCell:
			instead = "set it in the cell's own cell.env"
		case k.from == envFromRun:
			instead = "give it in the environment of the command it is for"
		case k.from == envInternal:
			instead = "it is not configuration"
		default:
			instead = "cellctl takes it from the environment of the command that runs it"
		}
		return r.why, instead, true
	}
	return "", "", false
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
// anywhere else is not one `set` can reach — it finds its cell.env through cellDir, which refuses
// a name that is not under the cells root — so that case arises only for a caller that hands
// these readers a file directly, and it resolves to the file beside that file's own cell
// directory: the answer never depends on a cells root the file is not in.
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
// read is false, with no error, when there is no file — and then e is as it was.
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
	keys, err = overlayCellDefaultsText(runtime.GOOS, e, raw, path)
	if err != nil {
		return nil, false, err
	}
	return keys, true, nil
}

// overlayCellDefaultsText is the strict reading of a defaults file's text: every line is blank,
// a comment or an assignment, and no assignment sets a key the file refuses. path only names the
// file in a refusal.
//
// An assignment whose value is empty sets nothing, for every key (emptyEnvValue says what empty
// is). This file outranks the process environment, so a line that assigned the empty string
// would take a value the launching shell exported away from every cell under the root, and the
// template prints exactly such a line — `# KEY=` — for every key with no compiled default: for
// CELL_MODEL_POLICY, uncommenting it unchanged would have meant "no policy" on every cell and no
// enforcement hook. So the line is read, vetted like any other (a refused key is refused
// whatever its value), and then assigns nothing: the key is not among the keys the file set,
// and its value and source stay those of the layer below. Only a cell's own cell.env can set a
// key to the empty string.
func overlayCellDefaultsText(goos string, e *Env, raw []byte, path string) ([]string, error) {
	return overlayEnvLines(goos, e, raw, layerDefaults, func(line int, key string) error {
		if key == "" {
			return fmt.Errorf("cell defaults file %s: line %d is not a KEY=VALUE assignment (a comment starts with #)", path, line)
		}
		if why, instead, refused := cellDefaultsRefusal(key); refused {
			return fmt.Errorf("cell defaults file %s: line %d sets %s, which %s — it cannot be a machine-wide default; %s", path, line, key, why, instead)
		}
		return nil
	}, true)
}

// createCellDefaults writes the template to path when nothing is there, and reports whether it
// did. A file, a directory or a symlink already at the path — whatever it holds, readable or
// not — is left exactly as it is: the create is exclusive, so there is no window in which an
// existing file could be opened for writing.
//
// It creates the file and nothing else. The cells root is `cellctl new`'s to create, with the
// mode `new` gives it; a root that is not there is an error here, not a directory to make.
func createCellDefaults(path string) (created bool, err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, nil
		}
		return false, err
	}
	werr := writeCellDefaultsTemplate(f)
	cerr := f.Close()
	if werr == nil {
		werr = cerr
	}
	if werr != nil {
		// A write that failed part-way leaves no file: half a template would be a defaults file
		// that is there, so it would never be written again, and one cut off inside a line could
		// refuse every cell under the root. Only ever the file this call created a moment ago.
		os.Remove(path)
		return false, werr
	}
	return true, nil
}

// writeCellDefaultsTemplate writes the template into the file createCellDefaults has just
// created. It is a variable for one reason: a test replaces it with a write that fails part-way,
// which no real file system can be asked to do on demand.
var writeCellDefaultsTemplate = func(f *os.File) error {
	_, err := f.WriteString(cellDefaultsTemplate())
	return err
}

const cellDefaultsCreated = "[defaults] created %s; every key in it is commented out, so it sets nothing until you uncomment a line\n"

// seedCellDefaults is `cellctl new`'s last step: a cells root that has no defaults file gets the
// template, once. It says one line when it writes the file, nothing when a file is already
// there, and a notice when what is there is not a regular file. A root it cannot write to is a
// notice, never a failed `new` — the cell is already scaffolded, and the file is optional.
func seedCellDefaults() {
	path := cellDefaultsPath(newEnvFromProcess())
	created, err := createCellDefaults(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NOTICE: no machine-wide defaults file written at %s: %v (`cellctl defaults init` writes it later)\n", path, err)
		return
	}
	if created {
		fmt.Printf(cellDefaultsCreated, path)
		return
	}
	// Something was already at the path, and it is left as it is. A file is the ordinary case
	// and needs no word. Anything else — a directory, a symlink to nothing — is read by no cell:
	// the loader refuses it, so the cell just scaffolded will not load until it is dealt with.
	if fi, err := os.Stat(path); err != nil || !fi.Mode().IsRegular() {
		fmt.Fprintf(os.Stderr, "NOTICE: %s is there and is not a regular file, so no cell under this cells root loads until it is replaced by one or removed (`cellctl defaults init` writes the template once nothing is there)\n", path)
	}
}

// cmdDefaults is `cellctl defaults init|print`: the two acts on the template that do not belong
// to a cell. `print` writes the template to stdout — the list of every key and its compiled
// default, and what to diff an existing file against. `init` creates the file for a cells root
// that predates it, on the same never-overwrite rule `cellctl new` follows.
func cmdDefaults(args []string) {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		usage(0)
	}
	if len(args) != 1 || (args[0] != "init" && args[0] != "print") {
		die("usage: cellctl defaults init|print (init creates CELLS_ROOT/%s without overwriting; print writes the template to stdout)", cellDefaultsFile)
	}
	if args[0] == "print" {
		fmt.Print(cellDefaultsTemplate())
		return
	}
	path := cellDefaultsPath(newEnvFromProcess())
	created, err := createCellDefaults(path)
	if errors.Is(err, fs.ErrNotExist) {
		die("cannot create %s: there is no cells root at %s (`cellctl new` creates the root with its first cell, and writes this file with it)", path, filepath.Dir(path))
	}
	if err != nil {
		die("cannot create %s: %v", path, err)
	}
	if !created {
		die("cannot create %s: it is already there, and an existing defaults file is never overwritten (`cellctl defaults print` prints the template to compare it with)", path)
	}
	fmt.Printf(cellDefaultsCreated, path)
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
