package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/acp"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// dispatch_native.go implements the native-primitive dispatch: instead of emitting an
// instruction for the operator to run (interim mode), it spawns the dispatched verifier as a
// real ACP child process, drives it through the core flow, and delivers the parsed structured
// Result through the SAME loopengine.Handle the engine already awaits. The frozen Loop/Handle
// contract is untouched — this lives entirely INSIDE Dispatch (arch doc §9.1).
//
// Every safety property the interim mode delegated to a human is now Go plumbing:
//   - isolation: the child's session cwd IS a fresh worktree at Item.TargetSHA; nothing else;
//   - permission: default-deny; verify-desk allows reads in the worktree and verify commands
//     only; every decision (allow AND refuse) is one audit line; stop-flags are re-checked on
//     EVERY permission callback, not just at iteration boundaries;
//   - result integrity: the worker's final report is PARSED into a typed Result; a report that
//     does not parse lands as VerdictBlocked with its raw tail preserved in the audit log —
//     free text never reaches Land.

const defaultNativeTimeout = 15 * time.Minute

// drainGrace bounds the post-Close wait for the update-drain goroutine to finish. That wait sits
// OUTSIDE NativeTimeout (which covers spawn→prompt), so without a bound a future teardown
// regression that failed to close Updates() would wedge the dispatch goroutine forever — no
// BLOCKED, no error, and the engine drain stalls. The bound converts any such regression into a
// BLOCKED instead. It is generous: on the healthy path Close kills+waits the child and onClose
// closes Updates() well within it.
const drainGrace = 30 * time.Second

// dispatchNative is the native branch of Dispatch. It creates the isolated worktree, then runs
// the ACP round-trip on a goroutine so Dispatch returns a Handle immediately (the engine awaits
// Handle.Done()). Any failure to even START (no runner configured, worktree create failed) is a
// synchronous error — the engine never gets a phantom in-flight item.
func (v *VerifyLoop) dispatchNative(it loopengine.Item, tier loopengine.Tier) (loopengine.Handle, error) {
	prompt := renderDispatchPrompt(it, tier)
	if err := assertNoSharedCheckout(prompt); err != nil {
		return nil, err
	}

	// Resolve the runner for this item's tier from the tier→runner table (or
	// the legacy single value). A refusal here — no runner configured at all, or the
	// tier is unconfigured in the table — is a SYNCHRONOUS error: the engine never gets a
	// phantom in-flight item for a dispatch that could not even name its runner.
	runnerCmd, runnerID, err := v.resolveRunner(tier)
	if err != nil {
		v.auditNative("dispatch", deskkit.ResultRefused, it, "native dispatch refused: "+err.Error())
		return nil, fmt.Errorf("native dispatch for %s: %w", it.ID, err)
	}

	mk := v.MakeWorktree
	if mk == nil {
		mk = gitDetachedWorktree
	}
	dir, cleanup, err := mk(it)
	if err != nil {
		v.auditNative("dispatch", deskkit.ResultUnverifiable, it, "native dispatch: isolated worktree create failed: "+err.Error())
		return nil, fmt.Errorf("native dispatch for %s: create isolated worktree at %s: %w", it.ID, it.TargetSHA, err)
	}
	if cleanup == nil {
		cleanup = func() {}
	}

	done := make(chan loopengine.Result, 1)
	go func() {
		defer cleanup()
		done <- v.runNativeDispatch(it, dir, prompt, runnerCmd, runnerID)
	}()
	return &handle{item: it, done: done}, nil
}

// resolveRunner picks the runner argv + RunnerID for one dispatch. The tier→runner
// table is THE runner surface when configured: it resolves the item's tier to a
// pinned entry and derives the RunnerID from that row (tier + cmd + model + pin), so attribution
// is known at spawn. With no table, it falls back to the legacy single RunnerCmd (which
// keeps the landed native path and its tests byte-for-byte). With neither, it refuses — a runner
// that reaches the network is never a default.
func (v *VerifyLoop) resolveRunner(tier loopengine.Tier) (cmd []string, runnerID string, err error) {
	if v.RunnerTable != nil {
		e, rerr := v.RunnerTable.Resolve(tier)
		if rerr != nil {
			return nil, "", rerr
		}
		// RunnerID flows from the resolved table entry — not v.RunnerID (the operator/session
		// identity, which stays the author!=runner left-hand side), and never a worker self-report.
		return e.Cmd, e.RunnerID(tier), nil
	}
	if len(v.RunnerCmd) > 0 {
		return v.RunnerCmd, v.nativeRunnerID(), nil
	}
	return nil, "", fmt.Errorf(
		"no runner configured — refusing (a runner that reaches the network needs an explicit RunnerTable entry or RunnerCmd; the runner table owns the tier→runner map)")
}

// runNativeDispatch drives one dispatched verifier to a typed Result. It NEVER returns an
// error: an unrecoverable protocol/spawn failure, and an unparseable report, both land as a
// VerdictBlocked Result (with the cause preserved in the audit log) so the drain continues and
// Land files a bug rather than the engine wedging.
func (v *VerifyLoop) runNativeDispatch(it loopengine.Item, dir, prompt string, runnerCmd []string, runnerID string) loopengine.Result {
	blocked := func(detail, raw string) loopengine.Result {
		if tail := auditTail(raw); tail != "" {
			detail += " | raw-tail: " + tail
		}
		v.auditNative("result", deskkit.ResultUnverifiable, it, detail)
		return loopengine.Result{Item: it, Verdict: loopengine.VerdictBlocked, RunnerID: runnerID}
	}

	// Print-target-before-contact: audit the exact argv about to be exec'd BEFORE any spawn or
	// network contact, so the runner identity is on record even if Spawn itself fails (it matters
	// more now that the runner table makes the runner a table lookup rather than a literal).
	v.auditNative("dispatch", deskkit.ResultOK, it, "native dispatch spawning runner ["+strings.Join(runnerCmd, " ")+"] runnerID="+runnerID+" cwd="+dir)

	var stderr bytes.Buffer
	cl, err := acp.Spawn(runnerCmd, acp.Opts{
		Dir:              dir,
		Env:              v.NativeEnv,
		Stderr:           &stderr,
		FSRoot:           dir, // the fs fence: reads/writes are refused outside the worktree
		PermissionPolicy: v.permissionPolicy(it, dir),
		ClientName:       "verifyloop",
		ClientVersion:    "native",
	})
	if err != nil {
		return blocked("acp spawn failed: "+err.Error(), stderr.String())
	}
	defer cl.Close()

	timeout := v.NativeTimeout
	if timeout <= 0 {
		timeout = defaultNativeTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if _, err := cl.Initialize(ctx); err != nil {
		return blocked("acp initialize failed: "+err.Error(), stderr.String())
	}
	sid, err := cl.NewSession(ctx, dir)
	if err != nil {
		return blocked("acp session/new failed: "+err.Error(), stderr.String())
	}
	// MANDATORY (acp README finding): the agent's default session mode "auto" resolves
	// session/request_permission internally and never calls the client back, so WITHOUT this
	// the permission gate above is dead code. Setting "default" routes every permission
	// decision through PermissionPolicy.
	if err := cl.SetMode(ctx, sid, "default"); err != nil {
		return blocked("acp session/set_mode(default) failed: "+err.Error(), stderr.String())
	}

	// Reconstruct the worker's final report by concatenating agent_message_chunk text across
	// the turn. Draining concurrently (rather than reading the buffered channel after the fact)
	// means a long report cannot overflow the update buffer and lose a chunk. The drain
	// terminates when Updates() closes at connection teardown (the acp revision this consumer
	// forced).
	var report strings.Builder
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for u := range cl.Updates() {
			if txt, ok := u.Text(); ok {
				report.WriteString(txt)
			}
		}
	}()

	pres, perr := cl.Prompt(ctx, sid, prompt)
	// Tear the connection down so the drain goroutine's range over Updates() ends, then read
	// the fully-accumulated report. Close is idempotent (defer cl.Close() also runs).
	_ = cl.Close()
	// Bound the drain wait (drainGrace): the healthy path closes Updates() promptly, but a
	// future teardown regression must land BLOCKED rather than wedge here forever. On timeout we
	// do NOT read report — the drain goroutine may still be writing it, and that read would race.
	select {
	case <-drainDone:
	case <-time.After(drainGrace):
		return blocked("native dispatch drain did not complete within grace after connection close (teardown regression)", "")
	}
	raw := report.String()

	if perr != nil {
		return blocked("acp session/prompt failed: "+perr.Error(), raw+"\n"+stderr.String())
	}

	r, ok := parseResult(raw)
	if !ok {
		// Parse-or-BLOCKED: an unparseable report is not a verdict. The raw tail is preserved
		// in the audit log; free text never reaches Land.
		return blocked("worker report did not parse into a structured Result (stopReason="+pres.StopReason+")", raw)
	}
	r.Item = it
	// RunnerID is recorded from what the ENGINE spawned (the resolved table
	// entry, or the legacy value), not self-reported by the worker — attribution is the
	// whole point, and a transcribed runner id would re-open the gap.
	r.RunnerID = runnerID
	v.auditNative("result", deskkit.ResultOK, it, "native dispatch parsed: verdict="+r.Verdict+" rows="+strconv.Itoa(len(r.Rows)))
	return r
}

// nativeRunnerID is the identity the engine dispatched AS, recorded from what it spawned. An
// explicit v.RunnerID (the operator/session identity) wins; otherwise it names the runner
// command, so Evidence attribution reflects the real child process.
func (v *VerifyLoop) nativeRunnerID() string {
	if v.RunnerID != "" {
		return v.RunnerID
	}
	if len(v.RunnerCmd) > 0 {
		return "acp:" + strings.Join(v.RunnerCmd, " ")
	}
	return "unknown-runner"
}

// permissionPolicy is the verify-desk permission profile: default-deny, reads inside the
// worktree and the listed verify commands only, every decision audited, stop-flags re-checked
// per callback.
func (v *VerifyLoop) permissionPolicy(it loopengine.Item, worktree string) acp.PermissionPolicy {
	return func(_ context.Context, req acp.PermissionRequest) acp.PermissionDecision {
		// Stop-flags / kill-switch checked on EVERY permission callback, not just at iteration
		// boundaries (brief facts). A stop armed mid-turn refuses the next tool call fail-closed;
		// deskkit.Guard writes its own disabled audit line, and we add the tool-call context.
		if err := deskkit.Guard(); err != nil {
			v.auditNative("permission", deskkit.ResultRefused, it, "stop/kill active mid-turn — refusing tool call ["+req.Kind+"] "+req.Title)
			return acp.PermissionDecision{Allow: false}
		}
		allow, reason := verifyDeskAllows(req, worktree)
		result := deskkit.ResultRefused
		if allow {
			result = deskkit.ResultOK
		}
		v.auditNative("permission", result, it, reason+" ["+req.Kind+"] "+req.Title)
		return acp.PermissionDecision{Allow: allow}
	}
}

// verifyDeskAllows is the pure decision function (no I/O, no audit) so it can be unit-tested
// directly. Default-deny: a verifier reads the tree it verifies and runs the verify commands;
// it never mutates, and it never runs a command that is not a recognized verify invocation.
func verifyDeskAllows(req acp.PermissionRequest, worktree string) (allow bool, reason string) {
	switch req.Kind {
	case "read", "fetch":
		// Reads are allowed; the fs fence (FSRoot=worktree) scopes fs/* callbacks to the
		// worktree independently, so an out-of-worktree read is refused there even if a read
		// tool call is permitted here.
		return true, "allow-read"
	case "edit", "delete", "move", "rename", "create":
		// A verifier NEVER mutates the tree under verification.
		return false, "refuse-mutation"
	}
	// execute / other: allow ONLY a recognized, un-chained verify command.
	cmd := extractCommand(req)
	if strings.TrimSpace(cmd) == "" {
		return false, "refuse-command-unparseable"
	}
	if isVerifyCommand(cmd) {
		return true, "allow-verify-command"
	}
	return false, "refuse-non-verify-command"
}

// verifyCommandPrefixes is the allowlist of verify-command leading tokens. A command is allowed
// only when it is (or starts with, on a token boundary) one of these AND carries no shell
// chaining/redirection/substitution (see isVerifyCommand). Kept deliberately narrow: this is
// safety plumbing where a subtle over-broad match survives tests.
//
// This allowlist is NOT the isolation boundary and must not be read as one. It matches the
// LEADING token only: `cat`, `head`, `tail`, `grep`, `rg`, `ls`, `wc` carry no path check, so an
// absolute path outside the worktree is not refused here (the fs fence, FSRoot=worktree, covers
// fs/* ACP callbacks but not commands the agent execs directly); and `go test` / `go run`
// execute arbitrary code by construction. Isolation therefore rests on the worktree CONTENTS at
// Item.TargetSHA — a detached checkout of exactly the SHA under verification and nothing else —
// not on this allowlist. The allowlist's job is narrower: keep a verify turn from reaching for an
// obviously non-verify tool (curl, a package installer) in the common case.
var verifyCommandPrefixes = []string{
	"go build", "go test", "go run", "go vet",
	"git diff", "git show", "git status", "git log", "git rev-parse", "git ls-files",
	"ls", "cat", "head", "tail", "grep", "rg", "wc", "gofmt", "true",
}

// isVerifyCommand reports whether cmd is a single, simple verify invocation. Any shell
// metacharacter that could chain to a non-verify command (`;`, `|`, `&`, redirection, command
// substitution, backticks, newline) makes it fail closed — a verify row is one invocation.
func isVerifyCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	if strings.ContainsAny(cmd, ";|&<>`\n") || strings.Contains(cmd, "$(") {
		return false
	}
	for _, p := range verifyCommandPrefixes {
		if cmd == p || strings.HasPrefix(cmd, p+" ") {
			return true
		}
	}
	return false
}

// wirePermissionToolCall pulls the command out of a session/request_permission payload. Agents
// vary in where they put it; this checks the common shapes (rawInput.command / rawInput.cmd).
// It deliberately does NOT fall back to the tool-call title (see extractCommand).
// PermissionRequest.Raw is the full request params.
type wirePermissionToolCall struct {
	ToolCall struct {
		Title    string `json:"title"`
		RawInput struct {
			Command string `json:"command"`
			Cmd     string `json:"cmd"`
		} `json:"rawInput"`
	} `json:"toolCall"`
}

func extractCommand(req acp.PermissionRequest) string {
	var w wirePermissionToolCall
	if err := json.Unmarshal(req.Raw, &w); err == nil {
		if c := strings.TrimSpace(w.ToolCall.RawInput.Command); c != "" {
			return c
		}
		if c := strings.TrimSpace(w.ToolCall.RawInput.Cmd); c != "" {
			return c
		}
	}
	// Default-deny: with no recognized command key present, do NOT fall back to the
	// tool-call Title. Title is display text the agent chooses freely — an agent could
	// title a payload `go test ./...` while hiding the real command under an unrecognized
	// key and thereby win an allow for a command the gate never saw. Returning empty lands
	// `refuse-command-unparseable`; new payload shapes are added above explicitly as they
	// are observed.
	return ""
}

// verdictLine matches the worker's verdict declaration anywhere in the report.
var verdictLine = regexp.MustCompile(`(?im)^\s*VERDICT:\s*(PASS|FAIL|BLOCKED|NEEDS_CONTEXT)\s*$`)

// parseResult turns the worker's final report text into a typed Result, or reports ok=false so
// the caller lands VerdictBlocked. The recognized format is a `VERDICT: <verdict>` line plus
// zero or more Evidence rows as markdown table rows `| command | exit | output |` (the exit
// cell must be an integer, which is exactly what skips the header and separator rows). A report
// with no parseable verdict is NOT a verdict — free text never becomes a PASS.
//
// The report is the whole concatenated turn, not just a closing block, so a worker may state a
// provisional verdict mid-turn and revise it at the end. The load-bearing verdict is therefore
// the LAST one — never the leftmost, which would land a provisional PASS an agent later revised
// to FAIL. And two DIFFERING verdicts in one transcript are treated as ambiguous and land
// BLOCKED (ok=false): a verifier that says PASS early and FAIL late must never resolve to PASS,
// so we refuse to guess rather than pick a side (parse-or-BLOCKED, the conservative rule the
// rest of this file follows).
func parseResult(raw string) (loopengine.Result, bool) {
	all := verdictLine.FindAllStringSubmatch(raw, -1)
	if len(all) == 0 {
		return loopengine.Result{}, false
	}
	verdict := strings.ToUpper(strings.TrimSpace(all[len(all)-1][1]))
	for _, m := range all {
		if strings.ToUpper(strings.TrimSpace(m[1])) != verdict {
			// Conflicting verdicts across the turn — ambiguous, not a verdict.
			return loopengine.Result{}, false
		}
	}
	return loopengine.Result{
		Verdict: verdict,
		Rows:    parseEvidenceRows(raw),
	}, true
}

func parseEvidenceRows(raw string) []loopengine.EvidenceRow {
	var rows []loopengine.EvidenceRow
	for _, ln := range strings.Split(raw, "\n") {
		ln = strings.TrimSpace(ln)
		if len(ln) < 2 || ln[0] != '|' || ln[len(ln)-1] != '|' {
			continue
		}
		cells := strings.Split(strings.Trim(ln, "|"), "|")
		if len(cells) != 3 {
			continue
		}
		exit, err := strconv.Atoi(strings.TrimSpace(cells[1]))
		if err != nil {
			continue // header row ("Exit") / separator row ("---") — not an evidence row
		}
		rows = append(rows, loopengine.EvidenceRow{
			Command: strings.TrimSpace(cells[0]),
			Exit:    exit,
			Output:  strings.TrimSpace(cells[2]),
		})
	}
	return rows
}

// auditTail returns a trimmed, single-line tail of a raw report for the audit detail — enough
// to diagnose an unparseable report without dumping the whole turn into the log.
func auditTail(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, "\n", " ⏎ ")
	const max = 400
	if len(raw) > max {
		raw = raw[len(raw)-max:]
	}
	return deskkit.StripControl(raw)
}

// auditNative appends exactly one audit line per native-dispatch decision. Best-effort: an
// audit-write failure never changes the dispatch outcome (the drain must not wedge on it).
func (v *VerifyLoop) auditNative(verb, result string, it loopengine.Item, detail string) {
	_ = deskkit.Log(deskkit.Entry{
		Tool:   "verifyloop",
		Verb:   verb,
		Repo:   v.Root,
		Result: result,
		Detail: it.ID + ": " + detail,
	})
}

// gitDetachedWorktree is the default MakeWorktree: a fresh detached worktree under the system
// temp dir at Item.TargetSHA, cleaned up with `git worktree remove --force`. The session cwd
// handed to the agent is this dir and nothing else.
func gitDetachedWorktree(it loopengine.Item) (string, func(), error) {
	if strings.TrimSpace(it.TargetSHA) == "" {
		return "", nil, fmt.Errorf("no TargetSHA to create an isolated worktree at")
	}
	base, err := os.MkdirTemp("", "verifyloop-"+sanitizeID(it.ID)+"-")
	if err != nil {
		return "", nil, err
	}
	dir := filepath.Join(base, "wt")
	cmd := exec.Command("git", "worktree", "add", "--detach", dir, it.TargetSHA)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(base)
		return "", nil, fmt.Errorf("git worktree add --detach %s %s: %v: %s", dir, it.TargetSHA, err, strings.TrimSpace(string(out)))
	}
	cleanup := func() {
		_ = exec.Command("git", "worktree", "remove", "--force", dir).Run()
		_ = os.RemoveAll(base)
	}
	return dir, cleanup, nil
}

func sanitizeID(id string) string {
	return strings.NewReplacer("/", "-", " ", "-", ":", "-").Replace(id)
}
