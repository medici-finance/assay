// Package topology is the desk-side reader for the ONE declared source of
// org/repo/App/product topology: `topology.yaml` at the repo root.
//
// WHAT PROBLEM IT SOLVES. The same topology facts were carried in five-plus
// parallel hand-maintained Go tables with no generator and no cross-check,
// and two of them had already drifted (issueboard's excluded-label
// set was missing `review-request`, which statusgen's carries). The stream
// convention is derive-or-diff: every fact on more than one surface has exactly
// ONE declared source, and every other occurrence is regenerated from it or
// DIFFED against it in CI. Hand-maintained second copies are the defect.
//
// WHY THERE IS STILL A COMPILED COPY, AND WHY THAT IS NOT THE DEFECT. The desk
// tools ship as pinned standalone binaries that run from an arbitrary working
// directory — `deskpost` gating a flip has no repo root to read, and a gate that
// needs the filesystem is a gate that fails open when the filesystem is not the
// repo. So the values live compiled in `compiled.go`, and `TestTopologyDriftRegistry`
// diffs that compiled value against `topology.yaml` and fails NAMING THE SITE
// when they disagree. One declared source, one derivation, one enforced diff.
//
// WHAT IS NOT IN topology.yaml, deliberately: the live adopter roster (the real
// allowed-repo set, trusted logins, App ids, human->login map). That is operator
// configuration read from outside every ref the tools evaluate
// (deskkit/rosterconfig.go) and this package does not reverse that. See the
// header of topology.yaml.
//
// NO SECRETS. This package models ids and roles only. It has no field for a
// token, a PEM path, a key location or any other credential surface, and adding
// one would be a schema change a reviewer must refuse.
package topology

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// SourceFile is the repo-relative path of the declared source. Every consumer
// that names the file names it through this constant, so a rename is one edit.
const SourceFile = "topology.yaml"

// Schema is the schema version this reader accepts. A source declaring anything
// else is REFUSED rather than read on a best-effort basis: a reader that guesses
// at an unknown schema is how a silently-changed meaning ships.
const Schema = "topology-v1"

// Visibility is a repo's stated GitHub visibility. The zero value is
// VisibilityUnknown on purpose: "not stated" must never read as "private".
// It mirrors deskkit.Visibility, which is the gate-side type; this package does
// not import deskkit (deskkit imports THIS package).
type Visibility int

const (
	// VisibilityUnknown — not stated, or a value this reader does not recognise.
	// Fails closed at every consumer.
	VisibilityUnknown Visibility = iota
	// VisibilityPublic — world-readable. Risk-classed unconditionally.
	VisibilityPublic
	// VisibilityPrivate — the only value that does not risk-class on visibility.
	VisibilityPrivate
)

func (v Visibility) String() string {
	switch v {
	case VisibilityPublic:
		return "public"
	case VisibilityPrivate:
		return "private"
	default:
		return "unknown"
	}
}

// parseVisibility maps a stated visibility to a Visibility. Only the exact
// strings "public" and "private" parse; "unknown", "", and anything else are
// VisibilityUnknown, which fails closed.
func parseVisibility(s string) Visibility {
	switch s {
	case "public":
		return VisibilityPublic
	case "private":
		return VisibilityPrivate
	default:
		return VisibilityUnknown
	}
}

// Relationship is the CELL-MODEL relationship between the cell whose instance
// this file is and one repo it states.
//
// THE ZERO VALUE IS RelationshipUpstream ON PURPOSE. Absent must never read as
// owned: `upstream` is the least-authority reading (a cell may file issues into
// an upstream repo, but never modifies it and never runs jobs against it), so a
// source that says nothing grants nothing. It mirrors VisibilityUnknown's
// fail-closed zero and the UNPINNED reading of an unbound role.
//
// IT IS NOT AN AUTHORISATION. Relationship is a STATED-TOPOLOGY fact and gates
// nothing. The write boundary stays roster configuration (ASSAY_ALLOWED_REPOS,
// deskkit/rosterconfig.go), which is already per-cell-shaped — each cell's
// deployment sets its own. A future reader that consults Relationship to decide
// whether a write is permitted has moved the write boundary into a tracked file,
// which is a change a reviewer must refuse.
type Relationship int

const (
	// RelationshipUpstream — the cell files issues into this repo and reads it,
	// but does not modify it and does not run jobs against it. The default.
	RelationshipUpstream Relationship = iota
	// RelationshipOwned — this cell is the repo's owner. A repo is owned by AT
	// MOST ONE cell; see the one-owner boundary in docs/adopting-assay.md.
	RelationshipOwned
)

func (r Relationship) String() string {
	if r == RelationshipOwned {
		return "owned"
	}
	return "upstream"
}

// parseRelationship maps a stated relationship to a Relationship. ok is false
// for anything that is not exactly "owned" or "upstream" — including the empty
// string. Unlike visibility, there is no third value that means "not stated":
// absence is expressed by the KEY being absent, so a present-but-unrecognised
// value is a typo, and a typo that silently defaulted would read as a stated
// fact nobody stated.
func parseRelationship(s string) (Relationship, bool) {
	switch s {
	case "owned":
		return RelationshipOwned, true
	case "upstream":
		return RelationshipUpstream, true
	}
	return RelationshipUpstream, false
}

// Repo is one repo's stated topology.
type Repo struct {
	// Slug is "owner/name".
	Slug string
	// Visibility is the STATED visibility. Unknown fails closed.
	Visibility Visibility
	// CIRequired is true when the repo runs PR CI, so an EMPTY check rollup
	// reads as "not reported yet" (unverifiable) rather than green. It defaults
	// to TRUE when unstated — the fail-closed direction.
	CIRequired bool
	// Root is the repo's default local checkout path for the multi-repo board,
	// or "" when the repo is not a board root.
	Root string
	// RiskPathTriggers are ADDITIONAL risk-classing path prefixes for this repo,
	// layered on top of the universal base list. Never narrows.
	RiskPathTriggers []string
	// Note is the prose rationale from the source. Display only.
	Note string
	// Relationship is THIS CELL's relationship to the repo. Absent in the source
	// reads as RelationshipUpstream — least authority.
	Relationship Relationship
	// RelationshipStated records whether the source stated `relationship:`
	// EXPLICITLY, as opposed to falling back to the default. It is parse
	// metadata, not a topology fact: it distinguishes "the file demonstrates the
	// field" from "the file is silent and the reader supplied a value", which is
	// what the public example's shape check needs and what no derivation can
	// carry (a compiled value has no notion of absence). The drift comparison
	// therefore compares Relationship and NOT this flag.
	RelationshipStated bool
}

// App is one desk App ROLE.
//
// ID is intentionally optional and ships absent: the live App ids are operator
// configuration, not tracked inventory. An absent id means UNPINNED, never id 0
// — an empty identity matches a deleted GitHub author, which is exactly the
// failure deskkit.RoleAppLogin's ok-return exists to make impossible.
type App struct {
	Role    string
	ID      int64
	IDSet   bool
	Purpose string
}

// Human is one human identity. The list ships EMPTY; see topology.yaml's header.
type Human struct {
	Name  string
	Login string
	ID    int64
	IDSet bool
}

// Product is one product the repos serve.
type Product struct {
	Name    string
	Summary string
	Repos   []string
}

// Label is one system-emitted GitHub label with the rationale for its class.
type Label struct {
	Name string
	Why  string
}

// Topology is the whole declared source.
type Topology struct {
	Schema string
	// Cell names the cell this file is the instance OF. There is ONE topology
	// file per cell — this tree's is the assay cell's, never an
	// enterprise registry — so the name answers "whose statement is this?" for
	// every `owned` claim below it.
	//
	// It may be EMPTY, and that is deliberate backward compatibility rather than
	// a hole: a topology-v1 file written before the cell model still loads, and
	// such a file cannot state `owned` anywhere (the field did not exist), so it
	// carries no unattributable ownership claim. Parse REFUSES the one dangerous
	// combination — an `owned` repo in a file that names no cell.
	Cell  string
	Repos []Repo
	// ReleaseRepo is the repo `deskrelease` cuts from by default.
	ReleaseRepo string
	Apps        []App
	Humans      []Human
	Products    []Product
	// SystemStateLabels disqualify an open issue from becoming a placeholder:
	// each names a CLOSEABLE STATE the desk machinery emits, not a unit of work.
	SystemStateLabels []Label
	// DecisionOwedLabels mark an issue as decision-owed (the SLA escalation scope).
	DecisionOwedLabels []Label
}

// LoadFile reads and parses the declared source at path.
//
// Every error is a COULD-NOT-CHECK for the caller: this function never returns a
// partially-populated Topology alongside an error, and never returns an empty
// Topology with a nil error. A caller that treats an error as "no topology
// stated" has converted an unread file into an empty inventory.
func LoadFile(path string) (Topology, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Topology{}, fmt.Errorf("COULD-NOT-CHECK: reading the declared topology source %s: %w", path, err)
	}
	t, err := Parse(data)
	if err != nil {
		return Topology{}, fmt.Errorf("COULD-NOT-CHECK: parsing %s: %w", path, err)
	}
	return t, nil
}

// Parse parses the declared source.
func Parse(data []byte) (Topology, error) {
	root, err := parseYAML(data)
	if err != nil {
		return Topology{}, err
	}
	var t Topology

	schema, ok, err := root.str("schema")
	if err != nil {
		return Topology{}, err
	}
	if !ok {
		return Topology{}, fmt.Errorf("no `schema:` key — an unversioned topology source is refused")
	}
	if schema != Schema {
		return Topology{}, fmt.Errorf("schema %q is not the %q this reader accepts — refusing to read it on a best-effort basis", schema, Schema)
	}
	t.Schema = schema

	if t.ReleaseRepo, _, err = root.str("release_repo"); err != nil {
		return Topology{}, err
	}

	// cell — the file's own identity. Absent is legal (see Topology.Cell);
	// present-but-empty is not: a key someone bothered to write and then left
	// blank is a half-made statement, and reading it as "unstated" is how a typo
	// becomes a default.
	cell, cellPresent, err := root.str("cell")
	if err != nil {
		return Topology{}, err
	}
	if cellPresent && strings.TrimSpace(cell) == "" {
		return Topology{}, fmt.Errorf("line %d: `cell:` is stated but EMPTY — name the cell this file is the instance of, or remove the key", lineOf(root, "cell"))
	}
	t.Cell = strings.TrimSpace(cell)

	repos, ok, err := root.mapSeq("repos")
	if err != nil {
		return Topology{}, err
	}
	if !ok {
		return Topology{}, fmt.Errorf("no `repos:` key")
	}
	seen := map[string]bool{}
	// The first `owned` entry, remembered so the no-claimant refusal below can
	// name a line rather than a bare rule.
	var ownedSlug string
	var ownedLine int
	for _, r := range repos {
		var repo Repo
		slug, ok, err := r.str("slug")
		if err != nil {
			return Topology{}, err
		}
		if !ok || strings.TrimSpace(slug) == "" {
			return Topology{}, fmt.Errorf("line %d: a repos entry has no `slug:`", r.line)
		}
		if seen[slug] {
			return Topology{}, fmt.Errorf("line %d: duplicate repo %q", r.line, slug)
		}
		seen[slug] = true
		repo.Slug = slug

		vis, _, err := r.str("visibility")
		if err != nil {
			return Topology{}, err
		}
		repo.Visibility = parseVisibility(vis)

		ci, present, err := r.boolAt("ci")
		if err != nil {
			return Topology{}, err
		}
		// Unstated CI is CI-REQUIRED: an empty rollup then reads as "not reported
		// yet", never as green.
		repo.CIRequired = !present || ci

		if repo.Root, _, err = r.str("root"); err != nil {
			return Topology{}, err
		}
		if repo.Note, _, err = r.str("note"); err != nil {
			return Topology{}, err
		}
		if repo.RiskPathTriggers, _, err = r.strSeq("risk_path_triggers"); err != nil {
			return Topology{}, err
		}

		// relationship — absent defaults to upstream (least authority);
		// present-but-unrecognised is an ERROR naming the line, never a
		// best-effort read.
		rel, relPresent, err := r.str("relationship")
		if err != nil {
			return Topology{}, err
		}
		if relPresent {
			parsed, valid := parseRelationship(rel)
			if !valid {
				return Topology{}, fmt.Errorf(
					"line %d: repos[%s].relationship = %q is neither `owned` nor `upstream` — "+
						"an unrecognised relationship is refused rather than defaulted, because a "+
						"typo that silently read as `upstream` would look exactly like a stated fact",
					lineOf(r, "relationship"), slug, rel)
			}
			repo.Relationship = parsed
			repo.RelationshipStated = true
			if parsed == RelationshipOwned {
				ownedSlug, ownedLine = slug, lineOf(r, "relationship")
			}
		}
		t.Repos = append(t.Repos, repo)
	}

	// ONE-OWNER, IN-FILE. A duplicate slug is already refused above, which is the
	// mechanism: within one cell's file a repo appears once, so it carries one
	// relationship. ACROSS cells the one-owner boundary is an audit convention
	// (the assay cell audits, read-only), explicitly NOT a mechanism —
	// no global cross-cell registry exists or is wanted.
	//
	// What IS refused here: an ownership claim with no claimant. `owned` means
	// "this cell owns it", so a file that names no cell cannot state it — there
	// is nothing for the cross-cell audit to attribute the claim to.
	if t.Cell == "" && ownedSlug != "" {
		return Topology{}, fmt.Errorf(
			"line %d: repos[%s] states `relationship: owned` but the file states no `cell:` — "+
				"an ownership claim with no claimant cannot be audited against the one-owner "+
				"boundary. Name the owning cell at the top of the file, or state `upstream`",
			ownedLine, ownedSlug)
	}

	apps, _, err := root.mapSeq("apps")
	if err != nil {
		return Topology{}, err
	}
	for _, a := range apps {
		var app App
		role, ok, err := a.str("role")
		if err != nil {
			return Topology{}, err
		}
		if !ok || strings.TrimSpace(role) == "" {
			return Topology{}, fmt.Errorf("line %d: an apps entry has no `role:`", a.line)
		}
		app.Role = strings.ToLower(role)
		if app.Purpose, _, err = a.str("purpose"); err != nil {
			return Topology{}, err
		}
		if app.ID, app.IDSet, err = intAt(a, "id"); err != nil {
			return Topology{}, err
		}
		t.Apps = append(t.Apps, app)
	}

	humans, _, err := root.mapSeq("humans")
	if err != nil {
		return Topology{}, err
	}
	for _, h := range humans {
		var hu Human
		if hu.Name, _, err = h.str("name"); err != nil {
			return Topology{}, err
		}
		if hu.Login, _, err = h.str("login"); err != nil {
			return Topology{}, err
		}
		if hu.ID, hu.IDSet, err = intAt(h, "id"); err != nil {
			return Topology{}, err
		}
		t.Humans = append(t.Humans, hu)
	}

	products, _, err := root.mapSeq("products")
	if err != nil {
		return Topology{}, err
	}
	for _, pr := range products {
		var p Product
		if p.Name, _, err = pr.str("name"); err != nil {
			return Topology{}, err
		}
		if p.Summary, _, err = pr.str("summary"); err != nil {
			return Topology{}, err
		}
		if p.Repos, _, err = pr.strSeq("repos"); err != nil {
			return Topology{}, err
		}
		t.Products = append(t.Products, p)
	}

	labels := root.lookup("labels")
	if labels == nil {
		return Topology{}, fmt.Errorf("no `labels:` key — the system-state set is what this loader exists to single-source")
	}
	if t.SystemStateLabels, err = labelSeq(labels, "system_state"); err != nil {
		return Topology{}, err
	}
	if len(t.SystemStateLabels) == 0 {
		return Topology{}, fmt.Errorf("`labels.system_state` is empty — an empty exclusion set would make every system-emitted issue a placeholder")
	}
	if t.DecisionOwedLabels, err = labelSeq(labels, "decision_owed"); err != nil {
		return Topology{}, err
	}
	return t, nil
}

func labelSeq(labels *node, key string) ([]Label, error) {
	entries, ok, err := labels.mapSeq(key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("no `labels.%s` key", key)
	}
	var out []Label
	seen := map[string]bool{}
	for _, e := range entries {
		name, ok, err := e.str("name")
		if err != nil {
			return nil, err
		}
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("line %d: a labels.%s entry has no `name:`", e.line, key)
		}
		why, _, err := e.str("why")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(why) == "" {
			// The rationale is the thing a reviewer audits. A label with no
			// stated reason is unreviewable, not merely undocumented — the same
			// rule docs/publication-manifest.yaml applies to its own rows.
			return nil, fmt.Errorf("line %d: labels.%s entry %q has no `why:` — a label with no stated rationale is unreviewable", e.line, key, name)
		}
		lower := strings.ToLower(name)
		if seen[lower] {
			return nil, fmt.Errorf("line %d: duplicate label %q in labels.%s", e.line, name, key)
		}
		seen[lower] = true
		out = append(out, Label{Name: name, Why: why})
	}
	return out, nil
}

// lineOf returns the 1-based source line a key's VALUE sits on, falling back to
// the containing node's line when the key is absent. Error text that names a
// line is the difference between "fix your file" and "find it yourself".
func lineOf(n *node, key string) int {
	if c := n.lookup(key); c != nil {
		return c.line
	}
	if n != nil {
		return n.line
	}
	return 0
}

func intAt(n *node, key string) (val int64, present bool, err error) {
	s, ok, err := n.str(key)
	if err != nil || !ok {
		return 0, false, err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false, nil
	}
	var v int64
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return 0, false, fmt.Errorf("line %d: %q = %q is not an integer id", n.line, key, s)
	}
	if v <= 0 {
		// id 0 is the empty identity that matches a deleted GitHub author.
		return 0, false, fmt.Errorf("line %d: %q = %d is not a usable id (an id of 0 matches a deleted author)", n.line, key, v)
	}
	return v, true, nil
}

// ---------------------------------------------------------------------------
// Accessors. Every one of these is a pure read over an already-parsed value.
// ---------------------------------------------------------------------------

// RepoSlugs returns every stated repo slug, sorted.
func (t Topology) RepoSlugs() []string {
	out := make([]string, 0, len(t.Repos))
	for _, r := range t.Repos {
		out = append(out, r.Slug)
	}
	sort.Strings(out)
	return out
}

// Repo returns the stated topology for one repo. ok is false for a repo this
// source says nothing about — which every consumer must treat as fail-closed,
// never as "no constraints".
func (t Topology) Repo(slug string) (Repo, bool) {
	for _, r := range t.Repos {
		if r.Slug == slug {
			return r, true
		}
	}
	return Repo{}, false
}

// OwnedRepos returns the slugs this cell states it OWNS, sorted. A repo absent
// from this list is not "unowned" globally — it is unowned BY THIS CELL, which
// is the only claim one cell's file can make. There is no global cross-cell
// registry to ask, and that is a non-goal, not a gap: cross-cell references ride
// GitHub's already-global names.
func (t Topology) OwnedRepos() []string {
	out := []string{}
	for _, r := range t.Repos {
		if r.Relationship == RelationshipOwned {
			out = append(out, r.Slug)
		}
	}
	sort.Strings(out)
	return out
}

// Roots returns the repo -> default local checkout path map for the multi-repo
// board: every repo that states a `root:`.
func (t Topology) Roots() map[string]string {
	out := map[string]string{}
	for _, r := range t.Repos {
		if r.Root != "" {
			out[r.Slug] = r.Root
		}
	}
	return out
}

// RiskPathTriggersByRepo returns the repo -> ADDITIONAL risk-path-trigger map:
// every repo that states at least one trigger.
func (t Topology) RiskPathTriggersByRepo() map[string][]string {
	out := map[string][]string{}
	for _, r := range t.Repos {
		if len(r.RiskPathTriggers) > 0 {
			out[r.Slug] = append([]string(nil), r.RiskPathTriggers...)
		}
	}
	return out
}

// SystemStateLabelSet returns the system-state exclusion set as a lowercase
// lookup map — the shape both the scanner and the board match labels against.
func (t Topology) SystemStateLabelSet() map[string]bool {
	return labelSet(t.SystemStateLabels)
}

// DecisionOwedLabelSet returns the decision-owed set as a lowercase lookup map.
func (t Topology) DecisionOwedLabelSet() map[string]bool {
	return labelSet(t.DecisionOwedLabels)
}

func labelSet(in []Label) map[string]bool {
	out := make(map[string]bool, len(in))
	for _, l := range in {
		out[strings.ToLower(l.Name)] = true
	}
	return out
}

// SystemStateLabelNames returns the system-state label names, sorted.
func (t Topology) SystemStateLabelNames() []string { return labelNames(t.SystemStateLabels) }

// DecisionOwedLabelNames returns the decision-owed label names, sorted.
func (t Topology) DecisionOwedLabelNames() []string { return labelNames(t.DecisionOwedLabels) }

func labelNames(in []Label) []string {
	out := make([]string, 0, len(in))
	for _, l := range in {
		out = append(out, l.Name)
	}
	sort.Strings(out)
	return out
}

// AppRoles returns every stated App role, sorted.
func (t Topology) AppRoles() []string {
	out := make([]string, 0, len(t.Apps))
	for _, a := range t.Apps {
		out = append(out, a.Role)
	}
	sort.Strings(out)
	return out
}
