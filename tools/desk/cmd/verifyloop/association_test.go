package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

func associationGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func associationRepo(t *testing.T, repo string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	associationGit(t, root, "init")
	associationGit(t, root, "config", "user.name", "fixture")
	associationGit(t, root, "config", "user.email", "fixture@example.invalid")
	associationGit(t, root, "remote", "add", "origin", "https://github.com/"+repo+".git")
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		associationGit(t, root, "add", name)
	}
	associationGit(t, root, "commit", "-m", "fixture")
	associationGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	associationGit(t, root, "checkout", "--detach")
	return root
}
func associationFixture(t *testing.T) (loopengine.Item, string, string) {
	t.Helper()
	for _, line := range strings.Split(fixtureRoster, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			t.Setenv(k, v)
		}
	}
	deskkit.SetToolClass(deskkit.ClassCI)
	t.Cleanup(func() { deskkit.SetToolClass(deskkit.ClassWrite) })
	source := associationRepo(t, "example-org/agents", map[string]string{"source.txt": "source\n"})
	const brief = "docs/streams/sample/brief-01.md"
	tracking := associationRepo(t, "example-org/tracker", map[string]string{
		brief:                           "---\ndeliverable_repo: source\n---\n# Local tracking context\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `cat source.txt` | source |\n\n## Evidence\n\nold-evidence-canary\n",
		"docs/streams/graph-repos.yaml": "schema: graph-repos-v1\nrepos:\n  source:\n    repo: example-org/agents\n",
	})
	t.Setenv(deskkit.RootsEnv, "example-org/agents="+source+",example-org/tracker="+tracking)
	it := loopengine.Item{ID: "sample/01", BriefPath: brief, TargetSHA: associationGit(t, tracking, "rev-parse", "HEAD"), Payload: map[string]string{"repo": "example-org/tracker", "root": tracking}}
	got, err := associateVerifierItem(it, tracking)
	if err != nil {
		t.Fatal(err)
	}
	return got, source, tracking
}

// regression: #2485
func TestCrossRepoQueueAssociation(t *testing.T) {
	it, source, tracking := associationFixture(t)
	if it.Payload["repo"] != "example-org/tracker" || it.Payload["source_repo"] != "example-org/agents" || it.TargetSHA != associationGit(t, source, "rev-parse", "HEAD") || it.Payload["tracking_sha"] != associationGit(t, tracking, "rev-parse", "HEAD") {
		t.Fatalf("wrong association: %+v", it)
	}
	if err := checkQueuedAssociation(it); err != nil {
		t.Fatal(err)
	}
	home, cleanup, err := gitDetachedWorktree(it)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if got := associationGit(t, home, "config", "--get", "remote.origin.url"); !strings.Contains(got, "example-org/agents") {
		t.Fatal(got)
	}
	if _, err := os.Stat(filepath.Join(home, it.BriefPath)); !os.IsNotExist(err) {
		t.Fatal("tracking brief copied into source")
	}
	// Wrong mapping and changed tracking/source refs refuse before allocating a
	// native home or passing a prompt to the interim feeder.
	cases := []string{"repo", "source_repo", "source_root", "tracking_sha", "target", "source-changed", "tracking-changed"}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			bad := it
			bad.Payload = map[string]string{}
			for k, v := range it.Payload {
				bad.Payload[k] = v
			}
			switch tc {
			case "target":
				bad.TargetSHA = strings.Repeat("a", 40)
			case "source-changed", "tracking-changed":
				root := source
				if tc == "tracking-changed" {
					root = tracking
				}
				old := associationGit(t, root, "rev-parse", "HEAD")
				associationGit(t, root, "commit", "--allow-empty", "-m", "changed")
				associationGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
				defer associationGit(t, root, "checkout", "--detach", old)
				defer associationGit(t, root, "update-ref", "refs/remotes/origin/main", old)
			default:
				bad.Payload[tc] = "wrong"
			}
			called := false
			v := &VerifyLoop{Root: home, Native: true, RunnerCmd: []string{"unlaunched-fixture"}, MakeWorktree: func(loopengine.Item) (string, func(), error) { called = true; return "", nil, nil }}
			if _, err := v.Dispatch(bad, loopengine.TierSession); err == nil || called {
				t.Fatalf("native launched invalid association: %v %v", err, called)
			}
			v.Native = false
			v.Feeder = func(loopengine.Item, loopengine.Tier, string) (loopengine.Result, error) {
				called = true
				return loopengine.Result{}, nil
			}
			if _, err := v.Dispatch(bad, loopengine.TierSession); err == nil || called {
				t.Fatalf("interim launched invalid association: %v %v", err, called)
			}
		})
	}
}

type associationSink struct {
	*recordingDurable
	item loopengine.Item
}

func (s *associationSink) WriteEvidenceAndFlip(it loopengine.Item, evidence string, flip bool) error {
	s.item = it
	return s.recordingDurable.WriteEvidenceAndFlip(it, evidence, flip)
}

// regression: #2485
func TestCrossRepoAdapterLanding(t *testing.T) {
	it, source, tracking := associationFixture(t)
	receipt := deskkit.VerifierReceipt{Issue: 41, BriefRoot: tracking, Binding: deskkit.VerifierBinding{Version: 2, Repo: it.Payload["repo"], SourceRepo: it.Payload["source_repo"], Source: it.TargetSHA, TrackingSource: it.Payload["tracking_sha"], Brief: it.BriefPath, Run: "fixture-run", Model: "gpt-6-astra", Tier: "strong"}}
	sink := &associationSink{recordingDurable: newRec()}
	var prompt bytes.Buffer
	v := &VerifyLoop{Root: source, Emit: &prompt, DurableSink: sink, Attest: func(loopengine.Item, string, string) (deskkit.VerifierReceipt, error) { return receipt, nil }, Feeder: func(item loopengine.Item, _ loopengine.Tier, p string) (loopengine.Result, error) {
		if !strings.Contains(p, "Local tracking context") || strings.Contains(p, "old-evidence-canary") {
			t.Error("prompt did not use admitted plan only")
		}
		return loopengine.Result{Item: item, Verdict: loopengine.VerdictPass}, nil
	}}
	h, err := v.Dispatch(it, loopengine.TierSession)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Land(<-h.Done()); err != nil {
		t.Fatal(err)
	}
	if sink.item.Payload["root"] != tracking || sink.item.Payload["repo"] != receipt.Binding.Repo || !strings.Contains(sink.evidence[it.ID], receipt.EvidenceBinding()) {
		t.Fatal("landing lost tracking destination or source binding")
	}
	for _, tc := range []string{"source", "brief", "tracking", "repo", "source-repo", "legacy"} {
		t.Run(tc, func(t *testing.T) {
			bad := receipt
			switch tc {
			case "source":
				bad.Binding.Source = "wrong"
			case "brief":
				bad.Binding.Brief = "wrong"
			case "tracking":
				bad.Binding.TrackingSource = "wrong"
			case "repo":
				bad.Binding.Repo = "wrong"
			case "source-repo":
				bad.Binding.SourceRepo = "wrong"
			case "legacy":
				bad.Binding.Version = 1
			}
			v.Attest = func(loopengine.Item, string, string) (deskkit.VerifierReceipt, error) { return bad, nil }
			v.Emit = io.Discard
			if _, err := v.Dispatch(it, loopengine.TierSession); err == nil {
				t.Fatal("admitted mismatched receipt")
			}
			v.rememberRun(it, source, receipt, nil)
			if err := v.Land(loopengine.Result{Item: it, Verdict: loopengine.VerdictPass}); err == nil {
				t.Fatal("landed mismatched receipt")
			}
		})
	}
}

// regression: #2485
func TestCrossRepoPromptCutsEvidence(t *testing.T) {
	for _, heading := range []string{"## Evidence", "### Evidence — earlier run", "  ##  Evidence ###", "# EVIDENCE:", "#### Evidence", "## 5. **Evidence**"} {
		if got := verifierPromptPlan("plan\n" + heading + "\nprevious-result\n## More\nsecret"); strings.Contains(got, "previous-result") || strings.Contains(got, "secret") {
			t.Fatal("Evidence leaked into prompt")
		}
	}
}

// associationRoster plants the fixture roster for a direct association call.
func associationRoster(t *testing.T) {
	t.Helper()
	for _, line := range strings.Split(fixtureRoster, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			t.Setenv(k, v)
		}
	}
	deskkit.SetToolClass(deskkit.ClassCI)
	t.Cleanup(func() { deskkit.SetToolClass(deskkit.ClassWrite) })
}

// lagCheckout leaves root's HEAD detached one commit behind fetched main: the
// ordinary state of a shared checkout that has fetched but not pulled.
func lagCheckout(t *testing.T, root string) {
	t.Helper()
	old := associationGit(t, root, "rev-parse", "HEAD")
	associationGit(t, root, "commit", "--allow-empty", "-m", "upstream")
	associationGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	associationGit(t, root, "checkout", "-q", "--detach", old)
}

// regression: #2485 (correctness F2). Whether a brief is cross-repository is
// decided before the fetched-main gate: a same-repository brief carrying an
// alias tag, a homed-in line or a deliverable_repo naming this repository keeps
// its legacy representation in a checkout lagging fetched main. A DISTINCT
// deliverable in that checkout is still refused by the gate, by its reason.
func TestCrossRepoAssociationLaggingCheckout(t *testing.T) {
	const registry = "schema: graph-repos-v1\nself: tracker\nrepos:\n  tracker:\n    repo: example-org/tracker\n  source:\n    repo: example-org/agents\n"
	const brief = "docs/streams/sample/brief-01.md"
	same := map[string]string{
		"files alias": "# Plan\n\nfiles:\n- `[tracker] source.txt`\n\n## Verify\n",
		"homed-in":    "---\nhomed-in: example-org/tracker\n---\n# Plan\n",
		"deliverable": "---\ndeliverable_repo: tracker\n---\n# Plan\n",
	}
	for name, body := range same {
		t.Run("same repository/"+name, func(t *testing.T) {
			associationRoster(t)
			tracking := associationRepo(t, "example-org/tracker", map[string]string{brief: body, "docs/streams/graph-repos.yaml": registry})
			t.Setenv(deskkit.RootsEnv, "example-org/tracker="+tracking)
			lagCheckout(t, tracking)
			it := loopengine.Item{ID: "sample/01", BriefPath: brief, TargetSHA: associationGit(t, tracking, "rev-parse", "HEAD"), Payload: map[string]string{"repo": "example-org/tracker", "root": tracking}}
			got, err := associateVerifierItem(it, tracking)
			if err != nil {
				t.Fatalf("same-repository brief refused in a lagging checkout: %v", err)
			}
			if got.TargetSHA != it.TargetSHA || got.Payload["source_repo"] != "" || got.Payload["tracking_sha"] != "" {
				t.Fatalf("same-repository item gained an association: %+v", got)
			}
		})
	}
	t.Run("distinct deliverable", func(t *testing.T) {
		associationRoster(t)
		source := associationRepo(t, "example-org/agents", map[string]string{"source.txt": "source\n"})
		tracking := associationRepo(t, "example-org/tracker", map[string]string{brief: "---\ndeliverable_repo: source\n---\n# Plan\n", "docs/streams/graph-repos.yaml": registry})
		t.Setenv(deskkit.RootsEnv, "example-org/agents="+source+",example-org/tracker="+tracking)
		lagCheckout(t, tracking)
		it := loopengine.Item{ID: "sample/01", BriefPath: brief, Payload: map[string]string{"repo": "example-org/tracker", "root": tracking}}
		if _, err := associateVerifierItem(it, tracking); err == nil || !strings.Contains(err.Error(), "not fetched origin/main") {
			t.Fatalf("lagging tracking checkout of a cross-repository brief not refused by the fetched-main gate: %v", err)
		}
	})
}

// regression: #2485. Queue association refusals, each by its own reason.
func TestCrossRepoAssociationRefusals(t *testing.T) {
	const brief = "docs/streams/sample/brief-01.md"
	t.Run("ambiguous deliverable", func(t *testing.T) {
		associationRoster(t)
		registry := "schema: graph-repos-v1\nrepos:\n  tracker:\n    repo: example-org/tracker\n  source:\n    repo: example-org/agents\n"
		tracking := associationRepo(t, "example-org/tracker", map[string]string{brief: "# Plan\n\nfiles:\n- `[source] source.txt`\n- `[tracker] docs/plan.md`\n", "docs/streams/graph-repos.yaml": registry})
		it := loopengine.Item{ID: "sample/01", BriefPath: brief, Payload: map[string]string{"repo": "example-org/tracker", "root": tracking}}
		if _, err := associateVerifierItem(it, tracking); err == nil || !strings.Contains(err.Error(), "one unambiguous declared deliverable") {
			t.Fatalf("two declared deliverables not refused as ambiguous: %v", err)
		}
	})
	t.Run("deliverable root origin", func(t *testing.T) {
		associationRoster(t)
		registry := "schema: graph-repos-v1\nrepos:\n  source:\n    repo: example-org/agents\n"
		tracking := associationRepo(t, "example-org/tracker", map[string]string{brief: "---\ndeliverable_repo: source\n---\n# Plan\n", "docs/streams/graph-repos.yaml": registry})
		// The roots map sends the deliverable to a checkout of ANOTHER configured
		// repository: its fetched-main snapshot is valid, its origin is not.
		impostor := associationRepo(t, "example-org/tracker", map[string]string{"source.txt": "source\n"})
		t.Setenv(deskkit.RootsEnv, "example-org/agents="+impostor+",example-org/tracker="+tracking)
		it := loopengine.Item{ID: "sample/01", BriefPath: brief, Payload: map[string]string{"repo": "example-org/tracker", "root": tracking}}
		if _, err := associateVerifierItem(it, tracking); err == nil || !strings.Contains(err.Error(), "deliverable root has the wrong origin") {
			t.Fatalf("deliverable root of another repository not refused by its origin: %v", err)
		}
	})
}
