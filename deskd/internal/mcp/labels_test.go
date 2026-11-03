package mcp

import (
	"testing"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// TestEscalationLabelsSingleOwner binds the filer to the adapter's label table:
// every kind the filer can file round-trips through the SAME table the poller
// parses with, and both argument spellings resolve. The topology drift registry
// cites this test as the reason the adapter table is the module's only copy.
func TestEscalationLabelsSingleOwner(t *testing.T) {
	wantWire := map[domain.EscalationKind]string{
		domain.KindNeedsDecision: "needs-decision",
		domain.KindQuestion:      "question",
		domain.KindHelpWanted:    "help wanted",
		domain.KindAsk:           "ask",
	}
	for kind, want := range wantWire {
		got, ok := adapter.WireLabel(kind)
		if !ok || got != want {
			t.Fatalf("WireLabel(%s) = %q,%v; want %q", kind, got, ok, want)
		}
		back, ok := adapter.KindForLabel(got)
		if !ok || back != kind {
			t.Fatalf("KindForLabel(%q) = %q,%v; want %q (the filer's label must parse back)", got, back, ok, kind)
		}
		if labels := escalationLabels(kind, domain.UrgencyRoutine); len(labels) != 1 || labels[0] != want {
			t.Fatalf("escalationLabels(%s) = %v; want [%q]", kind, labels, want)
		}
	}
	for _, arg := range []string{"help-wanted", "Help Wanted", " ask "} {
		if _, ok := adapter.KindForLabel(arg); !ok {
			t.Fatalf("KindForLabel(%q) did not resolve", arg)
		}
	}
	if _, ok := adapter.WireLabel(domain.EscalationKind("not-a-kind")); ok {
		t.Fatalf("WireLabel resolved an unknown kind")
	}
}
