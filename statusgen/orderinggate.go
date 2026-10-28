package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
)

// orderinggate.go — the issue #1250 lint: prose-only ordering gates.
//
// THE BLOCKER CLASS. statusgen computes Next-up purely from the typed
// `depends:` graph (nextup.go). A real ordering/behavioural prerequisite that
// lives ONLY in README prose — "no CronJob brief (05–07) starts before this
// lands" — carries no graph edge, so it is invisible to dispatch: a standing
// worker pool reading Next-up picks the brief up straight past its unmet gate.
// This is the most dangerous blocker class precisely because it is silent —
// invisible to statusgen and to every Next-up consumer (design spec §1, issue
// #1250).
//
// This check is the standalone first step of the fleet-dependency-graph spec
// (docs/dependency-graph-design.md §6): a prose-pattern scan over stream
// READMEs and brief bodies that flags gate-shaped prose which no typed edge
// encodes. It is honestly a HEURISTIC, not a parser — false positives and
// false negatives are both certain (§6.2). The convention (every ordering gate
// MUST be a graph edge, enforced at review) is the fence; this lint is only the
// tripwire that turns a silent premature-dispatch into a visible worklist line.
//
// SEVERITY: NOTICE only (design §6.3 Phase A, and Ian's resolved hybrid
// rollout §11.1). The existing tree carries prose gates today; a hard PROBLEM
// would red-gate main on landing, so — exactly as `gate-why` and `why:` staged
// — this ships as a NOTICE worklist first. It flips to PROBLEM in Phase B, once
// the `gates:`/`feathers:` fields exist for authors to reach for. A NOTICE
// never changes the exit code (main.go), so wiring this in cannot turn a green
// board red.

var (
	// orderingGateLexicon is the NAMED gate lexicon (design §6.1 step 1). It is
	// grown from REAL instances — the exact phrasings in desk-console/README and
	// the 2026-08-17 cross-stream blocker survey — never from speculation. A
	// novel phrasing escaping it is an accepted false negative (§6.2); the
	// convention, not the lexicon, is what eliminates the class.
	orderingGateLexicon = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bstarts?\s+before\b`), // "no CronJob brief (05–07) starts before this lands"
		regexp.MustCompile(`(?i)\b(?:must|should|has to|needs? to)\s+land\s+before\b`),
		regexp.MustCompile(`(?i)\bland(?:s|ed)?\s+before\b`), // "must land before X"
		regexp.MustCompile(`(?i)\bbefore\s+(?:this|it|that|which)\s+lands?\b`),
		regexp.MustCompile(`(?i)\bblocked\s+on\b`),
		regexp.MustCompile(`(?i)\bgated\s+on\b`),
		regexp.MustCompile(`(?i)\bwaits?\s+on\b`),
		regexp.MustCompile(`(?i)\bdepends?\s+on\b`),            // prose form of the frontmatter edge
		regexp.MustCompile(`(?i)\bnever\b[^.]{0,40}\buntil\b`), // "never … until Y lands"
		// "gates" as a VERB naming a ref ("gates desk-console/06 Verify 4"), NOT
		// the noun ("presence gates", "safety gates", "human gates") — the noun is
		// the single biggest false-positive source, so a following ref is required.
		regexp.MustCompile(`(?i)\bgates?\s+[^.\n]{0,30}?(?:[a-z][a-z0-9-]*/\d+[a-z]?|#\d+|brief[-\s]?\d+)`),
		regexp.MustCompile(`(?i)\[BLOCKED[:\s\]]`), // "[BLOCKED: model-risk]" stale-annotation tags
	}

	// orderingGateNegations are the certain false-positives (design §6.2): a line
	// that DENIES a block ("this edge never blocks brief-08", "the loop skills
	// must file-and-exit, never block") reads gate-shaped but asserts the
	// opposite. A negated line is skipped outright rather than flagged.
	orderingGateNegations = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bnever\s+blocks?\b`),
		regexp.MustCompile(`(?i)\b(?:does\s+not|doesn't|do\s+not|don't|won't|will\s+not|no\s+longer|cannot|can't|never)\s+block`),
		regexp.MustCompile(`(?i)\bnot\s+blocked\b`),
	}

	// A derived literal-prefix filter rejects ordinary prose before the lexicon is
	// tried. This is only a rejection filter: matching still uses the original
	// lexicon order, not the earliest alternative in the union. Derive the filter
	// from the lexicon itself so a new admitted phrase cannot be omitted.
	orderingGateCandidate = newOrderingCandidateFilter(orderingGateLexicon)
	orderingGateNegated   = orderingPatternUnion(orderingGateNegations)

	// Ref extractors (design §6.1 step 2). A typed `<stream>/<NN>` id, a numeric
	// range/chain (`05–07`, `05→07`), and a `brief-NN` mention. Bare standalone
	// numbers are intentionally NOT extracted — they manufacture spurious pairs.
	orderingBriefIDRe  = regexp.MustCompile(`\b([a-z][a-z0-9]*(?:-[a-z0-9]+)*)/(\d+[a-z]?)\b`)
	orderingBriefNumRe = regexp.MustCompile(`(?i)\bbrief[-\s]?(\d+[a-z]?)\b`)
	orderingNumRangeRe = regexp.MustCompile(`\b(\d{1,2})\s*(?:→|–|—|-|to)\s*(\d{1,2})\b`)

	// orderingGraphWaiverRe matches the explicit per-line waiver (design §6.2
	// mitigation b): `<!-- graph: not-a-gate -->` or
	// `<!-- graph: encoded desk-console/05→desk-hardening/13 -->`. A waiver
	// carrying a reason-class or an edge silences the line; a BARE `<!-- graph: -->`
	// is itself a NOTICE, so the waiver never becomes the new prose escape hatch.
	orderingGraphWaiverRe = regexp.MustCompile(`<!--\s*graph:\s*(.*?)\s*-->`)

	// Generated-view fences (design §7.3): once a stream's feathering/blocked
	// view is re-emitted between `<!-- graph:…:begin -->` / `:end` markers, the
	// prose inside is derived FROM the edges by construction and must not be
	// re-flagged as unencoded.
	orderingGraphViewBeginRe = regexp.MustCompile(`<!--\s*graph:.*:begin`)
	orderingGraphViewEndRe   = regexp.MustCompile(`<!--\s*graph:.*:end`)
)

// orderingPatternUnion shares one rejection scan across unchanged regex rules.
// The standard regexp engine owns matching; this adds no second prose grammar.
func orderingPatternUnion(patterns []*regexp.Regexp) *regexp.Regexp {
	parts := make([]string, len(patterns))
	for i, pattern := range patterns {
		parts[i] = "(?:" + pattern.String() + ")"
	}
	return regexp.MustCompile(strings.Join(parts, "|"))
}

// orderingCandidateFilter is a conservative rejection filter, never a verdict.
// On ASCII prose it searches literal prefixes derived by Go's regexp parser.
// Non-ASCII input (including Unicode simple-fold equivalents), or a regex whose
// first consumed expression is not a mandatory literal, uses the full union.
// No independent list of gate words or Unicode folding rules is maintained.
type orderingCandidateFilter struct {
	full     *regexp.Regexp
	prefixes []orderingLiteralPrefix
	ascii    bool
}
type orderingLiteralPrefix struct {
	text string
	fold bool
}

func newOrderingCandidateFilter(patterns []*regexp.Regexp) orderingCandidateFilter {
	f := orderingCandidateFilter{full: orderingPatternUnion(patterns), ascii: true}
	for _, pattern := range patterns {
		tree, err := syntax.Parse(pattern.String(), syntax.Perl)
		if err != nil {
			f.ascii = false
			break
		}
		prefixes, ok := orderingMandatoryPrefixes(tree)
		if !ok {
			f.ascii = false
			break
		}
		f.prefixes = append(f.prefixes, prefixes...)
	}
	return f
}

// orderingMandatoryPrefixes returns a covering set for the first literal each
// successful alternative must consume. It declines any nullable or unfamiliar
// expression rather than guessing: the caller then uses the original regexp.
func orderingMandatoryPrefixes(tree *syntax.Regexp) ([]orderingLiteralPrefix, bool) {
	switch tree.Op {
	case syntax.OpLiteral:
		text := string(tree.Rune)
		if text == "" || !orderingASCII(text) {
			return nil, false
		}
		fold := tree.Flags&syntax.FoldCase != 0
		if fold {
			text = strings.ToLower(text)
		}
		return []orderingLiteralPrefix{{text: text, fold: fold}}, true
	case syntax.OpCapture:
		return orderingMandatoryPrefixes(tree.Sub[0])
	case syntax.OpAlternate:
		var prefixes []orderingLiteralPrefix
		for _, sub := range tree.Sub {
			p, ok := orderingMandatoryPrefixes(sub)
			if !ok {
				return nil, false
			}
			prefixes = append(prefixes, p...)
		}
		return prefixes, len(prefixes) > 0
	case syntax.OpConcat:
		for _, sub := range tree.Sub {
			switch sub.Op {
			case syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine,
				syntax.OpBeginText, syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
				continue
			}
			return orderingMandatoryPrefixes(sub)
		}
	}
	return nil, false
}

func orderingASCII(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] >= 128 {
			return false
		}
	}
	return true
}

func (f orderingCandidateFilter) MatchString(line string) bool {
	if !f.ascii || !orderingASCII(line) {
		return f.full.MatchString(line)
	}
	lower := ""
	for _, prefix := range f.prefixes {
		text := line
		if prefix.fold {
			if lower == "" {
				lower = strings.ToLower(line)
			}
			text = lower
		}
		if strings.Contains(text, prefix.text) {
			return true
		}
	}
	return false
}

// orderingGateMatch preserves the original lexicon's priority and exact span.
// The derived filter only skips prose that cannot match an admitted rule.
func orderingGateMatch(line string) (loc, size int) {
	if !orderingGateCandidate.ascii || !orderingASCII(line) {
		// Preserve the exact old path for inputs the literal filter declines.
		// In particular, Unicode gate-heavy prose pays no extra union scan.
		for _, re := range orderingGateNegations {
			if re.MatchString(line) {
				return -1, 0
			}
		}
	} else if !orderingGateCandidate.MatchString(line) || orderingGateNegated.MatchString(line) {
		return -1, 0
	}
	for _, re := range orderingGateLexicon {
		if idx := re.FindStringIndex(line); idx != nil {
			return idx[0], idx[1] - idx[0]
		}
	}
	return -1, 0
}

// orderingGateEdges builds the undirected adjacency of every TYPED edge in the
// current root — `depends:` and `unblocks:` from brief-v1 frontmatter, both
// directions unioned. This is the ONLY graph the pre-`feathers:`/`gates:` lint
// cross-checks against (design §6.1 step 3). A gate whose prerequisite is
// encoded here is a caption, not an unencoded edge.
func orderingGateEdges(streams []*Stream) map[string]map[string]bool {
	adj := map[string]map[string]bool{}
	link := func(a, b string) {
		if a == "" || b == "" || a == b {
			return
		}
		if adj[a] == nil {
			adj[a] = map[string]bool{}
		}
		if adj[b] == nil {
			adj[b] = map[string]bool{}
		}
		adj[a][b] = true
		adj[b][a] = true
	}
	for _, s := range streams {
		for _, p := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(p)
			if err != nil || !ok {
				continue // malformed/legacy: checkBriefFiles owns that report
			}
			// Key by <stream>/<NN>, the form refs (and extractOrderingRefs) use: a
			// brief-v2 id is <cell>:<repo>:<stream>:<NN> and would never meet its
			// prose refs, so every v2 gate read as unencoded (#1960 defect class).
			// The filename-derived id is that <stream>/<NN> for both schemas and
			// drops no repo alias (TestAliasDropGuard).
			self, _, okName := expectedBriefID(p)
			if !okName {
				continue // a misnamed file: checkBriefFiles owns that report
			}
			for _, d := range bf.Depends {
				link(self, d)
			}
			for _, u := range bf.Unblocks {
				link(self, u)
			}
		}
	}
	return adj
}

// extractOrderingRefs pulls the candidate `<stream>/<NN>` refs out of a slice of
// prose. Bare `NN` and `brief-NN` mentions resolve against the CURRENT stream
// (an in-README "05" means this stream's brief 05); a typed `<stream>/<NN>` is
// taken verbatim. Order-preserving, de-duplicated.
func extractOrderingRefs(text, stream string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(r string) {
		if r != "" && !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	for _, m := range orderingBriefIDRe.FindAllStringSubmatch(text, -1) {
		add(strings.ToLower(m[1]) + "/" + m[2])
	}
	for _, m := range orderingBriefNumRe.FindAllStringSubmatch(text, -1) {
		add(stream + "/" + m[1])
	}
	for _, m := range orderingNumRangeRe.FindAllStringSubmatch(text, -1) {
		lo, err1 := strconv.Atoi(m[1])
		hi, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil || lo > hi || hi-lo > 30 {
			continue
		}
		width := len(m[1])
		for i := lo; i <= hi; i++ {
			add(stream + "/" + fmt.Sprintf("%0*d", width, i))
		}
	}
	return out
}

// orderingGateNotices is the #1250 lint. It does its own file I/O (like
// checkBriefFiles/linkProblems) and is wired into run() alongside them, so
// check() stays I/O-free. Every finding is a NOTICE (design §6.3 Phase A).
func orderingGateNotices(streams []*Stream) []string {
	adj := orderingGateEdges(streams)
	var notices []string
	notice := func(format string, a ...any) { notices = append(notices, fmt.Sprintf(format, a...)) }

	// edgeBetween: is any pair (one ref from left, one from right) connected by a
	// typed edge? This is the side-split cross-check — refs on OPPOSITE sides of
	// the gate keyword are the pair the sentence actually relates, so a gate
	// whose target is prose (an issue, a spec §, "this") has refs on only one
	// side and can never be clean.
	edgeBetween := func(left, right []string) bool {
		for _, a := range left {
			for _, b := range right {
				if adj[a][b] {
					return true
				}
			}
		}
		return false
	}

	for _, s := range streams {
		var files []string
		files = append(files, filepath.Join(s.Dir, "README.md"))
		files = append(files, briefFilePaths(s)...)

		for _, path := range files {
			raw, err := readFileMemo(path)
			if err != nil {
				continue // README absence is another check's problem; briefs were globbed present
			}
			rel := relPath(filepath.Dir(path)) + "/" + filepath.Base(path)

			inGeneratedView := false
			for i, line := range strings.Split(string(raw), "\n") {
				lineNo := i + 1

				// Generated-view fences: everything between them is derived from
				// the edges, so it is clean by construction.
				if orderingGraphViewBeginRe.MatchString(line) {
					inGeneratedView = true
					continue
				}
				if orderingGraphViewEndRe.MatchString(line) {
					inGeneratedView = false
					continue
				}
				if inGeneratedView {
					continue
				}

				// Explicit per-line waiver. A waiver with a reason-class or an
				// edge silences the line; a bare `<!-- graph: -->` is itself a
				// NOTICE (a waiver must not become the new prose escape hatch).
				if m := orderingGraphWaiverRe.FindStringSubmatch(line); m != nil {
					if strings.TrimSpace(m[1]) == "" {
						notice("ordering-gate: %s:%d bare `<!-- graph: -->` waiver states neither a reason-class nor an edge — write `<!-- graph: not-a-gate -->` or `<!-- graph: encoded <a>→<b> -->` (#1250)", rel, lineNo)
					}
					continue
				}

				// Feathering tables are the cross-repo (tier-2) encoding, migrated
				// to `feathers:` separately — not the prose (tier-3) gate this lint
				// targets. Skipping markdown table rows keeps the two tiers apart
				// and cuts the false-positive flood the `gates`/`blocked on` verbs
				// would otherwise raise on every table cell.
				if strings.HasPrefix(strings.TrimSpace(line), "|") {
					continue
				}

				loc, hitLen := orderingGateMatch(line)
				if loc < 0 {
					continue // no gate lexicon hit
				}

				// Side-split around the gate keyword: the refs the sentence
				// relates sit on opposite sides of it.
				left := extractOrderingRefs(line[:loc], s.Name)
				right := extractOrderingRefs(line[loc+hitLen:], s.Name)

				if len(left) > 0 && len(right) > 0 {
					if edgeBetween(left, right) {
						continue // encoded: the prose is now a caption on a real edge
					}
					notice("ordering-gate: %s:%d prose %q relates %s and %s but no typed edge (depends:/unblocks:) encodes it — encode the prerequisite on the owning brief, or waive with `<!-- graph: not-a-gate -->` if it is not a real gate (#1250)",
						rel, lineNo, snippet(line), strings.Join(left, ","), strings.Join(right, ","))
					continue
				}

				// One-sided: the gate NAMES briefs on one side but its target or
				// subject is prose (an issue, a spec §, "this lands"). This is the
				// exact desk-console §8 shape and the most dangerous case — the
				// unmet prerequisite has no ref at all. Give it an owning brief and
				// a typed edge, or waive it.
				//
				// A gate-shaped line with NO extractable brief ref anywhere is NOT
				// flagged: in Phase A that branch is almost entirely gate VOCABULARY
				// in ordinary prose ("presence gates", "credibility depends on it",
				// "## Verify … safety gates") rather than a real cross-brief
				// prerequisite, and flagging it buries the ref-bearing signal under
				// hundreds of noise lines. The convention + review gate own the
				// no-ref class (design §6.2: the lint is the tripwire, not the fence).
				refs := append(append([]string{}, left...), right...)
				if len(refs) > 0 {
					notice("ordering-gate: %s:%d prose %q names %s on one side but its gate target/subject is prose (an issue, a spec section, \"this\") — give the prerequisite an owning brief with a depends:/unblocks: edge, or waive with `<!-- graph: ... -->` (#1250)",
						rel, lineNo, snippet(line), strings.Join(refs, ","))
				}
			}
		}
	}
	return notices
}

// snippet trims a line to a readable quote for the notice message.
func snippet(line string) string {
	s := strings.TrimSpace(line)
	const max = 70
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
