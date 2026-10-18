package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// regression: #3342
func TestProtectedClosureRows(t *testing.T) {
	for _, name := range []string{"squash", "merge", "expect-edited", "command-edited", "divergent-original", "row-removed", "row-renumbered", "class-changed", "duplicate-id"} {
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
			if name == "row-removed" {
				currentVerify = strings.Replace(currentVerify, "| 1 | `true` | exit 0 |\n", "", 1)
			}
			if name == "row-renumbered" {
				currentVerify = strings.Replace(currentVerify, "| 1 |", "| 8 |", 1)
			}
			if name == "class-changed" {
				currentVerify = "| # | Class | Command | Expect |\n|---|---|---|---|\n| 1 | gate:human | `true` | exit 0 |\n| 2 | check | `false` | exit 1 |\n| 3 | check | `printf supplemental` | exit 0 |\n"
			}
			if name == "duplicate-id" {
				currentVerify += "| 1 | `echo duplicate` | exit 0 |\n"
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
			// Same delivered table on a real sibling: table equality alone must
			// not authenticate an unlanded delivery snapshot.
			vfGit(t, root, vfMailWorker, "checkout", "-q", "-b", "offbranch", base)
			if err := os.WriteFile(path, []byte(delivered), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "offbranch.txt"), []byte("sibling\n"), 0644); err != nil {
				t.Fatal(err)
			}
			vfCommit(t, root, vfMailWorker)
			sibling := vfRev(t, root, "HEAD")
			vfGit(t, root, vfMailWorker, "checkout", "-q", "--detach", delivery)
			marker := "Verification-Table: delivery=" + delivery + " base=" + base + " original=" + original + "\n\n"
			supplemental := "| 3 | `printf supplemental` | author-added; execution exit=0 sandbox=unshare; no acceptance credit | sha256:bbbbbbbbbbbb | 2026-08-13 | human:alex @ " + delivery + " |\n"
			evidence := marker + witnessTableFor(vfRow1, row2) + "\n" + supplemental
			if name == "row-renumbered" {
				evidence += strings.Replace(strings.Replace(supplemental, "| 3 |", "| 8 |", 1), "`printf supplemental`", "`true`", 1)
			}
			cases := []struct {
				name, verify, evidence string
				accepted               bool
			}{
				{"originals-pass", currentVerify, evidence, name == "squash" || name == "merge" || name == "expect-edited" || name == "command-edited"},
				{"missing-provenance", vfVerify + added, strings.TrimPrefix(evidence, marker), false},
				{"quoted-provenance", vfVerify + added, "> " + evidence, false},
				{"fenced-provenance", vfVerify + added, "```\n" + marker + "```\n" + strings.TrimPrefix(evidence, marker), false},
				{"duplicate-provenance", vfVerify + added, marker + evidence, false},
				{"code-spliced-marker", currentVerify, strings.Replace(evidence, "Verification-Table:", "Verification-Table: `example`", 1), false},
				{"tampered-base", vfVerify + added, strings.Replace(evidence, "base="+base, "base="+delivery, 1), false},
				{"missing-object", vfVerify + added, strings.Replace(evidence, "delivery="+delivery, "delivery="+strings.Repeat("f", 40), 1), false},
				{"offbranch-delivery", currentVerify, strings.Replace(evidence, "delivery="+delivery, "delivery="+sibling, 1), false},
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
					// Fixture execution witnesses name this fixture's actual code
					// revision, so coverage also checks real immutable row history.
					body = strings.ReplaceAll(body, " @ "+vfSHA, " @ "+delivery)
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
					switch c.name {
					case "originals-pass", "missing-provenance", "tampered-base", "missing-object", "original-fails", "original-wrong-exit", "supplement-pass-credit":
						coverage := evaluateCoverage(root, streams, coverageOptions{Revision: delivery})["vf/01"]
						if coverage.Released != c.accepted {
							t.Fatalf("coverage released=%v accepted=%v: %+v", coverage.Released, c.accepted, coverage.Claims)
						}
						if c.accepted && c.name == "originals-pass" {
							bf, ok, err := parseBriefFile(path)
							if err != nil || !ok {
								t.Fatalf("parse: %v", err)
							}
							obs := observeBrief(root, path, "vf/01", bf)
							if len(obs.VerifyRows) != 2 || obs.AcceptanceRevision != original || obs.AcceptanceProblem != "" {
								t.Fatalf("observed original contract: %+v", obs)
							}
							for _, rev := range []string{"unfixed-ref", strings.Repeat("f", 40)} {
								bad := obs
								bad.AcceptanceRevision = rev
								got := evaluation.EvaluateCoverage(evaluation.CoverageInput{Revision: delivery, Briefs: []evaluation.BriefObservation{bad}}, newCoverageRepo(root))["vf/01"]
								if got.Released || got.Claims[0].Result != covCouldNotCheck {
									t.Fatalf("invalid definition %s released: %+v", rev, got)
								}
							}
							if name == "expect-edited" {
								for _, rev := range []string{"", delivery} {
									changed := obs
									changed.AcceptanceRevision = rev
									got := evaluation.EvaluateCoverage(evaluation.CoverageInput{Revision: delivery, Briefs: []evaluation.BriefObservation{changed}}, newCoverageRepo(root))["vf/01"]
									if got.Released || got.Claims[1].Result != covError {
										t.Fatalf("historical Expect guard %s: %+v", rev, got)
									}
								}
							}
						}
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

// regression: #3342
func TestProtectedRerun(t *testing.T) {
	o := vfDefaults()
	o.status = "verified"
	o.verified = "2026-08-13 human:alex"
	o.evidence = ""
	root, _ := vfFixture(t, o)
	path := vfBrief(root)
	base := vfRev(t, root, "HEAD")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	verify := vfVerify + "| 3 | `printf supplemental` | exit 0 |\n"
	bare := strings.Replace(string(raw), vfVerify, verify, 1)
	if err := os.WriteFile(path, []byte(bare), 0644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailWorker)
	delivery := vfRev(t, root, "HEAD")
	marker := "Verification-Table: delivery=" + delivery + " base=" + base + " original=" + base + "\n\n"
	row1 := strings.ReplaceAll(vfRow1, vfSHA, delivery)
	table := strings.ReplaceAll(witnessTableFor(vfRow1, vfRow2), vfSHA, delivery) + "\n| 3 | `printf supplemental` | author-added; execution exit=0; no acceptance credit | sha256:bbbbbbbbbbbb | 2026-08-13 | " + vfRunner + " @ " + delivery + " |\n"
	run := marker + table + "\n**VERIFY: PASS**\n"
	cases := []struct {
		name, evidence string
		pass           bool
	}{
		{"single", run, true},
		{"two-runs", run + "\n" + run, true},
		{"old-failure-new-pass", strings.Replace(run, "**VERIFY: PASS**", "**VERIFY: FAIL**", 1) + "\n" + run, true},
		{"latest-failed", run + "\n" + strings.Replace(run, "**VERIFY: PASS**", "**VERIFY: FAIL**", 1), false},
		{"latest-failed-original", run + "\n" + strings.Replace(run, row1, strings.Replace(row1, "pass", "fail", 1), 1), false},
		{"latest-missing-row", run + "\n" + strings.Replace(run, row1, "", 1), false},
		{"duplicate-current-marker", run + "\n" + marker + run, false},
		{"latest-unfinished", run + "\n" + marker + table, false},
		{"latest-marker-only", run + "\n" + marker, false},
		{"latest-loose-pass", run + "\n" + strings.Replace(run, "**VERIFY: PASS**", "VERIFY: PASS", 1), false},
		{"latest-missing-marker", run + "\n" + strings.TrimPrefix(run, marker), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := strings.Replace(bare, "## Evidence\n", "## Evidence\n"+c.evidence, 1)
			if err := os.WriteFile(path, []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			resetReadMemo()
			info := &briefInfo{File: filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator))), Row: &briefInfoRow{Status: "verified", Verified: "2026-08-13 human:alex"}}
			if err := checkBriefInfoVerified(root, info); (err == nil) != c.pass {
				t.Fatalf("closure pass=%v: %v", c.pass, err)
			}
			code, out, diag := captureVerifyclosure(t, []string{"--root", root, "--brief", "vf/01"})
			if (code == 0) != c.pass {
				t.Fatalf("closure CLI=%d %s %s", code, out, diag)
			}
			result, diag := captureVerifyrun(t, []string{"--check", "--brief", path})
			if (result.code == 0) != c.pass {
				t.Fatalf("audit=%d %s", result.code, diag)
			}
			streams, _, err := loadStreams(root)
			if err != nil {
				t.Fatal(err)
			}
			if problems := protectedClosureProblems(streams); (len(problems) == 0) != c.pass {
				t.Fatalf("lint pass=%v: %v", c.pass, problems)
			}
			if got := evaluateCoverage(root, streams, coverageOptions{Revision: delivery})["vf/01"]; got.Released != c.pass {
				t.Fatalf("coverage pass=%v: %+v", c.pass, got)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != body {
				t.Fatal("audit changed retained Evidence")
			}
		})
	}
}

// regression: #3342
func TestProtectedMainDrift(t *testing.T) {
	o := vfDefaults()
	o.status = "verified"
	o.verified = "2026-08-13 human:alex"
	o.evidence = ""
	root, _ := vfFixture(t, o)
	path := vfBrief(root)
	base := vfRev(t, root, "HEAD")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	vfGit(t, root, vfMailWorker, "checkout", "-q", "-b", "implementation")
	if err := os.WriteFile(filepath.Join(root, "implementation.txt"), []byte("change\n"), 0644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailWorker)
	vfGit(t, root, vfMailHuman, "checkout", "-q", "main")
	verify := vfVerify + "| 3 | `printf supplemental` | exit 0 |\n"
	bare := strings.Replace(string(raw), vfVerify, verify, 1)
	if err := os.WriteFile(path, []byte(bare), 0644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailHuman)
	vfGit(t, root, vfMailHuman, "merge", "--no-ff", "-q", "-m", "delivery", "implementation")
	delivery := vfRev(t, root, "HEAD")
	evidence := "Verification-Table: delivery=" + delivery + " base=" + base + " original=" + base + "\n\n" + strings.ReplaceAll(witnessTableFor(vfRow1, vfRow2), vfSHA, delivery) + "\n| 3 | `printf supplemental` | author-added; execution exit=0; no acceptance credit | sha256:bbbbbbbbbbbb | 2026-08-13 | " + vfRunner + " @ " + delivery + " |\n"
	if err := os.WriteFile(path, []byte(strings.Replace(bare, "## Evidence\n", "## Evidence\n"+evidence, 1)), 0644); err != nil {
		t.Fatal(err)
	}
	resetReadMemo()
	if _, refusal := protectedAcceptance(verify, evidence, path); refusal == "" {
		t.Fatal("main acceptance changed after divergence; require independent re-derivation")
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := evaluateCoverage(root, streams, coverageOptions{Revision: delivery})["vf/01"]; got.Released {
		t.Fatal("changed original acceptance was credited")
	}
}
