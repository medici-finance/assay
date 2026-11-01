package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// pendingEvidenceSource is an optional capability of the scheduling source.
// Board-only scans and explicit verdict execution do not discover forge state.
// The production issue source supplies both capabilities at the same allowlist,
// role credential, notice and per-pass cache boundary.
type pendingEvidenceSource interface {
	PendingEvidence(root, repo string, it loopengine.Item, now time.Time, inputs deskkit.WakeInputs) (string, error)
}
type pendingChanges struct {
	prepared bool
	list     *deskkit.ChangeList
	err      error
	base     string
	reads    map[int]*pendingRead
	// These observations belong to this repository's admitted forge and this
	// selection only. A new selection drops the whole pendingChanges value.
	files      map[pendingFileKey]pendingFileRead
	receipts   map[pendingAttestationKey]pendingAttestation
	indexed    bool
	byBrief    map[string][]pendingCandidate
	unreadable []pendingCandidate
}

type pendingFileKey struct{ repo, sha, file string }
type pendingAttestationKey struct {
	repo   string
	number int
}
type pendingFileRead struct {
	content  []byte
	bindings []string
	err      error
}
type pendingAttestation struct {
	receipt deskkit.VerifierReceipt
	err     error
}
type pendingCandidate struct {
	order  int
	change deskkit.ChangeRef
}

func (c *pendingChanges) file(f deskkit.Forge, repo deskkit.ForgeRepo, file, sha string) (pendingFileRead, error) {
	if !pendingSHA(sha) {
		return pendingFileRead{}, fmt.Errorf("pending file requires an exact commit")
	}
	if c.files == nil {
		c.files = map[pendingFileKey]pendingFileRead{}
	}
	key := pendingFileKey{repo.Slug(), sha, file}
	read, ok := c.files[key]
	if !ok {
		read.content, read.err = pendingFile(f, repo, file, sha)
		read.content = bytes.Clone(read.content)
		if read.err == nil {
			read.bindings = deskkit.VerifierEvidenceBindings(read.content)
		}
		c.files[key] = read
	}
	// A caller cannot mutate an observation another item will evaluate.
	read.content, read.bindings = bytes.Clone(read.content), slices.Clone(read.bindings)
	return read, read.err
}

func (c *pendingChanges) attestation(f deskkit.Forge, repo deskkit.ForgeRepo, number int, performance verifyPerformance) (deskkit.VerifierReceipt, error) {
	if c.receipts == nil {
		c.receipts = map[pendingAttestationKey]pendingAttestation{}
	}
	key := pendingAttestationKey{repo.Slug(), number}
	read, ok := c.receipts[key]
	if !ok {
		stop := performance.phase("attestation")
		read.receipt, read.err = deskkit.ReadVerifierAttestation(f, repo, number)
		stop()
		c.receipts[key] = read
	}
	return read.receipt, read.err
}

// Build from complete reads ALREADY performed, after the first exhaustive walk.
// A successful early hold keeps the old lazy read behavior. Unreadable verifier
// changes remain candidates for every brief, in the original list order.
func (c *pendingChanges) indexChanges(verifier string) {
	if c.indexed {
		return
	}
	c.byBrief = map[string][]pendingCandidate{}
	c.unreadable = nil
	for i, change := range c.list.Changes {
		if !deskkit.SameActor(change.Author.Login, verifier) {
			continue
		}
		read := c.reads[change.Number]
		if read == nil {
			return // not an exhaustive walk yet
		}
		candidate := pendingCandidate{i, change}
		if read.err != nil {
			c.unreadable = append(c.unreadable, candidate)
			continue
		}
		seen := map[string]bool{}
		for _, file := range read.files {
			if file.Status != "removed" && !seen[file.Filename] {
				seen[file.Filename] = true
				c.byBrief[file.Filename] = append(c.byBrief[file.Filename], candidate)
			}
		}
	}
	c.indexed = true
}

func (c *pendingChanges) changesFor(brief string) []deskkit.ChangeRef {
	if !c.indexed {
		return c.list.Changes
	}
	matching, unknown := c.byBrief[brief], c.unreadable
	var changes []deskkit.ChangeRef
	for len(matching)+len(unknown) > 0 {
		if len(unknown) == 0 || len(matching) > 0 && matching[0].order < unknown[0].order {
			changes = append(changes, matching[0].change)
			matching = matching[1:]
		} else {
			changes = append(changes, unknown[0].change)
			unknown = unknown[1:]
		}
	}
	return changes
}

// pendingRead is one verifier-authored change's pull-request and complete file read,
// cached for the pass INCLUDING its failure, so an unreadable change costs one read per
// pass rather than one per queued item.
type pendingRead struct {
	pr    *deskkit.PullRequest
	files []deskkit.ChangedFile
	err   error
}

func (s *forgeIssueSource) PendingEvidence(root, slug string, it loopengine.Item, now time.Time, inputs deskkit.WakeInputs) (string, error) {
	defer s.performance.phase("pending-evidence")()
	if slug == "" {
		return "", fmt.Errorf("tracking repository unavailable; pending Evidence population is unknown")
	}
	owner, name, ok := strings.Cut(slug, "/")
	if !ok {
		return "", fmt.Errorf("unknown tracking repository")
	}
	repo := deskkit.ForgeRepo{Owner: owner, Name: name}
	f, err := s.forge(repo)
	if err != nil {
		return "", err
	}
	cache := s.pendingChanges(f, repo)
	if cache.err != nil {
		return "", cache.err
	}
	if cache.list == nil {
		return "", fmt.Errorf("empty open-change read")
	}
	verifier, bound := deskkit.RoleAppLogin("verifier")
	if !bound {
		return "", fmt.Errorf("verifier role login is unbound; pending Evidence authorship is unknown")
	}
	if !cache.prepared {
		cache.prepared = true
		if batch := deskkit.PendingChangeBatchReader(f); batch != nil {
			preparePendingChangeReads(cache, repo, verifier, batch)
		}
		if batch := deskkit.VerifierAttestationBatchReader(f); batch != nil && len(s.pendingTargets[slug]) > 0 {
			s.preparePendingRepository(f, repo, verifier, s.pendingTargets[slug], batch)
		}
	}
	var uncertain error
	for _, change := range cache.changesFor(it.BriefPath) {
		// Only the pinned verifier App can author admitted Evidence: the Evidence writer
		// requires that identity. A change by anyone else (a worker, the desk, a human, a
		// fork or deleted account) carries no hold and no uncertainty, and is filtered on
		// the list read's author BEFORE any per-change read, so no such change can withhold
		// a brief from dispatch or make another item's read fail.
		if !deskkit.SameActor(change.Author.Login, verifier) {
			continue
		}
		ref := fmt.Sprintf("#%d", change.Number)
		read := cache.change(f, repo, change.Number)
		if read.err != nil {
			// A verifier-authored change whose files cannot be listed may touch any brief.
			uncertain = fmt.Errorf("%s changed-file read: %w", ref, read.err)
			continue
		}
		files, pr := read.files, read.pr
		touches := false
		for _, file := range files {
			if file.Filename == it.BriefPath && file.Status != "removed" {
				touches = true
			}
		}
		if !touches {
			continue
		}
		// The list read named the verifier; the change read must confirm that identity by
		// login AND roster-pinned id, and the author must be independent of the implementer.
		if err := pendingChangeProvenance(pr, change, cache.base, slug, verifier, it.Implementer); err != nil {
			uncertain = fmt.Errorf("%s %w", ref, err)
			continue
		}
		file, e := cache.file(f, repo, it.BriefPath, pr.HeadSHA)
		if e != nil {
			uncertain = e
			continue
		}
		content, bindings := file.content, file.bindings
		if len(bindings) == 0 {
			uncertain = fmt.Errorf("%s has no admitted Evidence binding", ref)
			continue
		}
		for _, line := range bindings {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			recordRef, e := deskkit.ParseBlockerRef(fields[1], "", "")
			if e != nil || recordRef.Repo() != repo || recordRef.Kind == "change" {
				uncertain = fmt.Errorf("%s has an invalid attestation reference", ref)
				continue
			}
			receipt, e := cache.attestation(f, repo, recordRef.Number, s.performance)
			if e != nil {
				uncertain = e
				continue
			}
			if receipt.Binding.Brief != it.BriefPath {
				continue
			}
			if receipt.EvidenceBinding() != line {
				uncertain = fmt.Errorf("%s Evidence differs from its exact attested run", ref)
				continue
			}
			wait, e := pendingRun(f, repo, root, it, receipt, content, files, pr.HeadSHA, now, inputs, s, cache)
			if e != nil {
				uncertain = e
				continue
			}
			if !wait {
				continue
			}
			// The changed-file listing cannot be pinned by the Forge interface. Reconcile
			// its head again after immutable file reads; movement is never a stale hold.
			after, e := f.GetPullRequest(repo, change.Number)
			if e != nil {
				return "", e
			}
			if after == nil || after.HeadSHA != pr.HeadSHA || after.BaseRef != cache.base || after.State != "open" || after.Merged {
				return "", fmt.Errorf("%s moved during pending Evidence read", ref)
			}
			return ref, nil
		}
	}
	cache.indexChanges(verifier)
	if uncertain != nil {
		return "", uncertain
	}
	if cache.list.Incomplete {
		return "", fmt.Errorf("open-change population incomplete; absence of pending Evidence is unknown")
	}
	return "", nil
}

func pendingSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func pendingFile(f deskkit.Forge, repo deskkit.ForgeRepo, file, sha string) ([]byte, error) {
	got, err := f.ReadFile(repo, deskkit.ReadFileInput{File: file, Ref: sha})
	if err != nil {
		return nil, err
	}
	if got == nil || !got.Exists || got.Kind != deskkit.FileKindRegular {
		return nil, fmt.Errorf("Evidence file %s at %s is absent or not a proven regular file", file, sha)
	}
	return got.Content, nil
}

func pendingRun(f deskkit.Forge, repo deskkit.ForgeRepo, root string, it loopengine.Item, receipt deskkit.VerifierReceipt, content []byte, files []deskkit.ChangedFile, head string, now time.Time, inputs deskkit.WakeInputs, issues deskkit.IssueStateSource, observations *pendingChanges) (bool, error) {
	b := receipt.Binding
	if err := receipt.CheckEvidenceContent(b.Brief, content, nil); err != nil {
		return false, err
	}
	evidenceBody := extractEvidence(string(content))
	for _, line := range deskkit.VerifierEvidenceBindings(content) {
		evidenceBody = strings.ReplaceAll(evidenceBody, line, "")
	}
	if !evidenceHasContent(evidenceBody) {
		return false, fmt.Errorf("pending Evidence contains only a binding, no observed results")
	}
	current, err := os.ReadFile(filepath.Join(root, it.BriefPath))
	if err != nil {
		return false, err
	}
	// A changed plan wakes ordinary verification, never a pending-Evidence hold.
	if receipt.CheckEvidenceContent(b.Brief, current, nil) != nil {
		return false, nil
	}
	tracking := b.Source
	if b.Version == 2 {
		tracking = b.TrackingSource
	}
	source := pendingTree{root: root, sha: tracking}
	original, err := source.ReadFile(b.Brief)
	if err != nil {
		return false, err
	}
	if err := receipt.CheckObservedBrief(original); err != nil {
		return false, err
	}
	if b.Version == 2 {
		if b.Repo != payloadValue(it, "repo") || b.SourceRepo != payloadValue(it, "source_repo") || payloadValue(it, "tracking_sha") == "" || payloadValue(it, "source_root") == "" {
			return false, fmt.Errorf("cross-repository Evidence has no exact queued association")
		}
	} else if payloadValue(it, "source_repo") != "" {
		return false, fmt.Errorf("cross-repository queue item has only a same-repository attestation")
	}
	currentSHA, err := pendingGit(root, "rev-parse", "HEAD")
	if err != nil {
		return false, err
	}
	exact := strings.TrimSpace(string(currentSHA)) == tracking
	if b.Version == 2 {
		exact = exact && it.TargetSHA == b.Source
	}
	if it.TargetSHA != "" && b.Version == 1 {
		exact = exact && it.TargetSHA == b.Source
	}
	exactSource := func() (bool, error) {
		if !exact {
			return false, nil
		}
		if err := deskkit.CheckVerifierSourceTree(root, tracking); err != nil {
			return false, err
		}
		if b.Version == 2 {
			if err := deskkit.CheckVerifierSourceTree(payloadValue(it, "source_root"), b.Source); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	// With no failed/blocked wake receipt, whole-source identity is the only
	// freshness fact. No dependency heuristic makes a pass survive source change.
	var records []deskkit.OutcomeRecord
	stream := path.Base(path.Dir(b.Brief))
	num, _, _ := strings.Cut(strings.TrimPrefix(path.Base(b.Brief), "brief-"), "-")
	key := stream + "/" + num
	if _, _, err := deskkit.SplitBriefKey(key); err != nil {
		return false, err
	}
	for _, file := range files {
		if path.Dir(file.Filename) != path.Join(deskkit.OutcomeRecordsDir, stream) || !strings.HasPrefix(path.Base(file.Filename), num+"-") || file.Status == "removed" {
			continue
		}
		read, e := observations.file(f, repo, file.Filename, head)
		if e != nil {
			return false, e
		}
		raw := read.content
		if e := deskkit.CheckRecordKeys(raw); e != nil {
			return false, e
		}
		rec, e := deskkit.ParseRecord(raw)
		if e != nil {
			return false, e
		}
		expected, e := deskkit.RecordName(raw)
		if e != nil || expected != file.Filename || rec.Brief != key {
			return false, fmt.Errorf("pending record is not bound to the queued brief/path")
		}
		rec.Name = path.Base(file.Filename)
		records = append(records, rec)
	}
	latest, future := deskkit.LatestPerBriefAt(records, now)
	if future[key] {
		return false, fmt.Errorf("future pending Evidence receipt")
	}
	record, has := latest[key]
	if !has {
		if payloadValue(it, "wake_state") == "fire" {
			return false, nil
		}
		return exactSource()
	}
	if landedTS, err := time.Parse(time.RFC3339, payloadValue(it, "sidecar_ts")); err == nil {
		pendingTS, _ := time.Parse(time.RFC3339, record.TS)
		if !pendingTS.After(landedTS) {
			return false, nil
		}
	}
	rec, ok := deskkit.ParseWakeReceipt(record.Raw)
	if !ok {
		return false, fmt.Errorf("unreadable pending receipt")
	}
	verifier, _ := deskkit.RoleAppLogin("verifier")
	if !deskkit.SameActor(rec.Verifier, verifier) || rec.Repo != repo.Slug() || !pendingSHA(rec.SHA) {
		return false, fmt.Errorf("pending outcome has no exact verifier/repository/Evidence revision")
	}
	evidence, e := observations.file(f, repo, b.Brief, rec.SHA)
	if e != nil {
		return false, e
	}
	if !bytes.Equal(evidence.content, content) {
		return false, fmt.Errorf("pending outcome revision names different Evidence")
	}
	if !rec.IsFailedOrBlocked() {
		return exactSource()
	}
	if !rec.Complete() {
		return false, fmt.Errorf("pending non-pass receipt is incomplete")
	}
	if err := deskkit.CheckSuppliedReceipt(rec); err != nil {
		return false, err
	}
	// Re-derive the observation using the existing writer, including the original
	// source commit, declared directory membership, landed brief and source-repo
	// key. Caller-supplied or narrowed input maps never establish a hold.
	if rec.WakePredicate == deskkit.WakeRelevantInputChanged {
		observed := rec
		observed.Schema = ""
		raw, _ := json.Marshal(observed)
		in := deskkit.OutcomeReceiptInput{BriefPath: b.Brief, LandedBrief: content, ToolVersion: rec.ToolVersion, Repo: b.Repo}
		if b.Version == 2 {
			in.SourceRepo, in.Source = b.SourceRepo, b.Source
			if in.SiblingSources, e = deskkit.VerifierSiblingSources(b); e != nil {
				return false, e
			}
		}
		derived, e := deskkit.BuildOutcomeRecord(raw, in, source)
		if e != nil {
			return false, e
		}
		expected, _ := deskkit.ParseWakeReceipt(derived)
		if !maps.Equal(rec.Inputs, expected.Inputs) {
			return false, fmt.Errorf("pending receipt inputs do not describe its attested observation")
		}
		// Reuse the writer's declaration expansion at current main too: a new
		// file in a declared directory (or a formerly absent declared path) must
		// wake rather than disappear from the old receipt's set of file keys.
		currentRecord, err := deskkit.BuildOutcomeRecord(raw, in, pendingTree{root: root, sha: strings.TrimSpace(string(currentSHA))})
		if err != nil {
			return false, err
		}
		currentScope, _ := deskkit.ParseWakeReceipt(currentRecord)
		if len(currentScope.Inputs) != len(expected.Inputs) {
			return false, nil
		}
		for key := range expected.Inputs {
			if _, ok := currentScope.Inputs[key]; !ok {
				return false, nil
			}
		}
	}
	if inputs == nil {
		inputs = deskkit.NewRootRevisionReader(root, deskkit.ReleaseTagOrDev())
	}
	// A pending brief necessarily differs in Evidence from main. Both plans were
	// checked against the attestation above; substitute ONLY that landed brief's
	// exact writer hash, leaving every other existing wake input untouched.
	reader := pendingInputs{WakeInputs: inputs, brief: "file:" + b.Brief, digest: deskkit.Sha256Hex(content)}
	parsedRows := parseVerifyRowsIn(extractVerify(string(current)))
	var signals briefFrontmatter
	deriveContentSignalsRows(&signals, parsedRows)
	p := map[string]string{"deferred_rows": signals.DeferredRows}
	var rows []deskkit.VerifyRowID
	for _, row := range parsedRows {
		rows = append(rows, row.ID)
	}
	deriveWakePayload(p, rec, rows, reader, issues, repo.Slug(), now)
	switch p["wake_state"] {
	case "hold":
		return true, nil
	case "could-not-check":
		return false, fmt.Errorf("%s", p["wake_reason"])
	}
	return false, nil
}

type pendingInputs struct {
	deskkit.WakeInputs
	brief, digest string
}

func (p pendingInputs) Revision(key string) (string, bool) {
	if key == p.brief {
		return p.digest, true
	}
	return p.WakeInputs.Revision(key)
}

type pendingTree struct{ root, sha string }

func pendingGit(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"--no-replace-objects", "--literal-pathspecs", "-C", root}, args...)...)
	cmd.Env = deskkit.VerifierReadEnv()
	return cmd.Output()
}
func (p pendingTree) ReadFile(rel string) ([]byte, error) {
	if !pendingSHA(p.sha) {
		return nil, fmt.Errorf("invalid source revision")
	}
	return pendingGit(p.root, "cat-file", "blob", p.sha+":"+rel)
}
func (p pendingTree) PathKind(rel string) (string, error) {
	out, e := pendingGit(p.root, "ls-tree", p.sha, "--", rel)
	if e != nil {
		return "", e
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", nil
	}
	if len(fields) < 3 {
		return "", fmt.Errorf("unreadable source path kind")
	}
	return fields[1], nil
}
func (p pendingTree) ListFiles(rel string) ([]string, error) {
	out, e := pendingGit(p.root, "ls-tree", "-r", "--name-only", "-z", p.sha, "--", strings.TrimSuffix(rel, "/")+"/")
	if e != nil {
		return nil, e
	}
	text := strings.TrimSuffix(string(out), "\x00")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\x00"), nil
}

type offlineScheduling struct{}

func (offlineScheduling) IssueState(deskkit.BlockerRef) (deskkit.BlockerState, string) {
	return deskkit.BlockerCouldNotCheck, "forge reads disabled"
}
func (offlineScheduling) PendingEvidence(root, repo string, it loopengine.Item, now time.Time, inputs deskkit.WakeInputs) (string, error) {
	return "", fmt.Errorf("forge reads disabled; pending Evidence could not be checked")
}

// pendingChanges and change are shared by preparation and lazy evaluation. Neither
// path treats failed discovery or an incomplete file list as absence.
func (s *forgeIssueSource) pendingChanges(f deskkit.Forge, repo deskkit.ForgeRepo) *pendingChanges {
	if s.pending == nil {
		s.pending = map[string]*pendingChanges{}
	}
	cache := s.pending[repo.Slug()]
	if cache == nil {
		stop := s.performance.phase("pending-discovery")
		cache = &pendingChanges{reads: map[int]*pendingRead{}}
		cache.base, cache.err = f.RepoDefaultBranch(repo)
		if cache.err == nil && cache.base == "" {
			cache.err = fmt.Errorf("default branch is unreadable")
		}
		if cache.err == nil {
			cache.list, cache.err = f.ListChanges(repo, deskkit.ChangeStates{Open: true})
		}
		s.pending[repo.Slug()] = cache
		stop()
	}
	return cache
}
func (c *pendingChanges) change(f deskkit.Forge, repo deskkit.ForgeRepo, number int) *pendingRead {
	read := c.reads[number]
	if read == nil {
		read = &pendingRead{}
		read.pr, read.files, read.err = deskkit.CompleteChangedFilesOf(f, repo, number)
		c.reads[number] = read
	}
	return read
}
func pendingChangeProvenance(pr *deskkit.PullRequest, change deskkit.ChangeRef, base, slug, verifier, implementer string) error {
	if pr == nil || pr.Author.ID == 0 || !deskkit.SameActor(pr.Author.Login, verifier) || !deskkit.TrustedAuthorID(pr.Author.Login, pr.Author.ID) || (implementer != "" && loopengine.ImplementerIs(implementer, pr.Author.Login, deskkit.SameActor)) {
		return fmt.Errorf("has no independent trusted verifier author")
	}
	if pr.Number != change.Number || pr.State != "open" || pr.Merged || pr.BaseRef != base || pr.CrossRepo != deskkit.CrossRepoSame || change.CrossRepo != deskkit.CrossRepoSame || change.HeadRepo != slug || change.HeadSHA != pr.HeadSHA || !pendingSHA(pr.HeadSHA) {
		return fmt.Errorf("has incomplete open-change/source provenance")
	}
	return nil
}

// preparePendingEvidence changes the join order, not admission. Discover the
// distinct references in completely listed, relevant verifier PR files once,
// then bulk-read their observations. Evaluation below still checks each exact
// binding, outcome, wake inputs and current PR head/state in its original order.
// Other backends/decorators retain their existing lazy per-target path.
func (s *forgeIssueSource) preparePendingEvidence(items []loopengine.Item, fallbackRepo string) {
	byRepo := map[string]map[string][]loopengine.Item{}
	for _, it := range items {
		if payloadValue(it, "could_not_check") != "" {
			continue
		}
		slug := payloadValue(it, "repo")
		if slug == "" {
			slug = fallbackRepo
		}
		if byRepo[slug] == nil {
			byRepo[slug] = map[string][]loopengine.Item{}
		}
		byRepo[slug][it.BriefPath] = append(byRepo[slug][it.BriefPath], it)
	}
	s.pendingTargets = byRepo
}
func (s *forgeIssueSource) preparePendingRepository(f deskkit.Forge, repo deskkit.ForgeRepo, verifier string, briefs map[string][]loopengine.Item, batch func(deskkit.ForgeRepo, []int) map[int]deskkit.VerifierAttestationResult) {
	cache := s.pendingChanges(f, repo)
	if cache.err != nil || cache.list == nil {
		return
	}
	seen := map[int]bool{}
	var numbers []int
	for _, change := range cache.list.Changes {
		if !deskkit.SameActor(change.Author.Login, verifier) {
			continue
		}
		read := cache.change(f, repo, change.Number)
		if read.err != nil {
			continue
		}
		for _, file := range read.files {
			if file.Status == "removed" {
				continue
			}
			relevant := false
			for _, it := range briefs[file.Filename] {
				if pendingChangeProvenance(read.pr, change, cache.base, repo.Slug(), verifier, it.Implementer) == nil {
					relevant = true
					break
				}
			}
			if !relevant {
				continue
			}
			observed, err := cache.file(f, repo, file.Filename, read.pr.HeadSHA)
			if err != nil {
				continue
			}
			for _, line := range observed.bindings {
				fields := strings.Fields(line)
				if len(fields) < 2 {
					continue
				}
				ref, err := deskkit.ParseBlockerRef(fields[1], "", "")
				if err != nil || ref.Repo() != repo || ref.Kind == "change" || seen[ref.Number] {
					continue
				}
				seen[ref.Number] = true
				numbers = append(numbers, ref.Number)
			}
		}
	}
	if len(numbers) == 0 {
		return
	}
	stop := s.performance.phase("attestation")
	results := batch(repo, numbers)
	stop()
	if cache.receipts == nil {
		cache.receipts = map[pendingAttestationKey]pendingAttestation{}
	}
	for _, number := range numbers {
		result, ok := results[number]
		if !ok {
			result.Err = fmt.Errorf("attestation batch returned no observation for target")
		}
		cache.receipts[pendingAttestationKey{repo.Slug(), number}] = pendingAttestation{receipt: result.Receipt, err: result.Err}
	}
}

// Seed the existing per-pass cache, including failures. Evaluation still proves
// source/author/brief bindings and freshly reconciles every proposed hold.
func preparePendingChangeReads(cache *pendingChanges, repo deskkit.ForgeRepo, verifier string, batch func(deskkit.ForgeRepo, []int) map[int]deskkit.PendingChangeObservation) {
	var numbers []int
	for _, change := range cache.list.Changes {
		if deskkit.SameActor(change.Author.Login, verifier) && cache.reads[change.Number] == nil {
			numbers = append(numbers, change.Number)
		}
	}
	if len(numbers) == 0 {
		return
	}
	results := batch(repo, numbers)
	for _, number := range numbers {
		result, ok := results[number]
		if !ok {
			result.Err = fmt.Errorf("pending-change batch returned no observation")
		}
		cache.reads[number] = &pendingRead{pr: result.PR, files: result.Files, err: result.Err}
	}
}
