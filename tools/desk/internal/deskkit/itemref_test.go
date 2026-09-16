package deskkit

// itemref_test.go — the typed item-reference grammar.
//
// The properties under test are the two that make the grammar safe to adopt across the desk
// verbs at once: `!N` STATES a change, and `#N` states NOTHING. The second is the one that is
// easy to get wrong — reading `#` as "issue" looks tidier and would refuse the pull-request
// references every caller on a single-sequence forge already writes.

import (
	"strings"
	"testing"
)

func TestParseItemRefGrammar(t *testing.T) {
	cases := []struct {
		in       string
		wantRepo string
		wantNum  int
		wantKind TargetKind
	}{
		// Bare and `#` forms state NO kind. This is the regression that matters: every
		// existing caller writes one of these, and the forge resolves them exactly as before.
		{"12", "", 12, ""},
		{"#12", "", 12, ""},
		{"owner/repo#12", "owner/repo", 12, ""},
		// `!` states a CHANGE — the sigil GitLab writes a merge request with.
		{"!12", "", 12, TargetChange},
		{"owner/repo!12", "owner/repo", 12, TargetChange},
		// A web URL states the kind in its own path, so no flag is needed with one.
		{"https://github.com/owner/repo/issues/12", "owner/repo", 12, TargetIssue},
		{"https://github.com/owner/repo/pull/12", "owner/repo", 12, TargetChange},
		{"https://gitlab.example/owner/repo/-/merge_requests/12", "owner/repo", 12, TargetChange},
		{"https://gitlab.example/owner/repo/-/issues/12", "owner/repo", 12, TargetIssue},
		// Surrounding whitespace is not a different reference.
		{"  !12  ", "", 12, TargetChange},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseItemRef(c.in)
			if err != nil {
				t.Fatalf("ParseItemRef(%q): %v", c.in, err)
			}
			if got.Repo != c.wantRepo || got.Number != c.wantNum || got.Kind != c.wantKind {
				t.Fatalf("ParseItemRef(%q) = %+v, want repo=%q number=%d kind=%q",
					c.in, got, c.wantRepo, c.wantNum, c.wantKind)
			}
			if got.KindStated() != (c.wantKind != "") {
				t.Fatalf("ParseItemRef(%q).KindStated() = %v, want %v", c.in, got.KindStated(), c.wantKind != "")
			}
		})
	}
}

// TestParseItemRefHashIsNeutral is the regression that keeps the grammar adoptable: `#N` must
// NOT resolve to TargetIssue. On a forge with one number sequence `#7` is the ordinary way to
// write a pull request, and a typed read is an ASSERTION — so reading the sigil as "issue"
// would turn every existing `--by #N` naming a PR into a could-not-check refusal.
func TestParseItemRefHashIsNeutral(t *testing.T) {
	for _, in := range []string{"7", "#7", "owner/repo#7"} {
		got, err := ParseItemRef(in)
		if err != nil {
			t.Fatalf("ParseItemRef(%q): %v", in, err)
		}
		if got.Kind != "" {
			t.Fatalf("ParseItemRef(%q).Kind = %q — the neutral forms must state NO kind, or a "+
				"pull-request reference written as #N stops resolving", in, got.Kind)
		}
	}
}

func TestParseItemRefRefusals(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"empty", "  ", "names nothing"},
		{"not a number", "banana", "cannot read"},
		{"zero", "#0", "not a positive number"},
		{"negative", "-3", "cannot read"},
		{"trailing text", "#12x", "cannot read"},
		{"double sigil", "#!12", "cannot read"},
		// A nested-group project path cannot be reduced to the two-segment coordinate a repo
		// is addressed by, and picking two of its segments would name a DIFFERENT project that
		// may well exist — so it is refused, naming the shortfall.
		{"nested group url", "https://gitlab.example/group/sub/proj/-/issues/12", "not the two segments"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseItemRef(c.in)
			if err == nil {
				t.Fatalf("ParseItemRef(%q) returned no error — an unparseable reference must be refused, "+
					"never resolved to a guess", c.in)
			}
			if !IsRefused(err) {
				t.Fatalf("ParseItemRef(%q) error is not a refusal: %v", c.in, err)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("ParseItemRef(%q) = %q, want it to mention %q", c.in, err.Error(), c.want)
			}
		})
	}
}

func TestItemRefString(t *testing.T) {
	cases := []struct {
		ref  ItemRef
		want string
	}{
		{ItemRef{Number: 12}, "#12"},
		{ItemRef{Number: 12, Kind: TargetChange}, "!12"},
		{ItemRef{Repo: "o/r", Number: 12, Kind: TargetIssue}, "o/r#12"},
		{ItemRef{Repo: "o/r", Number: 12, Kind: TargetChange}, "o/r!12"},
	}
	for _, c := range cases {
		if got := c.ref.String(); got != c.want {
			t.Fatalf("ItemRef%+v.String() = %q, want %q", c.ref, got, c.want)
		}
	}
}

func TestResolveStatedKind(t *testing.T) {
	change := ItemRef{Number: 4, Kind: TargetChange}
	neutral := ItemRef{Number: 4}

	if got, err := ResolveStatedKind("--by", neutral, ""); err != nil || got != "" {
		t.Fatalf("neither side states a kind: got %q, %v — want unstated", got, err)
	}
	if got, err := ResolveStatedKind("--by", neutral, TargetIssue); err != nil || got != TargetIssue {
		t.Fatalf("flag alone: got %q, %v — want issue", got, err)
	}
	if got, err := ResolveStatedKind("--by", change, ""); err != nil || got != TargetChange {
		t.Fatalf("sigil alone: got %q, %v — want change", got, err)
	}
	if got, err := ResolveStatedKind("--by", change, TargetChange); err != nil || got != TargetChange {
		t.Fatalf("both agree: got %q, %v — want change", got, err)
	}

	// A sigil and a flag that disagree are two different statements about one object.
	// Resolving in favour of either would ignore something the operator actually typed.
	_, err := ResolveStatedKind("--by", change, TargetIssue)
	if err == nil || !IsRefused(err) {
		t.Fatalf("a disagreeing sigil and flag must be refused, got %v", err)
	}
	if !strings.Contains(err.Error(), "disagree") {
		t.Fatalf("refusal %q should say the two disagree", err.Error())
	}
}

// TestCloseIssueTypedRefusesReasonOnAChange pins the shared precondition both backends apply.
// Neither forge records a state reason on a change, so accepting one would report a success
// for a distinction that went nowhere.
func TestCloseIssueTypedRefusesReasonOnAChange(t *testing.T) {
	repo := ForgeRepo{Owner: "o", Name: "r"}
	if err := requireNoReasonOnChange(repo, 4, TargetChange, "not_planned"); err == nil || !IsRefused(err) {
		t.Fatalf("a state reason on a change must be refused, got %v", err)
	}
	if err := requireNoReasonOnChange(repo, 4, TargetChange, ""); err != nil {
		t.Fatalf("an empty reason on a change is the ordinary case: %v", err)
	}
	if err := requireNoReasonOnChange(repo, 4, TargetIssue, "not_planned"); err != nil {
		t.Fatalf("an issue carries a state reason: %v", err)
	}
	if err := requireNoReasonOnChange(repo, 4, TargetKind("banana"), ""); err == nil || !IsRefused(err) {
		t.Fatalf("an unknown kind must be refused rather than defaulted, got %v", err)
	}
}
