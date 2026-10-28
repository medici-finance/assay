package main

import (
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/runnercmds"
	"os"
	"path/filepath"
	"strings"
)

func executeVerifier(mode string, o dispatchOpts) (err error) {
	if o.root == "" || (mode == "--check-verifier" && o.brief == "") {
		return deskkit.Refused(mode + " requires --root and check requires --brief")
	}
	if mode == "--attest-verifier" {
		role, _, err := deskkit.SessionTokenRole(toolName)
		if err != nil {
			return err
		}
		// The standing verifier desk uses the SAME pre-existing desk custody as its
		// normal dispatch. A worker session never gains a dispatcher route here.
		if role != deskkit.DispatcherRole {
			return deskkit.Refused("verifier attestation recovery requires the coordinator desk; the verifier returns its original dispatch record for recovery")
		}
	}
	var receipt deskkit.VerifierReceipt
	defer func() {
		if !o.dryRun {
			result := deskkit.ResultOK
			detail := mode + " " + receipt.EvidenceBinding()
			if err != nil {
				result = deskkit.ResultUnverifiable
				if deskkit.ExitCodeOf(err) == deskkit.ExitRefused {
					result = deskkit.ResultRefused
				}
				detail = firstLine(err.Error())
			}
			_ = deskkit.Log(deskkit.Entry{Tool: toolName, Verb: strings.TrimPrefix(mode, "--"), Result: result, Detail: detail})
		}
	}()
	if o.dryRun {
		if err := deskkit.PlanVerifierAttestation(o.root, o.brief); err != nil {
			return err
		}
		fmt.Println("PLAN: validate/recover the existing verifier run; no claim, allocation, launch or mutation")
		return nil
	}
	if mode == "--check-verifier" {
		receipt, err = deskkit.CheckVerifierAttestation(o.root, o.brief)
	} else {
		// Recovery binds a record, so the run requires one before its stamp can
		// complete: a recovery whose admission refuses leaves no admissible run.
		if err = deskkit.RequireVerifierWorld(o.root); err != nil {
			return err
		}
		receipt, err = deskkit.RecoverVerifierAttestation(o.root)
		if err == nil {
			// Recovery finishes the same prepare step, so it binds the same
			// prerequisite record; binding is idempotent for an unchanged world.
			receipt, err = deskkit.AdmitAndBindVerifierWorld(o.root, runnercmds.Commands)
		}
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(receipt)
}

var attestVerifierDispatchFn = attestVerifierDispatch

// verifierEmittedDest is the file a --prompt-file dispatch actually writes:
// --prompt-file names the caller's own file, which is never written, so the
// destination checked is its emitted sibling.
func verifierEmittedDest(promptFile string) string {
	if strings.TrimSpace(promptFile) == "" {
		return ""
	}
	return emittedPathFor(promptFile)
}

// A cross-repository assignment contains the tracking brief's private locator.
// Keep explicit prompt/packet files outside every Git tree, not merely outside
// the newly allocated source home. Stdout and the owner-only packet cache remain
// local dispatcher channels, never forge publication destinations.
func verifierPrivatePromptDestination(file string) error {
	if file == "" {
		return nil
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	if info, err := os.Lstat(abs); err == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return deskkit.Refused("cross-repository verifier prompt must be a regular local file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return deskkit.Refused("cross-repository verifier prompt and packet must be outside repository trees")
		} else if !os.IsNotExist(err) {
			return err
		}
		if filepath.Dir(dir) == dir {
			return nil
		}
	}
}

func attestVerifierDispatch(o dispatchOpts, repo, home string) (string, error) {
	brief := o.brief
	if filepath.IsAbs(brief) && vpWithin(o.root, brief) {
		var err error
		brief, err = filepath.Rel(o.root, brief)
		if err != nil {
			return "", err
		}
	}
	if filepath.IsAbs(brief) && !vpWithin(o.root, brief) {
		dl, err := resolveDeliverable(o)
		if err != nil {
			return "", err
		}
		actual, err := deskkit.OriginRepoSlug(o.claimRoot)
		if err != nil || !dl.crossRepo(o.root) || dl.trackingRepo == "" || !strings.EqualFold(actual, dl.trackingRepo) || !vpWithin(o.claimRoot, brief) {
			return "", deskkit.Refused("cross-repository verifier requires the resolved tracking repository and its brief under --claim-root")
		}
	}
	if err := deskkit.PrepareVerifierAttestation(home, repo, brief, o.model, o.tier); err != nil {
		return "", err
	}
	// Marked BEFORE the stamp is issued: if the admission below is held or
	// could not check, the retained run has a closed stamp and no record, and
	// row and Evidence admission refuse it rather than skip the recheck.
	if err := deskkit.RequireVerifierWorld(home); err != nil {
		return "", err
	}
	if _, err := deskkit.RecoverVerifierAttestation(home); err != nil {
		return "", err
	}
	// The prepare step ends by recording the world the run was admitted into;
	// landing rechecks exactly this record (example-stream/50).
	receipt, err := deskkit.AdmitAndBindVerifierWorld(home, runnercmds.Commands)
	if err != nil {
		return "", err
	}
	return "OK: pre-work stamp applied and verified; " + receipt.EvidenceBinding(), nil
}

// Replace, rather than truncate, so an existing hardlink cannot carry private
// prompt bytes into another tree. The replacement is owner-readable only.
func writeVerifierPrompt(file string, content []byte) error {
	if err := verifierPrivatePromptDestination(file); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(file), ".verifier-prompt-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(content); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, file)
}
