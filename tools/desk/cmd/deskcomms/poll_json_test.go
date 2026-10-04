package main

import (
	"encoding/json"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

func TestPollJSONCarriesHandOffBody(t *testing.T) {
	d, gw, _, out := testDeps(t, "")
	gw.polled = []Notice{{ID: "handoff-1", From: comms.SenderID{Cell: "cell-a", Role: "the-desk"}, Verb: "handoff", Class: "routine", Payload: json.RawMessage(`"request-act\nReview change 42"`)}}
	if _, err := cmdPoll(d, []string{"--json"}); err != nil {
		t.Fatal(err)
	}
	var notices []Notice
	if err := json.Unmarshal(out.Bytes(), &notices); err != nil {
		t.Fatal(err)
	}
	if len(notices) != 1 || string(notices[0].Payload) != string(gw.polled[0].Payload) {
		t.Fatal("handoff body was dropped")
	}
	if len(gw.acked) != 0 {
		t.Fatal("reading must not acknowledge work")
	}
	out.Reset()
	gw.polled = nil
	if _, err := cmdPoll(d, []string{"--json"}); err != nil || out.String() != "[]\n" {
		t.Fatalf("empty mailbox: %q %v", out.String(), err)
	}
}
