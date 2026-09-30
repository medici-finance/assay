package main

// html_loginshape_parity_test.go — TestParityHTMLLoginShapes: the `--html` half of #1797.
//
// runHTML builds every card from the SAME fetchDetail reader walk uses, so the REST-vs-gh
// login-shape divergence reaches the page too. TestParityHTML cannot see it: it hands both
// sides already-rendered items. This test starts one step earlier, from the wire: the oracle
// side runs the gh-shaped detail through the real JQFMT program (build_item) and then the
// real JQHTML program; the Go side reads the REST-shaped detail through the real fetchDetail,
// then buildRendered and buildDecisionPage — the exact chain runHTML walks. The pages must be
// byte-identical.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestParityHTMLLoginShapes(t *testing.T) {
	requireJQ(t)
	fmtProgram := extractJQFMTProgram(t)
	htmlProgram := extractJQHTMLProgram(t)
	flowProgram := extractJQFLOWProgram(t)

	now := time.Now()
	createdAt := now.Add(-30 * 24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	repo := deskkit.ForgeRepo{Owner: "example-org", Name: "example-repo"}

	type wireItem struct {
		it       jqItem
		body     string
		comments []wireComment
	}
	queue := []wireItem{
		{
			it: jqItem{
				Repo: repo.Slug(), Number: 43, Title: "Selection across wire shapes",
				URL: "https://example.invalid/issues/43", CreatedAt: createdAt,
				Labels: []jqLabel{{Name: "needs-decision"}},
			},
			body: "## Context\nA ruling is needed on Z.\n\n## Options\nA. Keep Z\nB. Drop Z\n",
			comments: []wireComment{
				appComment("example-desk-app", "desk relay: options restated, awaiting the driver."),
				appComment("example-worker-app", "worker: pushed a fix to the branch."),
			},
		},
		{
			it: jqItem{
				Repo: repo.Slug(), Number: 44, Title: "Label across wire shapes",
				URL: "https://example.invalid/issues/44", CreatedAt: createdAt,
				Labels: []jqLabel{{Name: "question"}},
			},
			body: "## Context\nWhich way on W?\n",
			comments: []wireComment{
				userComment("someone", "a human question"),
				appComment("example-desk-app", "desk: relayed to the driver."),
			},
		},
	}

	fx := flowFixtures()[0]
	dir := t.TempDir()
	rawFile := filepath.Join(dir, "raw.json")
	if err := os.WriteFile(rawFile, []byte(fx.raw), 0o644); err != nil {
		t.Fatalf("write raw fixture: %v", err)
	}
	modelFile := filepath.Join(dir, "model.json")
	if err := os.WriteFile(modelFile, runCmd(t, "jq", "-f", writeProgFile(t, dir, "flow.jq", flowProgram), rawFile), 0o644); err != nil {
		t.Fatalf("write model file: %v", err)
	}
	var doc rawDoc
	if err := json.Unmarshal([]byte(fx.raw), &doc); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	model := interpretFlow(doc)

	// Oracle side: gh-shaped detail → JQFMT (build_item) → the items array JQHTML reads.
	var oracleItems []jqHTMLItem
	for k, w := range queue {
		out := runOracleFormatRaw(t, fmtProgram, w.it, oracleDetail(w.body, w.comments), k, len(queue))
		var hi jqHTMLItem
		if err := json.Unmarshal(out, &hi); err != nil {
			t.Fatalf("decode JQFMT output: %v\nraw: %s", err, out)
		}
		// deskinbox runs no screen (testdata/spec.md): its page is the oracle's unclassed
		// (`--no-screen`) shape, so the screen's annotation is dropped from the oracle item.
		hi.Class, hi.ClassEvidence = "", ""
		oracleItems = append(oracleItems, hi)
	}
	itemsBytes, _ := json.Marshal(oracleItems)
	itemsFile := filepath.Join(dir, "items.json")
	if err := os.WriteFile(itemsFile, itemsBytes, 0o644); err != nil {
		t.Fatalf("write items file: %v", err)
	}
	summary := "deskinbox: 2 item(s) across 1 repo(s)"
	want := runOracleHTML(t, htmlProgram, summary, false, modelFile, itemsFile)

	// Go side: REST-shaped detail through the real reader → buildRendered → the page.
	var cards []cardData
	for k, w := range queue {
		d := readDetailOverREST(t, repo, w.it.Number, w.body, w.comments)
		cards = append(cards, cardData{
			Item: toGoItem(w.it), Index: k + 1, Total: len(queue),
			Rendered: buildRendered(toGoItem(w.it), d, k, len(queue), now),
		})
	}
	got := buildDecisionPage(summary, cards, model)

	if got != string(want) {
		t.Errorf("decision page built from the REST reader diverges from the oracle's (gh wire shape)\n got:\n%s\n\nwant:\n%s", got, string(want))
	}
}

// runOracleFormatRaw is runOracleFormat returning the program's raw JSON object, so a caller
// can keep the fields JQFMT emits beyond the five-part subset (repo/number/url/title/index/
// total), exactly as build_item appends them to $TMP_ITEMS.
func runOracleFormatRaw(t *testing.T, program string, it jqItem, d jqDetail, k, n int) []byte {
	t.Helper()
	dir := t.TempDir()
	itemBytes, _ := json.Marshal(it)
	detailBytes, _ := json.Marshal(d)
	return runCmd(t, "jq", "-s",
		"--argjson", "k", strconv.Itoa(k), "--argjson", "n", strconv.Itoa(n),
		"-f", writeProgFile(t, dir, "fmt.jq", program),
		writeProgFile(t, dir, "item.json", string(itemBytes)),
		writeProgFile(t, dir, "detail.json", string(detailBytes)))
}
