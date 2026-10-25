package deskkit

// briefadmission.go — the two brief-side admission conditions a dispatcher and the board
// that ranks work for it must agree on (medici-finance/assay#2744):
//
//  1. EXEC TIER. A brief's `exec-tier:` is the minimum execution tier it demands. A dispatch
//     at a weaker `--tier` must not take it. TierCovers is the ONE comparison, over the same
//     closed vocabulary the model stamp validates against (execTierValues, weakest first).
//
//  2. AN UNRULED DECISION. A `gate: human` brief whose decision issue is open, carries the
//     decision label and has no ruling recorded on it is waiting on a human; a dispatch
//     spends a claim cycle and a worker slot finding that out. BriefDecisionWait names the
//     decision issue (the brief's `decision-issue:` frontmatter, else the issue carrying the
//     brief's hidden decision-gate marker) and reads whether it is still waiting.
//
// Both `deskdispatch` (refuses before the claim) and `deskboard dispatch` (drops the row)
// call these, so the ranked list and the dispatcher cannot disagree about which rows are
// startable — the depth defect medici-finance/assay#1958 describes.
//
// A READ THAT FAILS DOES NOT HOLD. A decision whose state cannot be read is reported as
// unknown and the caller proceeds with a note, the same convention as the pre-dispatch
// review gate: the human gate still binds approval downstream, so a missed hold costs a
// slot, while a false hold on a read failure would stop every human-gated brief.

import (
	"github.com/medici-finance/assay/statusgen/briefdoc"

	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DecisionLabel is the label a decision issue carries while it waits for a human.
const DecisionLabel = "needs-decision"

// DecidedLabel is the label a decision issue carries once the human has decided it.
const DecidedLabel = "human-decided"

// DecisionGateMarkerPrefix is the hidden-marker key the decision-issue helper writes into
// the issue body (`<!-- decision-gate: <brief id> -->`) and dedupes on.
const DecisionGateMarkerPrefix = "decision-gate"

// TierCovers reports whether a dispatch at dispatchTier may take a brief whose `exec-tier:`
// is briefTier. An absent briefTier is `any`, the schema default. The order is the model
// floor's tierRank (modelfloor.go), so the two comparisons cannot disagree. A value outside
// the vocabulary on either side is an error: an unrecognised tier is not evidence of either
// strength.
func TierCovers(dispatchTier, briefTier string) (bool, error) {
	d, ok := CanonicalDispatchTier(dispatchTier)
	if !ok {
		return false, fmt.Errorf("dispatch tier %q is outside the tier vocabulary (%s)", dispatchTier, strings.Join(DispatchTiers(), "|"))
	}
	b := strings.Trim(strings.TrimSpace(briefTier), `"'`)
	if b == "" {
		b = "any"
	}
	b, ok = CanonicalDispatchTier(b)
	if !ok {
		return false, fmt.Errorf("brief exec-tier %q is outside the tier vocabulary (%s)", briefTier, strings.Join(DispatchTiers(), "|"))
	}
	return tierRank(d) >= tierRank(b), nil
}

// BriefAdmission is the brief frontmatter the admission conditions read.
type BriefAdmission struct {
	ID            string // `brief:` — the id the decision-gate marker carries
	ExecTier      string // `exec-tier:`, "" when absent
	Gate          string // `gate:`, lower-cased, "" when absent
	DecisionIssue int    // `decision-issue:`, 0 when absent or not a positive number
	Frontmatter   bool   // the content opened with a frontmatter fence
}

// GatesHuman reports whether the brief declares `gate: human`.
func (b BriefAdmission) GatesHuman() bool { return b.Gate == "human" }

// ParseBriefAdmission reads the admission fields from a brief file's content.
func ParseBriefAdmission(content string) BriefAdmission {
	front, _, err := briefdoc.Split(content)
	if err != nil {
		return BriefAdmission{}
	}
	f, err := briefdoc.ParseFields(front)
	if err != nil {
		return BriefAdmission{Gate: "human", ExecTier: "strong"}
	}
	b := BriefAdmission{Frontmatter: true, ID: f.Values["brief"], ExecTier: f.Values["exec-tier"], Gate: strings.ToLower(f.Values["gate"])}
	if n, err := strconv.Atoi(strings.TrimPrefix(f.Values["decision-issue"], "#")); err == nil && n > 0 {
		b.DecisionIssue = n
	}
	return b
}

// DecisionForge is the read-only forge surface BriefDecisionWait uses. Every method is one
// Forge already has.
type DecisionForge interface {
	GetIssue(repo ForgeRepo, number int) (*Issue, error)
	SearchIssues(repo ForgeRepo, in SearchIssuesInput) ([]IssueSearchResult, error)
	ListIssueLabelEvents(repo ForgeRepo, number int) ([]LabelEvent, error)
	IssueTrustEvents(repo ForgeRepo, number int) (*TrustPayload, error)
}

// RulingState is whether a ruling is recorded on an item carrying the decision label.
type RulingState int

const (
	RulingUnknown RulingState = iota
	RulingRecorded
	RulingMissing
)

// RulingRecordedSince decides whether the ratifying identity has written on an item since
// the decision label was last applied to it. events is the item's label history in order;
// trust is its comments and reviews. Recorded: a comment or review by the ratifying identity
// created after that application. Missing: the history is complete and holds none. Unknown:
// the application cannot be dated, or the history is incomplete and holds none.
func RulingRecordedSince(events []LabelEvent, trust *TrustPayload, isRatifier func(string, int64) bool) RulingState {
	var applied time.Time
	standing := false
	for _, e := range events {
		if !strings.EqualFold(strings.TrimSpace(e.Name), DecisionLabel) {
			continue
		}
		standing = !e.Removed
		if standing {
			t, err := time.Parse(time.RFC3339, e.CreatedAt)
			if err != nil {
				return RulingUnknown
			}
			applied = t
		}
	}
	if !standing || applied.IsZero() || trust == nil {
		return RulingUnknown
	}
	for _, ev := range trust.Events {
		if isRatifier(ev.Author, ev.AuthorID) && ev.CreatedAt.After(applied) {
			return RulingRecorded
		}
	}
	if !trust.Complete {
		return RulingUnknown
	}
	return RulingMissing
}

// DecisionWait is the answer BriefDecisionWait gives for one brief.
type DecisionWait struct {
	// Issue is the decision issue read, 0 when the brief names none and no issue carries its
	// marker.
	Issue int
	// Waiting is true only when that issue is open, carries the decision label, carries no
	// decided label, and the ratifying identity has written nothing on it since the label
	// was applied.
	Waiting bool
	// Unknown is a sentence saying which read could not be completed; Waiting is then false.
	Unknown string
}

var decisionMarkerRe = regexp.MustCompile(DecisionGateMarkerPrefix + `:\s*([^\s>]+)`)

// BriefDecisionWait names the brief's decision issue and reads whether it still waits on a
// ruling. isRatifier answers who records a ruling; nil means no ratifying identity is
// configured, so no ruling state can be read. A brief that names no decision issue and whose
// marker no issue carries is not waiting: the dispatch is what files that issue.
func BriefDecisionWait(f DecisionForge, fr ForgeRepo, b BriefAdmission, isRatifier func(string, int64) bool) DecisionWait {
	n := b.DecisionIssue
	if n == 0 {
		var why string
		n, why = decisionIssueByMarker(f, fr, b.ID)
		if why != "" {
			return DecisionWait{Unknown: why}
		}
		if n == 0 {
			return DecisionWait{}
		}
	}
	w := DecisionWait{Issue: n}
	item, err := f.GetIssue(fr, n)
	if err != nil || item == nil {
		w.Unknown = fmt.Sprintf("decision issue %s#%d could not be read", fr.Slug(), n)
		return w
	}
	if !strings.EqualFold(strings.TrimSpace(item.State), "open") || !hasLabel(item.Labels, DecisionLabel) || hasLabel(item.Labels, DecidedLabel) {
		return w
	}
	if isRatifier == nil {
		w.Unknown = fmt.Sprintf("decision issue %s#%d is open and no ratifying identity is configured to read its ruling", fr.Slug(), n)
		return w
	}
	events, err := f.ListIssueLabelEvents(fr, n)
	if err != nil {
		w.Unknown = fmt.Sprintf("decision issue %s#%d's label history could not be read", fr.Slug(), n)
		return w
	}
	trust, err := f.IssueTrustEvents(fr, n)
	if err != nil {
		w.Unknown = fmt.Sprintf("decision issue %s#%d's comments could not be read", fr.Slug(), n)
		return w
	}
	switch RulingRecordedSince(events, trust, isRatifier) {
	case RulingMissing:
		w.Waiting = true
	case RulingUnknown:
		w.Unknown = fmt.Sprintf("whether a ruling is recorded on decision issue %s#%d could not be read", fr.Slug(), n)
	}
	return w
}

// decisionIssueByMarker finds the lowest-numbered issue whose body carries the brief's
// decision-gate marker, under either spelling of the brief id — the same match the
// decision-issue helper dedupes on. It returns 0 and no reason when none does.
func decisionIssueByMarker(f DecisionForge, fr ForgeRepo, id string) (int, string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, ""
	}
	spellings := decisionBriefSpellings(id)
	seen := map[int]bool{}
	var candidates []int
	for _, s := range spellings {
		hits, err := f.SearchIssues(fr, SearchIssuesInput{DecisionGateMarkers: true, DecisionGateBrief: s})
		if err != nil {
			return 0, decisionSearchUnknown(fr, id, err)
		}
		for _, h := range hits {
			if decisionBodyHasSpelling(h.Body, s) && !seen[h.Number] {
				seen[h.Number] = true
				candidates = append(candidates, h.Number)
			}
		}
	}
	return decisionIssueFromCandidates(f, fr, id, candidates)
}

// decisionIssueFromCandidates confirms marker membership on fresh issue reads;
// search bodies are an index, not authoritative membership. Shared sweeps reuse
// this same lowest-numbered-candidate rule as the single-brief dispatcher.
func decisionIssueFromCandidates(f DecisionForge, fr ForgeRepo, id string, candidates []int) (int, string) {
	sort.Ints(candidates)
	for _, c := range candidates {
		item, err := f.GetIssue(fr, c)
		if err != nil || item == nil {
			return 0, fmt.Sprintf("a candidate decision issue %s#%d for %s could not be read", fr.Slug(), c, id)
		}
		for _, spelling := range decisionBriefSpellings(id) {
			if decisionBodyHasSpelling(item.Body, spelling) {
				return c, ""
			}
		}
	}
	return 0, ""
}

func decisionBodyHasSpelling(body, spelling string) bool {
	for _, m := range decisionMarkerRe.FindAllStringSubmatch(body, -1) {
		if m[1] == spelling {
			return true
		}
	}
	return false
}

func decisionBriefSpellings(id string) []string {
	spellings := []string{id}
	if alt := otherDecisionBriefSpelling(id); alt != "" {
		spellings = append(spellings, alt)
	}
	return spellings
}

var (
	decisionSlashIDRe = regexp.MustCompile(`^([A-Za-z0-9_-]+)/([0-9]+[A-Za-z]?)$`)
	decisionColonIDRe = regexp.MustCompile(`^assay:[A-Za-z0-9_-]+:([A-Za-z0-9_-]+):([0-9]+[A-Za-z]?)$`)
)

// normalizeDecisionBriefID reduces both brief-id spellings to `stream:NN`, as the helper's
// dedupe does; an id of neither shape is compared as written.
func normalizeDecisionBriefID(id string) string {
	if m := decisionSlashIDRe.FindStringSubmatch(id); m != nil {
		return m[1] + ":" + m[2]
	}
	if m := decisionColonIDRe.FindStringSubmatch(id); m != nil {
		return m[1] + ":" + m[2]
	}
	return id
}

// otherDecisionBriefSpelling is the legacy `<stream>/<NN>` spelling of a colon id, "" otherwise.
func otherDecisionBriefSpelling(id string) string {
	if m := decisionColonIDRe.FindStringSubmatch(id); m != nil {
		return m[1] + "/" + m[2]
	}
	return ""
}

func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), want) {
			return true
		}
	}
	return false
}
