package main

// migrateanchors.go — `statusgen migrate gitlab-anchors [--dry-run] [--root DIR]`
// (#5004). The scaffolded .gitlab-ci.yml embeds statusgen/pushcred.sh and
// statusgen/boardadd.sh verbatim as the .statusgen-pushcred and
// .statusgen-boardadd script anchors, and `statusgen init` never overwrites an
// existing file, so a pin bump never reached the copy a pipeline runs. This
// target rewrites each embedded anchor that differs from this statusgen's copy,
// in place, and touches nothing else in the file: the comment lines above an
// anchor, every job, and every other key stay the adopter's.
//
// It is the executable half of a deskmigrate statusgen-regen op, and shares that
// contract: OFFLINE (reads and writes only <root>/.gitlab-ci.yml), IDEMPOTENT (a
// second run finds every anchor current and exits 0 writing nothing), and
// dry-runnable. No .gitlab-ci.yml, or a file that carries neither anchor, is a
// clean no-op that says so.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// migrateGitlabAnchors performs (or, under dryRun, plans) the anchor refresh
// over root. It returns the process exit code.
func migrateGitlabAnchors(root string, dryRun bool, stdout, stderr io.Writer) int {
	mode := "apply"
	if dryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(stdout, "statusgen migrate gitlab-anchors (%s) — root %s\n", mode, root)
	path := filepath.Join(root, ".gitlab-ci.yml")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		fmt.Fprintln(stdout, "  no .gitlab-ci.yml — nothing to refresh")
		return migrateExitOK
	}
	if err != nil {
		fmt.Fprintf(stderr, "statusgen migrate: %v\n", err)
		return migrateExitRefused
	}
	body, changed := refreshGitlabAnchors(string(raw), stdout, dryRun)
	if !changed {
		fmt.Fprintln(stdout, "  nothing to change")
		return migrateExitOK
	}
	if dryRun {
		return migrateExitOK
	}
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen migrate: %v\n", err)
		return migrateExitRefused
	}
	if err := os.WriteFile(path, []byte(body), info.Mode().Perm()); err != nil {
		fmt.Fprintf(stderr, "statusgen migrate: %v\n", err)
		return migrateExitRefused
	}
	fmt.Fprintln(stdout, "  wrote .gitlab-ci.yml — commit it so the pipeline runs the refreshed anchors")
	return migrateExitOK
}

// refreshGitlabAnchors returns body with every stale embedded anchor replaced
// by the current rendering, and whether anything changed. It reports one line
// per anchor. A CRLF file keeps its CRLF line endings.
func refreshGitlabAnchors(body string, stdout io.Writer, dryRun bool) (string, bool) {
	crlf := strings.Contains(body, "\r\n")
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	changed := false
	for _, a := range gitlabAnchors {
		start, end, ok := yamlAnchorSpan(lines, a.name)
		if !ok {
			fmt.Fprintf(stdout, "  .%s: not in the file — left alone\n", a.name)
			continue
		}
		want := gitlabAnchorWant(a.name, a.script)
		got, _ := yamlAnchorBlock(strings.Join(lines, "\n"), a.name)
		if got == want {
			fmt.Fprintf(stdout, "  .%s: current\n", a.name)
			continue
		}
		verb := "refreshed"
		if dryRun {
			verb = "WOULD refresh"
		}
		fmt.Fprintf(stdout, "  .%s: %s (lines %d-%d)\n", a.name, verb, start+1, end)
		next := append([]string{}, lines[:start]...)
		next = append(next, strings.Split(want, "\n")...)
		lines = append(next, lines[end:]...)
		changed = true
	}
	out := strings.Join(lines, "\n")
	if crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	return out, changed
}
