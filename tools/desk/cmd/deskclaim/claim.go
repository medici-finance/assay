package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const toolName = "deskclaim"

// validKinds is the compiled-in set of claim kinds (coverage table).
var validKinds = map[string]bool{
	deskkit.KindDispatch: true,
	deskkit.KindRoute:    true,
	deskkit.KindFile:     true,
	deskkit.KindClose:    true,
	deskkit.KindVerify:   true,
}

// claimsDir resolves ~/.config/assay/claims through deskkit.StateDir, so the CLI, the
// loopengine, and deskroster all read and write ONE directory (a missing HOME is
// Unverifiable — the tool cannot place state it cannot locate).
func claimsDir() (string, error) {
	dir, err := deskkit.StateDir()
	if err != nil {
		return "", deskkit.Unverifiable("cannot resolve desk-tools dir (HOME missing?)", err)
	}
	return filepath.Join(dir, "claims"), nil
}

// resolveOwner returns the claim owner: --owner flag, else $DESK_SESSION, else
// $CLAUDE_SESSION_ID, else "unknown". Owner is attribution, not a correctness axis, so it
// never fails closed on its own.
func resolveOwner(ownerFlag string) string {
	if ownerFlag != "" {
		return ownerFlag
	}
	if s := os.Getenv("DESK_SESSION"); s != "" {
		return s
	}
	return deskkit.SessionTag() // $CLAUDE_SESSION_ID or "unknown"
}

func cmdAcquire(args []string) error {
	fs := flag.NewFlagSet("acquire", flag.ContinueOnError)
	var kind, item, branch, owner, repo string
	fs.StringVar(&kind, "kind", "", "claim kind: dispatch|route|file|close|verify")
	fs.StringVar(&item, "item", "", "item id to claim (the lock key)")
	fs.StringVar(&branch, "branch", "", "optional branch protecting the claim from age-only reclaim")
	fs.StringVar(&owner, "owner", "", "claim owner (default: session id)")
	fs.StringVar(&repo, "repo", "", "git repo whose worktrees prove a --branch is live (default: cwd if it is a git repo)")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(fmt.Sprintf("acquire: %v", err))
	}
	if item == "" {
		return deskkit.Refused("acquire requires --item")
	}
	if kind == "" {
		return deskkit.Refused("acquire requires --kind (one of dispatch, route, file, close, verify)")
	}
	if !validKinds[kind] {
		return deskkit.Refused("acquire: unknown --kind " + kind + " (want one of dispatch, route, file, close, verify)")
	}

	dir, err := claimsDir()
	if err != nil {
		return err
	}
	cfg := deskkit.ClaimConfig{ClaimsDir: dir, StaleClaim: deskkit.DefaultStaleClaim}

	// Build the branch-liveness probe. The beacon signal is keyed on the EXISTING claim's
	// owner (the session that might still be live), not on the acquiring caller, so peek at
	// the on-disk claim for its owner before wiring the probe. A fresh acquire (no existing
	// claim) never calls the probe, so a "" owner here is harmless.
	priorOwner, _ := readClaimRecord(deskkit.ClaimPath(cfg, item))
	probe := &livenessProbe{repo: resolveRepo(repo), beaconDir: resolveBeaconDir(), owner: priorOwner}
	cfg.BranchActive = probe.active

	c := deskkit.Claim{Kind: kind, Item: item, Owner: resolveOwner(owner), Branch: branch}

	res, aerr := deskkit.AcquireDetailed(cfg, c)
	if aerr != nil {
		// Fail closed (#146): a lock we could not hold, or a claim we could not
		// read/write, is Unverifiable (exit 6) — never assumed free.
		audit("acquire", deskkit.ResultUnverifiable, item, aerr.Error())
		return aerr
	}
	if !res.Acquired {
		// A live holder owns it — REFUSE (exit 5). This is the whole point: the caller
		// must NOT proceed to dispatch.
		msg := "refused: item " + item + " is already claimed by a live holder — do not proceed"
		audit("acquire", deskkit.ResultRefused, item, msg)
		return deskkit.Refused(msg)
	}
	if res.Reclaimed {
		// A stale claim was reclaimed in place. The audit line and stdout distinguish this
		// from a fresh acquire so a later sweep can see the reclaim happened through the
		// tool (under the flock), not by a hand-delete.
		detail := fmt.Sprintf("reclaimed age=%dm ttl=%dm prior-owner=%s because=%s kind=%s owner=%s",
			mins(res.Age), mins(deskkit.DefaultStaleClaim), dashIfEmpty(res.PriorOwner), becauseOldNoLiveSignal, kind, c.Owner)
		audit("acquire", deskkit.ResultOK, item, detail)
		fmt.Printf("deskclaim: reclaimed %s (age=%dm prior-owner=%s kind=%s owner=%s)\n",
			item, mins(res.Age), dashIfEmpty(res.PriorOwner), kind, c.Owner)
		return nil
	}
	audit("acquire", deskkit.ResultOK, item, "fresh kind="+kind+" owner="+c.Owner)
	fmt.Printf("deskclaim: acquired %s (fresh, kind=%s owner=%s)\n", item, kind, c.Owner)
	return nil
}

// cmdStale is the READ-ONLY staleness probe. It computes the same verdict Acquire would use
// (deskkit.IsStale with the branch-liveness probe wired) and prints one report line, but it
// NEVER acquires, releases or rewrites — the claim file's bytes and mtime are untouched.
//
// Exit contract: 0 stale (reclaimable), 5 live (do not reclaim), 6 unreadable or missing
// (a missing claim is not stale — there is nothing to reclaim).
func cmdStale(args []string) error {
	fs := flag.NewFlagSet("stale", flag.ContinueOnError)
	var item, repo string
	fs.StringVar(&item, "item", "", "item id to probe")
	fs.StringVar(&repo, "repo", "", "git repo whose worktrees prove a --branch is live (default: cwd if it is a git repo)")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(fmt.Sprintf("stale: %v", err))
	}
	if item == "" {
		return deskkit.Refused("stale requires --item")
	}

	dir, err := claimsDir()
	if err != nil {
		return err
	}
	cfg := deskkit.ClaimConfig{ClaimsDir: dir, StaleClaim: deskkit.DefaultStaleClaim}
	path := deskkit.ClaimPath(cfg, item)

	fi, serr := os.Stat(path)
	if serr != nil {
		if os.IsNotExist(serr) {
			// A missing claim is NOT stale — there is nothing to reclaim. Report it as
			// unreadable/missing (exit 6), never as stale.
			fmt.Printf("item=%s age=-m ttl=%dm branch=- holder=- verdict=unreadable because=no-claim\n",
				item, mins(deskkit.DefaultStaleClaim))
			audit("stale", deskkit.ResultUnverifiable, item, "no claim")
			return deskkit.Unverifiable("stale: no claim for "+item, nil)
		}
		fmt.Printf("item=%s age=-m ttl=%dm branch=- holder=- verdict=unreadable because=stat-error\n",
			item, mins(deskkit.DefaultStaleClaim))
		audit("stale", deskkit.ResultUnverifiable, item, "cannot stat claim: "+serr.Error())
		return deskkit.Unverifiable("stale: cannot stat claim for "+item, serr)
	}

	owner, branch := readClaimRecord(path)
	age := time.Since(fi.ModTime())
	probe := &livenessProbe{repo: resolveRepo(repo), beaconDir: resolveBeaconDir(), owner: owner}
	cfg.BranchActive = probe.active

	// The authoritative verdict comes from the SAME code Acquire uses, so `stale` can never
	// disagree with what an acquire would do. IsStale calls probe.active as a side effect,
	// leaving the reason in probe.because for the report line.
	stale, ierr := deskkit.IsStale(cfg, item)
	if ierr != nil {
		fmt.Printf("item=%s age=%dm ttl=%dm branch=%s holder=%s verdict=unreadable because=read-error\n",
			item, mins(age), mins(deskkit.DefaultStaleClaim), dashIfEmpty(branch), dashIfEmpty(owner))
		audit("stale", deskkit.ResultUnverifiable, item, ierr.Error())
		return ierr
	}

	because := probe.because
	if because == "" {
		// The probe was not consulted: either the age floor held (young) or there is no
		// branch to protect (old, no branch).
		if age <= deskkit.DefaultStaleClaim {
			because = becauseAgeUnderTTL
		} else {
			because = becauseOldNoLiveSignal
		}
	}

	verdict := "live"
	line := fmt.Sprintf("item=%s age=%dm ttl=%dm branch=%s holder=%s verdict=%s because=%s\n",
		item, mins(age), mins(deskkit.DefaultStaleClaim), dashIfEmpty(branch), dashIfEmpty(owner), "%VERDICT%", because)
	if stale {
		verdict = "stale"
	}
	fmt.Print(strings.Replace(line, "%VERDICT%", verdict, 1))

	if stale {
		audit("stale", deskkit.ResultOK, item, "verdict=stale because="+because)
		return nil // exit 0 — reclaimable
	}
	audit("stale", deskkit.ResultOK, item, "verdict=live because="+because)
	return deskkit.Refused("stale: " + item + " is live (because=" + because + ") — do not reclaim")
}

// readClaimRecord reads an existing claim's owner and branch through the tolerant deskkit
// reader. It is a best-effort READ: an absent or unreadable/malformed file yields ("", "").
func readClaimRecord(path string) (owner, branch string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var rec deskkit.Claim
	if json.Unmarshal(b, &rec) != nil {
		return "", ""
	}
	return rec.Owner, rec.Branch
}

// mins renders a duration at whole-minute resolution — the precision the mtime clock the
// staleness decision runs on actually has.
func mins(d time.Duration) int {
	return int(d.Round(time.Minute) / time.Minute)
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func cmdRelease(args []string) error {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	var item string
	fs.StringVar(&item, "item", "", "item id to release")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(fmt.Sprintf("release: %v", err))
	}
	if item == "" {
		return deskkit.Refused("release requires --item")
	}

	dir, err := claimsDir()
	if err != nil {
		return err
	}
	cfg := deskkit.ClaimConfig{ClaimsDir: dir}
	if rerr := deskkit.Release(cfg, item); rerr != nil {
		audit("release", deskkit.ResultUnverifiable, item, rerr.Error())
		return rerr
	}
	audit("release", deskkit.ResultOK, item, "released")
	fmt.Printf("deskclaim: released %s\n", item)
	return nil
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(fmt.Sprintf("list: %v", err))
	}

	dir, err := claimsDir()
	if err != nil {
		return err
	}
	claims, lerr := deskkit.List(deskkit.ClaimConfig{ClaimsDir: dir})
	if lerr != nil {
		audit("list", deskkit.ResultUnverifiable, "", lerr.Error())
		return lerr
	}
	if len(claims) == 0 {
		fmt.Println("(no claims)")
		audit("list", deskkit.ResultOK, "", "0 claims")
		return nil
	}
	sort.Slice(claims, func(i, j int) bool {
		if claims[i].Kind != claims[j].Kind {
			return claims[i].Kind < claims[j].Kind
		}
		return claims[i].Item < claims[j].Item
	})
	fmt.Printf("%-10s %-28s %-16s %s\n", "KIND", "ITEM", "OWNER", "BRANCH")
	fmt.Printf("%-10s %-28s %-16s %s\n", "----", "----", "-----", "------")
	for _, c := range claims {
		branch := c.Branch
		if branch == "" {
			branch = "-"
		}
		fmt.Printf("%-10s %-28s %-16s %s\n", kindOrDash(c.Kind), c.Item, ownerOrDash(c.Owner), branch)
	}
	audit("list", deskkit.ResultOK, "", fmt.Sprintf("%d claims", len(claims)))
	return nil
}

func kindOrDash(s string) string {
	if s == "" {
		return "-" // a legacy claim (loopengine/roster/bash) carries no kind
	}
	return s
}

func ownerOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// audit writes exactly one audit line. A failure to write it is surfaced on stderr,
// never swallowed — the same shape deskkit.writeDisabledAudit uses.
func audit(verb, result, item, detail string) {
	if err := deskkit.Log(deskkit.Entry{
		Tool:   toolName,
		Verb:   verb,
		Result: result,
		Detail: detail,
		Title:  item,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "deskclaim: WARNING: could not write audit line: %v\n", err)
	}
}
