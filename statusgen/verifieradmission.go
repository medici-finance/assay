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
// It returns the attestation line, the brief to read and, for a version-2
// (cross-repository) receipt, the admitted deliverable home the rows must run
// in; a version-1 or unadmitted run returns no home.
func verifierAdmission(root, brief string) (string, string, string, error) {
	required := strings.Contains(strings.ToLower(os.Getenv("DESK_LOOP")), "verify")
	meta, e := exec.Command("git", "-C", root, "rev-parse", "--path-format=absolute", "--git-path", "assay-verifier-attestation.json").Output()
	if e == nil {
		if _, err := os.Stat(strings.TrimSpace(string(meta))); err == nil {
			required = true
		}
	}
	if !required {
		return "", brief, "", nil
	}
	out, err := exec.Command("deskdispatch", "--check-verifier", "--root", root, "--brief", brief).Output()
	if err != nil {
		detail := ""
		if exit, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(exit.Stderr))
		}
		return "", "", "", fmt.Errorf("pre-work verifier admission failed; no Verify rows executed: %w: %s", err, detail)
	}
	return verifierAdmissionReceipt(out, brief)
}

func verifierAdmissionReceipt(out []byte, brief string) (string, string, string, error) {
	var receipt struct {
		Issue      int
		BriefRoot  string
		SourceHome string
		Binding    struct {
			Version                                                           int
			Run, Repo, Source, SourceRepo, TrackingSource, Brief, Model, Tier string
		}
	}
	if err := json.Unmarshal(out, &receipt); err != nil {
		return "", "", "", fmt.Errorf("unreadable pre-work verifier receipt: %w", err)
	}
	b := receipt.Binding
	if receipt.Issue <= 0 || b.Run == "" || b.Source == "" || b.Model == "" || b.Tier == "" {
		return "", "", "", fmt.Errorf("incomplete pre-work verifier receipt")
	}
	if b.Version == 2 {
		if receipt.BriefRoot == "" || receipt.SourceHome == "" || b.Repo == "" || b.SourceRepo == "" || b.TrackingSource == "" || !filepath.IsAbs(receipt.BriefRoot) || !filepath.IsAbs(receipt.SourceHome) || !filepath.IsLocal(b.Brief) || filepath.ToSlash(filepath.Clean(b.Brief)) != b.Brief || b.Brief == "." {
			return "", "", "", fmt.Errorf("incomplete cross-repository pre-work verifier receipt")
		}
		return fmt.Sprintf("Verification-Attestation: %s#%d run=%s source-repo=%s source=%s tracking-source=%s brief=%s model=%s tier=%s", b.Repo, receipt.Issue, b.Run, b.SourceRepo, b.Source, b.TrackingSource, b.Brief, b.Model, b.Tier), filepath.Join(receipt.BriefRoot, b.Brief), receipt.SourceHome, nil
	}
	if b.Version != 1 {
		return "", "", "", fmt.Errorf("unsupported pre-work verifier receipt version")
	}
	return fmt.Sprintf("Verification-Attestation: %s#%d run=%s source=%s brief=%s model=%s tier=%s", b.Repo, receipt.Issue, b.Run, b.Source, b.Brief, b.Model, b.Tier), brief, "", nil
}

// admittedRowRootRefusal is the row-root rule for a version-2 (cross-repository)
// run. Admission swaps the brief to the isolated tracking checkout, so a row
// root derived from the brief's location would be the TRACKING tree, while the
// witness carries an attestation naming the deliverable commit. The rows run in
// the admitted deliverable home and nowhere else: an explicit --root, and the
// root admission was asked about, must each be that directory, or the run is
// refused before any row executes. Directories are compared by file-system
// identity after resolving symlinks, not by spelling.
func admittedRowRootRefusal(explicitRoot, admissionRoot, home string) string {
	if !filepath.IsAbs(home) || !sameDirectory(home, home) {
		return fmt.Sprintf("refusing to run: the admitted deliverable home %q is not an existing absolute directory, so no row root can be bound to it", home)
	}
	if explicitRoot != "" && !sameDirectory(explicitRoot, home) {
		return fmt.Sprintf("refusing --root %s: a cross-repository verifier run executes its Verify rows only in the admitted deliverable home %s, and its witness binds that home's commit. Omit --root, or pass that home.", explicitRoot, home)
	}
	if !sameDirectory(admissionRoot, home) {
		return fmt.Sprintf("refusing to run: admission was checked at %s, which is not the admitted deliverable home %s that the Verify rows would run in", admissionRoot, home)
	}
	return ""
}

// sameDirectory reports whether a and b resolve to one existing directory.
func sameDirectory(a, b string) bool {
	resolve := func(p string) (os.FileInfo, bool) {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, false
		}
		real, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, false
		}
		info, err := os.Stat(real)
		return info, err == nil && info.IsDir()
	}
	ai, ok := resolve(a)
	if !ok {
		return false
	}
	bi, ok := resolve(b)
	return ok && os.SameFile(ai, bi)
}

// admittedRowEnv is the environment an admitted run's Verify rows execute in.
// Admission compares the home with every inherited GIT_* variable removed, so
// the rows must read it the same way: a caller's GIT_DIR, GIT_WORK_TREE or
// GIT_INDEX_FILE (or config injected through GIT_CONFIG_*) would otherwise
// point a row's git at bytes admission never compared, and the run would still
// carry the attestation. The settings admission reads with are added: no
// replacement objects, no system attributes, no system config, no terminal
// prompt and no askpass helper, the last three being what a launch narrows git
// with on purpose. Grep's pattern settings are pinned to git's defaults, so
// neither the home's nor the caller's config can change what a row's git grep
// or git log --grep matches. One inherited GIT_* variable survives, and only in
// a form that narrows what git reads (admittedRowGlobal). Variables the row's
// shell would run or act on at startup are removed (admittedRowShellStartup),
// since they can set any of the above before the row's own command runs.
// Forge credential variables are removed (admittedRowForgeToken), so a row
// holds a forge credential only when its own command line supplies one. The
// invoking process keeps them: its own forge reads (the acceptance-approval
// read) happen there, from os.Getenv, and this function only builds the
// rows' copy.
//
// Out of scope, stated so it is not overclaimed: PATH (which git and which
// tools a row runs), HOME and XDG_CONFIG_HOME (where git finds the user's own
// config, exactly as admission does, and where a forge CLI keeps its own
// stored login) and SSH_ASKPASS pass through unchanged.
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
		case admittedRowForgeToken(key):
		case strings.HasPrefix(strings.ToUpper(key), "GIT_") && !admittedRowGlobal(kv, home):
		default:
			env = append(env, kv)
		}
	}
	return append(env,
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=",
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=grep.patternType", "GIT_CONFIG_VALUE_0=default",
		"GIT_CONFIG_KEY_1=grep.extendedRegexp", "GIT_CONFIG_VALUE_1=false")
}

// admittedRowGlobal reports whether an inherited GIT_* variable is a global
// config setting that only narrows what a row's git reads: the home's own
// ~/.gitconfig (git then skips the XDG one) or none. Any other value (one that
// points git at another file) is removed with the rest.
func admittedRowGlobal(kv, home string) bool {
	key, value, _ := strings.Cut(kv, "=")
	return key == "GIT_CONFIG_GLOBAL" && (value == os.DevNull || (filepath.IsAbs(home) && filepath.Clean(value) == filepath.Join(home, ".gitconfig")))
}

// admittedRowShellStartup reports whether a variable is one a row's shell runs
// or acts on at startup: the file bash sources (BASH_ENV, and ENV in POSIX
// mode), exported functions (BASH_FUNC_*), the options it imports (SHELLOPTS,
// BASHOPTS) and the trace prompt it expands (PS4).
func admittedRowShellStartup(key string) bool {
	key = strings.ToUpper(key)
	return strings.HasPrefix(key, "BASH_FUNC_") || slices.Contains([]string{"BASH_ENV", "ENV", "SHELLOPTS", "BASHOPTS", "PS4"}, key)
}

// admittedRowForgeToken reports whether a variable is one the forge CLI reads
// as a credential: GH_TOKEN and GITHUB_TOKEN for github.com, GH_ENTERPRISE_TOKEN
// and GITHUB_ENTERPRISE_TOKEN for an enterprise host. Matched whatever its case,
// as the GIT_* and shell startup variables are.
func admittedRowForgeToken(key string) bool {
	return slices.Contains([]string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"}, strings.ToUpper(key))
}
