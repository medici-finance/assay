package deskkit

// cicheckhistory.go — the CI-check history record (ci-check-v1).
//
// The desk tools read every pull request's CI results many times a day (deskflip, deskpost,
// deskautolane, deskboard, deskmonitor, deskroster, deskdisposition) and reduce them to an
// in-memory verdict. This file keeps one small local line per FINISHED check run or commit
// status, taken from reads the tools already make, so "which checks flake, how often and how
// long do they take" can be answered later at no extra forge cost.
//
// The recorder sits in the outbound Forge decorator (outboundforge.go), which is the one
// choke point every desk reader passes through (TestForgeSingleConstructionSite). It is
// strictly best-effort and strictly a SIDE EFFECT of a read:
//
//   - it makes no forge call;
//   - it runs only after the inner read succeeded, and the decorator returns the inner
//     result and error unchanged whatever the recorder does;
//   - a failed write, a lock held past ciLockWait or a panic is dropped (one stderr line per
//     process) and never reaches the caller.
//
// The record type has no field for check output, title, summary, annotations, log text,
// URLs, the actor or app that posted a check, or any session transcript — names and
// conclusions only. docs/streams/desk-supervision/ci-check-v1.md is the canonical schema;
// TestCICheckRecord_NoFreeTextFields holds the two together.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// CICheckSchema is the record's schema tag.
	CICheckSchema = "ci-check-v1"

	// CICheckKindRun / CICheckKindStatus are the two record kinds: a check run (the Actions
	// shape) and a commit status (the legacy shape every non-Actions CI still posts — a
	// required external scanner, a GitLab pipeline).
	CICheckKindRun    = "check-run"
	CICheckKindStatus = "status"

	ciChecksFile = "ci-checks.jsonl"
	ciChecksLock = "ci-checks.lock"
)

// CICheckRecord is one line of ci-checks.jsonl. The field list is the whole schema: adding a
// field here without adding it to ci-check-v1.md (and the reverse) fails
// TestCICheckRecord_NoFreeTextFields.
type CICheckRecord struct {
	Schema      string `json:"schema"`
	ObservedAt  string `json:"observed_at"`
	Tool        string `json:"tool"`
	Repo        string `json:"repo"`
	HeadSHA     string `json:"head_sha"`
	PR          int    `json:"pr,omitempty"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Attempt     string `json:"attempt"`
	Conclusion  string `json:"conclusion"`
	StartedAt   string `json:"started_at,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
	// DurationS is the check run's wall time in whole seconds; nil (omitted) for a status and
	// whenever either stamp is missing, malformed or unordered. A pointer so a genuine 0 s run
	// is kept rather than omitted.
	DurationS *int `json:"duration_s,omitempty"`
}

// ciKey is the dedupe key (repo, head_sha, kind, name, attempt). The unit separator cannot
// occur in any of the five fields as the forges render them.
func (r CICheckRecord) ciKey() string {
	return strings.Join([]string{r.Repo, r.HeadSHA, r.Kind, r.Name, r.Attempt}, "\x1f")
}

// Test hooks in the dirOverride mould: production never sets them and they are not wired to
// any env var or flag.
var (
	ciNow      = time.Now
	ciLockWait = 2 * time.Second
	// ciRecorderEnabled lets a test prove a verdict is identical with recording off.
	ciRecorderEnabled = true
)

var (
	ciMu       sync.Mutex
	ciSeen     = map[string]map[string]struct{}{} // live file path -> keys this process wrote or saw on disk
	ciWarned   bool
	ciWarnSink = func(msg string) { fmt.Fprintln(os.Stderr, msg) }
)

// SetCICheckRecording turns the CI-check recorder on or off and returns a function that
// restores the previous setting. It is a TEST HOOK for packages outside deskkit (a cmd
// package's flow test compares a verdict with recording on and off); production never calls
// it and it is not wired to any env var or flag.
func SetCICheckRecording(enabled bool) (restore func()) {
	ciMu.Lock()
	prev := ciRecorderEnabled
	ciRecorderEnabled = enabled
	ciMu.Unlock()
	return func() {
		ciMu.Lock()
		ciRecorderEnabled = prev
		ciMu.Unlock()
	}
}

// ciResetProcessState clears the per-process dedupe set and the once-per-process warning —
// what a fresh process starts with. Test hook.
func ciResetProcessState() {
	ciMu.Lock()
	defer ciMu.Unlock()
	ciSeen = map[string]map[string]struct{}{}
	ciWarned = false
}

func ciWarnOnce(reason string) {
	ciMu.Lock()
	first := !ciWarned
	ciWarned = true
	sink := ciWarnSink
	ciMu.Unlock()
	if first {
		sink("ci-check-history: batch not recorded (" + reason + "); the forge read is unaffected")
	}
}

func ciEnabled() bool {
	ciMu.Lock()
	defer ciMu.Unlock()
	return ciRecorderEnabled
}

// ciDurationS returns the run's wall time in whole seconds, or nil when either stamp is
// missing, malformed or the run ends before it starts.
func ciDurationS(started, completed string) *int {
	if started == "" || completed == "" {
		return nil
	}
	s, err1 := time.Parse(time.RFC3339, started)
	c, err2 := time.Parse(time.RFC3339, completed)
	if err1 != nil || err2 != nil || c.Before(s) {
		return nil
	}
	d := int(c.Sub(s) / time.Second)
	return &d
}

// ciRunRecord builds the record for one check run, or ok=false when it is not a finished run
// with a usable attempt key. id is CheckRun.ID / RollupNode.ID (may be ""); both are rendered
// by checkRunID, so the REST and GraphQL reads of one run agree on it.
func ciRunRecord(base CICheckRecord, id, name, status, conclusion, started, completed string) (CICheckRecord, bool) {
	if name == "" || strings.ToLower(status) != "completed" {
		return CICheckRecord{}, false
	}
	attempt := id
	if attempt == "" {
		if started == "" {
			return CICheckRecord{}, false
		}
		attempt = "t:" + started
	}
	base.Kind, base.Name, base.Attempt = CICheckKindRun, name, attempt
	base.Conclusion = strings.ToLower(conclusion)
	base.StartedAt, base.CompletedAt = started, completed
	base.DurationS = ciDurationS(started, completed)
	return base, true
}

// ciStatusRecord builds the record for one commit status, or ok=false when it is not a
// terminal state (success | failure | error) with a creation stamp to key the attempt on.
// GitHub's combined-status read returns the latest status per context, so CreatedAt is the
// only per-execution handle a status has.
func ciStatusRecord(base CICheckRecord, context, state, created string) (CICheckRecord, bool) {
	st := strings.ToLower(state)
	if context == "" || created == "" || (st != "success" && st != "failure" && st != "error") {
		return CICheckRecord{}, false
	}
	base.Kind, base.Name, base.Attempt = CICheckKindStatus, context, "t:"+created
	base.Conclusion = st
	return base, true
}

func ciBase(repo ForgeRepo, sha string, pr int, now time.Time) CICheckRecord {
	return CICheckRecord{
		Schema:     CICheckSchema,
		ObservedAt: now.UTC().Format(time.RFC3339),
		Tool:       CanonicalToolKeyOr(toolName()),
		Repo:       repo.Slug(),
		HeadSHA:    sha,
		PR:         pr,
	}
}

// ciRecordsFromChecks turns a ChecksAtHead read into records. The read carries no PR number.
func ciRecordsFromChecks(repo ForgeRepo, sha string, c *ChecksAtHead, now time.Time) []CICheckRecord {
	if c == nil || sha == "" {
		return nil
	}
	base := ciBase(repo, sha, 0, now)
	var out []CICheckRecord
	for _, r := range c.CheckRuns {
		if rec, ok := ciRunRecord(base, r.ID, r.Name, r.Status, r.Conclusion, r.StartedAt, r.CompletedAt); ok {
			out = append(out, rec)
		}
	}
	for _, s := range c.Statuses {
		if rec, ok := ciStatusRecord(base, s.Context, s.State, s.CreatedAt); ok {
			out = append(out, rec)
		}
	}
	return out
}

// ciRecordsFromRollup turns one change's rollup into records. A node the forge did not tag is
// classified by which name field it carries; the GitLab could-not-check sentinel is skipped.
func ciRecordsFromRollup(repo ForgeRepo, oc OpenChange, now time.Time) []CICheckRecord {
	if oc.HeadSHA == "" {
		return nil
	}
	base := ciBase(repo, oc.HeadSHA, oc.Number, now)
	var out []CICheckRecord
	for _, n := range oc.Rollup {
		kind := n.Typename
		switch {
		case kind == GitLabRollupUnmapped:
			continue
		case kind == "" && n.Context != "":
			kind = "StatusContext"
		case kind == "" && n.Name != "":
			kind = "CheckRun"
		}
		switch kind {
		case "CheckRun":
			if rec, ok := ciRunRecord(base, n.ID, n.Name, n.Status, n.Conclusion, n.StartedAt, n.CompletedAt); ok {
				out = append(out, rec)
			}
		case "StatusContext":
			if rec, ok := ciStatusRecord(base, n.Context, n.State, n.CreatedAt); ok {
				out = append(out, rec)
			}
		}
	}
	return out
}

// recordCIBestEffort runs build and writes what it returns. It is the ONLY entry the decorator
// uses, and it never returns an error and never panics: everything inside is recovered and
// reported as a once-per-process stderr line.
func recordCIBestEffort(build func(now time.Time) []CICheckRecord) {
	defer func() {
		if p := recover(); p != nil {
			ciWarnOnce(fmt.Sprintf("recorder panic: %v", p))
		}
	}()
	if !ciEnabled() {
		return
	}
	recs := build(ciNow())
	if len(recs) == 0 {
		return
	}
	if err := recordCIChecks(recs); err != nil {
		ciWarnOnce(err.Error())
	}
}

// ciFreshKeys returns recs minus any key this process already wrote or saw on disk for the
// live file, and minus repeats within the batch.
func ciFreshKeys(live string, recs []CICheckRecord) []CICheckRecord {
	ciMu.Lock()
	defer ciMu.Unlock()
	seen := ciSeen[live]
	batch := map[string]struct{}{}
	var fresh []CICheckRecord
	for _, r := range recs {
		k := r.ciKey()
		if _, ok := seen[k]; ok {
			continue
		}
		if _, ok := batch[k]; ok {
			continue
		}
		batch[k] = struct{}{}
		fresh = append(fresh, r)
	}
	return fresh
}

func ciMarkSeen(live string, keys []string) {
	ciMu.Lock()
	defer ciMu.Unlock()
	seen := ciSeen[live]
	if seen == nil {
		seen = map[string]struct{}{}
		ciSeen[live] = seen
	}
	for _, k := range keys {
		seen[k] = struct{}{}
	}
}

// recordCIChecks appends recs to <state dir>/ci-checks.jsonl under ci-checks.lock, skipping
// any key this process already wrote (layer 1, memory) and any key already in the live file
// (layer 2, read under the lock — another process may have written it).
func recordCIChecks(recs []CICheckRecord) error {
	dir, err := deskDir()
	if err != nil {
		return fmt.Errorf("cannot resolve the state directory: %w", err)
	}
	live := filepath.Join(dir, ciChecksFile)

	fresh := ciFreshKeys(live, recs)
	if len(fresh) == 0 {
		return nil // every key already known: no file is touched
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create the state directory: %w", err)
	}
	unlock, err := ciLock(dir)
	if err != nil {
		return err
	}
	defer unlock()

	ciRotateIfDue(dir, ciNow())

	onDisk, err := ciReadKeys(live)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	known := make([]string, 0, len(fresh))
	for _, r := range fresh {
		k := r.ciKey()
		known = append(known, k)
		if _, ok := onDisk[k]; ok {
			continue
		}
		b, merr := json.Marshal(r)
		if merr != nil {
			return fmt.Errorf("cannot encode a record: %w", merr)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	if buf.Len() > 0 {
		f, oerr := os.OpenFile(live, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if oerr != nil {
			return fmt.Errorf("cannot open %s: %w", ciChecksFile, oerr)
		}
		_, werr := f.Write(buf.Bytes())
		cerr := f.Close()
		if werr != nil {
			return fmt.Errorf("cannot append to %s: %w", ciChecksFile, werr)
		}
		if cerr != nil {
			return fmt.Errorf("cannot close %s: %w", ciChecksFile, cerr)
		}
	}
	ciMarkSeen(live, known)
	return nil
}

// ciLock takes ci-checks.lock, waiting at most ciLockWait. It is its own lock, never the
// audit lock: a busy CI recorder must not delay an audited write, nor the reverse.
func ciLock(dir string) (unlock func(), err error) {
	lf, err := os.OpenFile(filepath.Join(dir, ciChecksLock), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("cannot open the lock file: %w", err)
	}
	deadline := time.Now().Add(ciLockWait)
	for {
		lerr := TryLockExclusive(lf)
		if lerr == nil {
			return func() { _ = UnlockFile(lf); _ = lf.Close() }, nil
		}
		if !errors.Is(lerr, ErrLockBusy) {
			_ = lf.Close()
			return nil, fmt.Errorf("cannot take the lock: %w", lerr)
		}
		if !time.Now().Before(deadline) {
			_ = lf.Close()
			return nil, fmt.Errorf("lock held longer than %s", ciLockWait)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// ciSegmentPattern matches a rotated segment name EXACTLY. It cannot match audit.jsonl
// segments, and the audit log's segmentPattern cannot match these.
var ciSegmentPattern = regexp.MustCompile(`^ci-checks\.jsonl\.\d{4}-\d{2}-\d{2}(\.\d+)?$`)

// ciRotateIfDue renames the live file to ci-checks.jsonl.<its last day> when its last append
// fell on an earlier UTC day — rotateIfNeeded's rule and naming. Best-effort: every failure
// leaves the live file where it is. The caller holds ci-checks.lock.
func ciRotateIfDue(dir string, now time.Time) {
	path := filepath.Join(dir, ciChecksFile)
	fi, err := os.Stat(path)
	if err != nil || !rotationDue(fi.ModTime(), now) {
		return
	}
	base := ciChecksFile + "." + fi.ModTime().UTC().Format("2006-01-02")
	target := filepath.Join(dir, base)
	for i := 1; ; i++ {
		if _, serr := os.Stat(target); os.IsNotExist(serr) {
			break
		}
		target = filepath.Join(dir, fmt.Sprintf("%s.%d", base, i))
	}
	_ = os.Rename(path, target)
}

// ciScan calls fn for every parseable ci-check-v1 line of path; a missing file is empty.
func ciScan(path string, fn func(CICheckRecord)) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var r CICheckRecord
		if json.Unmarshal(line, &r) != nil || r.Schema != CICheckSchema {
			continue // a torn or foreign line is skipped, never fatal
		}
		fn(r)
	}
	return sc.Err()
}

func ciReadKeys(path string) (map[string]struct{}, error) {
	keys := map[string]struct{}{}
	if err := ciScan(path, func(r CICheckRecord) { keys[r.ciKey()] = struct{}{} }); err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", filepath.Base(path), err)
	}
	return keys, nil
}

// LoadCIChecks reads every ci-checks segment, oldest first, then the live file, and returns
// the records de-duplicated on (repo, head_sha, kind, name, attempt) — first occurrence wins —
// so a cross-day or cross-process duplicate on disk never reaches an analysis. A malformed
// line is skipped. A missing state directory is an empty history.
func LoadCIChecks() ([]CICheckRecord, error) {
	dir, err := deskDir()
	if err != nil {
		return nil, Unverifiable("cannot resolve the CI-check history path (HOME missing?)", err)
	}
	ents, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, Unverifiable("cannot list the desk-tools dir", err)
	}
	var segs []string
	for _, e := range ents {
		if !e.IsDir() && ciSegmentPattern.MatchString(e.Name()) {
			segs = append(segs, e.Name())
		}
	}
	sort.Strings(segs)
	paths := make([]string, 0, len(segs)+1)
	for _, s := range segs {
		paths = append(paths, filepath.Join(dir, s))
	}
	paths = append(paths, filepath.Join(dir, ciChecksFile))

	seen := map[string]struct{}{}
	var out []CICheckRecord
	for _, p := range paths {
		serr := ciScan(p, func(r CICheckRecord) {
			k := r.ciKey()
			if _, dup := seen[k]; dup {
				return
			}
			seen[k] = struct{}{}
			out = append(out, r)
		})
		if serr != nil {
			return nil, Unverifiable("cannot read "+filepath.Base(p), serr)
		}
	}
	return out, nil
}
