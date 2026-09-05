package deskkit

// briefschemagate.go — the brief-reading version gate (example-stream/06 §6).
//
// brief-v2 is the first contract-breaking brief schema. A brief-reading desk tool
// (deskboard, deskpr, deskclaim, deskevidence) built BELOW v1.0.0 would misread a
// brief-v2 tree — typed deps, gate derivation and the generated board all mean
// something different under v2 — so each such tool REFUSES a v2 tree with exit 6
// and a one-line "run assay:upgrade-assay" message, rather than reading it wrong.
//
// The gate is VERSION-KEYED off the build stamp: an unstamped local build (`dev` /
// empty / unparseable) is treated as LATEST and never gated, so running from
// source is never blocked; a build stamped at or above v1.0.0 is never gated. Only
// a stamped build strictly below v1.0.0 reading a v2 tree is refused. The version
// argument accepts both the bare `vX.Y.Z` shape (a `-X main.version` test/verify
// stamp) and the release-namespaced `desk-tools/vX.Y.Z` shape (the shape
// release-desk.yml stamps into ReleaseTag), so the SAME gate covers a real release
// and a verify-row build.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// briefSchemaV2 is the schema value that trips the gate.
const briefSchemaV2 = "brief-v2"

// gateBelowV1 reports whether a STAMPED version parses (bare or namespaced) and is
// strictly below v1.0.0. An unstamped/unparseable version ("dev", "") is treated
// as latest and returns false.
func gateBelowV1(version string) bool {
	v, err := parseVersion(version)
	if err != nil {
		return false // dev / unparseable → latest, never gated
	}
	return v[0] < 1
}

// treeHasBriefV2 reports whether any brief file under <root>/docs/streams declares
// `schema: brief-v2`. Pure, offline, cheap-first (stops at the first match).
func treeHasBriefV2(root string) bool {
	base := filepath.Join(root, "docs", "streams")
	found := false
	_ = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if !strings.HasPrefix(name, "brief-") || !strings.HasSuffix(name, ".md") {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for _, line := range strings.Split(string(raw), "\n") {
			t := strings.TrimSpace(line)
			if t == "schema: "+briefSchemaV2 || t == "schema: \""+briefSchemaV2+"\"" {
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}

// RefuseIfTreeV2BelowV1 is the gate a brief-reading tool calls before it reads any
// tree. If the tool's version is a stamped build below v1.0.0 AND any of roots is
// a brief-v2 tree, it writes the upgrade message to stderr and returns
// ExitUnverifiable (6). Otherwise it returns 0 (proceed).
func RefuseIfTreeV2BelowV1(roots []string, version, tool string, stderr io.Writer) int {
	if !gateBelowV1(version) {
		return 0
	}
	for _, r := range roots {
		if treeHasBriefV2(r) {
			fmt.Fprintf(stderr, "%s: tree is brief-v2; this %s is %s; run assay:upgrade-assay\n", tool, tool, version)
			return ExitUnverifiable
		}
	}
	return 0
}

// RootsFromArgs extracts every `--root <dir>` (repeatable) from a raw arg slice,
// defaulting to ["."] when none is given. It is deliberately permissive — it reads
// the flag WITHOUT owning it, so a tool that has no --root of its own still gets
// its working directory scanned, and one that does still resolves the same paths.
func RootsFromArgs(args []string) []string {
	var roots []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			roots = append(roots, args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--root=") {
			roots = append(roots, strings.TrimPrefix(args[i], "--root="))
		}
	}
	if len(roots) == 0 {
		return []string{"."}
	}
	return roots
}

// EffectiveToolVersion resolves a brief-reading tool's version for the gate: the
// explicit `-X main.version` stamp when set, else the release-namespaced
// ReleaseTag (`desk-tools/vX.Y.Z`) release-desk.yml stamps, else "dev". This lets
// the SAME gate cover a verify-row build (bare `-X main.version=v0.13.0`) and a
// real release (namespaced ReleaseTag).
func EffectiveToolVersion(mainVersion string) string {
	if strings.TrimSpace(mainVersion) != "" {
		return mainVersion
	}
	return ReleaseTagOrDev()
}
