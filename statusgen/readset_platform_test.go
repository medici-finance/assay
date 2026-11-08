package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// regression: #2026
// TestRecorderOffLinuxWritesCouldNotCheck — build-less-brittle/15 row 28:
// off Linux a row run through verifyrun carries the Linux-only could-not-check
// token and writes no read-set file; on Linux the platform-neutral function
// readset_other.go calls yields that token for a non-Linux OS name.
func TestRecorderOffLinuxWritesCouldNotCheck(t *testing.T) {
	const darwin = "(reads could-not-check: recorder is Linux-only (darwin))"
	if got := readSetOffLinuxToken("darwin"); got != darwin {
		t.Fatalf("readSetOffLinuxToken(darwin) = %q, want %q", got, darwin)
	}
	if runtime.GOOS == "linux" {
		return
	}
	want := "(reads could-not-check: recorder is Linux-only (" + runtime.GOOS + "))"
	pinClock(t)
	f := newRowsFixture(t, "")
	res, out := captureVerifyrunOut(t, f.args("--rows", "2"))
	if res.code != verifyrunExitPass {
		t.Fatalf("--rows 2: exit %d %s", res.code, out)
	}
	got := parseEvidenceRows(evidenceOf(t, f.brief))["2"]
	if len(got) != 1 {
		t.Fatalf("row 2 witnesses: %+v", got)
	}
	cells := witnessCells(got[0].Text)
	if cells == nil {
		t.Fatalf("row 2 witness does not parse: %q", got[0].Text)
	}
	runner := strings.TrimSpace(cells[witnessCellRunner])
	if !strings.Contains(runner, " @ ") || !strings.Contains(runner, want) {
		t.Errorf("Runner cell %q lacks %q", runner, want)
	}
	if strings.Contains(runner, "(reads sha256:") {
		t.Errorf("Runner cell %q names a read set off Linux", runner)
	}
	var written []string
	filepath.WalkDir(f.root, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() && d.Name() == "reads" {
			written = append(written, p)
		}
		return nil
	})
	if len(written) > 0 {
		t.Errorf("a read-set directory was written off Linux: %v", written)
	}
}

// regression: #2026
// TestBrief15RecorderRowsNeverPassWithoutRecorder — build-less-brittle/15
// row 29: the brief's Linux-only rows can never pass on a host without the
// recorder, read through verifyrun's own Expect parser, and row 27 needs both
// README phrases.
func TestBrief15RecorderRowsNeverPassWithoutRecorder(t *testing.T) {
	path := filepath.Join("..", "docs", "streams", "build-less-brittle", "brief-15-witness-inputs-are-recorded-not-parsed.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	expects := verifyExpects(t, string(raw))
	for _, id := range []string{"14", "16", "21", "23", "25"} {
		e, ok := expects[id]
		if !ok {
			t.Fatalf("Verify row %s not found", id)
		}
		spec := parseExpect(e)
		for _, c := range []struct {
			res  runResult
			pass bool
		}{
			{runResult{exit: 1}, false},
			{runResult{exit: 1, output: []byte("0\n")}, false},
			{runResult{exit: 0, output: []byte("1\n")}, true},
		} {
			state, note := spec.verdict(c.res)
			if (state == statePass) != c.pass {
				t.Errorf("row %s exit=%d output=%q: state %q (%s), want pass=%v", id, c.res.exit, c.res.output, state, note, c.pass)
			}
		}
	}
	e, ok := expects["27"]
	if !ok {
		t.Fatal("Verify row 27 not found")
	}
	spec := parseExpect(e)
	if state, _ := spec.verdict(runResult{output: []byte("1\n")}); state == statePass {
		t.Errorf("row 27 passes on one phrase")
	}
	if state, note := spec.verdict(runResult{output: []byte("2\n")}); state != statePass {
		t.Errorf("row 27 does not pass on both phrases: %q (%s)", state, note)
	}
}

// verifyExpects maps each Verify row's id to its Expect cell, read the way
// verifyrun reads the table.
func verifyExpects(t *testing.T, brief string) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, r := range briefVerifyRows(extractSectionByPrefix(brief, "Verify")) {
		m[r.ID] = r.Expect
	}
	return m
}

// regression: #2026
// TestEvidenceOnlyAdmitsOwnReadSet — build-less-brittle/15 Task step 7's one
// +dirty change: a brief's own new read-set file, named by a token in its
// working Evidence and hashing to its name, joins the own-brief exemption.
// A file of another brief, one its Evidence does not name, or one whose
// content differs from its name still marks the tree +dirty.
func TestEvidenceOnlyAdmitsOwnReadSet(t *testing.T) {
	content := []byte("assay-read-set v1\n")
	digest := evaluation.ReadSetDigest(content)
	cases := []struct {
		name  string
		file  string
		body  []byte
		token string
		dirty bool
	}{
		{"own named file", "docs/reads/" + digest, content, digest, false},
		{"own file the Evidence does not name", "docs/reads/" + digest, content, strings.Repeat("0", 64), true},
		{"content differs from the name", "docs/reads/" + digest, []byte("other\n"), digest, true},
		{"another brief's directory", "other/reads/" + digest, content, digest, true},
		{"not a digest name", "docs/reads/notes.txt", content, digest, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pinClock(t)
			f := newRowsFixture(t, "")
			row := "| 2 | `true` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | x @ 0123456789ab " + evaluation.ReadSetCellToken(c.token, "") + " |"
			raw := getFile(t, f.brief)
			putFile(t, f.brief, strings.Replace(raw, "## Evidence\n", "## Evidence\n\n"+witnessTableFor(row)+"\n", 1))
			p := filepath.Join(f.root, filepath.FromSlash(c.file))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, c.body, 0o644); err != nil {
				t.Fatal(err)
			}
			cell, ok := hostRunnerCell(f.root, f.brief)
			if !ok {
				t.Fatal("hostRunnerCell failed")
			}
			if got := strings.HasSuffix(cell.RunnerTree, "+dirty"); got != c.dirty {
				t.Errorf("tree %q: dirty=%v, want %v", cell.RunnerTree, got, c.dirty)
			}
		})
	}
}
