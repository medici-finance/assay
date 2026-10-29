package gitcore

import (
	"container/heap"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ReferenceState is one current refs/ name. ObjectID is resolved without peeling
// tags. SymbolicTarget is the final symbolic target, as in for-each-ref's symref.
type ReferenceState struct{ Name, ObjectID, SymbolicTarget string }

// ReferenceSnapshot observes the current reference store on every call. It does
// not use Refs(), whose deliberately hash-only contract omits symbolic names.
// Broken symbolic references are omitted, matching native enumeration; malformed
// store reads remain errors. No objects, hooks, network or subprocesses are used.
func (r *Repo) ReferenceSnapshot() ([]ReferenceState, error) {
	iter, err := r.repo.References()
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	rows := []ReferenceState{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if !strings.HasPrefix(ref.Name().String(), "refs/") {
			return nil
		}
		row := ReferenceState{Name: ref.Name().String()}
		seen := map[plumbing.ReferenceName]bool{}
		for ref.Type() == plumbing.SymbolicReference {
			// Native resolution permits four symbolic hops before the hash lookup.
			if len(seen) >= 4 || seen[ref.Name()] {
				return nil
			}
			seen[ref.Name()] = true
			row.SymbolicTarget = ref.Target().String()
			var e error
			ref, e = r.repo.Reference(ref.Target(), false)
			if errors.Is(e, plumbing.ErrReferenceNotFound) {
				return nil
			}
			if e != nil {
				return e
			}
		}
		row.ObjectID = ref.Hash().String()
		rows = append(rows, row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows, nil
}

// PathMetadata is the last commit selected by default, simplified path history.
// The full ID and committer time are facts, not a native abbreviation formatter.
type PathMetadata struct {
	ID            string
	CommitterTime time.Time
	// NativeReason is a positively observed header profile; never a read failure.
	NativeReason string
}
type metadataQueueItem struct {
	commit *object.Commit
	serial int
}
type metadataQueue []metadataQueueItem

func (q metadataQueue) Len() int { return len(q) }
func (q metadataQueue) Less(i, j int) bool {
	a, b := q[i], q[j]
	if a.commit.Committer.When.Equal(b.commit.Committer.When) {
		return a.serial < b.serial
	}
	return a.commit.Committer.When.After(b.commit.Committer.When)
}
func (q metadataQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *metadataQueue) Push(x any)   { *q = append(*q, x.(metadataQueueItem)) }
func (q *metadataQueue) Pop() any     { a := *q; x := a[len(a)-1]; *q = a[:len(a)-1]; return x }

// LastPathCommit matches log -1's default history simplification for one literal
// repository-relative file or directory. At a TREESAME merge only the first
// matching parent is followed. This is not full-history or first-parent mode.
// A missing object is an error, never an absent path or a native retry.
func (r *Repo) LastPathCommit(rev, literalPath string) (*PathMetadata, error) {
	if path.IsAbs(literalPath) || strings.ContainsAny(literalPath, "\\:*?[") || path.Clean(literalPath) == ".." || strings.HasPrefix(path.Clean(literalPath), "../") {
		return nil, fmt.Errorf("gitcore: nonliteral repository path")
	}
	literalPath = path.Clean(literalPath)
	h, err := r.Resolve(rev)
	if err != nil {
		return nil, err
	}
	load := func(id plumbing.Hash) (*object.Commit, string, error) {
		raw, err := r.RawCommit(id.String())
		if err != nil {
			return nil, "", err
		}
		headers := strings.SplitN(string(raw), "\n\n", 2)[0]
		authors, committers := 0, 0
		committer := ""
		for _, line := range strings.Split(headers, "\n") {
			if strings.HasPrefix(line, "author ") {
				authors++
			}
			if strings.HasPrefix(line, "committer ") {
				committers++
				committer = line
			}
		}
		if authors != 1 || committers != 1 {
			return nil, "noncanonical signature header multiplicity requires native timestamp semantics", nil
		}
		c, err := r.repo.CommitObject(id)
		if err != nil {
			return nil, "", err
		}
		var encoded strings.Builder
		if err := c.Committer.Encode(&encoded); err != nil {
			return nil, "", err
		}
		if committer != "committer "+encoded.String() {
			return nil, "noncanonical committer signature requires native timestamp semantics", nil
		}
		return c, "", nil
	}
	head, why, err := load(h)
	if err != nil {
		return nil, err
	}
	if why != "" {
		return &PathMetadata{NativeReason: why}, nil
	}
	q := &metadataQueue{}
	heap.Init(q)
	serial := 0
	seen := map[plumbing.Hash]bool{}
	push := func(c *object.Commit) {
		if !seen[c.Hash] {
			seen[c.Hash] = true
			heap.Push(q, metadataQueueItem{c, serial})
			serial++
		}
	}
	push(head)
	states := map[plumbing.Hash]string{}
	at := func(c *object.Commit) (string, error) {
		if v, ok := states[c.TreeHash]; ok {
			return v, nil
		}
		if literalPath == "." {
			return c.TreeHash.String(), nil
		}
		tree, e := c.Tree()
		if e != nil {
			return "", e
		}
		entry, e := tree.FindEntry(literalPath)
		if errors.Is(e, object.ErrEntryNotFound) || errors.Is(e, object.ErrDirectoryNotFound) {
			states[c.TreeHash] = ""
			return "", nil
		}
		if e != nil {
			return "", e
		}
		v := fmt.Sprintf("%o:%s", entry.Mode, entry.Hash)
		states[c.TreeHash] = v
		return v, nil
	}
	for q.Len() > 0 {
		c := heap.Pop(q).(metadataQueueItem).commit
		value, e := at(c)
		if e != nil {
			return nil, e
		}
		parents := []*object.Commit{}
		same := false
		for _, ph := range c.ParentHashes {
			parent, why, e := load(ph)
			if e != nil {
				return nil, e
			}
			if why != "" {
				return &PathMetadata{NativeReason: why}, nil
			}
			pvalue, e := at(parent)
			if e != nil {
				return nil, e
			}
			parents = append(parents, parent)
			if value == pvalue {
				parents = []*object.Commit{parent}
				same = true
				break
			}
		}
		if !same && (len(c.ParentHashes) > 0 || value != "") {
			return &PathMetadata{ID: c.Hash.String(), CommitterTime: c.Committer.When}, nil
		}
		for _, parent := range parents {
			push(parent)
		}
	}
	return nil, nil
}
