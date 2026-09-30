package main

// instrument-audit (the liveness balancing loop of this stream's three-loops brief) — the
// actuator-side counterpart to lint-audit.
//
// lint-audit (statusgen/01) asks the same question of a statusgen LINT RULE that this
// asks of a statusgen FLAG: is anything downstream still consuming it? A systems read
// found statusgen had grown a long tail of flags — opmetrics views, the
// drive dashboard, the ladder's operator steps — referenced by no workflow, skill,
// script or tool: sensors with no actuator. `--instrument-audit` makes that tail
// VISIBLE and, in --lint, NOTICE-able, so the retire block (part 3) has a reading to
// consume. It never removes a flag: retirement is a human judgement, exactly as
// lint-audit never retires a rule.
//
// For every flag declared in statusgen/main.go it reports the set of CONSUMERS found by
// grepping the given roots (default: this repo's .github/workflows, plugins/assay/skills,
// plugins/assay/scripts, Makefile, tools/desk/cmd; a house root adds its own
// workflows/skills/tools). The verdict is three-state, never green-by-omission:
//
//	WIRED  — >=1 consumer references `--<flag>`.
//	COLD   — 0 consumers.
//	DARK   — 0 consumers AND the flag was declared more than 30 days ago (git log -S),
//	         i.e. it has had time to acquire a consumer and did not. Only a DARK flag is
//	         a retirement candidate; a COLD flag may simply be new.
//
// The age check (git log -S) runs ONLY for the flags that are already COLD, so a run
// pays it for the handful of unwired flags, never for the whole set.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// instrumentState is a flag's three-state liveness verdict.
type instrumentState string

const (
	instrumentWired instrumentState = "WIRED"
	instrumentCold  instrumentState = "COLD"
	instrumentDark  instrumentState = "DARK"
)

// darkAgeDays is the threshold past which a consumer-less flag is DARK rather than
// merely COLD. It mirrors lint-audit's 30-day window: a flag that has had a month with
// no consumer has had time to acquire one.
const darkAgeDays = 30

// instrumentFlag is one flag's audit row.
type instrumentFlag struct {
	Name string `json:"name"`
	// State is WIRED / COLD / DARK.
	State instrumentState `json:"state"`
	// Consumers are the consumer-root-relative paths that reference `--<Name>`, sorted.
	Consumers []string `json:"consumers"`
	// DeclaredAgeDays is how long ago the flag was declared in main.go, per git log -S.
	// It is -1 when the age could not be read (not a git tree, or the flag string was
	// not found in history) — could-not-check, never rounded to 0 or to "recent".
	DeclaredAgeDays int `json:"declared_age_days"`
}

// instrumentAuditReport is the whole audit.
type instrumentAuditReport struct {
	// Flags is one row per declared flag, sorted by name.
	Flags []instrumentFlag `json:"flags"`
	// ConsumerRoots are the directories that were grepped for consumers.
	ConsumerRoots []string `json:"consumer_roots"`
	// Wired / Cold / Dark are the tallies, for a one-line summary.
	Wired int `json:"wired"`
	Cold  int `json:"cold"`
	Dark  int `json:"dark"`
}

// instrumentAuditConfig carries the audit's injectable edges. A nil field is filled with
// the real one by withDefaults; the test supplies its own so the suite is hermetic (no
// git, no real repo tree).
type instrumentAuditConfig struct {
	root string
	// flagFile is the source read for flag declarations (default <root>/statusgen/main.go).
	flagFile string
	// consumerRoots are the directories grepped for `--<flag>` references.
	consumerRoots []string
	// flagNames parses the declared flag names out of the flag source file.
	flagNames func(flagFile string) ([]string, error)
	// consumerFiles returns path->content for every file under the consumer roots, so the
	// grep runs once in memory rather than re-walking the tree per flag.
	consumerFiles func(roots []string) (map[string]string, error)
	// declAgeDays returns how many days ago the named flag was declared, and whether that
	// could be read at all. Consulted ONLY for a flag that is already COLD.
	declAgeDays func(root, flagFile, name string) (days int, known bool)
}

func (c instrumentAuditConfig) withDefaults() instrumentAuditConfig {
	if c.flagFile == "" {
		c.flagFile = filepath.Join(c.root, "statusgen", "main.go")
	}
	if len(c.consumerRoots) == 0 {
		c.consumerRoots = defaultInstrumentConsumerRoots(c.root)
	}
	if c.flagNames == nil {
		c.flagNames = parseFlagNames
	}
	if c.consumerFiles == nil {
		c.consumerFiles = readConsumerFiles
	}
	if c.declAgeDays == nil {
		c.declAgeDays = gitDeclAgeDays
	}
	return c
}

// splitInstrumentRoots parses the --roots comma-separated override into a slice, trimming
// blanks and dropping empties. An empty value yields nil, which withDefaults reads as "use
// the day-one consumer surface".
func splitInstrumentRoots(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(csv, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// defaultInstrumentConsumerRoots is the day-one consumer surface: the places a statusgen
// flag is actually invoked from. A directory that does not exist is simply grepped empty.
func defaultInstrumentConsumerRoots(root string) []string {
	return []string{
		filepath.Join(root, ".github", "workflows"),
		filepath.Join(root, "plugins", "assay", "skills"),
		filepath.Join(root, "plugins", "assay", "scripts"),
		filepath.Join(root, "Makefile"),
		filepath.Join(root, "tools", "desk", "cmd"),
	}
}

// flagDeclRe matches a global flag declaration and captures its NAME. The typed
// constructors (Bool/String/Int/...) carry the name as the first string literal; Var
// carries it as the SECOND argument, after the value pointer. `fs.` (subcommand) forms
// are deliberately NOT matched — the audit's subject is statusgen's top-level CLI flags.
var (
	flagDeclRe = regexp.MustCompile(`\bflag\.(?:Bool|String|Int|Int64|Uint|Uint64|Float64|Duration)\(\s*"([^"]+)"`)
	flagVarRe  = regexp.MustCompile(`\bflag\.Var\([^,]+,\s*"([^"]+)"`)
)

// parseFlagNames extracts the declared global flag names from the flag source file, sorted
// and de-duplicated.
func parseFlagNames(flagFile string) ([]string, error) {
	raw, err := os.ReadFile(flagFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read the flag source %s: %w", flagFile, err)
	}
	seen := map[string]bool{}
	var names []string
	for _, m := range flagDeclRe.FindAllStringSubmatch(string(raw), -1) {
		if n := m[1]; n != "" && !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	for _, m := range flagVarRe.FindAllStringSubmatch(string(raw), -1) {
		if n := m[1]; n != "" && !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names, nil
}

// readConsumerFiles walks every consumer root and returns path->content. A root that is a
// single file (Makefile) is read directly; a directory is walked. A missing root is not an
// error — it simply contributes nothing (a consumer surface a repo has not adopted yet).
func readConsumerFiles(roots []string) (map[string]string, error) {
	out := map[string]string{}
	for _, r := range roots {
		fi, err := os.Stat(r)
		if err != nil {
			continue // absent consumer surface: grepped empty, not an error
		}
		if !fi.IsDir() {
			if b, rerr := os.ReadFile(r); rerr == nil {
				out[r] = string(b)
			}
			continue
		}
		werr := filepath.WalkDir(r, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // an unreadable entry is skipped, not fatal
			}
			if d.IsDir() {
				return nil
			}
			if b, rerr := os.ReadFile(path); rerr == nil {
				out[path] = string(b)
			}
			return nil
		})
		if werr != nil {
			return nil, werr
		}
	}
	return out, nil
}

// consumerMatcher builds the token matcher for one flag: `--<name>` NOT followed by a
// letter, digit or dash, so `--budget` never matches `--budget-spec` and `--net` never
// matches `--net-flow`.
func consumerMatcher(name string) *regexp.Regexp {
	return regexp.MustCompile(`--` + regexp.QuoteMeta(name) + `([^A-Za-z0-9-]|$)`)
}

// gitDeclAgeDays reports how many days ago the flag's name string was first introduced into
// the flag source file, via `git log -S`. It is read-only. It returns known=false (a
// could-not-check) when the tree is not a git repo or the string is not found in history —
// never a fabricated 0.
func gitDeclAgeDays(root, flagFile, name string) (int, bool) {
	rel, err := filepath.Rel(root, flagFile)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = flagFile
	}
	// -S with the quoted name finds commits that changed the count of `"<name>"`; the
	// LAST line (git log is newest-first) is the introducing commit.
	cmd := exec.Command("git", "-C", root, "log", "-S", `"`+name+`"`, "--format=%ct", "--", rel)
	out, err := cmd.Output()
	if err != nil {
		return -1, false
	}
	lines := strings.Fields(strings.TrimSpace(string(out)))
	if len(lines) == 0 {
		return -1, false
	}
	oldest := lines[len(lines)-1]
	unix, err := strconv.ParseInt(oldest, 10, 64)
	if err != nil {
		return -1, false
	}
	days := int(time.Since(time.Unix(unix, 0)).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return days, true
}

// instrumentAudit computes the report from a config. It never returns an error for a flag
// it could not fully classify — an unread age is DeclaredAgeDays=-1 and keeps the flag COLD
// (a flag whose age we cannot read is not a proven retirement candidate) — but it DOES
// return an error when the flag source itself cannot be read, because an audit that read no
// flags proved nothing.
func instrumentAudit(cfg instrumentAuditConfig) (instrumentAuditReport, error) {
	cfg = cfg.withDefaults()
	names, err := cfg.flagNames(cfg.flagFile)
	if err != nil {
		return instrumentAuditReport{}, err
	}
	files, err := cfg.consumerFiles(cfg.consumerRoots)
	if err != nil {
		return instrumentAuditReport{}, fmt.Errorf("cannot read the consumer roots: %w", err)
	}

	rep := instrumentAuditReport{ConsumerRoots: cfg.consumerRoots}
	for _, name := range names {
		re := consumerMatcher(name)
		var consumers []string
		for path, content := range files {
			if re.MatchString(content) {
				consumers = append(consumers, relOrSelf(cfg.root, path))
			}
		}
		sort.Strings(consumers)

		row := instrumentFlag{Name: name, Consumers: consumers, DeclaredAgeDays: -1}
		if len(consumers) > 0 {
			row.State = instrumentWired
			rep.Wired++
		} else {
			// Consumer-less: is it OLD enough to be DARK, or merely new (COLD)?
			days, known := cfg.declAgeDays(cfg.root, cfg.flagFile, name)
			if known {
				row.DeclaredAgeDays = days
			}
			if known && days > darkAgeDays {
				row.State = instrumentDark
				rep.Dark++
			} else {
				row.State = instrumentCold
				rep.Cold++
			}
		}
		rep.Flags = append(rep.Flags, row)
	}
	return rep, nil
}

// relOrSelf renders a path relative to root, or the path itself when it is not under root.
func relOrSelf(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

// runInstrumentAudit is the --instrument-audit entry point (production config). jsonOut
// selects the machine-readable object; otherwise it prints a human table. It returns 6
// (could-not-check) when the flag source cannot be read, 0 otherwise.
func runInstrumentAudit(root string, consumerRoots []string, jsonOut bool) int {
	rep, err := instrumentAudit(instrumentAuditConfig{root: root, consumerRoots: consumerRoots})
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: instrument-audit could-not-check:", err)
		return 6
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintln(os.Stderr, "statusgen: instrument-audit could not encode JSON:", err)
			return 6
		}
		return 0
	}
	fmt.Printf("instrument-audit: %d flags — %d WIRED, %d COLD, %d DARK (consumer roots: %s)\n",
		len(rep.Flags), rep.Wired, rep.Cold, rep.Dark, strings.Join(rep.ConsumerRoots, ", "))
	for _, f := range rep.Flags {
		switch f.State {
		case instrumentWired:
			fmt.Printf("  WIRED  --%-28s %d consumer(s)\n", f.Name, len(f.Consumers))
		case instrumentCold:
			fmt.Printf("  COLD   --%-28s 0 consumers (declared %s)\n", f.Name, ageText(f.DeclaredAgeDays))
		case instrumentDark:
			fmt.Printf("  DARK   --%-28s 0 consumers, declared %dd ago — retirement candidate\n", f.Name, f.DeclaredAgeDays)
		}
	}
	return 0
}

// ageText renders a declared-age for the human table, honestly reporting an unread age
// rather than printing "0d ago".
func ageText(days int) string {
	if days < 0 {
		return "age unknown"
	}
	return strconv.Itoa(days) + "d ago"
}

// instrumentAuditDarkNotices returns one NOTICE per DARK flag, for --lint. Each names the
// flag a retirement candidate; nothing is retired by the tool. A could-not-check reading
// the flag source is surfaced as its own NOTICE rather than silently producing no rows.
func instrumentAuditDarkNotices(root string) []string {
	// The audit's subject is statusgen's OWN flag surface. A root that carries no
	// statusgen/main.go is an adopter repo (or a test fixture), not statusgen's source
	// tree — it has no statusgen instruments to audit, so the hook is SILENT rather than
	// emitting a could-not-check notice into every adopter's lint output.
	if _, err := os.Stat(filepath.Join(root, "statusgen", "main.go")); err != nil {
		return nil
	}
	rep, err := instrumentAudit(instrumentAuditConfig{root: root})
	if err != nil {
		return []string{fmt.Sprintf("[dark-instrument] could-not-check the statusgen instrument audit: %v", err)}
	}
	var out []string
	for _, f := range rep.Flags {
		if f.State == instrumentDark {
			out = append(out, fmt.Sprintf(
				"[dark-instrument] statusgen flag --%s is DARK: 0 consumers across the audited roots and "+
					"declared %dd ago — retirement candidate (run `statusgen --root . --instrument-audit`); "+
					"nothing is retired by the tool",
				f.Name, f.DeclaredAgeDays))
		}
	}
	return out
}
