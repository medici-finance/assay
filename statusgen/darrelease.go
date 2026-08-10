package main

// darrelease.go — release-pin declarations for a deploy env (oit issue #1333).
//
// dar-sync (darsync.go) validates every env's DAR artifacts against main's
// daml.yaml version. That is right for an env that tracks main, and wrong for
// an env that moves on a release cadence: prod is deliberately held at the last
// released version, so "prod's DAR is not daml.yaml's version" is the INTENDED
// state, not drift. Reported as a PROBLEM it is indistinguishable from a
// mistake, and a red check that is supposed to be red trains everyone to ignore
// it.
//
// The fix is to REPLACE the comparison for such an env, never to remove it:
// an env may declare, in its deploy manifest, that it is pinned to a released
// version, and it is then validated against THAT version — continuously, in
// main, exactly as strictly as before. Dropping the env from the check instead
// would leave it validated nowhere, which is the one outcome worse than a
// false red: a false red produces a signal someone can interrogate, silence
// produces nothing.
//
// The declaration is a `release:` block under the top-level `dar:` key of
// k8s/<env>/app/deploy-manifest.yaml:
//
//	dar:
//	  version: "0.1.44"
//	  sha256: "…"
//	  release:
//	    version: "0.1.44"   # the released DAR version this env is pinned to
//	    tag: "dar/v0.1.44"  # the release that produced it
//	    date: "2026-07-26"  # when it was cut
//
// Three properties make this safe to trust, and they are the whole point of
// the feature:
//
//  1. DECLARED IS CROSS-CHECKED AGAINST DERIVED. The declared version must
//     equal the version read out of the DAR BYTES in this env's DAR ConfigMap
//     set (darsync.go derives it from the zip
//     entry names). A declaration is therefore not a label that silences a
//     check — it is an assertion about an artifact that is verified against
//     that artifact. Without this the pin would be a string anyone could edit
//     to make an env "correct", which is precisely the issue #587 near-miss
//     shape this tooling exists to catch.
//
//  2. IT FAILS CLOSED. A declaration that is present but malformed, partial,
//     carries an unknown key, disagrees with its own manifest's dar.version,
//     is ahead of main, or whose DAR bytes cannot be read to cross-check it, is
//     a hard PROBLEM. There is no parse path that turns a broken declaration
//     into a silent pass or into "unpinned". And a rejected declaration relaxes
//     NOTHING on its way out: darSyncCheck's pinVerified — the gate on both the
//     #587 suppression and the informational notice — requires the pin to have
//     passed the cross-check AND to not be ahead of main, so a pin the checker
//     has already called wrong can never also be the reason a check went quiet.
//
//  3. ABSENCE CHANGES NOTHING. A manifest with no `release:` child under the
//     top-level `dar:` key produces no pin and no problems, so every env that
//     does not declare one — dev, and every other consumer of this checker —
//     behaves bit for bit as it did before.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// darReleaseManifestRel is the single file an env's release pin may be
// declared in. One surface, deliberately: a pin that could be asserted from
// several places is a pin nobody can audit.
func darReleaseManifestRel(env string) string {
	return filepath.Join("k8s", env, "app", "deploy-manifest.yaml")
}

var (
	darReleaseVersionRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	darReleaseTagRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/+-]*$`)
	darReleaseDateRe    = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	// yamlScalarRe matches an unquoted-or-double-quoted scalar with an optional
	// trailing comment. Every field of a release declaration is a single token
	// (semver, tag, ISO date), so a value containing whitespace is malformed
	// rather than something to guess at.
	yamlScalarRe = regexp.MustCompile(`^(?:"([^"]*)"|([^\s"#]*))\s*(?:#.*)?$`)
)

// darReleasePin is a parsed, structurally valid `dar.release` declaration.
// Structural validity is NOT sufficient to trust it — darSyncCheck still
// cross-checks Version against the DAR bytes before honouring the pin.
type darReleasePin struct {
	Version string // released DAR version this env is pinned to
	Tag     string // the release that produced it
	Date    string // when the release was cut (YYYY-MM-DD)
}

// darReleasePinFor reads env's release-pin declaration.
//
// Returns (nil, nil) when the env declares no pin — the default, and the case
// that must stay behaviour-preserving. Returns (nil, problems) when a
// declaration is present but cannot be trusted: a caller that gets no pin and
// no problems may treat the env as unpinned; a caller that gets problems must
// surface them and must NOT treat the env as pinned (falling back to the
// daml.yaml comparison, which is the stricter of the two).
func darReleasePinFor(root, env string) (*darReleasePin, []string) {
	rel := darReleaseManifestRel(env)
	raw, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no deploy manifest → no declaration; today's behaviour
		}
		// Present but unreadable: never assume "unpinned" from an I/O failure.
		return nil, []string{fmt.Sprintf("%s: cannot be read to check for a DAR release pin (oit issue #1333): %v", rel, err)}
	}
	return parseDarReleasePin(rel, string(raw))
}

// parseDarReleasePin extracts the `release:` block nested under the top-level
// `dar:` key of a deploy manifest.
//
// It is an indentation-aware scanner rather than a regex because the value it
// produces decides which version an env is validated against — a regex that
// can match the wrong `version:` (the hazard assay-toolkit #151 hardened
// deployManifestDarVerRe against) would silently repoint the check. Anything
// it cannot read unambiguously is reported, never guessed.
func parseDarReleasePin(rel, text string) (*darReleasePin, []string) {
	lines := strings.Split(text, "\n")

	// Locate every top-level `dar:` key. Duplicates are only interesting when a
	// declaration actually exists (below) — an ambiguous file is a problem, but
	// a file that declares nothing must produce nothing.
	var darStarts []int
	for i, ln := range lines {
		if isBlankOrYAMLComment(ln) || yamlIndent(ln) != 0 {
			continue
		}
		if strings.HasPrefix(strings.TrimRight(ln, " \t"), "dar:") {
			darStarts = append(darStarts, i)
		}
	}
	if len(darStarts) == 0 {
		return nil, nil
	}

	// Find the first top-level dar: block that has a `release:` child.
	var (
		block      []string
		relLineIdx = -1
		childInd   = -1
		declared   = 0
	)
	for _, start := range darStarts {
		b := yamlBlockAfter(lines, start, 0)
		ci, idx := yamlDirectChild(b, "release")
		if idx < 0 {
			continue
		}
		declared++
		if relLineIdx < 0 {
			block, childInd, relLineIdx = b, ci, idx
		}
	}
	if declared == 0 {
		return nil, nil // no declaration anywhere → today's behaviour, exactly
	}
	// From here on a declaration EXISTS, so every ambiguity is a hard problem.
	if declared > 1 || len(darStarts) > 1 {
		return nil, []string{fmt.Sprintf(
			"%s: more than one top-level dar: key — a DAR release pin must be declared exactly once, unambiguously (oit issue #1333)", rel)}
	}
	if strings.ContainsRune(strings.Join(block, "\n"), '\t') {
		return nil, []string{fmt.Sprintf(
			"%s: the dar: block contains a tab — YAML forbids tabs in indentation and the release pin cannot be read reliably (oit issue #1333)", rel)}
	}
	// `release: something` on one line is not a block declaration.
	if v := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(block[relLineIdx]), "release:")); v != "" && !strings.HasPrefix(v, "#") {
		return nil, []string{fmt.Sprintf(
			"%s: dar.release must be a block with version:, tag: and date: keys, not an inline value (oit issue #1333)", rel)}
	}

	sub := yamlBlockAfter(block, relLineIdx, childInd)
	fields := map[string]string{}
	var problems []string
	subInd := -1
	for _, ln := range sub {
		if isBlankOrYAMLComment(ln) {
			continue
		}
		ind := yamlIndent(ln)
		if subInd < 0 {
			subInd = ind
		}
		if ind != subInd {
			problems = append(problems, fmt.Sprintf(
				"%s: dar.release contains a nested or misindented entry (%q) — the block must be exactly version:, tag: and date: (oit issue #1333)", rel, strings.TrimSpace(ln)))
			continue
		}
		key, rest, ok := strings.Cut(strings.TrimSpace(ln), ":")
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: dar.release entry %q is not a key: value pair (oit issue #1333)", rel, strings.TrimSpace(ln)))
			continue
		}
		switch key {
		case "version", "tag", "date":
		default:
			problems = append(problems, fmt.Sprintf(
				"%s: unknown key %q in the dar.release block — a typo must not silently disable the release-pin check (oit issue #1333)", rel, key))
			continue
		}
		m := yamlScalarRe.FindStringSubmatch(strings.TrimSpace(rest))
		if m == nil {
			problems = append(problems, fmt.Sprintf("%s: dar.release.%s has an unreadable value %q (oit issue #1333)", rel, key, strings.TrimSpace(rest)))
			continue
		}
		if _, dup := fields[key]; dup {
			problems = append(problems, fmt.Sprintf("%s: dar.release.%s is declared more than once (oit issue #1333)", rel, key))
			continue
		}
		fields[key] = m[1] + m[2]
	}

	// Every key is required. A partial block is the careless-edit shape: it must
	// name the missing key rather than fall back to "unpinned" (which would
	// silently restore the daml.yaml comparison) or to "pinned" (which would
	// suppress checks on a half-written declaration).
	for _, key := range []string{"version", "tag", "date"} {
		if fields[key] == "" {
			problems = append(problems, fmt.Sprintf(
				"%s: dar.release is missing required key %q — a release pin must declare version, tag and date (oit issue #1333)", rel, key))
		}
	}
	if v := fields["version"]; v != "" && !darReleaseVersionRe.MatchString(v) {
		problems = append(problems, fmt.Sprintf("%s: dar.release.version %q is not a semver (oit issue #1333)", rel, v))
	}
	if v := fields["tag"]; v != "" && !darReleaseTagRe.MatchString(v) {
		problems = append(problems, fmt.Sprintf("%s: dar.release.tag %q is not a plausible release tag (oit issue #1333)", rel, v))
	}
	if v := fields["date"]; v != "" {
		if !darReleaseDateRe.MatchString(v) {
			problems = append(problems, fmt.Sprintf("%s: dar.release.date %q is not YYYY-MM-DD (oit issue #1333)", rel, v))
		} else if _, err := time.Parse("2006-01-02", v); err != nil {
			problems = append(problems, fmt.Sprintf("%s: dar.release.date %q is not a real date (oit issue #1333)", rel, v))
		}
	}

	// The declaring manifest's own dar.version must agree with the pin. These
	// are two statements about the same artifact in the same file; if they can
	// disagree, one of them is decorative — and darVersionPinProblems reads
	// dar.version, so a disagreement would mean the check and the declaration
	// are looking at different versions.
	if dv, ok := darDirectVersion(block, childInd); !ok {
		problems = append(problems, fmt.Sprintf(
			"%s: a dar.release pin is declared but the manifest has no dar.version to check it against (oit issue #1333)", rel))
	} else if v := fields["version"]; v != "" && dv != v {
		problems = append(problems, fmt.Sprintf(
			"%s: dar.version is %s but dar.release.version is %s — the manifest disagrees with its own release pin (oit issue #1333)", rel, dv, v))
	}

	if len(problems) > 0 {
		return nil, problems
	}
	return &darReleasePin{Version: fields["version"], Tag: fields["tag"], Date: fields["date"]}, nil
}

// darDirectVersion returns the `version:` key declared as a DIRECT child of the
// dar: block (indent == childInd), ignoring any nested one. This is the value
// darVersionPinProblems is meant to be reading; comparing it to the release pin
// is what keeps a nested key from being mistaken for the top-level one.
func darDirectVersion(block []string, childInd int) (string, bool) {
	for _, ln := range block {
		if isBlankOrYAMLComment(ln) || yamlIndent(ln) != childInd {
			continue
		}
		key, rest, ok := strings.Cut(strings.TrimSpace(ln), ":")
		if !ok || key != "version" {
			continue
		}
		m := yamlScalarRe.FindStringSubmatch(strings.TrimSpace(rest))
		if m == nil {
			return "", false
		}
		return m[1] + m[2], true
	}
	return "", false
}

// yamlBlockAfter returns the lines belonging to the block opened by lines[start]
// — everything after it, up to (not including) the next non-blank, non-comment
// line indented at or below parentInd. Blank and comment lines never close a
// block.
func yamlBlockAfter(lines []string, start, parentInd int) []string {
	var out []string
	for _, ln := range lines[start+1:] {
		if isBlankOrYAMLComment(ln) {
			out = append(out, ln)
			continue
		}
		if yamlIndent(ln) <= parentInd {
			break
		}
		out = append(out, ln)
	}
	return out
}

// yamlDirectChild finds key as a direct child of a block: the child indent is
// the indent of the block's first meaningful line, and only lines at exactly
// that indent are direct children. Returns (childIndent, index within block),
// or (-1, -1) when the key is not a direct child.
func yamlDirectChild(block []string, key string) (int, int) {
	childInd := -1
	for i, ln := range block {
		if isBlankOrYAMLComment(ln) {
			continue
		}
		ind := yamlIndent(ln)
		if childInd < 0 {
			childInd = ind
		}
		if ind != childInd {
			continue
		}
		if k, _, ok := strings.Cut(strings.TrimSpace(ln), ":"); ok && k == key {
			return childInd, i
		}
	}
	return childInd, -1
}

// yamlIndent counts leading spaces; a tab counts as one column and is rejected
// separately (YAML forbids tabs in indentation).
func yamlIndent(ln string) int {
	n := 0
	for _, r := range ln {
		if r != ' ' && r != '\t' {
			break
		}
		n++
	}
	return n
}

func isBlankOrYAMLComment(ln string) bool {
	t := strings.TrimSpace(ln)
	return t == "" || strings.HasPrefix(t, "#")
}

// semverLess reports whether a < b for dotted numeric triples. Both are known
// to match darReleaseVersionRe before this is called.
func semverLess(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		x, _ := strconv.Atoi(as[i])
		y, _ := strconv.Atoi(bs[i])
		if x != y {
			return x < y
		}
	}
	return false
}
