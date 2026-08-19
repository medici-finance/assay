package main

// shardcheck — the disjointness precondition for an intra-brief split
// (methodology/43).
//
// A brief may declare that its work decomposes into N concurrent SHARDS, each
// scoped to a file glob (`parallel-streams:` in brief-v1 frontmatter). This file
// decides whether that declared split is safe to dispatch in parallel. It is a
// PRECONDITION, never an optimisation hint: the split is refused unless the
// evidence for safety is in hand.
//
// # Why a path partition is not enough
//
// Partitioning by file path prevents exactly ONE collision class — two shards
// editing the same bytes. Every other class that has actually broken main
// survives a path partition untouched, so this checker names them and refuses
// the split when it cannot rule them out:
//
//	path-overlap     two shards' globs match a common file. The only class a
//	                 path partition prevents on its own, and only if it is
//	                 checked — an ASSERTED disjointness is not a checked one.
//	shared-surface   a shard owns a file that is a shared NUMBERING or ROW space
//	                 rather than a set of independent lines: docs/brief-rules.md
//	                 (two briefs authored rules 25 and 26 into it in parallel;
//	                 neither diff showed the other's numbers; both merged green
//	                 and the file now carries each number twice), a stream README
//	                 row table, a generated artifact, a module graph, a pin set.
//	                 Disjoint edits to one numbering space are still a collision,
//	                 because the thing being partitioned is not the bytes.
//	symbol-coupling  shard A changes a declaration that shard B calls. No textual
//	                 conflict, both shards green in isolation, main red on merge:
//	                 statusgen's own emit() grew an 8th parameter on one branch
//	                 while another still called it with 7 args. This is the class
//	                 a path partition is WORST at, because path disjointness is
//	                 what makes the two diffs look independent.
//
// # What it deliberately does NOT do
//
// It analyses one brief's own declared shards against each other. It says
// nothing about a collision between this brief and a DIFFERENT branch, which is
// how the emit() incident actually reached main — that is merge-time detection
// and belongs to desk-hardening/05, not here. It says nothing about whether the
// branch is merge-current (issue-flow/09). And its symbol analysis covers Go
// files inside one package only; every other cross-shard file pair is reported
// as could-not-check, which refuses the split. That cap is real and is printed
// on every run rather than defaulted away.
//
// # Three-state, and which way it fails
//
// checked-clean (0) / checked-failed (1) / could-not-check (2), matching the
// verifyrun exit contract in this same binary. A caller must treat 1 AND 2
// identically for dispatch: run the brief in ONE worker, serially. Refusing a
// safe split costs wall-clock; approving an unsafe one costs a red main, so
// every unproven state resolves to serial.

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ParallelStream is one declared shard of an intra-brief split: a name and the
// file globs that shard is allowed to touch. Parsed from the OPTIONAL brief-v1
// `parallel-streams:` key; absence means today's one-worker-per-brief dispatch
// and is never an error.
type ParallelStream struct {
	Name  string
	Files []string
}

// shardcheck exit codes. Same three-state contract, and the same numbers, as
// verifyrun above — one binary, one convention. 2 doubles as the usage code
// because a run that could not be set up is a run that could not check.
const (
	shardExitClean    = 0
	shardExitFailed   = 1
	shardExitCouldNot = 2
)

// sharedSurface is a path pattern that CANNOT be partitioned by ownership,
// because the thing two shards would contend on is not the bytes they each
// write. This table is the DECLARED SOURCE for that fact — the skills point
// here rather than restating the list, so there is one place to add a surface.
//
// A surface earns a row by having produced a real collision, or by being a
// space (numbering, ordering, generation) where two independently-correct edits
// compose into something wrong.
type sharedSurface struct {
	Pattern string
	Class   string
	Why     string
}

var sharedSurfaces = []sharedSurface{
	{
		Pattern: "docs/brief-rules.md",
		Class:   "numbering-space",
		Why:     "rule numbers are a single ascending space; two parallel authors both take the next free number and neither diff shows the other's. This file carries 25 and 26 twice for exactly that reason.",
	},
	{
		Pattern: "docs/streams/**/README.md",
		Class:   "row-table",
		Why:     "the brief table is one ordered row space; two shards flipping adjacent rows conflict textually and two shards adding rows collide on ordering.",
	},
	{
		Pattern: "STATUS.md",
		Class:   "generated",
		Why:     "single-writer generated artifact — main's CI writes it; a shard that edits it is editing the output, not the source.",
	},
	{
		Pattern: "go.work",
		Class:   "module-graph",
		Why:     "the module set is one list; adding a module in two shards yields a file that is textually merged and semantically wrong.",
	},
	{
		Pattern: "**/go.mod",
		Class:   "module-graph",
		Why:     "require/replace blocks are an ordered set with a checksum partner (go.sum); parallel edits produce a tree that neither shard tested.",
	},
	{
		Pattern: "**/go.sum",
		Class:   "module-graph",
		Why:     "derived from go.mod; a shard cannot own it independently of whoever owns go.mod.",
	},
	{
		Pattern: ".assay-versions",
		Class:   "pin-set",
		Why:     "one pin per tool, read by every consumer; two shards repinning different tools still produce one file whose combined state nobody built against.",
	},
	{
		Pattern: ".github/workflows/**",
		Class:   "ci-path-set",
		Why:     "a workflow's paths: globs are a whole-repo claim about what CI reads; a shard that adds a reader without the sibling shard's paths entry ships a check that never fires.",
	},
}

// checkParallelStreamShape validates a declared split's SHAPE — the part
// decidable from frontmatter alone, with no file tree in hand. It is shared by
// `--lint` (which has no dispatch context) and checkShardPlan (which runs it
// first, because there is no point resolving globs for a plan that cannot
// describe a split). Returns one message per problem; empty means shape-valid,
// which is emphatically NOT the same as safe-to-split.
func checkParallelStreamShape(streams []ParallelStream) []string {
	if len(streams) == 0 {
		return nil // absent: the one-worker default, never a finding
	}
	var out []string
	if len(streams) < 2 {
		out = append(out, fmt.Sprintf("parallel-streams: declares %d shard; parallel dispatch needs at least 2 — remove the field or declare the second shard", len(streams)))
	}
	seen := map[string]bool{}
	for _, s := range streams {
		name := strings.TrimSpace(s.Name)
		switch {
		case name == "":
			out = append(out, "parallel-streams: a shard has an empty name — a shard is claimed, dispatched and reported by name")
		case seen[name]:
			out = append(out, fmt.Sprintf("parallel-streams: shard %q is declared twice — two shards with one name share one claim key, so one of them is invisible to the dispatcher", name))
		}
		seen[name] = true
		if len(s.Files) == 0 {
			out = append(out, fmt.Sprintf("parallel-streams: shard %q declares no files: globs — an unscoped shard has no scope to be disjoint from", name))
		}
	}
	return out
}

// shardFinding is one reason the split was refused, carrying the class so a
// caller can tell a fixable declaration error from a structural one.
type shardFinding struct {
	Class  string
	Detail string
}

// shardCoverage records what the analysis could NOT reach. It exists so the
// result can be three-state: a plan with zero findings but non-empty coverage
// gaps is could-not-check, never clean. Without this the checker would report
// success on precisely the splits it understands least.
type shardCoverage struct {
	AnalysedPairs int
	Gaps          []string
}

type shardResult struct {
	// root is carried on the result so the coupling pass can read the Go files
	// it was handed relative paths for, without a second root parameter
	// threaded through every helper.
	root     string
	Files    map[string][]string
	Findings []shardFinding
	Coverage shardCoverage
}

// state collapses the result to the three-state verdict. Any finding is
// checked-failed; no finding but an unreachable pair is could-not-check; only a
// fully-covered, finding-free plan is checked-clean.
func (r shardResult) state() int {
	if len(r.Findings) > 0 {
		return shardExitFailed
	}
	if len(r.Coverage.Gaps) > 0 {
		return shardExitCouldNot
	}
	return shardExitClean
}

// globMatch matches a slash-separated relative path against a glob supporting
// `**` (zero or more path segments) plus path.Match's per-segment syntax. A
// trailing "/" is sugar for "/**" so `backend/` names a subtree.
//
// Written here rather than reached for from a dependency because the semantics
// are load-bearing: `**` matching ZERO segments is what makes `docs/**/README.md`
// cover `docs/README.md`, and a matcher that quietly required one segment would
// under-report shared surfaces — a failure in the unsafe direction.
func globMatch(pattern, name string) bool {
	pattern = strings.TrimPrefix(pattern, "./")
	if strings.HasSuffix(pattern, "/") {
		pattern += "**"
	}
	return globSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func globSegments(pat, seg []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			if len(pat) == 1 {
				return true
			}
			for i := 0; i <= len(seg); i++ {
				if globSegments(pat[1:], seg[i:]) {
					return true
				}
			}
			return false
		}
		if len(seg) == 0 {
			return false
		}
		ok, err := path.Match(pat[0], seg[0])
		if err != nil || !ok {
			return false
		}
		pat, seg = pat[1:], seg[1:]
	}
	return len(seg) == 0
}

// repoFiles enumerates the tracked-shaped file set under root: every regular
// file, relative and slash-separated, minus the directories no shard can own.
func repoFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		if rel == "." {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// checkShardPlan is the whole precondition. It returns a result even on a
// structurally-broken plan so the caller can print every problem at once; err is
// reserved for "could not read the tree at all", which is could-not-check.
func checkShardPlan(root string, streams []ParallelStream) (shardResult, error) {
	res := shardResult{root: root, Files: map[string][]string{}}
	add := func(class, format string, a ...any) {
		res.Findings = append(res.Findings, shardFinding{Class: class, Detail: fmt.Sprintf(format, a...)})
	}

	shapeProblems := checkParallelStreamShape(streams)
	for _, p := range shapeProblems {
		add("plan-shape", "%s", p)
	}
	if len(streams) < 2 {
		// Nothing downstream is meaningful without a pair of shards to compare.
		return res, nil
	}

	all, err := repoFiles(root)
	if err != nil {
		return res, err
	}

	// Expand each shard against the real tree. A glob that matches nothing is a
	// finding, not a shrug: it is either a typo or a shard with no work, and
	// both mean the declared plan is not the plan that would run.
	for _, s := range streams {
		var matched []string
		for _, g := range s.Files {
			hit := false
			for _, f := range all {
				if globMatch(g, f) {
					matched = append(matched, f)
					hit = true
				}
			}
			if !hit {
				add("dead-glob", "shard %q glob %q matches no file under %s — the shard would run with a scope it cannot fill", s.Name, g, root)
			}
		}
		sort.Strings(matched)
		res.Files[s.Name] = dedupeStrings(matched)
	}

	// CLASS 1 — path-overlap. The one class a path partition prevents, and only
	// when someone checks it.
	owner := map[string]string{}
	for _, s := range streams {
		for _, f := range res.Files[s.Name] {
			if prev, ok := owner[f]; ok && prev != s.Name {
				add("path-overlap", "%s is claimed by shards %q and %q — the globs are not disjoint, so the split has no isolation to offer", f, prev, s.Name)
				continue
			}
			owner[f] = s.Name
		}
	}

	// CLASS 2 — shared-surface. Ownership of a numbering/row/generated space is
	// not partitionable at all, so this refuses regardless of overlap.
	for _, s := range streams {
		for _, f := range res.Files[s.Name] {
			for _, ss := range sharedSurfaces {
				if globMatch(ss.Pattern, f) {
					add("shared-surface", "shard %q claims %s, a %s surface (%s) — this file is edited by the coordinating worker after the shards land, never inside a shard", s.Name, f, ss.Class, ss.Why)
				}
			}
		}
	}

	// CLASS 3 — symbol-coupling, plus the coverage accounting that makes an
	// unanalysable pair could-not-check instead of silently clean.
	analyseCoupling(&res, streams, add)
	return res, nil
}

func dedupeStrings(in []string) []string {
	out := in[:0:0]
	var last string
	for i, s := range in {
		if i > 0 && s == last {
			continue
		}
		out = append(out, s)
		last = s
	}
	return out
}

// goFileSymbols is what one Go file declares at package level and what
// identifiers it mentions anywhere.
type goFileSymbols struct {
	Declares map[string]bool
	Mentions map[string]bool
}

// analyseCoupling walks every cross-shard file PAIR. A pair is analysable when
// both sides are Go files in the same directory (== one package, so an
// unqualified identifier in one is the declaration in the other). Every other
// pair — non-Go, or Go across package boundaries — is a coverage gap, and a gap
// makes the whole plan could-not-check.
func analyseCoupling(res *shardResult, streams []ParallelStream, add func(string, string, ...any)) {
	syms := map[string]goFileSymbols{}
	parseFailed := map[string]string{}
	shardOf := map[string]string{}
	for _, s := range streams {
		for _, f := range res.Files[s.Name] {
			shardOf[f] = s.Name
		}
	}

	gapCount := map[string]int{}
	gapExample := map[string][]string{}
	noteGap := func(class, example string) {
		gapCount[class]++
		if len(gapExample[class]) < 3 {
			gapExample[class] = append(gapExample[class], example)
		}
	}

	names := make([]string, 0, len(shardOf))
	for f := range shardOf {
		names = append(names, f)
	}
	sort.Strings(names)

	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			a, b := names[i], names[j]
			if shardOf[a] == shardOf[b] {
				continue // same shard: one worker, ordinary serial editing
			}
			aGo, bGo := strings.HasSuffix(a, ".go"), strings.HasSuffix(b, ".go")
			if !aGo || !bGo {
				noteGap("non-Go file pair (no coupling analysis exists for this file type)", a+" ~ "+b)
				continue
			}
			if path.Dir(a) != path.Dir(b) {
				noteGap("Go pair across package directories (only same-package references are resolvable without a build)", a+" ~ "+b)
				continue
			}
			sa, oka := loadGoSymbols(res, syms, parseFailed, a)
			sb, okb := loadGoSymbols(res, syms, parseFailed, b)
			if !oka || !okb {
				bad := a
				if oka {
					bad = b
				}
				noteGap("Go file that would not parse: "+parseFailed[bad], a+" ~ "+b)
				continue
			}
			res.Coverage.AnalysedPairs++
			// Both directions: either shard can be the one that changes a
			// declaration the other calls.
			reportCoupling(add, shardOf, a, sa, b, sb)
			reportCoupling(add, shardOf, b, sb, a, sa)
		}
	}

	for _, class := range sortedKeys(gapCount) {
		res.Coverage.Gaps = append(res.Coverage.Gaps,
			fmt.Sprintf("%d cross-shard pair(s) unanalysed — %s; e.g. %s",
				gapCount[class], class, strings.Join(gapExample[class], ", ")))
	}
}

// reportCoupling records every declaration `from` owns that `to` references.
// Deterministic order matters — a checker whose findings reorder between runs
// cannot be diffed in CI.
func reportCoupling(add func(string, string, ...any), shardOf map[string]string, from string, fromSyms goFileSymbols, to string, toSyms goFileSymbols) {
	var hits []string
	for name := range fromSyms.Declares {
		if toSyms.Mentions[name] {
			hits = append(hits, name)
		}
	}
	sort.Strings(hits)
	for _, name := range hits {
		add("symbol-coupling",
			"%s (shard %q) declares %s, which %s (shard %q) references — a change to that declaration in one shard breaks the other with NO textual conflict, and both shards stay green in isolation",
			from, shardOf[from], name, to, shardOf[to])
	}
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func loadGoSymbols(res *shardResult, cache map[string]goFileSymbols, failed map[string]string, rel string) (goFileSymbols, bool) {
	if s, ok := cache[rel]; ok {
		return s, true
	}
	if _, bad := failed[rel]; bad {
		return goFileSymbols{}, false
	}
	src, err := os.ReadFile(filepath.Join(res.root, filepath.FromSlash(rel)))
	if err != nil {
		failed[rel] = err.Error()
		return goFileSymbols{}, false
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
	if err != nil {
		failed[rel] = err.Error()
		return goFileSymbols{}, false
	}
	s := goFileSymbols{Declares: map[string]bool{}, Mentions: map[string]bool{}}
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			// A method's name is only meaningful with its receiver type, and the
			// receiver type is itself a declaration one shard may own — so record
			// the method name too rather than skipping methods entirely. An
			// over-broad record refuses a split; an under-broad one approves a
			// bad one.
			if d.Name != nil {
				s.Declares[d.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch sp := spec.(type) {
				case *ast.TypeSpec:
					if sp.Name != nil {
						s.Declares[sp.Name.Name] = true
					}
				case *ast.ValueSpec:
					for _, n := range sp.Names {
						s.Declares[n.Name] = true
					}
				}
			}
		}
	}
	// Mentions is deliberately OVER-broad: it includes selector tails (`x.Foo`)
	// as well as bare identifiers, so a method call couples to a same-named
	// package-level declaration it may have nothing to do with. That direction
	// is chosen on purpose — an over-broad mention REFUSES a split (costing
	// wall-clock), while a narrow one APPROVES a split that breaks main.
	//
	// The one exclusion is the package clause. Every file in a package names
	// the package, so without this a `func main` in one file would "couple" to
	// every other file in package main — an artifact of the syntax, present in
	// every plan, and therefore information-free.
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id != file.Name {
			s.Mentions[id.Name] = true
		}
		return true
	})
	cache[rel] = s
	return s, true
}

// runShardcheck is the `statusgen shardcheck` sub-command: read a brief, decide
// whether its declared split may be dispatched in parallel, and say why in
// terms a dispatcher can act on without re-deriving anything.
func runShardcheck(args []string, stdout, stderr *os.File) int {
	flags := flag.NewFlagSet("shardcheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	briefPath := flags.String("brief", "", "brief file whose parallel-streams: split to check")
	root := flags.String("root", ".", "repository root the shard globs are resolved against")
	var shards shardFlags
	flags.Var(&shards, "shard", "check an ad-hoc shard instead of a brief's: name=glob[,glob] (repeatable)")
	if err := flags.Parse(args); err != nil {
		return shardExitCouldNot
	}
	// --shard is the dispatcher's form: a split can be checked BEFORE anyone
	// commits it to a brief, which is the only way a dispatcher can evaluate a
	// candidate split without first authoring the declaration it may have to
	// withdraw. Mutually exclusive with --brief because two sources for one
	// plan is two plans.
	if len(shards) > 0 {
		if *briefPath != "" {
			fmt.Fprintln(stderr, "statusgen shardcheck: --brief and --shard are alternatives; pass one")
			return shardExitCouldNot
		}
		return reportShardPlan(stdout, "the ad-hoc plan", *root, shards)
	}
	if *briefPath == "" {
		fmt.Fprintln(stderr, "statusgen shardcheck: --brief <path> or --shard name=glob is required")
		return shardExitCouldNot
	}
	bf, ok, err := parseBriefFile(*briefPath)
	if err != nil {
		fmt.Fprintf(stderr, "could-not-check: %v\n", err)
		return shardExitCouldNot
	}
	if !ok || bf == nil {
		fmt.Fprintf(stderr, "could-not-check: %s is not a schema: brief-v1 file — a split can only be declared in brief-v1 frontmatter\n", *briefPath)
		return shardExitCouldNot
	}
	if len(bf.ParallelStreams) == 0 {
		fmt.Fprintf(stdout, "SPLIT: none-declared — %s dispatches to ONE worker (unchanged behaviour)\n", bf.Brief)
		return shardExitClean
	}
	return reportShardPlan(stdout, bf.Brief, *root, bf.ParallelStreams)
}

// reportShardPlan runs the precondition and prints a verdict a dispatcher can
// act on without re-deriving anything: every collision with its class, every
// coverage gap, and — on any non-clean state — the instruction to run serially.
func reportShardPlan(stdout *os.File, subject, root string, streams []ParallelStream) int {
	res, err := checkShardPlan(root, streams)
	if err != nil {
		fmt.Fprintf(stdout, "SPLIT: REFUSED (could-not-check) — %v\n", err)
		return shardExitCouldNot
	}
	for _, s := range streams {
		fmt.Fprintf(stdout, "shard %-16s %d file(s)\n", s.Name, len(res.Files[s.Name]))
	}
	for _, f := range res.Findings {
		fmt.Fprintf(stdout, "COLLISION [%s] %s\n", f.Class, f.Detail)
	}
	for _, g := range res.Coverage.Gaps {
		fmt.Fprintf(stdout, "COVERAGE-GAP %s\n", g)
	}
	fmt.Fprintf(stdout, "coupling analysis covered %d cross-shard pair(s); %d gap class(es)\n",
		res.Coverage.AnalysedPairs, len(res.Coverage.Gaps))
	switch res.state() {
	case shardExitClean:
		fmt.Fprintf(stdout, "SPLIT: approved (checked-clean) — %s may dispatch %d concurrent shards\n", subject, len(streams))
		return shardExitClean
	case shardExitFailed:
		fmt.Fprintf(stdout, "SPLIT: REFUSED (checked-failed) — dispatch %s to ONE worker, serially\n", subject)
		return shardExitFailed
	default:
		fmt.Fprintf(stdout, "SPLIT: REFUSED (could-not-check) — dispatch %s to ONE worker, serially; an unproven split is not an approved one\n", subject)
		return shardExitCouldNot
	}
}

// shardFlags collects repeatable `--shard name=glob[,glob]` values.
type shardFlags []ParallelStream

func (f *shardFlags) String() string {
	var names []string
	for _, s := range *f {
		names = append(names, s.Name)
	}
	return strings.Join(names, ",")
}

func (f *shardFlags) Set(v string) error {
	name, globs, ok := strings.Cut(v, "=")
	if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(globs) == "" {
		return fmt.Errorf("want name=glob[,glob], got %q", v)
	}
	var files []string
	for _, g := range strings.Split(globs, ",") {
		if g = strings.TrimSpace(g); g != "" {
			files = append(files, g)
		}
	}
	*f = append(*f, ParallelStream{Name: strings.TrimSpace(name), Files: files})
	return nil
}
