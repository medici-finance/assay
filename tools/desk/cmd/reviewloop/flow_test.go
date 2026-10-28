package main

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var actionTable = func() map[string]rule {
	out := map[string]rule{}
	for _, a := range KnownActions() {
		out[a], _ = LookupAction(a)
	}
	return out
}()

// TestBoardReadWireContract pins the command's external error contract across the
// reviewflow extraction: the exact message text, exit code 6, and a cause that errors.As
// can still reach through the chain.
//
// regression: #3570
func TestBoardReadWireContract(t *testing.T) {
	const asOf = `{"asOf":"2026-01-01T00:00:00Z","rows":[]}`
	cases := []struct {
		name     string
		call     func() error
		wantMsg  string // exact text up to the cause; the cause text follows ": " when causeAs is set
		causeAs  func(error) bool
		hasCause bool
	}{
		{
			name: "empty actions",
			call: func() error { _, err := ReadBoard(nil, nil); return err },
			wantMsg: "reviewloop: `deskboard actions` produced no output — BLIND, not idle " +
				"(a rate-limited or refused sweep is empty for the same reason a quiet board is)",
		},
		{
			name:     "undecodable actions",
			call:     func() error { _, err := ReadBoard([]byte("{bad"), nil); return err },
			wantMsg:  "reviewloop: cannot decode `deskboard actions` JSON — BLIND, not idle",
			hasCause: true,
			causeAs:  func(e error) bool { var se *json.SyntaxError; return errors.As(e, &se) },
		},
		{
			name: "missing asOf",
			call: func() error { _, err := ReadBoard([]byte(`{"rows":[]}`), nil); return err },
			wantMsg: "reviewloop: `deskboard actions` payload carries no asOf timestamp — the sweep's own liveness " +
				"heartbeat is missing, so its freshness cannot be checked; BLIND, not idle",
		},
		{
			name:     "unparseable asOf",
			call:     func() error { _, err := ReadBoard([]byte(`{"asOf":"yesterday"}`), nil); return err },
			wantMsg:  "reviewloop: cannot parse the board's asOf timestamp yesterday",
			hasCause: true,
			causeAs:  func(e error) bool { var pe *time.ParseError; return errors.As(e, &pe) },
		},
		{
			name:     "undecodable prs",
			call:     func() error { _, err := ReadBoard([]byte(asOf), []byte("{bad")); return err },
			wantMsg:  "reviewloop: cannot decode `deskboard prs` JSON — the head-SHA half of the join is BLIND",
			hasCause: true,
			causeAs:  func(e error) bool { var se *json.SyntaxError; return errors.As(e, &se) },
		},
		{
			name: "unknown action",
			call: func() error { _, err := LookupAction("NOPE"); return err },
			wantMsg: "reviewloop: deskboard emitted ACTION NOPE which this reactor's action table does not know — " +
				"COULD-NOT-CHECK for that row (add it to actionTable; never treat an unknown action as no-op)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if err == nil {
				t.Fatal("want an error, got nil")
			}
			if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
				t.Errorf("exit code = %d, want %d", got, deskkit.ExitUnverifiable)
			}
			var de *deskkit.DeskError
			if !errors.As(err, &de) {
				t.Fatalf("error %T is not a *deskkit.DeskError", err)
			}
			if de.Msg != tc.wantMsg {
				t.Errorf("message changed:\n got %q\nwant %q", de.Msg, tc.wantMsg)
			}
			if !tc.hasCause {
				if de.Err != nil {
					t.Errorf("unexpected cause %v", de.Err)
				}
				if err.Error() != tc.wantMsg {
					t.Errorf("Error() = %q, want exactly %q", err.Error(), tc.wantMsg)
				}
				return
			}
			if de.Err == nil || !tc.causeAs(err) {
				t.Fatalf("the cause is not reachable through the error chain: %#v", de.Err)
			}
			if want := tc.wantMsg + ": " + de.Err.Error(); err.Error() != want {
				t.Errorf("Error() = %q, want %q", err.Error(), want)
			}
		})
	}
}
