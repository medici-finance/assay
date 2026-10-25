package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runnerform_test.go — #4299: one latest-PASS run may carry the Runner cell's
// sha in two forms (a container bracket's 12-hex tree token and a host
// transition run's 40-hex HEAD), and a host run taken after an Evidence-only
// edit must not read +dirty. Every forged or genuinely different cell still
// refuses.

// rfLong is a 40-hex sha whose 12-hex prefix is vfSHA.
const rfLong = vfSHA + "0123456789abcdef0123456789ab"

func rfRow(id, cell string) string { return rfRowAt(id, "2026-08-13", cell) }

func rfRowAt(id, date, cell string) string {
	return "| " + id + " | `true` | pass exit=0 | sha256:6f1a0b3c9d22 | " + date + " | " + cell + " |"
}

// regression: #4299
func TestFlipShaFormAgree(t *testing.T) {
	at := func(sha string) string { return vfRunner + " @ " + sha }
	cases := []struct {
		name  string
		cells []string
		ok    bool
	}{
		{"prefix", []string{at(vfSHA), at(rfLong)}, true},
		{"prefix-reversed", []string{at(rfLong), at(vfSHA)}, true},
		{"equal-short", []string{at(vfSHA), at(vfSHA)}, true},
		{"middle-prefix", []string{at(vfSHA), at(rfLong), at(rfLong[:20])}, true},
		{"other-commit", []string{at("c988d175ab12"), at(rfLong)}, false},
		{"too-short", []string{at(vfSHA[:11]), at(rfLong)}, false},
		{"seven-hex", []string{at(vfSHA[:7]), at(rfLong)}, false},
		{"other-login", []string{"other-runner[bot] @ " + vfSHA, at(rfLong)}, false},
		{"dirty-short", []string{at(vfSHA + "+dirty"), at(rfLong)}, false},
		{"qualifier", []string{at(vfSHA + " (other)"), at(rfLong)}, false},
		{"upper-prefix", []string{at(strings.ToUpper(vfSHA)), at(rfLong)}, false},
		{"diverging-long", []string{at(vfSHA), at(rfLong), at(vfSHA + "ffff")}, false},
		{"two-long", []string{at(rfLong), at(vfSHA + strings.Repeat("f", 28))}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var rows []string
			for i, cell := range c.cells {
				rows = append(rows, rfRow(string(rune('1'+i)), cell))
			}
			p, err := flipLatestPass(witnessTableFor(rows...) + "\n\n" + vfPass)
			if (err == nil) != c.ok {
				t.Fatalf("accepted=%v want %v: %v", err == nil, c.ok, err)
			}
			if !c.ok {
				return
			}
			// The run reads as ONE sha: the longest form every row agrees with.
			want := vfSHA
			for _, cell := range c.cells {
				if s := strings.TrimPrefix(cell, vfRunner+" @ "); len(s) > len(want) {
					want = s
				}
			}
			if p.SHA != want || p.Stamp != "2026-08-13 "+vfRunner+" @ "+want {
				t.Fatalf("sha %q stamp %q, want %q", p.SHA, p.Stamp, want)
			}
		})
	}
}

// regression: #4299
func TestTransitionMixedForms(t *testing.T) {
	cases := []string{"mixed", "forged-prefix", "short-prefix", "all-short", "other-runner"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			f := transitionFixtureAt(t)
			f.evidence(t, func(ev string) string {
				// Rows 1 and 2 stand for container-bracket witnesses; row 3 is
				// the host transition run's author-added row at the full HEAD.
				from := vfRunner + " @ " + f.amendment
				switch name {
				case "mixed":
					return strings.Replace(ev, from, vfRunner+" @ "+f.amendment[:12], 2)
				case "forged-prefix":
					return strings.Replace(ev, from, vfRunner+" @ "+f.delivery[:12], 2)
				case "short-prefix":
					return strings.Replace(ev, from, vfRunner+" @ "+f.amendment[:11], 2)
				case "all-short":
					return strings.ReplaceAll(ev, from, vfRunner+" @ "+f.amendment[:12])
				case "other-runner":
					return strings.Replace(ev, from, "other-runner[bot] @ "+f.amendment[:12], 2)
				}
				return ev
			})
			want := name == "mixed"
			verify, evidence, e := briefSections(f.path)
			if e != nil {
				t.Fatal(e)
			}
			findings, why := closureWitnesses(verify, evidence, f.path)
			pass := why == "" && len(findings) == 3
			for _, v := range findings {
				pass = pass && closureSatisfied(v)
			}
			if pass != want {
				t.Fatalf("closure accepted=%v want=%v: %s %+v", pass, want, why, findings)
			}
			res, diag := captureVerifyrun(t, []string{"--check", "--brief", f.path, "--acceptance-forge"})
			if (res.code == 0) != want {
				t.Fatalf("verifyrun --check: %d %s", res.code, diag)
			}
			code, out, diag := captureVerifyclosure(t, []string{"--root", f.root, "--brief", "vf/01", "--acceptance-forge"})
			if (code == 0) != want {
				t.Fatalf("verifyclosure: %d %s %s", code, out, diag)
			}
		})
	}
}

// regression: #4299
func TestEvidenceOnlyTreeClean(t *testing.T) {
	insert := func(t *testing.T, f rowsFixture, text string) {
		t.Helper()
		raw := getFile(t, f.brief)
		putFile(t, f.brief, strings.Replace(raw, "## Evidence\n", "## Evidence\n"+text, 1))
	}
	cases := []struct {
		name  string
		edit  func(t *testing.T, f rowsFixture)
		dirty bool
	}{
		{"evidence-only", func(t *testing.T, f rowsFixture) {
			insert(t, f, "\nA container witness was inserted here.\n")
		}, false},
		{"evidence-staged", func(t *testing.T, f rowsFixture) {
			insert(t, f, "\nA container witness was inserted here.\n")
			runGit(t, f.root, "add", f.brief)
		}, false},
		{"untracked-file", func(t *testing.T, f rowsFixture) {
			insert(t, f, "\nA container witness was inserted here.\n")
			putFile(t, filepath.Join(f.root, "extra.txt"), "x\n")
		}, true},
		{"outside-evidence", func(t *testing.T, f rowsFixture) {
			raw := getFile(t, f.brief)
			putFile(t, f.brief, strings.Replace(raw, "# Fixture\n", "# Fixture edited\n", 1))
		}, true},
		{"review-edit", func(t *testing.T, f rowsFixture) {
			raw := getFile(t, f.brief)
			putFile(t, f.brief, strings.Replace(raw, "\nnone\n", "\nchanged\n", 1))
		}, true},
		{"new-heading", func(t *testing.T, f rowsFixture) {
			insert(t, f, "\n## Smuggled\n\nnot Evidence\n")
		}, true},
		{"crlf", func(t *testing.T, f rowsFixture) {
			insert(t, f, "\nA container witness was inserted here.\r\n")
		}, true},
		{"other-brief", func(t *testing.T, f rowsFixture) {
			other := filepath.Join(f.root, "docs", "other.md")
			putFile(t, other, "# Other\n\n## Evidence\n\n## Review\n\nnone\n")
			runGit(t, f.root, "add", other)
			runGit(t, f.root, "commit", "-q", "-m", "other")
			raw := getFile(t, other)
			putFile(t, other, strings.Replace(raw, "## Evidence\n", "## Evidence\nx\n", 1))
		}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pinClock(t)
			f := newRowsFixture(t, "")
			c.edit(t, f)
			sha := gitCurrentSHA(f.root)[:treeSHALen]
			want := sha
			if c.dirty {
				want += "+dirty"
			}
			cell, ok := hostRunnerCell(f.root, f.brief)
			if !ok || cell.RunnerTree != want {
				t.Fatalf("bracket open tree %q (ok=%v), want %q", cell.RunnerTree, ok, want)
			}
			res, out := captureVerifyrunOut(t, f.args("--rows", "2"))
			if res.code != verifyrunExitPass {
				t.Fatalf("--rows 2: exit %d %s", res.code, out)
			}
			got := parseEvidenceRows(evidenceOf(t, f.brief))["2"]
			if len(got) != 1 || witnessTreeOf(got[0].Text) != want {
				t.Fatalf("host row tree %+v, want %q", got, want)
			}
			if c.dirty {
				return
			}
			// The issue's order: a container row recorded from a receipt opened on
			// the clean home, then the host --rows run. The latest-PASS check now
			// reads one Runner cell.
			insert(t, f, "\n"+witnessTableFor(rfRowAt("1", "2026-10-22", receiptRunnerCell(cell)))+"\n")
			raw := getFile(t, f.brief)
			putFile(t, f.brief, strings.Replace(raw, "\n## Review\n", "\n"+vfPass+"\n\n## Review\n", 1))
			if _, err := flipLatestPass(evidenceOf(t, f.brief)); err != nil {
				t.Fatalf("mixed container + host run refused: %v", err)
			}
		})
	}
}

// regression: #4299
func TestEvidenceOnlyNeedsEvidence(t *testing.T) {
	pinClock(t)
	f := newRowsFixture(t, "")
	// A brief with no Evidence section at HEAD cannot have an Evidence-only edit.
	putFile(t, f.brief, strings.Replace(getFile(t, f.brief), "## Evidence\n", "## Notes\n", 1))
	runGit(t, f.root, "commit", "-q", "-am", "no evidence")
	putFile(t, f.brief, getFile(t, f.brief)+"\nmore\n")
	if cell, _ := hostRunnerCell(f.root, f.brief); !strings.HasSuffix(cell.RunnerTree, "+dirty") {
		t.Fatalf("tree %q reads clean without an Evidence section", cell.RunnerTree)
	}
	if err := os.Remove(f.brief); err != nil {
		t.Fatal(err)
	}
	if cell, _ := hostRunnerCell(f.root, f.brief); !strings.HasSuffix(cell.RunnerTree, "+dirty") {
		t.Fatalf("tree %q reads clean with the brief deleted", cell.RunnerTree)
	}
}
