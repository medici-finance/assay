package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"os"
	"path/filepath"
	"strings"
)

func cmdVerifierAttestation(mode string, args []string) (err error) {
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	root := fs.String("root", "", "existing detached verifier worktree (required)")
	brief := fs.String("brief", "", "source brief (required for --check-verifier)")
	dry := fs.Bool("dry-run", false, "validate arguments without any mutation")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(err.Error())
	}
	if fs.NArg() != 0 || *root == "" || (mode == "--check-verifier" && *brief == "") {
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
		if !*dry {
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
	if *dry {
		if err := deskkit.PlanVerifierAttestation(*root, *brief); err != nil {
			return err
		}
		fmt.Println("PLAN: validate/recover the existing verifier run; no claim, allocation, launch or mutation")
		return nil
	}
	if mode == "--check-verifier" {
		receipt, err = deskkit.CheckVerifierAttestation(*root, *brief)
	} else {
		receipt, err = deskkit.RecoverVerifierAttestation(*root)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(receipt)
}

var attestVerifierDispatchFn = attestVerifierDispatch

func attestVerifierDispatch(o dispatchOpts, repo, home string) (string, error) {
	brief := o.brief
	if filepath.IsAbs(brief) {
		var err error
		brief, err = filepath.Rel(o.root, brief)
		if err != nil {
			return "", err
		}
	}
	if err := deskkit.PrepareVerifierAttestation(home, repo, brief, o.model, o.tier); err != nil {
		return "", err
	}
	receipt, err := deskkit.RecoverVerifierAttestation(home)
	if err != nil {
		return "", err
	}
	return "OK: pre-work stamp applied and verified; " + receipt.EvidenceBinding(), nil
}
