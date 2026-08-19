package deskkit

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// corpusleak_test.go — THE CLASS-CLOSER for withheld `docs/streams` paths in the
// tools/desk COPY SET.
//
// WHAT IT GUARDS. Since the 2026-08-10 publication-manifest pivot the WHOLE
// `docs/streams/` tree is `do-not-copy` (docs/publication-manifest.yaml carries one
// `docs/streams/` row: kind:tree, do-not-copy, "Supersedes every per-stream copy
// disposition"). A real stream path baked into a SHIPPING tools/desk file therefore
// publishes a map to withheld material — the exact class #1316's line-scrubbing
// failed to converge on (three Security-Review FAILs). Neutralising the literals is
// only half the fix; without a guard the next publish silently re-introduces the
// class. This test is the durable half: it greps the tools/desk copy set against the
// LIVE `docs/streams` + `docs/archive` directory listing and goes red the moment a
// real withheld-stream path reappears in a copy-flipped file.
//
// WHY DERIVED, NOT HAND-LISTED. The forbidden set is read from the tree at run time,
// so a stream added tomorrow is covered without editing this file — a hand list rots
// the moment a new stream lands and nobody remembers to add it.
//
// THE DO-NOT-TOUCH EXEMPTIONS, EACH WITH AN OWNER (an unexplained exemption is
// indistinguishable from a blind spot):
//
//   - corpusOperationalStreams — the stream roots the PUBLISHED tools READ/WRITE at
//     runtime (ScanDir="docs/streams/issue-loop", the intake/findings register roots,
//     the issue-flow rulings register). Genericising them BREAKS the tool, so they
//     are the operational contract an adopter recreates — never withheld, never
//     flagged. Same spirit as ScanDir.
//   - corpusCopySetExempt — the tools/desk files this guard need not scan because they
//     do NOT ship under #1316's copy flip. Keyed on #1316's copy set, NOT current
//     main's manifest (see its own doc): the flip removes the per-file `do-not-copy`
//     overrides for citrigger/s2sweep/README/etc, so those files SHIP and MUST be
//     scanned — and the exempt set shrinks to this guard's own file alone.
//
// THREE STATES, NOT TWO (the s2sweep lesson). A grep that finds nothing because the
// grep itself is broken must never read as clean. Once the copy set is neutralized no
// shipping file carries a real withheld path, so the control cannot lean on the copy
// files as carriers; it runs the SAME walker+reader+matcher over a DURABLE OPERATIONAL
// needle (docs/streams/issue-loop in scanbody.go) and requires it be found — if it is
// not, the instrument is broken and the guard reports could-not-check, a FAILURE, not a
// pass. See TestCorpusGuardMatcherIsLive.

// corpusRepoRoot: this package is tools/desk/internal/deskkit.
const corpusRepoRoot = "../../../.."

// corpusScanTree is the copy set this guard walks, relative to the repo root.
const corpusScanTree = "tools/desk"

// corpusSelfPath is THIS file. It derives its forbidden set from the tree rather than
// hard-coding any real stream name, so it carries no withheld path — but it is exempt
// anyway so a future edit that mentions one cannot make the guard flag itself.
const corpusSelfPath = "tools/desk/internal/deskkit/corpusleak_test.go"

// corpusControlWitness / corpusControlWitnessFile are the three-state control's needle
// (see TestCorpusGuardMatcherIsLive). After neutralization NO shipping file carries a
// real WITHHELD stream path — that is the point — so the "the matcher can find a real
// path" control can no longer lean on citrigger/s2sweep. It instead points at a DURABLE
// OPERATIONAL path the published tool reads at runtime: ScanDir = docs/streams/issue-loop,
// hard-coded in scanbody.go. Operational means never withheld (so never a gate hit) and
// never removable (so the control cannot rot). Anchoring to a file OTHER than this guard
// keeps the control from being its own witness (the s2sweep self-witness trap).
const corpusControlWitness = "docs/streams/issue-loop"
const corpusControlWitnessFile = "tools/desk/internal/deskkit/scanbody.go"

// corpusOperationalStreams: docs/streams roots the published tools read/write live.
// See the file header. NOT withheld, NOT flagged.
var corpusOperationalStreams = map[string]string{
	"issue-loop": "ScanDir / issueLoopDir — deskscanbody + issueboard read it live",
	"intake":     "intakeDir / deskpushguard register root",
	"findings":   "deskpushguard register root",
	"issue-flow": "rulings.md register — deskclose/deskmerge/deskdigest read it live",
}

// corpusCopySetExempt is the set of tools/desk files this guard does NOT scan
// because they do NOT ship under the disposition this guard protects — #1316's flip
// of the `tools/desk/` tree to `copy`. Under that flip the per-file `do-not-copy`
// overrides for s2sweep_test.go, citrigger_test.go, treesweep_pipefail_test.go,
// skillrepolist_test.go, topology/example_test.go and README.md are REMOVED, so they
// resolve to the tree default `copy` and SHIP — they must be SCANNED, not exempted.
// The exempt set therefore reflects #1316's copy set, NOT current main's manifest:
// it shrinks to this guard's own file alone.
//
// Anchoring to #1316's copy set (not current main's) is the whole point: on this
// branch the current manifest still carries those overrides, but a guard keyed on it
// would exempt exactly the files that flip to copy under the PR this guard is the
// prerequisite for, and so would read clean while citrigger/s2sweep/README ship real
// withheld paths — the defect this revision fixes. TestCorpusExemptionsAreLive keeps
// the one entry honest.
var corpusCopySetExempt = map[string]string{
	corpusSelfPath: "this guard — derives its forbidden set from the tree, hard-codes no real stream name; " +
		"exempt so a future edit that mentions one cannot make the guard flag itself",
}

// corpusWithheldStreamPrefixes reads the LIVE docs/streams and docs/archive directory
// listing and returns the repo-relative path prefixes that are withheld — every real
// stream directory EXCEPT the operational ones. Derived from the tree so a stream
// added later is covered the moment it lands.
func corpusWithheldStreamPrefixes(root string) ([]string, error) {
	var prefixes []string
	for _, base := range []string{"docs/streams", "docs/archive"} {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(base)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if base == "docs/streams" {
				if _, operational := corpusOperationalStreams[name]; operational {
					continue
				}
			}
			prefixes = append(prefixes, base+"/"+name)
		}
	}
	sort.Strings(prefixes)
	return prefixes, nil
}

// corpusIsNameByte reports whether b can continue a docs-path directory segment.
// Used for the boundary check below.
func corpusIsNameByte(b byte) bool {
	return b == '-' || b == '_' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// corpusContainsPrefix reports whether body references prefix as a docs path — the
// prefix followed by a path/quote/space boundary, NOT a longer identifier. This is
// why "docs/streams/withheld" does not fire on "docs/streams/withheldx": a substring
// match alone would flag a longer sibling stream and read a clean tree dirty.
func corpusContainsPrefix(body, prefix string) bool {
	from := 0
	for {
		i := strings.Index(body[from:], prefix)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(prefix)
		if end >= len(body) || !corpusIsNameByte(body[end]) {
			return true
		}
		from = i + 1
	}
}

// corpusIsIdentByte reports whether b is an identifier byte — a letter, digit, or
// underscore. NOTE this excludes '-' on purpose: unlike corpusIsNameByte (the
// full-path segment check), the BARE-SLUG leading boundary below must ADMIT a leading
// hyphen, because the collapsed anchor form joins the slug to the preceding heading
// words with '-' (`...-envelope-check-withheld-arch07`). What it must still reject is a
// slug that is the tail of a longer WORD (`backwithheld-arch07` for `withheld-arch`),
// where the char before the slug is a letter/digit/underscore.
func corpusIsIdentByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// corpusContainsBareSlug reports whether body references a withheld stream slug in the
// BARE brief-id form the full-path matcher above is BLIND to:
//
//   - `<slug>/NN`  — the bare brief-id (`withheld-stream/01`), no `docs/streams/` prefix; and
//   - `<slug>NN`   — the COLLAPSED anchor form GitHub derives from a `(<slug>/NN)` heading
//     suffix by stripping the slash (`withheld-arch/07` -> the anchor `...withheld-arch07`).
//
// Both were proven to slip past corpusContainsPrefix (which only fires on a literal
// `docs/streams/<slug>` / `docs/archive/<slug>` path) — the exact gap that let
// `withheld-stream/01` (citrigger_test.go) and `withheld-arch07` (README anchor) ship past
// CI while #1333 declared the class "closed corpus-wide".
//
// WORD-BOUNDING, both sides, is what keeps this from crying wolf:
//   - LEADING: start-of-body, or the preceding byte is NOT an identifier byte
//     (letter/digit/underscore). A leading '-' or '/' or space IS admitted (the anchor
//     and path cases), but a leading letter is not — so `backwithheld-arch07` does not
//     match `withheld-arch`, and `mywithheld-stream/01` does not match `withheld-stream`.
//   - TRAILING: the slug is immediately followed by a brief NUMBER — a digit (collapsed
//     form), or a '/' then a digit (`/NN` form). Requiring the digit distinguishes a
//     brief-id reference from an ordinary mention of the slug word in prose and from a
//     longer sibling slug (`withheld-streamer` never matches — 'e' is not a digit and not
//     '/'). But the digit run must NOT be followed by a '.', which would make it a
//     DECIMAL/SEMVER, not a brief number: one shipped tool's release-artifact prefix
//     coincides with a withheld stream slug, so that tool's release tag `<slug>/0.1.3`
//     (and any `<slug>/vX.Y.Z`, where the char after '/' is 'v', not a digit) is a
//     version — a public release ref — not a brief-id under that stream, and is NOT a
//     leak.
//
// SCOPE. The caller passes ONLY the withheld slug set — the live `docs/streams/*` +
// `docs/archive/*` directory names MINUS corpusOperationalStreams — so operational
// refs (`issue-loop/07`) never enter, and synthetic slugs an author may plant to
// preserve a test's meaning (`example-stream/02`, `attr`, `x`, `fixture`) are absent
// from the tree and therefore never forbidden.
func corpusContainsBareSlug(body, slug string) bool {
	from := 0
	for {
		i := strings.Index(body[from:], slug)
		if i < 0 {
			return false
		}
		i += from
		// LEADING boundary — reject a slug glued to the tail of a longer word.
		if i > 0 && corpusIsIdentByte(body[i-1]) {
			from = i + 1
			continue
		}
		end := i + len(slug)
		// Locate the first digit of a candidate brief number — immediately after the slug
		// (collapsed anchor `<slug>NN`) or after a single '/' (`<slug>/NN`).
		d := -1
		if end < len(body) && body[end] >= '0' && body[end] <= '9' {
			d = end
		} else if end+1 < len(body) && body[end] == '/' && body[end+1] >= '0' && body[end+1] <= '9' {
			d = end + 1
		}
		if d >= 0 {
			// Consume the digit run; a brief number is not a decimal — a run followed by
			// '.' is a version/semver (e.g. a shipped tool's release tag `<slug>/0.1.3`), not a brief-id.
			j := d
			for j < len(body) && body[j] >= '0' && body[j] <= '9' {
				j++
			}
			if j >= len(body) || body[j] != '.' {
				return true
			}
		}
		from = i + 1
	}
}

// corpusSlugsFromPrefixes reduces the withheld path prefixes (`docs/streams/<name>`,
// `docs/archive/<name>`) to their bare directory-name slugs, deduped. A slug present
// under both bases (a `docs/streams/x` and a `docs/archive/x`) collapses to one entry —
// the bare-slug matcher is base-agnostic. Operational streams never appear here because
// corpusWithheldStreamPrefixes already dropped them.
func corpusSlugsFromPrefixes(prefixes []string) []string {
	seen := map[string]struct{}{}
	var slugs []string
	for _, p := range prefixes {
		slug := p
		if idx := strings.LastIndexByte(p, '/'); idx >= 0 {
			slug = p[idx+1:]
		}
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	return slugs
}

// corpusScan walks corpusScanTree under root and returns every (file -> prefix) hit
// where a non-exempt file references a withheld stream prefix. The forbidden set is
// DERIVED from the live tree. scanned counts the text files actually read (a zero-file
// scan certifies nothing).
func corpusScan(root string, exempt map[string]string) (hits []string, forbidden []string, scanned int, err error) {
	forbidden, err = corpusWithheldStreamPrefixes(root)
	if err != nil {
		return nil, nil, 0, err
	}
	slugs := corpusSlugsFromPrefixes(forbidden)
	hits, scanned, err = corpusScanWith(root, exempt, forbidden, slugs)
	return hits, forbidden, scanned, err
}

// corpusScanWith is the shared walker: it walks corpusScanTree under root and returns
// every hit where a non-exempt file references one of the GIVEN forbidden path prefixes
// (via corpusContainsPrefix) OR one of the GIVEN bare withheld slugs (via
// corpusContainsBareSlug — the `<slug>/NN` and collapsed `<slug>NN` anchor forms).
// corpusScan calls it with the tree-derived withheld prefixes AND their bare slugs (the
// gate); the three-state control calls it with a single known-present operational prefix
// and nil slugs, so the walker, the reader, and corpusContainsPrefix are all exercised
// on the REAL tree by the identical code path that must return clean for the gate — a
// matcher that could certify a tree it never opened is the failure this indirection
// rules out.
func corpusScanWith(root string, exempt map[string]string, forbidden []string, slugs []string) (hits []string, scanned int, err error) {
	tree := filepath.Join(root, filepath.FromSlash(corpusScanTree))
	err = filepath.WalkDir(tree, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if _, ok := exempt[rel]; ok {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if bytes.IndexByte(raw, 0) >= 0 {
			return nil // binary; the copy-level gate owns binaries
		}
		scanned++
		body := string(raw)
		for _, p := range forbidden {
			if corpusContainsPrefix(body, p) {
				hits = append(hits, rel+" -> "+p)
			}
		}
		for _, s := range slugs {
			if corpusContainsBareSlug(body, s) {
				hits = append(hits, rel+" -> "+s+" (bare withheld-slug brief-id/anchor form)")
			}
		}
		return nil
	})
	sort.Strings(hits)
	return hits, scanned, err
}

// TestCorpusHasNoWithheldStreamPaths is THE GATE. The tools/desk copy set, with the
// do-not-touch exemptions applied, must reference no real withheld-stream path.
func TestCorpusHasNoWithheldStreamPaths(t *testing.T) {
	skipIfFixtureAbsent(t, filepath.Join(corpusRepoRoot, "docs", "streams"),
		"docs/streams (the withheld-stream corpus) is not part of this repository's published file set")
	hits, forbidden, scanned, err := corpusScan(corpusRepoRoot, corpusCopySetExempt)
	if err != nil {
		t.Fatalf("corpus scan could not walk the tree: %v — this is could-not-check, NOT clean", err)
	}
	if len(forbidden) == 0 {
		t.Fatal("the withheld-stream set is empty — re-point corpusRepoRoot; a guard with nothing forbidden certifies nothing")
	}
	if scanned == 0 {
		t.Fatal("the corpus scan read 0 files — re-point corpusRepoRoot; a zero-file scan certifies nothing")
	}
	if len(hits) > 0 {
		t.Fatalf("the tools/desk copy set references %d real withheld-stream path(s):\n  %s\n"+
			"A shipping file naming a `docs/streams`/`docs/archive` path that is do-not-copy publishes a "+
			"map to withheld material (the #1316 class). Neutralise the path to a synthetic `example-stream/…` "+
			"slug preserving the test's meaning; if the file is genuinely do-not-copy per the manifest, add it "+
			"to corpusCopySetExempt with its reason; if the tool READS the path at runtime, add its root to "+
			"corpusOperationalStreams with why.",
			len(hits), strings.Join(hits, "\n  "))
	}
}

// TestCorpusGuardMatcherIsLive is the control — the s2sweep three-state discipline. A
// grep that finds nothing because the grep itself is broken must never read as clean,
// so the SAME walker+reader+matcher the gate runs must find a needle KNOWN to be in the
// tree. After neutralization no shipping file carries a real WITHHELD stream path (that
// is precisely the state the gate certifies), so the control can no longer lean on
// citrigger/s2sweep as its carriers. It points instead at a DURABLE OPERATIONAL path —
// docs/streams/issue-loop, the ScanDir hard-coded in scanbody.go, read at runtime and
// therefore never withheld (never a gate hit) and never removable (so the control never
// rots). corpusScanWith runs it through the identical code path as the gate, with the
// real copy-set exemptions applied, so a walker that never opens scanbody.go, a reader
// that returns nothing, or a corpusContainsPrefix that never matches all surface here as
// could-not-check rather than as a clean gate.
func TestCorpusGuardMatcherIsLive(t *testing.T) {
	hits, scanned, err := corpusScanWith(corpusRepoRoot, corpusCopySetExempt, []string{corpusControlWitness}, nil)
	if err != nil {
		t.Fatalf("corpus control scan could not walk the tree: %v", err)
	}
	if scanned == 0 {
		t.Fatal("the control scan read 0 files — re-point corpusRepoRoot")
	}
	var sawWitness bool
	for _, h := range hits {
		if strings.HasPrefix(h, corpusControlWitnessFile+" ->") {
			sawWitness = true
			break
		}
	}
	if !sawWitness {
		t.Fatalf("control: the operational witness %q was not found in %q by the real scan (hits: %v).\n"+
			"The walker, the reader, or corpusContainsPrefix is broken — the gate's clean result would then be "+
			"could-not-check, not a pass. If ScanDir moved, re-point corpusControlWitness/File.",
			corpusControlWitness, corpusControlWitnessFile, hits)
	}
}

// TestCorpusGuardGoesRedOnAPlantedPath is the mutation proof, over a synthetic tree so
// it never depends on repo content. A guard that has never been observed FAILING has
// not been tested. It also binds the boundary check: a longer sibling name must NOT be
// flagged.
func TestCorpusGuardGoesRedOnAPlantedPath(t *testing.T) {
	root := t.TempDir()
	// A real withheld stream to forbid, and its operational sibling that must be spared.
	mkdir(t, root, "docs/streams/withheld-stream")
	mkdir(t, root, "docs/streams/issue-loop")    // operational — never forbidden
	mkdir(t, root, "docs/archive/withheld-arch") // archive is also swept
	// Register issue-loop as operational for this synthetic run.
	prefixes, err := corpusWithheldStreamPrefixes(root)
	if err != nil {
		t.Fatal(err)
	}
	// issue-loop is a global operational root, so it must be absent from the forbidden set.
	for _, p := range prefixes {
		if p == "docs/streams/issue-loop" {
			t.Fatal("operational stream issue-loop leaked into the forbidden set")
		}
	}

	tree := filepath.Join(root, "tools/desk/internal/deskkit")
	if err := os.MkdirAll(tree, 0o700); err != nil {
		t.Fatal(err)
	}
	// CLEAN first: only the operational path and a longer-sibling near-miss.
	writeFile(t, filepath.Join(tree, "clean.go"),
		"const a = \"docs/streams/issue-loop\"\n// note: docs/streams/withheld-streamX is a different name\n")
	cleanHits, _, scanned, err := corpusScan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if scanned == 0 {
		t.Fatal("planted-tree scan read 0 files")
	}
	if len(cleanHits) != 0 {
		t.Fatalf("baseline is not clean: %v — the operational exemption or the boundary check is wrong "+
			"(a longer sibling `withheld-streamX` must not match `withheld-stream`)", cleanHits)
	}

	// Now plant the real withheld path in a copy file.
	writeFile(t, filepath.Join(tree, "planted_test.go"),
		"var x = \"docs/streams/withheld-stream/brief-01-x.md\"\n"+
			"var y = \"docs/archive/withheld-arch/note.md\"\n")
	hits, _, _, err := corpusScan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"tools/desk/internal/deskkit/planted_test.go -> docs/archive/withheld-arch",
		"tools/desk/internal/deskkit/planted_test.go -> docs/streams/withheld-stream",
	}
	if len(hits) != len(want) {
		t.Fatalf("hits = %v, want exactly %v — a guard that cannot go red, or names the wrong files, is not a check", hits, want)
	}
	for i := range want {
		if hits[i] != want[i] {
			t.Fatalf("hits = %v, want exactly %v", hits, want)
		}
	}

	// And the exemption must suppress it.
	exempt := map[string]string{"tools/desk/internal/deskkit/planted_test.go": "test"}
	exHits, _, _, err := corpusScan(root, exempt)
	if err != nil {
		t.Fatal(err)
	}
	if len(exHits) != 0 {
		t.Fatalf("with the file exempt the scan still reports %v — the exemption is not honoured", exHits)
	}
}

// TestCorpusGuardGoesRedOnBareSlugAndCollapsedAnchor is the mutation proof for the
// bare-slug hardening — the forms the prefix-only matcher was blind to and that shipped
// past CI (withheld-stream/01 in citrigger_test.go; the withheld-arch07 README anchor). It
// proves fail-first over a synthetic tree, and — just as load-bearing — proves the
// word-bounding does NOT cry wolf on the operational, synthetic, and longer-word cases.
func TestCorpusGuardGoesRedOnBareSlugAndCollapsedAnchor(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, "docs/streams/withheld-stream") // withheld — bare-brief-id carrier
	mkdir(t, root, "docs/archive/withheld-arch")   // withheld (archive) — collapsed-anchor carrier
	mkdir(t, root, "docs/streams/issue-loop")      // operational — never forbidden
	// NOTE: example-stream is deliberately absent — a synthetic slug an author plants to
	// preserve a test's meaning is not a real dir, so it never enters the withheld set.

	tree := filepath.Join(root, "tools/desk/internal/deskkit")
	if err := os.MkdirAll(tree, 0o700); err != nil {
		t.Fatal(err)
	}

	// CLEAN baseline — every false-positive the coordinator called out must stay green:
	//   - operational bare ref  issue-loop/07      (operational slug, not in the set)
	//   - synthetic bare ref     example-stream/02  (not a real dir, not in the set)
	//   - longer-WORD near-miss  backwithheld-arch07 (leading letter 'k' — not withheld-arch)
	//   - longer-SIBLING /NN     withheld-streamer/12  (trailing 'e' — not withheld-stream)
	//   - a bare prose mention   withheld-stream       (no trailing number)
	writeFile(t, filepath.Join(tree, "clean.go"),
		"// operational issue-loop/07 stays green; synthetic example-stream/02 stays green.\n"+
			"// longer word backwithheld-arch07 must not match withheld-arch; withheld-streamer/12 must not\n"+
			"// match withheld-stream; and a bare withheld-stream mention with no number stays green.\n")
	cleanHits, _, scanned, err := corpusScan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if scanned == 0 {
		t.Fatal("bare-slug planted-tree scan read 0 files")
	}
	if len(cleanHits) != 0 {
		t.Fatalf("baseline is not clean: %v — the word-bounding or the withheld-vs-operational scoping is "+
			"wrong. Operational (issue-loop/07), synthetic (example-stream/02), a longer word "+
			"(backwithheld-arch07) and a longer sibling (withheld-streamer/12) must all stay green.", cleanHits)
	}

	// Now plant the two forms the prefix matcher is blind to: the bare `<slug>/NN` brief-id
	// and the collapsed `<slug>NN` anchor GitHub derives from a `(<slug>/NN)` heading suffix.
	writeFile(t, filepath.Join(tree, "planted_bare_test.go"),
		"// registered by withheld-stream/07 in the lower registry\n"+
			"// see [x](#deskroster-preflight--the-operating-envelope-check-withheld-arch07)\n")
	hits, _, _, err := corpusScan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"tools/desk/internal/deskkit/planted_bare_test.go -> withheld-arch (bare withheld-slug brief-id/anchor form)",
		"tools/desk/internal/deskkit/planted_bare_test.go -> withheld-stream (bare withheld-slug brief-id/anchor form)",
	}
	if len(hits) != len(want) {
		t.Fatalf("hits = %v, want exactly %v — the bare `<slug>/NN` and collapsed `<slug>NN` forms must both go red", hits, want)
	}
	for i := range want {
		if hits[i] != want[i] {
			t.Fatalf("hits = %v, want exactly %v", hits, want)
		}
	}
}

// TestCorpusExemptionsAreLive keeps both do-not-touch lists honest — the same rot the
// s2sweep exclusion-liveness test guards against. An exemption naming a path that no
// longer exists silently widens the blind spot.
func TestCorpusExemptionsAreLive(t *testing.T) {
	for rel, why := range corpusCopySetExempt {
		if why == "" {
			t.Errorf("exempt file %q carries no reason — an unexplained exemption is a blind spot", rel)
		}
		if _, err := os.Stat(filepath.Join(corpusRepoRoot, filepath.FromSlash(rel))); err != nil {
			t.Errorf("stale exemption: %s no longer exists (%v). Delete the row or fix the path — a "+
				"suppression for a path that is gone is how an exemption list turns into a blanket one.", rel, err)
		}
	}
	// The operational streams are CODE-LEVEL roots (ScanDir, the register roots the
	// guards read) — some, like `intake`, need not exist as a directory in every
	// checkout (deskpushguard creates the register entry on demand). So this only
	// checks each carries a reason; a missing directory is not a rot signal here, and
	// an operational name absent from the live listing simply never enters the
	// forbidden set anyway.
	for name, why := range corpusOperationalStreams {
		if why == "" {
			t.Errorf("operational stream %q carries no reason — an unexplained exemption is a blind spot", name)
		}
	}
}

// mkdir is a t.Helper for the planted-tree test.
func mkdir(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o700); err != nil {
		t.Fatal(err)
	}
}

// writeFile is a t.Helper for the planted-tree test.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
