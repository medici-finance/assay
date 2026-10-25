package evaluation

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// witness_fake_test.go — an in-memory repository implementing Repo and
// WitnessRepo for the witness-v2 tests, and the fixture builder that lands a
// v2 record the way a delivering change request does. Object ids are git's
// own blob ids (sha1 of "blob <n>\0<content>"), so a fixture's digests have
// the shape real records carry.

type fakeEntry struct {
	mode    string
	content string
}

type fakeCommit struct {
	id      string
	parents []string
	msg     string
	tree    map[string]fakeEntry
	attrs   map[string][]string
}

type fakeRepo struct {
	commits  map[string]*fakeCommit
	refs     map[string]string
	shallow  map[string]bool
	replaced map[string]bool
	expectAt map[string]string // VerifyExpectAt answers, keyed by revision
	// additions, when set, replaces the computed WitnessAdditions answer.
	additions func([]WitnessAddition) []WitnessAddition
	// objectParents, when set for a commit id, is the parent list that
	// commit's object states (CatObjects), apart from the graph the fake walks.
	objectParents map[string][]string
	// noFirstParents makes FirstParents report that it could not read.
	noFirstParents bool
	seq            int
	// reads counts the batched v2 port reads (CheckAttr, CatObjects).
	reads int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{commits: map[string]*fakeCommit{}, refs: map[string]string{}, shallow: map[string]bool{}, replaced: map[string]bool{}, expectAt: map[string]string{}}
}

func fakeBlobID(content string) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(content), content)))
	return hex.EncodeToString(sum[:])
}

// commit stores a commit and returns its id.
func (f *fakeRepo) commit(parents []string, msg string, tree map[string]fakeEntry, attrs map[string][]string) string {
	f.seq++
	paths := make([]string, 0, len(tree))
	for p := range tree {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var b strings.Builder
	fmt.Fprintf(&b, "%d\x00%s\x00%s\x00", f.seq, strings.Join(parents, ","), msg)
	for _, p := range paths {
		fmt.Fprintf(&b, "%s %s %s\x00", tree[p].mode, fakeBlobID(tree[p].content), p)
	}
	sum := sha1.Sum([]byte(b.String()))
	id := hex.EncodeToString(sum[:])
	f.commits[id] = &fakeCommit{id: id, parents: parents, msg: msg, tree: cloneTree(tree), attrs: cloneAttrs(attrs)}
	return id
}

func cloneTree(t map[string]fakeEntry) map[string]fakeEntry {
	out := make(map[string]fakeEntry, len(t))
	for k, v := range t {
		out[k] = v
	}
	return out
}

func cloneAttrs(a map[string][]string) map[string][]string {
	out := make(map[string][]string, len(a))
	for k, v := range a {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func (f *fakeRepo) resolve(token string) (string, bool) {
	if id, ok := f.refs[token]; ok {
		return id, true
	}
	if _, ok := f.commits[token]; ok {
		return token, true
	}
	return "", false
}

func (f *fakeRepo) InRepository() bool                        { return true }
func (f *fakeRepo) ResolveObject(token string) (string, bool) { return f.resolve(token) }
func (f *fakeRepo) ResolveCommit(token string) (string, bool) { return f.resolve(token) }
func (f *fakeRepo) IsShallow() (bool, bool)                   { return len(f.shallow) > 0, true }
func (f *fakeRepo) MergeBase(string, string) History          { return HistoryUnknown }
func (f *fakeRepo) MergeBaseAll(string, string) ([]string, bool) {
	return nil, false
}

func (f *fakeRepo) DiffNames(a, b string) ([]string, bool) {
	ca, ok1 := f.commits[a]
	cb, ok2 := f.commits[b]
	if !ok1 || !ok2 {
		return nil, false
	}
	set := map[string]bool{}
	for p, e := range ca.tree {
		if o, ok := cb.tree[p]; !ok || o != e {
			set[p] = true
		}
	}
	for p := range cb.tree {
		if _, ok := ca.tree[p]; !ok {
			set[p] = true
		}
	}
	return sortedKeys(set), true
}

func (f *fakeRepo) RevList(string, []string, int) ([]string, bool) { return nil, false }

func (f *fakeRepo) TreeFiles(rev string) []string {
	c, ok := f.commits[rev]
	if !ok {
		return nil
	}
	var out []string
	for p := range c.tree {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func (f *fakeRepo) TreeModes(rev string) ([]TreeEntry, bool) {
	c, ok := f.commits[rev]
	if !ok {
		return nil, false
	}
	var out []TreeEntry
	for _, p := range f.TreeFiles(rev) {
		out = append(out, TreeEntry{Mode: c.tree[p].mode, Path: p})
	}
	return out, true
}

func (f *fakeRepo) ObjectAt(rev, p string) (string, bool) {
	c, ok := f.commits[rev]
	if !ok {
		return "", false
	}
	if e, ok := c.tree[p]; ok {
		return fakeBlobID(e.content), true
	}
	return "", true
}

func (f *fakeRepo) RootPrefix() (string, bool) { return "", true }

func (f *fakeRepo) VerifyExpectAt(rev, _, _ string) (string, bool) {
	e, ok := f.expectAt[rev]
	return e, ok
}

func (f *fakeRepo) DeclaredFilesAt(string, string) ([]string, bool, bool) { return nil, false, false }

// --- WitnessRepo -------------------------------------------------------------

func (f *fakeRepo) ListTree(rev string) ([]TreeBlob, bool) {
	c, ok := f.commits[rev]
	if !ok {
		return nil, false
	}
	var out []TreeBlob
	for _, p := range f.TreeFiles(rev) {
		out = append(out, TreeBlob{Mode: c.tree[p].mode, OID: fakeBlobID(c.tree[p].content), Path: p})
	}
	return out, true
}

func (f *fakeRepo) CheckAttr(rev string, paths []string) (map[string][]string, bool) {
	c, ok := f.commits[rev]
	if !ok {
		return nil, false
	}
	f.reads++
	out := map[string][]string{}
	for _, p := range paths {
		if a, ok := c.attrs[p]; ok {
			out[p] = append([]string(nil), a...)
		}
	}
	return out, true
}

func (f *fakeRepo) CatObjects(specs []string) (map[string]CatObject, bool) {
	f.reads++
	out := map[string]CatObject{}
	for _, s := range specs {
		rev, path, isPath := strings.Cut(s, ":")
		c, ok := f.commits[rev]
		if !ok {
			continue
		}
		if !isPath {
			out[s] = CatObject{OID: c.id, Type: "commit", Content: f.commitObject(c)}
			continue
		}
		if e, ok := c.tree[path]; ok {
			out[s] = CatObject{OID: fakeBlobID(e.content), Type: "blob", Content: []byte(e.content)}
		}
	}
	return out, true
}

// commitObject renders c as a raw commit object: tree, parent, author and
// committer headers, a blank line, the message.
func (f *fakeRepo) commitObject(c *fakeCommit) []byte {
	parents := c.parents
	if p, ok := f.objectParents[c.id]; ok {
		parents = p
	}
	var b strings.Builder
	sum := sha1.Sum([]byte("tree " + c.id))
	b.WriteString("tree " + hex.EncodeToString(sum[:]) + "\n")
	for _, p := range parents {
		b.WriteString("parent " + p + "\n")
	}
	b.WriteString("author Fixture <fixture@example.test> 1700000000 +0000\ncommitter Fixture <fixture@example.test> 1700000000 +0000\n\n")
	b.WriteString(c.msg)
	return []byte(b.String())
}

func (f *fakeRepo) WitnessAdditions(rev string) ([]WitnessAddition, bool) {
	var adds []WitnessAddition
	for id := rev; id != ""; {
		c, ok := f.commits[id]
		if !ok {
			return nil, false
		}
		var parentTree map[string]fakeEntry
		if len(c.parents) > 0 {
			p, ok := f.commits[c.parents[0]]
			if !ok {
				return nil, false
			}
			parentTree = p.tree
		}
		for _, path := range f.TreeFiles(id) {
			if _, had := parentTree[path]; !had && strings.HasPrefix(path, WitnessRecordDir) {
				adds = append(adds, WitnessAddition{Path: path, Commit: id})
			}
		}
		id = ""
		if len(c.parents) > 0 {
			id = c.parents[0]
		}
	}
	if f.additions != nil {
		adds = f.additions(adds)
	}
	return adds, true
}

func (f *fakeRepo) FirstParents(tip string) ([]string, bool) {
	if f.noFirstParents {
		return nil, false
	}
	var out []string
	for id := tip; id != ""; {
		c, ok := f.commits[id]
		if !ok {
			return nil, false
		}
		out = append(out, id)
		id = ""
		if len(c.parents) > 0 {
			id = c.parents[0]
		}
	}
	return out, true
}

func (f *fakeRepo) ShallowCommits() (map[string]bool, bool)  { return f.shallow, true }
func (f *fakeRepo) ReplacedObjects() (map[string]bool, bool) { return f.replaced, true }
func (f *fakeRepo) HashWorktree([]string) (map[string]string, bool) {
	return nil, false
}

// plainRepo hides the WitnessRepo port: a Repo that can answer v1 questions
// only.
type plainRepo struct{ Repo }

// fakeForge is the `--forge` seam with canned answers.
type fakeForge struct {
	branch  string
	landed  map[string]string
	unknown bool
}

func (g fakeForge) DefaultBranch() (string, bool) { return g.branch, g.branch != "" }
func (g fakeForge) LandedCommit(cr string) (string, bool, bool) {
	if g.unknown {
		return "", false, false
	}
	c, ok := g.landed[cr]
	return c, ok, true
}

// --- the fixture -------------------------------------------------------------

const (
	fxBriefRel  = "docs/streams/s/brief-01-x.md"
	fxBriefID   = "s/01"
	fxPins      = "statusgen:\n  tag: statusgen/v1.0.59\ndesk-tools:\n  tag: desk-tools/v1.0.59\nharness:\n  digest: sha256:1111111111111111111111111111111111111111111111111111111111111111\n"
	fxRunCommit = "0123456789abcdef0123456789abcdef01234567" // squash-orphaned: in no object store
	fxOutput    = "2222222222222222222222222222222222222222222222222222222222222222"
)

// v2fx builds one brief with one v2 witness landed by change request 12.
type v2fx struct {
	t     *testing.T
	files map[string]fakeEntry // the tree the run read, and the base tree
	attrs map[string][]string
	row   VerifyRow
	// extraRows are further Verify rows of the brief, each with its witness
	// cited from extraWit (row id → witness edit).
	extraRows []VerifyRow
	declared  []string
	found     bool
	deps      *DependencyManifest

	form        string // github-merge (default), github-squash, gitlab-merge, none
	landingMsg  string // overrides the form's message
	landingPars int    // 0: the form's own parent count
	evilMerge   bool   // the feature branch lacks the record; the merge adds it
	briefInBase bool   // the citing brief's Evidence lands before the record does

	rec      func(*WitnessRecord) // edits before encoding
	raw      func([]byte) []byte  // edits after encoding (the cell then names the edited bytes' sha)
	cell     func(string) string
	wit      func(*Witness)
	afterRow func(*VerifyRow) // edits to the Verify row after the run
	atR      func(tree map[string]fakeEntry, attrs map[string][]string)
	feature  func(tree map[string]fakeEntry) // further files the landing carries
	refs     func(f *v2fx)                   // edits to the refs after the build

	policy        WitnessPolicy
	forge         Forge
	acceptance    string
	evalAtFeature bool
	revision      string // overrides the evaluated revision
	plainRepo     bool   // evaluate through a Repo without the WitnessRepo port

	repo                         *fakeRepo
	base, featureID, landing, R  string
	record                       WitnessRecord
	recordID, recordPath, runner string
	witness                      Witness
}

func newV2fx(t *testing.T) *v2fx {
	t.Helper()
	return &v2fx{
		t: t,
		files: map[string]fakeEntry{
			"tools/x/go.mod":        {"100644", "module example.test/x\n"},
			"tools/x/go.sum":        {"100644", "example.test/dep v1.0.0 h1:aaaa\n"},
			"tools/x/pkg/a.go":      {"100644", "package pkg\n\nfunc A() int { return 1 }\n"},
			"tools/x/pkg/a_test.go": {"100644", "package pkg\n"},
			"other/go.mod":          {"100644", "module example.test/other\n"},
			"other/go.sum":          {"100644", "example.test/odep v1.0.0 h1:bbbb\n"},
			"go.work":               {"100644", "go 1.25\n\nuse ./tools/x\n"},
			"README.md":             {"100644", "readme\n"},
			WitnessPinFile:          {"100644", fxPins},
			fxBriefRel:              {"100644", "brief v0\n"},
		},
		attrs: map[string][]string{
			"tools/x/pkg/a.go": {"text: set"},
		},
		row:      VerifyRow{ID: "1", Command: "cd tools/x && go test ./pkg/", Expect: "exit 0; PASS", ClassCell: "check:ci"},
		declared: []string{"tools/x/pkg/"},
		found:    true,
		form:     "github-merge",
		repo:     newFakeRepo(),
	}
}

func (fx *v2fx) listing(tree map[string]fakeEntry) []TreeBlob {
	var out []TreeBlob
	for p, e := range tree {
		out = append(out, TreeBlob{Mode: e.mode, OID: fakeBlobID(e.content), Path: p})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// defaultRecord is the record a pinned-container run writes for row r over
// the run tree.
func (fx *v2fx) defaultRecord(r VerifyRow) WitnessRecord {
	spec := WitnessClaimSpec(fx.declared, fx.found, fx.deps, r.ID)
	content := ComputeWitnessContent(spec, fx.listing(fx.files), fx.attrs)
	return WitnessRecord{
		Schema: 2, Brief: fxBriefID, Row: r.ID,
		CommandSHA256: WitnessTextDigest(r.Command), ExpectSHA256: WitnessTextDigest(r.Expect), ClassSHA256: WitnessTextDigest(r.ClassCell),
		Runner: "verifier-app", RunnerSource: "app-token", RunnerClass: WitnessRunnerContainer, Network: "none",
		Tools:       map[string]string{"statusgen": "statusgen/v1.0.59", "desk-tools": "desk-tools/v1.0.59", "harness": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "go": "go1.25.3"},
		ToolsSource: "recorded", Origin: "run", Result: "PASS", Exit: "0", OutputSHA256: fxOutput, Date: "2026-10-25",
		CR: "12", Scope: content.Scope, Inputs: content.Inputs, Implicit: content.Implicit, Attributes: content.Attributes,
		TreeDigest: content.TreeDigest, DirtyPaths: []string{},
		Informational: WitnessInformational{Commit: fxRunCommit},
	}
}

func (fx *v2fx) build() {
	t := fx.t
	t.Helper()
	rec := fx.defaultRecord(fx.row)
	if fx.rec != nil {
		fx.rec(&rec)
	}
	canonical, id, err := EncodeWitnessRecord(rec)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if fx.raw != nil {
		canonical = fx.raw(canonical)
		// The cell names the id the reader recomputes when the edited bytes
		// still parse (the edit then reaches a later step), else the bytes' own
		// sha (the edit is refused at step 1).
		if _, pid, err := ParseWitnessRecord(canonical); err == nil {
			id = pid
		} else {
			id = sha256Hex(canonical)
		}
	}
	fx.record, fx.recordID = rec, id
	fx.recordPath = WitnessRecordPath(fxBriefID, id)
	fx.runner = "verifier-app @ [w2] cr:12 rec:" + id
	if fx.cell != nil {
		fx.runner = fx.cell(fx.runner)
	}

	baseTree := cloneTree(fx.files)
	if fx.briefInBase {
		baseTree[fxBriefRel] = fakeEntry{"100644", "brief v1 (Evidence)\n"}
	}
	fx.base = fx.repo.commit(nil, "base", baseTree, fx.attrs)

	featureTree := cloneTree(baseTree)
	featureTree[fxBriefRel] = fakeEntry{"100644", "brief v1 (Evidence)\n"}
	recordEntry := fakeEntry{"100644", string(canonical)}
	if !fx.evilMerge {
		featureTree[fx.recordPath] = recordEntry
	}
	if fx.feature != nil {
		fx.feature(featureTree)
	}
	fx.featureID = fx.repo.commit([]string{fx.base}, "Evidence: row 1", featureTree, fx.attrs)

	landingTree := cloneTree(featureTree)
	landingTree[fx.recordPath] = recordEntry
	msg, parents := "", []string{fx.base}
	switch fx.form {
	case "github-merge":
		msg, parents = "Merge pull request #12 from example-org/feat\n\nEvidence: row 1\n", []string{fx.base, fx.featureID}
	case "github-squash":
		msg = "Evidence: row 1 (#12)\n"
	case "gitlab-merge":
		msg, parents = "Merge branch 'feat' into 'main'\n\nEvidence: row 1\n\nSee merge request example-org/example-repo!12\n", []string{fx.base, fx.featureID}
	case "none":
		msg = "Evidence: row 1\n"
	default:
		t.Fatalf("unknown landing form %q", fx.form)
	}
	if fx.landingMsg != "" {
		msg = fx.landingMsg
	}
	switch fx.landingPars {
	case 1:
		parents = []string{fx.base}
	case 2:
		parents = []string{fx.base, fx.featureID}
	}
	fx.landing = fx.repo.commit(parents, msg, landingTree, fx.attrs)

	fx.R = fx.landing
	if fx.atR != nil {
		tree, attrs := cloneTree(landingTree), cloneAttrs(fx.attrs)
		fx.atR(tree, attrs)
		fx.R = fx.repo.commit([]string{fx.landing}, "later change", tree, attrs)
	}
	if fx.evalAtFeature {
		fx.R = fx.featureID
	}
	fx.repo.refs["refs/remotes/origin/HEAD"] = fx.landing
	if fx.atR != nil {
		fx.repo.refs["refs/remotes/origin/HEAD"] = fx.R
	}
	if fx.refs != nil {
		fx.refs(fx)
	}

	fx.witness = Witness{Command: fx.row.Command, State: StatePass, Tree: "[w2]", Runner: fx.runner, Result: rec.Result, Output: rec.OutputSHA256, Date: rec.Date}
	if rec.Result == "FAIL" {
		fx.witness.State = StateFail
	}
	if fx.wit != nil {
		fx.wit(&fx.witness)
	}
	if fx.afterRow != nil {
		fx.afterRow(&fx.row)
	}
}

// eval builds the fixture (when not yet built) and returns the claims.
func (fx *v2fx) eval() []Claim {
	fx.t.Helper()
	if fx.R == "" {
		fx.build()
	}
	b := BriefObservation{
		ID: fxBriefID, BriefRel: fxBriefRel, DeclaredInputs: fx.declared, DeclaredInputsFound: fx.found,
		VerifyRows:         append([]VerifyRow{fx.row}, fx.extraRows...),
		Witnesses:          map[string][]Witness{fx.row.ID: {fx.witness}},
		AcceptanceRevision: fx.acceptance,
	}
	for _, r := range fx.extraRows {
		b.Witnesses[r.ID] = []Witness{fx.witness}
	}
	in := CoverageInput{Revision: fx.R, Briefs: []BriefObservation{b}, WitnessPolicy: fx.policy, Forge: fx.forge}
	if fx.revision != "" {
		in.Revision = fx.revision
	}
	if fx.deps != nil {
		in.Dependencies = map[string]DependencyManifest{fxBriefID: *fx.deps}
	}
	var repo Repo = fx.repo
	if fx.plainRepo {
		repo = plainRepo{fx.repo}
	}
	return EvaluateCoverage(in, repo)[fxBriefID].Claims
}

func (fx *v2fx) claim() Claim {
	fx.t.Helper()
	cs := fx.eval()
	if len(cs) == 0 {
		fx.t.Fatal("no claim")
	}
	return cs[0]
}

// wantRelease asserts a released claim.
func wantRelease(t *testing.T, name string, c Claim) {
	t.Helper()
	if !c.Released || c.Result != ResultPass {
		t.Errorf("%s: want a release, got %s: %s", name, c.Result, c.Reason)
	}
}

// wantGap asserts a refusal naming gap, with the result gapResult gives it and
// (when non-empty) a reason containing every one of contains.
func wantGap(t *testing.T, name string, c Claim, gap string, contains ...string) {
	t.Helper()
	if c.Released {
		t.Errorf("%s: want %s, got a release: %s", name, gap, c.Reason)
		return
	}
	if got := WitnessGap(c.Reason); got != gap {
		t.Errorf("%s: want gap %s, got %q (%s: %s)", name, gap, got, c.Result, c.Reason)
		return
	}
	if c.Result != gapResult(gap) {
		t.Errorf("%s: want result %s for %s, got %s", name, gapResult(gap), gap, c.Result)
	}
	for _, s := range contains {
		if !strings.Contains(c.Reason, s) {
			t.Errorf("%s: the reason %q does not name %q", name, c.Reason, s)
		}
	}
}
