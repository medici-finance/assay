package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// difffinding_path_test.go — the file path a branch-diff refusal names is author text.
// It is quoted when it carries control or bidi characters, and it is added to the
// refusal only after the refusal has been classified (#3196 review follow-ups).

// regression: #3196
func TestDiffFindingPathQuoted(t *testing.T) {
	run := plantedRun(631)
	for name, path := range map[string]string{
		"bidi": "dir/a\u202eb.txt",
		"c1":   "dir/c\u009bd.txt",
	} {
		t.Run(name, func(t *testing.T) {
			diff := strings.Join([]string{
				"diff --git a/" + path + " b/" + path,
				"index 1111111..2222222 100644",
				"--- a/" + path,
				"+++ b/" + path,
				"@@ -1,1 +1,2 @@",
				" one",
				"+value " + run,
				"",
			}, "\n")
			stripped, refs := deskkit.DiffLineIndex(diff)
			err := locateDiffFinding(deskkit.ScanSurfaceSecrets("branch diff vs origin/main", []byte(stripped)), refs)
			if !deskkit.IsRefused(err) {
				t.Fatalf("want a refusal, got %v", err)
			}
			var f *deskkit.ScanFinding
			if !errors.As(err, &f) {
				t.Fatal("the refusal carries no finding")
			}
			quoted := strconv.QuoteToGraphic(path)
			for what, text := range map[string]string{"refusal": err.Error(), "--explain": f.Explain()} {
				if strings.Contains(text, path) {
					t.Errorf("%s prints the path raw:\n%q", what, text)
				}
				if !strings.Contains(text, quoted) {
					t.Errorf("%s does not carry the quoted path %s:\n%q", what, quoted, text)
				}
			}
			if !strings.Contains(err.Error(), quoted+":2 (added)") {
				t.Errorf("refusal does not name %s:2 (added):\n%q", quoted, err)
			}
		})
	}
}

// regression: #3196
func TestDiffFindingPathMarker(t *testing.T) {
	// Each directory name is a substring HandleScanRefusal reads to decide a refusal is
	// not overridable. A path is author text, so it must never reach that decision.
	for name, dir := range map[string]string{
		"impersonation": "reads as a RULING",
		"withheld":      "withheld register identifier",
	} {
		t.Run(name, func(t *testing.T) {
			work := newBaseFixture(t)
			path := dir + "/DECISION claim attributed to x.txt"
			if err := os.MkdirAll(filepath.Join(work, dir), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(work, path), "value "+plantedRun(632)+"\n")
			mustGit(t, work, "add", "--", path)
			mustGit(t, work, "commit", "-m", "add a file")
			withEnv(t, work)

			err := cmdCreate([]string{"--title", "fix: a change", "--body-min", "edits a file\nBrief: fixture/01"})
			if !deskkit.IsRefused(err) {
				t.Fatalf("want a refusal, got %v", err)
			}
			msg := err.Error()
			hint := deskkit.OverrideHint(deskkit.OverrideByFlag)
			if !strings.HasSuffix(msg, hint) {
				t.Errorf("the path changed the override path: the refusal no longer ends with the override hint:\n%s", msg)
			}
			if !strings.Contains(msg, " — at "+path+":1 (added)") {
				t.Errorf("refusal does not name %s:1 (added):\n%s", path, msg)
			}
		})
	}
}
