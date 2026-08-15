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
// than restated. ground-truth/04 retired the hand table that used to sit at this
// spot (#276: the org topology carried in five parallel tables with no generator
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
// It is now a THIN WRAPPER over the generic ArtifactPin (distribution/04): the
// reader was generalised to any artifact line, and this preserves the exact
// behaviour every existing caller depends on — including the trailing-space
// prefix match, so it still never matches a `statusgen-<platform>` line.
func StatusgenPin(root string) (tag, sha string, err error) {
	return ArtifactPin(root, "statusgen")
}
