package drainloop

import (
	"testing"
	"time"
)

func passResult() Result  { return Result{Verdict: VerdictPass} }
func failResult() Result  { return Result{Verdict: VerdictFail} }
func errorResult() Result { return Result{Verdict: VerdictError} }

// A non-failing result is never retried.
func TestRetryTerminalOnSuccess(t *testing.T) {
	p := RetryPolicy{Max: 5, Classify: func(Result) Class { return ClassTransient }}
	if got := p.Decide(passResult(), 0); got != DecisionTerminal {
		t.Fatalf("a passing result must be terminal, got %v", got)
	}
}

// A transient failure with attempts remaining is retried; once exhausted it routes to a human
// (never silently dropped).
func TestRetryTransientRetriesThenRoutes(t *testing.T) {
	p := RetryPolicy{Max: 3, Classify: func(Result) Class { return ClassTransient }}
	if got := p.Decide(failResult(), 0); got != DecisionRetry {
		t.Fatalf("attempt 0 of 3 must retry, got %v", got)
	}
	if got := p.Decide(failResult(), 1); got != DecisionRetry {
		t.Fatalf("attempt 1 of 3 must retry, got %v", got)
	}
	if got := p.Decide(failResult(), 2); got != DecisionRoute {
		t.Fatalf("attempt 2 of 3 exhausts — must route to a human, got %v", got)
	}
}

// A deterministic failure is terminal, never retried.
func TestRetryDeterministicIsTerminal(t *testing.T) {
	p := RetryPolicy{Max: 5, Classify: func(Result) Class { return ClassDeterministic }}
	if got := p.Decide(failResult(), 0); got != DecisionTerminal {
		t.Fatalf("a deterministic failure must be terminal, got %v", got)
	}
}

// The third state: an unclassifiable failure routes to a human, never collapses into
// retry-forever or a silent drop. A nil Classifier means everything is unknown.
func TestRetryUnknownRoutesToHuman(t *testing.T) {
	p := RetryPolicy{Max: 5} // nil Classifier ⇒ ClassUnknown
	if got := p.Decide(failResult(), 0); got != DecisionRoute {
		t.Fatalf("an unclassifiable failure must route to a human, got %v", got)
	}
	if got := p.Decide(errorResult(), 0); got != DecisionRoute {
		t.Fatalf("an unclassifiable dispatch error must route to a human, got %v", got)
	}
}

// Exponential backoff doubles from base and honours a cap.
func TestExponentialBackoff(t *testing.T) {
	b := ExponentialBackoff(100*time.Millisecond, 1*time.Second)
	want := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1 * time.Second, // capped
		1 * time.Second, // stays capped
	}
	for i, w := range want {
		if got := b(i); got != w {
			t.Fatalf("backoff attempt %d: got %v want %v", i, got, w)
		}
	}
	// Uncapped doubles without bound.
	u := ExponentialBackoff(1*time.Second, 0)
	if got := u(3); got != 8*time.Second {
		t.Fatalf("uncapped backoff attempt 3: got %v want 8s", got)
	}
	// A nil Backoff waits zero.
	if got := (RetryPolicy{}).Wait(4); got != 0 {
		t.Fatalf("nil Backoff must wait 0, got %v", got)
	}
}
