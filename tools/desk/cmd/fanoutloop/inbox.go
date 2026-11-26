package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/deskcore/issueflow"
	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
	"github.com/medici-finance/assay/tools/desk/internal/topology"
)

// inbox.go — the addressed-inbox offer lane and the per-tick re-read of carried decision holds.
//
// The board lane only sees a `to:worker` message once intake has filed a placeholder brief for
// it, and the work-ready issue projection deliberately excludes addressed issues. So an issue
// addressed to this desk with no placeholder was read by nothing that dispatches: it sat in
// `issueboard issues --projection inbox --to worker` while the pool idled. This lane turns those
// inbox issues into dispatch offers in the leading (addressed) lane, behind the same gates every
// other offer passes:
//
//   - trust: an untrusted author without a current blessing is QUARANTINED, never offered;
//   - decision: an issue carrying a decision-owed label (needs-decision, question), help wanted,
//     or a system-state label is HELD, never offered;
//   - label history: an issue whose label history holds ANY event for one of those labels (applied
//     or removed, by anyone), an event for a label the forge no longer names, or an incomplete
//     history is HELD. Current labels alone would let anyone with label-write un-park an
//     undecided issue by removing its label; who cleared a park is statusgen's label-removal hold
//     to decide, so this lane never offers such an issue on its own reading;
//   - scope: only the plan's own repo (the repo --root maps to) is read, so each root's plan
//     offers only its own repo's issues and stamps its own merged-main SHA on them;
//   - dedupe: an issue an issue-loop placeholder in ANY configured root already carries flows
//     through the board lane, and an issue with an open or merged PR (`Issue:` trailer) or a live
//     dispatch claim is HELD;
//   - width: items join the same queue, so the pool width and the class reservation bound them
//     exactly as they bound every other item.
//
// Every could-not-check read HOLDS the issue (or the whole lane), never offers it.

// kindInbox tags an item offered from the addressed issue inbox. classOf buckets it as fresh work.
const kindInbox = "inbox-issue"

// InboxIssue is one open issue addressed to this desk, as the inbox source read it.
type InboxIssue struct {
	Repo   string // owner/name
	Number int
	Title  string
	Labels []string
	Author string
	// Trusted is the trust gate's answer: a trusted author, or an untrusted author with a
	// current blessing read in full.
	Trusted bool
	// TrustUnknown, when non-empty, is why the trust gate could not be read. The issue is held.
	TrustUnknown string
}

// inboxHold is one inbox issue the lane read and did not offer, with the reason.
type inboxHold struct {
	ref    string // owner/name#N
	reason string
}

// CarriedHold is a decision hold carried from an earlier tick: the held item and the decision
// issue that is its source. The plan re-reads the source on every run.
type CarriedHold struct {
	// Item names the held item in any of the forms holdKeys derives: the item ID exactly as plan
	// prints it on its DISPATCH line (`alpha/01`, `<short>--issue-7`), its claim key (`alpha--01`),
	// or the repo-qualified dispatch claim key deskdispatch acquires (`<short>--alpha--01`).
	Item  string
	Repo  deskkit.ForgeRepo
	Issue int
}

// carriedHoldResult is one carried hold after this tick's re-read. matched reports whether the
// hold named an item in this run's queue; a standing hold that matched nothing withheld nothing,
// and the plan says so instead of claiming it held.
type carriedHoldResult struct {
	hold    CarriedHold
	read    deskkit.DecisionHold
	matched bool
}

// inboxItemID is the dispatch key for an inbox issue: the `<short>--issue-<N>` claim-key form
// deskdispatch accepts for issue-only work (with --repo naming the target), so the plan key, the
// claim key and the release key are one string.
func inboxItemID(repo string, n int) string {
	return fmt.Sprintf("%s--issue-%d", deskkit.RepoShortLabel(repo), n)
}

func (it InboxIssue) ref() string { return it.Repo + "#" + strconv.Itoa(it.Number) }

func (it InboxIssue) toItem(targetSHA, role string) loopengine.Item {
	return loopengine.Item{
		ID:        inboxItemID(it.Repo, it.Number),
		TargetSHA: targetSHA,
		Payload: map[string]string{
			"repo":  it.Repo,
			"kind":  kindInbox,
			"issue": strconv.Itoa(it.Number),
			"title": it.Title,
			"to":    role,
		},
	}
}

// labelSetHas reports whether any label is in set (lower-cased, trimmed).
func labelSetHas(labels []string, set map[string]bool) (string, bool) {
	for _, l := range labels {
		if n := strings.ToLower(strings.TrimSpace(l)); set[n] {
			return strings.TrimSpace(l), true
		}
	}
	return "", false
}

// carriesAddress reports whether labels include `to:<role>` (case-insensitive).
func carriesAddress(labels []string, role string) bool {
	want := "to:" + strings.ToLower(strings.TrimSpace(role))
	for _, l := range labels {
		if strings.ToLower(strings.TrimSpace(l)) == want {
			return true
		}
	}
	return false
}

// inboxLabelSets is the label vocabulary the inbox gates hold on: the decision-owed set (plus
// needs-decision itself), help wanted, and the system-state set. It is the file's ONE compiled
// topology read (a declared build-time cap, see internal/topology/secondcell_test.go).
type inboxLabelSets struct {
	decision, helpWanted, systemState map[string]bool
}

func compiledInboxLabelSets() inboxLabelSets {
	t := topology.Compiled()
	decision := t.DecisionOwedLabelSet()
	decision[deskkit.DecisionLabel] = true
	return inboxLabelSets{decision: decision, helpWanted: map[string]bool{"help wanted": true}, systemState: t.SystemStateLabelSet()}
}

// any reports whether name (lower-cased, trimmed) is in any of the three sets.
func (s inboxLabelSets) any(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return s.decision[n] || s.helpWanted[n] || s.systemState[n]
}

// inboxLabelHold returns why an inbox issue's current labels keep it out of dispatch, or "".
func inboxLabelHold(labels []string, sets inboxLabelSets) string {
	if l, ok := labelSetHas(labels, sets.decision); ok {
		return fmt.Sprintf("decision gate: carries %q; a decision-owed issue is never offered", l)
	}
	if l, ok := labelSetHas(labels, sets.helpWanted); ok {
		return fmt.Sprintf("carries %q; it waits on a human, not a worker", l)
	}
	if l, ok := labelSetHas(labels, sets.systemState); ok {
		return fmt.Sprintf("carries system-state label %q; it is not work", l)
	}
	return ""
}

// inboxLabelHistoryHold is the label-removal hold for the inbox lane: the SAME superset test
// issueboard applies (labelHistoryHold), over the labels inboxLabelHold holds on. It returns why
// the history keeps the issue out of dispatch, or "". It never decides WHO removed a label or
// whether a ratifying comment followed: that rule lives once, in statusgen's label-removal hold.
// So the lane errs toward holding: an incomplete history, an event for a label the forge no
// longer names, or ANY event (applied or removed, by anyone) for a held label keeps the issue out.
func inboxLabelHistoryHold(h *deskkit.LabelHistory, sets inboxLabelSets) string {
	if h == nil || !h.Complete {
		return "label history could-not-check: the forge's history is incomplete; an issue whose park " +
			"may have been removed is never offered on its current labels"
	}
	for _, e := range h.Events {
		if strings.TrimSpace(e.Name) == "" {
			return "label-removal hold: the label history holds an event for a label the forge no longer names"
		}
		if sets.any(e.Name) {
			verb := "applied"
			if e.Removed {
				verb = "removed"
			}
			return fmt.Sprintf("label-removal hold: the label history holds an event for %q (%s); "+
				"an issue once parked is never offered on its current labels, the scanner's label-removal hold decides it", e.Name, verb)
		}
	}
	return ""
}

// inboxLane builds the inbox offers for this run. It never returns an error: a failed read is a
// HOLD recorded for the plan output, so one unreadable source cannot fail the whole plan.
func (f *FanoutLoop) inboxLane(role string) []loopengine.Item {
	if f.Inbox == nil || role == "" {
		return nil
	}
	issues, err := f.Inbox()
	if err != nil {
		f.noteInboxLaneHeld(fmt.Sprintf("could not read the addressed inbox (%v)", err))
		return nil
	}
	if f.issueIndex == nil {
		f.noteInboxLaneHeld("no PR index is wired to check each issue for an open or merged PR; " +
			"an inbox issue is never offered unchecked")
		return nil
	}
	if f.LabelHistory == nil {
		f.noteInboxLaneHeld("no label-history reader is wired to apply the label-removal hold; " +
			"an inbox issue is never offered on its current labels alone")
		return nil
	}
	ownRepo := strings.TrimSpace(f.issueBoardRepo)
	if !deskkit.ValidRepoSlug(ownRepo) {
		f.noteInboxLaneHeld(fmt.Sprintf("the plan's own repo %q is not owner/name; the lane offers only "+
			"that repo's issues, so it cannot offer any", ownRepo))
		return nil
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if !strings.EqualFold(issues[i].Repo, issues[j].Repo) {
			return strings.ToLower(issues[i].Repo) < strings.ToLower(issues[j].Repo)
		}
		return issues[i].Number < issues[j].Number
	})
	placeholders, phErr := f.allPlaceholderIssues()
	sets := compiledInboxLabelSets()
	var out []loopengine.Item
	seen := map[string]bool{}
	for _, iss := range issues {
		ref := iss.ref()
		key := strings.ToLower(ref)
		if seen[key] || iss.Number <= 0 || !deskkit.ValidRepoSlug(iss.Repo) {
			continue
		}
		seen[key] = true
		cand := issueflow.CandidateForLabels(issueflow.Candidate{}, iss.Labels)
		if cand.AddressUnknown && carriesAddress(iss.Labels, role) {
			// It names this desk but its address is malformed or conflicting: listed, never offered.
			f.noteInboxHold(ref, "address could-not-check: its to: labels are malformed or conflicting")
			continue
		}
		if !issueflow.Select(issueflow.Inbox, cand, role) {
			continue // not addressed to this desk: not this lane's row
		}
		if !strings.EqualFold(iss.Repo, ownRepo) {
			f.noteInboxHold(ref, "not this plan's repo ("+ownRepo+"); only the plan whose root maps to "+iss.Repo+" offers it")
			continue
		}
		id := inboxItemID(iss.Repo, iss.Number)
		if f.isHandled(id) {
			continue
		}
		if iss.TrustUnknown != "" {
			f.noteInboxHold(ref, "trust could-not-check: "+iss.TrustUnknown)
			continue
		}
		if !iss.Trusted {
			f.noteInboxHold(ref, fmt.Sprintf("QUARANTINED: author %q is untrusted and the issue has no current blessing", iss.Author))
			continue
		}
		if why := inboxLabelHold(iss.Labels, sets); why != "" {
			f.noteInboxHold(ref, why)
			continue
		}
		if phErr != nil {
			f.noteInboxHold(ref, "placeholder could-not-check: "+phErr.Error())
			continue
		}
		if ph, ok := placeholders[key]; ok {
			f.noteInboxHold(ref, "issue-loop placeholder "+ph+" carries it; it dispatches through the board lane")
			continue
		}
		pr, merr := f.issueIndex.Match(deskkit.DispatchIssue{Repo: iss.Repo, Number: iss.Number})
		if merr != nil {
			f.noteInboxHold(ref, "issue PR could-not-check: "+merr.Error())
			continue
		}
		if pr.Unchecked != "" {
			f.noteInboxHold(ref, "issue PR could-not-check: "+pr.Unchecked)
			continue
		}
		if pr.Number != 0 {
			f.noteInboxHold(ref, deskkit.DispatchIssue{Repo: iss.Repo, Number: iss.Number}.Representation(pr))
			continue
		}
		hist, herr := f.LabelHistory(iss.Repo, iss.Number)
		if herr != nil {
			f.noteInboxHold(ref, "label history could-not-check: "+herr.Error())
			continue
		}
		if why := inboxLabelHistoryHold(hist, sets); why != "" {
			f.noteInboxHold(ref, why)
			continue
		}
		held, cerr := f.issueClaimHeld(iss.Repo, iss.Number)
		if cerr != nil {
			f.noteInboxHold(ref, "dispatch claim could-not-check: "+cerr.Error())
			continue
		}
		if held {
			f.noteInboxHold(ref, "live dispatch claim "+id+" is held")
			continue
		}
		out = append(out, iss.toItem(f.TargetSHA, role))
	}
	return out
}

func (f *FanoutLoop) noteInboxHold(ref, reason string) {
	f.mu.Lock()
	f.inboxHolds = append(f.inboxHolds, inboxHold{ref: ref, reason: reason})
	f.mu.Unlock()
}

func (f *FanoutLoop) noteInboxLaneHeld(reason string) {
	f.mu.Lock()
	f.inboxHeld = reason
	f.mu.Unlock()
}

func (f *FanoutLoop) inboxHoldRows() ([]inboxHold, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]inboxHold(nil), f.inboxHolds...), f.inboxHeld
}

// issueClaimHeld reports whether a live dispatch claim names this issue. IssueClaim overrides the
// read; the default lists the root checkout's local claim refs in both claim namespaces.
func (f *FanoutLoop) issueClaimHeld(repo string, n int) (bool, error) {
	if f.IssueClaim != nil {
		return f.IssueClaim(repo, n)
	}
	return localIssueClaimHeld(f.Root, repo, n)
}

// claimRefMatches reports whether ref is the issue's claim ref (exact) or a suffixed re-dispatch
// of it (`--<suffix>`). The `--` boundary keeps issue-1 from matching issue-12.
func claimRefMatches(ref, prefix string) bool {
	r := strings.TrimSpace(ref)
	return r == prefix || strings.HasPrefix(r, prefix+"--")
}

// issueClaimPrefixes are the two namespaces a dispatch claim can live in: where claims are
// acquired today and where the Go claim readers list them.
func issueClaimPrefixes(repo string, n int) []string {
	key := inboxItemID(repo, n)
	return []string{deskkit.DispatchClaimActiveRefsPrefix + key, deskkit.ClaimRefsPrefix + key}
}

// localIssueClaimHeld is the offline claim read: the root checkout's local claim refs, read
// in-process through gitcore (no git binary). An unopenable checkout or an unreadable ref store is
// could-not-check, never "no claim".
func localIssueClaimHeld(root, repo string, n int) (bool, error) {
	r, err := gitcore.Open(root)
	if err != nil {
		return false, fmt.Errorf("cannot open %s to list local claim refs: %w", root, err)
	}
	refs, err := r.Refs()
	if err != nil {
		return false, fmt.Errorf("cannot list local claim refs in %s: %w", root, err)
	}
	for _, prefix := range issueClaimPrefixes(repo, n) {
		for name := range refs {
			if claimRefMatches(name, prefix) {
				return true, nil
			}
		}
	}
	return false, nil
}

// allPlaceholderIssues maps every issue-loop placeholder in this loop's Root AND in every configured
// root to the source issue it carries. A placeholder is not root-local to its issue's repo (its
// frontmatter names the source repo), so a placeholder in another root still carries an issue this
// plan would otherwise offer. Any root that cannot be read is an error: the lane cannot tell which
// issues it carries, so it must not offer any issue as uncarried.
func (f *FanoutLoop) allPlaceholderIssues() (map[string]string, error) {
	out, err := placeholderIssues(f.Root, f.issueBoardRepo)
	if err != nil {
		return nil, err
	}
	listRoots := f.PlaceholderRoots
	if listRoots == nil {
		listRoots = deskkit.ConfiguredRoots
	}
	roots, err := listRoots()
	if err != nil {
		return nil, fmt.Errorf("cannot read the configured roots to check their placeholders: %w", err)
	}
	for _, rc := range roots {
		abs, rerr := deskkit.ResolveRoot(rc)
		if rerr != nil {
			return nil, rerr
		}
		more, perr := placeholderIssues(abs, rc.Repo)
		if perr != nil {
			return nil, perr
		}
		for k, v := range more {
			if _, ok := out[k]; !ok {
				out[k] = rc.Repo + ":" + v
			}
		}
	}
	return out, nil
}

// placeholderIssues maps every issue-loop placeholder under root to the source issue it carries
// (`owner/name#N`, lower-cased -> placeholder id). An unreadable placeholder is an error: the lane
// cannot tell which issue it carries, so it must not offer any issue as uncarried.
func placeholderIssues(root, boardRepo string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(root) == "" {
		return out, nil
	}
	matches, err := filepath.Glob(filepath.Join(root, "docs", "streams", "*", "*issue-*.md"))
	if err != nil {
		return nil, err
	}
	for _, m := range matches {
		stem := strings.TrimSuffix(filepath.Base(m), ".md")
		if deskkit.PlaceholderIssueNumber(stem) == 0 {
			continue
		}
		id := filepath.Base(filepath.Dir(m)) + "/" + stem
		issue, perr := deskkit.DispatchPlaceholder(root, boardRepo, id)
		if perr != nil {
			return nil, perr
		}
		if issue.Number > 0 {
			out[strings.ToLower(issue.Repo+"#"+strconv.Itoa(issue.Number))] = id
		}
	}
	return out, nil
}

// --- carried decision holds -----------------------------------------------------------------

// rereadCarriedHolds re-reads every carried decision hold's source this run and returns the
// results in the order the holds were given. ReadDecisionHold overrides the read (tests); the
// default reads the issue through the session's forge.
func (f *FanoutLoop) rereadCarriedHolds() []carriedHoldResult {
	var out []carriedHoldResult
	for _, h := range f.DecisionHolds {
		var r deskkit.DecisionHold
		if f.ReadDecisionHold != nil {
			r = f.ReadDecisionHold(h.Repo, h.Issue)
		} else {
			fg, err := f.forgeResolver()(h.Repo)
			if err != nil {
				r = deskkit.DecisionHold{Repo: h.Repo, Issue: h.Issue, State: deskkit.DecisionHoldUnknown,
					Why: fmt.Sprintf("decision issue %s#%d could not be read: %v", h.Repo.Slug(), h.Issue, err)}
			} else {
				r = deskkit.RereadDecisionHold(fg, h.Repo, h.Issue)
			}
		}
		out = append(out, carriedHoldResult{hold: h, read: r})
	}
	return out
}

// holdKeys is every spelling a carried hold may use for it: the item ID as plan prints it, its
// claim key (claimKey), and the repo-qualified dispatch claim key deskdispatch acquires for it
// (its claimKeyFor rule: a key already carrying `--` or a qualified selector passes through,
// anything else is prefixed with the repo's short label). All lower-cased.
func holdKeys(it loopengine.Item, defaultRepo string) []string {
	keys := []string{strings.ToLower(it.ID), strings.ToLower(claimKey(it))}
	id := strings.TrimSpace(it.ID)
	if deskkit.NegotiatedWorkSyntax(id) || strings.Contains(id, ":") || strings.Contains(id, "--") {
		return keys
	}
	repo := strings.TrimSpace(it.Payload["repo"])
	if repo == "" {
		repo = strings.TrimSpace(defaultRepo)
	}
	if repo == "" {
		return keys
	}
	qualified := deskkit.RepoShortLabel(repo) + "--" + strings.ReplaceAll(strings.Trim(id, "/"), "/", "--")
	return append(keys, strings.ToLower(qualified))
}

// applyCarriedHolds withholds every queued item whose carried hold still stands this run, and
// records each hold's re-read (and whether it matched a queued item) for the plan output.
func (f *FanoutLoop) applyCarriedHolds(items []loopengine.Item) []loopengine.Item {
	if len(f.DecisionHolds) == 0 {
		return items
	}
	results := f.rereadCarriedHolds()
	// normalised hold key -> every standing hold that names it, so two holds on one item both
	// read as matched.
	standing := map[string][]int{}
	for i, r := range results {
		if r.read.Holds() {
			k := strings.ToLower(strings.TrimSpace(r.hold.Item))
			standing[k] = append(standing[k], i)
		}
	}
	kept := items[:0:0]
	for _, it := range items {
		withheld := false
		for _, k := range holdKeys(it, f.issueBoardRepo) {
			for _, i := range standing[k] {
				results[i].matched = true
				withheld = true
			}
		}
		if !withheld {
			kept = append(kept, it)
		}
	}
	f.mu.Lock()
	f.carriedHolds = results
	f.mu.Unlock()
	return kept
}

func (f *FanoutLoop) carriedHoldRows() []carriedHoldResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]carriedHoldResult(nil), f.carriedHolds...)
}

// carriedHoldLine renders one carried hold's re-read for the plan output.
func carriedHoldLine(r carriedHoldResult) string {
	switch {
	case r.read.State == deskkit.DecisionHoldLifted:
		return fmt.Sprintf("DROPPED HOLD: %s (lift met: %s)", r.hold.Item, r.read.Why)
	case !r.matched:
		return fmt.Sprintf("HOLD UNMATCHED: %s — still standing (%s) but names NO item in this run's queue, so it "+
			"withheld nothing; check the <item> spelling (the DISPATCH item ID, its claim key, or its "+
			"repo-qualified claim key); an item not in this run's queue (in flight, withheld or landed) "+
			"also reads unmatched",
			r.hold.Item, r.read.Why)
	case r.read.State == deskkit.DecisionHoldUnknown:
		return fmt.Sprintf("HOLD COULD-NOT-CHECK: %s — kept, NOT dispatched (%s)", r.hold.Item, r.read.Why)
	default:
		return fmt.Sprintf("HOLD LIVE: %s — NOT dispatched (%s)", r.hold.Item, r.read.Why)
	}
}

// parseCarriedHold parses one `--decision-hold <item>=<owner/name>#<N>` value.
func parseCarriedHold(v string) (CarriedHold, error) {
	item, ref, ok := strings.Cut(v, "=")
	item = strings.TrimSpace(item)
	if !ok || item == "" {
		return CarriedHold{}, fmt.Errorf("--decision-hold %q is not <item>=<owner/name>#<N>", v)
	}
	fr, n, err := deskkit.ParseIssueRef(ref)
	if err != nil {
		return CarriedHold{}, fmt.Errorf("--decision-hold %q: %v", v, err)
	}
	return CarriedHold{Item: item, Repo: fr, Issue: n}, nil
}

// holdList is the repeatable --decision-hold flag.
type holdList []CarriedHold

func (h *holdList) String() string { return fmt.Sprintf("%d hold(s)", len(*h)) }

func (h *holdList) Set(v string) error {
	c, err := parseCarriedHold(v)
	if err != nil {
		return err
	}
	*h = append(*h, c)
	return nil
}

// --- live sources -------------------------------------------------------------------------

// liveInboxIssues reads the open issues addressed to role in the plan's OWN repo (the repo --root
// maps to), through the typed forge, with the same trust gate the issue board applies: a trusted
// author passes; any other author passes only on a current blessing read in full. Reading only the
// own repo keeps one issue from being offered by every root's plan, and keeps each prompt's
// merged-main SHA the SHA of the issue's own repo. An issue whose to: labels name this desk but are
// malformed or conflicting is kept, so the lane lists it rather than dropping it unseen.
func liveInboxIssues(repo, role string, resolve forgeResolver) func() ([]InboxIssue, error) {
	return func() ([]InboxIssue, error) {
		owner, name, ok := strings.Cut(repo, "/")
		if !ok || !deskkit.ValidRepoSlug(repo) {
			return nil, fmt.Errorf("the plan's repo %q is not owner/name", repo)
		}
		fr := deskkit.ForgeRepo{Owner: owner, Name: name}
		fg, err := resolve(fr)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve a forge for %s: %w", repo, err)
		}
		if fg == nil {
			return nil, fmt.Errorf("no forge for %s", repo)
		}
		issues, err := fg.ListOpenIssues(fr)
		if err != nil {
			return nil, fmt.Errorf("cannot list open issues in %s: %w", repo, err)
		}
		var out []InboxIssue
		for _, is := range issues {
			cand := issueflow.CandidateForLabels(issueflow.Candidate{}, is.Labels)
			// A malformed address that names this desk is kept so the lane lists it as held.
			malformed := cand.AddressUnknown && carriesAddress(is.Labels, role)
			if !malformed && !issueflow.Select(issueflow.Inbox, cand, role) {
				continue
			}
			ii := InboxIssue{Repo: repo, Number: is.Number, Title: is.Title, Labels: is.Labels, Author: is.Author.Login}
			if deskkit.TrustedAuthor(is.Author.Login) {
				ii.Trusted = true
			} else if tp, terr := fg.IssueTrustEvents(fr, is.Number); terr != nil || tp == nil {
				ii.TrustUnknown = fmt.Sprintf("cannot read trust events for %s#%d", repo, is.Number)
			} else {
				ii.Trusted = tp.Complete && deskkit.Blessed(tp.BodyEdited, tp.Events)
			}
			out = append(out, ii)
		}
		return out, nil
	}
}

// liveLabelHistory reads one issue's whole label history through the typed forge. A read error is
// could-not-check and holds the issue.
func liveLabelHistory(resolve forgeResolver) func(string, int) (*deskkit.LabelHistory, error) {
	return func(repo string, n int) (*deskkit.LabelHistory, error) {
		owner, name, ok := strings.Cut(repo, "/")
		if !ok {
			return nil, fmt.Errorf("repo %q is not owner/name", repo)
		}
		fr := deskkit.ForgeRepo{Owner: owner, Name: name}
		fg, err := resolve(fr)
		if err != nil {
			return nil, err
		}
		if fg == nil {
			return nil, fmt.Errorf("no forge for %s", repo)
		}
		return fg.IssueLabelHistory(fr, n)
	}
}

// liveIssueClaim reads the issue's dispatch claim refs on the forge, in both claim namespaces.
// A failed listing is could-not-check, never "no claim".
func liveIssueClaim(resolve forgeResolver) func(string, int) (bool, error) {
	return func(repo string, n int) (bool, error) {
		owner, name, ok := strings.Cut(repo, "/")
		if !ok {
			return false, fmt.Errorf("repo %q is not owner/name", repo)
		}
		fr := deskkit.ForgeRepo{Owner: owner, Name: name}
		fg, err := resolve(fr)
		if err != nil {
			return false, err
		}
		if fg == nil {
			return false, fmt.Errorf("no forge for %s", repo)
		}
		for _, prefix := range issueClaimPrefixes(repo, n) {
			refs, lerr := fg.MatchingRefs(fr, prefix)
			if lerr != nil {
				return false, lerr
			}
			for _, r := range refs {
				if claimRefMatches(r, prefix) {
					return true, nil
				}
			}
		}
		return false, nil
	}
}
