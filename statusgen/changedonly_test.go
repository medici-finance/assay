package main

import (
	"strings"
	"testing"
)

// TestChangedOnlyRefusesInCIGate is Verify row 11's negative path: --lint --changed-only <paths>
// under the CI-gate environment REFUSES non-zero and names the requirement; outside it, it runs
// and prints the scope banner. The named mutation (row 11's table) is dropping the CI-gate
// detection so --changed-only merely warns instead of refusing — this test fails on that
// mutation because it asserts the REFUSAL result, not merely that something was printed.
//
// Every flag-combination row asserts the refusal wins over every other outcome: a usage error
// (both --changed and --changed-only, or no --lint, or an empty path list) must NEVER surface
// once inCI is true — the refusal has no override, including a flag combination that would
// otherwise itself be an error.
func TestChangedOnlyRefusesInCIGate(t *testing.T) {
	cases := []struct {
		name             string
		raw              string
		changedFileGiven bool
		lintMode         bool
		inCI             bool
		wantRefusal      bool
		wantUsageErr     bool
		wantPaths        []string
	}{
		{name: "not given at all — inert", raw: "", lintMode: true, inCI: true, wantRefusal: false, wantUsageErr: false},
		{name: "CI + lint + valid paths → REFUSED", raw: "docs/streams/x.md,statusgen/foo.go", lintMode: true, inCI: true, wantRefusal: true},
		{name: "CI + NOT lint → still REFUSED (CI wins over the usage error)", raw: "docs/streams/x.md", lintMode: false, inCI: true, wantRefusal: true},
		{name: "CI + both --changed and --changed-only → still REFUSED (CI wins over the usage error)", raw: "docs/streams/x.md", changedFileGiven: true, lintMode: true, inCI: true, wantRefusal: true},
		{name: "CI + empty-after-parse value → still REFUSED (CI wins over the usage error)", raw: " , ,, ", lintMode: true, inCI: true, wantRefusal: true},
		{name: "outside CI + lint + valid paths → runs, banner set", raw: "docs/streams/x.md, statusgen/foo.go", lintMode: true, inCI: false, wantPaths: []string{"docs/streams/x.md", "statusgen/foo.go"}},
		{name: "outside CI + not lint → usage error", raw: "docs/streams/x.md", lintMode: false, inCI: false, wantUsageErr: true},
		{name: "outside CI + both --changed and --changed-only → usage error", raw: "docs/streams/x.md", changedFileGiven: true, lintMode: true, inCI: false, wantUsageErr: true},
		{name: "outside CI + only commas/blank → usage error", raw: " , ,, ", lintMode: true, inCI: false, wantUsageErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := resolveChangedOnly(c.raw, c.changedFileGiven, c.lintMode, c.inCI)
			if c.wantRefusal {
				if r.Refusal == "" {
					t.Fatalf("expected a CI-gate refusal, got none (result: %+v)", r)
				}
				if r.UsageErr != "" {
					t.Fatalf("refusal case must not ALSO carry a usage error: %q", r.UsageErr)
				}
				if len(r.Paths) != 0 {
					t.Fatalf("a refused run must scope nothing, got paths %v", r.Paths)
				}
				if !strings.Contains(r.Refusal, "CI") && !strings.Contains(r.Refusal, "GITHUB_ACTIONS") {
					t.Fatalf("refusal message does not name the CI-gate reason: %q", r.Refusal)
				}
				return
			}
			if r.Refusal != "" {
				t.Fatalf("did not expect a refusal, got %q", r.Refusal)
			}
			if c.wantUsageErr {
				if r.UsageErr == "" {
					t.Fatalf("expected a usage error, got none (result: %+v)", r)
				}
				return
			}
			if r.UsageErr != "" {
				t.Fatalf("did not expect a usage error, got %q", r.UsageErr)
			}
			if c.wantPaths == nil {
				// The "not given at all" case: everything must stay zero-valued.
				if len(r.Paths) != 0 || r.Banner != "" {
					t.Fatalf("expected an inert (zero-value) result, got %+v", r)
				}
				return
			}
			if len(r.Paths) != len(c.wantPaths) {
				t.Fatalf("paths = %v, want %v", r.Paths, c.wantPaths)
			}
			for i, p := range c.wantPaths {
				if r.Paths[i] != p {
					t.Fatalf("paths[%d] = %q, want %q", i, r.Paths[i], p)
				}
			}
			if r.Banner == "" {
				t.Fatalf("expected a non-empty scope banner on the success path")
			}
			for _, p := range c.wantPaths {
				if !strings.Contains(r.Banner, p) {
					t.Fatalf("banner does not name examined path %q: %q", p, r.Banner)
				}
			}
			if !strings.Contains(strings.ToLower(r.Banner), "not looked for") {
				t.Fatalf("banner does not state that a full-tree defect outside the set was not looked for: %q", r.Banner)
			}
		})
	}
}

// TestChangedOnlyRefusalHasNoOverride is a second, narrower proof for the same row: the refusal
// function itself takes only the CI bool — there is no second parameter it could consult to
// waive the refusal, so a caller cannot construct an override by any argument order.
func TestChangedOnlyRefusalHasNoOverride(t *testing.T) {
	if got := changedOnlyRefusal(false); got != "" {
		t.Fatalf("changedOnlyRefusal(false) = %q, want empty", got)
	}
	if got := changedOnlyRefusal(true); got == "" {
		t.Fatalf("changedOnlyRefusal(true) = empty, want a refusal message")
	}
}

func TestChangedOnlyBannerListsEveryPath(t *testing.T) {
	paths := []string{"docs/streams/forge-neutral/brief-18.md", "statusgen/main.go", "tools/desk/cmd/deskread/main.go"}
	banner := changedOnlyBanner(paths)
	for _, p := range paths {
		if !strings.Contains(banner, p) {
			t.Fatalf("banner missing path %q:\n%s", p, banner)
		}
	}
	if !strings.Contains(banner, "3 path") {
		t.Fatalf("banner does not name the count of paths examined:\n%s", banner)
	}
}

func TestParseChangedOnlyDropsBlanksAndTrims(t *testing.T) {
	got := parseChangedOnly(" docs/a.md ,, statusgen/b.go ,  ")
	want := []string{"docs/a.md", "statusgen/b.go"}
	if len(got) != len(want) {
		t.Fatalf("parseChangedOnly = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseChangedOnly[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
