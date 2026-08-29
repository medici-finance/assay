package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// BriefFile is the validated frontmatter of a `schema: brief-v1` brief file.
// Validation is OPT-IN: only files whose frontmatter carries the `schema: brief-v1`
// marker are parsed here. Legacy briefs (no frontmatter, or a different schema)
// are exempt and produce no output — see parseBriefFile.
type BriefFile struct {
	Path     string
	Brief    string // "<stream>/<NN>", e.g. "example-app/01"
	Title    string
	Wave     int
	Depends  []string // typed IDs "<stream>/<NN>"
	Unblocks []string // typed IDs "<stream>/<NN>"
	Effort   string   // S | M | L
	Gate     string   // model | human
	GateWhy  string   // optional prose: WHY this brief is risk-gated (gate-why-rationale)
	Why      string   // optional prose: WHY this work exists at all — human-justifiable motivation
	Value    string   // optional worth signal: low | med | high; "" = absent (== med)
	// Domain is the optional brief-v1 `domain:` field — the Cynefin domain of
	// the work: clear | complicated | complex | chaotic. "" when absent, which
	// defaults to "complicated" (the safe Ordered default) for the ToC↔Cynefin
	// switch (agentic-metrics/10); a wrong TYPE is a parse error, a present-but-
	// unrecognized value is a lint PROBLEM. The --cynefin view surfaces an absent
	// value as Disorder (untagged — the author should classify), so absence is
	// operationally treated as complicated but reported as un-classified.
	Domain        string
	Risk          map[string]string
	Issues        []int
	DecisionIssue int // optional; the GitHub issue # for the open needs-decision issue
	Schema        string
	Authored      string
	Sources       []string
	// ExecTier is the optional brief-v1 `exec-tier:` field — "any" or "strong";
	// "" when absent (treated as "any"). Signals a minimum execution-model tier
	// to the dispatcher; a marker in Next-up, never a score input.
	ExecTier string
	// ExecTierWhy is the optional one-line rationale for exec-tier: strong —
	// which derivation question(s) it answered yes and why.
	// NOTICEd when exec-tier: strong but exec-tier-why is absent or empty.
	ExecTierWhy string
	// BlockedBy is the optional brief-v1 `blocked-by:` field — "env" when the
	// brief is blocked on infrastructure/environment; "" when absent (marks the env-blocked segment in the segmented Awaiting board).
	BlockedBy string
	// HomedIn is the optional brief-v1 `homed-in:` field — "<owner>/<repo>" when
	// the brief's deliverable lives in ANOTHER repo than the board that renders
	// it (a de-housing); "" when absent, the default (a normal in-repo brief). A
	// present value excludes the brief from THIS board's Next-up eligibility and
	// carries the target repo on the tracking row, so a cross-repo dispatcher
	// reads the right target instead of burning a slot to discover the mis-route.
	// A wrong TYPE is a parse error; a present-but-malformed shape is a hard
	// PROBLEM in checkBriefFiles. NEVER a Next-up score input (F-09 scope note).
	HomedIn string
	// Measures is the optional brief-v1 `measures:` field — the name of the
	// process queue this brief instruments. nil when absent (the neutral
	// default: not an instrumentation brief), non-nil when present, including
	// the empty string. Feeds the drain-before-instrument eligibility gate.
	Measures *string
	// Consumers is the optional brief-v1 `consumers:` list (brief-rule 9): the
	// readers of a shared value this brief changes,
	// each routed `<site>: fixed-here | follow-up <stream>/<NN> | out-of-scope
	// (<why>)`. Every entry is an implementer-written CLAIM, corroborated by
	// consumers.go — never treated as true because it is present.
	Consumers []string
	// ConsumersProse holds a `consumers:` written as a paragraph instead of a
	// routed list. Kept rather than rejected so the lint can say what is wrong
	// with it; a scalar here is NOT a parse error (several briefs on main use
	// the prose form and a hard error would red-gate the whole board).
	ConsumersProse string
	// ParallelStreams is the OPTIONAL brief-v1 `parallel-streams:` list
	// (methodology/43): the declared shards of an intra-brief split, each a
	// name plus the file globs that shard owns. nil when absent, which is the
	// default and means one worker per brief — the behaviour every existing
	// brief already has. Presence is the brief's explicit opt-in to a split; it
	// is a REQUEST, never an approval, and `statusgen shardcheck` is what
	// decides whether the split may actually be dispatched (shardcheck.go).
	ParallelStreams []ParallelStream
	Evidence        string // body of the `## Evidence` section (between it and the next `## `)
	Verify          string // body of the `## Verify` section (prefix-matched; decorated headings allowed)
	Body            string // full markdown body after the frontmatter (decision-reflection check)
	// DeclaredPaths are the repo-relative paths a brief names on the `files:` line
	// of its `## Context` section — the paths it declares it will touch. Parsed by
	// extractContextDeclaredPaths; the mistake-proofing/01 cross-read compares them
	// against the risk-path classifier (riskfilescrossread.go), and briefs 03/05
	// build on the same parse. nil when the line is absent/empty/unparseable, which
	// DeclaredPathsFound distinguishes from a genuinely empty declaration.
	DeclaredPaths []string
	// DeclaredPathsFound is true only when a `files:` line was present AND yielded
	// at least one path. false is a COULD-NOT-CHECK for any consumer — never round
	// it up to "no risky paths" (docs/three-state-instrument-rule.md).
	DeclaredPathsFound bool
}

var (
	// brief-NN or brief-NNa, optionally followed by a -slug, then .md.
	briefNameRe = regexp.MustCompile(`^brief-([0-9]+[a-z]?)(?:-.*)?\.md$`)

	// HTML comment block (the Evidence contract comment); stripped before
	// checking whether the section has any real content. An unterminated
	// comment (`<!--` with no closing `-->`) is stripped to end-of-input — it
	// consumes the rest of the section, so it cannot masquerade as content.
	htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?(?:-->|$)`)

	// briefSchemaCurrent is the one brief schema version this binary validates;
	// briefSchemaFamilyPrefix is the shared prefix of the brief-schema family
	// (`brief-v1`, and any future `brief-v2`, …). A value with this prefix that
	// is not briefSchemaCurrent fails CLOSED in parseBriefFile (#271), whereas a
	// value outside the family (contract-v1, placeholder-v1, …) stays exempt.
	briefSchemaCurrent      = "brief-v1"
	briefSchemaFamilyPrefix = "brief-v"

	// Required frontmatter keys. `schema` is intentionally absent: its presence
	// (== brief-v1) is the opt-in gate in parseBriefFile, so it is always present
	// by the time these are checked.
	requiredBriefKeys = []string{
		"brief", "title", "wave", "depends", "unblocks", "effort",
		"gate", "risk", "issues", "authored", "sources",
	}
	canonicalRiskKeys = []string{"regulatory", "customer", "irreversible", "sensitive-data"}
	canonicalRiskSet  = map[string]bool{"regulatory": true, "customer": true, "irreversible": true, "sensitive-data": true}
	validEffort       = map[string]bool{"S": true, "M": true, "L": true}
	validGate         = map[string]bool{"model": true, "human": true}
	// validExecTier is the allowed set for the optional brief-v1 `exec-tier:` field.
	// Absence ("") is always allowed (defaults to "any").
	validExecTier = map[string]bool{"any": true, "strong": true}
	// validValue is the allowed set for the optional brief-v1 `value:` field.
	// Absence ("") is always allowed (defaults to med at scoring time); only a
	// PRESENT-but-unrecognized value is a PROBLEM.
	validValue = map[string]bool{"low": true, "med": true, "high": true}
	// validDomain is the allowed set for the optional brief-v1 `domain:` field
	// (Cynefin classification, agentic-metrics/10). Absence ("") is always
	// allowed and defaults to defaultDomain at read time; only a PRESENT-but-
	// unrecognized value is a PROBLEM (echoed in the message).
	validDomain = map[string]bool{"clear": true, "complicated": true, "complex": true, "chaotic": true}
	// validBlockedBy is the allowed set for the optional brief-v1 `blocked-by:`
	// field. Absence ("") is always allowed (defaults to
	// "" = not blocked); only a PRESENT-but-unrecognized value is a PROBLEM.
	validBlockedBy = map[string]bool{"env": true}
	// validMeasuresQueue is the set of process queues statusgen actually knows
	// how to measure, for the optional brief-v1 `measures:` field
	// (drain-before-instrument). Absence is always allowed (nil = not an
	// instrumentation brief); a PRESENT-but-unrecognized name is a hard PROBLEM
	// — typo protection, because the runtime gate treats an unreadable queue as
	// could-not-check and holds the brief back, and a silent no-op there would
	// be the worst of both worlds.
	//
	// ONE queue is wired today. Do not add names here speculatively: a name in
	// this map is a promise that a depth and a threshold exist for it.
	validMeasuresQueue = map[string]bool{"verification-debt": true}
)

// validHomedInShape reports whether v is a well-formed `<owner>/<repo>` value
// for the optional brief-v1 `homed-in:` field: exactly one "/", both sides
// non-empty, and no whitespace anywhere. It deliberately does NOT check the
// value against a repo allowlist — statusgen has no such list and must not
// couple to one; the shape is all that can be validated locally.
func validHomedInShape(v string) bool {
	if strings.ContainsAny(v, " \t\n\r") {
		return false
	}
	owner, repo, found := strings.Cut(v, "/")
	if !found || owner == "" || repo == "" {
		return false
	}
	// A second "/" leaves it in `repo`, e.g. "a/b/c" → owner "a", repo "b/c".
	return !strings.Contains(repo, "/")
}

// measuresQueueNames lists the wired queue names, sorted, for lint messages.
func measuresQueueNames() []string {
	names := make([]string, 0, len(validMeasuresQueue))
	for k := range validMeasuresQueue {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// minWhySubstanceLength is the floor on a present why:'s trimmed length —
// below this it reads as a placeholder ("." or "TODO"), not a
// rationale. Chosen well under a genuine one-line justification (the shortest
// real example on file runs well over 100 characters) so it only catches
// content-free stand-ins, never a terse-but-real answer.
const minWhySubstanceLength = 25

// normalizeForDupCheck lowercases s and collapses every run of non-alphanumeric
// characters to a single space, so punctuation/casing differences don't hide a
// title restated as a why (or vice versa).
func normalizeForDupCheck(s string) string {
	var b strings.Builder
	prevSpace := true // trims leading separators for free
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			prevSpace = false
		} else if !prevSpace {
			b.WriteRune(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// whySubstanceIssue reports why a PRESENT why: fails the substance floor
// or "" if it clears it. Two failure modes, both content-free
// inputs that a presence-only check cannot distinguish from real rationale:
//   - too short to be a rationale (e.g. "." or "TODO").
//   - a substring of, or near-identical to, the brief's own title — a restated
//     title carries the "what" a reader already has, not the "why" the field
//     exists for.
//
// This stays a floor, not a rewrite of the field's semantics: it rejects only
// the cheapest content-free inputs, never a genuine short rationale that
// merely shares wording with the title.
func whySubstanceIssue(why, title string) string {
	trimmed := strings.TrimSpace(why)
	if len(trimmed) < minWhySubstanceLength {
		return fmt.Sprintf("only %d characters (want at least %d) — reads like a placeholder, not a rationale", len(trimmed), minWhySubstanceLength)
	}
	normWhy := normalizeForDupCheck(why)
	normTitle := normalizeForDupCheck(title)
	if normWhy != "" && normTitle != "" && (normWhy == normTitle || strings.Contains(normTitle, normWhy) || strings.Contains(normWhy, normTitle)) {
		return "restates the title instead of giving independent rationale"
	}
	return ""
}

// decisionIssueRefInBody reports whether the brief body text references a
// specific GitHub issue number (e.g., "#42") with a digit-boundary check: the
// reference must not be followed by another digit, so a brief's "#8" won't
// falsely match an unrelated "#88" (substring
// prefix-collision). Matches "#<num>" followed by a non-digit or end-of-text.
func decisionIssueRefInBody(body string, issueNum int) bool {
	// Compile per call: issue numbers are small and this runs at lint time, not
	// in a hot loop.
	re := regexp.MustCompile(fmt.Sprintf("#%d(?:[^0-9]|$)", issueNum))
	return re.MatchString(body)
}

// briefFilePaths returns the sorted brief-*.md paths in a stream directory.
//
// It reads the directory with os.ReadDir rather than filepath.Glob. Glob
// interprets the WHOLE joined path as a pattern, so a stream directory whose
// name contains an unbalanced glob metacharacter (e.g. `foo[bar`) makes
// filepath.Glob(filepath.Join(s.Dir, "brief-*.md")) return ErrBadPattern with
// zero matches — and the discarded error meant that stream's briefs silently
// vanished from every consumer (a could-not-check rendered as "no briefs here",
// docs/three-state-instrument-rule.md, sub-rule 1). os.ReadDir treats the name
// literally, so that blind spot cannot occur. s.Dir was already proven readable
// at load time (loadStreams stat'd and parsed its README.md), so a read error
// here is not expected; return nothing rather than a partial list.
func briefFilePaths(s *Stream) []string {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil
	}
	var matches []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if name := e.Name(); strings.HasPrefix(name, "brief-") && strings.HasSuffix(name, ".md") {
			matches = append(matches, filepath.Join(s.Dir, name))
		}
	}
	sort.Strings(matches)
	return matches
}

// expectedBriefID derives the canonical brief ID (<dirname>/<NN>) from a file
// path. ok is false when the basename is not a brief-<NN>[-slug].md file.
func expectedBriefID(path string) (id, num string, ok bool) {
	m := briefNameRe.FindStringSubmatch(filepath.Base(path))
	if m == nil {
		return "", "", false
	}
	num = m[1]
	return filepath.Base(filepath.Dir(path)) + "/" + num, num, true
}

// parseBriefFile reads a brief file and returns its validated frontmatter.
//
// Return contract:
//   - (nil, false, nil)  — exempt: no frontmatter, or a frontmatter block that
//     carries no `schema:` key at all (legacy).
//   - (nil, false, err)  — opted-in but malformed, OR a present `schema:` whose
//     value this binary does not recognize (schema evolution fails CLOSED, #271):
//     unreadable, bad YAML, a missing/ill-typed required field, or an unknown
//     schema. err is already prefixed with the path.
//   - (bf,  true,  nil)  — opted-in and structurally valid; semantic checks in
//     checkBriefFiles still apply.
//
// Callers MUST test err before ok.
func parseBriefFile(path string) (*BriefFile, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	// Normalize CRLF so a Windows-authored brief is not silently exempted.
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")

	// A brief opts in only via YAML frontmatter. No leading `---` → legacy, exempt.
	first, _, _ := strings.Cut(content, "\n")
	if strings.TrimSpace(first) != "---" {
		return nil, false, nil // legacy (no frontmatter) → exempt
	}
	fmRaw, body, err := splitFrontmatter(content)
	if err != nil {
		// A leading `---` means the file intends frontmatter; a split failure
		// (unterminated fence) is a real error, not an exemption.
		return nil, false, fmt.Errorf("%s: %v", path, err)
	}

	var data map[string]any
	if err := yaml.Unmarshal([]byte(fmRaw), &data); err != nil {
		return nil, false, fmt.Errorf("%s: frontmatter: %w", path, err)
	}
	// Opt-in by the PARSED schema value — robust to quoting, trailing comments,
	// and CRLF that a raw-text marker match would miss.
	//
	// Fail CLOSED on BRIEF-schema evolution (#271 / adversarial SY-4, RD-7): a
	// file that declares a brief-schema-family value this binary does not
	// recognize (a future `brief-v2`, `brief-v3`, …) is REFUSED, not silently
	// exempted. Consumers run pinned release binaries bumped explicitly via
	// `.assay-versions`; without this refusal, the day `schema: brief-v2` ships,
	// every not-yet-bumped consumer would lint v2 briefs green-by-exemption —
	// typed deps, gate derivation, attribution and demotion checks all silently
	// off, indistinguishable from a validated pass. The safe path is a hard
	// error telling the operator to upgrade statusgen.
	//
	// Scope note: only the BRIEF schema family (`brief-v*`) fails closed here.
	// Two other kinds of `schema:` value stay EXEMPT, exactly as before, because
	// this is the brief parser and they are out of its jurisdiction:
	//   - no `schema:` key at all  → a legacy (pre-schema) brief.
	//   - a different document kind → e.g. `contract-v1`, `placeholder-v1`,
	//     `publication-manifest-v1`: a `brief-*.md` file that is deliberately a
	//     non-brief document. statusgen has no brief-validation opinion on it.
	schemaVal, hasSchema := data["schema"]
	if !hasSchema {
		return nil, false, nil // frontmatter without a schema marker → legacy, exempt
	}
	s, isStr := schemaVal.(string)
	if !isStr {
		// A non-string schema is not a recognized brief marker; treat it as a
		// non-brief document (exempt) rather than guessing intent.
		return nil, false, nil
	}
	switch {
	case s == briefSchemaCurrent:
		// Recognized brief schema → validate below.
	case strings.HasPrefix(s, briefSchemaFamilyPrefix):
		// A brief-schema-family version this binary does not understand → the
		// #271 flag-day trap. Refuse it.
		return nil, false, fmt.Errorf("%s: unrecognized brief schema %q — this statusgen validates only %q; upgrade statusgen to a build that understands this schema (schema evolution fails closed, #271)", path, s, briefSchemaCurrent)
	default:
		// A different document kind (contract-v1, placeholder-v1, …) → not a
		// brief this parser validates. Exempt, as before.
		return nil, false, nil
	}

	var missing []string
	for _, k := range requiredBriefKeys {
		if _, ok := data[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, false, fmt.Errorf("%s: missing required field(s): %s", path, strings.Join(missing, ", "))
	}

	bf := &BriefFile{Path: path}
	var bad []string
	addBad := func(format string, a ...any) { bad = append(bad, fmt.Sprintf(format, a...)) }

	if v, ok := data["brief"].(string); ok {
		bf.Brief = v
	} else {
		addBad("brief must be a string")
	}
	if v, ok := data["title"].(string); ok {
		bf.Title = v
	} else {
		addBad("title must be a string")
	}
	switch w := data["wave"].(type) {
	case int:
		bf.Wave = w
	case int64:
		bf.Wave = int(w)
	default:
		addBad("wave must be an integer")
	}
	if v, err := stringList(data["depends"]); err == nil {
		bf.Depends = v
	} else {
		addBad("depends: %v", err)
	}
	if v, err := stringList(data["unblocks"]); err == nil {
		bf.Unblocks = v
	} else {
		addBad("unblocks: %v", err)
	}
	if v, ok := data["effort"].(string); ok {
		bf.Effort = v
	} else {
		addBad("effort must be a string")
	}
	if v, ok := data["gate"].(string); ok {
		bf.Gate = v
	} else {
		addBad("gate must be a string")
	}
	if v, err := riskMap(data["risk"]); err == nil {
		bf.Risk = v
	} else {
		addBad("risk: %v", err)
	}
	if v, err := intList(data["issues"]); err == nil {
		bf.Issues = v
	} else {
		addBad("issues: %v", err)
	}
	if v, ok := data["schema"].(string); ok {
		bf.Schema = v
	} else {
		addBad("schema must be a string")
	}
	if v, ok := data["authored"].(string); ok {
		bf.Authored = v // a bare-date authored decodes to time.Time; presence alone is required
	}
	if v, err := stringList(data["sources"]); err == nil {
		bf.Sources = v
	} else {
		addBad("sources: %v", err)
	}
	// gate-why is an OPTIONAL but KNOWN key: it records WHY a risk-gated brief
	// trips the human gate. Parsing it here means a brief carrying it is a
	// recognized field, not schema drift, which is what makes the later
	// per-brief backfill safe. Absence is a hard PROBLEM on risk-flagged
	// briefs (checkBriefFiles enforces it); only a wrong TYPE is a parse error.
	if v, ok := data["gate-why"]; ok {
		if s, ok := v.(string); ok {
			bf.GateWhy = s
		} else {
			addBad("gate-why must be a string")
		}
	}
	// exec-tier is an OPTIONAL but KNOWN key: the brief's
	// minimum execution-model tier — "any" or "strong". Absence defaults to
	// "any"; a wrong TYPE is a parse error, while a present-but-unrecognized
	// string is flagged semantically in checkBriefFiles.
	if v, ok := data["exec-tier"]; ok {
		if s, ok := v.(string); ok {
			bf.ExecTier = s
		} else {
			addBad("exec-tier must be a string")
		}
	}
	// exec-tier-why is an OPTIONAL but KNOWN key: a one-line
	// rationale for exec-tier: strong. NOTICEd when exec-tier: strong but
	// exec-tier-why is absent or empty.
	if v, ok := data["exec-tier-why"]; ok {
		if s, ok := v.(string); ok {
			bf.ExecTierWhy = s
		} else {
			addBad("exec-tier-why must be a string")
		}
	}
	// blocked-by is an OPTIONAL but KNOWN key: "env"
	// when the brief is blocked on infrastructure/environment. Absence defaults
	// to "" (not blocked). A present-but-unrecognized value is flagged
	// semantically in checkBriefFiles.
	//
	// PARSED INDEPENDENTLY of exec-tier-why on purpose. Nesting it inside the
	// exec-tier-why block made the whole feature inert for ordinary briefs:
	// exec-tier-why is only written on `exec-tier: strong` briefs, so a brief
	// carrying `blocked-by: env` alone parsed to "" and fell open into the
	// desk-actionable queue, and the invalid-value PROBLEM below was unreachable
	// on the same condition. Pinned by TestBlockedByParsedWithoutExecTierWhy.
	if v, ok := data["blocked-by"]; ok {
		if s, ok := v.(string); ok {
			bf.BlockedBy = s
		} else {
			addBad("blocked-by must be a string")
		}
	}

	// measures is an OPTIONAL but KNOWN key (drain-before-instrument): the
	// process queue this brief instruments. Stored as a POINTER so absent and
	// present-but-empty stay distinguishable — absent is the neutral default
	// that leaves the brief untouched, while an empty value is a written-but-
	// meaningless queue name and must be caught, not defaulted away. A wrong
	// TYPE is a parse error; an unrecognized name is flagged semantically in
	// checkBriefFiles so the bad name is echoed back.
	if v, ok := data["measures"]; ok {
		if s, ok := v.(string); ok {
			bf.Measures = &s
		} else {
			addBad("measures must be a string")
		}
	}
	// homed-in is an OPTIONAL but KNOWN key (statusgen/12): the "<owner>/<repo>"
	// the brief's deliverable was re-homed to. Absence defaults to "" (a normal
	// in-repo brief) and is never flagged. A wrong TYPE is a parse error; a
	// present-but-malformed shape is flagged semantically in checkBriefFiles so
	// the bad value is echoed back. Parsed independently of every other optional
	// key.
	if v, ok := data["homed-in"]; ok {
		if s, ok := v.(string); ok {
			bf.HomedIn = s
		} else {
			addBad("homed-in must be a string")
		}
	}
	// parallel-streams is an OPTIONAL but KNOWN key (methodology/43): the shards
	// of an intra-brief split. Absence is the default and is never flagged —
	// every brief on file today omits it and dispatches to one worker, which is
	// the point of making the field optional rather than adding it to
	// requiredBriefKeys.
	//
	// Only the SHAPE is validated here (names, globs, types). Whether the split
	// is safe to dispatch depends on the file tree, not the frontmatter, and is
	// decided by `statusgen shardcheck` — a declaration that parses is a
	// request, not a permission.
	if v, ok := data["parallel-streams"]; ok {
		ps, err := parallelStreamList(v)
		if err != nil {
			addBad("parallel-streams: %v", err)
		} else {
			bf.ParallelStreams = ps
		}
	}
	// value is an OPTIONAL but KNOWN key: the brief's
	// explicit worth, a Next-up score input. Absence is fine (defaults to med at
	// scoring time); a wrong TYPE here is a hard parse error, while a present-but-
	// unrecognized string is flagged semantically in checkBriefFiles so the value
	// is echoed in the message.
	if v, ok := data["value"]; ok {
		if s, ok := v.(string); ok {
			bf.Value = s
		} else {
			addBad("value must be a string")
		}
	}
	// domain is an OPTIONAL but KNOWN key (agentic-metrics/10): the brief's
	// Cynefin domain — clear|complicated|complex|chaotic. Absence defaults to
	// "complicated" (the safe Ordered default) at read time; a wrong TYPE is a
	// hard parse error, while a present-but-unrecognized string is flagged
	// semantically in checkBriefFiles so the bad token is echoed in the message.
	if v, ok := data["domain"]; ok {
		if s, ok := v.(string); ok {
			bf.Domain = s
		} else {
			addBad("domain must be a string")
		}
	}
	// why is an OPTIONAL but KNOWN key: the brief's VALUE
	// rationale — one to three lines a non-engineer could read and justify
	// the work from. Absence is a NOTICE in checkBriefFiles (non-fatal this
	// phase, same pattern as gate-why); only a wrong TYPE is a parse error.
	// PHASE 3 flips the NOTICE to a hard error once backfill lands — see
	// the notice comment in checkBriefFiles for the plan.
	if v, ok := data["why"]; ok {
		if s, ok := v.(string); ok {
			bf.Why = s
		} else {
			addBad("why must be a string")
		}
	}
	// consumers is an OPTIONAL but KNOWN key (brief-rule 9): the routed reader
	// list of a shared-value brief.
	// A scalar (prose paragraph) is accepted into ConsumersProse and flagged
	// semantically; only a list containing a non-string is a parse error.
	if v, ok := data["consumers"]; ok {
		if s, isStr := v.(string); isStr {
			bf.ConsumersProse = s
		} else if list, lerr := stringList(v); lerr == nil {
			bf.Consumers = list
		} else {
			addBad("consumers: %v", lerr)
		}
	}
	// decision-issue is an OPTIONAL but KNOWN key: the GitHub
	// issue # for the open needs-decision issue tracking this brief. Absence is
	// fine (most briefs do not need a human decision); a wrong TYPE is an error.
	if v, ok := data["decision-issue"]; ok {
		switch n := v.(type) {
		case int:
			bf.DecisionIssue = n
		case int64:
			bf.DecisionIssue = int(n)
		default:
			addBad("decision-issue must be an integer")
		}
	}
	if len(bad) > 0 {
		return nil, false, fmt.Errorf("%s: %s", path, strings.Join(bad, "; "))
	}
	bf.Evidence = extractEvidence(body)
	// The `## Verify` heading is DECORATED in practice ("## Verify (executable
	// — …)"), so match it by prefix — the same way verifyissues.go lifts it —
	// rather than the exact-match extractEvidence uses.
	bf.Verify = extractSectionByPrefix(body, "Verify")
	bf.Body = body
	bf.DeclaredPaths, bf.DeclaredPathsFound = extractContextDeclaredPaths(body)
	return bf, true, nil
}

var (
	// contextFilesLabelRe matches the `files:` declared-paths label line inside a
	// brief's `## Context` section. Leading whitespace is tolerated; the capture is
	// the optional inline value that follows the colon on the same line.
	contextFilesLabelRe = regexp.MustCompile(`^\s*files:\s*(.*)$`)
	// backtickSpanRe captures the content of a backtick-delimited span.
	backtickSpanRe = regexp.MustCompile("`([^`]+)`")
	// mdLinkRe matches a markdown [text](url) link so the text can be recovered.
	mdLinkTextRe = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
)

// extractContextDeclaredPaths reads the `files:` declared-paths line from a
// brief's `## Context` section and returns the paths it names. This is the first
// half of mistake-proofing/01: the declared-paths line is parsed by nothing else
// in the lint, and making it readable is what the cross-read (and briefs 03/05)
// build on.
//
// Two authored forms are accepted, matching the corpus:
//   - INLINE — `files: a/b, c/d` on the label line itself, comma/space separated.
//   - BULLETED — a bare `files:` label followed by markdown bullets on the next
//     lines, each naming one or more backticked paths, with optional trailing
//     prose after an em dash and indented continuation lines.
//
// Markdown decoration is stripped (backticks, `[text](url)` link syntax, and
// trailing prose after an em dash — which sits outside the backticks and so is
// dropped for free) so a path is compared as a path. A candidate is kept only if
// it is path-shaped — it contains a '/' or a '.' — so a backticked identifier in a
// bullet (a function or symbol name) is not mistaken for a declared path.
//
// found is false when the line is absent, empty, or yields no path. That is a
// COULD-NOT-CHECK for the cross-read, never a pass: the caller must not treat "no
// declared paths" as "no risky paths" (docs/three-state-instrument-rule.md).
func extractContextDeclaredPaths(body string) (paths []string, found bool) {
	ctx := extractSectionByPrefix(body, "Context")
	if strings.TrimSpace(ctx) == "" {
		return nil, false
	}
	lines := strings.Split(ctx, "\n")
	labelIdx := -1
	var inline string
	for i, l := range lines {
		if m := contextFilesLabelRe.FindStringSubmatch(l); m != nil {
			labelIdx = i
			inline = strings.TrimSpace(m[1])
			break
		}
	}
	if labelIdx < 0 {
		return nil, false
	}

	seen := map[string]bool{}
	add := func(tok string) {
		tok = cleanDeclaredPath(tok)
		// Keep only path-shaped tokens: a bare word (no '/' and no '.') is a
		// symbol or prose fragment, not a declared path, and matches no trigger.
		if tok == "" || seen[tok] || !strings.ContainsAny(tok, "/.") {
			return
		}
		seen[tok] = true
		paths = append(paths, tok)
	}

	if inline != "" {
		// INLINE form: recover link text, then split on comma/space/backtick.
		inline = mdLinkTextRe.ReplaceAllString(inline, "$1")
		for _, tok := range strings.FieldsFunc(inline, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t' || r == '`'
		}) {
			add(tok)
		}
		return paths, len(paths) > 0
	}

	// BULLETED form: collect the block of bullet + indented-continuation lines that
	// follow the label (stopping at a blank line or a new flush-left label such as
	// `facts:`), then pull every backticked span out of it.
	var block []string
	for _, l := range lines[labelIdx+1:] {
		t := strings.TrimSpace(l)
		if t == "" {
			break
		}
		isBullet := strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ")
		isIndentedCont := l != t // a leading-whitespace continuation of a bullet
		if !isBullet && !isIndentedCont {
			break // a new flush-left label ends the files: block
		}
		block = append(block, l)
	}
	blob := strings.Join(block, "\n")
	if spans := backtickSpanRe.FindAllStringSubmatch(blob, -1); len(spans) > 0 {
		for _, m := range spans {
			add(m[1])
		}
	} else {
		// No backticks: take the leading token of each bullet.
		for _, l := range block {
			t := mdLinkTextRe.ReplaceAllString(strings.TrimSpace(l), "$1")
			t = strings.TrimPrefix(strings.TrimPrefix(t, "- "), "* ")
			if f := strings.Fields(t); len(f) > 0 {
				add(f[0])
			}
		}
	}
	return paths, len(paths) > 0
}

// cleanDeclaredPath strips residual decoration from a candidate path token.
func cleanDeclaredPath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`")
	s = strings.TrimRight(s, ",;")
	return strings.TrimSpace(s)
}

// extractEvidence returns the body of the `## Evidence` section — the lines
// between that heading and the next `## ` heading (or EOF). Empty if absent.
// The heading must match `## Evidence` exactly (the fixed brief-v1 contract);
// a decorated heading like `## Evidence (notes)` yields no section.
func extractEvidence(body string) string {
	lines := strings.Split(body, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Evidence" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, l := range lines[start:] {
		if strings.HasPrefix(strings.TrimSpace(l), "## ") {
			break
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// evidenceHasContent reports whether an Evidence section has at least one
// content row: a non-empty line outside HTML comments (the contract comment
// alone does not count).
func evidenceHasContent(section string) bool {
	stripped := htmlCommentRe.ReplaceAllString(section, "")
	for _, l := range strings.Split(stripped, "\n") {
		if strings.TrimSpace(l) != "" {
			return true
		}
	}
	return false
}

// hasHumanReviewer reports whether a Reviewed-column value names a human — a
// whitespace-separated token with the "human:" prefix (e.g. "human:alex"). A tag
// that merely contains the substring (e.g. "superhuman:x") does NOT count.
func hasHumanReviewer(reviewed string) bool {
	for _, tok := range strings.Fields(reviewed) {
		if strings.HasPrefix(tok, "human:") {
			return true
		}
	}
	return false
}

// verifyTableHasRow reports whether a `## Verify` section body contains at least
// one table data row whose Command and Expect cells are both non-empty. It
// locates the header row naming the "Command" and "Expect" columns, then scans
// the data rows below it. This is a PRESENCE/STRUCTURE check only — it asserts
// the Verify table exists and has a runnable row, never that the row is any
// good (quality is the review gate's job, not this lint's).
func verifyTableHasRow(section string) bool {
	cmdIdx, expIdx := -1, -1
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			cmdIdx, expIdx = -1, -1 // left the table; the next one names its own columns
			continue
		}
		if separatorRowRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		cells := splitRow(line)
		if cmdIdx < 0 || expIdx < 0 {
			// Still looking for a header row that names Command and Expect.
			for j, c := range cells {
				switch strings.ToLower(strings.TrimSpace(c)) {
				case "command":
					cmdIdx = j
				case "expect":
					expIdx = j
				}
			}
			continue // the header row itself is not a data row
		}
		if cmdIdx < len(cells) && expIdx < len(cells) {
			cmd := normalizeMark(strings.TrimSpace(cells[cmdIdx]))
			exp := normalizeMark(strings.TrimSpace(cells[expIdx]))
			if cmd != "" && exp != "" {
				return true
			}
		}
	}
	return false
}

// verifySectionProblems is the methodology/19 Verify-table structure lint: every
// opted-in brief-v1 file must carry a `## Verify` section with at least one
// table row whose Command and Expect cells are non-empty. Scope is brief-v1
// files only (the same schema opt-in as checkBriefFiles); legacy no-frontmatter
// briefs are exempt. It is a SEPARATE check (like attributionProblems) so the
// file-structural rule runs on every brief regardless of its README-row status.
//
// Presence/structure ONLY: the error text names the missing structure,
// never quality — a Verify table that exists but is weak is the review gate's
// concern, not this lint's.
func verifySectionProblems(streams []*Stream) []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				// Malformed files are reported by checkBriefFiles; legacy/
				// opted-out files are exempt here as everywhere else.
				continue
			}
			if !verifyTableHasRow(bf.Verify) {
				add("%s: brief-v1 file needs a `## Verify` section with at least one table row (Command + Expect non-empty); this is a structure/presence check, not a judgement of the table's content — methodology/19", path)
			}
		}
	}
	sort.Strings(problems)
	return problems
}

// stringList coerces a YAML value into []string. A null (absent list content)
// is an empty list; anything that is not a list of strings is an error.
func stringList(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("must be a list")
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		s, ok := it.(string)
		if !ok {
			return nil, fmt.Errorf("must be a list of strings")
		}
		out = append(out, s)
	}
	return out, nil
}

// parallelStreamList coerces a YAML value into []ParallelStream. The shape is
// a list of mappings, each carrying a `name` and a `files` list of globs:
//
//	parallel-streams:
//	  - {name: engine, files: ["statusgen/**"]}
//	  - {name: docs,   files: ["docs/streams/example/**"]}
//
// A key other than name/files is REJECTED rather than ignored. A silently
// dropped key here would be a shard scoped by a field nobody reads — the shard
// would run wider than its author declared, which is the one failure mode a
// scoping declaration exists to prevent.
func parallelStreamList(v any) ([]ParallelStream, error) {
	if v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("must be a list of {name, files} mappings")
	}
	out := make([]ParallelStream, 0, len(items))
	for i, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("entry %d must be a mapping with name: and files:", i+1)
		}
		var ps ParallelStream
		for k := range m {
			if k != "name" && k != "files" {
				return nil, fmt.Errorf("entry %d has unknown key %q (only name and files are defined)", i+1, k)
			}
		}
		if n, ok := m["name"].(string); ok {
			ps.Name = n
		} else {
			return nil, fmt.Errorf("entry %d: name must be a string", i+1)
		}
		files, err := stringList(m["files"])
		if err != nil {
			return nil, fmt.Errorf("entry %d files: %v", i+1, err)
		}
		ps.Files = files
		out = append(out, ps)
	}
	return out, nil
}

// intList coerces a YAML value into []int (yaml.v3 decodes bare integers as int,
// or int64 when they overflow int).
func intList(v any) ([]int, error) {
	if v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("must be a list")
	}
	out := make([]int, 0, len(items))
	for _, it := range items {
		switch n := it.(type) {
		case int:
			out = append(out, n)
		case int64:
			out = append(out, int(n))
		default:
			return nil, fmt.Errorf("must be a list of integers")
		}
	}
	return out, nil
}

// riskMap validates the risk block. yaml.v3 (YAML 1.2 Core Schema) decodes bare
// `yes`/`no` as the strings "yes"/"no", so we require exactly those strings; a
// bare boolean (true/false) or any other value is a hard error, keeping the
// on-file convention identical to the author-brief template.
func riskMap(v any) (map[string]string, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("must be a mapping")
	}
	// The four canonical questions MUST all be answered — a missing question can
	// never fire the human gate (amendment item b).
	for _, k := range canonicalRiskKeys {
		if _, ok := m[k]; !ok {
			return nil, fmt.Errorf("missing canonical risk key %q (all four required: %s)", k, strings.Join(canonicalRiskKeys, ", "))
		}
	}
	out := make(map[string]string, len(m))
	for k, val := range m {
		if !canonicalRiskSet[k] {
			return nil, fmt.Errorf("unknown risk key %q (exactly the four canonical keys are allowed)", k)
		}
		s, ok := val.(string)
		if !ok || (s != "yes" && s != "no") {
			return nil, fmt.Errorf("%s must be yes or no", k)
		}
		out[k] = s
	}
	return out, nil
}

// checkBriefFiles validates every opted-in brief file in `streams` (the
// per-brief validation set — the product-scoped subset under --changed/--scope,
// or the whole house otherwise) and returns hard PROBLEM messages (exit 1) —
// path-prefixed. It performs its own file discovery and is wired into run()
// alongside linkProblems, keeping check() I/O-free.
//
// `allStreams` is the FULL house stream set and is used ONLY to resolve
// cross-stream `depends:`/`unblocks:` references. This mirrors
// checkScoped(scoped, all, findings): a single-product PR narrows WHICH briefs
// are validated, but a valid depends: may legitimately point at a stream that
// scoping dropped (another product's stream, a paused stream). Resolving refs
// against the scoped subset alone made such a valid dependency falsely report
// "unknown stream". Resolving against allStreams fixes that while
// still catching a genuinely-unknown/typo'd stream — it is absent from the full
// set too. Whole-house callers pass the same slice for both.
func checkBriefFiles(streams, allStreams []*Stream) (problems, notices []string) {
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	notice := func(format string, a ...any) { notices = append(notices, fmt.Sprintf(format, a...)) }

	byName := map[string]*Stream{}
	for _, s := range allStreams {
		byName[s.Name] = s
	}

	// Typed dependency-edge index for the reciprocity gate (phase 3), accumulated as
	// the brief files are parsed and validated by the reciprocity pass after the loop.
	recip := newDepEdgeIndex()

	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil {
				add("%s", err.Error()) // already path-prefixed
				continue
			}
			if !ok {
				continue // legacy / opted-out → exempt
			}

			id, num, okName := expectedBriefID(path)
			if !okName {
				add("%s: filename must match brief-<NN>[-slug].md", path)
				continue
			}
			if bf.Brief != id {
				add("%s: brief %q does not match filename-derived id %q", path, bf.Brief, id)
			}
			if !validEffort[bf.Effort] {
				add("%s: invalid effort %q (want S, M or L)", path, bf.Effort)
			}
			if !validGate[bf.Gate] {
				add("%s: invalid gate %q (want model or human)", path, bf.Gate)
			}
			// value is optional; only a present-but-unrecognized value is a
			// PROBLEM — absence defaults to med at scoring time and never
			// requires the field.
			if bf.Value != "" && !validValue[bf.Value] {
				add("%s: invalid value %q (want low, med or high)", path, bf.Value)
			}
			// domain is optional; only a present-but-unrecognized value is a
			// PROBLEM — absence defaults to complicated at read time and never
			// requires the field (additive schema change, agentic-metrics/10).
			if bf.Domain != "" && !validDomain[bf.Domain] {
				add("%s: invalid domain %q (want clear, complicated, complex or chaotic)", path, bf.Domain)
			}
			// exec-tier is optional; only a present-but-
			// unrecognized value is a PROBLEM — absence defaults to "any".
			if bf.ExecTier != "" && !validExecTier[bf.ExecTier] {
				add("%s: invalid exec-tier %q (want any or strong)", path, bf.ExecTier)
			}
			if bf.ExecTier == "strong" && strings.TrimSpace(bf.ExecTierWhy) == "" {
				notice("%s: brief %s has exec-tier: strong but no exec-tier-why — add a one-line rationale naming which derivation question(s) it answered yes", path, bf.Brief)
			}
			// blocked-by is optional; only a present-but-
			// unrecognized value is a PROBLEM — absence defaults to "" (not blocked).
			if bf.BlockedBy != "" && !validBlockedBy[bf.BlockedBy] {
				add("%s: invalid blocked-by %q (want env)", path, bf.BlockedBy)
			}
			// parallel-streams is optional; absence is the one-worker default
			// and is never flagged. What IS flagged is a declaration that
			// cannot describe a split: fewer than two shards, a nameless or
			// duplicated shard, or a shard with no globs. This is SHAPE only —
			// the collision classes (path overlap, shared surfaces, symbol
			// coupling) need the file tree and are decided by
			// `statusgen shardcheck`, which the dispatcher runs. The lint must
			// not imply a shape-valid split is a safe one.
			for _, f := range checkParallelStreamShape(bf.ParallelStreams) {
				add("%s: %s", path, f)
			}
			if len(bf.ParallelStreams) > 0 && bf.Effort == "S" {
				notice("%s: brief %s declares a %d-shard split on an Effort: S brief — the split is gated to work big enough to pay for the coordination, and an S brief is not it", path, bf.Brief, len(bf.ParallelStreams))
			}
			// measures is optional; only a PRESENT-but-unrecognized queue name is
			// a PROBLEM. Absence is the neutral default and is never flagged. A
			// typo must be loud here: at board-build time an unreadable queue is
			// could-not-check and the brief is held back, so a silently accepted
			// bad name would take a brief off the board with nothing to fix.
			if bf.Measures != nil && !validMeasuresQueue[strings.TrimSpace(*bf.Measures)] {
				add("%s: unknown measures queue %q (want one of: %s) — the drain-before-instrument gate can only read a wired queue",
					path, *bf.Measures, strings.Join(measuresQueueNames(), ", "))
			}
			// homed-in is optional; only a PRESENT-but-malformed shape is a hard
			// PROBLEM echoing the bad value. Absence defaults to "" (in-repo) and
			// is never flagged. A malformed value is left OFF the row below (like
			// value/exec-tier) so a typo cannot silently exclude a brief from
			// Next-up — it reddens lint instead.
			if bf.HomedIn != "" && !validHomedInShape(bf.HomedIn) {
				add("%s: invalid homed-in %q (want <owner>/<repo>)", path, bf.HomedIn)
			}
			anyYes := false
			for _, v := range bf.Risk {
				if v == "yes" {
					anyYes = true
					break
				}
			}
			if anyYes && bf.Gate != "human" {
				add("%s: a risk answer is yes but gate is %q (must be human)", path, bf.Gate)
			}
			// gate-why (gate-why-rationale, PHASE 3): a risk-gated brief
			// (gate: human OR any risk answer yes) MUST record WHY it is gated.
			// This is now a hard PROBLEM (exit 1) — the backfill landed,
			// so every risk-gated brief on main carries a
			// gate-why, and a new one added without it fails --lint.
			if (bf.Gate == "human" || anyYes) && strings.TrimSpace(bf.GateWhy) == "" {
				add("%s: brief %s is risk-gated but has no gate-why — add a gate-why explaining what makes this brief risky", path, bf.Brief)
			}
			// why NOTICE (PHASE 1): every brief-v1 brief SHOULD
			// carry a why: — one to three lines a non-engineer could read and
			// justify the work from (not just the what). This is a NON-FATAL
			// NOTICE this phase so the ~94 un-backfilled briefs do not red-CI
			// on --lint. PHASE 3 flips this to a hard add(...) error once the
			// per-brief backfill lands — same pattern as gate-why.
			// Backfill strategy: active-stream briefs first; done/archived briefs
			// exempt. Follow-up brief(s) mirror the same sequence.
			//
			// Substance floor (landed BEFORE the backfill on purpose):
			// presence alone made a title-paste, ".", or "TODO" score as fully
			// compliant — zero information, and a hard-error flip on top of that
			// would lock the emptiness in permanently. A present why: must also
			// clear a minimum length and not be a substring/near-duplicate of the
			// title. Still a NOTICE, same severity as the presence check — the
			// hard-error flip is a separate future step.
			if trimmed := strings.TrimSpace(bf.Why); trimmed == "" {
				notice("%s: brief %s has no why: — add a why: explaining what makes this work worth doing (one to three lines a non-engineer could justify the work from)", path, bf.Brief)
			} else if reason := whySubstanceIssue(bf.Why, bf.Title); reason != "" {
				notice("%s: brief %s has a why: that fails the substance floor (%s) — write independent rationale, not a restated title or placeholder", path, bf.Brief, reason)
			}
			if len(bf.Sources) == 0 {
				add("%s: sources must be non-empty", path)
			}

			// Cross-check the stream README table: the row for this brief exists
			// and its Wave matches the frontmatter.
			var row *Brief
			for i := range s.Briefs {
				if s.Briefs[i].Num == num {
					row = &s.Briefs[i]
					break
				}
			}
			if row == nil {
				add("%s: no row %q in the %s README brief table", path, num, s.Name)
			} else {
				// Wire BriefFile data into the Brief row for eligibility
				// gating: brief-v1 briefs are
				// gated on their own typed depends list, not the whole-wave
				// rule; legacy briefs keep Schema="" and Depends nil.
				row.Schema = bf.Schema
				row.Depends = bf.Depends
				// value flows into the Next-up score;
				// an invalid value is caught above and left off the row so the
				// score falls back to med rather than trusting a bad token.
				if validValue[bf.Value] {
					row.Value = bf.Value
				}
				// exec-tier flows into the Brief row as a marker:
				// "strong" renders [exec:strong] in Next-up and Awaiting; absent
				// or "any" renders nothing. NEVER a score input.
				// Invalid values are caught above and left off the row.
				if validExecTier[bf.ExecTier] {
					row.ExecTier = bf.ExecTier
				}
				// Gate worms from BriefFile into the Brief row for render-time
				// Awaiting-board segmentation. Always
				// wired — brief-v1 always carries a validated gate.
				row.Gate = bf.Gate
				// blocked-by worms into the Brief row for the env-blocked
				// segment. Only wired when valid;
				// invalid values are caught above and left off the row.
				if validBlockedBy[bf.BlockedBy] {
					row.BlockedBy = bf.BlockedBy
				}
				// homed-in worms into the Brief row for the Next-up eligibility
				// exclusion, the [homed→<owner/repo>] marker and the board-honesty
				// integration. Only wired when the shape is VALID; a malformed
				// value is caught above and left off the row (like value/exec-tier)
				// so a typo reddens lint rather than silently taking the brief off
				// the board. NEVER a Next-up score input (F-09 scope note).
				if validHomedInShape(bf.HomedIn) {
					row.HomedIn = bf.HomedIn
				}
				// measures worms into the Brief row for the drain-before-
				// instrument eligibility gate. Wired UNCONDITIONALLY — unlike
				// value/exec-tier/blocked-by, an invalid name is NOT dropped
				// back to the neutral default. Dropping it would convert a typo
				// into "no gate at all", which is precisely the fail-open the
				// gate exists to prevent; carried through, an unreadable queue
				// name becomes a named could-not-check on the board that points
				// straight at the brief to fix.
				row.Measures = bf.Measures
				// Evidence worms into the Brief row for render-time
				// VERIFY:PASS / VERIFY:FAIL classification.
				row.Evidence = bf.Evidence

				if row.Wave != bf.Wave {
					add("%s: frontmatter wave %d != README table wave %d", path, bf.Wave, row.Wave)
				}
				// A verified/done brief must carry real Evidence — a content row
				// beyond the contract comment.
				if (row.Status == "verified" || row.Status == "done") && !evidenceHasContent(bf.Evidence) {
					add("%s: status %q requires a filled ## Evidence section (a content row beyond the contract comment)", path, row.Status)
				}
				// A human-gated brief at done must name a human reviewer — a
				// "human:<name>" token in the Reviewed column; a
				// bare model sign-off does not close a risk-flagged brief.
				if bf.Gate == "human" && row.Status == "done" && !hasHumanReviewer(row.Reviewed) {
					add("%s: gate is human but the done Reviewed entry %q names no human — it needs a \"human:<name>\" token (e.g. 2026-07-15 human:alex)", path, row.Reviewed)
				}
				// An IRREVERSIBLE change — anything an author has flagged as unfixable
				// once shipped (on-ledger money AND publication-class work like a
				// released article or a committed-to name) — pulls the human gate one
				// step earlier: it may not even be marked `verified` on a model-only
				// sign-off. A model verifier can run the Verify table, but calling an
				// irreversible brief verified/done requires a human in the Reviewed
				// column; a model cannot pre-close a change that cannot be walked back.
				// If you flag irreversible:yes, a human signs off before verified.
				// (irreversible:yes implies gate:human via the anyYes check above, so
				// this only tightens the `verified` state.)
				if bf.Risk["irreversible"] == "yes" &&
					(row.Status == "verified" || row.Status == "done") && !hasHumanReviewer(row.Reviewed) {
					add("%s: risk.irreversible=yes but the %s Reviewed entry %q names no human — an irreversible brief needs a \"human:<name>\" review before it can be marked verified or done (a model verifier alone cannot close it)", path, row.Status, row.Reviewed)
				}

				// Risk-keyed verifier floor (methodology/19): a RISK-FLAGGED brief
				// (gate:human OR any risk answer yes) marked verified/done must be
				// verified by a human or by a runner ABOVE the floor — a model from
				// the belowFloorModels family list may verify a fully risk-clear
				// brief, but not a flagged one. The floor keys on CAPABILITY, not
				// price: see belowFloorModels in attribution.go for why a cheap-to-
				// run but strong model does not belong on the list. Irreversible
				// briefs are governed by the stricter human-at-verified rule just
				// above (via the Reviewed cell); that rule wins, so they
				// are EXEMPT here to avoid double-gating the same band.
				if (bf.Gate == "human" || anyYes) && bf.Risk["irreversible"] != "yes" &&
					(row.Status == "verified" || row.Status == "done") {
					if reason, failed := verifierFloorFailure(row.Verified); failed {
						add("%s: risk-flagged brief marked %s but the Verified cell %q does not clear the verifier floor — %s — risk-flagged briefs verify at a strong-tier runner or a human — methodology/19", path, row.Status, row.Verified, reason)
					} else if reason, failed := evidenceFloorFailure(bf.Evidence); failed {
						// The cell clears, but Evidence — the record of who actually
						// ran each row — shows the floor is not truly met. The floor
						// reads the complete signal, not just the one-line cell.
						add("%s: risk-flagged brief marked %s but its ## Evidence records rows run below the verifier floor with no strong-tier re-run curing them (%s) — the Verified cell %q names a clearing runner but does not speak for those rows — risk-flagged briefs verify at a strong-tier runner or a human — methodology/19", path, row.Status, reason, row.Verified)
					}
				}

				// Reviewed-cell attribution shape (methodology/19): at `done`, the
				// Reviewed cell must be a dated runner ("YYYY-MM-DD <runner>") — the
				// same shape attribution.go already requires of the Verified cell.
				// This makes the README's "runner-attribution on the Reviewed cell"
				// claim true. The gate:human human:<name> rule above is unchanged
				// (and remains stricter for risk-flagged briefs).
				if row.Status == "done" && !verifiedCellRe.MatchString(row.Reviewed) {
					add("%s: status done needs a dated Reviewed entry (\"YYYY-MM-DD <runner>\"); got %q — methodology/19", path, row.Reviewed)
				}

				// Decision-issue linkage, part (a): a gate:human
				// brief whose gate someone is actually WAITING on SHOULD carry a
				// decision-issue that tracks the human sign-off — the NOTICE fires
				// when the field is absent (invisible wait). "Waiting on" means
				// dispatched (in-progress) or awaiting sign-off (implemented/
				// verified). Backlog `todo` briefs are deliberately EXCLUDED here
				// — noticing every gated todo floods the register with items
				// nobody is waiting on; the brief's "top-of-Next-up"
				// case is covered by the Next-up-pick check in run() instead.
				if bf.Gate == "human" && bf.DecisionIssue == 0 &&
					(row.Status == "in-progress" || row.Status == "implemented" || row.Status == "verified") {
					notice("%s: brief %s is gate:human at %s but has no decision-issue — file one via --decision-issues", path, bf.Brief, row.Status)
				}
				// Decision-issue linkage, part (b): a done brief still carrying a
				// decision-issue whose outcome is NOT recorded in the brief body
				// — a decision was (presumably) made and closed without amending
				// the brief. A body reference to "#<NN>" counts as recorded; the
				// frontmatter linkage itself is the audit record and must STAY
				// (never advise deleting it). NOTICE only (not
				// PROBLEM); the true check (issue closed on GitHub, decision text
				// reflected) requires network access.
				if bf.DecisionIssue != 0 && row.Status == "done" &&
					!decisionIssueRefInBody(bf.Body, bf.DecisionIssue) {
					notice("%s: brief %s is done but its body never records the outcome of decision-issue #%d — append the chosen option (referencing #%d) to the brief; keep the frontmatter linkage as the audit record (part (b))", path, bf.Brief, bf.DecisionIssue, bf.DecisionIssue)
				}
				// Security-review recorded at done: a
				// risk-classed brief (gate:human OR any risk answer yes) at
				// `done` must carry the literal substring "security-review" in
				// its Reviewed cell (e.g. "2026-07-12 model:opus
				// +security-review(pass)"), matching the convention. NOTICE this
				// phase — the current tree has
				// risk-classed done rows with no such token; a follow-on brief
				// flips this to a hard PROBLEM after backfill (mirroring the
				// earlier backfill pattern).
				//
				// This is a FRONTMATTER-ONLY check: statusgen sees no diffs,
				// so the path-trigger class (a mislabeled brief touching a
				// risk-classified path, e.g. auth/) is covered by the desk-side
				// path triggers, not here. Only the brief's own risk classification
				// and its Reviewed cell are inspected.
				if (bf.Gate == "human" || anyYes) && row.Status == "done" && !strings.Contains(row.Reviewed, "security-review") {
					notice("%s: risk-classed brief %s is done but its Reviewed cell %q records no security-review — risk-classed briefs record a security review at done", path, bf.Brief, row.Reviewed)
				}
			}

			for _, ref := range bf.Depends {
				checkRef(add, path, "depends", ref, bf.Brief, byName)
			}
			// Accumulate the typed edge index for the reciprocity gate (below): the
			// declared depends/unblocks lists per brief-v1 id. Built here — where every
			// brief file is already parsed — so the reciprocity pass needs no second walk.
			recip.dependsOf[bf.Brief] = bf.Depends
			recip.unblocksOf[bf.Brief] = bf.Unblocks
			recip.knownV1[bf.Brief] = true
			// Same-wave-dep lint: a brief's depends: must point
			// only to briefs in strictly-earlier waves. A same-wave dep breaks strict
			// wave-layering and miscomputes the critical path.
			// Safety: bf.Wave == 0 here always means wave was legitimately 0.
			// An unparseable wave causes parseBriefFile to return an error, and
			// checkBriefFiles skips the brief entirely via continue before reaching
			// this check. The dep wave is from the README table (authoritative);
			// frontmatter/README wave consistency for the dep is enforced at its
			// own lint pass where frontmatter.Wave is compared to README.Wave.
			for _, ref := range bf.Depends {
				parts := strings.SplitN(ref, "/", 2)
				if len(parts) != 2 {
					continue // malformed refs are caught by checkRef above
				}
				depStream, ok := byName[parts[0]]
				if !ok {
					continue // unknown stream caught by checkRef
				}
				// Find the dep's brief row to get its wave.
				depWave := -1
				for i := range depStream.Briefs {
					if depStream.Briefs[i].Num == parts[1] {
						depWave = depStream.Briefs[i].Wave
						break
					}
				}
				if depWave < 0 {
					continue // unknown brief caught by checkRef
				}
				if depWave >= bf.Wave {
					notice("%s: depends on %s (wave %d) but this brief is wave %d — deps must point to strictly-earlier waves (depWave < briefWave)",
						path, ref, depWave, bf.Wave)
				}
			}
			for _, ref := range bf.Unblocks {
				checkRef(add, path, "unblocks", ref, bf.Brief, byName)
			}
		}
	}
	// Dependency-edge reciprocity lint (phase 3, anti-gaming): every depends edge
	// feeding blockedCount should be reciprocated by the target's unblocks, so a brief
	// cannot inflate its blockedCount into the critical tier by manufacturing spurious
	// one-sided inbound edges. Runs once over the accumulated index.
	//
	// NOTICE tier (Ian's ruling): this is a data-quality lint, not a
	// security control. statusgen HEAD tightened it from a phase-1 blockedCount
	// heuristic into a hard reciprocity requirement, and ~104 legitimate older
	// one-sided edges across the source repo's streams predate the two-sided
	// convention — so shipping it at PROBLEM would redden statusgen-lint
	// board-wide on the next release + repin. It ships at NOTICE (non-fatal, exit
	// 0) until those edges are reconciled two-sided, then returns to PROBLEM.
	// Follow-up reconciliation is tracked as a separate backlog item (below).
	notices = append(notices, recip.reciprocityNotices()...)
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

// depEdgeIndex is the typed dependency-edge index the reciprocity gate validates.
type depEdgeIndex struct {
	dependsOf  map[string][]string // brief id → declared depends ids
	unblocksOf map[string][]string // brief id → declared unblocks ids
	knownV1    map[string]bool     // brief-v1 ids present in the corpus
}

func newDepEdgeIndex() *depEdgeIndex {
	return &depEdgeIndex{
		dependsOf:  map[string][]string{},
		unblocksOf: map[string][]string{},
		knownV1:    map[string]bool{},
	}
}

// reciprocityNotices is the anti-gaming reciprocity lint (phase 3). blockedCount is
// the reverse typed-`depends:` walk (buildRevDeps), so a brief could climb into the
// high-unblocks arm of the critical tier by manufacturing spurious INBOUND depends
// edges. This flags any depends edge A→B that B does not reciprocate with an
// `unblocks: A` — a genuine dependency is two-sided (the author-brief methodology
// requires both Depends-on and Unblocks), so an unreciprocated inbound edge is
// spurious. Reconciling every edge two-sided makes blockedCount reflect only
// genuine, both-sided dependencies and un-gameable into the tier.
//
// TIER — NOTICE, not PROBLEM (Ian's ruling). This is a data-quality
// lint. The rule is stricter than the pinned release's, and ~104 legitimate
// older one-sided edges across the source repo's streams predate the two-sided
// convention; emitting PROBLEM would red statusgen-lint board-wide on the next
// release + repin. It emits NOTICE (non-fatal) until those edges are reconciled,
// then flips back to PROBLEM — tracked as a separate backlog item. The
// caller (checkBriefFiles) routes this into the `notices` channel accordingly.
//
// SCOPE (fail-safe against false positives): the check fires only when BOTH endpoints
// are brief-v1 (in knownV1). A dangling edge (target absent) is left to checkRef; a
// self-loop is left to checkRef; a legacy (non-brief-v1) target — which declares no
// unblocks — is exempt so mixed corpora are not reddened for the pre-typed-edge
// convention.
func (idx *depEdgeIndex) reciprocityNotices() []string {
	var notices []string
	var ids []string
	for id := range idx.dependsOf {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, a := range ids {
		for _, b := range idx.dependsOf[a] {
			if b == a || !idx.knownV1[b] {
				continue // self-loop (checkRef) / dangling (checkRef) / legacy target (exempt)
			}
			reciprocated := false
			for _, u := range idx.unblocksOf[b] {
				if u == a {
					reciprocated = true
					break
				}
			}
			if !reciprocated {
				notices = append(notices, fmt.Sprintf(
					"%s: depends edge to %s is one-sided — %s declares `depends: %s` but %s does not list %s in its `unblocks:`. A genuine dependency is reciprocal (the target unblocks the dependent); reconcile the edge two-sided so blockedCount cannot be inflated into the critical tier (anti-gaming). Data-quality NOTICE; flips back to PROBLEM once the backlog is reconciled.",
					a, b, a, b, b, a))
			}
		}
	}
	return notices
}

// checkRef validates that a typed ID "<stream>/<NN>" resolves to a real brief
// row in a known stream. Brief numbers compare as strings to preserve leading
// zeros and alphanumeric suffixes ("12a"). selfID is the id of the brief that
// DECLARED the ref: a ref equal to it is self-referential (`a depends on a`) and a
// hard PROBLEM — part of the dependency-edge reciprocity gate (phase 3) that keeps
// blockedCount un-gameable. Pass "" to skip the self-ref check.
func checkRef(add func(string, ...any), path, kind, ref, selfID string, byName map[string]*Stream) {
	if selfID != "" && ref == selfID {
		add("%s: %s %q is self-referential (a brief may not %s itself) — a self-loop is a spurious dependency edge that would inflate blockedCount", path, kind, ref, kind)
		return
	}
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		add("%s: %s %q is not a <stream>/<NN> id", path, kind, ref)
		return
	}
	s, ok := byName[parts[0]]
	if !ok {
		add("%s: %s %q references unknown stream %q", path, kind, ref, parts[0])
		return
	}
	for i := range s.Briefs {
		if s.Briefs[i].Num == parts[1] {
			return
		}
	}
	add("%s: %s %q references unknown brief %q in stream %s", path, kind, ref, parts[1], parts[0])
}
