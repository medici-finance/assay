package main

import (
	"os"
	"strings"
	"sync"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
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
func dispatchVerb(args []string, sinks ...func(perfrec.Context)) int {
	var pc perfrec.ContextInput
	var once sync.Once
	selectContext := func() {
		once.Do(func() {
			if len(sinks) > 0 && sinks[0] != nil {
				sinks[0](perfrec.ResolveContext(pc))
			}
		})
	}
	defer selectContext()
	verb := args[0]
	rest := args[1:]

	var id, repoFlag, owner, branch, reason, tokenFile string
	// The optional positional id: the first token, unless it is a flag or empty.
	if len(rest) > 0 && rest[0] != "" && !strings.HasPrefix(rest[0], "-") {
		id = rest[0]
		rest = rest[1:]
	}
	valueFlags := map[string]*string{
		"--repo": &repoFlag, "--owner": &owner, "--branch": &branch, "--reason": &reason, "--token-file": &tokenFile,
	}
	for i := 0; i < len(rest); i++ {
		// A value flag takes its value as the next argument or after "=". The spaced form never
		// takes an empty or blank value or another long option as its value: swallowing the next flag
		// shifts every later argument, so `--repo --branch -` would read "-" as a flag and
		// `--repo --branch=-` would select a claim namespace named "--branch=-" (#6896). A
		// single "-" stays a value: it is the dispatcher's placeholder for a branchless claim.
		name, value, inline := strings.Cut(rest[i], "=")
		if dst, ok := valueFlags[name]; ok {
			if !inline {
				value, i = spacedValue(rest, i)
			}
			if blank(value) {
				errf("refused: %s requires a value", name)
				return exitRefused
			}
			*dst = value
			continue
		}
		switch rest[i] {
		case "-h", "--help":
			return dieUsage()
		default:
			// A performance flag never takes another long option as its value, and a missing
			// value is refused, since either shifts the arguments that follow. An empty or
			// blank value is NOT refused: under the shared performance contract it clears the
			// inherited value and never changes a parser's result (internal/cli/performance.go).
			flag, missing := rest[i], false
			if cli.ManualPerformance(&pc, flag, func() string {
				if i+1 >= len(rest) || strings.HasPrefix(rest[i+1], "--") {
					missing = true
					return ""
				}
				i++
				return rest[i]
			}) {
				if missing {
					errf("refused: %s requires a value", name)
					return exitRefused
				}
				continue
			}
			errf("refused: unknown flag %s", flag)
			return exitRefused
		}
	}

	selectContext()
	// An inferred slug and its forge host must come from the same origin read.
	readOrigin := sync.OnceValues(func() (string, error) { return checkedOriginURL(".") })
	repo, rerr := resolveRepo(repoFlag, readOrigin)
	if rerr != nil || repo == "" {
		errf("unverifiable: could not resolve the target repo (pass --repo owner/name): %v", rerr)
		return exitUnverifiable
	}
	if owner == "" {
		owner = defaultOwner()
	}
	var selected *subjectClaimStore
	if id != "" && workClaimInput(id) {
		catalog, peers, complete, err := loadClaimWork(repo)
		if err != nil {
			errf("unverifiable: work reference source: %v", err)
			return exitUnverifiable
		}
		work, err := catalog.Select(id)
		if err != nil {
			errf("work reference: %v", err)
			return deskkit.ExitCodeOf(err)
		}
		if err := work.Validate(); err != nil {
			errf("work reference: %v", err)
			return deskkit.ExitCodeOf(err)
		}
		if err := deskkit.CheckWorkOperation("claim", peers, complete); err != nil {
			errf("work reference: %v", err)
			return deskkit.ExitCodeOf(err)
		}
		if verb == "release" || verb == "steal" {
			errf("refused: resolving a work reference does not authorize release or takeover; use the existing explicit physical-key operation")
			return exitRefused
		}
		selected = &subjectClaimStore{selected: work, peers: peers, complete: complete, repository: repo}
		id = work.ClaimKey
	}
	// The store is the RESOLVER's answer, never this tool's choice: there is no flag that
	// selects one. The forge-ref opener is installed so that, should the resolver resolve to
	// the legacy store (an unset key, or a valid key naming a store this build does not ship —
	// medici-finance/assay#2753), it is built here with this invocation's credential; a shipped
	// store whose preconditions fail is a refusal (exit 6) and never becomes the forge-ref store.
	deskkit.SetForgeRefClaimStoreOpener(func(r string) (deskkit.ClaimStore, error) {
		return buildStore(r, tokenFile, readOrigin)
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
	if selected != nil {
		selected.ClaimStore = store
		store = selected
	}
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
		// A lease written under the anonymous fallback has no identifiable
		// holder who can release it. Read-only commands and a free release
		// retain their usual behaviour when there is no session.
		if (verb == "acquire" || verb == "progress" || verb == "steal") && (owner == "" || owner == "unknown") {
			errf("refused: %s requires an identified owner; pass --owner or set DESK_SESSION", verb)
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
			return cmdRelease(id, repo, owner)
		case "steal":
			return cmdSteal(id, owner, reason, nil)
		case "show":
			return cmdShow(id)
		}
	}
	errf("refused: unknown verb %s", verb)
	return dieUsage()
}

// blank reports a value that carries nothing but whitespace. It counts as missing: the repo
// and credential resolvers trim a flag value and treat an empty result as unset, falling back
// to the origin remote and the ambient environment credential.
func blank(v string) bool { return strings.TrimSpace(v) == "" }

// spacedValue returns the value after the flag at position i and the advanced index. A
// missing or blank value, or another long option in the value's place, yields "" and leaves
// the index on the flag, so the caller refuses instead of shifting the arguments that follow.
func spacedValue(rest []string, i int) (string, int) {
	if i+1 >= len(rest) || blank(rest[i+1]) || strings.HasPrefix(rest[i+1], "--") {
		return "", i
	}
	return rest[i+1], i + 1
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
