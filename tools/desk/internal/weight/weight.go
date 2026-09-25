// Package weight is the desk tools' weight counter: five measures of how much surface
// the tool set carries, computed by pure counting functions over an fs.FS rather than by
// grep proxy (spec.md §3 row 5, §4.5, §5.2).
//
// WHY. Additions to tools/desk pass review easily one PR at a time and deletions are rare,
// so the tool set only grows. Nothing noticed that growth before this package: 63 verbs,
// hundreds of flags and about 845 refusal-constructor call sites accumulated with no
// counter watching any of them. This package is the counter; the CI ratchet it feeds
// (ceiling.txt, read by TestCeiling in weight_test.go) is what makes growth visible and
// requires the driver's explicit approval to raise a ceiling.
//
// THE FIVE DIMENSIONS, all counted over non-`_test.go` files:
//
//	verbs    — directories directly under tools/desk/cmd/ that contain a `package main`
//	           file. One verb is one shipped binary.
//	flags    — syntactic flag registrations under tools/desk/cmd/**: a call whose selector
//	           is one of String/Bool/Int/Int64/Uint/Uint64/Float64/Duration/Func/BoolFunc
//	           with a string-literal first argument, or one of the …Var forms (including
//	           TextVar) with a string-literal second argument. This is syntactic, not
//	           type-checked — the same tradeoff forgeban's shell-exec ban makes — so it
//	           reproduces identically at an old SHA without needing that SHA to build.
//	refusals — calls to Refused(, RefusedWithCause(, RefusedFinding( (any qualifier, or
//	           unqualified) under tools/desk/**. These constructors live at
//	           internal/deskkit/exitcodes.go.
//	ruletext — total lines (including blank) of the seven shared desk-role skill bodies
//	           plus deskdispatch's reference markdown files. THREE-STATE: when the plugin
//	           tree is absent (a consumer checkout of tools/desk alone), this dimension is
//	           could-not-check, never rounded to 0.
//	golines  — non-blank lines of non-test .go under tools/desk. REPORTED only; never
//	           ratcheted (RatchetedDimensions omits it).
//
// go/parser, never a regex over source text: a regex over Go source is one novel spelling
// (a multi-line call, an unusual receiver name) away from a false clean, the same reason
// forgeban's shell-exec ban parses rather than greps.
package weight

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Weight is the five measured dimensions over one source tree.
type Weight struct {
	Verbs    int
	Flags    int
	Refusals int
	GoLines  int

	// RuleText is meaningful only when RuleTextCouldNotCheck is false. Three-state: an
	// absent plugin tree is could-not-check, never a silent 0 (facts: "Three-state").
	RuleText              int
	RuleTextCouldNotCheck bool
	RuleTextReason        string
}

// RatchetedDimensions is every dimension ceiling.txt enforces, in the order Evaluate and
// PrintWeight report them. golines is deliberately absent: it is reported, never ratcheted.
var RatchetedDimensions = []string{"verbs", "flags", "refusals", "ruletext"}

// countOf returns the dimension's count by name, for the dimensions Evaluate compares
// against a Ceiling. Panics on an unknown name — a programmer error, never a runtime input.
func (w Weight) countOf(dim string) int {
	switch dim {
	case "verbs":
		return w.Verbs
	case "flags":
		return w.Flags
	case "refusals":
		return w.Refusals
	case "ruletext":
		return w.RuleText
	default:
		panic("weight: unknown dimension " + dim)
	}
}

// ruleTextFiles is the fixed list of shared desk-role skill bodies the ruletext dimension
// sums, relative to the repository root. It is exact rather than a glob: the dimension
// exists to measure the rule text every agent is actually dispatched with, and a glob one
// directory too wide would silently start counting something else (e.g. a house-only
// skill copy) under the same name.
var ruleTextFiles = []string{
	"plugins/assay/skills/the-desk/SKILL.md",
	"plugins/assay/skills/intake-desk/SKILL.md",
	"plugins/assay/skills/worker-desk/SKILL.md",
	"plugins/assay/skills/pr-review-desk/SKILL.md",
	"plugins/assay/skills/verify-desk/SKILL.md",
	"plugins/assay/skills/author-brief/SKILL.md",
	"plugins/assay/skills/pr-shepherd/SKILL.md",
}

// ruleTextReferencesDir holds deskdispatch's per-role reference markdown, globbed for
// *.md rather than listed by name: unlike the fixed skill bodies above, this directory is
// itself the unit a new reference file joins, and the dimension exists to measure
// everything dispatched from it.
const ruleTextReferencesDir = "tools/desk/cmd/deskdispatch/references"

// verbsRoot and refusalsRoot bound the verbs/flags and refusals scans respectively.
// flags shares verbsRoot: both are cmd/**-scoped per the brief's dimension definitions.
const (
	verbsRoot    = "tools/desk/cmd"
	refusalsRoot = "tools/desk"
)

// Count computes all five dimensions over fsys, which is expected to be rooted at the
// repository root (so that "tools/desk/cmd", "tools/desk" and "plugins/assay/skills" are
// all reachable from its root). A subtree that does not exist under fsys counts as 0 for
// verbs/flags/refusals/golines (an empty tree has no verbs — that is not a could-not-check,
// it is the true count); ruletext is the one dimension where an absent tree is reported as
// itself rather than rounded to 0, per the three-state rule.
func Count(fsys fs.FS) (Weight, error) {
	var w Weight

	verbs, err := countVerbs(fsys)
	if err != nil {
		return w, fmt.Errorf("counting verbs: %w", err)
	}
	w.Verbs = verbs

	flags, err := countFlags(fsys)
	if err != nil {
		return w, fmt.Errorf("counting flags: %w", err)
	}
	w.Flags = flags

	refusals, err := countRefusals(fsys)
	if err != nil {
		return w, fmt.Errorf("counting refusals: %w", err)
	}
	w.Refusals = refusals

	goLines, err := countGoLines(fsys)
	if err != nil {
		return w, fmt.Errorf("counting golines: %w", err)
	}
	w.GoLines = goLines

	ruleText, ok, reason, err := countRuleText(fsys)
	if err != nil {
		return w, fmt.Errorf("counting ruletext: %w", err)
	}
	w.RuleText = ruleText
	w.RuleTextCouldNotCheck = !ok
	w.RuleTextReason = reason

	return w, nil
}

// goFiles lists .go files under root (recursively), skipping vendor/testdata/.git/
// node_modules directories and, when excludeTest is set, _test.go files. A root that does
// not exist yields (nil, nil): an absent subtree has no files, which is the true count for
// every dimension but ruletext (which countRuleText checks for explicitly).
func goFiles(fsys fs.FS, root string, excludeTest bool) ([]string, error) {
	if _, err := fs.Stat(fsys, root); err != nil {
		return nil, nil
	}
	var files []string
	walkErr := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", "testdata", ".git", "node_modules":
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		if excludeTest && strings.HasSuffix(name, "_test.go") {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walking %s: %w", root, walkErr)
	}
	sort.Strings(files)
	return files, nil
}

// countVerbs counts the cmd/ subdirectories that ship a package-main file.
func countVerbs(fsys fs.FS) (int, error) {
	entries, err := fs.ReadDir(fsys, verbsRoot)
	if err != nil {
		return 0, nil // no cmd/ tree: 0 verbs, the true count, not a could-not-check
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := path.Join(verbsRoot, e.Name())
		files, rerr := fs.ReadDir(fsys, dir)
		if rerr != nil {
			return 0, fmt.Errorf("reading %s: %w", dir, rerr)
		}
		isMain := false
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			name := f.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			pkg, perr := packageNameOf(fsys, path.Join(dir, name))
			if perr != nil {
				return 0, perr
			}
			if pkg == "main" {
				isMain = true
				break
			}
		}
		if isMain {
			count++
		}
	}
	return count, nil
}

// packageNameOf reads only the package clause of a Go source file — the cheapest parse
// that can answer "is this package main".
func packageNameOf(fsys fs.FS, p string) (string, error) {
	src, err := fs.ReadFile(fsys, p)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", p, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, p, src, parser.PackageClauseOnly)
	if err != nil {
		return "", fmt.Errorf("parsing package clause of %s: %w", p, err)
	}
	return f.Name.Name, nil
}

// flagBaseSelectors is the plain (name is arg 0) half of the flag-registration
// vocabulary the brief names. The …Var forms (name is arg 1) are derived from this set
// in flagArgIndex, plus TextVar, which is a base-list name whose signature is Var-shaped.
var flagBaseSelectors = map[string]bool{
	"String": true, "Bool": true, "Int": true, "Int64": true, "Uint": true,
	"Uint64": true, "Float64": true, "Duration": true, "Func": true, "BoolFunc": true,
}

// flagArgIndex reports the index of the flag-name argument for selector sel, and whether
// sel is a flag-registration selector at all.
func flagArgIndex(sel string) (int, bool) {
	if flagBaseSelectors[sel] {
		return 0, true
	}
	if sel == "TextVar" {
		return 1, true
	}
	if base, ok := strings.CutSuffix(sel, "Var"); ok && flagBaseSelectors[base] {
		return 1, true
	}
	return 0, false
}

// countFlags counts syntactic flag registrations under tools/desk/cmd/**.
func countFlags(fsys fs.FS) (int, error) {
	files, err := goFiles(fsys, verbsRoot, true)
	if err != nil {
		return 0, err
	}
	fset := token.NewFileSet()
	count := 0
	for _, p := range files {
		f, perr := parseFile(fsys, fset, p)
		if perr != nil {
			return 0, perr
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			idx, ok := flagArgIndex(sel.Sel.Name)
			if !ok || idx >= len(call.Args) {
				return true
			}
			lit, ok := call.Args[idx].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			count++
			return true
		})
	}
	return count, nil
}

// refusalConstructors is the closed set of deskkit refusal constructors this dimension
// counts (internal/deskkit/exitcodes.go). Matched by selector/identifier name only — "any
// qualifier" per the brief — so deskkit.Refused(, an aliased import's Refused( and an
// unqualified Refused( from within the deskkit package itself are all one call site.
var refusalConstructors = map[string]bool{
	"Refused": true, "RefusedWithCause": true, "RefusedFinding": true,
}

// countRefusals counts refusal-constructor call sites under tools/desk/**.
func countRefusals(fsys fs.FS) (int, error) {
	files, err := goFiles(fsys, refusalsRoot, true)
	if err != nil {
		return 0, err
	}
	fset := token.NewFileSet()
	count := 0
	for _, p := range files {
		f, perr := parseFile(fsys, fset, p)
		if perr != nil {
			return 0, perr
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				if refusalConstructors[fn.Name] {
					count++
				}
			case *ast.SelectorExpr:
				if refusalConstructors[fn.Sel.Name] {
					count++
				}
			}
			return true
		})
	}
	return count, nil
}

// countGoLines counts non-blank lines of non-test .go under tools/desk. Reported only —
// never ratcheted.
func countGoLines(fsys fs.FS) (int, error) {
	files, err := goFiles(fsys, refusalsRoot, true)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, p := range files {
		n, lerr := countNonBlankLines(fsys, p)
		if lerr != nil {
			return 0, lerr
		}
		total += n
	}
	return total, nil
}

func countNonBlankLines(fsys fs.FS, p string) (int, error) {
	f, err := fsys.Open(p)
	if err != nil {
		return 0, fmt.Errorf("opening %s: %w", p, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	n := 0
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			n++
		}
	}
	if serr := sc.Err(); serr != nil {
		return 0, fmt.Errorf("scanning %s: %w", p, serr)
	}
	return n, nil
}

// countRuleText sums total lines (blank included — this measures rule TEXT, not code
// density) of the fixed skill-body list plus every *.md under the deskdispatch references
// directory. Three-state: when plugins/assay/skills is absent, it returns ok=false with a
// reason, never a silent 0.
func countRuleText(fsys fs.FS) (n int, ok bool, reason string, err error) {
	if _, statErr := fs.Stat(fsys, "plugins/assay/skills"); statErr != nil {
		return 0, false, "plugins/assay/skills not found under this root (a tools/desk-only checkout carries no plugin tree)", nil
	}
	refFiles, err := referenceMarkdownFiles(fsys)
	if err != nil {
		return 0, false, fmt.Sprintf("listing %s: %v", ruleTextReferencesDir, err), nil
	}
	if len(refFiles) == 0 {
		return 0, false, fmt.Sprintf("no *.md found under %s", ruleTextReferencesDir), nil
	}
	all := make([]string, 0, len(ruleTextFiles)+len(refFiles))
	all = append(all, ruleTextFiles...)
	all = append(all, refFiles...)

	total := 0
	for _, p := range all {
		data, rerr := fs.ReadFile(fsys, p)
		if rerr != nil {
			return 0, false, fmt.Sprintf("reading %s: %v", p, rerr), nil
		}
		total += bytes.Count(data, []byte("\n"))
		// A file with content after its last newline (no trailing newline) still has
		// one more line than the newline count says.
		if len(data) > 0 && data[len(data)-1] != '\n' {
			total++
		}
	}
	return total, true, "", nil
}

func referenceMarkdownFiles(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, ruleTextReferencesDir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		out = append(out, path.Join(ruleTextReferencesDir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}

func parseFile(fsys fs.FS, fset *token.FileSet, p string) (*ast.File, error) {
	src, err := fs.ReadFile(fsys, p)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", p, err)
	}
	f, err := parser.ParseFile(fset, p, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", p, err)
	}
	return f, nil
}

// --- Ceiling: ceiling.txt parsing and ratchet evaluation ---

// Ceiling is a parsed ceiling.txt: the file's mode line, the ratcheted ceiling per
// dimension, and the raw lines (kept so a caller can look for a "# grow" annotation above
// a given dimension's line).
type Ceiling struct {
	Mode   string // "advisory" or "blocking"
	Values map[string]int
	Lines  []string
}

// ParseCeiling parses ceiling.txt's format: a leading "# mode: <advisory|blocking>" line,
// "#"-prefixed comment/annotation lines (including "# grow <dim> +<n> <url>"), and one
// "<dimension> <N>" line per ratcheted dimension.
func ParseCeiling(data []byte) (Ceiling, error) {
	c := Ceiling{Values: map[string]int{}}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		c.Lines = append(c.Lines, line)
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case strings.HasPrefix(trimmed, "# mode:"):
			c.Mode = strings.TrimSpace(strings.TrimPrefix(trimmed, "# mode:"))
		case strings.HasPrefix(trimmed, "#"):
			continue
		default:
			fields := strings.Fields(trimmed)
			if len(fields) != 2 {
				return c, fmt.Errorf("malformed ceiling.txt line %q: want \"<dimension> <N>\"", line)
			}
			val, err := strconv.Atoi(fields[1])
			if err != nil {
				return c, fmt.Errorf("malformed ceiling.txt line %q: %w", line, err)
			}
			c.Values[fields[0]] = val
		}
	}
	if err := sc.Err(); err != nil {
		return c, err
	}
	if c.Mode != "advisory" && c.Mode != "blocking" {
		return c, fmt.Errorf("ceiling.txt: mode is %q, want \"advisory\" or \"blocking\"", c.Mode)
	}
	return c, nil
}

// DimensionResult is one ratcheted dimension's measured count against its ceiling.
type DimensionResult struct {
	Dimension string
	Count     int
	Ceiling   int
}

// Delta is Count - Ceiling: positive means growth past the ratchet.
func (d DimensionResult) Delta() int { return d.Count - d.Ceiling }

// Grown reports whether this dimension has grown past its ceiling.
func (d DimensionResult) Grown() bool { return d.Delta() > 0 }

// Evaluate compares w's ratcheted dimensions (RatchetedDimensions) against c, in that
// order. ruletext is omitted from the result — not reported as 0 — when w carries a
// could-not-check for it, so a caller never mistakes "could not measure" for "measured
// zero and clean".
func Evaluate(w Weight, c Ceiling) ([]DimensionResult, error) {
	out := make([]DimensionResult, 0, len(RatchetedDimensions))
	for _, dim := range RatchetedDimensions {
		if dim == "ruletext" && w.RuleTextCouldNotCheck {
			continue
		}
		ceil, ok := c.Values[dim]
		if !ok {
			return nil, fmt.Errorf("ceiling.txt has no line for ratcheted dimension %q", dim)
		}
		out = append(out, DimensionResult{Dimension: dim, Count: w.countOf(dim), Ceiling: ceil})
	}
	return out, nil
}

// GrowthMessage renders a grown dimension's ratchet-failure text — the shape both the
// blocking-mode test failure and the advisory-mode "GROWTH-NOTICE"-prefixed log use
// (Task step 4).
func GrowthMessage(r DimensionResult) string {
	return fmt.Sprintf(
		"%s: %d > ceiling %d (+%d). Reduce, or get the driver's `grow <PR#>` reply and raise the ceiling with a `# grow` line citing it.",
		r.Dimension, r.Count, r.Ceiling, r.Delta(),
	)
}

// SlackMessage renders a dimension that is at or below its ceiling: "slack <dim>=<n>",
// n the remaining headroom.
func SlackMessage(r DimensionResult) string {
	return fmt.Sprintf("slack %s=%d", r.Dimension, r.Ceiling-r.Count)
}

// PrintWeight renders the single summary line PR bodies, reviewers and an adopting
// project's baseline/close-out all quote (Task step 5):
//
//	weight: verbs=<n> flags=<n> refusals=<n> ruletext=<n|could-not-check> golines=<n>
func PrintWeight(w Weight) string {
	ruleText := strconv.Itoa(w.RuleText)
	if w.RuleTextCouldNotCheck {
		ruleText = "could-not-check"
	}
	return fmt.Sprintf("weight: verbs=%d flags=%d refusals=%d ruletext=%s golines=%d",
		w.Verbs, w.Flags, w.Refusals, ruleText, w.GoLines)
}

// GrowthAnnotationAbove reports whether Lines carries a "# grow <dim> +" annotation
// directly above dim's "<dimension> <N>" line — scanning back through the contiguous
// block of comment lines immediately preceding it, which is where such an annotation is
// required to sit (spec §4.5). It does not verify the annotation's URL or its author:
// that is review stage 06's job (the SPOF note in brief-03-weight-ratchet.md); this only
// checks presence.
func GrowthAnnotationAbove(lines []string, dim string) bool {
	target := -1
	for i, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 2 && fields[0] == dim {
			if _, err := strconv.Atoi(fields[1]); err == nil {
				target = i
			}
		}
	}
	if target < 0 {
		return false
	}
	prefix := "# grow " + dim + " +"
	for i := target - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || !strings.HasPrefix(trimmed, "#") {
			break
		}
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}
