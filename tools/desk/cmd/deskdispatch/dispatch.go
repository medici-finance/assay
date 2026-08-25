package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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

// claimScriptRel / decisionScriptRel are the CONSUMER-repo scripts this verb wraps. They
// are named by path and invoked; their content is never carried here. See main.go's
// "WRAP, NEVER RE-IMPLEMENT".
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

type dispatchOpts struct {
	item       string
	tier       string
	kit        string
	repo       string
	root       string
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
		item: item, tier: *tier, kit: *kit, repo: *repo, root: *root, model: *model,
		branch: *branch, brief: *brief, gateHuman: *gateHuman, pr: *pr,
		promptFile: *promptFile, quiet: *quiet, dryRun: *dryRun,
	}
	err := dispatch(o)
	audit(o, err)
	return err
}

func dispatch(o dispatchOpts) error {
	if !itemKeyRe.MatchString(o.item) {
		return deskkit.Refused(fmt.Sprintf(
			"step %s: %q is not a usable claim key (letters, digits, dot, dash, underscore, slash; no "+
				"leading dash). The key is passed through unchanged to the repo's claim tool, so a key this "+
				"verb would have to reshape is one that would not collide with the key another desk holds — "+
				"and a claim that does not collide is not a claim.",
			stepClaimAcquire, o.item))
	}
	// The tier vocabulary is CLOSED and is not a second list: it is derived from the
	// dispatch-tier set the stamp reader validates against.
	if !validTier(o.tier) {
		return deskkit.Refused(fmt.Sprintf(
			"step %s: --tier %q is outside the tier vocabulary (%s). The tier is an attestation of what was "+
				"launched, so an unrecognised value must not be recorded as though it meant something.",
			stepModelStamp, o.tier, strings.Join(deskkit.DispatchTiers(), "|")))
	}
	if _, err := kitText(o.kit); err != nil {
		return err
	}

	repo, err := o.resolveRepo()
	if err != nil {
		return err
	}
	if !deskkit.IsAllowedRepo(repo) {
		return deskkit.Refused(fmt.Sprintf(
			"step %s: %s is not in the desk repo set — this verb dispatches work only into repos the desk "+
				"is rostered to act on.", stepClaimAcquire, repo))
	}

	branch := o.branch
	if branch == "" {
		branch = "feat/" + sanitizeSegment(o.item)
	}
	wtName := sanitizeSegment(o.item)
	if !worktreeNameRe.MatchString(wtName) {
		return deskkit.Refused(fmt.Sprintf(
			"step %s: the item key %q does not reduce to a usable worktree name — pass --branch and a key "+
				"that does.", stepWorktreeCreate, o.item))
	}

	if o.dryRun {
		fmt.Printf("deskdispatch: PLAN (dry run — nothing touched) item=%s repo=%s tier=%s kit=%s branch=%s\n",
			o.item, repo, o.tier, o.kit, branch)
		for i, s := range dispatchSteps {
			fmt.Printf("  %d %s\n", i+1, s)
		}
		prompt, perr := assemblePrompt(o, repo, branch, "")
		if perr != nil {
			return perr
		}
		return emitPrompt(o, prompt)
	}

	// 1 — the durable claim, FIRST. Everything after this is work a second dispatcher
	// must not also be doing.
	if err := stepClaim(o, repo); err != nil {
		return err
	}
	o.say("%s OK: %s claimed in %s", stepClaimAcquire, o.item, repo)

	// 2 — the agent's worktree, in the ITEM's repo. deskwt owns the safety here (a
	// sanctioned path prefix, an unambiguous base, no clobber of an existing target), so
	// this step delegates rather than re-deriving any of it — INCLUDING where the
	// worktree lands: the path the prompt names is the one deskwt printed, never one this
	// verb predicted.
	wt := runCmd(o.root, "deskwt", "add", wtName, "--branch", branch, "--base", "refs/remotes/origin/main")
	if wt.err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: `deskwt add %s` failed in %s (%s). The claim is HELD — release it, or fix the tree "+
				"and re-run; do not launch an agent with no worktree of its own.",
			stepWorktreeCreate, wtName, o.root, firstLine(wt.stderr)), wt.err)
	}
	home := firstLine(wt.stdout)
	if home == "" || home == "(no output)" || !strings.HasPrefix(home, "/") {
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: `deskwt add %s` exited 0 but named no absolute worktree path (%q). The agent's home "+
				"is the isolation floor every other clause rests on, so a home this verb cannot state is a "+
				"dispatch it must not make.", stepWorktreeCreate, wtName, wt.stdout), nil)
	}
	o.say("%s OK: %s on %s", stepWorktreeCreate, home, branch)

	prompt, perr := assemblePrompt(o, repo, branch, home)
	if perr != nil {
		return perr
	}

	// 3 — roster registration.
	o.say("%s %s", stepRosterRegister, stepRoster(o, repo))

	// 4 — the human-decision gate.
	gate, gerr := stepDecision(o, repo)
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

// stepClaim acquires the durable claim by invoking the CONSUMER repo's own claim script.
//
// The script's exit codes ARE the deskkit contract (0 acquired · 5 a live holder owns it ·
// 6 could not be established), so they pass straight through with no re-interpretation. On
// contention this asks the script who holds it and prints the answer: a dispatcher that
// only says "refused" sends a human to go and find out by hand, and a dispatcher that
// STEALS turns a coordination failure into two agents on one branch.
func stepClaim(o dispatchOpts, repo string) error {
	script := filepath.Join(o.root, filepath.FromSlash(claimScriptRel))
	if _, err := os.Stat(script); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: %s is not present in %s, so no durable claim can be taken. A claim this verb cannot "+
				"place is NOT permission to proceed — a machine-local lock would serialise two dispatchers "+
				"on one machine and nothing at all across two, which is the case that double-dispatches.",
			stepClaimAcquire, claimScriptRel, o.root), err)
	}
	args := []string{"acquire", o.item, "--repo", repo}
	if o.branch != "" {
		args = append(args, "--branch", o.branch)
	}
	r := runCmd(o.root, script, args...)
	if r.err == nil {
		return nil
	}
	switch exitCodeOf(r.err) {
	case deskkit.ExitRefused:
		holder := firstLine(runCmd(o.root, script, "show", o.item, "--repo", repo).stdout)
		return deskkit.Refused(fmt.Sprintf(
			"step %s: %s is already claimed by a LIVE holder — do not proceed. Existing claim: %s. "+
				"This verb never steals: breaking a live claim is a deliberate, auditable act with a stated "+
				"reason, and it belongs to a human or to the claim tool's own steal verb.",
			stepClaimAcquire, o.item, holder))
	default:
		return deskkit.Unverifiable(fmt.Sprintf(
			"step %s: the claim on %s could not be established (%s) — fail closed, NEVER 'assume free'.",
			stepClaimAcquire, o.item, firstLine(r.stderr)), r.err)
	}
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
func stepDecision(o dispatchOpts, repo string) (string, error) {
	if !o.gateHuman {
		return "SKIPPED: item is not human-gated", nil
	}
	if strings.TrimSpace(o.brief) == "" {
		return "", deskkit.Refused(fmt.Sprintf(
			"step %s: --gate-human needs --brief <path> — the decision issue's content is DERIVED from the "+
				"item's own specification, never invented by the dispatcher.", stepDecisionGate))
	}
	script := filepath.Join(o.root, filepath.FromSlash(decisionScriptRel))
	if _, err := os.Stat(script); err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: %s is not present in %s, so the human-decision gate cannot be ensured. Dispatching a "+
				"human-gated item with nothing in front of the human is the failure this gate exists to "+
				"close.", stepDecisionGate, decisionScriptRel, o.root), err)
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
	labels, err := deskkit.ModelStampLabels(o.model, o.tier)
	if err != nil {
		return "", deskkit.Refused(fmt.Sprintf("step %s: %v", stepModelStamp, err))
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
