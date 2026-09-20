package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// This file ports the CELL_MODEL_POLICY semantics #1388 added to the shell oracle
// (tools/cellctl/cellctl's model_policy()/apply_model_policy()/policy_claude_preflight()) into
// the Go binary — desk-containers/10 shipped the Go port without them (assay#1390). The oracle
// and docs/cellctl-model-policy.md are the spec; tools/cellctl/tests/model-policy.test.py is the
// behavioural ORACLE this file's tests port cases from. See the PR body for exactly which
// oracle behaviours this file does, and does not, carry over — some of the oracle's launch-time
// mechanics (the live PreModelSwitch/PreToolUse Claude Code hook wiring, the local/managed
// settings.json availableModels/modelOverrides conflict scan, and the `up`/`check` per-role
// preflight loops) are NOT ported here; the schema, resolution, deny and effort-propagation
// contract is.

// policyTierNames is the fixed four-tier ladder every provider must pin exactly.
var policyTierNames = []string{"top", "strong", "mid", "fast"}

// policyAliasToTier is the model-name alias each tier answers to, on top of the tier name itself.
var policyAliasToTier = map[string]string{"fable": "top", "opus": "strong", "sonnet": "mid", "haiku": "fast"}

// policyKnownRoles is the fixed role set a policy's `roles` map may name.
var policyKnownRoles = []string{"the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk"}

var claudeEffortLevels = []string{"low", "medium", "high", "xhigh", "max"}
var codexEffortLevels = []string{"minimal", "low", "medium", "high", "xhigh"}

var policyModelIDRe = regexp.MustCompile(`^[A-Za-z0-9_.:/-]+(\[1m\])?$`)
var policyProviderNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// PolicyTier is one provider's one tier: an EXACT model ID (never a floating alias), its default
// effort and the efforts that ID actually supports.
type PolicyTier struct {
	Model            string
	Effort           string
	SupportedEfforts []string
}

// PolicyProvider is one entry of the policy's `providers` map.
type PolicyProvider struct {
	Harness string
	Tiers   map[string]PolicyTier
}

// PolicyRole is one entry of the policy's `roles` map: which provider and which of its tiers.
type PolicyRole struct {
	Provider string
	Tier     string
}

// ModelPolicy is a loaded, SCHEMA-VALIDATED CELL_MODEL_POLICY file. Validation covers every
// provider and role in the file, not only the ones the current invocation's role touches — a
// typo in an unused provider still refuses the whole policy (docs/cellctl-model-policy.md
// "Resolution").
type ModelPolicy struct {
	Schema    int
	Deny      []string
	Providers map[string]PolicyProvider
	Roles     map[string]PolicyRole
	// Banned is Deny plus the two built-in Opus-5 patterns, which apply even when `deny` is
	// empty or omitted — the prohibition is not something a policy file can lift.
	Banned []string
	SHA256 string
}

func policyFail(format string, args ...any) error {
	return fmt.Errorf("model-policy: "+format, args...)
}

// exactKeys reports whether m's key set is exactly `keys` — no fewer, no more. It is how this
// file ports every one of the oracle's `set(x) != {...}` refusals (a required field missing OR
// an unexpected extra field are the SAME refusal).
func exactKeys(m map[string]json.RawMessage, keys ...string) bool {
	if len(m) != len(keys) {
		return false
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

func exactKeysSlice(m map[string]json.RawMessage, keys []string) bool {
	return exactKeys(m, keys...)
}

func contains(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

func allIn(vals, allowed []string) bool {
	for _, v := range vals {
		if !contains(allowed, v) {
			return false
		}
	}
	return true
}

// policyBase strips a trailing (case-insensitive) "[1m]" context suffix and lowercases the rest —
// the form every alias/tier/deny comparison is made in, so `Opus5`, `OPUS-5[1m]` and `opus-5` are
// one value for matching purposes.
func policyBase(value string) string {
	lower := strings.ToLower(value)
	return strings.TrimSuffix(lower, "[1m]")
}

func isFloatingAlias(base string) bool {
	if contains(policyTierNames, base) || strings.HasSuffix(base, "-latest") {
		return true
	}
	for alias := range policyAliasToTier {
		if base == alias {
			return true
		}
	}
	switch base {
	case "default", "best", "latest", "inherit", "opusplan":
		return true
	}
	return false
}

// globToRegex translates a shell-glob DENY pattern (`*`/`?` only — the only metacharacters the
// example policy and the oracle's fnmatch-based deny list use) into an ANCHORED regex. This is
// NOT filepath.Match/path.Match: both of those treat '/' as a path separator `*` will not cross,
// but a denied model id can legitimately contain one (`gateway/claude-opus-5`), and the oracle's
// fnmatch does not special-case it either.
func globToRegex(pat string) string {
	var b strings.Builder
	b.WriteString("^")
	for _, r := range pat {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	return b.String()
}

func policyDenied(value string, banned []string) bool {
	b := policyBase(value)
	for _, pat := range banned {
		if regexp.MustCompile(globToRegex(strings.ToLower(pat))).MatchString(b) {
			return true
		}
	}
	return false
}

// loadModelPolicy reads, parses and fully validates one CELL_MODEL_POLICY file — schema, deny,
// every provider (harness/tiers shape, exact model IDs, floating-alias refusal, per-harness
// effort levels, the GLM/K3 effort restriction) and every role (known name, provider+tier only,
// both resolvable). A single sha256 of the raw bytes is computed once and carried on the result
// (docs/cellctl-model-policy.md "Inspect, launch and verify adoption": `show`/dry-run print it).
func loadModelPolicy(path string) (*ModelPolicy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, policyFail("cannot read policy file %s: %v", path, err)
	}
	sum := sha256.Sum256(raw)
	m := &ModelPolicy{SHA256: hex.EncodeToString(sum[:])}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, policyFail("invalid JSON in %s: %v", path, err)
	}
	for k := range top {
		switch k {
		case "schema", "deny", "providers", "roles":
		default:
			return nil, policyFail("unknown policy field %q", k)
		}
	}
	var schema int
	if v, ok := top["schema"]; ok {
		_ = json.Unmarshal(v, &schema)
	}
	if schema != 1 {
		return nil, policyFail("unsupported schema (expected 1)")
	}
	m.Schema = schema

	var deny []string
	if v, ok := top["deny"]; ok {
		if err := json.Unmarshal(v, &deny); err != nil {
			return nil, policyFail("deny must contain nonempty model patterns")
		}
	}
	for _, d := range deny {
		if d == "" {
			return nil, policyFail("deny must contain nonempty model patterns")
		}
	}
	m.Deny = deny
	m.Banned = append(append([]string{}, deny...), "*opus-5*", "*opus5*")

	providersRaw, ok := top["providers"]
	if !ok {
		return nil, policyFail("missing providers")
	}
	var providersMap map[string]json.RawMessage
	if err := json.Unmarshal(providersRaw, &providersMap); err != nil {
		return nil, policyFail("invalid providers: %v", err)
	}
	m.Providers = map[string]PolicyProvider{}
	for name, praw := range providersMap {
		if !policyProviderNameRe.MatchString(name) {
			return nil, policyFail("invalid provider name %q", name)
		}
		var item map[string]json.RawMessage
		if err := json.Unmarshal(praw, &item); err != nil {
			return nil, policyFail("provider %q: %v", name, err)
		}
		if !exactKeys(item, "harness", "tiers") {
			return nil, policyFail("provider must contain harness and tiers")
		}
		var harness string
		_ = json.Unmarshal(item["harness"], &harness)
		if (harness != "claude" && harness != "codex") || (harness == "codex") != (name == "codex") {
			return nil, policyFail("codex provider requires codex harness; other providers require claude")
		}
		var tiersMap map[string]json.RawMessage
		if err := json.Unmarshal(item["tiers"], &tiersMap); err != nil {
			return nil, policyFail("provider %q: invalid tiers: %v", name, err)
		}
		if !exactKeysSlice(tiersMap, policyTierNames) {
			return nil, policyFail("%s must pin top, strong, mid and fast", name)
		}
		tiers := map[string]PolicyTier{}
		for tierName, traw := range tiersMap {
			var spec map[string]json.RawMessage
			if err := json.Unmarshal(traw, &spec); err != nil {
				return nil, policyFail("tier must contain model, effort and supported_efforts")
			}
			if !exactKeys(spec, "model", "effort", "supported_efforts") {
				return nil, policyFail("tier must contain model, effort and supported_efforts")
			}
			var value, effort string
			var supported []string
			_ = json.Unmarshal(spec["model"], &value)
			_ = json.Unmarshal(spec["effort"], &effort)
			if err := json.Unmarshal(spec["supported_efforts"], &supported); err != nil {
				return nil, policyFail("unsupported effort for %s/%s", name, tierName)
			}
			if value == "" || !policyModelIDRe.MatchString(value) {
				return nil, policyFail("invalid exact model ID")
			}
			if isFloatingAlias(policyBase(value)) {
				return nil, policyFail("tier targets must be exact IDs, not floating aliases")
			}
			if policyDenied(value, m.Banned) {
				return nil, policyFail("tier map contains denied model %s", value)
			}
			levels := claudeEffortLevels
			if harness == "codex" {
				levels = codexEffortLevels
			}
			if len(supported) == 0 || !allIn(supported, levels) || !contains(supported, effort) {
				return nil, policyFail("unsupported effort for %s/%s", name, tierName)
			}
			if (strings.HasPrefix(value, "glm-5.3") || policyBase(value) == "k3") && !allIn(supported, []string{"low", "high", "max"}) {
				return nil, policyFail("GLM 5.3 and Kimi K3 support low, high or max effort")
			}
			tiers[tierName] = PolicyTier{Model: value, Effort: effort, SupportedEfforts: supported}
		}
		m.Providers[name] = PolicyProvider{Harness: harness, Tiers: tiers}
	}

	rolesRaw, ok := top["roles"]
	if !ok {
		return nil, policyFail("missing roles")
	}
	var rolesMap map[string]json.RawMessage
	if err := json.Unmarshal(rolesRaw, &rolesMap); err != nil {
		return nil, policyFail("invalid roles: %v", err)
	}
	m.Roles = map[string]PolicyRole{}
	for name, rraw := range rolesMap {
		if !contains(policyKnownRoles, name) {
			return nil, policyFail("unknown role %s", name)
		}
		var item map[string]json.RawMessage
		if err := json.Unmarshal(rraw, &item); err != nil {
			return nil, policyFail("role %q: %v", name, err)
		}
		if !exactKeys(item, "provider", "tier") {
			return nil, policyFail("role must contain provider and tier")
		}
		var provider, tier string
		_ = json.Unmarshal(item["provider"], &provider)
		_ = json.Unmarshal(item["tier"], &tier)
		if _, ok := m.Providers[provider]; !ok {
			return nil, policyFail("unknown provider or tier for %s", name)
		}
		if !contains(policyTierNames, tier) {
			return nil, policyFail("unknown provider or tier for %s", name)
		}
		m.Roles[name] = PolicyRole{Provider: provider, Tier: tier}
	}

	return m, nil
}

// resolveInTiers is the shared alias/tier/exact-ID resolver the oracle's `resolve()` nested
// function implements, used both for the top-level role resolution and for an explicit
// --model/--provider override or a child-model request. `value` is never empty (callers check).
func resolveInTiers(tiers map[string]PolicyTier, value string, banned []string) (*PolicyTier, string, error) {
	if value == "" {
		return nil, "", policyFail("empty model request")
	}
	if policyDenied(value, banned) {
		return nil, "", policyFail("denied model %s", value)
	}
	base := policyBase(value)
	tierName := base
	if alias, ok := policyAliasToTier[base]; ok {
		tierName = alias
	}
	if spec, ok := tiers[tierName]; ok {
		if strings.HasSuffix(strings.ToLower(value), "[1m]") && !strings.HasSuffix(strings.ToLower(spec.Model), "[1m]") {
			return nil, "", policyFail("context suffix not pinned for %s", value)
		}
		cp := spec
		return &cp, tierName, nil
	}
	var matches []PolicyTier
	var matchTiers []string
	for tn, spec := range tiers {
		if value == spec.Model {
			matches = append(matches, spec)
			matchTiers = append(matchTiers, tn)
		}
	}
	if len(matches) > 0 {
		efforts := map[string]bool{}
		for _, mm := range matches {
			efforts[mm.Effort] = true
		}
		if len(efforts) != 1 {
			return nil, "", policyFail("ambiguous effort for model ID; request a tier instead")
		}
		cp := matches[0]
		return &cp, matchTiers[0], nil
	}
	return nil, "", policyFail("unmapped model %s", value)
}

// PolicyResolution is one role's fully resolved provider/harness/model/effort, plus what the
// launch needs to propagate that: the CLAUDE env block (ANTHROPIC_* aliases, the subagent model,
// the effort level) or the CODEX `-c` argv fragments. `tiers`/`banned` are carried so a later
// ResolveChild call resolves within the SAME provider's tier map without re-loading the file.
type PolicyResolution struct {
	Provider     string
	Harness      string
	Role         string
	Model        string
	Effort       string
	Tier         string
	PolicySHA256 string
	ClaudeEnv    map[string]string
	CodexArgs    []string

	tiers  map[string]PolicyTier
	banned []string
}

// Resolve is the port of the oracle's `model_policy resolve|launch`: given a role, an optional
// --provider/--model/--harness override (any of which may be empty), it returns the one
// provider/model/effort that role's window boots with, or an error naming exactly what refused.
func (m *ModelPolicy) Resolve(role, providerOverride, requested, harnessOverride string) (*PolicyResolution, error) {
	assignment, ok := m.Roles[role]
	if !ok {
		return nil, policyFail("unknown role %s", role)
	}
	provider := providerOverride
	if provider == "" {
		if harnessOverride == "codex" {
			provider = "codex"
		} else {
			provider = assignment.Provider
		}
	}
	entry, ok := m.Providers[provider]
	if !ok {
		return nil, policyFail("unknown provider or tier for %s", role)
	}
	harness := entry.Harness
	if harnessOverride != "" && harnessOverride != harness {
		return nil, policyFail("harness override disagrees with provider; select a matching --provider")
	}

	var selected *PolicyTier
	var tierName string
	if requested != "" {
		var err error
		selected, tierName, err = resolveInTiers(entry.Tiers, requested, m.Banned)
		if err != nil {
			return nil, err
		}
	} else {
		t, ok := entry.Tiers[assignment.Tier]
		if !ok {
			return nil, policyFail("unknown provider or tier for %s", role)
		}
		selected, tierName = &t, assignment.Tier
	}
	model, effort := selected.Model, selected.Effort
	if role == "the-desk" && harness == "claude" && strings.Contains(strings.ToLower(model), "opus") {
		return nil, policyFail("the-desk requires a non-Opus top-tier model")
	}

	res := &PolicyResolution{
		Provider: provider, Harness: harness, Role: role, Model: model, Effort: effort,
		Tier: tierName, PolicySHA256: m.SHA256, tiers: entry.Tiers, banned: m.Banned,
	}
	if harness == "claude" {
		env := map[string]string{}
		for alias, tn := range policyAliasToTier {
			env["ANTHROPIC_DEFAULT_"+strings.ToUpper(alias)+"_MODEL"] = entry.Tiers[tn].Model
		}
		env["ANTHROPIC_MODEL"] = model
		env["CLAUDE_CODE_SUBAGENT_MODEL"] = model
		env["CLAUDE_CODE_EFFORT_LEVEL"] = effort
		if provider == "anthropic" {
			env["ANTHROPIC_BASE_URL"] = "https://api.anthropic.com"
		}
		res.ClaudeEnv = env
	} else {
		res.CodexArgs = []string{
			"-c", `model_provider="openai"`,
			"-c", fmt.Sprintf("model_reasoning_effort=%q", effort),
			"-c", fmt.Sprintf("agents.default_subagent_model=%q", model),
			"-c", fmt.Sprintf("agents.default_subagent_reasoning_effort=%q", effort),
		}
	}
	return res, nil
}

// ResolveChild is the port of the oracle's PreToolUse Agent/Task hook branch: given an explicit
// child-model request from a dispatched sub-agent, it resolves that request within the SAME
// provider's tier map and refuses a child tier whose supported_efforts does not include the
// PARENT's effort — a child cannot silently inherit an effort layer it does not support.
// "" and "inherit" are the no-op child requests: they carry the parent's own model/effort
// forward unchanged.
func (res *PolicyResolution) ResolveChild(requested string) (model, effort string, err error) {
	if requested == "" || requested == "inherit" {
		return res.Model, res.Effort, nil
	}
	target, _, err := resolveInTiers(res.tiers, requested, res.banned)
	if err != nil {
		return "", "", err
	}
	if !contains(target.SupportedEfforts, res.Effort) {
		return "", "", policyFail("child model does not support inherited effort %s", res.Effort)
	}
	return target.Model, res.Effort, nil
}

// claudeMinVersion is docs/cellctl-model-policy.md's floor: "Claude Code >=2.1.251 is required."
var claudeMinVersion = [3]int{2, 1, 251}

var semverRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// checkClaudeMinVersion is the version half of the oracle's `policy_claude_preflight` — the
// settings.json/managed-settings allowlist-conflict scan that function also runs is NOT ported
// (see this file's header comment and the PR body). It shells out to `<bin> --version` (a local
// binary invocation, not a network call) and refuses below the floor above.
func checkClaudeMinVersion(bin string) error {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return policyFail("cannot read %s version for model policy: %v", bin, err)
	}
	m := semverRe.FindStringSubmatch(string(out))
	if m == nil {
		return policyFail("cannot parse %s version output for model policy: %q", bin, strings.TrimSpace(string(out)))
	}
	var v [3]int
	for i := 0; i < 3; i++ {
		v[i], _ = strconv.Atoi(m[i+1])
	}
	if v[0] < claudeMinVersion[0] ||
		(v[0] == claudeMinVersion[0] && v[1] < claudeMinVersion[1]) ||
		(v[0] == claudeMinVersion[0] && v[1] == claudeMinVersion[1] && v[2] < claudeMinVersion[2]) {
		return policyFail("model policy requires Claude Code >=2.1.251")
	}
	return nil
}
