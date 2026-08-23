// support.go — the small shared surface the cross-domain reducer needs: the
// grouping order, the domains config reader and its refusals, date resolution,
// and the "owner/repo" spec helpers.
//
// The taxonomy — which groupings exist, in what order, and which one is the
// org-wide grouping — is NOT compiled into this tool. It is declared entirely
// in the domains config file (see domains.example.yaml for the schema) and
// resolved at load time. Nothing in this file shells out, reads a token, or
// talks to GitHub.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// groupingOrder is the deterministic iteration order over every grouping the
// loaded config declares — the product groupings in their configured order,
// followed by the org grouping if one is configured. It is empty until
// loadConfig has run and is populated from the config file, never hard-coded.
var groupingOrder []string

// productGroupings is the configured order of the PRODUCT groupings only — the
// org grouping is deliberately excluded and handled as a distinct top-level
// section (it is org-wide and is never summed into the product total). Empty
// until loadConfig populates it from the config file.
var productGroupings []string

// orgGrouping is the name of the single org-wide grouping, or "" when the
// config declares none. Populated from the config file by loadConfig.
var orgGrouping string

// domainsConfig is the parsed domains config file. The grouping names, their
// order, and which grouping is the org-wide one all come from the file.
type domainsConfig struct {
	// Products is the ordered list of product grouping names. Each is summed
	// into the all-products total, in this order.
	Products []string `yaml:"products"`
	// Org is the name of the single org-wide grouping — reported as a distinct
	// section and never summed into the product total. Optional; "" means the
	// config declares no org grouping.
	Org string `yaml:"org"`
	// Groupings maps every declared grouping name (each product, plus org when
	// set) to its list of "owner/repo" specs.
	Groupings map[string][]string `yaml:"groupings"`
}

func main() {
	args := os.Args[1:]
	// The workflow invokes `go run . aggregate …`; the subcommand word is
	// accepted (and is the documented form) but optional, since aggregation
	// is now the only thing this module does.
	if len(args) > 0 && args[0] == "aggregate" {
		args = args[1:]
	}
	os.Exit(runAggregate(args))
}

// defaultConfigPath locates domains.yaml alongside this source file, so the
// default works whether invoked as `go run .` from within
// tools/metrics-harvest or as `go run ./tools/metrics-harvest` from the repo
// root. The public tree ships no domains.yaml — the real roster is a private
// config provisioned to this path at runtime (see domains.example.yaml for the
// schema); the default only resolves when that file is present.
func defaultConfigPath() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "domains.yaml"
	}
	return filepath.Join(filepath.Dir(thisFile), "domains.yaml")
}

// defaultRoot derives the repo root from the config file's location: the config
// lives at <root>/tools/metrics-harvest/<config>, so the root is three
// directories up. This keeps reports/daily/ pinned at the repo root regardless
// of the caller's cwd.
func defaultRoot(cfgPath string) string {
	abs, err := filepath.Abs(cfgPath)
	if err != nil {
		return "."
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(abs)))
}

// loadConfig reads and validates the domains config, then publishes the
// resolved taxonomy into the package-level groupingOrder / productGroupings /
// orgGrouping. It REFUSES rather than reduce a config it cannot honestly honor.
func loadConfig(path string) (domainsConfig, error) {
	var cfg domainsConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("cannot read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("cannot parse %s: %w", path, err)
	}

	if len(cfg.Products) == 0 {
		return cfg, fmt.Errorf("%s: no product groupings declared (the `products:` list is required)", path)
	}

	// Resolve the iteration order from the file: products in declared order,
	// then org if one is named.
	order := append([]string(nil), cfg.Products...)
	if cfg.Org != "" {
		order = append(order, cfg.Org)
	}

	// Every named grouping must carry a roster entry (an empty list is allowed
	// — it renders not-configured — but the key must be present, so a typo
	// dropping the list is caught rather than read as a silent zero).
	seenName := map[string]bool{}
	for _, g := range order {
		if seenName[g] {
			return cfg, fmt.Errorf("%s: grouping %q is declared more than once", path, g)
		}
		seenName[g] = true
		if _, ok := cfg.Groupings[g]; !ok {
			return cfg, fmt.Errorf("%s: grouping %q is declared but has no `groupings:` roster entry", path, g)
		}
	}

	if err := validateConfig(cfg, order); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}

	groupingOrder = order
	productGroupings = append([]string(nil), cfg.Products...)
	orgGrouping = cfg.Org
	return cfg, nil
}

// validateConfig REFUSES a config the reducer cannot honestly reduce, rather
// than reducing it and publishing a wrong number as checked-clean. Three
// refusals, each from a measured defect in the prior reducer:
//
//   - a malformed spec (not "owner/repo"): there is no key to match a
//     snapshot entry on, so the repo would silently never be read;
//   - a DUPLICATE spec (defect 6): listing one repo twice inflated
//     every figure — five of them, including a re-weighted lead time — and
//     reported `reposWithGaps: []`, i.e. checked-clean;
//   - a BASE-NAME COLLISION within a grouping (defect 7):
//     `example-org/alpha` + `example-net/alpha` are two different repos that
//     the old base-name keying collapsed into one, double-counting the first
//     and never reading the second, again reporting checked-clean. This
//     reducer keys on the FULL spec so the collapse cannot happen, and still
//     refuses the collision, because a base name is what every human-readable
//     surface (table rows, gap lists) labels a repo with — an ambiguous label
//     is a wrong label.
//
// Every violation is reported, not just the first: a config fixed one error
// at a time is a config re-run four times.
func validateConfig(cfg domainsConfig, order []string) error {
	var problems []string

	seenSpec := map[string]string{} // spec -> grouping it was first seen in
	for _, g := range order {
		seenBase := map[string]string{} // base name -> spec it was first seen in
		for _, spec := range cfg.Groupings[g] {
			s := strings.TrimSpace(spec)
			if s == "" || strings.Count(s, "/") != 1 ||
				strings.HasPrefix(s, "/") || strings.HasSuffix(s, "/") {
				problems = append(problems, fmt.Sprintf(
					"grouping %q: %q is not an \"owner/repo\" spec", g, spec))
				continue
			}
			if first, dup := seenSpec[s]; dup {
				if first == g {
					problems = append(problems, fmt.Sprintf(
						"grouping %q: %q is listed twice — a duplicate inflates every figure "+
							"it contributes to and leaves no gap signal (defect 6)", g, s))
				} else {
					problems = append(problems, fmt.Sprintf(
						"%q appears in both grouping %q and grouping %q — it would be counted "+
							"twice in the all-products total (defect 6)", s, first, g))
				}
				continue
			}
			seenSpec[s] = g

			base := repoBaseName(s)
			if other, clash := seenBase[base]; clash {
				problems = append(problems, fmt.Sprintf(
					"grouping %q: %q and %q share the base name %q — an ambiguous label for two "+
						"different repos (defect 7)", g, other, s, base))
				continue
			}
			seenBase[base] = s
		}
	}

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("refusing to aggregate — %d config problem(s):\n  - %s",
		len(problems), strings.Join(problems, "\n  - "))
}

// resolveDate parses --date, or defaults to YESTERDAY (UTC) — the previous
// complete day, matching what the workflow's harvest legs stamp into their
// snapshots. UTC avoids timezone skew between a local run and CI.
func resolveDate(dateStr string) (string, error) {
	if dateStr == "" {
		return time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02"), nil
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return "", fmt.Errorf("--date %q must be YYYY-MM-DD", dateStr)
	}
	return t.Format("2006-01-02"), nil
}

// previousDate returns the calendar day before dateLabel, which must already
// be a valid YYYY-MM-DD (resolveDate guarantees that for every caller).
func previousDate(dateLabel string) (string, error) {
	t, err := time.Parse("2006-01-02", dateLabel)
	if err != nil {
		return "", fmt.Errorf("internal: unparseable date label %q: %w", dateLabel, err)
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02"), nil
}

// repoBaseName returns the repo segment of an "owner/repo" spec. It is used
// ONLY for human-readable labels and for the collision refusal above — never
// as a lookup key. Keying on it is defect 7.
func repoBaseName(spec string) string {
	parts := strings.Split(spec, "/")
	return parts[len(parts)-1]
}
