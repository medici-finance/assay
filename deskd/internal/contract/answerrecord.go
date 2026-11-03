package contract

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// AnswerRecord is the machine-readable half of a human answer, carried in the
// GitHub write body as an HTML comment marker so the service can parse the
// outcome back off GitHub on the next poll (§4: GitHub is the record). Its
// grammar is fixed by the answer-record grammar and unchanged from the answer flow:
//
//	<!-- assay:answer v1 login=<login> option=<id> options_rev=<R> -->
//
// The marker is followed by the human-readable decision line. At is the time
// the comment was authored; it is not carried in the marker (GitHub stamps it)
// and is populated by the parser's caller, not by [ParseAnswerRecord].
//
// Signed is the signed-intent extension (the authorization contract gap 3, item (4)):
// nil on a credential-path record, which therefore stays byte-identical to the
// the answer-record grammar grammar above; set on an intent-path record, whose marker then
// carries the fields [VerifyAnswerRecord] rebuilds the signed bytes from.
type AnswerRecord struct {
	Login      string
	OptionID   domain.OptionID
	OptionsRev string
	At         time.Time
	Signed     *SignedRecord
}

// SignedRecord is the item (4) extension of the answer marker, appended after
// the three v1 fields on an intent-path record:
//
//	via=intent action=<a> issue=<deep-link> iat=<RFC3339Nano> exp=<RFC3339Nano>
//	nonce=<n> key=<SHA-256 key fingerprint> sig=<base64url ed25519 signature>
//
// Together with login/option/options_rev these are exactly the fields
// [SignedIntent.signingBytes] covers, so anyone holding the enrolled-key file
// can rebuild the signed bytes and check the record without the service. The
// values are kept as the RAW marker text: [VerifyAnswerRecord] parses them, so
// a malformed field is a verification failure, never a parse-time guess.
type SignedRecord struct {
	Via       string
	Action    string
	Issue     string
	IssuedAt  string
	ExpiresAt string
	Nonce     string
	Key       string
	Sig       string
}

// SignedRecordVia is the only accepted `via=` value.
const SignedRecordVia = "intent"

// SignedRecordFor renders the item (4) fields for a verified intent si whose
// signature verified under the key with fingerprint keyFingerprint.
func SignedRecordFor(si SignedIntent, keyFingerprint string) *SignedRecord {
	return &SignedRecord{
		Via:       SignedRecordVia,
		Action:    string(si.Action),
		Issue:     si.Issue,
		IssuedAt:  si.IssuedAt.UTC().Format(time.RFC3339Nano),
		ExpiresAt: si.ExpiresAt.UTC().Format(time.RFC3339Nano),
		Nonce:     si.Nonce,
		Key:       keyFingerprint,
		Sig:       base64.RawURLEncoding.EncodeToString(si.Signature),
	}
}

// KeyFingerprint is the `key=` form of an ed25519 public key:
// "SHA256:" + unpadded standard base64 of SHA-256(pub) — the same form
// config/intentkeys.Key.Fingerprint prints and the audit line records.
func KeyFingerprint(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

const answerMarkerOpen = "<!-- assay:answer v1"
const answerMarkerClose = "-->"

// Marshal renders the record as the marker line followed by humanLine. The
// marker is deterministic (fixed key order) so a round-trip is stable.
func (r AnswerRecord) Marshal(humanLine string) string {
	signed := ""
	if s := r.Signed; s != nil {
		signed = fmt.Sprintf(" via=%s action=%s issue=%s iat=%s exp=%s nonce=%s key=%s sig=%s",
			s.Via, s.Action, s.Issue, s.IssuedAt, s.ExpiresAt, s.Nonce, s.Key, s.Sig)
	}
	marker := fmt.Sprintf("%s login=%s option=%s options_rev=%s%s %s",
		answerMarkerOpen, r.Login, r.OptionID, r.OptionsRev, signed, answerMarkerClose)
	if humanLine == "" {
		return marker
	}
	return marker + "\n" + humanLine
}

// ParseStatus is the three-way outcome of parsing a GitHub write body for an
// answer record. It is deliberately three-valued: a body carrying the marker
// but a malformed record is ANSWERED (the person did answer), record unparsed —
// it is never rounded down to unanswered.
type ParseStatus int

const (
	// NoMarker: the body carries no answer marker. The outcome is derived from
	// today's prose (close ⇒ answered by prose), unchanged from the answer flow.
	NoMarker ParseStatus = iota
	// Answered: the marker is present and the record parsed.
	Answered
	// AnsweredUnparsed: the marker is present but the record did not parse —
	// answered, record unparsed. NEVER treated as unanswered (§10 malformed
	// rule: render what is there, do not drop).
	AnsweredUnparsed
)

func (s ParseStatus) String() string {
	switch s {
	case Answered:
		return "answered"
	case AnsweredUnparsed:
		return "answered-unparsed"
	default:
		return "no-marker"
	}
}

// ParseAnswerRecord scans a GitHub write body for the answer marker and derives
// the record. It returns:
//
//   - (zero, NoMarker) when no marker is present;
//   - (record, Answered) when the marker parses;
//   - (partial, AnsweredUnparsed) when the marker is present but a required
//     field (login, options_rev) is missing/empty — answered, record unparsed.
//
// A malformed option slug is preserved as-is in the returned record but does
// NOT by itself force AnsweredUnparsed: the option's validity is the caller's
// to judge against the escalation. The load-bearing property is that a present
// marker never parses as NoMarker.
func ParseAnswerRecord(body string) (AnswerRecord, ParseStatus) {
	open := strings.Index(body, answerMarkerOpen)
	if open < 0 {
		return AnswerRecord{}, NoMarker
	}
	rest := body[open+len(answerMarkerOpen):]
	close := strings.Index(rest, answerMarkerClose)
	if close < 0 {
		// An unterminated marker is still a marker: the person answered.
		return AnswerRecord{}, AnsweredUnparsed
	}
	fields := parseMarkerFields(rest[:close])
	rec := AnswerRecord{
		Login:      fields["login"],
		OptionID:   domain.OptionID(fields["option"]),
		OptionsRev: fields["options_rev"],
	}
	if signed := signedFields(fields); signed != nil {
		rec.Signed = signed
	}
	// A record with no login or no options_rev cannot be trusted as parsed,
	// but the marker's presence proves an answer — report answered-unparsed.
	if rec.Login == "" || rec.OptionsRev == "" {
		return rec, AnsweredUnparsed
	}
	return rec, Answered
}

// signedFields lifts the item (4) fields off a parsed marker. Any one of them
// present yields a SignedRecord (possibly incomplete — [VerifyAnswerRecord]
// refuses an incomplete one); none present yields nil, the credential-path
// shape.
func signedFields(f map[string]string) *SignedRecord {
	s := SignedRecord{
		Via: f["via"], Action: f["action"], Issue: f["issue"], IssuedAt: f["iat"],
		ExpiresAt: f["exp"], Nonce: f["nonce"], Key: f["key"], Sig: f["sig"],
	}
	if s == (SignedRecord{}) {
		return nil
	}
	return &s
}

// AnswerRecordWindowSlack is how long after a record's `exp` its comment may
// still carry a GitHub created_at and verify: the write lands after the
// service accepted the intent, never before `iat`, and within this slack of
// `exp` (gap-3 item (5)).
const AnswerRecordWindowSlack = 120 * time.Second

// Refusals from [VerifyAnswerRecord]. Each wraps [ErrRecordUnverified] and
// names ONE check, so a test (and a log line) can tell which binding failed.
var (
	ErrRecordUnverified   = errors.New("answer record: not verified as the named login's signed answer")
	ErrRecordUnsigned     = fmt.Errorf("%w: a signed-intent field is missing or malformed (via, action, issue, iat, exp, nonce, key, sig)", ErrRecordUnverified)
	ErrRecordAction       = fmt.Errorf("%w: the signed action is not %q", ErrRecordUnverified, ActionAnswer)
	ErrRecordAnswerAs     = fmt.Errorf("%w: the record's login is not the cell's current answer_as", ErrRecordUnverified)
	ErrRecordIssue        = fmt.Errorf("%w: the record's issue= is not this escalation's deep link", ErrRecordUnverified)
	ErrRecordWindow       = fmt.Errorf("%w: the comment's created_at is outside [iat, exp+%s]", ErrRecordUnverified, AnswerRecordWindowSlack)
	ErrRecordUnknownKey   = fmt.Errorf("%w: no key enrolled for the record's login has the record's key= fingerprint", ErrRecordUnverified)
	ErrRecordBadSignature = fmt.Errorf("%w: the signature does not verify under the enrolled key", ErrRecordUnverified)
)

// AnswerBinding is what a record must be bound to, as the POLL knows it — never
// as the record claims it.
type AnswerBinding struct {
	// AnswerAs is the cell's CURRENT answer_as (evaluated at poll time, so an
	// answer_as handover un-derives earlier intent-path records).
	AnswerAs string
	// Issue is this escalation's platform deep link.
	Issue string
	// PostedAt is the comment's GitHub created_at (whole-second resolution).
	PostedAt time.Time
}

// VerifyAnswerRecord is the poll-side re-verification of an intent-path
// answer record (the authorization contract gap 3, item (5)). enrolled is the set of
// keys enrolled for rec.Login in the key file AS LOADED. A nil error means the
// record is the named login's signed answer to THIS escalation: every item (4)
// field is present, action=answer, login= is the cell's current answer_as,
// issue= is this escalation's deep link, the comment was posted within
// [iat, exp+AnswerRecordWindowSlack] (the lower edge at the forge's
// whole-second created_at resolution: iat floored to its second), and the signature over the rebuilt
// canonical bytes verifies under the enrolled key whose fingerprint is key=.
// option/options_rev need no separate check: they are inside the signed bytes.
//
// It deliberately does NOT reuse [SignedIntent.Verify]: that checks "now"
// against exp, which is the service's question at write time; the poll's
// question is when GitHub says the comment was posted.
func VerifyAnswerRecord(rec AnswerRecord, enrolled []ed25519.PublicKey, want AnswerBinding) error {
	s := rec.Signed
	if s == nil || s.Via != SignedRecordVia || rec.Login == "" || rec.OptionsRev == "" ||
		s.Action == "" || s.Issue == "" || s.IssuedAt == "" || s.ExpiresAt == "" ||
		s.Nonce == "" || s.Key == "" || s.Sig == "" {
		return ErrRecordUnsigned
	}
	iat, err := time.Parse(time.RFC3339Nano, s.IssuedAt)
	if err != nil {
		return fmt.Errorf("%w: iat: %v", ErrRecordUnsigned, err)
	}
	exp, err := time.Parse(time.RFC3339Nano, s.ExpiresAt)
	if err != nil {
		return fmt.Errorf("%w: exp: %v", ErrRecordUnsigned, err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(s.Sig)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("%w: sig is not a base64url ed25519 signature", ErrRecordUnsigned)
	}
	if Action(s.Action) != ActionAnswer {
		return fmt.Errorf("%w: got %q", ErrRecordAction, s.Action)
	}
	if want.AnswerAs == "" || !strings.EqualFold(rec.Login, want.AnswerAs) {
		return fmt.Errorf("%w: login=%q answer_as=%q", ErrRecordAnswerAs, rec.Login, want.AnswerAs)
	}
	if want.Issue == "" || s.Issue != want.Issue {
		return fmt.Errorf("%w: issue=%q this=%q", ErrRecordIssue, s.Issue, want.Issue)
	}
	// created_at is whole-second on the forge while iat is nanosecond, so the
	// lower edge is compared at the coarser resolution: iat floored to its
	// second. A comment posted in iat's own second (created_at truncated below
	// iat) verifies; one from an earlier second still does not. The upper edge
	// needs no such care: truncation only moves created_at earlier.
	if want.PostedAt.IsZero() || want.PostedAt.Before(iat.Truncate(time.Second)) || want.PostedAt.After(exp.Add(AnswerRecordWindowSlack)) {
		return fmt.Errorf("%w: created_at=%s iat=%s exp=%s", ErrRecordWindow,
			want.PostedAt.UTC().Format(time.RFC3339Nano), s.IssuedAt, s.ExpiresAt)
	}
	var pub ed25519.PublicKey
	for _, k := range enrolled {
		if len(k) == ed25519.PublicKeySize && KeyFingerprint(k) == s.Key {
			pub = k
			break
		}
	}
	if pub == nil {
		return fmt.Errorf("%w: key=%s", ErrRecordUnknownKey, s.Key)
	}
	si := SignedIntent{
		Action: Action(s.Action), Issue: s.Issue, Login: rec.Login, OptionID: rec.OptionID,
		OptionsRev: rec.OptionsRev, IssuedAt: iat, ExpiresAt: exp, Nonce: s.Nonce,
	}
	if !ed25519.Verify(pub, si.signingBytes(), sig) {
		return ErrRecordBadSignature
	}
	return nil
}

// parseMarkerFields splits "k=v k=v" pairs. Values run to the next space; the
// marker grammar uses no spaces inside a value (login, slug, hex rev).
func parseMarkerFields(s string) map[string]string {
	out := map[string]string{}
	for _, tok := range strings.Fields(s) {
		k, v, ok := strings.Cut(tok, "=")
		if !ok || v == "" {
			continue
		}
		out[k] = v
	}
	return out
}
