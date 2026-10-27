package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The lint hold and the generated drive-snapshot output a row move changes
// (#2694). A drive-plan page carries a generated region rendered from the board;
// moving a driven row's Status cell changes that render, so judging the README
// edit alone against the unchanged page always reported drive-region-drift and
// held the row, even though the README edit together with the re-rendered
// region lints clean. The hold now judges the complete proposed output.

func readPlan(t *testing.T, root, slug string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "docs", "roadmap", "drives", slug+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// reconcileRegen is reconcileWrites with reconcile's --regenerate opt-in on.
func reconcileRegen(root string, cells []BriefCell, write bool) ([]appliedRow, []heldRow, error) {
	return reconcileWritesWith(root, cells, write, holdOptions{crossCheck: holdCrossCheck, regenerate: true})
}

// boardReadmes returns every stream README's bytes under root.
func boardReadmes(t *testing.T, root string) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, "docs", "streams", "*", "README.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("fixture drift: no stream READMEs under %s (%v)", root, err)
	}
	out := map[string]string{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out[p] = string(b)
	}
	return out
}

// regression: #2694
func TestHoldRegeneratesDriveRegion(t *testing.T) {
	pinNow(t, driveTestNow)
	for _, write := range []bool{false, true} {
		root, cells := driveManyRowBoard(t)
		plan := filepath.Join(root, "docs", "roadmap", "drives", "d1.md")
		planBefore := readPlan(t, root, "d1")
		readmesBefore := boardReadmes(t, root)
		lintBefore := lintProblemLines(t, root)
		if len(lintBefore) != 0 {
			t.Fatalf("fixture drift: the clean board plus its drive snapshot must lint clean, got %v", lintBefore)
		}

		rows, held, err := reconcileRegen(root, cells, write)
		if err != nil {
			t.Fatalf("write=%v: %v", write, err)
		}
		if len(held) != 0 {
			t.Fatalf("write=%v: a driven row whose README edit and re-rendered region lint clean must not be held, got %+v", write, held)
		}
		if len(rows) != len(cells) {
			t.Fatalf("write=%v: want all %d witnessed rows admitted, got %d", write, len(cells), len(rows))
		}
		var last string
		driven := 0
		for _, r := range rows {
			isDriven := strings.HasPrefix(r.ID, "mr00/") || strings.HasPrefix(r.ID, "mr02/")
			if !isDriven {
				if len(r.Generated) != 0 {
					t.Fatalf("write=%v: an undriven row regenerated %+v", write, r.Generated)
				}
				continue
			}
			driven++
			if len(r.Generated) != 1 || r.Generated[0].Path != plan || r.Generated[0].Drive != "d1" {
				t.Fatalf("write=%v: driven row %s must report its regenerated region, got %+v", write, r.ID, r.Generated)
			}
			g := r.Generated[0]
			if g.RegionBefore == g.RegionAfter || g.RegionAfter == "" {
				t.Fatalf("write=%v: %s: the reported region must change, got before=after=%q", write, r.ID, g.RegionAfter)
			}
			if last != "" && g.RegionBefore != last {
				t.Fatalf("write=%v: %s: regions must chain row to row, before=%q want %q", write, r.ID, g.RegionBefore, last)
			}
			last = g.RegionAfter
		}
		if driven != 4 {
			t.Fatalf("fixture drift: want 4 driven moves, got %d", driven)
		}

		if !write {
			if got := readPlan(t, root, "d1"); got != planBefore {
				t.Fatalf("the read-only form wrote the drive plan:\n%s", got)
			}
			for p, c := range boardReadmes(t, root) {
				if readmesBefore[p] != c {
					t.Fatalf("the read-only form wrote %s", p)
				}
			}
			continue
		}
		planAfter := readPlan(t, root, "d1")
		if region, ok := extractDriveRegion(planAfter, "d1"); !ok || region != last {
			t.Fatalf("the written region must be the last reported regionAfter:\nwritten:  %q\nreported: %q", region, last)
		}
		head := planBefore[:strings.Index(planBefore, driveSnapshotBeginPrefix)]
		marker := driveSnapshotBeginPrefix + "d1 @deadbee 2026-08-15" + driveSnapshotMarkerSuffix + "\n"
		tail := planBefore[strings.Index(planBefore, driveSnapshotEndMarker):]
		if planAfter != head+marker+last+tail {
			t.Fatalf("only the text between the markers may change:\n%s", planAfter)
		}
		if fresh := newProblemLines(lintBefore, lintProblemLines(t, root)); len(fresh) != 0 {
			t.Fatalf("--apply wrote a tree the full lint rejects:\n%s", strings.Join(fresh, "\n"))
		}
	}
}

// TestHoldKeepsHandEditedRegion: a region that already differs from its render
// (a hand edit inside the fences) is never re-rendered by the writer — that
// would launder the hand edit the drift rule exists to catch. The move adds no
// PROBLEM (the drift is already there), so it is admitted, and the page is
// left byte for byte.
func TestHoldKeepsHandEditedRegion(t *testing.T) {
	pinNow(t, driveTestNow)
	root, cells := driveManyRowBoard(t)
	planPath := filepath.Join(root, "docs", "roadmap", "drives", "d1.md")
	edited := strings.Replace(readPlan(t, root, "d1"), driveSnapshotEndMarker, "hand edit\n"+driveSnapshotEndMarker, 1)
	if err := os.WriteFile(planPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, _ := driveRegionLintProblems(root, nil, nil, driveTestNow); len(p) != 1 {
		t.Fatalf("fixture drift: the hand edit must drift the region, got %v", p)
	}
	rows, held, err := reconcileRegen(root, cells, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(held) != 0 || len(rows) != len(cells) {
		t.Fatalf("want every row admitted, got rows=%d held=%+v", len(rows), held)
	}
	for _, r := range rows {
		if len(r.Generated) != 0 {
			t.Fatalf("%s re-rendered a hand-edited region: %+v", r.ID, r.Generated)
		}
	}
	if got := readPlan(t, root, "d1"); got != edited {
		t.Fatalf("the hand-edited page must be left byte for byte:\n%s", got)
	}
}

// plantGenRule registers a second rule of the defect class: it compares a
// generated file (docs/gen.txt, the count of implemented rows) against a fresh
// render of the board, the way the drive-region rule does. register, when
// true, registers its regenerator; render is what that regenerator writes.
func plantGenRule(t *testing.T, register bool, render func(n int) string) {
	t.Helper()
	count := func(root string) (int, error) {
		streams, _, err := loadStreams(root)
		if err != nil {
			return 0, err
		}
		n := 0
		for _, s := range streams {
			for _, b := range s.Briefs {
				if b.Status == "implemented" {
					n++
				}
			}
		}
		return n, nil
	}
	genPath := func(root string) string { return filepath.Join(root, "docs", "gen.txt") }
	const drift = "planted-gen-drift: docs/gen.txt differs from a fresh render"
	prevRules, prevGen := statusKeyedLintRules, generatedOutputs
	statusKeyedLintRules = append(append([]statusKeyedLintRule(nil), prevRules...), statusKeyedLintRule{
		name: "plantedGenRule",
		problems: func(e *lintEnv) []string {
			n, err := count(e.root)
			if err != nil {
				return []string{"planted: " + err.Error()}
			}
			b, err := readStaged(genPath(e.root))
			if err != nil || string(b) != fmt.Sprintf("%d\n", n) {
				return []string{drift}
			}
			return nil
		},
	})
	if register {
		generatedOutputs = append(append([]generatedOutput(nil), prevGen...), generatedOutput{
			rule: "plantedGenRule",
			drifted: func(root string, added []string) []string {
				for _, a := range added {
					if a == drift {
						return []string{genPath(root)}
					}
				}
				return nil
			},
			regenerate: func(root, path string) (string, generatedChange, bool) {
				n, err := count(root)
				if err != nil {
					return "", generatedChange{}, false
				}
				b, _ := readStaged(path)
				return render(n), generatedChange{Path: path, Drive: "planted", RegionBefore: string(b), RegionAfter: render(n)}, true
			},
		})
	}
	t.Cleanup(func() { statusKeyedLintRules, generatedOutputs = prevRules, prevGen })
}

// TestHoldGeneratedClass is the defect class beyond the drive region: a
// planted second generated-artifact rule. Without a regenerator its drift
// holds the move (never written stale); with a correct one the move is
// admitted and the artifact written by --apply only — under the --regenerate
// opt-in, and held as before without it; with a regenerator whose output the
// rule rejects, the move is held on the complete proposed output.
func TestHoldGeneratedClass(t *testing.T) {
	right := func(n int) string { return fmt.Sprintf("%d\n", n) }
	wrong := func(n int) string { return fmt.Sprintf("%d\n", n+7) }
	cells := []BriefCell{{ID: "dg/03", Cell: "implemented", Source: "pr", Witness: "PR #3 (merged 0000003)"}}
	for _, tc := range []struct {
		name     string
		register bool
		render   func(int) string
		optIn    bool
		admit    bool
	}{
		{"no regenerator", false, nil, true, false},
		{"right regenerator", true, right, true, true},
		{"right regenerator, no opt-in", true, right, false, false},
		{"wrong regenerator", true, wrong, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plantGenRule(t, tc.register, tc.render)
			for _, write := range []bool{false, true} {
				root := copyDesignGateFixture(t)
				streams, _, err := loadStreams(root)
				if err != nil {
					t.Fatal(err)
				}
				n := 0
				for _, s := range streams {
					for _, b := range s.Briefs {
						if b.Status == "implemented" {
							n++
						}
					}
				}
				gen := filepath.Join(root, "docs", "gen.txt")
				if err := os.WriteFile(gen, []byte(right(n)), 0o644); err != nil {
					t.Fatal(err)
				}
				rows, held, err := reconcileWritesWith(root, cells, write, holdOptions{crossCheck: holdCrossCheck, regenerate: tc.optIn})
				if err != nil {
					t.Fatalf("write=%v: %v", write, err)
				}
				got, _ := os.ReadFile(gen)
				if !tc.admit {
					if len(rows) != 0 || len(held) != 1 || !strings.Contains(strings.Join(held[0].Problems, "\n"), "planted-gen-drift") {
						t.Fatalf("write=%v: want dg/03 held on the generated drift, got rows=%+v held=%+v", write, rows, held)
					}
					if string(got) != right(n) {
						t.Fatalf("write=%v: a held move wrote the generated file: %q", write, got)
					}
					continue
				}
				if len(held) != 0 || len(rows) != 1 || len(rows[0].Generated) != 1 || rows[0].Generated[0].RegionAfter != right(n+1) {
					t.Fatalf("write=%v: want dg/03 admitted with gen.txt re-rendered, got rows=%+v held=%+v", write, rows, held)
				}
				want := right(n)
				if write {
					want = right(n + 1)
				}
				if string(got) != want {
					t.Fatalf("write=%v: gen.txt = %q, want %q", write, got, want)
				}
			}
		})
	}
	stagedReadmes.RLock()
	left := len(stagedReadmes.m)
	stagedReadmes.RUnlock()
	if left != 0 {
		t.Fatalf("reconcileWrites left %d staged file(s) behind", left)
	}
}
