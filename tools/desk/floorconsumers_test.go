package desk_test

// This is the FLOOR-CONSUMER GUARD (C1, brief-06 / #336). The model-capability floor's
// actor check is ONE predicate — deskkit.IsStampAuthorityLogin, which honours the bound
// dispatcher slugs PLUS the roster-configured ASSAY_STAMP_TRUSTED_LOGINS allowance. Every
// consumer must inject that SAME predicate; injecting the narrower deskkit.IsDispatcherLogin
// instead silently drops the allowance for that one call site — a PR whose stamp was
// legitimately widened under #336 would read Indeterminate there and nowhere else.
//
// WHY A SOURCE-LEVEL TEST AND NOT A UNIT TEST PER PACKAGE. Each call site lives in a
// DIFFERENT command package (deskflip, deskpost, deskautolane, deskdispatch), each with its
// own fake-forge suite that never varies which predicate production code happens to pass —
// the suites assert on the DECISION a stamp produces, not on which literal identifier
// supplied the predicate function. Reverting any one call site from
// deskkit.IsStampAuthorityLogin to deskkit.IsDispatcherLogin therefore changes production
// behaviour (the allowance stops applying there) while every existing test in that package's
// own suite keeps passing — exactly the six SURVIVING mutants the review found. A guard at
// the source level is the one thing that can fail on the substitution: it reads each known
// call site's own call expression and asserts the predicate argument is the wide one.
//
// This is a MODULE-ROOT test (like TestLayout in layout_test.go) so it runs whichever
// package's suite is invoked and is not itself scoped to any one command package.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// floorCallSite names one place a model-capability floor reader is invoked with an actor
// predicate — the six the review enumerated (C1).
type floorCallSite struct {
	file   string
	callee string // the deskkit function name the call site invokes
}

var floorCallSites = []floorCallSite{
	{"cmd/deskflip/flip.go", "ModelCapabilityFloor"},
	{"cmd/deskpost/ready.go", "ModelCapabilityFloor"},
	{"cmd/deskpost/review.go", "ModelCapabilityFloor"},
	{"cmd/deskpost/review.go", "ModelCapabilityFloorRiskAware"},
	{"cmd/deskautolane/lane.go", "AttestedModelStampOf"},
	{"cmd/deskdispatch/dispatch.go", "ReStampRemovals"},
}

// TestFloorConsumersInjectStampAuthorityLogin fails if any of the six known floor-reader
// call sites does not carry deskkit.IsStampAuthorityLogin as its actor predicate — in
// particular, if it has been reverted (by hand, or by a future edit) to the narrower
// deskkit.IsDispatcherLogin, which silently drops the #336 roster-configured allowance for
// that one caller.
func TestFloorConsumersInjectStampAuthorityLogin(t *testing.T) {
	callRe := regexp.MustCompile(`deskkit\.(ModelCapabilityFloorRiskAware|ModelCapabilityFloor|AttestedModelStampOf|ReStampRemovals)\(`)

	// perFile caches file contents and, per callee, how many call expressions of that exact
	// callee this file's positive assertion expects — a file with fewer occurrences than
	// floorCallSites declares means the call site MOVED or was REMOVED, which this guard
	// must also catch rather than passing vacuously.
	wantCount := map[string]int{}
	for _, s := range floorCallSites {
		wantCount[s.file+"|"+s.callee]++
	}

	seen := map[string]int{}
	for file, content := range readFloorConsumerFiles(t, floorCallSites) {
		locs := callRe.FindAllStringSubmatchIndex(content, -1)
		for _, loc := range locs {
			calleeStart, calleeEnd := loc[2], loc[3]
			callee := content[calleeStart:calleeEnd]
			seen[file+"|"+callee]++

			// Window from the call's opening paren to a fixed lookahead, generous enough
			// to cover every one of these calls' (possibly multi-line) argument lists as
			// formatted today, without needing a full paren-balancer for a handful of
			// known, simple call expressions.
			openParen := loc[1] // end of the whole match, i.e. index right after "("
			end := openParen + 300
			if end > len(content) {
				end = len(content)
			}
			window := content[openParen:end]

			if !regexp.MustCompile(`\bIsStampAuthorityLogin\b`).MatchString(window) {
				t.Errorf("%s: a %s(...) call does not pass deskkit.IsStampAuthorityLogin as its "+
					"actor predicate within %d bytes of the call — the #336 roster-configured "+
					"allowance would not apply to this caller", file, callee, end-openParen)
			}
			if regexp.MustCompile(`\bIsDispatcherLogin\b`).MatchString(window) {
				t.Errorf("%s: a %s(...) call passes deskkit.IsDispatcherLogin — the narrower, "+
					"bound-dispatcher-only predicate — where deskkit.IsStampAuthorityLogin is "+
					"required so the #336 roster-configured allowance applies uniformly", file, callee)
			}
		}
	}

	// Positive assertion: every declared call site was actually found the expected number
	// of times, so this guard is wired to the real call sites and not vacuously green
	// against a tree where one moved, was renamed, or was removed.
	for key, want := range wantCount {
		if seen[key] != want {
			t.Errorf("floor-consumer guard: expected %d call(s) matching %q, found %d — "+
				"a declared floor call site moved, was renamed, or was removed; update "+
				"floorCallSites in floorconsumers_test.go to match", want, key, seen[key])
		}
	}
}

// readFloorConsumerFiles reads each distinct file floorCallSites names, once, keyed by its
// declared path (relative to the module root, which is this test's own working directory).
func readFloorConsumerFiles(t *testing.T, sites []floorCallSite) map[string]string {
	t.Helper()
	out := map[string]string{}
	seenFiles := map[string]bool{}
	for _, s := range sites {
		if seenFiles[s.file] {
			continue
		}
		seenFiles[s.file] = true
		b, err := os.ReadFile(s.file)
		if err != nil {
			t.Fatalf("floor-consumer guard: could not read declared call-site file %q: %v", s.file, err)
		}
		out[s.file] = stripFullLineComments(string(b))
	}
	return out
}

// stripFullLineComments blanks out (rather than removes, to keep byte offsets meaningless
// but line count stable) every line whose first non-whitespace characters are "//" — a
// prose mention of a call site's name in a doc comment (this file's own package doc does
// exactly that) must never count as, or hide a mutation of, the real call expression.
func stripFullLineComments(content string) string {
	lines := strings.Split(content, "\n")
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "//") {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}
