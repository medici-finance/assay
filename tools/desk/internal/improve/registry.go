package improve

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/askassay"
)

// THE STRIP REGISTRY
// ------------------
// Four strips, four declared sources. A strip that is not here has no source,
// and a row set with no source does not render — so the way to make the pane
// show something new is to declare where it comes from, not to hand it a
// slice.
//
// Three of the four read a tool that DOES NOT EXIST YET. That is stated as
// data (see [StripDef.RequiresBrief]) rather than as a comment, because the
// difference between "this strip is empty" and "this strip has no source yet"
// is the entire subject of this package.

// The standing caveats for collections. Each is a measured property of this
// system, not a disclaimer.
const (
	// CaveatEmptyStripIsBlind — the collection form of "an empty result is
	// blind, not idle".
	CaveatEmptyStripIsBlind = "an empty strip is BLIND, not idle — a throttled, refused or unlanded source returns the same nothing as a genuinely empty one, so this strip renders could-not-check unless its definition states in writing why an empty read from ITS source is a real emptiness"

	// CaveatTaxonomyUnhardened — design doc §13.5b, recorded as open: the
	// good/bad/ugly taxonomy has not yet been exercised by a cadence of real
	// reports, and who assigns the class is an unruled question.
	CaveatTaxonomyUnhardened = "the good/bad/ugly class on these rows is DATA, not a settled fact — the taxonomy has not been hardened against a cadence of real reports and who assigns the class (generator, analysis automation, or filer) is an open ruling. The class is disputable like any other field, and this strip renders it rather than freezing it"

	// CaveatAdoptionIsHuman — §6.2, deck slide 10. The pane may show that
	// three reports cluster into one systemic issue and may carry a drafted
	// proposal; a retro action exists only when the human commits it.
	CaveatAdoptionIsHuman = "adoption is a GOLD GATE — this strip can show a cluster, and a draft proposal can be attached to it, but a retro action exists only when the human commits it, one per cadence. The adopt element on these rows routes to the human commit path and writes no register"

	// CaveatRegistersAppendOnly — INTAKE and RETRO keep their existing
	// append-only lint. The pane renders them and never writes around them.
	CaveatRegistersAppendOnly = "the intake and retro registers are append-only with their own lint — this strip RENDERS them and has no path that writes one, so a row shown here is a read of a register, never a shadow copy the pane maintains"

	// CaveatUnwiredMetricIsNotNoMovement — the measurement that judges the
	// change has to exist before it can judge. Measured 2026-08-13: of five
	// emitted DORA metrics, one was fully computed, one was computed but
	// partial, and three read unknown.
	CaveatUnwiredMetricIsNotNoMovement = "a target metric that cannot be read renders UNDETERMINED, never no-movement — the two look identical on a screen and mean opposite things. Measured on this repo: of five emitted flow metrics, one was fully computed, one was computed but flagged partial in its own value string, and three read unknown. A verdict here is only as good as the metric behind it"

	// CaveatEvidenceLinkNotEvidence — the presence of a link is not the
	// checking of it.
	CaveatEvidenceLinkNotEvidence = "an evidence link on a row is a POINTER, not a check — this strip requires every report to carry at least one, and refuses to render one that does not, but it does not follow the link or judge what is at the other end"
)

// requiredCaveats is the per-strip floor. A strip WITHOUT its caveats is a
// registry error, enforced by TestImprovePaneEveryStripCarriesItsCaveats.
var requiredCaveats = map[StripID][]string{
	StripReports:    {CaveatEmptyStripIsBlind, CaveatTaxonomyUnhardened, CaveatEvidenceLinkNotEvidence},
	StripClusters:   {CaveatEmptyStripIsBlind, CaveatTaxonomyUnhardened},
	StripRetroQueue: {CaveatEmptyStripIsBlind, CaveatAdoptionIsHuman, CaveatRegistersAppendOnly},
	StripDidItWork:  {CaveatEmptyStripIsBlind, CaveatUnwiredMetricIsNotNoMovement, CaveatAdoptionIsHuman},
}

// StripDef declares one strip: what it shows, where the rows come from, and
// which cross-stream brief has to have landed for that source to exist.
type StripDef struct {
	// ID is the registry key and the token that appears in a rendered strip.
	ID StripID
	// Title is the operator-facing strip name (§7.3 — named, never numbered).
	Title string
	// Source is the ONE declared origin of this strip's rows.
	Source askassay.Source

	// RequiresBrief names the cross-stream brief whose deliverable IS this
	// strip's source. Empty means the source exists in this tree today.
	//
	// This is the anti-mock. [Derive] refuses a strip whose RequiresBrief is
	// set unless the caller supplies a [SubstrateReport] measuring that brief
	// at a landed status — so a fixture cannot stand in for the tool.
	RequiresBrief string
	// RequiresWhy says what that brief actually delivers, so a reader does not
	// have to open the other stream to know what is missing.
	RequiresWhy string

	// SaturatesAt is the row cap at which this strip's list stops being a
	// list. 0 means the source declares no row cap — a claim the Limit field
	// has to justify in words.
	SaturatesAt int

	// EmptyMeansEmpty declares that an empty read from THIS source is a
	// genuine emptiness. The default is false, because an empty strip is
	// blind, not idle. Setting it true requires EmptyRationale.
	EmptyMeansEmpty bool
	// EmptyRationale is why an empty read here is trustworthy.
	EmptyRationale string

	// Caveats are qualifications specific to this strip, on top of the floor.
	Caveats []string
}

// allCaveats returns the strip's DECLARED caveats, deduplicated.
//
// It deliberately does NOT inject the class floor. Injecting it would make
// [StripDef.Validate]'s floor check vacuous — the check would be testing a set
// it had just filled in — and a check that cannot fail is worse than no check,
// because it reads as enforcement. The floor is enforced by refusing a
// definition that does not spell it out, so a new strip cannot be added
// without its qualifications and a removed one reddens.
func (d StripDef) allCaveats() []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range d.Caveats {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

// Validate reports why a strip definition cannot back a render.
func (d StripDef) Validate() error {
	if strings.TrimSpace(string(d.ID)) == "" {
		return errors.New("strip has no ID")
	}
	if _, ok := requiredCaveats[d.ID]; !ok {
		return fmt.Errorf("%s: not a declared strip", d.ID)
	}
	if strings.TrimSpace(d.Title) == "" {
		return fmt.Errorf("%s: strip has no operator-facing title — panes and strips are named, never numbered", d.ID)
	}
	if err := d.Source.Validate(); err != nil {
		return fmt.Errorf("%s: %w", d.ID, err)
	}
	if d.RequiresBrief != "" && strings.TrimSpace(d.RequiresWhy) == "" {
		return fmt.Errorf("%s: names a required brief but does not say what it delivers", d.ID)
	}
	if d.EmptyMeansEmpty && strings.TrimSpace(d.EmptyRationale) == "" {
		return fmt.Errorf("%s: declares an empty read to be a real emptiness but gives no rationale — an empty strip is blind unless something says why it is not", d.ID)
	}
	if !d.EmptyMeansEmpty && strings.TrimSpace(d.EmptyRationale) != "" {
		return fmt.Errorf("%s: carries an empty-read rationale but does not declare EmptyMeansEmpty", d.ID)
	}
	have := map[string]bool{}
	for _, c := range d.allCaveats() {
		have[c] = true
	}
	for _, c := range requiredCaveats[d.ID] {
		if !have[c] {
			return fmt.Errorf("%s: strip requires a caveat it does not carry — the floor is declared per strip and must be written out, so that adding a strip without its qualifications is a refusal rather than an omission", d.ID)
		}
	}
	return nil
}

// SubstrateReport is a MEASURED reading of the board status of the brief a
// strip's source comes from. It carries its own source and stamp so that
// asserting "the tool landed" is a statement with a provenance, not a boolean
// somebody set.
type SubstrateReport struct {
	// Brief is the brief whose status was read.
	Brief string
	// Status is the status the board carried at the stamp.
	Status string
	// Source is where that status was read from.
	Source askassay.Source
	// Stamp is when, and against what tree.
	Stamp askassay.Stamp
}

// landedStatuses are the board statuses at which a brief's deliverable is
// taken to exist. `implemented` is deliberately NOT one of them: the board
// carries `implemented` on work that has not been checked by anyone but its
// author, and a pane that renders against an unverified tool inherits that.
var landedStatuses = map[string]bool{"verified": true, "done": true}

// Landed reports whether the measured status is one at which the deliverable
// exists.
func (r SubstrateReport) Landed() bool {
	return landedStatuses[strings.ToLower(strings.TrimSpace(r.Status))]
}

// Validate reports why a substrate report cannot be relied on.
func (r SubstrateReport) Validate() error {
	if strings.TrimSpace(r.Brief) == "" {
		return errors.New("substrate report names no brief")
	}
	if strings.TrimSpace(r.Status) == "" {
		return fmt.Errorf("%s: substrate report carries no status — an unread status is not a landed one", r.Brief)
	}
	if err := r.Source.Validate(); err != nil {
		return fmt.Errorf("%s: substrate report %w", r.Brief, err)
	}
	if r.Stamp.Zero() {
		return fmt.Errorf("%s: substrate report carries no as-of stamp — a board status read at no particular time is not a reading", r.Brief)
	}
	return nil
}

// checkSubstrate is the anti-mock gate. A strip that reads a tool from another
// stream may not be derived until that tool is measured to exist.
func (d StripDef) checkSubstrate(r SubstrateReport) error {
	if d.RequiresBrief == "" {
		return nil
	}
	if err := r.Validate(); err != nil {
		return fmt.Errorf("%s reads %s (%s) and no usable measurement of that brief was supplied: %s. The strip renders could-not-check rather than an empty list, because the source does not exist yet and an empty list would say it returned nothing",
			d.ID, d.RequiresBrief, d.RequiresWhy, err.Error())
	}
	if !strings.EqualFold(strings.TrimSpace(r.Brief), d.RequiresBrief) {
		return fmt.Errorf("%s reads %s (%s) but the supplied measurement is of %s — a status read of the wrong brief is not evidence about this one",
			d.ID, d.RequiresBrief, d.RequiresWhy, r.Brief)
	}
	if !r.Landed() {
		return fmt.Errorf("%s reads %s (%s), measured at status %q as of %s — not landed. This strip therefore has NO SOURCE to read, and renders could-not-check. It is not empty: nothing was asked",
			d.ID, d.RequiresBrief, d.RequiresWhy, r.Status, r.Stamp.String())
	}
	return nil
}

// The MCP read surface the Improve pane is specified to render. Neither verb
// exists in any module of this tree — the surface that would deliver them is
// named on each strip below, and the strip cannot be derived until it is
// measured landed.
const (
	toolImproveList = "desk__improve_list"
	briefMCPSurface = "console-mcp-surface"
	briefMCPWhy     = "the read surface `" + toolImproveList + "` — reports, clusters, open proposals and adopted retro actions with their before/after"
)

// registry is the closed set of strips this pane can render.
var registry = map[StripID]StripDef{

	StripReports: {
		ID:    StripReports,
		Title: "Reports",
		Source: askassay.Source{
			Cmd:    toolImproveList + " --strip reports --class <good|bad|ugly> --program <program> --epic <epic>",
			Probe:  "the returned report array, one row per filed report, each carrying its class, its program and epic scope, and its evidence links",
			Window: "the filed-report stream over <window>, at the index's own freshness",
			Limit:  "UNKNOWN and therefore treated as unbounded-unproven — the tool does not exist, so its paging behaviour cannot be read from source. A limit that cannot be read is not a limit of none; it is the reason this strip refuses to render rows at all until the tool lands and its cap is declared here",
		},
		RequiresBrief: briefMCPSurface,
		RequiresWhy:   briefMCPWhy,
		Caveats: []string{
			CaveatEmptyStripIsBlind, CaveatTaxonomyUnhardened, CaveatEvidenceLinkNotEvidence,
			"the class/program/epic filters narrow what is SHOWN, never what was measured — a filtered strip is a subset with its filter stated, and the count of a filtered strip is never the count of the stream",
		},
	},

	StripClusters: {
		ID:    StripClusters,
		Title: "Clusters",
		Source: askassay.Source{
			Cmd:    toolImproveList + " --strip clusters --window <window>",
			Probe:  "the returned cluster array, one row per candidate systemic issue, each carrying the report IDs it groups",
			Window: "<window> — the span over which recurrence was judged, and part of the claim: three reports in two weeks and three reports in two years are different findings",
			Limit:  "UNKNOWN and therefore treated as unbounded-unproven — see the Reports strip. A clustering pass additionally has a JUDGEMENT bound this field cannot express: a cluster is an assertion that N signals are one issue, and the pane renders that assertion rather than confirming it",
		},
		RequiresBrief: briefMCPSurface,
		RequiresWhy:   briefMCPWhy,
		Caveats: []string{
			CaveatEmptyStripIsBlind, CaveatTaxonomyUnhardened,
			"a member ID that does not resolve to a rendered report is reported UNRESOLVED and stays in the cluster's member count — a cluster that lost two of five members is a cluster of five with two unresolved, never a tidy cluster of three",
		},
	},

	StripRetroQueue: {
		ID:    StripRetroQueue,
		Title: "Retro queue",
		Source: askassay.Source{
			Cmd:    toolImproveList + " --strip retro-queue",
			Probe:  "the returned proposal array — open entries in the append-only intake register, each with the evidence that motivated it",
			Window: "open proposals at the index's own freshness; a proposal adopted or rejected since the last index pass is stale in this strip and the as-of stamp is how you tell",
			Limit:  "UNKNOWN and therefore treated as unbounded-unproven — see the Reports strip. A truncated retro queue is the worst of the four to truncate silently, because the item that falls off the end looks exactly like an item nobody proposed",
		},
		RequiresBrief: briefMCPSurface,
		RequiresWhy:   briefMCPWhy,
		Caveats: []string{
			CaveatEmptyStripIsBlind, CaveatAdoptionIsHuman, CaveatRegistersAppendOnly,
			"one adoption per cadence is a HARD rule, not a suggestion — when the cadence's adoption has been taken, every remaining proposal renders as queued-not-adoptable, with the adoption that consumed the cadence named. It does not render as adoptable-and-ignored",
		},
	},

	StripDidItWork: {
		ID:    StripDidItWork,
		Title: "Did it work",
		Source: askassay.Source{
			Cmd:    toolImproveList + " --strip did-it-work",
			Probe:  "the returned retro-action array — one row per ADOPTED process change, each naming the metric it was expected to move; the before and after figures are separately sourced per row and are not read from this call",
			Window: "every adopted retro action, from the first to the most recent — this strip is deliberately not windowed, because the changes it stops showing are exactly the ones nobody is judging any more",
			Limit:  "UNKNOWN for the ROW set (see the Reports strip). The per-row before/after figures carry their OWN limits, one declared source each; this strip's limit says nothing about them and must not be read as covering them",
		},
		RequiresBrief: briefMCPSurface,
		RequiresWhy:   briefMCPWhy,
		Caveats: []string{
			CaveatEmptyStripIsBlind, CaveatUnwiredMetricIsNotNoMovement, CaveatAdoptionIsHuman,
			"before/after are two readings of the same metric at two times, not a controlled comparison — everything else in the system also changed between them, so a moved number is consistent with the change having worked, never proof that it did",
		},
	},
}

// Lookup returns a declared strip definition.
func Lookup(id StripID) (StripDef, bool) {
	d, ok := registry[id]
	return d, ok
}

// Strips returns every declared strip definition, in the §7.3 order the pane
// lays them out: the signal, its grouping, the proposals it motivates, and
// the judgement of what was adopted.
func Strips() []StripDef {
	order := []StripID{StripReports, StripClusters, StripRetroQueue, StripDidItWork}
	out := make([]StripDef, 0, len(order))
	for _, id := range order {
		if d, ok := registry[id]; ok {
			out = append(out, d)
		}
	}
	// Anything declared but not ordered is still returned, so a strip added
	// without an order entry is visible rather than silently dropped.
	seen := map[StripID]bool{}
	for _, id := range order {
		seen[id] = true
	}
	var extra []StripDef
	for id, d := range registry {
		if !seen[id] {
			extra = append(extra, d)
		}
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i].ID < extra[j].ID })
	return append(out, extra...)
}

// Undeclared builds the strip for an ID the registry does not know. It exists
// so that "there is no such strip" is a rendered could-not-check with a stamp
// rather than a blank area of screen.
func Undeclared(id StripID, st askassay.Stamp) Strip {
	return Strip{
		id:     id,
		state:  askassay.CouldNotCheck,
		stamp:  st,
		reason: fmt.Sprintf("no declared source for strip %q — this pane renders only from the strip registry, and a row set with no source behind it does not render", id),
	}
}
