package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// statusgen is a separate module. Admission delegates to the installed shared
// reader, never duplicates forge custody or accepts a caller-supplied receipt.
func verifierAdmission(root, brief string) (string, error) {
	required := strings.Contains(strings.ToLower(os.Getenv("DESK_LOOP")), "verify")
	meta, e := exec.Command("git", "-C", root, "rev-parse", "--path-format=absolute", "--git-path", "assay-verifier-attestation.json").Output()
	if e == nil {
		if _, err := os.Stat(strings.TrimSpace(string(meta))); err == nil {
			required = true
		}
	}
	if !required {
		return "", nil
	}
	out, err := exec.Command("deskdispatch", "--check-verifier", "--root", root, "--brief", brief).Output()
	if err != nil {
		detail := ""
		if exit, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(exit.Stderr))
		}
		return "", fmt.Errorf("pre-work verifier admission failed; no Verify rows executed: %w: %s", err, detail)
	}
	var receipt struct {
		Issue   int
		Binding struct{ Run, Repo, Source, Brief, Model, Tier string }
	}
	if err = json.Unmarshal(out, &receipt); err != nil {
		return "", fmt.Errorf("unreadable pre-work verifier receipt: %w", err)
	}
	b := receipt.Binding
	if receipt.Issue <= 0 || b.Run == "" || b.Source == "" || b.Model == "" || b.Tier == "" {
		return "", fmt.Errorf("incomplete pre-work verifier receipt")
	}
	return fmt.Sprintf("Verification-Attestation: %s#%d run=%s source=%s brief=%s model=%s tier=%s", b.Repo, receipt.Issue, b.Run, b.Source, b.Brief, b.Model, b.Tier), nil
}

// admittedRowEnv is the environment an admitted run's Verify rows execute in.
// Admission compares the home with every inherited GIT_* variable removed, so
// the rows must read it the same way: a caller's GIT_DIR, GIT_WORK_TREE or
// GIT_INDEX_FILE (or config injected through GIT_CONFIG_*) would otherwise
// point a row's git at bytes admission never compared, and the run would still
// carry the attestation. Two kinds of variable survive. A GIT_* variable that
// only NARROWS what git reads or does, in exactly that form, is kept, as a
// launch sets it on purpose (admittedRowNarrowing). The two settings admission
// reads with are added, and grep's pattern settings are pinned to git's
// defaults, so neither the home's nor the caller's config can change what a
// row's git grep or git log --grep matches. Variables the row's shell would run
// or act on at startup are removed (admittedRowShellStartup), since they can
// set any of the above before the row's own command runs.
//
// Out of scope, stated so it is not overclaimed: PATH (which git and which
// tools a row runs), HOME and XDG_CONFIG_HOME (where git finds the user's own
// config, exactly as admission does) and SSH_ASKPASS pass through unchanged.
// An unadmitted run keeps the caller's environment.
func admittedRowEnv(environ []string) []string {
	home := ""
	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, "HOME="); ok {
			home = v
		}
	}
	var env []string
	for _, kv := range environ {
		key, _, _ := strings.Cut(kv, "=")
		switch {
		case admittedRowShellStartup(key):
		case strings.HasPrefix(strings.ToUpper(key), "GIT_") && !admittedRowNarrowing(kv, home):
		default:
			env = append(env, kv)
		}
	}
	return append(env,
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1",
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=grep.patternType", "GIT_CONFIG_VALUE_0=default",
		"GIT_CONFIG_KEY_1=grep.extendedRegexp", "GIT_CONFIG_VALUE_1=false")
}

// admittedRowNarrowing reports whether an inherited GIT_* variable, in its
// exact form, only narrows what a row's git reads or does: no system config,
// a global config that is the home's own ~/.gitconfig (git then skips the XDG
// one) or none, no terminal prompt, and no askpass helper. Any other value of
// the same variable (one git would reject, which would fail every row's git, or
// one that widens what git reads) is removed with the rest.
func admittedRowNarrowing(kv, home string) bool {
	key, value, _ := strings.Cut(kv, "=")
	switch key {
	case "GIT_CONFIG_NOSYSTEM":
		return slices.Contains([]string{"1", "true", "yes", "on"}, strings.ToLower(value))
	case "GIT_TERMINAL_PROMPT":
		return slices.Contains([]string{"0", "false", "no", "off"}, strings.ToLower(value))
	case "GIT_ASKPASS":
		return value == ""
	case "GIT_CONFIG_GLOBAL":
		return value == os.DevNull || (filepath.IsAbs(home) && filepath.Clean(value) == filepath.Join(home, ".gitconfig"))
	}
	return false
}

// admittedRowShellStartup reports whether a variable is one a row's shell runs
// or acts on at startup: the file bash sources (BASH_ENV, and ENV in POSIX
// mode), exported functions (BASH_FUNC_*), the options it imports (SHELLOPTS,
// BASHOPTS) and the trace prompt it expands (PS4).
func admittedRowShellStartup(key string) bool {
	key = strings.ToUpper(key)
	return strings.HasPrefix(key, "BASH_FUNC_") || slices.Contains([]string{"BASH_ENV", "ENV", "SHELLOPTS", "BASHOPTS", "PS4"}, key)
}
