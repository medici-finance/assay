package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const smokeUsage = "cellctl smoke <cell> [--harness <claude|codex>] [--model <m>]"

// cmdSmoke is a scrubbed-cell-only, one-shot, tool-free, READ-ONLY readiness probe — the same
// composed environment `desk` uses, a prompt that asks nothing of any tool, and a pass/fail on
// the harness's own literal answer.
//
// Never a Verify row on a live harness (every row runs a stub); this is what an operator runs by
// hand to prove a cell actually answers before trusting it with real work.
func cmdSmoke(cell string, args []string) {
	c := loadCell(cell)
	if c.Kind != "scrubbed" {
		die("smoke is only defined for a scrubbed cell (kind=%s, got '%s')", c.Kind, cell)
	}
	harness := c.Harness
	modelOverride := ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--harness":
			harness = needFlagValue(args, &i, "--harness needs a value (claude|codex)")
		case "--model":
			modelOverride = needFlagValue(args, &i, "--model needs a value")
		default:
			if strings.HasPrefix(a, "--") {
				die("smoke: unknown flag %s", a)
			}
			die("smoke: unexpected argument '%s'", a)
		}
	}
	if !valueIn(harness, harnessValues) {
		die("smoke: --harness must be claude or codex, got '%s'", harness)
	}
	model := modelOverride
	if model == "" {
		rm := c.resolveRoleModel("smoke", harness)
		if !rm.OK {
			die("smoke: %s — pass --model explicitly", rm.Src)
		}
		model = rm.Model
	}
	const prompt = "Reply with the single word READY and nothing else."
	session := c.Name + "-smoke-" + utcStamp()
	if harness == "codex" {
		session += "-codex"
	}
	env := c.scrubbedComposeEnv("smoke", harness, session)
	var argv []string
	if harness == "codex" {
		argv = []string{"codex", "exec", "--ephemeral", "--sandbox", "read-only", "--skip-git-repo-check", "-m", model, prompt}
	} else {
		argv = []string{"claude", "-p", "--model", model, prompt}
	}
	if c.Env.Get("DRY_RUN") == "1" {
		fmt.Printf("[dry-run] cell=%s kind=scrubbed smoke harness=%s model=%s\n", c.Name, harness, model)
		c.printPlan(env, c.Repo, argv)
		return
	}
	fmt.Printf("[smoke] resolved model=%s harness=%s (read-only, tool-free, one shot)\n", model, harness)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = c.Repo
	cmd.Env = env.Pairs
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	rc := 0
	if err := cmd.Run(); err != nil {
		rc = exitStatus(err)
	}
	last := lastNonBlankLine(buf.String())
	if rc == 0 && last == "READY" {
		fmt.Println("READY")
		return
	}
	fmt.Printf("smoke: not ready: %s\n", last)
	exitWith(1)
}

// lastNonBlankLine is the oracle's `awk 'NF{l=$0} END{print l}'` — the last line carrying a
// non-whitespace field, or "" when there is none.
func lastNonBlankLine(s string) string {
	last := ""
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			last = l
		}
	}
	return last
}

var _ = os.Stdout
