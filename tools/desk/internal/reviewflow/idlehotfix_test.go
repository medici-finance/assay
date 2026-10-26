package reviewflow

import (
	"fmt"
	"testing"
	"time"
)

// regression: #4405
// TestIdleHotfixNotStale: a board whose desk-tools are a valid local hotfix reports
// staleState `hotfix` with stale=false; the idle gate reads it as a fresh board (not BLIND),
// while a hotfix that outlived its pin arrives as drift (stale=true) and stays BLIND.
func TestIdleHotfixNotStale(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	board := func(stale bool, state, detail string) []byte {
		return []byte(fmt.Sprintf(`{"asOf":%q,"stale":%t,"staleState":%q,"staleDetail":%q,`+
			`"scope":{"repos":["o/r"],"count":1,"source":"roster"},"prPopulation":{"complete":true,"cap":200},"rows":[]}`,
			now.Format(time.RFC3339), stale, state, detail))
	}
	b, err := ReadBoard(board(false, "hotfix", "HOTFIX: desk-tools v1.0.41+hotfix-0123456789ab over pin v1.0.41 (expires 2026-10-27T12:00:00Z)"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if v := Idle(b, now); v.State != IdleYes {
		t.Fatalf("valid hotfix board: idle = %v (blind %v), want IdleYes", v.State, v.BlindReasons)
	}
	b, err = ReadBoard(board(true, "drift", "desk-tools hotfix v1.0.41+hotfix-0123456789ab outlived its pin"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if v := Idle(b, now); v.State != IdleCouldNotCheck {
		t.Fatalf("outlived hotfix board: idle = %v, want IdleCouldNotCheck", v.State)
	}
}
