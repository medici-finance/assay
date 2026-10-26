package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

func TestSharedFrontmatter(t *testing.T) {
	fm := parseFrontmatter("---\nrisk: {regulatory: \"yes\"}\ngate: human\n---\n")
	if !fm.Risk.Regulatory || fm.ParseError != "" {
		t.Fatal(fm)
	}
	bad := parseFrontmatter("---\nrisk: [yes]\n---\n")
	if bad.ParseError == "" || bad.Gate != "human" || !bad.Risk.Regulatory {
		t.Fatal("unreadable input was treated as safe", bad)
	}
	for _, misplaced := range []string{"risks:\n  regulatory: yes\n", "meta:\n  customer: yes\n", "\u200bcustomer: yes\n", "\ufeff--- {customer: yes}\n", "\u2800customer: yes\n", "\u0301customer: yes\n", "\u00a0--- {customer: yes}\n", "cu\u017ftomer: yes\n", "notes: |\n  customer: yes\n"} {
		fm := parseFrontmatter("---\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" + misplaced + "---\n")
		if fm.ParseError == "" || fm.Gate != "human" || !fm.Risk.Regulatory || !fm.Risk.Customer {
			t.Fatalf("misplaced risk answer %q read as safe: %+v", misplaced, fm)
		}
	}
	for _, second := range []string{"...\nregulatory: yes\n", "--- {customer: yes}\n", "...\ngate: human\n"} {
		fm := parseFrontmatter("---\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" + second + "---\n")
		if fm.ParseError == "" || fm.Gate != "human" || !fm.Risk.Regulatory || !fm.Risk.Customer {
			t.Errorf("second YAML document %q read as safe: %+v", second, fm)
		}
	}
	for _, hidden := range continuationLines {
		fm := parseFrontmatter("---\n" + hidden + "\n---\n")
		if fm.ParseError == "" || fm.Gate != "human" || fm.ExecTier != "strong" {
			t.Errorf("continuation line %q read as %+v", hidden, fm)
		}
	}
}

// TestSharedFrontmatterQueue runs the shared parser through the board read and
// classification: an unquoted blocked-until date defers, and a risk answer
// outside the risk mapping keeps the brief away from model dispatch.
func TestSharedFrontmatterQueue(t *testing.T) {
	root := t.TempDir()
	table := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | A | 0 | S | implemented | — | — |\n" +
		"| 02 | B | 0 | S | implemented | — | — |\n" +
		"| 03 | C | 0 | S | implemented | — | — |\n"
	writeFixtureStream(t, root, "fixture", table, map[string]string{
		"01": briefBodyMarkers("blocked-until: 2026-11-01\n"),
		"02": briefBodyMarkers("meta:\n  customer: yes\n"),
		"03": briefBodyMarkers("risks:\n  regulatory: yes\n"),
	})
	v := &VerifyLoop{Root: root, TargetSHA: "abc"}
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	byID := map[string]loopengine.Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	if got := byID["fixture/01"].Payload["blocked_until"]; got != "2026-11-01" {
		t.Fatalf("unquoted blocked-until date not carried: %q (payload %v)", got, byID["fixture/01"].Payload)
	}
	for id, notWant := range map[string]disposition{"fixture/01": dispDispatch, "fixture/02": dispDispatch, "fixture/03": dispDispatch} {
		it, ok := byID[id]
		if !ok {
			t.Fatalf("%s missing from the queue", id)
		}
		tier, _ := (&VerifyLoop{}).TierPolicy(it)
		disp, why := classifyItem(it, tier)
		if disp == notWant {
			t.Fatalf("%s classified %v (%s); it must not dispatch", id, disp, why)
		}
		if id == "fixture/01" && disp != dispDeferred {
			t.Fatalf("%s classified %v (%s); want deferred", id, disp, why)
		}
	}
}

// continuationLines hide a column-0 policy line inside a multi-line value
// opened on an earlier line: the shared parser reads the line as text in that
// value, while a first-match line reader reads it as the key.
var continuationLines = []string{
	"meta: {a: 1,\ngate: human\n}\ngate: model",
	"meta: {a:1,\ngate: human\n}",
	"tags: [a,\ngate: human\n]\ngate: model",
	"tags: [a,\nexec-tier: strong]\nexec-tier: any",
	"title: \"first\ngate: human\n  end\"\ngate: model",
	"title: 'first\nexec-tier: strong\n  end'",
	"meta: {a: 1,\ndecision-issue: 12\n}",
}

// verifyLoopScalar is this loop's frontmatter reader before the shared parser:
// the first column-0 `key:` line, the gap "\s*", the inline comment kept,
// then space and quotes trimmed and space trimmed again.
func verifyLoopScalar(block, key string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:\s*(.*)$`)
	if m := re.FindStringSubmatch(block); m != nil {
		return strings.TrimSpace(strings.Trim(strings.TrimSpace(m[1]), `"'`))
	}
	return ""
}

// TestCommentedMarkersReadAsBefore pins that the queue markers read exactly as
// this loop's earlier reader read them, inline comment included, so a marker
// that held a brief before still holds it: "in-repair: no # x" stays in
// repair and `blocked-until: "" # x` stays deferred, while the plain values
// still dispatch.
func TestCommentedMarkersReadAsBefore(t *testing.T) {
	cases := []struct {
		marker string
		want   disposition
	}{
		{"in-repair: no # x", dispInRepair},
		{`blocked-until: "" # x`, dispDeferred},
		{"in-repair: no", dispDispatch},
		{`blocked-until: ""`, dispDispatch},
		{"verify-lane: online", dispAwaitingOnlineLane},
	}
	root := t.TempDir()
	table := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n"
	briefs := map[string]string{}
	for i, c := range cases {
		id := fmt.Sprintf("%02d", i+1)
		table += "| " + id + " | X | 0 | S | implemented | — | — |\n"
		briefs[id] = briefBodyMarkers(c.marker + "\n")
		fm := parseFrontmatter(briefs[id])
		block := frontmatterBlock(briefs[id])
		for key, got := range map[string]string{"in-repair": fm.InRepair, "blocked-until": fm.BlockedUntil, "verify-lane": fm.VerifyLane, "implementer": fm.Implementer} {
			if want := verifyLoopScalar(block, key); fm.ParseError != "" || got != want {
				t.Errorf("%q: %s read %q (parse error %q); the earlier reader read %q", c.marker, key, got, fm.ParseError, want)
			}
		}
	}
	writeFixtureStream(t, root, "fixture", table, briefs)
	items, err := (&VerifyLoop{Root: root, TargetSHA: "abc"}).SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	byID := map[string]loopengine.Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	for i, c := range cases {
		it, ok := byID[fmt.Sprintf("fixture/%02d", i+1)]
		if !ok {
			t.Fatalf("%q: brief missing from the queue", c.marker)
		}
		tier, _ := (&VerifyLoop{}).TierPolicy(it)
		if disp, why := classifyItem(it, tier); disp != c.want {
			t.Errorf("%q: classified %v (%s); want %v", c.marker, disp, why, c.want)
		}
	}
}
