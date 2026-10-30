package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// allowClusterEnv is the operator opt-in. It is declared in deskkit so the
// roster's unknown-ASSAY_-key refusal recognises it (see EnvAllowCluster); it
// is READ here, and only here, with a direct os.Getenv.
const allowClusterEnv = deskkit.EnvAllowCluster

// logName is the append-only refusal/pass-through record, written under the
// config home. It is the DETECTION surface: a stopped probe leaves a line.
const logName = "clusterguard.log"

// shimmedCLIs is the compiled-in set of cluster CLIs this guard stands in
// front of. The shim directory holds one symlink per name, all pointing at the
// single clusterguard binary; the guard reads which CLI it is from argv[0].
//
// The set is deliberately SMALL and explicit. Its limits are stated rather than
// implied: HTTP and SDK access (curl, a client library), a credentialed
// browser or tool integration, and ssh/docker hops are outside it entirely.
// This layer narrows one exec path; it is not a network boundary.
var shimmedCLIs = []string{"kubectl", "flux", "helm", "talosctl", "k9s"}

// readOnlyVerbs is an ALLOWLIST per CLI, which is the fail-closed direction: a
// verb nobody classified is treated as MUTATING, so a subcommand added upstream
// is refused rather than waved through on the day it appears.
//
// k9s carries an empty allowlist on purpose. It is an interactive TUI with no
// subcommand grammar, and every mutating operation it offers is reachable from
// inside the session — there is no read-only invocation the guard could verify
// from argv, so it has no read-only lane at all.
var readOnlyVerbs = map[string]map[string]bool{
	"kubectl": set("get", "describe", "logs", "top", "explain", "events",
		"api-resources", "api-versions", "cluster-info", "version", "diff"),
	"helm": set("list", "status", "get", "show", "search", "history",
		"template", "lint", "env", "version"),
	"flux": set("get", "logs", "check", "diff", "tree", "events", "stats", "version"),
	"talosctl": set("get", "list", "read", "dmesg", "health", "containers",
		"logs", "memory", "disks", "services", "stats", "time", "version"),
	"k9s": {},
}

// bareOnlyVerbs are allowlisted verbs that are read-only only in their BARE
// form. The subcommand also accepts positional arguments, and the allowlist
// entry vouches for the bare listing, not for what an argument selects — so a
// positional argument after the verb takes the call off the read-only tier,
// exactly as an unclassified verb is. Every entry here must also be in
// readOnlyVerbs.
var bareOnlyVerbs = map[string]map[string]bool{
	"talosctl": set("services"),
}

// flagRestrictedVerbs are allowlisted verbs that are read-only only with the
// flags listed for them. The subcommand also accepts flags that make it do more
// than read, and the allowlist entry vouches for the reading form, not for
// every flag the CLI may add to it — so after such a verb, a flag that is
// neither listed here nor in the CLI's valueFlags takes the call off the
// read-only tier, exactly as an unclassified verb does. Every flag listed here
// consumes a value. Every entry here must also be in readOnlyVerbs.
var flagRestrictedVerbs = map[string]map[string]map[string]bool{
	"talosctl": {
		"health": set("--wait-timeout", "--control-plane-nodes", "--worker-nodes",
			"--init-node", "--k8s-endpoint"),
	},
}

// valueFlags are the global flags that CONSUME the next argument. Without them
// the verb scan would read a flag's value as the verb — `kubectl -n prod get`
// would classify on "prod".
//
// The table is deliberately NOT complete, and its completeness is not what keeps
// the read-only tier safe. A flag before the subcommand that is neither listed
// here nor written `--flag=value` has an arity the guard does not know: it may
// consume the next argument (`kubectl --username get delete pod x` runs
// `delete`), so scanVerb treats the whole call as unclassifiable, which is the
// mutating side. A MISSING entry therefore costs a false refusal — the operator
// writes `--flag=value` instead — never a false pass.
//
// The one way this table CAN cause a false pass is an entry that does not really
// take a value: the scan would skip the real verb as that flag's "value" and
// classify on the token after it. Add a flag here only when the CLI's own help
// shows it consumes an argument.
var valueFlags = map[string]map[string]bool{
	"kubectl": set("-n", "--namespace", "--context", "--cluster", "--kubeconfig",
		"--user", "--server", "-s", "--as", "--as-group", "--token",
		"--request-timeout", "--cache-dir", "--tls-server-name"),
	"helm": set("-n", "--namespace", "--kube-context", "--kubeconfig",
		"--kube-apiserver", "--kube-token", "--registry-config", "--repository-config"),
	"flux": set("-n", "--namespace", "--context", "--kubeconfig", "--timeout",
		"--cluster", "--kube-api-burst", "--kube-api-qps"),
	"talosctl": set("-n", "--nodes", "-e", "--endpoints", "--context",
		"--talosconfig", "--cluster"),
	"k9s": set("-n", "--namespace", "--context", "--kubeconfig", "-c", "--command"),
}

// credentialFlags name arguments whose VALUE must never reach the log. The log
// is written on every call; it must not become a credential file.
var credentialFlags = []string{"token", "password", "secret", "passwd", "key", "credential"}

func set(vs ...string) map[string]bool {
	m := make(map[string]bool, len(vs))
	for _, v := range vs {
		m[v] = true
	}
	return m
}

// --- the opt-in ---------------------------------------------------------------

// optInTier is how far the operator opened the gate. There are two levels
// rather than one because the read-only and the mutating call are different
// acts: an operator who wants to LOOK at the cluster should not have to hand
// themselves the ability to delete from it in the same export.
type optInTier int

const (
	tierNone     optInTier = iota // no opt-in: the default posture, refuse everything
	tierReadOnly                  // read-only verbs pass; mutating verbs are refused
	tierMutate                    // everything passes
	tierInvalid                   // a value the guard does not recognise: refuse, never guess
)

func (t optInTier) String() string {
	switch t {
	case tierReadOnly:
		return "read-only"
	case tierMutate:
		return "mutate"
	case tierInvalid:
		return "unrecognised"
	default:
		return "absent"
	}
}

// parseTier maps the exported value to a tier. An UNRECOGNISED value is its own
// outcome, never a silent fall back to "absent" and never a promotion to
// "allowed": a typo in a safety opt-in must be visible.
func parseTier(v string, present bool) optInTier {
	if !present {
		return tierNone
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "":
		return tierNone
	case "1", "ro", "read-only", "readonly":
		return tierReadOnly
	case "mutate", "rw", "write":
		return tierMutate
	default:
		return tierInvalid
	}
}

// --- verb classification --------------------------------------------------------

// verbScan is what the guard could establish about an invocation's subcommand.
type verbScan struct {
	verb string // the subcommand; "" when none could be established
	at   int    // the index of verb in args; meaningful only when verb != ""
	// unlisted is the first flag before the subcommand whose arity the guard
	// does not know (not in valueFlags, no "="). When it is set, verb is "":
	// the token after such a flag may be its value, so no later token can be
	// trusted as the verb.
	unlisted string
}

// scanVerb finds the subcommand of an invocation: the first argument that is
// neither a flag nor a flag's value. `--flag=value` is self-contained and
// skipped; `--flag value` consumes its value when the flag is in the CLI's
// valueFlags table. Any OTHER flag before the subcommand stops the scan with
// no verb — it might consume the next argument, and guessing that it does not
// is exactly how `--username get delete` passed as a `get`. Everything after a
// bare `--` is an argument to the command, never the verb.
func scanVerb(cli string, args []string) verbScan {
	vf := valueFlags[cli]
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return verbScan{}
		}
		if strings.HasPrefix(a, "-") {
			if strings.Contains(a, "=") {
				continue // --flag=value: self-contained
			}
			if vf[a] {
				i++ // skip the value this flag consumes
				continue
			}
			return verbScan{unlisted: a} // unknown arity: unclassifiable
		}
		return verbScan{verb: a, at: i}
	}
	return verbScan{}
}

// hasPositional reports whether rest — the arguments after the verb — holds a
// positional (non-flag) argument. A listed value flag consumes its value and
// `--flag=value` is self-contained, as in scanVerb. Any OTHER flag is taken to
// consume nothing, so the token after it counts as positional: here that is
// the fail-closed guess, since guessing that the flag consumed it would hide a
// real argument. Everything after a bare `--` is positional, and so is a lone
// `-`, which the flag parser reads as an argument, not a flag.
func hasPositional(cli string, rest []string) bool {
	vf := valueFlags[cli]
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		if a == "--" {
			return i+1 < len(rest)
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			return true
		}
		if !strings.Contains(a, "=") && vf[a] {
			i++ // skip the listed flag's value
		}
	}
	return false
}

// unlistedArg reports the first argument of a call to the flagRestrictedVerbs
// entry at args[at] that the entry does not classify, with found true; found is
// false only when every argument is classified. BOTH sides of the verb are read:
// scanVerb skips a pre-verb `--flag=value` as self-contained, and the CLI's flag
// parser may still hand that flag to the subcommand, so a flag the entry does
// not list is refused before the verb exactly as after it. The bool, not the
// string, carries the verdict: an empty argument is itself unclassified and
// must not read as "nothing found".
func unlistedArg(cli string, args []string, at int) (string, bool) {
	verb := args[at]
	for _, seg := range [][]string{args[:at], args[at+1:]} {
		if a, found := unlistedIn(cli, verb, seg); found {
			return a, true
		}
	}
	return "", false
}

// unlistedIn scans one side of the verb. A classified argument is a flag listed
// for the verb or in the CLI's valueFlags (bare or written `--flag=value`), or
// the value such a flag consumes. A token that starts with "-" is never taken as
// a value, so a listed flag cannot hide the flag after it, and a `--flag=value`
// consumes nothing. Anything else — a positional argument, an empty argument, a
// lone `-`, a bare `--` — names no listed flag, so it is unclassified.
func unlistedIn(cli, verb string, seg []string) (string, bool) {
	listed := flagRestrictedVerbs[cli][verb]
	vf := valueFlags[cli]
	for i := 0; i < len(seg); i++ {
		a := seg[i]
		name, _, inline := strings.Cut(a, "=")
		if !listed[name] && !vf[name] {
			return a, true
		}
		if !inline && i+1 < len(seg) && !strings.HasPrefix(seg[i+1], "-") {
			i++ // skip the listed flag's value
		}
	}
	return "", false
}

// isReadOnly reports whether this invocation is on the CLI's read-only
// allowlist. No verb at all is NOT read-only: a bare invocation, or one whose
// verb sits behind a flag the guard cannot classify, is unclassifiable, and
// unclassifiable is the mutating side of the line. A bareOnlyVerbs entry is
// read-only only when no positional argument follows it; a flagRestrictedVerbs
// entry is read-only only when every argument of the call, before and after
// the verb, is classified.
func isReadOnly(cli string, args []string) bool {
	allowed, known := readOnlyVerbs[cli]
	if !known || len(allowed) == 0 {
		return false
	}
	s := scanVerb(cli, args)
	if s.verb == "" || !allowed[s.verb] {
		return false
	}
	if bareOnlyVerbs[cli][s.verb] && hasPositional(cli, args[s.at+1:]) {
		return false
	}
	if _, restricted := flagRestrictedVerbs[cli][s.verb]; restricted {
		if _, found := unlistedArg(cli, args, s.at); found {
			return false
		}
	}
	return true
}

// --- the verdict ------------------------------------------------------------------

type verdict struct {
	allow    bool
	code     int
	verb     string
	readOnly bool
	reason   string
}

// decide applies the policy. Every path that is not an explicit allow is a
// refusal with a stated reason — there is no default-allow branch.
func decide(cli string, args []string, t optInTier) verdict {
	scan := scanVerb(cli, args)
	ro := isReadOnly(cli, args)
	v := verdict{verb: scan.verb, readOnly: ro}

	switch t {
	case tierMutate:
		v.allow = true
		v.code = deskkit.ExitOK
		v.reason = "opt-in " + t.String()
		return v

	case tierReadOnly:
		if ro {
			v.allow = true
			v.code = deskkit.ExitOK
			v.reason = "opt-in read-only, verb on the read-only allowlist"
			return v
		}
		v.code = deskkit.ExitRefused
		if scan.unlisted != "" && len(readOnlyVerbs[cli]) > 0 {
			v.reason = fmt.Sprintf(
				"%s is exported at the read-only level, and the flag %q comes before the subcommand "+
					"but is not one whose arity this guard knows: it may consume the next argument, so "+
					"the subcommand cannot be established and the call is treated as mutating. Write it "+
					"as %s=<value> (or move it after the subcommand) to have the call classified; an "+
					"operator who means to mutate the cluster exports %s=mutate deliberately.",
				allowClusterEnv, scan.unlisted, scan.unlisted, allowClusterEnv)
			return v
		}
		if bareOnlyVerbs[cli][scan.verb] {
			v.reason = fmt.Sprintf(
				"%s is exported at the read-only level, and %q is on %s's read-only allowlist only in "+
					"its bare form: a positional argument after it is not classified, so the call is "+
					"treated as mutating. An operator who means to mutate the cluster exports %s=mutate deliberately.",
				allowClusterEnv, scan.verb, cli, allowClusterEnv)
			return v
		}
		if _, restricted := flagRestrictedVerbs[cli][scan.verb]; restricted {
			if a, found := unlistedArg(cli, args, scan.at); found {
				v.reason = fmt.Sprintf(
					"%s is exported at the read-only level, and %q is on %s's read-only allowlist only with "+
						"the flags this guard classifies for it: the argument %q (before or after it) is not one "+
						"of them, so the call is treated as mutating. An operator who means to mutate the cluster "+
						"exports %s=mutate deliberately.",
					allowClusterEnv, scan.verb, cli, a, allowClusterEnv)
				return v
			}
		}
		v.reason = fmt.Sprintf(
			"%s is exported at the read-only level, and %q is not on %s's read-only allowlist "+
				"(a verb the guard does not classify is treated as mutating). "+
				"An operator who means to mutate the cluster exports %s=mutate deliberately.",
			allowClusterEnv, displayVerb(scan.verb), cli, allowClusterEnv)
		return v

	case tierInvalid:
		v.code = deskkit.ExitRefused
		v.reason = fmt.Sprintf(
			"%s is set to a value this guard does not recognise. Accepted: 1 (or read-only) "+
				"for read-only verbs, mutate for everything. An unrecognised value is refused "+
				"rather than read as either yes or no.", allowClusterEnv)
		return v

	default:
		v.code = deskkit.ExitRefused
		v.reason = fmt.Sprintf(
			"desk sessions are offline-by-default: the cluster CLIs on this PATH resolve to "+
				"clusterguard, which refuses unless an OPERATOR shell exported %s "+
				"(=1 for read-only verbs, =mutate for everything). A desk window, a dispatched "+
				"worker, or a script they run must never export it.", allowClusterEnv)
		return v
	}
}

func displayVerb(v string) string {
	if v == "" {
		return "(no subcommand)"
	}
	return v
}

// --- pass-through resolution -------------------------------------------------------

// resolveReal finds the real CLI: the first executable named cli on PATH that
// is NOT this running binary. Skipping self is the whole difficulty — a shim
// that resolves by name alone finds its own symlink and re-execs itself
// forever, which is the classic way this control turns into a fork bomb.
//
// The test is os.SameFile against the running executable, not a path
// comparison: the shim is reached through a symlink whose textual path shares
// nothing with the binary's own, and a deployment may put the shim directory on
// PATH more than once.
func resolveReal(cli, pathEnv, self string) (string, error) {
	selfInfo, err := os.Stat(self)
	if err != nil {
		// Fail closed. Without knowing what "self" is there is no way to
		// guarantee the next exec is not this binary again.
		return "", deskkit.Unverifiable(
			"cannot identify this executable, so pass-through cannot be proven not to re-exec the guard itself", err)
	}
	var tried []string
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		cand := filepath.Join(dir, cli)
		info, err := os.Stat(cand) // follows symlinks: a shim link resolves to the guard
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		if os.SameFile(info, selfInfo) {
			tried = append(tried, dir)
			continue // this is us
		}
		return cand, nil
	}
	return "", deskkit.Unverifiable(fmt.Sprintf(
		"no %s found on PATH past this guard (skipped %d shim director%s). The opt-in is "+
			"exported but there is nothing to run — refusing rather than exiting 0 on a call "+
			"that never happened", cli, len(tried), plural(len(tried))), nil)
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// --- the log -------------------------------------------------------------------------

// redactArgv replaces the VALUE of any credential-bearing flag with a marker,
// in both the `--flag value` and `--flag=value` forms.
func redactArgv(argv []string) []string {
	out := make([]string, 0, len(argv))
	redactNext := false
	for _, a := range argv {
		if redactNext {
			out = append(out, "REDACTED")
			redactNext = false
			continue
		}
		if strings.HasPrefix(a, "-") {
			name, _, hasEq := strings.Cut(a, "=")
			if isCredentialFlag(name) {
				if hasEq {
					out = append(out, name+"=REDACTED")
					continue
				}
				redactNext = true
			}
		}
		out = append(out, a)
	}
	return out
}

func isCredentialFlag(name string) bool {
	n := strings.ToLower(strings.TrimLeft(name, "-"))
	for _, c := range credentialFlags {
		if strings.Contains(n, c) {
			return true
		}
	}
	return false
}

// appendLog writes one line for this invocation. A logging failure NEVER
// changes the verdict — the guard's job is to refuse, and a read-only
// filesystem must not turn a refusal into a pass-through — but it is reported
// on stderr so a silently-unrecorded surface is visible.
func appendLog(cli, verdictName, verb string, readOnly bool, argv []string, stderr io.Writer) {
	path := deskkit.ConfigHomeWritePath(logName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		fmt.Fprintf(stderr, "clusterguard: cannot create the log directory (%v) — the verdict stands, the record does not\n", err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintf(stderr, "clusterguard: cannot append to %s (%v) — the verdict stands, the record does not\n", path, err)
		return
	}
	defer f.Close()

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "?"
	}
	fmt.Fprintf(f, "%s cli=%s verdict=%s verb=%s read-only=%t cwd=%s argv=%q\n",
		time.Now().UTC().Format(time.RFC3339), cli, verdictName, displayVerb(verb), readOnly,
		cwd, strings.Join(redactArgv(argv), " "))
}

// --- entry point -----------------------------------------------------------------------

const usage = `clusterguard — exec-boundary shim for cluster CLIs

Installed as a directory of symlinks (one per shimmed CLI) that desk sessions
put on the FRONT of PATH. Every shimmed CLI then resolves to this binary, which
refuses the call unless an operator shell exported the opt-in, records both
verdicts, and otherwise execs the real CLI further along PATH.

  ASSAY_ALLOW_CLUSTER unset    every shimmed CLI is refused (exit 5)
  ASSAY_ALLOW_CLUSTER=1        read-only verbs pass; mutating verbs are refused
  ASSAY_ALLOW_CLUSTER=mutate   every verb passes

Exit: 0 passed through · 3 stop flag armed · 5 refused · 6 unverifiable.
`

func run(argv []string, stdout, stderr io.Writer) int {
	if len(argv) == 0 {
		fmt.Fprint(stderr, usage)
		return deskkit.ExitUnverifiable
	}
	name := filepath.Base(argv[0])
	args := argv[1:]

	// Invoked by its OWN name: the informational path. It contacts nothing.
	if name == "clusterguard" || strings.HasPrefix(name, "clusterguard.") {
		if len(args) > 0 && (args[0] == "--version" || args[0] == "-version") {
			sha, built := deskkit.Version()
			fmt.Fprintf(stdout, "clusterguard sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
			return deskkit.ExitOK
		}
		fmt.Fprint(stdout, usage)
		fmt.Fprintf(stdout, "shimmed CLIs: %s\n", strings.Join(shimmedCLIs, " "))
		return deskkit.ExitOK
	}

	if !isShimmed(name) {
		fmt.Fprintf(stderr,
			"clusterguard: invoked as %q, which is not one of the CLIs this guard stands in front of (%s). "+
				"Refusing to exec anything: a shim symlink the guard cannot classify is a misconfiguration, "+
				"and guessing which binary was meant is exactly the pass-through this control exists to deny.\n",
			name, strings.Join(shimmedCLIs, ", "))
		return deskkit.ExitUnverifiable
	}

	// deskkit.Guard() is the mandatory first stop-flag check, and here it can
	// only make the guard STRICTER. An armed kill switch that made a REFUSAL
	// guard stop intercepting would fail OPEN — it would hand every stopped
	// session the cluster CLIs back — so an armed flag is itself a refusal.
	// The uninstall path is removing the shim directory from PATH, not arming a
	// flag; the README records that decision.
	if err := deskkit.Guard(); err != nil {
		appendLog(name, "refused", "", false, argv, stderr)
		fmt.Fprintf(stderr,
			"clusterguard: %v. A stop flag NEVER opens this guard — cluster CLIs stay refused while it is "+
				"armed. To uninstall the guard, take the shim directory off PATH.\n", err)
		return deskkit.ExitCodeOf(err)
	}

	raw, present := os.LookupEnv(allowClusterEnv)
	v := decide(name, args, parseTier(raw, present))

	if !v.allow {
		appendLog(name, "refused", v.verb, v.readOnly, argv, stderr)
		fmt.Fprintf(stderr, "clusterguard: refused %s %s — %s\n",
			name, strings.Join(redactArgv(args), " "), v.reason)
		fmt.Fprintf(stderr,
			"clusterguard: this is exit %d, a REFUSAL. It is not a fallback trigger: do not reach the "+
				"same CLI by absolute path, by a rebuilt PATH, or through another tool. Recorded in %s.\n",
			v.code, deskkit.ConfigHomeWritePath(logName))
		return v.code
	}

	self, err := os.Executable()
	if err != nil {
		appendLog(name, "refused", v.verb, v.readOnly, argv, stderr)
		fmt.Fprintf(stderr, "clusterguard: cannot resolve this executable (%v) — refusing rather than "+
			"risking a pass-through that re-execs the guard itself\n", err)
		return deskkit.ExitUnverifiable
	}
	real, err := resolveReal(name, os.Getenv("PATH"), self)
	if err != nil {
		appendLog(name, "refused", v.verb, v.readOnly, argv, stderr)
		fmt.Fprintf(stderr, "clusterguard: %v\n", err)
		return deskkit.ExitCodeOf(err)
	}

	// Log BEFORE handing control away: the record of an allowed call must exist
	// whether or not the CLI it reached ever returns.
	appendLog(name, "allowed", v.verb, v.readOnly, argv, stderr)
	return passThrough(real, args)
}

// passThrough runs the real CLI with the arguments untouched and this process's
// stdio, then propagates its exit status.
//
// It spawns rather than replacing the process image (no syscall.Exec): the
// desk-tools build targets Windows as well as unix, exec-replacement has no
// portable form, and nothing in this guard's contract needs the process image
// to be the CLI's. Stdio is inherited directly, so an interactive TUI behaves
// exactly as it would unshimmed.
func passThrough(real string, args []string) int {
	cmd := exec.Command(real, args...) // #nosec G204 — real is resolved from PATH by this guard, args are the caller's own argv, and no shell is involved
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if ok := asExit(err, &ee); ok {
			return ee.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "clusterguard: %s could not be started: %v\n", real, err)
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}

func asExit(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

func isShimmed(name string) bool {
	i := sort.SearchStrings(sortedShimmed, name)
	return i < len(sortedShimmed) && sortedShimmed[i] == name
}

var sortedShimmed = func() []string {
	s := append([]string(nil), shimmedCLIs...)
	sort.Strings(s)
	return s
}()
