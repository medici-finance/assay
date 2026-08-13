package deskkit

// The risk-classification CALLOUT.
//
// WHY A CALLOUT AND NOT A CONFIG LIST. ASSAY_RISK_PATH_TRIGGERS_EXTRA already
// lets an adopter WIDEN the pattern classifier through configuration, but widening
// through a pattern LIST has a ceiling: narrowing an adopter's own triggers — a
// genuine, reviewable policy change — has nowhere to go except a silent config
// edit, which is the exact reviewability property the house's security gate
// refuses to lose. A callout moves the interpretation to the adopter's OWN repo:
// their real triggers stay compiled and code-reviewed there, and the desk here
// asks a yes/no question and fails closed on anything but a clean answer.
//
// THE CONTRACT. The desk execs
// `$ASSAY_RISK_CALLOUT classify`, feeding `{"repo":"...","changedFiles":[...]}`
// on stdin and reading `{"riskClassed":true|false,"reason":"..."}` from stdout.
// Exit 0 = answered. Every other outcome — non-zero exit, a timeout, output that
// does not parse, a missing or non-executable binary, an empty configured path
// discovered mid-run — answers classed=true, with a reason naming which failure
// happened.
//
// NOTE ON deskkit.Callout. The writeguard callout (ASSAY_WRITEGUARD_CALLOUT)
// provides SHARED exec/timeout/fail-closed plumbing in this package. This file's
// exec wrapper is deliberately self-contained, under names distinct from that
// shared type (`riskCallout*` rather than `callout*`/`Callout`), so it carries no
// build dependency on it. A follow-up could collapse this file's exec/timeout/
// fail-closed loop onto the shared deskkit.Callout type and delete the
// duplication. Nothing about the SAFETY PROPERTIES below is provisional; only
// which package supplies the plumbing is.
//
// ONLY-WIDENS lives structurally in unionClassifier (riskclassifier.go): the
// callout is one member of a union with patternClassifier, and no member's
// "false" can clear another member's "true".

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// riskCalloutTimeout bounds one callout invocation. The desk's risk-classification
// step sits in the interactive ready-gate / board-render path, so the bound is
// what keeps "the adopter's script hung" from becoming "the flip hung" — kept
// small per the brief's instruction to pick and document one.
const riskCalloutTimeout = 5 * time.Second

// riskCalloutWaitDelay is the grace period between killing a timed-out callout
// and cutting its output pipes. Without it, a callout that leaves a grandchild
// process holding the stdout/stderr pipe open (e.g. a backgrounded `sleep`) makes
// Wait block for as long as that grandchild lives — a timeout advertised but not
// enforced, which for a fail-closed caller means the gate hangs instead of
// refusing. The grace only starts AFTER the kill, so it costs nothing on a
// healthy run.
const riskCalloutWaitDelay = 250 * time.Millisecond

// maxRiskCalloutOutput caps what is read back from the callout. The answer this
// package reads is one short JSON line; a callout that streams beyond that is not
// answering the question, and how much memory is spent reading it must not be the
// adopter's to choose.
const maxRiskCalloutOutput = 64 << 10

// riskCalloutRequest is the Q1-ruled stdin envelope.
type riskCalloutRequest struct {
	Repo         string   `json:"repo"`
	ChangedFiles []string `json:"changedFiles"`
}

// riskCalloutResponse is the Q1-ruled stdout envelope.
type riskCalloutResponse struct {
	RiskClassed bool   `json:"riskClassed"`
	Reason      string `json:"reason"`
}

// calloutClassifier is the RiskClassifier that consults ASSAY_RISK_CALLOUT.
//
// It carries no fields on purpose: the configured path is read fresh from
// EffectiveConfig() on every Classify call rather than captured once, so a test
// (or a long-lived process observing ReloadConfig) sees the CURRENT configuration
// rather than whatever was in force when a value of this type was constructed.
type calloutClassifier struct{}

var _ RiskClassifier = calloutClassifier{}

// Classify implements RiskClassifier. It fails closed — returns classed=true —
// on every way the callout can fail to answer cleanly. An unconfigured callout
// (the empty path) is NOT a failure: it means "this classifier has nothing to
// add", which is exactly what the shipped, callout-free behavior always was.
func (calloutClassifier) Classify(repo string, changedFiles []string) (bool, string) {
	path := strings.TrimSpace(EffectiveConfig().RiskCallout)
	if path == "" {
		// Unset: contributes nothing to the union. notRiskClassed here is read as
		// "this classifier abstains", not as a claim the diff is clean — only the
		// union's own zero-classifiers-fired case renders that claim.
		return false, notRiskClassed
	}

	req := riskCalloutRequest{Repo: repo, ChangedFiles: changedFiles}
	payload, err := json.Marshal(req)
	if err != nil {
		// Unreachable for a struct of a string and a string slice, but a
		// fail-closed gate does not get an unreachable branch that returns false.
		return true, fmt.Sprintf("ASSAY_RISK_CALLOUT request could not be encoded: %v — fail closed", err)
	}

	stdout, err := runRiskCallout(path, string(payload))
	if err != nil {
		return true, fmt.Sprintf("ASSAY_RISK_CALLOUT %q did not answer: %v — fail closed", path, err)
	}

	var resp riskCalloutResponse
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		got := strings.TrimSpace(stdout)
		if got == "" {
			got = "(empty)"
		}
		return true, fmt.Sprintf("ASSAY_RISK_CALLOUT %q printed unparseable output %q: %v — fail closed",
			path, truncateForReason(got, 200), err)
	}

	if resp.RiskClassed {
		reason := strings.TrimSpace(resp.Reason)
		if reason == "" {
			reason = "ASSAY_RISK_CALLOUT flagged this diff (no reason given)"
		}
		return true, reason
	}
	// A clean answer never overrides notRiskClassed's textual contract — the union
	// is what decides whether the OVERALL verdict is classed, not this member.
	return false, notRiskClassed
}

// runRiskCallout execs path as `<path> classify`, feeding stdin on the process's
// standard input, and returns its trimmed standard output.
//
// It returns an error — never a partial answer — for every way the question can
// go unanswered: the path is not an absolute, executable, non-group/world-writable
// regular file in a non-group/world-writable directory, the process cannot be
// spawned, it exits non-zero, or it exceeds riskCalloutTimeout. Classify's
// fail-closed rule keys on `err != nil` alone.
//
// NO SHELL is involved: exec.CommandContext runs the binary directly, so the JSON
// payload on stdin is never word-split, glob-expanded, or substituted.
func runRiskCallout(path, stdin string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("not an absolute path")
	}
	if err := riskCalloutExecutableCheck(path); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), riskCalloutTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "classify")
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout = &riskCalloutLimitedWriter{w: &out, n: maxRiskCalloutOutput}
	cmd.Stderr = &riskCalloutLimitedWriter{w: &errb, n: maxRiskCalloutOutput}
	// See the package doc comment: without WaitDelay, killing a timed-out callout
	// does not close its output pipe if a grandchild process still holds it open,
	// and Wait blocks for as long as that grandchild lives.
	cmd.WaitDelay = riskCalloutWaitDelay

	runErr := cmd.Run()
	stdout := strings.TrimSpace(out.String())

	if ctx.Err() == context.DeadlineExceeded {
		return stdout, fmt.Errorf("did not answer within %s", riskCalloutTimeout)
	}
	if runErr != nil {
		detail := runErr.Error()
		if e := strings.TrimSpace(errb.String()); e != "" {
			detail += "; stderr: " + truncateForReason(e, 200)
		}
		return stdout, errors.New(detail)
	}
	return stdout, nil
}

// riskCalloutExecutableCheck enforces what must be true of the file at INVOCATION
// time — permissions can change between the run that loaded the roster and the
// run that invokes the callout, so this is checked fresh on every call rather
// than once at config load.
//
// Anything that can WRITE the callout (or the directory holding it) chooses what
// this gate decides, so a group- or world-writable callout, or one sitting in a
// group- or world-writable directory, is refused — a fail-closed caller treats
// that exactly like a missing binary rather than trusting it anyway.
func riskCalloutExecutableCheck(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot be read: %w", err)
	}
	if fi.IsDir() {
		return errors.New("is a directory, not an executable")
	}
	if !fi.Mode().IsRegular() {
		return errors.New("is not a regular file")
	}
	if fi.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("is not executable (mode %04o)", fi.Mode().Perm())
	}
	if m := fi.Mode().Perm(); m&0o022 != 0 {
		return fmt.Errorf("is group- or world-writable (mode %04o): anything that can write it "+
			"chooses what this gate decides", m)
	}
	dir := filepath.Dir(path)
	dfi, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("directory %q does not exist", dir)
		}
		return fmt.Errorf("directory %q cannot be read: %w", dir, err)
	}
	if m := dfi.Mode().Perm(); m&0o022 != 0 {
		return fmt.Errorf("directory %q is group- or world-writable (mode %04o): the executable in "+
			"it can be replaced without ever changing its own mode", dir, m)
	}
	return nil
}

// riskCalloutLimitedWriter writes at most n bytes and silently discards the rest.
type riskCalloutLimitedWriter struct {
	w *bytes.Buffer
	n int
}

func (l *riskCalloutLimitedWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil
	}
	if len(p) > l.n {
		l.w.Write(p[:l.n])
		l.n = 0
		return len(p), nil
	}
	l.w.Write(p)
	l.n -= len(p)
	return len(p), nil
}

// truncateForReason bounds a diagnostic string folded into a refusal reason, so a
// callout that dumps a stack trace does not blow up a board row or a flip
// refusal message.
func truncateForReason(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
