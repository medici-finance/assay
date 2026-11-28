package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/medici-finance/assay/gitcore"
)

// statusgen is a separate module. Admission delegates to the installed shared
// reader, never duplicates forge custody or accepts a caller-supplied receipt.
// It returns the attestation line, the brief to read and, for a version-2
// (cross-repository) receipt, the admitted deliverable home the rows must run
// in; a version-1 or unadmitted run returns no home.
func verifierAdmission(root, brief string) (string, string, string, string, error) {
	if !verifierAdmissionMarked(root) {
		return "", brief, "", "", nil
	}
	out, err := exec.Command("deskdispatch", "--check-verifier", "--root", root, "--brief", brief).Output()
	if err != nil {
		detail := ""
		if exit, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(exit.Stderr))
		}
		return "", "", "", "", fmt.Errorf("pre-work verifier admission failed; no Verify rows executed: %w: %s", err, detail)
	}
	line, path, home, err := verifierAdmissionReceipt(out, brief)
	if err != nil {
		return "", "", "", "", err
	}
	npm, err := verifierNpmStagingLine(out)
	if err != nil {
		return "", "", "", "", err
	}
	return line, path, home, npm, nil
}

// npmStagingPrefix opens the witness line an admitted run with an npm staging
// record carries (#7572): the npm counterpart of the container bracket's
// Module-Cache line, written directly above the attestation line.
const npmStagingPrefix = "Npm-Staging: "

var npmStagingDigestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// verifierNpmStagingLine is the Npm-Staging witness line for an admission
// receipt, or "" when the run has no npm staging record. Admission has already
// bound the staged directories to this digest; a malformed digest refuses.
func verifierNpmStagingLine(out []byte) (string, error) {
	var receipt struct {
		NpmStaging *struct{ Digest string }
	}
	if err := json.Unmarshal(out, &receipt); err != nil {
		return "", fmt.Errorf("unreadable pre-work verifier receipt: %w", err)
	}
	if receipt.NpmStaging == nil {
		return "", nil
	}
	if !npmStagingDigestRe.MatchString(receipt.NpmStaging.Digest) {
		return "", fmt.Errorf("pre-work verifier receipt carries a malformed npm staging digest")
	}
	return npmStagingPrefix + receipt.NpmStaging.Digest, nil
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
// The scrub is an allow-list. An inherited variable reaches a row only when it
// is on the pass list (admittedRowPass): the search path, the account and
// shell names (USER, LOGNAME, SHELL), the bare CI flag, locale and terminal
// settings, the Go toolchain settings, proxy and trust-root settings and the
// Windows system locations, each by exact name, and the named delivery
// coordinates while each holds a full commit id (admittedRowInput: the
// ASSAY_DELIVERY_* pair and the unprefixed DELIVERY_HEAD/DELIVERY_BASE pair).
// Every other variable is dropped, whatever its family, so a credential pointer nobody has thought to
// name (another forge's or another tool's, an agent socket, a runtime
// directory, a profile location) does not reach a row by default. A listed
// proxy or module proxy whose URL carries a user name is dropped as well. A row
// that reads a dropped variable by name is could-not-run, not fail
// (admittedRowScrubbedRefs), unless the row binds that name itself
// (admittedRowSelfBound): the scrub has still removed the inherited value.
//
// The named refusals below (admittedRowDenied) are checked first, so a name
// added to the pass list later cannot carry one of them through.
// Every variable that leads a row to a stored credential is refused
// (admittedRowCredential): HOME and XDG_CONFIG_HOME (where the forge CLI keeps
// its stored login, a git credential helper is configured and a role key is
// found), DESK_LOOP (the role a minter resolves), every GH_* variable
// (GH_CONFIG_DIR, the forge CLI's config directory, and that CLI's token
// variables among them), every ASSAY_* variable (the methodology's own
// configuration, role key paths included) except the two named delivery
// coordinates when they hold a commit id (admittedRowInput; the run adds one
// checked replay path back after this function, admittedReplayInput), CELLCTL_GH_AMBIENT (a cell's copy
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
// Out of scope, stated so it is not overclaimed: the listed variables pass
// with their values, PATH among them (which git and which tools a row runs).
// The scrub removes the pointers, not the files: a row runs as the same user
// and can still open a stored login or a key file by an absolute path it
// already knows. Several paths stay reachable without any variable: a
// credential helper configured in the checkout's own git config, a home that
// a tool resolves from the user's account record rather than from HOME (ssh
// resolves ~ that way), and the same user's local endpoints (an agent or
// another run's socket) at a path a row already knows or can list. The
// invoking process's own environment also stays reachable: a same-user
// process can generally read another's startup environment (ps -E on macOS,
// /proc/<pid>/environ on Linux), which keeps every variable removed here. A
// row that looks for a credential on purpose can therefore still find one;
// this removal stops one being found by default. Containment is a runner that
// holds no credential but the verifier's own (docs/verify-row-strength.md,
// "What an admitted row can reach"). An unadmitted run keeps the caller's
// environment.
func admittedRowEnv(environ []string) []string {
	var env []string
	for _, kv := range environ {
		switch {
		case admittedRowInput(kv):
			env = append(env, kv)
		case admittedRowDenied(kv):
		case admittedRowGlobal(kv), admittedRowPass(kv):
			env = append(env, kv)
		}
	}
	// The grep pins come from gitcore, the owner that qualifies exactly them
	// for its library reads, so a row's own scoped origin reads accept them.
	env = append(env,
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=")
	return append(env, gitcore.GrepPinEnv()...)
}

// admittedRowPassNames are the inherited variables an admitted row keeps,
// matched by exact name whatever its case, besides the named row inputs, the
// narrowing global git setting and the locale's LC_* settings. Each is a
// search path, an account or shell name (USER, LOGNAME, SHELL), the bare CI
// flag, a locale or terminal setting, a Go toolchain setting the
// container run's module controls depend on, a proxy or trust-root setting, or
// a Windows system location. None names a credential: GOAUTH (a credential
// command), GOENV (a settings file that can set one), the Windows profile
// locations and every family not listed here are dropped.
var admittedRowPassNames = []string{
	"PATH", "USER", "LOGNAME", "SHELL",
	"LANG", "LANGUAGE", "TERM", "COLORTERM", "NO_COLOR", "TZ", "COLUMNS", "LINES", "CI",
	"GOROOT", "GOPATH", "GOCACHE", "GOMODCACHE", "GOFLAGS", "GOTOOLCHAIN", "GOPROXY", "GOSUMDB",
	"GONOSUMDB", "GONOPROXY", "GOPRIVATE", "GOINSECURE", "GOVCS", "GODEBUG", "GOEXPERIMENT",
	"GOOS", "GOARCH", "GOAMD64", "GOARM", "GOARM64", "GO386", "GOWORK", "GOTMPDIR", "GOMAXPROCS",
	"GOGC", "GOMEMLIMIT", "GOTRACEBACK", "GORACE", "GOCOVERDIR", "GOFIPS140", "CGO_ENABLED", "CC", "CXX",
	"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "SSL_CERT_FILE", "SSL_CERT_DIR",
	"SYSTEMROOT", "SYSTEMDRIVE", "WINDIR", "COMSPEC", "PATHEXT", "TEMP", "TMP", "OS",
	"PROCESSOR_ARCHITECTURE", "NUMBER_OF_PROCESSORS", "PROGRAMFILES", "PROGRAMFILES(X86)",
	"PROGRAMW6432", "COMMONPROGRAMFILES", "COMMONPROGRAMFILES(X86)", "COMMONPROGRAMW6432", "PROGRAMDATA",
}

// admittedRowURLNames are the listed variables whose value is a URL a client
// authenticates with when it carries a user name. One that carries any '@'
// is dropped, so a proxy or module-proxy login never rides through on a
// listed name.
var admittedRowURLNames = []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "GOPROXY", "GOSUMDB"}

// admittedRowPass reports whether an inherited variable is on the pass list
// (admittedRowPassNames, or LC_*), and, for a URL-valued one, carries no user
// name. Everything else is dropped: the scrub is an allow-list, so a variable
// family nobody listed never reaches a row by default.
func admittedRowPass(kv string) bool {
	key, value, _ := strings.Cut(kv, "=")
	key = strings.ToUpper(key)
	if slices.Contains(admittedRowURLNames, key) && strings.Contains(value, "@") {
		return false
	}
	return strings.HasPrefix(key, "LC_") || slices.Contains(admittedRowPassNames, key)
}

// admittedRowDenied reports whether an inherited variable is one the scrub
// refuses by name: a shell startup variable, a forge token, a credential
// pointer, an inherited run credential or row-read socket, or a GIT_* variable
// other than the narrowing global setting. It is checked before the pass list,
// so a pass-list entry can never carry one of these through.
func admittedRowDenied(kv string) bool {
	key, _, _ := strings.Cut(kv, "=")
	switch {
	case admittedRowShellStartup(key):
	case admittedRowForgeToken(key):
	case admittedRowCredential(key):
	case strings.EqualFold(key, runCredentialEnv):
	case strings.EqualFold(key, rowreadSocketVar):
	case strings.HasPrefix(strings.ToUpper(key), "GIT_") && !admittedRowGlobal(kv):
	default:
		return false
	}
	return true
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

// admittedRowInputs are the delivery coordinates an admitted row receives as
// named row inputs. The ASSAY_* pair are the only ASSAY_* variables a row still
// gets: the container verify runner (images/verify-runner/witness.sh) exports
// them from its own argv before it starts an admitted run. DELIVERY_HEAD and
// DELIVERY_BASE are the unprefixed pair older consumer-diff Verify rows read
// (the desk-tools briefs' "Consumer-diff provenance" rows); they are on no
// family's deny list, so before the allow-list they passed through, and they
// are kept here under the same rule rather than dropped. All four are commit
// ids, not credentials. Each is named exactly: no prefix, no other case.
var admittedRowInputs = []string{"ASSAY_DELIVERY_SHA", "ASSAY_DELIVERY_BASE_SHA", "DELIVERY_HEAD", "DELIVERY_BASE"}

// admittedRowInput reports whether an inherited variable is one of the named
// row inputs (admittedRowInputs) holding a full commit id, the shape the runner
// itself accepts. Any other value under one of those names is scrubbed with the
// rest of the ASSAY_* family, so no other value rides through under the name.
func admittedRowInput(kv string) bool {
	key, value, _ := strings.Cut(kv, "=")
	return slices.Contains(admittedRowInputs, key) && fullObjectIDRe.MatchString(value)
}

// admittedReplayInputVar names the one file path an admitted row still
// receives: the read-only cohort replay that derived-board/10 Verify row 14
// reads (ruling A on medici-finance/assay#7454). It is not a commit id, so it
// is not one of admittedRowInputs. admittedRowEnv scrubs it with the rest of
// the ASSAY_* family, and the run adds it back after the scrub, only in the
// checked form admittedReplayInput returns.
const admittedReplayInputVar = "ASSAY_COHORT_REPLAY"

// admittedReplayInput returns the replay input an admitted run hands to rows
// that run in root, read from the invoking environment environ by exact name.
// The path is made absolute, its symlinks are resolved, and it must name an
// existing regular file that does not resolve inside root (relWithin). The
// derived-board/10 brief's amendment A sets the same rules for its replay
// paths, and root is the working tree the rows run in. The row receives the resolved path,
// so a relative value cannot read differently from the row's directory and a
// symlink cannot be re-pointed after the check.
//
// With the variable unset it returns neither. With a value that fails a check
// it returns a refusal and no input. The row then reads a scrubbed name and is
// could-not-run (admittedRowScrubbedRefs), never run without its input. The
// file is not opened. Read-only means that admission passes only the path:
// the row runs as the same user, so nothing stops a row from writing to the
// file. A row that does so has to be caught by review of the row's text.
func admittedReplayInput(environ []string, root string) (kv, refusal string) {
	value, found := "", false
	for _, e := range environ {
		if k, v, _ := strings.Cut(e, "="); k == admittedReplayInputVar {
			value, found = v, true
		}
	}
	if !found {
		return "", ""
	}
	if value == "" {
		return "", "it is set but empty"
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", "it cannot be made absolute: " + err.Error()
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", "it cannot be resolved: " + err.Error()
	}
	if info, err := os.Stat(real); err != nil || !info.Mode().IsRegular() {
		return "", fmt.Sprintf("%s is not an existing regular file", real)
	}
	if _, inside := relWithin(root, real); inside {
		return "", fmt.Sprintf("%s resolves inside the working tree %s; replay bytes never live in the tree", real, root)
	}
	return admittedReplayInputVar + "=" + real, ""
}

// admittedRowRefRe matches a row command's reads of a variable by name:
// $env:NAME (pwsh, whatever its case), $NAME, ${NAME}, ${NAME...} (a default,
// a length or an indirection: ${NAME:-x}, ${#NAME}, ${!NAME}) and %NAME% (cmd).
// The pwsh form comes first so $env is not read as a name of its own.
var admittedRowRefRe = regexp.MustCompile(`(?i:\$env:)([A-Za-z_][A-Za-z0-9_]*)|\$\{?[#!]?([A-Za-z_][A-Za-z0-9_]*)|%([A-Za-z_][A-Za-z0-9_]*)%`)

// admittedRowShellVars are the variables a row's own shell (or the run) sets,
// so a row that reads one is not reading an inherited input the scrub removed:
// bash and POSIX shell specials and pwsh's automatic variables that are spelt in
// capitals, plus TMPDIR, which the run sets per row after the scrub.
var admittedRowShellVars = []string{
	"PWD", "OLDPWD", "SHLVL", "IFS", "RANDOM", "SECONDS", "LINENO", "UID", "EUID", "PPID", "BASHPID",
	"HOSTNAME", "HOSTTYPE", "MACHTYPE", "OSTYPE", "FUNCNAME", "PIPESTATUS", "REPLY", "OPTIND", "OPTARG",
	"OPTERR", "MAPFILE", "BASH", "BASH_VERSION", "BASH_SOURCE", "BASH_LINENO", "BASH_REMATCH", "BASH_ARGV",
	"BASH_ARGC", "BASH_COMMAND", "BASH_SUBSHELL", "GROUPS", "PS1", "PS2", "PS3", "PS4", "LASTEXITCODE", "PID",
	"TMPDIR",
}

// admittedRowOnlyReads reports whether every mention of name in the command is
// a read of it (admittedRowRefRe), counting a whole-word mention spelt exactly
// as name is (an English word in a message is not the capitalised name). A
// bare mention beside it (NAME=x, for NAME in, read NAME, printenv NAME) means
// the row may set the name itself, and it is not counted.
func admittedRowOnlyReads(command, name string) bool {
	word := func(b byte) bool {
		return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
	}
	text := command
	mentions := 0
	for i := 0; i < len(text); {
		j := strings.Index(text[i:], name)
		if j < 0 {
			break
		}
		j += i
		end := j + len(name)
		if (j == 0 || !word(text[j-1])) && (end == len(text) || !word(text[end])) {
			mentions++
		}
		i = j + 1
	}
	reads := 0
	for _, m := range admittedRowRefRe.FindAllStringSubmatch(command, -1) {
		if m[1]+m[2]+m[3] == name {
			reads++
		}
	}
	return mentions == reads
}

// admittedRowEnvShaped reports whether a $NAME or ${NAME} read is spelt like an
// inherited variable: capitals, digits and underscores only. A lower-case or
// mixed-case $name is the row's own local, or a pwsh variable such as $true.
func admittedRowEnvShaped(name string) bool {
	return name == strings.ToUpper(name) && !strings.HasPrefix(name, "_")
}

// admittedRowScrubbedRefs returns, in the order first read, the variables a
// row's command reads that its admitted environment env cannot carry, so the
// row cannot run as written under admission (desk decisions 10 and 11 on #6773,
// and the same rule widened to every dropped input on #6802). Two sets count:
//
//   - every ASSAY_* variable the scrub drops, wherever it is read;
//   - any other variable the allow-list drops that a base without it would
//     have passed: not one of the named refusals (admittedRowDenied, dropped
//     before the allow-list too), not a shell or run variable
//     (admittedRowShellVars), spelt in capitals unless read through $env: or
//     %NAME%, and only read, never also assigned or named bare in the row
//     (admittedRowOnlyReads).
//
// A name counts only when admittedRowEnv itself drops it, whatever its value:
// it is probed with a full commit id, the one value a named row input
// (admittedRowInputs) keeps, so the delivery coordinates never count, and an
// abbreviated coordinate still gets the row's own failure. A name the run sets
// after the scrub (its run credential path, for a forge row) is in env and does
// not count.
//
// A name the row binds for itself does not count in either set
// (admittedRowSelfBound, #7134): every read of it supplies its own non-empty
// default or follows the row's own assignment, so the row sees its own value
// and never an inherited one. The scrub still removes the inherited value; this
// only stops the refusal of a row that does not need it.
//
// The rule reads the command's text, not the shell's parse. Any other $NAME,
// ${NAME or %NAME% counts, wherever it sits: inside single quotes, in an echo
// or comment, in a grep pattern, or with an empty default (${NAME:-}). That
// errs to could-not-run. A read written without that syntax (printenv NAME,
// env | grep NAME, or a program the row starts reading the variable itself)
// does not count, and the row runs as before, seeing the variable unset.
func admittedRowScrubbedRefs(command string, env []string) []string {
	var names []string
	for _, m := range admittedRowRefRe.FindAllStringSubmatch(command, -1) {
		name := m[1] + m[2] + m[3]
		if slices.Contains(names, name) {
			continue
		}
		if slices.ContainsFunc(env, func(kv string) bool {
			key, _, _ := strings.Cut(kv, "=")
			return strings.EqualFold(key, name)
		}) {
			continue
		}
		probe := name + "=" + strings.Repeat("0", 40)
		if slices.Contains(admittedRowEnv([]string{probe}), probe) {
			continue
		}
		if admittedRowSelfBound(command, name) {
			continue
		}
		if !strings.HasPrefix(strings.ToUpper(name), "ASSAY_") {
			if admittedRowDenied(probe) || slices.Contains(admittedRowShellVars, strings.ToUpper(name)) ||
				m[2] != "" && !admittedRowEnvShaped(name) || !admittedRowOnlyReads(command, name) {
				continue
			}
		}
		names = append(names, name)
	}
	return names
}

// admittedRowDirs makes one admitted row's own HOME and TMPDIR: fresh, empty
// 0700 directories under a new directory in the run's scratch, so a row's git
// and tools find no user config and no stored login there, and nothing an
// earlier row of the same run left behind. It returns that directory, which
// the caller removes when the row ends, and the two variables to set.
//
// With roster (a container run whose receipt carries the open step's roster
// copy, rostercopy.go) the fresh HOME also holds that copy as its config-home
// roster, and nothing else.
func admittedRowDirs(scratch string, roster []byte) (string, []string, error) {
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
	if roster != nil {
		if err := installRowRoster(home, roster); err != nil {
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
