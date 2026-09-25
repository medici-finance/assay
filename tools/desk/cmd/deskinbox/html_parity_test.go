package main

// html_parity_test.go — TestParityHTML: asserts buildDecisionPage/buildFlowOnlyPage
// (html.go/flowhtml.go) byte-for-byte against the REAL jq program the oracle ships
// (write_html_program, assay-inbox.sh's JQHTML heredoc), extracted verbatim at test time and
// run through the system `jq` binary on identical fixture input (an items array in the
// oracle's own per-item shape, plus a flow-model document from flow_parity_test.go's own
// fixtures) — the same discipline format_parity_test.go and flow_parity_test.go established.
//
// This is the direct check the brief's Review question 2 asks for: does the html renderer
// produce a genuinely self-contained page, through the SAME builders walk.go/flow.go use,
// with no second copy of either the five-part logic or the flow-model logic having drifted.
//
// Needs `jq` on the runner. Its absence is could-not-check (requireJQ), never a silent pass.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// extractJQHTMLProgram pulls the write_html_program() JQHTML heredoc body verbatim out of
// the oracle script.
func extractJQHTMLProgram(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(oracleScriptRelPath)
	if err != nil {
		t.Skipf("could-not-check: cannot read the oracle script at %s: %v", oracleScriptRelPath, err)
	}
	lines := strings.Split(string(raw), "\n")
	start, end := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "cat > \"$TMP_HTML\" <<'JQHTML'") {
			start = i + 1
			continue
		}
		if start != -1 && l == "JQHTML" {
			end = i
			break
		}
	}
	if start == -1 || end == -1 {
		t.Fatalf("could not locate the write_html_program JQHTML heredoc in %s (start=%d end=%d) — "+
			"the oracle's own contract may have moved; re-locate the markers before trusting this test",
			oracleScriptRelPath, start, end)
	}
	return strings.Join(lines[start:end], "\n")
}

// runOracleHTML runs the extracted JQHTML program exactly as the `html`/`flowhtml` MODE
// cases invoke it (assay-inbox.sh:1376-1377, 1395-1396): `jq -r --arg summary … --argjson
// flowonly … --slurpfile flowdoc FLOW -f prog ITEMS`.
func runOracleHTML(t *testing.T, program, summary string, flowOnly bool, flowDocFile, itemsFile string) []byte {
	t.Helper()
	dir := t.TempDir()
	progFile := filepath.Join(dir, "html.jq")
	if err := os.WriteFile(progFile, []byte(program), 0o644); err != nil {
		t.Fatalf("write program: %v", err)
	}
	flowOnlyArg := "false"
	if flowOnly {
		flowOnlyArg = "true"
	}
	out := runCmd(t, "jq", "-r", "--arg", "summary", summary, "--argjson", "flowonly", flowOnlyArg,
		"--slurpfile", "flowdoc", flowDocFile, "-f", progFile, itemsFile)
	return out
}

func runCmd(t *testing.T, name string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s failed: %v\nstderr: %s", name, err, errb.String())
	}
	return out.Bytes()
}

// jqHTMLItem is the oracle's per-item object (write_format_program's output — see
// format_parity_test.go's jqRenderedOut — PLUS the source fields walk.go/html.go hold on the
// item itself: repo/number/url/title/index/total). It is what build_item appends to
// $TMP_ITEMS and what write_html_program's card renderer reads.
type jqHTMLItem struct {
	Repo    string   `json:"repo"`
	Number  int      `json:"number"`
	URL     string   `json:"url"`
	Title   string   `json:"title"`
	Index   int      `json:"index"`
	Total   int      `json:"total"`
	Header  string   `json:"header"`
	Context []string `json:"context"`
	Options []struct {
		Letter      string `json:"letter"`
		Text        string `json:"text"`
		Recommended bool   `json:"recommended"`
	} `json:"options"`
	OptionsStated bool   `json:"optionsStated"`
	Reply         string `json:"reply"`
	Verification  string `json:"verification"`
	Unread        bool   `json:"unread"`
	// Class/ClassEvidence are the screen's per-item annotation (the oracle's `.class` /
	// `.classEvidence`). omitempty keeps an unclassed item's JSON exactly the oracle's
	// `--no-screen` shape (no key at all), which is what deskinbox's own page renders.
	Class         string `json:"class,omitempty"`
	ClassEvidence string `json:"classEvidence,omitempty"`
}

func htmlFixtureItems() []jqHTMLItem {
	return []jqHTMLItem{
		{
			Repo: "example-org/example-repo", Number: 42,
			URL: "https://example.invalid/issues/42", Title: "Example decision",
			Index: 1, Total: 3,
			Header:  "example-org/example-repo#42 — question 1 of 3",
			Context: []string{"The queue is stuck on X.", "blocked on the driver: carries needs-decision, urgent", "gate: needs-decision, urgent · age 3d · https://example.invalid/issues/42"},
			Options: []struct {
				Letter      string `json:"letter"`
				Text        string `json:"text"`
				Recommended bool   `json:"recommended"`
			}{{Letter: "A", Text: "Do the risky thing", Recommended: true}, {Letter: "B", Text: "Do the safe thing"}},
			OptionsStated: true,
			Reply:         "reply with one letter (A/B) — nothing else is needed.",
			Verification:  "the desk records the ruling on example-org/example-repo#42 as a relayed decision, presents question 2 of 3.",
		},
		{
			Repo: "example-org/example-repo", Number: 9,
			URL: "https://example.invalid/issues/9", Title: "Unreadable <item> & \"quoted\"",
			Index: 2, Total: 3,
			Header:        "example-org/example-repo#9 — question 2 of 3",
			Context:       []string{"could-not-check: this issue's body and comments could not be read (detail fetch failed) — the item is UNREAD, not empty"},
			OptionsStated: false,
			Options: []struct {
				Letter      string `json:"letter"`
				Text        string `json:"text"`
				Recommended bool   `json:"recommended"`
			}{{Letter: "A", Text: "options not yet stated — desk to fill", Recommended: true}},
			Reply:        "reply with the ruling in one line; the desk restates it as lettered options before acting.",
			Verification: "the desk records the ruling on example-org/example-repo#9 as a relayed decision, presents question 3 of 3.",
			Unread:       true,
		},
		{
			// A classed item: pins the card's `<p class="cls">` render branch (class AND its
			// evidence, both escaped) against the oracle's own, so the page stays byte-identical
			// once a classifier supplies the two strings.
			Repo: "example-org/example-repo", Number: 7,
			URL: "https://example.invalid/issues/7", Title: "Already answered",
			Index: 3, Total: 3,
			Header:  "example-org/example-repo#7 — question 3 of 3",
			Context: []string{"The driver already ruled on this."},
			Options: []struct {
				Letter      string `json:"letter"`
				Text        string `json:"text"`
				Recommended bool   `json:"recommended"`
			}{{Letter: "A", Text: "Keep it", Recommended: true}, {Letter: "B", Text: "Drop it"}},
			OptionsStated: true,
			Reply:         "reply with one letter (A/B) — nothing else is needed.",
			Verification:  "the desk records the ruling on example-org/example-repo#7 as a relayed decision, reports the queue drained.",
			Class:         "already-ruled",
			ClassEvidence: "ruling <A> after the newest ask & \"quoted\"",
		},
	}
}

func TestParityHTML(t *testing.T) {
	requireJQ(t)
	program := extractJQHTMLProgram(t)
	flowProgram := extractJQFLOWProgram(t)

	// EVERY flow fixture, not one representative: TestParityFlow proves the MODEL, but
	// flowhtml.go has its own rendering branches for a blind stage (could-not-check, never
	// 0), an n/a stage, and an AT LEAST partial count — only the blind/partial fixtures reach
	// them, so iterating only the all-ok fixture would let a could-not-check→0 rendering
	// regression pass (the drained-vs-unread confusion the flow model exists to prevent).
	for _, fx := range flowFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			dir := t.TempDir()
			rawFile := filepath.Join(dir, "raw.json")
			if err := os.WriteFile(rawFile, []byte(fx.raw), 0o644); err != nil {
				t.Fatalf("write raw fixture: %v", err)
			}
			modelBytes := runCmd(t, "jq", "-f", writeProgFile(t, dir, "flow.jq", flowProgram), rawFile)
			modelFile := filepath.Join(dir, "model.json")
			if err := os.WriteFile(modelFile, modelBytes, 0o644); err != nil {
				t.Fatalf("write model file: %v", err)
			}
			var doc rawDoc
			if err := json.Unmarshal([]byte(fx.raw), &doc); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			model := interpretFlow(doc)

			items := htmlFixtureItems()
			itemsBytes, _ := json.Marshal(items)
			itemsFile := filepath.Join(dir, "items.json")
			if err := os.WriteFile(itemsFile, itemsBytes, 0o644); err != nil {
				t.Fatalf("write items file: %v", err)
			}

			summary := "deskinbox: 3 item(s) across 1 repo(s)"
			want := runOracleHTML(t, program, summary, false, modelFile, itemsFile)

			var cards []cardData
			for _, it := range items {
				var opts []option
				for _, o := range it.Options {
					opts = append(opts, option{Letter: o.Letter, Text: o.Text, Recommended: o.Recommended})
				}
				cards = append(cards, cardData{
					Item:  item{Repo: it.Repo, Number: it.Number, URL: it.URL, Title: it.Title},
					Index: it.Index, Total: it.Total,
					Rendered: rendered{
						Context: it.Context, Options: opts, OptionsStated: it.OptionsStated,
						Reply: it.Reply, Verification: it.Verification, Unread: it.Unread,
					},
					Class: it.Class, ClassEvidence: it.ClassEvidence,
				})
			}
			got := buildDecisionPage(summary, cards, model)

			if got != string(want) {
				t.Errorf("%s: decision page mismatch\n got:\n%s\n\nwant:\n%s", fx.name, got, string(want))
			}

			// The flow-only page (`deskinbox flow --html`) shares the SAME flowSectionLines —
			// checked here too so a drift in either page's wrapper is caught.
			wantFlowOnly := runOracleHTML(t, program, "", true, modelFile, writeProgFile(t, dir, "empty.json", "[]"))
			gotFlowOnly := buildFlowOnlyPage(model)
			if gotFlowOnly != string(wantFlowOnly) {
				t.Errorf("%s: flow-only page mismatch\n got:\n%s\n\nwant:\n%s", fx.name, gotFlowOnly, string(wantFlowOnly))
			}
		})
	}
}

func writeProgFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// TestSelfContainedPage is a direct, standing check of the property Verify row 4 asserts at
// the CLI level (no `url(`, `<script`, ` src=`) — kept here too so a regression is caught by
// `go test` alone, without needing a live forge or statusgen/deskboard binary.
func TestSelfContainedPage(t *testing.T) {
	m := flowModel{AsOf: "now", Since: "always", Sources: []string{"x"}, Blind: []string{}}
	page := buildDecisionPage("deskinbox: 0 item(s) across 0 repo(s)", nil, m)
	for _, forbidden := range []string{"url(", "<script", " src="} {
		if strings.Contains(page, forbidden) {
			t.Errorf("decision page contains forbidden substring %q", forbidden)
		}
	}
	flowPage := buildFlowOnlyPage(m)
	for _, forbidden := range []string{"url(", "<script", " src="} {
		if strings.Contains(flowPage, forbidden) {
			t.Errorf("flow-only page contains forbidden substring %q", forbidden)
		}
	}
	if !strings.Contains(page, "<!doctype html>") {
		t.Errorf("decision page missing doctype")
	}
}
