package main

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"strings"
	"testing"
)

const unattributed4114 = "| # | Class | Command | Acceptance result | Supplemental host result (no credit) |\n|---|---|---|---|---|\n| 9 | check | statusgen --lint | pass | exit 0 |\n| 10 | check:ci | check | could-not-run: closure refused | no credit |\n"
const attributedBlocked = "| # | Command | Observed | Date | Runner |\n|---|---|---|---|---|\n| 9 | statusgen --lint | exit 0 | 2026-10-23 | human:operator |\n| 10 | check | could-not-run: closure refused | 2026-10-23 | human:operator |\n"

func TestEvidenceSubmissionAttribution(t *testing.T) {
	base := "# Brief\n\n## Evidence\n"
	for name, tc := range map[string]struct {
		content, old string
		refuse       bool
	}{
		"4114":                      {base + unattributed4114, base, true},
		"blocked":                   {base + attributedBlocked, base, false},
		"historical":                {base + unattributed4114 + "\n" + attributedBlocked, base + unattributed4114, false},
		"duplicate":                 {base + unattributed4114 + unattributed4114, base + unattributed4114, true},
		"missing_date":              {base + strings.ReplaceAll(attributedBlocked, "2026-10-23", ""), base, true},
		"invalid_date":              {base + strings.ReplaceAll(attributedBlocked, "2026-10-23", "2026-02-30"), base, true},
		"missing_runner":            {base + strings.ReplaceAll(attributedBlocked, "human:operator", ""), base, true},
		"placeholder":               {base + strings.ReplaceAll(attributedBlocked, "human:operator", "Unknown-runner"), base, true},
		"header_reinterpretation":   {base + strings.ReplaceAll(attributedBlocked, "| Runner |", "| Unknown |"), base + attributedBlocked, true},
		"when_header":               {base + strings.ReplaceAll(attributedBlocked, "Date", "When"), base, true},
		"cmd_outcome_header":        {base + "| # | Cmd | Outcome |\n|---|---|---|\n| 1 | check | blocked |\n", base, true},
		"observed_prefix_header":    {base + "| # | Observed (exit + output) |\n|---|---|\n| 1 | blocked |\n", base, true},
		"na_runner":                 {base + strings.ReplaceAll(attributedBlocked, "human:operator", "n/a"), base, true},
		"tbd_runner":                {base + strings.ReplaceAll(attributedBlocked, "human:operator", "TBD"), base, true},
		"commented_header":          {base + strings.ReplaceAll(unattributed4114, "Command", "Com<!-- note -->mand"), base, true},
		"actual_header":             {base + "| # | Check | Expected | Actual |\n|---|---|---|---|\n| 1 | check | pass | blocked |\n", base, true},
		"comments":                  {base + strings.ReplaceAll(unattributed4114, "| 9 |", "| <!--comment-->9 |"), base, true},
		"header_split_comment":      {base + "| # | Command <!--\n--> | Notes |\n|---|---|---|\n| 1 | statusgen --lint | pass |\n", base, true},
		"header_tail_comment":       {base + "| # | Command | Observed | <!--\n-->\n|---|---|---|\n| 1 | statusgen --lint | exit 0 |\n", base, true},
		"row_split_comment":         {base + "| # | Command | Observed | Date | Runner |\n|---|---|---|---|---|\n| 1 | statusgen <!--\n--> | exit 0 | 2026-10-23 | alice |\n", base, false},
		"en_dash_runner":            {base + strings.ReplaceAll(attributedBlocked, "human:operator", "\u2013"), base, true},
		"em_dash_runner":            {base + strings.ReplaceAll(attributedBlocked, "human:operator", "\u2014"), base, true},
		"zero_width_runner":         {base + strings.ReplaceAll(attributedBlocked, "human:operator", "\u200b"), base, true},
		"hangul_filler_runner":      {base + strings.ReplaceAll(attributedBlocked, "human:operator", "\u3164"), base, true},
		"emphasised_dash_runner":    {base + strings.ReplaceAll(attributedBlocked, "human:operator", "**\u2013**"), base, true},
		"date_not_beside_runner":    {base + "| # | Date | Command | Observed | Runner |\n|---|---|---|---|---|\n| 1 | 2026-10-23 | check | exit 0 | alice |\n", base, true},
		"emphasis_header":           {base + "| # | **Command** | Notes |\n|---|---|---|\n| 1 | check | pass |\n", base, true},
		"plural_header":             {base + "| # | Commands | Notes |\n|---|---|---|\n| 1 | check | pass |\n", base, true},
		"exit_code_header":          {base + "| # | Exit code | Notes |\n|---|---|---|\n| 1 | 0 | pass |\n", base, true},
		"zero_width_header":         {base + "| # | Com\u200bmand | Notes |\n|---|---|---|\n| 1 | check | pass |\n", base, true},
		"combining_mark_runner":     {base + strings.ReplaceAll(attributedBlocked, "human:operator", "\u034f"), base, true},
		"variation_selector_runner": {base + strings.ReplaceAll(attributedBlocked, "human:operator", "\ufe0f"), base, true},
		"unescaped_pipe_cell":       {base + strings.ReplaceAll(attributedBlocked, "| 9 | statusgen --lint |", "| 9 | statusgen | --lint |"), base, true},
		"extra_trailing_cell":       {base + strings.ReplaceAll(attributedBlocked, "human:operator |", "human:operator | extra |"), base, true},
		"escaped_pipe_cell":         {base + strings.ReplaceAll(attributedBlocked, "| 9 | statusgen --lint |", "| 9 | statusgen \\| --lint |"), base, false},
		"comment_over_heading":      {"# Brief\n\n## Context\n\nctx\n<!-- x\n## Evidence\n-->\n| # | Command | Observed |\n|---|---|---|\n| 1 | statusgen --lint | exit 0 |\n", "# Brief\n\n## Context\n\nctx\n<!-- x\n## Evidence\n-->\n", true},
		"comment_over_heading_real": {"# Brief\n\n## Context\n\nctx\n<!-- x\n## Evidence\n-->\n| # | Command | Observed |\n|---|---|---|\n| 1 | statusgen --lint | exit 0 |\n\n## Evidence\n", "# Brief\n\n## Context\n\nctx\n<!-- x\n## Evidence\n-->\n\n## Evidence\n", true},
		"short_row_trailing_column": {base + "| # | Command | Date | Runner | Base |\n|---|---|---|---|---|\n| 1 | cmd | 2026-10-23 | alice |\n", base, true},
		"quoted":                    {base + "```\n" + unattributed4114 + "```\n", base, false},
		"derivation":                {base + "| Rank | Value | Derivation |\n|---|---|---|\n| 1 | 5 | derived |\n", base, false},
	} {
		t.Run(name, func(t *testing.T) {
			err := checkAddedRowAttribution([]byte(tc.content), []byte(tc.old))
			if (err != nil) != tc.refuse {
				t.Fatalf("error=%v want refuse=%v", err, tc.refuse)
			}
		})
	}
}
func TestEvidenceAttributionWriteGate(t *testing.T) {
	for _, fragment := range []bool{false, true} {
		for _, dry := range []bool{false, true} {
			for _, valid := range []bool{false, true} {
				name := "direct"
				if fragment {
					name = "fragment"
				}
				if dry {
					name += "_dry"
				}
				if valid {
					name += "_attributed"
				}
				t.Run(name, func(t *testing.T) {
					f, _ := setupFake(t)
					path := "docs/streams/x/brief.md"
					base := "# Brief\n\n## Evidence\n"
					table := unattributed4114
					if valid {
						table = attributedBlocked
					}
					f.setFile(path, base)
					root := rootWithFile(t, path, base+table)
					args := []string{"example-org/tracker", "evidence", "--root", root, "--evidence-file", path}
					if fragment {
						args = []string{"example-org/tracker", "evidence", "--root", root, "--evidence-file", writeRepoFile(t, "rows.md", table), "--brief-path", path}
					}
					if dry {
						args = append(args, "--dry-run")
					}
					code := run(args)
					want := deskkit.ExitRefused
					if valid {
						want = deskkit.ExitOK
					}
					if code != want {
						t.Fatalf("exit=%d want=%d", code, want)
					}
					writes := 0
					if valid && !dry {
						writes = 1
					}
					if f.putCalls != writes {
						t.Fatalf("writes=%d want=%d", f.putCalls, writes)
					}
					if valid && !dry && !strings.Contains(f.putContent, "could-not-run: closure refused") {
						t.Fatal("blocked disposition lost")
					}
				})
			}
		}
	}
}

func TestAttributionRefusalLine(t *testing.T) {
	base := "# Brief\n\n## Evidence\n"
	content := base + "<!-- a\nmulti-line\ncomment -->\n" + unattributed4114
	err := checkAddedRowAttribution([]byte(content), []byte(base))
	if err == nil || !strings.Contains(err.Error(), "at line 9 ") {
		t.Fatalf("want refusal naming line 9, got %v", err)
	}
}

// TestAttributionRefusalLineAfterJoinedComment is a regression guard, not a fail-first test:
// the previous preflight kept comment newlines and so also named line 9. It pins the line
// mapping across a section-local strip (a comment on lines of its own, then a row joined
// across a comment), which a refusal that reported the stripped-line index would break.
func TestAttributionRefusalLineAfterJoinedComment(t *testing.T) {
	base := "# Brief\n\n## Evidence\n"
	content := base + "<!-- a\nb\nc -->\n| # | Command | Observed |\n|---|---|---|\n| 1 | a <!--\nx --> | b |\n"
	err := checkAddedRowAttribution([]byte(content), []byte(base))
	if err == nil || !strings.Contains(err.Error(), "at line 9 ") {
		t.Fatalf("want refusal naming line 9 (where the row starts), got %v", err)
	}
}

// An unescaped pipe in a cell shifts the columns; the refusal must name that cause rather than
// the generic missing-attribution text.
func TestAttributionPipeCauseNamed(t *testing.T) {
	base := "# Brief\n\n## Evidence\n"
	content := base + strings.ReplaceAll(attributedBlocked, "| 9 | statusgen --lint |", "| 9 | statusgen | --lint |")
	err := checkAddedRowAttribution([]byte(content), []byte(base))
	if err == nil || !strings.Contains(err.Error(), "unescaped |") || !strings.Contains(err.Error(), "6 cells under a header of 5") {
		t.Fatalf("want refusal naming the unescaped pipe and the cell counts, got %v", err)
	}
}

// The base is read the lint's way too: the section ends at the raw next heading, so a table the
// lint never read as Evidence (a comment opened in Evidence runs over ## Notes) is not historical.
func TestAttributionBaseSectionLocatedOnRawLines(t *testing.T) {
	table := "| # | Command | Observed |\n|---|---|---|\n| 1 | statusgen --lint | exit 0 |\n"
	base := "# Brief\n\n## Evidence\n<!-- x\n## Notes\n-->\n" + table
	content := "# Brief\n\n## Evidence\n\n" + table
	if err := checkAddedRowAttribution([]byte(content), []byte(base)); err == nil {
		t.Fatal("want refusal: the base's table sits outside its Evidence section, so the row is not historical")
	}
}
