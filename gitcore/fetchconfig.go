package gitcore

import (
	"bytes"
	"fmt"

	"github.com/go-git/go-git/v5"

	"github.com/go-git/go-git/v5/config"
	format "github.com/go-git/go-git/v5/plumbing/format/config"
)

// fetchConfig works around go-git v5.19.3 interpreting a URL subsection without
// insteadOf (for example pushInsteadOf-only) as an empty fetch rewrite prefix.
// Remove those subsections from this in-memory snapshot, then let the same upstream
// codec and longest-prefix resolver rebuild the URLs from their raw values. Never
// undo a rewrite by trimming its output or mutate the repository's config on disk.
func (r *Repo) fetchConfig() (*config.Config, error) {
	cfg, err := r.repo.Config()
	if err != nil {
		return cfg, err
	}
	return decodeFetchConfig(cfg.Raw)
}

// decodeFetchConfig is the shared codec/rewrite workaround for local reads and
// the closure metadata scope adapter. The caller owns this in-memory raw snapshot.
func decodeFetchConfig(rawConfig *format.Config) (*config.Config, error) {
	if rawConfig.HasSection("url") {
		urls := rawConfig.Section("url")
		kept := make(format.Subsections, 0, len(urls.Subsections))
		for _, sub := range urls.Subsections {
			if sub.HasOption("insteadOf") {
				kept = append(kept, sub)
			}
		}
		urls.Subsections = kept
	}
	// Git 2.46+ empty URL entries reset the accumulated list, including values
	// inherited from an earlier scope. Push-only URLs remain a separate list.
	for _, remote := range rawConfig.Section("remote").Subsections {
		for _, key := range []string{"url", "pushurl"} {
			values := remote.OptionAll(key)
			lastReset := -1
			for i, value := range values {
				if value == "" {
					lastReset = i
				}
			}
			values = values[lastReset+1:]
			remote.RemoveOption(key)
			if len(values) > 0 {
				remote.SetOption(key, values...)
			}
		}
	}
	var raw bytes.Buffer
	if err := format.NewEncoder(&raw).Encode(rawConfig); err != nil {
		return nil, err
	}
	corrected := config.NewConfig()
	if err := corrected.Unmarshal(raw.Bytes()); err != nil {
		return nil, err
	}
	return corrected, nil
}

// RemoteURL reads the first URL through the shared adapter for a repository opened
// by the caller. It does not reopen the repository or select another config source.
func RemoteURL(repo *git.Repository, name string) (string, error) {
	if repo == nil {
		return "", fmt.Errorf("gitcore: remote-url %s: no repository", name)
	}
	return (&Repo{repo: repo}).RemoteURL(name)
}
