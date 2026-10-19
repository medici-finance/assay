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
	list  *deskkit.ChangeList
	err   error
	base  string
	reads map[int]*pendingRead
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
	if s.pending == nil {
		s.pending = map[string]*pendingChanges{}
	}
	cache := s.pending[slug]
	if cache == nil {
		cache = &pendingChanges{reads: map[int]*pendingRead{}}
		cache.base, cache.err = f.RepoDefaultBranch(repo)
		if cache.err == nil && cache.base == "" {
			cache.err = fmt.Errorf("default branch is unreadable")
		}
		if cache.err == nil {
			cache.list, cache.err = f.ListChanges(repo, deskkit.ChangeStates{Open: true})
		}
		s.pending[slug] = cache
	}
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
	var uncertain error
	for _, change := range cache.list.Changes {
		// Only the pinned verifier App can author admitted Evidence: the Evidence writer
		// requires that identity. A change by anyone else (a worker, the desk, a human, a
		// fork or deleted account) carries no hold and no uncertainty, and is filtered on
		// the list read's author BEFORE any per-change read, so no such change can withhold
		// a brief from dispatch or make another item's read fail.
		if !deskkit.SameActor(change.Author.Login, verifier) {
			continue
		}
		ref := fmt.Sprintf("#%d", change.Number)
		read := cache.reads[change.Number]
		if read == nil {
			read = &pendingRead{}
			read.pr, read.files, read.err = deskkit.CompleteChangedFilesOf(f, repo, change.Number)
			cache.reads[change.Number] = read
		}
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
		if pr.Author.ID == 0 || !deskkit.SameActor(pr.Author.Login, verifier) || !deskkit.TrustedAuthorID(pr.Author.Login, pr.Author.ID) || (it.Implementer != "" && deskkit.SameActor(it.Implementer, pr.Author.Login)) {
			uncertain = fmt.Errorf("%s has no independent trusted verifier author", ref)
			continue
		}
		if pr.Number != change.Number || pr.State != "open" || pr.Merged || pr.BaseRef != cache.base || pr.CrossRepo != deskkit.CrossRepoSame || change.CrossRepo != deskkit.CrossRepoSame || change.HeadRepo != slug || change.HeadSHA != pr.HeadSHA || !pendingSHA(pr.HeadSHA) {
			uncertain = fmt.Errorf("%s has incomplete open-change/source provenance", ref)
			continue
		}
		content, e := pendingFile(f, repo, it.BriefPath, pr.HeadSHA)
		if e != nil {
			uncertain = e
			continue
		}
		bindings := deskkit.VerifierEvidenceBindings(content)
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
			receipt, e := deskkit.ReadVerifierAttestation(f, repo, recordRef.Number)
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
			wait, e := pendingRun(f, repo, root, it, receipt, content, files, pr.HeadSHA, now, inputs, s)
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

func pendingRun(f deskkit.Forge, repo deskkit.ForgeRepo, root string, it loopengine.Item, receipt deskkit.VerifierReceipt, content []byte, files []deskkit.ChangedFile, head string, now time.Time, inputs deskkit.WakeInputs, issues deskkit.IssueStateSource) (bool, error) {
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
		raw, e := pendingFile(f, repo, file.Filename, head)
		if e != nil {
			return false, e
		}
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
	evidence, e := pendingFile(f, repo, b.Brief, rec.SHA)
	if e != nil {
		return false, e
	}
	if !bytes.Equal(evidence, content) {
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
	p := map[string]string{}
	var rows []int
	for _, row := range parseVerifyRows(string(current)) {
		rows = append(rows, row.Num)
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
