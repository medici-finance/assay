package main

// plan.go — ONE plan value for `verifyloop plan` (brief 45).
//
// cmdPlan used to print as it decided, so the text WAS the plan and nothing else could read
// it. It now builds a planDoc first and renders from it: the text renderer (renderPlanText) and
// the opt-in JSON renderer (--json) are two views of the SAME value, so they cannot disagree
// about a root's state, an item's disposition, or whether the plan is the whole queue.
//
// ROOT COVERAGE is explicit per root: healthy (read and preflight green), skipped (the
// envelope preflight was red) or unreadable (the root path could not be resolved). A skipped or
// unreadable root contributes no items and is NEVER a zero: it is named in the doc, rendered in
// the text, and turns the exit code to 6.
//
// CANONICAL BASE (--base-queue, opt-in). The verification queue's classification is
// statusgen's (statusgen/awaiting_bucket.go queueFor, exported on `--gate-scores` rows as
// queueVersion/queueClass/...). verifyloop's own overlays (receipts, blocked-until, risk, stuck
// flips, online lane) read signals statusgen does not see; the two compose with ONE precedence,
// documented on applyBase in this file. With --base-queue off the plan is exactly the overlay
// plan it has always been.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// planVersion is the version of the JSON plan document. Fields are only ever added.
const planVersion = 1

// Root states in the plan document.
const (
	rootHealthy    = "healthy"
	rootSkipped    = "skipped"    // the per-root envelope preflight was red
	rootUnreadable = "unreadable" // the configured root path could not be resolved
)

// Item kinds in the plan document.
const (
	kindRefused     = "refused"
	kindTierError   = "tier-error"
	kindDispatch    = "dispatch"
	kindForEvidence = "dispatch-for-evidence"
	kindBucketed    = "bucketed"
)

// Authority says what a model may do with an item once dispatched. Evidence-only human work is
// a different authority from flip: the Evidence rows may be written, the status never.
const (
	authFlip         = "flip"
	authEvidenceOnly = "evidence-only"
	authNone         = "none"
)

// Base coverage states (per root, and per item) of the canonical queue reading.
const (
	baseOff         = "off"
	baseCanonical   = "canonical"   // every item found a supported classification
	basePartial     = "partial"     // some items found none
	baseUnsupported = "unsupported" // the statusgen on PATH predates the queue contract
	baseUnreadable  = "unreadable"  // statusgen could not be run or its output not parsed
)

// planRoot is one configured (or the single) root's reading.
type planRoot struct {
	Repo   string `json:"repo,omitempty"`
	Path   string `json:"path,omitempty"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
	// Queue is the canonical-queue coverage for this root ("off" unless --base-queue).
	Queue       string `json:"queue"`
	QueueReason string `json:"queueReason,omitempty"`
}

// planBase is the canonical classification an item was composed with.
type planBase struct {
	Class   string `json:"class"`
	Bucket  string `json:"bucket,omitempty"`
	Owner   string `json:"owner,omitempty"`
	NextAct string `json:"nextAct,omitempty"`
	Note    string `json:"note,omitempty"`
}

// planItem is one queue item's outcome.
type planItem struct {
	ID          string `json:"id"`
	Repo        string `json:"repo,omitempty"`
	Root        string `json:"root,omitempty"`
	Kind        string `json:"kind"`
	Disposition string `json:"disposition,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Tier        string `json:"tier,omitempty"`
	Authority   string `json:"authority"`
	Wake        string `json:"wake,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	// Overlay is the disposition the verifyloop overlays gave BEFORE the canonical base was
	// composed in; it differs from Disposition only when the base demoted the item.
	Overlay string    `json:"overlay,omitempty"`
	Base    *planBase `json:"base,omitempty"`

	disp disposition
}

// planCounts is the summary the text's last lines print.
type planCounts struct {
	Awaiting      int `json:"awaiting"`
	Dispatchable  int `json:"dispatchable"`
	ForEvidence   int `json:"forEvidence"`
	Bucketed      int `json:"bucketed"`
	Wait          int `json:"wait"`
	CouldNotCheck int `json:"couldNotCheck"`
}

// planDoc is the plan value. Everything either renderer prints is read off it.
type planDoc struct {
	Version   int        `json:"version"`
	Multi     bool       `json:"multiRoot"`
	BaseQueue string     `json:"baseQueue"` // "off" | "on"
	Roots     []planRoot `json:"roots"`
	Items     []planItem `json:"items"`
	Counts    planCounts `json:"counts"`
	// Skipped counts roots that contributed nothing (skipped + unreadable).
	Skipped int `json:"skippedRoots"`
	// Complete is false when the plan is not the whole queue: a root was skipped or unreadable,
	// or the canonical base was requested and some root's reading was not canonical.
	Complete bool `json:"complete"`
	// Gaps names every reason Complete is false.
	Gaps []string `json:"gaps"`

	configured int
	bucketed   map[disposition][]bucketMember
}

// --- canonical base ---------------------------------------------------------------------

// baseRow is the subset of a `statusgen --gate-scores` row the plan composes with.
type baseRow struct {
	Brief        string `json:"brief"`
	Status       string `json:"status"`
	QueueVersion int    `json:"queueVersion"`
	QueueClass   string `json:"queueClass"`
	QueueBucket  string `json:"queueBucket"`
	QueueOwner   string `json:"queueOwner"`
	QueueNextAct string `json:"queueNextAct"`
	QueueValid   bool   `json:"queueValid"`
	QueueWhy     string `json:"queueWhy"`
}

// baseQueueRows reads one root's canonical rows. It is a package var ONLY as the test seam
// (the fixture tests serve the shared statusgen golden through it); production runs the
// statusgen binary named by STATUSGEN_BIN, else the one on PATH, read-only.
var baseQueueRows = func(root string) ([]baseRow, error) {
	bin := strings.TrimSpace(os.Getenv("STATUSGEN_BIN"))
	if bin == "" {
		bin = "statusgen"
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("statusgen is not available (%v)", err)
	}
	cmd := exec.Command(path, "--gate-scores", "--root", root)
	out, err := deskkit.OutputProcess(cmd)
	if err != nil {
		return nil, fmt.Errorf("statusgen --gate-scores failed for %s: %v", root, err)
	}
	var rows []baseRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("cannot parse statusgen --gate-scores output for %s: %v", root, err)
	}
	return rows, nil
}

// queueContractVersion is the statusgen queue contract this binary composes with.
const queueContractVersion = 1

// baseLookup is one root's canonical reading, keyed <stream>/<NN>.
type baseLookup struct {
	state  string
	reason string
	rows   map[string]baseRow
}

func readBase(root string) baseLookup {
	rows, err := baseQueueRows(root)
	if err != nil {
		return baseLookup{state: baseUnreadable, reason: err.Error()}
	}
	b := baseLookup{state: baseCanonical, rows: map[string]baseRow{}}
	unsupported := 0
	for _, r := range rows {
		if r.QueueVersion != queueContractVersion || r.QueueClass == "" {
			unsupported++
			continue
		}
		b.rows[r.Brief] = r
	}
	if unsupported > 0 && len(b.rows) == 0 {
		b.state = baseUnsupported
		b.reason = fmt.Sprintf("%d row(s) carry no supported queueVersion (statusgen predates the queue contract v%d)",
			unsupported, queueContractVersion)
	} else if unsupported > 0 {
		b.state = basePartial
		b.reason = fmt.Sprintf("%d row(s) carry no supported queueVersion", unsupported)
	}
	return b
}

// itemKey is the statusgen row key (<stream>/<NN>) of a queue item: its ID minus the
// multi-root "<owner>/<repo>:" prefix.
func itemKey(it loopengine.Item) string {
	if repo := payloadValue(it, "repo"); repo != "" {
		return strings.TrimPrefix(it.ID, repo+":")
	}
	return it.ID
}

// applyBase composes the canonical base class under an item's overlay disposition.
//
// PRECEDENCE (documented once, here):
//
//  1. The verifyloop overlays decide first, exactly as classifyItem always has: could-not-check,
//     blocked-until, human gate / risk, stuck-flip, wake receipt, in-repair, online lane. Each
//     reads a signal the canonical base cannot see, and an item the overlays HOLD stays held
//     whatever the base says. The base never promotes anything.
//  2. The base is applied last and may only DEMOTE an item the overlays would hand to a model:
//     - a DISPATCH item must be judgment-dispatch. runnable-offline, continuation, human-action,
//     paused and parked are not work for a model judge: it is held as queue-held, with the
//     base's own bucket, owner and next act as the reason. An unreadable class, or no
//     supported classification at all (row missing, statusgen older than the contract, root
//     unread), is could-not-check: an instrument that did not look has cleared nothing.
//     - a DISPATCH-FOR-EVIDENCE item is Evidence-only human work. The base's human-action class
//     is exactly what it should read (the flip belongs to a person), so human-action and
//     judgment-dispatch keep it; the other classes demote it as above, and unreadable or
//     missing is could-not-check. It never gains flip authority from the base.
//  3. An item both layers agree on keeps its overlay reason; the base is recorded beside it.
func applyBase(it loopengine.Item, disp disposition, reason string, b *baseLookup) (disposition, string, *planBase) {
	if b == nil {
		return disp, reason, nil
	}
	if disp != dispDispatch && disp != dispDispatchForEvidence {
		// Held by an overlay: record what the base thinks when it has a reading, change nothing.
		if row, ok := b.rows[itemKey(it)]; ok {
			return disp, reason, baseOf(row)
		}
		return disp, reason, nil
	}
	row, ok := b.rows[itemKey(it)]
	if !ok {
		why := "no supported canonical classification for " + itemKey(it)
		if b.reason != "" {
			why += " (" + b.reason + ")"
		}
		return dispCouldNotCheck, "canonical queue: " + why, &planBase{Class: "unclassified", Note: b.reason}
	}
	pb := baseOf(row)
	if !row.QueueValid {
		return dispCouldNotCheck, "canonical queue: unreadable — " + firstNonEmpty(row.QueueWhy, row.QueueNextAct), pb
	}
	switch row.QueueClass {
	case "judgment-dispatch":
		return disp, reason, pb
	case "human-action":
		if disp == dispDispatchForEvidence {
			return disp, reason, pb
		}
	}
	return dispBaseHeld, fmt.Sprintf("canonical queue: %s (%s, owner %s): %s",
		row.QueueClass, firstNonEmpty(row.QueueBucket, "-"), firstNonEmpty(row.QueueOwner, "-"), row.QueueNextAct), pb
}

func baseOf(r baseRow) *planBase {
	return &planBase{Class: r.QueueClass, Bucket: r.QueueBucket, Owner: r.QueueOwner, NextAct: r.QueueNextAct, Note: r.QueueWhy}
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// --- build ------------------------------------------------------------------------------

// planOpts are the cmdPlan inputs that shape the plan value.
type planOpts struct {
	root      string
	sha       string
	runner    string
	multi     bool
	baseQueue bool
}

// buildPlan reads the roots, the queue and (optionally) the canonical base, and returns the
// plan value. It prints nothing: every diagnostic is in the value.
func buildPlan(v *VerifyLoop, o planOpts) (*planDoc, error) {
	doc := &planDoc{Version: planVersion, Multi: o.multi, BaseQueue: "off", Complete: true,
		Gaps: []string{}, bucketed: map[disposition][]bucketMember{}}
	if o.baseQueue {
		doc.BaseQueue = "on"
	}
	bases := map[string]*baseLookup{} // by root path

	if o.multi {
		if strings.TrimSpace(o.sha) != "" {
			return nil, deskkit.Refused("refused: --sha names ONE merged-main SHA but the plan spans every " +
				deskkit.RootsEnv + " root, each with its own main — pass --root to narrow to one")
		}
		roots, rerr := deskkit.ConfiguredRoots()
		if rerr != nil {
			return nil, rerr
		}
		doc.configured = len(roots)
		// A reused loop cannot carry an earlier pass's admitted roots forward.
		v.Roots = nil
		for _, rc := range roots {
			pr := planRoot{Repo: rc.Repo, Queue: baseOff}
			abs, err := deskkit.ResolveRoot(rc)
			if err != nil {
				pr.State, pr.Reason = rootUnreadable, err.Error()
				doc.Skipped++
				doc.Roots = append(doc.Roots, pr)
				continue
			}
			pr.Path = abs
			stop := v.performance.phase("preflight")
			err = preflightRoot(abs)
			stop()
			if err != nil {
				pr.State, pr.Reason = rootSkipped, err.Error()
				doc.Skipped++
				doc.Roots = append(doc.Roots, pr)
				continue
			}
			pr.State = rootHealthy
			doc.Roots = append(doc.Roots, pr)
			v.Roots = append(v.Roots, deskkit.RootConfig{Repo: rc.Repo, Path: abs})
		}
	} else {
		doc.Roots = []planRoot{{Repo: v.RootRepo, Path: o.root, State: rootHealthy, Queue: baseOff}}
	}
	if o.baseQueue {
		for i := range doc.Roots {
			r := &doc.Roots[i]
			if r.State != rootHealthy {
				r.Queue, r.QueueReason = baseUnreadable, "root not read: "+r.State
				continue
			}
			stop := v.performance.phase("base-queue")
			b := readBase(r.Path)
			stop()
			bases[r.Path] = &b
			r.Queue, r.QueueReason = b.state, b.reason
		}
	}

	var items []loopengine.Item
	// An empty admitted multi-root set is not the legacy single-root mode.
	// Keep its coverage gaps, but do no queue/source/provider work for it.
	if !o.multi || len(v.Roots) > 0 {
		var err error
		items, err = v.SelectQueue()
		if err != nil {
			return nil, deskkit.Unverifiable("cannot read the Awaiting queue", err)
		}
	}
	doc.Counts.Awaiting = len(items)

	baseFor := func(it loopengine.Item) *baseLookup {
		if !o.baseQueue {
			return nil
		}
		path := payloadValue(it, "root")
		if path == "" {
			path = o.root
		}
		if b, ok := bases[path]; ok {
			return b
		}
		return &baseLookup{state: baseUnreadable, reason: "root not read"}
	}

	stopClassify := v.performance.phase("classify")
	defer stopClassify()
	for _, it := range items {
		pi := planItem{ID: it.ID, Repo: payloadValue(it, "repo"), Root: payloadValue(it, "root"), Authority: authNone}
		// author != runner is a STRUCTURAL engine guard, shown here for transparency.
		if err := loopengine.CheckAuthorRunner(it, o.runner); err != nil {
			pi.Kind, pi.Reason = kindRefused, err.Error()
			doc.Items = append(doc.Items, pi)
			continue
		}
		tier, terr := v.TierPolicy(it)
		if terr != nil {
			pi.Kind, pi.Reason = kindTierError, terr.Error()
			doc.Items = append(doc.Items, pi)
			continue
		}
		pi.Tier = tier.String()
		disp, reason := classifyItem(it, tier)
		overlay := disp
		var pb *planBase
		disp, reason, pb = applyBase(it, disp, reason, baseFor(it))
		pi.Base = pb
		if disp != overlay {
			pi.Overlay = overlay.String()
		}
		pi.Disposition, pi.Reason, pi.disp = disp.String(), reason, disp

		switch disp {
		case dispDispatchForEvidence:
			prompt := renderEvidenceOnlyPrompt(it, reason)
			if err := assertNoSharedCheckout(prompt); err != nil {
				return nil, deskkit.Refused(err.Error())
			}
			pi.Kind, pi.Prompt, pi.Authority = kindForEvidence, prompt, authEvidenceOnly
			pi.Tier = loopengine.TierLocal.String()
			doc.Counts.ForEvidence++
		case dispDispatch:
			prompt := renderDispatchPrompt(it, tier)
			if err := assertNoSharedCheckout(prompt); err != nil {
				return nil, deskkit.Refused(err.Error())
			}
			pi.Kind, pi.Prompt, pi.Authority = kindDispatch, prompt, authFlip
			pi.Wake = payloadValue(it, "wake_reason")
			doc.Counts.Dispatchable++
		default:
			pi.Kind = kindBucketed
			doc.bucketed[disp] = append(doc.bucketed[disp], bucketMember{ID: it.ID, Reason: reason})
			doc.Counts.Bucketed++
		}
		doc.Items = append(doc.Items, pi)
	}
	doc.Counts.Wait = len(doc.bucketed[dispWaitReceipt])
	doc.Counts.CouldNotCheck = len(doc.bucketed[dispCouldNotCheck])

	for _, r := range doc.Roots {
		if r.State != rootHealthy {
			doc.Complete = false
			doc.Gaps = append(doc.Gaps, fmt.Sprintf("root %s %s: %s — its queue is NOT in this plan",
				firstNonEmpty(r.Repo, r.Path), r.State, r.Reason))
		}
	}
	if o.baseQueue {
		for _, r := range doc.Roots {
			if r.State == rootHealthy && r.Queue != baseCanonical {
				doc.Complete = false
				doc.Gaps = append(doc.Gaps, fmt.Sprintf("canonical queue for %s is %s: %s",
					firstNonEmpty(r.Repo, r.Path), r.Queue, r.QueueReason))
			}
		}
	}
	return doc, nil
}

// --- render -----------------------------------------------------------------------------

// renderPlanText prints the plan value as the text `verifyloop plan` has always printed (plus,
// only under --base-queue, one coverage line per root that was not canonical).
func renderPlanText(p *planDoc) {
	if p.Multi {
		fmt.Printf("verify-desk plan: %d configured root(s) (%s)\n", p.configured, deskkit.RootsEnv)
		for _, r := range p.Roots {
			switch r.State {
			case rootUnreadable:
				fmt.Printf("-- root %s: could-not-check — skipped: %s\n", r.Repo, r.Reason)
			case rootSkipped:
				fmt.Printf("-- root %s (%s): PREFLIGHT RED — skipped: %s\n", r.Repo, r.Path, r.Reason)
			default:
				fmt.Printf("-- root %s (%s): preflight green\n", r.Repo, r.Path)
			}
		}
	}
	if p.BaseQueue == "on" {
		for _, r := range p.Roots {
			if r.State == rootHealthy && r.Queue != baseCanonical {
				fmt.Printf("-- canonical queue %s (%s): %s — %s\n",
					firstNonEmpty(r.Repo, r.Path), r.Path, r.Queue, r.QueueReason)
			}
		}
	}
	nroots := 0
	for _, r := range p.Roots {
		if r.State == rootHealthy {
			nroots++
		}
	}
	if p.Multi && nroots > 0 {
		fmt.Printf("verify-desk plan: %d brief(s) awaiting across %d root(s) (tier-1 first, oldest-first within class)\n",
			p.Counts.Awaiting, nroots)
	} else {
		fmt.Printf("verify-desk plan: %d brief(s) awaiting (tier-1 first, oldest-first within class)\n", p.Counts.Awaiting)
	}

	for _, it := range p.Items {
		switch it.Kind {
		case kindRefused:
			fmt.Printf("\n-- %s: REFUSED (%s) — needs a different runner\n", it.ID, it.Reason)
		case kindTierError:
			fmt.Printf("\n-- %s: tier error: %s\n", it.ID, it.Reason)
		case kindDispatch:
			fmt.Printf("\n=== DISPATCH %s (tier=%s)%s ===\n%s\n", it.ID, it.Tier, rootSuffix(it), it.Prompt)
			if it.Wake != "" {
				fmt.Printf("wake: %s\n", it.Wake)
			}
		}
	}
	forEvidence := 0
	for _, it := range p.Items {
		if it.Kind != kindForEvidence {
			continue
		}
		forEvidence++
		fmt.Printf("\n=== DISPATCH-FOR-EVIDENCE %s (tier=%s)%s — ROUTE-HUMAN: %s — %s ===\n%s\n",
			it.ID, it.Tier, rootSuffix(it), it.Reason, evidenceOnlyMarker, it.Prompt)
	}
	if forEvidence > 0 {
		fmt.Printf("\nverify-desk plan: %d dispatch-for-evidence (Evidence rows + outcome sidecar only; flip never)\n", forEvidence)
	}
	printBuckets(p.Counts.Dispatchable, p.bucketed)
}

func rootSuffix(it planItem) string {
	if it.Root != "" {
		return " root=" + it.Root
	}
	return ""
}

// renderPlanJSON writes the plan value as one JSON document on stdout and nothing else.
func renderPlanJSON(p *planDoc) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "%s\n", b)
	return err
}
