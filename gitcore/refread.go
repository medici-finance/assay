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

// rawConfig is the repository's own config file as written, read past
// extensionTolerantStorer so extensions.worktreeConfig stays visible.
func (r *Repo) rawConfig() (*config.Config, error) {
	store := r.repo.Storer
	if wrapped, ok := store.(extensionTolerantStorer); ok {
		store = wrapped.Storer
	}
	return store.Config()
}

// worktreeScope returns the worktree-scoped config git merges over cfg: nil when
// extensions.worktreeConfig is off or this worktree has no config.worktree, and a
// reason instead when the activation value is outside the grammar read here.
func (r *Repo) worktreeScope(cfg *config.Config) (*config.Config, string, error) {
	ext := cfg.Raw.Section("extensions")
	if !ext.HasOption("worktreeconfig") {
		return nil, "", nil
	}
	enabled, boolErr := strconv.ParseBool(ext.Option("worktreeconfig"))
	if boolErr != nil {
		return nil, "native worktree config activation grammar", nil
	}
	if !enabled {
		return nil, "", nil
	}
	gitDir, _, err := resolveGitDir(r.dir)
	if err != nil {
		return nil, "", err
	}
	f, err := os.Open(filepath.Join(gitDir, "config.worktree"))
	if os.IsNotExist(err) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	scoped, readErr := config.ReadConfig(f)
	closeErr := f.Close()
	if readErr != nil {
		return nil, "", readErr
	}
	if closeErr != nil {
		return nil, "", closeErr
	}
	return scoped, "", nil
}

// ErrRemoteScopeUnmerged marks a remote URL read refused because config that
// RemoteURLs does not merge could change the URL list native git resolves.
var ErrRemoteScopeUnmerged = errors.New("remote URL could be changed by config the repository-local reader does not merge")

// ScopedRemoteURLs is RemoteURLs for a caller that decides a repository from the
// result. It fails closed (ErrRemoteScopeUnmerged) when an include/includeIf
// directive is in the repository config, or worktree-scope config (enabled by
// extensions.worktreeConfig) carries an include/includeIf directive, a
// remote.<name>.url entry or a url.<base>.insteadOf rule. Any of those can change
// the fetch URL list native git resolves while RemoteURLs reads the shared file
// alone. Only the kind of declaration is reported, never a configured value.
// Worktree-scope pushurl, identity and credential keys do not touch that list.
func (r *Repo) ScopedRemoteURLs(name string) ([]string, error) {
	why, err := r.remoteScopeReason(name)
	if err != nil {
		return nil, fmt.Errorf("gitcore: remote-urls %s: config scope unreadable: %w", name, err)
	}
	if why != "" {
		return nil, fmt.Errorf("gitcore: remote-urls %s: %w: %s", name, ErrRemoteScopeUnmerged, why)
	}
	return r.RemoteURLs(name)
}

func (r *Repo) remoteScopeReason(name string) (string, error) {
	cfg, err := r.rawConfig()
	if err != nil {
		return "", err
	}
	if cfg.Raw.HasSection("include") || cfg.Raw.HasSection("includeIf") {
		return "repository config carries include or includeIf directives", nil
	}
	scoped, why, err := r.worktreeScope(cfg)
	if err != nil || why != "" || scoped == nil {
		return why, err
	}
	if scoped.Raw.HasSection("include") || scoped.Raw.HasSection("includeIf") {
		return "worktree-scope config carries include or includeIf directives", nil
	}
	if remotes := scoped.Raw.Section("remote"); remotes.HasSubsection(name) && remotes.Subsection(name).HasOption("url") {
		return "worktree-scope config sets remote." + name + ".url", nil
	}
	for _, sub := range scoped.Raw.Section("url").Subsections {
		if sub.HasOption("insteadOf") {
			return "worktree-scope config carries a url.<base>.insteadOf rule", nil
		}
	}
	return "", nil
}

// NativeDiscoveryReason reports repository declarations whose effective worktree
// policy is not implemented by Toplevel. It does not resolve or apply overrides;
// callers keep that operation in native Git rather than adding another resolver.
func (r *Repo) NativeDiscoveryReason() (string, error) {
	cfg, err := r.rawConfig()
	if err != nil {
		return "", err
	}
	if why := nativeRepositoryReadReason(cfg); why != "" {
		return why, nil
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
	scoped, why, err := r.worktreeScope(cfg)
	if err != nil || why != "" || scoped == nil {
		return why, err
	}
	core := scoped.Raw.Section("core")
	if core.HasOption("bare") || core.HasOption("worktree") {
		return "worktree-scoped discovery declaration requires native effective config", nil
	}
	return "", nil
}
