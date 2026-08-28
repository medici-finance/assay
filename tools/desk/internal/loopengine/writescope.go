package loopengine

// writescope.go — advisory write-scopes and the dispatch-time
// overlap warning.
//
// A brief's write-scope set is the path prefixes it expects to write, derived by default
// from its Context `files:` list and optionally REPLACED by a `write-scopes:` frontmatter
// override. The set is an ADVISORY COORDINATION HINT, verbatim from the donor mechanism:
// "coordination hints, not locks". Nothing here blocks a dispatch, gates a claim, serializes
// execution, or authorizes/denies a filesystem write — the sole product is a WARNING LINE a
// human reads. The moment any of this code delayed or exit-code-gated a dispatch it would be
// a lock, which is a different (and rejected) design.
//
// Three-state honesty (spec §8): a brief whose scope set cannot be derived is reported as
// `could-not-derive`, NEVER rounded down to "no overlaps".

import (
	"regexp"
	"sort"
	"strings"
)

// WriteScope is one normalized advisory write-scope: a repo-relative path prefix, optionally
// addressed to a sibling repository. Repo == "" means the brief's own repo; a non-empty Repo
// is the sibling named by a `../<repo>/…` entry. Prefix carries no leading slash and no glob
// metacharacters — it is normalized to the directory/file prefix the entry addresses.
type WriteScope struct {
	Repo   string
	Prefix string
}

// String renders a scope for a warning line: a sibling scope is spelled `../<repo>/<prefix>`
// so the warning names the same shape the brief's Context entry used; a same-repo scope is
// the bare prefix (matching the donor's `internal/loopengine/` example).
func (s WriteScope) String() string {
	if s.Repo != "" {
		return "../" + s.Repo + "/" + s.Prefix
	}
	return s.Prefix
}

// WriteScopeSet is a brief's advisory write-scope set. Derivable == false is the honest
// could-not-derive state (§8): the set is UNKNOWN, not empty, and MUST NOT be read as "no
// overlaps". The zero value is therefore an underivable set, which is the safe default for
// any Item whose scopes were never derived.
type WriteScopeSet struct {
	Scopes    []WriteScope
	Derivable bool
}

// fmBlockRe / filesLineRe / scopesKeyRe are the small self-contained frontmatter/Context
// extractors. loopengine deliberately carries no YAML dependency (the engine core stays
// dependency-thin), so — exactly as the fanoutloop adapter's frontmatter.go does — a brief is
// read with line/regex extraction rather than a parser.
var (
	globMetaRe = regexp.MustCompile(`[*?\[\]{}]`)
	backtickRe = regexp.MustCompile("`([^`]+)`")
)

// DeriveWriteScopes computes a brief's advisory write-scope set from its full file content
// derivation rule: an explicit `write-scopes:` frontmatter array REPLACES the derived set;
// otherwise the set is derived from the Context `files:` list. A brief with neither a usable
// override nor a parseable `files:` entry is could-not-derive (Derivable == false) — never an
// empty-but-clear set.
func DeriveWriteScopes(content string) WriteScopeSet {
	fm := frontmatterOf(content)

	// 1. Explicit override wins.
	if raws, ok := frontmatterList(fm, "write-scopes"); ok {
		set := WriteScopeSet{}
		for _, r := range raws {
			if sc, ok := normalizeScopeEntry(r); ok {
				set.Scopes = append(set.Scopes, sc)
			}
		}
		// An override was authored: the set IS derivable (from the override), even if every
		// entry normalized away — that is an explicit, honest empty scope, not could-not-derive.
		set.Derivable = true
		set.Scopes = dedupeScopes(set.Scopes)
		return set
	}

	// 2. Derive from the Context `files:` list.
	raws, found := contextFilesEntries(content)
	if !found || len(raws) == 0 {
		return WriteScopeSet{Derivable: false} // could-not-derive — honest, never "clear"
	}
	set := WriteScopeSet{}
	for _, r := range raws {
		if sc, ok := normalizeScopeEntry(r); ok {
			set.Scopes = append(set.Scopes, sc)
		}
	}
	if len(set.Scopes) == 0 {
		// A `files:` block existed but no entry yielded a usable prefix — still unknown, not clear.
		return WriteScopeSet{Derivable: false}
	}
	set.Derivable = true
	set.Scopes = dedupeScopes(set.Scopes)
	return set
}

// normalizeScopeEntry normalizes ONE raw path entry to a WriteScope prefix. A `../<repo>/…`
// entry becomes that sibling repo plus prefix; a glob is trimmed back to its directory prefix
// (the last `/` before the first glob metacharacter). ok is false when the entry carries no
// usable prefix (e.g. it reduces to the repo root, too coarse to be a useful hint, or it is a
// bare sibling repo with no path).
func normalizeScopeEntry(raw string) (WriteScope, bool) {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, "`'\"")
	s = strings.TrimSpace(s)
	if s == "" {
		return WriteScope{}, false
	}
	// Drop a leading "./".
	s = strings.TrimPrefix(s, "./")

	repo := ""
	if strings.HasPrefix(s, "../") {
		rest := strings.TrimPrefix(s, "../")
		// rest = "<repo>/<path...>" or just "<repo>".
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			repo = rest[:i]
			s = rest[i+1:]
		} else {
			repo = rest
			s = ""
		}
	}

	// Trim a glob: keep the substring before the first glob metacharacter, then trim back to
	// the last path separator so a partial filename component (e.g. `brief-*`) is dropped.
	if loc := globMetaRe.FindStringIndex(s); loc != nil {
		s = s[:loc[0]]
		if i := strings.LastIndexByte(s, '/'); i >= 0 {
			s = s[:i+1]
		} else {
			s = "" // glob at the top level → only the repo root remains, too coarse
		}
	}

	s = strings.TrimPrefix(s, "/")
	prefix := strings.TrimRight(s, "/")
	if prefix == "" {
		// A repo-root prefix (empty) is too coarse to be a useful coordination hint; a bare
		// sibling repo (`../repo`) is likewise nothing to compare on. Drop it.
		return WriteScope{}, false
	}
	// Re-attach a trailing slash when the original addressed a directory, so the rendered hint
	// reads as a path prefix rather than a file. We cannot always know, so keep the slash form
	// only when the trimmed source ended in one; comparison is component-wise regardless.
	if strings.HasSuffix(s, "/") {
		prefix += "/"
	}
	return WriteScope{Repo: repo, Prefix: prefix}, true
}

// components splits a scope prefix into path components (no empty parts).
func (s WriteScope) components() []string {
	parts := strings.Split(strings.Trim(s.Prefix, "/"), "/")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// sharedPrefix reports the shared path prefix when two scopes in the SAME repo overlap — one
// is a component-wise path prefix of the other. The shared prefix is the SHORTER path (it
// contains the longer). ok is false when the repos differ or neither contains the other.
func sharedPrefix(a, b WriteScope) (WriteScope, bool) {
	if a.Repo != b.Repo {
		return WriteScope{}, false
	}
	ca, cb := a.components(), b.components()
	n := len(ca)
	if len(cb) < n {
		n = len(cb)
	}
	for i := 0; i < n; i++ {
		if ca[i] != cb[i] {
			return WriteScope{}, false // diverge before either is exhausted — no overlap
		}
	}
	// One is a prefix of the other; the shorter is the shared ancestor.
	if len(ca) <= len(cb) {
		return a, true
	}
	return b, true
}

// Overlap returns the shared-prefix strings between this set and another, for every scope
// pair that overlaps. Deterministic (sorted, de-duplicated). It is meaningful ONLY when both
// sets are Derivable; the caller is responsible for reporting could-not-derive honestly
// rather than treating an empty Overlap as "clear".
func (s WriteScopeSet) Overlap(other WriteScopeSet) []string {
	if !s.Derivable || !other.Derivable {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, a := range s.Scopes {
		for _, b := range other.Scopes {
			if sp, ok := sharedPrefix(a, b); ok {
				k := sp.String()
				if !seen[k] {
					seen[k] = true
					out = append(out, k)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// WriteOverlapWarnings builds the advisory dispatch-time warning lines: one
// `WRITE-OVERLAP: <candidate> ~ <inflight> on <prefix>` line per shared prefix between a
// candidate item's scopes and an in-flight item's scopes, plus one
// `<candidate>: scopes: could-not-derive` line for each candidate whose scopes could not be
// derived (three-state honesty — NEVER silently treated as clear). Disjoint scopes produce
// nothing. The lines are ADVISORY: the caller prints them and PROCEEDS.
//
// candidates are the items about to be dispatched; inflight are the items already claimed
// (the same universe the claim system tracks) for the same root. Self-comparison
// (candidate.ID == inflight.ID) is skipped.
func WriteOverlapWarnings(candidates, inflight []Item) []string {
	var out []string
	for _, c := range candidates {
		if !c.WriteScopes.Derivable {
			out = append(out, c.ID+": scopes: could-not-derive")
			continue
		}
		for _, f := range inflight {
			if f.ID == c.ID || !f.WriteScopes.Derivable {
				continue
			}
			for _, shared := range c.WriteScopes.Overlap(f.WriteScopes) {
				out = append(out, "WRITE-OVERLAP: "+c.ID+" ~ "+f.ID+" on "+shared)
			}
		}
	}
	return out
}

// dedupeScopes removes duplicate scopes (same repo + prefix), preserving first-seen order.
func dedupeScopes(in []WriteScope) []WriteScope {
	seen := map[string]bool{}
	var out []WriteScope
	for _, s := range in {
		k := s.Repo + "\x00" + s.Prefix
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
	}
	return out
}

// --- self-contained frontmatter / Context extraction ------------------------------------------

// frontmatterOf returns the YAML frontmatter block (between the first two `---` fences), or ""
// when the content carries none. Mirrors the fanoutloop adapter's frontmatterBlock so the two
// readers agree on what "the frontmatter" is.
func frontmatterOf(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n")
		}
	}
	return ""
}

// frontmatterList reads a top-level frontmatter key as a string list, accepting both the block
// form (`key:` then `  - item` lines) and the inline flow form (`key: [a, b]`). ok is false
// when the key is absent, so an absent override is distinguishable from an authored-but-empty
// one.
func frontmatterList(fm, key string) (items []string, ok bool) {
	lines := strings.Split(fm, "\n")
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if !strings.HasPrefix(t, key+":") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(t, key+":"))
		// Inline flow form: key: [a, b, c]
		if strings.HasPrefix(rest, "[") {
			inner := strings.TrimSuffix(strings.TrimPrefix(rest, "["), "]")
			for _, p := range strings.Split(inner, ",") {
				if v := strings.TrimSpace(strings.Trim(strings.TrimSpace(p), `"'`)); v != "" {
					items = append(items, v)
				}
			}
			return items, true
		}
		// A scalar value on the same line (e.g. an empty or `[]`) with no list — still an
		// authored key.
		if rest != "" && rest != "[]" {
			items = append(items, strings.Trim(rest, `"'`))
			return items, true
		}
		// Block form: subsequent `  - item` lines until the indentation returns to a new
		// top-level key.
		for j := i + 1; j < len(lines); j++ {
			raw := lines[j]
			tr := strings.TrimSpace(raw)
			if tr == "" {
				continue
			}
			if strings.HasPrefix(tr, "- ") || tr == "-" {
				v := strings.TrimSpace(strings.TrimPrefix(tr, "-"))
				if v = strings.Trim(v, `"'`); v != "" {
					items = append(items, v)
				}
				continue
			}
			// A non-list, non-blank line that is not indented deeper than the key ends the block.
			if !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") {
				break
			}
			// An indented non-list line (unexpected) also ends the list.
			break
		}
		return items, true
	}
	return nil, false
}

// contextFilesEntries extracts the raw path entries of the `## Context` `files:` list. It
// accepts the two authored shapes:
//
//	files:
//	- `path/one/` — description
//	- `../repo/path` — description
//
// and the inline single-line form `files: cmd/fanoutloop/`. It returns the raw entries
// (path plus any trailing description, which normalizeScopeEntry strips) and whether a
// `files:` line was found at all — an absent `files:` line is could-not-derive, distinct from
// a present-but-empty one.
func contextFilesEntries(content string) (entries []string, found bool) {
	lines := strings.Split(content, "\n")
	// Skip the frontmatter block so a `files:` inside it (there is none in brief-v1, but be
	// robust) is not mistaken for the Context list.
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}
	for i := start; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(t, "files:") {
			continue
		}
		found = true
		rest := strings.TrimSpace(strings.TrimPrefix(t, "files:"))
		if rest != "" {
			// Inline form: a single path (or comma/space-separated paths) on the line itself.
			for _, e := range splitInlineFiles(rest) {
				if e != "" {
					entries = append(entries, e)
				}
			}
			return entries, true
		}
		// Block list: `- entry` lines until a blank line or a new section/`key:` line.
		for j := i + 1; j < len(lines); j++ {
			lt := strings.TrimSpace(lines[j])
			if lt == "" {
				break // a blank line closes the list
			}
			if strings.HasPrefix(lt, "- ") || lt == "-" {
				e := extractPathToken(strings.TrimSpace(strings.TrimPrefix(lt, "-")))
				if e != "" {
					entries = append(entries, e)
				}
				continue
			}
			// A continuation line of the previous entry (indented, no leading dash) is
			// description prose — ignore it. Anything at the left margin ends the list.
			if !strings.HasPrefix(lines[j], " ") && !strings.HasPrefix(lines[j], "\t") {
				break
			}
		}
		return entries, true
	}
	return nil, false
}

// extractPathToken pulls the path out of one list entry. The authored convention wraps the
// path in backticks (`path` — description); when present that is authoritative. Otherwise the
// first whitespace-delimited token is taken.
func extractPathToken(entry string) string {
	entry = strings.TrimSpace(entry)
	if m := backtickRe.FindStringSubmatch(entry); m != nil {
		return strings.TrimSpace(m[1])
	}
	// No backticks: first token up to whitespace.
	if i := strings.IndexAny(entry, " \t"); i >= 0 {
		return strings.TrimSpace(entry[:i])
	}
	return entry
}

// splitInlineFiles splits an inline `files:` value into path tokens, honoring backticks and
// falling back to comma/space separation.
func splitInlineFiles(rest string) []string {
	if strings.Contains(rest, "`") {
		var out []string
		for _, m := range backtickRe.FindAllStringSubmatch(rest, -1) {
			out = append(out, strings.TrimSpace(m[1]))
		}
		if len(out) > 0 {
			return out
		}
	}
	// Take the first whitespace/comma-delimited token(s): a bare inline value like
	// `cmd/fanoutloop/` is one path; `a, b` is two.
	fields := strings.FieldsFunc(rest, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	return fields
}
