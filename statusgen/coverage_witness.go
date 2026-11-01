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
// (evaluation.WitnessRepo, statusgen/36). Every read is batched: one ls-tree,
// one check-attr and one first-parent log per revision; one cat-file --batch
// reads the records, the pin file and the adding commits together, and one
// more reads the objects those commits' own parents name.
// Every read runs with replace objects disabled twice over: --no-replace-objects on
// the command line and GIT_NO_REPLACE_OBJECTS=1 in the environment, so a
// `refs/replace/` ref can give no landing commit a parent shape it does not
// have. A corpus with no v2 cell makes none of these reads.

// witnessGit is coverageGit plus GIT_NO_REPLACE_OBJECTS=1 and no system or
// global attributes file, so a v2 digest does not depend on this machine's
// user-level or system-level attribute configuration. It does still honour the
// repository's own $GIT_DIR/info/attributes, which `git check-attr --source`
// reads whatever the flags: a line there can change a recorded attribute set
// on one clone and not another. That fails safe (a false wrong-revision, never
// a false release), and the committed .gitattributes files are the authority
// a digest is meant to bind.
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

func (r coverageRepo) CheckAttr(rev string, paths []string) (map[string][]string, bool) {
	attrs := map[string][]string{}
	if len(paths) == 0 {
		return attrs, true
	}
	var in bytes.Buffer
	for _, p := range paths {
		if strings.ContainsRune(p, 0) {
			return nil, false
		}
		in.WriteString(p)
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
	for i := 0; i < len(fields); i += 3 {
		attrs[fields[i]] = append(attrs[fields[i]], fields[i+1]+": "+fields[i+2])
	}
	return attrs, true
}

func (r coverageRepo) CatObjects(specs []string) (map[string]evaluation.CatObject, bool) {
	got := map[string]evaluation.CatObject{}
	if len(specs) == 0 {
		return got, true
	}
	var in bytes.Buffer
	for _, s := range specs {
		if strings.ContainsAny(s, "\n\x00") {
			return nil, false
		}
		in.WriteString(s + "\n")
	}
	cmd := witnessGit(r.root, "cat-file", "--batch")
	cmd.Stdin = &in
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	br := bufio.NewReader(bytes.NewReader(out))
	for _, s := range specs {
		header, err := br.ReadString('\n')
		if err != nil {
			return nil, false
		}
		f := strings.Fields(header)
		if len(f) >= 2 && (f[len(f)-1] == "missing" || f[len(f)-1] == "ambiguous") {
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
		got[s] = evaluation.CatObject{OID: f[0], Type: f[1], Content: body[:size]}
	}
	return got, true
}

// objectSize is one `cat-file --batch-check` answer.
type objectSize struct {
	Type string
	Size int64
}

// ObjectSizes reads each named object's type and size and no content: one
// `cat-file --batch-check`. A missing object is absent from the map.
func (r coverageRepo) ObjectSizes(oids []string) (map[string]objectSize, bool) {
	got := map[string]objectSize{}
	if len(oids) == 0 {
		return got, true
	}
	var in bytes.Buffer
	for _, s := range oids {
		if strings.ContainsAny(s, "\n\x00") {
			return nil, false
		}
		in.WriteString(s + "\n")
	}
	cmd := witnessGit(r.root, "cat-file", "--batch-check")
	cmd.Stdin = &in
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != len(oids) {
		return nil, false
	}
	for i, l := range lines {
		f := strings.Fields(l)
		if len(f) >= 2 && (f[len(f)-1] == "missing" || f[len(f)-1] == "ambiguous") {
			continue
		}
		if len(f) != 3 {
			return nil, false
		}
		size, err := strconv.ParseInt(f[2], 10, 64)
		if err != nil || size < 0 {
			return nil, false
		}
		got[oids[i]] = objectSize{Type: f[1], Size: size}
	}
	return got, true
}

// StreamObjects reads the named objects in order through one
// `cat-file --batch` whose output is consumed as it arrives, never buffered
// whole: each gets the object's type, size and a reader of exactly its body.
// A missing object, a malformed header or an error from each stops the read.
func (r coverageRepo) StreamObjects(oids []string, each func(oid, typ string, size int64, body io.Reader) error) error {
	if len(oids) == 0 {
		return nil
	}
	var in bytes.Buffer
	for _, s := range oids {
		if strings.ContainsAny(s, "\n\x00") {
			return errors.New("an object name holds a line break")
		}
		in.WriteString(s + "\n")
	}
	cmd := witnessGit(r.root, "cat-file", "--batch")
	cmd.Stdin = &in
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	read := func() error {
		br := bufio.NewReader(out)
		for _, s := range oids {
			header, err := br.ReadString('\n')
			if err != nil {
				return err
			}
			f := strings.Fields(header)
			if len(f) != 3 {
				return errors.New("cat-file: " + s + " is not readable")
			}
			size, err := strconv.ParseInt(f[2], 10, 64)
			if err != nil || size < 0 {
				return errors.New("cat-file: a malformed size for " + s)
			}
			body := io.LimitReader(br, size)
			if err := each(s, f[1], size, body); err != nil {
				return err
			}
			if _, err := io.Copy(io.Discard, body); err != nil {
				return err
			}
			if b, err := br.ReadByte(); err != nil || b != '\n' {
				return errors.New("cat-file: " + s + " is not followed by a line break")
			}
		}
		return nil
	}
	rerr := read()
	if rerr != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	werr := cmd.Wait()
	if rerr != nil {
		return rerr
	}
	return werr
}

// WitnessAdditions reads only which commit added which record: the log
// prints each commit's id and the added paths, NUL-terminated, and no commit
// text at all. The landing check reads the commit's parents and message from
// the commit object (CatObjects).
func (r coverageRepo) WitnessAdditions(rev string) ([]evaluation.WitnessAddition, bool) {
	out, err := witnessGit(r.root, "log", "-z", "--first-parent", "--diff-merges=first-parent", "--diff-filter=A", "--no-renames",
		"--format=%H", "--name-only", "--end-of-options", rev, "--", evaluation.WitnessRecordDir).Output()
	if err != nil {
		return nil, false
	}
	return parseWitnessAdditions(out)
}

// parseWitnessAdditions reads `git log -z --format=%H --name-only` output:
// NUL-terminated fields, each either a commit id or a path that commit added
// (git starts a commit's first path with a newline). A field that is neither
// a full object id nor a path under WitnessRecordDir, or a path before any
// commit id, fails the whole read.
func parseWitnessAdditions(out []byte) ([]evaluation.WitnessAddition, bool) {
	var adds []evaluation.WitnessAddition
	commit := ""
	for _, field := range splitNUL(out) {
		field = strings.TrimPrefix(field, "\n")
		switch {
		case field == "":
			continue
		case evaluation.IsObjectID(field):
			commit = field
		case strings.HasPrefix(field, evaluation.WitnessRecordDir) && commit != "":
			adds = append(adds, evaluation.WitnessAddition{Path: field, Commit: commit})
		default:
			return nil, false
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

func (m *memoRepo) CheckAttr(rev string, paths []string) (map[string][]string, bool) {
	return memoV2(m, memoKey("check-attr", append([]string{rev}, paths...)...), func(w evaluation.WitnessRepo) (map[string][]string, bool) {
		return w.CheckAttr(rev, paths)
	})
}

func (m *memoRepo) CatObjects(specs []string) (map[string]evaluation.CatObject, bool) {
	return memoV2(m, memoKey("cat-objects", specs...), func(w evaluation.WitnessRepo) (map[string]evaluation.CatObject, bool) {
		return w.CatObjects(specs)
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
