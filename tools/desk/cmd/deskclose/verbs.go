package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The CLOSED mode set. Every place that needs the list derives it from here — the
// dispatcher, the usage text, the manifest row validator — so a mode cannot exist in
// one and not another.
const (
	modeDuplicate     = "duplicate"
	modeSuperseded    = "superseded"
	modeReviewRequest = "review-request"
	modeManifest      = "manifest"
)

func modes() []string {
	return []string{modeDuplicate, modeSuperseded, modeReviewRequest, modeManifest}
}

func modeList() string { return strings.Join(modes(), " | ") }

// rowModes are the modes a manifest ROW may carry. `manifest` is not one of them: a
// manifest that could contain a manifest row is a recursion with a human authorization
// at only one level of it.
func rowModes() []string { return []string{modeDuplicate, modeSuperseded, modeReviewRequest} }

// GitHub state_reason values.
const (
	reasonNotPlanned = "not planned"
	reasonCompleted  = "completed"
)

// valueFlags are the flags that consume the following argument. The positional
// splitter needs them so `-R owner/repo 123` does not read the repo as the item number.
var valueFlags = map[string]bool{
	"-R": true, "--of": true, "--by": true, "--file": true, "--rulings": true,
	"--resume-from": true, "--max-wait": true, "--mined": true,
}

var numTokenRe = regexp.MustCompile(`^#?\d+$`)

// splitPositionals pulls bare item numbers out of an argv so the brief's documented
// shape (`deskclose duplicate -R <repo> <N> --of <M>`) parses. Go's flag package stops
// at the first non-flag token, which would silently drop every flag AFTER the number —
// and a dropped `--mined` is a mandatory assertion that quietly became optional.
func splitPositionals(args []string) (flags []string, positional []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if valueFlags[a] && i+1 < len(args) {
			flags = append(flags, a, args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			continue
		}
		if numTokenRe.MatchString(a) {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
	}
	return flags, positional
}

// common carries the flags every mode shares.
type common struct {
	repo    string
	dryRun  bool
	rulings string
}

func (c *common) bind(fs *flag.FlagSet) {
	fs.StringVar(&c.repo, "R", "", "owner/repo")
	fs.BoolVar(&c.dryRun, "dry-run", false, "validate and read the remote; write nothing")
	fs.StringVar(&c.rulings, "rulings", defaultRulingsPath, "path to the rulings register carrying "+rulingID)
}

// requireRepo applies the repo allowlist. The set is roster configuration read from
// outside every ref the tools evaluate; deskclose adds no list of its own and has no
// flag that widens it.
func requireRepo(repo string) error {
	if strings.TrimSpace(repo) == "" {
		return deskkit.Refused("refused: -R <owner/repo> is required")
	}
	if !deskkit.IsAllowedRepo(repo) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s is not in the desk's allowed repo set %v — deskclose introduces no repo list of "+
				"its own and has no flag that widens one", deskkit.StripControl(repo), deskkit.AllowedRepos()))
	}
	return nil
}

// closeReq is one close, fully specified. Nothing downstream of applyClose consults a
// flag: every decision it makes is a field here or a value it fetched itself.
type closeReq struct {
	repo   string
	number int
	mode   string
	// target is the canonical item this close points at (`--of` / `--by` / the PR ref
	// extracted from a review-request body). Empty only where the lane has none.
	target string
	// mined is the duplicate lane's mandatory "I checked for unique value first"
	// assertion, echoed verbatim into the close comment.
	mined  string
	dryRun bool
	g      grant
}

func (r closeReq) stateReason() string {
	if r.mode == modeReviewRequest {
		return reasonCompleted
	}
	return reasonNotPlanned
}

// ---------------------------------------------------------------- the verbs

func cmdDuplicate(args []string, out io.Writer) error {
	fs := flag.NewFlagSet(modeDuplicate, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var c common
	c.bind(fs)
	of := fs.String("of", "", "the canonical item this duplicates")
	mined := fs.String("mined", "", "one-line summary of the unique value folded into --of, or 'nothing-unique'")
	n, err := parseItemVerb(fs, args, &c)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*of) == "" {
		return deskkit.Refused("refused: duplicate requires --of <M>, the canonical item")
	}
	// --mined is mandatory and is checked BEFORE the lane refusal below, so the
	// contract stays enforced against the day the lane unlocks. A duplicate close
	// without a statement that the loser was mined for unique value first is the exact
	// content loss the two-role duplicate procedure exists to prevent.
	if strings.TrimSpace(*mined) == "" {
		return deskkit.Refused(
			"refused: duplicate requires --mined <summary> — a one-line statement of what unique value " +
				"was folded into the canonical item, or the literal 'nothing-unique'. It is echoed into " +
				"the close comment, so the claim is on the record and falsifiable.")
	}
	g, err := gateFor(&c)
	if err != nil {
		return err
	}
	// R-1 withdraws this lane from the desk. See supersedeMarker.
	if !g.DuplicateLaneGranted {
		return deskkit.Refused(fmt.Sprintf(
			"refused: the duplicate lane is NOT the desk's to run. %s leaves it with the "+
				"two-role duplicate procedure: a strong-tier worker reviews both items and folds "+
				"the loser's unique content into the FIRST-FILED survivor, and then a REVIEWER closes the "+
				"duplicate — only after agreeing the content moved. Two roles, and neither is a batch tool.\n"+
				"  Fold the content, then have the reviewer close %s#%d.\n"+
				"  This unlocks only if %s's sign-off says %q; accepting %s as written does not widen it.",
			rulingID, c.repo, n, rulingID, supersedeMarker, rulingID))
	}
	return applyClose(closeReq{
		repo: c.repo, number: n, mode: modeDuplicate,
		target: strings.TrimSpace(*of), mined: strings.TrimSpace(*mined),
		dryRun: c.dryRun, g: g,
	}, out)
}

func cmdSuperseded(args []string, out io.Writer) error {
	fs := flag.NewFlagSet(modeSuperseded, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var c common
	c.bind(fs)
	by := fs.String("by", "", "the issue or PR that supersedes this item")
	n, err := parseItemVerb(fs, args, &c)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*by) == "" {
		return deskkit.Refused("refused: superseded requires --by <ref> (an issue or PR)")
	}
	g, err := gateFor(&c)
	if err != nil {
		return err
	}
	return applyClose(closeReq{
		repo: c.repo, number: n, mode: modeSuperseded,
		target: strings.TrimSpace(*by), dryRun: c.dryRun, g: g,
	}, out)
}

func cmdReviewRequest(args []string, out io.Writer) error {
	fs := flag.NewFlagSet(modeReviewRequest, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var c common
	c.bind(fs)
	n, err := parseItemVerb(fs, args, &c)
	if err != nil {
		return err
	}
	g, err := gateFor(&c)
	if err != nil {
		return err
	}
	return applyClose(closeReq{
		repo: c.repo, number: n, mode: modeReviewRequest, dryRun: c.dryRun, g: g,
	}, out)
}

// parseItemVerb parses the flags + the single positional item number the three
// item-level verbs share, and applies the repo allowlist.
func parseItemVerb(fs *flag.FlagSet, args []string, c *common) (int, error) {
	flags, pos := splitPositionals(args)
	if err := fs.Parse(flags); err != nil {
		return 0, deskkit.Refused(fs.Name() + ": " + err.Error())
	}
	pos = append(pos, fs.Args()...)
	if len(pos) != 1 {
		return 0, deskkit.Refused(fmt.Sprintf(
			"refused: %s takes exactly one item number, got %d %v", fs.Name(), len(pos), pos))
	}
	n, err := atoiPositive(pos[0], "the item number")
	if err != nil {
		return 0, err
	}
	if err := requireRepo(c.repo); err != nil {
		return 0, err
	}
	return n, nil
}

// gateFor runs the ruling gate once per process. Every mode goes through it, so there
// is no verb that can reach a write without a fetched-and-verified human artifact
// behind it.
var cachedGrant *grant

func gateFor(c *common) (grant, error) {
	if cachedGrant != nil {
		return *cachedGrant, nil
	}
	g, err := authorize(c.rulings)
	if err != nil {
		return grant{}, err
	}
	cachedGrant = &g
	return g, nil
}

// ---------------------------------------------------------------- the close itself

// applyClose is the single path to a close. All four modes funnel through it, so a
// precondition added here cannot be missing from one lane.
//
// Order matters and is fixed:
//
//  1. read the item              — a read failure is exit 6, never an assumption
//  2. idempotent no-op           — already closed is success, so a resumed run is safe
//  3. decision-label refusal     — before any lane logic, in every mode
//  4. lane preconditions         — merged-not-closed, disposition record, target state
//  5. comment (charged write)    — the trail lands BEFORE the close
//  6. close   (charged write)
//
// Steps 5 and 6 are two charged writes per item. That is the budget arithmetic the
// manifest loop is bounded by, and it is deliberate: the comment is what makes a wrong
// batch auditable and reversible.
func applyClose(r closeReq, out io.Writer) error {
	a := &auditCtx{verb: r.mode, repo: r.repo, number: r.number}

	it, err := fetchItem(r.repo, r.number)
	if err != nil {
		a.log(deskkit.ResultUnwritten, err.Error())
		return err
	}
	a.title = it.Title

	if it.closed() {
		a.log(deskkit.ResultNoop, "already closed — idempotent no-op")
		fmt.Fprintf(out, "noop\t%s#%d\talready closed\n", r.repo, r.number)
		return nil
	}
	if err := refuseDecisionItem(r.repo, it); err != nil {
		a.log(deskkit.ResultRefused, err.Error())
		return err
	}

	target, disp, err := verifyLane(r, it)
	if err != nil {
		a.log(resultFor(err), err.Error())
		return err
	}
	r.target = target
	body := closeComment(r, it, disp)

	if r.dryRun {
		// ResultDryRun is emitted only from a path that provably performed no outward
		// write, and the flag that selects it is the flag that suppresses the write.
		a.log(deskkit.ResultDryRun, "dry-run: verified, wrote nothing")
		fmt.Fprintf(out, "dry-run\t%s#%d\t%s\ttarget=%s\n", r.repo, r.number, r.mode, r.target)
		fmt.Fprintln(out, body)
		return nil
	}

	if err := allowWrite(r.repo, r.number); err != nil {
		a.log(deskkit.ResultRateLimited, err.Error())
		return err
	}
	if err := postComment(r.repo, r.number, body); err != nil {
		a.log(deskkit.ResultUnverifiable, err.Error())
		return err
	}
	a.log(deskkit.ResultOK, "posted the pre-close comment")

	if err := allowWrite(r.repo, r.number); err != nil {
		// The comment landed and the close did not. That is the resumable state by
		// design: re-running re-posts a comment and closes, and the close is what the
		// idempotency check keys on.
		a.log(deskkit.ResultRateLimited, "comment posted, close deferred: "+err.Error())
		return err
	}
	if err := closeItem(r.repo, r.number, it.isPR(), r.stateReason()); err != nil {
		a.log(deskkit.ResultUnverifiable, err.Error())
		return err
	}
	a.log(deskkit.ResultOK, fmt.Sprintf("closed as %s via lane %s (target %s)", r.stateReason(), r.mode, r.target))
	fmt.Fprintf(out, "closed\t%s#%d\t%s\ttarget=%s\treason=%s\n", r.repo, r.number, r.mode, r.target, r.stateReason())
	return nil
}

// verifyLane applies the per-lane preconditions and returns the resolved target plus
// any disposition record consulted.
func verifyLane(r closeReq, it item) (string, dispositionRead, error) {
	var disp dispositionRead
	target := r.target

	if r.mode == modeReviewRequest {
		n, err := extractPRRef(r.repo, it)
		if err != nil {
			return "", disp, err
		}
		target = fmt.Sprintf("#%d", n)
	}

	// A PULL REQUEST being closed must already carry a terminal disposition record.
	// deskclose supplies the human authorization and the execution; the FINDING is the
	// worker's, recorded by deskdisposition.
	if it.isPR() {
		d, err := requireTerminalDisposition(r.repo, r.number, target)
		if err != nil {
			return "", disp, err
		}
		disp = d
		if target == "" {
			target = d.Record.Evidence
		}
	}

	// Lanes 2 and 3 require the referenced PR to be genuinely MERGED. A ref that names
	// an ISSUE skips this check — an issue cannot merge — so the ref kind is resolved
	// first, and an unresolvable ref is a refusal rather than a skipped check.
	if r.mode == modeSuperseded || r.mode == modeReviewRequest {
		repo, n, err := resolveRef(r.repo, target)
		if err != nil {
			return "", disp, err
		}
		refItem, err := fetchItem(repo, n)
		if err != nil {
			return "", disp, err
		}
		if refItem.isPR() {
			if err := requireMergedPR(repo, n); err != nil {
				return "", disp, err
			}
		} else if r.mode == modeReviewRequest {
			return "", disp, deskkit.Refused(fmt.Sprintf(
				"refused: %s#%d is an issue, not a pull request — the review-request lane closes on a "+
					"MERGED PR and nothing else", repo, n))
		}
	}

	if r.mode == modeDuplicate {
		repo, n, err := resolveRef(r.repo, target)
		if err != nil {
			return "", disp, err
		}
		refItem, err := fetchItem(repo, n)
		if err != nil {
			return "", disp, err
		}
		if refItem.closed() && !refItem.isPR() {
			return "", disp, deskkit.Refused(fmt.Sprintf(
				"refused: the canonical item %s#%d is itself closed — folding a duplicate into a closed "+
					"issue retires both", repo, n))
		}
	}
	return target, disp, nil
}

// resolveRef turns `#123`, `owner/repo#123` or a permalink into (repo, number),
// defaulting the repo to the item's own. An unparseable ref is a refusal: deskclose
// never proceeds on a target it could not identify.
func resolveRef(defaultRepo, ref string) (string, int, error) {
	n, ok := refNum(ref)
	if !ok {
		return "", 0, deskkit.Refused("refused: cannot read " + deskkit.StripControl(ref) +
			" as an item reference (want #N, owner/repo#N, or a github.com permalink)")
	}
	repo := refRepo(ref)
	if repo == "" {
		repo = defaultRepo
	}
	if err := requireRepo(repo); err != nil {
		return "", 0, err
	}
	return repo, n, nil
}

// closeComment renders the trail. It names the lane, the canonical target and the
// authorizing ruling artifact, so a reader of the closed item can check the authority
// rather than take the tool's word for it.
func closeComment(r closeReq, it item, d dispositionRead) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Closing via the **%s** lane.\n\n", r.mode)
	fmt.Fprintf(&b, "- Canonical target: %s\n", deskkit.StripControl(r.target))
	fmt.Fprintf(&b, "- State reason: %s\n", r.stateReason())
	fmt.Fprintf(&b, "- Authorizing ruling: %s — %s\n", rulingID, r.g.SignOffURL)
	if r.mined != "" {
		fmt.Fprintf(&b, "- Mined for unique value first: %s\n", deskkit.StripControl(r.mined))
	}
	if d.Record.Verdict != "" {
		fmt.Fprintf(&b, "- Disposition record: %s — evidence %s (recorded %s by %s)\n",
			deskkit.StripControl(d.Record.Verdict), deskkit.StripControl(d.Record.Evidence),
			deskkit.StripControl(d.Record.RecordedAt), deskkit.StripControl(d.Record.RecordedBy))
	}
	b.WriteString("\nThis close is the propagation of a human-authorized event, not a judgement by the " +
		"tool that performed it. If it is wrong, reopen it — nothing here is irreversible, and the " +
		"lane, the target and the authorizing artifact are all named above so the decision is auditable.\n")
	return b.String()
}

func resultFor(err error) string {
	switch {
	case deskkit.IsRefused(err):
		return deskkit.ResultRefused
	case deskkit.IsRateLimited(err):
		return deskkit.ResultRateLimited
	case deskkit.IsDisabled(err):
		return deskkit.ResultDisabled
	default:
		return deskkit.ResultUnwritten
	}
}

// ---------------------------------------------------------------- audit

type auditCtx struct {
	verb   string
	repo   string
	number int
	title  string
}

func (a *auditCtx) log(result, detail string) {
	n := a.number
	_ = deskkit.Log(deskkit.Entry{
		Tool:       toolName,
		Verb:       a.verb,
		Result:     result,
		Detail:     deskkit.StripControl(detail),
		Repo:       a.repo,
		PR:         &n,
		Title:      a.title,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
	})
}
