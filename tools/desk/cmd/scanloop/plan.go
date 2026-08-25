package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// plan.go — the read-only surface. It prints the inbound queue exactly as the drain would see it:
// surface, item, lane, age, claim state — plus the monitor's arming coverage, the trust gate's
// three-state tally, and this consumer's own unread surfaces.
//
// It deliberately does NOT run the monitor. A monitor run ADVANCES the per-repo baselines, so a
// plan that polled would consume the very events it was asked to report and the next `run` would
// find an empty surface. `plan` reads the state directory (arming) and takes its events from a
// captured poll (--inbound), which is what the standing window already has in hand.

// planOptions is the shared flag set of both subcommands, so `plan` and `run` cannot drift on how
// they resolve the root, the scope, the monitor or the window.
type planOptions struct {
	root       string
	stateDir   string
	monitor    string
	inbound    string
	worktrees  string
	window     time.Duration
	nowStr     string
	offline    bool
	dryRun     bool
	scanPR     int
	scanBranch string
	prCreated  string
}

func (o *planOptions) bind(fs *flag.FlagSet, withRun bool) {
	fs.StringVar(&o.root, "root", ".", "repo root the scan is rooted at and placeholder state is read from")
	fs.StringVar(&o.stateDir, "state-dir", "", "the inbound monitor's per-repo state dir (default: $"+EnvMonitorStateDir+")")
	fs.StringVar(&o.monitor, "monitor", "", "explicit path to the inbound monitor script (default: search the plugin trees)")
	fs.StringVar(&o.inbound, "inbound", "", "file of captured monitor output (or - for stdin) supplying this pass's events")
	fs.StringVar(&o.nowStr, "now", "", "RFC3339 instant to age the queue against (default: wall clock)")
	fs.DurationVar(&o.window, "coalesce-window", DefaultCoalesceWindow, "max age of an open scan PR that may still absorb this batch")
	fs.IntVar(&o.scanPR, "scan-pr", 0, "this session's open scan PR number, if one is open")
	fs.StringVar(&o.scanBranch, "scan-branch", "", "the open scan PR's branch")
	fs.StringVar(&o.prCreated, "scan-pr-created", "", "the open scan PR's RFC3339 createdAt — an unread age never coalesces")
	if withRun {
		fs.StringVar(&o.worktrees, "worktree-base", "", "ABSOLUTE dir the isolated scan worktrees are cut under (default: the parent of --root)")
		fs.BoolVar(&o.dryRun, "dry-run", false, "print every lane step without running it")
		fs.BoolVar(&o.offline, "offline", false, "do not run the monitor or the trust probe; take events from --inbound only")
	}
}

func (o *planOptions) now() (time.Time, error) {
	if strings.TrimSpace(o.nowStr) == "" {
		return time.Now().UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, o.nowStr)
	if err != nil {
		return time.Time{}, deskkit.Refused("scanloop: --now is not RFC3339: " + err.Error())
	}
	return t.UTC(), nil
}

func (o *planOptions) resolvedStateDir() string {
	if strings.TrimSpace(o.stateDir) != "" {
		return o.stateDir
	}
	if v := strings.TrimSpace(os.Getenv(EnvMonitorStateDir)); v != "" {
		return v
	}
	tmp := os.Getenv("TMPDIR")
	if tmp == "" {
		tmp = "/tmp"
	}
	return filepath.Join(tmp, "assay-inbound-monitor")
}

func (o *planOptions) openScanPR() *OpenScanPR {
	if o.scanPR <= 0 {
		return nil
	}
	pr := &OpenScanPR{Number: o.scanPR, Branch: o.scanBranch}
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(o.prCreated)); err == nil {
		pr.CreatedAt = t.UTC()
	}
	return pr
}

// cmdPlan is the read-only queue print.
func cmdPlan(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o planOptions
	o.bind(fs, false)
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("scanloop plan: bad flags: " + err.Error())
	}
	now, err := o.now()
	if err != nil {
		return err
	}

	if err := deskkit.ScanScopeError(); err != nil {
		return err
	}
	scope := deskkit.ScanRepos()

	fmt.Fprintf(stdout, "scanloop plan — intake-desk drain consumer\n")
	fmt.Fprintf(stdout, "scan scope: %d repo(s) — %s\n", len(scope), strings.Join(scope, ", "))

	// --- monitor arming coverage ------------------------------------------------
	stateDir := o.resolvedStateDir()
	state, serr := ReadMonitorState(stateDir, scope)
	if serr != nil {
		return serr
	}
	fmt.Fprintf(stdout, "\nMONITOR: state dir %s — %s\n", state.Dir, armingLine(state))
	if len(state.Unseeded) > 0 {
		fmt.Fprintf(stdout, "  UNSEEDED (blind until the next arm): %s\n", strings.Join(state.Unseeded, ", "))
	}
	if len(state.Foreign) > 0 {
		fmt.Fprintf(stdout, "  baselines outside the current scan scope: %s\n", strings.Join(state.Foreign, ", "))
	}
	if scriptPath, ferr := FindMonitorScript(o.root, o.monitor); ferr == nil {
		fmt.Fprintf(stdout, "  poller: %s (wrapped, never copied)\n", scriptPath)
	} else {
		fmt.Fprintf(stdout, "  poller: NOT FOUND — %s\n", ferr.Error())
	}

	// --- the events this pass has in hand ---------------------------------------
	report := &MonitorReport{}
	if strings.TrimSpace(o.inbound) != "" {
		raw, rerr := readInput(o.inbound)
		if rerr != nil {
			return deskkit.Unverifiable("scanloop plan: cannot read the captured monitor output", rerr)
		}
		report = ParseMonitorOutput(string(raw))
	}
	fmt.Fprintf(stdout, "\nINBOUND (from %s): %d event(s)\n", inboundSource(o.inbound), len(report.Inbound))
	for _, d := range report.Degraded {
		fmt.Fprintf(stdout, "  DEGRADED: %s\n", d)
	}
	for _, b := range report.Bursts {
		fmt.Fprintf(stdout, "  BURST (items suppressed, not enumerable this pass): %s\n", b)
	}
	for _, u := range report.Unparsed {
		fmt.Fprintf(stdout, "  UNPARSED monitor line (surfaced, never discarded): %s\n", u)
	}

	// --- the queue, computed by the SAME code the drain uses -----------------------
	// plan does not re-derive the queue with a second copy of the rules: it runs the drain's own
	// SelectQueue against the captured poll. Two implementations of one queue is exactly the
	// prose-vs-binary drift this consumer exists to end, and it would drift here first.
	//
	// The trust probe is deliberately UNWIRED: plan opens no network read of its own, so every
	// item reports COULD-NOT-CHECK rather than a guess. The counts still print, because a gate
	// that was not evaluated must be visible as not-evaluated.
	loop := &ScanLoop{
		Root:    o.root,
		Scope:   scope,
		Policy:  CoalescePolicy{Window: o.window},
		Now:     func() time.Time { return now },
		Monitor: func() (*MonitorReport, error) { return report, nil },
	}
	if _, qerr := loop.SelectQueue(); qerr != nil {
		return qerr
	}
	adm := loop.Admissions()
	admitted, quarantined, unknown := AdmissionCounts(adm)
	fmt.Fprintf(stdout, "\nTRUST GATE (applied BEFORE queueing): %d admitted · %d quarantined · %d could-not-check\n",
		admitted, quarantined, unknown)
	fmt.Fprintf(stdout, "  blessing authority: %s · trusted logins configured: %d\n",
		blessAuthority(), len(deskkit.TrustedLogins()))

	// --- the queue ----------------------------------------------------------------
	claims := loadClaims()
	fmt.Fprintf(stdout, "\nQUEUE — %-28s %-8s %-16s %-10s %s\n", "ITEM", "AGE", "LANE", "TIER", "CLAIM")
	if len(report.Inbound) == 0 {
		fmt.Fprintf(stdout, "  (no events in hand — that is NOT the same as an empty inbound surface)\n")
	}
	for _, a := range adm {
		it := loop.toItem(a)
		tier, terr := loop.TierPolicy(it)
		tierName := "?"
		if terr == nil {
			tierName = tier.String()
		}
		fmt.Fprintf(stdout, "  %-28s %-8s %-16s %-10s %s\n",
			a.Item.ID(), renderAge(a.Item.Age(now)), it.Payload["lane"], tierName, claimState(claims, it.ID))
		fmt.Fprintf(stdout, "      surface=issue trust=%s — %s\n", a.State, a.Why)
	}

	// --- the coalesce decision ------------------------------------------------------
	decision, why := CoalescePolicy{Window: o.window}.Decide(o.openScanPR(), now)
	fmt.Fprintf(stdout, "\nCOALESCE (window %s): %s — %s\n", o.window, decision, why)
	fmt.Fprintf(stdout, "  the scan PR's title and body are REGENERATED on every push, never carried over.\n")

	fmt.Fprintf(stdout, "\nEXITS — every queued item leaves by exactly one of: %s\n", strings.Join(exitNames(), " · "))
	fmt.Fprintf(stdout, "  the routing decision itself is the model tier's; this loop supplies the queue, the trust verdict and the ledger.\n")

	RenderUnreadSurfaces(stdout)

	// A pass that could not see the whole surface is could-not-check, not idle. rc=0 here would
	// be read by anything scripting this tool as "nothing inbound".
	if report.Blind() {
		return deskkit.Unverifiable("scanloop plan: the inbound surface was not fully readable this pass "+
			"(degraded repos or a suppressed burst) — blind is not idle", nil)
	}
	if !state.Armed {
		return deskkit.Unverifiable("scanloop plan: the inbound monitor has no baseline for any rostered repo — "+
			"nothing is being watched, which is COULD-NOT-CHECK rather than a quiet queue. Run `scanloop run` to arm it.", nil)
	}
	return nil
}

func armingLine(st *MonitorState) string {
	if !st.Armed {
		return "NOT ARMED (no repo has a baseline)"
	}
	return fmt.Sprintf("ARMED for %d/%d rostered repo(s)", len(st.Seeded), len(st.Seeded)+len(st.Unseeded))
}

func inboundSource(path string) string {
	switch strings.TrimSpace(path) {
	case "":
		return "no captured poll — pass --inbound"
	case "-":
		return "stdin"
	default:
		return path
	}
}

func renderAge(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// loadClaims reads the shared claims dir once per plan. A dir that cannot be read yields NO claims
// and the rows say `unknown` — never `free`, which is the "assume free" direction that produces
// double dispatch.
func loadClaims() map[string]deskkit.Claim {
	out := map[string]deskkit.Claim{}
	dir, err := deskkit.StateDir()
	if err != nil {
		return nil
	}
	claims, err := deskkit.List(deskkit.ClaimConfig{ClaimsDir: filepath.Join(dir, "claims")})
	if err != nil {
		return nil
	}
	for _, c := range claims {
		out[c.Item] = c
	}
	return out
}

func claimState(claims map[string]deskkit.Claim, id string) string {
	if claims == nil {
		return "unknown (claims dir unreadable — never read as free)"
	}
	c, ok := claims[id]
	if !ok {
		return "free"
	}
	owner := c.Owner
	if owner == "" {
		owner = "unknown"
	}
	return "HELD by " + owner
}

func readInput(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

// tierNames is used by the help text and by the runner-table test: the reachable set is a property
// of TierPolicy, and stating it lets a boot-time validation catch a tier with no runner.
func tierNames(ts []loopengine.Tier) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.String())
	}
	sort.Strings(out)
	return out
}
