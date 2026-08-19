package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// followUpTargetRe matches a `follow-up <stream>/<NN>` target inside a routing
// string. The number keeps the `12a` suffix form brief IDs already allow.
var followUpTargetRe = regexp.MustCompile(`follow-up\s+([a-z][a-z0-9]*(?:-[a-z0-9]+)*)/([0-9]+[a-z]?)\b`)

// ---------------------------------------------------------------------------
// consumers: — the routed consumer list of a shared-value brief (brief-rule 9).
//
// DESIGN NOTE (F-impl-claims-unproven). A `consumers:` entry is an
// IMPLEMENTER-WRITTEN CLAIM: "this reader of the shared value is fixed here /
// deferred to that brief / deliberately excluded". The finding
// F-impl-claims-unproven records what happens to implementer-written claims that
// no gate corroborates — they reach `implemented` false, and the machinery that
// checks only that the claim is PRESENT makes the falsehood look enforced.
//
// So this file never asks "is the field there?". Every entry is put into one of
// the three states the desk instrument rule demands, and the state is derived
// from evidence outside the brief:
//
//	CORROBORATED — an independent artifact agrees with the claim. `fixed-here`
//	               means the named path is in this branch's diff; `follow-up
//	               <stream>/<NN>` means that brief exists AND references back.
//	DISPROVED    — the artifact CONTRADICTS the claim. This is a hard failure:
//	               `--consumers` exits 1, so a false entry cannot merge.
//	UNCHECKED    — the instrument cannot see the evidence (an out-of-repo path,
//	               a prose component name, or `out-of-scope`, whose truth is a
//	               judgement no diff can settle). Never silently counted as a
//	               pass: it is printed, counted, and the residue is what the
//	               brief's Verify "Expect" column hands to the reviewer.
//
// Severity split between the two entry points:
//
//	--lint (whole corpus, offline, no diff): reports only what is decidable
//	    without a diff. A `follow-up` naming a brief that does not exist is
//	    DISPROVED by the stream tables alone → PROBLEM. Everything a diff would
//	    be needed to settle is a NOTICE here, because a corpus-wide lint runs
//	    against briefs merged months ago whose diff is long gone.
//	--consumers (this branch, diff-aware): the gate. Scoped to the briefs the
//	    branch actually touches, so it judges claims against the diff that was
//	    supposed to make them true. Exits 1 on any DISPROVED entry.
// ---------------------------------------------------------------------------

// Routing tokens. The grammar is deliberately tolerant of trailing detail —
// `fixed-here (bumps the pin in the same commit)` carries the most useful part
// of the entry, and a grammar that rejected it would be conformed to by deleting
// information.
//
// Tolerant of trailing detail, NOT of the token itself. `Fixed-here` and
// `fixed-here` preceded by two spaces are the same claim to every human who
// reads the brief; if they parsed differently, whether a claim is checkable at
// all would be back under the claimant's control at the cost of one keystroke —
// F-impl-claims-unproven wearing a typo as a disguise. Case and the space run
// after the colon are therefore normalized before matching.
const (
	routingFixedHere  = "fixed-here"
	routingFollowUp   = "follow-up"
	routingOutOfScope = "out-of-scope"
	routingUnknown    = ""
)

// consumerEntry is one parsed `consumers:` list item.
type consumerEntry struct {
	Raw     string   // the entry as written
	Site    string   // everything left of the routing token
	Routing string   // one of the routing* constants; routingUnknown when unparseable
	Detail  string   // the routing token and everything after it
	Targets []string // follow-up targets, typed "<stream>/<NN>" (possibly empty)
}

// Verdict states. Strings, not an enum, because they are printed verbatim and
// the printed word IS the contract with the reader.
const (
	stateCorroborated = "CORROBORATED"
	stateDisproved    = "DISPROVED"
	stateUnchecked    = "UNCHECKED"
)

type consumerVerdict struct {
	Brief  string
	Entry  string
	State  string
	Reason string
}

// routingRe finds a colon-introduced routing token. Anchoring on the colon
// rather than splitting on the last `": "` keeps a reason that itself contains a
// colon — `out-of-scope (blocked: #123)` — from being reported as unroutable.
//
// `(?i)` and `[ \t]*` are the finding-3 fix: the token is matched however it was
// spaced or capitalized. The trailing class is the token boundary — the same
// character set the old isRoutingBoundary used — so `fixed-hereabouts` still
// does not parse as `fixed-here`.
var routingRe = regexp.MustCompile(`(?i):[ \t]*(fixed-here|follow-up|out-of-scope)(?:[^A-Za-z0-9_-]|$)`)

// parseConsumerEntry splits one `consumers:` item into its site and routing.
// An item with no recognizable routing token returns Routing == routingUnknown
// with the whole item as Site — never an error: an unroutable entry is reported
// as such, not silently dropped.
func parseConsumerEntry(raw string) consumerEntry {
	e := consumerEntry{Raw: raw, Site: strings.TrimSpace(raw), Routing: routingUnknown}
	ms := routingRe.FindAllStringSubmatchIndex(raw, -1)
	if len(ms) == 0 {
		return e
	}
	// The LAST token wins: a site may legitimately quote one ("docs/a.md:
	// 'Notes: part two': fixed-here").
	m := ms[len(ms)-1]
	e.Routing = strings.ToLower(raw[m[2]:m[3]])
	e.Site = strings.TrimSpace(raw[:m[0]])
	e.Detail = strings.TrimSpace(raw[m[2]:])
	if e.Routing == routingFollowUp {
		e.Targets = followUpTargets(e.Detail)
	}
	return e
}

// followUpTargets extracts every `follow-up <stream>/<NN>` target in the routing
// text. Plural on purpose — a single consumer legitimately routes to two briefs
// ("follow-up example-app/41 (reads it), follow-up example-app/42 (reads it)"), and dropping the
// second would let an unbacked promise through.
func followUpTargets(detail string) []string {
	var out []string
	for _, m := range followUpTargetRe.FindAllStringSubmatch(detail, -1) {
		out = append(out, m[1]+"/"+m[2])
	}
	return out
}

// outOfScopeReason returns the parenthesized reason of an `out-of-scope (...)`
// routing, or "" when none was given.
func outOfScopeReason(detail string) string {
	open := strings.Index(detail, "(")
	if open < 0 {
		return ""
	}
	close := strings.LastIndex(detail, ")")
	if close <= open {
		return ""
	}
	return strings.TrimSpace(detail[open+1 : close])
}

// minOutOfScopeReasonLength is the floor on an out-of-scope reason, mirroring
// minWhySubstanceLength: "(n/a)" is a placeholder, not a reason a reviewer can
// weigh. The truth of the exclusion is never machine-checkable — the floor only
// keeps the entry from being content-free.
const minOutOfScopeReasonLength = 12

// ---------------------------------------------------------------------------
// Site classification
// ---------------------------------------------------------------------------

type siteKind int

const (
	siteRepoPath siteKind = iota // a path this root can resolve (or prove absent)
	siteForeign                  // out-of-repo / another tree — invisible from here
	siteProse                    // a component described in words, not a path
)

// site is a classified consumer site.
type site struct {
	Kind    siteKind
	Token   string   // the path token, when Kind == siteRepoPath
	Matches []string // repo-relative paths the token resolves to; empty = unresolved
	Reason  string   // why the site is unreachable, when Kind != siteRepoPath
}

// classifySite decides what, if anything, this root can say about a consumer
// site. The three kinds map onto the three instrument states: only siteRepoPath
// can be corroborated or disproved here.
func classifySite(root, raw string) site {
	s := strings.TrimSpace(raw)
	if s == "" {
		return site{Kind: siteProse, Reason: "empty site"}
	}
	if strings.HasPrefix(s, "[") {
		return site{Kind: siteForeign, Reason: "tagged for another repo/tree — not visible from this root"}
	}
	tok := strings.TrimRight(strings.Fields(s)[0], ",;:")
	if strings.HasPrefix(tok, "~") || strings.HasPrefix(tok, "/") {
		return site{Kind: siteForeign, Reason: "out-of-repo path — not visible from this root"}
	}
	if !pathish(tok) {
		return site{Kind: siteProse, Reason: "names a component in prose, not a path this root can resolve"}
	}
	matches, err := resolveSitePath(root, tok)
	if err != nil {
		return site{Kind: siteProse, Token: tok, Reason: err.Error()}
	}
	return site{Kind: siteRepoPath, Token: tok, Matches: matches}
}

// pathish reports whether a token looks like a repo path: it contains a
// separator, or it is a bare filename with an extension (CLAUDE.md, go.work).
func pathish(tok string) bool {
	if strings.Contains(tok, "/") {
		return true
	}
	dot := strings.LastIndex(tok, ".")
	return dot > 0 && dot < len(tok)-1
}

// resolveSitePath expands brace alternatives and globs a site token against the
// root, returning the repo-relative paths it matches (empty when it matches
// nothing — an ABSENT path, which is data, not an error). An error is returned
// only for a token that tries to escape the root.
func resolveSitePath(root, tok string) ([]string, error) {
	clean := strings.TrimSuffix(tok, "/")
	var out []string
	seen := map[string]bool{}
	for _, alt := range expandBraces(clean) {
		rel := filepath.Clean(alt)
		if rel == ".." || strings.HasPrefix(rel, "../") {
			return nil, fmt.Errorf("path escapes the repo root")
		}
		matches, err := filepath.Glob(filepath.Join(root, rel))
		if err != nil {
			continue // malformed pattern: matches nothing, same as absent
		}
		for _, m := range matches {
			r, rerr := filepath.Rel(root, m)
			if rerr != nil || seen[r] {
				continue
			}
			seen[r] = true
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out, nil
}

// expandBraces expands `a/{b,c}/d` into `a/b/d`, `a/c/d`. Brace lists are the
// established shorthand in real consumer entries
// (`.claude/skills/{the-desk,verify-desk}/SKILL.md`); without expansion every
// such entry would resolve to nothing and read as absent.
func expandBraces(s string) []string {
	open := strings.Index(s, "{")
	if open < 0 {
		return []string{s}
	}
	depth, closeIdx := 0, -1
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				closeIdx = i
			}
		}
		if closeIdx >= 0 {
			break
		}
	}
	if closeIdx < 0 {
		return []string{s} // unbalanced: treat literally
	}
	var out []string
	for _, alt := range splitTopLevel(s[open+1 : closeIdx]) {
		for _, tail := range expandBraces(s[closeIdx+1:]) {
			out = append(out, s[:open]+alt+tail)
		}
	}
	return out
}

// splitTopLevel splits a brace body on commas that are not inside a nested brace.
func splitTopLevel(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// ---------------------------------------------------------------------------
// Offline lint half — runs on the whole corpus, no diff available
// ---------------------------------------------------------------------------

// consumersCheck validates every brief's `consumers:` list with the evidence
// available offline.
//
// PROBLEM is reserved for a claim the stream tables DISPROVE — a `follow-up`
// naming a brief that does not exist. Everything else is a NOTICE, because
// without a diff this pass genuinely cannot tell a false `fixed-here` from a
// path the brief legitimately deleted; asserting either way would be the
// instrument reporting an answer it cannot establish. `--consumers` settles
// those against the branch diff.
func consumersCheck(root string, streams []*Stream) (problems, notices []string) {
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	notice := func(format string, a ...any) { notices = append(notices, fmt.Sprintf(format, a...)) }

	briefIndex := indexBriefIDs(streams)

	for _, s := range streams {
		statusByNum := map[string]string{}
		for i := range s.Briefs {
			statusByNum[s.Briefs[i].Num] = s.Briefs[i].Status
		}
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // reported by checkBriefFiles / exempt
			}
			_, num, okName := expectedBriefID(path)
			status := ""
			if okName {
				status = statusByNum[num]
			}
			// A closed brief is never rewritten to satisfy a lint added after it
			// landed — editing a shipped brief to green a gate is the
			// falsification this whole subsystem exists to catch. Advisory
			// notices about the SHAPE of a closed brief's list are therefore
			// suppressed; entry-level findings (below) still print, because they
			// are statements about the world, not demands to edit the file.
			open := status != "verified" && status != "done"
			if bf.ConsumersProse != "" {
				notice("%s: consumers: is a prose paragraph, not a routed list — nothing can corroborate it; use `- \"<site>: fixed-here|follow-up <stream>/<NN>|out-of-scope (<why>)\"` items (brief-rule 9)", path)
				continue
			}
			if len(bf.Consumers) == 0 {
				// The other half of brief-rule 9: a brief that changes a shared
				// value must enumerate its readers. Absence of the list is the
				// only thing detectable here, and only by a heuristic over prose
				// — so this is a PROMPT to the author, never a verdict, and it
				// never becomes a PROBLEM. Deliberately NARROW: it fires on
				// phrases that name a cross-component surface outright, not on
				// common English. Precision was measured against this repo's own
				// corpus at authoring time (Verify row 6).
				if phrase := sharedValueTrigger(bf.Body); open && phrase != "" {
					notice("%s: brief %s reads as changing a shared surface (%q) but enumerates no consumers: — list each reader and route it, or say why the phrase does not apply (brief-rule 9; heuristic prompt, never a verdict)", path, bf.Brief, phrase)
				}
				continue
			}
			// The corroboration command belongs in the Verify table: it is the
			// row a verifier re-runs, and the "Expect" column is where the
			// residue no machine can settle is written for a human to weigh
			// (F-impl-claims-unproven, recommendation 2).
			if open && !strings.Contains(bf.Verify, "--consumers") {
				notice("%s: brief %s carries consumers: but no Verify row runs `statusgen --consumers` — the routing claims are asserted in frontmatter with nothing corroborating them", path, bf.Brief)
			}
			for _, raw := range bf.Consumers {
				e := parseConsumerEntry(raw)
				switch e.Routing {
				case routingUnknown:
					notice("%s: consumers entry %q names no routing — end it with `: fixed-here`, `: follow-up <stream>/<NN>` or `: out-of-scope (<why>)` (brief-rule 9)", path, raw)
				case routingFollowUp:
					if len(e.Targets) == 0 {
						notice("%s: consumers entry %q routes to a follow-up but names no <stream>/<NN> target — a follow-up nobody owns is a deferral with no holder", path, raw)
						continue
					}
					for _, t := range e.Targets {
						if !briefIndex[t] {
							add("%s: consumers entry %q routes to follow-up %s, which is not a brief in any stream README — the routing claim is false", path, raw, t)
							continue
						}
						if !followUpReferencesBack(streams, t, bf.Brief) {
							notice("%s: consumers entry %q routes to follow-up %s, but %s never references %s — the coverage claim is one-way and unverified", path, raw, t, t, bf.Brief)
						}
					}
				case routingOutOfScope:
					if reason := outOfScopeReason(e.Detail); len(reason) < minOutOfScopeReasonLength {
						notice("%s: consumers entry %q is routed out-of-scope with no substantive reason — an exclusion no machine can check must at least state why, for the reviewer who can", path, raw)
					}
				}
			}
		}
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

// sharedValuePhrases are the narrow, high-precision phrases that say outright
// that a change is read by something other than the site being changed. Single
// common words ("secret", "party", "identity") were deliberately NOT included:
// on a prose corpus they fire everywhere, and an advisory that fires everywhere
// is trained away rather than acted on.
var sharedValuePhrases = []string{
	"shared value",
	"shared surface",
	"every consumer",
	"consumers of",
	"downstream consumer",
	"wire format",
	"breaking change to the schema",
}

// sharedValueTrigger returns the phrase that makes a brief read as shared-value-
// touching, or "" when none does.
func sharedValueTrigger(body string) string {
	lower := strings.ToLower(body)
	for _, p := range sharedValuePhrases {
		if strings.Contains(lower, p) {
			return p
		}
	}
	return ""
}

// indexBriefIDs builds the set of every "<stream>/<NN>" that has a README row.
func indexBriefIDs(streams []*Stream) map[string]bool {
	idx := map[string]bool{}
	for _, s := range streams {
		for i := range s.Briefs {
			idx[s.Name+"/"+s.Briefs[i].Num] = true
		}
	}
	return idx
}

// followUpReferencesBack reports whether the follow-up brief target names the
// referring brief anywhere — in `depends:` or in its body. A follow-up that has
// never heard of the brief deferring work to it is a promise with no holder;
// this is the cheapest evidence that the deferral was actually landed somewhere.
func followUpReferencesBack(streams []*Stream, target, referrer string) bool {
	stream, num, ok := strings.Cut(target, "/")
	if !ok {
		return false
	}
	for _, s := range streams {
		if s.Name != stream {
			continue
		}
		for _, path := range briefFilePaths(s) {
			id, _, okName := expectedBriefID(path)
			if !okName || id != stream+"/"+num {
				continue
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return false
			}
			return strings.Contains(string(raw), referrer)
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Diff-aware corroboration — the gate
// ---------------------------------------------------------------------------

// runConsumers is the `--consumers` sub-command: corroborate the routing claims
// of the briefs this branch touches against the branch's own diff.
//
// Exit codes are three-state, matching what the run established:
//
//	0 — no entry was disproved (corroborated + unchecked entries are listed).
//	1 — at least one entry was DISPROVED: the branch contradicts the claim.
//	2 — the instrument could not run at all (no git, unresolvable base). NOT 0:
//	    "could not check" must never be reported as "checked clean".
func runConsumers(root, base, briefFilter string) int {
	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: --consumers:", err)
		return 2
	}
	// Pin the base to a concrete commit ONCE, before any diff is taken, so the
	// whole run is a pure function of (merge-base, HEAD, tree) — reproducible on
	// re-run regardless of what the base ref does afterward.
	//
	// The gate is called with a live, MOVING ref (`origin/main`). Its two git
	// consumers — changedPathsSince's three-dot diff and consumerEntriesAtBase's
	// per-brief `merge-base`/`show` — each re-read that ref in a separate
	// subprocess. When a background commit advances origin/main between those
	// reads (or between two CI invocations of the identical command), the branch
	// three-dot base shifts underneath them and the SAME command flips its exit
	// code purely on ref motion — a flappy gate whose answer depends on WHEN it
	// runs, not on the PR's content. Resolving the ref to its merge-base SHA here
	// fixes it for every downstream read at once. This is a pure hardening: the
	// three-dot diff and the per-brief merge-base already reduced to
	// merge-base(base, HEAD), so pinning to that SHA changes no verdict — it only
	// removes the drift. When the merge-base cannot be resolved (no git, a
	// shallow clone missing it) we keep the ref as written; the diff step below
	// then reports COULD-NOT-CHECK exactly as it did before.
	if pinned, perr := pinConsumerBase(root, base); perr == nil {
		base = pinned
	}
	changed, err := changedPathsSince(root, base)
	if err != nil {
		fmt.Fprintf(os.Stderr, "statusgen: --consumers: COULD-NOT-CHECK: %v (base %q) — no diff, so no routing claim was corroborated\n", err, base)
		return 2
	}
	changedSet := map[string]bool{}
	for _, p := range changed {
		changedSet[p] = true
	}

	briefs := selectConsumerBriefs(root, streams, changedSet, briefFilter)
	if len(briefs) == 0 {
		if briefFilter != "" {
			fmt.Fprintf(os.Stderr, "statusgen: --consumers: COULD-NOT-CHECK: no brief-v1 file for %q\n", briefFilter)
			return 2
		}
		fmt.Printf("consumers: no brief files in the diff against %s — nothing to corroborate\n", base)
		return 0
	}

	// --brief lifts the diff SCOPING, not the need for a diff. Re-run after the
	// merge — HEAD == base, empty diff — and every `fixed-here` in the named
	// brief would be DISPROVED by the absence of the branch that made it true,
	// handing the post-merge verifier a red table and a page of false verdicts.
	// The instrument says COULD-NOT-CHECK instead,
	// and names the base that would work.
	if briefFilter != "" {
		for _, bf := range briefs {
			rel, err := filepath.Rel(root, bf.Path)
			if err != nil || changedSet[filepath.ToSlash(rel)] {
				continue
			}
			fmt.Fprintf(os.Stderr, "statusgen: --consumers: COULD-NOT-CHECK: %s is not in the diff against %s, so this run carries no evidence about its claims — no entry was corroborated and none was disproved.\n", bf.Brief, base)
			fmt.Fprintf(os.Stderr, "  To corroborate a MERGED brief, re-run against the diff that made the claims: check out the branch head and pass its own merge-base.\n")
			fmt.Fprintf(os.Stderr, "  For a merge commit M:  git checkout M^2 && statusgen --consumers --brief %s --base $(git merge-base M^1 M^2)\n", bf.Brief)
			return 2
		}
	}

	var verdicts []consumerVerdict
	var claimless []string
	for _, bf := range briefs {
		if bf.ConsumersProse == "" && len(bf.Consumers) == 0 {
			claimless = append(claimless, bf.Brief)
			continue
		}
		rel, err := filepath.Rel(root, bf.Path)
		if err != nil {
			rel = bf.Path
		}
		inherited := consumerEntriesAtBase(root, base, rel)
		verdicts = append(verdicts, corroborateBrief(root, streams, changedSet, inherited, bf)...)
	}

	nC, nD, nU := 0, 0, 0
	fmt.Printf("consumers corroboration — base %s (committed diff + uncommitted working tree), %d brief(s)\n", base, len(briefs))
	current := ""
	for _, v := range verdicts {
		if v.Brief != current {
			current = v.Brief
			fmt.Printf("\n%s\n", current)
		}
		switch v.State {
		case stateCorroborated:
			nC++
		case stateDisproved:
			nD++
		default:
			nU++
		}
		fmt.Printf("  %-13s %s\n", v.State, v.Entry)
		if v.Reason != "" {
			fmt.Printf("  %-13s   ↳ %s\n", "", v.Reason)
		}
	}
	// A brief in the diff that claims NOTHING and a brief whose claims all
	// corroborate used to print the same summary line and the same exit code —
	// an UNRUN check indistinguishable from a clean one, in the subsystem built
	// to tell those apart. They are now separate
	// counts with a separate roster.
	if len(claimless) > 0 {
		sort.Strings(claimless)
		fmt.Printf("\nno consumers: entries — the gate had nothing to check here, which is NOT a clean check:\n")
		for _, b := range claimless {
			fmt.Printf("  NO-CLAIM       %s\n", b)
		}
	}

	fmt.Printf("\nsummary: %d corroborated, %d disproved, %d unchecked, %d brief(s) claiming nothing\n", nC, nD, nU, len(claimless))
	if nU > 0 {
		fmt.Printf("UNCHECKED entries are NOT passes — each one's truth is the reviewer's call, stated in the brief's Verify Expect column (brief-rule 9).\n")
	}
	// Stated every run, because the limit is invisible in a green result: this
	// gate judges the entries that were WRITTEN. It never establishes that the
	// list is complete, so a consumer left off the list is not a finding here.
	fmt.Printf("LIMIT: only the entries written above were judged — an OMITTED consumer is invisible to this gate.\n")
	if nD > 0 {
		fmt.Fprintf(os.Stderr, "statusgen: --consumers: %d routing claim(s) DISPROVED by the diff against %s\n", nD, base)
		return 1
	}
	return 0
}

// selectConsumerBriefs picks the briefs to corroborate: the one named by
// --brief, else every brief-v1 file the diff touches. Scoping to touched briefs
// keeps the run small — an untouched brief's `fixed-here` refers to ITS branch's
// diff, not this one's.
//
// It is only HALF the protection, and the review measured the other
// half's absence: a brief the branch touches is not the same as a claim the
// branch makes, and an Evidence-only edit put six merged briefs on trial against
// a diff that never mentioned them. consumerEntriesAtBase supplies the rest.
func selectConsumerBriefs(root string, streams []*Stream, changed map[string]bool, filter string) []*BriefFile {
	var out []*BriefFile
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				continue
			}
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue
			}
			if filter != "" {
				if bf.Brief == filter {
					out = append(out, bf)
				}
				continue
			}
			if changed[filepath.ToSlash(rel)] {
				out = append(out, bf)
			}
		}
	}
	return out
}

// corroborateBrief turns one brief's consumer claims into verdicts.
//
// inherited is the entry set this brief already carried at the merge-base: those
// claims belong to the branch that wrote them and are UNCHECKED here (see
// consumerEntriesAtBase).
func corroborateBrief(root string, streams []*Stream, changed map[string]bool, inherited map[string]bool, bf *BriefFile) []consumerVerdict {
	var out []consumerVerdict
	add := func(state, entry, reason string) {
		out = append(out, consumerVerdict{Brief: bf.Brief, Entry: entry, State: state, Reason: reason})
	}
	if bf.ConsumersProse != "" {
		add(stateUnchecked, "consumers: (prose paragraph)", "not a routed list — no entry to corroborate")
		return out
	}
	briefIndex := indexBriefIDs(streams)
	for _, raw := range bf.Consumers {
		if inherited[strings.TrimSpace(raw)] {
			add(stateUnchecked, raw, "unchanged since the merge-base — this branch did not make this claim, so this branch's diff is not evidence about it")
			continue
		}
		e := parseConsumerEntry(raw)
		switch e.Routing {
		case routingUnknown:
			// Decidable without any diff, and decided against the entry: the
			// claimant wrote a routing nobody — no machine and no reviewer —
			// can read. Leaving it UNCHECKED would make "write it unreadably"
			// the cheapest way past the gate.
			add(stateDisproved, raw, "no routing token — end the entry with `: fixed-here`, `: follow-up <stream>/<NN>` or `: out-of-scope (<why>)`; an entry stating no routing claims nothing and can corroborate nothing")
		case routingFixedHere:
			st := classifySite(root, e.Site)
			missing := untouched(root, changed, st.Matches)
			switch {
			case st.Kind != siteRepoPath:
				add(stateUnchecked, raw, st.Reason)
			case len(st.Matches) == 0 && changed[filepath.ToSlash(strings.TrimSuffix(st.Token, "/"))]:
				// Absent from the tree but present in the diff: the branch
				// deleted or renamed it, which IS the fix.
				add(stateCorroborated, raw, "path is absent from the tree but present in the diff (deleted/renamed here)")
			case len(st.Matches) == 0:
				add(stateDisproved, raw, fmt.Sprintf("claims fixed-here but %q resolves to nothing in the tree and appears nowhere in the diff", st.Token))
			case len(missing) == 0:
				add(stateCorroborated, raw, directoryEvidence(root, changed, st.Matches))
			case len(missing) == len(st.Matches):
				add(stateDisproved, raw, fmt.Sprintf("claims fixed-here but no path it resolves to (%s) appears in the diff", strings.Join(st.Matches, ", ")))
			default:
				add(stateDisproved, raw, fmt.Sprintf("claims fixed-here for %d path(s) but the diff leaves %d of them untouched (%s) — a site naming a set claims the whole set; split the entry if only part of it was fixed", len(st.Matches), len(missing), strings.Join(missing, ", ")))
			}
		case routingFollowUp:
			if len(e.Targets) == 0 {
				add(stateUnchecked, raw, "follow-up names no <stream>/<NN> target — no brief to check")
				continue
			}
			for _, t := range e.Targets {
				switch {
				case !briefIndex[t]:
					add(stateDisproved, raw, fmt.Sprintf("routes to follow-up %s, which is not a brief in any stream README", t))
				case !followUpReferencesBack(streams, t, bf.Brief):
					add(stateUnchecked, raw, fmt.Sprintf("%s exists but never references %s — coverage is claimed, not shown", t, bf.Brief))
				default:
					add(stateCorroborated, raw, "")
				}
			}
		case routingOutOfScope:
			// Most out-of-scope judgements are genuinely unsettleable. This one
			// sub-case is not: the branch that declares a consumer excluded and
			// then edits it contradicts itself, and the diff says so outright.
			if st := classifySite(root, e.Site); st.Kind == siteRepoPath {
				if hit := exactlyChanged(changed, st.Matches); hit != "" {
					add(stateDisproved, raw, fmt.Sprintf("claims out-of-scope but this diff changes %s — the branch contradicts its own exclusion", hit))
					continue
				}
			}
			reason := outOfScopeReason(e.Detail)
			if len(reason) < minOutOfScopeReasonLength {
				add(stateUnchecked, raw, "out-of-scope with no substantive reason — a judgement stated without grounds")
				continue
			}
			add(stateUnchecked, raw, "out-of-scope is a judgement no diff can settle — the reviewer weighs the stated reason")
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Which branch made the claim
// ---------------------------------------------------------------------------
//
// Diff-scoping to "briefs this branch touches" is NOT the same as "claims this
// branch makes", and the gap is a false-positive factory. A verify-desk Evidence
// commit, a status flip, a typo fix — all edit a merged brief's FILE while
// changing none of its claims, and judging that file's year-old `fixed-here`
// entries against the editor's diff disproves them all. Measured on main at
// review time: 6 of the 18 briefs carrying `consumers:` exited 1 the moment any
// PR touched their file, and each failure named a person who changed nothing
// about the claim. A gate that fires on the innocent gets switched off, and then
// it protects nothing.
//
// So the unit of judgement is the ENTRY, not the file: an entry that is
// byte-identical to the one at the merge-base was authored on some earlier
// branch and corroborated (or not) there. Here it is UNCHECKED with that reason
// stated — never silently skipped, never a pass, and never DISPROVED by a diff
// that was never supposed to contain it.
//
// Fail direction, chosen deliberately: when the base state cannot be read at all
// (no git, shallow clone, unresolvable ref) this returns an EMPTY set, so every
// entry is treated as authored here and gets checked. The opposite default would
// let a broken git silently disable the gate — an UNRUN check reporting clean,
// which is the whole defect class this file exists to close.
//
// A package-level var so tests can substitute a base without a git fixture.
var consumerEntriesAtBase = func(root, base, rel string) map[string]bool {
	mb, err := exec.Command("git", "-C", root, "merge-base", base, "HEAD").Output()
	if err != nil {
		return nil
	}
	ref := strings.TrimSpace(string(mb)) + ":" + filepath.ToSlash(rel)
	content, err := exec.Command("git", "-C", root, "show", ref).Output()
	if err != nil {
		return nil // absent at the base: this branch adds the file, so it makes every claim in it
	}
	return consumerEntrySet(string(content))
}

// consumerEntrySet extracts the `consumers:` list from a brief file's raw
// content. It parses the frontmatter directly rather than going through
// parseBriefFile because the base blob is not a file on disk, and because a base
// version that fails brief-v1 validation must still yield its entries — the
// question here is "was this text already there?", not "is that brief valid?".
func consumerEntrySet(content string) map[string]bool {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	fmRaw, _, err := splitFrontmatter(content)
	if err != nil {
		return nil
	}
	var data map[string]any
	if err := yaml.Unmarshal([]byte(fmRaw), &data); err != nil {
		return nil
	}
	v, ok := data["consumers"]
	if !ok {
		return nil
	}
	list, lerr := stringList(v)
	if lerr != nil {
		return nil // prose form, or malformed: no entries to inherit
	}
	set := map[string]bool{}
	for _, s := range list {
		set[strings.TrimSpace(s)] = true
	}
	return set
}

// ---------------------------------------------------------------------------
// Matching a site against the diff
// ---------------------------------------------------------------------------

// touched reports whether one resolved path counts as changed by this branch.
//
// A DIRECTORY is touched when anything under it is:
// diffs name files and never directories, so `statusgen/: fixed-here` on the
// branch that rewrites the package resolved to the directory entry, matched
// nothing, and was DISPROVED — the gate calling a true claim false, which is
// worse than not checking it. brief-rules.md's own example used that form.
func touched(root string, changed map[string]bool, rel string) bool {
	return len(touchedBy(root, changed, rel)) > 0
}

// touchedBy returns the diff paths that establish `rel` as touched: `rel` itself
// when the diff names it outright, otherwise every diff path underneath it when
// `rel` is a directory. Empty means untouched.
//
// The paths are returned, not just a boolean, because a corroborated DIRECTORY
// site has to show its work. A directory is touched
// when ANY file under it is, so `docs/: fixed-here` and `docs/brief-rules.md:
// fixed-here` can rest on the same single file — and while both are honest
// claims, the wider one is the cheaper one to write. Printing a bare
// CORROBORATED for each left them indistinguishable in the log, so a reviewer
// had nothing to weigh: the gate corroborated the entry and told nobody how
// thinly. Naming the evidence keeps widening visible without rejecting it (the
// any-match rule for directories is finding 5's fix and is deliberately kept).
func touchedBy(root string, changed map[string]bool, rel string) []string {
	rel = filepath.ToSlash(strings.TrimSuffix(rel, "/"))
	if changed[rel] {
		return []string{rel}
	}
	if st, err := os.Stat(filepath.Join(root, rel)); err != nil || !st.IsDir() {
		return nil
	}
	prefix := rel + "/"
	var out []string
	for p := range changed {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// directoryEvidence describes, for a site whose resolved paths are all touched,
// how each DIRECTORY among them was established — so the reason line shows what
// the corroboration actually rests on. Returns "" when no resolved path is a
// directory, which keeps a plain file site's verdict reason empty as before.
func directoryEvidence(root string, changed map[string]bool, matches []string) string {
	var parts []string
	for _, m := range matches {
		rel := filepath.ToSlash(strings.TrimSuffix(m, "/"))
		if changed[rel] {
			continue // named outright by the diff; nothing widened
		}
		st, err := os.Stat(filepath.Join(root, rel))
		if err != nil || !st.IsDir() {
			continue
		}
		hits := touchedBy(root, changed, rel)
		if len(hits) == 0 {
			continue
		}
		shown := hits
		suffix := ""
		if len(shown) > maxDirectoryEvidencePaths {
			shown = shown[:maxDirectoryEvidencePaths]
			suffix = fmt.Sprintf(", +%d more", len(hits)-maxDirectoryEvidencePaths)
		}
		parts = append(parts, fmt.Sprintf("%s/ is a directory, corroborated by %d path(s) under it (%s%s)",
			rel, len(hits), strings.Join(shown, ", "), suffix))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ") + " — a directory site is touched when anything under it is, so a narrower site would claim less"
}

// maxDirectoryEvidencePaths caps the evidence list so a broad directory claim
// stays readable; the count before it is always exact.
const maxDirectoryEvidencePaths = 5

// untouched returns the resolved paths of a site that this branch does NOT
// change.
//
// EVERY resolved path must be touched, not merely one. `anyChanged` made a wildcard a free corroboration: `*/*: fixed-here`
// resolved to the whole tree and passed on a single match, so the cheapest way
// to green a claim was to widen it. A site that names a set claims the set —
// `docs/streams/*/README.md: fixed-here` says every stream README was updated —
// and an entry that fixed only part of its set says so by being split.
func untouched(root string, changed map[string]bool, paths []string) []string {
	var out []string
	for _, p := range paths {
		if !touched(root, changed, p) {
			out = append(out, p)
		}
	}
	return out
}

// exactlyChanged returns the first resolved path the diff changes OUTRIGHT.
// Deliberately not directory-aware: it backs the out-of-scope contradiction
// check, where a directory site incidentally containing one changed file is not
// evidence that the excluded consumer itself was touched.
func exactlyChanged(changed map[string]bool, paths []string) string {
	for _, p := range paths {
		if changed[filepath.ToSlash(p)] {
			return p
		}
	}
	return ""
}

// changedPathsSince returns the repo-relative paths this branch changes against
// base, using the three-dot (merge-base) range — the same surface a PR diff
// shows, so the check judges the branch's own work and not main's.
//
// Uncommitted working-tree changes are unioned in. In CI the checkout is clean,
// so this is exactly the PR diff; locally it means a run before `git commit`
// does not report the work-in-progress as a false claim. CI re-runs against the
// committed state either way, so this can only make the local loop honest, never
// let an uncommitted claim through the gate.
//
// pinConsumerBase resolves a (possibly moving) base ref to the concrete
// merge-base commit SHA it shares with HEAD, so runConsumers can fix the base
// once and every git read below sees the identical commit. The merge-base is
// the deterministic base: three-dot `base...HEAD` and the per-brief
// `merge-base base HEAD` both reduce to it already, so substituting the SHA
// keeps every verdict identical while removing the drift a live ref introduces.
//
// Fail direction: an error (no git, a shallow clone missing the merge-base, an
// unresolvable ref) leaves runConsumers on the ref as written, so a repo where
// the base cannot be pinned behaves exactly as before rather than silently
// changing what the gate checks. A package-level var so tests can pin a base
// without a git fixture.
var pinConsumerBase = func(root, base string) (string, error) {
	out, err := exec.Command("git", "-C", root, "merge-base", base, "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git -C %s merge-base %s HEAD: %w", root, base, err)
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "", fmt.Errorf("git -C %s merge-base %s HEAD: empty result", root, base)
	}
	return sha, nil
}

// A package-level var so tests can substitute a diff without a git fixture.
var changedPathsSince = func(root, base string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "diff", "--name-only", base+"...HEAD").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only %s...HEAD: %v: %s", base, err, strings.TrimSpace(string(out)))
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if p := strings.TrimSpace(line); p != "" {
			paths = append(paths, p)
		}
	}
	// Working-tree state: `--porcelain` prefixes each path with a two-column
	// status code; renames render as "old -> new" and both halves count as
	// touched.
	if st, sErr := exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=all").Output(); sErr == nil {
		for _, line := range strings.Split(string(st), "\n") {
			if len(line) < 4 {
				continue
			}
			for _, p := range strings.Split(strings.TrimSpace(line[3:]), " -> ") {
				if p = strings.Trim(strings.TrimSpace(p), `"`); p != "" {
					paths = append(paths, p)
				}
			}
		}
	}
	return paths, nil
}
