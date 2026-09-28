package main

// flow.go — `deskinbox flow [--root PATH ...] [--since YYYY-MM-DD] [--html OUT.html]`, the
// pipeline FLOW model: "how is the system performing", as opposed to `walk`/`html`'s "which
// decision is next". Ported from the oracle's collect_flow (assay-inbox.sh:949-1009),
// write_flow_program (assay-inbox.sh:1017-1233, the JQFLOW heredoc — the ONE interpreter
// both the terminal table and the SVG diagram read from) and render_flow_text
// (assay-inbox.sh:1236-1284).
//
// THE ONE PLACE THIS PACKAGE SHELLS A SUBPROCESS. Every other read in this package reaches
// the forge through deskkit.ForgeFor — no shell-out (main.go's header). The flow model's four
// numbers come from OTHER DESK BINARIES (statusgen, deskboard), not a forge read, so
// os/exec is the correct tool here, scoped to exactly those two binaries (Verify row 5: every
// exec.Command site in this package must name statusgen or deskboard).
//
// DERIVED, NOT PROBED. Nothing here opens a new source or re-parses STATUS.md; every number
// is lifted from one reader's own JSON, named beside it on the render.
//
// BLIND IS NOT ZERO. A reader that failed leaves its stage could-not-check, carrying the
// reader's own diagnostic — never rounded to 0, which would read as DRAINED rather than
// unread.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// ---------------------------------------------------------------- binary resolution --

// statusgenBinEnv / deskboardBinEnv are the oracle's own override variables
// (assay-inbox.sh:107-110) — kept as the oracle spells them (NOT deskboard's own
// STATUSGEN_BIN) because this reader is a port of the oracle's contract, quoted verbatim in
// the brief and in plugins/assay/commands/inbox.md.
const (
	statusgenBinEnv = "ASSAY_STATUSGEN"
	deskboardBinEnv = "ASSAY_DESKBOARD"
)

// lookPathFn is the PATH-resolution seam, swapped in tests so runFlow/runHTML's binary
// resolution never depends on whether a real statusgen/deskboard happens to be on the test
// runner's PATH — the interpretation logic under test (interpretFlow, readJSON) is driven
// through runReaderFn instead, which needs no real binary either.
var lookPathFn = exec.LookPath

// resolveFlowBin resolves the flow reader `want` ("statusgen" or "deskboard") from PATH, or
// from the override variable envVar. The override may re-point WHICH build of the reader
// runs (a pinned path, a sibling checkout's build), never WHAT runs: a value whose base name —
// with a trailing `.exe` stripped, case-insensitively, for Windows — is not `want` is refused.
// That bound is what makes runReaderFn's forge-surface ledger row
// (internal/forgeban/allowlist.go, UnresolvedArgv) true rather than true-by-convention: the
// exec site's argv[0] is always a statusgen or deskboard, never a forge CLI (`gh`, `glab`)
// or any other binary an environment variable happens to name.
func resolveFlowBin(envVar, want string) (string, error) {
	bin := strings.TrimSpace(os.Getenv(envVar))
	if bin == "" {
		bin = want
	}
	if base := flowBinBase(bin); base != want {
		return "", deskkit.Refused(envVar + "=" + bin + " does not name " + want +
			" (base name " + base + "); the override may point at a different " + want +
			" build, never at a different binary")
	}
	path, err := lookPathFn(bin)
	if err != nil {
		return "", deskkit.Unverifiable(bin+" is not on PATH (set "+envVar+" to override)", err)
	}
	if base := flowBinBase(path); base != want {
		// LookPath returns its input unchanged for a path-shaped name, but re-check what it
		// resolved rather than assume it.
		return "", deskkit.Refused(envVar + " resolved to " + path + ", which is not " + want)
	}
	return path, nil
}

// flowBinBase is a reader path's base name with a trailing `.exe` stripped
// (case-insensitively), separator-agnostic so a Windows-style override is judged the same way
// on every OS.
func flowBinBase(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		p = p[i+1:]
	}
	if len(p) > 4 && strings.EqualFold(p[len(p)-4:], ".exe") {
		p = p[:len(p)-4]
	}
	return p
}

// ---------------------------------------------------------------------- cell resolution --

// cellSpec is one resolved cell (a statusgen root) — the oracle's CELL_NAMES[i]/CELL_PATHS[i]
// pair.
type cellSpec struct {
	Name string
	Path string
}

var cellsTxtSplitRe = regexp.MustCompile(`\s+`)

// resolveCells mirrors resolve_cells() (assay-inbox.sh:222-244): --root args win outright;
// else ./.assay/cells.txt (NAME PATH | NAME\tPATH | bare PATH, blank/#-comment lines
// ignored); else "." named for the current directory.
func resolveCells(rootArgs []string, cwd string) ([]cellSpec, error) {
	if len(rootArgs) > 0 {
		out := make([]cellSpec, 0, len(rootArgs))
		for _, p := range rootArgs {
			out = append(out, cellSpec{Name: cellNameFor(p), Path: p})
		}
		return out, nil
	}

	cellsTxt := filepath.Join(cwd, ".assay", "cells.txt")
	if f, err := os.Open(cellsTxt); err == nil {
		defer f.Close()
		raw, rerr := io.ReadAll(f)
		if rerr != nil {
			return nil, deskkit.Unverifiable("cannot read "+cellsTxt, rerr)
		}
		var out []cellSpec
		for _, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			fields := cellsTxtSplitRe.Split(trimmed, 2)
			var name, path string
			if len(fields) == 2 && strings.TrimSpace(fields[1]) != "" {
				name = fields[0]
				path = strings.TrimSpace(fields[1])
			} else {
				path = fields[0]
				name = filepath.Base(path)
			}
			out = append(out, cellSpec{Name: name, Path: path})
		}
		return out, nil
	}

	return []cellSpec{{Name: cellNameFor(cwd), Path: "."}}, nil
}

// cellNameFor is the oracle's cell_name_for(): the origin remote's owner/name where path is
// a checkout with a resolvable remote, else the directory's own basename.
func cellNameFor(path string) string {
	if slug := deskkit.RepoSlugForDir(path); slug != "" {
		return slug
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Base(path)
	}
	return filepath.Base(abs)
}

// cellSHA is the oracle's `git -C "$path" rev-parse --short HEAD` — read via gitcore (pure
// Go, no shell-out: Verify row 5 scopes exec.Command to statusgen/deskboard only) rather than
// a `git` subprocess.
func cellSHA(path string) string {
	repo, err := gitcore.Open(path)
	if err != nil {
		return "could-not-check"
	}
	h, err := repo.Resolve("HEAD")
	if err != nil {
		return "could-not-check"
	}
	s := h.String()
	if len(s) > 7 {
		s = s[:7]
	}
	return s
}

// --------------------------------------------------------------------------- readers --

// runReaderFn is the subprocess seam — swapped in tests for a canned stdout/stderr/error, so
// the interpretation logic (interpretFlow) and its parity test never need a real statusgen or
// deskboard binary.
var runReaderFn = func(bin string, args []string) (stdout, stderr []byte, err error) {
	// bin is always resolveFlowBin's result, which refuses any base name but statusgen/deskboard
	// (Verify row 5; the forge-surface ledger row for this site states the same bound).
	cmd := exec.Command(bin, args...) // statusgen or deskboard only
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	return outBuf.Bytes(), errBuf.Bytes(), runErr
}

// stripControl removes every C0/DEL byte — the oracle's `tr -d '\000-\037'` on the
// diagnostic line.
func stripControl(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// firstDiagnosticLine is the oracle's `grep -v '^assay-config' | head -1 | tr -d
// '\000-\037'`: the first stderr line that does not start with the routine provenance
// banner, control-stripped.
func firstDiagnosticLine(stderr []byte) string {
	for _, line := range strings.Split(string(stderr), "\n") {
		if strings.HasPrefix(line, "assay-config") {
			continue
		}
		return stripControl(line)
	}
	return ""
}

// readJSON is the oracle's read_json(): run a reader, and report {ok, err} exactly as it
// does — a non-zero exit OR output that does not parse as JSON (or parses to a bare
// null/false, matching jq -e's truthiness) is could-not-check, never a crash and never
// treated as an empty result.
func readJSON(label, bin string, args []string) (ok bool, errText string, raw []byte) {
	stdout, stderr, runErr := runReaderFn(bin, args)
	if runErr != nil {
		detail := firstDiagnosticLine(stderr)
		if detail == "" {
			if ee, isExit := runErr.(*exec.ExitError); isExit {
				detail = fmt.Sprintf("exit %d, no diagnostic", ee.ExitCode())
			} else {
				detail = runErr.Error()
			}
		}
		fmt.Fprintf(os.Stderr, "deskinbox: FLOW READER FAILED %s: %s\n", label, detail)
		return false, detail, nil
	}
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 || !json.Valid(trimmed) || string(trimmed) == "null" || string(trimmed) == "false" {
		fmt.Fprintf(os.Stderr, "deskinbox: FLOW READER %s exited 0 but did not emit JSON\n", label)
		return false, "reader exited 0 but its output is not JSON", nil
	}
	return true, "", trimmed
}

// ----------------------------------------------------------------------- raw envelope --

type bottleneckStageJSON struct {
	Stage        string `json:"stage"`
	WIP          int    `json:"wip"`
	MedianDwell  string `json:"median_dwell"`
	UnknownDwell int    `json:"unknown_dwell"`
}

type bottleneckJSON struct {
	Constraint string                `json:"constraint"`
	Stages     []bottleneckStageJSON `json:"stages"`
}

type intakeDebtJSON struct {
	State     string `json:"state"`
	Untriaged int    `json:"untriaged"`
}

type netFlowStreamJSON struct {
	Arrivals    int `json:"arrivals"`
	Completions int `json:"completions"`
}

type netFlowJSON struct {
	State   string              `json:"state"`
	Streams []netFlowStreamJSON `json:"streams"`
}

type throughputStageJSON struct {
	Stage       string   `json:"stage"`
	Depth       *int     `json:"depth"`
	Slots       int      `json:"slots"`
	SlotsSource string   `json:"slotsSource"`
	Ratio       *float64 `json:"ratio"`
	MaxSlots    int      `json:"maxSlots"`
	BoundBy     string   `json:"boundBy"`
	Blind       string   `json:"blind"`
	DepthNote   string   `json:"depthNote"`
}

type throughputJSON struct {
	Bottleneck  string                `json:"bottleneck"`
	StagesRead  int                   `json:"stagesRead"`
	StagesTotal int                   `json:"stagesTotal"`
	Advice      string                `json:"advice"`
	Stages      []throughputStageJSON `json:"stages"`
}

// cellRaw is one cell's raw reader output — the oracle's per-cell object collect_flow
// assembles (assay-inbox.sh:982-990). A nil pointer is the oracle's `null`: the reader was
// not OK, so the field carries no parsed value, only its Err string.
//
// JSON tags match the oracle's own field spelling (assay-inbox.sh:982-990) exactly, so a
// single fixture file can drive both the real jq program and this port in the parity test
// (flow_parity_test.go) — the same discipline format_parity_test.go established.
type cellRaw struct {
	Cell          string          `json:"cell"`
	Root          string          `json:"root"`
	SHA           string          `json:"sha"`
	Bottleneck    *bottleneckJSON `json:"bottleneck"`
	BottleneckErr string          `json:"bottleneckErr"`
	Intake        *intakeDebtJSON `json:"intake"`
	IntakeErr     string          `json:"intakeErr"`
	NetFlow       *netFlowJSON    `json:"netflow"`
	NetFlowErr    string          `json:"netflowErr"`
}

// rawDoc is the oracle's $TMP_RAW — the dumb envelope write_flow_program interprets. JSON
// tags match collect_flow's own emission (assay-inbox.sh:1000-1006).
type rawDoc struct {
	AsOf          string          `json:"asOf"`
	Since         string          `json:"since"`
	Cells         []cellRaw       `json:"cells"`
	Throughput    *throughputJSON `json:"throughput"`
	ThroughputErr string          `json:"throughputErr"`
}

// decodeInto parses raw into a fresh *T, returning nil (could-not-check, not a crash) on any
// unmarshal failure — the same leniency the oracle's `jq -e .` + free-form field access
// tolerates from a schema-drifted reader. On failure it writes the decode error into *errOut
// (and onto stderr, as readJSON does for a reader that failed outright), so the blind stage
// carries WHY it is blind — a reader that exited 0 with JSON of the wrong shape — rather than
// the generic "not read" / "stage not emitted" fallback.
func decodeInto[T any](label string, raw []byte, errOut *string) *T {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		*errOut = "reader output is JSON but not the expected shape: " + stripControl(err.Error())
		fmt.Fprintf(os.Stderr, "deskinbox: FLOW READER %s: %s\n", label, *errOut)
		return nil
	}
	return &v
}

// collectFlow runs every reader once per cell plus ONE fleet-wide `deskboard throughput`,
// ported from collect_flow (assay-inbox.sh:949-1009).
func collectFlow(cells []cellSpec, since, statusgenBin, deskboardBin, asOf string) rawDoc {
	doc := rawDoc{AsOf: asOf, Since: since}
	for _, c := range cells {
		cr := cellRaw{Cell: c.Name, Root: c.Path, SHA: cellSHA(c.Path)}

		if ok, errText, raw := readJSON("statusgen --bottleneck ("+c.Name+")", statusgenBin,
			[]string{"--root", c.Path, "--bottleneck", "--json"}); ok {
			cr.Bottleneck = decodeInto[bottleneckJSON]("statusgen --bottleneck ("+c.Name+")", raw, &cr.BottleneckErr)
		} else {
			cr.BottleneckErr = errText
		}

		if ok, errText, raw := readJSON("statusgen --intake-debt ("+c.Name+")", statusgenBin,
			[]string{"--root", c.Path, "--intake-debt", "--json"}); ok {
			cr.Intake = decodeInto[intakeDebtJSON]("statusgen --intake-debt ("+c.Name+")", raw, &cr.IntakeErr)
		} else {
			cr.IntakeErr = errText
		}

		nfArgs := []string{"--root", c.Path, "--net-flow", "--json"}
		if since != "" {
			nfArgs = append(nfArgs, "--since", since)
		}
		if ok, errText, raw := readJSON("statusgen --net-flow ("+c.Name+")", statusgenBin, nfArgs); ok {
			cr.NetFlow = decodeInto[netFlowJSON]("statusgen --net-flow ("+c.Name+")", raw, &cr.NetFlowErr)
		} else {
			cr.NetFlowErr = errText
		}

		doc.Cells = append(doc.Cells, cr)
	}

	if ok, errText, raw := readJSON("deskboard throughput", deskboardBin, []string{"throughput", "--json"}); ok {
		doc.Throughput = decodeInto[throughputJSON]("deskboard throughput", raw, &doc.ThroughputErr)
	} else {
		doc.ThroughputErr = errText
	}

	return doc
}

// ------------------------------------------------------------------------ flow model --

// flowArrow is one edge between two adjacent stages.
// JSON tags on flowArrow/flowStage/flowRow/flowModel match the oracle's own output field
// spelling exactly (write_flow_program, assay-inbox.sh:1017-1233) so flow_parity_test.go can
// decode the REAL jq program's output straight into these types and compare against
// interpretFlow's result with reflect.DeepEqual — no hand-written second expectation to
// drift, the same discipline format_parity_test.go established for the format builder.
type flowArrow struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Weight *int   `json:"weight"`
	Blind  string `json:"blind"`
	Source string `json:"source"`
}

// flowStage is one stage's rendering data, per row (fleet or per-cell).
type flowStage struct {
	Stage        string   `json:"stage"`
	Label        string   `json:"label"`
	Count        *int     `json:"count"`
	CountPartial bool     `json:"countPartial"`
	Blind        string   `json:"blind"`
	NA           string   `json:"na"`
	CountSource  string   `json:"countSource"`
	Dwell        string   `json:"dwell"`
	UnknownDwell *int     `json:"unknownDwell"`
	Queue        *int     `json:"queue"`
	QueueBlind   string   `json:"queueBlind"`
	QueueSource  string   `json:"queueSource"`
	Slots        *int     `json:"slots"`
	SlotsSource  string   `json:"slotsSource"`
	Ratio        *float64 `json:"ratio"`
	MaxSlots     *int     `json:"maxSlots"`
	BoundBy      string   `json:"boundBy"`
	CapacityNote string   `json:"capacityNote"`
	IsBottleneck bool     `json:"isBottleneck"`
}

// flowRow is one row of the model — the fleet totals, or (when more than one cell resolved)
// one cell's own block.
type flowRow struct {
	Cell          string      `json:"cell"`
	Root          string      `json:"root"`
	SHA           string      `json:"sha"`
	Fleet         bool        `json:"fleet"`
	Constraint    string      `json:"constraint"`
	ConstraintSrc string      `json:"constraintSource"`
	Arrivals      *int        `json:"arrivals"`
	Completions   *int        `json:"completions"`
	FlowBlind     string      `json:"flowBlind"`
	Arrows        []flowArrow `json:"arrows"`
	CapacityNote  string      `json:"capacityNote"`
	Stages        []flowStage `json:"stages"`
}

// flowModel is the oracle's finished flow model (write_flow_program's output object) — both
// render_flow_text and the SVG/table page render this ONE structure.
type flowModel struct {
	AsOf             string    `json:"asOf"`
	Since            string    `json:"since"`
	Bottleneck       string    `json:"bottleneck"`
	BottleneckSource string    `json:"bottleneckSource"`
	Advice           string    `json:"advice"`
	StagesRead       int       `json:"stagesRead"`
	StagesTotal      int       `json:"stagesTotal"`
	Note             string    `json:"note"`
	Rows             []flowRow `json:"rows"`
	CellCount        int       `json:"cellCount"`
	Sources          []string  `json:"sources"`
	Blind            []string  `json:"blind"`
}

// stagedefs is the pipeline order, ported verbatim from the oracle (assay-inbox.sh:1025-1033).
var stagedefs = []struct{ Stage, Label, TP string }{
	{"intake", "raw-intake front door", "intake"},
	{"todo", "authored, not started", "dispatch"},
	{"in-progress", "dispatched, being worked", ""},
	{"review", "open PRs, no verdict at head", "review"},
	{"implemented", "merged, awaiting a verifier", "verify"},
	{"verified", "verified, awaiting the flip", ""},
	{"done", "exited the pipeline", ""},
}

const flowInteriorNote = "could-not-check: no reader in the tree emits per-stage transition counts for a window. " +
	"`--net-flow` measures arrivals INTO the pipeline and completions OUT of it, not the " +
	"steps between, and a delta inferred from two board reads would be a measurement this " +
	"system does not take."

var flowSources = []string{
	"statusgen --bottleneck --json — per-stage board WIP and median dwell, one call per cell",
	"statusgen --intake-debt --json — the intake front-door depth and its oldest entry",
	"statusgen --net-flow --json — arrivals into and completions out of the pipeline for the window",
	"deskboard throughput --json — per-stage queue depth against resolved pool width, and the bottleneck",
}

const flowNote = "This diagram RENDERS AUTHORED STATUS. The board lints consistency between status " +
	"cells, Evidence and PRs; it does not measure whether the work is done. Every count " +
	"here is exactly as good as the status somebody wrote."

func sgstage(bn *bottleneckJSON, name string) *bottleneckStageJSON {
	if bn == nil {
		return nil
	}
	for i := range bn.Stages {
		if bn.Stages[i].Stage == name {
			return &bn.Stages[i]
		}
	}
	return nil
}

func tpstage(tp *throughputJSON, name string) *throughputStageJSON {
	if tp == nil || name == "" {
		return nil
	}
	for i := range tp.Stages {
		if tp.Stages[i].Stage == name {
			return &tp.Stages[i]
		}
	}
	return nil
}

// tp2flow translates deskboard throughput's stage vocabulary into the flow model's — the
// oracle's tp2flow def.
func tp2flow(n string) string {
	switch n {
	case "dispatch":
		return "todo"
	case "review":
		return "review"
	case "verify":
		return "implemented"
	case "intake":
		return "intake"
	default:
		return ""
	}
}

func blindtext(err, fallback string) string {
	if err == "" {
		return fallback
	}
	return err
}

func intp(n int) *int { return &n }

// cellCountResult and cellDwellResult are cellcount()/celldwell()'s return shapes.
type cellCountResult struct {
	Count *int
	NA    string
	Blind string
}

type cellDwellResult struct {
	Dwell        string
	UnknownDwell *int
}

// cellcount is the oracle's cellcount($c; $sd): one cell's COUNT for one stage, with the
// reason when there is none (assay-inbox.sh:1062-1078).
func cellcount(c cellRaw, sd struct{ Stage, Label, TP string }) cellCountResult {
	switch sd.Stage {
	case "intake":
		if c.Intake == nil {
			return cellCountResult{NA: "", Blind: "statusgen --intake-debt: " + blindtext(c.IntakeErr, "not read")}
		}
		if c.Intake.State != "measured" {
			state := c.Intake.State
			if state == "" {
				state = "absent"
			}
			return cellCountResult{NA: "", Blind: "statusgen --intake-debt reported state=" + state}
		}
		return cellCountResult{Count: intp(c.Intake.Untriaged), NA: "", Blind: ""}
	case "review":
		return cellCountResult{Blind: "", NA: "read fleet-wide by `deskboard throughput`, which resolves no per-cell figure"}
	default:
		s := sgstage(c.Bottleneck, sd.Stage)
		if s == nil {
			return cellCountResult{NA: "", Blind: "statusgen --bottleneck: " + blindtext(c.BottleneckErr, "stage not emitted")}
		}
		return cellCountResult{Count: intp(s.WIP), NA: "", Blind: ""}
	}
}

func celldwell(c cellRaw, sd struct{ Stage, Label, TP string }) cellDwellResult {
	s := sgstage(c.Bottleneck, sd.Stage)
	if s == nil {
		return cellDwellResult{}
	}
	return cellDwellResult{Dwell: s.MedianDwell, UnknownDwell: intp(s.UnknownDwell)}
}

// cellflowResult is cellflow()'s return shape: this cell's arrivals/completions for the
// window, or the reason they are absent.
type cellflowResult struct {
	Arrivals, Completions *int
	FlowBlind             string
}

func cellflow(c cellRaw) cellflowResult {
	if c.NetFlow == nil {
		return cellflowResult{FlowBlind: "statusgen --net-flow: " + blindtext(c.NetFlowErr, "not read")}
	}
	if c.NetFlow.State != "ok" {
		state := c.NetFlow.State
		if state == "" {
			state = "absent"
		}
		return cellflowResult{FlowBlind: "statusgen --net-flow reported state=" + state}
	}
	var arr, comp int
	for _, s := range c.NetFlow.Streams {
		arr += s.Arrivals
		comp += s.Completions
	}
	return cellflowResult{Arrivals: intp(arr), Completions: intp(comp), FlowBlind: ""}
}

// arrows is the oracle's arrows($arr;$comp;$flowBlind): the six fixed edges between the
// seven stages (assay-inbox.sh:1098-1106).
func arrows(arr, comp *int, flowBlind string) []flowArrow {
	return []flowArrow{
		{From: "intake", To: "todo", Weight: arr, Blind: flowBlind,
			Source: "statusgen --net-flow --json (.streams[].arrivals, summed)"},
		{From: "todo", To: "in-progress", Weight: nil, Blind: flowInteriorNote, Source: ""},
		{From: "in-progress", To: "review", Weight: nil, Blind: flowInteriorNote, Source: ""},
		{From: "review", To: "implemented", Weight: nil, Blind: flowInteriorNote, Source: ""},
		{From: "implemented", To: "verified", Weight: nil, Blind: flowInteriorNote, Source: ""},
		{From: "verified", To: "done", Weight: comp, Blind: flowBlind,
			Source: "statusgen --net-flow --json (.streams[].completions, summed)"},
	}
}

// interpretFlow is the oracle's write_flow_program applied to the raw reader envelope — the
// ONE interpreter both render_flow_text and the SVG/table page read from
// (assay-inbox.sh:1017-1233).
func interpretFlow(raw rawDoc) flowModel {
	tp := raw.Throughput
	bneck := tp2flow("")
	if tp != nil {
		bneck = tp2flow(tp.Bottleneck)
	}

	// ---- per-cell rows -----------------------------------------------------------
	cellRows := make([]flowRow, 0, len(raw.Cells))
	for _, c := range raw.Cells {
		f := cellflow(c)
		row := flowRow{
			Cell: c.Cell, Root: c.Root, SHA: c.SHA, Fleet: false,
			ConstraintSrc: "statusgen --bottleneck --json (.constraint — WIP x dwell, the ToC locator)",
			Arrivals:      f.Arrivals, Completions: f.Completions, FlowBlind: f.FlowBlind,
			Arrows:       arrows(f.Arrivals, f.Completions, f.FlowBlind),
			CapacityNote: "pool width is resolved fleet-wide, so this block carries no slots or ratio — see the fleet row",
		}
		if c.Bottleneck != nil {
			row.Constraint = c.Bottleneck.Constraint
		}
		for _, sd := range stagedefs {
			cc := cellcount(c, sd)
			cd := celldwell(c, sd)
			countSource := "statusgen --bottleneck --json (.stages[].wip) @ " + c.SHA
			if sd.Stage == "intake" {
				countSource = "statusgen --intake-debt --json (.untriaged) @ " + c.SHA
			} else if sd.Stage == "review" {
				countSource = ""
			}
			row.Stages = append(row.Stages, flowStage{
				Stage: sd.Stage, Label: sd.Label,
				Count: cc.Count, CountPartial: false, Blind: cc.Blind, NA: cc.NA,
				CountSource: countSource, Dwell: cd.Dwell, UnknownDwell: cd.UnknownDwell,
			})
		}
		cellRows = append(cellRows, row)
	}

	// ---- the fleet row -------------------------------------------------------------
	var fleetStages []flowStage
	for _, sd := range stagedefs {
		var known []int
		var unknown []flowStage
		for _, cr := range cellRows {
			for _, st := range cr.Stages {
				if st.Stage != sd.Stage {
					continue
				}
				if st.Count != nil {
					known = append(known, *st.Count)
				} else {
					unknown = append(unknown, st)
				}
			}
		}
		t := tpstage(tp, sd.TP)

		var count *int
		if sd.Stage == "review" {
			if t != nil {
				count = t.Depth
			}
		} else if len(known) > 0 {
			sum := 0
			for _, k := range known {
				sum += k
			}
			count = intp(sum)
		}

		countPartial := len(unknown) > 0 && len(known) > 0 && sd.Stage != "review"

		var blind string
		if count != nil {
			blind = ""
		} else if sd.Stage == "review" {
			fallback := "stage not emitted"
			if t != nil {
				fallback = t.Blind
				if fallback == "" {
					fallback = "depth not read"
				}
			}
			blind = "deskboard throughput: " + blindtext(raw.ThroughputErr, fallback)
		} else {
			blind = "no reader supplied this stage"
			for _, u := range unknown {
				if u.Blind != "" {
					blind = u.Blind
					break
				}
			}
		}

		countSource := "statusgen --bottleneck --json (.stages[].wip), summed over cells"
		if sd.Stage == "intake" {
			countSource = "statusgen --intake-debt --json (.untriaged), summed over cells"
		} else if sd.Stage == "review" {
			countSource = "deskboard throughput --json (review depth)"
		}

		var dwell string
		var unknownDwell *int
		if len(cellRows) == 1 {
			for _, st := range cellRows[0].Stages {
				if st.Stage == sd.Stage {
					dwell = st.Dwell
					unknownDwell = st.UnknownDwell
					break
				}
			}
		}

		var queue *int
		if t != nil {
			queue = t.Depth
		}
		var queueBlind string
		if sd.TP == "" {
			queueBlind = ""
		} else if t == nil {
			queueBlind = blindtext(raw.ThroughputErr, "deskboard throughput not read")
		} else if t.Depth == nil {
			queueBlind = t.Blind
			if queueBlind == "" {
				queueBlind = "depth not read"
			}
		}
		var queueSource string
		if t != nil {
			queueSource = "deskboard throughput --json: " + t.DepthNote
		}
		var slots *int
		var slotsSource string
		var ratio *float64
		var maxSlots *int
		var boundBy string
		if t != nil {
			slots = intp(t.Slots)
			slotsSource = t.SlotsSource
			ratio = t.Ratio
			maxSlots = intp(t.MaxSlots)
			boundBy = t.BoundBy
		}
		capacityNote := ""
		if sd.TP == "" {
			capacityNote = "no loop owns this stage — it has no pool to size"
		}

		fleetStages = append(fleetStages, flowStage{
			Stage: sd.Stage, Label: sd.Label, Count: count, NA: "",
			CountPartial: countPartial, Blind: blind, CountSource: countSource,
			Dwell: dwell, UnknownDwell: unknownDwell,
			Queue: queue, QueueBlind: queueBlind, QueueSource: queueSource,
			Slots: slots, SlotsSource: slotsSource, Ratio: ratio, MaxSlots: maxSlots, BoundBy: boundBy,
			CapacityNote: capacityNote, IsBottleneck: sd.Stage == bneck && bneck != "",
		})
	}

	var arrs, comps []int
	for _, cr := range cellRows {
		if cr.Arrivals != nil {
			arrs = append(arrs, *cr.Arrivals)
		}
		if cr.Completions != nil {
			comps = append(comps, *cr.Completions)
		}
	}
	var fleetFlowBlind string
	for _, cr := range cellRows {
		if cr.FlowBlind != "" {
			fleetFlowBlind = cr.FlowBlind
			break
		}
	}
	var fleetArr, fleetComp *int
	if len(arrs) > 0 {
		s := 0
		for _, v := range arrs {
			s += v
		}
		fleetArr = intp(s)
	}
	if len(comps) > 0 {
		s := 0
		for _, v := range comps {
			s += v
		}
		fleetComp = intp(s)
	}

	fleetRow := flowRow{
		Cell: "fleet", Root: "", SHA: "", Fleet: true,
		ConstraintSrc: "per cell only: the ToC constraint is computed per root and is not summable",
		Arrivals:      fleetArr, Completions: fleetComp, FlowBlind: fleetFlowBlind,
		Arrows: arrows(fleetArr, fleetComp, fleetFlowBlind),
		Stages: fleetStages,
	}
	if len(cellRows) == 1 {
		fleetRow.Constraint = cellRows[0].Constraint
		fleetRow.ConstraintSrc = "statusgen --bottleneck --json (.constraint — WIP x dwell, the ToC locator)"
	}

	since := raw.Since
	if since == "" {
		since = "the reader's default window"
	}

	advice := adviceFor(tp, raw.ThroughputErr)
	stagesRead, stagesTotal := 0, 4
	if tp != nil {
		stagesRead, stagesTotal = tp.StagesRead, tp.StagesTotal
	}

	rows := []flowRow{fleetRow}
	if len(cellRows) > 1 {
		rows = append(rows, cellRows...)
	}

	blindLines := collectBlindLines(tp, raw.ThroughputErr, fleetStages, cellRows)

	return flowModel{
		AsOf: raw.AsOf, Since: since,
		Bottleneck:       bneck,
		BottleneckSource: "deskboard throughput --json (.bottleneck — the largest queue/slots ratio among the stages it read)",
		Advice:           advice,
		StagesRead:       stagesRead, StagesTotal: stagesTotal,
		Note: flowNote, Rows: rows, CellCount: len(cellRows),
		Sources: append([]string(nil), flowSources...),
		Blind:   blindLines,
	}
}

// collectBlindLines is the oracle's $blind list (assay-inbox.sh:1228-1231): unique() sorts
// ascending AND dedupes — Go's sort.Strings + a dedupe pass reproduces that exactly.
func collectBlindLines(tp *throughputJSON, tperr string, fleetStages []flowStage, cellRows []flowRow) []string {
	// jq's `unique` on an empty input emits `[]`, never `null` — matched here so a
	// reflect.DeepEqual against the decoded oracle output (flow_parity_test.go) does not
	// trip on a nil-vs-empty-slice difference that has no observable effect on any render.
	out := []string{}
	if tp == nil {
		out = append(out, "deskboard throughput: "+blindtext(tperr, "not read"))
	}
	for _, s := range fleetStages {
		if s.QueueBlind != "" {
			out = append(out, "queue depth for `"+s.Stage+"` — "+s.QueueBlind)
		}
	}
	for _, cr := range cellRows {
		if cr.FlowBlind != "" {
			out = append(out, cr.Cell+" — "+cr.FlowBlind)
		}
	}
	for _, cr := range cellRows {
		for _, s := range cr.Stages {
			if s.Blind != "" && s.Stage != "review" {
				out = append(out, s.Stage+" — "+s.Blind)
			}
		}
	}
	sort.Strings(out)
	deduped := out[:0]
	var prev string
	for i, v := range out {
		if i == 0 || v != prev {
			deduped = append(deduped, v)
		}
		prev = v
	}
	return deduped
}

// adviceFor is the oracle's inline `if $tp == null then "COULD-NOT-CHECK: ..." else
// $tp.advice end` (assay-inbox.sh:1201-1204) — the one line a coordinator acts on.
func adviceFor(tp *throughputJSON, tperr string) string {
	if tp == nil {
		return "COULD-NOT-CHECK: `deskboard throughput` was not read (" + blindtext(tperr, "no diagnostic") +
			"), so no stage here carries a queue, a slot count or a ratio, and NO bottleneck is named. Blind is not idle."
	}
	return tp.Advice
}

// -------------------------------------------------------------------- terminal render --

// jqNum formats a float64 exactly as jq's tostring does: the shortest decimal that
// round-trips to the same IEEE754 double, no exponent notation, no trailing ".0" for an
// integral value (strconv's 'f'/-1 verb combination matches this for every value the flow
// model computes — see flow_test.go's TestJQNumMatchesRealJQ).
func jqNum(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func nOf(v *int) string {
	if v == nil {
		return "n/a"
	}
	return strconv.Itoa(*v)
}

// rOf is the oracle's r($v): round to 2 decimals (half away from zero, matching jq's round),
// "n/a" when absent.
func rOf(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return jqNum(math.Round(*v*100) / 100)
}

func padStr(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func rowLine(a, b, c, d, e, f, g string) string {
	return "  " + padStr(a, 12) + " " + padStr(b, 7) + " " + padStr(c, 7) + " " + padStr(d, 6) +
		" " + padStr(e, 6) + " " + padStr(f, 9) + " " + g
}

// renderFlowText is the terminal form, ported from render_flow_text (assay-inbox.sh:1236-1284).
func renderFlowText(w io.Writer, m flowModel) {
	fmt.Fprintf(w, "pipeline flow — asOf %s · window %s\n", m.AsOf, m.Since)
	fmt.Fprintln(w)
	for _, row := range m.Rows {
		header := "cell " + row.Cell + " (" + row.Root + ")"
		if row.Fleet {
			header = "FLEET"
		}
		if row.SHA != "" {
			header += "  board " + row.SHA
		}
		fmt.Fprintln(w, "== "+header)
		fmt.Fprintln(w, rowLine("STAGE", "COUNT", "QUEUE", "SLOTS", "RATIO", "DWELL", "NOTE"))
		for _, s := range row.Stages {
			stageCol := s.Stage
			if s.IsBottleneck {
				stageCol += " *"
			}
			dwell := s.Dwell
			if dwell == "" {
				dwell = "-"
			}
			var note string
			switch {
			case s.Count == nil && s.NA != "":
				note = "n/a — " + s.NA
			case s.Count == nil:
				note = "could-not-check: " + s.Blind
			case s.CountPartial:
				note = "AT LEAST — some cells unread"
			case s.QueueBlind != "":
				note = "queue could-not-check (see below)"
			default:
				note = s.CapacityNote
			}
			fmt.Fprintln(w, rowLine(stageCol, nOf(s.Count), nOf(s.Queue), nOf(s.Slots), rOf(s.Ratio), dwell, note))
		}
		if row.CapacityNote != "" {
			fmt.Fprintln(w, "  "+row.CapacityNote)
		}
		fmt.Fprintln(w)
		flowSuffix := ""
		if row.FlowBlind != "" {
			flowSuffix = "   [" + row.FlowBlind + "]"
		}
		fmt.Fprintf(w, "  flow in window: arrivals %s -> ... -> completions %s%s\n",
			nOf(row.Arrivals), nOf(row.Completions), flowSuffix)
		interior := ""
		for _, a := range row.Arrows {
			if a.From == "todo" {
				interior = a.Blind
				break
			}
		}
		fmt.Fprintln(w, "  interior steps: "+interior)
		if row.Constraint == "" {
			fmt.Fprintln(w, "  dwell-weighted constraint (ToC): n/a — "+row.ConstraintSrc)
		} else {
			fmt.Fprintln(w, "  dwell-weighted constraint (ToC): "+row.Constraint)
		}
		fmt.Fprintln(w)
	}
	bneck := "none named"
	if m.Bottleneck != "" {
		bneck = m.Bottleneck + " *"
	}
	fmt.Fprintf(w, "bottleneck (largest queue/slots ratio): %s   read %d of %d stages\n", bneck, m.StagesRead, m.StagesTotal)
	fmt.Fprintln(w, "advice: "+m.Advice)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "sources:")
	for _, s := range m.Sources {
		fmt.Fprintln(w, "  - "+s)
	}
	if len(m.Blind) > 0 {
		fmt.Fprintln(w, "could-not-check:")
		for _, b := range m.Blind {
			fmt.Fprintln(w, "  - "+b)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, m.Note)
}

// ------------------------------------------------------------------------- CLI glue --

var flowSinceRe = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

func flowSummary(cellCount int, flowFailures int) string {
	s := fmt.Sprintf("deskinbox: flow across %d cell(s)", cellCount)
	if flowFailures > 0 {
		s += fmt.Sprintf("; %d reader(s) FAILED — THIS FLOW IS INCOMPLETE (see stderr)", flowFailures)
	}
	return s
}

// countFlowFailures counts how many of the readers behind a raw envelope came back
// could-not-check — the oracle's flow_failures counter (assay-inbox.sh:918-938), recomputed
// from the envelope rather than threaded through as a side channel.
func countFlowFailures(raw rawDoc) int {
	n := 0
	for _, c := range raw.Cells {
		if c.Bottleneck == nil {
			n++
		}
		if c.Intake == nil {
			n++
		}
		if c.NetFlow == nil {
			n++
		}
	}
	if raw.Throughput == nil {
		n++
	}
	return n
}

// runFlow implements `deskinbox flow` (terminal table) and `deskinbox flow --html OUT`
// (the SVG diagram page), ported from the oracle's `flow`/`flowhtml` MODE cases
// (assay-inbox.sh:1385-1402).
func runFlow(stdout, stderr io.Writer, rootArgs []string, since, htmlOut string, now func() string) int {
	if since != "" && !flowSinceRe.MatchString(since) {
		fmt.Fprintf(stderr, "deskinbox: --since must be YYYY-MM-DD (got %q)\n", since)
		return deskkit.ExitRefused
	}
	cwd, cerr := os.Getwd()
	if cerr != nil {
		fmt.Fprintln(stderr, deskkit.Unverifiable("cannot resolve the current directory", cerr))
		return deskkit.ExitUnverifiable
	}
	cells, rerr := resolveCells(rootArgs, cwd)
	if rerr != nil {
		fmt.Fprintln(stderr, rerr)
		return deskkit.ExitCodeOf(rerr)
	}
	if len(cells) == 0 {
		fmt.Fprintln(stderr, "deskinbox: no cells to read (no --root, no ./.assay/cells.txt)")
		return deskkit.ExitRefused
	}

	sgBin, sgErr := resolveFlowBin(statusgenBinEnv, "statusgen")
	if sgErr != nil {
		fmt.Fprintln(stderr, sgErr)
		return deskkit.ExitCodeOf(sgErr)
	}
	dbBin, dbErr := resolveFlowBin(deskboardBinEnv, "deskboard")
	if dbErr != nil {
		fmt.Fprintln(stderr, dbErr)
		return deskkit.ExitCodeOf(dbErr)
	}

	raw := collectFlow(cells, since, sgBin, dbBin, now())
	model := interpretFlow(raw)
	flowFailures := countFlowFailures(raw)

	if htmlOut != "" {
		page := buildFlowOnlyPage(model)
		if err := os.WriteFile(htmlOut, []byte(page), 0o600); err != nil {
			fmt.Fprintf(stderr, "deskinbox: failed to write %s: %v\n", htmlOut, err)
			return deskkit.ExitRefused
		}
		fmt.Fprintf(stdout, "deskinbox: wrote the flow diagram to %s\n", htmlOut)
	} else {
		renderFlowText(stdout, model)
	}
	fmt.Fprintln(stdout, flowSummary(model.CellCount, flowFailures))
	if flowFailures > 0 {
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}
