package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The on-behalf-of stamp (principalstamp.go). Every test plants its own roster: a blessing
// authority whose LOGIN (example-operator) differs from the NEUTRAL name the human-login map
// carries for it (operator), and one public and one private repo — so a leaked login and a
// correct neutral name can never render alike.

const (
	stampPublicRepo  = "example-org/stamp-public"
	stampPrivateRepo = "example-org/stamp-closed"
	stampLogin       = "example-operator"
	stampName        = "operator"
)

func stampRoster(humanMap bool) string {
	r := "ASSAY_BLESS_LOGIN=example-operator:4001\n" +
		"ASSAY_TRUSTED_LOGINS=example-operator:4001,shared-agent:4002,example-colleague:4003\n" +
		"ASSAY_TRUSTED_BOT_SLUGS=desk=assay-desk-app:300000001,reviewer=assay-reviewer-app:300000004," +
		"verifier=assay-verifier-app:300000005,worker=assay-worker-app:300000006\n" +
		"ASSAY_ALLOWED_REPOS=" + stampPublicRepo + ":ci:public," + stampPrivateRepo + ":ci:private\n"
	if humanMap {
		r += "ASSAY_HUMAN_LOGIN_MAP=operator:example-operator,colleague:example-colleague\n"
	}
	return r
}

// setupStamp is setupFake with the stamp roster planted over the fixture roster.
func setupStamp(t *testing.T, humanMap bool) *fakeForge {
	t.Helper()
	f, _ := setupFake(t)
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(stampRoster(humanMap)), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	return f
}

func readStampFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "principal-stamp", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const stampBrief = "docs/streams/mp/brief-01-stamped.md"

// regression: #2976
// TestStampAppRowPublic: a direct-write landing into a PUBLIC repo stamps every App-run row
// that carries no principal with the roster's NEUTRAL name, before a trailing source tag, and
// the result is byte-for-byte the golden the statusgen lint test reads. The login appears
// nowhere, the human-run row is untouched, and an ambient env value is never the source.
func TestStampAppRowPublic(t *testing.T) {
	f := setupStamp(t, true)
	t.Setenv("ASSAY_BLESS_LOGIN", "intruder:9")
	root := rootWithFile(t, stampBrief, readStampFixture(t, "brief-01-unstamped.md"))
	code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root})
	if code != deskkit.ExitOK {
		t.Fatalf("direct-write exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("want 1 write, got %d", f.putCalls)
	}
	if want := readStampFixture(t, "brief-01-stamped.md"); f.putContent != want {
		t.Fatalf("landed content differs from the golden:\n--- got\n%s\n--- want\n%s", f.putContent, want)
	}
	if strings.Contains(f.putContent, stampLogin) || strings.Contains(f.putContent, "intruder") {
		t.Fatalf("a public landing carries the login or the env value:\n%s", f.putContent)
	}
}

// regression: #2976
// TestStampPrivateNamesLogin: a target the roster states private names the login, as the commit trailer does.
func TestStampPrivateNamesLogin(t *testing.T) {
	f := setupStamp(t, true)
	root := rootWithFile(t, stampBrief, readStampFixture(t, "brief-01-unstamped.md"))
	if code := run([]string{stampPrivateRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("private direct-write exit = %d, want 0", code)
	}
	want := "example-app[bot] @ abc1234 (on-behalf-of human:" + stampLogin + ") (forge-identity) |"
	if !strings.Contains(f.putContent, want) {
		t.Fatalf("private landing missing %q:\n%s", want, f.putContent)
	}
}

// regression: #2976
// TestStampBriefFragment: a headerless --brief-path fragment is read under the brief's own
// table header (Runner is NOT the last column here), so the Runner cell is the one stamped;
// a re-run of the same fragment is then a block-level no-op, not a second landing.
func TestStampBriefFragment(t *testing.T) {
	f := setupStamp(t, true)
	brief := "# Brief\n\n## Evidence\n\n| # | Date | Runner | Result |\n|---|------|--------|--------|\n" +
		"| 1 | 2026-10-14 | human:operator | ok |\n"
	f.setFile(stampBrief, brief)
	frag := "| 2 | 2026-10-14 | example-app[bot] @ abc1234 (ci-env) | ok |\n"
	local := writeRepoFile(t, "row.md", frag)
	if code := run([]string{stampPublicRepo, "main", "--evidence-file", local, "--brief-path", stampBrief}); code != deskkit.ExitOK {
		t.Fatalf("brief-path exit = %d, want 0", code)
	}
	want := brief + "| 2 | 2026-10-14 | example-app[bot] @ abc1234 (on-behalf-of human:operator) (ci-env) | ok |\n"
	if f.putContent != want {
		t.Fatalf("merged brief:\n%s\nwant:\n%s", f.putContent, want)
	}
	if code := run([]string{stampPublicRepo, "main", "--evidence-file", local, "--brief-path", stampBrief}); code != deskkit.ExitOK {
		t.Fatalf("re-run exit = %d, want 0", code)
	}
	if f.putCalls != 1 {
		t.Fatalf("a re-run of a stamped block must be a noop; got %d writes", f.putCalls)
	}
}

// regression: #2976
// TestStampLeavesRowsAlone: human-run rows (even one carrying annotation-like text) and an
// unstamped App row ALREADY on the branch land byte-identical; with no added App row the
// roster is never consulted. (Added App rows that already carry an annotation are classified
// against the resolved principal — TestPrincipalStamp.)
func TestStampLeavesRowsAlone(t *testing.T) {
	f := setupStamp(t, true)
	calls := 0
	old := principalSubjectFn
	principalSubjectFn = func(repo string) (string, error) { calls++; return old(repo) }
	t.Cleanup(func() { principalSubjectFn = old })
	standing := "| 1 | `a` | 0 | ok | 2026-10-01 | example-app[bot] @ abc1234 (git-config) |\n"
	head := "# Brief\n\n## Evidence\n\n| # | Command | Exit | Result | Date | Runner |\n|---|---|---|---|---|---|\n"
	f.setFile(stampBrief, head+standing)
	frag := "| 3 | `c` | 0 | ok | 2026-10-14 | human:operator @ abc1234 |\n" +
		"| 4 | `d` | 0 | ok | 2026-10-14 | human:operator @ abc1234 (on-behalf-of desk) |\n"
	root := rootWithFile(t, stampBrief, head+standing+frag)
	if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	if f.putContent != head+standing+frag {
		t.Fatalf("rows that need no stamp were rewritten:\n%s", f.putContent)
	}
	if calls != 0 {
		t.Fatalf("the stamp resolved a principal %d time(s) with nothing to stamp", calls)
	}
}

// regression: #2976
// TestStampRefusesNoNeutral: a public target whose roster carries no neutral name for the
// blessing authority cannot be stamped without naming the login, so the landing refuses
// (exit 5) and nothing is written — no placeholder, no login.
func TestStampRefusesNoNeutral(t *testing.T) {
	f := setupStamp(t, false)
	root := rootWithFile(t, stampBrief, readStampFixture(t, "brief-01-unstamped.md"))
	code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d (refused)", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("an unstampable landing still wrote %d time(s)", f.putCalls)
	}
	if got := lastAudit(t).Detail; !strings.Contains(got, "cannot stamp") {
		t.Fatalf("refusal is not the stamp's own: %q", got)
	}
}

// regression: #2976
// TestStampRefusesNoRoster: with the roster gone the resolver refuses, and the stamp refuses
// with it before any write; exporting the bless login in the shell does not stand in for it.
func TestStampRefusesNoRoster(t *testing.T) {
	f := setupStamp(t, true)
	root := rootWithFile(t, stampBrief, readStampFixture(t, "brief-01-unstamped.md"))
	old := principalSubjectFn
	principalSubjectFn = func(repo string) (string, error) {
		if err := os.Remove(filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env")); err != nil {
			t.Fatal(err)
		}
		deskkit.ReloadConfig()
		return old(repo)
	}
	t.Cleanup(func() { principalSubjectFn = old })
	t.Setenv("ASSAY_BLESS_LOGIN", "example-operator:4001")
	if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("wrote %d time(s) with no principal", f.putCalls)
	}
	if got := lastAudit(t).Detail; !strings.Contains(got, "cannot stamp") {
		t.Fatalf("refusal is not the stamp's own: %q", got)
	}
}

// TestPrincipalStampReadsBodyNotFrontmatter: the stamp, its self-check and the backstop find
// the Evidence section in the brief BODY, as the lint does. A `## Evidence` line inside the
// frontmatter (a YAML comment there) does not move them off the real section: a body App row
// naming a different recognised human is still refused, an unannotated one is still stamped,
// and a frontmatter block that never closes refuses rather than land rows the lint cannot read.
//
// regression: #4070
func TestPrincipalStampReadsBodyNotFrontmatter(t *testing.T) {
	head := "---\nschema: brief-v1\n## Evidence\ngate: model\n---\n\n# Brief\n\n## Evidence\n\n" +
		"| # | Result | Date | Runner |\n|---|---|---|---|\n"
	for _, tc := range []struct {
		name, base, content, want string
		code                      int
		why                       string // a refusal's detail must name it ("" = any refusal)
	}{
		{name: "different-human", base: head,
			content: head + "| 1 | ok | 2026-10-23 | example-app[bot] @ abc1234 (on-behalf-of human:colleague) |\n",
			code:    deskkit.ExitRefused},
		{name: "different-human-new-file",
			content: head + "| 1 | ok | 2026-10-23 | example-app[bot] @ abc1234 (on-behalf-of human:colleague) |\n",
			code:    deskkit.ExitRefused},
		{name: "unannotated", base: head,
			content: head + "| 1 | ok | 2026-10-23 | example-app[bot] @ abc1234 (forge-identity) |\n",
			code:    deskkit.ExitOK,
			want:    head + "| 1 | ok | 2026-10-23 | example-app[bot] @ abc1234 (on-behalf-of human:operator) (forge-identity) |\n"},
		{name: "unclosed-frontmatter",
			content: "---\nschema: brief-v1\n\n# Brief\n\n## Evidence\n\n| # | Result | Date | Runner |\n|---|---|---|---|\n" +
				"| 1 | ok | 2026-10-23 | example-app[bot] @ abc1234 (forge-identity) |\n",
			code: deskkit.ExitRefused, why: "frontmatter block that never closes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			if tc.base != "" {
				f.setFile(stampBrief, tc.base)
			}
			root := rootWithFile(t, stampBrief, tc.content)
			if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != tc.code {
				t.Fatalf("exit = %d, want %d", code, tc.code)
			}
			if tc.code != deskkit.ExitOK {
				if f.putCalls != 0 {
					t.Fatalf("a refused landing still wrote %d time(s):\n%s", f.putCalls, f.putContent)
				}
				if got := lastAudit(t).Detail; !strings.Contains(got, tc.why) {
					t.Fatalf("refusal detail lacks %q: %q", tc.why, got)
				}
				return
			}
			if f.putContent != tc.want {
				t.Fatalf("landed:\n%s\nwant:\n%s", f.putContent, tc.want)
			}
		})
	}
}

// TestPrincipalStampRelandedRowUnderNewHeader: a row already on the branch counts as checked
// only where the lint reads it the same way. The base has the line under a header whose Runner
// column is a human; re-landed byte-identical under a header with no Runner column, the lint
// reads its last cell, an App runner naming a different human, so the landing is refused.
// The same line left under its own header lands as before.
//
// regression: #4070
func TestPrincipalStampRelandedRowUnderNewHeader(t *testing.T) {
	row := "| 1 | human:operator @ abc1234 | example-app[bot] @ abc1234 (on-behalf-of human:colleague) |\n"
	base := "# Brief\n\n## Evidence\n\n| # | Runner | Note |\n|---|---|---|\n" + row
	for _, tc := range []struct {
		name, content string
		code          int
	}{
		{"moved-runner-column", base + "\n| # | Who | Note |\n|---|---|---|\n" + row, deskkit.ExitRefused},
		{"same-header", base + "| 2 | human:operator @ abc1234 | ok |\n", deskkit.ExitOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupStamp(t, true)
			f.setFile(stampBrief, base)
			root := rootWithFile(t, stampBrief, tc.content)
			if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != tc.code {
				t.Fatalf("exit = %d, want %d", code, tc.code)
			}
			if tc.code != deskkit.ExitOK && f.putCalls != 0 {
				t.Fatalf("a refused landing still wrote %d time(s):\n%s", f.putCalls, f.putContent)
			}
		})
	}
}

// TestLandedPrincipalBackstop: the landing's last check reads the landed Evidence section the
// way the lint does — closed comments REMOVED, so a comment crossing a line break joins the
// lines around it — and accepts an added App-run Runner cell only when that reading carries
// exactly one annotation, in the exact form the stamp writes, naming the resolved subject.
//
// regression: #4070
func TestLandedPrincipalBackstop(t *testing.T) {
	section := "## Evidence\n\n| # | Result | Runner |\n|---|---|---|\n" +
		"| 1 | ok | example-app[bot] @ abc1234 <!--\n--> (on-behalf-of human:colleague) |\n"
	rows := lintEvidenceRunners(section)
	if len(rows) != 1 || rows[0].runner != "example-app[bot] @ abc1234  (on-behalf-of human:colleague)" {
		t.Fatalf("the lint-way read did not join the commented lines into one row: %+v", rows)
	}
	for runner, wantOK := range map[string]bool{
		"example-app[bot] @ abc1234 (on-behalf-of human:operator) (forge-identity)":                            true,
		"example-app[bot] @ abc1234 (forge-identity)":                                                          false,
		"example-app[bot] @ abc1234 (on-behalf-of human:colleague)":                                            false,
		"example-app[bot] @ abc1234 (On-Behalf-Of human:Operator)":                                             false,
		"example-app[bot] @ abc1234 (on-behalf-of human:operator) (on-behalf-of human:operator)":               false,
		"example-app[bot] @ abc1234 on-behalf-of human:operator":                                               false,
		"example-app[bot] @ abc1234 \u212a\u212a (on-behalf-of human:operator)":                                false,
		"example-app[bot] @ abc1234 (on-behalf-of human:colleague) (on-behalf-of human:operator) (git-config)": false,
	} {
		if got := landedPrincipalProblem(runner, stampName); (got == "") != wantOK {
			t.Fatalf("landedPrincipalProblem(%q) = %q, want accepted = %v", runner, got, wantOK)
		}
	}
}
