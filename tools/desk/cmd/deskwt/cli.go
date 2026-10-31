package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitmaintenance"
	"github.com/spf13/cobra"
)

// wtCLI keeps parsing and configuration apart from every domain authority. Tests replace
// the typed hand-off, never the path guards, locks or audit rows in the verb handlers.
type wtCLI struct {
	performance func(perfrec.Context)
	inputs      cli.Inputs
	before      func() error
	handle      func(wtCall) error
}

// wtCall is one parsed invocation: the verb and that verb's typed options. Nothing in it
// has been validated beyond its type; every domain rule stays in the handler.
type wtCall struct {
	verb     string
	add      addOpts
	remove   removeOpts
	prune    pruneFlags
	maintain maintainOpts
	objects  objectsOpts
	role     roleFlags
}

type addOpts struct {
	name, branch, base, upstream, role, lockSession, adoptRun string
	detach, dryRun                                            bool
}

type removeOpts struct {
	path   string
	dryRun bool
}

// pruneFlags carries the duration knobs as the operator's text: runPrune owns their
// parsing, their bounds and every combination refusal, exactly as before the port.
type pruneFlags struct {
	repo, interval, lockTTL, singletonTTL, cellsRoot, minIdle string
	reclaim, reapDead, dryRun, noSingleton                    bool
	noBranches, branchesOnly, noMaintenance, noCache          bool
}

type maintainOpts struct {
	repo              string
	apply             bool
	interval, timeout time.Duration
}

type objectsOpts struct {
	repo                string
	apply, dryRun, json bool
	interval, timeout   time.Duration
}

type roleFlags struct {
	role, session, repoRoot string
	noFetch                 bool
	positionals             []string
}

func str(key, def, usage string) cli.Binding {
	return cli.Binding{Key: key, Flag: key, Kind: cli.String, Default: def, Usage: usage}
}

func boolean(key, usage string) cli.Binding {
	return cli.Binding{Key: key, Flag: key, Kind: cli.Bool, Default: false, Usage: usage}
}

func duration(key string, def time.Duration, usage string) cli.Binding {
	return cli.Binding{Key: key, Flag: key, Kind: cli.Duration, Default: def, Usage: usage}
}

// wtBindings is every setting each command declares, by command path. It is the whole
// configuration surface: no setting is read from the environment or a config file, and a
// flag not listed here is an unknown flag.
func wtBindings() map[string][]cli.Binding {
	return map[string][]cli.Binding{
		"": {{Key: "trace", Flag: "trace", Kind: cli.Bool, Default: false, Persistent: true,
			Usage: "print the full cause chain and every child process (any position; DESK_TRACE=1 is the same switch)"}},
		"add": {
			str("branch", "", "new branch name for the worktree (default: the worktree name)"),
			boolean("detach", "check the base out as a DETACHED HEAD — no branch created or touched (the verifier shape)"),
			str("base", "origin/main", "start-point ref for the new worktree"),
			str("upstream", "", "with a full commit --base: the fully qualified remote branch to track; its tip must match the commit"),
			str("role", "", "stamp this desk role's App commit identity (token role or loop name) into the new "+
				"worktree's own config; omitted, the new worktree's user.name/user.email are CLEARED so it can never inherit "+
				"the shared checkout's identity"),
			boolean("dry-run", "print the lifecycle-hook plan (HOOK after_create: ...) and touch nothing"),
			str("lock-session", "", "lock the new worktree for this live session id, in role-init's lock-reason format"),
			str("adopt-run", "", "with --upstream: adopt an existing home stamped with this run key when it is proven finished "+
				"(registered, unlocked, clean incl. untracked, on --branch at the pinned --base) instead of refusing it"),
		},
		"remove": {boolean("dry-run", "print the lifecycle-hook plan (HOOK before_remove: ...) and touch nothing")},
		"prune": {
			str("repo", "", "run against this repo root instead of the current directory"),
			str("interval", "", "if set (e.g. 30m), loop: sweep every interval instead of once"),
			boolean("reclaim-stale-locks", "unlock worktree locks PROVEN stale (locking session gone, or older than --lock-ttl) so the normal prune rules can apply; default off"),
			str("lock-ttl", "0", "with --reclaim-stale-locks or --reap-dead-sessions: also treat any lock older than this (e.g. 24h) as stale; 0 disables the age test"),
			boolean("reap-dead-sessions", "remove a worktree no live session owns (dead-session lock, or no lock at all) when it is clean AND fully pushed, deleting its stale local branch too; default off"),
			boolean("dry-run", "report what a sweep would remove and the before_remove hook plan; delete nothing (one-shot only)"),
			str("singleton-ttl", defaultSingletonTTL.String(), "skip the sweep when one completed less than this ago (e.g. 10m); 0 disables the recency check (the singleton lock is always taken)"),
			boolean("no-singleton", "disable the singleton's recency check (equivalent to --singleton-ttl 0); the lock is still taken, so a live sweep still holds this one"),
			boolean("no-branches", "skip the local-branch GC (run the worktree arms only)"),
			boolean("branches-only", "run ONLY the local-branch GC (skip every worktree arm)"),
			boolean("no-maintenance", "skip the daily offline object-store maintenance pass"),
			boolean("no-cache", "skip the cell-cache pressure pass"),
			str("cells-root", "", "cells root the cache pressure pass sweeps for marked cell Go-cache roots (default: $CELLS_ROOT, then the per-user data-directory default)"),
			str("min-idle", "", "hold any worktree git shows activity in more recently than this (e.g. 2h; default "+defaultMinIdle.String()+"); must be positive"),
		},
		"maintain": {
			str("repo", "", "repository or linked worktree"),
			boolean("apply", "apply safe index tasks; otherwise read-only plan"),
			duration("min-interval", 6*time.Hour, "minimum time between attempts (at least 1h)"),
			duration("timeout", 30*time.Second, "total work budget (1s to 5m)"),
		},
		"maintain objects": {
			str("repo", "", "repository worktree to inspect (default current directory)"),
			boolean("apply", "perform bounded offline maintenance when due"),
			boolean("dry-run", "read-only inventory and task plan (the default)"),
			boolean("json", "emit the full report as JSON on stdout"),
			duration("min-interval", gitmaintenance.DefaultInterval, "minimum time between attempts for the common Git directory"),
			duration("timeout", gitmaintenance.DefaultTimeout, "total inventory and task time budget"),
		},
		"role-init":  roleBindings(),
		"role-clean": roleBindings(),
	}
}

func roleBindings() []cli.Binding {
	return []cli.Binding{
		str("role", "", "desk role to provision a worktree for (token role or loop name; may also be given positionally)"),
		str("session", "", "session id (default $DESK_SESSION, then $CLAUDE_SESSION_ID)"),
		str("repo-root", "", "checkout of the repo to provision from (default: the working directory)"),
		boolean("no-fetch", "start from the local origin/main as-is instead of fetching it fresh first"),
	}
}

const maintainLong = `maintain prints a read-only JSON plan of the safe index tasks (pack-refs, commit-graph) for
the repository's common Git directory. --apply runs them at most once per --min-interval
(at least 1h) within --timeout (1s to 5m), holding the common directory's maintenance lock
and then the temporary-reference custody lock (assay-maintenance-temprefs.lock) that
standalone ref cleanup shares; a held lock reports "held" and changes nothing. It deletes no
object and expires no reflog. ` + "`maintain objects`" + ` is the offline object-store pass.`

// newWtCLI builds the one command tree. argv is the raw command line, read only by the
// nested-verb shape check in maintain objects.
func newWtCLI(env wtCLI, argv []string) *cobra.Command {
	b := wtBindings()
	root := cli.NewRoot("deskwt", "safe worktree lifecycle and coordinated offline Git maintenance")
	root.Long = rootLong
	rootSet := cli.Declare(root, b[""]...)
	root.Args = cobra.NoArgs
	// --version is the root's own flag, not Cobra's built-in one: Cobra answers its version
	// flag before argument validation, so `deskwt --version extra` printed the version and
	// exited 0. Here the root's NoArgs refuses any word beside it (exit 2, nothing on stdout),
	// and the root's RunE prints it before anything is read, guarded or audited.
	root.Flags().Bool("version", false, "print the version and exit")
	root.RunE = func(cmd *cobra.Command, _ []string) error {
		if v, _ := cmd.Flags().GetBool("version"); v {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), cliVersion())
			return err
		}
		// No verb is a refusal, as it always was: the help screen on stderr, exit 5, and
		// nothing read, guarded or audited.
		cmd.SetOut(cmd.ErrOrStderr())
		_ = cmd.Help()
		return deskkit.Refused("deskwt: a verb is required")
	}

	verb := func(use, short, long string, args cobra.PositionalArgs, call func(*cli.Values, []string) wtCall) *cobra.Command {
		path := use
		if i := indexSpace(use); i >= 0 {
			path = use[:i]
		}
		c := &cobra.Command{Use: use, Short: short, Long: long, Args: args}
		set := cli.Declare(c, b[path]...)
		c.RunE = func(cmd *cobra.Command, args []string) error {
			if path == "objects" && !nestedObjects(root, argv) {
				return &deskkit.DeskError{Code: cli.ExitUsage, Msg: "deskwt: `maintain objects` takes its flags after objects, never between the two words\n" +
					"Run 'deskwt --help' for usage."}
			}
			rv, err := rootSet.Resolve(env.inputs)
			if err != nil {
				return deskkit.Refused(err.Error())
			}
			v, err := set.Resolve(env.inputs)
			if err != nil {
				return deskkit.Refused(err.Error())
			}
			name := path
			if path == "objects" {
				name = "maintain"
			}
			deskkit.PerfCommand(perfVerbs, name)
			if rv.Source("trace") == cli.Flag {
				deskkit.SetTrace(rv.Bool("trace"))
			}
			if env.before != nil {
				if err := env.before(); err != nil {
					return err
				}
			}
			w := call(v, args)
			w.verb = path
			return env.handle(w)
		}
		return c
	}

	root.AddCommand(verb("add <name>", "create a sanctioned worktree on a new tracking branch (or --detach)", addLong, cobra.ExactArgs(1),
		func(v *cli.Values, a []string) wtCall {
			return wtCall{add: addOpts{name: a[0], branch: v.String("branch"), detach: v.Bool("detach"), base: v.String("base"),
				upstream: v.String("upstream"), role: v.String("role"), dryRun: v.Bool("dry-run"),
				lockSession: v.String("lock-session"), adoptRun: v.String("adopt-run")}}
		}))
	root.AddCommand(verb("remove <path>", "remove a proven-safe worktree (there is no --force)", removeLong, cobra.ExactArgs(1),
		func(v *cli.Values, a []string) wtCall {
			return wtCall{remove: removeOpts{path: a[0], dryRun: v.Bool("dry-run")}}
		}))
	root.AddCommand(verb("prune", "sweep stale worktrees, locks, branches and caches under the safety gates", pruneLong, cobra.NoArgs,
		func(v *cli.Values, _ []string) wtCall {
			return wtCall{prune: pruneFlags{repo: v.String("repo"), interval: v.String("interval"), reclaim: v.Bool("reclaim-stale-locks"),
				lockTTL: v.String("lock-ttl"), reapDead: v.Bool("reap-dead-sessions"), dryRun: v.Bool("dry-run"),
				singletonTTL: v.String("singleton-ttl"), noSingleton: v.Bool("no-singleton"), noBranches: v.Bool("no-branches"),
				branchesOnly: v.Bool("branches-only"), noMaintenance: v.Bool("no-maintenance"), noCache: v.Bool("no-cache"),
				cellsRoot: v.String("cells-root"), minIdle: v.String("min-idle")}}
		}))
	maintain := verb("maintain", "plan or apply the safe index tasks for the common Git directory", maintainLong, cobra.NoArgs,
		func(v *cli.Values, _ []string) wtCall {
			return wtCall{maintain: maintainOpts{repo: v.String("repo"), apply: v.Bool("apply"),
				interval: v.Duration("min-interval"), timeout: v.Duration("timeout")}}
		})
	// The nested command's flags are its own; b["objects"] is its binding list.
	b["objects"] = b["maintain objects"]
	maintain.AddCommand(verb("objects", "inventory or apply bounded offline object-store maintenance", objectsLong, cobra.NoArgs,
		func(v *cli.Values, _ []string) wtCall {
			return wtCall{objects: objectsOpts{repo: v.String("repo"), apply: v.Bool("apply"), dryRun: v.Bool("dry-run"),
				json: v.Bool("json"), interval: v.Duration("min-interval"), timeout: v.Duration("timeout")}}
		}))
	root.AddCommand(maintain)
	role := func(v *cli.Values, a []string) wtCall {
		return wtCall{role: roleFlags{role: v.String("role"), session: v.String("session"), repoRoot: v.String("repo-root"),
			noFetch: v.Bool("no-fetch"), positionals: a}}
	}
	root.AddCommand(verb("role-init [<role>]", "provision (or reuse) a desk role's own locked, wired worktree; prints its path last", roleLong, cobra.MaximumNArgs(1), role))
	root.AddCommand(verb("role-clean [<role>]", "unlock and remove a desk role's worktree under remove's guards", roleLong, cobra.MaximumNArgs(1), role))
	return root
}

func indexSpace(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			return i
		}
	}
	return -1
}

// nestedObjects reports whether the command line names `maintain objects` as two adjacent
// words. Before the port, `maintain --apply objects` was a refusal (objects was a stray
// positional of maintain); the tree would otherwise route it to the object pass with --apply.
// The root's persistent flags (the global --trace, which the pre-port parser stripped
// wherever it stood, and the performance-context selectors) are not maintain's flags, so
// they are skipped, with the value a non-boolean one takes as its next word.
func nestedObjects(root *cobra.Command, argv []string) bool {
	for i, a := range argv {
		if a != "maintain" {
			continue
		}
		for j := i + 1; j < len(argv); j++ {
			w := argv[j]
			if w == "objects" {
				return true
			}
			name := strings.TrimPrefix(strings.TrimPrefix(w, "-"), "-")
			if name == w || name == "" || strings.HasPrefix(name, "-") {
				return false
			}
			name, _, inline := strings.Cut(name, "=")
			f := root.PersistentFlags().Lookup(name)
			if f == nil {
				return false
			}
			if !inline && f.NoOptDefVal == "" {
				j++
			}
		}
		return false
	}
	return false
}

// execute runs one parsed verb's handler.
func execute(c wtCall) error {
	switch c.verb {
	case "add":
		return runAdd(c.add)
	case "remove":
		return runRemove(c.remove)
	case "prune":
		return runPrune(c.prune)
	case "maintain":
		return runMaintain(c.maintain)
	case "objects":
		return runObjectMaintenance(c.objects)
	case "role-init":
		return runRoleInit(c.role)
	case "role-clean":
		return runRoleClean(c.role)
	}
	return deskkit.Unverifiable("deskwt: no handler for verb "+c.verb, nil)
}

func cliVersion() string {
	sha, built := deskkit.Version()
	return fmt.Sprintf("deskwt sourceSHA=%s builtAt=%s releaseTag=%s", sha, built, deskkit.ReleaseTagOrDev())
}

func parseWT(args []string, env wtCLI, streams cli.IO) int {
	return cli.Run(func() *cobra.Command { return newWtCLI(env, args) }, args, cli.Options{
		Performance: env.performance, IO: streams, GoFlagCompat: true,
		ExitCode: func(err error) int { deskkit.ReportError(streams.Err, err); return deskkit.ExitCodeOf(err) },
	})
}

// guardedExecute is the execution path after parsing: the kill switch first (Guard writes its
// own disabled row), the unpinned-build warning, then the verb.
func guardedExecute(c wtCall) error {
	if err := deskkit.Guard(); err != nil {
		return err
	}
	deskkit.WarnIfUnpinned(os.Stderr)
	return execute(c)
}

// Help prose, one verb per command. It is the text the retired single usage screen carried,
// split so each `deskwt <verb> --help` prints its own verb's contract.
const (
	addLong = `add --base <full-commit> --upstream <full-remote-ref> checks that the remote ref
resolves to that commit before allocating. It checks out the immutable commit with
--no-track, then sets branch tracking separately; a failed tracking setup rolls back
the new worktree. --upstream requires a fully qualified refs/remotes/ ref and is
incompatible with --detach. Without --upstream, the existing track-the-base behavior
is unchanged. Worker resume dispatches supply both values.

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

add --role and role-init GIVE the worktree the role App's OWN TRANSPORT instead of the one it would
inherit (an SSH origin, an operator's pushurl sentinel). At worktree scope it writes
remote.origin.pushurl and remote.origin.url as an empty entry (git's list reset, git 2.46+)
followed by https://<host>:443/<owner>/<name>.git — the explicit port keeps a global
https-to-SSH insteadOf from rewriting it — plus the role App's host-scoped credential helper
(the same one role-init writes). An SSH host alias is resolved to its real host with
` + "`ssh -G`" + ` (no connection). It then reads back what git itself resolves for fetch and push, and
REFUSES (exit 5) unless both are exactly that one URL. add rolls back its new worktree;
role-init retains its worktree for repair and stops before preflight. On any failure after
the URL writes (that refusal, or a credential that cannot be resolved) the worktree-scoped URL
lists are restored, and the inherited credential helper chain, cleared before the first URL
write, stays cleared. An origin on
git's local transport (a path) carries no key and is left as it is, unless it pushes over SSH.
An explicit HTTP service port, an HTTPS port other than 443, or an empty HTTP(S)
port is refused before URL or credential provisioning. Omitted HTTP ports retain
the established HTTPS migration.

Without --role, add REFUSES an SSH PUSH REMOTE under a bot identity. A worktree inherits this
checkout's remote, so an ssh:// or scp-style (user, at sign, host, colon, path) PUSH url
here is one in every worktree cut from it — and a session whose $DESK_LOOP resolves to a
role App would push under whatever key this machine's agent holds, a human's, while its
commits read as the App's. The refusal names
the url and the remedy. Fetch over SSH stays allowed (remote.origin.pushurl is what is
read whenever it is set), and with $DESK_LOOP unset the gate is inert — a human pushes under
their own key, which is what the SSH remote is for.

add --lock-session <id> LOCKS the new worktree for that live session, in role-init's lock
reason format (` + "`session=<id>`" + `), so prune holds it from its first second and
--reclaim-stale-locks / --reap-dead-sessions retire it only once that session's roster beacon
is stale. A lock that cannot be taken and verified rolls the worktree back.

add --adopt-run <run key> (resume only: needs --upstream) hands back a FINISHED home instead
of refusing it — the existing target, or the worktree holding --branch — when it carries that
worktree-scoped assay.runKey, is registered and unlocked, sits on --branch at exactly the
pinned --base, and has an empty status including untracked files. Nothing is deleted or reset;
a home failing any clause keeps the original refusal, which names the clause.

add resolves a LOCAL BRANCH COLLISION by name rather than dying on git's. Worktrees share
one refs store, so a branch left behind by an abandoned dispatch blocks every later add that
derives the same name. A leftover that is checked out in no worktree and carries no commit its
upstream (or --base) lacks is RECLAIMED — deleted and recreated — with an audit line. One that
is checked out somewhere, or carries unpushed commits, is REFUSED, naming the worktree path or
the commit count.`

	removeLong = `remove releases a LOCKED worktree only when the lock is the caller's own session's
($DESK_SESSION, then $CLAUDE_SESSION_ID) and the tree holds no untracked or uncommitted
files: that lock is retired just before the delete and restored if the delete fails. The
session id is self-asserted and inherited, so the own-lock arm also refuses a tree whose
"git status --porcelain" lists anything (untracked files, a modified submodule, a nested
repository), naming the rows (exit 5, nothing deleted). What git ignores is NOT listed and is
deleted with the tree: ignored files, a directory that ignores itself, and any worktree
nested under an ignored path (its own lock is not judged, and its registration is left
behind). An inherited $DESK_SESSION passes the session match; the loss is then limited to
what that status cannot see. Any other lock on the target — another session's, whatever
its roster beacon says (a beacon is written only when its session updates the roster, so an
old or missing one is not proof the session is gone), one naming no session, git's
"initializing" lock — is refused with exit 5 and the tree is left untouched; a clean, pushed
tree of a dead session is reaped by ` + "`deskwt prune --reap-dead-sessions`" + ` instead, once the tree
has been idle past --min-idle (1h by default; run it with --dry-run first, it sweeps every
ownerless tree). This is the end-of-PR step for a session that made its own home:
` + "`deskwt add <name> --lock-session <id>`" + ` at the start, ` + "`deskwt remove <path>`" + ` once the PR is
pushed and the tree is clean.

remove also clears a registration whose worktree has NO git link left — an empty-directory
husk, or a directory already gone — which the gates above cannot open (they exited 6 and left
the branch registered forever). It does so only when the directory holds nothing, no process
holds it open (lsof — without root it sees only the caller's own processes; unavailable = could-not-check, refuse), the admin HEAD is recoverable
(a branch that resolves, or a detached commit on a remote-tracking ref), and any lock is the
caller's own session's or its session is proven gone by the roster. A husk with files, a lock
naming no session, or git's "initializing" lock is refused. Then it unlocks, runs
` + "`git worktree prune`" + `, verifies the registration is gone, and removes the empty directory; a
failed clear restores the original lock. before_remove runs only when the directory exists (cwd = the husk); a
directory already gone has nothing to clean, so the hook is skipped.`

	objectsLong = `maintain objects prints a read-only inventory and task plan by default. --apply runs bounded,
offline ref packing, commit-graph and incremental object packing, once per common Git
directory per 24h attempt interval. Failed attempts consume the same interval. Every object ID is retained, including unreachable objects borrowed
by other clones. Configured reference-transaction hooks or uncertain native hook inventory
hold maintenance. It never fetches, runs gc, expires reflogs or changes branches/worktrees.
prune also attempts this daily maintenance after its lifecycle sweep; --no-maintenance
skips it, and --branches-only retains its branch-only meaning. Child process groups
or Windows jobs enforce the runtime budget before releasing repository ownership.`

	roleLong = `role-init provisions a DESK ROLE's own worktree in one idempotent call: a session-scoped
path under the sanctioned temporary-root prefix, a uniquely-named branch cut from a FRESHLY FETCHED
origin/main (so the preflight landing probe is green and the session does not start behind
main), a worktree lock, and the role's App commit identity set PER-WORKTREE (bot USER id,
#638) so concurrent sessions cannot race each other's identity via shared config, plus the
role's App fetch/push TRANSPORT and CREDENTIAL HELPER set per-worktree (URL list and helper
chain reset, one inline helper reading the
role's 0600 token file — never a token in argv or a URL), and finally the role's own
PREFLIGHT run against the provisioned worktree (red = exit 6, the path is still printed). An
existing valid worktree is reused (transport and helper re-wired, existing files preserved); a
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
--no-fetch cuts from the local origin/main as-is.`

	pruneLong = `prune first runs ` + "`git worktree prune`" + ` (drops entries for dirs already gone), then handles
stale registrations whose directory survives but whose .git gitdir file is gone (the shape
` + "`git worktree remove`" + ` refuses and a lock makes ` + "`git worktree prune`" + ` keep forever). It
clears the REGISTRATION only — the directory is left on disk — and only behind three
gates: the scope gates every arm enforces (never the shared checkout, the current
worktree, or outside the sanctioned prefixes), an admin-HEAD target retained by a branch that positively resolves to a commit,
or by a detached commit already on refs/remotes/origin/main,
and the lock rule — the default sweep never retires a lock, so a locked husk clears only
when --reclaim-stale-locks has proven its lock stale this sweep. Then it removes
ONLY worktrees it can prove safe: tracked-clean AND fully merged into origin/main. An
UNMERGED branch (an open PR in flight) is LEFT untouched — that is the active-worker guard.
A worktree whose HEAD IS the origin/main tip (zero landed commits — the fresh-worktree case,
and the shape a reclaim-by-detach leaves behind) must additionally pass three at-tip gates:
old enough to be no session's just-created workspace (the fresh-worktree safety window),
nothing uncommitted or untracked (the untracked-inclusive status), and no git-ignored
payload. An at-tip tree passing all three drains; any other is LEFT with the reason named.
A worktree holding a verifier admission record (an admitted verifier run's home, whose
Evidence still lands from it) is LEFT by both this sweep and --reap-dead-sessions for 24h
after admission; ` + "`deskwt remove`" + ` is the explicit release. An unreadable record holds that
worktree; a record that names no worktree holds every candidate in the sweep.
Ahead of every content gate, in this arm and the --reap-dead-sessions arm alike, the
ACTIVITY gate holds any worktree git shows activity in within --min-idle (default 1h;
tunable, never disableable): the newest mtime of its admin HEAD, index, logs/HEAD,
ORIG_HEAD and FETCH_HEAD and of the worktree root itself. It is held as "active <age> ago";
evidence that cannot be read holds it too. A tree a session is working in can be
content-identical to residue — this gate needs no lock to see the session.
With --interval (e.g. 30m) it loops forever, sweeping every interval (for a k8s desk pod's
prune loop); it honors the kill switch / STOP flags between ticks and exits 0 on SIGTERM.
Every sweep reports: pruned (bookkeeping), removed, held (and locked-held), locks-reclaimed,
dead-session-reaped, and branches-deleted — plus, when it acted, stale-registrations-cleared.

Every sweep ALSO runs the CELL-CACHE pressure pass (Step D, default ON; --no-cache skips it):
for each marked cell Go-cache root under the cells root (--cells-root, then $CELLS_ROOT,
then the compiled per-user default) it runs the managed cache's own pressure check —
reclaiming over the byte budget or under the free-space floor, and trimming over the
high-water mark (75% of budget). Active consumer records protect mod/, gopath/ and whole
roots; a trim removes build/ entries oldest first down to the low-water mark (60% of budget
by default), plus any past the policy's trim age, and never one younger than the trim
safety floor (2h by default) — and it reaps a cache root whose owning directory this same sweep removed,
gated on the root's lease set re-read under its own lock at act time. The pass is SILENT
when every cache is within policy; the summary gains a cache segment only when it
reclaimed, reaped, or found pressure it could not clear.

Every sweep ALSO garbage-collects LOCAL branches (Step C — the worktree lifecycle leaves a
stale local ref behind per worktree, and nothing else reaps them). A branch is collected
ONLY when its content is provably on refs/remotes/origin/main: its tip is an ancestor of
the mainline (a fresh, never-used handle is exactly this), or every unique commit's
patch-id is already in the mainline's history (the squash-merge case). Never collected:
main/master, any branch checked out in any worktree (guarded by the worktree-list holder
set, re-read immediately before each delete — git's ref-update layer has NO checked-out
refusal to lean on), and anything whose proof fails or cannot be completed — a branch with
unique patches is KEPT, always. The delete is a compare-and-delete against the proven sha
(git update-ref -d --no-deref), never a force verb. --no-branches skips this pass;
--branches-only runs it alone, so the boot path can control cadence. Live sweeps print the
per-class COUNTS only; --dry-run prints the per-branch COLLECT plan and deletes nothing.

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
the full plan — path, session, REAP/KEEP, reason — and changes nothing.`

	rootLong = `deskwt — safe worktree lifecycle and coordinated offline Git maintenance.

Worktree creation/removal acts only on paths that RESOLVE under the sanctioned
temporary-root prefix or <repo-root>/.claude/worktrees/. The shared checkout is refused
by identity, and remove/prune refuse a dirty tracked tree or unpushed commits. There is NO
--force flag. On any state it cannot positively verify it refuses.

Request each verb's own help: deskwt <verb> --help (or deskwt help <verb>).

Exit: 0 ok/noop · 2 usage (unknown verb or flag, bad value, wrong positional count) · 3 disabled · 5 refused · 6 unverifiable.

DIAGNOSTICS: DESK_TRACE=1 (or a global --trace, any position) prints the full cause
chain, every child process with its command line, exit status and elapsed time, and the
failing child's stderr in full. Credentials are redacted. With it off, output is
unchanged. See tools/desk/README.md, "Diagnostics — DESK_TRACE".`
)
