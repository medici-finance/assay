package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// `cellctl hotfix` — the local hotfix overlay's install, status and clear verbs (layer 0 of
// the four the README's "Hotfix overlay" section names). A hotfix is a desk-tools binary built
// from a commit already on main, stamped `<pin>+hotfix-<sha12>`, installed root-owned into the
// overlay directory beside the pinned install, and retired when its pin moves or it expires.
// Per the driver's ruling: install needs sudo, hotfix files are root-owned, only commits on main
// qualify, the default lifetime is HotfixDefaultTTL and HotfixMaxTTL is the ceiling.

// Exit classes: 0 done / valid, 1 refused / invalid, 2 usage, 6 could-not-check.
const (
	hotfixExitRefused = 1
	hotfixExitUsage   = 2
	hotfixExitUnknown = deskkit.ExitUnverifiable
)

const hotfixUsage = `usage:
  sudo cellctl hotfix install <tool>... --source <40-hex sha> [--ttl <dur>] [--ack <text>] [--go <path>]
       cellctl hotfix install <tool>... --source <40-hex sha> --dry-run
       cellctl hotfix status
  sudo cellctl hotfix clear [<tool>...]

install   clone the public repository at <sha> (a commit already on main), build each <tool> as the
          invoking user with a build environment set from scratch, and install it root-owned into
          the overlay directory with a manifest entry. --ttl defaults to 72h; above the ruled
          ceiling (168h) it is refused. --dry-run decides the source rule against the forge only:
          no sudo, no clone, no build.
status    read-only: each overlay entry and whether it is valid. Exit 0 all valid or none,
          1 any invalid (naming why), 6 could-not-check.
clear     remove the named entries (all when none is named) and their binaries.

Shims point at a hotfix only from the next ` + "`cellctl desk`/`cellctl up`" + `; a retired entry is inert
before that, because shim generation and deskboot's binary-pin step both refuse it.
cellctl and deskboot are never hotfix targets. Not supported on Windows.
`

// hotfixCmd is the hotfix verb. Its words are an opaque argv the verb parses itself (like
// defaults): a sole -h/--help is routed to the help verb by runTree.
func hotfixCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "hotfix install|status|clear",
		Short:              "install, inspect or clear a local desk-tools hotfix (install and clear need sudo)",
		Long:               "The local hotfix overlay: a desk-tools binary built from a commit already on main, installed\nroot-owned beside the pinned install, retired when its pin moves or it expires.\n\n" + hotfixUsage,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		Annotations:        map[string]string{noEcho: "1"},
	}
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if code := cmdHotfix(rawArgs(args), os.Stdout, os.Stderr); code != 0 {
			exitWith(code)
		}
		return nil
	}
	return cmd
}

// The hotfix seams. Production keeps every one at its real value; only _test.go files reassign
// them. A binary built with the test-only hotfixseam tag reads deskkit's environment seams
// instead (deskkit.HotfixSeamValue); the release build never passes that tag.
var (
	hotfixForge     = deskkit.ActiveHotfixForge
	hotfixWho       = realHotfixWho
	hotfixCloneURL  = defaultHotfixCloneURL
	hotfixGoProxy   = defaultHotfixGoProxy
	hotfixGoVersion = realHotfixGoVersion
	hotfixRunBuild  = realHotfixRunBuild
)

func defaultHotfixCloneURL() string {
	if v := deskkit.HotfixSeamValue(deskkit.HotfixSeamClone); v != "" {
		return v
	}
	return deskkit.HotfixRepoURL
}

func defaultHotfixGoProxy() string {
	if v := deskkit.HotfixSeamValue(deskkit.HotfixSeamGoProxy); v != "" {
		return v
	}
	return "https://proxy.golang.org,direct"
}

// cmdHotfix dispatches the hotfix sub-verbs and returns the exit code.
func cmdHotfix(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, hotfixUsage)
		return hotfixExitUsage
	}
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprint(stdout, hotfixUsage)
			return 0
		}
	}
	if runtime.GOOS == "windows" {
		fmt.Fprintln(stderr, "cellctl: hotfix: the hotfix overlay is not supported on Windows")
		return hotfixExitRefused
	}
	switch args[0] {
	case "install":
		return hotfixInstall(args[1:], stdout, stderr)
	case "status":
		if len(args) != 1 {
			fmt.Fprint(stderr, hotfixUsage)
			return hotfixExitUsage
		}
		return hotfixStatus(stdout, stderr)
	case "clear":
		return hotfixClear(args[1:], stdout, stderr)
	}
	fmt.Fprint(stderr, hotfixUsage)
	return hotfixExitUsage
}

// hotfixInvoker is who asked for the install: the sudo-invoking user the clone and build run as.
type hotfixInvoker struct {
	User     string
	UID, GID uint32
	// Drop is true when the process runs as real root: the clone and build then run with
	// privileges dropped to UID/GID.
	Drop bool
}

// realHotfixWho is the sudo gate (per the driver's ruling): the effective uid is root,
// SUDO_USER names a non-root user, and the running cellctl is itself a root-owned file under
// the pinned install directory. Every refusal names sudo.
func realHotfixWho() (hotfixInvoker, error) {
	rootUID := uint32(0)
	if v := deskkit.HotfixSeamValue(deskkit.HotfixSeamRootUID); v != "" {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			rootUID = uint32(n)
		}
	}
	euid := os.Geteuid()
	if euid < 0 || uint32(euid) != rootUID {
		return hotfixInvoker{}, fmt.Errorf("needs sudo: run it as `sudo cellctl hotfix ...` (effective uid %d is not root)", euid)
	}
	su := strings.TrimSpace(os.Getenv("SUDO_USER"))
	if su == "" || su == "root" {
		return hotfixInvoker{}, errors.New("needs sudo from a named non-root user: SUDO_USER is unset or root")
	}
	u, err := user.Lookup(su)
	if err != nil {
		return hotfixInvoker{}, fmt.Errorf("needs sudo from a resolvable user: SUDO_USER %q: %v", su, err)
	}
	uid, err1 := strconv.ParseUint(u.Uid, 10, 32)
	gid, err2 := strconv.ParseUint(u.Gid, 10, 32)
	if err1 != nil || err2 != nil || uid == 0 {
		return hotfixInvoker{}, fmt.Errorf("needs sudo from a non-root user: SUDO_USER %q has uid %s", su, u.Uid)
	}
	exe, err := os.Executable()
	if err != nil {
		return hotfixInvoker{}, fmt.Errorf("needs sudo running the pinned cellctl: own path unreadable: %v", err)
	}
	if r, rerr := filepath.EvalSymlinks(exe); rerr == nil {
		exe = r
	}
	pinned := deskkit.DeskToolsBinDir()
	if !deskkit.WithinDir(exe, pinned) {
		return hotfixInvoker{}, fmt.Errorf("needs sudo running the pinned cellctl: %s is not under %s", exe, pinned)
	}
	if oerr := deskkit.HotfixOwnedBy(rootUID)(exe); oerr != nil {
		return hotfixInvoker{}, fmt.Errorf("needs sudo running a root-owned cellctl: %v", oerr)
	}
	return hotfixInvoker{User: su, UID: uint32(uid), GID: uint32(gid), Drop: euid == 0}, nil
}

type hotfixInstallOpts struct {
	tools  []string
	source string
	ttl    time.Duration
	ack    string
	goBin  string
	dryRun bool
}

var hotfixToolName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func parseHotfixInstall(args []string) (hotfixInstallOpts, error) {
	o := hotfixInstallOpts{ttl: deskkit.HotfixDefaultTTL}
	for i := 0; i < len(args); i++ {
		a := args[i]
		val := func() (string, error) {
			if k, v, ok := strings.Cut(a, "="); ok && strings.HasPrefix(k, "--") {
				return v, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s needs a value", a)
			}
			i++
			return args[i], nil
		}
		name, _, _ := strings.Cut(a, "=")
		var err error
		switch name {
		case "--source":
			o.source, err = val()
		case "--ttl":
			var s string
			if s, err = val(); err == nil {
				if o.ttl, err = time.ParseDuration(s); err != nil || o.ttl <= 0 {
					err = fmt.Errorf("--ttl %q is not a positive duration", s)
				}
			}
		case "--ack":
			o.ack, err = val()
		case "--go":
			o.goBin, err = val()
		case "--dry-run":
			o.dryRun = true
		default:
			if strings.HasPrefix(a, "-") {
				return o, fmt.Errorf("unknown flag %s", a)
			}
			o.tools = append(o.tools, a)
		}
		if err != nil {
			return o, err
		}
	}
	if len(o.tools) == 0 {
		return o, errors.New("name at least one tool")
	}
	if o.source == "" {
		return o, errors.New("--source <40-hex sha> is required")
	}
	return o, nil
}

// hotfixInstall runs the install verb. The order is the control: every refusal that needs no
// network or privilege runs first, the source rule before the sudo gate only on --dry-run, and
// nothing is cloned until the invoker, the pin, the source rule and eligibility all hold.
func hotfixInstall(args []string, stdout, stderr io.Writer) int {
	o, err := parseHotfixInstall(args)
	if err != nil {
		fmt.Fprintf(stderr, "cellctl: hotfix install: %v\n%s", err, hotfixUsage)
		return hotfixExitUsage
	}
	for _, t := range o.tools {
		if !hotfixToolName.MatchString(t) {
			fmt.Fprintf(stderr, "cellctl: hotfix install: %q is not a desk tool name\n", t)
			return hotfixExitRefused
		}
		if deskkit.HotfixNeverTarget(t) {
			fmt.Fprintf(stderr, "cellctl: hotfix install: %s is never a hotfix target (it installs, selects or refuses hotfixes, so it always runs the pinned release)\n", t)
			return hotfixExitRefused
		}
	}
	if !deskkit.IsFullSHA(o.source) {
		fmt.Fprintf(stderr, "cellctl: hotfix install: --source %q is not a 40-hex lowercase commit\n", o.source)
		return hotfixExitRefused
	}
	if o.ttl > deskkit.HotfixMaxTTL {
		fmt.Fprintf(stderr, "cellctl: hotfix install: --ttl %s exceeds the ruled ceiling %s (HotfixMaxTTL)\n", o.ttl, deskkit.HotfixMaxTTL)
		return hotfixExitRefused
	}
	forge := hotfixForge()
	if o.dryRun {
		if err := forge.CheckSourceOnMain(o.source); err != nil {
			fmt.Fprintf(stderr, "cellctl: hotfix install --dry-run: %v\n", err)
			return deskkit.HotfixCodeOf(err)
		}
		fmt.Fprintf(stdout, "hotfix dry-run: source %s satisfies the source rule %s (the forge reports main...%s as behind or identical); nothing was cloned, built or installed\n",
			o.source, deskkit.HotfixSourceRule, o.source[:12])
		return 0
	}
	inv, err := hotfixWho()
	if err != nil {
		fmt.Fprintf(stderr, "cellctl: hotfix install %v\n", err)
		return hotfixExitRefused
	}
	cwd, _ := os.Getwd()
	pinRoot := deskkit.NearestPinRoot(cwd)
	pinTag, _, found := deskkit.DeskToolsReleasePin(pinRoot, runtime.GOOS+"-"+runtime.GOARCH)
	if !found {
		fmt.Fprintf(stderr, "cellctl: hotfix install: could-not-check: no desk-tools pin in a .assay-versions at or above %s\n", cwd)
		return hotfixExitUnknown
	}
	pin := bareReleaseTag(pinTag)
	if !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(pin) {
		fmt.Fprintf(stderr, "cellctl: hotfix install: the desk-tools pin %q is not a bare vX.Y.Z release\n", pinTag)
		return hotfixExitRefused
	}
	if err := forge.CheckSourceOnMain(o.source); err != nil {
		fmt.Fprintf(stderr, "cellctl: hotfix install: %v\n", err)
		return deskkit.HotfixCodeOf(err)
	}
	if err := forge.CheckEligible(pin, o.source); err != nil {
		fmt.Fprintf(stderr, "cellctl: hotfix install: %v\n", err)
		return deskkit.HotfixCodeOf(err)
	}
	overlay := deskkit.HotfixOverlayDir()
	owner := hotfixOwner()
	if err := ensureHotfixOverlay(overlay, owner); err != nil {
		fmt.Fprintf(stderr, "cellctl: hotfix install: %v\n", err)
		return deskkit.HotfixCodeOf(err)
	}
	built, err := buildHotfix(o, inv, pin, stderr)
	if built.tmp != "" {
		defer os.RemoveAll(built.tmp)
	}
	if err != nil {
		fmt.Fprintf(stderr, "cellctl: hotfix install: %v\n", err)
		return deskkit.HotfixCodeOf(err)
	}
	m, merr := deskkit.ReadHotfixManifest(overlay)
	if merr != nil && !errors.Is(merr, os.ErrNotExist) {
		fmt.Fprintf(stderr, "cellctl: hotfix install: %v\n", merr)
		return hotfixExitRefused
	}
	if m == nil {
		m = &deskkit.HotfixManifest{}
	}
	tag := deskkit.HotfixTag(pin, o.source)
	for _, tool := range o.tools {
		sum, err := installHotfixBinary(inv, overlay, tool, filepath.Join(built.out, tool), tag)
		if err != nil {
			fmt.Fprintf(stderr, "cellctl: hotfix install: %v\n", err)
			return deskkit.HotfixCodeOf(err)
		}
		e := deskkit.HotfixEntry{
			Tool: tool, SHA256: sum, SourceSHA: o.source, BasePin: pin,
			BuiltAt: built.builtAt, Expires: built.expires, SourceRule: deskkit.HotfixSourceRule,
			Ack: o.ack, InstalledBy: inv.User, GoVersion: built.goVersion, GoSumSHA256: built.goSum,
		}
		kept := m.Entries[:0]
		for _, old := range m.Entries {
			if old.Tool != tool {
				kept = append(kept, old)
			}
		}
		m.Entries = append(kept, e)
	}
	sort.Slice(m.Entries, func(i, j int) bool { return m.Entries[i].Tool < m.Entries[j].Tool })
	if err := deskkit.WriteHotfixManifest(overlay, m); err != nil {
		fmt.Fprintf(stderr, "cellctl: hotfix install: manifest: %v\n", err)
		return hotfixExitRefused
	}
	_ = chownRoot(deskkit.HotfixManifestPath(overlay))
	for _, tool := range o.tools {
		fmt.Fprintf(stdout, "hotfix installed: %s %s over pin %s (source %s, expires %s)\n", tool, tag, pin, o.source[:12], built.expires)
	}
	fmt.Fprintln(stdout, "shims point at it from the next `cellctl desk` / `cellctl up`")
	return 0
}

// bareReleaseTag strips an `<artifact>/` prefix: `desk-tools/v1.0.41` → `v1.0.41`.
func bareReleaseTag(t string) string {
	if i := strings.LastIndex(t, "/"); i >= 0 {
		return t[i+1:]
	}
	return t
}

// ensureHotfixOverlay creates the overlay and its bin directory 0755 when absent and refuses
// when either, or any parent, fails the owner check.
func ensureHotfixOverlay(overlay string, owner deskkit.HotfixOwnerCheck) error {
	for _, d := range []string{overlay, filepath.Join(overlay, "bin")} {
		if _, err := os.Stat(d); errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(d, 0o755); err != nil {
				return &deskkit.HotfixError{Reason: deskkit.HotfixReasonNotRootOwned, Detail: fmt.Sprintf("cannot create %s: %v", d, err), Code: hotfixExitRefused}
			}
			_ = os.Chmod(d, 0o755)
			_ = chownRoot(d)
		}
		if err := owner(d); err != nil {
			return &deskkit.HotfixError{Reason: deskkit.HotfixReasonNotRootOwned, Detail: err.Error(), Code: hotfixExitRefused}
		}
	}
	return nil
}

// chownRoot gives path to root when this process is root; otherwise it is a no-op (a test or
// a hotfixseam build, whose owner check names the running uid instead).
func chownRoot(path string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	return os.Lchown(path, 0, 0)
}

type hotfixBuilt struct {
	tmp, out         string
	builtAt, expires string
	goVersion, goSum string
}

// hotfixBuildSpec is one `go build` the install verb runs: everything the build reads is here.
type hotfixBuildSpec struct {
	Go      string
	Dir     string // the cloned tools/desk module
	Out     string // the output directory
	Tools   []string
	Ldflags string
	Env     []string
	Inv     hotfixInvoker
}

var goVersionLine = regexp.MustCompile(`(?m)^\s*GO_VERSION:\s*"?([0-9]+\.[0-9]+(?:\.[0-9]+)?)"?\s*$`)

func buildHotfix(o hotfixInstallOpts, inv hotfixInvoker, pin string, stderr io.Writer) (hotfixBuilt, error) {
	b := hotfixBuilt{}
	tmp, err := os.MkdirTemp("", "cellctl-hotfix-")
	if err != nil {
		return b, hotfixFail(hotfixExitUnknown, "build dir: %v", err)
	}
	b.tmp = tmp
	for _, d := range []string{"home", "gomodcache", "gocache", "gopath", "tmp", "out"} {
		if err := os.Mkdir(filepath.Join(tmp, d), 0o700); err != nil {
			return b, hotfixFail(hotfixExitUnknown, "build dir: %v", err)
		}
	}
	if inv.Drop {
		if err := chownTree(tmp, inv.UID, inv.GID); err != nil {
			return b, hotfixFail(hotfixExitUnknown, "build dir: %v", err)
		}
	}
	src := filepath.Join(tmp, "src")
	gitEnv := []string{
		"HOME=" + filepath.Join(tmp, "home"), "PATH=/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0",
		"TMPDIR=" + filepath.Join(tmp, "tmp"),
	}
	url := hotfixCloneURL()
	for _, step := range [][]string{
		{"init", "-q", src},
		{"-C", src, "fetch", "-q", "--depth", "1", "--no-tags", url, o.source},
		{"-C", src, "checkout", "-q", "--detach", "FETCH_HEAD"},
	} {
		if out, err := hotfixRun(inv, tmp, gitEnv, "git", step...); err != nil {
			return b, hotfixFail(hotfixExitUnknown, "could-not-check: git %s: %v: %s", step[len(step)-1], err, strings.TrimSpace(string(out)))
		}
	}
	head, err := hotfixRun(inv, tmp, gitEnv, "git", "-C", src, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(head)) != o.source {
		return b, hotfixFail(hotfixExitRefused, "the clone's HEAD %q is not the source %s", strings.TrimSpace(string(head)), o.source)
	}
	mod := filepath.Join(src, "tools", "desk")
	for _, tool := range o.tools {
		if st, err := os.Stat(filepath.Join(mod, "cmd", tool)); err != nil || !st.IsDir() {
			return b, hotfixFail(hotfixExitRefused, "the source %s has no desk tool %s (tools/desk/cmd/%s)", o.source[:12], tool, tool)
		}
	}
	rel, err := os.ReadFile(filepath.Join(src, ".github", "workflows", "release.yml"))
	if err != nil {
		return b, hotfixFail(hotfixExitRefused, "the source has no release workflow to read GO_VERSION from: %v", err)
	}
	mm := goVersionLine.FindSubmatch(rel)
	if mm == nil {
		return b, hotfixFail(hotfixExitRefused, "the source's release workflow names no GO_VERSION")
	}
	b.goVersion = string(mm[1])
	goBin := o.goBin
	if goBin == "" {
		if goBin, err = exec.LookPath("go"); err != nil {
			return b, hotfixFail(hotfixExitRefused, "no go toolchain on PATH (pass --go <path> to the go %s binary)", b.goVersion)
		}
	}
	if abs, err := filepath.Abs(goBin); err == nil {
		goBin = abs
	}
	env := hotfixBuildEnv(tmp, goBin)
	have, err := hotfixGoVersion(inv, goBin, env)
	if err != nil {
		return b, hotfixFail(hotfixExitUnknown, "could-not-check: %s env GOVERSION: %v", goBin, err)
	}
	if have != "go"+b.goVersion {
		return b, hotfixFail(hotfixExitRefused, "go version %s is not the source's release GO_VERSION %s", have, b.goVersion)
	}
	if b.goSum, err = deskkit.FileSHA256(filepath.Join(mod, "go.sum")); err != nil {
		return b, hotfixFail(hotfixExitRefused, "the source's tools/desk/go.sum: %v", err)
	}
	now := hotfixClock().UTC().Truncate(time.Second)
	b.builtAt = now.Format(time.RFC3339)
	b.expires = now.Add(o.ttl).Format(time.RFC3339)
	tag := deskkit.HotfixTag(pin, o.source)
	const pkg = "github.com/medici-finance/assay/tools/desk/internal/deskkit"
	ldflags := strings.Join([]string{
		"-X " + pkg + ".SourceSHA=" + o.source[:12],
		"-X " + pkg + ".BuiltAt=" + b.builtAt,
		"-X " + pkg + ".ReleaseTag=" + tag,
		"-X " + pkg + ".HotfixExpires=" + b.expires,
		"-X main.cellctlVersion=" + tag,
	}, " ")
	b.out = filepath.Join(tmp, "out")
	spec := hotfixBuildSpec{Go: goBin, Dir: mod, Out: b.out, Tools: o.tools, Ldflags: ldflags, Env: env, Inv: inv}
	if err := hotfixRunBuild(spec); err != nil {
		return b, hotfixFail(hotfixExitRefused, "build: %v", err)
	}
	return b, nil
}

// hotfixBuildEnv is the build's environment, set from scratch: nothing the caller exports
// reaches it. A fresh empty module cache, read-only module mode, the default proxy and checksum
// database with every bypass list empty, the local toolchain only, no go.env, no workspace.
func hotfixBuildEnv(tmp, goBin string) []string {
	return []string{
		"HOME=" + filepath.Join(tmp, "home"),
		"PATH=" + filepath.Dir(goBin) + ":/usr/bin:/bin",
		"TMPDIR=" + filepath.Join(tmp, "tmp"),
		"GOPATH=" + filepath.Join(tmp, "gopath"),
		"GOMODCACHE=" + filepath.Join(tmp, "gomodcache"),
		"GOCACHE=" + filepath.Join(tmp, "gocache"),
		"GOFLAGS=-mod=readonly",
		"GOPROXY=" + hotfixGoProxy(),
		"GOSUMDB=sum.golang.org",
		"GONOSUMDB=", "GONOSUMCHECK=", "GOPRIVATE=", "GOINSECURE=", "GONOPROXY=",
		"GOTOOLCHAIN=local",
		"GOENV=off",
		"GOWORK=off",
		"CGO_ENABLED=0",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0",
	}
}

func realHotfixGoVersion(inv hotfixInvoker, goBin string, env []string) (string, error) {
	out, err := hotfixRun(inv, "", env, goBin, "env", "GOVERSION")
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func realHotfixRunBuild(s hotfixBuildSpec) error {
	args := []string{"build", "-trimpath", "-buildvcs=false", "-ldflags", s.Ldflags, "-o", s.Out + string(filepath.Separator)}
	if deskkit.HotfixSeamBuild {
		// Only the end-to-end script's cellctl is a seam build; the release cellctl's constant
		// is false, so a release build never compiles a seam into a hotfix.
		args = append(args, "-tags", "hotfixseam")
	}
	for _, t := range s.Tools {
		args = append(args, "./cmd/"+t)
	}
	out, err := hotfixRun(s.Inv, s.Dir, s.Env, s.Go, args...)
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// hotfixRun runs name as the invoker (privileges dropped when running as root) with exactly env.
func hotfixRun(inv hotfixInvoker, dir string, env []string, name string, args ...string) ([]byte, error) {
	return hotfixRunTimeout(inv, 30*time.Minute, dir, env, name, args...)
}

func hotfixRunTimeout(inv hotfixInvoker, d time.Duration, dir string, env []string, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = dir, env
	cmd.SysProcAttr = hotfixDropAttr(inv)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	return buf.Bytes(), err
}

func chownTree(root string, uid, gid uint32) error {
	return filepath.Walk(root, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(p, int(uid), int(gid))
	})
}

func hotfixFail(code int, format string, a ...any) error {
	return &deskkit.HotfixError{Reason: "hotfix install refused", Detail: fmt.Sprintf(format, a...), Code: code}
}

// installHotfixBinary copies built into a root-only staging file inside the overlay, checks its
// --version prints exactly tag, sha256s it, and renames it into place 0755. The bytes checked
// are the bytes installed: the staging file is never writable by the invoking user.
func installHotfixBinary(inv hotfixInvoker, overlay, tool, built, tag string) (string, error) {
	binDir := filepath.Join(overlay, "bin")
	stage, err := os.CreateTemp(binDir, "."+tool+".staging-")
	if err != nil {
		return "", hotfixFail(hotfixExitRefused, "staging %s: %v", tool, err)
	}
	name := stage.Name()
	defer os.Remove(name)
	src, err := os.Open(built)
	if err != nil {
		stage.Close()
		return "", hotfixFail(hotfixExitRefused, "built %s: %v", tool, err)
	}
	_, cerr := io.Copy(stage, src)
	src.Close()
	if cerr == nil {
		// Root-owned and not writable by anyone else, but executable: the --version check below
		// runs it with the invoking user's privileges, never root's.
		cerr = stage.Chmod(0o755)
	}
	if err := stage.Close(); cerr == nil {
		cerr = err
	}
	if cerr != nil {
		return "", hotfixFail(hotfixExitRefused, "staging %s: %v", tool, cerr)
	}
	_ = chownRoot(name)
	got, err := stagedReleaseTag(inv, name)
	if err != nil {
		return "", hotfixFail(hotfixExitRefused, "staged %s --version: %v", tool, err)
	}
	if got != tag {
		return "", hotfixFail(hotfixExitRefused, "staged %s --version prints release tag %q, want exactly %s", tool, got, tag)
	}
	sum, err := deskkit.FileSHA256(name)
	if err != nil {
		return "", hotfixFail(hotfixExitRefused, "sha256 %s: %v", name, err)
	}
	if err := os.Chmod(name, 0o755); err != nil {
		return "", hotfixFail(hotfixExitRefused, "chmod %s: %v", name, err)
	}
	if err := os.Rename(name, deskkit.HotfixBinaryPath(overlay, tool)); err != nil {
		return "", hotfixFail(hotfixExitRefused, "install %s: %v", tool, err)
	}
	return sum, nil
}

// stagedReleaseTag runs the staged binary's --version as the invoking user (never as root) with
// a minimal environment and reads the release tag it prints: a `releaseTag=` field, else the
// whole trimmed output.
func stagedReleaseTag(inv hotfixInvoker, path string) (string, error) {
	env := []string{"PATH=/usr/bin:/bin", "HOME=" + os.TempDir()}
	if v := os.Getenv("DESK_TOOLS_HOTFIX"); v != "" {
		env = append(env, "DESK_TOOLS_HOTFIX="+v)
	}
	if v := os.Getenv("DESK_TOOLS_BIN"); v != "" {
		env = append(env, "DESK_TOOLS_BIN="+v)
	}
	// The staged binary's own self-check owner-checks the overlay, its bin and the file. Only a
	// test-only seam build (whose hotfix is built with the same tag) names a non-root owner for
	// that check; the release build's constant is false and passes nothing.
	if deskkit.HotfixSeamBuild {
		if v := deskkit.HotfixSeamValue(deskkit.HotfixSeamRootUID); v != "" {
			env = append(env, deskkit.HotfixSeamRootUID+"="+v)
		}
	}
	out, err := hotfixRunTimeout(inv, 30*time.Second, "", env, path, "--version")
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	if t := deskkit.ParseReleaseTagOutput(string(out)); t != "" {
		return t, nil
	}
	return strings.TrimSpace(string(out)), nil
}

// hotfixOverlayTools is every tool the overlay names: manifest entries and binaries alike.
func hotfixOverlayTools(overlay string) []string {
	seen := map[string]bool{}
	if m, err := deskkit.ReadHotfixManifest(overlay); err == nil {
		for _, e := range m.Entries {
			seen[e.Tool] = true
		}
	}
	if es, err := os.ReadDir(filepath.Join(overlay, "bin")); err == nil {
		for _, e := range es {
			if !strings.HasPrefix(e.Name(), ".") {
				seen[e.Name()] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// hotfixStatus is the read-only status verb: every OFFLINE check per overlay tool against the
// pin of the checkout the caller stands in. Exit 0 all valid or none, 1 any invalid, 6
// could-not-check (and 6 wins over 1).
func hotfixStatus(stdout, stderr io.Writer) int {
	overlay := deskkit.HotfixOverlayDir()
	cwd, _ := os.Getwd()
	pinTag, _, _ := deskkit.DeskToolsReleasePin(deskkit.NearestPinRoot(cwd), runtime.GOOS+"-"+runtime.GOARCH)
	pin := bareReleaseTag(pinTag)
	tools := hotfixOverlayTools(overlay)
	if len(tools) == 0 {
		fmt.Fprintf(stdout, "hotfix status: no hotfix in %s\n", overlay)
		return 0
	}
	owner, now, code := hotfixOwner(), hotfixClock(), 0
	for _, tool := range tools {
		e, err := deskkit.ValidateHotfix(overlay, tool, pin, now, owner)
		if errors.Is(err, deskkit.ErrNoHotfix) {
			continue
		}
		if err == nil && deskkit.HotfixNeverTarget(tool) {
			err = &deskkit.HotfixError{Reason: deskkit.HotfixReasonTampered, Detail: tool + " is never a hotfix target", Code: hotfixExitRefused}
		}
		if err != nil {
			fmt.Fprintf(stdout, "hotfix %s: INVALID — %v\n", tool, err)
			if c := deskkit.HotfixCodeOf(err); c > code {
				code = c
			}
			continue
		}
		fmt.Fprintf(stdout, "hotfix %s %s over pin %s (source %s, expires %s, ack %q, installed by %s): valid\n",
			tool, e.Tag(), e.BasePin, e.SourceSHA[:12], e.Expires, e.Ack, e.InstalledBy)
	}
	if code != 0 {
		fmt.Fprintf(stderr, "cellctl: hotfix status: at least one hotfix is not valid (exit %d)\n", code)
	}
	return code
}

// hotfixClear removes the named entries (all when none is named) and their binaries. It needs
// the same sudo gate as install.
func hotfixClear(args []string, stdout, stderr io.Writer) int {
	for _, a := range args {
		if strings.HasPrefix(a, "-") || !hotfixToolName.MatchString(a) {
			fmt.Fprintf(stderr, "cellctl: hotfix clear: %q is not a desk tool name\n%s", a, hotfixUsage)
			return hotfixExitUsage
		}
	}
	if _, err := hotfixWho(); err != nil {
		fmt.Fprintf(stderr, "cellctl: hotfix clear %v\n", err)
		return hotfixExitRefused
	}
	overlay := deskkit.HotfixOverlayDir()
	if _, err := os.Stat(overlay); errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stdout, "hotfix clear: no overlay at %s\n", overlay)
		return 0
	}
	if err := hotfixOwner()(overlay); err != nil {
		fmt.Fprintf(stderr, "cellctl: hotfix clear: %s: %v\n", deskkit.HotfixReasonNotRootOwned, err)
		return hotfixExitRefused
	}
	targets := args
	if len(targets) == 0 {
		targets = hotfixOverlayTools(overlay)
	}
	m, err := deskkit.ReadHotfixManifest(overlay)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		if len(args) != 0 {
			fmt.Fprintf(stderr, "cellctl: hotfix clear: %v (clear every hotfix with a bare `sudo cellctl hotfix clear`)\n", err)
			return hotfixExitRefused
		}
		m = nil // clearing everything: an unreadable manifest is rewritten empty
	}
	if m == nil {
		m = &deskkit.HotfixManifest{}
	}
	drop := map[string]bool{}
	for _, t := range targets {
		drop[t] = true
		if err := os.Remove(deskkit.HotfixBinaryPath(overlay, t)); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(stderr, "cellctl: hotfix clear: %v\n", err)
			return hotfixExitRefused
		}
		fmt.Fprintf(stdout, "hotfix cleared: %s\n", t)
	}
	kept := m.Entries[:0]
	for _, e := range m.Entries {
		if !drop[e.Tool] {
			kept = append(kept, e)
		}
	}
	m.Entries = kept
	if err := deskkit.WriteHotfixManifest(overlay, m); err != nil {
		fmt.Fprintf(stderr, "cellctl: hotfix clear: manifest: %v\n", err)
		return hotfixExitRefused
	}
	_ = chownRoot(deskkit.HotfixManifestPath(overlay))
	return 0
}
