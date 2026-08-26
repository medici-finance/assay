package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// linkCheckExtensions is the neutral, open-core set of file extensions a
// backticked deliverable path must carry to be treated as a real on-disk file
// the link-checker verifies. It is a var, not a compiled-in literal, so a
// deployment adds its own source extensions (e.g. a settlement language's) via
// registerLinkCheckExtensions rather than baking product knowledge into the
// open-core tree.
var linkCheckExtensions = []string{"md", "yaml", "yml", "go", "ts", "sh"}

// buildBacktickRe compiles the backticked-file-path matcher from an extension list.
func buildBacktickRe(exts []string) *regexp.Regexp {
	return regexp.MustCompile("`([A-Za-z0-9_./-]+\\.(?:" + strings.Join(exts, "|") + "))`")
}

// ---------------------------------------------------------------------------
// identifier dereference (mistake-proofing/02) — a named TEST or FUNCTION cited
// in a brief must resolve against the tree it describes, exactly as a backticked
// FILE path already must. The measured incident: three briefs reached
// `implemented` in one pass citing three test names that were in no file, and
// every presence-check row passed on a factually wrong deliverable. A presence
// check on a claim is judgment inspection; making the claim RESOLVE against the
// tree is source inspection on the claim itself (docs/mistake-proofing.md §4 B4).
//
// This check asks whether the NAME RESOLVES, not whether a name is present, and
// not whether the test passes or the function behaves as the brief says — the
// same posture the consumers-routing check took ("never asks 'is the field
// there?'"). Resolution is by name only; that boundary is stated in every
// failure message.
//
// COVERAGE BOUNDARY (disclosed divergence, spec §3 D6 — recorded here beside the
// check, not in a document nobody reads):
//   - Three shapes are matched, and no more: a bare `<name>_test.go` basename
//     with NO directory separator; a `Test<Name>` identifier; and a `func <Name>`
//     reference. Anything else — a struct name, a const, a variable, a method
//     cited without the `func` keyword — is deliberately NOT this check's
//     business; an over-eager matcher on prose earns a permanent exemption file.
//   - A shape must be the ENTIRE backtick span (parallel to buildBacktickRe,
//     whose no-space char-class has the same effect). A `Test<Name>` /
//     `func <Name>` / `<x>_test.go` placeholder carrying angle brackets, or an
//     identifier embedded mid-span inside a shell command, is therefore not
//     matched — the angle-bracket/`<>` placeholder convention and command spans
//     fall out for free.
//   - INHERITED from the backticked-path matcher and NOT fixed here: a
//     directory-shaped or extensionless target is unchecked, and a `_test.go`
//     token WITH a directory separator (e.g. `statusgen/foo_test.go`) is handled
//     as a PATH by buildBacktickRe, not as an identifier here. The bare-basename
//     case this brief reopens is precisely the token the no-directory-separator
//     path rule lets sail through.
//
// Escapes carried over unchanged from the path matcher: the `(planned)`/`(new)`/
// `(future …)` suffix family (the ONE addition this brief makes is that the same
// suffix now also excuses an IDENTIFIER a brief is about to create — no second
// escape syntax); the narrow scope (CLAUDE.md + docs/streams/** only, so
// outbound narrative prose is never subject to it); and the sibling-repo `../`
// prefix (vacuous for identifiers — an angle-bracket-free `_test.go` basename
// cannot carry a `/`).
//
// SEVERITY PHASING (brief task 5): identifierDereferenceFatal gates the class.
// It lands FALSE — every hit, and every could-not-check, is an advisory NOTICE —
// so the inherited corpus census recorded in the landing PR body can be fixed or
// waived with the `(planned)` escape before the check bites. Flipping the const
// to true (a follow-up once the census is zero) makes an unresolved identifier a
// fatal PROBLEM and an unsearchable tree a declining failure. A permanent NOTICE
// is not an acceptable resting state for this check.
const identifierDereferenceFatal = false

// buildIdentifierRe compiles the named-identifier matcher — the parallel to
// buildBacktickRe. It matches a backtick span whose ENTIRE content is one of the
// three shapes; the leading/trailing backticks anchor it, so a placeholder with
// angle brackets or an identifier buried inside a longer command span does not
// match (see the COVERAGE BOUNDARY note above).
func buildIdentifierRe() *regexp.Regexp {
	return regexp.MustCompile("`(" +
		// a bare test-file basename, ending _test.go, with NO directory separator
		// (the char-class excludes `/`); this is the incident's exact shape.
		`[A-Za-z0-9_.-]+_test\.go` + "|" +
		// a Test<Name> identifier (requires at least one identifier char after
		// `Test`, so a bare `Test<Name>` placeholder does not match).
		`Test[A-Za-z0-9_]+` + "|" +
		// a `func <Name>` reference.
		`func [A-Za-z0-9_]+` +
		")`")
}

// registerLinkCheckExtensions extends the neutral extension set and rebuilds the
// matcher. A product build calls this so its own file kinds are link-checked
// WITHOUT being named in the open-core tree.
func registerLinkCheckExtensions(exts ...string) {
	linkCheckExtensions = append(linkCheckExtensions, exts...)
	backtickRe = buildBacktickRe(linkCheckExtensions)
}

var (
	mdLinkRe   = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
	backtickRe = buildBacktickRe(linkCheckExtensions)
	// plannedRe matches the escape suffix for deliverables that do not exist yet:
	// `path` (planned) / `path` (new) / `path` (future ...). Checked immediately
	// after the closing backtick.
	plannedRe = regexp.MustCompile(`^ \((?:planned|new|future[^)]*)\)`)
	// fenceOpenRe matches an opening (or closing) markdown code fence: three or
	// more backticks or tildes at the start of an (optionally indented) line.
	fenceOpenRe = regexp.MustCompile("^(`{3,}|~{3,})")
	// inlineCodeRe matches an inline code span. `[^`\n]*` cannot cross a
	// backtick or a newline, so runs pair up left-to-right on a single line.
	inlineCodeRe = regexp.MustCompile("`+[^`\n]*`+")
	// identifierRe matches a backticked named identifier (test-file basename /
	// Test<Name> / func <Name>) — the parallel to backtickRe. See buildIdentifierRe.
	identifierRe = buildIdentifierRe()
	// funcDeclRe captures the NAME of a Go function or method declaration, so the
	// source index knows which Test<Name>/func <Name> identifiers resolve. It
	// spans an optional receiver — `func (r *T) Method(` and `func Fn(` and the
	// generic `func Fn[` all yield the bare name.
	funcDeclRe = regexp.MustCompile(`(?m)^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)`)
)

func skippable(target string) bool {
	return target == "" ||
		strings.HasPrefix(target, "http") ||
		strings.HasPrefix(target, "#") ||
		// Site-absolute target: a leading `/` is a route on the published docs
		// site (`/contracts`, `/llms-full.txt`), not a path in this checkout.
		// statusgen has no site-root concept — nothing here knows which docs
		// site a repo publishes to, or how its routes map onto files — so a
		// root-relative target is unverifiable and is skipped rather than
		// resolved. (Resolving against the repo root instead would be wrong
		// twice over: it would "pass" routes that merely collide with a repo
		// path and still flag every genuine site route.)
		strings.HasPrefix(target, "/") ||
		strings.ContainsAny(target, "<>* ")
}

// stripCode blanks out fenced code blocks and inline code spans. Markdown link
// SYNTAX inside code is literal text, not a link — a doc quoting
// "`[Title](URL): Description.`" as a format example, or pasting a Go test
// whose fixture string contains "[bad](./missing.md)", is describing links, not
// making them. Scanning those as links produces unfixable failures: the
// "target" is a placeholder that is not supposed to resolve. Applied to the
// markdown-link scan only; the backticked-path heuristic keys off backticks and
// is left reading raw content.
func stripCode(content string) string {
	lines := strings.Split(content, "\n")
	fence := ""
	for i, ln := range lines {
		trimmed := strings.TrimLeft(ln, " \t")
		if fence != "" {
			// Inside a fenced block: blank every line, and close on a fence of
			// the same kind and at least the same length.
			if strings.HasPrefix(trimmed, fence) &&
				strings.Trim(strings.TrimPrefix(trimmed, fence), string(fence[0])+" \t") == "" {
				fence = ""
			}
			lines[i] = ""
			continue
		}
		if open := fenceOpenRe.FindString(trimmed); open != "" {
			fence = open
			lines[i] = ""
			continue
		}
		lines[i] = inlineCodeRe.ReplaceAllString(ln, "")
	}
	return strings.Join(lines, "\n")
}

// withinRoot reports whether an already-joined path stays inside root after
// cleaning away any `..` segments. A `../`-relative doc link that resolves to a
// file INSIDE the repo (e.g. docs/articles/x.md → ../design/y.md) is verifiable
// and must be checked; one that escapes root (the `../<repo>/…` sibling-repo
// convention) cannot be verified in a single-repo checkout and is skipped.
func withinRoot(root, resolved string) bool {
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// backtickPathScope reports whether the strict backticked-path convention (the
// `(planned)` / `../<repo>/` escapes) applies to a file. That discipline is a
// brief-authoring gate, so it is enforced only for CLAUDE.md and the stream/brief
// docs under docs/streams/**. Outbound narrative docs get markdown-link checking
// but are exempt from the noisier backtick heuristic.
func backtickPathScope(root, f string) bool {
	if f == filepath.Join(root, "CLAUDE.md") {
		return true
	}
	rel, err := filepath.Rel(root, f)
	if err != nil {
		return false
	}
	return strings.HasPrefix(filepath.ToSlash(rel), "docs/streams/")
}

// docFiles returns the link-checkable file set (CLAUDE.md plus every *.md under
// docs/**) AND a list of walk problems: docs subtrees that exist but could not
// be read. A walk error was previously discarded (`_ = filepath.WalkDir(...)`
// with an `err == nil` guard inside), so an unreadable docs/ tree enumerated
// zero files, produced zero link problems, and the lint printed LINT: PASS —
// a could-not-check rendered as a clean read (docs/three-state-instrument-rule.md,
// sub-rule 1). The walk problems are returned so the caller can fail the lint
// instead. An ABSENT subtree (os.IsNotExist) is a legitimate empty and is not a
// problem; any OTHER read error is surfaced.
func docFiles(root string) (files []string, walkProblems []string) {
	if p := filepath.Join(root, "CLAUDE.md"); fileExists(p) {
		files = append(files, p)
	}
	// Walk ALL of docs/** — not just docs/streams/. Prospect-facing outbound
	// material (docs/articles/, docs/integrations/, docs/design/, …) must be
	// link-checked too, or broken doc links merge silently.
	docsDir := filepath.Join(root, "docs")
	_ = filepath.WalkDir(docsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Could-not-check: a docs subtree we could not read. An absent one is
			// a legitimate empty (skip); anything else is surfaced so the lint
			// fails rather than silently enumerating fewer files. Keep walking so
			// every unreadable subtree is reported in one pass.
			if !os.IsNotExist(err) {
				walkProblems = append(walkProblems, fmt.Sprintf("docs walk: %s: %v — could-not-check, the link lint's file set is a floor, not a total", p, err))
			}
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			// A generator must not grade its own generated artifacts against a
			// source-quality rule (#169). When statusgen actually emits a register
			// view here (docs/streams/INTAKE.md / FINDINGS.md), that view's
			// content is fixed by the generator's boilerplate templates and the
			// per-entry source files, not authored in the view. Link-checking it
			// would make an adopter fail its own --lint on a path statusgen itself
			// hardcoded (e.g. the `docs/v-next.md` reference in the intake
			// boilerplate) even though the adopter never wrote that path. The
			// per-entry SOURCE files under docs/streams/{intake,findings}/ stay in
			// this set, so genuine adopter-authored link breakage is still caught
			// at its source. isGeneratedRegisterView excludes ONLY a view
			// statusgen would itself write — a hand-authored or scaffolded file at
			// that path, which the generator does not emit, stays checked.
			if !isGeneratedRegisterView(root, p) {
				files = append(files, p)
			}
		}
		return nil
	})
	return files, walkProblems
}

// isGeneratedRegisterView reports whether p is a register view that statusgen
// ACTUALLY generates for this root: p sits directly under docs/streams/ with a
// basename in registerViewNames (INTAKE.md / FINDINGS.md) AND the matching
// generator emits non-empty content from the per-entry source files.
//
// The second condition is load-bearing. Location alone is not proof of
// generation. A file at docs/streams/INTAKE.md is only statusgen's output when
// there are per-entry intake files under docs/streams/intake/ to render from; a
// hand-authored legacy register (the append-only single-file INTAKE.md some
// repos still carry) or the INTAKE.md that `statusgen init` scaffolds for an
// adopter to edit lives in a root with no per-entry files, so the generator
// returns "" and writeRegisterViews writes nothing there. Such a file is
// authored, not generated — it stays link-checked. Excluding it would silently
// stop checking a real, editable document. Only a view statusgen would itself
// write is exempt from the source-quality link check (#169).
//
// An identically-named file elsewhere in the tree (docs/articles/INTAKE.md, a
// per-entry docs/streams/intake/INTAKE.md, a root-level INTAKE.md) is never a
// generated view and stays checked.
func isGeneratedRegisterView(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	// rel is already slash-normalised, so path.Split is the exact idiom.
	dir, base := path.Split(filepath.ToSlash(rel))
	if dir != "docs/streams/" || !registerViewNames[base] {
		return false
	}
	// Location and basename match a view name; confirm statusgen would actually
	// emit it here. An empty result means no per-entry source files exist, so the
	// on-disk file was authored rather than generated — keep checking it.
	var view string
	switch base {
	case "INTAKE.md":
		view, err = generateIntakeView(root)
	case "FINDINGS.md":
		view, err = generateFindingsView(root)
	default:
		return false
	}
	// A generator error is not a confirmation that the file is generated output;
	// fail safe by keeping the file in the link-check set. The same parse error
	// surfaces through the register lint / checkRegisterViews.
	return err == nil && view != ""
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// assayRootMarkers name the files whose presence marks a directory as an assay
// root: a self-contained tree with its own board (STATUS.md) and its own pinned
// toolchain (.assay-versions), both of which statusgen writes/reads at the TOP of
// a root (never at docs/streams/). A NESTED root occurs when a stream ships an
// end-user-extractable skeleton whose OWN repo root is a subdirectory of this
// repo — e.g. a tutorial skeleton the learner extracts and then treats as their
// repo root. Prose inside that skeleton is authored relative to THAT nested root
// (correct for the extracted-and-run end user), not the outer repo root.
var assayRootMarkers = []string{".assay-versions", "STATUS.md"}

// isAssayRootDir reports whether dir carries an assay-root marker.
func isAssayRootDir(dir string) bool {
	for _, m := range assayRootMarkers {
		if fileExists(filepath.Join(dir, m)) {
			return true
		}
	}
	return false
}

// nestedRootBase returns the nearest ancestor of file f that is itself an assay
// root nested strictly BELOW root — a directory carrying its own
// .assay-versions/STATUS.md. Backticked deliverable paths in markdown under such
// a directory resolve against THAT nested root (the extracted-repo root the
// end user runs), so the path-existence check adds it as a resolution base. ""
// means no nested root applies and resolution stays against the outer root and
// the containing directory, exactly as before — so a normal brief's resolution
// is byte-for-byte unchanged. The outer root itself is never returned (it is
// already a resolution base); the walk stops at root and never escapes above it.
func nestedRootBase(root, f string) string {
	root = filepath.Clean(root)
	dir := filepath.Clean(filepath.Dir(f))
	for {
		if dir == root {
			return "" // reached the outer root — already a resolution base
		}
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "" // walked outside/above root — stop, never escape the repo
		}
		if isAssayRootDir(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // filesystem root reached (defensive; root-clamp handles it)
		}
		dir = parent
	}
}

func linkProblems(root string, files []string) []string {
	var problems []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", f, err))
			continue
		}
		rel, _ := filepath.Rel(root, f)
		content := string(raw)

		// Check markdown links — outside code blocks and code spans, where
		// link syntax is quoted text rather than a link.
		for _, m := range mdLinkRe.FindAllStringSubmatch(stripCode(content), -1) {
			target := strings.TrimSpace(m[1])
			if i := strings.Index(target, "#"); i > 0 {
				target = target[:i]
			}
			if skippable(target) {
				continue
			}
			// Resolve `../`-relative targets rather than skipping them: they are
			// the exact form of typical cross-doc links. A
			// target that resolves outside the repo is a sibling-repo reference
			// and cannot be verified here.
			resolved := filepath.Join(filepath.Dir(f), target)
			if !withinRoot(root, resolved) {
				continue
			}
			if !fileExists(resolved) {
				problems = append(problems, fmt.Sprintf("%s: dead link %q", rel, m[1]))
			}
		}

		// Check backticked paths with (planned) marker support — but ONLY on the
		// authored, convention-bound surfaces (CLAUDE.md + docs/streams/** briefs).
		// Outbound narrative docs (articles, specs, design) legitimately mention
		// planned, cross-repo, or explicitly-absent paths in prose (e.g. "No
		// `docs/brand-guide.md` exists in this repo"), so the backticked-path
		// discipline heuristic produces false positives there — they still get
		// full markdown-link checking above.
		if !backtickPathScope(root, f) {
			continue
		}
		matches := backtickRe.FindAllStringSubmatchIndex(content, -1)
		for _, match := range matches {
			// match[0] and match[1] are start/end of full regex match
			// match[2] and match[3] are start/end of captured group (the path)
			fullMatchEnd := match[1]
			target := content[match[2]:match[3]]

			// The backticked-path convention documents `../<repo>/…` as the
			// escape for a sibling-repo file (see the failure message below), so
			// a `../`-prefixed backtick target is intentionally left unchecked.
			if skippable(target) || strings.HasPrefix(target, "../") || !strings.Contains(target, "/") {
				continue // bare filenames are too ambiguous to resolve
			}

			// Skip planned deliverables: closing backtick immediately followed
			// by " (planned)" / " (new)" / " (future ...)".
			if plannedRe.MatchString(content[fullMatchEnd:]) {
				continue
			}

			// Resolution bases: the outer repo root and the file's own directory,
			// plus — when the file lives under a NESTED assay root (a skeleton with
			// its own .assay-versions/STATUS.md) — that nested root, against which
			// the skeleton's prose paths are authored. A genuinely-broken path still
			// resolves against none of the bases and is still reported, so nested-root
			// awareness removes the false positive WITHOUT weakening real coverage.
			exists := fileExists(filepath.Join(root, target)) || fileExists(filepath.Join(filepath.Dir(f), target))
			if !exists {
				if nb := nestedRootBase(root, f); nb != "" {
					exists = fileExists(filepath.Join(nb, target))
				}
			}
			if !exists {
				problems = append(problems, fmt.Sprintf("%s: backticked path %q does not exist — for a deliverable this brief will create, mark it `%s` (planned); for a sibling-repo file, prefix it ../<repo>/", rel, target, target))
			}
		}
	}
	return problems
}

// sourceIndex is the one-pass index of the source tree the identifier check
// resolves against: every file BASENAME (so a bare `<name>_test.go` resolves if
// such a file exists anywhere), and every declared Go function/method NAME (so a
// `Test<Name>` or `func <Name>` resolves if a declaration of that name exists).
// Built once per lint, not a process spawn per token.
type sourceIndex struct {
	fileBasenames map[string]bool
	funcNames     map[string]bool
}

// buildSourceIndex walks the tree ONCE, recording file basenames and Go
// function/method declaration names. A walk error is returned, never swallowed:
// an unsearchable tree is could-not-check, not a clean read
// (docs/three-state-instrument-rule.md). The .git directory is skipped — it
// holds no source declarations and is large.
func buildSourceIndex(root string) (*sourceIndex, error) {
	idx := &sourceIndex{fileBasenames: map[string]bool{}, funcNames: map[string]bool{}}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Propagate: the caller turns this into a could-not-check that declines
			// the whole check rather than passing on a truncated index.
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		idx.fileBasenames[d.Name()] = true
		if strings.HasSuffix(p, ".go") {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			for _, m := range funcDeclRe.FindAllStringSubmatch(string(b), -1) {
				idx.funcNames[m[1]] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return idx, nil
}

// resolves reports whether a matched identifier token names something in the
// tree. A `_test.go` basename resolves against file basenames; a `Test<Name>` or
// `func <Name>` token resolves against declared function/method names. Resolution
// is BY NAME ONLY — it does not verify the test passes or the function does what
// the brief claims.
func (idx *sourceIndex) resolves(token string) bool {
	switch {
	case strings.HasPrefix(token, "func "):
		return idx.funcNames[strings.TrimSpace(strings.TrimPrefix(token, "func "))]
	case strings.HasSuffix(token, "_test.go"):
		return idx.fileBasenames[token]
	default: // Test<Name>
		return idx.funcNames[token]
	}
}

// identifierDereferenceCheck resolves every backticked named identifier in the
// convention-bound surfaces against the source tree. See the design note at the
// top of this file for the shapes, escapes, coverage boundary, and the
// identifierDereferenceFatal severity phasing. Returns (problems, notices); which
// one a hit lands in is gated by identifierDereferenceFatal.
func identifierDereferenceCheck(root string, files []string) (problems, notices []string) {
	const tag = "[id-dereference]"
	emit := func(msg string) {
		if identifierDereferenceFatal {
			problems = append(problems, tag+" "+msg)
		} else {
			notices = append(notices, tag+" "+msg)
		}
	}

	idx, err := buildSourceIndex(root)
	if err != nil {
		// Could-not-check: an unsearchable tree declines the whole check — it is
		// printed as could-not-check, never rounded up to a clean read.
		emit(fmt.Sprintf("identifier dereference COULD-NOT-CHECK: the source tree could not be indexed (%v) — the check declines rather than passing; absence of evidence is not evidence of absence (docs/three-state-instrument-rule.md)", err))
		return problems, notices
	}

	for _, f := range files {
		if !backtickPathScope(root, f) {
			continue // narrow scope: CLAUDE.md + docs/streams/** only, unchanged.
		}
		raw, e := os.ReadFile(f)
		if e != nil {
			emit(fmt.Sprintf("%v: identifier dereference COULD-NOT-CHECK: %v", f, e))
			continue
		}
		rel, _ := filepath.Rel(root, f)
		content := string(raw)
		for _, match := range identifierRe.FindAllStringSubmatchIndex(content, -1) {
			// match[1] = end of full match (the closing backtick); match[2:4] = the
			// captured identifier token.
			fullMatchEnd := match[1]
			token := content[match[2]:match[3]]
			// The `(planned)`/`(new)`/`(future …)` escape carries over from the path
			// matcher, and is the ONE addition this brief makes for identifiers: a
			// brief describing a name it is about to create marks it illustrative
			// with the same suffix, and is not blocked.
			if plannedRe.MatchString(content[fullMatchEnd:]) {
				continue
			}
			if idx.resolves(token) {
				continue
			}
			emit(fmt.Sprintf("%s: named identifier `%s` resolves against no file or declaration in the tree — this check resolves the NAME only (not that the test passes or the function behaves as described). If this is a name the brief is about to create, mark it `%s` (planned)", rel, token, token))
		}
	}
	return problems, notices
}
