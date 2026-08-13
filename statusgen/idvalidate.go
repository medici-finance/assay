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

// ----- ID format validation -----

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
	if hasNoGitDir(root) {
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
// returned empty because the landed set could not be determined — either
// origin/main could not be resolved (a real git checkout: shallow clone,
// offline, no remote), or root has no .git directory at all (a `git archive`
// export). In the origin/main case, idFormatProblems still fails
// CLOSED (T9) and fires numeric-regression PROBLEMs against every
// legitimately-landed legacy numeric entry, with messages that say the entry
// must use slug-form IDs rather than stating the real cause; this NOTICE
// names it. In the no-.git case, idFormatProblems now SKIPS the
// numeric-regression rule outright instead of mis-firing (see
// idFormatProblems), so this NOTICE instead explains why that rule did not
// run at all — the point is the same either way: a run against this tree is
// not comparable to a run against a real worktree, and a silent difference in
// which checks ran is what makes a differential lint comparison unsound.
//
// Only emits when there are actually register entries to check (an empty
// register has nothing to mis-fire on, and nothing worth degrading). Advisory
// only — never a hard problem.
func grandfatheredBaseFallbackNotices(root string) []string {
	var cause string
	switch {
	case hasNoGitDir(root):
		cause = "this tree has no .git directory at all (e.g. a `git archive` export), so no register-ID grandfathering could be determined and the numeric-regression rule was skipped entirely rather than mis-fire against every pre-existing legacy-numeric entry"
	default:
		mb, err := exec.Command("git", "-C", root, "merge-base", "HEAD", "origin/main").Output()
		if err == nil && strings.TrimSpace(string(mb)) != "" {
			return nil // origin/main resolved — no fallback needed
		}
		cause = "origin/main could not be resolved, so the grandfathered set is empty and all register entries are treated as new — legitimately-landed legacy numeric entries will fire numeric-regression PROBLEMs whose messages do not name this as the cause"
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
		"register ID validation is running degraded: %s (%d register entr%s affected). If this is CI, fetch origin/main before the lint step; if this is a git-archive export, lint a real worktree instead (`git worktree add`) for a result comparable to CI.",
		cause, n, map[bool]string{true: "y is", false: "ies are"}[n == 1])}
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

	// The numeric-regression rule below exists to catch a NEW entry minted
	// with a numeric id — it needs git history to tell that apart from an
	// old, already-landed one, which is exactly what `grandfathered` encodes.
	// When root has no .git directory at all (a `git archive` export),
	// grandfatheredIDs necessarily returns empty — there is no history
	// to read — and running the regression rule anyway would flag EVERY
	// pre-existing legacy-numeric entry as though it were freshly minted on
	// this branch, one spurious PROBLEM per legacy entry. That is a different situation from a real git
	// checkout whose origin/main happens to be unresolvable, where
	// grandfatheredIDs' fail-CLOSED default (T9) is intentional — an
	// adversarial branch could otherwise manufacture "this ref can't be
	// resolved" to win a bypass. There is no checkout to be adversarial about
	// in a git-archive export, so the rule is skipped outright rather than
	// mis-firing; grandfatheredBaseFallbackNotices reports the degradation.
	skipNumericRegression := hasNoGitDir(root)

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
		// Not git-dependent — a genuinely malformed id is malformed regardless
		// of whether history is available, so this rule always runs.
		if !isValidRegisterID(e.ID) {
			problems = append(problems, fmt.Sprintf(
				"intake register: invalid id %q — must be %s (new slug form: 10-20 chars after prefix, [a-z0-9-], starts/ends alphanumeric) or %s (legacy numeric, grandfathered)",
				e.ID, "[FI]-<slug>", "[FI]-NN(-a)?"))
		}

		// Numeric-regression check: a new entry using a numeric-form ID
		// is a regression to the counter. Skipped entirely with no .git (see
		// skipNumericRegression above).
		if !skipNumericRegression && isLegacyNumericID(e.ID) {
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
		if !skipNumericRegression && isLegacyNumericID(e.ID) {
			problems = append(problems, fmt.Sprintf(
				"findings register: %s uses numeric id %q — new entries must use slug-form ids (10-20 chars after prefix, [a-z0-9-]); numeric ids are frozen legacy",
				e.Date, e.ID))
		}
	}

	return problems
}
