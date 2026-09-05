package main

// newbrief — the brief-authoring FRONT DOOR (mistake-proofing/05, B1).
//
// A generator, not a blank file. Every field the brief format DERIVES is a field
// an author can no longer get wrong by typing it:
//
//   - every required frontmatter key is emitted (an empty value still carries its
//     key, because a MISSING key and an EMPTY key are different states and only one
//     is visible to an author reviewing the file);
//   - the gate is DERIVED by asking the four risk questions and computing the
//     conclusion — never accepted as a supplied value, and in non-interactive mode
//     an unanswered question is a REFUSAL, not a defaulted "no" (spec §1: the model
//     executor will not ask; a defaulted answer is the silent divergence that
//     produces the wrong gate);
//   - the wave is DERIVED from the declared dependencies (no deps → wave 0,
//     otherwise one more than the highest dependency wave), and a dependency that
//     does not exist is a refusal rather than a dangling edge;
//   - the INVERSE edge is WRITTEN, not checked: when the new brief declares a
//     dependency, the named brief's own `unblocks:` gains the new id in the SAME
//     change, atomically across every target — graph consistency becomes structural
//     rather than another lint;
//   - the freshness stamp is produced by a FETCH the tool performs, and on a failed
//     fetch it stamps NOTHING and says could-not-check — an absent stamp is honest,
//     an invented one is the defect.
//
// It deliberately does NOT remove any lint the earlier briefs in this stream add:
// the generator is a SOURCE-level device (it prevents the malformed document from
// existing) with exactly one bypass — hand-author the file — which must stay. The
// lint is the independent second layer, failing for a different reason (reading a
// finished file) in a different component (the pull-request gate). This file is not
// a licence to retire a check.
//
// Conventions inherited from the existing scaffolder (init.go), NOT its defects:
// it NEVER overwrites an existing file (each target created only if absent), and
// the identity is substituted in exactly one place so the name cannot drift between
// the directory, the frontmatter and the identifiers. init.go scaffolds a RETIRED
// register dialect; this is a sibling subcommand for BRIEFS and does not inherit or
// fix that — see the PR body.

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// newbrief exit codes — the three-state contract, and the same numbers, that
// conform/verifyrun/shardcheck use in this binary. A refusal (a supplied gate, an
// unanswered risk question, a nonexistent dependency, an unusable Verify command,
// any usage error) is could-not / could-not-check and exits 2; an I/O or
// atomic-write failure exits 1; a written brief exits 0.
const (
	newBriefExitOK     = 0
	newBriefExitWrite  = 1
	newBriefExitRefuse = 2
)

// newBriefFreshness performs the freshness fetch the tool stamps from. It is a
// package-level var so tests substitute a fake without a network call (the same
// seam gitinfo.go's listRemoteBranches uses); the default is defaultNewBriefFreshness.
var newBriefFreshness = defaultNewBriefFreshness

// defaultNewBriefFreshness fetches origin/main, resolves the commit, and returns
// the short sha + the commit's short date (YYYY-MM-DD). A NON-NIL error means the
// fetch/resolve failed — the caller MUST then stamp nothing and report
// could-not-check, never an invented value.
func defaultNewBriefFreshness(root string) (sha, date string, err error) {
	if out, ferr := exec.Command("git", "-C", root, "fetch", "origin", "main").CombinedOutput(); ferr != nil {
		return "", "", fmt.Errorf("`git fetch origin main` failed: %s", firstLine(string(out)))
	}
	shaOut, serr := exec.Command("git", "-C", root, "rev-parse", "--short", "refs/remotes/origin/main").Output()
	if serr != nil {
		return "", "", fmt.Errorf("`git rev-parse --short refs/remotes/origin/main` failed: %v", serr)
	}
	dateOut, derr := exec.Command("git", "-C", root, "show", "-s", "--format=%cs", "refs/remotes/origin/main").Output()
	if derr != nil {
		return "", "", fmt.Errorf("`git show -s --format=%%cs refs/remotes/origin/main` failed: %v", derr)
	}
	sha = strings.TrimSpace(string(shaOut))
	date = strings.TrimSpace(string(dateOut))
	if sha == "" || date == "" {
		return "", "", fmt.Errorf("origin/main resolved to an empty sha/date")
	}
	return sha, date, nil
}

// newBriefWriteFile is the file writer, a package-level var so a test can inject a
// mid-batch failure and assert the atomic rollback leaves no partial graph.
var newBriefWriteFile = os.WriteFile

// depList is a repeatable --depends flag: each occurrence adds one typed id.
type depList []string

func (d *depList) String() string { return strings.Join(*d, ", ") }
func (d *depList) Set(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return fmt.Errorf("empty --depends value")
	}
	*d = append(*d, v)
	return nil
}

// newBriefRow is one planned file write. origExisted records whether the target was
// present before the run, so a rollback can remove a newly-created file and restore
// a modified one to its exact prior bytes.
type newBriefRow struct {
	path        string
	content     string
	origExisted bool
	orig        []byte
}

var newBriefSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// newBriefSlug derives a filename slug from a title: lowercase, [a-z0-9] runs joined
// by a single '-'. An empty result falls back to "brief" so a title of pure
// punctuation still yields a legal filename.
func newBriefSlug(title string) string {
	s := newBriefSlugRe.ReplaceAllString(strings.ToLower(title), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "brief"
	}
	return s
}

// runNewBrief is the `statusgen newbrief` entry point. See the file header for the
// device it implements.
func runNewBrief(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("newbrief", flag.ContinueOnError)
	root := fs.String("root", ".", "repository root (the streams tree lives under docs/streams/)")
	stream := fs.String("stream", "", "the stream this brief belongs to (its docs/streams/<stream>/ must exist)")
	number := fs.String("number", "", "the brief number NN (default: the next free number in the stream)")
	title := fs.String("title", "", "the one-line brief title")
	effort := fs.String("effort", "M", "effort class: S, M or L")
	var deps depList
	fs.Var(&deps, "depends", "a typed dependency id <stream>/<NN> (repeatable); the wave is derived from these and the inverse edge is written into each")
	interactive := fs.Bool("interactive", false, "prompt for each of the four risk questions instead of taking them as flags")
	regulatory := fs.String("regulatory", "", "risk answer: yes|no (non-interactive mode; unanswered is a REFUSAL, never a default)")
	customer := fs.String("customer", "", "risk answer: yes|no")
	irreversible := fs.String("irreversible", "", "risk answer: yes|no")
	sensitiveData := fs.String("sensitive-data", "", "risk answer: yes|no")
	gate := fs.String("gate", "", "REFUSED: the gate is DERIVED from the risk answers, never supplied — this flag exists only to refuse a supplied value with a clear message")
	verifyCommand := fs.String("verify-command", "", "the first Verify row's command (optional; validated — an uncommand-spanned, untokenizable, or placeholder-carrying value is refused). Default: a `go test ./...` starter row")
	offline := fs.Bool("offline", false, "skip the freshness fetch (no stamp is written; use when there is deliberately no network)")
	dryRun := fs.Bool("dry-run", false, "print every file that would be written and its rendered body, and write NOTHING")
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return newBriefExitRefuse
	}

	// The gate is a CONCLUSION, never an input. There is no path that accepts a
	// supplied gate — the flag exists only so a supplied one gets a clear refusal
	// rather than being silently ignored.
	if strings.TrimSpace(*gate) != "" {
		fmt.Fprintln(stderr, "statusgen newbrief: --gate is not accepted — the gate is DERIVED from the four risk answers (any yes => human, all no => model). Answer the risk questions instead.")
		return newBriefExitRefuse
	}

	if strings.TrimSpace(*stream) == "" {
		fmt.Fprintln(stderr, "statusgen newbrief: --stream is required")
		return newBriefExitRefuse
	}
	if strings.TrimSpace(*title) == "" {
		fmt.Fprintln(stderr, "statusgen newbrief: --title is required")
		return newBriefExitRefuse
	}
	if !validEffort[*effort] {
		fmt.Fprintf(stderr, "statusgen newbrief: invalid --effort %q (want S, M or L)\n", *effort)
		return newBriefExitRefuse
	}

	streamDir := filepath.Join(*root, "docs", "streams", *stream)
	readmePath := filepath.Join(streamDir, "README.md")
	if _, err := os.Stat(readmePath); err != nil {
		fmt.Fprintf(stderr, "statusgen newbrief: stream %q has no README at %s — a brief is added to an existing stream (scaffold the stream with `statusgen init` first)\n", *stream, readmePath)
		return newBriefExitRefuse
	}

	// Risk answers → the four canonical questions. Non-interactive: one flag per
	// question, and an UNANSWERED question is a refusal. Interactive: prompt each.
	risk, code := newBriefRiskAnswers(*interactive, stdin, stdout, stderr,
		map[string]string{
			"regulatory":     *regulatory,
			"customer":       *customer,
			"irreversible":   *irreversible,
			"sensitive-data": *sensitiveData,
		})
	if code != newBriefExitOK {
		return code
	}
	// The gate is the conclusion of the four answers.
	derivedGate := "model"
	anyYes := false
	for _, k := range riskKeyOrder {
		if risk[k] == "yes" {
			anyYes = true
		}
	}
	if anyYes {
		derivedGate = "human"
	}

	// Resolve the number: the next free one in the stream, or an explicit one that
	// must be free (never overwrite an existing brief).
	num, code := newBriefResolveNumber(streamDir, *number, stderr)
	if code != newBriefExitOK {
		return code
	}
	slug := newBriefSlug(*title)
	briefPath := filepath.Join(streamDir, fmt.Sprintf("brief-%s-%s.md", num, slug))
	if _, err := os.Stat(briefPath); err == nil {
		fmt.Fprintf(stderr, "statusgen newbrief: %s already exists — newbrief never overwrites an existing file\n", briefPath)
		return newBriefExitRefuse
	}

	// Derive the wave from the declared dependencies, and refuse a dependency that
	// does not exist rather than emit a dangling edge. depPaths carries each dep's
	// resolved brief file, so the inverse edge is written into the right target.
	wave, depPaths, code := newBriefDeriveWave(*root, *stream, deps, stderr)
	if code != newBriefExitOK {
		return code
	}

	// The first Verify row: a supplied command is validated (step 7 — an unusable
	// row is not a Verify row); the default is a real, runnable starter row.
	verifyCmd := strings.TrimSpace(*verifyCommand)
	if verifyCmd == "" {
		verifyCmd = "go test ./..."
	} else if err := usableVerifyCommand(verifyCmd); err != nil {
		fmt.Fprintf(stderr, "statusgen newbrief: --verify-command is not a usable Verify row: %v\n", err)
		return newBriefExitRefuse
	}

	// The freshness stamp is produced by a fetch the tool performs. On failure it
	// stamps NOTHING and says could-not-check — never an invented value.
	var freshness string
	if *offline {
		fmt.Fprintln(stderr, "statusgen newbrief: --offline — no freshness stamp written")
	} else if sha, date, err := newBriefFreshness(*root); err != nil {
		fmt.Fprintf(stderr, "statusgen newbrief: could-not-check: freshness fetch failed (%v) — no stamp written (an absent stamp is honest; an invented one is the defect)\n", err)
	} else {
		freshness = fmt.Sprintf("freshness-checked %s @ %s (origin/main)", date, sha)
	}

	briefID := *stream + "/" + num
	body := renderNewBrief(newBriefSpec{
		id:        briefID,
		title:     *title,
		wave:      wave,
		depends:   []string(deps),
		effort:    *effort,
		gate:      derivedGate,
		risk:      risk,
		anyYes:    anyYes,
		stream:    *stream,
		freshness: freshness,
		verifyCmd: verifyCmd,
	})

	// Assemble every planned write: the new brief, the README row, and the inverse
	// edge into each named dependency — then commit them atomically (all or none).
	rows := []newBriefRow{{path: briefPath, content: body, origExisted: false}}

	readmeRaw, err := os.ReadFile(readmePath)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen newbrief: reading %s: %v\n", readmePath, err)
		return newBriefExitWrite
	}
	newReadme, err := insertBriefRow(string(readmeRaw), num, *title, slug, wave, *effort)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen newbrief: %s: %v\n", readmePath, err)
		return newBriefExitRefuse
	}
	rows = append(rows, newBriefRow{path: readmePath, content: newReadme, origExisted: true, orig: readmeRaw})

	for _, dp := range depPaths {
		depRaw, err := os.ReadFile(dp.path)
		if err != nil {
			fmt.Fprintf(stderr, "statusgen newbrief: reading dependency %s: %v\n", dp.path, err)
			return newBriefExitWrite
		}
		edited, err := addUnblocksEdge(string(depRaw), briefID)
		if err != nil {
			fmt.Fprintf(stderr, "statusgen newbrief: cannot write inverse edge into %s (%s): %v\n", dp.id, dp.path, err)
			return newBriefExitRefuse
		}
		rows = append(rows, newBriefRow{path: dp.path, content: edited, origExisted: true, orig: depRaw})
	}

	// Verify each target PARSES after the edit before writing anything — a brief
	// file that no longer parses, or a README whose table no longer reads, aborts
	// the whole change (write all or none).
	if code := newBriefValidatePlan(rows, stderr); code != newBriefExitOK {
		return code
	}

	if *dryRun {
		for _, r := range rows {
			verb := "would create"
			if r.origExisted {
				verb = "would update"
			}
			fmt.Fprintf(stdout, "%s  %s\n", verb, r.path)
			fmt.Fprintln(stdout, "----------------------------------------")
			fmt.Fprintln(stdout, r.content)
			fmt.Fprintln(stdout, "----------------------------------------")
		}
		return newBriefExitOK
	}

	if code := newBriefCommit(rows, stderr); code != newBriefExitOK {
		return code
	}

	fmt.Fprintf(stdout, "created  %s\n", briefPath)
	fmt.Fprintf(stdout, "updated  %s (brief %s row)\n", readmePath, num)
	for _, dp := range depPaths {
		fmt.Fprintf(stdout, "updated  %s (unblocks: += %s)\n", dp.path, briefID)
	}
	fmt.Fprintf(stdout, "\nbrief %s — gate: %s (derived from risk), wave: %d\n", briefID, derivedGate, wave)
	if freshness == "" && !*offline {
		fmt.Fprintln(stdout, "note: no freshness stamp (fetch could not be checked) — re-run with network to stamp it, or dereference-check the brief's Verify rows by hand.")
	}
	return newBriefExitOK
}

// riskKeyOrder is the canonical order of the four risk questions, so prompts and
// the emitted risk block are deterministic.
var riskKeyOrder = []string{"regulatory", "customer", "irreversible", "sensitive-data"}

// newBriefRiskAnswers resolves the four risk answers. Non-interactive: each must be
// "yes" or "no" — an empty (unanswered) one is a REFUSAL, because a defaulted
// answer is exactly the silent divergence a model executor produces under ambiguity.
// Interactive: prompt each question on stdout and read yes/no from stdin.
func newBriefRiskAnswers(interactive bool, stdin io.Reader, stdout, stderr io.Writer, flags map[string]string) (map[string]string, int) {
	out := map[string]string{}
	if interactive {
		r := bufio.NewReader(stdin)
		for _, k := range riskKeyOrder {
			ans, ok := promptYesNo(r, stdout, stderr, k)
			if !ok {
				fmt.Fprintf(stderr, "statusgen newbrief: no answer for risk question %q — the gate cannot be derived without all four answers\n", k)
				return nil, newBriefExitRefuse
			}
			out[k] = ans
		}
		return out, newBriefExitOK
	}
	for _, k := range riskKeyOrder {
		v := strings.TrimSpace(strings.ToLower(flags[k]))
		switch v {
		case "yes", "no":
			out[k] = v
		case "":
			fmt.Fprintf(stderr, "statusgen newbrief: risk question %q is unanswered — in non-interactive mode every risk question must be answered yes or no (the gate is DERIVED, never defaulted). Pass --%s yes|no or use --interactive.\n", k, k)
			return nil, newBriefExitRefuse
		default:
			fmt.Fprintf(stderr, "statusgen newbrief: risk question %q got %q — answer exactly yes or no\n", k, flags[k])
			return nil, newBriefExitRefuse
		}
	}
	return out, newBriefExitOK
}

// promptYesNo prompts one risk question until it reads yes/no, or the input closes.
func promptYesNo(r *bufio.Reader, stdout, stderr io.Writer, question string) (string, bool) {
	for {
		fmt.Fprintf(stdout, "risk — %s? [yes/no]: ", question)
		line, err := r.ReadString('\n')
		ans := strings.TrimSpace(strings.ToLower(line))
		switch ans {
		case "yes", "y":
			return "yes", true
		case "no", "n":
			return "no", true
		case "":
			if err != nil {
				return "", false // EOF with no answer
			}
			fmt.Fprintln(stderr, "  answer yes or no")
		default:
			if err != nil {
				return "", false
			}
			fmt.Fprintln(stderr, "  answer yes or no")
		}
	}
}

var newBriefNumRe = regexp.MustCompile(`^brief-([0-9]+[a-z]?)(?:-.*)?\.md$`)

// newBriefResolveNumber returns the brief number to use: an explicit one (validated
// to shape and required to be free) or the next free number in the stream.
func newBriefResolveNumber(streamDir, explicit string, stderr io.Writer) (string, int) {
	if e := strings.TrimSpace(explicit); e != "" {
		if !regexp.MustCompile(`^[0-9]+[a-z]?$`).MatchString(e) {
			fmt.Fprintf(stderr, "statusgen newbrief: --number %q is not a brief number (NN, e.g. 07 or 12a)\n", explicit)
			return "", newBriefExitRefuse
		}
		// Zero-pad a bare integer to two digits to match the corpus convention.
		if n, err := strconv.Atoi(e); err == nil && n < 10 && len(e) == 1 {
			e = fmt.Sprintf("%02d", n)
		}
		return e, newBriefExitOK
	}
	entries, err := os.ReadDir(streamDir)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen newbrief: reading %s: %v\n", streamDir, err)
		return "", newBriefExitWrite
	}
	max := 0
	for _, e := range entries {
		m := newBriefNumRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		// Strip an optional trailing letter suffix to read the numeric part.
		digits := strings.TrimRightFunc(m[1], func(r rune) bool { return r < '0' || r > '9' })
		if n, err := strconv.Atoi(digits); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%02d", max+1), newBriefExitOK
}

// depResolved is a named dependency resolved to its brief file and wave.
type depResolved struct {
	id   string
	path string
	wave int
}

// newBriefDeriveWave derives the wave from the declared dependencies and resolves
// each to its brief file for the inverse-edge write. No deps → wave 0. Otherwise
// one more than the highest dependency wave. A dependency that does not resolve to
// an existing brief file is a REFUSAL — never a dangling edge.
func newBriefDeriveWave(root, selfStream string, deps depList, stderr io.Writer) (int, []depResolved, int) {
	if len(deps) == 0 {
		return 0, nil, newBriefExitOK
	}
	var resolved []depResolved
	maxWave := -1
	for _, dep := range deps {
		parts := strings.SplitN(strings.TrimSpace(dep), "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			fmt.Fprintf(stderr, "statusgen newbrief: dependency %q is not a typed id <stream>/<NN>\n", dep)
			return 0, nil, newBriefExitRefuse
		}
		depStream, depNum := parts[0], parts[1]
		depDir := filepath.Join(root, "docs", "streams", depStream)
		path := findBriefFile(depDir, depNum)
		if path == "" {
			fmt.Fprintf(stderr, "statusgen newbrief: dependency %s does not exist (no brief-%s-*.md under docs/streams/%s/) — refusing rather than writing a dangling edge\n", dep, depNum, depStream)
			return 0, nil, newBriefExitRefuse
		}
		bf, ok, err := parseBriefFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "statusgen newbrief: dependency %s (%s) does not parse: %v\n", dep, path, err)
			return 0, nil, newBriefExitRefuse
		}
		if !ok {
			fmt.Fprintf(stderr, "statusgen newbrief: dependency %s (%s) is not a schema'd brief — cannot read its wave\n", dep, path)
			return 0, nil, newBriefExitRefuse
		}
		if bf.Wave > maxWave {
			maxWave = bf.Wave
		}
		resolved = append(resolved, depResolved{id: depStream + "/" + depNum, path: path, wave: bf.Wave})
	}
	return maxWave + 1, resolved, newBriefExitOK
}

// findBriefFile returns the brief file for number num in dir, or "" if absent.
func findBriefFile(dir, num string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var found []string
	for _, e := range entries {
		m := newBriefNumRe.FindStringSubmatch(e.Name())
		if m != nil && m[1] == num {
			found = append(found, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(found)
	if len(found) == 0 {
		return ""
	}
	return found[0]
}

// usableVerifyCommand reports whether a Verify command cell is a real, runnable row
// (step 7). It REUSES the lint's own tokenizer and placeholder detector rather than
// writing a second one: no code span → not a row; a code span that tokenizes to
// nothing → not a row; an unsubstituted placeholder → not a row.
func usableVerifyCommand(cell string) error {
	cmd := codeSpan(cell)
	if strings.TrimSpace(cmd) == "" {
		return fmt.Errorf("the command carries no code span (wrap it in backticks so a verifier can lift the literal command)")
	}
	toks := tokenizeCommand(cmd)
	nonOp := 0
	for _, t := range toks {
		if !t.op {
			nonOp++
		}
	}
	if nonOp == 0 {
		return fmt.Errorf("the code span %q tokenizes to no command word", cmd)
	}
	if mv := unsubstitutedMetavars(cmd); len(mv) > 0 {
		return fmt.Errorf("the command carries the unsubstituted placeholder(s) %s — substitute a concrete value or derive it in the command", strings.Join(mv, ", "))
	}
	return nil
}

// newBriefValidatePlan verifies every planned target PARSES after its edit, so a
// change that would corrupt a brief file or a README aborts before anything is
// written (write all or none). It validates from the in-memory content by staging
// each brief file to a temp path and re-parsing, and by re-parsing the README body.
func newBriefValidatePlan(rows []newBriefRow, stderr io.Writer) int {
	tmp, err := os.MkdirTemp("", "newbrief-validate-")
	if err != nil {
		fmt.Fprintf(stderr, "statusgen newbrief: cannot stage validation: %v\n", err)
		return newBriefExitWrite
	}
	defer os.RemoveAll(tmp)
	for _, r := range rows {
		base := filepath.Base(r.path)
		if base == "README.md" {
			if _, err := parseBriefTable(afterFrontmatter(r.content)); err != nil {
				fmt.Fprintf(stderr, "statusgen newbrief: the edited README table would not parse: %v — writing nothing\n", err)
				return newBriefExitRefuse
			}
			continue
		}
		staged := filepath.Join(tmp, base)
		if err := os.WriteFile(staged, []byte(r.content), 0o644); err != nil {
			fmt.Fprintf(stderr, "statusgen newbrief: cannot stage %s: %v\n", base, err)
			return newBriefExitWrite
		}
		if _, _, err := parseBriefFile(staged); err != nil {
			fmt.Fprintf(stderr, "statusgen newbrief: the edited brief %s would not parse: %v — writing nothing\n", r.path, err)
			return newBriefExitRefuse
		}
	}
	return newBriefExitOK
}

// afterFrontmatter returns the body of a README (everything after the frontmatter),
// tolerating a file with no frontmatter fence.
func afterFrontmatter(content string) string {
	if _, body, err := splitFrontmatter(content); err == nil {
		return body
	}
	return content
}

// newBriefCommit writes every planned row, rolling back on the first failure so a
// mid-write error leaves no partial graph. Rollback removes a newly-created file and
// restores a modified one to its exact prior bytes.
func newBriefCommit(rows []newBriefRow, stderr io.Writer) int {
	written := make([]newBriefRow, 0, len(rows))
	for _, r := range rows {
		if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
			newBriefRollback(written, stderr)
			fmt.Fprintf(stderr, "statusgen newbrief: %v — rolled back, wrote nothing\n", err)
			return newBriefExitWrite
		}
		if err := newBriefWriteFile(r.path, []byte(r.content), 0o644); err != nil {
			newBriefRollback(written, stderr)
			fmt.Fprintf(stderr, "statusgen newbrief: writing %s: %v — rolled back, wrote nothing\n", r.path, err)
			return newBriefExitWrite
		}
		written = append(written, r)
	}
	return newBriefExitOK
}

// newBriefRollback undoes the writes already made, newest first.
func newBriefRollback(written []newBriefRow, stderr io.Writer) {
	for i := len(written) - 1; i >= 0; i-- {
		r := written[i]
		var err error
		if r.origExisted {
			err = os.WriteFile(r.path, r.orig, 0o644)
		} else {
			err = os.Remove(r.path)
		}
		if err != nil {
			fmt.Fprintf(stderr, "statusgen newbrief: WARNING rollback of %s failed: %v\n", r.path, err)
		}
	}
}

// time source, a var so a test can pin the authored: date deterministically.
var newBriefNow = time.Now

// newBriefSpec carries the resolved inputs to renderNewBrief.
type newBriefSpec struct {
	id        string
	title     string
	wave      int
	depends   []string
	effort    string
	gate      string
	risk      map[string]string
	anyYes    bool
	stream    string
	freshness string
	verifyCmd string
}

// renderNewBrief emits the brief document: every required key present (empty values
// still carrying their key), the required body sections in order, the derived gate
// and wave, the gate-why / decision-section owed by a risk- or human-gated brief as
// empty-but-present, and the freshness stamp when the fetch produced one.
func renderNewBrief(s newBriefSpec) string {
	var b strings.Builder
	num := s.id
	if i := strings.LastIndex(s.id, "/"); i >= 0 {
		num = s.id[i+1:]
	}

	// depends: rendered as an inline typed-id list, [] when none.
	depItems := make([]string, 0, len(s.depends))
	for _, d := range s.depends {
		depItems = append(depItems, fmt.Sprintf("%q", strings.TrimSpace(d)))
	}
	dependsField := "[]"
	if len(depItems) > 0 {
		dependsField = "[" + strings.Join(depItems, ", ") + "]"
	}

	riskField := fmt.Sprintf("{regulatory: %s, customer: %s, irreversible: %s, sensitive-data: %s}",
		s.risk["regulatory"], s.risk["customer"], s.risk["irreversible"], s.risk["sensitive-data"])

	b.WriteString("---\n")
	b.WriteString("brief: " + s.id + "\n")
	b.WriteString("title: " + s.title + "\n")
	// why: emitted empty-but-present — REQUIRED for every new brief; the author
	// replaces it with one to three lines a non-engineer could justify the work from.
	b.WriteString("why: \"\"\n")
	b.WriteString(fmt.Sprintf("wave: %d\n", s.wave))
	b.WriteString("depends: " + dependsField + "\n")
	b.WriteString("unblocks: []\n")
	b.WriteString("effort: " + s.effort + "\n")
	b.WriteString("gate: " + s.gate + "\n")
	b.WriteString("risk: " + riskField + "\n")
	// gate-why: emitted empty-but-present ONLY for a risk-gated brief, which owes a
	// written rationale — an empty key an author sees and fills, never an omitted one.
	if s.anyYes || s.gate == "human" {
		b.WriteString("gate-why: \"\"\n")
	}
	b.WriteString("issues: []\n")
	b.WriteString("schema: brief-v1\n")
	b.WriteString("authored: " + newBriefNow().UTC().Format("2006-01-02") + " by statusgen newbrief\n")
	// sources: non-empty by construction (a stream provenance line), plus the
	// freshness stamp when the fetch produced one — never an invented value.
	b.WriteString("sources:\n")
	b.WriteString("  - \"" + s.stream + ": " + strings.ReplaceAll(s.title, "\"", "'") + "\"\n")
	if s.freshness != "" {
		b.WriteString("  - \"" + s.freshness + "\"\n")
	}
	b.WriteString("---\n\n")

	b.WriteString(fmt.Sprintf("# Brief %s — %s\n\n", num, s.title))

	b.WriteString("## Context\n")
	b.WriteString("files: \n")
	b.WriteString("facts:\n")
	b.WriteString("- \n\n")

	// A human-gated brief owes a self-contained decision section, emitted
	// empty-but-present so the author fills it rather than discovering it is missing.
	if s.gate == "human" {
		b.WriteString("## Human decision\n")
		b.WriteString("<!-- A human-gated brief owes a self-contained decision section: what must be\n")
		b.WriteString("     decided, the options, and the recommendation. Fill this before dispatch. -->\n\n")
	}

	b.WriteString("## Ground rules\n")
	b.WriteString("- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.\n")
	b.WriteString("- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).\n")
	b.WriteString("- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.\n\n")

	b.WriteString("## Task\n")
	b.WriteString("1. \n\n")

	b.WriteString("## Verify (executable — no prose-only DoD items)\n")
	b.WriteString("| # | Command | Expect |\n")
	b.WriteString("|---|---------|--------|\n")
	b.WriteString(fmt.Sprintf("| 1 | `%s` | exit 0 |\n", s.verifyCmd))
	b.WriteString("<!-- Obligation classes this brief's change SHAPE may owe (brief-rules.md):\n")
	b.WriteString("     - adds a CHECK/guard  -> a MUTATION-TEST row: break the guarded thing, confirm RED (rule 16, D1)\n")
	b.WriteString("     - a lister/flag/query used elsewhere -> a NEIGHBOUR row exercising the adjacent reader (rule 17)\n")
	b.WriteString("     - changes a value another component reads -> the consumers: field + a FLOW-level end-to-end row (rule 9)\n")
	b.WriteString("     - asserts a checkable FACT -> at least one DEREFERENCING row, not only presence counts (rule 43)\n")
	b.WriteString("     Presence is the control; adequacy stays the review gate (D7). -->\n\n")

	b.WriteString("## Evidence\n")
	b.WriteString("<!-- appended at implementation time by a NON-implementer: one row per Verify item\n")
	b.WriteString("     (command, exit code, output line(s) or hash, date, runner). -->\n\n")

	b.WriteString("## Review\n")
	b.WriteString(fmt.Sprintf("Gate: %s (from frontmatter — ", s.gate))
	if s.anyYes {
		b.WriteString("a risk answer is yes). Human gate is MANDATORY. ")
	} else {
		b.WriteString("all four risk answers no). ")
	}
	b.WriteString("Reviewer records verdict + date in the stream README table.\n")

	return b.String()
}

// insertBriefRow inserts a Briefs-table row for the new brief into a stream README,
// after the last contiguous row of the table. It refuses (error) when the README
// carries no recognizable Briefs table.
func insertBriefRow(readme, num, title, slug string, wave int, effort string) (string, error) {
	lines := strings.Split(readme, "\n")
	headerIdx := -1
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") {
			continue
		}
		lower := strings.ToLower(t)
		if strings.Contains(lower, "| brief ") && strings.Contains(lower, "| wave ") && strings.Contains(lower, "| status ") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return "", fmt.Errorf("no Briefs table found (a `| # | Brief | Wave | ... |` header)")
	}
	// The separator is headerIdx+1; rows run from headerIdx+2 while they start "|".
	last := headerIdx + 1
	for i := headerIdx + 2; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
			last = i
			continue
		}
		break
	}
	row := fmt.Sprintf("| %s | [%s](./brief-%s-%s.md) | %d | %s | todo | — | — |",
		num, title, num, slug, wave, effort)
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:last+1]...)
	out = append(out, row)
	out = append(out, lines[last+1:]...)
	return strings.Join(out, "\n"), nil
}

var inlineListRe = regexp.MustCompile(`^(\s*unblocks:\s*)\[(.*)\]\s*$`)

// addUnblocksEdge writes the inverse edge — it adds newID to the dependency brief's
// `unblocks:` list, in the SAME change, without a full YAML round-trip (which would
// reorder keys and drop comments). It handles the inline-list form (`unblocks: []`,
// `unblocks: ["x/01"]`) and the block form (a `- item` list under `unblocks:`). The
// edge is idempotent: an id already present is left unchanged. A missing unblocks:
// key is an error — a valid brief always carries one.
func addUnblocksEdge(content, newID string) (string, error) {
	fmRaw, _, err := splitFrontmatter(content)
	if err != nil {
		return "", fmt.Errorf("frontmatter: %v", err)
	}
	lines := strings.Split(content, "\n")
	// Locate the unblocks: line within the frontmatter (bounded by the second ---).
	fmEnd := 0
	seen := 0
	for i, l := range lines {
		if strings.TrimSpace(l) == "---" {
			seen++
			if seen == 2 {
				fmEnd = i
				break
			}
		}
	}
	if seen < 2 {
		return "", fmt.Errorf("unterminated frontmatter")
	}
	idx := -1
	for i := 1; i < fmEnd; i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "unblocks:") {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", fmt.Errorf("no unblocks: key in frontmatter")
	}
	_ = fmRaw

	// Inline-list form.
	if m := inlineListRe.FindStringSubmatch(lines[idx]); m != nil {
		items := parseInlineList(m[2])
		for _, it := range items {
			if unquote(it) == newID {
				return content, nil // already present — idempotent
			}
		}
		items = append(items, fmt.Sprintf("%q", newID))
		lines[idx] = m[1] + "[" + strings.Join(items, ", ") + "]"
		return strings.Join(lines, "\n"), nil
	}

	// Block-list form: `unblocks:` on its own, then `  - item` lines.
	if strings.TrimSpace(lines[idx]) == "unblocks:" {
		// Find the extent of the block list and its indentation.
		blockEnd := idx
		indent := "  - "
		for i := idx + 1; i < fmEnd; i++ {
			t := strings.TrimSpace(lines[i])
			if strings.HasPrefix(t, "- ") || t == "-" {
				if unquote(strings.TrimSpace(strings.TrimPrefix(t, "-"))) == newID {
					return content, nil // already present — idempotent
				}
				// Preserve the observed indentation.
				lead := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " "))]
				indent = lead + "- "
				blockEnd = i
				continue
			}
			break
		}
		newLine := indent + fmt.Sprintf("%q", newID)
		out := make([]string, 0, len(lines)+1)
		out = append(out, lines[:blockEnd+1]...)
		out = append(out, newLine)
		out = append(out, lines[blockEnd+1:]...)
		return strings.Join(out, "\n"), nil
	}

	return "", fmt.Errorf("unblocks: is neither an inline list nor a block list this tool can edit — add %q by hand", newID)
}

// parseInlineList splits the inside of a `[a, b]` list on top-level commas.
func parseInlineList(inner string) []string {
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(inner, ",") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// unquote strips a single layer of matching single/double quotes.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}
