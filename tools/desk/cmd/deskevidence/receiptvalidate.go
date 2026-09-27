package main

// receiptvalidate.go — #882 Task step 3's receipt validation: three recurring
// verify-wake-v1 receipt defects reviewers kept bouncing Evidence PRs for (medici-finance/assay
// issue #882 comment, 2026-09-27), now refused at the writer instead of caught per-PR in review.
//
//   - (a) `inputs` must name every declared deliverable the brief's `## Context` `files:` block
//     lists, read from the brief AT THE RECORD's sha (never the working tree).
//   - (b) `blocker_ref` must be an actual reference (#<N>, <owner>/<repo>#<N>, or a forge
//     issue/PR/run URL) that the configured forge can read — never free text.
//   - (c) the receipt's own `file:<brief-path>` hash must equal the brief AS IT LANDS on the
//     target branch (which already carries this landing's own Evidence append), read from the
//     forge — never the pre-Evidence copy the receipt might have hashed at wake-evaluation time.
//
// Only a record carrying `wake_schema:verify-wake-v1` is validated; a legacy-shape
// verified/verify-fail record (no wake_schema) is untouched, as today.

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// validateReceipt runs all three checks in order for a verify-wake-v1 record. root is the local
// checkout (--root, or "." — the SAME tree the statusgen PROBLEM-diff guard already lints)
// used ONLY to resolve the brief's own repo-relative path and to read it at the record's OWN sha
// (a git-local, offline read: the record's sha is already-landed history, unlike (c)'s branch-tip
// read, which must go through the forge). fg/fr/branch are used for the forge reads (b) and (c)
// need.
func validateReceipt(root string, fg deskkit.Forge, fr deskkit.ForgeRepo, branch, repoSlug string, wr deskkit.WakeReceipt) error {
	defaultOwner, defaultName := fr.Owner, fr.Name
	if wr.Repo != "" {
		if o, n, ok := splitRepo(wr.Repo); ok {
			defaultOwner, defaultName = o, n
		}
	}
	if err := validateReceiptInputsCoverDeliverables(root, wr); err != nil {
		return err
	}
	if err := validateReceiptBlockerRef(fg, defaultOwner, defaultName, wr.BlockerRef); err != nil {
		return err
	}
	if err := validateReceiptBriefHashAsLanded(fg, fr, branch, root, wr); err != nil {
		return err
	}
	return nil
}

// --- (a) inputs coverage ------------------------------------------------------------------

// filesBlockStartRe anchors the `files:` line inside a brief's `## Context` section — a bare
// line, no leading indentation, per the brief-v2 authoring convention.
var filesBlockStartRe = regexp.MustCompile(`(?m)^files:\s*$`)

// backtickSpanRe extracts every backtick-quoted span in the files: block. Every span is treated
// as a CANDIDATE declared path; one that is not a real repo path at the record's sha (a bare
// identifier, a glob/placeholder pattern such as `<stream>`) simply resolves ABSENT and is
// therefore never required — see validateReceiptInputsCoverDeliverables.
var backtickSpanRe = regexp.MustCompile("`([^`]+)`")

// parseDeclaredFiles returns the distinct backtick-quoted spans inside the brief's `## Context`
// `files:` block, in first-seen order. The block runs from the `files:` line to the first blank
// line (the brief-v2 convention every authored brief in this repo follows); an absent `files:`
// line is an empty declared set, never an error — not every brief-v2 file necessarily carries one.
func parseDeclaredFiles(content string) []string {
	loc := filesBlockStartRe.FindStringIndex(content)
	if loc == nil {
		return nil
	}
	block := content[loc[1]:]
	if end := strings.Index(block, "\n\n"); end >= 0 {
		block = block[:end]
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range backtickSpanRe.FindAllStringSubmatch(block, -1) {
		p := strings.TrimSpace(m[1])
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// localBriefPath resolves a brief key to its repo-relative file path via the SAME glob
// convention verifyloop's resolveBrief uses (docs/streams/<stream>/brief-<NN>-*.md) against the
// LOCAL checkout at root. This is a naming-convention lookup only — it never assumes the file's
// CONTENT at that path is current; (a) reads the content at the record's own sha via git, (c)
// reads it on the target branch via the forge.
func localBriefPath(root, stream, num string) (string, error) {
	pattern := filepath.Join(root, "docs", "streams", stream, "brief-"+num+"-*.md")
	matches, _ := filepath.Glob(pattern)
	if len(matches) == 0 {
		return "", fmt.Errorf("brief file not found: %s", pattern)
	}
	rel, err := filepath.Rel(root, matches[0])
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// gitShowAt reads a repo-relative path at a git ref from the LOCAL checkout at root — an
// offline, git-plumbing read (no forge call) of already-landed history. Used only for (a): the
// record's own sha is necessarily already in the past, so the local checkout (which every
// verify-desk invocation of deskevidence runs from) already carries it.
func gitShowAt(root, sha, relPath string) ([]byte, error) {
	cmd := exec.Command("git", "-C", root, "show", sha+":"+relPath)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

// gitPathKindAt reports whether relPath is a blob (file) or tree (directory) at sha in the local
// checkout at root. exists is false whenever the object cannot be resolved — deliberately not
// distinguished from a git-transport failure, because gitShowAt has already proven sha itself
// resolves in this checkout (it is called only after a successful gitShowAt of the brief file at
// the same sha), so any failure here is attributable to relPath simply being absent at that sha —
// exactly the "(planned) file not yet written" case the brief's rule exempts.
func gitPathKindAt(root, sha, relPath string) (kind string, exists bool) {
	cmd := exec.Command("git", "-C", root, "cat-file", "-t", sha+":"+relPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return strings.TrimSpace(out.String()), true
}

// validateReceiptInputsCoverDeliverables is check (a): every backticked path the brief's
// `## Context` `files:` block declares, read from the brief AT THE RECORD'S sha, must have
// either its own `file:<path>` input key (a file) or at least one `file:` key nested under it (a
// directory) — UNLESS the path is absent at that sha (a `(planned)` deliverable not yet
// written), which is never required.
func validateReceiptInputsCoverDeliverables(root string, wr deskkit.WakeReceipt) error {
	stream, num, kerr := deskkit.SplitBriefKey(wr.Brief)
	if kerr != nil {
		return deskkit.Refused("refused: receipt brief key " + wr.Brief + " is invalid: " + kerr.Error())
	}
	sha := strings.TrimSpace(wr.SHA)
	if sha == "" {
		return deskkit.Unverifiable("receipt for "+wr.Brief+" carries no sha to read its declared deliverables at", nil)
	}
	briefPath, perr := localBriefPath(root, stream, num)
	if perr != nil {
		return deskkit.Unverifiable("cannot resolve the brief file for "+wr.Brief+" under "+root+": "+perr.Error(), perr)
	}
	content, gerr := gitShowAt(root, sha, briefPath)
	if gerr != nil {
		return deskkit.Unverifiable("cannot read "+briefPath+" at "+sha+" (the record's own sha): "+gerr.Error(), gerr)
	}
	for _, raw := range parseDeclaredFiles(string(content)) {
		// A directory entry may be written with a trailing slash (`foo/`); strip it before
		// resolving at sha or building the nested `file:` key, so the key this builds is
		// `file:foo`, matching how a nested entry (`file:foo/bar.go`) is actually spelled.
		p := strings.TrimSuffix(raw, "/")
		if p == "" {
			continue
		}
		kind, exists := gitPathKindAt(root, sha, p)
		if !exists {
			continue // absent at this sha — a "(planned)" deliverable, never required
		}
		key := "file:" + p
		switch kind {
		case "blob":
			if _, ok := wr.Inputs[key]; !ok {
				return deskkit.Refused("refused: receipt inputs omits declared deliverable " + key +
					" for " + wr.Brief + " (declared in ## Context files: at " + sha + ")")
			}
		case "tree":
			prefix := key + "/"
			covered := false
			for k := range wr.Inputs {
				if strings.HasPrefix(k, prefix) {
					covered = true
					break
				}
			}
			if !covered {
				return deskkit.Refused("refused: receipt inputs has no file: key under declared directory " +
					p + " for " + wr.Brief + " (declared in ## Context files: at " + sha + ")")
			}
		}
	}
	return nil
}

// --- (b) blocker_ref must be an actual reference ------------------------------------------

var (
	blockerRefBareRe = regexp.MustCompile(`^#([0-9]+)$`)
	blockerRefRepoRe = regexp.MustCompile(`^([\w.-]+)/([\w.-]+)#([0-9]+)$`)
	blockerRefURLRe  = regexp.MustCompile(`^https?://[^/]+/([\w.-]+)/([\w.-]+)/(issues|pull|actions/runs)/([0-9]+)(?:[/?#].*)?$`)
)

// validateReceiptBlockerRef is check (b): blocker_ref must match #<N>, <owner>/<repo>#<N>, or a
// forge URL to an issue/PR/run — parsed into owner/repo/number, NEVER fetched as a URL — and the
// writer then reads that issue/PR/run through the configured forge API. A well-formed reference
// the forge does not have is refused; a read that fails for any other reason is could-not-check.
func validateReceiptBlockerRef(fg deskkit.Forge, defaultOwner, defaultName, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return deskkit.Refused("refused: receipt blocker_ref is empty — must be #<N>, <owner>/<repo>#<N>, or a forge issue/PR/run URL")
	}

	var owner, name, kind, runID string
	var number int
	switch {
	case blockerRefBareRe.MatchString(raw):
		m := blockerRefBareRe.FindStringSubmatch(raw)
		owner, name, kind = defaultOwner, defaultName, "numbered"
		number, _ = strconv.Atoi(m[1])
	case blockerRefRepoRe.MatchString(raw):
		m := blockerRefRepoRe.FindStringSubmatch(raw)
		owner, name, kind = m[1], m[2], "numbered"
		number, _ = strconv.Atoi(m[3])
	case blockerRefURLRe.MatchString(raw):
		m := blockerRefURLRe.FindStringSubmatch(raw)
		owner, name = m[1], m[2]
		switch m[3] {
		case "issues":
			kind = "issue"
			number, _ = strconv.Atoi(m[4])
		case "pull":
			kind = "change"
			number, _ = strconv.Atoi(m[4])
		case "actions/runs":
			kind = "run"
			runID = m[4]
		}
	default:
		return deskkit.Refused("refused: receipt blocker_ref " + strconv.Quote(raw) +
			" is not a reference — must be #<N>, <owner>/<repo>#<N>, or a forge issue/PR/run URL (free text and " +
			"an action: … sentence are refused)")
	}

	repo := deskkit.ForgeRepo{Owner: owner, Name: name}
	var lerr error
	switch kind {
	case "run":
		_, lerr = fg.RunStatus(repo, deskkit.RunRef{ID: runID})
	case "change":
		_, lerr = fg.GetIssueTyped(repo, number, deskkit.TargetChange)
	default: // "numbered" (ambiguous on GitHub) and "issue" both read via GetIssue
		_, lerr = fg.GetIssue(repo, number)
	}
	if lerr == nil {
		return nil
	}
	if deskkit.IsForgeNotFound(lerr) {
		return deskkit.Refused("refused: receipt blocker_ref " + raw + " does not exist on the forge")
	}
	return deskkit.Unverifiable("could not verify receipt blocker_ref "+raw, lerr)
}

// --- (c) the brief's file: revision must be the brief AS IT LANDS -------------------------

// validateReceiptBriefHashAsLanded is check (c): the receipt's own `file:<brief-path>` input
// must equal the SHA-256 of the brief as it stands on the TARGET BRANCH right now — the copy
// this very landing's own Evidence append already lands ahead of the outcome record (Task step
// 3: the outcome record is always the LAST write of a landing) — read from the forge, never the
// local checkout (which may not have fetched the latest branch tip).
func validateReceiptBriefHashAsLanded(fg deskkit.Forge, fr deskkit.ForgeRepo, branch, root string, wr deskkit.WakeReceipt) error {
	stream, num, kerr := deskkit.SplitBriefKey(wr.Brief)
	if kerr != nil {
		return deskkit.Refused("refused: receipt brief key " + wr.Brief + " is invalid: " + kerr.Error())
	}
	briefPath, perr := localBriefPath(root, stream, num)
	if perr != nil {
		return deskkit.Unverifiable("cannot resolve the brief file for "+wr.Brief+" under "+root+": "+perr.Error(), perr)
	}
	key := "file:" + briefPath
	wantHash, ok := wr.Inputs[key]
	if !ok {
		return deskkit.Refused("refused: receipt inputs has no " + key + " to compare against the brief as it lands on " + branch)
	}
	cur, rerr := fg.ReadFile(fr, deskkit.ReadFileInput{File: briefPath, Ref: branch})
	if rerr != nil {
		return deskkit.Unverifiable("cannot read "+briefPath+" on "+branch+" to check the receipt's brief revision", rerr)
	}
	gotHash := deskkit.Sha256Hex(cur.Content)
	if !strings.EqualFold(strings.TrimSpace(wantHash), gotHash) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: receipt %s does not match %s AS IT LANDS on %s (receipt=%s, landed=%s) — the brief's "+
				"file: revision must be hashed AFTER this landing's own Evidence append; re-issue the record",
			key, briefPath, branch, wantHash, gotHash))
	}
	return nil
}
