package gitcore

import (
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5/config"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-git/go-git/v5/plumbing"
)

// LocalRefHash reads a direct hash reference without dereferencing a symbolic
// alias; a symbolic reference is refused. The underlying reader can report an
// unopenable loose ref as missing, or malformed text as a zero hash. Callers
// retiring recovery state must validate hashes and independently prove absence.
func (r *Repo) LocalRefHash(name string) (string, bool, error) {
	ref, err := r.repo.Reference(plumbing.ReferenceName(name), false)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if ref.Type() != plumbing.HashReference {
		return "", true, fmt.Errorf("local ref %s is symbolic", name)
	}
	return ref.Hash().String(), true, nil
}

// HeadID reads HEAD's referenced object id without decoding or peeling its object,
// matching rev-parse HEAD even when the named commit has unusual raw headers.
func (r *Repo) HeadID() (string, error) {
	ref, err := r.repo.Head()
	if err != nil {
		return "", err
	}
	return ref.Hash().String(), nil
}

// NativeDiscoveryReason reports repository declarations whose effective worktree
// policy is not implemented by Toplevel. It does not resolve or apply overrides;
// callers keep that operation in native Git rather than adding another resolver.
func (r *Repo) NativeDiscoveryReason() (string, error) {
	store := r.repo.Storer
	if wrapped, ok := store.(extensionTolerantStorer); ok {
		store = wrapped.Storer
	}
	cfg, err := store.Config()
	if err != nil {
		return "", err
	}
	if cfg.Raw.HasSection("include") || cfg.Raw.HasSection("includeIf") {
		return "native discovery config includes are not merged by the selected reader", nil
	}
	if cfg.Core.IsBare {
		return "repository declares core.bare", nil
	}
	if cfg.Core.Worktree != "" {
		return "repository declares core.worktree", nil
	}
	ext := cfg.Raw.Section("extensions")
	if !ext.HasOption("worktreeconfig") {
		return "", nil
	}
	enabled, boolErr := strconv.ParseBool(ext.Option("worktreeconfig"))
	if boolErr != nil {
		return "native worktree config activation grammar", nil
	}
	if !enabled {
		return "", nil
	}
	gitDir, _, err := resolveGitDir(r.dir)
	if err != nil {
		return "", err
	}
	f, err := os.Open(filepath.Join(gitDir, "config.worktree"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	scoped, readErr := config.ReadConfig(f)
	closeErr := f.Close()
	if readErr != nil {
		return "", readErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	core := scoped.Raw.Section("core")
	if core.HasOption("bare") || core.HasOption("worktree") {
		return "worktree-scoped discovery declaration requires native effective config", nil
	}
	return "", nil
}
