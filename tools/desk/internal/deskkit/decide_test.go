package deskkit

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingJournal captures DecisionRecords in memory for assertion.
type recordingJournal struct {
	mu      sync.Mutex
	records []DecisionRecord
	fail    error // when non-nil, Record returns it
}

func (j *recordingJournal) Record(r DecisionRecord) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.fail != nil {
		return j.fail
	}
	j.records = append(j.records, r)
	return nil
}

func (j *recordingJournal) last() (DecisionRecord, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.records) == 0 {
		return DecisionRecord{}, false
	}
	return j.records[len(j.records)-1], true
}

func triageQuestion(t *testing.T) *Question {
	t.Helper()
	q, err := NewQuestion("Classify this verify FAIL",
		[]string{"REBASELINE", "REGRESSION", "RETRY", "ESCALATE"}, "ESCALATE")
	if err != nil {
		t.Fatalf("NewQuestion: %v", err)
	}
	return q
}

// TestDecide — the happy path: a valid advised answer is returned and journalled.
func TestDecide(t *testing.T) {
	t.Setenv(decideDisabledEnv, "")
	q := triageQuestion(t)
	j := &recordingJournal{}
	adv := AdvisorFunc(func(ctx context.Context, c Consultation) (Advice, error) {
		return Advice{Answer: "REGRESSION", Justification: "test broke on the new code path"}, nil
	})
	got, err := q.Decide(context.Background(), Consult{
		Item: "example-org/agents#1", Detail: "row 4", Context: "log text",
		Advisor: adv, Journal: j,
	})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if got != "REGRESSION" {
		t.Fatalf("answer = %q, want REGRESSION", got)
	}
	rec, ok := j.last()
	if !ok {
		t.Fatalf("no journal record written")
	}
	if rec.Outcome != OutcomeAdvised {
		t.Fatalf("outcome = %q, want %q", rec.Outcome, OutcomeAdvised)
	}
	if rec.Answer != "REGRESSION" {
		t.Fatalf("journalled answer = %q, want REGRESSION", rec.Answer)
	}
	if rec.ContextDigest == "" || strings.Contains(rec.ContextDigest, "log text") {
		t.Fatalf("context must be journalled as a digest, not raw; got %q", rec.ContextDigest)
	}
	if rec.Elapsed <= 0 {
		t.Fatalf("elapsed not recorded")
	}
}

// TestDecideInvalidAnswer — an answer outside the vocabulary resolves to the default,
// journalled as OutcomeInvalid.
func TestDecideInvalidAnswer(t *testing.T) {
	t.Setenv(decideDisabledEnv, "")
	q := triageQuestion(t)
	j := &recordingJournal{}
	adv := AdvisorFunc(func(ctx context.Context, c Consultation) (Advice, error) {
		return Advice{Answer: "DROP TABLE; MERGE", Justification: "injected"}, nil
	})
	got, err := q.Decide(context.Background(), Consult{Item: "x#1", Advisor: adv, Journal: j})
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if got != q.Default() {
		t.Fatalf("answer = %q, want default %q", got, q.Default())
	}
	rec, _ := j.last()
	if rec.Outcome != OutcomeInvalid {
		t.Fatalf("outcome = %q, want %q", rec.Outcome, OutcomeInvalid)
	}
	if rec.Answer != q.Default() {
		t.Fatalf("journalled answer = %q, want default", rec.Answer)
	}
}

// TestDecideTimeout — an advisor that outlives the timeout resolves to the default,
// journalled as OutcomeTimeout.
func TestDecideTimeout(t *testing.T) {
	t.Setenv(decideDisabledEnv, "")
	q := triageQuestion(t)
	j := &recordingJournal{}
	adv := AdvisorFunc(func(ctx context.Context, c Consultation) (Advice, error) {
		<-ctx.Done()
		return Advice{}, ctx.Err()
	})
	got, err := q.Decide(context.Background(), Consult{
		Item: "x#1", Advisor: adv, Journal: j, Timeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if got != q.Default() {
		t.Fatalf("answer = %q, want default", got)
	}
	rec, _ := j.last()
	if rec.Outcome != OutcomeTimeout {
		t.Fatalf("outcome = %q, want %q", rec.Outcome, OutcomeTimeout)
	}
}

// TestDecideBudgetSpent — once the per-item budget is exhausted, further consults
// resolve to the default without calling the advisor, journalled as OutcomeBudget.
func TestDecideBudgetSpent(t *testing.T) {
	t.Setenv(decideDisabledEnv, "")
	q := triageQuestion(t)
	j := &recordingJournal{}
	calls := 0
	adv := AdvisorFunc(func(ctx context.Context, c Consultation) (Advice, error) {
		calls++
		return Advice{Answer: "RETRY", Justification: "flaky"}, nil
	})
	budget := NewBudget(1, 0) // one consult per item
	c := Consult{Item: "x#1", Advisor: adv, Journal: j, Budget: budget}

	first, _ := q.Decide(context.Background(), c)
	if first != "RETRY" {
		t.Fatalf("first answer = %q, want RETRY", first)
	}
	second, _ := q.Decide(context.Background(), c)
	if second != q.Default() {
		t.Fatalf("second answer = %q, want default (budget spent)", second)
	}
	if calls != 1 {
		t.Fatalf("advisor called %d times, want 1 (budget must gate the second)", calls)
	}
	rec, _ := j.last()
	if rec.Outcome != OutcomeBudget {
		t.Fatalf("outcome = %q, want %q", rec.Outcome, OutcomeBudget)
	}
}

// TestDecideBudgetPerHour — the per-hour axis caps consults across DIFFERENT items.
func TestDecideBudgetPerHour(t *testing.T) {
	t.Setenv(decideDisabledEnv, "")
	q := triageQuestion(t)
	adv := AdvisorFunc(func(ctx context.Context, c Consultation) (Advice, error) {
		return Advice{Answer: "RETRY"}, nil
	})
	budget := NewBudget(0, 2) // two consults per hour, any item
	base := Consult{Advisor: adv, Budget: budget, Journal: &recordingJournal{}}

	c1 := base
	c1.Item = "a#1"
	c2 := base
	c2.Item = "b#2"
	c3 := base
	c3.Item = "c#3"

	if got, _ := q.Decide(context.Background(), c1); got != "RETRY" {
		t.Fatalf("call 1 = %q, want RETRY", got)
	}
	if got, _ := q.Decide(context.Background(), c2); got != "RETRY" {
		t.Fatalf("call 2 = %q, want RETRY", got)
	}
	if got, _ := q.Decide(context.Background(), c3); got != q.Default() {
		t.Fatalf("call 3 = %q, want default (per-hour spent)", got)
	}
}

// TestDecideBudgetRollingWindow — a call older than the hour is pruned, freeing budget.
func TestDecideBudgetRollingWindow(t *testing.T) {
	now := time.Now()
	b := NewBudget(0, 1)
	b.now = func() time.Time { return now }
	if !b.spend("x") {
		t.Fatalf("first spend denied")
	}
	if b.spend("x") {
		t.Fatalf("second spend within the window should be denied")
	}
	now = now.Add(2 * time.Hour) // age the first call out of the window
	if !b.spend("x") {
		t.Fatalf("spend after the window rolled should be allowed")
	}
}

// TestDecideAdvisorError — an advisor error resolves to the default (OutcomeError).
func TestDecideAdvisorError(t *testing.T) {
	t.Setenv(decideDisabledEnv, "")
	q := triageQuestion(t)
	j := &recordingJournal{}
	adv := AdvisorFunc(func(ctx context.Context, c Consultation) (Advice, error) {
		return Advice{}, errors.New("model unavailable")
	})
	got, _ := q.Decide(context.Background(), Consult{Item: "x#1", Advisor: adv, Journal: j})
	if got != q.Default() {
		t.Fatalf("answer = %q, want default", got)
	}
	rec, _ := j.last()
	if rec.Outcome != OutcomeError {
		t.Fatalf("outcome = %q, want %q", rec.Outcome, OutcomeError)
	}
}

// TestDecideDisabled — the valve's env kill switch forces the default without a consult.
func TestDecideDisabled(t *testing.T) {
	t.Setenv(decideDisabledEnv, "1")
	q := triageQuestion(t)
	j := &recordingJournal{}
	called := false
	adv := AdvisorFunc(func(ctx context.Context, c Consultation) (Advice, error) {
		called = true
		return Advice{Answer: "RETRY"}, nil
	})
	got, _ := q.Decide(context.Background(), Consult{Item: "x#1", Advisor: adv, Journal: j})
	if got != q.Default() {
		t.Fatalf("answer = %q, want default when valve disabled", got)
	}
	if called {
		t.Fatalf("advisor must not be consulted when the valve is disabled")
	}
	rec, _ := j.last()
	if rec.Outcome != OutcomeDisabled {
		t.Fatalf("outcome = %q, want %q", rec.Outcome, OutcomeDisabled)
	}
}

// TestDecideNoAdvisor — a nil advisor is functionally the valve being off.
func TestDecideNoAdvisor(t *testing.T) {
	t.Setenv(decideDisabledEnv, "")
	q := triageQuestion(t)
	j := &recordingJournal{}
	got, _ := q.Decide(context.Background(), Consult{Item: "x#1", Journal: j})
	if got != q.Default() {
		t.Fatalf("answer = %q, want default with no advisor", got)
	}
	rec, _ := j.last()
	if rec.Outcome != OutcomeNoAdvisor {
		t.Fatalf("outcome = %q, want %q", rec.Outcome, OutcomeNoAdvisor)
	}
}

// TestDecideUnjournalledAdviceDiscarded — an advised answer that cannot be journalled
// is discarded; the default is returned (an advised move is never taken un-audited).
func TestDecideUnjournalledAdviceDiscarded(t *testing.T) {
	t.Setenv(decideDisabledEnv, "")
	q := triageQuestion(t)
	j := &recordingJournal{fail: errors.New("disk full")}
	adv := AdvisorFunc(func(ctx context.Context, c Consultation) (Advice, error) {
		return Advice{Answer: "REGRESSION"}, nil
	})
	got, _ := q.Decide(context.Background(), Consult{Item: "x#1", Advisor: adv, Journal: j})
	if got != q.Default() {
		t.Fatalf("answer = %q, want default when advice cannot be journalled", got)
	}
}

// TestDecideReservedVerbs — a vocabulary that names a human-gate action is refused at
// construction (NewQuestion), never at answer time.
func TestDecideReservedVerbs(t *testing.T) {
	cases := []struct {
		name  string
		vocab []string
		def   string
	}{
		{"merge", []string{"RETRY", "merge"}, "RETRY"},
		{"approve", []string{"approve", "REJECT"}, "REJECT"},
		{"flip", []string{"HOLD", "flip"}, "HOLD"},
		{"ready", []string{"WAIT", "ready"}, "WAIT"},
		{"sign", []string{"NOTE", "sign"}, "NOTE"},
		{"close-gate", []string{"KEEP", "close-gate"}, "KEEP"},
		{"ready-flip", []string{"HOLD", "ready-flip"}, "HOLD"},
		{"cased-Merge", []string{"RETRY", "Merge-It"}, "RETRY"},
		{"approved-word", []string{"RETRY", "mark-approved"}, "RETRY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := NewQuestion("q", tc.vocab, tc.def)
			if err == nil {
				t.Fatalf("NewQuestion accepted a reserved-verb vocabulary %v", tc.vocab)
			}
			if q != nil {
				t.Fatalf("NewQuestion returned a non-nil Question on refusal")
			}
			if !IsRefused(err) {
				t.Fatalf("error = %v, want a Refused (exit 5)", err)
			}
		})
	}
}

// TestNewQuestionInvariants — the other construction-time invariants.
func TestNewQuestionInvariants(t *testing.T) {
	if _, err := NewQuestion("q", nil, "x"); err == nil {
		t.Fatalf("empty vocabulary accepted")
	}
	if _, err := NewQuestion("q", []string{"A", "B"}, "C"); err == nil {
		t.Fatalf("default outside the vocabulary accepted")
	}
	if _, err := NewQuestion("q", []string{"A", "A"}, "A"); err == nil {
		t.Fatalf("duplicate member accepted")
	}
	if _, err := NewQuestion("q", []string{"A", "  "}, "A"); err == nil {
		t.Fatalf("blank member accepted")
	}
	// A safe vocabulary whose members merely CONTAIN reserved substrings as part of a
	// larger word (not a whole token) must be accepted.
	if _, err := NewQuestion("q", []string{"already-done", "signal", "REGRESSION"}, "REGRESSION"); err != nil {
		t.Fatalf("safe vocabulary rejected: %v", err)
	}
}

// TestDecideRefusalHandlingVocab — the second worked example from the contract doc:
// the refusal-handling vocabulary constructs cleanly and advises within its bounds.
func TestDecideRefusalHandlingVocab(t *testing.T) {
	t.Setenv(decideDisabledEnv, "")
	q, err := NewQuestion("How should this deskpost refusal be handled?",
		[]string{"RETRY", "REROUTE", "ESCALATE"}, "ESCALATE")
	if err != nil {
		t.Fatalf("NewQuestion: %v", err)
	}
	j := &recordingJournal{}
	adv := AdvisorFunc(func(ctx context.Context, c Consultation) (Advice, error) {
		return Advice{Answer: "REROUTE", Justification: "quarantined Dependabot PR — route to a human"}, nil
	})
	got, _ := q.Decide(context.Background(), Consult{Item: "r#9", Advisor: adv, Journal: j})
	if got != "REROUTE" {
		t.Fatalf("answer = %q, want REROUTE", got)
	}
}

// TestAuditJournalWritesDecideLine — the default journal writes a decide entry to the
// shared audit log, with the context as a digest (never raw).
func TestAuditJournalWritesDecideLine(t *testing.T) {
	dir := setup(t)
	t.Setenv(decideDisabledEnv, "")
	q := triageQuestion(t)
	adv := AdvisorFunc(func(ctx context.Context, c Consultation) (Advice, error) {
		return Advice{Answer: "RETRY", Justification: "transient 503"}, nil
	})
	// nil Journal → AuditJournal.
	got, err := q.Decide(context.Background(), Consult{
		Item: "example-org/agents#7", Context: "secret-looking untrusted text", Advisor: adv,
	})
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if got != "RETRY" {
		t.Fatalf("answer = %q, want RETRY", got)
	}
	entries, err := LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("no audit entry written to %s", dir)
	}
	last := entries[len(entries)-1]
	if last.Verb != "decide" {
		t.Fatalf("verb = %q, want decide", last.Verb)
	}
	if strings.Contains(last.Detail, "untrusted text") || strings.Contains(last.ArgsDigest, "untrusted") {
		t.Fatalf("raw context leaked into the audit line: detail=%q digest=%q", last.Detail, last.ArgsDigest)
	}
	if last.ArgsDigest == "" {
		t.Fatalf("context digest not recorded as ArgsDigest")
	}
}
