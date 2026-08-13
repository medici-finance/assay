package deskkit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestCensusMatchesLive checks the compiled-in census against the world it describes:
// every row's CIRequired against the repo's ACTUAL workflow triggers, and the org's
// actual repo listing against the set of PUBLIC repos the census names.
//
// Why this exists. A census row is a claim about ANOTHER repository, and it goes wrong
// WITHOUT ANYONE TOUCHING THIS ONE. Both halves have already failed that way:
//
//   - CI (#310 R1). `CIRequired: false` means "this repo runs no PR CI, so an empty
//     rollup is everything there will ever be, so it is green" (cmd/deskpost/ready.go,
//     `case ciEmpty`). example-org/examples gained a `pull_request`-triggered
//     tools.yml and the row kept saying false — fail-OPEN, and the ordinary suite could
//     not see it, because ci_test.go's want map and this table are both compiled in and
//     agree with each other by construction.
//   - VISIBILITY (#310 R2). medici-finance/assay was created public and
//     entered desk write scope by org-default the same minute. Prose sentences
//     enumerating how many repos the org holds and how many are public went false without an
//     edit, and nothing reported it.
//
// So the check that matters cannot be hermetic. The PARSING half is — see
// cicensus_parse_test.go, which runs in ordinary CI — but the comparison against GitHub
// needs network and an authenticated `gh`, which CI here does not have for the example-org
// repos. Run this whenever a census row is added, a CIRequired value is re-dated, or a
// repo is created in the org:
//
//	DESKKIT_LIVE_CENSUS=1 go test ./internal/deskkit/ -run TestCensusMatchesLive -count=1
//
// A repo whose PR trigger is PATH-FILTERED counts as PR CI here: the boolean cannot say
// "PR CI for some diffs", and `true` is the fail-closed side of that limitation. Any row
// accepting that residual must say so in its comment (examples does).
func TestCensusMatchesLive(t *testing.T) {
	if os.Getenv("DESKKIT_LIVE_CENSUS") == "" {
		t.Skip("live census check: set DESKKIT_LIVE_CENSUS=1 (needs network + authenticated gh)")
	}
	if _, err := exec.LookPath("gh"); err != nil {
		t.Fatalf("DESKKIT_LIVE_CENSUS is set but `gh` is not on PATH: %v", err)
	}

	t.Run("ci", func(t *testing.T) {
		for _, repo := range AllowedRepos() {
			t.Run(repo, func(t *testing.T) {
				live, read := repoHasPullRequestWorkflow(t, repo)
				if len(read) == 0 {
					t.Logf("%s: no workflows readable (empty repo or no .github/workflows)", repo)
				}
				if got := CIRequired(repo); got != live {
					t.Fatalf("CIRequired(%q) = %v but the live workflows say PR CI = %v.\n"+
						"Workflows read: %v\n"+
						"Fix the census row in config.go AND cmd/deskpost/ci_test.go's want map, "+
						"or state the divergence in the row comment.",
						repo, got, live, read)
				}
			})
		}
	})

	// The R2 half. Every medici-finance repo is desk-writable by org-default, so a repo
	// created in the org enters write scope with no code edit and no announcement. The
	// census cannot be required to list them all — that would undo the widening — but a
	// PUBLIC one is different in kind: it is the surface an outsider can read after an
	// injection-driven write, and it is exactly what the threat model is
	// about. Requiring every public org repo
	// to appear in the census, with Visibility: VisibilityPublic, is what makes a new
	// world-readable repo IMPOSSIBLE to enter scope silently. It is also the only check
	// that would have caught medici-finance/assay on the day it appeared.
	t.Run("public_org_repos_are_in_the_census", func(t *testing.T) {
		raw, err := ghAPI("orgs/medici-finance/repos", "--paginate",
			"--jq", `.[]|"\(.full_name)\t\(.visibility)"`)
		if err != nil {
			t.Fatalf("listing medici-finance repos: %v", err)
		}
		var livePublic []string
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			parts := strings.Split(line, "\t")
			if len(parts) != 2 {
				continue
			}
			if parts[1] == "public" {
				livePublic = append(livePublic, parts[0])
			}
		}
		if len(livePublic) == 0 {
			t.Fatal("the org listing reported no public repos at all — that is a read failure, " +
				"not a clean result; an unread org is drift, never a pass")
		}
		for _, repo := range livePublic {
			if _, inCensus := allowedRepos()[repo]; !inCensus {
				t.Fatalf("%s is PUBLIC and desk-writable by org-default, but is absent from the "+
					"census in config.go. A world-readable repo must never enter desk write scope "+
					"unrecorded (#310 R2). Add the row with Visibility: VisibilityPublic and a "+
					"checked CIRequired value.", repo)
			}
			if got := RepoVisibility(repo); got != VisibilityPublic {
				t.Fatalf("%s is PUBLIC at the API but the census says %q — VisibilityRiskClassed "+
					"and every reader of the threat model are working from the wrong value", repo, got)
			}
		}
		// Counterweight: a census row claiming public must actually be public, so
		// "mark everything public" would not satisfy this test either.
		for _, repo := range AllowedRepos() {
			if RepoVisibility(repo) != VisibilityPublic {
				continue
			}
			found := false
			for _, p := range livePublic {
				if p == repo {
					found = true
					break
				}
			}
			if !found && strings.HasPrefix(repo, "medici-finance/") {
				t.Fatalf("census says %s is PUBLIC but the live org listing does not — the row is "+
					"stale in the other direction", repo)
			}
		}
		t.Logf("public medici-finance repos, live: %v", livePublic)
	})
}

// repoHasPullRequestWorkflow reports whether ANY workflow on repo's default branch
// triggers on pull_request, and returns the workflow filenames it read.
func repoHasPullRequestWorkflow(t *testing.T, repo string) (bool, []string) {
	t.Helper()

	listing, err := ghAPI("repos/" + repo + "/contents/.github/workflows")
	if err != nil {
		// An empty repo or a missing workflows dir is a legitimate "no PR CI"; any
		// other failure must NOT be swallowed into a false negative.
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "empty") {
			return false, nil
		}
		t.Fatalf("listing %s workflows: %v", repo, err)
	}

	var entries []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(listing, &entries); err != nil {
		t.Fatalf("parsing %s workflow listing: %v", repo, err)
	}

	var read []string
	found := false
	for _, e := range entries {
		if e.Type != "file" || !strings.HasSuffix(e.Name, ".yml") && !strings.HasSuffix(e.Name, ".yaml") {
			continue
		}
		body, err := ghAPI("repos/"+repo+"/contents/"+e.Path, "-H", "Accept: application/vnd.github.raw")
		if err != nil {
			t.Fatalf("reading %s/%s: %v", repo, e.Path, err)
		}
		label := e.Name
		if workflowTriggersPullRequest(string(body)) {
			label += " (pull_request)"
			found = true
		}
		read = append(read, label)
	}
	return found, read
}

func ghAPI(args ...string) ([]byte, error) {
	out, err := exec.Command("gh", append([]string{"api"}, args...)...).Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		return nil, fmt.Errorf("gh api %s: %v: %s", strings.Join(args, " "), err, stderr)
	}
	return out, nil
}
