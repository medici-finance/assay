package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Exit codes — this tool's three states (see main.go doc comment). Deliberately
// local, not deskkit's write-tool map: a lint reports clean / problems /
// could-not-check.
const (
	exitClean        = 0
	exitProblems     = 1
	exitCouldNotCheck = 2
)

// injectKey is one entry under inject.required or inject.optional.
type injectKey struct {
	Key   string `yaml:"key"`
	Range string `yaml:"range"`
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
	Component string   `yaml:"component"`
	Version   string   `yaml:"version"`
	Provides  []string `yaml:"provides"`
	Inject    struct {
		Required []injectKey `yaml:"required"`
		Optional []injectKey `yaml:"optional"`
	} `yaml:"inject"`
	Apply     []applyStep `yaml:"apply"`
	Intercept []string    `yaml:"intercept"`

	path string // discovery path, for reporting
}

// provider records who provides a key and at what version.
type provider struct {
	component string
	version   string
	path      string
}

func lintCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "tree root to discover component.yaml under")
	fs.Usage = func() { fmt.Fprintln(stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return exitCouldNotCheck
	}
	report, code := lint(*root)
	fmt.Fprint(stdout, report)
	return code
}

// lint runs the whole check over root and returns a report and an exit code.
func lint(root string) (string, int) {
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

	sort.Strings(problems)
	var b strings.Builder
	for _, p := range problems {
		fmt.Fprintf(&b, "PROBLEM: %s\n", p)
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
		for _, key := range m.Provides {
			if !strings.HasPrefix(key, "assay.") {
				out = append(out, fmt.Sprintf("%s (%s): provides key %q outside the assay. namespace from a component in the assay/ namespace (§3)", m.path, m.Component, key))
			}
		}
	}
	return out
}

// buildProviders maps each provided key to the components that provide it.
func buildProviders(ms []*manifest) map[string][]provider {
	out := map[string][]provider{}
	for _, m := range ms {
		for _, key := range m.Provides {
			out[key] = append(out[key], provider{component: m.Component, version: m.Version, path: m.path})
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
