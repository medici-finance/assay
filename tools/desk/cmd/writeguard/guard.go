package main

import (
	"fmt"
	"path/filepath"

	"github.com/medici-finance/assay/deskcore/writepolicy"
)

// Denial text belongs to the hook protocol. All decisions come from writepolicy.
func (c Config) sentinelBlock(tool, target string) Verdict {
	sentinel := sentinelPathForMessage()
	return Verdict{
		Block: true,
		Reason: fmt.Sprintf(`writeguard: BLOCKED — the shared-OK claim is HUMAN-ONLY.
This %s call would write the writeguard claim sentinel (%s):
  %s
No session can issue its own exemption. A claim that any tool call could set is not a gate: dispatched workers are shared-homed by construction (#1035) and are exactly the population the isolation backstop exists to constrain, so the claim is settable only OUT OF BAND — by a human, in their own terminal, where no hook runs:
  mkdir -p %s
  printf '%%s\n' <shared-checkout-path> > %s   # scoped to one checkout
  # or, to claim any checkout on this machine: touch %s
The claim expires after %s; renew or drop it the same way (this guard refuses both from a session).
Dispatched/worker session? The claim is not yours to make — isolate instead: git worktree add ../tracker-<name> -b <branch> origin/main (absolute sibling path) and work there.`,
			tool, sentinel, target,
			filepath.Dir(sentinel), sentinel, sentinel, sentinelTTL()),
	}
}

// blockEvasionGuidance is appended to every block message (#1193): it names
// the ONE sanctioned exit for an out-of-checkout false positive (re-issue the
// SAME command with targets the guard can resolve) and forbids substituting a
// different command. The 2026-08-16 incident's worker answered a
// false-positive `rm` block with `find … -delete` instead of re-issuing —
// this line turns that confusion into a correct self-service re-issue.
const blockEvasionGuidance = "If your write targets a path OUTSIDE the shared checkout, re-issue the same command with absolute target paths (or one `cd <abs-dir> && …` chain) — the guard admits those. Do NOT substitute a different command to achieve the write: a block is a stop signal, and substitution is an escalation-worthy policy violation."

func (c Config) renderBlock(detail writepolicy.Detail) Verdict {
	tool, target := detail.Tool, detail.Target
	if detail.SharedHomed {
		// Shared-homed session WITHOUT the opt-in token (#1035): with the
		// token it would have been exempt and never reached here. The
		// generic message below would name the shared checkout as "your own
		// worktree" — wrong guidance for this population.
		return Verdict{
			Block: true,
			Reason: fmt.Sprintf(`writeguard: BLOCKED — isolation backstop (shared-homed exemption is OPT-IN, #1035).
This session is homed in the SHARED checkout (%s) and this %s call targets it:
  %s
The shared-homed exemption must be CLAIMED explicitly (cf. ASSAY_MAIN_COMMIT_OK), and only a HUMAN can claim it — no session, including this one, can issue its own (#1190 review). Both mechanisms live outside the tool surface:
  - a human shell: export WRITEGUARD_SHARED_OK=1 (once per shell); or
  - a human terminal, outside Claude Code (the env var cannot be set from the Bash tool — fresh shell per call, #1190):
      mkdir -p %s
      printf '%%s\n' %s > %s   # scoped to this checkout; an empty file claims any
    The sentinel expires after %s; renewing and dropping it are human acts too — this guard refuses tool-surface writes to that path.
Dispatched/worker session? The claim is not yours to make — isolate instead: create your own worktree (git worktree add ../tracker-<name> -b <branch> origin/main, absolute sibling path) and work there.`,
				c.SharedRoot, tool, target,
				filepath.Dir(sentinelPathForMessage()), c.SharedRoot, sentinelPathForMessage(), sentinelTTL()) +
				"\n" + blockEvasionGuidance,
		}
	}
	home := detail.Home
	return Verdict{
		Block: true,
		Reason: fmt.Sprintf(`writeguard: BLOCKED — isolation backstop.
This session is homed in %s, but this %s call targets the SHARED checkout (%s):
  %s
Write ONLY inside your own worktree (%s). Never write to the shared checkout via absolute paths, and never cd into it — read it with git -C / absolute-path reads if you must. If this operation is genuinely meant for the shared checkout, it must run from a session homed there (the coordinator or a direct human session) with WRITEGUARD_SHARED_OK=1 exported.`,
			home, tool, c.SharedRoot, target, home) +
			"\n" + blockEvasionGuidance,
	}
}
