package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The container bracket (desktools-v2/71). Every test here runs on fixtures:
// a git home with a committed brief, a fake deskdispatch on PATH that logs
// each call, and the --network none interface list stubbed in. No container
// engine, network or forge is used.

const (
	fixtureRun    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	fixtureSource = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	fixtureImage  = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	fixtureOpened = "2026-10-22T00:00:00Z"
)

type containerFixture struct {
	home, brief, marker string
	check               string // the fake deskdispatch prints this file
	receipt             string // the bracket receipt the container reads
	witness             string // where the container writes its witness
	log                 string
}

// newContainerFixture builds the home, its brief (one Verify row that touches
// a marker OUTSIDE the home) and a fake deskdispatch that prints the file at
// FAKE_DD_RECEIPT, exits FAKE_DD_EXIT, and logs its arguments.
func newContainerFixture(t *testing.T) containerFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only: the fake deskdispatch is a #!/bin/sh script on PATH")
	}
	stubIfaces(t, netnoneIfaces)
	t.Setenv("GITHUB_ACTIONS", "")
	home := t.TempDir()
	outside := t.TempDir()
	fx := containerFixture{
		home:    home,
		brief:   filepath.Join(home, "docs", "brief.md"),
		marker:  filepath.Join(outside, "ran"),
		check:   filepath.Join(outside, "check.json"),
		receipt: filepath.Join(outside, "receipt.json"),
		witness: filepath.Join(outside, "witness.md"),
		log:     filepath.Join(outside, "deskdispatch.log"),
	}
	gitInit(t, home, "fixture", "fixture@example.invalid")
	if err := os.MkdirAll(filepath.Dir(fx.brief), 0o755); err != nil {
		t.Fatal(err)
	}
	tick := "`"
	body := "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | " + tick + "touch " + fx.marker + tick + " | exit 0 |\n\n## Evidence\n\n## Review\n\nnone\n"
	if err := os.WriteFile(fx.brief, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "source.txt"), []byte("attested source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, home, "add", ".")
	runGit(t, home, "commit", "-m", "fixture")
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$FAKE_DD_LOG\"\ncat \"$FAKE_DD_RECEIPT\"\nexit \"${FAKE_DD_EXIT:-0}\"\n"
	if err := os.WriteFile(filepath.Join(bin, "deskdispatch"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_DD_LOG", fx.log)
	t.Setenv("FAKE_DD_RECEIPT", fx.check)
	t.Setenv("FAKE_DD_EXIT", "0")
	return fx
}

// checkJSON is a `deskdispatch --check-verifier` receipt.
func checkJSON(version int, run, source, brief, model string) string {
	extra := ""
	if version == 2 {
		extra = `,"SourceRepo":"example-org/two","TrackingSource":"` + source + `"`
	}
	top := `"Issue":7`
	if version == 2 {
		top += `,"BriefRoot":"/tracking","SourceHome":"/home"`
	}
	return fmt.Sprintf(`{%s,"Binding":{"Version":%d,"Run":%q,"Repo":"example-org/one","Source":%q,"Brief":%q,"Model":%q,"Tier":"strong"%s}}`, top, version, run, source, brief, model, extra)
}

var validCheck = checkJSON(1, fixtureRun, fixtureSource, "docs/brief.md", "m")

func putFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func getFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(raw)
}

// fixtureRunner is the runner the host derives from the fixture home's git
// identity (user.name "fixture"): what --open-bracket records.
const fixtureRunner = `"runner":"human:fixture","runner_source":"git-config"`

// writeBracketReceipt writes the open step's receipt file directly, with the
// home's current tree digest and the fixture's host runner, so a container
// test starts no deskdispatch.
func (fx containerFixture) writeBracketReceipt(t *testing.T, check string) {
	t.Helper()
	fx.writeReceiptRunner(t, check, fixtureRunner)
}

// writeReceiptRunner is writeBracketReceipt with the receipt's runner fields
// given as a JSON fragment ("" for none). A fragment that names no
// runner_tree gets the home's host tree token, as the open step records it.
func (fx containerFixture) writeReceiptRunner(t *testing.T, check, runner string) {
	t.Helper()
	tree, err := homeTreeDigest(fx.home)
	if err != nil {
		t.Fatal(err)
	}
	if runner != "" && !strings.Contains(runner, `"runner_tree"`) {
		runner += `,"runner_tree":"` + treeSHA(fx.home) + `"`
	}
	if runner != "" {
		runner = "," + runner
	}
	putFile(t, fx.receipt, `{"check":`+check+`,"tree":"`+tree+`","opened":"`+fixtureOpened+`"`+runner+"}\n")
}

func (fx containerFixture) containerArgs(extra ...string) []string {
	return append([]string{"--sandbox=" + sandboxContainerNetns, "--root", fx.home, "--brief", fx.brief}, extra...)
}

func (fx containerFixture) runArgs() []string {
	return fx.containerArgs("--admission-receipt", fx.receipt, "--container-image-digest", fixtureImage, "--witness", fx.witness)
}

// assertNothingRan: no row executed, the brief is unchanged and no witness
// file was written.
func (fx containerFixture) assertNothingRan(t *testing.T, before string) {
	t.Helper()
	if _, err := os.Stat(fx.marker); err == nil {
		t.Fatal("a Verify row executed although the run was refused")
	}
	if got := getFile(t, fx.brief); got != before {
		t.Fatalf("a refused run changed the brief:\n%s", got)
	}
	if _, err := os.Stat(fx.witness); err == nil {
		t.Fatal("a refused run wrote a witness file")
	}
}

// regression: #4071
// TestContainerNetnsRequiresAdmissionReceipt: container-netns without a host
// receipt is refused before any row runs, with DESK_LOOP unset and no
// attestation file in the home (the case the conditional admission let
// through unattested), and equally with DESK_LOOP naming a verifier. The
// control proves the row does run once a receipt is given.
func TestContainerNetnsRequiresAdmissionReceipt(t *testing.T) {
	fx := newContainerFixture(t)
	for _, loop := range []string{"", "verify-desk"} {
		t.Setenv("DESK_LOOP", loop)
		before := getFile(t, fx.brief)
		res, stderr := captureVerifyrun(t, fx.containerArgs("--container-image-digest", fixtureImage, "--witness", fx.witness))
		if res.code != verifyrunExitUsageError || !strings.Contains(stderr, "without --admission-receipt") {
			t.Fatalf("DESK_LOOP=%q: exit %d stderr %q, want the missing-receipt usage refusal", loop, res.code, stderr)
		}
		fx.assertNothingRan(t, before)
	}
	if calls := getFile(t, fx.log); calls != "" {
		t.Fatalf("a container run started deskdispatch: %q", calls)
	}

	// Control: the same run with a receipt executes the row.
	t.Setenv("DESK_LOOP", "")
	fx.writeBracketReceipt(t, validCheck)
	res, stderr := captureVerifyrun(t, fx.runArgs())
	if res.code != verifyrunExitPass {
		t.Fatalf("control: exit %d stderr %q", res.code, stderr)
	}
	if _, err := os.Stat(fx.marker); err != nil {
		t.Fatal("control: the row did not run with a receipt")
	}
}

// regression: #4071
// TestContainerNetnsReceipt: a valid version-1 receipt yields exactly the
// attestation line the in-process path renders, plus the image and
// Tree-Digest lines; a version-2 receipt, a brief mismatch, an incomplete
// receipt, a receipt inside the home, a missing or malformed image digest,
// and a mounted tree that does not digest to the receipt's tree (one file
// changed, one file added) are each refused before any row runs.
// deskdispatch is never started, even with DESK_LOOP naming a verifier.
func TestContainerNetnsReceipt(t *testing.T) {
	fx := newContainerFixture(t)
	t.Setenv("DESK_LOOP", "verify-desk")
	source := filepath.Join(fx.home, "source.txt")
	refusals := []struct {
		name, check, why string
		args             []string
		mutate           func()
		restore          func()
	}{
		{name: "version 2", check: checkJSON(2, fixtureRun, fixtureSource, "docs/brief.md", "m"), why: "version 2 (cross-repository) receipts are not admitted"},
		{name: "brief mismatch", check: checkJSON(1, fixtureRun, fixtureSource, "docs/other.md", "m"), why: "the receipt admits brief docs/other.md"},
		{name: "incomplete", check: checkJSON(1, fixtureRun, fixtureSource, "docs/brief.md", ""), why: "incomplete pre-work verifier receipt"},
		{name: "short run", check: checkJSON(1, "r", fixtureSource, "docs/brief.md", "m"), why: "incomplete receipt"},
		{name: "missing digest", check: validCheck, why: "without --container-image-digest",
			args: []string{"--admission-receipt", "RECEIPT", "--witness", "WITNESS"}},
		{name: "malformed digest", check: validCheck, why: `"sha256:XYZ"`,
			args: []string{"--admission-receipt", "RECEIPT", "--container-image-digest", "sha256:XYZ", "--witness", "WITNESS"}},
		{name: "changed file", check: validCheck, why: "not the sha256:",
			mutate:  func() { putFile(t, source, "TAMPERED source\n") },
			restore: func() { putFile(t, source, "attested source\n") }},
		{name: "added file", check: validCheck, why: "not the sha256:",
			mutate: func() { putFile(t, filepath.Join(fx.home, "planted.txt"), "planted\n") },
			restore: func() {
				if err := os.Remove(filepath.Join(fx.home, "planted.txt")); err != nil {
					t.Fatal(err)
				}
			}},
	}
	for _, c := range refusals {
		fx.writeBracketReceipt(t, c.check)
		if c.mutate != nil {
			c.mutate()
		}
		args := fx.runArgs()
		if c.args != nil {
			args = fx.containerArgs(c.args...)
			for i := range args {
				switch args[i] {
				case "RECEIPT":
					args[i] = fx.receipt
				case "WITNESS":
					args[i] = fx.witness
				}
			}
		}
		before := getFile(t, fx.brief)
		res, stderr := captureVerifyrun(t, args)
		if res.code == verifyrunExitPass || !strings.Contains(stderr, c.why) {
			t.Fatalf("%s: exit %d stderr %q, want a refusal naming %q", c.name, res.code, stderr, c.why)
		}
		fx.assertNothingRan(t, before)
		if c.restore != nil {
			c.restore()
		}
	}

	// A receipt inside the home is refused: it would be an input the rows see.
	fx.writeBracketReceipt(t, validCheck)
	inside := filepath.Join(fx.home, "receipt.json")
	putFile(t, inside, getFile(t, fx.receipt))
	before := getFile(t, fx.brief)
	if res, stderr := captureVerifyrun(t, fx.containerArgs("--admission-receipt", inside, "--container-image-digest", fixtureImage, "--witness", fx.witness)); res.code == verifyrunExitPass || !strings.Contains(stderr, "inside the home") {
		t.Fatalf("a receipt inside the home: exit %d stderr %q", res.code, stderr)
	}
	fx.assertNothingRan(t, before)
	if err := os.Remove(inside); err != nil {
		t.Fatal(err)
	}

	// A row that changes the mounted home leaves no witness: the tree the
	// rows end on must be the tree they started on.
	planted := filepath.Join(fx.home, "planted-by-row")
	putFile(t, fx.brief, strings.Replace(before, "touch "+fx.marker, "touch "+planted, 1))
	fx.writeBracketReceipt(t, validCheck)
	moved, movedOut := captureVerifyrunOut(t, fx.runArgs())
	if moved.code == verifyrunExitPass || !strings.Contains(movedOut, "the rows changed the mounted home") {
		t.Fatalf("a row that changed the home: exit %d output %q", moved.code, movedOut)
	}
	if _, err := os.Stat(fx.witness); err == nil {
		t.Fatal("a run whose rows changed the home wrote a witness")
	}
	if strings.Contains(movedOut, treeDigestPrefix) || strings.Contains(movedOut, attestationPrefix) {
		t.Fatalf("a run whose rows changed the home still emitted a Tree-Digest or attestation line: %q", movedOut)
	}
	if err := os.Remove(planted); err != nil {
		t.Fatal(err)
	}
	putFile(t, fx.brief, before)
	fx.writeBracketReceipt(t, validCheck)

	res, stderr := captureVerifyrun(t, fx.runArgs())
	if res.code != verifyrunExitPass {
		t.Fatalf("valid receipt: exit %d stderr %q", res.code, stderr)
	}
	want, _, _, err := verifierAdmissionReceipt([]byte(validCheck), fx.brief)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := homeTreeDigest(fx.home)
	if err != nil {
		t.Fatal(err)
	}
	got := getFile(t, fx.witness)
	tail := strings.Join([]string{" |", treeDigestPrefix + tree, containerImagePrefix + fixtureImage, want, ""}, "\n")
	if !strings.HasSuffix(got, tail) {
		t.Fatalf("the witness does not end with the table, then the Tree-Digest line, the image line and the in-process attestation line %q:\n%s", want, got)
	}
	if !strings.Contains(got, "sandbox="+sandboxContainerNetns) {
		t.Fatalf("the witness row does not record the container sandbox:\n%s", got)
	}
	if getFile(t, fx.brief) != before {
		t.Fatal("a container run wrote into the brief inside the home")
	}
	if calls := getFile(t, fx.log); calls != "" {
		t.Fatalf("a container run started deskdispatch: %q", calls)
	}
}

// regression: #4071
// TestCloseBracket: the open step writes a receipt only on an admitted home;
// the close step appends one Admission-Bracket line only when the host check
// re-admits the run the witness names and the home's recomputed tree digest
// equals both the receipt's tree and the witness's Tree-Digest. A witness
// Tree-Digest that differs, a home that moved between open and close, a
// changed source, a different run, a refused host check and an already
// bracketed witness each append nothing and exit non-zero.
func TestCloseBracket(t *testing.T) {
	fx := newContainerFixture(t)
	t.Setenv("DESK_LOOP", "")
	// The clock is the test's: the open runs at clock, the close a minute
	// later, so opened= strictly precedes closed= however fast the run is.
	clock := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	savedNow := nowFunc
	nowFunc = func() time.Time { return clock }
	t.Cleanup(func() { nowFunc = savedNow })
	openArgs := []string{"--open-bracket", "--root", fx.home, "--brief", fx.brief, "--out", fx.receipt}

	// A refused open check writes no receipt.
	putFile(t, fx.check, validCheck)
	t.Setenv("FAKE_DD_EXIT", "1")
	if res, stderr := captureVerifyrun(t, openArgs); res.code == verifyrunExitPass || !strings.Contains(stderr, "host admission check failed") {
		t.Fatalf("refused open check: exit %d stderr %q", res.code, stderr)
	}
	if _, err := os.Stat(fx.receipt); err == nil {
		t.Fatal("a refused open check wrote a receipt")
	}
	t.Setenv("FAKE_DD_EXIT", "0")
	if res, stderr := captureVerifyrun(t, openArgs); res.code != verifyrunExitPass {
		t.Fatalf("open: exit %d stderr %q", res.code, stderr)
	}
	clock = clock.Add(time.Minute)
	if res, stderr := captureVerifyrun(t, fx.runArgs()); res.code != verifyrunExitPass {
		t.Fatalf("container run: exit %d stderr %q", res.code, stderr)
	}
	witness := getFile(t, fx.witness)
	tree := regexp.MustCompile(`(?m)^Tree-Digest: (\S+)$`).FindStringSubmatch(witness)
	if tree == nil {
		t.Fatalf("no Tree-Digest line in the witness:\n%s", witness)
	}
	closeArgs := []string{"--close-bracket", "--root", fx.home, "--brief", fx.brief, "--admission-receipt", fx.receipt, "--witness", fx.witness}
	source := filepath.Join(fx.home, "source.txt")
	otherTree := "sha256:" + strings.Repeat("e", 64)
	attLine := regexp.MustCompile(`(?m)^Verification-Attestation: .*$`).FindString(witness)
	parts := strings.SplitN(witness, "\n", 3)
	head, rest := parts[0]+"\n"+parts[1]+"\n", parts[2]
	rows, tail, _ := strings.Cut(rest, treeDigestPrefix)
	tail = treeDigestPrefix + tail

	refusals := []struct {
		name, check, exit, why string
		witness                string
		tamper                 bool
	}{
		{"witness from another tree", validCheck, "0", "not the witness's Tree-Digest", strings.Replace(witness, tree[1], otherTree, 1), false},
		{"home moved between open and close", validCheck, "0", "recorded at the open step", witness, true},
		{"changed source", checkJSON(1, fixtureRun, strings.Repeat("c", 40), "docs/brief.md", "m"), "0", "does not re-admit", witness, false},
		{"different run", checkJSON(1, strings.Repeat("d", 64), fixtureSource, "docs/brief.md", "m"), "0", "does not re-admit", witness, false},
		{"refused host check", validCheck, "1", "host admission check failed", witness, false},
		{"already bracketed", validCheck, "0", "already carries", witness + admissionBracketPrefix + "run=x\n", false},
		{"no witness header", validCheck, "0", "no witness table header lies above the append position", rest, false},
		// S11: the run and tree are read only from the span that ends at
		// the append position. Rows with no header of their own under
		// another table make that span take the other table's Tree-Digest
		// and attestation lines; a span with two Tree-Digest lines is
		// refused the same way; rows split from their header by a blank
		// line have no header of their own.
		{"another table's lines in the span", validCheck, "0", "holds 2 Tree-Digest:, 2 Container-Image: and 2 attestation lines", witness + "\n" + rest, false},
		{"two Tree-Digest lines in the span", validCheck, "0", "holds 2 Tree-Digest:, 1 Container-Image: and 1 attestation lines", head + rows + treeDigestPrefix + otherTree + "\n" + rows + tail, false},
		{"two Container-Image lines in the span", validCheck, "0", "holds 1 Tree-Digest:, 2 Container-Image: and 1 attestation lines", head + rows + containerImagePrefix + otherTree + "\n" + rows + tail, false},
		{"two attestation lines in the span", validCheck, "0", "holds 1 Tree-Digest:, 1 Container-Image: and 2 attestation lines", head + rows + attLine + "\n" + rows + tail, false},
		{"rows under another table's header", validCheck, "0", "carry no witness table header of their own", head + rows + "\n" + rows + tail, false},
	}
	for _, c := range refusals {
		putFile(t, fx.check, c.check)
		putFile(t, fx.witness, c.witness)
		t.Setenv("FAKE_DD_EXIT", c.exit)
		if c.tamper {
			putFile(t, source, "TAMPERED source\n")
		}
		res, stderr := captureVerifyrun(t, closeArgs)
		if res.code == verifyrunExitPass || !strings.Contains(stderr, c.why) {
			t.Fatalf("%s: exit %d stderr %q, want a refusal naming %q", c.name, res.code, stderr, c.why)
		}
		if got := getFile(t, fx.witness); got != c.witness {
			t.Fatalf("%s: a refused close changed the witness:\n%s", c.name, got)
		}
		if c.tamper {
			putFile(t, source, "attested source\n")
		}
	}

	// The close's own time check: a receipt opened at the close's time, or
	// after it, is refused and nothing is appended, so a written bracket's
	// opened= always strictly precedes its closed=.
	putFile(t, fx.check, validCheck)
	putFile(t, fx.witness, witness)
	t.Setenv("FAKE_DD_EXIT", "0")
	receiptBody := getFile(t, fx.receipt)
	receiptOpened := regexp.MustCompile(`"opened":"([^"]+)"`).FindStringSubmatch(receiptBody)
	if receiptOpened == nil {
		t.Fatalf("fixture: no opened time in the receipt %q", receiptBody)
	}
	for name, at := range map[string]time.Time{"opened at the close": clock, "opened after the close": clock.Add(time.Hour)} {
		putFile(t, fx.receipt, strings.Replace(receiptBody, receiptOpened[0], `"opened":"`+at.Format(time.RFC3339)+`"`, 1))
		res, stderr := captureVerifyrun(t, closeArgs)
		if res.code == verifyrunExitPass || !strings.Contains(stderr, "is not after the receipt's opened time") {
			t.Fatalf("%s: exit %d stderr %q, want the close refused", name, res.code, stderr)
		}
		if got := getFile(t, fx.witness); got != witness {
			t.Fatalf("%s: a refused close changed the witness:\n%s", name, got)
		}
	}
	putFile(t, fx.receipt, receiptBody)

	putFile(t, fx.check, validCheck)
	putFile(t, fx.witness, witness)
	t.Setenv("FAKE_DD_EXIT", "0")
	preClose := digestOf(t, witness)
	if err := os.Chmod(fx.witness, 0o640); err != nil {
		t.Fatal(err)
	}
	preInfo, err := os.Stat(fx.witness)
	if err != nil {
		t.Fatal(err)
	}
	res, stderr := captureVerifyrun(t, closeArgs)
	if res.code != verifyrunExitPass {
		t.Fatalf("matching close: exit %d stderr %q", res.code, stderr)
	}
	// The close replaces the witness by temp file and rename, never by
	// rewriting it in place: the path names a new file with the old file's
	// permission bits, and no temporary file is left behind.
	postInfo, err := os.Stat(fx.witness)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(preInfo, postInfo) {
		t.Fatal("the close rewrote the witness in place; an interrupted write could truncate it")
	}
	if postInfo.Mode().Perm() != 0o640 {
		t.Fatalf("the close changed the witness's permission bits to %v, want 0640", postInfo.Mode().Perm())
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(fx.witness), ".*.tmp-*")); len(left) != 0 {
		t.Fatalf("the close left temporary files: %q", left)
	}
	got := getFile(t, fx.witness)
	above, line, _ := strings.Cut(got, admissionBracketPrefix)
	if above != witness || strings.Count(got, admissionBracketPrefix) != 1 || !strings.HasSuffix(line, "\n") || strings.Count(line, "\n") != 1 {
		t.Fatalf("the close did not append exactly one line under the unchanged witness:\n%s", got)
	}
	want := fmt.Sprintf(`^run=%s source=%s tree=%s witness=%s opened=%s closed=\S+ before=receipt after=check-verifier\n$`,
		fixtureRun, fixtureSource, regexp.QuoteMeta(tree[1]), preClose, fixtureOpenedOrAny)
	if !regexp.MustCompile(want).MatchString(line) {
		t.Fatalf("the bracket line %q does not match %s", line, want)
	}
	// --bracket-digest on the bracketed file: one line, its own bracket and
	// header line, the recorded value, verdict=match, the span's own line
	// counts, exit 0.
	const oneEach = "tree-digest:1,container-image:1,attestation:1"
	n := strings.Count(witness, "\n") + 1
	if out := verbLines(t, got, verifyrunExitPass); len(out) != 1 || out[0] != fmt.Sprintf("bracket=L%d header=L1 witness=%s recorded=%s verdict=match span-lines=%s", n, preClose, preClose, oneEach) {
		t.Fatalf("--bracket-digest of the bracketed file: %q", out)
	}

	// The digest is blind to the on-behalf-of stamp and to the landing's
	// layout, and sees every other byte of the span.
	lines := strings.Split(witness, "\n")
	row := lines[2]
	if witnessRunnerColumn(lines[0]) != 5 || len(splitRowEscaped(row)) < 6 {
		t.Fatalf("fixture row is not a witness row: %q", row)
	}
	withRow := func(r string) string {
		out := append([]string{}, lines...)
		out[2] = r
		return strings.Join(out, "\n")
	}
	stamp := func(r, ann string) string { return strings.TrimSuffix(r, " |") + " " + ann + " |" }
	blind := map[string]string{
		"stamp added":       withRow(stamp(row, "(on-behalf-of human:someone)")),
		"stamp replaced":    withRow(stamp(row, "(on-behalf-of human:other)")),
		"landed in prose":   "## Evidence\n\nVerifier prose above the block.\n\n" + strings.ReplaceAll(withRow(stamp(row, "(on-behalf-of human:someone)")), "\n", "  \n") + admissionBracketPrefix + line,
		"crlf line endings": strings.ReplaceAll(witness, "\n", "\r\n"),
	}
	if blind["stamp added"] == witness {
		t.Fatal("the fixture stamp did not change the bytes; the normalisation is untested")
	}
	for name, text := range blind {
		if d := digestOf(t, text); d != preClose {
			t.Fatalf("%s: digest %s, want the bracket's %s", name, d, preClose)
		}
	}
	// The stamp lands before a trailing source tag, verifyrun's order: the
	// digest of the tagged row is unchanged by it, with no double space left.
	tagged := withRow(stamp(row, "(git-config)"))
	taggedStamped := withRow(stamp(stamp(row, "(on-behalf-of human:someone)"), "(git-config)"))
	if !strings.Contains(taggedStamped, "(on-behalf-of human:someone) (git-config) |") {
		t.Fatalf("fixture: the stamp does not sit before the source tag: %q", taggedStamped)
	}
	if digestOf(t, tagged) != digestOf(t, taggedStamped) {
		t.Fatal("a stamp before a trailing source tag changed the digest")
	}
	// The double space a stamp can leave before the tag is collapsed.
	if digestOf(t, tagged) != digestOf(t, withRow(strings.TrimSuffix(row, " |")+"  (on-behalf-of human:someone)(git-config) |")) {
		t.Fatal("whitespace left around a removed stamp changed the digest")
	}
	if digestOf(t, tagged) == preClose {
		t.Fatal("a source tag is outside the digest; only the exact stamp may be")
	}
	// Only the exact stamp form is outside the digest.
	for _, ann := range []string{"(On-Behalf-Of human:someone)", "(on-behalf-of worker-desk)", "(on-behalf-of human: someone)", "(note: on-behalf-of human:someone)", "(on-behalf-of human:)"} {
		if digestOf(t, withRow(stamp(row, ann))) == preClose {
			t.Fatalf("%s is not the exact stamp form but was removed from the digest", ann)
		}
	}
	cells := splitRowEscaped(row)
	for name, col := range map[string]int{"Command": 1, "Result": 2, "Date": 4} {
		v := strings.TrimSpace(cells[col])
		edited := strings.Replace(row, "| "+v+" |", "| "+v+"X |", 1)
		if edited == row {
			t.Fatalf("%s: could not edit cell %q in %q", name, v, row)
		}
		if d := digestOf(t, withRow(edited)); d == preClose {
			t.Fatalf("an edited %s cell left the digest unchanged", name)
		}
	}
	if d := digestOf(t, strings.Replace(witness, tree[1], otherTree, 1)); d == preClose {
		t.Fatal("an edited Tree-Digest line left the digest unchanged")
	}
	// Two bracketed witness tables in one file: each bracket covers only the
	// nearest header above it, and an edit in the first table moves only the
	// first bracket's digest.
	two := "## Evidence\n\n" + got + "\nprose between\n\n" + got
	h2 := 3 + strings.Count(got, "\n") + 3
	out := verbLines(t, two, verifyrunExitPass)
	if len(out) != 2 || !strings.HasPrefix(out[0], fmt.Sprintf("bracket=L%d header=L3 ", n+2)) || !strings.HasPrefix(out[1], fmt.Sprintf("bracket=L%d header=L%d ", h2+n-1, h2)) {
		t.Fatalf("two bracketed tables: %q", out)
	}
	cells0 := splitRowEscaped(row)
	res0 := strings.TrimSpace(cells0[2])
	editedTwo := strings.Replace(two, "| "+res0+" |", "| "+res0+"X |", 1)
	out = verbLines(t, editedTwo, verifyrunExitFail)
	if len(out) != 2 || !strings.Contains(out[0], " verdict=mismatch ") || !strings.Contains(out[1], " verdict=match ") {
		t.Fatalf("an edit in the first table must move only the first bracket: %q", out)
	}
	// S11: a bracket is bound to its own span. A tree= that is not its own
	// span's Tree-Digest, here the other table's, is a mismatch even though the
	// witness digest matches, and so is a span holding a second Tree-Digest,
	// Container-Image or attestation line under a recomputed witness=; the
	// verdict line prints the span's counts.
	otherTD := "tree=" + otherTree
	swapped := "## Evidence\n\n" + got + "\nprose between\n\n" + strings.Replace(got, "tree="+tree[1], otherTD, 1)
	if res, out := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", tempWitness(t, swapped)}); res.code != verifyrunExitFail ||
		!strings.Contains(out, "witness="+preClose+" recorded="+preClose+" verdict=mismatch") || !strings.Contains(out, "is not its own span's Tree-Digest") {
		t.Fatalf("a bracket whose tree= is not its own span's Tree-Digest: exit %d output %q", res.code, out)
	}
	for _, dup := range []struct{ name, before, extra, counts string }{
		{"Tree-Digest", treeDigestPrefix, treeDigestPrefix + otherTree, "tree-digest:2,container-image:1,attestation:1"},
		{"Container-Image", containerImagePrefix, containerImagePrefix + otherTree, "tree-digest:1,container-image:2,attestation:1"},
		{"attestation", attestationPrefix, attLine, "tree-digest:1,container-image:1,attestation:2"},
	} {
		extra := strings.Replace(witness, dup.before, dup.extra+"\n"+dup.before, 1)
		forged := extra + admissionBracketPrefix + strings.Replace(line, preClose, digestOf(t, extra), 1)
		if res, out := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", tempWitness(t, forged)}); res.code != verifyrunExitFail ||
			!strings.Contains(out, " verdict=mismatch span-lines="+dup.counts) || !strings.Contains(out, "not one of each") {
			t.Fatalf("a span with two %s lines: exit %d output %q", dup.name, res.code, out)
		}
	}
	// The span's Tree-Digest and Container-Image values must be sha256
	// digests, even under a recomputed witness= and a matching tree=.
	imageLine := regexp.MustCompile(`(?m)^Container-Image: (\S+)$`).FindStringSubmatch(witness)
	if imageLine == nil {
		t.Fatalf("fixture: no Container-Image line in the witness:\n%s", witness)
	}
	for _, bad := range []struct{ name, from, to string }{
		{"Container-Image", containerImagePrefix + imageLine[1], containerImagePrefix + "example/image:latest"},
		{"Tree-Digest", tree[1], "sha256:short"},
	} {
		badWitness := strings.Replace(witness, bad.from, bad.to, 1)
		badLine := strings.Replace(line, preClose, digestOf(t, badWitness), 1)
		if bad.name == "Tree-Digest" {
			badLine = strings.Replace(badLine, "tree="+tree[1], "tree="+bad.to, 1)
		}
		res, out := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", tempWitness(t, badWitness+admissionBracketPrefix+badLine)})
		if res.code != verifyrunExitFail || !strings.Contains(out, " verdict=mismatch ") || !strings.Contains(out, "is not a sha256:<64 hex> digest") {
			t.Fatalf("a span whose %s is not a digest: exit %d output %q", bad.name, res.code, out)
		}
	}
	// A sandbox=container-netns row in no bracket's span is never credited,
	// so the verb does not exit 0 while one is in the file, even when every
	// bracket matches; it names the rows' lines.
	if witnessSandboxOf(row) != sandboxContainerNetns {
		t.Fatalf("fixture row is not a container row: %q", row)
	}
	orphan := got + "\nprose between\n\n" + witness
	if res, out := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", tempWitness(t, orphan)}); res.code != verifyrunExitFail ||
		!strings.Contains(out, " verdict=match ") || !strings.Contains(out, "lie in no bracket's span (L") {
		t.Fatalf("a container row outside every span: exit %d output %q", res.code, out)
	}
	// The bracket line's own fields sit outside the span's digest, so the
	// verb binds them to the span instead: run= and source= must equal the
	// span's attestation, the line must be the close step's form, and opened=
	// and closed= must be UTC RFC 3339 times with opened strictly before closed. An
	// edit to any of them after landing is a mismatch.
	closedAt := regexp.MustCompile(` closed=(\S+) `).FindStringSubmatch(line)[1]
	openedAt := regexp.MustCompile(` opened=(\S+) `).FindStringSubmatch(line)[1]
	for _, edit := range []struct{ name, from, to, why string }{
		{"run", "run=" + fixtureRun, "run=" + strings.Repeat("d", 64), "are not its own span's attestation"},
		{"source", "source=" + fixtureSource, "source=" + strings.Repeat("c", 40), "are not its own span's attestation"},
		{"opened after closed", "opened=" + openedAt, "opened=2999-01-01T00:00:00Z", "is not before its closed="},
		{"closed before opened", "closed=" + closedAt, "closed=2000-01-01T00:00:00Z", "is not before its closed="},
		{"opened equal to closed", "opened=" + openedAt, "opened=" + closedAt, "is not before its closed="},
		{"opened not UTC RFC 3339", "opened=" + openedAt, "opened=" + strings.TrimSuffix(openedAt, "Z") + "+00:00", "is not a UTC RFC 3339 time"},
		{"closed not a time", "closed=" + closedAt, "closed=yesterday", "is not a UTC RFC 3339 time"},
		{"fixed tail dropped", " before=receipt after=check-verifier", "", "not the form the close step writes"},
		{"extra field", " before=receipt", " note=x before=receipt", "not the form the close step writes"},
		{"trailing field", " after=check-verifier", " after=check-verifier note=x", "not the form the close step writes"},
	} {
		edited := strings.Replace(line, edit.from, edit.to, 1)
		if edited == line {
			t.Fatalf("%s: fixture edit did not apply to %q", edit.name, line)
		}
		res, out := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", tempWitness(t, witness+admissionBracketPrefix+edited)})
		if res.code != verifyrunExitFail || !strings.Contains(out, "witness="+preClose+" recorded="+preClose+" verdict=mismatch") || !strings.Contains(out, edit.why) {
			t.Fatalf("a bracket with its %s edited: exit %d output %q, want a mismatch naming %q", edit.name, res.code, out, edit.why)
		}
	}
	// S10: an escaped \| is cell content, never a delimiter, as in the
	// landing's cellSpans: a Command cell carrying one still leaves the stamp
	// in the Runner cell outside the digest.
	cmd := strings.TrimSpace(cells[1])
	escRow := strings.Replace(row, "| "+cmd+" |", "| "+cmd+` \| cat |`, 1)
	if escRow == row || len(splitRowEscaped(escRow)) != len(splitRowEscaped(row)) {
		t.Fatalf("fixture: could not put an escaped pipe in the Command cell: %q", escRow)
	}
	if digestOf(t, withRow(escRow)) != digestOf(t, withRow(stamp(escRow, "(on-behalf-of human:someone)"))) {
		t.Fatal("an escaped pipe in the Command cell moved the Runner column: the stamp changed the digest")
	}
	if digestOf(t, withRow(escRow)) == preClose {
		t.Fatal("the escaped pipe is outside the digest")
	}
	// A second bracket under the same table: both are shared, exit non-zero.
	out = verbLines(t, got+admissionBracketPrefix+line, verifyrunExitFail)
	if len(out) != 2 || !strings.Contains(out[0], " verdict=shared ") || !strings.Contains(out[1], " verdict=shared ") {
		t.Fatalf("two brackets under one table: %q", out)
	}
	// A bracket with no witness header above it.
	out = verbLines(t, admissionBracketPrefix+line+got, verifyrunExitFail)
	if len(out) != 2 || out[0] != "bracket=L1 header=none witness=none recorded="+preClose+" verdict=no-table span-lines=none" || !strings.Contains(out[1], " verdict=match ") {
		t.Fatalf("a bracket with no table above it: %q", out)
	}
	// A file with no bracket is could-not-check; another flag is a usage error.
	if res, _ := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", tempWitness(t, witness)}); res.code != verifyrunExitCouldNot {
		t.Fatalf("--bracket-digest on a file with no bracket: exit %d", res.code)
	}
	if res, stderr := captureVerifyrun(t, []string{"--bracket-digest", "--witness", tempWitness(t, got), "--root", fx.home}); res.code != verifyrunExitUsageError || !strings.Contains(stderr, "takes only --witness") {
		t.Fatalf("--bracket-digest with another flag: exit %d stderr %q", res.code, stderr)
	}
	if !strings.Contains(getFile(t, fx.log), "--check-verifier --root "+fx.home+" --brief "+fx.brief) {
		t.Fatalf("the close step did not run the host check: %q", getFile(t, fx.log))
	}
	if res, stderr := captureVerifyrun(t, closeArgs); res.code == verifyrunExitPass || !strings.Contains(stderr, "already carries") {
		t.Fatalf("a closed witness was closed a second time: exit %d stderr %q", res.code, stderr)
	}
	if again := getFile(t, fx.witness); again != got {
		t.Fatalf("a second close changed the witness:\n%s", again)
	}
}

// fixtureOpenedOrAny matches the open step's own opened time, which is the
// wall clock at the open.
const fixtureOpenedOrAny = `[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z`

// verbLines runs `verifyrun --bracket-digest` on text, requires exit code
// want, and returns its output lines.
func verbLines(t *testing.T, text string, want int) []string {
	t.Helper()
	res, out := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", tempWitness(t, text)})
	if res.code != want {
		t.Fatalf("--bracket-digest: exit %d, want %d; output %q", res.code, want, out)
	}
	return strings.Split(strings.TrimSpace(out), "\n")
}

// digestOf is the digest --bracket-digest computes for the first bracket in
// text, a placeholder bracket appended when text has none.
func digestOf(t *testing.T, text string) string {
	t.Helper()
	if !strings.Contains(text, admissionBracketPrefix) {
		text = strings.TrimRight(text, "\r\n") + "\n" + admissionBracketPrefix + "witness=none\n"
	}
	_, out := captureVerifyrunOut(t, []string{"--bracket-digest", "--witness", tempWitness(t, text)})
	m := regexp.MustCompile(` witness=(sha256:[0-9a-f]{64}) `).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("--bracket-digest printed no digest: %q", out)
	}
	return m[1]
}

func tempWitness(t *testing.T, text string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "witness.md")
	putFile(t, f, text)
	return f
}

// captureVerifyrunOut is captureVerifyrun with stdout and stderr together.
func captureVerifyrunOut(t *testing.T, args []string) (verifyrunResult, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	code := runVerifyrun(args, f, f)
	f.Sync()
	return verifyrunResult{code}, getFile(t, path)
}

// A receipt or witness path that does not exist yet is inside the home when a
// symlinked parent directory leads into it, so the outside-home refusals name
// the real cause.
func TestRelWithinSymlinkedParent(t *testing.T) {
	home, outside := t.TempDir(), t.TempDir()
	link := filepath.Join(outside, "into-home")
	if err := os.Symlink(home, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	for _, p := range []string{filepath.Join(link, "receipt.json"), filepath.Join(link, "missing", "dir", "witness.md")} {
		if _, ok := relWithin(home, p); !ok {
			t.Fatalf("%s leads into the home through a symlinked parent but reads as outside it", p)
		}
	}
	if rel, ok := relWithin(home, filepath.Join(outside, "missing", "receipt.json")); ok {
		t.Fatalf("a path outside the home reads as inside it (%s)", rel)
	}
	// A ".." after a symlink steps out of the link's target, as the kernel
	// resolves it, never out of the link's own directory.
	sub := filepath.Join(home, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sub, filepath.Join(outside, "l")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(sub, "back")); err != nil {
		t.Fatal(err)
	}
	if _, ok := relWithin(home, outside+"/l/../w"); !ok {
		t.Fatalf("<outside>/l/../w with l linking to <home>/sub lands in the home but reads as outside it")
	}
	if rel, ok := relWithin(home, sub+"/back/../w"); ok {
		t.Fatalf("<home>/sub/back/../w with back linking outside lands outside the home but reads as inside it (%s)", rel)
	}
}
