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
	}
	lines := costPerPassedRow(in)
	byDesk := map[string]costLine{}
	for _, l := range lines {
		byDesk[l.Desk] = l
	}
	section := renderCostSection(lines)

	for _, desk := range []string{"absent-desk", "null-desk", "blind-desk", "unitless-desk", "idle-desk"} {
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
// input (wrong schema, duplicate desk/model).
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
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		if got := runCost([]string{"--input", write(c.body)}, &out, &errb); got != c.want {
			t.Errorf("%s: exit %d, want %d (stderr %q)", c.name, got, c.want, errb.String())
		}
	}
}
