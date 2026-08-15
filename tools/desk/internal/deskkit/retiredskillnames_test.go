package deskkit

// retiredskillnames_test.go — the DIFF half of derive-or-diff for the DESK LOOP NAMES
// (methodology/46).
//
// Two desks were renamed to name their function rather than their mechanism:
//
//	issue-loop    -> intake-desk   (issue-loop/13)
//	batch-fanout  -> worker-desk   (methodology/46)
//
// A rename is only finished when the old name cannot come back. It comes back the same
// way every time: a skill lives in TWO places in this repo — the canonical body under
// `.claude/skills/` and its port under `plugins/assay/skills/` — and nothing byte-diffs
// them. `tools/plugindrift` checks the bundle against SOURCES.yaml, but the bundled
// bodies are deliberately SCRUBBED ports (see plugins/assay/PARITY.md), so their
// expected state is `behind`, not `in-sync` — a name that lands in one copy and not the
// other produces no signal there at all. So the NAMES get diffed here, where the copies
// are checked against one declared vocabulary rather than against each other.
//
// WHAT IS BANNED, AND WHAT DELIBERATELY IS NOT. The retired names are still legitimate
// text in several places, and a check that banned them everywhere would be wrong:
//
//   - `issue-loop/NN`, `issue-loop/issue-<NN>` — REGISTER IDs (a stream and its brief
//     numbers), not skill paths. `docs/streams/issue-loop/` is not renamed; methodology/46
//     scopes the stream-directory rename out explicitly. Not banned.
//   - `issue-loop--issue-<NN>` — a superseded CLAIM-KEY form the skills document in order
//     to warn against it. Not banned.
//   - `<config-dir>/issue-loop-token` — the intake App's token path, read from whatever
//     config directory the operator keeps. That is an OPERATOR-CONFIG file path; renaming
//     an operator's on-disk config is a human act, not a rename pass's. Not banned.
//   - `issue-desk` — appears inside the real filename
//     `docs/streams/findings/2026-07-20-issue-desk-emits-briefs-not-prs.md`. Not banned.
//   - `batch-fanout` in `plugins/assay/PARITY.md`, `RELEASE-NOTES.md` and the `notes:` of
//     `SOURCES.yaml` — PROVENANCE: what the path was called at a pinned commit. Rewriting
//     those would make the provenance record false. Those files are outside the scanned
//     surface, by construction, not by allowlist.
//
// What IS banned, in the SKILL BODIES only:
//
//  1. a directory under either skills root named a retired name;
//  2. a `skills/<retired>` SKILL-PATH reference;
//  3. the bare token `batch-fanout` — unlike `issue-loop` it is not, and never was, a
//     register ID, a config path, or a filename. Any occurrence is the retired skill name.
//
// The scanner is a plain function over a root, so the POSITIVE CONTROL below can run the
// identical code against a fixture carrying each violation and prove the check fails on it.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// skillRoots are the two homes one desk skill has in this repo, relative to this package.
// Both are covered by the `.claude/**` and `plugins/**` path filters of
// .github/workflows/tools.yml, so a diff to either runs this test (pinned by
// citrigger_test.go).
var skillRoots = []string{
	"../../../../.claude/skills",
	"../../../../plugins/assay/skills",
}

// retiredDeskNames is the DECLARED SOURCE: the retired name -> what it is now.
var retiredDeskNames = map[string]string{
	"batch-fanout": "worker-desk",
	"issue-loop":   "intake-desk",
}

// retiredSkillPathRe matches a SKILL-PATH reference to a retired name — the form that
// survives a careless rename because it reads like prose (`.claude/skills/batch-fanout/`).
var retiredSkillPathRe = regexp.MustCompile(`skills/(batch-fanout|issue-loop|issue-desk)\b`)

// bareRetiredRe matches the retired names that can never be anything else. `issue-loop`
// is NOT here — see the header: it is a live register/stream/config identifier.
var bareRetiredRe = regexp.MustCompile(`batch-fanout`)

// currentDeskNames must all be present across the scanned corpus. Without this the check
// would pass vacuously against an empty, moved, or mis-rooted tree: "no retired names
// found" means nothing until the corpus is known to be the real one.
var currentDeskNames = []string{"worker-desk", "intake-desk", "pr-review-desk", "verify-desk"}

type retiredHit struct {
	file string
	line int
	text string
	why  string
}

// scanRetiredDeskNames walks one skills root and returns every violation, plus the number
// of SKILL.md files it actually read. A zero read count is a broken scan, never a pass.
func scanRetiredDeskNames(root string) (hits []retiredHit, scanned int, seen map[string]bool, err error) {
	seen = map[string]bool{}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, 0, seen, err
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Rule 1: no directory may carry a retired name.
		if now, retired := retiredDeskNames[e.Name()]; retired {
			hits = append(hits, retiredHit{
				file: filepath.Join(root, e.Name()),
				why:  "skill directory still carries the retired name; it is now " + now,
			})
		}

		path := filepath.Join(root, e.Name(), "SKILL.md")
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			continue // not every skill directory carries a SKILL.md
		}
		scanned++
		body := string(raw)
		for _, n := range currentDeskNames {
			if strings.Contains(body, n) {
				seen[n] = true
			}
		}
		for i, line := range strings.Split(body, "\n") {
			// Rule 2: no `skills/<retired>` path reference.
			if m := retiredSkillPathRe.FindString(line); m != "" {
				hits = append(hits, retiredHit{
					file: path, line: i + 1, text: strings.TrimSpace(line),
					why: "SKILL-PATH reference to a retired skill directory: " + m,
				})
				continue
			}
			// Rule 3: the bare token that can only ever be the retired desk name.
			if m := bareRetiredRe.FindString(line); m != "" {
				hits = append(hits, retiredHit{
					file: path, line: i + 1, text: strings.TrimSpace(line),
					why: "retired desk name " + m + "; it is now " + retiredDeskNames[m],
				})
			}
		}
	}
	return hits, scanned, seen, nil
}

// TestSkillBodiesCarryNoRetiredDeskName is the diff.
func TestSkillBodiesCarryNoRetiredDeskName(t *testing.T) {
	totalScanned := 0
	seenAll := map[string]bool{}

	for _, root := range skillRoots {
		hits, scanned, seen, err := scanRetiredDeskNames(root)
		if err != nil {
			t.Fatalf("cannot read skills root %s: %v", root, err)
		}
		if scanned == 0 {
			t.Fatalf("read no SKILL.md under %s — the scan is broken, which is never a pass", root)
		}
		totalScanned += scanned
		for n := range seen {
			seenAll[n] = true
		}
		for _, h := range hits {
			if h.line == 0 {
				t.Errorf("%s: %s", h.file, h.why)
				continue
			}
			t.Errorf("%s:%d: %s\n    %s", h.file, h.line, h.why, h.text)
		}
	}

	if totalScanned < len(skillRoots) {
		t.Fatalf("scanned only %d SKILL.md across %d roots", totalScanned, len(skillRoots))
	}
	// Corpus-identity guard: the real skills tree names every current desk.
	for _, n := range currentDeskNames {
		if !seenAll[n] {
			t.Errorf("current desk name %q appears in NO scanned SKILL.md — "+
				"this corpus is not the desk-skills tree, so a clean result proves nothing", n)
		}
	}
}

// TestRetiredDeskNameScannerFailsOnAPositiveControl is the proof the check CAN fail.
// A guard that has never been observed going red is a guard nobody has tested. Each rule
// gets its own un-renamed occurrence, and the scanner must catch each one.
func TestRetiredDeskNameScannerFailsOnAPositiveControl(t *testing.T) {
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

	// Rule 1 control: a directory still named `batch-fanout`.
	write("batch-fanout", "name: batch-fanout\n")
	// Rule 2 control: a SKILL-PATH reference to the retired directory.
	write("the-desk", "see `.claude/skills/issue-loop/SKILL.md` for the front door\n")
	// Rule 3 control: the bare retired token in prose.
	write("verify-desk", "hands the draft PRs back to the batch-fanout window\n")
	// Negative controls: legitimate text that must NOT be reported.
	write("intake-desk", strings.Join([]string{
		"dispatch of `issue-loop/issue-<NN>` placeholders belongs to worker-desk",
		"GH_TOKEN=$(cat <config-dir>/issue-loop-token)",
		"NOT the pre-#575 `issue-loop--issue-<NN>` form",
		"see docs/streams/findings/2026-07-20-issue-desk-emits-briefs-not-prs.md",
		"this is issue-loop/14, a register ID",
	}, "\n")+"\n")

	hits, scanned, _, err := scanRetiredDeskNames(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if scanned != 4 {
		t.Fatalf("fixture: scanned %d SKILL.md, want 4", scanned)
	}

	var gotDir, gotPath, gotBare int
	for _, h := range hits {
		switch {
		case h.line == 0:
			gotDir++
		case strings.Contains(h.why, "SKILL-PATH reference"):
			gotPath++
		default:
			gotBare++
		}
		if strings.Contains(h.file, "intake-desk") {
			t.Errorf("FALSE POSITIVE on legitimate text: %s:%d %s\n    %s",
				h.file, h.line, h.why, h.text)
		}
	}

	if gotDir == 0 {
		t.Error("rule 1 (retired directory name) did not fire on its positive control — " +
			"the check cannot catch a half-done rename")
	}
	if gotPath == 0 {
		t.Error("rule 2 (skills/<retired> path reference) did not fire on its positive control")
	}
	if gotBare == 0 {
		t.Error("rule 3 (bare retired token) did not fire on its positive control")
	}
}
