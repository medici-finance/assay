package main

// versiongate.go — two release-boundary controls from derived-board/06 (spec §6):
//
//  1. The BRIEF-READING version gate. brief-v2 is the first contract-breaking
//     schema; a statusgen built BEFORE v1.0.0 that RECOGNISES brief-v2 (i.e. a
//     build cut between derived-board/03 landing the parser and v1.0.0 cutting the
//     schema) must still refuse to read a v2 tree, because the fleet's version
//     boundary — not the parser's feature set — is what an adopter reasons about.
//     The refusal is VERSION-GATED by the build stamp (`-ldflags -X
//     main.statusgenVersion=vX.Y.Z`): an unstamped local build reports "dev" and
//     behaves as "latest" (no refusal), so running from source is never blocked.
//     The #271 fail-closed trap in parseBriefFile is the SEPARATE control for a
//     build too old to recognise brief-v2 at all; this one covers the build that
//     recognises it but predates its release.
//
//  2. The SAME-TAG pin lint. `.assay-versions` pins each artifact separately, so
//     an adopter can land statusgen v1.0.0 next to desk-tools v0.13.0 reading one
//     tree — the mixed-version misread §6 closes. `--lint` PROBLEMs an
//     .assay-versions whose artifact tags differ. One tag, one tree; no separate
//     min-version matrix.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// statusgenExitTreeTooNew is the exit code a brief-reading tool returns when the
// tree is on a brief schema newer than the tool's release boundary. It mirrors the
// deskkit "unverifiable" code (6): the tool cannot soundly read this tree.
const statusgenExitTreeTooNew = 6

// gateParseVersion parses a strict bare `vX.Y.Z` into a comparable tuple, ok=false
// for any other shape (including "dev" / "" — an unstamped build, treated as
// latest by the caller).
func gateParseVersion(tag string) (v [3]int, ok bool) {
	if !strings.HasPrefix(tag, "v") {
		return v, false
	}
	parts := strings.Split(tag[1:], ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// gateBelowV1 reports whether a STAMPED version parses and is strictly below
// v1.0.0. An unstamped build ("dev"/"" — gateParseVersion ok=false) is treated as
// latest and returns false: source builds are never gated.
func gateBelowV1(tag string) bool {
	v, ok := gateParseVersion(tag)
	if !ok {
		return false
	}
	return v[0] < 1
}

// treeHasBriefV2 reports whether any brief file under <root>/docs/streams declares
// `schema: brief-v2`. Pure, offline, and cheap-first: it stops at the first match.
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

// refuseIfTreeTooNew implements control (1) for statusgen: if any resolved root is
// a brief-v2 tree and this binary is a STAMPED build below v1.0.0, it writes the
// one-line upgrade message to stderr and returns the refusal exit code (>0). It
// returns 0 (proceed) for an unstamped/latest build or a tree with no brief-v2.
func refuseIfTreeTooNew(roots []string, version string, stderr io.Writer) int {
	if !gateBelowV1(version) {
		return 0
	}
	for _, r := range roots {
		if treeHasBriefV2(r) {
			fmt.Fprintf(stderr,
				"statusgen: tree is brief-v2; this statusgen is %s; run assay:upgrade-assay\n", version)
			return statusgenExitTreeTooNew
		}
	}
	return 0
}

// assayVersionsPinTagConsistency implements control (2): read <root>/.assay-versions
// and return a PROBLEM string when the ARTIFACT lines (every line but the umbrella
// line) do not all carry the same tag. It is a no-op — no problem — when the file
// is ABSENT (not every adopted tree carries a pin file; this repo's own root does
// not) or unreadable in a way that other checks already surface. A file present
// with differing artifact tags is the mixed-version state §6 exists to catch.
func assayVersionsPinTagConsistency(root string) (problem string, ok bool) {
	path := filepath.Join(root, ".assay-versions")
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false // absent/unreadable → not applicable, never a false PROBLEM
	}
	tags := map[string][]string{} // tag -> artifacts carrying it
	for _, line := range strings.Split(string(raw), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		// Strip trailing `# comment`.
		if i := strings.IndexByte(t, '#'); i >= 0 {
			t = strings.TrimSpace(t[:i])
		}
		fields := strings.Fields(t)
		if len(fields) < 2 {
			continue
		}
		artifact, tag := fields[0], fields[1]
		// The umbrella line (`assay vX.Y.Z`, no sha256) names a COMPOSITION, not an
		// artifact — it is deliberately excluded from the same-tag comparison.
		if artifact == "assay" {
			continue
		}
		tags[tag] = append(tags[tag], artifact)
	}
	if len(tags) <= 1 {
		return "", false
	}
	var parts []string
	for tag, arts := range tags {
		parts = append(parts, fmt.Sprintf("%s (%s)", tag, strings.Join(arts, ", ")))
	}
	// Stable ordering for a deterministic message.
	strSort(parts)
	return fmt.Sprintf(
		".assay-versions: artifact tags differ across pinned artifacts — %s. "+
			"statusgen and desk-tools read one tree and must be pinned to the SAME tag "+
			"(one tag, one tree; see the derived-board bundle-versioning rule)",
		strings.Join(parts, "; ")), true
}

// strSort is a tiny in-place sort avoiding a sort import churn in this file.
func strSort(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
