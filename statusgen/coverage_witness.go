package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// coverage_witness.go — the git-backed half of the witness-v2 port
// (evaluation.WitnessRepo, statusgen/36). Every read is batched per revision
// (one ls-tree, one check-attr, one cat-file --batch, one first-parent log)
// and runs with replace objects disabled twice over: --no-replace-objects on
// the command line and GIT_NO_REPLACE_OBJECTS=1 in the environment, so a
// `refs/replace/` ref can give no landing commit a parent shape it does not
// have. A corpus with no v2 cell makes none of these reads.

// witnessGit is coverageGit plus GIT_NO_REPLACE_OBJECTS=1 and no system or
// global attributes file: a v2 digest depends on the repository's own
// committed content, never on this machine's attribute configuration.
func witnessGit(root string, args ...string) *exec.Cmd {
	cmd := coverageGit(root, append([]string{"-c", "core.attributesFile=" + os.DevNull, "-c", "core.quotePath=false"}, args...)...)
	cmd.Env = append(cmd.Env, "GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1")
	return cmd
}

func (r coverageRepo) ListTree(rev string) ([]evaluation.TreeBlob, bool) {
	out, err := witnessGit(r.root, "ls-tree", "-r", "-z", "--full-tree", "--end-of-options", rev).Output()
	if err != nil {
		return nil, false
	}
	var entries []evaluation.TreeBlob
	for _, e := range splitNUL(out) {
		meta, name, found := strings.Cut(e, "\t")
		f := strings.Fields(meta)
		if !found || len(f) != 3 {
			return nil, false
		}
		entries = append(entries, evaluation.TreeBlob{Mode: f[0], OID: f[2], Path: name})
	}
	return entries, true
}

func (r coverageRepo) CheckAttr(rev string) (map[string][]string, bool) {
	files, ok := r.ListTree(rev)
	if !ok {
		return nil, false
	}
	var in bytes.Buffer
	for _, e := range files {
		in.WriteString(e.Path)
		in.WriteByte(0)
	}
	cmd := witnessGit(r.root, "check-attr", "-a", "-z", "--stdin", "--source="+rev)
	cmd.Stdin = &in
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	fields := splitNUL(out)
	if len(fields)%3 != 0 {
		return nil, false
	}
	attrs := map[string][]string{}
	for i := 0; i < len(fields); i += 3 {
		attrs[fields[i]] = append(attrs[fields[i]], fields[i+1]+": "+fields[i+2])
	}
	return attrs, true
}

func (r coverageRepo) ReadBlobs(rev string, paths []string) (map[string][]byte, bool) {
	var in bytes.Buffer
	for _, p := range paths {
		if strings.ContainsAny(p, "\n\x00") {
			return nil, false
		}
		in.WriteString(rev + ":" + p + "\n")
	}
	cmd := witnessGit(r.root, "cat-file", "--batch")
	cmd.Stdin = &in
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	got := map[string][]byte{}
	br := bufio.NewReader(bytes.NewReader(out))
	for _, p := range paths {
		header, err := br.ReadString('\n')
		if err != nil {
			return nil, false
		}
		f := strings.Fields(header)
		if len(f) == 2 && f[1] == "missing" {
			continue
		}
		if len(f) != 3 {
			return nil, false
		}
		size, err := strconv.Atoi(f[2])
		if err != nil || size < 0 {
			return nil, false
		}
		body := make([]byte, size+1)
		if _, err := io.ReadFull(br, body); err != nil || body[size] != '\n' {
			return nil, false
		}
		if f[1] == "blob" {
			got[p] = body[:size]
		}
	}
	return got, true
}

func (r coverageRepo) WitnessAdditions(rev string) ([]evaluation.WitnessAddition, bool) {
	out, err := witnessGit(r.root, "log", "--first-parent", "--diff-merges=first-parent", "--diff-filter=A", "--no-renames",
		"--format=%x1e%H%x1f%P%x1f%B%x1f", "--name-only", "--end-of-options", rev, "--", evaluation.WitnessRecordDir).Output()
	if err != nil {
		return nil, false
	}
	var adds []evaluation.WitnessAddition
	for _, chunk := range strings.Split(string(out), "\x1e") {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		f := strings.SplitN(chunk, "\x1f", 4)
		if len(f) != 4 {
			return nil, false
		}
		for _, name := range strings.Split(f[3], "\n") {
			if name = strings.TrimSpace(name); name != "" {
				adds = append(adds, evaluation.WitnessAddition{Path: name, Commit: f[0], Parents: strings.Fields(f[1]), Message: f[2]})
			}
		}
	}
	return adds, true
}

func (r coverageRepo) FirstParents(tip string) ([]string, bool) {
	out, err := witnessGit(r.root, "rev-list", "--first-parent", "--end-of-options", tip).Output()
	if err != nil {
		return nil, false
	}
	return strings.Fields(string(out)), true
}

func (r coverageRepo) ShallowCommits() (map[string]bool, bool) {
	out, err := witnessGit(r.root, "rev-parse", "--git-path", "shallow").Output()
	if err != nil {
		return nil, false
	}
	p := strings.TrimSpace(string(out))
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]bool{}, true
	}
	if err != nil {
		return nil, false
	}
	set := map[string]bool{}
	for _, c := range strings.Fields(string(raw)) {
		set[c] = true
	}
	return set, true
}

func (r coverageRepo) ReplacedObjects() (map[string]bool, bool) {
	out, err := witnessGit(r.root, "for-each-ref", "--format=%(refname)", "refs/replace/").Output()
	if err != nil {
		return nil, false
	}
	set := map[string]bool{}
	for _, ref := range strings.Fields(string(out)) {
		set[strings.TrimPrefix(ref, "refs/replace/")] = true
	}
	return set, true
}

func (r coverageRepo) HashWorktree(paths []string) (map[string]string, bool) {
	if len(paths) == 0 {
		return map[string]string{}, true
	}
	var in bytes.Buffer
	for _, p := range paths {
		if strings.ContainsAny(p, "\n\x00") {
			return nil, false
		}
		in.WriteString(p + "\n")
	}
	cmd := witnessGit(r.root, "hash-object", "--stdin-paths")
	cmd.Stdin = &in
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	ids := strings.Fields(string(out))
	if len(ids) != len(paths) {
		return nil, false
	}
	got := make(map[string]string, len(paths))
	for i, p := range paths {
		got[p] = ids[i]
	}
	return got, true
}

// --- memoRepo: one evaluation's v2 answers, asked once ---------------------

func (m *memoRepo) witnessInner() (evaluation.WitnessRepo, bool) {
	w, ok := m.inner.(evaluation.WitnessRepo)
	return w, ok
}

// memoV2 caches one typed v2 answer under key.
func memoV2[T any](m *memoRepo, key string, compute func(evaluation.WitnessRepo) (T, bool)) (T, bool) {
	m.mu.Lock()
	a, ok := m.v2calls[key]
	m.mu.Unlock()
	if ok {
		return a.v.(T), a.ok
	}
	var v T
	ok = false
	if w, has := m.witnessInner(); has {
		v, ok = compute(w)
	}
	m.mu.Lock()
	if m.v2calls == nil {
		m.v2calls = map[string]memoV2Answer{}
	}
	m.v2calls[key] = memoV2Answer{v: v, ok: ok}
	m.mu.Unlock()
	return v, ok
}

type memoV2Answer struct {
	v  any
	ok bool
}

func (m *memoRepo) ListTree(rev string) ([]evaluation.TreeBlob, bool) {
	v, ok := memoV2(m, memoKey("ls-tree-oid", rev), func(w evaluation.WitnessRepo) ([]evaluation.TreeBlob, bool) { return w.ListTree(rev) })
	return slices.Clone(v), ok
}

func (m *memoRepo) CheckAttr(rev string) (map[string][]string, bool) {
	return memoV2(m, memoKey("check-attr", rev), func(w evaluation.WitnessRepo) (map[string][]string, bool) { return w.CheckAttr(rev) })
}

func (m *memoRepo) ReadBlobs(rev string, paths []string) (map[string][]byte, bool) {
	return memoV2(m, memoKey("cat-file", append([]string{rev}, paths...)...), func(w evaluation.WitnessRepo) (map[string][]byte, bool) {
		return w.ReadBlobs(rev, paths)
	})
}

func (m *memoRepo) WitnessAdditions(rev string) ([]evaluation.WitnessAddition, bool) {
	v, ok := memoV2(m, memoKey("witness-adds", rev), func(w evaluation.WitnessRepo) ([]evaluation.WitnessAddition, bool) {
		return w.WitnessAdditions(rev)
	})
	return slices.Clone(v), ok
}

func (m *memoRepo) FirstParents(tip string) ([]string, bool) {
	v, ok := memoV2(m, memoKey("first-parents", tip), func(w evaluation.WitnessRepo) ([]string, bool) { return w.FirstParents(tip) })
	return slices.Clone(v), ok
}

func (m *memoRepo) ShallowCommits() (map[string]bool, bool) {
	return memoV2(m, memoKey("shallow-commits"), func(w evaluation.WitnessRepo) (map[string]bool, bool) { return w.ShallowCommits() })
}

func (m *memoRepo) ReplacedObjects() (map[string]bool, bool) {
	return memoV2(m, memoKey("replaced"), func(w evaluation.WitnessRepo) (map[string]bool, bool) { return w.ReplacedObjects() })
}

func (m *memoRepo) HashWorktree(paths []string) (map[string]string, bool) {
	// The working tree can change between calls; never memoized.
	if w, ok := m.witnessInner(); ok {
		return w.HashWorktree(paths)
	}
	return nil, false
}
