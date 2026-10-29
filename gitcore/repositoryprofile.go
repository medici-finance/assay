package gitcore

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// NativeRepositoryReadReason observes local declarations before Open can reject
// an unsupported extension. It grants no retry after a read failure: unreadable
// or malformed configuration still returns its original error.
func NativeRepositoryReadReason(dir string) (string, error) {
	cfg, err := repositoryReadConfig(dir)
	if err != nil {
		return "", err
	}
	return nativeRepositoryReadReason(cfg), nil
}

func nativeRepositoryReadReason(cfg *config.Config) string {
	if cfg.Raw.Section("core").HasOption("repositoryformatversion") && cfg.Raw.Section("core").Option("repositoryformatversion") == "" {
		return "explicit empty repository format uses native numeric configuration grammar"
	}
	// Pinned v5.19.3 repository_extensions.go accepts format versions 0/1,
	// builtin noop/noop-v1, and this owner's existing worktreeConfig shim.
	// Do not strip any additional extension from the storer to force it open.
	switch cfg.Core.RepositoryFormatVersion {
	case "", "0", "1":
	default:
		return "declared repository format is outside the selected reader's support"
	}
	for _, option := range cfg.Raw.Section("extensions").Options {
		name := strings.ToLower(option.Key)
		switch name {
		case "noop", "worktreeconfig":
			continue
		case "noop-v1":
			if cfg.Core.RepositoryFormatVersion == "1" {
				continue
			}
		}
		return fmt.Sprintf("repository extension %q requires native Git", name)
	}
	return ""
}

// NativeReplacementReadReason observes names of every reference type. Refs() is
// intentionally a hash-reference API and cannot establish absence of symbolic
// replacement references. No replacement is resolved or applied by this owner.
func (r *Repo) NativeReplacementReadReason() (string, error) {
	iter, err := r.repo.References()
	if err != nil {
		return "", err
	}
	defer iter.Close()
	reason := ""
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if strings.HasPrefix(string(ref.Name()), "refs/replace/") {
			reason = "replacement reference namespace requires native history semantics"
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return reason, nil
}
