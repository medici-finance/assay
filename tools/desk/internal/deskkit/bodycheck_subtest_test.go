package deskkit

import (
	"math/rand"
	"strings"
	"testing"
)

// Fixtures for the Go-subtest, diffstat-bar and OAuth shapes. Every identifier and token
// here is SPLIT across concatenation for the reason recorded at scanSecret40: written
// contiguously, the shapes the pre-fix scanner refuses would refuse the very PR that
// admits them.
var (
	// Admitted: Go test names with a subtest path, as `go test -run` and Verify Expect
	// cells quote them.
	subGo      = "TestSupersedeMarker" + "CitationRejects/go"           // 37, two-letter subtest
	subNo      = "TestSupersedeMarker" + "Authorship/no"                // 32
	subV2      = "TestSupersedeMarker" + "CitationRejects/v2"           // 37, version-ish subtest
	subK8s     = "TestK8sSecretDocScope" + "MultipleDocs/ok"            // 36, numeronym head
	subLong    = "TestLongIdentifierThatGoes" + "OnAndOnForeverMore/no" // 47, head past 48-char path cap
	subNested  = "BenchmarkSupersede" + "MarkerCitation/ok/go"          // 38, two short pieces = 4 chars
	subWords   = "TestSupersedeMarker" + "Citation/rejects/stale"       // 41, word subtests
	subTail16  = "TestSupersedeMarker" + "Citation/rejects/outdated"    // 44, tail at the 16-char cap
	subCommand = "go test -run 'TestSupersedeMarker" + "HeldSideRejects/no' ./internal/"

	// Refused: credential-shaped material of the same length and charset.
	subRandom     = "Qx7pLk2wZt9mNc4bYf6R" + "hVs8Ju3XoAe/G5nTq"       // 37, random base64 with a slash
	subRandomHead = "Test" + "Qx7pLk2wZt9mNc4bYf6Rh" + "Vs8J/no"       // 32+, Test prefix, random head
	subTokenTail  = "TestSupersedeMarker/" + scanSecret40              // token glued behind a test name
	subShortTail  = "TestSupersedeMarker" + "Rejects/ab12/cd34"        // 8 opaque chars, budget is 4
	subPadTail    = "TestSupersedeMarker" + "CitationRejects/a+b="     // '+'/'=' in a subtest
	subLowerHead  = "Testsupersedemarker" + "citationrejects/go"       // not a Go test name
	subEmptySeg   = "TestSupersedeMarker" + "CitationRejects//go"      // empty segment
	subLongTail   = "TestCIRequired" + "Matches/rejectsoutdatedmarker" // acronym head, 21-char tail past the 16 cap

	// Diffstat bar: admitted, and its controls.
	statBar      = "docs/loom-design.md | 108 " + strings.Repeat("+", 52)
	statBarMixed = " a.go | 80 " + strings.Repeat("+", 40) + strings.Repeat("-", 30)
	statPlusTok  = strings.Repeat("+", 20) + scanSecret40                            // bar glued to a token
	statSameLen  = "Qx7pLk2wZt+9mNc4bYf6R" + "hVs8Ju3+XoAeG5nTq" + "Zm4Lp+0Wd8Ry3Kc" // 52, mixed
	statLetter   = strings.Repeat("A", 52)                                           // repeated letter stays refused

	// OAuth: admitted, and its controls.
	oauthIdent   = "TestOAuthToken" + "ExchangeRejectsStale"  // 34
	oauthIdent2  = "TestOAuthCallback" + "RejectsStaleState"  // 34
	oauthLoneQ   = "TestQAuthToken" + "ExchangeRejectsStale"  // lone capital, not the closed word
	oauthDebris  = "Qx7pLk2wZtOAuth" + "Nc4bYf6RhVs8Ju3XoAe"  // the word inside base62
	oauthGlued   = "TestOAuthz" + "TokenExchangeRejectsStale" // OAuth not ending at a capital
	goSumDeleted = "-golang.org/x/text v0.3.7 h1:" + "olpwvP2KacW1ZWvsR7uQ" + "hoyTYvKAupfQrRGBFM352Gk="
)

// regression: #4076
func TestSubtestNamesAdmitted(t *testing.T) {
	for _, s := range []string{subGo, subNo, subV2, subK8s, subLong, subNested, subWords, subTail16, subCommand} {
		if err := BodyCheck([]byte("Expect: " + s + " passes\n")); err != nil {
			t.Errorf("Go subtest name %q refused: %v", s, err)
		}
	}
}

// regression: #4076
func TestSubtestNameControls(t *testing.T) {
	for _, s := range []string{subRandom, subRandomHead, subTokenTail, subShortTail, subPadTail, subLowerHead, subEmptySeg, subLongTail} {
		if err := BodyCheck([]byte("Expect: " + s + "\n")); !IsRefused(err) {
			t.Errorf("credential-shaped run %q admitted (err=%v)", s, err)
		}
	}
}

// TestSubtestRuleCostOnRandom pins what isGoSubtestName costs on token material, the way
// TestNewRulesCostOnRandom does for the identifier rules. On uniformly random base64 runs
// it must admit nothing. The adversarial case — a random run that already opens with
// `TestX` — is measured as the MARGINAL cost: runs the new rule admits that the existing
// identifier and path rules did not. The existing rules' own admissions on that shape are
// logged beside it for scale; the marginal count is held to the same ceiling of 5 per
// 500,000 that TestNewRulesCostOnRandom uses.
func TestSubtestRuleCostOnRandom(t *testing.T) {
	const trials = 500000
	rng := rand.New(rand.NewSource(20261021))
	rnd := func(n int) string {
		b := make([]byte, n)
		for j := range b {
			b[j] = base64Alphabet[rng.Intn(len(base64Alphabet))]
		}
		return string(b)
	}
	for _, n := range []int{runThreshold, 37, 40} {
		plain, marginal, baseline := 0, 0, 0
		for i := 0; i < trials; i++ {
			if isGoSubtestName(rnd(n)) {
				plain++
			}
			s := "TestX" + rnd(n-5)
			old := isIdentifierLike(s) || isPathLike(s)
			if old {
				baseline++
			}
			if !old && isGoSubtestName(s) {
				marginal++
			}
		}
		t.Logf("%d-char runs: random admitted %d; TestX-prefixed marginal %d (existing rules %d) of %d",
			n, plain, marginal, baseline, trials)
		if plain > 0 {
			t.Errorf("%d-char random runs admitted: %d (ceiling 0)", n, plain)
		}
		if marginal > 5 {
			t.Errorf("%d-char TestX-prefixed runs newly admitted: %d of %d (ceiling 5)", n, marginal, trials)
		}
	}
}

// TestSubtestTailCostOnRandom pins the tail half of isGoSubtestName on token material: a
// VALID, acronym-bearing test-name head followed by a slash and a random 32-char tail
// drawn from the lowercase alphabet, and from the lowercase-alphanumeric alphabet. The
// head is exempt on its own, so this measures exactly what the tail rule admits; every
// sample must still refuse through the full scanner (BodyCheck), as on main.
func TestSubtestTailCostOnRandom(t *testing.T) {
	const (
		trials = 3000
		tail   = 32
		lower  = "abcdefghijklmnopqrstuvwxyz"
	)
	head := "TestCIRequired" + "Matches" // acronym-bearing, exempt as a bare identifier
	if err := BodyCheck([]byte("Expect: " + head + "\n")); err != nil {
		t.Fatalf("control head %q refused on its own: %v", head, err)
	}
	for _, alpha := range []struct{ name, chars string }{
		{"lowercase", lower},
		{"lowercase-alnum", lower + "0123456789"},
	} {
		rng := rand.New(rand.NewSource(20261022))
		admitted := 0
		for i := 0; i < trials; i++ {
			b := make([]byte, tail)
			for j := range b {
				b[j] = alpha.chars[rng.Intn(len(alpha.chars))]
			}
			if err := BodyCheck([]byte("Expect: " + head + "/" + string(b) + "\n")); !IsRefused(err) {
				admitted++
			}
		}
		t.Logf("%s %d-char tails behind %q: admitted %d of %d", alpha.name, tail, head, admitted, trials)
		if admitted > 0 {
			t.Errorf("%s %d-char random tails admitted behind a valid test name: %d of %d (ceiling 0)",
				alpha.name, tail, admitted, trials)
		}
	}
}

// regression: #4076
func TestDiffstatBarAdmitted(t *testing.T) {
	for _, s := range []string{statBar, statBarMixed} {
		if err := BodyCheck([]byte(s + "\n")); err != nil {
			t.Errorf("diffstat line %q refused: %v", s, err)
		}
	}
}

// regression: #4076
func TestDiffstatBarControls(t *testing.T) {
	for _, s := range []string{statPlusTok, statSameLen, statLetter} {
		if err := BodyCheck([]byte(s + "\n")); !IsRefused(err) {
			t.Errorf("run %q admitted as a diffstat bar (err=%v)", s, err)
		}
	}
}

// regression: #4076
func TestOAuthWordAdmitted(t *testing.T) {
	for _, s := range []string{oauthIdent, oauthIdent2} {
		if err := BodyCheck([]byte("go test -run " + s + "\n")); err != nil {
			t.Errorf("identifier %q refused: %v", s, err)
		}
	}
}

// regression: #4076
func TestOAuthWordControls(t *testing.T) {
	for _, s := range []string{oauthLoneQ, oauthDebris, oauthGlued} {
		if err := BodyCheck([]byte(s + "\n")); !IsRefused(err) {
			t.Errorf("run %q admitted (err=%v)", s, err)
		}
	}
}

// TestDeletedGoSumLinePasses pins that a go.sum line on a DELETED diff line — `-` in
// front of the module path — is recognised by the go.sum h1: structured exemption exactly
// like an added one.
func TestDeletedGoSumLinePasses(t *testing.T) {
	if err := BodyCheck([]byte(goSumDeleted + "\n")); err != nil {
		t.Errorf("deleted go.sum line refused: %v", err)
	}
}
