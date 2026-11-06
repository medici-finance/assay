package pager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

var errTransient = errors.New("transient")
var errDenied = errors.New("denied")

func classify(err error) Class {
	if errors.Is(err, errTransient) {
		return Transient
	}
	return Stop
}

// fakeConn is an n-item connection served by cursor, where the cursor is the next offset.
type fakeConn struct {
	n     int
	fail  func(req Request, call int) error
	calls []Request
}

func (f *fakeConn) fetch(_ context.Context, req Request) (Page[int], error) {
	f.calls = append(f.calls, req)
	if f.fail != nil {
		if err := f.fail(req, len(f.calls)); err != nil {
			return Page[int]{}, err
		}
	}
	start := 0
	if req.Cursor != "" {
		start, _ = strconv.Atoi(req.Cursor)
	}
	end := min(start+req.Size, f.n)
	var items []int
	for i := start; i < end; i++ {
		items = append(items, i)
	}
	return Page[int]{Items: items, More: end < f.n, Next: strconv.Itoa(end)}, nil
}

func key(i int) string { return strconv.Itoa(i) }

// regression: #3096
func TestWalkPagesReproduceOneLargePage(t *testing.T) {
	for _, n := range []int{0, 1, 24, 25, 26, 57, 99, 100} {
		big := &fakeConn{n: n}
		one, err := Walk(context.Background(), Policy{PageSize: 100, MaxItems: 100, Classify: classify}, key, big.fetch)
		if err != nil || len(big.calls) != 1 {
			t.Fatalf("n=%d one large page: err=%v calls=%d", n, err, len(big.calls))
		}
		small := &fakeConn{n: n}
		paged, err := Walk(context.Background(), Policy{PageSize: 25, MaxItems: 100, Classify: classify}, key, small.fetch)
		if err != nil {
			t.Fatalf("n=%d paged: %v", n, err)
		}
		if !reflect.DeepEqual(one.Items, paged.Items) || one.Truncated != paged.Truncated {
			t.Fatalf("n=%d: paged walk %v/%v differs from one page %v/%v", n, paged.Items, paged.Truncated, one.Items, one.Truncated)
		}
		if want := max(1, (n+24)/25); paged.Stats.Requests != want || paged.Stats.Pages != want {
			t.Errorf("n=%d: %d requests / %d pages, want %d", n, paged.Stats.Requests, paged.Stats.Pages, want)
		}
	}
}

func TestWalkCapIsHonest(t *testing.T) {
	for _, c := range []struct {
		n         int
		truncated bool
	}{{99, false}, {100, false}, {101, true}, {250, true}} {
		f := &fakeConn{n: c.n}
		r, err := Walk(context.Background(), Policy{PageSize: 25, MaxItems: 100}, key, f.fetch)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Items) != min(c.n, 100) || r.Truncated != c.truncated {
			t.Errorf("n=%d: %d items truncated=%v, want %d truncated=%v", c.n, len(r.Items), r.Truncated, min(c.n, 100), c.truncated)
		}
		for _, req := range f.calls {
			if req.Size > 25 {
				t.Errorf("n=%d: asked for %d, above the page size", c.n, req.Size)
			}
		}
	}
	// A provider that ignores the requested size and overfills past the cap is truncated, not
	// silently clipped.
	over := func(_ context.Context, req Request) (Page[int], error) {
		items := make([]int, 120)
		for i := range items {
			items[i] = i
		}
		return Page[int]{Items: items}, nil
	}
	r, err := Walk(context.Background(), Policy{MaxItems: 100}, key, over)
	if err != nil || len(r.Items) != 100 || !r.Truncated {
		t.Fatalf("overfilled page: %d items truncated=%v err=%v", len(r.Items), r.Truncated, err)
	}
}

// regression: #3096
func TestWalkTransientOnceThenSmallerPageCompletes(t *testing.T) {
	f := &fakeConn{n: 60, fail: func(req Request, call int) error {
		if call == 2 { // the second page fails once at full size
			return fmt.Errorf("HTTP 504: %w", errTransient)
		}
		return nil
	}}
	r, err := Walk(context.Background(), Policy{PageSize: 25, MaxItems: 100, Classify: classify}, key, f.fetch)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) != 60 || r.Truncated {
		t.Fatalf("got %d items truncated=%v", len(r.Items), r.Truncated)
	}
	want := []Request{{"", 25}, {"25", 25}, {"25", 12}, {"37", 12}, {"49", 12}}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("requests %v, want %v (same cursor at half size, reduced size kept)", f.calls, want)
	}
	if r.Stats.Requests != 5 || r.Stats.Retries != 1 || r.Stats.PageSize != 12 {
		t.Errorf("stats %+v", r.Stats)
	}
}

// regression: #3096
func TestWalkSustainedOutageRequestCount(t *testing.T) {
	f := &fakeConn{n: 60, fail: func(Request, int) error { return errTransient }}
	r, err := Walk(context.Background(), Policy{PageSize: 25, MinPageSize: 6, MaxItems: 100, Classify: classify}, key, f.fetch)
	var ex *ExhaustedError
	if !errors.As(err, &ex) || !errors.Is(err, errTransient) {
		t.Fatalf("want ExhaustedError wrapping the cause, got %v", err)
	}
	if r.Items != nil {
		t.Fatalf("an exhausted walk returned items %v", r.Items)
	}
	want := []Request{{"", 25}, {"", 12}, {"", 6}}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("requests under a sustained outage %v, want exactly %v — each strictly smaller, none repeated", f.calls, want)
	}
	if ex.Stats.Requests != 3 || ex.Stats.Retries != 2 || !strings.Contains(ex.Error(), "floor") {
		t.Errorf("exhausted %+v: %v", ex.Stats, ex)
	}
}

func TestWalkRetryBudgetSpansCursors(t *testing.T) {
	// Every first attempt at a cursor fails: the walk-wide budget, not the floor, ends it.
	failed := map[string]bool{}
	f := &fakeConn{n: 100, fail: func(req Request, _ int) error {
		if !failed[req.Cursor] {
			failed[req.Cursor] = true
			return errTransient
		}
		return nil
	}}
	_, err := Walk(context.Background(), Policy{PageSize: 50, MinPageSize: 1, MaxItems: 100, MaxRetries: 2, Classify: classify}, key, f.fetch)
	var ex *ExhaustedError
	if !errors.As(err, &ex) || !strings.Contains(ex.Error(), "retry budget") || ex.Stats.Retries != 2 {
		t.Fatalf("want the retry budget to end the walk, got %v", err)
	}
}

// regression: #3096
func TestWalkStopFailuresAreNotRetried(t *testing.T) {
	for _, nilClassify := range []bool{false, true} {
		f := &fakeConn{n: 60, fail: func(Request, int) error { return errDenied }}
		pol := Policy{MaxItems: 100, Classify: classify}
		if nilClassify {
			pol.Classify = nil
		}
		_, err := Walk(context.Background(), pol, key, f.fetch)
		if err != errDenied {
			t.Fatalf("a Stop failure must come back unchanged, got %v", err)
		}
		if len(f.calls) != 1 {
			t.Fatalf("a Stop failure was retried: %v", f.calls)
		}
	}
	// With a nil classifier even a transient error is a Stop.
	f := &fakeConn{n: 60, fail: func(Request, int) error { return errTransient }}
	if _, err := Walk(context.Background(), Policy{MaxItems: 100}, key, f.fetch); err != errTransient || len(f.calls) != 1 {
		t.Fatalf("nil classifier retried: %v %v", err, f.calls)
	}
}

func TestWalkDedupesAndRefusesStalledCursor(t *testing.T) {
	// The connection shifts by one between pages: the boundary item repeats.
	pages := map[string]Page[int]{
		"":  {Items: []int{9, 8, 7}, More: true, Next: "a"},
		"a": {Items: []int{7, 6, 5}, More: false, Next: "b"},
	}
	fetch := func(_ context.Context, req Request) (Page[int], error) { return pages[req.Cursor], nil }
	r, err := Walk(context.Background(), Policy{PageSize: 3, MaxItems: 100}, key, fetch)
	if err != nil || !reflect.DeepEqual(r.Items, []int{9, 8, 7, 6, 5}) {
		t.Fatalf("dedupe: %v %v", r.Items, err)
	}
	for _, next := range []string{"", "x"} {
		stuck := func(_ context.Context, req Request) (Page[int], error) {
			return Page[int]{Items: []int{1}, More: true, Next: next}, nil
		}
		_, err := Walk(context.Background(), Policy{PageSize: 3, MaxItems: 100}, key, stuck)
		var st *StalledError
		var bud *BudgetError
		if !errors.As(err, &st) && !errors.As(err, &bud) {
			t.Fatalf("next=%q: a non-advancing cursor must refuse, got %v", next, err)
		}
	}
}

func TestWalkPolicyBounds(t *testing.T) {
	fetch := (&fakeConn{n: 1}).fetch
	if _, err := Walk(context.Background(), Policy{}, key, fetch); !errors.Is(err, ErrUnbounded) {
		t.Fatalf("an unbounded walk must be refused, got %v", err)
	}
	if _, err := Walk(context.Background(), Policy{MaxItems: -1}, key, fetch); err == nil {
		t.Fatal("a negative bound must be refused")
	}
	// MaxRequests alone bounds a walk; reaching it with more reported is a refusal.
	_, err := Walk(context.Background(), Policy{PageSize: 1, MaxRequests: 3}, key, (&fakeConn{n: 10}).fetch)
	var bud *BudgetError
	if !errors.As(err, &bud) || bud.Stats.Requests != 3 {
		t.Fatalf("want a BudgetError after 3 requests, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Walk(ctx, Policy{MaxItems: 10}, key, fetch); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled context must stop the walk, got %v", err)
	}
}

// TestPagerImportsNothingOnline: the pager is shared by every read adapter and the future forge
// client, so it carries no transport, process launch or other module package of its own.
func TestPagerImportsNothingOnline(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", ".")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	if got := strings.Fields(string(out)); !reflect.DeepEqual(got, []string{"github.com/medici-finance/assay/forgeread/adapters/pager"}) {
		t.Fatalf("non-standard closure %v", got)
	}
	std, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range strings.Fields(string(std)) {
		if d == "net/http" || d == "os/exec" || d == "database/sql" {
			t.Errorf("pager closure reaches %s", d)
		}
	}
}
