package forgelabels

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plantRoster writes body as the roster of a throwaway HOME and reloads deskkit's write-class
// config from it — the same file `deskfile` reads, so the vocabulary under test is the one
// `deskfile --to` resolves against.
func plantRoster(t *testing.T, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.SetToolClass(deskkit.ClassWrite)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

// fixtureRoster binds the six documented roles plus one more, so the derivation is shown
// to follow the roster rather than the documented list.
const fixtureRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=desk=example-desk-app:300000001,intake-loop=example-intake-loop-app:300000002,issue-loop=example-issue-loop-app:300000003,reviewer=example-reviewer-app:300000004,verifier=example-verifier-app:300000005,worker=example-worker-app:300000006,triage=example-triage-app:300000007
ASSAY_ALLOWED_REPOS=example-org/one:ci:private
`

// regression: #6127
// TestToRowsAreTheDeskfileToVocabulary is the parity the issue asks for: for a roster, the
// set built from that roster's bound roles carries exactly one `to:<role>` row per role
// `deskfile --to` accepts — each spelled as deskkit.AddressedToLabel spells it — and one
// `raised-by:<role>` row per role `--raised-by` accepts. A role bound in the roster but
// absent from the documented list (`triage` here) still gets both rows.
func TestToRowsAreTheDeskfileToVocabulary(t *testing.T) {
	plantRoster(t, fixtureRoster)
	roles := deskkit.RaisedByRoles()
	if len(roles) != 7 {
		t.Fatalf("fixture roster binds %d roles (%v), want 7", len(roles), roles)
	}
	set := Canonical(roles)
	var to, raised []string
	for _, l := range set {
		switch {
		case strings.HasPrefix(l.Name, deskkit.AddressedToPrefix):
			to = append(to, l.Name)
		case strings.HasPrefix(l.Name, deskkit.RaisedByPrefix):
			raised = append(raised, l.Name)
		}
	}
	var wantTo, wantRaised []string
	for _, r := range roles {
		tl, err := deskkit.AddressedToLabel(r)
		if err != nil {
			t.Fatalf("AddressedToLabel(%q): %v", r, err)
		}
		rl, err := deskkit.RaisedByLabel(r)
		if err != nil {
			t.Fatalf("RaisedByLabel(%q): %v", r, err)
		}
		wantTo, wantRaised = append(wantTo, tl), append(wantRaised, rl)
	}
	if strings.Join(to, ",") != strings.Join(wantTo, ",") {
		t.Errorf("to: rows = %v\nwant the --to vocabulary %v", to, wantTo)
	}
	if strings.Join(raised, ",") != strings.Join(wantRaised, ",") {
		t.Errorf("raised-by: rows = %v\nwant the --raised-by vocabulary %v", raised, wantRaised)
	}
}

// TestBoundRolesIsRaisedByRoles pins the one projection: the method cellctl calls on a cell's
// loaded roster answers exactly what RaisedByRoles answers over the effective config.
func TestBoundRolesIsRaisedByRoles(t *testing.T) {
	plantRoster(t, fixtureRoster)
	a, b := deskkit.RaisedByRoles(), deskkit.EffectiveConfig().BoundRoles()
	if strings.Join(a, ",") != strings.Join(b, ",") {
		t.Errorf("RaisedByRoles() = %v, Config.BoundRoles() = %v", a, b)
	}
	if got := (deskkit.Config{}).BoundRoles(); got != nil {
		t.Errorf("an unconfigured roster binds %v, want nil (no compiled default)", got)
	}
}

// TestCanonicalDocumentedSet pins the fallback set row for row: the nine rows the docs have
// always listed, now with the six to: rows between the raised-by stamps and the PR-state pair.
func TestCanonicalDocumentedSet(t *testing.T) {
	want := []string{
		"review-request",
		"raised-by:desk", "raised-by:worker", "raised-by:reviewer", "raised-by:verifier",
		"raised-by:issue-loop", "raised-by:intake-loop",
		"to:desk", "to:worker", "to:reviewer", "to:verifier", "to:issue-loop", "to:intake-loop",
		"authorization-needed", "approval-needed",
	}
	got := Names(Canonical(DocumentedRoles()))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Canonical(DocumentedRoles()) =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, l := range Canonical(DocumentedRoles()) {
		if len(l.Color) != 6 || strings.HasPrefix(l.Color, "#") {
			t.Errorf("%s: color %q is not six bare hex digits", l.Name, l.Color)
		}
		// GitHub caps a label description at 100 characters.
		if n := len([]rune(l.Description)); n == 0 || n > 100 {
			t.Errorf("%s: description is %d characters, want 1..100", l.Name, n)
		}
		// A single colon is a plain label on GitLab; a double colon would make it scoped
		// (mutually exclusive within its scope), which no stamp here means.
		if strings.Contains(l.Name, "::") {
			t.Errorf("%s: a double colon makes a GitLab scoped label", l.Name)
		}
	}
}

func TestCanonicalNormalizesRoles(t *testing.T) {
	got := Names(Canonical([]string{" Worker", "worker", "", "desk"}))
	want := "review-request,raised-by:worker,raised-by:desk,to:worker,to:desk,authorization-needed,approval-needed"
	if strings.Join(got, ",") != want {
		t.Errorf("got %v\nwant %s", got, want)
	}
}

func TestParseRoles(t *testing.T) {
	got, err := ParseRoles("desk, Worker ,,worker")
	if err != nil || strings.Join(got, ",") != "desk,worker" {
		t.Errorf("ParseRoles = %v, %v; want [desk worker]", got, err)
	}
	for _, bad := range []string{"", " , ", "to:desk", "desk worker", "a/b"} {
		if _, err := ParseRoles(bad); err == nil {
			t.Errorf("ParseRoles(%q) accepted a value that is not a role list", bad)
		}
	}
}

// TestDocumentedRolesIsAFreshSlice — one caller editing the fallback cannot change it for the
// next.
func TestDocumentedRolesIsAFreshSlice(t *testing.T) {
	a := DocumentedRoles()
	a[0] = "edited"
	if DocumentedRoles()[0] != "desk" {
		t.Error("DocumentedRoles returned a shared slice")
	}
}
