package gitcore

// Shared-history reachability for RefsContaining.
//
// `git for-each-ref --contains <commit>` asks ONE question of the whole ref set: which
// tips have <commit> as an ancestor. The first go-git port answered it by asking go-git's
// Commit.IsAncestor once PER REF — a fresh, unmemoized depth-first walk of that tip's
// entire history for every ref, with nothing shared between walks. On a checkout with
// ~1000 remote-tracking refs over ~17k commits that is ~10 million commit visits (and,
// for a brand-new commit reachable from nothing, every one of them walks to the roots
// before returning false): measured at ~108s for a single call, where the git binary
// answers in ~15ms. deskpushguard calls it once per commit ahead of origin/main from the
// pre-push hook, so a push cost minutes and blew the desk preflight's 45s budget.
//
// This file answers the same question EXACTLY, with two independent, compounding
// mechanisms — neither is a heuristic and neither can produce a false negative:
//
//  1. One shared walk instead of N. Every tip's history is explored once, into a single
//     parent->children adjacency; every commit node is decoded at most once per Repo and
//     cached for later calls (a commit's parents are immutable, so the cache can never go
//     stale). The answer is then a reverse breadth-first search from <commit> along the
//     recorded child edges: the tips it reaches are exactly the refs that contain it. Work
//     is bounded by the size of the reachable commit graph, not by refs x history.
//
//  2. Commit-graph generation numbers as an EXACT early cut-off. git's commit-graph file
//     (objects/info/commit-graph, or the split chain under commit-graphs/) stores each
//     commit's topological level: 1 for a root, else 1 + max(parents' level). By that
//     definition a strict ancestor ALWAYS has a strictly smaller level than its
//     descendant, so a commit whose level is <= <commit>'s cannot have <commit> as an
//     ancestor — and neither can anything above it, so the walk need not expand it at
//     all. This is the same bound `git for-each-ref --contains` uses internally. It is
//     an exact invariant of the numbering, NOT a date: committer dates are never read,
//     never compared, never used to skip anything — rebases, clock skew and imported
//     history cannot make a level lie. <commit> itself need not be in the graph: its
//     level is computed from its parents' by the same definition, and if any needed
//     level is unknown (no graph, a commit missing from it, a pre-2.19 graph written
//     with zero levels, or a level saturated at git's V1 cap) the cut-off simply
//     degrades to "unknown = never skip" and mechanism 1 alone carries the call. A
//     commit-graph is therefore an accelerator, never a requirement.
//
// A self-check backs mechanism 2 with mechanism 1: while walking, every expanded edge
// child->parent asserts level(parent) < level(child) whenever both are known. A
// commit-graph that violates it (a corrupt or hand-edited file) disables the cut-off for
// that call and the walk is redone without it. The check sees only edges the walk
// expands, so it catches a graph whose levels contradict each other along a path it
// walks; a tip whose recorded level is merely understated is cut off before any of its
// edges are expanded and is never examined. The commit-graph is therefore trusted to the
// same degree git itself trusts it: it lives inside the local .git alongside the refs
// and objects the walk already relies on, and is never transported by fetch or clone.

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5/plumbing"
	commitgraph "github.com/go-git/go-git/v5/plumbing/format/commitgraph/v2"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// genV1Max is git's GENERATION_NUMBER_V1_MAX: the topological level saturates at this
// value in the commit-graph file (30 bits), so a level >= genV1Max only means "at least
// this deep" and is NOT usable for the strict-inequality cut-off. genUnknown (0) is what
// a commit-graph written before git 2.19 stores for every commit, and what this package
// assigns to a commit whose level cannot be derived; it also disables the cut-off.
const (
	genV1Max   uint64 = 0x3FFFFFFF
	genUnknown uint64 = 0
)

// reachNode is one decoded commit: its parents and its topological level (genUnknown
// when not known). Both are immutable properties of the commit hash, so a node is
// cached for the life of the Repo.
type reachNode struct {
	parents []plumbing.Hash
	gen     uint64
}

// reachIndex is the per-Repo commit-node cache plus the (optional) commit-graph index it
// reads levels from. nodes only ever grows and only ever holds facts derived from
// immutable objects, so it is shared across every RefsContaining call on the Repo.
type reachIndex struct {
	mu    sync.Mutex // held for the whole of any call that reads or grows the index
	repo  *Repo
	graph commitgraph.Index // nil when the repository has no usable commit-graph
	nodes map[plumbing.Hash]*reachNode
	// shallow is the repository's shallow boundary (.git/shallow): commits whose parents
	// are absent BY DESIGN. git's --contains treats each one as parentless and answers
	// from the history it has; so does this walk. Read once, when the index is built.
	shallow map[plumbing.Hash]bool

	// Work counters, read by the package's regression test: how many commits were decoded
	// from the object database (the expensive path) and how many were served from the
	// commit-graph file. They are the deterministic proxy for "the N-walks regression is
	// back" that a wall-clock assertion could never be.
	odbLoads   int
	graphLoads int
	// genPushes counts every frame generation() pushes onto its derivation stack. It is
	// bounded by the number of distinct commits derived: a derived level always resolves
	// (a root is level 1, every child one above its highest parent), so a node is never
	// re-entered. The package's regression test pins that bound on a graph whose stored
	// levels are all zero, the case where every level has to be derived.
	genPushes int
	// shallowCuts counts commits recorded parentless because they sit on the shallow
	// boundary; the oracle test's shallow-clone state asserts the boundary was honoured.
	shallowCuts int
}

// errReachViolation is raised when an expanded edge contradicts the commit-graph's level
// invariant (level(parent) >= level(child) with both known). It is caught inside
// RefsContaining, which retries with the cut-off disabled.
var errReachViolation = errors.New("gitcore: commit-graph generation numbers are not monotonic")

// reach returns the Repo's shared reachability index, opening the commit-graph (if any)
// on first use. Never fails: a missing or unreadable commit-graph just leaves graph nil.
func (r *Repo) reach() *reachIndex {
	r.reachMu.Lock()
	defer r.reachMu.Unlock()
	if r.reachIdx == nil {
		r.reachIdx = &reachIndex{
			repo:    r,
			graph:   r.openCommitGraph(),
			nodes:   map[plumbing.Hash]*reachNode{},
			shallow: r.shallowSet(),
		}
	}
	return r.reachIdx
}

// openCommitGraph opens the repository's commit-graph file or split chain via go-git's
// reader, returning nil when there is none, when it cannot be read, or when the
// repository's own config sets core.commitGraph=false (the same switch git honours).
func (r *Repo) openCommitGraph() commitgraph.Index {
	if cfg, err := r.repo.Config(); err == nil && cfg != nil && cfg.Raw != nil {
		for _, o := range cfg.Raw.Section("core").Options {
			if strings.EqualFold(o.Key, "commitGraph") && strings.EqualFold(strings.TrimSpace(o.Value), "false") {
				return nil
			}
		}
	}
	st, ok := r.repo.Storer.(extensionTolerantStorer)
	if !ok {
		return nil
	}
	fst, ok := st.Storer.(*filesystem.Storage)
	if !ok {
		return nil
	}
	var fs billy.Filesystem = fst.Filesystem()
	idx, err := commitgraph.OpenChainOrFileIndex(fs)
	if err != nil {
		return nil
	}
	return idx
}

// shallowSet reads the repository's shallow boundary, matching what git reads from
// .git/shallow. Empty for a complete repository; an unreadable file is treated as empty,
// which only means a boundary commit's missing parent then surfaces as an error (the
// pre-existing "broken object store" path) instead of being cut cleanly.
func (r *Repo) shallowSet() map[plumbing.Hash]bool {
	hashes, err := r.repo.Storer.Shallow()
	if err != nil || len(hashes) == 0 {
		return nil
	}
	out := make(map[plumbing.Hash]bool, len(hashes))
	for _, h := range hashes {
		out[h] = true
	}
	return out
}

// newNode records commit h with the parents and level it was read with, unless h sits
// on the shallow boundary: then its parents are absent by design and it is recorded
// parentless with an unknown level, exactly as git's --contains treats a .git/shallow
// entry. (A shallow repository carries no commit-graph, so the unknown level costs
// nothing; if one is ever present, an unknown level only disables the cut-off above
// this commit — never a wrong answer.)
func (ri *reachIndex) newNode(h plumbing.Hash, parents []plumbing.Hash, gen uint64) *reachNode {
	if ri.shallow[h] {
		ri.shallowCuts++
		return &reachNode{parents: nil, gen: genUnknown}
	}
	return &reachNode{parents: parents, gen: gen}
}

// node returns the decoded commit h, from the cache, the commit-graph, or the object
// database, in that order. plumbing.ErrObjectNotFound (wrapped) when h is not a commit
// this repository can read.
func (ri *reachIndex) node(h plumbing.Hash) (*reachNode, error) {
	if n, ok := ri.nodes[h]; ok {
		return n, nil
	}
	if ri.graph != nil {
		if i, err := ri.graph.GetIndexByHash(h); err == nil {
			cd, err := ri.graph.GetCommitDataByIndex(i)
			if err == nil {
				n := ri.newNode(h, cd.ParentHashes, cd.Generation)
				ri.graphLoads++
				ri.nodes[h] = n
				return n, nil
			}
		}
	}
	c, err := object.GetCommit(ri.repo.repo.Storer, h)
	if err != nil {
		return nil, err
	}
	ri.odbLoads++
	n := ri.newNode(h, c.ParentHashes, genUnknown)
	ri.nodes[h] = n
	return n, nil
}

// peelToCommit follows h through any annotated-tag layers to the commit underneath,
// matching the peeling `for-each-ref --contains` applies to a tag ref. ok=false when h
// (or the object it ultimately points at) is not a readable commit — a tag of a tree or
// blob, a dangling ref — which for-each-ref simply does not list.
func (ri *reachIndex) peelToCommit(h plumbing.Hash) (plumbing.Hash, bool) {
	for depth := 0; depth < 32; depth++ { // a tag chain deeper than this is not a real repo
		if _, ok := ri.nodes[h]; ok {
			return h, true
		}
		if ri.graph != nil {
			if _, err := ri.graph.GetIndexByHash(h); err == nil {
				return h, true
			}
		}
		obj, err := ri.repo.repo.Object(plumbing.AnyObject, h)
		if err != nil {
			return h, false
		}
		switch o := obj.(type) {
		case *object.Commit:
			ri.odbLoads++
			ri.nodes[h] = ri.newNode(h, o.ParentHashes, genUnknown)
			return h, true
		case *object.Tag:
			h = o.Target
		default:
			return h, false
		}
	}
	return h, false
}

// generation returns h's topological level: the commit-graph's value when h is in the
// graph, else 1 + max(parents' levels) by the same definition git uses to write the
// file, computed iteratively (a long unindexed history must not blow the stack).
// genUnknown when any level it depends on is unknown, and genV1Max when any is
// saturated; both disable the cut-off for the caller.
func (ri *reachIndex) generation(h plumbing.Hash) (uint64, error) {
	type frame struct {
		h    plumbing.Hash
		n    *reachNode
		next int // index of the next parent to resolve
	}
	start, err := ri.node(h)
	if err != nil {
		return genUnknown, err
	}
	if start.gen != genUnknown {
		return start.gen, nil
	}
	stack := []frame{{h: h, n: start}}
	ri.genPushes++
	inProgress := map[plumbing.Hash]bool{h: true}
	for len(stack) > 0 {
		f := &stack[len(stack)-1]
		if f.next < len(f.n.parents) {
			p := f.n.parents[f.next]
			f.next++
			pn, err := ri.node(p)
			if err != nil {
				return genUnknown, err
			}
			if pn.gen == genUnknown && !inProgress[p] {
				inProgress[p] = true
				stack = append(stack, frame{h: p, n: pn})
				ri.genPushes++
			}
			continue
		}
		// Every parent resolved (or provably unknown): derive this level.
		gen := uint64(1)
		for _, p := range f.n.parents {
			pg := ri.nodes[p].gen
			if pg == genUnknown {
				gen = genUnknown
				break
			}
			if pg >= genV1Max {
				gen = genV1Max
				break
			}
			if pg+1 > gen {
				gen = pg + 1
			}
		}
		f.n.gen = gen
		delete(inProgress, f.h)
		stack = stack[:len(stack)-1]
	}
	return start.gen, nil
}

// cutoff reports whether a commit at level gen can be skipped when looking for
// descendants of a commit at level targetGen: only when BOTH levels are exactly known
// (nonzero, below the V1 saturation cap) and gen <= targetGen — a descendant's level is
// always strictly greater than its ancestor's.
func cutoff(gen, targetGen uint64) bool {
	return gen != genUnknown && gen < genV1Max &&
		targetGen != genUnknown && targetGen < genV1Max &&
		gen <= targetGen
}

// refsContaining is RefsContaining's implementation: see that method's contract.
func (ri *reachIndex) refsContaining(target plumbing.Hash, tips map[string]plumbing.Hash) ([]string, error) {
	targetGen := genUnknown
	if ri.graph != nil {
		g, err := ri.generation(target)
		if err != nil {
			return nil, fmt.Errorf("gitcore: commit %s: %w", target, err)
		}
		targetGen = g
	} else if _, err := ri.node(target); err != nil {
		return nil, fmt.Errorf("gitcore: commit %s: %w", target, err)
	}

	// Resolve each tip through tag layers to a commit once; refs whose tip is not a
	// commit are dropped here, matching for-each-ref.
	tipCommit := make(map[string]plumbing.Hash, len(tips))
	for name, h := range tips {
		if c, ok := ri.peelToCommit(h); ok {
			tipCommit[name] = c
		}
	}

	desc, err := ri.descendants(target, targetGen, tipCommit)
	if errors.Is(err, errReachViolation) {
		// The commit-graph's levels contradict themselves: distrust them for this call
		// and answer from the walk alone. Nodes stay cached, so this is cheap.
		desc, err = ri.descendants(target, genUnknown, tipCommit)
	}
	if err != nil {
		return nil, err
	}

	var out []string
	for name, c := range tipCommit {
		if desc[c] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// descendants walks the history behind every tip once (never expanding a commit the
// level cut-off proves cannot descend from target), recording parent->children edges,
// then returns the set of commits reachable FROM target along those edges — target
// itself included. A tip is in the set exactly when it contains target.
func (ri *reachIndex) descendants(target plumbing.Hash, targetGen uint64, tipCommit map[string]plumbing.Hash) (map[plumbing.Hash]bool, error) {
	children := map[plumbing.Hash][]plumbing.Hash{}
	visited := map[plumbing.Hash]bool{}
	stack := make([]plumbing.Hash, 0, len(tipCommit))
	for _, c := range tipCommit {
		stack = append(stack, c)
	}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited[x] {
			continue
		}
		visited[x] = true
		if x == target {
			continue // its own ancestry can never contain it; nothing below matters
		}
		n, err := ri.node(x)
		if err != nil {
			// An interior commit this repository cannot read: the answer is
			// unknowable, and a security caller must hear that rather than a
			// shorter list. (A tip that is not a commit was already dropped by
			// peelToCommit, so this is a genuinely broken object store.)
			return nil, fmt.Errorf("gitcore: commit %s: %w", x, err)
		}
		if cutoff(n.gen, targetGen) {
			continue
		}
		for _, p := range n.parents {
			// Load the parent now (it would be loaded when popped anyway) so the level
			// invariant is checked on EVERY expanded edge, not only on already-cached ones.
			pn, err := ri.node(p)
			if err != nil {
				return nil, fmt.Errorf("gitcore: commit %s (parent of %s): %w", p, x, err)
			}
			if targetGen != genUnknown && n.gen != genUnknown && pn.gen != genUnknown &&
				n.gen < genV1Max && pn.gen >= n.gen {
				return nil, errReachViolation
			}
			children[p] = append(children[p], x)
			stack = append(stack, p)
		}
	}

	desc := map[plumbing.Hash]bool{target: true}
	queue := []plumbing.Hash{target}
	for len(queue) > 0 {
		q := queue[0]
		queue = queue[1:]
		for _, c := range children[q] {
			if !desc[c] {
				desc[c] = true
				queue = append(queue, c)
			}
		}
	}
	return desc, nil
}

// reachStats is the regression test's window onto the index's work counters.
type reachStats struct {
	odbLoads, graphLoads, genPushes, shallowCuts int
	graph                                        bool
}

func (r *Repo) reachStats() reachStats {
	ri := r.reach()
	ri.mu.Lock()
	defer ri.mu.Unlock()
	return reachStats{odbLoads: ri.odbLoads, graphLoads: ri.graphLoads, genPushes: ri.genPushes, shallowCuts: ri.shallowCuts, graph: ri.graph != nil}
}
