package deskkit

import (
	"context"
	"runtime"
	"strings"

	"github.com/medici-finance/assay/perfrec"
)

type perfContextKey struct{}

// WithPerfContext carries immutable diagnostics beside an explicit call, never
// in package-global work state or operational authority.
func WithPerfContext(ctx context.Context, c perfrec.Context) context.Context {
	return context.WithValue(ctx, perfContextKey{}, c)
}
func PerfContext(ctx context.Context) (perfrec.Context, bool) {
	c, ok := ctx.Value(perfContextKey{}).(perfrec.Context)
	return c, ok
}
func PerfContextEnv(env []string, c perfrec.Context, toolChild bool) []string {
	return perfrec.ContextEnv(env, c, toolChild)
}
func PerfAgentContext(parent perfrec.Context, role string, work []perfrec.WorkRef, resume *perfrec.Attribution) perfrec.Context {
	return perfrec.ChildContext(parent, role, work, resume)
}

// An untyped boundary cannot identify its own spawning invocation. Keep the
// selected profile and agent linkage, but never forward a grandparent as parent.
func unknownPerfParentEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, pair := range env {
		key, _, _ := strings.Cut(pair, "=")
		if key == "ASSAY_PERF_PARENT_INVOCATION_ID" || runtime.GOOS == "windows" && strings.EqualFold(key, "ASSAY_PERF_PARENT_INVOCATION_ID") {
			continue
		}
		out = append(out, pair)
	}
	return out
}
