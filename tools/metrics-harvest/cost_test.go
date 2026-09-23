package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// zeroFigure matches a rendered zero in a figure position (": 0 " / ": 0.00 ").
var zeroFigure = regexp.MustCompile(`: 0(\.0+)?( |$)`)

// lineFor returns the one rendered line naming desk, or "".
func lineFor(section, desk string) string {
	for _, l := range strings.Split(section, "\n") {
		if strings.Contains(l, "desk `"+desk+"`") {
			return l
		}
	}
	return ""
}

// TestCostPerRowCouldNotCheck: a desk that reported no cost telemetry — the key
// absent, JSON null, or the "could-not-check" string the session vitals write —
// renders could-not-check, never 0; a measured desk renders its per-row figure
// with its unit; zero passed rows is could-not-check, not a division by zero.
func TestCostPerRowCouldNotCheck(t *testing.T) {
	in := []deskCostInput{
		{Desk: "worker-desk", Model: "example-model", Cost: json.RawMessage(`120000`), Unit: "tokens", PassedRows: json.RawMessage(`4`)},
		{Desk: "absent-desk", Model: "example-model", Unit: "tokens", PassedRows: json.RawMessage(`3`)},
		{Desk: "null-desk", Model: "example-model", Cost: json.RawMessage(`null`), Unit: "tokens", PassedRows: json.RawMessage(`3`)},
		{Desk: "blind-desk", Model: "example-model", Cost: json.RawMessage(`"could-not-check"`), Unit: "tokens", PassedRows: json.RawMessage(`3`)},
		{Desk: "unitless-desk", Model: "example-model", Cost: json.RawMessage(`500`), PassedRows: json.RawMessage(`3`)},
		{Desk: "idle-desk", Model: "example-model", Cost: json.RawMessage(`9000`), Unit: "tokens", PassedRows: json.RawMessage(`0`)},
		{Desk: "neg-desk", Model: "example-model", Cost: json.RawMessage(`-50`), Unit: "tokens", PassedRows: json.RawMessage(`2`)},
	}
	lines := costPerPassedRow(in)
	byDesk := map[string]costLine{}
	for _, l := range lines {
		byDesk[l.Desk] = l
	}
	section := renderCostSection(lines)

	for _, desk := range []string{"absent-desk", "null-desk", "blind-desk", "unitless-desk", "idle-desk", "neg-desk"} {
		l := byDesk[desk]
		if l.State != stateCouldNotCheck || l.PerRow != nil {
			t.Errorf("%s: want state %s with no per-row value, got state=%q perRow=%v", desk, stateCouldNotCheck, l.State, l.PerRow)
		}
		r := lineFor(section, desk)
		if !strings.Contains(r, "cost per passed row") || !strings.Contains(r, stateCouldNotCheck) {
			t.Errorf("%s: rendered line must name the figure and say %s; got %q", desk, stateCouldNotCheck, r)
		}
		if zeroFigure.MatchString(r) {
			t.Errorf("%s: an unmeasured cost must never render as 0; got %q", desk, r)
		}
	}

	w := byDesk["worker-desk"]
	if w.State != stateMeasured || w.PerRow == nil || *w.PerRow != 30000 {
		t.Fatalf("worker-desk: want measured 30000 per row, got state=%q perRow=%v", w.State, w.PerRow)
	}
	if r := lineFor(section, "worker-desk"); !strings.Contains(r, "cost per passed row") || !strings.Contains(r, "30000.00 tokens") {
		t.Errorf("worker-desk: measured line must carry the figure with its unit; got %q", r)
	}
}

// TestCostSubcommandExitCodes: the subcommand exits 3 (published, with a gap)
// when any desk is could-not-check, 0 when all are measured, 2 on a refused
// input (wrong schema, duplicate desk/model), and 3 when no desk was supplied
// at all — an empty join is a gap, not a fully measured run.
func TestCostSubcommandExitCodes(t *testing.T) {
	write := func(body string) string {
		p := filepath.Join(t.TempDir(), "cost.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name, body string
		want       int
	}{
		{"all-measured", `{"schema":"cost-per-row-input-v1","desks":[{"desk":"a","model":"m","cost":10,"unit":"USD","passedRows":2}]}`, exitOK},
		{"one-blind", `{"schema":"cost-per-row-input-v1","desks":[{"desk":"a","model":"m","cost":10,"unit":"USD","passedRows":2},{"desk":"b","model":"m","passedRows":2}]}`, exitPublished},
		{"wrong-schema", `{"schema":"other","desks":[]}`, exitRefused},
		{"duplicate", `{"schema":"cost-per-row-input-v1","desks":[{"desk":"a","model":"m"},{"desk":"a","model":"m"}]}`, exitRefused},
		// An input that supplies no desk is not a fully measured run.
		{"empty-desks", `{"schema":"cost-per-row-input-v1","desks":[]}`, exitPublished},
		{"no-desks-key", `{"schema":"cost-per-row-input-v1"}`, exitPublished},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		if got := runCost([]string{"--input", write(c.body)}, &out, &errb); got != c.want {
			t.Errorf("%s: exit %d, want %d (stderr %q)", c.name, got, c.want, errb.String())
		}
	}
}

// TestCostRefusesUnsafeLabels: desk, model and unit are rendered into the report
// line, so an input whose label carries a line break, a control character or a
// backtick is refused whole (exit 2) and nothing is printed — otherwise it could
// forge a measured-looking line. The duplicate refusal keys on the NORMALISED
// pair (trimmed, empty model defaulted), so two entries that would render as the
// same desk/model cannot both pass. A unit outside the budget grammar is refused.
func TestCostRefusesUnsafeLabels(t *testing.T) {
	write := func(body string) string {
		p := filepath.Join(t.TempDir(), "cost.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct{ name, desks string }{
		{"newline-in-model", `{"desk":"a","model":"m\n- cost per passed row — desk ` + "`x`" + `, model ` + "`m`" + `: 1.00 tokens","cost":1,"unit":"tokens","passedRows":1}`},
		{"newline-in-desk", `{"desk":"a\nb","model":"m","cost":1,"unit":"tokens","passedRows":1}`},
		{"control-in-unit", `{"desk":"a","model":"m","cost":1,"unit":"tokens\u001b[2K","passedRows":1}`},
		{"backtick-in-desk", `{"desk":"a` + "`" + `b","model":"m","cost":1,"unit":"tokens","passedRows":1}`},
		{"unit-outside-grammar", `{"desk":"a","model":"m","cost":1,"unit":"dollars","passedRows":1}`},
		{"empty-model-vs-default-label", `{"desk":"a","model":"","cost":1,"unit":"tokens","passedRows":1},{"desk":"a","model":"model-unreported","cost":9,"unit":"tokens","passedRows":1}`},
		{"untrimmed-duplicate", `{"desk":"a","model":"m","cost":1,"unit":"tokens","passedRows":1},{"desk":" a ","model":"m ","cost":9,"unit":"tokens","passedRows":1}`},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		body := `{"schema":"cost-per-row-input-v1","desks":[` + c.desks + `]}`
		if got := runCost([]string{"--input", write(body)}, &out, &errb); got != exitRefused {
			t.Errorf("%s: exit %d, want %d (refused); stdout:\n%s", c.name, got, exitRefused, out.String())
		}
		if out.Len() != 0 {
			t.Errorf("%s: a refused input must print no report; got:\n%s", c.name, out.String())
		}
	}
}

// TestCostNegativeIsNotMeasured: a negative reported cost is broken telemetry,
// not a measurement — could-not-check, never a (cheapest-looking) figure.
func TestCostNegativeIsNotMeasured(t *testing.T) {
	lines := costPerPassedRow([]deskCostInput{
		{Desk: "neg-desk", Model: "example-model", Cost: json.RawMessage(`-50`), Unit: "tokens", PassedRows: json.RawMessage(`2`)},
	})
	if len(lines) != 1 || lines[0].State != stateCouldNotCheck || lines[0].PerRow != nil {
		t.Fatalf("a negative cost must be could-not-check with no per-row value; got %+v", lines)
	}
	if r := renderCostSection(lines); strings.Contains(r, "-25") || !strings.Contains(r, stateCouldNotCheck) {
		t.Errorf("a negative cost must render could-not-check, never a figure; got:\n%s", r)
	}
}

// TestCostSubUnitKeepsDigits: a small measured per-row figure keeps significant
// digits instead of rounding to a zero that reads like the unmeasured case.
func TestCostSubUnitKeepsDigits(t *testing.T) {
	lines := costPerPassedRow([]deskCostInput{
		{Desk: "small-desk", Model: "example-model", Cost: json.RawMessage(`0.01`), Unit: "USD", PassedRows: json.RawMessage(`4`)},
	})
	r := renderCostSection(lines)
	if !strings.Contains(r, "0.0025 USD") || zeroFigure.MatchString(lineFor(r, "small-desk")) {
		t.Errorf("0.01 USD over 4 rows must render 0.0025 USD, not a zero; got:\n%s", r)
	}
}
