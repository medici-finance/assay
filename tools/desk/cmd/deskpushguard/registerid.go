// Register-id collision detection — the mechanical form of #72's second unique failure
// shape (tracker#566 reused `id: F-39`).
//
// A worktree correctly cut from origin/main can still collide with a SIBLING branch that
// was ALSO correctly cut from origin/main, if both independently add a register entry
// (docs/streams/findings/, docs/streams/intake/) whose `id:`
// frontmatter happens to match. Neither branch contains the other's commits — there is no
// foreign commit and no merge masquerade for foreigncommit.go's checks to find — so today the collision is invisible on either branch alone and is only
// discovered once the second one merges and statusgen's existing duplicate-id lint reds
// main's CI (statusgen/registers.go: duplicateIDs).
//
// This file closes that gap at push time: for every register entry newly ADDED or MODIFIED on
// the pushed ref whose id is NEW relative to origin/main, it checks whether the same id is
// already claimed by a register entry present on some OTHER remote branch that is not itself
// already merged into origin/main (an in-flight sibling). It enumerates candidate siblings from
// already-fetched remote-tracking branches (`git branch -r`), the same data source
// foreigncommit.go's checks would use, so the ordinary push adds no network/gh call, and it
// fails open on any ambiguity, matching this tool's stated Fail-OPEN contract (main.go's
// package doc). The one network touch is deliberate and rare (#189): when a collision would
// otherwise be reported, the sibling ref's liveness is confirmed against origin with a single
// `git ls-remote` so a stale remote-tracking ref (a merged-and-deleted sibling) is not treated
// as a live competing claim — a probe that runs only on the collision path and degrades to
// could-not-check, never an error, when origin is unreachable.
//
// ADVISORY, aligned with the authoritative gate. statusgen's duplicateIDs lint
// (splitFrontmatter + yaml.v3) is the authoritative fail-closed CI gate; this pre-push layer
// is an additive advisory signal that always defers to it. So the local signal AGREES with the
// authoritative one instead of failing open more often than it must, the id is extracted here
// with the SAME frontmatter-scoped yaml.v3 semantics statusgen uses (see extractRegisterID) —
// not the looser unanchored line regex it previously used, which diverged from statusgen on
// `id :` (whitespace before the colon), duplicate `id:` keys, and an `id:` line appearing in
// the body OUTSIDE the frontmatter block.
//
// NOTE on shared helpers: #746 (foreigncommit.go) has landed, so this file now shares that
// file's shaRe / gitOut / branchIsAncestorOfMain helpers rather than reproducing them under a
// regID* prefix — collapsing the duplication flagged inline on #759.
package main

import (
	"errors"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"gopkg.in/yaml.v3"
)

// registerEntryDirs are the register entry directories whose per-file `id:` frontmatter
// must be collision-free (spec/registers-v1.md). Trailing slash matches this file's own
// git-pathspec usage below.
var registerEntryDirs = []string{"docs/streams/findings/", "docs/streams/intake/"}

// registerIDCollision names one newly-pushed register entry whose id is already claimed by
// an entry on another in-flight branch.
type registerIDCollision struct {
	branch, id, ownPath, sourceBranch, sourcePath string
	sourceLive                                    refLiveness
}

// refLiveness is the tri-state answer to "is this remote-tracking ref still a live head on
// origin?" — the three-state instrument this check now reports per collision (#189, ask 3) so a
// stale local remote-tracking artifact is never presented as a live competing claim, and a
// collision whose source ref could not be verified against origin is reported AS unverified
// rather than rounded up to a confident "live".
type refLiveness int

const (
	livenessUnknown refLiveness = iota // could-not-check — origin unreachable (e.g. an offline push)
	livenessLive                       // ls-remote confirms the head is present on origin
	livenessStale                      // ls-remote confirms the head is ABSENT on origin
)

// note renders the per-refusal liveness clause (#189, ask 3), so a reader can tell a
// confirmed-live competing claim from one whose source ref could not be verified against origin.
func (l refLiveness) note() string {
	switch l {
	case livenessLive:
		return "source ref live"
	case livenessStale:
		// Never reached in a refusal — stale collisions are DROPPED before reporting (#189,
		// ask 2). Kept for completeness so a future caller printing every state is correct.
		return "source ref stale"
	default:
		return "source ref liveness unverified (origin unreachable)"
	}
}

// inRegisterDir reports whether path falls under one of registerEntryDirs.
func inRegisterDir(path string) bool {
	for _, d := range registerEntryDirs {
		if strings.HasPrefix(path, d) {
			return true
		}
	}
	return false
}

// splitFrontmatter isolates a register entry file's leading YAML frontmatter block, kept
// behaviourally identical to statusgen's canonical splitFrontmatter (statusgen/parse.go) so
// this advisory layer and the authoritative CI gate agree on WHERE the frontmatter is.
// Line-based and tolerant: the opening and closing fences only need to trim to "---", and
// CRLF is normalized to LF up front so a Windows-authored (CRLF) file parses identically to
// an LF one instead of silently swallowing the whole file as frontmatter. Returns
// (frontmatter, body, error); an absent first fence or an unterminated block is an error, on
// which the caller fails open.
func splitFrontmatter(content string) (string, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", errors.New("no frontmatter: first line must be ---")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), nil
		}
	}
	return "", "", errors.New("unterminated frontmatter")
}

// registerIDFrontmatter is the single frontmatter field this guard reads — the same `id` key
// statusgen's register-entry structs unmarshal (statusgen/registerentries.go).
type registerIDFrontmatter struct {
	ID string `yaml:"id"`
}

// extractRegisterID returns the `id:` frontmatter value from a register entry file's content,
// read with the SAME semantics as statusgen's authoritative duplicateIDs gate: splitFrontmatter
// to scope the read to the leading frontmatter block, then yaml.v3 to read the `id` key.
//
// It returns "" (skip — fail open) when there is no frontmatter, the frontmatter does not
// parse as yaml (yaml.v3 rejects duplicate `id:` keys — a shape statusgen also reds on
// authoritatively), or no `id` key is present (not every file under a register dir is a
// conforming entry — e.g. a generated view or a README; those are silently skipped).
//
// This deliberately replaces the earlier unanchored `(?m)^id:\s*"?(...)"?\s*$` line regex,
// whose divergences from statusgen this advisory layer used to miss: `id :` (whitespace before
// the colon, which yaml.v3 accepts and the regex did not), duplicate `id:` keys (the regex
// silently took the first; yaml.v3 rejects the file), and an `id:` line matched in the body
// OUTSIDE the frontmatter block (the multiline regex scanned the whole file).
func extractRegisterID(content string) string {
	fm, _, err := splitFrontmatter(content)
	if err != nil {
		return ""
	}
	var e registerIDFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &e); err != nil {
		return ""
	}
	return e.ID
}

// idClaimedOnMainAtPath reports whether origin/main ALREADY carries this id at THIS same path —
// i.e. the id is a pre-existing entry, not a new claim this push introduces.
//
// #189: a branch that edits an existing findings entry for an unrelated reason (repointing a
// backtick path, fixing a typo in the body) leaves the `id:` untouched, so the id is already
// on main in the same file. The check used to treat any ADDED-or-MODIFIED entry as staking a
// fresh claim on its id, which made every such branch collide with every remote-tracking ref
// that merely carried an unmodified copy of the same entry — a refusal that could not be
// satisfied as written, since renaming an id already on main is the actual defect. An id is a
// CLAIM only when it is NEW relative to origin/main: the file is newly added (absent on main —
// git show errors, so this returns false), or the id was changed in place (the
// F-original -> F-collide MODIFY shape, where main's id at this path differs, so this returns
// false and the new id is still flagged).
func idClaimedOnMainAtPath(dir, originMain, path, id string) bool {
	repo, err := openRepo(dir)
	if err != nil {
		return false
	}
	content, err := repo.FileAt(originMain, path)
	if err != nil {
		return false // path absent on main — a genuinely new file, so its id is a new claim
	}
	return extractRegisterID(content) == id
}

// remoteBranchNames returns every remote-tracking branch's short name (e.g.
// "origin/sibling"), matching `git branch -r`'s own listing (minus its leading marker/
// whitespace formatting, already handled by the caller's own trimming when it was
// git-binary output). gitcore's Refs does not surface a SYMBOLIC ref such as the
// `origin/HEAD -> origin/main` alias `git branch -r` prints (see gitcore.go's
// RefsContaining doc) — harmless here: the alias always mirrors a concrete branch ref
// (e.g. origin/main) that IS returned, and callers already exclude that ref by name.
func remoteBranchNames(repo *gitcore.Repo) ([]string, error) {
	refs, err := repo.Refs()
	if err != nil {
		return nil, err
	}
	const prefix = "refs/remotes/"
	var out []string
	for name := range refs {
		if strings.HasPrefix(name, prefix) {
			out = append(out, strings.TrimPrefix(name, prefix))
		}
	}
	sort.Strings(out)
	return out, nil
}

// remoteHeadLiveness asks origin DIRECTLY whether remoteRef (an `origin/<name>` remote-tracking
// ref) still corresponds to a live head — the resolution of #189's stale-ref half.
//
// A sibling branch that was merged and DELETED leaves refs/remotes/origin/<name> behind in this
// clone until someone prunes; `git branch -r` still lists it, and the check compared against
// that stale ref, reporting a collision against a branch that no longer exists. `git branch -r`
// cannot distinguish live from stale — only origin can — so this consults origin via
// `git ls-remote`.
//
// It runs ONLY when a collision would otherwise be reported (rare), so an ordinary push pays no
// network cost, and it never returns an error: an unreachable origin (an offline push) is
// reported as livenessUnknown, keeping this tool's offline-push and fail-open contracts intact.
// The fully-qualified `refs/heads/<name>` pattern is used so a head named `foo` cannot be
// matched by a stray `bar/foo` on the remote.
func remoteHeadLiveness(dir, remoteRef string) refLiveness {
	head := strings.TrimPrefix(remoteRef, "origin/")
	out, err := gitOut(dir, "ls-remote", "--heads", "origin", "refs/heads/"+head)
	if err != nil {
		return livenessUnknown // origin unreachable (offline) — could-not-check
	}
	if strings.TrimSpace(out) == "" {
		return livenessStale // origin has no such head — the local remote-tracking ref is stale
	}
	return livenessLive
}

// checkRegisterIDCollisions inspects register entry files newly ADDED or MODIFIED by localSHA
// relative to origin/main and reports any whose id is NEW relative to origin/main and is also
// claimed by a register entry file present on some other LIVE remote branch that is not itself
// already merged into origin/main.
//
// Two #189 scoping rules keep this from refusing a push it cannot let the author satisfy:
//
//   - NEW ids only. An id already present on origin/main in the SAME file is a pre-existing
//     entry, not a claim (idClaimedOnMainAtPath) — a branch editing an existing findings entry
//     for an unrelated reason does not stake a fresh claim on its id and cannot collide.
//   - LIVE source refs only. Before reporting a collision the source ref's liveness is
//     confirmed against origin (remoteHeadLiveness); a stale remote-tracking ref left behind by
//     a merged-and-deleted sibling is dropped, and each reported collision carries the source
//     ref's liveness (live / could-not-check) so a stale-ref artifact is distinguishable from a
//     real, in-flight collision.
//
// dir is the repository to run git in (empty = process cwd). ownBranch's own remote-tracking
// ref (if already pushed) is excluded from the "other branch" search so an UPDATE push never
// flags itself against its own prior state.
//
// Fails open (nil, nil) on any ambiguity: origin/main unresolvable, localSHA not a
// well-formed object id or not resolvable, or a git error partway through — per-branch and
// per-file errors just skip that branch/file rather than aborting the whole check.
func checkRegisterIDCollisions(dir, ownBranch, localSHA string) ([]registerIDCollision, error) {
	if !shaRe.MatchString(localSHA) {
		return nil, nil
	}
	// FULLY-QUALIFIED remote-tracking ref, not the bare short name `origin/main`
	// (#885): a stray local `refs/heads/origin/main` decoy would otherwise shadow
	// the real remote tip and drive the added-file diff off a stale base. The
	// sibling foreigncommit.go in this same binary already spells it in full — reuse
	// its resolveOriginMain rather than re-resolving here.
	originMain, err := resolveOriginMain(dir)
	if err != nil || originMain == "" {
		return nil, nil // cannot resolve origin/main — nothing to compare against
	}
	repo, err := openRepo(dir)
	if err != nil {
		return nil, nil
	}
	if ok, _ := repo.CommitVerifyQuiet(localSHA); !ok {
		return nil, nil // localSHA doesn't resolve in this repo — fail open
	}

	// Files newly added OR modified by this push. MODIFY is included alongside ADD: an id
	// changed in place on an existing entry is a MODIFY, not an ADD, and an ADD-only scan
	// would let it past this advisory layer even though statusgen's duplicateIDs gate still
	// sees the changed id.
	//
	// Repo-wide (no pathspec restriction to registerEntryDirs): gitcore has no pathspec
	// filtering, so this filters to inRegisterDir client-side below instead — the same
	// tradeoff deskscanbody's brief-03 migration made for its own repo-wide diff.
	changes, derr := repo.DiffNameStatus(originMain, localSHA)
	if derr != nil {
		return nil, nil
	}

	ownIDs := map[string]string{} // id -> own path
	for _, c := range changes {
		if c.Status != "A" && c.Status != "M" {
			continue
		}
		if !inRegisterDir(c.Path) {
			continue
		}
		content, cerr := repo.FileAt(localSHA, c.Path)
		if cerr != nil {
			continue // fail open on this file
		}
		if id := extractRegisterID(content); id != "" {
			// #189: only NEW ids are claims. An id already present on origin/main in this same
			// file is a pre-existing entry the push merely carries or edits for an unrelated
			// reason (a repointed backtick, a typo fix), and cannot collide with anything.
			if idClaimedOnMainAtPath(dir, originMain, c.Path, id) {
				continue
			}
			ownIDs[id] = c.Path
		}
	}
	if len(ownIDs) == 0 {
		return nil, nil
	}

	remoteBranches, berr := remoteBranchNames(repo)
	if berr != nil {
		return nil, nil
	}
	ownRemote := "origin/" + ownBranch
	livenessCache := map[string]refLiveness{} // #189: probe each source ref's liveness at most once
	var collisions []registerIDCollision
	for _, b := range remoteBranches {
		// Skip git's symbolic default-branch pointer `origin/HEAD` by EXACT match — the
		// prior HasSuffix(b, "HEAD") was too broad and would also skip a real branch whose
		// name merely ends in "HEAD" (e.g. origin/detached-HEAD), silently leaving its ids
		// unchecked. gitcore's ref enumeration does not surface the symbolic origin/HEAD
		// alias at all (see gitcore.go's RefsContaining doc), so these two checks are now
		// belt-and-braces rather than load-bearing, kept for defense and documentation.
		if b == "" || b == ownRemote || b == "origin/main" || b == "origin/HEAD" || strings.Contains(b, "->") {
			continue
		}
		isAnc, determinate := branchIsAncestorOfMain(repo, b, originMain)
		if !determinate || isAnc {
			// Already merged into origin/main (or unresolvable) — its ids are already
			// covered by the origin/main-relative diff above via a fresh checkout, or we
			// simply can't say anything useful; either way, skip.
			continue
		}

		paths, lserr := repo.Files(b)
		if lserr != nil {
			continue
		}
		for _, path := range paths {
			if !inRegisterDir(path) {
				continue
			}
			content, cerr := repo.FileAt(b, path)
			if cerr != nil {
				continue
			}
			id := extractRegisterID(content)
			if id == "" {
				continue
			}
			if ownPath, clash := ownIDs[id]; clash {
				// Skip the IDENTICAL-PATH case: the same register entry file touched on
				// BOTH this branch and the sibling is a MERGE concern, not the
				// added-vs-added id collision this guard exists to catch. Two branches
				// modifying one path resolve to a SINGLE file on merge, so no duplicate id
				// ever reaches main and statusgen's authoritative duplicateIDs gate never
				// reds on it — flagging it here over-fires and blocks a legitimate push.
				// The genuine collision is always two DIFFERENT files independently
				// claiming the same id (distinct paths, which a git tree guarantees are
				// distinct files); that case falls through and is still reported below.
				// Done before the liveness probe below so an identical-path pair never
				// pays the collision-path network cost (ls-remote).
				if ownPath == path {
					continue
				}
				live, cached := livenessCache[b]
				if !cached {
					live = remoteHeadLiveness(dir, b)
					livenessCache[b] = live
				}
				if live == livenessStale {
					// #189: the sibling's remote-tracking ref is a stale local artifact — origin
					// confirms the head is gone (merged and deleted). It is not a live claim, so
					// it cannot collide; skip it rather than refuse an unsatisfiable push.
					continue
				}
				collisions = append(collisions, registerIDCollision{
					branch:       ownBranch,
					id:           id,
					ownPath:      ownPath,
					sourceBranch: b,
					sourcePath:   path,
					sourceLive:   live,
				})
			}
		}
	}
	return collisions, nil
}
