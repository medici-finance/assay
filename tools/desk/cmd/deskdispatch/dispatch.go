package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "deskdispatch"

// Step names — the verb's contract. A caller reads which step stopped the dispatch out of
// the failure line, so these strings are as load-bearing as the exit codes.
const (
	// stepAdmission is the reservation gate (example-stream/18). It is NOT a member of
	// dispatchSteps below — that slice is the always-run 7-step contract, and this gate runs
	// ONLY when ASSAY_REPAIR_ADMISSION=on, before the claim. Naming it here lets its OK/refusal
	// lines identify themselves exactly as the numbered steps do.
	stepAdmission      = "admission"
	stepClaimAcquire   = "claim-acquire"
	stepWorktreeCreate = "worktree-create"
	stepRosterRegister = "roster-register"
	stepDecisionGate   = "decision-gate"
	stepModelStamp     = "model-stamp"
	stepQueueLabel     = "queue-label"
	stepPromptEmit     = "prompt-emit"
)

// queueLabelAuthorizationNeeded is the review-lane entry signal: a change a reviewer has
// been dispatched onto is "waiting on the reviewer's work before it is ready for a human".
// deskflip removes it and applies `approval-needed` at the ready-flip (deskflip's
// labelBeforeFlip/labelAfterFlip). It is defined here as well as in deskflip because each
// verb owns the label at its own end of the lane; the two spellings must stay identical.
// queueLabelColorHex is the bare hex the label is CREATED with when the repo does not yet
// carry it — no leading `#` (each backend renders its own form), matching deskflip.
const (
	queueLabelAuthorizationNeeded = "authorization-needed"
	queueLabelColorHex            = "0e8a16"
)

// stampLabelColorHex / stampLabelDescription are the cosmetics a dispatched-* stamp label is
// CREATED with when the repo or project does not yet carry it. The NAME is the load-bearing
// part (deskkit.ModelStampLabels); these exist because a forge's label-create call needs a
// colour (GitLab requires one) and a reader deserves to be told what the label attests.
const (
	stampLabelColorHex    = "ededed"
	stampLabelDescription = "Dispatch attestation: the model/tier the dispatcher launched for this change, applied under the dispatcher's own identity"
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
	stepQueueLabel,
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

// goClaimBinary is the pure-Go port of the claim script (cmd/deskclaim-ref). It is the
// PREFERRED claim tool: whenever it resolves on PATH (by bare name, the way deskwt/deskroster
// are), this verb invokes it and hands it the dispatching role's token as `--token-file`;
// the legacy tools/dispatch-claim.sh is the fallback for a tree that predates the binary,
// and it receives the same token as GH_TOKEN in its environment. A green-field adopter —
// including a native-Windows one, where a shebang `.sh` will not run under CreateProcess —
// therefore dispatches with NO consumer script on disk and NO tribal --claim-root. It speaks
// the SAME wire protocol as the script (the refs/dispatch/<id> claim namespace, the same
// holder encoding, the same 0/5/6 exit codes), so which one runs never changes where the
// claim lands or whether two dispatchers collide. Issues 708 and 1151.
const goClaimBinary = "deskclaim-ref"

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
	// worktree is an operator-STATED home for the agent, accepted ONLY with --dry-run.
	// A real dispatch names the path deskwt printed and nothing else — this flag never
	// reaches one. See validateOperatorWorktree for the fail-closed checks it must pass.
	worktree string
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
	worktree := fs.String("worktree", "", "with --dry-run ONLY: render the prompt against this operator-stated, "+
		"already-existing home worktree instead of the not-yet-known placeholder. Refused on a real dispatch")

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
		promptFile: *promptFile, quiet: *quiet, dryRun: *dryRun, worktree: *worktree,
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
		wtBanner := ""
		if plan.home != "" {
			// The banner records that the path was CHECKED, not predicted — a transcript reader
			// can tell an operator-supplied verified home from the placeholder the verb renders
			// when it does not know where the worktree will land.
			wtBanner = fmt.Sprintf(" worktree=%s (operator-supplied, verified)", plan.home)
		}
		shownBranch := branch
		if plan.detached {
			shownBranch = "(detached off origin/main — verifier touches no branch)"
		}
		fmt.Printf("deskdispatch: PLAN (dry run — nothing touched) item=%s repo=%s tier=%s kit=%s branch=%s%s\n",
			o.item, repo, o.tier, o.kit, shownBranch, wtBanner)
		for i, s := range dispatchSteps {
			fmt.Printf("  %d %s\n", i+1, s)
		}
		// Step 2 records the run key worktree-locally on a real dispatch; name it here so an
		// operator can see which STOP.run.<key> would stop this run before launching it.
		fmt.Printf("  run key (recorded at step %s as assay.runKey): %s\n", stepWorktreeCreate, plan.claimKey)
		if line, herr := deskkit.HookDryRunLine(deskkit.HookBeforeRun); herr != nil {
			return herr
		} else {
			fmt.Println("  " + line)
		}
		prompt, perr := assemblePrompt(o, plan, plan.home)
		if perr != nil {
			return perr
		}
		return emitPrompt(o, prompt)
	}

	// PHANTOM CHECK — before the claim (phantom.go). A fresh worker dispatch whose brief already
	// has an OPEN or MERGED PR is refused here, keyed on that PR's `Brief:` trailer rather than a
	// derived branch name, so the branch-naming mismatch that let phantom rows through is closed. It
	// wedges nothing (no claim yet) and is a no-op for a review/verifier dispatch or a --pr resume.
	if err := phantomCheck(o, repo); err != nil {
		return err
	}

	// Advisory write-scope overlap echo, BEFORE the claim — a coordination hint
	// the operator sees, then the dispatch PROCEEDS. It never blocks, never gates the claim,
	// and carries no exit code: a foreseeable merge collision on a shared file is surfaced now
	// rather than at merge, and proceeding over it is correct when the overlap is intended.
	echoWriteOverlap(os.Stderr, o)

	// 1 — the durable claim, FIRST. Everything after this is work a second dispatcher
	// must not also be doing. The claim child is handed the DISPATCHING role's credential
	// before it runs (resolveClaimAuth): the claim is a forge write, and the stamp step's mint
	// comes four steps too late to serve it — so the claim step mints on demand, from the
	// same seam, and fails closed on the same refusal. Issue 1151. The same resolution also
	// carries the credential the decision-gate script (step 4) runs under, so it too is taken
	// HERE, before anything durable, rather than at step 4 with the claim already held — issue
	// 1146.
	//
	// The credential is minted ONLY when the resolved claim store needs one (forge-neutral/21):
	// the forge-ref store writes refs on the forge; a store that writes none is handed none.
	// (Every store this build can resolve to is the forge-ref store, so every dispatch still
	// mints here exactly as before; the decision gate below reads the same resolution.)
	var auth claimAuth
	if plan.claimStore.NeedsForgeCredential {
		var aerr error
		auth, aerr = resolveClaimAuth(o, repo, plan.forgeKind, plan.claimToolIsScript)
		if aerr != nil {
			return aerr
		}
	}

	// ADMISSION (example-stream/18) — the per-class reservation gate, serialized across
	// dispatchers, BEFORE the durable item claim. It is inert unless ASSAY_REPAIR_ADMISSION=on
	// (returns a nil release and no error), so the shipped default flow is unchanged. When on, an
	// ADMIT hands back a lease-release that stays held THROUGH stepClaim so the count->decide->claim
	// window is atomic across hosts; a HOLD or a could-not-check refuses HERE, before any durable
	// item state exists — a refused fresh dispatch never wedges the item because no claim was taken.
	admitRelease, admitErr := enforceAdmission(o, plan, repo, auth)
	if admitErr != nil {
		return admitErr
	}

	if err := stepClaim(o, repo, plan.claimTool, plan.claimToolIsScript, auth, plan.claimKey); err != nil {
		// The admission lease was taken and no item claim was placed — release it so a corrected
		// re-run is not serialized behind a dispatcher that never claimed. The recovery order
		// (reserve -> claim -> release) holds: with no item claim there is no occupancy to leak.
		if admitRelease != nil {
			admitRelease()
		}
		return err
	}
	// The item claim is now the durable occupancy; the count->decide->claim window is closed, so
	// release the serialization lease (a crash before here would have expired it by TTL anyway).
	if admitRelease != nil {
		admitRelease()
	}
	o.say("%s OK: %s claimed in %s (claim key %s) via %s, store %s, authenticated by %s",
		stepClaimAcquire, o.item, repo, plan.claimKey, plan.claimTool, plan.claimStore.Label(), auth.source)

	// 2 — the agent's worktree, in the ITEM's repo. deskwt owns the safety here (a
	// sanctioned path prefix, an unambiguous base, no clobber of an existing target), so
	// this step delegates rather than re-deriving any of it — INCLUDING where the
	// worktree lands: the path the prompt names is the one deskwt printed, never one this
	// verb predicted.
	// Both arms take the SAME base expression: worktreeBase already answers mainlineRef for a
	// verifier kit (o.pr<=0 || reviewKit || verifierKit), so the detached lane is unchanged by
	// using it, while the branch lane keeps the PR-resume base main introduced. The only
	// difference between the arms is --detach vs --branch, which is the verifier's whole point:
	// it reads merged main and touches no feature branch.
	var wt runResult
	if plan.detached {
		wt = runCmd(o.root, "deskwt", "add", wtName, "--detach", "--base", worktreeBase(o, branch))
	} else {
		wt = runCmd(o.root, "deskwt", "add", wtName, "--branch", branch, "--base", worktreeBase(o, branch))
	}
	if wt.err != nil {
		// The durable claim was placed one step ago and this dispatch is now aborting, so
		// RELEASE it — exactly as the before_run failure path below does — rather than leave it
		// orphaned. An orphaned claim WEDGES the item: every later attempt, including the
		// operator's corrected re-run a second later, is told "already claimed by a LIVE holder",
		// and a human has to hand-delete the ref. A worktree-create abort that placed a claim and
		// never released it is the field defect this line closes.
		released := releaseClaim(o, plan.claimTool, auth, plan.claimKey, repo)
		// deskwt's OWN message is forwarded whole (SaidAll: preamble stripped, SCRUBBED —
		// see runtool.go), because it is the line that names the cause (which branch, which
		// worktree holds it, what to do). Not toolMessage(wt.stderr): that strips only the
		// `assay-config:` preamble and never scrubs, and this message reaches the operator
		// verbatim via FailVerbatim on every DESK_TRACE setting, on or off. The wrapper no
		// longer frames this as a transient tree fault to "fix and re-run"; instead it names
		// the commonest cause, which DIFFERS BY KIT and so must be selected by kit (#851). The
		// brief-lane hint — the brief's `feat/<id>` branch already existing — is meaningless on
		// the review lane, which has no brief and no feat branch; sending a reviewer to "look
		// for a merged/open PR" explains nothing. The review-lane hint points instead at the
		// reviewer-worktree lifecycle: a review kit checks the PR head out as a DETACHED HEAD,
		// so the earlier reviewer worktree for this PR must be reclaimed before a re-dispatch
		// on the same lane key can create its own.
		said := wt.run.SaidAll()
		msg := fmt.Sprintf(
			"step %s: `deskwt add %s` failed in %s. The claim was %s. %s deskwt said:\n%s",
			stepWorktreeCreate, wtName, o.root, released, worktreeCreateHint(o.kit, branch, said), said)
		// deskwt's exit code passes THROUGH: a refusal (5) is a decision it made — the branch
		// is held by a live worktree, or carries unpushed work — and flattening a decision
		// into "could not be established" tells the operator to retry something that will
		// never succeed on its own.
		// FailVerbatim, not Fail: the message above already carries deskwt's words in full,
		// so it is kept exactly as composed and only the out-of-band detail DESK_TRACE prints
		// (the `deskwt add` argv as executed, deskwt's exit status, its stderr whole) is
		// attached. The verdict is unchanged either way.
		if exitCodeOf(wt.err) == deskkit.ExitRefused {
			return wt.run.FailVerbatim(deskkit.ExitRefused, msg)
		}
		return wt.run.FailVerbatim(deskkit.ExitUnverifiable, msg)
	}
	home := firstLine(wt.stdout)
	// Absoluteness is tested with homeIsAbsolute (filepath.IsAbs under the host-OS seam), not a
	// literal leading-slash prefix: deskwt is Windows-aware and on native Windows deliberately picks
	// the sanctioned <repo-root>/.claude/worktrees/ prefix, whose home is a drive-rooted `C:\...`
	// that never starts with `/`. A POSIX-only `HasPrefix(home, "/")` here rejected that legitimate
	// home and made dispatch impossible on Windows (#757), the consumer half of the #727/#732 family.
	// This step delegates path SAFETY to deskwt (the sanctioned-prefix guard, comment above) and only
	// asserts the home is absolute — the same portable test brief.go uses — so producer and consumer
	// judge "absolute" the same way per OS and cannot disagree again.
	if home == "" || home == "(no output)" || !homeIsAbsolute(home) {
		// This abort is AFTER the durable claim was placed one step ago, so RELEASE it — exactly as
		// the deskwt-add-failed branch above does — rather than leave a phantom HELD claim that wedges
		// the item (every corrected re-run told "already claimed by a LIVE holder" until a human
		// hand-deletes the ref). A refused dispatch must not be a queue suppressor.
		released := releaseClaim(o, plan.claimTool, auth, plan.claimKey, repo)
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: `deskwt add %s` exited 0 but named no absolute worktree path (%q). The agent's home "+
				"is the isolation floor every other clause rests on, so a home this verb cannot state is a "+
				"dispatch it must not make. The claim was %s.", stepWorktreeCreate, wtName, wt.stdout, released), nil)
	}
	// IDENTITY, worktree-scoped (#1490). The dispatched agent's worktree must commit under its
	// OWN role's App identity, not the identity the shared checkout carries — otherwise a
	// verifier dispatched from a desk checkout reports the desk App as its runner and statusgen
	// stamps that wrong identity into every Evidence witness Runner cell. `deskwt add` has
	// already CLEARED the inherited identity worktree-scoped (its no-role floor is FATAL there),
	// so the worktree is FAIL-CLOSED — a commit refuses "Author identity unknown" — until this
	// stamps the right one. That floor is why this stamp is best-effort, the SAME class as the
	// run-key layer below: if the stamp cannot run, the worst outcome is a fail-closed worktree
	// whose first commit refuses loudly, NEVER one that silently inherits and misattributes. A
	// failure is REPORTED, never silent, and the identity is printed on the OK line only when it
	// was actually stamped, so the transcript never claims an identity the worktree lacks.
	idStamped := false
	if ext := runCmd(home, "git", "config", "extensions.worktreeConfig", "true"); ext.err == nil {
		nm := runCmd(home, "git", "config", "--worktree", "user.name", plan.identityName)
		em := runCmd(home, "git", "config", "--worktree", "user.email", plan.identityEmail)
		idStamped = nm.err == nil && em.err == nil
		if !idStamped {
			said := nm.run.Said()
			if nm.err == nil {
				said = em.run.Said()
			}
			o.say("%s WARNING: could not stamp the agent's %s-role commit identity in %s (%s) — the worktree "+
				"stays identity-CLEARED by deskwt add (fail-closed); a commit there refuses until an identity is set",
				stepWorktreeCreate, plan.identityRole, home, said)
		}
	} else {
		o.say("%s WARNING: could not enable extensions.worktreeConfig in %s (%s), so the agent's %s-role commit "+
			"identity was not stamped; the worktree stays identity-CLEARED (fail-closed) until one is set",
			stepWorktreeCreate, home, ext.run.Said(), plan.identityRole)
	}
	idSuffix := ""
	if idStamped {
		idSuffix = " identity=" + deskkit.RoleIdentityLabel(plan.identityRole)
	}
	if plan.detached {
		o.say("%s OK: %s detached off origin/main (verifier: no branch)%s", stepWorktreeCreate, home, idSuffix)
	} else {
		o.say("%s OK: %s on %s%s", stepWorktreeCreate, home, branch, idSuffix)
	}

	// Record the run key worktree-locally (assay.runKey) so the per-run stop layer
	// (deskkit.Guard's STOP.run.<key> check) resolves it from cwd with
	// NO agent cooperation: every desk verb the worker runs next reads the key from its own
	// worktree and refuses if that run has been stopped. extensions.worktreeConfig is already
	// on from the identity stamp above (a benign, idempotent repo setting). This is Layer A of
	// the two-layer stop; a failure here degrades to Layer B (the desk window's cadence sweep)
	// and is REPORTED, never silent — but it never fails the dispatch, which is already claimed,
	// homed and identity-stamped.
	if ext := runCmd(home, "git", "config", "extensions.worktreeConfig", "true"); ext.err == nil {
		if rk := runCmd(home, "git", "config", "--worktree", "assay.runKey", plan.claimKey); rk.err == nil {
			o.say("%s OK: recorded run key %s (assay.runKey) in %s", stepWorktreeCreate, plan.claimKey, home)
		} else {
			o.say("%s WARNING: could not record assay.runKey=%s in %s (%s) — the per-run stop's "+
				"cooperative layer is off for this run; the desk-window sweep still covers it",
				stepWorktreeCreate, plan.claimKey, home, rk.run.Said())
		}
	} else {
		o.say("%s WARNING: could not enable extensions.worktreeConfig in %s (%s) — the per-run "+
			"stop's cooperative layer is off for this run; the desk-window sweep still covers it",
			stepWorktreeCreate, home, ext.run.Said())
	}

	// before_run — runs after the worktree is prepared and BEFORE the prompt is emitted (the
	// agent's "run"). FATAL failure class: a failure ABORTS the attempt — no prompt is
	// emitted — and, because this is the one lifecycle point AFTER the durable claim, the
	// claim is RELEASED so a corrected re-run is not wedged behind a dispatcher that never
	// dispatched. The hook is the checked form of the KUBECONFIG=/dev/null envelope every
	// agent is otherwise asked to remember.
	if _, herr := deskkit.RunHook(deskkit.HookBeforeRun, deskkit.HookEnv{
		RunKey: plan.claimKey, Worktree: home, Repo: repo, Role: o.kit,
	}); herr != nil {
		released := releaseClaim(o, plan.claimTool, auth, plan.claimKey, repo)
		return deskkit.Unverifiable(fmt.Sprintf(
			"step before_run: the before_run hook failed, so no prompt is emitted. The claim was %s. Hook: %v",
			released, herr), herr)
	}

	prompt, perr := assemblePrompt(o, plan, home)
	if perr != nil {
		return perr
	}

	// 3 — roster registration.
	o.say("%s %s", stepRosterRegister, stepRoster(o, repo))

	// 4 — the human-decision gate. The effective gate (flag OR brief metadata) was decided
	// pre-claim and is carried on the plan.
	// The script child runs under the credential the claim step resolved (auth.scriptEnv), never
	// the ambient login — issue 1146.
	gate, gerr := stepDecision(o, plan.gateHuman, repo, plan.decisionScript, auth)
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

	// 6 — the review-lane queue label. When a reviewer is dispatched onto a known change,
	// apply `authorization-needed` forge-neutrally (the resolved forge's idempotent label
	// ensure+apply, under the reviewer role's own credential — a GitHub App token or a GitLab
	// PAT, never the deskpost verdict-write path), so a GitLab adopter's MR carries the same
	// queue signal a GitHub PR does (#795 §4). NON-FATAL: a label the forge would not accept
	// or a credential gap is a provisioning issue to file, never a reason to fail a dispatch
	// whose claim and worktree already stand — mirroring stepRoster and deskflip's
	// ensureLabelSwap.
	o.say("%s %s", stepQueueLabel, stepQueueLabelApply(o, repo))

	// 7 — the prompt.
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
	// claimTool is the RESOLVED claim-tool invocation target: the absolute
	// tools/dispatch-claim.sh path when the resolved root carries it, else the bare
	// goClaimBinary name (deskclaim-ref) resolved on PATH. claimToolIsScript records which,
	// so the presence backstop knows to os.Stat a path or lookPath a binary. Derived once so
	// the presence check and the invocation cannot drift onto two different tools.
	claimTool         string
	claimToolIsScript bool
	// claimReleaseHint is how the emitted prompt spells the claim tool in the agent's
	// release command: the repo-relative script path (machine-independent) for a legacy
	// script under --root, the resolved absolute path under --claim-root, or the bare
	// goClaimBinary name when the Go fallback is in use.
	claimReleaseHint string
	// decisionScript is the RESOLVED consumer decision-script path (under --claim-root when
	// given, else --root), derived once here so the presence check and the invocation cannot
	// drift onto two different files.
	decisionScript string
	// home is the operator-supplied, VALIDATED home worktree to render the dry-run prompt
	// against (from --worktree). It is "" on every path but a --dry-run that passed
	// validateOperatorWorktree — an empty value renders the not-yet-known placeholder, so a
	// real dispatch, which never sets it, is unaffected.
	home string
	// detached is set for a VERIFIER dispatch (#1309 item 6): the worktree is cut as a detached
	// HEAD off origin/main under a `verify-<item>` name (`deskwt add --detach`), branch is empty,
	// and no feature branch is created, named, or collided with.
	detached bool
	// identityRole is the DISPATCHED agent's own desk role — the one whose App commit
	// identity its worktree must carry (kitRole: worker/worker-objective→worker,
	// review→reviewer, verifier→verifier). It is distinct from stampRoleForKit, which names
	// the DISPATCHER's role for the model stamp; this names the AGENT's role for the worktree
	// commit identity. identityName/identityEmail are that role's resolved committer name and
	// email, resolved pre-claim through the SAME deskkit resolver role-init and `deskwt add
	// --role` use, so a role with no roster identity refuses BEFORE any durable state exists
	// (#1490) rather than a worktree inheriting the dispatching desk's identity and
	// misattributing every Evidence Runner cell.
	identityRole  string
	identityName  string
	identityEmail string
	// forgeKind is the resolved forge serving the target repo, set ONLY for a review
	// dispatch — the one kind whose prompt is forge-shaped (the head-fetch refspec: GitHub
	// refs/pull/<N>/head vs GitLab refs/merge-requests/<iid>/head, #773). A worker dispatch
	// emits no forge-shaped ref, so this stays empty for one and the worker path is
	// unchanged. Resolved pre-claim so a repo whose forge cannot be determined refuses the
	// review dispatch before any durable state exists.
	forgeKind deskkit.ForgeKind
	// claimStore is where the dispatch claim is kept, as deskkit.ResolveClaimStore decided it
	// from the roster (forge-neutral/21) — never a flag. Resolved pre-claim, so a store that
	// cannot be used refuses before any worktree is cut and before any credential is minted.
	claimStore deskkit.ClaimStoreResolution
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

	// --worktree is accepted ONLY together with --dry-run. On a real dispatch the home is the
	// path deskwt printed and nothing else, so an operator-stated one must never reach it —
	// letting it through would override deskwt's own placement, the one value the isolation
	// floor rests on. Checked FIRST, before any durable state or child process, so the refusal
	// costs nothing and TestWorktreeFlagRefusedOnRealDispatch can prove zero processes ran.
	if strings.TrimSpace(o.worktree) != "" && !o.dryRun {
		return plan, deskkit.Refused(fmt.Sprintf(
			"step %s: --worktree is accepted only with --dry-run. A real dispatch names the home worktree "+
				"deskwt printed and nothing else; an operator-stated path must not override that placement.",
			stepWorktreeCreate))
	}

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

	// The DISPATCHED agent's worktree commit identity, resolved HERE, pre-claim (#1490). The
	// worktree-create step stamps this into the new worktree's own config so it never inherits
	// the shared checkout's identity — the misattribution this closes: a verifier dispatched
	// from a desk checkout committed, and reported its runner, under the desk App's identity,
	// and statusgen stamped that wrong identity into every Evidence witness Runner cell. A kit
	// whose role has NO roster binding is REFUSED here, before any durable state exists, naming
	// the kit, the role and the roster key — never a worktree left to inherit an unrelated
	// identity. The resolver is the SAME one role-init and `deskwt add --role` use.
	idRole, ok := kitRole(o.kit)
	if !ok {
		return plan, deskkit.Refused(fmt.Sprintf(
			"step %s: --kit %q maps to no dispatched-agent identity role — this is a build defect (the kit "+
				"vocabulary is closed and was already validated), not a caller error.", stepWorktreeCreate, o.kit))
	}
	idName, idEmail, ierr := deskkit.RoleWorktreeCommitIdentity(idRole)
	if ierr != nil {
		return plan, deskkit.Refused(fmt.Sprintf(
			"step %s: --kit %s dispatches under the %s role, but that role has no commit identity in the roster "+
				"(%s), so the agent's worktree would INHERIT the dispatching desk's identity and misattribute every "+
				"Evidence Runner cell — refusing rather than stamping or inheriting a wrong identity. %v",
			stepWorktreeCreate, o.kit, idRole, deskkit.EnvTrustedBotSlugs, ierr))
	}
	plan.identityRole = idRole
	plan.identityName = idName
	plan.identityEmail = idEmail

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

	// WHERE the claim is kept is the resolver's answer (forge-neutral/21), read from the roster
	// and never from a flag. A configured store that cannot be used is a refusal HERE — exit 6,
	// before any child process, any worktree and any credential mint — and is never replaced by
	// another store. An unset key is the one-window legacy resolution, whose removal NOTICE is
	// printed on every run.
	store, serr := deskkit.ResolveClaimStore(repo)
	if serr != nil {
		return plan, deskkit.Unverifiable(fmt.Sprintf(
			"step %s: no dispatch claim can be taken for %s — the claim store did not resolve. Nothing was "+
				"claimed, cut or minted.", stepClaimAcquire, repo), serr)
	}
	plan.claimStore = store
	if store.Notice != "" {
		fmt.Fprintf(os.Stderr, "%s: %s\n", toolName, store.Notice)
	}

	// A REVIEW dispatch's prompt is forge-shaped: the reviewer fetches the change's HEAD from
	// the forge-specific server-side ref namespace (GitHub refs/pull/<N>/head ↔ GitLab
	// refs/merge-requests/<iid>/head). Resolve WHICH forge serves the target repo now, pre-claim,
	// so a GitHub-shaped fetch is never emitted into a GitLab reviewer's prompt (#773) and so a
	// repo whose forge cannot be determined refuses HERE, before any durable state exists, rather
	// than handing a reviewer a coordinate it cannot check out. A worker dispatch emits no
	// forge-shaped ref, so this is skipped for one and the worker path stays byte-for-byte
	// unchanged — including its executed-process count.
	if reviewKit(o.kit) {
		kind, ferr := o.resolveTargetForgeKind(repo)
		if ferr != nil {
			return plan, ferr
		}
		plan.forgeKind = kind
	}

	// A VERIFIER dispatch names no branch (#1309 item 6): its worktree is cut DETACHED off
	// origin/main under its own `verify-<item>` name and never touches the brief's feature
	// branch — a delivered brief's `feat/<id>` still sitting in a stale worker worktree used to
	// refuse the verifier with "already delivered or in progress", which is true of the brief
	// and irrelevant to a verify pass against merged main. An explicit --branch on a verifier
	// dispatch contradicts that shape and is refused here, pre-claim.
	if verifierKit(o.kit) {
		if strings.TrimSpace(o.branch) != "" {
			return plan, deskkit.Refused(fmt.Sprintf(
				"step %s: --branch is not accepted with --kit verifier — a verifier's worktree is cut DETACHED "+
					"off origin/main under its own name and never touches a feature branch.", stepWorktreeCreate))
		}
		plan.detached = true
	} else {
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
	}
	// The worktree DIR name is session-scoped so a FOREIGN session's leftover canonical dir
	// (`/private/tmp/tracker-<item>`) cannot dead-end an otherwise-valid dispatch with
	// `deskwt add … target already exists` (exit 5). deskdispatch used to derive the dir name
	// deterministically from the item key with no session-uniqueness, so any item whose
	// canonical dir was taken by another live/stale session (e.g. a role worktree) could not be
	// dispatched at all. The BRANCH and the CLAIM KEY stay deterministic — they are the
	// deliverable's cross-session identity — so only the local scratch dir gains the suffix,
	// mirroring `deskwt role-init`'s own `tracker-<prefix>-<sess>` naming. A session that does
	// not resolve to a safe single segment, or a suffix that would push the name past the
	// worktree-name grammar, falls back to the bare item-derived name (the pre-session
	// behaviour), so this never turns a usable name unusable.
	base := sanitizeSegment(o.item)
	if plan.detached {
		// The verifier's OWN name: a worker worktree for the same item (tracker-<item>-<sess>)
		// must never be the dir a verifier lands in or is refused by.
		base = "verify-" + base
	}
	if !worktreeNameRe.MatchString(base) {
		return plan, deskkit.Refused(fmt.Sprintf(
			"step %s: the item key %q does not reduce to a usable worktree name — pass --branch and a key "+
				"that does.", stepWorktreeCreate, o.item))
	}
	plan.wtName = base
	if sess := dispatchSessionSuffix(); sess != "" {
		if scoped := base + "-" + sess; worktreeNameRe.MatchString(scoped) {
			plan.wtName = scoped
		}
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
	plan.decisionScript = filepath.Join(scriptsRoot, filepath.FromSlash(decisionScriptRel))

	// The claim TOOL: prefer the pure-Go goClaimBinary when it resolves on PATH (it is
	// installed with desk-tools, takes the role credential as a --token-file flag, and runs
	// with no shell on every host), else fall back to the legacy tools/dispatch-claim.sh
	// when the resolved root carries it (a consumer whose tree predates the binary; the two
	// speak the same wire protocol). Only when NEITHER is available is this a fail-closed
	// refusal — a claim this verb cannot place is not permission to proceed. Resolved BEFORE
	// the claim, from disk/PATH alone, so it costs nothing durable and keeps the "no state
	// before this returns" invariant total. Issue 1151 inverted the order: the script used to
	// win when both resolved, which kept the ambient-`gh` claim path in use on every tree that
	// still carried the script.
	claimScriptPath := filepath.Join(scriptsRoot, filepath.FromSlash(claimScriptRel))
	_, goErr := lookPath(goClaimBinary)
	switch {
	case goErr == nil:
		plan.claimTool = goClaimBinary
		plan.claimToolIsScript = false
		plan.claimReleaseHint = goClaimBinary
	case fileExists(claimScriptPath):
		plan.claimTool = claimScriptPath
		plan.claimToolIsScript = true
		if strings.TrimSpace(o.claimRoot) != "" {
			plan.claimReleaseHint = claimScriptPath // the worktree does not carry it — state the path
		} else {
			plan.claimReleaseHint = claimScriptRel // in the agent's own worktree — stable relative spelling
		}
	default:
		return plan, deskkit.Unverifiable(fmt.Sprintf(
			"step %s: no claim tool is available — the pure-Go %s binary is not on PATH and %s is not "+
				"present in %s, so no durable claim can be taken. A claim this verb cannot place is NOT "+
				"permission to proceed: a machine-local lock would serialise two dispatchers on one machine "+
				"and nothing at all across two, which is the case that double-dispatches%s.",
			stepClaimAcquire, goClaimBinary, claimScriptRel, scriptsRoot, claimRootHint(o)), goErr)
	}

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

	// The operator-supplied home worktree (--dry-run only; the flag pairing was refused above
	// otherwise). All three checks fail closed with their own reason. On success plan.home
	// carries the RESOLVED path both placeholder sites render against.
	if strings.TrimSpace(o.worktree) != "" {
		home, err := validateOperatorWorktree(o.root, o.worktree)
		if err != nil {
			return plan, err
		}
		plan.home = home
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
// Every child here — acquire, and the show that qualifies an exit 5 — runs under the
// credential resolveClaimAuth handed over (auth), never the ambient one.
func stepClaim(o dispatchOpts, repo, script string, isScript bool, auth claimAuth, claimKey string) error {
	if err := claimToolAvailable(script, isScript); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the claim tool %s is not available, so no durable claim can be taken. A claim this verb "+
				"cannot place is NOT permission to proceed — a machine-local lock would serialise two "+
				"dispatchers on one machine and nothing at all across two, which is the case that "+
				"double-dispatches.", stepClaimAcquire, script), err)
	}
	args := []string{"acquire", claimKey, "--repo", repo}
	if o.branch != "" {
		args = append(args, "--branch", o.branch)
	}
	r := runCmdEnv(o.root, auth.env, script, append(args, auth.args...)...)
	if r.err == nil {
		return nil
	}
	switch exitCodeOf(r.err) {
	case deskkit.ExitRefused:
		show := runCmdEnv(o.root, auth.env, script, append([]string{"show", claimKey, "--repo", repo}, auth.args...)...)
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
			stepClaimAcquire, claimKey, r.run.Said(), holder))
	default:
		return r.run.FailVerbatim(deskkit.ExitUnverifiable, fmt.Sprintf(
			"step %s: the claim on %s could not be established (%s) — fail closed, NEVER 'assume free'.",
			stepClaimAcquire, claimKey, r.run.Said()))
	}
}

// fileExists reports whether path names an existing file (or directory). Used to decide
// whether the resolved root carries the legacy claim script before falling back to the Go
// binary.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// claimToolAvailable is stepClaim's presence backstop, matching how the tool was resolved:
// a legacy script is checked on disk (os.Stat), the Go binary on PATH (lookPath). It repeats
// the resolution's finding so the step that must not proceed without a claim tool cannot be
// reached with one that has vanished since validation.
func claimToolAvailable(tool string, isScript bool) error {
	if isScript {
		if _, err := os.Stat(tool); err != nil {
			return err
		}
		return nil
	}
	_, err := lookPath(tool)
	return err
}

// goos is the host-OS seam, mirroring cmd/deskwt's own `goos` var so the PRODUCER (deskwt,
// which selects the worktree prefix) and this CONSUMER (which accepts the home it printed)
// judge "absolute" the same way and cannot disagree again — the #757 defect, the consumer
// half of the #727/#732 Windows-portability family. Production reads runtime.GOOS; white-box
// tests set it to exercise the Windows codepath on a POSIX runner. Genuine drive-root
// resolution is a compile-time property of path/filepath and cannot otherwise be reproduced
// off-Windows, exactly as cmd/deskwt/windowsprefix_test.go documents for the producer half.
var goos = runtime.GOOS

// homeIsAbsolute reports whether the worktree home deskwt reported is an absolute path,
// using filepath.IsAbs — the portable test brief.go already uses, and the one that on a
// native-Windows build accepts the drive-rooted <repo-root>\.claude\worktrees\... home
// deskwt selects there. The `goos == "windows"` arm exists ONLY for the seam above: on a
// POSIX test binary filepath.IsAbs cannot see a `C:\...` path as absolute, so this lets the
// consumer half of the contract be pinned and shown red-first on the POSIX CI runner. On a
// real Windows build filepath.IsAbs already answers, so windowsAbs is never reached there.
func homeIsAbsolute(home string) bool {
	if filepath.IsAbs(home) {
		return true
	}
	return goos == "windows" && windowsAbs(home)
}

// windowsAbs recognises the two Windows absolute-path forms deskwt can emit — a drive-rooted
// path (`C:\...` or `C:/...`) and a UNC path (`\\host\share\...`). It serves the goos seam in
// homeIsAbsolute only; a real Windows build never consults it (filepath.IsAbs answers first).
func windowsAbs(p string) bool {
	if strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, `//`) { // UNC
		return true
	}
	if len(p) < 3 || p[1] != ':' || (p[2] != '\\' && p[2] != '/') {
		return false
	}
	return (p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z') // <drive>:\ or <drive>:/
}

// releaseClaim releases the durable claim via the consumer claim script's own `release`
// verb — the same tool the acquire went through, never a re-implementation. It is used when
// a lifecycle failure AFTER the claim (a failed before_run hook) must not wedge the item
// behind a dispatcher that never dispatched. It returns a human phrase for the report; a
// release that itself fails is surfaced in that phrase rather than swallowed, because a
// claim this verb believed it released but did not is worse than one it never touched.
func releaseClaim(o dispatchOpts, script string, auth claimAuth, claimKey, repo string) string {
	r := runCmdEnv(o.root, auth.env, script, append([]string{"release", claimKey, "--repo", repo}, auth.args...)...)
	if r.err != nil {
		return "NOT released (release failed: " + r.run.Said() + ") — release it by hand: " +
			script + " release " + claimKey + " --repo " + repo
	}
	return "released"
}

// claimAuth is the credential hand-off for every claim-tool child this verb starts (acquire,
// show, release) and, via scriptEnv, for the decision-gate script (issue 1146). Exactly one of
// the two claim carriers is populated per tool, matching how each tool reads its credential:
// the Go binary takes `--token-file <0600 path>` (args), the legacy script reads GH_TOKEN from
// its environment (env). A zero claimAuth means the claim child inherits
// the calling environment unchanged — the explicit-GH_TOKEN case, where the operator's export
// already IS the credential and both tools read it as-is.
//
// The token VALUE never appears in a message: source names the explicit export or the role and
// token-file PATH, which is what an operator needs and is safe to print.
//
// SCRIPT CHILDREN (issue 1146). The claim tool is not the only child that writes to the forge:
// the decision gate runs the consumer's tools/decision-issue.sh, a script that shells out to
// the forge CLI itself. A child named anything but the CLI never matched a "hand the token to
// `gh`" rule, so it ran on whatever login was ambient. scriptEnv is the ENVIRONMENT-shaped
// hand-over of the SAME credential — populated whatever shape the claim tool takes it in — so
// every script child this verb starts runs as the dispatching role, from ONE resolution (one
// mint, one identity for the claim and the decision issue alike). nil means the explicit
// GH_TOKEN export (inherited as-is) — or, on a zero claimAuth, that nothing was resolved,
// which stepDecision refuses rather than running the script on ambient auth.
type claimAuth struct {
	env    []string // non-nil REPLACES the child's environment (os/exec contract); nil inherits
	args   []string // appended after the verb's own positionals and flags
	source string   // for the claim-acquire report line

	scriptEnv    []string // env for a forge-CLI-shelling SCRIPT child (the decision gate); nil inherits
	scriptSource string   // for that child's report line
}

// resolveClaimAuth obtains the credential the claim child runs under. Issue 1151: the claim
// tools read only the AMBIENT credential — bare `gh api` in the script, GH_TOKEN/--token-file
// in the binary — so a dispatch from a sandboxed desk window, which holds no ambient `gh`
// login, failed closed at claim-acquire on every fresh item while every other write verb
// minted its own role token and succeeded. The mint is the same role token the stamp step
// attests under (the dispatching role stampRoleForKit names), taken here through mintTokenFn
// because the claim runs four steps before the stamp — which reads its own credential inside
// deskkit.ResolveForge.
//
// PRECEDENCE. An explicit GH_TOKEN already in the environment wins outright and nothing is
// minted: an operator who exported a credential chose it, and both tools already read it —
// the Go binary via its env fallback (no --token-file is passed, because that flag would
// outrank the export inside the binary). Otherwise the role token is minted and handed over
// in the tool's own shape: `--token-file <path>` for the binary (the minter's 0600 cache file,
// never a copy written anywhere new) and GH_TOKEN in the child environment for the script.
//
// FAIL CLOSED. A mint failure is UNVERIFIABLE and stops the dispatch before any claim child
// runs. The alternative — letting the child fall back to whatever `gh` is logged in as — is
// the field defect: nothing (401) in a sandboxed window, or a human OAuth login with no write
// on the target (404), and in the worst case a claim ref minted under a human's identity.
//
// FORGE (issue 1203). The GitHub App mint above is a GitHub-only custody. On a repo whose
// forge resolves to GitLab (ASSAY_REPO_FORGES=<slug>=gitlab) there is no App to mint against,
// and requiring a GitHub App ID there failed closed at claim-acquire even though deskboot had
// already cached the GitLab role PAT and every other write verb (deskpost/deskflip via
// deskkit.ResolveForge) uses it. So the claim child's credential FOLLOWS the resolved forge:
// GitLab-resolved repos hand the child the same role PAT custody the other verbs use, GitHub
// keeps the App mint unchanged. The explicit-GH_TOKEN override and the fail-closed direction
// hold for both — a forge that cannot be resolved refuses here, before any claim, rather than
// guessing.
func resolveClaimAuth(o dispatchOpts, repo string, forgeKind deskkit.ForgeKind, isScript bool) (claimAuth, error) {
	if strings.TrimSpace(os.Getenv("GH_TOKEN")) != "" {
		const explicit = "the GH_TOKEN already exported in this environment"
		return claimAuth{source: explicit, scriptSource: explicit}, nil
	}
	role := stampRoleForKit(o.kit)
	// forgeKind is pre-resolved by planClaim for a review dispatch (its prompt is forge-shaped);
	// a worker dispatch leaves it empty, so resolve it HERE, at execution time — after every
	// caller-flag precondition has passed, so a flag-mistake refusal never pays for this git
	// read (the "nothing durable before the flags are good" contract the zero-process tests pin).
	kind := forgeKind
	if strings.TrimSpace(string(kind)) == "" {
		var ferr error
		kind, ferr = o.resolveTargetForgeKind(repo)
		if ferr != nil {
			return claimAuth{}, ferr
		}
	}
	if kind == deskkit.ForgeGitLab {
		return resolveClaimAuthGitLab(role, repo, isScript)
	}
	tok, tokPath, err := mintTokenFn(role, repo)
	if err != nil {
		return claimAuth{}, deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the %s App installation token for %s could not be minted or read (%s): %v — so the "+
				"identity the claim would be taken under cannot be established. NO claim was attempted: the "+
				"claim tool is never run on the ambient `gh` credential (nothing at all in a sandboxed desk "+
				"window, or a human login with no write on the target — the two ways this step failed before "+
				"it minted its own token). Export GH_TOKEN to override the mint deliberately.",
			stepClaimAcquire, role, deskkit.OwnerOf(repo), tokenPathForMessage(tokPath), err), err)
	}
	scriptEnv := append(os.Environ(), "GH_TOKEN="+tok)
	scriptSource := fmt.Sprintf("the %s App token (GH_TOKEN in the child environment, %s)", role, tokenPathForMessage(tokPath))
	if isScript {
		return claimAuth{env: scriptEnv, source: scriptSource, scriptEnv: scriptEnv, scriptSource: scriptSource}, nil
	}
	if strings.TrimSpace(tokPath) == "" {
		return claimAuth{}, deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the %s App installation token for %s was minted but the minter named no token file, "+
				"so it cannot be handed to %s as --token-file. NO claim was attempted.",
			stepClaimAcquire, role, deskkit.OwnerOf(repo), goClaimBinary), nil)
	}
	return claimAuth{
		args:         []string{"--token-file", tokPath},
		source:       fmt.Sprintf("the %s App token (--token-file, %s)", role, tokenPathForMessage(tokPath)),
		scriptEnv:    scriptEnv,
		scriptSource: scriptSource,
	}, nil
}

// resolveClaimAuthGitLab is resolveClaimAuth's GitLab branch: it reads the role's already-
// provisioned GitLab PAT custody file (deskkit.GitLabRoleToken — the same custody
// deskpost/deskflip act under, NEVER the GitHub App minter) and hands it to the claim child in
// the tool's own shape. The Go binary (deskclaim-ref) resolves the forge itself and picks the
// GitLab git-basic-auth username (oauth2), so it needs only the right token file via
// --token-file; the legacy script reads GH_TOKEN/GITLAB_TOKEN from its environment, so both are
// set. A missing/loose/empty custody file is Refused (exit 5) — a precondition an operator
// fixes — and stops the dispatch before any claim child runs, the same fail-closed direction
// the GitHub mint takes. The token VALUE never appears in a message; the PATH and role do.
func resolveClaimAuthGitLab(role, repo string, isScript bool) (claimAuth, error) {
	tok, tokPath, err := deskkit.GitLabRoleToken(role)
	if err != nil {
		return claimAuth{}, deskkit.RefusedWithCause(fmt.Sprintf(
			"step %s: the %s GitLab role PAT for %s could not be read (%s) — so the identity the claim "+
				"would be taken under cannot be established. NO claim was attempted: the claim tool is never "+
				"run on the ambient credential. Provision the role's GitLab PAT custody file, or export "+
				"GH_TOKEN to override deliberately.",
			stepClaimAcquire, role, deskkit.OwnerOf(repo), err), err)
	}
	scriptEnv := append(os.Environ(), "GH_TOKEN="+tok, "GITLAB_TOKEN="+tok)
	scriptSource := fmt.Sprintf("the %s GitLab role PAT (GH_TOKEN/GITLAB_TOKEN in the child environment, %s)", role, tokenPathForMessage(tokPath))
	if isScript {
		return claimAuth{env: scriptEnv, source: scriptSource, scriptEnv: scriptEnv, scriptSource: scriptSource}, nil
	}
	if strings.TrimSpace(tokPath) == "" {
		return claimAuth{}, deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the %s GitLab role PAT for %s was read but named no custody file, so it cannot be "+
				"handed to %s as --token-file. NO claim was attempted.",
			stepClaimAcquire, role, deskkit.OwnerOf(repo), goClaimBinary), nil)
	}
	return claimAuth{
		args:         []string{"--token-file", tokPath},
		source:       fmt.Sprintf("the %s GitLab role PAT (--token-file, %s)", role, tokenPathForMessage(tokPath)),
		scriptEnv:    scriptEnv,
		scriptSource: scriptSource,
	}, nil
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
			"`deskroster list` will not show this work until the agent registers it", short, o.pr, r.run.Said())
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
//
// THE SCRIPT RUNS AS THE DISPATCHING ROLE, NEVER ON AMBIENT AUTH (issue 1146). The decision
// script files a forge issue by shelling out to the forge CLI itself, so it authenticates from
// its environment. It is handed auth.scriptEnv — the environment-shaped form of the credential
// the claim step resolved (resolveClaimAuth, before the claim: a mint failure has already
// stopped the dispatch with nothing durable taken). With no handed-over environment AND no
// explicit GH_TOKEN export, this REFUSES rather than start the script: an unresolved credential
// reaching here is a wiring defect, and running the script anyway would file the decision issue
// under whatever login the calling shell holds — the gap this step exists not to have.
func stepDecision(o dispatchOpts, gateHuman bool, repo, script string, auth claimAuth) (string, error) {
	if !gateHuman {
		return "SKIPPED: item is not human-gated", nil
	}
	if auth.scriptEnv == nil && strings.TrimSpace(os.Getenv("GH_TOKEN")) == "" {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: no credential was handed to %s and no GH_TOKEN is exported, so the decision issue for %s "+
				"would be filed under the AMBIENT forge login. NOT run: this verb never starts a forge-writing "+
				"script on ambient auth. Export GH_TOKEN to choose the identity deliberately.",
			stepDecisionGate, script, o.brief), nil)
	}
	r := runCmdEnv(o.root, auth.scriptEnv, script, "ensure", o.brief, "--repo", repo, "--at", "start")
	if r.err != nil {
		return "", r.run.FailVerbatim(deskkit.ExitUnverifiable, fmt.Sprintf(
			"step %s: the decision-issue gate for %s could not be ensured (%s) — a possible duplicate is the "+
				"cheap direction and a missing gate is the expensive one, so this fails closed.",
			stepDecisionGate, o.brief, r.run.Said()))
	}
	return "OK: " + firstLine(r.stdout) + ", authenticated by " + auth.scriptSource, nil
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
	// The identity comes FIRST, before any label is written. The role is the one deskkit
	// declares as the dispatcher — the same declaration the floor's reader resolves the
	// accepted applier login from — so the identity a stamp is WRITTEN under and the
	// identity it is VERIFIED against cannot drift apart. Writing the labels under the
	// calling session's own credential is what made a correctly dispatched strong-tier PR
	// read as a forged self-report and refuse every verdict and ready-flip on it.
	//
	// A credential the resolver cannot read is UNVERIFIABLE and stops the step: the
	// alternative — stamping under the ambient credential — produces an attestation the floor
	// must refuse, and a PR carrying an untrusted stamp is in a WORSE state than an unstamped
	// one (absent reads UNKNOWN and proceeds with a NOTICE). So no stamp at all is the safe
	// failure here.
	//
	// THE LANE'S OWN DISPATCHER. The role resolved here is the role that DISPATCHED this
	// session, which is not always the desk App: the review lane is dispatched by the
	// reviewer App, and stamping a review dispatch under the desk App would re-open the
	// dispatcher/applier split from the other side — an attestation written by an identity
	// that did not launch the session. deskkit accepts both roles (DispatcherRoles) precisely
	// so each lane's own dispatcher can attest for it, and stampRole is where that choice is
	// made once for every stamp this verb writes.
	//
	// THE FORGE IS RESOLVED, NEVER CHOSEN. ResolveForge answers GitHub or GitLab from the
	// roster/remote and reads the lane's own dispatcher credential for whichever it is (a
	// GitHub App installation token, a GitLab PAT). Every read and write below goes through
	// that one backend — the same seam the queue-label step uses — so a GitLab-served project
	// is stamped by the same code path a GitHub one is. Before #1154 this step read and wrote
	// labels through the GitHub CLI, so a stamped review dispatch on a GitLab project failed
	// closed here and the forge-neutral queue label (step 6) was never reached.
	stampRole := stampRoleForKit(o.kit)
	fr, ferr := forgeRepoOf(repo)
	if ferr != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf("step %s: %v", stepModelStamp, ferr), ferr)
	}
	fg, res, rerr := deskkit.ResolveForge(fr, stampRole)
	if rerr != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the forge serving %s could not be resolved under the %s role, or that role's credential "+
				"could not be read (%s) — so the identity the stamp would be applied under cannot be "+
				"established. NO label was applied: a stamp written under this session's own credential reads "+
				"as a non-dispatcher stamp and refuses every authority-bearing write on the PR, which is worse "+
				"than leaving it unstamped.",
			stepModelStamp, repo, stampRole, firstLine(rerr.Error())), rerr)
	}
	forge := string(res.Kind)

	// RE-STAMP, NOT ADD-ON-TOP. Labels are a SET, so applying a label the PR already carries
	// is a NO-OP — and a stamp the floor cannot read is exactly the state it refuses.
	// Re-running this step therefore changed nothing on the PRs that needed it most. A
	// forge's label history is APPEND-ONLY, so the only repair it offers is to REMOVE the
	// offending labels and re-apply the intended pair under the dispatcher; that is what the
	// floor's reader resolves (the actor of the last standing application event), so it is
	// what this step must do. The set to remove comes from deskkit.ReStampRemovals(labels):
	// every present dispatched-* label that is not part of the pair being applied (a
	// conflicting, stale, or malformed stamp — INCLUDING one an earlier run of the dispatcher
	// itself left), plus any half of the pair whose standing application is foreign. Clearing
	// only the foreign labels (the old ForeignStampLabels set) left a dispatcher-applied
	// conflicting label standing, so the re-dispatch reported OK while the floor kept
	// refusing — the present-but-unreadable deadlock this recovery exists to break. Reader
	// and writer project the standing-applier resolution from one place — and read it through
	// the same two enumerated operations (GetPullRequest for the present set, ListLabelEvents
	// for who applied each) — so they cannot disagree about it.
	//
	// The reads are UNVERIFIABLE on failure rather than best-effort: proceeding blind would
	// silently re-create the no-op — the labels would be "applied" and the PR would still
	// carry the foreign stamp, which is the failure this whole step exists to prevent.
	change, perr := fg.GetPullRequest(fr, o.pr)
	if perr != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: could not read the labels currently on %s#%d through the %s backend (%s) — whether "+
				"this PR already carries a foreign stamp is unknown, and adding a label over one is a no-op, "+
				"so stamping blind would report success on a PR that stays refused.",
			stepModelStamp, repo, o.pr, forge, firstLine(perr.Error())), perr)
	}
	events, eerr := fg.ListLabelEvents(fr, o.pr)
	if eerr != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: could not read the label history of %s#%d through the %s backend (%s) — WHO applied "+
				"the stamp this PR carries cannot be established, so a foreign application could not be "+
				"replaced.",
			stepModelStamp, repo, o.pr, forge, firstLine(eerr.Error())), eerr)
	}
	tl := deskkit.StampTimeline{Present: change.Labels, Events: events}
	stale := deskkit.ReStampRemovals(tl, labels, deskkit.IsDispatcherLogin)
	if len(stale) == 0 && labelsPresent(change.Labels, labels) {
		// An IDENTICAL stamp already standing under the dispatcher is a no-op: nothing is
		// removed and nothing is re-applied, so a re-dispatch neither churns the label
		// history nor leaves the PR briefly unstamped.
		return fmt.Sprintf("OK: %s already standing on %s#%d under the %s App, the identity the capability "+
			"floor accepts (%s) — an identical stamp is a no-op, nothing was written",
			strings.Join(labels, " + "), repo, o.pr, stampRole, forge), nil
	}
	if len(stale) > 0 {
		// The removal is its OWN write, ahead of the application, on purpose: a label named in
		// both halves of one LabelChange is skipped by the backends as a caller bug, and a
		// single reconciliation that removed and re-added the same name would leave the forge's
		// label set unchanged — no new application event, so the standing applier would not
		// change and the PR would stay refused. Two writes are two events.
		if _, aerr := fg.ApplyLabels(fr, o.pr, deskkit.LabelChange{
			Target: deskkit.TargetChange,
			Remove: stale,
		}); aerr != nil {
			return "", deskkit.Unverifiable(fmt.Sprintf(
				"step %s: could not remove the stamp label(s) %s from %s#%d through the %s backend (%s) — "+
					"re-applying the intended stamp on top would be a no-op, leaving the PR carrying labels "+
					"the floor refuses.",
				stepModelStamp, strings.Join(stale, " + "), repo, o.pr, forge, firstLine(aerr.Error())), aerr)
		}
	}

	// Both halves in ONE reconciliation: the backend ensures each label exists (an
	// already-exists is the success case, so two dispatchers stamping in parallel both end up
	// with the label present) and applies the pair in one request, so the stamp can never land
	// half-applied on a forge that writes the set atomically.
	add := make([]deskkit.LabelSpec, 0, len(labels))
	for _, l := range labels {
		add = append(add, deskkit.LabelSpec{Name: l, Color: stampLabelColorHex, Description: stampLabelDescription})
	}
	if _, aerr := fg.ApplyLabels(fr, o.pr, deskkit.LabelChange{
		Target: deskkit.TargetChange,
		Add:    add,
	}); aerr != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: could not apply %s to %s#%d through the %s backend (%s) — an INCOMPLETE or absent "+
				"stamp reads as indeterminate, and one half of a stamp is worse than no stamp at all.",
			stepModelStamp, strings.Join(labels, " + "), repo, o.pr, forge, firstLine(aerr.Error())), aerr)
	}
	// BOTH events are reported. A silent removal is a label disappearing from a PR with no
	// record of why; the removal is half the repair and belongs in the step report next to
	// the application it made possible.
	restamped := ""
	if len(stale) > 0 {
		restamped = fmt.Sprintf(" (RE-STAMPED: removed %s — a conflicting, stale, or foreign-applied "+
			"stamp the floor cannot read — before applying the intended stamp, since adding over a "+
			"present label is a no-op)", strings.Join(stale, " + "))
	}
	return fmt.Sprintf("OK: applied %s to %s#%d as the %s App, the identity the capability floor accepts (%s)%s",
		strings.Join(labels, " + "), repo, o.pr, stampRole, forge, restamped), nil
}

// labelsPresent reports whether every wanted label is already on the change, by the same
// case-insensitive comparison the stamp reader uses for label names.
func labelsPresent(present, want []string) bool {
	for _, w := range want {
		found := false
		for _, p := range present {
			if strings.EqualFold(strings.TrimSpace(p), strings.TrimSpace(w)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// forgeRepoOf splits an owner/name slug into the coordinate the Forge seam addresses.
func forgeRepoOf(repo string) (deskkit.ForgeRepo, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return deskkit.ForgeRepo{}, fmt.Errorf("repo %q does not split into owner/name", repo)
	}
	return deskkit.ForgeRepo{Owner: owner, Name: name}, nil
}

// applyQueueLabelFn applies the review-lane queue label to a picked-up change. It is a SEAM
// so a full-run dispatch test drives the step without real forge credentials or network. The default resolves the forge under the reviewer role and calls
// the forge's idempotent label ensure+apply — the SAME forge-neutral path deskflip's
// ensureLabelSwap uses, so GitHub and GitLab are labelled by one code path.
var applyQueueLabelFn = applyQueueLabelReal

// applyQueueLabelReal resolves the forge serving repo under the reviewer role and applies
// queueLabelAuthorizationNeeded to the change, creating the label first if the repo does not
// carry it (Forge.ApplyLabels' ensure step is idempotent — an already-exists is the success
// case). It returns the labels actually added (empty when the label was already present) and
// the resolved forge kind for the step report. Credential custody (GitHub App token / GitLab
// PAT) is resolved inside ResolveForge; this never touches the deskpost verdict-write path.
func applyQueueLabelReal(repo string, pr int) (added []string, forge string, err error) {
	fr, ferr := forgeRepoOf(repo)
	if ferr != nil {
		return nil, "", ferr
	}
	fg, res, rerr := deskkit.ResolveForge(fr, deskkit.ReviewDispatcherRole)
	if rerr != nil {
		return nil, "", rerr
	}
	out, aerr := fg.ApplyLabels(fr, pr, deskkit.LabelChange{
		Target: deskkit.TargetChange,
		Add: []deskkit.LabelSpec{{
			Name:        queueLabelAuthorizationNeeded,
			Color:       queueLabelColorHex,
			Description: "Queue: this change is in the review lane and still needs the reviewer's work before it is ready for a human",
		}},
	})
	if aerr != nil {
		return nil, string(res.Kind), aerr
	}
	if out != nil {
		added = out.Added
	}
	return added, string(res.Kind), nil
}

// stepQueueLabelApply applies the review-lane queue label when a reviewer is dispatched onto
// a known change, and is a documented no-op otherwise.
//
//   - Not a review dispatch → SKIPPED: the queue label is a review-lane signal only.
//   - Review dispatch with no --pr → DEFERRED: there is no change to label yet.
//   - Review dispatch with --pr → apply authorization-needed, idempotently and NON-FATALLY.
//
// It never returns an error: a queue label is a legibility aid, not a correctness gate (the
// claim already serialises the dispatch), so a forge/credential failure is a loud WARNING and
// the dispatch continues — the same contract as stepRoster and deskflip's ensureLabelSwap.
func stepQueueLabelApply(o dispatchOpts, repo string) string {
	if !reviewKit(o.kit) {
		kind := strings.TrimSpace(o.kit)
		if kind == "" {
			kind = "worker"
		}
		return "SKIPPED: authorization-needed is a review-lane signal; this is a " + kind + " dispatch"
	}
	if o.pr <= 0 {
		return "DEFERRED: no --pr — there is no change to label yet; authorization-needed is " +
			"applied when a reviewer is dispatched onto a known MR/PR"
	}
	added, forge, err := applyQueueLabelFn(repo, o.pr)
	if err != nil {
		return fmt.Sprintf("WARNING: could not apply %s on %s#%d (%s) — the queue label is a "+
			"provisioning/credential gap to file; the review dispatch stands (the label is a "+
			"legibility aid, not a claim on the change)",
			queueLabelAuthorizationNeeded, repo, o.pr, firstLine(err.Error()))
	}
	if len(added) == 0 {
		return fmt.Sprintf("OK: %s already present on %s#%d (%s)",
			queueLabelAuthorizationNeeded, repo, o.pr, forge)
	}
	return fmt.Sprintf("OK: applied %s on %s#%d (%s)",
		queueLabelAuthorizationNeeded, repo, o.pr, forge)
}

// stampRoleForKit names the App identity a dispatch of this kit must stamp under: the role
// that actually DISPATCHED the session.
//
// The review lane is dispatched by the reviewer App (pr-review-desk drives its own reviewers),
// every other lane by the desk App. The floor accepts both, and only these two — so this
// choice is the writer's half of the same one-list rule deskkit.DispatcherRoles holds for the
// reader: a lane whose dispatcher is not in that set has no way to mint a trusted stamp, which
// is the fail-closed direction.
func stampRoleForKit(kit string) string {
	if reviewKit(kit) {
		return deskkit.ReviewDispatcherRole
	}
	return deskkit.DispatcherRole
}

// tokenPathForMessage renders the token file path for a step report or refusal. The PATH is
// what an operator needs and is safe to print; the token VALUE never is, and never reaches
// a message from anywhere in this verb. The claim step (resolveClaimAuth) is its caller.
func tokenPathForMessage(path string) string {
	if strings.TrimSpace(path) == "" {
		return "the minter named no token path"
	}
	return "token file " + path
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

// dispatchSessionSuffix returns the session id used to disambiguate the worktree DIR name,
// or "" when none resolves to a safe single segment. It reads $DESK_SESSION then
// $CLAUDE_SESSION_ID — the same order `deskwt role-init` resolves a session — so the worker
// dispatch and the role worktrees share one notion of "which session". A value outside the
// worktree-name grammar is DROPPED (returns "") rather than sanitized into a different
// session's spelling: a wrong suffix would be worse than none.
func dispatchSessionSuffix() string {
	for _, env := range []string{"DESK_SESSION", "CLAUDE_SESSION_ID"} {
		if s := strings.TrimSpace(os.Getenv(env)); s != "" && worktreeNameRe.MatchString(s) {
			return s
		}
	}
	return ""
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
		return "", r.run.FailVerbatim(deskkit.ExitUnverifiable, fmt.Sprintf(
			"step %s: cannot read origin's URL in %s (%s) — pass --repo <owner/name>. An item dispatched "+
				"into the wrong repo is work nobody asked that repo for.",
			stepClaimAcquire, o.root, r.run.Said()))
	}
	slug := repoSlugFromURL(r.stdout)
	if slug == "" {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: origin %q does not parse to an owner/name — pass --repo <owner/name>.",
			stepClaimAcquire, r.stdout), nil)
	}
	return slug, nil
}

// resolveTargetForgeKind resolves WHICH forge serves the target repo, for the two places
// this verb branches on the answer: the review kit's forge-shaped head-fetch refspec, and the
// claim child's credential custody (GitHub App mint vs GitLab role PAT — issue 1203). It reads
// the TARGET repo's origin remote from o.root — the same checkout resolveRepo reads — through
// the runCmd seam, and hands the raw URL to deskkit, which owns the roster-first/host-map
// resolution and the well-known host table; deskdispatch re-derives neither. A repo the
// roster configures (ASSAY_REPO_FORGES) resolves even with an unreadable remote; otherwise
// the origin host decides. Unresolvable is deskkit's own could-not-check (exit 6), returned
// verbatim so the dispatch fails closed rather than guessing a forge.
func (o dispatchOpts) resolveTargetForgeKind(repo string) (deskkit.ForgeKind, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || strings.TrimSpace(owner) == "" || strings.TrimSpace(name) == "" {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: %q does not parse to an owner/name, so the forge serving it cannot be resolved.",
			stepClaimAcquire, repo), nil)
	}
	originURL := ""
	if r := runCmd(o.root, "git", "remote", "get-url", "origin"); r.err == nil {
		originURL = r.stdout
	}
	res, err := deskkit.ForgeKindForRepoRemote(deskkit.ForgeRepo{Owner: owner, Name: name}, originURL)
	if err != nil {
		return "", err
	}
	return res.Kind, nil
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
