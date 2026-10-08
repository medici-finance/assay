package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// referenceDoc is docs/cellctl-cli-reference.md, generated from the command tree itself so the
// documented surface cannot drift from the real one. Regenerate with
// CELLCTL_UPDATE_REFERENCE=1 go test -run '^TestCLIReferenceDoc$' ./cmd/cellctl
const referenceDoc = "../../../../docs/cellctl-cli-reference.md"

func referenceMarkdown() string {
	var b strings.Builder
	b.WriteString("# `cellctl` command reference\n\n")
	b.WriteString("<!-- GENERATED from the command tree by TestCLIReferenceDoc; do not edit by hand. -->\n\n")
	b.WriteString("Every command takes the cell as its first argument; `--cells-root` is accepted anywhere.\n")
	b.WriteString("Help (`-h`, `--help`, `help <command>`) and version (`--version`, `version`) read nothing:\n")
	b.WriteString("no cell, no credential file, no roster. Single-dash spellings of long flags (`-model x`) are\n")
	b.WriteString("still accepted. How this differs from the pre-Cobra surface is in\n")
	b.WriteString("[cellctl-cli-compat.md](cellctl-cli-compat.md).\n\n")
	root := buildRoot()
	var hidden []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		subs := append([]*cobra.Command{}, c.Commands()...)
		sort.Slice(subs, func(i, j int) bool { return subs[i].Name() < subs[j].Name() })
		for _, s := range subs {
			if s.Name() == "help" || s.Name() == "completion" {
				continue
			}
			if s.Hidden {
				hidden = append(hidden, s.Name()+" — "+s.Short)
				continue
			}
			fmt.Fprintf(&b, "## `cellctl %s`\n\n%s\n\n", s.Use, s.Short)
			fmt.Fprintf(&b, "```\n%s```\n\n", flagTable(s))
		}
	}
	walk(root)
	b.WriteString("## Global flag\n\n```\n" + flagTable(root) + "```\n\n")
	b.WriteString("## Internal entrypoints\n\nThese verbs are invoked by cellctl's own generated command lines and hooks. They take their\n")
	b.WriteString("argv verbatim (no flag parsing by the command layer) and are not part of the operator surface.\n\n")
	for _, h := range hidden {
		b.WriteString("- `" + strings.SplitN(h, " — ", 2)[0] + "`\n")
	}
	return b.String()
}

func flagTable(c *cobra.Command) string {
	var rows []string
	add := func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}
		name := "--" + f.Name
		if f.Shorthand != "" {
			name = "-" + f.Shorthand + ", " + name
		}
		typ := f.Value.Type()
		if typ == "bool" {
			typ = ""
		}
		rows = append(rows, fmt.Sprintf("  %-26s %-8s %s", name, typ, f.Usage))
	}
	c.LocalNonPersistentFlags().VisitAll(add)
	c.PersistentFlags().VisitAll(add)
	if len(rows) == 0 {
		return "  (no flags)\n"
	}
	return strings.Join(rows, "\n") + "\n"
}

func TestCLIReferenceDoc(t *testing.T) {
	want := referenceMarkdown()
	path := filepath.FromSlash(referenceDoc)
	if os.Getenv("CELLCTL_UPDATE_REFERENCE") == "1" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v (regenerate with CELLCTL_UPDATE_REFERENCE=1)", path, err)
	}
	if string(got) != want {
		t.Errorf("%s is stale against the command tree; regenerate with CELLCTL_UPDATE_REFERENCE=1 go test -run '^TestCLIReferenceDoc$' ./cmd/cellctl", path)
	}
}

// TestCLICompatDoc: every deliberate difference the non-help parity test allows is written up in
// the compatibility table, so an allowlist entry cannot land without its operator-facing note.
func TestCLICompatDoc(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash("../../../../docs/cellctl-cli-compat.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	for key, d := range deliberateDiffs {
		if !strings.Contains(doc, "`"+key+"`") {
			t.Errorf("deliberate difference %q (%s) is not in docs/cellctl-cli-compat.md", key, d.why)
		}
	}
}
