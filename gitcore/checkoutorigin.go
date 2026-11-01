package gitcore

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/config"
	format "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// snapshotOrigin carries optional authority metadata, never transport config.
// This bounded profile uses the owner's existing codec and fetch rewrite fix:
// system/global files, common config and enabled worktree config, in scope order.
// Includes, injected config, ambiguous rewrites and multiple fetch URLs are
// unavailable rather than an invitation to select another scope/repository.
// No failure here can abort an otherwise readable local history checkout.
func (r *Repo) snapshotOrigin() string {
	raw, err := r.originRawConfig()
	if err != nil {
		return ""
	}
	// The upstream resolver stores one insteadOf per base and cannot preserve
	// multi-valued or equal-prefix choices. Withhold those profiles.
	prefixes := map[string]bool{}
	for _, sub := range raw.Section("url").Subsections {
		values := sub.OptionAll("insteadOf")
		if len(values) == 0 {
			continue
		} // pushInsteadOf-only: fixed by decodeFetchConfig
		if len(values) != 1 || values[0] == "" || prefixes[values[0]] {
			return ""
		}
		prefixes[values[0]] = true
	}
	cfg, err := decodeFetchConfig(raw)
	if err != nil {
		return ""
	}
	remote := cfg.Remotes["origin"]
	if remote == nil || len(cfg.Raw.Section("remote").Subsection("origin").OptionAll("url")) != 1 {
		return ""
	}
	origin := remote.URLs[0]
	if strings.TrimSpace(origin) != origin {
		return ""
	}
	if !strings.Contains(origin, "://") {
		ep, err := transport.NewEndpoint(origin)
		if err != nil || ep.Protocol != "ssh" || ep.Host == "" {
			return ""
		}
		ep.User, ep.Password = "", ""
		origin = ep.String()
	}
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	switch u.Scheme {
	case "http", "https", "ssh":
	default:
		return ""
	}
	u.User, u.RawQuery, u.Fragment = nil, "", ""
	u.ForceQuery = false
	origin = u.String()

	// A copied effective URL must not be rewritten a second time by ambient
	// rules when the native approval consumer opens the disposable checkout.
	for _, rule := range cfg.URLs {
		if rule.ApplyInsteadOf(origin) != origin {
			return ""
		}
	}
	return origin
}

// originRawConfig merges observations with the selected config codec; it does
// not execute includes, commands, helpers or native Git, nor copy configuration.
func (r *Repo) originRawConfig() (*format.Config, error) {
	if OriginConfigInjected() {
		return nil, fmt.Errorf("injected config is outside snapshot profile")
	}
	for _, name := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE"} {
		if os.Getenv(name) != "" {
			return nil, fmt.Errorf("injected repository is outside snapshot profile")
		}
	}
	raw := format.New()
	read := func(path string, optional bool) error {
		if path == "" {
			return fmt.Errorf("unknown config path")
		}
		content, err := os.ReadFile(path)
		if optional && os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := format.NewDecoder(bytes.NewReader(content)).Decode(raw); err != nil {
			return err
		}
		if raw.HasSection("include") || raw.HasSection("includeIf") {
			return fmt.Errorf("includes are outside snapshot profile")
		}
		return nil
	}
	noSystem := strings.ToLower(os.Getenv("GIT_CONFIG_NOSYSTEM"))
	switch noSystem {
	case "", "0", "false", "no", "off":
		paths, err := config.Paths(config.SystemScope)
		if err != nil {
			return nil, err
		}
		if path, ok := os.LookupEnv("GIT_CONFIG_SYSTEM"); ok {
			paths = []string{path}
		}
		for _, path := range paths {
			if err := read(path, true); err != nil {
				return nil, err
			}
		}
	case "1", "true", "yes", "on":
	default:
		return nil, fmt.Errorf("unknown system config scope")
	}
	if path, ok := os.LookupEnv("GIT_CONFIG_GLOBAL"); ok {
		if err := read(path, true); err != nil {
			return nil, err
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg == "" {
			xdg = filepath.Join(home, ".config")
		}
		for _, path := range []string{filepath.Join(xdg, "git", "config"), filepath.Join(home, ".gitconfig")} {
			if err := read(path, true); err != nil {
				return nil, err
			}
		}
	}
	common, err := CommonDir(r.dir)
	if err != nil {
		return nil, err
	}
	if err := read(filepath.Join(common, "config"), false); err != nil {
		return nil, err
	}
	// The general storer's worktreeConfig shim intentionally removes the
	// extension; read its declaration from the raw source before choosing a file.
	switch strings.ToLower(raw.Section("extensions").Option("worktreeConfig")) {
	case "true", "yes", "on", "1":
		admin, _, err := resolveGitDir(r.dir)
		if err != nil {
			return nil, err
		}
		if err := read(filepath.Join(admin, "config.worktree"), true); err != nil {
			return nil, err
		}
	case "":
		if raw.Section("extensions").HasOption("worktreeConfig") {
			return nil, fmt.Errorf("ambiguous worktree config scope")
		}
	case "false", "no", "off", "0":
	default:
		return nil, fmt.Errorf("unknown worktree config scope")
	}
	return raw, nil
}

// OriginConfigInjected identifies command-priority config outside the origin
// snapshot profile. Native authority consumers must also refuse it: a local
// URL reset cannot suppress a later command-priority reset plus foreign URL.
func OriginConfigInjected() bool {
	count := os.Getenv("GIT_CONFIG_COUNT")
	return (count != "" && count != "0") || os.Getenv("GIT_CONFIG_PARAMETERS") != "" || os.Getenv("GIT_CONFIG") != ""
}

// OriginAuthority returns only the bounded owner's proven fetch authority.
// Unlike a native remote lookup, an absent URL cannot fall back to the name
// "origin" and be reinterpreted by an ambient rewrite. Unavailable metadata
// remains an error for authority consumers, not for ordinary history snapshots.
func OriginAuthority(root string) (string, error) {
	repo, err := Open(root)
	if err != nil {
		return "", err
	}
	origin := repo.snapshotOrigin()
	if origin == "" {
		return "", fmt.Errorf("repository origin authority is unavailable")
	}
	return origin, nil
}
