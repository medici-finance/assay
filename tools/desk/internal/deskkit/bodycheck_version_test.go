package deskkit

import (
	"math/rand"
	"strings"
	"testing"
)

// Fixtures for the #1642 version-segment shape. Every identifier is SPLIT across
// concatenation for the reason recorded at scanSecret40: written contiguously, a shape the
// pre-fix scanner refuses would refuse the very diff that admits it. Secret-shaped material
// is never written out at all; tokenOf builds it from a fixed seed at run time.
var (
	// Admitted: a `V` plus a 1-2 digit group, between words or closing the run.
	identV1  = "Test" + "V1" + "DesksDecodes" + "AsContractType"  // 32, the field refusal
	identV2  = "TestRoutesEvery" + "V2" + "RequestToNewHandler"   // 36
	identV10 = "TestMigratesThe" + "V10" + "SchemaWithoutLoss"    // 35, two digits
	identEnd = "TestDecodesEveryContract" + "TypeAs" + "V2"       // 32, closes the run
	verSnake = "Test_" + "V1" + "_Desks_Decodes_As_Contract_Type" // `_` splits the run

	// Refused: one row per bound of the version form, each isolating that bound.
	verDigits = "TestRoutesEvery" + "V123" + "RequestToNewHandler"  // DIGITS: 3+ digits
	verDebris = "TestRoutesEvery" + "V2x" + "RequestToNewHandler"   // FORWARD: debris after
	verLowerW = "TestRoutesEvery" + "V2req" + "RequestToNewHandler" // FORWARD: lowercase after
	verLead   = "V1" + "DesksDecodesAsContract" + "TypeHereNow"     // BACKWARD: opens the run
	verAfterD = "TestRoutes2" + "V2" + "RequestToNewHandlerNow"     // BACKWARD: after a digit
	verTwo    = "TestRoutesV1" + "AndV2RequestsTo" + "HandlerNow"   // BUDGETED: two versions
	verAcr    = "TestRoutesCI" + "AndV2RequestsTo" + "HandlerNow"   // BUDGETED: acronym too
	verAcrDig = "TestRoutesEvery" + "HTTP2" + "RequestToNewHandler" // acronym+digit stays out
	verB62    = "Qx7pLk2wZt" + "V1" + "Nc4bYf6RhVs8Ju3XoAeG5"       // version inside base62
	verOther  = "TestUploadsEvery" + "S3" + "ObjectInOneRequest"    // `V` ONLY: other letter
	verCapsV  = "TestRoutesEvery" + "XV1" + "RequestToNewHandler"   // `V` ONLY: caps run ending in V
	verLoneV  = "TestRoutesEvery" + "V" + "RequestToNewHandler"     // DIGITS: none before a word
	verAcrEnd = "TestRoutesCI" + "RequestsToNewHandler" + "V2"      // BUDGETED: closing version
	verLoneA  = "TestRoutesEvery" + "V2" + "AWriteToNewHandler"     // FORWARD: lone A, then a word

	// A version unit spends the run's acronym budget, and a CLOSING acronym's budget-free
	// pass does not apply in a run that already carries one: a version and an acronym never
	// share one run, whichever closes it. A numeronym is a word, not an acronym, and is free.
	verEndAcr  = "TestRoutesEvery" + "V2" + "RequestToHandler" + "OK"          // closing acronym
	verEndPl   = "TestRoutesEvery" + "V2" + "RequestFromOpen" + "PRs"          // closing plural
	identV2K8s = "TestRoutesEvery" + "V2" + "RequestUnder" + "K8s" + "Cluster" // numeronym

	// The leading-acronym shape. It is refused, and stays refused here: admitting it is the
	// removal of the BACKWARD / AFTER-A-WORD bound that identLeadAcr and a committed
	// mutation entry pin, which is a control change this PR does not make.
	identLeadPS = "PS" + "NativeCommandUse" + "ErrorActionPreference" // 39
)

// tokenOf returns n characters drawn from alpha with a fixed seed: real-shaped secret
// material that never appears in the source as a literal.
func tokenOf(alpha string, n int, seed int64) string {
	rng := rand.New(rand.NewSource(seed))
	b := make([]byte, n)
	for i := range b {
		b[i] = alpha[rng.Intn(len(alpha))]
	}
	return string(b)
}

const (
	hexAlpha  = "0123456789abcdef"
	b62Alpha  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "0123456789"
	upperDigs = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "0123456789"
)

// secretShapes returns real-shaped tokens of length n: base64, hex, mixed-case base62, an
// upper+digit run, and a CamelCase identifier head carrying an entropy tail.
func secretShapes(n int, seed int64) map[string]string {
	head := "Test" + "V1" + "Desks"
	return map[string]string{
		"base64":      tokenOf(base64Alphabet, n, seed),
		"hex":         tokenOf(hexAlpha, n, seed+1),
		"base62":      tokenOf(b62Alpha, n, seed+2),
		"upper+digit": tokenOf(upperDigs, n, seed+3),
		"camel head":  head + tokenOf(b62Alpha, n-len(head), seed+4),
	}
}

// TestVersionSegmentIdents pins the admission: a `V` directly followed by one or two
// digits (`V1`, `V10`) is one word unit when a word precedes it and a word (or the end of
// the run) follows.
func TestVersionSegmentIdents(t *testing.T) {
	for _, run := range []string{identV1, identV2, identV10, identEnd} {
		if !isIdentifierLike(run) {
			t.Errorf("isIdentifierLike(%q) = false, want true", run)
		}
		if err := BodyCheck([]byte("the assertion is in " + run + " today")); err != nil {
			t.Errorf("BodyCheck rejected a body naming %q: %v", run, err)
		}
	}
}

// TestVersionSegmentBounds: each bound of the version form, isolated by one row.
func TestVersionSegmentBounds(t *testing.T) {
	for name, run := range map[string]string{
		"3+ digits": verDigits, "debris after": verDebris, "lowercase after": verLowerW, "opens the run": verLead,
		"after a digit": verAfterD, "two versions": verTwo, "acronym and version": verAcr,
		"acronym then digit": verAcrDig, "inside base62": verB62, "leading acronym": identLeadPS,
		"letter other than V": verOther, "caps run ending in V": verCapsV, "lone V before a word": verLoneV,
		"acronym then closing version": verAcrEnd, "version then lone A": verLoneA,
	} {
		if isIdentifierLike(run) {
			t.Errorf("%s: isIdentifierLike(%q) = true, want false", name, run)
		}
		if err := BodyCheck([]byte("value " + run + " here")); !IsRefused(err) {
			t.Errorf("%s: BodyCheck(%q) = %v, want Refused", name, run, err)
		}
	}
}

// TestVersionSegmentClosingAcr: a closing acronym (plain or plural) spends no budget only
// in a run with no version unit; after a version it refuses. A numeronym is a word and is
// admitted beside a version.
func TestVersionSegmentClosingAcr(t *testing.T) {
	for name, run := range map[string]string{
		"version then closing acronym": verEndAcr, "version then closing plural": verEndPl,
	} {
		if isIdentifierLike(run) {
			t.Errorf("%s: isIdentifierLike(%q) = true, want false", name, run)
		}
		if err := BodyCheck([]byte("value " + run + " here")); !IsRefused(err) {
			t.Errorf("%s: BodyCheck(%q) = %v, want Refused", name, run, err)
		}
	}
	if !isIdentifierLike(identV2K8s) {
		t.Errorf("isIdentifierLike(%q) = false, want true", identV2K8s)
	}
	if err := BodyCheck([]byte("the assertion is in " + identV2K8s + " today")); err != nil {
		t.Errorf("BodyCheck rejected a body naming %q: %v", identV2K8s, err)
	}
}

// TestPlantedTokenBeside plants a real-shaped token of the SAME length right beside each
// newly-admitted identifier — separated by a space, and glued into the same run — and
// requires the scan to refuse. Admitting the identifier must never admit its neighbour.
func TestPlantedTokenBeside(t *testing.T) {
	for i, ident := range []string{identV1, identV2, identV10, identEnd} {
		for shape, tok := range secretShapes(len(ident), int64(1642+10*i)) {
			for _, body := range []string{ident + " " + tok, tok + " " + ident, ident + tok} {
				if err := BodyCheck([]byte(body)); !IsRefused(err) {
					t.Errorf("%s token beside %q admitted: BodyCheck = %v", shape, ident, err)
				}
			}
		}
	}
}

// TestIdentShapeClass is the class guard: identifier shapes (digit segments, a leading
// acronym, ALL-CAPS words, snake case mixed with CamelCase) against secret shapes of the
// same lengths. A widening anywhere in the high-entropy arm — not only in the version
// form — that admits secret material turns a secret row red.
func TestIdentShapeClass(t *testing.T) {
	type row struct {
		shape, run string
		admit      bool
	}
	rows := []row{
		{"digit segment mid-run", identV1, true},
		{"digit segment, two digits", identV10, true},
		{"digit segment closing", identEnd, true},
		{"snake mixed with a digit segment", verSnake, true},
		{"snake ALL-CAPS", "PS_NATIVE_COMMAND_" + "USE_ERROR_ACTION_PREFERENCE", true},
		{"snake mixed CamelCase", "Test_" + "NativeCommand_Use" + "ErrorAction_Preference", true},
		{"leading acronym (bound held)", identLeadPS, false},
		{"ALL-CAPS words", "NATIVECOMMAND" + "USEERRORACTION" + "PREFERENCE", false},
		{"5-letter caps mid-run", identMidCaps5, false},
	}
	for _, n := range []int{32, 36, 39, 48} {
		for shape, tok := range secretShapes(n, int64(n)) {
			rows = append(rows, row{"secret " + shape, tok, false})
		}
	}
	for _, r := range rows {
		err := BodyCheck([]byte("see " + r.run + " now"))
		if r.admit && err != nil {
			t.Errorf("%s: BodyCheck(%q) = %v, want admitted", r.shape, r.run, err)
		}
		if !r.admit && !IsRefused(err) {
			t.Errorf("%s: BodyCheck(%q) = %v, want Refused", r.shape, r.run, err)
		}
		if strings.Contains(r.shape, "secret") && isIdentifierLike(r.run) {
			t.Errorf("%s: isIdentifierLike(%q) = true on secret material", r.shape, r.run)
		}
	}
}
