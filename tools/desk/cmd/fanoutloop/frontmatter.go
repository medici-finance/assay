package main

import (
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// briefFrontmatter is the subset of brief-v1 frontmatter the batch adapter needs. As in
// verifyloop it is parsed with a small line/regex extractor rather than a YAML dep, so the desk
// module stays self-contained (statusgen is package main and unimportable anyway).
type briefFrontmatter struct {
	Gate        string
	Effort      string
	ExecTier    string
	Implementer string
	Risk        loopengine.RiskFlags
}

func parseFrontmatter(content string) briefFrontmatter {
	fm := briefFrontmatter{}
	body := frontmatterBlock(content)
	if body == "" {
		return fm
	}
	fm.Gate = scalar(body, "gate")
	fm.Effort = scalar(body, "effort")
	fm.ExecTier = scalar(body, "exec-tier")
	fm.Implementer = scalar(body, "implementer")
	fm.Risk = parseRisk(body)
	return fm
}

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

func scalar(body, key string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:\s*(.*)$`)
	if m := re.FindStringSubmatch(body); m != nil {
		return strings.Trim(strings.TrimSpace(m[1]), `"'`)
	}
	return ""
}

func parseRisk(body string) loopengine.RiskFlags {
	yes := func(key string) bool {
		return regexp.MustCompile(`(?i)` + regexp.QuoteMeta(key) + `\s*:\s*yes`).MatchString(body)
	}
	return loopengine.RiskFlags{
		Regulatory:    yes("regulatory"),
		Customer:      yes("customer"),
		Irreversible:  yes("irreversible"),
		SensitiveData: yes("sensitive-data"),
	}
}
