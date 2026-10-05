package main

// outcomerecord.go — the `--outcome-record` landing shape (#882).
//
// Retires the whole-file append model for verify outcomes: instead of merging one more line
// into a single shared docs/streams/verify-outcomes.jsonl (server-side-merge CONFLICTING every
// sibling Evidence PR touching that path concurrently), this commits ONE brand-new file at the
// path deskkit.RecordName derives from the record's own bytes. Two concurrent landings only ever
// pick the same path when they carry byte-identical content — the same outcome — and two
// identical adds merge cleanly.
//
// Records are IMMUTABLE: an existing path with identical bytes is a noop; an existing path with
// different bytes is refused (a correction is a NEW record, never an edit).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// nowFn is the writer's clock, overridable in tests. #1803 SR-1803-2: cmdOutcomeRecordWrite
// refuses a record whose `ts` is more than deskkit.MaxClockSkew ahead of nowFn() — the same bound
// deskkit.LatestPerBrief applies on the read side, so a record this writer ever lands can never
// itself trip the reader's could-not-check classification.
var nowFn = time.Now

// cmdOutcomeRecordWrite implements `deskevidence <owner/repo> <branch> --outcome-record <file>`.
// localFile is the LOCAL path to a file holding exactly one JSON verify-outcome record; root, if
// set, rebases a repo-relative localFile against it (mirrors --evidence-file's --root handling,
// #1709).
func cmdOutcomeRecordWrite(localFile, repoSlug, owner, name, branch, root string, ac *auditCtx) error {
	readPath := localFile
	if root != "" && !filepath.IsAbs(localFile) {
		readPath = filepath.Join(root, localFile)
	}
	raw, rerr := os.ReadFile(readPath)
	if rerr != nil {
		return deskkit.Unverifiable("cannot read --outcome-record "+readPath, rerr)
	}

	rec, perr := deskkit.ParseRecord(raw)
	if perr != nil {
		return deskkit.Refused("refused: invalid --outcome-record JSON: " + perr.Error())
	}
	// #1803 SR-1803-2: refuse a record whose ts is more than deskkit.MaxClockSkew ahead of the
	// writer's own clock, so an unbounded future ts can never land and later shadow every
	// genuine outcome for its brief under LatestPerBrief's newest-ts comparison.
	if deskkit.FutureTS(rec.TS, nowFn()) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: --outcome-record ts %s is more than %s ahead of now", rec.TS, deskkit.MaxClockSkew))
	}
	targetRepoPath, nerr := deskkit.RecordName(rec.Raw)
	if nerr != nil {
		return deskkit.Refused("refused: cannot name --outcome-record: " + nerr.Error())
	}
	// #1803 SR-1803-3: a second, independent check on the RESOLVED target, run after RecordName
	// has already computed it and sharing no code with RecordName/SplitBriefKey's own
	// validation, so a regression in either does not also blind this one.
	if perr := deskkit.UnderOutcomeRecordsDir(targetRepoPath); perr != nil {
		return deskkit.Refused("refused: --outcome-record target failed the path-prefix guard: " + perr.Error())
	}
	ac.file = targetRepoPath
	commitContent := deskkit.CanonicalBytes(rec.Raw)

	if len(commitContent) > maxBytes {
		return deskkit.Refused(fmt.Sprintf("refused: outcome record exceeds %d bytes (%d)", maxBytes, len(commitContent)))
	}

	// PUBLISH-identity gate — see cmdEvidence's own call for the full rationale (#1490 lane B).
	if ierr := publishIdentityGate(root, branch); ierr != nil {
		return ierr
	}

	if merr := mintTokenFn(repoSlug); merr != nil {
		return merr
	}
	fg, fr, ferr := forgeForFn(owner, name)
	if ferr != nil {
		return ferr
	}

	remoteExists := false
	var remoteContent []byte
	cur, cerr := fg.ReadFile(fr, deskkit.ReadFileInput{File: targetRepoPath, Ref: branch})
	if cerr != nil {
		if !deskkit.IsForgeNotFound(cerr) {
			return cerr
		}
	} else {
		remoteExists, remoteContent = cur.Exists, cur.Content
	}

	if remoteExists {
		if bytes.Equal(remoteContent, commitContent) {
			ac.successResult = deskkit.ResultNoop
			ac.detail = fmt.Sprintf("noop: %s already recorded on %s (sha %s)", targetRepoPath, branch, shortSHA(cur.SHA))
			fmt.Fprintln(stdout, ac.detail)
			return nil
		}
		return deskkit.Refused("refused: " + targetRepoPath + " already exists on " + branch +
			" with DIFFERENT bytes — verify-outcome records are immutable; a correction is a new record " +
			"(a fresh ts/digest), never an edit of an existing one")
	}

	// A brand-new file: scan it whole (no base to diff against).
	if berr := evidenceOutboundCheck(repoSlug, targetRepoPath, commitContent, commitContent); berr != nil {
		return berr
	}

	// Public-repo write gate — see cmdEvidence's own call for the full rationale.
	fetcher := deskkit.ForgeRepoInfoFetcher{Forge: fg}
	if gerr := publicRepoGateFn(fetcher, owner, name); gerr != nil {
		return gerr
	}

	bodyDig := deskkit.Sha256Hex(commitContent)
	ac.bodyDig = bodyDig

	lintRoot := root
	if lintRoot == "" {
		lintRoot = "."
	}

	// The verified-outcome closure gates apply to this record exactly as they apply to an
	// added log line today: "before" is empty (a brand-new file), "after" is the record's own
	// canonical bytes. isVerifyOutcomesSidecar recognises this per-file path, so the gates
	// activate for a `verified` record precisely as they did for an appended log line.
	if err := outcomeGuardFn(lintRoot, targetRepoPath, nil, commitContent, fg, fr, branch); err != nil {
		return err
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
	if verr := gateVerifiedSidecarLanding(targetRepoPath, lintRoot, nil, commitContent, false); verr != nil {
		return verr
	}

	// Receipt validation (#882 Task step 3): a record carrying
	// wake_schema:verify-wake-v1 additionally refuses the three recurring defects reviewers
	// keep bouncing Evidence PRs for. A legacy-shape verified/verify-fail record (no
	// wake_schema) is not receipt-validated, as today.
	var wr deskkit.WakeReceipt
	if jerr := json.Unmarshal(rec.Raw, &wr); jerr != nil {
		return deskkit.Refused("refused: invalid --outcome-record JSON: " + jerr.Error())
	}
	if wr.Schema == deskkit.SchemaWakeV1 {
		if verr := validateReceipt(lintRoot, fg, fr, branch, repoSlug, wr); verr != nil {
			return verr
		}
	}

	if werr := deskkit.AllowWrite(toolName, repoSlug, 0); werr != nil {
		return werr
	}

	commitSuffix, oerr := deskkit.OnBehalfOfCommitSuffix("", repoSlug)
	if oerr != nil {
		return oerr
	}

	res, werr := fg.WriteFile(fr, deskkit.WriteFileInput{
		File:    targetRepoPath,
		Branch:  branch,
		Content: commitContent,
		Message: "Evidence: verify-outcome record for " + wr.Brief + commitSuffix,
	})
	if werr != nil {
		return werr
	}

	if res.DefaultBranchNotWritable {
		return landOutcomeRecordAsChange(fg, fr, repoSlug, branch, targetRepoPath, commitContent, wr.Brief, ac)
	}

	attr, aerr := checkAttribution(fg, fr, res.SHA, res.Author)
	ac.detail = fmt.Sprintf("committed %s to %s on %s (sha %s) — %s", targetRepoPath, repoSlug, branch, shortSHA(res.SHA), attr)
	if aerr != nil {
		return aerr
	}
	fmt.Fprintf(stdout, "committed %s to %s on %s (sha %s) — %s\n", targetRepoPath, repoSlug, branch, shortSHA(res.SHA), attr)
	return nil
}

// landOutcomeRecordAsChange mirrors landEvidenceAsChange for the per-file record shape: on a
// forge whose default branch takes no direct write (GitLab), the record lands on a new side
// branch and a draft change is opened from it.
func landOutcomeRecordAsChange(fg deskkit.Forge, fr deskkit.ForgeRepo, repoSlug, base, target string, content []byte, brief string, ac *auditCtx) error {
	dig := deskkit.Sha256Hex(content)
	side := "evidence/" + sanitizeBranchComponent(filepath.Base(target)) + "-" + dig[:8]

	commitSuffix, oerr := deskkit.OnBehalfOfCommitSuffix("", repoSlug)
	if oerr != nil {
		return oerr
	}

	res, werr := fg.WriteFile(fr, deskkit.WriteFileInput{
		File:        target,
		Branch:      side,
		Content:     content,
		Message:     "Evidence: verify-outcome record for " + brief + commitSuffix,
		StartBranch: base,
	})
	if werr != nil {
		return werr
	}
	if res.DefaultBranchNotWritable {
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: the forge reported side branch %s not directly writable either — the "+
				"verify-outcome record was NOT landed", side), nil)
	}

	// CreateHeldDraftChange, never the raw CreateDraftChange — see landEvidenceAsChange (#2254).
	pr, perr := deskkit.CreateHeldDraftChange(fg, fr, deskkit.DraftChangeInput{
		Title: "Evidence: " + target,
		Body: "Verify-outcome record for `" + brief + "`, landed on branch `" + side + "` and opened as a " +
			"draft change because the default branch `" + base + "` takes no direct write on this forge.",
		Head: side,
		Base: base,
	})
	if perr != nil {
		if pr != nil {
			ac.detail = fmt.Sprintf("landed %s on %s in %s via draft change #%d, but its merge-hold was NOT opened",
				target, repoSlug, side, pr.Number)
		}
		return perr
	}

	headSHA := ""
	if res.Author == "" && pr != nil {
		if got, gerr := fg.GetPullRequest(fr, pr.Number); gerr == nil && got != nil {
			headSHA = got.HeadSHA
		}
	}
	attr, aerr := checkAttribution(fg, fr, headSHA, res.Author)
	loc := fmt.Sprintf("change #%d", pr.Number)
	if pr.URL != "" {
		loc = pr.URL
	}
	ac.detail = fmt.Sprintf("landed %s on %s in %s via draft %s — default branch not directly writable — %s",
		target, repoSlug, side, loc, attr)
	if aerr != nil {
		return aerr
	}
	fmt.Fprintf(stdout, "landed %s on %s: %s takes no direct write, so wrote branch %s and opened draft %s — %s\n",
		target, repoSlug, base, side, loc, attr)
	return nil
}
