package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// injectKey is one entry under inject.required or inject.optional
// (component-model.md §2).
type injectKey struct {
	Key   string `yaml:"key"`
	Range string `yaml:"range"`
}

// applyStep is one ordered effect and its reverse (§4).
type applyStep struct {
	ID           string `yaml:"id"`
	Effect       string `yaml:"effect"`
	Boundary     string `yaml:"boundary"` // "inside" | "outside"
	Inverse      string `yaml:"inverse"`
	Ledger       string `yaml:"ledger"`
	Compensation string `yaml:"compensation"`
}

// manifest is a parsed component.yaml (§2). Deliberately a separate, smaller
// type from deskmanifest's own — deskdisable reads the same file shape but
// has different needs (it walks apply steps in reverse; it does not lint
// ranges or cycles), and duplicating the ~30-line struct keeps the two tools
// independently correct rather than coupled through a shared internal type
// neither owns.
type manifest struct {
	Component string   `yaml:"component"`
	Version   string   `yaml:"version"`
	Provides  []string `yaml:"provides"`
	Inject    struct {
		Required []injectKey `yaml:"required"`
		Optional []injectKey `yaml:"optional"`
	} `yaml:"inject"`
	Apply []applyStep `yaml:"apply"`

	path string // discovery path, for error messages
}

// discoverManifests walks root for every file named component.yaml, skipping
// .git, and parses each. A missing or unreadable root is an error; a root
// that parses cleanly but holds an unparseable manifest is also an error —
// deskdisable must never compute a reverse plan from a tree it could not read
// in full.
func discoverManifests(root string) ([]*manifest, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("root %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root %s is not a directory", root)
	}

	var paths []string
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
			paths = append(paths, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walking %s: %w", root, walkErr)
	}
	sort.Strings(paths)

	out := make([]*manifest, 0, len(paths))
	for _, p := range paths {
		m, err := parseManifest(root, p)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func parseManifest(root, path string) (*manifest, error) {
	rel, relErr := filepath.Rel(root, path)
	if relErr != nil {
		rel = path
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: unreadable: %w", rel, err)
	}
	var m manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: not valid manifest YAML: %w", rel, err)
	}
	if strings.TrimSpace(m.Component) == "" {
		return nil, fmt.Errorf("%s: empty component id", rel)
	}
	m.path = rel
	return &m, nil
}

// byID indexes manifests by their component id.
func byID(ms []*manifest) map[string]*manifest {
	out := make(map[string]*manifest, len(ms))
	for _, m := range ms {
		out[m.Component] = m
	}
	return out
}

// dependentsOf returns the component ids (excluding target's own) that are
// still "present" (per the presence map) and whose inject.required lists any
// key target provides. Range is deliberately NOT checked here: an
// out-of-range requirement is still a reason to hesitate before removing the
// provider — dependentsOf errs toward over-caution (component-model.md §4's
// "reverses are local" and this brief's ground rule against a best-effort
// destructive step), not toward the more permissive resolution
// `deskmanifest lint` performs for activation.
func dependentsOf(target *manifest, all map[string]*manifest, present map[string]bool) []string {
	provided := map[string]bool{}
	for _, k := range target.Provides {
		provided[k] = true
	}
	var deps []string
	for id, m := range all {
		if id == target.Component || !present[id] {
			continue
		}
		for _, req := range m.Inject.Required {
			if provided[req.Key] {
				deps = append(deps, id)
				break
			}
		}
	}
	sort.Strings(deps)
	return deps
}
