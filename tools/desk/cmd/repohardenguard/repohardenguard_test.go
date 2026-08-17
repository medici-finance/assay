package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reposDeclared is the repos directive every fixture needs: the checklist must
// declare the set of repos it covers before any row may name one.
const reposDeclared = "<!-- repohardenguard:repos: o/r, o/r2 -->\n"

// The fixture mirrors the real checklist's shape: one public row, one
// admin-gated row, one ruleset row (two-hop), one not-available row.
const fixture = "" +
	"# fixture\n" +
	reposDeclared +
	rowsBegin + "\n" +
	"| ID | Repo | Setting | Gated | Read | Field | Required | Set |\n" +
	"|---|---|---|---|---|---|---|---|\n" +
	"| vis | o/r | visibility | public | `gh api repos/o/r` | visibility | public | admin only |\n" +
	"| scan | o/r | secret scanning | admin | `gh api repos/o/r` | security_and_analysis.secret_scanning.status | enabled | admin UI |\n" +
	"| bypass | o/r | protect-main bypass_actors | admin | `gh api repos/o/r/rulesets` | [name=protect-main].bypass_actors | [] | admin UI |\n" +
	"| tagrule | o/r | tag immutability | public | `gh api repos/o/r/rulesets` | [name=protect-release-tags].rules | contains:update | admin UI |\n" +
	"| doc | o/r | SECURITY.md present | public | `gh api repos/o/r/contents/SECURITY.md` | name | SECURITY.md | copy |\n" +
	"| rulesets | o/r2 | rulesets | admin | `gh api repos/o/r2/rulesets` | - | not available — private repo on a free plan | n/a |\n" +
	rowsEnd + "\n"

// fakeGH answers from a table of endpoint→body, and errors for anything else the
// way gh does (message plus "(HTTP nnn)").
type fakeGH struct {
	body map[string]string
	fail map[string]int // endpoint → status code
	seen []string
}

func (f *fakeGH) get(endpoint string) ([]byte, error) {
	f.seen = append(f.seen, endpoint)
	if code, ok := f.fail[endpoint]; ok {
		return nil, fmt.Errorf("gh api %s: HTTP %d (HTTP %d)", endpoint, code, code)
	}
	b, ok := f.body[endpoint]
	if !ok {
		return nil, fmt.Errorf("gh api %s: Not Found (HTTP 404)", endpoint)
	}
	return []byte(b), nil
}

// adminWorld is the world as a repository ADMIN sees it, fully hardened.
func adminWorld() *fakeGH {
	return &fakeGH{body: map[string]string{
		"user":       `{"login":"admin-person"}`,
		"repos/o/r":  `{"visibility":"public","security_and_analysis":{"secret_scanning":{"status":"enabled"}}}`,
		"repos/o/r2": `{"visibility":"private"}`,
		"repos/o/r/rulesets": `[{"id":1,"name":"protect-main"},` +
			`{"id":2,"name":"protect-release-tags"}]`,
		"repos/o/r/rulesets/1":           `{"id":1,"name":"protect-main","bypass_actors":[],"rules":[{"type":"pull_request"}]}`,
		"repos/o/r/rulesets/2":           `{"id":2,"name":"protect-release-tags","bypass_actors":[],"rules":[{"type":"update"},{"type":"deletion"}]}`,
		"repos/o/r/contents/SECURITY.md": `{"name":"SECURITY.md"}`,
	}}
}

// nonAdminWorld is the SAME hardened repo read with a token that lacks admin:
// security_and_analysis reads null and the ruleset detail carries no
// bypass_actors key. Both responses are HTTP 200. This is the exact shape
// measured against medici-finance/assay.
func nonAdminWorld() *fakeGH {
	w := adminWorld()
	w.body["user"] = `{"login":"member-person"}`
	w.body["repos/o/r"] = `{"visibility":"public","security_and_analysis":null}`
	w.body["repos/o/r/rulesets/1"] = `{"id":1,"name":"protect-main","current_user_can_bypass":"never","rules":[{"type":"pull_request"}]}`
	w.body["repos/o/r/rulesets/2"] = `{"id":2,"name":"protect-release-tags","current_user_can_bypass":"never","rules":[{"type":"update"},{"type":"deletion"}]}`
	return w
}

func writeFixture(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "checklist.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func runGuard(t *testing.T, w *fakeGH, path, repo string, extra ...string) (int, string, string) {
	t.Helper()
	orig := ghRun
	t.Cleanup(func() { ghRun = orig })
	ghRun = func(args ...string) ([]byte, error) {
		if len(args) != 2 || args[0] != "api" {
			t.Fatalf("guard shelled out to a non-GET gh call: %v", args)
		}
		return w.get(args[1])
	}
	var out, errb bytes.Buffer
	args := append([]string{"--repo", repo, "--checklist", path}, extra...)
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func stateOf(t *testing.T, out, id string) string {
	t.Helper()
	for _, ln := range strings.Split(out, "\n") {
		f := strings.Fields(ln)
		if len(f) >= 2 && f[len(f)-1] != "" {
			// state is the leading token(s); ids are the second column
			for i := range f {
				if f[i] == id && i > 0 {
					return strings.TrimSpace(strings.Join(f[:i], " "))
				}
			}
		}
	}
	t.Fatalf("row %q not present in output:\n%s", id, out)
	return ""
}

// Test names here carry an underscore on purpose. deskkit.BodyCheck refuses any
// 32-char run of [A-Za-z0-9+/=] that is not a git SHA or a slash-separated path,
// and a long CamelCase identifier is exactly such a run — it refused this
// branch's diff before the rename. The underscore breaks the run.
func TestAdminRun_HardenedRepoIsGreen(t *testing.T) {
	p := writeFixture(t, fixture)
	code, out, errb := runGuard(t, adminWorld(), p, "o/r")
	if code != deskkit.ExitOK {
		t.Fatalf("exit %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out, errb)
	}
	// The load-bearing assertion behind the greppable-output contract: an all-green run must not
	// contain the token at all, summary line included.
	if strings.Contains(out, string(StateUnknown)) {
		t.Fatalf("a green run mentions %q — a Verify row grepping for it would go red:\n%s", StateUnknown, out)
	}
	if !strings.Contains(out, "identity: admin-person") {
		t.Fatalf("output does not name the acting identity:\n%s", out)
	}
}

// The whole reason the program exists: the same hardened repo, read without
// admin, must report could-not-check on the admin-gated rows and exit non-zero.
// A two-state instrument passes this world, and reproduces #127.
func TestNonAdminReads_CouldNotCheckAndNonZeroExit(t *testing.T) {
	p := writeFixture(t, fixture)
	code, out, _ := runGuard(t, nonAdminWorld(), p, "o/r")
	if code == deskkit.ExitOK {
		t.Fatalf("a non-admin run exited 0 — that is the two-state failure #127 records:\n%s", out)
	}
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d (could-not-check, nothing wrong)\n%s", code, deskkit.ExitUnverifiable, out)
	}
	for _, id := range []string{"scan", "bypass"} {
		if got := stateOf(t, out, id); got != string(StateUnknown) {
			t.Errorf("admin-gated row %q reported %q, want %q — a null read is not an answer", id, got, StateUnknown)
		}
	}
	// Public rows are still genuinely checkable at this permission level.
	if got := stateOf(t, out, "vis"); got != string(StateOK) {
		t.Errorf("public row vis reported %q, want %q", got, StateOK)
	}
	if got := stateOf(t, out, "tagrule"); got != string(StateOK) {
		t.Errorf("public ruleset row reported %q, want %q", got, StateOK)
	}
}

// The required value lives in the DOCUMENT. Change it to
// something the world does not have and the verdict must flip.
func TestRequiredValueChange_FlipsTheVerdict(t *testing.T) {
	mutated := strings.Replace(fixture,
		"| vis | o/r | visibility | public | `gh api repos/o/r` | visibility | public |",
		"| vis | o/r | visibility | public | `gh api repos/o/r` | visibility | private |", 1)
	if mutated == fixture {
		t.Fatal("fixture mutation did not apply — the test would prove nothing")
	}
	p := writeFixture(t, mutated)
	code, out, _ := runGuard(t, adminWorld(), p, "o/r")
	if code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want %d on a mismatched value\n%s", code, deskkit.ExitRefused, out)
	}
	if got := stateOf(t, out, "vis"); got != string(StateWrong) {
		t.Fatalf("row vis reported %q, want %q — the guard fetched but did not compare", got, StateWrong)
	}
}

func TestBypassActors_NonEmptyIsWrong(t *testing.T) {
	w := adminWorld()
	w.body["repos/o/r/rulesets/1"] = `{"id":1,"name":"protect-main","bypass_actors":[{"actor_id":5,"actor_type":"Team","bypass_mode":"always"}]}`
	p := writeFixture(t, fixture)
	code, out, _ := runGuard(t, w, p, "o/r")
	if code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want %d — a populated bypass list must be red\n%s", code, deskkit.ExitRefused, out)
	}
	if got := stateOf(t, out, "bypass"); got != string(StateWrong) {
		t.Fatalf("bypass row reported %q, want %q", got, StateWrong)
	}
}

func TestNotAvailableRow_NeitherPassNorFail(t *testing.T) {
	p := writeFixture(t, fixture)
	w := adminWorld()
	code, out, _ := runGuard(t, w, p, "o/r2")
	if code != deskkit.ExitOK {
		t.Fatalf("exit %d, want 0 — a recorded divergence is not a failure\n%s", code, out)
	}
	if !strings.Contains(out, string(StateNotAvailable)) {
		t.Fatalf("output does not carry the literal %q:\n%s", StateNotAvailable, out)
	}
	// It must not have made the call it says is unavailable — asking would
	// produce a 403 that reads like a permission wall and muddy the two states
	// that matter. Assert on THIS world's call log, not a fresh one.
	if len(w.seen) == 0 {
		t.Fatal("the fake recorded no calls at all — the assertion below would be vacuous")
	}
	for _, ep := range w.seen {
		if ep == "repos/o/r2/rulesets" {
			t.Fatalf("the guard called %q, an endpoint the checklist records as not available", ep)
		}
	}
}

func TestAbsentFileOnPublicRow_IsWrongNotUnknown(t *testing.T) {
	w := adminWorld()
	delete(w.body, "repos/o/r/contents/SECURITY.md") // 404 from a readable repo = real absence
	p := writeFixture(t, fixture)
	code, out, _ := runGuard(t, w, p, "o/r")
	if code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want %d\n%s", code, deskkit.ExitRefused, out)
	}
	if got := stateOf(t, out, "doc"); got != string(StateWrong) {
		t.Fatalf("missing SECURITY.md reported %q, want %q", got, StateWrong)
	}
}

func TestForbiddenRead_UnknownEvenOnPublicRow(t *testing.T) {
	w := adminWorld()
	w.fail = map[string]int{"repos/o/r/contents/SECURITY.md": 403}
	p := writeFixture(t, fixture)
	code, out, _ := runGuard(t, w, p, "o/r")
	if got := stateOf(t, out, "doc"); got != string(StateUnknown) {
		t.Fatalf("a 403 reported %q, want %q — a permission wall says nothing about the value behind it", got, StateUnknown)
	}
	if code == deskkit.ExitOK {
		t.Fatalf("exit 0 with a could-not-check row\n%s", out)
	}
}

func TestUnreadableRepo_ReportsNothing(t *testing.T) {
	w := adminWorld()
	w.fail = map[string]int{"repos/o/r": 404}
	p := writeFixture(t, fixture)
	code, out, errb := runGuard(t, w, p, "o/r")
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if strings.Contains(out, string(StateWrong)) {
		t.Fatalf("a repo the token cannot read produced per-row verdicts:\n%s", out)
	}
	if !strings.Contains(errb, "cannot read o/r") {
		t.Fatalf("stderr does not say why nothing was checked: %s", errb)
	}
}

func TestMissingRulesetIsWrong(t *testing.T) {
	w := adminWorld()
	w.body["repos/o/r/rulesets"] = `[{"id":2,"name":"protect-release-tags"}]`
	p := writeFixture(t, fixture)
	code, out, _ := runGuard(t, w, p, "o/r")
	if code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want %d\n%s", code, deskkit.ExitRefused, out)
	}
	if got := stateOf(t, out, "bypass"); got != string(StateWrong) {
		t.Fatalf("a deleted ruleset reported %q, want %q", got, StateWrong)
	}
}

// A repo the checklist DECLARES but writes no rows for must refuse too: an empty
// row set checks nothing, and nothing-checked must never render as nothing-wrong.
func TestRepoWithNoRows_Refuses(t *testing.T) {
	body := "# lab\n" +
		"<!-- repohardenguard:repos: o/r, o/absent -->\n" +
		rowsBegin + "\n" +
		"| ID | Repo | Setting | Gated | Read | Field | Required | Set |\n" +
		"|---|---|---|---|---|---|---|---|\n" +
		"| vis | o/r | visibility | public | `gh api repos/o/r` | visibility | public | n/a |\n" +
		rowsEnd + "\n"
	p := writeFixture(t, body)
	code, out, errb := runGuard(t, adminWorld(), p, "o/absent")
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d — a repo with no rows must never look hardened\n%s%s", code, deskkit.ExitUnverifiable, out, errb)
	}
	if !strings.Contains(errb, "no rows for it") {
		t.Fatalf("stderr does not explain the refusal: %s", errb)
	}
}

// Finding 1 on PR #487. A row scoped to one repo whose Read cell reads a
// DIFFERENT repo used to be evaluated happily, and its verdict printed under the
// scoped repo's name — the guard certifying a repository it never measured.
// Without the parse-time coupling check this exits 0 and prints checked-ok.
func TestRowReadsAnotherRepo_IsFatalNotGreen(t *testing.T) {
	body := "# lab\n" +
		"<!-- repohardenguard:repos: o/r, o/other -->\n" +
		rowsBegin + "\n" +
		"| ID | Repo | Setting | Gated | Read | Field | Required | Set |\n" +
		"|---|---|---|---|---|---|---|---|\n" +
		"| lie | o/r | visibility | public | `gh api repos/o/other` | visibility | public | n/a |\n" +
		rowsEnd + "\n"
	w := adminWorld()
	w.body["repos/o/other"] = `{"visibility":"public"}` // the OTHER repo really is compliant
	p := writeFixture(t, body)
	code, out, errb := runGuard(t, w, p, "o/r")
	if code == deskkit.ExitOK {
		t.Fatalf("exit 0 — the guard certified o/r green from a read of o/other:\n%s", out)
	}
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d\n%s%s", code, deskkit.ExitUnverifiable, out, errb)
	}
	if strings.Contains(out, string(StateOK)) {
		t.Fatalf("a mis-scoped row produced a checked-ok verdict:\n%s", out)
	}
	if !strings.Contains(errb, "must not be measured against another") {
		t.Fatalf("stderr does not name the mis-scoping: %s", errb)
	}
}

// The boundary case that makes the coupling check non-trivial: `o/r` is a
// PREFIX of `o/r-toolkit`, exactly as `medici-finance/assay` is a prefix of
// `medici-finance/assay` in the shipped table. A prefix test without the
// segment separator would accept this row.
func TestPrefixRepoIsNotTheSameRepo(t *testing.T) {
	if scopedTo("repos/o/r-toolkit/rulesets", "o/r") {
		t.Fatal("repos/o/r-toolkit/rulesets accepted as scoped to o/r — a prefix is not a repo")
	}
	if scopedTo("repos/o/rr", "o/r") {
		t.Fatal("repos/o/rr accepted as scoped to o/r")
	}
	if !scopedTo("repos/o/r", "o/r") {
		t.Fatal("the bare repo endpoint must be scoped to itself")
	}
	if !scopedTo("repos/o/r/actions/permissions/workflow", "o/r") {
		t.Fatal("a sub-path of the repo must be scoped to it")
	}
}

// Finding 2 on PR #487, the same root cause. A one-character typo in a Repo cell
// used to DELETE the row: it matched no run, was never evaluated, and nothing in
// the output said so. The count stayed plausible while coverage shrank. The
// typo'd row here would otherwise be checked-wrong (no such ruleset exists).
func TestTypoedRepoCell_DoesNotSilentlyDropTheRow(t *testing.T) {
	body := "# lab\n" +
		"<!-- repohardenguard:repos: o/r -->\n" +
		rowsBegin + "\n" +
		"| ID | Repo | Setting | Gated | Read | Field | Required | Set |\n" +
		"|---|---|---|---|---|---|---|---|\n" +
		"| vis | o/r | visibility | public | `gh api repos/o/r` | visibility | public | n/a |\n" +
		"| tagimm | o/R | tag immutability | public | `gh api repos/o/r/rulesets` | [name=no-such-ruleset].enforcement | active | n/a |\n" +
		rowsEnd + "\n"
	p := writeFixture(t, body)
	code, out, errb := runGuard(t, adminWorld(), p, "o/r")
	if code == deskkit.ExitOK {
		t.Fatalf("exit 0 — a typo'd Repo cell removed a check and the run still went green:\n%s", out)
	}
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d\n%s%s", code, deskkit.ExitUnverifiable, out, errb)
	}
	// Either guard may fire first here — the typo desynchronises the Repo cell
	// from its own Read cell as well as from the declared set. What matters is
	// that the run refuses and names the row rather than dropping it.
	if !strings.Contains(errb, "tagimm") {
		t.Fatalf("stderr does not name the dropped row: %s", errb)
	}
}

// The form of Finding 2 that the Read/Repo coupling check CANNOT catch: the typo
// is applied consistently to both the Repo cell and its endpoint, so the two
// agree with each other and disagree only with reality. Nothing but membership
// in the declared set catches this, and without it the row is silently deleted
// and the run goes green.
func TestBothSidesTypoedRepo_IsFatalNotSilentlyDropped(t *testing.T) {
	body := "# lab\n" +
		"<!-- repohardenguard:repos: o/r -->\n" +
		rowsBegin + "\n" +
		"| ID | Repo | Setting | Gated | Read | Field | Required | Set |\n" +
		"|---|---|---|---|---|---|---|---|\n" +
		"| vis | o/r | visibility | public | `gh api repos/o/r` | visibility | public | n/a |\n" +
		"| tagimm | o/rr | tag immutability | public | `gh api repos/o/rr/rulesets` | [name=nope].enforcement | active | n/a |\n" +
		rowsEnd + "\n"
	p := writeFixture(t, body)
	code, out, errb := runGuard(t, adminWorld(), p, "o/r")
	if code == deskkit.ExitOK {
		t.Fatalf("exit 0 — a consistently typo'd repo deleted a check and the run went green:\n%s", out)
	}
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d\n%s%s", code, deskkit.ExitUnverifiable, out, errb)
	}
	if !strings.Contains(errb, "does not declare") {
		t.Fatalf("stderr does not name the undeclared repo cell: %s", errb)
	}
}

// The executed-check count must be visible and must account for the whole file,
// so a reader can see that no row went missing.
func TestScopeLine_AccountsForEveryRowInTheFile(t *testing.T) {
	p := writeFixture(t, fixture)
	_, out, _ := runGuard(t, adminWorld(), p, "o/r")
	// The fixture has 6 rows: 5 for o/r, 1 for o/r2.
	if !strings.Contains(out, "rows: 6 in the checklist") {
		t.Fatalf("output does not state the checklist's total row count:\n%s", out)
	}
	if !strings.Contains(out, "5 for o/r, evaluated below") {
		t.Fatalf("output does not state how many rows this run evaluated:\n%s", out)
	}
	if !strings.Contains(out, "1 for o/r2") {
		t.Fatalf("output does not account for the rows belonging to the other repo:\n%s", out)
	}
}

// A repo outside the declared set is a refusal, not an empty green run.
func TestUndeclaredRepoArgument_Refuses(t *testing.T) {
	p := writeFixture(t, fixture)
	code, out, errb := runGuard(t, adminWorld(), p, "o/never-heard-of-it")
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d\n%s%s", code, deskkit.ExitUnverifiable, out, errb)
	}
	if !strings.Contains(errb, "not a repo the checklist") {
		t.Fatalf("stderr does not explain the refusal: %s", errb)
	}
}

func TestChecklistParse_FailuresAreFatal(t *testing.T) {
	cases := map[string]string{
		"no markers":     "# just prose\n",
		"empty table":    reposDeclared + rowsBegin + "\n" + rowsEnd + "\n",
		"short row":      reposDeclared + rowsBegin + "\n| a | b |\n" + rowsEnd + "\n",
		"bad gated":      reposDeclared + rowsBegin + "\n| a | o/r | s | sometimes | `gh api repos/o/r` | visibility | public | x |\n" + rowsEnd + "\n",
		"not a gh api":   reposDeclared + rowsBegin + "\n| a | o/r | s | public | `curl https://api.github.com` | visibility | public | x |\n" + rowsEnd + "\n",
		"duplicate ids":  reposDeclared + rowsBegin + "\n| a | o/r | s | public | `gh api repos/o/r` | visibility | public | x |\n| a | o/r | s | public | `gh api repos/o/r` | visibility | public | x |\n" + rowsEnd + "\n",
		"no field":       reposDeclared + rowsBegin + "\n| a | o/r | s | public | `gh api repos/o/r` | - | public | x |\n" + rowsEnd + "\n",
		"bad repo shape": reposDeclared + rowsBegin + "\n| a | justname | s | public | `gh api repos/o/r` | visibility | public | x |\n" + rowsEnd + "\n",

		// The repos directive itself is required and must be unambiguous.
		"no repos directive": rowsBegin + "\n| a | o/r | s | public | `gh api repos/o/r` | visibility | public | x |\n" + rowsEnd + "\n",
		"empty repos directive": "<!-- repohardenguard:repos: -->\n" + rowsBegin +
			"\n| a | o/r | s | public | `gh api repos/o/r` | visibility | public | x |\n" + rowsEnd + "\n",
		"two repos directives": reposDeclared + reposDeclared + rowsBegin +
			"\n| a | o/r | s | public | `gh api repos/o/r` | visibility | public | x |\n" + rowsEnd + "\n",
		"declared repo not owner/name": "<!-- repohardenguard:repos: justname -->\n" + rowsBegin +
			"\n| a | o/r | s | public | `gh api repos/o/r` | visibility | public | x |\n" + rowsEnd + "\n",
		"row repo not declared": "<!-- repohardenguard:repos: o/r -->\n" + rowsBegin +
			"\n| a | o/elsewhere | s | public | `gh api repos/o/elsewhere` | visibility | public | x |\n" + rowsEnd + "\n",
		"read cell reads another repo": reposDeclared + rowsBegin +
			"\n| a | o/r | s | public | `gh api repos/o/r2` | visibility | public | x |\n" + rowsEnd + "\n",
		"not-available row mis-scoped": reposDeclared + rowsBegin +
			"\n| a | o/r | s | admin | `gh api repos/o/r2/rulesets` | - | not available — free plan | x |\n" + rowsEnd + "\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			p := writeFixture(t, body)
			code, _, errb := runGuard(t, adminWorld(), p, "o/r")
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("exit %d, want %d — a checklist the guard cannot parse must never exit 0 (%s)", code, deskkit.ExitUnverifiable, errb)
			}
		})
	}
}

// The surviving mutation from the #487 review: removing `|| cur == nil` from
// resolve() left the whole suite green, because the fixture's only null sits at
// a NON-terminal position where the next segment's type assertion catches it
// anyway. This pins the guarantee resolve()'s comment actually claims — a null
// at the END of the path is an absence, not a value. Without the guard the null
// reaches compare(), renders as the string "null", and the admin-gated row
// reports checked-wrong: a verdict manufactured from a non-answer.
func TestTerminalNull_IsAbsenceNotAValue(t *testing.T) {
	w := adminWorld()
	w.body["repos/o/r"] = `{"visibility":"public","security_and_analysis":{"secret_scanning":{"status":null}}}`
	p := writeFixture(t, fixture)
	code, out, _ := runGuard(t, w, p, "o/r")
	if got := stateOf(t, out, "scan"); got != string(StateUnknown) {
		t.Fatalf("a terminal null on an admin-gated row reported %q, want %q — null is not an answer", got, StateUnknown)
	}
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d\n%s", code, deskkit.ExitUnverifiable, out)
	}
}

// The same null on a PUBLIC row is a genuine absence, so it is checked-wrong
// rather than could-not-check. This pins the other half of resolve()'s contract:
// the nil guard routes to absent(), which is what distinguishes the two.
func TestTerminalNull_OnPublicRowIsWrong(t *testing.T) {
	w := adminWorld()
	w.body["repos/o/r"] = `{"visibility":null,"security_and_analysis":{"secret_scanning":{"status":"enabled"}}}`
	p := writeFixture(t, fixture)
	code, out, _ := runGuard(t, w, p, "o/r")
	if got := stateOf(t, out, "vis"); got != string(StateWrong) {
		t.Fatalf("a null on a public row reported %q, want %q", got, StateWrong)
	}
	if code == deskkit.ExitOK {
		t.Fatalf("exit 0 with a null value\n%s", out)
	}
}

func TestHTTPStatusParsing(t *testing.T) {
	cases := map[string]int{
		"gh api x: Bad credentials (HTTP 401)": 401,
		"gh api x: Not Found":                  404,
		"gh api x: dial tcp: no route":         0,
	}
	for msg, want := range cases {
		if got := httpStatus(fmt.Errorf("%s", msg)); got != want {
			t.Errorf("httpStatus(%q) = %d, want %d", msg, got, want)
		}
	}
}

// The shipped checklist must stay parseable and cover both repos. A table that
// rots into an unparseable shape would make every run exit 6 for the wrong
// reason; a table that loses a repo would silently stop checking it.
func TestShippedChecklistParses(t *testing.T) {
	// Relative to tools/desk/cmd/repohardenguard, the package directory `go test`
	// uses as its working directory.
	p := filepath.Join("..", "..", "..", "..", defaultChecklist)
	// The shipped checklist (docs/repo-hardening-checklist.md) is do-not-copy for
	// the public assay repo — self-classified "NOT FOR PUBLICATION". When it is
	// legitimately absent there is nothing to parse; the source repo always
	// carries it, so this parse-and-coverage check still runs there in full.
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		t.Skipf("%s not present in this tree — do-not-copy for the public assay repo (NOT FOR PUBLICATION)", p)
	}
	cl, err := ParseChecklistFile(p)
	if err != nil {
		t.Fatalf("the shipped checklist does not parse: %v", err)
	}
	rows := cl.Rows
	// The checklist must cover at least two repos — the public mirror and the
	// private source it is copied from — and one of them must be the public
	// medici-finance/assay. The private source repo is deliberately not named as
	// a literal here so this shipped test carries no private-repo slug; the
	// "every declared repo has rows" loop below is what guards against a declared
	// repo silently losing its coverage.
	if len(cl.Repos) < 2 {
		t.Errorf("the shipped checklist must cover at least two repos, got %v", cl.Repos)
	}
	if !slices.Contains(cl.Repos, "medici-finance/assay") {
		t.Errorf("the shipped checklist must cover medici-finance/assay, got %v", cl.Repos)
	}
	// Every declared repo must actually have rows, and every row's repo must be
	// declared. A declared repo with no rows would exit 6 for a confusing reason;
	// the reverse is a parse error, asserted separately.
	for _, repo := range cl.Repos {
		if !slices.Contains(RowRepos(rows), repo) {
			t.Errorf("the checklist declares %s but has no rows for it", repo)
		}
	}
	for _, repo := range RowRepos(rows) {
		if !cl.Covers(repo) {
			t.Errorf("row repo %s is not declared — the parser should have refused this", repo)
		}
	}

	// #127's bar, by row: fork-PR approval, default workflow permissions,
	// rulesets including tag immutability, secret scanning + push protection,
	// bypass_actors, and the two docs.
	needles := []string{"fork", "workflow-permissions", "secret-scanning", "push-protection",
		"ruleset-tags-immutable", "bypass", "doc-security", "doc-contributing"}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	joined := strings.Join(ids, " ")
	for _, n := range needles {
		if !strings.Contains(joined, n) {
			t.Errorf("no checklist row id contains %q — the #127 bar is not fully covered\nids: %s", n, joined)
		}
	}
}
