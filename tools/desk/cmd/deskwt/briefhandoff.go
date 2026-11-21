package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitexec"
)

// The forge read is made only for an explicitly requested brief handoff, after the
// checkout, branch and previous dispatch have been identified.
var handoffChange = func(repo string, pr int) (*deskkit.PullRequest, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, fmt.Errorf("invalid repository %q", repo)
	}
	target := deskkit.ForgeRepo{Owner: owner, Name: name}
	f, _, err := deskkit.ResolveForge(target, "worker")
	if err != nil {
		return nil, err
	}
	return f.GetPullRequest(target, pr)
}

// A present ref alone does not identify the new holder. Read the claim tool's
// HELD wire line and match only the fixed owner/state fields, never the note.
var handoffOwnedClaim = func(dir, repo, key, session string) (bool, error) {
	urls, err := originURLs(dir, false)
	if err != nil {
		return false, err
	}
	if len(urls) == 0 {
		return false, fmt.Errorf("no fetch URL for the claim repository")
	}
	credentialPath, err := roleCredentialPath("worker", repo, urls[0])
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), claimListTimeout)
	defer cancel()
	r := deskkit.Run(deskkit.ToolCall{Context: ctx, Dir: dir, Name: "deskclaim-ref", Start: execCommand,
		Args: []string{"show", key, "--repo", repo, "--token-file", credentialPath}, Env: claimListEnv()})
	if r.Failed() {
		return false, r.Fail(deskkit.ExitUnverifiable, "cannot confirm the PR claim owner")
	}
	line := strings.TrimSpace(r.Stdout)
	prefix := "dispatch-claim: HELD " + key + " — dispatch-claim " + key + " owner=" + session + " state=claimed branch="
	return !strings.Contains(line, "\n") && strings.HasPrefix(line, prefix), nil
}

// briefHandoff can return a dirty home. Its ownership proof is the original
// dispatch record paired to this checkout, never its directory name. No checkout,
// reset, unlock, merge or index operation belongs on this path.
func briefHandoff(g *pathGuard, dir, candidate, repo, branch, head, next string) (string, error) {
	rt, err := g.check(candidate)
	if err != nil {
		return "", err
	}
	admin, err := ownAdminEntry(g, rt)
	if err != nil {
		return "", err
	}
	session := deskkit.DispatchRecordSessionTag()
	if session == deskkit.UnknownSessionTag {
		return "", deskkit.Refused("brief handoff requires a known worker session")
	}
	locks, err := g.lockedWorktrees(dir)
	if err != nil {
		return "", err
	}
	if why, locked := locks[rt]; locked && sessionFromLockReason(why) != session {
		return "", deskkit.Refused("the checkout has another owner's lock")
	}
	if !worktreeConfigEnabled(rt) {
		return "", deskkit.Refused("the checkout has no worktree-scoped configuration")
	}
	checkout, err := gitcore.Open(rt)
	if err != nil {
		return "", deskkit.Unverifiable("cannot open the registered checkout", err)
	}
	ref, err := checkout.SymbolicRefTarget("HEAD")
	if err != nil || ref != "refs/heads/"+branch {
		return "", deskkit.Refused("the checkout no longer holds the PR branch")
	}
	// runAdd already checked that head names a commit in this common object store.
	tip, err := checkout.HeadID()
	if err != nil || tip != head {
		return "", deskkit.Refused("the checkout is not at the confirmed PR head")
	}
	rec, err := runRecordInAdmin(admin)
	if err != nil {
		return "", err
	}
	if rec.repo != repo || rec.store != deskkit.ClaimStoreForgeRef {
		return "", deskkit.Refused("the original dispatch's claim repository or store cannot be confirmed")
	}
	sourceRef := rec.dispatchRef
	if rec.key == next {
		// An interrupted handoff may already have written the PR key. The
		// original receipt survives both that write and the next dispatch ref.
		sourceRef = rec.handoffSourceRef
	}
	sourceKey, _, ok := strings.Cut(sourceRef, "@")
	if !ok || sourceKey == next || (rec.key != sourceKey && rec.key != next) || !deskkit.ValidDispatchRef(sourceKey, sourceRef) {
		return "", deskkit.Refused("the checkout has no matching original brief dispatch receipt")
	}
	previous, err := deskkit.LatestUsageDispatch(sourceKey)
	if err != nil || previous.DispatchRef == nil || *previous.DispatchRef != sourceRef ||
		previous.Repo != repo || previous.Branch == nil || *previous.Branch != branch ||
		previous.SessionTag != session || previous.PR != nil || previous.Kit == nil ||
		(*previous.Kit != "worker" && *previous.Kit != "worker-objective") {
		return "", deskkit.Refused("the original worker dispatch does not match this session, repository and branch")
	}
	prefix := deskkit.RepoShortLabel(repo) + "--"
	n, err := strconv.Atoi(strings.TrimPrefix(next, prefix+"pr-"))
	if err != nil || n <= 0 || next != prefix+"pr-"+strconv.Itoa(n) {
		return "", deskkit.Refused("the new run key is not a PR in this repository")
	}
	change, err := handoffChange(repo, n)
	if err != nil {
		return "", deskkit.Unverifiable("could not read the PR for this handoff", err)
	}
	if change == nil || change.Number != n || change.MergedAt != "" || change.CrossRepo != deskkit.CrossRepoSame || change.HeadSHA != head {
		return "", deskkit.Refused("the open PR no longer matches the checkout's repository, branch and head")
	}
	// This handoff preserves a staged index on its original branch. A second
	// branch at the same commit is insufficient to take ownership of that index.
	match, err := deskkit.CheckOwnPR("deskwt brief handoff", deskkit.OwnPRLocal{Branch: branch, Head: head},
		deskkit.OwnPRRemote{Number: n, State: change.State, HeadRef: change.HeadRef, HeadOid: change.HeadSHA})
	if err != nil || match != deskkit.OwnPRByBranch {
		return "", deskkit.Refused("the open PR no longer matches the checkout's repository, branch and head")
	}
	brief := deskkit.DeliveredLocalBrief(change.Body, repo)
	if brief == "" || sourceKey != prefix+strings.ReplaceAll(brief, "/", "--") {
		return "", deskkit.Refused("the PR does not deliver the brief that owns this checkout")
	}
	keys, err := listDispatchClaims(dir)
	if err != nil {
		return "", deskkit.Unverifiable("could not read the original and new dispatch claims", err)
	}
	if keys[sourceKey] || !keys[next] {
		return "", deskkit.Refused("the brief claim must be released and the PR claim acquired before handoff")
	}
	owned, err := handoffOwnedClaim(dir, repo, next, session)
	if err != nil {
		return "", deskkit.Unverifiable("cannot confirm the new claim holder", err)
	}
	if !owned {
		return "", deskkit.Refused("the new PR claim does not belong to this session")
	}
	if _, err := gitexec.Run("deskwt", rt, "config", "--file", filepath.Join(admin, "config.worktree"), "assay.handoffSourceRef", sourceRef); err != nil {
		return "", deskkit.Unverifiable("could not preserve the original dispatch receipt for retry", err)
	}
	return rt, nil
}
