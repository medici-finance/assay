package main

import (
	"flag"
	"fmt"
	"os"
	"path"
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

	// Read the local evidence file.
	localContent, rerr := os.ReadFile(evidenceRepoPath)
	if rerr != nil {
		return deskkit.Unverifiable("cannot read --evidence-file "+evidenceRepoPath, rerr)
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
	base := fmt.Sprintf("committed %s to %s on %s (sha %s)", targetRepoPath, repoSlug, branch, shortSHA(newSHA))
	ac.detail = base + " — " + attr
	if aerr != nil {
		return aerr
	}
	fmt.Fprintf(stdout, "committed %s to %s on %s (new tree sha %s) — %s\n",
		targetRepoPath, repoSlug, branch, shortSHA(newSHA), attr)
	return nil
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
