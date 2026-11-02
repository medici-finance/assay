package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// `deskperf reruns` (issue #4163): verification re-runs as waste. The run counts come
// from the verify-outcome register (every record of a brief, each re-validated with
// deskkit.CheckAttempts — the reader half of the attempt validator). The cost is NOT a new
// measure: each attempt window is joined to the workflow perf rows deskperf report already
// reads and summed with the same perfrec reducers. A measure that is pruned, missing or
// ambiguous is a stated state with its reason, never a zero and never apportioned.

const (
	stateMeasured      = "measured"
	stateCouldNotCheck = "could-not-check"
	stateAmbiguous     = "ambiguous"

	tokensReason = "token usage is recorded for worker dispatches only"
	notCounted   = "verifier dispatches that landed no outcome record are not counted: the register is the only source of runs"
)

var rerunsBriefKey = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*)/([0-9]+)$`)

// briefFrontmatterID reads the frontmatter `brief:` line of a brief file.
var briefFrontmatterID = regexp.MustCompile(`(?m)^brief:[ \t]*['"]?([^'"\s]+)['"]?[ \t]*$`)

// stateJSON is a three-state measure that is not a number: could-not-check or ambiguous, with
// its reason. A measure the report could not take is this object, never a numeric 0.
type stateJSON struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

// measure is a number when measured, else the stateJSON saying why not.
func measure(state, reason string, v int64) any {
	if state == stateMeasured {
		return v
	}
	return stateJSON{State: state, Reason: reason}
}

// rerunAttempt is one verification run with its joined cost.
type rerunAttempt struct {
	run         deskkit.AttemptRun
	state, why  string
	rows        int
	agents      int
	wallMs      int64
	cpuMs       int64
	cpuRecords  int
	requestAtts int64
}

// costSide is the total or the uncredited-plus-discarded part.
type costSide struct {
	runs, measured int
	wallMs, cpuMs  int64
	cpuRecords     int
	requestAtts    int64
	gaps           []string
}

type rerunsAttemptJSON struct {
	Run                 int    `json:"run"`
	Source              string `json:"source"`
	Attempt             int    `json:"attempt,omitempty"`
	Disposition         string `json:"disposition"`
	Reason              string `json:"reason,omitempty"`
	Started             string `json:"started,omitempty"`
	Finished            string `json:"finished,omitempty"`
	State               string `json:"state"`
	StateReason         string `json:"stateReason,omitempty"`
	Rows                any    `json:"rows"`
	SumOwnProcessWallMs any    `json:"sumOwnProcessWallMs"`
	SumOwnCPUMs         any    `json:"sumOwnCpuMs"`
	CPURecords          any    `json:"cpuRecords"`
	RequestAttempts     any    `json:"requestAttempts"`
}

type rerunsSideJSON struct {
	Runs                int `json:"runs"`
	Measured            int `json:"measured"`
	SumOwnProcessWallMs any `json:"sumOwnProcessWallMs"`
	SumOwnCPUMs         any `json:"sumOwnCpuMs"`
	CPURecords          any `json:"cpuRecords"`
	RequestAttempts     any `json:"requestAttempts"`
}

type rerunsInvalidJSON struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type rerunsCountJSON struct {
	Brief         string         `json:"brief"`
	Runs          int            `json:"runs"`
	ByDisposition map[string]int `json:"byDisposition"`
	ByReason      map[string]int `json:"byReason"`
}

type rerunsBriefJSON struct {
	rerunsCountJSON
	Days           int                 `json:"days"`
	WorkRef        perfrec.WorkRef     `json:"workRef"`
	Attempts       []rerunsAttemptJSON `json:"attempts"`
	Total          rerunsSideJSON      `json:"total"`
	Waste          rerunsSideJSON      `json:"uncreditedPlusDiscarded"`
	Share          any                 `json:"share"`
	Unassigned     map[string]any      `json:"unassigned"`
	VerifierTokens stateJSON           `json:"verifierTokens"`
	NotCounted     string              `json:"notCounted"`
	Invalid        []rerunsInvalidJSON `json:"invalid"`
}

type rerunsAllJSON struct {
	Briefs     []rerunsCountJSON   `json:"briefs"`
	Invalid    []rerunsInvalidJSON `json:"invalid"`
	NotCounted string              `json:"notCounted"`
}

func cmdReruns(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("deskperf reruns", flag.ContinueOnError)
	root := fs.String("root", "", "the repository checkout whose verify-outcome register is read (required)")
	brief := fs.String("brief", "", "one brief, <stream>/<NN>: counts plus the cost block")
	days := fs.Int("days", deskkit.PerfMaxDays, "UTC days of perf history to join (1-7)")
	asJSON := fs.Bool("json", false, "emit one JSON object with three-state values")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *root == "" {
		return deskkit.Refused("--root is required: the checkout whose verify-outcome register is read")
	}
	if err := window(*days); err != nil {
		return err
	}
	if *brief != "" && !rerunsBriefKey.MatchString(*brief) {
		return deskkit.Refused(fmt.Sprintf("--brief %q: want <stream>/<NN>", *brief))
	}
	records, err := deskkit.ReadVerifyOutcomes(*root)
	if err != nil {
		return deskkit.Unverifiable("cannot read the verify-outcome register under "+*root, err)
	}
	// The reader re-validates EVERY stored record with the writer's own pure check, so a record
	// that bypassed the writer (hand-landed, or written before a rule existed) is listed invalid
	// with its path and never counted.
	byBrief := map[string][]deskkit.OutcomeRecord{}
	var invalid []rerunsInvalidJSON
	for _, rec := range records {
		if *brief != "" && rec.Brief != *brief {
			continue
		}
		if err := storedAttemptsValid(rec); err != nil {
			invalid = append(invalid, rerunsInvalidJSON{Path: sourcePath(*root, rec.Source), Reason: err.Error()})
			continue
		}
		byBrief[rec.Brief] = append(byBrief[rec.Brief], rec)
	}
	if *brief == "" {
		return rerunsAll(byBrief, invalid, *asJSON, stdout)
	}
	if len(byBrief[*brief]) == 0 && len(invalid) == 0 {
		return deskkit.Unverifiable(fmt.Sprintf("the verify-outcome register under %s has no record for %s", *root, *brief), nil)
	}
	ref, err := rerunsWorkRef(*root, *brief)
	if err != nil {
		return err
	}
	rollup := deskkit.RollupAttempts(*brief, byBrief[*brief])
	for _, u := range rollup.Undecodable {
		invalid = append(invalid, rerunsInvalidJSON{Path: sourcePath(*root, u), Reason: "attempts could not be decoded"})
	}
	h, err := deskkit.LoadPerfHistory(*days)
	if err != nil {
		return deskkit.Unverifiable("cannot read the perf history directory", err)
	}
	attempts, unassigned, err := joinAttempts(rollup, h, ref, *days)
	if err != nil {
		return err
	}
	noPerf := ""
	if len(h.Records) == 0 {
		noPerf = "no perf rows: " + emptyHistoryReason(h, *days)
	}
	return rerunsBrief(rollup, attempts, unassigned, ref, invalid, noPerf, *days, *asJSON, stdout)
}

// storedAttemptsValid runs the writer's attempt validator over one stored record.
func storedAttemptsValid(rec deskkit.OutcomeRecord) error {
	var f map[string]json.RawMessage
	if err := json.Unmarshal(rec.Raw, &f); err != nil {
		return err
	}
	_, err := deskkit.CheckAttempts(f)
	return err
}

// sourcePath renders a record's Source ("record:<path>" / "legacy:<path>") relative to root.
func sourcePath(root, source string) string {
	p := source
	if _, rest, ok := strings.Cut(source, ":"); ok {
		p = rest
	}
	if rel, err := filepath.Rel(root, filepath.FromSlash(p)); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return p
}

// rerunsWorkRef is the join key: kind brief, repo = the root's origin, brief = PerfBriefRef of
// the brief file's frontmatter id. Any miss is unverifiable (exit 6) naming the path or value.
func rerunsWorkRef(root, brief string) (perfrec.WorkRef, error) {
	m := rerunsBriefKey.FindStringSubmatch(brief)
	pattern := filepath.Join(root, "docs", "streams", m[1], "brief-"+m[2]+"-*.md")
	files, _ := filepath.Glob(pattern)
	if len(files) != 1 {
		return perfrec.WorkRef{}, deskkit.Unverifiable(fmt.Sprintf("brief file for %s: want exactly one %s, found %d",
			brief, filepath.ToSlash(pattern), len(files)), nil)
	}
	raw, err := os.ReadFile(files[0])
	if err != nil {
		return perfrec.WorkRef{}, deskkit.Unverifiable("cannot read brief file "+files[0], err)
	}
	fm := briefFrontmatterID.FindStringSubmatch(briefdoc.Frontmatter(string(raw)))
	if fm == nil {
		return perfrec.WorkRef{}, deskkit.Unverifiable("brief file "+files[0]+" has no frontmatter brief: id", nil)
	}
	id, err := deskkit.PerfBriefRef(fm[1])
	if err != nil {
		return perfrec.WorkRef{}, deskkit.Unverifiable("brief file "+files[0]+": "+err.Error(), nil)
	}
	repo, err := deskkit.OriginRepoSlug(root)
	if err != nil {
		return perfrec.WorkRef{}, deskkit.Unverifiable("cannot read the origin repository of "+root, err)
	}
	return perfrec.WorkRef{Kind: "brief", Repo: repo, Brief: id}, nil
}

// workJoins compares exactly the three join fields; briefUuid, primary, forge and instance are
// ignored, so this is deliberately not perfrec's whole-struct work equality.
func workJoins(d perfrec.Attribution, ref perfrec.WorkRef) bool {
	for _, w := range d.Work {
		if w.Kind == "brief" && w.Repo == ref.Repo && w.Brief == ref.Brief {
			return true
		}
	}
	return false
}

type joinedRow struct {
	rec perfrec.Record
	at  time.Time
}

// joinAttempts gives every run its cost state and returns the matching rows outside every
// attempt window. A row inside two adjacent windows (one's finished == the next's started)
// belongs to the earlier attempt only, so no row is counted twice.
func joinAttempts(rollup deskkit.AttemptRollup, h deskkit.PerfHistory, ref perfrec.WorkRef, days int) ([]rerunAttempt, []perfrec.Record, error) {
	kept, _, err := perfrec.SelectV3(h.Records, perfrec.ProfileWorkflow, perfrec.V3Filter{Role: "verify"})
	if err != nil {
		return nil, nil, deskkit.Refused(err.Error())
	}
	var rows []joinedRow
	for _, r := range kept {
		if !workJoins(r.V3.Attribution, ref) {
			continue
		}
		at, perr := time.Parse(time.RFC3339, r.TS)
		if perr != nil {
			continue
		}
		rows = append(rows, joinedRow{rec: r, at: at.UTC()})
	}
	now := time.Now().UTC()
	oldest := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(days - 1))
	used := make([]bool, len(rows))
	refJSON, _ := json.Marshal(ref)
	out := make([]rerunAttempt, 0, len(rollup.Attempts))
	for _, run := range rollup.Attempts {
		a := rerunAttempt{run: run}
		var mine []perfrec.Record
		agents := map[string]bool{}
		if run.HasWindow {
			for i, row := range rows {
				if used[i] || row.at.Before(run.Started) || row.at.After(run.Finished) {
					continue
				}
				used[i] = true
				mine = append(mine, row.rec)
				agents[row.rec.V3.Attribution.AgentID] = true
			}
		}
		a.rows, a.agents = len(mine), len(agents)
		switch {
		case !run.HasWindow:
			a.state, a.why = stateCouldNotCheck, "no window: the record carries no started/finished for this run"
		case run.Started.Before(oldest):
			a.state, a.why = stateCouldNotCheck, fmt.Sprintf("pruned: the window starts before the oldest perf day read (%s, --days %d; the history keeps %d UTC days)",
				oldest.Format("2006-01-02"), days, deskkit.PerfMaxDays)
		case len(h.Records) == 0:
			a.state, a.why = stateCouldNotCheck, "no perf rows: "+emptyHistoryReason(h, days)
		case len(mine) == 0:
			a.state, a.why = stateCouldNotCheck, fmt.Sprintf("no perf rows: no workflow perf row with role verify and work %s in the window "+
				"(the verifier dispatch records it only when given --perf-work)", refJSON)
		case len(agents) > 1:
			a.state, a.why = stateAmbiguous, fmt.Sprintf("%d distinct agentIds in one window; excluded from both sums, never split", len(agents))
		default:
			a.state = stateMeasured
			v3, rerr := perfrec.ReduceV3(mine, perfrec.ProfileWorkflow, "role", perfrec.V3Filter{Role: "verify"})
			if rerr != nil {
				return nil, nil, deskkit.Refused(rerr.Error())
			}
			for _, g := range v3.Groups {
				a.wallMs += g.SumOwnProcessWallMs
				a.cpuMs += g.SumOwnCPUMs
				a.cpuRecords += g.CPURecords
			}
			for _, r := range v3.Requests {
				a.requestAtts += r.Attempts
			}
		}
		out = append(out, a)
	}
	var unassigned []perfrec.Record
	for i, row := range rows {
		if !used[i] {
			unassigned = append(unassigned, row.rec)
		}
	}
	return out, unassigned, nil
}

// noCPU is the reason an own-CPU sum is not a number: none of the summed rows carries a CPU
// measure (the platform had no CPU source, or every read failed), so 0 would be a fabrication.
const noCPU = "no summed row carries a CPU measure"

// cpuMeasure is the own-CPU sum when at least one summed row carried a CPU measure, else the
// could-not-check state.
func cpuMeasure(cpuMs int64, cpuRecords int) any {
	if cpuRecords == 0 {
		return stateJSON{State: stateCouldNotCheck, Reason: noCPU}
	}
	return cpuMs
}

func cpuText(cpuMs int64, cpuRecords int) string {
	if cpuRecords == 0 {
		return "own CPU " + stateCouldNotCheck + " (" + noCPU + ")"
	}
	return fmt.Sprintf("own CPU %d ms (%d records)", cpuMs, cpuRecords)
}

func wasted(disposition string) bool {
	return disposition == deskkit.AttemptUncredited || disposition == deskkit.AttemptDiscarded
}

func (s *costSide) add(a rerunAttempt, label string) {
	s.runs++
	if a.state != stateMeasured {
		s.gaps = append(s.gaps, label+" "+a.state)
		return
	}
	s.measured++
	s.wallMs += a.wallMs
	s.cpuMs += a.cpuMs
	s.cpuRecords += a.cpuRecords
	s.requestAtts += a.requestAtts
}

// state of a side: measured when at least one of its runs is (the sum then covers those runs,
// and the line says how many); could-not-check when it has runs and none is measured. A side
// with no runs at all is a measured, genuine 0.
func (s costSide) state() (string, string) {
	if s.runs > 0 && s.measured == 0 {
		return stateCouldNotCheck, "no run on this side has a measured cost: " + strings.Join(s.gaps, ", ")
	}
	return stateMeasured, ""
}

func (s costSide) json() rerunsSideJSON {
	st, why := s.state()
	cpu := measure(st, why, s.cpuMs)
	if st == stateMeasured {
		cpu = cpuMeasure(s.cpuMs, s.cpuRecords)
	}
	return rerunsSideJSON{Runs: s.runs, Measured: s.measured,
		SumOwnProcessWallMs: measure(st, why, s.wallMs), SumOwnCPUMs: cpu,
		CPURecords: measure(st, why, int64(s.cpuRecords)), RequestAttempts: measure(st, why, s.requestAtts)}
}

func (s costSide) text() string {
	if st, why := s.state(); st != stateMeasured {
		return st + " (" + why + ")"
	}
	return fmt.Sprintf("own-process wall %d ms; %s; request attempts %d — %d of %d run(s) measured",
		s.wallMs, cpuText(s.cpuMs, s.cpuRecords), s.requestAtts, s.measured, s.runs)
}

func runLabel(i int, a rerunAttempt) string {
	if a.run.Attempt == 0 {
		return fmt.Sprintf("run %d", i+1)
	}
	return fmt.Sprintf("run %d (attempt %d)", i+1, a.run.Attempt)
}

func countLine(r deskkit.AttemptRollup) string {
	return fmt.Sprintf("%s: %d verification runs, %d uncredited, %d discarded, %d unrecorded", r.Brief, r.Runs,
		r.ByDisposition[deskkit.AttemptUncredited], r.ByDisposition[deskkit.AttemptDiscarded], r.ByDisposition[deskkit.AttemptUnrecorded])
}

func countJSON(r deskkit.AttemptRollup) rerunsCountJSON {
	return rerunsCountJSON{Brief: r.Brief, Runs: r.Runs, ByDisposition: r.ByDisposition, ByReason: r.ByReason}
}

func rerunsAll(byBrief map[string][]deskkit.OutcomeRecord, invalid []rerunsInvalidJSON, asJSON bool, stdout io.Writer) error {
	var rollups []deskkit.AttemptRollup
	for b, recs := range byBrief {
		rollups = append(rollups, deskkit.RollupAttempts(b, recs))
	}
	sort.Slice(rollups, func(i, j int) bool {
		if rollups[i].Runs != rollups[j].Runs {
			return rollups[i].Runs > rollups[j].Runs
		}
		return rollups[i].Brief < rollups[j].Brief
	})
	if asJSON {
		out := rerunsAllJSON{Briefs: []rerunsCountJSON{}, Invalid: invalid, NotCounted: notCounted}
		if out.Invalid == nil {
			out.Invalid = []rerunsInvalidJSON{}
		}
		for _, r := range rollups {
			out.Briefs = append(out.Briefs, countJSON(r))
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return encodeErr(enc.Encode(out))
	}
	for _, r := range rollups {
		fmt.Fprintln(stdout, countLine(r))
	}
	for _, iv := range invalid {
		fmt.Fprintf(stdout, "invalid %s: %s\n", iv.Path, iv.Reason)
	}
	fmt.Fprintln(stdout, "not counted: "+notCounted)
	return nil
}

func rerunsBrief(r deskkit.AttemptRollup, attempts []rerunAttempt, unassigned []perfrec.Record, ref perfrec.WorkRef,
	invalid []rerunsInvalidJSON, noPerf string, days int, asJSON bool, stdout io.Writer) error {
	var total, waste costSide
	for i, a := range attempts {
		label := runLabel(i, a)
		total.add(a, label)
		if wasted(a.run.Disposition) {
			waste.add(a, label)
		}
	}
	shareState, shareWhy, sharePct := stateMeasured, "", 0.0
	if tst, _ := total.state(); tst != stateMeasured || total.measured == 0 {
		shareState, shareWhy = stateCouldNotCheck, "no measured total"
	} else if wst, _ := waste.state(); wst != stateMeasured {
		shareState, shareWhy = stateCouldNotCheck, "the uncredited-plus-discarded part has no measured run"
	} else if total.wallMs == 0 {
		shareState, shareWhy = stateCouldNotCheck, "the measured total is 0 ms"
	} else {
		sharePct = 100 * float64(waste.wallMs) / float64(total.wallMs)
	}
	// Unassigned rows are measured only when there was a history to look in.
	unState, unWhy := stateMeasured, ""
	if noPerf != "" {
		unState, unWhy = stateCouldNotCheck, noPerf
	}
	var unWall int64
	if len(unassigned) > 0 {
		v3, err := perfrec.ReduceV3(unassigned, perfrec.ProfileWorkflow, "role", perfrec.V3Filter{Role: "verify"})
		if err != nil {
			return deskkit.Refused(err.Error())
		}
		for _, g := range v3.Groups {
			unWall += g.SumOwnProcessWallMs
		}
	}

	if asJSON {
		out := rerunsBriefJSON{rerunsCountJSON: countJSON(r), Days: days, WorkRef: ref, Attempts: []rerunsAttemptJSON{},
			Total: total.json(), Waste: waste.json(),
			Unassigned: map[string]any{"rows": measure(unState, unWhy, int64(len(unassigned))),
				"sumOwnProcessWallMs": measure(unState, unWhy, unWall)},
			VerifierTokens: stateJSON{State: stateCouldNotCheck, Reason: tokensReason}, NotCounted: notCounted, Invalid: invalid}
		if out.Invalid == nil {
			out.Invalid = []rerunsInvalidJSON{}
		}
		if shareState == stateMeasured {
			out.Share = strconv.FormatFloat(sharePct, 'f', 1, 64) + "%"
		} else {
			out.Share = stateJSON{State: shareState, Reason: shareWhy}
		}
		for i, a := range attempts {
			aj := rerunsAttemptJSON{Run: i + 1, Source: a.run.Source, Attempt: a.run.Attempt, Disposition: a.run.Disposition,
				Reason: a.run.Reason, State: a.state, StateReason: a.why,
				Rows: measure(a.state, a.why, int64(a.rows)), SumOwnProcessWallMs: measure(a.state, a.why, a.wallMs),
				SumOwnCPUMs: measure(a.state, a.why, a.cpuMs), CPURecords: measure(a.state, a.why, int64(a.cpuRecords)),
				RequestAttempts: measure(a.state, a.why, a.requestAtts)}
			if a.state == stateMeasured {
				aj.SumOwnCPUMs = cpuMeasure(a.cpuMs, a.cpuRecords)
			}
			if a.run.HasWindow {
				aj.Started, aj.Finished = a.run.Started.Format(time.RFC3339), a.run.Finished.Format(time.RFC3339)
			}
			out.Attempts = append(out.Attempts, aj)
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return encodeErr(enc.Encode(out))
	}

	fmt.Fprintln(stdout, countLine(r))
	if len(r.ByReason) > 0 {
		var parts []string
		for k, n := range r.ByReason {
			parts = append(parts, fmt.Sprintf("%s %d", k, n))
		}
		sort.Strings(parts)
		fmt.Fprintln(stdout, "reasons: "+strings.Join(parts, ", "))
	}
	refJSON, _ := json.Marshal(ref)
	fmt.Fprintf(stdout, "cost — workflow perf rows, role verify, work %s, last %d UTC day(s):\n", refJSON, days)
	for i, a := range attempts {
		what := a.run.Disposition
		if a.run.Reason != "" {
			what += " " + a.run.Reason
		}
		win := "no window"
		if a.run.HasWindow {
			win = a.run.Started.Format(time.RFC3339) + " to " + a.run.Finished.Format(time.RFC3339)
		}
		cost := a.state + " (" + a.why + ")"
		if a.state == stateMeasured {
			cost = fmt.Sprintf("own-process wall %d ms; %s; request attempts %d; %d row(s)",
				a.wallMs, cpuText(a.cpuMs, a.cpuRecords), a.requestAtts, a.rows)
		}
		fmt.Fprintf(stdout, "  %s %s [%s]: %s\n", runLabel(i, a), what, win, cost)
	}
	fmt.Fprintln(stdout, "total: "+total.text())
	fmt.Fprintln(stdout, "uncredited+discarded: "+waste.text())
	if shareState == stateMeasured {
		fmt.Fprintf(stdout, "share: %s%% of the measured total own-process wall\n", strconv.FormatFloat(sharePct, 'f', 1, 64))
	} else {
		fmt.Fprintf(stdout, "share: %s (%s)\n", shareState, shareWhy)
	}
	if unState == stateMeasured {
		fmt.Fprintf(stdout, "unassigned: %d row(s), own-process wall %d ms — rows for this brief outside every attempt window, in neither sum\n",
			len(unassigned), unWall)
	} else {
		fmt.Fprintf(stdout, "unassigned: %s (%s)\n", unState, unWhy)
	}
	fmt.Fprintf(stdout, "verifier tokens: %s (%s)\n", stateCouldNotCheck, tokensReason)
	for _, iv := range invalid {
		fmt.Fprintf(stdout, "invalid %s: %s\n", iv.Path, iv.Reason)
	}
	fmt.Fprintln(stdout, "not counted: "+notCounted)
	return nil
}
