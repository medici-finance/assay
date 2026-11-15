package main

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A local flip has no outward admission/mint. Only an existing verifier session
// may obtain its own read credential, and only after the origin is provable.
func prepareFlipApproval(root string) error {
	if !acceptanceForge {
		return nil
	}
	role, _, err := deskkit.SessionTokenRole("deskevidence flip approval")
	if err != nil {
		return err
	}
	if role != "verifier" {
		return deskkit.Refused("flip approval requires the verifier session's own forge-read authority")
	}
	origin, err := gitcore.OriginAuthority(root)
	if err != nil {
		return deskkit.Unverifiable("flip approval requires proven repository authority", err)
	}
	repo, err := deskkit.RemoteRepoSlug(origin)
	if err != nil || !deskkit.IsAllowedRepo(repo) {
		return deskkit.Refused("flip approval repository is not authorized")
	}
	return mintTokenFn(repo)
}

type closureAudit string

const (
	auditLint     closureAudit = "--lint"
	auditBrief    closureAudit = "brief"
	auditClosure  closureAudit = "verifyclosure"
	auditFlip     closureAudit = "verifyflip"
	auditVerdict  closureAudit = "evidenceverdict"
	auditDecision closureAudit = "--decision-gate"
)

// statusgenAudit is the only statusgen child constructor. Its closed vocabulary
// contains closure audits, never row execution. Forge custody is scoped to this
// child and to explicit approval permission; Git configuration remains observable.
func statusgenAudit(ctx context.Context, kind closureAudit, root string, args ...string) *exec.Cmd {
	var argv []string
	switch kind {
	case auditLint, auditDecision:
		argv = []string{"--root", root, string(kind)}
	case auditBrief, auditClosure, auditFlip, auditVerdict:
		argv = []string{string(kind), "--root", root}
	default:
		cmd := exec.CommandContext(ctx, "statusgen")
		cmd.Err = deskkit.Unverifiable("unsupported statusgen closure audit", nil)
		return cmd
	}
	argv = append(argv, args...)
	permitted := acceptanceForge && kind != auditDecision
	if permitted {
		argv = append(argv, "--acceptance-forge")
	}
	cmd := exec.CommandContext(ctx, "statusgen", argv...)
	cmd.Dir = os.TempDir()
	var env []string
	for _, kv := range cmd.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(key) {
		case "GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN",
			"GITLAB_TOKEN", "CI_JOB_TOKEN", "CELLCTL_GH_AMBIENT", "GH_HOST", "GH_REPO", "GH_DEBUG":
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = env
	if permitted {
		if strings.TrimSpace(ghToken) == "" {
			cmd.Err = deskkit.Unverifiable("approval audit requires a minted verifier credential; ambient forge credentials are not used", nil)
			return cmd
		}
		if ghTokenForge != deskkit.ForgeGitHub {
			cmd.Err = deskkit.Unverifiable("approval audit requires GitHub verifier custody; an unknown or foreign-forge credential is not forwarded", nil)
			return cmd
		}
		cmd.Env = append(env, "GH_TOKEN="+ghToken, "GH_HOST=github.com")
	} else {
		// A child outside verify-desk may otherwise ask gh's stored login even
		// after token variables are removed. The noncredential refusal marker
		// follows statusgen's own ghGuard convention and cannot authenticate.
		cmd.Env = append(env, "GH_TOKEN=stored-login-refused", "GH_HOST=github.com")
	}
	return cmd
}
