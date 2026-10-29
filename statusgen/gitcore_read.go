package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/gitcore"
)

// gitCoreRead is an adapter observation owned by the existing run-scoped reader.
// Only gitcore opens repositories, resolves revisions and caches decoded objects.
// native names a capability mismatch observed BEFORE any object read. An error
// never turns into a native retry: provenance uncertainty keeps its outcome.
type gitCoreRead struct {
	repo   *gitcore.Repo
	prefix string
	top    string
	native string
	err    error
}

func openGitCoreRead(root string) gitCoreRead {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return gitCoreRead{err: err}
	}
	physicalRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return gitCoreRead{err: err}
	}
	info, err := os.Stat(physicalRoot)
	if err != nil {
		return gitCoreRead{err: err}
	}
	if !info.IsDir() {
		return gitCoreRead{err: fmt.Errorf("repository command context is not a directory")}
	}
	root = physicalRoot
	top, err := gitcore.Toplevel(root)
	if err != nil {
		return gitCoreRead{err: err}
	}
	why, err := gitcore.NativeFilesystemReadReason(root, top)
	if err != nil {
		return gitCoreRead{err: err}
	}
	if why != "" {
		return gitCoreRead{native: why}
	}
	format, err := gitcore.ObjectFormat(root)
	if err != nil {
		return gitCoreRead{err: err}
	}
	if format != gitcore.BuildObjectFormat() {
		return gitCoreRead{native: "runtime repository hash format differs from go-git's build format"}
	}
	why, err = gitcore.NativeRepositoryReadReason(top)
	if err != nil {
		return gitCoreRead{err: err}
	}
	if why != "" {
		return gitCoreRead{native: why}
	}
	r, err := gitcore.Open(top)
	if err != nil {
		return gitCoreRead{err: err}
	}
	if why := r.NativeObjectReadReason(); why != "" {
		return gitCoreRead{native: why}
	}
	why, err = r.NativeDiscoveryReason()
	if err != nil {
		return gitCoreRead{err: err}
	}
	if why != "" {
		return gitCoreRead{native: why}
	}

	abs := physicalRoot
	prefix, err := filepath.Rel(top, abs)
	if err != nil {
		return gitCoreRead{err: err}
	}
	if prefix == "." {
		prefix = ""
	} else {
		prefix = filepath.ToSlash(prefix) + "/"
	}
	return gitCoreRead{repo: r, prefix: prefix, top: top}
}

func gitCoreFor(root string) gitCoreRead {
	// These unstripped native policies are observed per operation, including
	// explicitly empty values. They are never frozen in the repository cache.
	for _, key := range []string{"GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS", "GIT_SHALLOW_FILE"} {
		if _, present := os.LookupEnv(key); present {
			return gitCoreRead{native: "native discovery/config/shallow environment profile"}
		}
	}
	if activeGitReads == nil {
		return observeHistoryProfile(openGitCoreRead(root))
	}
	s := activeGitReads
	s.coreMu.Lock()
	if s.core == nil {
		s.core = map[string]gitCoreRead{}
	}
	r, ok := s.core[root]
	if !ok {
		r = openGitCoreRead(root)
		s.core[root] = r
	}
	s.coreMu.Unlock()
	// Do not store negative metadata absence in the invocation repository cache.
	return observeHistoryProfile(r)
}

func observeHistoryProfile(read gitCoreRead) gitCoreRead {
	if read.err != nil || read.native != "" {
		return read
	}
	why, err := read.repo.NativeDiscoveryReason()
	if err != nil {
		return gitCoreRead{err: err}
	}
	if why != "" {
		return gitCoreRead{native: why}
	}
	why, err = read.repo.NativeHistoryReadReason()
	if err != nil {
		return gitCoreRead{err: err}
	}
	if why != "" {
		return gitCoreRead{native: why}
	}
	return read
}

// Ordinary reads historically honor native Git's repository-location and
// replacement profiles. go-git intentionally does not implement them. Keep that
// explicitly selected profile native rather than quietly hardening its answer.
func ordinaryGitCore(root string) gitCoreRead {
	for _, key := range append(historyRepoEnv, "GIT_REPLACE_REF_BASE", "GIT_GRAFT_FILE") {
		if _, present := os.LookupEnv(key); present {
			return gitCoreRead{native: "native Git environment selects repository/object/history state"}
		}
	}
	r := gitCoreFor(root)
	if r.err != nil || r.native != "" {
		return r
	}
	why, err := r.repo.NativeReplacementReadReason()
	if err != nil {
		return gitCoreRead{err: err}
	}
	if why != "" {
		return gitCoreRead{native: why}
	}
	common, err := gitcore.CommonDir(root)
	if err != nil {
		return gitCoreRead{err: err}
	}
	b, err := os.ReadFile(filepath.Join(common, "info", "grafts"))
	if err != nil && !os.IsNotExist(err) {
		return gitCoreRead{err: err}
	}
	if len(strings.TrimSpace(string(b))) != 0 {
		return gitCoreRead{native: "ordinary read honors grafted history"}
	}
	return r
}

// provenanceRepoTop keeps the immutable repository-selection profile while
// reusing the same canonical discovery and run lifetime as object reads.
func provenanceRepoTop(dir string) (string, error) {
	read := gitCoreFor(dir)
	if read.err != nil {
		return "", read.err
	}
	if read.native == "" {
		why, err := read.repo.NativeDiscoveryReason()
		if err != nil {
			return "", err
		}
		if why == "" && os.Getenv("GIT_CONFIG_COUNT") == "" && os.Getenv("GIT_CONFIG_PARAMETERS") == "" {
			return read.top, nil
		}
	}
	out, err := historyGit(dir, "rev-parse", "--show-toplevel").Output()
	return strings.TrimSpace(string(out)), err
}

// gitHeadOutput and gitShallowOutput preserve the existing native framing for
// callers while their supported read goes through the one canonical Git owner.
func gitHeadOutput(root string) ([]byte, error) {
	read := ordinaryGitCore(root)
	if read.err != nil {
		return nil, read.err
	}
	if read.native != "" {
		return exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	}
	id, err := read.repo.HeadID()
	if err != nil {
		return nil, err
	}
	return []byte(id + "\n"), nil
}
func gitShallowOutput(root string) ([]byte, error) {
	read := ordinaryGitCore(root)
	if read.err != nil {
		return nil, read.err
	}
	if read.native != "" {
		return exec.Command("git", "-C", root, "rev-parse", "--is-shallow-repository").Output()
	}
	// The canonical capability observation positively established absence of
	// shallow metadata. Present metadata always uses its original native parser.
	return []byte("false\n"), nil
}

// nativeRevisionSyntax names native selectors the selected commit resolver does
// not preserve. This is a pre-read capability classification, not a parser or
// fallback from a failed in-process resolution.
func nativeRevisionSyntax(rev string) bool {
	return rev == "" || strings.HasPrefix(rev, ":") || strings.Contains(rev, "@{") || strings.Contains(rev, "^{tree}") || strings.Contains(rev, "^{blob}")
}

// nativeCommitSelector restricts only the new commit-tree adapter. Generic show
// accepts tree/tag/reflog/index/expression selectors; exact object-kind lookup
// below handles full IDs and exact refs without adding a revision resolver.
func nativeCommitSelector(rev string) bool {
	if nativeRevisionSyntax(rev) || strings.ContainsAny(rev, "^~:@{}\\") {
		return true
	}
	if rev == "HEAD" || strings.HasPrefix(rev, "refs/") {
		return false
	}
	width := 40
	if gitcore.BuildObjectFormat() == "sha256" {
		width = 64
	}
	if len(rev) != width {
		return true
	}
	for _, c := range rev {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return true
		}
	}
	return false
}

// gitCoreForCommits gates the new commit-only readers through the same exact
// selector scope. Expressions/abbreviations remain native; successful kind
// observations distinguish native treeish rendering from actual storage errors.
func gitCoreForCommits(root string, revisions ...string) gitCoreRead {
	for _, rev := range revisions {
		if nativeCommitSelector(rev) || strings.HasPrefix(rev, "refs/") {
			return gitCoreRead{native: "revision selector outside qualified exact commit scope"}
		}
	}
	read := gitCoreFor(root)
	if read.err != nil || read.native != "" {
		return read
	}
	for _, rev := range revisions {
		why, err := read.repo.CommitTreeReadReason(rev)
		if err != nil {
			return gitCoreRead{err: err}
		}
		if why != "" {
			return gitCoreRead{native: why}
		}
	}
	return read
}

// provenanceRootPrefix is the uncached primitive shared with coverageRepo. The
// existing coverage memo keeps its original scope; new callers do not inherit it.
func provenanceRootPrefix(root string) (string, error) {
	read := gitCoreFor(root)
	if read.err != nil {
		return "", read.err
	}
	if read.native != "" {
		out, err := historyGit(root, "rev-parse", "--show-prefix").Output()
		return strings.TrimSpace(string(out)), err
	}
	return read.prefix, nil
}

// provenanceParents retains the original %P query for positively selected
// unsupported profiles (including every present shallow boundary). Its supported
// exact-commit read uses the existing canonical, qualified parent-order API.
func provenanceParents(root, rev string) ([]string, error) {
	read := gitCoreForCommits(root, rev)
	if read.err != nil {
		return nil, read.err
	}
	if read.native != "" {
		out, err := historyGit(root, "show", "-s", "--format=%P", "--end-of-options", rev).Output()
		return strings.Fields(string(out)), err
	}
	return read.repo.ParentHashes(rev)
}

// ordinaryCommitRead applies the qualified commit selector scope to the ordinary
// native profile. It observes capabilities only; unsupported calls keep their
// original argv at the caller, and actual reader errors never request a retry.
func ordinaryCommitRead(root, rev string) gitCoreRead {
	if nativeCommitSelector(rev) {
		return gitCoreRead{native: "revision selector outside qualified exact commit scope"}
	}
	read := ordinaryGitCore(root)
	if read.err != nil || read.native != "" {
		return read
	}
	// Resolve's qualified fully-named-ref semantics include native ref expansion;
	// unlike an exact object id, a name must not be prematurely treated as an OID.
	if strings.HasPrefix(rev, "refs/") {
		return read
	}
	why, err := read.repo.CommitTreeReadReason(rev)
	if err != nil {
		return gitCoreRead{err: err}
	}
	if why != "" {
		return gitCoreRead{native: why}
	}
	return read
}

// commitReadOutput preserves rev-parse's successful full-id framing around the
// one canonical Resolve API. false means a positively preselected native
// capability, never an in-process read failure. Native argv remains caller-owned.
func commitReadOutput(read gitCoreRead, rev string) ([]byte, error, bool) {
	if read.err != nil {
		return nil, read.err, true
	}
	if read.native != "" {
		return nil, nil, false
	}
	hash, err := read.repo.Resolve(rev)
	if err != nil {
		return nil, err, true
	}
	return []byte(hash.String() + "\n"), nil, true
}

// gitPrefixOutput is the ordinary-profile counterpart of provenanceRootPrefix.
// Discovery and environment policies stay with the existing read owner.
func gitPrefixOutput(root string) ([]byte, error) {
	read := ordinaryGitCore(root)
	if read.err != nil {
		return nil, read.err
	}
	if read.native != "" {
		return exec.Command("git", "-C", root, "rev-parse", "--show-prefix").Output()
	}
	return []byte(read.prefix + "\n"), nil
}
