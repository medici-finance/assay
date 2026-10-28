package cli

import (
	"context"
	"flag"
	"strings"
	"sync"

	"github.com/medici-finance/assay/perfrec"
	"github.com/spf13/cobra"
)

var perfNames = []string{"perf-profile", "perf-session-id", "perf-agent-id", "perf-parent-agent-id", "perf-parent-invocation-id", "perf-loop-role", "perf-role", "perf-work"}

type perfKey struct{}

// BindPerformance registers only the canonical optional metadata flags. The
// typed perfrec resolver owns their meaning and named environment sources.
func BindPerformance(cmd *cobra.Command) {
	for _, name := range perfNames {
		if cmd.PersistentFlags().Lookup(name) != nil {
			continue
		}
		if name == "perf-work" {
			cmd.PersistentFlags().StringArray(name, nil, "local performance work reference (bounded JSON object; repeats replace inherited work)")
		} else {
			cmd.PersistentFlags().String(name, "", "optional local performance context (never operational authority)")
		}
	}
}

func perfSet(in *perfrec.ContextInput, name, value string) {
	v := perfrec.Value{Value: value, Present: true}
	switch name {
	case "perf-profile":
		in.Profile = v
	case "perf-session-id":
		in.SessionID = v
	case "perf-agent-id":
		in.AgentID = v
	case "perf-parent-agent-id":
		in.ParentAgentID = v
	case "perf-parent-invocation-id":
		in.ParentInvocationID = v
	case "perf-loop-role":
		in.DeskRole = v
	case "perf-role":
		in.Role = v
	case "perf-work":
		in.Work = append(in.Work, value)
		in.WorkPresent = true
	}
}

// PerformanceInput reads pflag's parsed state, never argv or a config file.
func PerformanceInput(cmd *cobra.Command) perfrec.ContextInput {
	var in perfrec.ContextInput
	root := cmd.Root()
	for _, name := range perfNames {
		f := root.PersistentFlags().Lookup(name)
		if f == nil || !f.Changed {
			continue
		}
		if name == "perf-work" {
			in.Work, _ = root.PersistentFlags().GetStringArray(name)
			in.WorkPresent = true
		} else {
			perfSet(&in, name, f.Value.String())
		}
	}
	return in
}

// PerformanceContext returns a copy-safe context attached before this handler.
func PerformanceContext(cmd *cobra.Command) (perfrec.Context, bool) {
	if cmd.Context() == nil {
		return perfrec.Context{}, false
	}
	c, ok := cmd.Context().Value(perfKey{}).(perfrec.Context)
	return c, ok
}

func performanceSelection(root *cobra.Command, sink func(perfrec.Context)) func(*cobra.Command) {
	var once sync.Once
	var selected perfrec.Context
	return func(cmd *cobra.Command) {
		once.Do(func() {
			selected = perfrec.ResolveContext(PerformanceInput(root))
			if sink != nil {
				sink(selected)
			}
		})
		parent := cmd.Context()
		if parent == nil {
			parent = context.Background()
		}
		cmd.SetContext(context.WithValue(parent, perfKey{}, selected))
	}
}

type perfFlag struct {
	in   *perfrec.ContextInput
	name string
}

func (v perfFlag) String() string     { return "" }
func (v perfFlag) Set(s string) error { perfSet(v.in, v.name, s); return nil }

// GoPerformance extends an existing Go FlagSet without replacing its parser.
// Call the returned finalizer after Parse and before the operation. Optional
// invalid metadata is handled by perfrec, so it never changes parser results.
func GoPerformance(fs *flag.FlagSet, sink func(perfrec.Context)) func() {
	in := &perfrec.ContextInput{}
	for _, name := range perfNames {
		fs.Var(perfFlag{in, name}, name, "optional local performance context")
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			if sink != nil {
				sink(perfrec.ResolveContext(*in))
			}
		})
	}
}

// ManualPerformance binds one token in a current manual parser's switch. It
// consumes only canonical metadata flags; operational options stay with owner.
func ManualPerformance(in *perfrec.ContextInput, arg string, next func() string) bool {
	name, value, equals := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
	if !strings.HasPrefix(arg, "--") {
		return false
	}
	for _, known := range perfNames {
		if name == known {
			if !equals {
				value = next()
			}
			perfSet(in, name, value)
			return true
		}
	}
	return false
}

// GoPerformanceSink is the optional sink form used by current-parser adapters.
func GoPerformanceSink(fs *flag.FlagSet, sinks ...func(perfrec.Context)) func() {
	var sink func(perfrec.Context)
	if len(sinks) > 0 {
		sink = sinks[0]
	}
	return GoPerformance(fs, sink)
}
