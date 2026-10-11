package deskkit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The fixture ask: three options in the body, B marked recommended, so the inbox walk
// re-letters them B→A, A→B, C→C. Logins and ids are the fixture roster's.
const (
	decBody = "<!-- needs-decision: demo-stream/07 -->\n## Context\nWhich parser fix.\n\n" +
		"## Options\nA. Ship the small fix\nB. Rewrite the parser (recommended)\nC. Do nothing\n"
	decAppLogin       = "assay-desk-app[bot]"
	decAppID    int64 = 300000001
	// The relay restates the options in the walk's order (recommended first), as the inbox
	// presented them.
	decRelayOpts = "Relayed ask.\n\n## Options\nA. Rewrite the parser (recommended)\nB. Ship the small fix\nC. Do nothing\n"
)

func decComment(id int64, login string, uid int64, at, body string) Comment {
	return Comment{DatabaseID: id, Author: Account{Login: login, ID: uid}, CreatedAt: at, Body: body}
}

func TestParseDecisionOptions(t *testing.T) {
	o := ParseDecisionOptions(decBody)
	if !reflect.DeepEqual(o.Letters, []string{"A", "B", "C"}) || o.Recommended != 1 {
		t.Fatalf("letters %v recommended %d, want [A B C] 1", o.Letters, o.Recommended)
	}
	if !reflect.DeepEqual(o.Texts, []string{"Ship the small fix", "Rewrite the parser", "Do nothing"}) {
		t.Fatalf("texts %q", o.Texts)
	}
	if !reflect.DeepEqual(o.Walk(), []int{1, 0, 2}) {
		t.Fatalf("walk %v, want [1 0 2]", o.Walk())
	}
	none := ParseDecisionOptions("## Options\n- a) one\n- b) two\n")
	if none.Recommended != -1 || !reflect.DeepEqual(none.Letters, []string{"A", "B"}) {
		t.Fatalf("unmarked: %+v", none)
	}
	if ParseDecisionOptions("no options here\nA. not under a heading\n").Stated() {
		t.Fatal("options outside an Options section were read")
	}
}

// regression: medici-finance/assay#2426 (C-2, C-3, SEC-3, SEC-4, SEC-6) — every row below
// that a person would read as anything but ONE definite option must not record one.
func TestParseRulingPick(t *testing.T) {
	relay := func(login string, uid int64, extra string) Comment {
		return decComment(50, login, uid, "2026-10-01T10:00:00Z", decRelayOpts+extra)
	}
	app := func(id int64, at, body string) Comment { return decComment(id, decAppLogin, decAppID, at, body) }
	minimized := func(c Comment) Comment { c.Minimized = true; return c }
	numBody := "## Options\n1. Ship in two days\n2. Ship in three days\n3. Do not ship\n"
	cases := []struct {
		name       string
		body       string    // "" = decBody
		thread     []Comment // before the ruling
		ruling     string
		wantPicked string
		wantVia    string
	}{
		{"direct letter", "", nil, "B", "B", DecisionViaDirect},
		{"option word", "", nil, "Option C — leave it as it stands", "C", DecisionViaDirect},
		{"ratified via relay", "",
			[]Comment{relay(decAppLogin, decAppID, "\nAnswer: A\n")}, "ratified", "A", DecisionViaRatified},
		{"untrusted relay", "",
			[]Comment{relay("mallory", 4242, "\nAnswer: A\n")}, "ratified", DecisionPickUnparsed, DecisionViaRatified},
		{"wrong-id relay", "",
			[]Comment{relay(decAppLogin, decAppID+1, "\nAnswer: A\n")}, "I ratify", DecisionPickUnparsed, DecisionViaRatified},
		{"source vs walk", "", []Comment{relay(decAppLogin, decAppID, "")}, "B", DecisionPickAmbiguous, DecisionViaDirect},
		{"same under both", "", []Comment{relay(decAppLogin, decAppID, "")}, "C", "C", DecisionViaDirect},
		{"answer-only relay walk", "",
			[]Comment{app(51, "2026-10-01T11:00:00Z", "Ruling relayed.\nAnswer: A\n")},
			"approved", DecisionPickAmbiguous, DecisionViaRatified},
		{"question", "", nil, "B?", DecisionPickUnparsed, DecisionViaDirect},
		{"hold", "", nil, "Hold for now", DecisionPickUnparsed, DecisionViaDirect},
		{"not offered", "", nil, "D", DecisionPickUnparsed, DecisionViaDirect},
		{"no grammar", "", nil, "Looks fine to me", DecisionPickUnparsed, DecisionViaDirect},

		// C-2: a ratification that names an option, or qualifies what it ratifies.
		{"approved names B", "", nil, "Approved — B", DecisionPickUnparsed, DecisionViaRatified},
		{"approved names another than the relay", "",
			[]Comment{relay(decAppLogin, decAppID, "\nAnswer: A\n")}, "Approved — C", DecisionPickUnparsed, DecisionViaRatified},
		{"approved with changes", "", nil, "Approved with changes: do C instead", DecisionPickUnparsed, DecisionViaRatified},
		{"ratified except", "", nil, "Ratified, except use C", DecisionPickUnparsed, DecisionViaRatified},
		{"approve none", "", nil, "I approve of none of these", DecisionPickUnparsed, DecisionViaRatified},
		// C-2: a ratification never reaches past a newer ask, or past a newer word answer.
		{"re-ask after answered relay", "", []Comment{
			relay(decAppLogin, decAppID, "\nAnswer: C\n"),
			app(52, "2026-10-01T12:00:00Z", decRelayOpts),
		}, "approved", DecisionPickUnparsed, DecisionViaRatified},
		{"newer word answer", "", []Comment{
			relay(decAppLogin, decAppID, "\nAnswer: C\n"),
			app(52, "2026-10-01T12:00:00Z", "Relayed.\nAnswer: yes, go ahead\n"),
		}, "ratified", DecisionPickUnparsed, DecisionViaRatified},
		// C-2: a direct letter beside any other offered id.
		{"two letters", "", nil, "A, B", DecisionPickUnparsed, DecisionViaDirect},
		{"letter then actually", "", nil, "A. Actually B.", DecisionPickUnparsed, DecisionViaDirect},
		{"letter rejected", "", nil, "B — rejected, go with A", DecisionPickUnparsed, DecisionViaDirect},
		{"per-option lines", "", nil, "A: rejected\nB: rejected\nC: approved", DecisionPickUnparsed, DecisionViaDirect},
		{"second line changes it", "", nil, "B\nOn reflection, go with C.", DecisionPickUnparsed, DecisionViaDirect},
		{"restated options", "", nil, "A. Ship the small fix\nB. Rewrite the parser\nGo with B", DecisionPickUnparsed, DecisionViaDirect},
		{"numeric range", numBody, nil, "2-3 days of work is fine; go with 1", DecisionPickUnparsed, DecisionViaDirect},
		// C-3: a question, a hold or a qualifier after a letter.
		{"letter is it final", "", nil, "B — is that final?", DecisionPickUnparsed, DecisionViaDirect},
		{"letter hold", "", nil, "B. Hold for now.", DecisionPickUnparsed, DecisionViaDirect},
		{"approved not yet", "", nil, "Approved, but not yet", DecisionPickUnparsed, DecisionViaRatified},

		// SEC-3: a minimized comment is neither an ask nor a relay.
		{"minimized anchor", "", []Comment{minimized(relay(decAppLogin, decAppID, ""))}, "B", "B", DecisionViaDirect},
		{"minimized ratified relay", "",
			[]Comment{minimized(relay(decAppLogin, decAppID, "\nAnswer: A\n"))}, "ratified", DecisionPickUnparsed, DecisionViaRatified},
		{"minimized answer after anchor", "", []Comment{
			relay(decAppLogin, decAppID, ""),
			minimized(app(52, "2026-10-01T12:00:00Z", "Relayed.\nAnswer: C\n")),
		}, "approved", DecisionPickUnparsed, DecisionViaRatified},
		// A trusted HUMAN restating options is not an App ask: the body stays the anchor.
		{"human restatement", "", []Comment{relay(fixtureBlessLogin, fixtureBlessID, "")}, "B", "B", DecisionViaDirect},
		// SEC-6: an App comment with no account id cannot be a relay.
		{"zero-id relay", "", []Comment{relay(decAppLogin, 0, "\nAnswer: A\n")}, "ratified", DecisionPickUnparsed, DecisionViaRatified},
		// SEC-4: a quoted Answer line is someone else's text.
		{"quoted answer only", "", []Comment{relay(decAppLogin, decAppID, "\n> Answer: C\n")}, "ratified", DecisionPickUnparsed, DecisionViaRatified},
		{"quoted answer beside own", "", []Comment{relay(decAppLogin, decAppID, "\n> Answer: C\nAnswer: A\n")}, "ratified", "A", DecisionViaRatified},
		{"two different answers", "", []Comment{relay(decAppLogin, decAppID, "\nAnswer: A\nAnswer: C\n")}, "ratified", DecisionPickUnparsed, DecisionViaRatified},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.body
			if body == "" {
				body = decBody
			}
			r := decComment(90, fixtureBlessLogin, fixtureBlessID, "2026-10-02T10:00:00Z", tc.ruling)
			thread := append(append([]Comment(nil), tc.thread...), r)
			idx := len(thread) - 1
			anchor := DecisionAnchor(thread[:idx])
			picked, via := ParseRulingPick(body, thread, anchor, idx, thread[idx])
			if picked != tc.wantPicked || via != tc.wantVia {
				t.Fatalf("ruling %q: picked %q via %q, want %q via %q", tc.ruling, picked, via, tc.wantPicked, tc.wantVia)
			}
		})
	}
}

// regression: medici-finance/assay#2426 (SEC-5) — a ruling the thread does not locate has
// no "before"; no later comment may stand in for an earlier relay.
func TestParseRulingPickUnlocatedRuling(t *testing.T) {
	r := decComment(90, fixtureBlessLogin, fixtureBlessID, "2026-10-02T10:00:00Z", "ratified")
	thread := []Comment{decComment(50, decAppLogin, decAppID, "2026-10-03T10:00:00Z", decRelayOpts+"\nAnswer: A\n")}
	for _, idx := range []int{-1, len(thread), len(thread) + 3} {
		if picked, _ := ParseRulingPick(decBody, thread, -1, idx, r); picked != DecisionPickUnparsed {
			t.Fatalf("ruling index %d: picked %q, want unparsed", idx, picked)
		}
	}
	if got := DecisionRulingIndex(thread, r); got != -1 {
		t.Fatalf("DecisionRulingIndex located an absent ruling at %d", got)
	}
	rec := ComposeDecisionRecord(DecisionInput{
		Repo: "example-org/tracker", Issue: 120, Tracker: "example-org/tracker#7",
		IssueBody: decBody, IssueCreatedAt: "2026-10-01T06:00:00Z",
		Comments: thread, Ruling: r, Now: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC),
	})
	if rec.Picked != DecisionPickUnparsed || rec.OptionsSource != DecisionSourceBody {
		t.Fatalf("an unlocated ruling read the later relay: %+v", rec)
	}
}

func validDecisionRecord(t *testing.T) HumanDecisionRecord {
	t.Helper()
	rec := ComposeDecisionRecord(DecisionInput{
		Repo: "example-org/tracker", Issue: 120, Tracker: "example-org/tracker#7",
		IssueBody: decBody, IssueCreatedAt: "2026-10-01T08:00:00+02:00",
		Comments: []Comment{decComment(90, fixtureBlessLogin, fixtureBlessID, "2026-10-02T06:00:30Z", "B")},
		Ruling:   decComment(90, fixtureBlessLogin, fixtureBlessID, "2026-10-02T06:00:30Z", "B"),
		Now:      time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC),
	})
	if err := ValidateDecisionRecord(rec); err != nil {
		t.Fatalf("fixture record invalid: %v (%+v)", err, rec)
	}
	return rec
}

func TestComposeDecisionRecord(t *testing.T) {
	rec := validDecisionRecord(t)
	if rec.OpenedAt != "2026-10-01T06:00:00Z" || rec.OpenedToRuledS != 86430 || rec.AskedToRuledS != 86430 {
		t.Fatalf("timestamps not normalised to UTC or latency wrong: %+v", rec)
	}
	if rec.Picked != "B" || rec.Recommended != "B" || !rec.PickedIsRecommended || rec.OptionsSource != DecisionSourceBody {
		t.Fatalf("pick: %+v", rec)
	}
	if rec.Brief != "demo-stream/07" || rec.Ruler != DecisionRuler {
		t.Fatalf("brief/ruler: %+v", rec)
	}
	if rec.Options[1].TextSHA256 != Sha256Hex([]byte("Rewrite the parser")) {
		t.Fatal("option digest is not the cleaned text's sha256")
	}

	// A trusted relay that restates the options becomes the anchor: its letters, its time.
	relay := decComment(50, decAppLogin, decAppID, "2026-10-01T12:00:00Z", decRelayOpts+"\nAnswer: A\n")
	rul := decComment(90, fixtureBlessLogin, fixtureBlessID, "2026-10-02T12:00:00Z", "ratified")
	rr := ComposeDecisionRecord(DecisionInput{
		Repo: "example-org/tracker", Issue: 120, Tracker: "example-org/tracker#7",
		IssueBody: decBody, IssueCreatedAt: "2026-10-01T06:00:00Z",
		Comments: []Comment{relay, rul}, Ruling: rul, Now: time.Date(2026, 10, 2, 13, 0, 0, 0, time.UTC),
	})
	if err := ValidateDecisionRecord(rr); err != nil {
		t.Fatal(err)
	}
	if rr.OptionsSource != DecisionSourceRelay || rr.Recommended != "A" || rr.Picked != "A" ||
		rr.PickedVia != DecisionViaRatified || rr.AskedAt != "2026-10-01T12:00:00Z" || rr.AskedToRuledS != 86400 {
		t.Fatalf("relay-anchored record: %+v", rr)
	}
}

// regression: medici-finance/assay#2426 (brief grammar) — only a whole-line marker in the
// brief-id grammar names a brief; anything else leaves the field out and the record valid.
func TestComposeDecisionBrief(t *testing.T) {
	opts := "\n## Options\nA. Ship the small fix\nB. Rewrite the parser (recommended)\n"
	cases := []struct{ name, body, want string }{
		{"own line", "<!-- needs-decision: demo-stream/07 -->" + opts, "demo-stream/07"},
		{"indented own line", "  <!-- needs-decision: demo-stream/07 -->  " + opts, "demo-stream/07"},
		{"prose id", "<!-- needs-decision: see the parser thread -->" + opts, ""},
		{"oversized", "<!-- needs-decision: " + strings.Repeat("a", 60000) + "/07 -->" + opts, ""},
		{"inside a sentence", "As asked in <!-- needs-decision: demo-stream/07 --> here." + opts, ""},
		{"quoted", "> <!-- needs-decision: demo-stream/07 -->" + opts, ""},
		{"conflicting markers", "<!-- needs-decision: demo-stream/07 -->\n<!-- needs-decision: demo-stream/08 -->" + opts, ""},
		{"repeated marker", "<!-- needs-decision: demo-stream/07 -->\n<!-- needs-decision: demo-stream/07 -->" + opts, "demo-stream/07"},
		{"upper-case stream", "<!-- needs-decision: Demo/07 -->" + opts, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := decComment(90, fixtureBlessLogin, fixtureBlessID, "2026-10-02T06:00:30Z", "B")
			rec := ComposeDecisionRecord(DecisionInput{
				Repo: "example-org/tracker", Issue: 120, Tracker: "example-org/tracker#7",
				IssueBody: tc.body, IssueCreatedAt: "2026-10-01T06:00:00Z",
				Comments: []Comment{r}, Ruling: r, Now: time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC),
			})
			if rec.Brief != tc.want {
				t.Fatalf("brief %q, want %q", rec.Brief, tc.want)
			}
			if err := ValidateDecisionRecord(rec); err != nil {
				t.Fatalf("record refused: %v", err)
			}
		})
	}
}

// regression: medici-finance/assay#2426 (SEC-1/SEC-2: edits after the ruling) — the record
// says what stood when the driver ruled, or says it cannot tell; it never reports a pick or a
// recommendation read from text edited after the ruling.
func TestComposeDecisionEditState(t *testing.T) {
	const ruledAt = "2026-10-02T10:00:00Z"
	const before, after = "2026-10-01T09:00:00Z", "2026-10-02T11:00:00Z"
	edited := func(c Comment, at string) Comment { c.UpdatedAt = at; return c }
	relay := edited(decComment(50, decAppLogin, decAppID, "2026-10-01T09:00:00Z", decRelayOpts+"\nAnswer: A\n"), before)
	direct := edited(decComment(90, fixtureBlessLogin, fixtureBlessID, ruledAt, "B"), ruledAt)
	ratify := edited(decComment(90, fixtureBlessLogin, fixtureBlessID, ruledAt, "ratified"), ruledAt)
	// C-4: an anchor ask and a SEPARATE answer-only relay, so the relay's own edit time is
	// the only one that can mark the record edited. C is C under both letterings, so the
	// unedited control resolves to a definite pick.
	ask := edited(decComment(49, decAppLogin, decAppID, "2026-10-01T08:00:00Z", decRelayOpts), before)
	answer := edited(decComment(51, decAppLogin, decAppID, "2026-10-01T09:30:00Z", "Ruling relayed.\nAnswer: C\n"), before)
	cases := []struct {
		name          string
		thread        []Comment
		bodyEditedAt  string
		bodyEditKnown bool
		wantState     string
		wantPicked    string
	}{
		{"nothing edited", []Comment{direct}, "", true, DecisionEditUnedited, "B"},
		{"body edited before", []Comment{direct}, before, true, DecisionEditUnedited, "B"},
		{"body edited after", []Comment{direct}, after, true, DecisionEditEdited, DecisionPickUnparsed},
		{"body edit unknown", []Comment{direct}, "", false, DecisionEditUnverifiable, "B"},
		{"body edit stamp garbled", []Comment{direct}, "yesterday", true, DecisionEditUnverifiable, "B"},
		{"ruling edited", []Comment{edited(direct, after)}, "", true, DecisionEditEdited, DecisionPickUnparsed},
		{"ruling update time missing", []Comment{edited(direct, "")}, "", true, DecisionEditUnverifiable, "B"},
		{"relay unedited", []Comment{relay, ratify}, "", true, DecisionEditUnedited, "A"},
		{"relay edited after", []Comment{edited(relay, after), ratify}, "", true, DecisionEditEdited, DecisionPickUnparsed},
		{"relay update time missing", []Comment{edited(relay, ""), ratify}, "", true, DecisionEditUnverifiable, "A"},
		{"edit after beats unknown", []Comment{edited(relay, ""), ratify}, after, true, DecisionEditEdited, DecisionPickUnparsed},
		{"separate relay unedited", []Comment{ask, answer, ratify}, "", true, DecisionEditUnedited, "C"},
		{"separate relay edited after", []Comment{ask, edited(answer, after), ratify}, "", true, DecisionEditEdited, DecisionPickUnparsed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.thread[len(tc.thread)-1]
			rec := ComposeDecisionRecord(DecisionInput{
				Repo: "example-org/tracker", Issue: 120, Tracker: "example-org/tracker#7",
				IssueBody: decBody, IssueCreatedAt: "2026-10-01T06:00:00Z",
				Comments: tc.thread, Ruling: r, Now: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC),
				IssueBodyEditedAt: tc.bodyEditedAt, IssueBodyEditKnown: tc.bodyEditKnown,
			})
			if rec.EditState != tc.wantState || rec.Picked != tc.wantPicked {
				t.Fatalf("edit_state %q picked %q, want %q %q", rec.EditState, rec.Picked, tc.wantState, tc.wantPicked)
			}
			if rec.EditState == DecisionEditEdited && (rec.Recommended != DecisionRecommendedNone || rec.PickedIsRecommended) {
				t.Fatalf("an edited record kept a recommendation: %+v", rec)
			}
			if err := ValidateDecisionRecord(rec); err != nil {
				t.Fatalf("record refused: %v", err)
			}
		})
	}
}

// regression: medici-finance/assay#2426 (wire schema) — the documented field names are the
// encoded keys, in order; a rename is a schema change, never a refactor.
func TestDecisionRecordWireKeys(t *testing.T) {
	line, err := EncodeDecisionRecord(validDecisionRecord(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"schema", "repo", "issue", "tracker", "brief", "options", "options_source", "recommended",
		"picked", "picked_via", "picked_is_recommended", "edit_state", "ruler", "ruling_sha256", "opened_at",
		"asked_at", "ruled_at", "recorded_at", "opened_to_ruled_s", "asked_to_ruled_s", "tool_sha"}
	dec := json.NewDecoder(strings.NewReader(string(line)))
	var got []string
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, k.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wire keys:\n got %v\nwant %v", got, want)
	}
	b, _ := json.Marshal(DecisionOption{ID: "A", TextSHA256: "x"})
	if string(b) != `{"id":"A","text_sha256":"x"}` {
		t.Fatalf("option keys: %s", b)
	}
}

func TestValidateDecisionRecord(t *testing.T) {
	if err := ValidateDecisionRecord(validDecisionRecord(t)); err != nil {
		t.Fatalf("valid record refused: %v", err)
	}
	cases := []struct {
		name  string
		field string
		mut   func(r *HumanDecisionRecord)
	}{
		{"ruler is a login", "ruler", func(r *HumanDecisionRecord) { r.Ruler = fixtureBlessLogin }},
		{"picked outside set", "picked", func(r *HumanDecisionRecord) { r.Picked = "D"; r.PickedIsRecommended = false }},
		{"five options", "options", func(r *HumanDecisionRecord) {
			for _, id := range []string{"D", "1"} {
				r.Options = append(r.Options, DecisionOption{ID: id, TextSHA256: r.Options[0].TextSHA256})
			}
		}},
		{"unknown source", "options_source", func(r *HumanDecisionRecord) { r.OptionsSource = "walk" }},
		{"unknown via", "picked_via", func(r *HumanDecisionRecord) { r.PickedVia = "inferred" }},
		{"latency mismatch", "opened_to_ruled_s", func(r *HumanDecisionRecord) { r.OpenedToRuledS++ }},
		{"asked latency off", "asked_to_ruled_s", func(r *HumanDecisionRecord) { r.AskedToRuledS = 0 }},
		{"negative latency", "opened_to_ruled_s", func(r *HumanDecisionRecord) {
			r.OpenedAt, r.AskedAt = "2026-10-03T06:00:30Z", "2026-10-03T06:00:30Z"
			r.OpenedToRuledS, r.AskedToRuledS = -86400, -86400
		}},
		{"newline in field", "tracker", func(r *HumanDecisionRecord) { r.Tracker = "example-org/tracker#7\nx" }},
		{"newline in brief", "brief", func(r *HumanDecisionRecord) { r.Brief = "a\rb" }},
		{"rec flag lies", "picked_is_recommended", func(r *HumanDecisionRecord) { r.PickedIsRecommended = false }},
		{"not UTC", "ruled_at", func(r *HumanDecisionRecord) { r.RuledAt = "2026-10-02T08:00:30+02:00" }},
		{"bad recommended", "recommended", func(r *HumanDecisionRecord) { r.Recommended = "Z" }},
		{"bad schema", "schema", func(r *HumanDecisionRecord) { r.Schema = "human-decision-v2" }},
		// regression: medici-finance/assay#2426 (validator gaps)
		{"option digest shape", "options", func(r *HumanDecisionRecord) { r.Options[0].TextSHA256 = "not-a-digest" }},
		{"option digest upper", "options", func(r *HumanDecisionRecord) {
			r.Options[0].TextSHA256 = strings.ToUpper(r.Options[0].TextSHA256)
		}},
		{"ruling digest shape", "ruling_sha256", func(r *HumanDecisionRecord) { r.RulingSHA256 = "abc" }},
		{"duplicate id", "options", func(r *HumanDecisionRecord) { r.Options[2].ID = "A" }},
		{"id shape", "options", func(r *HumanDecisionRecord) { r.Options[2].ID = "E" }},
		{"brief prose", "brief", func(r *HumanDecisionRecord) { r.Brief = "see the parser thread" }},
		{"brief oversized", "brief", func(r *HumanDecisionRecord) { r.Brief = strings.Repeat("a", 60000) + "/07" }},
		{"edit state unknown", "edit_state", func(r *HumanDecisionRecord) { r.EditState = "maybe" }},
		{"edited with a pick", "edit_state", func(r *HumanDecisionRecord) { r.EditState = DecisionEditEdited }},
		{"edited with a recommendation", "edit_state", func(r *HumanDecisionRecord) {
			r.EditState, r.Picked, r.PickedIsRecommended = DecisionEditEdited, DecisionPickUnparsed, false
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := validDecisionRecord(t)
			tc.mut(&r)
			err := ValidateDecisionRecord(r)
			var de *DecisionRecordError
			if !errors.As(err, &de) || de.Field != tc.field {
				t.Fatalf("got %v, want a refusal naming %q", err, tc.field)
			}
			if _, encErr := EncodeDecisionRecord(r); encErr == nil {
				t.Fatal("EncodeDecisionRecord encoded a record the validator refuses")
			}
		})
	}
}

func TestAppendDecisionRecord(t *testing.T) {
	dir := setup(t)
	line, err := EncodeDecisionRecord(validDecisionRecord(t))
	if err != nil {
		t.Fatal(err)
	}
	if b := DecisionRecordBlock(line); strings.Count(b, "-->") != 1 || !strings.HasSuffix(b, "} -->") ||
		!strings.HasPrefix(b, "<!-- human-decision-v1 {") {
		t.Fatalf("block shape: %s", DecisionRecordBlock(line))
	}
	for i := 0; i < 2; i++ {
		if err := AppendDecisionRecord(line); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(dir, "decision-records.jsonl")
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(line)+"\n"+string(line)+"\n" {
		t.Fatalf("file:\n%s", got)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, want 0600", fi.Mode().Perm())
	}
	if err := AppendDecisionRecord([]byte("{\"a\":1}\n{\"b\":2}")); err == nil {
		t.Fatal("a two-line payload was appended")
	}
}

// regression: medici-finance/assay#2426 (SEC-8) — the local file is never written through a
// link, and a file left wider than 0600 is brought back to 0600 on the next append.
func TestAppendDecisionRecordFileGuards(t *testing.T) {
	line, err := EncodeDecisionRecord(validDecisionRecord(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("symlink refused", func(t *testing.T) {
		dir := setup(t)
		target := filepath.Join(t.TempDir(), "elsewhere")
		if err := os.WriteFile(target, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, "decision-records.jsonl")); err != nil {
			t.Skipf("no symlinks here: %v", err)
		}
		if err := AppendDecisionRecord(line); err == nil {
			t.Fatal("appended through a symlink")
		}
		if b, _ := os.ReadFile(target); len(b) != 0 {
			t.Fatalf("the link target was written: %q", b)
		}
	})
	t.Run("wide mode repaired", func(t *testing.T) {
		dir := setup(t)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, "decision-records.jsonl")
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendDecisionRecord(line); err != nil {
			t.Fatal(err)
		}
		if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v, want 0600", fi.Mode().Perm())
		}
	})
}

// TestDecisionBlockOutbound — the forge copy rides a close comment through the outbound
// check (OutboundChecked wraps every production Forge): the block's digests and closed-set
// values must pass it on a public and a private target alike, or the lane could never post.
func TestDecisionBlockOutbound(t *testing.T) {
	setup(t)
	for _, repo := range []string{obPublic, obPrivate} {
		// The block names the repo it is posted on (and, here, a tracker in that repo) — the
		// close comment's own prose already names --tracker, so the block adds no new reference.
		rec := validDecisionRecord(t)
		rec.Repo, rec.Tracker = repo, repo+"#7"
		line, err := EncodeDecisionRecord(rec)
		if err != nil {
			t.Fatal(err)
		}
		body := "Closing as not planned: recorded human decision.\n\n" + DecisionRecordBlock(line) + "\n"
		if err := OutboundCheck(OutboundWrite{Tool: "deskclose", Verb: "triage", Role: "desk", Repo: repo,
			Kind: OutboundKindComment, NumberHint: 120, Fields: []OutboundField{{Name: "body", Text: body}}}); err != nil {
			t.Fatalf("%s: the record block is refused outbound: %v", repo, err)
		}
	}
}
