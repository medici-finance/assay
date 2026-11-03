package web

import (
	"fmt"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// This file derives the numbers the panes render (design §5.2, §7.6). It is a
// RENDERING layer over the derived index and nothing else: generators compute,
// the Console renders, Ask Assay cites. Every DORA/TOC/OODA/escalation figure is
// DELEGATED to statusgen's emitters (tracker's tools/statusgen — done, verified),
// which this build does not ingest, so those figures render an honest
// could-not-check naming their one declared source rather than a fabricated
// number. The index-native figures (open/waiting/oldest) are checked-clean from
// the local table. A tile that cannot be derived is NEVER a zero — spec §10.
//
// pollInterval is the design's 60s index poll (§3.4). "Stale" is past 2× it, at
// which point every staleness treatment fires: the "as of ⟨t⟩" chip greys and
// the strip is marked stale (§10, §3.4). failedToStartN is D3's implementer
// constant, carried here only so the render can name it.
const (
	pollInterval   = 60 * time.Second
	staleAfter     = 2 * pollInterval
	failedToStartN = 300 * time.Second
)

// hardness is the sensor-hardness tag every figure wears (§7.6): ● for a
// platform-timestamp or index-native fact, ◐ for a content-derived one. It never
// stands as colour alone — the glyph IS the encoding.
type hardness string

const (
	hardPlatform hardness = "●" // platform-timestamp / index-native — F-08-resistant
	hardContent  hardness = "◐" // content-derived — F-08-bounded, gameable
)

// tileVM is one number on a pane: its label, the query that produced it
// (data-query), its sensor-hardness (data-hardness), and its three-state value.
// A tile is checked-clean only when the index derived it; everything delegated to
// statusgen renders could-not-check with its declared source, never a zero.
type tileVM struct {
	Key      string
	Label    string
	Query    string // data-query — the scoped desk__metrics_query / delegated source
	Hardness hardness
	OK       bool   // checked-clean: a value may be shown
	Value    string // rendered value, when OK
	Reason   string // could-not-check reason, when !OK
	Tuning   bool   // §5.3 tuning-only instrument (acceptance rate), never an audit figure
	Scope    string
}

// ambientVM is the metrics strip carried in every pane's chrome (§5.2 ambient
// rule, binding). Scope comes from the route context; the numbers are the four
// DORA figures + the current constraint + the waiting-on-you count, computed for
// whatever the pane is showing. It carries the index-age staleness treatment
// (§10) like every other surface.
type ambientVM struct {
	Scope        string
	AsOf         string // RFC3339 of the last successful poll, or "" when never polled
	AsOfHuman    string // compact age, or "never"
	Stale        bool   // index age past 2× the poll interval — grey the chip, mark the strip
	StaleReason  string
	WaitingOnYou tileVM
	Constraint   tileVM
	DORA         []tileVM
}

// metricsSource bundles what the derivations read.
type metricsSource struct {
	rows      []index.Row
	now       time.Time
	indexAge  time.Duration
	ageOK     bool // the index age is known (a poll has completed)
	ageAssess domain.Assessment
}

// newMetricsSource reads the index once for a scope render.
func (s *Server) newMetricsSource() (metricsSource, error) {
	now := s.deps.Now()
	rows, err := s.deps.DB.Inbox("", true) // all cells; scope narrows in the derivations
	if err != nil {
		return metricsSource{}, err
	}
	age, assess := s.deps.DB.IndexAge(now)
	return metricsSource{
		rows:      rows,
		now:       now,
		indexAge:  age,
		ageOK:     assess.Confidence != domain.CouldNotCheck,
		ageAssess: assess,
	}, nil
}

// waitingOnYou counts rows whose derived attention state is `waiting` — the
// human-gate rollup (§1.2). It is index-native and checked-clean.
func waitingOnYou(rows []index.Row) int {
	n := 0
	for _, r := range rows {
		if r.Attention != nil && r.Attention.State == domain.Waiting {
			n++
		}
	}
	return n
}

// oldestAge returns the oldest open-escalation age in seconds (index-native).
func oldestAge(rows []index.Row) int64 {
	var oldest int64
	for _, r := range rows {
		if r.AgeSeconds > oldest {
			oldest = r.AgeSeconds
		}
	}
	return oldest
}

// delegatedTile builds a could-not-check tile for a figure this build does not
// ingest — its one declared source is statusgen's emitter (spec §5.2). This is
// the honest render of "the pane consumes tracker's output": until that substrate is
// ingested the figure is could-not-check, never a zero and never a guess.
func delegatedTile(key, label, statusgenSource string, h hardness, scope string) tileVM {
	return tileVM{
		Key:      key,
		Label:    label,
		Query:    "statusgen " + statusgenSource + " (delegated) @ scope=" + scope,
		Hardness: h,
		OK:       false,
		Reason: "delegated to statusgen's emitter and not ingested in this build — " +
			"the Console renders scoped metrics from statusgen rather than computing a rival roll-up (§5.2 single-writer)",
		Scope: scope,
	}
}

// indexTile builds a checked-clean index-native tile.
func indexTile(key, label, metric string, h hardness, scope, value string) tileVM {
	return tileVM{
		Key:      key,
		Label:    label,
		Query:    fmt.Sprintf("desk__metrics_query{scope=%s,metric=%s}", scope, metric),
		Hardness: h,
		OK:       true,
		Value:    value,
		Scope:    scope,
	}
}

// ambientFor builds the metrics strip for a scope. The DORA four and the
// constraint are delegated (could-not-check); the waiting-on-you count is
// index-native (checked-clean). Staleness applies per §10.
func ambientFor(src metricsSource, scope string) ambientVM {
	a := ambientVM{Scope: scope}

	// Staleness (§10, §3.4): grey the chip past 2× the poll interval; a never-
	// polled index has an UNKNOWN age, which is stale-as-a-question, never fresh.
	if !src.ageOK {
		a.AsOfHuman = "never"
		a.Stale = true
		a.StaleReason = src.ageAssess.Reason
	} else {
		a.AsOf = src.now.Add(-src.indexAge).Format(time.RFC3339)
		a.AsOfHuman = humanAge(int64(src.indexAge.Seconds()))
		if src.indexAge > staleAfter {
			a.Stale = true
			a.StaleReason = fmt.Sprintf("index age %s exceeds 2× the %s poll interval",
				humanAge(int64(src.indexAge.Seconds())), pollInterval)
		}
	}

	a.WaitingOnYou = indexTile("waiting_on_you", "waiting on you", "waiting_on_you",
		hardPlatform, scope, fmt.Sprintf("%d", waitingOnYou(src.rows)))
	a.Constraint = delegatedTile("constraint", "constraint (deepest queue)", "--trend / TOC stage waits", hardPlatform, scope)
	a.DORA = []tileVM{
		delegatedTile("verified_throughput", "verified throughput /wk", "--dora deployment-frequency", hardPlatform, scope),
		delegatedTile("lead_time", "lead time (claim→verified)", "--dora lead-time", hardPlatform, scope),
		delegatedTile("change_failure_rate", "change failure rate", "--dora change-failure-rate", hardContent, scope),
		delegatedTile("mttr", "MTTR (finding→resolved)", "--dora mttr", hardPlatform, scope),
	}
	return a
}

// measuresVM is the Measures pane — where a number decomposes on click (§5.2,
// §7.3 pane 5). Every tile carries its query chip and hardness tag; the two new
// decision-first tiles (acceptance rate ◐ tuning-only, time-to-decision ●) and
// dwell-after-look land here (spec §3.1.1).
type measuresVM struct {
	Ambient    ambientVM
	IndexTiles []tileVM // the index-native figures this build DOES derive
	DORA       []tileVM
	TOC        []tileVM
	OODA       []tileVM
	Escalation []tileVM
}

// measuresFor builds the full Measures decomposition for a scope.
func measuresFor(src metricsSource, scope string) measuresVM {
	m := measuresVM{Ambient: ambientFor(src, scope)}

	// Index-native, checked-clean — the figures that prove the pane is live.
	m.IndexTiles = []tileVM{
		indexTile("open_escalations", "open escalations", "open_escalations", hardPlatform, scope,
			fmt.Sprintf("%d", len(src.rows))),
		indexTile("waiting_on_you", "waiting on you", "waiting_on_you", hardPlatform, scope,
			fmt.Sprintf("%d", waitingOnYou(src.rows))),
		indexTile("oldest_open_age", "oldest open age", "oldest_age_seconds", hardPlatform, scope,
			humanAge(oldestAge(src.rows))),
	}

	// DORA (§5.2) — delegated to statusgen's --dora emitter.
	m.DORA = []tileVM{
		delegatedTile("verified_throughput", "verified throughput /wk", "--dora deployment-frequency", hardPlatform, scope),
		delegatedTile("lead_time", "lead time (claim→verified) p50", "--dora lead-time", hardPlatform, scope),
		delegatedTile("change_failure_rate", "change failure rate", "--dora change-failure-rate", hardContent, scope),
		delegatedTile("mttr", "MTTR (finding→resolved)", "--dora mttr", hardPlatform, scope),
	}

	// TOC — name the constraint; median wait per ring stage (§5.2).
	m.TOC = []tileVM{
		delegatedTile("toc_constraint", "deepest queue (the constraint)", "--trend TOC stage waits", hardPlatform, scope),
		delegatedTile("flow_efficiency", "flow efficiency (active/elapsed)", "--trend flow-efficiency", hardPlatform, scope),
	}

	// OODA — decompose the decision loop; decide is the dominant term (§5.2).
	m.OODA = []tileVM{
		delegatedTile("ooda_observe", "observe (filed→indexed)", "--trend ooda-observe", hardPlatform, scope),
		delegatedTile("ooda_orient", "orient (indexed→seen)", "--trend ooda-orient", hardPlatform, scope),
		delegatedTile("ooda_decide", "decide (seen→answered) — dominant", "--trend ooda-decide", hardPlatform, scope),
		delegatedTile("ooda_act", "act (answered→resumed)", "--trend ooda-act", hardPlatform, scope),
	}

	// Escalation health (§3.1.1) — the two new decision-first tiles + dwell.
	accept := delegatedTile("acceptance_rate", "escalation acceptance rate", "--dora escalation-acceptance", hardContent, scope)
	accept.Tuning = true // ◐, tuning-only: default_id is agent-declared and gameable (§3.1.1, §5.3)
	accept.Reason = "delegated to statusgen and not ingested; and by §3.1.1 a TUNING instrument only — " +
		"default_id is agent-declared (◐), so acceptance rate is never an audit figure"
	m.Escalation = []tileVM{
		accept,
		delegatedTile("time_to_decision_p50", "time-to-decision p50 (both ● — faces an auditor)", "--dora time-to-decision", hardPlatform, scope),
		delegatedTile("dwell_after_look", "dwell-after-look (the diagnostic hop)", "--dora dwell", hardPlatform, scope),
	}
	return m
}
