package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Physical ranges exclude only the synthetic element after a terminal newline.
// The second heading plants the same boundary in a separate section consumer.
// regression: #3659
func TestSectionRangeEOF(t *testing.T) {
	for _, heading := range []string{"## Evidence", "## Receipt"} {
		for _, tc := range []struct {
			name, body string
			end        int
			ok         bool
		}{
			{"empty", "", 0, false},
			{"unterminated", "row", 2, true},
			{"terminated", "row\n", 2, true},
			{"blank_tail", "row\n\n", 3, true},
			{"comment_tail", "row\n<!-- note -->\n", 3, true},
			{"next_section", "row\n\n## Next\n", 3, true},
		} {
			t.Run(heading+"/"+tc.name, func(t *testing.T) {
				start, end, ok := sectionLineRange(heading+"\n"+tc.body,
					func(s string) bool { return s == heading })
				if ok != tc.ok || (ok && (start != 2 || end != tc.end)) {
					t.Fatalf("range = (%d, %d, %v), want (2, %d, %v)", start, end, ok, tc.end, tc.ok)
				}
			})
		}
	}
}

// Use real Git blame, including a genuinely empty final line and CRLF input.
// All changes and verifyflip writes stay inside the temporary Git fixtures.
// regression: #3659
func TestVflipEvidenceEOF(t *testing.T) {
	for _, suffix := range []string{"", "\n", "\n\n", "\n<!-- note\ncontinued -->\n"} {
		for _, crlf := range []bool{false, true} {
			t.Run(strings.ReplaceAll(suffix, "\n", "LF")+map[bool]string{false: "/LF", true: "/CRLF"}[crlf], func(t *testing.T) {
				root, _ := vfFixture(t, vfDefaults())
				vfEdit(t, root, "\n\n## Review\nGate: model.\n", suffix)
				if crlf {
					raw, err := os.ReadFile(vfBrief(root))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(vfBrief(root), []byte(strings.ReplaceAll(string(raw), "\n", "\r\n")), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				vfCommit(t, root, vfMailVerifier)
				if code, out, errOut := runVF(t, root, "--dry-run"); code != verifyflipExitOK || !strings.Contains(out, "verified-stamp: "+vfStamp) {
					t.Fatalf("exit = %d, out=%q err=%q", code, out, errOut)
				}
			})
		}
	}
}

// The actual final content line still needs its own authorized author.
// regression: #3659
func TestVflipEOFUntrusted(t *testing.T) {
	root, readme := vfFixture(t, vfDefaults())
	vfEdit(t, root, "\n\n## Review\nGate: model.\n", "\n")
	vfCommit(t, root, vfMailVerifier)
	vfEdit(t, root, vfPass+"\n", vfPass+"\nAn additional observation.\n")
	vfCommit(t, root, vfMailWorker)
	vfExpect(t, root, readme, verifyflipExitRefused, "a content line of the latest PASS run or after it")
}

// The production fold must consume the same EOF evidence as verifyflip, using
// its ordinary revision, coverage, merged-PR and reviewer bindings.
// regression: #3659
func TestFoldEvidenceEOF(t *testing.T) {
	root := writeFoldFixture(t,
		[]foldFixtureBrief{{num: "01", id: "fx/01", gate: "model", ev: foldPassEv}},
		map[string][3]string{"01": {"implemented", "—", "—"}})
	path := filepath.Join(root, "docs", "streams", "fx", "brief-01-fixture.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body, _, ok := strings.Cut(string(raw), "\n## Review\n")
	if !ok {
		t.Fatal("fixture has no Review boundary")
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	foldGit(t, root, foldMailVerifier, "commit", "-q", "-am", "evidence ends at EOF")
	foldServer(t, foldStdPulls()[:1], foldIssues())
	foldFlipSeam(t, foldApproved())
	wantCell(t, foldCells(t, root), "fx/01", "done", "authorized EOF witness with bound approval")
}
