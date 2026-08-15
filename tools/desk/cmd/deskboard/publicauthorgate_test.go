package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestTrustGate_PublicRepoAuthorGate (#943): on a PUBLIC repo the review-desk
// auto-reviews a PR only if its author is a role App or a mapped human. The
// shared machine account (here `shared-agent`) is a configured trusted LOGIN — so
// the plain TrustedAuthor gate admits it — but it must NOT clear the public bar,
// and neither must a fork author. On a PRIVATE repo the behaviour is unchanged
// (TrustedAuthor still admits the shared account).
//
// example-org/example-k8s is `:public` and example-org/tracker is `:private` in
// the fixture roster planted by installFakeGH.
func TestTrustGate_PublicRepoAuthorGate(t *testing.T) {
	const pubRepo = "example-org/example-k8s"
	const privRepo = "example-org/tracker"

	prJSON := func(author string) string {
		return `[{"number":7,"title":"a PR","isDraft":true,"author":{"login":"` + author +
			`"},"headRefOid":"abc123","mergeStateStatus":"CLEAN",` +
			`"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS"}]}]`
	}

	// quarantined reports whether PR #7 by author on repo lands in the EXTERNAL
	// section (no ACTION row). Each call gets its own fake gh + fixture roster so
	// the env is scoped to the subtest.
	quarantined := func(t *testing.T, repo, author string) bool {
		t.Helper()
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PR_REPO", repo)
		t.Setenv("DESKBOARD_GH_PRLIST_JSON", prJSON(author))

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

	t.Run("public repo: role App author is reviewed", func(t *testing.T) {
		if quarantined(t, pubRepo, "app/assay-desk-app") {
			t.Error("a role-App-authored PR on a public repo must NOT be quarantined")
		}
	})

	t.Run("public repo: shared machine account is quarantined", func(t *testing.T) {
		// shared-agent ∈ ASSAY_TRUSTED_LOGINS, so TrustedAuthor(shared-agent)=true —
		// yet it is neither a role App nor a mapped human, so the public gate rejects
		// it. This is the exact hole #943 closes (a shared push account in deploy).
		if !quarantined(t, pubRepo, "shared-agent") {
			t.Error("shared-agent on a PUBLIC repo must be quarantined — the #943 author gate")
		}
	})

	t.Run("public repo: fork author is quarantined", func(t *testing.T) {
		if !quarantined(t, pubRepo, "some-fork-account") {
			t.Error("an arbitrary fork author on a public repo must be quarantined")
		}
	})

	t.Run("PRIVATE repo: shared machine account is unchanged", func(t *testing.T) {
		// Private-repo behaviour is unchanged: the plain TrustedAuthor bar still
		// admits the shared trusted-login account, so it is NOT quarantined.
		if quarantined(t, privRepo, "shared-agent") {
			t.Error("shared-agent on a PRIVATE repo must NOT be quarantined — private behaviour is unchanged")
		}
	})
}
