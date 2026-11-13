package evaluation

import (
	"slices"
	"testing"
)

// regression: #2026
// TestCoverageTokenNotJudged — build-less-brittle/15, review S20: in a build
// with no read-set recorder (CoverageInput.ReadSetsRecorded false), a
// `(reads sha256:…)` token is never judged. Each row comes out as the same
// row with no token, whatever its set records, however its file was
// observed and whatever the later commit changed; its reason only gains the
// note naming the missing recorder. The same fixtures judged as recorded
// still narrow, so the switch is what holds them.
func TestCoverageTokenNotJudged(t *testing.T) {
	unrelated := edit(".claude-plugin/marketplace.json", "{\"v\":2}\n")
	cases := []struct {
		name     string
		setup    func(*rsFx)
		change   func(map[string]fakeEntry, map[string][]string)
		held     bool // the row with no token is held
		narrowed bool // judged as recorded, the token releases it
	}{
		{"listing recorded, unrelated change", nil, unrelated, true, true},
		{"no listing, compiled file added", func(fx *rsFx) {
			fx.entries = slices.DeleteFunc(slices.Clone(fx.entries), func(e string) bool { return e == "list tools/x" })
		}, edit("tools/x/c.go", "package x\n\nfunc C() int { return 3 }\n"), true, true},
		{"no compile step declared, compiled code edited", func(fx *rsFx) {
			fx.entries = []string{"read tools/x/go.mod", "read tools/x/a.go"}
		}, edit("tools/x/b.go", "package x\n\nfunc B() int { return 3 }\n"), true, true},
		{"no introduction binding, unrelated change", func(fx *rsFx) {
			fx.obs = func(o *ReadSetObservation) { o.IntroFile = false }
		}, unrelated, true, false},
		{"file absent", func(fx *rsFx) {
			fx.obs = func(o *ReadSetObservation) { o.Found = false }
		}, nil, false, false},
		{"file over the bounds", func(fx *rsFx) {
			fx.obs = func(o *ReadSetObservation) { o.Problem = "fixture bound" }
		}, nil, false, false},
		{"unchanged", nil, nil, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			claim := func(token string, unrecorded bool) Claim {
				fx := newRsFx(t)
				if c.setup != nil {
					c.setup(fx)
				}
				fx.token, fx.unrecorded = token, unrecorded
				return fx.item(c.change)
			}
			got, twin := claim("sha256", true), claim("none", true)
			if got.Result != twin.Result || got.Released != twin.Released || got.Revision != twin.Revision {
				t.Fatalf("token row %s/%v at %s (%s); want the row without it: %s/%v at %s (%s)",
					got.Result, got.Released, got.Revision, got.Reason, twin.Result, twin.Released, twin.Revision, twin.Reason)
			}
			if want := twin.Reason + "; " + ReadSetNotJudged; got.Reason != want {
				t.Errorf("reason %q, want %q", got.Reason, want)
			}
			if c.held && twin.Released {
				t.Errorf("the row without a token released (%s): the fixture no longer changes its scope", twin.Reason)
			}
			if c.narrowed {
				if rec := claim("sha256", false); !rec.Released {
					t.Errorf("judged as recorded, want the token to release; got %s (%s)", rec.Result, rec.Reason)
				}
			}
		})
	}
}
