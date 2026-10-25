package main

import (
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
	"path/filepath"
	"sync"
)

type verifierRun struct {
	home    string
	receipt deskkit.VerifierReceipt
	cleanup func()
}
type verifierRuns struct {
	sync.Mutex
	runs map[string]verifierRun
}

// attestRun is injectable at the forge boundary in offline adapter tests. The
// production default always issues/reads the shared dispatcher-owned record.
func (v *VerifyLoop) attestRun(it loopengine.Item, home, model string) (deskkit.VerifierReceipt, error) {
	if v.Attest != nil {
		receipt, err := v.Attest(it, home, model)
		return queuedVerifierReceipt(it, receipt, err)
	}
	if model != "" {
		if err := checkQueuedAssociation(it); err != nil {
			return deskkit.VerifierReceipt{}, err
		}
	}
	brief := it.BriefPath
	sourceRepo := it.Payload["repo"]
	if it.Payload["source_repo"] != "" {
		sourceRepo = it.Payload["source_repo"]
		brief = filepath.Join(it.Payload["root"], it.BriefPath)
	}
	if model != "" {
		repo := sourceRepo
		if repo == "" {
			var err error
			repo, err = deskkit.OriginRepoSlug(home)
			if err != nil {
				return deskkit.VerifierReceipt{}, err
			}
		}
		if repo == "" {
			return deskkit.VerifierReceipt{}, fmt.Errorf("native verifier requires an exact repository in the item")
		}
		tier := it.ExecTier
		if tier == "" {
			tier = "any"
		}
		if err := deskkit.PrepareVerifierAttestation(home, repo, brief, model, tier); err != nil {
			return deskkit.VerifierReceipt{}, err
		}
		receipt, err := deskkit.RecoverVerifierAttestation(home)
		if err != nil {
			return receipt, err
		}
	}
	receipt, err := deskkit.CheckVerifierAttestation(home, it.BriefPath)
	return queuedVerifierReceipt(it, receipt, err)
}

func queuedVerifierReceipt(it loopengine.Item, receipt deskkit.VerifierReceipt, err error) (deskkit.VerifierReceipt, error) {
	if err == nil && (receipt.Binding.Source != it.TargetSHA || receipt.Binding.Brief != it.BriefPath) {
		err = fmt.Errorf("verifier admission differs from queued source or brief")
	}
	if err == nil && receipt.Binding.Version == 2 {
		b := receipt.Binding
		if b.Repo != it.Payload["repo"] || b.SourceRepo != it.Payload["source_repo"] || b.TrackingSource != it.Payload["tracking_sha"] {
			err = fmt.Errorf("verifier admission differs from queued tracking/source association")
		}
	} else if err == nil && it.Payload["source_repo"] != "" {
		err = fmt.Errorf("cross-repository queue item has no cross-repository attestation")
	}
	return receipt, err
}
func (v *VerifyLoop) rememberRun(it loopengine.Item, home string, r deskkit.VerifierReceipt, cleanup func()) {
	v.attested.Lock()
	defer v.attested.Unlock()
	if v.attested.runs == nil {
		v.attested.runs = map[string]verifierRun{}
	}
	v.attested.runs[it.ID] = verifierRun{home: home, receipt: r, cleanup: cleanup}
}
func (v *VerifyLoop) takeRun(it loopengine.Item) (verifierRun, bool) {
	v.attested.Lock()
	defer v.attested.Unlock()
	r, ok := v.attested.runs[it.ID]
	delete(v.attested.runs, it.ID)
	return r, ok
}

// worldGate binds each queued run to its prerequisite record (desk-tools/50).
// The root's record comes from that root's envelope preflight; each item's is
// admitted on top of it at selection, rechecked at dispatch, bound into the
// receipt, and rechecked again at landing. A nil gate on the loop is the
// legacy path: no record is required or consumed.
type worldGate struct {
	mu sync.Mutex
	// root returns the admitted record for an absolute stream root, or nil.
	root   func(abs string) *deskkit.RunWorld
	items  map[string]deskkit.RunWorld
	probes deskkit.PreflightProbes
}

func newWorldGate() *worldGate { return &worldGate{root: admittedRootWorld} }

// admittedRoots holds every root record this process's preflight admitted.
var admittedRoots struct {
	sync.Mutex
	m map[string]deskkit.RunWorld
}

// recordRootWorld stores (or, for a held root, forgets) root's admitted record.
func recordRootWorld(root string, w *deskkit.RunWorld) {
	abs := absRoot(root)
	admittedRoots.Lock()
	defer admittedRoots.Unlock()
	if admittedRoots.m == nil {
		admittedRoots.m = map[string]deskkit.RunWorld{}
	}
	if w == nil {
		delete(admittedRoots.m, abs)
		return
	}
	admittedRoots.m[abs] = *w
}

func admittedRootWorld(abs string) *deskkit.RunWorld {
	admittedRoots.Lock()
	defer admittedRoots.Unlock()
	if w, ok := admittedRoots.m[abs]; ok {
		return &w
	}
	return nil
}

func absRoot(root string) string {
	if root == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return root
}

// admitItems admits every still-dispatchable item's record. An item whose
// prerequisites are missing or contradict their pins is held as could-not-check
// with the admission's exact reason, so no row is spent on it; the other items
// (and the other roots) keep their coverage.
func (g *worldGate) admitItems(v *VerifyLoop, items []loopengine.Item) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.items = map[string]deskkit.RunWorld{}
	for i := range items {
		it := &items[i]
		if payloadValue(*it, "could_not_check") != "" || payloadValue(*it, "awaiting_merge") != "" {
			continue
		}
		if it.Payload == nil {
			it.Payload = map[string]string{}
		}
		root, repo := payloadValue(*it, "root"), payloadValue(*it, "repo")
		if root == "" {
			root = v.Root
		}
		if repo == "" {
			repo = v.RootRepo
		}
		base := g.root(absRoot(root))
		if base == nil {
			it.Payload["could_not_check"] = "environment could-not-check: no admitted prerequisite record for root " + absRoot(root)
			continue
		}
		spec, err := itemWorldSpec(*it, root)
		if err != nil {
			it.Payload["could_not_check"] = "environment could-not-check: " + oneLine(err)
			continue
		}
		spec.Repo, spec.Base = repo, base
		adm := deskkit.AdmitWorld(spec, g.probes)
		if !adm.Ready() {
			it.Payload["could_not_check"] = adm.Reason()
			continue
		}
		g.items[it.ID] = adm.World
	}
}

func (g *worldGate) item(id string) (deskkit.RunWorld, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	w, ok := g.items[id]
	return w, ok
}

// landWorld is landing's consumption of the record the run was dispatched
// with: the admission read back now must carry the same record, and that
// record must still hold. Either failing refuses before any Evidence is
// written or any bug is filed — the result describes a world that is gone.
func (v *VerifyLoop) landWorld(dispatched, now deskkit.VerifierReceipt) error {
	if dispatched.World == nil {
		if v.World != nil {
			return deskkit.Refused("verifier result has no admitted prerequisite record")
		}
		return nil
	}
	if now.World != nil && now.World.Digest() != dispatched.World.Digest() {
		return deskkit.Refused("verifier result prerequisite record changed since dispatch")
	}
	var probes deskkit.PreflightProbes
	if v.World != nil {
		probes = v.World.probes
	}
	return dispatched.World.Recheck(probes)
}
