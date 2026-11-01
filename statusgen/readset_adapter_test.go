package main

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// gitFullHead is root's full HEAD commit.
func gitFullHead(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// regression: #2026
// TestGoFloorFromWitnessTree — build-less-brittle/15 row 13: the floor is
// computed by the go tool over the exported witness tree, with the recorded
// -tags and package arguments, and fails closed on a missing dependency.
func TestGoFloorFromWitnessTree(t *testing.T) {
	root := t.TempDir()
	writes(
		"go.mod", "module example.com/fl\n\ngo 1.22\n",
		"p/p.go", "package p\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar D string\n",
		"p/data.txt", "fixture\n",
		"p/c.go", "package p\n\n// int one(void) { return 1; }\nimport \"C\"\n\nfunc One() int { return int(C.one()) }\n",
		"p/extra.go", "//go:build extra\n\npackage p\n\nimport _ \"example.com/fl/r\"\n",
		"p/never.go", "//go:build never\n\npackage p\n",
		"p/p_x_test.go", "package p_test\n\nimport _ \"example.com/fl/p\"\n",
		"r/r.go", "package r\n",
		"q/q.go", "package q\n",
	)(t, root)
	mustGitInit(t, root)
	commit := gitFullHead(t, root)
	src := newReadSetSource(root)
	defer src.close()

	g := evaluation.GoRun{Dir: ".", Args: []string{"go", "test", "-tags=extra", "-count=1", "./p"}, CGOEnabled: "1"}
	fl := src.goFloor(commit, g)
	if fl.Problem != "" {
		t.Fatalf("floor problem: %s", fl.Problem)
	}
	for _, want := range []string{"go.mod", "p/c.go", "p/data.txt", "p/never.go", "p/p_x_test.go", "p/p.go", "p/extra.go", "r/r.go"} {
		if !slices.Contains(fl.Files, want) {
			t.Errorf("floor %v omits %s", fl.Files, want)
		}
	}
	if slices.Contains(fl.Files, "q/q.go") {
		t.Errorf("floor %v lists q/q.go: the recorded package argument ./p did not reach go list", fl.Files)
	}
	// Every floor entry is a file of the tree: the generated test main go
	// list names for a package with test files is not one.
	for _, f := range fl.Files {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(f))); err != nil {
			t.Errorf("floor entry %s is not in the witness tree", f)
		}
	}
	// Without the recorded tag, r is not a dependency: the tag reached go list.
	untagged := src.goFloor(commit, evaluation.GoRun{Dir: ".", Args: []string{"go", "test", "./p"}, CGOEnabled: "1"})
	if untagged.Problem != "" || slices.Contains(untagged.Files, "r/r.go") {
		t.Errorf("untagged floor lists r/r.go or failed: %v (%s)", untagged.Files, untagged.Problem)
	}
	// -C and a recorded working directory resolve against the exported tree.
	if c := src.goFloor(commit, evaluation.GoRun{Dir: ".", Args: []string{"go", "-C", "p", "vet", "."}}); c.Problem != "" || !slices.Contains(c.Files, "p/p.go") {
		t.Errorf("-C floor: %v (%s)", c.Files, c.Problem)
	}

	t.Run("a dependency missing from the module cache is could-not-check", func(t *testing.T) {
		t.Setenv("GOMODCACHE", t.TempDir())
		t.Setenv("GOFLAGS", "")
		dep := t.TempDir()
		writes(
			"go.mod", "module example.com/dep\n\ngo 1.22\n\nrequire example.com/missing v1.0.0\n",
			"go.sum", "",
			"d.go", "package d\n\nimport _ \"example.com/missing\"\n",
		)(t, dep)
		mustGitInit(t, dep)
		s := newReadSetSource(dep)
		defer s.close()
		got := s.goFloor(gitFullHead(t, dep), evaluation.GoRun{Dir: ".", Args: []string{"go", "build", "./..."}})
		if got.Problem == "" || !strings.Contains(got.Problem, "go list failed") {
			t.Fatalf("want a go list problem; got %+v", got)
		}
	})
	t.Run("-modfile and a GOWORK path are problems", func(t *testing.T) {
		for _, g := range []evaluation.GoRun{
			{Dir: ".", Args: []string{"go", "test", "-modfile=alt.mod", "./p"}},
			{Dir: ".", Args: []string{"go", "test", "./p"}, GOWORK: "/elsewhere/go.work"},
			{Dir: ".", Args: []string{"go", "test", "./p"}, GOFLAGS: "-overlay=o.json"},
		} {
			if fl := src.goFloor(commit, g); fl.Problem == "" {
				t.Errorf("%+v: want a problem, got %v", g, fl.Files)
			}
		}
	})
}

// afReadSetFixture builds the af fixture with a go module, commits it, and
// lands af/01's witness at that commit carrying a read-set token and file
// recording `recorded`. It returns the root.
func afReadSetFixture(t *testing.T, recorded []string, extra ...string) string {
	t.Helper()
	root, _ := loadAFStreams(t)
	briefRel := "docs/streams/af/brief-01-model-approved-at-head.md"
	p := filepath.Join(root, filepath.FromSlash(briefRel))
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	i, j := strings.Index(string(raw), "## Evidence\n"), strings.Index(string(raw), "## Review")
	if i < 0 || j < i {
		t.Fatal("af/01 Evidence section not found: the fixture drifted")
	}
	// The module commit empties af/01's Evidence, so every line of the run
	// the verifier lands next is the verifier's own (flipProvenance).
	head, tail := string(raw[:i])+"## Evidence\n\n", string(raw[j:])
	if err := os.WriteFile(p, []byte(head+tail), 0o644); err != nil {
		t.Fatal(err)
	}
	writes(
		"go.mod", "module example.com/af\n\ngo 1.22\n",
		"a.go", "package af\n\nfunc A() int { return B() }\n",
		"b.go", "package af\n\nfunc B() int { return 1 }\n",
	)(t, root)
	writes(extra...)(t, root)
	mustGitCommitAll(t, root, "fixture: af module")
	full := gitFullHead(t, root)
	entries := []string{evaluation.GoRunEntry(evaluation.GoRun{Dir: ".", Args: []string{"go", "vet", "./..."}})}
	for _, p := range recorded {
		entries = append(entries, "read "+p)
	}
	content := evaluation.FormatReadSet(evaluation.ReadSetHeader{
		Commit: full, Command: evaluation.ReadSetCommandDigest("go vet ./..."), Brief: briefRel, Row: "1",
	}, entries)
	digest := evaluation.ReadSetDigest(content)
	mustWriteFile(t, root, "docs/streams/af/reads/"+digest, string(content))
	row := "| 1 | `go vet ./...` | pass exit=0 | sha256:abc123def456 | 2026-07-08 | " + vfRunner + " @ " +
		full[:treeSHALen] + " " + evaluation.ReadSetCellToken(digest, "") + " |"
	evidence := witnessTableFor(row) + "\n\n**VERIFY: PASS**\n\n"
	if err := os.WriteFile(p, []byte(head+evidence+tail), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailVerifier) // the bound verifier lands the row and its file in one commit
	return root
}

// coverageAF01 runs the three callers and returns autoflip's af/01 result,
// the reconcile fold's af/01 released bit, and the --coverage af/01 line.
func coverageAF01(t *testing.T, root string) (modelFlipResult, bool, string) {
	t.Helper()
	resetReadMemo()
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	results, err := autoFlipModel(root, streams, afSource(), ghReviewer(afReviewer), afNow, true)
	if err != nil {
		t.Fatal(err)
	}
	resetReadMemo()
	data, fstreams, err := loadReconcileFoldData(root)
	if err != nil {
		t.Fatal(err)
	}
	tree := treeFoldInputs(root, fstreams, data)
	resetReadMemo()
	out := captureStdout(t, func() { runCoverage(root, false) })
	line := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "af/01 ") {
			line = l
		}
	}
	return afResult(t, results, "af/01"), tree.witnesses["af/01"].Released, line
}

// regression: #2026
// TestCoverageCallersApplyFloor — build-less-brittle/15 row 11: autoflip's
// dry run, the reconcile fold and --coverage all take coverageOptionsFor, so
// a recorded read set releases an unrelated change through each, and a
// compiled file planted missing from the record refuses through each with the
// floor reason — the floor computed by the adapter, never injected.
func TestCoverageCallersApplyFloor(t *testing.T) {
	t.Run("a recorded witness releases an unrecorded change", func(t *testing.T) {
		root := afReadSetFixture(t, []string{"a.go", "b.go", "go.mod"})
		mustWriteFile(t, root, ".claude-plugin/marketplace.json", "{}\n")
		mustGitCommitAll(t, root, "fixture: unrelated change")
		flip, folded, line := coverageAF01(t, root)
		if flip.Outcome != flipDone {
			t.Errorf("autoflip dry run: want flipDone; got %v (%s)", flip.Outcome, flip.Reason)
		}
		if !folded {
			t.Errorf("reconcile fold: want af/01 folded released")
		}
		if !strings.HasPrefix(line, "af/01 released ") {
			t.Errorf("--coverage: want af/01 released; got %q", line)
		}
	})
	t.Run("a complete record of a package with test files releases", func(t *testing.T) {
		root := afReadSetFixture(t, []string{"a.go", "a_test.go", "b.go", "go.mod"},
			"a_test.go", "package af\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { _ = A() }\n")
		mustWriteFile(t, root, ".claude-plugin/marketplace.json", "{}\n")
		mustGitCommitAll(t, root, "fixture: unrelated change")
		flip, folded, line := coverageAF01(t, root)
		if flip.Outcome != flipDone || !folded || !strings.HasPrefix(line, "af/01 released ") {
			t.Errorf("want a release through all three callers; got %v (%s), folded %v, %q", flip.Outcome, flip.Reason, folded, line)
		}
	})
	t.Run("a compiled file missing from the record refuses with the floor reason", func(t *testing.T) {
		root := afReadSetFixture(t, []string{"a.go", "go.mod"})
		mustWriteFile(t, root, "b.go", "package af\n\nfunc B() int { return 2 }\n")
		mustGitCommitAll(t, root, "fixture: change only the unrecorded compiled file")
		flip, folded, line := coverageAF01(t, root)
		if flip.Outcome == flipDone {
			t.Errorf("autoflip dry run: want a refusal; got flipDone (%s)", flip.Reason)
		}
		if !strings.Contains(flip.Reason, evaluation.ReasonFloorOmits) {
			t.Errorf("autoflip reason %q lacks the floor reason", flip.Reason)
		}
		if folded {
			t.Errorf("reconcile fold: want af/01 not released")
		}
		if !strings.HasPrefix(line, "af/01 held ") || !strings.Contains(line, evaluation.ReasonFloorOmits) {
			t.Errorf("--coverage: want af/01 held with the floor reason; got %q", line)
		}
	})
}

// TestReadSetIntroObservation — the adapter's introduction observation, read
// through the in-process history walk and the batch object read: the earliest
// commit holding the row names its Runner cell and whether the read-set file
// arrived in that same commit. A file or token a later commit added never
// moves the introduction.
func TestReadSetIntroObservation(t *testing.T) {
	const briefRel = "docs/streams/s/brief-01-x.md"
	digest := strings.Repeat("ab", 32)
	token := evaluation.ReadSetCellToken(digest, "")
	rowWith := func(tok string) string {
		r := "| 1 | `true` | pass exit=0 | sha256:6f1a0b3c9d22 | 2026-08-13 | " + vfRunner + " @ b988d175ab12"
		if tok != "" {
			r += " " + tok
		}
		return r + " |"
	}
	brief := func(row string) string { return "# Brief\n\n## Evidence\n\n" + witnessTableFor(row) + "\n" }
	fileRel := "docs/streams/s/reads/" + digest
	cases := []struct {
		name              string
		first, second     map[string]string // files written before each commit
		wantFile, wantTok bool
	}{
		{"row and file in one commit", map[string]string{briefRel: brief(rowWith(token)), fileRel: "x\n"}, nil, true, true},
		{"file a commit later", map[string]string{briefRel: brief(rowWith(token))}, map[string]string{fileRel: "x\n"}, false, true},
		{"token and file a commit later", map[string]string{briefRel: brief(rowWith(""))}, map[string]string{briefRel: brief(rowWith(token)), fileRel: "x\n"}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			for p, body := range c.first {
				mustWriteFile(t, root, p, body)
			}
			mustGitInit(t, root)
			if c.second != nil {
				for p, body := range c.second {
					mustWriteFile(t, root, p, body)
				}
				mustGitCommitAll(t, root, "fixture: later")
			}
			src := newReadSetSource(root)
			defer src.close()
			w := evaluation.Witness{Command: "true", Date: "2026-08-13", Runner: vfRunner + " @ b988d175ab12 " + token}
			src.observe(filepath.Join(root, filepath.FromSlash(briefRel)), briefRel, "1", &w)
			o := w.ReadSet
			if o == nil || !o.Found || !o.IntroFound {
				t.Fatalf("observation %+v: want the file found and the row's introduction found", o)
			}
			if o.IntroFile != c.wantFile {
				t.Errorf("IntroFile = %v, want %v", o.IntroFile, c.wantFile)
			}
			if got := strings.Contains(o.IntroRunner, token); got != c.wantTok {
				t.Errorf("introduction Runner %q carries the token = %v, want %v", o.IntroRunner, got, c.wantTok)
			}
		})
	}
}

// goFloorFixture is a two-package module committed in a fresh repository, with
// its read-set source; the source is closed when the test ends.
func goFloorFixture(t *testing.T) (*readSetSource, string) {
	t.Helper()
	root := t.TempDir()
	writes(
		"go.mod", "module example.com/fl\n\ngo 1.22\n",
		"p/p.go", "package p\n\nimport _ \"example.com/fl/r\"\n",
		"r/r.go", "package r\n",
	)(t, root)
	mustGitInit(t, root)
	src := newReadSetSource(root)
	t.Cleanup(src.close)
	return src, gitFullHead(t, root)
}

// regression: #2026
// TestFloorRefusesUnreadFlags — the floor reads a recorded invocation through
// an allow-list: a flag, GOFLAGS word, directory or package argument it does
// not reproduce is a problem, in either spelling, never a narrower floor and
// never a value handed to the go tool.
func TestFloorRefusesUnreadFlags(t *testing.T) {
	src, commit := goFloorFixture(t)
	outside := t.TempDir()
	test := func(extra ...string) []string { return append([]string{"go", "test"}, append(extra, "./p")...) }
	cases := map[string]evaluation.GoRun{
		"-toolexec":                {Dir: ".", Args: test("-toolexec=x")},
		"--toolexec":               {Dir: ".", Args: test("--toolexec=x")},
		"-toolexec, value apart":   {Dir: ".", Args: test("-toolexec", "x")},
		"-exec":                    {Dir: ".", Args: test("-exec=x")},
		"-ldflags":                 {Dir: ".", Args: test("-ldflags=-s")},
		"-modfile":                 {Dir: ".", Args: test("-modfile=alt.mod")},
		"-overlay":                 {Dir: ".", Args: test("-overlay=o.json")},
		"-mod=mod":                 {Dir: ".", Args: test("-mod=mod")},
		"-e over a missing pkg":    {Dir: ".", Args: []string{"go", "test", "-e", "./missing"}},
		"GOFLAGS -toolexec":        {Dir: ".", Args: test(), GOFLAGS: "-toolexec=x"},
		"GOFLAGS --toolexec":       {Dir: ".", Args: test(), GOFLAGS: "--toolexec=x"},
		"GOFLAGS -e, missing pkg":  {Dir: ".", Args: []string{"go", "test", "./missing"}, GOFLAGS: "-e"},
		"GOFLAGS bare word":        {Dir: ".", Args: test(), GOFLAGS: "toolexec=x"},
		"GOFLAGS -tags, no value":  {Dir: ".", Args: test(), GOFLAGS: "-tags"},
		"GOFLAGS -mod=mod":         {Dir: ".", Args: test(), GOFLAGS: "-mod=mod"},
		"-tags with a separator":   {Dir: ".", Args: test("-tags=a;b")},
		"-C absolute":              {Dir: ".", Args: []string{"go", "-C", outside, "test", "."}},
		"-C= absolute":             {Dir: ".", Args: []string{"go", "-C=" + outside, "test", "."}},
		"-C climbing":              {Dir: ".", Args: []string{"go", "-C", "../x", "test", "."}},
		"-C climbing from a dir":   {Dir: "p", Args: []string{"go", "-C", "../..", "test", "."}},
		"dir climbing":             {Dir: "../x", Args: test()},
		"absolute package":         {Dir: ".", Args: []string{"go", "test", outside}},
		"climbing package":         {Dir: ".", Args: []string{"go", "test", "../x"}},
		"versioned package":        {Dir: ".", Args: []string{"go", "run", "example.com/tool@latest"}},
		"GOWORK path":              {Dir: ".", Args: test(), GOWORK: "elsewhere/go.work"},
		"GOOS not a platform":      {Dir: ".", Args: test(), GOOS: "linux x"},
		"CGO_ENABLED not 0 or 1":   {Dir: ".", Args: test(), CGOEnabled: "yes"},
		"no subcommand after a -C": {Dir: ".", Args: []string{"go", "-C", "p"}},
	}
	for name, g := range cases {
		if fl := src.goFloor(commit, g); fl.Problem == "" || len(fl.Files) != 0 {
			t.Errorf("%s: want a problem and no files; got %v (%q)", name, fl.Files, fl.Problem)
		}
	}
	// Flags that never change what compiles are read and dropped; a relative
	// -C resolves inside the tree.
	for name, g := range map[string]evaluation.GoRun{
		"ignored flags": {Dir: ".", Args: test("-count=1", "-run", "X", "-v", "-timeout", "5m", "-race=false")},
		"relative -C":   {Dir: ".", Args: []string{"go", "-C", "p", "test", "."}},
		"GOFLAGS -mod":  {Dir: ".", Args: test(), GOFLAGS: "-mod=readonly -count=1"},
	} {
		if fl := src.goFloor(commit, g); fl.Problem != "" || !slices.Contains(fl.Files, "p/p.go") || !slices.Contains(fl.Files, "r/r.go") {
			t.Errorf("%s: want p/p.go and r/r.go; got %v (%q)", name, fl.Files, fl.Problem)
		}
	}
}

// regression: #2026
// TestFloorEmptyIsProblem — a floor with no file of the witness tree is a
// problem, never an empty floor the library would read as nothing omitted.
func TestFloorEmptyIsProblem(t *testing.T) {
	src, commit := goFloorFixture(t)
	for name, g := range map[string]evaluation.GoRun{
		"a pattern matching nothing": {Dir: ".", Args: []string{"go", "vet", "./nomatch/..."}},
		// p exists, so go list succeeds and names no package at all: this
		// case reaches the no-in-tree-file guard itself.
		"a pattern under a package matching nothing": {Dir: ".", Args: []string{"go", "vet", "./p/nothing..."}},
		"only standard packages":                     {Dir: ".", Args: []string{"go", "vet", "fmt"}},
	} {
		if fl := src.goFloor(commit, g); fl.Problem == "" {
			t.Errorf("%s: want a problem; got the floor %v", name, fl.Files)
		}
	}
}

// regression: #2026
// TestFloorEnvIsBuilt — the go tool runs with an environment built from
// nothing the caller set: ambient GOFLAGS, proxy, experiment, workspace and
// any other variable never reach it, and the pins hold.
func TestFloorEnvIsBuilt(t *testing.T) {
	for k, v := range map[string]string{
		"GOFLAGS": "-toolexec=x", "GOPROXY": "https://proxy.invalid", "GOINSECURE": "*", "GOEXPERIMENT": "nosuchexperiment",
		"GOWORK": "elsewhere/go.work", "GONOSUMDB": "*", "GOTOOLCHAIN": "go1.99.0", "FLOOR_PROBE_SECRET": "leaked",
	} {
		t.Setenv(k, v)
	}
	scratch := t.TempDir()
	tool := &floorTool{goroot: filepath.Join(scratch, "goroot"), modcache: filepath.Join(scratch, "mod"), goos: "linux", goarch: "amd64", cgo: "0"}
	env := floorEnv(tool, scratch, floorPlan{goos: "darwin"})
	got := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		if _, dup := got[k]; dup {
			t.Errorf("%s is set twice", k)
		}
		got[k] = v
	}
	for k, want := range map[string]string{
		"GOFLAGS": "", "GOPROXY": "off", "GOSUMDB": "off", "GOVCS": "*:off", "GOTOOLCHAIN": "local", "GOENV": "off",
		"GOEXPERIMENT": "", "GOINSECURE": "", "GONOSUMDB": "", "GO111MODULE": "on", "GOOS": "darwin", "GOARCH": "amd64",
		"CGO_ENABLED": "0", "GOMODCACHE": tool.modcache, "GOROOT": tool.goroot, "PATH": filepath.Join(tool.goroot, "bin"),
	} {
		if v, ok := got[k]; !ok || v != want {
			t.Errorf("%s = %q (set %v), want %q", k, v, ok, want)
		}
	}
	for _, k := range []string{"FLOOR_PROBE_SECRET", "GOWORK"} {
		if _, ok := got[k]; ok {
			t.Errorf("%s reached the floor's environment", k)
		}
	}
	for _, k := range []string{"HOME", "GOPATH", "GOCACHE", "TMPDIR"} {
		if rel, ok := inRoots([]string{scratch}, got[k]); !ok || rel == "." {
			t.Errorf("%s = %q, want a directory of the floor's own scratch", k, got[k])
		}
	}
	// And through the go tool: the ambient settings above change nothing.
	src, commit := goFloorFixture(t)
	if fl := src.goFloor(commit, evaluation.GoRun{Dir: ".", Args: []string{"go", "test", "./p"}}); fl.Problem != "" || !slices.Contains(fl.Files, "r/r.go") {
		t.Errorf("floor under ambient settings: %v (%q)", fl.Files, fl.Problem)
	}
}

// regression: #2026
// TestFloorRunIsBounded — a go tool run that outlives floorTimeout is a
// problem, never a wait without end.
func TestFloorRunIsBounded(t *testing.T) {
	src, commit := goFloorFixture(t)
	g := evaluation.GoRun{Dir: ".", Args: []string{"go", "test", "./p"}}
	if fl := src.goFloor(commit, g); fl.Problem != "" {
		t.Fatalf("control floor: %q", fl.Problem)
	}
	defer func(d time.Duration) { floorTimeout = d }(floorTimeout)
	floorTimeout = time.Nanosecond
	if fl := src.goFloor(commit, g); !strings.Contains(fl.Problem, "did not finish") {
		t.Errorf("want a did-not-finish problem; got %v (%q)", fl.Files, fl.Problem)
	}
}

// gitPipe runs git in dir with stdin and returns its trimmed output.
func gitPipe(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Stdin = dir, strings.NewReader(stdin)
	cmd.Env = append(fixtureGitEnv(os.Environ()), fixtureGitIdentity...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// regression: #2026
// TestExportTreeContains — a crafted tree never writes outside the export
// directory: a `..` or `.git` component, two names equal ignoring case, a
// duplicate name, a path through a link, and a link out of the tree are each
// an error, the partial export is removed, and nothing lands beside it.
func TestExportTreeContains(t *testing.T) {
	repo := t.TempDir()
	writes("seed.txt", "seed\n")(t, repo)
	mustGitInit(t, repo)
	obj := func(body string) string { return gitPipe(t, repo, body, "hash-object", "-w", "--stdin") }
	tree := func(lines ...string) string { return gitPipe(t, repo, strings.Join(lines, "\n")+"\n", "mktree") }
	blob := obj("planted\n")
	sub := tree("100644 blob " + blob + "\tx")
	climb := tree("040000 tree " + tree("040000 tree "+sub+"\t..") + "\t..")
	cases := map[string]string{
		"a .. component":            climb,
		"a .git directory":          tree("040000 tree " + sub + "\t.git"),
		"a .GIT file":               tree("100644 blob " + blob + "\t.GIT"),
		"names equal ignoring case": tree("100644 blob "+blob+"\tA", "100644 blob "+blob+"\ta"),
		"a duplicate name":          tree("100644 blob "+blob+"\td", "040000 tree "+sub+"\td"),
		"a path through a link":     tree("120000 blob "+obj("..")+"\td", "040000 tree "+sub+"\tD"),
		"a link climbing out":       tree("120000 blob " + obj("../../outside") + "\tl"),
		"an absolute link":          tree("120000 blob " + obj("/") + "\tl"),
	}
	for name, tr := range cases {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			inner := filepath.Join(base, "inner")
			if err := os.Mkdir(inner, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TMPDIR", inner)
			commit := gitPipe(t, repo, "", "commit-tree", tr, "-m", "fixture: crafted")
			if dir, err := exportTree(repo, commit); err == nil {
				t.Errorf("want an error; got the export %s", dir)
			}
			for dir, want := range map[string]int{base: 1, inner: 0} {
				if ents, _ := os.ReadDir(dir); len(ents) != want {
					t.Errorf("%s holds %d entries after the refused export, want %d", dir, len(ents), want)
				}
			}
		})
	}
	// An ordinary tree exports: files, an executable and an in-tree link.
	ok := tree("100644 blob "+blob+"\tf", "100755 blob "+blob+"\tx.sh", "120000 blob "+obj("sub/x")+"\tl", "040000 tree "+sub+"\tsub")
	dir, err := exportTree(repo, gitPipe(t, repo, "", "commit-tree", ok, "-m", "fixture: ordinary"))
	if err != nil {
		t.Fatalf("ordinary tree: %v", err)
	}
	defer os.RemoveAll(dir)
	if b, err := os.ReadFile(filepath.Join(dir, "l")); err != nil || string(b) != "planted\n" {
		t.Errorf("in-tree link: %q, %v", b, err)
	}
}

// allocated returns the bytes the process allocated while f ran.
func allocated(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// regression: #2026
// TestExportCapBeforeRead — the export's caps are checked against the listed
// sizes before any content is read, and the content is streamed: a tree whose
// entries repeat one large object past the byte cap is refused having
// allocated far less than one copy of that object, and a tree under the cap
// exports with each distinct object read once, never held whole.
func TestExportCapBeforeRead(t *testing.T) {
	const big = 4 << 20
	repo := t.TempDir()
	writes("seed.txt", "seed\n")(t, repo)
	mustGitInit(t, repo)
	blob := gitPipe(t, repo, strings.Repeat("\x00", big), "hash-object", "-w", "--stdin")
	small := gitPipe(t, repo, "f100", "hash-object", "-w", "--stdin")
	commitOf := func(n int, extra ...string) string {
		var lines []string
		for i := 0; i < n; i++ {
			lines = append(lines, "100644 blob "+blob+"\tf"+strconv.Itoa(100+i))
		}
		tr := gitPipe(t, repo, strings.Join(append(lines, extra...), "\n")+"\n", "mktree")
		return gitPipe(t, repo, "", "commit-tree", tr, "-m", "fixture: repeated object")
	}
	t.Setenv("TMPDIR", t.TempDir())

	over := commitOf(32)
	defer func(n int64) { exportMaxBytes = n }(exportMaxBytes)
	exportMaxBytes = 2 * big
	var err error
	if got := allocated(func() { _, err = exportTree(repo, over) }); got > big/4 {
		t.Errorf("the refused export allocated %d bytes, want under %d: content was read before the cap", got, big/4)
	} else {
		t.Logf("the refused export allocated %d bytes", got)
	}
	if err == nil || !strings.Contains(err.Error(), "export cap") {
		t.Errorf("want the byte cap refusal; got %v", err)
	}

	exportMaxBytes = 64 * big
	under := commitOf(16, "100644 blob "+small+"\tlinkcopy", "120000 blob "+small+"\tl")
	var dir string
	if got := allocated(func() { dir, err = exportTree(repo, under) }); got > big/2 {
		t.Errorf("the export allocated %d bytes, want under %d: content was buffered", got, big/2)
	} else {
		t.Logf("the export under the cap allocated %d bytes", got)
	}
	if err != nil {
		t.Fatalf("export under the cap: %v", err)
	}
	defer os.RemoveAll(dir)
	for _, p := range []string{"f100", "f115"} {
		if fi, err := os.Stat(filepath.Join(dir, p)); err != nil || fi.Size() != big {
			t.Errorf("%s: %v, %v", p, fi, err)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "l")); err != nil || string(b) != strings.Repeat("\x00", big) {
		t.Errorf("link through a repeated object: %d bytes, %v", len(b), err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "linkcopy")); err != nil || string(b) != "f100" {
		t.Errorf("a file sharing the link's object: %q, %v", b, err)
	}

	exportMaxLinkBytes, exportMaxBytes = 2, 64*big
	defer func() { exportMaxLinkBytes = 1 << 20 }()
	if _, err := exportTree(repo, under); err == nil || !strings.Contains(err.Error(), "link targets") {
		t.Errorf("want the link-target cap refusal; got %v", err)
	}

	defer func(n int) { exportMaxEntries = n }(exportMaxEntries)
	exportMaxLinkBytes, exportMaxEntries = 1<<20, 17
	if _, err := exportTree(repo, under); err == nil || !strings.Contains(err.Error(), "entries, over the export cap") {
		t.Errorf("want the entry cap refusal; got %v", err)
	}
}

// regression: #2026
// TestIntroIgnoresCommitDate — the introduction is read from ancestry, never
// from commit dates: a later commit back-dated before the row's first commit
// does not become its introduction, and a row two branches each added has
// none, so neither shape binds a read set.
func TestIntroIgnoresCommitDate(t *testing.T) {
	const briefRel = "docs/streams/s/brief-01-x.md"
	digest := strings.Repeat("cd", 32)
	token := evaluation.ReadSetCellToken(digest, "")
	fileRel := "docs/streams/s/reads/" + digest
	row := func(tok string) string {
		r := "| 1 | `true` | pass exit=0 | sha256:6f1a0b3c9d22 | 2026-08-13 | " + vfRunner + " @ b988d175ab12"
		if tok != "" {
			r += " " + tok
		}
		return "# Brief\n\n## Evidence\n\n" + witnessTableFor(r+" |") + "\n"
	}
	early := []string{"GIT_AUTHOR_DATE=2001-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2001-01-01T00:00:00Z"}
	observe := func(t *testing.T, root string) *evaluation.ReadSetObservation {
		src := newReadSetSource(root)
		defer src.close()
		w := evaluation.Witness{Command: "true", Date: "2026-08-13", Runner: vfRunner + " @ b988d175ab12 " + token}
		src.observe(filepath.Join(root, filepath.FromSlash(briefRel)), briefRel, "1", &w)
		return w.ReadSet
	}
	t.Run("a back-dated later commit", func(t *testing.T) {
		root := t.TempDir()
		mustWriteFile(t, root, briefRel, row(""))
		mustGitInit(t, root)
		mustWriteFile(t, root, briefRel, row(token))
		mustWriteFile(t, root, fileRel, "x\n")
		runGitEnv(t, root, nil, "add", "-A")
		runGitEnv(t, root, early, "commit", "-q", "-m", "fixture: token and file, back-dated")
		o := observe(t, root)
		if o == nil || !o.IntroFound {
			t.Fatalf("observation %+v: want the row's introduction found", o)
		}
		if o.IntroFile || strings.Contains(o.IntroRunner, token) {
			t.Errorf("introduction moved to the back-dated commit: IntroFile=%v Runner=%q", o.IntroFile, o.IntroRunner)
		}
	})
	t.Run("a row two branches added", func(t *testing.T) {
		root := t.TempDir()
		mustWriteFile(t, root, "seed.txt", "seed\n")
		mustGitInit(t, root)
		runGitEnv(t, root, nil, "branch", "side")
		mustWriteFile(t, root, briefRel, row(""))
		mustGitCommitAll(t, root, "fixture: row on the main line")
		runGitEnv(t, root, nil, "checkout", "-q", "side")
		mustWriteFile(t, root, briefRel, row(token))
		mustWriteFile(t, root, fileRel, "x\n")
		runGitEnv(t, root, nil, "add", "-A")
		runGitEnv(t, root, early, "commit", "-q", "-m", "fixture: row, token and file on a side branch, back-dated")
		runGitEnv(t, root, nil, "checkout", "-q", "-")
		runGitEnv(t, root, nil, "merge", "-q", "--no-edit", "-X", "theirs", "side")
		if o := observe(t, root); o == nil || o.IntroFound {
			t.Errorf("observation %+v: want no introduction for a row two branches added", o)
		}
	})
}

// boundsBrief is a brief whose one Evidence row carries a read-set token,
// and the paths and witness that go with it.
func boundsBrief(digest string) (briefRel, fileRel, brief string, w evaluation.Witness) {
	token := evaluation.ReadSetCellToken(digest, "")
	briefRel = "docs/streams/s/brief-01-x.md"
	fileRel = "docs/streams/s/reads/" + digest
	brief = "# Brief\n\n## Evidence\n\n" + witnessTableFor("| 1 | `true` | pass exit=0 | sha256:6f1a0b3c9d22 | 2026-08-13 | "+vfRunner+" @ b988d175ab12 "+token+" |") + "\n"
	w = evaluation.Witness{Command: "true", Date: "2026-08-13", Runner: vfRunner + " @ b988d175ab12 " + token}
	return briefRel, fileRel, brief, w
}

// obsText is an observation without its content.
func obsText(o *evaluation.ReadSetObservation) string {
	if o == nil {
		return "<none>"
	}
	return "found=" + strconv.FormatBool(o.Found) + " intro=" + strconv.FormatBool(o.IntroFound) + " introFile=" + strconv.FormatBool(o.IntroFile) + " problem=" + strconv.Quote(o.Problem)
}

// observeAlloc observes w's read set in root and returns the observation and
// the bytes the observation allocated.
func observeAlloc(root, briefRel string, w evaluation.Witness) (*evaluation.ReadSetObservation, uint64) {
	src := newReadSetSource(root)
	defer src.close()
	got := allocated(func() { src.observe(filepath.Join(root, filepath.FromSlash(briefRel)), briefRel, "1", &w) })
	return w.ReadSet, got
}

// regression: #2026
// TestIntroHistoryBounded — finding an introduction reads history within
// caps checked before any content is read: a brief version over the per-blob
// cap in reachable history (added, then removed again), or a history over the
// total cap, is the observation's problem, having allocated far less than
// that version; and the read-set file's existence at the introduction commit
// is a batch check that reads none of its content.
func TestIntroHistoryBounded(t *testing.T) {
	const big = 8 << 20
	briefRel, fileRel, brief, w := boundsBrief(strings.Repeat("ef", 32))
	defer func(a, b int64) { introMaxBlobBytes, introMaxHistoryBytes = a, b }(introMaxBlobBytes, introMaxHistoryBytes)
	introMaxBlobBytes, introMaxHistoryBytes = big/8, 64*big

	t.Run("an over-size brief version in history", func(t *testing.T) {
		root := t.TempDir()
		writes(briefRel, brief, fileRel, "x\n")(t, root)
		mustGitInit(t, root)
		mustWriteFile(t, root, briefRel, brief+strings.Repeat("\n", big))
		mustGitCommitAll(t, root, "fixture: an over-size version")
		mustWriteFile(t, root, briefRel, brief)
		mustGitCommitAll(t, root, "fixture: restored")
		o, got := observeAlloc(root, briefRel, w)
		t.Logf("the refused history read allocated %d bytes", got)
		if o == nil || !strings.Contains(o.Problem, "over the cap") {
			t.Errorf("observation %s: want the per-version cap problem", obsText(o))
		}
		if got > big/4 {
			t.Errorf("the refused history read allocated %d bytes, want under %d: content was read before the cap", got, big/4)
		}
	})
	t.Run("a history over the total cap", func(t *testing.T) {
		root := t.TempDir()
		writes(briefRel, brief, fileRel, "x\n")(t, root)
		mustGitInit(t, root)
		mustWriteFile(t, root, briefRel, brief+"\n")
		mustGitCommitAll(t, root, "fixture: a second version")
		defer func(n int64) { introMaxHistoryBytes = n }(introMaxHistoryBytes)
		introMaxHistoryBytes = int64(len(brief)) + 1
		if o, _ := observeAlloc(root, briefRel, w); o == nil || !strings.Contains(o.Problem, "history holds more than") {
			t.Errorf("observation %s: want the total cap problem", obsText(o))
		}
	})
	t.Run("the read-set file at the introduction", func(t *testing.T) {
		root := t.TempDir()
		writes(briefRel, brief, fileRel, strings.Repeat("\x00", big))(t, root)
		mustGitInit(t, root)
		mustWriteFile(t, root, fileRel, "x\n")
		mustGitCommitAll(t, root, "fixture: the file replaced")
		o, got := observeAlloc(root, briefRel, w)
		t.Logf("the existence check allocated %d bytes", got)
		if o == nil || o.Problem != "" || !o.Found || !o.IntroFound || !o.IntroFile {
			t.Fatalf("observation %s: want the file found at its introduction", obsText(o))
		}
		if got > big/4 {
			t.Errorf("the existence check allocated %d bytes, want under %d: it read the file's content", got, big/4)
		}
	})
}

// regression: #2026
// TestReadSetFileBounded — the read-set file on disk is read only as a
// regular file within readSetMaxBytes: an over-size file is a problem having
// allocated far less than it, and a symbolic link is a problem, never
// followed.
func TestReadSetFileBounded(t *testing.T) {
	const big = 8 << 20
	briefRel, fileRel, brief, w := boundsBrief(strings.Repeat("ab", 32))
	defer func(n int64) { readSetMaxBytes = n }(readSetMaxBytes)
	readSetMaxBytes = big / 8

	root := t.TempDir()
	writes(briefRel, brief, fileRel, strings.Repeat("\x00", big))(t, root)
	mustGitInit(t, root)
	o, got := observeAlloc(root, briefRel, w)
	t.Logf("the refused read-set file read allocated %d bytes", got)
	if o == nil || o.Found || !strings.Contains(o.Problem, "over the cap") {
		t.Errorf("observation %s: want the read-set file cap problem", obsText(o))
	}
	if got > big/4 {
		t.Errorf("the refused read allocated %d bytes, want under %d", got, big/4)
	}

	if runtime.GOOS == "windows" {
		return // creating a symbolic link needs a privilege there
	}
	linked := t.TempDir()
	outside := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(outside, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writes(briefRel, brief)(t, linked)
	if err := os.MkdirAll(filepath.Join(linked, filepath.FromSlash(filepath.Dir(fileRel))), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(linked, filepath.FromSlash(fileRel))); err != nil {
		t.Fatal(err)
	}
	mustGitInit(t, linked)
	if o, _ := observeAlloc(linked, briefRel, w); o == nil || o.Found || !strings.Contains(o.Problem, "not a regular file") {
		t.Errorf("observation %s: want a linked read-set file refused", obsText(o))
	}
}

// regression: #2026
// TestListingCapStreams — the export's listing is read as it arrives and
// stops at its caps: a tree listing far more entries, or far more bytes, than
// the caps is refused having allocated far less than the listing.
func TestListingCapStreams(t *testing.T) {
	const n = 40000
	repo := t.TempDir()
	writes("seed.txt", "seed\n")(t, repo)
	mustGitInit(t, repo)
	blob := gitPipe(t, repo, "x\n", "hash-object", "-w", "--stdin")
	var b strings.Builder
	for i := 0; i < n; i++ {
		name := strconv.Itoa(i)
		b.WriteString("100644 blob " + blob + "\t" + strings.Repeat("n", 200-len(name)) + name + "\n")
	}
	listing := b.Len()
	commit := gitPipe(t, repo, "", "commit-tree", gitPipe(t, repo, b.String(), "mktree"), "-m", "fixture: a wide tree")
	t.Setenv("TMPDIR", t.TempDir())
	defer func(e int, l int64) { exportMaxEntries, exportMaxListBytes = e, l }(exportMaxEntries, exportMaxListBytes)

	exportMaxEntries = 100
	var err error
	got := allocated(func() { _, err = exportTree(repo, commit) })
	t.Logf("a %d-byte listing: the entry cap refusal allocated %d bytes", listing, got)
	if got > uint64(listing/4) {
		t.Errorf("the entry cap refusal allocated %d bytes, want under %d: the listing was buffered first", got, listing/4)
	}
	if err == nil || !strings.Contains(err.Error(), "entries, over the export cap") {
		t.Errorf("want the entry cap refusal; got %v", err)
	}
	exportMaxEntries, exportMaxListBytes = 2*n, 256<<10
	got = allocated(func() { _, err = exportTree(repo, commit) })
	t.Logf("the listing byte cap refusal allocated %d bytes", got)
	if got > uint64(listing/4) {
		t.Errorf("the listing byte cap refusal allocated %d bytes, want under %d", got, listing/4)
	}
	if err == nil || !strings.Contains(err.Error(), "listing holds more than") {
		t.Errorf("want the listing byte cap refusal; got %v", err)
	}
}

// regression: #2026
// TestFloorEvalBudget — the floor is bounded across one evaluation, whatever
// the number of read sets: past floorEvalRuns go tool runs, or once
// floorEvalBudget has gone to them, a floor is a problem with no run; and
// every export of the evaluation together writes at most exportEvalBytes.
func TestFloorEvalBudget(t *testing.T) {
	src, commit := goFloorFixture(t)
	defer func(r int, d time.Duration, b int64) { floorEvalRuns, floorEvalBudget, exportEvalBytes = r, d, b }(floorEvalRuns, floorEvalBudget, exportEvalBytes)
	p := evaluation.GoRun{Dir: ".", Args: []string{"go", "test", "./p"}}
	r := evaluation.GoRun{Dir: ".", Args: []string{"go", "test", "./r"}}
	// The control floor starts three go tool runs: the one read of the
	// evaluator's tool, go env GOWORK and go list.
	floorEvalRuns = 3
	if fl := src.goFloor(commit, p); fl.Problem != "" {
		t.Fatalf("control floor: %q", fl.Problem)
	}
	if fl := src.goFloor(commit, r); !strings.Contains(fl.Problem, "times, its cap") {
		t.Errorf("want the run cap problem; got %v (%q)", fl.Files, fl.Problem)
	}
	floorEvalRuns, floorEvalBudget = 64, time.Nanosecond
	if fl := src.goFloor(commit, r); !strings.Contains(fl.Problem, "on compile-input floors, its cap") {
		t.Errorf("want the time budget problem; got %v (%q)", fl.Files, fl.Problem)
	}

	root := src.root
	mustWriteFile(t, root, "r/s.go", "package r\n")
	mustGitCommitAll(t, root, "fixture: a second commit")
	second := gitFullHead(t, root)
	d, n, err := exportTreeWithin(root, commit, exportMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(d)
	exportEvalBytes = n
	both := newReadSetSource(root)
	defer both.close()
	if _, why := both.export(commit); why != "" {
		t.Fatalf("first export within the evaluation cap: %s", why)
	}
	if _, why := both.export(second); !strings.Contains(why, "already hold the cap") {
		t.Errorf("want the evaluation export cap problem; got %q", why)
	}
	exportEvalBytes = n - 1
	short := newReadSetSource(root)
	defer short.close()
	if _, why := short.export(commit); !strings.Contains(why, "export cap of "+strconv.FormatInt(n-1, 10)+" bytes") {
		t.Errorf("want the export bounded by what the evaluation has left; got %q", why)
	}
}

// fakeGo puts first on PATH a go tool whose `go env -json` names its own
// GOROOT and whose `go list` runs before, then prints listJSON ($PWD
// substituted). A floor run's PATH is that GOROOT's bin only, so before names
// any other program by its absolute path.
func fakeGo(t *testing.T, before, listJSON string) {
	t.Helper()
	goroot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(goroot, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncase \"$1\" in\nenv)\n  if [ \"$2\" = -json ]; then\n    printf '{\"GOROOT\":\"%s\",\"GOMODCACHE\":\"%s\",\"GOOS\":\"linux\",\"GOARCH\":\"amd64\",\"CGO_ENABLED\":\"0\"}\\n' '" +
		goroot + "' '" + filepath.Join(goroot, "modcache") + "'\n  else\n    echo\n  fi ;;\nlist)\n  " + before + "\n  printf '%s\\n' \"" + listJSON + "\" ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(goroot, "bin", "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(goroot, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// regression: #2026
// TestFloorFakeGoBranches — the floor's own refusals of what the go tool
// says, with a go tool that says it: a listed file the witness tree does not
// hold is a problem, and a tool that exits while a child keeps its output
// open is not waited for past floorWaitDelay.
func TestFloorFakeGoBranches(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell-script go tool on PATH is POSIX-only")
	}
	pkg := `{\"ImportPath\":\"example.com/fl/p\",\"Dir\":\"$PWD/p\",\"GoFiles\":[\"p.go\",\"ghost.go\"],\"Module\":{\"GoMod\":\"$PWD/go.mod\",\"Main\":true}}`
	g := evaluation.GoRun{Dir: ".", Args: []string{"go", "test", "./p"}}
	t.Run("a listed file the tree does not hold", func(t *testing.T) {
		fakeGo(t, ":", pkg)
		src, commit := goFloorFixture(t)
		if fl := src.goFloor(commit, g); !strings.Contains(fl.Problem, "p/ghost.go, which the witness tree does not hold") {
			t.Errorf("want the phantom problem; got %v (%q)", fl.Files, fl.Problem)
		}
	})
	t.Run("a child holding the output open", func(t *testing.T) {
		fakeGo(t, "/bin/sleep 20 &", pkg)
		src, commit := goFloorFixture(t)
		defer func(d time.Duration) { floorWaitDelay = d }(floorWaitDelay)
		floorWaitDelay = 200 * time.Millisecond
		start := time.Now()
		fl := src.goFloor(commit, g)
		if took := time.Since(start); took > 10*time.Second {
			t.Errorf("the floor waited %s for a child holding its output", took)
		}
		if fl.Problem == "" {
			t.Errorf("want a problem for a run whose output was cut off; got %v", fl.Files)
		}
	})
}

// importHistory commits on top of root's HEAD, through one fast-import run,
// one commit per entry of commits, each setting the named paths to the blob
// at the given index of blobs. It returns the full commit ids in order.
func importHistory(t *testing.T, root string, blobs []string, commits []map[string]int) []string {
	t.Helper()
	ref := gitPipe(t, root, "", "symbolic-ref", "HEAD")
	var b strings.Builder
	for i, body := range blobs {
		fmt.Fprintf(&b, "blob\nmark :%d\ndata %d\n%s\n", i+1, len(body), body)
	}
	for i, c := range commits {
		fmt.Fprintf(&b, "commit %s\nmark :%d\ncommitter f <f@example.com> %d +0000\ndata 7\nhistory\n", ref, len(blobs)+i+1, 1700000000+i)
		if i == 0 {
			fmt.Fprintf(&b, "from %s\n", gitFullHead(t, root))
		}
		paths := slices.Sorted(maps.Keys(c))
		for _, p := range paths {
			fmt.Fprintf(&b, "M 100644 :%d %s\n", c[p]+1, p)
		}
	}
	for i := range commits {
		fmt.Fprintf(&b, "get-mark :%d\n", len(blobs)+i+1)
	}
	b.WriteString("done\n")
	marks := strings.Fields(gitPipe(t, root, b.String(), "fast-import", "--quiet", "--done"))
	if len(marks) != len(commits) {
		t.Fatalf("fast-import named %d commits, want %d", len(marks), len(commits))
	}
	return marks
}

// historicRows is a brief version of n witness rows numbered from first,
// none carrying a read-set token, padded with blank lines to size bytes when
// size is larger.
func historicRows(first, n, size int) string {
	var b strings.Builder
	b.WriteString("# Brief\n\n## Evidence\n\n" + witnessHeader + "\n")
	for i := first; i < first+n; i++ {
		fmt.Fprintf(&b, "| %d | `true` | pass exit=0 | sha256:6f1a0b3c9d22 | 2026-08-13 | r |\n", i)
	}
	if pad := size - b.Len(); pad > 0 {
		b.WriteString(strings.Repeat("\n", pad))
	}
	return b.String()
}

// tokenRows is a brief whose n Evidence rows each carry the read-set token
// for digest, with the witnesses that go with them keyed by row id.
func tokenRows(digest string, n int) (string, map[string][]evaluation.Witness) {
	token := evaluation.ReadSetCellToken(digest, "")
	ws := map[string][]evaluation.Witness{}
	var rows []string
	for i := 1; i <= n; i++ {
		id := strconv.Itoa(i)
		rows = append(rows, "| "+id+" | `true` | pass exit=0 | sha256:6f1a0b3c9d22 | 2026-08-13 | "+vfRunner+" @ b988d175ab12 "+token+" |")
		ws[id] = []evaluation.Witness{{Command: "true", Date: "2026-08-13", Runner: vfRunner + " @ b988d175ab12 " + token}}
	}
	return "# Brief\n\n## Evidence\n\n" + witnessTableFor(rows...) + "\n", ws
}

// regression: #2026
// TestCoverageObserveBudgetReadSet — one read-set file named by many token
// rows is read once, and every row's content is charged to the evaluation's
// budget: past it a row is refused, so the bytes the observations allocate
// together stay near one file however many rows name it.
func TestCoverageObserveBudgetReadSet(t *testing.T) {
	const fileCap, rows = 1 << 20, 64
	defer func(a, b int64) { readSetMaxBytes, observeEvalBytes = a, b }(readSetMaxBytes, observeEvalBytes)
	readSetMaxBytes, observeEvalBytes = fileCap, 8*fileCap
	digest := strings.Repeat("a1", 32)
	brief, ws := tokenRows(digest, rows)
	const briefRel = "docs/streams/s/brief-01-x.md"
	root := t.TempDir()
	writes(briefRel, brief, "docs/streams/s/reads/"+digest, strings.Repeat("\n", fileCap-1))(t, root)
	mustGitInit(t, root)
	src := newReadSetSource(root)
	defer src.close()
	got := allocated(func() { src.observeBrief(filepath.Join(root, filepath.FromSlash(briefRel)), briefRel, ws) })
	t.Logf("%d rows naming one %d-byte read-set file allocated %d bytes", rows, fileCap-1, got)
	if got > 6*fileCap {
		t.Errorf("the observations allocated %d bytes, want under %d: the file was read per row", got, 6*fileCap)
	}
	var ok, refused int
	var first *byte
	for id, w := range ws {
		o := w[0].ReadSet
		switch {
		case o == nil:
			t.Fatalf("row %s: no observation", id)
		case o.Problem == "" && o.Found:
			ok++
			if first == nil {
				first = &o.Content[0]
			} else if &o.Content[0] != first {
				t.Errorf("row %s holds its own copy of the read-set file", id)
			}
		case strings.Contains(o.Problem, "observations already hold the cap"):
			refused++
		default:
			t.Errorf("row %s: %s", id, obsText(o))
		}
	}
	if ok == 0 || ok > 8 || ok+refused != rows {
		t.Errorf("%d rows observed and %d refused, want between 1 and 8 observed and the rest refused", ok, refused)
	}
}

// regression: #2026
// TestCoverageObserveBudgetIntroTouches — the introduction walk keeps only
// the keys the brief's token rows ask for: a history that brings a version of
// many rows back across many touching commits allocates about one read of
// that version, whatever the number of touches.
func TestCoverageObserveBudgetIntroTouches(t *testing.T) {
	const touches = 200
	digest := strings.Repeat("b2", 32)
	head, ws := tokenRows(digest, 1)
	wide := historicRows(1000, 20000, 0)
	const briefRel = "docs/streams/s/brief-01-x.md"
	root := t.TempDir()
	writes(briefRel, head)(t, root)
	mustGitInit(t, root)
	var commits []map[string]int
	for i := 0; i < touches; i++ {
		commits = append(commits, map[string]int{briefRel: i % 2})
	}
	importHistory(t, root, []string{wide, head}, commits)
	src := newReadSetSource(root)
	defer src.close()
	got := allocated(func() { src.observeBrief(filepath.Join(root, filepath.FromSlash(briefRel)), briefRel, ws) })
	t.Logf("%d touches of a %d-byte version allocated %d bytes", touches, len(wide), got)
	// Parsing a version allocates about fifteen times its size (each row is
	// split into cells and keyed); the bound is two such reads, whatever the
	// number of touches.
	if limit := 30 * len(wide); got > uint64(limit) {
		t.Errorf("the introduction walk allocated %d bytes, want under %d: it held every row of every touch", got, limit)
	}
	if o := ws["1"][0].ReadSet; o == nil || o.Problem != "" {
		t.Errorf("observation %s: want no problem", obsText(o))
	}
	// The budget is charged the two versions once and, per touch, only the
	// one wanted key: a walk keeping every row is charged every row's key at
	// every touch.
	if limit := int64(len(wide)+len(head)) + touches*1024; src.observed > limit {
		t.Errorf("the walk charged %d bytes to the budget, want at most %d: it compared keys the head does not ask for", src.observed, limit)
	}
}

// regression: #2026
// TestCoverageObserveBudgetSharedHistory — the history bytes of every brief
// are charged to one evaluation budget, versions shared between briefs
// charged for each: many briefs over the same near-cap history are refused
// past the budget, so the bytes they parse together stay near the budget.
func TestCoverageObserveBudgetSharedHistory(t *testing.T) {
	const blob, versions, briefs = 256 << 10, 4, 24
	defer func(a, b, c int64) { introMaxBlobBytes, introMaxHistoryBytes, observeEvalBytes = a, b, c }(introMaxBlobBytes, introMaxHistoryBytes, observeEvalBytes)
	introMaxBlobBytes, introMaxHistoryBytes, observeEvalBytes = blob, versions*blob+(64<<10), 3*versions*blob
	digest := strings.Repeat("c3", 32)
	head, _ := tokenRows(digest, 1)
	var blobs []string
	for v := 0; v < versions; v++ {
		blobs = append(blobs, historicRows(1000+v*4000, 3000, blob-1))
	}
	blobs = append(blobs, head)
	rel := func(i int) string { return fmt.Sprintf("docs/streams/s/brief-%02d-x.md", i+1) }
	root := t.TempDir()
	for i := 0; i < briefs; i++ {
		writes(rel(i), head)(t, root)
	}
	mustGitInit(t, root)
	var commits []map[string]int
	for v := 0; v <= versions; v++ {
		c := map[string]int{}
		for i := 0; i < briefs; i++ {
			c[rel(i)] = v
		}
		commits = append(commits, c)
	}
	importHistory(t, root, blobs, commits)
	src := newReadSetSource(root)
	defer src.close()
	all := map[string]map[string][]evaluation.Witness{}
	for i := 0; i < briefs; i++ {
		_, all[rel(i)] = tokenRows(digest, 1)
	}
	start := time.Now()
	got := allocated(func() {
		for i := 0; i < briefs; i++ {
			src.observeBrief(filepath.Join(root, filepath.FromSlash(rel(i))), rel(i), all[rel(i)])
		}
	})
	t.Logf("%d briefs sharing a %d-byte history allocated %d bytes in %s", briefs, versions*blob, got, time.Since(start))
	// Parsing allocates about fifteen times the bytes parsed; the bound is
	// the budget's worth of parsing, whatever the number of briefs.
	if limit := 16 * observeEvalBytes; got > uint64(limit) {
		t.Errorf("the observations allocated %d bytes, want under %d: shared history was parsed past the budget", got, limit)
	}
	var ok, refused int
	for i := 0; i < briefs; i++ {
		switch o := all[rel(i)]["1"][0].ReadSet; {
		case o != nil && o.Problem == "":
			ok++
		case o != nil && strings.Contains(o.Problem, "observations already hold the cap"):
			refused++
		default:
			t.Errorf("%s: %s", rel(i), obsText(o))
		}
	}
	if ok == 0 || ok > 3 || ok+refused != briefs {
		t.Errorf("%d briefs observed and %d refused, want between 1 and 3 observed and the rest refused", ok, refused)
	}
}

// regression: #2026
// TestCoverageObserveBudgetTouchCompares — each touching commit's comparison
// of the kept keys is charged to the evaluation's budget: a history touching
// the brief's token rows more times than the budget allows is refused, while
// the same versions touched a few times are observed.
func TestCoverageObserveBudgetTouchCompares(t *testing.T) {
	digest := strings.Repeat("d4", 32)
	head, ws := tokenRows(digest, 500)
	defer func(n int64) { observeEvalBytes = n }(observeEvalBytes)
	observeEvalBytes = int64(16 * len(head))
	for _, c := range []struct {
		touches int
		refused bool
	}{{4, false}, {200, true}} {
		root := t.TempDir()
		const briefRel = "docs/streams/s/brief-01-x.md"
		writes(briefRel, head)(t, root)
		mustGitInit(t, root)
		var commits []map[string]int
		for i := 0; i < c.touches; i++ {
			commits = append(commits, map[string]int{briefRel: (i + 1) % 2})
		}
		importHistory(t, root, []string{head, head + "\n"}, commits)
		src := newReadSetSource(root)
		w := maps.Clone(ws)
		for id := range w {
			w[id] = slices.Clone(w[id])
		}
		src.observeBrief(filepath.Join(root, filepath.FromSlash(briefRel)), briefRel, w)
		src.close()
		o := w["1"][0].ReadSet
		if got := o != nil && strings.Contains(o.Problem, "observations already hold the cap"); got != c.refused {
			t.Errorf("%d touches: observation %s, want refused %v", c.touches, obsText(o), c.refused)
		}
	}
}

// regression: #2026
// TestCoverageFloorBudgetCharges — the floor's per-evaluation budgets are
// charged as each step starts, failed or not: every go tool run (the read of
// the evaluator's tool, go env GOWORK, go list) counts against floorEvalRuns;
// an export that fails after it began writing is charged its bytes, and the
// floor its time; and a link target is checked before anything is written.
func TestCoverageFloorBudgetCharges(t *testing.T) {
	defer func(r int, b int64) { floorEvalRuns, exportEvalBytes = r, b }(floorEvalRuns, exportEvalBytes)
	t.Run("each go tool run as it starts", func(t *testing.T) {
		src, commit := goFloorFixture(t)
		floorEvalRuns = 2
		fl := src.goFloor(commit, evaluation.GoRun{Dir: ".", Args: []string{"go", "test", "./p"}})
		if !strings.Contains(fl.Problem, "go tool 2 times, its cap") {
			t.Errorf("want the floor refused at its third go tool run; got %v (%q)", fl.Files, fl.Problem)
		}
		floorEvalRuns = 64
	})

	repo := t.TempDir()
	writes("seed.txt", "seed\n")(t, repo)
	mustGitInit(t, repo)
	rng := rand.New(rand.NewPCG(1, 2))
	noise := func(n int) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(rng.Uint32())
		}
		return string(b)
	}
	const size = 256 << 10
	good := gitPipe(t, repo, noise(size), "hash-object", "-w", "--stdin")
	torn := gitPipe(t, repo, noise(size), "hash-object", "-w", "--stdin")
	obj := filepath.Join(repo, ".git", "objects", torn[:2], torn[2:])
	b, err := os.ReadFile(obj)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(obj, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(obj, b[:len(b)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	commitOf := func(lines ...string) string {
		return gitPipe(t, repo, "", "commit-tree", gitPipe(t, repo, strings.Join(lines, "\n")+"\n", "mktree"), "-m", "fixture: budget")
	}
	bad := commitOf("100644 blob "+good+"\tf1", "100644 blob "+torn+"\tf2")
	small := commitOf("100644 blob " + gitPipe(t, repo, "x\n", "hash-object", "-w", "--stdin") + "\tf")
	t.Setenv("TMPDIR", t.TempDir())

	t.Run("a failed export's bytes and time", func(t *testing.T) {
		exportEvalBytes = 2 * size
		src := newReadSetSource(repo)
		defer src.close()
		if _, why := src.export(bad); why == "" {
			t.Fatal("want the export of a torn object to fail")
		}
		if _, why := src.export(small); !strings.Contains(why, "already hold the cap") {
			t.Errorf("want the failed export charged to the evaluation; got %q", why)
		}
		timed := newReadSetSource(repo)
		defer timed.close()
		if fl := timed.goFloor(bad, evaluation.GoRun{Dir: ".", Args: []string{"go", "test", "./..."}}); fl.Problem == "" {
			t.Fatalf("want a problem for a floor whose export failed; got %v", fl.Files)
		}
		if timed.floorSpent <= 0 {
			t.Errorf("a floor whose export failed charged %s of wall time, want more than none", timed.floorSpent)
		}
	})

	t.Run("a link target before any write", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("a read-only directory is POSIX-only here")
		}
		locked := t.TempDir()
		if err := os.Chmod(locked, 0o555); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(locked, 0o755)
		t.Setenv("TMPDIR", locked)
		climb := commitOf("100644 blob "+good+"\tf1", "120000 blob "+gitPipe(t, repo, "../../outside", "hash-object", "-w", "--stdin")+"\tl")
		_, n, err := exportTreeWithin(repo, climb, exportMaxBytes)
		if err == nil || !strings.Contains(err.Error(), "symbolic link whose target") || n != 0 {
			t.Errorf("want the link refused before the export directory is made, nothing charged; got %d bytes, %v", n, err)
		}
	})
}

// regression: #2026
// TestCoverageFloorOutputCapped — a floor go tool run's output is read as it
// arrives: standard output past floorMaxOutputBytes stops the run and is a
// problem, and standard error is kept only to floorMaxStderrBytes, each having
// allocated far less than the tool wrote.
func TestCoverageFloorOutputCapped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell-script go tool on PATH is POSIX-only")
	}
	const wrote = 16 << 20
	defer func(o int64, d time.Duration) { floorMaxOutputBytes, floorWaitDelay = o, d }(floorMaxOutputBytes, floorWaitDelay)
	floorMaxOutputBytes, floorWaitDelay = 1<<20, 200*time.Millisecond
	pkg := `{\"ImportPath\":\"example.com/fl/p\",\"Dir\":\"$PWD/p\",\"GoFiles\":[\"p.go\"],\"Module\":{\"GoMod\":\"$PWD/go.mod\",\"Main\":true}}`
	g := evaluation.GoRun{Dir: ".", Args: []string{"go", "test", "./p"}}
	for _, c := range []struct{ name, before, want string }{
		{"standard output", "/usr/bin/head -c " + strconv.Itoa(wrote) + ` /dev/zero | /usr/bin/tr '\0' ' '`, "go list wrote more than the cap of 1048576 bytes"},
		{"standard error", "/usr/bin/head -c " + strconv.Itoa(wrote) + " /dev/zero >&2; exit 1", "go list failed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			fakeGo(t, c.before, pkg)
			src, commit := goFloorFixture(t)
			var fl evaluation.GoFloor
			got := allocated(func() { fl = src.goFloor(commit, g) })
			t.Logf("a go list writing %d bytes to %s allocated %d bytes", wrote, c.name, got)
			if !strings.Contains(fl.Problem, c.want) {
				t.Errorf("want %q; got %v (%q)", c.want, fl.Files, fl.Problem)
			}
			if got > wrote/4 {
				t.Errorf("the floor allocated %d bytes, want under %d: the output was read whole", got, wrote/4)
			}
		})
	}
}

// regression: #2026
// TestCoverageFloorOutsideTree — a recorded working directory that resolves,
// through a link in the tree, to a directory outside it is a problem before
// any go tool run starts.
func TestCoverageFloorOutsideTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating a symbolic link needs a privilege there")
	}
	tree, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(tree, "sub")); err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	tool := &floorTool{goroot: filepath.Join(scratch, "goroot"), modcache: filepath.Join(scratch, "mod")}
	fl := runFloor(tool, scratch, tree, floorPlan{dir: "sub"}, func() string { return "a go tool run started" })
	if fl.Problem != "the recorded working directory is outside the tree" {
		t.Errorf("want the outside-tree problem before any run; got %v (%q)", fl.Files, fl.Problem)
	}
}
