package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// regression: #2703
func TestTrailerWriterObservationScope(t *testing.T) {
	for _, mode := range []string{"writer", "audit", "backfill writer"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", "test-token")
			root := writeBoard(t, map[string]string{"apstream": applyFixtureHeader +
				"| 01 | [first](brief-01-first.md) | 0 | M | done | — | — |\n" +
				"| 02 | [second](brief-02-second.md) | 0 | M | todo | — | — |\n"})
			commitFixture(t, root)
			reads := authoringServer(t, []map[string]any{
				mergedPull(1, "feat/a", "Brief: apstream/01\n"),
				mergedPull(2, "feat/b", "Brief: apstream/02\n"),
			}, map[int][]map[string]any{2: deliveryFiles}, map[int]bool{1: true})
			args := []string{"--root", root, "--repo", "o/r", "--json"}
			if mode != "audit" {
				args = append(args, "--apply")
			}
			if mode == "backfill writer" {
				args = append(args, "--backfill")
			}
			code, out := runReconcileCaptured(t, args...)
			if mode == "backfill writer" {
				if code != reconcileCouldNotCheck || len(*reads) != 2 {
					t.Fatalf("backfill must request both rows and refuse failed reads: code=%d reads=%v", code, *reads)
				}
				return
			}
			if code != reconcileOK {
				t.Fatalf("exit %d: %s", code, out)
			}
			done := cellOf(t, out, "apstream/01")
			if done.Cell != "unknown" {
				t.Fatalf("unobserved or unread row must be unknown: %+v", done)
			}
			var res reconcileResult
			if err := json.Unmarshal([]byte(out), &res); err != nil {
				t.Fatal(err)
			}
			if mode == "writer" {
				if len(*reads) != 1 || (*reads)[0] != 2 || len(res.Unread) != 0 || !strings.Contains(done.Reason, "outside trailer-writer scope") || done.Witness != "" || done.MergedPR != "" {
					t.Fatalf("scope exclusion must read nothing and expose no raw provenance: cell=%+v reads=%v unread=%v", done, *reads, res.Unread)
				}
				if got := cellOf(t, out, "apstream/02"); got.Cell != "implemented" || !strings.Contains(got.MergedPR, "PR #2 ") {
					t.Fatalf("selected delivery lost: %+v", got)
				}
			} else if len(*reads) != 2 || len(res.Unread) == 0 || strings.Contains(done.Reason, "outside trailer-writer scope") {
				t.Fatalf("audit must actually classify completed rows and disclose failed reads: %+v reads=%v", done, *reads)
			}
		})
	}
}

// Synthetic cardinalities from review 5591214765, NOT a replay of its unavailable
// PR dataset: 51 writable, 593 completed, 4 blocked, 39 unmapped; 267 shapes,
// 399 brief heads and 12 outcome-only lists. The real HTTP/cache seams count
// reads. PRShape's summary + single file page + heads cost is modeled from its
// production implementation; this fixture uses fewer than 100 files per PR.
// regression: #2703
func TestTrailerWriterCohortReadBound(t *testing.T) {
	for _, writer := range []bool{false, true} {
		t.Run(fmt.Sprint(writer), func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", "test-token")
			root := writeBoard(t, map[string]string{"apstream": applyFixtureHeader +
				"| 01 | [selected](brief-01-selected.md) | 0 | M | todo | — | — |\n" +
				"| 02 | [completed](brief-02-completed.md) | 0 | M | done | — | — |\n" +
				"| 03 | [blocked](brief-03-blocked.md) | 0 | M | blocked | — | — |\n"})
			var pulls []map[string]any
			files := map[int][]map[string]any{}
			src := &fakeFlipSource{shapes: map[int]prShape{}}
			for n := 1; n <= 687; n++ {
				id := "apstream/01"
				switch {
				case n > 648:
					id = "unmapped/01"
				case n > 644:
					id = "apstream/03"
				case n > 51:
					id = "apstream/02"
				}
				pulls = append(pulls, mergedPull(n, "feat/example", "Brief: "+id+"\n"))
				files[n] = deliveryFiles
				if n <= 50 || (n >= 52 && n <= 268) {
					shape := reportedEvidence(t)
					if n >= 52 && n <= 183 { // 132 of the 267 shapes need a second head.
						for p, h := range shape.Heads {
							second := strings.Replace(p, "brief-27-", "brief-28-", 1)
							shape.Files = append(shape.Files, second)
							shape.Heads[second], shape.Patches[second] = h, shape.Patches[p]
							break
						}
					}
					src.shapes[n], files[n] = shape, evidenceFiles(shape)
				} else if n >= 269 && n <= 280 {
					files[n] = []map[string]any{{"filename": "docs/streams/verify-outcomes/apstream/02.json", "status": "added"}}
				}
			}
			var mu sync.Mutex
			counts := map[string]int{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				kind := "pulls"
				if strings.HasSuffix(r.URL.Path, "/files") {
					kind = "files"
				} else if strings.Contains(r.URL.Path, "/issues") {
					kind = "issues"
				}
				mu.Lock()
				counts[kind]++
				mu.Unlock()
				switch kind {
				case "files":
					parts := strings.Split(r.URL.Path, "/")
					n, _ := strconv.Atoi(parts[len(parts)-2])
					_ = json.NewEncoder(w).Encode(files[n])
				case "issues":
					_, _ = w.Write([]byte(`[]`))
				default:
					pg, _ := strconv.Atoi(r.URL.Query().Get("page"))
					start := (pg - 1) * 100
					end := min(start+100, len(pulls))
					_ = json.NewEncoder(w).Encode(pulls[start:end])
				}
			}))
			defer srv.Close()
			prev := reconcileGHClient
			reconcileGHClient = func(token string) *ghClient { return &ghClient{doer: srv.Client(), base: srv.URL, token: token} }
			t.Cleanup(func() { reconcileGHClient = prev })
			foldFlipSeam(t, src)
			args := []string{"--root", root, "--repo", "o/r", "--json"}
			if writer {
				args = append(args, "--apply")
			}
			code, out := runReconcileCaptured(t, args...)
			if code != reconcileOK {
				t.Fatalf("exit %d: %s", code, out)
			}
			heads := 0
			for _, n := range src.shapesSeen {
				heads += len(src.shapes[n].Heads)
			}
			wantFiles, wantShapes, wantHeads := 648, 267, 399
			if writer {
				wantFiles, wantShapes, wantHeads = 51, 50, 50
				for _, id := range []string{"apstream/02", "apstream/03"} {
					if c := cellOf(t, out, id); c.Cell != "unknown" || c.Witness != "" || c.MergedPR != "" || !strings.Contains(c.Reason, "outside trailer-writer scope") {
						t.Errorf("excluded row leaked provenance: %+v", c)
					}
				}
			}
			if c := cellOf(t, out, "apstream/01"); c.Cell != "implemented" || !strings.Contains(c.MergedPR, "PR #51 ") {
				t.Errorf("Evidence candidates displaced real delivery: %+v", c)
			}
			if counts["files"] != wantFiles || len(src.shapesSeen) != wantShapes || heads != wantHeads || counts["pulls"] != 7 || counts["issues"] != 1 {
				t.Errorf("read bound: HTTP=%v shapes=%d heads=%d, want files=%d shapes=%d heads=%d and 7 pulls + 1 issues", counts, len(src.shapesSeen), heads, wantFiles, wantShapes, wantHeads)
			}
			modeled := counts["files"] + 2*len(src.shapesSeen) + heads + counts["pulls"] + counts["issues"]
			t.Logf("synthetic 687 PR cohort: HTTP=%v shape seam=%d head inputs=%d; modeled upstream requests=%d (previous writer: 51 file reads + 8 shared = 59)", counts, len(src.shapesSeen), heads, modeled)
			if writer && modeled > 209 {
				t.Errorf("scheduled cohort exceeds its 209-request bound: %d", modeled)
			}
		})
	}
}

// regression: #2703
func TestPartialTrailerReadBound(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	declareDelivery(t, root, "03", `{in: "fx#31", covers: partial, note: "skeleton only"}`)
	reads := authoringServer(t, []map[string]any{mergedPull(31, "feat/dg-03", "Brief: dg/03\n")}, nil, map[int]bool{31: true})
	code, out := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK || len(*reads) != 0 {
		t.Fatalf("declared partial must require no file read: exit=%d reads=%v output=%s", code, *reads, out)
	}
	if c := cellOf(t, out, "dg/03"); c.Cell != "in-progress" || c.MergedPR != "" {
		t.Fatalf("declared partial is not a complete witness: %+v", c)
	}
}

// regression: #2703
func TestEvidencePrefetchSet(t *testing.T) {
	pull := func(n int, id string) ghPull { return ghPull{Number: n, MergedAt: "2026-10-01T00:00:00Z", Body: id} }
	pulls := []ghPull{pull(30, "demo/01"), pull(20, "demo/01"), pull(10, "demo/01"), pull(40, "demo/02")}
	shape := reportedEvidence(t)
	reads := authoringServer(t, nil, map[int][]map[string]any{30: evidenceFiles(shape), 20: deliveryFiles, 10: deliveryFiles, 40: deliveryFiles}, nil)
	src := &fakeFlipSource{shapes: map[int]prShape{30: shape}}
	foldFlipSeam(t, src)
	cl := newAuthoringClassifier(reconcileGHClient("test-token"), "o/r", nil)
	briefs := [][2]string{{"demo", "01"}, {"demo", "02"}}
	cl.prefetchBackfill(pulls, briefs)
	sort.Ints(*reads)
	if !equalInts(*reads, []int{20, 30, 40}) || len(src.shapesSeen) != 1 {
		t.Fatalf("prefetch must complete the Evidence-aware walk before matching: files=%v shapes=%v", *reads, src.shapesSeen)
	}
	for i, b := range briefs {
		got, ok, why := matchBackfillPRChecked(pulls, b[0], b[1], cl.nonDelivery)
		if !ok || why != "" || got.Number != []int{20, 40}[i] {
			t.Fatalf("match %v = #%d, %v, %s", b, got.Number, ok, why)
		}
	}
	if len(*reads) != 3 || len(src.shapesSeen) != 1 {
		t.Fatalf("matching repeated prefetch reads: files=%v shapes=%v", *reads, src.shapesSeen)
	}
}
