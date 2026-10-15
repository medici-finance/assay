package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// The command-tree checks of the cell-operation contract (desk-containers/15, spec/cell-op-v1.md
// §"State-changing invocations"). They read the cobra tree, so they hold in every build; the rows
// that run the binary on a terminal are in cellop_consent_tty_test.go (build tag cellopfixture).

// cellopInvocation is one state-changing invocation: a command path, and the flag whose form it
// is ("" when the command itself is annotated, so every form except --plan is state-changing).
type cellopInvocation struct {
	path string
	flag string
}

func (i cellopInvocation) String() string {
	if i.flag == "" {
		return i.path
	}
	return i.path + " --" + i.flag
}

// walkTree visits every command of root, hidden ones included.
func walkTree(root *cobra.Command, visit func(*cobra.Command)) {
	visit(root)
	for _, c := range root.Commands() {
		walkTree(c, visit)
	}
}

// eachFlag visits the flags a command declares, local and persistent.
func eachFlag(c *cobra.Command, visit func(*pflag.Flag)) {
	seen := map[string]bool{}
	for _, fs := range []*pflag.FlagSet{c.Flags(), c.PersistentFlags()} {
		fs.VisitAll(func(f *pflag.Flag) {
			if !seen[f.Name] {
				seen[f.Name] = true
				visit(f)
			}
		})
	}
}

// stateChangingInvocations is every annotated invocation in root, sorted.
func stateChangingInvocations(root *cobra.Command) []cellopInvocation {
	var out []cellopInvocation
	walkTree(root, func(c *cobra.Command) {
		if commandStateChanging(c) {
			out = append(out, cellopInvocation{path: c.CommandPath()})
		}
		eachFlag(c, func(f *pflag.Flag) {
			if flagStateChanging(f) {
				out = append(out, cellopInvocation{path: c.CommandPath(), flag: f.Name})
			}
		})
	})
	sort.Slice(out, func(a, b int) bool { return out[a].String() < out[b].String() })
	return out
}

// cellopEnumerated is the contract's enumerated list: each must be registered and annotated.
var cellopEnumerated = []cellopInvocation{
	{path: "cellctl up", flag: "apply"},
	{path: "cellctl drain"}, {path: "cellctl pause"}, {path: "cellctl resume"}, {path: "cellctl restart"},
	{path: "cellctl doctor", flag: "apply"},
	{path: "cellctl gc", flag: "run"}, {path: "cellctl gc", flag: "apply"},
	{path: "cellctl upgrade", flag: "apply"},
}

// cellopConsentNames are the flag names that must always be annotated where they appear.
var cellopConsentNames = []string{"apply", "run", "now", "foreign"}

// cellopConsentCarveOuts are the invocations exempt from the four-name rule, each with its reason.
// `scratch --apply` predates the contract: it applies a scratch-retention cleanup that defaults to
// a dry run and is driven off a terminal by desks and tests, so it is not a cell operation.
var cellopConsentCarveOuts = map[string]string{
	"cellctl scratch --apply": "scratch retention cleanup; not a cell operation (desk-containers/15 decision)",
}

func commandAt(root *cobra.Command, path string) *cobra.Command {
	var found *cobra.Command
	walkTree(root, func(c *cobra.Command) {
		if c.CommandPath() == path {
			found = c
		}
	})
	return found
}

func declaredFlag(c *cobra.Command, name string) *pflag.Flag {
	if f := c.Flags().Lookup(name); f != nil {
		return f
	}
	return c.PersistentFlags().Lookup(name)
}

// enumeratedProblems is every entry of the enumerated list that is not registered and annotated.
func enumeratedProblems(root *cobra.Command) []string {
	var out []string
	for _, inv := range cellopEnumerated {
		c := commandAt(root, inv.path)
		switch {
		case c == nil:
			out = append(out, inv.String()+": not registered")
		case inv.flag == "" && !commandStateChanging(c):
			out = append(out, inv.String()+": command not annotated "+stateChanging)
		case inv.flag != "" && declaredFlag(c, inv.flag) == nil:
			out = append(out, inv.String()+": flag not registered")
		case inv.flag != "" && !flagStateChanging(declaredFlag(c, inv.flag)):
			out = append(out, inv.String()+": flag not annotated "+stateChanging)
		}
	}
	return out
}

// consentNameProblems is every --apply/--run/--now/--foreign in root that is not annotated, on the
// flag or on its command, apart from the recorded carve-outs.
func consentNameProblems(root *cobra.Command) []string {
	var out []string
	walkTree(root, func(c *cobra.Command) {
		eachFlag(c, func(f *pflag.Flag) {
			for _, n := range cellopConsentNames {
				inv := cellopInvocation{path: c.CommandPath(), flag: f.Name}
				if f.Name != n || flagStateChanging(f) || commandStateChanging(c) {
					continue
				}
				if _, ok := cellopConsentCarveOuts[inv.String()]; !ok {
					out = append(out, inv.String()+": not annotated "+stateChanging)
				}
			}
		})
	})
	return out
}

// applyTraceViolations reads the lines the fixture appends at the apply path ("<command
// path>\t<flags given>") against root: every arrival must be an annotated invocation, and no
// --plan line may arrive at all.
func applyTraceViolations(root *cobra.Command, trace []string) []string {
	var out []string
	for _, line := range trace {
		if line == "" {
			continue
		}
		path, given, _ := strings.Cut(line, "\t")
		var flags []string
		if given != "" {
			flags = strings.Split(given, ",")
		}
		c := commandAt(root, path)
		if c == nil {
			out = append(out, line+": no such command")
			continue
		}
		ok := commandStateChanging(c)
		for _, n := range flags {
			if n == "plan" {
				ok = false
				break
			}
			if f := declaredFlag(c, n); f != nil && flagStateChanging(f) {
				ok = true
			}
		}
		if !ok {
			out = append(out, strings.TrimSpace(path+" "+given)+": reached the apply path without a "+stateChanging+" annotation")
		}
	}
	return out
}

// approvalNames are the names the contract forbids anywhere in the tree: no approval mechanism is
// built, and nothing skips the typed confirm or the re-measure.
var approvalNames = []string{"approve", "approval", "yes", "y", "assume-yes", "skip-verify"}

// approvalSurfaces is every command name or alias and every flag name or shorthand in root that
// is one of approvalNames.
func approvalSurfaces(root *cobra.Command) []string {
	forbidden := map[string]bool{}
	for _, n := range approvalNames {
		forbidden[n] = true
	}
	var out []string
	walkTree(root, func(c *cobra.Command) {
		for _, n := range append([]string{c.Name()}, c.Aliases...) {
			if forbidden[n] {
				out = append(out, "command "+c.CommandPath()+" (name or alias "+n+")")
			}
		}
		eachFlag(c, func(f *pflag.Flag) {
			if forbidden[f.Name] {
				out = append(out, c.CommandPath()+" --"+f.Name)
			}
			if forbidden[f.Shorthand] {
				out = append(out, c.CommandPath()+" -"+f.Shorthand)
			}
		})
	})
	return out
}

// No approve verb, --approval, --yes/-y, --assume-yes or --skip-verify may ever skip the typed confirm (desk-containers/15).
// regression: #2890
func TestNoApprovalSurface(t *testing.T) {
	if got := approvalSurfaces(buildRoot()); len(got) != 0 {
		t.Fatalf("the command tree has an approval or confirm-skipping surface: %v", got)
	}
	// The scan must see each kind of surface: a hidden command, an alias, a flag and a shorthand.
	bad := buildRoot()
	bad.AddCommand(&cobra.Command{Use: "approve", Hidden: true, Run: func(*cobra.Command, []string) {}})
	bad.AddCommand(&cobra.Command{Use: "ok", Aliases: []string{"approval"}, Run: func(*cobra.Command, []string) {}})
	ls := commandAt(bad, "cellctl ls")
	ls.Flags().BoolP("assume-yes", "y", false, "")
	ls.PersistentFlags().Bool("skip-verify", false, "")
	if got := approvalSurfaces(bad); len(got) != 5 {
		t.Fatalf("approvalSurfaces missed a planted surface; found %d of 5: %v", len(got), got)
	}
}

// An apply path outside cellopApply would bypass the annotation and terminal gates (desk-containers/15).
// regression: #2890
func TestOneApplyPath(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var builders []string
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "cellop.Session{") {
			builders = append(builders, p)
		}
	}
	if len(builders) != 1 || builders[0] != "cellop_cli.go" {
		t.Fatalf("cellop.Session must be built only by cellopApply in cellop_cli.go; built in %v", builders)
	}
}

// The tree checks must catch an exempted invocation and an unannotated --run, --now or --foreign (desk-containers/15).
// regression: #2890
func TestConsentTreeChecks(t *testing.T) {
	root := buildRoot()
	if got := consentNameProblems(root); len(got) != 0 {
		t.Fatalf("unannotated --apply/--run/--now/--foreign: %v", got)
	}
	if got := applyTraceViolations(root, stateChangingTraceLines(root)); len(got) != 0 {
		t.Fatalf("annotated invocations read as violations: %v", got)
	}
	// Planted: an unannotated --now on ls, and an apply reached from ls's bare form and its --plan.
	ls := commandAt(root, "cellctl ls")
	ls.Flags().Bool("now", false, "")
	if got := consentNameProblems(root); len(got) != 1 {
		t.Fatalf("consentNameProblems: want the planted --now, got %v", got)
	}
	markStateChangingFlag(ls, "now")
	trace := []string{"cellctl ls\t", "cellctl ls\tnow,plan", "cellctl ls\tnow"}
	if got := applyTraceViolations(root, trace); len(got) != 2 {
		t.Fatalf("applyTraceViolations: want the bare form and the --plan form, got %v", got)
	}
}

// stateChangingTraceLines is the trace each annotated invocation of root leaves at the apply path.
func stateChangingTraceLines(root *cobra.Command) []string {
	var out []string
	for _, inv := range stateChangingInvocations(root) {
		out = append(out, inv.path+"\t"+inv.flag)
	}
	return out
}
