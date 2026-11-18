package main

// flipoutcome.go — the verify-outcome cross-check of `statusgen verifyflip`
// (#6734).
//
// THE GAP IT CLOSES. verifyflip takes its verdict from the latest strict
// `**VERIFY: PASS**` in Evidence. The same verify run also writes a
// verify-outcome record (one file per run under outcomeRecordsDir), and the
// two can disagree: an Evidence entry can read PASS while the run's
// own record says `blocked`. Without this check the flip acted on the
// Evidence alone.
//
// THE RULE. The flip is allowed only when the brief's LATEST verify-outcome
// record (latestOutcomePerBrief: newest ts, the record layer's own reduction)
// names the revision being flipped AND says `verified`. Each disagreement is a
// refusal that names which of the three it was:
//
//   - outcome record absent: no record names the brief;
//   - outcome record revision mismatch: the record names another revision;
//   - outcome record result mismatch: the record's outcome is not `verified`.
//
// The record's revision is `source_sha`, the commit the verify run observed
// (the one its Runner cells name as `<login> @ <sha>`). A record that carries
// no `source_sha` is an older shape whose only revision is `sha`, so `sha` is
// read then — but on such records `sha` is usually the commit the Evidence
// landed at, not the run's revision, so a mismatch there says so and asks for
// a re-run that writes `source_sha`.
//
// WHICH RECORDS COUNT. The reduction is only as good as its inputs, so before
// trusting it every record naming the brief must be datable and in place:
//
//   - its `ts` parses as RFC 3339 — the reduction ranks an unparseable ts as
//     the oldest, so a newer `blocked` record would silently lose to an older
//     `verified` one;
//   - a per-file record sits at the path outcomeRecordName gives for its own
//     bytes (its brief's stream directory, its ts, its digest) — a record
//     filed under another stream, or edited after it was named, is not
//     counted silently;
//   - no legacy log line naming the brief was skipped by the legacy
//     tolerance (malformed, or no computable name).
//
// Any of these is could-not-check. And the records must be the committed
// ones, written by the verifier: the brief's stream record directory and the
// legacy logs must be clean against HEAD (an untracked, modified or deleted
// record refuses, like uncommitted Evidence), and the commit that added the
// deciding record must be the bound verifier's or a roster human's, judged
// on the raw commit author exactly as flipProvenance judges Evidence. A
// deciding legacy log line has no adding commit of its own, so it is
// could-not-check.
//
// Both reads (the clean check and the adding-commit walk) go through the
// run's typed reader (gitCoreFor), never a native git launch: a repository
// the typed reader declines is could-not-check, and so is a history it
// cannot read in full. The typed status read trusts the index's cached file
// metadata, so every tracked record file in scope is also compared by content
// against its HEAD blob.
//
// Known differences from native `git status`, each failing closed (a refused
// legitimate flip, never a passed one): the typed read does not honour
// `.git/info/exclude` or a `core.excludesFile` (repo or global), so an
// untracked file ignored only there refuses; it does not honour
// `core.fileMode=false`, so an exec-bit change on a record refuses; and a
// split index is could-not-check.
//
// KNOWN LIMIT. A record DELETED in an earlier commit by someone other than
// the verifier leaves no file to judge; the history walk flipProvenance does
// for Evidence is not repeated for the record directory. The deciding record
// itself is always the verifier's.
//
// A record store that cannot be read, a record that is not valid JSON, a
// record with no revision, or a record dated beyond the clock-skew tolerance
// (it never wins the reduction, so the latest record is unknown) is
// could-not-check: the flip refuses, fail-closed, and never reads it as
// agreeing.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/statusgen/evaluation"
)

// flipOutcomeCheck cross-checks the brief's latest verify-outcome record under
// root against a flip of key to verified at wantSHA. A *flipRefusal refuses;
// any other error is could-not-check.
func flipOutcomeCheck(root, key, wantSHA string) error {
	records, skipped, err := readVerifyOutcomeRecordsSkips(root)
	if err != nil {
		return fmt.Errorf("outcome record: verify-outcome records unreadable: %v", err)
	}
	if err := flipOutcomeInputs(records, skipped, key); err != nil {
		return err
	}
	if err := flipOutcomeCommitted(root, key); err != nil {
		return err
	}
	latest, future := latestOutcomePerBrief(records)
	if future[key] {
		return fmt.Errorf("outcome record: a verify-outcome record for %s is dated beyond the clock-skew tolerance — it never wins, so the latest record is unknown", key)
	}
	rec, ok := latest[key]
	if !ok {
		return refuseFlip("outcome record absent: no verify-outcome record names %s — the flip needs the record its verify run wrote", key)
	}
	var o struct {
		Outcome   string `json:"outcome"`
		SHA       string `json:"sha"`
		SourceSHA string `json:"source_sha"`
	}
	if err := json.Unmarshal(rec.Raw, &o); err != nil {
		return fmt.Errorf("outcome record: %s cannot be read: %v", rec.Source, err)
	}
	rev, legacyRev := strings.TrimSpace(o.SourceSHA), false
	if rev == "" {
		rev, legacyRev = strings.TrimSpace(o.SHA), true
	}
	if !flipShaRe.MatchString(rev) {
		return fmt.Errorf("outcome record: %s names no readable revision (source_sha/sha %q)", rec.Source, rev)
	}
	r, w := strings.ToLower(rev), strings.ToLower(wantSHA)
	if !strings.HasPrefix(r, w) && !strings.HasPrefix(w, r) {
		if legacyRev {
			return refuseFlip("outcome record revision mismatch: the latest verify-outcome record for %s (%s) carries no source_sha, and its sha %s is not the flip's %s — on an older record sha is usually the Evidence landing commit, not the run's revision; re-run the verify so it writes a record with source_sha", key, rec.Source, rev, wantSHA)
		}
		return refuseFlip("outcome record revision mismatch: the latest verify-outcome record for %s (%s) names %s, the flip is at %s", key, rec.Source, rev, wantSHA)
	}
	if evaluation.ParseVerificationOutcome(o.Outcome) != evaluation.OutcomeVerified {
		return refuseFlip("outcome record result mismatch: the latest verify-outcome record for %s (%s) says outcome %q, the flip requests verified", key, rec.Source, o.Outcome)
	}
	return flipOutcomeAuthor(root, rec)
}

// flipOutcomeInputs refuses to trust the reduction for key when any record
// naming it cannot be dated or is not where its own bytes say it belongs, or
// when the legacy tolerance skipped a log line naming it. Each is
// could-not-check.
func flipOutcomeInputs(records []outcomeRecord, skipped []outcomeSkipped, key string) error {
	for _, rec := range records {
		if rec.Brief != key {
			continue
		}
		if _, err := time.Parse(time.RFC3339, rec.TS); err != nil {
			return fmt.Errorf("outcome record: %s names %s with ts %q, which is not RFC 3339 — it cannot be ranked, so the latest record is unknown", rec.Source, key, rec.TS)
		}
		if rec.Path == "" {
			continue
		}
		want, err := outcomeRecordName(rec.Raw)
		if err != nil {
			return fmt.Errorf("outcome record: %s names %s but has no record name: %v", rec.Source, key, err)
		}
		if want != rec.Path {
			return fmt.Errorf("outcome record: %s names %s but its bytes belong at %s — a misfiled or edited record is not counted", rec.Source, key, want)
		}
	}
	quoted := []byte(fmt.Sprintf("%q", key))
	for _, sk := range skipped {
		var head struct {
			Brief string `json:"brief"`
		}
		if json.Unmarshal(sk.Line, &head) == nil && head.Brief != key {
			continue
		}
		if head.Brief == key || bytes.Contains(sk.Line, quoted) {
			return fmt.Errorf("outcome record: a legacy line in %s naming %s was skipped as unreadable (malformed or undatable) — the latest record is unknown", sk.Source, key)
		}
	}
	return nil
}

// flipOutcomeCommitted refuses when the records the check reads for key are
// not the committed ones: anything untracked, modified or deleted under the
// brief's stream record directory or in a legacy log.
func flipOutcomeCommitted(root, key string) error {
	stream, _, err := splitOutcomeBriefKey(key)
	if err != nil {
		return fmt.Errorf("outcome record: %v", err)
	}
	read, err := flipOutcomeRepo(root)
	if err != nil {
		return fmt.Errorf("outcome record: git status: %v", err)
	}
	status, err := read.repo.DirtyPorcelain()
	if err != nil {
		return fmt.Errorf("outcome record: git status: %v", err)
	}
	if dirty := outcomeStatusLines(status, read.prefix, stream); len(dirty) > 0 {
		return refuseFlip("outcome record: uncommitted verify-outcome record changes for %s (%s) — an uncommitted record has no author to judge; commit it as the verifier or discard it", key, dirty[0])
	}
	return flipOutcomeHeadBytes(root, read, stream)
}

// flipOutcomeHeadBytes compares every tracked record file in scope (the
// brief's stream record directory and the legacy logs) against its HEAD blob
// by content, as flipProvenance compares Evidence against HEAD. The typed
// status read decides "unchanged" from the index's cached file metadata, so
// this content comparison is what binds the files the check reads to HEAD.
func flipOutcomeHeadBytes(root string, read gitCoreRead, stream string) error {
	files, err := read.repo.Files("HEAD")
	if err != nil {
		return fmt.Errorf("outcome record: reading the HEAD tree: %v", err)
	}
	dir, glob := outcomeScope(stream)
	for _, f := range files {
		rel, ok := strings.CutPrefix(f, read.prefix)
		if !ok || !outcomeInScope(rel, dir, glob) {
			continue
		}
		head, err := read.repo.FileAt("HEAD", f)
		if err != nil {
			return fmt.Errorf("outcome record: git show HEAD:%s: %v", f, err)
		}
		work, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || strings.ReplaceAll(string(work), "\r\n", "\n") != strings.ReplaceAll(head, "\r\n", "\n") {
			return refuseFlip("outcome record: uncommitted verify-outcome record changes (%s differs from HEAD) — an uncommitted record has no author to judge; commit it as the verifier or discard it", rel)
		}
	}
	return nil
}

// outcomeScope is the brief's stream record directory and the legacy-log
// pathspec glob, both relative to root.
func outcomeScope(stream string) (string, *regexp.Regexp) {
	return path.Join(outcomeRecordsDir, stream), outcomePathspecGlob(path.Join("docs/streams", verifyOutcomesGlob))
}

// outcomeInScope reports whether rel falls under dir or matches glob.
func outcomeInScope(rel, dir string, glob *regexp.Regexp) bool {
	return rel == dir || strings.HasPrefix(rel, dir+"/") || glob.MatchString(rel)
}

// flipOutcomeRepo opens root through the run's typed reader. The record checks
// read only through it: a repository whose profile the typed reader declines
// (it names a native reason) is could-not-check, never a native retry.
func flipOutcomeRepo(root string) (gitCoreRead, error) {
	read := gitCoreFor(root)
	if read.err != nil {
		return read, read.err
	}
	if read.native != "" {
		return read, fmt.Errorf("could-not-check: the typed reader declines this repository (%s)", read.native)
	}
	return read, nil
}

// outcomeStatusLines keeps the porcelain lines (top-relative paths, as
// DirtyPorcelain and `git status --porcelain` print them) whose path falls
// under the two pathspecs the check reads, taken relative to root (prefix is
// root's top-relative slash prefix): the brief's stream record directory, and
// docs/streams/<verifyOutcomesGlob>, matched as a Git pathspec glob, where a
// wildcard also matches '/'.
func outcomeStatusLines(porcelain, prefix, stream string) []string {
	dir, glob := outcomeScope(stream)
	var keep []string
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 4 {
			continue
		}
		for _, p := range strings.Split(line[3:], " -> ") {
			rel, ok := strings.CutPrefix(p, prefix)
			if !ok {
				continue
			}
			if outcomeInScope(rel, dir, glob) {
				keep = append(keep, line)
				break
			}
		}
	}
	return keep
}

// outcomePathspecGlob compiles a Git pathspec glob in its default (non-
// :(glob)) form: '*' and '?' match any character, '/' included.
func outcomePathspecGlob(spec string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for _, r := range spec {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

// outcomeAddingCommits lists, over the history reachable from HEAD, the
// commits that add rel (a slash path relative to the reader's root) — the set
// `git log --full-history --diff-filter=A HEAD -- <rel>` prints. A history the
// walk cannot read in full is an error, never an empty set.
func outcomeAddingCommits(read gitCoreRead, rel string) ([]string, error) {
	var commits []string
	err := read.repo.PathHistory("HEAD", read.prefix+path.Clean(rel), func(c gitcore.PathCommit) bool {
		if outcomeAddingCommit(c) {
			commits = append(commits, c.Hash)
		}
		return true
	})
	return commits, err
}

// outcomeAddingCommit reports whether c is one `git log --full-history
// --diff-filter=A -- <path>` lists: a non-merge commit (merges carry no diff
// in that log) that holds the path where its parent, or the empty tree of a
// root commit, does not.
func outcomeAddingCommit(c gitcore.PathCommit) bool {
	if c.PathID == "" || len(c.Parents) > 1 {
		return false
	}
	return len(c.Parents) == 0 || c.ParentPathIDs[0] == ""
}

// flipOutcomeAuthor judges the commit(s) that added the deciding record: each
// must be the bound verifier's or a roster human's, by the raw commit author.
func flipOutcomeAuthor(root string, rec outcomeRecord) error {
	if rec.Path == "" {
		return fmt.Errorf("outcome record: the deciding record for %s is a legacy log line (%s), which has no adding commit of its own to judge", rec.Brief, rec.Source)
	}
	p := evidenceActorPolicyFromRoster()
	if p.Unavailable != "" {
		return fmt.Errorf("outcome record: %s", p.Unavailable)
	}
	read, err := flipOutcomeRepo(root)
	if err != nil {
		return fmt.Errorf("outcome record: git log %s: %v", rec.Path, err)
	}
	commits, err := outcomeAddingCommits(read, rec.Path)
	if err != nil {
		return fmt.Errorf("outcome record: git log %s: %v", rec.Path, err)
	}
	if len(commits) == 0 {
		return fmt.Errorf("outcome record: no commit adds %s — its author cannot be judged", rec.Path)
	}
	for _, c := range commits {
		a, err := rawCommitAuthor(root, c)
		if err != nil {
			return fmt.Errorf("outcome record: %s: %v", rec.Path, err)
		}
		if v, why := p.classify(a.Name, a.Email); v != actorVerifier && v != actorHuman {
			return refuseFlip("outcome record: the deciding verify-outcome record %s was not added by the bound verifier or a roster human — %s", rec.Path, why)
		}
	}
	return nil
}
