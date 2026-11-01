package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
func afReadSetFixture(t *testing.T, recorded []string) string {
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
		"only standard packages":     {Dir: ".", Args: []string{"go", "vet", "fmt"}},
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
