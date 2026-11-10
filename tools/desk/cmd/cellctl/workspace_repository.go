package main

// workspace_repository.go — cellctl's composition of cell-owned repository stores (the shared
// `workspace` module and its local adapter). cellctl owns three things here and nothing more:
//
//   - the canonical repository identity of a topology entry (forge kind, forge host, and the
//     owner/name the cell's stream-root map or remote names), folded the way the forge routes it;
//   - the legacy scope a managed store must stay physically outside of, drawn from the legacy
//     keys (CELL_REPO, CELL_ROOTS) and the cell's legacy cleanup roots, which it only reads;
//   - publication: the separately named, generation-stamped mapping (run/workspaces/stores.json),
//     written only after every requested store, and every store already published, verifies.
//
// It does not choose a transport or a credential: the caller composes the existing role
// transport and passes it in. It never writes CELL_REPO, CELL_ROOTS or cell.env, and never
// exports the mapping under a legacy key. Provisioning stays off until the operator activation
// decision for the first cell is recorded (docs/cellctl.md, "Managed repository stores"); no
// verb calls this yet.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/workspace"
	"github.com/medici-finance/assay/workspace/local"
)

// workspaceRepoRef resolves one topology entry — an owner/name slug or any remote URL shape the
// shared parser reads — to its canonical repository identity on the cell's forge. GitHub and
// GitLab both route repository paths case-insensitively, so the canonical form is lowercase and
// two spellings of one repository are one store; the forge host is part of the identity, so the
// same owner/name on two forges, or two owners' same-named repositories, never are.
func workspaceRepoRef(forge, host, spec string) (workspace.RepoRef, error) {
	switch forge {
	case "github", "gitlab":
	default:
		return workspace.RepoRef{}, fmt.Errorf("forge %q has no repository identity rule", forge)
	}
	path := strings.TrimSpace(spec)
	if strings.Contains(path, "://") || strings.Contains(path, "@") {
		owner, repo, err := deskkit.ParseRemoteRepo(path)
		if err != nil {
			return workspace.RepoRef{}, err
		}
		path = owner + "/" + repo
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	ref := workspace.RepoRef{Forge: strings.ToLower(host), Repository: strings.ToLower(path)}
	if _, err := workspace.PlanStores(workspace.CellRef{UUID: "00000000-0000-4000-8000-000000000000", Root: "/"},
		[]workspace.StoreRequest{{Repo: ref, Subject: strings.Repeat("0", 40)}}); err != nil {
		return workspace.RepoRef{}, err
	}
	return ref, nil
}

// workspaceLegacyScope is the legacy scope of one cell, read from its environment: every
// checkout CELL_REPO and CELL_ROOTS select, and every legacy cleanup root — the cell's
// worktrees, comms, tmp and managed Go cache directories, the old sanctioned worktree prefix
// and each legacy checkout's .claude/worktrees. It reads the legacy keys; it never sets them.
func workspaceLegacyScope(e *Env, cellDir string) local.Legacy {
	var l local.Legacy
	add := func(p string) {
		if p != "" {
			l.Checkouts = append(l.Checkouts, p)
			l.Cleanup = append(l.Cleanup, filepath.Join(p, ".claude", "worktrees"))
		}
	}
	add(e.Get("CELL_REPO"))
	for _, entry := range rootEntries(e.Get("CELL_ROOTS")) {
		add(entry[1])
	}
	l.Cleanup = append(l.Cleanup,
		filepath.Join(cellDir, "worktrees"),
		filepath.Join(cellDir, "comms"),
		filepath.Join(cellDir, "tmp"),
		e.GetOr("CELL_GO_CACHE_ROOT", filepath.Join(cellDir, "go-cache")),
		filepath.Join(legacyTmpBase, "tracker-*"),
	)
	return l
}

// legacyTmpBase is the parent of deskwt's compiled-in sanctioned worktree prefix (deskwt's
// tmpBaseDir): a legacy cleanup root on every POSIX host, whatever the cell.
const legacyTmpBase = "/private/tmp"

// workspaceStoreScope is what publication needs about the cell: its admitted identity, the
// machine-level registry binding that identity to one root, and its legacy scope.
type workspaceStoreScope struct {
	Cell     workspace.CellRef
	Registry string
	Legacy   local.Legacy
}

// storeOutcome is one repository's explicit result in a publication attempt.
type storeOutcome struct {
	Repo  workspace.RepoRef
	Entry workspace.StoreEntry
	Err   error
}

// publishWorkspaceStores provisions every requested store and publishes the cell's mapping at the
// generation after expected, or publishes nothing. Every repository's outcome is returned, so a
// partial failure stays explicit per repository; the mapping is written only when every new
// store and every already published one verifies, so no reader ever sees a mix of verified and
// unverified stores. A store provisioned in a failed attempt remains unpublished and is found,
// re-verified, by the next attempt.
func publishWorkspaceStores(ctx context.Context, stores local.Stores, scope workspaceStoreScope, reqs []workspace.StoreRequest, expected uint64) (workspace.StoreMapping, []storeOutcome, error) {
	if err := local.BindCell(scope.Registry, scope.Cell); err != nil {
		return workspace.StoreMapping{}, nil, err
	}
	plan, err := workspace.PlanStores(scope.Cell, reqs)
	if err != nil {
		return workspace.StoreMapping{}, nil, err
	}
	prior, err := readStoreMapping(scope.Cell)
	if err != nil {
		return workspace.StoreMapping{}, nil, err
	}
	var outcomes []storeOutcome
	var entries []workspace.StoreEntry
	var failed error
	for _, q := range plan {
		e, err := stores.Provision(ctx, scope.Cell, q, scope.Legacy)
		outcomes = append(outcomes, storeOutcome{Repo: q.Repo, Entry: e, Err: err})
		if err != nil {
			if failed == nil {
				failed = fmt.Errorf("store %s/%s: %w", q.Repo.Forge, q.Repo.Repository, err)
			}
			continue
		}
		entries = append(entries, e)
	}
	if failed != nil {
		return workspace.StoreMapping{}, outcomes, failed
	}
	// The stores already published must still hold too: a mapping is republished whole.
	if prior != nil {
		for _, s := range prior.Stores {
			if _, err := stores.Observe(ctx, scope.Cell, workspace.StoreRequest{Repo: s.Repo, Subject: s.Subject}, scope.Legacy); err != nil {
				return workspace.StoreMapping{}, outcomes, fmt.Errorf("published store %s/%s: %w", s.Repo.Forge, s.Repo.Repository, err)
			}
		}
	}
	next, err := workspace.NextMapping(prior, expected, scope.Cell, entries)
	if err != nil {
		return workspace.StoreMapping{}, outcomes, err
	}
	if err := writeStoreMapping(scope.Cell, next); err != nil {
		return workspace.StoreMapping{}, outcomes, err
	}
	return next, outcomes, nil
}

// readStoreMapping is the published mapping, nil when none has been published.
func readStoreMapping(cell workspace.CellRef) (*workspace.StoreMapping, error) {
	b, err := os.ReadFile(workspace.MappingPath(cell))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, &workspace.Refusal{Reason: workspace.ReasonEvidenceUnavailable, Field: "mapping", Msg: err.Error()}
	}
	m, err := workspace.DecodeStoreMapping(b)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// writeStoreMapping replaces the mapping atomically: a temporary file in the control directory,
// renamed over the old one only after the published generation is re-read unchanged.
func writeStoreMapping(cell workspace.CellRef, m workspace.StoreMapping) error {
	b, err := workspace.EncodeStoreMapping(m)
	if err != nil {
		return err
	}
	path := workspace.MappingPath(cell)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return &workspace.Refusal{Reason: workspace.ReasonEvidenceUnavailable, Field: "mapping", Msg: err.Error()}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".stores-*.json")
	if err != nil {
		return &workspace.Refusal{Reason: workspace.ReasonEvidenceUnavailable, Field: "mapping", Msg: err.Error()}
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.Write(b)
	if serr := tmp.Sync(); werr == nil {
		werr = serr
	}
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return &workspace.Refusal{Reason: workspace.ReasonEvidenceUnavailable, Field: "mapping", Msg: werr.Error()}
	}
	cur, err := readStoreMapping(cell)
	if err != nil {
		return err
	}
	have := uint64(0)
	if cur != nil {
		have = cur.Generation
	}
	if have != m.Generation-1 {
		return &workspace.Refusal{Reason: workspace.ReasonStaleGeneration, Field: "generation", Msg: "the mapping was published concurrently"}
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return &workspace.Refusal{Reason: workspace.ReasonEvidenceUnavailable, Field: "mapping", Msg: err.Error()}
	}
	return nil
}
