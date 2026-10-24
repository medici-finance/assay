package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/askassay"
)

// ---- fixtures ---------------------------------------------------------------

var fixedNow = time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

// fixtureTools is a ToolCaller whose results are canned, so a test can state
// exactly what the index "returned" and watch what the numbers rule does with it.
type fixtureTools struct {
	results map[string]string
	calls   []string
}

func (f *fixtureTools) Tools() []ToolDescriptor { return fullRegistry() }

func (f *fixtureTools) CallTool(_ context.Context, name string, _ []byte) ([]byte, error) {
	f.calls = append(f.calls, name)
	raw, ok := f.results[name]
	if !ok {
		return []byte(`{}`), nil
	}
	return []byte(raw), nil
}

// fixtureBackend answers with a scripted reply per turn.
type fixtureBackend struct {
	name    string
	replies []Reply
	models  []string // the model each Complete was called with, in order
	turn    int
}

func (b *fixtureBackend) Name() string { return b.name }

func (b *fixtureBackend) Complete(_ context.Context, model string, _ Prompt) (Reply, error) {
	b.models = append(b.models, model)
	if b.turn >= len(b.replies) {
		return Reply{Text: "(no further reply)"}, nil
	}
	r := b.replies[b.turn]
	b.turn++
	return r, nil
}

// cleanMetricsResult is a desk__metrics_query result the way internal/mcp
// builds one: a value plus a fully declared probe and a checked-clean
// assessment.
func cleanMetricsResult(metric string, value int64, limit int64) string {
	m := map[string]any{
		"scope":  "portfolio",
		"metric": metric,
		"value":  value,
		"assessment": map[string]any{
			"confidence": "checked-clean",
			"probe": map[string]any{
				"source": "derived_escalations (SQLite) via desk__metrics_query — the index only",
				"method": "local-table",
				"window": "open escalations of cell assay",
				"limit":  limit,
			},
		},
	}
	raw, _ := json.Marshal(m)
	return string(raw)
}

func newFixtureClient(t *testing.T, tools *fixtureTools, be *fixtureBackend) *Client {
	t.Helper()
	c, err := New(Options{
		Tools:   tools,
		Backend: be,
		Models:  []string{"deepseek-chat", "glm-5.2"},
		Now:     func() time.Time { return fixedNow },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// ---- Verify row 4 -----------------------------------------------------------

// TestModelStatedNumberNeverRenders is Verify row 4. A backend answers
// "142 PRs are blocked" with NO tool call. The transcript must carry the text
// — suppressing what the model said would be its own kind of dishonesty — and
// figures[] must be empty, so the pane has no chip and no digit to render.
func TestModelStatedNumberNeverRenders(t *testing.T) {
	tools := &fixtureTools{}
	be := &fixtureBackend{name: "fixture", replies: []Reply{{
		Text: "142 PRs are blocked, and 17 of them have been blocked for over a week.",
	}}}
	ans, err := newFixtureClient(t, tools, be).Ask(context.Background(), "what's blocked and why")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}

	if !strings.Contains(ans.Text, "142") {
		t.Fatalf("the transcript dropped the model's own words: %q", ans.Text)
	}
	if len(ans.Figures) != 0 {
		t.Fatalf("a model-stated number produced %d figure(s) — there is a path from model text to a rendered digit",
			len(ans.Figures))
	}
	if len(ans.ToolCalls) != 0 {
		t.Fatalf("no tool was requested, but %d call(s) were recorded", len(ans.ToolCalls))
	}
	if len(tools.calls) != 0 {
		t.Fatalf("no tool was requested, but the tool surface was called: %v", tools.calls)
	}
}

// TestModelCannotCallAnUnboundTool proves the dispatch-time re-check: a reply
// naming a write tool is RECORDED as refused and never dispatched.
func TestModelCannotCallAnUnboundTool(t *testing.T) {
	tools := &fixtureTools{}
	be := &fixtureBackend{name: "fixture", replies: []Reply{{
		Text: "I will file an escalation about this.",
		ToolCalls: []RequestedCall{
			{Tool: "desk__escalate", Args: []byte(`{"cell":"assay","kind":"ask","title":"x","situation":"y"}`)},
			{Tool: "desk__answer", Args: []byte(`{}`)},
		},
	}}}
	ans, err := newFixtureClient(t, tools, be).Ask(context.Background(), "file something for me")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if len(tools.calls) != 0 {
		t.Fatalf("an unbound tool reached the tool surface: %v", tools.calls)
	}
	if len(ans.ToolCalls) != 2 {
		t.Fatalf("want 2 recorded calls, got %d", len(ans.ToolCalls))
	}
	for _, c := range ans.ToolCalls {
		if c.OK || c.Refused == "" {
			t.Fatalf("%s was not recorded as refused: %+v", c.Tool, c)
		}
	}
	if len(ans.Figures) != 0 {
		t.Fatalf("a refused call produced %d figure(s)", len(ans.Figures))
	}
}

// ---- Verify row 5 -----------------------------------------------------------

// TestFigureComesFromToolResultOnly is Verify row 5. A desk__metrics_query
// result builds an askassay.Answer rendering its value plus query key; a
// saturated / empty / throttled result renders could-not-check. The chat figure design
// rules hold UNCHANGED through the MCP source — every downgrade below is the
// imported package's decision, not a local one.
func TestFigureComesFromToolResultOnly(t *testing.T) {
	t.Run("a clean tool result renders its value and its query key", func(t *testing.T) {
		tools := &fixtureTools{results: map[string]string{
			"desk__metrics_query": cleanMetricsResult("open_escalations", 12, 0),
		}}
		be := &fixtureBackend{name: "fixture", replies: []Reply{{
			Text:      "Twelve items are open.",
			ToolCalls: []RequestedCall{{Tool: "desk__metrics_query", Args: []byte(`{"scope":"portfolio"}`)}},
		}}}
		ans, err := newFixtureClient(t, tools, be).Ask(context.Background(), "how many are open")
		if err != nil {
			t.Fatalf("Ask: %v", err)
		}
		if len(ans.Figures) != 1 {
			t.Fatalf("want 1 figure, got %d", len(ans.Figures))
		}
		f := ans.Figures[0]
		if f.State() != askassay.Checked {
			t.Fatalf("state is %s, want checked: %s", f.State(), f.Render())
		}
		v, ok := f.Value()
		if !ok || v != 12 {
			t.Fatalf("value is (%d, %v), want (12, true)", v, ok)
		}
		if f.QueryKey() != "open_escalations" {
			t.Fatalf("query key is %q, want open_escalations", f.QueryKey())
		}
		if href := f.MeasuresHref(); !strings.Contains(href, "key=open_escalations") {
			t.Fatalf("click-through %q does not carry the query key", href)
		}
	})

	// Each of the three downgrades the row names, driven by the tool's OWN
	// declaration rather than by anything this package decides.
	t.Run("a saturated result renders could-not-check", func(t *testing.T) {
		// value == the declared cap: a full page is the same bytes as a
		// truncated one, so askassay.Computed refuses it.
		f := figureOrFail(t, cleanMetricsResult("open_escalations", 500, 500))
		assertCouldNotCheck(t, f, "cap")
	})

	t.Run("a throttled result renders could-not-check", func(t *testing.T) {
		f := figureOrFail(t, `{"scope":"portfolio","metric":"open_escalations","value":0,
			"assessment":{"confidence":"could-not-check","reason":"the forge returned 403 (secondary rate limit) — this probe ran blind",
			"probe":{"source":"gh api","method":"api","window":"instantaneous","limit":0}}}`)
		assertCouldNotCheck(t, f, "403")
	})

	t.Run("an empty-because-blind result renders could-not-check, never 0", func(t *testing.T) {
		f := figureOrFail(t, `{"scope":"portfolio","metric":"open_escalations","value":0,
			"assessment":{"confidence":"could-not-check","reason":"no repo in the cell could be read this cycle",
			"probe":{"source":"gh api","method":"api","window":"instantaneous","limit":0}}}`)
		if _, ok := f.Value(); ok {
			t.Fatal("a blind probe produced a value — could-not-check decayed into a zero")
		}
		if !strings.Contains(f.Render(), askassay.FigureField) {
			t.Fatalf("render does not carry the could-not-check token: %s", f.Render())
		}
		if strings.Contains(f.Render(), "figure=0") {
			t.Fatalf("a blind probe rendered a zero: %s", f.Render())
		}
	})

	t.Run("an under-declared source renders could-not-check", func(t *testing.T) {
		// The tool reported checked-clean but declared no window. The IMPORTED
		// rule refuses it — this package writes no such check of its own.
		f := figureOrFail(t, `{"scope":"portfolio","metric":"open_escalations","value":7,
			"assessment":{"confidence":"checked-clean",
			"probe":{"source":"derived_escalations","method":"local-table","window":"","limit":0}}}`)
		assertCouldNotCheck(t, f, "window")
	})

	t.Run("a breakdown is not a figure", func(t *testing.T) {
		f := figureOrFail(t, `{"scope":"portfolio","metric":"by_urgency","value":{"routine":3,"urgent":1},
			"assessment":{"confidence":"checked-clean",
			"probe":{"source":"derived_escalations","method":"local-table","window":"open escalations","limit":0}}}`)
		assertCouldNotCheck(t, f, "breakdown")
	})

	t.Run("a non-metrics read yields no figure at all", func(t *testing.T) {
		if _, ok := FigureFromToolResult("desk__inbox_list", []byte(`{"count":9,"items":[]}`), fixedNow); ok {
			t.Fatal("a list read produced a figure — a count field on an arbitrary read must not become a cited number")
		}
	})
}

func figureOrFail(t *testing.T, raw string) Figure {
	t.Helper()
	f, ok := FigureFromToolResult("desk__metrics_query", []byte(raw), fixedNow)
	if !ok {
		t.Fatalf("FigureFromToolResult declined to build a figure from %s", raw)
	}
	return f
}

func assertCouldNotCheck(t *testing.T, f Figure, wantInReason string) {
	t.Helper()
	if f.State() != askassay.CouldNotCheck {
		t.Fatalf("state is %s, want could-not-check: %s", f.State(), f.Render())
	}
	if _, ok := f.Value(); ok {
		t.Fatal("a could-not-check figure carries a value")
	}
	if !strings.Contains(f.Render(), askassay.FigureField) {
		t.Fatalf("render does not carry %q: %s", askassay.FigureField, f.Render())
	}
	if wantInReason != "" && !strings.Contains(f.Reason(), wantInReason) {
		t.Fatalf("reason %q does not mention %q", f.Reason(), wantInReason)
	}
}
