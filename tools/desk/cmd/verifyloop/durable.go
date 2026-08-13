package main

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

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
// loop (commit -> pull --rebase -> push) in code. `run` is the injectable command
// runner (defaults to exec) so the retry loop is unit-testable without a live remote — and so
// the reference build's tests never actually push.
type gitDurable struct {
	root     string
	run      func(args ...string) (string, error)
	maxRetry int
}

func newGitDurable(root string) *gitDurable {
	return &gitDurable{
		root:     root,
		maxRetry: 5,
		run: func(args ...string) (string, error) {
			cmd := exec.Command(args[0], args[1:]...)
			b, err := cmd.CombinedOutput()
			return string(b), err
		},
	}
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
		if _, err := git("push", "origin", "HEAD:main"); err == nil {
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
