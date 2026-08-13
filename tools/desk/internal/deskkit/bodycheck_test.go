package deskkit

import (
	"math/rand"
	"strings"
	"testing"
)

// awsExampleSecretKey is AWS's own published EXAMPLE secret access key — a deliberate
// non-secret used throughout their documentation. It is the canonical shape of a real
// AWS secret access key: 40 base64 characters that routinely contain `/`. Split across
// concatenation so a naive grep for a leaked key does not match this file.
const awsExampleSecretKey = "wJalrXUtnFEMI" + "/K7MDENG/" + "bPxRfiCYEXAMPLEKEY"

// Fixtures for the #663 / #775 / #781 rows, assembled from SPLIT
// fragments for the reason recorded at scanSecret40 below — with one extra twist worth
// spelling out, because it is the sharpest possible demonstration of the bug being fixed.
//
// deskpr secret-scans the branch diff before it pushes. These fixtures are precisely the
// shapes the PRE-fix scanner refuses, so written contiguously they would refuse the very
// PR that fixes them: the change could not be pushed by the tool it repairs. That is not
// hypothetical — it is the same wall #663 records four PRs hitting, one of
// them noting that its own prior head "could never have been deskpr-pushed under the
// current scanner". Splitting the literal is the house workaround already used for the
// credential vectors below; it changes no runtime value, so both directions are still
// exercised in full against the exact strings.
var (
	// #781 — CamelCase Go test names that turn on a single two-letter
	// English word. Each is over the 32-char run threshold and refused pre-fix.
	identVs   = "TestRouterPrefersOn" + "DemandVsScheduled"
	identAn   = "TestParserHandles" + "AnEmptyInputStream"
	identIsOn = "TestHonoredFilterIs" + "OnByDefaultForEveryRoute"
	identNoOf = "TestReportsNo" + "FindingsOfTheTripwireClass"
	// Capital-plus-one-lowercase pairs that are NOT English words — the shapes a blanket
	// relaxation would admit and the curated list must keep refusing.
	nonWordPairs = "QxPxXoZtNcYf" + "RhVsJuAeWnDz" + "KtEvFiYf"
	identAllCaps = "TestHandlesHTTPSPROXY" + "UpstreamHealthCheck"

	// #775 — the Verify/Evidence `key=path` shell-assignment idiom.
	assignRun      = "f=docs/streams/" + "distribution/version"
	assignEvidence = "f=docs/streams/" + "distribution/version-scheme.md"
	assignTwoChar  = "fp=tools/desk/" + "internal/deskkit/bodycheck"
	assignWordKey  = "manifest=docs/" + "publication/manifest/classification"
	assignIdentVal = "t=TestHonoredFilter" + "LeavesTripwireSilent"
	// Guard-strength: the key half is valid in each, so only the VALUE can be refusing.
	assignNoKey     = "=docs/streams/" + "distribution/version"
	assignOpaqueKey = "Qx7pLk2wZt9mNc4bYf6Rh" + assignNoKey
	assignPlusVal   = "f=config/Zm9v" + "+YmFy+YmF6+cXV4" + "+1234567890abcdef"
	base64Unpadded  = "ZGVhZGJlZWZkZWFk" + "YmVlZmRlYWRiZWVm" + "ZGVhZGJlZWY"
	base64Padded    = "ZGVhZGJlZWZkZWFk" + "YmVlZmRlYWRiZWVm" + "=="
	// The randomness alphabet for TestTwoLetterWordsCostAlmostNothing — 64 base64 chars
	// contiguous is itself a refused run, so it is split like everything else here.
	base64Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "0123456789+/"
)

// TestBodyCheck covers BOTH directions: git SHAs and clean
// prose pass; every credential-shaped pattern refuses with ExitRefused.
func TestBodyCheck(t *testing.T) {
	const sha1 = "5d529c27e3b1a04f9c2d8e7b6a1f0c3d4e5f6a7b"                           // 40 lc hex
	const sha256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // 64 lc hex

	cases := []struct {
		name       string
		body       string
		wantRefuse bool
	}{
		// --- pass ---
		{"empty", "", false},
		{"plain prose", "LGTM — approving this PR, the logic reads correctly.", false},
		{"40-char git sha alone", sha1, false},
		{"64-char git sha alone", sha256, false},
		{"sha inline in a verdict", "verified at " + sha1 + " — APPROVE", false},
		{"both shas inline", "base " + sha1 + " head " + sha256, false},
		// #1052 path exemptions — legitimate deep paths with all short segments.
		{"issue #1052 exact repro path", "the skill text contained the absolute path `/Users/operator/work/example/slides` — a 35-char run", false},
		{"other recognised path root: home", "config lives at /home/user/documents/notes/reference on that box", false},
		{"other recognised path root: private", "scratch dir was /private/var/folders/zz/abcdefghijk this run", false},
		{"other recognised path root: opt", "binary at /opt/homebrew/opt/python311/bin/python3 on the runner", false},
		{"legitimate deep path under tmp", "cache dir /tmp/build/go/pkg/mod/cache/download/sumdb", false},
		// #209 / #1255 — repo-relative `file:line` references. This is
		// the MANDATED finding format for desk review bodies, so refusing it bricked
		// every verdict dense enough to cite its evidence (the original report's
		// "38-char path run"). The 37-char run below is
		// `tools/desk/internal/deskkit/bodycheck`; the `.go:45` tail is outside the
		// base64 charset so it does not extend the run.
		{"repo-relative file:line, the #209 repro", "the check lives at tools/desk/internal/deskkit/bodycheck.go:45", false},
		{"repo-relative file:line, no backticks", "blocker at services/app-service/internal/api/handlers/reviewwrite.go:212", false},
		{"deep frontend component path", "frontend/src/components/dashboard/PositionSummaryPanel.tsx:88", false},
		// CamelCase template paths — the shape that makes case-transition and
		// mean-segment-length heuristics useless as discriminators (see isPathLike):
		// this and an AWS secret access key are identical on both measures.
		{"camelcase template path", "see src/Example/Settlement.go:120 for SettlementView", false},
		{"a review body dense with findings", "## Review\n\n- `tools/desk/internal/deskkit/ratelimit.go:26` counts all attempts\n- `tools/desk/internal/deskkit/bodycheck.go:59` refuses the run\n- `frontend/src/lib/updatesCursor.ts:41` cursor handling\n\nVerdict: request-changes", false},
		// Go module paths. #209 records these as a SECOND false-positive class that a
		// `file:line`-only narrowing would NOT have fixed: any review of Go code carries
		// import paths, and `go test` output — which the methodology mandates pasting as
		// Evidence — is nothing but module paths.
		{"go module path, this repo", "ok github.com/medici-finance/assay/tools/desk/internal/deskkit 2.531s", false},
		{"go module path, nested cmd package", "ok github.com/medici/desk/cmd/deskpost/internal/bodycheck 0.378s", false},
		{"deep github blob url path run", "https://github.com/medici/desk/blob/main/tools/desk/internal/deskkit/bodycheck.go", false},
		// Shapes with EMPTY slash-separated segments: a URL's `//` and a directory
		// reference written with a trailing slash. A draft of the #1261 fix rejected
		// empty segments and re-created the bug on both; they carry no material.
		{"url run carrying a leading double slash", "read via https://localhost/api/v2/state/activecontracts on the box", false},
		{"directory path with a trailing slash", "the directory services/app-service/internal/api/handlers/ holds it", false},
		// Numeric path components: `2026`, `07`, `30` and the `v2` of an API path are not
		// word-shaped, but they are far too short to matter and only spend opaque budget.
		{"dated report path", "report at docs/reports/2026/07/30/factory-floor-summary.md:14", false},
		// #253 — long CamelCase Go test identifiers, bare (no `/` to
		// trigger isPathLike). #1588's review post refused with exit 5 on exactly
		// this shape: a 39/33-char identifier-only run naming the tests under review.
		{"issue #253 repro: 39-char Go test identifier", "`ZeroContractTemplateDecodesZeroElements` now covers the empty-elements branch", false},
		{"issue #253 repro: 33-char Go test identifier", "see HonoredFilterLeavesTripwireSilent for the tripwire assertion", false},
		{"Test-prefixed identifier over the run threshold", "TestZeroContractTemplateDecodesZeroElements passed locally", false},
		{"review body naming several long test idents", "## Review\n\n- TestReducesQuorumCommitmentUsingTripwireEvent covers the happy path\n- HonoredFilterLeavesTripwireSilent covers the silence case\n\nVerdict: approve", false},
		// #663 exact repro. A 41-char CamelCase Go test name — the PR's own
		// shipped identifier, which could never be reworded to dodge the scan — blocked
		// #332 outright. It decomposes into eight word units
		// (Test/Assay/Lint/Single/Writer/Guard/Fails/Closed) and carries no `/`, so the
		// bare-identifier exemption is the only thing that can clear it.
		{"issue #663 repro: 41-char CamelCase Go test name", "`TestAssayLintSingleWriterGuardFailsClosed` is the PR's own shipped test identifier", false},
		// #781 class: an ordinary two-letter English word inside an otherwise
		// unremarkable CamelCase test name used to read as base64 debris, because the
		// decomposition demanded two lowercase letters behind every capital. Twelve test
		// names in one new package tripped this at once, and `Is` alone blocks 124 real
		// identifiers in this repo. Each row below is over the 32-char run threshold and
		// turns on exactly one such word.
		{"issue #781 repro: two-letter word Vs inside a test name", identVs + " asserts the precedence", false},
		{"issue #781 repro: two-letter word An inside a test name", identAn + " covers the empty case", false},
		{"issue #781 repro: two-letter words Is and On together", "see " + identIsOn + " for the default", false},
		{"issue #781 repro: two-letter words No and Of", identNoOf + " stays silent", false},
		{"two-letter word inside a path segment, not just a bare identifier", "handler at tools/desk/internal/IsOnByDefault/router.go:31", false},
		// #775 exact repro: the `key=path` shell-assignment idiom used by
		// every Verify/Evidence row that records the command it actually ran. The run is
		// `f=docs/streams/…/version` — 35 chars, and the `=` disqualified it from the path
		// exemption. This row could not be reworded to dodge the scanner without
		// desyncing a recorded audit trail from what was actually executed.
		{"issue #775 repro: f=<path> Evidence shell assignment", assignEvidence + "; test -f \"$f\" && echo present", false},
		{"key=path assignment with a word-shaped key", assignWordKey + ".yaml", false},
		{"key=identifier assignment", assignIdentVal + "; go test -run \"$t\"", false},
		{"key=<git sha> assignment in an Evidence row", "base=" + sha1 + " head=" + sha256, false},
		// #781 — a console-banner separator line. Its 34+ run of '='
		// clears the base64ish threshold; deskpr scans deleted diff lines, so a banner already
		// on origin cannot be edited out. An all-'=' run is padding with no payload.
		{"#781: console-banner separator line (all '=')", "-\techo \"" + strings.Repeat("=", 50) + "\"", false},
		// Bare-word "sops" false positives (the fixed over-match). None of these carries a
		// secret: a prose mention, a config filename, a key-name reference. The old
		// strings.Contains(s, "sops") refused all four; the sops-DOCUMENT signature
		// (line-anchored `sops:` key AND a sops-metadata field) does not.
		{"prose mention of sops", "we encrypt secrets with sops", false},
		{"sops config filename", ".sops.yaml", false},
		{"sops key-name reference", "sops-gpg", false},
		{"platform-build runner brief phrase", "both sops-gpg (KMS)", false},
		{"sops: prose line without a metadata field", "sops: encrypted at rest, see the runbook", false},
		// --- refuse ---
		{"github classic token", "token ghp" + "_0123456789ABCDEFabcdef0123456789ABCD", true},
		{"github fine-grained pat", "github" + "_pat_11ABCDE0123456789_abcDEF0123456789abcDEF", true},
		{"github server token ghs_", "ghs" + "_abcdEFGH1234abcdEFGH1234abcdEFGH1234", true},
		{"github oauth token gho_", "gho" + "_abcdEFGH1234abcdEFGH1234abcdEFGH1234", true},
		{"aws access key id", "key AKIA" + "IOSFODNN7EXAMPLE here", true},
		{"pem header", "-----BEGIN RSA " + "PRIVATE KEY-----\nMIIE...", true},
		{"jwt shape", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJ", true},
		{"sops marker", "sops:\n  kms: []", true},
		// A realistic sops document footer: the `sops:` mapping key, a `mac:` metadata
		// field, and an ENC[AES256_GCM,data:…] encrypted value. Refused both by the ENC
		// marker and by the sops-document signature.
		{"realistic sops footer", "sops:\n    mac: ENC[AES256_GCM,data:Zm9vYmFy,iv:abc,tag:def,type:str]\n    version: 3.7.3", true},
		{"enc marker", "value: ENC[AES256_GCM,data:Zm9v,iv:bar,tag:baz,type:str]", true},
		{"64-char uppercase hex is not a git sha", strings.Repeat("AB", 32), true},
		{"32-char lowercase hex is too short to be an exempt sha", strings.Repeat("ab", 16), true},
		{"long base64 blob", "ZGVhZGJlZWZkZWFkYmVlZmRlYWRiZWVmZGVhZGJlZWZkZWFkYmVlZg==", true},
		// Guard-strength: a realistic secret must still be refused even when
		// carried as a path segment of a deep path. The segment gate (isPathLike, gate 2)
		// refuses the exemption when any slash-separated segment is >=32 chars or contains
		// += (the base64 padding/special chars).
		{"40-char secret as last segment under deep /Users/", "/Users/a/b/c/" + "Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz", true},
		{"40-char secret as last segment under deep /tmp/", "/tmp/a/b/c/" + "Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz", true},
		{"40-char secret as last segment of realistic deep prefix", "/Users/operator/work/example/" + "Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz", true},
		{"deep path with = padded base64 tail", "/Users/a/b/c/" + "ZGVhZGJlZWZkZWFkYmVlZmRlYWRiZWVm==", true},
		{"deep path with + segments (base64)", "/Users/a/b/c/" + "Zm9v+YmFy+YmF6+cXV4+1234567890abcd", true},
		{"200-char single segment after deep path prefix", "/Users/a/b/c/" + strings.Repeat("x", 200), true},
		// Shallow /tmp/ + secret: the segment is >=32 chars, so the old depth-gate
		// failure mode is still covered (and still refused by the segment gate).
		{"40-char secret-like run embedded after a path-like prefix is still refused", "/tmp/" + "Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz", true},
		// #209 guard-strength. Widening the exemption from "absolute paths
		// under five mac-home roots" to "anything path-SHAPED" must not open a laundering
		// route. Every row below was demonstrated PASSING against #1261's
		// segment-length-only rule by that PR's reviewer, and must refuse here.
		{"40-char secret as a segment of a REPO-RELATIVE path", "tools/desk/" + "Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz", true},
		{"40-char secret as a bare relative two-segment path", "a/" + "Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz", true},
		{"unpadded 40-char secret under a k8s manifest path", "k8s/dev/ledger/" + "R2hpJ0kZ7vQx3TmLp9WdYc1BnEa6UfSg4XoIrKtZ", true},
		{"relative path whose tail is padded base64", "config/secrets/ZGVhZGJlZWZkZWFkYmVlZmRlYWRiZWVm==", true},
		{"relative path whose segments carry base64 + chars", "config/Zm9v+YmFy+YmF6+cXV4+1234567890abcdef", true},
		{"64-char base64 with slashes placed to keep every span short", "Kj8mQ2vRt/Lz3wXy7Nb5cHj/1Ae4Gf6Sd0Uk/Pq7Rm2Xn9Tv/Bw3CyZl6Jh8Dr2", true},
		{"secret split across two segments of a real repo path", "tools/desk/" + "Qx7pLk2wZt9mNc4bYf6RhVs8/Ju3XoAeG5idWn1DzPqLmXvBn", true},
		// The AWS SECRET ACCESS KEY, BARE. This is the row #1261's reviewer asked for
		// by name: that PR's table wrote it as `AWS_SECRET_ACCESS_KEY=<key>` and passed —
		// but only because the `=` of the SHELL ASSIGNMENT landed inside the run and
		// tripped the base64-padding rule. Delete the two-character `Y=` prefix and the
		// credential itself sailed through. The value below is AWS's own published
		// example key (a deliberate non-secret): 40 base64 characters containing two `/`,
		// every segment under 32 chars, no `+` or `=`. It is the single clearest case for
		// why a length-only segment gate is not enough, and it must refuse WITHOUT help
		// from any surrounding punctuation.
		{"aws secret access key, BARE (no assignment operator)", awsExampleSecretKey, true},
		{"aws secret access key on a yaml credential line", "aws_secret_access_key: " + awsExampleSecretKey, true},
		{"aws secret access key as a shell assignment", "AWS_SECRET_ACCESS_KEY=" + awsExampleSecretKey, true},
		// #781 guard: the all-'=' banner exemption must not be read as "runs
		// containing '='". A VAR=secret assignment carries the secret's own base64 AFTER the
		// '=', so its run is not all-'=' and must still refuse. (No '/' here, so isPathLike
		// was never a candidate; the run fails isAllEquals and isIdentifierLike alike.)
		{"#781 guard: TOKEN=<secret> is not an all-'=' run and must still refuse", "TOKEN=" + "wJalrXUtnFEMIK7MDENGbPxRfiCYEXAMPLEKEY", true},
		// A Slack incoming-webhook URL: the token tail is 24 opaque uppercase chars and
		// the two channel ids are opaque too, so the run is refused on opaque budget.
		{"slack incoming-webhook url with its token", "posted to https://hooks" + ".slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX", true},
		{"pem private-key body line", "MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQ", true},
		{"bare 44-char base64 with no slashes", "Zm9vYmFyYmF6cXV4MTIzNDU2Nzg5MGFiY2RlZmdoaWpr", true},
		// #253 guard-strength: the new bare-identifier exemption must not
		// widen to a real secret of comparable length just because it also has no `/`.
		// This is the same 40-char mixed-case token used above under path prefixes,
		// here BARE — it fails looksLikeWords on its very first character run (`Q`
		// followed by the single lowercase `x`, a lone-capital shape), so
		// isIdentifierLike does not exempt it and it must still refuse.
		{"bare 40-char secret with no path, no padding — must still refuse", "Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz", true},
		// #582 review guard-strength: the two-word floor alone stops only
		// an UNBROKEN lowercase run. Realistic lowercase token material carries short
		// digit groups, and every digit group ends one letter-word unit and starts
		// another, so these decompose into 2+ word units and were admitted by the
		// floor — both rows below were demonstrated ADMITTED through this very
		// BodyCheck entry point on the pre-fix head. The capital-led-word requirement
		// keeps them refused; under a too-broad rule (drop that requirement) both go
		// red. The "32-char lowercase hex is too short to be an exempt sha" case above
		// could not detect this class: its fixture is one repeated letter pair — an
		// unbroken span, ONE word — while real hex always carries digits. Literals are
		// split across concatenation so no contiguous 32-char token sits on an added
		// line of the diff (deskpr secret-scans the branch diff before it pushes).
		{"bare 32-char lowercase hex with digit groups, md5-shaped — must still refuse", "dcef1701acc78ada" + "90fcefb638adeabf", true},
		{"32-char lowercase base36 token carried in prose — must still refuse", "rolled token wdehl5mcgoou" + "6fuo4ihkpokq" + "3skqeost" + " in the deploy log", true},
		// #775 guard-strength. The assignment exemption is a prefix STRIP: the
		// value must clear isPathLike/isIdentifierLike on its own. The sharpest row is an
		// AWS secret access key wearing the very idiom the exemption was added for, with a
		// one-character key that passes the key half cleanly — so the ONLY thing refusing
		// this is the value being judged exactly as it would be standing bare. This row
		// exists because #1261's table once claimed a shell-assigned AWS key was
		// "still caught" when in truth only the stray `=` was doing the catching.
		{"aws secret access key behind a one-char shell key — must still refuse", "f=" + awsExampleSecretKey, true},
		{"40-char slash-free secret behind a shell key — must still refuse", "t=" + scanSecret40, true},
		// Base64 PADDING must not be launderable as an assignment: the value half may not
		// carry its own `=`, so a padded blob is refused however it is prefixed.
		{"padded base64 blob behind a shell key — must still refuse", "data=" + base64Padded, true},
		{"nested a=b=<secret> assignment — must still refuse", "a=b=" + scanSecret40, true},
		// #781 guard-strength for the two-letter-word relaxation. Each of these
		// carries a capital-plus-one-lowercase pair that is NOT an English word — the exact
		// shapes a blanket "capital plus one lowercase" relaxation would have admitted, and
		// the exact debris (Ym, Jl, Vh, Zm, Qx, Px) that shows up in real base64.
		{"base64 blob whose pairs are Zm/Jl/Vh, not words — must still refuse", "Zm9vYmFyYmF6" + "JlcXV4cXV1eA" + "Vhc2RmZ2hqa2w", true},
		{"capital-plus-one-lowercase pairs that are not English words", nonWordPairs, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := BodyCheck([]byte(c.body))
			if c.wantRefuse {
				if !IsRefused(err) {
					t.Fatalf("BodyCheck(%q) = %v, want Refused (exit 5)", c.body, err)
				}
				if ExitCodeOf(err) != ExitRefused {
					t.Fatalf("ExitCodeOf = %d, want %d", ExitCodeOf(err), ExitRefused)
				}
			} else if err != nil {
				t.Fatalf("BodyCheck(%q) = %v, want nil (clean body)", c.body, err)
			}
		})
	}
}

// TestIsPathLikeIsNotWiderThanTheAnchoredRule probes isPathLike directly, because
// BodyCheck can refuse a body for an unrelated reason and mask what the exemption
// actually does. That masking is not hypothetical: #1261's table asserted an AWS
// secret access key was "still caught" when in truth the `=` of the shell assignment
// operator was doing the catching, and the exemption let the bare key through.
//
// The rule replaced here exempted a run only when it was anchored at
// `^/(Users|home|private|tmp|opt)/` AND every segment was <32 chars. Dropping the anchor
// is what fixes #209, and dropping it MUST NOT exempt anything the anchored rule
// refused. Every `false` row below is a string #1261's reviewer demonstrated passing
// under the segment-length-only rule.
func TestIsPathLikeIsNotWiderThanTheAnchoredRule(t *testing.T) {
	cases := []struct {
		name string
		run  string
		want bool
	}{
		// Exempt: real paths, absolute and relative, that the anchored rule refused or
		// allowed. These are the #1052 / #209 pass set.
		{"absolute mac-home path (the #1052 case)", "/Users/operator/work/example/slides", true},
		{"repo-relative file path (the #209 case)", "tools/desk/internal/deskkit/bodycheck", true},
		{"go module path run", "com/medici/desk/internal/loopengine", true},
		{"camelcase template path", "src/Example/Settlement", true},
		{"url run with an empty leading segment", "//localhost/api/v2/state/activecontracts", true},
		{"directory run with a trailing slash", "services/internal/api/handlers/reviewwrite/", true},

		// NOT exempt: opaque token material. Each of these satisfies "every segment is
		// <32 chars, no +/=" and would be exempted by a length-only gate.
		{"aws secret access key, bare", awsExampleSecretKey, false},
		{"slack webhook token run", "com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX", false},
		{"64-char base64 with well-placed slashes", "Kj8mQ2vRt/Lz3wXy7Nb5cHj/1Ae4Gf6Sd0Uk/Pq7Rm2Xn9Tv/Bw3CyZl6Jh8Dr2", false},
		{"secret as the tail segment of a real repo path", "tools/desk/Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz", false},
		{"no slash at all — a bare token is never path-like", "Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz", false},
		{"carries base64 padding", "config/secrets/ZGVhZGJlZWZkZWFkYmVlZmRlYWRiZWVm==", false},
		{"carries base64 plus chars", "config/Zm9v+YmFy+YmF6+cXV4+1234567890abcdef", false},
		{"degenerate 200-char lowercase stretch is not a word", "/Users/a/b/c/" + strings.Repeat("x", 200), false},

		// A git SHA inside a path stays exempt: a BARE SHA is already exempt above, so
		// refusing it only when it wears a URL would refuse every commit link the
		// methodology quotes.
		{"commit url run carrying a 40-char sha", "com/medici/desk/commit/5d529c27e3b1a04f9c2d8e7b6a1f0c3d4e5f6a7b", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isPathLike(c.run); got != c.want {
				t.Fatalf("isPathLike(%q) = %v, want %v", c.run, got, c.want)
			}
		})
	}
}

// TestIsIdentifierLike probes the third exemption directly (#253): a run
// with no `/` at all — so isPathLike never even reaches its segment logic — must still be
// exempted when it is a single word-shaped identifier, and must still be refused when it
// is opaque token material of the same length and charset.
func TestIsIdentifierLike(t *testing.T) {
	cases := []struct {
		name string
		run  string
		want bool
	}{
		// Exempt: real Go identifiers, the #253 repro shapes.
		{"33-char CamelCase Go test identifier (the #253 repro)", "HonoredFilterLeavesTripwireSilent", true},
		{"39-char CamelCase Go test identifier", "ZeroContractTemplateDecodesZeroElements", true},
		{"Test-prefixed identifier", "TestZeroContractTemplateDecodesZeroElements", true},
		{"identifier with an embedded short digit group", "HandlesPython311PathCorrectly", true},
		// #781 — a CamelCase unit that is a known two-letter English word
		// (`By`, `On`, `Is`, `Of`, `Vs`, …). A pre-existing reviewed test name
		// (`…StrandedByArchived…`) and the #392 batch were lost to exactly one such word.
		{"#781: identifier carrying the 2-letter word 'By'", "FinalizeNoneFallbackStrandedByArchivedQuote", true},
		{"#781: identifier carrying 'On', 'Is', 'Vs'", "DecodesOnZeroWhenInputIsEmptyVsFull", true},
		// Guard: the allowance is a CLOSED word set, not "any capital+one-lowercase". A
		// pair that is not an English word (`Xz`) is still debris, so a token is not
		// laundered by planting a whitelisted word in front of it.
		{"#781 guard: unknown capital+one-lowercase pair 'Xz' is still debris", "StrandedByXzArchivedQuoteFallbackNoneHere", false},
		{"#781 guard: whitelisted words then opaque debris still refuses", "OnByIsAtQx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5", false},

		// NOT exempt: opaque token material of comparable length, no slash so isPathLike
		// was never a candidate exemption for these either.
		{"aws secret access key, bare, no slash context", "wJalrXUtnFEMIK7MDENGbPxRfiCYEXAMPLEKEY", false},
		{"40-char mixed-case secret, no slash (the #253 guard-strength case)", "Qx7pLk2wZt9mNc4bYf6RhVs8Ju3XoAeG5idWn1Dz", false},
		{"bare base64 blob with padding", "ZGVhZGJlZWZkZWFkYmVlZmRlYWRiZWVmZGVhZGJlZWY=", false},
		{"degenerate 200-char lowercase stretch is not a word", strings.Repeat("x", 200), false},
		// Regression guard: a single unbroken lowercase run under maxWordSegment
		// decomposes as exactly ONE word unit under the shared decomposition (one word
		// is enough for a PATH SEGMENT, which gets its boundary from the surrounding
		// `/`). A bare run has no such boundary, so isIdentifierLike must require a
		// SECOND word unit — otherwise a 35-char lowercased hex/hash string,
		// indistinguishable in shape from a git SHA one character short of the 40/64
		// exemption, would be wrongly exempted as "identifier-like".
		{"single long lowercase run is one word, not two — must not be exempt", strings.Repeat("q", 35), false},
		// #582 review: lowercase token material with digit groups
		// decomposes into TWO OR MORE letter-word units (the digit groups supply the
		// boundaries), so the two-word floor alone exempted it — both shapes below
		// returned true on the pre-fix head. They carry no capital-led word unit, so
		// they are not identifiers and must not be exempt.
		{"lowercase hex with digit groups, 32 chars, md5-shaped", "dcef1701acc78ada" + "90fcefb638adeabf", false},
		{"lowercase base36 token with digit groups, 32 chars", "wdehl5mcgoou" + "6fuo4ihkpokq" + "3skqeost", false},

		// #663 / #781 — two-letter English words inside a CamelCase
		// identifier. Before twoLetterWords, a single `Is`, `On`, `Vs` or `An` anywhere in
		// the name failed the decomposition at that character and sent the whole
		// identifier to the high-entropy refusal.
		{"issue #663 repro: 41-char CamelCase Go test name", "TestAssayLintSingleWriterGuardFailsClosed", true},
		{"two-letter word Vs", identVs, true},
		{"two-letter word An", identAn, true},
		{"two-letter words Is and On in one name", identIsOn, true},
		{"two-letter words No and Of", identNoOf, true},
		// The relaxation is a CLOSED LIST, not "capital plus any one lowercase". A pair
		// that is not an English word is still debris, which is the whole reason the list
		// costs the scanner almost nothing (see TestTwoLetterWordsCostAlmostNothing).
		{"capital-plus-one-lowercase pairs that are not English words", nonWordPairs, false},
		{"base64 debris pairs Zm/Jl/Vh are not words", "Zm9vYmFyYmF6" + "JlcXV4cXV1eA" + "Vhc2RmZ2hqa2w", false},
		// A capital with ZERO lowercase behind it is untouched by the relaxation: a lone
		// capital and an ALL-CAPS stretch are still refused outright, which is what keeps
		// a webhook token tail and an AWS key's ALL-CAPS tail opaque.
		{"ALL-CAPS stretch is still not a word", identAllCaps, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isIdentifierLike(c.run); got != c.want {
				t.Fatalf("isIdentifierLike(%q) = %v, want %v", c.run, got, c.want)
			}
		})
	}
}

// TestAssignmentStripsOnlyTheKey probes the fourth exemption directly (#775).
// The contract it must hold is narrow: stripping a `key=` prefix may not change the
// verdict the VALUE would get standing alone. So every row here is really two assertions
// — that the idiom is admitted when the value is a genuine path or identifier, and that
// it is refused the instant the value is anything the scanner would refuse bare.
func TestAssignmentStripsOnlyTheKey(t *testing.T) {
	cases := []struct {
		name string
		run  string
		want bool
	}{
		// Exempt: the Verify/Evidence command idiom, with the value a real path.
		{"the #775 repro run", assignRun, true},
		{"two-char shell scratch key", assignTwoChar, true},
		{"word-shaped key over a path", assignWordKey, true},
		{"key over a bare CamelCase identifier value", assignIdentVal, true},
		// The prefix strip must be no STRONGER than the bare bar either. A git SHA is
		// exempt standing alone and the methodology quotes SHAs constantly, so a
		// SHA-valued assignment must not refuse what the SHA alone permits.
		{"key over a 40-char git sha", "base=" + "5d529c27e3b1a04f9c2d8e7b6a1f0c3d4e5f6a7b", true},
		{"key over a 64-char git sha", "sha=" + "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", true},

		// NOT exempt: the value is credential material. The key half is deliberately
		// valid in each row, so the refusal can only be coming from the value.
		{"one-char key over a bare AWS secret access key", "f=" + awsExampleSecretKey, false},
		{"one-char key over a 40-char slash-free secret", "t=" + scanSecret40, false},
		{"word-shaped key over a base64 blob", "data=" + base64Unpadded, false},
		{"value carrying base64 padding is never exempt", "data=" + base64Padded, false},
		{"value carrying base64 plus chars is never exempt", assignPlusVal, false},

		// NOT exempt: the KEY half fails. An over-long, non-word-shaped key is opaque
		// material in its own right and may not be waved through as "a variable name".
		// `AWS_SECRET_ACCESS_KEY=<key>` reduces to a run keyed `KEY` — the underscores are
		// outside the base64 charset, so they truncate the run — and three ALL-CAPS chars
		// are neither short enough nor word-shaped.
		{"ALL-CAPS three-char key is not a shell scratch variable", "KEY=" + awsExampleSecretKey, false},
		{"opaque long key over a legitimate path", assignOpaqueKey, false},

		// Structural rejects.
		{"no equals sign at all", "docs/streams/distribution/version", false},
		{"nothing before the equals", assignNoKey, false},
		{"nothing after the equals", "f=", false},
		{"nested assignment leaves an equals in the value", "a=b=" + scanSecret40, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isAssignmentLike(c.run); got != c.want {
				t.Fatalf("isAssignmentLike(%q) = %v, want %v", c.run, got, c.want)
			}
		})
	}
}

// TestTwoLetterWordsCostAlmostNothing is the guard-strength measurement behind
// twoLetterWords, and the reason that relaxation is a closed list of English words rather
// than the obvious "a capital followed by any single lowercase letter".
//
// Both spellings fix the reported false positives equally well, so the only thing that
// can choose between them is what each costs the scanner on real token material. This
// draws uniformly random base64 runs from a FIXED seed and counts how many each rule
// would exempt. The blanket relaxation admits roughly thirty-five times as many as the
// curated list, which lands within noise of the unrelaxed rule it replaces.
//
// The assertion is a ratio rather than an absolute count so the test pins the PROPERTY —
// "the curated list stays close to the old rule, and far from the blanket one" — instead
// of a seed-specific number that any innocuous edit would have to be re-blessed against.
// If someone later widens twoLetterWords far enough to matter, this goes red.
func TestTwoLetterWordsCostAlmostNothing(t *testing.T) {
	const trials = 2000000
	alphabet := base64Alphabet

	// anyPairDecomposition is the BLANKET alternative: identical to wordDecomposition
	// except that any capital followed by one lowercase counts as a word.
	anyPairIdentifierLike := func(run string) bool {
		if len(run) > maxWordSegment {
			return false
		}
		words, camel := 0, false
		for i := 0; i < len(run); {
			c := run[i]
			switch {
			case c >= 'a' && c <= 'z':
				j := i
				for j < len(run) && run[j] >= 'a' && run[j] <= 'z' {
					j++
				}
				if j-i < minLowerWord {
					return false
				}
				i, words = j, words+1
			case c >= 'A' && c <= 'Z':
				j := i + 1
				for j < len(run) && run[j] >= 'a' && run[j] <= 'z' {
					j++
				}
				if j-(i+1) < 1 {
					return false
				}
				i, words, camel = j, words+1, true
			case c >= '0' && c <= '9':
				j := i
				for j < len(run) && run[j] >= '0' && run[j] <= '9' {
					j++
				}
				if j-i > maxDigitRun {
					return false
				}
				i = j
			default:
				return false
			}
		}
		return words >= 2 && camel
	}

	rng := rand.New(rand.NewSource(20260812))
	var curated, blanket int
	for i := 0; i < trials; i++ {
		b := make([]byte, runThreshold)
		for j := range b {
			b[j] = alphabet[rng.Intn(len(alphabet))]
		}
		run := string(b)
		if isIdentifierLike(run) {
			curated++
		}
		if anyPairIdentifierLike(run) {
			blanket++
		}
	}
	t.Logf("random %d-char base64 runs exempted out of %d: curated list %d, blanket relaxation %d",
		runThreshold, trials, curated, blanket)

	// The curated list must stay an order of magnitude tighter than the blanket rule.
	// Both numbers are tiny, so compare with a floor that keeps the test meaningful
	// without making it flaky: the blanket rule leaks single digits per 400k here.
	if curated*4 > blanket {
		t.Errorf("twoLetterWords has been widened to blanket-relaxation cost: curated "+
			"exempted %d of %d random base64 runs vs blanket %d. The point of a closed "+
			"English-word list is that it buys back the false-positive class without "+
			"measurably loosening the scanner's grip on random token material; if the "+
			"two are now comparable, the list has stopped paying for itself.",
			curated, trials, blanket)
	}
	if curated > trials/10000 {
		t.Errorf("curated list exempted %d of %d random base64 runs — too many for a "+
			"credential seatbelt; twoLetterWords should hold only genuine short words",
			curated, trials)
	}
}

// TestLooksLikeWordsSeparatesNamesFromTokens pins the discriminator itself. It is the
// only thing standing between "a review body may cite `file:line`" and "a credential may
// wear slashes", so it gets its own table rather than being tested only through two
// layers of caller.
func TestLooksLikeWordsSeparatesNamesFromTokens(t *testing.T) {
	words := []string{
		"tools", "internal", "deskkit", "bodycheck", "Users", "Payments",
		"Example", "Settlement", "PositionSummaryPanel",
		"python311", "activecontracts", "ledger", "handlers",
	}
	tokens := []string{
		"wJalrXUtnFEMI",                 // segment 1 of an AWS secret access key
		"K7MDENG",                       // segment 2 — a lone capital, then digits
		"bPxRfiCYEXAMPLEKEY",            // segment 3 — one-letter runs and an ALL-CAPS tail
		"T00000000",                     // a Slack channel id
		"XXXXXXXXXXXXXXXXXXXXXXXX",      // a webhook token tail
		"Qx7pLk2wZt9mNc4bYf6RhVs8Ju3Xo", // raw base64
		"2026",                          // digits alone are not a word
		"v2",                            // a one-letter run
		"zz",                            // a two-letter run
		strings.Repeat("x", 200),        // word-SHAPED but far past maxWordSegment
	}
	for _, w := range words {
		if !looksLikeWords(w) {
			t.Errorf("looksLikeWords(%q) = false, want true — a real path segment was "+
				"classified as opaque; enough of these in one run and a legitimate "+
				"review body is refused again (#209)", w)
		}
	}
	for _, tok := range tokens {
		if looksLikeWords(tok) {
			t.Errorf("looksLikeWords(%q) = true, want false — token material classified "+
				"as name material spends no opaque budget, which is how a credential "+
				"gets exempted", tok)
		}
	}
}

// #328 test vectors are assembled from SPLIT fragments for the same reason
// awsExampleSecretKey above is: deskpr secret-scans the branch diff before it pushes, so a
// contiguous credential literal on an ADDED line refuses the very PR that adds the test.
// Runtime values are exact, so detection is exercised in full.
var (
	scanSecret40 = "Qx7pLk2wZt9mNc4bYf6RhVs8" + "Ju3XoAeG5idWn1Dz"
	scanGHToken  = "ghp" + "_0123456789abcdefghij"
	scanAWSKeyID = "AKIA" + "IOSFODNN7EXAMPLE"
	scanPEM      = "-----BEGIN RSA " + "PRIVATE KEY-----"
	scanJWT      = "eyJ" + "hbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.abc"
	scanENC      = "ENC" + "[AES256_GCM,data:x]"
	// 53 of the 64 hex characters of a real pin — the length git's funcname truncation
	// produces, which is neither 40 nor 64 and so falls outside the git-SHA exemption.
	scanTruncSHA = "a9145f3aa48deb0d85a2b42944381a0" + "2aea10a7d7b9154592d26b"
)

// TestScanSurfaceNamesIt covers the second half of #328: a tool that scans
// several surfaces before it writes must say WHICH one refused. deskpr scans the PR
// title, the branch name and the branch diff; every one of them used to report as "body",
// so an operator hit by a diff-triggered refusal spent two rounds rewriting a body that
// was never the problem.
func TestScanSurfaceNamesIt(t *testing.T) {
	cases := []struct {
		surface string
		content string
	}{
		{"branch diff vs origin/main", "+token = " + scanSecret40},
		{"PR title", "fix: " + scanSecret40},
		{"branch name", "fix/" + scanSecret40},
		{"commit content", scanGHToken},
		{"evidence file", scanPEM},
		{"review body", scanAWSKeyID},
		{"comment", scanJWT},
		{"issue body", scanENC},
	}
	for _, c := range cases {
		err := ScanSurface(c.surface, []byte(c.content))
		if err == nil {
			t.Errorf("ScanSurface(%q, …) = nil, want a refusal", c.surface)
			continue
		}
		if !strings.Contains(err.Error(), c.surface) {
			t.Errorf("ScanSurface(%q, …) refusal does not name its surface: %v", c.surface, err)
		}
		if ExitCodeOf(err) != ExitRefused {
			t.Errorf("ScanSurface(%q, …) exit code = %d, want %d", c.surface, ExitCodeOf(err), ExitRefused)
		}
	}
}

// TestScanSurfaceSameVerdict pins that naming the surface changed only the MESSAGE. Every vector must get the same verdict from both entry points, and BodyCheck
// must still say "body" for the callers that never name a surface.
func TestScanSurfaceSameVerdict(t *testing.T) {
	vectors := []string{
		"",
		"a clean review body with a file ref tools/desk/internal/deskkit/bodycheck.go:45",
		"sha e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"truncated " + scanTruncSHA,
		scanGHToken,
		scanAWSKeyID,
		scanPEM,
		scanJWT,
		scanENC,
		scanSecret40,
		awsExampleSecretKey,
	}
	for _, v := range vectors {
		bodyErr, surfErr := BodyCheck([]byte(v)), ScanSurface("PR title", []byte(v))
		if (bodyErr == nil) != (surfErr == nil) {
			t.Errorf("verdicts diverge for %q: BodyCheck=%v ScanSurface=%v", v, bodyErr, surfErr)
		}
		if bodyErr != nil && !strings.Contains(bodyErr.Error(), "body contains") {
			t.Errorf("BodyCheck(%q) no longer says \"body\": %v", v, bodyErr)
		}
	}
	// An empty surface degrades to the default rather than emitting "refused:  contains".
	if err := ScanSurface("", []byte(scanGHToken)); err == nil ||
		!strings.Contains(err.Error(), SurfaceBody+" contains") {
		t.Errorf("ScanSurface(\"\", …) should fall back to %q, got: %v", SurfaceBody, err)
	}
}
