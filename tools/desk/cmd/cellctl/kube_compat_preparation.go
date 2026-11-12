package main

import (
	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/kubecompat/preparation"
)

func compatPrepareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "compat-prepare", Short: "plan or prepare a cold Claude desk volume locally",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{noEcho: "1", leadingArgsKey: "0"},
	}
	settings := declare(cmd,
		bBool("prepare", "explicitly create cold descriptor/history; default is read-only plan"),
		bBool("readback", "read validated unused preparation bindings without changing them"),
		bDef("work", "absolute retained work directory", "/work"),
		bStr("cell", "approved cell name"), bStr("role", "desk role"),
		bStr("agent-version", "exact installed Claude package version"),
		bStr("model", "explicit approved versioned Claude model ID"),
		bStr("agent", "preinstalled executable path"),
		bStr("version-manifest", "installed Claude package manifest path"),
		bStr("instructions", "approved instructions path; default is the image-baked role skill"),
		bStr("workdir", "preinstalled work checkout path"),
	)
	for _, flag := range []string{"cell", "role", "agent-version", "model"} {
		if err := cmd.MarkFlagRequired(flag); err != nil {
			panic(err)
		}
	}
	cmd.MarkFlagsMutuallyExclusive("prepare", "readback")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		v := resolve(settings, nil)
		options := preparation.Options{
			Prepare: v.Bool("prepare"), Readback: v.Bool("readback"),
			Work: v.String("work"), Cell: v.String("cell"), Role: v.String("role"),
			AgentVersion: v.String("agent-version"), Model: v.String("model"),
			Agent: v.String("agent"), VersionManifest: v.String("version-manifest"),
			Instructions: v.String("instructions"), Workdir: v.String("workdir"),
		}
		exitWith(preparation.Run(cmd.Context(), options, cmd.OutOrStdout(), cmd.ErrOrStderr()))
		return nil
	}
	return cmd
}
