package adapter_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

var update = flag.Bool("update", false, "rewrite the golden files from the current parse")

// goldenParse is the recorded parse of one fixture body.
type goldenParse struct {
	Encoding   adapter.OptionsEncoding `json:"encoding"`
	Options    []domain.Option         `json:"options"`
	OptionsRev string                  `json:"options_rev"`
	Problems   []string                `json:"problems,omitempty"`
	Confidence domain.Confidence       `json:"confidence"`
}

// TestOptionsEncodings is the read surface Verify row 2 and spec §5.5 08a row 1:
// golden fixtures for BOTH wire encodings parse — prose-v1 ⇒ Options = nil,
// typed-v2 ⇒ a typed list plus an OptionsRev.
//
// It is a golden-FILE test (design §11) rather than a table of inline
// expectations because the thing under test is a wire grammar: a fixture on
// disk is a body a human can paste into a real issue and compare, and a
// golden file makes a grammar change show up as a reviewable diff instead of
// a moved assertion.
//
// The malformed cases are the load-bearing half. Design §10's
// malformed-escalation rule says an unparseable escalation still appears,
// raw, and is never silently dropped — so the parse must distinguish three
// outcomes, not two:
//
//	prose-v1                 Options = nil, CHECKED-CLEAN  (there are none)
//	typed-v2                 Options = [...], CHECKED-CLEAN
//	typed-v2, did not parse  Options = nil, COULD-NOT-CHECK (there should be)
//
// Folding the third into the first is the whole failure this test exists to
// catch: a broken escalation would quietly render as a legacy one.
func TestOptionsEncodings(t *testing.T) {
	bodies, err := filepath.Glob("testdata/bodies/*.md")
	if err != nil || len(bodies) == 0 {
		t.Fatalf("could-not-check: no fixture bodies found (%v)", err)
	}

	for _, path := range bodies {
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("could-not-check: %v", err)
			}
			res := adapter.ParseOptions(string(raw))
			got := goldenParse{
				Encoding:   res.Encoding,
				Options:    res.Options,
				OptionsRev: res.Rev,
				Problems:   res.Problems,
				Confidence: res.Assessment().Confidence,
			}

			goldenPath := filepath.Join("testdata", "golden", name+".json")
			enc, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			enc = append(enc, '\n')
			if *update {
				if err := os.WriteFile(goldenPath, enc, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("could-not-check: golden file missing for %s (%v) — run `go test ./internal/adapter -update`", name, err)
			}
			if string(enc) != string(want) {
				t.Errorf("parse drifted from the golden record.\n--- got ---\n%s\n--- want ---\n%s", enc, want)
			}

			// Invariants that hold for every fixture, checked here so a
			// careless `-update` cannot bless a violation of the contract.
			switch res.Encoding {
			case adapter.EncodingProseV1:
				if res.Options != nil {
					t.Errorf("prose-v1 body produced a non-nil option list — spec §1.4: no marker ⇒ Options = nil")
				}
				if res.Rev != "" {
					t.Errorf("prose-v1 body produced a rev %q — the rev is \"\" when Options is nil", res.Rev)
				}
				if res.Assessment().Confidence != domain.CheckedClean {
					t.Errorf("prose-v1 is not a degraded parse; it must be checked-clean, got %s", res.Assessment().Confidence)
				}
			case adapter.EncodingTypedV2:
				if len(res.Options) == 0 {
					t.Fatalf("typed-v2 with an empty option list should have been classed malformed")
				}
				if res.Rev == "" {
					t.Errorf("typed-v2 produced no OptionsRev")
				}
				if res.Rev != domain.OptionsRev(res.Options) {
					t.Errorf("OptionsRev does not match a re-derivation from the parsed options")
				}
				seen := map[domain.OptionID]bool{}
				for _, o := range res.Options {
					if !o.ID.Valid() {
						t.Errorf("option id %q does not match the slug grammar", o.ID)
					}
					if seen[o.ID] {
						t.Errorf("duplicate option id %q survived the parse", o.ID)
					}
					seen[o.ID] = true
					if o.Label == "" || o.Consequence == "" {
						t.Errorf("option %q is missing a label or consequence", o.ID)
					}
				}
			case adapter.EncodingTypedV2Malformed:
				if res.Options != nil {
					t.Errorf("a malformed v2 block published %d option(s) — a partial list misnumbers the number keys the answer flow binds to", len(res.Options))
				}
				if len(res.Problems) == 0 {
					t.Errorf("malformed classification with no stated problem is not actionable")
				}
				if res.Assessment().Confidence != domain.CouldNotCheck {
					t.Errorf("a malformed v2 block must be could-not-check, got %s — otherwise a broken escalation renders as a legacy one",
						res.Assessment().Confidence)
				}
			}
		})
	}
}

// TestEmDashIsLoadBearing pins the one byte-level fact most likely to be
// "fixed" by a future reader: the id/label separator is U+2014, and an ASCII
// hyphen must NOT be accepted.
//
// Accepting both would be friendlier and wrong. `desk__escalate`
// emits the em-dash form and this parser round-trips it; a parser that also
// accepted a hyphen would let two spellings of the same grammar exist, and
// the emitter and the parser would stop being each other's test.
func TestEmDashIsLoadBearing(t *testing.T) {
	const emDash = "### Options\n<!-- assay:options v2 -->\n#### `a-id` — A label\nConsequence: a thing happens.\n"
	const ascii = "### Options\n<!-- assay:options v2 -->\n#### `a-id` - A label\nConsequence: a thing happens.\n"

	if got := adapter.ParseOptions(emDash); got.Encoding != adapter.EncodingTypedV2 {
		t.Fatalf("em-dash body did not parse as typed-v2: %s %v", got.Encoding, got.Problems)
	}
	got := adapter.ParseOptions(ascii)
	if got.Encoding != adapter.EncodingTypedV2Malformed {
		t.Fatalf("ASCII-hyphen body parsed as %s — the grammar's separator is U+2014 and a second accepted spelling breaks the emitter/parser round trip",
			got.Encoding)
	}
	if !strings.Contains(strings.Join(got.Problems, " "), "em-dash") {
		t.Errorf("the failure does not tell the reader what is wrong: %v", got.Problems)
	}
}

// TestOptionsRevIsBoundaryProof guards the fingerprint against the classic
// concatenation collision. The rev crosses the wire as a shorthash and an
// answer either matches it exactly or is detectably stale — so two different
// option sets hashing alike would make a stale answer look current.
func TestOptionsRevIsBoundaryProof(t *testing.T) {
	a := []domain.Option{{ID: "ab", Label: "c", Consequence: "d"}}
	b := []domain.Option{{ID: "a", Label: "bc", Consequence: "d"}}
	if domain.OptionsRev(a) == domain.OptionsRev(b) {
		t.Fatal("two different option sets share a rev — the length-prefixed encoding is not doing its job")
	}
	if domain.OptionsRev(nil) != "" {
		t.Fatal("a nil option set must fingerprint as the empty string, not as a hash of nothing")
	}
	// Order is part of the identity: the spec says ORDERED set, and the
	// render binds number keys positionally.
	c := []domain.Option{{ID: "x", Label: "l", Consequence: "q"}, {ID: "y", Label: "m", Consequence: "r"}}
	d := []domain.Option{{ID: "y", Label: "m", Consequence: "r"}, {ID: "x", Label: "l", Consequence: "q"}}
	if domain.OptionsRev(c) == domain.OptionsRev(d) {
		t.Fatal("reordering the options did not change the rev, but the spec fingerprints an ORDERED set")
	}
}
