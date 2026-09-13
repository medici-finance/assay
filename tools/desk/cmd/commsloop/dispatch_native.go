package main

import (
	"bytes"
	"context"
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

// dispatch_native.go — the executor dispatch leg (brief 08): the FULL
// autonomous path from an accepted, assigned message to a fired executor session and a
// parsed-or-BLOCKED Result, with no operator relay step. Copies verifyloop's native
// dispatch pattern (cmd/verifyloop/dispatch_native.go) — isolated worktree as session
// cwd, session mode forced off "auto" (the acp README finding: "auto" resolves
// session/request_permission internally and never calls the client back), parse-or-
// BLOCKED, every permission decision audited — with the per-role permission fence
// (roleprofile.go) standing in for verify-desk's single fixed profile.
//
// The FOUR standing guards this brief's Task 3 requires are all on THIS path, not
// documented beside it:
//   - kill switch: l.guard() before every spawn (dispatchNative), re-checked on every
//     permission callback (roleprofile.go's rolePermissionPolicy) — not just once per
//     drain cycle;
//   - per-hour firing budget: deskkit.AllowWrite, scoped per dispatch target, before any
//     runner is resolved or worktree created — exhaustion refuses synchronously (zero
//     sessions spawned) and the engine's own retry/backoff on a RateLimited error is what
//     leaves the item un-landed ("queue holds") rather than this file re-implementing a
//     hold path;
//   - role fence: an unknown role refuses before spawn; a known role's profile is the
//     ONLY permission surface an executor session gets;
//   - daily sweep: every spawn decision and every permission decision is one audit line
//     (auditNative), the same journal brief 09's sweep reads.

// defaultExecutorTimeout bounds one executor dispatch (spawn->initialize->session->
// prompt->parse) when NativeTimeout is unset.
const defaultExecutorTimeout = 15 * time.Minute

// executorDrainGrace bounds the post-Close wait for the update-drain goroutine, the same
// teardown-regression backstop verifyloop's drainGrace documents: without a bound, a
// future regression that failed to close Updates() would wedge forever instead of
// landing BLOCKED.
const executorDrainGrace = 30 * time.Second

// dispatchBudgetTool is the canonical audit/budget key the firing-budget check is
// counted under (audittoolkey.go already registers "commsloop").
const dispatchBudgetTool = "commsloop"

// dispatchNative is the native branch of Dispatch for a dispatchable (non-local) tier.
// Every refusal below is SYNCHRONOUS — no worktree, no runner resolution, no spawn — so
// "no fire" is literal: the engine never gets a phantom in-flight item for a dispatch
// that never even started.
func (l *Loop) dispatchNative(it loopengine.Item, tier loopengine.Tier) (loopengine.Handle, error) {
	// Kill switch FIRST, before every spawn — not just once per SelectQueue cycle. A STOP
	// armed between two items dispatched in the same drain pass must still halt the second.
	if err := l.guard(); err != nil {
		l.auditNative("dispatch", deskkit.ResultRefused, it, "native executor dispatch refused before spawn: kill switch/stop active: "+err.Error())
		return nil, fmt.Errorf("commsloop: native dispatch for %s: %w", it.ID, err)
	}

	role, err := targetRoleFor(it)
	if err != nil {
		l.auditNative("dispatch", deskkit.ResultRefused, it, "native executor dispatch refused: "+err.Error())
		return nil, err
	}
	// Unknown role = refuse dispatch, never a permissive default profile (Task 2).
	if err := resolveRoleProfile(role); err != nil {
		l.auditNative("dispatch", deskkit.ResultRefused, it, err.Error())
		return nil, err
	}

	// Per-hour firing budget (deskkit ratelimit): exhaustion = no fire, and the engine's
	// own RateLimited retry/backoff (loopengine/retry.go Classify) is what holds the item
	// rather than landing it — this refusal is returned VERBATIM so that classification
	// fires exactly as it does for every other deskkit-rate-limited write in this codebase.
	scope := dispatchBudgetScope(it)
	if err := deskkit.AllowWrite(dispatchBudgetTool, scope, 0); err != nil {
		l.auditNative("dispatch", deskkit.ResultRateLimited, it,
			"native executor dispatch refused: firing budget exhausted, no session fired, item stays queued: "+err.Error())
		return nil, err
	}

	runnerCmd, runnerID, err := l.resolveRunner(tier)
	if err != nil {
		l.auditNative("dispatch", deskkit.ResultRefused, it, "native executor dispatch refused: "+err.Error())
		return nil, fmt.Errorf("commsloop: native dispatch for %s: %w", it.ID, err)
	}

	prompt := renderExecutorPrompt(it, tier, role)

	mk := l.MakeWorktree
	if mk == nil {
		mk = gitDetachedWorktreeComms
	}
	dir, cleanup, err := mk(it)
	if err != nil {
		l.auditNative("dispatch", deskkit.ResultUnverifiable, it, "native executor dispatch: isolated worktree create failed: "+err.Error())
		return nil, fmt.Errorf("commsloop: native dispatch for %s: create isolated worktree: %w", it.ID, err)
	}
	if cleanup == nil {
		cleanup = func() {}
	}

	done := make(chan loopengine.Result, 1)
	go func() {
		defer cleanup()
		done <- l.runNativeDispatch(it, role, dir, prompt, runnerCmd, runnerID)
	}()
	return &nativeHandle{item: it, done: done}, nil
}

// targetRoleFor extracts the target desk role from the item's "to" payload field
// (loop.go's toLoopItem writes it as "<cell>/<role>"), the same envelope-derived value
// every accepted message already carries.
func targetRoleFor(it loopengine.Item) (string, error) {
	to := strings.TrimSpace(it.Payload["to"])
	idx := strings.LastIndex(to, "/")
	if idx < 0 || idx == len(to)-1 {
		return "", fmt.Errorf("commsloop: item %s carries no parseable target role in payload[\"to\"]=%q", it.ID, to)
	}
	return to[idx+1:], nil
}

// dispatchBudgetScope is the firing budget's scope string — the same to-cell/to-role
// pair the role fence keys on, so a runaway toward one target cannot exhaust the budget
// for every other target sharing the tool-wide bucket.
func dispatchBudgetScope(it loopengine.Item) string {
	return it.Payload["to"]
}

// resolveRunner picks the runner argv + RunnerID for one dispatch, mirroring
// verifyloop's own resolveRunner: the tier->runner table is THE runner surface when
// configured; RunnerCmd is the legacy fallback; neither configured refuses rather than
// reach the network with an implicit default.
func (l *Loop) resolveRunner(tier loopengine.Tier) (cmd []string, runnerID string, err error) {
	if l.RunnerTable != nil {
		e, rerr := l.RunnerTable.Resolve(tier)
		if rerr != nil {
			return nil, "", rerr
		}
		return e.Cmd, e.RunnerID(tier), nil
	}
	if len(l.RunnerCmd) > 0 {
		return l.RunnerCmd, l.nativeRunnerID(), nil
	}
	return nil, "", fmt.Errorf(
		"no runner configured for the executor dispatch leg — refusing (a runner that reaches the network needs an explicit RunnerTable entry or RunnerCmd)")
}

func (l *Loop) nativeRunnerID() string {
	if l.RunnerID != "" {
		return l.RunnerID
	}
	if len(l.RunnerCmd) > 0 {
		return "acp:" + strings.Join(l.RunnerCmd, " ")
	}
	return "unknown-runner"
}

// runNativeDispatch drives one executor session to a typed Result. It NEVER returns an
// error: an unrecoverable protocol/spawn failure, and an unparseable report, both land
// as a VerdictBlocked Result (cause preserved in the audit log) so the drain continues.
func (l *Loop) runNativeDispatch(it loopengine.Item, role, dir, prompt string, runnerCmd []string, runnerID string) loopengine.Result {
	blocked := func(detail, raw string) loopengine.Result {
		if tail := executorAuditTail(raw); tail != "" {
			detail += " | raw-tail: " + tail
		}
		l.auditNative("result", deskkit.ResultUnverifiable, it, detail)
		return loopengine.Result{Item: it, Verdict: loopengine.VerdictBlocked, RunnerID: runnerID}
	}

	l.auditNative("dispatch", deskkit.ResultOK, it,
		"native executor dispatch spawning runner ["+strings.Join(runnerCmd, " ")+"] role="+role+" runnerID="+runnerID+" cwd="+dir)

	var stderr bytes.Buffer
	cl, err := acp.Spawn(runnerCmd, acp.Opts{
		Dir:              dir,
		Env:              l.NativeEnv,
		Stderr:           &stderr,
		FSRoot:           dir, // fs fence: reads/writes refused outside the worktree
		PermissionPolicy: l.rolePermissionPolicy(role, it, dir),
		FileAccessPolicy: rootScopedFileAccessComms(dir),
		ClientName:       "commsloop",
		ClientVersion:    "native",
	})
	if err != nil {
		return blocked("acp spawn failed: "+err.Error(), stderr.String())
	}
	defer cl.Close()

	timeout := l.NativeTimeout
	if timeout <= 0 {
		timeout = defaultExecutorTimeout
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
	// MANDATORY (acp README finding): "auto" mode resolves session/request_permission
	// internally and never calls the client back, so without this the role fence above is
	// dead code. Setting "default" routes every permission decision through it.
	if err := cl.SetMode(ctx, sid, "default"); err != nil {
		return blocked("acp session/set_mode(default) failed: "+err.Error(), stderr.String())
	}

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
	_ = cl.Close()
	select {
	case <-drainDone:
	case <-time.After(executorDrainGrace):
		return blocked("native executor dispatch drain did not complete within grace after connection close (teardown regression)", "")
	}
	raw := report.String()

	if perr != nil {
		return blocked("acp session/prompt failed: "+perr.Error(), raw+"\n"+stderr.String())
	}

	r, ok := parseExecutorResult(raw)
	if !ok {
		return blocked("executor report did not parse into a structured Result (stopReason="+pres.StopReason+")", raw)
	}
	r.Item = it
	// RunnerID is recorded from what the ENGINE spawned, never self-reported.
	r.RunnerID = runnerID
	l.auditNative("result", deskkit.ResultOK, it, "native executor dispatch parsed: verdict="+r.Verdict+" rows="+strconv.Itoa(len(r.Rows)))
	return r
}

// nativeHandle is the in-flight tracker for a native executor dispatch: Done() fires
// when runNativeDispatch returns.
type nativeHandle struct {
	item loopengine.Item
	done chan loopengine.Result
}

func (h *nativeHandle) Done() <-chan loopengine.Result { return h.done }
func (h *nativeHandle) Item() loopengine.Item          { return h.item }

// renderExecutorPrompt builds the per-item executor prompt: role, tier, the envelope's
// from/to/verb, and the structured-result contract. This is the ONE per-loop-authored
// prose for the executor leg (arch doc §4 precedent).
func renderExecutorPrompt(it loopengine.Item, tier loopengine.Tier, role string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "EXECUTE %s (role=%s, tier=%s)\n\n", it.ID, role, tier)
	fmt.Fprintf(&b, "From: %s\n", it.Payload["from"])
	fmt.Fprintf(&b, "To: %s\n", it.Payload["to"])
	fmt.Fprintf(&b, "Verb: %s\n\n", it.Payload["verb"])
	b.WriteString("You are firing as this target role's fenced session — your permissions are the\n")
	b.WriteString(role + " profile only; an out-of-profile action is refused, not warned.\n\n")
	b.WriteString("Report back a STRUCTURED result: a clear verdict PASS|FAIL|BLOCKED, your runner\n")
	b.WriteString("identity, and one Evidence row per action taken (command, exit, output). Free-text\n")
	b.WriteString("verdicts are not accepted.\n")
	return b.String()
}

// executorVerdictLine / parseExecutorResult / parseExecutorEvidenceRows mirror
// verifyloop's parseResult/parseEvidenceRows exactly (same parse-or-BLOCKED contract,
// same "last verdict wins, differing verdicts are ambiguous" rule) — a package-local
// copy, not a shared import, for the same reason roleAllows is (see roleprofile.go).
var executorVerdictLine = regexp.MustCompile(`(?im)^\s*VERDICT:\s*(PASS|FAIL|BLOCKED|NEEDS_CONTEXT)\s*$`)

func parseExecutorResult(raw string) (loopengine.Result, bool) {
	all := executorVerdictLine.FindAllStringSubmatch(raw, -1)
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
		Rows:    parseExecutorEvidenceRows(raw),
	}, true
}

func parseExecutorEvidenceRows(raw string) []loopengine.EvidenceRow {
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
			continue // header/separator row — not an evidence row
		}
		rows = append(rows, loopengine.EvidenceRow{
			Command: strings.TrimSpace(cells[0]),
			Exit:    exit,
			Output:  strings.TrimSpace(cells[2]),
		})
	}
	return rows
}

// executorAuditTail returns a trimmed, single-line tail of a raw report for the audit
// detail — a package-local copy of verifyloop's auditTail.
func executorAuditTail(raw string) string {
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

// auditNative appends exactly one audit line per native-dispatch decision — the journal
// the daily lane-violation sweep (09) reads. Best-effort: an audit-write failure never
// changes the dispatch outcome.
func (l *Loop) auditNative(verb, result string, it loopengine.Item, detail string) {
	_ = deskkit.Log(deskkit.Entry{
		Tool:   "commsloop",
		Verb:   verb,
		Repo:   l.Root,
		Result: result,
		Detail: it.ID + ": " + detail,
	})
}

// gitDetachedWorktreeComms is the default MakeWorktree: a fresh detached worktree under
// the system temp dir at the current HEAD, cleaned up with `git worktree remove
// --force`. Unlike verifyloop's equivalent, a comms-dispatched item carries no
// TargetSHA (it is a routed action, not a brief pinned to a merged SHA) — HEAD is the
// only meaningful anchor here.
func gitDetachedWorktreeComms(it loopengine.Item) (string, func(), error) {
	base, err := os.MkdirTemp("", "commsloop-"+sanitizeExecutorID(it.ID)+"-")
	if err != nil {
		return "", nil, err
	}
	dir := filepath.Join(base, "wt")
	cmd := exec.Command("git", "worktree", "add", "--detach", dir, "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(base)
		return "", nil, fmt.Errorf("git worktree add --detach %s HEAD: %v: %s", dir, err, strings.TrimSpace(string(out)))
	}
	cleanup := func() {
		_ = exec.Command("git", "worktree", "remove", "--force", dir).Run()
		_ = os.RemoveAll(base)
	}
	return dir, cleanup, nil
}

func sanitizeExecutorID(id string) string {
	return strings.NewReplacer("/", "-", " ", "-", ":", "-").Replace(id)
}
