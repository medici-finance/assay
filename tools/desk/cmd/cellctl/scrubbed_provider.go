package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/spf13/cobra"
)

var scrubbedProviderName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func (c *Cell) scrubbedTokenPath(name string) string {
	return filepath.Join(c.Home, ".config", "assay", "providers", name+".token")
}

// This is a separate custody path from host providers. Neither TOKEN_ENV nor a shell
// credential is consulted. No configurable path can reach another cell or the operator home.
func (c *Cell) scrubbedProviderCredential(name string) (Provider, error) {
	var p Provider
	if !scrubbedProviderName.MatchString(name) {
		return p, fmt.Errorf("invalid scrubbed provider name")
	}
	p.BaseURL, _ = c.providerValue(name, "BASE_URL")
	p.Model, _ = c.providerValue(name, "MODEL")
	u, err := url.Parse(p.BaseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return p, fmt.Errorf("scrubbed provider %s requires an HTTPS endpoint without credentials, query or fragment", name)
	}
	if p.Model == "" {
		return p, fmt.Errorf("scrubbed provider %s requires a model", name)
	}
	// Check every directory below the cell, including intermediate directories: a regular
	// token under a symlinked providers directory is still an operator credential import.
	dir := c.Dir
	for _, part := range []string{"home", ".config", "assay", "providers"} {
		dir = filepath.Join(dir, part)
		if err := privateCommsDir(dir); err != nil {
			return p, fmt.Errorf("provider custody: %w", err)
		}
	}
	path := c.scrubbedTokenPath(name)
	if err := privateCommsFile(path); err != nil {
		return p, fmt.Errorf("provider custody: %w", err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		return p, err
	}
	f, err := os.OpenInRoot(c.Home, filepath.Join(".config", "assay", "providers", name+".token"))
	if err != nil {
		return p, err
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		return p, fmt.Errorf("provider token changed during custody check")
	}
	raw, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil {
		return p, fmt.Errorf("could not read provider token")
	}
	token := strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
	if len(raw) > 16384 || token == "" || strings.ContainsAny(token, " \t\r\n\x00") {
		return p, fmt.Errorf("provider token must be one nonempty line, at most 16384 bytes")
	}
	p.TokenVal = token
	return p, nil
}

func scrubbedProviderEnv(env []string, p Provider, model string) []string {
	// The input is composed, but unset explicitly so future additions cannot resurrect
	// a native API key that takes precedence over the selected cell credential.
	env = envUnset(env, "ANTHROPIC_API_KEY")
	env = envSet(env, envAssayCockpit, "tmux")
	env = envSet(env, "ANTHROPIC_BASE_URL", p.BaseURL)
	env = envSet(env, "ANTHROPIC_AUTH_TOKEN", p.TokenVal)
	env = envSet(env, "ANTHROPIC_MODEL", model)
	for _, tier := range []string{"OPUS", "SONNET", "HAIKU"} {
		env = envSet(env, "ANTHROPIC_DEFAULT_"+tier+"_MODEL", p.Model)
	}
	return env
}

// With a provider, only the internal entrypoint's non-secret arguments reach tmux.
// The entrypoint reads the token AFTER env -i, then hands it to the harness through
// exec.Cmd.Env. It never appears in a pane command, process argv, plan or receipt.
func (c *Cell) scrubbedLaunchArgv(role, harness, model, session, wt, provider string) []string {
	if provider == "" {
		return harnessArgv(harness, role, model, session, wt)
	}
	root, ref := c.reenter()
	return []string{selfPath(), "--cells-root", root, "scrubbed-run", ref, role, harness, model, session, wt, provider}
}

func scrubbedRunCmd() *cobra.Command {
	return &cobra.Command{Use: "scrubbed-run", Hidden: true, DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := rawArgs(args)
			if len(a) != 7 {
				return fmt.Errorf("scrubbed-run is an internal entrypoint")
			}
			c := loadCell(a[0])
			role, harness, model, session, wt, provider := a[1], a[2], a[3], a[4], a[5], a[6]
			if c.Kind != "scrubbed" || harness != "claude" || !valueIn(role, c.Roles) || !valueIn(role, knownRoles) || filepath.Clean(wt) != filepath.Join(c.Dir, "worktrees", role) {
				return fmt.Errorf("invalid scrubbed-run context")
			}
			if role == "the-desk" {
				refuseOpusForTheDesk(model)
			}
			p, err := c.providerCredential(provider)
			if err != nil {
				return err
			}
			env := c.scrubbedComposeEnv(role, harness, session)
			// This entrypoint belongs to the same launched agent, not a new tool
			// invocation. Reuse the validated metadata from the outer env -i.
			env.Pairs = deskkit.PerfContextEnv(env.Pairs, perfrec.ResolveContext(perfrec.ContextInput{}), false)
			runForeground(harnessArgv(harness, role, model, session, wt), scrubbedProviderEnv(env.Pairs, p, model), wt)
			return nil
		}}
}
