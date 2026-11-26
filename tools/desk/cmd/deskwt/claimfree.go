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
// "Absent from the listing" proves a release only for the claim namespace the listing reads. So
// a key counts as released only when the home also records WHERE its claim was taken, in the
// forge-ref store the listing reads: deskdispatch records `assay.claimRepo` and
// `assay.claimStore` beside `assay.runKey`, and the listing is read in that repository. A home
// claimed in this checkout's origin is read there. A cross-repository verifier claims in the
// TRACKING repo while its home is cut from the deliverable checkout; the claim tool dialed the
// tracking repository on this checkout's forge host, so its listing is read at the address
// origin's URL gives with its trailing owner/name replaced by the recorded repository (same
// scheme, host and rewrite rules, so the same credentials). An address that cannot be derived,
// or a listing that cannot be read there, is could-not-check, which holds.
//
// A home recorded before the namespace was recorded holds too: its claim may have been taken
// in either repository, and an absence in the wrong one proves nothing. `deskwt remove <home>
// --claim-repo <owner/name>` is the sanctioned path for one: the operator names the repository
// the dispatch claimed in (forge-ref, the one store shipped then), and every other gate still
// applies. A home whose store is not forge-ref holds.
//
// The claim listing is ONE `git ls-remote` per repository per sweep, made lazily (only once a
// candidate with a run key is reached). Both namespaces are read: the one
// claims are acquired in today and the branch-shaped one the fleet's readers list
// (deskkit/claimref.go). Just before a destructive step the key is re-read once more
// (claimStillFree), in the same repository, so a resume that re-acquired the claim after the
// sweep's listing is seen.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	// remote is where the claim listing was read: originRemote, or the address of the
	// repository the claim was taken in. claimStillFree re-reads the same one.
	remote string
}

// originRemote names this checkout's own origin as the listing's remote.
const originRemote = "origin"

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

// claimListTimeout bounds one claim listing, so an unresponsive forge is a could-not-check and
// never a hung sweep.
const claimListTimeout = 60 * time.Second

// claimListEnv is the environment of the claim listing: the caller's own with every GIT_*
// variable dropped (an inherited GIT_DIR or GIT_CONFIG_* could point `origin` at a readable but
// wrong listing, which reads every live claim as released), then the two switches that make an
// unattended read fail instead of wait: no credential prompt, and ssh in batch mode so a
// passphrase or host-key question is an error, not a hang.
func claimListEnv() []string {
	return append(scrubbedGitEnv(), "GIT_TERMINAL_PROMPT=0", "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
}

// listDispatchClaims lists the dispatch-claim keys present on remote, read from dir: remote
// is originRemote or a repository address claimRepoURL derived. It is a package-level var so
// a test can count the reads and simulate an unreadable forge; the default reads the
// fixture's or the real repository with git itself, under claimListEnv and claimListTimeout.
var listDispatchClaims = func(dir, remote string) (map[string]bool, error) {
	shown := deskkit.RedactRemoteUserinfo(remote)
	if remote == "" || strings.HasPrefix(remote, "-") {
		return nil, deskkit.Unverifiable(fmt.Sprintf("refusing to list dispatch claims at %q", shown), nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), claimListTimeout)
	defer cancel()
	r := deskkit.Run(deskkit.ToolCall{
		Context: ctx,
		Name:    "git",
		Args: []string{"ls-remote", remote,
			deskkit.DispatchClaimActiveRefsPrefix + "*", deskkit.ClaimRefsPattern},
		Dir:   dir,
		Env:   claimListEnv(),
		Start: execCommand,
	})
	if r.Failed() {
		return nil, r.Fail(deskkit.ExitUnverifiable, "git ls-remote %s (dispatch claims)", shown)
	}
	return parseClaimListing(r.Stdout)
}

// parseClaimListing turns `git ls-remote` output into the set of claim keys it names. A line
// that is not "<oid>\t<ref>" is an error: a listing this reader cannot parse is unread.
//
// Every ref under either claim prefix is a key, nested ones included: the claim tool accepts a
// key containing `/` and lists it verbatim (deskclaim-ref's List), so a reader that dropped
// nested refs would read such a held claim as released (SEC-5). Keeping too many keys only
// holds trees; dropping one would free a live run's home.
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
		ref := strings.TrimSuffix(fields[1], "^{}")
		for _, prefix := range []string{deskkit.DispatchClaimActiveRefsPrefix, deskkit.ClaimRefsPrefix} {
			if rest, ok := strings.CutPrefix(ref, prefix); ok && rest != "" {
				keys[rest] = true
			}
		}
	}
	return keys, nil
}

// state answers for one key, reading remote's listing on first use. where is "" for this
// checkout's origin, else " in <owner/name>", the repository the reasons name.
func (l *claimListing) state(dir, remote, where, key string) (claimState, string) {
	if !l.read {
		l.read = true
		l.keys, l.err = listDispatchClaims(dir, remote)
	}
	if l.err != nil {
		return claimUnknown, "could-not-check: dispatch claim " + key + " (claim listing" + where + " unreadable: " + errText(l.err) + ")"
	}
	if l.keys[key] {
		return claimHeld, "dispatch claim " + key + " is held" + where
	}
	return claimFree, "dispatch claim " + key + " is released" + where
}

// runRecord holds the dispatch key, claim namespace and receipts stamped in a home.
// A field absent from the worktree's configuration is empty.
type runRecord struct {
	key              string // assay.runKey
	repo             string // assay.claimRepo: the repository whose claim namespace holds the claim
	store            string // assay.claimStore: the resolved claim store
	dispatchRef      string // receipt of the current dispatch
	handoffSourceRef string // original brief receipt retained while re-keying a checkout
}

// runRecordInAdmin reads the run record directly from the registered admin directory
// (<common>/worktrees/<name>/config.worktree)
// without starting git: all empty when the file or the keys are absent. A file that exists but
// cannot be read or parsed is an error — the caller treats it as a run key it could not read,
// which holds the tree.
func runRecordInAdmin(admin string) (runRecord, error) {
	data, err := os.ReadFile(filepath.Join(admin, "config.worktree"))
	if os.IsNotExist(err) {
		return runRecord{}, nil
	}
	if err != nil {
		return runRecord{}, deskkit.Unverifiable("cannot read the worktree's config.worktree", err)
	}
	cfg := config.New()
	if derr := config.NewDecoder(bytes.NewReader(data)).Decode(cfg); derr != nil {
		return runRecord{}, deskkit.Unverifiable("cannot parse the worktree's config.worktree", nil)
	}
	sec := cfg.Section("assay")
	return runRecord{
		key:              strings.TrimSpace(sec.Option("runKey")),
		repo:             strings.TrimSpace(sec.Option("claimRepo")),
		store:            strings.TrimSpace(sec.Option("claimStore")),
		dispatchRef:      strings.TrimSpace(sec.Option("dispatchRef")),
		handoffSourceRef: strings.TrimSpace(sec.Option("handoffSourceRef")),
	}, nil
}

// originSlug is this checkout's origin as owner/name, read once per sweep without starting git.
func (sc *sweepCtx) originSlug(dir string) (string, error) {
	if !sc.originRead {
		sc.originRead = true
		sc.originURL, sc.origin, sc.originErr = currentOrigin(dir)
	}
	return sc.origin, sc.originErr
}

// claimRemote is where the home's claim listing is read: originRemote when the claim was taken
// in this checkout's origin, or the derived address of the repository that took it. hold is
// the could-not-check reason that holds the tree when neither can be named.
func (sc *sweepCtx) claimRemote(dir string, rec runRecord) (remote, hold string) {
	pre := "could-not-check: dispatch claim " + rec.key
	switch {
	case rec.store == "":
		return "", pre + " (the home records no claim store, so a listing here proves nothing about it; " +
			"if it predates recorded claim namespaces, name the repository its dispatch claimed in: " +
			"deskwt remove <home> --claim-repo <owner/name>. A worker or review dispatch claims in this " +
			"checkout's origin, a cross-repository verifier in its tracking repository)"
	case rec.store != deskkit.ClaimStoreForgeRef:
		return "", pre + " (taken in the " + rec.store + " store; only the " + deskkit.ClaimStoreForgeRef + " listing is read here)"
	case rec.repo == "":
		return "", pre + " (the home records no claim repository, so a listing here proves nothing about it)"
	}
	origin, err := sc.originSlug(dir)
	if err != nil {
		return "", pre + " (this checkout's origin cannot be read: " + errText(err) + ")"
	}
	if strings.EqualFold(origin, rec.repo) {
		return originRemote, ""
	}
	url, err := claimRepoURL(sc.originURL, rec.repo)
	if err != nil {
		return "", pre + " (taken in " + rec.repo + ", not in this checkout's origin " + origin +
			", and that repository's address cannot be derived from origin: " + errText(err) + ")"
	}
	return url, ""
}

// claimRepoURL is the address of repository repo on the forge origin names: originURL with its
// trailing owner/name replaced by repo, keeping the scheme, host (or SSH alias), port, userinfo
// and a ".git" suffix, so git applies the same rewrite rules and credentials it applies to
// origin. The claim tool dialed the claim repository on this checkout's origin host, so this is
// the namespace the claim was taken in. Anything it cannot derive exactly is an error.
func claimRepoURL(originURL, repo string) (string, error) {
	if !deskkit.ValidRepoSlug(repo) {
		return "", fmt.Errorf("the recorded claim repository %q is not an owner/name", repo)
	}
	owner, name, err := deskkit.ParseRemoteRepo(originURL)
	if err != nil {
		return "", err
	}
	base := strings.TrimRight(strings.TrimSpace(originURL), "/")
	suffix := ""
	if trimmed, ok := strings.CutSuffix(base, ".git"); ok {
		base, suffix = trimmed, ".git"
	}
	head, ok := strings.CutSuffix(base, owner+"/"+name)
	if !ok || head == "" || !strings.ContainsAny(head[len(head)-1:], "/:") {
		return "", fmt.Errorf("origin %s does not end in its own %s/%s", deskkit.RedactRemoteUserinfo(originURL), owner, name)
	}
	return head + repo + suffix, nil
}

// assertClaimRepo applies `remove --claim-repo`: the repository the operator names as the one
// a home's dispatch claimed in. It fills the namespace of a home that records none (in the
// forge-ref store, the only one shipped before namespaces were recorded) and must agree with
// a namespace the home does record; a disagreement is refused, never resolved either way.
func assertClaimRepo(rec runRecord, repo string) (runRecord, error) {
	if repo == "" {
		return rec, nil
	}
	if rec.key == "" {
		return rec, deskkit.Refused("refused: --claim-repo names a claim repository, but the home records no dispatch run key (assay.runKey)")
	}
	if rec.repo != "" && !strings.EqualFold(rec.repo, repo) {
		return rec, deskkit.Refused("refused: --claim-repo " + repo + " contradicts the claim repository the home records (" + rec.repo + ")")
	}
	if rec.store != "" && rec.store != deskkit.ClaimStoreForgeRef {
		return rec, deskkit.Refused("refused: --claim-repo asserts a " + deskkit.ClaimStoreForgeRef +
			" claim, but the home records the " + rec.store + " store")
	}
	rec.repo, rec.store = repo, deskkit.ClaimStoreForgeRef
	return rec, nil
}

// claimFor is the sweep's claim verdict for rt. An unreadable admin entry or run key is
// reported as a held claim on an unknown key, so it holds the tree like any could-not-check.
//
// When the sweep's admin inventory is incomplete (one sibling entry could not be read), the
// candidate's run record is read from its OWN admin entry instead, confirmed both ways exactly
// as `remove` does (ownAdminEntry). A broken sibling therefore cannot hold every other tree
// (F8), while a candidate whose own entry cannot be confirmed is still held. Any other index
// error (a different common store, two entries naming rt, no entry naming rt) still holds.
func (sc *sweepCtx) claimFor(guard *pathGuard, dir, rt string) runClaim {
	admin, err := sc.adminEntryFor(guard, rt)
	if err != nil {
		if !sc.claimForFallbackOK(guard, rt) {
			return runClaim{key: "?", state: claimUnknown, why: "could-not-check: run key (" + errText(err) + ")"}
		}
		own, oerr := ownAdminEntry(guard, rt)
		if oerr != nil {
			return runClaim{key: "?", state: claimUnknown,
				why: "could-not-check: run key (" + errText(err) + "; own admin entry: " + errText(oerr) + ")"}
		}
		admin = own
	}
	return sc.claimFromAdmin(dir, admin)
}

// adminIndexIncomplete reports whether the sweep's admin inventory, for guard's own common
// store, was loaded and failed as a whole (some entry could not be inventoried).
func (sc *sweepCtx) adminIndexIncomplete(guard *pathGuard) bool {
	return guard.commonDir == sc.commonDir && sc.admins != nil && sc.admins.err != nil
}

// claimForFallbackOK reports whether rt may take the own-entry fallback: the inventory is
// incomplete, and the part that WAS read does not already name rt twice.
func (sc *sweepCtx) claimForFallbackOK(guard *pathGuard, rt string) bool {
	return sc.adminIndexIncomplete(guard) && !sc.admins.ambiguous[rt]
}

// claimFromAdmin is the claim verdict for the home whose admin entry is admin.
func (sc *sweepCtx) claimFromAdmin(dir, admin string) runClaim {
	rec, err := runRecordInAdmin(admin)
	if err != nil {
		return runClaim{key: "?", state: claimUnknown, why: "could-not-check: run key (" + errText(err) + ")"}
	}
	return sc.claimFromRecord(dir, rec)
}

// claimFromRecord is the claim verdict for a home's run record, read in the repository the
// record names (claimRemote), one listing per repository per sweep.
func (sc *sweepCtx) claimFromRecord(dir string, rec runRecord) runClaim {
	if rec.key == "" {
		return runClaim{}
	}
	remote, hold := sc.claimRemote(dir, rec)
	if hold != "" {
		return runClaim{key: rec.key, state: claimUnknown, why: hold}
	}
	if sc.claims == nil {
		sc.claims = map[string]*claimListing{}
	}
	l := sc.claims[remote]
	if l == nil {
		l = &claimListing{}
		sc.claims[remote] = l
	}
	where := ""
	if remote != originRemote {
		where = " in " + rec.repo
	}
	st, why := l.state(dir, remote, where, rec.key)
	return runClaim{key: rec.key, state: st, why: why, remote: remote}
}

// ownAdminEntry is the admin directory of rt itself, found from rt's own `.git` file and
// confirmed from the entry's `gitdir` back-pointer. It reads nothing of any sibling, so a
// malformed sibling entry cannot change the verdict on rt. Any link it cannot confirm is an
// error, which holds rt.
func ownAdminEntry(guard *pathGuard, rt string) (string, error) {
	if guard.commonDir == "" {
		return "", deskkit.Unverifiable("the shared git-common-dir is not resolved", nil)
	}
	raw, err := os.ReadFile(filepath.Join(rt, ".git"))
	if err != nil {
		return "", deskkit.Unverifiable("cannot read the worktree's .git file", err)
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(string(raw)), "gitdir:")
	target = strings.TrimSpace(target)
	if !ok || target == "" {
		return "", deskkit.Unverifiable("the worktree's .git file names no admin directory", nil)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(rt, target)
	}
	admin := resolvePath(target)
	if filepath.Dir(admin) != filepath.Join(guard.commonDir, "worktrees") {
		return "", deskkit.Unverifiable("the worktree's admin directory is not under this repository's registry", nil)
	}
	back, err := os.ReadFile(filepath.Join(admin, "gitdir"))
	pointer := strings.TrimSpace(string(back))
	if err != nil || pointer == "" {
		return "", deskkit.Unverifiable("cannot read the worktree's admin back-pointer", err)
	}
	if !filepath.IsAbs(pointer) {
		pointer = filepath.Join(admin, pointer)
	}
	if resolvePath(filepath.Dir(pointer)) != rt {
		return "", deskkit.Unverifiable("the worktree's admin entry points at another path", nil)
	}
	return admin, nil
}

// claimForRemove is `remove`'s claim verdict for its one target. It reads the run record from
// the target's OWN admin entry (no git process: the git-exec ratchet, and no read of any
// sibling entry), so a broken sibling cannot change the verdict on a home with no run key,
// while `remove` and `prune` still read one home's run record the same way. claimRepo is
// `--claim-repo` (assertClaimRepo); a contradiction with the record is the error.
func claimForRemove(guard *pathGuard, dir, rt, claimRepo string) (runClaim, error) {
	admin, err := ownAdminEntry(guard, rt)
	if err != nil {
		return runClaim{key: "?", state: claimUnknown, why: "could-not-check: run key (" + errText(err) + ")"}, nil
	}
	rec, err := runRecordInAdmin(admin)
	if err != nil {
		return runClaim{key: "?", state: claimUnknown, why: "could-not-check: run key (" + errText(err) + ")"}, nil
	}
	if rec, err = assertClaimRepo(rec, claimRepo); err != nil {
		return runClaim{}, err
	}
	sc := &sweepCtx{commonDir: guard.commonDir}
	return sc.claimFromRecord(dir, rec), nil
}

// claimStillFree re-reads the claim listing for key on remote (the claim's own repository, as
// claimFromRecord named it), bypassing the sweep's cached read, just before a destructive
// step. It is "" only when the fresh listing is readable and does not hold key; otherwise the
// reason the step must not go ahead.
func claimStillFree(dir, remote, key string) string {
	keys, err := listDispatchClaims(dir, remote)
	if err != nil {
		return "could-not-check: dispatch claim " + key + " (re-read before removal failed: " + errText(err) + ")"
	}
	if keys[key] {
		return "dispatch claim " + key + " was re-acquired after the sweep's listing; kept"
	}
	return ""
}
