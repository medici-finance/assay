package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Exit codes — this tool's three states (see main.go doc comment). Deliberately
// local, not deskkit's write-tool map: a lint reports clean / problems /
// could-not-check.
const (
	exitClean         = 0
	exitProblems      = 1
	exitCouldNotCheck = 2
)

// injectKey is one entry under inject.required or inject.optional. A `flavour`
// constraint (§9) narrows resolution to a provider whose matching `provides`
// entry carries that flavour — e.g. the hooks component requires
// `assay.harness` with `flavour: claude-code` since the SessionStart/PreToolUse
// mechanism is Claude Code's alone.
type injectKey struct {
	Key     string `yaml:"key"`
	Range   string `yaml:"range"`
	Flavour string `yaml:"flavour"`
}

// provideEntry is one entry under `provides:` (§2). It is either a bare key
// string, or a mapping carrying attributes: `flavour` distinguishes an
// exclusively-bound key's competing implementations (§9 — the harness
// adapters each provide `assay.harness` with their own flavour); `evidence`
// names a repo-root-relative file or directory whose presence is this brief's
// answer to "the presence of exactly one adapter's installed shape selects the
// binding" (component-model.md §9), ahead of the desired-state record (§7,
// still planned) that will supersede it. A provider with no `evidence` is
// active exactly when its own `inject.required` resolves, as before.
type provideEntry struct {
	Key      string
	Flavour  string
	Evidence string
}

// UnmarshalYAML accepts either a bare scalar (`- assay.forge`) or a mapping
// (`- key: assay.harness` with `flavour:`/`evidence:` siblings), so every
// manifest already in the tree keeps parsing unchanged.
func (p *provideEntry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		p.Key = node.Value
		return nil
	}
	var m struct {
		Key      string `yaml:"key"`
		Flavour  string `yaml:"flavour"`
		Evidence string `yaml:"evidence"`
	}
	if err := node.Decode(&m); err != nil {
		return err
	}
	p.Key = m.Key
	p.Flavour = m.Flavour
	p.Evidence = m.Evidence
	return nil
}

// applyStep is one ordered effect and its reverse (§4).
type applyStep struct {
	ID           string `yaml:"id"`
	Effect       string `yaml:"effect"`
	Boundary     string `yaml:"boundary"`
	Inverse      string `yaml:"inverse"`
	Ledger       string `yaml:"ledger"`
	Compensation string `yaml:"compensation"`
}

// manifest is a parsed component.yaml (§2).
type manifest struct {
	Component string         `yaml:"component"`
	Version   string         `yaml:"version"`
	Provides  []provideEntry `yaml:"provides"`
	Inject    struct {
		Required []injectKey `yaml:"required"`
		Optional []injectKey `yaml:"optional"`
	} `yaml:"inject"`
	Apply     []applyStep `yaml:"apply"`
	Intercept []string    `yaml:"intercept"`

	path string // discovery path, for reporting
}

// provider records who provides a key, at what version and flavour.
type provider struct {
	component string
	version   string
	flavour   string
	path      string
}

func lintCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "tree root to discover component.yaml under")
	activationFlag := fs.Bool("activation", false, "include an ACTIVATION report (§6.1): every component's ACTIVE/INACTIVE state and, when INACTIVE, why")
	fs.Usage = func() { fmt.Fprintln(stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return exitCouldNotCheck
	}
	report, code := lintReport(*root, *activationFlag)
	fmt.Fprint(stdout, report)
	return code
}

// lint runs the whole check over root and returns a report and an exit code.
// It is the no-activation-report form of lintReport, kept for callers (and
// the existing test suite) that don't need the per-component ACTIVATION
// listing.
func lint(root string) (string, int) {
	return lintReport(root, false)
}

// lintReport runs the whole check over root and returns a report and an exit
// code. showActivation additionally appends an ACTIVATION section (§6.1) —
// every component's computed state — without changing the exit code: an
// INACTIVE component is a legitimate state, not itself a problem. Only the
// exclusivity violation (§9: more than one ACTIVE provider of an exclusive
// key) is a PROBLEM, and that is checked regardless of showActivation.
func lintReport(root string, showActivation bool) (string, int) {
	paths, err := discover(root)
	if err != nil {
		// The lint could not run at all — three-state could-not-check, never
		// rounded to clean and never to a problem it did not observe.
		return fmt.Sprintf("could-not-check: %v\n", err), exitCouldNotCheck
	}

	var problems []string
	manifests := make([]*manifest, 0, len(paths))
	for _, p := range paths {
		m, perr := parseManifest(root, p)
		if perr != nil {
			problems = append(problems, perr.Error())
			continue
		}
		manifests = append(manifests, m)
	}

	// A tree with parseable manifests can still surface problems; a tree we
	// could open but that holds no manifests is clean-but-empty, not
	// could-not-check (we DID look).
	problems = append(problems, checkIDs(manifests)...)
	problems = append(problems, checkNamespace(manifests)...)

	providers := buildProviders(manifests)
	problems = append(problems, checkResolution(manifests, providers)...)
	problems = append(problems, checkCycles(manifests, providers)...)
	problems = append(problems, checkOutsideLedger(manifests)...)

	exclusiveKeys, exErr := loadExclusiveKeys(root)
	if exErr != nil {
		return fmt.Sprintf("could-not-check: %v\n", exErr), exitCouldNotCheck
	}
	state := computeActivation(root, manifests, providers)
	problems = append(problems, checkExclusive(providers, state, exclusiveKeys)...)

	sort.Strings(problems)
	var b strings.Builder
	for _, p := range problems {
		fmt.Fprintf(&b, "PROBLEM: %s\n", p)
	}
	if showActivation {
		fmt.Fprintln(&b, "ACTIVATION:")
		ids := make([]string, 0, len(state))
		for id := range state {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if st := state[id]; st.active {
				fmt.Fprintf(&b, "  %s: ACTIVE\n", id)
			} else {
				fmt.Fprintf(&b, "  %s: INACTIVE — %s\n", id, st.reason)
			}
		}
	}
	if len(problems) > 0 {
		fmt.Fprintf(&b, "checked-failed: %d problem(s) across %d manifest(s)\n", len(problems), len(manifests))
		return b.String(), exitProblems
	}
	fmt.Fprintf(&b, "checked-clean: %d manifest(s), every inject.required resolves in range, no cycles\n", len(manifests))
	// The final line the Verify table greps for is exactly `checked-clean`.
	fmt.Fprintln(&b, "checked-clean")
	return b.String(), exitClean
}

// discover walks root for files named component.yaml, skipping .git. A missing
// or unreadable root returns an error → the caller reports could-not-check.
func discover(root string) ([]string, error) {
	info, err := statRoot(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root %s is not a directory", root)
	}
	var out []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "component.yaml" {
			out = append(out, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("could not walk %s: %w", root, walkErr)
	}
	sort.Strings(out)
	return out, nil
}

// parseManifest reads and validates one component.yaml against §2's required
// keys. A missing required key, or a malformed file, is a PROBLEM (we looked and
// it is broken) — distinct from could-not-check (we could not look at all).
func parseManifest(root, path string) (*manifest, error) {
	rel := relOr(root, path)
	data, err := readFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: unreadable: %v", rel, err)
	}

	// Presence pass: §2 requires component, version, provides, inject, apply as
	// KEYS — an empty provides or apply must still carry the key. Unmarshalling
	// into a struct cannot tell an absent key from an empty one, so check the
	// raw mapping first.
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: not valid YAML: %v", rel, err)
	}
	for _, req := range []string{"component", "version", "provides", "inject", "apply"} {
		if _, ok := raw[req]; !ok {
			return nil, fmt.Errorf("%s: missing required key %q (§2: component, version, provides, inject, apply are REQUIRED)", rel, req)
		}
	}

	var m manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: not valid manifest YAML: %v", rel, err)
	}
	if strings.TrimSpace(m.Component) == "" {
		return nil, fmt.Errorf("%s: empty component id", rel)
	}
	if strings.TrimSpace(m.Version) == "" {
		return nil, fmt.Errorf("%s: %s: empty version", rel, m.Component)
	}
	m.path = rel
	return &m, nil
}

// checkIDs flags a component id used by more than one manifest — ids MUST be
// unique in the tree (§2).
func checkIDs(ms []*manifest) []string {
	seen := map[string][]string{}
	for _, m := range ms {
		seen[m.Component] = append(seen[m.Component], m.path)
	}
	var out []string
	for id, paths := range seen {
		if len(paths) > 1 {
			sort.Strings(paths)
			out = append(out, fmt.Sprintf("duplicate component id %q declared by: %s", id, strings.Join(paths, ", ")))
		}
	}
	return out
}

// checkNamespace enforces §3: a component whose id is in the assay/ namespace
// MUST NOT provide a key outside the assay. namespace.
func checkNamespace(ms []*manifest) []string {
	var out []string
	for _, m := range ms {
		if !strings.HasPrefix(m.Component, "assay/") {
			continue
		}
		for _, pe := range m.Provides {
			if !strings.HasPrefix(pe.Key, "assay.") {
				out = append(out, fmt.Sprintf("%s (%s): provides key %q outside the assay. namespace from a component in the assay/ namespace (§3)", m.path, m.Component, pe.Key))
			}
		}
	}
	return out
}

// buildProviders maps each provided key to the components that provide it.
func buildProviders(ms []*manifest) map[string][]provider {
	out := map[string][]provider{}
	for _, m := range ms {
		for _, pe := range m.Provides {
			out[pe.Key] = append(out[pe.Key], provider{component: m.Component, version: m.Version, flavour: pe.Flavour, path: m.path})
		}
	}
	return out
}

// loadExclusiveKeys scans components/KEYS.md under root for keys the catalogue
// marks `exclusive: true` (§9's format: a line naming the backtick-quoted key
// that also carries the literal text `exclusive: true`). The file is a
// planned artifact this brief starts populating; a tree without one yet (a
// bare fixture, an adopter pre-04) simply enforces no exclusivity — that is
// not a could-not-check, it is "nothing is exclusive here."
var exclusiveKeyPattern = regexp.MustCompile("`(assay\\.[A-Za-z0-9_.-]+)`")

func loadExclusiveKeys(root string) (map[string]bool, error) {
	path := filepath.Join(root, "components", "KEYS.md")
	data, err := readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("components/KEYS.md: unreadable: %w", err)
	}
	out := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "exclusive: true") {
			continue
		}
		for _, m := range exclusiveKeyPattern.FindAllStringSubmatch(line, -1) {
			out[m[1]] = true
		}
	}
	return out, nil
}

// activationState is one component's computed §6.1 state.
type activationState struct {
	active bool
	reason string // set when !active
}

// computeActivation implements §6.1 over the discovered manifests: a
// component is ACTIVE iff every inject.required key resolves (key exists,
// flavour matches when constrained, in range when constrained) to a provider
// that is itself ACTIVE. It is a greatest-fixpoint computation (start
// optimistic, deactivate on an unmet requirement, repeat to convergence), so
// a cycle among inject.required correctly leaves every member of the cycle
// INACTIVE (§6.1) without special-casing.
//
// Ahead of the desired-state record (§7, still planned), a provider whose
// `provides` entry names an `evidence` marker is active only while that
// marker exists under root — this is how the harness adapters answer "the
// presence of exactly one adapter's installed shape selects the binding"
// (§9) before there is a record to consult.
func computeActivation(root string, ms []*manifest, providers map[string][]provider) map[string]*activationState {
	state := make(map[string]*activationState, len(ms))
	for _, m := range ms {
		state[m.Component] = &activationState{active: true}
	}
	for _, m := range ms {
		for _, pe := range m.Provides {
			if pe.Evidence == "" {
				continue
			}
			if !fileExists(filepath.Join(root, pe.Evidence)) {
				state[m.Component] = &activationState{active: false, reason: fmt.Sprintf("installed-shape marker %s absent", pe.Evidence)}
			}
		}
	}
	for pass := 0; pass < len(ms)+2; pass++ {
		changed := false
		for _, m := range ms {
			st := state[m.Component]
			if !st.active {
				continue
			}
			if ok, reason := requiredSatisfied(m, providers, state); !ok {
				state[m.Component] = &activationState{active: false, reason: reason}
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return state
}

// requiredSatisfied reports whether every inject.required entry of m resolves
// to an ACTIVE provider, and if not, a human-readable reason for the first
// entry that fails — "no provider" (nothing anywhere provides the key), a
// flavour mismatch (a provider exists but not in the required flavour), or
// no active/in-range provider (a provider exists, matches flavour, but is
// itself inactive or out of range).
func requiredSatisfied(m *manifest, providers map[string][]provider, state map[string]*activationState) (bool, string) {
	for _, req := range m.Inject.Required {
		provs := providers[req.Key]
		if len(provs) == 0 {
			return false, fmt.Sprintf("%s has no provider", req.Key)
		}
		matched := false
		var wrongFlavours []string
		for _, p := range provs {
			if req.Flavour != "" && p.flavour != req.Flavour {
				wrongFlavours = append(wrongFlavours, p.flavour)
				continue
			}
			if req.Range != "" {
				inRange, rerr := satisfies(p.version, req.Range)
				if rerr != nil || !inRange {
					continue
				}
			}
			if pst, ok := state[p.component]; ok && !pst.active {
				continue
			}
			matched = true
			break
		}
		if !matched {
			if req.Flavour != "" && len(wrongFlavours) > 0 {
				sort.Strings(wrongFlavours)
				wrongFlavours = uniqueStrings(wrongFlavours)
				return false, fmt.Sprintf("%s flavour %s ≠ %s", req.Key, strings.Join(wrongFlavours, ","), req.Flavour)
			}
			return false, fmt.Sprintf("%s has no active provider", req.Key)
		}
	}
	return true, ""
}

func uniqueStrings(in []string) []string {
	out := in[:0]
	var last string
	for i, s := range in {
		if i == 0 || s != last {
			out = append(out, s)
		}
		last = s
	}
	return out
}

// checkExclusive enforces §9: an exclusive key MUST have at most one ACTIVE
// provider. A second is a lint PROBLEM, named with the key and every ACTIVE
// provider's component id.
func checkExclusive(providers map[string][]provider, state map[string]*activationState, exclusive map[string]bool) []string {
	if len(exclusive) == 0 {
		return nil
	}
	keys := make([]string, 0, len(exclusive))
	for k := range exclusive {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []string
	for _, key := range keys {
		var active []string
		for _, p := range providers[key] {
			if st, ok := state[p.component]; ok && st.active {
				active = append(active, p.component)
			}
		}
		if len(active) > 1 {
			sort.Strings(active)
			out = append(out, fmt.Sprintf("%s has %d ACTIVE providers: %s", key, len(active), strings.Join(active, ", ")))
		}
	}
	return out
}

// checkResolution enforces §2/§6.1: every inject.required key MUST have a
// provider, and where the entry carries a range at least one provider MUST be in
// range. inject.optional keys are allowed to be unresolved (absent → default).
func checkResolution(ms []*manifest, providers map[string][]provider) []string {
	var out []string
	for _, m := range ms {
		for _, req := range m.Inject.Required {
			provs, ok := providers[req.Key]
			if !ok || len(provs) == 0 {
				out = append(out, fmt.Sprintf("%s (%s): inject.required key %q has no provider (unresolved)", m.path, m.Component, req.Key))
				continue
			}
			if strings.TrimSpace(req.Range) == "" {
				continue
			}
			inRange := false
			var seen []string
			for _, p := range provs {
				seen = append(seen, fmt.Sprintf("%s@%s", p.component, p.version))
				ok, rerr := satisfies(p.version, req.Range)
				if rerr != nil {
					out = append(out, fmt.Sprintf("%s (%s): inject.required key %q has unparseable range %q: %v", m.path, m.Component, req.Key, req.Range, rerr))
					inRange = true // don't also report out-of-range for a range we couldn't parse
					break
				}
				if ok {
					inRange = true
					break
				}
			}
			if !inRange {
				out = append(out, fmt.Sprintf("%s (%s): inject.required key %q needs a provider in range %q; provider(s) out of range: %s", m.path, m.Component, req.Key, req.Range, strings.Join(seen, ", ")))
			}
		}
	}
	return out
}

// checkCycles reports cycles among inject.required edges (§6.1). An edge runs
// from a component to each component that provides one of its required keys.
// inject.optional edges are excluded — an optional edge never forms a cycle.
func checkCycles(ms []*manifest, providers map[string][]provider) []string {
	// adjacency by component id
	adj := map[string]map[string]bool{}
	byID := map[string]bool{}
	for _, m := range ms {
		byID[m.Component] = true
		if adj[m.Component] == nil {
			adj[m.Component] = map[string]bool{}
		}
		for _, req := range m.Inject.Required {
			for _, p := range providers[req.Key] {
				if p.component == "" {
					continue
				}
				adj[m.Component][p.component] = true
			}
		}
	}

	// DFS with colouring; collect distinct cycles by their sorted member set.
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	reported := map[string]bool{}
	var out []string
	var stack []string

	var dfs func(node string)
	dfs = func(node string) {
		color[node] = gray
		stack = append(stack, node)
		for next := range adj[node] {
			if !byID[next] {
				continue
			}
			switch color[next] {
			case gray:
				// Found a back edge — extract the cycle from the stack.
				cycle := extractCycle(stack, next)
				members := append([]string(nil), cycle...)
				sort.Strings(members)
				sig := strings.Join(members, "|")
				if !reported[sig] {
					reported[sig] = true
					out = append(out, fmt.Sprintf("cycle among inject.required: %s", strings.Join(cycle, " -> ")+" -> "+next))
				}
			case white:
				dfs(next)
			}
		}
		stack = stack[:len(stack)-1]
		color[node] = black
	}

	// Deterministic iteration order over components.
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if color[id] == white {
			dfs(id)
		}
	}
	sort.Strings(out)
	return out
}

// checkOutsideLedger enforces component-model.md §5: an outside apply step
// MUST write a ledger line naming what it created, so removal can find it
// later. A step's `ledger:` field is that promise made at manifest time — an
// outside step with no ledger: value is a defect the lint MUST flag
// this brief adds, the same way an unresolved inject key is: it is
// something the manifest alone can already prove is broken, before any
// install ever runs.
func checkOutsideLedger(ms []*manifest) []string {
	var out []string
	for _, m := range ms {
		for _, step := range m.Apply {
			if step.Boundary != "outside" {
				continue
			}
			if strings.TrimSpace(step.Ledger) == "" {
				out = append(out, fmt.Sprintf(
					"%s (%s): outside apply step %q has no ledger: value (component-model.md §5 — an outside effect with no ledger line cannot be found for compensation)",
					m.path, m.Component, step.ID))
			}
		}
	}
	return out
}

// extractCycle returns the suffix of stack starting at the first occurrence of
// target — the components that form the cycle back to target.
func extractCycle(stack []string, target string) []string {
	for i, n := range stack {
		if n == target {
			return append([]string(nil), stack[i:]...)
		}
	}
	return append([]string(nil), stack...)
}
