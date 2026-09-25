package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	verbCheck     = "check"
	verbRecompute = "recompute"
	verbMerge     = "merge"
)

// laneLoop is the loop identity that owns the lane: the review desk, whose App is the
// reviewer role — the identity that applies the admission label and would perform a merge.
const laneLoop = "pr-review-desk"

// reviewerRole / workerRole are the roster roles the lane reads. Logins are never literals
// here: they come from the roster's role bindings.
const (
	reviewerRole = "reviewer"
	workerRole   = "worker"
)

// The human-queue labels an ejection swaps to — deskflip's two, unchanged.
const (
	labelBeforeFlip = "authorization-needed"
	labelAfterFlip  = "approval-needed"
	laneLabelColor  = "5319e7"
	queueLabelColor = "0e8a16"
)

// Condition names — the verb's contract. A refusal names one of these.
const (
	condCallerRole       = "caller-role"
	condConfig           = "config"
	condAppToken         = "app-token"
	condPROpenReady      = "pr-open-ready"
	condPriorEjection    = "prior-ejection"
	condAreaAdmit        = "area-admit"
	condScore            = "score"
	condReviewerApproved = "reviewer-approved"
	condChecksGreen      = "checks-green"
	condMergeable        = "mergeable"
	condLaneArmed        = "lane-armed"
	condRulingSigned     = "ruling-signed"
	condHeadStable       = "head-stable"
)

// mergeConditions is the ORDERED chain `merge` evaluates, pinned by a test. The order:
//
//   - caller-role, then config — both before the FIRST forge request, so a session that is
//     not the review desk, or a lane that is closed or misconfigured, costs no read at all;
//   - app-token — before the first forge call, so nothing is read on an ambient credential;
//   - pr-open-ready — the one PR read every later condition consumes;
//   - prior-ejection — the one-way latch, read from the local audit log AND the reviewer
//     App's marked ejection comment on the PR. It runs BEFORE the category and the score so
//     an ejected PR is refused as ejected, whatever a later recompute at a since-cleaned head
//     would read;
//   - area-admit, score — the category and the demotion score; either failing EJECTS;
//   - reviewer-approved, checks-green, mergeable — the merge step's own re-read at the
//     current head, catching a state the score alone did not see;
//   - lane-armed — the kill switch, the kill signal and the daily cap;
//   - ruling-signed — the enactment gate;
//   - head-stable — LAST, because its purpose is to be the final read before the mutation.
var mergeConditions = []string{
	condCallerRole,
	condConfig,
	condAppToken,
	condPROpenReady,
	condPriorEjection,
	condAreaAdmit,
	condScore,
	condReviewerApproved,
	condChecksGreen,
	condMergeable,
	condLaneArmed,
	condRulingSigned,
	condHeadStable,
}

type opts struct {
	verb    string
	pr      int
	repo    string
	root    string
	rulings string // repo-relative register path, read through the forge
	// rulingsRepo is the owner/name the register is read from, at ITS default branch — never
	// from a caller's worktree, which may be a checkout of a PR head.
	rulingsRepo string
	fpyFile     string
	dryRun      bool
	quiet       bool
	out         io.Writer

	// defaults caches each repo's default branch for the run (lower-cased slug → branch).
	defaults map[string]string

	// signOffThread is the configured sign-off thread (0 = unset → could-not-check).
	signOffThread int

	// head is the head the verb decided on, for the audit line; wrote records whether an
	// outward write was attempted, so an unverifiable outcome is billed correctly.
	head  string
	wrote bool
}

// laneNow is the clock the daily cap reads. A test hook; not wired to any flag or env var.
var laneNow = time.Now

func (o *opts) say(format string, args ...any) {
	if !o.quiet {
		fmt.Fprintf(o.out, format+"\n", args...)
	}
}

func parseArgs(args []string, out io.Writer) (*opts, error) {
	o := &opts{verb: args[0], out: out}
	rest := args[1:]
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		n, err := strconv.Atoi(rest[0])
		if err != nil || n <= 0 {
			return nil, deskkit.Refused("refused: the PR argument must be a positive number, got " +
				deskkit.StripControl(rest[0]))
		}
		o.pr = n
		rest = rest[1:]
	}
	fs := flag.NewFlagSet(toolName, flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	repo := fs.String("repo", "", "owner/name the PR belongs to")
	root := fs.String("root", ".", "checkout root whose local .assay-surfaces the only-narrowing config check reads")
	rulings := fs.String("rulings", deskkit.AutoLaneRulingsPath,
		"repo-relative path of the rulings register, read through the forge at the register repo's default branch")
	rulingsRepo := fs.String("rulings-repo", "", "owner/name the rulings register lives in (default: --repo)")
	fpy := fs.String("fpy-file", "", "harvested per-class first-pass-yield file (absent = the lane holds)")
	dry := fs.Bool("dry-run", false, "merge: evaluate every condition and stop before the mutation")
	quiet := fs.Bool("quiet", false, "suppress the per-condition OK lines")
	if err := fs.Parse(rest); err != nil {
		return nil, deskkit.Refused("refused: bad flags: " + err.Error())
	}
	if fs.NArg() != 0 {
		return nil, deskkit.Refused("refused: unexpected arguments: " + strings.Join(fs.Args(), " "))
	}
	o.repo, o.root, o.fpyFile, o.dryRun, o.quiet = strings.TrimSpace(*repo), *root, *fpy, *dry, *quiet
	o.rulings = strings.TrimSpace(*rulings)
	o.rulingsRepo = strings.TrimSpace(*rulingsRepo)
	o.defaults = map[string]string{}
	if o.repo == "" {
		return nil, deskkit.Refused("refused: --repo <owner/repo> is required")
	}
	if o.rulingsRepo == "" {
		o.rulingsRepo = o.repo
	}
	// The register is a path IN a repository, read through the forge — never a local file.
	// It must sit on the compiled never-admit set, so no lane merge can edit the line that
	// enacts the lane.
	if o.rulings == "" || strings.HasPrefix(o.rulings, "/") || path.Clean(o.rulings) != o.rulings ||
		strings.HasPrefix(o.rulings, "../") || o.rulings == ".." {
		return nil, deskkit.Refused("refused: --rulings must be a clean repo-relative path, got " +
			deskkit.StripControl(o.rulings))
	}
	if !deskkit.MatchSurfaceGlob(deskkit.AutoLaneRulingsGlob, o.rulings) || !deskkit.IsAutoLaneNeverAdmitPath(o.rulings) {
		return nil, deskkit.Refused("refused: --rulings " + deskkit.StripControl(o.rulings) + " is not a rulings " +
			"register on the lane's compiled never-admit set (" + deskkit.AutoLaneRulingsGlob + "), so a lane merge " +
			"could edit its own enactment line")
	}
	if o.verb != verbCheck && o.pr == 0 {
		return nil, deskkit.Refused("refused: " + deskkit.StripControl(o.verb) + " requires a PR number")
	}
	if o.dryRun && o.verb != verbMerge {
		return nil, deskkit.Refused("refused: --dry-run applies to merge only")
	}
	return o, nil
}

// --- the shared preamble ------------------------------------------------------------------

func checkCallerRole() error {
	raw := strings.TrimSpace(os.Getenv("DESK_LOOP"))
	if raw == "" {
		return deskkit.Refused(fmt.Sprintf("refused: %s — $DESK_LOOP is unset; the lane belongs to %s",
			condCallerRole, laneLoop))
	}
	names, known := deskkit.LoopFlagNames(raw)
	if !known {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — $DESK_LOOP=%q is not a loop name the "+
			"kill switch recognises", condCallerRole, raw), nil)
	}
	if names[0] != laneLoop {
		return deskkit.Refused(fmt.Sprintf("refused: %s — this session presents loop %q; the lane belongs to %s",
			condCallerRole, names[0], laneLoop))
	}
	return nil
}

// loadConfig is condition `config`: the lane keys load and every rule that needs NO forge
// read holds. It runs before the first forge request, so a closed or refused lane costs none.
//
// It also runs the surface-overlap rule against the LOCAL checkout's `.assay-surfaces` when
// one is present. That rule is only-narrowing (it can refuse, never admit), so a local copy a
// caller could edit can only add refusals; the AUTHORITATIVE overlap test reads the base
// branch's copy through the forge at area-admit.
func loadConfig(o *opts) (deskkit.AutoLaneConfig, error) {
	ld := deskkit.LoadAutoLaneConfig()
	switch ld.State {
	case deskkit.AutoLaneUnconfigured:
		return deskkit.AutoLaneConfig{}, deskkit.Refused(fmt.Sprintf(
			"refused: %s — the lane is CLOSED: no ASSAY_AUTOAPPROVE_* key is set in the roster (the shipped "+
				"state). Nothing is admitted, ejected or merged.", condConfig))
	case deskkit.AutoLaneConfigRefused:
		auditConfigRefused(o, ld.Problem)
		return deskkit.AutoLaneConfig{}, deskkit.Refused(ld.Problem)
	}
	cfg := ld.Config
	o.signOffThread = cfg.SignOffThread
	if data, err := os.ReadFile(filepath.Join(o.root, ".assay-surfaces")); err == nil {
		if p := deskkit.AutoLaneSurfaceOverlap(cfg, o.repo, deskkit.ParseSurfaceGlobs(data)); p != "" {
			auditConfigRefused(o, p)
			return deskkit.AutoLaneConfig{}, deskkit.Refused(p)
		}
	}
	repos := map[string]bool{}
	for _, a := range cfg.Areas {
		repos[strings.ToLower(a.Repo)] = true
	}
	var sorted []string
	for r := range repos {
		sorted = append(sorted, r)
	}
	sort.Strings(sorted)
	for _, r := range sorted {
		if p := deskkit.AutoLaneAreaTripwires(cfg, r, deskkit.RiskPathTriggered); p != "" {
			auditConfigRefused(o, p)
			return deskkit.AutoLaneConfig{}, deskkit.Refused(p)
		}
	}
	o.say("%s OK: %d area(s), eject line %d, fpy floor %.2f, daily cap %d",
		condConfig, len(cfg.Areas), cfg.EjectLine, cfg.FPYFloor, cfg.DailyCap)
	return cfg, nil
}

func preamble(o *opts) (deskkit.AutoLaneConfig, deskkit.ForgeRepo, laneForge, error) {
	if err := checkCallerRole(); err != nil {
		return deskkit.AutoLaneConfig{}, deskkit.ForgeRepo{}, nil, err
	}
	o.say("%s OK: caller presents %s", condCallerRole, laneLoop)
	if !deskkit.IsAllowedRepo(o.repo) {
		return deskkit.AutoLaneConfig{}, deskkit.ForgeRepo{}, nil, deskkit.Refused(fmt.Sprintf(
			"refused: %s is not in the desk repo set", deskkit.StripControl(o.repo)))
	}
	cfg, err := loadConfig(o)
	if err != nil {
		return cfg, deskkit.ForgeRepo{}, nil, err
	}
	for _, role := range []string{reviewerRole, workerRole} {
		if rerr := deskkit.RequireRole(role); rerr != nil {
			return cfg, deskkit.ForgeRepo{}, nil, deskkit.Unverifiable(fmt.Sprintf(
				"could-not-check: %s — %v", condAreaAdmit, rerr), nil)
		}
	}
	owner, name, ok := strings.Cut(o.repo, "/")
	if !ok || owner == "" || name == "" {
		return cfg, deskkit.ForgeRepo{}, nil, deskkit.Refused("refused: --repo must be owner/name")
	}
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	// The register repo is an operator input like --repo: it must be in the desk repo set and
	// under --repo's own owner, so the enactment line is always read from a repo the lane's
	// owner controls.
	rOwner, rName, rok := strings.Cut(o.rulingsRepo, "/")
	if !rok || rOwner == "" || rName == "" || !deskkit.IsAllowedRepo(o.rulingsRepo) || !strings.EqualFold(rOwner, owner) {
		return cfg, deskkit.ForgeRepo{}, nil, deskkit.Refused(fmt.Sprintf(
			"refused: --rulings-repo %s must be an owner/name in the desk repo set, under %s",
			deskkit.StripControl(o.rulingsRepo), deskkit.StripControl(owner)))
	}
	fg, err := checkAppToken(o, fr)
	return cfg, fr, fg, err
}

// defaultBranch resolves fr's default branch through the forge's repo document, once per run.
// The lane reads its base-branch inputs at the DEFAULT branch — never at a PR's author-chosen
// base, and never from a caller's worktree.
func defaultBranch(o *opts, fg laneForge, fr deskkit.ForgeRepo) (string, error) {
	key := strings.ToLower(fr.Slug())
	if b, ok := o.defaults[key]; ok {
		return b, nil
	}
	raw, err := fg.RepoHardeningRead(fr, deskkit.HardeningReadRepo)
	if err != nil {
		return "", fmt.Errorf("the repo document of %s could not be read: %w", fr.Slug(), err)
	}
	var doc struct {
		DefaultBranch string `json:"default_branch"`
	}
	if jerr := json.Unmarshal(raw, &doc); jerr != nil || strings.TrimSpace(doc.DefaultBranch) == "" {
		return "", errors.New("the repo document of " + fr.Slug() + " names no default branch")
	}
	b := strings.TrimSpace(doc.DefaultBranch)
	o.defaults[key] = b
	return b, nil
}

// --- facts ----------------------------------------------------------------------------------

// facts is everything one gate reads about the PR, three-state: each read keeps its error.
type facts struct {
	pr         *deskkit.PullRequest
	head       string
	files      []string
	filesErr   error
	reviews    []deskkit.Review
	reviewsErr error
	checks     *deskkit.ChecksAtHead
	checksErr  error
	events     []deskkit.LabelEvent
	eventsErr  error
	model      deskkit.ModelState

	surfPresent bool
	surfGlobs   []string
	surfErr     error

	// defaultBranch is --repo's default branch (defaultErr when it could not be resolved);
	// comments is the PR's own thread, which carries the forge half of the ejection latch.
	defaultBranch string
	defaultErr    error
	comments      []deskkit.Comment
	commentsErr   error

	// inLane is whether the PR carries the admission label as applied by the reviewer App;
	// laneWhy says why not.
	inLane  bool
	laneWhy string
}

func gather(o *opts, fg laneForge, fr deskkit.ForgeRepo) (*facts, error) {
	pr, err := fg.GetPullRequest(fr, o.pr)
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — PR #%d could not be read",
			condPROpenReady, o.pr), err)
	}
	f := &facts{pr: pr, head: strings.TrimSpace(pr.HeadSHA)}
	o.head = f.head

	cf, ferr := fg.ListChangedFiles(fr, o.pr)
	switch {
	case ferr != nil:
		f.filesErr = ferr
	case pr.ChangedFiles > len(cf):
		f.filesErr = fmt.Errorf("the forge asserts %d changed file(s) but served %d", pr.ChangedFiles, len(cf))
	default:
		for _, c := range cf {
			f.files = append(f.files, c.Filename)
			if c.PreviousFilename != "" && c.PreviousFilename != c.Filename {
				f.files = append(f.files, c.PreviousFilename)
			}
		}
	}
	f.reviews, f.reviewsErr = fg.ReviewsAtHead(fr, o.pr)
	if f.head != "" {
		f.checks, f.checksErr = fg.ChecksAtHead(fr, f.head)
	} else {
		f.checksErr = fmt.Errorf("no head")
	}
	f.events, f.eventsErr = fg.ListLabelEvents(fr, o.pr)
	if f.eventsErr == nil {
		_, f.model = deskkit.AttestedModelStampOf(
			deskkit.StampTimeline{Present: pr.Labels, Events: f.events}, deskkit.IsDispatcherLogin)
	}

	f.comments, f.commentsErr = fg.ListComments(fr, o.pr)
	f.defaultBranch, f.defaultErr = defaultBranch(o, fg, fr)

	// The surfaces are read at the DEFAULT branch: a PR's base is the author's choice, and a
	// base the author controls could carry a permissive .assay-surfaces.
	var fc *deskkit.FileContent
	serr := f.defaultErr
	if serr == nil {
		fc, serr = fg.ReadFile(fr, deskkit.ReadFileInput{File: ".assay-surfaces", Ref: f.defaultBranch})
	}
	switch {
	case serr != nil && deskkit.IsForgeNotFound(serr):
		f.surfPresent = false
	case serr != nil:
		f.surfErr = serr
	case fc == nil || !fc.Exists:
		f.surfPresent = false
	default:
		f.surfPresent = true
		f.surfGlobs = deskkit.ParseSurfaceGlobs(fc.Content)
	}

	reviewer, _ := deskkit.RoleAppLogin(reviewerRole)
	switch applier, ok := deskkit.AutoLaneLabelApplier(pr.Labels, f.events); {
	case f.eventsErr != nil:
		f.laneWhy = "the label timeline could not be read, so the admission label cannot be attributed"
	case !hasLabel(pr.Labels, deskkit.AutoLaneLabel):
		f.laneWhy = "the PR carries no " + deskkit.AutoLaneLabel + " label"
	case !ok:
		f.laneWhy = "the " + deskkit.AutoLaneLabel + " label on the PR cannot be attributed to any applier"
	case !strings.EqualFold(applier, reviewer):
		f.laneWhy = "the " + deskkit.AutoLaneLabel + " label was applied by " + deskkit.StripControl(applier) +
			", not the reviewer App — a self-applied admission is no admission"
	default:
		f.inLane = true
	}
	return f, nil
}

func (f *facts) admit(cfg deskkit.AutoLaneConfig, repo string) deskkit.AutoLaneAdmit {
	worker, _ := deskkit.RoleAppLogin(workerRole)
	return deskkit.AdmitAutoLane(cfg, deskkit.AutoLaneAdmitInput{
		Repo:            repo,
		AuthorLogin:     f.pr.Author.Login,
		WorkerAppLogin:  worker,
		Body:            f.pr.Body,
		ChangedFiles:    f.files,
		FilesErr:        f.filesErr,
		Labels:          f.pr.Labels,
		Reviews:         f.reviews,
		ReviewsErr:      f.reviewsErr,
		SurfacesPresent: f.surfPresent,
		SurfaceGlobs:    f.surfGlobs,
		SurfacesErr:     f.surfErr,
		RiskClassed:     deskkit.RiskPathTriggered,
	})
}

func (f *facts) score() deskkit.AutoLaneScore {
	reviewer, _ := deskkit.RoleAppLogin(reviewerRole)
	return deskkit.ScoreAutoLane(deskkit.AutoLaneScoreInput{
		Head: f.head, Reviews: f.reviews, ReviewsErr: f.reviewsErr,
		Checks: f.checks, ChecksErr: f.checksErr, Labels: f.pr.Labels,
		Model: f.model, ModelErr: f.eventsErr,
		Events: f.events, ReviewerLogin: reviewer,
	})
}

// validateBase is the authoritative half of the config's surface rule: the DEFAULT branch's
// own `.assay-surfaces` (never a caller's worktree), read through the forge. It first confines
// the lane to PRs whose base IS the default branch: a PR opened against a branch its author
// controls is never in the lane, whatever that branch declares.
func validateBase(o *opts, cfg deskkit.AutoLaneConfig, f *facts) error {
	if f.defaultErr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — the default branch of %s could not be "+
			"resolved, so the PR's base cannot be checked against it", condAreaAdmit, o.repo), f.defaultErr)
	}
	if strings.TrimSpace(f.pr.BaseRef) != f.defaultBranch {
		return deskkit.Refused(fmt.Sprintf("refused: %s — PR #%d targets %q, not the default branch %q; the lane "+
			"admits and merges only into the default branch", condAreaAdmit, o.pr,
			deskkit.StripControl(f.pr.BaseRef), f.defaultBranch))
	}
	if f.surfErr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — the base branch's .assay-surfaces could "+
			"not be read", condAreaAdmit), f.surfErr)
	}
	if p := deskkit.ValidateAutoLaneRepo(cfg, o.repo, f.surfPresent, f.surfGlobs, deskkit.RiskPathTriggered); p != "" {
		auditConfigRefused(o, p)
		return deskkit.Refused(p)
	}
	return nil
}

// verdict is one gate's combined category + score outcome.
type verdict struct {
	admit   deskkit.AutoLaneAdmit
	score   deskkit.AutoLaneScore
	reasons []string // every tripwire and fired signal, when the PR must leave the lane
}

func decide(cfg deskkit.AutoLaneConfig, repo string, f *facts) verdict {
	v := verdict{admit: f.admit(cfg, repo), score: f.score()}
	seen := map[string]bool{}
	add := func(n string) {
		if !seen[n] {
			seen[n] = true
			v.reasons = append(v.reasons, n)
		}
	}
	if !v.admit.Admitted {
		for _, t := range v.admit.Tripwires {
			add(t)
		}
	}
	if v.score.Ejects(cfg.EjectLine) {
		for _, s := range v.score.Fired {
			add(s)
		}
	}
	return v
}

// onlyUnreadable reports whether the only reason the PR fails is that something could not be
// read. That is could-not-check: the PR does not merge, but a transient read failure is not
// written down as a one-way ejection.
func (v verdict) onlyUnreadable() bool {
	if len(v.reasons) == 0 {
		return false
	}
	for _, r := range v.reasons {
		if r != deskkit.SignalUnreadable && r != deskkit.TripUnreadable {
			return false
		}
	}
	return true
}

// --- the writes -----------------------------------------------------------------------------

// eject performs the ejection for a PR in the lane: remove the admission label, add the
// human-queue label, post ONE marked comment, and write the autolane:eject audit line — the
// latch. Every write is gated on the enactment gate; unenacted, it writes NOTHING and says
// what it would have done. It always returns a refusal (exit 5) naming the reasons, or an
// unverifiable when a write failed.
func eject(o *opts, fg laneForge, fr deskkit.ForgeRepo, f *facts, reasons []string) error {
	names := strings.Join(reasons, ", ")
	if ok, gerr := enactment(o, fg); !ok {
		return deskkit.Refused(fmt.Sprintf("eject: %s (NOT written — the lane is not enacted: %s)",
			names, firstLine(gerr.Error())))
	}
	queue := labelBeforeFlip
	if !f.pr.Draft {
		queue = labelAfterFlip
	}
	o.wrote = true
	_, lerr := fg.ApplyLabels(fr, o.pr, deskkit.LabelChange{
		Remove: []string{deskkit.AutoLaneLabel},
		Add:    []deskkit.LabelSpec{{Name: queue, Color: queueLabelColor}},
	})
	if lerr != nil {
		logEject(o, deskkit.ResultUnverifiable, names)
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: the ejection label swap on %s#%d failed "+
			"(the ejection is latched in the audit log regardless)", o.repo, o.pr), lerr)
	}
	reviewer, _ := deskkit.RoleAppLogin(reviewerRole)
	comments, cerr := fg.ListComments(fr, o.pr)
	if cerr != nil {
		logEject(o, deskkit.ResultUnverifiable, names)
		return deskkit.Unverifiable("could-not-check: the PR's comments could not be read, so the one ejection "+
			"comment's idempotency cannot be established (the ejection is latched regardless)", cerr)
	}
	marked := false
	for _, c := range comments {
		if strings.Contains(c.Body, deskkit.AutoLaneEjectMarker) && strings.EqualFold(c.Author.Login, reviewer) {
			marked = true
			break
		}
	}
	if !marked {
		if _, perr := fg.PostComment(fr, o.pr, deskkit.AutoLaneEjectComment(reasons, f.head)); perr != nil {
			logEject(o, deskkit.ResultUnverifiable, names)
			return deskkit.Unverifiable("could-not-check: the ejection comment could not be posted (the "+
				"ejection is latched regardless)", perr)
		}
	}
	logEject(o, deskkit.ResultOK, names)
	return deskkit.Refused("eject: " + names + " — the PR left the lane for the human queue (" + queue + ")")
}

// --- verbs ----------------------------------------------------------------------------------

func cmdMerge(o *opts) error {
	cfg, fr, fg, err := preamble(o)
	if err != nil {
		return err
	}

	// --- pr-open-ready ---
	f, err := gather(o, fg, fr)
	if err != nil {
		return err
	}
	if !strings.EqualFold(f.pr.State, "open") {
		return deskkit.Refused(fmt.Sprintf("refused: %s — PR #%d is %s", condPROpenReady, o.pr, f.pr.State))
	}
	if f.pr.Draft {
		return deskkit.Refused(fmt.Sprintf("refused: %s — PR #%d is still a draft; a lane merge follows the "+
			"ready flip, never precedes it", condPROpenReady, o.pr))
	}
	if f.head == "" {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — PR #%d reports no head", condPROpenReady, o.pr), nil)
	}
	o.say("%s OK: open, ready, at %s", condPROpenReady, short(f.head))

	// --- prior-ejection ---
	if err := checkPriorEjection(o, f); err != nil {
		return err
	}
	o.say("%s OK: no ejection recorded for %s#%d", condPriorEjection, o.repo, o.pr)

	// --- area-admit + score ---
	if err := validateBase(o, cfg, f); err != nil {
		return err
	}
	if !f.inLane {
		return deskkit.Refused(fmt.Sprintf("refused: %s — not admitted: %s", condAreaAdmit, f.laneWhy))
	}
	v := decide(cfg, o.repo, f)
	if v.onlyUnreadable() {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — unreadable input(s): %s; nothing merges on "+
			"an unread input", unreadableCond(v), strings.Join(v.score.Unreadable, ", ")), nil)
	}
	if len(v.reasons) > 0 {
		if o.dryRun {
			// A dry run writes NOTHING — no label swap, no comment, no latch. The ejection
			// is one-way, so previewing it must never perform it.
			names := strings.Join(v.reasons, ", ")
			fmt.Fprintf(o.out, "dry-run: would eject: %s\n", names)
			return deskkit.Refused("dry-run: would eject: " + names + " (NOT written — --dry-run writes nothing)")
		}
		return eject(o, fg, fr, f, v.reasons)
	}
	o.say("%s OK: every changed path inside an opted-in area, no tripwire", condAreaAdmit)
	o.say("%s OK: %d (line %d)", condScore, v.score.Score, cfg.EjectLine)

	// --- reviewer-approved ---
	if err := checkReviewerApproved(f); err != nil {
		return err
	}
	o.say("%s OK: the reviewer App APPROVED at %s", condReviewerApproved, short(f.head))

	// --- checks-green ---
	if err := checkChecksGreen(fg, fr, f); err != nil {
		return err
	}
	o.say("%s OK", condChecksGreen)

	// --- mergeable ---
	switch strings.ToUpper(strings.TrimSpace(f.pr.Mergeable)) {
	case "MERGEABLE":
	case "CONFLICTING":
		return deskkit.Refused(fmt.Sprintf("refused: %s — PR #%d is CONFLICTING", condMergeable, o.pr))
	default:
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — the forge reports mergeable=%q",
			condMergeable, f.pr.Mergeable), nil)
	}
	o.say("%s OK", condMergeable)

	// --- lane-armed ---
	if err := checkLaneArmed(o, cfg); err != nil {
		return err
	}

	// --- ruling-signed ---
	if ok, gerr := enactment(o, fg); !ok {
		return gerr
	}
	o.say("%s OK: %s's Sign-off resolves to the blessing authority", condRulingSigned, deskkit.AutoLaneRulingID)

	// --- head-stable ---
	pr2, err := fg.GetPullRequest(fr, o.pr)
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — the PR could not be re-read", condHeadStable), err)
	}
	if h2 := strings.TrimSpace(pr2.HeadSHA); h2 != f.head {
		return deskkit.Refused(fmt.Sprintf("refused: %s — the head moved during the checks (%s -> %s)",
			condHeadStable, short(f.head), short(h2)))
	}
	o.say("%s OK: still %s", condHeadStable, short(f.head))

	if o.dryRun {
		fmt.Fprintf(o.out, "dry-run: would merge %s into %s (merge commit)\n", f.head, f.pr.BaseRef)
		return nil
	}
	return deskkit.Refused(fmt.Sprintf("refused: merge-write — every condition held for %s#%d at %s, but this "+
		"release carries NO merge mutation: the lane's merge operation is a separate, later change. Nothing "+
		"was merged.", o.repo, o.pr, short(f.head)))
}

func cmdRecompute(o *opts) error {
	cfg, fr, fg, err := preamble(o)
	if err != nil {
		return err
	}
	f, err := gather(o, fg, fr)
	if err != nil {
		return err
	}
	if !strings.EqualFold(f.pr.State, "open") {
		return deskkit.Refused(fmt.Sprintf("refused: %s — PR #%d is %s", condPROpenReady, o.pr, f.pr.State))
	}
	ejected, perr := priorEjection(o, f)
	if perr != nil {
		return perr
	}
	if err := validateBase(o, cfg, f); err != nil {
		return err
	}
	v := decide(cfg, o.repo, f)
	fmt.Fprintf(o.out, "admit: %s\n", admitLine(v.admit))
	fmt.Fprintf(o.out, "score: %s\n", scoreLine(v.score, cfg.EjectLine))

	switch {
	case f.inLane && v.onlyUnreadable():
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — unreadable input(s): %s", unreadableCond(v),
			strings.Join(v.score.Unreadable, ", ")), nil)
	case f.inLane && (ejected || len(v.reasons) > 0):
		reasons := v.reasons
		if ejected {
			reasons = append([]string{condPriorEjection}, reasons...)
		}
		return eject(o, fg, fr, f, reasons)
	case f.inLane:
		fmt.Fprintf(o.out, "in lane: %s#%d at %s — no signal\n", o.repo, o.pr, short(f.head))
		return nil
	case ejected:
		return deskkit.Refused(fmt.Sprintf("refused: %s — %s#%d was ejected earlier; nothing re-admits it",
			condPriorEjection, o.repo, o.pr))
	case len(v.reasons) > 0:
		return deskkit.Refused(fmt.Sprintf("refused: %s — not admitted: %s", condAreaAdmit, strings.Join(v.reasons, ", ")))
	case hasLabel(f.pr.Labels, deskkit.AutoLaneLabel):
		// The label is on the PR but not under the reviewer App: never "re-apply" over a
		// foreign admission, which would launder it.
		return deskkit.Refused(fmt.Sprintf("refused: %s — %s", condAreaAdmit, f.laneWhy))
	}
	// Admissible and not yet in the lane: ADMIT — a write, so the enactment gate first.
	if ok, gerr := enactment(o, fg); !ok {
		return deskkit.Refused(fmt.Sprintf("admit: %s#%d is admissible (NOT written — the lane is not enacted: %s)",
			o.repo, o.pr, firstLine(gerr.Error())))
	}
	o.wrote = true
	if _, err := fg.ApplyLabels(fr, o.pr, deskkit.LabelChange{
		Add: []deskkit.LabelSpec{{Name: deskkit.AutoLaneLabel, Color: laneLabelColor}},
	}); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: the admission label write on %s#%d failed",
			o.repo, o.pr), err)
	}
	logLine(o, deskkit.AutoLaneVerbAdmit, deskkit.ResultOK, "admitted")
	fmt.Fprintf(o.out, "admitted: %s#%d at %s\n", o.repo, o.pr, short(f.head))
	return nil
}

func cmdCheck(o *opts) error {
	cfg, fr, fg, err := preamble(o)
	if err != nil {
		return err
	}
	var problems []error
	enacted, gerr := enactment(o, fg)
	if enacted {
		fmt.Fprintf(o.out, "enactment: %s signed by the blessing authority\n", deskkit.AutoLaneRulingID)
	} else {
		fmt.Fprintf(o.out, "enactment: not enacted — %s\n", firstLine(gerr.Error()))
		problems = append(problems, gerr)
	}
	if aerr := checkLaneArmed(o, cfg); aerr != nil {
		problems = append(problems, aerr)
	}
	if o.pr > 0 {
		f, err := gather(o, fg, fr)
		if err != nil {
			return err
		}
		o.head = f.head
		ejected, perr := priorEjection(o, f)
		if perr != nil {
			return perr
		}
		if verr := validateBase(o, cfg, f); verr != nil {
			return verr
		}
		v := decide(cfg, o.repo, f)
		fmt.Fprintf(o.out, "admit: %s\n", admitLine(v.admit))
		fmt.Fprintf(o.out, "score: %s\n", scoreLine(v.score, cfg.EjectLine))
		fmt.Fprintf(o.out, "prior-ejection: %t\n", ejected)
		if f.inLane {
			fmt.Fprintf(o.out, "in-lane: yes\n")
		} else {
			fmt.Fprintf(o.out, "in-lane: no — %s\n", f.laneWhy)
		}
		switch {
		case ejected:
			problems = append(problems, deskkit.Refused("refused: "+condPriorEjection))
		case v.onlyUnreadable():
			problems = append(problems, deskkit.Unverifiable("could-not-check: "+unreadableCond(v), nil))
		case len(v.reasons) > 0:
			problems = append(problems, deskkit.Refused("would eject: "+strings.Join(v.reasons, ", ")))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	// The strongest outcome wins: a could-not-check is reported as itself only when nothing
	// positively refused.
	for _, p := range problems {
		if deskkit.ExitCodeOf(p) == deskkit.ExitRefused {
			return p
		}
	}
	return problems[0]
}

// --- conditions -----------------------------------------------------------------------------

// priorEjection reads the one-way latch from BOTH of its halves: the local audit log's
// autolane:eject line, and the reviewer App's marked ejection comment on the PR itself. The
// comment is the half every host sees — a second host, a fresh HOME or a rotated ledger still
// finds it. Either half positively set is an ejection; with neither set, an unreadable half is
// could-not-check, never "not ejected".
func priorEjection(o *opts, f *facts) (bool, error) {
	entries, err := deskkit.LoadEntries()
	if err != nil {
		return false, deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — the audit log could not be read",
			condPriorEjection), err)
	}
	if deskkit.AutoLanePriorEjection(entries, o.repo, o.pr) {
		return true, nil
	}
	reviewer, _ := deskkit.RoleAppLogin(reviewerRole)
	if deskkit.AutoLaneEjectedOnForge(f.comments, reviewer) {
		return true, nil
	}
	if f.commentsErr != nil {
		return false, deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — the PR's comments could not be read, "+
			"so the forge-side ejection marker cannot be checked", condPriorEjection), f.commentsErr)
	}
	return false, nil
}

func checkPriorEjection(o *opts, f *facts) error {
	ejected, err := priorEjection(o, f)
	if err != nil {
		return err
	}
	if ejected {
		return deskkit.Refused(fmt.Sprintf("refused: %s — %s#%d was ejected from the lane earlier. The ejection "+
			"is one-way: a later clean recompute does not re-admit it.", condPriorEjection, o.repo, o.pr))
	}
	return nil
}

// checkReviewerApproved is the merge step's own re-read of the correctness verdict: the
// reviewer App's LATEST decisive correctness review must be APPROVED at the current head.
// A security-marker review is not a correctness verdict and is skipped.
func checkReviewerApproved(f *facts) error {
	if f.reviewsErr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — the reviews could not be read",
			condReviewerApproved), f.reviewsErr)
	}
	reviewer, _ := deskkit.RoleAppLogin(reviewerRole)
	var last *deskkit.Review
	for i := range f.reviews {
		r := &f.reviews[i]
		if !strings.EqualFold(r.Author.Login, reviewer) {
			continue
		}
		if deskkit.HasSecurityReviewPass(r.Body) || deskkit.HasSecurityReviewFail(r.Body) {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(r.State)) {
		case "APPROVED", "CHANGES_REQUESTED", "DISMISSED":
			last = r
		}
	}
	switch {
	case last == nil:
		return deskkit.Refused(fmt.Sprintf("refused: %s — the reviewer App has posted no correctness verdict",
			condReviewerApproved))
	case !strings.EqualFold(last.State, "APPROVED"):
		return deskkit.Refused(fmt.Sprintf("refused: %s — the reviewer App's latest verdict is %s",
			condReviewerApproved, last.State))
	case strings.TrimSpace(last.CommitID) != f.head:
		return deskkit.Refused(fmt.Sprintf("refused: %s — the reviewer App's APPROVED is at %s, not the current "+
			"head %s: a later commit landed after it", condReviewerApproved, short(last.CommitID), short(f.head)))
	}
	return nil
}

// checkChecksGreen re-reads CI at the current head through the SAME reduction the score's
// ci-nonsuccess signal uses, and additionally requires every branch-protection-required
// context to be PRESENT — an absent required verdict is could-not-check, never a pass.
func checkChecksGreen(fg laneForge, fr deskkit.ForgeRepo, f *facts) error {
	checks, err := fg.ChecksAtHead(fr, f.head)
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — the checks at %s could not be read",
			condChecksGreen, short(f.head)), err)
	}
	switch st, detail := deskkit.EvalChecksAtHead(checks); st {
	case deskkit.ChecksGreen:
	case deskkit.ChecksNotGreen:
		return deskkit.Refused(fmt.Sprintf("refused: %s — %s", condChecksGreen, detail))
	default:
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — %s", condChecksGreen, detail), nil)
	}
	required, rerr := fg.RequiredStatusChecks(fr, f.defaultBranch)
	if rerr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — the required-check set on %q could not "+
			"be read", condChecksGreen, f.defaultBranch), rerr)
	}
	have := map[string]bool{}
	for _, r := range checks.CheckRuns {
		have[strings.ToLower(strings.TrimSpace(r.Name))] = true
	}
	for _, s := range checks.Statuses {
		have[strings.ToLower(strings.TrimSpace(s.Context))] = true
	}
	var missing []string
	for _, r := range required {
		if k := strings.ToLower(strings.TrimSpace(r)); k != "" && !have[k] {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — required check(s) never reported at this "+
			"head: %s", condChecksGreen, strings.Join(missing, ", ")), nil)
	}
	return nil
}

// checkLaneArmed is condition lane-armed: the desk kill switch is off, the kill signal is
// not holding, and the repo's daily cap is not reached.
func checkLaneArmed(o *opts, cfg deskkit.AutoLaneConfig) error {
	if err := deskkit.Guard(); err != nil {
		return err
	}
	ks := deskkit.ReadAutoLaneKillSignal(o.fpyFile, cfg.FPYFloor)
	fmt.Fprintln(o.out, ks.Line)
	if !ks.Armed() {
		return deskkit.Refused(fmt.Sprintf("refused: %s — %s", condLaneArmed, ks.Line))
	}
	entries, err := deskkit.LoadEntries()
	if err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: %s — the audit log could not be read, so the "+
			"daily cap cannot be counted", condLaneArmed), err)
	}
	n := deskkit.AutoLaneMergesOn(entries, o.repo, laneNow())
	fmt.Fprintf(o.out, "daily-cap: %d/%d lane merge(s) today for %s\n", n, cfg.DailyCap, o.repo)
	if n >= cfg.DailyCap {
		return deskkit.Refused(fmt.Sprintf("refused: daily-cap — %d of %d lane merges already made today for %s",
			n, cfg.DailyCap, o.repo))
	}
	o.say("%s OK", condLaneArmed)
	return nil
}

// --- rendering + audit ----------------------------------------------------------------------

func admitLine(a deskkit.AutoLaneAdmit) string {
	if a.Admitted {
		return "ok"
	}
	return "tripped (" + strings.Join(a.Tripwires, ", ") + ")"
}

func scoreLine(s deskkit.AutoLaneScore, line int) string {
	fired := "none"
	if len(s.Fired) > 0 {
		fired = strings.Join(s.Fired, ", ")
	}
	return fmt.Sprintf("%d (line %d; fired: %s)", s.Score, line, fired)
}

func unreadableCond(v verdict) string {
	for _, u := range v.score.Unreadable {
		if u == "checks" {
			return "checks"
		}
	}
	return condScore
}

func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), want) {
			return true
		}
	}
	return false
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func logLine(o *opts, verb, result, detail string) {
	var prp *int
	if o.pr > 0 {
		n := o.pr
		prp = &n
	}
	var hp *string
	if o.head != "" {
		h := o.head
		hp = &h
	}
	if err := deskkit.Log(deskkit.Entry{
		Tool: toolName, Verb: verb, Repo: o.repo, PR: prp, HeadSHA: hp, Result: result,
		Detail: detail, Title: fmt.Sprintf("%s#%d", o.repo, o.pr),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "%s: WARNING: could not write audit line: %v\n", toolName, err)
	}
}

func logEject(o *opts, result, names string) { logLine(o, deskkit.AutoLaneVerbEject, result, names) }

func auditConfigRefused(o *opts, problem string) {
	logLine(o, deskkit.AutoLaneVerbConfig, deskkit.ResultRefused, firstLine(problem))
}

// audit writes the verb-level line. Only a real lane merge could ever write
// `autolane:merge result=ok` — the line the daily cap counts — and this release has none.
func audit(o *opts, err error) {
	verb := map[string]string{
		verbCheck: deskkit.AutoLaneVerbCheck, verbRecompute: "autolane:recompute", verbMerge: deskkit.AutoLaneVerbMerge,
	}[o.verb]
	if verb == "" {
		verb = "autolane:unknown"
	}
	result, detail := deskkit.ResultOK, "ok"
	switch {
	case err == nil && o.dryRun:
		result, detail = deskkit.ResultDryRun, "dry-run: every condition held"
	case err == nil:
		if o.verb == verbMerge {
			// Unreachable in this release (merge without --dry-run always refuses), and kept
			// fail-safe: never a result the daily cap would count.
			result = deskkit.ResultRefused
		}
	case deskkit.ExitCodeOf(err) == deskkit.ExitRefused:
		result, detail = deskkit.ResultRefused, firstLine(err.Error())
	case o.wrote:
		result, detail = deskkit.ResultUnverifiable, firstLine(err.Error())
	default:
		result, detail = deskkit.ResultUnwritten, firstLine(err.Error())
	}
	logLine(o, verb, result, detail)
}
