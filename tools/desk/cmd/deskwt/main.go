// Command deskwt is the worktree-lifecycle desk tool.
// It encodes TWO local-only verbs — "add a worktree under a sanctioned prefix" and
// "remove a worktree I can prove is safe to delete" — so the mandated isolate-first rule
// is the path of least resistance instead of
// a prompt, and so cleanup can never wipe another session's uncommitted work the way a raw
// rm-class delete can.
//
// Safety is by construction:
//   - both verbs act only under the RESOLVED (EvalSymlinks) prefixes `/private/tmp/tracker-*`
//     and `<repo-root>/.claude/worktrees/`; a path that resolves elsewhere is refused;
//   - the shared checkout is refused by IDENTITY (git-common-dir's parent), not prefix;
//   - remove refuses a dirty TRACKED tree, unpushed commits, or a branch with no upstream,
//     and there is NO --force / override verb anywhere;
//   - add never clobbers an existing target and refuses an unresolvable --base.
//
// These are local-only verbs: they take the full audit line and the kill switch
// but NOT the outward-write rate limit (deskkit/ratelimit.go "Verb classes").
//
// Exit codes (deskkit contract): 0 success/noop, 3 disabled, 4 rate-limited
// (unused here), 5 refused, 6 unverifiable. See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskwt — add, remove, or prune git worktrees, only under sanctioned prefixes.

USAGE:
  deskwt add <name> [--branch B | --detach] [--base origin/main] [--role R]
  deskwt remove <path>
  deskwt prune [--repo <path>] [--interval <dur>] [--reclaim-stale-locks]
               [--reap-dead-sessions] [--lock-ttl <dur>] [--dry-run]
  deskwt role-init  <role> [--repo-root <checkout>] [--session <s>] [--no-fetch]
  deskwt role-clean <role> [--repo-root <checkout>] [--session <s>]
  deskwt --version

role-init provisions a DESK ROLE's own worktree in one idempotent call: a session-scoped
path under /private/tmp/tracker-*, a uniquely-named branch cut from a FRESHLY FETCHED
origin/main (so the preflight landing probe is green and the session does not start behind
main), a worktree lock, and the role's App commit identity set PER-WORKTREE (bot USER id,
#638) so concurrent sessions cannot race each other's identity via shared config, plus the
role's App CREDENTIAL HELPER set per-worktree (chain reset, one inline helper reading the
role's 0600 token file — never a token in argv or a URL), and finally the role's own
PREFLIGHT run against the provisioned worktree (red = exit 6, the path is still printed). An
existing valid worktree is reused (helper re-wired, so a polluted chain is scrubbed); a
foreign-repo path is refused, never re-pointed. The
LAST line on stdout is the worktree's absolute path — the launcher contract:
` + "`cd \"$(deskwt role-init <role> --repo-root <checkout>)\"`" + `.
role-clean unlocks and removes it under the same safety guards as remove.

<role> is EVERY desk role desktoken mints (desk, worker, reviewer, verifier, issue-loop,
intake-loop), spelled either as that token role or as the loop name deskboot boots
(the-desk, worker-desk, pr-review-desk, verify-desk, intake-desk); --role <role> is the
same thing spelled as a flag. --repo-root names the checkout to provision FROM (default:
the working directory). The shared checkout's index and user.* config are never touched:
the identity lands in the NEW worktree's own config, and the one shared-config write is
enabling extensions.worktreeConfig (once, idempotent) so that scoping takes effect.
--no-fetch cuts from the local origin/main as-is.

add STAMPS or CLEARS the new worktree's commit identity so it never INHERITS the shared
checkout's. With --role R (a token role or a loop name, folded the same way role-init folds
it) the role's App commit identity — the bot USER id noreply address (#638), resolved through
the SAME shared resolver role-init uses — is written to the new worktree's own config
(extensions.worktreeConfig), and an unbound role is REFUSED (exit 5) before the worktree is
created. WITHOUT --role the new worktree's user.name/user.email are CLEARED (set empty at
worktree scope, which shadows the shared value — an --unset would fall through to it), so a
commit there fails closed ("Author identity unknown") until an identity is set, rather than
committing under an unrelated inherited identity. The shared checkout's config is never
touched either way. The identity (or the cleared state) is echoed to stderr; stdout stays the
bare worktree path.

add --role GIVES the new worktree the role App's OWN TRANSPORT instead of the one it would
inherit (an SSH origin, an operator's pushurl sentinel). At worktree scope it writes
remote.origin.pushurl and remote.origin.url as an empty entry (git's list reset, git 2.46+)
followed by https://<host>:443/<owner>/<name>.git — the explicit port keeps a global
https-to-SSH insteadOf from rewriting it — plus the role App's host-scoped credential helper
(the same one role-init writes). An SSH host alias is resolved to its real host with
` + "`ssh -G`" + ` (no connection). It then reads back what git itself resolves for fetch and push, and
REFUSES (exit 5), rolling the worktree back, unless both are exactly that one URL. An origin on
git's local transport (a path) carries no key and is left as it is, unless it pushes over SSH.

Without --role, add REFUSES an SSH PUSH REMOTE under a bot identity. A worktree inherits this
checkout's remote, so an ssh:// or git@host:path PUSH url here is one in every worktree cut
from it — and a session whose $DESK_LOOP resolves to a role App would push under whatever key
this machine's agent holds, a human's, while its commits read as the App's. The refusal names
the url and the one-line remedy. Fetch over SSH stays allowed (remote.origin.pushurl is what is
read whenever it is set), and with $DESK_LOOP unset the gate is inert — a human pushes under
their own key, which is what the SSH remote is for.

add resolves a LOCAL BRANCH COLLISION by name rather than dying on git's. Worktrees share
one refs store, so a branch left behind by an abandoned dispatch blocks every later add that
derives the same name. A leftover that is checked out in no worktree and carries no commit its
upstream (or --base) lacks is RECLAIMED — deleted and recreated — with an audit line. One that
is checked out somewhere, or carries unpushed commits, is REFUSED, naming the worktree path or
the commit count.

deskwt is safe by construction: every verb acts ONLY on paths that RESOLVE under
/private/tmp/tracker-* or <repo-root>/.claude/worktrees/, the shared checkout is refused by
identity, and remove/prune refuse a dirty tracked tree or unpushed commits. There is NO
--force flag. On any state it cannot positively verify it refuses.

prune first runs ` + "`git worktree prune`" + ` (drops entries for dirs already gone), then removes
ONLY worktrees it can prove safe: tracked-clean AND fully merged into origin/main. An
UNMERGED branch (an open PR in flight) is LEFT untouched — that is the active-worker guard.
With --interval (e.g. 30m) it loops forever, sweeping every interval (for a k8s desk pod's
prune loop); it honors the kill switch / STOP flags between ticks and exits 0 on SIGTERM.
Every sweep reports: pruned (bookkeeping), removed, held (and locked-held), locks-reclaimed,
dead-session-reaped, and branches-deleted.

A LOCKED worktree is always held — and nothing else ever unlocks one, so a lock taken by a
session that has since died is permanent and the locked population only grows.
--reclaim-stale-locks (default OFF) gives the lock a lifecycle: it UNLOCKS the locks it can
prove stale — the ` + "`session=<id>`" + ` in the lock reason has no live roster beacon, or (with
--lock-ttl 24h) the lock is older than the TTL — and then the ORDINARY rules decide, unchanged.
It never removes anything itself: a reclaimed worktree that is dirty, unpushed or unmerged is
still LEFT. Every unlock prints the worktree, the lock reason, and why it was judged stale.

That leaves the worktrees of sessions that died with work in flight: an unmerged branch is
"active work" to the ordinary rules, so a dead session's open-PR worktree is held forever and
every later resume of that branch fails at worktree-create (git allows one worktree per
branch). --reap-dead-sessions (default OFF) judges those by a different question — does a
LIVE session still own this tree, and is deleting it provably lossless? A tree is reaped only
when no live session owns it (its lock names a session the roster shows is gone, or it
carries no lock at all) AND it is clean with UNTRACKED FILES COUNTED AND HEAD is already
reachable from its upstream or from refs/remotes/origin/main. Its stale local branch is
deleted with it (non-force ` + "`git branch -d`" + `) so the next add cuts fresh from origin. A live
session's lock holds its worktree unconditionally, and anything dirty, unpushed or
unverifiable is LISTED with the reason and left. Pair it with --dry-run first: that prints
the full plan — path, session, REAP/KEEP, reason — and changes nothing.

Exit: 0 ok/noop · 3 disabled · 5 refused · 6 unverifiable.

DIAGNOSTICS: DESK_TRACE=1 (or a global --trace, any position) prints the full cause
chain, every child process with its command line, exit status and elapsed time, and the
failing child's stderr in full. Credentials are redacted. With it off, output is
unchanged. See tools/desk/README.md, "Diagnostics — DESK_TRACE".`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (a correctness review found: SetToolClass had no caller anywhere,
	// so "ClassWrite is the safe default" was true only by luck). This tool ACTS on
	// the roster, so it is ciEligible=false: it reads the config-home file and never
	// the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// P3: echo the effective roster once per run. Every tool that reads a configured
	// control surface echoes it — a value that lives in settings rather than in a diff
	// is only visible at RUN time, and a NARROWING must be as visible as a widening.
	deskkit.EchoEffectiveConfig(os.Stderr)
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// The global diagnostic switch, taken BEFORE any subcommand dispatch so `--trace` works
	// on every deskwt verb and is invisible to each subcommand's own FlagSet. The env form
	// (DESK_TRACE=1) needs no help and is the portable one: it survives being invoked by
	// deskdispatch, by a loop supervisor, or by a dispatched agent's wrapper.
	args = deskkit.TakeTraceFlag(args)

	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskwt sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// TIER ONE of the help retrofit (deskkit/helprequest.go). A SUBCOMMAND help request —
	// `deskwt <sub> --help` — is a request for a help screen, not an invocation of the verb, and
	// until this it was recorded as `refused: bad flags: flag: help requested`: exit 5 plus one
	// row appended to a ledger the write budget counts and nothing rotates (1,043 such rows
	// measured on one operating desk host over 32 days). It returns HERE, before Guard, and
	// writes nothing. HelpOnly matches only the unambiguous single-token shape, so a `--help`
	// that is another flag's VALUE cannot be mistaken for one; every wider spelling falls
	// through to the subcommand's own parse, where flag.ErrHelp is recognised instead.
	if deskkit.HelpOnly(args) {
		fmt.Fprintln(os.Stderr, usage)
		return deskkit.ExitOK
	}

	// kill-switch check is the FIRST action of the tool. Guard writes its own
	// result=disabled audit line.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(os.Stderr)

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "add":
		err = cmdAdd(rest)
	case "remove":
		err = cmdRemove(rest)
	case "prune":
		err = cmdPrune(rest)
	case "role-init":
		err = cmdRoleInit(rest)
	case "role-clean":
		err = cmdRoleClean(rest)
	default:
		fmt.Fprintf(os.Stderr, "deskwt: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	// TIER TWO terminus (deskkit/helprequest.go): print the help screen the operator asked
	// for and exit 0. The sentinel's own message is never what they wanted to read.
	if deskkit.IsHelpRequest(err) {
		fmt.Fprintln(os.Stderr, usage)
		return deskkit.ExitOK
	}
	// The shared exit path. With DESK_TRACE off this is byte-identical to the
	// fmt.Fprintln(os.Stderr, err.Error()) it replaces.
	deskkit.ReportError(os.Stderr, err)
	return deskkit.ExitCodeOf(err)
}
