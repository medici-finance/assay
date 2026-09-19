package main

import "testing"

func TestIsBriefPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"docs/streams/verify-integrity/brief-01-protected-verifier-paths.md", true},
		{"docs/streams/verify-integrity/README.md", false},
		{"statusgen/unrun.go", false},
		{"tools/desk/cmd/statusgen/brief-fake.go", false},
	}
	for _, c := range cases {
		if got := isBriefPath(c.path); got != c.want {
			t.Errorf("isBriefPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsFixturePath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"statusgen/testdata/golden.md", true},
		{"tools/desk/cmd/deskflip/testdata/fixture.json", true},
		{"testdata/x.json", true},
		{"statusgen/unrun.go", false},
	}
	for _, c := range cases {
		if got := isFixturePath(c.path); got != c.want {
			t.Errorf("isFixturePath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsVerifyDPath(t *testing.T) {
	if !isVerifyDPath("scripts/verify.d/10-lint.sh") {
		t.Error("scripts/verify.d/10-lint.sh should be protected")
	}
	if !isVerifyDPath("verify.d/10-lint.sh") {
		t.Error("verify.d/10-lint.sh should be protected")
	}
	if isVerifyDPath("scripts/verifyd/10-lint.sh") {
		t.Error("verifyd (no dot) must not match verify.d")
	}
}

// --- Evaluate: Verify rows 1, 2, 3, 4 -----------------------------------------------------

const deskLogin = "assay-desk-app[bot]"
const verifierLogin = "assay-verifier-app[bot]"
const workerLogin = "assay-worker-app[bot]"

func baseIdentities() (desk, verifier []string) {
	return []string{deskLogin, "app/assay-desk-app"}, []string{verifierLogin, "app/assay-verifier-app"}
}

// TestEvaluateRow1 is Verify row 1: a worker-identity PR touching one Go source file under
// statusgen plus a brief file whose Verify table is edited is labelled and gate-forced.
// FAIL-FIRST perturbation: the SAME diff, author swapped to the desk identity, must NOT
// label (row 1's own "perturb" clause).
func TestEvaluateRow1(t *testing.T) {
	desk, verifier := baseIdentities()
	files := []FileEntry{
		{Path: "statusgen/unrun.go"},
		{Path: "docs/streams/verify-integrity/brief-01-protected-verifier-paths.md", VerifySectionTouched: true},
	}

	worker := Evaluate(EvalInput{
		AuthorLogin: workerLogin, DeskLogins: desk, VerifierLogins: verifier,
		Files: files, DiffReadable: true,
	})
	if worker.State != stateCheckedFailed || !worker.Label || !worker.GateForced {
		t.Fatalf("worker identity: want checked-failed + label + gate-forced, got %+v", worker)
	}

	// perturb: same diff, desk identity → no label.
	deskAuthor := Evaluate(EvalInput{
		AuthorLogin: deskLogin, DeskLogins: desk, VerifierLogins: verifier,
		Files: files, DiffReadable: true,
	})
	if deskAuthor.State != stateCheckedClean || deskAuthor.Label {
		t.Fatalf("desk identity perturb: want checked-clean, no label, got %+v", deskAuthor)
	}
}

// TestEvaluateRow2 is Verify row 2: a worker PR touching the brief file ONLY (pure
// authoring) is exempt. FAIL-FIRST perturbation: adding a Go source file to the diff flips
// it to labelled.
func TestEvaluateRow2(t *testing.T) {
	desk, verifier := baseIdentities()
	briefOnly := []FileEntry{
		{Path: "docs/streams/verify-integrity/brief-01-protected-verifier-paths.md", VerifySectionTouched: true},
	}
	clean := Evaluate(EvalInput{
		AuthorLogin: workerLogin, DeskLogins: desk, VerifierLogins: verifier,
		Files: briefOnly, DiffReadable: true,
	})
	if clean.State != stateCheckedClean || clean.Label {
		t.Fatalf("brief-only diff: want checked-clean, no label, got %+v", clean)
	}

	// perturb: add a Go source file → now labelled.
	withCode := append(append([]FileEntry{}, briefOnly...), FileEntry{Path: "statusgen/verifyrows.go"})
	labelled := Evaluate(EvalInput{
		AuthorLogin: workerLogin, DeskLogins: desk, VerifierLogins: verifier,
		Files: withCode, DiffReadable: true,
	})
	if labelled.State != stateCheckedFailed || !labelled.Label {
		t.Fatalf("brief + code diff perturb: want checked-failed + label, got %+v", labelled)
	}
}

// TestEvaluateRow3 is Verify row 3: a PR carrying a `regen:` label whose diff touches
// testdata + code is exempt (the regen exemption), even though the same diff without the
// label would be labelled — that "without the label" case is the fail-first perturbation.
func TestEvaluateRow3(t *testing.T) {
	desk, verifier := baseIdentities()
	files := []FileEntry{
		{Path: "statusgen/testdata/golden.md"},
		{Path: "statusgen/verifyrows.go"},
	}

	withoutRegen := Evaluate(EvalInput{
		AuthorLogin: workerLogin, DeskLogins: desk, VerifierLogins: verifier,
		Files: files, DiffReadable: true,
	})
	if withoutRegen.State != stateCheckedFailed || !withoutRegen.Label {
		t.Fatalf("perturb (no regen: label): want checked-failed + label, got %+v", withoutRegen)
	}

	withRegen := Evaluate(EvalInput{
		AuthorLogin: workerLogin, DeskLogins: desk, VerifierLogins: verifier,
		ExistingLabels: []string{"regen:testdata"},
		Files:          files, DiffReadable: true,
	})
	if withRegen.State != stateCheckedClean || withRegen.Label {
		t.Fatalf("regen: label present: want checked-clean, no label, got %+v", withRegen)
	}
}

// TestEvaluateRow4 is Verify row 4: a diff that could not be read is could-not-check, and
// NEVER rendered as clean — the fail-first perturbation is the same input with
// DiffReadable flipped true, which must NOT be could-not-check.
func TestEvaluateRow4(t *testing.T) {
	desk, verifier := baseIdentities()
	unreadable := Evaluate(EvalInput{
		AuthorLogin: workerLogin, DeskLogins: desk, VerifierLogins: verifier,
		DiffReadable: false,
	})
	if unreadable.State != stateCouldNotCheck {
		t.Fatalf("unreadable diff: want could-not-check, got %+v", unreadable)
	}
	if unreadable.Label {
		t.Fatalf("could-not-check must never also carry a label decision: %+v", unreadable)
	}

	readable := Evaluate(EvalInput{
		AuthorLogin: workerLogin, DeskLogins: desk, VerifierLogins: verifier,
		DiffReadable: true,
	})
	if readable.State == stateCouldNotCheck {
		t.Fatalf("perturb (diff readable, no files): must not be could-not-check, got %+v", readable)
	}
}
