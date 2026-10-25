package main

import (
	"github.com/medici-finance/assay/statusgen/briefdoc"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// briefFrontmatter is the subset of brief-v1 frontmatter the batch adapter needs. As in
// verifyloop it is parsed with a small line/regex extractor rather than a YAML dep, so the desk
// module stays self-contained (statusgen is package main and unimportable anyway).
type briefFrontmatter struct {
	ParseError  string
	Gate        string
	Effort      string
	ExecTier    string
	Implementer string
	Risk        loopengine.RiskFlags
}

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
	fm.Gate = f.Values["gate"]
	fm.Effort = f.Values["effort"]
	fm.ExecTier = f.Values["exec-tier"]
	fm.Implementer = f.Values["implementer"]
	fm.Risk = loopengine.RiskFlags(f.Risk)
	return fm
}

func frontmatterBlock(content string) string { return briefdoc.Frontmatter(content) }

func scalar(body, key string) string { return briefdoc.Scalar(body, key) }

func parseRisk(body string) loopengine.RiskFlags {
	r, e := briefdoc.Risks(body)
	if e != nil {
		r = briefdoc.UnreadableRisk()
	}
	return loopengine.RiskFlags(r)
}
