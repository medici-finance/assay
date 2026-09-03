package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// workpad.go is deskreply's `--workpad` verb: find-or-create ONE upserted progress
// comment per PR, authored by the worker identity, carrying the deskkit.WorkpadMarker
// line. It calls the same preflight/mint/gate/scan machinery cmdReply already runs for a
// plain reply — see cmdReply's --workpad branch — and adds exactly one further decision on
// top: is there ALREADY a candidate comment to edit, or does this call create the first
// one.
//
// SINGLE POINT OF FAILURE (brief): the marker match. Everything downstream of it —
// filterWorkpadCandidates requiring the WORKER identity (never a look-alike human
// comment), the trust gate already refusing every write verb on an untrusted PR — is a
// layer this file leans on rather than re-implements.

// workpadNode is the shape one comment takes in the GraphQL response: enough to filter on
// identity, marker and resolution state, and to drive either write path (the GraphQL node
// id for an edit, the REST-numbered id for the worktree-local record and for display).
type workpadNode struct {
	ID          string `json:"id"`
	DatabaseID  int    `json:"databaseId"`
	Body        string `json:"body"`
	IsMinimized bool   `json:"isMinimized"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
}

// workpadCandidate is one workpadNode that survived filterWorkpadCandidates — a comment
// authored by the worker identity, carrying the marker, not minimised.
type workpadCandidate struct {
	NodeID     string // GraphQL global id — the updateIssueComment mutation's target
	DatabaseID int    // REST numeric id — what the worktree config and "#<id>" messages carry
}

// workpadCommentsQuery fetches the first 100 comments on a PR. 100 is every PR this
// tooling has driven to date; a PR that outgrows it is a stated residual (this file does
// not follow a pagination cursor), not a silent mishandling — the newest-wins rule below
// would simply be reading a page that is not actually the newest.
const workpadCommentsQuery = `query($owner:String!, $name:String!, $number:Int!) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      comments(first: 100) {
        nodes {
          id
          databaseId
          body
          isMinimized
          author { login }
        }
      }
    }
  }
}`

type workpadCommentsResponse struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				Comments struct {
					Nodes []workpadNode `json:"nodes"`
				} `json:"comments"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// parseWorkpadCommentsResponse decodes the GraphQL response body into the comment nodes it
// carries. A non-empty top-level "errors" array is reported as an error even when "data"
// also came back partially populated — GraphQL's own convention for a partial failure —
// because a partial comment list must never be silently read as the complete one the
// newest-wins rule assumes.
func parseWorkpadCommentsResponse(raw string) ([]workpadNode, error) {
	var resp workpadCommentsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("cannot parse workpad comments response: %w", err)
	}
	if len(resp.Errors) > 0 {
		msgs := make([]string, len(resp.Errors))
		for i, e := range resp.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("workpad comments GraphQL error: %s", strings.Join(msgs, "; "))
	}
	return resp.Data.Repository.PullRequest.Comments.Nodes, nil
}

// filterWorkpadCandidates is the pure identity/marker/resolution filter: a node is a
// candidate ONLY when its author is the worker identity (deskkit.SameActor, which folds
// the gh-CLI and REST renderings of one App identity), it carries the exact-match workpad
// marker as its own line (deskkit.HasWorkpadMarker), and it is NOT minimised.
//
// This is what makes "a human writes the marker and the bot overwrites their comment"
// (Verify row 3) and "a resolved worker comment is skipped and a new one created"
// structurally impossible rather than merely untested: a human-authored node never has
// SameActor(node.Author.Login, workerLogin) true, and a minimised node never survives the
// IsMinimized check, so NEITHER can ever reach the caller's edit path.
func filterWorkpadCandidates(nodes []workpadNode, workerLogin string) []workpadCandidate {
	var cands []workpadCandidate
	for _, n := range nodes {
		if !deskkit.SameActor(n.Author.Login, workerLogin) {
			continue
		}
		if !deskkit.HasWorkpadMarker(n.Body) {
			continue
		}
		if n.IsMinimized {
			continue
		}
		cands = append(cands, workpadCandidate{NodeID: n.ID, DatabaseID: n.DatabaseID})
	}
	return cands
}

// newestWorkpadCandidate returns the LAST entry of cands. The comments connection GitHub's
// API returns is chronologically ascending (its documented default order), so the last
// surviving candidate is the newest — "two upserts ⇒ one comment" (Verify row 2) depends
// on this being deterministic across repeated calls against an unchanged comment list.
func newestWorkpadCandidate(cands []workpadCandidate) (workpadCandidate, bool) {
	if len(cands) == 0 {
		return workpadCandidate{}, false
	}
	return cands[len(cands)-1], true
}

// listWorkpadCandidatesGH is the REAL transport behind workpadFinder: one bounded GraphQL
// read (workpadCommentsQuery) via `gh api graphql`, parsed and filtered. It is the only
// function in this file that shells out.
func listWorkpadCandidatesGH(dir, repo string, pr int, workerLogin string) ([]workpadCandidate, error) {
	owner, name := splitOwnerRepo(repo)
	out, err := gh(dir, "api", "graphql",
		"-f", "query="+workpadCommentsQuery,
		"-f", "owner="+owner, "-f", "name="+name, "-F", "number="+strconv.Itoa(pr))
	if err != nil {
		return nil, err
	}
	nodes, perr := parseWorkpadCommentsResponse(out)
	if perr != nil {
		return nil, perr
	}
	return filterWorkpadCandidates(nodes, workerLogin), nil
}

const workpadUpdateMutation = `mutation($id: ID!, $body: String!) {
  updateIssueComment(input: {id: $id, body: $body}) {
    issueComment { databaseId }
  }
}`

// editWorkpadCommentGH is the REAL transport behind workpadEditor: the updateIssueComment
// GraphQL mutation, targeting the comment's GraphQL node id — the ONLY mutating call this
// file ever issues, and the only place in the whole deskreply binary that edits (rather
// than creates) a comment; the plain-reply path never reaches this file at all.
func editWorkpadCommentGH(dir, nodeID, bodyPath string) error {
	_, err := gh(dir, "api", "graphql",
		"-f", "query="+workpadUpdateMutation,
		"-f", "id="+nodeID, "-F", "body=@"+bodyPath)
	return err
}

// workpadFinder and workpadEditor are the seams cmdWorkpadUpsert calls through. Tests stub
// them directly rather than driving a fake `gh api graphql` subprocess: the behaviour under
// test is the UPSERT DECISION deskreply makes from what a finder returns (idempotent
// upsert, never a foreign marker, dry-run reporting), which is exactly as observable
// through a stub as through a real transport — and filterWorkpadCandidates /
// parseWorkpadCommentsResponse, the parts that actually decode GitHub's wire shape, have
// their own direct tests with no process at all.
var (
	workpadFinder = listWorkpadCandidatesGH
	workpadEditor = editWorkpadCommentGH
)

// workpadConfigKey is the worktree-local git-config key a successful upsert records, so a
// re-dispatched worker in the SAME worktree finds its own workpad without a search. It is
// advisory only: cmdWorkpadUpsert always lists and filters candidates itself (the search
// Task item 2 names as "the fallback when the config is absent" — this tool takes the
// fallback path unconditionally rather than trusting a hint it cannot re-verify without the
// same list call anyway), so a stale or absent value never causes a wrong edit or a missed
// one.
const workpadConfigKey = "assay.workpad"

// recordWorkpadID best-effort records id under workpadConfigKey, scoped to THIS worktree
// via `git config --worktree` — never `--local`, which in a git LINKED worktree (the shape
// every dispatched worker runs in) writes the checkout's SHARED .git/config instead of
// anything private to this worktree (#638/#1068's commit-identity lesson applies here
// identically). `--worktree` only actually scopes to config.worktree once
// extensions.worktreeConfig is enabled, so this enables it first — itself a repo-wide (not
// worktree-local) toggle, correctly written without --worktree.
//
// A failure here is NEVER escalated to the caller: the comment itself has already been
// posted or edited successfully by the time this runs, and the recorded id is a
// convenience for the NEXT invocation, not a correctness requirement of THIS one — see the
// workpadConfigKey comment for why an absent or stale value only costs a redundant list
// call, never a wrong write.
func recordWorkpadID(dir string, id int) {
	if id <= 0 {
		return
	}
	if cur, _ := git(dir, "config", "--get", "extensions.worktreeConfig"); strings.TrimSpace(cur) != "true" {
		if _, err := git(dir, "config", "extensions.worktreeConfig", "true"); err != nil {
			return
		}
	}
	_, _ = git(dir, "config", "--worktree", workpadConfigKey, strconv.Itoa(id))
}

// commentIDFromURL extracts the trailing numeric id from a `.../pull/N#issuecomment-ID`
// URL, as `gh pr comment` prints on success. Returns 0 (never treated as a valid id by any
// caller) when the URL does not carry the expected suffix.
func commentIDFromURL(url string) int {
	const marker = "issuecomment-"
	idx := strings.LastIndex(url, marker)
	if idx < 0 {
		return 0
	}
	id, err := strconv.Atoi(strings.TrimSpace(url[idx+len(marker):]))
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

// cmdWorkpadUpsert is cmdReply's --workpad tail: it runs strictly AFTER the same
// preflight/mint/public-repo-gate/PR-state verification the plain-reply path already ran
// (ac.head is already set, the worker token already minted), and replaces the plain
// path's idempotency+post block with the find-or-create decision.
func cmdWorkpadUpsert(ac *auditCtx, dir, repo string, pr int, body []byte, dryRun bool) error {
	workerLogin, ok := deskkit.RoleAppLogin("worker")
	if !ok {
		return deskkit.Unverifiable(
			"cannot resolve the worker App identity (role \"worker\" is unbound in the roster) — "+
				"refuse rather than guess which comment is mine to edit", nil)
	}

	cands, lerr := workpadFinder(dir, repo, pr, workerLogin)
	if lerr != nil {
		return deskkit.Unverifiable("cannot list PR comments for the workpad upsert decision", lerr)
	}
	target, found := newestWorkpadCandidate(cands)

	if dryRun {
		if found {
			fmt.Printf("WORKPAD: would edit #%d\n", target.DatabaseID)
		} else {
			fmt.Println("WORKPAD: would create")
		}
		ac.successResult = deskkit.ResultNoop
		ac.detail = "dry-run"
		return nil
	}

	if werr := deskkit.AllowWrite("deskreply", repo, pr); werr != nil {
		return werr
	}

	bodyPath, cleanup, terr := writeTempBody(body)
	if terr != nil {
		return deskkit.Unverifiable("cannot stage workpad body", terr)
	}
	defer cleanup()

	if found {
		if eerr := workpadEditor(dir, target.NodeID, bodyPath); eerr != nil {
			return deskkit.Unverifiable("workpad comment edit failed", eerr)
		}
		recordWorkpadID(dir, target.DatabaseID)
		ac.detail = fmt.Sprintf("edited workpad comment #%d", target.DatabaseID)
		fmt.Printf("WORKPAD: edited #%d\n", target.DatabaseID)
		return nil
	}

	out, cErr := gh(dir, "pr", "comment", strconv.Itoa(pr), "-R", repo, "--body-file", bodyPath)
	if cErr != nil {
		return deskkit.Unverifiable("gh pr comment failed", cErr)
	}
	url := lastURL(out)
	id := commentIDFromURL(url)
	recordWorkpadID(dir, id)
	if url != "" {
		ac.detail = "created workpad comment " + url
		fmt.Println(url)
	} else {
		ac.detail = "created workpad comment on PR #" + strconv.Itoa(pr)
		fmt.Printf("WORKPAD: created on PR #%d\n", pr)
	}
	return nil
}
