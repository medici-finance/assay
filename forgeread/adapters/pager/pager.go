// Package pager is the shared, forge-neutral cursor walk for bounded provider reads: cursor
// iteration, adaptive page-size backoff, transient-failure classification and request-count
// accounting. It performs no request of its own. A caller hands it a Fetch that sends ONE page
// request through the caller's already-admitted transport, and a Classify that says which
// failures of that request are worth one more, smaller, attempt.
//
// It exists because a large page can be refused by the provider's own time budget (a 504, an
// HTTP 200 with an empty body, or a body cut off mid-JSON) while smaller pages of the same
// read succeed. The walk therefore starts at a modest page size and, on a transient failure,
// halves the size and asks for the SAME cursor again. Every retry is strictly smaller than the
// attempt it replaces, and a failure at the floor ends the walk, so a sustained outage costs
// a fixed, small number of requests and never repeats an identical one.
//
// Read-side only. The package has no notion of a write and must never wrap one: a write whose
// outcome is unknown is reconciled, not repeated.
//
// Three states, never two: a walk returns either every item up to its cap (with Truncated
// saying whether the provider holds more), or an error and NO items. A partial page set is
// never handed back as a shorter answer.
package pager

import (
	"context"
	"errors"
	"fmt"
)

// Request is one page request: the cursor to continue from ("" is the start of the
// connection) and how many items to ask for.
type Request struct {
	Cursor string
	Size   int
}

// Page is one page answer: its items, the provider's more-items signal and the cursor that
// continues after the last item.
type Page[T any] struct {
	Items []T
	More  bool
	Next  string
}

// Fetch sends ONE page request and returns the decoded page or the request's error. It must not
// retry on its own; the walk decides whether a failure earns another attempt.
type Fetch[T any] func(ctx context.Context, req Request) (Page[T], error)

// Class is a failure's retry class.
type Class int

const (
	// Stop ends the walk with the error unchanged: permission, scope, authentication, rate
	// limits, a malformed request and anything the caller does not positively know to be
	// transient.
	Stop Class = iota
	// Transient means the provider failed to answer THIS page in time or in full, and a
	// smaller page of the same cursor may succeed.
	Transient
)

// Policy bounds a walk. The zero value of an optional field takes its documented default.
type Policy struct {
	// PageSize is the first page size asked for. Default DefaultPageSize.
	PageSize int
	// MinPageSize is the floor: a transient failure at or below it ends the walk. Default
	// DefaultMinPageSize, and never above PageSize.
	MinPageSize int
	// MaxItems caps the walk. Zero means no item cap; MaxRequests still bounds it.
	MaxItems int
	// MaxRetries bounds the transient retries of the WHOLE walk, across all cursors. Default
	// DefaultMaxRetries. Halving already bounds the retries of one cursor; this bounds a
	// walk whose every page needs one.
	MaxRetries int
	// MaxRequests is the hard request ceiling for the walk, retries included. Zero means
	// derived: enough pages to reach MaxItems at the floor size, plus MaxRetries. A walk with
	// neither MaxItems nor MaxRequests is refused, because it would have no bound at all.
	MaxRequests int
	// Classify decides a failed Fetch. Nil classifies every failure as Stop.
	Classify func(error) Class
}

const (
	DefaultPageSize    = 25
	DefaultMinPageSize = 6
	DefaultMaxRetries  = 4
)

// Result is a completed walk.
type Result[T any] struct {
	// Items are the walk's items in provider order, de-duplicated by the walk's key when one
	// was given, at most MaxItems of them.
	Items []T
	// Truncated is true when the walk stopped at MaxItems and the provider holds more (it
	// said so, or a page carried items past the cap).
	Truncated bool
	Stats     Stats
}

// Stats is the walk's request accounting. It is filled on success and on failure.
type Stats struct {
	// Requests is every Fetch call made, retries included.
	Requests int
	// Retries is how many of those were a smaller retry of a cursor that failed.
	Retries int
	// Pages is how many pages were answered successfully.
	Pages int
	// PageSize is the size the walk ended at (the reduced size after any backoff).
	PageSize int
}

// ExhaustedError is returned when a transient failure could not be retried: the page size was
// already at the floor, or the walk's retry or request budget was spent. Cause is the last
// transient failure, reachable through errors.As / errors.Is.
type ExhaustedError struct {
	Cursor string
	Size   int
	Stats  Stats
	Reason string
	Cause  error
}

func (e *ExhaustedError) Error() string {
	return fmt.Sprintf("page read still failing after %d request(s), %d at a smaller page (last size %d; %s): %v",
		e.Stats.Requests, e.Stats.Retries, e.Size, e.Reason, e.Cause)
}

func (e *ExhaustedError) Unwrap() error { return e.Cause }

// StalledError is returned when the provider says more items exist but hands back no cursor,
// or the cursor it was just asked for: following it would repeat the same page forever.
type StalledError struct {
	Cursor string
	Stats  Stats
}

func (e *StalledError) Error() string {
	return fmt.Sprintf("provider reports more items but its cursor does not advance (after %q, %d request(s)) — "+
		"refusing to repeat the same page or hand back a partial set", e.Cursor, e.Stats.Requests)
}

// BudgetError is returned when the walk's MaxRequests is reached while the provider still
// reports more items below MaxItems.
type BudgetError struct {
	Stats Stats
}

func (e *BudgetError) Error() string {
	return fmt.Sprintf("page walk reached its request ceiling (%d request(s)) with more items still reported — "+
		"refusing to hand back a partial set", e.Stats.Requests)
}

// ErrUnbounded is returned for a Policy with neither MaxItems nor MaxRequests.
var ErrUnbounded = errors.New("pager: a walk needs MaxItems or MaxRequests; an unbounded walk is refused")

func (p Policy) normalized() (Policy, error) {
	if p.MaxItems < 0 || p.MaxRequests < 0 || p.PageSize < 0 || p.MinPageSize < 0 || p.MaxRetries < 0 {
		return p, errors.New("pager: negative policy bound")
	}
	if p.MaxItems == 0 && p.MaxRequests == 0 {
		return p, ErrUnbounded
	}
	if p.PageSize == 0 {
		p.PageSize = DefaultPageSize
	}
	if p.MinPageSize == 0 {
		p.MinPageSize = DefaultMinPageSize
	}
	if p.MinPageSize > p.PageSize {
		p.MinPageSize = p.PageSize
	}
	if p.MaxRetries == 0 {
		p.MaxRetries = DefaultMaxRetries
	}
	if p.MaxRequests == 0 {
		p.MaxRequests = (p.MaxItems+p.MinPageSize-1)/p.MinPageSize + p.MaxRetries
	}
	if p.Classify == nil {
		p.Classify = func(error) Class { return Stop }
	}
	return p, nil
}

// Walk reads pages through fetch until the provider reports no more, MaxItems is reached, or a
// failure ends it. key, when non-nil, de-duplicates items across pages (first occurrence wins):
// a connection that shifts while it is walked can repeat an item at a page boundary.
//
// A Stop failure is returned unchanged, so a caller's own classification of it (not found,
// forbidden, rate limited) is untouched. A transient failure is retried at half the page size,
// floored at MinPageSize, for the SAME cursor; the reduced size is kept for the rest of the
// walk. When a transient failure cannot be retried the walk returns an *ExhaustedError
// wrapping it. On any error no items are returned.
func Walk[T any](ctx context.Context, policy Policy, key func(T) string, fetch Fetch[T]) (Result[T], error) {
	p, err := policy.normalized()
	if err != nil {
		return Result[T]{}, err
	}
	var (
		items  []T
		seen   map[string]bool
		stats  = Stats{PageSize: p.PageSize}
		cursor string
	)
	if key != nil {
		seen = map[string]bool{}
	}
	size := p.PageSize
	for {
		want := size
		if p.MaxItems > 0 && p.MaxItems-len(items) < want {
			want = p.MaxItems - len(items)
		}
		if stats.Requests >= p.MaxRequests {
			return Result[T]{Stats: stats}, &BudgetError{Stats: stats}
		}
		if err := ctx.Err(); err != nil {
			return Result[T]{Stats: stats}, err
		}
		stats.Requests++
		page, ferr := fetch(ctx, Request{Cursor: cursor, Size: want})
		if ferr != nil {
			if p.Classify(ferr) != Transient {
				return Result[T]{Stats: stats}, ferr
			}
			reason := ""
			switch {
			case want <= p.MinPageSize:
				reason = fmt.Sprintf("page size at its floor of %d", p.MinPageSize)
			case stats.Retries >= p.MaxRetries:
				reason = fmt.Sprintf("retry budget of %d spent", p.MaxRetries)
			case stats.Requests >= p.MaxRequests:
				reason = fmt.Sprintf("request ceiling of %d reached", p.MaxRequests)
			}
			if reason != "" {
				return Result[T]{Stats: stats}, &ExhaustedError{Cursor: cursor, Size: want, Stats: stats, Reason: reason, Cause: ferr}
			}
			size = max(want/2, p.MinPageSize)
			stats.PageSize = size
			stats.Retries++
			continue
		}
		stats.Pages++
		for _, it := range page.Items {
			if seen != nil {
				k := key(it)
				if seen[k] {
					continue
				}
				seen[k] = true
			}
			if p.MaxItems > 0 && len(items) >= p.MaxItems {
				// The page carried more than the cap asked for: the provider holds more.
				return Result[T]{Items: items, Truncated: true, Stats: stats}, nil
			}
			items = append(items, it)
		}
		if !page.More {
			return Result[T]{Items: items, Stats: stats}, nil
		}
		if p.MaxItems > 0 && len(items) >= p.MaxItems {
			return Result[T]{Items: items, Truncated: true, Stats: stats}, nil
		}
		if page.Next == "" || page.Next == cursor {
			return Result[T]{Stats: stats}, &StalledError{Cursor: cursor, Stats: stats}
		}
		cursor = page.Next
	}
}
