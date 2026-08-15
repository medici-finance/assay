package draft

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// TestDraftSurvivesARoundTrip is the executable half of §7.6: a composed
// draft is never lost before the human sees it. The store belongs to a shell
// that does not exist; the FORM is what can be built and falsified here.
func TestDraftSurvivesARoundTrip(t *testing.T) {
	orig := mustCompose(t)
	back, err := Decode(orig.Encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if back.Render() != orig.Render() {
		t.Fatalf("the restored draft does not render identically:\nwant:\n%s\ngot:\n%s", orig.Render(), back.Render())
	}
	if back.Encode() != orig.Encode() {
		t.Fatal("encoding is not deterministic across a round trip")
	}
}

// TestRoundTripSurvivesAdversarialContent — the encoding is tab- and
// newline-delimited, so the content most likely to break it is content that
// contains tabs, newlines and backslashes. A caveat lost to a bad escape is
// the same failure as a caveat lost to a truncation.
func TestRoundTripSurvivesAdversarialContent(t *testing.T) {
	nasty := fakeClaim{
		id:      "nasty-claim",
		render:  "nasty-claim: figure=1 · source=a\tb\nc\\d · probe=p · window=w · limit=l",
		caveats: []string{"a caveat with\na newline", "one with\ta tab", `one with a \\ backslash pair`, `trailing \n literal`},
		value:   1,
		ok:      true,
	}
	orig, err := Compose(KindAnswer, Target{Repo: "example-org/example-repo", Kind: "pull-request", Ref: "head"},
		"title with\ta tab", "It is {{figure:nasty-claim}} — and this line has a\ttab.", nasty)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	back, err := Decode(orig.Encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if back.Render() != orig.Render() {
		t.Fatalf("adversarial content did not survive:\nwant:\n%q\ngot:\n%q", orig.Render(), back.Render())
	}
	got := back.Caveats()
	if len(got) != len(nasty.caveats) {
		t.Fatalf("restored %d caveats, want %d", len(got), len(nasty.caveats))
	}
	for i := range got {
		if got[i] != nasty.caveats[i] {
			t.Errorf("caveat %d = %q, want %q", i, got[i], nasty.caveats[i])
		}
	}
}

// TestADamagedRestoreIsRefusedWhole — the failure this fails closed against is
// a body that survives while a caveat does not, leaving a confident claim with
// its qualification silently stripped in front of a human about to sign it.
func TestADamagedRestoreIsRefusedWhole(t *testing.T) {
	good := mustCompose(t).Encode()
	lines := strings.Split(strings.TrimRight(good, "\n"), "\n")

	drop := func(pred func(string) bool) string {
		var out []string
		for _, ln := range lines {
			if pred(ln) {
				continue
			}
			out = append(out, ln)
		}
		return strings.Join(out, "\n") + "\n"
	}

	cases := []struct {
		name    string
		payload string
	}{
		{"a caveat line dropped", drop(func(s string) bool { return strings.HasPrefix(s, "caveat\t") })},
		{"a citation line dropped", drop(func(s string) bool { return strings.HasPrefix(s, "cite\t") })},
		{"the body altered", strings.Replace(good, "Recommend draining", "Recommend merging", 1)},
		{"truncated before the digest", strings.Join(lines[:len(lines)-3], "\n") + "\n"},
		{"no digest at all", drop(func(s string) bool { return strings.HasPrefix(s, "sum\t") })},
		{"wrong version banner", strings.Replace(good, encodeVersion, "draft/v2", 1)},
		{"an undeclared field spliced in", strings.Replace(good, "title\t", "approved\ttrue\ntitle\t", 1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := Decode(c.payload)
			if err == nil {
				t.Fatal("a damaged payload restored cleanly")
			}
			if !d.Zero() {
				t.Fatal("a refused Decode returned a non-zero draft — a partial restore is the failure this check exists for")
			}
		})
	}

	// The control in the other direction: the undamaged payload restores.
	if _, err := Decode(good); err != nil {
		t.Fatalf("the undamaged payload was refused: %v — the checks above would then be satisfied by refusing everything", err)
	}
}

// TestDecodeRefusesAWellFormedPayloadThatBreaksTheCompositionRules — a valid
// digest proves the bytes are intact, not that they came from Compose. A
// hand-written payload must still meet the rules a composed draft meets.
func TestDecodeRefusesAWellFormedPayloadThatBreaksTheCompositionRules(t *testing.T) {
	forge := func(d Draft) string { return d.Encode() }

	cases := []struct {
		name  string
		draft Draft
		want  error
	}{
		{"undeclared kind", Draft{kind: "proposed-merge", target: testTarget, title: "t", body: "b"}, ErrMalformed},
		{"no target", Draft{kind: KindAnswer, title: "t", body: "b"}, ErrMalformed},
		{"no body", Draft{kind: KindAnswer, target: testTarget, title: "t"}, ErrMalformed},
		{"cites a claim but carries no caveat",
			Draft{kind: KindAnswer, target: testTarget, title: "t", body: "b", cited: []string{"x"}, chips: []string{"chip"}},
			ErrUncaveatedClaim},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := Decode(forge(c.draft))
			if !errors.Is(err, c.want) {
				t.Fatalf("Decode = %v, want %v", err, c.want)
			}
			if !d.Zero() {
				t.Fatal("a refused Decode returned a non-zero draft")
			}
		})
	}
}

// TestEscapingRoundTripsOverlappingSequences — the reason unesc is a scan and
// not three ReplaceAll calls. Sequential replacement turns a literal backslash
// followed by n into a newline, which silently rewrites content.
func TestEscapingRoundTripsOverlappingSequences(t *testing.T) {
	for _, s := range []string{`\n`, `\\n`, `\t`, `\\`, "\n", "\t", `a\b`, `\`, ""} {
		got, err := unesc(esc(s))
		if err != nil {
			t.Fatalf("unesc(esc(%q)): %v", s, err)
		}
		if got != s {
			t.Errorf("round trip of %q produced %q", s, got)
		}
	}
	if _, err := unesc(`a\`); !errors.Is(err, ErrMalformed) {
		t.Errorf("a dangling escape decoded to %v, want ErrMalformed", err)
	}
	if _, err := unesc(`a\q`); !errors.Is(err, ErrMalformed) {
		t.Errorf("an undeclared escape decoded to %v, want ErrMalformed", err)
	}
}

// TestARestoredDraftStillCannotPost — persistence must not be a way around the
// invariant. A draft that came back from a store is refused at exactly the
// same hand-off destinations as one just composed.
func TestARestoredDraftStillCannotPost(t *testing.T) {
	back, err := Decode(mustCompose(t).Encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var checked int
	for _, dest := range Destinations() {
		if !dest.Transmits {
			continue
		}
		checked++
		if _, err := back.HandOff(dest.ID); !errors.Is(err, ErrNoAutoPost) {
			t.Errorf("a restored draft handed off to %q returned %v, want ErrNoAutoPost", dest.ID, err)
		}
	}
	if checked == 0 {
		t.Fatal("no transmitting destination was checked — vacuous")
	}
}

// resign rebuilds a valid digest over a hand-edited canonical body. It exists
// because the digest check would otherwise MASK every other refusal in Decode:
// a spliced field changes the content, so the payload fails on the digest and
// the field-level arm is never reached. A properly signed forgery is the only
// input that exercises those arms — and it is also the realistic adversary,
// since anything that can edit a payload can re-sign it.
func resign(canonicalLines []string) string {
	body := strings.Join(canonicalLines, "\n") + "\n"
	sum := sha256.Sum256([]byte(body))
	return body + "sum\t" + hex.EncodeToString(sum[:]) + "\n"
}

func canonicalLinesOf(t *testing.T, payload string) []string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(payload, "\n"), "\n")
	for i, ln := range lines {
		if strings.HasPrefix(ln, "sum\t") {
			return lines[:i]
		}
	}
	t.Fatal("payload carries no digest line")
	return nil
}

// TestDecodeRefusesAValidlySignedForgery — a correct digest proves the bytes
// are intact. It proves nothing about whether they say something legitimate.
// Every case below carries a digest that VERIFIES, so each one reaches an arm
// that the digest check would otherwise hide.
func TestDecodeRefusesAValidlySignedForgery(t *testing.T) {
	base := canonicalLinesOf(t, mustCompose(t).Encode())

	splice := func(after, line string) []string {
		var out []string
		for _, ln := range base {
			out = append(out, ln)
			if strings.HasPrefix(ln, after) {
				out = append(out, line)
			}
		}
		return out
	}

	cases := []struct {
		name  string
		lines []string
	}{
		{"an undeclared field, correctly signed", splice("title\t", "approved\ttrue")},
		{"a field that looks like an authorization", splice("title\t", "authority\thuman-signed")},
		{"a citation line with no provenance chip", splice("title\t", "cite\tsmuggled-claim")},
		{"a field name that is a near-miss for a real one", splice("title\t", "caveats\tone caveat, misspelled")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			payload := resign(c.lines)
			// The forgery must genuinely pass the digest, or this test is
			// measuring the digest check again rather than the arm below it.
			if _, err := Decode(strings.Replace(payload, "approved", "approved", 1)); err == nil {
				t.Fatal("a validly signed forgery decoded cleanly — Decode ignored a field it does not understand, which is how a caveat field silently stops being read")
			} else if strings.Contains(err.Error(), "digest does not match") {
				t.Fatalf("this case was rejected by the DIGEST, not by the field arm it exists to test: %v", err)
			}
		})
	}

	// The control in the other direction: re-signing an unmodified body must
	// still decode, or `resign` is simply producing garbage.
	if _, err := Decode(resign(base)); err != nil {
		t.Fatalf("a re-signed unmodified payload was refused: %v — resign is broken, so the cases above proved nothing", err)
	}
}
