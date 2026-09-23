package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const deskUsage = "cellctl desk <cell> <role> [--model <m>] [--set] [--provider <name>] [--harness <claude|codex>] [--kind <k>] [--cockpit <c>] [CLAUDE_CONFIG_DIR]"

// cmdDesk is the port of `cmd_desk`: one role window, in its own worktree, with the real HOME
// and the cell's shims — or, on a scrubbed cell, in a fully COMPOSED environment.
//
// The ordering below is load-bearing and mirrors the oracle exactly, because every refusal is a
// contract: --kind is applied before the cell loads (its preconditions are asserted there), the
// role is validated before the flags, the flags before the provider, the provider before the
// model, and the Opus rule binds the RESOLVED model whether it came from a pin or an override.
func cmdDesk(cell string, args []string) {
	if len(args) == 0 || args[0] == "" {
		fmt.Fprintln(os.Stderr, "cellctl: "+deskUsage)
		exitWith(1)
	}
	role := args[0]
	args = args[1:]

	kindOverride := prescanKindOverride(args)
	c := loadCellWithKind(cell, kindOverride)
	if !valueIn(role, knownRoles) {
		die("unknown role '%s'", role)
	}

	// --model overrides the cell.env pin for THIS run only; DESK_MODEL_OVERRIDE is the
	// equivalent env form for a wrapper that cannot pass a flag, and an explicit --model wins
	// when both are given.
	modelOverride := c.Env.Get("DESK_MODEL_OVERRIDE")
	persist := false
	cfgIn, provider := "", ""
	harness := c.Harness
	harnessFlag, cockpitFlag, providerFlag := "", "", ""

	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--model":
			modelOverride = needFlagValue(args, &i, "--model needs a value")
		case "--set":
			persist = true
		case "--provider":
			provider = needFlagValue(args, &i, "--provider needs a value (kimi|glm, or a name with CELL_PROVIDER_<NAME>_BASE_URL/_TOKEN_ENV in cell.env)")
			providerFlag = provider
		case "--harness":
			harness = needFlagValue(args, &i, "--harness needs a value (claude|codex)")
			harnessFlag = harness
		case "--kind":
			// consumed by prescanKindOverride above; c.Kind already reflects it
			i++
		case "--cockpit":
			cockpitFlag = needFlagValue(args, &i, "--cockpit needs a value ("+joinPipe(cockpitValues)+")")
		default:
			if strings.HasPrefix(a, "--") {
				die("desk: unknown flag %s", a)
			}
			cfgIn = a
		}
	}
	if cockpitFlag != "" && !valueIn(cockpitFlag, cockpitValues) {
		die("desk: --cockpit must be one of %s, got '%s'", joinPipe(cockpitValues), cockpitFlag)
	}
	if persist && !anyGiven(modelOverride, harnessFlag, providerFlag, c.KindOverride, cockpitFlag) {
		die("desk: --set needs --model <m> (or DESK_MODEL_OVERRIDE in the environment), --harness, --provider, --kind or --cockpit — nothing to persist otherwise")
	}
	if !valueIn(harness, harnessValues) {
		die("desk: --harness must be claude or codex, got '%s'", harness)
	}

	// CELL_MODEL_POLICY (assay#1390, porting #1388) supersedes the legacy namespace/tier
	// resolution below entirely — provider, model AND effort come from the policy file. It is
	// resolved here, before the kind switch, because a container/scrubbed cell refuses it
	// outright (same ordering as the oracle's apply_model_policy) and --set combined with an
	// active policy is refused as ambiguous before any other flag is even inspected.
	var policyRes *PolicyResolution
	policy, policyAbsPath, err := c.cellModelPolicy()
	if err != nil {
		die("%s", err)
	}
	if policy != nil {
		if persist {
			die("desk: --set with a model policy is ambiguous; edit the policy or provider defaults instead")
		}
		if c.Kind == "container" || c.Kind == "scrubbed" {
			die("model policy currently requires a house or k8s cell; %s cannot apply it", c.Kind)
		}
		policyRes, err = policy.Resolve(role, providerFlag, modelOverride, harnessFlag)
		if err != nil {
			die("%s", err)
		}
		harness = policyRes.Harness
	}

	cfg := ""
	switch c.Kind {
	case "container":
		if cfgIn != "" || provider != "" || c.Env.Get("CELL_PROVIDER") != "" {
			die("container desks do not accept host config directories or providers; configure credentials in the container launcher")
		}
		if !valueIn(role, c.Roles) {
			die("role '%s' is not enabled in this container cell", role)
		}
	case "scrubbed":
		// A scrubbed cell's own config home is what CLAUDE_CONFIG_DIR/CODEX_HOME resolve to
		// below — a host config dir or a provider from the launching shell would defeat the
		// whole point.
		if cfgIn != "" || provider != "" || c.Env.Get("CELL_PROVIDER") != "" {
			die("scrubbed desks do not accept a host config directory or a provider — the cell composes its own")
		}
		if !valueIn(role, c.Roles) {
			die("role '%s' is not enabled in this scrubbed cell", role)
		}
	default:
		cfg = resolveCfg(c.Env, cfgIn)
	}

	// The minimum-Claude-Code-version half of the oracle's policy_claude_preflight (the
	// local/managed settings.json availableModels/modelOverrides conflict scan that same
	// function also runs is NOT ported — see policy.go's header comment and the PR body).
	if policyRes != nil && harness == "claude" {
		if err := checkClaudeMinVersion(); err != nil {
			die("%s", err)
		}
	}

	if policyRes != nil {
		// Native providers (anthropic/codex) carry no CELL_PROVIDER_* credential row of their
		// own — the policy's env/args ARE the credential switch. glm/kimi still resolve through
		// the existing preset/cell.env provider machinery below for BASE_URL/TOKEN_ENV/TOKEN_VAL;
		// only the MODEL comes from the policy, never from the provider's own preset model.
		if policyRes.Provider == "anthropic" || policyRes.Provider == "codex" {
			provider = ""
		} else {
			provider = policyRes.Provider
		}
	} else if provider == "" {
		provider = c.Env.Get("CELL_PROVIDER")
	}
	var prov Provider
	if provider != "" {
		prov = c.resolveProvider(provider)
	}

	// ONE name for both surfaces: the roster beacon (DESK_SESSION) and the session's display
	// name (--name). A house or scrubbed cell stamps the boot time on: its windows are re-booted
	// by hand across days, and the roster / claims should tell one boot from the next.
	short := strings.TrimSuffix(role, "-desk")
	if role == "the-desk" {
		short = "the-desk"
	}
	wt := filepath.Join(c.Dir, "worktrees", role)
	session := c.Name + "-" + short
	if c.Kind == "house" || c.Kind == "scrubbed" {
		session = c.Name + "-" + role + "-" + utcStamp()
	}
	if harness == "codex" {
		session += "-codex"
	}
	if c.Kind != "container" {
		if _, err := os.Stat(filepath.Join(c.Config, "roster.env")); err != nil {
			die("cell home has no roster: %s/roster.env", c.Config)
		}
	}

	// mvar is also what --set persists into, so it must name the ACTIVE harness's own namespace
	// — CODEX_MODEL_<role> on codex, DESK_MODEL_<role> on claude — never the other one.
	mvar := "DESK_MODEL_" + underscore(role)
	if harness == "codex" {
		mvar = "CODEX_MODEL_" + underscore(role)
	}
	var persistKVs []string
	if modelOverride != "" {
		persistKVs = append(persistKVs, mvar+"="+modelOverride)
	}
	if harnessFlag != "" {
		persistKVs = append(persistKVs, "CELL_HARNESS="+harnessFlag)
	}
	if providerFlag != "" {
		persistKVs = append(persistKVs, "CELL_PROVIDER="+providerFlag)
	}
	if c.KindOverride != "" {
		persistKVs = append(persistKVs, "CELL_KIND="+c.KindOverride)
	}
	if cockpitFlag != "" {
		persistKVs = append(persistKVs, "CELL_COCKPIT="+cockpitFlag)
	}

	// A provider is a claude-harness seam (ANTHROPIC_BASE_URL/ANTHROPIC_AUTH_TOKEN); codex has
	// no equivalent here, so the combination is refused rather than silently launching codex
	// against Anthropic with a provider the operator asked for.
	if harness == "codex" && provider != "" {
		die("desk: --provider '%s' is a claude-harness switch and has no codex equivalent — drop the provider or use --harness claude", provider)
	}

	var model string
	resolvedSrc := ""
	switch {
	case policyRes != nil:
		// The policy already resolved provider, model AND effort (including its own the-desk
		// non-Opus rule and deny-list check) — never run through the legacy namespace/tier
		// resolution below.
		model = policyRes.Model
		resolvedSrc = "policy:" + policyAbsPath + "@" + policyRes.PolicySHA256
	case modelOverride != "":
		// An explicit --model (or DESK_MODEL_OVERRIDE) passes through VERBATIM to the selected
		// harness — never run through the namespace/tier resolution, on either harness.
		model = modelOverride
	case provider != "" && c.Env.Get(mvar) == "" && prov.Model != "":
		// A provider's own model is what a provider window runs when neither --model nor a
		// PER-ROLE pin names one: the harness-wide default / tier fallback are Anthropic names
		// the provider's endpoint would reject.
		model = prov.Model
		resolvedSrc = "provider:" + provider + " (" + providerVar(provider, "MODEL") + ")"
	default:
		rm := c.resolveRoleModel(role, harness)
		if !rm.OK {
			die("desk: %s — set %s in cell.env, or the harness default, or a tier fallback (docs/cellctl.md Pinned models), or pass --model explicitly", rm.Src, mvar)
		}
		model, resolvedSrc = rm.Model, rm.Src
	}
	// The Opus refusal binds the RESOLVED model, override or pin alike, on the CLAUDE arm ONLY:
	// an override is not an escape hatch from it, but Opus is a Claude-family alias with no
	// meaning to codex, where this refuses nothing. Under a policy this is also already enforced
	// by ModelPolicy.Resolve; re-checking here is a no-op, never a conflicting second opinion.
	if role == "the-desk" && harness == "claude" {
		refuseOpusForTheDesk(model)
	}

	dryRun := c.Env.Get("DRY_RUN") == "1"

	if c.Kind == "container" {
		if persist && !dryRun {
			applyEnvKVs(c.Env, filepath.Join(c.Dir, "cell.env"), false, persistKVs)
		}
		c.containerRun("desk", role, "--harness", harness, "--model", model)
		return
	}

	// modelDisp is what every line below PRINTS: the resolved model, plus "(override)" when
	// --model/DESK_MODEL_OVERRIDE is the reason, or the tier/provider source when the namespace
	// pin fell all the way through — the harness itself always gets the bare model.
	// Mirrors the oracle exactly, including its quirk under a policy: an explicit --model/
	// DESK_MODEL_OVERRIDE still displays "(override)" even though the value was actually
	// resolved (tier/alias/exact-ID, deny-checked) through the policy, because model_override
	// is never cleared once apply_model_policy consumes it as the "requested" argument.
	modelDisp := model
	if modelOverride != "" {
		modelDisp = model + " (override)"
	} else if strings.HasPrefix(resolvedSrc, "tier:") || strings.HasPrefix(resolvedSrc, "provider:") || strings.HasPrefix(resolvedSrc, "policy:") {
		modelDisp = model + " (" + resolvedSrc + ")"
	}

	// providerDisp/effortDisp are what the [dry-run]/[launch] lines print for provider/effort —
	// the REAL policy provider name (never blanked, unlike `provider` above, which is blanked for
	// anthropic/codex so the legacy credential-resolution block below is skipped for them) and
	// the policy's resolved effort, or "harness-default" with no policy (docs/cellctl-model-policy.md
	// "Inspect, launch and verify adoption").
	providerDisp, effortDisp := provider, "harness-default"
	if policyRes != nil {
		providerDisp, effortDisp = policyRes.Provider, policyRes.Effort
	}

	deskRoots := c.Env.Get("CELL_ROOTS")
	if deskRoots != "" {
		if !rootsValid(deskRoots) {
			die("cell.env: CELL_ROOTS is malformed")
		}
		if c.Env.Get("CELL_FF_ROOTS") == "1" && !dryRun {
			c.ffRoots()
		}
	} else {
		fmt.Fprintln(os.Stderr, "NOTICE: cell.env has no CELL_ROOTS — the window boots WITHOUT DESK_ROOTS (desk verbs fall back to their compiled placeholder topology)")
	}

	// The cell's ONE cockpit value, exported as ASSAY_COCKPIT into the window so the worker-desk
	// skill's worktree-create step cuts dispatched worktrees with the same cockpit the windows are
	// hosted in (tmux = the plain `git worktree add` arm). Resolved exactly as `up` resolves it —
	// --cockpit beats cell.env CELL_COCKPIT beats the `auto` default — and always to a CONCRETE
	// value: `auto` never reaches the window, so the skill's PATH-order autodetect runs only where
	// no cell launcher exported anything. An explicit cockpit that is not available is refused
	// here, as `up` refuses it, never exported as some other value. A scrubbed cell composes its
	// own environment and opens no cockpit, so it carries none.
	var cockpit cockpitResolution
	if c.Kind != "scrubbed" {
		want, src := c.cockpitWant(cockpitFlag)
		cockpit = c.resolveCockpit(want, src)
		if cockpit.Err != "" {
			die("desk: %s — it is exported to the window as ASSAY_COCKPIT, so an unavailable explicit choice is refused rather than replaced; install it, or pass --cockpit tmux (or set CELL_COCKPIT)", cockpit.Err)
		}
	}

	if dryRun {
		kindShown := c.Kind
		if c.KindOverride != "" {
			kindShown += " (override)"
		}
		fmt.Printf("[dry-run] cell=%s kind=%s role=%s model=%s effort=%s provider=%s harness=%s cfg=%s wt=%s session=%s desk_roots=%s shims→HOME=%s\n",
			c.Name, kindShown, role, modelDisp, effortDisp, orDefault(providerDisp, "anthropic"), harness, cfg, wt, session,
			orDefault(deskRoots, "unset"), c.Home)
		// When the repair-admission opt-in is on, show it composed into the launch — the exact
		// KEY=VALUE the child process (and, through it, deskdispatch) will carry. Off/unset is
		// absent here, matching the composed environment (deskLaunch omits it).
		if rav := c.repairAdmissionValue(); rav != "" {
			fmt.Printf("[dry-run] env %s=%s (repair-admission dispatch gate opt-in)\n", deskkit.EnvRepairAdmission, rav)
		}
		if cockpit.Cockpit != "" {
			fmt.Printf("[dry-run] env %s=%s (%s)\n", envAssayCockpit, cockpit.Cockpit, cockpit.Why)
		}
		if harness == "claude" && c.Kind != "scrubbed" {
			fmt.Println("[dry-run] env CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false")
		}
		if persist {
			for _, kv := range persistKVs {
				fmt.Printf("[dry-run] --set: would persist %s into %s/cell.env (not written — dry run)\n", kv, c.Dir)
			}
		}
		if c.Kind == "scrubbed" {
			// The [dry-run] line above stays — this ADDS the plan grammar the scrubbed-cell
			// brief defines and the parity harness diffs against.
			env := c.scrubbedComposeEnv(role, harness, session)
			c.printPlan(env, wt, harnessArgv(harness, role, model, session, wt))
		}
		return
	}

	c.deskLaunch(role, harness, model, modelDisp, session, wt, cfg, provider, prov, deskRoots, persist, persistKVs, policyRes, cockpit)
}

func needFlagValue(args []string, i *int, msg string) string {
	if *i+1 >= len(args) || args[*i+1] == "" {
		die("%s", msg)
	}
	*i++
	return args[*i]
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func utcStamp() string { return time.Now().UTC().Format("20060102T150405Z") }

// resolveCfg is the host CLAUDE_CONFIG_DIR a k8s or house window keeps: the positional if given,
// else $CLAUDE_CONFIG_DIR, else $HOME/.claude. It must be a directory.
func resolveCfg(e *Env, in string) string {
	if in == "" {
		in = e.Get("CLAUDE_CONFIG_DIR")
	}
	if in == "" {
		in = filepath.Join(e.Get("HOME"), ".claude")
	}
	st, err := os.Stat(in)
	if err != nil || !st.IsDir() {
		die("CLAUDE_CONFIG_DIR not a directory: %s", in)
	}
	abs, err := filepath.Abs(in)
	if err != nil {
		die("CLAUDE_CONFIG_DIR not a directory: %s", in)
	}
	return abs
}

// anyGiven reports whether any of the per-run overrides was actually given. `--set` persists
// only what an invocation NAMED — a cell's own values are never re-written by a `--set` that did
// not name them — so "nothing to persist" is a refusal, not a silent no-op.
func anyGiven(vals ...string) bool {
	for _, v := range vals {
		if v != "" {
			return true
		}
	}
	return false
}
