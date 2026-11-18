package main

// claimfree.go — the DISPATCH-CLAIM ownership signal for `prune` and `remove`.
//
// The defect this file exists for: a dispatched agent's home (a `tracker-*` worktree) outlives
// its run. Its agent usually locks it at boot with a bare `git worktree lock` (no reason, no
// session), so no roster beacon can ever prove that owner gone and the lock gate holds the tree
// forever. Each such home is a full checkout, and they accumulated without bound.
//
// Yet the run that owns such a home already has a durable, cross-machine record of whether it
// is still live: its DISPATCH CLAIM. deskdispatch records the claim key in the home as the
// worktree-scoped `assay.runKey` when it creates the home, and the claim itself is a ref on the
// forge. While that ref exists the run is live; once it is released, nothing is entitled to the
// home any more, whatever lock its agent left behind.
//
// The rule, in both directions:
//
//	claim HELD    → the tree is held, by every arm and by `remove`. An active claim is a
//	                live run, whatever the lock (or the lack of one) says.
//	claim FREE    → the tree is ownerless. A lock on it no longer holds it; the SAFETY gates
//	                (clean, untracked files included; every commit on the remote) still do.
//	could-not-check → held. An unreadable claim listing is not a released claim.
//
// A tree that records no run key is untouched by this file: it was not dispatched, so it has
// no claim to consult, and the gates that judged it before still judge it.
//
// The claim listing is ONE `git ls-remote` per sweep, made lazily (only once a candidate with
// a run key is reached), against the repository's own origin — the repo the dispatcher took
// the claim in. Both namespaces are read: the one claims are acquired in today and the
// branch-shaped one the fleet's readers list (deskkit/claimref.go).

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/config"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// claimState is what the claim listing says about one run key.
type claimState int

const (
	claimUnknown claimState = iota // the listing could not be read
	claimHeld
	claimFree
)

// runClaim is the claim verdict for ONE worktree: the run key it records ("" when none) and
// the state of that key's claim. why names the evidence, for the plan and for refusals.
type runClaim struct {
	key   string
	state claimState
	why   string
}

// releasedOwner reports whether the claim proves the tree's run is over.
func (c runClaim) releasedOwner() bool { return c.key != "" && c.state == claimFree }

// holds reports whether the claim, by itself, must hold the tree: a run key whose claim is
// held, or one whose claim could not be read. A tree with no run key is never held here.
func (c runClaim) holds() bool { return c.key != "" && c.state != claimFree }

// claimListing is a sweep's single read of the dispatch claims on origin.
type claimListing struct {
	read bool
	keys map[string]bool
	err  error
}

// listDispatchClaims lists the dispatch-claim keys present on dir's origin. It is a
// package-level var so a test can count the reads (one per sweep) and simulate an unreadable
// forge; the default reads the fixture's or the real origin with git itself.
var listDispatchClaims = func(dir string) (map[string]bool, error) {
	r := deskkit.Run(deskkit.ToolCall{
		Name: "git",
		Args: []string{"ls-remote", "origin",
			deskkit.DispatchClaimActiveRefsPrefix + "*", deskkit.ClaimRefsPattern},
		Dir: dir,
		// A sweep runs unattended: a credential prompt would hang it, so a forge that wants
		// one is a could-not-check, never a wait.
		Env:   append(os.Environ(), "GIT_TERMINAL_PROMPT=0"),
		Start: execCommand,
	})
	if r.Failed() {
		return nil, r.Fail(deskkit.ExitUnverifiable, "git ls-remote origin (dispatch claims)")
	}
	return parseClaimListing(r.Stdout)
}

// parseClaimListing turns `git ls-remote` output into the set of claim keys it names. A line
// that is not "<oid>\t<ref>" is an error: a listing this reader cannot parse is unread.
func parseClaimListing(out string) (map[string]bool, error) {
	keys := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, deskkit.Unverifiable("unparseable dispatch-claim listing line", nil)
		}
		ref := fields[1]
		if rest, ok := strings.CutPrefix(ref, deskkit.DispatchClaimActiveRefsPrefix); ok {
			if rest != "" && !strings.Contains(rest, "/") {
				keys[rest] = true
			}
			continue
		}
		if key, ok := deskkit.ClaimKeyFromRef(ref); ok {
			keys[key] = true
		}
	}
	return keys, nil
}

// state answers for one key, reading the listing on first use.
func (l *claimListing) state(dir, key string) (claimState, string) {
	if !l.read {
		l.read = true
		l.keys, l.err = listDispatchClaims(dir)
	}
	if l.err != nil {
		return claimUnknown, "could-not-check: dispatch claim " + key + " (claim listing unreadable: " + errText(l.err) + ")"
	}
	if l.keys[key] {
		return claimHeld, "dispatch claim " + key + " is held"
	}
	return claimFree, "dispatch claim " + key + " is released"
}

// runKeyInAdmin reads the worktree-scoped assay.runKey from a worktree's admin directory
// (<common>/worktrees/<name>/config.worktree) without starting git: "" when the file or the
// key is absent. A file that exists but cannot be read or parsed is an error — the caller
// treats it as a run key it could not read, which holds the tree.
func runKeyInAdmin(admin string) (string, error) {
	data, err := os.ReadFile(filepath.Join(admin, "config.worktree"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", deskkit.Unverifiable("cannot read the worktree's config.worktree", err)
	}
	cfg := config.New()
	if derr := config.NewDecoder(bytes.NewReader(data)).Decode(cfg); derr != nil {
		return "", deskkit.Unverifiable("cannot parse the worktree's config.worktree", nil)
	}
	return strings.TrimSpace(cfg.Section("assay").Option("runKey")), nil
}

// claimFor is the sweep's claim verdict for rt. An unreadable admin entry or run key is
// reported as a held claim on an unknown key, so it holds the tree like any could-not-check.
func (sc *sweepCtx) claimFor(guard *pathGuard, dir, rt string) runClaim {
	admin, err := sc.adminEntryFor(guard, rt)
	if err != nil {
		return runClaim{key: "?", state: claimUnknown, why: "could-not-check: run key (" + errText(err) + ")"}
	}
	key, kerr := runKeyInAdmin(admin)
	if kerr != nil {
		return runClaim{key: "?", state: claimUnknown, why: "could-not-check: run key (" + errText(kerr) + ")"}
	}
	if key == "" {
		return runClaim{}
	}
	if sc.claims == nil {
		sc.claims = &claimListing{}
	}
	st, why := sc.claims.state(dir, key)
	return runClaim{key: key, state: st, why: why}
}

// claimForRemove is `remove`'s claim verdict for its one target, read through git's own
// worktree-config resolution (the target is a single path, so no sweep index exists).
func claimForRemove(dir, rt string) runClaim {
	gitDir, err := runGit(rt, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return runClaim{key: "?", state: claimUnknown, why: "could-not-check: run key (" + errText(err) + ")"}
	}
	key, kerr := runKeyInAdmin(strings.TrimSpace(gitDir))
	if kerr != nil {
		return runClaim{key: "?", state: claimUnknown, why: "could-not-check: run key (" + errText(kerr) + ")"}
	}
	if key == "" {
		return runClaim{}
	}
	l := &claimListing{}
	st, why := l.state(dir, key)
	return runClaim{key: key, state: st, why: why}
}
