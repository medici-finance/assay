package main

// prfacts.go — the per-PR skip (#2421): a PR whose head, last update and check state are
// all unchanged since the previous sweep reuses that sweep's per-PR read results instead of
// re-reading them.
//
// What is reused is narrow on purpose. The review set and the check rollup are NOT cached —
// they arrive fresh with every sweep's one repo-level snapshot read, so an approval or a
// check result can never be served stale from here. The facts kept are the PR's changed-file
// set (the risk-path scan and the benign-merge compare) and the files touched between a
// reviewed sha and the head: reads whose answer is a function of the head commit and the
// base branch, re-fetched whenever anything that could move them moved.
//
// The reuse key is (head sha, updatedAt, base branch, a digest of the check rollup); an
// entry is reused only when ALL FOUR match and updatedAt is non-empty. A backend that does
// not report updatedAt (GitLab today) therefore never skips — it reads every sweep exactly as
// before. Only complete, error-free reads are stored: a truncated diff or a failed read is
// re-read next sweep, so a degrade is never remembered as a fact. The trust-gate blessing read
// for an untrusted author is not cached either: that gate stays a live read.
//
// One file per repo, rewritten at the end of each successful repo sweep with exactly the PRs
// that sweep saw (so closed PRs fall out). A corrupt, unreadable or other-schema file is an
// empty store — every PR re-reads.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const prFactsSchema = 1

// prFacts is one PR's reusable read results, valid only for the key it was stored under.
type prFacts struct {
	Head      string `json:"head"`
	UpdatedAt string `json:"updatedAt"`
	BaseRef   string `json:"baseRef"`
	Rollup    string `json:"rollup"`

	HaveFiles bool     `json:"haveFiles,omitempty"`
	Files     []string `json:"files,omitempty"`

	// Intervals maps a reviewed sha to the files touched between it and Head.
	Intervals map[string][]string `json:"intervals,omitempty"`
}

type prFactsFile struct {
	Schema int             `json:"schema"`
	Repo   string          `json:"repo"`
	PRs    map[int]prFacts `json:"prs"`
}

// prFactStore holds the previous sweep's facts for one repo and collects this sweep's.
// A nil store is valid and reuses nothing (every caller outside the actions sweep).
type prFactStore struct {
	mu    sync.Mutex
	repo  string
	path  string
	prior map[int]prFacts
	next  map[int]prFacts
}

func prFactsPath(repo string) string {
	root, err := boardCacheDir()
	if err != nil || root == "" {
		return ""
	}
	return filepath.Join(root, "prfacts", safeName(repo)+".json")
}

// loadPRFacts opens repo's store; any problem reading it yields an empty one.
func loadPRFacts(repo string) *prFactStore {
	s := &prFactStore{repo: repo, path: prFactsPath(repo), prior: map[int]prFacts{}, next: map[int]prFacts{}}
	if s.path == "" {
		return s
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return s
	}
	var f prFactsFile
	if json.Unmarshal(raw, &f) != nil || f.Schema != prFactsSchema || f.Repo != repo || f.PRs == nil {
		return s
	}
	s.prior = f.PRs
	return s
}

// save writes this sweep's facts atomically. Best effort: a failed write costs a re-read.
func (s *prFactStore) save() {
	if s == nil || s.path == "" {
		return
	}
	s.mu.Lock()
	raw, err := json.Marshal(prFactsFile{Schema: prFactsSchema, Repo: s.repo, PRs: s.next})
	s.mu.Unlock()
	if err == nil {
		_ = writeFileAtomic(s.path, raw)
	}
}

// factKey is the key a PR's facts are valid under.
func factKey(p prBase) prFacts {
	b, _ := json.Marshal(p.StatusCheckRollup)
	sum := sha256.Sum256(b)
	return prFacts{Head: p.HeadRefOid, UpdatedAt: p.UpdatedAt, BaseRef: p.BaseRefName, Rollup: hex.EncodeToString(sum[:])}
}

func sameKey(a, b prFacts) bool {
	return a.Head == b.Head && a.UpdatedAt == b.UpdatedAt && a.BaseRef == b.BaseRef && a.Rollup == b.Rollup
}

// current returns the facts recorded so far this sweep for p, seeded from the prior sweep's
// entry when its key matches. ok=false means p cannot reuse anything (no updatedAt, or a
// nil store). Caller holds s.mu.
func (s *prFactStore) current(p prBase) (prFacts, bool) {
	if s == nil || p.UpdatedAt == "" || p.HeadRefOid == "" {
		return prFacts{}, false
	}
	key := factKey(p)
	if f, ok := s.next[p.Number]; ok && sameKey(f, key) {
		return f, true
	}
	if f, ok := s.prior[p.Number]; ok && sameKey(f, key) {
		return f, true
	}
	return key, true
}

// changedFiles is fetchChangedFiles with reuse: the PR's changed-file set from this sweep or
// the previous one when the key matches, else a fresh read (stored only when complete).
func (s *prFactStore) changedFiles(repo string, p prBase) (map[string]bool, bool, error) {
	if s != nil {
		s.mu.Lock()
		f, ok := s.current(p)
		if ok && f.HaveFiles {
			s.next[p.Number] = f
			s.mu.Unlock()
			boardMeter.skip(2) // the PR-metadata read and the file-list read
			return toSet(f.Files), true, nil
		}
		s.mu.Unlock()
	}
	files, complete, err := fetchChangedFiles(repo, p.Number)
	if s == nil || err != nil || !complete {
		return files, complete, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.current(p); ok {
		f.HaveFiles, f.Files = true, fromSet(files)
		s.next[p.Number] = f
	}
	return files, complete, err
}

// interval is changedFilesBetween with reuse, keyed by the reviewed sha under the same PR key.
func (s *prFactStore) interval(repo string, p prBase, reviewed string) (map[string]bool, error) {
	if s != nil {
		s.mu.Lock()
		f, ok := s.current(p)
		if iv, hit := f.Intervals[reviewed]; ok && hit {
			s.next[p.Number] = f
			s.mu.Unlock()
			// The compare and at least the head commit's read; a longer chain saved more.
			boardMeter.skip(2)
			return toSet(iv), nil
		}
		s.mu.Unlock()
	}
	files, err := changedFilesBetween(repo, reviewed, p.HeadRefOid)
	if s == nil || err != nil {
		return files, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.current(p); ok {
		iv := make(map[string][]string, len(f.Intervals)+1)
		for k, v := range f.Intervals {
			iv[k] = v
		}
		iv[reviewed] = fromSet(files)
		f.Intervals = iv
		s.next[p.Number] = f
	}
	return files, nil
}

func toSet(files []string) map[string]bool {
	set := make(map[string]bool, len(files))
	for _, f := range files {
		set[f] = true
	}
	return set
}

func fromSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}
