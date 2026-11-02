package main

// briefv4.go — the brief-v4 typed-reference surface (milestone-support/04).
//
// brief-v4 is brief-v3 plus the negotiated reference syntax of spec/brief-v1.md
// §3.7–§3.8:
//
//   - the `brief:` definition id is the full colon-only prefixed form
//     <cell>:<repo>:<stream>:b-<NN> (no revision, no slash);
//   - `version:` (the authored Task/Verify revision) and `id:` (the uuid) are
//     REQUIRED — a reference may pin the revision, so it is never defaulted;
//   - `reader-requirements:` is REQUIRED and must hold back a reader that
//     advertises only the legacy schemas (the shared preflight reads a record
//     with no envelope as carrying no requirement);
//   - `requires-milestones:` carries exact versioned milestone start
//     requirements, interpreted by the shared statusgen/workref resolver;
//   - `depends:` / `unblocks:` may be written <stream>:b-<NN>[@<version>] (or a
//     legacy compatibility form) and NEVER name a milestone.
//
// Parsing NORMALIZES onto the compatibility forms every existing reader keys on:
// bf.Brief becomes the internal fullId <cell>:<repo>:<stream>:<NN> and each
// depends/unblocks entry becomes <stream>/<NN>. The prefixed spelling, the
// revision selector and the milestone list ride alongside in their own fields,
// so no legacy reader is ever handed new syntax it would have to guess at.
//
// Nothing here mints a uuid, a task or a claim: an alias or a revision selector
// binds an existing brief at the selected source snapshot or stays unknown.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/workref"
)

// BriefRefSelector is one brief-v4 depends:/unblocks: entry as written, kept
// beside the normalized <stream>/<NN> key the legacy graph readers consume.
type BriefRefSelector struct {
	Field    string            // "depends" or "unblocks"
	Spelling string            // exactly as written
	Ref      workref.Reference // parsed identity; Cell/Repo empty when elided
	Key      string            // normalized same-root <stream>/<NN>
}

// briefV4KeyFor normalizes a parsed brief reference to the <stream>/<NN> key.
func briefV4KeyFor(ref workref.Reference) string { return ref.Stream + "/" + ref.Number }

// parseBriefV4Keys parses and normalizes the brief-v4 surface. It runs after
// parseBriefV2Keys (brief-v4 is in the v2 family) and is purely syntactic; tree
// resolution lives in checkBriefV4Semantics and the snapshot binder.
func parseBriefV4Keys(data map[string]any, bf *BriefFile, addBad func(string, ...any)) {
	// version: REQUIRED (parseBriefV2Keys defaults an absent one to 1, which a
	// pinned selector must never silently bind to).
	if _, ok := data["version"]; !ok {
		addBad("version is required under brief-v4 (the authored revision a reference may pin is never defaulted)")
	}
	if strings.TrimSpace(bf.ID) == "" {
		addBad("id is required under brief-v4 (every alias and revision selector normalizes to this one uuid; none mints another)")
	}

	// reader-requirements: REQUIRED. It is the only thing that makes a reader
	// advertising just the legacy schemas refuse this record: the shared preflight
	// reads a record with no envelope as carrying no requirement.
	if v, ok := data["reader-requirements"]; !ok {
		addBad("reader-requirements is required under brief-v4 (the envelope is what makes a reader that does not advertise brief-v4 syntax refuse the record; without it a legacy reader sees no requirement)")
	} else if msg := briefV4EnvelopeProblem(v); msg != "" {
		addBad("reader-requirements: %s", msg)
	}

	// brief: the full colon-only prefixed definition id.
	if bf.Brief != "" { // briefid:raw shape check of the authored id, not a key
		ref, err := workref.Parse(bf.Brief, workref.TypedBrief) // briefid:raw parsing the authored definition id itself
		switch {
		case err != nil:
			addBad("brief %q is not a typed brief id: %v", bf.Brief, err)
		case strings.Contains(bf.Brief, "/") || ref.Cell == "" || !strings.HasPrefix(lastColonSegment(bf.Brief), "b-") || ref.Revision != 0: // briefid:raw shape of the authored id
			addBad("brief %q must be the full colon-only definition id <cell>:<repo>:<stream>:b-<NN> (no slash, no @<version>; the revision is the separate version: key)", bf.Brief)
		default:
			bf.TypedID = bf.Brief
			bf.Brief = ref.Compatibility()
		}
	}

	// requires-milestones: exact versioned milestone references, colon-only.
	if v, ok := data["requires-milestones"]; ok {
		list, err := stringList(v)
		if err != nil {
			addBad("requires-milestones: %v", err)
		} else {
			seen := map[string]bool{}
			for _, m := range list {
				if strings.Contains(m, "/") {
					addBad("requires-milestones: %q must be colon-only (<stream>:m-<NN>@<version>, <repo>:<stream>:m-<NN>@<version> or <cell>:<repo>:<stream>:m-<NN>@<version>)", m)
					continue
				}
				if _, err := workref.Parse(m, workref.TypedMilestone); err != nil {
					addBad("requires-milestones: %q is not an exact versioned milestone reference: %v", m, err)
					continue
				}
				if seen[m] {
					addBad("requires-milestones: %q is listed twice", m)
					continue
				}
				seen[m] = true
				bf.RequiresMilestones = append(bf.RequiresMilestones, m)
			}
			if bf.RequiresMilestones == nil {
				bf.RequiresMilestones = []string{} // present-and-empty: explicitly none
			}
		}
	}

	// depends/unblocks: brief references only; normalized, each brief once.
	bf.Depends = normalizeBriefV4Edges("depends", bf.Depends, bf, addBad)
	bf.Unblocks = normalizeBriefV4Edges("unblocks", bf.Unblocks, bf, addBad)
}

// legacyBriefReaderAdvertisement is the advertisement a reader that predates
// brief-v4 gives the shared preflight: the operations statusgen's own
// advertisement lists (syntax and requirements preflight, the same operation
// set the desk-tools readers advertise), with the syntax schemas limited to the
// shared legacy registry. Deriving it from statusgenReaderCaps keeps the
// operation set in step with the real advertisement instead of a stand-in.
func legacyBriefReaderAdvertisement() readercaps.Advertisement {
	adv := statusgenReaderCaps()
	ops := make([]readercaps.Operation, 0, len(adv.Operations))
	for _, op := range adv.Operations {
		if op.Operation == "syntax" {
			op.Schemas = briefdoc.SchemaVersions()
		}
		ops = append(ops, op)
	}
	adv.Operations = ops
	adv.Provider, adv.Version = "legacy-brief-reader", "legacy"
	return adv
}

// briefV4EnvelopeProblem decodes a brief-v4 reader-requirements envelope and
// checks it does its one job: a reader advertising only the shared legacy
// schemas (and the preflight every real reader advertises) must assess it
// unsupported. It returns "" when it does.
func briefV4EnvelopeProblem(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	req, err := readercaps.DecodeRequirements(raw)
	if err != nil {
		return err.Error()
	}
	if r := readercaps.Assess(req, readercaps.Observation{State: readercaps.Current, Advertisement: legacyBriefReaderAdvertisement()}); r.State != readercaps.Unsupported {
		return "must require syntax support for " + briefSchemaV4 + " so a reader advertising only " + strings.Join(briefdoc.SchemaVersions(), ", ") + " refuses the record (a legacy reader was assessed " + string(r.State) + ")"
	}
	return ""
}

// normalizeBriefV4Edges parses each written edge with the shared resolver,
// refuses milestone references and duplicate aliases of one brief, records the
// selector and returns the normalized <stream>/<NN> list.
func normalizeBriefV4Edges(field string, in []string, bf *BriefFile, addBad func(string, ...any)) []string {
	if in == nil {
		return nil
	}
	out := make([]string, 0, len(in))
	firstSpelling := map[string]string{}
	for _, raw := range in {
		ref, err := workref.Parse(raw, workref.Typed)
		if err != nil {
			addBad("%s: %q is not a brief reference: %v", field, raw, err)
			continue
		}
		if ref.Kind != workref.Brief {
			addBad("%s: %q names a milestone — %s names briefs only; a milestone start requirement belongs in requires-milestones", field, raw, field)
			continue
		}
		key := briefV4KeyFor(ref)
		if prev, dup := firstSpelling[key]; dup {
			addBad("%s: %q and %q name the same brief %s — legacy and prefixed aliases are read once, never as two edges", field, prev, raw, key)
			continue
		}
		firstSpelling[key] = raw
		out = append(out, key)
		bf.RefSelectors = append(bf.RefSelectors, BriefRefSelector{Field: field, Spelling: raw, Ref: ref, Key: key})
	}
	return out
}

func lastColonSegment(s string) string {
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// briefV4Registry builds the workref root registry from graph-repos.yaml: each
// alias is a root under the registry's own cell (where a brief id names it) and,
// when the entry names another cell, under that cell too. No bindings are
// supplied — resolution normalizes roots only and never mints a key.
func briefV4Registry(reg *graphRepos) (workref.Registry, error) {
	var roots []workref.Root
	aliases := make([]string, 0, len(reg.Aliases))
	for a := range reg.Aliases {
		aliases = append(aliases, a)
	}
	sort.Strings(aliases)
	for _, a := range aliases {
		roots = append(roots, workref.Root{Cell: reg.Cell, Repo: a})
		if c := reg.Aliases[a].Cell; c != "" && c != reg.Cell {
			roots = append(roots, workref.Root{Cell: c, Repo: a})
		}
	}
	return workref.NewRegistry(roots, nil)
}

// checkBriefV4Semantics runs the tree-dependent brief-v4 rules: milestone
// references resolve to registered roots with no duplicate or conflicting
// revision, and every qualified depends/unblocks selector names this root (a
// cross-root brief edge is a gates:/feathers: edge, never depends/unblocks).
// The v2-family semantic pass has already checked the id against the path.
func checkBriefV4Semantics(add, notice func(string, ...any), path string, bf *BriefFile, reg *graphRepos) {
	if reg == nil || bf.TypedID == "" {
		return // the v2 pass reported the missing registry; parse reported the id
	}
	cell, alias, _, _, ok := parseBriefV2ID(bf.Brief) // briefid:raw compat id was derived at parse
	if !ok {
		return
	}
	wr, err := briefV4Registry(reg)
	if err != nil {
		add("%s: docs/streams/graph-repos.yaml cannot serve as a typed-reference registry: %v", path, err)
		return
	}
	declaring := &workref.Context{Cell: cell, Repo: alias}

	revisionOf := map[string]uint64{}
	spellingOf := map[string]string{}
	for _, m := range bf.RequiresMilestones {
		res, err := workref.Resolve(m, declaring, wr, workref.TypedMilestone)
		if err != nil {
			add("%s: requires-milestones %q does not resolve: %v", path, m, err)
			continue
		}
		if prev, dup := revisionOf[res.LogicalKey]; dup {
			if prev == res.Revision {
				add("%s: requires-milestones %q and %q name the same milestone version %s@%d", path, spellingOf[res.LogicalKey], m, res.LogicalKey, res.Revision)
			} else {
				add("%s: requires-milestones %q and %q are conflicting revision requirements on %s (@%d vs @%d) — surface the conflict, never pick one", path, spellingOf[res.LogicalKey], m, res.LogicalKey, prev, res.Revision)
			}
			continue
		}
		revisionOf[res.LogicalKey] = res.Revision
		spellingOf[res.LogicalKey] = m
	}
	if len(bf.RequiresMilestones) > 0 {
		notice("%s: brief %s requires-milestones %s — milestone acceptance is not evaluated by this reader; the brief is held could-not-check, never offered as ready", path, bf.TypedID, strings.Join(bf.RequiresMilestones, ", "))
	}

	for _, sel := range bf.RefSelectors {
		if sel.Ref.Cell == "" && sel.Ref.Repo == "" {
			continue // elided: this root by construction
		}
		res, err := workref.Resolve(sel.Spelling, declaring, wr, workref.Typed)
		if err != nil {
			add("%s: %s %q does not resolve: %v", path, sel.Field, sel.Spelling, err)
			continue
		}
		own, err := workref.Resolve(sel.Ref.Stream+":b-"+sel.Ref.Number, declaring, wr, workref.TypedBrief)
		if err != nil {
			add("%s: %s %q cannot be compared with this brief's own root: %v", path, sel.Field, sel.Spelling, err)
			continue
		}
		if res.Cell != own.Cell || res.Repo != own.Repo {
			add("%s: %s %q names brief work in another root (%s:%s) — %s is same-root; declare a cross-root brief edge in gates:/feathers:", path, sel.Field, sel.Spelling, res.Cell, res.Repo, sel.Field)
		}
	}
}

// briefSnapshotEntry is the observed authored revision of one brief in the
// selected source snapshot. Version 0 means the record carries no authored
// revision (brief-v1).
type briefSnapshotEntry struct {
	Version int
}

// briefSnapshot maps a same-root <stream>/<NN> key to its observed entry.
type briefSnapshot map[string]briefSnapshotEntry

// briefBinding is the outcome of binding one selector against a snapshot.
// Bound=false is UNKNOWN — the requested revision is not available in this
// snapshot — and is never a substitution of the revision that is.
type briefBinding struct {
	Bound   bool
	Key     string
	Version int    // the bound (observed) revision when Bound
	Reason  string // why it is unknown, when !Bound
}

// bindBriefSelector binds a selector to the selected source snapshot. An
// unversioned selector binds the observed revision (recorded, so execution and
// assessment cite it); a pinned selector binds only an exactly matching
// revision. It never mints a key, uuid or task: the key is the target's own.
func bindBriefSelector(sel BriefRefSelector, snap briefSnapshot) briefBinding {
	entry, ok := snap[sel.Key]
	if !ok {
		return briefBinding{Key: sel.Key, Reason: "brief " + sel.Key + " is not in the selected source snapshot"}
	}
	if sel.Ref.Revision == 0 {
		return briefBinding{Bound: true, Key: sel.Key, Version: entry.Version}
	}
	if entry.Version == 0 {
		return briefBinding{Key: sel.Key, Reason: fmt.Sprintf("brief %s records no authored revision, so revision %d is unavailable", sel.Key, sel.Ref.Revision)}
	}
	if uint64(entry.Version) != sel.Ref.Revision {
		return briefBinding{Key: sel.Key, Reason: fmt.Sprintf("revision %d of brief %s is unavailable in the selected source snapshot (it carries revision %d, which is never substituted)", sel.Ref.Revision, sel.Key, entry.Version)}
	}
	return briefBinding{Bound: true, Key: sel.Key, Version: entry.Version}
}

// briefV4PendingBinding is one pinned selector awaiting the post-scan bind.
type briefV4PendingBinding struct {
	path, root string
	row        *Brief
	sel        BriefRefSelector
}

// briefV4HeldEdges returns the extra eligibility gates a brief-v4 row carries:
// one per milestone start requirement. Their refs match no tree reference form,
// so the evaluator reports them could-not-check and HOLDS the brief — a
// requested gate is never silently dropped by a reader that cannot evaluate it.
func briefV4HeldEdges(bf *BriefFile) []GraphEdge {
	var out []GraphEdge
	for _, m := range bf.RequiresMilestones {
		out = append(out, GraphEdge{Ref: m, Type: "behavioural-gate", Reason: "requires-milestones: milestone acceptance is not evaluated by this reader"})
	}
	return out
}

// bindBriefV4Selectors binds every pinned selector after the whole scan has
// observed each root's snapshot. An unavailable revision is a NOTICE (unknown)
// and, on a depends: edge, an extra could-not-check gate on the row, so the
// dependent brief is held rather than satisfied by a different revision.
func bindBriefV4Selectors(notice func(string, ...any), pending []briefV4PendingBinding, snaps map[string]briefSnapshot) {
	for _, p := range pending {
		b := bindBriefSelector(p.sel, snaps[p.root])
		if b.Bound {
			continue
		}
		notice("%s: %s %q is unknown: %s", p.path, p.sel.Field, p.sel.Spelling, b.Reason)
		if p.sel.Field == "depends" && p.row != nil {
			p.row.Gates = append(p.row.Gates, GraphEdge{Ref: p.sel.Spelling, Type: "build-dep", Reason: "pinned brief revision is unknown: " + b.Reason})
		}
	}
}
