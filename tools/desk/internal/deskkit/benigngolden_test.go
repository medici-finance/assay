package deskkit

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The VERDICT GOLDEN for the secret scan's benign-shape admissions.
//
// The registry in benignshapes.go replaced a hand-ordered chain of admission checks. A
// refactor of a trust-boundary control is only safe when it is verdict-for-verdict the
// same as what it replaced, so this test pins the scan's verdict on a large derived input
// set, recorded ONCE against the unmigrated scanner and never edited by a later commit.
//
// The test is deliberately written against API that predates the registry
// (ScanSurfaceSecrets, the *ScanFinding a refusal carries, and loadCorpus), so the same
// file can be copied onto the pre-migration tree and must pass there too: that run is
// what proves the golden is the OLD scanner's behaviour rather than the new one's.
//
// Inputs are derived, never written, and the golden holds no input bytes, only ids:
//
//   - corpus:<file>               every corpus fixture's payload, whole;
//   - corpus:<file>:<line>        every non-blank payload line of every fixture, alone;
//   - gen:<index>                 goldenRuns runs of length 30-90 from a fixed seed, over
//                                 the base64, hex and alphanumeric alphabets, alone;
//   - gen:<index>:<file>:<line>:<position>
//                                 the same run placed into a negative fixture's payload
//                                 line at four positions (start, eq, slash, digest), the
//                                 rest of the payload kept, so line and artifact context
//                                 both reach the scan.
//
// Placement rotates: run i lands in negative fixture i mod N, on payload line
// (i / N) mod L of that fixture, at all four positions. Over the runs every negative
// fixture and every payload line of it is reached, while the golden stays a reviewable
// size (a full cross product is over a million inputs).
//
// The verdict is `<rule>:<line>` from the refusal's ScanFinding, or `clean`.

const (
	goldenFile = "benign-golden.txt"
	goldenRuns = 2000
	goldenSeed = 61
	// goldenRecordEnv, set to 1, rewrites the golden from the scanner under test. Only the
	// first commit of the migration records it; every later run compares.
	goldenRecordEnv = "BENIGN_GOLDEN_RECORD"
)

// goldenAlphabets are the three alphabets the generated runs are drawn from.
var goldenAlphabets = []string{
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "0123456789+/",
	"0123456789abcdef",
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "abcdefghijklmnopqrstuvwxyz" + "0123456789",
}

// goldenPositions are the four places a generated run is put into a payload line. When
// the line already holds the anchor the run goes directly after its first occurrence;
// otherwise the anchor and the run are appended to the line.
var goldenPositions = []struct{ name, anchor string }{
	{"start", ""},
	{"eq", "="},
	{"slash", "/"},
	{"digest", "@sha256:"},
}

type goldenInput struct {
	id      string
	content string
}

// goldenRunsFrom returns n runs from a fixed seed: lengths 30 to 90, alphabet rotating.
func goldenRunsFrom(n int) []string {
	r := rand.New(rand.NewSource(goldenSeed))
	out := make([]string, n)
	for i := range out {
		alpha := goldenAlphabets[i%len(goldenAlphabets)]
		b := make([]byte, 30+r.Intn(61))
		for j := range b {
			b[j] = alpha[r.Intn(len(alpha))]
		}
		out[i] = string(b)
	}
	return out
}

// placeRun puts run into line at the named position.
func placeRun(line, run, anchor string) string {
	if anchor == "" {
		return run + line
	}
	if k := strings.Index(line, anchor); k >= 0 {
		k += len(anchor)
		return line[:k] + run + line[k:]
	}
	return line + anchor + run
}

// nonBlankLines returns the 0-based indexes of payload's non-blank lines.
func nonBlankLines(lines []string) []int {
	var idx []int
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			idx = append(idx, i)
		}
	}
	return idx
}

// goldenInputs derives every input in a stable order.
func goldenInputs(t *testing.T) []goldenInput {
	t.Helper()
	corpus := loadCorpus(t)
	sort.Slice(corpus, func(i, j int) bool { return corpus[i].file < corpus[j].file })
	var in []goldenInput
	var negs []corpusArtifact
	for _, a := range corpus {
		in = append(in, goldenInput{"corpus:" + a.file, a.payload})
		lines := strings.Split(a.payload, "\n")
		for _, k := range nonBlankLines(lines) {
			in = append(in, goldenInput{fmt.Sprintf("corpus:%s:%d", a.file, k+1), lines[k]})
		}
		if a.expect == "clean" {
			negs = append(negs, a)
		}
	}
	if len(negs) == 0 {
		t.Fatal("no negative corpus fixtures: the golden's placements have nowhere to go")
	}
	for i, run := range goldenRunsFrom(goldenRuns) {
		in = append(in, goldenInput{fmt.Sprintf("gen:%d", i), run})
		a := negs[i%len(negs)]
		lines := strings.Split(a.payload, "\n")
		slots := nonBlankLines(lines)
		if len(slots) == 0 {
			continue
		}
		k := slots[(i/len(negs))%len(slots)]
		for _, p := range goldenPositions {
			placed := append([]string(nil), lines...)
			placed[k] = placeRun(lines[k], run, p.anchor)
			in = append(in, goldenInput{
				fmt.Sprintf("gen:%d:%s:%d:%s", i, a.file, k+1, p.name),
				strings.Join(placed, "\n"),
			})
		}
	}
	return in
}

// goldenVerdict is the scan's verdict on content, in the golden's format.
func goldenVerdict(content string) string {
	err := ScanSurfaceSecrets("golden input", []byte(content))
	if err == nil {
		return "clean"
	}
	var f *ScanFinding
	if errors.As(err, &f) && f != nil {
		return fmt.Sprintf("%s:%d", f.Rule, f.Line)
	}
	return "unexplained:0"
}

func TestBenignGoldenVerdicts(t *testing.T) {
	path := filepath.Join("testdata", goldenFile)
	inputs := goldenInputs(t)
	got := make([]string, len(inputs))
	for i, in := range inputs {
		got[i] = in.id + "\t" + goldenVerdict(in.content)
	}

	if os.Getenv(goldenRecordEnv) == "1" {
		body := strings.Join(got, "\n") + "\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("record %s: %v", path, err)
		}
		t.Logf("recorded %d verdicts to %s", len(got), path)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the golden %s: %v — an unread golden proves nothing", path, err)
	}
	want := map[string]string{}
	var refused, clean int
	for n, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		id, verdict, ok := strings.Cut(line, "\t")
		if !ok || id == "" || verdict == "" {
			t.Fatalf("%s:%d: malformed line %q (want <id> TAB <verdict>)", path, n+1, line)
		}
		if _, dup := want[id]; dup {
			t.Fatalf("%s:%d: duplicate id %s", path, n+1, id)
		}
		want[id] = verdict
		if verdict == "clean" {
			clean++
		} else {
			refused++
		}
	}
	if refused < 1000 || clean < 100 {
		t.Fatalf("golden holds %d refused and %d clean inputs; want at least 1000 and 100",
			refused, clean)
	}

	var diffs []string
	seen := map[string]bool{}
	for i, in := range inputs {
		seen[in.id] = true
		_, verdict, _ := strings.Cut(got[i], "\t")
		w, ok := want[in.id]
		switch {
		case !ok:
			diffs = append(diffs, fmt.Sprintf("%s: not in the golden (scan says %s)", in.id, verdict))
		case w != verdict:
			diffs = append(diffs, fmt.Sprintf("%s: golden %s, scan %s", in.id, w, verdict))
		}
	}
	for id := range want {
		if !seen[id] {
			diffs = append(diffs, id+": in the golden but no longer derived")
		}
	}
	if len(diffs) > 0 {
		sort.Strings(diffs)
		if len(diffs) > 40 {
			diffs = append(diffs[:40], fmt.Sprintf("… and %d more", len(diffs)-40))
		}
		t.Fatalf("%d verdicts differ from the recorded golden:\n%s", len(diffs),
			strings.Join(diffs, "\n"))
	}
	t.Logf("%d inputs match the golden (%d refused, %d clean)", len(inputs), refused, clean)
}
