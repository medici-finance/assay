package main

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// This file is a STRENGTHENING added in the assay port of #1556 and has no
// counterpart in the source PR.
//
// In the source, every transport-exec option was refused only INCIDENTALLY — as an
// "unknown flag" by the FlagSet, or (for `deskgit fetch origin --upload-pack=…`, where
// Go's flag package stops parsing at the first non-flag operand) as an "extra operand".
// Both refuse, so the source was not exploitable. But an incidental refusal is a weak
// guard for three reasons, and #1555's whole thesis is that a guard nobody can point at
// is a guard nobody can verify:
//
//  1. it is a SIDE EFFECT of the flag table. Adding any future flag, or switching to a
//     parser that ignores unknown flags rather than erroring, silently reopens the
//     vector with no test failing;
//  2. the refusal reason is generic ("flag provided but not defined", or "takes no
//     positional operands"), so the audit line does not name the vector that was
//     attempted — the operator cannot tell an RCE probe from a typo;
//  3. it cannot be tested BY NAME. A test asserting "some refusal happened" passes for
//     the wrong reason forever.
//
// So the named options below are refused FIRST, explicitly, with their own reason,
// before the FlagSet ever sees the argv. This is defence in depth, not the only layer:
// deskgit still builds a fixed git argv, so none of these could reach git regardless.
//
// The refusal is deliberately over-broad (prefix matching in BOTH directions) because
// git honours unambiguous long-option ABBREVIATIONS — `--upload-p` is `--upload-pack`.
// Fail closed: refusing a flag deskgit does not implement costs nothing.

// deniedTransportExec are the git options that name a program for git to execute, or
// that inject config which can name one. Each is refused by name.
//
//   - upload-pack   — names the program run on the "remote" end. For a local-path
//     remote that end is THIS machine: the proven #1555 RCE.
//   - exec          — git fetch's own alias for --upload-pack. Same vector, other spelling.
//   - receive-pack  — the push-side twin (and `--exec`'s meaning there). deskgit push now
//     PINS `--receive-pack=git-receive-pack` in its own fixed argv (the push-side twin of
//     fetch's upload-pack pin), so a caller-supplied `--receive-pack` is refused here by
//     name — the caller can never override the pinned program by any spelling.
//   - upload-archive — the git-archive-side program-naming option.
//   - config-env / c — config injection. `-c remote.origin.uploadpack=<prog>` reaches
//     upload-pack by another route, and `-c core.sshCommand=…` reaches a shell.
//   - exec-path     — relocates git's own helper directory, so every `git-*` helper git
//     invokes becomes attacker-chosen.
//
// Matching is on the option NAME (leading dashes and any `=value` stripped) against this
// list in both prefix directions, so `--upload-pack=x`, `--upload-p`, `--u`, and
// `--upload-packet` are all refused.
var deniedTransportExec = []string{
	"upload-pack",
	"receive-pack",
	"upload-archive",
	"exec",
	"exec-path",
	"config-env",
}

// deniedShortOpts are single-character options refused by name. Lowercase `c`, `u` and
// `e` already fall out of the prefix rule above (prefixes of config-env, upload-pack and
// exec respectively); uppercase `-C` does not, and needs its own entry: it changes git's
// working directory, which would move the fetch off the very worktree whose origin URL
// deskgit verified — defeating the repo gate rather than the exec guard.
var deniedShortOpts = map[string]string{
	"C": "changes git's working directory, which would move the act off the verified worktree",
}

// deniedPushOpts are options that do not name a program (so they are NOT #1555
// transport-exec vectors — checkTransportExec's message would be wrong for them) but that
// would widen `deskgit push` past its fixed argv: change what the push writes, or skip the
// pre-push hook. Each is refused by NAME with its OWN reason, before the FlagSet, so the
// audit line names the exact property that was attempted and the guard can be tested by
// name rather than passing incidentally as "unknown flag". `receive-pack` is NOT here — it
// names a program and is already refused by deniedTransportExec, which cmdPush also runs.
//
// Matching is on the option name with the same bidirectional prefix rule as the
// transport-exec table, because git honours unambiguous long-option abbreviations
// (`--force` ← `--f` is refused, and so is any extension).
// `force` also covers `--force-with-lease` via the extension half of the prefix rule.
var deniedPushOpts = map[string]string{
	"force":     "would overwrite the remote ref non-fast-forward — deskgit push only fast-forwards the current branch",
	"delete":    "would delete the remote ref — deskgit push only advances the current branch's own ref",
	"prune":     "would delete remote refs absent locally — deskgit push touches only the current branch",
	"mirror":    "would force every local ref onto the remote — deskgit push touches only the current branch",
	"tags":      "would push tags — deskgit push moves only the current branch's ref",
	"no-verify": "would skip the pre-push hook (deskpushguard) — deskgit never bypasses it",
}

// checkPushSafety refuses, BY NAME and before the FlagSet, any argv token naming an option
// that would push past `deskgit push`'s fixed argv (a forced/deleting/hook-skipping push).
// It is the push-side twin of checkTransportExec; cmdPush runs BOTH. Returns nil when no
// token is denied.
func checkPushSafety(args []string) error {
	for _, arg := range args {
		name := optionName(arg)
		if name == "" {
			continue
		}
		for denied, why := range deniedPushOpts {
			if strings.HasPrefix(denied, name) || strings.HasPrefix(name, denied) {
				return deskkit.Refused(fmt.Sprintf(
					"refused: %q names the option --%s, which %s. deskgit push builds a fixed argv "+
						"and passes no caller flag to git; the option is refused by name as well",
					arg, denied, why))
			}
		}
	}
	return nil
}

// optionName reduces an argv token to the option name it denotes, or "" if the token is
// not an option. It strips leading dashes and any `=value` suffix. `-x` and `--x` are
// treated identically because Go's flag package accepts both spellings for the same flag,
// so a guard that only inspected `--` forms would miss half the surface.
func optionName(arg string) string {
	if !strings.HasPrefix(arg, "-") || arg == "-" || arg == "--" {
		return ""
	}
	name := strings.TrimLeft(arg, "-")
	if i := strings.IndexByte(name, '='); i >= 0 {
		name = name[:i]
	}
	return name
}

// checkTransportExec refuses, BY NAME, any argv token naming a transport-exec or
// config-injection option — in ANY position, including after a positional operand where
// Go's flag package would have stopped parsing and reported only a generic
// "extra operand". Returns nil when no token is denied.
func checkTransportExec(args []string) error {
	for _, arg := range args {
		name := optionName(arg)
		if name == "" {
			continue
		}
		if why, ok := deniedShortOpts[name]; ok {
			return deskkit.Refused(fmt.Sprintf(
				"refused: -%s is not accepted — it %s (issue #1555)", name, why))
		}
		for _, denied := range deniedTransportExec {
			// Both directions: `--upload-p` abbreviates the denied option, and
			// `--upload-packet` extends it. Neither is implemented by deskgit, so
			// refusing both is free.
			if strings.HasPrefix(denied, name) || strings.HasPrefix(name, denied) {
				return deskkit.Refused(fmt.Sprintf(
					"refused: %q names the transport-exec/config-injection option --%s, "+
						"which makes git execute a caller-chosen program — the issue #1555 "+
						"vector. deskgit never passes a caller flag to git; the option is "+
						"refused by name as well", arg, denied))
			}
		}
	}
	return nil
}
