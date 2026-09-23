// cost.go — cost per passed Verify row, per desk, per model.
//
// The board counts activity; this counts what the activity cost. For each
// (desk, model) pair the caller supplies the cost the desk's sessions reported
// (desk session telemetry — the `tokens` vital a desk session self-reports) and
// the number of Verify rows that PASSED on the work those sessions did, and the
// reducer divides one by the other.
//
// The one rule that matters here is the same rule as the rest of this module:
// A FIGURE THAT WAS NOT MEASURED NEVER RENDERS AS A NUMERAL. A desk that reported
// no telemetry has an UNKNOWN cost, not a zero one — publishing `0` would make the
// desk that reported nothing look like the cheapest desk on the board. So:
//
//   - cost absent, JSON null, or the string "could-not-check" → could-not-check
//   - cost present but no unit                                → could-not-check
//   - passed-row count absent or null                         → could-not-check
//   - passed-row count zero                                   → could-not-check
//     (a per-row figure with no rows has no denominator; the total spend is
//     still shown, but never divided into a made-up per-row number)
//
// The reducer never imputes, averages across desks, or back-fills from a prior
// day. Joining telemetry and Verify results into this input is the caller's job
// (the harvest that owns both); this file is the pure reduction and its
// rendering, with no gh/git reads.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// costInputSchema is the marker a cost input file must carry. An unmarked or
// differently-marked file is refused, never guessed at.
const costInputSchema = "cost-per-row-input-v1"

// costInput is the file the `cost` subcommand reads.
type costInput struct {
	Schema string          `json:"schema"`
	Desks  []deskCostInput `json:"desks"`
}

// deskCostInput is one (desk, model) pair's inputs. Cost and PassedRows keep
// their raw JSON so absent, null, and "could-not-check" are all kept apart from
// a measured value — the same shape the desk session telemetry writes.
type deskCostInput struct {
	Desk       string          `json:"desk"`
	Model      string          `json:"model"`
	Cost       json.RawMessage `json:"cost"`
	Unit       string          `json:"unit"`
	PassedRows json.RawMessage `json:"passedRows"`
}

// costLine is one published per-desk, per-model figure. PerRow is nil for every
// state but measured.
type costLine struct {
	Desk       string   `json:"desk"`
	Model      string   `json:"model"`
	State      string   `json:"state"`
	PerRow     *float64 `json:"perRow"`
	Total      *float64 `json:"total"`
	Unit       string   `json:"unit,omitempty"`
	PassedRows *int     `json:"passedRows"`
	Note       string   `json:"note,omitempty"`
}

// rawNumber reads a raw JSON value as a measured number. ok is false for an
// absent value, JSON null, any string (including "could-not-check"), or anything
// else that is not a JSON number — every one of those is "not measured".
func rawNumber(raw json.RawMessage) (float64, bool) {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || bytes.Equal(t, []byte("null")) || t[0] == '"' {
		return 0, false
	}
	var n json.Number
	dec := json.NewDecoder(bytes.NewReader(t))
	dec.UseNumber()
	if err := dec.Decode(&n); err != nil {
		return 0, false
	}
	f, err := n.Float64()
	if err != nil {
		return 0, false
	}
	return f, true
}

// costPerPassedRow reduces the inputs into one line per (desk, model), sorted by
// desk then model. It never returns a measured line whose inputs were not both
// measured.
func costPerPassedRow(in []deskCostInput) []costLine {
	out := make([]costLine, 0, len(in))
	for _, d := range in {
		line := costLine{Desk: d.Desk, Model: d.Model, State: stateCouldNotCheck, Unit: strings.TrimSpace(d.Unit)}
		if line.Model == "" {
			line.Model = "model-unreported"
		}
		cost, costOK := rawNumber(d.Cost)
		rowsF, rowsOK := rawNumber(d.PassedRows)
		if rowsOK && (rowsF < 0 || rowsF != float64(int(rowsF))) {
			rowsOK = false // a negative or fractional row count is not a count
		}
		if rowsOK {
			r := int(rowsF)
			line.PassedRows = &r
		}
		switch {
		case !costOK:
			line.Note = "no cost telemetry reported"
		case line.Unit == "":
			line.Note = "cost reported without a unit"
		default:
			c := cost
			line.Total = &c
			switch {
			case !rowsOK:
				line.Note = "passed-row count not reported"
			case *line.PassedRows == 0:
				line.Note = "no passed Verify rows — no per-row denominator"
			default:
				per := cost / float64(*line.PassedRows)
				line.PerRow = &per
				line.State = stateMeasured
			}
		}
		out = append(out, line)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Desk != out[j].Desk {
			return out[i].Desk < out[j].Desk
		}
		return out[i].Model < out[j].Model
	})
	return out
}

// fmtAmount renders a reported amount exactly as reported, with no trailing ".0".
func fmtAmount(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// fmtPerRow renders a derived per-row figure at two decimals.
func fmtPerRow(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// renderCostSection renders the lines as a markdown section. Every line starts
// with "cost per passed row" and names its desk, so a reader (or a grep) finds
// the figure per desk; a measured figure always carries its unit, and an
// unmeasured one renders the word for what happened to it in the same place a
// number would have been.
func renderCostSection(lines []costLine) string {
	var b strings.Builder
	b.WriteString("## Cost per passed Verify row\n\n")
	if len(lines) == 0 {
		b.WriteString("- cost per passed row: not-configured (no desk supplied)\n")
		return b.String()
	}
	for _, l := range lines {
		fmt.Fprintf(&b, "- cost per passed row — desk `%s`, model `%s`: ", l.Desk, l.Model)
		if l.State == stateMeasured {
			fmt.Fprintf(&b, "%s %s (%s %s over %d passed rows)\n",
				fmtPerRow(*l.PerRow), l.Unit, fmtAmount(*l.Total), l.Unit, *l.PassedRows)
			continue
		}
		fmt.Fprintf(&b, "%s (%s)", stateCouldNotCheck, l.Note)
		if l.Total != nil {
			fmt.Fprintf(&b, "; total spend %s %s", fmtAmount(*l.Total), l.Unit)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// runCost is the `cost` subcommand: read one cost input file, print the rendered
// section to stdout. Exit 0 when every line is measured, 3 when at least one is
// could-not-check (the section still prints — the markers are the evidence), 2
// when the input is refused.
func runCost(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("metrics-harvest cost", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inPath := fs.String("input", "", "path to a "+costInputSchema+" JSON file")
	if err := fs.Parse(args); err != nil {
		return exitRefused
	}
	if *inPath == "" {
		fmt.Fprintln(stderr, "error: --input is required")
		return exitRefused
	}
	raw, err := os.ReadFile(*inPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitError
	}
	var in costInput
	if err := json.Unmarshal(raw, &in); err != nil {
		fmt.Fprintf(stderr, "error: %s: %v\n", *inPath, err)
		return exitRefused
	}
	if in.Schema != costInputSchema {
		fmt.Fprintf(stderr, "error: %s: schema %q, want %q — refusing to reduce an unrecognized input\n", *inPath, in.Schema, costInputSchema)
		return exitRefused
	}
	seen := map[string]bool{}
	for _, d := range in.Desks {
		if strings.TrimSpace(d.Desk) == "" {
			fmt.Fprintf(stderr, "error: %s: an entry has no desk name\n", *inPath)
			return exitRefused
		}
		k := d.Desk + "\x00" + d.Model
		if seen[k] {
			fmt.Fprintf(stderr, "error: %s: desk %q model %q appears twice — ambiguous input\n", *inPath, d.Desk, d.Model)
			return exitRefused
		}
		seen[k] = true
	}
	lines := costPerPassedRow(in.Desks)
	fmt.Fprint(stdout, renderCostSection(lines))
	for _, l := range lines {
		if l.State != stateMeasured {
			return exitPublished
		}
	}
	return exitOK
}
