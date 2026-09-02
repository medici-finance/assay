package attribution

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// provenance adapter (Verify #2)
// ---------------------------------------------------------------------------

func TestCommitIssueAdapter_ParsesRefsAndStates(t *testing.T) {
	a := CommitIssueAdapter{}

	// A PR-carrying inducing change whose message references two issues resolves
	// both the PR rung and the issue rung; refs are sorted+de-duplicated.
	got, err := a.Resolve(Inducing{
		Commit:  "abc123",
		PR:      "42",
		Message: "Refs #9\n\nFixes #3 and also fixes #3 again, closes #12",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	pr, ok := got.Link(LinkKindPR)
	if !ok || pr.State != LinkResolved {
		t.Fatalf("PR rung: want resolved, got %+v (ok=%v)", pr, ok)
	}
	iss, ok := got.Link(LinkKindIssue)
	if !ok || iss.State != LinkResolved {
		t.Fatalf("issue rung: want resolved, got %+v (ok=%v)", iss, ok)
	}
	if iss.Ref != "#3, #9, #12" {
		t.Fatalf("issue refs: want sorted+deduped %q, got %q", "#3, #9, #12", iss.Ref)
	}
	if got.Broken() {
		t.Fatalf("chain with a resolved issue rung must not be Broken()")
	}

	// A bare-git inducing commit with no PR and no issue reference: PR absent,
	// issue absent, chain BROKEN (no requirement rung to attribute against).
	bare, err := a.Resolve(Inducing{Commit: "def456", Message: "tidy up formatting"})
	if err != nil {
		t.Fatalf("Resolve bare: %v", err)
	}
	if pr, _ := bare.Link(LinkKindPR); pr.State != LinkAbsent {
		t.Fatalf("bare PR rung: want absent, got %v", pr.State)
	}
	if iss, _ := bare.Link(LinkKindIssue); iss.State != LinkAbsent {
		t.Fatalf("bare issue rung: want absent, got %v", iss.State)
	}
	if !bare.Broken() {
		t.Fatalf("chain with no requirement rung must be Broken()")
	}
}

func TestDefaultAdapterRegistered(t *testing.T) {
	a, ok := DefaultAdapter()
	if !ok {
		t.Fatalf("no default adapter registered")
	}
	if _, isCI := a.(CommitIssueAdapter); !isCI {
		t.Fatalf("default adapter is not the commit-issue reference adapter: %T", a)
	}
	if _, ok := Adapter(CommitIssueAdapterName); !ok {
		t.Fatalf("commit-issue adapter not registered under its name")
	}
}

// ---------------------------------------------------------------------------
// dossier determinism (Verify #3)
// ---------------------------------------------------------------------------

// tracedInputBriefGap is the shared fixture: a TRACED defect whose inducing change
// faithfully implements its brief (touches only fileA, which the brief covers) while
// the DEFECT surface is fileB, which the brief does NOT cover. Slice inputs are
// given in deliberately UN-sorted order so a non-canonical assembly would differ.
func tracedInputBriefGap() DossierInput {
	return DossierInput{
		Trace: Trace{
			FixCommit:       "fix000",
			FixPR:           7,
			InducingCommits: []string{"indZ", "indA"},
			InducingPRs:     []string{"5"},
			EvidenceTier:    2,
			TraceState:      traceStateTraced,
		},
		Chain: Chain{
			Inducing: Inducing{Commit: "indA", PR: "5", Message: "Fixes #11"},
			Links: []ChainLink{
				{Kind: LinkKindPR, Ref: "5", State: LinkResolved},
				{Kind: LinkKindIssue, Ref: "#11", State: LinkResolved},
			},
		},
		Brief: BriefSnapshot{
			Path:         "docs/streams/x/brief-01.md",
			AtMergeSHA:   "base999",
			Content:      "plan covering fileA",
			Coverage:     []string{"fileA.go"},
			Present:      true,
			ReflectsSpec: SignalTrue,
		},
		InducingDiff:  InducingDiff{Files: []string{"fileA.go"}, Surface: []string{"fileA.go"}, Patch: "@@ fileA @@"},
		Reviews:       []ReviewVerdict{{Lane: "style", Approved: true}, {Lane: "correctness", Approved: true}},
		Rulings:       []Ruling{{Ref: "#20", Date: "2026-01-02"}},
		DefectSurface: []string{"fileB.go"},
	}
}

func TestDossierDeterministic(t *testing.T) {
	in := tracedInputBriefGap()

	d1 := AssembleDossier(in)

	// Re-assemble from the SAME logical input but with every slice shuffled, to
	// prove the canonical ordering — not input order — fixes the content address.
	in2 := tracedInputBriefGap()
	in2.Trace.InducingCommits = []string{"indA", "indZ"} // reversed
	in2.Reviews = []ReviewVerdict{{Lane: "correctness", Approved: true}, {Lane: "style", Approved: true}}
	d2 := AssembleDossier(in2)

	if d1.Hash != d2.Hash {
		t.Fatalf("dossier hash not order-invariant: %s != %s", d1.Hash, d2.Hash)
	}
	b1, _ := json.Marshal(d1)
	b2, _ := json.Marshal(d2)
	if string(b1) != string(b2) {
		t.Fatalf("dossier not byte-identical across equivalent inputs:\n%s\n%s", b1, b2)
	}
	if len(d1.Hash) != 64 {
		t.Fatalf("content hash is not 64 hex chars: %q", d1.Hash)
	}

	// Cross-run stability: pin the golden content address. A change here is a
	// deliberate schema change, not a flake.
	const golden = goldenBriefGapHash
	if d1.Hash != golden {
		t.Fatalf("dossier content hash drifted from golden:\n  got  %s\n  want %s", d1.Hash, golden)
	}
}

// ---------------------------------------------------------------------------
// stage classifier — plan-gap dereferencing (Verify #4)
// ---------------------------------------------------------------------------

func TestStagePlantedBriefGap(t *testing.T) {
	d := AssembleDossier(tracedInputBriefGap())
	t.Logf("dossier: defectSurface=%v briefCoverage=%v changeSurface=%v traced=%v broken=%v",
		d.DefectSurface, d.Brief.Coverage, d.InducingDiff.Surface, d.Trace.Traceable(), d.Chain.Broken())

	call := Classify(d)
	if call.Stage != StageBrief {
		t.Fatalf("planted plan-gap defect: want stage %q (the brief did not cover the defect surface), got %q — rationale: %s",
			StageBrief, call.Stage, call.Rationale)
	}
	if call.Stage == StageImplementation {
		t.Fatalf("plan-gap must NOT be mis-attributed to implementation")
	}
	if call.Stage == StageSpec {
		t.Fatalf("plan-gap must NOT be mis-attributed to spec")
	}
	if call.DossierHash != d.Hash {
		t.Fatalf("stage call must record the dossier hash it decided from: got %q want %q", call.DossierHash, d.Hash)
	}
}

// A companion so the plan-VIOLATION and spec branches are pinned too — the
// classifier must distinguish all three, not just return brief.
func TestStageDistinguishesAllBranches(t *testing.T) {
	base := tracedInputBriefGap

	// implementation: brief covers the defect surface, but the change touched
	// surface OUTSIDE the brief (violated the plan).
	impl := base()
	impl.DefectSurface = []string{"fileA.go"}                                                    // covered
	impl.InducingDiff = InducingDiff{Files: []string{"fileZ.go"}, Surface: []string{"fileZ.go"}} // outside plan
	if got := Classify(AssembleDossier(impl)).Stage; got != StageImplementation {
		t.Fatalf("plan-violation: want %q, got %q", StageImplementation, got)
	}

	// spec: change faithful to brief, brief covers the surface, brief reflects spec.
	spec := base()
	spec.DefectSurface = []string{"fileA.go"} // covered; change stays on fileA; reflectsSpec=true
	if got := Classify(AssembleDossier(spec)).Stage; got != StageSpec {
		t.Fatalf("requirement-fault: want %q, got %q", StageSpec, got)
	}

	// brief (derivation): same as spec but the brief does NOT reflect the spec.
	deriv := base()
	deriv.DefectSurface = []string{"fileA.go"}
	deriv.Brief.ReflectsSpec = SignalFalse
	if got := Classify(AssembleDossier(deriv)).Stage; got != StageBrief {
		t.Fatalf("plan-derivation-fault: want %q, got %q", StageBrief, got)
	}
}

// ---------------------------------------------------------------------------
// untraceable is never zeroed into a stage (Verify #5)
// ---------------------------------------------------------------------------

func TestStageUntraceableNotZeroed(t *testing.T) {
	in := tracedInputBriefGap()
	// A traced inducing commit whose provenance chain is BROKEN: the reference
	// adapter finds no PR and no issue reference, so no requirement rung resolves.
	broken, err := CommitIssueAdapter{}.Resolve(Inducing{Commit: "indA", Message: "no linkage here"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	in.Chain = broken
	d := AssembleDossier(in)

	if !d.Chain.Broken() {
		t.Fatalf("fixture chain must be broken")
	}
	call := Classify(d)
	if call.Stage != StageUntraceable {
		t.Fatalf("broken chain: want %q, got %q — rationale: %s", StageUntraceable, call.Stage, call.Rationale)
	}
	if call.Stage == StageImplementation {
		t.Fatalf("untraceable must NEVER be silently placed in implementation")
	}

	// And it is COUNTED in the untraceable bucket, not elsewhere.
	dir := t.TempDir()
	l := NewLedger(dir)
	if _, err := l.Write(entryFrom("q", "w1", call)); err != nil {
		t.Fatalf("write: %v", err)
	}
	entries, err := l.ReadAll()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	r := RollupOf(entries)
	if r.Overall.Untraceable != 1 {
		t.Fatalf("untraceable bucket: want 1, got %+v", r.Overall)
	}
	if r.Overall.Implementation != 0 {
		t.Fatalf("untraceable must not leak into implementation bucket: %+v", r.Overall)
	}
}

// ---------------------------------------------------------------------------
// review-escape overlay (Verify #6)
// ---------------------------------------------------------------------------

func TestReviewEscapeOverlay(t *testing.T) {
	in := tracedInputBriefGap()
	in.Reviews = []ReviewVerdict{
		{Lane: "security", Verdict: "changes-requested", Approved: false},
		{Lane: "style", Verdict: "approved", Approved: true},
		{Lane: "correctness", Verdict: "approved", Approved: true},
		{Lane: "correctness", Verdict: "approved", Approved: true}, // dup lane
	}
	call := Classify(AssembleDossier(in))

	want := []string{"correctness", "style"} // sorted, deduped, approving-only
	if !reflect.DeepEqual(call.ReviewEscape.Lanes, want) {
		t.Fatalf("review-escape overlay: want exactly %v (the lanes that APPROVED the inducing PR at head), got %v",
			want, call.ReviewEscape.Lanes)
	}
}

// ---------------------------------------------------------------------------
// ledger append-only + tombstone amendment (Verify #7)
// ---------------------------------------------------------------------------

func TestLedgerAppendOnlyTombstone(t *testing.T) {
	dir := t.TempDir()
	l := NewLedger(dir)

	// Original attribution: brief.
	orig, err := l.Write(LedgerEntry{
		DefectID: "pr-7", Stream: "quality", Window: "2026-Q1",
		Stage: StageBrief, DossierHash: "hash-orig",
		ReviewEscape: ReviewEscape{Lanes: []string{"correctness"}},
		RecordedAt:   "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("write original: %v", err)
	}
	origPath := l.entryPath(LedgerEntry{DefectID: "pr-7", Stream: "quality"})
	before, err := os.ReadFile(origPath)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}

	// A second silent write of the same defect is REFUSED (append-only).
	if _, err := l.Write(orig); err == nil {
		t.Fatalf("a silent re-write of an existing entry must be refused")
	}

	// Correction via tombstone amendment: brief -> implementation.
	tomb, err := l.Amend(orig, LedgerEntry{
		Stage: StageImplementation, DossierHash: "hash-corrected",
		ReviewEscape: ReviewEscape{Lanes: []string{"correctness"}},
		RecordedAt:   "2026-02-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("amend: %v", err)
	}
	if !tomb.Tombstone || tomb.Supersedes != orig.EntryHash() {
		t.Fatalf("amendment must be a tombstone superseding the original: %+v", tomb)
	}

	// The prior entry's file is UNCHANGED on disk.
	after, err := os.ReadFile(origPath)
	if err != nil {
		t.Fatalf("re-read original: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("prior dossier file was mutated by the amendment:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	// Both files exist (full audit trail); the rollup reflects CURRENT state only.
	tombPath := l.entryPath(tomb)
	if !fileExists(t, tombPath) || !fileExists(t, origPath) {
		t.Fatalf("both the original and the tombstone must persist on disk")
	}
	entries, err := l.ReadAll()
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("trail must hold both entries, got %d", len(entries))
	}
	r := RollupOf(entries)
	if r.Overall.Implementation != 1 || r.Overall.Brief != 0 {
		t.Fatalf("rollup must reflect the correction (current state): want implementation=1 brief=0, got %+v", r.Overall)
	}
	if r.ByStream["quality"].Implementation != 1 {
		t.Fatalf("per-stream rollup wrong: %+v", r.ByStream)
	}
	if r.ReviewEscapeByLane["correctness"] != 1 {
		t.Fatalf("review-escape distribution wrong: %+v", r.ReviewEscapeByLane)
	}
}

// ---------------------------------------------------------------------------
// LoadTraces round-trip (Verify #2)
// ---------------------------------------------------------------------------

func TestLoadTraces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "defects.jsonl")
	content := `{"fix_commit":"a","fix_pr":1,"inducing_commits":["x"],"inducing_prs":[],"trace_state":"traced"}
` + "\n" + `{"fix_commit":"b","inducing_commits":[],"inducing_prs":[],"trace_state":"could-not-trace"}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := LoadTraces(path)
	if err != nil {
		t.Fatalf("LoadTraces: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 traces, got %d", len(got))
	}
	if !got[0].Traceable() || got[1].Traceable() {
		t.Fatalf("traceability wrong: %+v", got)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func entryFrom(stream, window string, call StageCall) LedgerEntry {
	return LedgerEntry{
		DefectID:      "pr-7",
		Stream:        stream,
		Window:        window,
		Stage:         call.Stage,
		ReviewEscape:  call.ReviewEscape,
		DossierHash:   call.DossierHash,
		Rationale:     call.Rationale,
		ModelAssisted: call.ModelAssisted,
		RecordedAt:    "2026-01-01T00:00:00Z",
	}
}

func fileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}
