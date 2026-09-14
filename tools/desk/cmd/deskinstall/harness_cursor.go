package main

// harness_cursor.go implements deskinstall's SECOND mode, `--harness cursor`:
// it places the Cursor-consumable skills/references/rules tree into an
// adopter repo and writes the shared AGENTS.md bindings block. It is the
// Go-native replacement for steps (1)-(3) of the five manual copy steps
// documented three times (docs/adopting-assay.md, plugins/assay/references/
// cursor.md, plugins/assay/cursor/packaging.md).
//
// It does NOT regenerate plugins/assay/cursor/assay.mdc — that stays
// tools/harnessgen's job, bundle-scoped — this mode only CONSUMES that
// artifact when present. It reads the packaging coverage roster
// (plugins/assay/cursor/packaging.md's `assay:cursor-packaging` block) as the
// single source of the packaged skill set — never a glob over skills/*, which
// would silently ship a skill the roster excludes.
//
// tools/harnessgen is a separate Go module (tools/harnessgen/go.mod) built as
// package main, so its readPackaging/coverageViolations cannot be imported
// here; the roster format is reimplemented against the same marker and the
// same "packaged bare name / `<name> :: EXCLUDED: <reason>`" grammar.
//
// buildCursorPlan is the single source of truth for "what would a run place":
// HarnessCursorPlace (write) and HarnessCursorCheck (--check, no writes) both
// start from it, so the two can never mean something different by "the plan".
import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// cursorPackagingMarker opens the machine-readable coverage roster block in
// plugins/assay/cursor/packaging.md.
const cursorPackagingMarker = "<!-- assay:cursor-packaging"

// bindingsBegin / bindingsEnd delimit the AGENTS.md bindings block this mode
// writes — the same begin/end shape the repo already uses for
// `statusgen:briefs:begin`/`:end` in stream READMEs, so a re-run REPLACES the
// span between them instead of appending a second block.
const (
	bindingsBegin = "<!-- assay:bindings:begin -->"
	bindingsEnd   = "<!-- assay:bindings:end -->"
)

// Harness --check exit contract, matching tools/harnessgen cursor's so the
// two read alike: 0 clean, 1 drift, 2 could-not-check.
const (
	exitHarnessClean         = 0
	exitHarnessDrift         = 1
	exitHarnessCouldNotCheck = 2
)

// HarnessOptions drives one `--harness cursor` run, place or --check.
type HarnessOptions struct {
	BundleDir string // source: a plugins/assay-shaped tree (skills/, references/, cursor/, codex/)
	RepoRoot  string // destination: the adopter repo --repo points at
	Forge     string // "github" | "gitlab" — which desk-transport vocabulary the bindings name
}

// driftKind names the three ways a placed path can disagree with the plan.
type driftKind string

const (
	driftMissing        driftKind = "missing"
	driftExtra          driftKind = "extra"
	driftContentDiffers driftKind = "content-differs"
)

// DriftFinding is one disagreement between a --check run's re-derived plan
// and what is actually on disk under --repo.
type DriftFinding struct {
	Kind driftKind
	Path string // relative to RepoRoot, forward-slash form (OS-neutral in reports)
}

func (d DriftFinding) String() string { return fmt.Sprintf("%s: %s", d.Kind, d.Path) }

// plannedFile is one file the plan wants, Rel relative to RepoRoot (native
// separators for "AGENTS.md", ".cursor/..."-relative for everything else —
// callers add the ".cursor" prefix; see buildCursorPlan's escape check).
type plannedFile struct {
	rel     string
	content []byte
}

// cursorPlan is the fully validated, side-effect-free result of re-deriving
// what a `--harness cursor` run would place. Building it never touches disk
// under RepoRoot beyond a single existing-AGENTS.md read.
type cursorPlan struct {
	files      []plannedFile // ".cursor/skills/**", ".cursor/references/*.md", ".cursor/rules/assay.mdc" (if present), "AGENTS.md"
	ruleNotice string        // non-empty when plugins/assay/cursor/assay.mdc is absent from the bundle (informational, never a failure)
}

// validateRosterName refuses a roster entry that could escape --repo: no
// path separator, no "..", not empty. A roster name is a bare skill
// directory name, never a path.
func validateRosterName(name string) error {
	if name == "" {
		return errors.New("empty roster entry")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("roster entry %q would escape the skills root — refusing to place it", name)
	}
	return nil
}

// readCursorRoster parses the `assay:cursor-packaging` block: bare lines are
// packaged names, `<name> :: EXCLUDED: <reason>` lines are exclusions with a
// mandatory reason, `#`-prefixed and blank lines are comments. An absent
// marker, an unterminated block, a malformed exclusion, an escaping name, or
// a block that parses to zero entries is an error — never a silent
// zero-coverage pass.
func readCursorRoster(path string) (packaged map[string]bool, excluded map[string]string, err error) {
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return nil, nil, fmt.Errorf("reading coverage roster %s: %w", path, rerr)
	}
	text := string(raw)
	start := strings.Index(text, cursorPackagingMarker)
	if start == -1 {
		return nil, nil, fmt.Errorf("coverage roster %s: marker %q not found", path, cursorPackagingMarker)
	}
	rest := text[start+len(cursorPackagingMarker):]
	end := strings.Index(rest, "-->")
	if end == -1 {
		return nil, nil, fmt.Errorf("coverage roster %s: unterminated %q block", path, cursorPackagingMarker)
	}
	block := rest[:end]

	packaged = map[string]bool{}
	excluded = map[string]string{}
	for _, ln := range strings.Split(block, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		if strings.Contains(ln, " :: ") {
			parts := strings.SplitN(ln, " :: ", 2)
			name := strings.TrimSpace(parts[0])
			tail := strings.TrimSpace(parts[1])
			reason := strings.TrimSpace(strings.TrimPrefix(tail, "EXCLUDED:"))
			if !strings.HasPrefix(tail, "EXCLUDED:") || reason == "" || name == "" {
				return nil, nil, fmt.Errorf("coverage roster %s: malformed excluded entry %q", path, ln)
			}
			if verr := validateRosterName(name); verr != nil {
				return nil, nil, fmt.Errorf("coverage roster %s: %w", path, verr)
			}
			excluded[name] = reason
			continue
		}
		if verr := validateRosterName(ln); verr != nil {
			return nil, nil, fmt.Errorf("coverage roster %s: %w", path, verr)
		}
		packaged[ln] = true
	}
	if len(packaged) == 0 && len(excluded) == 0 {
		return nil, nil, fmt.Errorf("coverage roster %s: block parsed to zero entries — refusing to report clean", path)
	}
	return packaged, excluded, nil
}

// diskSkillNames returns the set of skill directory names under skillsDir
// that carry a SKILL.md — the disk side of the coverage rule. A symlinked
// entry at this level is refused outright (Ground rules: "a symlink in the
// source tree is a REFUSAL, not a normalisation").
func diskSkillNames(skillsDir string) (map[string]bool, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("reading skills dir %s: %w", skillsDir, err)
	}
	set := map[string]bool{}
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("skills dir %s: refusing symlinked entry %q", skillsDir, e.Name())
		}
		if !e.IsDir() {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(skillsDir, e.Name(), "SKILL.md")); statErr == nil {
			set[e.Name()] = true
		}
	}
	return set, nil
}

// coverageViolations enforces the bidirectional roster<->disk rule: every
// skill on disk is packaged or excluded-with-reason; every packaged name has
// a directory on disk. An excluded name present on disk is NOT a violation
// here (unlike tools/harnessgen's bundle-build rule, which ships the whole
// skills/ directory as one blob) — this mode places by NAME from the roster,
// so an excluded-but-present skill is simply skipped rather than shipped.
func coverageViolations(disk, packaged map[string]bool, excluded map[string]string) []string {
	var msgs []string
	for _, name := range sortedKeys(disk) {
		if !packaged[name] {
			if _, isExcluded := excluded[name]; !isExcluded {
				msgs = append(msgs, fmt.Sprintf("skill %q is on disk but appears in neither the packaged roster nor the excluded list", name))
			}
		}
	}
	for _, name := range sortedKeys(packaged) {
		if !disk[name] {
			msgs = append(msgs, fmt.Sprintf("packaged skill %q has no skills/%s/SKILL.md on disk — the roster is stale", name, name))
		}
	}
	return msgs
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// forgeBindingLine is the one forge-substituted line this mode adds to the
// shared AGENTS.md fragment: the fragment itself carries no per-forge text,
// so the whole forge vocabulary lives in this single line. The gitlab line
// never contains the standalone backticked token `gh`, and the github line
// never contains "glab" / "--forge gitlab" — the property Verify row 11
// checks.
func forgeBindingLine(forge string) (string, error) {
	switch forge {
	case "github":
		return "**Desk transport for this repo:** GitHub — desk write verbs use `gh`.", nil
	case "gitlab":
		return "**Desk transport for this repo:** GitLab — desk write verbs use `glab` / `--forge gitlab`.", nil
	default:
		return "", fmt.Errorf("unknown --forge %q: expected github or gitlab", forge)
	}
}

// mergeAgentsBindings returns the full desired AGENTS.md content: existing
// content outside the assay:bindings block is kept byte-for-byte; the span
// between the delimiters (if present) is REPLACED with block; otherwise
// block is appended after a single separating blank line. Idempotent:
// feeding its own output back in with an unchanged block reproduces the same
// bytes.
func mergeAgentsBindings(existing []byte, block string) []byte {
	full := bindingsBegin + "\n" + block + "\n" + bindingsEnd + "\n"
	text := string(existing)
	if bi := strings.Index(text, bindingsBegin); bi != -1 {
		if rel := strings.Index(text[bi:], bindingsEnd); rel != -1 {
			ei := bi + rel + len(bindingsEnd)
			after := strings.TrimPrefix(text[ei:], "\n")
			return []byte(text[:bi] + full + after)
		}
	}
	trimmed := strings.TrimRight(text, "\n")
	if trimmed == "" {
		return []byte(full)
	}
	return []byte(trimmed + "\n\n" + full)
}

// collectTree walks src (one skill directory) and returns plannedFiles with
// Rel of the form filepath.Join(relPrefix, <path-under-src>). Refuses on any
// symlink at any depth.
func collectTree(src, relPrefix string) ([]plannedFile, error) {
	var out []plannedFile
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("refusing to copy symlink %s", path)
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		out = append(out, plannedFile{rel: filepath.Join(relPrefix, rel), content: content})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collecting %s: %w", src, err)
	}
	return out, nil
}

// collectFlatMarkdown returns plannedFiles for every top-level *.md file
// directly under dir (non-recursive — references/ carries no subdirs today),
// refusing on a symlinked entry. An empty result is an error: a references/
// placement that copies nothing is the exact silent-break this brief exists
// to prevent.
func collectFlatMarkdown(dir, relPrefix string) ([]plannedFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	var out []plannedFile
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("refusing symlinked entry %s/%s", dir, e.Name())
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			return nil, readErr
		}
		out = append(out, plannedFile{rel: filepath.Join(relPrefix, e.Name()), content: content})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no *.md files found — refusing an empty references placement", dir)
	}
	return out, nil
}

// buildCursorPlan re-derives, WITHOUT touching disk under RepoRoot beyond
// reading any existing AGENTS.md, the complete set of files a `--harness
// cursor` run would place plus the AGENTS.md content it would write.
// HarnessCursorPlace and HarnessCursorCheck both call this so neither can
// drift from what the other means by "the plan".
func buildCursorPlan(opts HarnessOptions) (*cursorPlan, error) {
	if opts.RepoRoot == "" {
		return nil, errors.New("--repo is required")
	}
	repoInfo, statErr := os.Stat(opts.RepoRoot)
	if statErr != nil || !repoInfo.IsDir() {
		return nil, fmt.Errorf("--repo %q does not resolve to a directory: %v", opts.RepoRoot, statErr)
	}

	forgeLine, err := forgeBindingLine(opts.Forge)
	if err != nil {
		return nil, err
	}

	packagingPath := filepath.Join(opts.BundleDir, "cursor", "packaging.md")
	packaged, excluded, err := readCursorRoster(packagingPath)
	if err != nil {
		return nil, err
	}

	skillsDir := filepath.Join(opts.BundleDir, "skills")
	disk, err := diskSkillNames(skillsDir)
	if err != nil {
		return nil, err
	}

	if violations := coverageViolations(disk, packaged, excluded); len(violations) > 0 {
		return nil, fmt.Errorf("coverage rule failed:\n  %s", strings.Join(violations, "\n  "))
	}

	plan := &cursorPlan{}

	for _, name := range sortedKeys(packaged) {
		files, cerr := collectTree(filepath.Join(skillsDir, name), filepath.Join("skills", name))
		if cerr != nil {
			return nil, cerr
		}
		plan.files = append(plan.files, files...)
	}

	refsDir := filepath.Join(opts.BundleDir, "references")
	refFiles, rerr := collectFlatMarkdown(refsDir, "references")
	if rerr != nil {
		return nil, rerr
	}
	plan.files = append(plan.files, refFiles...)

	rulePath := filepath.Join(opts.BundleDir, "cursor", "assay.mdc")
	switch info, lerr := os.Lstat(rulePath); {
	case lerr == nil && info.Mode()&os.ModeSymlink != 0:
		return nil, fmt.Errorf("refusing symlinked bundle artifact %s", rulePath)
	case lerr == nil:
		content, readErr := os.ReadFile(rulePath)
		if readErr != nil {
			return nil, fmt.Errorf("reading %s: %w", rulePath, readErr)
		}
		plan.files = append(plan.files, plannedFile{rel: filepath.Join("rules", "assay.mdc"), content: content})
	case os.IsNotExist(lerr):
		plan.ruleNotice = fmt.Sprintf("NOTICE: %s not present in the bundle — skipping .cursor/rules/assay.mdc (the skills+references copy still stands, cursor.md:25)", rulePath)
	default:
		return nil, fmt.Errorf("stat %s: %w", rulePath, lerr)
	}

	agentsFragmentPath := filepath.Join(opts.BundleDir, "codex", "AGENTS-assay.md")
	fragment, ferr := os.ReadFile(agentsFragmentPath)
	if ferr != nil {
		return nil, fmt.Errorf("reading shared bindings fragment %s: %w", agentsFragmentPath, ferr)
	}
	blockBody := strings.TrimRight(string(fragment), "\n") + "\n\n" + forgeLine

	existingAgents, aerr := os.ReadFile(filepath.Join(opts.RepoRoot, "AGENTS.md"))
	if aerr != nil && !os.IsNotExist(aerr) {
		return nil, fmt.Errorf("reading %s: %w", filepath.Join(opts.RepoRoot, "AGENTS.md"), aerr)
	}
	desiredAgents := mergeAgentsBindings(existingAgents, blockBody)
	plan.files = append(plan.files, plannedFile{rel: "AGENTS.md", content: desiredAgents})

	// Second, independent escape gate (defense in depth — names are already
	// validated at roster-parse time): every non-AGENTS.md planned path must
	// resolve UNDER RepoRoot/.cursor.
	base := filepath.Join(opts.RepoRoot, ".cursor")
	for _, f := range plan.files {
		if f.rel == "AGENTS.md" {
			continue
		}
		full := filepath.Join(base, f.rel)
		rel, relErr := filepath.Rel(base, full)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("planned path %q escapes %s — refusing", f.rel, base)
		}
	}

	return plan, nil
}

// destPath resolves a plannedFile's Rel to its absolute destination:
// RepoRoot/AGENTS.md for the bindings file, RepoRoot/.cursor/<rel> for
// everything else.
func destPath(repoRoot string, f plannedFile) string {
	if f.rel == "AGENTS.md" {
		return filepath.Join(repoRoot, "AGENTS.md")
	}
	return filepath.Join(repoRoot, ".cursor", f.rel)
}

// HarnessCursorPlace validates opts' full plan and, only once the whole plan
// is known-valid, writes it. buildCursorPlan performs every validation and
// disk READ with zero writes under RepoRoot, so a validation failure never
// leaves a partial tree an adopter could mistake for a completed install.
func HarnessCursorPlace(opts HarnessOptions, out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	plan, err := buildCursorPlan(opts)
	if err != nil {
		return err
	}
	placed := 0
	for _, f := range plan.files {
		dst := destPath(opts.RepoRoot, f)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, f.content, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", dst, err)
		}
		if f.rel != "AGENTS.md" {
			placed++
		}
	}
	if plan.ruleNotice != "" {
		fmt.Fprintln(out, plan.ruleNotice)
	}
	fmt.Fprintf(out, "placed cursor harness: %d file(s) under %s%c.cursor, bindings written to %s%cAGENTS.md\n",
		placed, opts.RepoRoot, filepath.Separator, opts.RepoRoot, filepath.Separator)
	return nil
}

// HarnessCursorCheck re-derives the plan and diffs it against what is
// actually on disk under --repo, writing nothing. It returns the drift
// findings (empty == clean) and a could-not-check error when the plan itself
// cannot be built (bad roster, unresolvable repo root, missing bundle
// artifact) — the same three-state contract tools/harnessgen cursor's own
// --check uses.
func HarnessCursorCheck(opts HarnessOptions) ([]DriftFinding, error) {
	plan, err := buildCursorPlan(opts)
	if err != nil {
		return nil, err
	}

	desired := map[string][]byte{}
	for _, f := range plan.files {
		relToRepo := f.rel
		if f.rel != "AGENTS.md" {
			relToRepo = filepath.Join(".cursor", f.rel)
		}
		desired[relToRepo] = f.content
	}

	var findings []DriftFinding
	for relToRepo, content := range desired {
		full := filepath.Join(opts.RepoRoot, relToRepo)
		actual, rerr := os.ReadFile(full)
		switch {
		case os.IsNotExist(rerr):
			findings = append(findings, DriftFinding{Kind: driftMissing, Path: filepath.ToSlash(relToRepo)})
		case rerr != nil:
			return nil, fmt.Errorf("reading %s: %w", full, rerr)
		case !bytes.Equal(actual, content):
			findings = append(findings, DriftFinding{Kind: driftContentDiffers, Path: filepath.ToSlash(relToRepo)})
		}
	}

	// "extra" — anything actually on disk under the placed roots the current
	// plan does not want. This is the single-point-of-failure's second layer:
	// a roster change AFTER a placement shows up here even with the bundle
	// build's own coverage rule bypassed.
	for _, root := range []string{
		filepath.Join(opts.RepoRoot, ".cursor", "skills"),
		filepath.Join(opts.RepoRoot, ".cursor", "references"),
		filepath.Join(opts.RepoRoot, ".cursor", "rules"),
	} {
		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
			if werr != nil {
				if os.IsNotExist(werr) {
					return nil
				}
				return werr
			}
			if d.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(opts.RepoRoot, path)
			if relErr != nil {
				return relErr
			}
			if _, ok := desired[rel]; !ok {
				findings = append(findings, DriftFinding{Kind: driftExtra, Path: filepath.ToSlash(rel)})
			}
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("walking %s: %w", root, walkErr)
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		return findings[i].Path < findings[j].Path
	})
	return findings, nil
}
