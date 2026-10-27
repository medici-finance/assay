package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskcore/attestation"
)

// The container bracket with a version-2 (cross-repository) receipt. The home
// is the deliverable checkout the rows run in; the brief lives in a separate
// tracking checkout outside it, as the host check's isolated tracking home
// does. Both repositories are bound: the tracking one as repo#issue, the
// deliverable one as source-repo=, and the brief by its bytes.

const (
	crossTracking       = "example-org/one"
	crossSource         = "example-org/two"
	crossTrackingSource = "dddddddddddddddddddddddddddddddddddddddd"
	crossCheckout       = "autocrlf=false eol=lf"
)

// newCrossRepoFixture is the container fixture with its brief moved to a
// tracking checkout outside the home. fx.brief is the tracking brief; the
// home keeps its own committed copy, which a cross-repository run never
// reads. It returns the fixture and the tracking checkout.
func newCrossRepoFixture(t *testing.T) (containerFixture, string) {
	t.Helper()
	fx := newContainerFixture(t)
	tracking := t.TempDir()
	brief := filepath.Join(tracking, "docs", "brief.md")
	if err := os.MkdirAll(filepath.Dir(brief), 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, brief, getFile(t, fx.brief))
	fx.brief = brief
	return fx, tracking
}

// crossBinding is a complete version-2 binding for the fixture: the tracking
// brief's own bytes, both repositories, and well-formed digests and checkout
// conversions.
func crossBinding(t *testing.T, fx containerFixture) attestation.Binding {
	t.Helper()
	sum := sha256.Sum256([]byte(getFile(t, fx.brief)))
	return attestation.Binding{
		Version: 2, SourceRepo: crossSource, TrackingSource: crossTrackingSource,
		TrackingHomeSHA256: strings.Repeat("e", 64), TrackingCheckout: crossCheckout,
		Run: fixtureRun, Repo: crossTracking, Source: fixtureSource, Brief: "docs/brief.md",
		BriefSHA256: hex.EncodeToString(sum[:]), PlanSHA256: strings.Repeat("f", 64), HomeSHA256: strings.Repeat("a", 64),
		Model: "m", Tier: "strong", Checkout: crossCheckout,
	}
}

// crossCheck is a version-2 `deskdispatch --check-verifier` receipt for b,
// admitting home as the deliverable home and tracking as the brief root.
func crossCheck(t *testing.T, b attestation.Binding, tracking, home string) string {
	t.Helper()
	raw, err := json.Marshal(struct {
		Issue                 int
		BriefRoot, SourceHome string
		Binding               attestation.Binding
	}{7, tracking, home, b})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// crossLine is the version-2 attestation line for b at issue 7.
func crossLine(b attestation.Binding) string {
	return fmt.Sprintf("Verification-Attestation: %s#7 run=%s source-repo=%s source=%s tracking-source=%s brief=%s model=%s tier=%s",
		b.Repo, b.Run, b.SourceRepo, b.Source, b.TrackingSource, b.Brief, b.Model, b.Tier)
}

// regression: #4619
// TestCrossRepoBracket: a version-2 receipt is admitted on all three steps.
// The open step writes a receipt when the host check admits --root as the
// deliverable home; the container run executes the row and records the
// attestation line naming BOTH repositories; the close step re-admits the
// run and appends one bracket line; --bracket-digest then matches. Before
// the close succeeds, a host check whose binding differs from the receipt's
// (another source repository, or a field outside the line), that admits
// another home, or that returns a version-1 receipt, and a witness whose
// line names another or the same repository, each append nothing.
func TestCrossRepoBracket(t *testing.T) {
	fx, tracking := newCrossRepoFixture(t)
	t.Setenv("DESK_LOOP", "")
	clock := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	savedNow := nowFunc
	nowFunc = func() time.Time { return clock }
	t.Cleanup(func() { nowFunc = savedNow })

	b := crossBinding(t, fx)
	check := crossCheck(t, b, tracking, fx.home)
	putFile(t, fx.check, check)
	if res, stderr := captureVerifyrun(t, []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt}); res.code != verifyrunExitPass {
		t.Fatalf("open: exit %d stderr %q", res.code, stderr)
	}
	clock = clock.Add(time.Minute)
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code != verifyrunExitPass {
		t.Fatalf("container run: exit %d stderr %q", res.code, stderr)
	}
	if _, err := os.Stat(fx.marker); err != nil {
		t.Fatal("the cross-repository container run did not execute its row")
	}
	witness := getFile(t, fx.witness)
	want := crossLine(b)
	if !strings.HasSuffix(witness, "\n"+want+"\n") {
		t.Fatalf("the witness does not end with the attestation line naming both repositories %q:\n%s", want, witness)
	}
	tree := regexp.MustCompile(`(?m)^Tree-Digest: (\S+)$`).FindStringSubmatch(witness)
	if tree == nil {
		t.Fatalf("no Tree-Digest line in the witness:\n%s", witness)
	}

	closeArgs := []string{"--close-bracket", "--root", fx.home, "--brief", fx.brief, "--admission-receipt", fx.receipt, "--witness", fx.witness}
	otherHome := t.TempDir()
	withBinding := func(edit func(*attestation.Binding)) string {
		c := b
		edit(&c)
		return crossCheck(t, c, tracking, fx.home)
	}
	refusals := []struct {
		name, check, witness, why string
	}{
		{"host binds another source repository", withBinding(func(c *attestation.Binding) { c.SourceRepo = "example-org/three" }), witness, "binding is not the receipt's"},
		{"host binds another tracking repository", withBinding(func(c *attestation.Binding) { c.Repo = "example-org/three" }), witness, "binding is not the receipt's"},
		{"host binding differs outside the line", withBinding(func(c *attestation.Binding) { c.TrackingHomeSHA256 = strings.Repeat("9", 64) }), witness, "binding is not the receipt's"},
		{"host admits another home", crossCheck(t, b, tracking, otherHome), witness, "admitted the deliverable home"},
		{"host returns version 1", checkJSON(1, fixtureRun, fixtureSource, "docs/brief.md", "m"), witness, "returned a version-1 receipt"},
		{"witness names another source repository", check, strings.Replace(witness, "source-repo="+crossSource, "source-repo=example-org/three", 1), "does not name the run the witness names"},
		{"witness names one repository twice", check, strings.Replace(witness, "source-repo="+crossSource, "source-repo="+crossTracking, 1), "not the version-1 or version-2 form"},
	}
	for _, c := range refusals {
		t.Run(c.name, func(t *testing.T) {
			putFile(t, fx.check, c.check)
			putFile(t, fx.witness, c.witness)
			res, stderr := captureVerifyrun(t, closeArgs)
			if res.code == verifyrunExitPass || !strings.Contains(stderr, c.why) {
				t.Fatalf("exit %d stderr %q, want a refusal naming %q", res.code, stderr, c.why)
			}
			if got := getFile(t, fx.witness); got != c.witness {
				t.Fatalf("a refused close changed the witness:\n%s", got)
			}
		})
	}

	putFile(t, fx.check, check)
	putFile(t, fx.witness, witness)
	preClose := digestOf(t, witness)
	if res, stderr := captureVerifyrun(t, closeArgs); res.code != verifyrunExitPass {
		t.Fatalf("matching close: exit %d stderr %q", res.code, stderr)
	}
	got := getFile(t, fx.witness)
	above, line, _ := strings.Cut(got, admissionBracketPrefix)
	if above != witness || strings.Count(got, admissionBracketPrefix) != 1 {
		t.Fatalf("the close did not append exactly one line under the unchanged witness:\n%s", got)
	}
	bracket := fmt.Sprintf(`^run=%s source=%s tree=%s witness=%s opened=%s closed=\S+ before=receipt after=check-verifier\n$`,
		fixtureRun, fixtureSource, regexp.QuoteMeta(tree[1]), preClose, fixtureOpenedOrAny)
	if !regexp.MustCompile(bracket).MatchString(line) {
		t.Fatalf("the bracket line %q does not match %s", line, bracket)
	}
	n := strings.Count(witness, "\n") + 1
	if out := verbLines(t, got, verifyrunExitPass); len(out) != 1 || !strings.Contains(out[0], fmt.Sprintf("bracket=L%d header=L1 witness=%s recorded=%s verdict=match", n, preClose, preClose)) {
		t.Fatalf("--bracket-digest of the bracketed cross-repository file: %q", out)
	}
	// Both repositories are inside the digest: renaming either one in the
	// attestation line moves it.
	for _, from := range []string{crossTracking + "#7", "source-repo=" + crossSource} {
		to := strings.Replace(from, "example-org/", "example-org/x", 1)
		if d := digestOf(t, strings.Replace(witness, from, to, 1)); d == preClose {
			t.Fatalf("renaming %s in the attestation line left the digest unchanged", from)
		}
	}
}

// regression: #4619
// TestCrossRepoBindingRefused: a version-2 receipt that leaves either
// repository unbound, names it in a form that is not owner/name, names one
// repository as both, drops the tracking commit, or does not bind the brief
// this run reads (other bytes, another path, a brief inside the home) is
// refused by the container run before any row executes, and by the open
// step with no receipt written. The control proves the same receipt with
// the full binding is admitted, so each refusal is the binding's.
func TestCrossRepoBindingRefused(t *testing.T) {
	fx, tracking := newCrossRepoFixture(t)
	t.Setenv("DESK_LOOP", "verify-desk")
	base := crossBinding(t, fx)
	insideHome := filepath.Join(fx.home, "docs", "brief.md")
	cases := []struct {
		name  string
		edit  func(*attestation.Binding)
		brief string // "" is the tracking brief
		why   string
	}{
		{"tracking repository missing", func(b *attestation.Binding) { b.Repo = "" }, "", `tracking repository "" is not an owner/name`},
		{"source repository missing", func(b *attestation.Binding) { b.SourceRepo = "" }, "", `source repository "" is not an owner/name`},
		{"tracking repository not owner/name", func(b *attestation.Binding) { b.Repo = "one" }, "", `tracking repository "one" is not an owner/name`},
		{"source repository not owner/name", func(b *attestation.Binding) { b.SourceRepo = "two" }, "", `source repository "two" is not an owner/name`},
		{"one repository as both", func(b *attestation.Binding) { b.SourceRepo = "Example-Org/One" }, "", "as both its tracking and its source repository"},
		{"tracking source missing", func(b *attestation.Binding) { b.TrackingSource = "" }, "", "a 40-hex tracking source"},
		{"brief bytes differ", func(b *attestation.Binding) { b.BriefSHA256 = strings.Repeat("0", 64) }, "", "does not hold the bytes the receipt admits"},
		{"another brief path", func(b *attestation.Binding) { b.Brief = "docs/other.md" }, "", "the receipt admits brief docs/other.md"},
		{"brief inside the home", nil, insideHome, "sits inside the home"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { crossRefusal(t, fx, tracking, base, c.edit, c.brief, c.why) })
	}

	// The open step also refuses a check that admitted another home.
	putFile(t, fx.check, crossCheck(t, base, tracking, t.TempDir()))
	if res, stderr := captureVerifyrun(t, []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt}); res.code == verifyrunExitPass || !strings.Contains(stderr, "admitted the deliverable home") {
		t.Fatalf("another admitted home: open exit %d stderr %q", res.code, stderr)
	}

	// Control: the full binding is admitted and the row runs.
	for _, p := range []string{fx.receipt, fx.witness, fx.marker} {
		os.Remove(p)
	}
	fx.writeBracketReceipt(t, crossCheck(t, base, tracking, fx.home))
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code != verifyrunExitPass {
		t.Fatalf("control: exit %d stderr %q", res.code, stderr)
	}
	if _, err := os.Stat(fx.marker); err != nil {
		t.Fatal("control: the row did not run with the full binding")
	}
}

// crossRefusal runs one TestCrossRepoBindingRefused case: the binding is base
// with edit applied, and brief ("" for the tracking brief) is the brief both
// steps are given. The container run and the open step must each refuse.
func crossRefusal(t *testing.T, fx containerFixture, tracking string, base attestation.Binding, edit func(*attestation.Binding), brief, why string) {
	t.Helper()
	// A case that was wrongly admitted must not leave its run behind for the
	// next one.
	for _, p := range []string{fx.receipt, fx.witness, fx.marker} {
		os.Remove(p)
	}
	b := base
	if edit != nil {
		edit(&b)
	}
	if brief == "" {
		brief = fx.brief
	}
	check := crossCheck(t, b, tracking, fx.home)

	// The container run.
	fx.writeBracketReceipt(t, check)
	before := getFile(t, fx.brief)
	args := []string{"--sandbox=" + sandboxContainerNetns, "--root", fx.home, "--brief", brief,
		"--admission-receipt", fx.receipt, "--container-image-digest", fixtureImage, "--witness", fx.witness}
	res, stderr := captureVerifyrun(t, args)
	if res.code == verifyrunExitPass || !strings.Contains(stderr, why) {
		t.Fatalf("container run exit %d stderr %q, want a refusal naming %q", res.code, stderr, why)
	}
	fx.assertNothingRan(t, before)

	// The open step.
	if err := os.Remove(fx.receipt); err != nil {
		t.Fatal(err)
	}
	putFile(t, fx.check, check)
	res, stderr = captureVerifyrun(t, []string{"--open-bracket", "--root", fx.home, "--brief", brief, "--out", fx.receipt})
	if res.code == verifyrunExitPass || !strings.Contains(stderr, "no receipt written") {
		t.Fatalf("open exit %d stderr %q, want the open refused", res.code, stderr)
	}
	if _, err := os.Stat(fx.receipt); err == nil {
		t.Fatal("a refused open wrote a receipt")
	}
}
