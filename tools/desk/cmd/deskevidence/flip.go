package main

// flip.go — `deskevidence flip`: the sanctioned implemented → verified board flip
// for a gate: model brief (#2074).
//
// The flip lands as a LOCAL commit on a branch of the verifier's worktree,
// authored AND committed by the verifier App's bound bot identity
// (deskkit.RoleWorktreeCommitIdentity("verifier")), so the draft PR that
// carries it passes deskpr's publish-identity gate exactly as that gate stands.
// It never pushes, never opens the PR, and never runs on main/master — the
// Contents-API Evidence landing (the main verb) is unchanged and separate.
//
// The stamp is not typed by anyone: `statusgen verifyflip` derives it from the
// brief's recorded strict **VERIFY: PASS** and refuses on a verdict, sha or
// runner mismatch, gate: human, any risk: yes and irreversible. This verb adds
// the tree-level checks around that write: the verified-closure check, a diff
// that is exactly the one README line, the identity of the commit it made, the
// publish-identity gate over the branch, and the statusgen PROBLEM-diff (no
// PROBLEM introduced) judged on the COMMIT, since some checks read history.

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const flipUsage = `deskevidence flip — commit one gate: model brief's implemented → verified flip as the verifier App, on a branch.

USAGE:
  deskevidence flip --root <worktree> --brief <stream>/<NN> --sha <sha> [--base main] [--dry-run]

Runs ` + "`statusgen verifyflip`" + ` (the stamp is derived from the recorded strict
**VERIFY: PASS**; refused on a verdict/sha/runner mismatch, gate: human, any risk: yes,
irreversible), refuses if the flip introduces a statusgen PROBLEM or the tree does not
then present a verified closure, and commits the README alone as the verifier App's bot
identity on the current branch. Refused on main/master, a detached HEAD, staged changes,
a README that differs from HEAD, or a branch whose commits the publish-identity gate
would refuse. Nothing is pushed: open the draft PR from the worktree with deskpr.

The after-lint judges the COMMITTED flip (some checks read history, not the tree), so a
refusal there — and --dry-run — makes the commit and then takes it back off the branch
(reset --soft to the parent, README restored): the branch, index and tree end as they began.`

// verifyFlipFn runs `statusgen verifyflip` (seam: tests stub it). It returns the
// README path relative to root and the derived stamp.
var verifyFlipFn = statusgenVerifyFlip

// checkVerifiedFn is the verified-closure check (seam: tests stub it).
var checkVerifiedFn = checkVerifiedClosure

func runFlip(args []string) error {
	fs := flag.NewFlagSet("flip", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", "", "")
	brief := fs.String("brief", "", "")
	sha := fs.String("sha", "", "")
	base := fs.String("base", "main", "")
	dryRun := fs.Bool("dry-run", false, "")
	if err := fs.Parse(args); err != nil {
		if deskkit.IsHelpRequest(err) {
			fmt.Fprintln(stderr, flipUsage)
			return deskkit.ErrHelpRequested
		}
		return deskkit.Refused("flip: " + err.Error() + "\n" + flipUsage)
	}
	if fs.NArg() > 0 || *root == "" || *brief == "" || *sha == "" || *base == "" {
		return deskkit.Refused("flip: --root, --brief and --sha are required\n" + flipUsage)
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		return deskkit.Unverifiable("flip: cannot resolve --root", err)
	}

	// The worktree: its own toplevel, on a named branch that is not the base.
	top, err := gitOut(abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return deskkit.Unverifiable("flip: --root is not a git worktree", err)
	}
	if !samePath(top, abs) {
		return deskkit.Refused("flip: --root must be the worktree's toplevel (" + top + ")")
	}
	branch, err := gitOut(abs, "symbolic-ref", "-q", "--short", "HEAD")
	if err != nil || branch == "" {
		return deskkit.Refused("flip: HEAD is detached — check out a branch for the flip PR first")
	}
	if branch == "main" || branch == "master" || branch == *base {
		return deskkit.Refused("flip: refusing to commit on " + branch + " — the flip lands on a branch and reaches " + *base + " by a reviewed PR")
	}
	if _, err := gitOut(abs, "diff", "--cached", "--quiet"); err != nil {
		return deskkit.Refused("flip: the index has staged changes — the flip commit carries the README alone")
	}
	// Every commit already on the branch must pass the gate the PR will meet.
	if err := publishIdentityGate(abs, *base); err != nil {
		return err
	}

	name, email, err := deskkit.RoleWorktreeCommitIdentity("verifier")
	if err != nil {
		return err
	}

	// Plan: every refusal statusgen owns, nothing written.
	rel, stamp, err := verifyFlipFn(abs, *brief, *sha, name, true)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(rel, "docs/streams/") || filepath.Base(rel) != "README.md" || filepath.Clean(rel) != filepath.FromSlash(rel) {
		return deskkit.Unverifiable(fmt.Sprintf("flip: statusgen named an unexpected README path %q", rel), nil)
	}
	readme := filepath.Join(abs, filepath.FromSlash(rel))
	if _, err := gitOut(abs, "diff", "--quiet", "HEAD", "--", rel); err != nil {
		return deskkit.Refused("flip: " + rel + " differs from HEAD — commit or discard that change before the flip")
	}
	orig, err := os.ReadFile(readme)
	if err != nil {
		return deskkit.Unverifiable("flip: cannot read "+rel, err)
	}
	restore := func() { _ = os.WriteFile(readme, orig, 0o644) }

	// The PROBLEM baseline, read on the tree as it stands.
	before, err := statusgenLintFn(abs)
	if err != nil {
		return err
	}

	// The write: statusgen's own refusals run again on the live tree.
	if _, stamp2, err := verifyFlipFn(abs, *brief, *sha, name, false); err != nil {
		restore()
		return err
	} else if stamp2 != stamp {
		restore()
		return deskkit.Unverifiable("flip: statusgen derived two different stamps for one tree", nil)
	}
	if _, err := checkVerifiedFn(abs, *brief); err != nil {
		restore()
		return err
	}
	numstat, err := gitOut(abs, "diff", "--numstat")
	if err != nil || numstat != "1\t1\t"+rel {
		restore()
		return deskkit.Refused(fmt.Sprintf("flip: the working-tree change is not exactly one line of %s (numstat %q)", rel, numstat))
	}

	// The commit. Several lint checks (the human-stamp gain gate among them) judge
	// COMMITTED history against the base, so the after-lint runs on the commit; a
	// refusal, or --dry-run, then takes that unpushed commit back off the branch.
	msg := fmt.Sprintf("verify(%s): implemented → verified\n\nVerified: %s\nStamp derived by deskevidence flip from the recorded strict VERIFY: PASS.", *brief, stamp)
	commit := exec.Command("git", "-C", abs, "-c", "commit.gpgsign=false", "commit", "-q", "-m", msg, "--", rel)
	commit.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME="+name, "GIT_AUTHOR_EMAIL="+email,
		"GIT_COMMITTER_NAME="+name, "GIT_COMMITTER_EMAIL="+email)
	if out, err := commit.CombinedOutput(); err != nil {
		restore()
		return deskkit.Unverifiable("flip: git commit failed: "+strings.TrimSpace(string(out)), err)
	}
	got, err := gitOut(abs, "log", "-1", "--format=%an%x00%ae%x00%cn%x00%ce%x00%H")
	parts := strings.Split(got, "\x00")
	if err != nil || len(parts) != 5 {
		return deskkit.Unverifiable(fmt.Sprintf("flip: cannot read back the flip commit (%q) — inspect HEAD before any push", got), err)
	}
	flipSHA := parts[4]
	uncommit := func() error {
		if head, herr := gitOut(abs, "rev-parse", "HEAD"); herr != nil || head != flipSHA {
			return deskkit.Unverifiable("flip: HEAD moved off the flip commit "+flipSHA+" — inspect the branch before any push", herr)
		}
		if _, rerr := gitOut(abs, "reset", "-q", "--soft", "HEAD~1"); rerr != nil {
			return deskkit.Unverifiable("flip: cannot take the flip commit "+flipSHA+" back — drop it before any push", rerr)
		}
		if _, rerr := gitOut(abs, "reset", "-q", "--", rel); rerr != nil {
			return deskkit.Unverifiable("flip: cannot unstage "+rel+" — inspect the index before any push", rerr)
		}
		restore()
		return nil
	}
	if parts[0] != name || parts[1] != email || parts[2] != name || parts[3] != email {
		if uerr := uncommit(); uerr != nil {
			return uerr
		}
		return deskkit.Unverifiable(fmt.Sprintf("flip: the commit did not carry the verifier identity (%q) — taken back", got), nil)
	}
	if err := publishIdentityGate(abs, *base); err != nil {
		if uerr := uncommit(); uerr != nil {
			return uerr
		}
		return err
	}
	after, err := statusgenLintFn(abs)
	if err != nil {
		if uerr := uncommit(); uerr != nil {
			return uerr
		}
		return err
	}
	if introduced := newProblems(before, after); len(introduced) > 0 {
		if uerr := uncommit(); uerr != nil {
			return uerr
		}
		return deskkit.Refused("flip: the flip would introduce statusgen PROBLEM(s) — the commit was taken back:\n" + strings.Join(introduced, "\n"))
	}
	if *dryRun {
		if uerr := uncommit(); uerr != nil {
			return uerr
		}
		fmt.Fprintf(stdout, "deskevidence flip: dry-run — %s would be committed on %s as %s <%s>\nverified-stamp: %s\nlint: no statusgen PROBLEM introduced (%d before, %d after)\n",
			rel, branch, name, email, stamp, len(before), len(after))
		return nil
	}
	fmt.Fprintf(stdout, "deskevidence flip: committed %s on %s as %s <%s>\nverified-stamp: %s\nlint: no statusgen PROBLEM introduced (%d before, %d after)\nnext: open the draft PR from this worktree with deskpr (nothing was pushed)\n",
		flipSHA, branch, name, email, stamp, len(before), len(after))
	return nil
}

// statusgenVerifyFlip shells `statusgen verifyflip`: exit 1 is a refusal, any
// other failure is could-not-check.
func statusgenVerifyFlip(root, brief, sha, runner string, dryRun bool) (rel, stamp string, err error) {
	if _, lerr := exec.LookPath("statusgen"); lerr != nil {
		return "", "", deskkit.Unverifiable("flip: statusgen is not on PATH", lerr)
	}
	args := []string{"verifyflip", "--root", root, "--brief", brief, "--sha", sha, "--runner", runner}
	if dryRun {
		args = append(args, "--dry-run")
	}
	cmd := exec.Command("statusgen", args...)
	cmd.Dir = os.TempDir()
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if rerr := cmd.Run(); rerr != nil {
		if e, ok := rerr.(*exec.ExitError); ok && e.ExitCode() == 1 {
			return "", "", deskkit.Refused("flip: " + strings.TrimSpace(errb.String()))
		}
		return "", "", deskkit.Unverifiable("flip: statusgen verifyflip could not check: "+strings.TrimSpace(errb.String()), rerr)
	}
	for _, ln := range strings.Split(out.String(), "\n") {
		if v, ok := strings.CutPrefix(ln, "readme: "); ok {
			rel = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(ln, "verified-stamp: "); ok {
			stamp = strings.TrimSpace(v)
		}
	}
	if rel == "" || stamp == "" {
		return "", "", deskkit.Unverifiable("flip: statusgen verifyflip printed no readme/stamp (an older statusgen?)", nil)
	}
	return rel, stamp, nil
}

// newProblems is after minus before: the PROBLEM lines the flip introduced.
func newProblems(before, after []string) []string {
	had := make(map[string]bool, len(before))
	for _, p := range before {
		had[p] = true
	}
	var out []string
	for _, p := range after {
		if !had[p] {
			had[p] = true
			out = append(out, p)
		}
	}
	return out
}

// gitOut runs git -C dir args and returns trimmed stdout.
func gitOut(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

func samePath(a, b string) bool {
	ra, ea := filepath.EvalSymlinks(a)
	rb, eb := filepath.EvalSymlinks(b)
	return ea == nil && eb == nil && ra == rb
}
