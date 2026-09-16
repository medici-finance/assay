package main

import (
	"bytes"
	"encoding/json"
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
// admin-gated row, one ruleset row (two-hop), one not-available row. Read cells use the
// the forge-gitlab guard-read-custody brief's grammar (`read <kind>` / `read file <path>`) — the retired `gh api
// <endpoint>` form is exercised separately (TestChecklistRefusesGhApiCell).
const fixture = "" +
	"# fixture\n" +
	reposDeclared +
	rowsBegin + "\n" +
	"| ID | Repo | Setting | Gated | Read | Field | Required | Set |\n" +
	"|---|---|---|---|---|---|---|---|\n" +
	"| vis | o/r | visibility | public | `read repo` | visibility | public | admin only |\n" +
	"| scan | o/r | secret scanning | admin | `read repo` | security_and_analysis.secret_scanning.status | enabled | admin UI |\n" +
	"| bypass | o/r | protect-main bypass_actors | admin | `read rulesets` | [name=protect-main].bypass_actors | [] | admin UI |\n" +
	"| tagrule | o/r | tag immutability | public | `read rulesets` | [name=protect-release-tags].rules | contains:update | admin UI |\n" +
	"| doc | o/r | SECURITY.md present | public | `read file SECURITY.md` | - | present | copy |\n" +
	"| rulesets | o/r2 | rulesets | admin | `read rulesets` | - | not available — private repo on a free plan | n/a |\n" +
	rowsEnd + "\n"

// stubForge is a minimal deskkit.Forge for these tests: it embeds the interface (nil), so it
// satisfies deskkit.Forge at compile time, and overrides only the two operations
// cmd/repohardenguard actually calls (RepoHardeningRead, ReadFile) — any other method called
// through it panics on the embedded nil, which would itself fail a test loudly rather than
// silently answering something plausible.
type stubForge struct {
	deskkit.Forge
	hardening map[string]hardeningFixture
	files     map[string]fileFixture
	seen      []string // kinds/paths read, in call order
	// kind is the forge this stub stands in for — what forgeForFn reports as the resolution,
	// and therefore which preflight document the guard asks for. "" reads as GitHub.
	kind deskkit.ForgeKind
}

type hardeningFixture struct {
	raw json.RawMessage
	err error
}

type fileFixture struct {
	fc  *deskkit.FileContent
	err error
}

func (s *stubForge) RepoHardeningRead(repo deskkit.ForgeRepo, kind deskkit.HardeningReadKind) (json.RawMessage, error) {
	s.seen = append(s.seen, "read "+string(kind))
	f, ok := s.hardening[string(kind)]
	if !ok {
		return nil, fmt.Errorf("stubForge: no fixture for hardening kind %q", kind)
	}
	return f.raw, f.err
}

func (s *stubForge) ReadFile(repo deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error) {
	s.seen = append(s.seen, "read file "+in.File)
	f, ok := s.files[in.File]
	if !ok {
		return nil, fmt.Errorf("stubForge: no fixture for file %q", in.File)
	}
	return f.fc, f.err
}

func rawObj(s string) json.RawMessage { return json.RawMessage(s) }

// adminWorld is the world as a repository ADMIN sees it, fully hardened.
func adminWorld() *stubForge {
	return &stubForge{
		hardening: map[string]hardeningFixture{
			"repo": {raw: rawObj(`{"visibility":"public","security_and_analysis":{"secret_scanning":{"status":"enabled"}}}`)},
			"rulesets": {raw: rawObj(`[` +
				`{"id":1,"name":"protect-main","bypass_actors":[],"rules":[{"type":"pull_request"}]},` +
				`{"id":2,"name":"protect-release-tags","bypass_actors":[],"rules":[{"type":"update"},{"type":"deletion"}]}]`)},
		},
		files: map[string]fileFixture{
			"SECURITY.md": {fc: &deskkit.FileContent{Exists: true, Content: []byte("policy\n")}},
		},
	}
}

// nonAdminWorld is the SAME hardened repo read with a token that lacks admin:
// security_and_analysis reads null and the ruleset detail carries no
// bypass_actors key. This is the exact shape measured against medici-finance/assay.
func nonAdminWorld() *stubForge {
	w := adminWorld()
	w.hardening["repo"] = hardeningFixture{raw: rawObj(`{"visibility":"public","security_and_analysis":null}`)}
	w.hardening["rulesets"] = hardeningFixture{raw: rawObj(`[` +
		`{"id":1,"name":"protect-main","current_user_can_bypass":"never","rules":[{"type":"pull_request"}]},` +
		`{"id":2,"name":"protect-release-tags","current_user_can_bypass":"never","rules":[{"type":"update"},{"type":"deletion"}]}]`)}
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

// runGuard drives run() with forgeForFn stubbed onto w — no network, no minted token, no
// desktoken subprocess. It restores the real forgeForFn afterward so tests do not leak state.
func runGuard(t *testing.T, w *stubForge, path, repo string, extra ...string) (int, string, string) {
	t.Helper()
	origForgeFor := forgeForFn
	t.Cleanup(func() { forgeForFn = origForgeFor })
	kind := w.kind
	if kind == "" {
		kind = deskkit.ForgeGitHub
	}
	forgeForFn = func(r string) (deskkit.Forge, deskkit.ForgeRepo, deskkit.ForgeKind, error) {
		owner, name, _ := strings.Cut(r, "/")
		return w, deskkit.ForgeRepo{Owner: owner, Name: name}, kind, nil
	}
	var out, errb bytes.Buffer
	args := append([]string{"--repo", repo, "--checklist", path}, extra...)
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// wantIdentity is what production's identity() renders absent any deployment-specific
// AUDITOR_APP binding — computed the same way identity() itself does, so this assertion
// tracks the real wiring rather than a value hand-copied out of it.
func wantIdentity() string { return deskkit.AppBinding("auditor") + "[bot]" }

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
	if !strings.Contains(out, "identity: "+wantIdentity()) {
		t.Fatalf("output does not name the acting identity (want %s):\n%s", wantIdentity(), out)
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
		"| vis | o/r | visibility | public | `read repo` | visibility | public |",
		"| vis | o/r | visibility | public | `read repo` | visibility | private |", 1)
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
	w.hardening["rulesets"] = hardeningFixture{raw: rawObj(
		`[{"id":1,"name":"protect-main","bypass_actors":[{"actor_id":5,"actor_type":"Team","bypass_mode":"always"}]},` +
			`{"id":2,"name":"protect-release-tags","bypass_actors":[],"rules":[{"type":"update"},{"type":"deletion"}]}]`)}
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
	// produce a permission-wall-shaped error and muddy the two states that matter. Assert on
	// THIS world's call log, not a fresh one. The preflight's own `read repo` call is expected
	// (it always runs); a "read rulesets" beyond that would be the leak.
	for _, k := range w.seen {
		if k == "read rulesets" {
			t.Fatalf("the guard read a kind the checklist records as not available: %v", w.seen)
		}
	}
}

func TestAbsentFileOnPublicRow_IsWrongNotUnknown(t *testing.T) {
	w := adminWorld()
	w.files["SECURITY.md"] = fileFixture{err: &deskkit.ForgeAPIError{Status: 404, Method: "GET", Path: "repos/o/r/contents/SECURITY.md"}} // absent from a readable repo = real absence
	p := writeFixture(t, fixture)
	code, out, _ := runGuard(t, w, p, "o/r")
	if code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want %d\n%s", code, deskkit.ExitRefused, out)
	}
	if got := stateOf(t, out, "doc"); got != string(StateWrong) {
		t.Fatalf("missing SECURITY.md reported %q, want %q", got, StateWrong)
	}
}

func TestForbiddenIsCouldNotCheck(t *testing.T) {
	w := adminWorld()
	w.files["SECURITY.md"] = fileFixture{err: &deskkit.ForgeAPIError{Status: 403, Method: "GET", Path: "repos/o/r/contents/SECURITY.md"}}
	p := writeFixture(t, fixture)
	code, out, _ := runGuard(t, w, p, "o/r")
	if got := stateOf(t, out, "doc"); got != string(StateUnknown) {
		t.Fatalf("a 403 reported %q, want %q — a permission wall says nothing about the value behind it", got, StateUnknown)
	}
	if code == deskkit.ExitOK {
		t.Fatalf("exit 0 with a could-not-check row\n%s", out)
	}
}

// The three-state verdicts survive the seam: an admin-gated null is could-not-check, a
// forbidden read is could-not-check, and a not-found on a public row is absent-therefore-wrong
// — exactly what TestNonAdminReads_CouldNotCheckAndNonZeroExit,
// TestForbiddenIsCouldNotCheck and TestAbsentFileOnPublicRow_IsWrongNotUnknown each already
// pin per-scenario; these three focused tests are the names Verify row 6 dereferences.
func TestAdminNullIsCouldNotCheck(t *testing.T) {
	w := adminWorld()
	w.hardening["repo"] = hardeningFixture{raw: rawObj(`{"visibility":"public","security_and_analysis":{"secret_scanning":{"status":null}}}`)}
	p := writeFixture(t, fixture)
	code, out, _ := runGuard(t, w, p, "o/r")
	if got := stateOf(t, out, "scan"); got != string(StateUnknown) {
		t.Fatalf("a terminal null on an admin-gated row reported %q, want %q — null is not an answer", got, StateUnknown)
	}
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d\n%s", code, deskkit.ExitUnverifiable, out)
	}
}

func TestPublicNotFoundIsAbsent(t *testing.T) {
	w := adminWorld()
	w.files["SECURITY.md"] = fileFixture{err: &deskkit.ForgeAPIError{Status: 404, Method: "GET", Path: "repos/o/r/contents/SECURITY.md"}}
	p := writeFixture(t, fixture)
	code, out, _ := runGuard(t, w, p, "o/r")
	if got := stateOf(t, out, "doc"); got != string(StateWrong) {
		t.Fatalf("an absent file on a public row reported %q, want %q", got, StateWrong)
	}
	if code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want %d", code, deskkit.ExitRefused)
	}
}

func TestUnreadableRepo_ReportsNothing(t *testing.T) {
	w := adminWorld()
	w.hardening["repo"] = hardeningFixture{err: &deskkit.ForgeAPIError{Status: 404, Method: "GET", Path: "repos/o/r"}}
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
	w.hardening["rulesets"] = hardeningFixture{raw: rawObj(`[{"id":2,"name":"protect-release-tags","bypass_actors":[],"rules":[{"type":"update"},{"type":"deletion"}]}]`)}
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
		"| vis | o/r | visibility | public | `read repo` | visibility | public | n/a |\n" +
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

// The old grammar is refused BY NAME at parse time, naming the enumerated replacement — never
// silently reaching the fetcher as a malformed kind.
func TestChecklistRefusesGhApiCell(t *testing.T) {
	body := reposDeclared + rowsBegin +
		"\n| a | o/r | s | public | `gh api repos/o/r` | visibility | public | x |\n" + rowsEnd + "\n"
	p := writeFixture(t, body)
	_, _, errb := runGuard(t, adminWorld(), p, "o/r")
	if !strings.Contains(errb, "retired `gh api <endpoint>` form") {
		t.Fatalf("stderr does not name the retired grammar: %s", errb)
	}
	if !strings.Contains(errb, "repo") || !strings.Contains(errb, "rulesets") {
		t.Fatalf("stderr does not enumerate the replacement vocabulary: %s", errb)
	}
}

func TestChecklistParse_FailuresAreFatal(t *testing.T) {
	cases := map[string]string{
		"no markers":  "# just prose\n",
		"empty table": reposDeclared + rowsBegin + "\n" + rowsEnd + "\n",
		"short row":   reposDeclared + rowsBegin + "\n| a | b |\n" + rowsEnd + "\n",
		"bad gated":   reposDeclared + rowsBegin + "\n| a | o/r | s | sometimes | `read repo` | visibility | public | x |\n" + rowsEnd + "\n",
		"not a read cell": reposDeclared + rowsBegin +
			"\n| a | o/r | s | public | `curl https://api.github.com` | visibility | public | x |\n" + rowsEnd + "\n",
		"duplicate ids": reposDeclared + rowsBegin +
			"\n| a | o/r | s | public | `read repo` | visibility | public | x |\n| a | o/r | s | public | `read repo` | visibility | public | x |\n" + rowsEnd + "\n",
		"no field":       reposDeclared + rowsBegin + "\n| a | o/r | s | public | `read repo` | - | public | x |\n" + rowsEnd + "\n",
		"bad repo shape": reposDeclared + rowsBegin + "\n| a | justname | s | public | `read repo` | visibility | public | x |\n" + rowsEnd + "\n",

		// The repos directive itself is required and must be unambiguous.
		"no repos directive": rowsBegin + "\n| a | o/r | s | public | `read repo` | visibility | public | x |\n" + rowsEnd + "\n",
		"empty repos directive": "<!-- repohardenguard:repos: -->\n" + rowsBegin +
			"\n| a | o/r | s | public | `read repo` | visibility | public | x |\n" + rowsEnd + "\n",
		"two repos directives": reposDeclared + reposDeclared + rowsBegin +
			"\n| a | o/r | s | public | `read repo` | visibility | public | x |\n" + rowsEnd + "\n",
		"declared repo not owner/name": "<!-- repohardenguard:repos: justname -->\n" + rowsBegin +
			"\n| a | o/r | s | public | `read repo` | visibility | public | x |\n" + rowsEnd + "\n",
		"row repo not declared": "<!-- repohardenguard:repos: o/r -->\n" + rowsBegin +
			"\n| a | o/elsewhere | s | public | `read repo` | visibility | public | x |\n" + rowsEnd + "\n",
		"read file with no path": reposDeclared + rowsBegin +
			"\n| a | o/r | s | public | `read file` | - | present | x |\n" + rowsEnd + "\n",
		"read cell with trailing tokens": reposDeclared + rowsBegin +
			"\n| a | o/r | s | public | `read repo extra` | visibility | public | x |\n" + rowsEnd + "\n",
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
	w.hardening["repo"] = hardeningFixture{raw: rawObj(`{"visibility":"public","security_and_analysis":{"secret_scanning":{"status":null}}}`)}
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
	w.hardening["repo"] = hardeningFixture{raw: rawObj(`{"visibility":null,"security_and_analysis":{"secret_scanning":{"status":"enabled"}}}`)}
	p := writeFixture(t, fixture)
	code, out, _ := runGuard(t, w, p, "o/r")
	if got := stateOf(t, out, "vis"); got != string(StateWrong) {
		t.Fatalf("a null on a public row reported %q, want %q", got, StateWrong)
	}
	if code == deskkit.ExitOK {
		t.Fatalf("exit 0 with a null value\n%s", out)
	}
}

// The shipped checklist must stay parseable and cover both repos. A table that
// rots into an unparseable shape would make every run exit 6 for the wrong
// reason; a table that loses a repo would silently stop checking it.
func TestShippedChecklistParses(t *testing.T) {
	// Relative to tools/desk/cmd/repohardenguard, the package directory `go test`
	// uses as its working directory.
	p := filepath.Join("..", "..", "..", "..", defaultChecklist)
	// The shipped checklist is not part of every checkout of this repository. When
	// it is absent there is nothing to parse, so skip; where it is present this
	// parse-and-coverage check runs in full. Only os.ErrNotExist skips — a
	// checklist that exists but cannot be read still fails.
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		t.Skipf("%s not present in this tree", p)
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
