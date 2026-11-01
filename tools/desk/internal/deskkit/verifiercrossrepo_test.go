package deskkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The fixture injects only the parsed roster and forge transport. Git paths,
// commits, source closure, local records and dispatcher stamp checks are real.
// No machine home or credential configuration is changed.
func crossRepoRoster(t *testing.T) {
	t.Helper()
	crossRepoVisibilityRoster(t, "private", "public")
}

func crossRepoVisibilityRoster(t *testing.T, tracking, source string) {
	t.Helper()
	cfgMu.Lock()
	previous, once := cfgValue, cfgOnce
	cfgValue = parseConfig(ClassWrite, "offline fixture", parseDotenv(strings.ReplaceAll(raisedByFixtureRoster, "example-desk-app", "example-desk")+"\nASSAY_ALLOWED_REPOS=example-org/one:ci:"+source+",example-org/tracking:ci:"+tracking+"\n"))
	cfgOnce = true
	cfgMu.Unlock()
	t.Cleanup(func() { cfgMu.Lock(); cfgValue, cfgOnce = previous, once; cfgMu.Unlock() })
}

func crossRepoGitFixture(t *testing.T, repo, name, body string) string {
	t.Helper()
	root := t.TempDir()
	if repo == "example-org/tracking" {
		body = "---\nhomed-in: example-org/one\n---\n" + body
		reg := filepath.Join(root, "docs/streams/graph-repos.yaml")
		if err := os.MkdirAll(filepath.Dir(reg), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(reg, []byte("schema: graph-repos-v1\nself: track\nrepos:\n  track:\n    repo: example-org/tracking\n  source:\n    repo: example-org/one\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	attestGit(t, root, "init")
	attestGit(t, root, "config", "user.name", "fixture")
	attestGit(t, root, "config", "user.email", "fixture@example.invalid")
	attestGit(t, root, "remote", "add", "origin", "https://github.com/"+repo+".git")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	attestGit(t, root, "add", name)
	if repo == "example-org/tracking" {
		attestGit(t, root, "add", "docs/streams/graph-repos.yaml")
	}
	attestGit(t, root, "commit", "-m", "fixture")
	attestGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	attestGit(t, root, "checkout", "--detach")
	return root
}

// regression: #2485
func TestVerifierCrossRepoRoundTrip(t *testing.T) {
	crossRepoRoster(t)
	home := crossRepoGitFixture(t, "example-org/one", "source.txt", "observed deliverable\n")
	brief := "docs/streams/sample/brief-01-check.md"
	tracking := crossRepoGitFixture(t, "example-org/tracking", brief, "# Private fixture canary\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `test -f source.txt` | exit 0 |\n\n## Evidence\n\nPending.\n")
	if err := PrepareVerifierAttestation(home, "example-org/one", filepath.Join(tracking, brief), "gpt-6-astra", "strong"); err != nil {
		t.Fatalf("two-repository pre-work binding: %v", err)
	}
	r, err := verifierLoad(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(r.TrackingHome)) })
	f := &crossRepoForge{verifierForge: verifierForge{actor: "example-desk[bot]"}}
	if _, err := CheckVerifierAttestationWithForge(home, brief, f); err == nil {
		t.Fatal("pending run admitted")
	}
	f.loseCreate = true
	if _, err := IssueVerifierAttestation(home, f); err == nil {
		t.Fatal("lost create response reported success")
	}
	failed, _ := verifierLoad(home)
	if failed.Binding.Run != r.Binding.Run || failed.TrackingHome != r.TrackingHome {
		t.Fatal("failed create replaced provenance")
	}
	f.loseCreate = false
	receipt, err := IssueVerifierAttestation(home, f)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Binding.Version != 2 || receipt.Binding.Repo != "example-org/tracking" || receipt.Binding.SourceRepo != "example-org/one" || receipt.BriefRoot != r.TrackingHome || f.creates != 1 {
		t.Fatalf("wrong recovered association: %+v creates=%d", receipt, f.creates)
	}
	for _, destination := range f.destinations {
		if destination != "example-org/tracking" {
			t.Fatalf("private attestation reached %s", destination)
		}
	}
	if strings.Contains(f.issue.Body, "Private fixture canary") || strings.Contains(f.issue.Body, tracking) || strings.Contains(f.issue.Body, home) {
		t.Fatal("attestation exposed local path or brief body")
	}
	if _, err := os.Stat(filepath.Join(home, brief)); !os.IsNotExist(err) {
		t.Fatal("tracking brief copied into deliverable")
	}
	if _, err := CheckVerifierAttestationWithForge(home, filepath.Join(r.TrackingHome, brief), f); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckVerifierEvidenceWithForge(home, "example-org/one", brief, f); err == nil {
		t.Fatal("Evidence could land in deliverable repo")
	}
	content, _ := os.ReadFile(filepath.Join(r.TrackingHome, brief))
	content = append(content, []byte("\n"+receipt.EvidenceBinding()+"\n")...)
	if err := os.WriteFile(filepath.Join(r.TrackingHome, brief), content, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckVerifierEvidenceWithForge(home, "example-org/tracking", brief, f); err != nil {
		t.Fatal(err)
	}
	if err := receipt.CheckEvidenceContent(brief, content, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(tracking, brief)); strings.Contains(string(got), receipt.EvidenceBinding()) {
		t.Fatal("shared tracking source was edited")
	}
}

type crossRepoForge struct {
	verifierForge
	destinations []string
}

func (f *crossRepoForge) FileIssue(repo ForgeRepo, in IssueInput) (*IssueRef, error) {
	f.destinations = append(f.destinations, repo.Slug())
	return f.verifierForge.FileIssue(repo, in)
}
func (f *crossRepoForge) GetIssueTyped(repo ForgeRepo, n int, k TargetKind) (*Issue, error) {
	f.destinations = append(f.destinations, repo.Slug())
	return f.verifierForge.GetIssueTyped(repo, n, k)
}

// regression: #2485
func TestVerifierCrossRepoTampering(t *testing.T) {
	cases := map[string]func(*testing.T, string, *verifierLocal, *verifierForge){
		"source bytes": func(t *testing.T, home string, _ *verifierLocal, _ *verifierForge) {
			os.WriteFile(filepath.Join(home, "source.txt"), []byte("changed"), 0600)
		},
		"tracking plan": func(t *testing.T, _ string, r *verifierLocal, _ *verifierForge) {
			os.WriteFile(filepath.Join(r.TrackingHome, r.Binding.Brief), []byte("new plan"), 0600)
		},
		"source repo": func(t *testing.T, _ string, r *verifierLocal, _ *verifierForge) {
			r.Binding.SourceRepo = r.Binding.Repo
		},
		"tracking repo": func(t *testing.T, _ string, r *verifierLocal, _ *verifierForge) {
			r.Binding.Repo = r.Binding.SourceRepo
		},
		"source revision": func(t *testing.T, _ string, r *verifierLocal, _ *verifierForge) {
			r.Binding.Source = strings.Repeat("a", 40)
		},
		"tracking revision": func(t *testing.T, _ string, r *verifierLocal, _ *verifierForge) {
			r.Binding.TrackingSource = strings.Repeat("a", 40)
		},
		"missing provenance": func(t *testing.T, _ string, r *verifierLocal, _ *verifierForge) { r.Binding.TrackingSource = "" },
		"tracking home":      func(t *testing.T, home string, r *verifierLocal, _ *verifierForge) { r.TrackingHome = home },
		"nonce": func(t *testing.T, _ string, r *verifierLocal, _ *verifierForge) {
			r.Binding.Run = strings.Repeat("a", 64)
		},
		"model":             func(t *testing.T, _ string, r *verifierLocal, _ *verifierForge) { r.Binding.Model = "gpt-6.1-sol" },
		"brief path escape": func(t *testing.T, _ string, r *verifierLocal, _ *verifierForge) { r.Binding.Brief = "../brief.md" },
		"missing history":   func(t *testing.T, _ string, _ *verifierLocal, f *verifierForge) { f.missingHistory = true },
		"unreadable stamp":  func(t *testing.T, _ string, _ *verifierLocal, f *verifierForge) { f.unreadable = true },
		"unverified stamp":  func(t *testing.T, _ string, _ *verifierLocal, f *verifierForge) { f.events = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			crossRepoRoster(t)
			home := crossRepoGitFixture(t, "example-org/one", "source.txt", "source\n")
			tracking := crossRepoGitFixture(t, "example-org/tracking", "brief.md", "# Plan\n\n## Evidence\n\nPending.\n")
			if err := PrepareVerifierAttestation(home, "example-org/one", filepath.Join(tracking, "brief.md"), "gpt-6-astra", "strong"); err != nil {
				t.Fatal(err)
			}
			r, _ := verifierLoad(home)
			trackingHome := r.TrackingHome
			t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(trackingHome)) })
			f := &verifierForge{actor: "example-desk[bot]"}
			if _, err := IssueVerifierAttestation(home, f); err != nil {
				t.Fatal(err)
			}
			r, _ = verifierLoad(home)
			mutate(t, home, &r, f)
			if err := verifierSave(home, r); err != nil {
				t.Fatal(err)
			}
			if _, err := CheckVerifierAttestationWithForge(home, "brief.md", f); err == nil {
				t.Fatal("tampered run admitted")
			}
			if _, err := CheckVerifierEvidenceWithForge(home, "example-org/tracking", "brief.md", f); err == nil {
				t.Fatal("tampered Evidence admitted")
			}
		})
	}
}

// regression: #2485
// regression: #2798 — the synthetic receipt's brief key/path follow the writer contract.
func TestCrossRepoWakeManifest(t *testing.T) {
	crossRepoRoster(t)
	source := crossRepoGitFixture(t, "example-org/one", "source.txt", "source\n")
	const brief = "docs/streams/sample/brief-01-plan.md"
	tracking := crossRepoGitFixture(t, "example-org/tracking", brief, "# Plan\n")
	sha, _ := verifierGit(source, "rev-parse", "HEAD")
	t.Setenv(RootsEnv, "example-org/one="+source)
	reader := NewRootRevisionReader(tracking, "test-tool")
	originalBrief, err := os.ReadFile(filepath.Join(tracking, brief))
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]string{InputKeyRepoSourcePrefix + "example-org/one": sha, "file:" + brief: Sha256Hex(originalBrief), "tool": "test-tool"}
	r := NewWakeReceipt("fixture", "example-org/tracking", "sample/01", "verifier", "verify-fail", sha, nil, input, "test-tool", "implementation", "#1", WakeRelevantInputChanged, "", "", "2026-10-13T20:00:00Z")
	// Use the input reader directly: a blocker forge has no role in source identity.
	if state, why := Unchanged(r, reader); state != InputsUnchanged {
		t.Fatalf("unchanged: %v %s", state, why)
	}
	os.WriteFile(filepath.Join(tracking, brief), []byte("# Changed plan\n"), 0600)
	if state, _ := Unchanged(r, reader); state != InputsChanged {
		t.Fatal("tracking change did not wake")
	}
	os.WriteFile(filepath.Join(tracking, brief), originalBrief, 0600)
	os.WriteFile(filepath.Join(source, "source.txt"), []byte("dirty\n"), 0600)
	if state, _ := Unchanged(r, reader); state != InputsCouldNotCheck {
		t.Fatal("dirty source claimed unchanged")
	}
	attestGit(t, source, "add", "source.txt")
	attestGit(t, source, "commit", "-m", "repair")
	if state, _ := Unchanged(r, reader); state != InputsChanged {
		t.Fatal("deliverable-only commit did not wake")
	}
	current, _ := verifierGit(source, "rev-parse", "HEAD")
	r.Inputs[InputKeyRepoSourcePrefix+"example-org/one"] = current
	attestGit(t, source, "commit", "--allow-empty", "-m", "unrelated commit")
	if state, _ := Unchanged(r, reader); state != InputsChanged {
		t.Fatal("conservative commit scope did not wake")
	}
	t.Setenv(RootsEnv, "example-org/one="+tracking)
	if state, _ := Unchanged(r, reader); state != InputsCouldNotCheck {
		t.Fatal("wrong repository read as source")
	}
	t.Setenv(RootsEnv, "example-org/one="+filepath.Join(t.TempDir(), "missing"))
	if state, _ := Unchanged(r, reader); state != InputsCouldNotCheck {
		t.Fatal("missing repository read as source")
	}
	encoded, _ := json.Marshal(r.Inputs)
	if strings.Contains(string(encoded), source) || strings.Contains(string(encoded), tracking) {
		t.Fatal("manifest includes machine paths")
	}
}

// regression: #2485
func TestCrossRepoPrepareRefusals(t *testing.T) {
	for _, tc := range []string{"wrong declaration", "missing registry", "malformed registry", "symlink", "tracking modified", "tracking not main", "source not main"} {
		t.Run(tc, func(t *testing.T) {
			crossRepoRoster(t)
			source := crossRepoGitFixture(t, "example-org/one", "source.txt", "source\n")
			tracking := crossRepoGitFixture(t, "example-org/tracking", "brief.md", "# Plan\n")
			brief := filepath.Join(tracking, "brief.md")
			committed := false
			switch tc {
			case "wrong declaration":
				os.WriteFile(brief, []byte("---\nhomed-in: example-org/tracking\n---\n# Plan\n"), 0600)
				attestGit(t, tracking, "add", "brief.md")
				committed = true
			case "missing registry":
				attestGit(t, tracking, "rm", "docs/streams/graph-repos.yaml")
				committed = true
			case "malformed registry":
				os.WriteFile(filepath.Join(tracking, "docs/streams/graph-repos.yaml"), []byte("schema: graph-repos-v1\nrepos:\n  source:\n    repo: example-org/one\n  BAD alias:\n    repo: example-org/tracking\n"), 0600)
				attestGit(t, tracking, "add", "docs/streams/graph-repos.yaml")
				committed = true
			case "symlink":
				link := filepath.Join(t.TempDir(), "brief.md")
				if err := os.Symlink(brief, link); err != nil {
					t.Fatal(err)
				}
				brief = link
			case "tracking modified":
				os.WriteFile(brief, []byte("# Changed\n"), 0600)
			case "tracking not main":
				attestGit(t, tracking, "commit", "--allow-empty", "-m", "new")
			case "source not main":
				attestGit(t, source, "commit", "--allow-empty", "-m", "new")
			}
			if committed {
				attestGit(t, tracking, "commit", "-m", "declaration fixture")
				attestGit(t, tracking, "update-ref", "refs/remotes/origin/main", "HEAD")
			}
			if err := PrepareVerifierAttestation(source, "example-org/one", brief, "gpt-6-astra", "strong"); err == nil {
				t.Fatal("invalid association prepared")
			}
			record, err := verifierRecordPath(source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(record); !os.IsNotExist(err) {
				t.Fatal("refused preparation persisted a run")
			}
		})
	}
}

// regression: #2485. A "cross-repository" run whose tracking repository IS the
// deliverable is refused by name at both ends: when the run is prepared, and
// when an admitted record is later read back with its tracking clone and
// binding both re-pointed at the deliverable. Each refusal is asserted by its
// own reason, so a later check refusing for another cause does not stand in
// for the distinctness check it backs up.
func TestCrossRepoSameRepositoryRefusals(t *testing.T) {
	t.Run("prepare", func(t *testing.T) {
		crossRepoRoster(t)
		source := crossRepoGitFixture(t, "example-org/one", "source.txt", "source\n")
		tracking := crossRepoGitFixture(t, "example-org/tracking", "brief.md", "# Plan\n")
		attestGit(t, tracking, "remote", "set-url", "origin", "https://github.com/example-org/one.git")
		err := PrepareVerifierAttestation(source, "example-org/one", filepath.Join(tracking, "brief.md"), "gpt-6-astra", "strong")
		if err == nil || !strings.Contains(err.Error(), "distinct allowed tracking repository") {
			t.Fatalf("same-repository tracking brief not refused as non-distinct: %v", err)
		}
		record, rerr := verifierRecordPath(source)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if _, serr := os.Stat(record); !os.IsNotExist(serr) {
			t.Fatal("refused preparation persisted a run")
		}
	})
	t.Run("check", func(t *testing.T) {
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
		t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(r.TrackingHome)) })
		f := &verifierForge{actor: "example-desk[bot]"}
		if _, err := IssueVerifierAttestation(home, f); err != nil {
			t.Fatal(err)
		}
		if _, err := CheckVerifierAttestationWithForge(home, "brief.md", f); err != nil {
			t.Fatalf("untampered run refused: %v", err)
		}
		r, _ = verifierLoad(home)
		attestGit(t, r.TrackingHome, "remote", "set-url", "origin", "https://github.com/example-org/one.git")
		r.Binding.Repo = r.Binding.SourceRepo
		if err := verifierSave(home, r); err != nil {
			t.Fatal(err)
		}
		if _, err := CheckVerifierAttestationWithForge(home, "brief.md", f); err == nil || !strings.Contains(err.Error(), "names only one repository") {
			t.Fatalf("one-repository binding not refused as non-distinct: %v", err)
		}
	})
}

// regression: #2485
func TestCrossRepoLegacyRecovery(t *testing.T) {
	crossRepoRoster(t)
	source := crossRepoGitFixture(t, "example-org/one", "source.txt", "source\n")
	f := &verifierForge{actor: "example-desk[bot]"}
	if _, err := IssueVerifierAttestation(source, f); err == nil || !strings.Contains(err.Error(), "original") || !strings.Contains(err.Error(), "provenance") {
		t.Fatalf("missing record did not explain recovery boundary: %v", err)
	}
	if f.creates != 0 {
		t.Fatal("legacy missing provenance replaced by new issue")
	}
}

// regression: #2485
func TestCrossRepoVisibility(t *testing.T) {
	for _, tc := range []struct{ tracking, source string }{{"public", "private"}, {"public", "internal"}, {"unknown", "public"}, {"private", "unknown"}} {
		t.Run(tc.tracking+"-"+tc.source, func(t *testing.T) {
			crossRepoVisibilityRoster(t, tc.tracking, tc.source)
			source := crossRepoGitFixture(t, "example-org/one", "source.txt", "source\n")
			tracking := crossRepoGitFixture(t, "example-org/tracking", "brief.md", "# Plan\n")
			if err := PrepareVerifierAttestation(source, "example-org/one", filepath.Join(tracking, "brief.md"), "gpt-6-astra", "strong"); err == nil {
				t.Fatal("unsafe or unknown publication boundary admitted")
			}
		})
	}
}

// regression: #2485
func TestCrossRepoCheckoutPrograms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX sentinel programs; ordinary checkout coverage is portable")
	}
	for _, kind := range []string{"filter", "hook"} {
		t.Run(kind, func(t *testing.T) {
			root := crossRepoGitFixture(t, "example-org/one", "base.txt", "base\n")
			before := attestGitOutput(t, root, "rev-parse", "HEAD")
			os.WriteFile(filepath.Join(root, "new.txt"), []byte("new\n"), 0600)
			attestGit(t, root, "add", "new.txt")
			if kind == "filter" {
				os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("new.txt filter=sentinel\n"), 0600)
				attestGit(t, root, "add", ".gitattributes")
			}
			attestGit(t, root, "commit", "-m", "checkout target")
			target := attestGitOutput(t, root, "rev-parse", "HEAD")
			attestGit(t, root, "checkout", "--detach", before)
			marker := filepath.Join(t.TempDir(), "executed")
			program := filepath.Join(t.TempDir(), "sentinel.sh")
			script := "#!/bin/sh\nprintf ran > '" + strings.ReplaceAll(marker, "'", "'\\''") + "'\ncat\n"
			if err := os.WriteFile(program, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			if kind == "filter" {
				attestGit(t, root, "config", "filter.sentinel.smudge", "'"+strings.ReplaceAll(program, "'", "'\\''")+"'")
			} else {
				hooks := filepath.Join(t.TempDir(), "hooks")
				os.Mkdir(hooks, 0700)
				os.WriteFile(filepath.Join(hooks, "post-checkout"), []byte(script), 0700)
				attestGit(t, root, "config", "core.hooksPath", hooks)
			}
			if err := checkoutVerifierTracking(root, target); err == nil {
				t.Fatal("checkout program admitted")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("checkout program executed before refusal")
			}
			if got := attestGitOutput(t, root, "rev-parse", "HEAD"); got != before {
				t.Fatal("refusal moved tracking HEAD")
			}
		})
	}
}

func attestGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := verifierGit(root, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestVerifierVisibilityRank — the tracking destination may not be MORE visible than the
// deliverable repository: private < internal < public. An internal tracker must refuse a
// private source, which a check keyed only on "public" would admit.
//
// regression: #4978
func TestVerifierVisibilityRank(t *testing.T) {
	for _, tc := range []struct {
		tracking, source string
		ok               bool
	}{
		{"internal", "private", false},
		{"public", "internal", false},
		{"internal", "internal", true},
		{"internal", "public", true},
		{"private", "internal", true},
	} {
		t.Run(tc.tracking+"-"+tc.source, func(t *testing.T) {
			crossRepoVisibilityRoster(t, tc.tracking, tc.source)
			if !EffectiveConfig().Configured() {
				t.Fatalf("fixture roster did not load: %v", EffectiveConfig().Problems)
			}
			err := verifierVisibility("example-org/tracking", "example-org/one")
			if (err == nil) != tc.ok {
				t.Fatalf("verifierVisibility(tracking=%s, source=%s) = %v, want ok=%v", tc.tracking, tc.source, err, tc.ok)
			}
		})
	}
}
