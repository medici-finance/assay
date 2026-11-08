package evaluation

import "fmt"

// coverage_repo.go — the Repo port: every read the coverage rule makes of a
// repository's history, as typed methods. The evaluator never runs git, opens a
// file or consults the environment itself; a caller that has a repository
// implements Repo (statusgen's own adapter shells to `git` with replacement
// objects disabled), and a caller that has frozen observations implements it
// over them. Each method answers one question about immutable history and has
// no side effect; a failure is reported as ok=false and the evaluator then
// resolves the affected claim could-not-check or keeps the conservative scope,
// never invents an answer.

// History is the relation MergeBase reports between two commits.
type History int

const (
	// HistoryUnknown: the relation could not be established (git failed).
	HistoryUnknown History = iota
	// HistoryRelated: the two commits share a common ancestor.
	HistoryRelated
	// HistoryUnrelated: the two commits are established to share no ancestor.
	HistoryUnrelated
)

// TreeEntry is one entry of a recursive tree listing: its mode and its path
// from the repository's toplevel.
type TreeEntry struct {
	Mode string
	Path string
}

// Repo is the read-only history the coverage rule is evaluated against. A nil
// Repo is "no repository": every question is answered "cannot tell".
type Repo interface {
	// InRepository reports whether the evaluated tree is inside a git repository.
	InRepository() bool
	// ResolveObject resolves a revision token to the full object id it names.
	ResolveObject(token string) (id string, ok bool)
	// ResolveCommit resolves a revision token to the commit it names (an
	// annotated tag peels to its commit).
	ResolveCommit(token string) (id string, ok bool)
	// IsShallow reports whether the clone's history is cut. ok is false when
	// that could not be read.
	IsShallow() (shallow, ok bool)
	// MergeBase relates two full commit ids.
	MergeBase(a, b string) History
	// MergeBaseAll lists every best common ancestor of a and b.
	MergeBaseAll(a, b string) (bases []string, ok bool)
	// DiffNames lists the paths that differ between the trees of a and b,
	// renames reported as a delete plus an add, so a moved file names its old
	// path too.
	DiffNames(a, b string) (paths []string, ok bool)
	// RevList lists up to limit commits reachable from tip and from none of
	// exclude.
	RevList(tip string, exclude []string, limit int) (commits []string, ok bool)
	// TreeFiles lists every file path in rev's tree; nil when rev is not
	// readable.
	TreeFiles(rev string) []string
	// TreeModes lists every entry of rev's tree with its mode.
	TreeModes(rev string) (entries []TreeEntry, ok bool)
	// ObjectAt returns the object id of path in rev's tree ("" when the path is
	// absent there). ok is false when the lookup itself could not run.
	ObjectAt(rev, path string) (oid string, ok bool)
	// RootPrefix is the evaluated tree's path below its repository's toplevel
	// ("" at the toplevel, "sub/" below it).
	RootPrefix() (prefix string, ok bool)
	// VerifyExpectAt returns the Expect text of Verify row rowID of the brief at
	// briefRel as it stood at rev. ok is false when the brief or the row cannot
	// be read there.
	VerifyExpectAt(rev, briefRel, rowID string) (expect string, ok bool)
	// DeclaredFilesAt returns the brief's declared `files:` entries as they
	// stood at rev. readable is false when the brief cannot be read there;
	// found is false when it declares none that parse.
	DeclaredFilesAt(rev, briefRel string) (entries []string, found, readable bool)
}

// noRepo answers every question "cannot tell": the shape of a bare copy of a
// tree with no git history.
type noRepo struct{}

func (noRepo) InRepository() bool                           { return false }
func (noRepo) ResolveObject(string) (string, bool)          { return "", false }
func (noRepo) ResolveCommit(string) (string, bool)          { return "", false }
func (noRepo) IsShallow() (bool, bool)                      { return false, false }
func (noRepo) MergeBase(string, string) History             { return HistoryUnknown }
func (noRepo) MergeBaseAll(string, string) ([]string, bool) { return nil, false }
func (noRepo) DiffNames(string, string) ([]string, bool)    { return nil, false }
func (noRepo) RevList(string, []string, int) ([]string, bool) {
	return nil, false
}
func (noRepo) TreeFiles(string) []string                            { return nil }
func (noRepo) TreeModes(string) ([]TreeEntry, bool)                 { return nil, false }
func (noRepo) ObjectAt(string, string) (string, bool)               { return "", false }
func (noRepo) RootPrefix() (string, bool)                           { return "", false }
func (noRepo) VerifyExpectAt(string, string, string) (string, bool) { return "", false }
func (noRepo) DeclaredFilesAt(string, string) ([]string, bool, bool) {
	return nil, false, false
}

// DefaultMaxLandingCandidates bounds the squash-commit search of the landing
// check: a witness further from the item's revision than this is
// could-not-check, never assumed landed.
const DefaultMaxLandingCandidates = 1000

// evalCtx is one EvaluateCoverage call's read context: the repository, the
// landing-search bound, and a memo of completed landing searches (many rows
// share one witness commit, and an unlanded one costs up to the bound's worth
// of diffs). It lives and dies with the call.
type evalCtx struct {
	repo       Repo
	maxLanding int
	landed     map[string]landedResult
	// v2 is the witness-v2 read cache (witness.go); policy and forge are the
	// caller's environment rule and optional forge seam.
	v2     *v2Ctx
	policy WitnessPolicy
	forge  Forge
	// goFloors is CoverageInput.GoFloors and readSetsRecorded
	// CoverageInput.ReadSetsRecorded (readset.go).
	goFloors         GoFloorSource
	readSetsRecorded bool
	// readSetDeadline is CoverageInput.ReadSetDeadline, and rsSorted holds
	// each revision's file listing sorted for presence lookups (readset.go).
	readSetDeadline func() string
	rsSorted        map[string][]string
}

func newEvalCtx(repo Repo, maxLanding int) *evalCtx {
	if repo == nil {
		repo = noRepo{}
	}
	if maxLanding <= 0 {
		maxLanding = DefaultMaxLandingCandidates
	}
	return &evalCtx{repo: repo, maxLanding: maxLanding, landed: map[string]landedResult{}, v2: newV2Ctx()}
}

type landedResult struct {
	at     string
	landed bool
}

// treeFilesAt lists every file path in rev's tree; nil when rev does not
// resolve — the caller then finds no match, which widens the scope, never
// narrows it.
func (x *evalCtx) treeFilesAt(rev string) []string {
	if rev == "" {
		return nil
	}
	return x.repo.TreeFiles(rev)
}

// objectAt returns the object id of path p in rev's tree ("" when absent).
func (x *evalCtx) objectAt(rev, p string) (oid string, ok bool) {
	if rev == "" {
		return "", false
	}
	return x.repo.ObjectAt(rev, p)
}

func (x *evalCtx) verifyExpectAt(rev, briefRel, rowID string) (string, bool) {
	if briefRel == "" || rev == "" {
		return "", false
	}
	return x.repo.VerifyExpectAt(rev, briefRel, rowID)
}

func (x *evalCtx) declaredFilesAt(rev, briefRel string) (entries []string, found, readable bool) {
	if briefRel == "" || rev == "" {
		return nil, false, false
	}
	return x.repo.DeclaredFilesAt(rev, briefRel)
}

// rootPrefix is the evaluated tree's path below its repository's toplevel. ok
// is false, never "", when that cannot be said.
func (x *evalCtx) rootPrefix() (string, bool) { return x.repo.RootPrefix() }

// treeModes lists every entry of rev's tree with its mode, so a symbolic link
// (120000) or submodule (160000) on an input's path is seen.
func (x *evalCtx) treeModes(rev string) ([]TreeEntry, bool) { return x.repo.TreeModes(rev) }

// witnessLanded is #2026 cause 1's landing check, judged by TREES, never by
// ancestry: a witness at commit w is honoured at the item's revision t only
// when some commit on t's history since their merge base(s) — the base itself
// when w is on t's history, or the squash commit when w's branch was squash-
// merged — has a tree identical to w's outside docs/streams/** and STATUS.md.
// A witness whose branch never landed as it was tested (it was squash-merged
// onto a main that had moved, or never merged) is a mismatch. why is set when
// the search cannot be completed (a shallow clone, a git failure, more than
// the landing bound's commits to compare): the caller resolves could-not-check.
func (x *evalCtx) witnessLanded(w, t string) (landedAt string, landed bool, why string) {
	key := w + "\x00" + t
	if got, ok := x.landed[key]; ok {
		return got.at, got.landed, ""
	}
	landedAt, landed, why = x.searchLanded(w, t)
	if why == "" {
		x.landed[key] = landedResult{landedAt, landed}
	}
	return landedAt, landed, why
}

// searchLanded is witnessLanded's uncached search.
func (x *evalCtx) searchLanded(w, t string) (landedAt string, landed bool, why string) {
	if why := x.shallowCloneWhy(); why != "" {
		return "", false, why
	}
	bases, ok := x.repo.MergeBaseAll(w, t)
	if !ok {
		return "", false, "the witness's commit and the item's revision could not be related (git merge-base --all failed)"
	}
	if len(bases) == 0 {
		return "", false, "git merge-base --all named no common ancestor"
	}
	// The merge bases and t need no search: compare them first, so a witness on
	// the item's own history (its merge base is the witness itself) lands
	// however many commits followed it. Only the search for a squash commit
	// between them is bounded.
	if at, ok, why := x.landedAmong(w, append(append([]string(nil), bases...), t)); ok || why != "" {
		return at, ok, why
	}
	more, ok := x.repo.RevList(t, bases, x.maxLanding+1)
	if !ok {
		return "", false, "the commits between the witness and the item's revision could not be listed (git rev-list failed)"
	}
	if len(more) > x.maxLanding {
		return "", false, fmt.Sprintf("more than %d commits lie between the witness's merge base and the item's revision, past the bound this search compares", x.maxLanding)
	}
	return x.landedAmong(w, more)
}

// landedAmong reports the first of candidates whose tree is identical to w's
// outside docs/streams/** and STATUS.md; why is set when git cannot compare.
func (x *evalCtx) landedAmong(w string, candidates []string) (landedAt string, landed bool, why string) {
	for _, c := range candidates {
		d, ok := x.repo.DiffNames(w, c)
		if !ok {
			return "", false, "the witness's tree could not be compared with " + c + " (git diff failed)"
		}
		same := true
		for _, p := range d {
			if !isBoardBookkeepingPath(p) {
				same = false
				break
			}
		}
		if same {
			return c, true, ""
		}
	}
	return "", false, ""
}

// shallowCloneWhy is non-empty when the clone is shallow (or that cannot be
// read): its history is cut, so neither the merge base nor the landing search
// can be trusted, and the claim is could-not-check.
func (x *evalCtx) shallowCloneWhy() string {
	shallow, ok := x.repo.IsShallow()
	if !ok {
		return "whether this clone is shallow could not be read (git rev-parse failed)"
	}
	if shallow {
		return "this clone is shallow, so the history between the witness and the item's revision cannot all be read; fetch the full history"
	}
	return ""
}
