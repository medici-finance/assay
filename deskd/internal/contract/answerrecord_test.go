package contract

import "testing"

// TestAnswerRecordRoundTrip: Marshal → Parse recovers the record's marker
// fields (At is stamped by GitHub, not carried in the marker).
func TestAnswerRecordRoundTrip(t *testing.T) {
	in := AnswerRecord{Login: "operator", OptionID: "ship-it", OptionsRev: "abc123def456"}
	body := in.Marshal("Decision: `ship-it` — go ahead.")
	got, status := ParseAnswerRecord(body)
	if status != Answered {
		t.Fatalf("status = %s, want answered", status)
	}
	if got.Login != in.Login || got.OptionID != in.OptionID || got.OptionsRev != in.OptionsRev {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, in)
	}
}

// TestAnswerRecordNoMarker: a prose-only body (today's the answer flow close-as-answer
// case) parses as NoMarker — unchanged, never invented as an answer record.
func TestAnswerRecordNoMarker(t *testing.T) {
	_, status := ParseAnswerRecord("Closing this — went with the obvious option, no ceremony.")
	if status != NoMarker {
		t.Fatalf("status = %s, want no-marker for a prose body", status)
	}
}

// TestAnswerRecordMalformedIsStillAnswered: a body carrying the marker
// but a malformed record parses as answered-unparsed — the person DID answer;
// it is NEVER rounded down to unanswered (§10 malformed rule).
func TestAnswerRecordMalformedIsStillAnswered(t *testing.T) {
	cases := map[string]string{
		"missing login":       "<!-- assay:answer v1 option=ship-it options_rev=abc123 -->\ngo",
		"missing options_rev": "<!-- assay:answer v1 login=operator option=ship-it -->\ngo",
		"unterminated marker": "<!-- assay:answer v1 login=operator option=ship-it options_rev=abc123",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, status := ParseAnswerRecord(body)
			if status == NoMarker {
				t.Fatalf("a body with the marker parsed as no-marker (%s) — a present marker must never round down to unanswered", name)
			}
			if status != AnsweredUnparsed {
				t.Fatalf("status = %s, want answered-unparsed for %s", status, name)
			}
		})
	}
}

// TestAnswerRecordEmbeddedInLongBody: the marker is found even when it is not
// at the start of the body (a comment with preamble).
func TestAnswerRecordEmbeddedInLongBody(t *testing.T) {
	body := "Thanks for the options.\n\n<!-- assay:answer v1 login=ian option=hold options_rev=deadbeef00 -->\nHold for now."
	got, status := ParseAnswerRecord(body)
	if status != Answered {
		t.Fatalf("status = %s, want answered", status)
	}
	if got.Login != "ian" || got.OptionID != "hold" || got.OptionsRev != "deadbeef00" {
		t.Fatalf("embedded parse mismatch: %+v", got)
	}
}
