package cellop

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
)

// drawSequence makes drawID return ids in order, then fail the test if it is called again. It
// counts the draws and restores the real drawID when the test ends.
func drawSequence(t *testing.T, ids ...string) *int {
	t.Helper()
	real, n := drawID, 0
	drawID = func() (string, error) {
		if n >= len(ids) {
			t.Fatalf("drawID called %d times, want at most %d", n+1, len(ids))
		}
		n++
		return ids[n-1], nil
	}
	t.Cleanup(func() { drawID = real })
	return &n
}

// saveAs builds the one-action drain plan of testPlan under id.
func saveAs(t *testing.T, s Store, id string) Plan {
	t.Helper()
	p, err := NewPlan(id, "drain", []string{"example"}, []Action{{
		ID: "drain-example", Cell: "example", Summary: "drain example",
		Preconditions: []Precondition{{Key: "example/state", Value: "running"}},
	}}, nil, t0)
	if err != nil {
		t.Fatal(err)
	}
	save(t, s, p)
	return p
}

func build(cell string) func(id string) (Plan, error) {
	return func(id string) (Plan, error) {
		return NewPlan(id, "drain", []string{cell}, []Action{{
			ID: "drain-" + cell, Cell: cell, Summary: "drain " + cell,
			Preconditions: []Precondition{{Key: cell + "/state", Value: "running"}},
		}}, nil, t0)
	}
}

// Every word is 3 to 8 lowercase letters, in one list only, and the lists are big enough that
// a store of a few hundred plans rarely redraws. The substring check is a cheap backstop for a
// careless addition; reading the lists in review is the real control.
// regression: #6266
func TestPlanIDWordLists(t *testing.T) {
	word := regexp.MustCompile(`^[a-z]{3,8}$`)
	seen := map[string]string{}
	for list, words := range map[string][]string{"adjective": idAdjectives[:], "noun": idNouns[:]} {
		if len(words) < 150 {
			t.Errorf("%s list has %d words, want at least 150", list, len(words))
		}
		for _, w := range words {
			if !word.MatchString(w) {
				t.Errorf("%s %q is not 3 to 8 lowercase letters", list, w)
			}
			if prev, dup := seen[w]; dup {
				t.Errorf("%q is in the %s list and the %s list", w, prev, list)
			}
			seen[w] = list
			for _, bad := range []string{"ass", "tit", "cock", "cum", "fag", "nig", "sex", "piss", "shit", "rape", "dick", "cunt", "slut", "porn", "anal", "anus", "butt", "boob", "homo", "kill", "dead", "die"} {
				if strings.Contains(w, bad) {
					t.Errorf("%s %q contains %q", list, w, bad)
				}
			}
		}
	}
	for range 200 {
		id, err := drawID()
		if err != nil {
			t.Fatal(err)
		}
		if !idRE.MatchString(id) || checkID(id) != nil {
			t.Fatalf("drawn id %q is not a valid plan id", id)
		}
		a, n, _ := strings.Cut(id, "-")
		if seen[a] != "adjective" || seen[n] != "noun" {
			t.Fatalf("drawn id %q is not an adjective then a noun from the lists", id)
		}
	}
}

// A drawn id that a stored plan or a consumed marker already holds is drawn again; the
// existing plan is untouched.
// regression: #6266
func TestSaveNewRedrawsTakenIDs(t *testing.T) {
	s := StoreFor(t.TempDir())
	stored := saveAs(t, s, "amber-falcon")
	gone := saveAs(t, s, "brisk-otter")
	if err := s.Consume(gone.ID, t0); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(s.Path(gone.ID)); err != nil { // consumed, its plan file since removed
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.Path(stored.ID))
	draws := drawSequence(t, "amber-falcon", "brisk-otter", "calm-heron")
	p, err := s.SaveNew(build("example"))
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "calm-heron" || *draws != 3 {
		t.Fatalf("got id %q after %d draws, want calm-heron after 3", p.ID, *draws)
	}
	if got, err := s.Load("calm-heron"); err != nil || got.Digest != p.Digest {
		t.Fatalf("the new plan is not stored under its id: %v", err)
	}
	if after, _ := os.ReadFile(s.Path(stored.ID)); string(after) != string(before) {
		t.Fatal("a redraw changed the plan already stored under the taken id")
	}
}

// A save that loses a race for its id (the file appears between the check and the save) draws
// again; when every draw is taken SaveNew gives up after maxDraws draws, saving nothing.
// regression: #6266
func TestSaveNewRaceAndExhaustion(t *testing.T) {
	s := StoreFor(t.TempDir())
	drawSequence(t, "quick-lynx", "steady-wren")
	raced := false
	p, err := s.SaveNew(func(id string) (Plan, error) {
		if !raced {
			raced = true
			saveAs(t, s, id) // another writer takes the id first
		}
		return build("example")(id)
	})
	if err != nil || p.ID != "steady-wren" {
		t.Fatalf("got %q, %v; want steady-wren after a lost race", p.ID, err)
	}

	full := StoreFor(t.TempDir())
	saveAs(t, full, "amber-falcon")
	ids := make([]string, maxDraws)
	for i := range ids {
		ids[i] = "amber-falcon"
	}
	draws := drawSequence(t, ids...)
	if _, err := full.SaveNew(build("example")); err == nil || !strings.Contains(err.Error(), "no free plan id") {
		t.Fatalf("a store with no free id: %v", err)
	}
	if *draws != maxDraws {
		t.Fatalf("%d draws, want %d", *draws, maxDraws)
	}
	entries, _ := os.ReadDir(full.Dir)
	if len(entries) != 1 {
		t.Fatalf("an exhausted SaveNew left %d entries, want the one plan", len(entries))
	}
}

// A plan saved under a drawn two-word id applies by that id, with the typed confirm, once.
// regression: #6266
func TestApplyByTwoWordID(t *testing.T) {
	s := StoreFor(t.TempDir())
	w, c := newWorld(), &clock{now: t0}
	w.set("example/state", "running")
	p, err := s.SaveNew(build("example"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[a-z]{3,8}-[a-z]{3,8}$`).MatchString(p.ID) {
		t.Fatalf("new plan id %q is not two words", p.ID)
	}
	sess := session(s, w, c, "example", "drain")
	sess.Confirm = func(Plan, []Plan) (string, error) { return "00000000", nil }
	expect(t, sess.Apply(p.ID), Refused, ReasonConfirmMismatch)
	sess = session(s, w, c, "example", "drain")
	r := sess.Apply(p.ID)
	expect(t, r, OK, "")
	if r.PlanID == nil || *r.PlanID != p.ID || w.runs("drain-example") != 1 {
		t.Fatalf("plan_id %v, ran %d times", r.PlanID, w.runs("drain-example"))
	}
	if _, err := os.Lstat(s.MarkerPath(p.ID)); err != nil {
		t.Fatalf("no consumed marker named for the two-word id: %v", err)
	}
	expect(t, sess.Apply(p.ID), Refused, ReasonAlreadyConsumed)
}

// A plan stored under a 24-hex id before two-word ids still loads and applies by that id.
// regression: #6266
func TestApplyByLegacyHexID(t *testing.T) {
	s := StoreFor(t.TempDir())
	w, c := newWorld(), &clock{now: t0}
	w.set("example/state", "running")
	p := saveAs(t, s, "4f1c2a9e7b3d5c8e0a6f2b1d")
	r := session(s, w, c, "example", "drain").Apply(p.ID)
	expect(t, r, OK, "")
	if w.runs("drain-example") != 1 || !consumed(t, s, p.ID) {
		t.Fatal("the legacy plan did not run once and consume")
	}
}

// The Go pattern and the schema's planId agree: both id forms pass, and nothing that could leave
// the store directory or that the draw never makes does.
// regression: #6266
func TestPlanIDPatternMatchesSchema(t *testing.T) {
	sv := loadSchema(t)
	s := StoreFor(t.TempDir())
	for id, want := range map[string]bool{
		"amber-falcon": true, "4f1c2a9e7b3d5c8e0a6f2b1d": true,
		"../x": false, "amber-falcon-x": false, "Amber-Falcon": false, "amber_falcon": false,
		"am-falcon": false, "amber-": false, "amberfalcon": false, "4F1C2A9E7B3D5C8E0A6F2B1D": false,
		"4f1c2a9e7b3d5c8e0a6f2b1": false, "amber-falcon.json": false, "": false,
	} {
		if got := idRE.MatchString(id); got != want {
			t.Errorf("idRE(%q) = %v, want %v", id, got, want)
		}
		r := NewResult("drain", "example", t0)
		r.PlanID = &id
		raw, _ := json.Marshal(r)
		doc, _ := decodeDoc(raw)
		if got := len(sv.validate(doc)) == 0; got != want {
			t.Errorf("schema plan_id %q valid = %v, want %v", id, got, want)
		}
		if _, err := s.Load(id); !want && !errors.Is(err, ErrInvalid) {
			t.Errorf("Load(%q) = %v, want ErrInvalid", id, err)
		}
	}
}
