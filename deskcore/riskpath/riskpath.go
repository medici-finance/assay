// Package riskpath owns offline path matching and additive trigger composition.
package riskpath

import (
	"sort"
	"strings"
)

// Base returns a fresh copy of the universal trigger set. Callers add scoped values.
func Base() []string { return []string{"secrets/", ".github/workflows/", "k8s/*/rbac.yaml"} }
func Union(lists ...[]string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 8)
	for _, l := range lists {
		for _, p := range l {
			p = strings.TrimSpace(p)
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func Match(pattern, path string) bool {
	dirPrefix := strings.HasSuffix(pattern, "/")
	pp := strings.Split(strings.Trim(pattern, "/"), "/")
	fp := strings.Split(strings.Trim(path, "/"), "/")

	if dirPrefix {
		if len(fp) < len(pp) {
			return false
		}
	} else {
		if len(fp) != len(pp) {
			return false
		}
	}
	for i, seg := range pp {
		if seg == "*" {
			continue
		}
		if fp[i] != seg {
			return false
		}
	}
	return true
}
