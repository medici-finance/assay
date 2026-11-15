package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// rotatingcallers_test.go — the #6068 defect class: a shipped skill or reference that tells a
// caller to run `desktoken <role>` with neither `--no-rotate` nor an explicit forge. On a
// GitLab-resolved repo that form ROTATES the role's credential, so a caller that only needed
// the credential's location spends a rotation and, if it then discards the printed file, locks
// the role out. Explicit forms pass: `--no-rotate` (read-only), `--forge github` (never
// rotates), `--forge gitlab` (the deliberate rotation) and `--fresh` (a GitHub re-mint).

// rotatingCallerRe matches `desktoken [flags] <role>` plus the rest of its code span, allowing
// one wrapped continuation line, so a flag that follows the role on the next line still counts.
var rotatingCallerRe = regexp.MustCompile("desktoken((?:[ \\t]+--?[a-z][a-z-]*)*)[ \\t]+" +
	`(?:<role>|"\$role"|\$role|"\$ROLE"|\$ROLE|reviewer|verifier|worker|desk|issue-loop|intake-loop|cell-issues)` +
	"(?:[`)]|$|[ \\t\\n\"][^`)\\n]*(?:\\n[ \\t]*[^`)\\n]*)?)")

var explicitForms = []string{"--no-rotate", "--forge github", "--forge gitlab", "--fresh"}

// rotatingCallers returns "line: span" for each rotating-form call in src.
func rotatingCallers(src string) []string {
	var hits []string
	for _, loc := range rotatingCallerRe.FindAllStringIndex(src, -1) {
		span := src[loc[0]:loc[1]]
		explicit := false
		for _, f := range explicitForms {
			explicit = explicit || strings.Contains(span, f)
		}
		if !explicit {
			line := strings.Count(src[:loc[0]], "\n") + 1
			hits = append(hits, strconv.Itoa(line)+": "+strings.SplitN(span, "\n", 2)[0])
		}
	}
	return hits
}

// regression: #6068
func TestShippedCallersNoRotate(t *testing.T) {
	planted, err := os.ReadFile(filepath.Join("testdata", "rotatingcallers", "planted.md"))
	if err != nil {
		t.Fatalf("read the planted control: %v", err)
	}
	if got := rotatingCallers(string(planted)); len(got) != 3 {
		t.Fatalf("positive control: the matcher flagged %d planted call(s), want 3 — a broken guard "+
			"must fail, not report clean:\n  %s", len(got), strings.Join(got, "\n  "))
	}

	scanned := 0
	for _, dir := range []string{"plugins/assay/skills", "plugins/assay/references"} {
		root := filepath.Join(adopterDocsRepoRoot, filepath.FromSlash(dir))
		if _, err := os.Stat(root); os.IsNotExist(err) {
			t.Skipf("could-not-check: %s is not in this checkout", dir)
		}
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
			if werr != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
				return werr
			}
			raw, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			scanned++
			for _, h := range rotatingCallers(string(raw)) {
				t.Errorf("%s:%s — use `desktoken --no-rotate <role> --repo <slug>` for a read, or name "+
					"the forge explicitly (#6068)", filepath.ToSlash(p), h)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v — could-not-check, not a pass", dir, err)
		}
	}
	if scanned == 0 {
		t.Fatal("scanned 0 markdown files — could-not-check, not a pass")
	}
}
