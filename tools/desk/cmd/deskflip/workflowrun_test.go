package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestReadPR_UnreadableWorkflowRun_CanStillFlip is the assay-toolkit#2024 flip half: the
// single-PR state read the pr-open-draft condition depends on must not need `actions:read`.
// gh's built-in `pr view --json statusCheckRollup` selects `checkSuite.workflowRun`; under a
// `checks:read`-only identity (the reviewer App) that sub-field 403s FORBIDDEN and `gh pr
// view` fails the whole read, so readPR wrapped it Unverifiable and the desk could not flip
// ANY private PR ("a PR whose state could not be read is not a PR that may be flipped").
//
// This models that identity: a fake gh that refuses any read still requesting the built-in
// statusCheckRollup via `pr view`, or the workflowRun/checkSuite sub-field at all, and
// answers only a read that omits it. Before the fix readPR issued `gh pr view --json
// …statusCheckRollup`, so the fake fails it and readPR errors — RED. After the fix readPR
// issues a hand-authored `gh api graphql` that never asks for workflowRun, the fake answers,
// the PR state parses and the checks-green gate evaluates green — GREEN.
func TestReadPR_UnreadableWorkflowRun_CanStillFlip(t *testing.T) {
	// runCmd refuses gh with no App token (the write-identity backstop); the flip flow sets
	// it from the app-token condition. Here we read directly, so set it and restore.
	oldTok := ghToken
	ghToken = "stub-installation-token"
	t.Cleanup(func() { ghToken = oldTok })

	var gotArgs []string
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		joined := name + " " + strings.Join(args, " ")
		gotArgs = append([]string{name}, args...)

		// A read that requests the Actions run link at all is refused, the way the
		// checks:read-only App is refused it — via the workflowRun/checkSuite sub-field,
		// or via gh's built-in statusCheckRollup field on `pr view` (which pulls it in).
		if strings.Contains(joined, "workflowRun") || strings.Contains(joined, "checkSuite") ||
			(strings.Contains(joined, "pr view") && strings.Contains(joined, "statusCheckRollup")) {
			return exec.Command("/bin/sh", "-c",
				"echo 'GraphQL: Resource not accessible by integration (checkSuite.workflowRun)' 1>&2; exit 1")
		}
		// A read that omits the Actions link is served: the SAME flat prInfo shape gh api
		// graphql's --jq emits in production. checks:read covers every conclusion.
		return echo(mustJSON(t, greenPR()))
	}
	t.Cleanup(func() { execCommand = old })

	pr, err := readPR(flipOpts{pr: 7}, "medici-finance/assay")
	if err != nil {
		t.Fatalf("readPR must not fail when checkSuite.workflowRun is unreadable; the desk cannot "+
			"flip any private PR otherwise. got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	if pr.Number != 7 || pr.State != "OPEN" || !pr.IsDraft {
		t.Fatalf("PR state must parse from the read; got %+v", pr)
	}
	// The checks-green gate the flip runs on this rollup must still evaluate — and be green,
	// since CheckRun.conclusion/StatusContext.state are covered by checks:read.
	if got := evalRollup(latestPerRollupName(pr.StatusCheckRollup)); got != ciGreen {
		t.Errorf("rollup must classify green from checks:read alone; got ciState %d", got)
	}
	// Belt-and-braces: the read we actually issued must never request the actions:read field.
	joined := strings.Join(gotArgs, " ")
	if strings.Contains(joined, "workflowRun") || strings.Contains(joined, "checkSuite") {
		t.Errorf("the PR-state read must not request checkSuite.workflowRun (needs actions:read); "+
			"args were: %s", joined)
	}
}
