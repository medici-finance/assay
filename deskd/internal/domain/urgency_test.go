package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// TestUrgencyJSONRoundTrips pins the fix the service contract needs to serve
// domain.Escalation over the v1 contract seam: urgency marshals to a word and
// MUST decode back to the same value. A custom marshaller without its matching
// unmarshaller is a one-way lie (render.go says exactly this of the Escalation
// join) — before the Urgency.UnmarshalJSON fix, decoding "routine"/"gold" failed
// with `cannot unmarshal string into domain.Urgency`, so an InboxResponse with
// any real item was undecodable by the client.
func TestUrgencyJSONRoundTrips(t *testing.T) {
	for _, u := range []domain.Urgency{
		domain.UrgencyRoutine, domain.UrgencyElevated, domain.UrgencyUrgent, domain.UrgencyGold,
	} {
		b, err := json.Marshal(u)
		if err != nil {
			t.Fatalf("marshal %v: %v", u, err)
		}
		var got domain.Urgency
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("unmarshal %s: %v", b, err)
		}
		if got != u {
			t.Errorf("round-trip: %v -> %s -> %v", u, b, got)
		}
	}
}

// TestUrgencyRoundTripsInsideEscalation is the end-to-end reason the fix exists:
// a domain.Escalation with a set urgency must survive a JSON round-trip, because
// that is exactly what /v1/inbox -> contract.Client.Inbox does.
func TestUrgencyRoundTripsInsideEscalation(t *testing.T) {
	e := domain.Escalation{ID: "IT1", Cell: "assay", Urgency: domain.UrgencyGold, Title: "t"}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal escalation: %v", err)
	}
	var got domain.Escalation
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal escalation: %v", err)
	}
	if got.Urgency != domain.UrgencyGold {
		t.Errorf("urgency did not survive the round-trip: got %v", got.Urgency)
	}
}

// TestUrgencyUnmarshalRejectsUnknown — an unknown word is an error, never a
// silent fallback to routine, so a garbled urgency is visible not downgraded.
func TestUrgencyUnmarshalRejectsUnknown(t *testing.T) {
	var u domain.Urgency
	if err := json.Unmarshal([]byte(`"catastrophic"`), &u); err == nil {
		t.Fatal("an unknown urgency word decoded without error — a garbled value would be silently accepted")
	}
}
