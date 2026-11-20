package main

// milestones.go — `deskboard milestones` (milestone-support/11).
//
// The verb is a pure consumer of the versioned stream-view projection. It
// decodes stream-view documents IN PROCESS through statusgen/streamview (no
// subprocess, no fetch, no store) and renders what the wire says. It derives
// nothing: work remaining, acceptance and availability are shown exactly as the
// producer projected them. In particular zero work remaining is displayed as
// "0 remaining" next to the acceptance state — the renderer never concludes
// "done" from a count.
//
// Contract handling:
//   - a document whose contract version this binary cannot decode is REFUSED
//     (exit 5), naming the file — a required wire semantic that is not
//     understood is never skipped;
//   - a stream-view/v1 document predates milestones: it is listed as
//     "not assessed", never as "no milestones";
//   - a view from a repository outside the board's allow-list is withheld, and
//     evidence or source locators that name such a repository are shown as
//     [withheld] — an inaccessible locator is never echoed;
//   - legacy aliases collapse onto the canonical milestone: an alias never
//     adds a row and never changes approval state.

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/medici-finance/assay/statusgen/streamview"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// maxStreamViewBytes bounds one input document.
const maxStreamViewBytes = 4 << 20

const withheldLocator = "[withheld]"

type milestoneViewRow struct {
	File         string   `json:"file"`
	Stream       string   `json:"stream"`
	Contract     string   `json:"contract"`
	Availability string   `json:"availability"`
	Coverage     string   `json:"coverage,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	Diagnostics  []string `json:"diagnostics,omitempty"`
	Unknowns     []string `json:"unknowns,omitempty"`
	Sources      int      `json:"sources"`
	Items        int      `json:"items"`
	Withheld     bool     `json:"withheld,omitempty"`
}

type milestoneRow struct {
	Ref           string                         `json:"ref"`
	Title         string                         `json:"title"`
	Owner         string                         `json:"owner,omitempty"`
	Aliases       []string                       `json:"aliases,omitempty"`
	Streams       []string                       `json:"streams"`
	Scope         []string                       `json:"supportedScope"`
	Work          streamview.MilestoneWork       `json:"work"`
	Acceptance    streamview.MilestoneAcceptance `json:"acceptance"`
	Availability  string                         `json:"availability"`
	Prerequisites []string                       `json:"prerequisites,omitempty"`
	Holds         []streamview.MilestoneHold     `json:"holds,omitempty"`
	Evidence      []string                       `json:"evidence,omitempty"`
	Conflicts     []string                       `json:"conflicts,omitempty"`
}

type milestoneMember struct {
	Brief      string   `json:"brief"`
	Milestones []string `json:"milestones"`
}

type milestonesReport struct {
	Header
	Views      []milestoneViewRow `json:"views"`
	Milestones []milestoneRow     `json:"milestones"`
	Membership []milestoneMember  `json:"membership,omitempty"`
	// Withheld counts the views and locators not shown because they name a
	// repository outside the board's allow-list.
	WithheldViews    int `json:"withheldViews"`
	WithheldLocators int `json:"withheldLocators"`
}

// cmdMilestones reads each named stream-view document ("-" is stdin) and renders
// the milestone projection. It never writes anything.
func cmdMilestones(hdr Header, files []string) (*Report, error) {
	if len(files) == 0 {
		return nil, deskkit.Refused("refused: milestones needs at least one <stream-view.json> (or - for stdin)")
	}
	rep := milestonesReport{Header: hdr, Views: []milestoneViewRow{}, Milestones: []milestoneRow{}}
	var items []pendingItem
	members := map[string]map[string]bool{}

	for _, f := range files {
		data, err := readStreamView(f)
		if err != nil {
			return nil, deskkit.Unverifiable("milestones: cannot read "+inert(f, 80)+": "+err.Error(), err)
		}
		v, err := streamview.Decode(data)
		if err != nil {
			// Includes UnsupportedVersionError: a contract this binary cannot
			// decode is refused, never partially rendered.
			return nil, deskkit.Refused("refused: milestones: " + inert(f, 80) + ": " + flatMilestoneErr(err.Error()))
		}
		row := milestoneViewRow{File: f, Contract: v.Contract, Stream: v.Identity.Key.Repo + ":" + v.Identity.Key.Slug}
		if !deskkit.IsAllowedRepo(v.Identity.Key.Repo) {
			rep.WithheldViews++
			rep.Views = append(rep.Views, milestoneViewRow{File: withheldLocator, Stream: withheldLocator, Contract: v.Contract,
				Availability: "withheld", Withheld: true})
			continue
		}
		switch {
		case v.Milestones == nil:
			row.Availability = string(streamview.NotAssessed)
			row.Reason = "view predates milestones (" + v.Contract + ")"
		default:
			m := v.Milestones
			row.Availability = string(m.Provenance.Availability)
			row.Coverage = m.Coverage
			row.Reason = m.Provenance.Reason
			row.Diagnostics = m.Provenance.Diagnostics
			row.Unknowns = m.Unknowns
			row.Items = len(m.Items)
			for _, s := range m.Provenance.Sources {
				if deskkit.IsAllowedRepo(s.Repo) {
					row.Sources++
				} else {
					rep.WithheldLocators++
				}
			}
			for _, it := range m.Items {
				items = append(items, pendingItem{stream: row.Stream, item: it})
			}
			for _, mb := range m.Membership {
				if members[mb.Brief] == nil {
					members[mb.Brief] = map[string]bool{}
				}
				for _, r := range mb.Milestones {
					members[mb.Brief][r] = true
				}
			}
		}
		rep.Views = append(rep.Views, row)
	}

	// Alias index: an alias denotes the canonical milestone that lists it.
	canonical := map[string]string{}
	for _, p := range items {
		for _, a := range p.item.Aliases {
			canonical[a] = p.item.Ref
		}
	}
	resolve := func(ref string) string {
		if c, ok := canonical[ref]; ok {
			return c
		}
		return ref
	}

	rows := map[string]*milestoneRow{}
	// Canonical holders first, so an alias-spelled duplicate can never be the
	// row that supplies approval state.
	ordered := append([]pendingItem(nil), items...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return resolve(ordered[i].item.Ref) == ordered[i].item.Ref && resolve(ordered[j].item.Ref) != ordered[j].item.Ref
	})
	for _, p := range ordered {
		it := p.item
		key := resolve(it.Ref)
		if cur, ok := rows[key]; ok {
			cur.Streams = appendUnique(cur.Streams, p.stream)
			cur.Aliases = appendUnique(cur.Aliases, it.Aliases...)
			if it.Ref != key {
				cur.Aliases = appendUnique(cur.Aliases, it.Ref)
			}
			if d := milestoneDiff(*cur, it); d != "" {
				cur.Conflicts = appendUnique(cur.Conflicts, p.stream+": "+d)
			}
			continue
		}
		r := milestoneRow{
			Ref: key, Title: it.Title, Owner: it.Owner, Aliases: appendUnique(nil, it.Aliases...),
			Streams: []string{p.stream}, Scope: append([]string{}, it.Scope...),
			Work: it.Work, Acceptance: it.Acceptance, Availability: it.Availability,
			Prerequisites: it.Prerequisites, Holds: it.Holds,
		}
		for _, e := range it.Evidence {
			loc, shown := evidenceLocator(e)
			if !shown {
				rep.WithheldLocators++
			}
			r.Evidence = append(r.Evidence, loc)
		}
		rows[key] = &r
	}
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sort.Strings(rows[k].Aliases)
		rep.Milestones = append(rep.Milestones, *rows[k])
	}
	briefs := make([]string, 0, len(members))
	for b := range members {
		briefs = append(briefs, b)
	}
	sort.Strings(briefs)
	for _, b := range briefs {
		seen := map[string]bool{}
		var ms []string
		for r := range members[b] {
			c := resolve(r)
			if !seen[c] {
				seen[c] = true
				ms = append(ms, c)
			}
		}
		sort.Strings(ms)
		rep.Membership = append(rep.Membership, milestoneMember{Brief: b, Milestones: ms})
	}

	detail := fmt.Sprintf("milestones views=%d rows=%d withheldViews=%d", len(rep.Views), len(rep.Milestones), rep.WithheldViews)
	return &Report{value: rep, render: func(w io.Writer) { renderMilestones(w, rep) }, detail: detail}, nil
}

type pendingItem struct {
	stream string
	item   streamview.MilestoneItem
}

func readStreamView(path string) ([]byte, error) {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	data, err := io.ReadAll(io.LimitReader(r, maxStreamViewBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxStreamViewBytes {
		return nil, fmt.Errorf("document exceeds %d bytes", maxStreamViewBytes)
	}
	return data, nil
}

// milestoneDiff names a disagreement between a collected row and another view's
// reading of the same milestone. Disagreement is reported, never resolved by
// guessing: the row keeps the first reading and says it was contested.
func milestoneDiff(cur milestoneRow, it streamview.MilestoneItem) string {
	var d []string
	if cur.Work.State != it.Work.State || cur.Work.Remaining != it.Work.Remaining {
		d = append(d, "work")
	}
	if cur.Acceptance.State != it.Acceptance.State {
		d = append(d, "acceptance")
	}
	if cur.Availability != it.Availability {
		d = append(d, "availability")
	}
	if len(d) == 0 {
		return ""
	}
	return "disagrees on " + strings.Join(d, ", ")
}

func appendUnique(dst []string, vals ...string) []string {
	for _, v := range vals {
		dup := false
		for _, d := range dst {
			if d == v {
				dup = true
				break
			}
		}
		if !dup {
			dst = append(dst, v)
		}
	}
	return dst
}

func flatMilestoneErr(s string) string {
	return strings.Join(strings.Fields(deskkit.StripControl(s)), " ")
}

// evidenceLocator renders one evidence reference, withholding any locator that
// names a repository the board may not read.
func evidenceLocator(e streamview.EvidenceRef) (string, bool) {
	switch e.Kind {
	case streamview.EvidencePath:
		if deskkit.IsAllowedRepo(e.Repo) {
			s := e.Repo + ":" + e.Path
			if e.Planned {
				s += " (planned)"
			}
			return s, true
		}
	case streamview.EvidenceForge:
		if deskkit.IsAllowedRepo(e.Repo) {
			return fmt.Sprintf("%s#%d", e.Repo, e.Number), true
		}
	case streamview.EvidenceURL:
		if u, err := url.Parse(e.URL); err == nil && u.Scheme == "https" && u.Host == "github.com" {
			if segs := strings.Split(strings.Trim(u.Path, "/"), "/"); len(segs) >= 2 && deskkit.IsAllowedRepo(segs[0]+"/"+segs[1]) {
				return e.URL, true
			}
		}
	}
	return withheldLocator, false
}

func workCell(w streamview.MilestoneWork) string {
	if w.State != streamview.WorkKnown {
		return "unknown (" + inert(w.Reason, 80) + ")"
	}
	return fmt.Sprintf("%d remaining", w.Remaining)
}

func acceptanceCell(a streamview.MilestoneAcceptance) string {
	if a.Reason != "" {
		return a.State + " (" + inert(a.Reason, 80) + ")"
	}
	return a.State
}

func renderMilestones(w io.Writer, rep milestonesReport) {
	fmt.Fprintf(w, "MILESTONES as of %s — %d view(s), %d milestone(s)\n", rep.AsOf, len(rep.Views), len(rep.Milestones))
	fmt.Fprintln(w, "Displays the projected state only: work remaining, acceptance and availability are separate facts.")
	fmt.Fprintln(w, "\nSOURCE COVERAGE")
	for _, v := range rep.Views {
		if v.Withheld {
			fmt.Fprintf(w, "  %s contract=%s availability=withheld (repository outside the board's allow-list)\n", withheldLocator, v.Contract)
			continue
		}
		fmt.Fprintf(w, "  %s contract=%s availability=%s coverage=%s sources=%d items=%d\n",
			inert(v.Stream, 80), v.Contract, v.Availability, covCell(v.Coverage), v.Sources, v.Items)
		if v.Reason != "" {
			fmt.Fprintf(w, "    reason: %s\n", inert(v.Reason, 160))
		}
		for _, d := range v.Diagnostics {
			fmt.Fprintf(w, "    diagnostic: %s\n", inert(d, 160))
		}
		for _, u := range v.Unknowns {
			fmt.Fprintf(w, "    UNKNOWN: %s\n", inert(u, 160))
		}
	}
	if len(rep.Milestones) == 0 {
		fmt.Fprintln(w, "\nNo milestone rows were projected by the views above (see source coverage; absence here is not a clean result).")
	}
	for _, m := range rep.Milestones {
		fmt.Fprintf(w, "\nMILESTONE %s  %s\n", inert(m.Ref, 120), inert(m.Title, 120))
		fmt.Fprintf(w, "  work: %s | acceptance: %s | availability: %s\n", workCell(m.Work), acceptanceCell(m.Acceptance), m.Availability)
		fmt.Fprintf(w, "  streams: %s\n", joinInert(m.Streams, 80))
		if len(m.Aliases) > 0 {
			fmt.Fprintf(w, "  aliases (same milestone, no separate state): %s\n", joinInert(m.Aliases, 80))
		}
		fmt.Fprintf(w, "  supported scope: %s\n", orNone(joinInert(m.Scope, 80)))
		if len(m.Work.Pending) > 0 {
			fmt.Fprintf(w, "  work remaining on: %s\n", joinInert(m.Work.Pending, 80))
		}
		if len(m.Prerequisites) > 0 {
			fmt.Fprintf(w, "  prerequisites: %s\n", joinInert(m.Prerequisites, 80))
		}
		for _, h := range m.Holds {
			fmt.Fprintf(w, "  HOLD %s %s %s: %s\n", h.Kind, inert(h.Ref, 80), inert(h.State, 40), inert(h.Reason, 160))
		}
		if len(m.Evidence) > 0 {
			ev := make([]string, len(m.Evidence))
			for i, e := range m.Evidence {
				if e == withheldLocator {
					ev[i] = e
				} else {
					ev[i] = inert(e, 120)
				}
			}
			fmt.Fprintf(w, "  evidence: %s\n", strings.Join(ev, ", "))
		}
		for _, c := range m.Conflicts {
			fmt.Fprintf(w, "  CONFLICT: %s\n", inert(c, 160))
		}
	}
	if len(rep.Membership) > 0 {
		fmt.Fprintln(w, "\nMEMBERSHIP (work item -> milestones)")
		for _, mb := range rep.Membership {
			fmt.Fprintf(w, "  %s -> %s\n", inert(mb.Brief, 80), joinInert(mb.Milestones, 80))
		}
	}
	if rep.WithheldViews > 0 || rep.WithheldLocators > 0 {
		fmt.Fprintf(w, "\nWITHHELD: %d view(s), %d locator(s) name a repository outside the board's allow-list\n", rep.WithheldViews, rep.WithheldLocators)
	}
}

func covCell(c string) string {
	if c == "" {
		return "n/a"
	}
	return c
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

func joinInert(vals []string, n int) string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = inert(v, n)
	}
	return strings.Join(out, ", ")
}
