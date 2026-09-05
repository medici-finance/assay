package main

// migrate.go — `statusgen migrate brief-v1-to-v2 [--dry-run] [--root DIR]`
// (derived-board/06). The brief-v1 → brief-v2 flag-day migration, run as the
// executable half of a `deskmigrate` `statusgen-regen` op so the migration file
// stays declarative and dry-runnable.
//
// The verb rewrites every `schema: brief-v1` brief in the tree to brief-v2:
//
//   - `schema: brief-v1`            → `schema: brief-v2`
//   - `brief: <stream>/<NN>`        → `brief: <cell>:<repo>:<stream>:<NN>`
//     (the hierarchical id, spec §5 / dependency-graph-design.md §3.3), resolving
//     <cell>/<repo> from the tree's docs/streams/graph-repos.yaml alias registry.
//   - adds `version: 1` where the brief carries no `version:` key.
//   - mints `id: <uuid v4>` where the brief carries no `id:` key (spec §8 — the
//     id is minted ONCE, at migration; a uuid added later is a uuid with no
//     history).
//
// and wraps each stream README's Briefs table in the generated-region markers
// (readmetable.go) and adds `board: generated` to its frontmatter.
//
// It REFUSES (exit 5) when docs/streams/graph-repos.yaml is absent — the v2 id
// form cannot be minted without the alias registry, so the adopter writes the
// registry first (the release note says so). It REFUSES (exit 5) when a stream
// README carries a Briefs section with no recognisable table to wrap.
//
// It is IDEMPOTENT: a brief already on brief-v2 is left byte-for-byte untouched,
// and a README already wrapped in markers with `board: generated` is untouched.
// A second run therefore changes nothing and exits 0 — the property the
// deskmigrate op relies on.
//
// It is pure over the tree and OFFLINE: it reads and writes only files under
// --root, never the network.

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// migrateExitOK / migrateExitRefused mirror the deskkit exit-code contract this
// verb shares with deskmigrate: 0 ok (applied, or a clean no-op), 5 refused (a
// determinate precondition failure — no registry, or an un-wrappable README).
const (
	migrateExitOK      = 0
	migrateExitRefused = 5
	migrateExitUsage   = 2
)

// migrateSchemaV1Line / migrateSchemaV2Line are the exact frontmatter lines the
// rewrite swaps. Matching the whole line (after trimming) keeps a `schema:` value
// that is some other document kind untouched.
const (
	migrateFromSchema = "brief-v1"
	migrateToSchema   = "brief-v2"
)

// runMigrate is the `statusgen migrate <target> [flags]` entry point.
func runMigrate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "statusgen migrate: a migration target is required (only brief-v1-to-v2 is defined)")
		return migrateExitUsage
	}
	target := args[0]
	if target != "brief-v1-to-v2" {
		fmt.Fprintf(stderr, "statusgen migrate: unknown migration target %q (only brief-v1-to-v2 is defined)\n", target)
		return migrateExitUsage
	}

	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "tree root to migrate")
	dryRun := fs.Bool("dry-run", false, "preview only; write nothing")
	if err := fs.Parse(args[1:]); err != nil {
		return migrateExitUsage
	}

	return migrateBriefV1ToV2(*root, *dryRun, stdout, stderr)
}

// migratePlan is one line of the per-file migration plan a --dry-run prints and an
// apply performs.
type migratePlan struct {
	path    string
	desc    string
	changed bool
}

// migrateBriefV1ToV2 performs (or, under dryRun, plans) the whole brief-v1→v2
// migration over root. It returns the process exit code.
func migrateBriefV1ToV2(root string, dryRun bool, stdout, stderr io.Writer) int {
	// The alias registry is REQUIRED — the hierarchical id cannot be minted
	// without a cell + repo alias, so its absence is a determinate refusal, not a
	// silent skip. Checked first so the refusal fires before any file is touched.
	reg, err := loadMigrateRegistry(root)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen migrate: %v\n", err)
		return migrateExitRefused
	}

	mode := "apply"
	if dryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(stdout, "statusgen migrate brief-v1-to-v2 (%s) — root %s, cell %s, repo %s\n",
		mode, root, reg.cell, reg.self)

	var plans []migratePlan

	// 1. Briefs.
	briefs, err := migrateBriefFiles(root)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen migrate: %v\n", err)
		return migrateExitRefused
	}
	for _, path := range briefs {
		p, err := migrateOneBrief(path, reg, dryRun)
		if err != nil {
			fmt.Fprintf(stderr, "statusgen migrate: %v\n", err)
			return migrateExitRefused
		}
		if p != nil {
			plans = append(plans, *p)
		}
	}

	// 2. Stream READMEs — wrap the Briefs table + add board: generated.
	readmes, err := migrateStreamReadmes(root)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen migrate: %v\n", err)
		return migrateExitRefused
	}
	for _, path := range readmes {
		p, err := migrateOneReadme(path, dryRun)
		if err != nil {
			fmt.Fprintf(stderr, "statusgen migrate: %v\n", err)
			return migrateExitRefused
		}
		if p != nil {
			plans = append(plans, *p)
		}
	}

	// After wrapping, NORMALISE each now-generated README's Briefs region to a
	// fresh render from the (now brief-v2) frontmatter, so the migrated tree is
	// immediately `statusgen --lint`-clean rather than reddening on a stale
	// authoring cell (e.g. a `./brief-…` link the generator spells without `./`).
	// Lifecycle columns are preserved by the regen path. Apply mode only — a
	// dry-run writes nothing.
	if !dryRun {
		if err := migrateRegenReadmes(root); err != nil {
			fmt.Fprintf(stderr, "statusgen migrate: %v\n", err)
			return migrateExitRefused
		}
	}

	changedCount := 0
	for _, p := range plans {
		prefix := "  "
		if p.changed {
			changedCount++
			if dryRun {
				prefix = "  WOULD "
			}
		}
		fmt.Fprintf(stdout, "%s%s: %s\n", prefix, relTo(root, p.path), p.desc)
	}
	if changedCount == 0 {
		fmt.Fprintln(stdout, "statusgen migrate: nothing to do (already brief-v2) — clean no-op")
	} else if dryRun {
		fmt.Fprintf(stdout, "statusgen migrate: %d file(s) WOULD change (dry-run wrote nothing)\n", changedCount)
	} else {
		fmt.Fprintf(stdout, "statusgen migrate: %d file(s) changed\n", changedCount)
	}
	return migrateExitOK
}

// migrateRegistry is the minimal view of graph-repos.yaml the migration needs:
// the cell, and the SELF repo alias whose namespace this tree's own briefs carry.
type migrateRegistry struct {
	cell string
	self string
}

// loadMigrateRegistry reads docs/streams/graph-repos.yaml and resolves the self
// repo alias. The self alias is the explicit `self:` key when present; otherwise
// it is inferred as the single published alias (a non-null `repo:` that is not
// `unpublished`). Zero or several published aliases with no explicit `self:` is a
// refusal — the migration must not guess which repo the tree is.
func loadMigrateRegistry(root string) (migrateRegistry, error) {
	path := filepath.Join(root, "docs", "streams", "graph-repos.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return migrateRegistry{}, fmt.Errorf(
				"docs/streams/graph-repos.yaml is absent — the brief-v2 id form <cell>:<repo>:<stream>:<NN> "+
					"cannot be minted without the alias registry; write docs/streams/graph-repos.yaml first (see the release note)")
		}
		return migrateRegistry{}, fmt.Errorf("cannot read docs/streams/graph-repos.yaml: %v", err)
	}
	var doc struct {
		Schema string `yaml:"schema"`
		Cell   string `yaml:"cell"`
		Self   string `yaml:"self"`
		Repos  map[string]struct {
			Cell        string `yaml:"cell"`
			Repo        string `yaml:"repo"`
			Unpublished bool   `yaml:"unpublished"`
		} `yaml:"repos"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return migrateRegistry{}, fmt.Errorf("cannot parse docs/streams/graph-repos.yaml: %v", err)
	}
	if doc.Schema != "graph-repos-v1" {
		return migrateRegistry{}, fmt.Errorf("docs/streams/graph-repos.yaml schema must be graph-repos-v1, got %q", doc.Schema)
	}
	if doc.Cell == "" {
		return migrateRegistry{}, fmt.Errorf("docs/streams/graph-repos.yaml cell must be non-empty")
	}
	if doc.Self != "" {
		if _, ok := doc.Repos[doc.Self]; !ok {
			return migrateRegistry{}, fmt.Errorf("docs/streams/graph-repos.yaml self alias %q is not in repos", doc.Self)
		}
		return migrateRegistry{cell: doc.Cell, self: doc.Self}, nil
	}
	// Infer: the single published alias.
	var published []string
	for alias, e := range doc.Repos {
		if e.Repo != "" && !e.Unpublished {
			published = append(published, alias)
		}
	}
	switch len(published) {
	case 1:
		return migrateRegistry{cell: doc.Cell, self: published[0]}, nil
	case 0:
		return migrateRegistry{}, fmt.Errorf(
			"docs/streams/graph-repos.yaml names no published repo alias to use as self — add a `self: <alias>` key naming this tree's own repo")
	default:
		return migrateRegistry{}, fmt.Errorf(
			"docs/streams/graph-repos.yaml names %d published repo aliases (%s) — add a `self: <alias>` key naming this tree's own repo so the migration does not guess",
			len(published), strings.Join(published, ", "))
	}
}

// migrateBriefFiles lists brief files under docs/streams, ordered for stable
// output.
func migrateBriefFiles(root string) ([]string, error) {
	base := filepath.Join(root, "docs", "streams")
	var out []string
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasPrefix(name, "brief-") && strings.HasSuffix(name, ".md") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cannot walk docs/streams: %v", err)
	}
	return out, nil
}

var migrateBriefLineRe = regexp.MustCompile(`^brief:\s*([a-z0-9][a-z0-9-]*)/([0-9]+[a-z]?)\s*$`)

// migrateOneBrief rewrites a single brief-v1 file to brief-v2. It is a no-op
// (returns nil) for a file that is not schema: brief-v1 (already v2, or a
// non-brief document). It refuses when a brief-v1 file's `brief:` line is not the
// <stream>/<NN> form it must rewrite.
func migrateOneBrief(path string, reg migrateRegistry, dryRun bool) (*migratePlan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %v", path, err)
	}
	content := string(raw)
	// Only a leading `---` frontmatter file participates.
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return nil, nil
	}
	// Split the frontmatter block (between the first two `---` fences).
	nl := strings.IndexByte(content, '\n')
	rest := content[nl+1:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, nil // unterminated frontmatter — not this migration's concern
	}
	front := rest[:end]
	after := rest[end:] // begins with "\n---..."

	lines := strings.Split(front, "\n")
	// Only migrate a file explicitly on schema: brief-v1.
	isV1 := false
	hasVersion := false
	hasID := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "schema: "+migrateFromSchema || t == "schema: \""+migrateFromSchema+"\"" {
			isV1 = true
		}
		if strings.HasPrefix(t, "version:") {
			hasVersion = true
		}
		if strings.HasPrefix(t, "id:") {
			hasID = true
		}
	}
	if !isV1 {
		return nil, nil
	}

	for i, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case t == "schema: "+migrateFromSchema || t == "schema: \""+migrateFromSchema+"\"":
			lines[i] = "schema: " + migrateToSchema
		default:
			if m := migrateBriefLineRe.FindStringSubmatch(t); m != nil {
				lines[i] = fmt.Sprintf("brief: %s:%s:%s:%s", reg.cell, reg.self, m[1], m[2])
			}
		}
	}
	// Append version: 1 and a minted id: where absent, in a deterministic place
	// (end of frontmatter), so a re-run that finds them present is a no-op.
	if !hasVersion {
		lines = append(lines, "version: 1")
	}
	if !hasID {
		id, err := newUUIDv4()
		if err != nil {
			return nil, fmt.Errorf("cannot mint id for %s: %v", path, err)
		}
		lines = append(lines, "id: "+id)
	}

	newContent := "---\n" + strings.Join(lines, "\n") + after
	if newContent == content {
		return nil, nil
	}
	plan := &migratePlan{path: path, desc: "rewrite brief-v1 → brief-v2 (schema, hierarchical id, version, id)", changed: true}
	if dryRun {
		return plan, nil
	}
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return nil, fmt.Errorf("cannot write %s: %v", path, err)
	}
	return plan, nil
}

// migrateStreamReadmes lists stream README.md files (one per docs/streams/<stream>
// dir), ordered for stable output.
func migrateStreamReadmes(root string) ([]string, error) {
	base := filepath.Join(root, "docs", "streams")
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read docs/streams: %v", err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(base, e.Name(), "README.md")
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out, nil
}

var migrateTableHeadRe = regexp.MustCompile(`(?m)^\|\s*#\s*\|\s*Brief\s*\|`)

// migrateOneReadme adds board: generated to a stream README frontmatter and wraps
// its Briefs table in the generated-region markers. It is a no-op when the README
// is already board: generated with markers. It refuses when the README has a
// Briefs section but no recognisable table to wrap.
func migrateOneReadme(path string, dryRun bool) (*migratePlan, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %v", path, err)
	}
	content := string(raw)

	alreadyGenerated := regexp.MustCompile(`(?m)^board:\s*generated\s*$`).MatchString(content)
	alreadyWrapped := strings.Contains(content, briefsMarkerBegin) && strings.Contains(content, briefsMarkerEnd)
	if alreadyGenerated && alreadyWrapped {
		return nil, nil // idempotent no-op
	}

	// Locate the table (header row `| # | Brief | ... |` plus its separator and
	// the consecutive `|...|` data rows).
	loc := migrateTableHeadRe.FindStringIndex(content)
	if loc == nil {
		return nil, fmt.Errorf("%s: stream README has no recognisable Briefs table (a `| # | Brief | … |` header) to wrap in generated markers", path)
	}
	tableStart := loc[0]
	// Extend to the end of the contiguous table (all following lines beginning
	// with `|`).
	tail := content[tableStart:]
	tblLines := strings.Split(tail, "\n")
	n := 0
	for n < len(tblLines) {
		t := strings.TrimSpace(tblLines[n])
		if strings.HasPrefix(t, "|") {
			n++
			continue
		}
		break
	}
	tableText := strings.Join(tblLines[:n], "\n")
	tableEnd := tableStart + len(tableText)

	newContent := content
	if !alreadyWrapped {
		newContent = content[:tableStart] + briefsMarkerBegin + "\n" + tableText + "\n" + briefsMarkerEnd + content[tableEnd:]
	}
	if !alreadyGenerated {
		newContent, err = migrateAddBoardGenerated(newContent)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", path, err)
		}
	}
	if newContent == content {
		return nil, nil
	}
	plan := &migratePlan{path: path, desc: "wrap Briefs table in generated markers + add board: generated", changed: true}
	if dryRun {
		return plan, nil
	}
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return nil, fmt.Errorf("cannot write %s: %v", path, err)
	}
	return plan, nil
}

// migrateRegenReadmes regenerates the Briefs region of every board: generated
// stream README under root from the brief frontmatter (the same path `statusgen
// regen --readmes` takes), so the migration's output is lint-clean. Preserves the
// lifecycle columns. A tree with no board root is a clean no-op.
func migrateRegenReadmes(root string) error {
	boardRoot, found := findBoardRoot(root)
	if !found {
		return nil
	}
	streams, _, err := loadStreams(boardRoot)
	if err != nil {
		return fmt.Errorf("regen readmes: %v", err)
	}
	for _, s := range streams {
		if s.Board != "generated" {
			continue
		}
		if _, err := rewriteReadmeRegion(s, s.Dir+"/README.md"); err != nil {
			return fmt.Errorf("regen readmes: %v", err)
		}
	}
	return nil
}

// migrateAddBoardGenerated inserts a `board: generated` line into the README's
// YAML frontmatter (before the closing `---`). It refuses a README with no
// frontmatter — a stream README always carries one.
func migrateAddBoardGenerated(content string) (string, error) {
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return "", fmt.Errorf("stream README has no `---` frontmatter to add board: generated to")
	}
	nl := strings.IndexByte(content, '\n')
	rest := content[nl+1:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", fmt.Errorf("stream README frontmatter is not closed with `---`")
	}
	front := rest[:end]
	after := rest[end:]
	return "---\n" + front + "\nboard: generated" + after, nil
}

// newUUIDv4 mints a canonical random uuid v4 (spec §8: minted once, at migration).
func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// relTo renders path relative to root for readable plan output, falling back to
// the absolute path when it cannot.
func relTo(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}
