package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// issueLister resolves an issue number to its state ("OPEN", "CLOSED", or "UNKNOWN").
// It is a package-level var so tests can inject a mock. When nil, the tool uses
// `gh issue view <N> --json state`.
var issueLister func(number string) string

func main() {
	dryRun := flag.Bool("dry-run", false, "print prune/keep, delete nothing")
	root := flag.String("root", ".", "repository root")
	flag.Parse()

	os.Exit(run(*root, *dryRun))
}

func run(root string, dryRun bool) int {
	bugsDir := filepath.Join(root, "bugs")
	pruned, kept, err := collect(bugsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bugs-gc: %v\n", err)
		return 1
	}

	if len(pruned) == 0 && len(kept) == 0 {
		fmt.Println("bugs-gc: no bug files found")
		return 0
	}

	fmt.Printf("prune: %d\n", len(pruned))
	for _, p := range pruned {
		fmt.Printf("  prune: bugs/%s\n", p)
		if !dryRun {
			path := filepath.Join(bugsDir, p)
			if err := os.Remove(path); err != nil {
				fmt.Fprintf(os.Stderr, "bugs-gc: error removing %s: %v\n", path, err)
				return 1
			}
		}
	}

	fmt.Printf("keep:  %d\n", len(kept))
	for _, k := range kept {
		fmt.Printf("  keep:  bugs/%s\n", k)
	}

	return 0
}

// collect reads bugs/*.md, resolves each issue's state, and returns separate
// sorted lists of files to prune (CLOSED issue) and keep (OPEN issue).
// README.md and non-.md files are skipped. UNKNOWN states are skipped with a
// warning but do not cause an error.
func collect(bugsDir string) (prune, keep []string, err error) {
	entries, err := os.ReadDir(bugsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("reading bugs dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if e.Name() == "README.md" {
			continue // never prune the convention doc
		}

		number := strings.TrimSuffix(e.Name(), ".md")
		state := resolveState(number)

		switch state {
		case "CLOSED":
			prune = append(prune, e.Name())
		case "OPEN":
			keep = append(keep, e.Name())
		default:
			fmt.Fprintf(os.Stderr, "bugs-gc: warning: skipping bugs/%s (unknown state: %s)\n", e.Name(), state)
		}
	}

	sort.Strings(prune)
	sort.Strings(keep)
	return prune, keep, nil
}

func resolveState(number string) string {
	if issueLister != nil {
		return issueLister(number)
	}
	cmd := exec.Command("gh", "issue", "view", number, "--json", "state", "-q", ".state")
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bugs-gc: warning: gh issue view %s: %v\n", number, err)
		return "UNKNOWN"
	}
	return strings.TrimSpace(string(out))
}
