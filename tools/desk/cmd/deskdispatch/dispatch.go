package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "deskdispatch"

// Step names — the verb's contract. A caller reads which step stopped the dispatch out of
// the failure line, so these strings are as load-bearing as the exit codes.
const (
	stepClaimAcquire   = "claim-acquire"
	stepWorktreeCreate = "worktree-create"
	stepRosterRegister = "roster-register"
	stepDecisionGate   = "decision-gate"
	stepModelStamp     = "model-stamp"
	stepPromptEmit     = "prompt-emit"
)

// dispatchSteps is the ordered step list, pinned by a test so a step cannot be silently
// dropped or reordered. The order is the safety property: the CLAIM is first, before any
// worktree exists and before any prompt is emitted, because everything after it is work
// that a second dispatcher must not also be doing.
var dispatchSteps = []string{
	stepClaimAcquire,
	stepWorktreeCreate,
	stepRosterRegister,
	stepDecisionGate,
	stepModelStamp,
	stepPromptEmit,
}

// claimScriptRel / decisionScriptRel are the consumer scripts this verb wraps. They are
// named by path and invoked; their content is never carried here. See main.go's
// "WRAP, NEVER RE-IMPLEMENT".
//
// WHERE THEY RESOLVE. Both are resolved under --claim-root when it is given, else under
// --root. The scripts were centralized out of the consumer repos when tools/desk was
// relocated, so the checkout that carries the TOOLS is no longer always the checkout the
// item belongs to. The claim itself does not move with the script: it is a ref in the
// TARGET repo (--repo), created via the forge API by the script from wherever the script
// sits — so decoupling the script's location from the worktree source changes which FILE
// runs and nothing about where the claim durably lands.
const (
	claimScriptRel    = "tools/dispatch-claim.sh"
	decisionScriptRel = "tools/decision-issue.sh"
)

// itemKeyRe bounds what may be passed to a shell script as a claim key. The key goes into
// an argv slice, never a shell string, so this is defence in depth rather than the only
// guard — but a key carrying a path segment or a leading dash is a key that would be read
// as a flag, or would escape the ref namespace it is supposed to name.
var itemKeyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`)

// worktreeNameRe bounds the worktree name derived from the item key.
var worktreeNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// branchNameRe MIRRORS the worktree verb's own --branch constraint. That verb remains the
// authority; this copy exists only so a branch name it would reject is refused BEFORE the
// durable claim is taken rather than after. It is deliberately no LOOSER than the original
// — a pre-check that accepted more than the real one would hand the rejection back to the
// expensive path it exists to protect.
var branchNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

type dispatchOpts struct {
	item       string
	tier       string
	kit        string
	repo       string
	root       string
	claimRoot  string
	model      string
	branch     string
	brief      string
	gateHuman  bool
	pr         int
	promptFile string
	quiet      bool
	dryRun     bool
}

func cmdDispatch(args []string) error {
	fs := flag.NewFlagSet("deskdispatch", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	tier := fs.String("tier", "any", "execution tier the item demands: strong|any")
	kit := fs.String("kit", "worker", "prompt kit for the dispatched agent class: "+joinKits())
	repo := fs.String("repo", "", "owner/name the item belongs to (default: derived from --root's origin)")
	root := fs.String("root", ".", "the ITEM's own repo root — the checkout the worktree is cut from")
	claimRoot := fs.String("claim-root", "", "the checkout carrying the consumer claim/decision scripts, when the "+
		"item's own repo does not (default: --root). The claim still lands in --repo's own ref namespace — this "+
		"names only where the TOOL lives, never where the worktree is cut from")
	model := fs.String("model", "", "lowercase slug of the model being launched, for the dispatcher's attestation stamp")
	branch := fs.String("branch", "", "branch name for the agent's worktree (default: derived from the item key)")
	brief := fs.String("brief", "", "path to the item's specification file, for the decision-issue gate and the prompt")
	gateHuman := fs.Bool("gate-human", false, "the item is human-gated: ensure its decision issue exists before dispatch")
	pr := fs.Int("pr", 0, "an ALREADY-OPEN PR for this item; enables the roster work entry and the label stamp")
	promptFile := fs.String("prompt-file", "", "write the assembled prompt here instead of stdout")
	quiet := fs.Bool("quiet", false, "suppress the per-step OK lines")
	dryRun := fs.Bool("dry-run", false, "print the plan and the prompt; touch nothing")

	if len(args) == 0 {
		return deskkit.Refused("deskdispatch requires an <item-key>")
	}
	item := args[0]
	if strings.HasPrefix(item, "-") {
		return deskkit.Refused("deskdispatch: first argument must be the <item-key>, not a flag (" + item + ")")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return deskkit.Refused("deskdispatch: bad flags: " + err.Error())
	}
	if fs.NArg() != 0 {
		return deskkit.Refused("deskdispatch: unexpected extra arguments after <item-key>: " + strings.Join(fs.Args(), " "))
	}

	o := dispatchOpts{
		item: item, tier: *tier, kit: *kit, repo: *repo, root: *root, claimRoot: *claimRoot,
		model: *model, branch: *branch, brief: *brief, gateHuman: *gateHuman, pr: *pr,
		promptFile: *promptFile, quiet: *quiet, dryRun: *dryRun,
	}
	err := dispatch(o)
	audit(o, err)
	return err
}

func dispatch(o dispatchOpts) error {
	// EVERY caller-controlled precondition is checked HERE, before the claim. See
	// validateCallerPreconditions for why that placement is a correctness property and not
	// a tidiness preference.
	plan, err := validateCallerPreconditions(o)
	if err != nil {
		return err
	}
	repo, branch, wtName := plan.repo, plan.branch, plan.wtName

	if o.dryRun {
		fmt.Printf("deskdispatch: PLAN (dry run — nothing touched) item=%s repo=%s tier=%s kit=%s branch=%s\n",
			o.item, repo, o.tier, o.kit, branch)
		for i, s := range dispatchSteps {
			fmt.Printf("  %d %s\n", i+1, s)
		}
		prompt, perr := assemblePrompt(o, plan, "")
		if perr != nil {
			return perr
		}
		return emitPrompt(o, prompt)
	}

	// Advisory write-scope overlap echo, BEFORE the claim — a coordination hint
	// the operator sees, then the dispatch PROCEEDS. It never blocks, never gates the claim,
	// and carries no exit code: a foreseeable merge collision on a shared file is surfaced now
	// rather than at merge, and proceeding over it is correct when the overlap is intended.
	echoWriteOverlap(os.Stderr, o)

	// 1 — the durable claim, FIRST. Everything after this is work a second dispatcher
	// must not also be doing.
	if err := stepClaim(o, repo, plan.claimScript, plan.claimKey); err != nil {
		return err
	}
	o.say("%s OK: %s claimed in %s (claim key %s)", stepClaimAcquire, o.item, repo, plan.claimKey)

	// 2 — the agent's worktree, in the ITEM's repo. deskwt owns the safety here (a
	// sanctioned path prefix, an unambiguous base, no clobber of an existing target), so
	// this step delegates rather than re-deriving any of it — INCLUDING where the
	// worktree lands: the path the prompt names is the one deskwt printed, never one this
	// verb predicted.
	wt := runCmd(o.root, "deskwt", "add", wtName, "--branch", branch, "--base", "refs/remotes/origin/main")
	if wt.err != nil {
		// deskwt's OWN message, whole and verbatim — it is the one that names the cause
		// (which branch, which worktree holds it, what to do). Reducing it to the first
		// stderr line reduced it to the config echo, and an operator who cannot see the
		// cause re-runs the claim machinery instead of clearing the stray ref.
		msg := fmt.Sprintf(
			"step %s: `deskwt add %s` failed in %s. The claim is HELD — release it, or fix the tree "+
				"and re-run; do not launch an agent with no worktree of its own. deskwt said:\n%s",
			stepWorktreeCreate, wtName, o.root, toolMessage(wt.stderr))
		// deskwt's exit code passes THROUGH: a refusal (5) is a decision it made — the branch
		// is held by a live worktree, or carries unpushed work — and flattening a decision
		// into "could not be established" tells the operator to retry something that will
		// never succeed on its own.
		if exitCodeOf(wt.err) == deskkit.ExitRefused {
			return deskkit.Refused(msg)
		}
		return deskkit.Unverifiable(msg, wt.err)
	}
	home := firstLine(wt.stdout)
	if home == "" || home == "(no output)" || !strings.HasPrefix(home, "/") {
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: `deskwt add %s` exited 0 but named no absolute worktree path (%q). The agent's home "+
				"is the isolation floor every other clause rests on, so a home this verb cannot state is a "+
				"dispatch it must not make.", stepWorktreeCreate, wtName, wt.stdout), nil)
	}
	o.say("%s OK: %s on %s", stepWorktreeCreate, home, branch)

	prompt, perr := assemblePrompt(o, plan, home)
	if perr != nil {
		return perr
	}

	// 3 — roster registration.
	o.say("%s %s", stepRosterRegister, stepRoster(o, repo))

	// 4 — the human-decision gate. The effective gate (flag OR brief metadata) was decided
	// pre-claim and is carried on the plan.
	gate, gerr := stepDecision(o, plan.gateHuman, repo, plan.decisionScript)
	if gerr != nil {
		return gerr
	}
	o.say("%s %s", stepDecisionGate, gate)

	// 5 — the dispatcher's model attestation.
	stamp, serr := stepStamp(o, repo)
	if serr != nil {
		return serr
	}
	o.say("%s %s", stepModelStamp, stamp)

	// 6 — the prompt.
	return emitPrompt(o, prompt)
}

// dispatchPlan is what validateCallerPreconditions derives once, so no later step
// re-derives a value the validation was performed against. Re-deriving is how a check and
// the thing it checked drift apart.
type dispatchPlan struct {
	repo   string
	branch string
	wtName string
	// gateHuman is the EFFECTIVE human-decision gate: the explicit --gate-human flag OR a
	// --brief whose own frontmatter gates on a human (`gate: human`). Derived once here so
	// the decision-gate step, its pre-claim precondition (the decision script must exist),
	// and the prompt's human-gated line all read the SAME answer — keying the gate on
	// --gate-human alone silently dropped the metadata half of the contract.
	gateHuman bool
	// claimKey is the key the durable claim is taken (and later released) under —
	// derived once from the item key by claimKeyFor, so the acquire call and the release
	// hint in the prompt cannot drift onto two different keys.
	claimKey string
	// claimScript / decisionScript are the RESOLVED consumer-script paths (under
	// --claim-root when given, else --root), derived once here so the presence check and
	// the invocation cannot drift onto two different files.
	claimScript    string
	decisionScript string
}

// validateCallerPreconditions checks EVERY caller-controlled precondition, and it runs
// BEFORE the claim is acquired.
//
// WHY THE PLACEMENT IS THE WHOLE POINT. The claim is a DURABLE, cross-machine lock and the
// worktree is real state on disk. A refusal raised after either exists does not merely
// fail — it WEDGES the item: the claim stays held by a dispatcher that never dispatched, so
// every later attempt (including the operator's corrected re-run one second later) is told
// "already claimed by a LIVE holder", and the worktree is leaked with nothing recording
// that it should be reclaimed. The cost of a bad flag value must be a refusal, not an item
// nobody can pick up until a human hand-deletes a ref.
//
// So the rule this function exists to enforce is: **anything the CALLER controls, and that
// is therefore knowable without touching any durable state, is decided here.** After this
// returns, the only remaining failures are ones that could not have been known earlier — a
// claim someone else holds, a tree that would not yield a worktree, a forge that would not
// answer. Those are genuinely unverifiable, and they say so.
//
// A validation that migrates back down into a step is a regression this file's tests are
// written to catch: TestNoCallerPreconditionIsCheckedAfterTheClaim drives the whole table
// of bad inputs and asserts NOTHING was executed.
func validateCallerPreconditions(o dispatchOpts) (dispatchPlan, error) {
	var plan dispatchPlan

	if !itemKeyRe.MatchString(o.item) {
		return plan, deskkit.Refused(fmt.Sprintf(
			"step %s: %q is not a usable item key (letters, digits, dot, dash, underscore, slash; no "+
				"leading dash). The claim key is derived from it by a fixed rule every desk shares, so a key "+
				"outside this alphabet is one another desk would not derive the same claim from — and a claim "+
				"that does not collide is not a claim.",
			stepClaimAcquire, o.item))
	}

	// The tier vocabulary is CLOSED and is not a second list: it is derived from the
	// dispatch-tier set the stamp reader validates against.
	if !validTier(o.tier) {
		return plan, deskkit.Refused(fmt.Sprintf(
			"step %s: --tier %q is outside the tier vocabulary (%s). The tier is an attestation of what was "+
				"launched, so an unrecognised value must not be recorded as though it meant something.",
			stepModelStamp, o.tier, strings.Join(deskkit.DispatchTiers(), "|")))
	}

	// BOTH kits must be readable now. The common kit is checked here and not only at
	// assembly time because a binary built without it would otherwise take the claim and
	// then discover it cannot produce a prompt.
	if _, err := kitText(o.kit); err != nil {
		return plan, err
	}
	if _, err := commonKitText(); err != nil {
		return plan, err
	}

	// The model stamp is validated HERE, not in its own step. The stamp is applied last,
	// but its INPUT is a caller flag: discovering a malformed slug at step 5 would mean
	// discovering it with the claim held and the worktree built.
	if strings.TrimSpace(o.model) != "" {
		if _, err := deskkit.ModelStampLabels(o.model, o.tier); err != nil {
			return plan, deskkit.Refused(fmt.Sprintf("step %s: %v", stepModelStamp, err))
		}
	}

	repo, err := o.resolveRepo()
	if err != nil {
		return plan, err
	}
	if !deskkit.IsAllowedRepo(repo) {
		return plan, deskkit.Refused(fmt.Sprintf(
			"step %s: %s is not in the desk repo set — this verb dispatches work only into repos the desk "+
				"is rostered to act on.", stepClaimAcquire, repo))
	}
	plan.repo = repo
	plan.claimKey = claimKeyFor(o.item, repo)

	plan.branch = o.branch
	if plan.branch == "" {
		plan.branch = "feat/" + sanitizeSegment(o.item)
	}
	// The worktree verb is the AUTHORITY on what branch and worktree names it accepts; this
	// is a pre-check, deliberately no looser than its constraint, whose only job is to keep
	// a name it would reject from costing a held claim. It does not replace that check.
	if !branchNameRe.MatchString(plan.branch) || strings.Contains(plan.branch, "..") {
		return plan, deskkit.Refused(fmt.Sprintf(
			"step %s: --branch %q is not a plain branch name (letters, digits, dot, dash, underscore, "+
				"slash; no leading dash, no '..'), so the worktree verb would refuse it.",
			stepWorktreeCreate, plan.branch))
	}
	plan.wtName = sanitizeSegment(o.item)
	if !worktreeNameRe.MatchString(plan.wtName) {
		return plan, deskkit.Refused(fmt.Sprintf(
			"step %s: the item key %q does not reduce to a usable worktree name — pass --branch and a key "+
				"that does.", stepWorktreeCreate, o.item))
	}

	// The consumer scripts' HOME. --root is the ITEM's repo and stays the worktree source;
	// --claim-root exists because the scripts were centralized out of the consumer repos,
	// so a cross-repo dispatch needs "where the claim tool lives" and "which checkout the
	// worker branches from" to be two different answers. An explicit --claim-root is
	// AUTHORITATIVE — there is no silent fall-back to --root, because a fall-back would
	// turn a mispointed flag into a dispatch that only looked configured.
	scriptsRoot := o.root
	if s := strings.TrimSpace(o.claimRoot); s != "" {
		abs := s
		if a, err := filepath.Abs(s); err == nil {
			abs = a
		}
		fi, err := os.Stat(abs)
		if err != nil || !fi.IsDir() {
			return plan, deskkit.Refused(fmt.Sprintf(
				"step %s: --claim-root %q is not a directory this verb can read — it must name the checkout "+
					"that carries %s.", stepClaimAcquire, s, claimScriptRel))
		}
		scriptsRoot = abs
	}
	plan.claimScript = filepath.Join(scriptsRoot, filepath.FromSlash(claimScriptRel))
	plan.decisionScript = filepath.Join(scriptsRoot, filepath.FromSlash(decisionScriptRel))

	// The human-decision gate's own preconditions: the flag pairing AND the script's
	// presence. Both are knowable now, and both used to be discovered at step 4.
	//
	// An item is human-gated when the caller says so (--gate-human) OR when the brief's own
	// frontmatter gates on a human (`gate: human`). The metadata half is not a nicety — it is
	// half the contract the skills state, and keying the gate on --gate-human alone silently
	// defeated it: a `gate: human` brief passed by --brief alone printed "not human-gated" and
	// dispatched a worker against an EMPTY decision surface. The explicit flag stays the
	// guaranteed path; brief-detection is additive and best-effort (an unreadable brief falls
	// back to the flag — briefGatesHuman never yields a false positive).
	if o.gateHuman && strings.TrimSpace(o.brief) == "" {
		return plan, deskkit.Refused(fmt.Sprintf(
			"step %s: --gate-human needs --brief <path> — the decision issue's content is DERIVED from "+
				"the item's own specification, never invented by the dispatcher.", stepDecisionGate))
	}
	plan.gateHuman = o.gateHuman
	if !plan.gateHuman && strings.TrimSpace(o.brief) != "" && briefGatesHuman(o.root, o.brief) {
		plan.gateHuman = true
	}
	if plan.gateHuman {
		if _, err := os.Stat(plan.decisionScript); err != nil {
			return plan, deskkit.Unverifiable(fmt.Sprintf(
				"step %s: %s is not present in %s, so the human-decision gate cannot be ensured. Dispatching "+
					"a human-gated item with nothing in front of the human is the failure this gate exists to "+
					"close.", stepDecisionGate, decisionScriptRel, scriptsRoot), err)
		}
	}

	// The claim script's presence is knowable now too. stepClaim keeps its own check —
	// it is the step that must not proceed without one — but finding it missing here costs
	// nothing and keeps the "no durable state before this returns" invariant total.
	if _, err := os.Stat(plan.claimScript); err != nil {
		return plan, deskkit.Unverifiable(fmt.Sprintf(
			"step %s: %s is not present in %s, so no durable claim can be taken. A claim this verb cannot "+
				"place is NOT permission to proceed%s.", stepClaimAcquire, claimScriptRel, scriptsRoot,
			claimRootHint(o)), err)
	}

	// --prompt-file's directory. The prompt is written LAST, so an unwritable destination
	// would otherwise be discovered with the claim held, the worktree built, and the
	// decision issue filed — the most expensive possible moment to learn it.
	if p := strings.TrimSpace(o.promptFile); p != "" {
		dir := filepath.Dir(p)
		fi, err := os.Stat(dir)
		if err != nil {
			return plan, deskkit.Refused(fmt.Sprintf(
				"step %s: --prompt-file %q names a directory that does not exist (%s).",
				stepPromptEmit, p, dir))
		}
		if !fi.IsDir() {
			return plan, deskkit.Refused(fmt.Sprintf(
				"step %s: --prompt-file %q sits under %s, which is not a directory.",
				stepPromptEmit, p, dir))
		}
	}

	return plan, nil
}

// stepClaim acquires the durable claim by invoking the CONSUMER repo's own claim script.
//
// The script's exit codes ARE the deskkit contract (0 acquired · 5 refused · 6 could not
// be established), so they pass straight through with no re-interpretation. But exit 5
// covers two OPPOSITE conditions and only the script's other verbs can tell them apart: a
// LIVE holder owns the key (a collision — do not proceed, never steal), or the script
// refused the INVOCATION itself (a malformed key, a bad flag) and no claim was ever read.
// Reporting the second as the first turned every claim-tool refusal into a phantom
// "already claimed by a LIVE holder — (no output)" that no release or steal could clear,
// because there was nothing to clear. So on exit 5 this asks `show` for the holder and
// reports a collision ONLY when a holder was actually read; otherwise it surfaces the
// script's own refusal text as the error it is.
// The script runs with the ITEM's checkout as its working directory even when the script
// FILE resolves under --claim-root: the target repo is always passed explicitly via
// --repo, and any cwd-derived fallback inside the script should resolve to the item's
// repo, never to the checkout that merely happens to carry the tool.
func stepClaim(o dispatchOpts, repo, script, claimKey string) error {
	if _, err := os.Stat(script); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: %s is not present, so no durable claim can be taken. A claim this verb cannot "+
				"place is NOT permission to proceed — a machine-local lock would serialise two dispatchers "+
				"on one machine and nothing at all across two, which is the case that double-dispatches.",
			stepClaimAcquire, script), err)
	}
	args := []string{"acquire", claimKey, "--repo", repo}
	if o.branch != "" {
		args = append(args, "--branch", o.branch)
	}
	r := runCmd(o.root, script, args...)
	if r.err == nil {
		return nil
	}
	switch exitCodeOf(r.err) {
	case deskkit.ExitRefused:
		show := runCmd(o.root, script, "show", claimKey, "--repo", repo)
		holder := firstLine(show.stdout)
		// A holder was READ: the show verb succeeded, said something, and did not say the
		// key is FREE. Only this is a collision — but a collision is not the same as a LIVE
		// holder. The two-phase claim TTL (dispatch-claim.sh: state=claimed→20m,
		// state=dispatched→120m) makes a claim past its TTL DEAD and reclaimable, not live.
		// Reporting any non-FREE holder as "a LIVE holder — do not proceed" is what wedged an
		// item behind a dead dispatcher's claim for days: the operator was sent to hand-clear
		// a ref the claim tool would reclaim on its very next acquire.
		if show.err == nil && strings.TrimSpace(show.stdout) != "" &&
			!strings.Contains(show.stdout, "FREE "+claimKey) {
			if stale, state, ageMin := holderIsStale(show.stdout); stale {
				return deskkit.Refused(fmt.Sprintf(
					"step %s: %s is held by a STALE claim (state=%s age=%dm, past its TTL) — a dead "+
						"dispatcher's claim, NOT a live holder. It is reclaimable: the claim tool reclaims a "+
						"stale claim on its next `acquire`, so re-run; if it persists (its branch is still in "+
						"flight, a branch-as-claim the tool keeps) reclaim deliberately with `%s steal %s "+
						"--repo %s --reason <why>`. This verb still does not steal inline. Existing claim: %s.",
					stepClaimAcquire, claimKey, state, ageMin, script, claimKey, repo, holder))
			}
			return deskkit.Refused(fmt.Sprintf(
				"step %s: %s is already claimed by a LIVE holder — do not proceed. Existing claim: %s. "+
					"This verb never steals: breaking a live claim is a deliberate, auditable act with a stated "+
					"reason, and it belongs to a human or to the claim tool's own steal verb.",
				stepClaimAcquire, claimKey, holder))
		}
		return deskkit.Refused(fmt.Sprintf(
			"step %s: the claim tool refused to acquire %s (%s). No live holder was read (show: %s), so "+
				"this is a claim-acquire error, NOT a collision — fix the key or the invocation and re-run.",
			stepClaimAcquire, claimKey, firstLine(refusalDetail(r)), holder))
	default:
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the claim on %s could not be established (%s) — fail closed, NEVER 'assume free'.",
			stepClaimAcquire, claimKey, firstLine(r.stderr)), r.err)
	}
}

// refusalDetail picks the claim tool's refusal text: its errors go to stderr, its DEDUP
// log lines to stdout, so stderr is preferred and stdout is the fallback.
func refusalDetail(r runResult) string {
	if strings.TrimSpace(r.stderr) != "" {
		return r.stderr
	}
	return r.stdout
}

// claimedClaimTTL is the age past which a `state=claimed` dispatch claim — acquired but never
// advanced to `dispatched` — is DEAD (its dispatcher never reached the `progress` verb). It
// mirrors the two-phase dispatch-claim contract's CLAIMED_TTL_MIN. The `dispatched` half of
// that contract IS deskkit.DefaultStaleClaim (the one named "120m, no live branch" constant,
// reused so the two do not drift); the `claimed` half has no deskkit constant to borrow, so it
// is named here against the same contract rather than as a bare literal.
const claimedClaimTTL = 20 * time.Minute

// claimStateFieldRe / claimAgeFieldRe pull the `state=` and `age=<N>m` fields out of the claim
// tool's `show` output (dispatch-claim.sh cmd_show prints
// `HELD <id> — ... state=<state> ... age=<N>m`).
var (
	claimStateFieldRe = regexp.MustCompile(`state=([A-Za-z]+)`)
	claimAgeFieldRe   = regexp.MustCompile(`age=(\d+)m`)
)

// holderIsStale reports whether a claim `show` output describes a holder past its state's TTL —
// a DEAD claim the two-phase dispatch-claim contract makes reclaimable, not a live holder.
//
// The TTLs are that contract: state=dispatched → deskkit.DefaultStaleClaim (120m),
// state=claimed → claimedClaimTTL (20m). A missing or unparseable state/age, or an
// unrecognised state, yields FALSE — a claim this verb cannot PROVE dead is treated as live
// and never reported as reclaimable, the same fail-closed direction deskkit's own isStale
// takes (never steal a claim you cannot prove is dead). state and ageMin are returned for the
// message even when stale is false, so the caller can name what it read.
func holderIsStale(showOut string) (stale bool, state string, ageMin int) {
	ageMin = -1
	sm := claimStateFieldRe.FindStringSubmatch(showOut)
	am := claimAgeFieldRe.FindStringSubmatch(showOut)
	if sm != nil {
		state = strings.ToLower(sm[1])
	}
	if am == nil {
		return false, state, ageMin
	}
	n, err := strconv.Atoi(am[1])
	if err != nil {
		return false, state, ageMin
	}
	ageMin = n

	var ttlMin int
	switch state {
	case "dispatched":
		ttlMin = int(deskkit.DefaultStaleClaim.Minutes())
	case "claimed":
		ttlMin = int(claimedClaimTTL.Minutes())
	default:
		return false, state, ageMin // unknown/absent state — cannot prove dead
	}
	return ageMin >= ttlMin, state, ageMin
}

// stepRoster registers the work entry when a PR is already known.
//
// At first dispatch there is usually no PR yet, and the roster's work entry is keyed on
// one. Rather than inventing a placeholder number — a roster row pointing at a PR that
// does not exist is worse than no row, because a sweep will act on it — the registration
// becomes the AGENT's first act after its PR opens, and the exact command is carried in
// the prompt. This step says which of the two happened; it never goes silent.
func stepRoster(o dispatchOpts, repo string) string {
	if o.pr <= 0 {
		return "DEFERRED: no PR yet — the agent self-registers the instant its draft PR opens " +
			"(the exact command is in the emitted prompt)"
	}
	short := shortRepo(repo)
	r := runCmd("", "deskroster", "set", "--repo", short, "--pr", fmt.Sprint(o.pr), "--what", o.item)
	if r.err != nil {
		// A roster row is a legibility aid, not a correctness gate: the claim already
		// serialises the dispatch. Failing the whole dispatch here would trade a real
		// dispatch for a bookkeeping miss, so this reports and continues — loudly.
		return fmt.Sprintf("WARNING: could not register %s#%d on the roster (%s) — dispatch continues; "+
			"`deskroster list` will not show this work until the agent registers it", short, o.pr, firstLine(r.stderr))
	}
	return fmt.Sprintf("OK: %s#%d registered", short, o.pr)
}

// stepDecision ensures a human-gated item has a decision issue in front of the human
// BEFORE its agent starts.
//
// The gate exists because a human-gated item used to wait on a decision nobody had been
// asked to make: the item sat there with the human-decision surface empty. Filing at first
// dispatch is what puts a concrete thing in the queue. The consumer script owns the
// content rules and the dedupe; this verb only ensures it runs at the right moment.
// Its caller-controlled preconditions — the --gate-human/--brief pairing and the script's
// presence — are validated in validateCallerPreconditions, before the claim exists. What
// remains here is only what running the script can tell us.
func stepDecision(o dispatchOpts, gateHuman bool, repo, script string) (string, error) {
	if !gateHuman {
		return "SKIPPED: item is not human-gated", nil
	}
	r := runCmd(o.root, script, "ensure", o.brief, "--repo", repo, "--at", "start")
	if r.err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the decision-issue gate for %s could not be ensured (%s) — a possible duplicate is the "+
				"cheap direction and a missing gate is the expensive one, so this fails closed.",
			stepDecisionGate, o.brief, firstLine(r.stderr)), r.err)
	}
	return "OK: " + firstLine(r.stdout), nil
}

// stepStamp computes the dispatcher's model attestation and applies it when a PR is known.
//
// THE STAMP IS ATTESTATION OF DISPATCH, NOT SURVEILLANCE OF EXECUTION. The dispatcher
// knows exactly which model it launched; that is the one input on this axis that is not
// self-report. It does NOT claim to have observed every token the agent later produced.
// The labels are therefore applied under the DISPATCHER's identity — a stamp an agent
// could apply to itself reads as indeterminate by design, which is the whole point.
//
// With no --pr there is no PR to label yet, so the step computes and VALIDATES the labels
// and emits them for the moment the draft PR opens. Validating early is deliberate: a
// malformed slug discovered at label time is discovered after the agent has already run.
func stepStamp(o dispatchOpts, repo string) (string, error) {
	if strings.TrimSpace(o.model) == "" {
		return "SKIPPED: no --model given — this dispatch contributes NO model-keyed signal " +
			"(unknown is never a default model)", nil
	}
	// The slug and tier were already validated in validateCallerPreconditions, before the
	// claim existed; this re-derives the labels from the same inputs. An error here would
	// mean the two calls disagreed, which is a defect rather than a caller mistake — so it
	// is UNVERIFIABLE, not a refusal, and it names the contradiction.
	labels, err := deskkit.ModelStampLabels(o.model, o.tier)
	if err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the model stamp validated before the claim was taken but will not build now (%v) — "+
				"the two reads of the same inputs disagree", stepModelStamp, err), err)
	}
	if o.pr <= 0 {
		return "PENDING: apply " + strings.Join(labels, " + ") +
			" under the DISPATCHER's identity the instant the draft PR opens", nil
	}
	for _, l := range labels {
		// Label provisioning is idempotent and an already-exists error is the success
		// case: two dispatchers stamping in parallel must both end up with the label
		// present, not one of them failing.
		_ = runCmd("", "gh", "label", "create", l, "-R", repo, "--force")
		if r := runCmd("", "gh", "pr", "edit", fmt.Sprint(o.pr), "-R", repo, "--add-label", l); r.err != nil {
			return "", deskkit.Unverifiable(fmt.Sprintf(
				"step %s: could not apply %s to %s#%d (%s) — an INCOMPLETE stamp (one label of two) reads "+
					"as indeterminate, which is worse than no stamp at all.",
				stepModelStamp, l, repo, o.pr, firstLine(r.stderr)), r.err)
		}
	}
	return "OK: applied " + strings.Join(labels, " + "), nil
}

// validTier checks the tier against the dispatch-tier vocabulary the stamp reader owns,
// rather than against a second hand-written list here.
func validTier(t string) bool {
	for _, v := range deskkit.DispatchTiers() {
		if strings.EqualFold(strings.TrimSpace(t), v) {
			return true
		}
	}
	return false
}

// claimRootHint names the way out of a missing-claim-script failure when no --claim-root
// was given: the scripts were centralized, so the target repo lacking them is the
// EXPECTED cross-repo shape, not a broken checkout.
func claimRootHint(o dispatchOpts) string {
	if strings.TrimSpace(o.claimRoot) != "" {
		return ""
	}
	return " — if the item's repo does not carry the consumer scripts (they are centralized), " +
		"point --claim-root at the checkout that does; --root stays the item's own repo"
}

// claimKeyFor derives the durable claim key the repo's claim tool requires from a plan
// item key.
//
// dispatch-claim.sh's key grammar is `<repo>--<stream>--<NN>` (or `<repo>--issue-<NN>`):
// the `<repo>` prefix is MANDATORY (two repos can own a stream of the same name), and any
// key with no `--` in it is refused outright. The drain planners, though, name items in
// board form — verifyloop plan emits `<stream>/<NN>` — and passing that form through
// unchanged made every verifier dispatch die at claim-acquire on a malformed-key refusal.
//
// The rule is deterministic, which is the property a claim key must have — two desks
// dispatching the same item MUST derive the same key or their claims do not collide:
//
//   - a key already carrying `--` IS a claim key (the worker/reviewer paths pass
//     `<repo>--<stream>--<NN>` directly) and passes through byte-for-byte;
//   - anything else is a plan item key: the repo's short label — the ONE shared resolver,
//     deskkit.RepoShortLabel: the configured repo-alias short name, else the repo
//     basename — is prefixed, and every `/` becomes `--`, so `verdict-lane/05` in
//     owner/name becomes `<short>--verdict-lane--05`.
//
// Because the alias short name participates in the key, desks that co-dispatch a repo
// must share their alias configuration on this path — the alias is part of the
// coordination contract here, not display.
//
// Only the CLAIM calls (acquire, the show on contention, the release hint in the prompt)
// use the derived key. The human-facing derivations — worktree name, branch, brief path,
// the prompt's item key — stay on the ORIGINAL item key: reshaping those is exactly what
// made passing the translated key by hand corrupt the dispatch instead of working around
// it.
func claimKeyFor(item, repo string) string {
	if strings.Contains(item, "--") {
		return item
	}
	return deskkit.RepoShortLabel(repo) + "--" + strings.ReplaceAll(strings.Trim(item, "/"), "/", "--")
}

// sanitizeSegment reduces an item key to one filesystem/branch-safe segment.
func sanitizeSegment(item string) string {
	s := strings.Trim(item, "/")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "..", "-")
	return s
}

// shortRepo is the roster's short repo name (the name half of owner/name).
func shortRepo(repo string) string {
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		return repo[i+1:]
	}
	return repo
}

// resolveRepo returns the owner/name the item belongs to: --repo, else the root's origin.
func (o dispatchOpts) resolveRepo() (string, error) {
	if strings.TrimSpace(o.repo) != "" {
		return strings.TrimSpace(o.repo), nil
	}
	r := runCmd(o.root, "git", "remote", "get-url", "origin")
	if r.err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: cannot read origin's URL in %s (%s) — pass --repo <owner/name>. An item dispatched "+
				"into the wrong repo is work nobody asked that repo for.",
			stepClaimAcquire, o.root, firstLine(r.stderr)), r.err)
	}
	slug := repoSlugFromURL(r.stdout)
	if slug == "" {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: origin %q does not parse to an owner/name — pass --repo <owner/name>.",
			stepClaimAcquire, r.stdout), nil)
	}
	return slug, nil
}

func (o dispatchOpts) say(format string, args ...any) {
	if o.quiet {
		return
	}
	fmt.Fprintf(os.Stderr, "deskdispatch: "+format+"\n", args...)
}

// emitPrompt writes the assembled prompt to --prompt-file or stdout. The file is written
// with 0600 and never appended to: a prompt file that accumulated two dispatches would
// hand the second agent the first one's item.
func emitPrompt(o dispatchOpts, prompt string) error {
	if strings.TrimSpace(o.promptFile) == "" {
		fmt.Println(prompt)
		return nil
	}
	if err := os.WriteFile(o.promptFile, []byte(prompt+"\n"), 0o600); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: cannot write the prompt to %s", stepPromptEmit, o.promptFile), err)
	}
	o.say("%s OK: prompt written to %s (%d bytes)", stepPromptEmit, o.promptFile, len(prompt))
	return nil
}

func audit(o dispatchOpts, err error) {
	result := deskkit.ResultOK
	detail := "dispatch prepared item=" + o.item + " tier=" + o.tier + " kit=" + o.kit
	if o.dryRun {
		detail = "dry-run item=" + o.item
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
		Verb:   "dispatch",
		Result: result,
		Detail: detail,
		Title:  o.item,
	}); lerr != nil {
		fmt.Fprintf(os.Stderr, "deskdispatch: WARNING: could not write audit line: %v\n", lerr)
	}
}
