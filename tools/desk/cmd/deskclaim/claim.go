package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

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
	var kind, item, branch, owner string
	fs.StringVar(&kind, "kind", "", "claim kind: dispatch|route|file|close|verify")
	fs.StringVar(&item, "item", "", "item id to claim (the lock key)")
	fs.StringVar(&branch, "branch", "", "optional branch protecting the claim from age-only reclaim")
	fs.StringVar(&owner, "owner", "", "claim owner (default: session id)")
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
	c := deskkit.Claim{Kind: kind, Item: item, Owner: resolveOwner(owner), Branch: branch}

	acquired, aerr := deskkit.Acquire(cfg, c)
	if aerr != nil {
		// Fail closed (#146): a lock we could not hold, or a claim we could not
		// read/write, is Unverifiable (exit 6) — never assumed free.
		audit("acquire", deskkit.ResultUnverifiable, item, aerr.Error())
		return aerr
	}
	if !acquired {
		// A live holder owns it — REFUSE (exit 5). This is the whole point: the caller
		// must NOT proceed to dispatch.
		msg := "refused: item " + item + " is already claimed by a live holder — do not proceed"
		audit("acquire", deskkit.ResultRefused, item, msg)
		return deskkit.Refused(msg)
	}
	audit("acquire", deskkit.ResultOK, item, "kind="+kind+" owner="+c.Owner)
	fmt.Printf("deskclaim: acquired %s (kind=%s owner=%s)\n", item, kind, c.Owner)
	return nil
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
