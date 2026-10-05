package clicontract

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Budget is the routing limit for one owning brief: one complex entrypoint alone, or at
// most MaxSimple simple ones.
const MaxSimple = 5

var (
	ownerRE    = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)/([0-9]+)$`)
	quotedRE   = regexp.MustCompile(`"([^"]+)"`)
	runFlagRE  = regexp.MustCompile(`(?:^|\s)-run[ =]+('([^']*)'|"([^"]*)"|([^\s'"|]+))`)
	selectorRE = regexp.MustCompile(`^\^Test[A-Za-z0-9_]+\$$`)
	cliRunRE   = regexp.MustCompile(`(?:^|\s)-run[ =]+'\^TestCLI[A-Za-z0-9_]+\$'`)
)

var ruleClasses = map[string]bool{"test": true, "fixture": true, "library": true, "vendor": true, "demo": true}

// Brief is the part of a brief file the routing checks read.
type Brief struct {
	ID      string
	File    string
	Depends []string
	Text    string
}

// LoadBrief finds and parses brief <stream>/<NN> under docs/streams/<stream>/. It
// returns nil when no brief file carries that number.
func LoadBrief(fsys fs.FS, id string) (*Brief, error) {
	m := ownerRE.FindStringSubmatch(id)
	if m == nil {
		return nil, fmt.Errorf("%q is not a <stream>/<NN> brief id", id)
	}
	n, _ := strconv.Atoi(m[2])
	matches, err := fs.Glob(fsys, fmt.Sprintf("docs/streams/%s/brief-%02d-*.md", m[1], n))
	if err != nil || len(matches) == 0 {
		return nil, err
	}
	raw, err := fs.ReadFile(fsys, matches[0])
	if err != nil {
		return nil, err
	}
	b := &Brief{ID: id, File: matches[0], Text: string(raw)}
	b.Depends = frontmatterList(b.Text, "depends")
	return b, nil
}

// frontmatterList returns the quoted items of a one-line YAML flow list key in the
// leading frontmatter block.
func frontmatterList(text, key string) []string {
	if !strings.HasPrefix(text, "---\n") {
		return nil
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return nil
	}
	for _, line := range strings.Split(text[4:4+end], "\n") {
		if strings.HasPrefix(line, key+":") {
			var out []string
			for _, q := range quotedRE.FindAllStringSubmatch(line, -1) {
				out = append(out, q[1])
			}
			return out
		}
	}
	return nil
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// Validate checks reg against independent discovery over fsys and returns every
// problem found, sorted. An empty result is the only pass.
func Validate(fsys fs.FS, reg *Registry) []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	if reg.Schema != "cli-migration/v1" {
		add("registry schema %q is not cli-migration/v1", reg.Schema)
	}
	if reg.Reference == "" || reg.FinalGate == "" || reg.Stream == "" {
		add("registry must name its stream, reference and final_gate")
	}

	found, err := Discover(fsys)
	if err != nil {
		return []string{"discovery failed: " + err.Error()}
	}
	if len(found) == 0 {
		return []string{"discovery found no entrypoints; the root is wrong"}
	}
	discovered := map[string]Discovered{}
	for _, d := range found {
		discovered[d.Path] = d
	}

	// Exclusion rules: well-formed, live, and never covering a shipped launcher.
	ruleHits := make([]int, len(reg.ExclusionRules))
	for i, r := range reg.ExclusionRules {
		if !ruleClasses[r.Class] {
			add("exclusion rule %q has class %q; want test, fixture, library, vendor or demo", r.Pattern, r.Class)
		}
		if strings.TrimSpace(r.Reason) == "" {
			add("exclusion rule %q has no reason", r.Pattern)
		}
		if strings.TrimSpace(r.Pattern) == "" {
			add("exclusion rule %d has no pattern", i)
		}
	}
	excludedBy := func(p string) int {
		for i, r := range reg.ExclusionRules {
			if r.Pattern != "" && MatchPattern(r.Pattern, p) {
				return i
			}
		}
		return -1
	}
	for _, d := range found {
		i := excludedBy(d.Path)
		if i < 0 {
			continue
		}
		ruleHits[i]++
		r := reg.ExclusionRules[i]
		if Shipped(fsys, d.Path) {
			add("shipped entrypoint %s is excluded by rule %q; a shipped launcher is not a %s", d.Path, r.Pattern, r.Class)
		}
		if r.Class == "library" && d.Kind == "script" && (hasShebang(fsys, d.Path) || hasMainGuard(fsys, d.Path)) {
			add("library-class %s is executable (shebang or __main__ guard); route it as an entrypoint", d.Path)
		}
	}
	for i, r := range reg.ExclusionRules {
		if ruleHits[i] == 0 && r.Pattern != "" {
			add("stale exclusion rule %q matches no discovered entrypoint", r.Pattern)
		}
	}

	// Rows: unique, discovered, complete, owned.
	rows := map[string]*Entry{}
	owners := map[string][]*Entry{}
	for i := range reg.Entrypoints {
		e := &reg.Entrypoints[i]
		if _, dup := rows[e.Path]; dup {
			add("duplicate row %s", e.Path)
			continue
		}
		rows[e.Path] = e
		if e.Path == "" {
			add("row %d has no path", i)
			continue
		}
		d, ok := discovered[e.Path]
		switch {
		case e.State == StateRetired:
			if ok {
				add("retired row %s still exists in the tree", e.Path)
			}
		case !ok:
			add("stale row %s: no such entrypoint is discovered", e.Path)
		case d.Kind != e.Kind:
			add("row %s has kind %q; discovered %q", e.Path, e.Kind, d.Kind)
		}
		if i := excludedBy(e.Path); i >= 0 {
			add("row %s is also matched by exclusion rule %q; route it one way", e.Path, reg.ExclusionRules[i].Pattern)
		}
		for name, v := range map[string]string{"executable": e.Executable, "kind": e.Kind, "module": e.Module,
			"release": e.Release, "parser": e.Parser, "basis": e.Basis} {
			if strings.TrimSpace(v) == "" {
				add("row %s has an empty %s", e.Path, name)
			}
		}
		if len(e.Consumers) == 0 {
			add("row %s lists no consumers", e.Path)
		}
		if e.Complexity != "simple" && e.Complexity != "complex" {
			add("row %s has complexity %q; want simple or complex", e.Path, e.Complexity)
		}
		switch e.State {
		case StateExcluded:
			if !ruleClasses[e.Class] || strings.TrimSpace(e.Reason) == "" {
				add("excluded row %s needs a class (test, fixture, library, vendor, demo) and a reason", e.Path)
			}
			if Shipped(fsys, e.Path) {
				add("shipped entrypoint %s is an excluded row; a shipped launcher is not a %s", e.Path, e.Class)
			}
			continue
		case StatePending, StateMigrated, StateRetired:
		default:
			add("row %s has state %q; want pending, migrated, retired or excluded", e.Path, e.State)
		}
		if e.Owner == "" {
			add("row %s has no owner", e.Path)
			continue
		}
		owners[e.Owner] = append(owners[e.Owner], e)
		if e.State == StateMigrated {
			validateMigrated(fsys, e, rows, reg, add)
		}
	}
	for _, d := range found {
		if _, ok := rows[d.Path]; !ok && excludedBy(d.Path) < 0 {
			add("unrouted entrypoint %s (%s): add a registry row with an owning brief, or an exclusion rule with a class and reason", d.Path, d.Kind)
		}
	}

	// Owners: real briefs, wired between the reference and the final gate, within budget.
	final, err := LoadBrief(fsys, reg.FinalGate)
	if err != nil || final == nil {
		add("final gate %s has no brief file", reg.FinalGate)
	}
	ownerIDs := make([]string, 0, len(owners))
	for o := range owners {
		ownerIDs = append(ownerIDs, o)
	}
	sort.Strings(ownerIDs)
	for _, o := range ownerIDs {
		es := owners[o]
		b, err := LoadBrief(fsys, o)
		if err != nil || b == nil {
			add("orphan owner %s: no brief file carries it (rows %s)", o, rowPaths(es))
			continue
		}
		if o != reg.Reference && !contains(b.Depends, reg.Reference) {
			add("owner %s does not depend on %s, the reference adapter", o, reg.Reference)
		}
		if final != nil && !contains(final.Depends, o) {
			add("final gate %s does not depend on %s", reg.FinalGate, o)
		}
		simple, complex := 0, 0
		for _, e := range es {
			if e.Complexity == "complex" {
				complex++
			} else {
				simple++
			}
		}
		if complex > 1 || (complex == 1 && simple > 0) || simple > MaxSimple {
			add("owner %s over budget: %d complex and %d simple rows (one complex alone, or at most %d simple)", o, complex, simple, MaxSimple)
		}
		validateSelectors(b, add)
	}
	sort.Strings(problems)
	return problems
}

func rowPaths(es []*Entry) string {
	ps := make([]string, len(es))
	for i, e := range es {
		ps[i] = e.Path
	}
	return strings.Join(ps, ", ")
}

// validateSelectors checks that every go test -run selector in an owner brief names a
// real anchored test and that the brief selects at least one TestCLI test.
func validateSelectors(b *Brief, add func(string, ...any)) {
	for _, m := range runFlagRE.FindAllStringSubmatch(b.Text, -1) {
		sel := m[2] + m[3] + m[4]
		if !selectorRE.MatchString(sel) {
			add("owner %s has an empty or non-test -run selector %q", b.ID, sel)
		}
	}
	if !cliRunRE.MatchString(b.Text) {
		add("owner %s selects no TestCLI test in its Verify rows", b.ID)
	}
}

// validateMigrated checks that a migrated row's code actually moved: a Go row imports
// Cobra or the desk adapter; a script row names the Go entrypoint it moved to and, if it
// still exists, delegates to it.
func validateMigrated(fsys fs.FS, e *Entry, rows map[string]*Entry, reg *Registry, add func(string, ...any)) {
	switch e.Kind {
	case "go":
		if !importsAdapter(fsys, e.Path) {
			add("migrated row %s imports neither github.com/spf13/cobra nor the desk cli adapter", e.Path)
		}
	case "script":
		to, ok := rows[e.MigratedTo]
		if e.MigratedTo == "" || !ok || to.Kind != "go" {
			add("migrated script %s must name the inventoried Go entrypoint it moved to in migrated_to", e.Path)
			return
		}
		raw, err := fs.ReadFile(fsys, e.Path)
		if err == nil && !strings.Contains(string(raw), path.Base(e.MigratedTo)) {
			add("migrated script %s does not delegate to %s", e.Path, e.MigratedTo)
		}
	}
}

func importsAdapter(fsys fs.FS, dir string) bool {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return false
	}
	for _, d := range entries {
		n := d.Name()
		if d.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		src, err := fs.ReadFile(fsys, path.Join(dir, n))
		if err != nil {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), n, src, parser.ImportsOnly)
		if err != nil {
			continue
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if p == "github.com/spf13/cobra" || strings.HasSuffix(p, "/tools/desk/internal/cli") {
				return true
			}
		}
	}
	return false
}

func hasMainGuard(fsys fs.FS, p string) bool {
	raw, err := fs.ReadFile(fsys, p)
	return err == nil && strings.Contains(string(raw), "__main__")
}
