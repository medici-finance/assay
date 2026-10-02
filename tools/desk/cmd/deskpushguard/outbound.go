package main

// outbound.go — the push path of the outbound-write check (desktools-v2/10).
//
// A `git push` never crosses a Forge, so the checking decorator every desk verb writes
// through cannot see it. This hook runs on EVERY push from a checkout that installed it —
// deskpr's, a shepherd's detached update push, a hand-typed one — and calls the same
// deskkit.OutboundCheckPush deskpr's own pre-push scan calls, so one function decides what a
// pushed branch name, commit message or added line may publish.

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// EnvScanOverride is deskkit.EnvPushGuardScanOverride (see there): an audited override
// reason carried into the hook.
const EnvScanOverride = deskkit.EnvPushGuardScanOverride

// outboundRefusal is one ref the outbound check refused.
type outboundRefusal struct {
	branch string
	msg    string
}

// isZeroSHA reports a deletion line's local side (all zeros) or an absent one.
func isZeroSHA(sha string) bool {
	return strings.Trim(sha, "0") == ""
}

// checkOutbound runs the outbound-write check over each pushed ref, measured from
// refs/remotes/<remoteName>/main. repo is the derived target ("" when it could not be
// derived — the public layers then run, as for any target whose visibility is not stated).
func checkOutbound(stderr io.Writer, remoteName, repo string, refs []refLine) (refused []outboundRefusal, unchecked []uncheckedRef) {
	restore := deskkit.SetOutboundNoticeWriter(stderr)
	defer restore()

	reason := strings.TrimSpace(os.Getenv(EnvScanOverride))
	if reason != "" {
		if err := deskkit.ValidateScanOverride(reason); err != nil {
			for _, ref := range refs {
				refused = append(refused, outboundRefusal{branch: ref.branch,
					msg: EnvScanOverride + " is set but not a usable override reason: " + err.Error()})
			}
			auditOutbound(stderr, refused)
			return refused, nil
		}
	}
	deskkit.SetOutboundContext(deskkit.OutboundContext{Tool: "deskpushguard", Verb: "pre-push", OverrideReason: reason})

	role := ""
	if r, _, err := deskkit.SessionTokenRole("deskpushguard"); err == nil {
		role = r
	}
	base := "refs/remotes/" + remoteName + "/main"
	for _, ref := range refs {
		if isZeroSHA(ref.localSHA) {
			continue // a deletion publishes nothing
		}
		err := deskkit.OutboundCheckPush(deskkit.OutboundPush{
			Dir: "", Repo: repo, Base: base, Head: ref.localSHA, Branch: ref.branch, Role: role,
		})
		switch {
		case err == nil:
		case deskkit.IsRefused(err):
			refused = append(refused, outboundRefusal{branch: ref.branch, msg: err.Error()})
		default:
			unchecked = append(unchecked, uncheckedRef{branch: ref.branch,
				reason: fmt.Sprintf("the outbound-write check could not read the pushed range against %s (%v)", base, err)})
		}
	}
	if len(refused) > 0 {
		auditOutbound(stderr, refused)
	}
	return refused, unchecked
}

// auditOutbound records the refused refs. The row names the branch only — never the refusal
// message, which quotes the matched text.
func auditOutbound(stderr io.Writer, refused []outboundRefusal) {
	parts := make([]string, len(refused))
	for i, o := range refused {
		parts[i] = o.branch
	}
	if err := deskkit.Log(deskkit.Entry{
		Tool:   "deskpushguard",
		Verb:   "pre-push",
		Result: deskkit.ResultRefused,
		Detail: "refused push: the outbound-write check refused " + strings.Join(parts, ", "),
	}); err != nil {
		fmt.Fprintf(stderr, "deskpushguard: audit log error: %v\n", err)
	}
}
