package draft

import (
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// fakeClaim stands in for the read-only answer layer's rendered answer. It
// implements [Claim] with the same method set, which is the point: the
// production type satisfies this interface implicitly, with no adapter.
type fakeClaim struct {
	id      string
	render  string
	caveats []string
	value   int64
	ok      bool
}

func (c fakeClaim) Question() string     { return c.id }
func (c fakeClaim) Render() string       { return c.render }
func (c fakeClaim) Caveats() []string    { return append([]string(nil), c.caveats...) }
func (c fakeClaim) Value() (int64, bool) { return c.value, c.ok }

func computed(id string, v int64) fakeClaim {
	return fakeClaim{
		id:      id,
		render:  id + ": figure=" + itoa(v) + " · state=checked · source=statusgen --root <root> --json · probe=count of rows · window=the tree at <sha> · limit=none on rows · as-of=2026-08-13T00:00:00Z@deadbeef",
		caveats: []string{"board status is not work state", "counts move while you read them"},
		value:   v,
		ok:      true,
	}
}

func unavailable(id string) fakeClaim {
	return fakeClaim{
		id:      id,
		render:  id + ": figure=could-not-check · state=could-not-check · source=deskboard actions --json · probe=count of action rows · window=instantaneous · limit=the watched repo set · as-of=2026-08-13T00:00:00Z@clock-only: live count",
		caveats: []string{"an empty result from this probe is BLIND, not idle"},
		ok:      false,
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

var testTarget = Target{Repo: "example-org/example-repo", Kind: "issue", Ref: "escalation-alpha"}

// mustCompose builds a valid draft for the tests that need one. It is in this
// file rather than the authority suite so the authority tests exercise the
// same composition path everything else does.
func mustCompose(t *testing.T) Draft {
	t.Helper()
	d, err := Compose(KindAnswer, testTarget,
		"proposed answer: where the constraint is",
		"The board holds {{figure:brief-status-count}} rows at todo, and the action queue "+
			"reports {{figure:pr-action-count}} items waiting. Recommend draining the queue first.",
		computed("brief-status-count", 47), unavailable("pr-action-count"))
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	return d
}

// TestTheDraftBannerSaysWhatItIs pins the banner's CONTENT, not just its
// presence. Without this, emptying the constant would satisfy every
// HasPrefix(out, DraftBanner) assertion in the suite — an empty prefix matches
// everything, so the banner could silently stop appearing while the tests
// stayed green. This is the check on the check.
func TestTheDraftBannerSaysWhatItIs(t *testing.T) {
	for _, want := range []string{"DRAFT", "NOT AN ACTION", "cannot post"} {
		if !strings.Contains(DraftBanner, want) {
			t.Errorf("the draft banner %q does not say %q", DraftBanner, want)
		}
	}
	if len(DraftBanner) < 40 {
		t.Fatalf("the draft banner is %d characters — an empty or near-empty banner is a prefix that matches everything, which silently disarms every render assertion in this suite", len(DraftBanner))
	}
	if strings.TrimSpace(ExitStatement) == "" || !strings.Contains(ExitStatement, "only the human commits") {
		t.Errorf("the exit statement does not carry the §6.2 rule: %q", ExitStatement)
	}
}

func TestComposeProducesADraftThatSaysItIsADraft(t *testing.T) {
	d := mustCompose(t)
	out := d.Render()
	if !strings.HasPrefix(out, DraftBanner) {
		t.Fatalf("a rendered draft does not lead with the draft banner:\n%s", out)
	}
	for _, want := range []string{"proposes: " + string(KindAnswer), testTarget.String(), "provenance:", "caveats carried by the claims above:", ExitStatement} {
		if !strings.Contains(out, want) {
			t.Errorf("the rendered draft omits %q", want)
		}
	}
	if len(d.Cited()) != 2 {
		t.Errorf("draft cites %d claims, want 2", len(d.Cited()))
	}
}

// TestAnUncomputableFigureRendersCouldNotCheckNeverZero is the numbers rule
// crossing into drafting. This is the single most important behaviour here:
// a draft that says "0 items are waiting" for a probe that came back blind is
// a falsehood about to acquire a human's authority.
func TestAnUncomputableFigureRendersCouldNotCheckNeverZero(t *testing.T) {
	d, err := Compose(KindAnswer, testTarget, "t",
		"The queue holds {{figure:pr-action-count}} items.", unavailable("pr-action-count"))
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	out := d.Render()
	if !strings.Contains(out, "The queue holds "+FigureUnavailable+" items.") {
		t.Fatalf("the unavailable figure did not render as %q:\n%s", FigureUnavailable, out)
	}
	if regexp.MustCompile(`The queue holds [0-9]`).MatchString(out) {
		t.Fatal("an unavailable figure rendered as a digit — the exact falsehood this layer exists to prevent")
	}
}

// TestAMeasuredZeroStillRenders is the control in the other direction. Without
// it, the rule above would be satisfiable by refusing every number.
func TestAMeasuredZeroStillRenders(t *testing.T) {
	d, err := Compose(KindAnswer, testTarget, "t",
		"The queue holds {{figure:pr-action-count}} items.", computed("pr-action-count", 0))
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if !strings.Contains(d.Render(), "The queue holds 0 items.") {
		t.Fatalf("a genuinely measured zero did not render:\n%s", d.Render())
	}
}

func TestFigureTokenIsNotADigit(t *testing.T) {
	if regexp.MustCompile(`[0-9]`).MatchString(FigureUnavailable) {
		t.Fatalf("the unavailable-figure token %q contains a digit", FigureUnavailable)
	}
	if FigureUnavailable != "could-not-check" {
		t.Fatalf("the token is %q — it must equal the answer layer's, or a draft and the pane it quotes disagree about how 'no number' looks", FigureUnavailable)
	}
}

// TestComposeRefusesABareNumeral — a number in a body with nothing behind it
// is the model narrating a figure, which is what the numbers rule forbids.
func TestComposeRefusesABareNumeral(t *testing.T) {
	cases := []string{
		"There are 47 rows at todo.",
		"Roughly 3x the usual load.",
		"See the report from 2026-08-13.",
		"Item #7 is the blocker.",
	}
	for _, body := range cases {
		if _, err := Compose(KindAnswer, testTarget, "t", body); !errors.Is(err, ErrUncitedFigure) {
			t.Errorf("Compose(%q) = %v, want ErrUncitedFigure", body, err)
		}
	}
	// The control in the other direction: prose with no numeral composes.
	if _, err := Compose(KindAnswer, testTarget, "t", "The queue is the constraint."); err != nil {
		t.Fatalf("numeral-free prose was refused: %v", err)
	}
}

// TestTheLiteralEscapeCostsAReason — an uncitable literal is allowed exactly
// once it is reviewable. A bare one is not.
func TestTheLiteralEscapeCostsAReason(t *testing.T) {
	ok, err := Compose(KindAnswer, testTarget, "t",
		"Cadence is {{lit:14 days|the cadence is a policy constant from the stream README, not a measurement}}.")
	if err != nil {
		t.Fatalf("a reasoned literal was refused: %v", err)
	}
	if !strings.Contains(ok.Render(), "Cadence is 14 days.") {
		t.Fatalf("the literal did not render:\n%s", ok.Render())
	}
	for _, bad := range []string{
		"Cadence is {{lit:14 days}}.",
		"Cadence is {{lit:14 days|}}.",
		"Cadence is {{lit:14 days|   }}.",
	} {
		if _, err := Compose(KindAnswer, testTarget, "t", bad); !errors.Is(err, ErrUncitedFigure) {
			t.Errorf("Compose(%q) = %v, want ErrUncitedFigure — a literal with no reason is a bare numeral wearing a placeholder", bad, err)
		}
	}
}

// TestCaveatsTravelWithTheClaimByClass — the caveat is not typed by the
// drafter. It is lifted off the claim, so a draft cannot quote a figure and
// leave its qualification behind.
func TestCaveatsTravelWithTheClaimByClass(t *testing.T) {
	d := mustCompose(t)
	got := d.Caveats()
	want := []string{
		"board status is not work state",
		"counts move while you read them",
		"an empty result from this probe is BLIND, not idle",
	}
	if len(got) != len(want) {
		t.Fatalf("draft carries %d caveats, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("caveat %d = %q, want %q", i, got[i], want[i])
		}
	}
	out := d.Render()
	for _, c := range want {
		if !strings.Contains(out, c) {
			t.Errorf("the rendered draft drops the caveat %q", c)
		}
	}
}

// TestComposeRefusesAnUncaveatedClaim — a claim carrying no caveat has either
// escaped its class or been hand-assembled. Both are reasons not to quote it.
func TestComposeRefusesAnUncaveatedClaim(t *testing.T) {
	naked := fakeClaim{id: "bare-count", render: "bare-count: figure=9 · source=somewhere", value: 9, ok: true}
	_, err := Compose(KindAnswer, testTarget, "t", "It is {{figure:bare-count}}.", naked)
	if !errors.Is(err, ErrUncaveatedClaim) {
		t.Fatalf("Compose with an uncaveated claim = %v, want ErrUncaveatedClaim", err)
	}
}

func TestComposeRefusesAnUnquotedCitation(t *testing.T) {
	_, err := Compose(KindAnswer, testTarget, "t",
		"The queue is the constraint.", computed("brief-status-count", 47))
	if !errors.Is(err, ErrUnquotedCitation) {
		t.Fatalf("Compose with an unquoted citation = %v, want ErrUnquotedCitation", err)
	}
}

func TestComposeRefusesAQuotationWithNoCitation(t *testing.T) {
	_, err := Compose(KindAnswer, testTarget, "t", "The board holds {{figure:brief-status-count}} rows.")
	if !errors.Is(err, ErrUncitedFigure) {
		t.Fatalf("Compose quoting an uncited claim = %v, want ErrUncitedFigure", err)
	}
}

func TestComposeRefusesMalformedInput(t *testing.T) {
	cases := []struct {
		name string
		run  func() (Draft, error)
	}{
		{"undeclared kind", func() (Draft, error) {
			return Compose(Kind("proposed-merge"), testTarget, "t", "prose")
		}},
		{"no repo", func() (Draft, error) {
			return Compose(KindAnswer, Target{Kind: "issue", Ref: "a"}, "t", "prose")
		}},
		{"undeclared target kind", func() (Draft, error) {
			return Compose(KindAnswer, Target{Repo: "o/r", Kind: "release", Ref: "a"}, "t", "prose")
		}},
		{"no ref", func() (Draft, error) {
			return Compose(KindAnswer, Target{Repo: "o/r", Kind: "issue"}, "t", "prose")
		}},
		{"no title", func() (Draft, error) { return Compose(KindAnswer, testTarget, "  ", "prose") }},
		{"no body", func() (Draft, error) { return Compose(KindAnswer, testTarget, "t", "  ") }},
		{"nil citation", func() (Draft, error) {
			return Compose(KindAnswer, testTarget, "t", "prose", nil)
		}},
		{"citation with no ID", func() (Draft, error) {
			return Compose(KindAnswer, testTarget, "t", "prose", fakeClaim{caveats: []string{"c"}, render: "r"})
		}},
		{"same claim cited twice", func() (Draft, error) {
			return Compose(KindAnswer, testTarget, "t", "It is {{figure:x}}.",
				fakeClaim{id: "x", render: "r", caveats: []string{"c"}, ok: true},
				fakeClaim{id: "x", render: "r", caveats: []string{"c"}, ok: true})
		}},
		{"citation with a blank provenance chip", func() (Draft, error) {
			return Compose(KindAnswer, testTarget, "t", "It is {{figure:x}}.",
				fakeClaim{id: "x", caveats: []string{"c"}, ok: true})
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := c.run()
			if err == nil {
				t.Fatal("Compose accepted malformed input")
			}
			if !d.Zero() {
				t.Fatal("a refused Compose returned a non-zero draft — a caller that drops the error would hold a partially valid one")
			}
		})
	}
}

// TestTheZeroDraftRendersAsAbsentNotEmpty — an empty draft on a screen looks
// like a draft with nothing to say. It has to look like a draft that does not
// exist, and it must not render a figure position at all.
func TestTheZeroDraftRendersAsAbsentNotEmpty(t *testing.T) {
	out := Draft{}.Render()
	if !strings.Contains(out, FigureUnavailable) {
		t.Errorf("the zero draft does not render as %s:\n%s", FigureUnavailable, out)
	}
	if !strings.HasPrefix(out, DraftBanner) {
		t.Error("the zero draft does not carry the draft banner")
	}
}

func TestPriorityKindComposes(t *testing.T) {
	d, err := Compose(KindPriority, Target{Repo: "example-org/example-repo", Kind: "stream", Ref: "example-stream"},
		"proposed priority change: drain before starting",
		"This stream holds {{figure:brief-status-count}} rows at todo; propose moving the drain ahead of new starts.",
		computed("brief-status-count", 7))
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if d.Kind() != KindPriority {
		t.Fatalf("kind = %q", d.Kind())
	}
	if !strings.Contains(d.Render(), string(KindPriority)) {
		t.Error("the rendered priority draft does not say what it proposes")
	}
}

func TestQuestionsIsSortedAndCitedIsOrdered(t *testing.T) {
	d := mustCompose(t)
	cited := d.Cited()
	if len(cited) != 2 || cited[0] != "brief-status-count" || cited[1] != "pr-action-count" {
		t.Fatalf("Cited() = %v, want citation order", cited)
	}
	qs := d.Questions()
	if len(qs) != 2 || qs[0] > qs[1] {
		t.Fatalf("Questions() = %v, want sorted", qs)
	}
	// Mutating the returned slices must not reach the draft.
	cited[0] = "mutated"
	if d.Cited()[0] == "mutated" {
		t.Fatal("Cited() hands out the draft's own backing array")
	}
}

// TestClaimIsTheAnswerLayersMethodSet pins the bridge to the read-only answer
// layer. This package deliberately does not import that layer (doing so would
// pull os/exec into its dependency closure), so the interface is satisfied
// implicitly — which means nothing in this tree would notice if the method set
// drifted. This test is what notices.
//
// The method set below was verified to be satisfied by the real type: the
// answer layer's branch was checked out into a scratch tree, a bridge
// assertion `var _ draft.Claim = askassay.Answer{}` compiled clean, and an
// end-to-end test composed a real draft from a real blind answer and a real
// measured one. That check cannot ship against an unmerged branch; this one
// can, and it fails the moment either side moves.
func TestClaimIsTheAnswerLayersMethodSet(t *testing.T) {
	typ := reflect.TypeOf((*Claim)(nil)).Elem()
	want := map[string]string{
		"Question": "func() string",
		"Render":   "func() string",
		"Caveats":  "func() []string",
		"Value":    "func() (int64, bool)",
	}
	if typ.NumMethod() != len(want) {
		t.Fatalf("Claim declares %d methods, want %d — a method added here must exist on the answer layer's Answer, or the two stop composing with no adapter", typ.NumMethod(), len(want))
	}
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		sig, ok := want[m.Name]
		if !ok {
			t.Errorf("Claim declares an undeclared method %q", m.Name)
			continue
		}
		if got := m.Type.String(); got != sig {
			t.Errorf("Claim.%s has signature %s, want %s", m.Name, got, sig)
		}
	}
	// No method may return a type this package defines: a named return type
	// would break implicit satisfaction and force an adapter, which is the
	// whole thing this interface exists to avoid.
	for i := 0; i < typ.NumMethod(); i++ {
		mt := typ.Method(i).Type
		for j := 0; j < mt.NumOut(); j++ {
			if pkg := mt.Out(j).PkgPath(); pkg != "" {
				t.Errorf("Claim.%s returns %s from package %q — only builtin types keep this interface implicitly satisfiable", typ.Method(i).Name, mt.Out(j), pkg)
			}
		}
	}
}
