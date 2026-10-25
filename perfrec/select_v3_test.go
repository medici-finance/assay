package perfrec

import "testing"

// TestSelectV3IsReduceSet pins SelectV3 as the set ReduceV3 aggregates: a reader that
// buckets the kept records counts exactly what the reducer counts, with the same
// duplicate, conflict and invalid exclusions.
// regression: #3959
func TestSelectV3IsReduceSet(t *testing.T) {
	t.Setenv(EnvVar, "1")
	rec := func() Record {
		r := v3Record(v3Context("tool"))
		r.Install, _ = NewID()
		return r
	}
	a, b, c := rec(), rec(), rec()
	moved := b
	moved.Verb = "other"
	bad := rec()
	bad.Install = "not-an-install-id"
	recs := []Record{a, a, b, moved, c, bad}
	kept, sel, err := SelectV3(recs, ProfileTool, V3Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 2 || sel != (V3Selection{Duplicates: 1, Conflicts: 1, Invalid: 1}) {
		t.Fatalf("kept %d, selection %+v; want 2 kept, 1 duplicate, 1 conflict, 1 invalid", len(kept), sel)
	}
	s, err := ReduceV3(recs, ProfileTool, "tool", V3Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Records != len(kept) || s.Duplicates != sel.Duplicates || s.Conflicts != sel.Conflicts || s.Invalid != sel.Invalid {
		t.Fatalf("reduce %d/%d/%d/%d disagrees with select %d/%+v", s.Records, s.Duplicates, s.Conflicts, s.Invalid, len(kept), sel)
	}
	if V3Filter.Used(V3Filter{}) || !(V3Filter{SessionID: "x"}).Used() {
		t.Fatal("Used misreads the filter")
	}
	if _, _, err := SelectV3(recs, ProfileTool, V3Filter{SessionID: "x"}); err == nil {
		t.Fatal("filter accepted without workflow permission")
	}
}
