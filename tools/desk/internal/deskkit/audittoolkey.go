package deskkit

// audittoolkey.go — the canonical tool-key roster behind the audit log's per-tool
// accounting (the rate-limit budget, the circuit breaker, and the audit trail).
//
// THE DEFECT THIS CLOSES. `audit.jsonl` is load-bearing state: the rate-limit counter
// (ratelimit.go) counts only the audit entries whose `tool` field equals the caller's
// key, and the audit trail is grouped by that same field. But the key was not resolved
// the same way everywhere. The outward-write path passes a compiled-in CONSTANT
// ("deskpost", "deskrelease", …), while `guard()` keyed its audit line off the running
// binary's BASENAME (`filepath.Base(os.Args[0])`). A binary invoked under any other file
// name — a test build (`deskpost.test`), a locally built copy (`deskpr-bin`, `deskpr-322`),
// a renamed or `go run` binary — therefore wrote audit lines under a DIFFERENT key than
// its own budget counted, and that variant key was a fresh, uncounted bucket:
//
//   - the per-tool WRITE BUDGET was escaped — a renamed/copied/test-built binary got a
//     full budget the canonical tool's meter could never see;
//   - the AUDIT TRAIL was silently split — "what did deskpost do this hour" became an
//     incomplete question, because the variant spellings answered to nobody.
//
// THE FIX. One canonical key per tool, resolved the SAME way wherever the audit log is
// read or written, and NEVER derived from `os.Args[0]` for budget purposes:
//
//   - CanonicalToolKey collapses a variant spelling of a known tool to that tool's one
//     canonical key. Counting (pointsFor) compares canonical forms, so variant lines of
//     the same tool now share ONE budget instead of each getting its own. Log records
//     the canonical key, so a guard line written under a basename lands in the same bucket
//     the write path counts.
//   - A key that resolves to NO known tool is a loud Unverifiable at the write gate, not a
//     silent new bucket (see RequireCanonicalToolKey). Failing closed there is the whole
//     point: the escape was silent, and the replacement must be noisy — an unregistered
//     binary asking the budget gate for admission is refused and named, never quietly
//     granted a private budget.
//
// COMPILED IN, LIKE THE LOOP-NAME ROSTER (loopnames.go). The roster is a compiled-in set,
// never a runtime file, for the same reason: there must be no `tools.env` whose corruption
// could refuse every tool on the machine at once. Adding a tool is a PR; that is the
// intended price, and audittoolkey_test.go diffs this roster against the cmd/ directory so
// a new binary that forgets to register here goes red rather than silently earning an
// Unverifiable at its first write.

import (
	"sort"
	"strconv"
	"strings"
)

// canonicalToolKeys is the set of canonical desk-tool audit/budget keys. Each is the file
// name of a binary under tools/desk/cmd/ (the name that binary passes to AllowWrite* and
// records on its audit lines), plus the small number of SYNTHETIC keys that are audit tools
// without a binary of their own.
//
// A canonical key is matched EXACTLY first (so a synthetic multi-token key such as
// VerdictIssueTool keeps its own bucket), then as a delimited token of a variant spelling.
var canonicalToolKeys = map[string]struct{}{
	// tools/desk/cmd/* — one entry per binary.
	"clusterguard":      {},
	"deskack":           {},
	"commsgw":           {},
	"commsloop":         {},
	"deskadvisory":      {},
	"deskaudit":         {},
	"deskavatar":        {},
	"deskboard":         {},
	"deskboot":          {},
	"deskclaim":         {},
	"deskclaim-ref":     {},
	"deskclose":         {},
	"deskcomms":         {},
	"deskdigest":        {},
	"deskdispatch":      {},
	"deskdisable":       {},
	"deskdisposition":   {},
	"deskevidence":      {},
	"deskfile":          {},
	"deskflip":          {},
	"deskgit":           {},
	"deskinstall":       {},
	"deskmanifest":      {},
	"deskmerge":         {},
	"deskmigrate":       {},
	"deskpins":          {},
	"deskpost":          {},
	"deskpr":            {},
	"deskpreflight":     {},
	"deskpushguard":     {},
	"deskrelease":       {},
	"deskreply":         {},
	"deskroster":        {},
	"deskscanbody":      {},
	"deskscanuntrusted": {},
	"desksourceguard":   {},
	"desksupervise":     {},
	"desktoken":         {},
	"deskverdict":       {},
	"deskversion":       {},
	"deskwt":            {},
	"fanoutloop":        {},
	"issueboard":        {},
	"muhar":             {},
	"opmetrics":         {},
	"repohardenguard":   {},
	"reviewloop":        {},
	"scanloop":          {},
	"untrustcorpus":     {},
	"upgrade-assay":     {},
	"verifyloop":        {},
	"writeguard":        {},

	// Synthetic keys — audit/budget tools without a binary of their own. These MUST be
	// matched exactly (never as a token of a variant), because the whole point of a
	// synthetic key is that it is a DISTINCT bucket from the tool it derives from:
	// VerdictIssueTool ("verifyloop-verdict") is metered separately from "verifyloop" on
	// purpose (see VerdictIssueTool in ratelimit.go), so it must never collapse into it.
	VerdictIssueTool: {},
}

// CanonicalToolKey resolves raw — a compiled-in tool constant, a binary basename, or a
// variant spelling of either — to the ONE canonical audit/budget key for that tool.
//
// known=false means raw resolves to no registered tool. The caller MUST decide what that
// means for its context and must NOT invent a fresh bucket for it: the write gate treats it
// as could-not-attribute (Unverifiable, exit 6 — see RequireCanonicalToolKey); the counting
// and recording paths treat an already-written unknown key as its own opaque bucket rather
// than failing a whole read closed (a single unattributable historical line must not brick
// every desk tool — that is the desk-wide-DoS shape this file's sibling recovery path,
// RecoverCorruptAudit, exists to avoid).
//
// Resolution order, most specific first:
//
//  1. EXACT match against a canonical key — a caller's constant, a correctly named binary,
//     or a synthetic key (VerdictIssueTool) all land here and are returned unchanged.
//  2. DELIMITED-TOKEN match — split raw on any non-[a-z0-9] byte and, if exactly one
//     distinct canonical key appears as a whole token, return it. This collapses
//     "deskpost.test", "vfy713-deskpost", "deskpr-322", "deskpr-bin", "deskboard-v4test",
//     "vd-dt07-deskreply" onto their tools. Whole-token matching is why "deskpreflight"
//     resolves to itself and NOT to "deskpr" (there "deskpr" is a substring, not a token).
//     Two DIFFERENT canonical tokens in one string is ambiguous and returns unknown rather
//     than guessing.
func CanonicalToolKey(raw string) (canonical string, known bool) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return "", false
	}
	if _, ok := canonicalToolKeys[key]; ok {
		return key, true
	}
	lower := strings.ToLower(key)
	if _, ok := canonicalToolKeys[lower]; ok {
		return lower, true
	}
	// Delimited-token match. A token is a maximal run of [a-z0-9] in the lowercased key.
	seen := map[string]struct{}{}
	for _, tok := range strings.FieldsFunc(lower, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		if _, ok := canonicalToolKeys[tok]; ok {
			seen[tok] = struct{}{}
		}
	}
	if len(seen) == 1 {
		for tok := range seen {
			return tok, true
		}
	}
	return "", false
}

// CanonicalToolKeyOr resolves raw to its canonical key, or returns raw unchanged when raw
// resolves to no known tool. It is the form the audit READ and RECORD paths use, where an
// unattributable key must remain its own opaque bucket rather than fail a whole file closed.
// The write GATE uses RequireCanonicalToolKey instead, which fails closed.
func CanonicalToolKeyOr(raw string) string {
	if c, ok := CanonicalToolKey(raw); ok {
		return c
	}
	return raw
}

// RequireCanonicalToolKey resolves raw to its canonical key or returns an Unverifiable
// error naming the offending key and the known set. It is the write-budget gate's resolver:
// a key that attributes to no known tool is refused loudly (exit 6) rather than being handed
// a private, uncounted budget — the silent escape this whole file replaces.
func RequireCanonicalToolKey(raw string) (string, error) {
	if c, ok := CanonicalToolKey(raw); ok {
		return c, nil
	}
	return "", Unverifiable(
		"audit tool key "+strconv.Quote(raw)+" resolves to no known desk tool, so its outward-write "+
			"budget cannot be attributed — a variant or renamed binary must not run the write gate under an "+
			"unregistered key (register the tool in audittoolkey.go, or invoke the canonical binary). Known tools: "+
			strings.Join(KnownToolKeys(), ", "), nil)
}

// KnownToolKeys returns every registered canonical tool key, sorted — for the refusal
// message above and for the drift test.
func KnownToolKeys() []string {
	out := make([]string, 0, len(canonicalToolKeys))
	for k := range canonicalToolKeys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
