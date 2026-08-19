package deskkit

// skillrepolist_test.go — the DIFF half of derive-or-diff for the DESK WRITE-REPO SET
// (#258).
//
// The acting desks each ENUMERATED the repos they watch/act on, and those lists had
// drifted from the tools' write-authorisation set (`ASSAY_ALLOWED_REPOS`, the exit-5
// boundary) in BOTH directions: they named repos the tools REFUSE to act on (phantom
// coverage — a session believing it covers a repo the write boundary excludes) while
// OMITTING ones the tools do cover (a silent monitoring blind spot). Neither direction is
// visible from inside a skill file, because a prose list has nothing to disagree with.
// This is the F-30 anti-pattern — a hand-maintained parallel list going stale — reborn in
// the skill bodies.
//
// The fix is NOT a better-synchronised list. It is having no second list: the acting-desk
// skills say "run `deskroster repos`", and the answer comes from the same values the gates
// read (a list you PRINT cannot drift from the list you ENFORCE). This test is the guard
// that keeps the second list from creeping back.
//
// WHAT IS BANNED, AND WHAT DELIBERATELY IS NOT. Three distinct repo sets exist and only
// ONE is at issue here — the WRITE-authorisation boundary. A check that banned every
// out-of-boundary repo name everywhere would be wrong, because the other two sets
// legitimately name such repos:
//
//   - the STREAM-BOARD roots the worker desk scans for the dispatch queue
//     (`../example-platform` — a board waiting for its first stream). A board root is
//     WHERE briefs live, not a write-authorisation claim. Not banned.
//   - the house-wide REPORT read scope the dailies role sweeps (`example-site`, the deck
//     repos as report targets). A report reads broadly and writes nothing. Not banned,
//     and dailies is out of the acting-desk scan entirely.
//
// What IS banned, in the ACTING-desk SKILL bodies only, are the three deck-repo TOKENS
// `reconciler-slides`, `assay-slides`, `example-slides`. These three are a FINGERPRINT of the
// retired hardcoded write-list, not a claim about the roster: all three are inside the
// write boundary today (`deskroster repos --scope write` prints them), so a deck repo is
// legitimate write-authorised coverage — the ban is not "these repos are out of bounds".
// It is that these tokens never had any home in an acting-desk SKILL body OTHER than the
// drifted write-list — unlike `example-platform` (a real board root) or `example-site` (a
// real report target), which appear in skill prose for reasons of their own — so their
// presence is that specific list creeping back in.
//
// SCOPE, STATED HONESTLY. The token ban catches a verbatim re-paste of THE HISTORICAL
// list (any one of its deck tokens is enough); it does NOT catch a freshly-drafted drift
// that enumerates only in-boundary repos, or a new phantom repo that was never in the old
// paste. That residual gap is real. The positive requirement below (each acting desk must
// carry the `deskroster repos` marker) narrows but does not close it — a body can carry
// both a hardcoded list AND the marker. The durable defense is the prose itself telling
// the operator to read the set from the tool; this guard is a tripwire against a literal
// re-paste of the old list, not a proof of no-drift.
//
// The POSITIVE requirement is the other half: each acting desk must actually carry the
// `deskroster repos` instruction that REPLACES the list. Deleting the deck tokens without
// telling the desk where to get the set would leave it with no repo set at all; requiring
// the marker closes that gap.
//
// The scanner is a plain function over a root, so the POSITIVE CONTROL below runs the
// identical code against a fixture carrying each violation and proves the check fails on
// it. A guard never observed going red is a guard nobody has tested.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skillRepoListRoots is the canonical acting-desk home scanned by this guard. Covered by
// the `.claude/**` path filter of .github/workflows/tools.yml, so a diff to it runs this
// test (pinned by citrigger_test.go, same as retiredskillnames_test).
//
// The `plugins/assay/skills` port is DELIBERATELY OUT OF SCOPE and NOT scanned. Its expected
// state is `behind` (a scrubbed mirror tracked by tools/plugindrift + plugins/assay/PARITY.md),
// and the v0.1.0 scrub rewrites `*-decks` → `*-slides`, so the deck-repo tokens this guard
// bans cannot occur there at all — scanning the port would be a pure no-op that bought a false
// "covers both roots" claim rather than coverage. Closing the #258 drift in the port is the
// port-sync's job, not this tripwire's.
var skillRepoListRoots = []string{
	"../../../../.claude/skills",
}

// canonicalSkillRoot is the authoritative body. The derive-from-tool marker is REQUIRED here
// (the port is out of scope, above).
const canonicalSkillRoot = "../../../../.claude/skills"

// actingDeskSkills ACT on repos — review/flip PRs, dispatch workers, verify cross-repo —
// and so must take their repo set from the tool rather than a hardcoded list. The dailies
// REPORT role reads a broader house-wide scope and is deliberately NOT here.
var actingDeskSkills = []string{"pr-review-desk", "worker-desk", "verify-desk"}

// retiredWriteListTokens are the deck-repo tokens that only ever appeared in the retired
// hardcoded write-list — a FINGERPRINT of that paste, not a claim they are out of the write
// boundary (all three are inside it today). See the header for why these three.
var retiredWriteListTokens = []string{"reconciler-slides", "assay-slides", "example-slides"}

// deriveFromToolMarker is the instruction that REPLACES the list.
const deriveFromToolMarker = "deskroster repos"

type skillRepoHit struct {
	file string
	line int
	text string
	why  string
}

// scanSkillRepoList walks one skills root and returns every violation across the acting
// desks, plus the number of acting-desk SKILL.md files it actually read. requireMarker
// adds the positive check that each scanned acting desk carries deriveFromToolMarker; it
// is applied only for the canonical root. A zero read count is a broken scan, never a pass.
func scanSkillRepoList(root string, requireMarker bool) (hits []skillRepoHit, scanned int, err error) {
	for _, skill := range actingDeskSkills {
		path := filepath.Join(root, skill, "SKILL.md")
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			continue // a scrubbed port need not carry every acting desk
		}
		scanned++
		body := string(raw)

		for i, line := range strings.Split(body, "\n") {
			for _, tok := range retiredWriteListTokens {
				if strings.Contains(line, tok) {
					hits = append(hits, skillRepoHit{
						file: path, line: i + 1, text: strings.TrimSpace(line),
						why: "deck-repo token " + tok + " is a fingerprint of the retired hardcoded " +
							"write-list (#258) — banned in an acting-desk body regardless of the repo's " +
							"current write-boundary membership; take the repo set from `deskroster repos`, " +
							"not a pasted list",
					})
				}
			}
		}

		if requireMarker && !strings.Contains(body, deriveFromToolMarker) {
			hits = append(hits, skillRepoHit{
				file: path,
				why: "acting desk carries no `" + deriveFromToolMarker + "` instruction — " +
					"the derive-from-tool source that replaces the hardcoded repo list (#258)",
			})
		}
	}
	return hits, scanned, nil
}

// TestActingDeskSkillsCarryNoHardcodedRepoList is the diff.
func TestActingDeskSkillsCarryNoHardcodedRepoList(t *testing.T) {
	canonicalScanned := 0

	for _, root := range skillRepoListRoots {
		requireMarker := root == canonicalSkillRoot
		hits, scanned, err := scanSkillRepoList(root, requireMarker)
		if err != nil {
			t.Fatalf("cannot scan skills root %s: %v", root, err)
		}
		if requireMarker {
			canonicalScanned = scanned
		}
		for _, h := range hits {
			if h.line == 0 {
				t.Errorf("%s: %s", h.file, h.why)
				continue
			}
			t.Errorf("%s:%d: %s\n    %s", h.file, h.line, h.why, h.text)
		}
	}

	// Corpus-identity guard: the canonical tree must carry EVERY acting desk. Without
	// this the check would pass vacuously against an empty, moved, or mis-rooted tree —
	// "no hardcoded lists found" means nothing until the corpus is known to be real.
	if canonicalScanned != len(actingDeskSkills) {
		t.Fatalf("read %d of %d acting-desk SKILL.md under %s — the scan is broken, which is never a pass",
			canonicalScanned, len(actingDeskSkills), canonicalSkillRoot)
	}
}

// TestSkillRepoListScannerFailsOnAPositiveControl is the proof the check CAN fail.
func TestSkillRepoListScannerFailsOnAPositiveControl(t *testing.T) {
	root := t.TempDir()

	write := func(dir, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Violation 1: a retired-write-list deck token re-pasted into an acting desk. It also
	// carries the marker, so ONLY the token rule should fire on it.
	write("pr-review-desk", "watch assay, example-reconciler, reconciler-slides\nrun `deskroster repos`\n")
	// Violation 2: an acting desk with NO derive-from-tool marker.
	write("worker-desk", "sweep the open PRs across all watched repos\n")
	// Negative control: legitimate non-banned repo names (a board root + a report
	// target, neither a retired-write-list deck token) plus the marker — must NOT fire.
	write("verify-desk", strings.Join([]string{
		"resync the sibling checkouts under `../example-platform` before verifying",
		"the report roster still lists example-site as a read target",
		"read the repo set from `deskroster repos` — this skill carries no list",
	}, "\n")+"\n")

	hits, scanned, err := scanSkillRepoList(root, true)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if scanned != len(actingDeskSkills) {
		t.Fatalf("fixture: scanned %d acting-desk SKILL.md, want %d", scanned, len(actingDeskSkills))
	}

	var gotToken, gotMarker int
	for _, h := range hits {
		if strings.Contains(h.file, "verify-desk") {
			t.Errorf("FALSE POSITIVE on legitimate text: %s:%d %s\n    %s",
				h.file, h.line, h.why, h.text)
		}
		switch {
		case strings.Contains(h.why, "fingerprint of the retired hardcoded"):
			gotToken++
		case strings.Contains(h.why, "no `"+deriveFromToolMarker+"`"):
			gotMarker++
		}
	}

	if gotToken == 0 {
		t.Error("the phantom-token rule did not fire on its positive control — " +
			"the check cannot catch a re-pasted write-list")
	}
	if gotMarker == 0 {
		t.Error("the derive-from-tool-marker rule did not fire on its positive control — " +
			"the check cannot catch an acting desk that dropped the list without a source")
	}
}
