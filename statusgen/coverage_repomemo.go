package main

import (
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// coverage_repomemo.go — one evaluation's answers, asked once.
//
// EvaluateCoverage asks the Repo port the same questions once per brief: the
// same HEAD resolved, the same shallow check, the same merge-base and diff
// between a witness sha and HEAD. On a board of a few hundred briefs that was
// thousands of `git` processes for a few hundred distinct questions (the
// reconcile --backfill profile on the toolkit board: ~2,500 of ~2,800 calls
// were repeats). memoRepo answers each distinct question from the first
// answer.
//
// SCOPE. A memoRepo lives for ONE evaluateCoverage call, so it never holds an
// answer across a change to the repository: nothing writes refs or objects
// while one evaluation runs. The process-wide tree caches in coverage_repo.go
// stay as they are (a tree id is immutable).
//
// Every answer is cached as given, ok=false included: a read that failed once
// in this evaluation reads as failed for the rest of it, the same verdict a
// re-run of the failing git command would have to be trusted to repeat.
// Slices are copied out so a caller that sorts or appends cannot change a
// later caller's answer.

type memoRepo struct {
	inner evaluation.Repo

	mu    sync.Mutex
	calls map[string]memoAnswer
	// v2calls holds the witness-v2 port's typed answers (coverage_witness.go).
	v2calls map[string]memoV2Answer
}

type memoAnswer struct {
	s     string
	list  []string
	ok    bool
	found bool
	hist  evaluation.History
}

func newMemoRepo(inner evaluation.Repo) *memoRepo {
	return &memoRepo{inner: inner, calls: map[string]memoAnswer{}}
}

// memo returns the cached answer for key, or computes, stores and returns it.
// The computation runs outside the lock; two concurrent first askers may both
// compute, and the answers agree.
func (m *memoRepo) memo(key string, compute func() memoAnswer) memoAnswer {
	m.mu.Lock()
	a, ok := m.calls[key]
	m.mu.Unlock()
	if ok {
		return a
	}
	a = compute()
	m.mu.Lock()
	m.calls[key] = a
	m.mu.Unlock()
	return a
}

func memoKey(op string, args ...string) string {
	return op + "\x00" + strings.Join(args, "\x00")
}

func (m *memoRepo) InRepository() bool {
	return m.memo(memoKey("in-repo"), func() memoAnswer { return memoAnswer{ok: m.inner.InRepository()} }).ok
}

func (m *memoRepo) ResolveObject(token string) (string, bool) {
	a := m.memo(memoKey("object", token), func() memoAnswer {
		s, ok := m.inner.ResolveObject(token)
		return memoAnswer{s: s, ok: ok}
	})
	return a.s, a.ok
}

func (m *memoRepo) ResolveCommit(token string) (string, bool) {
	a := m.memo(memoKey("commit", token), func() memoAnswer {
		s, ok := m.inner.ResolveCommit(token)
		return memoAnswer{s: s, ok: ok}
	})
	return a.s, a.ok
}

func (m *memoRepo) IsShallow() (bool, bool) {
	a := m.memo(memoKey("shallow"), func() memoAnswer {
		shallow, ok := m.inner.IsShallow()
		return memoAnswer{found: shallow, ok: ok}
	})
	return a.found, a.ok
}

func (m *memoRepo) MergeBase(a, b string) evaluation.History {
	return m.memo(memoKey("merge-base", a, b), func() memoAnswer {
		return memoAnswer{hist: m.inner.MergeBase(a, b)}
	}).hist
}

func (m *memoRepo) MergeBaseAll(a, b string) ([]string, bool) {
	ans := m.memo(memoKey("merge-base-all", a, b), func() memoAnswer {
		list, ok := m.inner.MergeBaseAll(a, b)
		return memoAnswer{list: list, ok: ok}
	})
	return slices.Clone(ans.list), ans.ok
}

func (m *memoRepo) DiffNames(a, b string) ([]string, bool) {
	ans := m.memo(memoKey("diff", a, b), func() memoAnswer {
		list, ok := m.inner.DiffNames(a, b)
		return memoAnswer{list: list, ok: ok}
	})
	return slices.Clone(ans.list), ans.ok
}

func (m *memoRepo) RevList(tip string, exclude []string, limit int) ([]string, bool) {
	key := memoKey("rev-list", append([]string{tip, strconv.Itoa(limit)}, exclude...)...)
	ans := m.memo(key, func() memoAnswer {
		list, ok := m.inner.RevList(tip, exclude, limit)
		return memoAnswer{list: list, ok: ok}
	})
	return slices.Clone(ans.list), ans.ok
}

// TreeFiles, TreeModes and RootPrefix are already cached process-wide by
// coverageRepo (keyed by immutable tree revision); they pass straight through.
func (m *memoRepo) TreeFiles(rev string) []string { return m.inner.TreeFiles(rev) }

func (m *memoRepo) TreeModes(rev string) ([]evaluation.TreeEntry, bool) {
	return m.inner.TreeModes(rev)
}

func (m *memoRepo) RootPrefix() (string, bool) { return m.inner.RootPrefix() }

func (m *memoRepo) ObjectAt(rev, p string) (string, bool) {
	a := m.memo(memoKey("object-at", rev, p), func() memoAnswer {
		s, ok := m.inner.ObjectAt(rev, p)
		return memoAnswer{s: s, ok: ok}
	})
	return a.s, a.ok
}

func (m *memoRepo) VerifyExpectAt(rev, briefRel, rowID string) (string, bool) {
	a := m.memo(memoKey("expect-at", rev, briefRel, rowID), func() memoAnswer {
		s, ok := m.inner.VerifyExpectAt(rev, briefRel, rowID)
		return memoAnswer{s: s, ok: ok}
	})
	return a.s, a.ok
}

func (m *memoRepo) DeclaredFilesAt(rev, briefRel string) ([]string, bool, bool) {
	a := m.memo(memoKey("declared-at", rev, briefRel), func() memoAnswer {
		list, found, readable := m.inner.DeclaredFilesAt(rev, briefRel)
		return memoAnswer{list: list, found: found, ok: readable}
	})
	return slices.Clone(a.list), a.found, a.ok
}
