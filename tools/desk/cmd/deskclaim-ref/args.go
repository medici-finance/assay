package main

import (
	"os"
	"os/exec"
	"strings"
)

// dispatchVerb parses argv the way the bash script's main() does: <verb> then an optional
// positional <id> (anything not starting with "-") then flags, and routes to the verb.
// It preserves the script's refusal/ordering exactly — gh presence and repo resolution are
// checked before the verb runs, and an unknown flag or verb is a refusal (exit 5).
func dispatchVerb(args []string) int {
	verb := args[0]
	rest := args[1:]

	var id, repoFlag, owner, branch, reason string
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
		case "-h", "--help":
			return dieUsage()
		default:
			errf("refused: unknown flag %s", rest[i])
			return exitRefused
		}
	}

	if _, err := ghLookPath("gh"); err != nil {
		errf("unverifiable: gh is not on PATH")
		return exitUnverifiable
	}
	repo := resolveRepo(repoFlag)
	if repo == "" {
		errf("unverifiable: could not resolve the target repo (pass --repo owner/name)")
		return exitUnverifiable
	}
	if owner == "" {
		owner = defaultOwner()
	}

	switch verb {
	case "list":
		return cmdList(repo)
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
			return cmdAcquire(repo, id, owner, branch)
		case "progress":
			if branch == "" {
				errf("refused: progress requires --branch")
				return exitRefused
			}
			return cmdProgress(repo, id, owner, branch)
		case "release":
			return cmdRelease(repo, id)
		case "steal":
			return cmdSteal(repo, id, owner, reason)
		case "show":
			return cmdShow(repo, id)
		}
	}
	errf("refused: unknown verb %s", verb)
	return dieUsage()
}

// ghLookPath is the PATH probe for the gh binary. A seam ONLY so a test can exercise the
// "gh not on PATH -> unverifiable" refusal without altering the process PATH.
var ghLookPath = exec.LookPath

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
