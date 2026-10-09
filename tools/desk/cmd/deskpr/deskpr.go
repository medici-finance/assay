package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// deskprStderr is the seam for warnIfConflicting's advisory output and run()'s terminal
// error. Production writes to
// os.Stderr; tests redirect it to a buffer to assert on the warning text without
// capturing the real process stream.
var deskprStderr io.Writer = os.Stderr

// pollAttempts / pollSleep pace warnIfConflicting's wait for GitHub's asynchronous
// `mergeable` computation to settle out of UNKNOWN (#1264). GitHub returns UNKNOWN for a
// just-created/updated PR until a background job computes the test-merge, so a single read
// can miss a real CONFLICTING as a transient UNKNOWN. They are package vars so tests can
// shrink the sleep to a no-op and set a deterministic attempt count.
var (
	pollAttempts = 6
	pollSleep    = func() { time.Sleep(700 * time.Millisecond) }
)

// getwd is the seam for the tool's working directory (the worktree it runs in).
// Production uses os.Getwd; tests point it at a scratch worktree fixture without
// os.Chdir (which would race parallel processes).
var getwd = os.Getwd

const maxBodyBytes = 16 * 1024 // body cap

var (
	baseRe = regexp.MustCompile(`^[A-Za-z0-9._][A-Za-z0-9._/-]*$`)
	pullRe = regexp.MustCompile(`/pull/(\d+)`)
)

// gitFacts is the positively-verified state preflight establishes before any write.
type gitFacts struct {
	dir           string
	branch        string
	defaultBranch string
	defaultRef    string // fully-qualified remote-tracking ref, e.g. "refs/remotes/origin/main" (unambiguous by construction, #840)
	repo          string // owner/name
	head          string // HEAD sha
	originURL     string // origin's fetch URL as git resolves it (effectiveOriginURL)
}

// auditCtx accumulates the fields for the ONE audit line every invocation emits
// (audit every path). finalize is deferred so exactly one line is written no
// matter which branch returns.
type auditCtx struct {
	verb          string
	repo          string
	pr            *int
	head          string
	detail        string
	successResult string // ResultOK unless a noop set it to ResultNoop
}

func (a *auditCtx) log(result, detail string) {
	var headp *string
	if a.head != "" {
		h := a.head
		headp = &h
	}
	_ = deskkit.Log(deskkit.Entry{
		Tool:       "deskpr",
		Verb:       a.verb,
		Result:     result,
		Detail:     detail,
		Repo:       a.repo,
		PR:         a.pr,
		HeadSHA:    headp,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
	})
}

// finalize maps the terminal error (or success) to exactly one audit result.
func (a *auditCtx) finalize(err error) {
	// A help screen is not an invocation of the verb, so it appends NO row. The ledger this
	// would land in is append-only, never rotated, and counted per tool for the write budget
	// and the circuit breaker (deskkit/audit.go, ratelimit.go) — see helprequest.go.
	if deskkit.IsHelpRequest(err) {
		return
	}
	if err == nil {
		result := a.successResult
		if result == "" {
			result = deskkit.ResultOK
		}
		a.log(result, a.detail)
		return
	}
	var result string
	switch deskkit.ExitCodeOf(err) {
	case deskkit.ExitDisabled:
		result = deskkit.ResultDisabled
	case deskkit.ExitRateLimited:
		result = deskkit.ResultRateLimited
	case deskkit.ExitRefused:
		result = deskkit.ResultRefused
	default:
		result = deskkit.ResultUnverifiable
	}
	a.log(result, err.Error())
}

// explainScanRefusal is the deferred tail every deskpr verb registers after flag parse:
// when --explain was passed and the terminal error carries a secret-scan ScanFinding, it
// prints one scan-explain line (rule id + line number, never the offending span). Without
// the flag, and on any non-scan error, it is a no-op — so the verb's default output is
// byte-identical to before --explain existed.
func explainScanRefusal(explain bool, err error) {
	deskkit.MaybeExplain(os.Stderr, explain, err)
}

// cmdCreate implements `deskpr create`. Flow: verify
// preconditions → secret-scan → idempotency → push (plain, never --force) →
// `gh pr create --draft` → print URL.
func cmdCreate(args []string) (err error) {
	ac := &auditCtx{verb: "create"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder)) // suppress flag's own output; we craft messages
	title := fs.String("title", "", "PR title (required)")
	bodyFile := fs.String("body-file", "", "path to a file containing the PR body")
	bodyMin := fs.String("body-min", "", "one-line PR body (alternative to --body-file)")
	base := fs.String("base", "main", "base branch")
	root := fs.String("root", ".", "repo root the Brief: trailer resolves against (docs/streams under it)")
	scanOverride := fs.String(deskkit.ScanOverrideFlag, "", "override a secret-scan refusal, stating why; writes an audit row (tool, surface digest, reason, identity)")
	explain := fs.Bool("explain", false, "on a secret-scan refusal, also print a scan-explain line naming the rule id and line number (never the offending span)")
	decided := fs.String("decided", "", "path to a file declaring desk-taken decisions (decision:/alternative:/cost: triples, one item per numbered line) — writes the `## Desk-decided` block into the body and applies the desk-decided label; a PR that only transcribes recorded rulings passes none of this")
	check := fs.Bool("check", false, "run every LOCAL gate (flags, branch state, the Brief:/Authors:/Issue: trailer, the secret scan, the public-repo self-containment scan, the push-transport gate, the publish-identity gate) and stop BEFORE minting a token or opening any connection — exit 0 only when every local gate passed; a category this cannot decide offline is reported, by name, as not checked")
	if perr := fs.Parse(args); perr != nil {
		// TIER TWO: `-h`/`--help` in any spelling reaches flag.Parse as flag.ErrHelp.
		// A help screen is not a refusal and writes no audit row — the finalizer
		// skips it (deskkit/helprequest.go).
		if deskkit.IsHelpRequest(perr) {
			return deskkit.ErrHelpRequested
		}
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	defer func() { explainScanRefusal(*explain, err) }()
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: create takes no positional arguments")
	}
	// Validate the override BEFORE anything else runs, so a malformed one refuses in
	// milliseconds rather than after a token mint and a remote read.
	if *scanOverride != "" {
		if verr := deskkit.ValidateScanOverride(*scanOverride); verr != nil {
			return verr
		}
	}
	if strings.TrimSpace(*title) == "" {
		return deskkit.SchemaRefusal("deskpr", "--check", "refused: --title is required")
	}
	if (*bodyFile == "") == (*bodyMin == "") {
		return deskkit.Refused("refused: provide exactly one of --body-file or --body-min")
	}
	if !baseRe.MatchString(*base) {
		return deskkit.Refused("refused: --base must be a plain branch name")
	}
	body, berr := readBody(*bodyFile, *bodyMin)
	if berr != nil {
		return berr
	}
	// --decided (attention-budget/19): fold the desk-decided block into the body BEFORE any
	// scan or network call — an empty file or an item missing a field refuses here (exit 5),
	// with no PR call made. See decided.go for why create refuses on a hand-written heading
	// where edit instead replaces one in place.
	body, dberr := injectDecidedBlock(body, *decided, "create")
	if dberr != nil {
		return dberr
	}
	if serr := deskkit.HandleScanRefusal(deskkit.ScanOverride{
		Tool: "deskpr", Verb: "create", Reason: *scanOverride,
		Surface: "PR body", Content: body,
	}, deskkit.ScanSurface("PR body", body)); serr != nil {
		return serr
	}

	dir, gerr := getwd()
	if gerr != nil {
		return deskkit.Unverifiable("cannot resolve working directory", gerr)
	}

	// example-stream/02: the PR→brief link is a data edge, not a convention. Refuse a
	// body without exactly one trailer — BEFORE any network call (getwd/preflight are
	// local; token mint and PR listing come after) — and resolve the brief under --root.
	trailerIssue, terr := requireTrailer(body, *root, dir)
	if terr != nil {
		return terr
	}
	facts, perr := preflight(dir, *base)
	if perr != nil {
		return perr
	}
	ac.repo, ac.head = facts.repo, facts.head

	// PUSH-destination gate (#1623): git's own resolved push URL list must be exactly one https
	// URL naming facts.repo. It asks git where the push will actually go (pushurl from every
	// scope, insteadOf / pushInsteadOf, multi-valued lists) and refuses anything else,
	// fail-closed, naming each value's scope and a worktree-scoped remedy. It runs FIRST, so an
	// SSH destination is refused here with that remedy; the transport gate below still runs for
	// its https credential-helper NOTICE.
	if derr := pushDestinationGate(facts.dir, "create", facts.repo, facts.originURL); derr != nil {
		return derr
	}

	// #1339: a `Brief:` trailer on a branch that only AUTHORS the brief is refused before
	// anything leaves the machine — it would make the brief read as delivered on merge. The
	// mirror case (review F1 on #1641) is also refused here: an `Authors:` trailer on a
	// branch whose diff is NOT provably authoring-only for every listed id, since `Authors:`
	// switches off the security lane's brief-declared risk term for a `Brief:` PR. Local (git
	// only), so it is part of --check. create only: update/edit act on an existing PR whose
	// trailer is immutable, and refusing them would strand that PR.
	if aerr := authoringTrailerGate(body, facts.dir, "refs/remotes/origin/"+*base); aerr != nil {
		return aerr
	}

	// PUSH-transport custody gate (#861). An SSH push from a bot session goes out under
	// whatever key this machine's agent holds — a human's — so the forge records the human
	// as the branch creator and the App's permission envelope is bypassed while every
	// commit still reads as the App's. Refuse before the mint and before any network call.
	if terr := pushTransportGate(facts.dir, "create"); terr != nil {
		return terr
	}

	// PUBLISH-identity gate (#1490 lane B). Refuse before the push if any commit the push
	// would publish (refs/remotes/origin/<base>..HEAD) is authored or committed by an
	// identity other than this session role's bound bot. A worktree that acquired a stale
	// `user.*` — from a shared checkout, a manual `git worktree add`, an editor's git —
	// would otherwise publish commits attributed to the wrong actor; the provisioning fixes
	// stop new such worktrees, this stops the publish from any route. Local (git + roster),
	// so it runs before the token mint and is part of --check.
	//
	// create offers NO remote tip (#1967): it opens a NEW change, so there is no forge-held
	// PR head to anchor a narrower range on, and the forge surface has no branch-head read to
	// confirm a local remote-tracking ref against. The whole range the new PR introduces is
	// judged — the fail-closed choice. update, which has a PR head, narrows.
	if ierr := publishIdentityGate(facts.dir, *base, ""); ierr != nil {
		return ierr
	}

	// seatbelt: scan title, branch, and the diff-vs-default before any push.
	if scanErr := scanWrite(facts, *title, "create", *scanOverride); scanErr != nil {
		return scanErr
	}

	// #203: the PUBLIC-REPO SELF-CONTAINMENT scan. It runs HERE rather than beside the
	// secret scan above because it needs the target repo, which only preflight establishes
	// — the secret scan's question ("is there a credential in this text") is repo-
	// independent, this one is not. It is a no-op on a known-private repo and on an
	// unconfigured roster (deskkit.SelfContainApplies), so the create path on a private
	// repo is byte-for-byte what it was.
	//
	// The verdict routes through HandleScanRefusal like every other scan on this path, so
	// the refusal advertises the SAME audited override rather than introducing a second
	// bypass a worker would have to learn — and an override taken here writes its row
	// before the push.
	scOpts := deskkit.SelfContainOpts{Repo: facts.repo, NumberHint: trailerIssue}
	for _, sc := range []struct {
		surface string
		content []byte
	}{
		{"PR body", body},
		{"PR title", []byte(*title)},
	} {
		if serr := deskkit.HandleScanRefusal(deskkit.ScanOverride{
			Tool: "deskpr", Verb: "create", Repo: facts.repo, Reason: *scanOverride,
			Surface: sc.surface, Content: sc.content,
		}, deskkit.SelfContainCheck(sc.surface, sc.content, scOpts)); serr != nil {
			return serr
		}
	}

	// --check stops HERE, before the token mint and before any forge call. Every gate
	// above it is local: flags, the secret scan (title/branch/diff, plus the body scan
	// earlier), the Brief:/Authors:/Issue: trailer, branch state (preflight), the push-transport
	// gate, and the public-repo self-containment scan (its bare-#N category already
	// reports itself "not checked" on stderr via SelfContainOpts.Notices when no local
	// hint is available — see selfcontain.go — so nothing here rounds that up to a pass).
	// It mints no token, opens no connection, and pushes nothing: a real `create` run
	// past this point can still fail on remote state (an existing open PR, a red rate
	// limit, a non-authorized public repo), which --check never claims to have checked.
	// The publish-identity gate (#1490) ran above with the rest of the local gates.
	if *check {
		ac.successResult = deskkit.ResultDryRun
		ac.detail = "check: every local gate passed"
		fmt.Println("check: ok — every local gate passed; no connection opened, nothing pushed. " +
			"Not checked (needs the forge, not run here): an existing open PR on this branch, " +
			"the outward-write rate limit, and the public-repo authorization gate.")
		return nil
	}

	// Mint the session-role App installation token and resolve the forge that serves this
	// repo under that App's custody. Every change read and write below goes through the
	// resolved backend; there is no ambient-identity fallback (the retired --as-app path).
	if merr := mintWorkerToken(facts.repo); merr != nil {
		return deskkit.Unverifiable("cannot mint the App token", merr)
	}
	fg, fr, ferr := forgeForFn(facts.repo)
	if ferr != nil {
		return ferr
	}

	// idempotency (#140/#148 duplicate-PR class): an open PR already on this head
	// branch → print its URL, noop, exit 0. Checked BEFORE any push or create.
	if existing, lerr := fg.OpenChangeForBranch(fr, facts.branch); lerr != nil {
		return deskkit.Unverifiable("cannot check for an existing PR on the branch", lerr)
	} else if existing != nil {
		ac.pr = &existing.Number
		ac.successResult = deskkit.ResultNoop
		ac.detail = "open PR already exists " + existing.URL
		fmt.Printf("noop: open PR already exists for %s: %s\n", facts.branch, existing.URL)
		return nil
	}

	// Outward-write rate limit — checked immediately before the push.
	//
	// REPO-WIDE, at the per-PR cap, because a create's own audit line records the number of
	// the PR it is about to make (`ac.pr = &n`, below) — a new number every time. Scoping
	// the gate on a PR number therefore reads a bucket this call site can never fill: an
	// earlier fix aimed it at the repo's UNNUMBERED bucket, which left creates on the
	// 100/hr per-repo tier while looking like a 10/hr cap, and inverted the meter so that
	// only FAILED creates (which record no number) accumulated (#439, third
	// review). The repo-wide scope counts every line this tool writes on this repo, so the
	// bucket the gate reads is a superset of wherever the write lands — see
	// deskkit.AllowWriteRepoWide.
	//
	// Held at the per-PR cap, not the per-repo one: this is the verb behind the
	// PR-flood risk and it was hard-capped at 10/hr before the tiers existed.
	if werr := deskkit.AllowWriteRepoWide("deskpr", facts.repo); werr != nil {
		return werr
	}

	// Public-repo gate: refuse an outward write unless the repo is authorized.
	// The authorization is repository-scoped (a listed `:public` allowed-repos entry,
	// or private) — see deskkit.PublicRepoGate. A create no longer needs an issue/PR
	// number: the former per-item `+1` on `trailerIssue` is gone, which is exactly what
	// makes the FIRST pull request on a listed public repo openable (a brief-carrying
	// create has no issue number and used to fail closed here). `trailerIssue` is still
	// resolved above for the trailer/self-containment hint; it is simply no longer passed
	// to the gate.
	owner, name := splitOwnerRepo(facts.repo)
	// The gate's visibility read goes through the SAME forge backend already resolved above
	// (forgeForFn's fg) — never a second, hardcoded GitHub-only client (assay#1054): a
	// GitLab-resolved repo must have its visibility answered by GitLab's own API, not
	// GitHub's, and fg is already whichever backend the resolver picked.
	fetcher := deskkit.ForgeRepoInfoFetcher{Forge: fg}
	if gerr := publicRepoGateFn(fetcher, owner, name); gerr != nil {
		return gerr
	}

	// Plain push, in-process (push.go): the refspec is built here with no "+" and no Force,
	// so a push that would need force is refused by the protocol; no caller flag reaches it.
	if pushErr := pushFn(pushSpec{
		dir: facts.dir, repo: facts.repo, originURL: facts.originURL,
		srcRef: "refs/heads/" + facts.branch, dstBranch: facts.branch, setUpstream: true,
	}); pushErr != nil {
		return pushErr
	}

	// On-behalf-of trailer (multi-principal/01), appended to the body sent to the forge
	// only — every gate above (the Brief:/Authors:/Issue: trailer parse, the secret/self-contain
	// scans) already ran against the caller-supplied body, so this cannot change what any
	// of them saw.
	prBody, oerr := deskkit.AppendOnBehalfOf(body, "", facts.repo)
	if oerr != nil {
		return oerr
	}

	// CreateHeldDraftChange opens the change as a DRAFT — the frozen property of the seam;
	// there is no path on which it opens ready — and then opens the desk's merge-hold marker
	// thread on it (the forge-gitlab merge-hold brief): a resolvable discussion thread that
	// blocks GitLab's merge button (only_allow_merge_if_all_discussions_are_resolved) until the
	// reviewer's approve verdict releases it at the current head. GitHub returns the typed
	// not-applicable (its twin control is server-side branch protection), a no-op there. The
	// pairing lives in deskkit so every verb that opens a change gets it (#2254). A hold-open
	// failure is LOUD: the change already exists, and an operator who is not told it is
	// missing its gate would not find out until a ready-flip refuses for a reason that reads
	// like a different problem. The body goes straight to the backend, so there is no temp
	// file and no `--body-file` argv any more.
	ref, cErr := deskkit.CreateHeldDraftChange(fg, fr, deskkit.DraftChangeInput{
		Title: *title, Body: string(prBody), Head: facts.branch, Base: *base,
	})
	if cErr != nil {
		if ref == nil {
			return deskkit.Unverifiable("create draft change failed", cErr)
		}
		if ref.Number > 0 {
			n := ref.Number
			ac.pr = &n
		}
		return cErr
	}
	url := ref.URL
	detail := "created " + url
	if ref.Number > 0 {
		n := ref.Number
		ac.pr = &n
		// --decided (attention-budget/19): the block is already IN the body the create call
		// just published — this only mirrors it as the at-a-glance label. A PR with no
		// --decided applies no label at all (the transcribe-only shape stays byte-for-byte
		// what it was before this flag existed).
		if *decided != "" {
			if lerr := applyDeskDecidedLabel(fg, fr, n); lerr != nil {
				return deskDecidedLabelFailure(url, "the PR was opened with its Desk-decided block", lerr)
			}
		}
		// Post-create mergeable check (#770): a PR GitHub reports CONFLICTING gets zero
		// pull_request runs at its head — indistinguishable, on the audit line or any
		// board, from "checks still pending" until something names the mergeable state
		// specifically. Advisory only: this can never turn a create that already
		// succeeded into a reported failure.
		detail += warnIfConflicting(fg, fr, n)
	}
	ac.detail = detail
	fmt.Println(url)
	return nil
}

// warnIfConflicting reads a just-created PR's mergeable status via `gh pr view` and
// prints a loud WARNING to deskprStderr when GitHub reports CONFLICTING, returning a
// non-empty suffix for the audit detail line in that case (empty otherwise, including on
// a read/parse failure — see below).
//
// Why this matters (#770): GitHub skips `pull_request` Actions runs on a PR whose merge
// state is CONFLICTING, so a conflicted PR sits at zero check-runs indefinitely. That
// zero reads identically to "checks haven't started yet" on the pr-review-desk and any
// board, so a conflicted PR silently stalls — exactly what happened to #749/#750, which
// were correct code stuck only on a rebase no one was told to do.
//
// GitHub computes `mergeable` asynchronously, so a freshly created/updated PR reports
// UNKNOWN until a background test-merge settles (#1264). A single read taken too early
// would miss a real CONFLICTING as a transient UNKNOWN, so this polls briefly
// (pollAttempts reads, pollSleep between) for the field to leave UNKNOWN before deciding.
//
// Advisory only: a transient `gh pr view` failure, an unparseable payload, or a value
// that never settles out of UNKNOWN must never turn an already-successful create/update
// into a reported failure, so every non-CONFLICTING path here is a stderr note (or
// silence), not a returned error — the PR already exists by the time this runs.
func warnIfConflicting(fg deskkit.Forge, fr deskkit.ForgeRepo, prNum int) string {
	mergeable := deskkit.MergeableUnknown
	for attempt := 0; ; attempt++ {
		pr, err := fg.GetPullRequest(fr, prNum)
		if err != nil {
			fmt.Fprintf(deskprStderr, "deskpr: WARNING could not read mergeable status for %s#%d — %v\n", fr.Slug(), prNum, err)
			return ""
		}
		mergeable = pr.Mergeable
		// Settled (MERGEABLE / CONFLICTING) or out of attempts: stop polling.
		if mergeable != deskkit.MergeableUnknown || attempt >= pollAttempts-1 {
			break
		}
		pollSleep()
	}
	if mergeable == deskkit.MergeableUnknown {
		fmt.Fprintf(deskprStderr, "deskpr: WARNING mergeable status for %s#%d did not settle out of UNKNOWN after "+
			"%d polls — GitHub had not finished computing it; re-check the PR's merge state before relying on CI (#1264)\n",
			fr.Slug(), prNum, pollAttempts)
		return ""
	}
	if mergeable != deskkit.MergeableConflicting {
		return ""
	}
	fmt.Fprintf(deskprStderr, "deskpr: WARNING %s#%d is CONFLICTING — the forge will not run "+
		"pull_request checks at this head; merge/rebase the base branch into this PR before expecting CI to fire "+
		"(#770)\n", fr.Slug(), prNum)
	return " — CONFLICTING, CI will not run until resolved"
}

// cmdUpdate implements `deskpr update`: a follow-up push of the current branch to its
// EXISTING open PR — draft OR ready-flipped (the fix→re-review hot path, and #788's
// keep-an-approved-PR-current path). It refuses if no open PR exists for the branch;
// that same refusal covers closed and merged PRs, because the listing is --state open.
// The draft/ready distinction is intentionally NOT a gate: the author-owns-branch (head
// match), bodycheck, rate-limit and public-repo guards apply identically either way. No
// PR is ever created here.
func cmdUpdate(args []string) (err error) {
	ac := &auditCtx{verb: "update"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	scanOverride := fs.String(deskkit.ScanOverrideFlag, "", "override a secret-scan refusal, stating why; writes an audit row (tool, surface digest, reason, identity)")
	root := fs.String("root", ".", "repo root the Brief: trailer resolves against (docs/streams under it)")
	explain := fs.Bool("explain", false, "on a secret-scan refusal, also print a scan-explain line naming the rule id and line number (never the offending span)")
	check := fs.Bool("check", false, "run every LOCAL gate (flags, branch state, the secret scan, the push-transport gate, the publish-identity gate) and stop BEFORE minting a token or opening any connection; the Brief:/Authors:/Issue: trailer lives on the EXISTING PR's forge-held body and is reported not checked, by name, rather than skipped silently")
	prNum := fs.Int("pr", 0, "push HEAD to THIS open PR's head branch instead of the PR whose head branch is this worktree's branch (#2085): for a worktree that cannot be on the PR's head branch (git allows one worktree per branch). HEAD must descend from the PR's current head commit")
	remoteBranch := fs.String("branch", "", "push HEAD to the open PR whose head branch is this REMOTE branch name instead of this worktree's branch (#2085); same admission rule as --pr")
	if perr := fs.Parse(args); perr != nil {
		// TIER TWO: `-h`/`--help` in any spelling reaches flag.Parse as flag.ErrHelp.
		// A help screen is not a refusal and writes no audit row — the finalizer
		// skips it (deskkit/helprequest.go).
		if deskkit.IsHelpRequest(perr) {
			return deskkit.ErrHelpRequested
		}
		return deskkit.Refused("refused: bad flags: " + perr.Error())
	}
	defer func() { explainScanRefusal(*explain, err) }()
	if fs.NArg() != 0 {
		return deskkit.Refused("refused: update takes no arguments")
	}
	if *prNum < 0 {
		return deskkit.Refused(fmt.Sprintf("refused: --pr must be a positive PR number, got %d", *prNum))
	}
	if *prNum > 0 && *remoteBranch != "" {
		return deskkit.Refused("refused: --pr and --branch both name the target PR — give one")
	}
	if *remoteBranch != "" && !baseRe.MatchString(*remoteBranch) {
		return deskkit.Refused("refused: --branch " + fmt.Sprintf("%q", *remoteBranch) + " is not a valid branch name")
	}
	// override is true when the target PR is NAMED (--pr / --branch) rather than found by the
	// worktree's own branch. Only then does the push destination come from the PR's head ref.
	override := *prNum > 0 || *remoteBranch != ""
	if isDefaultName(*remoteBranch) {
		return deskkit.Refused("refused: --branch names the default branch (" + *remoteBranch + ") — deskpr only pushes feature branches")
	}
	if *scanOverride != "" {
		if verr := deskkit.ValidateScanOverride(*scanOverride); verr != nil {
			return verr
		}
	}
	dir, gerr := getwd()
	if gerr != nil {
		return deskkit.Unverifiable("cannot resolve working directory", gerr)
	}
	// update has no --base flag: it refreshes an EXISTING PR's body, so the ahead-count
	// stays pinned to the repo default (origin/HEAD) exactly as before — an empty base
	// selects that default inside preflight.
	// With --pr N the PR is named, so a detached HEAD is admitted (the local branch name plays
	// no part in finding the PR or choosing the destination).
	facts, perr := preflightMode(dir, "", *prNum > 0, detachedRefusal)
	if perr != nil {
		return perr
	}
	ac.repo, ac.head = facts.repo, facts.head
	// --branch B: the REMOTE branch is known offline, so every offline stage below judges B
	// where the default path judges the local branch name — the secret scan of the branch name
	// and the publish-identity anchor (#2085). With --pr N the head ref is only known from the
	// forge, so the offline stage keeps the local branch name for the scan and judges the
	// whole range; the live stage below re-judges both against the PR's own head ref.
	if *remoteBranch != "" {
		facts.branch = *remoteBranch
	}

	// PUSH-destination gate (#1623): git's own resolved push URL list must be exactly one https
	// URL naming facts.repo. It asks git where the push will actually go (pushurl from every
	// scope, insteadOf / pushInsteadOf, multi-valued lists) and refuses anything else,
	// fail-closed, naming each value's scope and a worktree-scoped remedy. It runs FIRST, so an
	// SSH destination is refused here with that remedy; the transport gate below still runs for
	// its https credential-helper NOTICE.
	if derr := pushDestinationGate(facts.dir, "update", facts.repo, facts.originURL); derr != nil {
		return derr
	}

	// PUSH-transport custody gate (#861) — same reason as create: this verb pushes.
	if terr := pushTransportGate(facts.dir, "update"); terr != nil {
		return terr
	}

	// PUBLISH-identity gate (#1490 lane B) — same reason as create. update has no --base, so
	// the published range is measured against the repo default (origin/HEAD), exactly the
	// base preflight resolved for the ahead-count.
	//
	// #1967: update pushes onto a PR head that may already carry commits by ANOTHER trusted
	// App (a mixed-author PR). Those are already on the remote; re-judging them refused every
	// update to such a PR. This OFFLINE stage (it is what --check runs) anchors the range on
	// the local remote-tracking ref for the branch — absent, unresolvable, or not an ancestor
	// of HEAD, it falls back to the whole range. It is an estimate of the remote, so the live
	// stage below re-judges against the forge's own PR head before anything is pushed.
	//
	// #2432: with --pr N the PR's head branch is unknown offline, so this stage has no tip and
	// would judge the whole range from the default branch — and refuse, before the live stage
	// ran, every update to a PR whose published head already carries another App's commit. A
	// real `update --pr N` therefore skips this stage: the live stage below always runs before
	// the push, against the head the forge reports, which HEAD must descend from and the push
	// destination must hold. Under --check there is no live stage, so the whole range is
	// judged here (fail closed) and a refusal names the offline way to the narrow judgement.
	offlineTip := remoteTrackingTip(facts.branch)
	if *prNum > 0 {
		offlineTip = "" // head ref unknown offline: judge the whole range, fail closed
	}
	if *prNum == 0 || *check {
		if ierr := publishIdentityGate(facts.dir, facts.defaultBranch, offlineTip); ierr != nil {
			if *prNum > 0 {
				return prCheckWideNote(ierr)
			}
			return ierr
		}
	}

	if scanErr := scanWrite(facts, "", "update", *scanOverride); scanErr != nil {
		return scanErr
	}

	// --check stops HERE, before the token mint and before any forge call. Every gate
	// above it is local: flags, branch state (preflight), the push-transport gate, and
	// the secret scan (branch name + diff). The Brief:/Authors:/Issue: trailer check is NOT run:
	// update pushes commits to an EXISTING PR and validates the trailer against that
	// PR's CURRENT body, which lives on the forge — there is no local copy to check it
	// against, so it is reported as not checked, by name, rather than silently skipped
	// or (worse) rounded up to a pass.
	if *check {
		ac.successResult = deskkit.ResultDryRun
		ac.detail = "check: every local gate passed"
		fmt.Println("check: ok — every local gate passed; no connection opened, nothing pushed. " +
			"Not checked (needs the forge, not run here): the Brief:/Authors:/Issue: trailer on the existing " +
			"PR's current body, whether an open PR exists for this branch, the outward-write rate " +
			"limit, the public-repo authorization gate, and the publish-identity range against the " +
			"PR's LIVE head (judged here against the local remote-tracking ref — with --pr N, the whole range " +
			"from the default branch; the real run re-judges it against the live head).")
		return nil
	}

	if merr := mintWorkerToken(facts.repo); merr != nil {
		return deskkit.Unverifiable("cannot mint the App token", merr)
	}
	fg, fr, ferr := forgeForFn(facts.repo)
	if ferr != nil {
		return ferr
	}

	var pr *deskkit.PullRequest
	if *prNum > 0 {
		// --pr N (#2085): read the named PR directly. It must be OPEN; the lineage rule that
		// replaces the branch-name compare is enforced below, once the head ref is known.
		named, nerr := fg.GetPullRequest(fr, *prNum)
		if nerr != nil {
			return deskkit.Unverifiable(fmt.Sprintf("cannot read PR #%d", *prNum), nerr)
		}
		if !strings.EqualFold(strings.TrimSpace(named.State), "open") {
			return deskkit.Refused(fmt.Sprintf("refused: PR #%d is %s, not OPEN — deskpr update only pushes to open PRs", *prNum, named.State))
		}
		pr = named
	} else {
		var lerr error
		pr, lerr = fg.OpenChangeForBranch(fr, facts.branch)
		if lerr != nil {
			return deskkit.Unverifiable("cannot check for an open PR on the branch", lerr)
		}
		if pr == nil {
			return deskkit.Refused("refused: no open PR for " + facts.branch + " — run `deskpr create` first")
		}
	}
	// #788: a ready-flipped (non-draft) OPEN PR is accepted here too. The draft-only
	// refusal that used to sit here was an artifact of update being written for the
	// draft-PR workflow first; it left approved PRs that go stale against a moving main
	// with no sanctioned push path. Every other guard (head-owns-branch above, bodycheck
	// in scanWrite, AllowWrite rate limit, and the public-repo gate below) is unchanged,
	// so lifting the draft distinction does not widen what update may push — only which
	// open PR states it will push to.
	ac.pr = &pr.Number

	// example-stream/02: an update pushes to a PR whose BODY lives on the forge, so the
	// trailer check reads it from the PR — via the authoritative single-change read
	// (GetPullRequest), the pairing the brief names for OpenChangeForBranch. A PR whose body
	// lacks the link line refuses here (exit 5, message names the line to add) — the worker
	// edits the body, then re-runs update. This is the migration-window behavior for
	// pre-trailer PRs.
	full, berr := fg.GetPullRequest(fr, pr.Number)
	if berr != nil {
		return deskkit.Unverifiable("cannot read PR body for trailer check", berr)
	}
	// update ignores the trailer's issue number: the gate below is asked about the
	// PR being updated (pr.Number), which is the reactions surface for an update.
	if _, terr := requireTrailer([]byte(full.Body), *root, dir); terr != nil {
		return terr
	}

	// Named-PR push destination (#2085). When the PR was named (--pr / --branch), the branch
	// pushed to is the PR's OWN head branch as the forge reports it — never the worktree's
	// local branch name, which may differ because git lets only one worktree hold a branch.
	// The checkout is admitted by LINEAGE, not by name: HEAD must descend from the PR's
	// current head commit, so the push is a fast-forward of that PR and nothing else (git
	// would refuse a non-fast-forward anyway; this names the reason). Fail closed on a head
	// the forge did not report or this checkout has not fetched.
	pushDest := facts.branch
	if override {
		if full.HeadRef == "" {
			return deskkit.Unverifiable("the forge did not report PR #"+strconv.Itoa(pr.Number)+"'s head branch — cannot choose a push destination", nil)
		}
		pushDest = full.HeadRef
		if isDefaultName(pushDest) {
			return deskkit.Refused("refused: PR #" + strconv.Itoa(pr.Number) + "'s head branch is the default branch (" + pushDest + ") — deskpr only pushes feature branches")
		}
		if *remoteBranch != "" && pushDest != *remoteBranch {
			return deskkit.Refused(fmt.Sprintf("refused: the open PR #%d's head branch is %q, not %q", pr.Number, pushDest, *remoteBranch))
		}
		// The head ref is a bare branch NAME and origin is the BASE repository, so the push
		// lands on the PR only when the PR's head lives in the base repository. A fork PR's
		// head ref names a branch in the fork: pushing that name to origin would create a stray
		// base-repo branch, or fast-forward a same-named branch backing a DIFFERENT PR.
		if cerr := requireSameRepoHead(full.CrossRepo, pr.Number); cerr != nil {
			return cerr
		}
		if serr := scanWrite(&gitFacts{dir: facts.dir, branch: pushDest, defaultRef: facts.defaultRef, repo: facts.repo}, "", "update", *scanOverride); serr != nil {
			return serr
		}
		if lerr := requireDescendsFromPRHead(facts.dir, full.HeadSHA, pr.Number, pushDest, facts.defaultRef); lerr != nil {
			return lerr
		}
		// Second, independent layer: the push destination itself must hold the head ref at
		// the head commit the forge reported. This catches a forge read that is wrong or stale
		// (a head ref the base repository does not hold) on a signal git reads, not the forge.
		if rerr := requireRemoteHoldsHead(facts.dir, pushDest, full.HeadSHA, pr.Number); rerr != nil {
			return rerr
		}
	}

	// PUBLISH-identity gate, LIVE stage (#1967). The offline stage above anchored the range on
	// this checkout's remote-tracking ref, which can be stale; the forge's own PR head is the
	// authority on what the remote branch holds. Re-judge against it before anything is
	// pushed. A head the forge did not report, one not fetched here, or one this branch does
	// not descend from (a force-moved PR head) falls back to the whole range — fail closed.
	if ierr := publishIdentityGate(facts.dir, facts.defaultBranch, full.HeadSHA); ierr != nil {
		return ierr
	}

	// idempotency: this exact head already pushed to this PR → noop.
	if deskkit.AlreadyDone(facts.repo, pr.Number, facts.head, "update") {
		ac.successResult = deskkit.ResultNoop
		ac.detail = "head already pushed to " + pr.URL
		fmt.Printf("noop: %s already pushed to %s\n", shortSHA(facts.head), pr.URL)
		return nil
	}

	if werr := deskkit.AllowWrite("deskpr", facts.repo, pr.Number); werr != nil {
		return werr
	}

	// Public-repo gate: refuse an outward write unless the repo is authorized
	// (private, or a listed :public allowed-repos entry — see deskkit.PublicRepoGate).
	owner, name := splitOwnerRepo(facts.repo)
	// The gate's visibility read goes through the SAME forge backend already resolved above
	// (forgeForFn's fg) — never a second, hardcoded GitHub-only client (assay#1054): a
	// GitLab-resolved repo must have its visibility answered by GitLab's own API, not
	// GitHub's, and fg is already whichever backend the resolver picked.
	fetcher := deskkit.ForgeRepoInfoFetcher{Forge: fg}
	if gerr := publicRepoGateFn(fetcher, owner, name); gerr != nil {
		return gerr
	}

	spec := pushSpec{
		dir: facts.dir, repo: facts.repo, originURL: facts.originURL,
		srcRef: "refs/heads/" + facts.branch, dstBranch: facts.branch,
	}
	if override {
		// Explicit refspec: HEAD onto the PR's head branch. The worktree's own branch has a
		// different name, so its upstream config is left alone (git push <src>:<dst> never set
		// one); the tracking ref for the PR head branch is still recorded (push.go recordPushed).
		spec.srcRef, spec.dstBranch = "HEAD", pushDest
	}
	if pushErr := pushFn(spec); pushErr != nil {
		return pushErr
	}
	// Post-update mergeable check (#1264): the push moved the head, so GitHub recomputes
	// mergeability. A push that lands the PR in CONFLICTING gets zero pull_request runs at
	// the new head — the same silent stall the create path guards against — so warn loudly
	// here too. Advisory only: this can never turn an already-completed push into a failure.
	detail := "pushed to " + pr.URL
	detail += warnIfConflicting(fg, fr, pr.Number)
	ac.detail = detail
	fmt.Println(pr.URL)
	return nil
}

// requireDescendsFromPRHead is `deskpr update --pr/--branch`'s lineage rule (#2085): the
// checkout's HEAD must be the PR's current head commit or a descendant of it, so the push is a
// pure fast-forward of that PR's head branch. An unreported head, a head this checkout has not
// fetched, and a HEAD that does not descend from it all refuse — never a guess.
//
// It also refuses a PR head that is already contained in the default branch (defaultRef): such
// a PR carries nothing of its own, so a HEAD descending from it says nothing about whether this
// is the PR the caller meant — a mistyped --pr naming a stale PR would otherwise pass.
func requireDescendsFromPRHead(dir, headSHA string, prNum int, headRef, defaultRef string) error {
	if strings.TrimSpace(headSHA) == "" {
		return deskkit.Refused(fmt.Sprintf("refused: the forge reported no head commit for PR #%d — cannot prove HEAD descends from it", prNum))
	}
	r, oerr := gitcore.Open(dir)
	if oerr != nil {
		return deskkit.Unverifiable("cannot open the repository to check PR head lineage", oerr)
	}
	ok, aerr := r.IsAncestor(headSHA, "HEAD")
	if aerr != nil {
		return deskkit.Refused(fmt.Sprintf("refused: cannot prove HEAD descends from PR #%d's head commit %s (%v) — "+
			"`git fetch origin %s` and merge it into this branch, then re-run", prNum, shortSHA(headSHA), aerr, headRef))
	}
	if !ok {
		return deskkit.Refused(fmt.Sprintf("refused: HEAD does not descend from PR #%d's head commit %s on %q — "+
			"merge origin/%s into this branch (never rebase), then re-run", prNum, shortSHA(headSHA), headRef, headRef))
	}
	inDefault, derr := r.IsAncestor(headSHA, defaultRef)
	if derr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("cannot check whether PR #%d's head commit is already in %s", prNum, defaultRef), derr)
	}
	if inDefault {
		return deskkit.Refused(fmt.Sprintf("refused: PR #%d's head commit %s is already contained in %s — the PR carries "+
			"nothing of its own, so descending from it does not show this is the PR you meant; check the PR number",
			prNum, shortSHA(headSHA), defaultRef))
	}
	return nil
}

// prCheckWideNote adds, to a publish-identity refusal from `update --pr N --check`, why the
// whole range was judged and how to get the narrow judgement offline. The exit code and the
// gate's own message are kept; only a refusal gains the note.
func prCheckWideNote(err error) error {
	var de *deskkit.DeskError
	if !errors.As(err, &de) || de.Code != deskkit.ExitRefused {
		return err
	}
	out := *de
	out.Msg += " With --pr N the PR's head branch is unknown offline, so --check judged " +
		"the whole range; a real `update --pr N` judges only the commits the push adds beyond the PR's head. " +
		"To check that offline, name the head branch: `deskpr update --branch <head-branch> --check`."
	return &out
}

// requireSameRepoHead refuses a named-PR push unless the forge established that the PR's head
// branch lives in the BASE repository (#2085). A fork head is refused; an EMPTY answer (the
// forge did not say — e.g. a deleted fork) is could-not-check and refuses too, never read as
// "same". Same rule, same reading of the empty value, as deskmerge's eligibility gate.
func requireSameRepoHead(crossRepo string, prNum int) error {
	switch crossRepo {
	case deskkit.CrossRepoSame:
		return nil
	case deskkit.CrossRepoFork:
		return deskkit.Refused(fmt.Sprintf("refused: PR #%d's head branch lives in a fork — deskpr update pushes only "+
			"to a PR whose head branch is in the base repository", prNum))
	default:
		return deskkit.Unverifiable(fmt.Sprintf("could-not-check: PR #%d did not report which repository its head "+
			"branch lives in — deskpr update pushes only to a branch it has established is in the base repository", prNum), nil)
	}
}

// requireRemoteHoldsHead reads the push destination's own copy of refs/heads/<headRef> and
// requires it to be EXACTLY the head commit the forge reported (#2085). Absent → the push
// would CREATE a branch rather than update the PR; a different commit → the forge read is
// stale or names a different branch. Both refuse. The destination is read from the same push
// URL the push uses (already pinned to one URL by the push-destination gate).
func requireRemoteHoldsHead(dir, headRef, headSHA string, prNum int) error {
	dest, uerr := git(dir, "remote", "get-url", "--push", "origin")
	if uerr != nil || strings.TrimSpace(dest) == "" {
		return deskkit.Unverifiable("cannot resolve origin's push URL to read the PR head branch", uerr)
	}
	ref := "refs/heads/" + headRef
	out, lerr := git(dir, "ls-remote", "--refs", strings.TrimSpace(dest), ref)
	if lerr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("cannot read %s from the push destination", ref), lerr)
	}
	got := ""
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[1] == ref {
			got = f[0]
		}
	}
	if got == "" {
		return deskkit.Refused(fmt.Sprintf("refused: the push destination holds no %s, the head branch the forge reports "+
			"for PR #%d — pushing would create a new branch rather than update the PR", ref, prNum))
	}
	if !strings.EqualFold(got, strings.TrimSpace(headSHA)) {
		return deskkit.Refused(fmt.Sprintf("refused: the push destination's %s is at %s but the forge reports PR #%d's "+
			"head as %s — re-run once they agree (fetch, merge, never rebase)", ref, shortSHA(got), prNum, shortSHA(headSHA)))
	}
	return nil
}

// preflight verifies the preconditions IN-TOOL and returns the positively-verified
// facts. Ordering matters: the default-branch refusal fires on branch name alone
// (exit 5) BEFORE origin/HEAD is consulted, so a missing origin/HEAD can never mask a
// push to main; a detached HEAD or unreadable origin/HEAD is unverifiable (exit 6).
func preflight(dir, base string) (*gitFacts, error) {
	return preflightMode(dir, base, false, detachedRefusal)
}

// detachedRefusal is preflight's exit-6 message for a detached HEAD on create and update,
// which push the branch and so need one.
const detachedRefusal = "detached HEAD — check out a feature branch first"

// editDetachedRefusal is the same exit-6 refusal for `deskpr edit` WITHOUT --pr (#1901). A
// detached rework worker hits exactly this refusal, and for edit the remedy is not a branch:
// it is naming the PR, so the message points at --pr N.
const editDetachedRefusal = "detached HEAD — deskpr edit finds its PR by branch; name the PR with --pr N " +
	"(admitted when HEAD is exactly that PR's head commit, or when that PR is in this repository and " +
	"already carries a link trailer), or check out the PR's head branch"

// preflightMode is preflight with one switch: allowDetached admits a detached HEAD, recorded
// as branch "". Only `deskpr edit --pr N` passes true (#1901) — it pushes nothing and names
// its PR explicitly, and findEditTarget then decides whether this checkout may edit that PR
// (deskkit.CheckOwnPR, or a same-repo PR already carrying a link trailer — #2085). create and
// update push the branch and keep refusing.
// detachedMsg is the exit-6 message used when a detached HEAD is refused.
func preflightMode(dir, base string, allowDetached bool, detachedMsg string) (*gitFacts, error) {
	return preflightOpts(dir, base, allowDetached, detachedMsg, true)
}

// preflightOpts is preflightMode with the commits-ahead precondition switchable. requireAhead
// false is only for `deskpr edit --pr N` (#2085): edit pushes nothing, so the checkout it runs
// from needs no work of its own — a desk worktree with no commits ahead of the default branch
// may correct a body — and the ahead-count measures a branch the verb never publishes.
func preflightOpts(dir, base string, allowDetached bool, detachedMsg string, requireAhead bool) (*gitFacts, error) {
	gitRepo, gerr := gitcore.Open(dir)
	if gerr != nil || !gitRepo.InsideWorkTree() {
		return nil, deskkit.Unverifiable("not inside a git worktree", gerr)
	}
	branch, err := gitRepo.AbbrevRefHEAD()
	if err != nil {
		return nil, deskkit.Unverifiable("cannot resolve current branch", err)
	}
	if branch == "HEAD" || branch == "" {
		if !allowDetached {
			return nil, deskkit.Unverifiable(detachedMsg, nil)
		}
		branch = ""
	}
	// Refuse the default branch names outright, even if origin/HEAD is unreadable.
	if isDefaultName(branch) {
		return nil, deskkit.Refused("refused: on the default branch (" + branch + ") — deskpr only pushes feature branches")
	}
	// Resolve origin/HEAD to its FULLY-QUALIFIED target (no --short). A stray local branch
	// literally named `origin/main` (the `deskwt --branch origin/main` gotcha) makes the
	// short name `origin/main` ambiguous: `symbolic-ref --short` then disambiguates its
	// output to `remotes/origin/main`, which `TrimPrefix(…, "origin/")` cannot strip, so the
	// old `"origin/" + defaultBranch` produced the unresolvable `origin/remotes/origin/main`
	// and every rev-list/diff below aborted exit 128 (#840). The un-shortened target
	// `refs/remotes/origin/main` is unambiguous by construction, so derive the branch name
	// AND the base ref from it and use that fully-qualified ref everywhere downstream.
	defOut, derr := gitRepo.SymbolicRefTarget("refs/remotes/origin/HEAD")
	if derr != nil {
		return nil, deskkit.Unverifiable(
			"cannot read origin/HEAD (default branch unverifiable) — run `git remote set-head origin --auto`", derr)
	}
	defaultRef := strings.TrimSpace(defOut)
	defaultBranch := strings.TrimPrefix(defaultRef, "refs/remotes/origin/")
	if defaultBranch == "" || defaultBranch == defaultRef {
		return nil, deskkit.Unverifiable("origin/HEAD resolved to an unexpected target: "+defaultRef, nil)
	}
	if branch == defaultBranch {
		return nil, deskkit.Refused("refused: on the default branch (" + branch + ")")
	}

	// Decide the repo on origin's URL AS GIT RESOLVES IT (#1623), never on go-git's read of the
	// repository config file alone: the push below resolves origin itself (worktree and global
	// scope, insteadOf), and a gate that reads something else can pass a repo git never uses.
	originURL, oerr := effectiveOriginURL(dir)
	if oerr != nil {
		return nil, oerr
	}
	repo, rerr := parseRepo(originURL)
	if rerr != nil {
		return nil, deskkit.Unverifiable("cannot parse origin repo from "+originURL, rerr)
	}
	if !deskkit.IsAllowedRepo(repo) {
		return nil, deskkit.Refused("refused: origin " + repo + " is not in the desk-tools repo set")
	}

	// No staged-but-uncommitted changes: exits 1 (Refused) when the index has content
	// not yet committed, matching `git diff --cached --quiet`'s exit code.
	if staged, serr := gitRepo.HasStagedChanges(); serr != nil {
		// An object this checkout's own history references could not be read — most often
		// a `git clone --shared`/`--reference` checkout whose borrowed object store has
		// moved, gone, or is declared in a form that cannot be resolved. That is
		// COULD-NOT-CHECK, and it is reported as itself: the index may or may not be
		// clean, and neither answer may be guessed from a partially readable repository.
		if errors.Is(serr, gitcore.ErrObjectStoreIncomplete) {
			return nil, deskkit.Unverifiable(
				"cannot check staged changes: part of this checkout's object store is unreadable — "+
					"if it borrows objects from another repository (`git clone --shared`/`--reference`), "+
					"materialise them with `git repack -a` in this checkout, or use a full clone", serr)
		}
		return nil, deskkit.Unverifiable("cannot check staged changes", serr)
	} else if staged {
		return nil, deskkit.Refused("refused: staged-but-uncommitted changes — commit them first")
	}

	// Count "commits ahead" against the base the PR will ACTUALLY open against — the
	// caller's --base (`gh pr create --base <base>`), resolved to its fully-qualified
	// remote-tracking ref. Counting against origin/HEAD instead (the repo default) is a
	// bug for any --base other than the default branch: a branch legitimately ahead of
	// its intended base — e.g. a stacked PR whose base is another feature branch — but
	// NOT ahead of the default branch was false-refused as "no commits ahead", forcing
	// the caller onto the ambient-identity `gh pr create` fallback that mis-attributes
	// the PR author (#55). An empty base (the `update` verb, which has no --base and must
	// stay pinned to origin/HEAD as before) keeps the repo default.
	baseRef := defaultRef
	if base != "" {
		baseRef = "refs/remotes/origin/" + base
	}
	// The base must have a resolvable remote-tracking ref. Without this, a missing/
	// unfetched base ref aborts `git rev-list` at exit 128 and reads as unverifiable — but
	// we surface it with a precise, actionable message rather than a bare count failure.
	if ok, verr := gitRepo.CommitVerifyQuiet(baseRef); verr != nil || !ok {
		return nil, deskkit.Unverifiable(
			"base ref "+baseRef+" does not resolve — fetch the base branch (`git fetch origin`) first", verr)
	}
	if requireAhead {
		cnt, cerr := gitRepo.AheadCount(baseRef, "HEAD")
		if cerr != nil {
			return nil, deskkit.Unverifiable("cannot count commits ahead of "+baseRef, cerr)
		}
		if cnt == 0 {
			return nil, deskkit.Refused("refused: branch has no commits ahead of " + baseRef)
		}
	}

	headHash, herr := gitRepo.Resolve("HEAD")
	if herr != nil {
		return nil, deskkit.Unverifiable("cannot resolve HEAD sha", herr)
	}
	head := headHash.String()
	return &gitFacts{
		dir: dir, branch: branch, defaultBranch: defaultBranch,
		defaultRef: defaultRef, repo: repo, head: head, originURL: originURL,
	}, nil
}

// scanWrite runs the shared secret scan over the title (when set), the branch name, and
// the diff-vs-default before any push (a best-effort seatbelt; the committed-code
// residual is out of scope here).
//
// Each surface is scanned through deskkit.ScanSurface with its own NAME, so a refusal
// says which of the three fired (#328): all three used to report as "body",
// and a refusal that misidentifies its own surface sends the operator to rewrite text
// that was never the problem.
//
// override (#585): every surface here routes its verdict through
// deskkit.HandleScanRefusal, so a refusal on ANY of the three advertises the audited
// bypass and, when one is supplied, records exactly WHICH surface was waved through. The
// branch diff is the surface that matters most in practice — it is the one carrying
// go.sum blocks, lockfile digests and pre-existing content the branch cannot edit away.
func scanWrite(f *gitFacts, title, verb, override string) error {
	scanWith := func(surface string, scanner func(string, []byte) error, content []byte) error {
		return deskkit.HandleScanRefusal(deskkit.ScanOverride{
			Tool: "deskpr", Verb: verb, Repo: f.repo, Reason: override,
			Surface: surface, Content: content,
		}, scanner(surface, content))
	}
	if title != "" {
		if err := scanWith("PR title", deskkit.ScanSurface, []byte(title)); err != nil {
			return err
		}
	}
	if err := scanWith("branch name", deskkit.ScanSurface, []byte(f.branch)); err != nil {
		return err
	}
	scanRepo, rerr := gitcore.Open(f.dir)
	if rerr != nil {
		return deskkit.Unverifiable("cannot compute diff vs "+f.defaultRef+" for the secret scan", rerr)
	}
	diff, err := scanRepo.DiffSymmetric(f.defaultRef, "HEAD", 3)
	if err != nil {
		return deskkit.Unverifiable("cannot compute diff vs "+f.defaultRef+" for the secret scan", err)
	}
	// #1052 (second vector): git-diff header lines (`diff --git a/<path> b/<path>`,
	// `index …`, `--- a/<path>`, `+++ b/<path>`, `rename from/to …`, `similarity index …`)
	// are tool-generated, not author content — but a repo path routinely runs ≥32 chars
	// of deskkit's [A-Za-z0-9+/=] base64ish charset (e.g. `a/tools/desk/internal/deskkit/
	// config.go`), so BodyCheck's high-entropy-run check refused diffs touching those
	// files. Strip ONLY the strictly-matched meta lines before scanning; every content
	// line (context, `+` added, `-` removed) still goes through the secret arms in full,
	// so detection strength on author-written content is unchanged. Gated here at the
	// diff-scanning callsite, not inside deskkit.BodyCheck — BodyCheck is generic (also
	// used verbatim on PR bodies/comments/reviews) and must not grow diff-format
	// awareness.
	//
	// The diff surface is scanned in TWO passes because its two checks need DIFFERENT
	// line directions (see deskkit.ScanSurfaceSecrets):
	//
	//   - the SECRET arms read the whole stripped diff — added, removed and context
	//     lines alike. A credential on a removed line is still in the repository's
	//     history, and a banner or Secret manifest already on origin arrives on exactly
	//     this surface; that breadth is long-standing and deliberate.
	//   - the impersonated-human-ruling guard reads the ADDED lines only. A deletion
	//     cannot introduce a forged ruling — only added text can claim a human's voice —
	//     and scanning removed lines false-positived on retirement branches whose whole
	//     point was to DELETE old attribution lines ("Ruling: … — <name>" and kin).
	//     Because that guard is non-overridable by design, the false positive was a
	//     hard stop with no audited way through. Narrowing it to the added direction
	//     removes that class while catching an added forged attribution exactly as
	//     before.
	if err := scanWith("branch diff vs "+f.defaultRef, deskkit.ScanSurfaceSecrets,
		[]byte(stripDiffMetaLines(diff))); err != nil {
		return err
	}
	if err := scanWith("added lines of branch diff vs "+f.defaultRef, deskkit.ScanSurfaceRulingClaim,
		[]byte(addedDiffLines(diff))); err != nil {
		return err
	}
	// desktools-v2/10: the outbound-write check over what the push publishes — the branch
	// name (kind ref), every commit message in the range (kind commit) and the range's ADDED
	// lines per file (kind file). It is the SAME function the deskpushguard pre-push hook
	// calls, so a push by any route meets one check. The credential arms above keep their
	// whole-diff breadth; this pass adds the personal-data and, on a target that is not
	// stated private, the self-containment and withheld-identifier layers. The audited
	// override is deskpr's existing flag; a withheld identifier and a ruling claim stay
	// non-overridable.
	deskkit.SetOutboundContext(deskkit.OutboundContext{Tool: "deskpr", Verb: verb, OverrideReason: override})
	// The pre-push hook re-runs this check on the push below: hand it exactly this verdict's
	// override reason, and none when there is none (an ambient value never overrides).
	var envErr error
	if strings.TrimSpace(override) != "" {
		envErr = os.Setenv(deskkit.EnvPushGuardScanOverride, override)
	} else {
		envErr = os.Unsetenv(deskkit.EnvPushGuardScanOverride)
	}
	if envErr != nil {
		return deskkit.Unverifiable("cannot hand the override reason to the pre-push hook", envErr)
	}
	return deskkit.OutboundCheckPush(deskkit.OutboundPush{
		Dir: f.dir, Repo: f.repo, Base: f.defaultRef, Head: "HEAD", Branch: f.branch, Role: pushRole(),
	})
}

// pushRole is the App role the push goes out under, resolved the way mintWorkerToken
// resolves it (the worker App when the session names none). It is recorded on an override
// row; it never widens or narrows what the check refuses.
func pushRole() string {
	if mintedRole != "" {
		return mintedRole
	}
	if r, _, err := deskkit.SessionTokenRole("deskpr"); err == nil {
		return r
	}
	return "worker"
}

// reHunkHeader matches the deterministic RANGE part of a unified-diff hunk header —
// `@@ -27,5 +27,5 @@` — and nothing after it. Group 1 is the part that is kept; whatever
// follows on the line is git's funcname (see stripDiffMetaLines). Written to be strict:
// if a line does not match, it is kept WHOLE and scanned, so a malformed or unfamiliar
// header can only ever cause MORE scanning, never less.
var reHunkHeader = regexp.MustCompile(`^(@@ -[0-9]+(?:,[0-9]+)? \+[0-9]+(?:,[0-9]+)? @@)`)

// stripDiffMetaLines drops git-GENERATED text from a unified diff, leaving every line of
// author content intact for BodyCheck. Two kinds of generated text:
//
//  1. HEADER LINES, via a two-state scanner: header-mode (entered at `^diff --git `)
//     drops every line until the first hunk header (`^@@ `); after that the scanner exits
//     header mode and keeps everything (#1052 second vector).
//  2. HUNK-HEADER FUNCNAMES, the tail of a `@@ … @@ <funcname>` line (#328).
//
// (2) is what unblocked `.assay-versions` pin bumps, and the reason is worth stating
// because it is not a content judgement. The funcname is a copy of the nearest preceding
// line that git's funcname heuristic matched, TRUNCATED to 80 bytes. `.assay-versions`
// line 16 is `statusgen statusgen/v0.6.0 <64-hex>`; it starts with a letter, so git adopts
// it as the funcname for every hunk below it and emits the sha256 cut to 53 hex. 53 is
// neither 40 nor 64, so deskkit's git-SHA exemption misses it and it scans as a secret —
// and since line 16 and the per-platform pin block are ~15 lines apart, at the default -U3
// no diff shape can merge them into one headerless hunk. Every pin bump was refused, on
// the most routine edit made to the file the pinning scheme depends on.
//
// The generic statement, which is why this is fixed here and not by special-casing a
// filename: a funcname is diff METADATA that git derives by truncating a line of the
// file. Truncation is exactly what defeats a length-anchored exemption, and it can happen
// to any file whose funcname line carries a long token — not just this one. Nothing is
// lost by not scanning it: it is an UNCHANGED line of an already-committed file (not
// something this branch introduces, which is what a pre-push scan is for), and the
// truncated copy could not be scanned reliably anyway.
//
// The RANGE (`@@ -27,5 +27,5 @@`) is kept: it is pure line arithmetic, carries no file
// content, and keeping it preserves the diff's shape for anything reading the scanned text.
//
// Taken together this ensures:
//   - All git-generated header lines are stripped (index, ---, +++, rename, similarity,
//     mode changes, binary-file markers -- the exhaustive enumeration of git's header
//     grammar would be fragile).
//   - A hunk content line `++ <secret>` (rendered as `+++ <secret>` in the unified
//     diff) is NOT mistaken for a `+++ b/...` header -- inside a hunk the scanner is
//     out of header mode, so every line is kept and scanned by BodyCheck. The same holds
//     for a content line beginning `@@`: in a real diff it carries a ` `/`+`/`-` marker,
//     so it never matches reHunkHeader's `^@@ ` anchor.
//   - A diff quoted in documentation (no `^diff --git ` lines) is never stripped -- the
//     scanner never enters header mode, and the funcname strip is gated on that same
//     signal, so a prose-quoted hunk header is scanned as the prose it is.
func stripDiffMetaLines(diff string) string {
	lines := strings.Split(diff, "\n")
	kept := make([]string, 0, len(lines))
	inHeader, isGitOutput := false, false
	for _, ln := range lines {
		if strings.HasPrefix(ln, "diff --git ") {
			inHeader, isGitOutput = true, true
			continue
		}
		if inHeader && strings.HasPrefix(ln, "@@ ") {
			inHeader = false
		}
		if inHeader {
			continue
		}
		if isGitOutput {
			if m := reHunkHeader.FindStringSubmatch(ln); m != nil {
				ln = m[1]
			} else if len(ln) > 0 && (ln[0] == '+' || ln[0] == '-' || ln[0] == ' ') {
				// Strip the unified-diff line marker (+/-/space): it is diff SYNTAX glued to
				// the content, not part of it. Leaving it glued caused false refusals when the
				// marker joined otherwise-clean content (an added line
				// `+/tools/approvalguard/approvalguard` — the leading `+` made isPathLike's
				// "no +/=" rule reject an otherwise word-shaped 34-char path). Stripping the
				// one marker CANNOT weaken detection — the real content is what we want to
				// scan, and `+ghp_…` / `+<base64-secret>` still match once the `+` is gone —
				// while removing the whole "diff marker glued to content" false-positive class.
				ln = ln[1:]
			}
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

// addedDiffLines extracts the ADDED content lines of a unified diff — the lines this
// branch INTRODUCES — with their leading `+` markers stripped, one per line. It is the
// input to the impersonated-human-ruling guard's pass over the diff surface (see
// scanWrite): only added text can claim a human's voice, so removed and context lines
// are excluded by construction rather than by asking the guard to understand diffs.
//
// It mirrors stripDiffMetaLines' two-state scanner so the two views cannot disagree
// about what is a header:
//
//   - header mode (entered at `^diff --git `, exited at the first `^@@ `) drops
//     git-generated header lines, which is what keeps a `+++ b/<path>` file header from
//     ever being read as an added content line;
//   - inside a hunk, a line is kept exactly when it begins `+` (an added line), with
//     that one marker removed — so line-start positioning and per-line citation
//     handling inside the guard see the line as it exists in the file. A content line
//     that itself begins `+` (rendered `++…` in the diff) keeps its remaining
//     characters, which can only widen what is scanned, never narrow it;
//   - hunk headers (`@@ … @@`) are pure line arithmetic and carry no author voice, so
//     they are dropped;
//   - text that never presents a `diff --git ` line is not git-diff output at all; it
//     is returned WHOLE, so a malformed or unfamiliar input can only ever cause MORE
//     scanning, never less — the same fail-open-toward-scanning posture as
//     reHunkHeader's strictness.
func addedDiffLines(diff string) string {
	lines := strings.Split(diff, "\n")
	kept := make([]string, 0, len(lines))
	inHeader, isGitOutput := false, false
	for _, ln := range lines {
		if strings.HasPrefix(ln, "diff --git ") {
			inHeader, isGitOutput = true, true
			continue
		}
		if inHeader && strings.HasPrefix(ln, "@@ ") {
			inHeader = false
		}
		if inHeader {
			continue
		}
		if !isGitOutput {
			kept = append(kept, ln)
			continue
		}
		if strings.HasPrefix(ln, "+") {
			kept = append(kept, ln[1:])
		}
	}
	return strings.Join(kept, "\n")
}

// parseRepo extracts owner/name from a git remote URL in every shape git accepts, plus the
// rewritten/hybrid forms an `insteadOf` config or a bad URL-composition bakes onto an ssh
// host-alias remote (issue 1470). It is a thin wrapper over the single shared parser in
// deskkit, so the ssh-alias/hybrid handling lives in exactly one place across deskwt,
// deskpr, deskreply and preflight and the class cannot recur from a drifted copy.
func parseRepo(raw string) (string, error) {
	return deskkit.RemoteRepoSlug(raw)
}

// requireTrailer enforces the example-stream/02 link grammar on a PR body: exactly one
// `Brief: <stream>/<NN>` that resolves to a brief file under --root (the PR DELIVERS that
// brief), `Authors: <stream>/<NN>[, …]` whose every entry resolves the same way (the PR only
// AUTHORS those briefs — #1339), or `Issue: #<N>` for issue-only work. Absence, duplicates,
// mixed kinds and non-resolving briefs are all constraint refusals (exit 5). There is deliberately no bypass flag — a worker-typeable
// bypass makes the edge asserted again.
//
// The one exempt body is the machine-derived issue-loop scan carrier, recognised by the
// deskkit.ScanBodyMarker that `deskscanbody emit` writes at its head. That body is
// regenerated from the branch diff on every push and reconciles a whole-scope scan
// spanning many issues, so it structurally cannot carry one per-issue trailer: no
// `Issue: #N` can be both correct and stable across a re-push. The trailer gate exists to
// force HUMAN-authored PRs to name their work item, which does not apply to this one
// machine-owned body — so it is exempt, and only it (the marker is emitter-written, not a
// worker-typeable bypass flag). Every human-authored body still faces the full gate below.
//
// On success it also returns the trailer's issue number: the parsed `#<N>` for an
// `Issue:` trailer, or 0 for a `Brief:` trailer (a brief resolves to a file, not an
// issue, so it has no reactions surface) — and 0 for the exempt scan carrier likewise.
// The create path feeds this to the public-repo gate so a non-blessed public repo gains
// the per-issue-+1 path (a +1 on the named tracking issue admits the create) instead of
// the structural no-number hard-fail (#1707).
func requireTrailer(body []byte, root, dir string) (int, error) {
	// Head-anchored, not a whole-body substring: the emitter writes ScanBodyMarker as
	// the FIRST line (deskkit.ScanPRBody), so the exemption matches it only at the body
	// head. A body that merely quotes the marker somewhere in its prose is NOT exempt —
	// this keeps the carve-out keyed to genuinely emitter-produced carrier bodies.
	if strings.HasPrefix(strings.TrimLeft(string(body), " \t\r\n"), deskkit.ScanBodyMarker) {
		return 0, nil
	}
	trs, err := deskkit.ParseTrailers(body)
	if err != nil {
		return 0, deskkit.Refused("refused: " + err.Error())
	}
	if len(trs) == 0 {
		return 0, deskkit.SchemaRefusal("deskpr", "--check", "refused: PR body carries no trailer — add exactly one line "+
			"`Brief: <stream>/<NN>` naming the brief this PR delivers (e.g. `Brief: example-stream/02`), "+
			"`Authors: <stream>/<NN>[, …]` naming the brief(s) a briefs-authoring PR writes, "+
			"or `Issue: #<N>` for issue-only work")
	}
	if trs[0].Kind == deskkit.TrailerIssue {
		// Value is the bare digits (trailer.go guarantees `[0-9]+`); Atoi cannot fail,
		// but treat any parse anomaly as "no number" (0) rather than admitting a
		// negative that would read as a sentinel to the gate.
		n, perr := strconv.Atoi(trs[0].Value)
		if perr != nil || n <= 0 {
			return 0, nil
		}
		return n, nil
	}
	// A relative --root resolves against the WORK DIR (the getwd seam), never the
	// process cwd — tests call cmdCreate directly with a bound getwd, and a glob
	// against the real process cwd would silently miss the fixture.
	if !filepath.IsAbs(root) {
		root = filepath.Join(dir, root)
	}
	if trs[0].Kind == deskkit.TrailerAuthors {
		ids, aerr := deskkit.SplitAuthorsTrailer(trs[0].Value)
		if aerr != nil {
			return 0, deskkit.Refused("refused: " + aerr.Error())
		}
		for _, id := range ids {
			stream, nn, _ := splitBriefTrailer(id)
			if rerr := resolveBriefFile(root, "Authors", id, stream, nn); rerr != nil {
				return 0, rerr
			}
		}
		return 0, nil
	}
	stream, nn, ok := splitBriefTrailer(trs[0].Value)
	if !ok {
		return 0, deskkit.Refused(fmt.Sprintf("refused: trailer %q does not name a brief as <stream>/<NN> or <stream>:<NN>", trs[0].Value))
	}
	if rerr := resolveBriefFile(root, "Brief", trs[0].Value, stream, nn); rerr != nil {
		return 0, rerr
	}
	return 0, nil
}

// resolveBriefFile refuses unless docs/streams/<stream>/brief-<NN>-*.md exists under root.
// keyword and value name the trailer entry in the refusal, exactly as written.
func resolveBriefFile(root, keyword, value, stream, nn string) error {
	pattern := filepath.Join(root, "docs", "streams", stream, "brief-"+nn+"-*.md")
	matches, _ := filepath.Glob(pattern)
	if len(matches) == 0 {
		return deskkit.Refused(fmt.Sprintf("refused: `%s: %s` does not resolve to a brief under --root: no %s found",
			keyword, value, pattern))
	}
	return nil
}

// authoringTrailerGate refuses two mirror-image mismatches between a `Brief:`/`Authors:`
// trailer and what the branch's diff actually is (#1339, hardened for review F1 on
// medici-finance/assay#1641):
//
//   - `Brief:` on a branch whose diff only AUTHORS that brief. `Brief:` asserts delivery: the
//     dispatcher's phantom check, the planner's reconciliation and the derived board all read
//     it as "this PR delivers the brief", so a docs-only PR that merely wrote the brief file
//     and carried `Brief:` made the brief read as delivered the moment it merged.
//   - `Authors:` on a branch whose diff is NOT provably authoring-only for every listed id.
//     `Authors:` asserts the opposite — "no delivery" — and nothing that reads `Brief:` as
//     delivery matches it, INCLUDING the security lane's brief-declared risk term
//     (deskkit.BriefRiskFromBody keys on Brief: only). A PR that actually delivers code or a
//     document for a `gate: human` / `risk: yes` brief could therefore carry `Authors:` and
//     switch that term off. deskflip's checkSecurityVerdict carries the binding half of this
//     fix (deskkit.AuthorsRiskFromBody, read again at flip time so this client-side gate is
//     not the only thing standing between the diff and the claim); this is the writer half,
//     refusing before the mismatch ever leaves the machine.
//
// Both directions are judged by the SAME classification, deskkit.BriefAuthoringOnly — the
// dispatcher applies it to already-merged PRs, so the writer and every reader agree on what
// an authoring PR is. The branch's changed files come from the merge-base (baseRef to HEAD).
// A rename's old path is not visible to this local diff read, so BriefAuthoringOnly cannot
// prove authoring-only across one; the two directions treat that ONLY-WIDENING failure
// oppositely, on purpose — `Brief:` is left alone (not provably authoring-only leaves the
// existing "this is a delivery" answer standing), while `Authors:` is refused (not provably
// authoring-only means the claim is not backed, so it is refused rather than trusted; #1339
// review F1's "self-verifying" ask). `Issue:` bodies are never inspected by either direction.
// There is no bypass flag: the remedy is to write the right trailer, which costs nothing.
func authoringTrailerGate(body []byte, dir, baseRef string) error {
	trs, err := deskkit.ParseTrailers(body)
	if err != nil || len(trs) == 0 {
		return nil
	}
	switch trs[0].Kind {
	case deskkit.TrailerBrief:
		return authoringTrailerGateBrief(trs[0].Value, dir, baseRef)
	case deskkit.TrailerAuthors:
		return authoringTrailerGateAuthors(trs[0].Value, dir, baseRef)
	default:
		return nil
	}
}

// branchAuthoringFiles reads the branch's changed files (merge-base with baseRef to HEAD) in
// the deskkit.ChangedFile shape deskkit.BriefAuthoringOnly takes. hadRename reports whether
// the diff included a rename/copy, whose pre-image path this local git read does not surface
// (git diff --name-status reports only the destination for an R/C entry without `-M`/`-C`
// detection turned on here) — neither authoringTrailerGate direction can PROVE the authoring
// shape across one, so the caller decides what "not provably authoring-only" means for its
// trailer kind (see authoringTrailerGate's doc).
func branchAuthoringFiles(dir, baseRef string) (files []deskkit.ChangedFile, hadRename bool, err error) {
	repo, oerr := gitcore.Open(dir)
	if oerr != nil {
		return nil, false, oerr
	}
	mb, merr := repo.MergeBase(baseRef, "HEAD")
	if merr != nil {
		return nil, false, merr
	}
	changes, derr := repo.DiffNameStatus(strings.TrimSpace(mb), "HEAD")
	if derr != nil {
		return nil, false, derr
	}
	files = make([]deskkit.ChangedFile, 0, len(changes))
	for _, c := range changes {
		switch c.Status {
		case "A":
			files = append(files, deskkit.ChangedFile{Filename: c.Path, Status: "added"})
		case "M":
			files = append(files, deskkit.ChangedFile{Filename: c.Path, Status: "modified"})
		case "D":
			files = append(files, deskkit.ChangedFile{Filename: c.Path, Status: "removed"})
		default:
			hadRename = true
		}
	}
	return files, hadRename, nil
}

// authoringTrailerGateBrief is the `Brief:` direction of authoringTrailerGate.
func authoringTrailerGateBrief(value, dir, baseRef string) error {
	id := deskkit.CanonicalBriefID(value)
	if id == "" {
		return nil
	}
	files, hadRename, rerr := branchAuthoringFiles(dir, baseRef)
	if rerr != nil {
		return deskkit.Unverifiable("cannot read the branch diff to check the Brief: trailer against it", rerr)
	}
	if hadRename {
		return nil // not provably authoring-only: a rename's old path is not visible here
	}
	if !deskkit.BriefAuthoringOnly(id, files) {
		return nil
	}
	return deskkit.Refused(fmt.Sprintf(
		"refused: the body carries `Brief: %s`, but this branch only AUTHORS that brief — it adds the brief's "+
			"file and touches nothing but stream board READMEs, brief files and changelog fragments. `Brief:` "+
			"means the PR DELIVERS the brief, and every reader (the dispatcher's phantom check, the planner, the "+
			"derived board) would then treat %s as delivered the moment this merges, so it could never be "+
			"dispatched. Replace the line with `Authors: %s` (list every brief the PR writes, comma-separated), "+
			"or `Issue: #<N>` if the authoring answers an issue.", value, id, id))
}

// authoringTrailerGateAuthors is the `Authors:` direction of authoringTrailerGate (#1339
// review F1). It refuses unless the branch diff satisfies deskkit.BriefAuthoringOnly for
// EVERY id the trailer lists — including refusing (not silently leaving alone) when the diff
// carries a rename this local read cannot classify, since `Authors:` is refused whenever the
// authoring shape is not PROVEN, not merely when it is disproven. A malformed/empty value is
// left to the separate resolveBriefFile trailer-shape gate, which already refuses it.
func authoringTrailerGateAuthors(value, dir, baseRef string) error {
	ids, aerr := deskkit.SplitAuthorsTrailer(value)
	if aerr != nil || len(ids) == 0 {
		return nil
	}
	files, hadRename, rerr := branchAuthoringFiles(dir, baseRef)
	if rerr != nil {
		return deskkit.Unverifiable("cannot read the branch diff to check the Authors: trailer against it", rerr)
	}
	if hadRename {
		return deskkit.Refused(fmt.Sprintf(
			"refused: the body carries `Authors: %s`, but this branch's diff includes a rename whose old path "+
				"this local read cannot see, so the authoring shape cannot be proven. `Authors:` asserts the "+
				"branch ONLY authors the named brief(s) — including the security lane's brief-declared risk term, "+
				"which does not consult a Brief: this trailer replaces — so an unprovable diff is refused rather "+
				"than trusted. Split the rename out of this branch, or use `Brief:`/`Issue:` if the branch "+
				"delivers code or a document.", value))
	}
	for _, id := range ids {
		if deskkit.BriefAuthoringOnly(id, files) {
			continue
		}
		return deskkit.Refused(fmt.Sprintf(
			"refused: the body carries `Authors: %s`, but this branch's diff is not authoring-only for %s — it "+
				"touches a path other than a stream board README, a brief file or a changelog fragment, or it "+
				"does not ADD %s's own brief file. `Authors:` switches off the security lane's brief-declared "+
				"risk term for %s (deskkit.BriefRiskFromBody reads `Brief:` only), so it is refused unless the "+
				"diff provably authors only the listed brief(s). Use `Brief:` (singular) if this branch delivers "+
				"%s's own content, drop %s from the list if this branch only modifies its brief file, or split the "+
				"non-authoring change into its own PR.", value, id, id, id, id, id))
	}
	return nil
}

// splitBriefTrailer reduces the accepted trailer value forms to (stream, NN):
// <stream>/<NN> (brief-v1), <stream>:<NN>, <repo>:<stream>:<NN>, and the full
// <cell>:<repo>:<stream>:<NN>. For the colon forms the LAST two parts are stream and NN;
// the repo/cell prefixes resolve against graph-repos.yaml elsewhere (example-stream/01)
// and are not needed for the file resolution here. NN must be numeric.
//
// It delegates to deskkit.SplitBriefTrailer so this writer-side validation and the
// reader-side phantom key (RepresentedBriefs) apply ONE reduction and cannot disagree
// on which trailer spelling names which brief.
func splitBriefTrailer(v string) (stream, nn string, ok bool) {
	return deskkit.SplitBriefTrailer(v)
}

func readBody(bodyFile, bodyMin string) ([]byte, error) {
	if bodyFile != "" {
		b, err := os.ReadFile(bodyFile)
		if err != nil {
			return nil, deskkit.Unverifiable("cannot read --body-file", err)
		}
		if len(b) > maxBodyBytes {
			return nil, deskkit.Refused(fmt.Sprintf("refused: body exceeds %d bytes (%d)", maxBodyBytes, len(b)))
		}
		return b, nil
	}
	b := []byte(bodyMin)
	if len(b) > maxBodyBytes {
		return nil, deskkit.Refused(fmt.Sprintf("refused: --body-min exceeds %d bytes", maxBodyBytes))
	}
	return b, nil
}

func isDefaultName(branch string) bool { return branch == "main" || branch == "master" }

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// splitOwnerRepo splits "owner/name" into its components.
func splitOwnerRepo(repo string) (owner, name string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
