package deskkit

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"
)

// THE S2 DISCLOSURE SWEEP.
//
// WHAT IT GUARDS. The desk pipeline files findings about PUBLIC-repo PRs into a private
// review channel precisely so outside readers cannot see them. Before this sweep a
// tree could ship that channel's repo name — compiled into allowedRepos, in the
// adopter-facing desk README, in the App setup guide, in test files, and inside streams
// the #389 ruling classified PUBLIC. Publishing any one of them tells a reader exactly
// where to look for the material that was withheld, which converts the channel from a
// mitigation into a pointer. This sweep is what keeps the token from coming back.
//
// SCOPE. The branch-level gate covers THE BRANCH; a separate copy-level gate covers THE
// COPY, over a staged tree and a whole token list. This file is deliberately the narrow
// half: one token, this repo, every commit. The copy-level gate supersedes nothing here
// — it reads a different tree.
//
// WHY THE TOKEN IS ASSEMBLED AND NOT WRITTEN. A sweep that carries its own forbidden
// literal flags itself, and the usual repair — excluding the sweep from its own search — is
// worse than the disease: the exclusion ships the token in a file nobody is checking. So the
// token is joined at run time from fragments that are individually meaningless. This is not
// obfuscation for its own sake; it is what lets the sweep search the WHOLE tree, itself
// included, with no exclusion carved for its own benefit.
//
// THREE STATES, NOT TWO. A search that finds nothing
// because the search itself is broken must never read as clean. Every run therefore also
// looks for a CONTROL token known to be present in the same tree, through the same walker,
// the same reader and the same matcher. Control absent ⇒ could-not-check, which is a
// FAILURE, not a pass. The measured precedent this rule comes from:
// `git grep -lIE '\bAda\b'` returns 0 files while `git grep -lIw 'Ada'` returns many — git's
// ERE engine does not implement \b, so the pattern matched nothing and read as clean.

// s2ForbiddenToken is the token this self-test forbids from the tree, assembled at run
// time (see the note above). In the OSS bundle it is a NEUTRAL placeholder for "a private
// review-channel slug": the real channel name is never in this source to guard, and the
// branch-level disclosure gate owns the deployment-specific token. The
// assembly-from-fragments mechanism is kept intact so the sweep can still search the whole
// tree, itself included, with no self-exclusion.
func s2ForbiddenToken() string { return "example" + "-" + "withheld-channel" }

// s2ControlToken is a token KNOWN present in this tree, searched by the same code path as
// the forbidden one. It is the instrument check: if this stops being found, the sweep is
// broken and reports could-not-check rather than clean.
//
// Chosen deliberately: `allowedRepos` is the write-authorisation set, so any change large
// enough to move it is a change a human is already reading. A control
// that can vanish quietly is not a control.
const s2ControlToken = "allowedRepos"

// s2SweepSelfPath is THIS file, and the control tally deliberately ignores it.
//
// MEASURED, not theorised. The first cut of this file counted control hits everywhere, and
// the row-9 mutation — replace the control with a token the tree does not carry, expect
// could-not-check — came back GREEN at exit 0. The reason is that the sweep walks its own
// source: the `const s2ControlToken = "…"` line put the literal into the tree, so whatever
// the control was set to, the sweep found it exactly once and pronounced itself live. An
// instrument may not be its own witness; the control has to be a claim about the REST of the
// tree, or breaking it cannot be observed.
//
// Ignoring the file is the robust repair rather than assembling the control from fragments
// the way s2ForbiddenToken is: fragments only hold while every future editor remembers to
// use them, whereas this exclusion holds however the constant is written.
const s2SweepSelfPath = "tools/desk/internal/deskkit/s2sweep_test.go"

// s2SweepExclusions are the OPERATIONAL paths routed AWAY rather than scrubbed —
// generated views, third-party registries, the manifest that must name what it
// withholds. Every one is a decision with an owner, restated here because an
// unexplained exclusion is indistinguishable from a blind spot.
// TestS2SweepExclusionsAreLive keeps them honest: an exclusion naming a path that no
// longer exists is deleted, not left to widen silently.
//
// The WITHHELD-STREAM exclusions (paths whose content names the private channel by
// construction — a private stream whose own README justifies the withholding) are
// NOT hard-coded here: naming them in
// this shipping file is the very leak the sweep prevents once tools/desk copies to
// public (#1316). They come from the house at run time via
// EnvSweepWithheldStreams (see sweepconfig.go) and are merged in by
// s2SweepExclusionSet below. Unset — the public/adopter case, where docs/streams is
// not published at all — yields no extra exclusions.
var s2SweepExclusions = map[string]string{
	// The token appears in DELIBERATE-ABSENCE commentary here — it explains why the channel
	// is EXCLUDED from the scan surface. A separate copy-level scrub rewrites that
	// commentary to a neutral form; stripping it here would contradict authored
	// commentary and collide with it.
	"statusgen/scanissues.go":      "routed to the copy-level scrub",
	"statusgen/scanissues_test.go": "routed to the copy-level scrub",
	"tools/desk/cmd/issueboard":    "routed to the copy-level scrub",

	// docs/research/ has no publication classification at all — #245 records that gap
	// itself. The classification step assigns one; do not infer it here.
	"docs/research": "unclassified — the classification step assigns the disposition",

	// (A private stream whose own README names the channel to justify withholding it
	// is routed away too, but its path is house-supplied via EnvSweepWithheldStreams
	// rather than hard-coded here; see the var doc above and s2SweepExclusionSet below.)

	// GENERATED, single writer is main's CI (status-regen.yml). A branch must never modify
	// it, so the token cannot be removed here at all: it arrives from a brief title and
	// leaves when main regenerates. The regeneration step owns it.
	"STATUS.md": "generated; single writer is main CI — the regeneration step owns the regen",

	// The withheld-items index (docs/publication-manifest.yaml). Landed on
	// main after this sweep did, from a stream this PR never touched. Its own `reason:`
	// column for THIS path states the identical logic as the publication-stream exclusion
	// above: "the `reason:` column exists to justify withholding, which means it describes
	// the withheld material" — a manifest that classifies every path in the repo has to name
	// the channel to explain why paths naming it are do-not-copy. Same shape, different file.
	"docs/publication-manifest.yaml": "withheld-items index — must name the channel to justify withholding it",

	// The copy-level withheld-token registry (tools/leaksweep). Its own header says it
	// is do-not-copy per publication-manifest.yaml, for the identical reason as the manifest
	// exclusion above: a list that defines the token has to name it to withhold it.
	"docs/leak-sweep-tokens.yaml": "the copy-level withheld-token registry — must name the channel to define the token it withholds; do-not-copy",

	// The copy-level deliberately-dirty leaksweep test fixture. Its own file header says
	// "DO NOT SCRUB" — every token in leak-sweep-tokens.yaml is planted here on purpose so the
	// per-token dirty-tree test proves the sweep goes red on each one individually.
	"tools/leaksweep/testdata/dirty/leaks.md": "leaksweep's own deliberately-dirty fixture — file header says DO NOT SCRUB",

	// GENERATED, single writer is main's CI (release-daily-harvest.yml). A branch must never
	// hand-edit these snapshots; a harvested issue/PR title can legitimately name the channel
	// (e.g. an issue ABOUT the token) and the fix belongs in whatever authored the source
	// issue/PR, not in the harvest that faithfully recorded it. Prefix match covers every
	// dated snapshot directory, not just the ones present today.
	"reports/daily": "generated daily-harvest snapshots; single writer is main CI (release-daily-harvest.yml)",
}

// s2SweepExclusionSet returns the full routed-away set for a run: the hard-coded
// OPERATIONAL exclusions above merged with the house-configured WITHHELD-STREAM
// exclusions (EnvSweepWithheldStreams, see sweepconfig.go). The withheld-stream
// paths are supplied by the deployment at run time rather than compiled in, so this
// shipping file names none of them — the leak #1316 must not reintroduce. A base
// entry always wins a key collision (its reason is the more specific one).
func s2SweepExclusionSet() map[string]string {
	m := make(map[string]string, len(s2SweepExclusions)+4)
	for k, v := range s2SweepExclusions {
		m[k] = v
	}
	for _, p := range SweepWithheldStreamExclusions() {
		if _, ok := m[p]; !ok {
			m[p] = "house-configured withheld stream (ASSAY_SWEEP_WITHHELD_STREAMS): its content names " +
				"the private channel by construction, so the sweep routes past it rather than flag it"
		}
	}
	return m
}

// sweep states.
const (
	sweepCheckedClean  = "checked-clean"
	sweepCheckedFailed = "checked-failed"
	sweepCouldNotCheck = "could-not-check"
)

type sweepReport struct {
	state       string
	hits        []string // repo-relative paths carrying the forbidden token
	controlHits int      // files carrying the control token
	scanned     int      // text files actually read
	skipped     int      // files skipped as binary
}

// s2DecodeUTF16 turns a UTF-16 byte stream (BOM stripped by the caller) into UTF-8 text so the
// forbidden-token match runs on the same string it would over a plain UTF-8 file.
//
// WHY THIS EXISTS (review at PR #505, adversarial pass). The NUL-byte binary heuristic below
// exists to skip real binaries — but a UTF-16 text file is ALSO NUL-heavy (every ASCII
// codepoint carries a zero high or low byte), so the heuristic misclassifies it as binary and
// the content match never runs. A file that is legitimate, readable prose — a Windows-authored
// note, a BOM-prefixed export — would carry the token past the sweep with no read at all. The
// gate that was supposed to exclude binaries was excluding a text encoding instead.
func s2DecodeUTF16(raw []byte, bigEndian bool) string {
	if len(raw)%2 != 0 {
		raw = raw[:len(raw)-1] // truncated trailing byte; best-effort, never a crash
	}
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		if bigEndian {
			units = append(units, uint16(raw[i])<<8|uint16(raw[i+1]))
		} else {
			units = append(units, uint16(raw[i+1])<<8|uint16(raw[i]))
		}
	}
	return string(utf16.Decode(units))
}

// s2Sweep walks root and classifies it. It reads every regular file except .git and the
// exclusions, decodes UTF-16 (BOM-prefixed, either endianness) rather than treating it as
// binary, skips anything ELSE holding a NUL byte (git grep -I semantics; other binary
// artifacts are the copy-level gate's problem, over the staged tree), and matches by literal
// substring. A symlink's TARGET string is matched the same way its name is — see the inline
// comment at the symlink branch for why.
//
// Literal matching is not a shortcut, it is the lesson: git grep's ERE engine silently
// accepts \b and matches nothing, so the natural "word boundary" pattern reports a dirty
// tree clean. There is no pattern language here to get wrong.
//
// controlIgnore is a repo-relative path whose control hits do not count. See
// s2SweepSelfPath for why that is not an optimisation but the difference between a control
// and a tautology.
func s2Sweep(root, forbidden, control string, exclusions map[string]string, controlIgnore string) (sweepReport, error) {
	var rep sweepReport
	lowerForbidden := strings.ToLower(forbidden)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			// .git, node_modules, and vendor are excluded by NAME rather than in
			// s2SweepExclusions on purpose: they are never-authored third-party trees that
			// may not exist in this repo today (neither node_modules/ nor vendor/ does, as
			// of this writing) but would appear anywhere a future dependency lands, so a
			// path-keyed map entry would fail TestS2SweepExclusionsAreLive's liveness check
			// the moment the directory is absent — the opposite of what that check is for.
			// This is a structural exemption from the reasoned-map rule, not an oversight.
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			if _, excluded := exclusions[rel]; excluded {
				return filepath.SkipDir
			}
		} else if _, excluded := exclusions[rel]; excluded {
			return nil
		}

		// Match the path itself, case-insensitively, BEFORE any content read and before the
		// regular/binary gates below — not after them (review at PR #505, finding 2). A
		// directory listing or a search index shows the name without opening the file, and
		// three shapes never reach a content read at all: a binary file (skipped on its NUL
		// byte), a symlink (its own content is never read; its TARGET gets a separate check
		// below, at the symlink branch), and an empty directory (no content to read, ever).
		// Testing the name only for text files that were successfully read misses exactly
		// those three, so the name is tested here, for every entry, first.
		hitByName := strings.Contains(strings.ToLower(rel), lowerForbidden)
		if hitByName {
			rep.hits = append(rep.hits, rel)
		}

		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			// A symlink is never IsRegular, so nothing below this branch ever reads its
			// content — by design, following it would mean either escaping root (a target
			// outside the swept tree) or risking a cycle. But the link's TARGET is itself
			// text checked into this tree (git stores it as the blob content of the link),
			// and neither the name check above nor a content read ever inspects it. A
			// symlink pointed at the withheld channel — name innocuous, target is not — was
			// invisible to every existing gate (review at PR #505, adversarial pass).
			if target, lerr := os.Readlink(path); lerr == nil && !hitByName &&
				strings.Contains(strings.ToLower(target), lowerForbidden) {
				rep.hits = append(rep.hits, rel)
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		var body string
		switch {
		case len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE:
			body = s2DecodeUTF16(raw[2:], false)
			rep.scanned++
		case len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF:
			body = s2DecodeUTF16(raw[2:], true)
			rep.scanned++
		case bytes.IndexByte(raw, 0) >= 0:
			rep.skipped++
			return nil
		default:
			body = string(raw)
			rep.scanned++
		}
		// Match content, case-insensitively: `strings.Contains` alone would pass a
		// same-token mention with either word capitalised. Only the forbidden-token side is
		// case-folded — the control stays exact-match, a known compiled identifier with no
		// case-variant hazard. Guarded on !hitByName so a file whose name AND body both
		// carry the token is recorded once, not twice.
		if !hitByName && strings.Contains(strings.ToLower(body), lowerForbidden) {
			rep.hits = append(rep.hits, rel)
		}
		if rel != controlIgnore && strings.Contains(body, control) {
			rep.controlHits++
		}
		return nil
	})
	if err != nil {
		return rep, err
	}
	sort.Strings(rep.hits)
	switch {
	case rep.scanned == 0 || rep.controlHits == 0:
		rep.state = sweepCouldNotCheck
	case len(rep.hits) > 0:
		rep.state = sweepCheckedFailed
	default:
		rep.state = sweepCheckedClean
	}
	return rep, nil
}

// s2RepoRoot: this package is tools/desk/internal/deskkit.
const s2RepoRoot = "../../../.."

// TestS2DisclosureSweepIsClean is the gate. It runs over the WHOLE repository, this file
// included, and goes red the moment the token reappears anywhere that is not routed away.
func TestS2DisclosureSweepIsClean(t *testing.T) {
	rep, err := s2Sweep(s2RepoRoot, s2ForbiddenToken(), s2ControlToken, s2SweepExclusionSet(), s2SweepSelfPath)
	if err != nil {
		t.Fatalf("sweep could not walk the tree: %v — this is could-not-check, NOT clean", err)
	}
	if rep.state == sweepCouldNotCheck {
		t.Fatalf("sweep reports %s (scanned=%d, control hits=%d): the instrument is broken, so "+
			"its silence proves nothing. Fix the sweep before reading its result.",
			rep.state, rep.scanned, rep.controlHits)
	}
	if rep.state != sweepCheckedClean {
		t.Fatalf("sweep reports %s — the S2 disclosure is back in %d file(s):\n  %s\n"+
			"Publishing the channel's name tells a reader exactly where the withheld findings "+
			"are. Replace it with a neutral form that names no repo (\"a private review "+
			"channel configured by the operator\"), or route the file in s2SweepExclusions "+
			"with the brief that owns it.",
			rep.state, len(rep.hits), strings.Join(rep.hits, "\n  "))
	}
}

// TestS2SweepControlIsLive is row 2's shape as a test: the exact search that returns zero
// above must be shown to return NON-zero for a token known present in the same tree, read
// through the same walker. Without this, a sweep that reads nothing at all passes.
func TestS2SweepControlIsLive(t *testing.T) {
	rep, err := s2Sweep(s2RepoRoot, s2ForbiddenToken(), s2ControlToken, s2SweepExclusionSet(), s2SweepSelfPath)
	if err != nil {
		t.Fatalf("sweep could not walk the tree: %v", err)
	}
	if rep.scanned == 0 {
		t.Fatal("the sweep read 0 files — re-point s2RepoRoot; a zero-file sweep certifies nothing")
	}
	if rep.controlHits == 0 {
		t.Fatalf("control token %q was not found in %d scanned files. The sweep's zero is "+
			"could-not-check, not checked-clean — re-point the control at a token this tree "+
			"really carries.", s2ControlToken, rep.scanned)
	}
}

// TestS2SweepGoesRedOnAPlantedToken is the mutation proof. A
// sweep that has never been observed FAILING has not been tested — the failure mode this
// whole file exists to prevent is a check that cannot go red.
func TestS2SweepGoesRedOnAPlantedToken(t *testing.T) {
	dir := t.TempDir()
	// A control-bearing file, so the tree is checkable at all.
	if err := os.WriteFile(filepath.Join(dir, "config.go"), []byte("var "+s2ControlToken+" = map[string]int{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	clean, err := s2Sweep(dir, s2ForbiddenToken(), s2ControlToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if clean.state != sweepCheckedClean {
		t.Fatalf("baseline fixture reports %s, want %s — the mutation below would prove nothing "+
			"against a fixture that was already red", clean.state, sweepCheckedClean)
	}

	// Now plant the disclosure, in the shape it really took: a compiled-in map entry.
	planted := filepath.Join(dir, "planted.go")
	body := "\t\"medici-finance/" + s2ForbiddenToken() + "\": {CIRequired: false},\n"
	if err := os.WriteFile(planted, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	// A second plant: the token is present in the BODY only through casing — the filename
	// is clean. Binds the case-fold half of the match (review at PR #505, finding 1): drop
	// the case-fold and this file's body-match silently stops firing while the suite still
	// passes on `planted.go` alone.
	//
	// Built from fragments, not a contiguous literal — same discipline as s2ForbiddenToken
	// (see the file header): a literal case-varied token here would itself trip
	// TestS2DisclosureSweepIsClean against this very file.
	casedToken := "Example" + "-" + "Withheld-Channel"
	cased := filepath.Join(dir, "cased.txt")
	casedBody := "See " + casedToken + " for the writeup.\n"
	if err := os.WriteFile(cased, []byte(casedBody), 0o600); err != nil {
		t.Fatal(err)
	}

	// A third plant: the token is present in the NAME only — the body is clean. Binds the
	// path-match half (review at PR #505, finding 1): drop the `rel` check and this file
	// becomes invisible while the suite still passes on `planted.go` and `cased.txt` alone.
	namedFile := "notes-" + s2ForbiddenToken() + "-plan.txt"
	named := filepath.Join(dir, namedFile)
	if err := os.WriteFile(named, []byte("nothing sensitive here\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := s2Sweep(dir, s2ForbiddenToken(), s2ControlToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.state != sweepCheckedFailed {
		t.Fatalf("sweep on a tree carrying the token reports %s, want %s — a sweep that cannot "+
			"go red is not a check", got.state, sweepCheckedFailed)
	}
	wantHits := []string{"cased.txt", namedFile, "planted.go"}
	sort.Strings(wantHits)
	if len(got.hits) != len(wantHits) {
		t.Fatalf("hits = %v, want exactly %v — the sweep must NAME every file, or the red "+
			"tells nobody what to fix", got.hits, wantHits)
	}
	for i, h := range got.hits {
		if h != wantHits[i] {
			t.Fatalf("hits = %v, want exactly %v", got.hits, wantHits)
		}
	}
}

// TestS2SweepCatchesTokenInNonTextPaths is the mutation proof for review finding 2 (PR
// #505): a name match performed only after the binary/non-regular/read gates is unreachable
// for exactly the shapes where a directory listing discloses the name without opening the
// file — a binary blob, a symlink, and an empty directory. None of these carry content a
// text match could ever read, so if the name check sits below those gates it never runs for
// any of the three.
func TestS2SweepCatchesTokenInNonTextPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.go"), []byte("var "+s2ControlToken+" = map[string]int{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	forbidden := s2ForbiddenToken()

	// A binary file NAMED with a CAPITALISED variant of the token: content is a NUL byte, so
	// it is skipped as binary before any text match — the name is all this file discloses.
	// The capitalisation is deliberate, not decorative (review at PR #505, round 4, finding
	// 1): every other named plant in this suite uses the token in its natural lowercase, so
	// none of them separates "the name is matched at all" from "the name is matched
	// case-insensitively" — dropping the `strings.ToLower` on the name side leaves the whole
	// suite green. This is the one plant that binds it. Assembled from fragments, not a
	// contiguous literal — same discipline as s2ForbiddenToken (see the file header).
	binName := "Example" + "-" + "Withheld-Channel" + ".bin"
	if err := os.WriteFile(filepath.Join(dir, binName), []byte{0x00, 0x01, 0x02}, 0o600); err != nil {
		t.Fatal(err)
	}

	// A symlink NAMED with the token: not a regular file, so its content is never read.
	linkName := forbidden + ".link"
	linkPath := filepath.Join(dir, linkName)
	if err := os.Symlink(filepath.Join(dir, "config.go"), linkPath); err != nil {
		t.Skipf("symlink not supported on this platform: %v", err)
	}

	// An empty directory NAMED with the token: has no content to read, ever.
	dirName := forbidden + "-archive"
	if err := os.Mkdir(filepath.Join(dir, dirName), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := s2Sweep(dir, forbidden, s2ControlToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.state != sweepCheckedFailed {
		t.Fatalf("state = %s, want %s — none of a binary file, a symlink, or an empty "+
			"directory named with the token were caught", got.state, sweepCheckedFailed)
	}
	want := map[string]bool{dirName: false, binName: false, linkName: false}
	for _, h := range got.hits {
		if _, ok := want[h]; ok {
			want[h] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("%s carries the token in its name and was not reported as a hit", name)
		}
	}
}

// TestS2SweepWithABrokenControlIsCouldNotCheck is the three-state property.
// Break the instrument and the answer must be could-not-check — never clean.
// This is the row that separates this sweep from a checklist.
func TestS2SweepWithABrokenControlIsCouldNotCheck(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "clean.md"), []byte("nothing to see here\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// The tree is genuinely clean of the forbidden token …
	withControl, err := s2Sweep(dir, s2ForbiddenToken(), "nothing to see here", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if withControl.state != sweepCheckedClean {
		t.Fatalf("with a control that IS present the same tree reports %s, want %s — without "+
			"this half, the assertion below passes for the wrong reason",
			withControl.state, sweepCheckedClean)
	}

	// … but with a control the tree does not carry, the sweep must refuse to certify it.
	broken, err := s2Sweep(dir, s2ForbiddenToken(), "a-token-this-tree-does-not-carry", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if broken.state == sweepCheckedClean {
		t.Fatal("a sweep whose control token was not found reported checked-clean. A broken " +
			"instrument must read could-not-check; reporting clean is how a zero-hit search " +
			"certifies a dirty tree.")
	}
	if broken.state != sweepCouldNotCheck {
		t.Fatalf("state = %s, want %s", broken.state, sweepCouldNotCheck)
	}
}

// TestS2SweepMissingTreeIsCouldNotCheck — the likeliest real invocation error. A path that
// does not exist must never certify.
func TestS2SweepMissingTreeIsCouldNotCheck(t *testing.T) {
	rep, err := s2Sweep(filepath.Join(t.TempDir(), "no-such-tree"), s2ForbiddenToken(), s2ControlToken, nil, "")
	if err == nil && rep.state == sweepCheckedClean {
		t.Fatal("sweeping a nonexistent tree reported checked-clean")
	}
	if err == nil && rep.state != sweepCouldNotCheck {
		t.Fatalf("state = %s, want an error or %s", rep.state, sweepCouldNotCheck)
	}
}

// TestS2SweepExclusionsAreLive keeps the routed-away list honest. An exclusion whose path is
// gone is dead weight that quietly widens the blind spot the next time a file lands under
// that name — the same rot ciRegistryOptOut guards against for the CI-trigger registry.
func TestS2SweepExclusionsAreLive(t *testing.T) {
	skipIfFixtureAbsent(t, filepath.Join(s2RepoRoot, "docs", "leak-sweep-tokens.yaml"),
		"docs/leak-sweep-tokens.yaml and the swept house tree are not part of this repository's published file set")
	// The self path is not an exclusion, but it rots the same way: if this file is renamed
	// and the constant is not, the control silently starts counting this file again and the
	// row-9 mutation goes green against a broken instrument. That is the exact regression
	// this constant was introduced to fix, so it is pinned here rather than trusted.
	if _, err := os.Stat(filepath.Join(s2RepoRoot, filepath.FromSlash(s2SweepSelfPath))); err != nil {
		t.Errorf("s2SweepSelfPath = %q does not exist (%v). Re-point it at this file: while it "+
			"is wrong the control tally includes this file's own literal and cannot be broken, "+
			"so could-not-check becomes unreachable.", s2SweepSelfPath, err)
	}
	for rel, why := range s2SweepExclusionSet() {
		if why == "" {
			t.Errorf("exclusion %q carries no reason — an unexplained exclusion is a blind spot", rel)
		}
		if _, err := os.Stat(filepath.Join(s2RepoRoot, filepath.FromSlash(rel))); err != nil {
			t.Errorf("stale exclusion: %s no longer exists (%v). Delete the row — a suppression "+
				"for a path that is gone is how a routed-away list turns into a blanket one.\n"+
				"Its reason was: %s", rel, err, why)
		}
	}
}

// TestS2SweepStillFindsTheTokenInTheRoutedAwayTree is the strongest control available: it
// runs the sweep over the REAL tree with the exclusions lifted and requires it to go red.
//
// It proves two things at once that a synthetic fixture cannot. First, that the walker
// reaches those directories at all — a matcher can be perfect and still certify a tree it
// never opened. Second, that the exclusions in s2SweepExclusions are doing real work rather
// than describing an already-empty set, which is the state in which a routed-away list looks
// identical whether or not anyone ever honoured it.
//
// If this goes red, the routed-away files were scrubbed elsewhere (a copy-level scrub is
// the likely author). That is good news, not a defect — retire this control and re-point it,
// deliberately, rather than deleting it.
func TestS2SweepStillFindsTheTokenInTheRoutedAwayTree(t *testing.T) {
	// OSS re-point (as this control's doc always anticipated): the forbidden token is now a
	// NEUTRAL placeholder that is never planted in the real tree, so the original form —
	// sweep the real tree with exclusions lifted and require RED — has nothing to find. The
	// property it still must prove is that lifting an exclusion makes the walker REACH the
	// path it named, so an exclusion can never quietly blind the sweep. Proved here over a
	// synthetic tree that plants the token under a would-be-excluded relative path.
	// The path is a SYNTHETIC slug built here in a t.TempDir(), not a real stream: it
	// never touches the repo and names no withheld stream, so it is safe in this
	// shipping file. The property under test is the walker's REACH, which is
	// independent of the specific relative path chosen.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.go"),
		[]byte("var "+s2ControlToken+" = map[string]int{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	routedRel := "docs/streams/example-stream"
	routed := filepath.Join(dir, filepath.FromSlash(routedRel))
	if err := os.MkdirAll(routed, 0o700); err != nil {
		t.Fatal(err)
	}
	body := "the withheld channel is " + s2ForbiddenToken() + "\n"
	if err := os.WriteFile(filepath.Join(routed, "brief.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	// WITH the path excluded, the sweep is clean — the token lives ONLY under it.
	excl := map[string]string{routedRel: "routed away — never copied"}
	clean, err := s2Sweep(dir, s2ForbiddenToken(), s2ControlToken, excl, "")
	if err != nil {
		t.Fatal(err)
	}
	if clean.state != sweepCheckedClean {
		t.Fatalf("with %s excluded the sweep reports %s, want %s — the exclusion is not suppressing "+
			"the only planted hit", routedRel, clean.state, sweepCheckedClean)
	}

	// With every exclusion lifted the walker MUST reach it and go red — proving the
	// exclusion, not a blind walker, is what suppressed the hit above.
	rep, err := s2Sweep(dir, s2ForbiddenToken(), s2ControlToken, nil, "")
	if err != nil {
		t.Fatalf("sweep could not walk the tree: %v", err)
	}
	if rep.state != sweepCheckedFailed {
		t.Fatalf("with every exclusion lifted the sweep reports %s, want %s — the walker never "+
			"reached the routed-away path, so an exclusion could silently blind the sweep",
			rep.state, sweepCheckedFailed)
	}
	if len(rep.hits) != 1 || rep.hits[0] != routedRel+"/brief.md" {
		t.Fatalf("hits = %v, want exactly [%s/brief.md]", rep.hits, routedRel)
	}
}

// TestS2SweepCatchesUTF16ContentOnly is the mutation proof for the UTF-16 gap found by the
// PR #505 adversarial pass: the NUL-byte binary heuristic misclassifies UTF-16 text as
// binary (every ASCII codepoint carries a zero byte), so a legitimately-encoded text file —
// a Windows-authored note, a BOM-prefixed export — could carry the token past the sweep with
// the content never read at all. The file's NAME carries no trace of the token, so this
// isolates the content path specifically.
func TestS2SweepCatchesUTF16ContentOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.go"), []byte("var "+s2ControlToken+" = map[string]int{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Built from fragments, not a contiguous literal — same discipline as s2ForbiddenToken
	// (see the file header).
	token := s2ForbiddenToken()

	baseline, err := s2Sweep(dir, s2ForbiddenToken(), s2ControlToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if baseline.state != sweepCheckedClean {
		t.Fatalf("baseline fixture reports %s, want %s", baseline.state, sweepCheckedClean)
	}

	// UTF-16LE, BOM-prefixed: "see " + token + " for detail\n" encoded two bytes per rune.
	text := "see " + token + " for detail\n"
	var le []byte
	le = append(le, 0xFF, 0xFE)
	for _, r := range text {
		le = append(le, byte(r), 0x00)
	}
	if err := os.WriteFile(filepath.Join(dir, "utf16le.txt"), le, 0o600); err != nil {
		t.Fatal(err)
	}

	// UTF-16BE, BOM-prefixed — the other endianness, so both branches are exercised.
	var be []byte
	be = append(be, 0xFE, 0xFF)
	for _, r := range text {
		be = append(be, 0x00, byte(r))
	}
	if err := os.WriteFile(filepath.Join(dir, "utf16be.txt"), be, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := s2Sweep(dir, s2ForbiddenToken(), s2ControlToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.state != sweepCheckedFailed {
		t.Fatalf("state = %s, want %s — a UTF-16 file carrying the token in its content only "+
			"was not caught (scanned=%d skipped=%d hits=%v)", got.state, sweepCheckedFailed,
			got.scanned, got.skipped, got.hits)
	}
	want := map[string]bool{"utf16le.txt": false, "utf16be.txt": false}
	for _, h := range got.hits {
		if _, ok := want[h]; ok {
			want[h] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("%s carries the token in UTF-16 content and was not reported as a hit", name)
		}
	}
}

// TestS2SweepCatchesSymlinkTargetToken is the mutation proof for the symlink-target gap
// found by the PR #505 adversarial pass: a symlink's own content is never read (by design —
// following it risks escaping root or a cycle), but its TARGET string is data checked into
// this tree the same way its name is, and nothing tested that until now. The target lives
// OUTSIDE the swept root so the walker never independently discovers it by any other path —
// isolating whether the sweep inspects os.Readlink at all.
func TestS2SweepCatchesSymlinkTargetToken(t *testing.T) {
	token := s2ForbiddenToken()

	outside := t.TempDir()
	target := filepath.Join(outside, token+"-secret.txt")
	if err := os.WriteFile(target, []byte("withheld content, unrelated wording\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.go"), []byte("var "+s2ControlToken+" = map[string]int{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "totally-innocuous-name.link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported on this platform: %v", err)
	}

	got, err := s2Sweep(dir, s2ForbiddenToken(), s2ControlToken, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.state != sweepCheckedFailed {
		t.Fatalf("state = %s, want %s — a symlink whose TARGET (not name) carries the token "+
			"was not caught (hits=%v)", got.state, sweepCheckedFailed, got.hits)
	}
	if len(got.hits) != 1 || got.hits[0] != "totally-innocuous-name.link" {
		t.Fatalf("hits = %v, want exactly [totally-innocuous-name.link]", got.hits)
	}
}
