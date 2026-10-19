package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/medici-finance/assay/forgeread"
)

// Explicit for deskboard, including when its caller forgot DESK_LOOP. A failed
// role read is unknown claims, never a retry through native Git's ambient login.
var roleClaimReads bool

func roleRemoteBranches(root string) ([]string, error) {
	timeout := remoteBranchTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// Resolve only the repository identity locally; deskgit rechecks the exact
	// destination against this slug before obtaining its bound role credential.
	repo := trackedGitHubRepo(root)
	if repo == "" {
		return nil, fmt.Errorf("cannot identify repository for role claim read")
	}
	cmd := exec.CommandContext(ctx, "deskgit", "heads", "--repo", repo)
	cmd.Dir = root
	killWholeProcessGroup(cmd)
	cmd.WaitDelay = time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("role claim read timed out after %s", timeout)
		}
		return nil, fmt.Errorf("deskgit heads: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	heads, err := forgeread.ParseHeadsEnvelope(stdout.Bytes(), repo)
	if err != nil {
		return nil, err
	}
	branches := make([]string, 0, len(heads))
	for _, head := range heads {
		branches = append(branches, strings.TrimPrefix(head, "refs/heads/"))
	}
	return branches, nil
}
