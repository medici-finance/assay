package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// dryRunDurable is the DEFAULT Durable. It NEVER pushes to main, opens a PR, or
// files an issue — it prints what the real sink WOULD do. The dry-run ground rules forbid
// pushing to main / triggering workflows, and the autonomous cutover is BLOCKED-ON-HUMAN, so
// the safe default is an emitter. The real git-pushing sink (gitDurable) is wired only at
// cutover, on the owner's call.
type dryRunDurable struct{ out io.Writer }

func (d dryRunDurable) WriteEvidenceAndFlip(it loopengine.Item, evidence string, flip bool) error {
	action := "write Evidence (NO flip)"
	if flip {
		action = "write Evidence + flip implemented->verified, commit straight to main (push-race retry)"
	}
	fmt.Fprintf(d.out, "[dry-run] %s for %s (%s)\n%s\n", action, it.ID, it.BriefPath, evidence)
	return nil
}

func (d dryRunDurable) Checkpoint(it loopengine.Item, evidence string) (string, error) {
	fmt.Fprintf(d.out, "[dry-run] open checkpoint PR (irreversible, human flip) for %s\n", it.ID)
	return "dry-run://checkpoint/" + it.ID, nil
}

func (d dryRunDurable) RouteHuman(it loopengine.Item) (string, error) {
	fmt.Fprintf(d.out, "[dry-run] route %s to human (risk-flagged: label issue / checkpoint PR); drain continues\n", it.ID)
	return "dry-run://human/" + it.ID, nil
}

func (d dryRunDurable) FileBug(it loopengine.Item, detail string) (string, error) {
	fmt.Fprintf(d.out, "[dry-run] file `bug` issue for %s: %s\n", it.ID, detail)
	return "dry-run://bug/" + it.ID, nil
}

// gitDurable is the REAL durable sink used only at cutover. It carries the push-race retry
// loop (commit -> pull --rebase -> push) in code. `run` is the injectable command runner for
// the LOCAL steps (add, commit, the race's pull --rebase — go-git has no rebase, so that
// redesign is a follow-on) and `push` the injectable push, so the retry loop is unit-testable
// without a live remote — and so the reference build's tests never actually push.
//
// The PUSH is in-process (desktools-go-git/06): pushHeadToMain sends HEAD to refs/heads/main
// with gitcore.Push and the role's in-memory credential — no git child, no credential helper,
// no token in a URL or the environment — after the checkout's pre-push hook has run.
type gitDurable struct {
	root     string
	run      func(args ...string) (string, error)
	push     func() error
	maxRetry int
}

// newGitDurable builds the cutover sink for the checkout at root, pushing as role to repo
// (owner/name) — the forge endpoint and credential come from deskkit's custody for that pair.
func newGitDurable(root, repo, role string) *gitDurable {
	g := &gitDurable{
		root:     root,
		maxRetry: 5,
		run: func(args ...string) (string, error) {
			cmd := exec.Command(args[0], args[1:]...)
			b, err := cmd.CombinedOutput()
			return string(b), err
		},
	}
	g.push = func() error {
		return pushHeadToMain(root, func(originURL string) (deskkit.ForgeGitEndpoint, error) {
			return deskkit.ForgeGitEndpointForCheckout(repo, role, originURL)
		}, g.run)
	}
	return g
}

// pushHeadToMain pushes the checkout's HEAD commit to the remote's refs/heads/main,
// in-process. endpoint resolves the URL + in-memory credential from origin's fetch URL (a
// forge-kind hint only — the credential goes to the resolved forge's canonical URL). The
// refspec source is the RESOLVED commit, never "HEAD" (go-git silently skips a symbolic
// source), it carries no "+", and Force is never set: a push main has moved past is refused
// as non-fast-forward — the race the caller's retry loop resolves. The checkout's pre-push
// hook runs first (run answers its path), and its refusal stops the push.
func pushHeadToMain(root string, endpoint func(originURL string) (deskkit.ForgeGitEndpoint, error), run func(args ...string) (string, error)) error {
	repo, err := gitcore.Open(root)
	if err != nil {
		return err
	}
	origin, _ := repo.RemoteURL("origin")
	ep, err := endpoint(origin)
	if err != nil {
		return err
	}
	head, err := repo.Resolve("HEAD")
	if err != nil {
		return err
	}
	hook := deskkit.PrePushHook{
		Dir: root,
		HookPath: func() (string, error) {
			out, herr := run("git", "-C", root, "rev-parse", "--path-format=absolute", "--git-path", "hooks/pre-push")
			if herr != nil {
				return "", fmt.Errorf("%v: %s", herr, out)
			}
			return out, nil
		},
		Stderr: os.Stderr,
	}
	if herr := hook.Run("origin", ep.Opts.URL, "HEAD", head.String(), "refs/heads/main"); herr != nil {
		return herr
	}
	return repo.Push(gitcore.PushOpts{
		URL:      ep.Opts.URL,
		RefSpecs: []string{head.String() + ":refs/heads/main"},
		Auth:     ep.Opts.Auth,
	})
}

// commitPushRace commits the given paths on main and pushes, retrying the documented race
// resolution (pull --rebase then push) up to maxRetry times. It is the code home of the
// "Evidence-straight-to-main via the push race loop" rule (verify-desk skill git-push policy).
func (g *gitDurable) commitPushRace(paths []string, msg string) error {
	git := func(args ...string) (string, error) {
		full := append([]string{"git", "-C", g.root}, args...)
		return g.run(full...)
	}
	for _, p := range paths {
		if out, err := git("add", p); err != nil {
			return fmt.Errorf("git add %s: %v: %s", p, err, out)
		}
	}
	if out, err := git("commit", "-m", msg); err != nil {
		// Nothing to commit is not a race failure — surface other errors.
		if strings.Contains(out, "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit: %v: %s", err, out)
	}
	var lastErr error
	for attempt := 0; attempt < g.maxRetry; attempt++ {
		if err := g.push(); err == nil {
			return nil // pushed
		} else {
			lastErr = err
		}
		// Lost the race — rebase onto the moved main and retry.
		if out, err := git("pull", "--rebase", "origin", "main"); err != nil {
			return fmt.Errorf("git pull --rebase after push race: %v: %s", err, out)
		}
	}
	return fmt.Errorf("push race unresolved after %d attempts: %w", g.maxRetry, lastErr)
}
