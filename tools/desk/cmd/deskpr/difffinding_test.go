package main

import (
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// difffinding_test.go — #3196: a branch-diff secret-scan refusal names the file,
// the line in that file and the side, not the line's position in the stripped diff (#3196).

// plantedRun returns a 40-character mixed-case alphanumeric run from a fixed seed — the
// high-entropy shape the scan refuses — so no credential-shaped literal lives in a test.
func plantedRun(seed int64) string {
	// Built from short pieces so the alphabet is not itself a long high-entropy run.
	const alphabet = "ABCDEFGHJKLMN" + "PQRSTUVWXYZ" + "abcdefghijk" + "mnopqrstuvwxyz" + "23456789"
	r := rand.New(rand.NewSource(seed))
	b := make([]byte, 40)
	for i := range b {
		b[i] = alphabet[r.Intn(len(alphabet))]
	}
	return string(b)
}

// sideFixture builds a branch whose diff against origin/main touches three files —
// alpha.txt, beta.txt and the fixture's feature.txt — with the planted run in beta.txt,
// the second of the three, on the requested side. beta.txt has 700 lines on main; the
// branch inserts five lines at its top, so a head-side line sits five below its base-side
// twin, and the run sits near base line 660 / head line 665 (hunk `@@ -657,7 +662,7 @@`).
// It returns the work tree and the file line and path the refusal must name.
func sideFixture(t *testing.T, side deskkit.DiffSide, run string) (work, path string, line int) {
	t.Helper()
	work = newBaseFixture(t)
	mustGit(t, work, "checkout", "main")
	base := make([]string, 700)
	for i := range base {
		base[i] = fmt.Sprintf("beta line %d", i+1)
	}
	alpha := "alpha one\nalpha two\nalpha three\n"
	if side != deskkit.DiffSideAdded {
		base[659] = "value " + run // base line 660
	}
	writeFile(t, filepath.Join(work, "alpha.txt"), alpha)
	writeFile(t, filepath.Join(work, "beta.txt"), strings.Join(base, "\n")+"\n")
	mustGit(t, work, "add", "alpha.txt", "beta.txt")
	mustGit(t, work, "commit", "-m", "seed alpha and beta")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", mustGit(t, work, "rev-parse", "HEAD"))
	mustGit(t, work, "checkout", "feature/test-branch")
	mustGit(t, work, "merge", "--no-edit", "main")

	head := append([]string{"top 1", "top 2", "top 3", "top 4", "top 5"}, base...)
	switch side {
	case deskkit.DiffSideAdded:
		head[664] = "value " + run // head line 665, replacing base line 660
	case deskkit.DiffSideRemoved:
		head[664] = "value retired" // base line 660 removed
	case deskkit.DiffSideContext:
		head[665] = "beta line 661 edited" // base line 660 stays, as context at head line 665
	}
	writeFile(t, filepath.Join(work, "alpha.txt"), alpha+"alpha four\n")
	writeFile(t, filepath.Join(work, "beta.txt"), strings.Join(head, "\n")+"\n")
	mustGit(t, work, "add", "alpha.txt", "beta.txt")
	mustGit(t, work, "commit", "-m", "edit alpha and beta")
	if side == deskkit.DiffSideRemoved {
		return work, "beta.txt", 660
	}
	return work, "beta.txt", 665
}

// strippedLine is where the scanner itself places the finding: the line in the stripped
// diff text — the number #3196 reported and this brief retires from the output. It also
// returns the scanner's own refusal text, which carries no line at all.
func strippedLine(t *testing.T, work string) (int, string) {
	t.Helper()
	repo, err := gitcore.Open(work)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	diff, err := repo.DiffSymmetric("origin/main", "HEAD", 3)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	var f *deskkit.ScanFinding
	err = deskkit.ScanSurfaceSecrets("branch diff vs refs/remotes/origin/main", []byte(stripDiffMetaLines(diff)))
	if !errors.As(err, &f) {
		t.Fatal("the planted run did not refuse on the stripped diff")
	}
	return f.Line, err.Error()
}

// regression: #3196
func TestDiffFindingFileLine(t *testing.T) {
	for i, side := range []deskkit.DiffSide{deskkit.DiffSideAdded, deskkit.DiffSideRemoved, deskkit.DiffSideContext} {
		t.Run(string(side), func(t *testing.T) {
			work, path, line := sideFixture(t, side, plantedRun(int64(620+i)))
			withEnv(t, work)
			old, scanMsg := strippedLine(t, work)
			if old == line {
				t.Fatalf("fixture cannot tell the lines apart: stripped line = file line = %d", line)
			}

			var err error
			out := captureStderr(t, func() {
				err = cmdCreate([]string{"--title", "fix: a change", "--body-min", "edits beta\nBrief: fixture/01", "--explain"})
			})
			if !deskkit.IsRefused(err) {
				t.Fatalf("want a refusal, got %v", err)
			}
			at := fmt.Sprintf("%s:%d (%s)", path, line, side)
			if !strings.Contains(err.Error(), at) {
				t.Errorf("refusal does not name %q:\n%s", at, err)
			}
			ex := fmt.Sprintf("line=%d path=%s side=%s ", line, path, side)
			if !strings.Contains(out, "scan-explain: rule=high-entropy-run "+ex) {
				t.Errorf("--explain does not carry %q:\n%s", ex, out)
			}
			var f *deskkit.ScanFinding
			if !errors.As(err, &f) || f.Line != line || f.Path != path || f.Side != string(side) {
				t.Errorf("finding = %+v, want line %d path %s side %s", f, line, path, side)
			}
			// The stripped-text line appears nowhere the refusal adds: --explain never prints
			// it, and the refusal carries it no more often than the scanner's own wording does
			// (that wording names no line; a digit run in it is not a location).
			oldRE := regexp.MustCompile(fmt.Sprintf(`(^|[^0-9])%d([^0-9]|$)`, old))
			if oldRE.MatchString(out) {
				t.Errorf("--explain still carries the stripped-text line %d:\n%s", old, out)
			}
			if n, base := len(oldRE.FindAllString(err.Error(), -1)), len(oldRE.FindAllString(scanMsg, -1)); n != base {
				t.Errorf("refusal carries the stripped-text line %d (%d times, the scanner's wording %d):\n%s", old, n, base, err)
			}
			if strings.Contains(err.Error()+out, plantedRun(int64(620+i))) {
				t.Error("the output carries the offending span")
			}
		})
	}
}

// TestDiffFindingVerdicts runs the branch-diff scan over the same diffs with and without
// the index — the legacy surface (the scanner's own finding) against the located one —
// and asserts the same refused-or-clean result, the same rule, exit code and message
// prefix, and the same override path (a non-overridable refusal stays one; an overridable
// one advertises the same override).
func TestDiffFindingVerdicts(t *testing.T) {
	run := plantedRun(626)
	gitFile := func(path string, body ...string) []string {
		out := []string{"diff --git a/" + path + " b/" + path, "index 1111111..2222222 100644",
			"--- a/" + path, "+++ b/" + path, fmt.Sprintf("@@ -20,%d +21,%d @@", len(body), len(body))}
		return append(out, body...)
	}
	diffs := map[string][]string{
		"clean": gitFile("a.txt", " one", "-two", "+three"),
		"added run": append(gitFile("a.txt", " one", "+two"),
			gitFile("b.txt", " one", "+value "+run)...),
		"removed run": gitFile("b.txt", " one", "-value "+run, "+value gone"),
		"context run": gitFile("b.txt", " value "+run, "-two", "+three"),
		"decrypted k8s secret": gitFile("deploy/s.yaml", "+apiVersion: v1", "+kind: Secret",
			"+metadata:", "+  name: x", "+data:", "+  password: aHVudGVyMg=="),
		"workpad marker": gitFile("notes.md", "+"+deskkit.WorkpadMarker, "+value "+run),
	}
	for name, lines := range diffs {
		t.Run(name, func(t *testing.T) {
			diff := strings.Join(lines, "\n") + "\n"
			surface := "branch diff vs origin/main"
			o := deskkit.ScanOverride{Tool: "deskpr", Verb: "create", Repo: "o/r", Surface: surface}

			legacyText := legacyDiffStrip(diff)
			o.Content = []byte(legacyText)
			legacy := deskkit.HandleScanRefusal(o, deskkit.ScanSurfaceSecrets(surface, []byte(legacyText)))

			stripped, refs := deskkit.DiffLineIndex(diff)
			if stripped != legacyText {
				t.Fatalf("scanned bytes changed:\nlegacy: %q\nindex:  %q", legacyText, stripped)
			}
			o.Content = []byte(stripped)
			located := locateDiffFinding(deskkit.HandleScanRefusal(o, deskkit.ScanSurfaceSecrets(surface, []byte(stripped))), refs)

			if (legacy == nil) != (located == nil) {
				t.Fatalf("verdict changed: legacy=%v located=%v", legacy, located)
			}
			if wantRefused := name != "clean"; (legacy != nil) != wantRefused {
				t.Fatalf("fixture %q: refused = %v, want %v (%v)", name, legacy != nil, wantRefused, legacy)
			}
			if legacy == nil {
				return
			}
			if deskkit.IsRefused(legacy) != deskkit.IsRefused(located) {
				t.Fatalf("refused-ness changed: legacy=%v located=%v", legacy, located)
			}
			var le, ne *deskkit.DeskError
			if !errors.As(legacy, &le) || !errors.As(located, &ne) || le.Code != ne.Code {
				t.Fatalf("exit code changed: legacy=%v located=%v", legacy, located)
			}
			var lf, nf *deskkit.ScanFinding
			if errors.As(legacy, &lf) != errors.As(located, &nf) {
				t.Fatalf("finding presence changed")
			}
			if lf != nil && (lf.Rule != nf.Rule || lf.Length != nf.Length || lf.Shape != nf.Shape) {
				t.Fatalf("finding changed beyond its location: legacy=%+v located=%+v", lf, nf)
			}
			// The message is the legacy refusal with only the location inserted before the
			// override hint, so the override path (hint present or absent) is unchanged.
			hint := deskkit.OverrideHint(o.Form)
			if strings.HasSuffix(legacy.Error(), hint) != strings.HasSuffix(located.Error(), hint) {
				t.Fatalf("override path changed:\nlegacy:  %s\nlocated: %s", legacy, located)
			}
			lp := strings.TrimSuffix(legacy.Error(), hint)
			if !strings.HasPrefix(located.Error(), lp+" — at ") {
				t.Fatalf("refusal text changed beyond the location:\nlegacy:  %s\nlocated: %s", legacy, located)
			}
		})
	}
}

// legacyDiffStrip is a frozen, verbatim copy of the strip deskpr ran before the #3196 fix
// moved it into deskkit.DiffLineIndex: the "without the index" half of the verdict check.
func legacyDiffStrip(diff string) string {
	hunk := regexp.MustCompile(`^(@@ -[0-9]+(?:,[0-9]+)? \+[0-9]+(?:,[0-9]+)? @@)`)
	lines := strings.Split(diff, "\n")
	kept := make([]string, 0, len(lines))
	inHeader, isGitOutput := false, false
	for _, ln := range lines {
		if strings.HasPrefix(ln, "diff --git ") {
			inHeader, isGitOutput = true, true
			continue
		}
		if inHeader && strings.HasPrefix(ln, "@@ ") {
			inHeader = false
		}
		if inHeader {
			continue
		}
		if isGitOutput {
			if m := hunk.FindStringSubmatch(ln); m != nil {
				ln = m[1]
			} else if len(ln) > 0 && (ln[0] == '+' || ln[0] == '-' || ln[0] == ' ') {
				ln = ln[1:]
			}
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}
