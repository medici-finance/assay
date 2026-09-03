package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Spec/scoping-doc lifecycle header reader — spec/lifecycle-v1.md §8.
//
// PLACEMENT DECISION (why this is NOT in brieffile.go). brieffile.go parses a
// brief's YAML FRONTMATTER: a `---`-fenced block of typed brief-v1 fields run
// through the brief validator. A spec/scoping document carries no such
// frontmatter — its lifecycle state lives in a BOLD-LABEL line in the markdown
// header block, `**Status:** approved` / `**Routes-to:** docs/streams/x/`
// (§8.1/§8.3). Those are two different surfaces (typed YAML vs. a prose
// bold-label line) over two different artifact kinds (a brief vs. the spec a
// brief is authored FROM), so this reader is deliberately its own file rather
// than folded into parseBriefFile: a spec doc is not a brief, has no brief-v1
// schema, and must never be handed to the brief validator. It shares statusgen's
// repo-relative-path discipline (Rel is the citation key §8.5 dereferences), but
// reads its own, simpler grammar.

// Lifecycle states (§8.1). The machine value is EXACTLY one of these tokens;
// anything else leaves the document unclassified (legacy) and MUST NOT be
// rounded up to a real state (§8.6). Case-sensitive: §8.1 says "exactly one of".
const (
	lifecycleDraft    = "draft"
	lifecycleApproved = "approved"
	lifecycleRouted   = "routed"
)

// specLifecycleHeader is the parsed §8 header block of one spec/scoping document.
// A document is a lifecycle document iff HasStatus — carrying a `**Status:**`
// line is the §8.1 opt-in. A document without one is not tracked by this
// lifecycle at all and is silently ignored (never defaulted up to a state).
type specLifecycleHeader struct {
	Path      string // absolute path on disk
	Rel       string // repo-relative, slash-separated — the §8.5 citation key
	HasStatus bool   // a `**Status:**` header line was present (the opt-in signal)
	State     string // one of the three lifecycle states, or "" when unclassified
	StateRaw  string // the state token as written (for the unclassified NOTICE)
	Routes    bool   // a `**Routes-to:**` line with a non-empty destination was present
	RoutesTo  string // the destination as written (trimmed), when Routes
}

// Classified reports whether the document carries a recognized lifecycle state.
// An unclassified (HasStatus but bad first token) document is NOT classified and
// is never owed-eligible nor Routes-to-required.
func (h *specLifecycleHeader) Classified() bool { return h.State != "" }

// parseSpecLifecycleHeader reads a spec/scoping document's §8 header block.
//
// The header block is everything before the first H2 section heading (`## `).
// Scoping to the header block is load-bearing: spec/lifecycle-v1.md itself
// QUOTES the grammar (`a line of the form `+"`**Status:** <state>`"+`) deep in
// its own body, and a whole-file scan would read that documentation as the
// document's own state. Real header lines sit above the first section; the prose
// mentions sit below it, inside backticks, and never begin a header-block line.
func parseSpecLifecycleHeader(root, path string) (*specLifecycleHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rel := path
	if r, e := filepath.Rel(root, path); e == nil {
		rel = filepath.ToSlash(r)
	}
	h := &specLifecycleHeader{Path: path, Rel: rel}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		// The header block ends at the first H2 section heading.
		if strings.HasPrefix(line, "## ") {
			break
		}
		if rest, ok := cutHeaderLabel(line, "**Status:**"); ok && !h.HasStatus {
			h.HasStatus = true
			h.State, h.StateRaw = lifecycleStateOf(rest)
		}
		if rest, ok := cutHeaderLabel(line, "**Routes-to:**"); ok && !h.Routes {
			if dest := strings.TrimSpace(rest); dest != "" {
				h.Routes = true
				h.RoutesTo = dest
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return h, nil
}

// cutHeaderLabel returns the remainder of a header-block line that begins with
// the given bold label (e.g. `**Status:**`), and whether it matched. Matching
// requires the label at the START of the (already whitespace-trimmed) line, so a
// prose mention or a backtick-quoted example never registers as a header.
func cutHeaderLabel(line, label string) (string, bool) {
	if strings.HasPrefix(line, label) {
		return strings.TrimSpace(line[len(label):]), true
	}
	return "", false
}

// lifecycleStateOf resolves a `**Status:**` line's remainder to its machine
// state (§8.1). The state is the FIRST whitespace-delimited token; the optional
// ` — <free prose>` tail is discarded for the machine value. A first token
// outside {draft, approved, routed} yields state "" (unclassified) with the raw
// token preserved for the diagnostic — it is NEVER rounded up to a real state.
func lifecycleStateOf(rest string) (state, raw string) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "", ""
	}
	raw = strings.Fields(rest)[0]
	switch raw {
	case lifecycleDraft, lifecycleApproved, lifecycleRouted:
		return raw, raw
	}
	return "", raw
}
