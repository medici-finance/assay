package main

// removesuperseded.go — the `--remove-superseded` landing shape.
//
// Outcome records are immutable: a correction is a newer record, never an edit. A record a
// newer record for the same brief supersedes may still need to leave the tree, and that removal
// is a write like any other: it goes through the same mint, admission, identity gate and write
// window as an --outcome-record landing, and lands as ONE pinned, conditional forge commit (the
// path a verified-outcome write takes), never as a local commit.
//
// Only a superseded outcome record may be removed. Both paths must name records under
// deskkit.OutcomeRecordsDir whose bytes derive exactly that path (deskkit.RecordName), both
// records must name the same brief, and the newer one must win deskkit.LatestPerBriefAt over
// the pair. Anything else is refused before the write.

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// supersededShape is the record-shape check both paths pass before any forge read.
func supersededShape(flagName, p string) error {
	if p != path.Clean(p) || !strings.HasSuffix(p, ".json") {
		return deskkit.Refused("refused: " + flagName + " must be a clean repo-relative outcome record path ending .json, got " + p)
	}
	if err := deskkit.UnderOutcomeRecordsDir(p); err != nil {
		return deskkit.Refused("refused: " + flagName + " is not an outcome record path: " + err.Error())
	}
	return nil
}

// readPinnedRecord reads one outcome record at the pinned commit and proves the bytes are
// the record that path names.
func readPinnedRecord(fg deskkit.Forge, fr deskkit.ForgeRepo, p, sha, use string) (deskkit.OutcomeRecord, *deskkit.FileContent, error) {
	fc, err := fg.ReadFile(fr, deskkit.ReadFileInput{File: p, Ref: sha})
	if err != nil {
		if deskkit.IsForgeNotFound(err) {
			return deskkit.OutcomeRecord{}, nil, deskkit.Refused("refused: " + use + " " + p + " is not on the branch at " + shortSHA(sha))
		}
		return deskkit.OutcomeRecord{}, nil, err
	}
	if kerr := plainFileRead(fc, fr, p, sha, use, false); kerr != nil {
		return deskkit.OutcomeRecord{}, nil, kerr
	}
	if !fc.Exists {
		return deskkit.OutcomeRecord{}, nil, deskkit.Refused("refused: " + use + " " + p + " is not on the branch at " + shortSHA(sha))
	}
	rec, perr := deskkit.ParseRecord(fc.Content)
	if perr != nil {
		return deskkit.OutcomeRecord{}, nil, deskkit.Refused("refused: " + use + " " + p + " is not an outcome record: " + perr.Error())
	}
	name, nerr := deskkit.RecordName(fc.Content)
	if nerr != nil || name != p || !bytes.Equal(deskkit.CanonicalBytes(rec.Raw), fc.Content) {
		return deskkit.OutcomeRecord{}, nil, deskkit.Refused("refused: " + use + " " + p + " does not hold the outcome record its name derives")
	}
	rec.Name = path.Base(p)
	rec.Source = "record:" + p
	return rec, fc, nil
}

// cmdOutcomeRecordRemove removes the superseded record at oldPath, citing newerPath as the
// record that supersedes it.
func cmdOutcomeRecordRemove(oldPath, newerPath, repoSlug, owner, name, branch, root string, dryRun bool, ac *auditCtx) error {
	if err := supersededShape("--remove-superseded", oldPath); err != nil {
		return err
	}
	if err := supersededShape("--superseded-by", newerPath); err != nil {
		return err
	}
	if oldPath == newerPath {
		return deskkit.Refused("refused: --remove-superseded and --superseded-by name the same record")
	}
	if !sameBriefSlot(oldPath, newerPath) {
		return deskkit.Refused("refused: --superseded-by " + newerPath + " is not a record for the same brief as " + oldPath)
	}
	ac.file = oldPath

	fg, fr, receipt, aerr := admitLanding(root, repoSlug, owner, name, oldPath, branch, ac)
	if aerr != nil {
		return aerr
	}
	if err := receipt.CheckEvidenceTarget(oldPath); err != nil {
		return deskkit.Refused("refused: " + oldPath + " is not bound to the admitted run: " + err.Error())
	}
	if ierr := publishEvidenceIdentityGate(root, receipt); ierr != nil {
		return ierr
	}

	tip, err := fg.GetCommit(fr, branch)
	if err != nil || tip == nil || len(tip.SHA) != 40 {
		return deskkit.Unverifiable("cannot resolve the exact branch head the removal is judged on", err)
	}
	oldRec, oldFile, err := readPinnedRecord(fg, fr, oldPath, tip.SHA, "the superseded record")
	if err != nil {
		return err
	}
	newRec, _, err := readPinnedRecord(fg, fr, newerPath, tip.SHA, "the superseding record")
	if err != nil {
		return err
	}
	if oldRec.Brief != newRec.Brief {
		return deskkit.Refused(fmt.Sprintf("refused: %s names brief %q and %s names %q; only a record for the same brief supersedes",
			oldPath, oldRec.Brief, newerPath, newRec.Brief))
	}
	latest, _ := deskkit.LatestPerBriefAt([]deskkit.OutcomeRecord{oldRec, newRec}, nowFn())
	if w, ok := latest[oldRec.Brief]; !ok || w.Digest != newRec.Digest || w.Name != newRec.Name {
		return deskkit.Refused("refused: " + newerPath + " does not supersede " + oldPath +
			" (the newer ts wins; a record is removed only when a newer record for its brief stands)")
	}

	fetcher := deskkit.ForgeRepoInfoFetcher{Forge: fg}
	if gerr := publicRepoGateFn(fetcher, owner, name); gerr != nil {
		return gerr
	}
	addVisibilityRecheck(ac, owner, name)

	resolveSuffix := func() (string, error) { return deskkit.OnBehalfOfCommitSuffix("", repoSlug) }
	commitSuffix, oerr := resolveSuffix()
	if oerr != nil {
		return oerr
	}
	ac.addRecheck(func() error {
		again, err := resolveSuffix()
		if err != nil || again != commitSuffix {
			return changedSincePrep("the on-behalf-of principal", err)
		}
		return nil
	})
	ac.bodyDig = deskkit.Sha256Hex(oldFile.Content)
	if dryRun {
		ac.successResult = deskkit.ResultNoop
		ac.detail = fmt.Sprintf("dry-run: would remove %s on %s via conditional forge write for %s (superseded by %s)",
			oldPath, branch, repoSlug, newerPath)
		fmt.Fprintf(stdout, "dry-run plan: conditional forge write to %s — remove %s on branch %s at %s (superseded by %s for %s), "+
			"authenticated as the verifier App (%s); no local `git commit` in this plan\n",
			repoSlug, oldPath, branch, shortSHA(tip.SHA), newerPath, oldRec.Brief, deskkit.RoleAppLoginOrEmpty("verifier"))
		return nil
	}

	if werr := ac.enterWriteWindow(toolName, repoSlug, 0); werr != nil {
		return werr
	}
	res, werr := fg.WriteFile(fr, deskkit.WriteFileInput{
		File:         oldPath,
		Branch:       branch,
		Delete:       true,
		ExpectedSHA:  oldFile.SHA,
		ExpectedHead: tip.SHA,
		Message: "Evidence: remove superseded verify-outcome record for " + oldRec.Brief +
			"\n\nSuperseded-by: " + newerPath + ac.attestationTrailer() + commitSuffix,
	})
	if werr != nil {
		return werr
	}
	if res == nil || res.DefaultBranchNotWritable || !res.Changed {
		return deskkit.Unverifiable("the pinned removal of "+oldPath+" was not confirmed as a landed commit; reconcile the branch before retrying", nil)
	}
	attr, aerr := checkWriteAttribution(fg, fr, res)
	ac.detail = fmt.Sprintf("removed %s from %s on %s (sha %s) — %s", oldPath, repoSlug, branch, string(res.CommitSHA), attr)
	if aerr != nil {
		return aerr
	}
	fmt.Fprintln(stdout, ac.detail)
	return nil
}
