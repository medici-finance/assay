package main

import (
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
