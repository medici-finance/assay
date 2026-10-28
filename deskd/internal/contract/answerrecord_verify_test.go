package contract

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"
)

const verifyTestIssue = "https://github.com/owner/name/issues/7"

// signedRecordFixture is an intent-path record exactly as the gateway writes
// it (Marshal → the comment body), parsed back the way the poll reads it.
func signedRecordFixture(t *testing.T, mutate func(*SignedIntent)) (AnswerRecord, ed25519.PublicKey, time.Time) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	iat := time.Date(2026, 10, 1, 12, 0, 0, 123456789, time.UTC)
	si := SignedIntent{
		Action: ActionAnswer, Issue: verifyTestIssue, Login: "operator",
		OptionID: "ship-it", OptionsRev: "abc123", IssuedAt: iat,
		ExpiresAt: iat.Add(5 * time.Minute), Nonce: "nonce-0000000000000001",
	}
	if mutate != nil {
		mutate(&si)
	}
	si.Sign(priv)
	in := AnswerRecord{Login: si.Login, OptionID: si.OptionID, OptionsRev: si.OptionsRev, Signed: SignedRecordFor(si, KeyFingerprint(pub))}
	rec, status := ParseAnswerRecord(in.Marshal("Decision: `ship-it`"))
	if status != Answered {
		t.Fatalf("status = %s, want answered", status)
	}
	return rec, pub, iat.Add(time.Second)
}

// TestCredentialMarkerByteStable: a credential-path record (no Signed) still
// marshals to EXACTLY the answer-record grammar marker — the item (4) extension changes
// nothing for it.
func TestCredentialMarkerByteStable(t *testing.T) {
	got := AnswerRecord{Login: "operator", OptionID: "ship-it", OptionsRev: "abc123"}.Marshal("Decision: `ship-it`")
	want := "<!-- assay:answer v1 login=operator option=ship-it options_rev=abc123 -->\nDecision: `ship-it`"
	if got != want {
		t.Fatalf("credential marker changed:\n got %q\nwant %q", got, want)
	}
	rec, _ := ParseAnswerRecord(got)
	if rec.Signed != nil {
		t.Fatalf("credential marker parsed a signed record: %+v", rec.Signed)
	}
}

// TestSignedMarkerRoundTrip: an intent-path record verifies against the key it
// was signed with and this escalation's binding.
func TestSignedMarkerRoundTrip(t *testing.T) {
	rec, pub, at := signedRecordFixture(t, nil)
	want := AnswerBinding{AnswerAs: "operator", Issue: verifyTestIssue, PostedAt: at}
	if err := VerifyAnswerRecord(rec, []ed25519.PublicKey{pub}, want); err != nil {
		t.Fatalf("VerifyAnswerRecord(valid) = %v", err)
	}
}

// TestVerifyAnswerRecordRefusals is item (5)'s checks one at a time, each
// refused with ITS sentinel (all wrapping ErrRecordUnverified).
func TestVerifyAnswerRecordRefusals(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*SignedIntent)
		edit   func(*AnswerRecord, *[]ed25519.PublicKey, *AnswerBinding)
		want   error
	}{
		{name: "credential record", edit: func(r *AnswerRecord, _ *[]ed25519.PublicKey, _ *AnswerBinding) { r.Signed = nil }, want: ErrRecordUnsigned},
		{name: "missing sig", edit: func(r *AnswerRecord, _ *[]ed25519.PublicKey, _ *AnswerBinding) { r.Signed.Sig = "" }, want: ErrRecordUnsigned},
		{name: "via other", edit: func(r *AnswerRecord, _ *[]ed25519.PublicKey, _ *AnswerBinding) { r.Signed.Via = "cookie" }, want: ErrRecordUnsigned},
		{name: "short sig", edit: func(r *AnswerRecord, _ *[]ed25519.PublicKey, _ *AnswerBinding) { r.Signed.Sig = "AAAA" }, want: ErrRecordUnsigned},
		{name: "ack action", mutate: func(si *SignedIntent) { si.Action = ActionAck }, want: ErrRecordAction},
		{name: "not answer_as", edit: func(_ *AnswerRecord, _ *[]ed25519.PublicKey, b *AnswerBinding) { b.AnswerAs = "someone-else" }, want: ErrRecordAnswerAs},
		{name: "no answer_as", edit: func(_ *AnswerRecord, _ *[]ed25519.PublicKey, b *AnswerBinding) { b.AnswerAs = "" }, want: ErrRecordAnswerAs},
		{name: "other issue", edit: func(_ *AnswerRecord, _ *[]ed25519.PublicKey, b *AnswerBinding) { b.Issue += "0" }, want: ErrRecordIssue},
		{name: "before iat", edit: func(_ *AnswerRecord, _ *[]ed25519.PublicKey, b *AnswerBinding) {
			b.PostedAt = b.PostedAt.Add(-time.Hour)
		}, want: ErrRecordWindow},
		{name: "after exp+slack", edit: func(_ *AnswerRecord, _ *[]ed25519.PublicKey, b *AnswerBinding) {
			b.PostedAt = b.PostedAt.Add(5*time.Minute + AnswerRecordWindowSlack)
		}, want: ErrRecordWindow},
		{name: "no created_at", edit: func(_ *AnswerRecord, _ *[]ed25519.PublicKey, b *AnswerBinding) { b.PostedAt = time.Time{} }, want: ErrRecordWindow},
		{name: "key revoked", edit: func(_ *AnswerRecord, k *[]ed25519.PublicKey, _ *AnswerBinding) { *k = nil }, want: ErrRecordUnknownKey},
		{name: "other key", edit: func(_ *AnswerRecord, k *[]ed25519.PublicKey, _ *AnswerBinding) {
			other, _, _ := ed25519.GenerateKey(rand.Reader)
			*k = []ed25519.PublicKey{other}
		}, want: ErrRecordUnknownKey},
		{name: "option swapped", edit: func(r *AnswerRecord, _ *[]ed25519.PublicKey, _ *AnswerBinding) { r.OptionID = "drop-it" }, want: ErrRecordBadSignature},
		{name: "rev swapped", edit: func(r *AnswerRecord, _ *[]ed25519.PublicKey, _ *AnswerBinding) { r.OptionsRev = "def456" }, want: ErrRecordBadSignature},
		{name: "nonce swapped", edit: func(r *AnswerRecord, _ *[]ed25519.PublicKey, _ *AnswerBinding) {
			r.Signed.Nonce = "nonce-0000000000000002"
		}, want: ErrRecordBadSignature},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, pub, at := signedRecordFixture(t, tc.mutate)
			keys := []ed25519.PublicKey{pub}
			want := AnswerBinding{AnswerAs: "operator", Issue: verifyTestIssue, PostedAt: at}
			if tc.edit != nil {
				tc.edit(&rec, &keys, &want)
			}
			err := VerifyAnswerRecord(rec, keys, want)
			if !errors.Is(err, tc.want) || !errors.Is(err, ErrRecordUnverified) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestRecordWindowEdgesAccepted: the window is closed at both ends — posted at
// exactly iat, and at exactly exp+slack, both verify.
func TestRecordWindowEdgesAccepted(t *testing.T) {
	rec, pub, _ := signedRecordFixture(t, nil)
	iat, _ := time.Parse(time.RFC3339Nano, rec.Signed.IssuedAt)
	exp, _ := time.Parse(time.RFC3339Nano, rec.Signed.ExpiresAt)
	for _, at := range []time.Time{iat, exp.Add(AnswerRecordWindowSlack)} {
		want := AnswerBinding{AnswerAs: "operator", Issue: verifyTestIssue, PostedAt: at}
		if err := VerifyAnswerRecord(rec, []ed25519.PublicKey{pub}, want); err != nil {
			t.Fatalf("posted at %s: %v", at, err)
		}
	}
	if !strings.HasPrefix(rec.Signed.Key, "SHA256:") {
		t.Fatalf("key= %q is not the SHA256 fingerprint form", rec.Signed.Key)
	}
}

// TestRecordWindowSecondTruncated pins finding C1 and its class: GitHub reports
// a comment's created_at at whole-second resolution while iat is carried at
// nanoseconds, so a genuine record posted shortly after a fractional iat has a
// created_at that truncates to BEFORE iat. The lower edge must be compared at
// created_at's resolution. The sweep covers every iat fraction and sign-to-post
// delay that stays inside the same wall-clock second, plus the first one that
// does not (created_at then names the next second, which is later still).
func TestRecordWindowSecondTruncated(t *testing.T) {
	fracs := []time.Duration{0, time.Nanosecond, time.Millisecond, 123456789 * time.Nanosecond, 700 * time.Millisecond, 999999999 * time.Nanosecond}
	delays := []time.Duration{0, 50 * time.Millisecond, 200 * time.Millisecond, 900 * time.Millisecond, 1500 * time.Millisecond}
	for _, f := range fracs {
		for _, d := range delays {
			base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
			iat := base.Add(f)
			rec, pub, _ := signedRecordFixture(t, func(si *SignedIntent) {
				si.IssuedAt = iat
				si.ExpiresAt = iat.Add(5 * time.Minute)
			})
			createdAt := iat.Add(d).Truncate(time.Second)
			want := AnswerBinding{AnswerAs: "operator", Issue: verifyTestIssue, PostedAt: createdAt}
			if err := VerifyAnswerRecord(rec, []ed25519.PublicKey{pub}, want); err != nil {
				t.Errorf("iat=.%09d delay=%s created_at=%s (second-truncated): %v", f.Nanoseconds(), d, createdAt.Format(time.RFC3339), err)
			}
		}
	}
}

// TestRecordWindowBeforeIatSecondRefused is the other half of C1: truncating
// the comparison to created_at's resolution must not admit a comment from a
// whole second before iat's second.
func TestRecordWindowBeforeIatSecondRefused(t *testing.T) {
	for _, f := range []time.Duration{0, 700 * time.Millisecond, 999999999 * time.Nanosecond} {
		iat := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC).Add(f)
		rec, pub, _ := signedRecordFixture(t, func(si *SignedIntent) {
			si.IssuedAt = iat
			si.ExpiresAt = iat.Add(5 * time.Minute)
		})
		for _, at := range []time.Time{iat.Truncate(time.Second).Add(-time.Second), iat.Truncate(time.Second).Add(-time.Nanosecond), iat.Add(-time.Hour)} {
			want := AnswerBinding{AnswerAs: "operator", Issue: verifyTestIssue, PostedAt: at}
			if err := VerifyAnswerRecord(rec, []ed25519.PublicKey{pub}, want); !errors.Is(err, ErrRecordWindow) {
				t.Errorf("iat=.%09d created_at=%s: err = %v, want ErrRecordWindow", f.Nanoseconds(), at.Format(time.RFC3339Nano), err)
			}
		}
	}
}
