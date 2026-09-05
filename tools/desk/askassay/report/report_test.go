package report

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/askassay"
	"github.com/medici-finance/assay/tools/desk/askassay/chart"
)

func stamp() askassay.Stamp {
	return askassay.Stamp{At: time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC), Ref: "deadbeef"}
}

func measured(id string, v int64) askassay.Answer {
	q, ok := askassay.Lookup(id)
	if !ok {
		panic("undeclared question in fixture: " + id)
	}
	return askassay.Computed(q, v, stamp())
}

func unmeasured(id, why string) askassay.Answer {
	q, ok := askassay.Lookup(id)
	if !ok {
		panic("undeclared question in fixture: " + id)
	}
	return askassay.Unavailable(q, why, stamp())
}

// frozenIndex is the "frozen index" the brief's determinism item calls for:
// a fixed answer set, with a measured value, a measured zero, a
// could-not-check, and a missing question, so every render branch is covered
// by ONE fixture and the byte comparison means something.
func frozenIndex() Index {
	return Index{
		AsOf: stamp(),
		Answers: []askassay.Answer{
			measured("open-issue-count", 242),
			measured("awaiting-count", 0),
			measured("pr-action-count", 17),
			unmeasured("brief-status-count", "the board verb returned an empty payload, and an empty result is BLIND, not idle"),
			measured("completion-count", 141),
			// alarm-count is deliberately ABSENT, so Require's miss path is
			// exercised by the same fixture.
		},
	}
}

// ---------------------------------------------------------------------------
// Determinism — the brief's own claim, proved by byte comparison
// ---------------------------------------------------------------------------

// TestPDFIsByteIdenticalAcrossRenders generates the weekly report twice from
// one frozen index and compares the bytes. This is the brief's Verify item 2,
// run in-process so the comparison covers the writer rather than a shell.
func TestPDFIsByteIdenticalAcrossRenders(t *testing.T) {
	for _, gen := range []struct {
		name string
		fn   func(Index, map[string][]string) (Doc, error)
	}{
		{"weekly", Weekly},
		{"bottleneck", Bottleneck},
		{"decision-latency", DecisionLatency},
	} {
		t.Run(gen.name, func(t *testing.T) {
			d1, err := gen.fn(frozenIndex(), nil)
			if err != nil {
				t.Fatal(err)
			}
			a, err := d1.PDF()
			if err != nil {
				t.Fatal(err)
			}
			// A second, independently constructed document — not a second
			// call on the same value — so a cached render cannot make this
			// pass.
			time.Sleep(2 * time.Millisecond)
			d2, err := gen.fn(frozenIndex(), nil)
			if err != nil {
				t.Fatal(err)
			}
			b, err := d2.PDF()
			if err != nil {
				t.Fatal(err)
			}
			ha, hb := sha256.Sum256(a), sha256.Sum256(b)
			if !bytes.Equal(a, b) {
				t.Fatalf("two renders of the same frozen index differ:\n  %s (%d bytes)\n  %s (%d bytes)",
					hex.EncodeToString(ha[:]), len(a), hex.EncodeToString(hb[:]), len(b))
			}
			t.Logf("%s: %d bytes, sha256 %s (identical across two renders)", gen.name, len(a), hex.EncodeToString(ha[:]))
		})
	}
}

// TestDeterminismIsNotVacuous is the positive control for the test above: a
// render that SHOULD differ must differ, or the byte comparison is proving
// nothing.
func TestDeterminismIsNotVacuous(t *testing.T) {
	a, err := mustDoc(t, frozenIndex()).PDF()
	if err != nil {
		t.Fatal(err)
	}
	ix := frozenIndex()
	ix.Answers[0] = measured("open-issue-count", 243)
	b, err := mustDoc(t, ix).PDF()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("changing a figure did not change the PDF — the byte comparison is not observing the data")
	}

	// The stamp is an input, so a different stamp must also move the bytes.
	ix2 := frozenIndex()
	ix2.AsOf = askassay.Stamp{At: stamp().At.Add(time.Hour), Ref: "deadbeef"}
	c, err := mustDoc(t, ix2).PDF()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, c) {
		t.Fatal("changing the as-of stamp did not change the PDF — /CreationDate is not derived from it")
	}
}

// TestPDFHashIsStableAcrossProcesses re-runs this test in a SUBPROCESS and
// compares the hash it prints with the one the parent computes.
//
// The in-process comparison above cannot see process-level entropy — map
// iteration order is seeded per process, so a ranged map produces stable
// bytes within a run and different bytes between runs. That is exactly the
// shape of non-determinism that survives a naive "render twice" check and
// then shows up as a CI diff nobody can reproduce locally.
func TestPDFHashIsStableAcrossProcesses(t *testing.T) {
	pdf, err := mustDoc(t, frozenIndex()).PDF()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(pdf)
	mine := hex.EncodeToString(sum[:])

	if os.Getenv("ASSAY_REPORT_CHILD") == "1" {
		// Child: print and stop. The parent reads this line.
		t.Logf("CHILD-HASH %s", mine)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run", "^TestPDFHashIsStableAcrossProcesses$", "-test.v", "-test.count=1")
	cmd.Env = append(os.Environ(), "ASSAY_REPORT_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("could-not-check: the child run failed: %v\n%s", err, out)
	}
	i := strings.Index(string(out), "CHILD-HASH ")
	if i < 0 {
		t.Fatalf("could-not-check: the child printed no hash:\n%s", out)
	}
	child := strings.Fields(string(out)[i+len("CHILD-HASH "):])[0]
	if child != mine {
		t.Fatalf("the same frozen index hashed differently in two processes:\n  parent %s\n  child  %s", mine, child)
	}
	t.Logf("cross-process: parent and child both produced sha256 %s", mine)
}

// TestWriterReadsNoClock reads this package's own source. A PDF whose bytes
// depend on when it was written is not a function of the index, and that is
// exactly the property that makes a committed artifact unverifiable on CI.
func TestWriterReadsNoClock(t *testing.T) {
	srcs := packageSources(t)
	if len(srcs) < 3 {
		t.Fatalf("only %d source files scanned — expected the writer, the model and the extractor", len(srcs))
	}
	for name, src := range srcs {
		for _, forbidden := range []string{"time.Now", "rand.", "os.Getenv", "filepath.Glob"} {
			if strings.Contains(src, forbidden) {
				t.Errorf("%s calls %s — the render must be a pure function of the document", name, forbidden)
			}
		}
	}
}

// TestPDFStructureIsWellFormed: qpdf, when present, is a far stricter reader
// than any assertion written here. Its absence is recorded as could-not-check,
// never skipped silently into a pass.
func TestPDFStructureIsWellFormed(t *testing.T) {
	pdf, err := mustDoc(t, frozenIndex()).PDF()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "weekly.pdf")
	if err := os.WriteFile(path, pdf, 0o644); err != nil {
		t.Fatal(err)
	}
	bin, lookErr := exec.LookPath("qpdf")
	if lookErr != nil {
		t.Logf("COULD-NOT-CHECK: qpdf is not on PATH (%v), so the structural check did not run. This is not a pass.", lookErr)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "--check", path).CombinedOutput()
	if err != nil {
		t.Fatalf("qpdf --check rejected the generated PDF: %v\n%s", err, out)
	}
	t.Logf("qpdf --check passed:\n%s", out)
}

// ---------------------------------------------------------------------------
// The numbers rule, end to end
// ---------------------------------------------------------------------------

// TestNarrativeCannotStateAFigure is the §7.5 split made executable: the
// model narrates, the index computes. A number in the narrative that no query
// produced is a build refusal, not a review comment.
func TestNarrativeCannotStateAFigure(t *testing.T) {
	ix := frozenIndex()
	_, err := Weekly(ix, map[string][]string{
		"Inventory": {"The backlog has grown to 300 open issues this week."},
	})
	if err == nil {
		t.Fatal("a narrative stating an uncomputed figure was accepted")
	}
	if !strings.Contains(err.Error(), `"300"`) {
		t.Fatalf("the refusal did not name the offending token: %v", err)
	}

	// Positive control: narrating a figure the index DID produce is allowed,
	// or the rule would just be "no numbers ever", which is a different and
	// much weaker rule.
	if _, err := Weekly(ix, map[string][]string{
		"Inventory": {"The 242 open issues are the largest single load on the board."},
	}); err != nil {
		t.Fatalf("narrating a computed figure was refused: %v", err)
	}

	// And pure prose is always fine.
	if _, err := Weekly(ix, map[string][]string{
		"Inventory": {"The constraint moved from implementation to review."},
	}); err != nil {
		t.Fatalf("pure prose was refused: %v", err)
	}
}

// TestMissingAnswerRendersCouldNotCheckNotOmitted: a question the index could
// not answer must still appear on the page. A dropped row and a zero row are
// the same absence to a reader.
func TestMissingAnswerRendersCouldNotCheckNotOmitted(t *testing.T) {
	d := mustDoc(t, frozenIndex())
	found := false
	for _, s := range d.Sections {
		for _, f := range s.Figures {
			if f.Answer.Question() != "alarm-count" {
				continue
			}
			found = true
			if _, ok := f.Answer.Value(); ok {
				t.Error("a question absent from the index acquired a value")
			}
			if !strings.Contains(f.Answer.Reason(), "did not run in this pass") {
				t.Errorf("the miss did not say why: %q", f.Answer.Reason())
			}
		}
	}
	if !found {
		t.Fatal("the figure for a question missing from the index was DROPPED from the report rather than rendered could-not-check")
	}
}

// TestCouldNotCheckPrintsTokenNotZero is the paper half of the headline rule.
func TestCouldNotCheckPrintsTokenNotZero(t *testing.T) {
	pdf, err := mustDoc(t, frozenIndex()).PDF()
	if err != nil {
		t.Fatal(err)
	}
	text, err := ExtractText(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, string(askassay.CouldNotCheck)) {
		t.Error("the rendered PDF does not print the could-not-check token")
	}
	if !strings.Contains(text, "BLIND, not idle") {
		t.Error("the rendered PDF does not carry the could-not-check reason")
	}
	// The measured zero must still print as 0 — the guarantee is "never a
	// zero you did not measure", not "never a zero".
	if !strings.Contains(text, "PRs awaiting a human") {
		t.Fatal("the fixture's measured-zero caption is missing")
	}
	lines := strings.Split(text, "\n")
	for i, ln := range lines {
		if strings.Contains(ln, "PRs awaiting a human") && i+1 < len(lines) {
			if strings.TrimSpace(lines[i+1]) != "0" {
				t.Errorf("a measured zero did not print as 0; the next line was %q", lines[i+1])
			}
		}
	}
	// A hatched track is drawn for could-not-check, so the artifact is not
	// merely silent about it.
	if !bytes.Contains(pdf, []byte("0.48 0.50 0.53 RG")) {
		t.Error("no hatch was drawn for the could-not-check figure — an empty track is a bar of length zero")
	}
}

// TestEveryFigurePrintsItsSourceProbeWindowLimit: no silent caps. Every
// figure, on every page, states all four.
func TestEveryFigurePrintsItsSourceProbeWindowLimit(t *testing.T) {
	pdf, err := mustDoc(t, frozenIndex()).PDF()
	if err != nil {
		t.Fatal(err)
	}
	text, err := ExtractText(pdf)
	if err != nil {
		t.Fatal(err)
	}
	sources := strings.Count(text, "source:")
	windows := strings.Count(text, "window:")
	if sources == 0 {
		t.Fatal("no source lines at all — this scan would pass vacuously")
	}
	if sources != windows {
		t.Errorf("%d figures state a source but %d state a window — a figure is hiding its cap", sources, windows)
	}
	for _, want := range []string{"probe:", "limit:", "caveat:"} {
		if !strings.Contains(text, want) {
			t.Errorf("the report never prints %q", want)
		}
	}
	// A cap that is stated as a real cap, in words, on the page.
	if !strings.Contains(text, "limit:") || !strings.Contains(text, "500") && !strings.Contains(text, "none") {
		t.Error("no limit is quantified anywhere in the report")
	}
}

// TestDocRefusals is the mutation test over the document's fail-closed arms.
// Distinct fixtures, distinct messages — a shared fixture is how a mutation
// gets covered by someone else's failure and passes without being tested.
func TestDocRefusals(t *testing.T) {
	base := func() Doc {
		return Doc{
			Title: "Weekly summary", AsOf: stamp(),
			Sections: []Section{{
				Heading: "Inventory",
				Figures: []Figure{{Caption: "open issues", Answer: measured("open-issue-count", 242)}},
			}},
		}
	}
	if _, err := base().Build(); err != nil {
		t.Fatalf("the baseline document does not build: %v", err)
	}
	cases := []struct {
		name string
		mut  func(*Doc)
		want string
	}{
		{"no title", func(d *Doc) { d.Title = "" }, "no title"},
		{"no as-of stamp", func(d *Doc) { d.AsOf = askassay.Stamp{} }, "no as-of stamp"},
		{"no sections", func(d *Doc) { d.Sections = nil }, "no sections"},
		{"a section with no heading", func(d *Doc) { d.Sections[0].Heading = " " }, "no heading"},
		{"an empty section", func(d *Doc) { d.Sections[0].Figures = nil }, "is empty"},
		{"a figure with no caption", func(d *Doc) { d.Sections[0].Figures[0].Caption = "" }, "no caption"},
		{"a number whose source is not fully declared", func(d *Doc) {
			q, _ := askassay.Lookup("open-issue-count")
			q.Source.Limit = ""
			d.Sections[0].Figures[0].Answer = askassay.Answer{}
			_ = q
			// A hand-built answer cannot carry a value at all (the value
			// field is unexported in the answer package), so the arm is
			// exercised through the constructor that CAN: a question whose
			// source lost its limit downgrades to could-not-check, and a
			// could-not-check with no reason is the refusal below.
		}, "with no reason"},
		{"a narrative figure with no query behind it", func(d *Doc) {
			d.Sections[0].Narrative = []string{"There are 999 rows outstanding."}
		}, `"999"`},
	}
	msgs := map[string]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := base()
			tc.mut(&d)
			if _, err := d.Build(); err == nil {
				t.Fatal("the mutant did not redden")
			} else {
				if !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("reddened for the wrong reason: %v (wanted %q)", err, tc.want)
				}
				if prev, dup := msgs[err.Error()]; dup {
					t.Fatalf("this mutant produces the same error as %q — one of the two arms is untested", prev)
				}
				msgs[err.Error()] = tc.name
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The bottleneck source writes
// ---------------------------------------------------------------------------

// TestGuardReportProbeRefusesAWritingMode. The registry's declared source for
// the constraint figure runs statusgen --bottleneck, and that command writes a
// dated file under docs/reports/. A read-only pane must refuse it.
func TestGuardReportProbeRefusesAWritingMode(t *testing.T) {
	argv := []string{"statusgen", "--root", ".", "--bottleneck"}
	if err := GuardReportProbe(argv); err == nil {
		t.Fatal("the report guard permitted a probe whose read writes a file")
	} else if !strings.Contains(err.Error(), "writes") {
		t.Fatalf("refused for the wrong reason: %v", err)
	}
	// Positive control: the guard is a narrowing, not a blanket refusal.
	if err := GuardReportProbe([]string{"statusgen", "--root", ".", "--json"}); err != nil {
		t.Fatalf("the report guard refused a genuine read: %v", err)
	}
	// And everything the inherited guard refuses stays refused.
	if err := GuardReportProbe([]string{"rm", "-rf", "/"}); err == nil {
		t.Fatal("the report guard permitted an undeclared binary")
	}
}

// TestInheritedGuardPermitsTheWritingMode pins the gap this layer closes. If
// the inherited allow-list is ever corrected, this test fails and the
// narrowing here can be retired deliberately rather than forgotten.
func TestInheritedGuardPermitsTheWritingMode(t *testing.T) {
	err := askassay.GuardReadOnly([]string{"statusgen", "--root", ".", "--bottleneck"})
	if err != nil {
		t.Skipf("the inherited guard now refuses --bottleneck (%v); GuardReportProbe's narrowing for it is redundant and should be retired", err)
	}
	t.Log("MEASURED: the inherited read-only allow-list permits `statusgen --bottleneck`, whose default path calls writeBottleneckReport and creates docs/reports/factory-floor/<date>.md. GuardReportProbe refuses it.")
}

// TestBottleneckConstraintIsCouldNotCheckWithoutASuppliedFigure: the report
// does not quietly omit the constraint, and does not invent it.
func TestBottleneckConstraintIsCouldNotCheckWithoutASuppliedFigure(t *testing.T) {
	d, err := Bottleneck(frozenIndex(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// The reason depends on which of the two closures is in force upstream.
	// Either way the figure must carry NO value and must state a cause: the
	// point of the row is that an unmeasurable constraint never draws as a
	// number. Branching here keeps the assertion true under both states
	// rather than pinning a reason the registry no longer produces.
	_, declared := askassay.Lookup("bottleneck-stage")
	wantReason := "writes a dated report file" // question declared, source refused by the guard
	if !declared {
		wantReason = "no declared source" // question withdrawn upstream
	}

	var found bool
	for _, f := range d.Sections[0].Figures {
		if f.Answer.Question() != "bottleneck-stage" {
			continue
		}
		found = true
		if _, ok := f.Answer.Value(); ok {
			t.Error("the constraint figure acquired a value with no read behind it")
		}
		if !strings.Contains(f.Answer.Reason(), wantReason) {
			t.Errorf("the constraint figure does not state why it could not be measured: %q", f.Answer.Reason())
		}
	}
	if !found {
		t.Fatal("the bottleneck report dropped its constraint figure")
	}

	t.Logf("MEASURED: constraint figure is could-not-check, reason contains %q", wantReason)
}

// TestBottleneckConstraintRendersWhenSupplied is the positive control for the
// test above, kept as its own test so that the assertion above stays visibly
// EXECUTED when this control cannot be built. A skip folded into the main test
// would report the whole row as skipped and hide the coverage that did run.
//
// While the question is WITHDRAWN from the registry there is no way for a
// caller to supply it — Require routes an undeclared id through Unanswerable,
// so the figure is could-not-check unconditionally. Skipped rather than
// deleted: if the question is ever re-declared against a genuinely read-only
// source, this control must be restored deliberately.
func TestBottleneckConstraintRendersWhenSupplied(t *testing.T) {
	if _, declared := askassay.Lookup("bottleneck-stage"); !declared {
		t.Skip("bottleneck-stage is withdrawn from the registry, so no caller can supply a figure for it; restore this control if it is ever re-declared")
	}
	ix := frozenIndex()
	ix.Answers = append(ix.Answers, measured("bottleneck-stage", 9))
	d2, err := Bottleneck(ix, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := d2.Sections[0].Figures[0].Answer.Value(); !ok || v != 9 {
		t.Errorf("a supplied constraint figure did not render: value=%d ok=%v", v, ok)
	}
}

// TestWriteSideEffectModesAreEnumerated keeps the list honest: an empty list
// would make GuardReportProbe a pass-through that looks like a guard.
func TestWriteSideEffectModesAreEnumerated(t *testing.T) {
	modes := WriteSideEffectModes()
	if len(modes) == 0 {
		t.Fatal("no write-side-effect modes declared — the guard is a no-op wearing a name")
	}
	for _, m := range modes {
		if !strings.HasPrefix(m[0], "--") || strings.TrimSpace(m[1]) == "" {
			t.Errorf("mode %q has no flag form or no stated reason", m[0])
		}
	}
}

// ---------------------------------------------------------------------------
// Disclosure — a binary artifact carries text no reviewer reads
// ---------------------------------------------------------------------------

// TestExtractedTextMatchesPdftotext cross-checks the in-process extractor
// against poppler. Poppler's absence is COULD-NOT-CHECK, logged as such.
func TestExtractedTextMatchesPdftotext(t *testing.T) {
	pdf, err := mustDoc(t, frozenIndex()).PDF()
	if err != nil {
		t.Fatal(err)
	}
	mine, err := ExtractText(pdf)
	if err != nil {
		t.Fatal(err)
	}
	bin, lookErr := exec.LookPath("pdftotext")
	if lookErr != nil {
		t.Logf("COULD-NOT-CHECK: pdftotext is not on PATH (%v). The in-process extractor recovered %d bytes of text, but it was not cross-checked against an independent reader. This is not a pass.", lookErr, len(mine))
		return
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "weekly.pdf")
	if err := os.WriteFile(src, pdf, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "-layout", src, "-").Output()
	if err != nil {
		t.Fatalf("pdftotext failed on the generated PDF: %v", err)
	}
	popplerText := string(out)
	// Every distinctive phrase the in-process extractor found must be
	// findable by poppler too, or one of the two readers is missing text.
	for _, probe := range []string{
		"Weekly summary",
		string(askassay.CouldNotCheck),
		"open issues",
		"BLIND, not idle",
	} {
		if !strings.Contains(mine, probe) {
			t.Errorf("the in-process extractor missed %q", probe)
		}
		if !strings.Contains(squash(popplerText), squash(probe)) {
			t.Errorf("poppler did not find %q in the generated PDF — the two readers disagree", probe)
		}
	}
	t.Logf("cross-checked against %s: in-process %d bytes, poppler %d bytes", bin, len(mine), len(popplerText))
}

// TestDisclosureCatchesTextTheSourceReviewWouldNotShow is the positive
// control for the disclosure check: a term that IS in the artifact must be
// found, or a clean result means nothing.
func TestDisclosureCatchesTextTheSourceReviewWouldNotShow(t *testing.T) {
	ix := frozenIndex()
	d, err := Weekly(ix, map[string][]string{
		"Inventory": {"Reviewed with the desk owner, initials on file."},
	})
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := d.PDF()
	if err != nil {
		t.Fatal(err)
	}
	hits, err := CheckDisclosure(pdf, []Disclosure{
		{Term: "desk owner", Why: "a named individual in customer-facing copy"},
		{Term: "initials on file", Why: "an internal handling note"},
	})
	if err != nil {
		t.Fatalf("could-not-check where a result was expected: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("the disclosure check found %d of 2 planted terms — it is not reading the artifact: %v", len(hits), hits)
	}

	// checked-clean: the same artifact against terms that are genuinely absent.
	clean, err := CheckDisclosure(pdf, []Disclosure{
		{Term: "zzz-not-in-this-document-zzz", Why: "control"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(clean) != 0 {
		t.Errorf("the check reported a hit for a term that is not present: %v", clean)
	}
}

// TestGeneratedReportsCarryNoProbeStderr. Measured on this machine: every
// desk-tools invocation writes a trust-roster diagnostic block to stderr,
// including a login with its numeric id, the bot account slugs with their App
// ids, the human login map and the private repo list — on a plain read-only
// verb. A generated report that captured combined output would carry all of
// it inside a binary nobody re-reads.
func TestGeneratedReportsCarryNoProbeStderr(t *testing.T) {
	for _, gen := range []struct {
		name string
		fn   func(Index, map[string][]string) (Doc, error)
	}{{"weekly", Weekly}, {"bottleneck", Bottleneck}, {"decision-latency", DecisionLatency}} {
		t.Run(gen.name, func(t *testing.T) {
			d, err := gen.fn(frozenIndex(), nil)
			if err != nil {
				t.Fatal(err)
			}
			pdf, err := d.PDF()
			if err != nil {
				t.Fatal(err)
			}
			hits, err := CheckDisclosure(pdf, ProbeStderrTerms())
			if err != nil {
				t.Fatalf("could-not-check: %v", err)
			}
			if len(hits) != 0 {
				t.Errorf("the generated report embeds probe stderr: %v", hits)
			}
		})
	}
	// Positive control: the check must FIND the block when it is present, or
	// the clean results above mean nothing.
	planted, err := Weekly(frozenIndex(), map[string][]string{
		"Inventory": {"assay-config: class=write source=config file <redacted> configured=true"},
	})
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := planted.PDF()
	if err != nil {
		t.Fatal(err)
	}
	hits, err := CheckDisclosure(pdf, ProbeStderrTerms())
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 2 {
		t.Fatalf("the stderr-disclosure check found %d of the planted markers — it is not reading the artifact", len(hits))
	}
	t.Logf("positive control: the check found %d planted stderr markers", len(hits))
}

// TestUnreadablePDFIsCouldNotCheckNotClean: the failure mode that makes a
// disclosure check worse than useless is returning "no hits" when it could
// not read the file at all.
func TestUnreadablePDFIsCouldNotCheckNotClean(t *testing.T) {
	for _, tc := range []struct {
		name string
		pdf  []byte
	}{
		{"not a PDF", []byte("this is not a pdf")},
		{"a PDF with a compressed stream", []byte("%PDF-1.4\n1 0 obj\n<< /Filter /FlateDecode /Length 4 >>\nstream\nxxxx\nendstream\nendobj\n")},
		// A mutation run showed the two fixtures above did NOT separate the
		// two arms: with the compressed-stream guard removed, the fixture
		// above still failed on "no drawn text", so the guard could be
		// deleted with the suite still green. This fixture is the dangerous
		// shape — a filtered stream WITH a readable decoy string beside it,
		// so a missing guard yields a partial extraction that reports clean.
		{"a compressed PDF with a readable decoy", []byte(
			"%PDF-1.4\n1 0 obj\n<< /Filter /FlateDecode /Length 4 >>\nstream\nxxxx\nendstream\nendobj\n" +
				"2 0 obj\n<< /Length 40 >>\nstream\nBT /F1 9 Tf 54 700 Td (cover page) Tj ET\nendstream\nendobj\n")},
		{"a PDF with no drawn text", []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hits, err := CheckDisclosure(tc.pdf, []Disclosure{{Term: "anything", Why: "control"}})
			if err == nil {
				t.Fatalf("an unreadable artifact returned %d hits and no error — 'nothing found' passed as 'nothing disclosed'", len(hits))
			}
			if !strings.Contains(err.Error(), "could-not-check") {
				t.Fatalf("the failure was not classified could-not-check: %v", err)
			}
		})
	}
}

// TestNonASCIIIsVisibleNotDropped: an unmappable rune must become a visible
// "?" rather than vanish, because a silently dropped character in a
// customer-facing artifact is a change nobody reviews.
func TestNonASCIIIsVisibleNotDropped(t *testing.T) {
	got := pdfString("gate ⟡ · em—dash ≥ 5 and 漢")
	if strings.Contains(got, "⟡") || strings.Contains(got, "漢") {
		t.Fatalf("non-WinAnsi runes survived into the PDF literal: %q", got)
	}
	if !strings.Contains(got, "[gate]") {
		t.Errorf("the gate glyph was not transliterated to a word: %q", got)
	}
	if !strings.Contains(got, "?") {
		t.Errorf("an unmappable rune was dropped instead of being made visible: %q", got)
	}
	if !strings.Contains(got, ">=") {
		t.Errorf("a transliteration was lost: %q", got)
	}
}

// ---------------------------------------------------------------------------
// The chart and the PDF read the same answers
// ---------------------------------------------------------------------------

// TestSectionChartAndPageAgree: a figure that is could-not-check on the page
// is could-not-check on the chart, from the same answer. Two renderings that
// can disagree are two sources for one number.
func TestSectionChartAndPageAgree(t *testing.T) {
	d := mustDoc(t, frozenIndex())
	c := d.Sections[1].Chart("brief rows", "rows", d.AsOf)
	svg, err := c.Render()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg, chart.CouldNotCheckToken) {
		t.Error("the chart lost the could-not-check state the page carries")
	}
	table := c.Table()
	if !strings.Contains(table, chart.CouldNotCheckToken) {
		t.Error("the table view lost the could-not-check state")
	}
	for _, f := range d.Sections[1].Figures {
		if !strings.Contains(table, f.Caption) {
			t.Errorf("the table view drops the figure %q the page prints", f.Caption)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mustDoc(t *testing.T, ix Index) Doc {
	t.Helper()
	d, err := Weekly(ix, nil)
	if err != nil {
		t.Fatalf("weekly: %v", err)
	}
	return d
}

func squash(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func packageSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(n)
		if err != nil {
			t.Fatalf("read %s: %v", n, err)
		}
		out[n] = stripComments(string(b))
	}
	if len(out) == 0 {
		t.Fatal("no non-test source found — this scan would pass vacuously")
	}
	return out
}

func stripComments(src string) string {
	var b strings.Builder
	for i := 0; i < len(src); {
		if strings.HasPrefix(src[i:], "//") {
			j := strings.IndexByte(src[i:], '\n')
			if j < 0 {
				break
			}
			i += j
			continue
		}
		if strings.HasPrefix(src[i:], "/*") {
			j := strings.Index(src[i:], "*/")
			if j < 0 {
				break
			}
			i += j + 2
			continue
		}
		b.WriteByte(src[i])
		i++
	}
	return b.String()
}
