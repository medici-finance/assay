package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Per-role starting context (#2438). Every role window of a cell starts with the same context
// today: every enabled plugin and skill, one shared memory index, every connector, the whole
// built-in tool set — and whatever the window dispatches inherits most of it, re-read on every
// request. CELL_ROLE_CONTEXT names a JSON declaration that NARROWS that per role; this file
// loads it, validates it and binds it to the harness. docs/cellctl-role-context.md is the
// operator-facing page.
//
// Three properties the rest of the file exists to hold:
//
//   - Silent when undeclared. A cell without CELL_ROLE_CONTEXT, and a role the declaration does
//     not name, launch with byte-identical argv and print nothing new (the Go↔oracle parity
//     harness diffs that output).
//   - Narrow only. The schema has no key that can enable a plugin, add a permission allow rule,
//     a hook, an environment variable, a connector or a model; agent definitions carry no model
//     and no permission mode. It composes with the model policy's --settings by ADDING keys and
//     refuses a key the launcher already set.
//   - Nothing the session can edit. The whole binding travels inline in the launch argv, at the
//     harness's command-line precedence — above every settings file a session can write.
//
// The declaration is harness-neutral; the binding below is Claude Code's. A role the cell runs
// on another harness refuses a non-empty declaration rather than silently ignoring it.

const (
	roleContextKey = "CELL_ROLE_CONTEXT"
	// roleContextMaxFile bounds every file the declaration names (itself, an instruction file,
	// an agent definition); roleContextMaxArg bounds one composed argv element, under the
	// smallest per-argument limit of the supported hosts.
	roleContextMaxFile = 64 << 10
	roleContextMaxArg  = 96 << 10
	builtinAgentPrefix = "builtin:"
)

// roleContextVerified is the Claude Code version the binding's settings keys and flags were
// verified on. An older harness ignores a settings key it does not know, so the role would start
// WIDER than declared (never wider than undeclared) — `check` warns, launch does not refuse.
var roleContextVerified = [3]int{2, 1, 295}

//go:embed agents/*.json
var builtinAgentFS embed.FS

var (
	rcPluginRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(@[A-Za-z0-9][A-Za-z0-9._-]*)?$`)
	rcSkillRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	rcToolRe      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
	rcAgentTypeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*(:[A-Za-z][A-Za-z0-9_-]*)?$`)
	rcAgentNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
)

// agentCapabilities is the neutral vocabulary an agent definition asks for, in the order the
// binding emits it. claudeCapabilityTools is Claude Code's binding of each.
var (
	agentCapabilities     = []string{"shell", "file-read", "file-write"}
	claudeCapabilityTools = map[string][]string{
		"shell":      {"Bash"},
		"file-read":  {"Read"},
		"file-write": {"Write", "Edit"},
	}
)

func roleContextFail(format string, args ...any) error {
	return fmt.Errorf("role-context: "+format, args...)
}

// roleContextSpec is one role's entry, exactly as written. Every key is optional and every
// absent key means "what the window does today".
type roleContextSpec struct {
	PluginsOff      []string `json:"plugins_off"`
	SkillsOff       []string `json:"skills_off"`
	MemoryDir       string   `json:"memory_dir"`
	MemoryOff       bool     `json:"memory_off"`
	Instructions    string   `json:"instructions"`
	InstructionsOff []string `json:"instructions_off"`
	Agents          []string `json:"agents"`
	DispatchAgent   string   `json:"dispatch_agent"`
	AgentsOff       []string `json:"agents_off"`
	ToolsOff        []string `json:"tools_off"`
	ConnectorsOff   bool     `json:"connectors_off"`
}

// agentDefinition is the harness-neutral definition of an agent a window may dispatch. It can
// say what the agent is for, what it may touch (capabilities) and whether it inherits the
// project's instruction files. It deliberately cannot name a model, a permission mode, a hook,
// a connector or a skill: the model policy's child-model hook lets an agent with no explicit
// model inherit the parent's pinned one, and an installed definition must not become a way
// around that or around the window's permission settings.
type agentDefinition struct {
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Prompt              string   `json:"prompt"`
	Capabilities        []string `json:"capabilities"`
	InheritInstructions *bool    `json:"inherit_instructions"`

	source string
}

func (a *agentDefinition) inheritsInstructions() bool {
	return a.InheritInstructions == nil || *a.InheritInstructions
}

// roleContext is one role's entry with every path resolved and every named file read.
type roleContext struct {
	Role             string
	Spec             roleContextSpec
	MemoryDir        string // absolute; "" when not declared
	InstructionsPath string // absolute; "" when not declared
	InstructionsText string
	Agents           []agentDefinition
}

type roleContextDecl struct {
	Path   string
	SHA256 string
	Roles  map[string]*roleContext
}

// empty reports a role entry that declares nothing ({}), which launches exactly like no entry.
func (rc *roleContext) empty() bool {
	s := rc.Spec
	return len(s.PluginsOff) == 0 && len(s.SkillsOff) == 0 && s.MemoryDir == "" && !s.MemoryOff &&
		s.Instructions == "" && len(s.InstructionsOff) == 0 && len(s.Agents) == 0 &&
		s.DispatchAgent == "" && len(s.AgentsOff) == 0 && len(s.ToolsOff) == 0 && !s.ConnectorsOff
}

// decodeStrict decodes exactly one JSON value and refuses an unknown field or trailing data —
// a typo'd key must be a refusal, never a silently ignored narrowing.
func decodeStrict(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("unexpected data after the JSON value")
	}
	return nil
}

// readBounded reads a regular file the declaration names, refusing an oversized or empty one.
func readBounded(path string) ([]byte, error) {
	raw, err := readPolicySource(path)
	if err != nil {
		return nil, err
	}
	if len(raw) > roleContextMaxFile {
		return nil, fmt.Errorf("%s: %d bytes, over the %d-byte limit", path, len(raw), roleContextMaxFile)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("%s: empty", path)
	}
	return raw, nil
}

func cellRelative(cellDir, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(cellDir, p)
}

// checkNames validates one list key: every value matches re, none is repeated.
func checkNames(role, key string, vals []string, re *regexp.Regexp) error {
	seen := map[string]bool{}
	for _, v := range vals {
		if !re.MatchString(v) {
			return roleContextFail("role %s: %s: %q is not a valid name", role, key, v)
		}
		if seen[v] {
			return roleContextFail("role %s: %s: %q is listed twice", role, key, v)
		}
		seen[v] = true
	}
	return nil
}

// cleanText refuses text an argv element cannot carry faithfully.
func cleanText(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

func parseAgentDefinition(raw []byte, source string) (agentDefinition, error) {
	var a agentDefinition
	if err := decodeStrict(raw, &a); err != nil {
		return a, roleContextFail("agent definition %s: %v", source, err)
	}
	a.source = source
	if !rcAgentNameRe.MatchString(a.Name) {
		return a, roleContextFail("agent definition %s: name %q must match %s", source, a.Name, rcAgentNameRe)
	}
	if strings.TrimSpace(a.Description) == "" || strings.TrimSpace(a.Prompt) == "" {
		return a, roleContextFail("agent definition %s: description and prompt are required", source)
	}
	if !cleanText(a.Description) || !cleanText(a.Prompt) {
		return a, roleContextFail("agent definition %s: description and prompt must be valid UTF-8 without NUL", source)
	}
	if len(a.Capabilities) == 0 {
		return a, roleContextFail("agent definition %s: capabilities is required (%s)", source, strings.Join(agentCapabilities, ", "))
	}
	seen := map[string]bool{}
	for _, c := range a.Capabilities {
		if !contains(agentCapabilities, c) {
			return a, roleContextFail("agent definition %s: unknown capability %q (%s)", source, c, strings.Join(agentCapabilities, ", "))
		}
		if seen[c] {
			return a, roleContextFail("agent definition %s: capability %q is listed twice", source, c)
		}
		seen[c] = true
	}
	return a, nil
}

// loadAgentDefinition resolves one `agents` entry: `builtin:<name>` is a definition this binary
// ships, anything else is a file path (relative to the cell directory, or absolute).
func loadAgentDefinition(ref, cellDir string) (agentDefinition, error) {
	if name, ok := strings.CutPrefix(ref, builtinAgentPrefix); ok {
		if !rcAgentNameRe.MatchString(name) {
			return agentDefinition{}, roleContextFail("unknown built-in agent definition %q (shipped: %s)", ref, strings.Join(builtinAgentNames(), ", "))
		}
		raw, err := builtinAgentFS.ReadFile("agents/" + name + ".json")
		if err != nil {
			return agentDefinition{}, roleContextFail("unknown built-in agent definition %q (shipped: %s)", ref, strings.Join(builtinAgentNames(), ", "))
		}
		return parseAgentDefinition(raw, ref)
	}
	path := cellRelative(cellDir, ref)
	raw, err := readBounded(path)
	if err != nil {
		return agentDefinition{}, roleContextFail("cannot read agent definition %s: %v", path, err)
	}
	return parseAgentDefinition(raw, path)
}

func builtinAgentNames() []string {
	entries, _ := builtinAgentFS.ReadDir("agents")
	var names []string
	for _, e := range entries {
		names = append(names, builtinAgentPrefix+strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(names)
	return names
}

// loadRoleContext reads and fully validates a declaration: every role it names is a known role,
// every key is a known key, and everything it points at exists and is readable NOW — a launch
// never discovers a missing file after the window is up.
func loadRoleContext(path, cellDir string) (*roleContextDecl, error) {
	raw, err := readBounded(path)
	if err != nil {
		return nil, roleContextFail("cannot read %s: %v", path, err)
	}
	var top struct {
		Version *int                       `json:"version"`
		Roles   map[string]json.RawMessage `json:"roles"`
	}
	if err := decodeStrict(raw, &top); err != nil {
		return nil, roleContextFail("%s: %v", path, err)
	}
	if top.Version == nil || *top.Version != 1 {
		return nil, roleContextFail("%s: \"version\" must be 1", path)
	}
	sum := sha256.Sum256(raw)
	d := &roleContextDecl{Path: path, SHA256: hex.EncodeToString(sum[:]), Roles: map[string]*roleContext{}}
	roles := make([]string, 0, len(top.Roles))
	for role := range top.Roles {
		roles = append(roles, role)
	}
	sort.Strings(roles) // a declaration with two faults always names the same one first
	for _, role := range roles {
		if !valueIn(role, knownRoles) {
			return nil, roleContextFail("%s: unknown role %q (%s)", path, role, strings.Join(knownRoles, ", "))
		}
		rc := &roleContext{Role: role}
		if err := decodeStrict(top.Roles[role], &rc.Spec); err != nil {
			return nil, roleContextFail("%s: role %s: %v", path, role, err)
		}
		if err := rc.resolve(cellDir); err != nil {
			return nil, err
		}
		// Compose once now, so an argument too large to launch with is a refusal here — in
		// `check`, and before a worktree exists — never a failure after the window is prepared.
		if _, err := applyClaudeRoleContext([]string{"claude", "prompt"}, rc); err != nil {
			return nil, fmt.Errorf("%w (role %s)", err, role)
		}
		d.Roles[role] = rc
	}
	return d, nil
}

func (rc *roleContext) resolve(cellDir string) error {
	s, role := &rc.Spec, rc.Role
	for _, l := range []struct {
		key  string
		vals []string
		re   *regexp.Regexp
	}{
		{"plugins_off", s.PluginsOff, rcPluginRe},
		{"tools_off", s.ToolsOff, rcToolRe},
		{"agents_off", s.AgentsOff, rcAgentTypeRe},
	} {
		if err := checkNames(role, l.key, l.vals, l.re); err != nil {
			return err
		}
	}
	for _, v := range s.SkillsOff {
		if strings.Contains(v, ":") {
			return roleContextFail("role %s: skills_off: %q is a plugin's skill; the harness hides those only with the whole plugin (plugins_off)", role, v)
		}
	}
	if err := checkNames(role, "skills_off", s.SkillsOff, rcSkillRe); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, g := range s.InstructionsOff {
		if g == "" || !cleanText(g) || !(filepath.IsAbs(g) || strings.HasPrefix(g, "**/")) {
			return roleContextFail("role %s: instructions_off: %q must be an absolute path glob (or start with **/)", role, g)
		}
		if seen[g] {
			return roleContextFail("role %s: instructions_off: %q is listed twice", role, g)
		}
		seen[g] = true
	}

	if s.MemoryOff && s.MemoryDir != "" {
		return roleContextFail("role %s: memory_dir and memory_off are mutually exclusive", role)
	}
	if s.MemoryDir != "" {
		dir := cellRelative(cellDir, s.MemoryDir)
		info, err := os.Stat(dir)
		if err != nil {
			return roleContextFail("role %s: memory_dir %s: %v", role, dir, err)
		}
		if !info.IsDir() {
			return roleContextFail("role %s: memory_dir %s is not a directory", role, dir)
		}
		rc.MemoryDir = dir
	}
	if s.Instructions != "" {
		p := cellRelative(cellDir, s.Instructions)
		raw, err := readBounded(p)
		if err != nil {
			return roleContextFail("role %s: instructions: %v", role, err)
		}
		if !cleanText(string(raw)) {
			return roleContextFail("role %s: instructions %s must be valid UTF-8 without NUL", role, p)
		}
		rc.InstructionsPath, rc.InstructionsText = p, strings.TrimSpace(string(raw))
	}

	names := map[string]bool{}
	for _, ref := range s.Agents {
		a, err := loadAgentDefinition(ref, cellDir)
		if err != nil {
			return fmt.Errorf("%w (role %s)", err, role)
		}
		if names[a.Name] {
			return roleContextFail("role %s: agents: two definitions are named %q", role, a.Name)
		}
		names[a.Name] = true
		rc.Agents = append(rc.Agents, a)
	}
	if s.DispatchAgent != "" {
		if !names[s.DispatchAgent] {
			return roleContextFail("role %s: dispatch_agent %q is not one of this role's agents", role, s.DispatchAgent)
		}
		if contains(s.AgentsOff, s.DispatchAgent) {
			return roleContextFail("role %s: dispatch_agent %q is also in agents_off", role, s.DispatchAgent)
		}
	}
	return nil
}

// summary is the one-line account `check` and a dry run print: what the role will start with,
// each part either named or shown as "default" (what every window gets today).
func (rc *roleContext) summary() string {
	s := rc.Spec
	list := func(v []string) string {
		if len(v) == 0 {
			return "none"
		}
		return strings.Join(v, ",")
	}
	memory := "default"
	switch {
	case s.MemoryOff:
		memory = "off"
	case rc.MemoryDir != "":
		memory = rc.MemoryDir
	}
	var agents []string
	for _, a := range rc.Agents {
		agents = append(agents, a.Name)
	}
	connectors := "default"
	if s.ConnectorsOff {
		connectors = "off"
	}
	return fmt.Sprintf("plugins_off=%s skills_off=%s memory=%s instructions=%s instructions_off=%s agents=%s dispatch_agent=%s agents_off=%s tools_off=%s connectors=%s",
		list(s.PluginsOff), list(s.SkillsOff), memory, orDefault(rc.InstructionsPath, "none"), list(s.InstructionsOff),
		list(agents), orDefault(s.DispatchAgent, "none"), list(s.AgentsOff), list(s.ToolsOff), connectors)
}

// ── Claude Code binding ─────────────────────────────────────────────────────────────────────

// claudeContextSettings is the role context's share of the --settings object. Every key is one a
// launch can only use to take something away: a plugin switched off, a skill hidden, the memory
// index relocated or disabled, instruction files excluded, connectors disabled, deny rules.
func (rc *roleContext) claudeContextSettings() map[string]any {
	s := rc.Spec
	out := map[string]any{}
	if len(s.PluginsOff) > 0 {
		m := map[string]bool{}
		for _, id := range s.PluginsOff {
			m[id] = false
		}
		out["enabledPlugins"] = m
	}
	if len(s.SkillsOff) > 0 {
		m := map[string]string{}
		for _, name := range s.SkillsOff {
			m[name] = "off"
		}
		out["skillOverrides"] = m
	}
	if rc.MemoryDir != "" {
		out["autoMemoryDirectory"] = rc.MemoryDir
	}
	if s.MemoryOff {
		out["autoMemoryEnabled"] = false
	}
	if len(s.InstructionsOff) > 0 {
		out["claudeMdExcludes"] = s.InstructionsOff
	}
	if s.ConnectorsOff {
		out["disableClaudeAiConnectors"] = true
	}
	var deny []string
	deny = append(deny, s.ToolsOff...)
	for _, t := range s.AgentsOff {
		deny = append(deny, "Agent("+t+")")
	}
	if len(deny) > 0 {
		out["permissions"] = map[string]any{"deny": deny}
	}
	return out
}

// claudeAgentsJSON renders the role's agent definitions as the harness's inline agent map.
func (rc *roleContext) claudeAgentsJSON() (string, error) {
	agents := map[string]any{}
	for _, a := range rc.Agents {
		var tools []string
		for _, capName := range agentCapabilities {
			if contains(a.Capabilities, capName) {
				tools = append(tools, claudeCapabilityTools[capName]...)
			}
		}
		def := map[string]any{"description": a.Description, "prompt": a.Prompt, "tools": tools}
		if !a.inheritsInstructions() {
			def["omitClaudeMd"] = true
		}
		agents[a.Name] = def
	}
	return compactJSON(agents)
}

// claudeAppendedPrompt is the text added to the window's own system prompt: the role's
// instruction file, then the one line that tells the window which installed agent to dispatch.
// The harness has no "default agent type" setting, so the instruction is how the window is told;
// agents_off is how the alternative is taken away.
func (rc *roleContext) claudeAppendedPrompt() string {
	var parts []string
	if rc.InstructionsText != "" {
		parts = append(parts, rc.InstructionsText)
	}
	if a := rc.Spec.DispatchAgent; a != "" {
		parts = append(parts, "Launcher role context: when you dispatch a worker agent, dispatch it as the \""+a+
			"\" agent type. It is installed for this window and starts with a smaller context than the default agent type. "+
			"Do not fall back to another agent type unless a step names one.")
	}
	return strings.Join(parts, "\n\n")
}

func compactJSON(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

// applyClaudeRoleContext rewrites a composed `claude …` launch argv to carry the role context.
// Invariants it holds for every caller downstream of the launcher:
//
//   - argv[0] and the final element (the skill prompt) do not move — the cadence launcher
//     inserts its print-mode flags after argv[0] and rewrites the last element;
//   - every flag it adds takes exactly zero or one value, so none can swallow the prompt;
//   - a --settings value already in argv (the model policy's) keeps every key byte for byte —
//     the context ADDS keys, and a key present on both sides is a refusal, never a merge.
//
// nil or an empty context returns argv itself, untouched.
func applyClaudeRoleContext(argv []string, rc *roleContext) ([]string, error) {
	if rc == nil || rc.empty() {
		return argv, nil
	}
	if len(argv) < 2 {
		return nil, roleContextFail("launch argv has no prompt to preserve")
	}
	middle := append([]string{}, argv[1:len(argv)-1]...)

	var flags []string
	if rc.Spec.ConnectorsOff {
		flags = append(flags, "--strict-mcp-config")
	}
	if len(rc.Agents) > 0 {
		agents, err := rc.claudeAgentsJSON()
		if err != nil {
			return nil, roleContextFail("cannot encode agent definitions: %v", err)
		}
		flags = append(flags, "--agents", agents)
	}
	if text := rc.claudeAppendedPrompt(); text != "" {
		flags = append(flags, "--append-system-prompt", text)
	}

	if add := rc.claudeContextSettings(); len(add) > 0 {
		merged := map[string]json.RawMessage{}
		at := -1
		for i := 0; i+1 < len(middle); i++ {
			if middle[i] == "--settings" {
				at = i + 1
				break
			}
		}
		if at >= 0 {
			if err := json.Unmarshal([]byte(middle[at]), &merged); err != nil {
				return nil, roleContextFail("the launch's own --settings is not a JSON object: %v", err)
			}
		}
		keys := make([]string, 0, len(add))
		for k := range add {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if _, clash := merged[k]; clash {
				return nil, roleContextFail("settings key %q is already set by the launcher; a role context never overrides it", k)
			}
			raw, err := compactJSON(add[k])
			if err != nil {
				return nil, roleContextFail("cannot encode %s: %v", k, err)
			}
			merged[k] = json.RawMessage(raw)
		}
		settings, err := compactJSON(merged)
		if err != nil {
			return nil, roleContextFail("cannot encode --settings: %v", err)
		}
		if at >= 0 {
			middle[at] = settings
		} else {
			flags = append(flags, "--settings", settings)
		}
	}

	out := make([]string, 0, len(argv)+len(flags))
	out = append(out, argv[0])
	out = append(out, flags...)
	out = append(out, middle...)
	out = append(out, argv[len(argv)-1])
	for _, a := range out {
		if len(a) > roleContextMaxArg {
			return nil, roleContextFail("a composed launch argument is %d bytes, over the %d-byte limit; shorten the role's instructions or agent prompts", len(a), roleContextMaxArg)
		}
	}
	return out, nil
}

// ── cell wiring ─────────────────────────────────────────────────────────────────────────────

// cellRoleContext loads the cell's declaration; (nil, nil) when CELL_ROLE_CONTEXT is unset.
// A container/scrubbed cell composes its own launch and has no binding here, so a declaration
// there is refused rather than silently unapplied — the same rule as CELL_MODEL_POLICY.
func (c *Cell) cellRoleContext() (*roleContextDecl, error) {
	path := c.Env.Get(roleContextKey)
	if path == "" {
		return nil, nil
	}
	if c.Kind == "container" || c.Kind == "scrubbed" {
		return nil, roleContextFail("%s currently requires a house or k8s cell; %s cannot apply it", roleContextKey, c.Kind)
	}
	return loadRoleContext(cellRelative(c.Dir, path), c.Dir)
}

// roleContextFor is the validated context one role launches with on its resolved harness.
// The role context is nil when the cell declares nothing for the role. A non-empty context on a
// harness with no binding is an error: the operator asked for a narrowing this launch cannot do.
func (c *Cell) roleContextFor(role, harness string) (*roleContextDecl, *roleContext, error) {
	d, err := c.cellRoleContext()
	if err != nil || d == nil {
		return nil, nil, err
	}
	rc := d.Roles[role]
	if rc == nil || rc.empty() {
		return d, nil, nil
	}
	if harness != "claude" {
		return d, nil, roleContextFail("role %s runs on %s; a role context is bound for the claude harness only — remove the role's entry or run it on claude", role, harness)
	}
	return d, rc, nil
}

// mustRoleContext is cmdDesk's early gate: a broken declaration stops the launch before a
// worktree is created or a lease is taken.
func (c *Cell) mustRoleContext(role, harness string) {
	if _, _, err := c.roleContextFor(role, harness); err != nil {
		die("%s", err)
	}
}

// roleContextArgv applies the role's context to the composed claude launch argv.
func (c *Cell) roleContextArgv(role string, argv []string) []string {
	_, rc, err := c.roleContextFor(role, "claude")
	if err != nil {
		die("%s", err)
	}
	out, err := applyClaudeRoleContext(argv, rc)
	if err != nil {
		die("%s", err)
	}
	return out
}

// printRoleContextPlan is the dry run's account. It prints NOTHING for a cell without a
// declaration, which is what keeps the dry-run output byte-identical for every existing cell.
func (c *Cell) printRoleContextPlan(role, harness string) {
	d, rc, err := c.roleContextFor(role, harness)
	if err != nil {
		die("%s", err)
	}
	if d == nil {
		return
	}
	if rc == nil {
		fmt.Printf("[dry-run] context role=%s source=%s sha256=%s (nothing declared for this role)\n", role, d.Path, d.SHA256)
		return
	}
	fmt.Printf("[dry-run] context role=%s source=%s sha256=%s %s\n", role, d.Path, d.SHA256, rc.summary())
	fmt.Printf("[dry-run] context flags: %s\n", strings.Join(rc.claudeFlagSummary(), " "))
}

// claudeFlagSummary names what the launch adds, sizes instead of contents: the instruction text
// and agent prompts are the operator's, and a dry run is often pasted somewhere.
func (rc *roleContext) claudeFlagSummary() []string {
	out, err := applyClaudeRoleContext([]string{"claude", "prompt"}, rc)
	if err != nil {
		return []string{"(" + err.Error() + ")"}
	}
	var flags []string
	for i := 1; i < len(out)-1; i++ {
		switch out[i] {
		case "--strict-mcp-config":
			flags = append(flags, out[i])
		case "--agents", "--append-system-prompt":
			flags = append(flags, out[i]+"=<"+strconv.Itoa(len(out[i+1]))+" bytes>")
			i++
		case "--settings":
			var m map[string]json.RawMessage
			_ = json.Unmarshal([]byte(out[i+1]), &m)
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			flags = append(flags, "--settings+="+strings.Join(keys, ","))
			i++
		}
	}
	return flags
}

// checkRoleContext is `cellctl check`'s section: one row per role naming what it will start
// with, a MISS for anything the declaration names that is missing or cannot be applied, and a
// warn where the declaration is valid but probably not what was meant. No declaration, no rows.
// harnessOf resolves a role's harness the way the caller already did (policy route or cell-wide).
func (c *Cell) checkRoleContext(k *checker, config string, harnessOf func(role string) string) {
	if c.Env.Get(roleContextKey) == "" {
		return
	}
	d, err := c.cellRoleContext()
	if err != nil {
		k.chk(false, "%s", err)
		return
	}
	k.chk(true, "role context: source=%s sha256=%s", d.Path, d.SHA256)
	anyClaude := false
	for _, role := range c.Roles {
		rc := d.Roles[role]
		if rc == nil || rc.empty() {
			k.na("role context: %s — nothing declared; starts with the cell-wide context", role)
			continue
		}
		harness := harnessOf(role)
		if harness != "claude" {
			k.chk(false, "role context: %s runs on %s; a role context is bound for the claude harness only", role, harness)
			continue
		}
		anyClaude = true
		k.chk(true, "role context: %s %s", role, rc.summary())
		if rc.MemoryDir != "" {
			if _, err := os.Stat(filepath.Join(rc.MemoryDir, "MEMORY.md")); err != nil {
				k.warn("role context: %s memory_dir %s has no MEMORY.md — the role starts with an EMPTY memory index (seed it from the shared one first; docs/cellctl-role-context.md)", role, rc.MemoryDir)
			}
		}
		if len(rc.Spec.PluginsOff) > 0 {
			enabled := enabledClaudePlugins(config, c.Repo, filepath.Join(c.Dir, "worktrees", role))
			for _, id := range rc.Spec.PluginsOff {
				if !enabled[id] {
					k.warn("role context: %s plugins_off names %s, which no settings file enables — nothing to switch off (check the id)", role, id)
				}
			}
		}
		for _, a := range rc.Agents {
			if !a.inheritsInstructions() {
				k.warn("role context: %s agent %s does not inherit the project's instruction files or the memory index — its dispatch prompt must be self-contained", role, a.Name)
			}
		}
	}
	for role := range d.Roles {
		if !valueIn(role, c.Roles) {
			k.warn("role context: %s is declared but this cell does not run it", role)
		}
	}
	if anyClaude {
		if v, ok := claudeVersionTriple(); ok && versionLess(v, roleContextVerified) {
			k.warn("role context: verified on Claude Code >=%d.%d.%d, found %d.%d.%d — an older harness ignores settings it does not know, so a role may start wider than declared",
				roleContextVerified[0], roleContextVerified[1], roleContextVerified[2], v[0], v[1], v[2])
		}
	}
}

// enabledClaudePlugins is every plugin id some settings file the window reads switches on.
func enabledClaudePlugins(config string, projects ...string) map[string]bool {
	out := map[string]bool{}
	for _, project := range projects {
		if _, err := os.Stat(project); err != nil {
			continue
		}
		for _, p := range claudeSettingsPaths(config, project) {
			raw, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			var s struct {
				EnabledPlugins map[string]json.RawMessage `json:"enabledPlugins"`
			}
			if json.Unmarshal(raw, &s) != nil {
				continue
			}
			for id, v := range s.EnabledPlugins {
				if string(bytes.TrimSpace(v)) != "false" {
					out[id] = true
				}
			}
		}
	}
	return out
}

func versionLess(a, b [3]int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
