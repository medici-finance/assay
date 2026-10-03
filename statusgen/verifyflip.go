package main

// verifyflip.go — `statusgen verifyflip`: the sanctioned implemented → verified
// README flip for a gate: model brief (#2074).
//
// THE GAP IT CLOSES. A gate: model brief whose Evidence records a passing
// verifier run still needs its stream-README row moved implemented → verified
// with a Verified stamp. Until now that row was hand-edited: a person or a
// session typed a stamp that was meant to match the recorded run, and nothing
// checked that it did. This verb DERIVES the stamp from the recorded run and
// refuses rather than guess.
//
// WHAT IT ACCEPTS — every condition below is fail-closed; a miss refuses with
// NO write:
//
//   - Brief file: resolved as `statusgen brief --check-verified` resolves it —
//     exactly one brief-<NN> file in the stream directory the key names, inside
//     the board that carries the row.
//   - Frontmatter: `gate: model`, and every `risk:` answer is `no`. A
//     `gate: human` brief is closed by the human sign-off path, never here; an
//     `irreversible: yes` (or any other `yes`) brief is never flipped by a model.
//   - Verify rows: none classed `gate:human` (a person decides that row) and
//     none of a class this binary does not know.
//   - Board row: Status `implemented`, Verified cell empty.
//   - Verdict: the LAST verdict token in Evidence (`VERIFY: <WORD>`, after the
//     usual quotation hygiene) is PASS, and it is the strict bold marker
//     (verifyVerdictBoldRe — the ratified gate regex). A later FAIL, BLOCKED or
//     PARTIAL, or a pass written in looser prose, refuses.
//   - Runner: the Date/Runner table rows recorded for THAT pass (the rows
//     between the previous verdict token and the PASS line) all carry one date
//     and one runner, the runner reads `<login> @ <sha> …`, and that login and
//     sha match what the caller expects (--runner, --sha). No Date/Runner row
//     follows the PASS. The date is a YYYY-MM-DD calendar date, not after today.
//   - Provenance: git blame owns every line the stamp is built from (the PASS
//     marker and those rows) to the roster's bound verifier or a roster human.
//     No roster, a shallow clone or a failed blame is could-not-check.
//   - No unrouted HELD/could-not-check line contradicts the pass, no FAIL is
//     left unanswered, and every Verify row has a passing execution witness
//     inside the PASS run itself.
//
// WHAT IT WRITES — exactly two cells of one README row: Status → `verified`
// and Verified → `<date> <runner>`, both from the recorded run. An
// `(on-behalf-of human:<name>)` qualifier on the runner stays in the Evidence
// and is left out of the stamp: verify-gate-close.yml is the sole writer of a
// `human:` token into a Verified cell, so any other one left refuses. The Reviewed
// cell and every other byte of the file are left as they were — checked after
// the rewrite against the header's own Status and Verified columns. The README
// must be a regular file, never a link. It never touches STATUS.md.
//
// It does not commit, push or run the lint; the deskevidence `flip` verb wraps
// it with the lint before/after and the verifier-identity commit on a branch.

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	verifyflipExitOK            = 0
	verifyflipExitRefused       = 1
	verifyflipExitCouldNotCheck = 2
)

const verifyflipUsage = `statusgen verifyflip — flip one gate: model brief implemented → verified, stamp derived from its recorded strict PASS

Usage:
  statusgen verifyflip --brief <stream>/<NN> --sha <sha> --runner <login> [--root <dir>] [--dry-run]

Writes Status → verified and the Verified cell → "<date> <runner>" taken from the
Date/Runner rows of the brief's latest strict **VERIFY: PASS**. Refuses (exit 1,
no write) on: gate: human, any risk: yes (irreversible included), a Verify row
classed gate:human or of an unknown class, a latest verdict that is not PASS, a
non-strict PASS, a recorded sha or runner that differs from --sha / --runner, rows
that disagree on date, sha or runner, rows after the PASS, a date that is not
YYYY-MM-DD or is after today, a PASS line or row not committed by the bound
verifier or a roster human, an unrouted HELD row, a Verify row without a passing
witness in the PASS run, or a README that is not a regular file. Exit 2 is
could-not-check (including an ambiguous brief file, no roster, a shallow clone).

Flags:
  --brief <stream>/<NN>   the brief key (required)
  --sha <sha>             the commit the recorded run must name (7–40 hex, required)
  --runner <login>        the runner login the recorded run must name (required)
  --root <dir>            repo root (default: the cwd)
  --dry-run               print the stamp, write nothing
`

// flipRefusal is a refusal (exit 1): the tree was read and does not license the
// flip. Any other error is could-not-check (exit 2).
type flipRefusal struct{ msg string }

func (e *flipRefusal) Error() string { return e.msg }

func refuseFlip(format string, a ...any) error {
	return &flipRefusal{fmt.Sprintf("refusing: "+format, a...)}
}

// verifyFlipPlan is the result of a successful plan: what would be written.
type verifyFlipPlan struct {
	Readme  string // path of the stream README
	Stamp   string // the Verified cell value
	Updated []byte // the README with the row flipped
}

var (
	// flipVerdictTokRe is a verdict token of ANY word. Wider than
	// verifyVerdictRe on purpose: BLOCKED and PARTIAL are verdicts here, so a
	// BLOCKED recorded after a PASS is the latest verdict and refuses.
	flipVerdictTokRe = regexp.MustCompile(`VERIFY:[ \t]*([A-Z][A-Z-]*)`)
	// flipRunnerRe reads `<login> @ <sha><rest>` from a Runner cell.
	flipRunnerRe = regexp.MustCompile(`^(\S+) @ ([0-9a-fA-F]{7,40})\b(.*)$`)
	flipShaRe    = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
	flipDateRe   = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

	// flipRewriteFn is the README rewrite (seam: the wiring test swaps in a
	// bad rewrite to prove planVerifyFlip's diff guard refuses it).
	flipRewriteFn = flipRowToVerified
	// flipToday is today's date, UTC, as YYYY-MM-DD.
	flipToday = func() string { return time.Now().UTC().Format("2006-01-02") }
)

// runVerifyflip is the `statusgen verifyflip` entry point.
func runVerifyflip(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("verifyflip", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {}
	briefKey := fs.String("brief", "", "")
	sha := fs.String("sha", "", "")
	runner := fs.String("runner", "", "")
	rootDir := fs.String("root", ".", "")
	dryRun := fs.Bool("dry-run", false, "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, verifyflipUsage)
			return verifyflipExitOK
		}
		fmt.Fprint(stderr, verifyflipUsage)
		return verifyflipExitCouldNotCheck
	}
	if fs.NArg() > 0 || *briefKey == "" || *sha == "" || *runner == "" {
		fmt.Fprint(stderr, verifyflipUsage)
		return verifyflipExitCouldNotCheck
	}
	if !flipShaRe.MatchString(*sha) {
		fmt.Fprintf(stderr, "statusgen verifyflip: --sha must be 7–40 hex characters, got %q\n", *sha)
		return verifyflipExitCouldNotCheck
	}

	plan, err := planVerifyFlip(*rootDir, *briefKey, *sha, *runner)
	if err != nil {
		var r *flipRefusal
		if errors.As(err, &r) {
			fmt.Fprintf(stderr, "statusgen verifyflip: %s: %v\n", *briefKey, err)
			return verifyflipExitRefused
		}
		fmt.Fprintf(stderr, "statusgen verifyflip: could-not-check — %s: %v\n", *briefKey, err)
		return verifyflipExitCouldNotCheck
	}
	rel, rerr := filepath.Rel(*rootDir, plan.Readme)
	if rerr != nil {
		rel = plan.Readme
	}
	fmt.Fprintf(stdout, "brief: %s\nreadme: %s\nverified-stamp: %s\n", *briefKey, filepath.ToSlash(rel), plan.Stamp)
	if *dryRun {
		fmt.Fprintln(stdout, "dry-run: nothing written")
		return verifyflipExitOK
	}
	info, err := os.Lstat(plan.Readme)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen verifyflip: could-not-check — %v\n", err)
		return verifyflipExitCouldNotCheck
	}
	if !info.Mode().IsRegular() {
		fmt.Fprintf(stderr, "statusgen verifyflip: %s: %v\n", *briefKey, refuseFlip("%s is not a regular file — the flip never writes through a link", plan.Readme))
		return verifyflipExitRefused
	}
	if err := os.WriteFile(plan.Readme, plan.Updated, info.Mode().Perm()); err != nil {
		fmt.Fprintf(stderr, "statusgen verifyflip: could-not-check — write %s: %v\n", plan.Readme, err)
		return verifyflipExitCouldNotCheck
	}
	fmt.Fprintln(stdout, "flipped: implemented → verified")
	return verifyflipExitOK
}

// planVerifyFlip runs every check and returns what would be written. A
// *flipRefusal error refuses; any other error is could-not-check.
func planVerifyFlip(root, key, wantSHA, wantRunner string) (verifyFlipPlan, error) {
	stream, num, ok := splitBriefKey(key)
	if !ok || strings.Contains(stream, ":") {
		return verifyFlipPlan{}, fmt.Errorf("--brief must be <stream>/<NN>, got %q", key)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		return verifyFlipPlan{}, fmt.Errorf("cannot read the board at %s: %w", root, err)
	}
	s, row := findBriefRow(streams, stream, num)
	if s == nil || row == nil {
		return verifyFlipPlan{}, fmt.Errorf("brief %s is not on any stream board under %s", key, root)
	}
	// The brief file: resolved exactly as `statusgen brief --check-verified`
	// (the deskevidence flip's closure check) resolves it — the stream
	// DIRECTORY named by the key, exactly one brief-<NN> file in it. Zero or
	// several files, or a key naming the stream by anything but that
	// directory, is could-not-check, never a silent pick.
	info, err := resolveBriefKey(root, key)
	if err != nil {
		return verifyFlipPlan{}, err
	}
	path := filepath.Join(root, filepath.FromSlash(info.File))
	if filepath.Clean(filepath.Dir(path)) != filepath.Clean(s.Dir) {
		return verifyFlipPlan{}, fmt.Errorf("brief %s resolves to %s, outside the board at %s", key, info.File, s.Dir)
	}
	bf, okParse, perr := parseBriefFile(path)
	if perr != nil || !okParse {
		return verifyFlipPlan{}, fmt.Errorf("cannot parse brief file %s as brief-v1: %v", path, perr)
	}

	// Frontmatter: who may flip this brief at all.
	switch bf.Gate {
	case "model":
	case "human":
		return verifyFlipPlan{}, refuseFlip("gate is human — a human-gated brief is closed by the human sign-off path, never by this verb")
	default:
		return verifyFlipPlan{}, refuseFlip("gate is %q, not model", bf.Gate)
	}
	if len(bf.Risk) == 0 {
		return verifyFlipPlan{}, refuseFlip("the brief has no risk block — no risk answer, no model flip")
	}
	if strings.TrimSpace(bf.Risk["irreversible"]) != "no" {
		return verifyFlipPlan{}, refuseFlip("risk.irreversible is %q, not no — an irreversible brief is never flipped by a model", bf.Risk["irreversible"])
	}
	for k, v := range bf.Risk {
		if strings.TrimSpace(v) != "no" {
			return verifyFlipPlan{}, refuseFlip("risk.%s is %q, not no — any risk: yes brief needs a human, not this verb", k, v)
		}
	}

	// Board row.
	if row.Status != "implemented" {
		return verifyFlipPlan{}, refuseFlip("README Status is %q, not implemented", row.Status)
	}
	if normalizeMark(strings.TrimSpace(row.Verified)) != "" {
		return verifyFlipPlan{}, refuseFlip("README Verified cell already reads %q — this verb never overwrites a stamp", row.Verified)
	}

	// Verdict, and the run that recorded it.
	stamp, run, marks, err := flipStampFromEvidence(bf.Evidence, wantSHA, wantRunner)
	if err != nil {
		return verifyFlipPlan{}, err
	}
	if err := closeVerifyHeldRefusal(key, row.Status, bf.Evidence); err != nil {
		return verifyFlipPlan{}, &flipRefusal{err.Error()}
	}
	if err := closeVerifyFailRefusal(key, row.Status, bf.Evidence); err != nil {
		return verifyFlipPlan{}, &flipRefusal{err.Error()}
	}

	// Witness: every Verify row carries a passing execution witness INSIDE the
	// PASS run — the same lines the stamp comes from. A row witnessed only by
	// an earlier run (at another sha, closed by another verdict) never ran at
	// the stamped sha, so it does not count.
	verify, _, err := briefSections(path)
	if err != nil {
		return verifyFlipPlan{}, fmt.Errorf("cannot read the brief file for %s: %w", key, err)
	}
	// Row class: a gate: model brief can still carry a Verify row a HUMAN
	// decides. Such a row is closed by a person, so the brief is never flipped
	// here; a class this binary does not know cannot say who decides, so it
	// refuses too.
	for _, r := range briefVerifyRows(verify) {
		switch {
		case r.Class == classGateHuman:
			return verifyFlipPlan{}, refuseFlip("gate-class: Verify row %s is classed gate:human — a human decides that row, so this verb never flips the brief", r.ID)
		case !knownRowClasses[r.Class]:
			return verifyFlipPlan{}, refuseFlip("gate-class: Verify row %s has unknown class %q — the verb cannot tell who decides it", r.ID, r.Class)
		}
	}
	findings, refusal := closureWitnesses(verify, run)
	if refusal != "" {
		return verifyFlipPlan{}, refuseFlip("%s", refusal)
	}
	if len(findings) == 0 {
		return verifyFlipPlan{}, refuseFlip("the brief has no Verify rows — a PASS with nothing to attest is not a flip signal")
	}
	if checkExitCode(findings) != verifyrunExitPass {
		var bad []string
		for _, f := range findings {
			if f.State != statePass {
				bad = append(bad, "row "+f.ID+": "+f.State)
			}
		}
		return verifyFlipPlan{}, refuseFlip("Verify row(s) lack a passing execution witness (%s)", strings.Join(bad, "; "))
	}

	// Provenance: the text says who ran it; git says who wrote it. Every line
	// the stamp is built from must be owned by the bound verifier or a roster
	// human.
	if err := flipProvenance(path, bf.Evidence, marks); err != nil {
		return verifyFlipPlan{}, err
	}

	readme := filepath.Join(s.Dir, "README.md")
	if fi, lerr := os.Lstat(readme); lerr != nil {
		return verifyFlipPlan{}, lerr
	} else if !fi.Mode().IsRegular() {
		return verifyFlipPlan{}, refuseFlip("%s is not a regular file — the flip never writes through a link", readme)
	}
	raw, err := os.ReadFile(readme)
	if err != nil {
		return verifyFlipPlan{}, err
	}
	updated, err := flipRewriteFn(string(raw), num, stamp)
	if err == nil {
		err = flipRowDiffCheck(string(raw), updated, num)
	}
	if err != nil {
		return verifyFlipPlan{}, fmt.Errorf("%s: %w", readme, err)
	}
	return verifyFlipPlan{Readme: readme, Stamp: stamp, Updated: []byte(updated)}, nil
}

// flipVerdictTok is one verdict token found in Evidence.
type flipVerdictTok struct {
	line   int
	word   string
	strict bool // the token opens a strict bold **VERIFY: PASS…** marker
	struck bool // inside a ~~struck~~ span: never a verdict, still the end of a run
}

// flipVerdictTokens returns every verdict token in document order, after the
// hygiene lastVerifyVerdict applies (fenced code, blockquotes, struck spans)
// plus inline code spans — a verdict in any of them is a quotation. A token in
// a struck span is returned with struck set: a retracted verdict is never the
// latest verdict, but it still closes the run recorded above it.
func flipVerdictTokens(lines []string) []flipVerdictTok {
	var toks []flipVerdictTok
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || strings.HasPrefix(trimmed, ">") {
			continue
		}
		for _, span := range strikethroughRe.FindAllString(line, -1) {
			for _, m := range flipVerdictTokRe.FindAllStringSubmatch(inlineCodeRe.ReplaceAllString(span, ""), -1) {
				toks = append(toks, flipVerdictTok{line: i, word: m[1], struck: true})
			}
		}
		line = strikethroughRe.ReplaceAllString(line, "")
		line = inlineCodeRe.ReplaceAllString(line, "")
		strictAt := map[int]bool{}
		for _, m := range verifyVerdictBoldRe.FindAllStringSubmatchIndex(line, -1) {
			if line[m[2]:m[3]] == "PASS" {
				strictAt[m[0]+2] = true // the token starts after the opening **
			}
		}
		for _, m := range flipVerdictTokRe.FindAllStringSubmatchIndex(line, -1) {
			toks = append(toks, flipVerdictTok{line: i, word: line[m[2]:m[3]], strict: strictAt[m[0]]})
		}
	}
	return toks
}

// flipStampFromEvidence derives the Verified stamp from the latest strict PASS
// and checks the recorded run against wantSHA / wantRunner. run is that PASS's
// own lines (comments stripped): the witness check reads them and nothing else.
// marks are the line indices, in the comment-stripped Evidence, of every
// Date/Runner row the stamp is built from and of the PASS marker itself: the
// provenance check blames exactly those lines.
func flipStampFromEvidence(evidence, wantSHA, wantRunner string) (stamp, run string, marks []int, err error) {
	stripped, unterminated := stripRowComments(evidence)
	if unterminated >= 0 {
		return "", "", nil, refuseFlip("Evidence carries an unterminated <!-- — the record cannot be read as written")
	}
	lines := strings.Split(stripped, "\n")
	toks := flipVerdictTokens(lines)
	li := -1
	for i, t := range toks {
		if !t.struck {
			li = i
		}
	}
	if li < 0 {
		return "", "", nil, refuseFlip("verdict mismatch: Evidence records no VERIFY verdict")
	}
	last := toks[li]
	if last.word != "PASS" {
		return "", "", nil, refuseFlip("verdict mismatch: the latest recorded verdict is VERIFY: %s, not PASS", last.word)
	}
	if !last.strict {
		return "", "", nil, refuseFlip("non-strict PASS: the latest verdict is not the strict **VERIFY: PASS** marker — record the canonical bold marker; the gate is not loosened")
	}
	start := 0
	for i := li - 1; i >= 0; i-- {
		if toks[i].line < last.line {
			start = toks[i].line + 1
			break
		}
	}
	// Verdict-first layouts put a run's rows BELOW its verdict; there the
	// rows above this PASS belong to the run before it. Rows after the latest
	// PASS mean the convention does not hold here: refuse, never guess.
	if after, _ := flipDateRunnerRows(lines[last.line+1:]); len(after) > 0 {
		return "", "", nil, refuseFlip("runner mismatch: %d Date/Runner row(s) follow the latest PASS — record each run's rows after the previous verdict and before its own", len(after))
	}
	rows, at := flipDateRunnerRows(lines[start:last.line])
	if len(rows) == 0 {
		return "", "", nil, refuseFlip("runner mismatch: no Date/Runner rows are recorded for the latest PASS")
	}
	date, runner := rows[0][0], rows[0][1]
	if !flipDateRe.MatchString(date) {
		return "", "", nil, refuseFlip("bad date: the latest PASS is dated %q, not YYYY-MM-DD", date)
	}
	if _, perr := time.Parse("2006-01-02", date); perr != nil {
		return "", "", nil, refuseFlip("bad date: the latest PASS is dated %q, not a calendar date", date)
	}
	if today := flipToday(); date > today {
		return "", "", nil, refuseFlip("bad date: the latest PASS is dated %s, after today (%s UTC)", date, today)
	}
	m0 := flipRunnerRe.FindStringSubmatch(runner)
	for _, r := range rows {
		if r[0] == "" || normalizeMark(r[0]) == "" || r[1] == "" {
			return "", "", nil, refuseFlip("runner mismatch: a row of the latest PASS has no Date or Runner")
		}
		m := flipRunnerRe.FindStringSubmatch(r[1])
		if m == nil {
			return "", "", nil, refuseFlip("sha mismatch: runner %q does not read <login> @ <sha>", r[1])
		}
		if r[0] != date {
			return "", "", nil, refuseFlip("date mismatch: rows of the latest PASS carry %q and %q", date, r[0])
		}
		if m[1] != m0[1] {
			return "", "", nil, refuseFlip("runner mismatch: rows of the latest PASS name %q and %q", m0[1], m[1])
		}
		if !strings.EqualFold(m[2], m0[2]) {
			return "", "", nil, refuseFlip("sha mismatch: rows of the latest PASS name %s and %s", m0[2], m[2])
		}
		if r[1] != runner {
			return "", "", nil, refuseFlip("runner mismatch: rows of the latest PASS read %q and %q", runner, r[1])
		}
	}
	rec, want := strings.ToLower(m0[2]), strings.ToLower(wantSHA)
	if !strings.HasPrefix(rec, want) && !strings.HasPrefix(want, rec) {
		return "", "", nil, refuseFlip("sha mismatch: the latest PASS ran at %s, expected %s", m0[2], wantSHA)
	}
	if m0[1] != wantRunner {
		return "", "", nil, refuseFlip("runner mismatch: the latest PASS was run by %q, expected %q", m0[1], wantRunner)
	}
	stamp, err = flipStamp(date, m0)
	for _, i := range at {
		marks = append(marks, start+i)
	}
	marks = append(marks, last.line)
	return stamp, strings.Join(lines[start:last.line], "\n"), marks, err
}

// flipQualRe matches one trailing parenthetical qualifier of a Runner cell.
var flipQualRe = regexp.MustCompile(`^\s*\(([^()]*)\)`)

// flipStamp renders the Verified stamp from the recorded runner. A qualifier
// naming a `human:` principal (the Evidence row's `on-behalf-of human:<name>`
// annotation) stays in the Evidence and is left out of the stamp: the sole
// writer of a `human:` token in a Verified cell is verify-gate-close.yml, and
// the human-stamp lint rejects one gained on a branch. Every other qualifier is
// kept verbatim; a stamp that would still carry `human:` is refused.
func flipStamp(date string, m []string) (string, error) {
	tail, kept := m[3], ""
	for tail != "" {
		q := flipQualRe.FindStringSubmatch(tail)
		if q == nil {
			kept += tail // not a clean qualifier list: kept whole, judged below
			break
		}
		if !strings.Contains(strings.ToLower(q[1]), "human:") {
			kept += " (" + q[1] + ")"
		}
		tail = tail[len(q[0]):]
	}
	stamp := strings.TrimSpace(date + " " + m[1] + " @ " + m[2] + kept)
	if strings.Contains(strings.ToLower(stamp), "human:") {
		return "", refuseFlip("human-stamp: the derived stamp %q carries a `human:` token — the sole human: writer is verify-gate-close.yml", stamp)
	}
	return stamp, nil
}

// flipDateRunnerRows returns (date, runner) for every data row of every table
// in lines whose header names Date and Runner, and the index of each in lines. Like evidenceVerifierInfo it
// reads the RIGHTMOST two cells, which survives an unescaped | in a command.
func flipDateRunnerRows(lines []string) (rows [][2]string, at []int) {
	inTable := false
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "|") {
			inTable = false
			continue
		}
		if separatorRowRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		if i+1 < len(lines) && separatorRowRe.MatchString(strings.Trim(strings.TrimSpace(lines[i+1]), "|")) {
			hasDate, hasRunner := false, false
			for _, c := range splitRow(line) {
				switch strings.ToLower(strings.TrimSpace(c)) {
				case "date":
					hasDate = true
				case "runner":
					hasRunner = true
				}
			}
			inTable = hasDate && hasRunner
			i++
			continue
		}
		if !inTable {
			continue
		}
		cells := splitRow(line)
		if len(cells) < 2 {
			continue
		}
		rows = append(rows, [2]string{strings.TrimSpace(cells[len(cells)-2]), strings.TrimSpace(cells[len(cells)-1])})
		at = append(at, i)
	}
	return rows, at
}

// flipRowDiffCheck is the post-condition on the README rewrite (the rewrite
// itself is transcribe-verdict's flipRowToVerified): exactly one line changed,
// it is row num, it rebuilds byte-for-byte from its cells, and of its cells
// only two differ — the Status and Verified columns, resolved by name from the
// header row of the row's own table. Status must go implemented → verified and
// Verified must have been empty; the Reviewed cell (empty or not) and every
// other cell are never touched. A row whose header cannot be resolved refuses.
func flipRowDiffCheck(before, after, num string) error {
	bl, al := strings.Split(before, "\n"), strings.Split(after, "\n")
	if len(bl) != len(al) {
		return fmt.Errorf("the rewrite changed the README's line count")
	}
	changed := -1
	for i := range bl {
		if bl[i] != al[i] {
			if changed >= 0 {
				return fmt.Errorf("the rewrite changed more than one README line")
			}
			changed = i
		}
	}
	if changed < 0 {
		return fmt.Errorf("the rewrite changed nothing")
	}
	oc, nc := splitRow(bl[changed]), splitRow(al[changed])
	if "|"+strings.Join(oc, "|")+"|" != bl[changed] {
		return fmt.Errorf("row #%s cannot be rewritten byte-for-byte", num)
	}
	if len(oc) != len(nc) || len(oc) < 2 || strings.TrimSpace(oc[0]) != num {
		return fmt.Errorf("the rewrite touched a line that is not row #%s", num)
	}
	si, vi, err := flipHeaderCols(bl, changed, len(oc))
	if err != nil {
		return fmt.Errorf("row #%s: %w", num, err)
	}
	diff := 0
	for j := range oc {
		if oc[j] == nc[j] {
			continue
		}
		diff++
		o, n := strings.TrimSpace(oc[j]), strings.TrimSpace(nc[j])
		switch {
		case j == si && o == "implemented" && n == "verified":
		case j == vi && normalizeMark(o) == "" && normalizeMark(n) != "":
		default:
			return fmt.Errorf("the rewrite changed cell %d of row #%s (%q → %q), which is neither Status implemented → verified nor the empty Verified cell", j, num, o, n)
		}
	}
	if diff != 2 {
		return fmt.Errorf("the rewrite changed %d cells of row #%s, want exactly Status and Verified", diff, num)
	}
	return nil
}

// flipHeaderCols walks up from data row `row` to the top of its table, and
// returns the Status and Verified column indices named by the header row (the
// line above the table's |---| separator). width is the data row's cell count;
// a header of another width, a missing separator, or a missing column errs.
func flipHeaderCols(lines []string, row, width int) (si, vi int, err error) {
	top := row
	for top > 0 && strings.HasPrefix(strings.TrimSpace(lines[top-1]), "|") {
		top--
	}
	if top+1 >= row || !separatorRowRe.MatchString(strings.Trim(strings.TrimSpace(lines[top+1]), "|")) {
		return 0, 0, fmt.Errorf("no header row with a |---| separator above it — the Status and Verified columns cannot be resolved")
	}
	hdr := splitRow(lines[top])
	if len(hdr) != width {
		return 0, 0, fmt.Errorf("the header has %d cells, the row %d — the columns cannot be resolved", len(hdr), width)
	}
	si, vi = -1, -1
	for j, c := range hdr {
		switch strings.ToLower(strings.TrimSpace(c)) {
		case "status":
			si = j
		case "verified":
			vi = j
		}
	}
	if si < 0 || vi < 0 {
		return 0, 0, fmt.Errorf("the header names no Status or no Verified column")
	}
	return si, vi, nil
}

// flipProvenance is the who-wrote-it check on the latest PASS. The Runner cell
// is text, and whoever can edit the brief can type the verifier's login into
// it; who committed a line is git metadata, not text. So the PASS marker and
// every Date/Runner row the stamp is built from (marks: line indices in the
// comment-stripped Evidence) are blamed, and each author is judged by the
// Evidence-actor policy (evidenceactor.go): the bound verifier or a roster
// human passes, anything else — another App, an unknown address, a line not
// yet committed — refuses. This is stricter than the Evidence-actor lint,
// which asks only whether ANY Evidence line is the verifier's: a verifier's
// older FAIL above an appended PASS satisfies that lint, never this check.
//
// Could-not-check (never a pass): no roster or no bound verifier, a shallow
// clone (blame cannot reach the commits behind the lines), an Evidence section
// that cannot be mapped to file lines, or blame failing.
func flipProvenance(path, evidence string, marks []int) error {
	p := evidenceActorPolicyFromRoster()
	if p.Unavailable != "" {
		return fmt.Errorf("provenance: %s", p.Unavailable)
	}
	if len(marks) == 0 {
		return fmt.Errorf("provenance: no PASS lines to blame")
	}
	dir, base := filepath.Dir(path), filepath.Base(path)
	if isShallowRepository(dir) {
		return fmt.Errorf("provenance: %s is in a shallow clone — blame cannot reach the commits behind the PASS", base)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	ev := strings.Split(evidence, "\n")
	start, _, ok := evidenceLineRange(content)
	if !ok || start-1+len(ev) > len(lines) || strings.Join(lines[start-1:start-1+len(ev)], "\n") != evidence {
		return fmt.Errorf("provenance: the Evidence section of %s cannot be mapped to file lines", base)
	}
	spans, ok := flipStrippedSpans(evidence)
	if !ok {
		return fmt.Errorf("provenance: the comment-stripped Evidence of %s cannot be mapped to file lines", base)
	}
	args := []string{"-C", dir, "blame", "--line-porcelain"}
	for _, m := range marks {
		if m < 0 || m >= len(spans) {
			return fmt.Errorf("provenance: PASS line %d is outside the Evidence of %s", m, base)
		}
		args = append(args, "-L", fmt.Sprintf("%d,%d", start+spans[m][0], start+spans[m][1]))
	}
	out, err := exec.Command("git", append(args, "--", base)...).Output()
	if err != nil {
		return fmt.Errorf("provenance: git blame %s: %v", base, err)
	}
	authors, _ := blamePorcelainAuthors(string(out))
	if len(authors) == 0 {
		return fmt.Errorf("provenance: blame of %s named no author for the PASS lines", base)
	}
	for _, a := range authors {
		if v, why := p.classify(a.Name, a.Email); v != actorVerifier && v != actorHuman {
			return refuseFlip("provenance: a line of the latest PASS (the marker or a Date/Runner row) is not owned by the bound verifier or a roster human — %s", why)
		}
	}
	return nil
}

// flipStrippedSpans maps each line of stripRowComments(evidence) to the raw
// evidence lines it came from, as an inclusive 0-based [first, last] range: a
// comment that spans lines folds them into one stripped line, and blame must
// see every raw line behind it. ok is false when the walk does not rebuild
// stripRowComments' output exactly — then the mapping is not trusted.
func flipStrippedSpans(evidence string) (spans [][2]int, ok bool) {
	want, unterminated := stripRowComments(evidence)
	if unterminated >= 0 {
		return nil, false
	}
	var b strings.Builder
	line, first, at := 0, 0, 0
	for _, m := range append(htmlCommentClosedRe.FindAllStringIndex(evidence, -1), []int{len(evidence), len(evidence)}) {
		for _, c := range []byte(evidence[at:m[0]]) {
			b.WriteByte(c)
			if c == '\n' {
				spans = append(spans, [2]int{first, line})
				line++
				first = line
			}
		}
		line += strings.Count(evidence[m[0]:m[1]], "\n")
		at = m[1]
	}
	spans = append(spans, [2]int{first, line})
	if b.String() != want || len(spans) != strings.Count(want, "\n")+1 {
		return nil, false
	}
	return spans, true
}
