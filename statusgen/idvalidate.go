package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ----- ID format validation (methodology/35) -----

// newFormatIDRe matches slug-form register IDs: F-<slug> or I-<slug> where
// the slug is 10-20 characters, [a-z0-9-], starts and ends with [a-z0-9].
// Full ID: 12-22 characters total (2-char prefix + 10-20 char slug).
var newFormatIDRe = regexp.MustCompile(`^[FI]-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)

// legacyNumericIDRe matches legacy numeric IDs: F-NN or I-NN, plus the
// collision-suffix form F-NN-a (grandfathered per human:<name>'s call on F-22-a).
var legacyNumericIDRe = regexp.MustCompile(`^[FI]-\d+(-[a-z])?$`)

// isValidRegisterID returns true when id matches EITHER the new slug format
// or the legacy numeric (grandfathered) format.
func isValidRegisterID(id string) bool {
	return newFormatIDRe.MatchString(id) || legacyNumericIDRe.MatchString(id)
}

// isNewFormatID returns true when id matches the new slug format.
func isNewFormatID(id string) bool {
	return newFormatIDRe.MatchString(id)
}

// isLegacyNumericID returns true when id matches the legacy numeric format.
func isLegacyNumericID(id string) bool {
	return legacyNumericIDRe.MatchString(id)
}

// ----- numeric-regression check -----

// grandfatheredIDs returns the set of register entry IDs that exist at the
// merge-base of HEAD and origin/main — the "landed" set. These entries are
// grandfathered: any valid ID format is accepted.
//
// T9: When origin/main can't be resolved, returns an empty set (fail-closed):
// all IDs are treated as new and subject to the uniqueness/numeric-regression
// checks. The old fallback to base=HEAD would grandfather the very ID the
// check exists to reject — a fail-open hole.
//
// Unlike the earlier file-path-derivation approach, this reads each file's
// YAML frontmatter directly from the git tree at the merge-base, so it works
// regardless of whether the filename was derived from slugFromTitle or
// hand-crafted.
func grandfatheredIDs(root string) map[string]bool {
	out := map[string]bool{}
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		return out // not a git checkout — no legacy to freeze against
	}
	mb, err := exec.Command("git", "-C", root, "merge-base", "HEAD", "origin/main").Output()
	if err != nil || strings.TrimSpace(string(mb)) == "" {
		// T9: fail-closed — origin/main unresolvable, treat all IDs as new.
		// The old fallback to base=HEAD grandfathered the brand-new numeric ID
		// the check exists to reject.
		return out
	}
	base := strings.TrimSpace(string(mb))
	for _, dir := range []string{"docs/streams/intake", "docs/streams/findings"} {
		// List all .md files at the merge-base.
		lsCmd := exec.Command("git", "-C", root, "ls-tree", "--name-only", "-r", base, dir+"/")
		lsOut, err := lsCmd.Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(lsOut), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || !strings.HasSuffix(line, ".md") {
				continue
			}
			// Read the file's YAML frontmatter from the merge-base git tree.
			showCmd := exec.Command("git", "-C", root, "show", base+":"+line)
			showOut, err := showCmd.Output()
			if err != nil {
				continue
			}
			id := extractIDFromYAMLFrontmatter(showOut)
			if id != "" {
				out[id] = true
			}
		}
	}
	return out
}

// extractIDFromYAMLFrontmatter parses the "id:" field from a YAML
// frontmatter block. Returns "" if no id is found.
// grandfatheredBaseFallbackNotices emits a NOTICE when grandfatheredIDs
// returned empty because origin/main could not be resolved — the numeric-
// regression check (idFormatProblems) then fires PROBLEMs against every
// legitimately-landed legacy numeric entry, with messages that say the
// entry must use slug-form IDs rather than stating the real cause (the
// base could not be resolved). This NOTICE names the real cause so the
// run is not misleading.
//
// Only emits when there are actually register entries to check (an empty
// register has nothing to mis-fire on). Advisory only — never a hard
// problem.
func grandfatheredBaseFallbackNotices(root string) []string {
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		return nil
	}
	mb, err := exec.Command("git", "-C", root, "merge-base", "HEAD", "origin/main").Output()
	if err == nil && strings.TrimSpace(string(mb)) != "" {
		return nil // origin/main resolved — no fallback needed
	}
	// Only worth saying when there are actually register entries.
	n := 0
	for _, dir := range []string{"docs/streams/intake", "docs/streams/findings"} {
		files, readErr := os.ReadDir(filepath.Join(root, dir))
		if readErr != nil {
			continue
		}
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
				n++
			}
		}
	}
	if n == 0 {
		return nil
	}
	return []string{fmt.Sprintf(
		"register ID validation is running degraded: origin/main could not be resolved, so the grandfathered set is empty and all %d register entr%s are treated as new — legitimately-landed legacy numeric entries will fire numeric-regression PROBLEMs whose messages do not name this as the cause. If this is CI, fetch origin/main before the lint step.",
		n, map[bool]string{true: "y is", false: "ies are"}[n == 1])}
}

func extractIDFromYAMLFrontmatter(raw []byte) string {
	fm, _, err := splitFrontmatter(string(raw))
	if err != nil {
		return ""
	}
	var entry struct {
		ID string `yaml:"id"`
	}
	if err := yaml.Unmarshal([]byte(fm), &entry); err != nil {
		return ""
	}
	return entry.ID
}

// idFormatProblems validates every register entry's ID and enforces the
// numeric-form regression rule (new entries must use slug format).
// Returns lint PROBLEM strings; empty slice = all IDs valid.
func idFormatProblems(root string) []string {
	var problems []string

	// Gather the set of IDs that are grandfathered (exist at the merge-base).
	grandfathered := grandfatheredIDs(root)

	// --- intake ---
	intakeEntries, err := parseIntakeDir(root)
	if err != nil {
		return []string{fmt.Sprintf("id format: reading intake: %v", err)}
	}
	for _, e := range intakeEntries {
		// Grandfathered entries (exist at merge-base) are exempt from all
		// ID format rules — they were committed before this brief and freeze
		// in whatever form they had.
		if grandfathered[e.ID] {
			continue
		}

		// Format check: new entries must match either new slug or legacy numeric.
		if !isValidRegisterID(e.ID) {
			problems = append(problems, fmt.Sprintf(
				"intake register: invalid id %q — must be %s (new slug form: 10-20 chars after prefix, [a-z0-9-], starts/ends alphanumeric) or %s (legacy numeric, grandfathered)",
				e.ID, "[FI]-<slug>", "[FI]-NN(-a)?"))
		}

		// Numeric-regression check: a new entry using a numeric-form ID
		// is a regression to the counter.
		if isLegacyNumericID(e.ID) {
			problems = append(problems, fmt.Sprintf(
				"intake register: %s uses numeric id %q — new entries must use slug-form ids (10-20 chars after prefix, [a-z0-9-]); numeric ids are frozen legacy",
				e.Date, e.ID))
		}
	}

	// --- findings ---
	findingEntries, err := parseFindingsDir(root)
	if err != nil {
		return append(problems, fmt.Sprintf("id format: reading findings: %v", err))
	}
	for _, e := range findingEntries {
		if grandfathered[e.ID] {
			continue
		}
		if !isValidRegisterID(e.ID) {
			problems = append(problems, fmt.Sprintf(
				"findings register: invalid id %q — must be %s (new slug form: 10-20 chars after prefix, [a-z0-9-], starts/ends alphanumeric) or %s (legacy numeric, grandfathered)",
				e.ID, "[FI]-<slug>", "[FI]-NN(-a)?"))
		}
		if isLegacyNumericID(e.ID) {
			problems = append(problems, fmt.Sprintf(
				"findings register: %s uses numeric id %q — new entries must use slug-form ids (10-20 chars after prefix, [a-z0-9-]); numeric ids are frozen legacy",
				e.Date, e.ID))
		}
	}

	return problems
}
