package topology

// compiled.go — the desk module's DERIVATION of topology.yaml.
//
// This is the ONLY copy of these facts in tools/desk. It exists because the desk
// tools ship as pinned standalone binaries that run from an arbitrary working
// directory: `deskpost` gating a flip has no repo root to read, and a gate that
// needs the filesystem is a gate that fails open when the filesystem is not the
// repo. Compiled-in is legal; FIVE HAND-SYNCED COPIES were the defect.
//
// IT IS NOT HAND-MAINTAINED IN THE SENSE THAT MATTERS. `TestTopologyDriftRegistry`
// (drift_test.go) reads `topology.yaml`, parses it with this package's own
// reader, and fails NAMING THE SITE when this value disagrees. Editing this file
// without editing the source — or the source without this file — is a red test,
// not a silent divergence. That is the derive-or-diff convention.
//
// PROSE IS NOT COMPILED. `note:`, `purpose:`, `summary:` and `why:` in the source
// are rationale for the reviewer, not values any gate reads, so they are left
// zero here and the drift check compares the load-bearing fields only. That is
// stated rather than implied: a reader wondering why the prose is missing should
// not have to infer that it is deliberate. The `why:` field is still MANDATORY in
// the source — Parse refuses a label without one.
//
// TO CHANGE A VALUE: edit `topology.yaml` FIRST, then mirror it here, then run
//
//	cd tools/desk && go test -run TestTopologyDriftRegistry ./...
//
// which is what CI runs.

// compiled is the derivation. Field-for-field with topology.yaml, prose omitted.
var compiled = Topology{
	Schema: Schema,
	// Cell — the file this derives from is ONE CELL'S instance, so its cell name
	// is part of the derivation. `RelationshipStated` deliberately is NOT: a
	// compiled value has no notion of a key being absent, so statedness is
	// parse metadata the diff does not compare (see Repo.RelationshipStated).
	Cell: "assay",
	Repos: []Repo{
		{
			Slug:             "example-org/tracker",
			Visibility:       VisibilityUnknown,
			CIRequired:       true,
			Root:             ".",
			RiskPathTriggers: nil,
			Relationship:     RelationshipOwned,
		},
		{
			Slug:         "example-org/example-k8s",
			Visibility:   VisibilityPublic,
			CIRequired:   true,
			Relationship: RelationshipUpstream,
			RiskPathTriggers: []string{
				"base/",
				"deploy/",
				"admin/",
				"hack/",
				".github/workflows/",
			},
		},
		{
			Slug:       "medici-finance/assay",
			Visibility: VisibilityPublic,
			CIRequired: true,
			Root:       "../assay",
			// OWNED — by the assay cell, and by no other. Every OTHER
			// cell states this repo `upstream`: a cell may file issues into the
			// toolkit but never modifies it and never runs jobs against it.
			Relationship: RelationshipOwned,
			RiskPathTriggers: []string{
				"tools/desk/internal/deskkit/",
				"tools/desk/cmd/deskpost/",
				"tools/desk/cmd/deskboard/",
				"tools/desk/cmd/desktoken/",
				"tools/desk/cmd/deskpushguard/",
				"tools/desk/cmd/writeguard/",
			},
		},
	},
	ReleaseRepo: "medici-finance/assay",
	Apps: []App{
		{Role: "desk"},
		{Role: "reviewer"},
		{Role: "verifier"},
		{Role: "worker"},
	},
	Humans: nil,
	Products: []Product{
		{Name: "assay", Repos: []string{"medici-finance/assay"}},
	},
	SystemStateLabels: []Label{
		{Name: "verify-gate"},
		{Name: "live-verify"},
		{Name: "needs-decision"},
		// review-request: the entry issueboard's hand copy was MISSING.
		{Name: "review-request"},
		// needs-human: human-decision-queue class — a human-decision item,
		// not dispatchable work. Also in DecisionOwedLabels (like
		// needs-decision) so the board shows it AWAIT/ESCALATE, not NONE.
		{Name: "needs-human"},
	},
	DecisionOwedLabels: []Label{
		{Name: "needs-decision"},
		{Name: "question"},
		{Name: "needs-human"},
	},
}

// Compiled returns the desk module's topology. It is a COPY: an accessor that
// handed out the package variable would let one caller's mutation change every
// other caller's gate inputs.
func Compiled() Topology {
	out := Topology{
		Schema:      compiled.Schema,
		Cell:        compiled.Cell,
		ReleaseRepo: compiled.ReleaseRepo,
	}
	out.Repos = make([]Repo, 0, len(compiled.Repos))
	for _, r := range compiled.Repos {
		r.RiskPathTriggers = append([]string(nil), r.RiskPathTriggers...)
		out.Repos = append(out.Repos, r)
	}
	out.Apps = append([]App(nil), compiled.Apps...)
	out.Humans = append([]Human(nil), compiled.Humans...)
	for _, p := range compiled.Products {
		p.Repos = append([]string(nil), p.Repos...)
		out.Products = append(out.Products, p)
	}
	out.SystemStateLabels = append([]Label(nil), compiled.SystemStateLabels...)
	out.DecisionOwedLabels = append([]Label(nil), compiled.DecisionOwedLabels...)
	return out
}
