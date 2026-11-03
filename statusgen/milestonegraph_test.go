package main

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// The milestone-support/05 fixture is the spec's mixed example: build m-03@1
// is contributed by build briefs 11 and 12 (at revision 2) and requires
// stream-y m-01@1 and stream-z m-02@1 for its own completion. Only brief 12
// declares stream-y m-01@1 as a start requirement; stream-z m-02@1 stays a
// final-acceptance prerequisite of the milestone and gates no brief's start.

const (
	ms05M3  = "assay:assay:build:m-03@1"
	ms05Y   = "assay:assay:stream-y:m-01@1"
	ms05Z   = "assay:assay:stream-z:m-02@1"
	ms05B11 = "assay:assay:build:b-11"
	ms05B12 = "assay:assay:build:b-12"
)

// ms05Tree copies the fixture into a fresh root and applies overlays (path
// relative to docs/streams -> edit).
func ms05Tree(t *testing.T, edits map[string]func(string) string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/milestone-support/05/tree")); err != nil {
		t.Fatal(err)
	}
	for name, edit := range edits {
		p := filepath.Join(root, "docs", "streams", filepath.FromSlash(name))
		raw, err := os.ReadFile(p)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(edit(string(raw))), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func ms05Load(t *testing.T, roots ...string) []*Stream {
	t.Helper()
	var all []*Stream
	for _, root := range roots {
		streams, _, err := loadStreams(root)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, streams...)
	}
	return all
}

// ms05Graph rebuilds the combined graph exactly as the post-scan adapter does,
// from the loaded streams and their brief files, so a test can read milestone
// completion as well as the start gates the scan wrote back.
func ms05Graph(t *testing.T, streams []*Stream) evaluation.MilestoneGraph {
	t.Helper()
	snaps := map[string]briefSnapshot{}
	var pending []milestoneStartPending
	for _, s := range streams {
		for i := range s.Briefs {
			row := &s.Briefs[i]
			files, _ := filepath.Glob(filepath.Join(s.Dir, "brief-"+row.Num+"-*.md"))
			if len(files) != 1 {
				continue
			}
			bf, ok, err := parseBriefFile(files[0])
			if err != nil || !ok {
				t.Fatalf("parse %s: ok=%v err=%v", files[0], ok, err)
			}
			if snaps[s.Root] == nil {
				snaps[s.Root] = briefSnapshot{}
			}
			snaps[s.Root][s.Name+"/"+row.Num] = briefSnapshotEntry{Version: bf.Version, ID: bf.ID}
			row.Depends = bf.Depends
			if len(bf.RequiresMilestones) > 0 {
				pending = append(pending, milestoneStartPending{path: files[0], stream: s, row: row, requires: bf.RequiresMilestones})
			}
		}
	}
	in, _, _ := milestoneGraphInput(streams, pending, snaps, func(root string) *graphRepos {
		reg, _, _ := loadGraphRepos(root)
		return reg
	})
	return evaluation.BuildMilestoneGraph(in)
}

func ms05SetStatus(streams []*Stream, status string, ids ...string) {
	for _, s := range streams {
		for i := range s.Briefs {
			if slices.Contains(ids, s.Name+"/"+s.Briefs[i].Num) {
				s.Briefs[i].Status = status
			}
		}
	}
}

func ms05HoldOn(e Eligibility, sub string) bool {
	for _, h := range e.Holds {
		if strings.Contains(h.Ref, sub) {
			return true
		}
	}
	return false
}

func ms05Has(list []string, subs ...string) bool { return hasProblem(list, subs...) }

// regression: milestone-support/05
func TestMilestoneMixedGraph(t *testing.T) {
	t.Run("11 starts independently, 12 needs Y, Z stays final-only", func(t *testing.T) {
		advertiseMilestones(t)
		streams := ms05Load(t, ms05Tree(t, nil))
		problems, notices := checkBriefFiles(streams, streams)
		if len(problems) != 0 {
			t.Fatalf("the mixed fixture must lint clean; problems:\n%s", strings.Join(problems, "\n"))
		}
		elig := eligibilityForStreams(streams)
		if e := elig["build/11"]; e.Verdict != VerdictEligible || len(e.Holds) != 0 {
			t.Errorf("brief 11 contributes to m-03 but declares no start requirement: it must start independently, never inherit m-03's prerequisites; got %+v", e)
		}
		e12 := elig["build/12"]
		if e12.Verdict != VerdictHeld || !hasHold(e12, "stream-y:m-01@1", StateUnsatisfied) {
			t.Errorf("brief 12 must be HELD unsatisfied on its own start requirement stream-y m-01@1 (known remaining work); got %+v", e12)
		}
		for _, h := range e12.Holds {
			if h.Ref == "stream-y:m-01@1" && !strings.Contains(h.Why, "contributor assay:assay:stream-y:b-01 is todo") {
				t.Errorf("the hold must name the known blocker; got %q", h.Why)
			}
		}
		for id, e := range elig {
			if ms05HoldOn(e, "stream-z") || ms05HoldOn(e, "m-02") {
				t.Errorf("stream-z m-02@1 is a final-acceptance prerequisite of m-03 only; %s must not be held on it: %+v", id, e)
			}
		}
		if !ms05Has(notices, "brief-12-gated.md", "requires-milestones stream-y:m-01@1", "held unsatisfied", "milestone blocked") {
			t.Errorf("brief 12's start requirement must be announced as held unsatisfied; notices:\n%s", strings.Join(notices, "\n"))
		}
		if ms05Has(notices, "brief-11-independent.md", "requires-milestones") {
			t.Errorf("brief 11 has no start requirement to announce; notices:\n%s", strings.Join(notices, "\n"))
		}

		g := ms05Graph(t, streams)
		m3 := g.Milestones[ms05M3]
		if m3.Completion != evaluation.GraphBlocked {
			t.Fatalf("m-03 has known remaining work and must be blocked; got %+v", m3)
		}
		if strings.Join(m3.Prerequisites, ",") != ms05Y+","+ms05Z {
			t.Errorf("m-03 prerequisites = %v, want both Y and Z (a deleted prerequisite is a silently weaker contract)", m3.Prerequisites)
		}
		for _, want := range []string{ms05Y + ": contributor assay:assay:stream-y:b-01 is todo", ms05Z + ": contributor assay:assay:stream-z:b-01 is todo", "contributor " + ms05B11 + " is todo", "contributor " + ms05B12 + " is todo"} {
			if !slices.Contains(m3.Blockers, want) {
				t.Errorf("m-03 blockers %q lack %q", m3.Blockers, want)
			}
		}
		if s := g.Briefs[ms05B11]; len(s.Gates) != 0 || !slices.Equal(s.Membership, []string{ms05M3}) {
			t.Errorf("brief 11: membership is derived (m-03) and adds no start gate; got %+v", s)
		}
		s12 := g.Briefs[ms05B12]
		if s12.ID != "00000000-0000-4000-8000-000000000512" || s12.Root == "" {
			t.Errorf("the source port must carry the brief's root and uuid through unchanged; got root=%q id=%q", s12.Root, s12.ID)
		}
		if len(s12.Gates) != 1 || s12.Gates[0].Milestone != ms05Y || s12.Gates[0].State != evaluation.GraphBlocked {
			t.Errorf("brief 12 must carry exactly its own start gate on Y, blocked; got %+v", s12.Gates)
		}
		for key, s := range g.Briefs {
			for _, gate := range s.Gates {
				if gate.Milestone == ms05Z || gate.Milestone == ms05M3 {
					t.Errorf("%s gained a start gate on %s it never declared", key, gate.Milestone)
				}
			}
		}
	})

	t.Run("a satisfied-looking prerequisite is still never a satisfied start gate", func(t *testing.T) {
		advertiseMilestones(t)
		streams := ms05Load(t, ms05Tree(t, nil))
		ms05SetStatus(streams, "done", "stream-y/01")
		checkBriefFiles(streams, streams)
		e := eligibilityForStreams(streams)["build/12"]
		if e.Verdict != VerdictHeld || !hasHold(e, "stream-y:m-01@1", StateCouldNotCheck) {
			t.Fatalf("with Y's work done its acceptance is still unobserved: brief 12 must be held could-not-check, never released; got %+v", e)
		}
		ms05SetStatus(streams, "verified", "stream-y/01")
		g := ms05Graph(t, streams)
		if y := g.Milestones[ms05Y]; y.Completion != evaluation.GraphBlocked {
			t.Errorf("verified is not done: Y must stay blocked; got %+v", y)
		}
	})

	t.Run("an unobserved milestone stays unknown, never no dependency", func(t *testing.T) {
		// milestone-v1 unadvertised: no record is read, so the requirement is unknown.
		streams := ms05Load(t, ms05Tree(t, nil))
		_, notices := checkBriefFiles(streams, streams)
		e := eligibilityForStreams(streams)["build/12"]
		if !hasHold(e, "stream-y:m-01@1", StateCouldNotCheck) || !ms05Has(notices, "brief-12-gated.md", "held could-not-check", "not a loaded record version") {
			t.Fatalf("an unread milestone record must hold could-not-check with its reason; got %+v\n%s", e, strings.Join(notices, "\n"))
		}
		if e := eligibilityForStreams(streams)["build/11"]; len(e.Holds) != 0 {
			t.Errorf("brief 11 declares nothing and must stay unheld; got %+v", e)
		}
	})
}

// regression: milestone-support/05
func TestMilestoneGraphUnknowns(t *testing.T) {
	t.Run("a brief requiring its own milestone is a refused cycle", func(t *testing.T) {
		advertiseMilestones(t)
		root := ms05Tree(t, map[string]func(string) string{
			"build/brief-12-gated.md": ms04Replace(`requires-milestones: ["stream-y:m-01@1"]`, `requires-milestones: ["build:m-03@1"]`),
		})
		streams := ms05Load(t, root)
		problems, _ := checkBriefFiles(streams, streams)
		if !ms05Has(problems, "milestone graph: dependency cycle", "brief "+ms05B12, "milestone "+ms05M3, "refused") {
			t.Fatalf("brief 12 requiring m-03, which it contributes to, must be refused as a cycle; problems:\n%s", strings.Join(problems, "\n"))
		}
		if e := eligibilityForStreams(streams)["build/12"]; !hasHold(e, "build:m-03@1", StateUnsatisfied) {
			t.Errorf("the cyclic start requirement must hold unsatisfied; got %+v", e)
		}
		if gate := ms05Graph(t, streams).Briefs[ms05B12].Gates; len(gate) != 1 || gate[0].State != evaluation.GraphCycle {
			t.Errorf("the gate must be typed cycle; got %+v", gate)
		}
	})

	t.Run("an indirect cycle across roots is refused", func(t *testing.T) {
		advertiseMilestones(t)
		// Root A: brief 11 requires the other root's m-05. Root B: m-05 requires
		// root A's m-03, which brief 11 contributes to. No single edge is a loop.
		rootA := ms05Tree(t, map[string]func(string) string{
			"build/brief-11-independent.md": ms04Replace("schema: brief-v4\nversion: 1\n", "schema: brief-v4\nversion: 1\nrequires-milestones: [\"assay:other:remote:m-05@1\"]\n"),
		})
		rootB := t.TempDir()
		writeMilestoneFile(t, rootB, "docs/streams/graph-repos.yaml", "schema: graph-repos-v1\ncell: assay\nself: other\nrepos:\n  assay: {cell: assay, repo: example/assay}\n  other: {cell: assay, repo: example/other}\n")
		writeMilestoneFile(t, rootB, "docs/spec.md", "spec\n")
		readme, _ := os.ReadFile(filepath.Join(rootA, "docs/streams/stream-z/README.md"))
		writeMilestoneFile(t, rootB, "docs/streams/remote/README.md", strings.NewReplacer("stream-z", "remote", "brief-01-z-work.md", "brief-01-remote.md", "Z work", "Remote").Replace(string(readme)))
		brief, _ := os.ReadFile(filepath.Join(rootA, "docs/streams/stream-z/brief-01-z-work.md"))
		writeMilestoneFile(t, rootB, "docs/streams/remote/brief-01-remote.md", strings.NewReplacer("assay:assay:stream-z:", "assay:other:remote:", "000000000531", "000000000541", "Z work", "Remote").Replace(string(brief)))
		record, _ := os.ReadFile(filepath.Join(rootA, "docs/streams/stream-z/milestones/m-02-z-capability.md"))
		writeMilestoneFile(t, rootB, "docs/streams/remote/milestones/m-05-remote.md", strings.NewReplacer("assay:assay:stream-z:m-02", "assay:other:remote:m-05", "assay:assay:stream-z:b-01", "assay:other:remote:b-01", "requires-milestones: []", `requires-milestones: ["assay:assay:build:m-03@1"]`).Replace(string(record)))
		streams := ms05Load(t, rootA, rootB)
		problems, _ := checkBriefFiles(streams, streams)
		if !ms05Has(problems, "milestone graph: dependency cycle", "brief "+ms05B11, "milestone assay:other:remote:m-05@1", "milestone "+ms05M3) {
			t.Fatalf("the cross-root cycle 11 -> other m-05 -> m-03 -> 11 must be refused; problems:\n%s", strings.Join(problems, "\n"))
		}
		if e := eligibilityForStreams(streams)["build/11"]; !hasHold(e, "assay:other:remote:m-05@1", StateUnsatisfied) {
			t.Errorf("brief 11's cyclic requirement must hold unsatisfied; got %+v", e)
		}
	})

	t.Run("aliases of one brief deduplicate", func(t *testing.T) {
		advertiseMilestones(t)
		// A second record names brief 11 by its elided spelling.
		m04, _ := os.ReadFile("testdata/milestone-support/05/tree/docs/streams/stream-z/milestones/m-02-z-capability.md")
		root := ms05Tree(t, map[string]func(string) string{
			"build/milestones/m-04-shared.md": func(string) string {
				return strings.NewReplacer("assay:assay:stream-z:m-02", "assay:assay:build:m-04", `["assay:assay:stream-z:b-01"]`, `["build:b-11"]`, "`assay:assay:stream-z:b-01`", "`build:b-11`").Replace(string(m04))
			},
		})
		streams := ms05Load(t, root)
		g := ms05Graph(t, streams)
		m4 := "assay:assay:build:m-04@1"
		if got := g.Milestones[m4].Contributors; !slices.Equal(got, []string{ms05B11}) {
			t.Fatalf("an elided spelling must normalize to the one logical key; m-04 contributors = %v", got)
		}
		if got := g.Briefs[ms05B11].Membership; !slices.Equal(got, []string{ms05M3, m4}) {
			t.Errorf("membership must be derived and deduplicated; got %v", got)
		}
		if n := g.ContributorCount(ms05M3, m4); n != 2 {
			t.Errorf("brief 11 shared by m-03 and m-04 must be counted once: got %d contributors, want 2", n)
		}
	})

	t.Run("a missing pinned revision stays unknown and never falls back", func(t *testing.T) {
		advertiseMilestones(t)
		// m-03 pins brief 12 at revision 2; the snapshot holds revision 3.
		root := ms05Tree(t, map[string]func(string) string{
			"build/brief-12-gated.md": ms04Replace("version: 2\n", "version: 3\n"),
		})
		streams := ms05Load(t, root)
		ms05SetStatus(streams, "done", "build/11", "build/12", "stream-y/01", "stream-z/01")
		m3 := ms05Graph(t, streams).Milestones[ms05M3]
		if m3.Completion != evaluation.GraphUnknown {
			t.Fatalf("with every brief done but b-12@2 unavailable, m-03 must be unknown, never complete; got %+v", m3)
		}
		if !ms05Has(m3.Unknowns, "contributor assay:assay:build:b-12@2 is unknown", "revision") {
			t.Errorf("the unknown must name the pinned revision; got %q", m3.Unknowns)
		}
	})

	t.Run("unknown roots and withdrawn work never satisfy", func(t *testing.T) {
		advertiseMilestones(t)
		root := ms05Tree(t, map[string]func(string) string{
			"stream-z/milestones/m-02-z-capability.md": ms04Replace(`contributes: ["assay:assay:stream-z:b-01"]`, `contributes: ["assay:assay:stream-z:b-01", "assay:other:remote:b-07"]`),
		})
		streams := ms05Load(t, root)
		ms05SetStatus(streams, "done", "stream-z/01")
		ms05SetStatus(streams, "withdrawn", "stream-y/01")
		g := ms05Graph(t, streams)
		if z := g.Milestones[ms05Z]; z.Completion != evaluation.GraphUnknown || !ms05Has(z.Unknowns, "contributor assay:other:remote:b-07 is unknown", "not in a loaded root") {
			t.Errorf("a contributor in an unloaded root is unknown, never no dependency; got %+v", z)
		}
		y := g.Milestones[ms05Y]
		if y.Completion != evaluation.GraphBlocked || !ms05Has(y.Blockers, "withdrawn work never satisfies") {
			t.Errorf("a withdrawn contributor must block, never satisfy; got %+v", y)
		}
		checkBriefFiles(streams, streams)
		if e := eligibilityForStreams(streams)["build/12"]; !hasHold(e, "stream-y:m-01@1", StateUnsatisfied) {
			t.Errorf("brief 12's gate on a milestone with withdrawn work must hold unsatisfied; got %+v", e)
		}
	})

	for _, advertised := range []bool{true, false} {
		t.Run("a logical-key collision across roots holds every claimant, never picks a winner (milestone-v1 advertised="+strconv.FormatBool(advertised)+")", func(t *testing.T) {
			if advertised {
				advertiseMilestones(t)
			}
			// Root B is the same fixture under the same cell and self alias, with
			// brief 12's start requirement deleted and fresh uuids. Without the
			// guard root A's row 12 receives root B's empty gate list.
			rootA := ms05Tree(t, nil)
			fresh := strings.NewReplacer("-000000000511", "-000000000611", "-000000000512", "-000000000612", "-000000000521", "-000000000621", "-000000000531", "-000000000631")
			rootB := ms05Tree(t, map[string]func(string) string{
				"build/brief-12-gated.md": func(s string) string {
					return fresh.Replace(ms04Replace("requires-milestones: [\"stream-y:m-01@1\"]\n", "")(s))
				},
				"build/brief-11-independent.md": fresh.Replace,
				"stream-y/brief-01-y-work.md":   fresh.Replace,
				"stream-z/brief-01-z-work.md":   fresh.Replace,
			})
			streams := ms05Load(t, rootA, rootB)
			_, notices := checkBriefFiles(streams, streams)
			found := false
			for _, s := range streams {
				if s.Root != rootA || s.Name != "build" {
					continue
				}
				for _, row := range s.Briefs {
					if row.Num != "12" {
						continue
					}
					held := false
					found = true
					for _, e := range row.Gates {
						held = held || (e.Ref == "stream-y:m-01@1" && e.Resolved == StateCouldNotCheck && strings.Contains(e.Why, "claimed by more than one scanned brief"))
					}
					if !held {
						t.Fatalf("root A's brief 12 must keep its start requirement, held could-not-check naming the collision; gates %+v\n%s", row.Gates, strings.Join(notices, "\n"))
					}
				}
			}
			if !found {
				t.Fatal("root A's build brief 12 was not scanned")
			}
			if !ms05Has(notices, "brief-12-gated.md", "held could-not-check", "assay:assay:build:b-12", "claimed by more than one scanned brief") {
				t.Errorf("the collision must be announced; notices:\n%s", strings.Join(notices, "\n"))
			}
			if !advertised {
				return // records are not read while milestone-v1 is unadvertised
			}
			if m3 := ms05Graph(t, streams).Milestones[ms05M3]; m3.Completion != evaluation.GraphUnknown || !ms05Has(m3.Unknowns, "declared by more than one loaded record") {
				t.Errorf("a milestone declared in both roots must stay unknown, never pick one record; got %+v", m3)
			}
		})
	}

	t.Run("a root whose owner cannot be established holds could-not-check", func(t *testing.T) {
		advertiseMilestones(t)
		root := ms05Tree(t, nil)
		if err := os.Remove(filepath.Join(root, "docs/streams/graph-repos.yaml")); err != nil {
			t.Fatal(err)
		}
		streams := ms05Load(t, root)
		_, notices := checkBriefFiles(streams, streams)
		if e := eligibilityForStreams(streams)["build/12"]; !hasHold(e, "stream-y:m-01@1", StateCouldNotCheck) {
			t.Fatalf("an unresolvable owning root must hold could-not-check; got %+v\n%s", e, strings.Join(notices, "\n"))
		}
	})
}

// regression: milestone-support/05
func TestMilestoneLatticeStaysLinear(t *testing.T) {
	// A diamond lattice: every milestone in layer i requires both milestones of
	// layer i+1, and each milestone has one unfinished contributor of its own.
	// Re-qualifying a prerequisite's detail on every path doubles it per layer
	// (2^depth); the closure must carry each blocker once.
	const depth = 16
	ref := func(i int, side string) string { return "assay:assay:lat:m-" + side + strconv.Itoa(i) }
	var in evaluation.MilestoneGraphInput
	for i := 0; i < depth; i++ {
		for _, side := range []string{"a", "b"} {
			m := evaluation.GraphMilestone{Key: ref(i, side), Version: 1, Contributes: []evaluation.WorkPin{{Key: "assay:assay:lat:b-" + side + strconv.Itoa(i)}}}
			if i+1 < depth {
				m.Requires = []evaluation.MilestonePin{{Key: ref(i+1, "a"), Version: 1}, {Key: ref(i+1, "b"), Version: 1}}
			}
			in.Milestones = append(in.Milestones, m)
		}
	}
	in.Briefs = []evaluation.GraphBrief{{Key: "assay:assay:lat:b-start", Status: "planned", Requires: []evaluation.MilestonePin{{Key: ref(0, "a"), Version: 1, Spelling: "lat:m-a0@1"}}}}
	in.Lookup = func(pin evaluation.WorkPin) evaluation.PinObservation {
		return evaluation.PinObservation{Found: true, Brief: evaluation.GraphBrief{Key: pin.Key, Status: "in-progress"}}
	}
	g := evaluation.BuildMilestoneGraph(in)
	top := g.Milestones[ref(0, "a")+"@1"]
	if want := 1 + 2*(depth-1); top.Completion != evaluation.GraphBlocked || len(top.Blockers) != want {
		t.Fatalf("the top of a %d-layer lattice must carry each of its %d reachable blockers once; got %s with %d", depth, want, top.Completion, len(top.Blockers))
	}
	gate := g.Briefs["assay:assay:lat:b-start"].Gates[0]
	if gate.State != evaluation.GraphBlocked || len(gate.Blockers) != 21 || !strings.HasPrefix(gate.Blockers[20], "… and 11 more blockers") {
		t.Fatalf("a start gate's hold detail must be bounded; got %s with %d blockers, last %q", gate.State, len(gate.Blockers), gate.Blockers[len(gate.Blockers)-1])
	}
}
