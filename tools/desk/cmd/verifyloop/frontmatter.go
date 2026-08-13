package main

import (
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// parseFrontmatter extracts the brief-v1 fields the verify adapter needs from a brief file.
// It is a deliberately small line/regex extractor (no YAML dep — the desk module stays
// self-contained). It handles both the inline flow form
//
//	risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
//
// and per-line `key: value` forms for gate / effort / exec-tier / implementer.
func parseFrontmatter(content string) briefFrontmatter {
	fm := briefFrontmatter{}
	body := frontmatterBlock(content)
	if body == "" {
		return fm
	}
	fm.Gate = strings.TrimSpace(scalar(body, "gate"))
	fm.Effort = strings.TrimSpace(scalar(body, "effort"))
	fm.ExecTier = strings.TrimSpace(scalar(body, "exec-tier"))
	fm.Implementer = strings.TrimSpace(scalar(body, "implementer"))
	fm.Risk = parseRisk(body)
	return fm
}

// frontmatterBlock returns the text between the leading `---` fences, or "" if absent.
func frontmatterBlock(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n")
		}
	}
	return ""
}

// scalar returns the value of a top-level `key: value` line in the frontmatter block.
func scalar(body, key string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:\s*(.*)$`)
	if m := re.FindStringSubmatch(body); m != nil {
		return strings.Trim(strings.TrimSpace(m[1]), `"'`)
	}
	return ""
}

// parseRisk reads the four canonical risk answers from either the inline flow map or
// per-line entries. A key answered "yes" (case-insensitive) sets its flag.
func parseRisk(body string) loopengine.RiskFlags {
	yes := func(key string) bool {
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(key) + `\s*:\s*yes`)
		return re.MatchString(body)
	}
	return loopengine.RiskFlags{
		Regulatory:    yes("regulatory"),
		Customer:      yes("customer"),
		Irreversible:  yes("irreversible"),
		SensitiveData: yes("sensitive-data"),
	}
}

// extractEvidence returns the body of the `## Evidence` section (between that exact heading
// and the next `## `). Mirrors statusgen/brieffile.go extractEvidence: the heading must match
// `## Evidence` exactly.
func extractEvidence(content string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Evidence" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, l := range lines[start:] {
		if strings.HasPrefix(strings.TrimSpace(l), "## ") {
			break
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

var htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

// evidenceHasContent reports whether an Evidence section has real content beyond the
// contract HTML comment / whitespace (mirrors statusgen's evidenceHasContent).
func evidenceHasContent(evidence string) bool {
	stripped := htmlCommentRe.ReplaceAllString(evidence, "")
	return strings.TrimSpace(stripped) != ""
}
