package deskkit

import "testing"

const (
	sbHead      = "aaaaaaaabbbbbbbbccccccccdddddddd11111111"
	sbStaleHead = "1111111122222222333333334444444455555555"
)

// The predicate's whole table (#1985): only a blocking entry stands; of those, only the exact
// state `resolved` with evidence at the head (or no evidence head, which is the record's own
// head) retires. A stale evidence head, and any state that is not exactly `resolved`, stands.
func TestStandingBlockerAt(t *testing.T) {
	cases := []struct {
		name string
		f    Finding
		want bool
	}{
		{"advisory never stands", Finding{Severity: SeverityAdvisory, State: StateOpen}, false},
		{"open stands", Finding{Severity: SeverityBlocking, State: StateOpen}, true},
		{"fixed-awaiting-review stands", Finding{Severity: SeverityBlocking, State: StateFixedAwaitingReview}, true},
		{"disputed stands", Finding{Severity: SeverityBlocking, State: StateDisputed}, true},
		{"awaiting-arbitration stands", Finding{Severity: SeverityBlocking, State: StateAwaitingArbitration}, true},
		{"empty state stands", Finding{Severity: SeverityBlocking, State: ""}, true},
		{"mis-cased resolved stands", Finding{Severity: SeverityBlocking, State: "Resolved"}, true},
		{"padded resolved stands", Finding{Severity: SeverityBlocking, State: " resolved"}, true},
		{"unknown state stands", Finding{Severity: SeverityBlocking, State: "superseded"}, true},
		{"resolved, no evidence head, retires", Finding{Severity: SeverityBlocking, State: StateResolved}, false},
		{"resolved at the head retires", Finding{Severity: SeverityBlocking, State: StateResolved, EvidenceHead: sbHead}, false},
		{"resolved at a stale head stands", Finding{Severity: SeverityBlocking, State: StateResolved, EvidenceHead: sbStaleHead}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.f.StandingBlockerAt(sbHead); got != c.want {
				t.Fatalf("StandingBlockerAt(%s) = %v, want %v for %+v", sbHead[:12], got, c.want, c.f)
			}
		})
	}
}

// The external-prerequisite parser, pinned at the parser itself rather than through the
// ready gate (whose ledger layer would refuse the same fixtures and hide which layer did):
// a standing content entry sets HasContentBlocker, a standing prerequisite is a declared
// Condition, and a resolved-at-head entry is neither.
func TestParsePrereqDeclaration_StandingStates(t *testing.T) {
	body := func(fs ...Finding) string {
		return "External-Prereq-Only: shared CI repair\n\n" + RenderFindingBlock(FindingBlockV1{Findings: fs})
	}
	content := func(st FindingState, ev string) Finding {
		return Finding{ID: "f-code-1", Class: "nil-deref", Severity: SeverityBlocking, Blocker: BlockerCodeContent,
			State: st, EvidenceHead: ev, Failure: "nil deref"}
	}
	prereq := func(st FindingState, ev string) Finding {
		return Finding{ID: "f-pre-1", Class: "shared-ci", Severity: SeverityBlocking, Blocker: BlockerExternalPrereq,
			State: st, EvidenceHead: ev, Failure: "shared CI red", SharedRepair: "example/repo#1"}
	}
	for _, st := range []FindingState{StateOpen, StateFixedAwaitingReview, StateDisputed, StateAwaitingArbitration, "", "Resolved"} {
		t.Run("standing/"+string(st), func(t *testing.T) {
			d := ParsePrereqDeclaration(body(content(st, ""), prereq(st, "")), sbHead)
			if !d.HasContentBlocker || len(d.Conditions) != 1 {
				t.Fatalf("state %q: HasContentBlocker=%v Conditions=%d, want true and 1", st, d.HasContentBlocker, len(d.Conditions))
			}
		})
	}
	t.Run("resolved at the head", func(t *testing.T) {
		d := ParsePrereqDeclaration(body(content(StateResolved, sbHead), prereq(StateResolved, "")), sbHead)
		if d.HasContentBlocker || len(d.Conditions) != 0 {
			t.Fatalf("HasContentBlocker=%v Conditions=%d, want false and 0", d.HasContentBlocker, len(d.Conditions))
		}
	})
	t.Run("resolved at a stale head", func(t *testing.T) {
		d := ParsePrereqDeclaration(body(content(StateResolved, sbStaleHead), prereq(StateResolved, sbStaleHead)), sbHead)
		if !d.HasContentBlocker || len(d.Conditions) != 1 {
			t.Fatalf("HasContentBlocker=%v Conditions=%d, want true and 1", d.HasContentBlocker, len(d.Conditions))
		}
	})
}
