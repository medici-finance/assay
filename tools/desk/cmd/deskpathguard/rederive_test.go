package main

import (
	"strings"
	"testing"
)

const baseTableMD = `---
brief: assay:at:x:09
---

# Brief 09

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | go test ./foo/... | pass |
| 2 | check | go test ./bar/... | pass |

## Evidence
`

// headTableMD is baseTableMD with row 1's command edited (would be silently trusted if the
// re-derivation ran the HEAD table) and a brand-new row 3 added.
const headTableMD = `---
brief: assay:at:x:09
---

# Brief 09

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | true | pass |
| 2 | check | go test ./bar/... | pass |
| 3 | check | true | pass |

## Evidence
`

// TestRederiveTableRow7 is Verify row 7: a post-change table with one extra row reports
// that row author-added, and every OTHER row runs from the merge-base table's own command —
// never the (possibly edited) head command. FAIL-FIRST perturbation: the base table (no
// added row, no edited command) re-derives to zero author-added rows and the ORIGINAL
// command for row 1.
func TestRederiveTableRow7(t *testing.T) {
	rows := RederiveTable(baseTableMD, headTableMD)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows at HEAD, got %d: %+v", len(rows), rows)
	}
	byNum := map[string]rederivedRow{}
	for _, r := range rows {
		byNum[r.Num] = r
	}
	if !byNum["3"].AuthorAdded {
		t.Fatalf("row 3 exists only at HEAD and must be reported author-added: %+v", byNum["3"])
	}
	if byNum["1"].AuthorAdded {
		t.Fatalf("row 1 exists at BASE and must NOT be author-added: %+v", byNum["1"])
	}
	// The command that would RUN for row 1 is the BASE command, not the (edited) head one.
	if byNum["1"].Base.Command != "go test ./foo/..." {
		t.Fatalf("row 1 must carry the BASE command, not the head edit: got %q", byNum["1"].Base.Command)
	}
	if byNum["2"].AuthorAdded {
		t.Fatalf("row 2 is unchanged at both ends and must not be author-added: %+v", byNum["2"])
	}

	// perturb: re-derive base against itself — no row is new, no command differs.
	unperturbed := RederiveTable(baseTableMD, baseTableMD)
	for _, r := range unperturbed {
		if r.AuthorAdded {
			t.Fatalf("perturb (base vs base): no row should be author-added, got %+v", r)
		}
	}
}

func TestRederiveTableBriefAbsentAtBase(t *testing.T) {
	// A brand-new brief (no prior version at the merge-base): every row is author-added.
	rows := RederiveTable("", headTableMD)
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	for _, r := range rows {
		if !r.AuthorAdded {
			t.Fatalf("every row of a brief absent at base must be author-added: %+v", r)
		}
	}
}

func TestExtractVerifySectionStopsAtNextHeading(t *testing.T) {
	section := extractVerifySection(headTableMD)
	if section == "" {
		t.Fatal("expected a non-empty ## Verify section")
	}
	if strings.Contains(section, "## Evidence") {
		t.Fatalf("the extracted section must stop before ## Evidence: %q", section)
	}
	if !strings.Contains(section, "| 3 | check | true | pass |") {
		t.Fatalf("the extracted section must include row 3: %q", section)
	}
}
