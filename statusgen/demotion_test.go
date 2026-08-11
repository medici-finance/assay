package main

import (
	"strings"
	"testing"
)

// Brief-04 (findings demotion, scope-change re-entry): a desk-ACKED unresolved
// finding targeting an in-flight brief is a hard error; an unacked one is a
// non-fatal notice (F-09 DoS mitigation — filing a finding is unverified input).

func demotionFixture(status string) []*Stream {
	return []*Stream{mkStream("ok", "active", "P1", Brief{Num: "07", Wave: 0, Status: status})}
}

func hasEntry(entries []string, want string) bool {
	for _, e := range entries {
		if strings.Contains(e, want) {
			return true
		}
	}
	return false
}

func TestDemotionAckedInProgress(t *testing.T) {
	problems, notices := check(demotionFixture("in-progress"), []Finding{
		{ID: "F-13", Affects: []string{"ok/brief-07"}, Ack: "2026-07-09 desk"},
	})
	if !hasEntry(problems, "ok/brief-07: unresolved F-13 (desk-acked) — demote to todo (re-gate) or resolve the finding") {
		t.Errorf("acked finding on in-progress brief must be a problem, got %v", problems)
	}
	if len(notices) != 0 {
		t.Errorf("acked finding must not also notice: %v", notices)
	}
}

func TestDemotionAckedImplemented(t *testing.T) {
	problems, _ := check(demotionFixture("implemented"), []Finding{
		{ID: "F-13", Affects: []string{"ok/brief-07"}, Ack: "2026-07-09 desk"},
	})
	if !hasEntry(problems, "unresolved F-13 (desk-acked) — demote to todo") {
		t.Errorf("acked finding on implemented brief must be a problem, got %v", problems)
	}
}

func TestDemotionUnackedInProgress(t *testing.T) {
	problems, notices := check(demotionFixture("in-progress"), []Finding{
		{ID: "F-13", Affects: []string{"ok/brief-07"}},
	})
	if len(problems) != 0 {
		t.Errorf("unacked finding must never exit 1, got %v", problems)
	}
	if !hasEntry(notices, "ok/brief-07: unresolved F-13 awaits desk ack") {
		t.Errorf("unacked finding on in-progress brief must notice, got %v", notices)
	}
}

func TestDemotionTodoFlaggedOnly(t *testing.T) {
	streams := demotionFixture("todo")
	findings := []Finding{{ID: "F-13", Affects: []string{"ok/brief-07"}, Ack: "2026-07-09 desk"}}
	problems, notices := check(streams, findings)
	if len(problems) != 0 || len(notices) != 0 {
		t.Errorf("todo brief must stay problem/notice-free: %v %v", problems, notices)
	}
	applyFindings(streams, findings)
	if streams[0].Briefs[0].StaleRef != "F-13" {
		t.Errorf("todo brief must still be stale-flagged: %+v", streams[0].Briefs[0])
	}
}

func TestDemotionResolvedInProgress(t *testing.T) {
	problems, notices := check(demotionFixture("in-progress"), []Finding{
		{ID: "F-13", Affects: []string{"ok/brief-07"}, Ack: "2026-07-09 desk", Resolved: true},
	})
	if len(problems) != 0 || len(notices) != 0 {
		t.Errorf("resolved finding must be inert: %v %v", problems, notices)
	}
}

func TestDemotionMalformedAck(t *testing.T) {
	problems, notices := check(demotionFixture("in-progress"), []Finding{
		{ID: "F-13", Affects: []string{"ok/brief-07"}, Ack: "yesterday, by someone"},
	})
	if !hasEntry(problems, "malformed Ack") {
		t.Errorf("a malformed Ack must fail loudly, not silently count as unacked: %v", problems)
	}
	if hasEntry(problems, "desk-acked") {
		t.Errorf("a malformed Ack must not also count as desk-acked: %v", problems)
	}
	if !hasEntry(notices, "awaits desk ack") {
		t.Errorf("a malformed Ack is not an ack — the awaits-ack notice still applies: %v", notices)
	}
}

func TestDemotionResolvedMalformedAckInert(t *testing.T) {
	problems, notices := check(demotionFixture("in-progress"), []Finding{
		{ID: "F-13", Affects: []string{"ok/brief-07"}, Ack: "garbage", Resolved: true},
	})
	if len(problems) != 0 || len(notices) != 0 {
		t.Errorf("resolved findings are fully inert, even with a malformed Ack: %v %v", problems, notices)
	}
}
