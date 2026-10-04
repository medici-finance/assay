package conformance

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/medici-finance/assay/loopadmin/runner"
)

// ErrStaleClaim is the reference caller's own refusal: the claim generation it
// holds is no longer the generation the result ran under.
var ErrStaleClaim = errors.New("conformance: caller's claim generation is stale")

// Claims is a caller's own record of authority generations: what a desk's
// claim store or a controller's attempt table is to the real caller. It is
// independent of the runner contract, which only borrows it as a Fence.
type Claims struct {
	mu   sync.Mutex
	gens map[string]uint64
}

// NewClaims returns an empty record.
func NewClaims() *Claims { return &Claims{gens: map[string]uint64{}} }

// Set records the current generation for a key.
func (c *Claims) Set(key string, gen uint64) {
	c.mu.Lock()
	c.gens[key] = gen
	c.mu.Unlock()
}

// CurrentGeneration implements runner.Fence over the record.
func (c *Claims) CurrentGeneration(_ context.Context, key string) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	g, ok := c.gens[key]
	if !ok {
		return 0, fmt.Errorf("no claim recorded for %q", key)
	}
	return g, nil
}

// StaleFence is the fixture that bypasses the contract's generation fence: it
// reports one fixed generation as current whatever the caller's record says,
// like a fence that was skipped or a faulty adapter that reports a fenced
// attempt as live.
type StaleFence struct{ Generation uint64 }

// CurrentGeneration implements runner.Fence.
func (f StaleFence) CurrentGeneration(context.Context, string) (uint64, error) {
	return f.Generation, nil
}

// ReferenceCaller is the smallest caller that honours the contract's last
// rule: whatever the runner contract accepted, the caller checks its OWN claim
// generation before it commits the result. It is the shape /26's desk binding
// and /21's controller follow.
type ReferenceCaller struct{ Claims *Claims }

// Commit refuses an accepted result whose generation is no longer the one the
// caller's own claim record holds. It fails closed when the record cannot be
// read.
func (c ReferenceCaller) Commit(ctx context.Context, acc runner.Accepted) error {
	cur, err := c.Claims.CurrentGeneration(ctx, acc.Authority.Key)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrStaleClaim, err)
	}
	if cur != acc.Authority.Generation {
		return fmt.Errorf("%w: result ran under %d, claim is %d", ErrStaleClaim, acc.Authority.Generation, cur)
	}
	return nil
}
