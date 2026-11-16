package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6695
// Two --stamp-only runs naming one launched model by its full id and by its family alias
// flipped the authored stamp on every run, each removing the other's label as stale. The
// alias run must now leave the standing full id alone and write nothing.
func TestStampOnlyAliasNoFlip(t *testing.T) {
	s := &stub{}
	s.install(t)
	t.Setenv("DESK_LOOP", "the-desk")
	gh := installGHStamp(t)
	if rc := run(onlyArgs("claude-opus-5-5")); rc != 0 {
		t.Fatalf("full-id stamp rc=%d", rc)
	}
	for i, model := range []string{"opus", "claude-opus-5-5", "opus"} {
		n := len(gh.requests)
		out := captureStdout(t, func() {
			if rc := run(onlyArgs(model)); rc != 0 {
				t.Fatalf("run %d (%s) rc=%d", i, model, rc)
			}
		})
		// The receipt must name the pair that stands, not a removal that never happened.
		if strings.Contains(out, "RE-STAMPED") || !strings.Contains(out, "authored-model:claude-opus-5-5") {
			t.Fatalf("run %d (%s) receipt does not name the standing stamp: %s", i, model, out)
		}
		for _, r := range gh.requests[n:] {
			if r.Method != "GET" {
				t.Fatalf("run %d (%s) wrote to the change: %s %s %s", i, model, r.Method, r.Path, r.Body)
			}
		}
	}
	got, st := deskkit.StampAuthored.AttestedStampOf(deskkit.StampTimeline{Present: gh.labels, Events: ghEvents(gh)}, deskkit.IsStampAuthorityLogin)
	if st != deskkit.ModelStamped || got.Model != "claude-opus-5-5" {
		t.Fatalf("authored stamp = %+v %v, want claude-opus-5-5", got, st)
	}
}

// regression: #6695
// The other order settles on the full id too: a standing alias is replaced, once.
func TestStampOnlyFullIDWins(t *testing.T) {
	s := &stub{}
	s.install(t)
	t.Setenv("DESK_LOOP", "the-desk")
	gh := installGHStamp(t)
	for _, model := range []string{"opus", "claude-opus-5-5"} {
		if rc := run(onlyArgs(model)); rc != 0 {
			t.Fatalf("%s rc=%d", model, rc)
		}
	}
	n := len(gh.requests)
	if rc := run(onlyArgs("opus")); rc != 0 {
		t.Fatalf("alias rc=%d", rc)
	}
	for _, r := range gh.requests[n:] {
		if r.Method != "GET" {
			t.Fatalf("alias after full id wrote: %s %s", r.Method, r.Path)
		}
	}
	got, st := deskkit.StampAuthored.AttestedStampOf(deskkit.StampTimeline{Present: gh.labels, Events: ghEvents(gh)}, deskkit.IsStampAuthorityLogin)
	if st != deskkit.ModelStamped || got.Model != "claude-opus-5-5" {
		t.Fatalf("authored stamp = %+v %v", got, st)
	}
}
