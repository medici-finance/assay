package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
