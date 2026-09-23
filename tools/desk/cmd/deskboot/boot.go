package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "deskboot"

// Step names. They are CONSTANTS because they are the verb's contract: a caller (a skill,
// a runbook, a loop's boot phase) reads which step failed out of the failure line, and a
// step whose name drifts breaks every consumer that keyed on it.
const (
	stepLoopIdentity    = "loop-identity"
	stepWorktreePrune   = "worktree-prune"
	stepWorktreeLock    = "worktree-lock"
	stepRosterSet       = "roster-set"
	stepRosterPreflight = "roster-preflight"
	stepTokenMint       = "token-mint"
	stepBoardFetch      = "board-fetch"
	stepWorktreeCurrent = "worktree-current"
)

// bootSteps is the ordered step list, used for the plan output and pinned by a test so a
// step cannot be silently reordered or dropped. Order is load-bearing: the envelope
// preflight runs BEFORE the token mint proof only because the preflight's own cold-mint
// check is the stricter instrument, and the board fetch runs after it because a desk with
// no verified envelope has no business reading a queue it might act on. The currency
// check comes LAST because it reads the FETCH_HEAD the board fetch wrote (#1157): a boot
// that stopped at the fetch had proved main was readable, not that this tree was main.
var bootSteps = []string{
	stepLoopIdentity,
	stepWorktreePrune,
	stepWorktreeLock,
	stepRosterSet,
	stepRosterPreflight,
	stepTokenMint,
	stepBoardFetch,
	stepWorktreeCurrent,
}

// loopToTokenRole maps a desk LOOP name (what a session presents in $DESK_LOOP, and what
// a human arms a STOP.<name> flag on) to the APP role the token mint and the envelope
// preflight are keyed on.
//
// The two vocabularies are genuinely different — a loop is a window, a role is an App
// identity — and keeping them separate is why a session names its role ONCE instead
// of spelling both halves at every call site and eventually spelling them apart. The loop
// half is not restated: it is DERIVED from the kill switch's compiled roster
// (deskkit.KnownLoopNames), so a loop that exists cannot be missing from deskboot, and a
// loop deskboot knows cannot be one the kill switch would fail to recognise.
//
// The TABLE itself now lives in deskkit, because the read verbs need the same mapping to
// know which App identity to authenticate their reads as. A second copy here would be a
// second answer to "which App is this window", and the two would drift.
var loopToTokenRole = deskkit.LoopTokenRoles()

// knownRoles returns the loop names deskboot accepts, sorted. It is the INTERSECTION of
// the kill switch's roster and the mapping above: a loop with no App role cannot be
// booted (there is no envelope to check), and a mapping entry the kill switch does not
// know would boot a session whose stop flag never matches — the exact silent failure the
// loop roster exists to close.
func knownRoles() []string {
	var out []string
	for loop := range loopToTokenRole {
		if deskkit.IsKnownLoopName(loop) {
			out = append(out, loop)
		}
	}
	sort.Strings(out)
	return out
}

// tokenRoleFor returns the App role for a loop name, or "" when the loop has none.
func tokenRoleFor(loop string) string { return loopToTokenRole[loop] }

// bootOpts is the parsed invocation.
type bootOpts struct {
	role   string
	root   string
	repo   string
	quiet  bool
	dryRun bool
}

func cmdBoot(args []string) error {
	fs := flag.NewFlagSet("deskboot", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	root := fs.String("root", ".", "repo root this desk operates from (its own worktree)")
	repo := fs.String("repo", "", "owner/name whose INSTALLATION the token mints against (default: derived from the root's origin)")
	quiet := fs.Bool("quiet", false, "suppress the per-step OK lines; failures and the summary still print")
	dryRun := fs.Bool("dry-run", false, "print the plan and stop before any state is touched")

	// Go's flag package stops at the first positional, and the role IS the first
	// positional — so parse the flags that follow it explicitly rather than silently
	// ignoring them.
	if len(args) == 0 {
		return deskkit.Refused("deskboot requires a <role>: one of " + strings.Join(knownRoles(), ", "))
	}
	role := args[0]
	if strings.HasPrefix(role, "-") {
		return deskkit.Refused("deskboot: first argument must be the <role>, not a flag (" + role + ")")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return deskkit.Refused("deskboot: bad flags: " + err.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("deskboot: unexpected extra arguments after <role>: " + strings.Join(fs.Args(), " "))
	}

	o := bootOpts{role: role, root: *root, repo: *repo, quiet: *quiet, dryRun: *dryRun}
	err := boot(o)
	audit(o, err)
	return err
}

// boot runs the sequence. Every step returns an error that ALREADY names its step, so the
// caller never has to reconstruct which one stopped the boot from context.
func boot(o bootOpts) error {
	tokenRole := tokenRoleFor(o.role)
	if tokenRole == "" || !deskkit.IsKnownLoopName(o.role) {
		return deskkit.Refused(fmt.Sprintf(
			"step %s: %q is not a desk role deskboot can boot (want one of: %s). "+
				"A role deskboot does not know is a role whose STOP flag would never match, so it "+
				"refuses rather than booting a session no human could halt.",
			stepLoopIdentity, o.role, strings.Join(knownRoles(), ", ")))
	}

	if o.dryRun {
		printPlan(o, tokenRole)
		return nil
	}

	// 1 — loop identity.
	if err := stepIdentity(o); err != nil {
		return err
	}
	o.say("%s OK: $DESK_LOOP resolves to %s", stepLoopIdentity, o.role)

	// 2 — prune stale worktrees.
	if r := runCmd("", "deskwt", "prune", "--repo", o.root); r.err != nil {
		return stepFailed(stepWorktreePrune, r,
			"re-run `deskwt prune --repo "+o.root+"` by hand and read its refusal; deskwt removes ONLY "+
				"tracked-clean, fully-merged worktrees, so a refusal here is about the prune itself, "+
				"never about active work being in the way")
	}
	o.say("%s OK", stepWorktreePrune)

	// 3 — lock this session's worktree.
	locked, err := stepLock(o)
	if err != nil {
		return err
	}
	o.say("%s OK: %s", stepWorktreeLock, locked)

	// 4 — register the role on the roster.
	if r := runCmd("", "deskroster", "set", "--role", o.role); r.err != nil {
		return stepFailed(stepRosterSet, r,
			"the roster keys ONE beacon per session name, so an unresolvable session identity is the "+
				"usual cause — set $DESK_SESSION (or $CLAUDE_SESSION_ID) to a name unique to this role")
	}
	o.say("%s OK: role=%s", stepRosterSet, o.role)

	// 5 — the operating-envelope preflight. A red preflight is could-not-run for the
	// WHOLE pass: the boot stops and NOTHING is claimed. Deliberately no override flag —
	// a preflight a caller can wave past is not an envelope check.
	//
	// A red envelope is now also ALARMED (the credfence balancing loop): a red preflight otherwise
	// reaches no one but the window it printed to, so deskboot files ONE `to:desk` issue
	// naming each failing check and its state (never the checks' local details), deduped by a
	// marker per role per day. The alarm is a side effect on the way to the SAME refusal — it
	// never changes the verdict, and a filing failure is a stderr warning, never a reason the
	// boot reports something other than red. The refusal text reports what the alarm actually
	// did; it never claims a filing that did not happen.
	if r := runCmd("", "deskroster", "preflight", "--role", tokenRole, "--root", o.root); r.err != nil {
		summary := preflightSummary(r.stderr, r.stdout)
		outcome, aerr := fileRedPreflightAlarm(o, tokenRole, summary)
		if aerr != nil {
			fmt.Fprintf(os.Stderr, "deskboot: WARNING: could not file the red-preflight alarm: %v\n", aerr)
		} else if outcome == alarmFiled {
			o.say("%s: filed a to:desk alarm for the red envelope (role %s)", stepRosterPreflight, tokenRole)
		}
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the operating envelope is NOT green for role %s — %s. "+
				"A red preflight is could-not-run for the whole pass: claim nothing, burn no pass. %s "+
				"Each failing check still names its own remediation. A probe REJECTION is a STOP — never "+
				"retry it under another identity.",
			stepRosterPreflight, tokenRole, summary, alarmSentence(outcome, aerr)), r.err)
	}
	o.say("%s OK: envelope green for role %s", stepRosterPreflight, tokenRole)

	// 6 — token mint proof.
	tokenPath, err := stepMint(o, tokenRole)
	if err != nil {
		return err
	}
	o.say("%s OK: %s token cached at %s", stepTokenMint, tokenRole, tokenPath)

	// 7 — board fetch summary.
	summary, err := stepBoard(o)
	if err != nil {
		return err
	}
	o.say("%s OK: %s", stepBoardFetch, summary)

	// 8 — worktree currency. Seven green steps on a tree three days behind main was the
	// #1157 boot: the fetch proved main READABLE, and nothing proved this tree WAS main.
	current, err := stepCurrent(o)
	if err != nil {
		return err
	}
	o.say("%s OK: %s", stepWorktreeCurrent, current)

	fmt.Printf("deskboot: BOOT COMPLETE — role=%s token-role=%s root=%s (%d/%d steps)\n",
		o.role, tokenRole, o.root, len(bootSteps), len(bootSteps))
	return nil
}

// stepIdentity checks that $DESK_LOOP is set AND names this role's loop class.
//
// It cannot EXPORT the variable — a child process has no reach into its caller's shell —
// and pretending otherwise would be the worst available outcome: a boot that reported the
// identity "set" while the session's own environment still carried nothing, so every
// STOP.<name> flag a human armed would silently fail to match. So the check is real and
// the remediation is the exact line to run.
func stepIdentity(o bootOpts) error {
	raw := strings.TrimSpace(os.Getenv("DESK_LOOP"))
	if raw == "" {
		return deskkit.Refused(fmt.Sprintf(
			"step %s: $DESK_LOOP is unset, so a STOP.%s flag a human is holding would never match this "+
				"session and the stop would silently fail. Run `export DESK_LOOP=%s` in THIS shell, then "+
				"re-run deskboot. (deskboot is a child process; it cannot export into your shell.)",
			stepLoopIdentity, o.role, o.role))
	}
	names, known := deskkit.LoopFlagNames(raw)
	if !known {
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: $DESK_LOOP=%q is not a loop name the kill switch recognises, so whether a stop flag "+
				"is held for this session CANNOT be established — that is could-not-check, never "+
				"'no stop held'. Known loop names: %s.",
			stepLoopIdentity, raw, strings.Join(deskkit.KnownLoopNames(), ", ")), nil)
	}
	// names[0] is the canonical name of whatever was presented; a retired spelling of
	// this role's loop is ACCEPTED here for the same reason the kill switch accepts it —
	// a rename must not orphan a session, in either direction.
	if names[0] != canonicalOf(o.role) {
		return deskkit.Refused(fmt.Sprintf(
			"step %s: $DESK_LOOP=%q resolves to loop %q, but this boot is for role %q. Booting a role under "+
				"another loop's identity points every stop flag at the wrong window. Export "+
				"DESK_LOOP=%s, or boot the role the identity actually names.",
			stepLoopIdentity, raw, names[0], o.role, o.role))
	}
	return nil
}

// canonicalOf resolves a role's own loop name through the same alias table, so comparing
// a presented identity against a requested role compares two CANONICAL names rather than
// two spellings.
func canonicalOf(role string) string {
	names, known := deskkit.LoopFlagNames(role)
	if !known || len(names) == 0 {
		return role
	}
	return names[0]
}

// isolateFix names the exact command(s) that get a desk out of the shared checkout, for
// the shared-checkout refusal. It is spelled so it can be run VERBATIM — the loop name
// deskboot was given (which `deskwt role-init` accepts as-is) and the shared checkout's
// absolute path as --repo-root — because a refusal that names a command the tool then
// refuses is a boot with no clean path out. The worktree path role-init prints on its
// last line is what the re-run's --root takes. When `cellctl` is on PATH the cell launcher
// is named too, since it wraps the same isolate-then-boot sequence for a cell.
func isolateFix(o bootOpts) string {
	root := o.root
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	fix := fmt.Sprintf("Run `deskwt role-init %s --repo-root %s` (its last stdout line is the new "+
		"worktree's absolute path: `cd \"$(deskwt role-init %s --repo-root %s)\"`)",
		o.role, root, o.role, root)
	if _, err := lookPath("cellctl"); err == nil {
		fix += fmt.Sprintf(", or `cellctl desk <cell> %s` (cellctl is on PATH)", o.role)
	}
	return fix
}

// stepLock locks this session's worktree so the prune supervisor cannot reclaim it out
// from under a live desk — the cooperative half of the prune liveness guard.
//
// It REFUSES to boot in the shared checkout. That is the isolate-first rule made
// mechanical rather than restated: a desk operating in the shared checkout writes
// generated files into everyone else's tree, and git will not lock a main worktree in any
// case, so a "lock" step there could only ever be a no-op reported as done.
func stepLock(o bootOpts) (string, error) {
	gitDir := runCmd(o.root, "git", "rev-parse", "--absolute-git-dir")
	if gitDir.err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: %s is not inside a git worktree (%s) — the boot cannot establish where this desk "+
				"is operating, and a root it cannot identify is one it must not lock.",
			stepWorktreeLock, o.root, firstLine(gitDir.stderr)), gitDir.err)
	}
	commonDir := runCmd(o.root, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if commonDir.err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: cannot read the shared git dir for %s (%s) — whether this is a linked worktree or "+
				"the shared checkout is then unknown, and 'unknown' is not permission to proceed.",
			stepWorktreeLock, o.root, firstLine(commonDir.stderr)), commonDir.err)
	}
	// A LINKED worktree has its own git dir under the shared one; the shared checkout has
	// exactly one, so the two paths are equal there. This is the identity test deskwt
	// uses, and identity is right where a prefix test is not: a path check can be spoofed
	// by a symlink, a git-dir cannot.
	if gitDir.stdout == commonDir.stdout {
		return "", deskkit.Refused(fmt.Sprintf(
			"step %s: %s is the SHARED checkout, not a session worktree. A desk boots into its own "+
				"worktree — the shared tree is where generated files from a session's writes strew across "+
				"every other session's work, and git refuses to lock a main worktree in any case. "+
				"%s and re-run deskboot with --root <that worktree>.",
			stepWorktreeLock, o.root, isolateFix(o)))
	}

	r := runCmd(o.root, "git", "worktree", "lock", "--reason", o.role+" live session", o.root)
	if r.err != nil {
		// An ALREADY-locked worktree is the idempotent success case, not a failure: the
		// desired end state (this tree is locked) already holds. Anything else is a real
		// failure, because an unlocked live worktree is reclaimable underneath the desk.
		if strings.Contains(strings.ToLower(r.stderr), "already locked") {
			return o.root + " (already locked — idempotent)", nil
		}
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: could not lock %s (%s) — an unlocked live worktree is reclaimable by the prune "+
				"supervisor mid-session, so this is not a warning to boot past.",
			stepWorktreeLock, o.root, firstLine(r.stderr)), r.err)
	}
	return o.root, nil
}

// stepMint proves the role's App token mints from THIS shell, at boot, before an
// auto-approval classifier tightens mid-session and wedges a desk trying to change
// identity late.
//
// It records the token's PATH and size, and NEVER its value: a token echoed into a
// transcript is a token in every log that transcript reaches.
func stepMint(o bootOpts, tokenRole string) (string, error) {
	repo, err := o.resolveRepo()
	if err != nil {
		return "", err
	}
	if !deskkit.IsAllowedRepo(repo) {
		return "", deskkit.Refused(fmt.Sprintf(
			"step %s: %s is not in the desk repo set, so no installation is rostered to mint against it. "+
				"Boot from a repo the desk operates on, or pass --repo <owner/name> naming one.",
			stepTokenMint, repo))
	}
	// Which forge serves this repo decides the mint's custody path. It is RESOLVED from
	// the SAME single seam the preflight's cold-mint check reads (#659: deskkit.ForgeKindForRepo
	// wraps the one resolveForgeKind — ASSAY_REPO_FORGES, then the origin remote host), never
	// a caller's choice, so the mint and the preflight that precedes it can never disagree
	// about the forge. Unresolved reads as GitHub, the historical default, so a GitHub adopter
	// with no forge config mints byte-for-byte as before (#671's preflight fix made the
	// GitLab envelope green; this closes the leftover boot half). A GitLab-resolved repo takes
	// desktoken's GitLab PAT custody path via `--forge gitlab` — desktoken then rotates the
	// role's `gitlab-<role>.token` rather than looking for a GitHub App PEM / apps.env a
	// GitLab deployment does not have, which was the boot's stopping point (#676).
	mintArgs := []string{tokenRole, "--repo", repo}
	if deskkit.ForgeKindForRepo(repo) == deskkit.ForgeGitLab {
		mintArgs = append(mintArgs, "--forge", "gitlab")
	}
	// --repo is NOT optional. A role's App is installed on more than one account, and a
	// mint with no repo defaults to whichever owner the tool considers first — producing
	// a token with no access to the repo this desk is about to work in, which surfaces
	// later as an unrelated-looking API error.
	r := runCmd("", "desktoken", mintArgs...)
	if r.err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: `desktoken %s` did not mint (%s). Say so IN the artifact you were about "+
				"to post — never silently fall back to another identity.",
			stepTokenMint, strings.Join(mintArgs, " "), firstLine(r.stderr)), r.err)
	}
	path := firstLine(r.stdout)
	fi, serr := os.Stat(path)
	if serr != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: desktoken reported the cache path %q but it cannot be read — a mint whose product "+
				"cannot be found is not a proven mint.", stepTokenMint, path), serr)
	}
	if fi.Size() == 0 {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the token cache at %s is EMPTY. An empty token file is the shape that authenticates "+
				"as nobody while every call still 'succeeds' locally — refusing rather than booting on it.",
			stepTokenMint, path), nil)
	}
	return path, nil
}

// stepBoard fetches origin/main and summarises the board AT THE FETCHED HEAD.
//
// READ-ONLY BY CONSTRUCTION, and that is the whole point of doing it here. A board regen
// run from a session's home is a WRITE: it rewrites the generated board and register views
// in whatever tree it was run from, so a boot that "just refreshed the board" leaves
// uncommitted diffs over generated files in a shared checkout, which then present as
// register corruption. Reading the committed file at FETCH_HEAD answers the same question
// and writes nothing.
func stepBoard(o bootOpts) (string, error) {
	if r := runCmd(o.root, "git", "fetch", "--no-tags", "origin", "main"); r.err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: `git fetch --no-tags origin main` failed in %s (%s) — a board read from a stale tree "+
				"is a stale board, and a desk that cannot see current main must not start draining.",
			stepBoardFetch, o.root, firstLine(r.stderr)), r.err)
	}
	r := runCmd(o.root, "git", "show", "FETCH_HEAD:STATUS.md")
	if r.err != nil {
		// A repo with no board file publishes no queue. That is a FACT about the repo,
		// stated plainly, not a failed boot — but it is never left silent, because a
		// missing board and an empty board are different situations.
		return "no board file at FETCH_HEAD:STATUS.md — this repo publishes no queue", nil
	}
	rows, section := summariseBoard(r.stdout)
	if section == "" {
		return fmt.Sprintf("board read at FETCH_HEAD (%d lines); no Next-up section", countLines(r.stdout)), nil
	}
	return fmt.Sprintf("board read at FETCH_HEAD (%d lines); %d row(s) under %q", countLines(r.stdout), rows, section), nil
}

// stepCurrent proves this worktree IS the main the board fetch just read (#1157): HEAD
// must contain FETCH_HEAD, and the worktree's `.assay-versions` must equal FETCH_HEAD's.
//
// Both halves are needed. The commit-graph check catches the ordinary stale tree (a role
// worktree that carried a local commit, could never fast-forward again, and sat days
// behind main). The pin check is what the desk verbs actually key their drift banner on —
// a tree whose graph contains main but whose pin was changed by a local commit or an
// uncommitted edit would boot green and then read STALE:drift on its first board read,
// which is the same blind hour this step exists to remove. A refusal here is exit 6 (a
// step ran and could not prove the tree current) and names the ONE-LINE self-heal: a
// merge of origin/main, spelled with the fully-qualified ref so a stray local branch named
// `origin/main` can never be what gets merged. Merge, never rebase: force-push is denied,
// and a rebase rewrites lineage a reviewer has already seen.
//
// READ-ONLY, like the fetch before it: the step reports and refuses; it never merges on
// the session's behalf. The cell launcher (`cellctl desk`) is the layer that merges at
// boot, and this step is what proves the launcher — or a hand boot — actually left the
// tree current rather than printing a notice and moving on.
func stepCurrent(o bootOpts) (string, error) {
	root := o.root
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	head := runCmd(o.root, "git", "rev-parse", "FETCH_HEAD")
	if head.err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: FETCH_HEAD does not resolve in %s after the board fetch (%s) — whether this tree "+
				"is current cannot be established, and 'unknown' is not 'current'.",
			stepWorktreeCurrent, o.root, firstLine(head.stderr)), head.err)
	}
	sha := firstLine(head.stdout)
	short := sha
	if len(short) > 8 {
		short = short[:8]
	}
	heal := "git -C " + root + " merge refs/remotes/origin/main"

	// The pins on both sides, read before the graph check so a behind-main refusal can
	// ALSO say which desk-tools pin the operator is about to be stuck on.
	fetched := runCmd(o.root, "git", "show", "FETCH_HEAD:"+deskkit.AssayVersionsFile)
	fetchedPin := ""
	if fetched.err == nil {
		fetchedPin = strings.TrimSpace(fetched.stdout)
	}
	localRaw, _ := os.ReadFile(filepath.Join(o.root, deskkit.AssayVersionsFile))
	localPin := strings.TrimSpace(string(localRaw))
	pinNote := ""
	if fetchedPin != localPin {
		pinNote = fmt.Sprintf(" This worktree's %s pins [%s]; origin/main's pins [%s].",
			deskkit.AssayVersionsFile, pinSummary(localPin), pinSummary(fetchedPin))
	}

	if r := runCmd(o.root, "git", "merge-base", "--is-ancestor", "FETCH_HEAD", "HEAD"); r.err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: %s is BEHIND origin/main — HEAD does not contain FETCH_HEAD %s, the main the board "+
				"fetch just read.%s A desk booted here reads a current board and acts from a stale tree: its "+
				"first board read says STALE:drift, its loop refuses every flip, and nothing in the boot said "+
				"so. Self-heal (one line, merge — never rebase): `%s`, then re-run deskboot.",
			stepWorktreeCurrent, o.root, short, pinNote, heal), r.err)
	}
	if fetchedPin != localPin {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: %s carries a different %s from origin/main although HEAD already contains FETCH_HEAD "+
				"%s — a local commit or an uncommitted edit in this worktree changed the pin.%s A desk boots "+
				"only on main's pin (every drift check keys on it). Restore main's: `git -C %s checkout "+
				"FETCH_HEAD -- %s` if the edit is uncommitted, else re-cut the role worktree from origin/main "+
				"(`deskwt role-init %s --repo-root <shared checkout>`), then re-run deskboot.",
			stepWorktreeCurrent, o.root, deskkit.AssayVersionsFile, short, pinNote, root,
			deskkit.AssayVersionsFile, o.role), nil)
	}
	if fetchedPin == "" {
		return "HEAD contains FETCH_HEAD " + short + "; no " + deskkit.AssayVersionsFile + " on either side", nil
	}
	return "HEAD contains FETCH_HEAD " + short + "; " + deskkit.AssayVersionsFile + " equals origin/main's (" +
		pinSummary(fetchedPin) + ")", nil
}

// pinSummary renders a `.assay-versions` body as its `<artifact> <tag>` pairs, one per
// line, comments and digests dropped — enough to see WHICH pin differs without quoting a
// 64-hex digest per line into a boot log. An empty body renders as "none".
func pinSummary(body string) string {
	var pairs []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) >= 2 {
			pairs = append(pairs, f[0]+" "+f[1])
		} else {
			pairs = append(pairs, f[0])
		}
	}
	if len(pairs) == 0 {
		return "none"
	}
	return strings.Join(pairs, ", ")
}

// summariseBoard counts the table rows under the board's Next-up section without
// interpreting them. Counting is all a boot needs: the queue itself is read by the loop's
// own SelectQueue, and a boot that PARSED the queue would be a second, drifting reader of
// a format it does not own.
func summariseBoard(board string) (rows int, section string) {
	inSection := false
	for _, line := range strings.Split(board, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if inSection {
				break // the next heading ends the section
			}
			// statusgen emits the heading as `## Next up` (a SPACE); older boards and some
			// fixtures spell it `## Next-up` (a hyphen). Accept BOTH so the boot summary
			// stays locked to whatever spelling statusgen renders.
			if lower := strings.ToLower(trimmed); strings.Contains(lower, "next up") || strings.Contains(lower, "next-up") {
				inSection = true
				section = strings.TrimLeft(trimmed, "# ")
			}
			continue
		}
		if !inSection || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		// Skip the header row and the |---|---| separator.
		if strings.Contains(trimmed, "---") {
			rows = 0 // everything before the separator was the header
			continue
		}
		rows++
	}
	return rows, section
}

func countLines(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// resolveRepo returns the owner/name the token mints against: --repo when given, else the
// root's origin remote. A remote it cannot parse is UNVERIFIABLE, never a guess — minting
// against a guessed installation is how a desk ends up holding a token for the wrong
// account and discovering it three calls later.
func (o bootOpts) resolveRepo() (string, error) {
	if strings.TrimSpace(o.repo) != "" {
		return strings.TrimSpace(o.repo), nil
	}
	r := runCmd(o.root, "git", "remote", "get-url", "origin")
	if r.err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: cannot read origin's URL in %s (%s) — pass --repo <owner/name>.",
			stepTokenMint, o.root, firstLine(r.stderr)), r.err)
	}
	slug := repoSlugFromURL(r.stdout)
	if slug == "" {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: origin %q does not parse to an owner/name — pass --repo <owner/name> rather than "+
				"letting the mint default to an installation nobody chose.",
			stepTokenMint, r.stdout), nil)
	}
	return slug, nil
}

// repoSlugFromURL reduces an origin URL to owner/name for the SSH and HTTPS spellings.
// Anything else returns "" so the caller refuses instead of minting against a guess.
func repoSlugFromURL(url string) string {
	s := strings.TrimSpace(url)
	s = strings.TrimSuffix(s, ".git")
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
		if at := strings.Index(s, "@"); at >= 0 {
			s = s[at+1:]
		}
	} else if i := strings.Index(s, ":"); i >= 0 && strings.Contains(s[:i], "@") {
		s = s[i+1:]
		parts := strings.Split(s, "/")
		if len(parts) < 2 {
			return ""
		}
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}

// stepFailed renders a sub-verb's non-zero exit as an Unverifiable naming the step, the
// tool's own first line, and the remediation. Unverifiable (6), not Refused (5): a step
// that ran and came back non-zero has left the boot unable to PROVE the envelope, which
// is what exit 6 means. Exit 5 stays reserved for the preconditions the caller controls,
// so that a human never learns to route around a 5.
func stepFailed(step string, r runResult, remedy string) error {
	return deskkit.Unverifiable(fmt.Sprintf("step %s: %s — %s", step, firstLine(r.stderr), remedy), r.err)
}

// say prints a per-step OK line unless --quiet. Failures never go through here: they are
// errors, and an error is printed whatever the noise floor.
func (o bootOpts) say(format string, args ...any) {
	if o.quiet {
		return
	}
	fmt.Printf("deskboot: "+format+"\n", args...)
}

func printPlan(o bootOpts, tokenRole string) {
	fmt.Printf("deskboot: PLAN (dry run — nothing touched) role=%s token-role=%s root=%s\n",
		o.role, tokenRole, o.root)
	for i, s := range bootSteps {
		fmt.Printf("  %d %s\n", i+1, s)
	}
}

// audit writes exactly one audit line for the boot. A failure to write it is surfaced on
// stderr, never swallowed.
func audit(o bootOpts, err error) {
	result := deskkit.ResultOK
	detail := "boot complete role=" + o.role
	if o.dryRun {
		detail = "dry-run role=" + o.role
	}
	if err != nil {
		switch deskkit.ExitCodeOf(err) {
		case deskkit.ExitRefused:
			result = deskkit.ResultRefused
		default:
			result = deskkit.ResultUnverifiable
		}
		detail = firstLine(err.Error())
	}
	if lerr := deskkit.Log(deskkit.Entry{
		Tool:   toolName,
		Verb:   "boot",
		Result: result,
		Detail: detail,
		Title:  o.role,
	}); lerr != nil {
		fmt.Fprintf(os.Stderr, "deskboot: WARNING: could not write audit line: %v\n", lerr)
	}
}
