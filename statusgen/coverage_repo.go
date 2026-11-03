package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// coverage_repo.go — statusgen's git-backed implementation of the evaluation
// package's Repo port. This is the ONLY place the coverage rule's history reads
// reach `git`: the evaluator asks typed questions, this adapter answers them by
// running git with replacement objects disabled (--no-replace-objects): what a
// witness is compared against is the object store's own content, never a
// `refs/replace/` substitution. Every argument form is the one the evaluator
// used before the extraction (statusgen/15), so the verdicts are unchanged.

type coverageRepo struct{ root string }

func newCoverageRepo(root string) evaluation.Repo { return coverageRepo{root: root} }

// coverageGitHook, when set (tests only), is told about every git invocation
// the coverage port builds, so a test can bound them (TestCoverageGitCallsBounded).
var coverageGitHook func(root string, args []string)

// coverageGit runs git in root with replacement objects ignored. Like every
// provenance read (historyGitEnv) it runs without the repository-locating
// environment variables, so the -C directory alone names the repository: the
// consumer-delivery snapshot enumerates its tree through this adapter.
func coverageGit(root string, args ...string) *exec.Cmd {
	if h := coverageGitHook; h != nil {
		h(root, args)
	}
	return historyGitEnv(exec.Command("git", append([]string{"--no-replace-objects", "-C", root}, args...)...))
}

// The per-(root, revision) listings below are memoized: a commit's tree never
// changes, and a --coverage run asks for the same few revisions once per brief.
var (
	repoCacheMu     sync.Mutex
	treeFilesCache  = map[string][]string{}
	treeModesCache  = map[string][]evaluation.TreeEntry{}
	rootPrefixCache = map[string]string{}
)

func (r coverageRepo) InRepository() bool {
	read := gitCoreFor(r.root)
	if read.native != "" {
		return coverageGit(r.root, "rev-parse", "--git-dir").Run() == nil
	}
	return read.err == nil
}

func (r coverageRepo) ResolveObject(token string) (string, bool) {
	out, err := coverageGit(r.root, "rev-parse", "--verify", "--quiet", "--end-of-options", token).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func (r coverageRepo) ResolveCommit(token string) (string, bool) {
	if nativeCommitSelector(token) {
		out, err := coverageGit(r.root, "rev-parse", "--verify", "--quiet", "--end-of-options", token+"^{commit}").Output()
		if err != nil {
			return "", false
		}
		return strings.TrimSpace(string(out)), true
	}
	read := gitCoreForCommits(r.root, token)
	if strings.HasPrefix(token, "refs/") {
		read = gitCoreFor(r.root)
	}
	if read.err != nil {
		return "", false
	}
	if read.native != "" {
		out, err := coverageGit(r.root, "rev-parse", "--verify", "--quiet", "--end-of-options", token+"^{commit}").Output()
		return strings.TrimSpace(string(out)), err == nil
	}
	h, err := read.repo.Resolve(token)
	if err != nil {
		return "", false
	}
	return h.String(), true
}

func (r coverageRepo) IsShallow() (shallow, ok bool) {
	read := gitCoreFor(r.root)
	if read.err != nil {
		return false, false
	}
	if read.native != "" {
		out, err := coverageGit(r.root, "rev-parse", "--is-shallow-repository").Output()
		return strings.TrimSpace(string(out)) != "false", err == nil
	}
	// Fresh canonical metadata observation proved the shallow file absent.
	return false, true
}

func (r coverageRepo) MergeBase(a, b string) evaluation.History {
	err := coverageGit(r.root, "merge-base", "--end-of-options", a, b).Run()
	if err == nil {
		return evaluation.HistoryRelated
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return evaluation.HistoryUnrelated
	}
	return evaluation.HistoryUnknown
}

func (r coverageRepo) MergeBaseAll(a, b string) ([]string, bool) {
	out, err := coverageGit(r.root, "merge-base", "--all", "--end-of-options", a, b).Output()
	if err != nil {
		return nil, false
	}
	return strings.Fields(string(out)), true
}

// DiffNames: --no-renames (a rename prints only its DESTINATION otherwise, so a
// declared file renamed away never showed up as a change to its old path); -z
// keeps every path byte-exact (no core.quotePath quoting).
func (r coverageRepo) DiffNames(a, b string) ([]string, bool) {
	out, err := coverageGit(r.root, "diff", "--name-only", "--no-renames", "-z", "--end-of-options", a, b, "--").Output()
	if err != nil {
		return nil, false
	}
	return splitNUL(out), true
}

func (r coverageRepo) RevList(tip string, exclude []string, limit int) ([]string, bool) {
	args := []string{"rev-list", "--max-count=" + strconv.Itoa(limit), "--end-of-options", tip}
	for _, b := range exclude {
		args = append(args, "^"+b)
	}
	out, err := coverageGit(r.root, args...).Output()
	if err != nil {
		return nil, false
	}
	return strings.Fields(string(out)), true
}

// TreeFiles lists every file path in rev's tree (`git ls-tree -r -z`, so a path
// with unusual bytes is never quoted into something no declaration matches).
func (r coverageRepo) TreeFiles(rev string) []string {
	key := r.root + "\x00" + rev
	repoCacheMu.Lock()
	files, ok := treeFilesCache[key]
	repoCacheMu.Unlock()
	if ok {
		return files
	}
	read := gitCoreForCommits(r.root, rev)
	if read.err != nil {
		return nil
	}
	if read.native != "" {
		out, err := coverageGit(r.root, "ls-tree", "-r", "-z", "--name-only", "--full-tree", "--end-of-options", rev).Output()
		if err != nil {
			return nil
		}
		files = splitNUL(out)
	} else {
		entries, err := read.repo.TreeEntries(rev, true, false)
		if err != nil {
			return nil
		}
		files = make([]string, 0, len(entries))
		for _, e := range entries {
			files = append(files, e.Path)
		}
	}
	repoCacheMu.Lock()
	treeFilesCache[key] = files
	repoCacheMu.Unlock()
	return files
}

// TreeModes lists every entry of rev's tree with its mode, so a symbolic link
// (120000) or submodule (160000) on an input's path is seen.
func (r coverageRepo) TreeModes(rev string) ([]evaluation.TreeEntry, bool) {
	key := r.root + "\x00" + rev
	repoCacheMu.Lock()
	got, ok := treeModesCache[key]
	repoCacheMu.Unlock()
	if ok {
		return got, true
	}
	read := gitCoreForCommits(r.root, rev)
	if read.err != nil {
		return nil, false
	}
	var entries []evaluation.TreeEntry
	var out []byte
	if read.native == "" {
		got, err := read.repo.TreeEntries(rev, true, false)
		if err != nil {
			return nil, false
		}
		for _, e := range got {
			entries = append(entries, evaluation.TreeEntry{Mode: e.Mode, Path: e.Path})
		}
	} else {
		var err error
		out, err = coverageGit(r.root, "ls-tree", "-r", "-z", "--full-tree", "--end-of-options", rev).Output()
		if err != nil {
			return nil, false
		}
	}
	for _, e := range splitNUL(out) {
		meta, name, found := strings.Cut(e, "\t")
		f := strings.Fields(meta)
		if !found || len(f) != 3 {
			return nil, false // unparseable ls-tree entry
		}
		entries = append(entries, evaluation.TreeEntry{Mode: f[0], Path: name})
	}
	repoCacheMu.Lock()
	treeModesCache[key] = entries
	repoCacheMu.Unlock()
	return entries, true
}

func (r coverageRepo) ObjectAt(rev, p string) (string, bool) {
	out, err := coverageGit(r.root, "ls-tree", "-z", "--full-tree", "--end-of-options", rev, "--", p).Output()
	if err != nil {
		return "", false
	}
	for _, e := range splitNUL(out) {
		meta, name, found := strings.Cut(e, "\t")
		if !found || name != p {
			continue
		}
		if f := strings.Fields(meta); len(f) == 3 {
			return f[2], true
		}
	}
	return "", true
}

func (r coverageRepo) RootPrefix() (string, bool) {
	repoCacheMu.Lock()
	got, ok := rootPrefixCache[r.root]
	repoCacheMu.Unlock()
	if ok {
		return got, true
	}
	prefix, err := provenanceRootPrefix(r.root)
	if err != nil {
		return "", false
	}

	repoCacheMu.Lock()
	rootPrefixCache[r.root] = prefix
	repoCacheMu.Unlock()
	return prefix, true
}

// briefBodyAt reads a brief's markdown body (frontmatter stripped) as it stood
// at git revision rev (`git show <rev>:./<path>`). rev must be a BASE revision —
// a `+dirty` token is not a git object. ok is false whenever the path does not
// resolve to a usable git object at rev.
func (r coverageRepo) briefBodyAt(rev, briefRel string) (string, bool) {
	out, err := coverageGit(r.root, "show", "--end-of-options", rev+":./"+briefRel).Output()
	if err != nil {
		return "", false
	}
	content := strings.ReplaceAll(string(out), "\r\n", "\n")
	body := content
	if first, _, _ := strings.Cut(content, "\n"); strings.TrimSpace(first) == "---" {
		if _, b, ferr := splitFrontmatter(content); ferr == nil {
			body = b
		}
	}
	return body, true
}

func (r coverageRepo) VerifyExpectAt(rev, briefRel, rowID string) (string, bool) {
	body, ok := r.briefBodyAt(rev, briefRel)
	if !ok {
		return "", false
	}
	for _, hr := range briefVerifyRows(extractSectionByPrefix(body, "Verify")) {
		if hr.ID == rowID {
			return hr.Expect, true
		}
	}
	return "", false
}

func (r coverageRepo) DeclaredFilesAt(rev, briefRel string) ([]string, bool, bool) {
	body, ok := r.briefBodyAt(rev, briefRel)
	if !ok {
		return nil, false, false
	}
	entries, found := extractContextDeclaredEntriesRaw(body)
	// Historical entries are normalized the way the current tree's are, so a
	// backslash-spelled entry compares equal on both sides.
	for i, e := range entries {
		entries[i] = filepath.ToSlash(e)
	}
	return entries, found, true
}

// splitNUL splits git `-z` output into its non-empty entries.
func splitNUL(out []byte) []string {
	var parts []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			parts = append(parts, f)
		}
	}
	return parts
}
