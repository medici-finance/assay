package briefdoc

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Declared input compilation (tool-refactoring/03).
//
// This file is the ONE home of the strict, pure reading of a brief's
// `## Context` `files:` declaration: which lines are declarations, how a path,
// a brace group, a glob or a ../<sibling>/ path is read, and what is refused
// instead of being read as an empty scope. tools/desk's deskkit projects the
// result into receipts and scope digests; it keeps no copy of the grammar. A
// refusal is returned as *Refusal so the caller can map it onto its own exit
// code without this package knowing about it.

// Refusal is a declaration the strict grammar will not read. Its Msg is the
// complete operator-facing sentence.
type Refusal struct{ Msg string }

func (r *Refusal) Error() string { return r.Msg }

func refuse(msg string) error { return &Refusal{Msg: msg} }

func isRefusal(err error) bool {
	_, ok := err.(*Refusal)
	return ok
}

// MaxScopePaths bounds the paths one declaration may expand to.
const MaxScopePaths = 4096

// DeclaredFiles reads only declaration positions in Context's files field.
// Unsupported declarations are errors, never an accidentally empty input scope.
func DeclaredFiles(content string) ([]string, error) {
	context, seenContext, block, found := false, false, false, false
	fence := ""
	endedByBlank := false
	// The current "- " bullet's indent, and whether its declaration reached a
	// description a deeper-indented line may wrap; -1 and false outside one.
	bulletIndent, described := -1, false
	// listClosed: the last declaration line ended the list (a "- " bullet, or
	// an inline line ending in "."). annotated: the block's one trailing
	// sentence was read, so any further line in the block is refused.
	listClosed, annotated := false, false
	var out []string
	seen := map[string]bool{}
	for _, raw := range strings.Split(content, "\n") {
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
				return nil, refuse("refused: section heading inside fenced region; block containment is ambiguous")
			} else if scopeFilesFieldPattern.MatchString(scopeMarkerText(line)) || scopeBroadFilesMarker(scopeMarkerText(line)) {
				return nil, refuse("refused: files marker inside fenced region; block containment is ambiguous")
			}
			continue
		}
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			// Nested/list/HTML container boundaries are outside this field grammar.
			// Refuse ambiguous Context fences instead of guessing their extent.
			if context && !columnZero {
				return nil, refuse("refused: indented Context fence may belong to a container")
			}
			if context && (block || endedByBlank) {
				return nil, refuse("refused: fence inside or following files: declaration")
			}
			length := len(line) - len(strings.TrimLeft(line, string(line[0])))
			if !fenceAtColumn || (line[0] == '`' && strings.ContainsRune(line[length:], '`')) {
				if context {
					return nil, refuse("refused: ambiguous Markdown fence in Context; declared scope cannot be established")
				}
				continue
			}
			fence = line[:length]
			continue
		}
		if context && (block || endedByBlank || strings.HasPrefix(line, "files:")) &&
			(strings.Contains(line, "<!--") || (endedByBlank && strings.HasPrefix(line, "<"))) &&
			!(columnZero && scopeFieldKey(line)) {
			return nil, refuse("refused: HTML may hide a section boundary in declared files scope")
		}
		if strings.HasPrefix(line, "## ") {
			if context && !columnZero {
				return nil, refuse("refused: indented section heading in Context cannot end declared scope")
			}
			context = line == "## Context"
			if context {
				if seenContext {
					return nil, refuse("refused: duplicate Context section")
				}
				seenContext = true
			}
			block, endedByBlank = false, false
			continue
		}
		markerLine := scopeMarkerText(line)
		fieldMarker := scopeFilesFieldPattern.MatchString(markerLine)
		outsideFieldMarker := scopeOutsideFilesFieldPattern.MatchString(strings.TrimLeft(line, "*_"))
		marker := outsideFieldMarker || (context && (fieldMarker || scopeBroadFilesMarker(markerLine)))
		if marker && (!context || !columnZero) {
			return nil, refuse("refused: files: field must be at column zero in Context")
		}
		if !context {
			continue
		}
		if marker && !strings.HasPrefix(line, "files:") {
			return nil, refuse("refused: unsupported Context files: field marker")
		}
		if strings.HasPrefix(line, "files:") {
			if found {
				return nil, refuse("refused: duplicate Context files: declaration")
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
					return nil, refuse("refused: unknown Context field after files: declaration")
				} else if scopeLooseContinuation(raw, line) {
					return nil, refuse("refused: ambiguous files: list continuation after blank line")
				}
			}
			continue
		} else if line == "" || (columnZero && scopeFieldKey(line)) {
			block, endedByBlank = false, line == ""
			continue
		} else if columnZero && scopeContextFieldPattern.MatchString(line) {
			return nil, refuse("refused: unknown Context field inside files: declaration")
		}
		if !block {
			continue
		}
		if annotated {
			return nil, refuse("refused: prose line inside Context files: list; only one sentence may follow the list, as its last line")
		}
		indent := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace))
		// A "- " bullet whose declaration reached its description (" — " or ':')
		// may wrap that description onto deeper-indented lines (#3697). Such a
		// line that the declaration grammar cannot read is the description's
		// prose, exactly as it is when joined onto the bullet. One that does read
		// as declarations is still declarations, so a wrap never drops a path the
		// line would declare on its own; a list item or quote is never a wrap.
		wrap := described && indent > bulletIndent && scopeListPrefix(line) == 0 && !strings.HasPrefix(line, ">")
		paths, desc, err := declarationLine(line)
		if err != nil {
			if wrap && isRefusal(err) {
				continue
			}
			// The one trailing sentence (#5097): it declares nothing, and it
			// must end the block, so it cannot hide a path or end the list early.
			if isRefusal(err) && columnZero && listClosed && scopeAnnotation(line, out) {
				annotated = true
				continue
			}
			return nil, err
		}
		if strings.HasPrefix(strings.TrimSpace(raw), "- ") {
			bulletIndent, described = indent, desc
		} else if !wrap {
			bulletIndent, described = -1, false
		}
		listClosed = bulletIndent >= 0 || strings.HasSuffix(line, ".")
		for _, p := range paths {
			expanded, err := ExpandDeclaration(p)
			if err != nil {
				return nil, err
			}
			for _, p := range expanded {
				if !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
				if len(out) > MaxScopePaths {
					return nil, refuse("refused: Context files: exceeds declaration limit")
				}
			}
		}
	}
	if fence != "" {
		return nil, refuse("refused: unclosed Markdown fence; declared scope cannot be established")
	}
	if found && len(out) == 0 {
		return nil, refuse("refused: empty Context files: declaration")
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
// being the accepted "files:" form, so the caller refuses it instead of reading
// a malformed declaration as an empty scope. Whitespace after "files" is always
// a marker, hard-wrapped prose included: neither the line nor its neighbours
// can tell a sentence from a declaration that lost its colon (sec-4809-1). A
// brief whose prose wraps onto "files " is reflowed, not exempted (#4199).
func scopeBroadFilesMarker(line string) bool {
	if !strings.HasPrefix(line, "files") {
		return false
	}
	tail := strings.TrimPrefix(line, "files")
	return tail == "" || strings.ContainsAny(tail[:1], ":*_`(") || tail != strings.TrimLeftFunc(tail, unicode.IsSpace)
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

// scopeAnnotationWord is the capitalised plain word a trailing sentence opens with.
var scopeAnnotationWord = regexp.MustCompile(`^[A-Z][a-z]+$`)

// scopeAnnotation reports whether line, which the declaration grammar refused,
// is a plain sentence that may stand after a closed files: list as its
// annotation (#5097), e.g. "The short paths above are relative to tools/x/.".
// Every condition must hold, and anything ambiguous refuses:
//
//   - printable ASCII only, at least four words, opening with a capitalised
//     word of letters and ending in exactly one ".";
//   - no list or quote marker, and none of the characters a declaration,
//     annotation, description or glob uses: ` * _ { } [ ] < > | \ , ; + ( ) :
//     and no " — ", "..", "#", "?", "!", "=", "$", "&", "@", "%", "~" or '"';
//   - every word holding "/" or "." names a path the list already declares, or
//     a directory above one, so the sentence can neither add a path nor hide
//     one the list leaves out;
//   - every declared path holds a "/" and none ends in ".", so a sentence never
//     blesses a short name it explains as relative, nor an inline tail that kept
//     the closing full stop.
//
// The caller adds the positional rules: column zero, after a list that ended,
// and the block's last line.
func scopeAnnotation(line string, declared []string) bool {
	for i := 0; i < len(line); i++ {
		if line[i] < 0x20 || line[i] > 0x7e {
			return false
		}
	}
	if scopeListPrefix(line) != 0 || strings.ContainsAny(line, "`*_{}[]<>|\\,;+():\"#?!=$&@%~") ||
		strings.Contains(line, "..") || !strings.HasSuffix(line, ".") {
		return false
	}
	words := strings.Fields(strings.TrimSuffix(line, "."))
	if len(words) < 4 || !scopeAnnotationWord.MatchString(words[0]) {
		return false
	}
	// A sentence may only follow a list of full paths. A short name (no '/') is the
	// shape a "relative to" sentence explains, and would bind at the repository root;
	// a declared path ending in '.' is an unbackticked inline tail that kept the
	// sentence-closing full stop. Either way the record would bind the wrong path.
	for _, d := range declared {
		if !strings.Contains(d, "/") || strings.HasSuffix(d, ".") {
			return false
		}
	}
	for _, w := range words {
		if !strings.ContainsAny(w, "/.") {
			continue
		}
		named, covered := strings.TrimSuffix(w, "/"), false
		for _, d := range declared {
			covered = covered || (named != "" && (d == named || strings.HasPrefix(d, named+"/")))
		}
		if !covered {
			return false
		}
	}
	return true
}

// declarationLine reads one line's declarations. described reports that the line
// ended in a description (after " — " or a ':'), which a wrapped line may continue.
func declarationLine(line string) (paths []string, described bool, err error) {
	for _, sep := range []string{" — "} {
		if i := strings.Index(line, sep); i >= 0 {
			line, described = line[:i], true
		}
	}
	line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
	if line == "their tests," {
		return nil, described, nil
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
				return nil, false, refuse("refused: unclosed Context files: action annotation")
			}
			line = strings.TrimSpace(line[end+4:])
			continue
		}
		var p string
		if strings.HasPrefix(line, "`") {
			i := strings.IndexByte(line[1:], '`')
			if i < 0 {
				return nil, false, refuse("refused: unclosed Context files: path quote")
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
			return nil, false, refuse("refused: empty Context files: path")
		}
		result = append(result, p)
		line = strings.TrimSpace(line)
		// Annotations are stripped BEFORE the ':' and terminal tests, so their
		// position never decides whether the remaining tail is understood.
		for {
			for strings.HasPrefix(line, "(") {
				i := strings.IndexByte(line, ')')
				if i < 0 {
					return nil, false, refuse("refused: unclosed Context files: annotation")
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
			described = true
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
		return nil, false, refuse("refused: unsupported Context files: declaration tail " + line)
	}
	return result, described, nil
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

func ExpandDeclaration(decl string) ([]string, error) {
	bad := func() ([]string, error) {
		return nil, refuse("refused: unsupported Context files: path " + fmt.Sprintf("%q", decl))
	}
	if len(decl) == 0 || len(decl) > 4096 || strings.ContainsAny(decl, "\\:<>") || strings.Contains(decl, "...") || strings.Contains(decl, "…") || strings.IndexFunc(decl, unicode.IsSpace) >= 0 || strings.IndexFunc(decl, unicode.IsControl) >= 0 {
		return bad()
	}
	// A recursive glob is accepted in exactly one shape (#5097): one final /**
	// segment after a literal, glob-free directory, as in docs/guide/**. It
	// selects every file below that directory. Any other ** (mid-path, glued to
	// a name, repeated, bare, or straight after a sibling's name) is refused.
	if strings.Contains(decl, "**") {
		dir, recursive := strings.CutSuffix(decl, "/**")
		if !recursive || dir == "" || strings.ContainsAny(dir, "*?[") {
			return bad()
		}
		if _, rel, ok := strings.Cut(strings.TrimPrefix(dir, "../"), "/"); strings.HasPrefix(dir, "../") && (!ok || rel == "") {
			return bad()
		}
	}
	// The sibling form is decided on the declaration as written: a brace or
	// glob can never produce or choose the ../<sibling>/ prefix.
	// Defence in depth, not dead code: the per-path SiblingDeclaration and
	// CanonicalRepoPath checks below are the primary guard and refuse the same
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
			// CanonicalRepoPath(rel) check below (an empty rel fails it) are
			// the primary guards; keep this explicit refusal beside them.
			if !ok {
				return bad()
			}
			check = rel
		}
		if !CanonicalRepoPath(check) || strings.ContainsAny(p, "{},`\"'") {
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
func CanonicalRepoPath(p string) bool {
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
	// and CanonicalRepoPath(rel) are the primary guard against every
	// noncanonical shape it would also catch.
	if !found || !siblingName(sibling) || !CanonicalRepoPath(rel) || path.Clean(decl) != decl {
		return "", "", false
	}
	return sibling, rel, true
}
