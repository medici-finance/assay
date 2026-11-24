package deskkit

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/workref"
)

const (
	rsgRepo    = "example-org/example-repo"
	rsgIssue   = 41
	rsgComment = 9001
	rsgHuman   = "example-human"
	rsgKey     = "00000000-0000-4000-8000-000000000018"
	rsgBrief   = "example:ex:sdlc:18"
	rsgPre     = "2026-09-01T10:00:00Z"
	rsgPost    = "2026-12-02T10:00:00Z"
	rsgMarker  = "<!-- decision-gate: " + rsgBrief + " -->"
)

func rsgCard(version, extra string) string {
	return `<!-- assay-card: {"schema":"gate-card/v1","purpose":"decision","brief":"` + rsgBrief +
		`","local":"sdlc/18","repository":"` + rsgRepo + `"` + extra + `,"version":` + version + `} -->`
}

func rsgRegistry(t *testing.T) (*workref.Context, workref.Registry) {
	t.Helper()
	brief := workref.Identity{Kind: workref.Brief, Cell: "example", Repo: "ex", Stream: "sdlc", Number: "18"}
	renumbered := workref.Identity{Kind: workref.Brief, Cell: "example", Repo: "ex", Stream: "sdlc", Number: "07"}
	reg, err := workref.NewRegistry(
		[]workref.Root{{Cell: "example", Repo: "ex", Aliases: []string{"e"}}, {Cell: "example", Repo: "elsewhere"}},
		[]workref.Binding{{Identity: brief, Key: rsgKey, Aliases: []workref.Identity{renumbered}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return &workref.Context{Cell: "example", Repo: "ex"}, reg
}

// milestone-support/09: only the same human, issue and exact work
// subject carries a ruling across aliases; a copied ruling, another revision or
// root, a bot actor or a changed card never does.
// regression: #7088
func TestRulingAliasAuthority(t *testing.T) {
	declaring, registry := rsgRegistry(t)
	issueBody := "A human decision is needed.\n\n" + rsgMarker + "\n"
	card1 := rsgCard("1", "")
	withCard := issueBody + "\n" + card1 + "\n"
	line := func(ref, card string) string {
		return "Option 1.\n\nRuling-Subject: " + ref + " card=" + card + "\n"
	}
	base := func() RulingSubjectInput {
		return RulingSubjectInput{
			Comment: &RulingCommentObservation{Repo: rsgRepo, Issue: rsgIssue, ID: rsgComment,
				Author: Account{Login: rsgHuman, Type: "User"}, Body: line("example:e:sdlc:b-18@1", "none"),
				CreatedAt: rsgPre, UpdatedAt: rsgPre},
			Issue:         &RulingIssueObservation{Repo: rsgRepo, Number: rsgIssue, Body: issueBody},
			ExpectedRepo:  rsgRepo,
			ExpectedIssue: rsgIssue,
			ExpectedID:    rsgComment,
			Humans:        map[string]string{"example": rsgHuman},
			Declared:      rsgBrief,
			Version:       1,
			Declaring:     declaring,
			Registry:      registry,
		}
	}
	type want struct {
		state  workref.BindingState
		reason string
		legacy bool
	}
	bound := want{state: workref.Bound}
	cases := []struct {
		name string
		edit func(*RulingSubjectInput)
		want want
	}{
		// Passes: an alias of the same key and explicit revision.
		{"alias root, card none", func(*RulingSubjectInput) {}, bound},
		{"renumbered alias, matching card", func(in *RulingSubjectInput) {
			in.Comment.Body = line("sdlc:b-07@1", workref.CardDigest(card1))
			in.Issue.Body = withCard
		}, bound},
		{"after cutover with a subject line", func(in *RulingSubjectInput) {
			in.Comment.CreatedAt, in.Comment.UpdatedAt = rsgPost, rsgPost
		}, bound},

		// Refusals.
		{"copied ruling on another issue", func(in *RulingSubjectInput) { in.Comment.Issue = 99 }, want{workref.Refused, RulingSubjectIssue, false}},
		{"copied ruling, another comment", func(in *RulingSubjectInput) { in.Comment.ID = 9002 }, want{workref.Refused, RulingSubjectIssue, false}},
		{"issue in another repository", func(in *RulingSubjectInput) { in.Issue.Repo = "example-org/other-repo" }, want{workref.Refused, RulingSubjectIssue, false}},
		{"other root, same stream and number", func(in *RulingSubjectInput) {
			in.Comment.Body = line("example:elsewhere:sdlc:b-18@1", "none")
		}, want{workref.Refused, RulingSubjectSubject, false}},
		{"marker names another root", func(in *RulingSubjectInput) {
			in.Issue.Body = "A human decision is needed.\n\n<!-- decision-gate: example:elsewhere:sdlc:18 -->\n"
		}, want{workref.Refused, RulingSubjectIssue, false}},
		{"wrong revision", func(in *RulingSubjectInput) { in.Version = 2 }, want{workref.Refused, RulingSubjectSubject, false}},
		{"absent explicit version, card shows 1", func(in *RulingSubjectInput) {
			in.Version = 0
			in.Comment.Body = line("example:e:sdlc:b-18@1", workref.CardDigest(card1))
			in.Issue.Body = withCard
		}, want{workref.Refused, RulingSubjectSubject, false}},
		{"duplicate subject lines", func(in *RulingSubjectInput) {
			in.Comment.Body += "Ruling-Subject: example:e:sdlc:b-18@1 card=none\n"
		}, want{workref.Refused, RulingSubjectSubject, false}},
		{"edited comment", func(in *RulingSubjectInput) { in.Comment.UpdatedAt = "2026-09-02T10:00:00Z" }, want{workref.Refused, RulingSubjectEdited, false}},
		{"edit state unknown", func(in *RulingSubjectInput) { in.Comment.UpdatedAt = "" }, want{workref.CouldNotCheck, RulingSubjectEdited, false}},
		{"bot actor", func(in *RulingSubjectInput) { in.Comment.Author = Account{Login: "example-app", Type: "Bot"} }, want{workref.Refused, RulingSubjectActor, false}},
		{"App login", func(in *RulingSubjectInput) {
			in.Comment.Author = Account{Login: "example-app[bot]", Type: "User"}
			in.Humans["app"] = "example-app[bot]"
		}, want{workref.Refused, RulingSubjectActor, false}},
		{"unmapped person", func(in *RulingSubjectInput) { in.Comment.Author = Account{Login: "someone-else", Type: "User"} }, want{workref.Refused, RulingSubjectActor, false}},
		{"actor type unknown", func(in *RulingSubjectInput) { in.Comment.Author.Type = "" }, want{workref.CouldNotCheck, RulingSubjectActor, false}},
		{"comment unread", func(in *RulingSubjectInput) { in.Comment = nil }, want{workref.CouldNotCheck, RulingSubjectIssue, false}},

		// Holds: the card the human saw is no longer the card.
		{"card edited to v2, brief moved to v2", func(in *RulingSubjectInput) {
			in.Version = 2
			in.Comment.Body = line("example:e:sdlc:b-18@1", workref.CardDigest(card1))
			in.Issue.Body = issueBody + "\n" + rsgCard("2", "") + "\n"
		}, want{workref.Refused, RulingSubjectSubject, false}},
		{"card changed, brief stays v1", func(in *RulingSubjectInput) {
			in.Comment.Body = line("example:e:sdlc:b-18@1", workref.CardDigest(card1))
			in.Issue.Body = issueBody + "\n" + rsgCard("1", `,"source_url":"https://example.invalid/x"`) + "\n"
		}, want{workref.CouldNotCheck, RulingSubjectCard, false}},
		{"card removed, marker remains", func(in *RulingSubjectInput) {
			in.Comment.Body = line("example:e:sdlc:b-18@1", workref.CardDigest(card1))
		}, want{workref.CouldNotCheck, RulingSubjectCard, false}},
		{"card added after card=none", func(in *RulingSubjectInput) { in.Issue.Body = withCard }, want{workref.CouldNotCheck, RulingSubjectCard, false}},
		{"two cards", func(in *RulingSubjectInput) {
			in.Comment.Body = line("example:e:sdlc:b-18@1", workref.CardDigest(card1))
			in.Issue.Body = withCard + card1 + "\n"
		}, want{workref.CouldNotCheck, RulingSubjectCard, false}},

		// Historical rulings.
		{"no line, before cutover", func(in *RulingSubjectInput) { in.Comment.Body = "Option 1." }, want{workref.Bound, "", true}},
		{"no line, after cutover", func(in *RulingSubjectInput) {
			in.Comment.Body = "Option 1."
			in.Comment.CreatedAt, in.Comment.UpdatedAt = rsgPost, rsgPost
		}, want{workref.Refused, RulingSubjectSubject, false}},
		{"no line, unknown creation time", func(in *RulingSubjectInput) {
			in.Comment.Body = "Option 1."
			in.Comment.CreatedAt, in.Comment.UpdatedAt = "unknown", "unknown"
		}, want{workref.CouldNotCheck, RulingSubjectSubject, false}},
		{"typed line before cutover, card missing", func(in *RulingSubjectInput) {
			in.Comment.Body = line("example:e:sdlc:b-18@1", workref.CardDigest(card1))
		}, want{workref.CouldNotCheck, RulingSubjectCard, false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base()
			tc.edit(&in)
			got := RulingSubjectGate(in)
			if got.State != tc.want.state || got.Reason != tc.want.reason || got.Legacy != tc.want.legacy {
				t.Fatalf("got %+v, want state %s reason %q legacy %v", got, tc.want.state, tc.want.reason, tc.want.legacy)
			}
		})
	}

	// Capability stays fail-closed: no reader capability advertises prefixed
	// references, so a record that requires them is never judged supported.
	for _, op := range ReaderCapabilities("dev", "deskevidence").Operations {
		for _, s := range op.Semantics {
			if strings.Contains(s, "prefixed") {
				t.Errorf("reader capability advertises %q", s)
			}
		}
	}
	need := readercaps.Requirements{Schema: readercaps.RequirementsSchema, Required: []readercaps.Operation{
		{Operation: "syntax", Schemas: []string{"brief-v2"}, Semantics: []string{"prefixed-references"}}}}
	if got := readercaps.Assess(need, readercaps.Observation{State: readercaps.Current, Advertisement: ReaderCapabilities("dev", "deskevidence")}); got.State == readercaps.Supported {
		t.Errorf("a prefixed-reference requirement was judged supported: %+v", got)
	}
}
