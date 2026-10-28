package deskkit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
)

// VerifierAttestationResult retains each target's own admission or uncertainty.
// It grants no execution or publication authority.
type VerifierAttestationResult struct {
	Receipt VerifierReceipt
	Err     error
}

// VerifierAttestationBatchReader returns a planning-only reader when the admitted
// forge supports ordered bulk observations. Unknown decorators, read-only fences
// and other backends keep their existing per-target reader. In particular, this
// does not unwrap an arbitrary Forge or expose a credential or write capability.
func VerifierAttestationBatchReader(f Forge) func(ForgeRepo, []int) map[int]VerifierAttestationResult {
	g := githubPlanningBatchForge(f)
	if g == nil {
		return nil
	}
	return func(repo ForgeRepo, numbers []int) map[int]VerifierAttestationResult {
		return g.verifierAttestations(f, repo, numbers)
	}
}

// Shared optional-capability fence. Never unwrap arbitrary decorators.
func githubPlanningBatchForge(f Forge) *GitHubForge {
	switch v := f.(type) {
	case *GitHubForge:
		return v
	case *outboundForge:
		g, _ := v.Forge.(*GitHubForge)
		return g
	default:
		return nil
	}
}

// Each chunk has four distinct observation boundaries. Never merge the current
// issue read into body history, or read label history before current labels.
// Thirty-two targets keep each bounded connection well below the provider's
// node limit; larger populations use further ordered chunks.
const verifierAttestationBatchCap = 32
const verifierActorSelection = `login __typename ...on User{databaseId} ...on Bot{databaseId}`
const verifierIssueSelection = `number title body state url author{` + verifierActorSelection + `} labels(first:100){pageInfo{hasNextPage} nodes{name}}`
const verifierHistorySelection = issueTrustSelection
const verifierLabelsSelection = `timelineItems(first:100,itemTypes:[LABELED_EVENT,UNLABELED_EVENT]){pageInfo{hasNextPage} nodes{__typename ...on LabeledEvent{createdAt actor{` + verifierActorSelection + `} label{name}} ...on UnlabeledEvent{createdAt actor{` + verifierActorSelection + `} label{name}}}}`

// Fixed documents keep same-operation cost observation inside perf-v3's closed
// query inventory. Target numbers are variables, never interpolated provider data.
var verifierIssueBatchQuery = verifierBatchQuery(verifierIssueSelection)
var verifierHistoryBatchQuery = verifierBatchQuery(verifierHistorySelection)
var verifierLabelsBatchQuery = verifierBatchQuery(verifierLabelsSelection)

func verifierBatchQuery(selection string) string { return githubNumberedBatchQuery("issue", selection) }

func githubNumberedBatchQuery(field, selection string) string {
	var variables, fields strings.Builder
	for slot := 0; slot < verifierAttestationBatchCap; slot++ {
		fmt.Fprintf(&variables, ",$n%d:Int!,$use%d:Boolean!", slot, slot)
		fmt.Fprintf(&fields, "a%d:%s(number:$n%d) @include(if:$use%d){%s} ", slot, field, slot, slot, selection)
	}
	return `query($owner:String!,$name:String!` + variables.String() + `){repository(owner:$owner,name:$name){` + fields.String() + `}}`
}

type verifierBatchObservation struct {
	first, current *Issue
	trust          *TrustPayload
	labels         []LabelEvent
	errs           [4]error
}

// This adapter supplies ONLY the four observations consumed by the existing
// validator. It owns no live Forge: an unexpected read cannot escape to a fresh
// request or accidentally reuse an observation from another admission.
type verifierBatchForge struct {
	repo          ForgeRepo
	number, reads int
	observation   *verifierBatchObservation
}

func (f *verifierBatchForge) GetIssueTyped(repo ForgeRepo, n int, kind TargetKind) (*Issue, error) {
	if repo != f.repo || n != f.number || kind != TargetIssue || f.reads >= 2 {
		return nil, Unverifiable("unexpected batched attestation target or read", nil)
	}
	f.reads++
	stage, issue := 0, f.observation.first
	if f.reads == 2 {
		stage, issue = 2, f.observation.current
	}
	if err := f.observation.errs[stage]; err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, Unverifiable("missing batched attestation issue", nil)
	}
	copy := *issue
	copy.Labels = slices.Clone(issue.Labels)
	return &copy, nil
}
func (f *verifierBatchForge) IssueTrustEvents(repo ForgeRepo, n int) (*TrustPayload, error) {
	if repo != f.repo || n != f.number {
		return nil, Unverifiable("unexpected batched history target", nil)
	}
	if err := f.observation.errs[1]; err != nil {
		return nil, err
	}
	if f.observation.trust == nil {
		return nil, Unverifiable("missing batched body history", nil)
	}
	copy := *f.observation.trust
	copy.Events = slices.Clone(copy.Events)
	return &copy, nil
}
func (f *verifierBatchForge) ListIssueLabelEvents(repo ForgeRepo, n int) ([]LabelEvent, error) {
	if repo != f.repo || n != f.number {
		return nil, Unverifiable("unexpected batched label target", nil)
	}
	if err := f.observation.errs[3]; err != nil {
		return nil, err
	}
	return slices.Clone(f.observation.labels), nil
}

func (g *GitHubForge) verifierAttestations(f Forge, repo ForgeRepo, numbers []int) map[int]VerifierAttestationResult {
	out := map[int]VerifierAttestationResult{}
	ns := slices.Clone(numbers)
	sort.Ints(ns)
	ns = slices.Compact(ns)
	if !ValidRepoSlug(repo.Slug()) || g.onlyRepo != "" && !strings.EqualFold(repo.Slug(), g.onlyRepo) {
		for _, n := range ns {
			out[n] = VerifierAttestationResult{Err: Refused("batched attestation repository is outside credential scope")}
		}
		return out
	}
	valid := ns[:0]
	for _, n := range ns {
		if n <= 0 || int64(n) > 1<<31-1 {
			out[n] = VerifierAttestationResult{Err: Refused("verifier attestation number must be a positive GraphQL Int")}
		} else {
			valid = append(valid, n)
		}
	}
	ns = valid
	for len(ns) > 0 {
		count := min(len(ns), verifierAttestationBatchCap)
		chunk := ns[:count]
		ns = ns[count:]
		observations := map[int]*verifierBatchObservation{}
		for _, n := range chunk {
			observations[n] = &verifierBatchObservation{}
		}
		for stage, query := range []string{verifierIssueBatchQuery, verifierHistoryBatchQuery, verifierIssueBatchQuery, verifierLabelsBatchQuery} {
			raw, failures, err := g.githubReadBatch(repo, chunk, query)
			for _, n := range chunk {
				o := observations[n]
				switch {
				case err != nil:
					o.errs[stage] = err
				case failures[n] != nil:
					o.errs[stage] = failures[n]
				default:
					switch stage {
					case 0, 2:
						issue, more, e := githubVerifierIssue(raw[n], n)
						// Preserve the existing complete issue read for large current label sets.
						// At stage two this remains AFTER body history and BEFORE label history.
						if e == nil && more {
							issue, e = f.GetIssueTyped(repo, n, TargetIssue)
						}
						o.errs[stage] = e
						if stage == 0 {
							o.first = issue
						} else {
							o.current = issue
						}
					case 1:
						o.trust, o.errs[stage] = githubVerifierHistory(raw[n])
					case 3:
						labels, more, e := githubVerifierLabels(raw[n])
						if e == nil && more {
							labels, e = f.ListIssueLabelEvents(repo, n)
						}
						o.labels, o.errs[stage] = labels, e
					}
				}
			}
			if err != nil {
				// In particular, do not compound a scope refusal or rate limit with more
				// batches. Unobserved stages are unavailable, never empty histories.
				for _, n := range chunk {
					for later := stage + 1; later < 4; later++ {
						observations[n].errs[later] = err
					}
				}
				for _, n := range ns {
					out[n] = VerifierAttestationResult{Err: err}
				}
				ns = nil
				break
			}
		}
		for _, n := range chunk {
			reader := &verifierBatchForge{repo: repo, number: n, observation: observations[n]}
			receipt, err := readVerifierAttestation(reader, repo, n)
			out[n] = VerifierAttestationResult{Receipt: receipt, Err: err}
		}
	}
	return out
}

// Alias errors belong to that target only. Errors with no attributable target
// make the whole observation unavailable. No provider error text is echoed.
func (g *GitHubForge) githubReadBatch(repo ForgeRepo, numbers []int, selection string) (map[int]json.RawMessage, map[int]error, error) {
	variables := map[string]any{"owner": repo.Owner, "name": repo.Name}
	aliases := map[string]int{}
	for slot := 0; slot < verifierAttestationBatchCap; slot++ {
		number := 0
		active := slot < len(numbers)
		if active {
			number = numbers[slot]
			aliases[fmt.Sprintf("a%d", slot)] = number
		}
		variables[fmt.Sprintf("n%d", slot)] = number
		variables[fmt.Sprintf("use%d", slot)] = active
	}
	in := map[string]any{"query": selection, "variables": variables}
	var env struct {
		Data   struct{ Repository map[string]json.RawMessage }
		Errors []struct{ Path []any }
	}
	if err := g.doJSON(http.MethodPost, "/graphql", in, &env); err != nil {
		return nil, nil, err
	}
	bad := func() error { return Unverifiable("incomplete or errored batched attestation observation", nil) }
	failures := map[int]error{}
	for _, e := range env.Errors {
		if len(e.Path) < 2 || e.Path[0] != "repository" {
			return nil, nil, bad()
		}
		alias, ok := e.Path[1].(string)
		if !ok {
			return nil, nil, bad()
		}
		n, ok := aliases[alias]
		if !ok {
			return nil, nil, bad()
		}
		failures[n] = bad()
	}
	if env.Data.Repository == nil {
		return nil, nil, bad()
	}
	raw := map[int]json.RawMessage{}
	for alias, n := range aliases {
		v, ok := env.Data.Repository[alias]
		if !ok || string(v) == "null" || len(v) == 0 {
			failures[n] = bad()
		} else {
			raw[n] = v
		}
	}
	return raw, failures, nil
}

func githubVerifierIssue(raw json.RawMessage, n int) (*Issue, bool, error) {
	var w githubEvidenceIssue
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, false, Unverifiable("malformed batched issue", nil)
	}
	if w.Number != n || w.Labels == nil || w.Labels.PageInfo == nil || w.Labels.PageInfo.HasNextPage == nil || w.Labels.Nodes == nil || (w.State != "OPEN" && w.State != "CLOSED") {
		return nil, false, Unverifiable("missing batched issue or label provenance", nil)
	}
	issue := &Issue{Number: w.Number, Title: w.Title, Body: w.Body, State: strings.ToLower(w.State), URL: w.URL, Author: gqlActorAccount(w.Author)}
	for _, label := range w.Labels.Nodes {
		issue.Labels = append(issue.Labels, label.Name)
	}
	return issue, *w.Labels.PageInfo.HasNextPage, nil
}
func githubVerifierHistory(raw json.RawMessage) (*TrustPayload, error) {
	var w githubEvidenceIssue
	var item gqlItem
	if json.Unmarshal(raw, &w) != nil || json.Unmarshal(raw, &item) != nil || w.Comments == nil || w.Comments.PageInfo == nil || w.Comments.PageInfo.HasNextPage == nil || w.Comments.Nodes == nil || !item.bodyHistoryKnown {
		return nil, Unverifiable("missing batched body-history provenance", nil)
	}
	var env gqlEnvelope
	env.Data.Repository.Issue = &item
	trust, err := trustFromEnvelope(env, false)
	return &trust, err
}
func githubVerifierLabels(raw json.RawMessage) ([]LabelEvent, bool, error) {
	var w githubEvidenceIssue
	if json.Unmarshal(raw, &w) != nil || w.TimelineItems == nil || w.TimelineItems.PageInfo == nil || w.TimelineItems.PageInfo.HasNextPage == nil || w.TimelineItems.Nodes == nil {
		return nil, false, Unverifiable("missing batched label-history provenance", nil)
	}
	var labels []LabelEvent
	for _, ev := range w.TimelineItems.Nodes {
		if ev.Typename != "LabeledEvent" && ev.Typename != "UnlabeledEvent" || ev.Label == nil || ev.Label.Name == "" {
			return nil, false, Unverifiable("unknown batched label event", nil)
		}
		actor := gqlActorAccount(ev.Actor)
		labels = append(labels, LabelEvent{Name: ev.Label.Name, AppliedBy: actor.Login, AppliedByID: actor.ID, CreatedAt: ev.CreatedAt, Removed: ev.Typename == "UnlabeledEvent"})
	}
	return labels, *w.TimelineItems.PageInfo.HasNextPage, nil
}
