package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Filesystem helpers, wrapped so the lint's could-not-check path is exercised
// from one place.

func statRoot(root string) (os.FileInfo, error) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("root %s does not exist", root)
		}
		return nil, fmt.Errorf("root %s unreadable: %w", root, err)
	}
	return info, nil
}

func readFile(path string) ([]byte, error) { return os.ReadFile(path) }

// fileExists reports whether path exists (file or directory) — used for a
// provider's `evidence` marker (§9's activation gate).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func relOr(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

// Minimal semver: enough for the manifests' ranges, with no external dependency
// (the offline envelope forbids fetching a semver library, and x/mod is not a
// dependency of this module). It parses MAJOR.MINOR.PATCH, tolerates a leading
// "v", and ignores any pre-release/build suffix for comparison.

type semver struct{ major, minor, patch int }

func parseSemver(s string) (semver, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return semver{}, fmt.Errorf("not a semver: %q", s)
	}
	var v semver
	dst := []*int{&v.major, &v.minor, &v.patch}
	for i := 0; i < len(parts); i++ {
		n, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil {
			return semver{}, fmt.Errorf("not a semver: %q", s)
		}
		*dst[i] = n
	}
	return v, nil
}

// cmp returns -1, 0, +1 comparing a to b.
func (a semver) cmp(b semver) int {
	for _, pair := range [][2]int{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return 0
}

// satisfies reports whether version meets rangeExpr, a whitespace-separated set
// of ANDed constraints, each an operator (>=, <=, >, <, =, ==) followed by a
// semver. A bare version means "=".
func satisfies(version, rangeExpr string) (bool, error) {
	v, err := parseSemver(version)
	if err != nil {
		return false, fmt.Errorf("provider version %q: %w", version, err)
	}
	fields := strings.Fields(rangeExpr)
	if len(fields) == 0 {
		return true, nil
	}
	for _, f := range fields {
		op, rest := splitOp(f)
		want, err := parseSemver(rest)
		if err != nil {
			return false, fmt.Errorf("range term %q: %w", f, err)
		}
		c := v.cmp(want)
		ok := false
		switch op {
		case ">=":
			ok = c >= 0
		case "<=":
			ok = c <= 0
		case ">":
			ok = c > 0
		case "<":
			ok = c < 0
		case "=", "==":
			ok = c == 0
		default:
			return false, fmt.Errorf("unsupported range operator in %q", f)
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func splitOp(f string) (op, rest string) {
	for _, o := range []string{">=", "<=", "==", ">", "<", "="} {
		if strings.HasPrefix(f, o) {
			return o, strings.TrimPrefix(f, o)
		}
	}
	return "=", f
}
