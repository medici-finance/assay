package pgtest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fixtureDiagnosticDriverError struct{ state, detail string }

func (e *fixtureDiagnosticDriverError) Error() string    { return e.detail }
func (e *fixtureDiagnosticDriverError) SQLState() string { return e.state }

// regression: F-fixture-cleanup-safe-diagnostics
func TestFixtureCleanupDiagnostics(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()
	for _, tc := range []struct {
		name, state, wantState, wantContext string
		ctx                                 context.Context
		sentinel                            error
	}{
		{"server-code-only", "57014", "57014", "none", context.Background(), nil},
		{"canceled", "57014", "57014", "canceled", canceled, context.Canceled},
		{"deadline", "57014", "57014", "deadline-exceeded", expired, context.DeadlineExceeded},
		{"invalid-code", "57014 driver-detail", "unknown", "none", context.Background(), nil},
		{"lowercase-code", "xx000", "unknown", "none", context.Background(), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := &fixtureDiagnosticDriverError{state: tc.state, detail: "redacted-driver-detail"}
			got := cleanupFailure(tc.ctx, raw)
			var state interface{ SQLState() string }
			if !errors.As(got, &state) || state.SQLState() != tc.wantState || !strings.Contains(got.Error(), "context="+tc.wantContext+")") {
				t.Fatal("safe cleanup classification unavailable")
			}
			if strings.Contains(got.Error(), raw.detail) || errors.Is(got, raw) {
				t.Fatal("raw driver detail retained in cleanup error or chain")
			}
			if tc.sentinel != nil && !errors.Is(got, tc.sentinel) {
				t.Fatal("safe cancellation sentinel unavailable")
			}
		})
	}
	t.Run("wrapped-cancellation", func(t *testing.T) {
		got := cleanupFailure(context.Background(), errors.Join(context.DeadlineExceeded, errors.New("redacted-driver-detail")))
		if !errors.Is(got, context.DeadlineExceeded) || strings.Contains(got.Error(), "redacted-driver-detail") {
			t.Fatal("wrapped cancellation lost or raw detail escaped")
		}
	})
}
