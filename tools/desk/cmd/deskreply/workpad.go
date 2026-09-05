package main

import (
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

// workpadCandidate is one comment that survived filterWorkpadCandidates — a comment
// authored by the worker identity, carrying the marker, not minimised.
type workpadCandidate struct {
	// CommentID is the forge's OPAQUE comment id — EditComment's target. It is whatever the
	// backend minted (a GraphQL global id on GitHub, a project-scoped coordinate on GitLab)
	// and is passed back verbatim, never composed here.
	CommentID string
	// DatabaseID is the forge's own numeric id — what the worktree config and the "#<id>"
	// messages carry.
	DatabaseID int
}

// workpadNode is the shape one comment takes as the forge seam reports it: enough to filter
// on identity, marker and resolution state, and to drive either write path (the OPAQUE id
// for an edit, the forge's own numeric id for the worktree-local record and for display).
//
// The list is bounded by the backend's own comment read (100 on the GitHub backend). A change
// that outgrows it is a stated residual — this file follows no pagination cursor — not a
// silent mishandling: the newest-wins rule below would simply be reading a page that is not
// actually the newest.

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
func filterWorkpadCandidates(nodes []deskkit.Comment, workerLogin string) []workpadCandidate {
	var cands []workpadCandidate
	for _, n := range nodes {
		if !deskkit.SameActor(n.Author.Login, workerLogin) {
			continue
		}
		if !deskkit.HasWorkpadMarker(n.Body) {
			continue
		}
		if n.Minimized {
			continue
		}
		cands = append(cands, workpadCandidate{CommentID: n.ID, DatabaseID: int(n.DatabaseID)})
	}
	return cands
}

// newestWorkpadCandidate returns the LAST entry of cands. The seam's comment list is
// oldest-first on BOTH backends — a contract ListComments states and each backend upholds in
// its own way (GitHub's connection is ascending by default; the GitLab backend REQUESTS
// ascending order rather than reversing a page after the fact) — so the last surviving
// candidate is the newest. "two upserts ⇒ one comment" (Verify row 2) depends on this being
// deterministic across repeated calls against an unchanged comment list.
func newestWorkpadCandidate(cands []workpadCandidate) (workpadCandidate, bool) {
	if len(cands) == 0 {
		return workpadCandidate{}, false
	}
	return cands[len(cands)-1], true
}

// listWorkpadCandidates is the REAL transport behind workpadFinder: ONE bounded comment read
// through the resolved forge backend, filtered to the worker's own unminimised marked
// comments.
func listWorkpadCandidates(fg deskkit.Forge, fr deskkit.ForgeRepo, pr int, workerLogin string) ([]workpadCandidate, error) {
	comments, err := fg.ListComments(fr, pr)
	if err != nil {
		return nil, err
	}
	return filterWorkpadCandidates(comments, workerLogin), nil
}

// editWorkpadComment is the REAL transport behind workpadEditor: the seam's EditComment,
// targeting the comment's OPAQUE id — the id ListComments reported, never one composed here.
// It is the only place in the whole deskreply binary that edits (rather than creates) a
// comment; the plain-reply path never reaches this file at all.
func editWorkpadComment(fg deskkit.Forge, fr deskkit.ForgeRepo, commentID, body string) error {
	return fg.EditComment(fr, commentID, body)
}

// workpadFinder and workpadEditor are the seams cmdWorkpadUpsert calls through. Tests stub
// them directly rather than driving a recorded backend: the behaviour under test is the
// UPSERT DECISION deskreply makes from what a finder returns (idempotent upsert, never a
// foreign marker, dry-run reporting), which is exactly as observable through a stub as
// through a real transport — and filterWorkpadCandidates, the part that decides which
// comment is mine to edit, has its own direct test with no transport at all.
var (
	workpadFinder = listWorkpadCandidates
	workpadEditor = editWorkpadComment
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

// cmdWorkpadUpsert is cmdReply's --workpad tail: it runs strictly AFTER the same
// preflight/mint/public-repo-gate/PR-state verification the plain-reply path already ran
// (ac.head is already set, the worker token already minted), and replaces the plain
// path's idempotency+post block with the find-or-create decision.
func cmdWorkpadUpsert(ac *auditCtx, fg deskkit.Forge, fr deskkit.ForgeRepo, dir, repo string, pr int, body []byte, dryRun bool) error {
	workerLogin, ok := deskkit.RoleAppLogin("worker")
	if !ok {
		return deskkit.Unverifiable(
			"cannot resolve the worker App identity (role \"worker\" is unbound in the roster) — "+
				"refuse rather than guess which comment is mine to edit", nil)
	}

	cands, lerr := workpadFinder(fg, fr, pr, workerLogin)
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

	if found {
		if eerr := workpadEditor(fg, fr, target.CommentID, string(body)); eerr != nil {
			return deskkit.Unverifiable("workpad comment edit failed", eerr)
		}
		recordWorkpadID(dir, target.DatabaseID)
		ac.detail = fmt.Sprintf("edited workpad comment #%d", target.DatabaseID)
		fmt.Printf("WORKPAD: edited #%d\n", target.DatabaseID)
		return nil
	}

	// The create path. The id recorded for the NEXT invocation is the one the FORGE reported
	// for the comment it just created — not a number parsed back out of a printed URL, which
	// only ever worked for one forge's URL shape and silently yielded 0 for anything else.
	ref, cErr := fg.PostComment(fr, pr, string(body))
	if cErr != nil {
		return deskkit.Unverifiable("posting the workpad comment failed", cErr)
	}
	url := ""
	if ref != nil {
		recordWorkpadID(dir, int(ref.DatabaseID))
		url = ref.URL
	}
	if url != "" {
		ac.detail = "created workpad comment " + url
		fmt.Println(url)
	} else {
		ac.detail = "created workpad comment on PR #" + strconv.Itoa(pr)
		fmt.Printf("WORKPAD: created on PR #%d\n", pr)
	}
	return nil
}
