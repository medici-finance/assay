package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// decisionLabel is the GitHub label for human-decision issues.
// Distinct from verify-gate (verify stage) — this is the DECISION stage: a brief
// whose gate question must be answered before/while work proceeds.
const decisionLabel = "needs-decision"

// decisionIssue is one emitted element of the --decision-issues JSON array — a
// self-contained GitHub issue payload for a brief awaiting a human decision
// (gate:human + implemented/verified, no existing decision issue). The workflow
// feeds title/labels/body straight to `gh issue create`; marker is the
// idempotency key.
type decisionIssue struct {
	Card   gateCard `json:"card"`
	Brief  string   `json:"brief"`
	Title  string   `json:"title"`
	Labels []string `json:"labels"`
	Marker string   `json:"marker"`
	Body   string   `json:"body"`
}

// decisionMarkerRe matches a hidden idempotency marker in an issue body (or a
// bare marker line). Tolerant of raw issue bodies so the workflow can pipe
// `gh issue list --json body` output straight in.
var decisionMarkerRe = regexp.MustCompile(`<!-- needs-decision: [^>]*? -->`)

// decisionMarker renders the hidden per-brief marker. It is the first line of
// the issue body (idempotency).
func decisionMarker(brief string) string {
	return "<!-- needs-decision: " + brief + " -->"
}

// loadDecisionMarkers reads the --decision-markers file and returns the set of
// decision markers already present in existing issues. A missing/empty path or
// a non-existent file yields an empty set (nothing exists yet). It extracts
// every `<!-- needs-decision: … -->` occurrence, so it accepts either one marker
// per line OR raw issue bodies.
func loadDecisionMarkers(path string) (map[string]bool, error) {
	set := map[string]bool{}
	if path == "" {
		return set, nil
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return set, nil
	}
	if err != nil {
		return nil, err
	}
	for _, m := range decisionMarkerRe.FindAllString(string(raw), -1) {
		set[strings.TrimSpace(m)] = true
	}
	return set, nil
}

// renderDecisionBody builds the self-contained markdown issue body for a brief
// awaiting a human decision. It follows the decision-issue template from the
// issue-loop stream README: Situation (prose), Options (pros/cons), What happens
// on each answer, and links. Fully offline: everything is lifted from the brief
// file plus the static repo slug.
func renderDecisionBody(root string, bf *BriefFile, row *Brief) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", decisionMarker(bf.Brief)) // briefid:raw identity marker; open issues carry the raw id, dedupe must match
	b.WriteString(renderCardLinks(makeGateCard(root, bf, cardDecision)))

	// Situation
	fmt.Fprintf(&b, "## Situation\n\n")
	fmt.Fprintf(&b, "**%s** has been **%s** and is awaiting a human decision. This decision does not accept the later Verify results or mark the brief `done`.\n\n", bf.Title, row.Status)

	// gate-why as the situation prose
	if gw := strings.TrimSpace(bf.GateWhy); gw != "" {
		fmt.Fprintf(&b, "> **Why this needs your decision:** %s\n\n", gw)
	}

	// why: the brief's VALUE rationale — surfaced above the
	// mechanical gate-reason line so a human reads the real motivation first.
	// Omitted (no error) when absent — pre-backfill briefs simply show no why.
	if w := strings.TrimSpace(bf.Why); w != "" {
		fmt.Fprintf(&b, "> **Why this work exists:** %s\n\n", w)
	}

	reasons := gateReasons(bf.Risk)
	fmt.Fprintf(&b, "**Gate reason** — `gate: human` because these risk answers are `yes`: %s.\n\n",
		strings.Join(reasons, ", "))

	// Options with pros/cons
	fmt.Fprintf(&b, "## Options\n\n")

	fmt.Fprintf(&b, "### Option A: Sign off — accept the brief as-is\n\n")
	fmt.Fprintf(&b, "The brief proceeds through verification and eventually to `done`.\n\n")
	fmt.Fprintf(&b, "**Why we'd want it:** the proposed direction and documented risks are acceptable. Verification results require their own independent check and final acceptance.\n\n")
	fmt.Fprintf(&b, "**What it limits or costs:** signing off on a human-gated brief means accepting the risks (%s) the brief documents. Once accepted, the work enters the verification queue and moves toward irreversibility.\n\n", strings.Join(reasons, ", "))

	fmt.Fprintf(&b, "### Option B: Request changes\n\n")
	fmt.Fprintf(&b, "The brief is held at its current status (`%s`) until corrective work is done.\n\n", row.Status)
	fmt.Fprintf(&b, "**Why we'd want it:** there are gaps in the implementation, evidence, or risk mitigation that need addressing before a human can sign off.\n\n")
	fmt.Fprintf(&b, "**What it limits or costs:** delays the brief and anything it unblocks. Each round trip adds latency to the delivery pipeline.\n\n")

	// What happens on each answer
	fmt.Fprintf(&b, "## What happens on each answer\n\n")
	fmt.Fprintf(&b, "| Answer | Effect |\n")
	fmt.Fprintf(&b, "|--------|--------|\n")
	fmt.Fprintf(&b, "| Option A (sign off) | Brief can proceed to verification; a verify-gate issue will be filed at `verified` for the final done-close |\n")
	fmt.Fprintf(&b, "| Option B (request changes) | Brief stays at `%s`; implementer addresses feedback and resubmits for review |\n", row.Status)

	// Unblocks context
	if len(bf.Unblocks) > 0 {
		fmt.Fprintf(&b, "\n**Blocked by this brief:** %s\n", strings.Join(bf.Unblocks, ", "))
	}

	// Links
	fmt.Fprintf(&b, "\n## Links\n\n")
	rel, err := filepath.Rel(root, bf.Path)
	if err != nil {
		rel = bf.Path
	}
	fmt.Fprintf(&b, "- **Brief:** [%s](https://github.com/%s/blob/main/%s)\n",
		bf.Brief, verifyRepoSlug(), filepath.ToSlash(rel))

	body := briefBody(bf.Path)
	if prs := prLinks(body); len(prs) > 0 {
		fmt.Fprintf(&b, "- **PR:** %s\n", strings.Join(prs, ", "))
	}

	// Evidence section (verbatim from brief)
	if ev := strings.TrimSpace(bf.Evidence); ev != "" {
		fmt.Fprintf(&b, "\n### Recorded results (Evidence)\n\n%s\n", ev)
	}

	// Verify section (verbatim from brief)
	if verify := strings.TrimSpace(extractSectionByPrefix(body, "Verify")); verify != "" {
		fmt.Fprintf(&b, "\n### Verify\n\n%s\n", verify)
	}

	// Decider instructions
	fmt.Fprintf(&b, "\n### Decider\n\n")
	fmt.Fprintf(&b, "Close this issue with a comment stating the chosen option (A or B). Only a verified human account is honored.\n")

	return b.String()
}

// decisionIssues computes the newly-eligible decision issues: briefs that are
// gate: human, whose README-row status is implemented or verified, whose
// marker is not already in the supplied existing-markers set, and which do
// NOT already carry a decision-issue in frontmatter (manually backfilled).
// Output is sorted by brief id for deterministic emission.
func decisionIssues(root string, streams []*Stream, existing map[string]bool) []decisionIssue {
	out := []decisionIssue{}
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // malformed (reported elsewhere) or legacy/opted-out
			}
			if bf.Gate != "human" {
				continue
			}
			// Skip briefs that already carry a decision-issue in frontmatter
			// — they were backfilled manually and
			// should never receive a second issue from --decision-issues.
			if bf.DecisionIssue != 0 {
				continue
			}
			_, num, okName := expectedBriefID(path)
			if !okName {
				continue
			}
			row := findRow(s, num)
			if row == nil {
				continue
			}
			if row.Status != "implemented" && row.Status != "verified" {
				continue
			}
			marker := decisionMarker(bf.Brief) // briefid:raw identity marker; open issues carry the raw id, dedupe must match
			if existing[marker] {
				continue
			}
			out = append(out, decisionIssue{
				Card:   makeGateCard(root, bf, cardDecision),
				Brief:  bf.Brief,
				Title:  issueTitle("needs-decision: ", bf.Brief, bf.Title),
				Labels: []string{decisionLabel},
				Marker: marker,
				Body:   renderDecisionBody(root, bf, row),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Brief < out[j].Brief })
	return out
}

// nextUpDecisionNotices covers the "top-of-Next-up" leg of the
// part (a) lint: a gate:human brief picked for Next-up while still `todo` has
// no decision issue — the human gate is about to be hit with nothing filed.
// Dispatched (in-progress) and awaiting (implemented/verified) briefs are
// covered status-wide in checkBriefFiles; restricting the todo leg to actual
// picks keeps every backlog gate:human todo from flooding the register.
// Placeholder picks have no brief file and are skipped.
func nextUpDecisionNotices(nu NextUp) []string {
	var out []string
	for _, p := range nu.Picks {
		if p.Brief.Status != "todo" || p.Brief.Schema == "placeholder-v1" {
			continue
		}
		for _, path := range briefFilePaths(p.Stream) {
			_, num, okName := expectedBriefID(path)
			if !okName || num != p.Brief.Num {
				continue
			}
			if bf, ok, err := parseBriefFile(path); err == nil && ok &&
				bf.Gate == "human" && bf.DecisionIssue == 0 {
				out = append(out, fmt.Sprintf("%s: brief %s is gate:human and picked for Next-up at todo but has no decision-issue — file one via --decision-issues", path, bf.Brief))
			}
			break
		}
	}
	return out
}

// runDecisionIssues is the --decision-issues entrypoint: emit the eligible-brief
// JSON array to stdout. Returns a process exit code.
func runDecisionIssues(root, markersPath string) int {
	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	existing, err := loadDecisionMarkers(markersPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: reading markers:", err)
		return 1
	}
	issues := decisionIssues(root, streams, existing)
	enc, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Println(string(enc))
	return 0
}

// gateCard extends both emitter payloads. Links are observations only: they
// cannot combine human-authorized acts, satisfy a Verify row or write a stamp.
const (
	cardDecision   = "decision"
	cardAcceptance = "verification-acceptance"
	cardPrefix     = "<!-- assay-card: "
)

type gateCard struct {
	Schema         string `json:"schema"`
	Purpose        string `json:"purpose"`
	Brief          string `json:"brief"`
	Local          string `json:"local"`
	Repository     string `json:"repository,omitempty"`
	ID             string `json:"id,omitempty"`
	Version        int    `json:"version"`
	SourceRevision string `json:"source_revision,omitempty"`
	SourceURL      string `json:"source_url,omitempty"`
	VerifyRevision string `json:"verification_revision,omitempty"`
	VerifyURL      string `json:"verification_url,omitempty"`
	DecisionURL    string `json:"decision_url,omitempty"`
	AcceptanceURL  string `json:"acceptance_url,omitempty"`
	Unavailable    string `json:"could_not_check,omitempty"`
}

func makeGateCard(root string, bf *BriefFile, purpose string) gateCard {
	c := gateCard{Schema: "gate-card/v1", Purpose: purpose, Repository: verifyRepoSlug(), ID: bf.ID, Version: max(bf.Version, 1)}
	var err error
	c.Brief, c.Local, err = cardBriefIdentity(root, bf)
	if err != nil {
		c.Unavailable = "brief identity unresolved"
		return c
	}
	rel, err := filepath.Rel(root, bf.Path)
	if err != nil {
		c.Unavailable = "brief path unresolved"
		return c
	}
	rel = filepath.ToSlash(rel)
	source := gitCurrentSHA(root)
	var committed []byte
	var readErr error
	if source != "" {
		committed, readErr = gitShowObject(root, source, rel)
	}
	current, diskErr := os.ReadFile(bf.Path)
	if source != "" && readErr == nil && diskErr == nil && bytes.Equal(committed, current) {
		c.SourceRevision = source
		if c.Repository != "" {
			c.SourceURL = fmt.Sprintf("https://github.com/%s/blob/%s/%s", c.Repository, source, rel)
		}
	} else {
		c.Unavailable = "source revision unavailable or brief differs from HEAD"
	}
	if pass, err := flipLatestPass(bf.Evidence); err == nil {
		c.VerifyRevision = pass.SHA
		// This points to the recorded run, not a newly certified witness.
		if c.Repository != "" {
			c.VerifyURL = fmt.Sprintf("https://github.com/%s/tree/%s", c.Repository, pass.SHA)
		}
	}
	if c.Repository != "" {
		base := "https://github.com/" + c.Repository
		c.DecisionURL = base + "/issues?q=" + url.QueryEscape("is:issue label:needs-decision in:body \""+c.Local+"\"")
		if bf.DecisionIssue > 0 {
			c.DecisionURL = fmt.Sprintf("%s/issues/%d", base, bf.DecisionIssue)
		}
		c.AcceptanceURL = base + "/issues?q=" + url.QueryEscape("is:issue label:verify-gate in:body \""+c.Local+"\"")
	}
	return c
}

func renderCardLinks(c gateCard) string {
	raw, _ := json.Marshal(c) // all fields have JSON's fixed, encodable types
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s -->\n\n**Card purpose:** `%s`. Related records remain separate; links grant no acceptance authority.\n\n", cardPrefix, raw, c.Purpose)
	if c.Brief != "" {
		fmt.Fprintf(&b, "**Canonical brief:** `%s` (revision %d).\n\n", c.Brief, c.Version)
	}
	for _, link := range []struct{ label, url string }{{"Decision record (or search)", c.DecisionURL}, {"Verification acceptance records (search)", c.AcceptanceURL}, {"Brief source revision", c.SourceURL}, {"Recorded verification revision (not acceptance)", c.VerifyURL}} {
		if link.url != "" {
			fmt.Fprintf(&b, "- [%s](%s)\n", link.label, link.url)
		}
	}
	if c.Unavailable != "" {
		fmt.Fprintf(&b, "\nLink provenance: could-not-check — %s.\n", c.Unavailable)
	}
	b.WriteString("\nA closed earlier decision does not satisfy verification. Final acceptance still requires the independent verification prerequisite and the existing human-only close path. Combined decision/acceptance cards are not supported.\n\n")
	return b.String()
}

// readGateCard is opt-in: legacy markers remain readable, but a present typed
// card must be one literal, complete record. Quoted, duplicate and unknown
// records cannot silently fall back to legacy authority.
func readGateCard(body string) (*gateCard, error) {
	if !strings.Contains(body, "assay-card:") {
		return nil, nil
	}
	if strings.Count(body, "assay-card:") != 1 {
		return nil, fmt.Errorf("ambiguous card links")
	}
	for _, line := range strings.Split(stripCode(body), "\n") {
		if !strings.HasPrefix(line, cardPrefix) || !strings.HasSuffix(line, " -->") {
			continue
		}
		raw := strings.TrimSuffix(strings.TrimPrefix(line, cardPrefix), " -->")
		var c gateCard
		dec := json.NewDecoder(strings.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&c); err != nil {
			return nil, err
		}
		if (c.Purpose == cardDecision && verifyMarkerRe.MatchString(body)) || (c.Purpose == cardAcceptance && (decisionGateMarkerRe.MatchString(body) || decisionMarkerRe.MatchString(body))) {
			return nil, fmt.Errorf("combined cards are unsupported")
		}
		if c.Schema != "gate-card/v1" || (c.Purpose != cardDecision && c.Purpose != cardAcceptance) || c.Brief == "" || !refInRepoBriefRe.MatchString(c.Local) || c.Version < 1 {
			return nil, fmt.Errorf("invalid card links")
		}
		// Canonical identity may be local or v2; neither may name a different item.
		if c.Brief != c.Local {
			// Keep the full ID (including alias) for the existing repository-aware
			// classifier; this check only checks internal consistency, never resolves it.
			_, alias, stream, num, ok := parseBriefV2ID(c.Brief)
			if !ok || alias == "" || stream+"/"+num != c.Local {
				return nil, fmt.Errorf("mixed card identities")
			}
		}
		if dec.Decode(new(any)) != io.EOF {
			return nil, fmt.Errorf("trailing card data")
		}
		return &c, nil
	}
	return nil, fmt.Errorf("malformed card links")
}
