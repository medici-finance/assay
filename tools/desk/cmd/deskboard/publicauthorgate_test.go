package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// #808: the board's author bar is now the SAME predicate on every
// repo, private or public — `deskkit.TrustedAuthor`. The narrower #943 public-only
// bar (role App or mapped human only) is retired: a trusted SHARED automation login
// (here `shared-agent`, a configured trusted login but neither a role App nor a
// mapped human) is now REVIEWABLE on a public/risk-classed repo too.
//
// example-org/example-k8s is `:public` and example-org/tracker is `:private` in
// the fixture roster planted by installFakeGH.
const (
	pubRepoFixture  = "example-org/example-k8s"
	privRepoFixture = "example-org/tracker"
)

func publicAuthorGatePRJSON(author string) string {
	return `[{"number":7,"title":"a PR","isDraft":true,"author":{"login":"` + author +
		`"},"headRefOid":"abc123","mergeStateStatus":"CLEAN",` +
		`"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS"}]}]`
}

// quarantined reports whether PR #7 by author on repo lands in the EXTERNAL
// section (no ACTION row). Each call gets its own fake gh + fixture roster so the
// env is scoped to the subtest.
func publicAuthorGateQuarantined(t *testing.T, repo, author string) bool {
	t.Helper()
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", repo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", publicAuthorGatePRJSON(author))

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	for _, e := range rep.External {
		if e.Number == 7 && e.Author == author {
			return true
		}
	}
	return false
}

// TestTrustedSharedLoginIsReviewableOnPublicRepo — Verify row 2 (brief 17): a
// trusted shared automation login authoring a PR on a public/risk-classed repo is
// REVIEWABLE — it produces an ACTION row and does NOT appear in the
// EXTERNAL / UNBLESSED list. shared-agent ∈ ASSAY_TRUSTED_LOGINS but is neither a
// role App nor a mapped human — exactly the account the retired #943 bar refused.
func TestTrustedSharedLoginIsReviewableOnPublicRepo(t *testing.T) {
	if publicAuthorGateQuarantined(t, pubRepoFixture, "shared-agent") {
		t.Error("shared-agent on a PUBLIC repo must NOT be quarantined — one trust bar on every repo (#808)")
	}
}

// TestTrustedSharedLoginReviewableOnPrivateRepoUnchanged pins that private-repo
// behaviour, already correct before this brief, did not change: the plain
// TrustedAuthor bar still admits the shared trusted-login account there too.
func TestTrustedSharedLoginReviewableOnPrivateRepoUnchanged(t *testing.T) {
	if publicAuthorGateQuarantined(t, privRepoFixture, "shared-agent") {
		t.Error("shared-agent on a PRIVATE repo must NOT be quarantined — private behaviour is unchanged")
	}
}

// TestPublicRepoRoleAppStillReviewable pins that the case the OLD #943 bar already
// admitted — a role App authoring on a public repo — still does, unchanged by the
// widening.
func TestPublicRepoRoleAppStillReviewable(t *testing.T) {
	if publicAuthorGateQuarantined(t, pubRepoFixture, "app/assay-desk-app") {
		t.Error("a role-App-authored PR on a public repo must NOT be quarantined")
	}
}

// TestUnlistedAuthorStillQuarantined — Verify row 4, the NEGATIVE control: an
// unlisted login on a public repo is still EXTERNAL / UNBLESSED with no ACTION
// row, and is admitted only by a blessing. This is what proves the brief ALIGNED
// the bar rather than removing the quarantine gate outright.
func TestUnlistedAuthorStillQuarantined(t *testing.T) {
	if !publicAuthorGateQuarantined(t, pubRepoFixture, "some-fork-account") {
		t.Error("an arbitrary fork author on a public repo must still be quarantined — the gate is ALIGNED, not removed")
	}

	// Same unlisted author, but ada has reviewed (blessed) the PR: the manual
	// override still admits it, exactly as before this brief.
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PR_REPO", pubRepoFixture)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", publicAuthorGatePRJSON("some-fork-account"))
	t.Setenv("DESKBOARD_GH_GRAPHQL_JSON",
		gqlPRPayload("", nil, []string{gqlPRReview("ada", "User", 2001, "2026-07-21T10:00:00Z")}))

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	for _, e := range rep.External {
		if e.Number == 7 {
			t.Errorf("a blessed unlisted-author PR must not stay in EXTERNAL; got %+v", rep.External)
		}
	}
	found := false
	for _, r := range rep.Rows {
		if r.Number == 7 {
			found = true
		}
	}
	if !found {
		t.Errorf("a blessed unlisted-author PR must classify normally; got rows=%+v", rep.Rows)
	}
}
