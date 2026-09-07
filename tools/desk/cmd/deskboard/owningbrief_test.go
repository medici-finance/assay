package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// owningbrief_test.go — the board resolves a PR's owning brief from its body's `Brief:`
// trailer (not from the branch name) and consults the brief's OWN gate/risk frontmatter as
// a risk term. This is the false-negative the fix closes: a `gate: human` / `risk: yes`
// brief whose PR touches no compiled trigger path used to render owningBrief="" and
// risk=false, so the board marked it FLIP with no security gate.

// actionsWithBrief runs `actions` over one approved-at-head draft PR carrying the given
// body, with a brief planted under DESK_ROOTS, and returns PR #42's row. The changed-file
// fixture is a single clean README so NO path trigger fires — the only thing that can
// risk-class the row is the brief term.
func actionsWithBrief(t *testing.T, body, stream, nn, briefContent string) actionRow {
	t.Helper()
	head := "deadbeefcafe"
	installFakeGH(t)

	// Plant the brief under a temp root and point DESK_ROOTS at it for cfRepo.
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", stream)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief-"+nn+"-thing.md"), []byte(briefContent), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(deskkit.RootsEnv, cfRepo+"="+root)

	t.Setenv("DESKBOARD_GH_PR_REPO", cfRepo)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":42,"title":"t","body":`+jsonStr(body)+`,"isDraft":true,"author":{"login":"app/assay-worker-app"},"headRefOid":"`+head+`","headRefName":"feat/no-brief-in-branch","mergeStateStatus":"CLEAN","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS","name":"ci"}]}]`)
	t.Setenv("DESKBOARD_GH_REVIEWS_JSON",
		`[{"user":{"login":"`+reviewerBotDisplay()+`"},"state":"APPROVED","commit_id":"`+head+`","body":`+jsonStr("looks good")+`,"submitted_at":"2026-07-10T00:00:00Z"}]`)
	t.Setenv("DESKBOARD_GH_PRFILES_JSON", `[{"filename":"README.md"}]`)
	t.Setenv("DESKBOARD_GH_PRMETA_JSON", `{"changed_files":1}`)

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	for _, r := range rep.Rows {
		if r.Number == 42 {
			return r
		}
	}
	t.Fatal("PR #42 missing from the actions report")
	return actionRow{}
}

const gateHumanBrief = "---\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}\n---\nbody"
const nonRiskBrief = "---\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\nbody"

// TestOwningBriefResolvesFromTrailerSlashForm — the slash form `Brief: <stream>/<NN>`
// resolves owningBrief non-empty (the branch name carries no brief pattern), and a
// gate:human brief risk-classes the row into SEC-REVIEW-REQUIRED.
func TestOwningBriefResolvesFromTrailerSlashForm(t *testing.T) {
	row := actionsWithBrief(t, "Delivers the change.\n\nBrief: example-stream/15\n", "example-stream", "15", gateHumanBrief)
	if row.OwningBrief != "example-stream/15" {
		t.Fatalf("OwningBrief = %q, want example-stream/15 (resolved from the trailer, not the branch)", row.OwningBrief)
	}
	if !row.RiskClassed {
		t.Fatal("a gate:human brief PR must be risk-classed even with a clean diff")
	}
	if row.Action != actSecReview {
		t.Fatalf("action = %s, want %s — a risk-classed PR with no Security-Review: pass", row.Action, actSecReview)
	}
}

// TestOwningBriefResolvesFromTrailerColonForm — the colon form `Brief: <stream>:<NN>`
// resolves the same brief and risk-classes identically.
func TestOwningBriefResolvesFromTrailerColonForm(t *testing.T) {
	row := actionsWithBrief(t, "Brief: example-stream:15\n", "example-stream", "15", gateHumanBrief)
	if row.OwningBrief != "example-stream/15" {
		t.Fatalf("OwningBrief = %q, want example-stream/15 (colon trailer form)", row.OwningBrief)
	}
	if !row.RiskClassed || row.Action != actSecReview {
		t.Fatalf("colon-form brief PR: risk=%v action=%s, want risk=true action=%s",
			row.RiskClassed, row.Action, actSecReview)
	}
}

// TestOwningBriefResolvesButNonRiskBriefDoesNotClassify — the control: the trailer still
// resolves owningBrief, but a non-risk brief does not risk-class (the brief term only
// widens; it does not brick every brief-carrying PR).
func TestOwningBriefResolvesButNonRiskBriefDoesNotClassify(t *testing.T) {
	row := actionsWithBrief(t, "Brief: example-stream/15\n", "example-stream", "15", nonRiskBrief)
	if row.OwningBrief != "example-stream/15" {
		t.Fatalf("OwningBrief = %q, want example-stream/15", row.OwningBrief)
	}
	if row.RiskClassed {
		t.Fatal("a non-risk brief must not risk-class the row")
	}
}
