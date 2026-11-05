package main

import (
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// parseFrontmatter projects the shared YAML parser into the verify adapter's
// fields. Unreadable metadata takes the conservative human/strong/risk profile.
func parseFrontmatter(content string) briefFrontmatter {
	fm := briefFrontmatter{}
	body, _, err := briefdoc.Split(content)
	if err != nil {
		fm.Gate = "human"
		fm.ExecTier = "strong"
		fm.Risk = loopengine.RiskFlags(briefdoc.UnreadableRisk())
		fm.ParseError = err.Error()
		return fm
	}
	f, err := briefdoc.ParseFields(body)
	if err != nil {
		fm.Gate = "human"
		fm.ExecTier = "strong"
		fm.Risk = loopengine.RiskFlags(briefdoc.UnreadableRisk())
		fm.ParseError = err.Error()
		return fm
	}
	// This loop trimmed space from every value its earlier reader returned.
	fm.Gate = strings.TrimSpace(f.Values["gate"])
	fm.Effort = strings.TrimSpace(f.Values["effort"])
	fm.ExecTier = strings.TrimSpace(f.Values["exec-tier"])
	fm.Implementer = strings.TrimSpace(f.Values["implementer"])
	fm.BlockedUntil = strings.TrimSpace(f.Values["blocked-until"])
	fm.VerifyLane = strings.TrimSpace(f.Values["verify-lane"])
	fm.InRepair = strings.TrimSpace(f.Values["in-repair"])
	fm.Risk = loopengine.RiskFlags(f.Risk)
	return fm
}

// frontmatterBlock returns the text between the leading `---` fences, or "" if absent.
func frontmatterBlock(content string) string { return briefdoc.Frontmatter(content) }

// scalar returns the value of a top-level `key: value` line in the frontmatter block.
func scalar(body, key string) string { return briefdoc.Scalar(body, key) }

// parseRisk reads the four canonical risk answers from either the inline flow map or
// per-line entries. A key answered "yes" (case-insensitive) sets its flag.
func parseRisk(body string) loopengine.RiskFlags {
	r, e := briefdoc.Risks(body)
	if e != nil {
		r = briefdoc.UnreadableRisk()
	}
	return loopengine.RiskFlags(r)
}

// extractEvidence returns the body of the `## Evidence` section (between that exact heading
// and the next `## `). Mirrors statusgen/brieffile.go extractEvidence: the heading must match
// `## Evidence` exactly.
func extractEvidence(content string) string { return briefdoc.Section(content, "Evidence", false) }

// extractVerify returns the body of the `## Verify` section (between that heading and the next
// `## `). Unlike extractEvidence, the heading is matched as a PREFIX: a real brief's Verify
// heading carries an author-appended qualifier, e.g. `## Verify (executable — no prose-only DoD
// items)`, so an exact `## Verify` match reads an empty section for every real brief and the
// bucketer derives nothing — the load-bearing bug that kept the deferred/online-lane buckets from
// ever firing on the board. The bucketer derives the online-lane and longitudinal signals from
// this text when a brief carries no explicit marker (see briefscan.go).
func extractVerify(content string) string { return briefdoc.Section(content, "Verify", true) }

// evidenceHasContent reports whether an Evidence section has real content beyond the
// contract HTML comment / whitespace (mirrors statusgen's evidenceHasContent).
func evidenceHasContent(evidence string) bool { return briefdoc.HasContent(evidence) }
