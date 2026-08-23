package main

// gitleaks engine — MIT-licensed CLI binary, invoked as an external process.
//
// NOT the gitleaks GitHub Action: that is license-key-gated for organisations, and adopting
// it would put a paid key between this repo and its own security gate. The CLI
// is MIT and is all that is used. Nothing in this file imports, vendors, or
// copies gitleaks code; it writes a config, runs a binary, and reads JSON.
//
// Two invocations per run, deliberately separate:
//
//   - the HOUSE pass: our compiled rules ONLY (`[extend] useDefault = false`),
//     over the projection. Keeping the stock rules out of this pass means a house
//     rule's result is never confounded with a stock rule's, so "which rules
//     fired" answers the liveness question cleanly.
//   - the STOCK pass (scanStock): gitleaks' own default rule set, over the canary
//     fixture. This is the only thing that can prove the ecosystem rule set —
//     the entire reason for adopting gitleaks — is actually running.
//
// The exit code is used as a THREE-state signal, not two: `--exit-code 9` makes
// "leaks found" distinguishable from "the binary failed". 0 and 9 both mean the
// scan completed; anything else means it did not, and a scan that did not
// complete is could-not-check.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type gitleaksEngine struct{ binOverride string }

func (g *gitleaksEngine) name() string { return "gitleaks" }

// gitleaksLeaksExit is the exit code we ask gitleaks to use when it finds
// something, chosen well away from the codes a failed exec produces so "found
// leaks" can never be confused with "could not run".
const gitleaksLeaksExit = 9

func (g *gitleaksEngine) locate() (string, string, error) {
	bin, err := resolveBinary(g.binOverride, "LEAKSWEEP_GITLEAKS_BIN", "gitleaks")
	if err != nil {
		return "", "", err
	}
	v, err := probeVersion(bin, "version")
	if err != nil {
		return "", "", fmt.Errorf("gitleaks at %s did not answer a version probe: %v", bin, err)
	}
	return bin, v, nil
}

// gitleaksReport is the subset of gitleaks' JSON finding we consume. Extra fields
// are ignored by encoding/json, so a gitleaks upgrade that ADDS fields does not
// break the parse; one that RENAMES these is caught by the liveness check, which
// would see zero rules fire on the sample fixture and refuse to certify.
type gitleaksReport struct {
	RuleID    string `json:"RuleID"`
	File      string `json:"File"`
	StartLine int    `json:"StartLine"`
	Match     string `json:"Match"`
}

func (g *gitleaksEngine) scan(bin, dir string, rules []rule) ([]finding, error) {
	cfg, err := os.CreateTemp("", "leaksweep-gitleaks-*.toml")
	if err != nil {
		return nil, err
	}
	defer os.Remove(cfg.Name())
	if _, err := cfg.WriteString(gitleaksConfig(rules)); err != nil {
		cfg.Close()
		return nil, err
	}
	cfg.Close()
	return g.run(bin, dir, cfg.Name(), false)
}

func (g *gitleaksEngine) scanStock(bin, dir string) ([]string, error) {
	// No -c at all: gitleaks uses its embedded default config, which is exactly
	// what the canary must prove is live.
	f, err := g.run(bin, dir, "", true)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var ids []string
	for _, x := range f {
		if !seen[x.ruleID] {
			seen[x.ruleID] = true
			ids = append(ids, x.ruleID)
		}
	}
	return ids, nil
}

func (g *gitleaksEngine) run(bin, dir, cfgPath string, stock bool) ([]finding, error) {
	rep, err := os.CreateTemp("", "leaksweep-gitleaks-report-*.json")
	if err != nil {
		return nil, err
	}
	rep.Close()
	defer os.Remove(rep.Name())

	args := []string{"dir", dir, "--no-banner", "--exit-code", fmt.Sprint(gitleaksLeaksExit),
		"-f", "json", "-r", rep.Name()}
	if cfgPath != "" {
		args = append(args, "-c", cfgPath)
	}
	cmd := exec.Command(bin, args...)
	out, runErr := cmd.CombinedOutput()
	code := cmd.ProcessState.ExitCode()
	// 0 = ran, nothing found. gitleaksLeaksExit = ran, findings. Anything else is
	// a scanner that did not complete — and a scan that did not complete is
	// could-not-check, never an empty clean result.
	if code != 0 && code != gitleaksLeaksExit {
		return nil, fmt.Errorf("gitleaks exited %d (not 0 or %d): %v: %s",
			code, gitleaksLeaksExit, runErr, strings.TrimSpace(lastLines(string(out), 6)))
	}
	raw, err := os.ReadFile(rep.Name())
	if err != nil {
		return nil, fmt.Errorf("gitleaks report unreadable: %v", err)
	}
	var reports []gitleaksReport
	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &reports); err != nil {
			// A report we cannot read is a scan we did not observe.
			return nil, fmt.Errorf("gitleaks report did not parse as JSON: %v", err)
		}
	}
	var out2 []finding
	for _, r := range reports {
		p := r.File
		if abs, err := filepath.Rel(dir, r.File); err == nil && !strings.HasPrefix(abs, "..") {
			p = abs
		}
		out2 = append(out2, finding{
			engine: "gitleaks", ruleID: r.RuleID, path: filepath.ToSlash(p),
			line: r.StartLine, match: r.Match,
		})
	}
	return out2, nil
}

// gitleaksConfig renders the engine-neutral rules as a gitleaks TOML config.
//
// `[extend] useDefault = false` is explicit and load-bearing: the house pass must
// carry house rules and nothing else, so that "rule X did not fire" is an
// unambiguous statement about rule X. The stock rule set gets its own pass.
func gitleaksConfig(rules []rule) string {
	var b strings.Builder
	b.WriteString("# GENERATED by tools/leaksweep — do not edit, do not commit.\n")
	b.WriteString("# Written to a temp file at run time because it contains the withheld\n")
	b.WriteString("# token list in plaintext; the list itself is do-not-copy.\n")
	b.WriteString("title = \"leaksweep house rules\"\n\n")
	b.WriteString("[extend]\nuseDefault = false\n\n")
	for _, r := range rules {
		fmt.Fprintf(&b, "[[rules]]\nid = %q\ndescription = %q\nregex = '''%s'''\n",
			r.id, r.kind+" rule for one house token entry", r.regex)
		if len(r.excepts) > 0 {
			// regexTarget = "match" applies the allowlist to the MATCHED TEXT, not
			// the line. That distinction is the whole feature: a sanctioned form and
			// a bare occurrence on the SAME LINE must produce one suppression and
			// one finding, which a line-targeted allowlist could not do — it would
			// swallow both and certify a leak clean.
			b.WriteString("[rules.allowlist]\nregexTarget = \"match\"\nregexes = [\n")
			for _, x := range r.excepts {
				fmt.Fprintf(&b, "  '''^%s$''',\n", regexp.QuoteMeta(x))
			}
			b.WriteString("]\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// resolveBinary picks the binary path from, in order: an explicit flag, an
// environment override, then PATH. It returns an ERROR when nothing is found —
// never a silent "engine disabled", because a disabled engine that still lets the
// run certify is the entire failure mode this tool exists to prevent.
func resolveBinary(override, envVar, name string) (string, error) {
	if override != "" {
		if _, err := os.Stat(override); err != nil {
			return "", fmt.Errorf("%s binary %s: %v", name, override, err)
		}
		return override, nil
	}
	if v := os.Getenv(envVar); v != "" {
		if _, err := os.Stat(v); err != nil {
			return "", fmt.Errorf("%s binary from %s (%s): %v", name, envVar, v, err)
		}
		return v, nil
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found on PATH and no %s set — an engine that is not present cannot certify anything", name, envVar)
	}
	return p, nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
