package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Isolation guard for --scan-issues (2026-08-13 incident).
//
// --scan-issues WRITES under --root: new placeholder files, close-out
// (retire/reactivate) edits, and un-block edits — and even --dry-run performs
// the un-block writes today. Run with --root pointed at the PRIMARY checkout a
// live session is homed in (the shared checkout), it silently dirties that
// tree: the 2026-08-13 incident left 11 stale-modified placeholders plus 13
// untracked ones in the shared assay checkout, none committed, and —
// because that checkout was BEHIND origin/main — recreated/edited placeholders
// from a STALE base, resurrecting content main had already superseded.
//
// The writeguard hook is not sufficient here: a session homed in the shared
// checkout with the human-claimed WRITEGUARD_SHARED_OK exemption (#1035) is
// exempt from the hook entirely, and non-hook contexts (cron, a human shell)
// never pass through it. So the tool refuses on its own: the sanctioned scan
// environment is an ISOLATED LINKED WORKTREE cut fresh from origin/main
// (intake-desk SKILL: worktree → scan → commit the delta → draft PR), and a
// linked worktree is mechanically distinguishable — its git dir differs from
// its git COMMON dir, while in a primary checkout the two are the same.
//
// Decision table for scanIsolationRefusal(root):
//
//   - root not a git checkout at all → ALLOWED (nothing shared to dirty;
//     keeps the offline t.TempDir() fixtures and any bare-directory use
//     working — mirrors writeguard's fail-open-on-unknowable philosophy).
//   - root inside a LINKED WORKTREE (git dir != git common dir) → ALLOWED —
//     the sanctioned isolation.
//   - root inside a PRIMARY checkout (git dir == git common dir) → REFUSED,
//     loudly, with the sanctioned recipe in the message — unless a human
//     claimed the override below.
//
// scanPrimaryOKEnv is the human-claimed override, mirroring the
// WRITEGUARD_SHARED_OK / ASSAY_MAIN_COMMIT_OK opt-in pattern: a primary
// checkout is refused BY DEFAULT, and someone who genuinely means to scan one
// (e.g. a fresh dedicated clone that is its own isolation) says so explicitly.
// It is deliberately NOT a roster key: an unknown key in roster.env fail-closes
// the whole trust roster, so operational toggles never go there.
const scanPrimaryOKEnv = "STATUSGEN_SCAN_PRIMARY_OK"

// scanGitDir runs `git -C dir rev-parse --<which>` and returns the trimmed
// output. Used with "absolute-git-dir" and "git-common-dir".
func scanGitDir(dir, which string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--"+which).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// scanIsolationRefusal reports why --scan-issues must refuse to run against
// root ("" = acceptable). See the decision table above. Any git failure —
// binary missing, root not a repository — is read as "not a git checkout" and
// ALLOWED: the guard exists to protect a shared PRIMARY checkout, and a root
// git cannot even describe is not one. A refusal names the incident, the
// sanctioned worktree flow, and the human override.
func scanIsolationRefusal(root string) string {
	if v := strings.TrimSpace(os.Getenv(scanPrimaryOKEnv)); v == "1" || strings.EqualFold(v, "true") {
		return ""
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	gitDir, err := scanGitDir(abs, "absolute-git-dir")
	if err != nil {
		return "" // not a git checkout (or no git) — nothing shared to protect
	}
	commonDir, err := scanGitDir(abs, "git-common-dir")
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(abs, commonDir)
	}
	gd, cd := filepath.Clean(gitDir), filepath.Clean(commonDir)
	if r, err := filepath.EvalSymlinks(gd); err == nil {
		gd = r
	}
	if r, err := filepath.EvalSymlinks(cd); err == nil {
		cd = r
	}
	if gd != cd {
		return "" // linked worktree — the sanctioned isolation
	}
	return fmt.Sprintf(
		"--root %s is the PRIMARY checkout of its repository (git dir == git common dir), "+
			"not an isolated linked worktree.\n"+
			"--scan-issues writes under the root (placeholder files, close-out and un-block edits in "+
			"docs/streams/%s/ — --dry-run included), and against a live shared checkout that leaves "+
			"uncommitted dirt and lets a stale base resurrect superseded placeholder content "+
			"(2026-08-13 incident).\n"+
			"Sanctioned flow: git fetch origin && git worktree add <abs-sibling-path> -b <branch> "+
			"refs/remotes/origin/main, run the scan there, then commit docs/streams/%s/ and open a "+
			"draft PR.\n"+
			"A HUMAN deliberately scanning a primary checkout (e.g. a fresh dedicated clone) can "+
			"override with %s=1.",
		abs, scanStreamName, scanStreamName, scanPrimaryOKEnv)
}
