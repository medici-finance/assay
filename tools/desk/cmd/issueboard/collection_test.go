package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: class issue-collection-scope
func TestCollectionScopedReader(t *testing.T) {
	for _, projection := range []string{"full", "work-ready", "intake", "inbox"} {
		t.Run(projection, func(t *testing.T) {
			root, calls := projectionFixture(t)
			resolve := forgeFor
			counts := map[string]int{}
			forgeFor = func(repo string, obs ...deskkit.RequestObserver) (deskkit.Forge, deskkit.ForgeRepo, error) {
				counts[repo]++
				return resolve(repo, obs...)
			}
			args := []string{"--projection", projection}
			if projection == "inbox" {
				args = append(args, "--to", "worker")
			}
			code, first := projectionRun(t, root, args...)
			if code != 0 {
				t.Fatal(code, first)
			}
			firstReads := append([]forgeCall(nil), (*calls)...)
			*calls = nil
			code, second := projectionRun(t, root, args...)
			if code != 0 || projectionStable(first) != projectionStable(second) {
				t.Fatal("invocation parity", code, second)
			}
			if !reflect.DeepEqual(firstReads, *calls) {
				t.Fatal("reads were reused or reordered across invocations", firstReads, *calls)
			}
			t.Logf("two invocations: resolver setups=%v, provider operations per invocation=%d", counts, len(firstReads))
			repos := deskkit.ScanRepos()
			if len(counts) != len(repos) {
				t.Fatal("wrong repository scope", counts, repos)
			}
			for _, repo := range repos {
				if counts[repo] != 2 {
					t.Errorf("resolver setups for %s = %d; want one per invocation", repo, counts[repo])
				}
				n := 0
				for _, c := range firstReads {
					if c.repo == repo && c.op == "ListOpenIssues" {
						n++
					}
				}
				if n != 1 {
					t.Fatal("repository was omitted or reused", repo, firstReads)
				}
			}
		})
	}
}

// A previously successful collection cannot lend identity, facts or success to
// a later denied invocation. Resolution still fails before any provider read.
// regression: class issue-collection-scope
func TestCollectionResolutionFailureIsFresh(t *testing.T) {
	root, calls := projectionFixture(t)
	if code, out := projectionRun(t, root, "--projection", "work-ready"); code != 0 {
		t.Fatal(code, out)
	}
	*calls = nil
	forgeFor = func(string, ...deskkit.RequestObserver) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable("fixture credential unavailable", nil)
	}
	code, out := projectionRun(t, root, "--projection", "work-ready")
	if code != 6 || !strings.Contains(out, "fixture credential unavailable") || len(*calls) != 0 {
		t.Fatal("failed resolution reused a previous invocation", code, out, *calls)
	}
}
