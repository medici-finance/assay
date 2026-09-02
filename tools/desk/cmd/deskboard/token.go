package main

// token.go — the identity the READ path authenticates as.
//
// THE DEFECT THIS CLOSES. Every read here shells out to `gh`. With no GH_TOKEN in the
// child's environment `gh` authenticates as whatever account the HOME's keyring holds. On
// a machine whose desk config home is not the operator's own, that account cannot
// authenticate, and the read comes back 401 for every private repo — the board goes blind
// to exactly the queue it exists to surface. The WRITE verbs already resolve a cached App
// installation token and put it in GH_TOKEN; the read path did not. That asymmetry is the
// whole bug: the mint path was wired, the read path just never consumed it.
//
// WHY IT IS WORSE THAN A 401. An unusable keyring account has been observed to make
// GraphQL reads report a bogus rate-limit error, and to make some GraphQL reads return an
// EMPTY list rather than an error at all. An empty result is an absence that reads like an
// answer, so it never trips a fail-closed path. Injecting a token that actually
// authenticates removes the whole class rather than the one symptom that was loud.
//
// PER-OWNER. A GitHub App is installed per account, so a token minted for one account
// resolves only that account's repositories. The cache below is therefore keyed on the
// repo OWNER, and the token is asked for once per owner per process.
//
// AMBIENT IS STILL THE FALLBACK ON THE READ PATH, DELIBERATELY. This is a read-only board;
// a human running it from a plain shell with no loop identity has no App role to mint for,
// and refusing outright would take away a diagnostic that works fine on an operator's own
// credential. So an unresolvable token is a NOTICE on stderr naming the role and the path
// it tried, printed once, and the read proceeds on the ambient identity. The opposite rule
// binds the outward verbs: there an unresolvable token is a refusal, because a write under
// the wrong identity cannot be taken back.

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// sessionTokenRoleFn and ownerTokenFn are the seams the token resolution runs through, so
// a test can exercise the injection without a real App credential. Production binds them
// to the shared deskkit resolvers.
var (
	sessionTokenRoleFn = deskkit.SessionTokenRole
	ownerTokenFn       = deskkit.RoleTokenForOwner
)

// tokenState memoises the per-owner lookup for the life of the process. A failed lookup is
// cached too: the sweep asks for the same owner dozens of times, and re-shelling out to the
// minter on every one of them would turn one missing credential into dozens of subprocesses.
var tokenState = struct {
	mu       sync.Mutex
	roleOnce bool
	role     string
	byOwner  map[string]string // owner → token ("" = looked up, unavailable)
	noticed  map[string]bool   // owner → its NOTICE has been printed
}{byOwner: map[string]string{}, noticed: map[string]bool{}}

// ghTokenForOwner returns the App installation token this session's role holds on owner,
// or "" when there is none to be had. It never returns an error: on the read path an
// unavailable token is a stated degradation, not a dead board.
func ghTokenForOwner(owner string) string {
	if owner == "" {
		return ""
	}
	tokenState.mu.Lock()
	defer tokenState.mu.Unlock()

	if !tokenState.roleOnce {
		tokenState.roleOnce = true
		role, _, err := sessionTokenRoleFn("deskboard")
		if err != nil {
			fmt.Fprintf(os.Stderr, "deskboard: NOTICE reads run on the AMBIENT gh identity — %s. "+
				"Private repos this account cannot see will 401 or come back empty.\n", firstLineOf(err.Error()))
			return ""
		}
		tokenState.role = role
	}
	if tokenState.role == "" {
		return ""
	}
	if tok, seen := tokenState.byOwner[owner]; seen {
		return tok
	}
	tok, path, err := ownerTokenFn(tokenState.role, owner)
	if err != nil {
		tokenState.byOwner[owner] = ""
		if !tokenState.noticed[owner] {
			tokenState.noticed[owner] = true
			fmt.Fprintf(os.Stderr, "deskboard: NOTICE reads of %s repos run on the AMBIENT gh identity — "+
				"the %s App token (%s) could not be resolved: %s\n",
				owner, tokenState.role, displayTokenPath(path), firstLineOf(err.Error()))
		}
		return ""
	}
	tokenState.byOwner[owner] = tok
	return tok
}

// displayTokenPath renders the path a token WOULD have been read from for a message. The
// path is safe to print; the token value never is, and nothing here ever holds one in a
// message.
func displayTokenPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "no token path resolved"
	}
	return path
}

func firstLineOf(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// ownerFromArgs recovers the ACCOUNT a `gh` argv targets, so ghRun can authenticate the
// call as the App installed on that account.
//
// It reads the four shapes this package actually emits, and nothing else:
//
//	-R owner/name / --repo owner/name   `gh pr list`, `gh pr diff`
//	api repos/owner/name/...            every REST read
//	--owner owner                       `gh search prs`
//	-f owner=owner                      the trust gate's GraphQL query
//
// An argv it cannot attribute returns "" and the call runs unauthenticated-as-App rather
// than under a token minted for the WRONG account — a mismatched installation token does
// not 401, it reports "could not resolve to a repository", which reads like a missing repo
// instead of a wrong identity.
func ownerFromArgs(args []string) string {
	for i, a := range args {
		switch {
		case (a == "-R" || a == "--repo") && i+1 < len(args):
			return deskkit.OwnerOf(args[i+1])
		case a == "--owner" && i+1 < len(args):
			return strings.TrimSpace(args[i+1])
		case strings.HasPrefix(a, "-f") && i+1 < len(args) && strings.HasPrefix(args[i+1], "owner="):
			return strings.TrimSpace(strings.TrimPrefix(args[i+1], "owner="))
		case strings.HasPrefix(a, "repos/"):
			parts := strings.Split(a, "/")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}
