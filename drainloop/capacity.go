package drainloop

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// RoleCapacity is ONE desk's approximate estimate of how much work its role is
// running across the cell. It is a pacing gauge for the next bounded refill, not
// an admission budget, a lock or an ownership boundary.
//
// What it is not, on purpose:
//   - no cross-desk mutex, semaphore, leased slot or count-and-reserve step: every
//     desk builds its own RoleCapacity from shared observations, and two desks that
//     read the same gap may both fill it with different items;
//   - no authority: per-item claims (Claimer), claim generations, effect identity
//     and the hard brakes in budget.go keep their own contracts and never consult
//     this estimate;
//   - no fatal refresh: a failed or missing observation never blocks unrelated,
//     otherwise authorized work.
//
// The mutex below guards this desk's own map against its own refresh and refill
// goroutines. It is never shared between desks.
type RoleCapacity struct {
	cfg        CapacityConfig
	mu         sync.Mutex
	obs        map[string]CapacityObservation
	refreshErr error
}

// CapacityConfig scopes one estimator. Cell and Role bind it to one cell and one
// role; an observation for any other cell or role is refused, never counted.
type CapacityConfig struct {
	Cell string
	Role string
	// Instance is this desk's own instance id. It only splits Own from the role
	// total in the estimate; the target is never partitioned by instance.
	Instance string
	// Target is the configured role work level. It is the role-wide refill
	// target, so adding an instance does not multiply it.
	Target int
	// MaxAge is how long one observation stays usable. An older one ages out, so
	// a stale overestimate cannot starve the role permanently. That holds because
	// Observe refuses a reading dated more than MaxSkew ahead of Now: a reading
	// stamped in the future would otherwise stay fresh past MaxAge and outrank
	// every later correction from its source.
	MaxAge time.Duration
	// MaxSkew is how far ahead of Now an observation may be dated, to absorb
	// ordinary clock skew between hosts. Zero means DefaultCapacitySkew.
	MaxSkew time.Duration
	// Burst bounds one refill decision (per call, not per period of unknown
	// data). Zero means DefaultRefillBurst; it may not exceed Target.
	Burst int
	// Now is a test seam. nil uses time.Now.
	Now func() time.Time
}

// DefaultRefillBurst is the per-decision refill bound when CapacityConfig.Burst
// is zero. The estimate is re-read before every decision, so a small bound
// converges without a reservation.
const DefaultRefillBurst = 1

// DefaultCapacitySkew is the future-dating tolerance when CapacityConfig.MaxSkew
// is zero.
const DefaultCapacitySkew = 5 * time.Second

// CapacityObservation is one source's latest reading of active role work. Source
// is the stable observer key (a claim summary, a heartbeat, a supervisor process
// fact); a newer reading from the same Source replaces the older one in full.
// Instance and Generation name the desk instance and execution generation the
// reading describes. Draining marks work still running on a paused or draining
// predecessor; it counts toward the role total because the work is real.
type CapacityObservation struct {
	Source     string
	Cell       string
	Role       string
	Instance   string
	Generation string
	Active     int
	Draining   bool
	ObservedAt time.Time
}

// CapacityObserver is the adapter that reads shared observations. Its error is
// could-not-check, never a zero reading.
type CapacityObserver interface {
	ObserveCapacity(ctx context.Context) ([]CapacityObservation, error)
}

// CapacityEstimate is what a desk currently believes. Known is false when no
// fresh observation exists: Active is then 0 because nothing was measured, not
// because the role is idle. Gap is Target-Active and is negative above target.
type CapacityEstimate struct {
	Active   int
	Own      int
	Draining int
	Target   int
	Gap      int
	Fresh    int
	Stale    int
	Known    bool
	// OldestAge is the age of the oldest fresh observation.
	OldestAge time.Duration
	// RefreshErr is the last refresh failure, shown so a stale picture is visible.
	RefreshErr string
}

// NewRoleCapacity validates the scope and returns an empty estimator.
func NewRoleCapacity(cfg CapacityConfig) (*RoleCapacity, error) {
	switch {
	case cfg.Cell == "" || cfg.Role == "":
		return nil, errors.New("drainloop: capacity needs a cell and a role")
	case cfg.Target < 1:
		return nil, fmt.Errorf("drainloop: capacity target must be >= 1, got %d", cfg.Target)
	case cfg.MaxAge <= 0:
		return nil, errors.New("drainloop: capacity MaxAge must be positive")
	case cfg.Burst < 0:
		return nil, errors.New("drainloop: capacity Burst must not be negative")
	case cfg.Burst > cfg.Target:
		return nil, fmt.Errorf("drainloop: capacity Burst %d exceeds Target %d", cfg.Burst, cfg.Target)
	case cfg.MaxSkew < 0:
		return nil, errors.New("drainloop: capacity MaxSkew must not be negative")
	}
	if cfg.Burst == 0 {
		cfg.Burst = DefaultRefillBurst
	}
	if cfg.MaxSkew == 0 {
		cfg.MaxSkew = DefaultCapacitySkew
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &RoleCapacity{cfg: cfg, obs: map[string]CapacityObservation{}}, nil
}

// Observe records one reading. A foreign cell or role, an empty source, a
// negative count, a missing time or a time more than MaxSkew ahead of Now is
// refused and changes nothing. A reading older than the one held for the same
// source is dropped as an out-of-order delivery; a newer one replaces it even
// when it is smaller, which is how an overestimate is corrected. Refusing
// future-dated readings is what keeps that true: an accepted reading is never
// dated past Now+MaxSkew, so any later reading from its source replaces it.
func (c *RoleCapacity) Observe(o CapacityObservation) error {
	switch {
	case o.Source == "":
		return errors.New("drainloop: capacity observation has no source")
	case o.Cell != c.cfg.Cell || o.Role != c.cfg.Role:
		return fmt.Errorf("drainloop: capacity observation %q is for another cell or role", o.Source)
	case o.Active < 0:
		return fmt.Errorf("drainloop: capacity observation %q has a negative count", o.Source)
	case o.ObservedAt.IsZero():
		return fmt.Errorf("drainloop: capacity observation %q has no time", o.Source)
	case o.ObservedAt.After(c.cfg.Now().Add(c.cfg.MaxSkew)):
		return fmt.Errorf("drainloop: capacity observation %q is dated %s, beyond the %s skew tolerance ahead of now", o.Source, o.ObservedAt.Format(time.RFC3339), c.cfg.MaxSkew)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if prev, ok := c.obs[o.Source]; ok && o.ObservedAt.Before(prev.ObservedAt) {
		return nil
	}
	c.obs[o.Source] = o
	return nil
}

// Refresh reads the shared observations and folds them in. Its error is for the
// caller's log: it is retained on the estimate for visibility and never makes
// Refill refuse. When the observer itself fails, nothing is applied: readings
// already held stay usable until they age out. When the observer succeeds,
// each reading is folded in and a refused one is reported in the joined error.
func (c *RoleCapacity) Refresh(ctx context.Context, src CapacityObserver) error {
	obs, err := src.ObserveCapacity(ctx)
	if err != nil {
		c.mu.Lock()
		c.refreshErr = err
		c.mu.Unlock()
		return fmt.Errorf("drainloop: capacity refresh could-not-check (non-fatal): %w", err)
	}
	var bad []error
	for _, o := range obs {
		if e := c.Observe(o); e != nil {
			bad = append(bad, e)
		}
	}
	c.mu.Lock()
	c.refreshErr = errors.Join(bad...)
	c.mu.Unlock()
	return errors.Join(bad...)
}

// Estimate sums every fresh observation for the role, this desk's and every
// other instance's, including draining predecessors.
func (c *RoleCapacity) Estimate() CapacityEstimate {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.cfg.Now()
	est := CapacityEstimate{Target: c.cfg.Target}
	for _, o := range c.obs {
		age := now.Sub(o.ObservedAt)
		if age > c.cfg.MaxAge {
			est.Stale++
			continue
		}
		est.Fresh++
		est.Active = satAdd(est.Active, o.Active)
		if o.Instance == c.cfg.Instance {
			est.Own = satAdd(est.Own, o.Active)
		}
		if o.Draining {
			est.Draining = satAdd(est.Draining, o.Active)
		}
		if age > est.OldestAge {
			est.OldestAge = age
		}
	}
	est.Known = est.Fresh > 0
	est.Gap = est.Target - est.Active
	if c.refreshErr != nil {
		est.RefreshErr = c.refreshErr.Error()
	}
	return est
}

// Refill returns how many more items this desk may start now: 0 at or above
// target, otherwise the gap capped by the burst bound. With no fresh observation
// it returns one burst, since missing data is not a measured zero and must not
// refill the whole target in one decision, nor hold the role. The burst bound is
// per call: repeated calls under prolonged unknown data each return a burst, so
// only a positive instanceTarget (and the budget brakes) bound a desk's total
// then, and callers should always pass one. A positive instanceTarget caps the
// desk to instanceTarget-instanceActive; it shapes this desk's refill and does
// not partition the role target.
func (c *RoleCapacity) Refill(instanceActive, instanceTarget int) int {
	est := c.Estimate()
	n := c.cfg.Burst
	switch {
	case !est.Known:
	case est.Gap <= 0:
		return 0
	case est.Gap < n:
		n = est.Gap
	}
	if instanceTarget > 0 {
		if room := instanceTarget - instanceActive; room < n {
			n = room
		}
	}
	if n < 0 {
		n = 0
	}
	return n
}

// satAdd adds two non-negative counts, saturating at math.MaxInt so a huge
// reading reads as "far above target" instead of wrapping negative.
func satAdd(a, b int) int {
	if b > math.MaxInt-a {
		return math.MaxInt
	}
	return a + b
}
