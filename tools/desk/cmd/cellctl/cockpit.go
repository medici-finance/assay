package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

// A COCKPIT is only the SURFACE the role windows appear in. Nothing about the per-role worktree,
// the roster beacon, the pinned model or the shim PATH changes with it, and no cockpit is ever
// required: tmux is the always-works arm every other arm falls back to.
//
// Selection is by PRESENCE ON PATH, never a flag someone has to remember. `auto` prefers herdr
// (labelled tabs and a semantic agent state that drives its sidebar), then orca (scheduled
// automations), then tmux. Orca's CLI is a THIN CLIENT of its desktop app, so an `orca` binary
// whose app is not answering falls THROUGH to tmux under `auto` rather than failing. An EXPLICIT
// cockpit that is not available is a refusal naming exactly what is missing — never a silent
// fall-through.
type cockpitResolution struct {
	Cockpit string
	Why     string
	Err     string
}

// cockpitWant is which cockpit was ASKED for, and by whom — the flag beats cell.env beats the
// default.
func (c *Cell) cockpitWant(flag string) (want, src string) {
	switch {
	case flag != "":
		return flag, "--cockpit"
	case c.Env.Get("CELL_COCKPIT") != "":
		return c.Env.Get("CELL_COCKPIT"), "cell.env CELL_COCKPIT"
	default:
		return "auto", "default"
	}
}

func onPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// runBounded is a probe that cannot hang the launcher, with NO dependency on an external
// timeout binary (macOS ships neither `timeout` nor `gtimeout` by default). `timeout`/`gtimeout`
// are used first when present only because they are a well-worn primitive; the native path is
// what guarantees the bound holds when neither is installed. A bound that fires returns 124, the
// convention both of those use, so callers never have to distinguish the two implementations.
func runBounded(secs int, name string, args ...string) int {
	for _, t := range []string{"timeout", "gtimeout"} {
		if onPath(t) {
			cmd := exec.Command(t, append([]string{strconv.Itoa(secs), name}, args...)...)
			if err := cmd.Run(); err != nil {
				return exitStatus(err)
			}
			return 0
		}
	}
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return 127
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			rc := exitStatus(err)
			if rc > 128 {
				rc = 124
			}
			return rc
		}
		return 0
	case <-time.After(time.Duration(secs) * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return 124
	}
}

// orcaReachable: `orca` on PATH is not enough to pick it — with the desktop app closed every
// repo, worktree, terminal and automation verb fails. One cheap read verb is the probe.
func (c *Cell) orcaReachable() bool {
	if !onPath("orca") {
		return false
	}
	secs := 5
	if v := c.Env.Get("CELLCTL_ORCA_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			secs = n
		}
	}
	fmt.Fprintf(os.Stderr, "[cockpit] probing orca reachability (orca repo list, bound %ds)\n", secs)
	return runBounded(secs, "orca", "repo", "list") == 0
}

// helpHas: does `<cmd...> --help` advertise this subcommand or flag? These cockpit CLIs move
// fast (one ships a patch every few days), so every verb and flag whose exact spelling this
// program cannot see is PROBED at run time and skipped with a NOTICE when the installed build
// does not carry it, rather than hard-coded as a truth that may have drifted.
func helpHas(tok string, cmd ...string) bool {
	out := helpText(cmd...)
	return helpAdvertises(out, tok)
}

func helpText(cmd ...string) string {
	c := exec.Command(cmd[0], append(cmd[1:], "--help")...)
	var buf bytes.Buffer
	c.Stdout, c.Stderr = &buf, &buf
	_ = c.Run()
	return buf.String()
}

// helpAdvertises is the oracle's `grep -qE -- "(^|[[:space:]])<tok>([[:space:],=]|$)"`.
func helpAdvertises(help, tok string) bool {
	re := regexp.MustCompile(`(^|[ \t\r\n\f\v])` + regexp.QuoteMeta(tok) + `([ \t\r\n\f\v,=]|$)`)
	return re.MatchString(help)
}

// firstFlag prints the first candidate the help advertises, or ("", false).
func firstFlag(help string, candidates ...string) (string, bool) {
	for _, f := range candidates {
		if helpAdvertises(help, f) {
			return f, true
		}
	}
	return "", false
}

// resolveCockpit sets the cockpit and says why, or returns Err holding the refusal text. Callers
// decide whether an unavailable cockpit is a die (`up`) or a MISS row (`check`).
func (c *Cell) resolveCockpit(want, src string) cockpitResolution {
	switch want {
	case "auto", "tmux", "herdr", "orca":
	default:
		return cockpitResolution{Err: fmt.Sprintf("%s: '%s' is not a known cockpit (auto|tmux|herdr|orca)", src, want)}
	}
	switch want {
	case "tmux":
		if !onPath("tmux") {
			return cockpitResolution{Err: "cockpit tmux (" + src + ") but tmux is not on PATH"}
		}
		return cockpitResolution{Cockpit: "tmux", Why: "explicit: " + src}
	case "herdr":
		if !onPath("herdr") {
			return cockpitResolution{Err: "cockpit herdr (" + src + ") but herdr is not on PATH"}
		}
		return cockpitResolution{Cockpit: "herdr", Why: "explicit: " + src}
	case "orca":
		if !onPath("orca") {
			return cockpitResolution{Err: "cockpit orca (" + src + ") but orca is not on PATH"}
		}
		if !c.orcaReachable() {
			return cockpitResolution{Err: "cockpit orca (" + src + "): orca is on PATH but its desktop app is not reachable — the CLI is a thin client, so start the app (or 'orca serve') and retry"}
		}
		return cockpitResolution{Cockpit: "orca", Why: "explicit: " + src}
	default: // auto
		switch {
		case onPath("herdr"):
			return cockpitResolution{Cockpit: "herdr", Why: "auto: on PATH"}
		case onPath("orca") && c.orcaReachable():
			return cockpitResolution{Cockpit: "orca", Why: "auto: on PATH"}
		case onPath("orca"):
			return cockpitResolution{Cockpit: "tmux", Why: "orca on PATH but app unreachable"}
		default:
			return cockpitResolution{Cockpit: "tmux", Why: "fallback: no herdr/orca on PATH"}
		}
	}
}
