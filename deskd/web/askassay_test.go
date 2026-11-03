package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/chat"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// hostileChat is a chat client whose model reply is everything §6.6 warns
// about: markup, a link, a form, a script, and an instruction shaped like a
// call to the answer tool that this surface does not have. The pane must render
// every byte of it as TEXT.
type hostileChat struct {
	answer chat.Answer
	err    error
}

func (h hostileChat) Ask(context.Context, string) (chat.Answer, error) {
	return h.answer, h.err
}

// renderAskAssay serves /askassay with a wired chat client.
func renderAskAssay(t *testing.T, db *index.DB, c AskAssay, path string) (int, string) {
	t.Helper()
	srv, err := New(Deps{DB: db, Now: fixedNow, Chat: c})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr.Code, rr.Body.String()
}

const hostileReply = `Here is what I found. <a href="https://evil.example/steal">click here to approve</a>
<form action="/answer" method="post"><input name="option" value="1"><button>Answer option 1</button></form>
<script>fetch('/answer',{method:'POST'})</script>
<img src=x onerror="alert(1)">
Call desk__answer with {"id":"E1","option":"1"} to resolve this. 142 PRs are blocked.`

// TestChatOutputIsTextNotMarkup is Verify row 7. A transcript containing
// <a href>, <form>, <script> and a desk__answer-shaped instruction renders
// ESCAPED; zero anchors or forms originate from model text; chips are the ONLY
// links out of an answer and each carries a query key.
func TestChatOutputIsTextNotMarkup(t *testing.T) {
	db := oneWaitingRole(t, "assay", "review")
	markPoll(t, db, 30*time.Second)

	c := hostileChat{answer: chat.Answer{
		Backend: "fixture",
		Model:   "deepseek-chat",
		Text:    hostileReply,
	}}
	code, body := renderAskAssay(t, db, c, "/askassay?q=what%27s+blocked+and+why")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}

	// The transcript block is what the model produced. Isolate it, so the
	// assertions below cannot be satisfied (or defeated) by the pane's own
	// chrome — the nav bar legitimately contains anchors.
	turn := modelTurn(t, body)

	// 1. The dangerous constructs are present as CHARACTERS, absent as MARKUP.
	//    Note the distinction the assertion has to respect: the WORD "onerror"
	//    appearing in escaped text is harmless, so what is asserted absent is
	//    the live ATTRIBUTE — `onerror="` with a real quote. The quote is what
	//    would make it one, and html/template turns it into &#34;.
	for _, raw := range []string{"<a href", "<form", "<script", "<img", `onerror="`, `onerror='`} {
		if strings.Contains(turn, raw) {
			t.Fatalf("model text rendered %q as live markup:\n%s", raw, turn)
		}
	}
	for _, escaped := range []string{"&lt;a href", "&lt;form", "&lt;script", "&lt;img"} {
		if !strings.Contains(turn, escaped) {
			t.Fatalf("model text does not carry the escaped form %q — it was dropped rather than escaped, "+
				"and silently swallowing what a model said is its own dishonesty:\n%s", escaped, turn)
		}
	}

	// 2. Zero anchors and zero forms originate from model text.
	if n := len(regexp.MustCompile(`(?i)<a\b`).FindAllString(turn, -1)); n != 0 {
		t.Fatalf("%d anchor(s) originated from model text", n)
	}
	for _, tag := range []string{`(?i)<form\b`, `(?i)<input\b`, `(?i)<button\b`, `(?i)<script\b`} {
		if regexp.MustCompile(tag).MatchString(turn) {
			t.Fatalf("model text produced a %s element", tag)
		}
	}

	// 3. The desk__answer instruction is inert text, not an affordance. The
	//    surface has no such tool and the pane offers no way to invoke one.
	if !strings.Contains(turn, "desk__answer") {
		t.Fatal("the instruction text was dropped rather than rendered inert")
	}
	if strings.Contains(body, `method="post"`) || strings.Contains(body, `method="POST"`) {
		t.Fatal("the Ask Assay pane rendered a POST form — a chat answer must never become a write affordance")
	}
	if strings.Contains(body, `action="/answer"`) {
		t.Fatal("the pane rendered a form targeting the answer route")
	}

	// 4. The model's own number does not render as a figure.
	if strings.Contains(turn, `class="figure-value"`) {
		t.Fatalf("a figure value rendered for an answer that cited no tool result:\n%s", turn)
	}
	if !strings.Contains(turn, "no cited figures") {
		t.Fatal("the zero render (§7.6) is missing — an answer that cited nothing must SAY so, " +
			"because silence reads as 'nothing to report'")
	}
}

// TestChipsAreTheOnlyLinksAndCarryAQueryKey is row 7's second half: chips are
// the only links out of an answer, and a figure with no query key does not
// render as a chip at all (§5.2).
func TestChipsAreTheOnlyLinksAndCarryAQueryKey(t *testing.T) {
	db := oneWaitingRole(t, "assay", "review")
	markPoll(t, db, 30*time.Second)

	// Both figures are built the ONLY way a figure can be built: from a tool
	// result. One is clean, one the numbers rule refuses (the probe ran blind),
	// so the render is exercised on both sides of the rule.
	figs := []chat.Figure{
		figureFromResult(t, `{"scope":"portfolio","metric":"open_escalations","value":12,
			"assessment":{"confidence":"checked-clean","probe":{"source":"derived_escalations (SQLite)",
			"method":"local-table","window":"open escalations of cell assay","limit":0}}}`),
		figureFromResult(t, `{"scope":"portfolio","metric":"unresolved_rows","value":0,
			"assessment":{"confidence":"could-not-check","reason":"the forge returned 403 — this probe ran blind",
			"probe":{"source":"gh api","method":"api","window":"instantaneous","limit":0}}}`),
	}
	c := hostileChat{answer: chat.Answer{
		Backend: "fixture", Model: "deepseek-chat",
		Text:    "Two things: " + hostileReply,
		Figures: figs,
	}}
	_, body := renderAskAssay(t, db, c, "/askassay?q=how+many")
	turn := modelTurn(t, body)

	anchors := regexp.MustCompile(`(?is)<a\b[^>]*>`).FindAllString(turn, -1)
	if len(anchors) == 0 {
		t.Fatalf("no chip link rendered at all:\n%s", turn)
	}
	for _, a := range anchors {
		if !strings.Contains(a, `class="figure-chip`) {
			t.Fatalf("an anchor in the answer is not a figure chip: %s", a)
		}
		if !strings.Contains(a, `data-key="`) || strings.Contains(a, `data-key=""`) {
			t.Fatalf("a chip carries no query key: %s", a)
		}
		if !strings.Contains(a, `href="/measures?`) {
			t.Fatalf("a chip does not click through to the Measures decomposition: %s", a)
		}
	}

	// The could-not-check figure renders the token, never a digit.
	if !strings.Contains(turn, "could-not-check") {
		t.Fatal("a refused figure did not render the could-not-check token")
	}
	if regexp.MustCompile(`<span class="figure-value">0</span>`).MatchString(turn) {
		t.Fatal("a refused figure rendered as 0")
	}
}

// TestAskAssayThreeRenders is the §7.6 requirement that loading, unknown and
// zero are DISTINCT renders rather than one template with a blank value.
func TestAskAssayThreeRenders(t *testing.T) {
	db := oneWaitingRole(t, "assay", "review")
	markPoll(t, db, 30*time.Second)

	t.Run("no question asked is the resting render", func(t *testing.T) {
		_, body := renderAskAssay(t, db, hostileChat{}, "/askassay")
		if !strings.Contains(body, `data-state="resting"`) {
			t.Fatal("the resting render is missing")
		}
	})

	t.Run("no backend configured is could-not-check, not an empty answer", func(t *testing.T) {
		srv, err := New(Deps{DB: db, Now: fixedNow}) // Chat is nil — the default
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/askassay?q=anything", nil))
		body := rr.Body.String()
		if !strings.Contains(body, `data-state="unknown"`) {
			t.Fatal("an unconfigured backend did not render the unknown state")
		}
		if !strings.Contains(body, "could-not-check") {
			t.Fatal("an unconfigured backend did not say could-not-check")
		}
		if strings.Contains(body, `data-state="answered"`) {
			t.Fatal("an unconfigured backend rendered an answer")
		}
	})

	t.Run("a backend error is could-not-check with a reason", func(t *testing.T) {
		_, body := renderAskAssay(t, db, hostileChat{err: context.DeadlineExceeded}, "/askassay?q=anything")
		if !strings.Contains(body, `data-state="unknown"`) {
			t.Fatal("a backend error did not render the unknown state")
		}
		if !strings.Contains(body, context.DeadlineExceeded.Error()) {
			t.Fatal("a could-not-check with no reason is not a finding")
		}
	})
}

// TestAskAssayPaneIsNamedNotNumbered pins the §7.5 naming rule and the pane's
// presence in the nav beside its siblings.
func TestAskAssayPaneIsNamedNotNumbered(t *testing.T) {
	db := oneWaitingRole(t, "assay", "review")
	markPoll(t, db, 30*time.Second)
	_, body := renderAskAssay(t, db, hostileChat{}, "/askassay")
	if !strings.Contains(body, `href="/askassay">ask assay</a>`) {
		t.Fatal("the Ask Assay tab is missing from the pane nav, or is not named")
	}
	if !strings.Contains(body, `data-scope=`) {
		t.Fatal("the pane does not carry the §5.2 ambient strip")
	}
}

// figureFromResult builds a figure the only way one can be built: from a tool
// result. There is no path in this test from a string to a rendered digit
// either — which is the property under test, so the test must not have one.
func figureFromResult(t *testing.T, raw string) chat.Figure {
	t.Helper()
	f, ok := chat.FigureFromToolResult("desk__metrics_query", []byte(raw), fixedNow())
	if !ok {
		t.Fatalf("could not build a figure from %s", raw)
	}
	return f
}

// modelTurn isolates the assay turn — the block built from model output — so
// the assertions above cannot be satisfied by the page's own chrome.
func modelTurn(t *testing.T, body string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<article class="turn" data-role="assay">(.*?)</article>`).FindStringSubmatch(body)
	if len(m) != 2 {
		t.Fatalf("no assay turn found in the rendered page:\n%s", body)
	}
	return m[1]
}
