// Command claimguard flags named-third-party-product-shaped claims (a
// CamelCase or ALL-CAPS-acronym token) that carry no resolving URL nearby —
// the "SLAW"/"VerdictCI" shape of unresolved outward claim: not a dead link
// (a separate link-resolution lint covers that), but no link at all.
//
// Usage:
//
//	go run . [--allow-file words.txt] [--window N] <path>...
//
// Each path is a markdown file, or a directory (scanned recursively for
// *.md). Exit status is 1 if any unresolved candidate is found, 0 otherwise.
// This tool does not fetch anything over the network and makes no CI-gate
// decision on its own — see README.md for scope and known limitations.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("claimguard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	allowFile := fs.String("allow-file", "", "extra allowlist: one word per line, '#' comments allowed")
	window := fs.Int("window", 0, "tokens to look either side of a candidate for a resolving URL (0 = default)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 {
		fmt.Fprintln(stderr, "claimguard: no paths given")
		return 2
	}

	allow := newDefaultAllow()
	if *allowFile != "" {
		extra, err := loadAllowFile(*allowFile)
		if err != nil {
			fmt.Fprintf(stderr, "claimguard: %v\n", err)
			return 2
		}
		for w := range extra {
			allow[w] = true
		}
	}

	files, err := collectMarkdownFiles(paths)
	if err != nil {
		fmt.Fprintf(stderr, "claimguard: %v\n", err)
		return 2
	}

	total := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(stderr, "claimguard: %v\n", err)
			return 2
		}
		findings := Scan(string(data), allow, *window)
		for _, fnd := range findings {
			fmt.Fprintf(stdout, "%s:%d: unresolved named-product-shaped claim %q (no URL within window): %s\n",
				f, fnd.Line, fnd.Token, strings.TrimSpace(fnd.Context))
			total++
		}
	}

	if total > 0 {
		fmt.Fprintf(stderr, "claimguard: %d unresolved claim(s)\n", total)
		return 1
	}
	return 0
}

func collectMarkdownFiles(paths []string) ([]string, error) {
	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".md") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func loadAllowFile(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	allow := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		allow[line] = true
	}
	return allow, sc.Err()
}
