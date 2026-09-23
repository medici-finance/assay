package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// open.go — the --open orchestration: rewrite the ONE proven-stale Verify row, commit it on a
// `rebaseline/<stream>-<NN>-row-<K>` branch, and open a DRAFT PR through the sanctioned write
// verb `deskpr create`. The verb never pushes with raw git and never lands on main: deskpr
// does the push and PR under the loop identity ($DESK_LOOP — the verifier App under
// DESK_LOOP=verify-desk), running every gate (secret scan, self-containment, budget,
// push-transport, App attribution) the fleet's writes run through. A human merges it.

// idiomReplacement maps a retired tool idiom to its recorded replacement (facts.go decides
// WHEN a row is safe:idiom; this supplies the new form for the re-baseline edit). The one
// recorded ruling the brief names is the `--consumers` form.
//
// SCAFFOLDING: reachable only once gatherRowFacts sets RetiredIdiom, which it does not yet —
// safe:idiom is not produced by the shipped verb (see the SafeIdiom const). Kept for the
// tracked follow-up that wires the idiom facts.
var idiomReplacement = map[string]string{
	"--consumers": "--consumer",
}

// rebaselineBranch is the deterministic one-row branch name for row K of a brief file.
func rebaselineBranch(briefPath string, row int) string {
	stream, nn := streamNNFromPath(briefPath)
	return fmt.Sprintf("rebaseline/%s-%s-row-%d", stream, nn, row)
}

// streamNNFromPath derives (stream, NN) from a brief path like
// `docs/streams/<stream>/brief-<NN>-<slug>.md`. Falls back to ("brief", "0") if it cannot.
func streamNNFromPath(briefPath string) (stream, nn string) {
	base := filepath.Base(briefPath)
	dir := filepath.Base(filepath.Dir(briefPath))
	stream = dir
	if m := regexp.MustCompile(`brief-(\d+)-`).FindStringSubmatch(base); m != nil {
		nn = m[1]
	}
	if stream == "" || stream == "." {
		stream = "brief"
	}
	if nn == "" {
		nn = "0"
	}
	return stream, nn
}

// computeNewRow produces the re-baselined Command and Expect cells for a safe verdict. For a
// rename it substitutes the moved path; for a retired idiom it substitutes the recorded new
// form; for a count it re-measures the command's trailing integer. It returns changed=false
// when it cannot compute a concrete edit, so the caller refuses rather than committing a
// no-op re-baseline.
//
// Only the SafeRename arm is reachable in the shipped verb today: gatherRowFacts produces no
// safe:count or safe:idiom verdict (see classify.go's SafeCount/SafeIdiom consts), so the
// SafeCount and SafeIdiom arms below are scaffolding for the tracked follow-up that wires
// those facts. They are kept and correct so that follow-up need only supply the facts.
func computeNewRow(root string, row verifyRow, f RowFacts, cls Classification) (newCommand, newExpect string, changed bool) {
	newCommand, newExpect = row.Command, row.Expect
	switch cls.Verdict {
	case SafeRename:
		if f.PathRef == "" || f.RenameHop == "" {
			return "", "", false
		}
		newCommand = strings.ReplaceAll(newCommand, f.PathRef, f.RenameHop)
		newExpect = strings.ReplaceAll(newExpect, f.PathRef, f.RenameHop)
		return newCommand, newExpect, newCommand != row.Command || newExpect != row.Expect
	case SafeIdiom:
		repl, ok := idiomReplacement[f.RetiredIdiom]
		if !ok {
			return "", "", false
		}
		newCommand = strings.ReplaceAll(newCommand, f.RetiredIdiom, repl)
		return newCommand, newExpect, newCommand != row.Command
	case SafeCount:
		n, ok := measureTrailingInt(root, row.Command)
		if !ok {
			return "", "", false
		}
		re := regexp.MustCompile(`\d+`)
		if re.MatchString(newExpect) {
			newExpect = re.ReplaceAllString(newExpect, n)
			return newCommand, newExpect, newExpect != row.Expect
		}
		return "", "", false
	default:
		return "", "", false
	}
}

// measureTrailingInt runs command and returns the last integer token of its stdout — the
// current value of a count Expect. It is used only after facts.go has already proven the
// count drift is pure-additions (safe:count).
func measureTrailingInt(root, command string) (string, bool) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "KUBECONFIG=/dev/null") // offline envelope (C3); see runCommand
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	nums := regexp.MustCompile(`\d+`).FindAllString(string(out), -1)
	if len(nums) == 0 {
		return "", false
	}
	return nums[len(nums)-1], true
}

// applyRebaseline rewrites row K's line in content's `## Verify` table, substituting the new
// Command (and Expect) cells. It returns the new content and the old/new row lines for the PR
// body, or ok=false when the row line cannot be located or nothing changed.
//
// The scan is SCOPED to the `## Verify` section (verifySectionBounds), exactly as
// parseVerifyRows extracts that section before parsing. Scanning the whole brief would let an
// EARLIER pipe table with a numeric first column (a facts table, an example) whose first cell
// equals the row number capture the rewrite — a wrong-line edit (F-applyrebaseline-unscoped,
// PR #1511). Confining the match to the Verify section closes that: only the table the
// classifier decided on can be rewritten.
func applyRebaseline(content string, row verifyRow, newCommand, newExpect string) (newContent, oldLine, newLine string, ok bool) {
	lines := strings.Split(content, "\n")
	lo, hi := verifySectionBounds(lines)
	if lo < 0 {
		return "", "", "", false // no ## Verify section — nothing safely rewritable
	}
	for i := lo; i < hi; i++ {
		line := lines[i]
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cells := splitRow(line)
		if len(cells) == 0 {
			continue
		}
		if strings.TrimSpace(cells[0]) != fmt.Sprintf("%d", row.Num) {
			continue
		}
		// Substitute within the raw line so column layout and any surrounding backticks are
		// preserved: replace the old command/expect text where it appears.
		updated := line
		if newCommand != row.Command {
			updated = strings.Replace(updated, row.Command, newCommand, 1)
		}
		if newExpect != "" && newExpect != row.Expect {
			updated = strings.Replace(updated, row.Expect, newExpect, 1)
		}
		if updated == line {
			return "", "", "", false
		}
		oldLine, newLine = strings.TrimSpace(line), strings.TrimSpace(updated)
		lines[i] = updated
		return strings.Join(lines, "\n"), oldLine, newLine, true
	}
	return "", "", "", false
}

// evidenceDate returns the brief's authored/Evidence date for the PR body — best-effort from
// the frontmatter `authored:` line, else "unknown".
func evidenceDate(content string) string {
	if m := regexp.MustCompile(`(?m)^authored:\s*"?([0-9]{4}-[0-9]{2}-[0-9]{2})`).FindStringSubmatch(content); m != nil {
		return m[1]
	}
	return "unknown"
}

// openRebaselinePR performs the safe re-baseline: rewrite the row, commit it on the one-row
// branch, and open the draft PR via deskpr create. It makes NO raw git push.
func openRebaselinePR(root, repo string, lb loadedBrief, row verifyRow, f RowFacts, cls Classification) int {
	newCommand, newExpect, changed := computeNewRow(root, row, f, cls)
	if !changed {
		fmt.Fprintf(stderr, "deskrebaseline: %s classified safe but no concrete row edit could be computed — refusing rather than committing a no-op\n", cls.Verdict)
		return deskkit.ExitRefused
	}
	newContent, oldLine, newLine, ok := applyRebaseline(lb.Content, row, newCommand, newExpect)
	if !ok {
		fmt.Fprintf(stderr, "deskrebaseline: could not locate row %d's line to rewrite in %s\n", row.Num, lb.Path)
		return deskkit.ExitRefused
	}

	branch := rebaselineBranch(lb.Path, row.Num)
	if _, ok := gitOutput(root, "-C", root, "checkout", "-b", branch); !ok {
		fmt.Fprintf(stderr, "deskrebaseline: could not create branch %s (does it already exist?)\n", branch)
		return deskkit.ExitRefused
	}
	if err := os.WriteFile(lb.Path, []byte(newContent), 0o644); err != nil {
		fmt.Fprintf(stderr, "deskrebaseline: could not write re-baselined brief: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	rel, _ := filepath.Rel(root, lb.Path)
	if rel == "" {
		rel = lb.Path
	}
	if _, ok := gitOutput(root, "-C", root, "add", rel); !ok {
		fmt.Fprintf(stderr, "deskrebaseline: could not stage %s\n", rel)
		return deskkit.ExitUnverifiable
	}
	stream, nn := streamNNFromPath(lb.Path)
	commitMsg := fmt.Sprintf("chore(verify): re-baseline %s/%s row %d (%s)", stream, nn, row.Num, cls.Verdict)
	if _, ok := gitOutput(root, "-C", root, "commit", "-m", commitMsg); !ok {
		fmt.Fprintf(stderr, "deskrebaseline: commit failed\n")
		return deskkit.ExitUnverifiable
	}

	body, err := writePRBody(stream, nn, row.Num, cls, f, oldLine, newLine, evidenceDate(lb.Content))
	if err != nil {
		fmt.Fprintf(stderr, "deskrebaseline: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	defer os.Remove(body)

	title := fmt.Sprintf("re-baseline %s/%s Verify row %d (%s)", stream, nn, row.Num, cls.Verdict)
	dp := exec.Command("deskpr", "create", "--title", title, "--body-file", body, "--base", "main")
	dp.Dir = root
	dp.Stdout = stdout
	dp.Stderr = stderr
	if err := dp.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintf(stderr, "deskrebaseline: deskpr create could not run: %v (is it on PATH?)\n", err)
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}

// writePRBody mints a per-invocation temp file (worker kit §3) with the re-baseline PR body:
// the original Evidence date, the old and new row, the intactness evidence, and the Brief:
// trailer. It carries no path off this machine and no private identifier — the brief slug and
// the two row lines only — so it passes the public-repo self-containment scan deskpr runs.
func writePRBody(stream, nn string, row int, cls Classification, f RowFacts, oldLine, newLine, evDate string) (string, error) {
	tmp, err := os.CreateTemp("", "rebaseline-body.*.md")
	if err != nil {
		return "", fmt.Errorf("could not mint PR body file: %w", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "One-row re-baseline of a provably-intact-but-stale Verify row.\n\n")
	fmt.Fprintf(&b, "- Brief: %s/%s, Verify row %d\n", stream, nn, row)
	fmt.Fprintf(&b, "- Original Evidence date: %s\n", evDate)
	fmt.Fprintf(&b, "- Classification: `%s` — %s\n\n", cls.Verdict, cls.Reason)
	fmt.Fprintf(&b, "## Row change\n\n")
	fmt.Fprintf(&b, "Old:\n\n    %s\n\n", oldLine)
	fmt.Fprintf(&b, "New:\n\n    %s\n\n", newLine)
	fmt.Fprintf(&b, "## Intactness evidence\n\n")
	if f.PathRef != "" && f.RenameHop != "" {
		fmt.Fprintf(&b, "- Pinned path `%s` moved by a single rename hop to `%s` (git rename detection).\n", f.PathRef, f.RenameHop)
	}
	if f.RetiredIdiom != "" {
		fmt.Fprintf(&b, "- Row pins tool idiom `%s`, retired by a recorded ruling.\n", f.RetiredIdiom)
	}
	if cls.Verdict == SafeCount {
		fmt.Fprintf(&b, "- Count Expect drifted upward; target files changed only by additions since the Evidence date.\n")
	}
	fmt.Fprintf(&b, "\nThe verb classified this row against a narrow safe set and refuses anything it cannot prove intact; a reviewer and a human still gate this PR.\n\n")
	fmt.Fprintf(&b, "Brief: %s/%s\n", stream, nn)
	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		return "", err
	}
	return tmp.Name(), tmp.Close()
}
