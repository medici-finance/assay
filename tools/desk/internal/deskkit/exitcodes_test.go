package deskkit

import (
	"errors"
	"testing"
)

func TestExitCodeOf(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil is success", nil, ExitOK},
		{"disabled", Disabled("x"), ExitDisabled},
		{"rate limited", RateLimited("x"), ExitRateLimited},
		{"refused", Refused("x"), ExitRefused},
		{"unverifiable", Unverifiable("x", nil), ExitUnverifiable},
		// Fail closed: an unexpected non-DeskError must map to 6, NEVER 0.
		{"unknown error fails closed to 6", errors.New("boom"), ExitUnverifiable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ExitCodeOf(c.err); got != c.want {
				t.Fatalf("ExitCodeOf(%v) = %d, want %d", c.err, got, c.want)
			}
		})
	}
}

func TestDeskErrorPredicates(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		disable bool
		rate    bool
		refuse  bool
		unver   bool
	}{
		{"disabled", Disabled("x"), true, false, false, false},
		{"rate", RateLimited("x"), false, true, false, false},
		{"refused", Refused("x"), false, false, true, false},
		{"unverifiable", Unverifiable("x", nil), false, false, false, true},
		{"plain error matches none", errors.New("x"), false, false, false, false},
		{"nil matches none", nil, false, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if IsDisabled(c.err) != c.disable {
				t.Errorf("IsDisabled=%v want %v", IsDisabled(c.err), c.disable)
			}
			if IsRateLimited(c.err) != c.rate {
				t.Errorf("IsRateLimited=%v want %v", IsRateLimited(c.err), c.rate)
			}
			if IsRefused(c.err) != c.refuse {
				t.Errorf("IsRefused=%v want %v", IsRefused(c.err), c.refuse)
			}
			if IsUnverifiable(c.err) != c.unver {
				t.Errorf("IsUnverifiable=%v want %v", IsUnverifiable(c.err), c.unver)
			}
		})
	}
}

func TestDeskErrorUnwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := Unverifiable("wrapper", cause)
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is did not find the wrapped cause")
	}
	if err.ExitCode() != ExitUnverifiable {
		t.Fatalf("ExitCode() = %d, want %d", err.ExitCode(), ExitUnverifiable)
	}
}

// TestExitCodeTableMatchesDerivation pins the exit-code table to the convention
// recorded in the `Derivation:` doc block above the const block in exitcodes.go:
// 0 is success; 1 and 2 stay the shell's own general-error / builtin-misuse
// codes and are never used by a desk tool; the four refusal classes are exactly
// 3..6 — contiguous, distinct, and in handling order (disabled, rate-limited,
// constraint-refused, unverifiable), one code per CLASS, not one per message.
// Any value that drifts from that convention fails HERE, in the same file as
// the table, instead of in some caller's branch on the wrong integer.
func TestExitCodeTableMatchesDerivation(t *testing.T) {
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"ExitOK", ExitOK, 0},
		{"ExitDisabled", ExitDisabled, 3},
		{"ExitRateLimited", ExitRateLimited, 4},
		{"ExitRefused", ExitRefused, 5},
		{"ExitUnverifiable", ExitUnverifiable, 6},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d (the derivation pins each refusal class to its own code in 3..6)", c.name, c.got, c.want)
		}
	}

	// 1 and 2 are the shell's own general-error and builtin-misuse codes; no
	// desk-tool code may take either, or a caller cannot tell a tool's refusal
	// from the shell's own error.
	for _, code := range []int{ExitDisabled, ExitRateLimited, ExitRefused, ExitUnverifiable} {
		if code == 1 || code == 2 {
			t.Errorf("refusal code %d collides with the shell's own general-error/misuse codes 1/2", code)
		}
	}

	// The refusal classes are 3..6 contiguous — one code per class, none
	// skipped, none doubled (a swap of two values collapses the set). Built at
	// runtime rather than as a literal so a doubling is an assertion failure
	// naming both classes, not a bare compile error.
	classes := map[int]string{}
	for _, c := range []struct {
		code int
		name string
	}{
		{ExitDisabled, "disabled"},
		{ExitRateLimited, "rate-limited"},
		{ExitRefused, "constraint-refused"},
		{ExitUnverifiable, "unverifiable"},
	} {
		if prev, dup := classes[c.code]; dup {
			t.Errorf("code %d carries two refusal classes (%s and %s) — one code per class", c.code, prev, c.name)
		}
		classes[c.code] = c.name
	}
	if len(classes) != 4 {
		t.Errorf("the four refusal classes are not distinct codes: %v", classes)
	}
	for code := 3; code <= 6; code++ {
		if _, ok := classes[code]; !ok {
			t.Errorf("code %d carries no refusal class — the classes must be contiguous 3..6", code)
		}
	}
}
