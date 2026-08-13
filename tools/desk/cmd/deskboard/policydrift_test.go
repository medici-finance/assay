package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// publicRepos lists the allowed repos the compiled-in policy calls PUBLIC. Derived from
// the policy itself so the fixture cannot drift from it independently.
func publicRepos() []string {
	var out []string
	for _, r := range deskkit.AllowedRepos() {
		if deskkit.RepoVisibility(r) == deskkit.VisibilityPublic {
			out = append(out, r)
		}
	}
	return out
}

// TestPolicyDriftClean — the compiled-in table matching the world exits 0 and lists
// every allowed repo with its visibility and whether that visibility risk-classes it.
func TestPolicyDriftClean(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(publicRepos(), " "))

	var out, errb bytes.Buffer
	if code := run([]string{"policydrift"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("policydrift on a truthful API = exit %d, want 0; stderr=%s", code, errb.String())
	}
	var rep struct {
		Repos []struct {
			Repo       string `json:"repo"`
			CompiledIn string `json:"compiledIn"`
			Observed   string `json:"observed"`
			RiskClass  bool   `json:"riskClassedByVisibility"`
		} `json:"repos"`
		Drift []string `json:"drift"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("policydrift JSON: %v (%s)", err, out.String())
	}
	if len(rep.Drift) != 0 {
		t.Fatalf("drift = %v, want none", rep.Drift)
	}
	if len(rep.Repos) != len(deskkit.AllowedRepos()) {
		t.Fatalf("policydrift reported %d repos, want %d — a repo it cannot see is a repo it cannot check",
			len(rep.Repos), len(deskkit.AllowedRepos()))
	}
	for _, r := range rep.Repos {
		if r.CompiledIn != r.Observed {
			t.Fatalf("%s: compiled-in %q vs observed %q slipped past the drift check", r.Repo, r.CompiledIn, r.Observed)
		}
		if (r.CompiledIn != "private") != r.RiskClass {
			t.Fatalf("%s: visibility %q but riskClassedByVisibility=%t", r.Repo, r.CompiledIn, r.RiskClass)
		}
	}
}

// TestPolicyDriftDetectsFlip — the drift anti-pattern scenario: a repo the table calls public has been
// made private (or vice versa) and nobody updated the table. The check must FAIL LOUD.
func TestPolicyDriftDetectsFlip(t *testing.T) {
	installFakeGH(t)
	// Every repo answers "private" — so the compiled-in PUBLIC repos are now drift.
	t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", "")

	var out, errb bytes.Buffer
	code := run([]string{"policydrift"}, &out, &errb)
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("policydrift on a stale table = exit %d, want %d (fail closed)", code, deskkit.ExitUnverifiable)
	}
	msg := errb.String()
	if !strings.Contains(msg, "DRIFT") {
		t.Fatalf("drift must be reported loudly; stderr = %q", msg)
	}
	for _, r := range publicRepos() {
		if !strings.Contains(msg, r) {
			t.Fatalf("drift message does not name %s; stderr = %q", r, msg)
		}
	}
}

// TestPolicyDriftRejectsUnparseableVisibility — an API value this code does not
// understand ("internal", "") is not silently mapped to anything; it is drift.
func TestPolicyDriftRejectsUnparseableVisibility(t *testing.T) {
	for _, body := range []string{
		`{"visibility":"internal","private":true}`,
		`{"visibility":"","private":true}`,
		`{}`,
		`{"visibility":"public","private":true}`, // API self-contradiction
	} {
		t.Run(body, func(t *testing.T) {
			installFakeGH(t)
			t.Setenv("DESKBOARD_GH_REPOMETA_OVERRIDE", body)

			var out, errb bytes.Buffer
			if code := run([]string{"policydrift"}, &out, &errb); code != deskkit.ExitUnverifiable {
				t.Fatalf("policydrift on %s = exit %d, want %d", body, code, deskkit.ExitUnverifiable)
			}
		})
	}
}

// TestPolicyDriftFailsClosedOnUnreadableRepo — a repo whose metadata could not be read
// fails the WHOLE run. A check that skips what it cannot see verifies nothing.
func TestPolicyDriftFailsClosedOnUnreadableRepo(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(publicRepos(), " "))
	t.Setenv("DESKBOARD_GH_FAIL_REPO", "repos/example-org/example-k8s")

	var out, errb bytes.Buffer
	if code := run([]string{"policydrift"}, &out, &errb); code != deskkit.ExitUnverifiable {
		t.Fatalf("policydrift with an unreadable repo = exit %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(errb.String(), "example-org/example-k8s") {
		t.Fatalf("failure must name the repo; stderr = %q", errb.String())
	}
}

// TestActions_CarriesPolicyDriftClean — an internal hardening review: the
// the public-repo risk rule visibility-drift check now rides `actions` the same way mainHealth rides it
// (#295), so an ordinary board read carries the signal without anyone typing
// `deskboard policydrift` separately.
func TestActions_CarriesPolicyDriftClean(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(publicRepos(), " "))

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	if rep.Header.PolicyDrift == nil {
		t.Fatal("actions header must carry policyDrift (an internal hardening review)")
	}
	if len(rep.Header.PolicyDrift.Drift) != 0 {
		t.Errorf("policyDrift.drift = %v, want none on a truthful table", rep.Header.PolicyDrift.Drift)
	}
	if len(rep.Header.PolicyDrift.Scope) != len(deskkit.AllowedRepos()) {
		t.Errorf("policyDrift scope = %d, want %d", len(rep.Header.PolicyDrift.Scope), len(deskkit.AllowedRepos()))
	}
}

// TestActions_PolicyDriftFlipAlarmsWithoutFailingTheBoard is the positive control the
// internal review demanded for finding 1: a repo whose real visibility no longer matches the
// compiled-in table must show up as drift on the very next `actions` read — and, unlike
// the standalone fail-closed gate, must NOT take the whole PR sweep down with it (same
// non-fatal contract mainHealth already established for #295).
func TestActions_PolicyDriftFlipAlarmsWithoutFailingTheBoard(t *testing.T) {
	installFakeGH(t)
	// Every repo answers "private" — so every compiled-in PUBLIC repo is now drift.
	t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", "")

	var out, errb bytes.Buffer
	if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("a policy-visibility flip must not fail actions; exit %d, stderr=%s", code, errb.String())
	}
	var rep actionsReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
	}
	if rep.Header.PolicyDrift == nil || len(rep.Header.PolicyDrift.Drift) == 0 {
		t.Fatalf("expected drift entries in the actions header; got %+v", rep.Header.PolicyDrift)
	}
	for _, r := range publicRepos() {
		found := false
		for _, d := range rep.Header.PolicyDrift.Drift {
			if strings.Contains(d, r) {
				found = true
			}
		}
		if !found {
			t.Errorf("drift list does not name %s; got %v", r, rep.Header.PolicyDrift.Drift)
		}
	}

	// Table path: the alarm line must be visible to a human reading the board.
	out.Reset()
	errb.Reset()
	if code := run([]string{"actions", "--table"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("run(actions --table) = exit %d", code)
	}
	if !strings.Contains(out.String(), "POLICY-DRIFT:") {
		t.Fatalf("table output must carry a POLICY-DRIFT line; got:\n%s", out.String())
	}
}

// TestPolicyDriftAlarm_AbsentOnNonActionsVerbs — an ABSENT policyDrift field means "this
// verb did not probe" (same three-state discipline #295 established for mainHealth). The
// standalone `policydrift` verb carries its own top-level report and must not ALSO stamp
// the Header's policyDrift rider — two keys with the same meaning is exactly the
// ambiguity the mainHealth N2 finding (review) rejected.
func TestPolicyDriftAlarm_AbsentOnNonActionsVerbs(t *testing.T) {
	for _, verb := range []string{"prs", "queue", "policydrift"} {
		t.Run(verb, func(t *testing.T) {
			installFakeGH(t)
			t.Setenv("DESKBOARD_GH_PUBLIC_REPOS", strings.Join(publicRepos(), " "))
			var out, errb bytes.Buffer
			if code := run([]string{verb}, &out, &errb); code != deskkit.ExitOK {
				t.Fatalf("run(%s) = exit %d, stderr=%s", verb, code, errb.String())
			}
			if strings.Contains(out.String(), `"policyDrift"`) {
				t.Errorf("%s must not carry the policyDrift rider — it never probed; got:\n%s", verb, out.String())
			}
		})
	}
}

// TestAnyRiskPathIsRepoAware — deskboard used to carry its OWN copy of the trigger list
// (the drift anti-pattern shape riskpath.go's doc comment already forbade). It now delegates, so the
// board and deskpost's ready gate cannot disagree about whether a PR is risk-classed.
func TestAnyRiskPathIsRepoAware(t *testing.T) {
	cases := []struct {
		repo  string
		files map[string]bool
		want  bool
	}{
		{"example-org/tracker", map[string]bool{"secrets/x.yaml": true}, true},
		{"example-org/tracker", map[string]bool{"k8s/dev/rbac.yaml": true}, true},
		{"example-org/tracker", map[string]bool{"README.md": true}, false},
		// the public-repo risk rule: the public infra repo is risk-classed; under the old copy it never was.
		{"example-org/example-k8s", map[string]bool{"base/ledger/identity.yaml": true}, true},
		{"example-org/example-k8s", map[string]bool{"README.md": true}, true},
		// fail-closed inputs
		{"example-org/example-k8s", nil, true},
		{"example-org/tracker", nil, true},
		{"attacker/whatever", map[string]bool{"README.md": true}, true},
	}
	for _, c := range cases {
		if got := anyRiskPath(c.repo, c.files); got != c.want {
			t.Errorf("anyRiskPath(%q, %v) = %v, want %v", c.repo, c.files, got, c.want)
		}
	}
}
