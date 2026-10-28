package runner

import (
	"context"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

type performanceKey struct{}
type performanceInput struct {
	parent perfrec.Context
	role   string
	work   []perfrec.WorkRef
}

// PerformanceIdentity is local optional diagnostic context. ContinuityGap is
// explicit when a requested native resume has no retained opaque mapping.
type PerformanceIdentity struct {
	Context       perfrec.Context
	ContinuityGap bool
}

// WithPerformance selects diagnostic input for one Start. It neither modifies
// LaunchRequest nor adds a protocol extension, capability or authority claim.
func WithPerformance(ctx context.Context, parent perfrec.Context, role string, work []perfrec.WorkRef) context.Context {
	return context.WithValue(ctx, performanceKey{}, performanceInput{parent, role, append([]perfrec.WorkRef(nil), work...)})
}

// PerformanceContext lets the existing adapter carry the selected opaque
// identity through its own admitted local launch seam. Native IDs stay private.
func PerformanceContext(ctx context.Context) (PerformanceIdentity, bool) {
	c, ok := ctx.Value(performanceKey{}).(PerformanceIdentity)
	return c, ok
}

type performanceMap struct {
	attribution perfrec.Attribution
	day         time.Time
}

func performanceDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (c *Client) performanceStart(ctx context.Context, req LaunchRequest) (context.Context, *PerformanceIdentity) {
	input, ok := ctx.Value(performanceKey{}).(performanceInput)
	if !ok {
		return ctx, nil
	}
	today := performanceDay(time.Now())
	for id, m := range c.performance {
		if m.day.Before(today.AddDate(0, 0, -6)) || m.day.After(today) {
			delete(c.performance, id)
		}
	}
	var resume *perfrec.Attribution
	gap := false
	if req.Resume != nil {
		if m, exists := c.performance[req.Resume.SessionID]; exists {
			d := m.attribution
			resume = &d
		} else {
			gap = true
		}
	}
	identity := PerformanceIdentity{Context: perfrec.ChildContext(input.parent, input.role, input.work, resume), ContinuityGap: gap}
	return context.WithValue(ctx, performanceKey{}, identity), &identity
}

func (c *Client) performanceRemember(receipt Receipt, identity *PerformanceIdentity) {
	if identity == nil || receipt.SessionID == "" || identity.Context.Profile() != perfrec.ProfileWorkflow {
		return
	}
	c.performance[receipt.SessionID] = performanceMap{identity.Context.Data(), performanceDay(time.Now())}
}
