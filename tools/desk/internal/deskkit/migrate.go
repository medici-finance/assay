package deskkit

// migrate.go — the migration format and runner (distribution/08). A migration
// carries an adopter's repository across one version step: from-version to
// to-version. The format is specified in
// `docs/streams/distribution/migrations.md`; this file is its executable half.
//
// A migration file is HUMAN + AGENT readable — one file, two audiences (a
// distribution/08 requirement so distribution/09's `upgrade-assay` can surface a
// human "release notes" view of the very migration it is about to apply):
//
//   - YAML FRONTMATTER (agent-readable): the ordered identity `id`, the `from`/`to`
//     version range, and an ordered list of idempotent `apply:` steps.
//   - A MARKDOWN BODY (human-readable): "what changed" release-note prose a person
//     reads. It is the source `upgrade-assay` shows as release notes.
//
// Four properties every migration must have, all enforced or enabled here:
//
//   - AN ORDERED IDENTITY.  Migrations apply in `id` order (ties broken by
//     filename), so a run is deterministic regardless of directory iteration.
//   - AN IDEMPOTENT APPLY.  Re-running a migration changes nothing the first run
//     already did. The only step type shipped, ensure-line, is idempotent by
//     construction: it appends a line only when the file does not already contain
//     it.
//   - A DRY-RUN.  `Plan` computes exactly what an apply WOULD do and writes
//     nothing, so an adopter can preview a migration before consenting.
//   - A NO-OP PATH.  A migration whose steps are all already satisfied applies
//     cleanly and mutates nothing. Most releases will ship NO migration at all;
//     the common upgrade path is therefore empty and silent.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// MigrationsDir is the conventional directory, under a migration root, holding one
// file per migration.
const MigrationsDir = "migrations"

// EnsureLine is the one idempotent step type shipped: guarantee File (relative to
// the run root) contains a line exactly equal to Text, appending it only if absent.
// Absent-then-append and already-present-then-skip are the two branches that make
// it idempotent.
type EnsureLine struct {
	File string `yaml:"file"`
	Text string `yaml:"text"`
}

// Step is one apply step. Exactly one op field is set; a step with none is
// malformed and fails the migration closed (never a silent skip). New op types are
// added as additional pointer fields — additive, and an older runner that does not
// know a newer op reports it rather than ignoring it.
type Step struct {
	EnsureLine *EnsureLine `yaml:"ensure-line"`
}

// Migration is one parsed migration file.
type Migration struct {
	ID    string `yaml:"id"`
	From  string `yaml:"from"`
	To    string `yaml:"to"`
	Apply []Step `yaml:"apply"`

	// Notes is the human-readable "what changed" markdown body (everything after
	// the frontmatter). It is release-note prose, not executed.
	Notes string `yaml:"-"`
	// path is the source file, used for tie-breaking equal ids and for messages.
	path string
}

// parseVersion parses a strict `vX.Y.Z` (no leading zeros, no pre-release) into a
// comparable tuple. It accepts a bare tag (the shipped shape) or a namespaced
// `<component>/vX.Y.Z`, taking the numeric part after any `/`.
func parseVersion(tag string) ([3]int, error) {
	v := tag
	if i := strings.IndexByte(v, '/'); i >= 0 {
		v = v[i+1:]
	}
	var out [3]int
	if !strings.HasPrefix(v, "v") {
		return out, fmt.Errorf("version %q must start with v", tag)
	}
	parts := strings.Split(v[1:], ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("version %q is not vX.Y.Z", tag)
	}
	for i, p := range parts {
		if len(p) > 1 && p[0] == '0' {
			return out, fmt.Errorf("version %q has a leading zero in %q", tag, p)
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, fmt.Errorf("version %q has a non-numeric field %q", tag, p)
		}
		out[i] = n
	}
	return out, nil
}

// cmpVersion returns -1/0/1 for a<b / a==b / a>b.
func cmpVersion(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// LoadMigrations reads and parses every migration file under dir, sorted into
// apply order (by id, then filename). A dir that does not exist yields no
// migrations and no error — a release with no migrations is the common case, and
// the empty set must be cheap, not an error. A file that is present but does not
// parse fails closed.
func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, Unverifiable("cannot read migrations dir "+dir, err)
	}
	var migs []Migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		m, err := parseMigrationFile(path)
		if err != nil {
			return nil, err
		}
		migs = append(migs, m)
	}
	sort.Slice(migs, func(i, j int) bool {
		if migs[i].ID != migs[j].ID {
			return migs[i].ID < migs[j].ID
		}
		return migs[i].path < migs[j].path
	})
	return migs, nil
}

// parseMigrationFile splits a migration file into its YAML frontmatter and its
// markdown "what changed" body. The frontmatter is delimited by a leading `---`
// line and a closing `---` line; everything after the close is the human body.
func parseMigrationFile(path string) (Migration, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Migration{}, Unverifiable("cannot read migration "+path, err)
	}
	text := string(raw)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return Migration{}, Unverifiable("migration "+path+" has no `---` frontmatter", nil)
	}
	rest := text[strings.IndexByte(text, '\n')+1:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return Migration{}, Unverifiable("migration "+path+" frontmatter is not closed with `---`", nil)
	}
	front := rest[:end]
	body := ""
	if nl := strings.IndexByte(rest[end+1:], '\n'); nl >= 0 {
		body = rest[end+1+nl+1:]
	}

	var m Migration
	if err := yaml.Unmarshal([]byte(front), &m); err != nil {
		return Migration{}, Unverifiable(fmt.Sprintf("cannot parse migration frontmatter %s: %v", path, err), err)
	}
	m.Notes = strings.TrimSpace(body)
	m.path = path

	// Fail closed on a migration that cannot be ordered or ranged, or that names a
	// step with no known op — none of these can be applied safely.
	if m.ID == "" {
		return Migration{}, Unverifiable("migration "+path+" has no id", nil)
	}
	if _, err := parseVersion(m.From); err != nil {
		return Migration{}, Unverifiable("migration "+path+" bad from-version: "+err.Error(), err)
	}
	if _, err := parseVersion(m.To); err != nil {
		return Migration{}, Unverifiable("migration "+path+" bad to-version: "+err.Error(), err)
	}
	if strings.TrimSpace(m.Notes) == "" {
		return Migration{}, Unverifiable("migration "+path+" has no human-readable `what changed` body", nil)
	}
	for i, s := range m.Apply {
		if s.EnsureLine == nil {
			return Migration{}, Unverifiable(
				fmt.Sprintf("migration %s apply step %d names no known op (want ensure-line)", path, i+1), nil)
		}
		if s.EnsureLine.File == "" || s.EnsureLine.Text == "" {
			return Migration{}, Unverifiable(
				fmt.Sprintf("migration %s ensure-line step %d needs both file and text", path, i+1), nil)
		}
	}
	return m, nil
}

// SelectMigrations returns, in apply order, the migrations whose [from,to] span
// lies within the requested [from,to] span — i.e. mig.from >= from and mig.to <=
// to. A requested span with no migrations inside it returns an empty slice, which
// the runner applies as a clean no-op.
func SelectMigrations(migs []Migration, from, to string) ([]Migration, error) {
	lo, err := parseVersion(from)
	if err != nil {
		return nil, Unverifiable("bad --from: "+err.Error(), err)
	}
	hi, err := parseVersion(to)
	if err != nil {
		return nil, Unverifiable("bad --to: "+err.Error(), err)
	}
	if cmpVersion(lo, hi) > 0 {
		return nil, Refused(fmt.Sprintf("--from %s is higher than --to %s", from, to))
	}
	var out []Migration
	for _, m := range migs {
		mf, _ := parseVersion(m.From)
		mt, _ := parseVersion(m.To)
		if cmpVersion(mf, lo) >= 0 && cmpVersion(mt, hi) <= 0 {
			out = append(out, m)
		}
	}
	return out, nil
}

// StepAction is one line of a dry-run or apply report: what a step did, or would do.
type StepAction struct {
	Migration string
	Desc      string
	Changed   bool // true when applying it changed (or would change) the tree
}

// RunMigrations applies (or, when dryRun, plans) the selected migrations against
// root. It returns one StepAction per step. When dryRun is true it writes NOTHING;
// when false it performs the idempotent writes. Re-running with the same arguments
// is a no-op on an already-migrated tree.
func RunMigrations(root string, selected []Migration, dryRun bool) ([]StepAction, error) {
	var actions []StepAction
	for _, m := range selected {
		for _, s := range m.Apply {
			el := s.EnsureLine
			target := filepath.Join(root, filepath.Clean(el.File))
			// Refuse a path escaping root — a migration mutates the adopter repo,
			// never anything above it.
			rel, err := filepath.Rel(root, target)
			if err != nil || strings.HasPrefix(rel, "..") {
				return actions, Refused("migration "+m.ID+" ensure-line file escapes root: "+el.File)
			}
			present, err := fileHasLine(target, el.Text)
			if err != nil {
				return actions, err
			}
			if present {
				actions = append(actions, StepAction{
					Migration: m.ID, Desc: fmt.Sprintf("ensure-line %s already has %q (no-op)", el.File, el.Text)})
				continue
			}
			desc := fmt.Sprintf("ensure-line append %q to %s", el.Text, el.File)
			if dryRun {
				actions = append(actions, StepAction{Migration: m.ID, Desc: "WOULD " + desc, Changed: true})
				continue
			}
			if err := appendMigLine(target, el.Text); err != nil {
				return actions, err
			}
			actions = append(actions, StepAction{Migration: m.ID, Desc: desc, Changed: true})
		}
	}
	return actions, nil
}

// fileHasLine reports whether path contains a line exactly equal to text. A file
// that does not exist has no such line (not an error) — ensure-line creates it.
func fileHasLine(path, text string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, Unverifiable("cannot read "+path, err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if line == text {
			return true, nil
		}
	}
	return false, nil
}

// appendMigLine appends text as a new line to path, creating it if absent and
// ensuring the addition starts on its own line.
func appendMigLine(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Unverifiable("cannot create dir for "+path, err)
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return Unverifiable("cannot read "+path, err)
	}
	var b strings.Builder
	b.Write(existing)
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		b.WriteByte('\n')
	}
	b.WriteString(text)
	b.WriteByte('\n')
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return Unverifiable("cannot write "+path, err)
	}
	return nil
}
