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
