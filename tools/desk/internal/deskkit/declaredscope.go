package deskkit

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// InputKeyScopePrefix binds declaration membership, including planned absence.
// Older readers cannot resolve this additive key and report could-not-check.
const InputKeyScopePrefix = "scope:"

// scopeDigestFormat is exactly what ResolveDeclaredScope writes as a scope: value.
var scopeDigestFormat = regexp.MustCompile(`^[0-9a-f]{64}$`)

func scopeDeclIsGlob(decl string) bool { return strings.ContainsAny(decl, "*?[") }

// ScopeInputKey is the input key for a canonical declaration whose literal path was
// observed as kind. A literal directory ("tree") is keyed with a trailing slash; a
// literal file, an absent literal and every glob keep scope:<decl>. The slash belongs
// to the key, never to the declaration, and the digest is the same either way.
func ScopeInputKey(decl, kind string) string {
	if !scopeDeclIsGlob(decl) && kind == "tree" {
		return InputKeyScopePrefix + decl + "/"
	}
	return InputKeyScopePrefix + decl
}

// ParseScopeInputKey returns the canonical declaration a scope: key names, and whether
// the key was in the directory (trailing slash) form. It strips at most one slash and
// never normalises: an empty declaration, a double slash, a non-canonical declaration
// or a slash-form glob (never emitted) is ok=false.
func ParseScopeInputKey(key string) (decl string, dirForm bool, ok bool) {
	rest, found := strings.CutPrefix(key, InputKeyScopePrefix)
	if !found {
		return "", false, false
	}
	decl, dirForm = strings.CutSuffix(rest, "/")
	if decl == "" || strings.HasSuffix(decl, "/") {
		return "", false, false
	}
	if dirForm && scopeDeclIsGlob(decl) {
		return "", false, false
	}
	canonical, err := expandDeclaration(decl)
	if err != nil || len(canonical) != 1 || canonical[0] != decl {
		return "", false, false
	}
	return decl, dirForm, true
}

const maxScopePaths = 4096
const maxScopeWork = 32768

// BriefDeclaredFiles reads only declaration positions in Context's files field.
// Unsupported declarations are errors, never an accidentally empty input scope.
func BriefDeclaredFiles(content string) ([]string, error) {
	context, seenContext, block, found := false, false, false, false
	fence := ""
	endedByBlank := false
	var out []string
	seen := map[string]bool{}
	lines := strings.Split(content, "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		columnZero := raw == strings.TrimLeftFunc(raw, unicode.IsSpace)
		// Markdown fences allow at most three leading ASCII spaces. A tab,
		// four-space indent or inline code span must not hide a visible field.
		fenceLine := strings.TrimLeft(raw, " ")
		fenceAtColumn := len(raw)-len(fenceLine) <= 3 &&
			(strings.HasPrefix(fenceLine, "```") || strings.HasPrefix(fenceLine, "~~~"))
		if fence != "" {
			if fenceAtColumn && strings.HasPrefix(line, fence) && strings.TrimSpace(strings.TrimLeft(line, string(fence[0]))) == "" {
				fence = ""
			} else if strings.HasPrefix(line, "## ") {
				return nil, Refused("refused: section heading inside fenced region; block containment is ambiguous")
			} else if scopeFilesFieldPattern.MatchString(scopeMarkerText(line)) || scopeBroadFilesMarker(scopeMarkerText(line), scopeWrappedProse(lines, i)) {
				return nil, Refused("refused: files marker inside fenced region; block containment is ambiguous")
			}
			continue
		}
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			// Nested/list/HTML container boundaries are outside this field grammar.
			// Refuse ambiguous Context fences instead of guessing their extent.
			if context && !columnZero {
				return nil, Refused("refused: indented Context fence may belong to a container")
			}
			if context && (block || endedByBlank) {
				return nil, Refused("refused: fence inside or following files: declaration")
			}
			length := len(line) - len(strings.TrimLeft(line, string(line[0])))
			if !fenceAtColumn || (line[0] == '`' && strings.ContainsRune(line[length:], '`')) {
				if context {
					return nil, Refused("refused: ambiguous Markdown fence in Context; declared scope cannot be established")
				}
				continue
			}
			fence = line[:length]
			continue
		}
		if context && (block || endedByBlank || strings.HasPrefix(line, "files:")) &&
			(strings.Contains(line, "<!--") || (endedByBlank && strings.HasPrefix(line, "<"))) &&
			!(columnZero && scopeFieldKey(line)) {
			return nil, Refused("refused: HTML may hide a section boundary in declared files scope")
		}
		if strings.HasPrefix(line, "## ") {
			if context && !columnZero {
				return nil, Refused("refused: indented section heading in Context cannot end declared scope")
			}
			context = line == "## Context"
			if context {
				if seenContext {
					return nil, Refused("refused: duplicate Context section")
				}
				seenContext = true
			}
			block, endedByBlank = false, false
			continue
		}
		markerLine := scopeMarkerText(line)
		fieldMarker := scopeFilesFieldPattern.MatchString(markerLine)
		outsideFieldMarker := scopeOutsideFilesFieldPattern.MatchString(strings.TrimLeft(line, "*_"))
		marker := outsideFieldMarker || (context && (fieldMarker || scopeBroadFilesMarker(markerLine, scopeWrappedProse(lines, i))))
		if marker && (!context || !columnZero) {
			return nil, Refused("refused: files: field must be at column zero in Context")
		}
		if !context {
			continue
		}
		if marker && !strings.HasPrefix(line, "files:") {
			return nil, Refused("refused: unsupported Context files: field marker")
		}
		if strings.HasPrefix(line, "files:") {
			if found {
				return nil, Refused("refused: duplicate Context files: declaration")
			}
			block, found = true, true
			line = strings.TrimSpace(strings.TrimPrefix(line, "files:"))
			if line == "" {
				continue
			}
		} else if !block {
			if endedByBlank {
				if columnZero && scopeFieldKey(line) {
					endedByBlank = false
				} else if columnZero && scopeContextFieldPattern.MatchString(line) {
					return nil, Refused("refused: unknown Context field after files: declaration")
				} else if scopeLooseContinuation(raw, line) {
					return nil, Refused("refused: ambiguous files: list continuation after blank line")
				}
			}
			continue
		} else if line == "" || (columnZero && scopeFieldKey(line)) {
			block, endedByBlank = false, line == ""
			continue
		} else if columnZero && scopeContextFieldPattern.MatchString(line) {
			return nil, Refused("refused: unknown Context field inside files: declaration")
		}
		if !block {
			continue
		}
		paths, err := declarationLine(line)
		if err != nil {
			return nil, err
		}
		for _, p := range paths {
			expanded, err := expandDeclaration(p)
			if err != nil {
				return nil, err
			}
			for _, p := range expanded {
				if !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
				if len(out) > maxScopePaths {
					return nil, Refused("refused: Context files: exceeds declaration limit")
				}
			}
		}
	}
	if fence != "" {
		return nil, Refused("refused: unclosed Markdown fence; declared scope cannot be established")
	}
	if found && len(out) == 0 {
		return nil, Refused("refused: empty Context files: declaration")
	}
	return out, nil
}

var (
	scopeContextFieldPattern      = regexp.MustCompile(`^(out-of-repo files|[a-z0-9-]+)(?: \([^)]*\))?:`)
	scopeFilesFieldPattern        = regexp.MustCompile("(?i)^files?[*_`]*(?:\\s*\\([^)]*\\))?[*_`]*\\s*:")
	scopeOutsideFilesFieldPattern = regexp.MustCompile(`(?i)^files[*_]*(?:\s*\([^)]*\))?[*_]*\s*:`)
)

// Only known Context fields end the declaration. An arbitrary prose key such as
// "see:" cannot silently discard the rest of the files list.
func scopeFieldKey(line string) bool {
	match := scopeContextFieldPattern.FindStringSubmatch(line)
	if match == nil {
		return false
	}
	switch match[1] {
	case "facts", "layering", "design-fit", "single-point-of-failure", "risk-answers", "layout", "out-of-repo files":
		return true
	}
	return false
}

// Strip formatting only for detecting unsupported field markers. This never
// converts a formatted marker into an accepted files: declaration.
func scopeMarkerText(line string) string {
	for {
		before := line
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, ">") {
			line = strings.TrimPrefix(line, ">")
		} else if n := scopeListPrefix(line); n != 0 {
			line = line[n:]
		} else {
			line = strings.TrimLeft(line, "*_`")
		}
		if line == before {
			return strings.ToLower(line)
		}
	}
}

// scopeBroadFilesMarker reports a line that looks like a files field without
// being the accepted "files:" form, so the caller refuses it rather than letting
// a malformed declaration read as an empty scope. Whitespace after "files" stays
// a marker ("files tools/a.go") unless the caller has positive evidence that the
// line is wrapped prose (wrapped, from scopeWrappedProse) and its tail reads as a
// sentence (#4199). When in doubt it is a marker.
func scopeBroadFilesMarker(line string, wrapped bool) bool {
	if !strings.HasPrefix(line, "files") {
		return false
	}
	tail := strings.TrimPrefix(line, "files")
	if tail == "" || strings.ContainsAny(tail[:1], ":*_`(") {
		return true
	}
	return tail != strings.TrimLeftFunc(tail, unicode.IsSpace) && !(wrapped && scopeProseTail(tail))
}

// scopeWrappedProse is the positive evidence that lines[i] continues a
// hard-wrapped paragraph rather than starting a declaration. The line starts
// with "files " at column zero with no formatting. The line before it is
// unformatted column-zero prose that is not a field, not another files line and
// does not end a sentence or clause. The line after it is blank or the same kind
// of prose, never a list item or an indented block. Anything else is not prose.
func scopeWrappedProse(lines []string, i int) bool {
	if i == 0 || !strings.HasPrefix(strings.ToLower(lines[i]), "files ") {
		return false
	}
	prev := strings.TrimRightFunc(lines[i-1], unicode.IsSpace)
	if !scopeProseLine(prev) || strings.ContainsAny(prev[len(prev)-1:], ".:;!?") {
		return false
	}
	if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" && !scopeProseLine(lines[i+1]) {
		return false
	}
	return true
}

// scopeProseLine reports a line that starts with a letter at column zero, so it
// is no heading, list item, quote, fence, table, markup or indented block, and
// that is neither a Context field nor a files line.
func scopeProseLine(line string) bool {
	r, _ := utf8.DecodeRuneInString(line)
	return unicode.IsLetter(r) && !scopeContextFieldPattern.MatchString(line) &&
		!strings.HasPrefix(strings.ToLower(line), "files")
}

// scopeProseTail reports a tail that continues a sentence: a single space, then
// a plain word (letters, with inner hyphens or apostrophes) followed by more
// text, as in a paragraph hard-wrapped onto "files in the public tree. ...". A
// colon (ASCII or full-width), a backtick or a path-shaped word anywhere in the
// tail may carry a declaration, so such a tail is never prose.
func scopeProseTail(tail string) bool {
	if !strings.HasPrefix(tail, " ") || strings.HasPrefix(tail, "  ") ||
		strings.ContainsAny(tail, ":`\uff1a\ufe13\ufe55") {
		return false
	}
	words := strings.Fields(tail)
	if len(words) < 2 {
		return false
	}
	for i, r := range words[0] {
		if !unicode.IsLetter(r) && (i == 0 || (r != '-' && r != '\'')) {
			return false
		}
	}
	for _, w := range words {
		if scopePathWord(w) {
			return false
		}
	}
	return true
}

// scopePathWord reports a word shaped like a path: a slash or backslash, or a
// dot between two characters ("main.go"), once outer punctuation is trimmed.
func scopePathWord(w string) bool {
	w = strings.Trim(w, ".,;!?()[]\"'")
	if strings.ContainsAny(w, "/\\") {
		return true
	}
	dot := strings.LastIndexByte(w, '.')
	return dot > 0 && dot < len(w)-1
}

func scopeListPrefix(line string) int {
	if line == "" {
		return 0
	}
	i := 0
	if strings.ContainsRune("-*+", rune(line[0])) {
		i = 1
	} else {
		for i < len(line) && line[i] >= '0' && line[i] <= '9' {
			i++
		}
		if i == 0 || i == len(line) || (line[i] != '.' && line[i] != ')') {
			return 0
		}
		i++
	}
	if i == len(line) || line[i:] != strings.TrimLeftFunc(line[i:], unicode.IsSpace) {
		return i
	}
	return 0
}

func scopeLooseContinuation(raw, line string) bool {
	return line != "" && (raw != strings.TrimLeftFunc(raw, unicode.IsSpace) ||
		scopeListPrefix(line) != 0 || strings.HasPrefix(line, "`") || strings.HasPrefix(line, ">"))
}

func declarationLine(line string) ([]string, error) {
	for _, sep := range []string{" — "} {
		if i := strings.Index(line, sep); i >= 0 {
			line = line[:i]
		}
	}
	line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
	if line == "their tests," {
		return nil, nil
	}
	var result []string
	for line != "" {
		line = strings.TrimSpace(line)
		if line == "their tests," {
			break
		}
		// Existing action annotations are prose, not path candidates.
		if strings.HasPrefix(line, "**") {
			end := strings.Index(line[2:], "**")
			if end < 0 {
				return nil, Refused("refused: unclosed Context files: action annotation")
			}
			line = strings.TrimSpace(line[end+4:])
			continue
		}
		var p string
		if strings.HasPrefix(line, "`") {
			i := strings.IndexByte(line[1:], '`')
			if i < 0 {
				return nil, Refused("refused: unclosed Context files: path quote")
			}
			p, line = TrimScopeAnnotation(line[1:i+1]), line[i+2:]
		} else {
			i, braces := 0, 0
			for i < len(line) {
				c := line[i]
				if c == '{' {
					braces++
				}
				if c == '}' {
					braces--
				}
				// A glued "(…)" is an annotation, never part of the path.
				if c == ' ' || c == '\t' || ((c == ',' || c == ';' || c == '+' || c == '(') && braces == 0) {
					break
				}
				i++
			}
			p, line = line[:i], line[i:]
		}
		if p == "" {
			return nil, Refused("refused: empty Context files: path")
		}
		result = append(result, p)
		line = strings.TrimSpace(line)
		// Annotations are stripped BEFORE the ':' and terminal tests, so their
		// position never decides whether the remaining tail is understood.
		for {
			for strings.HasPrefix(line, "(") {
				i := strings.IndexByte(line, ')')
				if i < 0 {
					return nil, Refused("refused: unclosed Context files: annotation")
				}
				line = strings.TrimSpace(line[i+1:])
			}
			// "and tests" names the declaration's tests, never a path.
			rest, ok := strings.CutPrefix(line, "and tests")
			if !ok || !(rest == "" || rest == "." || strings.ContainsAny(rest[:1], " ,(")) {
				break
			}
			line = strings.TrimSpace(rest)
		}
		if strings.HasPrefix(line, ":") {
			break
		} // description after a declaration and its annotations
		if line == "" || line == "." || line == "their tests," {
			break
		}
		if strings.HasPrefix(line, ",") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "+") {
			line = strings.TrimSpace(line[1:])
			continue
		}
		if strings.HasPrefix(line, "and ") {
			line = strings.TrimSpace(line[4:])
			continue
		}
		return nil, Refused("refused: unsupported Context files: declaration tail " + line)
	}
	return result, nil
}

// TrimScopeAnnotation drops the trailing " (…)" annotations an author wrote
// INSIDE a path's code span, so `a.go (planned)` declares a.go exactly as
// `a.go` (planned) does. Only a non-empty, unnested parenthetical that follows
// whitespace and ends the span is an annotation. Anything else is returned
// unchanged for the path checks to accept or refuse, so a path with real
// whitespace is still refused.
func TrimScopeAnnotation(p string) string {
	for strings.HasSuffix(p, ")") {
		open := strings.LastIndexByte(p, '(')
		if open <= 0 {
			return p
		}
		// Decode the whole character before the paren: a lone UTF-8
		// continuation byte (0x85, 0xA0) would read as a space, so a glued
		// "à(x)" would be cut to "à".
		if r, _ := utf8.DecodeLastRuneInString(p[:open]); !unicode.IsSpace(r) {
			return p
		}
		inner := p[open+1 : len(p)-1]
		if strings.TrimSpace(inner) == "" || strings.ContainsRune(inner, ')') {
			return p
		}
		head := strings.TrimRightFunc(p[:open], unicode.IsSpace)
		if head == "" {
			return p
		}
		p = head
	}
	return p
}

func expandDeclaration(decl string) ([]string, error) {
	bad := func() ([]string, error) {
		return nil, Refused("refused: unsupported Context files: path " + fmt.Sprintf("%q", decl))
	}
	if len(decl) == 0 || len(decl) > 4096 || strings.ContainsAny(decl, "\\:<>") || strings.Contains(decl, "...") || strings.Contains(decl, "…") || strings.Contains(decl, "**") || strings.IndexFunc(decl, unicode.IsSpace) >= 0 || strings.IndexFunc(decl, unicode.IsControl) >= 0 {
		return bad()
	}
	// The sibling form is decided on the declaration as written: a brace or
	// glob can never produce or choose the ../<sibling>/ prefix.
	// Defence in depth, not dead code: the per-path SiblingDeclaration and
	// canonicalRepoPath checks below are the primary guard and refuse the same
	// shapes; this precheck also refuses before any brace is expanded.
	sibling := strings.HasPrefix(decl, "../")
	if sibling {
		if i := strings.IndexByte(decl[3:], '/'); i < 0 || !siblingName(decl[3:3+i]) {
			return bad()
		}
	}
	pending := []string{decl}
	var out []string
	for len(pending) > 0 {
		p := pending[0]
		pending = pending[1:]
		if i := strings.IndexByte(p, '{'); i >= 0 {
			j := strings.IndexByte(p[i:], '}')
			if j < 0 {
				return bad()
			}
			j += i
			choices := strings.Split(p[i+1:j], ",")
			if len(choices) < 2 || strings.ContainsAny(p[i+1:j], "{}") {
				return bad()
			}
			for _, choice := range choices {
				if choice == "" {
					return bad()
				}
				pending = append(pending, p[:i]+choice+p[j+1:])
			}
			if len(pending)+len(out) > 256 {
				return bad()
			}
			continue
		}
		p = strings.TrimSuffix(p, "/")
		check := p
		if sibling {
			_, rel, ok := SiblingDeclaration(p)
			// Defence in depth, not dead code: the precheck above and the
			// canonicalRepoPath(rel) check below (an empty rel fails it) are
			// the primary guards; keep this explicit refusal beside them.
			if !ok {
				return bad()
			}
			check = rel
		}
		if !canonicalRepoPath(check) || strings.ContainsAny(p, "{},`\"'") {
			return bad()
		}
		if _, err := path.Match(p, ""); err != nil {
			return bad()
		}
		out = append(out, p)
	}
	return out, nil
}

// Tree members and file: keys are literal paths, not glob expressions. A file
// selected by *.go may itself contain brackets; never interpret it a second time.
func canonicalRepoPath(p string) bool {
	if p == "" || len(p) > 4096 || p == "." || strings.HasPrefix(p, "/") || path.Clean(p) != p || strings.ContainsAny(p, "\\:") || strings.IndexFunc(p, unicode.IsControl) >= 0 || strings.IndexFunc(p, unicode.IsSpace) >= 0 {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." || part == ".git" {
			return false
		}
	}
	return true
}

// A cross-repository brief names its deliverables beside the tracking checkout,
// as ../<sibling>/<path>. That is the ONLY accepted form containing "..": exactly
// one leading ../, a literal repository-name segment, then a canonical path. It
// is never read from any tree. A receipt covers it only through the immutable
// repo-source:<owner>/<sibling> commit its run bound (CheckSiblingCoverage).
var siblingNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func siblingName(name string) bool {
	return siblingNamePattern.MatchString(name) && name != "." && name != ".." && name != ".git"
}

// SiblingDeclaration splits a ../<sibling>/<path> declaration. Every other
// shape, including any further "..", reports ok=false.
func SiblingDeclaration(decl string) (sibling, rel string, ok bool) {
	rest, found := strings.CutPrefix(decl, "../")
	if !found {
		return "", "", false
	}
	sibling, rel, found = strings.Cut(rest, "/")
	// path.Clean(decl) != decl is defence in depth, not dead code: siblingName
	// and canonicalRepoPath(rel) are the primary guard against every
	// noncanonical shape it would also catch.
	if !found || !siblingName(sibling) || !canonicalRepoPath(rel) || path.Clean(decl) != decl {
		return "", "", false
	}
	return sibling, rel, true
}

// SplitSiblingDeclarations separates the declarations this tree resolves from
// the sibling-repository ones only a cross-repository source binding covers.
func SplitSiblingDeclarations(declarations []string) (local, siblings []string) {
	for _, decl := range declarations {
		if _, _, ok := SiblingDeclaration(decl); ok {
			siblings = append(siblings, decl)
		} else {
			local = append(local, decl)
		}
	}
	return local, siblings
}

// CheckSiblingCoverage requires every sibling declaration to name exactly one
// repo-source:<owner>/<sibling> input. That key pins the whole deliverable
// commit, so any change to it wakes the receipt; a sibling the run did not bind
// is refused rather than resolved anywhere else.
func CheckSiblingCoverage(siblings []string, inputs map[string]string) error {
	for _, decl := range siblings {
		// A malformed declaration has no name, so nothing binds it.
		name, _, _ := SiblingDeclaration(decl)
		bound := 0
		for k, v := range inputs {
			repo, isSource := strings.CutPrefix(k, InputKeyRepoSourcePrefix)
			if _, repoName := splitRepo(repo); isSource && name != "" && strings.TrimSpace(v) != "" && strings.EqualFold(repoName, name) {
				bound++
			}
		}
		if bound != 1 {
			return Refused("refused: Context files: path " + fmt.Sprintf("%q", decl) + " names a sibling repository this run's cross-repository source does not bind")
		}
	}
	return nil
}

// DeclaredScope has one membership digest per canonical declaration, and every
// existing file it selects. No bytes are taken from a different tree.
type DeclaredScope struct {
	Files  []string
	Shapes map[string]string
}

func ResolveDeclaredScope(declarations []string, tree OutcomeTree) (DeclaredScope, error) {
	result := DeclaredScope{Shapes: map[string]string{}}
	files := map[string]bool{}
	work := 0
	consume := func(n int) error {
		if n > maxScopeWork-work {
			return Refused("refused: declared scope exceeds resolution work limit")
		}
		work += n
		return nil
	}
	for _, decl := range declarations {
		if err := consume(1); err != nil {
			return result, err
		}
		canonical, err := expandDeclaration(decl)
		if err != nil {
			return result, err
		}
		if len(canonical) != 1 || canonical[0] != decl {
			return result, Refused("refused: noncanonical input scope declaration")
		}
		if _, _, ok := SiblingDeclaration(decl); ok {
			return result, Refused("refused: Context files: path " + fmt.Sprintf("%q", decl) + " names a sibling repository; it never resolves against this tree")
		}
		paths := []string{decl}
		literalKind := ""
		if scopeDeclIsGlob(decl) {
			// List only below the literal prefix; match files AND directories,
			// then expand matched directories recursively, as literal directories do.
			i := strings.IndexAny(decl, "*?[")
			prefix := path.Dir(decl[:i] + "x")
			all, err := tree.ListFiles(prefix)
			if err != nil {
				return result, Unverifiable("cannot list declared glob "+decl, err)
			}
			if len(all) > maxScopePaths {
				return result, Refused("refused: declared glob exceeds file limit")
			}
			if err := consume(len(all)); err != nil {
				return result, err
			}
			candidates := map[string]bool{}
			for _, f := range all {
				for p := f; p != "." && p != "/"; p = path.Dir(p) {
					if err := consume(1); err != nil {
						return result, err
					}
					candidates[p] = true
				}
			}
			paths = nil
			for p := range candidates {
				if err := consume(1); err != nil {
					return result, err
				}
				if matched, _ := path.Match(decl, p); matched {
					paths = append(paths, p)
				}
			}
			sort.Strings(paths)
		}
		var shape []string
		for _, p := range paths {
			if err := consume(1); err != nil {
				return result, err
			}
			kind, err := tree.PathKind(p)
			if err != nil {
				return result, Unverifiable("cannot resolve declared input "+p, err)
			}
			shape = append(shape, kind+":"+p)
			literalKind = kind
			switch kind {
			case "":
			case "blob":
				files[p] = true
			case "tree":
				members, err := tree.ListFiles(p)
				if err != nil {
					return result, Unverifiable("cannot list declared directory "+p, err)
				}
				if len(members) > maxScopePaths {
					return result, Refused("refused: declared directory exceeds file limit")
				}
				if err := consume(len(members)); err != nil {
					return result, err
				}
				sort.Strings(members)
				for _, f := range members {
					if !canonicalRepoPath(f) || !strings.HasPrefix(f, p+"/") {
						return result, Unverifiable("invalid declared directory member", nil)
					}
					files[f] = true
					shape = append(shape, "blob:"+f)
				}
			default:
				return result, Refused("refused: unsupported declared input kind " + kind + " for " + p)
			}
			if len(files) > maxScopePaths {
				return result, Refused("refused: declared input scope exceeds file limit")
			}
		}
		sort.Strings(shape)
		result.Shapes[ScopeInputKey(decl, literalKind)] = Sha256Hex([]byte(strings.Join(shape, "\x00")))
	}
	for f := range files {
		result.Files = append(result.Files, f)
	}
	sort.Strings(result.Files)
	return result, nil
}

// rootScopeTree never follows ancestor symlinks. A final symlink remains a blob
// (its link text), matching Git's treatment; directory enumeration never follows it.
type rootScopeTree struct{ root string }

// CheckReceiptScope prevents an immutable historical partial manifest from
// establishing unchangedness. This reads today's declarations only to identify
// missing coverage; it never migrates the receipt's original observed hashes.
func (rr *RootRevisionReader) CheckReceiptScope(receipt WakeReceipt) error {
	stream, num, err := SplitBriefKey(receipt.Brief)
	if err != nil {
		return fmt.Errorf("cannot identify the brief for scope completeness")
	}
	prefix := "file:docs/streams/" + stream + "/brief-" + num + "-"
	brief := ""
	for k := range receipt.Inputs {
		if strings.HasPrefix(k, prefix) && strings.HasSuffix(k, ".md") {
			if brief != "" {
				return fmt.Errorf("ambiguous brief input scope")
			}
			brief = strings.TrimPrefix(k, "file:")
		}
	}
	if brief == "" {
		return fmt.Errorf("missing brief input for scope completeness")
	}
	full, err := (rootScopeTree{root: rr.Root}).safePath(brief)
	if err != nil {
		return err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("brief scope is not a plain file")
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	declarations, err := BriefDeclaredFiles(string(b))
	if err != nil {
		return err
	}
	// A slash-form key's value sits where the pattern sweep's generic rule no longer
	// looks, so refuse to read any scope: value that is not the producer's digest.
	for k, v := range receipt.Inputs {
		if strings.HasPrefix(k, InputKeyScopePrefix) && !scopeDigestFormat.MatchString(v) {
			return fmt.Errorf("%s value is not a scope digest", k)
		}
	}
	// Only local declarations carry scope: keys. A ../<sibling>/ declaration is
	// covered by its repo-source: commit below; ranging over every declaration
	// here would report each sibling as missing scope and disable sibling briefs.
	local, siblings := SplitSiblingDeclarations(declarations)
	for _, decl := range local {
		plain := receipt.Inputs[InputKeyScopePrefix+decl] != ""
		dir := !scopeDeclIsGlob(decl) && receipt.Inputs[InputKeyScopePrefix+decl+"/"] != ""
		if plain && dir {
			return fmt.Errorf("ambiguous scope key for %s: both scope:%s and scope:%s/ are present", decl, decl, decl)
		}
		if !plain && !dir {
			return fmt.Errorf("missing declared scope %s; fresh admission and observation required", decl)
		}
	}
	return CheckSiblingCoverage(siblings, receipt.Inputs)
}

func (r rootScopeTree) safePath(rel string) (string, error) {
	if rel == "." {
		return r.root, nil
	}
	if !canonicalRepoPath(rel) {
		return "", fmt.Errorf("invalid scope path")
	}
	parts := strings.Split(rel, "/")
	full := r.root
	for i, part := range parts {
		full = filepath.Join(full, part)
		if i == len(parts)-1 {
			break
		}
		info, err := os.Lstat(full)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("input ancestor is a symlink")
		}
	}
	return full, nil
}

func (r rootScopeTree) PathKind(rel string) (string, error) {
	full, err := r.safePath(rel)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(full)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "tree", nil
	}
	if info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "blob", nil
	}
	return "", fmt.Errorf("unsupported input kind")
}

func (r rootScopeTree) ReadFile(rel string) ([]byte, error) {
	return nil, fmt.Errorf("scope reader does not read file content")
}

func (r rootScopeTree) ListFiles(rel string) ([]string, error) {
	full, err := r.safePath(rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(full)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scope prefix is not a directory")
	}
	var out []string
	err = filepath.WalkDir(full, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		name, err := filepath.Rel(r.root, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(name))
		if len(out) > maxScopePaths {
			return fmt.Errorf("scope enumeration exceeds file limit")
		}
		return nil
	})
	return out, err
}
