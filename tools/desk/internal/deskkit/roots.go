package deskkit

// roots.go — where each allowed repo's streams live ON THIS MACHINE, for the
// cross-repo Next-up board.
//
// The methodology is going multi-repo: statusgen now emits one board per repo
// root, and `deskboard nextup` merges those boards into one queue. To do that it
// has to know which local directory holds which repo's `docs/streams/`.
//
// Two properties this file must keep:
//
//   - The repo SET is not widened here. Every configured root must already be in
//     the fixed allowed set; an override naming anything else is REFUSED.
//     Paths are machine-local configuration, so they are overridable; trust is
//     not, so it is not.
//   - Configuration is EXPLICIT. A root that is configured but unreadable is an
//     error, never a skip. The whole point of the multi-repo board is that a
//     repo's work cannot silently stop appearing on it.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/topology"
)

// RootsEnv names the environment variable that overrides root PATHS:
//
//	DESK_ROOTS="example-org/tracker=/src/tracker,medici-finance/assay=/src/toolkit"
//
// It replaces the defaults outright rather than merging, so a caller can state
// exactly which roots the board covers. It cannot introduce a repo outside the
// allowed set.
const RootsEnv = "DESK_ROOTS"

// defaultRootMap maps an allowed repo to its default local checkout path, using
// the sibling-checkout convention (a repo's siblings sit next to it). Paths are
// relative to the process's
// working directory, which for the desk tools is the primary repo root.
//
// Only repos whose streams statusgen actually generates a board for belong here.
// Adding one is a deliberate act: it puts that repo's briefs on the desk's
// Next-up queue and makes its absence a hard error — which is exactly why the
// set is DECLARED ONCE, in `topology.yaml` (repos[].root), and read here rather
// than restated. The topology-driven registry retired the hand table that used to sit at this
// spot (the org topology used to be carried in five parallel tables with no generator
// and no cross-check); TestTopologyDriftRegistry fails NAMING THIS SITE if a
// second copy comes back.
func defaultRootMap() map[string]string { return topology.Compiled().Roots() }

// RootConfig binds a repo (owner/name) to its local root directory.
type RootConfig struct {
	Repo string `json:"repo"`
	Path string `json:"path"`
}

// ConfiguredRoots returns the roots the multi-repo board covers, sorted by repo
// for deterministic output. It reads DESK_ROOTS when set, otherwise the
// compiled-in defaults.
//
// Errors are Refused (exit 5): a malformed override or a repo outside the fixed
// set is a caller mistake, and continuing on a partial roots list would produce a
// board that silently omits a repo.
func ConfiguredRoots() ([]RootConfig, error) {
	raw := strings.TrimSpace(os.Getenv(RootsEnv))
	if raw == "" {
		defaults := defaultRootMap()
		out := make([]RootConfig, 0, len(defaults))
		for repo, path := range defaults {
			out = append(out, RootConfig{Repo: repo, Path: path})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Repo < out[j].Repo })
		return out, nil
	}

	var out []RootConfig
	seen := map[string]bool{}
	for _, spec := range strings.Split(raw, ",") {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		repo, path, ok := strings.Cut(spec, "=")
		repo, path = strings.TrimSpace(repo), strings.TrimSpace(path)
		if !ok || repo == "" || path == "" {
			return nil, Refused("refused: " + RootsEnv + " entry " + spec + " is not <owner>/<repo>=<path>")
		}
		if !IsAllowedRepo(repo) {
			return nil, Refused("refused: " + RootsEnv + " names repo " + repo +
				" which is outside the fixed set — the repo set is compiled in and cannot be widened by env")
		}
		if seen[repo] {
			return nil, Refused("refused: " + RootsEnv + " names repo " + repo + " twice")
		}
		seen[repo] = true
		out = append(out, RootConfig{Repo: repo, Path: path})
	}
	if len(out) == 0 {
		return nil, Refused("refused: " + RootsEnv + " is set but names no roots")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Repo < out[j].Repo })
	return out, nil
}

// RootForRepo returns the configured local root for repo, or "" when the repo is
// not part of the multi-repo board.
func RootForRepo(repo string) string {
	roots, err := ConfiguredRoots()
	if err != nil {
		return ""
	}
	for _, r := range roots {
		if r.Repo == repo {
			return r.Path
		}
	}
	return ""
}

// ResolveRoot turns a configured root into an absolute path, verifying it is a
// readable directory that actually holds a `docs/streams/` tree.
//
// Fail-closed: a missing, unreadable or stream-less root is Unverifiable
// (exit 6), never a skip. Skipping is how a whole repo's briefs vanish from the
// board while it still reads as a successful run — the exact failure the
// multi-repo board exists to prevent.
func ResolveRoot(r RootConfig) (string, error) {
	abs, err := filepath.Abs(r.Path)
	if err != nil {
		return "", Unverifiable("cannot resolve root "+r.Path+" for repo "+r.Repo, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", Unverifiable("root "+abs+" for repo "+r.Repo+" is not readable "+
			"(set "+RootsEnv+" to point at your checkouts)", err)
	}
	if !info.IsDir() {
		return "", Unverifiable("root "+abs+" for repo "+r.Repo+" is not a directory", nil)
	}
	streams := filepath.Join(abs, "docs", "streams")
	if info, err := os.Stat(streams); err != nil || !info.IsDir() {
		return "", Unverifiable("root "+abs+" for repo "+r.Repo+" has no docs/streams/ — "+
			"it is not a statusgen root", err)
	}
	return abs, nil
}

// StatusgenPin returns the tag and sha256 pinned for statusgen in the repo root's
// `.assay-versions`. The pin file is the single record of which statusgen release
// the desk runs; a consumer that cannot read it cannot claim to be pinned, so a
// missing or malformed pin is Unverifiable (exit 6) rather than a default.
//
// It is now a THIN WRAPPER over the generic ArtifactPin: the
// reader was generalised to any artifact line, and this preserves the exact
// behaviour every existing caller depends on — including the trailing-space
// prefix match, so it still never matches a `statusgen-<platform>` line.
func StatusgenPin(root string) (tag, sha string, err error) {
	return ArtifactPin(root, "statusgen")
}

// RepoForLocalPath resolves a BARE LOCAL remote path to the allowed repo whose configured
// local root contains it, or ("", false) when it matches no root. It is the identity model
// deskgit's parseRepo needs for a host-less path: a local
// repo lives at an arbitrary absolute path, so its identity cannot be read off its last two
// path components — any directory can be NAMED to spell an allowed slug, and a
// `url.<base>.insteadOf` rewrite can point origin at one, so the lenient last-two rule let
// foreign content land on the tracking refs the desk loops merge. A local path's identity
// must instead come from the roots the desk was CONFIGURED to trust (roots.go / DESK_ROOTS),
// never from the path the caller supplies.
//
// The check is anchored two ways that a plain string compare is not:
//
//   - CANONICALISATION. Both the candidate and every configured root are reduced to their
//     symlink-resolved absolute form (canonicalPath) before comparison, so a symlink named
//     to spell an allowed slug — or one that dresses an outside directory up as a child of a
//     trusted root — cannot substitute identity. The two sides are always reduced the SAME
//     way, so the comparison is between real filesystem locations, not spellings of them.
//   - EQUALITY, not descendant containment. A candidate is admitted ONLY when it IS a
//     configured root, never merely a path under one. A checkout's root is the repo, but
//     `<root>/vendor/cache/x.git` is a DIFFERENT directory — and a caller who controls
//     `.git/config` also controls the worktree, so they can plant a bare repo under a
//     trusted root and, under descendant admission, have it inherit the root's identity
//     (the descendant-admission blocker): with DESK_ROOTS unset the compiled home root is `.`, which
//     canonicalises to the very worktree deskgit fetches into, turning the whole subtree
//     into a trust anchor. Equality closes that; a legitimate local remote names its
//     checkout root exactly. A separator-boundary compare (not a bare string prefix) is
//     what used to keep a sibling like `/src/toolkit-evil` from reading as under
//     `/src/toolkit`; equality subsumes it.
//   - A ROOT THAT RESOLVES TO THE PROCESS WORKING DIRECTORY is skipped outright. It names
//     "wherever the tool happens to be running" (the `.` default above), which cannot
//     anchor remote identity — so in the default configuration a bare local path is admitted
//     only once an operator sets DESK_ROOTS to real, non-here checkouts.
//
// Fail-closed: a candidate that resolves to no configured root, or an unloadable roots
// allowlist, returns ("", false). The caller must then REFUSE — there is no lenient fallback.
func RepoForLocalPath(path string) (repo string, ok bool) {
	target, terr := canonicalPath(path)
	if terr != nil {
		return "", false
	}
	roots, err := ConfiguredRoots()
	if err != nil {
		// A malformed or out-of-set DESK_ROOTS is a Refused config error; for identity
		// purposes it means "no trusted local roots", so nothing local is admitted.
		return "", false
	}
	cwd, cwdErr := canonicalWD()
	for _, r := range roots {
		root, rerr := canonicalPath(r.Path)
		if rerr != nil {
			continue
		}
		// A root meaning "here" cannot anchor identity — see the doc comment above.
		if cwdErr == nil && root == cwd {
			continue
		}
		if target == root {
			return r.Repo, true
		}
	}
	return "", false
}

// canonicalWD returns the canonicalised process working directory, used to reject a
// configured root that merely resolves to "here" and so cannot anchor remote identity.
func canonicalWD() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return canonicalPath(wd)
}

// canonicalPath reduces p to an absolute, symlink-resolved path for identity comparison.
// filepath.EvalSymlinks resolves the real on-disk location (defeating a symlink that spells
// an allowed slug); when p does not exist yet — or cannot be resolved — the cleaned absolute
// form is used so the function is TOTAL and comparison still happens on a normalised path
// rather than failing open. Relative paths resolve against the process working directory,
// matching ResolveRoot's use of filepath.Abs.
func canonicalPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return filepath.Clean(abs), nil
}
