package main

import (
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// buildStore constructs the forge-ref store from the resolved repo + optional token file, when
// deskkit.ResolveClaimStore resolves to it. It is a package var ONLY so a test can install an
// in-memory forge (mirroring the old ghRun seam) without a live remote. Production always
// builds the go-git store (gogit.go).
var buildStore = newForgeStore

// dispatchVerb parses argv the way the bash script's main() does: <verb> then an optional
// positional <id> (anything not starting with "-") then flags, and routes to the verb.
// It preserves the script's refusal/ordering exactly — repo resolution and the forge/credential
// setup are checked before the verb runs, and an unknown flag or verb is a refusal (exit 5).
func dispatchVerb(args []string) int {
	verb := args[0]
	rest := args[1:]

	var id, repoFlag, owner, branch, reason, tokenFile string
	// The optional positional id: the first token, unless it is a flag or empty.
	if len(rest) > 0 && rest[0] != "" && !strings.HasPrefix(rest[0], "-") {
		id = rest[0]
		rest = rest[1:]
	}
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--repo":
			repoFlag, i = next(rest, i)
		case "--owner":
			owner, i = next(rest, i)
		case "--branch":
			branch, i = next(rest, i)
		case "--reason":
			reason, i = next(rest, i)
		case "--token-file":
			tokenFile, i = next(rest, i)
		case "-h", "--help":
			return dieUsage()
		default:
			errf("refused: unknown flag %s", rest[i])
			return exitRefused
		}
	}

	repo := resolveRepo(repoFlag)
	if repo == "" {
		errf("unverifiable: could not resolve the target repo (pass --repo owner/name)")
		return exitUnverifiable
	}
	if owner == "" {
		owner = defaultOwner()
	}
	// The store is the RESOLVER's answer, never this tool's choice: there is no flag that
	// selects one. The forge-ref opener is installed so that, should the resolver resolve to
	// the legacy store, it is built here with this invocation's credential; a configured store
	// whose preconditions fail is a refusal (exit 6) and never becomes the forge-ref store.
	deskkit.SetForgeRefClaimStoreOpener(func(r string) (deskkit.ClaimStore, error) {
		return buildStore(r, tokenFile)
	})
	res, err := deskkit.ResolveClaimStore(repo)
	if err != nil {
		errf("unverifiable: %s", err.Error())
		return exitUnverifiable
	}
	if res.Store == nil {
		errf("unverifiable: claim store %s resolved for %s but could not be opened", res.Label(), repo)
		return exitUnverifiable
	}
	store = res.Store
	// The removal NOTICE is printed AFTER the verb, so the verb's own first stderr line — the
	// one a dispatcher quotes in its step report — is never displaced by it.
	if res.Notice != "" {
		defer errf("%s", res.Notice)
	}

	switch verb {
	case "list":
		return cmdList()
	case "acquire", "progress", "release", "steal", "show":
		if id == "" {
			errf("refused: %s requires a claim id", verb)
			return exitRefused
		}
		if !validID(id) {
			errf("refused: '%s' is not a valid claim key (want <repo>--<stream>--<NN> or <repo>--issue-<NN>)", id)
			return exitRefused
		}
		switch verb {
		case "acquire":
			return cmdAcquire(id, owner, branch)
		case "progress":
			if branch == "" {
				errf("refused: progress requires --branch")
				return exitRefused
			}
			return cmdProgress(id, owner, branch)
		case "release":
			return cmdRelease(id)
		case "steal":
			return cmdSteal(id, owner, reason)
		case "show":
			return cmdShow(id)
		}
	}
	errf("refused: unknown verb %s", verb)
	return dieUsage()
}

// next returns the value after flag position i and the advanced index. A missing value
// yields "" (the bash `${2:-}`), which the verb-level checks then refuse where required.
func next(rest []string, i int) (string, int) {
	if i+1 < len(rest) {
		return rest[i+1], i + 1
	}
	return "", i
}

func defaultOwner() string {
	for _, env := range []string{"DESK_SESSION", "CLAUDE_SESSION_ID"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	return "unknown"
}

// dieUsage prints the usage and returns the refused exit code (bash die_usage exits REFUSED).
func dieUsage() int {
	errf("%s", usage)
	return exitRefused
}
