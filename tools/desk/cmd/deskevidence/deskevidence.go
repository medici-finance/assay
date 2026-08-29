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

	// Determine the target repo path and content to commit.
	var targetRepoPath string
	var commitContent []byte

	if *briefPath != "" {
		targetRepoPath = *briefPath
		// Append evidence to the brief file's Evidence section.
		// Fetch the current brief from GitHub, find ## Evidence, append.
		merged, merr := mergeEvidence(owner, name, branch, *briefPath, localContent)
		if merr != nil {
			return merr
		}
		commitContent = merged
	} else {
		targetRepoPath = evidenceRepoPath
		commitContent = localContent
	}
	ac.file = targetRepoPath

	// Append-only sidecars (the .jsonl streams under docs/streams/) grow row-by-row and
	// never shrink in normal use; a net row DROP is the #1709 signature. Auto-enable the
	// shrink guard for that class, and honour an explicit --append-only for any other file.
	// The actual comparison happens once the remote row count is known.
	appendOnly := *appendOnlyFlag || strings.HasSuffix(targetRepoPath, ".jsonl")

	// Secret-scan the content that will be committed.
	if berr := deskkit.BodyCheck(commitContent); berr != nil {
		return berr
	}

	// Public-repo trust gate. deskevidence commits a file directly to a
	// remote branch via PUT /repos/{owner}/{repo}/contents/{path} — an outward write.
	// Placed here, after all cheap/stateless refusals (repo-set gate, main-branch guard, STATUS.md,
	// file-read, oversize, secret-scan) and before the first remote fetch.
	// There is no associated issue/PR number, so the gate fails closed (exit 6) for
	// public repos (a file commit has no reactions surface to consult) and passes
	// through for private/internal repos.
	tok, terr := mintVerifierToken(owner, name)
	if terr != nil {
		return terr
	}
	fetcher := &deskkit.HTTPRepoInfoFetcher{Token: tok, BaseURL: apiBaseURL}
	if gerr := deskkit.PublicRepoGate(fetcher, owner, name, 0); gerr != nil {
		return gerr
	}

	bodyDig := deskkit.Sha256Hex(commitContent)
	ac.bodyDig = bodyDig

	// Fetch remote file to get current SHA and check for idempotency.
	remoteSHA, remoteContent, ferr := fetchRemoteFile(owner, name, targetRepoPath, branch)
	if ferr != nil {
		return ferr // already a *DeskError from the API layer
	}

	// Check if the remote content matches what we'd commit.
	// If identical, this is a noop.
	remoteDig := deskkit.Sha256Hex(remoteContent)
	if remoteDig == bodyDig {
		// Idempotency: same content already on the branch.
		ac.successResult = deskkit.ResultNoop
		ac.detail = fmt.Sprintf("noop: %s already has this content on %s (sha %s)", targetRepoPath, branch, shortSHA(remoteSHA))
		fmt.Println("noop: " + targetRepoPath + " already has this content at " + shortSHA(remoteSHA))
		return nil
	}

	// Append-only shrink guard (#1709). For a line-oriented sidecar, a commit that leaves
	// FEWER rows than the remote already holds is almost always a stale-cwd or wrong-file
	// mistake — the whole-file Contents-API PUT would otherwise revert the file and report
	// success, deleting rows unattributably. Refuse before spending a write budget; the
	// operator either re-points --root at the right checkout or passes --allow-shrink when
	// the reduction is genuinely intended. Placed after the noop check so an idempotent
	// re-commit is never mistaken for a shrink.
	if appendOnly && !*allowShrink {
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

	// Outward-write rate limit. A deskevidence commit targets a BRANCH, not a
	// PR, so there is no number to pass and the audit line it writes records none either.
	// pr=0 is therefore the repo's unnumbered bucket. deskevidence carries a per-tool
	// override on that bucket (deskkit.UnnumberedCapFor("deskevidence"), currently 30 — see
	// unnumberedBucketCap in ratelimit.go), which RAISES its effective ceiling above the base
	// per-PR cap without dropping the per-PR tier: it is NOT an exemption — passing 0 used to
	// skip the per-PR tier outright and leave this Contents-API write path on the 100/hr
	// per-repo cap (#439 review). The breaker and the per-repo tier still apply on top.
	if werr := deskkit.AllowWrite(toolName, repoSlug, 0); werr != nil {
		return werr
	}

	// Commit via GitHub Contents API as the verifier App.
	newSHA, author, cerr := commitFile(owner, name, targetRepoPath, branch, remoteSHA, commitContent)
	if cerr != nil {
		return cerr
	}

	// Post-condition: the commit that landed must carry the verifier App's identity.
	// Checked against the response GitHub returned, not against the token we sent —
	// the token is what we *intended*, the author is what actually happened
	// (#228).
	attr, aerr := checkAttribution(author)
	// Name the net row delta so the success message can no longer hide a replace or a
	// deletion behind a "committed … success" (#1709). +A names rows the commit adds,
	// -R names rows it drops, both computed against the remote content at the target path.
	added, removed := rowDelta(remoteContent, commitContent)
	delta := fmt.Sprintf("+%d/-%d rows", added, removed)
	base := fmt.Sprintf("committed %s to %s on %s (sha %s, %s)", targetRepoPath, repoSlug, branch, shortSHA(newSHA), delta)
	ac.detail = base + " — " + attr
	if aerr != nil {
		return aerr
	}
	fmt.Fprintf(stdout, "committed %s to %s on %s (new tree sha %s, %s) — %s\n",
		targetRepoPath, repoSlug, branch, shortSHA(newSHA), delta, attr)
	return nil
}

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

// mergeEvidence reads the brief file from GitHub, finds the ## Evidence section,
// and appends the evidence content. It returns the merged content.
func mergeEvidence(owner, name, branch, briefPath string, evidence []byte) ([]byte, error) {
	_, remoteContent, err := fetchRemoteFile(owner, name, briefPath, branch)
	if err != nil {
		return nil, err
	}

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
		// No Evidence section found — append one at the end.
		merged := strings.TrimRight(remoteStr, "\n") + "\n\n## Evidence\n" + evidenceStr + "\n"
		return []byte(merged), nil
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
	merged := remoteStr[:idx] + evidenceMarker
	if existingEvidence != "" {
		merged += existingEvidence + "\n"
	}
	merged += evidenceStr + "\n"
	if evidenceEnd < len(remoteStr) {
		merged += remoteStr[evidenceEnd:]
	}

	return []byte(merged), nil
}
