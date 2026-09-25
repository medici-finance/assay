package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed providers.json
var defaultProviders []byte

// ProviderDesk selects a provider tier and, optionally, a desk-specific effort.
// The tier owns the model ID, so updating it changes all desks inheriting it.
type ProviderDesk struct {
	Tier   string `json:"tier"`
	Effort string `json:"effort,omitempty"`
}

type catalogProvider struct {
	Harness string                  `json:"harness"`
	Tiers   json.RawMessage         `json:"tiers"`
	Desks   map[string]ProviderDesk `json:"desks"`
}

type providerCatalog struct {
	Schema          int                        `json:"schema"`
	DefaultProvider string                     `json:"default_provider"`
	Deny            []string                   `json:"deny"`
	Providers       map[string]catalogProvider `json:"providers"`
	Roles           map[string]string          `json:"roles"`
}

func parseProviderCatalog(raw []byte, source, selectedProvider string) (*ModelPolicy, error) {
	var catalog providerCatalog
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&catalog); err != nil {
		return nil, policyFail("invalid provider defaults %s: %v", source, err)
	}
	if !json.Valid(raw) {
		return nil, policyFail("invalid provider defaults %s", source)
	}
	if _, ok := catalog.Providers[catalog.DefaultProvider]; !ok {
		return nil, policyFail("unknown default_provider %q", catalog.DefaultProvider)
	}
	if selectedProvider == "" {
		selectedProvider = catalog.DefaultProvider
	}
	if _, ok := catalog.Providers[selectedProvider]; !ok {
		return nil, policyFail("unknown provider %q", selectedProvider)
	}
	providers := map[string]any{}
	roles := map[string]any{}
	for name, p := range catalog.Providers {
		providers[name] = map[string]any{"harness": p.Harness, "tiers": p.Tiers}
		if len(p.Desks) != len(policyKnownRoles) {
			return nil, policyFail("provider %s must configure all five desks", name)
		}
		for _, role := range policyKnownRoles {
			desk, ok := p.Desks[role]
			if !ok || !contains(policyTierNames, desk.Tier) {
				return nil, policyFail("invalid desk %s/%s tier", name, role)
			}
		}
	}
	for role, provider := range catalog.Roles {
		if !contains(policyKnownRoles, role) {
			return nil, policyFail("unknown role %s", role)
		}
		if _, ok := catalog.Providers[provider]; !ok {
			return nil, policyFail("unknown provider %q for %s", provider, role)
		}
	}
	for _, role := range policyKnownRoles {
		provider := selectedProvider
		if override := catalog.Roles[role]; override != "" {
			provider = override
		}
		roles[role] = map[string]string{"provider": provider, "tier": catalog.Providers[provider].Desks[role].Tier}
	}
	compiled, err := json.Marshal(map[string]any{"schema": catalog.Schema, "deny": catalog.Deny, "providers": providers, "roles": roles})
	if err != nil {
		return nil, err
	}
	policy, err := parseModelPolicy(compiled, source)
	if err != nil {
		return nil, err
	}
	policy.ProviderDesks = map[string]map[string]ProviderDesk{}
	for name, p := range catalog.Providers {
		for role, desk := range p.Desks {
			tier := policy.Providers[name].Tiers[desk.Tier]
			if desk.Effort != "" && !contains(tier.SupportedEfforts, desk.Effort) {
				return nil, policyFail("unsupported desk effort for %s/%s", name, role)
			}
		}
		policy.ProviderDesks[name] = p.Desks
	}
	// Validate all provider/desk combinations, not just today's selected provider.
	for name := range catalog.Providers {
		for _, role := range policyKnownRoles {
			if _, err := policy.Resolve(role, name, "", ""); err != nil {
				return nil, err
			}
		}
	}
	sum := sha256.Sum256(raw)
	policy.SHA256 = hex.EncodeToString(sum[:])
	return policy, nil
}

// mergeProviderOverrides merges objects by key. Lists/scalars replace; null is never
// deletion. Shared deny patterns accumulate so a cell cannot remove a shared prohibition.
func mergeProviderOverrides(base, override map[string]any) error {
	for key, value := range override {
		if value == nil {
			return policyFail("provider override %s cannot be null", key)
		}
		if child, ok := value.(map[string]any); ok {
			parent, ok := base[key].(map[string]any)
			if !ok {
				parent = map[string]any{}
			}
			if err := mergeProviderOverrides(parent, child); err != nil {
				return err
			}
			base[key] = parent
		} else if key == "deny" {
			values, ok := value.([]any)
			if !ok {
				return policyFail("deny override must be an array")
			}
			prior, _ := base[key].([]any)
			base[key] = append(prior, values...)
		} else {
			base[key] = value
		}
	}
	return nil
}

func configuredPath(e *Env, key, fallback, root string) (string, bool) {
	path := e.Get(key)
	explicit := path != ""
	if !explicit {
		path = fallback
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return path, explicit
}

// cellModelPolicy is shared by launch and inspection. A complete legacy policy wins;
// otherwise all cells read the shared catalog on every invocation and merge only their
// local exceptions. No catalog means legacy behavior, not a silent synthesized policy.
func (c *Cell) cellModelPolicy() (*ModelPolicy, string, error) {
	if path := c.Env.Get("CELL_MODEL_POLICY"); path != "" {
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.Dir, path)
		}
		m, err := loadModelPolicy(path)
		return m, path, err
	}
	// Host defaults must never bleed into isolated container/scrubbed environments.
	if c.Kind == "container" || c.Kind == "scrubbed" {
		if c.Env.Get("CELL_PROVIDER_DEFAULTS") != "" || c.Env.Get("CELL_PROVIDER_OVERRIDES") != "" {
			return nil, "", policyFail("provider defaults require a house or k8s cell")
		}
		return nil, "", nil
	}
	path, explicit := configuredPath(c.Env, "CELL_PROVIDER_DEFAULTS", "providers.json", cellsRoot(c.Env))
	local, localExplicit := configuredPath(c.Env, "CELL_PROVIDER_OVERRIDES", "providers.json", c.Dir)
	raw, err := readPolicySource(path)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			_, localErr := os.Stat(local)
			if os.IsNotExist(localErr) && !localExplicit {
				return nil, "", nil
			}
		}
		return nil, "", policyFail("cannot read provider defaults %s: %v", path, err)
	}
	selected := c.Env.Get("CELL_PROVIDER")
	if selected == "" && c.Harness == "codex" {
		selected = "codex"
	}
	// A broken shared file is an error even if a local overlay would mask the defect.
	policy, err := parseProviderCatalog(raw, path, "")
	if err != nil {
		return nil, "", err
	}
	source := path
	overlay, err := readPolicySource(local)
	if err == nil {
		var base, patch map[string]any
		if err := json.Unmarshal(raw, &base); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(overlay, &patch); err != nil || patch == nil {
			return nil, "", policyFail("invalid provider overrides %s", local)
		}
		if err := mergeProviderOverrides(base, patch); err != nil {
			return nil, "", err
		}
		raw, err = json.Marshal(base)
		if err != nil {
			return nil, "", err
		}
		source += " + " + local
	} else if !os.IsNotExist(err) || localExplicit {
		return nil, "", policyFail("cannot read provider overrides %s: %v", local, err)
	}
	policy, err = parseProviderCatalog(raw, source, selected)
	return policy, source, err
}

func cmdProviders(args []string) {
	if len(args) != 1 || args[0] != "init" {
		die("usage: cellctl providers init (creates CELLS_ROOT/providers.json without overwriting)")
	}
	path := filepath.Join(cellsRoot(newEnvFromProcess()), "providers.json")
	if _, err := parseProviderCatalog(defaultProviders, "embedded providers.json", ""); err != nil {
		die("%s", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		die("%s", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		die("cannot create %s: %v (existing defaults are never overwritten)", path, err)
	}
	_, writeErr := f.Write(defaultProviders)
	closeErr := f.Close()
	if writeErr != nil {
		die("%s", writeErr)
	}
	if closeErr != nil {
		die("%s", closeErr)
	}
	fmt.Printf("[providers] created %s; edit this file to update defaults for future cell launches\n", path)
}
