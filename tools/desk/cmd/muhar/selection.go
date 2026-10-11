// Coverage-guided test selection (-select, #2636).
//
// Without selection every mutant re-runs the spec's whole `test` command. With it,
// muhar first records which tests execute which code (one coverage profile per
// top-level test, taken on the pristine tree after the baseline is green), and
// then runs each mutant against only the tests that executed the FUNCTION the
// mutation lands in, plus the spec's always-run list.
//
// SAFETY PROPERTY. Selection can only REMOVE tests from a mutant's run. The
// narrowed command is the spec's own `go test` invocation with the same flags,
// over a subset of its packages, with -run narrowed to an anchored list of
// top-level test names drawn from the suite's own `-test.list` of the spec's
// -run filter. Every test it runs is one the full command would run. So a red
// narrowed run is a red the full suite would also produce, and the only verdict
// selection can change is caught -> NOT CAUGHT (a test that would have caught
// the mutant was not selected). It can never turn a surviving mutant green, and
// any divergence from the full sweep shows as a NOT CAUGHT, which reddens the
// gate — never as a silent pass. Two things keep that honest:
//
//   - The positive control runs UNDER SELECTION, through the same path as every
//     mutant, so a narrowed suite that cannot fail breaks the run (exit 2).
//   - Every test is run alone on pristine source during the coverage pass, and a
//     test that fails alone breaks the run: a narrowed run of an order-dependent
//     test could otherwise go red for a reason that has nothing to do with the
//     mutation.
//
// What cannot be attributed is never narrowed: a mutation in a non-Go file, in a
// file the coverage build did not instrument, or outside any function body (a
// signature or a package-level declaration), or one that adds or removes a
// top-level declaration, runs the full command. A mutation inside an
// instrumented function that no test executed, with an empty always-run list, is
// NOT CAUGHT without a run — the verdict the full suite gives unless some test
// catches it without executing it (one that reads the source text, a golden
// file), which is what the always-run list is for.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// selectSpec is the spec's optional "select" object.
type selectSpec struct {
	// Always lists top-level test names added to EVERY narrowed run: the tests
	// that catch mutants without executing the mutated function.
	Always []string `json:"always"`
}

// Selection is what the selector decided for one mutation.
type Selection struct {
	// Skip: no selected test executes the mutated function; the verdict is NOT
	// CAUGHT without a suite run. Never Caught — skipping cannot produce red.
	Skip bool
	// Test is the narrowed suite. nil means run the spec's full command.
	Test func(root string) bool
}

// Selector narrows each mutant's suite. Prepare runs once on the pristine tree,
// after the baseline is green; For is then called concurrently by the workers.
type Selector interface {
	Prepare(root string) error
	For(m Mutation, orig []byte) Selection
}

// goTestCmd is a spec `test` command that selection can model: `go test` with
// a known set of flags and package patterns, nothing else.
type goTestCmd struct {
	flags []string // every flag token except -run, verbatim and in order
	run   string   // the -run filter ("" = every test)
	pkgs  []string // package patterns
}

// selectableFlags are the go test flags a narrowed command can carry over
// unchanged. Anything else is refused: a flag selection does not understand
// could change which tests run, and guessing would break the subset property.
var selectableFlags = map[string]bool{"-count": true, "-timeout": true, "-v": true, "-short": true, "-race": true, "-failfast": true}

// parseGoTest models cmd, or refuses it. Refusal is loud (the caller exits 1):
// a spec whose command selection cannot model is never silently run in full
// under -select, because the operator asked for selection and would read the
// result as a selected sweep.
func parseGoTest(cmd string) (goTestCmd, error) {
	var g goTestCmd
	toks, err := shellWords(cmd)
	if err != nil {
		return g, err
	}
	if len(toks) < 3 || toks[0] != "go" || toks[1] != "test" {
		return g, fmt.Errorf("-select needs a plain `go test [flags] <packages>` command, got %q", cmd)
	}
	seenRun := false
	for i := 2; i < len(toks); i++ {
		t := toks[i]
		if !strings.HasPrefix(t, "-") {
			g.pkgs = append(g.pkgs, t)
			continue
		}
		name, val, hasVal := strings.Cut(t, "=")
		name = "-" + strings.TrimLeft(name, "-")
		if name == "-run" {
			if seenRun {
				return g, fmt.Errorf("-select: test command passes -run twice")
			}
			seenRun = true
			if !hasVal {
				if i+1 >= len(toks) {
					return g, fmt.Errorf("-select: -run has no value")
				}
				i++
				val = toks[i]
			}
			if strings.Contains(val, "/") {
				return g, fmt.Errorf("-select cannot model a -run filter with a subtest part (%q)", val)
			}
			if _, err := regexp.Compile(val); err != nil {
				return g, fmt.Errorf("-select: -run %q: %v", val, err)
			}
			g.run = val
			continue
		}
		if !selectableFlags[name] {
			return g, fmt.Errorf("-select cannot model go test flag %q", t)
		}
		if !hasVal && (name == "-count" || name == "-timeout") {
			if i+1 >= len(toks) {
				return g, fmt.Errorf("-select: %s has no value", name)
			}
			t += "=" + toks[i+1]
			i++
		}
		g.flags = append(g.flags, t)
	}
	if len(g.pkgs) == 0 {
		return g, fmt.Errorf("-select: test command names no packages")
	}
	return g, nil
}

// shellWords splits a command into words with plain single/double quoting and
// refuses every other piece of shell syntax (pipes, &&, $, globs, escapes).
func shellWords(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	in := false
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
				continue
			}
			if quote == '"' && strings.IndexByte("$`\\", c) >= 0 {
				return nil, fmt.Errorf("-select cannot model shell syntax %q in %q", c, s)
			}
			cur.WriteByte(c)
		case c == '\'' || c == '"':
			quote, in = c, true
		case c == ' ' || c == '\t':
			if in {
				out = append(out, cur.String())
				cur.Reset()
				in = false
			}
		case strings.IndexByte("|&;<>()$`\\*?[]{}~#\n", c) >= 0:
			return nil, fmt.Errorf("-select cannot model shell syntax %q in %q", c, s)
		default:
			cur.WriteByte(c)
			in = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("-select: unterminated quote in %q", s)
	}
	if in {
		out = append(out, cur.String())
	}
	return out, nil
}

// testRef names one top-level test of one test package.
type testRef struct {
	pkg  int // index into coverSelector.pkgs
	name string
}

type testPkg struct {
	importPath string
	dir        string // absolute, in the preflight root
	tests      []int  // indices into coverSelector.tests
}

// cblock is one coverage block of a mutated file: its line span and the tests
// that executed it.
type cblock struct {
	l1, l2 int
	tests  []int
}

type coverSelector struct {
	cmd    goTestCmd
	always []string
	files  []string // every spec-relative file the control and mutations name
	jobs   int
	env    []string
	log    io.Writer
	// goTest runs `go test <args>` in root and reports whether it FAILED.
	goTest func(root string, args []string) bool

	// Filled by Prepare.
	pkgs      []testPkg
	tests     []testRef
	alwaysIDs []int
	blocks    map[string][]cblock // spec-relative file -> instrumented blocks
}

func newCoverSelector(s spec, cmd goTestCmd, jobs int, env []string, log io.Writer, goTest func(string, []string) bool) *coverSelector {
	cs := &coverSelector{cmd: cmd, jobs: jobs, env: env, log: log, goTest: goTest}
	if s.Select != nil {
		cs.always = s.Select.Always
	}
	seen := map[string]bool{}
	for _, m := range append([]Mutation{s.Control}, s.Mutations...) {
		if !seen[m.File] {
			seen[m.File] = true
			cs.files = append(cs.files, m.File)
		}
	}
	return cs
}

type listedPkg struct {
	ImportPath   string
	Dir          string
	TestGoFiles  []string
	XTestGoFiles []string
	Error        *struct{ Err string }
}

func (cs *coverSelector) goList(root string, patterns []string) ([]listedPkg, error) {
	cmd := exec.Command("go", append([]string{"list", "-e", "-json"}, patterns...)...)
	cmd.Dir = root
	cmd.Env = cs.env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list %v: %v: %s", patterns, err, strings.TrimSpace(stderr.String()))
	}
	var pkgs []listedPkg
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p listedPkg
		if err := dec.Decode(&p); err != nil {
			return nil, fmt.Errorf("parse go list output: %w", err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

var testNameRe = regexp.MustCompile(`^(Test|Example|Fuzz)[\p{L}\p{N}_]*$`)

// Prepare runs the coverage pass: build each test package once with coverage
// over the mutated files' packages, list the suite's tests, run each one alone
// with a coverage profile, and index the executed blocks of every mutated file.
func (cs *coverSelector) Prepare(root string) error {
	start := time.Now()
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		root = wd
	}
	// The mutated files' packages are what the coverage build instruments.
	// A file whose directory is not a buildable package is left out, and its
	// mutations run the full command.
	var dirs []string
	dirSeen := map[string]bool{}
	for _, f := range cs.files {
		d := filepath.Dir(f)
		if !filepath.IsAbs(d) {
			d = "./" + filepath.ToSlash(d)
		}
		if !dirSeen[d] {
			dirSeen[d] = true
			dirs = append(dirs, d)
		}
	}
	mpkgs, err := cs.goList(root, dirs)
	if err != nil {
		return err
	}
	byDir := map[string]string{} // abs dir -> import path
	var coverpkg []string
	for _, p := range mpkgs {
		if p.Error == nil && p.ImportPath != "" {
			byDir[p.Dir] = p.ImportPath
			coverpkg = append(coverpkg, p.ImportPath)
		}
	}
	// profile key ("importpath/base.go") -> spec-relative file
	keyToFile := map[string]string{}
	for _, f := range cs.files {
		abs := f
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, f)
		}
		if ip, ok := byDir[filepath.Dir(abs)]; ok {
			keyToFile[ip+"/"+filepath.Base(f)] = f
		}
	}
	cs.blocks = map[string][]cblock{}
	if len(coverpkg) == 0 {
		return cs.resolveAlways() // nothing instrumentable: every mutant runs the full command
	}

	tpkgs, err := cs.goList(root, cs.cmd.pkgs)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "muhar-select-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	listFilter := cs.cmd.run
	if listFilter == "" {
		listFilter = "."
	}
	bins := map[int]string{}
	for _, p := range tpkgs {
		if p.Error != nil {
			return fmt.Errorf("test package %s: %s", p.ImportPath, p.Error.Err)
		}
		if len(p.TestGoFiles)+len(p.XTestGoFiles) == 0 {
			continue
		}
		pi := len(cs.pkgs)
		bin := filepath.Join(tmp, strconv.Itoa(pi)+".test")
		build := exec.Command("go", "test", "-c", "-cover", "-covermode=set",
			"-coverpkg="+strings.Join(coverpkg, ","), "-o", bin, p.ImportPath)
		build.Dir = root
		build.Env = cs.env
		if out, err := build.CombinedOutput(); err != nil {
			return fmt.Errorf("coverage build of %s: %v: %s", p.ImportPath, err, strings.TrimSpace(string(out)))
		}
		list := exec.Command(bin, "-test.list", listFilter)
		list.Dir = p.Dir
		list.Env = cs.env
		out, err := list.Output()
		if err != nil {
			return fmt.Errorf("list tests of %s: %v", p.ImportPath, err)
		}
		tp := testPkg{importPath: p.ImportPath, dir: p.Dir}
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if testNameRe.MatchString(line) {
				tp.tests = append(tp.tests, len(cs.tests))
				cs.tests = append(cs.tests, testRef{pkg: pi, name: line})
			}
		}
		cs.pkgs = append(cs.pkgs, tp)
		bins[pi] = bin
	}
	if err := cs.resolveAlways(); err != nil {
		return err
	}

	// One process per test, `jobs` at a time. Each profile is parsed and only
	// the mutated files' executed blocks are kept, so memory stays bounded.
	type res struct {
		id     int
		blocks map[string][][2]int // file -> executed spans
		inst   map[string][][2]int // file -> every instrumented span
		err    error
	}
	ids := make(chan int)
	results := make(chan res)
	jobs := cs.jobs
	if jobs < 1 {
		jobs = 1
	}
	var wg sync.WaitGroup
	for w := 0; w < jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range ids {
				t := cs.tests[id]
				prof := filepath.Join(tmp, fmt.Sprintf("c%d.out", id))
				run := exec.Command(bins[t.pkg], "-test.run", "^"+regexp.QuoteMeta(t.name)+"$",
					"-test.count=1", "-test.timeout=10m", "-test.coverprofile="+prof)
				run.Dir = cs.pkgs[t.pkg].dir
				run.Env = cs.env
				out, err := run.CombinedOutput()
				if err != nil {
					results <- res{id: id, err: fmt.Errorf("%s fails when run alone on pristine source (%v) — a narrowed run could go red for a reason that is not the mutation:\n%s",
						t.name, err, tail(out, 20))}
					continue
				}
				ex, inst, err := parseProfile(prof, keyToFile)
				_ = os.Remove(prof)
				results <- res{id: id, blocks: ex, inst: inst, err: err}
			}
		}()
	}
	go func() {
		for id := range cs.tests {
			ids <- id
		}
		close(ids)
		wg.Wait()
		close(results)
	}()
	var firstErr error
	instSeen := map[string]bool{}
	type spanKey struct {
		f      string
		l1, l2 int
	}
	idx := map[spanKey]int{}
	for r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		for f, spans := range r.inst {
			if instSeen[f] {
				continue
			}
			instSeen[f] = true
			for _, s := range spans {
				k := spanKey{f, s[0], s[1]}
				if _, ok := idx[k]; !ok {
					idx[k] = len(cs.blocks[f])
					cs.blocks[f] = append(cs.blocks[f], cblock{l1: s[0], l2: s[1]})
				}
			}
		}
		for f, spans := range r.blocks {
			for _, s := range spans {
				k := spanKey{f, s[0], s[1]}
				i, ok := idx[k]
				if !ok {
					i = len(cs.blocks[f])
					idx[k] = i
					cs.blocks[f] = append(cs.blocks[f], cblock{l1: s[0], l2: s[1]})
				}
				cs.blocks[f][i].tests = append(cs.blocks[f][i].tests, r.id)
			}
		}
	}
	if firstErr != nil {
		return firstErr
	}
	fmt.Fprintf(cs.log, "muhar: select: coverage pass recorded %d tests across %d package(s); %d of %d mutated file(s) instrumented, in %s\n",
		len(cs.tests), len(cs.pkgs), len(cs.blocks), len(cs.files), time.Since(start).Round(100*time.Millisecond))
	return nil
}

// resolveAlways maps the spec's always-run names to tests. A name the suite
// does not list is an error: a misspelt always-run entry would otherwise add
// nothing, silently.
func (cs *coverSelector) resolveAlways() error {
	cs.alwaysIDs = nil
	for _, a := range cs.always {
		found := false
		for id, t := range cs.tests {
			if t.name == a {
				cs.alwaysIDs = append(cs.alwaysIDs, id)
				found = true
			}
		}
		if !found && len(cs.tests) > 0 {
			return fmt.Errorf("select.always names %q, which is not a test of the suite (go test %s -run %q)", a, strings.Join(cs.cmd.pkgs, " "), cs.cmd.run)
		}
	}
	return nil
}

func tail(b []byte, n int) string {
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// parseProfile reads a text coverage profile and returns, for the files in
// keyToFile only, the executed spans and every instrumented span.
func parseProfile(prof string, keyToFile map[string]string) (executed, inst map[string][][2]int, err error) {
	raw, err := os.ReadFile(prof)
	if err != nil {
		return nil, nil, fmt.Errorf("read coverage profile: %w", err)
	}
	executed = map[string][][2]int{}
	inst = map[string][][2]int{}
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		colon := strings.LastIndex(line, ":")
		if colon < 0 {
			return nil, nil, fmt.Errorf("malformed profile line %q", line)
		}
		f, ok := keyToFile[line[:colon]]
		if !ok {
			continue
		}
		var l1, c1, l2, c2, n, count int
		if _, err := fmt.Sscanf(line[colon+1:], "%d.%d,%d.%d %d %d", &l1, &c1, &l2, &c2, &n, &count); err != nil {
			return nil, nil, fmt.Errorf("malformed profile line %q: %v", line, err)
		}
		inst[f] = append(inst[f], [2]int{l1, l2})
		if count > 0 {
			executed[f] = append(executed[f], [2]int{l1, l2})
		}
	}
	return executed, inst, nil
}

// funcLines returns the line span of the function declaration whose BODY
// encloses the first occurrence of old in src, or ok=false when old is absent,
// src does not parse, or the text is not strictly inside a function body. A
// signature or a package-level declaration is not attributable: an edit there
// can add a declaration (#2022's planted check site) or change what other code
// compiles against, which no single function's coverage describes.
func funcLines(filename string, src []byte, old string) (l1, l2 int, ok bool) {
	off := bytes.Index(src, []byte(old))
	if off < 0 {
		return 0, 0, false
	}
	end := off + len(old)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.SkipObjectResolution)
	if err != nil {
		return 0, 0, false
	}
	for _, d := range f.Decls {
		fd, isFn := d.(*ast.FuncDecl)
		if !isFn || fd.Body == nil {
			continue
		}
		lb, rb := fset.Position(fd.Body.Lbrace), fset.Position(fd.Body.Rbrace)
		if lb.Offset < off && end <= rb.Offset {
			return fset.Position(fd.Pos()).Line, rb.Line, true
		}
	}
	return 0, 0, false
}

// declCount is the number of top-level declarations in src, or -1 when src
// does not parse.
func declCount(filename string, src []byte) int {
	f, err := parser.ParseFile(token.NewFileSet(), filename, src, parser.SkipObjectResolution)
	if err != nil {
		return -1
	}
	return len(f.Decls)
}

// For decides m's suite. orig is the pristine content of m's file.
func (cs *coverSelector) For(m Mutation, orig []byte) Selection {
	full := func(why string) Selection {
		fmt.Fprintf(cs.log, "muhar: select %q: full suite (%s)\n", m.Name, why)
		return Selection{}
	}
	if filepath.Ext(m.File) != ".go" {
		return full("not a Go file")
	}
	blocks, ok := cs.blocks[m.File]
	if !ok {
		return full("file not instrumented by the coverage build")
	}
	l1, l2, ok := funcLines(m.File, orig, m.Old)
	if !ok {
		return full("mutation is outside any function body")
	}
	// The replacement must stay inside that body too: New text that closes the
	// function and opens another adds a declaration no coverage describes. A
	// mutated file that no longer parses is a compile failure, which the full
	// command reports as faithfully as any subset.
	if mutated, err := applyEdit(string(orig), m); err != nil || declCount(m.File, []byte(mutated)) != declCount(m.File, orig) {
		return full("mutation adds or removes a top-level declaration")
	}
	instrumented := false
	picked := map[int]bool{}
	for _, b := range blocks {
		if b.l2 < l1 || b.l1 > l2 {
			continue
		}
		instrumented = true
		for _, id := range b.tests {
			picked[id] = true
		}
	}
	if !instrumented {
		return full("no coverage block in the enclosing function")
	}
	covering := len(picked)
	for _, id := range cs.alwaysIDs {
		picked[id] = true
	}
	if len(picked) == 0 {
		fmt.Fprintf(cs.log, "muhar: select %q: no test executes the mutated function — NOT CAUGHT without a run\n", m.Name)
		return Selection{Skip: true}
	}
	args := cs.narrowedArgs(picked)
	fmt.Fprintf(cs.log, "muhar: select %q: %d of %d tests (%d covering, %d always-run)\n",
		m.Name, len(picked), len(cs.tests), covering, len(picked)-covering)
	return Selection{Test: func(root string) bool { return cs.goTest(root, args) }}
}

// narrowedArgs builds the subset command: the spec's flags verbatim, -run
// anchored to the picked names, and only the packages that hold one of them.
// Every name came from `-test.list` of the spec's own -run filter, so every test
// this runs is one the full command runs too (the SAFETY PROPERTY above).
func (cs *coverSelector) narrowedArgs(picked map[int]bool) []string {
	names := map[string]bool{}
	pkgs := map[int]bool{}
	for id := range picked {
		names[cs.tests[id].name] = true
		pkgs[cs.tests[id].pkg] = true
	}
	var ns []string
	for n := range names {
		ns = append(ns, regexp.QuoteMeta(n))
	}
	sort.Strings(ns)
	args := append([]string{"test"}, cs.cmd.flags...)
	args = append(args, "-run", "^("+strings.Join(ns, "|")+")$")
	var ps []string
	for pi := range pkgs {
		ps = append(ps, cs.pkgs[pi].importPath)
	}
	sort.Strings(ps)
	return append(args, ps...)
}
