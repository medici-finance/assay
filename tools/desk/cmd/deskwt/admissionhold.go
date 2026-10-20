package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// verifierAdmissionHold is how long prune holds a worktree after a verifier run was
// admitted in it. An admitted home is clean and, once origin/main moves past the commit it
// was dispatched at, merged — exactly the shape the merge and tracked-clean gates remove —
// yet its Evidence still has to land FROM it (the admission record in its admin dir is read
// back at Evidence time). The record carries no release or expiry of its own, so its age is
// the only signal: inside this window the home is held; past it the run is treated as over
// and the ordinary gates decide. `deskwt remove` (the owner's explicit release) is not gated.
const verifierAdmissionHold = 24 * time.Hour

// admissionEvidence is one worktree's admission record as the sweep found it.
type admissionEvidence struct {
	mtime time.Time
	err   error // non-nil: a record may exist but could not be read — hold, could-not-check
}

// verifierAdmissions maps resolved worktree root → that worktree's admission record, read
// ONCE per sweep (one ReadDir of the admin registry, one stat per entry), keyed the way
// lockFileMtimes keys so lookups compare equal to the roots the sweep iterates. A worktree
// with no record is absent from the map. Unlike lockFileMtimes this is fail-CLOSED: the
// answer gates a deletion, so a registry that cannot be read, or a record that cannot be
// tied to its worktree, is returned as an error and the caller holds every candidate.
func (g *pathGuard) verifierAdmissions() (map[string]admissionEvidence, error) {
	out := make(map[string]admissionEvidence)
	if g.commonDir == "" {
		return nil, deskkit.Unverifiable("the shared git-common-dir is not resolved", nil)
	}
	adminRoot := filepath.Join(g.commonDir, "worktrees")
	entries, err := os.ReadDir(adminRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil // no linked worktree is registered, so none was admitted
	}
	if err != nil {
		return nil, deskkit.Unverifiable("cannot read the worktree admin registry", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		admin := filepath.Join(adminRoot, e.Name())
		fi, serr := os.Stat(filepath.Join(admin, deskkit.VerifierAttestationRecord))
		if errors.Is(serr, fs.ErrNotExist) {
			continue // never admitted, or released
		}
		ev := admissionEvidence{}
		if serr != nil {
			ev.err = serr
		} else {
			ev.mtime = fi.ModTime()
		}
		gd, rerr := os.ReadFile(filepath.Join(admin, "gitdir"))
		dotGit := strings.TrimSpace(string(gd))
		if rerr != nil || dotGit == "" {
			return nil, deskkit.Unverifiable("admin entry "+e.Name()+" holds a verifier admission record but names no worktree", rerr)
		}
		out[resolvePath(filepath.Dir(dotGit))] = ev
	}
	return out, nil
}

// admissionHoldReason returns the hold reason for rt, or "" when no admitted verifier run
// holds it (no record, or a record older than verifierAdmissionHold). A record dated in the
// future is held: its age cannot be proven past the window.
func admissionHoldReason(adm map[string]admissionEvidence, rt string, now time.Time) string {
	ev, ok := adm[rt]
	if !ok {
		return ""
	}
	if ev.err != nil {
		return "could-not-check: verifier admission record unreadable: " + errText(ev.err)
	}
	age := now.Sub(ev.mtime)
	if age >= verifierAdmissionHold {
		return ""
	}
	when := "admitted " + roundAge(age) + " ago"
	if age < 0 {
		when = "admission record dated in the future"
	}
	return "active verifier admission (" + when + "; held for " + verifierAdmissionHold.String() +
		" after admission, or until `deskwt remove` releases it)"
}
