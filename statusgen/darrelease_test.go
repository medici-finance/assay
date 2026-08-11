package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ----- release-pin fixtures -----
//
// The shape under test is PR #737's: main's daml.yaml at 0.1.45, dev tracking
// it, prod deliberately held at the 0.1.44 release. Today that costs 3 gating
// PROBLEMs and is indistinguishable from a mistake; with a release pin declared
// it must cost zero problems and one informational line — and every way of
// faking the pin must still be a hard problem.

// writeEnvDarParts writes env's DAR ConfigMap set holding a DAR whose package
// entry names carry darVer. The ConfigMap prefix and DAML package are the
// injected product identities (darCMPrefix()/darPkg()). Split mid-version-string,
// as writeDarCore does, so reassembly is exercised.
func writeEnvDarParts(t *testing.T, root, env, darVer string) {
	t.Helper()
	prefix := darCMPrefix()
	dar := []byte("PK\x03\x04noise/" + darPkg() + "-v9-" + darVer + "-abc123/ExampleModule.dalfPK\x05\x06tail")
	third := len(dar)/3 + 3
	two := 2 * (len(dar) / 3)
	for n, chunk := range [][]byte{dar[:third], dar[third:two], dar[two:]} {
		name := fmt.Sprintf("%s%d", prefix, n+1)
		mustWriteFile(t, filepath.Join(root, "k8s", env, "app", name+".yaml"),
			"---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: "+name+"\nbinaryData:\n  dar.b64: "+base64.StdEncoding.EncodeToString(chunk)+"\n")
	}
}

// releaseManifestYAML renders a deploy manifest pinning darVer as dar.version
// with a `release:` block. An empty releaseBody omits the block entirely (the
// undeclared shape). The manifest carries decoy `version:` keys before and
// after the dar: block, as the real one does.
func releaseManifestYAML(darVer, releaseBody string) string {
	out := "---\napiVersion: example.loan/deploy-v1alpha1\nkind: DeployManifest\npackageName: \"#" + darPkg() + "-v9\"\n\nimages:\n  ledgerService:\n    version: \"9.9.9\"\n\n"
	out += "dar:\n  version: \"" + darVer + "\"\n  sha256: \"\"  # filled at deploy time\n"
	if releaseBody != "" {
		out += releaseBody
	}
	return out + "\ngovernanceContracts:\n  - name: CommissionConfig\n    version: \"0.1.0\"\n"
}

// releaseBlock renders a well-formed dar.release block.
func releaseBlock(version, tag, date string) string {
	return "  release:\n    version: \"" + version + "\"\n    tag: \"" + tag + "\"\n    date: \"" + date + "\"\n"
}

// writeLagFixture lays out the PR #737 shape: daml.yaml + dev at damlVer, prod
// at prodDarVer, with prod's deploy manifest carrying releaseBody (possibly
// empty) and pinning prodPin as dar.version. Both envs get a example-deploy Job
// pinning their own version.
func writeLagFixture(t *testing.T, damlVer, prodDarVer, prodPin, releaseBody string) string {
	t.Helper()
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "daml.yaml"),
		"sdk-version: 3.5.1\nname: "+darPkg()+"-v9\nversion: "+damlVer+"\nsource: daml\n")
	writeEnvDarParts(t, root, "dev", damlVer)
	writeEnvDarParts(t, root, "prod", prodDarVer)
	mustWriteFile(t, filepath.Join(root, "k8s", "dev", "app", "example-deploy.yaml"), deployJobYAML(damlVer))
	mustWriteFile(t, filepath.Join(root, "k8s", "prod", "app", "example-deploy.yaml"), deployJobYAML(prodPin))
	mustWriteFile(t, filepath.Join(root, "k8s", "prod", "app", "deploy-manifest.yaml"), releaseManifestYAML(prodPin, releaseBody))
	return root
}

func problemsContaining(problems []string, sub string) []string {
	var out []string
	for _, p := range problems {
		if strings.Contains(p, sub) {
			out = append(out, p)
		}
	}
	return out
}

// TestDarSyncLaggingProdWithoutDeclarationStillProblems is the control: this is
// today's behaviour and the cost PR #737 is paying. It must NOT change for an
// env that declares nothing.
func TestDarSyncLaggingProdWithoutDeclarationStillProblems(t *testing.T) {
	root := writeLagFixture(t, "0.1.45", "0.1.44", "0.1.44", "")
	problems, notices := darSyncCheck(root, nil)
	if len(problems) != 3 {
		t.Fatalf("expected 3 problems for an undeclared lagging prod (DAR bytes + 2 pins), got %d: %v", len(problems), problems)
	}
	for _, want := range []string{
		"k8s/prod/app/" + darCMPrefix() + "{1,2,3}.yaml: DAR ConfigMaps hold 0.1.44 but daml.yaml is 0.1.45",
		"EXPECTED_DAR_VERSION is 0.1.44 but daml.yaml is 0.1.45",
		"dar.version is 0.1.44 but daml.yaml is 0.1.45",
	} {
		if len(problemsContaining(problems, want)) == 0 {
			t.Errorf("missing expected problem %q in %v", want, problems)
		}
	}
	if len(notices) != 0 {
		t.Errorf("an env with no declaration must produce no notices, got %v", notices)
	}
}

// TestDarSyncReleasePinnedProdIsInformational is the whole point: the same tree
// with a declared, artifact-verified release pin costs zero problems and says
// plainly that prod lags.
func TestDarSyncReleasePinnedProdIsInformational(t *testing.T) {
	root := writeLagFixture(t, "0.1.45", "0.1.44", "0.1.44", releaseBlock("0.1.44", "dar/v0.1.44", "2026-07-26"))
	problems, notices := darSyncCheck(root, nil)
	if len(problems) != 0 {
		t.Fatalf("a verified release-pinned prod must produce no problems, got %v", problems)
	}
	if len(notices) != 1 {
		t.Fatalf("expected exactly 1 informational notice, got %v", notices)
	}
	for _, want := range []string{"prod pinned to 0.1.44", "release dar/v0.1.44", "main at 0.1.45"} {
		if !strings.Contains(notices[0], want) {
			t.Errorf("notice %q missing %q (the ruled phrasing)", notices[0], want)
		}
	}
}

// TestDarSyncReleasePinnedEnvInStepStillSaysSo — the pinned state is never
// invisible, even when the pin happens to equal main.
func TestDarSyncReleasePinnedEnvInStepStillSaysSo(t *testing.T) {
	root := writeLagFixture(t, "0.1.44", "0.1.44", "0.1.44", releaseBlock("0.1.44", "dar/v0.1.44", "2026-07-26"))
	problems, notices := darSyncCheck(root, nil)
	if len(problems) != 0 {
		t.Fatalf("expected no problems, got %v", problems)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "in step with main") {
		t.Fatalf("expected an in-step notice, got %v", notices)
	}
}

// TestDarSyncReleasePinFailureModes covers every way a declaration can be wrong
// or forged. Each row must be a HARD problem — a check that has not been seen
// to fail has not been shown to work.
func TestDarSyncReleasePinFailureModes(t *testing.T) {
	tests := []struct {
		name        string
		damlVer     string
		prodDarVer  string
		prodPin     string
		release     string
		wantProblem string
	}{
		{
			name: "declared pin does not match the DAR bytes", damlVer: "0.1.45", prodDarVer: "0.1.44", prodPin: "0.1.43",
			release:     releaseBlock("0.1.43", "dar/v0.1.43", "2026-07-26"),
			wantProblem: "DAR ConfigMaps hold 0.1.44 but k8s/prod/app/deploy-manifest.yaml declares dar.release.version 0.1.43",
		},
		{
			name: "declared-only edit: every label agrees, the bytes do not (#587 shape)", damlVer: "0.1.45", prodDarVer: "0.1.44", prodPin: "0.1.40",
			release:     releaseBlock("0.1.40", "dar/v0.1.40", "2026-07-26"),
			wantProblem: "the release pin does not match the DAR bytes it claims to pin",
		},
		// A divergent EXPECTED_DAR_VERSION cannot be expressed in this table
		// (writeLagFixture derives the Job pin from prodPin), so it lives in
		// TestDarSyncReleasePinDeployJobMustAgree rather than as a row here
		// with an empty wantProblem — a skipped row reads like coverage and
		// asserts nothing.
		{
			name: "missing a required key", damlVer: "0.1.45", prodDarVer: "0.1.44", prodPin: "0.1.44",
			release:     "  release:\n    version: \"0.1.44\"\n    tag: \"dar/v0.1.44\"\n",
			wantProblem: `dar.release is missing required key "date"`,
		},
		{
			name: "typo'd key must not silently disable the check", damlVer: "0.1.45", prodDarVer: "0.1.44", prodPin: "0.1.44",
			release:     "  release:\n    versoin: \"0.1.44\"\n    tag: \"dar/v0.1.44\"\n    date: \"2026-07-26\"\n",
			wantProblem: `unknown key "versoin" in the dar.release block`,
		},
		{
			name: "inline value instead of a block", damlVer: "0.1.45", prodDarVer: "0.1.44", prodPin: "0.1.44",
			release:     "  release: \"0.1.44\"\n",
			wantProblem: "dar.release must be a block with version:, tag: and date: keys",
		},
		{
			name: "version is not a semver", damlVer: "0.1.45", prodDarVer: "0.1.44", prodPin: "0.1.44",
			release:     releaseBlock("latest", "dar/v0.1.44", "2026-07-26"),
			wantProblem: `dar.release.version "latest" is not a semver`,
		},
		{
			name: "date is not a real date", damlVer: "0.1.45", prodDarVer: "0.1.44", prodPin: "0.1.44",
			release:     releaseBlock("0.1.44", "dar/v0.1.44", "2026-13-45"),
			wantProblem: "is not a real date",
		},
		{
			name: "date is not YYYY-MM-DD", damlVer: "0.1.45", prodDarVer: "0.1.44", prodPin: "0.1.44",
			release:     releaseBlock("0.1.44", "dar/v0.1.44", "yesterday"),
			wantProblem: "is not YYYY-MM-DD",
		},
		{
			name: "tag is empty", damlVer: "0.1.45", prodDarVer: "0.1.44", prodPin: "0.1.44",
			release:     releaseBlock("0.1.44", "", "2026-07-26"),
			wantProblem: `dar.release is missing required key "tag"`,
		},
		{
			name: "manifest disagrees with its own pin", damlVer: "0.1.45", prodDarVer: "0.1.44", prodPin: "0.1.44",
			release:     releaseBlock("0.1.43", "dar/v0.1.43", "2026-07-26"),
			wantProblem: "dar.version is 0.1.44 but dar.release.version is 0.1.43",
		},
		{
			name: "pin ahead of main", damlVer: "0.1.44", prodDarVer: "0.1.45", prodPin: "0.1.45",
			release:     releaseBlock("0.1.45", "dar/v0.1.45", "2026-07-26"),
			wantProblem: "is AHEAD of daml.yaml 0.1.44 — a release pin may lag main, never lead it",
		},
		{
			name: "tab in the dar block", damlVer: "0.1.45", prodDarVer: "0.1.44", prodPin: "0.1.44",
			release:     "  release:\n\t\tversion: \"0.1.44\"\n    tag: \"dar/v0.1.44\"\n    date: \"2026-07-26\"\n",
			wantProblem: "contains a tab",
		},
		{
			name: "nested junk inside the release block", damlVer: "0.1.45", prodDarVer: "0.1.44", prodPin: "0.1.44",
			release:     "  release:\n    version: \"0.1.44\"\n      nested: true\n    tag: \"dar/v0.1.44\"\n    date: \"2026-07-26\"\n",
			wantProblem: "nested or misindented entry",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No skip clause: a row with nothing to assert must fail loudly
			// rather than pass silently.
			if tt.wantProblem == "" {
				t.Fatalf("row %q asserts nothing", tt.name)
			}
			root := writeLagFixture(t, tt.damlVer, tt.prodDarVer, tt.prodPin, tt.release)
			problems, _ := darSyncCheck(root, nil)
			if len(problemsContaining(problems, tt.wantProblem)) == 0 {
				t.Fatalf("expected a problem containing %q, got %v", tt.wantProblem, problems)
			}
		})
	}
}

// TestDarSyncReleasePinDeployJobMustAgree — the Job manifest's
// EXPECTED_DAR_VERSION is checked against the RELEASE pin, not against
// daml.yaml, and disagreeing with it is a hard problem.
func TestDarSyncReleasePinDeployJobMustAgree(t *testing.T) {
	root := writeLagFixture(t, "0.1.45", "0.1.44", "0.1.44", releaseBlock("0.1.44", "dar/v0.1.44", "2026-07-26"))
	// Job pins a version nothing else carries.
	mustWriteFile(t, filepath.Join(root, "k8s", "prod", "app", "example-deploy.yaml"), deployJobYAML("0.1.42"))
	problems, _ := darSyncCheck(root, nil)
	if len(problemsContaining(problems, "EXPECTED_DAR_VERSION is 0.1.42 but this env is pinned to the 0.1.44 release")) == 0 {
		t.Fatalf("a Job pin disagreeing with the release pin must be a problem, got %v", problems)
	}
}

// TestDarSyncUnverifiedPinReportsAgainstThePin — once an env declares a pin,
// the pin is the authority the deploy manifests are measured against, so the
// message must name it rather than daml.yaml (which is no longer what want
// holds). Wrong attribution in a failure message sends the reader to the wrong
// file.
func TestDarSyncUnverifiedPinReportsAgainstThePin(t *testing.T) {
	// Declared 0.1.43; the DAR bytes hold 0.1.44 (cross-check fails) and the Job
	// pins 0.1.44 as well.
	root := writeLagFixture(t, "0.1.45", "0.1.44", "0.1.43", releaseBlock("0.1.43", "dar/v0.1.43", "2026-07-26"))
	mustWriteFile(t, filepath.Join(root, "k8s", "prod", "app", "example-deploy.yaml"), deployJobYAML("0.1.44"))
	problems, notices := darSyncCheck(root, nil)
	if len(problemsContaining(problems, "EXPECTED_DAR_VERSION is 0.1.44 but this env is pinned to the 0.1.43 release")) == 0 {
		t.Fatalf("expected the pin-attributed message, got %v", problems)
	}
	for _, p := range problems {
		if strings.Contains(p, "but daml.yaml is 0.1.43") {
			t.Fatalf("message must not attribute the pin's value to daml.yaml: %q", p)
		}
	}
	if len(notices) != 0 {
		t.Fatalf("an unverified pin must not emit the informational line, got %v", notices)
	}
}

// TestDarSyncReleasePinCannotBeCrossCheckedFailsClosed — a pin whose artifact
// cannot be read is unverified, and unverified is a problem, not a pass.
func TestDarSyncReleasePinCannotBeCrossCheckedFailsClosed(t *testing.T) {
	root := writeLagFixture(t, "0.1.45", "0.1.44", "0.1.44", releaseBlock("0.1.44", "dar/v0.1.44", "2026-07-26"))
	if err := os.Remove(filepath.Join(root, darConfigMapPart(darCMPrefix(), "prod", 2))); err != nil {
		t.Fatal(err)
	}
	problems, notices := darSyncCheck(root, nil)
	if len(problemsContaining(problems, "so the release pin is unverified")) == 0 {
		t.Fatalf("an un-cross-checkable pin must be a problem, got %v", problems)
	}
	if len(notices) != 0 {
		t.Fatalf("an unverified pin must not emit the reassuring informational line, got %v", notices)
	}
}

// TestDarSyncReleasePinNeitherManifestStillProblems — the release-pin rule
// survives: an env with no deploy manifest at all is still a hard problem, and
// a release pin cannot be used to talk around it.
func TestDarSyncReleasePinNeitherManifestStillProblems(t *testing.T) {
	root := writeLagFixture(t, "0.1.45", "0.1.44", "0.1.44", releaseBlock("0.1.44", "dar/v0.1.44", "2026-07-26"))
	for _, f := range []string{"example-deploy.yaml", "deploy-manifest.yaml"} {
		if err := os.Remove(filepath.Join(root, "k8s", "prod", "app", f)); err != nil {
			t.Fatal(err)
		}
	}
	problems, _ := darSyncCheck(root, nil)
	if len(problemsContaining(problems, "k8s/prod/app: no deploy version pin found")) == 0 {
		t.Fatalf("expected #151's no-pin-found problem, got %v", problems)
	}
}

// TestDarSyncReleasePinAmbiguousManifest — two top-level dar: keys means the
// declaration is not unambiguous, so it is not honoured.
func TestDarSyncReleasePinAmbiguousManifest(t *testing.T) {
	root := writeLagFixture(t, "0.1.45", "0.1.44", "0.1.44", releaseBlock("0.1.44", "dar/v0.1.44", "2026-07-26"))
	p := filepath.Join(root, "k8s", "prod", "app", "deploy-manifest.yaml")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(raw, []byte("\ndar:\n  version: \"0.1.45\"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, notices := darSyncCheck(root, nil)
	if len(problemsContaining(problems, "more than one top-level dar: key")) == 0 {
		t.Fatalf("expected an ambiguity problem, got %v", problems)
	}
	if len(notices) != 0 {
		t.Fatalf("an ambiguous manifest must not read as pinned, got %v", notices)
	}
}

// TestDarSyncReleasePinSuppressesContentDriftForThatEnvOnly — the #587 check is
// suppressed for a VERIFIED pinned env (daml/** moving while its ConfigMaps
// stay put is the expected state under a release cadence) and for no one else.
func TestDarSyncReleasePinSuppressesContentDriftForThatEnvOnly(t *testing.T) {
	root := writeLagFixture(t, "0.1.45", "0.1.44", "0.1.44", releaseBlock("0.1.44", "dar/v0.1.44", "2026-07-26"))
	mustWriteFile(t, filepath.Join(root, "daml", "ExampleModule.daml"), "-- v1\nmodule ExampleModule where\n")

	gitRun(t, root, "init", "-q")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "main: dev at 0.1.45, prod pinned to the 0.1.44 release")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	// A PR that changes daml/ without touching EITHER env's ConfigMaps.
	mustWriteFile(t, filepath.Join(root, "daml", "ExampleModule.daml"), "-- v2: second revision\nmodule ExampleModule where\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "pr: daml source changed, no DAR rebuild")

	problems, _ := darSyncCheck(root, nil)
	if len(problemsContaining(problems, "k8s/prod/app/"+darCMPrefix()+"{1,2,3}.yaml: daml/** changed")) != 0 {
		t.Errorf("content drift must be suppressed for a verified release-pinned env, got %v", problems)
	}
	if len(problemsContaining(problems, "k8s/dev/app/"+darCMPrefix()+"{1,2,3}.yaml: daml/** changed")) == 0 {
		t.Errorf("content drift must still fire for the unpinned env, got %v", problems)
	}
}

// TestDarSyncBadPinDoesNotSuppressContentDrift — the suppression is gated on the
// declared-vs-derived cross-check, so a forged declaration cannot be the thing
// that turns #587 off.
//
// The fixture must reach the cross-check to test it. It is therefore the
// "declared-only edit" (#587) shape: every version STRING agrees — the
// manifest's dar.version, dar.release.version and the Job's
// EXPECTED_DAR_VERSION all say 0.1.40 — while the committed DAR bytes hold
// 0.1.44. Nothing structural is wrong with the declaration, so it parses, the
// env IS pinned, and the only thing standing between it and the suppression is
// `derived == pin.Version`. An earlier version of this test set dar.version to
// 0.1.44 against a 0.1.40 release block; that died in parseDarReleasePin on the
// self-agreement check, never reached pinVerified, and passed even with the
// equality term removed. The assertion below keeps it from drifting back.
func TestDarSyncBadPinDoesNotSuppressContentDrift(t *testing.T) {
	root := writeLagFixture(t, "0.1.45", "0.1.44", "0.1.40", releaseBlock("0.1.40", "dar/v0.1.40", "2026-07-26"))
	mustWriteFile(t, filepath.Join(root, "daml", "ExampleModule.daml"), "-- v1\nmodule ExampleModule where\n")

	gitRun(t, root, "init", "-q")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "main")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	mustWriteFile(t, filepath.Join(root, "daml", "ExampleModule.daml"), "-- v2\nmodule ExampleModule where\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "pr: daml changed")

	problems, notices := darSyncCheck(root, nil)
	// Guard the fixture, not just the outcome: if the declaration ever fails to
	// parse again, `pinned` is false, the #587 problem below fires for a reason
	// that has nothing to do with the cross-check, and this test goes vacuous.
	if len(problemsContaining(problems, "disagrees with its own release pin")) != 0 {
		t.Fatalf("fixture regression: the declaration must PARSE so the cross-check is what rejects it, got %v", problems)
	}
	if len(problemsContaining(problems, "the release pin does not match the DAR bytes it claims to pin")) == 0 {
		t.Fatalf("expected the cross-check to reject the forged pin, got %v", problems)
	}
	if len(problemsContaining(problems, "k8s/prod/app/"+darCMPrefix()+"{1,2,3}.yaml: daml/** changed")) == 0 {
		t.Fatalf("a pin that failed its cross-check must not suppress the #587 check, got %v", problems)
	}
	if len(notices) != 0 {
		t.Fatalf("a pin that failed its cross-check must not emit the reassuring notice, got %v", notices)
	}
}

// TestDarSyncAheadPinRelaxesNothing — an AHEAD pin is documented as failing
// closed, so it must not simultaneously buy the two things a good pin buys.
// The declaration here is otherwise perfect (complete block, self-consistent,
// deploy pins agree, and the DAR bytes really do carry the declared version),
// so the ONLY thing disqualifying it is that it leads main. It must still lose
// the #587 suppression and the reassuring notice: a pin the checker has already
// called wrong cannot be trusted enough to turn another check off. The run is
// red either way, but "red plus a line saying prod is validated against its own
// release pin" is a mixed signal, and the suppression would outlive the red the
// moment daml.yaml caught up.
func TestDarSyncAheadPinRelaxesNothing(t *testing.T) {
	root := writeLagFixture(t, "0.1.44", "0.1.45", "0.1.45", releaseBlock("0.1.45", "dar/v0.1.45", "2026-07-26"))
	mustWriteFile(t, filepath.Join(root, "daml", "ExampleModule.daml"), "-- v1\nmodule ExampleModule where\n")

	gitRun(t, root, "init", "-q")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "main")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	mustWriteFile(t, filepath.Join(root, "daml", "ExampleModule.daml"), "-- v2\nmodule ExampleModule where\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "pr: daml changed")

	problems, notices := darSyncCheck(root, nil)
	if len(problemsContaining(problems, "is AHEAD of daml.yaml 0.1.44")) == 0 {
		t.Fatalf("expected the AHEAD problem, got %v", problems)
	}
	// Guard the fixture: the pin must otherwise be verifiable, or this test
	// proves nothing about `ahead` specifically.
	if len(problemsContaining(problems, "does not match the DAR bytes")) != 0 {
		t.Fatalf("fixture regression: the pin must cross-check clean so AHEAD is the only disqualifier, got %v", problems)
	}
	if len(problemsContaining(problems, "k8s/prod/app/"+darCMPrefix()+"{1,2,3}.yaml: daml/** changed")) == 0 {
		t.Errorf("an AHEAD pin must not suppress the #587 content-drift check, got %v", problems)
	}
	if len(notices) != 0 {
		t.Errorf("an AHEAD pin must not emit the reassuring informational line, got %v", notices)
	}
}

// TestDarSyncNoDeclarationIsBitForBitUnchanged — every env without a
// declaration must behave exactly as before, including the notice channel.
func TestDarSyncNoDeclarationIsBitForBitUnchanged(t *testing.T) {
	for _, tt := range []struct {
		name     string
		manifest string
	}{
		{"no deploy-manifest at all", ""},
		{"deploy-manifest with no release block", deployManifestYAML("0.1.16")},
		{"release: key under a different top-level block", "images:\n  release: \"nightly\"\n\ndar:\n  version: \"0.1.16\"\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := writeDarFixture(t, "0.1.16", "0.1.16", "0.1.16")
			if tt.manifest != "" {
				mustWriteFile(t, filepath.Join(root, "k8s", "prod", "app", "deploy-manifest.yaml"), tt.manifest)
			}
			problems, notices := darSyncCheck(root, nil)
			if len(problems) != 0 || len(notices) != 0 {
				t.Fatalf("undeclared tree must be clean and silent, got problems=%v notices=%v", problems, notices)
			}
		})
	}
}

// TestDarReleasePinUnreadableManifestFailsClosed — an I/O failure must never be
// read as "this env declares nothing".
func TestDarReleasePinUnreadableManifestFailsClosed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "k8s", "prod", "app", "deploy-manifest.yaml")
	if err := os.MkdirAll(dir, 0o755); err != nil { // a directory where a file belongs
		t.Fatal(err)
	}
	pin, problems := darReleasePinFor(root, "prod")
	if pin != nil || len(problems) == 0 {
		t.Fatalf("unreadable manifest must yield no pin and a problem, got pin=%v problems=%v", pin, problems)
	}
}
