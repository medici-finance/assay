package deskkit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/adapters"
	github "github.com/medici-finance/assay/forgeread/adapters/github"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/workflow/store"
)

const coldInventoryRows = 3569

// coldInventoryPages models descending updated-time order and real 100-row page
// boundaries. Size comes from synthetic PR bodies, with one extra body byte on
// the final page for the overflow case.
func coldInventoryPages(t *testing.T, newest time.Time, extra int64) []string {
	t.Helper()
	rows := make([]string, coldInventoryRows)
	baseline := int64(0)
	for i := range rows {
		number := coldInventoryRows - i
		stamp := newest.Add(-time.Duration(i) * time.Millisecond)
		row := ingestRow(ForgeGitHub, mirror.Changes, number, stamp, true)
		row = strings.Replace(row, `"sha":"head"`, fmt.Sprintf(`"sha":"%040x"`, number), 1)
		if number%10 != 0 {
			row = strings.Replace(row, `"state":"open"`, `"state":"closed"`, 1)
		}
		rows[i] = row
		// One separator per row, plus one extra bracket byte per page.
		baseline += int64(len(row) + 1)
	}
	pageCount := (len(rows) + github.PerPage - 1) / github.PerPage
	budget := adapters.MaxUpdateBytes - baseline - int64(pageCount)
	if budget < 0 {
		t.Fatal("synthetic inventory cannot fit the requested payload size")
	}
	bodySize := int(budget / int64(len(rows)))
	remainder := int(budget % int64(len(rows)))
	for i, row := range rows {
		n := bodySize
		if i == len(rows)-1 {
			n += remainder + int(extra)
		}
		rows[i] = strings.Replace(row, `"body":""`, `"body":"`+strings.Repeat("x", n)+`"`, 1)
	}
	pages := make([]string, 0, pageCount)
	var total int64
	for start := 0; start < len(rows); start += github.PerPage {
		page := "[" + strings.Join(rows[start:min(start+github.PerPage, len(rows))], ",") + "]"
		if int64(len(page)) >= adapters.MaxUpdateBytes || total >= adapters.MaxUpdateBytes {
			t.Fatal("synthetic inventory crosses the aggregate ceiling before the final response or exceeds the per-response ceiling")
		}
		pages = append(pages, page)
		total += int64(len(page))
	}
	size := adapters.MaxUpdateBytes + extra
	if total != size {
		t.Fatalf("fixture produced %d native bytes, want %d", total, size)
	}
	return pages
}

func checkPersistedColdInventory(t *testing.T) {
	newest := time.Now().UTC().Truncate(time.Millisecond).Add(-time.Minute)
	for _, test := range []struct {
		name  string
		extra int64
	}{
		{"at-byte-ceiling", 0},
		{"one-byte-over", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			size := adapters.MaxUpdateBytes + test.extra
			pages := coldInventoryPages(t, newest, test.extra)
			if len(pages) != 36 {
				t.Fatalf("want a 36-page inventory, got %d pages", len(pages))
			}
			// Native response and persisted snapshot budgets are independent.
			// Keep SQL capacity out of this control while using the pilot's
			// native request, freshness, and fill budgets.
			limits := store.MirrorLimits{MaxRecords: 4000, MaxBytes: 16 << 20}
			f := newPersistedNativeWithLimits(t, ForgeGitHub, mirror.Changes, func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				page, err := strconv.Atoi(q.Get("page"))
				if err != nil || page < 1 || page > len(pages) || r.URL.Path != "/repos/o/r/pulls" ||
					q.Get("per_page") != strconv.Itoa(github.PerPage) || q.Get("state") != "all" ||
					q.Get("sort") != "updated" || q.Get("direction") != "desc" || q.Get("since") != "" {
					t.Errorf("unexpected full-inventory request %s", r.URL.RequestURI())
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if page < len(pages) {
					w.Header().Set("Link", fmt.Sprintf(`<https://%s/repos/o/r/pulls?page=%d>; rel="next"`, r.Host, page+1))
				}
				_, _ = io.WriteString(w, pages[page-1])
			}, []string{"caller-a"}, true, limits, func(c *store.RefreshConfig) {
				c.Native = store.NativeLimits{CallerPerWindow: 96, AdmissionPerWindow: 96, SourcePerWindow: 96, PerFill: 48, Concurrent: 1, Reservations: 512, Window: time.Hour}
				c.Limits.CallerFillsPerWindow, c.Limits.AdmissionFillsPerWindow = 2, 2
				c.Limits.MinFreshness, c.Limits.MaxFreshness = 5*time.Minute, 30*time.Minute
				c.Limits.ActiveFills, c.Limits.QueuedCallers, c.Limits.AdmissionFills, c.Limits.AdmissionWaiters = 1, 1, 1, 1
				c.Limits.FailureEntries, c.Limits.Cooldown, c.Limits.FailureTTL = 8, time.Minute, 5*time.Minute
				c.Limits.WaitTimeout, c.Limits.FillTimeout = 20*time.Second, 2*time.Minute
			})
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if receipt, err := f.operator.ProvisionRefresh(ctx, *f.initial); err != nil || receipt.Records != 3 {
				t.Fatalf("operator could not initialize the synthetic cold scope: %v", err)
			}
			before, err := f.observations.Read(ctx, f.admission, f.scope)
			if err != nil || before.Version != "" || before.Coverage.State != mirror.NeverRead || len(before.Records) != 0 {
				t.Fatalf("expected an empty cold SQL scope: %v", err)
			}

			started := time.Now()
			result, err := joinedRead(t, f, "caller-a", f.config.Limits.MaxFreshness)
			coldElapsed := time.Since(started)
			if err != nil {
				t.Fatal(err)
			}
			after, err := f.observations.Read(ctx, f.admission, f.scope)
			if err != nil {
				t.Fatal(err)
			}
			if test.extra == 0 {
				if !result.Fulfilled || !result.Available || len(result.Snapshot.Records) != coldInventoryRows ||
					after.Coverage.State != mirror.Complete || after.Coverage.LastComplete.IsZero() ||
					!after.Coverage.LastInventory.Equal(after.Coverage.LastComplete) ||
					!after.Coverage.ObservedUpTo.Equal(newest) || len(after.Records) != coldInventoryRows {
					t.Fatalf("fitting cold inventory did not commit all rows and a complete coverage watermark: %+v, rows %d", after.Coverage, len(after.Records))
				}
				for i, record := range after.Records {
					if record.ID.Number != int64(i+1) || !record.InInventory {
						t.Fatalf("persisted record %d missing or out of order", i+1)
					}
				}
			} else {
				if result.Fulfilled || result.Available || result.Reason != mirror.Truncated || result.RetryAt.IsZero() ||
					after.Coverage.State != mirror.Unavailable || after.Coverage.Reason != mirror.Truncated ||
					!after.Coverage.LastComplete.IsZero() || !after.Coverage.LastInventory.IsZero() || !after.Coverage.ObservedUpTo.IsZero() ||
					len(result.Snapshot.Records) != 0 || len(after.Records) != 0 {
					t.Fatalf("overflow exposed or persisted a cold inventory: reason %s, coverage %+v, rows %d", result.Reason, after.Coverage, len(after.Records))
				}
			}

			warm, err := joinedRead(t, f, "caller-a", f.config.Limits.MaxFreshness)
			if test.extra == 0 && err != nil || test.extra != 0 && !errors.Is(err, refresh.ErrHeld) {
				t.Fatalf("repeat query did not use the persisted result or cooldown: %v", err)
			}
			if warm.Fulfilled != result.Fulfilled || warm.Reason != result.Reason ||
				f.requests.Load() != int64(len(pages)) || f.observer.scans.Load() != 1 {
				t.Fatalf("repeat query changed coverage or fetched upstream again: %v, native requests %d, scans %d", err, f.requests.Load(), f.observer.scans.Load())
			}
			if test.extra != 0 {
				retry := persistedRetry(t, f)
				if retry == nil || !retry.Equal(result.RetryAt) {
					t.Fatal("overflow cooldown changed on a repeated query")
				}
			}
			sql, err := f.fixture.ConnectOwned(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := sql.Close(); err != nil {
					t.Error(err)
				}
			})
			var requests, charged, decoded int64
			err = sql.QueryRowContext(ctx, `
				SELECT (SELECT http_used FROM public.refresh_budgets),
				       count(*), coalesce(sum(decoded_bytes),0)
				FROM public.refresh_attempts
				WHERE observed AND http_known AND decoded_known AND response_complete AND status=200
			`).Scan(&charged, &requests, &decoded)
			if err != nil || requests != int64(len(pages)) || charged != int64(len(pages)) || decoded != size {
				t.Fatalf("SQL native accounting mismatch: %v, requests %d, charged %d, decoded %d", err, requests, charged, decoded)
			}
			t.Logf("Cold input %d PRs, %d native pages, %d decoded bytes, %d persisted rows, coverage %s, repeat native requests %d, cold latency %s",
				coldInventoryRows, requests, decoded, len(after.Records), after.Coverage.State, f.requests.Load()-requests, coldElapsed.Round(time.Millisecond))
		})
	}
}
