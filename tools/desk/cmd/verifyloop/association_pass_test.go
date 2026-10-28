package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

func TestAssociationPassQueueParityAndReuse(t *testing.T) {
	v := associationQueueFixture(t, 96)
	// The original fresh reader is an independent oracle for the production scan.
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		want, err := associateVerifierItem(it, v.Root)
		if err != nil || !reflect.DeepEqual(want, it) {
			t.Fatalf("fresh-reader parity %s: %v\ngot %+v\nwant %+v", it.ID, err, it, want)
		}
	}
	// Poison caller Git context after fixture creation. Repository identity
	// preserves the existing scoped Git read; all object/ref reads use gitcore.
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "not-the-source"))
	t.Setenv("GIT_WORK_TREE", t.TempDir())
	t.Setenv("ASSAY_PERF_PROFILE", "tool")
	t.Setenv("ASSAY_TELEMETRY", "1")
	v.performance = performanceFixture()
	got, err := v.SelectQueue()
	if err != nil || !reflect.DeepEqual(got, items) {
		t.Fatalf("in-process parity: %v", err)
	}
	record := v.performance.session.Finish("verifyloop", "plan", perfrec.RoleNone, 0, "")
	if record.Schema != perfrec.SchemaV3 {
		t.Fatalf("schema: %s", record.Schema)
	}
	// 64 declared briefs share two roots and one immutable registry.
	for name, want := range map[string]int64{"association": 64, "association-root": 2, "association-registry": 1, "queue-scan": 1} {
		if got := phaseCount(record, name); got != want {
			t.Fatalf("%s=%d want %d", name, got, want)
		}
	}
	// The three compiled phase labels must fit the existing recorder's bound.
	if len(verifyPerfPhases) > perfrec.MaxPhases {
		t.Fatal("phase roster truncated")
	}
}

func TestAssociationPassRefreshesOnNextSelection(t *testing.T) {
	v := associationQueueFixture(t, 3)
	before, err := v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	source := deskkit.RootForRepo("example-org/agents")
	associationCommitFile(t, source, "source.txt", "new source\n")
	after, err := v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	if before[0].Payload["source_repo"] == "" || before[0].TargetSHA == after[0].TargetSHA {
		t.Fatal("selection reused earlier pass's source revision")
	}
	if after[0].TargetSHA != associationGit(t, source, "rev-parse", "HEAD") {
		t.Fatal("selection missed new source revision")
	}
	// A registry failure stays unavailable, and a later pass sees its repair.
	associationCommitFile(t, v.Root, "docs/streams/graph-repos.yaml", "schema: invalid\n")
	broken, err := v.SelectQueue()
	if err != nil || !strings.Contains(broken[0].Payload["could_not_check"], "invalid verifier association registry") {
		t.Fatalf("invalid registry: %v %+v", err, broken)
	}
	associationCommitFile(t, v.Root, "docs/streams/graph-repos.yaml", "schema: graph-repos-v1\nrepos:\n  source:\n    repo: example-org/agents\n  tracker:\n    repo: example-org/tracker\n")
	repaired, err := v.SelectQueue()
	if err != nil || repaired[0].Payload["could_not_check"] != "" {
		t.Fatalf("failed observation leaked into next pass: %v %+v", err, repaired)
	}
}

func TestAssociationPassDispatchRechecks(t *testing.T) {
	for _, change := range []string{"source-head", "source-main", "tracking-head", "tracking-main", "source-origin", "tracking-origin", "registry", "brief", "root-map"} {
		t.Run(change, func(t *testing.T) {
			v := associationQueueFixture(t, 3)
			items, err := v.SelectQueue()
			if err != nil {
				t.Fatal(err)
			}
			it := items[0]
			source := it.Payload["source_root"]
			if source == "" {
				t.Fatal("fixture has no cross-repository item")
			}
			switch change {
			case "source-head", "tracking-head", "source-main", "tracking-main":
				root := source
				if strings.HasPrefix(change, "tracking") {
					root = v.Root
				}
				old := associationGit(t, root, "rev-parse", "HEAD")
				associationGit(t, root, "commit", "--allow-empty", "-qm", "new revision")
				associationGit(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
				if strings.HasSuffix(change, "main") {
					associationGit(t, root, "checkout", "--detach", old)
				}
			case "source-origin":
				associationGit(t, source, "config", "remote.origin.url", "https://github.com/example-org/tracker.git")
			case "tracking-origin":
				associationGit(t, v.Root, "config", "remote.origin.url", "https://github.com/example-org/agents.git")
			case "registry":
				associationCommitFile(t, v.Root, "docs/streams/graph-repos.yaml", "schema: graph-repos-v1\nrepos:\n  source:\n    repo: example-org/tracker\n")
			case "brief":
				associationCommitFile(t, v.Root, it.BriefPath, "---\ndeliverable_repo: tracker\n---\n# Plan\n")
			case "root-map":
				t.Setenv(deskkit.RootsEnv, "example-org/agents="+v.Root+",example-org/tracker="+v.Root)
			}
			for _, native := range []bool{false, true} {
				launched := false
				v.Native = native
				v.RunnerCmd = []string{"must-not-launch"}
				v.MakeWorktree = func(loopengine.Item) (string, func(), error) { launched = true; return "", func() {}, nil }
				v.Feeder = func(loopengine.Item, loopengine.Tier, string) (loopengine.Result, error) {
					launched = true
					return loopengine.Result{}, nil
				}
				if _, err := v.Dispatch(it, loopengine.TierSession); err == nil || launched {
					t.Fatalf("dispatch native=%v reused stale planning observation: err=%v launched=%v", native, err, launched)
				}
			}
		})
	}
}

func TestAssociationPassRootAndRevisionIsolation(t *testing.T) {
	v := associationQueueFixture(t, 3)
	pass := deskkit.NewVerifierAssociationPass(nil)
	_, sha, err := pass.Head(v.Root)
	if err != nil {
		t.Fatal(err)
	}
	path := "docs/streams/sample/brief-01-fixture.md"
	got, err := pass.Deliverables(v.Root, sha, path)
	if err != nil || len(got) != 1 || got[0] != "example-org/agents" {
		t.Fatalf("initial: %v %v", got, err)
	}
	got[0] = "caller-mutated"
	again, err := pass.Deliverables(v.Root, sha, path)
	if err != nil || again[0] != "example-org/agents" {
		t.Fatal("caller changed memoized declaration")
	}
	// One pass can query two immutable revisions without crossing their registries.
	associationCommitFile(t, v.Root, "docs/streams/graph-repos.yaml", "schema: graph-repos-v1\nrepos:\n  source:\n    repo: example-org/tracker\n")
	next := associationGit(t, v.Root, "rev-parse", "HEAD")
	changed, err := pass.Deliverables(v.Root, next, path)
	if err != nil || changed[0] != "example-org/tracker" {
		t.Fatalf("revision collision: %v %v", changed, err)
	}
	old, err := pass.Deliverables(v.Root, sha, path)
	if err != nil || old[0] != "example-org/agents" {
		t.Fatal("immutable key changed with HEAD")
	}
	if _, err := pass.Deliverables(v.Root, "HEAD", path); err == nil {
		t.Fatal("mutable ref accepted as immutable key")
	}
	// Separate checkouts at different commits remain separate observation roots.
	other := associationRepo(t, "example-org/tracker", map[string]string{path: "---\ndeliverable_repo: source\n---\n", "docs/streams/graph-repos.yaml": "schema: graph-repos-v1\nrepos:\n  source:\n    repo: example-org/tracker\n"})
	_, otherSHA, err := pass.Head(other)
	if err != nil {
		t.Fatal(err)
	}
	distinct, err := pass.Deliverables(other, otherSHA, path)
	if err != nil || distinct[0] != "example-org/tracker" {
		t.Fatalf("root collision: %v %v", distinct, err)
	}
	// Linked worktrees sharing objects must still have independent HEAD facts.
	linked := filepath.Join(t.TempDir(), "linked")
	associationGit(t, v.Root, "worktree", "add", "--detach", linked, sha)
	_, linkedSHA, err := pass.Head(linked)
	if err != nil || linkedSHA != sha {
		t.Fatalf("linked root: %s %v", linkedSHA, err)
	}
	linkedRepos, err := pass.Deliverables(linked, sha, path)
	if err != nil || linkedRepos[0] != "example-org/agents" {
		t.Fatalf("linked immutable read: %v %v", linkedRepos, err)
	}
	// Missing roots are cached for this pass, refreshed by a new pass.
	missing := filepath.Join(t.TempDir(), "not-yet-present")
	if _, _, err := pass.Head(missing); err == nil {
		t.Fatal("missing root accepted")
	}
	if err := os.Rename(other, missing); err != nil {
		t.Fatal(err)
	}
	if _, _, err := pass.Head(missing); err == nil {
		t.Fatal("failed root was reread within pass")
	}
	if _, _, err := deskkit.NewVerifierAssociationPass(nil).Head(missing); err != nil {
		t.Fatalf("root failure persisted across passes: %v", err)
	}
}

func TestAssociationPassPlanningLaggingCheckouts(t *testing.T) {
	for _, which := range []string{"tracking", "source"} {
		t.Run(which, func(t *testing.T) {
			v := associationQueueFixture(t, 3)
			root := v.Root
			if which == "source" {
				root = deskkit.RootForRepo("example-org/agents")
			}
			lagCheckout(t, root)
			items, err := v.SelectQueue()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(items[0].Payload["could_not_check"], "not fetched origin/main") {
				t.Fatalf("cross-repository lag admitted: %+v", items[0])
			}
			for _, it := range items[1:] {
				if it.Payload["could_not_check"] != "" || it.Payload["source_repo"] != "" {
					t.Fatalf("legacy/same-repository lag changed: %+v", it)
				}
			}
		})
	}
}

// The pass must use the same effective origin as fresh admission. A local-only
// URL read can misclassify a declared source as same-repository and skip recheck.
func TestAssociationPassScopedOriginParity(t *testing.T) {
	for _, scope := range []string{"include", "worktree"} {
		t.Run(scope, func(t *testing.T) {
			v := associationQueueFixture(t, 3)
			associationGit(t, v.Root, "config", "remote.origin.url", "https://github.com/example-org/agents.git")
			if scope == "include" {
				config := filepath.Join(t.TempDir(), "identity.config")
				if err := os.WriteFile(config, []byte("[remote \"origin\"]\n url = https://github.com/example-org/tracker.git\n"), 0600); err != nil {
					t.Fatal(err)
				}
				associationGit(t, v.Root, "config", "include.path", config)
			} else {
				associationGit(t, v.Root, "config", "extensions.worktreeConfig", "true")
				associationGit(t, v.Root, "config", "--worktree", "remote.origin.url", "https://github.com/example-org/tracker.git")
			}
			items, err := v.SelectQueue()
			if err != nil {
				t.Fatal(err)
			}
			cross := items[0]
			if cross.Payload["source_repo"] != "example-org/agents" || cross.Payload["repo"] != "example-org/tracker" || cross.Payload["could_not_check"] != "" {
				t.Fatalf("effective origin misclassified: %+v", cross)
			}
			want, err := associateVerifierItem(cross, v.Root)
			if err != nil || !reflect.DeepEqual(want, cross) {
				t.Fatalf("fresh/pass identity mismatch: %v", err)
			}
		})
	}
}
