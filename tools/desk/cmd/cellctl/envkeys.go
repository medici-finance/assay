package main

import (
	"strconv"
	"strings"
)

// The registry of every environment key cellctl reads: what it is, its compiled default, and
// which file may set it. It is the one list the others are drawn from —
//
//   - the machine-wide defaults template (`cellctl defaults print`, written once by `cellctl new`
//     and `cellctl defaults init`) is generated from it;
//   - the keys defaults.env refuses (cellDefaultsRefused, defaults.go) are every key here that is
//     not settable machine-wide: the per-cell keys and the not-a-lever keys;
//   - the keys `cellctl set` accepts without --force (knownCellEnvKey, set.go) are its levers.
//
// TestEnvKeyRegistryIsComplete walks every reader call in the package and fails when a key is
// read that this table does not classify, so a new key cannot ship without a decision here.
type envKeyClass int

const (
	// envMachine is a lever that may be set once for every cell, in defaults.env (and in any
	// one cell's cell.env, which overrides it).
	envMachine envKeyClass = iota
	// envCell is a lever that names, scopes or binds ONE cell. It belongs in that cell's
	// cell.env; defaults.env refuses it.
	envCell
	// envNotLever is read by cellctl but is not configuration this file may carry: a per-run
	// switch, a location followed from the launching shell, one of the host's own variables, or
	// internal plumbing. defaults.env refuses every one — the file outranks the process
	// environment, so a line for one here would be a value no launching shell could take back.
	envNotLever
)

type envKey struct {
	key   string
	class envKeyClass
	// group is the template section an envMachine key is listed under.
	group string
	// def is, for an envMachine key, the compiled default written after `=` in the template.
	// Empty means the key is unset by default or its default is derived; desc then says which.
	def string
	// desc is one line: what the key is, its values, and its default when def cannot carry it.
	desc string
	// why is the reason defaults.env refuses the key (envCell and envNotLever): it completes the
	// sentence "<file> sets KEY, which <why>".
	why string
	// from says where an envNotLever key comes from instead, which is also where the template
	// names it: envFromRun, envFromShell, envFromHost or envInternal.
	from envKeyFrom
	// unread is why no reader in this package reads the key, for the few that are written or
	// documented but consumed elsewhere.
	unread string
}

// envKeyFrom is where a not-a-lever key's value comes from, since no file may carry it.
type envKeyFrom int

const (
	// envFromRun is a switch for one run of one command, given in that command's environment.
	envFromRun envKeyFrom = iota + 1
	// envFromShell is a location cellctl follows from the launching shell: the cells root's
	// default, the operator's config home, another tool's own home.
	envFromShell
	// envFromHost is one of the host's own variables (HOME, PATH, TERM, ...).
	envFromHost
	// envInternal is plumbing: set by cellctl for a child and read back there, or a test hook.
	// The template does not name these.
	envInternal
)

// The template sections, in the order they are printed.
const (
	envGroupCache    = "Managed Go cache and task scratch"
	envGroupHarness  = "Harness, cockpit and launch"
	envGroupCadence  = "Cadence and boot"
	envGroupModels   = "Models and tiers"
	envGroupProvider = "Providers and model policy"
	envGroupPolicy   = "Policy"
)

var envKeyGroups = []string{
	envGroupCache, envGroupHarness, envGroupCadence, envGroupModels, envGroupProvider, envGroupPolicy,
}

// Compiled defaults shared with the code that applies them, so the template cannot drift from
// the behaviour it describes.
const (
	scrubbedPathTail         = "/usr/bin:/bin:/usr/sbin:/sbin"
	orcaProbeSeconds         = 5
	autoCompactWindowDefault = "200000"
	deskwtDefault            = "1"
)

// envKeys is the registry. Order is presentation order within each class.
var envKeys = buildEnvKeys()

func buildEnvKeys() []envKey {
	keys := []envKey{
		// ── settable machine-wide ────────────────────────────────────────────────────────────
		{key: "CELL_GO_CACHE", class: envMachine, group: envGroupCache, def: "off",
			desc: "Managed per-cell Go build cache: on or off (macOS and Linux only)."},
		{key: "CELL_GO_CACHE_BYTES", class: envMachine, group: envGroupCache, def: "8589934592",
			desc: "Size budget of one cell's managed Go cache, in bytes (8 GiB)."},
		{key: "CELL_GO_CACHE_MIN_FREE", class: envMachine, group: envGroupCache, def: "10737418240",
			desc: "Free space the managed Go cache leaves on its volume, in bytes (10 GiB)."},
		{key: "CELL_SCRATCH_MAX_AGE", class: envMachine, group: envGroupCache, def: "168h",
			desc: "Age past which managed task scratch is swept, as a duration."},
		{key: "CELL_SCRATCH_MAX_BYTES", class: envMachine, group: envGroupCache, def: "268435456",
			desc: "Total size past which managed task scratch is swept oldest first, in bytes (256 MiB)."},

		{key: "CELL_HARNESS", class: envMachine, group: envGroupHarness, def: "claude",
			desc: "Harness every role window boots on: " + strings.Join(harnessValues, ", ") + " (cursor needs a house cell)."},
		{key: "CELL_COCKPIT", class: envMachine, group: envGroupHarness, def: "auto",
			desc: "Surface `up` opens the role windows in: " + strings.Join(cockpitValues, ", ") + "."},
		{key: "CELLCTL_ORCA_TIMEOUT", class: envMachine, group: envGroupHarness, def: strconv.Itoa(orcaProbeSeconds),
			desc: "Seconds the orca reachability probe may take before orca counts as unavailable."},
		{key: "CLAUDE_CODE_AUTO_COMPACT_WINDOW", class: envMachine, group: envGroupHarness, def: autoCompactWindowDefault,
			desc: "Context size, in tokens, at which a Claude role window compacts."},
		{key: "DESK_TOOLS_BIN", class: envMachine, group: envGroupHarness,
			desc: "Directory holding the desk verbs the shims wrap. Default: /opt/desk-tools/bin (on Windows, Assay/bin under the local application data directory)."},
		{key: "CELL_PATH", class: envMachine, group: envGroupHarness, def: scrubbedPathTail,
			desc: "Scrubbed cells: the trailing system part of the composed PATH."},
		{key: "CELL_ROLE_CONTEXT", class: envMachine, group: envGroupHarness,
			desc: "Role context declaration file; a relative path is taken under each cell's directory. Unset by default. House and k8s cells only: a container or scrubbed cell refuses it."},

		{key: "CELL_CADENCE", class: envMachine, group: envGroupCadence,
			desc: "Interval between ticks of a role window, a duration from 1s to 24h, or off. Default: none, except 5m on a house cell running codex. House cells only: any other kind refuses a cadence."},
		{key: "CELL_TICK_BUDGET", class: envMachine, group: envGroupCadence,
			desc: "Time one tick may run, a duration from 61s to 24h. Default: 20m once a cadence is set; without a cadence a launch refuses it."},
		{key: "CELL_FF_ROOTS", class: envMachine, group: envGroupCadence, def: "0",
			desc: "1 fast-forwards, at desk boot, every stream root that can move without a decision."},
		{key: "CELLCTL_DESKWT", class: envMachine, group: envGroupCadence, def: deskwtDefault,
			desc: "1 lets an installed deskwt name each role worktree; any other value uses cellctl's own worktree path."},
		{key: "CELLCTL_GENERATED_FILES", class: envMachine, group: envGroupCadence, def: "STATUS.md docs/streams/FINDINGS.md",
			desc: "Files a boot merge takes from the main branch outright, separated by spaces."},

		{key: "DESK_MODEL_DEFAULT", class: envMachine, group: envGroupModels, def: "sonnet",
			desc: "Model every claude role window launches on when no per-role pin names one."},
	}
	const pinClaude = "Per-role model pin on claude (unset by default). An Opus pin is refused for the-desk."
	for _, role := range strings.Fields(rolesDefault) {
		keys = append(keys, envKey{key: "DESK_MODEL_" + underscore(role), class: envMachine, group: envGroupModels, desc: pinClaude})
	}
	keys = append(keys, envKey{key: "CODEX_MODEL_default", class: envMachine, group: envGroupModels,
		desc: "Model every codex role window launches on when no per-role pin names one (unset by default: the tier map decides)."})
	const pinCodex = "Per-role model pin on codex (unset by default)."
	for _, role := range strings.Fields(rolesDefault) {
		keys = append(keys, envKey{key: "CODEX_MODEL_" + underscore(role), class: envMachine, group: envGroupModels, desc: pinCodex})
	}
	keys = append(keys, envKey{key: "CURSOR_MODEL_default", class: envMachine, group: envGroupModels,
		desc: "Model every cursor role window launches on when no per-role pin names one (unset by default; cursor has no compiled model)."})
	const pinCursor = "Per-role model pin on cursor (unset by default)."
	for _, role := range strings.Fields(rolesDefault) {
		keys = append(keys, envKey{key: "CURSOR_MODEL_" + underscore(role), class: envMachine, group: envGroupModels, desc: pinCursor})
	}
	keys = append(keys, envKey{key: "CELL_MODEL_TTL_DAYS", class: envMachine, group: envGroupModels,
		desc: "Days a model pin recorded DEFAULT (`set ... --default`) stays before a launch lowers it to the MID tier: Nd, a number of days, or a Go duration; 0 turns the reset off. Default: 7."})
	const tier = "Tier map, used when no pin and no harness default names a model: the-desk resolves at TOP, every other role at MID."
	for _, harness := range []string{"claude", "codex", "cursor"} {
		for _, level := range []string{"TOP", "MID", "FAST"} {
			k := "TIER_MODEL_" + level + "_" + strings.ToUpper(harness)
			keys = append(keys, envKey{key: k, class: envMachine, group: envGroupModels, def: tierModelDefaults[k], desc: tier})
		}
	}
	keys = append(keys,
		envKey{key: "CELL_PROVIDER", class: envMachine, group: envGroupProvider,
			desc: "Provider `desk` and `up` launch on (unset by default: the harness's own provider)."},
	)
	const preset = "Built-in provider presets: the endpoint, the NAME of the variable holding the token (never the token), and the model."
	for _, name := range []string{"kimi", "glm"} {
		for _, suffix := range providerSuffixes {
			keys = append(keys, envKey{key: providerVar(name, suffix), class: envMachine, group: envGroupProvider,
				def: providerPreset(name, suffix), desc: preset})
		}
	}
	keys = append(keys,
		envKey{key: "CELL_PROVIDER_DEFAULTS", class: envMachine, group: envGroupProvider,
			desc: "Shared provider catalog. Default: providers.json in the cells root. House and k8s cells only: a container or scrubbed cell refuses it."},
		envKey{key: "CELL_PROVIDER_OVERRIDES", class: envMachine, group: envGroupProvider,
			desc: "A cell's exceptions to the shared catalog; a relative path is taken under each cell's directory. Default: providers.json there. House and k8s cells only."},
		envKey{key: "CELL_MODEL_POLICY", class: envMachine, group: envGroupProvider,
			desc: "Complete per-role model policy file; a relative path is taken under each cell's directory. Unset by default. House and k8s cells only."},

		envKey{key: "ASSAY_REPAIR_ADMISSION", class: envMachine, group: envGroupPolicy, def: "off",
			desc: "Dispatch-boundary repair-admission gate for the desks a cell launches: on or off."},

		// ── per-cell: refused in defaults.env ────────────────────────────────────────────────
		envKey{key: "CELL", class: envCell, desc: "The cell's name (default: its directory name).",
			why: "names one cell"},
		envKey{key: "CELL_KIND", class: envCell, desc: "The cell's kind: " + strings.Join(kindValues, ", ") + " (default: k8s).",
			why: "is one cell's kind, with that kind's own preconditions"},
		envKey{key: "ROLES", class: envCell, desc: "The role windows `up` opens (default: all five; the-desk on a container cell).",
			why: "is one cell's set of role windows"},
		envKey{key: "CELL_REPO", class: envCell, desc: "The checkout the cell's role worktrees are created from.",
			why: "is the one checkout a cell's role worktrees are created from"},
		envKey{key: "CELL_REPO_SLUG", class: envCell, desc: "Scrubbed cells: the one repository the cell is scoped to.",
			why: "is the one repository a scrubbed cell is scoped to"},
		envKey{key: "CELL_ROOTS", class: envCell, desc: "The cell's stream-root map, exported to its role windows.",
			why: "is one cell's stream-root map, the scope its desks read"},
		envKey{key: "CELLS_CONFIG", class: envCell, desc: "The cell's cells.yaml slice (default: cells-<cell>.yaml in the cell's directory).",
			why: "is one cell's own cells.yaml slice"},
		envKey{key: "CELL_COMMS_CONFIG", class: envCell, desc: "The cell's comms manifest (unset by default).",
			why: "is a comms manifest that names the single cell it belongs to"},
		envKey{key: "DESKD", class: envCell, desc: "1 stands a deskd for the cell (default: 1 on a k8s cell, 0 otherwise).",
			why: "decides whether one cell runs its own deskd, which a container or scrubbed cell refuses outright"},
		envKey{key: "DESKD_ADDR", class: envCell, desc: "The address the cell's deskd serves on (default: 127.0.0.1:8787).",
			why: "is the address one cell's deskd listens on"},
		envKey{key: "DESKD_INDEX", class: envCell, desc: "The cell's deskd index (default: index/index.db in the cell's directory).",
			why: "is one cell's deskd index"},
		envKey{key: "TMUX_SESSION", class: envCell, desc: "The cell's tmux session name (default: <cell>-cell).",
			why: "is one cell's session name, which `down` stops"},
		envKey{key: "CELL_GO_CACHE_ROOT", class: envCell, desc: "The cell's managed Go cache root (default: go-cache in the cell's directory).",
			why: "is a cache root marked for exactly one cell; a second cell is refused it"},
		envKey{key: "CELL_CONTAINER_CONFIG", class: envCell, desc: "Container cells: the configuration file the cell is defined in.",
			why: "binds one cell to its container definition"},
		envKey{key: "CELL_CONTAINER_LAUNCHER", class: envCell, desc: "Container cells: the launcher the cell is started with.",
			why: "binds one cell to its container launcher"},
		envKey{key: "CELL_FORGE", class: envCell, desc: "The cell's forge: github or gitlab (default: github).",
			why: "is one cell's forge"},
		envKey{key: "GITHUB_HOST", class: envCell, desc: "GitHub cells: the forge host (default: github.com).",
			why: "is the forge host one cell's credentials are sent to"},
		envKey{key: "FORGE_API_BASE", class: envCell, desc: "The forge API endpoint (default: derived from the forge and its host).",
			why: "is the forge endpoint one cell's credentials are sent to"},
		envKey{key: "GITLAB_API_BASE", class: envCell, desc: "GitLab cells: the GitLab API base (default: https://gitlab.com/api/v4).",
			why: "is the forge endpoint one cell's credentials are sent to"},
		envKey{key: "GITLAB_GROUP", class: envCell, desc: "GitLab cells: the group the cell reads.",
			why: "is the group one cell reads"},
		envKey{key: "GITLAB_TOKEN_STORE", class: envCell, desc: "GitLab cells: the directory holding the role token files (default: the cell's config home).",
			why: "is one cell's token store"},
		envKey{key: "DESKD_GITLAB_TOKEN_FILE", class: envCell, desc: "GitLab cells: the deskd read token file (default: gitlab-deskd.token in the token store).",
			why: "is one cell's deskd read token"},
		envKey{key: "DESKD_APP_PEM", class: envCell, desc: "GitHub cells: the deskd read App's private key file.",
			why: "is one cell's deskd read key"},
		envKey{key: "DESKD_APP_ID_VAR", class: envCell, desc: "GitHub cells: the variable name holding the deskd read App's id (default: DESK_APP_ID).",
			why:    "names the App id one cell's deskd mints with",
			unread: "written into cell.env by `cellctl new` and documented; the deskd credential now comes from the role credential path, so nothing in cellctl reads it back"},
		envKey{key: "ORGS", class: envCell, desc: "GitHub cells: the orgs the cell's deskd reads.",
			why: "is the set of organisations one cell mints tokens for"},

		// ── not a lever: switches for one run ────────────────────────────────────────────────
		envKey{key: "CELLS_ROOT", class: envNotLever, from: envFromRun,
			desc: "Where cells live, or --cells-root (default: assay/cells under the user data directory).",
			why:  "locates this file and every cell, and is resolved before the file is read"},
		envKey{key: "DRY_RUN", class: envNotLever, from: envFromRun,
			desc: "1 prints what one command would do and changes nothing.",
			why:  "is one run's switch, and would turn every launch on the machine into a dry run"},
		envKey{key: "CELL_ATTENDED", class: envNotLever, from: envFromRun,
			desc: "1 states that a person is at this command (`deskd`, `up`).",
			why:  "states that a person is at one run, which a file cannot state"},
		envKey{key: "DESK_MODEL_OVERRIDE", class: envNotLever, from: envFromRun,
			desc: "The model for one run, the same as --model.",
			why:  "outranks every model pin, so no cell.env could override it"},

		// ── not a lever: locations followed from the launching shell ─────────────────────────
		envKey{key: "XDG_DATA_HOME", class: envNotLever, from: envFromShell,
			desc: "The user data directory the default cells root is under.",
			why:  "locates the default cells root, which is resolved before this file is read"},
		envKey{key: "ASSAY_CONFIG_HOME", class: envNotLever, from: envFromShell,
			desc: "The operator's assay config home (default: .config/assay under the home directory).",
			why:  "locates the operator's config home, where every desk tool finds its credentials"},
		envKey{key: "XDG_CONFIG_HOME", class: envNotLever, from: envFromShell,
			desc: "The user config directory the GitHub CLI login is looked for in.",
			why:  "locates the operator's GitHub CLI login"},
		envKey{key: "GH_CONFIG_DIR", class: envNotLever, from: envFromShell,
			desc: "The GitHub CLI's config directory.",
			why:  "locates the operator's GitHub CLI login"},
		envKey{key: "CLAUDE_CONFIG_DIR", class: envNotLever, from: envFromShell,
			desc: "The Claude harness's home.",
			why:  "locates the operator's Claude harness home"},
		envKey{key: "CODEX_HOME", class: envNotLever, from: envFromShell,
			desc: "The Codex harness's home.",
			why:  "locates the operator's Codex harness home"},

		// ── not a lever: the host's own variables ────────────────────────────────────────────
		envKey{key: "HOME", class: envNotLever, from: envFromHost,
			why: "is the operator's home directory, which the default cells root and every config home are found under"},
		envKey{key: "USERPROFILE", class: envNotLever, from: envFromHost,
			why: "is the operator's home directory on Windows, which the default cells root and every config home are found under"},
		envKey{key: "APPDATA", class: envNotLever, from: envFromHost,
			why: "is where Windows keeps the operator's GitHub CLI login"},
		envKey{key: "LOCALAPPDATA", class: envNotLever, from: envFromHost,
			why: "is where Windows installs the desk verbs (DESK_TOOLS_BIN is the lever for that directory)"},
		envKey{key: "PATH", class: envNotLever, from: envFromHost,
			why: "is the launching shell's search path (DESK_TOOLS_BIN and CELL_PATH are the levers for the parts cellctl composes)"},
		envKey{key: "TERM", class: envNotLever, from: envFromHost,
			why: "describes the terminal one command runs in"},
		envKey{key: "LANG", class: envNotLever, from: envFromHost,
			why: "is the launching shell's locale"},

		// ── not a lever: internal plumbing ───────────────────────────────────────────────────
		envKey{key: "CELL_KIND_OVERRIDE_INTERNAL", class: envNotLever, from: envInternal,
			why: "carries one invocation's --kind and would re-kind every cell on every run"},
		envKey{key: "ASSAY_SCRATCH_ID", class: envNotLever, from: envInternal,
			why: "is set by cellctl in a managed task's environment and read back by `cellctl scratch` there"},
		envKey{key: "DESK_SESSION", class: envNotLever, from: envInternal,
			why: "is set by cellctl in each role window and read back by `cellctl scratch` there"},
		envKey{key: "CELLCTL_PARITY_MUTATE", class: envNotLever, from: envInternal,
			why: "is a test hook compiled only under the parity build tag"},
	)
	return keys
}

// tierModelDefaults is the compiled tier map (cursor has no compiled model, so no entry).
// loadCell applies it and the registry prints it, from this one table.
var tierModelDefaults = map[string]string{
	"TIER_MODEL_TOP_CLAUDE":  "fable",
	"TIER_MODEL_MID_CLAUDE":  "sonnet",
	"TIER_MODEL_FAST_CLAUDE": "haiku",
	"TIER_MODEL_TOP_CODEX":   "gpt-5.6-terra",
	"TIER_MODEL_MID_CODEX":   "gpt-5.6-terra",
	"TIER_MODEL_FAST_CODEX":  "gpt-5.6-terra",
}

// providerSuffixes are the three variables one provider is declared with.
var providerSuffixes = []string{"BASE_URL", "TOKEN_ENV", "MODEL"}

// envKeyFamilies are the keys whose middle is a name the operator chooses, so no table can list
// them: a provider other than the built-in presets. They are levers, settable machine-wide.
const envKeyProviderFamily = "CELL_PROVIDER_<NAME>_BASE_URL, CELL_PROVIDER_<NAME>_TOKEN_ENV, CELL_PROVIDER_<NAME>_MODEL"

// isProviderFamilyKey reports a CELL_PROVIDER_<NAME>_{BASE_URL,TOKEN_ENV,MODEL} key with a
// non-empty <NAME>.
func isProviderFamilyKey(k string) bool {
	if !strings.HasPrefix(k, "CELL_PROVIDER_") {
		return false
	}
	for _, suffix := range providerSuffixes {
		if strings.HasSuffix(k, "_"+suffix) && len(k) > len("CELL_PROVIDER_")+len(suffix)+1 {
			return true
		}
	}
	return false
}

var envKeyIndex = func() map[string]int {
	idx := make(map[string]int, len(envKeys))
	for i, k := range envKeys {
		idx[k.key] = i
	}
	return idx
}()

// envKeyLookup is the registry's answer for one key: its entry when it is listed, or the
// provider family's when its name is one the operator chose.
func envKeyLookup(key string) (envKey, bool) {
	if i, ok := envKeyIndex[key]; ok {
		return envKeys[i], true
	}
	if isProviderFamilyKey(key) {
		return envKey{key: key, class: envMachine, group: envGroupProvider,
			desc: "A provider's endpoint, token variable name, or model."}, true
	}
	return envKey{}, false
}

// envKeyIsLever reports a key that belongs in a file: machine-wide or per-cell. It is what
// `cellctl set` accepts without --force.
func envKeyIsLever(key string) bool {
	k, ok := envKeyLookup(key)
	return ok && k.class != envNotLever
}

// envKeyRefusals is every key defaults.env refuses, with the reason, in registry order: every
// key cellctl reads that is not settable machine-wide. A key the registry does not list at all is
// one cellctl does not read, and the file may carry it, as a cell.env may.
func envKeyRefusals() []struct{ key, why string } {
	var out []struct{ key, why string }
	for _, k := range envKeys {
		if k.class != envMachine {
			out = append(out, struct{ key, why string }{k.key, k.why})
		}
	}
	return out
}

// envTemplateValue writes a default as a cell.env right-hand side: bare when every byte is one
// no reader treats specially, double-quoted otherwise (a space, a bracket).
func envTemplateValue(v string) string {
	for i := 0; i < len(v); i++ {
		c := v[i]
		bare := c == '_' || c == '-' || c == '.' || c == '/' || c == ':' ||
			(c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		if !bare {
			return `"` + v + `"`
		}
	}
	return v
}

// cellDefaultsTemplate is the machine-wide defaults file as cellctl writes it: every settable
// key commented out under its description, then the keys the file refuses and the switches no
// file carries. Unedited, it sets nothing.
func cellDefaultsTemplate() string {
	var b strings.Builder
	w := func(lines ...string) {
		for _, l := range lines {
			b.WriteString(strings.TrimRight(l, " "))
			b.WriteByte('\n')
		}
	}
	w("# cellctl machine-wide cell defaults: "+cellDefaultsFile,
		"#",
		"# One file per cells root, read for every cell under it, in cell.env's grammar",
		"# (one KEY=VALUE per line). A value comes from the highest layer that sets it:",
		"#",
		"#   compiled default  <  process environment  <  this file  <  the cell's cell.env",
		"#",
		"# so a line in a cell's own cell.env overrides the same line here.",
		"#",
		"# Every line below is commented out: this file sets nothing until one is uncommented.",
		"# The value shown is the compiled default. An empty value means the key is unset by",
		"# default, or its default is derived; the line above it says which.",
		"#",
		"# A line whose value is empty sets nothing: the layer below stands, as if the line were",
		"# still commented out. A line WITH a value outranks the same key exported in the shell,",
		"# also when the value is the compiled default shown here.",
		"#",
		"# cellctl writes this file once, when there is none (`cellctl new`, `cellctl defaults init`),",
		"# and never changes it afterwards. `cellctl defaults print` prints the current template, to",
		"# compare against. `cellctl check <cell>` names the keys this file sets for that cell.")
	for _, group := range envKeyGroups {
		w("", "# --- "+group+" ---")
		last := ""
		for _, k := range envKeys {
			if k.class != envMachine || k.group != group {
				continue
			}
			if k.desc != last {
				w("#")
				for _, l := range wrapComment(k.desc) {
					w("# " + l)
				}
				last = k.desc
			}
			w("# " + k.key + "=" + envTemplateValue(k.def))
		}
		if group == envGroupProvider {
			w("#")
			for _, l := range wrapComment("Any other provider is declared the same way, under a name of your choosing: " + envKeyProviderFamily + ".") {
				w("# " + l)
			}
		}
	}
	w("", "# --- Per-cell keys: refused in this file ---",
		"#",
		"# Each of these names, scopes or binds one cell, so a machine-wide value would be wrong",
		"# for every cell but one. Set them in the cell's own cell.env (`cellctl set <cell> ...`).",
		"# A line for one here is refused, naming the key and the line.")
	for _, k := range envKeys {
		if k.class != envCell {
			continue
		}
		w("#")
		for _, l := range wrapComment(k.key + ": " + k.desc) {
			w("# " + l)
		}
		for _, l := range wrapComment("refused here: it " + k.why + ".") {
			w("#     " + l)
		}
	}
	w("", "# --- Taken from the command's environment: refused in this file ---",
		"#",
		"# cellctl reads these from the environment of the command that runs it. A line for one",
		"# here is refused: this file outranks that environment, so no shell could take it back.")
	for _, from := range []envKeyFrom{envFromRun, envFromShell} {
		w("#")
		if from == envFromRun {
			w("# Switches for one run:")
		} else {
			w("# Locations followed from the launching shell:")
		}
		for _, k := range envKeys {
			if k.class != envNotLever || k.from != from {
				continue
			}
			for i, l := range wrapComment(k.key + ": " + k.desc) {
				if i == 0 {
					w("#   " + l)
				} else {
					w("#       " + l)
				}
			}
		}
	}
	var host []string
	for _, k := range envKeys {
		if k.class == envNotLever && k.from == envFromHost {
			host = append(host, k.key)
		}
	}
	w("#")
	for _, l := range wrapComment("The host's own variables, refused the same way: " + strings.Join(host, ", ") + ".") {
		w("# " + l)
	}
	return b.String()
}

// wrapComment breaks one description into lines of at most 88 columns, on spaces.
func wrapComment(s string) []string {
	const width = 88
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		if line != "" && len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
