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
	for i := 0; i < len(rest); i++ {
		// A value flag takes `--flag value` or `--flag=value`. The bare `-` is a value (the
		// branch placeholder a branchless claim records); another `--` token never is.
		name, inline, hasInline := rest[i], "", false
		if strings.HasPrefix(name, "--") {
			name, inline, hasInline = strings.Cut(name, "=")
		}
		var dst *string
		switch name {
		case "--repo":
			dst = &repoFlag
		case "--owner":
			dst = &owner
		case "--branch":
			dst = &branch
		case "--reason":
			dst = &reason
		case "--token-file":
			dst = &tokenFile
		case "-h", "--help":
			if !hasInline {
				return dieUsage()
			}
		}
		if dst != nil {
			v, j, ok := flagValue(rest, i, inline, hasInline)
			if !ok {
				errf("refused: %s requires a value", name)
				return exitRefused
			}
			*dst, i = v, j
			continue
		}
		missing := false
		if cli.ManualPerformance(&pc, rest[i], func() string {
			v, j, ok := flagValue(rest, i, "", false)
			missing = !ok
			i = j
			return v
		}) {
			if missing {
				errf("refused: %s requires a value", name)
				return exitRefused
			}
			continue
		}
		errf("refused: unknown flag %s", rest[i])
		return exitRefused
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
			return cmdRelease(id, repo)
		case "steal":
			return cmdSteal(id, owner, reason)
		case "show":
			return cmdShow(id)
		}
	}
	errf("refused: unknown verb %s", verb)
	return dieUsage()
}

// flagValue is the one place a value flag takes its value: the inline `--flag=value` part,
// else the token after position i (returning the advanced index). It mirrors the bash
// script's guard: an absent or empty value, or a next token that is itself a `--` flag, is
// missing (ok=false) and is never consumed. Consuming it turned `--repo --branch -` into
// repo="--branch" and then "unknown flag -", or `--repo --branch=-` into a repository named
// "--branch=-" whose work source has no owning root (#6896). A single `-` is a value.
func flagValue(rest []string, i int, inline string, hasInline bool) (string, int, bool) {
	if hasInline {
		return inline, i, inline != ""
	}
	if i+1 >= len(rest) || rest[i+1] == "" || strings.HasPrefix(rest[i+1], "--") {
		return "", i, false
	}
	return rest[i+1], i + 1, true
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
