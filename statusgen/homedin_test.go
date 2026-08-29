package main

import (
	"strings"
	"testing"
)

// TestHomedInShape pins the <owner>/<repo> shape validator: exactly one "/",
// both sides non-empty, no whitespace, and no allowlist coupling.
func TestHomedInShape(t *testing.T) {
	valid := []string{"medici-finance/assay", "acme/widgets", "a/b"}
	for _, v := range valid {
		if !validHomedInShape(v) {
			t.Errorf("validHomedInShape(%q) = false, want true", v)
		}
	}
	invalid := []string{"", "not-a-repo", "owner/", "/repo", "a/b/c", "own er/repo", "owner/re po", "owner\trepo"}
	for _, v := range invalid {
		if validHomedInShape(v) {
			t.Errorf("validHomedInShape(%q) = true, want false", v)
		}
	}
}

// TestHomedInParse covers the optional-known-key parse: a valid value lands on
// BriefFile.HomedIn, absence defaults to "", and a wrong TYPE is a parse error.
func TestHomedInParse(t *testing.T) {
	t.Run("valid value parses onto the BriefFile", func(t *testing.T) {
		bf, ok, err := parseBriefFile("testdata/briefschema/docs/streams/alpha/brief-90-valid-homed-in.md")
		if err != nil || !ok {
			t.Fatalf("parse failed: err=%v ok=%v", err, ok)
		}
		if bf.HomedIn != "medici-finance/assay" {
			t.Errorf("HomedIn = %q, want medici-finance/assay", bf.HomedIn)
		}
	})

	t.Run("absent field defaults to empty", func(t *testing.T) {
		bf, ok, err := parseBriefFile("testdata/briefschema/docs/streams/alpha/brief-01-valid.md")
		if err != nil || !ok {
			t.Fatalf("parse failed: err=%v ok=%v", err, ok)
		}
		if bf.HomedIn != "" {
			t.Errorf("HomedIn = %q, want empty for a brief without the field", bf.HomedIn)
		}
	})

	t.Run("a non-string value is a parse error", func(t *testing.T) {
		fm := "---\nbrief: alpha/01\ntitle: t\nwave: 0\ndepends: []\nunblocks: []\n" +
			"effort: S\ngate: model\nhomed-in: [a, b]\n" +
			"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
			"issues: []\nschema: brief-v1\nauthored: 2026-08-28 by x\nsources: [\"s\"]\n---\n\n# t\n"
		p := writeTemp(t, t.TempDir(), "brief-01-x.md", fm)
		if _, ok, err := parseBriefFile(p); ok || err == nil || !strings.Contains(err.Error(), "homed-in must be a string") {
			t.Fatalf("want a homed-in-must-be-a-string parse error; ok=%v err=%v", ok, err)
		}
	})
}

// TestHomedInSchemaChecks covers checkBriefFiles: a valid value is clean and
// worms onto the Brief row; a malformed value is a hard PROBLEM echoing the value
// and is left OFF the row so a typo cannot silently exclude a brief.
func TestHomedInSchemaChecks(t *testing.T) {
	problems, _ := briefSchemaChecks(t)

	t.Run("valid homed-in is clean", func(t *testing.T) {
		if hasProblem(problems, "brief-90-valid-homed-in.md") {
			t.Errorf("valid homed-in must be clean; got:\n%s", strings.Join(problems, "\n"))
		}
	})

	t.Run("malformed homed-in is a PROBLEM echoing the value", func(t *testing.T) {
		if !hasProblem(problems, "brief-91-bad-homed-in.md", "invalid homed-in", "not-a-repo", "<owner>/<repo>") {
			t.Errorf("want an invalid-homed-in PROBLEM echoing the value; got:\n%s", strings.Join(problems, "\n"))
		}
	})
}

// TestHomedInEligibility is the load-bearing property: a valid homed-in brief is
// held OUT of Next-up (all statuses), keeps its tracking row, and is NAMED with
// its target on the view — while a sibling without the field stays a pick.
func TestHomedInEligibility(t *testing.T) {
	homed := Brief{Num: "01", Title: "re-homed", Wave: 0, Status: "in-progress", HomedIn: "acme/widgets"}
	local := Brief{Num: "02", Title: "local", Wave: 0, Status: "in-progress"}
	s := mkStream("alpha", "active", "P1", homed, local)
	s.Track = "platform"

	nu := nextUp([]*Stream{s}, ClaimView{}, nil)

	// The homed-in brief must not be a pick; the local sibling must be.
	var sawHomed, sawLocal bool
	for _, p := range nu.Picks {
		switch p.Brief.Num {
		case "01":
			sawHomed = true
		case "02":
			sawLocal = true
		}
	}
	if sawHomed {
		t.Errorf("a homed-in brief must be excluded from Next-up picks; picks: %+v", nu.Picks)
	}
	if !sawLocal {
		t.Errorf("a sibling without homed-in must still be a pick; picks: %+v", nu.Picks)
	}

	// Named with its target on the view — never silently dropped.
	if got := nu.HomedElsewhere["alpha/01"]; got != "acme/widgets" {
		t.Errorf("HomedElsewhere[alpha/01] = %q, want acme/widgets; map: %v", got, nu.HomedElsewhere)
	}

	// Tracking row preserved: still present in the stream's brief set.
	found := false
	for _, b := range s.Briefs {
		if b.Num == "01" {
			found = true
		}
	}
	if !found {
		t.Errorf("the homed-in brief's tracking row must be preserved in s.Briefs")
	}
}

// TestHomedInInert is the additive-inert invariant at the eligibility layer: a
// board where no brief carries homed-in populates nothing new — HomedElsewhere is
// empty and the plain briefs remain eligible exactly as before.
func TestHomedInInert(t *testing.T) {
	a := Brief{Num: "01", Title: "a", Wave: 0, Status: "in-progress"}
	b := Brief{Num: "02", Title: "b", Wave: 0, Status: "in-progress"}
	s := mkStream("alpha", "active", "P1", a, b)
	s.Track = "platform"

	nu := nextUp([]*Stream{s}, ClaimView{}, nil)
	if len(nu.HomedElsewhere) != 0 {
		t.Errorf("no brief carries homed-in, so HomedElsewhere must be empty; got %v", nu.HomedElsewhere)
	}
	if len(nu.Picks) != 2 {
		t.Errorf("both plain briefs must remain eligible; got %d picks", len(nu.Picks))
	}
}

// TestHomedInMarkerAndBanner covers the two render sites: the [homed→...] marker
// on a tracking row (here the Awaiting board, where a homed-in brief can still
// show at implemented) and the "HOMED IN ANOTHER REPO" board line naming id +
// target.
func TestHomedInMarkerAndBanner(t *testing.T) {
	// Marker on an implemented homed-in brief (Awaiting board).
	impl := Brief{Num: "07", Title: "re-homed impl", Wave: 0, Status: "implemented", HomedIn: "medici-finance/assay"}
	sm := mkStream("alpha", "active", "P1", impl)
	sm.Track = "platform"
	num := nextUp([]*Stream{sm}, ClaimView{}, nil)
	out := emit([]*Stream{sm}, nil, num, nil, nil, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(out, "[homed→medici-finance/assay]") {
		t.Errorf("Awaiting output missing [homed→...] marker:\n%s", out)
	}

	// Board line naming a held todo homed-in brief + its target.
	todo := Brief{Num: "03", Title: "re-homed todo", Wave: 0, Status: "todo", Schema: "brief-v1", HomedIn: "acme/widgets"}
	sb := mkStream("beta", "active", "P1", todo)
	sb.Track = "platform"
	nub := nextUp([]*Stream{sb}, ClaimView{}, nil)
	outb := emit([]*Stream{sb}, nil, nub, nil, nil, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(outb, "HOMED IN ANOTHER REPO") || !strings.Contains(outb, "beta/03 → acme/widgets") {
		t.Errorf("Next-up output missing the homed-in board line naming id + target:\n%s", outb)
	}

	// A board with no homed-in brief renders neither the marker nor the line.
	plain := Brief{Num: "01", Title: "local", Wave: 0, Status: "implemented"}
	sp := mkStream("gamma", "active", "P1", plain)
	sp.Track = "platform"
	nup := nextUp([]*Stream{sp}, ClaimView{}, nil)
	outp := emit([]*Stream{sp}, nil, nup, nil, nil, IntakeAlarmResult{}, nil, "")
	if strings.Contains(outp, "[homed→") || strings.Contains(outp, "HOMED IN ANOTHER REPO") {
		t.Errorf("a board with no homed-in brief must render neither marker nor line:\n%s", outp)
	}
}

// TestHomedInSupersedesPhantom covers the board-honesty integration: a valid
// homed-in on a row whose body would trip a work-moved phantom class SUPPRESSES
// that heuristic NOTICE (the explicit field is the successor), while a row with
// the same banner but no homed-in still surfaces.
func TestHomedInSupersedesPhantom(t *testing.T) {
	t.Run("valid homed-in suppresses the out-of-repo guess", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "gtm", "Active stream.\n",
			"08", "The deliverable lands in another repo.\n")
		s.Briefs = []Brief{{Num: "08", Status: "todo", HomedIn: "medici-finance/assay"}}
		if got := boardHonestyNotices([]*Stream{s}, nil, nil); len(got) != 0 {
			t.Fatalf("a valid homed-in must suppress the work-moved phantom NOTICE; got %v", got)
		}
	})

	t.Run("same banner without homed-in still surfaces", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "gtm", "Active stream.\n",
			"08", "The deliverable lands in another repo.\n")
		s.Briefs = []Brief{{Num: "08", Status: "todo"}}
		got := boardHonestyNotices([]*Stream{s}, nil, nil)
		if len(got) != 1 || !strings.Contains(got[0], phantomOutOfRepo) {
			t.Fatalf("without homed-in the phantom must still surface; got %v", got)
		}
	})

	t.Run("homed-in does NOT suppress the git-derived already-merged class", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "oss", "Active stream.\n", "08", "Plain body.\n")
		s.Briefs = []Brief{{Num: "08", Status: "todo", HomedIn: "medici-finance/assay"}}
		merged := []mergedPR{{Number: 1, Subject: "Merge pull request #1 from x/brief/oss-08", Briefs: []string{"oss/08"}}}
		got := boardHonestyNotices([]*Stream{s}, merged, nil)
		if len(got) != 1 || !strings.Contains(got[0], phantomMergedUnflipped) {
			t.Fatalf("homed-in must not mask a merged-unflipped row; got %v", got)
		}
	})
}
