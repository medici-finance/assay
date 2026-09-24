package deskkit

import (
	"math/rand"
	"strings"
	"testing"
)

// Fixtures for #1642. Every identifier here is SPLIT across concatenation for the reason
// recorded at scanSecret40: written contiguously, the shapes the pre-fix scanner refuses
// would refuse the very PR that admits them.
var (
	// Admitted: the three field refusals, verbatim shapes.
	identK8s    = "TestEveryImageUnder" + "K8sIsDigestPinned"    // 36, numeronym K8s
	identPRs    = "TestRepresentingPRsBy" + "BriefKeepsEveryPR"  // 38, plural PRs + closing PR
	identExact  = "TestPhantomExactAuthoring" + "ShapeAdmitted"  // 38, already word-shaped
	identIDs    = "TestListsPullRequest" + "IDsAcrossEveryRepo"  // 38, plural IDs mid-run
	identI18n   = "TestRendersEveryPage" + "WithI18nFallback"    // 36, numeronym I18n
	argDigest   = "HEX=" + strings.Repeat("e5496277be5d09bc", 4) // 68, ARG default = sha256 hex
	argDigest40 = "SHA=" + strings.Repeat("5d529c27", 5)         // 44, key + 40-hex sha1

	// Refused: each new shape wearing real-secret material of a similar length.
	k8sDebris    = "Qx7pLk2wZtK8sNc4bYf6Rh" + "Vs8Ju3XoAeG5"          // numeronym inside base62
	prsDebris    = "Qx7pLk2wZtPRsNc4bYf6Rh" + "Vs8Ju3XoAeG5"          // plural acronym inside base62
	camelPrefix  = "TestEveryImageUnder" + "K8s" + "Qx7pLk2wZt9mNc4b" // CamelCase head, entropy tail
	camelPrefix2 = "TestRepresentingPRs" + "Qx7pLk2wZt9mNc4bYf6R"     // plural acronym, entropy tail
	pluralNoWord = "TestRepresentingPRs" + "sQxvbnmkpQxvbnm"          // `s` not followed by a word
	twoPlurals   = "TestListsIDsAndPRs" + "TogetherInOneNameNow"      // budget: two mid acronyms
	capsKeyAWS   = "KEY=" + awsExampleSecretKey                       // caps key, AWS secret value
	capsKeyB62   = "TOKEN=" + scanSecret40                            // caps key, 40 base62 value
	capsKeyLong  = "ABCDEFGHIJKLMNOPQ=" + "5d529c27e3b1a04f9c2d8e7b" + "6a1f0c3d4e5f6a7b"
	capsKeyMixed = "HEXa=" + strings.Repeat("e5496277be5d09bc", 4)
	glToken      = "glpat-" + "xq7Rk2PzLw9vNc4bYf6H"
)

// TestPluralAndNumeronymIdents pins the #1642 admissions: a short acronym carrying a plural
// `s` (`PRs`, `IDs`) and a closed list of numeronyms (`K8s`, `I18n`) no longer sink an
// otherwise word-shaped identifier, and one closing acronym no longer spends the budget.
func TestPluralAndNumeronymIdents(t *testing.T) {
	for _, run := range []string{identK8s, identPRs, identExact, identIDs, identI18n} {
		if !isIdentifierLike(run) {
			t.Errorf("isIdentifierLike(%q) = false, want true", run)
		}
		if err := BodyCheck([]byte("the assertion is in " + run + " today")); err != nil {
			t.Errorf("BodyCheck rejected a body naming %q: %v", run, err)
		}
	}
}

// TestCapsKeyAssignment pins the ALL-CAPS assignment key: a Dockerfile `ARG NAME_HEX=<sha>`
// line reaches the loop as `HEX=<sha>` (the `_` is outside the run class). The value must
// still clear the bar it would clear alone, and the key is bounded in length and shape.
func TestCapsKeyAssignment(t *testing.T) {
	for _, run := range []string{argDigest, argDigest40} {
		if !isAssignmentLike(run) {
			t.Errorf("isAssignmentLike(%q) = false, want true", run)
		}
		line := "ARG BASE_DIGEST_" + run + "\n"
		if err := BodyCheck([]byte(line)); err != nil {
			t.Errorf("BodyCheck rejected %q: %v", line, err)
		}
	}
	for _, run := range []string{capsKeyAWS, capsKeyB62, capsKeyLong, capsKeyMixed} {
		if isAssignmentLike(run) {
			t.Errorf("isAssignmentLike(%q) = true, want false", run)
		}
		if err := BodyCheck([]byte(run)); !IsRefused(err) {
			t.Errorf("BodyCheck(%q) = %v, want Refused", run, err)
		}
	}
}

// TestSubstitutionLines: `${NAME}` / `$NAME` never form a run of their own — `$`, `{`, `}`
// and `_` are outside the run class — so the Dockerfile FROM line is admitted with no rule.
// A secret pasted as a substitution's DEFAULT still refuses.
func TestSubstitutionLines(t *testing.T) {
	for _, line := range []string{
		"FROM ghcr.io/actions/actions-runner:${BASE_TAG}@sha256:${BASE_DIGEST_HEX}",
		"RUN echo $BASE_DIGEST_HEX ${IMAGE_REPOSITORY_NAME} $HOME",
	} {
		if err := BodyCheck([]byte(line)); err != nil {
			t.Errorf("BodyCheck rejected %q: %v", line, err)
		}
	}
	if err := BodyCheck([]byte("echo ${TOKEN:-" + scanSecret40 + "}")); !IsRefused(err) {
		t.Errorf("a secret as a substitution default was admitted: %v", err)
	}
}

// TestNewShapesStillRefuse: every #1642 shape wearing real-secret material is refused.
func TestNewShapesStillRefuse(t *testing.T) {
	for _, run := range []string{k8sDebris, prsDebris, camelPrefix, camelPrefix2, pluralNoWord, twoPlurals} {
		if isIdentifierLike(run) {
			t.Errorf("isIdentifierLike(%q) = true, want false", run)
		}
		if err := BodyCheck([]byte("value " + run + " here")); !IsRefused(err) {
			t.Errorf("BodyCheck(%q) = %v, want Refused", run, err)
		}
	}
}

// TestTokenShapesStillRefused: real-shaped tokens and random 36-64 char runs refuse.
func TestTokenShapesStillRefused(t *testing.T) {
	cases := map[string]string{
		"github ghp_":         scanGHToken,
		"github fine-grained": "github" + "_pat_11ABCDEFG0123456789_abcdefghijklmnop",
		"gitlab glpat-":       glToken,
		"aws key id":          scanAWSKeyID,
		"aws secret key":      awsExampleSecretKey,
	}
	const b62 = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "0123456789"
	rng := rand.New(rand.NewSource(1642))
	for _, n := range []int{36, 48, 64} {
		for name, alpha := range map[string]string{"base62": b62, "base64": base64Alphabet} {
			b := make([]byte, n)
			for i := range b {
				b[i] = alpha[rng.Intn(len(alpha))]
			}
			cases[name+" random "+string(rune('0'+n/10))+string(rune('0'+n%10))] = string(b)
		}
	}
	for name, v := range cases {
		if err := BodyCheck([]byte("value " + v + " here")); !IsRefused(err) {
			t.Errorf("%s: BodyCheck(%q) = %v, want Refused", name, v, err)
		}
	}
}

// TestNewRulesCostOnRandom measures the #1642 widening on random material at the lengths
// the field refusals sat at (36/38), where TestShortAcronymCostsAlmostNothing measures 32.
func TestNewRulesCostOnRandom(t *testing.T) {
	const trials = 500000
	rng := rand.New(rand.NewSource(20260924))
	for _, n := range []int{runThreshold, 36, 38} {
		got := 0
		for i := 0; i < trials; i++ {
			b := make([]byte, n)
			for j := range b {
				b[j] = base64Alphabet[rng.Intn(len(base64Alphabet))]
			}
			if isIdentifierLike(string(b)) {
				got++
			}
		}
		t.Logf("%d-char random base64 runs admitted: %d of %d", n, got, trials)
		if got > 5 {
			t.Errorf("%d-char random runs admitted %d of %d (ceiling 5)", n, got, trials)
		}
	}
}
