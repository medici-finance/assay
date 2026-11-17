package gitcore

import (
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5/config"
	"os"
	"path/filepath"
	"strings"

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
	enabled, ok := gitConfigBool(ext.Option("worktreeconfig"))
	if !ok {
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
		// Git accepts forms this decoder rejects (for example a dotted section header or
		// a key on the header line), and those can set a remote URL. The parse error is
		// not echoed: it can quote the file's content.
		return nil, "worktree-scope config is in a form this reader does not parse", nil
	}
	if closeErr != nil {
		return nil, "", closeErr
	}
	return scoped, "", nil
}

// gitConfigBool reads the spellings git's own boolean parser and this package's
// origin snapshot owner both accept: true/yes/on/1 and false/no/off/0, in any case.
// ok is false for everything else, including the empty and no-value forms and the
// t/f spellings strconv.ParseBool also takes, so a caller refuses rather than guesses.
func gitConfigBool(v string) (value, ok bool) {
	switch strings.ToLower(v) {
	case "true", "yes", "on", "1":
		return true, true
	case "false", "no", "off", "0":
		return false, true
	}
	return false, false
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
// alone. It also refuses while command-priority config (OriginConfigInjected) or
// GIT_DIR, GIT_COMMON_DIR or GIT_WORK_TREE is set in the environment. Only the kind of declaration is reported, never a configured value.
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
	// Command-priority config and repository-location variables are read by native git
	// and by no file this reader opens; the origin snapshot owner refuses the same set.
	if OriginConfigInjected() {
		return "command-priority config (GIT_CONFIG_COUNT, GIT_CONFIG_PARAMETERS or GIT_CONFIG) is set", nil
	}
	for _, v := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE"} {
		if os.Getenv(v) != "" {
			return "a repository-location variable (GIT_DIR, GIT_COMMON_DIR or GIT_WORK_TREE) is set", nil
		}
	}
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
		return worktreeURLReason(name), nil
	}
	for _, sub := range scoped.Raw.Section("url").Subsections {
		if sub.HasOption("insteadOf") {
			return "worktree-scope config carries a url.<base>.insteadOf rule", nil
		}
	}
	return "", nil
}

func worktreeURLReason(name string) string {
	return "worktree-scope config sets remote." + name + ".url"
}

// PinnedRemoteURLs is ScopedRemoteURLs for a caller that also runs in a worktree whose
// transport is pinned per worktree. It refuses everything ScopedRemoteURLs refuses,
// except one shape of worktree-scope remote.<name>.url: an empty reset followed by
// exactly one URL, so git resolves that URL alone. For that shape it returns the
// repository-local list together with the pinned URL, the repository's insteadOf rules
// applied as for any fetch URL; pinned is "" when worktree scope sets no URL. Whether
// the pinned URL may stand for the repository-local one is the caller's decision.
func (r *Repo) PinnedRemoteURLs(name string) (local []string, pinned string, err error) {
	unreadable := func(err error) error {
		return fmt.Errorf("gitcore: remote-urls %s: config scope unreadable: %w", name, err)
	}
	refuse := func(why string) error {
		return fmt.Errorf("gitcore: remote-urls %s: %w: %s", name, ErrRemoteScopeUnmerged, why)
	}
	why, err := r.remoteScopeReason(name)
	if err != nil {
		return nil, "", unreadable(err)
	}
	if why == "" {
		local, err = r.RemoteURLs(name)
		return local, "", err
	}
	if why != worktreeURLReason(name) {
		return nil, "", refuse(why)
	}
	// remoteScopeReason stops at the first declaration, so the check after the URL one
	// is repeated here before the pinned shape is accepted.
	cfg, err := r.rawConfig()
	if err != nil {
		return nil, "", unreadable(err)
	}
	scoped, _, err := r.worktreeScope(cfg)
	if err != nil || scoped == nil {
		return nil, "", unreadable(fmt.Errorf("worktree scope changed during the read: %v", err))
	}
	for _, sub := range scoped.Raw.Section("url").Subsections {
		if sub.HasOption("insteadOf") {
			return nil, "", refuse("worktree-scope config carries a url.<base>.insteadOf rule")
		}
	}
	values := scoped.Raw.Section("remote").Subsection(name).OptionAll("url")
	if len(values) != 2 || values[0] != "" || values[1] == "" {
		return nil, "", refuse(why + " in a shape other than one reset and one URL")
	}
	local, err = r.RemoteURLs(name)
	if err != nil {
		return nil, "", err
	}
	// Append the worktree entries to the shared snapshot in git's scope order, and let
	// the shared decoder apply the reset and the insteadOf rules.
	cfg.Raw.Section("remote").Subsection(name).AddOption("url", "").AddOption("url", values[1])
	merged, err := decodeFetchConfig(cfg.Raw)
	if err != nil {
		return nil, "", unreadable(err)
	}
	remote, ok := merged.Remotes[name]
	if !ok || len(merged.Raw.Section("remote").Subsection(name).OptionAll("url")) != 1 || len(remote.URLs) == 0 {
		return nil, "", refuse(why + " that does not resolve to one URL")
	}
	return local, remote.URLs[0], nil
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
