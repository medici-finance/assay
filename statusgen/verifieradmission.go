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
	if !verifierAdmissionMarked(root) {
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
//
// Every variable that leads a row to a stored credential is removed too
// (admittedRowCredential): HOME and XDG_CONFIG_HOME (where the forge CLI keeps
// its stored login, a git credential helper is configured and a role key is
// found), DESK_LOOP (the role a minter resolves), every GH_* variable
// (GH_CONFIG_DIR, the forge CLI's config directory, and that CLI's token
// variables among them), every ASSAY_* variable (the methodology's own
// configuration, role key paths included), CELLCTL_GH_AMBIENT (a cell's copy
// of the stored login) and the GITHUB_* token variables (admittedRowForgeToken).
// An inherited row-read socket variable is removed as well: the run names its
// own socket after this function, and no other. The caller gives each row a
// fresh, empty HOME and TMPDIR of its own (admittedRowDirs, withRowVars). This
// function only builds the rows' copy. A row reads
// forge state through the run's own read-only lanes: statusgen rowread, which
// the run answers from its own process (rowreadbroker.go), or the run's
// read-only credential file for a row that passes --acceptance-forge. It never
// reads with a credential of its own.
// The invoking process keeps them in its own environment but no longer
// reads them while admitted: its forge reads and those of a statusgen
// started inside a row use only the verifier's run-scoped read-only
// credential (runcredential.go), never a variable and never a stored forge
// login (storedLoginRefusal, forgecredential.go). Without that credential
// such a read is a could-not-check. An inherited runCredentialEnv is
// removed too (it is an ASSAY_* variable): the run hands its own
// credential path to the one row that needs it, after this function.
//
// Out of scope, stated so it is not overclaimed: PATH (which git and which
// tools a row runs), SSH_AUTH_SOCK (an agent a row's git can authenticate
// through), SSH_ASKPASS, the Windows profile variables, and credential
// variables outside the families above, such as other forges' and other
// tools', pass through unchanged. The scrub removes the
// pointers, not the files: a row runs as the same user and can still open a
// stored login or a key file by an absolute path it already knows. Two more
// paths stay reachable without any variable: a credential helper configured in
// the checkout's own git config, and a home that a tool resolves from the
// user's account record rather than from HOME (ssh resolves ~ that way). The
// invoking process's own environment also stays reachable: a same-user
// process can generally read another's startup environment (ps -E on macOS,
// /proc/<pid>/environ on Linux), which keeps every variable removed here. A
// row that looks for a credential on purpose can therefore still find one;
// this removal stops one being found by default. Containment is a runner that
// holds no credential but the verifier's own. An unadmitted run keeps the
// caller's environment.
func admittedRowEnv(environ []string) []string {
	var env []string
	for _, kv := range environ {
		key, _, _ := strings.Cut(kv, "=")
		switch {
		case admittedRowShellStartup(key):
		case admittedRowForgeToken(key):
		case admittedRowCredential(key):
		case strings.EqualFold(key, runCredentialEnv):
		case strings.EqualFold(key, rowreadSocketVar):
		case strings.HasPrefix(strings.ToUpper(key), "GIT_") && !admittedRowGlobal(kv):
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
// config setting that only narrows what a row's git reads: none at all. Any
// other value (the invoking home's own ~/.gitconfig, which can name a
// credential helper, or another file) is removed with the rest.
func admittedRowGlobal(kv string) bool {
	key, value, _ := strings.Cut(kv, "=")
	return key == "GIT_CONFIG_GLOBAL" && value == os.DevNull
}

// admittedRowCredential reports whether a variable leads a row to a stored
// credential or names the role one is minted for: HOME, XDG_CONFIG_HOME,
// DESK_LOOP, CELLCTL_GH_AMBIENT and every GH_* and ASSAY_* variable. Matched
// whatever its case.
func admittedRowCredential(key string) bool {
	key = strings.ToUpper(key)
	return strings.HasPrefix(key, "GH_") || strings.HasPrefix(key, "ASSAY_") ||
		slices.Contains([]string{"HOME", "XDG_CONFIG_HOME", "DESK_LOOP", "CELLCTL_GH_AMBIENT"}, key)
}

// admittedRowDirs makes one admitted row's own HOME and TMPDIR: fresh, empty
// 0700 directories under a new directory in the run's scratch, so a row's git
// and tools find no user config and no stored login there, and nothing an
// earlier row of the same run left behind. It returns that directory, which
// the caller removes when the row ends, and the two variables to set.
func admittedRowDirs(scratch string) (string, []string, error) {
	dir, err := os.MkdirTemp(scratch, "row-")
	if err != nil {
		return "", nil, err
	}
	home, tmp := filepath.Join(dir, "home"), filepath.Join(dir, "tmp")
	for _, d := range []string{home, tmp} {
		if err := os.Mkdir(d, 0o700); err != nil {
			removeRowScratch(dir)
			return "", nil, err
		}
	}
	return dir, []string{"HOME=" + home, "TMPDIR=" + tmp}, nil
}

// withRowVars returns env with each of vars set, replacing any entry of the
// same name, whatever its case.
func withRowVars(env []string, vars ...string) []string {
	out := make([]string, 0, len(env)+len(vars))
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		if !slices.ContainsFunc(vars, func(v string) bool {
			k, _, _ := strings.Cut(v, "=")
			return strings.EqualFold(k, key)
		}) {
			out = append(out, kv)
		}
	}
	return append(out, vars...)
}

// admittedRowGo pins the Go toolchain's build cache and module path where the
// invoking user's already are (unless the caller set them), so a row that
// builds or tests Go code neither starts cold nor downloads modules into its
// own HOME, where their read-only files would outlive the row. Both are
// resolved in the invoking process, which still holds the original HOME. They
// name a directory under that HOME, so a row can learn its path; a row runs as
// the same user, and PATH usually names it too.
func admittedRowGo(environ []string) []string {
	var add []string
	set := func(key string) bool {
		return slices.ContainsFunc(environ, func(kv string) bool { return strings.HasPrefix(kv, key+"=") })
	}
	if !set("GOCACHE") {
		if dir, err := os.UserCacheDir(); err == nil {
			add = append(add, "GOCACHE="+filepath.Join(dir, "go-build"))
		}
	}
	if !set("GOPATH") {
		if dir, err := os.UserHomeDir(); err == nil {
			add = append(add, "GOPATH="+filepath.Join(dir, "go"))
		}
	}
	return add
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
// and GITHUB_ENTERPRISE_TOKEN for an enterprise host (the two GH_* ones are
// also covered by admittedRowCredential). Matched whatever its case, as the
// GIT_* and shell startup variables are.
func admittedRowForgeToken(key string) bool {
	return slices.Contains([]string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"}, strings.ToUpper(key))
}
