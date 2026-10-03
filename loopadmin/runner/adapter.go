package runner

import "context"

// Adapter is the narrow API a concrete harness integration implements. It is
// the only thing that ever touches a provider; the contract around it is
// provider-agnostic.
//
// Start must be idempotent on (Caller, ID) where the adapter declares
// LaunchDedupe. It returns an error wrapping ErrDefiniteFailure ONLY when it
// can state the launch did not start; any other error, including a timeout or
// ErrAckLost, is an unknown outcome the client will hold for reconcile.
type Adapter interface {
	Capabilities() Capabilities
	Start(ctx context.Context, req LaunchRequest) (Receipt, error)
	Observe(ctx context.Context, ref Ref) (Observation, error)
	Cancel(ctx context.Context, ref Ref) (CancelAck, error)
	// Reconcile looks an invocation up by its stable identity, without a
	// receipt. It reports StateAbsent when it has no record.
	Reconcile(ctx context.Context, ref Ref) (Observation, error)
}

// Fence reports the current authority generation for a key. It reads the
// caller's own record of authority (a desk claim, a controller's attempt
// generation); the runner contract does not own it.
type Fence interface {
	CurrentGeneration(ctx context.Context, key string) (uint64, error)
}

// FenceFunc adapts a function to Fence.
type FenceFunc func(ctx context.Context, key string) (uint64, error)

// CurrentGeneration implements Fence.
func (f FenceFunc) CurrentGeneration(ctx context.Context, key string) (uint64, error) {
	return f(ctx, key)
}
