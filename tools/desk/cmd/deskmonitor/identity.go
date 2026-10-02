package main

// identity.go — property A, EXPLICIT IDENTITY.
//
// A background poller inherits the environment of the shell that launched it, and desk work
// routinely exports a role App token into GH_TOKEN. An inherited token that cannot see the repo set
// 404s every read — byte-indistinguishable from "no open issues" once the error is discarded. So
// this verb never reads as whatever the launching shell carried:
//
//   - main() UNSETS GH_TOKEN and GITHUB_TOKEN from its own process before anything else runs, so
//     neither this process nor any child it starts (the token minter) can see them;
//   - the identity each repo is read as is, in order,
//     1. an EXPLICIT read identity the caller names per owner with `--token-file OWNER=PATH` —
//     an owner-only file holding an installation token already minted for that owner, read in
//     place and never copied (the hand-off scanloop makes);
//     2. otherwise THIS SESSION'S App role from the roster ($DESK_LOOP → role → the minted
//     installation token), through deskkit.ForgeFor — the resolver every migrated desk read
//     uses.
//
// The second step is where this verb deliberately departs from the bash oracle, whose fallback is
// the gh keyring account. A Go desk verb never reaches a forge through the gh CLI (the closed forge
// surface, internal/forgeban) and never through an ambient credential (deskkit.ForgeFor refuses to
// build a client without a minted token). A repo whose owner has no token file and whose session
// has no resolvable role is therefore a FAILED read — MONITOR-DEGRADED, loudly — never a read as
// some other identity.
//
// A named token file that cannot be used (missing, not a regular file, not owner-only, unreadable,
// empty) is a precondition failure: the verb never swaps the identity it was told to use for another.

import (
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// tokenFileFlag is the explicit-read-identity flag: `--token-file OWNER=PATH`, one per owner.
const tokenFileFlag = "--token-file"

// tokenFileRole is the role label a token-file owner's reads are resolved under. ForgeFor needs a
// role to route custody; for an owner the caller handed a token file, the custody step below
// answers from that file and never mints, so the label names the identity SOURCE in any refusal
// rather than an App.
const tokenFileRole = "token-file"

var tokenFileArgRe = regexp.MustCompile(`^([A-Za-z0-9._-]+)=(.+)$`)

// splitTokenFiles pulls every `--token-file OWNER=PATH` out of args (it may appear anywhere) and
// returns the owner → path map plus the remaining arguments, in order. Owners compare
// case-insensitively, as the forge does.
func splitTokenFiles(args []string) (map[string]string, []string, error) {
	files := map[string]string{}
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] != tokenFileFlag {
			rest = append(rest, args[i])
			continue
		}
		if i+1 >= len(args) || args[i+1] == "" {
			return nil, nil, precondition("--token-file needs a value (OWNER=PATH)")
		}
		v := args[i+1]
		i++
		m := tokenFileArgRe.FindStringSubmatch(v)
		if m == nil {
			return nil, nil, precondition("--token-file '%s' — expected OWNER=PATH (an installation token belongs to one owner's installation)", v)
		}
		owner := strings.ToLower(m[1])
		if _, dup := files[owner]; dup {
			return nil, nil, precondition("--token-file for owner '%s' given twice", owner)
		}
		files[owner] = m[2]
	}
	return files, rest, nil
}

// readTokenFiles validates and reads every named token file BEFORE any read, in the order the
// scripts and deskclaim-ref check theirs: exists, regular file, owner-only, readable, non-empty.
// Owner-only is the shared custody rule (deskkit.VerifyCustodyOwnerOnly) — mode 0600 on unix, an
// owner-only ACL on Windows, where mode bits are synthetic. A symlink is refused, as the script's
// `find -perm` refuses one (a link's own mode is never owner-only). The token VALUE is never printed.
func readTokenFiles(files map[string]string) (map[string]string, error) {
	tokens := map[string]string{}
	for owner, path := range files {
		target, fi, err := deskkit.LstatCustody(path, deskkit.CustodyNoLinks)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, precondition("cannot read --token-file %s for owner '%s' (no such file)", path, owner)
			}
			return nil, precondition("--token-file %s is group/world accessible or not a plain file; it must be 0600 (%v)", path, err)
		}
		if !fi.Mode().IsRegular() {
			return nil, precondition("--token-file %s is not a regular file", path)
		}
		if verr := deskkit.VerifyCustodyOwnerOnly(target, fi); verr != nil {
			return nil, precondition("--token-file %s is group/world accessible; it must be 0600 (%v)", path, verr)
		}
		b, rerr := os.ReadFile(target)
		if rerr != nil {
			return nil, precondition("cannot read --token-file %s", path)
		}
		tok := strings.Join(strings.Fields(string(b)), "")
		if tok == "" {
			return nil, precondition("--token-file %s is empty", path)
		}
		tokens[owner] = tok
	}
	return tokens, nil
}

var (
	// mintTokenFn is the per-repo App-token lookup for an owner with no token file, swapped in
	// tests for a stub. Production binds it to the shared resolver.
	mintTokenFn = deskkit.GitHubRoleToken
	// sessionRoleFn resolves this session's App role from its loop identity, swapped in tests.
	sessionRoleFn = deskkit.SessionTokenRole
	// forgeAPIBase redirects the resolved GitHub backend at an httptest server in tests; empty in
	// production means the real API host. Read at call time by the custody step.
	forgeAPIBase string
	// explicitTokens is the owner → token map the running cycle was handed (--token-file). The
	// custody step reads it at call time; a cycle installs it before its first read.
	explicitTokens = map[string]string{}
)

func init() {
	// Route ForgeFor's GitHub custody through this verb's identity order — the seam every
	// migrated desk read installs.
	deskkit.SetGitHubCustodyMinter(func(role string, repo deskkit.ForgeRepo) (string, string, error) {
		if tok, ok := explicitTokens[strings.ToLower(repo.Owner)]; ok {
			return tok, forgeAPIBase, nil
		}
		tok, _, err := mintTokenFn(role, repo.Slug())
		if err != nil {
			return "", "", err
		}
		return tok, forgeAPIBase, nil
	})
}

// forgeFor resolves the backend serving repo under the identity order above. It is a package var
// so a test can observe the construction path; production resolves through deskkit.ForgeFor, the
// one constructor in the tree.
var forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name, _ := strings.Cut(repo, "/")
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	role := tokenFileRole
	if _, ok := explicitTokens[strings.ToLower(owner)]; !ok {
		r, _, err := sessionRoleFn("deskmonitor")
		if err != nil {
			return nil, fr, deskkit.Unverifiable("no --token-file for owner '"+strings.ToLower(owner)+
				"' and no App role resolvable for this session — deskmonitor reads as an explicit token file "+
				"or this session's minted App token, never an inherited GH_TOKEN or an ambient gh identity", err)
		}
		role = r
	}
	f, err := deskkit.ForgeFor(fr, role)
	if err != nil {
		return nil, fr, err
	}
	return f, fr, nil
}

// dropInheritedTokens is the first thing main does: whatever App token the launching shell
// exported, this poller is not it.
func dropInheritedTokens() {
	_ = os.Unsetenv("GH_TOKEN")
	_ = os.Unsetenv("GITHUB_TOKEN")
}
