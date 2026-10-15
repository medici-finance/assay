package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The cross-repository guards below are each pinned by their OWN refusal
// reason. The record they re-check is a local, editable file, and several of
// the faults would also be refused by a later layer; asserting only "an error"
// would let any one guard be deleted unnoticed. testdata/crossrepo_mutations.py
// removes each guard in turn and requires the matching case here to fail.

// crossRepoIssued prepares and stamps one valid run and returns its home, its
// loaded record and the forge that holds the stamp.
func crossRepoIssued(t *testing.T) (string, verifierLocal, *verifierForge) {
	t.Helper()
	crossRepoRoster(t)
	home := crossRepoGitFixture(t, "example-org/one", "source.txt", "source\n")
	tracking := crossRepoGitFixture(t, "example-org/tracking", "brief.md", "# Plan\n\n## Evidence\n\nPending.\n")
	if err := PrepareVerifierAttestation(home, "example-org/one", filepath.Join(tracking, "brief.md"), "gpt-6-astra", "strong"); err != nil {
		t.Fatal(err)
	}
	r, err := verifierLoad(home)
	if err != nil {
		t.Fatal(err)
	}
	trackingHome := r.TrackingHome
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(trackingHome)) })
	f := &verifierForge{actor: "example-desk[bot]"}
	if _, err := IssueVerifierAttestation(home, f); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckVerifierAttestationWithForge(home, "brief.md", f); err != nil {
		t.Fatalf("untampered run refused: %v", err)
	}
	r, _ = verifierLoad(home)
	return home, r, f
}

// regression: #2485. Landing-time re-checks in verifierCrossRepoCheck.
func TestCrossRepoCheckGuardReasons(t *testing.T) {
	const provenance = "lacks tracking/source repository, revision, checkout or home provenance"
	const isolated = "tracking checkout differs from the attested isolated home"
	const identity = "verifier repository identity differs from its binding"
	cases := map[string]struct {
		mutate func(t *testing.T, home string, r *verifierLocal)
		reason string
	}{
		"missing source repo":       {func(_ *testing.T, _ string, r *verifierLocal) { r.Binding.SourceRepo = "" }, provenance},
		"missing tracking source":   {func(_ *testing.T, _ string, r *verifierLocal) { r.Binding.TrackingSource = "" }, provenance},
		"missing tracking digest":   {func(_ *testing.T, _ string, r *verifierLocal) { r.Binding.TrackingHomeSHA256 = "" }, provenance},
		"missing tracking home":     {func(_ *testing.T, _ string, r *verifierLocal) { r.TrackingHome = "" }, provenance},
		"missing tracking checkout": {func(_ *testing.T, _ string, r *verifierLocal) { r.Binding.TrackingCheckout = "" }, provenance},
		// The record names a genuine tracking clone, with its true digest, that
		// sits INSIDE the deliverable home: only the containment test refuses it.
		// (The deliverable home itself as tracking home is refused by tracking
		// identity, since the binding's two repositories are distinct.)
		"tracking home nested in the deliverable": {func(t *testing.T, home string, r *verifierLocal) {
			nested := filepath.Join(home, "nested-tracking")
			attestGit(t, home, "clone", "-q", "--no-hardlinks", r.TrackingHome, nested)
			attestGit(t, nested, "remote", "set-url", "origin", "https://github.com/example-org/tracking.git")
			real, err := filepath.EvalSymlinks(nested)
			if err != nil {
				t.Fatal(err)
			}
			r.TrackingHome, r.Binding.TrackingHomeSHA256 = real, verifierDigest([]byte(real))
		}, isolated},
		// The tracking home is the real isolated clone, but its digest is not the
		// attested one: only the digest comparison can refuse it.
		"tracking home digest": {func(_ *testing.T, _ string, r *verifierLocal) {
			r.Binding.TrackingHomeSHA256 = verifierDigest([]byte(r.TrackingHome + "-elsewhere"))
		}, isolated},
		"deliverable origin": {func(t *testing.T, home string, _ *verifierLocal) {
			attestGit(t, home, "remote", "set-url", "origin", "https://github.com/example-org/other.git")
		}, identity},
		"tracking origin": {func(t *testing.T, _ string, r *verifierLocal) {
			attestGit(t, r.TrackingHome, "remote", "set-url", "origin", "https://github.com/example-org/other.git")
		}, identity},
		// The destination became public after preparation, while the deliverable
		// is private: the binding may no longer be published there.
		"visibility": {func(t *testing.T, _ string, _ *verifierLocal) {
			crossRepoVisibilityRoster(t, "public", "private")
		}, "cannot publish the deliverable repository binding"},
		// The attested commit, but on a branch: refs can move under a branch.
		"attached branch": {func(t *testing.T, home string, _ *verifierLocal) {
			attestGit(t, home, "checkout", "-q", "-b", "attached")
		}, "requires its attested detached source commit"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			home, r, f := crossRepoIssued(t)
			tc.mutate(t, home, &r)
			if err := verifierSave(home, r); err != nil {
				t.Fatal(err)
			}
			if _, err := CheckVerifierAttestationWithForge(home, "brief.md", f); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("not refused by %q: %v", tc.reason, err)
			}
		})
	}
}

// regression: #2485. Preparation refusals that no other layer names.
func TestCrossRepoPrepareGuardReasons(t *testing.T) {
	t.Run("run exists", func(t *testing.T) {
		home, r, _ := crossRepoIssued(t)
		tracking := crossRepoGitFixture(t, "example-org/tracking", "brief.md", "# Plan\n")
		err := PrepareVerifierAttestation(home, "example-org/one", filepath.Join(tracking, "brief.md"), "gpt-6-astra", "strong")
		if err == nil || !strings.Contains(err.Error(), "verifier run already exists") {
			t.Fatalf("second preparation not refused as replacing provenance: %v", err)
		}
		if got, _ := verifierLoad(home); got.Binding.Run != r.Binding.Run {
			t.Fatal("refused preparation replaced the run record")
		}
	})
	t.Run("authority", func(t *testing.T) {
		roster := strings.ReplaceAll(raisedByFixtureRoster, "example-desk-app", "example-desk")
		roster = strings.ReplaceAll(roster, "verifier=example-verifier-app:300000005", "verifier=example-desk:300000001")
		cfgMu.Lock()
		previous, once := cfgValue, cfgOnce
		cfgValue = parseConfig(ClassWrite, "offline fixture", parseDotenv(roster+"\nASSAY_ALLOWED_REPOS=example-org/one:ci:public,example-org/tracking:ci:private\n"))
		cfgOnce = true
		cfgMu.Unlock()
		t.Cleanup(func() { cfgMu.Lock(); cfgValue, cfgOnce = previous, once; cfgMu.Unlock() })
		home := crossRepoGitFixture(t, "example-org/one", "source.txt", "source\n")
		tracking := crossRepoGitFixture(t, "example-org/tracking", "brief.md", "# Plan\n")
		err := PrepareVerifierAttestation(home, "example-org/one", filepath.Join(tracking, "brief.md"), "gpt-6-astra", "strong")
		if err == nil || !strings.Contains(err.Error(), "distinct configured desk and verifier actors") {
			t.Fatalf("one actor for desk and verifier not refused: %v", err)
		}
	})
}

// regression: #2485. A tracking tree carrying a gitlink is refused before
// checkout: the submodule's content is not part of the attested revision.
func TestCrossRepoTrackingSubmodule(t *testing.T) {
	root := crossRepoGitFixture(t, "example-org/one", "base.txt", "base\n")
	before := attestGitOutput(t, root, "rev-parse", "HEAD")
	attestGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+before+",vendored")
	attestGit(t, root, "commit", "-m", "gitlink")
	target := attestGitOutput(t, root, "rev-parse", "HEAD")
	attestGit(t, root, "checkout", "--detach", before)
	if err := checkoutVerifierTracking(root, target); err == nil || !strings.Contains(err.Error(), "unattested submodule") {
		t.Fatalf("gitlink not refused as an unattested submodule: %v", err)
	}
	if got := attestGitOutput(t, root, "rev-parse", "HEAD"); got != before {
		t.Fatal("refusal moved tracking HEAD")
	}
}

// regression: #2485. Declaration refusals in VerifierDeliverables.
func TestCrossRepoDeclarationReasons(t *testing.T) {
	const registry = "schema: graph-repos-v1\nself: track\nrepos:\n  track:\n    repo: example-org/tracking\n  source:\n    repo: example-org/one\n"
	cases := map[string]struct{ brief, registry, reason string }{
		"registry schema":    {"---\ndeliverable_repo: source\n---\n# Plan\n", strings.Replace(registry, "graph-repos-v1", "graph-repos-v2", 1), "invalid verifier association registry"},
		"conflicting":        {"---\ndeliverable_repo: source\nhomed-in: example-org/tracking\n---\n# Plan\n", registry, "conflicting verifier deliverable declarations"},
		"unregistered homed": {"---\nhomed-in: example-org/elsewhere\n---\n# Plan\n", registry, "homed-in repository is not registered"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			attestGit(t, root, "init", "-q")
			for path, body := range map[string]string{"brief.md": tc.brief, "docs/streams/graph-repos.yaml": tc.registry} {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
				attestGit(t, root, "add", path)
			}
			attestGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "-m", "fixture")
			repos, err := VerifierDeliverables(root, attestGitOutput(t, root, "rev-parse", "HEAD"), "brief.md")
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("not refused by %q: %v %v", tc.reason, repos, err)
			}
		})
	}
}
