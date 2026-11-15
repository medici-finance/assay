package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The restamp-or-refuse rule (principalstamp.go, needsPrincipalStamp): an added App-run row's
// existing annotation is read with the lint's own parse and either left (it names the
// resolved principal), restamped (it names no roster principal, or is not in the `human:`
// form), or refused (it names a different recognised roster human). Rosters come from
// stampRoster: subject `operator` (login example-operator) and a second recognised human
// `colleague` (login example-colleague).

const restampHead = "# Brief\n\n## Evidence\n\n| # | Command | Exit | Result | Date | Runner |\n|---|---|---|---|---|---|\n"

// bracketExactRe is the exact-form stamp a container witness bracket's digest drops from a
// Runner cell (desktools-v2/71): `(on-behalf-of human:<subject>)`, the subject one or more
// characters other than whitespace and parentheses, matched case-sensitively. No looser form
// is dropped.
var bracketExactRe = regexp.MustCompile(`\(on-behalf-of human:[^\s()]+\)`)

var bracketSpaceRe = regexp.MustCompile(`[ \t]+`)

// bracketRunner is that digest's Runner-cell normalisation: drop every exact-form stamp,
// collapse each run of spaces and tabs to one space, trim.
func bracketRunner(cell string) string {
	return strings.TrimSpace(bracketSpaceRe.ReplaceAllString(bracketExactRe.ReplaceAllString(cell, ""), " "))
}

func restampRow(date, runner string) string {
	return "| 2 | `b` | 0 | ok | " + date + " | " + runner + " |\n"
}

type restampCase struct {
	name   string
	repo   string
	runner string // the Runner cell as landed
	want   string // the Runner cell expected on the branch; "" = refused, nothing written
}

func restampCases() []restampCase {
	const app = "example-app[bot] @ abc1234"
	pub, priv := stampPublicRepo, stampPrivateRepo
	return []restampCase{
		{"public/bare", pub, app + " (forge-identity)", app + " (on-behalf-of human:operator) (forge-identity)"},
		{"public/same-subject", pub, app + " (on-behalf-of human:operator) (forge-identity)", app + " (on-behalf-of human:operator) (forge-identity)"},
		{"public/same-subject-case", pub, app + " (On-Behalf-Of human:Operator)", app + " (on-behalf-of human:operator)"},
		{"public/same-subject-extra-words", pub, app + " (on-behalf-of human:operator and colleague)", app + " (on-behalf-of human:operator)"},
		{"public/desk-role", pub, app + " (on-behalf-of verify-desk) (forge-identity)", app + " (on-behalf-of human:operator) (forge-identity)"},
		{"public/desk-role-bare", pub, app + " on-behalf-of verify-desk", app + " (on-behalf-of human:operator)"},
		{"public/empty-human", pub, app + " (on-behalf-of human:) (ci-env)", app + " (on-behalf-of human:operator) (ci-env)"},
		{"public/unknown-human", pub, app + " (on-behalf-of human:someone) (ci-env)", app + " (on-behalf-of human:operator) (ci-env)"},
		{"public/login-form", pub, app + " (on-behalf-of human:example-operator) (forge-identity)", app + " (on-behalf-of human:operator) (forge-identity)"},
		{"public/first-occurrence-spoof", pub, app + " (on-behalf-of human:operator) (on-behalf-of human:someone) (git-config)", app + " (on-behalf-of human:operator) (git-config)"},
		{"public/unclosed", pub, app + " (on-behalf-of verify-desk", app + " (on-behalf-of human:operator)"},
		{"public/unclosed-before-tag", pub, app + " (on-behalf-of verify-desk (forge-identity)", app + " (on-behalf-of human:operator) (forge-identity)"},
		{"public/unclosed-human-before-tag", pub, app + " (on-behalf-of human:x (forge-identity)", app + " (on-behalf-of human:operator) (forge-identity)"},
		{"public/glued-tag-login", pub, app + " (on-behalf-of human:example-operator)(git-config)", app + " (on-behalf-of human:operator)(git-config)"},
		{"public/glued-before-login", pub, app + "(on-behalf-of human:example-operator) (forge-identity)", app + "(on-behalf-of human:operator) (forge-identity)"},
		{"public/tab-led-tag", pub, app + "\t(ci-env)", app + " (on-behalf-of human:operator)\t(ci-env)"},
		{"public/tab-led-tag-desk-role", pub, app + " (on-behalf-of verify-desk)\t(ci-env)", app + " (on-behalf-of human:operator)\t(ci-env)"},
		{"public/adjacent-fragments", pub, app + " (on-behalf-of a)(on-behalf-of b) (forge-identity)", app + " (on-behalf-of human:operator) (forge-identity)"},
		{"public/case-variant-tag", pub, app + " (Forge-Identity)", app + " (on-behalf-of human:operator) (Forge-Identity)"},
		{"public/different-human", pub, app + " (on-behalf-of human:colleague) (forge-identity)", ""},
		{"public/different-human-login", pub, app + " (on-behalf-of human:example-colleague)", ""},
		{"public/different-human-shadowed", pub, app + " (on-behalf-of human:colleague) (on-behalf-of human:operator)", ""},
		{"private/login", priv, app + " (on-behalf-of human:example-operator) (forge-identity)", app + " (on-behalf-of human:example-operator) (forge-identity)"},
		{"private/glued-tag-neutral", priv, app + " (on-behalf-of human:operator)(forge-identity)", app + " (on-behalf-of human:example-operator)(forge-identity)"},
		{"private/neutral-form", priv, app + " (on-behalf-of human:operator) (forge-identity)", app + " (on-behalf-of human:example-operator) (forge-identity)"},
		{"private/desk-role", priv, app + " (on-behalf-of the-desk)", app + " (on-behalf-of human:example-operator)"},
		{"private/different-human", priv, app + " (on-behalf-of human:colleague)", ""},
	}
}

// TestPrincipalStamp: every added App-run row is stamped, left or refused per the rule above,
// through the real landing path; a refusal writes nothing and names the row; a human runner
// and a row already on the branch are never touched, whatever they carry.
// regression: #4070
func TestPrincipalStamp(t *testing.T) {
	standing := "| 1 | `a` | 0 | ok | 2026-10-01 | example-app[bot] @ abc1234 (on-behalf-of verify-desk) |\n"
	human := "| 3 | `c` | 0 | ok | 2026-10-14 | human:operator @ abc1234 (on-behalf-of human:colleague) |\n"
	for _, tc := range restampCases() {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			f.setFile(stampBrief, restampHead+standing)
			landed := restampHead + standing + restampRow("2026-10-14", tc.runner) + human
			root := rootWithFile(t, stampBrief, landed)
			code := run([]string{tc.repo, "main", "--evidence-file", stampBrief, "--root", root})
			if tc.want == "" {
				if code != deskkit.ExitRefused {
					t.Fatalf("exit = %d, want %d (refused)", code, deskkit.ExitRefused)
				}
				if f.putCalls != 0 {
					t.Fatalf("a refused landing wrote %d time(s)", f.putCalls)
				}
				got := lastAudit(t).Detail
				if !strings.Contains(got, "different recognised roster human") || !strings.Contains(got, "line 8") ||
					!strings.Contains(got, strings.TrimSpace(restampRow("2026-10-14", tc.runner))) {
					t.Fatalf("refusal does not name the row: %q", got)
				}
				return
			}
			if code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0", code)
			}
			want := restampHead + standing + restampRow("2026-10-14", tc.want) + human
			if f.putContent != want {
				t.Fatalf("landed:\n%s\nwant:\n%s", f.putContent, want)
			}
			// A cell the tool wrote carries the stamp only as ONE exact-form parenthetical, so a
			// witness bracket's digest drops all of it. The digest is then unchanged exactly
			// when every annotation in the raw cell was itself exact-form; a non-exact one
			// (a desk role, an empty or spaced principal) stayed in the hash, so its restamp
			// moves the digest and the bracket fails closed.
			if tc.want != tc.runner {
				if n := len(bracketExactRe.FindAllString(tc.want, -1)); n != 1 {
					t.Fatalf("the written cell carries %d exact-form stamps, want 1: %q", n, tc.want)
				}
				landedNorm, rawNorm := bracketRunner(tc.want), bracketRunner(tc.runner)
				if strings.Contains(strings.ToLower(landedNorm), onBehalfOfCellMarker) {
					t.Fatalf("the written cell keeps an annotation the bracket digest does not drop: %q", landedNorm)
				}
				rawExact := !strings.Contains(strings.ToLower(rawNorm), onBehalfOfCellMarker)
				if (landedNorm == rawNorm) != rawExact {
					t.Fatalf("bracket digest: raw all exact-form = %v, but raw %q vs landed %q", rawExact, rawNorm, landedNorm)
				}
			}
			if tc.repo == stampPublicRepo && strings.Contains(strings.TrimPrefix(f.putContent, restampHead+standing), "human:"+stampLogin) {
				t.Fatalf("a public landing names the login:\n%s", f.putContent)
			}
		})
	}
}

// TestPrincipalStampLintParity runs the REAL statusgen --lint, with the same fixture roster
// configured, over each fixture's row before and after the stamp:
//   - wherever the stamp succeeds, the landed row carries no attribution PROBLEM — on both
//     sides of the cutover date;
//   - the raw rows the lint does reject (no annotation after the cutover, a non-`human:`
//     annotation, an unrecognised principal, a spoof the lint reads by its last occurrence)
//     are PROBLEMs before the stamp, so the clean result is the stamp's doing;
//   - a refusal (a different recognised human) is a row the lint ACCEPTS: the lint checks
//     only that a principal is recognised, never that it is this landing's principal, so the
//     refusal is the tool being deliberately stricter than the lint, not a lint verdict.
//
// regression: #4070
func TestPrincipalStampLintParity(t *testing.T) {
	landedStatusgenOnPath(t)
	setupStamp(t, true)
	for _, line := range strings.Split(stampRoster(true), "\n") {
		if key, value, ok := strings.Cut(line, "="); ok && strings.HasPrefix(key, "ASSAY_") {
			t.Setenv(key, value)
		}
	}
	// rawProblem: the lint rejects the raw row (post-cutover dates). Pre-cutover, an
	// unannotated or non-`human:` row is only a NOTICE; an unrecognised principal is a
	// PROBLEM at any date.
	rawProblem := map[string]bool{
		"public/bare": true, "public/desk-role": true, "public/desk-role-bare": true,
		"public/empty-human": true, "public/unknown-human": true, "public/first-occurrence-spoof": true,
		"public/unclosed": true, "private/desk-role": true,
		"public/unclosed-before-tag": true, "public/unclosed-human-before-tag": true,
		"public/tab-led-tag": true, "public/tab-led-tag-desk-role": true,
		"public/adjacent-fragments": true, "public/case-variant-tag": true,
	}
	anyDateProblem := map[string]bool{"public/unknown-human": true, "public/first-occurrence-spoof": true,
		"public/unclosed-human-before-tag": true}
	for _, date := range []string{"2026-10-14", "2026-09-01"} {
		for _, tc := range restampCases() {
			t.Run(date+"/"+tc.name, func(t *testing.T) {
				raw := restampHead + restampRow(date, tc.runner)
				out, stamped, err := stampEvidencePrincipal([]byte(raw), []byte(restampHead),
					func() (string, error) { return principalSubjectFn(tc.repo) })
				rawProblems := parityLint(t, raw)
				if err != nil {
					if tc.want != "" {
						t.Fatalf("stamp refused a fixture it should land: %v", err)
					}
					if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
						t.Fatalf("refusal exit = %d, want %d", deskkit.ExitCodeOf(err), deskkit.ExitRefused)
					}
					if out != nil || stamped != nil {
						t.Fatalf("a refusal returned content to write:\n%s", out)
					}
					// Outside the parity claim, and stated: the lint accepts this row (any
					// recognised human passes it), so the refusal is the tool being stricter.
					if len(rawProblems) != 0 {
						t.Fatalf("the refused fixture is one the lint also rejects — the stricter-than-lint premise no longer holds:\n%s",
							strings.Join(rawProblems, "\n"))
					}
					return
				}
				if tc.want == "" {
					t.Fatalf("stamp landed a fixture it must refuse:\n%s", out)
				}
				if got := parityLint(t, string(out)); len(got) != 0 {
					t.Fatalf("the stamped row still carries an attribution PROBLEM:\n%s\nrow: %s", strings.Join(got, "\n"), stamped)
				}
				if !strings.Contains(string(out), restampRow(date, tc.want)) {
					t.Fatalf("stamped content lacks the expected row (target's form) %q:\n%s", tc.want, out)
				}
				wantRaw := rawProblem[tc.name]
				if date < "2026-09-17" {
					wantRaw = anyDateProblem[tc.name]
				}
				if wantRaw != (len(rawProblems) != 0) {
					t.Fatalf("control: raw-row PROBLEM = %v, want %v:\n%s", len(rawProblems) != 0, wantRaw, strings.Join(rawProblems, "\n"))
				}
			})
		}
	}
}

// parityLint lands content as the brief of a one-stream tree and returns the attribution
// PROBLEM lines statusgen --lint reports for it.
func parityLint(t *testing.T, evidence string) []string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "mp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	board := "---\nstream: mp\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# MP\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [Parity](./brief-01-parity.md) | 0 | S | implemented | — | — |\n"
	fixture, err := os.ReadFile(filepath.Join("testdata", "principal-stamp", "brief-01-unstamped.md"))
	if err != nil {
		t.Fatal(err)
	}
	brief := string(fixture[:bytes.Index(fixture, []byte("## Evidence"))]) + strings.TrimPrefix(evidence, "# Brief\n\n")
	for name, body := range map[string]string{"README.md": board, "brief-01-parity.md": brief} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	problems, err := statusgenLintAt(root, childOptions{})
	if err != nil {
		t.Fatalf("statusgen --lint: %v", err)
	}
	var out []string
	for _, p := range problems {
		if strings.Contains(p, "on-behalf-of principal") {
			out = append(out, p)
		}
	}
	return out
}

// TestDryRunRestampsPlantedPrincipal: a --dry-run of a landing whose App row carries a planted
// desk-role annotation runs the PROBLEM-diff on the RESTAMPED content, prints the restamped
// row as it would land, and writes nothing — so the dry-run, not a lint of the home, is the
// verifier's pre-check.
// regression: #4070
func TestDryRunRestampsPlantedPrincipal(t *testing.T) {
	f := setupStamp(t, true)
	planted := restampRow("2026-10-14", "example-app[bot] @ abc1234 (on-behalf-of verify-desk) (forge-identity)")
	restamped := restampRow("2026-10-14", "example-app[bot] @ abc1234 (on-behalf-of human:operator) (forge-identity)")
	f.setFile(stampBrief, restampHead)
	root := rootWithFile(t, stampBrief, restampHead+planted)
	var linted []byte
	old := lintDiffFn
	lintDiffFn = func(_, _ string, content []byte, options childOptions) ([]string, error) {
		linted = content
		return nil, nil
	}
	t.Cleanup(func() { lintDiffFn = old })
	if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root, "--dry-run"}); code != deskkit.ExitOK {
		t.Fatalf("dry-run exit = %d, want 0", code)
	}
	if f.putCalls != 0 {
		t.Fatalf("--dry-run wrote %d time(s)", f.putCalls)
	}
	if string(linted) != restampHead+restamped {
		t.Fatalf("the PROBLEM-diff did not run on the restamped content; linted:\n%s", linted)
	}
	out := stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "dry-run plan") || !strings.Contains(out, strings.TrimSpace(restamped)) {
		t.Fatalf("dry-run output does not show the restamped row:\n%s", out)
	}
	if strings.Contains(out, "verify-desk") {
		t.Fatalf("dry-run output still shows the planted annotation:\n%s", out)
	}
}

// TestPrincipalStampRefusesUnreadable: an added App-run row whose Runner cell the lint would
// read differently from its rendered text is refused before any write, never stamped:
//   - an HTML comment crossing a line break (the lint removes it and reads the two lines as
//     one row, so a different annotation becomes this row's principal);
//   - a character whose lowercase form has a different byte length (the lint matches in a
//     lowercased copy and slices the original at those offsets — Ⱥ grows, the Kelvin sign
//     shrinks — so it would read a principal the cell does not render);
//   - an invisible format character in the marker, and a comment inside the cell (the
//     rendered marker and the one the lint matches are different text);
//   - any other non-ASCII character (a combining mark, a Cyrillic homoglyph in the marker), and
//     a control character anywhere in the row (a lone CR renders a second row the lint never
//     reads).
//
// regression: #4070
func TestPrincipalStampRefusesUnreadable(t *testing.T) {
	const app = "example-app[bot] @ abc1234"
	const crosses, inside, length = "crosses a line break", "sits inside the cell", "lowercase form has a different byte length"
	cases := []struct{ name, row, why string }{
		{"comment-join-colleague", "| 2 | `b` | 0 | ok | 2026-10-14 | " + app + " <!--\n--> (on-behalf-of human:colleague) |\n", crosses},
		{"comment-join-unknown", "| 2 | `b` | 0 | ok | 2026-10-14 | " + app + " <!--\n--> (on-behalf-of human:someone) |\n", crosses},
		{"lowercase-grows-10", restampRow("2026-10-14", app+" "+strings.Repeat("\u023a", 10)+" (on-behalf-of human:colleague)operator)"), length},
		{"lowercase-grows-18", restampRow("2026-10-14", app+" "+strings.Repeat("\u023a", 18)+" (on-behalf-of human:example-colleague)operator)"), length},
		{"lowercase-shrinks", restampRow("2026-10-14", app+" \u212a\u212a (on-behalf-of human:operator)"), length},
		{"zero-width-in-marker", restampRow("2026-10-14", app+" (on-behalf\u200b-of human:colleague)"), "invisible format character U+200B"},
		{"comment-in-marker", restampRow("2026-10-14", app+" (on-behalf-<!-- -->of human:colleague) (forge-identity)"), inside},
		{"invalid-utf8", restampRow("2026-10-14", app+" \xff (on-behalf-of human:operator)"), "not valid UTF-8"},
		{"combining-mark-in-marker", restampRow("2026-10-14", app+" (on-behalf-of́ human:colleague)"), "non-ASCII character U+0301"},
		{"homoglyph-in-marker", restampRow("2026-10-14", app+" (оn-behalf-of human:colleague)"), "non-ASCII character U+043E"},
		{"lone-cr-second-app-row", "| 2 | `b` | 0 | ok | 2026-10-14 | " + app + " |\r| 3 | `c` | 0 | ok | 2026-10-14 | " + app +
			" (on-behalf-of human:colleague) |\n", "control character U+000D"},
		{"lone-cr-after-human-row", "| 2 | `b` | 0 | ok | 2026-10-14 | human:operator @ abc1234 |\r| 3 | `c` | 0 | ok | 2026-10-14 | " +
			app + " (on-behalf-of human:colleague) |\n", "control character U+000D"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			f.setFile(stampBrief, restampHead)
			root := rootWithFile(t, stampBrief, restampHead+tc.row)
			if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d (refused); landed:\n%s", code, deskkit.ExitRefused, f.putContent)
			}
			if f.putCalls != 0 {
				t.Fatalf("an unreadable Runner cell still wrote %d time(s):\n%s", f.putCalls, f.putContent)
			}
			if got := lastAudit(t).Detail; !strings.Contains(got, "cannot read the way the lint does") || !strings.Contains(got, "line 7") ||
				!strings.Contains(got, tc.why) {
				t.Fatalf("refusal is not the unreadable-cell refusal for the row: %q", got)
			}
		})
	}
}

// TestPrincipalStampRefusesControlInAnyLine: a control character in ANY line a landing adds to
// the Evidence section is refused before any write, whatever kind of line it is. The lint
// splits lines on LF only and reads only data rows, but a renderer can treat a lone CR as a
// line break: a header line can then hide a data row, a separator line can render as two, and
// a line that is not a table row at all can hold a whole table, each showing an App row naming
// another human that the lint never reads. A C1 control (NEL) is refused the same way. A line
// counts as already on the branch only when it stands in the base's own Evidence section, so a
// line copied in from elsewhere in the base file is checked too; one already in the base's
// Evidence section is not this landing's and is left alone.
//
// regression: #4070
func TestPrincipalStampRefusesControlInAnyLine(t *testing.T) {
	const app = "example-app[bot] @ abc1234"
	other := "| 3 | " + app + " (on-behalf-of human:colleague) |"
	crRow := "| 2 | `b` | 0 | ok | 2026-10-14 | " + app + " |\r" + other
	cases := []struct {
		name, base, added, line, why string
	}{
		{"header-hides-row", restampHead, "\n| # | Runner |\r" + other + "\n|---|---|\n", "line 8", "U+000D"},
		{"separator-line", restampHead, "\n| # | Runner |\n|---|---|\r|---|---|\n| 3 | " + app + " |\n", "line 9", "U+000D"},
		{"non-table-line-holds-table", restampHead, "\nnote\r| # | Runner |\r|---|---|\r" + other + "\n", "line 8", "U+000D"},
		{"c1-next-line", restampHead, "| 2 | `b` | 0 | ok\u0085 | 2026-10-14 | human:operator @ abc1234 |\n", "line 7", "U+0085"},
		{"line-from-another-section", restampHead + "\n## Notes\n\n" + crRow + "\n", crRow + "\n", "line 7", "U+000D"},
		// regression: #4075 — the heading line that ends the section is the landing's too.
		{"section-ending-heading", restampHead, "\n## \r| # | Runner |\r|---|---|\r" + other + "\n", "line 8", "U+000D"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			f.setFile(stampBrief, tc.base)
			content := restampHead + tc.added + strings.TrimPrefix(tc.base, restampHead)
			root := rootWithFile(t, stampBrief, content)
			if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d (refused); landed:\n%q", code, deskkit.ExitRefused, f.putContent)
			}
			if f.putCalls != 0 {
				t.Fatalf("a control character still wrote %d time(s):\n%q", f.putCalls, f.putContent)
			}
			if got := lastAudit(t).Detail; !strings.Contains(got, "cannot read the way the lint does") ||
				!strings.Contains(got, tc.line) || !strings.Contains(got, "control character "+tc.why) {
				t.Fatalf("refusal is not the control-character refusal for %s: %q", tc.line, got)
			}
		})
	}
	t.Run("already-in-base-evidence", func(t *testing.T) {
		f := setupStamp(t, true)
		base := restampHead + crRow + "\n"
		f.setFile(stampBrief, base)
		root := rootWithFile(t, stampBrief, base+"| 4 | `d` | 0 | ok | 2026-10-14 | human:operator @ abc1234 |\n")
		if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want 0: a line already in the base Evidence section is not this landing's (%q)",
				code, lastAudit(t).Detail)
		}
	})
}
