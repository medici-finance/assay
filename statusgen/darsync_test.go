package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mustWriteFile writes content at path, creating parent directories.
func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeDarCore lays out daml.yaml and both envs' DAR ConfigMap triples — the
// half of the fixture that is independent of which deploy manifest an env uses.
// The fake DAR payload embeds the package entry name and is split
// MID-version-string across part1/part2/part3 to prove the check reassembles
// before matching (the real byte split can land anywhere).
func writeDarCore(t *testing.T, root, damlVer, darVer string) {
	t.Helper()
	pkg, prefix := darPkg(), darCMPrefix()
	mustWriteFile(t, filepath.Join(root, "daml.yaml"),
		"sdk-version: 3.5.1\nname: "+pkg+"-v6\nversion: "+damlVer+"\nsource: daml\n")

	// Fake DAR bytes: zip-ish noise around the entry name (the configured DAML
	// package). Split MID-version-string across the parts so reassembly is exercised.
	dar := []byte("PK\x03\x04noise/" + pkg + "-v6-" + darVer + "-abc123/ExampleModule.dalfPK\x05\x06tail")
	third := len(dar)/3 + 3 // deliberately splits inside the version substring
	two := 2 * (len(dar) / 3)
	for _, env := range []string{"dev", "prod"} {
		for n, chunk := range [][]byte{dar[:third], dar[third:two], dar[two:]} {
			name := fmt.Sprintf("%s%d", prefix, n+1)
			mustWriteFile(t, filepath.Join(root, "k8s", env, "app", name+".yaml"),
				"---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: "+name+"\nbinaryData:\n  dar.b64: "+base64.StdEncoding.EncodeToString(chunk)+"\n")
		}
	}
}

// deployJobYAML renders a example-deploy Job manifest pinning expectedEnvVer as
// EXPECTED_DAR_VERSION. An empty expectedEnvVer renders a Job with no such env
// at all — the "gate unwired" shape.
func deployJobYAML(expectedEnvVer string) string {
	out := "apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: example-deploy-v31\nspec:\n  template:\n    spec:\n      containers:\n        - name: deploy\n          env:\n"
	if expectedEnvVer != "" {
		return out + "            - name: EXPECTED_DAR_VERSION\n              # must match daml.yaml and the DAR ConfigMaps\n              value: \"" + expectedEnvVer + "\"\n"
	}
	return out + "            - name: VERSION\n              value: \"321d7a8\"\n"
}

// deployManifestYAML renders a deploy-controller manifest pinning darVer as
// dar.version. An empty darVer omits the dar: block entirely — the "gate
// unwired" shape.
//
// The rendered manifest deliberately carries a quoted `version:` BOTH before
// the dar: block (images.ledgerService) and after it (governanceContracts),
// mirroring the real k8s/dev/app/deploy-manifest.yaml. An unanchored
// "any indented quoted semver" regex reads one of those decoys instead of the
// DAR pin, so every table row below exercises the anchoring.
func deployManifestYAML(darVer string) string {
	out := "---\napiVersion: example.loan/deploy-v1alpha1\nkind: DeployManifest\npackageName: \"#" + darPkg() + "-v9\"\n\nimages:\n  ledgerService:\n    version: \"9.9.9\"\n\n"
	if darVer != "" {
		out += "dar:\n  version: \"" + darVer + "\"\n  sha256: \"\"  # filled at deploy time from the ConfigMap DAR parts\n\n"
	}
	return out + "governanceContracts:\n  - name: CommissionConfig\n    version: \"0.1.0\"\n"
}

// writeDarFixture lays out daml.yaml, both DAR ConfigMap triples, and both
// envs' example-deploy Job manifests for the given versions — the pre-cutover
// topology, which most tests below assume.
func writeDarFixture(t *testing.T, damlVer, darVer, expectedEnvVer string) string {
	t.Helper()
	root := t.TempDir()
	writeDarCore(t, root, damlVer, darVer)
	for _, env := range []string{"dev", "prod"} {
		mustWriteFile(t, filepath.Join(root, "k8s", env, "app", "example-deploy.yaml"), deployJobYAML(expectedEnvVer))
	}
	return root
}

func TestDarSyncInSync(t *testing.T) {
	root := writeDarFixture(t, "0.1.16", "0.1.16", "0.1.16")
	if problems := darSyncProblems(root, nil); len(problems) != 0 {
		t.Fatalf("expected no problems, got %v", problems)
	}
}

func TestDarSyncStaleConfigMaps(t *testing.T) {
	// The #465 shape exactly: daml.yaml bumped, ConfigMaps + manifests stale.
	root := writeDarFixture(t, "0.1.16", "0.1.8", "0.1.8")
	problems := darSyncProblems(root, nil)
	if len(problems) != 4 {
		t.Fatalf("expected 4 problems (2 CM pairs + 2 manifests), got %d: %v", len(problems), problems)
	}
	for _, env := range []string{"dev", "prod"} {
		wantCM := "k8s/" + env + "/app/" + darCMPrefix() + "{1,2,3}.yaml: DAR ConfigMaps hold 0.1.8 but daml.yaml is 0.1.16"
		wantEnv := "EXPECTED_DAR_VERSION is 0.1.8 but daml.yaml is 0.1.16"
		var foundCM, foundEnv bool
		for _, p := range problems {
			if strings.Contains(p, wantCM) {
				foundCM = true
			}
			if strings.Contains(p, "k8s/"+env+"/") && strings.Contains(p, wantEnv) {
				foundEnv = true
			}
		}
		if !foundCM || !foundEnv {
			t.Fatalf("%s: missing expected problems (cm=%v env=%v) in %v", env, foundCM, foundEnv, problems)
		}
	}
}

// TestDarSyncNoOpWhenProductConfigUnset is the fail-safe: with the Medici-Loan
// DAR product config UNSET — the state of the OSS tool and every non-product repo
// — the DAR-drift check is a clean NO-OP even over a tree that is genuinely
// drifted. Not an error, not a false problem. The check only runs where the
// product identities are supplied.
func TestDarSyncNoOpWhenProductConfigUnset(t *testing.T) {
	// Build a drifting fixture under the neutral identity that IS configured...
	root := writeDarFixture(t, "0.1.16", "0.1.8", "0.1.8") // #465 drift present
	if problems := darSyncProblems(root, nil); len(problems) == 0 {
		t.Fatal("precondition: the fixture must be drifted while the DAR config is set")
	}
	// ...then clear the product config and re-run: the very same tree must produce
	// nothing, because with no product identity there is nothing here to check.
	installRosterWithoutDarConfig(t)
	if problems, notices := darSyncCheck(root, nil); problems != nil || notices != nil {
		t.Fatalf("unset MEDICI_LOAN_DAR_* must make darSyncCheck a no-op, got problems=%v notices=%v", problems, notices)
	}
}

// TestDarSyncProductConfigIdentityAgnostic proves the externalisation changed the
// SOURCE of the DAR names, never the check: the same drift scenario yields the
// same problems under any configured identity, with only the names substituted.
// Two scenarios are exercised — a stale-ConfigMap drift (names the prefix) and a
// missing-DAR-entry case (names the package) — so both identities are covered.
func TestDarSyncProductConfigIdentityAgnostic(t *testing.T) {
	run := func(prefix, pkg string) (norm, raw string) {
		installRosterWithDarConfig(t, prefix, pkg)
		// Stale ConfigMaps (names the prefix) ...
		staleRoot := writeDarFixture(t, "0.1.16", "0.1.8", "0.1.8")
		// ... and a tree whose DAR carries NO package entry at all (names the
		// package): daml.yaml + deploy pins present, ConfigMaps holding junk bytes.
		missRoot := writeDarFixture(t, "0.1.16", "0.1.16", "0.1.16")
		for _, env := range []string{"dev", "prod"} {
			for n := 1; n <= 3; n++ {
				mustWriteFile(t, filepath.Join(missRoot, darConfigMapPart(prefix, env, n)),
					"---\nkind: ConfigMap\nbinaryData:\n  dar.b64: "+base64.StdEncoding.EncodeToString([]byte("no entry here"))+"\n")
			}
		}
		problems := append(darSyncProblems(staleRoot, nil), darSyncProblems(missRoot, nil)...)
		raw = strings.Join(problems, "\n")
		n := strings.ReplaceAll(raw, prefix, "<PFX>")
		n = strings.ReplaceAll(n, pkg, "<PKG>")
		return n, raw
	}
	normA, rawA := run("example-dar-part", "example-tracker")
	normB, rawB := run("alt-cm", "alt-pkg")
	if normA != normB {
		t.Fatalf("DAR check is not identity-agnostic:\n--- A ---\n%s\n--- B ---\n%s", normA, normB)
	}
	if !strings.Contains(normA, "<PFX>") || !strings.Contains(normA, "<PKG>") {
		t.Fatalf("expected the normalised problems to reference both product identities, got:\n%s", normA)
	}
	// Each run must have named ITS OWN identity in the raw messages — proof the
	// names are genuinely data-driven, not still compiled in.
	if !strings.Contains(rawA, "example-dar-part") || !strings.Contains(rawB, "alt-cm") {
		t.Fatalf("raw problems did not carry each run's own configured prefix:\nA=%s\nB=%s", rawA, rawB)
	}
}

func TestDarSyncMissingExpectedVersionEnv(t *testing.T) {
	root := writeDarFixture(t, "0.1.16", "0.1.16", "")
	problems := darSyncProblems(root, nil)
	if len(problems) != 2 {
		t.Fatalf("expected 2 problems (unwired gate per env), got %d: %v", len(problems), problems)
	}
	for _, p := range problems {
		if !strings.Contains(p, "no EXPECTED_DAR_VERSION env") {
			t.Fatalf("unexpected problem: %s", p)
		}
	}
}

func TestDarSyncNoDamlYaml(t *testing.T) {
	if problems := darSyncProblems(t.TempDir(), nil); problems != nil {
		t.Fatalf("tree without daml.yaml must be out of scope, got %v", problems)
	}
}

// Path-scoping (mm/31): a changed-set that never touches the deploy surface
// (k8s/, daml/, daml.yaml) must suppress the drift — it is pre-existing main
// state the PR did not introduce (the PR #788 false-red).
func TestDarSyncChangedGateSuppressesOffDomain(t *testing.T) {
	root := writeDarFixture(t, "0.1.16", "0.1.8", "0.1.8") // drift present
	changed := []string{"docs/streams/assay-launch/README.md", "README.md"}
	if problems := darSyncProblems(root, changed); problems != nil {
		t.Fatalf("off-domain change must suppress DAR drift, got %v", problems)
	}
}

// A changed-set that DOES touch the deploy surface still fires — scope narrows
// WHEN the check runs, never WHETHER a real deploy-surface drift is caught.
func TestDarSyncChangedGateFiresInDomain(t *testing.T) {
	root := writeDarFixture(t, "0.1.16", "0.1.8", "0.1.8")
	for _, changed := range [][]string{
		{"daml.yaml"},
		{"k8s/dev/app/" + darCMPrefix() + "1.yaml"},
		{"daml/ExampleModule/Governance.daml"},
		{"docs/streams/x/README.md", "daml.yaml"}, // mixed set — one in-domain path is enough
	} {
		if problems := darSyncProblems(root, changed); len(problems) == 0 {
			t.Fatalf("in-domain change %v must still catch drift, got none", changed)
		}
	}
}

func TestDarSyncMissingConfigMap(t *testing.T) {
	root := writeDarFixture(t, "0.1.16", "0.1.16", "0.1.16")
	if err := os.Remove(filepath.Join(root, darConfigMapPart(darCMPrefix(), "dev", 2))); err != nil {
		t.Fatal(err)
	}
	problems := darSyncProblems(root, nil)
	var found bool
	for _, p := range problems {
		if strings.Contains(p, darCMPrefix()+"2.yaml: missing DAR ConfigMap") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing-ConfigMap problem, got %v", problems)
	}
}

// ----- deploy version-pin topology -----
//
// The deploy surface is migrating off the example-deploy Job onto the
// deploy-controller: dev's Job manifest is deleted and its
// expected-DAR version moves to dar.version in deploy-manifest.yaml, while prod
// keeps EXPECTED_DAR_VERSION in example-deploy.yaml. The check must accept
// EITHER manifest per env, so the statusgen pin bump and the k8s cutover can
// land in either order without one red-gating the other.

// darPinTopology describes which deploy manifests an env has and what each
// pins. A nil pointer means the file is absent; a pointer to "" means the file
// exists with its version gate unwired.
type darPinTopology struct {
	job      *string // EXPECTED_DAR_VERSION in example-deploy.yaml
	manifest *string // dar.version in deploy-manifest.yaml
}

func writeDarPinFixture(t *testing.T, damlVer, darVer string, dev, prod darPinTopology) string {
	t.Helper()
	root := t.TempDir()
	writeDarCore(t, root, damlVer, darVer)
	for env, topo := range map[string]darPinTopology{"dev": dev, "prod": prod} {
		if topo.job != nil {
			mustWriteFile(t, filepath.Join(root, "k8s", env, "app", "example-deploy.yaml"), deployJobYAML(*topo.job))
		}
		if topo.manifest != nil {
			mustWriteFile(t, filepath.Join(root, "k8s", env, "app", "deploy-manifest.yaml"), deployManifestYAML(*topo.manifest))
		}
	}
	return root
}

func TestDarSyncDeployPinTopology(t *testing.T) {
	const ver = "0.1.43"
	at := func(s string) *string { return &s }

	for _, tc := range []struct {
		name string
		dev  darPinTopology
		prod darPinTopology
		want []string // expected problem substrings; empty = no problems at all
	}{
		{
			name: "dev new manifest only (post-cutover), prod Job only",
			dev:  darPinTopology{manifest: at(ver)},
			prod: darPinTopology{job: at(ver)},
		},
		{
			name: "dev old Job only (pre-cutover), prod Job only",
			dev:  darPinTopology{job: at(ver)},
			prod: darPinTopology{job: at(ver)},
		},
		{
			// The cutover window: both manifests present. The Job is still the live
			// deploy path and pins the right version, so the not-yet-authoritative
			// example-service manifest must not red-gate the transition.
			name: "dev both present, Job pins want and new manifest lags",
			dev:  darPinTopology{job: at(ver), manifest: at("0.1.17")},
			prod: darPinTopology{job: at(ver)},
		},
		{
			// Order-independence, the other way round: the cutover landed first and
			// the Job manifest is what lags.
			name: "dev both present, new manifest pins want and Job lags",
			dev:  darPinTopology{job: at("0.1.17"), manifest: at(ver)},
			prod: darPinTopology{job: at(ver)},
		},
		{
			name: "prod cut over to deploy-manifest too",
			dev:  darPinTopology{manifest: at(ver)},
			prod: darPinTopology{manifest: at(ver)},
		},
		{
			name: "dev has neither manifest",
			dev:  darPinTopology{},
			prod: darPinTopology{job: at(ver)},
			want: []string{"k8s/dev/app: no deploy version pin found"},
		},
		{
			name: "dev new manifest drifted with no Job to fall back on",
			dev:  darPinTopology{manifest: at("0.1.17")},
			prod: darPinTopology{job: at(ver)},
			want: []string{"k8s/dev/app/deploy-manifest.yaml: dar.version is 0.1.17 but daml.yaml is 0.1.43"},
		},
		{
			name: "prod Job drifted",
			dev:  darPinTopology{manifest: at(ver)},
			prod: darPinTopology{job: at("0.1.17")},
			want: []string{"k8s/prod/app/example-deploy.yaml: EXPECTED_DAR_VERSION is 0.1.17 but daml.yaml is 0.1.43"},
		},
		{
			// Neither pin agrees — every present manifest is named, so the message
			// says every place to fix rather than only the first.
			name: "dev both present and both drifted",
			dev:  darPinTopology{job: at("0.1.8"), manifest: at("0.1.17")},
			prod: darPinTopology{job: at(ver)},
			want: []string{
				"k8s/dev/app/example-deploy.yaml: EXPECTED_DAR_VERSION is 0.1.8 but daml.yaml is 0.1.43",
				"k8s/dev/app/deploy-manifest.yaml: dar.version is 0.1.17 but daml.yaml is 0.1.43",
			},
		},
		{
			name: "dev new manifest present but dar: block missing",
			dev:  darPinTopology{manifest: at("")},
			prod: darPinTopology{job: at(ver)},
			want: []string{"k8s/dev/app/deploy-manifest.yaml: no dar.version with a semver value"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeDarPinFixture(t, ver, ver, tc.dev, tc.prod)
			problems := darSyncProblems(root, nil)
			if len(tc.want) == 0 {
				if len(problems) != 0 {
					t.Fatalf("expected no problems, got %v", problems)
				}
				return
			}
			if len(problems) != len(tc.want) {
				t.Fatalf("expected %d problems, got %d: %v", len(tc.want), len(problems), problems)
			}
			for _, want := range tc.want {
				var found bool
				for _, p := range problems {
					if strings.Contains(p, want) {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing problem %q in %v", want, problems)
				}
			}
		})
	}
}

// The dar.version pin must be read from the top-level dar: block and nowhere
// else. deployManifestYAML surrounds that block with decoy quoted semvers
// (images.ledgerService, governanceContracts) exactly as the real manifest
// does; an unanchored "indented quoted semver" regex reads a decoy instead.
func TestDeployManifestDarVersionIgnoresOtherVersionFields(t *testing.T) {
	m := deployManifestDarVerRe.FindStringSubmatch(deployManifestYAML("0.1.43"))
	if m == nil {
		t.Fatal("expected dar.version to match")
	}
	if m[1] != "0.1.43" {
		t.Fatalf("read the wrong version field: got %s, want 0.1.43", m[1])
	}
	if deployManifestDarVerRe.MatchString(deployManifestYAML("")) {
		t.Fatal("a manifest with no dar: block must not match any other version field")
	}
}

// Quotes are optional — `version: 0.1.43` is equally valid YAML and must not
// read as an unwired gate.
func TestDeployManifestDarVersionAcceptsUnquoted(t *testing.T) {
	m := deployManifestDarVerRe.FindStringSubmatch("dar:\n  version: 0.1.43\n  sha256: \"\"\n")
	if m == nil || m[1] != "0.1.43" {
		t.Fatalf("expected unquoted dar.version to match, got %v", m)
	}
}

// ----- content-drift check (issue #587) -----
//
// The label check above (TestDarSyncInSync et al.) only compares version
// STRINGS — it is blind to a ConfigMap DAR that never got rebuilt as long as
// daml.yaml, the ConfigMap's embedded version, and EXPECTED_DAR_VERSION all
// still agree. #432's near-miss hit exactly this shape: daml.yaml was
// DOWNGRADED to match a stale ConfigMap instead of the ConfigMap being
// regenerated, so every label lined up while the DAR held none of the PR's
// DAML changes. These tests exercise the independent git-diff signal added
// for #587: daml/** changed since the merge-base with origin/main but a
// env's DAR ConfigMaps did not.

// gitRun is defined in registers_test.go (same package) and reused here.

func TestDarSyncContentDriftLabelOnlyGap(t *testing.T) {
	root := writeDarFixture(t, "0.1.16", "0.1.16", "0.1.16")
	if err := os.MkdirAll(filepath.Join(root, "daml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "daml", "ExampleModule.daml"),
		[]byte("-- v1\nmodule ExampleModule where\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	gitRun(t, root, "init", "-q")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "main: v0.1.16 daml + DAR ConfigMaps in sync")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Labels-only-gap reproduction: substantively change daml/ source but leave
	// daml.yaml, the ConfigMaps, and EXPECTED_DAR_VERSION untouched — exactly
	// the "downgrade daml.yaml to match the stale DAR" resolution #432 took.
	// Every label still agrees at 0.1.16.
	if err := os.WriteFile(filepath.Join(root, "daml", "ExampleModule.daml"),
		[]byte("-- v2: second revision\nmodule ExampleModule where\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "pr: daml source changed, DAR/labels left alone")

	problems := darSyncProblems(root, nil)
	for _, env := range []string{"dev", "prod"} {
		want := "k8s/" + env + "/app/" + darCMPrefix() + "{1,2,3}.yaml: daml/** changed since the merge-base with origin/main but these DAR ConfigMaps did not"
		var found bool
		for _, p := range problems {
			if strings.Contains(p, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: expected content-drift problem (label-only check must not mask this), got %v", env, problems)
		}
	}
}

func TestDarSyncContentDriftClearsWhenConfigMapsRebuilt(t *testing.T) {
	root := writeDarFixture(t, "0.1.16", "0.1.16", "0.1.16")
	if err := os.MkdirAll(filepath.Join(root, "daml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "daml", "ExampleModule.daml"),
		[]byte("-- v1\nmodule ExampleModule where\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	gitRun(t, root, "init", "-q")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "main: v0.1.16 daml + DAR ConfigMaps in sync")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Same daml/ edit, but this time the ConfigMaps were actually regenerated
	// (bytes differ, even though the embedded version string is unchanged —
	// e.g. a rebuild that nets to the same version). The content-drift check
	// must NOT fire once the ConfigMaps moved too.
	if err := os.WriteFile(filepath.Join(root, "daml", "ExampleModule.daml"),
		[]byte("-- v2: second revision\nmodule ExampleModule where\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, env := range []string{"dev", "prod"} {
		p1 := filepath.Join(root, darConfigMapPart(darCMPrefix(), env, 1))
		raw, err := os.ReadFile(p1)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p1, append(raw, []byte("# rebuilt\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "pr: daml source changed AND DAR ConfigMaps regenerated")

	problems := darSyncProblems(root, nil)
	for _, p := range problems {
		if strings.Contains(p, "issue #587") {
			t.Fatalf("content-drift problem must clear once ConfigMaps are rebuilt, got %v", problems)
		}
	}
}

func TestDarSyncContentDriftNoGitCheckoutIsOutOfScope(t *testing.T) {
	// writeDarFixture's tempdir has no .git — the content-drift signal must be
	// silently out of scope there (mirrors deletedRegisterFiles/grandfatheredIDs),
	// same as the pre-existing label check's own no-.git behavior.
	root := writeDarFixture(t, "0.1.16", "0.1.16", "0.1.16")
	if err := os.MkdirAll(filepath.Join(root, "daml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "daml", "ExampleModule.daml"), []byte("module ExampleModule where\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if problems := darSyncProblems(root, nil); len(problems) != 0 {
		t.Fatalf("no-.git tempdir must produce no content-drift problems, got %v", problems)
	}
}

// ----- Job-name-bump enforcement (issue #465) -----
//
// The example-deploy Job's spec.template is immutable in Kubernetes: changing
// the spec/env/init/scripts without bumping the Job name (example-deploy-v<N>)
// will fail at deploy time because the existing Job cannot be updated in-place.
// These tests exercise the check that detects a content change without a name
// bump.

// writeJobBumpFixture creates a git repo with a minimal example-deploy.yaml in
// both envs, commits it as "main", and sets the origin/main ref. Returns the
// repo root. The Job name defaults to example-deploy-v31.
func writeJobBumpFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, env := range []string{"dev", "prod"} {
		dir := filepath.Join(root, "k8s", env, "app")
		mustWriteFile(t, filepath.Join(dir, "example-deploy.yaml"), deployJobYAML(""))
		// Also add a example-deploy-scripts.yaml to prove it doesn't cause false
		// positives when the scripts change but the Job name doesn't.
		mustWriteFile(t, filepath.Join(dir, "example-deploy-scripts.yaml"),
			"---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example-deploy-scripts\ndata:\n  seed.sh: |\n    echo hello\n")
	}
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "main: initial example-deploy-v31")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	return root
}

func TestJobNameBumpNoChange(t *testing.T) {
	// No example-deploy files changed vs merge-base → no problems.
	root := writeJobBumpFixture(t)
	if problems, _ := jobNameBumpProblems(root); len(problems) != 0 {
		t.Fatalf("expected no problems, got %v", problems)
	}
}

func TestJobNameBumpNameNotBumped(t *testing.T) {
	// Edit the deploy Job spec without bumping the name → PROBLEM.
	root := writeJobBumpFixture(t)
	for _, env := range []string{"dev", "prod"} {
		p := filepath.Join(root, "k8s", env, "app", "example-deploy.yaml")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		// Append an env var — same Job name, but spec changed.
		updated := strings.Replace(string(raw),
			"- name: VERSION\n              value: \"321d7a8\"\n",
			"- name: VERSION\n              value: \"321d7a8\"\n            - name: NEW_VAR\n              value: \"changed\"\n",
			1)
		if err := os.WriteFile(p, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "pr: edit deploy Job without bumping name")

	problems, _ := jobNameBumpProblems(root)
	if len(problems) != 2 {
		t.Fatalf("expected 2 problems (one per env), got %d: %v", len(problems), problems)
	}
	for _, p := range problems {
		if !strings.Contains(p, "Job name was not bumped") {
			t.Fatalf("expected 'Job name was not bumped' in problem, got: %s", p)
		}
	}
}

func TestJobNameBumpNameBumped(t *testing.T) {
	// Edit the deploy Job spec AND bump the name → no problem.
	root := writeJobBumpFixture(t)
	for _, env := range []string{"dev", "prod"} {
		p := filepath.Join(root, "k8s", env, "app", "example-deploy.yaml")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		// Bump the name from v31 to v32 AND add an env var.
		updated := strings.Replace(string(raw), "example-deploy-v31", "example-deploy-v32", 1)
		updated = strings.Replace(updated,
			"- name: VERSION\n              value: \"321d7a8\"\n",
			"- name: VERSION\n              value: \"321d7a8\"\n            - name: NEW_VAR\n              value: \"changed\"\n",
			1)
		if err := os.WriteFile(p, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "pr: edit deploy Job AND bump name to v32")

	if problems, _ := jobNameBumpProblems(root); len(problems) != 0 {
		t.Fatalf("expected no problems (name was bumped), got %v", problems)
	}
}

func TestJobNameBumpNoGitCheckout(t *testing.T) {
	// A tree without .git must silently skip the check — no false alarm.
	root := t.TempDir()
	dir := filepath.Join(root, "k8s", "dev", "app")
	mustWriteFile(t, filepath.Join(dir, "example-deploy.yaml"), deployJobYAML(""))
	if problems, _ := jobNameBumpProblems(root); len(problems) != 0 {
		t.Fatalf("expected no problems without git, got %v", problems)
	}
}

func TestJobNameBumpNewFile(t *testing.T) {
	// A example-deploy.yaml that exists only at HEAD (wasn't at merge-base)
	// is implicitly a bump → no problem.
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	// Commit an unrelated file so there IS a merge-base.
	mustWriteFile(t, filepath.Join(root, "README.md"), "# repo\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "main: initial")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Now add the example-deploy.yaml as a new file.
	dir := filepath.Join(root, "k8s", "dev", "app")
	mustWriteFile(t, filepath.Join(dir, "example-deploy.yaml"), deployJobYAML(""))
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "pr: add example-deploy Job")

	if problems, _ := jobNameBumpProblems(root); len(problems) != 0 {
		t.Fatalf("expected no problems (new file = implicit bump), got %v", problems)
	}
}

func TestJobNameBumpScriptsChangedNameBumped(t *testing.T) {
	// Changing example-deploy-scripts.yaml AND bumping the Job name → no problem.
	root := writeJobBumpFixture(t)
	for _, env := range []string{"dev", "prod"} {
		// Change the scripts ConfigMap.
		sp := filepath.Join(root, "k8s", env, "app", "example-deploy-scripts.yaml")
		raw, err := os.ReadFile(sp)
		if err != nil {
			t.Fatal(err)
		}
		updated := strings.Replace(string(raw), "echo hello", "echo hello\necho world", 1)
		if err := os.WriteFile(sp, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
		// Also bump the Job name.
		jp := filepath.Join(root, "k8s", env, "app", "example-deploy.yaml")
		raw, err = os.ReadFile(jp)
		if err != nil {
			t.Fatal(err)
		}
		updated = strings.Replace(string(raw), "example-deploy-v31", "example-deploy-v32", 1)
		if err := os.WriteFile(jp, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "pr: change scripts AND bump name to v32")

	if problems, _ := jobNameBumpProblems(root); len(problems) != 0 {
		t.Fatalf("expected no problems (scripts changed but name was bumped), got %v", problems)
	}
}

func TestJobNameBumpOnlyScriptsChangedNameNotBumped(t *testing.T) {
	// Changing ONLY example-deploy-scripts.yaml without bumping the Job name
	// is still a PROBLEM — the scripts are part of the deploy surface.
	root := writeJobBumpFixture(t)
	for _, env := range []string{"dev", "prod"} {
		sp := filepath.Join(root, "k8s", env, "app", "example-deploy-scripts.yaml")
		raw, err := os.ReadFile(sp)
		if err != nil {
			t.Fatal(err)
		}
		updated := strings.Replace(string(raw), "echo hello", "echo hello\necho world", 1)
		if err := os.WriteFile(sp, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "pr: change scripts without bumping name")

	problems, _ := jobNameBumpProblems(root)
	if len(problems) != 2 {
		t.Fatalf("expected 2 problems (one per env), got %d: %v", len(problems), problems)
	}
	for _, p := range problems {
		if !strings.Contains(p, "Job name was not bumped") {
			t.Fatalf("expected 'Job name was not bumped' in problem, got: %s", p)
		}
	}
}

func TestJobNameBumpNameExtractionFailSafe(t *testing.T) {
	// A example-deploy.yaml whose metadata.name is not in the expected format
	// must not crash and must not produce a false positive.
	root := writeJobBumpFixture(t)
	// Replace the Job with one that lacks a recognizable example-deploy-v<N> name.
	for _, env := range []string{"dev", "prod"} {
		p := filepath.Join(root, "k8s", env, "app", "example-deploy.yaml")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		// Remove metadata.name entirely (replace with a non-matching line).
		updated := strings.Replace(string(raw), "  name: example-deploy-v31", "  no-name: here", 1)
		if err := os.WriteFile(p, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "pr: remove metadata.name from Job")

	// The check cannot extract a name → must not flag anything (fail-safe).
	if problems, _ := jobNameBumpProblems(root); len(problems) != 0 {
		t.Fatalf("expected no problems (name extraction fail-safe), got %v", problems)
	}
}

// TestJobNameBumpViaDarSyncCheck proves the job-name-bump check is wired
// through darSyncCheck → darSyncProblems and not just callable directly.
// Deleting the append at darsync.go:374 (mutant M4 from the review) must
// turn this test red — the suite must fail when the check is disconnected
// from the gate that produces the exit code.
func TestJobNameBumpViaDarSyncCheck(t *testing.T) {
	const ver = "0.1.16"
	root := t.TempDir()
	// daml.yaml + DAR ConfigMaps matching it (so the pre-existing checks generate
	// no unrelated problems), plus a example-deploy Job per env.
	writeDarCore(t, root, ver, ver)
	for _, env := range []string{"dev", "prod"} {
		mustWriteFile(t, filepath.Join(root, "k8s", env, "app", "example-deploy.yaml"),
			deployJobYAML(ver))
	}

	gitRun(t, root, "init", "-q")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "main: initial")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Violating change: edit example-deploy.yaml without bumping the Job name.
	for _, env := range []string{"dev", "prod"} {
		p := filepath.Join(root, "k8s", env, "app", "example-deploy.yaml")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		// Append an env var — same Job name, spec changed.
		updated := strings.Replace(string(raw),
			"- name: EXPECTED_DAR_VERSION\n              # must match daml.yaml and the DAR ConfigMaps\n              value: \""+ver+"\"\n",
			"- name: EXPECTED_DAR_VERSION\n              # must match daml.yaml and the DAR ConfigMaps\n              value: \""+ver+"\"\n            - name: NEW_VAR\n              value: \"changed\"\n",
			1)
		if err := os.WriteFile(p, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "pr: edit deploy Job without bumping name")

	problems := darSyncProblems(root, nil)
	var found bool
	for _, p := range problems {
		if strings.Contains(p, "Job name was not bumped") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'Job name was not bumped' in darSyncProblems, got %v", problems)
	}
}
