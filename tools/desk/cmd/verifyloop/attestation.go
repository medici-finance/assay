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
