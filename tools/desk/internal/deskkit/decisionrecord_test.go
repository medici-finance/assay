package deskkit

import (
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

func TestParseRulingPick(t *testing.T) {
	relay := func(login string, uid int64, extra string) Comment {
		return decComment(50, login, uid, "2026-10-01T10:00:00Z", decRelayOpts+extra)
	}
	ruling := func(body string) Comment {
		return decComment(90, fixtureBlessLogin, fixtureBlessID, "2026-10-02T10:00:00Z", body)
	}
	cases := []struct {
		name       string
		thread     []Comment // before the ruling
		ruling     string
		wantPicked string
		wantVia    string
	}{
		{"direct letter", nil, "B", "B", DecisionViaDirect},
		{"option word", nil, "Option C — leave it as it stands", "C", DecisionViaDirect},
		{"ratified via relay",
			[]Comment{relay(decAppLogin, decAppID, "\nAnswer: A\n")}, "ratified", "A", DecisionViaRatified},
		{"untrusted relay",
			[]Comment{relay("mallory", 4242, "\nAnswer: A\n")}, "ratified", DecisionPickUnparsed, DecisionViaRatified},
		{"wrong-id relay",
			[]Comment{relay(decAppLogin, decAppID+1, "\nAnswer: A\n")}, "I ratify", DecisionPickUnparsed, DecisionViaRatified},
		{"source vs walk",
			[]Comment{relay(decAppLogin, decAppID, "")}, "B", DecisionPickAmbiguous, DecisionViaDirect},
		{"same under both", []Comment{relay(decAppLogin, decAppID, "")}, "C", "C", DecisionViaDirect},
		{"answer-only relay walk",
			[]Comment{decComment(51, decAppLogin, decAppID, "2026-10-01T11:00:00Z", "Ruling relayed.\nAnswer: A\n")},
			"approved", DecisionPickAmbiguous, DecisionViaRatified},
		{"question", nil, "B?", DecisionPickUnparsed, DecisionViaDirect},
		{"hold", nil, "Hold for now", DecisionPickUnparsed, DecisionViaDirect},
		{"not offered", nil, "D", DecisionPickUnparsed, DecisionViaDirect},
		{"no grammar", nil, "Looks fine to me", DecisionPickUnparsed, DecisionViaDirect},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			thread := append(append([]Comment(nil), tc.thread...), ruling(tc.ruling))
			idx := len(thread) - 1
			anchor := DecisionAnchor(thread[:idx])
			picked, via := ParseRulingPick(decBody, thread, anchor, idx, thread[idx])
			if picked != tc.wantPicked || via != tc.wantVia {
				t.Fatalf("ruling %q: picked %q via %q, want %q via %q", tc.ruling, picked, via, tc.wantPicked, tc.wantVia)
			}
		})
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
