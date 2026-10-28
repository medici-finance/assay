// Package answer is the human-gate WRITE PATH: the ONE place a
// operator-authored mutation is produced, behind a live human session. It is the
// authorization artifact's referent for §13.1 (the operator, the custody design).
//
// # The three invariants this package exists to enforce
//
//  1. Exactly ONE write path. Every human-gate platform write — the answer
//     (comment + close) and the ack write-through (👀 reaction) — funnels
//     through [HumanWriter], the only type here that builds a non-GET GitHub
//     request. TestExactlyOneWritePath parses the package and fails if a second
//     one appears, so the property is checked on every build, not argued.
//  2. Gated on a LIVE HUMAN session. Every write requires a *Session that
//     [SessionStore.Begin] minted — the only setter of the unexported `live`
//     bit — so a bot or an unauthenticated caller has no way to reach a write.
//  3. Stamped with the human's attribution. The write authors as operator
//     (the credential's identity), checked against the session actor before it
//     goes out; a mutation with any other author is refused.
//
// # What it is NOT
//
// It is not a second write path beside the MCP surface's App path — it is the
// human path, and it is deliberately unreachable from MCP (the credential lives
// in internal/custody, which no file MCP can reach may import; the §11 authority
// test asserts it). It adds no bypass, no auto-answer, and no way for a caller
// to name a different author.
package answer

import (
	"context"
	"errors"
	"fmt"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// Sentinel errors. They are first-class so a handler can map each to the right
// status and the web layer can keep the draft on exactly the recoverable ones.
var (
	// ErrNoSession — no session was presented.
	ErrNoSession = errors.New("no live session: the answer path is reachable only behind a human session (§6)")
	// ErrNotLiveHuman — a session exists but was not established interactively.
	ErrNotLiveHuman = errors.New("session is not a live human presence: only a Begin-minted session may write")
	// ErrSessionExpired — the session TTL elapsed.
	ErrSessionExpired = errors.New("session expired: begin a new console session to answer")
	// ErrNoCredential — no operator credential is held to author with.
	ErrNoCredential = errors.New("no human credential accompanied the request")
	// ErrRaced — the escalation changed since the human last saw it
	// (spec §10 answer-race). The draft is kept; the human re-reads and retries.
	ErrRaced = errors.New("answer race: the escalation changed since you loaded it — draft kept, re-read before answering")
	// ErrEmptyAnswer — neither an option nor text was supplied.
	ErrEmptyAnswer = errors.New("an answer must choose an option or carry text")
	// ErrBadRef — the escalation's platform ref could not be used.
	ErrBadRef = errors.New("the escalation has no usable platform ref to write against")
	// ErrCommittedOpen — the decision comment posted but the close failed. The
	// decision is recorded on the platform; the escalation is still open.
	ErrCommittedOpen = errors.New("decision comment posted but the close failed: the escalation is still open — closing again is safe, do not re-submit the comment")
)

// AckContent is the reaction the ack write-through posts (D7): GitHub's "eyes"
// is 👀. It is a reaction, never a close.
const AckContent = "eyes"

// platformWriter is what the Service depends on. HumanWriter satisfies it; a
// test supplies a fake with no network. Keeping it an interface is what lets
// TestWritePathRequiresLiveSession assert "no write happened" by counting the
// fake's calls.
type platformWriter interface {
	Actor() string
	CurrentState(ctx context.Context, ref Ref) (IssueState, error)
	Comment(ctx context.Context, ref Ref, body string) error
	Close(ctx context.Context, ref Ref) error
	React(ctx context.Context, ref Ref, content string) error
}

// AnswerRequest is one human decision to commit. It carries what the human saw
// (Ref, OptionsRev, LastSeenUpdatedAt) so the race check can tell whether the
// escalation is still the one they were answering.
type AnswerRequest struct {
	ID  domain.EscalationID
	Ref Ref
	// Option is the chosen option id (the digit-staged choice). Empty is
	// allowed only when Text is not, for an option-less (prose) escalation.
	Option domain.OptionID
	// OptionsRev is the options-rev the human answered against; it is recorded
	// in the decision comment so the answer names the exact option set it chose
	// from (Verify 2).
	OptionsRev string
	// Text is the free-text half of the answer.
	Text string
	// LastSeenUpdatedAt is the issue's updated_at when the human loaded it. A
	// later platform updated_at means the escalation changed under them — a race.
	LastSeenUpdatedAt time.Time
}

// AnswerResult is what a committed answer reports back.
type AnswerResult struct {
	ID         domain.EscalationID `json:"id"`
	Ref        Ref                 `json:"ref"`
	AuthoredAs string              `json:"authored_as"`
	// SignedBy is the human whose verified signed intent this answer carries
	// (the authorization contract ruling 2); empty on the live-session path, where the
	// write is authored as the human directly.
	SignedBy   string          `json:"signed_by,omitempty"`
	Option     domain.OptionID `json:"option,omitempty"`
	OptionsRev string          `json:"options_rev,omitempty"`
	Closed     bool            `json:"closed"`
	At         time.Time       `json:"at"`
}

// Service is the write path. It holds exactly one writer.
type Service struct {
	writer platformWriter
	drafts DraftStore
	acks   AckRecorder
	now    func() time.Time
	// answerAs is the cell's authorized-answerer login (cells.yaml, default
	// humanActor()). The NEW answerer-login binding (the capability model task 5, SPOF layer)
	// lives in requireLive: a live session may write ONLY when its authenticated
	// Actor equals answerAs. Empty means "default to the write identity", which
	// preserves the pre-brief behaviour (answer as operator, gated on operator).
	answerAs string
}

// NewService builds the write path. A nil DraftStore falls back to an in-memory
// one (drafts then do not survive a restart — deskd always supplies the durable
// index-backed store).
func NewService(w platformWriter, drafts DraftStore, acks AckRecorder, now func() time.Time) *Service {
	if drafts == nil {
		drafts = NewMemoryDraftStore()
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{writer: w, drafts: drafts, acks: acks, now: now}
}

// SetAnswerAs binds the Service to a cell's authorized-answerer login (the capability model
// task 5). deskd calls it with the cell's answer_as (default humanActor()). It is
// the authorization identity a live session's Actor must equal to write; the
// write is still authored as operator by the custody credential, unchanged.
func (s *Service) SetAnswerAs(login string) { s.answerAs = login }

// AnswerAs reports the configured authorized-answerer login, defaulting to the
// write identity when unset.
func (s *Service) AnswerAs() string {
	if s.answerAs == "" {
		return s.writer.Actor()
	}
	return s.answerAs
}

// requireLive is the session gate every write passes through. It also checks
// the session's actor matches the writer's — both must be operator — so a write
// can never be stamped with an identity the session did not establish.
func (s *Service) requireLive(sess *Session) error {
	if !humanid.Enabled() {
		return humanid.ErrDisabled
	}
	if sess == nil {
		return ErrNoSession
	}
	if !sess.Live() {
		return ErrNotLiveHuman
	}
	if s.now().After(sess.ExpiresAt) {
		return ErrSessionExpired
	}
	// The NEW answerer-login binding: the session's
	// authenticated login must equal the cell's authorized answerer. This is the
	// answer path's own gate, independent of the capability middleware — an
	// attacker past the mux still cannot answer unless their authenticated login
	// IS the answerer. The write is still authored as operator by the custody
	// credential (github.go token()), which is a separate, unchanged guarantee.
	if sess.Actor == "" || s.AnswerAs() == "" || !strings.EqualFold(sess.Actor, s.AnswerAs()) {
		return fmt.Errorf("session login %q is not the cell's authorized answerer %q — refusing to answer (a commenter-tier user can never reach the answer path)", sess.Actor, s.AnswerAs())
	}
	return nil
}

// SaveDraft persists an in-progress answer. It requires a live session too — a
// draft is the human's own state and must not be writable by a background
// caller — but it performs NO platform write.
func (s *Service) SaveDraft(sess *Session, d Draft) error {
	if err := s.requireLive(sess); err != nil {
		return err
	}
	d.SavedAt = s.now()
	return s.drafts.SaveDraft(d)
}

// LoadDraft returns any saved draft for an escalation, behind the session gate.
func (s *Service) LoadDraft(sess *Session, id domain.EscalationID) (Draft, bool, error) {
	if err := s.requireLive(sess); err != nil {
		return Draft{}, false, err
	}
	return s.drafts.LoadDraft(id)
}

// Answer commits a human decision: comment + close as operator, conditional on
// the escalation not having changed since the human loaded it (spec §10). The
// draft is written before the platform writes and cleared only on full success,
// so a crash or a failure leaves the text to reopen to (§7.6).
func (s *Service) Answer(ctx context.Context, sess *Session, req AnswerRequest) (AnswerResult, error) {
	if err := s.requireLive(sess); err != nil {
		return AnswerResult{}, err
	}
	return s.commit(ctx, req, nil)
}

// commit is the answer's one body, shared by the live-session path
// ([Service.Answer]) and the signed-intent path ([Service.AnswerSigned]). Each
// caller has already passed its own gate; act is nil on the session path and
// selects the decision body that records the signer on the signed path.
func (s *Service) commit(ctx context.Context, req AnswerRequest, act *SignedAct) (AnswerResult, error) {
	if !req.Ref.valid() {
		return AnswerResult{}, ErrBadRef
	}
	if req.Option == "" && req.Text == "" {
		return AnswerResult{}, ErrEmptyAnswer
	}

	// Persist the draft up front so a mid-commit crash (Verify 9) leaves it.
	draft := Draft{EscalationID: req.ID, Option: string(req.Option), Text: req.Text, SavedAt: s.now()}
	_ = s.drafts.SaveDraft(draft)

	// Answer-race check (spec §10): re-read the current platform state and
	// refuse if the escalation is already closed or was updated since the human
	// loaded it. The draft is kept on a race.
	cur, err := s.writer.CurrentState(ctx, req.Ref)
	if err != nil {
		return AnswerResult{}, fmt.Errorf("could not read current state for the race check (draft kept): %w", err)
	}
	if cur.Closed {
		return AnswerResult{}, fmt.Errorf("%w: it is already closed", ErrRaced)
	}
	if !req.LastSeenUpdatedAt.IsZero() && cur.UpdatedAt.After(req.LastSeenUpdatedAt) {
		return AnswerResult{}, fmt.Errorf("%w: it was updated at %s, after you loaded it at %s", ErrRaced, cur.UpdatedAt.Format(time.RFC3339), req.LastSeenUpdatedAt.Format(time.RFC3339))
	}

	// The decision comment records the chosen option id and the options-rev it
	// was chosen from (Verify 2), authored as operator — or, on the signed path,
	// by the gateway carrying the signer's answer record.
	body := decisionBody(req)
	if act != nil {
		body = signedDecisionBody(req, act)
	}
	if err := s.writer.Comment(ctx, req.Ref, body); err != nil {
		return AnswerResult{}, fmt.Errorf("posting the decision comment failed (draft kept): %w", err)
	}
	if err := s.writer.Close(ctx, req.Ref); err != nil {
		// The comment landed; the close did not. Keep the draft and report the
		// partial state precisely rather than pretend it committed.
		return AnswerResult{}, fmt.Errorf("%w: %v", ErrCommittedOpen, err)
	}

	if err := s.drafts.ClearDraft(req.ID); err != nil {
		// The answer committed; failing to clear the draft is a cosmetic
		// residue, not a failed answer. Report success with the note attached.
		return s.result(req), fmt.Errorf("answer committed but the draft could not be cleared: %w", err)
	}
	return s.result(req), nil
}

func (s *Service) result(req AnswerRequest) AnswerResult {
	return AnswerResult{
		ID:         req.ID,
		Ref:        req.Ref,
		AuthoredAs: s.writer.Actor(),
		Option:     req.Option,
		OptionsRev: req.OptionsRev,
		Closed:     true,
		At:         s.now(),
	}
}

// Ack is the D7 write-through: post a operator 👀 reaction on the escalation and
// record the local ack timestamp. The issue STAYS OPEN — an ack is "seen,
// parked", not answered.
func (s *Service) Ack(ctx context.Context, sess *Session, id domain.EscalationID, ref Ref) error {
	if err := s.requireLive(sess); err != nil {
		return err
	}
	return s.ack(ctx, id, ref, s.writer.Actor())
}

// ack is the ack's one body, shared by the live-session and signed paths.
// ackedBy is recorded with the local timestamp: the write identity on the
// session path (unchanged), the SIGNER on the signed path.
func (s *Service) ack(ctx context.Context, id domain.EscalationID, ref Ref, ackedBy string) error {
	if !ref.valid() {
		return ErrBadRef
	}
	if err := s.writer.React(ctx, ref, AckContent); err != nil {
		return fmt.Errorf("posting the ack reaction failed: %w", err)
	}
	// Record the local ack timestamp (spec §1.1.2). The reaction is platform
	// state; the timestamp is deskd's own. A failure here does not un-post the
	// reaction, so it is reported but not treated as a failed ack.
	if s.acks != nil {
		if err := s.acks.SetAck(id, ackedBy, s.now()); err != nil {
			return fmt.Errorf("ack reaction posted but the local timestamp did not record: %w", err)
		}
	}
	return nil
}

// decisionBody formats the operator-authored decision comment. It records the
// chosen option id with its options-rev (Verify 2) and marks the console as the
// author path, so a reader on GitHub sees exactly which option, from which
// option set, the human chose.
func decisionBody(req AnswerRequest) string {
	body := ""
	if req.Option != "" {
		rev := req.OptionsRev
		if rev != "" {
			rev = domain.ShortRev(rev)
			body += fmt.Sprintf("Decision: `%s` (options-rev %s)\n\n", req.Option, rev)
		} else {
			body += fmt.Sprintf("Decision: `%s`\n\n", req.Option)
		}
	}
	if req.Text != "" {
		body += req.Text + "\n\n"
	}
	body += fmt.Sprintf("— answered as %s via the console", humanid.Login())
	return body
}
