package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/medici-finance/assay/forgeread"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"github.com/spf13/cobra"
)

// heads is a credential-free subprocess interface: only complete advertisements
// are emitted. The existing custody and in-process transport own authentication.
func cmdHeads(args []string) (err error) {
	ac := &auditCtx{verb: "heads"}
	defer func() { ac.finalize(err) }()
	cmd := &cobra.Command{Use: "heads", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	set := cli.Declare(cmd, cli.Binding{Key: "repo", Flag: "repo", Kind: cli.String, Default: "", Usage: "expected owner/name repository"})
	var repo string
	executed := false
	cmd.RunE = func(_ *cobra.Command, _ []string) error {
		executed = true
		v, e := set.Resolve(cli.Inputs{})
		if e != nil {
			return e
		}
		repo = v.String("repo")
		return nil
	}
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return deskkit.Refused("bad heads arguments: " + err.Error())
	}
	if !executed {
		return nil
	}
	if !deskkit.ValidRepoSlug(repo) {
		return deskkit.Refused("heads requires --repo owner/name")
	}
	role, _, err := deskkit.SessionTokenRole("deskgit heads")
	if err != nil {
		return err
	}
	root, err := getwd()
	if err != nil {
		return deskkit.Unverifiable("cannot resolve heads checkout", err)
	}
	endpoint, err := deskkit.CheckoutGitRead(root, repo, role)
	if err != nil {
		return err
	}
	refs, err := gitcore.List(endpoint)
	if err != nil {
		return deskkit.Unverifiable("cannot read complete origin heads", err)
	}
	heads := []string{}
	for _, ref := range refs {
		if ref.Name().IsBranch() {
			heads = append(heads, ref.Name().String())
		}
	}
	sort.Strings(heads)
	if err := json.NewEncoder(os.Stdout).Encode(forgeread.HeadsEnvelope{Schema: forgeread.EnvelopeSchema, Repo: repo, Complete: true, Heads: heads}); err != nil {
		return fmt.Errorf("cannot emit heads: %w", err)
	}
	ac.repo = repo
	ac.detail = "complete origin heads"
	return nil
}
