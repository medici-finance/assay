package main

// projectobligations.go — versioned source obligations and project applicability
// (spec/project-obligations-v1.md; schemas/project-obligations-v1.json).
//
// The chain this file makes checkable:
//
//	permitted source revision -> obligation mapping -> existing REQ ids
//	                                   |
//	                     applicability decision (proposed != accepted)
//
// What it does, and deliberately does NOT do:
//
//   - It does NOT add a requirement lifecycle. A mapping REFERENCES REQ ids and the
//     production parser (parseRequirementsDir) dereferences them; requirements.go
//     keeps the meaning of a REQ and no legacy lint behaviour changes.
//   - It does NOT add an approval mechanism. Acceptance is a LINK to an existing
//     DECISIONS record, bound to the exact subject by digest, and authenticated by
//     the existing offline corroboration seam (decisiongateanchor.go). A decoded
//     JSON object is never a corroboration receipt: trustedReceipt can only be built
//     by trustReceipts, which asks decisionGateCorroboration.
//   - It does NOT read the forge, the clock or the network. asOf is an explicit
//     argument; corroboration state is pre-fetched by the caller and passed in.
//   - It does NOT judge semantic correctness. A source can be correctly identified,
//     correctly permitted and correctly cited, and the interpretation still wrong;
//     that is a qualified reviewer's call, recorded as a review reference.
//
// Layering: obligationCtx is the adapter's output (registers + receipts read from
// disk). validateObligationInput is the CANDIDATE-input validator (shape). The
// authority boundary is resolveObligations -> acceptApplicability: it re-derives
// every binding from the explicit inputs and does not trust that the candidate
// validator ran, so a forged acceptance is refused even with that validator bypassed.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const obligationsSchema = "project-obligations-v1"

// Closed vocabularies.
var (
	poKinds    = []string{"standard", "regulation", "contract", "policy", "other"}
	poUses     = []string{"permitted", "denied", "unknown"}
	poOutcomes = []string{"applicable", "not-applicable", "unresolved"}
)

var (
	poSourceIDRe   = regexp.MustCompile(`^SRC-[a-z0-9][a-z0-9-]{3,38}[a-z0-9]$`)
	poMappingIDRe  = regexp.MustCompile(`^MAP-[a-z0-9][a-z0-9-]{3,38}[a-z0-9]$`)
	poDecisionIDRe = regexp.MustCompile(`^APD-[a-z0-9][a-z0-9-]{3,38}[a-z0-9]$`)
	poDigestRe     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// poMaxParaphrase bounds the human-authored paraphrase: a mapping carries the
// reader's own words, not a copy of the clause.
const poMaxParaphrase = 600

func poIn(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- records

// poRef names one revision of a source or a project profile.
type poRef struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
}

func (r poRef) key() string { return r.ID + "@" + r.Revision }

// poUse is the access / permitted-use record. An absent or "unknown" use is
// treated exactly as denied: it permits authorized metadata only.
type poUse struct {
	AccessRef string `json:"accessRef"`
	Storage   string `json:"storage"`
	AI        string `json:"ai"`
}

func (u poUse) norm(v string) string {
	if poIn(poUses, v) {
		return v
	}
	return "unknown"
}

// poSource is one immutable source revision. A URI alone is not a content version:
// Revision (the edition) is required, and ContentDigest pins content where the use
// record authorizes keeping it.
type poSource struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Issuer        string `json:"issuer"`
	Locator       string `json:"locator"`
	Revision      string `json:"revision"`
	Published     string `json:"published"`
	Effective     string `json:"effective"`
	CapturedAt    string `json:"capturedAt"`
	ContentDigest string `json:"contentDigest,omitempty"`
	Predecessor   string `json:"predecessor,omitempty"` // <id>@<revision>
	Use           poUse  `json:"use"`
	Text          string `json:"text,omitempty"` // only where storage is permitted
}

func (s poSource) key() string { return s.ID + "@" + s.Revision }

// poContext is the optional entity/activity/jurisdiction scope of a mapping.
type poContext struct {
	Entity       string `json:"entity,omitempty"`
	Activity     string `json:"activity,omitempty"`
	Jurisdiction string `json:"jurisdiction,omitempty"`
}

// poMapping is one revision of an obligation mapping.
type poMapping struct {
	ID         string    `json:"id"`
	Revision   int       `json:"revision"`
	Source     poRef     `json:"source"`
	Clause     string    `json:"clause"`
	Paraphrase string    `json:"paraphrase"`
	Reqs       []string  `json:"reqs"`
	Profile    poRef     `json:"profile"`
	Context    poContext `json:"context"`
	Controls   []string  `json:"controls,omitempty"`
	Owner      string    `json:"owner"`
	Supersedes string    `json:"supersedes,omitempty"` // <id>@<revision>
}

func (m poMapping) key() string { return fmt.Sprintf("%s@%d", m.ID, m.Revision) }

// poAcceptance is the acceptance LINK: references to existing decision / review
// records, never a new approval. Nothing in it authenticates by itself.
type poAcceptance struct {
	DecisionID       string `json:"decisionId"`
	SubjectDigest    string `json:"subjectDigest"`
	Disposition      string `json:"disposition"`
	CorroborationRef string `json:"corroborationRef"`
	ReviewRef        string `json:"reviewRef"`
}

// poDecision is an applicability decision. Everything but Acceptance is the
// PROPOSAL; a proposal with no acceptance link is a candidate, not a decision.
type poDecision struct {
	ID              string        `json:"id"`
	MappingID       string        `json:"mappingId"`
	MappingRevision int           `json:"mappingRevision"`
	Source          poRef         `json:"source"`
	Profile         poRef         `json:"profile"`
	Outcome         string        `json:"outcome"`
	Reason          string        `json:"reason,omitempty"`
	Scope           string        `json:"scope"`
	EffectiveFrom   string        `json:"effectiveFrom"`
	EffectiveTo     string        `json:"effectiveTo,omitempty"`
	DecidedAt       string        `json:"decidedAt"`
	ProposedBy      string        `json:"proposedBy"`
	Reviewer        string        `json:"reviewer"`
	Supersedes      string        `json:"supersedes,omitempty"` // an earlier decision id
	Acceptance      *poAcceptance `json:"acceptance,omitempty"`
}

// obligationInput is the explicitly supplied, local, synthetic-or-adopter document.
type obligationInput struct {
	Schema    string       `json:"schema"`
	Sources   []poSource   `json:"sources"`
	Mappings  []poMapping  `json:"mappings"`
	Decisions []poDecision `json:"decisions"`
}

// ---------------------------------------------------------------- digests

func poSum(s string) string {
	h := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(h[:])
}

// poNet encodes one field as a netstring: "<byte length>:<bytes>,". A sequence of
// netstrings is uniquely decodable whatever the field bytes contain (newlines,
// commas, '@', ':'), so the canonical text — and therefore the digest input — is
// injective: two different field tuples can never serialize to the same bytes.
func poNet(s string) string { return fmt.Sprintf("%d:%s,", len(s), s) }

// poNetList encodes a list as ONE field: the netstring of its sorted elements'
// netstrings concatenated, so ["a,b"] and ["a","b"] stay distinct and order never
// changes the identity.
func poNetList(xs []string) string {
	s := append([]string(nil), xs...)
	sort.Strings(s)
	var b strings.Builder
	for _, x := range s {
		b.WriteString(poNet(x))
	}
	return poNet(b.String())
}

// poCanon is the digest of a tag followed by fields, each a netstring (a list
// field arrives already encoded by poNetList and is passed through raw).
func poCanon(fields ...string) string {
	var b strings.Builder
	for _, f := range fields {
		b.WriteString(f)
	}
	return poSum(b.String())
}

// mappingDigest pins the exact mapping revision content (reqs and controls
// sorted, so order never changes the identity). Ids and revisions are separate
// fields, never an "<id>@<revision>" join, so an '@' inside either cannot alias.
func mappingDigest(m poMapping) string {
	return poCanon(
		poNet(obligationsSchema+"/mapping"),
		poNet(m.ID), poNet(fmt.Sprint(m.Revision)),
		poNet(m.Source.ID), poNet(m.Source.Revision),
		poNet(m.Clause), poNet(m.Paraphrase),
		poNetList(m.Reqs),
		poNet(m.Profile.ID), poNet(m.Profile.Revision),
		poNet(m.Context.Entity), poNet(m.Context.Activity), poNet(m.Context.Jurisdiction),
		poNetList(m.Controls),
		poNet(m.Owner),
	)
}

// subjectDigest is the exact subject an acceptance must bind: the current source
// revision (and its content digest), the current mapping revision (and its content
// digest), the profile revision, and EVERY field of the decision proposal — its id,
// outcome and not-applicable reason, scope and effective period, decision time,
// proposer, reviewer and the decision it supersedes. Binding the whole proposal is
// what makes supersession, decision time and the recorded reason part of the
// authorized subject (stream spec §3): rewriting any of them in the input breaks
// the digest the decision record carries.
func subjectDigest(src poSource, m poMapping, d poDecision) string {
	return poCanon(
		poNet(obligationsSchema+"/subject"),
		poNet(src.ID), poNet(src.Revision), poNet(src.ContentDigest),
		poNet(m.ID), poNet(fmt.Sprint(m.Revision)), poNet(mappingDigest(m)),
		poNet(m.Profile.ID), poNet(m.Profile.Revision),
		poNet(d.ID), poNet(d.Outcome), poNet(d.Reason), poNet(d.Scope),
		poNet(d.EffectiveFrom), poNet(d.EffectiveTo), poNet(d.DecidedAt),
		poNet(d.ProposedBy), poNet(d.Reviewer), poNet(d.Supersedes),
	)
}

// ---------------------------------------------------------------- model input

// poModelSource is the ONLY shape a source takes on its way to a model. Text is
// present only in mode "full"; every other mode is authorized metadata.
type poModelSource struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Revision string `json:"revision"`
	Issuer   string `json:"issuer"`
	Mode     string `json:"mode"`  // "full" | "metadata-only"
	Trust    string `json:"trust"` // always "untrusted-data": source text is data, never instructions
	Text     string `json:"text,omitempty"`
}

// aiEligible: a source's content may enter a model only when the use record names
// an access reference AND permits both storage and AI processing. Unknown, denied
// or absent is not eligible.
func aiEligible(s poSource) bool {
	return s.Use.AccessRef != "" && s.Text != "" &&
		s.Use.norm(s.Use.Storage) == "permitted" && s.Use.norm(s.Use.AI) == "permitted"
}

// modelInputs builds the model payload for a source set. It is the single choke
// point: nothing else assembles model input from a source.
func modelInputs(srcs []poSource) []poModelSource {
	out := make([]poModelSource, 0, len(srcs))
	for _, s := range srcs {
		p := poModelSource{ID: s.ID, Kind: s.Kind, Revision: s.Revision, Issuer: s.Issuer,
			Mode: "metadata-only", Trust: "untrusted-data"}
		if aiEligible(s) {
			p.Mode, p.Text = "full", s.Text
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID+"@"+out[i].Revision < out[j].ID+"@"+out[j].Revision
	})
	return out
}

// ---------------------------------------------------------------- candidate validator

func poDate(v string) bool {
	_, err := time.Parse("2006-01-02", v)
	return err == nil
}

func poDateOrUnknown(v string) bool { return v == "unknown" || poDate(v) }

func poSplitKey(k string) (id, rev string, ok bool) {
	i := strings.LastIndex(k, "@")
	if i <= 0 || i == len(k)-1 {
		return "", "", false
	}
	return k[:i], k[i+1:], true
}

// validateObligationInput is the CANDIDATE-input validator: shape, vocabulary,
// identity and lineage of what was supplied. It cannot make a decision accepted;
// resolveObligations does not depend on it having run.
func validateObligationInput(in obligationInput) []string {
	var p []string
	add := func(f string, a ...any) { p = append(p, fmt.Sprintf(f, a...)) }
	if in.Schema != obligationsSchema {
		add("schema %q is not %q", in.Schema, obligationsSchema)
	}

	srcs := map[string]poSource{}
	for _, s := range in.Sources {
		l := "source " + s.key()
		if !poSourceIDRe.MatchString(s.ID) {
			add("%s: id %q is not SRC-<slug>", l, s.ID)
		}
		if !poIn(poKinds, s.Kind) {
			add("%s: kind %q is not one of %s", l, s.Kind, strings.Join(poKinds, ", "))
		}
		if strings.TrimSpace(s.Issuer) == "" || strings.TrimSpace(s.Locator) == "" {
			add("%s: issuer and locator are required", l)
		}
		if strings.TrimSpace(s.Revision) == "" {
			add("%s: revision (edition) is required — a locator alone is not a content version", l)
		}
		if !poDateOrUnknown(s.Published) || !poDateOrUnknown(s.Effective) {
			add("%s: published and effective must be YYYY-MM-DD or the explicit value \"unknown\"", l)
		}
		if !poDate(s.CapturedAt) {
			add("%s: capturedAt %q is not YYYY-MM-DD", l, s.CapturedAt)
		}
		for name, v := range map[string]string{"storage": s.Use.Storage, "ai": s.Use.AI} {
			if v != "" && !poIn(poUses, v) {
				add("%s: use.%s %q is not one of %s", l, name, v, strings.Join(poUses, ", "))
			}
		}
		storage := s.Use.norm(s.Use.Storage) == "permitted"
		if s.Text != "" && (!storage || s.Use.AccessRef == "") {
			add("%s: text is present but the use record does not name an access reference and permit storage — keep licensed text out of the record", l)
		}
		if s.ContentDigest != "" {
			switch {
			case !poDigestRe.MatchString(s.ContentDigest):
				add("%s: contentDigest is not sha256:<64 hex>", l)
			case !storage:
				add("%s: contentDigest present but storage is not permitted — metadata only", l)
			case s.Text != "" && poSum(s.Text) != s.ContentDigest:
				add("%s: contentDigest does not match the supplied text", l)
			}
		} else if s.Text != "" {
			add("%s: text is present without a contentDigest", l)
		}
		if prev, dup := srcs[s.key()]; dup {
			add("%s: duplicate source revision (revisions are immutable)%s", l,
				map[bool]string{true: " with a different content digest", false: ""}[prev.ContentDigest != s.ContentDigest])
		}
		srcs[s.key()] = s
	}
	for _, s := range in.Sources {
		if s.Predecessor == "" {
			continue
		}
		pid, _, ok := poSplitKey(s.Predecessor)
		if !ok || pid != s.ID {
			add("source %s: predecessor %q must be <same id>@<revision>", s.key(), s.Predecessor)
		} else if _, found := srcs[s.Predecessor]; !found || s.Predecessor == s.key() {
			add("source %s: predecessor %q is not an earlier supplied revision", s.key(), s.Predecessor)
		}
	}

	maps := map[string]poMapping{}
	revs := map[string][]int{}
	for _, m := range in.Mappings {
		l := "mapping " + m.key()
		if !poMappingIDRe.MatchString(m.ID) {
			add("%s: id %q is not MAP-<slug>", l, m.ID)
		}
		if m.Revision < 1 {
			add("%s: revision must be >= 1", l)
		}
		if _, ok := srcs[m.Source.key()]; !ok {
			add("%s: source %s is not a supplied source revision", l, m.Source.key())
		}
		if strings.TrimSpace(m.Clause) == "" || strings.TrimSpace(m.Owner) == "" ||
			m.Profile.ID == "" || m.Profile.Revision == "" {
			add("%s: clause, owner and a profile id/revision are required", l)
		}
		if utf8.RuneCountInString(m.Paraphrase) > poMaxParaphrase {
			add("%s: paraphrase exceeds %d characters — a mapping carries a bounded paraphrase, not the clause", l, poMaxParaphrase)
		}
		for _, r := range m.Reqs {
			if !requirementRefInRepoRe.MatchString(r) && !requirementRefCrossRepoRe.MatchString(r) {
				add("%s: %q is not a requirement reference", l, r)
			}
		}
		if _, dup := maps[m.key()]; dup {
			add("%s: duplicate mapping revision", l)
		}
		maps[m.key()] = m
		revs[m.ID] = append(revs[m.ID], m.Revision)
	}
	ids := make([]string, 0, len(revs))
	for id := range revs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		rs := revs[id]
		sort.Ints(rs)
		for i, r := range rs {
			if r != i+1 {
				add("mapping %s: revisions must run 1..n with no gap — a prior revision was dropped or skipped (found %v)", id, rs)
				break
			}
		}
	}
	for _, m := range in.Mappings {
		want := ""
		if m.Revision > 1 {
			want = fmt.Sprintf("%s@%d", m.ID, m.Revision-1)
		}
		if m.Supersedes != want {
			add("mapping %s: supersedes %q, want %q", m.key(), m.Supersedes, want)
		}
	}

	decs := map[string]poDecision{}
	for _, d := range in.Decisions {
		l := "decision " + d.ID
		if !poDecisionIDRe.MatchString(d.ID) {
			add("%s: id is not APD-<slug>", l)
		}
		if _, dup := decs[d.ID]; dup {
			add("%s: duplicate decision id", l)
		}
		decs[d.ID] = d
		if _, ok := maps[fmt.Sprintf("%s@%d", d.MappingID, d.MappingRevision)]; !ok {
			add("%s: mapping %s@%d is not a supplied mapping revision", l, d.MappingID, d.MappingRevision)
		}
		if !poIn(poOutcomes, d.Outcome) {
			add("%s: outcome %q is not one of %s", l, d.Outcome, strings.Join(poOutcomes, ", "))
		}
		if d.Outcome == "not-applicable" && strings.TrimSpace(d.Reason) == "" {
			add("%s: not-applicable needs a reason", l)
		}
		if !poDate(d.EffectiveFrom) || !poDate(d.DecidedAt) || (d.EffectiveTo != "" && !poDate(d.EffectiveTo)) {
			add("%s: effectiveFrom, decidedAt and effectiveTo must be YYYY-MM-DD", l)
		} else if d.EffectiveTo != "" && d.EffectiveTo < d.EffectiveFrom {
			add("%s: effectiveTo precedes effectiveFrom", l)
		}
		if strings.TrimSpace(d.Scope) == "" || strings.TrimSpace(d.ProposedBy) == "" {
			add("%s: scope and proposedBy are required", l)
		}
	}
	for _, d := range in.Decisions {
		if d.Supersedes == "" {
			continue
		}
		if o, ok := decs[d.Supersedes]; !ok || o.MappingID != d.MappingID || d.Supersedes == d.ID {
			add("decision %s: supersedes %q, which is not another decision on the same mapping", d.ID, d.Supersedes)
		}
	}
	sort.Strings(p)
	return p
}

// ---------------------------------------------------------------- adapter (the only I/O)

// trustedReceipt is a corroboration receipt that came from the existing seam. Its
// fields are unexported: a decoded JSON object cannot construct one.
type trustedReceipt struct {
	decisionID string
	file       string // basename of the record file the receipt was minted from
	ref        string // the corroborating issue ("owner/repo#N")
	closer     string // the login that closed it
	sealed     bool
}

// poFileBound reports whether a parsed record is its own file: the frontmatter id
// equals the id the file name carries. Only such a record may stand for its id.
func poFileBound(e decisionEntry) bool {
	id, ok := decisionRecordID(e.File)
	return ok && id == e.ID
}

// admitDecisionRecords is the ONE rule that selects which parsed DECISIONS record
// stands for an id. Both halves of an acceptance's evidence — the body that must
// carry the subject digest and the corroboration receipt — are built from its
// output, so they always come from the same file. A record is admitted only when it
// is file-bound. An id that more than one parsed file claims (whatever the file
// names, whatever order the directory lists them in) is ambiguous: it is held, and
// no file is picked for it.
func admitDecisionRecords(decs []decisionEntry) (admitted map[string]decisionEntry, ambiguous map[string][]string) {
	claims := map[string][]string{}
	for _, e := range decs {
		claims[e.ID] = append(claims[e.ID], e.File)
	}
	admitted, ambiguous = map[string]decisionEntry{}, map[string][]string{}
	for id, files := range claims {
		if len(files) > 1 {
			sort.Strings(files)
			ambiguous[id] = files
		}
	}
	for _, e := range decs {
		if _, held := ambiguous[e.ID]; held || !poFileBound(e) {
			continue
		}
		admitted[e.ID] = e
	}
	return admitted, ambiguous
}

// trustReceipts builds receipts ONLY for admitted records, and ONLY from the issue
// the existing decision-gate seam (decisionGateCorroboratingIssue) names as
// corroborating, over pre-fetched issue state. It never re-selects among the linked
// issues itself: the receipt's ref and closer are exactly the seam's answer. nil
// gates (no state supplied) yields no receipts — applicability then stays
// unresolved; the loader never fetches the forge.
func trustReceipts(admitted map[string]decisionEntry, gates decisionGateLinks) map[string]trustedReceipt {
	out := map[string]trustedReceipt{}
	for id, e := range admitted {
		file := "docs/streams/" + decisionsDirName + "/" + e.File
		iss, _, ok := decisionGateCorroboratingIssue(file, gates)
		if !ok {
			continue
		}
		out[id] = trustedReceipt{decisionID: id, file: e.File, ref: iss.Ref, closer: iss.ClosedBy, sealed: true}
	}
	return out
}

// obligationCtx is everything the authority boundary reads besides the input.
// Decisions holds only admitted (file-bound, unambiguous) records; Ambiguous names
// every id more than one record file claims, with those files.
type obligationCtx struct {
	Reqs      map[string]requirementEntry
	Decisions map[string]decisionEntry
	Ambiguous map[string][]string
	Receipts  map[string]trustedReceipt
}

// loadObligationCtx reads the REQUIREMENTS and DECISIONS registers through their
// production parsers (three-state: an unreadable register is an error, never an
// empty one) and builds receipts from the supplied corroboration state.
func loadObligationCtx(root string, gates decisionGateLinks) (obligationCtx, error) {
	reqs, err := parseRequirementsDir(root)
	if err != nil {
		return obligationCtx{}, fmt.Errorf("requirements register unreadable: %w", err)
	}
	decs, err := parseDecisionsDir(root)
	if err != nil {
		return obligationCtx{}, fmt.Errorf("decisions register unreadable: %w", err)
	}
	admitted, ambiguous := admitDecisionRecords(decs)
	ctx := obligationCtx{Reqs: map[string]requirementEntry{}, Decisions: admitted, Ambiguous: ambiguous,
		Receipts: trustReceipts(admitted, gates)}
	for _, r := range reqs {
		ctx.Reqs[r.ID] = r
	}
	return ctx, nil
}

// readObligationInput reads one explicitly named LOCAL file. A URL is refused; the
// loader has no fetch path. Unknown fields are an error so a typo cannot silently
// drop a binding.
func readObligationInput(path string) (obligationInput, error) {
	var in obligationInput
	if strings.Contains(path, "://") {
		return in, fmt.Errorf("%q: only a local, explicitly supplied file is accepted", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return in, err
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return in, fmt.Errorf("%s: %w", path, err)
	}
	return in, nil
}

// loadObligations is the whole adapter path: read, validate the candidate input,
// resolve through the authority boundary. Candidate problems make the result
// incomplete but never substitute for the boundary's own verdicts.
func loadObligations(root, path, asOf string, gates decisionGateLinks) (obligationResult, error) {
	in, err := readObligationInput(path)
	if err != nil {
		return obligationResult{}, err
	}
	ctx, err := loadObligationCtx(root, gates)
	if err != nil {
		return obligationResult{}, err
	}
	res := resolveObligations(in, ctx, asOf)
	res.Problems = append(validateObligationInput(in), res.Problems...)
	return res, nil
}

// ---------------------------------------------------------------- authority boundary

// Mapping states. Only "accepted" and "not-applicable" count toward a complete
// review; "not-applicable" is still listed (with its reason) in the inventory.
const (
	poAccepted      = "accepted"
	poNotApplicable = "not-applicable"
	poUnresolved    = "unresolved"
	poConflict      = "conflict"
	poRejected      = "rejected"
	poSuperseded    = "superseded"
)

// poVerdict is one mapping revision's resolved state.
type poVerdict struct {
	Mapping    string
	State      string
	Reasons    []string
	DecisionID string
	Reason     string // the recorded not-applicable reason
	History    []string
}

type obligationResult struct {
	Verdicts []poVerdict
	Problems []string
}

// Complete reports whether every CURRENT mapping is accepted or recorded
// not-applicable, with no candidate problems. An empty set is never complete.
func (r obligationResult) Complete() bool {
	cur := 0
	for _, v := range r.Verdicts {
		if v.State == poSuperseded {
			continue
		}
		cur++
		if v.State != poAccepted && v.State != poNotApplicable {
			return false
		}
	}
	return cur > 0 && len(r.Problems) == 0
}

// Accepted lists the mappings that resolved to an accepted applicable decision.
func (r obligationResult) Accepted() []poVerdict {
	var out []poVerdict
	for _, v := range r.Verdicts {
		if v.State == poAccepted {
			out = append(out, v)
		}
	}
	return out
}

// Reasons that leave a decision PENDING (unresolved) rather than refused.
var poPending = map[string]bool{
	"no-acceptance-link":        true,
	"no-trusted-corroboration":  true,
	"outcome-unresolved":        true,
	"outside-effective-period":  true,
	"review-ref-missing":        true,
	"decision-record-ambiguous": true, // two files claim the id: held, never picked
	"review-ref-ambiguous":      true,
}

// acceptApplicability is the lower decision-validation boundary. It returns the
// reasons the decision cannot be accepted for this exact mapping and source; empty
// means accepted. Every binding is re-derived from the arguments.
func acceptApplicability(d poDecision, m poMapping, src poSource, ctx obligationCtx, asOf string) []string {
	var r []string
	add := func(s string) { r = append(r, s) }

	if d.Acceptance == nil {
		return []string{"no-acceptance-link"}
	}
	a := *d.Acceptance

	if d.MappingID != m.ID || d.MappingRevision != m.Revision {
		add("stale-mapping-revision")
	}
	if d.Source != m.Source {
		add("stale-source-revision")
	}
	if d.Profile != m.Profile {
		add("stale-profile-revision")
	}
	switch d.Outcome {
	case "applicable":
	case "not-applicable":
		if strings.TrimSpace(d.Reason) == "" {
			add("not-applicable-needs-reason")
		}
	case "unresolved":
		add("outcome-unresolved")
	default:
		add("unknown-outcome")
	}
	if !poDate(d.EffectiveFrom) || !poDate(d.DecidedAt) || !poDate(asOf) ||
		(d.EffectiveTo != "" && !poDate(d.EffectiveTo)) {
		add("bad-date")
	} else if asOf < d.EffectiveFrom || (d.EffectiveTo != "" && asOf > d.EffectiveTo) {
		add("outside-effective-period")
	} else if d.DecidedAt > asOf {
		add("decision-in-future")
	}
	if strings.TrimSpace(d.Scope) == "" {
		add("scope-missing")
	}
	if strings.TrimSpace(d.Reviewer) == "" || strings.TrimSpace(d.ProposedBy) == "" {
		add("actor-missing")
	} else if strings.EqualFold(d.Reviewer, d.ProposedBy) {
		add("self-approval")
	}

	want := subjectDigest(src, m, d)
	if a.SubjectDigest != want {
		add("subject-digest-mismatch")
	}
	if a.Disposition != "approved" {
		add("disposition-not-approved")
	}

	// The body that must carry the digest and the receipt that corroborates it are
	// re-checked here to be ONE record: the record is its own file, and the receipt
	// was minted from that same file. The boundary does not assume the adapter's
	// admission rule (admitDecisionRecords) ran.
	rec, known := ctx.Decisions[a.DecisionID]
	switch {
	case len(ctx.Ambiguous[a.DecisionID]) > 0:
		add("decision-record-ambiguous")
	case !decisionIDRe.MatchString(a.DecisionID) || !known:
		add("decision-record-unknown")
	case !poFileBound(rec) || rec.ID != a.DecisionID:
		add("decision-record-not-file-bound")
	case !strings.Contains(rec.Body, want):
		add("decision-record-not-subject-bound")
	}

	rc, got := ctx.Receipts[a.DecisionID]
	switch {
	case !got || !rc.sealed || rc.decisionID != a.DecisionID:
		add("no-trusted-corroboration")
	case known && rc.file != rec.File:
		add("receipt-record-mismatch")
	case rc.ref != a.CorroborationRef:
		add("corroboration-ref-mismatch")
	case !strings.EqualFold(rc.closer, d.Reviewer):
		add("reviewer-not-corroborated")
	}

	switch {
	case strings.TrimSpace(a.ReviewRef) == "":
		add("review-ref-missing")
	case a.ReviewRef == a.DecisionID:
		add("review-not-independent")
	case len(ctx.Ambiguous[a.ReviewRef]) > 0:
		add("review-ref-ambiguous")
	default:
		if _, ok := ctx.Decisions[a.ReviewRef]; !ok {
			add("review-ref-unknown")
		}
	}
	return r
}

func poReqReasons(m poMapping, ctx obligationCtx) (reject, hold []string) {
	for _, q := range m.Reqs {
		switch {
		case requirementRefCrossRepoRe.MatchString(q):
			hold = append(hold, "req-cross-repo-unchecked:"+q)
		case !requirementRefInRepoRe.MatchString(q):
			reject = append(reject, "req-malformed:"+q)
		default:
			e, ok := ctx.Reqs[q]
			switch {
			case !ok:
				reject = append(reject, "req-unknown:"+q)
			case strings.TrimSpace(e.Status) == "withdrawn":
				reject = append(reject, "req-withdrawn:"+q)
			}
		}
	}
	return reject, hold
}

// resolveObligations turns explicit inputs into per-mapping verdicts. It is the
// authority boundary and is safe to call with unvalidated input: it re-derives
// identity, lineage and acceptance itself and never assumes the candidate
// validator ran. Unresolved, conflicting or unauthorized inputs never become
// "accepted".
func resolveObligations(in obligationInput, ctx obligationCtx, asOf string) obligationResult {
	var res obligationResult
	srcs := map[string]poSource{}
	for _, s := range in.Sources {
		if _, dup := srcs[s.key()]; dup {
			res.Problems = append(res.Problems, "duplicate source revision "+s.key())
			continue
		}
		srcs[s.key()] = s
	}
	latest := map[string]int{}
	for _, m := range in.Mappings {
		if m.Revision > latest[m.ID] {
			latest[m.ID] = m.Revision
		}
	}
	sorted := append([]poMapping(nil), in.Mappings...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].key() < sorted[j].key() })

	for _, m := range sorted {
		v := poVerdict{Mapping: m.key()}
		if m.Revision != latest[m.ID] {
			v.State = poSuperseded
			res.Verdicts = append(res.Verdicts, v)
			continue
		}
		src, ok := srcs[m.Source.key()]
		reject, hold := poReqReasons(m, ctx)
		if !ok {
			reject = append(reject, "source-unknown:"+m.Source.key())
		}
		if len(reject) > 0 {
			v.State, v.Reasons = poRejected, reject
			res.Verdicts = append(res.Verdicts, v)
			continue
		}
		poDecide(&v, m, src, in.Decisions, ctx, asOf, hold)
		res.Verdicts = append(res.Verdicts, v)
	}
	return res
}

// poDecide fills in the decision-derived state for one current mapping.
func poDecide(v *poVerdict, m poMapping, src poSource, all []poDecision, ctx obligationCtx, asOf string, hold []string) {
	var onRev []poDecision
	for _, d := range all {
		if d.MappingID != m.ID {
			continue
		}
		if d.MappingRevision != m.Revision {
			v.History = append(v.History, fmt.Sprintf("%s bound to stale mapping revision %d", d.ID, d.MappingRevision))
			continue
		}
		onRev = append(onRev, d)
	}
	// A supersedes pointer takes effect ONLY when the decision carrying it is itself
	// accepted: the pointer is inside that decision's subject digest, so it is the
	// authorized decision — never the input — that retires an earlier one. An
	// unaccepted proposal naming a predecessor retires nothing; it is one more
	// current decision, and the mapping is in conflict until it is accepted or
	// withdrawn.
	verdicts := make([][]string, len(onRev))
	supersededBy := map[string]string{}
	for i, d := range onRev {
		verdicts[i] = acceptApplicability(d, m, src, ctx, asOf)
		if d.Supersedes != "" && d.Supersedes != d.ID && len(verdicts[i]) == 0 {
			supersededBy[d.Supersedes] = d.ID
		}
	}
	var cur []poDecision
	var curReasons [][]string
	for i, d := range onRev {
		if by, ok := supersededBy[d.ID]; ok {
			v.History = append(v.History, fmt.Sprintf("%s superseded by %s", d.ID, by))
			continue
		}
		cur = append(cur, d)
		curReasons = append(curReasons, verdicts[i])
	}
	switch {
	case len(cur) == 0:
		v.State, v.Reasons = poUnresolved, append([]string{"no-current-decision"}, hold...)
		return
	case len(cur) > 1:
		ids := make([]string, 0, len(cur))
		for _, d := range cur {
			ids = append(ids, d.ID)
		}
		sort.Strings(ids)
		v.State, v.Reasons = poConflict, []string{"conflicting-decisions:" + strings.Join(ids, ",")}
		return
	}
	d := cur[0]
	v.DecisionID = d.ID
	reasons := append(append([]string(nil), curReasons[0]...), hold...)
	if len(reasons) == 0 {
		v.State = poAccepted
		if d.Outcome == "not-applicable" {
			v.State, v.Reason = poNotApplicable, d.Reason
		}
		return
	}
	v.State, v.Reasons = poUnresolved, reasons
	for _, x := range reasons {
		if !poPending[x] && !strings.HasPrefix(x, "req-cross-repo-unchecked:") {
			v.State = poRejected
			return
		}
	}
}
