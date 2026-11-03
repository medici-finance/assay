package adapter_test

import (
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// TestDeriveAnswerRequiresClosed pins the load-bearing guard: an OPEN issue
// never derives an Answer, even when a comment carries a well-formed marker —
// the escalation is still being decided, so asserting an outcome would be a
// fabrication no poll actually observed as final.
func TestDeriveAnswerRequiresClosed(t *testing.T) {
	iss := adapter.RawIssue{
		Number: 1,
		Closed: false,
		Comments: []adapter.RawComment{
			{Body: "<!-- assay:answer v1 login=operator option=opt-a options_rev=deadbeef -->", CreatedAt: time.Now()},
		},
	}
	if ans, ok := adapter.DeriveAnswer(iss, adapter.AnswerVerifier{}); ok || ans != nil {
		t.Fatalf("DeriveAnswer on an open issue = (%v, %v), want (nil, false)", ans, ok)
	}
}

// TestDeriveAnswerParsesMarker is the service contract increment 3 task 6's core
// claim: contract.ParseAnswerRecord, wired into internal/adapter, derives
// {answered_by, option, options_rev} from a polled comment.
func TestDeriveAnswerParsesMarker(t *testing.T) {
	at := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	iss := adapter.RawIssue{
		Number: 2,
		Closed: true,
		Comments: []adapter.RawComment{
			{Body: "an earlier, unrelated remark", CreatedAt: at.Add(-time.Hour), Login: "someone"},
			{
				Body:      "<!-- assay:answer v1 login=operator option=opt-a options_rev=deadbeef -->\nDecision: `opt-a`",
				CreatedAt: at,
				Login:     "operator",
			},
		},
	}
	ans, ok := adapter.DeriveAnswer(iss, adapter.AnswerVerifier{})
	if !ok || ans == nil {
		t.Fatalf("DeriveAnswer on a closed, marked issue = (%v, %v), want an answer", ans, ok)
	}
	if ans.By.Name != "operator" {
		t.Errorf("By.Name = %q, want operator", ans.By.Name)
	}
	if ans.Option == nil || *ans.Option != domain.OptionID("opt-a") {
		t.Errorf("Option = %v, want opt-a", ans.Option)
	}
	if ans.OptionsRev != "deadbeef" {
		t.Errorf("OptionsRev = %q, want deadbeef", ans.OptionsRev)
	}
	if !ans.At.Equal(at) {
		t.Errorf("At = %v, want %v", ans.At, at)
	}
}

// TestDeriveAnswerPrefersMostRecentMarker: a corrected decision posted as a
// follow-up comment must win over an earlier one. The marker grammar itself
// carries no sequence number, so scan order (most-recent-first) is the only
// signal this function has.
func TestDeriveAnswerPrefersMostRecentMarker(t *testing.T) {
	base := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	iss := adapter.RawIssue{
		Number: 3,
		Closed: true,
		Comments: []adapter.RawComment{
			{Body: "<!-- assay:answer v1 login=operator option=opt-a options_rev=rev1 -->", CreatedAt: base, Login: "operator"},
			{Body: "<!-- assay:answer v1 login=operator option=opt-b options_rev=rev2 -->", CreatedAt: base.Add(time.Minute), Login: "operator"},
		},
	}
	ans, ok := adapter.DeriveAnswer(iss, adapter.AnswerVerifier{})
	if !ok || ans == nil {
		t.Fatalf("DeriveAnswer = (%v, %v), want an answer", ans, ok)
	}
	if ans.Option == nil || *ans.Option != domain.OptionID("opt-b") {
		t.Fatalf("Option = %v, want opt-b (the later comment)", ans.Option)
	}
	if ans.OptionsRev != "rev2" {
		t.Fatalf("OptionsRev = %q, want rev2 (the later comment)", ans.OptionsRev)
	}
}

// TestDeriveAnswerUnparsedStillAnswered pins design §10's malformed-marker
// rule at this call site: a present-but-incomplete marker is still an answer,
// never rounded down to "no marker".
func TestDeriveAnswerUnparsedStillAnswered(t *testing.T) {
	iss := adapter.RawIssue{
		Number: 4,
		Closed: true,
		Comments: []adapter.RawComment{
			{Body: "<!-- assay:answer v1 option=opt-a -->", CreatedAt: time.Now(), Login: "operator"},
		},
	}
	ans, ok := adapter.DeriveAnswer(iss, adapter.AnswerVerifier{})
	if !ok || ans == nil {
		t.Fatalf("DeriveAnswer on an unparsed-but-present marker = (%v, %v), want answered", ans, ok)
	}
}

// TestDeriveAnswerNoMarkerFalls: a closed issue with no marker anywhere in
// its comment window derives nothing here — the caller's prose fallback (if
// any) is out of this function's concern.
func TestDeriveAnswerNoMarkerFalls(t *testing.T) {
	iss := adapter.RawIssue{
		Number:   5,
		Closed:   true,
		Comments: []adapter.RawComment{{Body: "closing as done, thanks!", CreatedAt: time.Now(), Login: "operator"}},
	}
	if ans, ok := adapter.DeriveAnswer(iss, adapter.AnswerVerifier{}); ok || ans != nil {
		t.Fatalf("DeriveAnswer on a closed, unmarked issue = (%v, %v), want (nil, false)", ans, ok)
	}
}
