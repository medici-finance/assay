package deskkit

import "strings"

// The BENIGN-SHAPE REGISTRY of the shared secret scan.
//
// scanSurface refuses every run of 32 or more characters from the base64 alphabet
// (reBase64ish) unless the run is a benign shape. Before this file, "benign" was a
// hand-ordered chain of predicates in the scan loop, and every false positive was fixed
// by one more branch in it. Nothing tied a branch to the refusal it must keep, so three
// widenings had to be narrowed again after review found real-shaped material admitted.
//
// Every admission is now one declared entry below, and the loop admits a run only when an
// entry does. An entry names:
//
//   - ID       a stable id, unique in the registry;
//   - Context  what the admission reads: ctxRun (the run alone), ctxLine (the run and the
//     raw surface around it), or ctxArtifact (a span finder over the whole
//     surface; the run is admitted when one span fully contains it);
//   - Doc      one line, rendered into the high-entropy refusal message and into the
//     BodyCheck row of tools/desk/README.md (renderBenignTable);
//   - Issue    the issue or commit that added the shape;
//   - Admitted negative corpus fixtures (`neg-….txt`) or in-test cases (`case:…`) the
//     entry is what admits;
//   - Bounds   positive corpus fixtures (`pos-….txt`) or in-test cases (`case:…`) that
//     must stay refused: the refusals that bound the entry.
//
// benignshapes_test.go holds the in-test cases (generated at test time, never written as
// a literal) and proves the registry's shape: TestBenignEntriesBounded forces each entry
// open in turn and requires one of its Bounds to flip, so an entry whose widening would
// go unnoticed fails by name; TestBenignKeepsLiteralArms forces every entry open at once
// and requires every literal-arm fixture to keep its rule.
//
// What a registry entry can never do: suppress a literal credential arm. The GitHub and
// GitLab token prefixes, the AWS access-key id, plaintext PEM, JWT, a decrypted
// Kubernetes Secret and a sops block all run before the high-entropy loop, over the whole
// surface, and are not entries. The URL-userinfo arm runs after it. The run class, the
// 32-character threshold and the refusal of an unknown opaque run are not entries either.
//
// The predicates stay where they were (bodycheck.go, structured.go) as the entries'
// implementations. The closed word lists only admission predicates read live here. The
// two armor-label lists (encryptedArmorLabels, armorCiphertextOrPublicLabels) stay beside
// the literal arms that also read them.

// Contexts an entry's admission reads.
const (
	ctxRun      = "run"
	ctxLine     = "line"
	ctxArtifact = "artifact"
)

// benignShape is one declared admission of the high-entropy loop. Exactly one of run, line
// and spans is set, matching Context.
type benignShape struct {
	ID       string
	Context  string
	Doc      string
	Issue    string
	Admitted []string
	Bounds   []string

	run   func(run string) bool
	line  func(raw string, start, end int) bool
	spans func(raw string) []span
}

// benignShapes is the production registry: the one value scanSurface walks. Order does
// not change a verdict (an entry admits or it does not), so entries are grouped by context.
var benignShapes = []benignShape{
	{
		ID: "git-sha", Context: ctxRun, Issue: "4d8e66826 (first scan)",
		Doc:      "a git SHA (exactly 40 or 64 lowercase hex)",
		Admitted: []string{"case:git-sha-40", "case:git-sha-64"},
		Bounds:   []string{"case:hex-41", "case:hex-upper-40"},
		run:      isGitSHA,
	},
	{
		ID: "path-like", Context: ctxRun, Issue: "#209, #1052",
		Doc:      "a slash-separated path, absolute or repo-relative, built from word-shaped segments",
		Admitted: []string{"neg-file-line-findings.txt", "neg-diff-marker-glued-path.txt"},
		Bounds: []string{"pos-aws-secret-access-key.txt", "pos-aws-key-slash-layout-synthetic.txt",
			"pos-opaque-token-material.txt"},
		run: isPathLike,
	},
	{
		ID: "identifier", Context: ctxRun, Issue: "8d90074d3; widened by #1643, #2036, #4077",
		Doc: "a bare word-shaped CamelCase identifier (a 2-4 letter acronym, optionally plural " +
			"like PRs, a V plus 1-2 digits like V1, the numeronyms K8s, I18n, L10n, A11y and " +
			"the term OAuth count as words)",
		Admitted: []string{"neg-camelcase-test-identifiers.txt"},
		Bounds:   []string{"pos-opaque-token-material.txt", "case:mixed-case-base62"},
		run:      isIdentifierLike,
	},
	{
		ID: "assignment", Context: ctxRun, Issue: "4d8e66826 (first scan); #775",
		Doc:      "a key=VALUE assignment whose key is short or word-shaped and whose value is a git SHA, path or identifier",
		Admitted: []string{"neg-key-path-shell-assignment.txt"},
		Bounds:   []string{"case:assign-opaque-value"},
		run:      isAssignmentLike,
	},
	{
		ID: "all-equals", Context: ctxRun, Issue: "#781",
		Doc:      "an all-'=' banner separator",
		Admitted: []string{"neg-console-banner-separators.txt"},
		Bounds:   []string{"case:equals-then-payload"},
		run:      isAllEquals,
	},
	{
		ID: "issue-list", Context: ctxRun, Issue: "cea8ebb75",
		Doc:      "a slash-list of short issue numbers",
		Admitted: []string{"neg-issue-number-slash-list.txt"},
		Bounds:   []string{"pos-digit-token-wearing-slashes.txt"},
		run:      isIssueNumberList,
	},
	{
		ID: "enum-slash-list", Context: ctxRun, Issue: "#966",
		Doc:      "a slash-list of short ALL-CAPS enum words",
		Admitted: []string{"neg-enum-slash-list.txt"},
		Bounds:   []string{"pos-enum-word-too-long.txt"},
		run:      isEnumSlashList,
	},
	{
		ID: "quantifier-path", Context: ctxRun, Issue: "#883",
		Doc:      "a word-shaped path behind one leading '+' quantifier or diff marker",
		Admitted: []string{"neg-regex-quantifier-glued-path.txt"},
		Bounds:   []string{"pos-token-wearing-a-leading-plus.txt"},
		run:      isQuantifierGluedPath,
	},
	{
		ID: "go-subtest", Context: ctxRun, Issue: "#4077",
		Doc:      "a Go subtest name (TestX/sub) whose head is an exempt identifier and whose subtest path is words of at most 16 chars",
		Admitted: []string{"case:go-subtest-name"},
		Bounds:   []string{"case:subtest-token-tail"},
		run:      isGoSubtestName,
	},
	{
		ID: "diffstat-bar", Context: ctxRun, Issue: "#4077",
		Doc:      "an all-'+' diffstat bar",
		Admitted: []string{"case:diffstat-bar"},
		Bounds:   []string{"case:plus-then-payload"},
		run:      isDiffstatBar,
	},
	{
		ID: "digest-key", Context: ctxLine, Issue: "#1643",
		Doc: "a git SHA behind an ALL-CAPS env-var key ending in SHA, SHA1, SHA256, DIGEST, " +
			"CHECKSUM or COMMIT (optionally then HEX), never a credential-named variable",
		Admitted: []string{"case:digest-key-sha"},
		Bounds:   []string{"case:credential-key-sha"},
		line: func(raw string, start, end int) bool {
			return isDigestKeyAssign(raw, start, raw[start:end])
		},
	},
	{
		ID: "caps-key", Context: ctxLine, Issue: "#1642",
		Doc: "an ALL-CAPS env-var key of up to 16 letters in front of a value exempt on its own, " +
			"unless the name carries a credential stem such as TOKEN, SECRET, KEY or PASS, or a digest stem",
		Admitted: []string{"case:caps-key-test-name"},
		Bounds:   []string{"case:credential-key-ident"},
		line: func(raw string, start, end int) bool {
			return isCapsKeyAssign(raw, start, raw[start:end])
		},
	},
	{
		ID: "doc-path-hex", Context: ctxLine, Issue: "cea8ebb75",
		Doc:      "a doc-path segment of exactly 32 lowercase hex beside a doc extension or a word-shaped segment",
		Admitted: []string{"neg-doc-path-hex-segment.txt"},
		Bounds:   []string{"pos-hex-token-wearing-a-doc-path.txt"},
		line:     isDocPathHexSegment,
	},
	{
		ID: "k8s-uid", Context: ctxLine, Issue: "#966",
		Doc:      "a 32-hex run behind a recognised Kubernetes object-kind prefix and hyphen (pvc-UID and the like)",
		Admitted: []string{"neg-k8s-uid-hex-segment.txt"},
		Bounds:   []string{"pos-hex-after-unlisted-prefix.txt"},
		line:     isK8sUIDHexSegment,
	},
	{
		ID: "pgp-fingerprint", Context: ctxLine, Issue: "#750, #1161",
		Doc: "a PGP key fingerprint (40 uppercase hex after a pgp: or fp: field, or on a line " +
			"that names it with fingerprint, fpr, pgp or gpg)",
		Admitted: []string{"case:pgp-recipient"},
		Bounds:   []string{"case:upper-hex-unnamed"},
		line:     isPGPFingerprint,
	},
	{
		ID: "go-debug-list", Context: ctxLine, Issue: "#6821",
		Doc:      "a Go GODEBUG settings list (DefaultGODEBUG= or GODEBUG= then lowercase name=DIGITS items)",
		Admitted: []string{"neg-go-buildinfo-godebug.txt"},
		Bounds:   []string{"pos-godebug-token-value.txt"},
		line: func(raw string, start, _ int) bool {
			return isGoDebugList(raw, start)
		},
	},
	{
		ID: "go-sum", Context: ctxArtifact, Issue: "#778",
		Doc:      "a go.sum h1: digest (exactly 43 base64 and one pad behind the h1: marker)",
		Admitted: []string{"neg-go-sum-checksums.txt"},
		Bounds:   []string{"pos-digest-anchor-wrong-length.txt", "pos-unanchored-digest-runs.txt"},
		spans:    goSumSpans,
	},
	{
		ID: "sri-integrity", Context: ctxArtifact, Issue: "#778",
		Doc:      "an SRI integrity digest of the exact length its algorithm fixes (sha256, sha384, sha512)",
		Admitted: []string{"neg-package-lock-integrity.txt"},
		Bounds:   []string{"pos-sri-anchor-wrong-length.txt", "pos-unanchored-digest-runs.txt"},
		spans:    sriSpans,
	},
	{
		ID: "sops-document", Context: ctxArtifact, Issue: "#778",
		Doc: "the envelope payloads, metadata block and armor bodies of a complete encrypted sops " +
			"document, and only when the whole surface is one",
		Admitted: []string{"neg-sops-encrypted-manifest.txt"},
		Bounds:   []string{"pos-unanchored-digest-runs.txt", "pos-opaque-token-material.txt"},
		spans:    sopsDocumentSpans,
	},
}

// The closed word lists the admission predicates read. Each is a deliberate, measured
// list rather than a shape rule; see each list's own comment for what it costs.

// numeronyms are the closed list of letter-digit-letter abbreviations an identifier may
// carry as ONE CamelCase word unit (#1642): `…Under|K8s|Is…`. The lone capital and the
// short lowercase tail would otherwise each sink the run. A closed list, like
// twoLetterWords, rather than a shape rule: a general "capital, digits, letter" unit is
// common in random base62, an exact four-entry list is not.
var numeronyms = []string{"K8s", "I18n", "L10n", "A11y"}

// mixedCaseWords are the closed list of established mixed-case terms an identifier may
// carry as ONE CamelCase word unit, on the same terms as numeronyms (identifier callers
// only, must end the run or be followed by a capital). `OAuth` opens with a lone capital
// followed by another capital-led unit, which wordDecomposition otherwise admits only for
// the English words `A` and `I`; a `TestOAuth…` name therefore refused as a whole. A
// closed one-entry list, not an "any lone capital" relaxation: a lone `O` (or any other
// letter) before a capital-led word stays debris, so random base62 gains nothing but the
// exact five-letter spelling (TestOAuthWordControls, TestNewRulesCostOnRandom).
var mixedCaseWords = []string{"OAuth"}

// digestKeys are the closed list of env-var name TAILS isDigestKeyAssign will strip in
// front of a git-SHA-shaped value (#1642). A closed list, not an ALL-CAPS shape rule: a
// shape rule admits `TOKEN=<64 hex>`, and a 64-hex value is exactly what `openssl rand -hex
// 32` prints, so the key is the only thing telling a digest from a credential (#1643 review).
// `HEX` is not on the list: it names an encoding, not a digest, so it is admitted only
// directly behind one of these (`…_DIGEST_HEX`, `…_SHA256_HEX`).
var digestKeys = []string{"SHA", "SHA1", "SHA256", "DIGEST", "CHECKSUM", "COMMIT"}

// credentialStems refuse the digest-key strip when any of them appears ANYWHERE in the
// full env-var name, so `SIGNING_SECRET_SHA=` or `ENCRYPTION_KEY_DIGEST_HEX=` never pass on
// their digest-looking tail. This is a second fence behind digestKeys, not a substitute:
// a stem nobody listed still has to get past the closed tail list and the SHA-only value.
var credentialStems = []string{
	"SECRET", "TOKEN", "PASS", "PWD", "KEY", "AUTH", "CRED", "PRIV", "SESSION",
	"COOKIE", "SIGN", "HMAC", "SALT", "API", "CERT", "BEARER", "JWT", "SEED",
}

// twoLetterWords are the two-letter units that count as WORDS in a CamelCase
// decomposition even though they carry only one lowercase letter after the capital
// (see wordDecomposition's capital branch, which otherwise requires two).
//
// The membership rule is deliberately a CLOSED LIST OF ENGLISH WORDS rather than the
// broader "a capital followed by any single lowercase letter", and the difference is
// measurable rather than stylistic. Both spellings clear the reported false positives
// equally well, so the only thing that can choose between them is what each costs on real
// token material: against 2,000,000 uniformly random 32-char base64 runs the blanket
// relaxation exempts 35, while this list exempts 0 — the same as the stricter rule it
// replaces. The list buys back the whole false-positive class without measurably loosening
// the scanner's grip. TestTwoLetterWordsCostAlmostNothing pins that comparison against a
// fixed seed, so a future widening to "any capital plus one lowercase" — or a list grown
// long enough to amount to the same thing — cannot be waved through on intuition.
//
// The list is grounded in this repo's own identifiers, not guessed: the units that
// actually block real long CamelCase names here are, in frequency order, Is (124),
// No (48), On (41), At, By, In, To, As, An, Of, Up, Do, Or, It, So, Me, Be — every one an
// English function word. The debris further down that same frequency table (Ym, Jl, Vh,
// Zm, Qx, Px, Xo) comes from base64 test fixtures, and an English-word list leaves all of
// it on the opaque budget where it belongs.
//
// `Id` and `Ok` are the only two non-words admitted: both are universal Go identifier
// idioms (`UserIdIsStable`, `ReportsOkOnEmptyInput`) and both are English-lexicon enough
// to be indistinguishable in cost from the rest of the list. Do NOT extend this to
// general abbreviations (`Db`, `Ui`, `Io`, `Pr`, `Cd`, `Tx`) — each addition is another
// face on the die that random base64 gets to roll, and the frequency table above shows
// they buy back almost nothing: one or two identifiers each.
var twoLetterWords = map[string]bool{
	"Am": true, "An": true, "As": true, "At": true, "Be": true, "By": true,
	"Do": true, "Go": true, "He": true, "If": true, "In": true, "Is": true,
	"It": true, "Me": true, "My": true, "No": true, "Of": true, "On": true,
	"Or": true, "So": true, "To": true, "Up": true, "Us": true, "Vs": true,
	"We": true, "Id": true, "Ok": true,
}

// goTestPrefixes are the function-name prefixes `go test` discovers. A subtest name is
// always `<TopLevel>/<sub>[/<sub>…]`, and the top level carries one of these.
var goTestPrefixes = []string{"Test", "Benchmark", "Fuzz", "Example"}

// k8sUIDPrefixes are the short Kubernetes object-KIND prefixes isK8sUIDHexSegment's
// anchor recognises immediately before the hyphen — a CLOSED list, the same discipline
// twoLetterWords uses for the same reason (see its comment): an anchor that admitted "any
// lowercase word before a hyphen" would just as happily launder a real secret pasted as
// `token-<32hex>` or `key-<32hex>`, so the list is grounded in Kubernetes' OWN
// generated-name convention rather than widened to anything word-shaped. `pvc`/`pv` are
// #966's own repro (a PersistentVolumeClaim's bound PersistentVolume); the rest are the
// other object kinds whose default name generator suffixes a hash or a UID onto a base
// name the same way.
var k8sUIDPrefixes = map[string]bool{
	"pvc": true, "pv": true, "pod": true, "rs": true, "rc": true,
	"ds": true, "sts": true, "job": true, "cm": true, "svc": true,
}

// godebugHeads are the only keys Rule 9 reads a GODEBUG settings list behind: the
// build-info row `go version -m` prints for a binary's compiled-in defaults, and the
// environment variable the Go runtime reads the same list from.
var godebugHeads = []string{"DefaultGODEBUG", "GODEBUG"}

// benignWalk is the registry prepared for one scanned surface: each artifact entry's spans
// are found ONCE, before the loop, so a recognised field is judged by its format rather
// than by the run's own shape.
type benignWalk struct {
	reg   []benignShape
	spans [][]span
}

func newBenignWalk(reg []benignShape, raw string) benignWalk {
	w := benignWalk{reg: reg, spans: make([][]span, len(reg))}
	for i, e := range reg {
		if e.spans != nil {
			w.spans[i] = e.spans(raw)
		}
	}
	return w
}

// admitting returns the index of the first entry that admits the run raw[lo:hi], or
// -1 and false when none does and the run must be refused.
func (w benignWalk) admitting(raw string, lo, hi int) (int, bool) {
	for i, e := range w.reg {
		if e.run != nil && e.run(raw[lo:hi]) {
			return i, true
		}
		if e.line != nil && e.line(raw, lo, hi) {
			return i, true
		}
		if e.spans != nil && coveredBy(w.spans[i], lo, hi) {
			return i, true
		}
	}
	return -1, false
}

// benignRefusalText is the exemption half of the high-entropy refusal message, rendered
// from the registry's Doc lines so the message never drifts from what the scan admits.
func benignRefusalText(reg []benignShape) string {
	docs := make([]string, len(reg))
	for i, e := range reg {
		docs[i] = e.Doc
	}
	return "only these shapes are exempt: " + strings.Join(docs, "; ") +
		" (pass --explain to see the refusing rule and line)"
}

// renderBenignTable is the registry as the BodyCheck row of tools/desk/README.md carries
// it, between the benign-shapes markers: one `<br>`-separated line per entry, since the
// row is itself a table cell. TestBenignDocsParity fails when the README and this differ.
func renderBenignTable(reg []benignShape) string {
	var b strings.Builder
	for _, e := range reg {
		b.WriteString("<br>`" + e.ID + "` (" + e.Context + "; " + e.Issue + "): " + e.Doc)
	}
	return b.String()
}
