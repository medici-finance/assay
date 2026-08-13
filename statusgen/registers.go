package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ----- register integrity (Task 6 — re-homed anti-falsification guarantee) -----

// registerIntegrityProblems replaces the old registerSequenceProblems (contiguous-
// numbering gap/dup check on the single-file registers). The new check:
//   - Flags duplicate ids across per-entry files.
//   - Flags any register entry file that has ever existed on main (per git history)
//     but is absent from the working tree — a deletion where only a tombstone
//     (disposition: rejected / resolved: yes) is allowed (F-05/F-08 lineage).
func registerIntegrityProblems(root string) []string {
	var problems []string

	// intake duplicate-id check
	intakeEntries, err := parseIntakeDir(root)
	if err != nil {
		return []string{fmt.Sprintf("register integrity: reading intake: %v", err)}
	}
	if dups := duplicateIDs(entriesToKeyed(intakeEntries)); len(dups) > 0 {
		for _, d := range dups {
			problems = append(problems, fmt.Sprintf("intake register: duplicate id %s (two or more per-entry files claim this id)", d))
		}
	}

	// intake field-quality checks: unparseable dates and missing disposition key
	// are a lint problem — they make the untriaged-age alarm partially blind.
	intakeDir := filepath.Join(root, "docs", "streams", "intake")
	for _, e := range intakeEntries {
		// date parse check: a malformed date makes an entry permanently age-invisible
		if _, err := time.Parse("2006-01-02", strings.TrimSpace(e.Date)); err != nil {
			problems = append(problems, fmt.Sprintf(
				"intake register: %s: unparseable date %q — age not computable; fix the date: field",
				e.ID, e.Date))
		}
	}

	// disposition-key check: parse each raw file's frontmatter and look for the
	// "disposition" YAML key directly — after the parser defaults an absent key
	// to "new", the frontmatter is invisible to the alarm but the source is
	// still incomplete. This used to raw-substring-scan the frontmatter text for
	// "\ndisposition:", which is only safe if the fence
	// split it operates on is itself reliable; parsing the key structurally
	// via splitFrontmatter + yaml.Unmarshal removes that fragility outright —
	// no text scan, no risk of matching a line that merely starts with
	// "disposition:" inside a body/code-fence example.
	intakeFiles, intakeDirErr := os.ReadDir(intakeDir)
	if intakeDirErr == nil {
		for _, f := range intakeFiles {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") || f.Name() == "README.md" {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(intakeDir, f.Name()))
			if err != nil {
				continue
			}
			fm, _, err := splitFrontmatter(string(raw))
			if err != nil {
				continue // malformed frontmatter is reported by the field-quality/parse checks elsewhere
			}
			var keys map[string]any
			if err := yaml.Unmarshal([]byte(fm), &keys); err != nil {
				continue
			}
			if _, ok := keys["disposition"]; !ok {
				// Extract the ID from the frontmatter for the problem message
				var entry intakeEntry
				if err := yaml.Unmarshal([]byte(fm), &entry); err != nil || entry.ID == "" {
					continue
				}
				problems = append(problems, fmt.Sprintf(
					"intake register: %s: missing disposition key — add 'disposition: new' (or another valid disposition) to the frontmatter",
					entry.ID))
			}
		}
	}

	// findings duplicate-id check
	findingEntries, err := parseFindingsDir(root)
	if err != nil {
		return append(problems, fmt.Sprintf("register integrity: reading findings: %v", err))
	}
	if dups := duplicateIDs(entriesToKeyed(findingEntries)); len(dups) > 0 {
		for _, d := range dups {
			problems = append(problems, fmt.Sprintf("findings register: duplicate id %s (two or more per-entry files claim this id)", d))
		}
	}

	// file-presence-vs-git-history check: any entry file ever added on main
	// that is now absent from the working tree is a tombstone-not-delete
	// violation. Only meaningful in a git checkout; silently skip otherwise.
	if deleted := deletedRegisterFiles(root); len(deleted) > 0 {
		for _, d := range deleted {
			problems = append(problems, fmt.Sprintf("register entry removed (tombstone-not-delete): %s — a register entry file that was landed as of the merge-base with origin/main has been deleted; withdraw in-place (disposition: rejected or resolved: yes), never delete the file", d))
		}
	}

	// in-place field-gutting check: deleting a
	// whole entry is caught above, but silently GUTTING the load-bearing fields
	// of a finding that survives in the tree is the same falsification by a
	// subtler route. Fires as a HARD problem unless a verified-human anchor
	// authorizes it.
	problems = append(problems, guttedRegisterFields(root)...)

	// ID format validation: every entry must use either the new
	// slug form ([FI]-<slug>, 10-20 chars after prefix, [a-z0-9-]) or a
	// grandfathered legacy numeric form ([FI]-NN(-a)?). New entries using the
	// numeric form are a PROBLEM (regression to the counter).
	problems = append(problems, idFormatProblems(root)...)

	return problems
}

type keyedEntry interface {
	GetID() string
}

func (e intakeEntry) GetID() string  { return e.ID }
func (e findingEntry) GetID() string { return e.ID }

func entriesToKeyed[T keyedEntry](entries []T) []keyedEntry {
	out := make([]keyedEntry, len(entries))
	for i, e := range entries {
		out[i] = e
	}
	return out
}

func duplicateIDs(entries []keyedEntry) []string {
	seen := map[string]int{}
	for _, e := range entries {
		seen[e.GetID()]++
	}
	var dups []string
	for id, n := range seen {
		if n > 1 {
			dups = append(dups, id)
		}
	}
	sort.Strings(dups)
	return dups
}

// deletedRegisterFiles returns register entry file paths (relative to repo root)
// that were landed as of the shared history with main but are absent from the
// working tree — a tombstone-not-delete violation. The "landed" cut is the
// MERGE-BASE of HEAD and origin/main, not HEAD and not origin/main's tip:
//   - a feature branch that ADDS a register file and then removes/renames it (a
//     correction of its own not-yet-landed work) is added AFTER the merge-base,
//     so it is NOT flagged;
//   - a file main gained AFTER the merge-base (a branch simply behind main that
//     hasn't merged yet) is likewise not in the merge-base history, so NOT
//     flagged;
//   - only a file that existed at the shared merge-base and is now gone from the
//     working tree — an actual deletion of landed history — fires.
//
// Falls back to HEAD when origin/main can't be resolved (main's own CI after the
// fetch/reset, where HEAD IS the authoritative tip, or a fixture with no remote).
//
// "Landed" is measured on main's FIRST-PARENT line (--first-parent in the add
// enumeration below), so post-merge strictness holds regardless of squash vs
// merge: a squash collapses a branch's transient add+delete into
// nothing; a non-squash merge keeps both in main's full history but nets them to
// nothing along the merge commit's first-parent diff — so neither smuggles a
// never-landed entry into the "landed" set. A file only counts as landed once it
// appears in a first-parent main snapshot; deleting such a file fires. Scoping to
// the merge-base additionally relaxes the check for a branch deleting its OWN
// not-yet-landed add; it never weakens protection for anything landed on main.
//
// (Earlier this check ran WITHOUT --first-parent, on the theory that strictness
// held "regardless of squash vs merge" — that was false: a non-squash merge of a
// transient add+delete then fired a tombstone PROBLEM on main for branch-local
// churn that never landed. Scoping to the first-parent line is the fix.)
func deletedRegisterFiles(root string) []string {
	// Only meaningful in a git checkout.
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		return nil
	}

	// Cut at the merge-base of HEAD and origin/main: files added up to that
	// shared point are landed; files added on either side after it are not
	// violations. Fall back to HEAD when origin/main can't be resolved.
	base := "HEAD"
	if mb, err := exec.Command("git", "-C", root, "merge-base", "HEAD", "origin/main").Output(); err == nil && strings.TrimSpace(string(mb)) != "" {
		base = strings.TrimSpace(string(mb))
	}

	var deleted []string
	for _, dir := range []string{"docs/streams/intake", "docs/streams/findings"} {
		// Enumerate files ADDED on main's first-parent line up to the landed
		// merge-base. --first-parent is load-bearing: "landed" must mean
		// "appeared in a main first-parent snapshot", not "added in any ancestor
		// commit". Without it, a non-squash merge of a branch that ADDS a register
		// file and then deletes/renames it before merging smuggles that transient
		// add into main's full history; the file is absent from the tree, so the
		// check fires on main for a branch-local churn that was never a landed
		// entry (the escape this guards against). Along the merge's first-parent
		// diff the add+delete net to nothing, so it is correctly not counted.
		cmd := exec.Command("git", "-C", root, "log", "--diff-filter=A", "--first-parent",
			"--name-only", "--format=", base, "--", dir+"/")
		out, err := cmd.Output()
		if err != nil {
			continue // git unavailable or no history — skip
		}
		seen := map[string]bool{}
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			seen[line] = true
		}
		// T8: build a set of register IDs present in the current working tree,
		// so a RENAMED file (same ID, new path) is not flagged as deleted.
		idInTree := map[string]bool{}
		fsDir := filepath.Join(root, dir)
		if treeFiles, readErr := os.ReadDir(fsDir); readErr == nil {
			for _, f := range treeFiles {
				if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
					continue
				}
				raw, readErr := os.ReadFile(filepath.Join(fsDir, f.Name()))
				if readErr != nil {
					continue
				}
				id := extractIDFromYAMLFrontmatter(raw)
				if id != "" {
					idInTree[id] = true
				}
			}
		}
		// Check each ever-added file against the working tree.
		for f := range seen {
			if _, err := os.Stat(filepath.Join(root, f)); err == nil {
				continue // still exists — not a deletion
			}
			// T8: the file is gone from its old path, but if a file with the
			// same register ID exists in the current tree, it was RENAMED —
			// not deleted. Parse the ID from the landed base tree.
			showCmd := exec.Command("git", "-C", root, "show", base+":"+f)
			showOut, showErr := showCmd.Output()
			if showErr == nil {
				landedID := extractIDFromYAMLFrontmatter(showOut)
				if landedID != "" && idInTree[landedID] {
					continue // rename: same ID exists at a new path — not a deletion
				}
			}
			deleted = append(deleted, f)
		}
	}
	sort.Strings(deleted)
	return deleted
}

// ----- in-place field-gutting guard -----

// registerLandedBase returns the git ref whose tree holds the LANDED version of a
// register entry: the merge-base of HEAD and origin/main (mirroring
// deletedRegisterFiles). The second return reports whether that merge-base
// actually resolved. When it did NOT, the returned base is "HEAD" — a FAIL-OPEN
// fallback: a mutation that is already COMMITTED is then compared against itself,
// diffs to nothing, and passes silently. Callers that gate on the result must
// surface that (see registerBaseFallbackNotices).
func registerLandedBase(root string) (base string, resolved bool) {
	if mb, err := exec.Command("git", "-C", root, "merge-base", "HEAD", "origin/main").Output(); err == nil && strings.TrimSpace(string(mb)) != "" {
		return strings.TrimSpace(string(mb)), true
	}
	return "HEAD", false
}

// registerBaseFallbackNotices emits a NOTICE when the field-gutting guard is
// running against the fail-open base=HEAD fallback while there are findings to
// guard.
//
// Why this exists (carried from the security review, which recorded it as
// a NOTE rather than a fix): with base == HEAD a COMMITTED gutting is compared
// against itself and silently passes. In the upstream repo that is unreachable because its lint
// workflow runs `git fetch --no-tags origin main` BEFORE the lint step, so
// origin/main always resolves — but nothing tests that ordering, so a future
// workflow edit that drops the fetch disables the control with no failing test and
// no signal at all.
//
// This repo's own .github/workflows/statusgen.yml is WEAKER on exactly that point:
// its explicit `git fetch --no-tags origin main` is in the STATUS.md-guard step
// AFTER `go run . --root .. --lint`, so the lint step's origin/main resolution
// rests entirely on actions/checkout's implicit fetch-depth: 0 refspec. That is an
// implementation detail of a third-party action, not a guarantee this repo states.
// Hence a NOTICE rather than silence: if the ref is not there, the run says so.
//
// Advisory only — never a hard problem. A local clone or a fixture legitimately
// has no origin/main, and the guard still catches WORKING-TREE gutting there; only
// already-committed gutting escapes.
//
// A tree with NO .git directory at all (a `git archive` export) is
// a third case, distinct from "origin/main unresolvable": guttedRegisterFields
// skips outright rather than falling back to base=HEAD (it has no ref to fall
// back to), so nothing is silently compared against itself — but the guard
// still did not run, and that absence deserves the same NOTICE for the same
// reason: a silent difference in which checks ran is what makes a differential
// lint comparison across two trees unsound.
func registerBaseFallbackNotices(root string) []string {
	var cause string
	switch {
	case hasNoGitDir(root):
		cause = "this tree has no .git directory at all (e.g. a `git archive` export), so the guard could not read a landed base at all and was skipped entirely"
	default:
		if _, resolved := registerLandedBase(root); resolved {
			return nil
		}
		cause = "origin/main could not be resolved, so the landed base fell back to HEAD and finding entries are compared against themselves — already-COMMITTED gutting cannot be detected in this run (working-tree gutting still is)"
	}
	// Only worth saying when there is actually a findings register to guard.
	files, err := os.ReadDir(filepath.Join(root, "docs", "streams", "findings"))
	if err != nil {
		return nil
	}
	n := 0
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	return []string{fmt.Sprintf(
		"register field-gutting guard is running degraded: %s (%d finding entr%s affected). If this is CI, fetch origin/main before the lint step; if this is a git-archive export, lint a real worktree instead (`git worktree add`) for a result comparable to CI.",
		cause, n, map[bool]string{true: "y is", false: "ies are"}[n == 1])}
}

// guttedRegisterFields flags any finding entry whose LOAD-BEARING fields were
// mutated in place, relative to the version landed at the merge-base with
// origin/main, in a way that silently unblocks a brief the finding was demoting —
// unless a verified-human anchor authorizes the change.
//
// Three gutting shapes are caught. The first two are the original guard's; the third closes a
// hole found by the security review:
//   - resolved: no -> yes — flipping a finding resolved erases its brief demotion.
//   - affects: losing any entry (narrowed or emptied) — statusgen keys the
//     stale-knowledge block on affects, so dropping a stream/brief unblocks it.
//   - ack: non-empty -> empty — check() reads a THIRD load-bearing field. With a
//     well-formed ack, an unresolved finding on an in-progress/implemented brief
//     is a hard PROBLEM ("demote to todo (re-gate) or resolve the finding",
//     checks.go); with ack absent it degrades to a mere NOTICE ("awaits desk
//     ack"). Note the asymmetry that makes removal silent: CORRUPTING ack is
//     caught by the malformed-ack check, but that check is guarded by
//     `f.Ack != ""`, so REMOVING it is not caught anywhere else.
//
// The opposite moves (re-opening a finding, WIDENING affects, ADDING an ack) are
// never gutting — they add caution, not remove it — so they are not flagged.
//
// HARD GATE (human-gate lineage): a detected gutting is a lint PROBLEM
// unless the CURRENT entry carries a verified-human anchor in its frontmatter
// `authorized-by:` key (see authorizedByVerifiedHuman). An unknown name or a bare
// agent-written justification does NOT authorize.
//
// That anchor is NOT verified online today, and the strength claim is stated here
// rather than omitted because a reader needs the gate's real reach, not a tidier
// absence. `statusgen --corroborate <pr>` — which checks a human:<name> token ADDED
// in a PR diff against that account's on-PR action — is wired into NO workflow
// yet, so in CI the anchor is one frontmatter line an agent can write for
// itself. The ruling on I-corroborate-lint took the
// STRICTER arm instead: the human-allowlist workflow becomes
// the sole permitted writer of the human:alex stamp, and --lint rejects that stamp
// added anywhere else. That arm is scoped to human:<name> stamps in the
// Verified/Reviewed cells of stream README tables; it deliberately does NOT govern
// this `authorized-by:` frontmatter key, whose design is precisely that a PR adds
// it — a blanket rule would forbid the token this gate depends on. So the anchor
// survives unchanged and --corroborate keeps a live consumer here.
//
// What remains enforced meanwhile is not nothing: the gutting itself is detected
// against a merge-base the branch author cannot forge, which nothing did before, and
// removing caution now costs a deliberate, attributable line in the diff instead of
// a silent field edit a reviewer never sees. The residual is that the line is
// self-issuable — not that the detection is.
//
// The comparison base is measured with `git show <base>:<path>`: an entry absent at
// the base is a not-yet-landed add (not a mutation of landed history) and is
// skipped, and a deleted entry (absent from the working tree) is left to
// deletedRegisterFiles — this function never double-reports it. Only meaningful in
// a git checkout; silently skips otherwise. When origin/main is unresolvable the
// base is HEAD, so committed mutations can't be diffed — that fail-open is
// reported as a NOTICE by registerBaseFallbackNotices; working-tree gutting is
// still caught.
//
// Two further fail-opens are documented but NOT fixed here, both carried from the
// review:
//   - A branch whose merge-base PREDATES the finding sees `git show base:rel` fail
//     and skips. The backstop is git, not statusgen: main added that path after the
//     merge-base and the branch adds it too, so the merge is an add/add CONFLICT;
//     resolving it means merging origin/main, which advances the merge-base and
//     re-arms this guard on the next push.
//   - The INTAKE register gets no in-place guard: this scans findings only.
//     Intake entries have the deletion/tombstone guard (deletedRegisterFiles) but
//     an in-place `disposition:` flip is not caught. Intake disposition drives only
//     NOTICE-level output (untriaged-age alarm, decision-issue notice), so no hard
//     gate is bypassed — an asymmetry, not a hole.
func guttedRegisterFields(root string) []string {
	// Only meaningful in a git checkout.
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		return nil
	}
	base, _ := registerLandedBase(root)

	const dir = "docs/streams/findings"
	fsDir := filepath.Join(root, "docs", "streams", "findings")
	files, err := os.ReadDir(fsDir)
	if err != nil {
		return nil // no findings dir — nothing to guard
	}

	// T8: build a map of register IDs to their landed base paths so a renamed
	// entry is still diffed against its landed version. Without this, a file
	// renamed from old-slug.md to new-slug.md causes git show base:new-slug.md
	// to fail (the base tree only knows the old path), and the entry is
	// silently skipped — see the S4/S5 bypass surfaced in an independent review.
	//
	// Scan the base tree for all .md files, read each one's YAML frontmatter,
	// and index by register ID. A working-tree entry whose path does not exist
	// at the base is resolved via this index: if its ID was landed at a
	// DIFFERENT path, diffs are computed against that base version — closing
	// the rename+gut one-commit window.
	landedByID := map[string]string{} // register ID -> base raw content string
	lsCmd := exec.Command("git", "-C", root, "ls-tree", "--name-only", "-r", base, dir+"/")
	if lsOut, lsErr := lsCmd.Output(); lsErr == nil {
		for _, line := range strings.Split(string(lsOut), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || !strings.HasSuffix(line, ".md") {
				continue
			}
			showCmd := exec.Command("git", "-C", root, "show", base+":"+line)
			showOut, showErr := showCmd.Output()
			if showErr != nil {
				continue
			}
			id := extractIDFromYAMLFrontmatter(showOut)
			if id != "" {
				landedByID[id] = string(showOut)
			}
		}
	}

	var problems []string
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		rel := dir + "/" + f.Name()
		curRaw, err := os.ReadFile(filepath.Join(fsDir, f.Name()))
		if err != nil {
			// Deleted / unreadable — deletedRegisterFiles owns deletion and this
			// function must never double-report it. That property has TWO
			// independent guards, and mutation testing shows either one alone
			// still holds it: the loop enumerates the WORKING TREE, so a deleted
			// entry is never listed in the first place, AND this skip catches it
			// if the enumeration ever grows a base-side source. Keep both.
			continue
		}
		curE, err := parseFindingFile(curRaw)
		if err != nil {
			continue
		}

		// Landed (base) version. If the file at the SAME path exists at the
		// base, compare against it directly (the common case). If not, the
		// file may have been RENAMED — resolve by register ID via the
		// landedByID index (T8: closes the rename+gut bypass).
		var baseE *findingEntry
		baseRaw, pathErr := exec.Command("git", "-C", root, "show", base+":"+rel).Output()
		if pathErr == nil {
			baseE, err = parseFindingFile(baseRaw)
			if err != nil {
				continue
			}
		} else if landedRaw, ok := landedByID[curE.ID]; ok {
			// T8: same register ID exists at a different path in the base tree —
			// a rename. Diff against THAT version so a rename+gut in one commit
			// is still caught.
			baseE, err = parseFindingFile([]byte(landedRaw))
			if err != nil {
				continue
			}
		} else {
			// Not found by path or by ID — a not-yet-landed add, skip.
			continue
		}

		var guts []string
		if !baseE.Resolved && curE.Resolved {
			guts = append(guts, "resolved flipped no->yes")
		}
		if dropped := removedAffects(baseE.Affects, curE.Affects); len(dropped) > 0 {
			guts = append(guts, fmt.Sprintf("affects dropped [%s]", strings.Join(dropped, ", ")))
		}
		if strings.TrimSpace(baseE.Ack) != "" && strings.TrimSpace(curE.Ack) == "" {
			guts = append(guts, "ack removed (downgrades the hard re-gate problem to a notice)")
		}
		if len(guts) == 0 {
			continue
		}

		// Gutting present — HARD unless a verified-human anchor authorizes it.
		if authorizedByVerifiedHuman(curRaw) {
			continue
		}
		problems = append(problems, fmt.Sprintf(
			"register field-gutting (unauthorized): %s — %s vs the version landed at the merge-base with origin/main, with no verified-human authorization. In-place gutting of a finding's load-bearing fields silently unblocks the brief it demoted. This is a HUMAN gate: add an `authorized-by: human:<name>` key to the entry's YAML frontmatter whose name is mapped in the configured ASSAY_HUMAN_LOGIN_MAP; an agent-written justification is not sufficient. Know what this check does and does not do before you add that key: it does NOT verify the anchor online — `statusgen --corroborate <pr>` is not yet wired into a workflow, and the ruled direction (the human-allowlist workflow as sole writer of human:<name> stamps in README Verified/Reviewed cells) does not cover this frontmatter key either. So writing the key on your own authority passes this check silently, and is exactly the falsification it exists to prevent. Get the named human to authorize the change.",
			rel, strings.Join(guts, "; ")))
	}
	sort.Strings(problems)
	return problems
}

// removedAffects returns the affects entries present in base but absent from cur —
// the set narrowing/emptying that unblocks a brief. Whitespace is trimmed and empty
// entries ignored; ordering of the inputs does not matter.
func removedAffects(base, cur []string) []string {
	curSet := map[string]bool{}
	for _, a := range cur {
		curSet[strings.TrimSpace(a)] = true
	}
	var removed []string
	for _, a := range base {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if !curSet[a] {
			removed = append(removed, a)
		}
	}
	sort.Strings(removed)
	return removed
}

// authorizedByVerifiedHuman reports whether raw carries a verified-human anchor —
// a human:<name> token whose name resolves to a real GitHub login in
// the configured ASSAY_HUMAN_LOGIN_MAP (corroborate.go's HumanLogin) — under the DESIGNATED
// `authorized-by:` key of the entry's YAML FRONTMATTER. A bare or unknown name (an
// agent-written justification token) never authorizes, and neither does anything
// in the entry's prose.
//
// The frontmatter/designated-key scoping is the fix for BLOCKER 1 of the
// security review. The upstream repo's version ran humanStampRe over `string(raw)` — the WHOLE
// FILE — so any finding whose PROSE merely mentions `human:alex` was permanently
// exempt from the guard. That was live, not theoretical: on the upstream repo's branch,
// docs/streams/findings/2026-07-17-human-stamps-never-corroborated-by-ci.md is an
// OPEN finding (resolved: false) demoting a live stream whose body says human:alex
// while DESCRIBING this very weakness — gutting it produced zero lint problems.
//
// Both halves of the intended design were blind to it: --corroborate only reads
// ADDED diff lines, so a PR that guts such a finding adds no human: token for the
// online check to corroborate, while --lint treats the weeks-old prose as
// authorization. Scoping the match to a designated frontmatter key is what the upstream repo's
// own tests already expressed as the convention, so closing the hole leaves every
// existing test outcome unchanged.
//
// Fails CLOSED: unparseable frontmatter, a missing key, or a non-scalar value all
// return false (no authorization).
func authorizedByVerifiedHuman(raw []byte) bool {
	fm, _, err := splitFrontmatter(string(raw))
	if err != nil {
		return false
	}
	var keys map[string]any
	if err := yaml.Unmarshal([]byte(fm), &keys); err != nil {
		return false
	}
	v, ok := keys["authorized-by"]
	if !ok {
		return false
	}
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, m := range humanStampRe.FindAllStringSubmatch(s, -1) {
		if _, ok := HumanLogin(m[1]); ok {
			return true
		}
	}
	return false
}

// ----- view generation (Task 5) -----

// generateIntakeView renders docs/streams/INTAKE.md from the per-entry files.
// Output is byte-deterministic (entries sorted by date, then id). Returns "" when there
// are no entry files (empty register — don't write a header-only file).
func generateIntakeView(root string) (string, error) {
	entries, err := parseIntakeDir(root)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}
	// Split: decision-needed entries render in a distinct top section.
	// Both groups remain sorted by date, then id for byte-determinism.
	var decisionEntries, otherEntries []intakeEntry
	for _, e := range entries {
		if e.Disposition == "decision-needed" {
			decisionEntries = append(decisionEntries, e)
		} else {
			otherEntries = append(otherEntries, e)
		}
	}
	var b strings.Builder
	b.WriteString("# Intake\n\n")
	b.WriteString("Append-only front door for raw ideas (brainstorms, \"we should…\", strategy sessions)\n")
	b.WriteString("while they are neither rejected nor scoped. Entries are NOT briefs — no waves, no DoD.\n")
	b.WriteString("An idea becomes work only by becoming a stream (scoping doc + `author-brief`) or a\n")
	b.WriteString("brief in an existing stream; flip its disposition to `scoped → <stream>` then.\n")
	b.WriteString("Brainstorming sessions end by writing an entry here, never by creating a new ad-hoc\n")
	b.WriteString("docs/ directory. Deep strategic questions stay in `docs/v-next.md`.\n\n")
	b.WriteString("Format: heading `## I-<slug> — YYYY-MM-DD — title`, one paragraph, then a\n")
	b.WriteString("`Disposition: new | watching | scoped → <stream> | rejected — <why> | decision-needed → issue #NN` line.\n")
	b.WriteString("**IDs:** letter-prefixed slugs — `I-<slug>` where the slug is\n")
	b.WriteString("10–20 characters, `[a-z0-9-]`, derived from the title (example: `I-model-mix-tiers`).\n")
	b.WriteString("Older entries use numeric IDs (`I-01`) — those are frozen legacy and remain valid.\n")
	b.WriteString("New entries must use the slug form; a numeric ID on a new entry triggers a lint PROBLEM.\n")
	b.WriteString("**Tamper-wire:** sequential IDs carried a contiguity check to detect deletions — slugs\n")
	b.WriteString("trade contiguity for no-collision parallel write. Entry-file deletion is PR-diff-visible,\n")
	b.WriteString("this view is single-writer on main (a deletion shows as a view diff in the regen commit),\n")
	b.WriteString("and the tombstone rule (amend disposition to `rejected`, never delete the file) stays.\n\n")
	b.WriteString("**This register's source of truth is the per-entry files under\n")
	b.WriteString("`docs/streams/intake/`.** This `.md` file is a generated view (single-writer,\n")
	b.WriteString("main CI only — same discipline as `STATUS.md`). Do not edit it by hand.\n")
	b.WriteString("Deleting an entry file is a tombstone-not-delete violation and trips `--lint`.\n\n")

	// Decision queue section — pointer into the needs-decision queue.
	// Renders at the top because these are the humans-blocking list.
	if len(decisionEntries) > 0 {
		b.WriteString("## Decision queue — waiting on a human\n\n")
		b.WriteString("Entries flagged `decision-needed` — decisions routed through the single\n")
		b.WriteString("`needs-decision` GitHub issue queue. Each entry must link\n")
		b.WriteString("its decision issue below; this section is a pointer into that queue, not a\n")
		b.WriteString("second decision queue.\n\n")
		for _, e := range decisionEntries {
			b.WriteString(fmt.Sprintf("### %s — %s — %s\n", e.ID, e.Date, e.Title))
			if e.Body != "" {
				b.WriteString("\n")
				b.WriteString(rebaseEntryBodyLinks(e.Body, "intake"))
				b.WriteString("\n")
			}
			b.WriteString("\n")
			disp := "decision-needed"
			if e.DecisionIssue != "" {
				disp = fmt.Sprintf("decision-needed → issue #%s", e.DecisionIssue)
			}
			b.WriteString(fmt.Sprintf("Disposition: %s\n\n", disp))
		}
	}

	// Remaining entries (new, watching, scoped, rejected — everything that is not decision-needed).
	for _, e := range otherEntries {
		b.WriteString(fmt.Sprintf("## %s — %s — %s\n", e.ID, e.Date, e.Title))
		if e.Body != "" {
			b.WriteString("\n")
			b.WriteString(rebaseEntryBodyLinks(e.Body, "intake"))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		disp := e.Disposition
		switch disp {
		case "scoped":
			if e.ScopedTo != "" {
				disp = fmt.Sprintf("scoped → %s", e.ScopedTo)
			}
		case "rejected":
			if e.Why != "" {
				disp = fmt.Sprintf("rejected — %s", e.Why)
			}
		}
		b.WriteString(fmt.Sprintf("Disposition: %s\n\n", disp))
	}
	return b.String(), nil
}

// generateFindingsView renders docs/streams/FINDINGS.md from the per-entry files.
// Output is byte-deterministic (entries sorted by date, then id). Returns "" when there
// are no entry files (empty register — don't write a header-only file).
func generateFindingsView(root string) (string, error) {
	entries, err := parseFindingsDir(root)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString("# Findings\n\n")
	b.WriteString("Append-only log of knowledge that invalidates or updates existing briefs. Each entry\n")
	b.WriteString("names the briefs it affects; statusgen flags affected briefs ⚠ stale-knowledge (and\n")
	b.WriteString("excludes them from Next-up) until the finding is `Resolved: yes`. Resolve a finding by\n")
	b.WriteString("updating the affected brief/README to reflect it, then flipping the flag.\n\n")
	b.WriteString("Format: each entry is a per-entry file under `docs/streams/findings/` with YAML\n")
	b.WriteString("frontmatter (id, date, title, affects, ack, resolved) and a body paragraph.\n")
	b.WriteString("**IDs:** letter-prefixed slugs — `F-<slug>` where the slug is\n")
	b.WriteString("10–20 characters, `[a-z0-9-]`, derived from the title (example: `F-ws-token-expiry`).\n")
	b.WriteString("Older entries use numeric IDs (`F-01`) — those are frozen legacy and remain valid.\n")
	b.WriteString("New entries must use the slug form; a numeric ID on a new entry triggers a lint PROBLEM.\n")
	b.WriteString("**Tamper-wire:** sequential IDs carried a contiguity check to detect deletions — slugs\n")
	b.WriteString("trade contiguity for no-collision parallel write. Entry-file deletion is PR-diff-visible,\n")
	b.WriteString("this view is single-writer on main (a deletion shows as a view diff in the regen commit),\n")
	b.WriteString("and the tombstone rule (amend `resolved: yes`, never delete the file) stays.\n\n")
	b.WriteString("This `.md` file is a generated view (single-writer, main CI only — same discipline as\n")
	b.WriteString("`STATUS.md`). Do not edit it by hand. Deleting an entry file is a\n")
	b.WriteString("tombstone-not-delete violation and trips `--lint`.\n\n")

	for _, e := range entries {
		b.WriteString(fmt.Sprintf("## %s — %s — %s\n", e.ID, e.Date, e.Title))
		if e.Body != "" {
			b.WriteString("\n")
			b.WriteString(rebaseEntryBodyLinks(e.Body, "findings"))
			b.WriteString("\n")
		}
		if len(e.Affects) > 0 {
			b.WriteString(fmt.Sprintf("\nAffects: %s\n", strings.Join(e.Affects, ", ")))
		}
		if e.Ack != "" {
			b.WriteString(fmt.Sprintf("Ack: %s\n", e.Ack))
		}
		if e.Resolved {
			b.WriteString("Resolved: yes\n")
		} else {
			b.WriteString("Resolved: no\n")
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

// intakeDecisionIssueNotices returns NOTICEs for decision-needed intake entries
// that lack a decision-issue field. Advisory only — exit code unchanged (the entry
// may have been flipped moments before the issue is filed). The convention is that
// every decision-needed entry links a needs-decision GitHub issue via decision-issue
// in the frontmatter, routing into the SINGLE decision queue.
func intakeDecisionIssueNotices(entries []intakeEntry) []string {
	var out []string
	for _, e := range entries {
		if e.Disposition != "decision-needed" {
			continue
		}
		if e.DecisionIssue == "" {
			out = append(out, fmt.Sprintf(
				"intake entry %s: disposition is decision-needed but has no decision-issue — file a needs-decision issue and add 'decision-issue: <NN>' to the frontmatter",
				e.ID))
		}
	}
	return out
}

// ----- write-mode hooks -----

// writeRegisterViews regenerates INTAKE.md and FINDINGS.md from per-entry files.
// Called by main CI write mode only — never on a branch (single-writer discipline).
// Missing or empty register directories are not an error.
func writeRegisterViews(root string) error {
	// Intake — skip if no per-entry files exist.
	intakeView, err := generateIntakeView(root)
	if err != nil {
		return fmt.Errorf("generating INTAKE.md view: %w", err)
	}
	if intakeView != "" {
		intakePath := filepath.Join(root, "docs", "streams", "INTAKE.md")
		if err := os.WriteFile(intakePath, []byte(intakeView), 0o644); err != nil {
			return fmt.Errorf("writing INTAKE.md: %w", err)
		}
	}

	// Findings — skip if no per-entry files exist.
	findingsView, err := generateFindingsView(root)
	if err != nil {
		return fmt.Errorf("generating FINDINGS.md view: %w", err)
	}
	if findingsView != "" {
		findingsPath := filepath.Join(root, "docs", "streams", "FINDINGS.md")
		if err := os.WriteFile(findingsPath, []byte(findingsView), 0o644); err != nil {
			return fmt.Errorf("writing FINDINGS.md: %w", err)
		}
	}
	return nil
}

// checkRegisterViews verifies the committed INTAKE.md/FINDINGS.md byte-match
// a fresh regen from the per-entry files (--check mode). Skips registers with
// no per-entry files (the commit may still have the old single-file format).
func checkRegisterViews(root string) []string {
	var problems []string

	intakeView, err := generateIntakeView(root)
	if err != nil {
		return []string{fmt.Sprintf("register view check: %v", err)}
	}
	if intakeView != "" {
		existing, err := os.ReadFile(filepath.Join(root, "docs", "streams", "INTAKE.md"))
		if err != nil {
			return []string{fmt.Sprintf("register view check: cannot read INTAKE.md: %v", err)}
		}
		if string(existing) != intakeView {
			problems = append(problems, "INTAKE.md is out of date — run: go run ./tools/statusgen")
		}
	}

	findingsView, err := generateFindingsView(root)
	if err != nil {
		return append(problems, fmt.Sprintf("register view check: %v", err))
	}
	if findingsView != "" {
		existing, err := os.ReadFile(filepath.Join(root, "docs", "streams", "FINDINGS.md"))
		if err != nil {
			return append(problems, fmt.Sprintf("register view check: cannot read FINDINGS.md: %v", err))
		}
		if string(existing) != findingsView {
			problems = append(problems, "FINDINGS.md is out of date — run: go run ./tools/statusgen")
		}
	}

	return problems
}
