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

	// Word-shaped Go test names that CLOSE on a short (2-4 char) ALL-CAPS acronym. Each is
	// over the 32-char run threshold and — pre-fix — was refused as a high-entropy run
	// regardless of maxIdentifierRun, because the trailing acronym's leading capital hit
	// wordDecomposition's "capital with no lowercase" branch and sank the whole identifier.
	// The trailing-acronym relaxation admits them. Split across concatenation for the same
	// deskpr-diff-scan reason as scanSecret40 below (each fragment is under the threshold).
	identTrailOK   = "TestHandlesSilenceAnd" + "ReturnsNotFoundOK" // 38 chars, trailing 2-char OK
	identTrailHTTP = "TestRequestParserSpeaks" + "OnlyPlainHTTP"   // 36 chars, trailing 4-char HTTP
	// Guard fixtures the relaxation must STILL refuse:
	identTrailHTTPS = "TestRequestParserSpeaks" + "OnlyPlainHTTPS" // 37 chars, trailing acronym is 5 chars — too long
	// A high-entropy run that merely ENDS in two capitals — the relaxation must not launder
	// it: its body is base64 debris (the `Qx` pair is capital-plus-one-lowercase, not a word),
	// so it refuses at that pair long before the tail, exactly as it did before. Named without
	// a credential keyword so the pattern-sweep's keyword-triggered generic rule stays quiet.
	endsCapsDebris = "Qx7pLk2wZt9mNc4bYf6Rh" + "Vs8Ju3XoAeAB"

	// #775 — the Verify/Evidence `key=path` shell-assignment idiom.
	assignRun      = "f=docs/streams/" + "example-stream/version"
	assignEvidence = "f=docs/streams/" + "example-stream/version-scheme.md"
	assignTwoChar  = "fp=tools/desk/" + "internal/deskkit/bodycheck"
	assignWordKey  = "manifest=docs/" + "publication/manifest/classification"
	assignIdentVal = "t=TestHonoredFilter" + "LeavesTripwireSilent"
	// Guard-strength: the key half is valid in each, so only the VALUE can be refusing.
	assignNoKey     = "=docs/streams/" + "example-stream/version"
	assignOpaqueKey = "Qx7pLk2wZt9mNc4bYf6Rh" + assignNoKey
	assignPlusVal   = "f=config/Zm9v" + "+YmFy+YmF6+cXV4" + "+1234567890abcdef"
	base64Unpadded  = "ZGVhZGJlZWZkZWFk" + "YmVlZmRlYWRiZWVm" + "ZGVhZGJlZWY"
	base64Padded    = "ZGVhZGJlZWZkZWFk" + "YmVlZmRlYWRiZWVm" + "=="
	// The randomness alphabet for TestTwoLetterWordsCostAlmostNothing — 64 base64 chars
	// contiguous is itself a refused run, so it is split like everything else here.
	base64Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "0123456789+/"

	// #203 — Go test names carrying a MID-RUN short acronym. Each is over the 32-char run
	// threshold and was refused before the acronym relaxation moved off the closing
	// position, because the acronym's leading capital hit wordDecomposition's "capital with
	// no lowercase" branch and sank the whole identifier. These four are verbatim shapes
	// taken from this repository's own suites — `CI`, `ID`, `PDF`, `HTTP` — which is the
	// point: the author of prose about them cannot reword a shipped symbol.
	// Split across concatenation for the same deskpr-diff-scan reason as scanSecret40.
	identMidCI   = "TestCIRequiredMatches" + "AllowedRepoPolicy"    // 38, mid-run 2-char CI
	identMidID   = "TestResolveInstallID" + "HasNoSilentFallback"   // 39, mid-run 2-char ID
	identMidPDF  = "TestPDFIsByteIdentical" + "AcrossRenders"       // 35, mid-run 3-char PDF
	identMidHTTP = "TestHTTPClientIs" + "BoundedNotDefaultAnywhere" // 41, mid-run 4-char HTTP
	// Guard fixtures the MID-RUN relaxation must STILL refuse, one per bound. Each isolates
	// ONE of shortAcronymUnit's bounds, so a mutation that removes that bound is caught by
	// exactly one row and the report says which bound stopped holding.
	identMidCaps5 = "TestHandlesHTTPSProxy" + "UpstreamHealthCheck"  // SHORT: 5-letter caps run
	identMidDigit = "abcdEFGH1234" + "abcdEFGH1234" + "abcdEFGH1234" // random-ish: caps runs among digits
	identAcrDigit = "TestHandlesRetryHTTP2" + "AndThenTheProxy"      // ANCHORED FORWARD: one acronym, digit after it
	identLeadAcr  = "CIRequiredMatches" + "AllowedRepoPolicyHere"    // BACKWARD/AFTER-A-WORD: acronym opens the run
	identTwoAcr   = "TestCIAndPDFTogether" + "InOneNameHereNow"      // BUDGETED: two acronyms in one run
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
		{"repo-relative file:line, no backticks", "blocker at pkg/service/internal/api/handlers/reviewwrite.go:212", false},
		{"deep frontend component path", "frontend/src/components/dashboard/ConfigurationManager.tsx:88", false},
		// CamelCase package template paths — the shape that makes case-transition and
		// mean-segment-length heuristics useless as discriminators (see isPathLike):
		// this and an AWS secret access key are identical on both measures.
		{"camelcase package template path", "see pkg/Example/Widget.pkg:120 for WidgetPV2", false},
		{"a review body dense with findings", "## Review\n\n- `tools/desk/internal/deskkit/ratelimit.go:26` counts all attempts\n- `tools/desk/internal/deskkit/bodycheck.go:59` refuses the run\n- `frontend/src/lib/updatesCursor.ts:41` cursor handling\n\nVerdict: request-changes", false},
		// Go module paths. #209 records these as a SECOND false-positive class that a
		// `file:line`-only narrowing would NOT have fixed: any review of Go code carries
		// import paths, and `go test` output — which the methodology mandates pasting as
		// Evidence — is nothing but module paths.
		{"go module path, this repo", "ok github.com/medici-finance/assay/tools/desk/internal/deskkit 2.531s", false},
		{"go module path, nested cmd package", "ok github.com/example/desk/cmd/deskpost/internal/bodycheck 0.378s", false},
		{"deep github blob url path run", "https://github.com/example/desk/blob/main/tools/desk/internal/deskkit/bodycheck.go", false},
		// Shapes with EMPTY slash-separated segments: a URL's `//` and a directory
		// reference written with a trailing slash. A draft of the #1261 fix rejected
		// empty segments and re-created the bug on both; they carry no material.
		{"url run carrying a leading double slash", "read via https://localhost/api/v2/state/deployments on the box", false},
		{"directory path with a trailing slash", "the directory pkg/service/internal/api/handlers/ holds it", false},
		// Numeric path components: `2026`, `07`, `30` and the `v2` of an API path are not
		// word-shaped, but they are far too short to matter and only spend opaque budget.
		{"dated report path", "report at docs/reports/2026/07/30/factory-floor-summary.md:14", false},
		// #253 — long CamelCase Go test identifiers, bare (no `/` to
		// trigger isPathLike). #1588's review post refused with exit 5 on exactly
		// this shape: a 39/33-char identifier-only run naming the tests under review.
		{"issue #253 repro: 39-char Go test identifier", "`ZeroContractTemplateDecodesZeroElements` now covers the empty-elements branch", false},
		{"issue #253 repro: 33-char Go test identifier", "see HonoredFilterLeavesTripwireSilent for the tripwire assertion", false},
		{"Test-prefixed identifier over the run threshold", "TestZeroContractTemplateDecodesZeroElements passed locally", false},
		{"review body naming several long test idents", "## Review\n\n- TestReducesQueueBacklogUsingTripwireEvent covers the happy path\n- HonoredFilterLeavesTripwireSilent covers the silence case\n\nVerdict: approve", false},
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
		// #778 — ENCRYPTED material is not a plaintext secret. The scan catches
		// UNENCRYPTED secrets; sops ciphertext committed at rest is the sanctioned flow,
		// and the blanket "any ENC[ or sops document refuses" rule blocked it outright.
		//
		// This is the row #778 exists for, and it is the one that passes: a COMPLETE
		// encrypted manifest — full envelope grammar, document signature, and encrypted
		// content outside the metadata block. sanctionedSopsDocument recognises it and
		// markerSurface neutralises its markers, so no arm fires on it.
		{"#778: fully sops-encrypted k8s Secret", encryptedSecretFixture, false},
		// The same sanctioned artifact in JSON. This is the CRY-WOLF guard for the widened
		// `kind:`/`data:` anchors that let the rule see JSON at all: widening what a
		// detector can see must not make it refuse the artifact the feature exists to
		// permit. Measured passing.
		{"#778: fully sops-encrypted k8s Secret, JSON form", sanctionedJSONSecretFixture, false},
		// --- refuse ---
		// The FRAGMENTS below were drafted as passes on this branch and are refusals on the
		// landed rules, which is the correct verdict and is asserted from the other side by
		// structured_test.go and the pos-sops-* corpus fixtures. The pass has to be earned
		// by the whole shape, never by one marker: a lone envelope is a value somebody
		// pasted, and a bare `sops:` footer is a fragment whose own justification
		// ("the secrets in this file are ciphertext") says nothing about it. Neither
		// carries a base64 run over the 32-char threshold, so the entropy loop cannot cover
		// them — the sops arm is the only thing that does.
		{"#778: sops metadata mapping alone", "sops:\n  kms: []", true},
		{"#778: sops document footer (mac ciphertext)", "sops:\n    mac: " + encVal("Zm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFy") + "\n    version: 3.7.3", true},
		{"#778: a single sops-encrypted value", "value: " + encVal("Zm9vZm9vZm9vZm9vZm9vZm9vZm9vZm9v"), true},
		// An age recipient is a PUBLIC key and an armor body is ciphertext, so neither is a
		// leak — but neither is exempt either, OUTSIDE a recognised sops document. This
		// branch had added a standalone age-recipient exemption and an armor-body span;
		// both were dropped as part of taking the landed, document-anchored rules, so both
		// of these now refuse as high-entropy runs. Recorded as a known over-refusal rather
		// than left as a surprise: see TestArmorBodyStillScansOutsideSopsDocuments, which
		// asserts the armor half deliberately.
		{"#778: age recipient is a PUBLIC key", "recipients:\n  - " + ageRecipientFixture, true},
		{"#778: age armor block body", ageArmorFixture, true},
		// --- refuse ---
		{"github classic token", "token ghp" + "_0123456789ABCDEFabcdef0123456789ABCD", true},
		{"github fine-grained pat", "github" + "_pat_11ABCDE0123456789_abcDEF0123456789abcDEF", true},
		{"github server token ghs_", "ghs" + "_abcdEFGH1234abcdEFGH1234abcdEFGH1234", true},
		{"github oauth token gho_", "gho" + "_abcdEFGH1234abcdEFGH1234abcdEFGH1234", true},
		{"aws access key id", "key AKIA" + "IOSFODNN7EXAMPLE here", true},
		{"pem header", "-----BEGIN RSA " + "PRIVATE KEY-----\nMIIE...", true},
		{"jwt shape", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJ", true},
		// #778 POSITIVE CONTROLS — the leak this rule exists to catch, and the two
		// bypasses an earlier draft of it admitted. Each row is red against the draft
		// that shipped without it; see TestEncryptedExemptionsAreContained for the
		// reproduction of the two bypasses in isolation.
		//
		// (a) The headline case: a DECRYPTED Secret. `data:` is base64 ENCODING, and a
		// short value never reaches the 32-char entropy rule, so nothing else catches it.
		{"#778: DECRYPTED k8s Secret (short base64 data value)", decryptedSecretFixture, true},
		{"#778: DECRYPTED k8s Secret (stringData, plaintext)", stringDataSecretFixture, true},
		// (b) PARTIAL encryption must not license the plaintext field beside it. The
		// whole-document "is there an ENC[ marker anywhere" guard passed this.
		{"#778: partially-encrypted k8s Secret (one plaintext value)", partialSecretFixture, true},
		// (c) A foreign blob merely SHARING A LINE with a sops bracket is not inside it.
		// The line-scoped exemption passed this.
		{"#778: secret co-located on a line with a sops ENC bracket", "note: " + encOpen + "data:x,type:str] leaked=" + scanSecret40, true},
		// (d) A Secret whose data mapping cannot be read is a could-not-check, not clean.
		{"#778: k8s Secret with an unreadable data mapping", "kind: Secret\ndata:\n  # values injected at runtime\n", true},
		// (e) Encrypted-at-rest is not a licence for every armor label: a PKCS#8
		// passphrase-encrypted key rests on a passphrase this scan cannot weigh.
		{"#778: PKCS#8 ENCRYPTED PRIVATE KEY is still refused", pkcs8Fixture, true},
		// (f) A real credential does not stop being one because a sops block is nearby.
		{"#778: github token inside a sops document", "sops:\n  mac: " + encVal("Zm9v") + "\ntoken: " + scanGHToken, true},
		{"#778: unencrypted RSA key beside a sops block", "sops:\n  kms: []\n" + scanPEM + "\nMIIE...", true},
		// (g) `age1` as a bare prefix is not an age recipient. A 40-char run wearing it
		// must still refuse — the exemption is length- and alphabet-locked.
		{"#778: age1-prefixed run that is not a recipient", "key: age1" + scanSecret40, true},
		// (h) SURFACE COVERAGE. The rule is only worth what the shapes it can SEE are
		// worth, and the first spelling of these anchors — a bare unquoted `kind:` key
		// ending at `\s*$` — could not see the two commonest accidental-paste shapes.
		// Each row below carries the SAME plaintext value as (a) and was measured
		// ADMITTED before the anchors were widened, while (a) itself refused.
		{"#778: DECRYPTED Secret, kind line with a trailing comment",
			"apiVersion: v1\nkind: Secret # app creds\ndata:\n  password: aHVudGVyMg==\n", true},
		{"#778: DECRYPTED Secret, pretty-printed JSON (kubectl get -o json)",
			"{\n  \"apiVersion\": \"v1\",\n  \"kind\": \"Secret\",\n  \"data\": {\n" +
				"    \"password\": \"aHVudGVyMg==\"\n  }\n}\n", true},
		// (b) again, on the JSON surface: partial encryption must not license the
		// plaintext value beside it there either.
		{"#778: partially-encrypted k8s Secret, JSON form", partialJSONSecretFixture, true},
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
		{"unpadded 40-char secret under a k8s manifest path", "k8s/dev/config/" + "R2hpJ0kZ7vQx3TmLp9WdYc1BnEa6UfSg4XoIrKtZ", true},
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
		{"go module path run", "com/example/desk/internal/loopengine", true},
		{"camelcase package template path", "pkg/Example/Widget", true},
		{"url run with an empty leading segment", "//localhost/api/v2/state/deployments", true},
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
		{"commit url run carrying a 40-char sha", "com/example/desk/commit/5d529c27e3b1a04f9c2d8e7b6a1f0c3d4e5f6a7b", true},
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
		// A short (2-4 char) ALL-CAPS acronym CLOSING an otherwise word-shaped identifier is
		// admitted — the trailing acronym rides on the word-shaped body and maxIdentifierRun
		// governs length. Only the closing position and 2-4 letters are relaxed.
		{"trailing 2-char acronym OK closes a word-shaped identifier", identTrailOK, true},
		{"trailing 4-char acronym HTTP closes a word-shaped identifier", identTrailHTTP, true},
		// Guard: a 5+ char trailing acronym is NOT admitted; a mid-run acronym is not either
		// (identAllCaps above); and a token that merely ends in two capitals is not laundered.
		{"trailing acronym over 4 chars (HTTPS) still refuses", identTrailHTTPS, false},
		{"a run that merely ends in two capitals is not laundered", endsCapsDebris, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isIdentifierLike(c.run); got != c.want {
				t.Fatalf("isIdentifierLike(%q) = %v, want %v", c.run, got, c.want)
			}
		})
	}
}

// TestTrailingAcronymIdentifiers pins the trailing-short-acronym relaxation end-to-end
// through the real BodyCheck write gate. It is the security gate for the change: the fix is
// worthless if it weakens secret detection, so the refuse table asserts that a genuine
// high-entropy run, a 5+ char trailing acronym, and a mid-run ALL-CAPS stretch all still hit
// the exit-5 refusal — the relaxation buys ONLY a short acronym that closes a word-shaped run.
func TestTrailingAcronymIdentifiers(t *testing.T) {
	// PASS: a word-shaped identifier ending in a 2-4 char acronym, at 33-100 chars, that the
	// PRE-fix scanner refused as a high-entropy run regardless of the maxIdentifierRun cap.
	pass := []struct {
		name string
		run  string
	}{
		{"trailing 2-char acronym (OK), 38 chars", identTrailOK},
		{"trailing 4-char acronym (HTTP), 36 chars", identTrailHTTP},
	}
	for _, c := range pass {
		t.Run("pass/"+c.name, func(t *testing.T) {
			if !isIdentifierLike(c.run) {
				t.Fatalf("isIdentifierLike(%q) = false, want true — a legitimate identifier is refused", c.run)
			}
			if err := BodyCheck([]byte("see " + c.run + " for the assertion")); err != nil {
				t.Fatalf("BodyCheck rejected a body naming %q: %v", c.run, err)
			}
		})
	}

	// REFUSE: the relaxation must NOT widen high-entropy detection. Every row here is 33+
	// chars, so BodyCheck scans it, and every one must still refuse with exit 5.
	refuse := []struct {
		name string
		run  string
	}{
		{"genuine 40-char base64 secret still refuses (the entropy guard)", scanSecret40},
		{"AWS example secret key, slashes stripped, still refuses", "wJalrXUtnFEMI" + "K7MDENG" + "bPxRfiCYEXAMPLEKEY"},
		{"trailing acronym over 4 chars (HTTPS) still refuses", identTrailHTTPS},
		{"mid-run ALL-CAPS stretch still refuses", identAllCaps},
		{"a run ending in two capitals is not laundered", endsCapsDebris},
		{"32 capital A's — a trailing run of only capitals is one word, needs two", strings.Repeat("A", 40)},
	}
	for _, c := range refuse {
		t.Run("refuse/"+c.name, func(t *testing.T) {
			if isIdentifierLike(c.run) {
				t.Fatalf("isIdentifierLike(%q) = true, want false — token/over-length material admitted", c.run)
			}
			if err := BodyCheck([]byte(c.run)); !IsRefused(err) {
				t.Fatalf("BodyCheck(%q) = %v, want Refused (exit 5)", c.run, err)
			}
		})
	}
}

// TestMidRunAcronymIdentifiers is #203's half of the acronym relaxation: the acronym no
// longer has to CLOSE the identifier. Every pass row is a verbatim Go test name from this
// repository, so the fixture and the bug report are the same artifact.
//
// The refuse rows are one per bound in shortAcronymUnit's SECURITY SHAPE list. They are the
// reason this is a widening of the identifier exemption rather than a hole: a caps run that
// is too long, one adjacent to digits rather than to a word, one that opens the run, and a
// run spending more than the per-run budget all still refuse.
func TestMidRunAcronymIdentifiers(t *testing.T) {
	pass := []struct {
		name string
		run  string
	}{
		{"mid-run 2-char acronym (CI)", identMidCI},
		{"mid-run 2-char acronym (ID)", identMidID},
		{"mid-run 3-char acronym (PDF)", identMidPDF},
		{"mid-run 4-char acronym (HTTP)", identMidHTTP},
	}
	for _, c := range pass {
		t.Run("pass/"+c.name, func(t *testing.T) {
			if !isIdentifierLike(c.run) {
				t.Fatalf("isIdentifierLike(%q) = false, want true — a shipped test name is refused, "+
					"and renaming the symbol is not available to the author of prose about it", c.run)
			}
			if err := BodyCheck([]byte("the assertion is in " + c.run + " today")); err != nil {
				t.Fatalf("BodyCheck rejected a body naming %q: %v", c.run, err)
			}
		})
	}

	refuse := []struct {
		name, bound string
		run         string
	}{
		{"5-letter caps run (HTTPS)", "SHORT", identMidCaps5},
		{"one acronym followed by a digit, not a word", "ANCHORED FORWARD", identAcrDigit},
		{"caps runs scattered among digit groups", "ANCHORED FORWARD / BUDGETED", identMidDigit},
		{"acronym OPENS the run", "AFTER A WORD / ANCHORED BACKWARD", identLeadAcr},
		{"two acronyms in one run", "BUDGETED", identTwoAcr},
		{"a genuine 40-char base64 secret", "the entropy guard itself", scanSecret40},
		{"AWS example secret key, slashes stripped", "the entropy guard itself",
			"wJalrXUtnFEMI" + "K7MDENG" + "bPxRfiCYEXAMPLEKEY"},
	}
	for _, c := range refuse {
		t.Run("refuse/"+c.name, func(t *testing.T) {
			if isIdentifierLike(c.run) {
				t.Fatalf("isIdentifierLike(%q) = true, want false — the %q bound has stopped holding",
					c.run, c.bound)
			}
			if err := BodyCheck([]byte(c.run)); !IsRefused(err) {
				t.Fatalf("BodyCheck(%q) = %v, want Refused (exit 5)", c.run, err)
			}
		})
	}
}

// TestShortAcronymCostsAlmostNothing is the guard-strength measurement behind the acronym
// relaxation, in the shape TestTwoLetterWordsCostAlmostNothing established for the
// two-letter-word list: a fixed seed, the same 2,000,000 uniformly random base64 runs, and
// a count of how many each rule exempts.
//
// It exists because the FIRST version of the mid-run relaxation — 2-4 caps, anchored forward
// only — measured 27 exempted, the same order as the blanket capital-plus-one-lowercase rule
// twoLetterWords was written to stay well under. The three further bounds (backward anchor,
// after-a-word, per-run budget) were added in response to that number, not to a hunch, and
// this test is what keeps a future maintainer from removing one on the reasoning that "the
// acronym rule is already bounded".
//
// The assertions are a ratio against the WITHOUT-acronyms rule plus an absolute ceiling.
// The ratio alone would be unusable here: the without-acronyms rule exempts ZERO at this
// length, and nothing is a useful multiple of zero.
func TestShortAcronymCostsAlmostNothing(t *testing.T) {
	const trials = 2000000
	// The absolute ceiling. The measured value at this seed is 7; the ceiling sits above it
	// with room for an innocuous edit and far below the ~35 the blanket relaxation costs, so
	// a real widening trips it and a rounding does not.
	const ceiling = 20

	withoutAcronyms := func(run string) bool {
		if len(run) > maxIdentifierRun {
			return false
		}
		ok, words, camel := wordDecomposition(run, false /* allowShortAcronym */)
		return ok && words >= 2 && camel
	}

	rng := rand.New(rand.NewSource(20260812))
	var shipped, noAcronym int
	for i := 0; i < trials; i++ {
		b := make([]byte, runThreshold)
		for j := range b {
			b[j] = base64Alphabet[rng.Intn(len(base64Alphabet))]
		}
		run := string(b)
		if isIdentifierLike(run) {
			shipped++
		}
		if withoutAcronyms(run) {
			noAcronym++
		}
	}
	t.Logf("random %d-char base64 runs exempted out of %d: shipped rule %d, acronym rule OFF %d",
		runThreshold, trials, shipped, noAcronym)

	if shipped > ceiling {
		t.Errorf("the short-acronym relaxation now exempts %d of %d random base64 runs "+
			"(ceiling %d, acronym rule off %d). One of shortAcronymUnit's six bounds has been "+
			"loosened or removed; the relaxation exists to admit shipped test names, not to "+
			"give random token material a route through the entropy rule.",
			shipped, trials, ceiling, noAcronym)
	}
	// Cross-check the arithmetic the ceiling rests on: the rule with acronyms OFF must be
	// the tighter of the two, or the measurement is not measuring what it says it is.
	if noAcronym > shipped {
		t.Errorf("acronym rule OFF exempted MORE (%d) than the shipped rule (%d) — the "+
			"comparison is inverted and the ceiling above means nothing", noAcronym, shipped)
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
		{"no equals sign at all", "docs/streams/example-stream/version", false},
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
		"tools", "internal", "deskkit", "bodycheck", "Users", "Reports",
		"Example", "Widget", "ConfigurationManager",
		"python311", "deployments", "records", "handlers",
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
	// 53 of the 64 hex characters of a real pin — the length git's funcname truncation
	// produces, which is neither 40 nor 64 and so falls outside the git-SHA exemption.
	scanTruncSHA = "a9145f3aa48deb0d85a2b42944381a0" + "2aea10a7d7b9154592d26b"
)

// #778 fixtures. Like awsExampleSecretKey and the scan* vars above, the sops/armor
// markers are assembled from SPLIT fragments: deskpr scans the branch diff before it
// pushes, and the scanner in force while this PR is open still refuses a contiguous
// `ENC[AES256_GCM` / `-----BEGIN` literal on an added line (#380).
var (
	encOpen  = "ENC" + "[AES256_GCM,"
	pemOpen  = "-----" + "BEGIN "
	pemClose = "-----" + "END "
	dashes   = "-----"

	// encVal returns a well-formed sops encrypted value bracket carrying `data` bytes.
	encVal = func(data string) string {
		return encOpen + "data:" + data + ",iv:aXZpdml2aXZpdml2,tag:dGFndGFndGFn,type:str]"
	}

	// A DECRYPTED Secret: the value is base64 ENCODING, not encryption, and at 12
	// characters it is far under the 32-char high-entropy run threshold. Nothing but a
	// positive `kind: Secret` rule catches this.
	decryptedSecretFixture = "apiVersion: v1\nkind: Secret\nmetadata:\n  name: app-creds\ntype: Opaque\ndata:\n  password: aHVudGVyMg==\n"

	// The same leak in stringData, where the value is not even base64.
	stringDataSecretFixture = "apiVersion: v1\nkind: Secret\nmetadata:\n  name: app-creds\nstringData:\n  password: hunter2-plaintext\n"

	// PARTIAL encryption — one encrypted field, one short plaintext field beside it (the
	// sops `unencrypted_suffix` shape). A whole-document "any ENC[ marker present" guard
	// switches the Secret rule off for the entire file and lets the plaintext through.
	partialSecretFixture = "apiVersion: v1\nkind: Secret\nmetadata:\n  name: app-creds\ndata:\n  tls.key: " +
		encVal("Zm9vYmFyZm9vYmFy") + "\n  password: aHVudGVyMg==\n"

	// A fully sops-encrypted Secret — every data value is ciphertext. This is the
	// sanctioned commit-at-rest shape #778 asked for, and it must pass.
	encryptedSecretFixture = "apiVersion: v1\nkind: Secret\nmetadata:\n  name: app-creds\ndata:\n  password: " +
		encVal("Zm9vYmFyZm9vYmFyZm9vYmFy") + "\n  tls.key: " + encVal("a2V5a2V5a2V5a2V5a2V5") +
		"\nsops:\n  mac: " + encVal("bWFjbWFjbWFjbWFjbWFj") + "\n  version: 3.7.3\n"

	// The JSON serialisations of the two shapes above. `kubectl get secret -o json` is
	// how a Secret most often reaches a paste buffer, so the rule has to read it, and the
	// PASS case has to keep passing there — a widening that made the sanctioned artifact
	// refuse would just be a differently-shaped false positive.
	sanctionedJSONSecretFixture = "{\n  \"apiVersion\": \"v1\",\n  \"kind\": \"Secret\",\n  \"data\": {\n" +
		"    \"password\": \"" + encVal("Zm9vYmFyZm9vYmFyZm9vYmFy") + "\"\n  },\n" +
		"  \"sops\": {\n    \"mac\": \"" + encVal("bWFjbWFjbWFjbWFjbWFj") + "\",\n" +
		"    \"version\": \"3.7.3\"\n  }\n}\n"

	partialJSONSecretFixture = "{\n  \"apiVersion\": \"v1\",\n  \"kind\": \"Secret\",\n  \"data\": {\n" +
		"    \"tls.key\": \"" + encVal("Zm9vYmFyZm9vYmFy") + "\",\n" +
		"    \"password\": \"aHVudGVyMg==\"\n  },\n" +
		"  \"sops\": {\n    \"mac\": \"" + encVal("bWFjbWFjbWFjbWFjbWFj") + "\",\n" +
		"    \"version\": \"3.7.3\"\n  }\n}\n"

	// An age X25519 recipient: `age1` + 58 bech32 characters. A PUBLIC key.
	ageRecipientFixture = "age1" + strings.Repeat("qpzry9x8gf2tvdw0s3jn54khce6mua7l", 2)[:58]

	// An age armor block. Its base64 body is ciphertext bound to a key the repo does
	// not hold.
	ageArmorFixture = pemOpen + "AGE ENCRYPTED FILE" + dashes + "\n" +
		"YWdlLWVuY3J5cHRpb24ub3JnL3YxCi0+IFgyNTUxOSBhYmNkZWZnaGlqa2xtbm9w\n" +
		"cXJzdHV2d3h5ejAxMjM0NTY3ODlhYmNkZWZnaGlqa2xtbm9wcXJzdA\n" +
		pemClose + "AGE ENCRYPTED FILE" + dashes

	// PKCS#8 passphrase-encrypted key — deliberately NOT in encryptedArmorLabels.
	pkcs8Fixture = pemOpen + "ENCRYPTED PRIVATE KEY" + dashes + "\nMIIE...\n" +
		pemClose + "ENCRYPTED PRIVATE KEY" + dashes
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
		{"issue body", decryptedSecretFixture},
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
		decryptedSecretFixture,
		encryptedSecretFixture,
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

// TestEncryptedExemptionsAreContained is the fail-first proof for the two bypasses that
// the first draft of the #778 rules admitted. Each subtest is written against the
// PREVIOUS spelling of the rule and is red under it; the current spelling is what makes
// them green. Keeping them here means a future simplification back to "is the marker
// nearby" cannot land quietly.
//
//   - proximityExempt reproduces the LINE-SCOPED exemption: a base64 run was exempted
//     whenever the literal `ENC[AES256_GCM` appeared anywhere on the run's physical
//     line. A 40-char run that refuses on its own therefore passed simply by being
//     printed beside a real sops value.
//   - documentEncrypted reproduces the WHOLE-DOCUMENT k8s guard: the `kind: Secret`
//     refusal was switched off for the entire file if any ENC[ marker existed anywhere
//     in it, so a single encrypted field licensed every plaintext field beside it.
func TestEncryptedExemptionsAreContained(t *testing.T) {
	proximityExempt := func(s string, start int, run string) bool {
		lineStart := strings.LastIndexByte(s[:start], '\n') + 1
		lineEnd := start + len(run)
		if nl := strings.IndexByte(s[lineEnd:], '\n'); nl >= 0 {
			lineEnd += nl
		} else {
			lineEnd = len(s)
		}
		return strings.Contains(s[lineStart:lineEnd], encOpen[:len(encOpen)-1])
	}
	documentEncrypted := func(s string) bool {
		return !strings.Contains(s, encOpen[:len(encOpen)-1])
	}

	t.Run("co-located blob is not inside the bracket", func(t *testing.T) {
		body := "note: " + encOpen + "data:x,type:str] leaked=" + scanSecret40
		start := strings.Index(body, scanSecret40)
		if !proximityExempt(body, start, scanSecret40) {
			t.Fatal("positive control is not positive: the superseded line-scoped rule " +
				"must exempt this run, or it proves nothing")
		}
		if err := BodyCheck([]byte(body)); err == nil {
			t.Error("BodyCheck admitted a 40-char secret co-located with a sops value")
		}
	})

	// A bare envelope is NOT a pass. This subtest asserted the opposite while #806 carried
	// its own bracket-interior exemption; the exemption is now structuredExemptSpans' alone
	// and it requires the full envelope grammar PLUS the sops document signature. An
	// envelope on its own is a value someone pasted, not a manifest the house committed —
	// the same contract as the pos-sops-envelope-without-document corpus fixture.
	t.Run("bracket interior alone does not earn the pass", func(t *testing.T) {
		body := "password: " + encVal("Zm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFy")
		if err := BodyCheck([]byte(body)); err == nil {
			t.Error("a lone sops envelope outside any sops document must refuse")
		}
	})

	t.Run("partial encryption does not license the plaintext beside it", func(t *testing.T) {
		if documentEncrypted(partialSecretFixture) {
			t.Fatal("positive control is not positive: the superseded whole-document guard " +
				"must consider this fixture encrypted, or it proves nothing")
		}
		if !decryptedK8sSecret(partialSecretFixture) {
			t.Error("per-value rule missed a plaintext value in a partially-encrypted Secret")
		}
		if err := BodyCheck([]byte(partialSecretFixture)); err == nil {
			t.Error("BodyCheck admitted a partially-encrypted Secret carrying a plaintext value")
		}
	})

	t.Run("fully encrypted Secret still passes", func(t *testing.T) {
		if decryptedK8sSecret(encryptedSecretFixture) {
			t.Error("a Secret whose every value is ciphertext must not be refused")
		}
		if err := BodyCheck([]byte(encryptedSecretFixture)); err != nil {
			t.Errorf("sanctioned encrypted-at-rest Secret refused: %v", err)
		}
	})

	t.Run("short plaintext value is caught where entropy cannot reach", func(t *testing.T) {
		// The whole reason this is a POSITIVE detection: the value is 12 characters, so
		// the 32-char high-entropy run rule never sees it. Strip `kind: Secret` and the
		// same body passes — that is the measure of what the rule is carrying.
		if err := BodyCheck([]byte(decryptedSecretFixture)); err == nil {
			t.Error("decrypted Secret with a short base64 value was admitted")
		}
		notASecret := strings.Replace(decryptedSecretFixture, "kind: Secret", "kind: ConfigMap", 1)
		if err := BodyCheck([]byte(notASecret)); err != nil {
			t.Fatalf("control body should be clean without kind: Secret, got: %v", err)
		}
	})

	t.Run("diff-marked Secret lines are still seen", func(t *testing.T) {
		var diff strings.Builder
		for _, l := range strings.Split(strings.TrimRight(decryptedSecretFixture, "\n"), "\n") {
			diff.WriteString("+" + l + "\n")
		}
		if err := ScanSurface("branch diff vs origin/main", []byte(diff.String())); err == nil {
			t.Error("a decrypted Secret added by a diff was admitted — the branch diff is " +
				"the surface a committed Secret actually arrives on")
		}
	})
}

// TestEncryptedExemptionsDoNotWidenFalseNegatives is the measurement the house bar asks
// for whenever a change NARROWS what a secret scanner flags. #778 narrows in one place:
// material that is ENCRYPTED (a sops ENC[…] bracket interior, an age/pgp armor body) or
// PUBLIC (an age recipient) is no longer refused. The question that has to be answered
// with numbers rather than intuition is whether that narrowing gives a PLAINTEXT
// credential a new way through — i.e. whether it widens the false-negative surface.
//
// The invariant measured is containment, and it is the strongest form available here:
// for a run that BodyCheck refuses on its own, the presence of encrypted material
// anywhere in the same body must not change that verdict. This is robust against the
// scanner's pre-existing residual gaps (the isPathLike opaque-budget gap, #410) because
// it compares each run against ITSELF rather than against an absolute admit rate — a run
// already admitted in isolation is not something #778 made worse.
//
// Measured at the fixed seed below over 20,000 credential-shaped runs in each of five
// contexts (100,000 scans): 0 runs changed verdict from refused to admitted.
//
// The exemption being measured is now structuredExemptSpans' alone — #806's own
// isEncryptedMaterial/isAgeRecipient predicates were dropped when the branch merged the
// stricter landed rules, so the direct per-predicate accept counts this test used to
// assert went with them. The containment measurement below is the surviving and stronger
// statement: it exercises the exemption THROUGH BodyCheck, which is the surface that
// actually decides.
func TestEncryptedExemptionsDoNotWidenFalseNegatives(t *testing.T) {
	const trials = 20000
	rng := rand.New(rand.NewSource(778))
	randRun := func(n int) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = base64Alphabet[rng.Intn(len(base64Alphabet))]
		}
		return string(b)
	}

	contexts := []struct {
		name string
		wrap func(string) string
	}{
		{"same line as a sops ENC bracket", func(r string) string {
			return "mac: " + encVal("Zm9vYmFy") + " note=" + r
		}},
		{"document that also carries a sops metadata block", func(r string) string {
			return "sops:\n  mac: " + encVal("Zm9vYmFy") + "\n  version: 3.7.3\nleak: " + r
		}},
		{"document that also carries an age armor block", func(r string) string {
			return ageArmorFixture + "\nleak: " + r
		}},
		{"beside an age recipient", func(r string) string {
			return "recipient: " + ageRecipientFixture + "\nleak: " + r
		}},
		{"appended to a fully sops-encrypted Secret", func(r string) string {
			return encryptedSecretFixture + "leak: " + r
		}},
	}

	widened := make([]int, len(contexts))
	for i := 0; i < trials; i++ {
		// Alternate the two shapes that matter: an AWS secret access key is 40 chars,
		// and 44 is the length of a base64'd 32-byte key (the k8s/sops value shape).
		run := randRun(40)
		if i%2 == 1 {
			run = randRun(44)
		}
		if BodyCheck([]byte("value: "+run)) == nil {
			continue // admitted in isolation — a pre-existing gap, not one #778 opened
		}
		for c, ctx := range contexts {
			if BodyCheck([]byte(ctx.wrap(run))) == nil {
				widened[c]++
			}
		}
	}

	for c, ctx := range contexts {
		if widened[c] != 0 {
			t.Errorf("%s: %d/%d runs that refuse in isolation were admitted once encrypted "+
				"material was present — the #778 exemptions widened the false-negative surface",
				ctx.name, widened[c], trials)
		}
	}
}

// TestLongIdentifiersAndOneLetterWords pins the two moves that unblocked verify-desk's
// Evidence writes:
//
//   - the length cap moved OFF wordDecomposition (was maxWordSegment=48) and onto the
//     bare-identifier caller at the wider maxIdentifierRun=100, so a legitimately long Go
//     test name no longer reads as a high-entropy run just for being over 48 chars; and
//   - the one-letter English words `A` and `I`, when followed by a capital-led word, count
//     as a word unit in the CamelCase decomposition, so a name like
//     `TestDryRunResultIsOnlyReachableWithoutAWrite` decomposes instead of dying on `A`.
//
// Both directions are exercised: the real names that were refused now pass, and every
// shape the relaxation must NOT admit (ALL-CAPS, bare token material, an over-length
// CamelCase run) still refuses.
func TestLongIdentifiersAndOneLetterWords(t *testing.T) {
	// Real Go test names that the pre-fix scanner refused. Split across concatenation so no
	// contiguous 32+ char run sits on an added diff line (deskpr scans the branch diff).
	name53 := "TestLeadingZeroTagsDoNot" + "CollideWithTheirCanonicalForm" // 53 chars, was > the old 48 cap
	nameA := "TestDryRunResultIsOnly" + "ReachableWithoutAWrite"           // one-letter word `A` before `Write`
	nameI := "TestClientReconnects" + "WhenIRestartTheStream"              // one-letter word `I` before `Restart`

	// The AWS example secret key with its slashes removed — a bare 38-char token that must
	// keep refusing; it fails on its very first char run (`w`, a lone lowercase before a capital).
	awsNoSlash := "wJalrXUtnFEMI" + "K7MDENG" + "bPxRfiCYEXAMPLEKEY"
	// A 32-char lowercase md5-shaped hex with digit groups: decomposes into several
	// lowercase word units but has NO capital-led word, so camel stays false and it refuses.
	lowerHex := "dcef1701acc78ada" + "90fcefb638adeabf"
	// A CamelCase run exactly at the cap passes; one character past it refuses on the cap
	// ALONE (the decomposition itself is clean), which is what pins maxIdentifierRun.
	camelAtCap := strings.Repeat("Abcde", 20) // 100 chars — 20 clean CamelCase words
	camelOverCap := camelAtCap + "s"          // 101 chars

	pass := []struct {
		name string
		run  string
	}{
		{"53-char test name over the old 48 cap", name53},
		{"one-letter word A before a capital-led word", nameA},
		{"one-letter word I before a capital-led word", nameI},
		{"CamelCase run exactly at maxIdentifierRun", camelAtCap},
	}
	for _, c := range pass {
		t.Run("pass/"+c.name, func(t *testing.T) {
			if !isIdentifierLike(c.run) {
				t.Fatalf("isIdentifierLike(%q) = false, want true — a legitimate identifier is refused", c.run)
			}
			// And end-to-end through the real write gate, embedded in prose.
			if err := BodyCheck([]byte("see " + c.run + " for the assertion")); err != nil {
				t.Fatalf("BodyCheck rejected a body naming %q: %v", c.run, err)
			}
		})
	}

	refuse := []struct {
		name string
		run  string
	}{
		{"AWS secret key with slashes stripped", awsNoSlash},
		{"32-char lowercase hex with digit groups", lowerHex},
		{"32 capital A's (no lowercase, no follower word)", strings.Repeat("A", 32)},
		{"32 X's — an ALL-CAPS stretch", strings.Repeat("X", 32)},
		{"CamelCase run one char past maxIdentifierRun", camelOverCap},
	}
	for _, c := range refuse {
		t.Run("refuse/"+c.name, func(t *testing.T) {
			if isIdentifierLike(c.run) {
				t.Fatalf("isIdentifierLike(%q) = true, want false — token/over-length material admitted", c.run)
			}
			if err := BodyCheck([]byte(c.run)); !IsRefused(err) {
				t.Fatalf("BodyCheck(%q) = %v, want Refused (exit 5)", c.run, err)
			}
		})
	}
}

// TestIdentifierExemptionAdmitsNoRandomBase64At96Chars is the guard-strength stress test for
// the relaxed bare-identifier exemption, in the TestTwoLetterWordsCostAlmostNothing style: over
// two million uniformly random 96-char [A-Za-z0-9] runs, drawn from a FIXED seed, the number
// isIdentifierLike admits MUST stay 0. The relaxation added acceptance paths (the wider length
// cap and the lone-A/I word), and this pins that none of them opens a hole a random token run
// can walk through — a 96-char run that decomposes ENTIRELY into words is astronomically
// unlikely, and the test fails loudly if a future edit makes it merely unlikely.
func TestIdentifierExemptionAdmitsNoRandomBase64At96Chars(t *testing.T) {
	const (
		trials   = 2000000
		runLen   = 96
		alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "0123456789"
	)
	rng := rand.New(rand.NewSource(20260815))
	admitted := 0
	b := make([]byte, runLen)
	for i := 0; i < trials; i++ {
		for j := range b {
			b[j] = alphabet[rng.Intn(len(alphabet))]
		}
		if isIdentifierLike(string(b)) {
			admitted++
		}
	}
	if admitted != 0 {
		t.Fatalf("isIdentifierLike admitted %d of %d random %d-char runs — the bare-identifier "+
			"exemption has developed a hole a token run can walk through; it must admit ZERO",
			admitted, trials, runLen)
	}
}
