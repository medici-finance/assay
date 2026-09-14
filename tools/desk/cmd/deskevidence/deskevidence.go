package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	toolName = "deskevidence"
	maxBytes = 256 * 1024 // generous but bounded; brief files are ~a few KB
)

// cmdEvidence implements the evidence-commit logic. Flow:
//  1. Parse args, validate repo + evidence file
//  2. Read local evidence file
//  3. BodyCheck (secret scan)
//  4. Resolve target content: if --brief-path is given, merge evidence into brief
//  5. Fetch remote file to get current SHA + content for idempotency
//  6. Idempotency check: same content digest at same head → noop
//  7. AllowWrite rate limit
//  8. Commit via GitHub Contents API as verifier App
//  9. Verify the landed commit's author is the verifier App bot
//
// It is called ONLY from runOutward, which owns the audit lock and the single
// deferred audit line (`ac`). Adding a second entry point without the lock would
// re-open #227.
func cmdEvidence(args []string, ac *auditCtx) (err error) {
	// Skip the tool name prefix that run() already removed.
	rest := args

	if len(rest) < 2 {
		return deskkit.Refused("usage: deskevidence <owner/repo> <branch> --evidence-file <repo-path> [--brief-path <repo-path>]")
	}
	repoSlug := rest[0]
	branch := rest[1]
	flagArgs := rest[2:]

	owner, name, ok := splitRepo(repoSlug)
	if !ok {
		return deskkit.Refused("repo must be owner/name, got " + repoSlug)
	}
	if strings.TrimSpace(branch) == "" {
		return deskkit.Refused("branch is required")
	}
	ac.repo = repoSlug

	// The repo set is compiled in and no flag or env widens it. This is a cheap
	// refusal placed before any network call, so a typo'd or hostile owner/repo never
	// reaches the App-token commit path. deskevidence was the only outward-writing desk
	// command missing this gate (#1282).
	if !deskkit.IsAllowedRepo(repoSlug) {
		return deskkit.Refused("refused: " + repoSlug + " is not in the desk-tools repo set")
	}

	// Main-branch write guard (#1282). A Contents-API commit is NOT a local commit
	// awaiting a push: it writes to the remote branch immediately, so committing with
	// branch=main IS a write to main. main/master are human-gated, so refuse unless the
	// caller has explicitly sanctioned it via VERIFIER_MAIN_OK.
	//
	// The comparison is made on the ref name with any "refs/heads/" prefix stripped, so
	// spelling the branch "refs/heads/main" cannot walk past the guard. Only the guard's
	// view is normalised — the value handed to the API is the caller's, unchanged.
	//
	// The sanction must be exactly "1", matching this repo's convention for an
	// arming variable (internal/deskkit/killswitch.go) and the tool's own help text.
	// VERIFIER_MAIN_OK=0 therefore does NOT sanction a main write.
	if bare := strings.TrimPrefix(branch, "refs/heads/"); bare == "main" || bare == "master" {
		if os.Getenv("VERIFIER_MAIN_OK") != "1" {
			return deskkit.Refused("refused: branch " + branch +
				" is human-gated — a Contents-API commit lands on the remote branch immediately; " +
				"set VERIFIER_MAIN_OK=1 to sanction a main-branch Evidence commit")
		}
	}

	fs := flag.NewFlagSet("deskevidence", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	evidenceFile := fs.String("evidence-file", "", "repo-relative path to the evidence/brief file (required)")
	briefPath := fs.String("brief-path", "", "if set, merge evidence into this brief file instead of committing evidence-file directly")
	// --root binds the LOCAL read of a repo-relative --evidence-file to an explicit
	// checkout, instead of the current working directory. #1709: a writeguard can reset a
	// session's cwd to a SHARED checkout between shell calls, so a deskevidence run that did
	// not first `cd` into its worktree read the shared checkout's STALE copy of the file and
	// committed it — silently reverting the file while reporting success. Passing
	// --root <worktree> makes a repo-relative --evidence-file resolve against that worktree
	// wherever the process happens to be. It rebases only the LOCAL read; the path committed
	// to the remote branch stays the repo-relative one.
	root := fs.String("root", "", "resolve a repo-relative --evidence-file against this directory (e.g. the verifier worktree) instead of the current working directory")
	// --append-only guards a line-oriented sidecar against a net row DELETION. #1709: the
	// whole-file Contents-API commit model has no protection against an append-only file
	// shrinking, so a stale-base/wrong-file mistake reverted a sidecar (25→17 rows) as a
	// "success". It is auto-enabled for .jsonl targets (the sidecar convention) and can be
	// forced for any file; --allow-shrink is the intentional-edit override.
	appendOnlyFlag := fs.Bool("append-only", false, "refuse the commit if it would reduce the target's row count below the current remote (auto-enabled for .jsonl sidecars)")
	allowShrink := fs.Bool("allow-shrink", false, "override the append-only shrink guard when a row reduction is genuinely intended")
	if perr := fs.Parse(flagArgs); perr != nil {
		return deskkit.Refused("bad flags: " + perr.Error())
	}
	if *evidenceFile == "" {
		return deskkit.Refused("--evidence-file is required")
	}

	// Generated-file guard. STATUS.md is generated and its single writer is main's CI:
	// it is regenerated locally and never committed, on main or on a branch. This tool
	// is what makes main a *sanctioned* channel (see the main-branch guard above), and
	// VERIFIER_MAIN_OK is set routinely in the verify-desk window, so one coarse env var
	// must not also open main to a generated file. Refused for BOTH targets and on every
	// branch, before any network call — with --brief-path set the brief is fetched to be
	// merged, so checking only the resolved target would fetch first.
	for _, p := range []string{*evidenceFile, *briefPath} {
		if p != "" && path.Base(p) == "STATUS.md" {
			return deskkit.Refused("refused: " + p +
				" is generated — main's CI is its single writer; regenerate locally, never commit it")
		}
	}

	evidenceRepoPath := *evidenceFile
	ac.file = evidenceRepoPath

	// Determine the target repo path this commit will land at. Computed here, ahead of the
	// local-read resolution right below, because the docs/streams scoping guard a few lines
	// down needs it and depends on nothing else (not --root, not the file's own content) —
	// so it stays a cheap, pre-network check regardless of where exactly it runs relative to
	// the root-contradiction/oversize checks that follow.
	var targetRepoPath string
	if *briefPath != "" {
		targetRepoPath = *briefPath
	} else {
		targetRepoPath = evidenceRepoPath
	}
	ac.file = targetRepoPath

	// Resolve the LOCAL read path. With --root set, a repo-relative --evidence-file is read
	// from that checkout (#1709) rather than the process cwd; the target repo path committed
	// to the branch stays evidenceRepoPath either way. An absolute --evidence-file with
	// --root is contradictory (the join would be meaningless), so it is refused rather than
	// silently ignoring one of them.
	localReadPath := evidenceRepoPath
	if *root != "" {
		if filepath.IsAbs(evidenceRepoPath) {
			return deskkit.Refused("refused: --evidence-file must be a repo-relative path when --root is set, got absolute " + evidenceRepoPath)
		}
		if info, serr := os.Stat(*root); serr != nil || !info.IsDir() {
			return deskkit.Unverifiable("--root "+*root+" is not a readable directory", serr)
		}
		localReadPath = filepath.Join(*root, evidenceRepoPath)
	}

	// Read the local evidence file.
	localContent, rerr := os.ReadFile(localReadPath)
	if rerr != nil {
		return deskkit.Unverifiable("cannot read --evidence-file "+localReadPath, rerr)
	}
	if len(localContent) > maxBytes {
		return deskkit.Refused(fmt.Sprintf("refused: evidence file exceeds %d bytes (%d)", maxBytes, len(localContent)))
	}

	// docs/streams/ scoping guard (deskevidence: refuse a landing that adds a statusgen
	// PROBLEM or lands outside docs/streams/). deskevidence exists to commit Evidence rows
	// and the brief-status flips that accompany them under that tree; a target path
	// resolving anywhere else — a stray root file, a directory-traversal escape, an
	// absolute path — is refused here, before any network call. This is exactly the
	// "landed outside docs/streams/" main-red shape: a stray root file landed by a
	// deskevidence commit, outside the tree the tool is meant to write to.
	//
	// Checked on the CLEANED repo-relative form: path.Clean collapses any ../ segments the
	// string carries, so an escape (docs/streams/../../etc/passwd) shows up as a path that
	// no longer starts with docs/streams/ rather than surviving as a literal "..". An
	// absolute path fails path.IsAbs outright — a Contents-API repo path is never absolute
	// in real use, so refusing one here is not a new restriction, it is the first thing
	// that has ever checked the shape. Placed after the root-contradiction and oversize
	// checks above (both more specific refusals for the shapes they cover) so neither is
	// ever shadowed by this more general one.
	if !underDocsStreams(targetRepoPath) {
		return deskkit.Refused("refused: target path " + targetRepoPath +
			" resolves outside docs/streams/ — deskevidence only writes under that tree")
	}

	// Mint the verifier App installation token and resolve the forge that serves this repo,
	// under the verifier App's custody. The JWT→installation-token exchange moved OUT of this
	// package to the identity layer (mintTokenFn → `desktoken verifier`); ForgeFor hands the
	// minted token to the backend it constructs and never falls back to an ambient identity.
	// Placed after the cheap/stateless refusals so a doomed call never mints.
	if merr := mintTokenFn(repoSlug); merr != nil {
		return merr
	}
	fg, fr, ferr := forgeForFn(owner, name)
	if ferr != nil {
		return ferr
	}

	// Read the current remote content of the target ONCE, up front. It is the base every
	// scoping decision below is judged against: the brief merge (a genuine read → transform →
	// write, which is why ReadFile exists on the seam alongside WriteFile — the merge cannot be
	// folded into a backend-agnostic write), the secret scan's "what is genuinely NEW" question
	// (#966, below), and — later — the idempotency and shrink-guard checks. A single fetch means
	// none of those can disagree about what is already on the branch. A path ABSENT on the
	// branch is a create (empty remote), not an error, EXCEPT with --brief-path: a brief must
	// already exist to be merged into, so a not-found is propagated exactly as it was when this
	// same ReadFile lived inside mergeEvidence.
	var remoteContent []byte
	remoteExists := false
	remoteSHA := ""
	cur, rerr := fg.ReadFile(fr, deskkit.ReadFileInput{File: targetRepoPath, Ref: branch})
	if rerr != nil {
		if !deskkit.IsForgeNotFound(rerr) || *briefPath != "" {
			return rerr
		}
	} else {
		remoteContent, remoteExists = cur.Content, cur.Exists
		remoteSHA = cur.SHA
	}

	// Resolve the content to commit: with --brief-path, merge the new evidence into the brief's
	// ## Evidence section against the remote content just fetched; otherwise the local evidence
	// file IS the target's whole content (the caller merged it locally before calling this tool).
	var commitContent []byte
	blockAlready := false
	if *briefPath != "" {
		merged, present := mergeEvidenceContent(remoteContent, localContent)
		commitContent = merged
		blockAlready = present
	} else {
		commitContent = localContent
	}

	// Block-level idempotency: a fresh Evidence block byte-equivalent (after normalising line
	// endings, trailing whitespace and trailing blank lines) to the block already standing in
	// the brief's ## Evidence section is a no-op — it proves what the last run proved and has
	// nothing to land. Decided on the content the fetch above already returned, so it adds no
	// extra API call, and taken BEFORE the shrink guard (like the file-level noop) so an
	// idempotent re-run is never mistaken for a shrink. NOTHING is fetched-then-put.
	if blockAlready {
		ac.successResult = deskkit.ResultNoop
		ac.detail = fmt.Sprintf("noop: Evidence block already present in %s on %s (sha %s)",
			targetRepoPath, branch, shortSHA(remoteSHA))
		fmt.Fprintln(stdout, ac.detail)
		return nil
	}

	// Append-only sidecars (the .jsonl streams under docs/streams/) grow row-by-row and
	// never shrink in normal use; a net row DROP is the #1709 signature. Auto-enable the
	// shrink guard for that class, and honour an explicit --append-only for any other file.
	appendOnly := *appendOnlyFlag || strings.HasSuffix(targetRepoPath, ".jsonl")

	// Secret-scan only the bytes THIS commit ADDS relative to what is already on the branch
	// (#966). With --brief-path that is exactly localContent — mergeEvidenceContent never
	// touches a pre-existing byte of the brief, so the evidence being merged in already IS the
	// added bytes, never commitContent (which is the whole MERGED file: every pre-existing byte
	// of the brief plus the new block). WITHOUT --brief-path the caller hands over the whole
	// target file (it merged the row into a local working copy itself before calling this tool),
	// so localContent can legitimately be almost entirely PRE-EXISTING text — scanning it whole
	// reads a Verify row's own quoted secret-shaped material (a slash-list of stream
	// identifiers, a Kubernetes PVC uid, a fingerprint quoted in prose — all of it already
	// reviewed and merged through the normal PR path) as if this commit had just typed it, and a
	// brief carrying either shape could never receive another Evidence append through the
	// sanctioned tool at all. addedLines diffs localContent against the remote content fetched
	// above and hands the scanner only the lines that are actually new — the same "what did THIS
	// commit add" question already answered for the --brief-path merge, now judged against the
	// real remote base instead of an implicit empty one. A brand-new file (remoteExists false)
	// has no base to diff against, so the whole thing is genuinely new and is scanned whole,
	// exactly as before.
	scanTarget := localContent
	if *briefPath == "" && remoteExists {
		scanTarget = addedLines(remoteContent, localContent)
	}
	if berr := deskkit.BodyCheck(scanTarget); berr != nil {
		return berr
	}

	// Public-repo write gate. deskevidence writes a file directly to a remote branch — an
	// outward write. A public/internal target is authorized only by a listed `:public`
	// allowed-repos entry (deskkit.PublicRepoGate); private/internal-without-entry refuse.
	// The fetcher uses the minted verifier token and the backend's own default host (this tool
	// no longer binds a GitHub API host literal of its own).
	fetcher := &deskkit.HTTPRepoInfoFetcher{Token: ghToken}
	if gerr := publicRepoGateFn(fetcher, owner, name); gerr != nil {
		return gerr
	}

	bodyDig := deskkit.Sha256Hex(commitContent)
	ac.bodyDig = bodyDig

	// Idempotency: same content already on the branch → noop, before any write budget is spent.
	if remoteExists && deskkit.Sha256Hex(remoteContent) == bodyDig {
		ac.successResult = deskkit.ResultNoop
		ac.detail = fmt.Sprintf("noop: %s already has this content on %s", targetRepoPath, branch)
		fmt.Fprintln(stdout, "noop: "+targetRepoPath+" already has this content on "+branch)
		return nil
	}

	// Append-only shrink guard (#1709), pre-checked here so a doomed write never spends a
	// budget; the WriteFile op enforces the SAME constraint post-fetch as an independent second
	// layer (constraint passed in via AppendOnly, backend refuses after its own fetch).
	if appendOnly && !*allowShrink && remoteExists {
		remoteRows := rowCount(remoteContent)
		newRows := rowCount(commitContent)
		if newRows < remoteRows {
			return deskkit.Refused(fmt.Sprintf(
				"refused: %s is append-only and this commit would SHRINK it from %d to %d rows (%d fewer) — "+
					"almost always a stale-cwd or wrong-file mistake, not an intended edit; "+
					"pass --root <checkout> so --evidence-file resolves against the right worktree, "+
					"or --allow-shrink to override when the reduction is intended",
				targetRepoPath, remoteRows, newRows, remoteRows-newRows))
		}
	}

	// statusgen PROBLEM-diff guard (deskevidence: refuse a landing that adds a statusgen
	// PROBLEM or lands outside docs/streams/). Diffs the PROBLEM set statusgen --lint
	// reports for the landing worktree BEFORE this write against the set it reports AFTER
	// staging commitContent at targetRepoPath, and refuses if the landing would introduce
	// any PROBLEM not already present — a pre-existing red elsewhere in the repo never
	// blocks a clean landing (#1078), only a PROBLEM THIS write would add. Placed after the
	// noop/shrink checks (nothing to lint when nothing is landing) and before the rate
	// limit (a doomed write should not spend a budget slot).
	//
	// The landing worktree is --root when given, the process cwd otherwise — the same
	// fallback the local-read resolution above already uses, so "the tree this check lints"
	// and "the tree a bare --evidence-file would have been read from" are always the same
	// tree.
	lintRoot := *root
	if lintRoot == "" {
		lintRoot = "."
	}
	introduced, lerr := lintDiffFn(lintRoot, targetRepoPath, commitContent)
	if lerr != nil {
		return lerr
	}
	if len(introduced) > 0 {
		return deskkit.Refused(fmt.Sprintf(
			"refused: landing %s would introduce %d new statusgen PROBLEM(s) not present in %s before this change:\n%s",
			targetRepoPath, len(introduced), lintRoot, strings.Join(introduced, "\n")))
	}

	// Outward-write rate limit. pr=0 is the repo's unnumbered bucket; deskevidence carries a
	// per-tool override on it (see unnumberedBucketCap in ratelimit.go).
	if werr := deskkit.AllowWrite(toolName, repoSlug, 0); werr != nil {
		return werr
	}

	// Write the Evidence row through the resolved forge, as the verifier App.
	res, cerr := fg.WriteFile(fr, deskkit.WriteFileInput{
		File:        targetRepoPath,
		Branch:      branch,
		Content:     commitContent,
		Message:     "Evidence: verification row for " + targetRepoPath,
		AppendOnly:  appendOnly,
		AllowShrink: *allowShrink,
	})
	if cerr != nil {
		return cerr
	}

	// The Evidence lane on a forge whose default branch takes no direct write. WriteFile
	// reports the sentinel WITHOUT writing; land the row on a side branch and open a draft
	// change instead, and say so on stdout. It never attempts the direct write and reports
	// success, and it never skips the row.
	if res.DefaultBranchNotWritable {
		return landEvidenceAsChange(fg, fr, repoSlug, branch, targetRepoPath, commitContent, appendOnly, *allowShrink, remoteContent, ac)
	}

	// Post-condition: the write that landed must carry the verifier App's identity. Checked
	// against what the forge reported the write recorded, not the token we sent (#228).
	attr, aerr := checkAttribution(res.Author)
	// Name the net row delta so a success line can no longer hide a replace or a deletion
	// behind a "committed … success" (#1709).
	added, removed := rowDelta(remoteContent, commitContent)
	delta := fmt.Sprintf("+%d/-%d rows", added, removed)
	base := fmt.Sprintf("committed %s to %s on %s (sha %s, %s)", targetRepoPath, repoSlug, branch, shortSHA(res.SHA), delta)
	ac.detail = base + " — " + attr
	if aerr != nil {
		return aerr
	}
	fmt.Fprintf(stdout, "committed %s to %s on %s (sha %s, %s) — %s\n",
		targetRepoPath, repoSlug, branch, shortSHA(res.SHA), delta, attr)
	return nil
}

// landEvidenceAsChange is the Evidence lane for a forge whose default branch takes no direct
// write (GitLab — pilot D-8). It writes the row to a NEW side branch (WriteFile with
// StartBranch cutting it from the closed default), opens a DRAFT change from that branch, and
// names the change on stdout — so a verified brief's Evidence still lands, as a reviewable
// change rather than a direct commit, and NO direct write to the default branch is attempted.
func landEvidenceAsChange(fg deskkit.Forge, fr deskkit.ForgeRepo, repoSlug, base, target string, content []byte, appendOnly, allowShrink bool, remoteContent []byte, ac *auditCtx) error {
	dig := deskkit.Sha256Hex(content)
	side := "evidence/" + sanitizeBranchComponent(path.Base(target)) + "-" + dig[:8]

	res, werr := fg.WriteFile(fr, deskkit.WriteFileInput{
		File:        target,
		Branch:      side,
		Content:     content,
		Message:     "Evidence: verification row for " + target,
		StartBranch: base,
		AppendOnly:  appendOnly,
		AllowShrink: allowShrink,
	})
	if werr != nil {
		return werr
	}
	if res.DefaultBranchNotWritable {
		// The side branch is not the default; a sentinel here is a backend contradiction, not
		// a lane to fall further through.
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: the forge reported side branch %s not directly writable either — the Evidence "+
				"row was NOT landed", side), nil)
	}

	pr, perr := fg.CreateDraftChange(fr, deskkit.DraftChangeInput{
		Title: "Evidence: " + target,
		Body: "Verification Evidence row for `" + target + "`, landed on branch `" + side + "` and opened " +
			"as a draft change because the default branch `" + base + "` takes no direct write on this forge. " +
			"A reviewer verdict lands the row.",
		Head: side,
		Base: base,
	})
	if perr != nil {
		return perr
	}

	attr, aerr := checkAttribution(res.Author)
	added, removed := rowDelta(remoteContent, content)
	delta := fmt.Sprintf("+%d/-%d rows", added, removed)
	loc := fmt.Sprintf("change #%d", pr.Number)
	if pr.URL != "" {
		loc = pr.URL
	}
	ac.detail = fmt.Sprintf("landed %s on %s in %s via draft %s (%s) — default branch not directly writable — %s",
		target, repoSlug, side, loc, delta, attr)
	if aerr != nil {
		return aerr
	}
	fmt.Fprintf(stdout, "landed %s on %s: %s takes no direct write, so wrote branch %s and opened draft %s (%s) — %s\n",
		target, repoSlug, base, side, loc, delta, attr)
	return nil
}

// sanitizeBranchComponent renders a file-base into a git-branch-safe component: only letters,
// digits, dot, dash and underscore survive, and everything else collapses to a dash.
func sanitizeBranchComponent(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		out = "evidence"
	}
	return out
}

// publicRepoGateFn is the seam for deskkit.PublicRepoGate — tests set it to a no-op stub so
// they need no live repo-visibility read. Production uses the real gate.
var publicRepoGateFn = deskkit.PublicRepoGate

// rowCount returns the number of non-empty (row-bearing) lines in b. Trailing newlines and
// blank lines do not count, so a sidecar with or without a final newline reports the same
// row count.
func rowCount(b []byte) int {
	n := 0
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) != "" {
			n++
		}
	}
	return n
}

// rowDelta reports how many non-empty rows newer adds and removes relative to older,
// comparing the two as MULTISETS of trimmed line text. For an append-only sidecar (one
// JSON object per line) this is exact row accounting that is insensitive to reordering; for
// any other file it is a serviceable line-level delta. A pure append yields (added>0,
// removed=0); the #1709 clobber (25→17) yields removed=8.
func rowDelta(older, newer []byte) (added, removed int) {
	count := func(b []byte) map[string]int {
		m := map[string]int{}
		for _, ln := range strings.Split(string(b), "\n") {
			if t := strings.TrimSpace(ln); t != "" {
				m[t]++
			}
		}
		return m
	}
	o, n := count(older), count(newer)
	for row, nc := range n {
		if extra := nc - o[row]; extra > 0 {
			added += extra
		}
	}
	for row, oc := range o {
		if gone := oc - n[row]; gone > 0 {
			removed += gone
		}
	}
	return added, removed
}

// addedLines returns the lines of newer that are NOT already accounted for by older, as a
// MULTISET diff — the same "how many rows changed" comparison rowDelta already makes for the
// audit trail (+N/-M rows), applied here to recover the literal TEXT of what changed rather than
// just its count. It is the #966 fix's scanning base for an Evidence commit made WITHOUT
// --brief-path: deskevidence has no other way to know what a caller-supplied whole-file body
// actually adds relative to the branch, and scanning the whole thing re-refuses on any
// secret-shaped run the file already carried, forever (see cmdEvidence's secret-scan comment).
//
// A line repeated in newer beyond how many times older carried it is added only for the SURPLUS
// occurrences, exactly like rowDelta's own counting — so an unchanged line does not get counted
// as "new" just because some OTHER unchanged line further down happens to read the same. Order
// follows newer's own line order, which keeps the result readable; reBase64ish (the scanner's
// high-entropy run detector) never matches across a newline, so nothing a line-oriented diff
// could split a run across is lost by joining the added lines back with "\n".
//
// Unlike rowDelta this does NOT trim or drop blank lines: a blank line carries no secret, so
// including it costs the scanner nothing, and dropping it would risk gluing two unrelated lines'
// characters together across what was a line boundary in the original file.
func addedLines(older, newer []byte) []byte {
	remaining := map[string]int{}
	for _, ln := range strings.Split(string(older), "\n") {
		remaining[ln]++
	}
	var out strings.Builder
	for _, ln := range strings.Split(string(newer), "\n") {
		if remaining[ln] > 0 {
			remaining[ln]--
			continue
		}
		out.WriteString(ln)
		out.WriteByte('\n')
	}
	return []byte(out.String())
}

// auditCtx accumulates fields for the ONE audit line per invocation.
// finalize is deferred so exactly one line is written.
type auditCtx struct {
	verb          string
	repo          string
	file          string
	bodyDig       string
	detail        string
	successResult string // ResultOK unless a noop set it to ResultNoop
}

func (a *auditCtx) log(result, detail string) {
	_ = deskkit.Log(deskkit.Entry{
		Tool:       toolName,
		Verb:       a.verb,
		ArgsDigest: deskkit.ArgsDigest(os.Args[1:]),
		BodyDigest: a.bodyDig,
		Repo:       a.repo,
		Result:     result,
		Detail:     detail,
	})
}

func (a *auditCtx) finalize(err error) {
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
	// On the failure paths the durable record must still say WHAT happened, not only why
	// the invocation was refused. cmdEvidence sets a.detail exactly once on an erroring
	// path — the attribution post-condition, where the commit has ALREADY landed on the
	// remote under a foreign identity — and `err.Error()` alone names the wrong identity
	// but drops the path, repo, branch and SHA that landed. That path is the one where the
	// audit line is the only record, because the tool cannot undo the commit: it is "a
	// report rather than a prevention", so the report has to carry the facts. Keep both.
	detail := err.Error()
	if a.detail != "" {
		detail = a.detail + " — " + detail
	}
	a.log(result, detail)
}

// underDocsStreams reports whether repoPath, once cleaned, resolves under docs/streams/ —
// the tree deskevidence exists to write to (see the docs/streams/ scoping guard in
// cmdEvidence). An absolute path is refused outright: a Contents-API repo path is never
// absolute in real use. A relative path is checked on its path.Clean form, so a
// directory-traversal escape (docs/streams/../../etc/passwd) shows up as failing the
// prefix check rather than surviving as a literal "..".
func underDocsStreams(repoPath string) bool {
	cleaned := path.Clean(repoPath)
	return !path.IsAbs(cleaned) && strings.HasPrefix(cleaned, "docs/streams/")
}

func splitRepo(s string) (owner, name string, ok bool) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// mergeEvidenceContent finds the ## Evidence section in remoteContent and appends the evidence
// content. It returns the merged content and a flag that is true when the fresh block is already
// standing in the section (a block-level no-op — see blockAlreadyPresent).
//
// This is the pure text-transform half of what was a single mergeEvidence function; the read
// itself (the ReadFile op) now happens once, up front in cmdEvidence, alongside the fetch the
// secret scan's added-bytes scoping (#966) and the idempotency/shrink-guard checks all judge
// themselves against — a brief absent on the branch is refused there (a brief must exist to be
// merged into), so remoteContent here is always a real brief body, never a not-found.
func mergeEvidenceContent(remoteContent, evidence []byte) (merged []byte, alreadyPresent bool) {
	// Find the ## Evidence section and append.
	// The Evidence section starts with "## Evidence" and ends at end of file
	// or at the next "## " heading.
	evidenceStr := string(evidence)
	remoteStr := string(remoteContent)

	// Clean up evidence: remove leading/trailing whitespace but keep internal newlines
	evidenceStr = strings.TrimSpace(evidenceStr)

	// If the remote has a "## Evidence" section, append to it.
	evidenceMarker := "\n## Evidence\n"
	idx := strings.Index(remoteStr, evidenceMarker)
	if idx < 0 {
		// Also try "## Evidence\r\n"
		evidenceMarker = "\n## Evidence\r\n"
		idx = strings.Index(remoteStr, evidenceMarker)
	}
	if idx < 0 {
		// No Evidence section found — append one at the end. This CREATES the section, so it is
		// never a block-level no-op.
		out := strings.TrimRight(remoteStr, "\n") + "\n\n## Evidence\n" + evidenceStr + "\n"
		return []byte(out), false
	}

	// Find the end of the Evidence section (next ## heading or EOF).
	afterMarker := remoteStr[idx+len(evidenceMarker):]
	nextHeading := strings.Index(afterMarker, "\n## ")
	evidenceEnd := len(remoteStr)
	if nextHeading >= 0 {
		evidenceEnd = idx + len(evidenceMarker) + nextHeading
	}

	// Build the merged content: before Evidence + Evidence header + existing + new + after.
	existingEvidence := strings.TrimRight(remoteStr[idx+len(evidenceMarker):evidenceEnd], "\n")

	// Block-level idempotency: if the fresh block is already standing in the section, report it
	// so the flow can no-op WITHOUT a second fetch or any write. Decided on the content this
	// read already returned.
	alreadyPresent = blockAlreadyPresent(existingEvidence, evidenceStr)

	out := remoteStr[:idx] + evidenceMarker
	if existingEvidence != "" {
		out += existingEvidence + "\n"
	}
	out += evidenceStr + "\n"
	if evidenceEnd < len(remoteStr) {
		out += remoteStr[evidenceEnd:]
	}

	return []byte(out), alreadyPresent
}

// normalizeEvidenceText normalises a block or an Evidence section for the block-equivalence
// check: line endings collapse to "\n", trailing whitespace is trimmed per line, and trailing
// blank lines are dropped. Nothing looser — once these are applied the comparison stays exact,
// so a re-run on a different date or with a different runner (genuinely new text) still differs.
func normalizeEvidenceText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(ln, " \t")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// blockAlreadyPresent reports whether the fresh Evidence block, after normalisation, is already
// standing at the tail of the existing ## Evidence section — a contiguous substring anchored at
// the section's end. The most-recently-appended block sits at that tail, so an exact re-run
// matches it, while:
//   - a PARTIAL re-run (a prefix of the standing block) does not reach the section's end and is
//     new content that lands;
//   - a block differing by even one character does not match and lands;
//   - a SUPERSET (the standing block plus new rows) is longer than the tail and lands, carrying
//     its whole fresh content unchanged.
//
// Anchoring at the end (rather than a match anywhere in the section) is what keeps the prefix
// case new content; a leading placeholder comment or an older block ahead of the tail is skipped
// by the same anchoring.
func blockAlreadyPresent(existingSection, freshBlock string) bool {
	section := normalizeEvidenceText(existingSection)
	block := normalizeEvidenceText(freshBlock)
	if block == "" {
		return false
	}
	return section == block || strings.HasSuffix(section, "\n"+block)
}
