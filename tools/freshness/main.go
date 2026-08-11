// freshness — deterministic artifact-staleness checker.
//
// Reads freshness.yaml from the assay root and, for each listed artifact,
// reports whether it is FRESH or STALE based on:
//  1. last-reviewed date + max-age-days vs today (or --as-of).
//  2. Any upstream git commit newer than last-reviewed.
//
// By default every artifact in the manifest is checked and the exit code reflects
// the whole manifest (exit 0 = all fresh; exit 1 = one or more stale). Pass --only
// (repeatable, or comma-separated) to scope the run — and its exit code — to a
// subset of artifacts by path, so a caller that only cares about one artifact's
// own freshness is not coupled to unrelated staleness elsewhere in the manifest.
//
// Exit 2 = usage/config error (bad manifest, unknown --only path, bad --as-of).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type config struct {
	Artifacts []artifact `yaml:"artifacts"`
}

type artifact struct {
	Path         string     `yaml:"path"`
	LastReviewed string     `yaml:"last-reviewed"`
	MaxAgeDays   int        `yaml:"max-age-days"`
	Upstreams    []upstream `yaml:"upstreams"`
}

type upstream struct {
	Repo  string   `yaml:"repo"`
	Globs []string `yaml:"globs"`
}

// onlyFlag collects one or more --only values: repeatable (--only a --only b)
// and/or comma-separated (--only a,b). Order of first appearance is preserved
// for the "unknown path" error message; duplicates are harmless.
type onlyFlag struct {
	paths []string
}

func (o *onlyFlag) String() string { return strings.Join(o.paths, ",") }

func (o *onlyFlag) Set(v string) error {
	for _, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			o.paths = append(o.paths, p)
		}
	}
	return nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the freshness check and returns the process exit code. It is
// factored out of main so tests can drive it without an os.Exit call.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("freshness", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asOf := fs.String("as-of", "", "Pretend today is this date (YYYY-MM-DD) for testing")
	root := fs.String("root", ".", "Path to assay root (containing freshness.yaml)")
	var only onlyFlag
	fs.Var(&only, "only", "Scope the check (and exit code) to this artifact path; repeatable or comma-separated")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfgPath := filepath.Join(*root, "freshness.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "freshness: cannot read %s: %v\n", cfgPath, err)
		return 2
	}

	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(stderr, "freshness: cannot parse %s: %v\n", cfgPath, err)
		return 2
	}

	artifacts := cfg.Artifacts
	if len(only.paths) > 0 {
		artifacts, err = selectOnly(cfg.Artifacts, only.paths)
		if err != nil {
			fmt.Fprintf(stderr, "freshness: %v\n", err)
			return 2
		}
	}

	today := time.Now().Truncate(24 * time.Hour)
	if *asOf != "" {
		today, err = time.Parse("2006-01-02", *asOf)
		if err != nil {
			fmt.Fprintf(stderr, "freshness: invalid --as-of date %q: %v\n", *asOf, err)
			return 2
		}
	}

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "freshness: cannot resolve root: %v\n", err)
		return 2
	}

	anyStale := false
	for _, a := range artifacts {
		stale, reason := checkArtifact(a, today, absRoot)
		status := "FRESH"
		if stale {
			status = "STALE"
			anyStale = true
		}
		fmt.Fprintf(stdout, "%s  %s  %s\n", status, a.Path, reason)
	}

	if anyStale {
		return 1
	}
	return 0
}

// selectOnly filters artifacts to those whose Path is in wantPaths, in the
// order wantPaths lists them (duplicates collapsed). It errors if a requested
// path matches no artifact, so a typo in --only fails loudly instead of
// silently checking nothing (or, worse, silently checking everything).
func selectOnly(artifacts []artifact, wantPaths []string) ([]artifact, error) {
	byPath := make(map[string]artifact, len(artifacts))
	for _, a := range artifacts {
		byPath[a.Path] = a
	}

	seen := make(map[string]bool, len(wantPaths))
	var out []artifact
	for _, p := range wantPaths {
		if seen[p] {
			continue
		}
		seen[p] = true
		a, ok := byPath[p]
		if !ok {
			return nil, fmt.Errorf("--only %q does not match any artifact in freshness.yaml", p)
		}
		out = append(out, a)
	}
	return out, nil
}

func checkArtifact(a artifact, today time.Time, root string) (stale bool, reason string) {
	lr, err := time.Parse("2006-01-02", a.LastReviewed)
	if err != nil {
		return true, fmt.Sprintf("bad last-reviewed date %q: %v", a.LastReviewed, err)
	}

	if a.MaxAgeDays > 0 {
		deadline := lr.AddDate(0, 0, a.MaxAgeDays)
		if today.After(deadline) {
			return true, fmt.Sprintf("max-age %dd exceeded (reviewed %s, deadline %s, today %s)",
				a.MaxAgeDays, a.LastReviewed, deadline.Format("2006-01-02"), today.Format("2006-01-02"))
		}
	}

	for _, u := range a.Upstreams {
		repoDir := filepath.Join(root, u.Repo)
		for _, g := range u.Globs {
			pattern := filepath.Join(repoDir, g)
			matches, _ := filepath.Glob(pattern)
			if len(matches) == 0 {
				continue
			}
			for _, m := range matches {
				newer, err := hasCommitNewerThan(repoDir, m, lr)
				if err != nil {
					return true, fmt.Sprintf("upstream check error for %s in %s: %v", m, u.Repo, err)
				}
				if newer {
					return true, fmt.Sprintf("upstream %s has commits newer than %s", m, a.LastReviewed)
				}
			}
		}
	}

	return false, fmt.Sprintf("reviewed %s, max-age %dd", a.LastReviewed, a.MaxAgeDays)
}

func hasCommitNewerThan(repoDir, path string, since time.Time) (bool, error) {
	// Check for commits strictly AFTER the review date (commits on the review
	// date itself are considered accounted-for).
	afterDate := since.AddDate(0, 0, 1)
	afterStr := afterDate.Format("2006-01-02")
	cmd := exec.Command("git", "log", "--oneline", "-1",
		"--since="+afterStr+" 00:00:00",
		"--", path)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 128 {
				return false, fmt.Errorf("not a git repo or no commits: %s", repoDir)
			}
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}
