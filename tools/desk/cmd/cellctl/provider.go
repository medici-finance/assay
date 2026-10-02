package main

import (
	"fmt"
	"strings"
)

// providerVar is the cell.env variable name for a provider's BASE_URL/TOKEN_ENV/MODEL, e.g.
// providerVar("zai", "BASE_URL") → CELL_PROVIDER_ZAI_BASE_URL. The name is upper-cased and its
// `-`s become `_`s so a `--provider z.ai`-style typo is refused by a MISSING variable rather
// than silently reading someone else's.
func providerVar(name, suffix string) string {
	up := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	return "CELL_PROVIDER_" + up + "_" + suffix
}

// providerPreset is the compiled default for a built-in provider, or "" for a name with none.
// A preset carries NO CREDENTIAL of any kind — only the NAME of the env var the operator
// exports.
func providerPreset(name, suffix string) string {
	switch name + ":" + suffix {
	case "kimi:BASE_URL":
		return "https://api.kimi.com/coding"
	case "kimi:TOKEN_ENV":
		return "KIMI_API_KEY"
	case "kimi:MODEL":
		return "k3[1m]"
	case "glm:BASE_URL":
		return "https://api.z.ai/api/anthropic"
	case "glm:TOKEN_ENV":
		return "ZAI_API_KEY"
	case "glm:MODEL":
		return "glm-5.3[1m]"
	}
	return ""
}

// providerValue is the EFFECTIVE value (cell.env/env line, else the preset, else empty) and
// where it came from (`cell.env`, `preset`, `unset`).
func (c *Cell) providerValue(name, suffix string) (string, string) {
	if v := c.Env.Get(providerVar(name, suffix)); v != "" {
		return v, "cell.env"
	}
	if v := providerPreset(name, suffix); v != "" {
		return v, "preset"
	}
	return "", "unset"
}

// Provider is the resolved endpoint/credential triple a non-Anthropic model needs. The TOKEN
// VALUE is never stored in cell.env — only the NAME of an env var the OPERATOR's shell is
// expected to carry — so a leaked cell.env never leaks a credential itself.
type Provider struct {
	BaseURL  string
	TokenEnv string
	TokenVal string
	Model    string
}

func (c *Cell) resolveProvider(name string) Provider {
	p, err := c.providerCredential(name)
	if err != nil {
		die("%s", err)
	}
	return p
}

// providerCredential is resolveProvider without the exit: the same three refusals, returned as
// an error so a preflight loop (`up`/`check` under a model policy) can report every role before
// deciding, instead of dying on the first.
func (c *Cell) providerCredential(name string) (Provider, error) {
	var p Provider
	baseVar, tokenVar := providerVar(name, "BASE_URL"), providerVar(name, "TOKEN_ENV")
	p.BaseURL, _ = c.providerValue(name, "BASE_URL")
	p.TokenEnv, _ = c.providerValue(name, "TOKEN_ENV")
	p.Model, _ = c.providerValue(name, "MODEL")
	if p.BaseURL == "" {
		return p, fmt.Errorf("cell.env: %s is not set — declare it for provider '%s' (e.g. %s=https://api.%s.example)", baseVar, name, baseVar, name)
	}
	if p.TokenEnv == "" {
		up := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
		return p, fmt.Errorf("cell.env: %s is not set — declare it for provider '%s' as the NAME of an env var carrying the token (never the token itself), e.g. %s=%s_API_KEY", tokenVar, name, tokenVar, up)
	}
	p.TokenVal = c.Env.Get(p.TokenEnv)
	if p.TokenVal == "" {
		return p, fmt.Errorf("provider '%s': $%s (named by %s) is not set in this shell — export it before running cellctl, cellctl does not manage credential values", name, p.TokenEnv, tokenVar)
	}
	return p, nil
}
