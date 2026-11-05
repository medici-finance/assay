package main

import (
	"fmt"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/spf13/cobra"
)

func recordCommand() *cobra.Command {
	var r deskkit.UsageRecord
	var claim string
	counts := map[string]*string{}
	cmd := &cobra.Command{Use: "record [dispatch_ref]", Short: "Record only counts supplied by the completion notification", Args: cobra.MaximumNArgs(1)}
	f := cmd.Flags()
	f.StringVar(&claim, "claim-key", "", "Resolve the latest dispatched event for this claim")
	f.StringVar(&r.Repo, "repo", "", "Repository owner/name")
	f.StringVar(&r.Item, "item", "", "Brief stream/NN or issue-N")
	f.StringVar(&r.Tier, "tier", "", "Dispatch tier")
	for _, k := range []string{"input-tokens", "output-tokens", "total-tokens", "cost-micros", "wall-seconds"} {
		counts[k] = f.String(k, "", "Reported nonnegative integer or could-not-check; omit if unknown")
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if (len(args) == 1) == (claim != "") {
			return deskkit.Refused("supply exactly one dispatch_ref or --claim-key")
		}
		if claim != "" {
			d, e := deskkit.LatestUsageDispatch(claim)
			if e != nil {
				return e
			}
			if d.Repo != r.Repo {
				return deskkit.Refused("usage repo differs from dispatched record")
			}
			r.DispatchRef = *d.DispatchRef
		} else {
			r.DispatchRef = args[0]
		}
		ptrs := map[string]**deskkit.VitalField{"input-tokens": &r.InputTokens, "output-tokens": &r.OutputTokens, "total-tokens": &r.TotalTokens, "cost-micros": &r.CostMicros, "wall-seconds": &r.WallSeconds}
		for k, p := range ptrs {
			if f.Changed(k) {
				v, e := deskkit.ParseUsageCount(*counts[k])
				if e != nil {
					return e
				}
				*p = v
			}
		}
		if e := deskkit.AppendUsageRecord(r); e != nil {
			return e
		}
		_, e := fmt.Fprintln(cmd.OutOrStdout(), "recorded usage counts")
		return e
	}
	return cmd
}
