package main

// The changelog-fragment aggregator: the release-time engine, ported from
// tools/changelog/aggregate.py (which is now an adapter onto these verbs).
//
// A notable change is recorded as one fragment file per pull request under changelog/
// (changelog/<slug>.md), never as a hand-edit of CHANGELOG.md's `## Unreleased` section.
// This file AGGREGATES the fragments (sorted, de-duplicated) into the release body's
// highlights and into a dated CHANGELOG.md section, and it is the shared parser the
// pull-request gate reuses. Buckets are the Keep-a-Changelog trio Added / Fixed / Changed;
// a bullet with no bucket heading above it is Changed. Emission order is Added, Fixed,
// Changed. Residual `## Unreleased` bullets are folded into the aggregate (the cutover fold).
//
// Text rules follow the Python engine exactly: a file is read as strict UTF-8 and split at
// every Python line boundary, and "whitespace" is Python's (unicode.IsSpace plus the four
// information separators U+001C..U+001F).

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

var buckets = []string{"Added", "Fixed", "Changed"}

const defaultBucket = "Changed"

// creditSuffix is appended to each bullet of a credited fragment.
const creditSuffix = " — thanks %s"

// unresolved marks a fragment whose pull request could not be resolved confidently.
const unresolved = "unresolved"

// The landing-commit subject shapes: a merge commit's `Merge pull request #N `, and a squash
// commit's trailing `(#N)`. \p{Nd} is the digit class Python's \d matches.
var (
	mergePRRe  = regexp.MustCompile(`^Merge pull request #(\p{Nd}+) `)
	squashPRRe = regexp.MustCompile(`\(#(\p{Nd}+)\)\s*$`)
)

// pointerLines is the standing note left in `## Unreleased` after a roll.
var pointerLines = []string{
	"Pending notable changes are recorded as one-file-per-PR fragments under",
	"`changelog/` (see `changelog/README.md`), aggregated into a dated section",
	"here at release time. This section is written only by the release workflow;",
	"do not add highlight bullets to it directly.",
}

const refuseEmptyMsg = "changelog: nothing to aggregate — no fragment files under the " +
	"changelog/ directory and no residual '## Unreleased' bullets. A " +
	"release must carry descriptive highlights; add a fragment " +
	"(changelog/<slug>.md) before cutting, or the release is refused " +
	"(consistent with the retired empty-'## Unreleased' refusal).\n"

// pySpace is Python's str.isspace for one rune.
func pySpace(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }

func pyStrip(s string) string  { return strings.TrimFunc(s, pySpace) }
func pyLStrip(s string) string { return strings.TrimLeftFunc(s, pySpace) }

// lowerIs reports whether Python's s.lower() equals the ASCII target. U+0130 is the one
// rune whose Python lower-case mapping (two runes) differs from Go's (one ASCII rune).
func lowerIs(s, target string) bool {
	return !strings.ContainsRune(s, 'İ') && strings.ToLower(s) == target
}

// splitLines is Python's str.splitlines() (universal-newline reading folds \r\n and \r into
// \n first, which splitlines treats identically).
func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			out = append(out, s[start:i])
			i += size
			start = i
		case '\r':
			out = append(out, s[start:i])
			i += size
			if i < len(s) && s[i] == '\n' {
				i++
			}
			start = i
		default:
			i += size
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// errUndecodable is a file that is not UTF-8.
var errUndecodable = errors.New("not valid UTF-8")

// readLines reads a UTF-8 text file into its lines.
func readLines(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("%s: %w", path, errUndecodable)
	}
	return splitLines(string(b)), nil
}

// headingBucket matches `^\s*###\s+(.+?)\s*$` and returns the stripped heading text.
func headingBucket(ln string) (string, bool) {
	rest, ok := strings.CutPrefix(pyLStrip(ln), "###")
	if !ok || utf8.RuneCountInString(rest) < 2 {
		return "", false
	}
	if r, _ := utf8.DecodeRuneInString(rest); !pySpace(r) {
		return "", false
	}
	return pyStrip(rest), true
}

// isSection matches `^##\s+`.
func isSection(ln string) bool {
	rest, ok := strings.CutPrefix(ln, "##")
	if !ok || rest == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return pySpace(r)
}

// isBullet matches `^\s*-\s+\S`.
func isBullet(ln string) bool {
	rest, ok := strings.CutPrefix(pyLStrip(ln), "-")
	if !ok || rest == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return pySpace(r) && pyStrip(rest) != ""
}

func canonicalBucket(name string) string {
	n := pyStrip(name)
	for _, b := range buckets {
		if lowerIs(n, strings.ToLower(b)) {
			return b
		}
	}
	return ""
}

type entry struct{ bucket, text string }

// parseBullets parses a run of markdown lines into (bucket, entry) pairs in source order.
func parseBullets(lines []string) []entry {
	var out []entry
	bucket := defaultBucket
	current := -1
	for _, ln := range lines {
		if name, ok := headingBucket(ln); ok {
			if canon := canonicalBucket(name); canon != "" {
				bucket = canon
			}
			current = -1
			continue
		}
		if isSection(ln) {
			current = -1
			continue
		}
		if isBullet(ln) {
			out = append(out, entry{bucket, pyStrip(ln)})
			current = len(out) - 1
			continue
		}
		if current >= 0 && pyStrip(ln) != "" && (strings.HasPrefix(ln, " ") || strings.HasPrefix(ln, "\t")) {
			out[current].text += "\n  " + pyStrip(ln)
			continue
		}
		current = -1
	}
	return out
}

func isUnreleasedHeading(ln string) bool { return lowerIs(pyStrip(ln), "## unreleased") }

// unreleasedSlice is the lines strictly inside `## Unreleased`.
func unreleasedSlice(lines []string) []string {
	var out []string
	capturing := false
	for _, ln := range lines {
		if !capturing && isUnreleasedHeading(ln) {
			capturing = true
			continue
		}
		if capturing && isSection(ln) {
			break
		}
		if capturing {
			out = append(out, ln)
		}
	}
	return out
}

// unreleasedEntries is the bullet entries under `## Unreleased` in text, each on one line.
func unreleasedEntries(lines []string) []string {
	var out []string
	for _, e := range parseBullets(unreleasedSlice(lines)) {
		out = append(out, strings.ReplaceAll(e.text, "\n", " "))
	}
	return out
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// fragmentNames is every <dir>/*.md except README.md, sorted.
func fragmentNames(dir string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if n := e.Name(); strings.HasSuffix(n, ".md") && n != "README.md" {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// validLogin is `^@[A-Za-z0-9](?:[A-Za-z0-9]|-(?=[A-Za-z0-9])){0,38}$`.
func validLogin(v string) bool {
	s, ok := strings.CutPrefix(v, "@")
	if !ok || len(s) < 1 || len(s) > 39 {
		return false
	}
	alnum := func(c byte) bool { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }
	for i := 0; i < len(s); i++ {
		switch {
		case alnum(s[i]):
		case s[i] == '-' && i > 0 && i+1 < len(s) && alnum(s[i+1]):
		default:
			return false
		}
	}
	return true
}

// loadCredits reads a credits map into {fragment: suffix}. Missing or unreadable is empty:
// a credit must never be able to fail a release.
func loadCredits(path string) map[string]string {
	out := map[string]string{}
	if path == "" {
		return out
	}
	lines, err := readLines(path)
	if err != nil {
		return out
	}
	for _, ln := range lines {
		if pyStrip(ln) == "" || strings.HasPrefix(pyLStrip(ln), "#") {
			continue
		}
		parts := strings.Split(ln, "\t")
		if len(parts) < 2 {
			continue
		}
		name, value := pyStrip(parts[0]), pyStrip(parts[1])
		if name == "" || !validLogin(value) {
			continue
		}
		out[name] = fmt.Sprintf(creditSuffix, value)
	}
	return out
}

// creditEntry appends suffix to the bullet line of a possibly multi-line entry.
func creditEntry(e, suffix string) string {
	head, tail, found := strings.Cut(e, "\n")
	if !found {
		return head + suffix
	}
	return head + suffix + "\n" + tail
}

// collect gathers bucket -> sorted unique entries from every fragment plus the fold.
func collect(dir, changelog string, credits map[string]string) (map[string][]string, int, error) {
	all := map[string][]string{}
	for _, name := range fragmentNames(dir) {
		lines, err := readLines(filepath.Join(dir, name))
		if err != nil {
			return nil, 0, err
		}
		suffix := credits[name]
		for _, e := range parseBullets(lines) {
			t := e.text
			if suffix != "" {
				t = creditEntry(t, suffix)
			}
			all[e.bucket] = append(all[e.bucket], t)
		}
	}
	if exists(changelog) {
		lines, err := readLines(changelog)
		if err != nil {
			return nil, 0, err
		}
		for _, e := range parseBullets(unreleasedSlice(lines)) {
			all[e.bucket] = append(all[e.bucket], e.text)
		}
	}
	result := map[string][]string{}
	total := 0
	for _, b := range buckets {
		seen := map[string]bool{}
		var uniq []string
		for _, e := range all[b] {
			if !seen[e] {
				seen[e] = true
				uniq = append(uniq, e)
			}
		}
		sort.Strings(uniq)
		if len(uniq) > 0 {
			result[b] = uniq
			total += len(uniq)
		}
	}
	return result, total, nil
}

func render(c map[string][]string) string {
	var blocks []string
	for _, b := range buckets {
		if len(c[b]) == 0 {
			continue
		}
		blocks = append(blocks, strings.Join(append([]string{"### " + b}, c[b]...), "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func runUnreleased(changelog string, stdout, stderr io.Writer) int {
	if !exists(changelog) {
		return 0
	}
	lines, err := readLines(changelog)
	if err != nil {
		fmt.Fprintf(stderr, "deskchangelog: %v\n", err)
		return 1
	}
	for _, e := range unreleasedEntries(lines) {
		fmt.Fprintln(stdout, e)
	}
	return 0
}

func runHighlights(dir, changelog, creditsPath string, stdout, stderr io.Writer) int {
	c, total, err := collect(dir, changelog, loadCredits(creditsPath))
	if err != nil {
		fmt.Fprintf(stderr, "deskchangelog: %v\n", err)
		return 1
	}
	if total == 0 {
		fmt.Fprint(stderr, refuseEmptyMsg)
		return 2
	}
	fmt.Fprint(stdout, render(c)+"\n")
	return 0
}

func runRoll(dir, changelog, tag, date, creditsPath string, stderr io.Writer) int {
	c, total, err := collect(dir, changelog, loadCredits(creditsPath))
	if err != nil {
		fmt.Fprintf(stderr, "deskchangelog: %v\n", err)
		return 1
	}
	if total == 0 {
		fmt.Fprint(stderr, refuseEmptyMsg)
		return 2
	}
	highlights := render(c)
	lines, err := readLines(changelog)
	if err != nil {
		fmt.Fprintf(stderr, "deskchangelog: %v\n", err)
		return 1
	}
	idx := -1
	for i, ln := range lines {
		if isUnreleasedHeading(ln) {
			idx = i
			break
		}
	}
	if idx < 0 {
		fmt.Fprintf(stderr, "changelog: no '## Unreleased' heading in %s — refusing to guess where the dated section goes.\n", changelog)
		return 2
	}
	j := idx + 1
	for j < len(lines) && !isSection(lines[j]) {
		j++
	}
	rebuilt := append([]string{}, lines[:idx+1]...)
	rebuilt = append(rebuilt, "")
	rebuilt = append(rebuilt, pointerLines...)
	rebuilt = append(rebuilt, "", fmt.Sprintf("## %s — %s", tag, date), "", highlights, "")
	rebuilt = append(rebuilt, lines[j:]...)
	out := strings.TrimRight(strings.Join(rebuilt, "\n"), "\n") + "\n"
	if err := os.WriteFile(changelog, []byte(out), 0o644); err != nil {
		fmt.Fprintf(stderr, "deskchangelog: %v\n", err)
		return 1
	}
	return 0
}

// gitIn runs `git -C repo args...` and returns its stdout whatever the exit status, with
// stderr discarded: an unresolvable credit is a missing name, never a refused release.
func gitIn(repo string, args ...string) (string, bool) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var out strings.Builder
	cmd.Stdout = &out
	err := cmd.Run()
	return strings.ReplaceAll(out.String(), "\r\n", "\n"), err == nil
}

// prFromSubject is the pull-request number in a landing commit's subject, or "".
func prFromSubject(subject string) string {
	if subject == "" {
		return ""
	}
	if m := mergePRRe.FindStringSubmatch(subject); m != nil {
		return m[1]
	}
	if m := squashPRRe.FindStringSubmatch(subject); m != nil {
		return m[1]
	}
	return ""
}

// resolveFragmentPR walks adding commit -> its own subject -> the first first-parent merge
// whose second parent contains it. Order and containment are both load-bearing.
func resolveFragmentPR(repo, rel string) string {
	adds, _ := gitIn(repo, "log", "--diff-filter=A", "--format=%H", "--", rel)
	var shas []string
	for _, ln := range splitLines(adds) {
		if s := pyStrip(ln); s != "" {
			shas = append(shas, s)
		}
	}
	if len(shas) == 0 {
		return ""
	}
	adding := shas[len(shas)-1]
	if own, _ := gitIn(repo, "log", "-1", "--format=%s", adding); own != "" {
		if pr := prFromSubject(pyStrip(own)); pr != "" {
			return pr
		}
	}
	merges, _ := gitIn(repo, "log", "--first-parent", "--ancestry-path", "--merges", "--reverse", "--format=%H%x09%s", adding+"..HEAD")
	for _, ln := range splitLines(merges) {
		if pyStrip(ln) == "" {
			continue
		}
		sha, subject, _ := strings.Cut(ln, "\t")
		sha = pyStrip(sha)
		if sha == "" {
			continue
		}
		if _, ok := gitIn(repo, "merge-base", "--is-ancestor", adding, sha+"^2"); !ok {
			continue
		}
		if pr := prFromSubject(pyStrip(subject)); pr != "" {
			return pr
		}
	}
	return ""
}

// realAbs is the absolute, symlink-resolved form of p (p itself when it cannot be resolved).
func realAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if r, err := filepath.EvalSymlinks(a); err == nil {
		return r
	}
	return a
}

func runCredits(dir string, stdout io.Writer) int {
	names := fragmentNames(dir)
	if len(names) == 0 {
		fmt.Fprintf(stdout, "%s\t%s\n", dir, unresolved)
		return 0
	}
	top, _ := gitIn(dir, "rev-parse", "--show-toplevel")
	root := pyStrip(top)
	for _, name := range names {
		pr := ""
		if root != "" {
			if rel, err := filepath.Rel(realAbs(root), filepath.Join(realAbs(dir), name)); err == nil {
				pr = resolveFragmentPR(root, filepath.ToSlash(rel))
			}
		}
		if pr == "" {
			pr = unresolved
		}
		fmt.Fprintf(stdout, "%s\t%s\n", name, pr)
	}
	return 0
}

// aggregateCommands are the four aggregate.py verbs.
func aggregateCommands(stdout, stderr io.Writer) []*cobra.Command {
	unrel := &cobra.Command{
		Use:   "unreleased-bullets <changelog>",
		Short: "print the bullet entries under '## Unreleased', one per line",
		Long: `Prints, one per line, the normalized bullet entries under '## Unreleased' in
<changelog> (a multi-line entry on one line). A missing file prints nothing. The
changelog gate uses this to detect a pull request that ADDS such a bullet.

Exit codes: 0 ok · 1 unreadable or non-UTF-8 file · 2 usage.`,
		Args: argsBetween(1, 1),
	}
	unrelSet := cli.Declare(unrel, creditsBinding(true))
	unrel.RunE = func(_ *cobra.Command, a []string) error {
		if _, err := resolve(unrelSet); err != nil {
			return err
		}
		return codeErr(runUnreleased(a[0], stdout, stderr))
	}

	hl := &cobra.Command{
		Use:   "highlights <fragment-dir> <changelog>",
		Short: "print the aggregated highlights as '### <bucket>' blocks",
		Long: `Prints the aggregated release highlights: every <fragment-dir>/*.md except
README.md, plus any residual '## Unreleased' bullets in <changelog>, as
Keep-a-Changelog '### Added' / '### Fixed' / '### Changed' blocks, each bucket
sorted and de-duplicated. With --credits, every bullet of a credited fragment
gains the credit suffix; a missing map credits nobody and changes nothing.

Exit codes: 0 ok · 1 an unreadable or non-UTF-8 input · 2 usage, or nothing to
aggregate (a release must carry highlights).`,
		Args: argsBetween(2, 2),
	}
	hlSet := cli.Declare(hl, creditsBinding(false))
	hl.RunE = func(_ *cobra.Command, a []string) error {
		v, err := resolve(hlSet)
		if err != nil {
			return err
		}
		return codeErr(runHighlights(a[0], a[1], v.String("credits"), stdout, stderr))
	}

	roll := &cobra.Command{
		Use:   "roll <fragment-dir> <changelog> <tag> <date>",
		Short: "write the dated CHANGELOG.md section from the fragments",
		Long: `Rewrites <changelog> in place: the '## Unreleased' body becomes the standing
pointer note, and a fresh '## <tag> — <date>' section carrying the aggregated
highlights is inserted beneath it. The caller removes the aggregated fragment
files in the same commit.

Exit codes: 0 written · 1 an unreadable, non-UTF-8 or unwritable input · 2 usage,
nothing to aggregate, or no '## Unreleased' heading (the file is left untouched).`,
		Args: argsBetween(4, 4),
	}
	rollSet := cli.Declare(roll, creditsBinding(false))
	roll.RunE = func(_ *cobra.Command, a []string) error {
		v, err := resolve(rollSet)
		if err != nil {
			return err
		}
		return codeErr(runRoll(a[0], a[1], a[2], a[3], v.String("credits"), stderr))
	}

	cr := &cobra.Command{
		Use:   "credits <fragment-dir>",
		Short: "print <fragment><TAB><pull-request-number|unresolved> per fragment",
		Long: `Prints one line per fragment: the pull request it arrived on, resolved from
local git history alone (the adding commit, then the merge that brought it in), or
'unresolved'. A missing or empty directory prints '<fragment-dir><TAB>unresolved'.
Needs full history: on a shallow clone every fragment is unresolved.

Exit codes: 0 always after parsing (a credit never fails a release) · 2 usage.`,
		Args: argsBetween(1, 1),
	}
	crSet := cli.Declare(cr, creditsBinding(true))
	cr.RunE = func(_ *cobra.Command, a []string) error {
		if _, err := resolve(crSet); err != nil {
			return err
		}
		return codeErr(runCredits(a[0], stdout))
	}
	return []*cobra.Command{unrel, hl, roll, cr}
}
