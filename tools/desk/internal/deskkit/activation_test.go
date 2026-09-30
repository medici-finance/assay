package deskkit

// activation_test.go pins component-model §6.1's activation rule using SYNTHETIC
// component names and keys throughout (ground rule: no house value in tests or
// fixtures) — never a real component or key from this tree's own manifests, so a
// future rename of a real component cannot make these tests pass or fail for the
// wrong reason.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// writeManifest writes a minimal component.yaml under dir/name/component.yaml —
// one file per component, the same layout deskmanifest lint discovers.
func writeManifest(t *testing.T, dir, component string, provides []string, required, optional []string) {
	t.Helper()
	sub := filepath.Join(dir, strings.ReplaceAll(component, "/", "-"))
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("component: " + component + "\n")
	b.WriteString("version: 0.0.1\n")
	b.WriteString("provides:\n")
	for _, p := range provides {
		b.WriteString("  - " + p + "\n")
	}
	if len(provides) == 0 {
		b.WriteString("  []\n")
	}
	b.WriteString("inject:\n  required:\n")
	for _, k := range required {
		b.WriteString("    - key: " + k + "\n")
	}
	if len(required) == 0 {
		b.WriteString("    []\n")
	}
	b.WriteString("  optional:\n")
	for _, k := range optional {
		b.WriteString("    - key: " + k + "\n")
	}
	if len(optional) == 0 {
		b.WriteString("    []\n")
	}
	if err := os.WriteFile(filepath.Join(sub, "component.yaml"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---- ComputeActivation: the pure engine, synthetic manifests in-memory --------

func manifestOf(component string, provides []string, required []string) componentManifest {
	m := componentManifest{Component: component, Provides: provides}
	for _, k := range required {
		m.Inject.Required = append(m.Inject.Required, manifestInjectKey{Key: k})
	}
	return m
}

// TestComputeActivation_ExtensionKeyInvalidDeactivatesOnlyDependents is Task item 4's
// core case: synthetic manifest A injects assay.roster.ext.x (required), B does not.
// ext.x invalid deactivates ONLY A.
func TestComputeActivation_ExtensionKeyInvalidDeactivatesOnlyDependents(t *testing.T) {
	a := manifestOf("test/a", nil, []string{"assay.roster.trust", "assay.roster.ext.x"})
	b := manifestOf("test/b", nil, []string{"assay.roster.trust"})
	manifests := []componentManifest{a, b}

	ext := map[string]ExtKeyResult{"x": {Status: ExtInvalid, Reason: "TEST_KEY: synthetic rejection"}}
	results := ComputeActivation(manifests, ext, true /* trustConfigured */)

	if results["test/a"].Active {
		t.Fatal("test/a requires an INVALID extension key and must be INACTIVE")
	}
	if got := results["test/a"].Key; got != "assay.roster.ext.x" {
		t.Errorf("test/a's unresolved key = %q, want assay.roster.ext.x", got)
	}
	if !strings.Contains(results["test/a"].Reason, "TEST_KEY") {
		t.Errorf("test/a's reason does not carry the recorded ext reason: %q", results["test/a"].Reason)
	}
	if !results["test/b"].Active {
		t.Fatalf("test/b does not inject the invalid key and must stay ACTIVE: %+v", results["test/b"])
	}
}

// TestComputeActivation_ExtensionKeyUnsetOrOKResolves: unset and ok both resolve a
// required extension key (component-model §6.1's "unset is a complete configuration").
func TestComputeActivation_ExtensionKeyUnsetOrOKResolves(t *testing.T) {
	a := manifestOf("test/a", nil, []string{"assay.roster.trust", "assay.roster.ext.x"})
	manifests := []componentManifest{a}

	for name, ext := range map[string]map[string]ExtKeyResult{
		"unset":           {"x": {Status: ExtUnset}},
		"ok":              {"x": {Status: ExtOK}},
		"absent from ext": {},
	} {
		t.Run(name, func(t *testing.T) {
			results := ComputeActivation(manifests, ext, true)
			if !results["test/a"].Active {
				t.Fatalf("%s: test/a = %+v, want Active", name, results["test/a"])
			}
		})
	}
}

// TestComputeActivation_TrustUnsetRefusesEveryComponentThatRequiresIt is Task item 4's
// second case: an unset/malformed trust surface leaves EVERY component that requires
// assay.roster.trust INACTIVE, regardless of what else they inject — the trust
// surface does not change (component-model §6.2).
func TestComputeActivation_TrustUnsetRefusesEveryComponentThatRequiresIt(t *testing.T) {
	a := manifestOf("test/a", nil, []string{"assay.roster.trust", "assay.roster.ext.x"})
	b := manifestOf("test/b", nil, []string{"assay.roster.trust"})
	manifests := []componentManifest{a, b}

	ext := map[string]ExtKeyResult{"x": {Status: ExtOK}} // extension is FINE
	results := ComputeActivation(manifests, ext, false /* trust NOT configured */)

	if results["test/a"].Active {
		t.Error("test/a requires assay.roster.trust and trust is unconfigured — must be INACTIVE")
	}
	if results["test/b"].Active {
		t.Error("test/b requires assay.roster.trust and trust is unconfigured — must be INACTIVE")
	}
	for name, r := range results {
		if r.Key != "assay.roster.trust" {
			t.Errorf("%s: unresolved key = %q, want assay.roster.trust", name, r.Key)
		}
	}
}

// TestComputeActivation_TransitiveProvider: a component requiring a NON-roster key
// resolves iff the component that provides it is itself ACTIVE.
func TestComputeActivation_TransitiveProvider(t *testing.T) {
	provider := manifestOf("test/provider", []string{"test.thing"}, []string{"assay.roster.trust"})
	dependent := manifestOf("test/dependent", nil, []string{"test.thing"})
	manifests := []componentManifest{provider, dependent}

	t.Run("provider active makes dependent active", func(t *testing.T) {
		results := ComputeActivation(manifests, nil, true)
		if !results["test/provider"].Active || !results["test/dependent"].Active {
			t.Fatalf("both should be ACTIVE: %+v", results)
		}
	})
	t.Run("provider inactive (trust unset) cascades to dependent", func(t *testing.T) {
		results := ComputeActivation(manifests, nil, false)
		if results["test/provider"].Active {
			t.Fatal("provider requires trust and trust is unset — must be INACTIVE")
		}
		if results["test/dependent"].Active {
			t.Fatalf("dependent's only provider is INACTIVE — dependent must cascade to INACTIVE: %+v",
				results["test/dependent"])
		}
		if !strings.Contains(results["test/dependent"].Reason, "test/provider") {
			t.Errorf("dependent's reason does not name the inactive provider: %q", results["test/dependent"].Reason)
		}
	})
}

// TestComputeActivation_DuplicateIDFirstWins pins ComputeActivation's by-name
// index to FIRST-declaration-wins, matching its providers index. An in-memory
// caller can pass two manifests with one id (DiscoverManifests already drops the
// later one); the first must decide the component's state. Here the second copy
// requires an INVALID extension key, so a last-wins index turns it INACTIVE.
func TestComputeActivation_DuplicateIDFirstWins(t *testing.T) {
	first := manifestOf("test/dup", nil, []string{"assay.roster.trust"})
	second := manifestOf("test/dup", nil, []string{"assay.roster.trust", "assay.roster.ext.x"})
	ext := map[string]ExtKeyResult{"x": {Status: ExtInvalid, Reason: "TEST_KEY: synthetic rejection"}}
	results := ComputeActivation([]componentManifest{first, second}, ext, true)
	if !results["test/dup"].Active {
		t.Fatalf("test/dup = %+v, want ACTIVE: the FIRST declaration must win, not the later duplicate", results["test/dup"])
	}
}

// TestComputeActivation_NoProviderIsInactive: a required key with NO declared
// provider anywhere fails closed rather than being assumed fine.
func TestComputeActivation_NoProviderIsInactive(t *testing.T) {
	a := manifestOf("test/a", nil, []string{"test.nothing-provides-this"})
	results := ComputeActivation([]componentManifest{a}, nil, true)
	if results["test/a"].Active {
		t.Fatal("a key with no provider anywhere must leave the component INACTIVE")
	}
}

// TestComputeActivation_CycleLeavesBothInactive pins §6.1's cycle rule: two
// components each requiring a key only the OTHER provides leaves BOTH inactive,
// never a stack overflow and never one arbitrarily "winning".
func TestComputeActivation_CycleLeavesBothInactive(t *testing.T) {
	a := manifestOf("test/a", []string{"test.a-key"}, []string{"test.b-key"})
	b := manifestOf("test/b", []string{"test.b-key"}, []string{"test.a-key"})
	results := ComputeActivation([]componentManifest{a, b}, nil, true)
	if results["test/a"].Active || results["test/b"].Active {
		t.Fatalf("a cycle must leave every member INACTIVE: %+v", results)
	}
}

// ---- DiscoverManifests: real files on disk -------------------------------------

func TestDiscoverManifests_FindsAndSkips(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "test/good", []string{"test.thing"}, []string{"assay.roster.trust"}, nil)
	// A broken manifest: unparseable YAML, must be SKIPPED not fatal.
	bad := filepath.Join(dir, "test-bad")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "component.yaml"), []byte("not: [valid: yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A .git directory must be skipped entirely, matching cmd/deskmanifest's discover().
	gitDir := filepath.Join(dir, ".git", "test-in-git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "component.yaml"), []byte("component: test/in-git\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifests, skipped, err := DiscoverManifests(dir)
	if err != nil {
		t.Fatalf("DiscoverManifests: %v", err)
	}
	if len(manifests) != 1 || manifests[0].Component != "test/good" {
		t.Fatalf("manifests = %+v, want exactly test/good", manifests)
	}
	if len(skipped) != 1 || !strings.Contains(skipped[0], "test-bad") {
		t.Fatalf("skipped = %v, want the unparseable manifest named", skipped)
	}
}

func TestDiscoverManifests_MissingRootIsError(t *testing.T) {
	if _, _, err := DiscoverManifests(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("a missing root must error, not return an empty clean result")
	}
}

// ---- VerbComponent / VerbActivationRefusal / CheckVerbActivation ---------------

func TestVerbComponent_ResolvesTheProviderOfAssayDeskVerbs(t *testing.T) {
	manifests := []componentManifest{
		manifestOf("test/other", []string{"test.something"}, nil),
		manifestOf("test/desk-tools", []string{"assay.desk.verbs"}, []string{"assay.roster.trust"}),
	}
	if got := VerbComponent(manifests); got != "test/desk-tools" {
		t.Fatalf("VerbComponent = %q, want test/desk-tools", got)
	}
}

func TestVerbComponent_EmptyWhenNoneProvidesDeskVerbs(t *testing.T) {
	manifests := []componentManifest{manifestOf("test/other", []string{"test.something"}, nil)}
	if got := VerbComponent(manifests); got != "" {
		t.Fatalf("VerbComponent = %q, want empty (no owning component)", got)
	}
}

// TestVerbActivationRefusal_ExtensionKeyInvalid is the end-to-end row-8/row-2 shape
// Task item 4 asks for, driven off REAL files on disk and the REAL EffectiveConfig()
// (via withRoster), exercising the exact message shape the brief's Verify table names:
// "could-not-check: assay/<component> inactive — <key> <reason>".
func TestVerbActivationRefusal_ExtensionKeyInvalid(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "test/desk-tools", []string{"assay.desk.verbs"},
		[]string{"assay.roster.trust", "assay.roster.ext.repo-aliases"}, nil)

	r := goldenRoster()
	r[EnvRepoAliases] = "not=a=valid=shape" // malformed — see repoalias_test.go's shapes
	withRoster(t, r)

	if !EffectiveConfig().Configured() {
		t.Fatalf("precondition: trust must be configured here — this test is about the EXTENSION "+
			"path, not the trust one: %v", EffectiveConfig().Problems)
	}

	msg := VerbActivationRefusal(dir)
	if msg == "" {
		t.Fatal("VerbActivationRefusal returned \"\" — an invalid required extension key must refuse")
	}
	if !strings.HasPrefix(msg, "could-not-check: test/desk-tools inactive — assay.roster.ext.repo-aliases ") {
		t.Fatalf("refusal shape = %q, want the could-not-check: <component id> inactive — <key> <reason> form", msg)
	}
}

// TestVerbActivationRefusal_ActiveWhenExtensionKeyValid is the row-1 shape: a
// component that does not inject the broken key (or, as here, the SAME component but
// with a VALID value) proceeds.
func TestVerbActivationRefusal_ActiveWhenExtensionKeyValid(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "test/desk-tools", []string{"assay.desk.verbs"},
		[]string{"assay.roster.trust", "assay.roster.ext.repo-aliases"}, nil)

	r := goldenRoster()
	r[EnvRepoAliases] = "example-org/example=short:product" // VALID shape
	withRoster(t, r)

	if msg := VerbActivationRefusal(dir); msg != "" {
		t.Fatalf("VerbActivationRefusal = %q, want \"\" (a valid extension value resolves it)", msg)
	}
}

// TestVerbActivationRefusal_TrustUnsetRefuses is Task item 4's trust-key-unset case
// at the VerbActivationRefusal layer: unset trust refuses regardless of extension state.
func TestVerbActivationRefusal_TrustUnsetRefuses(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "test/desk-tools", []string{"assay.desk.verbs"},
		[]string{"assay.roster.trust"}, nil)

	withRoster(t, map[string]string{}) // nothing configured at all

	msg := VerbActivationRefusal(dir)
	if msg == "" {
		t.Fatal("VerbActivationRefusal returned \"\" with an unconfigured trust surface — must refuse")
	}
	if !strings.Contains(msg, "assay.roster.trust") {
		t.Errorf("refusal does not name assay.roster.trust: %q", msg)
	}
}

// TestVerbActivationRefusal_NoOwningComponentProceeds pins the facts note: a verb with
// no owning component in the discovered manifests is treated as trust-only and this
// mechanism never refuses it on its own (trust itself is still enforced by every write
// path's own RosterUnconfiguredError()).
func TestVerbActivationRefusal_NoOwningComponentProceeds(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "test/unrelated", []string{"test.something-else"}, nil, nil)
	withRoster(t, map[string]string{}) // trust unconfigured too — still must not refuse HERE
	if msg := VerbActivationRefusal(dir); msg != "" {
		t.Fatalf("VerbActivationRefusal = %q, want \"\" — no manifest provides assay.desk.verbs here", msg)
	}
}

// TestVerbActivationRefusal_UndiscoverableRootProceeds: a root that does not exist (a
// verb run outside any checkout) must degrade to "no activation information", never a
// refusal the mechanism cannot see the reason for.
func TestVerbActivationRefusal_UndiscoverableRootProceeds(t *testing.T) {
	if msg := VerbActivationRefusal(filepath.Join(t.TempDir(), "does-not-exist")); msg != "" {
		t.Fatalf("VerbActivationRefusal = %q, want \"\" for an undiscoverable root", msg)
	}
}

// TestCheckVerbActivation_PrintsAndReportsFalseWhenInactive exercises the actual call
// site every desk command's main() makes, with root injected via the process's own
// working directory (os.Chdir into a synthetic tree — the only seam CheckVerbActivation
// exposes, since it resolves its own root via gitcore.Toplevel(cwd)).
func TestCheckVerbActivation_PrintsAndReportsFalseWhenInactive(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, dir, "test/desk-tools", []string{"assay.desk.verbs"},
		[]string{"assay.roster.trust"}, nil)
	withRoster(t, map[string]string{})

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if CheckVerbActivation(&buf) {
		t.Fatal("CheckVerbActivation = true with an unconfigured trust surface and an owning manifest — want false")
	}
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "could-not-check:") {
		t.Fatalf("printed message = %q, want a could-not-check: line", buf.String())
	}
}

// TestVerbRefusal_NamespacedIDNotDoubled pins the refusal against a component id
// that already carries the assay/ namespace — the shape every real manifest uses,
// which the test/-prefixed fixtures above could not catch. The refusal names the
// id verbatim: `assay/example-verbs`, never `assay/assay/example-verbs`.
func TestVerbRefusal_NamespacedIDNotDoubled(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "assay/example-verbs", []string{"assay.desk.verbs"},
		[]string{"assay.roster.trust", "assay.roster.ext.repo-aliases"}, nil)

	r := goldenRoster()
	r[EnvRepoAliases] = "not=a=valid=shape"
	withRoster(t, r)

	msg := VerbActivationRefusal(dir)
	want := "could-not-check: assay/example-verbs inactive — assay.roster.ext.repo-aliases "
	if !strings.HasPrefix(msg, want) {
		t.Fatalf("refusal = %q, want prefix %q", msg, want)
	}
	if strings.Contains(msg, "assay/assay/") {
		t.Fatalf("refusal doubles the namespace: %q", msg)
	}
}

// doubledNamespace matches a Go string literal that prepends the assay/ namespace
// to a formatted or concatenated value — the shape that printed
// `assay/assay/<id>` when the value was a component id that already carries it.
var doubledNamespace = regexp.MustCompile(`"assay/(%[sqv]|"\s*\+)`)

// TestNoNamespacePrefixedFormatting is the CLASS guard for the doubled prefix:
// no non-test Go source in tools/desk formats a value behind a literal assay/
// prefix. Component ids carry their own namespace; print them verbatim. The
// allow-list is empty. A positive control proves the matcher still matches.
func TestNoNamespacePrefixedFormatting(t *testing.T) {
	for _, plant := range []string{`fmt.Sprintf("assay/%s", id)`, `"assay/" + id`} {
		if !doubledNamespace.MatchString(plant) {
			t.Fatalf("positive control: matcher no longer flags %s", plant)
		}
	}
	root := filepath.Join("..", "..") // tools/desk
	var hits []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for i, line := range strings.Split(string(data), "\n") {
			if doubledNamespace.MatchString(line) {
				hits = append(hits, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("could-not-check: walking %s: %v", root, err)
	}
	if len(hits) > 0 {
		t.Fatalf("namespace-prefixed formatting (print the id verbatim):\n%s", strings.Join(hits, "\n"))
	}
}

// TestDiscoverManifests_SkipsNestedCheckouts: a nested clone (.git directory) or
// linked worktree (.git file) below root is another checkout. Its manifests must
// not change this tree's activation — here each nested copy requires a key that
// would deactivate the verbs component if it were read. The nested directories
// are named to sort BEFORE the root's own manifest directory (`a-…` <
// `assay-example-verbs/`), so a nested copy that is read wins the duplicate-id
// tie-break and flips activation: the activation assertion proves the skip on
// its own, independent of the skipped-list assertion. Each arm (the .git
// directory and the .git FILE) is pinned by name in skipped.
func TestDiscoverManifests_SkipsNestedCheckouts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err) // root's own .git is the normal case and must not skip root
	}
	writeManifest(t, root, "assay/example-verbs", []string{"assay.desk.verbs"},
		[]string{"assay.roster.trust"}, nil)
	for _, nested := range []struct {
		dir    string
		gitDir bool
	}{{"a-clone", true}, {"a-wt", false}} {
		sub := filepath.Join(root, nested.dir)
		writeManifest(t, sub, "assay/example-verbs", []string{"assay.desk.verbs"},
			[]string{"assay.roster.trust", "assay.roster.ext.repo-aliases"}, nil)
		var err error
		if nested.gitDir {
			err = os.MkdirAll(filepath.Join(sub, ".git"), 0o755)
		} else {
			err = os.WriteFile(filepath.Join(sub, ".git"), []byte("gitdir: /elsewhere\n"), 0o644)
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	ms, skipped, err := DiscoverManifests(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].path != "assay-example-verbs/component.yaml" {
		t.Fatalf("manifests = %+v, want only the root tree's one", ms)
	}
	wantSkipped := []string{
		"a-clone: nested checkout (carries .git), not this tree",
		"a-wt: nested checkout (carries .git), not this tree",
	}
	if strings.Join(skipped, "\n") != strings.Join(wantSkipped, "\n") {
		t.Fatalf("skipped = %q, want exactly %q (the .git-directory AND the .git-file arm)", skipped, wantSkipped)
	}

	r := goldenRoster()
	r[EnvRepoAliases] = "not=a=valid=shape"
	withRoster(t, r)
	if msg := VerbActivationRefusal(root); msg != "" {
		t.Fatalf("a nested checkout's manifest changed activation: %q", msg)
	}
}

// TestDiscoverManifests_DuplicateIDDeterministic: two manifests declaring one
// component id resolve to the one at the lexicographically first path, every
// time, and the other is named in skipped.
func TestDiscoverManifests_DuplicateIDDeterministic(t *testing.T) {
	root := t.TempDir()
	write := func(sub string, required []string) {
		dir := filepath.Join(root, sub)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "component: assay/example-dup\nversion: 0.0.1\nprovides: [assay.desk.verbs]\ninject:\n  required:\n"
		for _, k := range required {
			body += "    - key: " + k + "\n"
		}
		if err := os.WriteFile(filepath.Join(dir, "component.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("b", []string{"assay.roster.trust", "assay.roster.ext.repo-aliases"})
	write("a", []string{"assay.roster.trust"})

	for i := 0; i < 20; i++ {
		ms, skipped, err := DiscoverManifests(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(ms) != 1 || ms[0].path != "a/component.yaml" {
			t.Fatalf("run %d: manifests = %+v, want only a/component.yaml", i, ms)
		}
		if len(skipped) != 1 || !strings.HasPrefix(skipped[0], "b/component.yaml: duplicate component id") {
			t.Fatalf("run %d: skipped = %q, want b/component.yaml named as the duplicate", i, skipped)
		}
	}
}
