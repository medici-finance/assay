package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// gitlab_test.go — the GitLab fixture block (the forge-gitlab GitLab-hardening-reads brief).
//
// Same guard, same three-state rule, a GitLab-resolved project: the preflight reads `project`
// (never `repo`), a Free row is checked against the forge's own document, a
// `not available — Premium` row makes NO request at all, and a tier-gated 403/404 is
// could-not-check — never a value, never a pass. Test names carry underscores for the same
// BodyCheck reason repohardenguard_test.go explains.

const glReposDeclared = "<!-- repohardenguard:repos: gl/p -->\n"

// glFixture is the Community-Edition checklist shape the adopter doc's template carries: Free
// rows on the project / protected-branch / protected-tag documents, the approvals read on an
// admin-gated row (some self-managed CE answers 404 there), the push-rules row recorded as a
// divergence, and an Owner-visible project field on an admin-gated row.
const glFixture = "" +
	"# gitlab fixture\n" +
	glReposDeclared +
	rowsBegin + "\n" +
	"| ID | Repo | Setting | Gated | Read | Field | Required | Set |\n" +
	"|---|---|---|---|---|---|---|---|\n" +
	"| vis | gl/p | visibility | public | `read project` | visibility | private | project settings |\n" +
	"| mr-pipeline | gl/p | pipeline must succeed before merge (B5) | public | `read project` | only_allow_merge_if_pipeline_succeeds | true | project settings |\n" +
	"| main-no-force | gl/p | main: force push closed (B1) | public | `read protected-branches` | [name=main].allow_force_push | false | protected branches |\n" +
	"| main-push-no-one | gl/p | main: push = No one (B2, role-level on CE) | public | `read protected-branches` | [name=main].push_access_levels.0.access_level | 0 | protected branches |\n" +
	"| release-tags | gl/p | v* tags: create = Maintainers (B12) | public | `read protected-tags` | [name=v*].create_access_levels.0.access_level | 40 | protected tags |\n" +
	"| author-approval | gl/p | author cannot approve (B4, advisory on CE) | admin | `read approvals` | merge_requests_author_approval | false | approval settings |\n" +
	"| push-rules | gl/p | signed commits required (C5) | admin | `read push-rules` | reject_unsigned_commits | not available — Premium | n/a on CE |\n" +
	"| fork-pipelines | gl/p | fork pipelines cannot run in the parent (Owner-visible) | admin | `read project` | ci_allow_fork_pipelines_to_run_in_parent_project | false | project CI settings |\n" +
	"| doc | gl/p | SECURITY.md present | public | `read file SECURITY.md` | - | present | copy |\n" +
	rowsEnd + "\n"

// glPremiumRow is the push-rules row as a PREMIUM checklist writes it: a real requirement on
// an admin-gated row (a 403/404 there is a tier wall, not "unsigned commits allowed").
const glPremiumRow = "| push-rules | gl/p | signed commits required (C5) | admin | `read push-rules` | reject_unsigned_commits | true | push rules |\n"

// gitlabWorld is a hardened Community-Edition project read by an auditor whose role can see
// the protected-branch/tag lists, the approvals document and the Owner-visible
// `ci_allow_fork_pipelines_to_run_in_parent_project` field (the Owner-only-field test drops
// that field to model a lower role). It carries NO push-rules document: CE has none.
func gitlabWorld() *stubForge {
	return &stubForge{
		kind: deskkit.ForgeGitLab,
		hardening: map[string]hardeningFixture{
			"project": {raw: rawObj(`{"id":12,"visibility":"private","only_allow_merge_if_pipeline_succeeds":true,` +
				`"only_allow_merge_if_all_discussions_are_resolved":true,"ci_allow_fork_pipelines_to_run_in_parent_project":false}`)},
			"protected-branches": {raw: rawObj(`[{"id":1,"name":"main","allow_force_push":false,` +
				`"push_access_levels":[{"access_level":0,"access_level_description":"No one"}],` +
				`"merge_access_levels":[{"access_level":40,"access_level_description":"Maintainers"}]}]`)},
			"protected-tags": {raw: rawObj(`[{"name":"v*","create_access_levels":[{"access_level":40,"access_level_description":"Maintainers"}]}]`)},
			"approvals": {raw: rawObj(`{"approvals_before_merge":0,"reset_approvals_on_push":true,` +
				`"merge_requests_author_approval":false,"merge_requests_disable_committers_approval":true}`)},
		},
		files: map[string]fileFixture{
			"SECURITY.md": {fc: &deskkit.FileContent{Exists: true, Content: []byte("policy\n")}},
		},
	}
}

func premiumFixture(t *testing.T) string {
	t.Helper()
	mutated := strings.Replace(glFixture,
		"| push-rules | gl/p | signed commits required (C5) | admin | `read push-rules` | reject_unsigned_commits | not available — Premium | n/a on CE |\n",
		glPremiumRow, 1)
	if mutated == glFixture {
		t.Fatal("fixture mutation did not apply — the test would prove nothing")
	}
	return mutated
}

// A GitLab-resolved run preflights on `project`, never on `repo`, and every Free row is
// judged against the forge's own document — including the `[name=main]` selector over the
// protected-branches list and the `.0.access_level` index into its access-level entries.
func TestGitLabRun_FreeRowsGreen(t *testing.T) {
	w := gitlabWorld()
	p := writeFixture(t, glFixture)
	code, out, errb := runGuard(t, w, p, "gl/p")
	if code != deskkit.ExitOK {
		t.Fatalf("exit %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out, errb)
	}
	if len(w.seen) == 0 || w.seen[0] != "read project" {
		t.Fatalf("the preflight must read the GitLab project document first, got call log %v", w.seen)
	}
	for _, k := range w.seen {
		if k == "read repo" {
			t.Fatalf("a GitLab run asked for the GitHub `repo` kind: %v", w.seen)
		}
	}
	for _, id := range []string{"vis", "mr-pipeline", "main-no-force", "main-push-no-one", "release-tags", "author-approval", "fork-pipelines", "doc"} {
		if got := stateOf(t, out, id); got != string(StateOK) {
			t.Errorf("row %q reported %q, want %q", id, got, StateOK)
		}
	}
	if strings.Contains(out, string(StateUnknown)) {
		t.Fatalf("a green run mentions %q:\n%s", StateUnknown, out)
	}
}

// A `not available — Premium` row issues ZERO reads: the guard records the divergence and
// never asks, because on CE the answer would be a permission-wall-shaped error that muddies
// the two states that matter (Verify row 3, first half).
func TestGitLabNotAvailableNoRequest(t *testing.T) {
	w := gitlabWorld()
	p := writeFixture(t, glFixture)
	code, out, _ := runGuard(t, w, p, "gl/p")
	if code != deskkit.ExitOK {
		t.Fatalf("exit %d, want 0 — a recorded divergence is not a failure\n%s", code, out)
	}
	if got := stateOf(t, out, "push-rules"); got != string(StateNotAvailable) {
		t.Fatalf("push-rules row reported %q, want %q", got, StateNotAvailable)
	}
	for _, k := range w.seen {
		if k == "read push-rules" {
			t.Fatalf("the guard read a kind the checklist records as not available: %v", w.seen)
		}
	}
	if _, fixtureHasIt := w.hardening["push-rules"]; fixtureHasIt {
		t.Fatal("the world must carry NO push-rules document, so a read would have failed loudly rather than passed")
	}
}

// A tier-gated 403 or 404 on a Premium row is could-not-check — never a value, never a pass,
// never "absent therefore wrong" (Verify row 3, second half). Both statuses are exercised:
// CE has no push-rule route at all (404), and a locked-down instance answers 403.
func TestGitLabTierGateCouldNotCheck(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{
		{"forbidden_403", 403},
		{"not_found_404_ce_has_no_route", 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := gitlabWorld()
			w.hardening["push-rules"] = hardeningFixture{err: deskkit.Unverifiable(
				"could-not-check: GET /projects/gl%2Fp/push_rule — permission or tier gate",
				&deskkit.ForgeAPIError{Status: tc.status, Method: "GET", Path: "/projects/gl%2Fp/push_rule"})}
			p := writeFixture(t, premiumFixture(t))
			code, out, _ := runGuard(t, w, p, "gl/p")
			if got := stateOf(t, out, "push-rules"); got != string(StateUnknown) {
				t.Fatalf("a tier-gated %d reported %q, want %q — a tier wall says nothing about the value behind it\n%s",
					tc.status, got, StateUnknown, out)
			}
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("exit %d, want %d (could-not-check is not a pass)\n%s", code, deskkit.ExitUnverifiable, out)
			}
			if strings.Contains(out, "checked-ok        push-rules") {
				t.Fatalf("a tier-gated row rendered as a pass:\n%s", out)
			}
		})
	}
	// And a Premium project with NO push rule answers the literal `null` document: on an
	// admin-gated row that absence is could-not-check, not a satisfied requirement.
	t.Run("premium_null_document_is_not_a_value", func(t *testing.T) {
		w := gitlabWorld()
		w.hardening["push-rules"] = hardeningFixture{raw: rawObj(`null`)}
		p := writeFixture(t, premiumFixture(t))
		code, out, _ := runGuard(t, w, p, "gl/p")
		if got := stateOf(t, out, "push-rules"); got != string(StateUnknown) {
			t.Fatalf("a null push-rule document reported %q, want %q", got, StateUnknown)
		}
		if code == deskkit.ExitOK {
			t.Fatalf("exit 0 with a could-not-check row\n%s", out)
		}
	})
	// The positive control: a Premium project WITH the rule is checked-ok through the same row.
	t.Run("premium_value_is_checked", func(t *testing.T) {
		w := gitlabWorld()
		w.hardening["push-rules"] = hardeningFixture{raw: rawObj(`{"id":3,"reject_unsigned_commits":true,"prevent_secrets":true}`)}
		p := writeFixture(t, premiumFixture(t))
		code, out, _ := runGuard(t, w, p, "gl/p")
		if got := stateOf(t, out, "push-rules"); got != string(StateOK) {
			t.Fatalf("a Premium push-rule document reported %q, want %q\n%s", got, StateOK, out)
		}
		if code != deskkit.ExitOK {
			t.Fatalf("exit %d, want 0\n%s", code, out)
		}
	})
}

// The `.0.access_level` index selector really compares: a role-level entry other than
// "No one" (Maintainers, 40) on the push list is checked-wrong, exit 5.
func TestGitLabPushAccessLevel_IndexSelectorFlips(t *testing.T) {
	w := gitlabWorld()
	w.hardening["protected-branches"] = hardeningFixture{raw: rawObj(`[{"id":1,"name":"main","allow_force_push":false,` +
		`"push_access_levels":[{"access_level":40,"access_level_description":"Maintainers"}],` +
		`"merge_access_levels":[{"access_level":40,"access_level_description":"Maintainers"}]}]`)}
	p := writeFixture(t, glFixture)
	code, out, _ := runGuard(t, w, p, "gl/p")
	if got := stateOf(t, out, "main-push-no-one"); got != string(StateWrong) {
		t.Fatalf("push = Maintainers reported %q, want %q\n%s", got, StateWrong, out)
	}
	if code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want %d\n%s", code, deskkit.ExitRefused, out)
	}
	// An unprotected main (no entry named main in the list) is a real absence: wrong, not unknown.
	w.hardening["protected-branches"] = hardeningFixture{raw: rawObj(`[]`)}
	code, out, _ = runGuard(t, w, p, "gl/p")
	if got := stateOf(t, out, "main-no-force"); got != string(StateWrong) {
		t.Fatalf("an unprotected main reported %q, want %q\n%s", got, StateWrong, out)
	}
	if code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want %d\n%s", code, deskkit.ExitRefused, out)
	}
}

// An Owner/admin-visible project field (`ci_allow_fork_pipelines_to_run_in_parent_project`)
// is simply ABSENT from the document at a lower role — on its admin-gated row that is
// could-not-check, never "absent therefore off".
func TestGitLabOwnerOnlyField_AbsentIsCouldNotCheck(t *testing.T) {
	w := gitlabWorld()
	w.hardening["project"] = hardeningFixture{raw: rawObj(`{"id":12,"visibility":"private",` +
		`"only_allow_merge_if_pipeline_succeeds":true,"only_allow_merge_if_all_discussions_are_resolved":true}`)}
	p := writeFixture(t, glFixture)
	code, out, _ := runGuard(t, w, p, "gl/p")
	if got := stateOf(t, out, "fork-pipelines"); got != string(StateUnknown) {
		t.Fatalf("an Owner-only field absent at Maintainer reported %q, want %q\n%s", got, StateUnknown, out)
	}
	if got := stateOf(t, out, "vis"); got != string(StateOK) {
		t.Fatalf("the Free row beside it reported %q, want %q", got, StateOK)
	}
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d\n%s", code, deskkit.ExitUnverifiable, out)
	}
}

// A GitLab project the auditor cannot read at all is refused at the preflight — on the
// `project` document — and no row is reported.
func TestGitLabUnreadableProject_ReportsNothing(t *testing.T) {
	w := gitlabWorld()
	w.hardening["project"] = hardeningFixture{err: deskkit.Unverifiable("could-not-check: GET /projects/gl%2Fp — not visible",
		&deskkit.ForgeAPIError{Status: 404, Method: "GET", Path: "/projects/gl%2Fp"})}
	p := writeFixture(t, glFixture)
	code, out, errb := runGuard(t, w, p, "gl/p")
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if strings.Contains(out, string(StateWrong)) || strings.Contains(out, string(StateOK)) {
		t.Fatalf("a project the token cannot read produced per-row verdicts:\n%s", out)
	}
	if !strings.Contains(errb, "cannot read gl/p") {
		t.Fatalf("stderr does not say why nothing was checked: %s", errb)
	}
	if len(w.seen) != 1 || w.seen[0] != "read project" {
		t.Fatalf("expected exactly the preflight read, got %v", w.seen)
	}
}

// The adopter doc's Community Edition template is a real checklist: it parses under the
// guard's own grammar, carries the divergence rows as `not available — <tier>`, and names
// only GitLab kinds. A template that drifted from the parser would send an adopter a
// parse-time refusal instead of a verdict. Skips only when the doc is absent from the tree.
func TestGitLabAdopterTemplateParses(t *testing.T) {
	p := filepath.Join("..", "..", "..", "..", "docs", "adopting-assay-gitlab.md")
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		t.Skipf("%s not present in this tree", p)
	}
	cl, err := ParseChecklistFile(p)
	if err != nil {
		t.Fatalf("the GitLab adopter template does not parse: %v", err)
	}
	var notAvailable, pushRules int
	for _, r := range cl.Rows {
		parsed, perr := r.ParseRead()
		if perr != nil {
			t.Fatalf("row %q: %v", r.ID, perr)
		}
		if parsed.Kind != "" {
			k, verr := deskkit.ValidateHardeningReadKind(parsed.Kind)
			if verr != nil {
				t.Errorf("row %q names an unknown kind %q: %v", r.ID, parsed.Kind, verr)
			} else if deskkit.HardeningReadKindForge(k) != deskkit.ForgeGitLab {
				t.Errorf("row %q names %q, which is not a gitlab kind", r.ID, parsed.Kind)
			}
			if parsed.Kind == "push-rules" {
				pushRules++
				if !strings.EqualFold(r.Gated, "admin") {
					t.Errorf("row %q: a tier-gated push-rules row must be admin-gated, got %q", r.ID, r.Gated)
				}
			}
		}
		if r.notAvailable() {
			notAvailable++
		}
	}
	if pushRules == 0 {
		t.Error("the template carries no push-rules row")
	}
	if notAvailable == 0 {
		t.Error("the template records no `not available — <tier>` divergence row")
	}
	t.Logf("template: %d rows, %d not-available, %d push-rules", len(cl.Rows), notAvailable, pushRules)
}
