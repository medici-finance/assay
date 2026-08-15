package askassay

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestGuardRefusesEveryWriteShape — default-deny, demonstrated. The permitted
// half is asserted too: a guard that refuses everything is not read-only, it
// is broken.
func TestGuardRefusesEveryWriteShape(t *testing.T) {
	permitted := [][]string{
		{"gh", "api", "graphql", "-f", "query=query{viewer{login}}"},
		{"gh", "api", "graphql", "-f", "query=query{viewer{login}}", "-F", "o=example-org", "-F", "r=loans"},
		{"gh", "api", "repos/example-org/loans/issues", "-X", "GET"},
		// Field flags on a REST path ARE permitted once the read method is
		// stated: that is gh's documented way to send them as a query string.
		// Without this control the new rule could be satisfied by refusing
		// every field flag, which would refuse the registry's own count probe.
		{"gh", "api", "repos/example-org/loans/issues", "--method", "GET", "-f", "state=open"},
		{"gh", "api", "repos/example-org/loans/issues", "-XGET", "-fstate=open"},
		{"gh", "issue", "list", "-R", "example-org/loans", "--state", "open"},
		{"gh", "pr", "view", "12", "--json", "number"},
		{"gh", "run", "list", "--limit", "5"},
		{"statusgen", "--root", ".", "--lint"},
		{"statusgen", "--root", ".", "--json"},
		{"statusgen", "--root", ".", "--dora", "--json"},
		{"deskboard", "actions", "--json"},
		{"deskboard", "prs"},
		{"deskboard", "health", "--json"},
	}
	for _, argv := range permitted {
		if err := GuardReadOnly(argv); err != nil {
			t.Errorf("guard refused a declared read probe %v: %v", argv, err)
		}
	}

	writeShapes := []struct {
		name string
		argv []string
	}{
		{"no argv at all", nil},
		{"git", []string{"git", "push"}},
		{"curl", []string{"curl", "-XPOST", "https://example.invalid"}},
		{"shell", []string{"sh", "-c", "gh issue list"}},
		{"deskpost", []string{"deskpost", "comment", "--pr", "1"}},
		{"desktoken", []string{"desktoken", "mint"}},
		{"gh with no subcommand", []string{"gh"}},
		{"gh issue create", []string{"gh", "issue", "create", "-t", "x"}},
		{"gh issue comment", []string{"gh", "issue", "comment", "1", "-b", "x"}},
		{"gh pr create", []string{"gh", "pr", "create", "--draft"}},
		{"gh pr merge", []string{"gh", "pr", "merge", "1"}},
		{"gh pr review", []string{"gh", "pr", "review", "1", "--approve"}},
		{"gh repo", []string{"gh", "repo", "edit"}},
		{"gh workflow run", []string{"gh", "workflow", "run", "tools.yml"}},
		{"gh api POST split", []string{"gh", "api", "repos/x/y/issues", "-X", "POST"}},
		{"gh api PATCH joined", []string{"gh", "api", "repos/x/y/issues/1", "-XPATCH"}},
		{"gh api DELETE long flag", []string{"gh", "api", "repos/x/y", "--method", "DELETE"}},
		{"gh api lowercase post", []string{"gh", "api", "repos/x/y", "-X", "post"}},
		{"gh api graphql mutation", []string{"gh", "api", "graphql", "-f", "query=mutation{addComment(input:{}){clientMutationId}}"}},

		// The IMPLICIT-POST family. gh switches an api call to POST as soon as
		// request parameters are added, so every one of these is a write with
		// no method token and no mutation keyword anywhere in the argv. This
		// was the gap: the guard only looked for an explicit method flag.
		{"gh api implicit POST via -f", []string{"gh", "api", "repos/x/y/issues", "-f", "title=x"}},
		{"gh api implicit POST via --raw-field", []string{"gh", "api", "repos/x/y/issues", "--raw-field", "title=x"}},
		{"gh api implicit POST via -F", []string{"gh", "api", "repos/x/y/issues", "-F", "title=x"}},
		{"gh api implicit POST via --field", []string{"gh", "api", "repos/x/y/issues", "--field", "title=x"}},
		{"gh api implicit POST joined shorthand", []string{"gh", "api", "repos/x/y/issues", "-ftitle=x"}},
		{"gh api implicit POST joined long form", []string{"gh", "api", "repos/x/y/issues", "--field=title=x"}},
		{"gh api implicit POST beside an unrelated read flag", []string{"gh", "api", "repos/x/y/issues", "--jq", ".number", "-f", "title=x"}},

		// A body file forces POST and is opaque to the guard.
		{"gh api --input body file", []string{"gh", "api", "repos/x/y/rulesets", "--input", "body.json"}},
		{"gh api --input joined", []string{"gh", "api", "repos/x/y/rulesets", "--input=body.json"}},
		{"gh api --input stdin", []string{"gh", "api", "repos/x/y/rulesets", "--input", "-"}},

		// Value indirection on the graphql path: field flags are permitted
		// there, so an @file value would be the way to smuggle a mutation past
		// the keyword scan.
		{"gh api graphql query from a file", []string{"gh", "api", "graphql", "-F", "query=@op.graphql"}},
		{"gh api graphql query from stdin", []string{"gh", "api", "graphql", "-F", "query=@-"}},

		// Method allow-list, not write-verb deny-list.
		{"gh api PUT", []string{"gh", "api", "repos/x/y/z", "-X", "PUT"}},
		{"gh api method with no value", []string{"gh", "api", "repos/x/y/z", "--method"}},
		{"gh api unknown method", []string{"gh", "api", "repos/x/y/z", "-X", "PURGE"}},
		{"gh api method as a query-string dodge", []string{"gh", "api", "repos/x/y/z", "--method=OPTIONS"}},
		{"gh non-api subcommand carrying a method flag", []string{"gh", "pr", "list", "-X", "POST"}},

		{"statusgen --bottleneck writes a dated report", []string{"statusgen", "--root", ".", "--bottleneck"}},
		{"statusgen --bottleneck beside a read flag", []string{"statusgen", "--root", ".", "--json", "--bottleneck"}},
		{"statusgen default mode writes", []string{"statusgen", "--root", "."}},
		{"statusgen --record", []string{"statusgen", "--root", ".", "--record"}},
		{"statusgen --record beside a read flag", []string{"statusgen", "--root", ".", "--json", "--record"}},
		{"statusgen --scan-issues", []string{"statusgen", "--root", ".", "--scan-issues"}},
		{"statusgen --close-verify", []string{"statusgen", "--root", ".", "--close-verify=stream/01"}},
		{"statusgen --register-links", []string{"statusgen", "--root", ".", "--register-links"}},
		{"statusgen --roadmap", []string{"statusgen", "--root", ".", "--roadmap"}},
		{"statusgen --export-evidence", []string{"statusgen", "--root", ".", "--export-evidence"}},
		{"deskboard with no verb", []string{"deskboard", "--json"}},
		{"deskboard claim", []string{"deskboard", "claim"}},
		{"deskboard ready", []string{"deskboard", "ready", "--pr", "1"}},
	}
	for _, c := range writeShapes {
		t.Run(c.name, func(t *testing.T) {
			err := GuardReadOnly(c.argv)
			if err == nil {
				t.Fatalf("guard PERMITTED a write shape: %v", c.argv)
			}
			if !errors.Is(err, ErrRefused) {
				t.Errorf("refusal is not ErrRefused (%v) — refusals must be one recognisable class", err)
			}
		})
	}
}

// TestRefusedArgvNeverReachesASubprocess — the guard runs BEFORE the seam, not
// alongside it.
func TestRefusedArgvNeverReachesASubprocess(t *testing.T) {
	var reached bool
	r := Runner{run: func(context.Context, string, []string) ([]byte, error) {
		reached = true
		return nil, nil
	}}
	if _, err := r.Run(context.Background(), []string{"gh", "pr", "merge", "1"}); err == nil {
		t.Fatal("Run permitted a merge")
	}
	if reached {
		t.Fatal("a refused argv reached the subprocess seam — the guard is advisory, not structural")
	}
	if _, err := r.Run(context.Background(), []string{"deskboard", "prs", "--json"}); err != nil {
		t.Fatalf("Run refused a declared read probe: %v", err)
	}
	if !reached {
		t.Fatal("a permitted argv did not reach the seam")
	}
}

// TestEmptyResultIsBlindNotIdle — an empty payload does not become a zero
// unless the question has argued, in writing, that it should.
func TestEmptyResultIsBlindNotIdle(t *testing.T) {
	q := wellFormed()
	st, why := Classify(q, []byte("   \n "), nil)
	if st != CouldNotCheck {
		t.Fatalf("empty payload: state = %q, want %q", st, CouldNotCheck)
	}
	if !strings.Contains(why, "blind") {
		t.Errorf("reason does not name the blindness: %q", why)
	}

	declared := wellFormed()
	declared.EmptyMeansZero = true
	declared.EmptyRationale = "the emitter writes one line per row and one line per zero-row summary, so no output means the command did not run"
	if st, _ := Classify(declared, []byte(""), nil); st != Checked {
		t.Errorf("a question that declares its empty payload trustworthy still refused it: %q", st)
	}
}

// TestThrottledProbeIsBlindEvenOnCleanExit — this is the shape that produced a
// confident wrong answer: exit 0, a body that says 403, and a caller that
// counted the rows it got.
func TestThrottledProbeIsBlindEvenOnCleanExit(t *testing.T) {
	q := wellFormed()
	for _, body := range []string{
		`{"message":"API rate limit exceeded for user"}`,
		`You have exceeded a secondary rate limit`,
		`gh: HTTP 403: Resource not accessible`,
		`gh: Bad credentials (HTTP 401)`,
		`{"message":"Could not resolve to a Repository"}`,
	} {
		st, why := Classify(q, []byte(body), nil) // nil error: the call "succeeded"
		if st != CouldNotCheck {
			t.Errorf("body %q classified %q on a clean exit, want %q", body, st, CouldNotCheck)
		}
		if !strings.Contains(why, "BLIND") {
			t.Errorf("body %q: reason does not say the probe was blind: %q", body, why)
		}
	}
	// Positive control: a real payload must NOT be classified blind, or the
	// pane answers could-not-check to everything and the check is worthless.
	if st, _ := Classify(q, []byte(`[{"number":1},{"number":2}]`), nil); st != Checked {
		t.Fatalf("POSITIVE CONTROL FAILED: a good payload classified %q", st)
	}
}

// declaredBlindPatterns is the test-side copy of the blindness roster. It is
// hand-written ON PURPOSE: a test that iterates the production list proves
// nothing, because deleting a pattern deletes its own test case with it. That
// is not hypothetical — it was MEASURED. A mutation that removed one pattern
// from blindPatterns left this suite green, because two of the entries
// happened to cover the same fixture. This roster is what closes that hole:
// removing a pattern from probe.go now reddens the equality check below.
var declaredBlindPatterns = []string{
	"api rate limit exceeded",
	"secondary rate limit",
	"you have exceeded a secondary rate limit",
	"was submitted too quickly",
	"abuse detection",
	"http 403",
	"http 401",
	"http 502",
	"http 503",
	"bad credentials",
	"could not resolve to a repository",
	"gh: not found",
	"context deadline exceeded",
	"signal: killed",
}

// TestEveryBlindPatternIsIndividuallyProven — each declared pattern must, on
// its own, turn a clean-exit payload into could-not-check.
func TestEveryBlindPatternIsIndividuallyProven(t *testing.T) {
	if len(blindPatterns) != len(declaredBlindPatterns) {
		t.Fatalf("probe.go declares %d blindness patterns, this test declares %d — a pattern added or removed without a matching edit here is a blindness the suite cannot see",
			len(blindPatterns), len(declaredBlindPatterns))
	}
	have := map[string]bool{}
	for _, p := range blindPatterns {
		have[p] = true
	}
	for _, want := range declaredBlindPatterns {
		if !have[want] {
			t.Errorf("pattern %q is declared here but is GONE from probe.go — a probe failing that way would now render as a number", want)
			continue
		}
		// Prove it on its own, in a payload containing nothing else that could
		// trip a sibling pattern.
		body := "upstream said: " + strings.ToUpper(want) + " [end]"
		st, why := Classify(wellFormed(), []byte(body), nil)
		if st != CouldNotCheck {
			t.Errorf("pattern %q on a clean exit classified %q, want %q", want, st, CouldNotCheck)
		}
		if !strings.Contains(why, "BLIND") {
			t.Errorf("pattern %q: reason does not say the probe was blind: %q", want, why)
		}
	}
}

// TestRefusedProbeClassifiesAsCouldNotCheck — a refusal is not a zero either.
func TestRefusedProbeClassifiesAsCouldNotCheck(t *testing.T) {
	st, why := Classify(wellFormed(), nil, ErrRefused)
	if st != CouldNotCheck {
		t.Fatalf("state = %q, want %q", st, CouldNotCheck)
	}
	if !strings.Contains(why, "REFUSED") {
		t.Errorf("reason does not name the refusal: %q", why)
	}
}

// TestEndToEndBlindProbeRendersCouldNotCheck is the brief's positive control,
// run as a test: a question whose data source is unavailable renders
// could-not-check, not 0.
func TestEndToEndBlindProbeRendersCouldNotCheck(t *testing.T) {
	q, ok := Lookup("pr-action-count")
	if !ok {
		t.Fatal("pr-action-count is not declared")
	}
	r := Runner{run: func(context.Context, string, []string) ([]byte, error) {
		return []byte(`{"message":"You have exceeded a secondary rate limit"}`), nil
	}}
	out, err := r.Run(context.Background(), []string{"deskboard", "actions", "--json"})
	st, why := Classify(q, out, err)
	var a Answer
	if st == Checked {
		a = Computed(q, 0, testStamp()) // what a naive pane would do
	} else {
		a = Unavailable(q, why, testStamp())
	}
	if got := figureOf(t, a); got != FigureField {
		t.Fatalf("a throttled probe rendered figure=%q — the pane published a number it never measured", got)
	}
	if !strings.Contains(a.Render(), "could-not-check") {
		t.Errorf("rendered answer does not carry the could-not-check state:\n%s", a.Render())
	}
}
