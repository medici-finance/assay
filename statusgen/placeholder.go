package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Placeholder is the validated frontmatter of a `schema: placeholder-v1` file — a
// REDUCED brief-v1 variant that stands in for an open GitHub issue on the Next-up
// board (issue-loop/01). It duplicates NO spec: the GitHub issue body IS the spec.
// A placeholder carries only the scheduling metadata Next-up needs and points at
// the issue (`issue:`, `repo:`). It is recognized by BOTH its filename
// (`issue-<NN>.md`, or `<repo-slug>-issue-<NN>.md` for a foreign repo) AND the
// `schema: placeholder-v1` discriminator.
//
// A placeholder is DELIBERATELY exempt from the brief-v1 Verify/Evidence/point-
// quality lint: it has no Task/Verify by design (the issue holds acceptance), and
// it is not a brief-*.md file, so every brief-v1 gate — which globs brief-*.md —
// simply never sees it. Its lifecycle is issue-driven (issue-loop/04 close-out).
type Placeholder struct {
	Path      string
	Brief     string   // "<stream>/<stem>", e.g. "issue-loop/issue-300"
	Issue     int      // GitHub issue number
	Repo      string   // "owner/name"
	Wave      int      // default 0
	Effort    string   // DERIVED (default M) unless overridden in frontmatter
	Gate      string   // DERIVED from labels/title (model|human) unless overridden
	Status    string   // lifecycle token; default "todo"
	Labels    []string // issue labels — drives gate derivation + the bug-label boost
	Value     string   // optional low|med|high scoring input (methodology-metrics/14)
	Blocked   string   // issue-loop/03: "" or "awaiting-issue-response"; excludes from Next-up
	BlockedAt string   // ISO 8601 timestamp when blocked was set; "" when not blocked
}

var (
	// placeholderNameRe matches a placeholder filename: issue-<NN>.md, optionally
	// prefixed by a <repo-slug>- for a foreign-repo issue (e.g.
	// assay-toolkit-issue-7.md). Capture group 1 is the issue number. A stray
	// "reissue-2.md" does NOT match — the optional prefix must terminate in "-",
	// so "issue-" is required at a real token boundary.
	placeholderNameRe = regexp.MustCompile(`^(?:[a-z0-9][a-z0-9-]*-)?issue-([0-9]+)\.md$`)

	// issueBranchRe matches a branch that claims work on a GitHub issue —
	// e.g. fix/issue-300, feat/issue-73-onboard-guidance (CLAUDE.md's
	// `git worktree add ../oit-issue-<NN> -b fix/issue-<NN>` recipe). Capture
	// group 1 is the issue number. Unlike the stream/brief branch conventions in
	// claims.go, this keys on the issue, not a <stream>-<NN> pair.
	issueBranchRe = regexp.MustCompile(`^(?:fix|feature|feat|docs|chore)/issue-(\d+)(?:-.+)?$`)

	// placeholderRiskLabels are the issue labels that force a placeholder to
	// gate: human (issue-loop/01 derivation rule).
	placeholderRiskLabels = map[string]bool{"security": true, "funds": true, "daml": true}

	// placeholderRiskKeywordRe matches the daml/auth/funds risk trigger in an
	// issue's title or label text. methodology/31 notes that statusgen sees no
	// diff, so — unlike the review-desk PATH trigger (methodology/30) — the
	// placeholder derivation applies the daml/auth/funds trigger to the issue
	// TEXT the scanner supplies. Case-insensitive, word-boundary.
	placeholderRiskKeywordRe = regexp.MustCompile(`(?i)\b(daml|auth|funds|security)\b`)
)

// derivePlaceholderGate returns the gate a placeholder inherits from its issue:
// "human" when any risk label (security/funds/daml) is present OR the title/labels
// trip the daml/auth/funds trigger, else "model" (issue-loop/01). The DERIVATION
// is the canonical rule the scanner (issue-loop/02) applies when it writes a
// placeholder; a human may override the stored result by writing an explicit
// `gate:` in the file.
func derivePlaceholderGate(labels []string, title string) string {
	for _, l := range labels {
		if placeholderRiskLabels[strings.ToLower(strings.TrimSpace(l))] {
			return "human"
		}
	}
	hay := strings.ToLower(title + " " + strings.Join(labels, " "))
	if placeholderRiskKeywordRe.MatchString(hay) {
		return "human"
	}
	return "model"
}

// derivePlaceholderEffort returns the effort a placeholder inherits. It is always
// "M": an untriaged issue's true size is unknown-until-triaged (issue-loop/01), so
// M is the neutral default. The signature carries labels/title for a future
// size-hinting heuristic; today they are intentionally unused.
func derivePlaceholderEffort(labels []string, title string) string {
	return "M"
}

// placeholderFilePaths returns the sorted, de-duplicated placeholder file paths
// (issue-<NN>.md and <repo-slug>-issue-<NN>.md) in a stream directory. Non-
// conforming names that a loose glob may catch are filtered by placeholderNameRe.
func placeholderFilePaths(s *Stream) []string {
	var matches []string
	for _, pat := range []string{"issue-*.md", "*-issue-*.md"} {
		m, _ := filepath.Glob(filepath.Join(s.Dir, pat))
		matches = append(matches, m...)
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range matches {
		if seen[p] || !placeholderNameRe.MatchString(filepath.Base(p)) {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// parsePlaceholderFile reads a placeholder file and returns its validated
// frontmatter. Return contract mirrors parseBriefFile:
//   - (nil, false, nil)  — exempt: no frontmatter, or no `schema: placeholder-v1`.
//   - (nil, false, err)  — opted-in but malformed (unreadable, bad YAML, or a
//     missing/ill-typed required field). err is already path-prefixed.
//   - (ph,  true,  nil)  — opted-in and structurally valid.
//
// gate/effort are DERIVED from the labels (issue-loop/01) unless the file carries
// an explicit value, which wins (human override). Callers MUST test err before ok.
func parsePlaceholderFile(path string) (*Placeholder, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")

	// A placeholder opts in only via YAML frontmatter.
	first, _, _ := strings.Cut(content, "\n")
	if strings.TrimSpace(first) != "---" {
		return nil, false, nil // no frontmatter → not a placeholder
	}
	fmRaw, _, err := splitFrontmatter(content)
	if err != nil {
		return nil, false, fmt.Errorf("%s: %v", path, err)
	}
	var data map[string]any
	if err := yaml.Unmarshal([]byte(fmRaw), &data); err != nil {
		return nil, false, fmt.Errorf("%s: frontmatter: %w", path, err)
	}
	if s, _ := data["schema"].(string); s != "placeholder-v1" {
		return nil, false, nil // not a placeholder → exempt
	}

	ph := &Placeholder{Path: path, Wave: 0, Status: "todo"}
	var bad []string
	addBad := func(format string, a ...any) { bad = append(bad, fmt.Sprintf(format, a...)) }

	if v, ok := data["brief"].(string); ok {
		ph.Brief = v
	} else {
		addBad("brief must be a string")
	}
	switch n := data["issue"].(type) {
	case int:
		ph.Issue = n
	case int64:
		ph.Issue = int(n)
	default:
		addBad("issue must be an integer")
	}
	if v, ok := data["repo"].(string); ok && strings.TrimSpace(v) != "" {
		ph.Repo = strings.TrimSpace(v)
	} else {
		addBad("repo must be a non-empty string (owner/name)")
	}
	if v, ok := data["wave"]; ok {
		switch w := v.(type) {
		case int:
			ph.Wave = w
		case int64:
			ph.Wave = int(w)
		default:
			addBad("wave must be an integer")
		}
	}
	if v, ok := data["status"]; ok {
		if s, ok := v.(string); ok {
			ph.Status = strings.ToLower(strings.TrimSpace(s))
		} else {
			addBad("status must be a string")
		}
	}
	if v, ok := data["labels"]; ok {
		if lst, err := stringList(v); err == nil {
			ph.Labels = lst
		} else {
			addBad("labels: %v", err)
		}
	}
	if v, ok := data["value"]; ok {
		if s, ok := v.(string); ok {
			ph.Value = s
		} else {
			addBad("value must be a string")
		}
	}
	if v, ok := data["blocked"]; ok {
		if s, ok := v.(string); ok {
			ph.Blocked = strings.TrimSpace(s)
		} else {
			addBad("blocked must be a string")
		}
	}
	if v, ok := data["blockedAt"]; ok {
		// YAML v3 decodes a bare ISO 8601 timestamp as time.Time, not
		// string. Accept both and normalise to the RFC 3339 form the
		// scanner's temporal comparison expects.
		switch t := v.(type) {
		case string:
			ph.BlockedAt = strings.TrimSpace(t)
		case time.Time:
			ph.BlockedAt = t.Format(time.RFC3339)
		default:
			addBad("blockedAt must be a string (ISO 8601) or an unquoted timestamp")
		}
	}
	// title is not a placeholder field (the issue IS the spec); offline the label
	// set is the derivation's trigger surface. A scanner that has the title passes
	// it through the derivation and writes the resulting gate — read back here.
	if v, ok := data["gate"].(string); ok && v != "" {
		ph.Gate = v // explicit human override
	} else {
		ph.Gate = derivePlaceholderGate(ph.Labels, "")
	}
	if v, ok := data["effort"].(string); ok && v != "" {
		ph.Effort = v
	} else {
		ph.Effort = derivePlaceholderEffort(ph.Labels, "")
	}

	if len(bad) > 0 {
		return nil, false, fmt.Errorf("%s: %s", path, strings.Join(bad, "; "))
	}
	return ph, true, nil
}

// placeholderNum is the synthetic Brief number for a placeholder: the filename
// stem (unique per file, collision-free even when two repos share an issue
// number). e.g. "issue-300", "assay-toolkit-issue-7".
func placeholderNum(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".md")
}

// toBrief renders a placeholder as a synthetic Brief so the whole Next-up
// pipeline — eligibility, scoring, claim-awareness, rendering — treats it as a
// first-class row with no second code path (issue-loop/01). The derived gate is
// surfaced in the Title because the Next-up table has no gate column.
func (p *Placeholder) toBrief() Brief {
	title := fmt.Sprintf("%s#%d — see issue (gate:%s, effort:%s)", p.Repo, p.Issue, p.Gate, p.Effort)
	if p.Blocked != "" {
		title = fmt.Sprintf("[blocked:%s] %s", p.Blocked, title)
	}
	return Brief{
		Num:     placeholderNum(p.Path),
		Title:   title,
		Wave:    p.Wave,
		Effort:  p.Effort,
		Status:  p.Status,
		Schema:  "placeholder-v1",
		Value:   p.Value,
		Blocked: p.Blocked,
	}
}

// attachPlaceholders parses every placeholder file in each stream directory,
// stores the parsed set on the stream, and appends a synthetic Brief per
// placeholder so Next-up (and the rest of the pipeline) sees it. Malformed
// placeholders are skipped here — checkPlaceholderFiles reports them as hard
// problems, which short-circuits run() before the view is built. Idempotent per
// process: run() calls it once on freshly-loaded streams.
func attachPlaceholders(streams []*Stream) {
	for _, s := range streams {
		for _, path := range placeholderFilePaths(s) {
			ph, ok, err := parsePlaceholderFile(path)
			if err != nil || !ok {
				continue
			}
			s.Placeholders = append(s.Placeholders, *ph)
			s.Briefs = append(s.Briefs, ph.toBrief())
		}
	}
}

// checkPlaceholderFiles validates every placeholder file across all streams. It
// mirrors checkBriefFiles' opt-in discipline (only schema: placeholder-v1 files
// are checked) but asserts ONLY the reduced schema is well-formed — a placeholder
// is exempt from the Verify/Evidence/point-quality gates by design (issue-loop/01;
// acceptance lives in the GitHub issue, not the file). It performs its own file
// discovery, like checkBriefFiles, and is wired into run() alongside it.
func checkPlaceholderFiles(streams []*Stream) (problems, notices []string) {
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	for _, s := range streams {
		for _, path := range placeholderFilePaths(s) {
			ph, ok, err := parsePlaceholderFile(path)
			if err != nil {
				add("%s", err.Error()) // already path-prefixed
				continue
			}
			if !ok {
				continue // opted-out → exempt
			}
			m := placeholderNameRe.FindStringSubmatch(filepath.Base(path))
			if m == nil {
				add("%s: placeholder filename must match [<repo-slug>-]issue-<NN>.md", path)
				continue
			}
			if strconv.Itoa(ph.Issue) != m[1] {
				add("%s: frontmatter issue %d != filename issue %s", path, ph.Issue, m[1])
			}
			wantID := s.Name + "/" + placeholderNum(path)
			if ph.Brief != wantID {
				add("%s: brief %q does not match filename-derived id %q", path, ph.Brief, wantID)
			}
			if !validEffort[ph.Effort] {
				add("%s: invalid effort %q (want S, M or L)", path, ph.Effort)
			}
			if !validGate[ph.Gate] {
				add("%s: invalid gate %q (want model or human)", path, ph.Gate)
			}
			if !validBriefStatus[ph.Status] {
				add("%s: invalid status %q", path, ph.Status)
			}
			if ph.Value != "" && !validValue[ph.Value] {
				add("%s: invalid value %q (want low, med or high)", path, ph.Value)
			}
			if ph.Blocked != "" && ph.Blocked != "awaiting-issue-response" {
				add("%s: invalid blocked %q (want \"awaiting-issue-response\" or omit)", path, ph.Blocked)
			}
			if ph.Blocked != "" && ph.BlockedAt == "" {
				add("%s: blocked is set but blockedAt is missing — blockedAt must be an ISO 8601 timestamp", path)
			}
			if ph.BlockedAt != "" {
				if _, err := time.Parse(time.RFC3339, ph.BlockedAt); err != nil {
					add("%s: blockedAt %q is not a valid RFC 3339 timestamp", path, ph.BlockedAt)
				}
			}
		}
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

// claimedPlaceholders maps open issue-branches to placeholder claim keys
// ("<stream>/<stem>") so a placeholder is excluded from Next-up while its issue
// has an open branch (methodology-metrics/08 claim-awareness, applied to the
// issue). A branch names an issue NUMBER; the key resolves through the parsed
// placeholders (requires attachPlaceholders to have run) so the excluded row is
// exactly the placeholder for that issue — never a coincidental stream/brief. The
// result merges into the same claimed map claimedBriefs feeds nextUp.
func claimedPlaceholders(streams []*Stream, branches []string) map[string]bool {
	claimed := map[string]bool{}
	for _, b := range branches {
		m := issueBranchRe.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		issue, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		for _, s := range streams {
			for _, ph := range s.Placeholders {
				if ph.Issue == issue {
					claimed[s.Name+"/"+placeholderNum(ph.Path)] = true
				}
			}
		}
	}
	return claimed
}
