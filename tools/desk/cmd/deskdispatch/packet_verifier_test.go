package main

// packet_verifier_test.go — the verifier packet is a reading aid and cannot be a source of
// results.
//
// THE RISK. A verifier's verdict is supposed to come from running rows. A file handed to it
// before it starts is the one place a row's result could be copied from instead. So these
// tests are mostly about what the packet does NOT hold: nothing from the brief's Evidence
// section, nothing from the sections after it, no commit subject or message, no Expect cell
// in the command list, and no packet at all when the Verify table itself has a column that
// reads as observed results.
//
// THE FIXTURE. A real git repository in a temporary directory, because the provider reads
// the brief from a commit. Every place a result could leak from carries a sentinel string.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/packet"
)

const vpBriefRel = "docs/streams/example-stream/brief-07-thing.md"

// vpBriefBefore is the brief ahead of its Evidence heading: what a packet may carry.
const vpBriefBefore = "---\n" +
	"brief: example-stream/07\n" +
	"title: A thing\n" +
	"gate: model\n" +
	"gate-why: nothing here is irreversible\n" +
	"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
	"status: implemented\n" +
	"---\n\n" +
	"# Brief 07 — A thing\n\n" +
	"## Context\n\nCONTEXT-MARKER the thing is needed.\n\n" +
	"## Verify (executable — no prose-only DoD items)\n\n" +
	"| # | Class | Command | Expect |\n" +
	"|---|---|---|---|\n" +
	"| 1 | unit | `go test ./pkg/...` | EXPECT-SENTINEL-1 every package ok |\n" +
	"| 2 | grep | `grep -c \"alpha\\|beta\" pkg/thing.go` | EXPECT-SENTINEL-2 prints 2 |\n" +
	"| 3 | lint | `gofmt -l pkg` | EXPECT-SENTINEL-3 prints nothing |\n\n" +
	"| Failure mode of the work | Caught by |\n" +
	"|---|---|\n" +
	"| the thing is absent | row 2 |\n\n"

// vpBriefAfter is the Evidence section and what follows it: what a packet may never carry.
const vpBriefAfter = "## Evidence\n\n" +
	"EVIDENCE-PROSE-SENTINEL **VERIFY: PASS** — 3 of 3 rows.\n\n" +
	"| # | Command | Expected | Observed | Date / Runner |\n" +
	"|---|---|---|---|---|\n" +
	"| 1 | `go test ./pkg/...` | every package ok | exit 0 — ok example/pkg 0.42s OBSERVED-SENTINEL-1 | 2031-01-02 RUNNER-SENTINEL |\n" +
	"| 2 | `grep -c \"alpha\\|beta\" pkg/thing.go` | prints 2 | exit 0 — 2 OBSERVED-SENTINEL-2 | 2031-01-02 RUNNER-SENTINEL |\n" +
	"| 3 | `gofmt -l pkg` | prints nothing | exit 0 — (no output) OBSERVED-SENTINEL-3 | 2031-01-02 RUNNER-SENTINEL |\n\n" +
	"## Review\n\nREVIEW-SENTINEL reviewed 2031-01-03, PASS.\n\n" +
	"## Verdict: PASS HEADING-SENTINEL\n\nrecorded.\n"

var vpSentinels = []string{
	"EVIDENCE-PROSE-SENTINEL", "OBSERVED-SENTINEL", "RUNNER-SENTINEL", "REVIEW-SENTINEL", "HEADING-SENTINEL",
	"SUBJECT-SENTINEL", "BODY-SENTINEL", "VERIFY: PASS", "VERIFY: FAIL", "exit 0", "2031-01-02",
}

// vpClock gives every fixture commit its own second, so "newest first" has one answer.
var vpClock atomic.Int64

// vpGit runs git in dir with a fixed identity, a clock that advances one second a call, and
// no configuration from the machine.
func vpGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	when := fmt.Sprintf("%d +0000", 1000000000+vpClock.Add(1))
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"GIT_AUTHOR_DATE="+when, "GIT_COMMITTER_DATE="+when)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func vpWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// vpRepo builds the fixture repository and returns its root: the brief authored, the work
// delivered by a commit that names the brief without touching it, and an evidence landing
// whose subject and message state a verdict.
func vpRepo(t *testing.T, brief string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	vpGit(t, root, "init", "-q")
	vpWrite(t, root, vpBriefRel, vpBriefBefore)
	vpGit(t, root, "add", ".")
	vpGit(t, root, "commit", "-q", "-m", "docs: author example-stream/07")
	vpWrite(t, root, "pkg/thing.go", "package pkg\n\n// alpha beta\n")
	vpGit(t, root, "add", ".")
	vpGit(t, root, "commit", "-q", "-m", "feat: the thing SUBJECT-SENTINEL\n\nBODY-SENTINEL\n\nBrief: example-stream/07")
	vpWrite(t, root, vpBriefRel, brief)
	vpGit(t, root, "add", ".")
	vpGit(t, root, "commit", "-q", "-m",
		"verify(example-stream/07): evidence — VERIFY: PASS SUBJECT-SENTINEL\n\nBODY-SENTINEL exit 0\n\nBrief: example-stream/07")
	return root
}

func vpInput(root string) packetInput {
	return packetInput{
		o:    dispatchOpts{kit: "verifier", item: "example-stream/07", root: root, brief: filepath.Join(root, vpBriefRel), quiet: true},
		plan: dispatchPlan{claimKey: "repo--example-stream--07"},
		repo: "example/repo",
		home: root,
	}
}

// vpBuild runs the provider and the shared builder and returns the packet text.
func vpBuild(t *testing.T, in packetInput) string {
	t.Helper()
	spec, err := verifierPacket(in)
	if err != nil {
		t.Fatalf("verifierPacket: %v", err)
	}
	spec.Token = "t0ken"
	built, err := packet.Build(spec)
	if err != nil {
		t.Fatalf("packet.Build: %v", err)
	}
	return built.Text
}

// vpItem returns the body of the quoted item whose boundary line names label. The label on a
// boundary line is a value, so it is looked for the way the builder writes one.
func vpItem(t *testing.T, text, label string) string {
	t.Helper()
	i := strings.Index(text, " — "+packet.Code(label)+" — ")
	if i < 0 {
		t.Fatalf("packet has no item labelled %q:\n%s", label, text)
	}
	rest := text[i:]
	rest = rest[strings.Index(rest, "\n")+1:]
	// The closing line is matched whole and with the fixture's token: a body may hold a line
	// that only looks like one.
	end := strings.Index("\n"+rest, "\n<<<END-UNTRUSTED-CONTENT t0ken>>>\n")
	if end < 0 {
		t.Fatalf("item %q is not closed", label)
	}
	return rest[:end]
}

// TestVerifierPacketCarriesNoResultContent is the control. The fixture's Evidence section,
// the sections after it, and its commit subjects and messages all hold results; none of
// that text may reach the packet, by sentinel and line by line.
func TestVerifierPacketCarriesNoResultContent(t *testing.T) {
	root := vpRepo(t, vpBriefBefore+vpBriefAfter)
	text := vpBuild(t, vpInput(root))

	for _, s := range vpSentinels {
		if strings.Contains(text, s) {
			t.Errorf("the packet holds %q, which is result content:\n%s", s, text)
		}
	}
	// Line by line: no line of the Evidence section or of what follows it is in the packet,
	// unless the very same line also stands ahead of the Evidence heading.
	ahead := map[string]bool{}
	for _, l := range strings.Split(vpBriefBefore, "\n") {
		ahead[strings.TrimSpace(l)] = true
	}
	for _, l := range strings.Split(vpBriefAfter, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || ahead[l] {
			continue
		}
		if strings.Contains(text, l) {
			t.Errorf("the packet holds a line from the Evidence section or after it: %q", l)
		}
	}
	// The command list is the # and Command cells only: no Expect cell, though the brief
	// text above it carries the brief's own Expect column.
	cmds := vpItem(t, text, "Verify row commands (the # and Command cells only)")
	if strings.Contains(cmds, "EXPECT-SENTINEL") {
		t.Errorf("the command list holds an Expect cell:\n%s", cmds)
	}
	if !strings.Contains(text, "EXPECT-SENTINEL-2 prints 2") {
		t.Errorf("the brief text should carry the brief's own Expect column:\n%s", text)
	}
	// It says so itself, and names what it left out.
	for _, want := range []string{
		"The dispatcher ran no row, so nothing in this file is a result of this run",
		"Left out: the brief from its first Evidence heading to its end; the Expect cells from the command list; " +
			"every commit subject, message and date",
		"A row's result comes only from running the row at the verified commit; nothing in this file is evidence",
		"The dispatcher ran none of them",
		"`the Evidence section and the sections after it` (section \"Earlier Evidence: its size, never its rows\")",
		"earlier observed results are never copied into a packet",
		"2 further section(s) follow it",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the packet does not say %q:\n%s", want, text)
		}
	}
}

// TestVerifierPacketCarriesWhatAVerifierReadsFirst: the commit, the gate and risk lines, the
// brief ahead of Evidence, each row's command exactly as written, and the commits that
// changed or name the brief with the paths they changed.
func TestVerifierPacketCarriesWhatAVerifierReadsFirst(t *testing.T) {
	root := vpRepo(t, vpBriefBefore+vpBriefAfter)
	head := vpGit(t, root, "rev-parse", "HEAD")
	delivered := vpGit(t, root, "rev-parse", "HEAD~1")
	authored := vpGit(t, root, "rev-parse", "HEAD~2")
	text := vpBuild(t, vpInput(root))

	for _, want := range []string{
		"- **Head commit:** `" + head + "`",
		"- Brief: `" + vpBriefRel + "`",
		fmt.Sprintf("At most %d commits are listed, from a search of at most %d commits back, and at most %d changed paths for one commit.",
			verifierPacketCommits, verifierPacketScan, verifierPacketFiles),
		"CONTEXT-MARKER the thing is needed.",
		"| 2 | grep | `grep -c \"alpha\\|beta\" pkg/thing.go` | EXPECT-SENTINEL-2 prints 2 |",
		"The Verify section has 3 row(s) at this commit",
		"- `" + head + "`: changed the brief file and its message has a `Brief: example-stream/07` line",
		"- `" + delivered + "`: its message has a `Brief: example-stream/07` line",
		"- `" + authored + "`: changed the brief file",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the packet lacks %q:\n%s", want, text)
		}
	}
	if got, want := vpItem(t, text, "frontmatter gate and risk lines"),
		"gate: model\ngate-why: nothing here is irreversible\n"+
			"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n"; got != want {
		t.Errorf("gate and risk lines = %q, want %q", got, want)
	}
	if got, want := vpItem(t, text, "Verify row commands (the # and Command cells only)"),
		"[1] `go test ./pkg/...`\n[2] `grep -c \"alpha\\|beta\" pkg/thing.go`\n[3] `gofmt -l pkg`\n"; got != want {
		t.Errorf("command list = %q, want %q", got, want)
	}
	if got := vpItem(t, text, "paths changed by "+delivered[:12]); got != "pkg/thing.go\n" {
		t.Errorf("paths changed by the delivering commit = %q, want pkg/thing.go", got)
	}
	// The first commit has no parent to compare against: listed, never guessed.
	if !strings.Contains(text, "`paths changed by "+authored[:12]+"`") {
		t.Errorf("the parentless commit's paths should be listed as omitted:\n%s", text)
	}
}

// TestVerifierPacketDeclinesAResultLikeVerifyColumn: a Verify table that itself has a column
// reading as observed results is not quoted — the provider declines, and through the one
// call dispatch makes that is no file and no path, never a failure.
func TestVerifierPacketDeclinesAResultLikeVerifyColumn(t *testing.T) {
	brief := strings.Replace(vpBriefBefore, "| # | Class | Command | Expect |\n|---|---|---|---|\n",
		"| # | Class | Command | Observed |\n|---|---|---|---|\n", 1)
	root := vpRepo(t, brief+vpBriefAfter)
	in := vpInput(root)
	if _, err := verifierPacket(in); err == nil || !strings.Contains(err.Error(), "result-like column") {
		t.Fatalf("verifierPacket err = %v, want a result-like-column refusal", err)
	}
	in.o.promptFile = filepath.Join(t.TempDir(), "prompt.md")
	if got := buildDispatchPacket(in.o, in.plan, in.repo, in.home); got != "" {
		t.Errorf("buildDispatchPacket = %q, want no packet", got)
	}
	if _, err := os.Stat(strings.TrimSuffix(in.o.promptFile, ".md") + packetSuffix); !os.IsNotExist(err) {
		t.Errorf("a packet file was written for a brief the provider declined (stat err = %v)", err)
	}
}

func TestVerifierPacketResultColumnWords(t *testing.T) {
	cases := []struct {
		header string
		want   bool
	}{
		{"| # | Command | Expect |", false},
		{"| # | Class | Command | Expect |", false},
		{"| # | Command | Expect | Class | Shell |", false},
		{"| Failure mode of the work | Caught by |", false},
		{"| Row | The mutation that must redden it |", false},
		{"| # | Command | Expect | Observed |", true},
		{"| # | Command | Expect | **Result** |", true},
		{"| # | Command | Expected output |", true},
		{"| # | Command | Exit code |", true},
		{"| # | Command | Expect | Status |", true},
		{"| # | Command | Expect | Date / Runner |", true},
		{"| # | Command | Expect | Pass/Fail |", true},
	}
	for _, tc := range cases {
		doc := parseVerifierBrief("## Verify\n\n" + tc.header + "\n|---|---|\n| 1 | `true` | x |\n\n## Evidence\n")
		if got := doc.resultColumn() != ""; got != tc.want {
			t.Errorf("resultColumn(%q) found = %v, want %v", tc.header, got, tc.want)
		}
	}
}

// TestVerifierPacketCutsAtTheFirstEvidenceHeading: any level, no account taken of code
// fences. Cutting early costs a read; cutting late could carry results.
func TestVerifierPacketCutsAtTheFirstEvidenceHeading(t *testing.T) {
	cases := []struct {
		name, text, keep, drop string
		hasEv                  bool
	}{
		{"level two", "intro KEEP\n\n## Evidence\n\nDROP\n", "KEEP", "DROP", true},
		{"with a qualifier", "KEEP\n\n## Evidence (run 2)\n\nDROP\n", "KEEP", "DROP", true},
		{"lower case, level three", "KEEP\n\n### evidence so far\n\nDROP\n", "KEEP", "DROP", true},
		{"inside a code fence", "KEEP\n\n```\n## Evidence\nDROP\n```\n\n## Evidence\n\nDROP\n", "KEEP", "DROP", true},
		{"a title that only mentions it", "# Brief 3 — Evidence rules KEEP\n\nKEEP2\n", "KEEP2", "", false},
		{"none", "KEEP\n\n## Verify\n", "KEEP", "", false},
	}
	for _, tc := range cases {
		doc := parseVerifierBrief(tc.text)
		if doc.hasEv != tc.hasEv {
			t.Errorf("%s: hasEv = %v, want %v", tc.name, doc.hasEv, tc.hasEv)
		}
		if !strings.Contains(doc.before, tc.keep) {
			t.Errorf("%s: the carried text lost %q: %q", tc.name, tc.keep, doc.before)
		}
		if tc.drop != "" && strings.Contains(doc.before, tc.drop) {
			t.Errorf("%s: the carried text holds %q from the Evidence section: %q", tc.name, tc.drop, doc.before)
		}
	}
}

// TestVerifierPacketRowCommandsNeverGuess: a cell is copied as written, and a row the tool
// cannot split with certainty yields no list at all.
func TestVerifierPacketRowCommandsNeverGuess(t *testing.T) {
	if got, want := vpSplitRow("| 4 | `a \\| b` |  `c`  | d |"), []string{"4", "`a \\| b`", "`c`", "d"}; strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("vpSplitRow = %q, want %q", got, want)
	}
	build := func(section packet.Content) string {
		t.Helper()
		p, err := packet.Build(packet.Spec{Head: "0123abcd", Token: "t0ken", Sections: []packet.Section{
			packet.NewSection("Rows", func() (packet.Content, error) { return section, nil })}})
		if err != nil {
			t.Fatal(err)
		}
		return p.Text
	}
	// An unescaped pipe inside a cell gives the row one cell too many.
	uneven := parseVerifierBrief("## Verify\n\n| # | Command | Expect |\n|---|---|---|\n" +
		"| 1 | `true` | ok |\n| 2 | `cat f | wc -l` | UNEVEN-EXPECT |\n\n## Evidence\n")
	text := build(uneven.rowCommands())
	if strings.Contains(text, "[1]") || strings.Contains(text, "UNEVEN-EXPECT") || strings.Contains(text, "wc -l") {
		t.Errorf("a table with an uneven row must give no command list:\n%s", text)
	}
	if !strings.Contains(text, "The tool will not guess where a command ends") {
		t.Errorf("the packet should say why there is no command list:\n%s", text)
	}
	for name, brief := range map[string]string{
		"no Verify heading": "## Task\n\ndo\n\n## Evidence\n",
		"no row table":      "## Verify\n\nprose only\n\n## Evidence\n",
		// A Verify heading that stands only AFTER the Evidence heading is not read.
		"Verify after Evidence": "## Evidence\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | ok |\n",
	} {
		text := build(parseVerifierBrief(brief).rowCommands())
		if strings.Contains(text, "[1]") || !strings.Contains(text, "Read the brief.") {
			t.Errorf("%s: want no command list and a pointer to the brief:\n%s", name, text)
		}
	}
}

// TestVerifierPacketNeverFailsOrDirtiesTheHome: every way the provider can decline ends in
// "no packet", and a packet that IS written lands outside the verifier home, which stays
// clean — an additional file there would refuse the pre-work check.
func TestVerifierPacketNeverFailsOrDirtiesTheHome(t *testing.T) {
	root := vpRepo(t, vpBriefBefore+vpBriefAfter)

	inside := vpInput(root)
	inside.o.promptFile = filepath.Join(root, "prompt.md")
	if _, err := verifierPacket(inside); err == nil || !strings.Contains(err.Error(), "inside the verifier home") {
		t.Errorf("a prompt file in the home: err = %v, want a refusal", err)
	}
	if got := buildDispatchPacket(inside.o, inside.plan, inside.repo, inside.home); got != "" {
		t.Errorf("a prompt file in the home: buildDispatchPacket = %q, want no packet", got)
	}

	outside := vpInput(root)
	outside.o.brief = filepath.Join(t.TempDir(), "brief-01-elsewhere.md")
	if _, err := verifierPacket(outside); err == nil || !strings.Contains(err.Error(), "not a file inside the dispatched repository") {
		t.Errorf("a brief outside --root: err = %v, want a refusal", err)
	}

	notGit := vpInput(root)
	notGit.home = t.TempDir()
	notGit.o.promptFile = filepath.Join(t.TempDir(), "prompt.md")
	if got := buildDispatchPacket(notGit.o, notGit.plan, notGit.repo, notGit.home); got != "" {
		t.Errorf("a home that is not a repository: buildDispatchPacket = %q, want no packet", got)
	}

	ok := vpInput(root)
	ok.o.promptFile = filepath.Join(t.TempDir(), "prompt.md")
	dest := buildDispatchPacket(ok.o, ok.plan, ok.repo, ok.home)
	if want := strings.TrimSuffix(ok.o.promptFile, ".md") + packetSuffix; dest != want {
		t.Fatalf("buildDispatchPacket = %q, want %q", dest, want)
	}
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(dest); err != nil || st.Mode().Perm() != 0o600 {
			t.Errorf("packet stat = %v, %v; want an owner-only file", st, err)
		}
	}
	if vpWithin(root, dest) {
		t.Errorf("the packet %s was written inside the verifier home %s", dest, root)
	}
	if body, _ := os.ReadFile(dest); !strings.HasPrefix(string(body), "# Dispatch packet — verifier — repo--example-stream--07\n") {
		t.Errorf("the written packet does not open with the verifier title:\n%.200s", body)
	}
	if dirty := vpGit(t, root, "status", "--porcelain", "--ignored"); dirty != "" {
		t.Errorf("building the packet changed the verifier home:\n%s", dirty)
	}
}

// TestVerifierPacketBoundsCommitsAndPaths: the commit list stops at its stated limit, newest
// first, and a commit that changed more paths than the limit has its list left out and named
// in the omission list — never cut to the first few.
func TestVerifierPacketBoundsCommitsAndPaths(t *testing.T) {
	root := vpRepo(t, vpBriefBefore+vpBriefAfter)
	oldest := vpGit(t, root, "rev-parse", "HEAD~2")
	for i := 0; i <= verifierPacketFiles; i++ {
		vpWrite(t, root, fmt.Sprintf("wide/file-%03d.txt", i), "x\n")
	}
	vpGit(t, root, "add", ".")
	vpGit(t, root, "commit", "-q", "-m", "feat: a wide change\n\nBrief: example-stream/07")
	wide := vpGit(t, root, "rev-parse", "HEAD")
	for i := 0; i < verifierPacketCommits; i++ {
		vpWrite(t, root, "pkg/thing.go", fmt.Sprintf("package pkg\n\n// alpha beta %d\n", i))
		vpGit(t, root, "add", ".")
		vpGit(t, root, "commit", "-q", "-m", fmt.Sprintf("fix: follow-up %d\n\nBrief: example-stream/07", i))
	}
	text := vpBuild(t, vpInput(root))
	if got := len(regexp.MustCompile("(?m)^- `[0-9a-f]{40}`: ").FindAllString(text, -1)); got != verifierPacketCommits {
		t.Errorf("the packet lists %d commits, want the limit of %d:\n%s", got, verifierPacketCommits, text)
	}
	for _, old := range []string{wide, oldest} {
		if strings.Contains(text, old) {
			t.Errorf("commit %s is older than the newest %d and must not be listed", old, verifierPacketCommits)
		}
	}

	// Now the wide commit is the newest: its path list is over the limit.
	root = vpRepo(t, vpBriefBefore+vpBriefAfter)
	for i := 0; i <= verifierPacketFiles; i++ {
		vpWrite(t, root, fmt.Sprintf("wide/file-%03d.txt", i), "x\n")
	}
	vpGit(t, root, "add", ".")
	vpGit(t, root, "commit", "-q", "-m", "feat: a wide change\n\nBrief: example-stream/07")
	wide = vpGit(t, root, "rev-parse", "HEAD")
	text = vpBuild(t, vpInput(root))
	if !strings.Contains(text, "- `"+wide+"`: its message has a `Brief: example-stream/07` line") {
		t.Fatalf("the newest commit %s is not listed:\n%s", wide, text)
	}
	if strings.Contains(text, "wide/file-") {
		t.Errorf("a path list over the %d-path limit was copied, whole or in part:\n%s", verifierPacketFiles, text)
	}
	want := fmt.Sprintf("%d paths, over the %d-path limit", verifierPacketFiles+1, verifierPacketFiles)
	if !strings.Contains(text, want) {
		t.Errorf("the omission list does not say %q:\n%s", want, text)
	}
}

// TestVerifierPacketRefusesAHomeThatMoved: the packet describes one commit. If the home's
// HEAD is no longer that commit once the sections are built, nothing is handed over.
// TestVerifierPacketSearchEndsWhereTheBriefBegan: the history is searched back to the commit
// that added the brief file and no further, and the packet says so. A commit older than the
// brief is never listed, whatever its message claims.
func TestVerifierPacketSearchEndsWhereTheBriefBegan(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	vpGit(t, root, "init", "-q")
	vpWrite(t, root, "pkg/thing.go", "package pkg\n")
	vpGit(t, root, "add", ".")
	vpGit(t, root, "commit", "-q", "-m", "feat: before the brief existed\n\nBrief: example-stream/07")
	before := vpGit(t, root, "rev-parse", "HEAD")
	vpWrite(t, root, vpBriefRel, vpBriefBefore)
	vpGit(t, root, "add", ".")
	vpGit(t, root, "commit", "-q", "-m", "docs: author the brief")
	added := vpGit(t, root, "rev-parse", "HEAD")
	vpWrite(t, root, "pkg/thing.go", "package pkg\n\n// more\n")
	vpGit(t, root, "add", ".")
	vpGit(t, root, "commit", "-q", "-m", "feat: unrelated")
	unrelated := vpGit(t, root, "rev-parse", "HEAD")

	text := vpBuild(t, vpInput(root))
	if !strings.Contains(text, "- `"+added+"`: changed the brief file") {
		t.Fatalf("the commit that added the brief is not listed:\n%s", text)
	}
	// The newest commit is the head the packet records, so look for a LISTED line.
	for _, sha := range []string{before, unrelated} {
		if strings.Contains(text, "- `"+sha+"`") {
			t.Errorf("commit %s neither changed the brief nor is inside the search, yet it is listed:\n%s", sha, text)
		}
	}
	if !strings.Contains(text, "The search goes no further back than the commit that added the brief file.") {
		t.Errorf("the packet does not say where its history search ends:\n%s", text)
	}
}

// TestVerifierPacketSearchBoundIsACountAndIsStated: the search stops after its bound of
// commits, lists nothing older, and says in the packet that it stopped — a search cut short
// never reads as "nothing older exists".
func TestVerifierPacketSearchBoundIsACountAndIsStated(t *testing.T) {
	root := vpRepo(t, vpBriefBefore+vpBriefAfter)
	head := vpGit(t, root, "rev-parse", "HEAD")
	delivered := vpGit(t, root, "rev-parse", "HEAD~1")
	authored := vpGit(t, root, "rev-parse", "HEAD~2")
	repo, _, err := vpOpenAtHead(root)
	if err != nil {
		t.Fatal(err)
	}
	section := func(scan int) string {
		c, err := verifierPacketCommitsSection(repo, head, vpBriefRel, "example-stream/07", scan)
		if err != nil {
			t.Fatal(err)
		}
		built, err := packet.Build(packet.Spec{Kit: "verifier", Item: "k", Head: head, Token: "t0ken",
			Sections: []packet.Section{packet.NewSection("Commits", func() (packet.Content, error) { return c, nil })}})
		if err != nil {
			t.Fatal(err)
		}
		return built.Text
	}

	text := section(2)
	for _, sha := range []string{head, delivered} {
		if !strings.Contains(text, "- `"+sha+"`: ") {
			t.Errorf("commit %s is inside a search of 2 and is not listed:\n%s", sha, text)
		}
	}
	if strings.Contains(text, authored) {
		t.Errorf("commit %s is past a search of 2 and is listed:\n%s", authored, text)
	}
	if !strings.Contains(text, "the history search stops after 2 commits") {
		t.Errorf("the packet does not say its search stopped at the bound:\n%s", text)
	}

	// Control: with room to spare all three are listed and no stop is reported.
	text = section(verifierPacketScan)
	if !strings.Contains(text, "- `"+authored+"`: changed the brief file") {
		t.Errorf("control: the oldest commit is not listed by an unbounded search:\n%s", text)
	}
	if strings.Contains(text, "the history search stops after") {
		t.Errorf("control: a search that reached the brief's first commit reports a stop:\n%s", text)
	}
}

// TestVerifierPacketSaysWhenTheHistoryStopsShort: in a home whose history cannot be read all
// the way back (a shallow clone), the commits read before the gap are still listed, the
// packet says the search ended there, and the commit at the gap is NOT reported as having
// changed the brief — an unreadable parent settles nothing.
func TestVerifierPacketSaysWhenTheHistoryStopsShort(t *testing.T) {
	root := vpRepo(t, vpBriefBefore+vpBriefAfter)
	head := vpGit(t, root, "rev-parse", "HEAD")
	boundary := vpGit(t, root, "rev-parse", "HEAD~1")
	clone := filepath.Join(t.TempDir(), "shallow")
	vpGit(t, root, "clone", "-q", "--depth", "2", "file://"+root, clone)
	if got := vpGit(t, clone, "rev-list", "--count", "HEAD"); got != "2" {
		t.Fatalf("control: the shallow clone holds %s commits, want 2", got)
	}

	text := vpBuild(t, vpInput(clone))
	if !strings.Contains(text, "- `"+head+"`: changed the brief file") {
		t.Errorf("the commit read before the gap is not listed:\n%s", text)
	}
	if strings.Contains(text, boundary) {
		t.Errorf("the commit at the gap (%s) is listed although its parent could not be read:\n%s", boundary, text)
	}
	if !strings.Contains(text, "the history could not be read further in this clone") {
		t.Errorf("the packet does not say its history search stopped short:\n%s", text)
	}
	if strings.Contains(text, "No commit searched changed the brief file") {
		t.Errorf("a search that stopped short reads as an empty one:\n%s", text)
	}
}

func TestVerifierPacketRefusesAHomeThatMoved(t *testing.T) {
	root := vpRepo(t, vpBriefBefore+vpBriefAfter)
	spec, err := verifierPacket(vpInput(root))
	if err != nil {
		t.Fatal(err)
	}
	vpWrite(t, root, "pkg/later.go", "package pkg\n")
	vpGit(t, root, "add", ".")
	vpGit(t, root, "commit", "-q", "-m", "later")
	if _, err := packet.Build(spec); err == nil || !strings.Contains(err.Error(), "moved from") {
		t.Fatalf("packet.Build after the home moved: err = %v, want a refusal", err)
	}
}

// TestVerifierPromptNamesThePacketOnce: with a packet the verifier's assignment carries
// exactly one `Packet:` line and the kit clause that says what the file is; with none, no
// line — the clause is then inert by its own words.
func TestVerifierPromptNamesThePacketOnce(t *testing.T) {
	if _, ok := packetProviders["verifier"]; !ok {
		t.Fatal("no packet provider is registered for the verifier kit")
	}
	o := dispatchOpts{kit: "verifier", item: "example-stream/07", root: "/example/root", brief: "/example/root/" + vpBriefRel}
	const path = "/example/cache/assay/packets/repo--example-stream--07.packet.md"
	with, err := assemblePrompt(o, dispatchPlan{repo: "example/repo", claimKey: "repo--example-stream--07", packetPath: path}, "/example/home")
	if err != nil {
		t.Fatal(err)
	}
	if got := packetLines(t, with); len(got) != 1 || got[0] != "Packet: "+path {
		t.Errorf("assignment `Packet:` lines = %q, want exactly one naming %s", got, path)
	}
	if !strings.Contains(with, "A packet is a READING AID ONLY") {
		t.Error("the verifier prompt lacks the kit's packet clause")
	}
	without, err := assemblePrompt(o, dispatchPlan{repo: "example/repo", claimKey: "repo--example-stream--07"}, "/example/home")
	if err != nil {
		t.Fatal(err)
	}
	if got := packetLines(t, without); len(got) != 0 {
		t.Errorf("no packet was written, yet the assignment carries %q", got)
	}
}
