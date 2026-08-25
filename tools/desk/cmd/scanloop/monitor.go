package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// monitor.go — the inbound surface.
//
// The desk's inbound events come from a durable shell monitor that ships in the plugin tree. This
// file LOCATES, ARMS and PARSES that script; it never copies it. The script carries three
// properties a hand-rolled `gh` poll keeps losing, each one closing a way a monitor goes SILENTLY
// blind — it sets its own identity rather than inheriting a role token that cannot see the repo
// set, it keeps state PER REPO and retains a repo's baseline when its read fails or collapses, and
// it collapses a mass update to one burst line. Re-implementing the poll in Go would re-acquire all
// three bugs and gain nothing.

// monitorScriptName is the durable poller's file name inside the plugin tree.
const monitorScriptName = "inbound-monitor.sh"

// monitorScriptRelPath is where the plugin tree ships it, relative to a repo root.
var monitorScriptRelPath = filepath.Join("plugins", "assay", "scripts", monitorScriptName)

// EnvMonitorScript names an explicit path to the poller, for a layout the search below does not
// cover (an installed plugin directory, a vendored copy).
const EnvMonitorScript = "ASSAY_INBOUND_MONITOR"

// EnvMonitorStateDir is the poller's own per-repo state directory. Its contents are the ARMING
// evidence: a repo with no state file has never been polled.
const EnvMonitorStateDir = "INBOUND_MONITOR_STATE_DIR"

// FindMonitorScript resolves the poller, searching in declared order and naming every path it
// looked at when it fails. A not-found is UNVERIFIABLE, never "run without a monitor": a drain with
// no inbound surface is blind, and blind is not idle.
//
//  1. explicit — the --monitor flag
//  2. the ASSAY_INBOUND_MONITOR environment override
//  3. <root>/plugins/assay/scripts/inbound-monitor.sh
//  4. the same path under each immediate SIBLING checkout of <root> (one level, sorted) — the
//     ordinary multi-checkout layout, where the tree that ships the plugin is not the tree the
//     scan is rooted at
func FindMonitorScript(root, explicit string) (string, error) {
	var tried []string

	check := func(p string) (string, bool) {
		if p == "" {
			return "", false
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		tried = append(tried, abs)
		if st, err := os.Stat(abs); err == nil && !st.IsDir() {
			return abs, true
		}
		return "", false
	}

	if strings.TrimSpace(explicit) != "" {
		p, ok := check(explicit)
		if !ok {
			return "", deskkit.Refused("scanloop: --monitor " + explicit + " does not exist. " +
				"The poller is WRAPPED, never copied — point at the one the plugin tree ships.")
		}
		return p, nil
	}
	if p, ok := check(os.Getenv(EnvMonitorScript)); ok {
		return p, nil
	}
	if p, ok := check(filepath.Join(root, monitorScriptRelPath)); ok {
		return p, nil
	}

	// Sibling checkouts, one level up, in a stable order.
	absRoot, err := filepath.Abs(root)
	if err == nil {
		parent := filepath.Dir(absRoot)
		if entries, derr := os.ReadDir(parent); derr == nil {
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				if e.IsDir() && filepath.Join(parent, e.Name()) != absRoot {
					names = append(names, e.Name())
				}
			}
			sort.Strings(names)
			for _, n := range names {
				if p, ok := check(filepath.Join(parent, n, monitorScriptRelPath)); ok {
					return p, nil
				}
			}
		}
	}

	return "", deskkit.Unverifiable("scanloop: cannot locate "+monitorScriptName+
		" — the inbound surface is unreadable, which is COULD-NOT-CHECK, never an empty queue. "+
		"Searched: "+strings.Join(tried, ", ")+
		". Pass --monitor <path> or set "+EnvMonitorScript+".", nil)
}

// MonitorState is what the poller's state directory says about arming coverage. It is read, never
// written: a plan that ran the poller would advance its per-repo baselines and swallow the events
// it was asked to report.
type MonitorState struct {
	Dir      string
	Armed    bool     // at least one rostered repo has a baseline
	Seeded   []string // rostered repos WITH a baseline
	Unseeded []string // rostered repos WITHOUT one — blind until the next arm
	Foreign  []string // baselines for repos outside the current scan scope (a narrowed roster)
}

// stateFileName maps a repo slug to the poller's own state-file name. Slashes are the only
// reserved character and the script maps them to "__"; this must stay byte-identical to it or the
// arming read silently reports every repo unseeded.
func stateFileName(slug string) string {
	return strings.ReplaceAll(slug, "/", "__") + ".state"
}

func slugFromStateFile(name string) string {
	return strings.ReplaceAll(strings.TrimSuffix(name, ".state"), "__", "/")
}

// ReadMonitorState reports arming coverage for the rostered scan scope. A missing directory is not
// an error — it is the honest "never armed" answer — but an unreadable one is.
func ReadMonitorState(dir string, scope []string) (*MonitorState, error) {
	st := &MonitorState{Dir: dir}
	present := map[string]bool{}

	entries, err := os.ReadDir(dir)
	switch {
	case os.IsNotExist(err):
		// never armed
	case err != nil:
		return nil, deskkit.Unverifiable("scanloop: cannot read the monitor state dir "+dir, err)
	default:
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".state") {
				continue
			}
			present[slugFromStateFile(e.Name())] = true
		}
	}

	inScope := map[string]bool{}
	for _, r := range scope {
		inScope[r] = true
		if present[r] {
			st.Seeded = append(st.Seeded, r)
		} else {
			st.Unseeded = append(st.Unseeded, r)
		}
	}
	for slug := range present {
		if !inScope[slug] {
			st.Foreign = append(st.Foreign, slug)
		}
	}
	sort.Strings(st.Seeded)
	sort.Strings(st.Unseeded)
	sort.Strings(st.Foreign)
	st.Armed = len(st.Seeded) > 0
	return st, nil
}

// Inbound is one event the poller reported: an issue that is new, or whose updatedAt moved (a new
// comment — a parked worker resuming, or a human answering a decision request).
type Inbound struct {
	Repo      string
	Number    int
	UpdatedAt time.Time
	Raw       string
}

// ID is the stable claim key. It carries the repo prefix because two repos routinely own issue
// numbers of the same value, and an un-prefixed key would over-lock them onto one claim.
func (i Inbound) ID() string { return fmt.Sprintf("%s#%d", i.Repo, i.Number) }

// Age is how long the item has been quiet since its last update.
func (i Inbound) Age(now time.Time) time.Duration {
	if i.UpdatedAt.IsZero() {
		return 0
	}
	return now.Sub(i.UpdatedAt)
}

// MonitorReport is one poll cycle's output, parsed.
type MonitorReport struct {
	// Armed is true when THIS cycle seeded (the poller printed its armed line). A seed cycle
	// deliberately reports no inbound: emitting the whole backlog on first sight is the phantom
	// flood the per-repo state exists to prevent.
	Armed      bool
	ArmedTotal int
	Inbound    []Inbound
	// Bursts are the collapsed mass-update lines. They are events the drain CANNOT enumerate, so
	// they are carried as a stated bound, never dropped.
	Bursts []string
	// Degraded are the per-repo could-not-check lines. A degraded repo retained its baseline, so
	// its events are not lost — but this cycle is BLIND for it and must say so.
	Degraded []string
	// Unparsed are lines the parser did not recognise. An unknown line is surfaced rather than
	// discarded: a poller that grew a new line kind must not be silently half-read.
	Unparsed []string
}

// Blind reports whether this cycle could not see the whole surface — a degraded repo or a
// suppressed burst. It is the three-state instrument: not idle, not clean, could-not-check.
func (r *MonitorReport) Blind() bool { return len(r.Degraded) > 0 || len(r.Bursts) > 0 }

// ParseMonitorOutput turns the poller's stdout into a report. It is deliberately tolerant about
// trailing text on a line and strict about the leading token: the script's own contract is the
// prefix, and matching loosely on the remainder is what keeps this parser from breaking when a
// diagnostic gains a word.
func ParseMonitorOutput(out string) *MonitorReport {
	r := &MonitorReport{}
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "MONITOR-ARMED:"):
			r.Armed = true
			if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "MONITOR-ARMED:"))); err == nil {
				r.ArmedTotal = n
			}
		case strings.HasPrefix(line, "MONITOR-DEGRADED:"):
			r.Degraded = append(r.Degraded, strings.TrimSpace(strings.TrimPrefix(line, "MONITOR-DEGRADED:")))
		case strings.HasPrefix(line, "INBOUND-BURST:"):
			r.Bursts = append(r.Bursts, strings.TrimSpace(strings.TrimPrefix(line, "INBOUND-BURST:")))
		case strings.HasPrefix(line, "INBOUND:"):
			if in, ok := parseInboundLine(strings.TrimSpace(strings.TrimPrefix(line, "INBOUND:"))); ok {
				r.Inbound = append(r.Inbound, in)
			} else {
				r.Unparsed = append(r.Unparsed, line)
			}
		default:
			r.Unparsed = append(r.Unparsed, line)
		}
	}
	return r
}

// parseInboundLine reads "<owner>/<name>#<num> <RFC3339>". A malformed timestamp does NOT discard
// the item — the item is the work; the timestamp is only its age — but a malformed key does, since
// there is nothing to claim or route.
func parseInboundLine(s string) (Inbound, bool) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return Inbound{}, false
	}
	slug, numStr, ok := strings.Cut(fields[0], "#")
	if !ok {
		return Inbound{}, false
	}
	if !strings.Contains(slug, "/") {
		return Inbound{}, false
	}
	num, err := strconv.Atoi(numStr)
	if err != nil || num <= 0 {
		return Inbound{}, false
	}
	in := Inbound{Repo: slug, Number: num, Raw: s}
	if len(fields) > 1 {
		if ts, terr := time.Parse(time.RFC3339, fields[1]); terr == nil {
			in.UpdatedAt = ts.UTC()
		}
	}
	return in, true
}

// RunMonitor executes the poller over the rostered scan scope and parses its output. It is the ONLY
// place this binary runs it, and it runs it only from `run` — arming and draining are the same act
// (the first cycle seeds silently, every later one reports the delta), which is why `plan` reads
// the state dir instead.
//
// The poller's exit 2 means at least one repo went degraded and RETAINED its baseline. That is
// could-not-check for those repos, not a failed run: the report carries the degraded lines and the
// caller decides. Only a precondition failure (exit 1) is fatal.
func RunMonitor(script, stateDir string, scope []string, runner func(script string, env []string, args ...string) (string, int, error)) (*MonitorReport, error) {
	if len(scope) == 0 {
		return nil, deskkit.Unverifiable("scanloop: the intake SCAN scope is empty — "+
			"an empty sweep is never a clean, empty board", nil)
	}
	if runner == nil {
		runner = execMonitor
	}
	env := append(os.Environ(), EnvMonitorStateDir+"="+stateDir)
	out, code, err := runner(script, env, scope...)
	report := ParseMonitorOutput(out)
	switch {
	case code == 2:
		// Degraded repos: retained baselines, and the report already names them.
		return report, nil
	case err != nil || code != 0:
		return report, deskkit.Unverifiable(fmt.Sprintf(
			"scanloop: the inbound monitor exited %d — the inbound surface could not be read: %s",
			code, strings.TrimSpace(out)), err)
	}
	return report, nil
}

func execMonitor(script string, env []string, args ...string) (string, int, error) {
	cmd := exec.Command("/bin/bash", append([]string{script}, args...)...)
	cmd.Env = env
	b, err := cmd.CombinedOutput()
	code := 0
	var ee *exec.ExitError
	if err != nil {
		if asExit(err, &ee) {
			code = ee.ExitCode()
			err = nil
		} else {
			code = -1
		}
	}
	return string(b), code, err
}

// asExit is errors.As specialised to the one type this file cares about, kept as a named helper so
// the exec path above reads as the two-state thing it is: the process ran and returned a code, or
// it never ran at all.
func asExit(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}
