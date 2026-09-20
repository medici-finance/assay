package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

const showUsage = "cellctl show <cell> [--kind <k>] [--cockpit <c>] [--harness <h>] [--provider <p>] [--model <m>]"

// cmdShow is a READ: the EFFECTIVE value of every per-run choice, one greppable
// `[show] KEY=VALUE (source)` line each, where source is `flag` (given on this invocation),
// `cell.env` (the file's own line) or `default` (the compiled fallback) — plus one
// `[show] model <role>=<m> (source)` line per role, the same resolution a plain `desk` boot
// would make on the shown harness.
//
// It launches nothing and writes nothing. The same flags `desk`/`up` take are accepted so "what
// would THIS invocation resolve to" is answerable BEFORE booting it; a `--kind` the cell is not
// provisioned for refuses exactly as `desk` would.
func cmdShow(cell string, args []string) {
	kindOverride := prescanKindOverride(args)
	c := loadCellWithKind(cell, kindOverride)
	envfile := filepath.Join(c.Dir, "cell.env")

	harnessFlag, cockpitFlag, providerFlag, modelFlag := "", "", "", ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--kind":
			i++
		case "--cockpit":
			cockpitFlag = needFlagValue(args, &i, "--cockpit needs a value ("+joinPipe(cockpitValues)+")")
		case "--harness":
			harnessFlag = needFlagValue(args, &i, "--harness needs a value ("+joinPipe(harnessValues)+")")
		case "--provider":
			providerFlag = needFlagValue(args, &i, "--provider needs a value")
		case "--model":
			modelFlag = needFlagValue(args, &i, "--model needs a value")
		default:
			if strings.HasPrefix(a, "--") {
				die("show: unknown flag %s", a)
			}
			die("show: unexpected argument '%s'", a)
		}
	}
	if harnessFlag != "" && !valueIn(harnessFlag, harnessValues) {
		die("show: --harness must be one of %s, got '%s'", joinPipe(harnessValues), harnessFlag)
	}
	if cockpitFlag != "" && !valueIn(cockpitFlag, cockpitValues) {
		die("show: --cockpit must be one of %s, got '%s'", joinPipe(cockpitValues), cockpitFlag)
	}

	// showLine: source is flag > cell.env line > default.
	showLine := func(key, flag, eff string) {
		src := "default"
		switch {
		case flag != "":
			src = "flag"
		default:
			if _, ok := envFileValue(envfile, key); ok {
				src = "cell.env"
			}
		}
		fmt.Printf("[show] %s=%s (%s)\n", key, eff, src)
	}

	fmt.Printf("[show] cell=%s dir=%s\n", c.Name, c.Dir)
	showLine("CELL_KIND", c.KindOverride, c.Kind)
	want, _ := c.cockpitWant(cockpitFlag)
	showLine("CELL_COCKPIT", cockpitFlag, want)
	harness := harnessFlag
	if harness == "" {
		harness = c.Harness
	}
	showLine("CELL_HARNESS", harnessFlag, harness)
	provider := providerFlag
	if provider == "" {
		provider = c.Env.Get("CELL_PROVIDER")
	}
	if provider != "" {
		showLine("CELL_PROVIDER", providerFlag, provider)
		// The provider's effective endpoint, token env NAME (+ set/unset — never its value) and
		// model, each tagged cell.env / preset / unset.
		pb, pbs := c.providerValue(provider, "BASE_URL")
		pt, pts := c.providerValue(provider, "TOKEN_ENV")
		pm, pms := c.providerValue(provider, "MODEL")
		tokstate := "unset"
		if pt != "" && c.Env.Get(pt) != "" {
			tokstate = "set"
		}
		fmt.Printf("[show] provider %s base_url=%s (%s)\n", provider, orDefault(pb, "<unset>"), pbs)
		fmt.Printf("[show] provider %s token_env=%s (%s; %s in this shell)\n", provider, orDefault(pt, "<unset>"), pts, tokstate)
		fmt.Printf("[show] provider %s model=%s (%s)\n", provider, orDefault(pm, "<unset>"), pms)
	} else {
		fmt.Printf("[show] CELL_PROVIDER=%s (%s)\n", "unset", "default: anthropic")
	}
	for _, r := range c.Roles {
		if modelFlag != "" {
			fmt.Printf("[show] model %s=%s (flag)\n", r, modelFlag)
			continue
		}
		rm := c.resolveRoleModel(r, harness)
		if !rm.OK {
			fmt.Printf("[show] model %s=%s (%s)\n", r, "unresolved", rm.Src)
			continue
		}
		src := ""
		if strings.HasPrefix(rm.Src, "tier:") {
			src = "default: " + rm.Src
		} else if _, ok := envFileValue(envfile, rm.Src); ok {
			src = "cell.env " + rm.Src
		} else {
			src = "default: " + rm.Src
		}
		fmt.Printf("[show] model %s=%s (%s)\n", r, rm.Model, src)
	}
}
