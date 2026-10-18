package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #3342
func TestProtectedClosureRows(t *testing.T) {
	for _, name := range []string{"squash", "merge", "expect-edited", "command-edited", "divergent-original"} {
		t.Run(name, func(t *testing.T) {
			o := vfDefaults()
			o.status, o.verified, o.evidence = "verified", "2026-08-13 human:alex", ""
			root, _ := vfFixture(t, o)
			path := vfBrief(root)
			base := vfRev(t, root, "HEAD")
			original := base
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			added := "| 3 | `printf supplemental` | exit 0 |\n"
			baselineVerify := vfVerify
			row2 := vfRow2
			if name == "divergent-original" {
				baselineVerify = strings.Replace(vfVerify, "exit 1", "exit 9", 1)
				raw = []byte(strings.Replace(string(raw), vfVerify, baselineVerify, 1))
				if err := os.WriteFile(path, raw, 0644); err != nil {
					t.Fatal(err)
				}
				vfCommit(t, root, vfMailHuman)
				base = vfRev(t, root, "HEAD")
				row2 = strings.Replace(vfRow2, "exit=1", "exit=9", 1)
			}
			currentVerify := baselineVerify + added
			if name == "expect-edited" {
				currentVerify = strings.Replace(currentVerify, "exit 1", "exit 0", 1)
			}
			if name == "command-edited" {
				currentVerify = strings.Replace(currentVerify, "`true`", "`echo changed`", 1)
			}
			delivered := strings.Replace(string(raw), baselineVerify, currentVerify, 1)
			if name == "merge" {
				vfGit(t, root, vfMailWorker, "checkout", "-q", "-b", "delivery")
			}
			if err := os.WriteFile(path, []byte(delivered), 0644); err != nil {
				t.Fatal(err)
			}
			vfCommit(t, root, vfMailWorker)
			if name == "merge" {
				vfGit(t, root, vfMailHuman, "checkout", "-q", "main")
				vfGit(t, root, vfMailHuman, "merge", "--no-ff", "-q", "-m", "delivery", "delivery")
			}
			delivery := vfRev(t, root, "HEAD")
			marker := "Verification-Table: delivery=" + delivery + " base=" + base + " original=" + original + "\n\n"
			supplemental := "| 3 | `printf supplemental` | author-added; execution exit=0 sandbox=unshare; no acceptance credit | sha256:bbbbbbbbbbbb | 2026-08-13 | human:alex @ " + delivery + " |\n"
			evidence := marker + witnessTableFor(vfRow1, row2) + "\n" + supplemental
			cases := []struct {
				name, verify, evidence string
				accepted               bool
			}{
				{"originals-pass", currentVerify, evidence, name != "divergent-original"},
				{"missing-provenance", vfVerify + added, strings.TrimPrefix(evidence, marker), false},
				{"quoted-provenance", vfVerify + added, "> " + evidence, false},
				{"fenced-provenance", vfVerify + added, "```\n" + marker + "```\n" + strings.TrimPrefix(evidence, marker), false},
				{"duplicate-provenance", vfVerify + added, marker + evidence, false},
				{"code-spliced-marker", currentVerify, strings.Replace(evidence, "Verification-Table:", "Verification-Table: `example`", 1), false},
				{"tampered-base", vfVerify + added, strings.Replace(evidence, "base="+base, "base="+delivery, 1), false},
				{"missing-object", vfVerify + added, strings.Replace(evidence, "delivery="+delivery, "delivery="+strings.Repeat("f", 40), 1), false},
				{"tampered-original", currentVerify, strings.Replace(evidence, "original="+base, "original="+delivery, 1), false},
				{"missing-original", currentVerify, strings.Replace(evidence, "original="+base, "original="+strings.Repeat("f", 40), 1), false},
				{"short-object", currentVerify, strings.Replace(evidence, "delivery="+delivery, "delivery="+delivery[:12], 1), false},
				{"original-fails", vfVerify + added, strings.Replace(evidence, "pass exit=0", "fail exit=1", 1), false},
				{"original-unwitnessed", vfVerify + added, strings.Replace(evidence, vfRow1, "", 1), false},
				{"original-uncredited", vfVerify + added, strings.Replace(evidence, "pass exit=0", "author-added exit=0", 1), false},
				{"original-label-pass", currentVerify, strings.Replace(evidence, "pass exit=0", "author-added; pass exit=0", 1), false},
				{"original-wrong-exit", currentVerify, strings.Replace(evidence, "pass exit=1", "pass exit=0", 1), false},
				{"original-removed", strings.Replace(vfVerify+added, "| 1 | `true` | exit 0 |\n", "", 1), evidence, false},
				{"original-renumbered", strings.Replace(vfVerify+added, "| 1 |", "| 8 |", 1), evidence, false},
				{"original-edited", strings.Replace(vfVerify+added, "`true`", "`echo weak`", 1), evidence, false},
				{"expect-edited-after-delivery", strings.Replace(currentVerify, "exit 0", "exit 7", 1), evidence, false},
				{"extra-later-row", vfVerify + added + "| 4 | `true` | exit 0 |\n", evidence, false},
				{"supplement-pass-credit", vfVerify + added, strings.Replace(evidence, "author-added; execution exit=0 sandbox=unshare; no acceptance credit", "pass exit=0", 1), false},
				{"latest-supplement-pass", vfVerify + added, evidence + strings.Replace(supplemental, "author-added; execution exit=0 sandbox=unshare; no acceptance credit", "pass exit=0", 1), false},
				{"supplement-label-pass", currentVerify, strings.Replace(evidence, "author-added; execution", "author-added; pass", 1), false},
				{"new-command-only", currentVerify, strings.Replace(evidence, vfRow1, strings.Replace(vfRow1, "`true`", "`echo changed`", 1), 1), false},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					body := strings.Replace(delivered, currentVerify, c.verify, 1)
					body = strings.Replace(body, "## Evidence\n", "## Evidence\n"+c.evidence, 1)
					if err := os.WriteFile(path, []byte(body), 0644); err != nil {
						t.Fatal(err)
					}
					resetReadMemo()
					info := &briefInfo{File: filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator))), Row: &briefInfoRow{Status: "verified", Verified: "2026-08-13 human:alex"}}
					err := checkBriefInfoVerified(root, info)
					if (err == nil) != c.accepted {
						t.Fatalf("closure error=%v accepted=%v", err, c.accepted)
					}
					code, out, diag := captureVerifyclosure(t, []string{"--root", root, "--brief", "vf/01"})
					if (code == 0) != c.accepted {
						t.Fatalf("verifyclosure code=%d out=%s err=%s", code, out, diag)
					}
					result, diag := captureVerifyrun(t, []string{"--check", "--brief", path})
					if (result.code == 0) != c.accepted {
						t.Fatalf("verifyrun code=%d err=%s", result.code, diag)
					}
					streams, _, err := loadStreams(root)
					if err != nil {
						t.Fatal(err)
					}
					problems := protectedClosureProblems(streams)
					if c.accepted && len(problems) != 0 {
						t.Fatalf("lint disagreements: %v", problems)
					}
					if c.accepted {
						findings, refusal := closureWitnesses(c.verify, c.evidence, path)
						if refusal != "" || len(findings) != 3 || findings[0].State != statePass || findings[1].State != statePass || findings[2].State != stateAuthorAdded {
							t.Fatalf("acceptance credit: %v %s", findings, refusal)
						}
					}
				})
			}
		})
	}
}
