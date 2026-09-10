package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Exit codes are a three-state instrument, not pass/fail
// (docs/three-state-instrument-rule.md):
//
//	0 — clean: artifacts written, or --check found them identical to the source
//	1 — drift: --check found a committed artifact that differs from the source
//	2 — could-not-check: the source is missing/unparseable, or a committed
//	    artifact could not be read. This is NOT a pass — a source the tool
//	    cannot read is an unknown, and a generator that silently drops a rule
//	    ships a weakened method to every session.
const (
	exitClean         = 0
	exitDrift         = 1
	exitCouldNotCheck = 2
)

// artifactPaths names, relative to root, the single source, the plugin
// manifest the Header's {{VERSION}} token derives from, and each generated
// artifact.
type artifactPaths struct {
	source         string
	pluginManifest string
	claudePayload  string
	codexFragment  string
}

func pathsFor(root string) artifactPaths {
	return artifactPaths{
		source:         filepath.Join(root, "plugins", "assay", "resident-rules.md"),
		pluginManifest: filepath.Join(root, "plugins", "assay", ".claude-plugin", "plugin.json"),
		claudePayload:  filepath.Join(root, "plugins", "assay", "hooks", "resident-rules.payload.txt"),
		codexFragment:  filepath.Join(root, "plugins", "assay", "codex", "AGENTS-assay.md"),
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: harnessgen <verb> [flags]\n\nverbs:\n  resident   generate the resident-rules delivery artifacts\n  codex      generate the Codex plugin manifest (.codex-plugin/plugin.json)\n  cursor     generate the Cursor resident-rules rule (.cursor/rules → cursor/assay.mdc)")
		os.Exit(exitCouldNotCheck)
	}
	switch os.Args[1] {
	case "resident":
		os.Exit(residentCmd(os.Args[2:]))
	case "codex":
		os.Exit(codexCmd(os.Args[2:]))
	case "cursor":
		os.Exit(cursorCmd(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "harnessgen: unknown verb %q (known: resident, codex, cursor)\n", os.Args[1])
		os.Exit(exitCouldNotCheck)
	}
}

func residentCmd(args []string) int {
	fs := flag.NewFlagSet("resident", flag.ContinueOnError)
	check := fs.Bool("check", false, "do not write; exit non-zero if the committed artifacts differ from what the source would generate")
	root := fs.String("root", ".", "repository root the plugins/ paths are resolved against")
	if err := fs.Parse(args); err != nil {
		return exitCouldNotCheck
	}
	p := pathsFor(*root)

	srcBytes, err := os.ReadFile(p.source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen resident: could-not-check: reading source %s: %v\n", p.source, err)
		return exitCouldNotCheck
	}
	s, err := parseSource(string(srcBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen resident: could-not-check: %s is unparseable: %v\n", p.source, err)
		return exitCouldNotCheck
	}

	// The Header's {{VERSION}} token derives from the plugin manifest, never a
	// literal typed into resident-rules.md (assay#730) — read it only when the
	// Header actually asks for it, so a Header with no placeholder never
	// requires the manifest to exist.
	var version string
	if strings.Contains(s.Header, versionPlaceholder) {
		meta, err := readClaudeManifest(p.pluginManifest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "harnessgen resident: could-not-check: %v\n", err)
			return exitCouldNotCheck
		}
		version = meta.Version
	}

	arts, err := generate(s, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen resident: could-not-check: %s: %v\n", p.source, err)
		return exitCouldNotCheck
	}

	targets := []struct {
		path string
		want string
	}{
		{p.claudePayload, arts.ClaudePayload},
		{p.codexFragment, arts.CodexFragment},
	}

	if *check {
		var diffs []string
		var unreadable []string
		for _, t := range targets {
			got, err := os.ReadFile(t.path)
			if err != nil {
				unreadable = append(unreadable, fmt.Sprintf("%s (%v)", t.path, err))
				continue
			}
			if string(got) != t.want {
				diffs = append(diffs, t.path)
			}
		}
		if len(unreadable) > 0 {
			for _, u := range unreadable {
				fmt.Fprintf(os.Stderr, "harnessgen resident --check: could-not-check: cannot read committed artifact %s\n", u)
			}
			return exitCouldNotCheck
		}
		if len(diffs) > 0 {
			fmt.Fprintln(os.Stderr, "harnessgen resident --check: DRIFT — committed artifacts differ from the source; run `go run ./tools/harnessgen resident` and commit:")
			for _, d := range diffs {
				fmt.Fprintf(os.Stderr, "  %s\n", d)
			}
			return exitDrift
		}
		fmt.Println("harnessgen resident --check: clean — committed artifacts match the source")
		return exitClean
	}

	// Write mode.
	if err := writeArtifacts(targets); err != nil {
		fmt.Fprintf(os.Stderr, "harnessgen resident: %v\n", err)
		return exitCouldNotCheck
	}
	for _, t := range targets {
		fmt.Printf("wrote %s\n", t.path)
	}
	return exitClean
}

func writeArtifacts(targets []struct {
	path string
	want string
}) error {
	var errs []error
	for _, t := range targets {
		if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := os.WriteFile(t.path, []byte(t.want), 0o644); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
