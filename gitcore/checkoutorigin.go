package gitcore

import (
	"bytes"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/config"
	format "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// OriginUnavailable names why the bounded owner withholds repository authority.
// Class is always one of this package's fixed literals: it never carries a URL,
// credential, path or decoder text, so consumers may print it verbatim.
type OriginUnavailable struct{ Class string }

func (e *OriginUnavailable) Error() string {
	return "repository origin authority is unavailable: " + e.Class
}

func unavailable(class string) error { return &OriginUnavailable{Class: class} }

// snapshotOrigin carries optional authority metadata, never transport config.
// No failure here can abort an otherwise readable local history checkout.
func (r *Repo) snapshotOrigin() string {
	origin, err := r.originAuthority()
	if err != nil {
		return ""
	}
	return origin
}

// originAuthority is the bounded owner of a checkout's fetch authority. It uses
// the owner's existing codec and fetch rewrite fix over system/global files,
// common config and enabled worktree config, in scope order. Includes are
// evaluated by Git's own conditions (originincludes.go) and never contribute
// values; injected config, ambiguous rewrites and multiple fetch URLs are
// unavailable rather than an invitation to select another scope/repository.
func (r *Repo) originAuthority() (string, error) {
	raw, err := r.originRawConfig()
	if err != nil {
		return "", err
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
			return "", unavailable("url rewrite is multi-valued or ambiguous")
		}
		prefixes[values[0]] = true
	}
	cfg, err := decodeFetchConfig(raw)
	if err != nil {
		return "", unavailable("config is malformed")
	}
	remote := cfg.Remotes["origin"]
	if remote == nil || len(cfg.Raw.Section("remote").Subsection("origin").OptionAll("url")) != 1 {
		return "", unavailable("origin has no single fetch URL")
	}
	origin, ok := sanitizeOrigin(remote.URLs[0])
	if !ok {
		return "", unavailable("origin URL form is unsupported")
	}
	// A copied effective URL must not be rewritten to another repository when a
	// native consumer opens the disposable checkout. A rule that only adds what
	// sanitizing removes (for example a credential on the same host) is idempotent.
	for _, rule := range cfg.URLs {
		if again, ok := sanitizeOrigin(rule.ApplyInsteadOf(origin)); !ok || again != origin {
			return "", unavailable("an ambient rewrite changes the origin")
		}
	}
	return origin, nil
}

// sanitizeOrigin reduces a fetch URL to its repository identity: scheme, host,
// port and path. Credentials, query and fragment never leave this function.
func sanitizeOrigin(origin string) (string, bool) {
	if strings.TrimSpace(origin) != origin {
		return "", false
	}
	if !strings.Contains(origin, "://") {
		// Support Git's ordinary [user@]host:path form only. The endpoint
		// parser also accepts multi-colon/second-@ spellings that native Git
		// interprets as a different host; they cannot supply proven authority.
		colon := strings.IndexByte(origin, ':')
		if strings.Count(origin, ":") != 1 || strings.Count(origin, "@") > 1 || colon <= 0 || strings.ContainsAny(origin[:colon], "/\\") {
			return "", false
		}
		ep, err := transport.NewEndpoint(origin)
		if err != nil || ep.Protocol != "ssh" || ep.Host == "" {
			return "", false
		}
		ep.User, ep.Password = "", ""
		origin = ep.String()
	}
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return "", false
	}
	switch u.Scheme {
	case "http", "https", "ssh":
	default:
		return "", false
	}
	u.User, u.RawQuery, u.Fragment = nil, "", ""
	u.ForceQuery = false
	return u.String(), true
}

// originRawConfig merges observations with the selected config codec. Include
// observations never contribute values. No command/helper/native Git is
// executed and no credential configuration is copied.
func (r *Repo) originRawConfig() (*format.Config, error) {
	if OriginConfigInjected() {
		return nil, unavailable("injected configuration is outside the snapshot profile")
	}
	for _, name := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE"} {
		if os.Getenv(name) != "" {
			return nil, unavailable("injected repository location is outside the snapshot profile")
		}
	}
	raw := format.New()
	// read returns the file's own parse (nil when an optional file is absent).
	read := func(path string, optional bool) (*format.Config, error) {
		if path == "" {
			return nil, unavailable("config path is unknown")
		}
		content, err := os.ReadFile(path)
		if optional && os.IsNotExist(err) {
			return nil, nil
		}
		if err != nil {
			return nil, unavailable("config file is unreadable")
		}
		current := format.New()
		if err := format.NewDecoder(bytes.NewReader(content)).Decode(current); err != nil {
			return nil, unavailable("config file is malformed")
		}
		if err := r.checkOriginIncludes(current, path); err != nil {
			return nil, err
		}
		// Included values never enter the snapshot: a matching include that
		// carries authority sections was refused above, so ignoring the rest
		// preserves authority and avoids importing credential configuration.
		current.RemoveSection("include").RemoveSection("includeIf")
		var encoded bytes.Buffer
		if err := format.NewEncoder(&encoded).Encode(current); err != nil {
			return nil, unavailable("config file is malformed")
		}
		if err := format.NewDecoder(bytes.NewReader(encoded.Bytes())).Decode(raw); err != nil {
			return nil, unavailable("config file is malformed")
		}
		return current, nil
	}
	noSystem := strings.ToLower(os.Getenv("GIT_CONFIG_NOSYSTEM"))
	switch noSystem {
	case "", "0", "false", "no", "off":
		paths, err := config.Paths(config.SystemScope)
		if err != nil {
			return nil, unavailable("system config scope is unknown")
		}
		if path, ok := os.LookupEnv("GIT_CONFIG_SYSTEM"); ok {
			paths = []string{path}
		}
		for _, path := range paths {
			if _, err := read(path, true); err != nil {
				return nil, err
			}
		}
	case "1", "true", "yes", "on":
	default:
		return nil, unavailable("system config scope is unknown")
	}
	if path, ok := os.LookupEnv("GIT_CONFIG_GLOBAL"); ok {
		if _, err := read(path, true); err != nil {
			return nil, err
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, unavailable("global config scope is unknown")
		}
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg == "" {
			xdg = filepath.Join(home, ".config")
		}
		for _, path := range []string{filepath.Join(xdg, "git", "config"), filepath.Join(home, ".gitconfig")} {
			if _, err := read(path, true); err != nil {
				return nil, err
			}
		}
	}
	common, err := CommonDir(r.dir)
	if err != nil {
		return nil, unavailable("repository layout is unreadable")
	}
	repository, err := read(filepath.Join(common, "config"), false)
	if err != nil {
		return nil, err
	}
	// Git honours repository-format extensions only from the repository's own
	// config file (never system, global or an include), so the declaration is read
	// from that file's parse. The general storer's worktreeConfig shim removes it.
	value, declared := "", false
	for _, section := range repository.Sections {
		if section.IsName("extensions") {
			for _, option := range section.Options {
				if option.IsKey("worktreeConfig") {
					value, declared = option.Value, true
				}
			}
		}
	}
	switch strings.ToLower(value) {
	case "true", "yes", "on", "1":
		admin, _, err := resolveGitDir(r.dir)
		if err != nil {
			return nil, unavailable("repository layout is unreadable")
		}
		if _, err := read(filepath.Join(admin, "config.worktree"), true); err != nil {
			return nil, err
		}
	case "":
		if declared {
			return nil, unavailable("worktree config scope is ambiguous")
		}
	case "false", "no", "off", "0":
	default:
		return nil, unavailable("worktree config scope is unknown")
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
// remains an error for authority consumers, not for ordinary history snapshots;
// every error is an *OriginUnavailable whose class is safe to print.
func OriginAuthority(root string) (string, error) {
	repo, err := Open(root)
	if err != nil {
		return "", unavailable("repository is unreadable")
	}
	origin, err := repo.originAuthority()
	var class *OriginUnavailable
	if err != nil && !errors.As(err, &class) {
		return "", unavailable("config is unreadable")
	}
	return origin, err
}
