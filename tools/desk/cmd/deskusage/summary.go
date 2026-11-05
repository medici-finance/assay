package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/spf13/cobra"
)

type countSummary struct {
	Measured      int      `json:"measured"`
	Sum           int64    `json:"sum"`
	Median        *float64 `json:"median"`
	Null          int      `json:"null"`
	CouldNotCheck int      `json:"could_not_check"`
	values        []int64
}
type usageGroup struct {
	Group   string                   `json:"group"`
	Records int                      `json:"records"`
	Counts  map[string]*countSummary `json:"counts"`
}

func summaryCommand() *cobra.Command {
	var by string
	var asJSON bool
	cmd := &cobra.Command{Use: "summary", Short: "Aggregate by item or tier; never prints individual runs", Args: cobra.NoArgs}
	cmd.Flags().StringVar(&by, "by", "item", "item or tier")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON aggregates")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if by != "item" && by != "tier" {
			return deskkit.Refused("summary --by must be item or tier")
		}
		rows, e := deskkit.ReadUsageRecords()
		if e != nil {
			return deskkit.Unverifiable("cannot read usage store", e)
		}
		groups := map[string]*usageGroup{}
		for _, r := range rows {
			key := r.Item
			if by == "tier" {
				key = r.Tier
			}
			g := groups[key]
			if g == nil {
				g = &usageGroup{Group: key, Counts: map[string]*countSummary{}}
				groups[key] = g
			}
			g.Records++
			for k, v := range r.Counts() {
				s := g.Counts[k]
				if s == nil {
					s = &countSummary{}
					g.Counts[k] = s
				}
				if v == nil {
					s.Null++
				} else if v.CouldNotCheck {
					s.CouldNotCheck++
				} else {
					n := v.Value.(int64)
					if n > 0 && s.Sum > int64(^uint64(0)>>1)-n {
						return deskkit.Unverifiable("usage aggregate exceeds int64", nil)
					}
					s.Measured++
					s.Sum += n
					s.values = append(s.values, n)
				}
			}
		}
		out := make([]*usageGroup, 0, len(groups))
		for _, g := range groups {
			for _, s := range g.Counts {
				if len(s.values) > 0 {
					sort.Slice(s.values, func(i, j int) bool { return s.values[i] < s.values[j] })
					n := len(s.values)
					m := float64(s.values[n/2])
					if n%2 == 0 {
						m = float64(s.values[n/2-1])/2 + float64(s.values[n/2])/2
					}
					s.Median = &m
				}
			}
			out = append(out, g)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Group < out[j].Group })
		if asJSON {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
		}
		for _, g := range out {
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %d records\n", g.Group, g.Records)
			keys := make([]string, 0, len(g.Counts))
			for k := range g.Counts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				s := g.Counts[k]
				median := "unknown"
				if s.Median != nil {
					median = fmt.Sprint(*s.Median)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s measured=%d sum=%d median=%s null=%d could-not-check=%d\n", k, s.Measured, s.Sum, median, s.Null, s.CouldNotCheck)
			}
		}
		return nil
	}
	return cmd
}
