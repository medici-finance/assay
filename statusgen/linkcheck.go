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

			if !fileExists(filepath.Join(root, target)) && !fileExists(filepath.Join(filepath.Dir(f), target)) {
				problems = append(problems, fmt.Sprintf("%s: backticked path %q does not exist — for a deliverable this brief will create, mark it `%s` (planned); for a sibling-repo file, prefix it ../<repo>/", rel, target, target))
			}
		}
	}
	return problems
}
