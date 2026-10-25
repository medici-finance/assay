package adapter_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// The poll-side layer (the authorization contract gap 3, item (5); the authorization contract
// row 18) with the SERVICE BYPASSED: no deskd handler, guard or intent
// verifier runs. Each test plants a comment directly — as a compromised or
// mis-scoped gateway could post it — and feeds adapter.DeriveAnswer the raw
// issue.

const (
	thisIssue  = "https://github.com/owner/name/issues/7"
	otherIssue = "https://github.com/owner/name/issues/8"
	gatewayBot = "gateway-app[bot]"
	answerAs   = "operator"
)

var posted = time.Date(2026, 10, 1, 12, 0, 30, 0, time.UTC)

type keyRing map[string][]ed25519.PublicKey

func (r keyRing) verifier() adapter.AnswerVerifier {
	return adapter.AnswerVerifier{
		Keys:     func(login string) []ed25519.PublicKey { return r[strings.ToLower(login)] },
		AnswerAs: answerAs,
	}
}

func newKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

// recordBody is a signed answer record as the gateway renders it: login,
// option and rev from the intent, the item (4) fields after them, and the
// signature over the canonical bytes by priv; key= is pub's fingerprint.
func recordBody(login, issue string, pub ed25519.PublicKey, priv ed25519.PrivateKey) string {
	iat := posted.Add(-30 * time.Second)
	si := contract.SignedIntent{
		Action: contract.ActionAnswer, Issue: issue, Login: login,
		OptionID: "opt-a", OptionsRev: "rev1", IssuedAt: iat,
		ExpiresAt: iat.Add(5 * time.Minute), Nonce: "nonce-0000000000000001",
	}
	si.Sign(priv)
	rec := contract.AnswerRecord{Login: login, OptionID: si.OptionID, OptionsRev: si.OptionsRev, Signed: contract.SignedRecordFor(si, contract.KeyFingerprint(pub))}
	return rec.Marshal("Decision: `opt-a`")
}

func closedWith(c adapter.RawComment) adapter.RawIssue {
	return adapter.RawIssue{Number: 7, URL: thisIssue, Closed: true, Comments: []adapter.RawComment{c}}
}

func botComment(body string, at time.Time) adapter.RawComment {
	return adapter.RawComment{Body: body, CreatedAt: at, Login: gatewayBot, AuthorBot: true}
}

// wantUnverified fails unless ans is answered-unverified attributed to the
// comment's real author, carrying no option — never the human's answer.
func wantUnverified(t *testing.T, ans *domain.Answer, ok bool, author string, isBot bool) {
	t.Helper()
	if !ok || ans == nil {
		t.Fatalf("DeriveAnswer = (%v, %v), want answered-unverified", ans, ok)
	}
	if !ans.Unverified {
		t.Fatalf("derived a verified answer from an unverifiable record: By=%+v option=%v", ans.By, ans.Option)
	}
	if ans.By.Name != author || ans.By.IsBot != isBot {
		t.Fatalf("By = %+v, want the comment author {%s %v}", ans.By, author, isBot)
	}
	if ans.Option != nil || ans.OptionsRev != "" {
		t.Fatalf("answered-unverified carries option=%v rev=%q, want none", ans.Option, ans.OptionsRev)
	}
}

// wantCause asserts contract.VerifyAnswerRecord refuses body for exactly the
// cause a leg varies, so a leg cannot pass on an unrelated refusal.
func wantCause(t *testing.T, body string, v adapter.AnswerVerifier, at time.Time, cause error) {
	t.Helper()
	rec, status := contract.ParseAnswerRecord(body)
	if status != contract.Answered {
		t.Fatalf("record did not parse: %v", status)
	}
	err := contract.VerifyAnswerRecord(rec, v.Keys(rec.Login), contract.AnswerBinding{AnswerAs: v.AnswerAs, Issue: thisIssue, PostedAt: at})
	if !errors.Is(err, cause) {
		t.Fatalf("VerifyAnswerRecord = %v, want cause %v", err, cause)
	}
}

// TestDeriveForgedMarkerRefused: a bot-authored marker naming the cell's
// answer_as derives the human's answer only when it is a verifiable signed
// record bound to THIS escalation, window and answer_as. Legs (a)–(f) of
// row 18; each derives answered-unverified attributed to the bot.
func TestDeriveForgedMarkerRefused(t *testing.T) {
	pub, priv := newKey(t)
	alicePub, alicePriv := newKey(t)
	ring := keyRing{answerAs: {pub}, "alice": {alicePub}}
	v := ring.verifier()

	t.Run("a no signed fields", func(t *testing.T) {
		body := contract.AnswerRecord{Login: answerAs, OptionID: "opt-a", OptionsRev: "rev1"}.Marshal("Decision: `opt-a`")
		ans, ok := adapter.DeriveAnswer(closedWith(botComment(body, posted)), v)
		wantUnverified(t, ans, ok, gatewayBot, true)
	})
	t.Run("b sig does not verify", func(t *testing.T) {
		body := strings.Replace(recordBody(answerAs, thisIssue, pub, priv), "option=opt-a", "option=opt-b", 1)
		ans, ok := adapter.DeriveAnswer(closedWith(botComment(body, posted)), v)
		wantUnverified(t, ans, ok, gatewayBot, true)
		wantCause(t, body, v, posted, contract.ErrRecordBadSignature)
	})
	t.Run("c key not enrolled", func(t *testing.T) {
		strangerPub, strangerPriv := newKey(t)
		body := recordBody(answerAs, thisIssue, strangerPub, strangerPriv)
		ans, ok := adapter.DeriveAnswer(closedWith(botComment(body, posted)), v)
		wantUnverified(t, ans, ok, gatewayBot, true)
		wantCause(t, body, v, posted, contract.ErrRecordUnknownKey)
	})
	t.Run("d replayed onto other issue", func(t *testing.T) {
		body := recordBody(answerAs, otherIssue, pub, priv)
		ans, ok := adapter.DeriveAnswer(closedWith(botComment(body, posted)), v)
		wantUnverified(t, ans, ok, gatewayBot, true)
		wantCause(t, body, v, posted, contract.ErrRecordIssue)
	})
	t.Run("e posted outside window", func(t *testing.T) {
		body := recordBody(answerAs, thisIssue, pub, priv)
		late := posted.Add(time.Hour)
		ans, ok := adapter.DeriveAnswer(closedWith(botComment(body, late)), v)
		wantUnverified(t, ans, ok, gatewayBot, true)
		wantCause(t, body, v, late, contract.ErrRecordWindow)
	})
	t.Run("f enrolled login not answer_as", func(t *testing.T) {
		body := recordBody("alice", thisIssue, alicePub, alicePriv)
		ans, ok := adapter.DeriveAnswer(closedWith(botComment(body, posted)), v)
		wantUnverified(t, ans, ok, gatewayBot, true)
		wantCause(t, body, v, posted, contract.ErrRecordAnswerAs)
	})
	t.Run("zero verifier trusts nothing", func(t *testing.T) {
		body := recordBody(answerAs, thisIssue, pub, priv)
		ans, ok := adapter.DeriveAnswer(closedWith(botComment(body, posted)), adapter.AnswerVerifier{})
		wantUnverified(t, ans, ok, gatewayBot, true)
	})
	t.Run("unparsed bot marker", func(t *testing.T) {
		body := "<!-- assay:answer v1 option=opt-a -->"
		ans, ok := adapter.DeriveAnswer(closedWith(botComment(body, posted)), v)
		wantUnverified(t, ans, ok, gatewayBot, true)
	})
}

// TestDeriveSignedMarkerAccepted: the genuine record derives the HUMAN's
// answer (not the bot's); revoking its key, or handing answer_as to someone
// else, re-derives it answered-unverified on the next poll.
func TestDeriveSignedMarkerAccepted(t *testing.T) {
	pub, priv := newKey(t)
	ring := keyRing{answerAs: {pub}}
	body := recordBody(answerAs, thisIssue, pub, priv)
	iss := closedWith(botComment(body, posted))

	ans, ok := adapter.DeriveAnswer(iss, ring.verifier())
	if !ok || ans == nil || ans.Unverified {
		t.Fatalf("DeriveAnswer(genuine) = (%+v, %v), want the human's answer", ans, ok)
	}
	if ans.By != (domain.Actor{Name: answerAs}) {
		t.Fatalf("By = %+v, want the signer %s (not the bot)", ans.By, answerAs)
	}
	if ans.Option == nil || *ans.Option != "opt-a" || ans.OptionsRev != "rev1" || !ans.At.Equal(posted) {
		t.Fatalf("answer = option %v rev %q at %v", ans.Option, ans.OptionsRev, ans.At)
	}

	revoked := keyRing{}
	ans, ok = adapter.DeriveAnswer(iss, revoked.verifier())
	wantUnverified(t, ans, ok, gatewayBot, true)

	handedOver := ring.verifier()
	handedOver.AnswerAs = "someone-else"
	ans, ok = adapter.DeriveAnswer(iss, handedOver)
	wantUnverified(t, ans, ok, gatewayBot, true)
}

// TestDeriveHumanLoginMismatch: a HUMAN-authored record derives only for its
// own author; one naming somebody else is answered-unverified, attributed to
// the comment's real author.
func TestDeriveHumanLoginMismatch(t *testing.T) {
	body := contract.AnswerRecord{Login: answerAs, OptionID: "opt-a", OptionsRev: "rev1"}.Marshal("Decision: `opt-a`")
	iss := closedWith(adapter.RawComment{Body: body, CreatedAt: posted, Login: "mallory"})
	ans, ok := adapter.DeriveAnswer(iss, keyRing{}.verifier())
	wantUnverified(t, ans, ok, "mallory", false)

	iss = closedWith(adapter.RawComment{Body: body, CreatedAt: posted, Login: "Operator"})
	ans, ok = adapter.DeriveAnswer(iss, keyRing{}.verifier())
	if !ok || ans == nil || ans.Unverified || ans.By.Name != answerAs || ans.Option == nil || *ans.Option != "opt-a" {
		t.Fatalf("own-author record = (%+v, %v), want operator's answer", ans, ok)
	}
}

// TestDeriveSubSecondIatSameSecond pins finding C1 through the poll: an intent
// signed at a fractional second and posted 200ms later has a created_at that
// GitHub reports truncated to the second, before iat. The genuine record must
// still derive the human's verified answer.
func TestDeriveSubSecondIatSameSecond(t *testing.T) {
	pub, priv := newKey(t)
	ring := keyRing{answerAs: {pub}}
	iat := time.Date(2026, 10, 1, 12, 0, 0, 700*int(time.Millisecond), time.UTC)
	si := contract.SignedIntent{
		Action: contract.ActionAnswer, Issue: thisIssue, Login: answerAs,
		OptionID: "opt-a", OptionsRev: "rev1", IssuedAt: iat,
		ExpiresAt: iat.Add(5 * time.Minute), Nonce: "nonce-0000000000000001",
	}
	si.Sign(priv)
	rec := contract.AnswerRecord{Login: answerAs, OptionID: si.OptionID, OptionsRev: si.OptionsRev, Signed: contract.SignedRecordFor(si, contract.KeyFingerprint(pub))}
	createdAt := iat.Add(200 * time.Millisecond).Truncate(time.Second)

	ans, ok := adapter.DeriveAnswer(closedWith(botComment(rec.Marshal("Decision: `opt-a`"), createdAt)), ring.verifier())
	if !ok || ans == nil || ans.Unverified || ans.Option == nil || *ans.Option != "opt-a" {
		t.Fatalf("DeriveAnswer(genuine, second-truncated created_at) = (%+v, %v), want the human's verified answer", ans, ok)
	}
}
