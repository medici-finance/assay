package main

// context.go is `statusgen context`, the thin command line over the offline
// context resolver (statusgen/contexthandoff, statusgen/20):
//
//	statusgen context discover --snapshot DIR [--json]
//	statusgen context recheck  --snapshot DIR RECEIPT [--json]
//	statusgen context check    --snapshot DIR HANDOFF [MANIFEST] [PATCH...] [--json]
//	statusgen context dispose  RECEIPT --subject ID --outcome REF --decision D
//	                           --expected-revision REV --rationale TEXT --by ROLE --basis DIGEST [--json]
//
// The adapter opens the snapshot root and the named files and calls the
// package; every conclusion is the package's. It reads no git repository,
// forge, clock or environment variable, and runs no process. It is built on
// Cobra with a command-local Viper (docs/streams/desktools-v2/cli-contract.md
// §7): help resolves no setting, and a setting comes only from its flag or its
// declared default.
//
// Exit codes are three-state: 0 pass (complete coverage, current receipt,
// every obligation passes), 1 refuse (an obligation is refused, the receipt
// must be reconciled, or an input is invalid), 2 usage, 3 hold (could-not-check
// on an unavailable input, scoped to it, or partial coverage on either side,
// such as a query truncated at the result limit).

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/medici-finance/assay/statusgen/contexthandoff"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	ctxExitPass   = 0
	ctxExitRefuse = 1
	ctxExitUsage  = 2
	ctxExitHold   = 3

	// ctxMaxInputBytes bounds every receipt, manifest or patch file the
	// adapter reads; the snapshot has its own limits.
	ctxMaxInputBytes = 16 << 20
)

// ctxExit carries a verb's exit code out of Cobra.
type ctxExit struct{ code int }

func (e ctxExit) Error() string { return fmt.Sprintf("exit %d", e.code) }

func ctxExitWith(code int) error {
	if code == 0 {
		return nil
	}
	return ctxExit{code}
}

// runContext runs one `statusgen context` command line and returns its exit code.
func runContext(args []string, stdout, stderr io.Writer) int {
	root := newContextCmd()
	root.SetIn(strings.NewReader(""))
	root.SetOut(stdout)
	root.SetErr(stderr)
	if args == nil {
		args = []string{}
	}
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return ctxExitPass
	}
	var ce ctxExit
	if errors.As(err, &ce) {
		return ce.code
	}
	fmt.Fprintf(stderr, "statusgen context: %v\nRun 'statusgen context --help' for usage.\n", err)
	return ctxExitUsage
}

func newContextCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "statusgen context",
		Short: "offline future-state resolver over an explicit snapshot",
		Long: `statusgen context — offline future-state resolver (statusgen/20)

discover  enumerate owners and consumers of the declaring brief's target and
          emit a context-handoff receipt
recheck   compare a prior receipt with a new snapshot: preserved, reconcile or hold
check     check a handoff's amendment obligations (and manifest, patches) against
          the snapshot
dispose   record a continue, adapt or stop decision on a product-binding
          mismatch in a context-handoff-v3 receipt

Every verb reads only --snapshot DIR (dispose reads none) and the files it
names. A snapshot with no product section gives an architecture-only receipt:
product alignment is reported unchecked, never aligned. A snapshot with one
gives a context-handoff-v3 receipt that assesses each brief's declared
outcome-revision against the current approved product target; an unbound or
undiscovered binding is unknown, and a mismatch is a projection, never a veto.
Exit: 0 pass, 1 refuse/reconcile/invalid input, 2 usage, 3 hold (could-not-check).
See docs/context-handoff.md.`,
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown verb %q (want one of: discover, recheck, check, dispose)", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(cmd.ErrOrStderr(), cmd.UsageString())
			return ctxExit{ctxExitUsage}
		},
	}
	root.AddCommand(newContextDiscoverCmd(), newContextRecheckCmd(), newContextCheckCmd(), newContextDisposeCmd())
	root.AddCommand(&cobra.Command{
		Use:                cobra.ShellCompRequestCmd,
		Aliases:            []string{cobra.ShellCompNoDescRequestCmd},
		Hidden:             true,
		DisableFlagParsing: true,
		Args: func(cmd *cobra.Command, _ []string) error {
			return fmt.Errorf("unknown verb %q (want one of: discover, recheck, check, dispose)", cmd.CalledAs())
		},
		Run: func(*cobra.Command, []string) {},
	})
	return root
}

// ctxSettings are one verb's resolved settings.
type ctxSettings struct {
	snapshot string
	json     bool
}

// ctxDeclare registers the flags every verb shares.
func ctxDeclare(cmd *cobra.Command) {
	cmd.Flags().String("snapshot", "", "snapshot root directory (snapshot.json plus sources/<id>/); required")
	cmd.Flags().Bool("json", false, "emit the result as JSON")
}

// ctxResolve resolves the declared settings through a command-local Viper:
// a changed flag wins over the declared default, and nothing else is read.
func ctxResolve(cmd *cobra.Command) (ctxSettings, error) {
	v := viper.NewWithOptions(viper.KeyDelimiter("::"))
	v.SetDefault("snapshot", "")
	v.SetDefault("json", false)
	for _, name := range []string{"snapshot", "json"} {
		if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
			v.Set(name, f.Value.String())
		}
	}
	s := ctxSettings{snapshot: v.GetString("snapshot"), json: v.GetBool("json")}
	if s.snapshot == "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "statusgen context %s: --snapshot DIR is required\n", cmd.Name())
		return s, ctxExit{ctxExitUsage}
	}
	return s, nil
}

// ctxLoadSnapshot opens the snapshot root and hands its fs.FS to the package.
// os.Root keeps every read inside DIR, whatever a symlink in it points at.
func ctxLoadSnapshot(cmd *cobra.Command, dir string) (*contexthandoff.Snapshot, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, ctxFail(cmd, "snapshot: %v", err)
	}
	defer root.Close()
	s, err := contexthandoff.LoadSnapshot(root.FS(), contexthandoff.DefaultLimits)
	if err != nil {
		return nil, ctxFail(cmd, "%v", err)
	}
	return s, nil
}

// ctxReadFile reads one named input, bounded.
func ctxReadFile(cmd *cobra.Command, path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ctxFail(cmd, "%v", err)
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, ctxMaxInputBytes+1))
	if err != nil {
		return nil, ctxFail(cmd, "%s: %v", path, err)
	}
	if len(b) > ctxMaxInputBytes {
		return nil, ctxFail(cmd, "%s: larger than %d bytes", path, ctxMaxInputBytes)
	}
	return b, nil
}

// ctxFail reports an invalid input and exits 1.
func ctxFail(cmd *cobra.Command, format string, a ...any) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "statusgen context %s: %s\n", cmd.Name(), fmt.Sprintf(format, a...))
	return ctxExit{ctxExitRefuse}
}

func ctxReadHandoff(cmd *cobra.Command, path string) (*contexthandoff.Handoff, error) {
	b, err := ctxReadFile(cmd, path)
	if err != nil {
		return nil, err
	}
	h, err := contexthandoff.DecodeHandoff(b)
	if err != nil {
		return nil, ctxFail(cmd, "%s: %v", path, err)
	}
	return h, nil
}

func ctxWriteJSON(w io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}

// ctxCoverageLines prints the coverage a receipt states: architecture only,
// or architecture plus the product-binding assessment of a v3 receipt.
func ctxCoverageLines(w io.Writer, h *contexthandoff.Handoff) {
	cov := h.Discovery.Coverage
	fmt.Fprintf(w, "coverage: %s\n", cov.State)
	for _, u := range cov.Unavailable {
		fmt.Fprintf(w, "  could-not-check: %s: %s\n", u.Source, u.Reason)
	}
	pr := h.Product
	if pr == nil {
		fmt.Fprintf(w, "scope: %s\n", contexthandoff.ArchitectureOnly)
		return
	}
	fmt.Fprintf(w, "scope: %s\n", contexthandoff.ProductAssessed)
	for _, t := range pr.Targets {
		fmt.Fprintf(w, "product target %s@%s: %s (%s)\n", t.Outcome, t.Revision, t.Approval, t.Reference)
	}
	for _, b := range pr.Bindings {
		fmt.Fprintf(w, "product binding %s: %s outcome %s declared %s selected %s: %s\n", b.Subject, b.State, ctxOpt(b.Outcome), ctxOpt(b.Declared), ctxOpt(b.Selected), b.Reason)
	}
	for _, a := range pr.Assessments {
		fmt.Fprintf(w, "product %s %s %s: %s (%s) bound %s current %s: %s\n", a.Relation, a.Subject, ctxOpt(a.Outcome), a.Result, a.Coverage, ctxOpt(a.Bound), ctxOpt(a.Current), a.Reason)
	}
	for _, d := range pr.Dispositions {
		fmt.Fprintf(w, "product disposition %s %s: %s against %s by %s on %s: %s\n", d.Subject, d.Outcome, d.Decision, d.ExpectedRevision, d.DecidedBy, d.Basis, d.Rationale)
	}
}

func ctxOpt(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

func newContextDiscoverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover --snapshot DIR [--json]",
		Short: "enumerate owners and consumers and emit a context-handoff receipt",
		Args:  cobra.NoArgs,
	}
	ctxDeclare(cmd)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		set, err := ctxResolve(cmd)
		if err != nil {
			return err
		}
		s, err := ctxLoadSnapshot(cmd, set.snapshot)
		if err != nil {
			return err
		}
		h, err := contexthandoff.Discover(s)
		if err != nil {
			return ctxFail(cmd, "%v", err)
		}
		out := cmd.OutOrStdout()
		if set.json {
			fmt.Fprintf(out, "%s\n", h.Canonical())
		} else {
			fmt.Fprintf(out, "receipt %s declared by %s\n", h.Digest(), h.DeclaredBy)
			ctxCoverageLines(out, h)
			for _, im := range h.Impacts {
				fmt.Fprintf(out, "impact %s: %s (%s %s at %s): %s\n", im.Subject, im.Disposition, im.Relationship.Basis, im.Relationship.Edge, im.Relationship.Locator, im.Reason)
			}
			for _, w := range h.ActiveWork {
				fmt.Fprintf(out, "active work %s: %s: %s\n", w.Subject, w.Claim, w.Treatment)
			}
			fmt.Fprintf(out, "next act: %s\n", h.Readiness.NextAct)
			for _, p := range h.Readiness.Prerequisites {
				fmt.Fprintf(out, "  prerequisite: %s\n", p)
			}
		}
		if h.Discovery.Coverage.State != "complete" {
			return ctxExitWith(ctxExitHold)
		}
		return nil
	}
	return cmd
}

func newContextRecheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recheck --snapshot DIR RECEIPT [--json]",
		Short: "compare a prior receipt with a new snapshot",
		Args:  cobra.ExactArgs(1),
	}
	ctxDeclare(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		set, err := ctxResolve(cmd)
		if err != nil {
			return err
		}
		prior, err := ctxReadHandoff(cmd, args[0])
		if err != nil {
			return err
		}
		s, err := ctxLoadSnapshot(cmd, set.snapshot)
		if err != nil {
			return err
		}
		rc, err := contexthandoff.Recheck(prior, s)
		if err != nil {
			return ctxFail(cmd, "%v", err)
		}
		if rc.Findings == nil {
			rc.Findings = []contexthandoff.Finding{}
		}
		outcome, code := "current", ctxExitPass
		switch {
		case rc.Reconcile():
			outcome, code = "reconcile", ctxExitRefuse
		case rc.Hold():
			outcome, code = "hold", ctxExitHold
		}
		out := cmd.OutOrStdout()
		if set.json {
			if err := ctxWriteJSON(out, struct {
				Outcome  string                   `json:"outcome"`
				Prior    string                   `json:"prior"`
				Receipt  *contexthandoff.Handoff  `json:"receipt"`
				Findings []contexthandoff.Finding `json:"findings"`
			}{outcome, prior.Digest(), rc.Receipt, rc.Findings}); err != nil {
				return ctxFail(cmd, "%v", err)
			}
		} else {
			fmt.Fprintf(out, "recheck %s: %s\n", prior.Digest(), outcome)
			for _, f := range rc.Findings {
				fmt.Fprintf(out, "%s: %s: %s\n", f.Class, f.Scope, f.Reason)
			}
			fmt.Fprintf(out, "receipt %s\n", rc.Receipt.Digest())
			ctxCoverageLines(out, rc.Receipt)
		}
		return ctxExitWith(code)
	}
	return cmd
}

func newContextCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check --snapshot DIR HANDOFF [MANIFEST] [PATCH...] [--json]",
		Short: "check a handoff's amendment obligations against the snapshot",
		Long: `check reads HANDOFF (a context-handoff record) and any further files: one
amendment-manifest-v1 record, recognised by its schema, and candidate patches,
keyed by their digest. It refuses an obligation that is unbound, does not apply
exactly to its base revision, rewrites populated Evidence or a dated source
account, rebinds an active claim without an acknowledged adjustment or
cancellation, patches a delivered brief, or contradicts an approved rival.`,
		Args: cobra.MinimumNArgs(1),
	}
	ctxDeclare(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		set, err := ctxResolve(cmd)
		if err != nil {
			return err
		}
		h, err := ctxReadHandoff(cmd, args[0])
		if err != nil {
			return err
		}
		var manifest *contexthandoff.Manifest
		patches := map[string][]byte{}
		for _, p := range args[1:] {
			b, err := ctxReadFile(cmd, p)
			if err != nil {
				return err
			}
			var probe struct {
				Schema string `json:"schema"`
			}
			if json.Unmarshal(b, &probe) == nil && probe.Schema == contexthandoff.ManifestSchema {
				if manifest != nil {
					return ctxFail(cmd, "%s: a second manifest; check takes at most one", p)
				}
				if manifest, err = contexthandoff.DecodeManifest(b); err != nil {
					return ctxFail(cmd, "%s: %v", p, err)
				}
				continue
			}
			patches[contexthandoff.PatchDigest(b)] = b
		}
		s, err := ctxLoadSnapshot(cmd, set.snapshot)
		if err != nil {
			return err
		}
		rep, err := contexthandoff.Check(h, manifest, patches, s)
		if err != nil {
			return ctxFail(cmd, "%v", err)
		}
		out := cmd.OutOrStdout()
		if set.json {
			if err := ctxWriteJSON(out, rep); err != nil {
				return ctxFail(cmd, "%v", err)
			}
		} else {
			fmt.Fprintf(out, "check %s: %s (current: %t)\n", rep.Receipt, rep.Outcome, rep.Current)
			for _, f := range rep.Findings {
				fmt.Fprintf(out, "%s: %s: %s\n", f.Class, f.Scope, f.Reason)
			}
			for _, v := range rep.Verdicts {
				rule := ""
				if v.Rule != "" {
					rule = " [" + v.Rule + "]"
				}
				fmt.Fprintf(out, "obligation %s on %s: %s%s: %s\n", v.Obligation, v.Subject, v.Outcome, rule, v.Reason)
			}
			ctxCoverageLines(out, h)
		}
		switch rep.Outcome {
		case contexthandoff.OutcomeRefuse:
			return ctxExitWith(ctxExitRefuse)
		case contexthandoff.OutcomeHold:
			return ctxExitWith(ctxExitHold)
		}
		return nil
	}
	return cmd
}

func newContextDisposeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dispose RECEIPT --subject ID --outcome REF --decision continue|adapt|stop --expected-revision REV --rationale TEXT --by ROLE --basis DIGEST [--json]",
		Short: "record a continue, adapt or stop decision on a product-binding mismatch",
		Long: `dispose reads RECEIPT (a context-handoff-v3 receipt) and writes it back to
stdout with one product disposition appended. --basis is the digest of the
receipt the decision was made on and --expected-revision the current approved
product target it was made against: a disposition prepared on another receipt,
or against a target that is no longer current (a stale rebinding), is refused.
--by must be the receipt's deciding role. A disposition changes no lifecycle,
impact disposition or next act; stop is a recorded decision, not a cancellation.
Exit: 0 recorded, 1 refused or invalid input, 2 usage.`,
		Args: cobra.ExactArgs(1),
	}
	f := cmd.Flags()
	for _, name := range []string{"subject", "outcome", "decision", "expected-revision", "rationale", "by", "basis"} {
		f.String(name, "", name+" of the disposition; required")
	}
	f.Bool("json", false, "emit the updated receipt as canonical JSON (default: a summary)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		v := viper.NewWithOptions(viper.KeyDelimiter("::"))
		vals := map[string]string{}
		for _, name := range []string{"subject", "outcome", "decision", "expected-revision", "rationale", "by", "basis"} {
			v.SetDefault(name, "")
			if fl := cmd.Flags().Lookup(name); fl != nil && fl.Changed {
				v.Set(name, fl.Value.String())
			}
			if vals[name] = v.GetString(name); vals[name] == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "statusgen context dispose: --%s is required\n", name)
				return ctxExit{ctxExitUsage}
			}
		}
		asJSON := false
		if fl := cmd.Flags().Lookup("json"); fl != nil && fl.Changed {
			asJSON = fl.Value.String() == "true"
		}
		h, err := ctxReadHandoff(cmd, args[0])
		if err != nil {
			return err
		}
		out, err := contexthandoff.Dispose(h, contexthandoff.ProductDisposition{
			Subject: vals["subject"], Outcome: vals["outcome"], Decision: vals["decision"],
			ExpectedRevision: vals["expected-revision"], Rationale: vals["rationale"],
			DecidedBy: vals["by"], Basis: vals["basis"],
		})
		if err != nil {
			return ctxFail(cmd, "%v", err)
		}
		w := cmd.OutOrStdout()
		if asJSON {
			fmt.Fprintf(w, "%s\n", out.Canonical())
			return nil
		}
		fmt.Fprintf(w, "receipt %s (was %s)\n", out.Digest(), h.Digest())
		ctxCoverageLines(w, out)
		return nil
	}
	return cmd
}
