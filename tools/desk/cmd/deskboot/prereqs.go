package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// hostPlatform is the `<os>-<arch>` token the per-platform desk-tools pin line is spelled
// with — the same token deskboard's drift banner and deskinstall resolve.
var hostPlatform = runtime.GOOS + "-" + runtime.GOARCH

// inTreeDeskRef is the source-tree provenance: origin/main's tools/desk tree. It is the
// FULLY-QUALIFIED remote-tracking ref (#885), the same one deskboard's fallback reads, so a
// stray local branch named `origin/main` can never be what proves provenance.
const inTreeDeskRef = "refs/remotes/origin/main:tools/desk"

// stepPrereqs proves the setup the role's FIRST SWEEP reads actually exists (#2986).
//
// Every earlier step proves something real, and none of them proved these two:
//
//   - a FORGE MAPPING for the repo in the roster. The preflight's write-transport check and
//     the token mint both let the checkout's origin host answer "which forge?", so they pass
//     with no mapping at all. `desksupervise status --stops` resolves its endpoint from the
//     roster ALONE — deliberately, because in a cross-repo sweep the checkout's origin names
//     an unrelated repo's forge — and so it was the first command to fail.
//   - DESK-TOOLS PROVENANCE: a desk-tools pin line (bare or this host's platform), a
//     desk-tools-source line with a full commit, or the in-tree source ref. worktree-current
//     proves the local pin file EQUALS origin/main's; two equal statusgen-only files pass that
//     and still leave deskboard's drift banner STALE-UNKNOWN on its first read. Equality is
//     not presence, so presence is checked here, with the board's own reader.
//
// It is a READ: no network, no credential, no write. The forge check consults the roster and
// nothing else, so it can never infer a credential endpoint from an unrelated checkout; the
// pin check reads files and one local ref. Both gaps are reported in ONE refusal, so an
// operator fixes the setup in one pass. The check is the same for every role: each role's
// sweep runs desk-tools binaries, and every one that reads stops or the board needs both.
func stepPrereqs(o bootOpts) (string, error) {
	repo, err := o.resolveRepo()
	if err != nil {
		return "", err
	}
	var missing []string
	forge, gap := forgeMapping(repo)
	if gap != "" {
		missing = append(missing, gap)
	}
	prov, gap := deskToolsProvenance(o)
	if gap != "" {
		missing = append(missing, gap)
	}
	if len(missing) > 0 {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the setup %s's first sweep reads is incomplete for %s — %s. The earlier steps passed "+
				"without these, and the sweep's first read would not: fix the setup, then re-run deskboot.",
			stepFirstSweep, o.role, repo, strings.Join(missing, "; ")), nil)
	}
	return fmt.Sprintf("%s → forge %s (%s); desk-tools provenance %s", repo, forge, deskkit.EnvRepoForges, prov), nil
}

// forgeMapping returns the forge the ROSTER names for repo, or a gap naming the exact line
// to add. The origin URL is passed as "" on purpose: only the roster may answer.
func forgeMapping(repo string) (forge, gap string) {
	owner, name, _ := strings.Cut(repo, "/")
	res, err := deskkit.ForgeKindForRepoRemote(deskkit.ForgeRepo{Owner: owner, Name: name}, "")
	if err == nil {
		return string(res.Kind), ""
	}
	return "", fmt.Sprintf(
		"no forge mapping names %s: add `%s=%s=github` (or `=gitlab`) to %s — append it with a comma "+
			"to an existing %s line, operator-approved like every roster value. The origin remote's host "+
			"is deliberately NOT accepted: `desksupervise status --stops` resolves its endpoint from the "+
			"roster alone, so a forge the origin implies is one the stop inspection cannot use. A "+
			"malformed entry empties the whole map; the assay-config echo names it",
		repo, deskkit.EnvRepoForges, repo, deskkit.ConfigHomePath(), deskkit.EnvRepoForges)
}

// deskToolsProvenance returns what proves which desk tools this setup runs, or a gap naming
// the remedy. The order is deskboard's: release pin (bare, then this host's platform), then a
// full-commit source pin, then the in-tree source ref.
func deskToolsProvenance(o bootOpts) (prov, gap string) {
	dir := deskkit.NearestPinRoot(o.root)
	pinFile := filepath.Join(dir, deskkit.AssayVersionsFile)
	if tag, artifact, ok := deskkit.DeskToolsReleasePin(dir, hostPlatform); ok {
		return fmt.Sprintf("%s %s in %s", artifact, tag, pinFile), ""
	}
	if dir != "" {
		if ref, hit, err := deskkit.SourcePin(dir, deskkit.DeskToolsArtifact); err == nil && hit && ref.Commit != "" {
			return fmt.Sprintf("%s %s in %s", ref.Artifact, ref.Commit, pinFile), ""
		}
	}
	if r := runCmd(o.root, "git", "rev-parse", "--verify", "--quiet", inTreeDeskRef); r.err == nil && firstLine(r.stdout) != "" {
		return "in-tree " + inTreeDeskRef, ""
	}
	where := "no " + deskkit.AssayVersionsFile + " at or above " + o.root
	if dir != "" {
		where = pinFile + " carries no `desk-tools`, `" + deskkit.DeskToolsPlatformArtifact(hostPlatform) +
			"` or full-commit `desk-tools-source` line (statusgen lines do not pin the desk tools)"
	}
	return "", fmt.Sprintf(
		"no desk-tools provenance: %s, and %s does not resolve. Pin the desk tools — `assay-install.sh pin "+
			"--kind desk-tools …` (the install-desk-tools setup step) — and land the pin on main, or boot "+
			"from the source tree. deskboard's drift check reads exactly these, and without one its first "+
			"read is STALE-UNKNOWN",
		where, inTreeDeskRef)
}
