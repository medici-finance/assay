package main

// format_parity_test.go — TestParityWalk: asserts buildRendered (format.go) byte-for-byte
// against the REAL jq program the oracle ships (write_format_program, assay-inbox.sh:425-540),
// extracted verbatim at test time and run through the system `jq` binary. This is a
// direct parity check on the FORMAT BUILDER itself — the piece both `--walk` and `--html`
// share in the oracle, and the one this brief's split keeps byte-identical. It needs no
// network and no bash 3.2 environment to run.
//
// It is NOT independent of the wire format, and must not be (#1797): the two tools read
// comments over different wires — the oracle over `gh issue view --json comments`, where an
// App's author.login is the bare `<slug>`; deskinbox over REST, where the same App's
// user.login is `<slug>[bot]`. A fixture handing both sides one login string proved the
// builder equal while the two readers disagreed upstream of it. So the Go side of every
// readable case reads its detail through the REAL fetchDetail reader, served in the REST
// shape by an httptest backend (wireshape_test.go), while the oracle is fed the gh shape of
// the same comments.
//
// Needs `jq` on the runner (the oracle's own hard dependency). Its absence is
// could-not-check, per the three-state instrument rule — never a silent skip read as a
// pass.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const oracleScriptRelPath = "../../../../plugins/assay/scripts/assay-inbox.sh"

// extractJQFMTProgram pulls the write_format_program() heredoc body verbatim out of the
// oracle script — the same bytes `write_format_program` writes to $TMP_FMT at runtime.
func extractJQFMTProgram(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(oracleScriptRelPath)
	if err != nil {
		t.Skipf("could-not-check: cannot read the oracle script at %s: %v", oracleScriptRelPath, err)
	}
	lines := strings.Split(string(raw), "\n")
	start, end := -1, -1
	for i, l := range lines {
		if strings.Contains(l, "cat > \"$TMP_FMT\" <<'JQFMT'") {
			start = i + 1
			continue
		}
		if start != -1 && l == "JQFMT" {
			end = i
			break
		}
	}
	if start == -1 || end == -1 {
		t.Fatalf("could not locate the write_format_program JQFMT heredoc in %s (start=%d end=%d) — "+
			"the oracle's own contract may have moved; re-locate the markers before trusting this test",
			oracleScriptRelPath, start, end)
	}
	return strings.Join(lines[start:end], "\n")
}

func requireJQ(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("could-not-check: `jq` not found on PATH — the oracle's own hard dependency; cannot run the parity check")
	}
}

// jqItem, jqLabel, jqDetail and jqComment mirror the exact JSON shapes the oracle's
// $TMP_SORTED entries and `gh issue view --json body,comments` produce.
type jqLabel struct {
	Name string `json:"name"`
}
type jqItem struct {
	Repo      string    `json:"repo"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	CreatedAt string    `json:"createdAt"`
	Labels    []jqLabel `json:"labels"`
}
type jqCommentAuthor struct {
	Login string `json:"login"`
}
type jqComment struct {
	Author jqCommentAuthor `json:"author"`
	Body   string          `json:"body"`
}
type jqDetail struct {
	Body              string      `json:"body,omitempty"`
	Comments          []jqComment `json:"comments,omitempty"`
	DetailUnavailable bool        `json:"detailUnavailable,omitempty"`
}

// jqRenderedOut is the subset of the oracle's per-item output object this test compares —
// the five-part fields render_walk actually reads (header/context/options/reply/
// verification/unread), i.e. everything this brief's rendered struct also carries.
type jqRenderedOut struct {
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
}

// runOracleFormat runs the extracted jq program on one (item, detail, k, n) input, exactly
// as build_item() invokes it (`jq -s --argjson k … --argjson n … -f prog itemfile
// detailfile`), and decodes the result.
func runOracleFormat(t *testing.T, program string, it jqItem, d jqDetail, k, n int) jqRenderedOut {
	t.Helper()
	dir := t.TempDir()
	progFile := filepath.Join(dir, "fmt.jq")
	if err := os.WriteFile(progFile, []byte(program), 0o644); err != nil {
		t.Fatalf("write program: %v", err)
	}
	itemFile := filepath.Join(dir, "item.json")
	itemBytes, _ := json.Marshal(it)
	if err := os.WriteFile(itemFile, itemBytes, 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
	detailFile := filepath.Join(dir, "detail.json")
	detailBytes, _ := json.Marshal(d)
	if err := os.WriteFile(detailFile, detailBytes, 0o644); err != nil {
		t.Fatalf("write detail: %v", err)
	}

	cmd := exec.Command("jq", "-s",
		"--argjson", "k", strconv.Itoa(k),
		"--argjson", "n", strconv.Itoa(n),
		"-f", progFile, itemFile, detailFile)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("jq failed: %v\nstderr: %s", err, errb.String())
	}
	var res jqRenderedOut
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("cannot decode jq output: %v\nraw: %s", err, out.String())
	}
	return res
}

// toGoItem converts the jq-shaped item fixture to this port's own type, so both sides of the
// comparison are built from literally the SAME item data. The DETAIL is not converted: each
// side receives it in its own reader's wire shape (see the file header).
func toGoItem(it jqItem) item {
	var labels []string
	for _, l := range it.Labels {
		labels = append(labels, l.Name)
	}
	return item{
		Repo: it.Repo, Number: it.Number, Title: it.Title, URL: it.URL,
		CreatedAt: it.CreatedAt, Labels: labels, Rank: rankOf(labels),
	}
}

// assertRenderedMatchesOracle compares the oracle's rendering of oracleD (gh wire shape)
// against the port's rendering of goD (whatever deskinbox's own reader produced).
func assertRenderedMatchesOracle(t *testing.T, name string, it jqItem, oracleD jqDetail, goD issueDetail, k, n int, now time.Time) {
	t.Helper()
	program := extractJQFMTProgram(t)
	want := runOracleFormat(t, program, it, oracleD, k, n)
	got := buildRendered(toGoItem(it), goD, k, n, now)

	if got.Header != want.Header {
		t.Errorf("%s: header mismatch\n got:  %q\n want: %q", name, got.Header, want.Header)
	}
	if len(got.Context) != len(want.Context) {
		t.Errorf("%s: context length mismatch\n got:  %#v\n want: %#v", name, got.Context, want.Context)
	} else {
		for i := range got.Context {
			if got.Context[i] != want.Context[i] {
				t.Errorf("%s: context[%d] mismatch\n got:  %q\n want: %q", name, i, got.Context[i], want.Context[i])
			}
		}
	}
	if len(got.Options) != len(want.Options) {
		t.Errorf("%s: options length mismatch\n got:  %#v\n want: %#v", name, got.Options, want.Options)
	} else {
		for i := range got.Options {
			g, w := got.Options[i], want.Options[i]
			if g.Letter != w.Letter || g.Text != w.Text || g.Recommended != w.Recommended {
				t.Errorf("%s: options[%d] mismatch\n got:  %+v\n want: %+v", name, i, g, w)
			}
		}
	}
	if got.OptionsStated != want.OptionsStated {
		t.Errorf("%s: optionsStated mismatch: got %v want %v", name, got.OptionsStated, want.OptionsStated)
	}
	if got.Reply != want.Reply {
		t.Errorf("%s: reply mismatch\n got:  %q\n want: %q", name, got.Reply, want.Reply)
	}
	if got.Verification != want.Verification {
		t.Errorf("%s: verification mismatch\n got:  %q\n want: %q", name, got.Verification, want.Verification)
	}
	if got.Unread != want.Unread {
		t.Errorf("%s: unread mismatch: got %v want %v", name, got.Unread, want.Unread)
	}
}

func TestParityWalk(t *testing.T) {
	requireJQ(t)
	// The oracle's own age math runs off jq's built-in `now` (wall-clock at execution),
	// which this test cannot pin — so it reads the real clock on BOTH sides, as close
	// together as the two invocations allow, rather than assert a fixed age and risk a
	// day-boundary flake independent of anything this port got wrong.
	now := time.Now()
	createdAt := now.Add(-30 * 24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")

	cases := []struct {
		name     string
		it       jqItem
		body     string
		comments []wireComment
		blind    bool
		k, n     int
	}{
		{
			name: "typical with context, options, recommended, unblocks, desk-note",
			it: jqItem{
				Repo: "example-org/example-repo", Number: 42, Title: "Example decision",
				URL: "https://example.invalid/issues/42", CreatedAt: createdAt,
				Labels: []jqLabel{{Name: "needs-decision"}, {Name: "urgent"}},
			},
			body: "Some preamble.\n\n## Context\n\nThe queue is stuck on X.\nY needs a ruling.\n\n" +
				"unblocks: example-stream/14\n\n## Options\n\n" +
				"A. Do the safe thing\nB. Do the risky thing — (Recommended)\nC. Do nothing\n",
			comments: []wireComment{
				userComment("someone", "not the desk"),
				appComment("example-worker-app", "latest status: waiting on a ruling.\nsecond line here."),
			},
			k: 0, n: 3,
		},
		{
			// #1797's selection divergence: a desk App's note followed by a comment from an
			// App whose slug does not read "desk". The oracle (gh shape, no `[bot]`) quotes the
			// desk App's note; a reader that kept REST's `[bot]` would quote the later App.
			name: "desk-App note precedes a non-desk App comment (both wire shapes)",
			it: jqItem{
				Repo: "example-org/example-repo", Number: 43, Title: "Selection across wire shapes",
				URL: "https://example.invalid/issues/43", CreatedAt: createdAt,
				Labels: []jqLabel{{Name: "needs-decision"}},
			},
			body: "## Context\nA ruling is needed on Z.\n\n## Options\nA. Keep Z\nB. Drop Z\n",
			comments: []wireComment{
				userComment("someone", "a human question"),
				appComment("example-desk-app", "desk relay: options restated, awaiting the driver."),
				appComment("example-worker-app", "worker: pushed a fix to the branch."),
				userComment("someone-else", "a later human reply"),
			},
			k: 1, n: 3,
		},
		{
			// #1797's label divergence: the desk App's own note is the latest — both tools
			// pick it, and its author must render in the oracle's (gh) spelling.
			name: "desk-App note is the latest, label in the gh login shape",
			it: jqItem{
				Repo: "example-org/example-repo", Number: 44, Title: "Label across wire shapes",
				URL: "https://example.invalid/issues/44", CreatedAt: createdAt,
				Labels: []jqLabel{{Name: "question"}},
			},
			body: "## Context\nWhich way on W?\n",
			comments: []wireComment{
				appComment("example-worker-app", "worker status"),
				appComment("example-desk-app", "desk: relayed to the driver."),
			},
			k: 2, n: 3,
		},
		{
			name: "no headings falls back to leading body lines, no options stated",
			it: jqItem{
				Repo: "example-org/example-repo", Number: 7, Title: "Plain issue",
				URL: "https://example.invalid/issues/7", CreatedAt: createdAt,
				Labels: []jqLabel{{Name: "question"}},
			},
			body: "This is just a plain paragraph with no headings at all.\nA second line of prose.\n",
			k:    1, n: 3,
		},
		{
			name: "blind detail fetch",
			it: jqItem{
				Repo: "example-org/example-repo", Number: 9, Title: "Unreadable",
				URL: "https://example.invalid/issues/9", CreatedAt: createdAt,
				Labels: []jqLabel{{Name: "help wanted"}},
			},
			blind: true,
			k:     2, n: 3,
		},
		{
			name: "recommended option not first gets reordered and reletterred",
			it: jqItem{
				Repo: "example-org/example-repo", Number: 11, Title: "Reorder",
				URL: "https://example.invalid/issues/11", CreatedAt: createdAt,
				Labels: []jqLabel{{Name: "needs-decision"}},
			},
			body: "## Options\nA. First choice\nB. Second choice\nC. Third choice, this one is recommended\nD. Fourth choice\n",
			k:    0, n: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.blind {
				assertRenderedMatchesOracle(t, tc.name, tc.it, jqDetail{DetailUnavailable: true},
					issueDetail{Unavailable: true}, tc.k, tc.n, now)
				return
			}
			repo := deskkit.ForgeRepo{Owner: "example-org", Name: "example-repo"}
			goD := readDetailOverREST(t, repo, tc.it.Number, tc.body, tc.comments)
			assertRenderedMatchesOracle(t, tc.name, tc.it, oracleDetail(tc.body, tc.comments), goD, tc.k, tc.n, now)
		})
	}
}
