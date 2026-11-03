package answer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// fakeWriter is a platformWriter that records calls and makes no network call,
// so a test can assert exactly what did — and did NOT — reach the platform.
type fakeWriter struct {
	actor       string
	state       IssueState
	stateErr    error
	comments    int
	closes      int
	reacts      int
	lastComment string
	lastReact   string
	commentErr  error
	closeErr    error
	reactErr    error
}

func (f *fakeWriter) Actor() string {
	if f.actor == "" {
		return humanActor()
	}
	return f.actor
}
func (f *fakeWriter) CurrentState(context.Context, Ref) (IssueState, error) {
	return f.state, f.stateErr
}
func (f *fakeWriter) Comment(_ context.Context, _ Ref, body string) error {
	f.comments++
	f.lastComment = body
	return f.commentErr
}
func (f *fakeWriter) Close(context.Context, Ref) error { f.closes++; return f.closeErr }
func (f *fakeWriter) React(_ context.Context, _ Ref, content string) error {
	f.reacts++
	f.lastReact = content
	return f.reactErr
}

type fakeAcks struct {
	acked map[domain.EscalationID]string
}

func (f *fakeAcks) SetAck(id domain.EscalationID, actor string, _ time.Time) error {
	if f.acked == nil {
		f.acked = map[domain.EscalationID]string{}
	}
	f.acked[id] = actor
	return nil
}

func liveSession(t *testing.T) *Session {
	t.Helper()
	st := NewSessionStore(func() time.Time { return time.Now().UTC() }, func() bool { return true })
	s, err := st.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	return s
}

var testRef = Ref{Repo: "example-org/console", Number: 42, URL: "https://github.com/example-org/console/issues/42"}

func testReq() AnswerRequest {
	return AnswerRequest{ID: "01ID", Ref: testRef, Option: "keep-it", OptionsRev: strings.Repeat("a", 40), Text: "because"}
}

// TestWritePathRequiresLiveSession is the core security assertion: NO platform
// write happens without a live human session.
func TestWritePathRequiresLiveSession(t *testing.T) {
	cases := []struct {
		name string
		sess *Session
		want error
	}{
		{"nil session", nil, ErrNoSession},
		{"non-live session (struct literal cannot forge live)", &Session{Actor: humanActor(), ExpiresAt: time.Now().Add(time.Hour)}, ErrNotLiveHuman},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := &fakeWriter{}
			svc := NewService(w, NewMemoryDraftStore(), &fakeAcks{}, time.Now)
			if _, err := svc.Answer(context.Background(), c.sess, testReq()); !errors.Is(err, c.want) {
				t.Fatalf("Answer error = %v, want %v", err, c.want)
			}
			if w.comments != 0 || w.closes != 0 {
				t.Fatalf("a write happened without a live session: comments=%d closes=%d", w.comments, w.closes)
			}
		})
	}
}

// TestExpiredSessionCannotWrite proves the TTL is load-bearing.
func TestExpiredSessionCannotWrite(t *testing.T) {
	w := &fakeWriter{}
	// now is fixed in the future so the session (minted "now") is already past.
	svc := NewService(w, NewMemoryDraftStore(), &fakeAcks{}, func() time.Time { return time.Now().Add(2 * SessionTTL) })
	sess := liveSession(t)
	if _, err := svc.Answer(context.Background(), sess, testReq()); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
	if w.comments != 0 {
		t.Fatal("an expired session wrote")
	}
}

// TestAnswerStampsHuman proves the write authors as operator and records the
// chosen option id with its options-rev (Verify 2).
func TestAnswerStampsHuman(t *testing.T) {
	w := &fakeWriter{}
	svc := NewService(w, NewMemoryDraftStore(), &fakeAcks{}, time.Now)
	res, err := svc.Answer(context.Background(), liveSession(t), testReq())
	if err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if res.AuthoredAs != "operator" {
		t.Fatalf("authored as %q, want operator", res.AuthoredAs)
	}
	if w.comments != 1 || w.closes != 1 {
		t.Fatalf("expected one comment and one close, got comments=%d closes=%d", w.comments, w.closes)
	}
	if !strings.Contains(w.lastComment, "`keep-it`") {
		t.Fatalf("comment does not record the option id: %q", w.lastComment)
	}
	if !strings.Contains(w.lastComment, "options-rev "+domain.ShortRev(strings.Repeat("a", 40))) {
		t.Fatalf("comment does not record the options-rev: %q", w.lastComment)
	}
}

// TestAnswerRefusesWrongIdentity proves the answerer-login binding (the capability model
// task 5): a live session whose AUTHENTICATED login is not the cell's answer_as
// cannot write — a commenter-tier user, even past the middleware, cannot answer.
// The write's own identity (operator, via custody) is a separate guarantee
// enforced in github.go token(); this asserts the AUTHORIZATION binding.
func TestAnswerRefusesWrongIdentity(t *testing.T) {
	w := &fakeWriter{} // authors as operator
	svc := NewService(w, NewMemoryDraftStore(), &fakeAcks{}, time.Now)
	svc.SetAnswerAs("operator") // the cell's authorized answerer

	// A live session established as a DIFFERENT authenticated login.
	st := NewSessionStore(func() time.Time { return time.Now().UTC() }, func() bool { return true })
	commenter, err := st.BeginAs("some-commenter")
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Answer(context.Background(), commenter, testReq())
	if err == nil || !strings.Contains(err.Error(), "authorized answerer") {
		t.Fatalf("expected an answerer-binding refusal, got %v", err)
	}
	if w.comments != 0 {
		t.Fatal("a non-answerer session wrote")
	}

	// The SAME request as the authorized answerer commits.
	answerer, err := st.BeginAs("operator")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Answer(context.Background(), answerer, testReq()); err != nil {
		t.Fatalf("the authorized answerer was refused: %v", err)
	}
	if w.comments != 1 || w.closes != 1 {
		t.Fatalf("the authorized answerer's write did not happen: comments=%d closes=%d", w.comments, w.closes)
	}
}

// TestAnswerRace proves the answer is conditional on last-seen state and the
// draft survives a race (spec §10, §7.6).
func TestAnswerRace(t *testing.T) {
	t.Run("already closed", func(t *testing.T) {
		w := &fakeWriter{state: IssueState{Closed: true}}
		drafts := NewMemoryDraftStore()
		svc := NewService(w, drafts, &fakeAcks{}, time.Now)
		_, err := svc.Answer(context.Background(), liveSession(t), testReq())
		if !errors.Is(err, ErrRaced) {
			t.Fatalf("expected ErrRaced, got %v", err)
		}
		if w.comments != 0 || w.closes != 0 {
			t.Fatal("wrote despite a race")
		}
		if _, ok, _ := drafts.LoadDraft("01ID"); !ok {
			t.Fatal("draft was not kept across a race")
		}
	})
	t.Run("updated since last seen", func(t *testing.T) {
		seen := time.Now().Add(-time.Hour)
		w := &fakeWriter{state: IssueState{UpdatedAt: time.Now()}}
		svc := NewService(w, NewMemoryDraftStore(), &fakeAcks{}, time.Now)
		req := testReq()
		req.LastSeenUpdatedAt = seen
		if _, err := svc.Answer(context.Background(), liveSession(t), req); !errors.Is(err, ErrRaced) {
			t.Fatalf("expected ErrRaced on update, got %v", err)
		}
		if w.comments != 0 {
			t.Fatal("wrote despite an update race")
		}
	})
}

// TestDraftSurvivesCommitFailure proves a failed platform write keeps the draft.
func TestDraftSurvivesCommitFailure(t *testing.T) {
	w := &fakeWriter{commentErr: errors.New("502 from GitHub")}
	drafts := NewMemoryDraftStore()
	svc := NewService(w, drafts, &fakeAcks{}, time.Now)
	if _, err := svc.Answer(context.Background(), liveSession(t), testReq()); err == nil {
		t.Fatal("expected a commit failure")
	}
	if _, ok, _ := drafts.LoadDraft("01ID"); !ok {
		t.Fatal("draft was not kept across a failed commit")
	}
}

// TestCloseFailureIsPartialNotClean proves a comment-posted/close-failed answer
// reports the partial state and keeps the draft rather than claiming success.
func TestCloseFailureIsPartialNotClean(t *testing.T) {
	w := &fakeWriter{closeErr: errors.New("close 403")}
	drafts := NewMemoryDraftStore()
	svc := NewService(w, drafts, &fakeAcks{}, time.Now)
	_, err := svc.Answer(context.Background(), liveSession(t), testReq())
	if !errors.Is(err, ErrCommittedOpen) {
		t.Fatalf("expected ErrCommittedOpen, got %v", err)
	}
	if _, ok, _ := drafts.LoadDraft("01ID"); !ok {
		t.Fatal("draft was not kept after a partial commit")
	}
}

// TestAckWriteThrough proves the ack posts a operator 👀 reaction, records the
// local timestamp, and leaves the issue OPEN (D7).
func TestAckWriteThrough(t *testing.T) {
	w := &fakeWriter{}
	acks := &fakeAcks{}
	svc := NewService(w, NewMemoryDraftStore(), acks, time.Now)
	if err := svc.Ack(context.Background(), liveSession(t), "01ID", testRef); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if w.reacts != 1 || w.lastReact != AckContent {
		t.Fatalf("expected one %q reaction, got reacts=%d last=%q", AckContent, w.reacts, w.lastReact)
	}
	if w.closes != 0 {
		t.Fatal("ack closed the issue — it must stay open")
	}
	if acks.acked["01ID"] != "operator" {
		t.Fatalf("local ack not recorded as operator: %v", acks.acked)
	}
}

// TestAckRequiresLiveSession proves the ack write-through is gated too.
func TestAckRequiresLiveSession(t *testing.T) {
	w := &fakeWriter{}
	svc := NewService(w, NewMemoryDraftStore(), &fakeAcks{}, time.Now)
	if err := svc.Ack(context.Background(), nil, "01ID", testRef); !errors.Is(err, ErrNoSession) {
		t.Fatalf("expected ErrNoSession, got %v", err)
	}
	if w.reacts != 0 {
		t.Fatal("acked without a live session")
	}
}
