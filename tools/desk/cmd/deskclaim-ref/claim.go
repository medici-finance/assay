package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// out / errOut are the tool's stdout and stderr sinks. They are package vars ONLY so a test
// can capture the exact lines this tool emits — the `show`/`list` output is a wire contract
// its Go readers (desksupervise, deskdispatch) parse, so it must be asserted byte-for-byte.
var (
	out    io.Writer = os.Stdout
	errOut io.Writer = os.Stderr
)

// Exit codes — the deskkit contract, named here so the port reads against the script it
// mirrors (dispatch-claim.sh: OK/REFUSED/UNVERIFIABLE).
const (
	exitOK           = deskkit.ExitOK           // 0
	exitRefused      = deskkit.ExitRefused      // 5
	exitUnverifiable = deskkit.ExitUnverifiable // 6
)

// refPrefix is the claim ref namespace on the forge — IDENTICAL to the bash script's
// REF_PREFIX. See main.go's SCOPE NOTE on why it is not deskkit.ClaimRefsPrefix.
const refPrefix = "refs/dispatch"

// logPrefix keeps the script's exact stdout/stderr line shape (`dispatch-claim: …`). It is
// load-bearing OUTPUT parity, not cosmetic: desksupervise/live.go and
// deskdispatch/dispatch.go parse this tool's `show` output (state=/age=/owner=/branch= and
// the "FREE <key>" marker), and a changed prefix or message body would break those readers.
const logPrefix = "dispatch-claim"

func logf(format string, a ...any) { fmt.Fprintf(out, logPrefix+": "+format+"\n", a...) }
func errf(format string, a ...any) { fmt.Fprintf(errOut, logPrefix+": "+format+"\n", a...) }

// claimedTTL / dispatchedTTL are the two-phase TTLs, env-overridable exactly as the script
// (CLAIMED_TTL_MIN / DISPATCHED_TTL_MIN). state=claimed but never advanced to dispatched is
// dead after the short TTL; state=dispatched after the long one.
func claimedTTL() int    { return envMin("CLAIMED_TTL_MIN", 20) }
func dispatchedTTL() int { return envMin("DISPATCHED_TTL_MIN", 120) }

func envMin(name string, def int) int {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

// --- the gh seam ------------------------------------------------------------
// Every forge call goes through ghRun, exactly as the script shells `gh`. It is a package
// var so tests drive an in-memory forge without a live remote or a real gh binary.

type ghResult struct {
	stdout string
	stderr string
	code   int   // process exit code; 0 = success. -1 = could not be started at all.
	start  error // set only when the process could not be started (gh missing, etc.)
}

func (r ghResult) combined() string {
	return strings.TrimSpace(strings.TrimSpace(r.stdout) + " " + strings.TrimSpace(r.stderr))
}

var ghRun = realGHRun

func realGHRun(args ...string) ghResult {
	cmd := exec.Command("gh", args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	r := ghResult{stdout: strings.TrimSpace(out.String()), stderr: strings.TrimSpace(errb.String())}
	if err == nil {
		return r
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		r.code = ee.ExitCode()
		return r
	}
	r.code = -1
	r.start = err
	return r
}

// ghOut mirrors the script's `x=$(gh … 2>/dev/null) || x=""`: the stdout is used ONLY when
// gh exited 0. gh prints an error BODY to stdout on a 4xx, so gating on the exit status
// rather than on the captured text is what keeps a JSON error blob out of the next call.
func ghOut(args ...string) string {
	r := ghRun(args...)
	if r.code != 0 {
		return ""
	}
	return r.stdout
}

// ghOK reports whether a gh call succeeded (exit 0), for the `>/dev/null 2>&1` probes.
func ghOK(args ...string) bool { return ghRun(args...).code == 0 }

// --- helpers ----------------------------------------------------------------

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// validID rejects keys git itself would reject as a ref component and requires the
// mandatory <repo>-- prefix. It mirrors the bash `valid_id` case patterns exactly.
func validID(id string) bool {
	if !strings.Contains(id, "--") {
		return false
	}
	if strings.ContainsAny(id, " ~^:?*[\\") {
		return false
	}
	if strings.Contains(id, "..") || strings.Contains(id, "@{") {
		return false
	}
	if strings.HasPrefix(id, ".") || strings.HasSuffix(id, ".lock") {
		return false
	}
	return true
}

// fieldOf pulls `key=value` out of a space-separated claim message (bash `field_of`).
func fieldOf(msg, key string) string {
	for _, tok := range strings.Fields(msg) {
		if v, ok := strings.CutPrefix(tok, key+"="); ok {
			return v
		}
	}
	return ""
}

// ageMinutes parses the claim's GitHub-stamped ISO date and returns whole minutes since,
// mirroring the bash `age_minutes`. ok=false when the date is unparseable — the caller
// treats that as unverifiable, never as age 0.
func ageMinutes(iso string) (int, bool) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(iso))
	if err != nil {
		return 0, false
	}
	return int(time.Since(t).Minutes()), true
}

// sanitizeNote mirrors the bash `tr ' \n' '_ ' | tr -d '\n'`: space -> '_', newline -> ' '.
func sanitizeNote(note string) string {
	var b strings.Builder
	for _, r := range note {
		switch r {
		case ' ':
			b.WriteByte('_')
		case '\n':
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// resolveRepo returns owner/name: --repo when given, else the cwd's origin remote via gh.
func resolveRepo(repoFlag string) string {
	if strings.TrimSpace(repoFlag) != "" {
		return strings.TrimSpace(repoFlag)
	}
	return ghOut("repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
}

// --- claim reads ------------------------------------------------------------

// claimStatus is the three-state result of a claim read (bash read_claim's rc).
type claimStatus int

const (
	claimHeld         claimStatus = iota // 0 — a holder exists
	claimFree                            // 1 — no such ref
	claimUnverifiable                    // 6 — the read itself failed
)

// readClaim returns the held claim's tag sha, message and GitHub-stamped date, plus status.
// It mirrors bash read_claim exactly, including the fail-closed distinction between "no such
// ref" (free) and "the API is unreachable" (unverifiable), probed by re-reading the repo.
func readClaim(repo, id string) (objsha, msg, date string, status claimStatus) {
	objsha = ghOut("api", "repos/"+repo+"/git/ref/dispatch/"+id, "--jq", ".object.sha")
	if objsha == "" {
		if ghOK("api", "repos/"+repo, "--jq", ".full_name") {
			return "", "", "", claimFree
		}
		return "", "", "", claimUnverifiable
	}
	payload := ghOut("api", "repos/"+repo+"/git/tags/"+objsha,
		"--jq", `(.message | gsub("[\n\t]"; " ")) + "\t" + .tagger.date`)
	if payload == "" {
		return "", "", "", claimUnverifiable
	}
	// The --jq joins message and date with a tab; gsub already stripped tabs from the
	// message, so the first tab is the separator.
	if i := strings.IndexByte(payload, '\t'); i >= 0 {
		return objsha, payload[:i], strings.TrimSpace(payload[i+1:]), claimHeld
	}
	return "", "", "", claimUnverifiable
}

// branchExists reports whether a recorded branch is already on the remote — the
// branch-as-claim takeover signal (bash branch_exists).
func branchExists(repo, branch string) bool {
	if branch == "" || branch == "-" {
		return false
	}
	return ghOK("api", "repos/"+repo+"/git/ref/heads/"+branch, "--jq", ".ref")
}

// mintClaim creates the annotated tag object carrying the claim's holder/state/branch/note
// and returns its sha. tagger is DELIBERATELY omitted so GitHub stamps tagger.date
// server-side: one clock for every machine, so a racing desk cannot back-date a claim off a
// skewed local clock. ok=false is the unverifiable path (bash rc6).
func mintClaim(repo, id, owner, state, branch, note string) (sha string, ok bool) {
	base := ghOut("api", "repos/"+repo+"/git/ref/heads/main", "--jq", ".object.sha")
	if base == "" {
		return "", false
	}
	msg := "dispatch-claim " + id + " owner=" + owner + " state=" + state + " branch=" + dashIfEmpty(branch)
	if note != "" {
		msg += " note=" + sanitizeNote(note)
	}
	out := ghOut("api", "repos/"+repo+"/git/tags",
		"-f", "tag=dispatch/"+id, "-f", "message="+msg, "-f", "object="+base, "-f", "type=commit", "--jq", ".sha")
	if out == "" {
		return "", false
	}
	return out, true
}

// reportHolder is the DEDUP LOG: a second dispatch attempt must say what it deduplicated
// against (which desk, when), never no-op silently (bash report_holder).
func reportHolder(id, msg, date string) {
	owner := fieldOf(msg, "owner")
	state := fieldOf(msg, "state")
	branch := fieldOf(msg, "branch")
	age, ok := ageMinutes(date)
	ageStr := "?"
	if ok {
		ageStr = strconv.Itoa(age)
	}
	logf("DEDUP %s — already claimed by owner=%s state=%s branch=%s at %s (%sm ago); not dispatching",
		id, dashOrValue(owner), dashOrValue(state), dashIfEmpty(branch), date, ageStr)
}

func dashOrValue(s string) string {
	if s == "" {
		return "?"
	}
	return s
}

// --- verbs ------------------------------------------------------------------

func cmdAcquire(repo, id, owner, branch string) int {
	tagsha, ok := mintClaim(repo, id, owner, "claimed", branch, "")
	if !ok {
		errf("unverifiable: could not mint the claim object in %s", repo)
		return exitUnverifiable
	}
	create := ghRun("api", "repos/"+repo+"/git/refs", "-f", "ref="+refPrefix+"/"+id, "-f", "sha="+tagsha, "--jq", ".ref")
	if create.code == 0 && create.stdout == refPrefix+"/"+id {
		logf("acquired %s (repo=%s owner=%s state=claimed) — %s/%s", id, repo, owner, refPrefix, id)
		return exitOK
	}
	// The create failed. Exactly one benign cause: someone else won the race. Anything else
	// is unverifiable and must fail closed.
	objsha, msg, date, status := readClaim(repo, id)
	_ = objsha
	switch status {
	case claimFree:
		errf("unverifiable: creating %s/%s failed but no claim exists (%s)", refPrefix, id, create.combined())
		return exitUnverifiable
	case claimUnverifiable:
		errf("unverifiable: creating %s/%s failed and the claim could not be read (%s)", refPrefix, id, create.combined())
		return exitUnverifiable
	}
	state := fieldOf(msg, "state")
	hbranch := fieldOf(msg, "branch")
	hage, aok := ageMinutes(date)
	if !aok {
		errf("unverifiable: holder of %s has an unreadable claim date (%s)", id, date)
		return exitUnverifiable
	}
	if branchExists(repo, hbranch) {
		reportHolder(id, msg, date)
		logf("  branch-as-claim: %s exists on the remote — the work is in flight, not stalled", hbranch)
		return exitRefused
	}
	ttl := dispatchedTTL()
	if state == "claimed" {
		ttl = claimedTTL()
	}
	if hage >= ttl {
		logf("stale claim on %s: state=%s age=%dm >= %dm TTL — reclaiming", id, dashOrValue(state), hage, ttl)
		return cmdSteal(repo, id, owner, fmt.Sprintf("TTL: state=%s age=%dm >= %dm", dashOrValue(state), hage, ttl))
	}
	reportHolder(id, msg, date)
	logf("  live (age %dm < %dm TTL for state=%s)", hage, ttl, dashOrValue(state))
	return exitRefused
}

func cmdProgress(repo, id, owner, branch string) int {
	_, msg, _, status := readClaim(repo, id)
	switch status {
	case claimFree:
		errf("refused: %s has no claim to advance (acquire first)", id)
		return exitRefused
	case claimUnverifiable:
		errf("unverifiable: could not read the claim on %s", id)
		return exitUnverifiable
	}
	if holder := fieldOf(msg, "owner"); holder != "" && holder != owner {
		errf("refused: %s is held by %s, not %s — only the holder advances its own claim", id, holder, owner)
		return exitRefused
	}
	tagsha, ok := mintClaim(repo, id, owner, "dispatched", branch, "")
	if !ok {
		errf("unverifiable: could not mint the advanced claim for %s", id)
		return exitUnverifiable
	}
	if !ghOK("api", "-X", "PATCH", "repos/"+repo+"/git/refs/dispatch/"+id, "-f", "sha="+tagsha, "-F", "force=true", "--jq", ".ref") {
		errf("unverifiable: could not advance %s/%s", refPrefix, id)
		return exitUnverifiable
	}
	logf("progressed %s (state=dispatched branch=%s owner=%s) — TTL now %dm", id, dashIfEmpty(branch), owner, dispatchedTTL())
	return exitOK
}

func cmdRelease(repo, id string) int {
	if ghOK("api", "-X", "DELETE", "repos/"+repo+"/git/refs/dispatch/"+id) {
		logf("released %s", id)
		return exitOK
	}
	if _, _, _, status := readClaim(repo, id); status == claimFree {
		logf("released %s (no claim — no-op)", id)
		return exitOK
	}
	errf("unverifiable: could not delete %s/%s", refPrefix, id)
	return exitUnverifiable
}

func cmdSteal(repo, id, owner, reason string) int {
	if reason == "" {
		errf("refused: steal requires --reason (a takeover with no recorded reason is a hand-delete)")
		return exitRefused
	}
	_ = ghRun("api", "-X", "DELETE", "repos/"+repo+"/git/refs/dispatch/"+id)
	tagsha, ok := mintClaim(repo, id, owner, "claimed", "", reason)
	if !ok {
		errf("unverifiable: could not mint the replacement claim for %s", id)
		return exitUnverifiable
	}
	create := ghRun("api", "repos/"+repo+"/git/refs", "-f", "ref="+refPrefix+"/"+id, "-f", "sha="+tagsha, "--jq", ".ref")
	if create.code == 0 && create.stdout == refPrefix+"/"+id {
		logf("stole %s (owner=%s reason=%s)", id, owner, reason)
		return exitOK
	}
	errf("refused: %s was re-claimed by another desk during the steal", id)
	return exitRefused
}

func cmdShow(repo, id string) int {
	_, msg, date, status := readClaim(repo, id)
	switch status {
	case claimFree:
		logf("FREE %s (no %s/%s in %s)", id, refPrefix, id, repo)
		return exitOK
	case claimUnverifiable:
		errf("unverifiable: could not read the claim on %s", id)
		return exitUnverifiable
	}
	ageStr := "?"
	if age, ok := ageMinutes(date); ok {
		ageStr = strconv.Itoa(age)
	}
	logf("HELD %s — %s at=%s age=%sm", id, msg, date, ageStr)
	return exitOK
}

func cmdList(repo string) int {
	refs := ghOut("api", "repos/"+repo+"/git/matching-refs/dispatch/", "--jq", ".[].ref")
	if refs == "" {
		if ghOK("api", "repos/"+repo, "--jq", ".full_name") {
			logf("(no dispatch claims in %s)", repo)
			return exitOK
		}
		errf("unverifiable: could not list dispatch claims in %s", repo)
		return exitUnverifiable
	}
	rc := exitOK
	for _, r := range strings.Split(refs, "\n") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		id := strings.TrimPrefix(r, refPrefix+"/")
		if c := cmdShow(repo, id); c != exitOK {
			rc = c
		}
	}
	return rc
}
