package deskkit

import (
	"errors"
	"strings"
	"testing"
)

func TestDispositionVocabularyIsClosed(t *testing.T) {
	for _, in := range []string{"superseded", "SUPERSEDED", " Superseded ", "disposition:superseded"} {
		v, err := ParseDispositionVerdict(in)
		if err != nil || v != DispositionSuperseded {
			t.Errorf("ParseDispositionVerdict(%q) = %q, %v; want SUPERSEDED", in, v, err)
		}
	}
	// Positive control: a plausible-but-unlisted verdict must be REFUSED, not coerced.
	// A typo that silently became valid would write a wrong terminal record on a live PR.
	for _, bad := range []string{"", "supersedes", "STALE", "close", "wontfix", "resolved"} {
		if _, err := ParseDispositionVerdict(bad); err == nil {
			t.Errorf("ParseDispositionVerdict(%q) accepted a word outside the vocabulary", bad)
		} else if ExitCodeOf(err) != ExitRefused {
			t.Errorf("ParseDispositionVerdict(%q) must be exit 5 refused, got %d", bad, ExitCodeOf(err))
		}
	}
}

func TestDispositionSuppressesDispatch(t *testing.T) {
	if !DispositionSuperseded.SuppressesDispatch() || !DispositionResolvedElsewhere.SuppressesDispatch() {
		t.Error("the terminal verdicts must suppress dispatch — that is the whole #728 fix")
	}
	if DispositionNeedsRebase.SuppressesDispatch() {
		t.Error("NEEDS-REBASE is live work: a worker can still act on it")
	}
}

func TestDispositionLabelRoundTrip(t *testing.T) {
	for _, v := range DispositionVerdicts() {
		got, ok := ParseDispositionLabel(v.Label())
		if !ok || got != v {
			t.Errorf("label round-trip failed for %s: %q -> %q, ok=%t", v, v.Label(), got, ok)
		}
	}
	for _, l := range []string{"bug", "question", "help wanted", "dispositional", ""} {
		if _, ok := ParseDispositionLabel(l); ok {
			t.Errorf("%q must not read as a disposition label", l)
		}
	}
}

func TestDispositionValidateRequiresEvidence(t *testing.T) {
	// Positive control: a terminal verdict with no evidence is exactly the
	// unfalsifiable prose comment the record replaces, and must be refused.
	d := Disposition{Verdict: DispositionSuperseded, RecordedAt: "2026-08-13"}
	if err := d.Validate(); err == nil {
		t.Fatal("SUPERSEDED with no evidence must be refused")
	}
	d.Evidence = "https://github.com/x/y/pull/223"
	if err := d.Validate(); err != nil {
		t.Fatalf("a complete record must validate: %v", err)
	}
	// NEEDS-REBASE is not terminal, so it needs no evidence link.
	if err := (Disposition{Verdict: DispositionNeedsRebase, RecordedAt: "2026-08-13"}).Validate(); err != nil {
		t.Fatalf("NEEDS-REBASE without evidence must validate: %v", err)
	}
	if err := (Disposition{Verdict: DispositionNeedsRebase}).Validate(); err == nil {
		t.Fatal("a record with no date must be refused")
	}
}

func TestDispositionMarkerRoundTrip(t *testing.T) {
	want := Disposition{
		Verdict:    DispositionSuperseded,
		Evidence:   "https://github.com/example-org/tracker/pull/223",
		RecordedBy: "gt05-worker",
		RecordedAt: "2026-08-13",
	}
	got, ok := ParseDispositionMarker(want.Marker())
	if !ok {
		t.Fatalf("the rendered marker must parse back; body:\n%s", want.Marker())
	}
	if got != want {
		t.Errorf("round-trip lost fields: got %+v want %+v", got, want)
	}
	if !strings.Contains(want.Marker(), "deskclose") {
		t.Error("a terminal record must say the close is deskclose's, not this record's")
	}
}

func TestParseDispositionMarkerNegatives(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"prose recommendation — the #728 defect itself", "Investigated: this is superseded by #223, recommend close."},
		{"envelope with no verdict line", DispositionMarkerOpen + "\nEvidence: https://x/y/pull/1\n"},
		{"envelope with an out-of-vocabulary verdict", DispositionMarkerOpen + "\nDisposition: STALE\n"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := ParseDispositionMarker(tc.body); ok {
				t.Errorf("must not read as a record: %q", tc.body)
			}
		})
	}
}

// TestParseDispositionMarkerSkipsFencedBlocks is the doc-quoting control: this very
// schema is quoted verbatim in docs/brief-rules.md and in two skills. A fenced example
// must never read as a live verdict on the PR that adds the documentation.
func TestParseDispositionMarkerSkipsFencedBlocks(t *testing.T) {
	body := "Here is the schema:\n\n```\n" + DispositionMarkerOpen +
		"\nDisposition: SUPERSEDED\nEvidence: https://example/1\n```\n\nThat is all."
	if _, ok := ParseDispositionMarker(body); ok {
		t.Error("a fenced documentation example must not register as a disposition")
	}
}

func TestReadDispositionThreeState(t *testing.T) {
	label := DispositionSuperseded.Label()
	rec := Disposition{Verdict: DispositionSuperseded, Evidence: "https://x/y/pull/223", RecordedAt: "2026-08-13"}

	t.Run("checked-clean: read succeeded, nothing found, dispatchable", func(t *testing.T) {
		r := ReadDisposition([]string{"bug"}, []string{"a normal review comment"}, nil)
		if r.State != DispositionCheckedClean || !r.DispatchEligible() {
			t.Fatalf("got %+v; want checked-clean and dispatch-eligible", r)
		}
	})

	t.Run("checked-failed: a full record suppresses dispatch", func(t *testing.T) {
		r := ReadDisposition([]string{label}, []string{rec.Marker()}, nil)
		if r.State != DispositionCheckedFailed {
			t.Fatalf("got state %q; want checked-failed", r.State)
		}
		if r.DispatchEligible() {
			t.Fatal("a SUPERSEDED PR must never be handed to a fresh worker — this is the 8-of-10 waste")
		}
		if r.Record.Evidence != rec.Evidence {
			t.Errorf("deskclose needs the evidence link; got %q", r.Record.Evidence)
		}
	})

	t.Run("could-not-check: a read error is never reported as clean", func(t *testing.T) {
		r := ReadDisposition(nil, nil, errors.New("API rate limit exceeded"))
		if r.State != DispositionCouldNotCheck {
			t.Fatalf("got state %q; want could-not-check", r.State)
		}
		if r.DispatchEligible() {
			t.Fatal("an instrument that could not read must not answer the question it was asked")
		}
	})

	t.Run("could-not-check wins over partial data", func(t *testing.T) {
		r := ReadDisposition([]string{label}, []string{rec.Marker()}, errors.New("boom"))
		if r.State != DispositionCouldNotCheck {
			t.Fatalf("a dropped read error is how a false green happens; got %+v", r)
		}
	})

	t.Run("NEEDS-REBASE stays dispatchable", func(t *testing.T) {
		nr := Disposition{Verdict: DispositionNeedsRebase, RecordedAt: "2026-08-13"}
		r := ReadDisposition([]string{DispositionNeedsRebase.Label()}, []string{nr.Marker()}, nil)
		if r.State != DispositionCheckedFailed || !r.DispatchEligible() {
			t.Fatalf("got %+v; want checked-failed but still dispatchable", r)
		}
	})

	t.Run("label without record: verdict honoured, evidence gap reported", func(t *testing.T) {
		r := ReadDisposition([]string{label}, []string{"just prose"}, nil)
		if r.DispatchEligible() {
			t.Fatal("ignoring a bare label would re-dispatch the PR the label protects")
		}
		if !strings.Contains(r.Reason, "no evidence") {
			t.Errorf("the evidence gap must be reported for deskclose; got %q", r.Reason)
		}
	})

	t.Run("record without label: index gap reported", func(t *testing.T) {
		r := ReadDisposition([]string{"bug"}, []string{rec.Marker()}, nil)
		if r.DispatchEligible() {
			t.Fatal("a full record must suppress dispatch even when the label is missing")
		}
		if !strings.Contains(r.Reason, "label") {
			t.Errorf("the missing index must be reported; got %q", r.Reason)
		}
	})

	t.Run("disagreement resolves to the more restrictive answer", func(t *testing.T) {
		nr := Disposition{Verdict: DispositionNeedsRebase, RecordedAt: "2026-08-13"}
		r := ReadDisposition([]string{label}, []string{nr.Marker()}, nil)
		if r.DispatchEligible() {
			t.Fatal("when either half says terminal, the cheap direction to be wrong in is 'wait for a human'")
		}
		if !strings.Contains(r.Reason, "disagreement") {
			t.Errorf("a writer bug must be named, not silently resolved; got %q", r.Reason)
		}
	})

	t.Run("the LAST marker wins", func(t *testing.T) {
		first := Disposition{Verdict: DispositionNeedsRebase, RecordedAt: "2026-08-10"}
		r := ReadDisposition(nil, []string{first.Marker(), rec.Marker()}, nil)
		if r.Record.Verdict != DispositionSuperseded {
			t.Fatalf("a worker must be able to supersede its own earlier record; got %q", r.Record.Verdict)
		}
	})
}

func TestReadDispositionIndex(t *testing.T) {
	t.Run("labels only is enough to suppress dispatch", func(t *testing.T) {
		r := ReadDispositionIndex([]string{"bug", DispositionSuperseded.Label()}, nil)
		if r.State != DispositionCheckedFailed || r.DispatchEligible() {
			t.Fatalf("got %+v; want checked-failed and not dispatch-eligible", r)
		}
	})
	t.Run("no disposition label is a real candidate", func(t *testing.T) {
		r := ReadDispositionIndex([]string{"bug", "question"}, nil)
		if r.State != DispositionCheckedClean || !r.DispatchEligible() {
			t.Fatalf("got %+v; want checked-clean and dispatch-eligible", r)
		}
	})
	t.Run("a failed list read is could-not-check, not an empty queue", func(t *testing.T) {
		r := ReadDispositionIndex(nil, errors.New("403 secondary rate limit"))
		if r.State != DispositionCouldNotCheck || r.DispatchEligible() {
			t.Fatalf("got %+v; want could-not-check and not dispatch-eligible", r)
		}
	})
	t.Run("two labels resolve to the restrictive one", func(t *testing.T) {
		r := ReadDispositionIndex([]string{DispositionNeedsRebase.Label(), DispositionSuperseded.Label()}, nil)
		if r.DispatchEligible() {
			t.Fatalf("got %+v; a terminal label present must win", r)
		}
	})
}
