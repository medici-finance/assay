package main

// verifyrun --in-container — run a brief's Verify rows inside the pinned harness
// container (windows-port/10).
//
// WHY THIS EXISTS. `verifyrun` executes each Verify row under `bash -o pipefail`
// (verifyrun.go). On native Windows that shell is often unavailable or is the WSL
// launcher shim, which exits before the row's own command ever runs — issue #1418,
// the defect that made verifyrun record could-not-run for a whole table on a
// Windows host. The fix in #1418 was to STOP calling those rows a `fail`; it did
// not give Windows a way to actually RUN them. This is that way: the supported
// execution-witness runner on Windows is not a native shell at all — it is the
// LINUX harness container, where a real `bash` exists. `--in-container` is the
// thin launcher that runs `statusgen verifyrun` inside that container against the
// checkout bind-mounted at /work, so the witness the brief's Evidence records was
// produced by a real POSIX shell regardless of the host OS.
//
// WHAT IT IS, AND IS NOT. `--in-container` performs NO Verify-row execution on the
// host: it composes a `docker run` argv and execs it. The row execution still
// happens inside `verifyrun` (verifyrun.go's controlled subshell), just in the
// container. So the host statusgen stays a pure launcher here — it never runs a
// Verify command itself, preserving verifyrun.go's invariant that only its own
// subshell path executes lifted markdown commands.
//
// PINNED BY TAG AND DIGEST, NEVER `latest`. The image reference is read from the
// `harness:` block of plugins/assay/paired-versions.yaml and is used by its
// sha256 DIGEST (`<image>@sha256:<64hex>`), not by a floating tag. A `:latest`
// reference, an absent digest, or a placeholder digest is REFUSED (fail-closed,
// exit 2) — the same "never a floating/rolling ref, never a hand-invented hash"
// contract paired-versions.yaml already holds for the release binaries. The tag
// is kept alongside the digest for human/audit legibility only; the digest is
// what docker resolves.
//
// CREDENTIALS — RUNTIME ENV ONLY, NEVER BAKED, NEVER LOGGED. Per the runtime
// credential contract (containers/secrets.md), the ONLY credential surface is the
// operator-supplied role env-file, passed straight through as `--env-file <path>`.
// The wrapper passes the PATH; it never reads, echoes, or logs the file's
// contents, and it bakes nothing into the image or into an argv token. No
// credential is passed on the command line.
//
// HOST-OWNED EVIDENCE. The container writes the witness back into the brief on the
// bind mount. On a POSIX host the wrapper maps `--user <uid>:<gid>` to the
// invoking user so those writes land owned by the HOST user, not by container
// root. On Windows os.Getuid() is -1 (no POSIX uid); `--user` is omitted there and
// Docker Desktop's filesystem sharing maps ownership to the host user itself.
//
// SAFE.DIRECTORY — TREES OWNED BY ANOTHER UID ACROSS THE MOUNT. On a Windows
// Docker backend the checkout bind-mounted at /work lands root-owned while the
// image USER is the unprivileged `desk`, so the inner git refuses the tree
// (`detected dubious ownership`) and attribution fails before any Verify row
// runs. The launcher fixes this WITHOUT widening the image's trust: it passes
// three ephemeral git-config env vars (`GIT_CONFIG_COUNT=1`,
// `GIT_CONFIG_KEY_0=safe.directory`, `GIT_CONFIG_VALUE_0=/work`) so the inner git
// treats EXACTLY /work as safe — never a global `safe.directory=*`, which would
// trust every tree the container ever sees. These are non-secret per-run config
// values, so they ride as inline `-e KEY=VALUE`, not through the credential
// env-file (whose contents the wrapper keeps opaque).
//
// HOST GIT IDENTITY — RESOLVED ON THE HOST, CARRIED NAME-ONLY (#2097). The inner
// verifyrun derives the witness runner from git `user.name`/`user.email`. The bind
// mount carries the checkout's LOCAL config, never the host's global config — and a
// global-only identity is the common Windows setup — so the launcher resolves those
// keys on the host (the value a native host run would read) and carries them as
// further GIT_CONFIG_KEY_<n>/GIT_CONFIG_VALUE_<n> slots. The KEY is inline; the
// VALUE is name-only (`-e GIT_CONFIG_VALUE_<n>`) and lives only in the docker
// client's env, so the identity never appears in the printed argv or a log line.
// A value with a control character is refused on the host. The home directory and
// the global config file are never mounted.
//
// BIND-MOUNT CAVEATS (windows-port/10 deliverable 2), documented so a reader of a
// could-not-run row knows the cause:
//   - NTFS MTIME IS COARSE. A bind-mounted tree carries the host filesystem's
//     mtime resolution (NTFS is ~1s; some sharing backends coarser still). Nothing
//     in this path keys a cache on mtime — verifyrun re-reads the brief every run,
//     and the brief-parse cache keys on CONTENT HASH, not mtime, exactly so a
//     coarse-mtime filesystem cannot serve a stale parse (assay#1410, the
//     parseBriefFile memo-cache-coherency fix). The container path inherits that
//     property; it does not add an mtime dependency of its own.
//   - THE EXEC BIT MAY NOT SURVIVE THE MOUNT. Some host/container/sharing
//     combinations do not preserve the POSIX executable bit across a bind mount, so
//     a Verify row that invokes a repo-local script directly (`./x.sh`) can hit
//     exit 126 (found, not executable). verifyrun already records 126/127 as
//     could-not-run WITH THE REASON (verifyrun.go), never a silent skip and never a
//     forced pass — which is the correct three-state answer: the check did not run,
//     said so, and named why. A row that must stay runnable under the mount invokes
//     its interpreter explicitly (`bash ./x.sh`) rather than relying on the bit.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const (
	// inContainerEnvFileVar is the fallback source for the role env-file when
	// --env-file is not given. Named in the house ASSAY_* convention.
	inContainerEnvFileVar = "ASSAY_VERIFY_ENV_FILE"

	// containerWorkDir is where the checkout is bind-mounted and the working
	// directory is set inside the container.
	containerWorkDir = "/work"

	// pairedVersionsRel is the harness-image pin's home, relative to the repo root.
	pairedVersionsRel = "plugins/assay/paired-versions.yaml"
)

// attributionEnvVars are the process-environment sources executingRunner
// (verifyrun.go) reads to derive the witness runner. The launcher forwards each by
// NAME into the container so the inner verifyrun attributes the run exactly as the
// host would (#2092). A new env source in executingRunner that is not listed here
// fails TestVICAttribEnvClass.
var attributionEnvVars = []string{"GITHUB_ACTIONS", "GITHUB_ACTOR"}

// identityConfigKeys are the git-config keys the witness-runner derivation
// (executingRunner and forgeRoleRunner, verifyrun.go) reads. The container sees
// the checkout's LOCAL config through the bind mount but never the host's global
// or system config, so a host whose identity lives only in global config (the
// common Windows setup) attributed nothing inside the container (#2097). The
// launcher resolves each key ON THE HOST, through the same gitConfigValue the
// derivation uses (local, then global, then system — the value a native host run
// would read), and carries it across as an ephemeral GIT_CONFIG_KEY_<n>/
// GIT_CONFIG_VALUE_<n> pair. A git-config key the derivation reads that is not
// listed here fails TestVICAttribEnvClass.
var identityConfigKeys = []string{"user.name", "user.email"}

// identityPair is one host-resolved git-config entry carried into the container.
type identityPair struct {
	key, value string
}

// hostGitIdentity resolves identityConfigKeys on the host for the checkout at
// root. An unset or empty key is skipped — the launcher never invents an identity,
// and with nothing resolved the inner run still refuses could-not-attribute. A
// value carrying a control character (newline, carriage return, backspace, …)
// cannot be carried across intact and could forge a line in anything that renders
// it, so it is REFUSED here, on the host, before docker runs; the error names the
// key and never the value.
func hostGitIdentity(root string) ([]identityPair, error) {
	var out []identityPair
	for _, key := range identityConfigKeys {
		v := gitConfigValue(root, key)
		if v == "" {
			continue
		}
		if strings.IndexFunc(v, unicode.IsControl) >= 0 {
			return nil, fmt.Errorf("the host git identity `%s` contains a control character, so it cannot be carried into the container intact — fix `git config %s` and re-run (the value is not printed)", key, key)
		}
		out = append(out, identityPair{key: key, value: v})
	}
	return out, nil
}

// identityValueVar is the container env var that carries identity pair i. Slot 0
// is the safe.directory pair, so identity pairs start at 1.
func identityValueVar(i int) string { return fmt.Sprintf("GIT_CONFIG_VALUE_%d", i+1) }

// dockerClientEnv is the environment the docker CLIENT runs with: base plus one
// GIT_CONFIG_VALUE_<n>=<value> per identity pair, replacing any same-named entry
// in base. composeDockerArgs names those variables name-only (`-e NAME`), so docker
// copies the values from here into the container and they never appear in the
// argv — which the launcher prints — or in any log line. PURE, like
// composeDockerArgs, so a test can pin it without a real docker.
func dockerClientEnv(base []string, identity []identityPair) []string {
	if len(identity) == 0 {
		return base
	}
	set := map[string]string{}
	for i, p := range identity {
		set[identityValueVar(i)] = p.value
	}
	env := make([]string, 0, len(base)+len(set))
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if _, dup := set[k]; !dup {
			env = append(env, kv)
		}
	}
	for i := range identity {
		env = append(env, identityValueVar(i)+"="+set[identityValueVar(i)])
	}
	return env
}

// pinDigestRe is the ONLY accepted digest shape: a full sha256. A placeholder
// (`PENDING-HARVEST`), a truncated hash, or an upper-cased one all fail it, so the
// wrapper refuses rather than run an unpinned or hand-invented reference.
var pinDigestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// harnessPin is the pinned witness-runner container image.
type harnessPin struct {
	Image  string
	Tag    string
	Digest string
}

// ref is the digest-pinned reference docker resolves. The digest, not the tag, is
// authoritative: `<image>@sha256:<64hex>`.
func (p harnessPin) ref() string { return p.Image + "@" + p.Digest }

// validate fails closed on anything that is not a concrete, digest-pinned,
// non-latest reference. Every branch names what to fix.
func (p harnessPin) validate() error {
	if strings.TrimSpace(p.Image) == "" {
		return fmt.Errorf("harness image is empty in %s (`harness.image:`) — nothing to run", pairedVersionsRel)
	}
	// A `latest` (or any floating) tag on the image field is refused outright: the
	// whole point of the pin is that the reference is immutable.
	if strings.HasSuffix(p.Image, ":latest") || strings.Contains(p.Image, ":latest@") {
		return fmt.Errorf("harness image %q is pinned to `latest` — a floating tag is refused; pin by digest", p.Image)
	}
	if strings.TrimSpace(p.Digest) == "" {
		return fmt.Errorf("harness image has no digest in %s (`harness.digest:`) — refusing to run an un-digest-pinned image; harvest the release image's `sha256:` digest and pin it", pairedVersionsRel)
	}
	if !pinDigestRe.MatchString(p.Digest) {
		return fmt.Errorf("harness digest %q is not a full `sha256:<64 hex>` (a placeholder, truncated, or upper-cased digest is refused, never run) — harvest the real image digest and pin it", p.Digest)
	}
	return nil
}

// pairedVersionsHarness is the minimal view of paired-versions.yaml this reader
// needs; yaml.Unmarshal ignores every other key (statusgen, desk-tools, …), so
// adding the `harness:` block does not disturb the existing manifest consumers.
type pairedVersionsHarness struct {
	Harness struct {
		Image  string `yaml:"image"`
		Tag    string `yaml:"tag"`
		Digest string `yaml:"digest"`
	} `yaml:"harness"`
}

// readHarnessPin reads the harness-image pin from paired-versions.yaml under root.
// A missing file, an unreadable one, or an absent `harness:` block is an error —
// could-not-check is a failure here, never a quiet default to some image.
func readHarnessPin(root string) (harnessPin, error) {
	path := filepath.Join(root, filepath.FromSlash(pairedVersionsRel))
	raw, err := os.ReadFile(path)
	if err != nil {
		return harnessPin{}, fmt.Errorf("cannot read the harness-image pin (%s): %w", pairedVersionsRel, err)
	}
	var doc pairedVersionsHarness
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return harnessPin{}, fmt.Errorf("cannot parse %s: %w", pairedVersionsRel, err)
	}
	if strings.TrimSpace(doc.Harness.Image) == "" && strings.TrimSpace(doc.Harness.Digest) == "" {
		return harnessPin{}, fmt.Errorf("%s has no `harness:` block — the witness-runner container image is unpinned; add `harness: {image, tag, digest}`", pairedVersionsRel)
	}
	return harnessPin{
		Image:  strings.TrimSpace(doc.Harness.Image),
		Tag:    strings.TrimSpace(doc.Harness.Tag),
		Digest: strings.TrimSpace(doc.Harness.Digest),
	}, nil
}

// containerInvocation is everything composeDockerArgs needs, resolved once.
type containerInvocation struct {
	pin      harnessPin
	root     string   // absolute host checkout to bind-mount
	briefRel string   // brief path relative to root (resolves under /work)
	envFile  string   // role env-file path, or "" to omit --env-file
	inner    []string // the command run inside the container
	uid, gid int      // host uid/gid, or <0 to omit --user (Windows)
	// identity is the host-resolved git identity (hostGitIdentity). Only its KEYS
	// reach the argv; its values ride in the docker client env (dockerClientEnv).
	identity []identityPair
}

// composeDockerArgs builds the `docker run …` argv. It is PURE (no I/O, no
// exec) so a test can assert exactly what the wrapper would invoke docker with —
// the image ref (digest-pinned, not latest), the bind mount, the workdir, the
// --user mapping, and the --env-file passthrough — without a real Docker.
func composeDockerArgs(inv containerInvocation) []string {
	argv := []string{"run", "--rm"}
	argv = append(argv, "-v", inv.root+":"+containerWorkDir)
	argv = append(argv, "-w", containerWorkDir)
	// Mark EXACTLY the bind-mounted /work tree safe for the inner git. On a
	// Windows Docker backend the bind mount lands root-owned while the image USER
	// is the unprivileged `desk`, so git otherwise refuses the tree
	// (`detected dubious ownership`) and attribution fails. These three ephemeral
	// GIT_CONFIG_* env vars set `safe.directory=/work` for the inner git only —
	// scoped to the one mount, never a global `safe.directory=*` that would trust
	// every tree the container sees. Passed as inline `-e KEY=VALUE` (not through
	// the credential env-file, which is contents-opaque) precisely because these
	// are non-secret, per-run config values.
	argv = append(argv, "-e", fmt.Sprintf("GIT_CONFIG_COUNT=%d", 1+len(inv.identity)))
	argv = append(argv, "-e", "GIT_CONFIG_KEY_0=safe.directory")
	argv = append(argv, "-e", "GIT_CONFIG_VALUE_0="+containerWorkDir)
	// Carry the host-resolved git identity (#2097) in the same ephemeral config
	// slots. The KEY is a fixed name from identityConfigKeys and rides inline; the
	// VALUE is NAME-ONLY (`-e GIT_CONFIG_VALUE_<n>`), copied by docker from the
	// client env dockerClientEnv builds, so the identity never appears in this argv
	// — which the launcher prints — and a value shaped like an option (`--x`) or
	// carrying `=` can never be parsed as anything but a value.
	for i, p := range inv.identity {
		argv = append(argv, "-e", fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", i+1, p.key))
		argv = append(argv, "-e", identityValueVar(i))
	}
	// Carry the host's attribution environment into the container (#2092). The
	// inner verifyrun derives the witness runner from the process (executingRunner,
	// verifyrun.go); without these the container sees no GITHUB_ACTOR, and on a
	// runner whose checkout carries no git identity it refuses could-not-attribute
	// (exit 2) where the host itself would have attributed the run. NAME-ONLY form
	// (`-e NAME`): docker copies the value from the launcher's own environment and
	// leaves the variable unset in the container when the host has none, so this
	// never invents an identity, and the value is never rendered in the printed
	// argv. The set is pinned to executingRunner's env reads by
	// TestVICAttribEnvClass.
	for _, name := range attributionEnvVars {
		argv = append(argv, "-e", name)
	}
	// --user maps container writes to the host user so the Evidence the container
	// appends lands host-owned, not root-owned. Omitted when there is no POSIX
	// uid (Windows: os.Getuid() == -1), where Docker Desktop maps ownership to the
	// host user itself.
	if inv.uid >= 0 && inv.gid >= 0 {
		argv = append(argv, "--user", fmt.Sprintf("%d:%d", inv.uid, inv.gid))
	}
	// Credential passthrough is the env-file PATH only (containers/secrets.md §3).
	// Never a credential on the argv; never the file's contents.
	if inv.envFile != "" {
		argv = append(argv, "--env-file", inv.envFile)
	}
	argv = append(argv, inv.pin.ref())
	argv = append(argv, inv.inner...)
	return argv
}

// buildInnerCommand reconstructs the command run INSIDE the container: the same
// verifyrun invocation, minus --in-container (the inner run does the real work),
// with the brief addressed by its path under /work. The pass-through flags mirror
// what the outer run parsed.
func buildInnerCommand(briefRel string, check, dryRun, ci bool, timeout string) []string {
	inner := []string{"statusgen", "verifyrun", "--brief", briefRel}
	if check {
		inner = append(inner, "--check")
	}
	if dryRun {
		inner = append(inner, "--dry-run")
	}
	if ci {
		inner = append(inner, "--ci")
	}
	if timeout != "" {
		inner = append(inner, "--timeout", timeout)
	}
	return inner
}

// runInContainer is the `--in-container` entry point, called from runVerifyrun
// after flag parsing. It resolves the pin, refuses fail-closed on anything
// unpinned, composes the docker argv, and execs docker, streaming its output.
func runInContainer(briefPath, root, envFile string, check, dryRun, ci bool, timeout string, stdout, stderr *os.File) int {
	pin, err := readHarnessPin(root)
	if err != nil {
		fmt.Fprintln(stderr, "statusgen verifyrun --in-container:", err)
		return verifyrunExitCouldNot
	}
	if err := pin.validate(); err != nil {
		fmt.Fprintln(stderr, "statusgen verifyrun --in-container: refusing to run —", err)
		return verifyrunExitCouldNot
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(stderr, "statusgen verifyrun --in-container: cannot resolve root:", err)
		return verifyrunExitCouldNot
	}
	absBrief, err := filepath.Abs(briefPath)
	if err != nil {
		fmt.Fprintln(stderr, "statusgen verifyrun --in-container: cannot resolve brief path:", err)
		return verifyrunExitCouldNot
	}
	briefRel, err := filepath.Rel(absRoot, absBrief)
	if err != nil || strings.HasPrefix(briefRel, "..") {
		fmt.Fprintf(stderr, "statusgen verifyrun --in-container: the brief %s is not under the root %s — the container only sees the checkout bind-mounted at %s\n", briefPath, root, containerWorkDir)
		return verifyrunExitCouldNot
	}
	// Address the brief with forward slashes: it resolves inside a Linux container
	// regardless of the host path separator.
	briefRel = filepath.ToSlash(briefRel)

	// The env-file is a PATH we pass through; we only STAT it (never read its
	// contents) so a mistyped path fails here rather than inside the container.
	if envFile != "" {
		if _, err := os.Stat(envFile); err != nil {
			fmt.Fprintf(stderr, "statusgen verifyrun --in-container: --env-file %s is not readable: %v (pass the role env-file path per containers/secrets.md, or omit it for an offline run)\n", envFile, err)
			return verifyrunExitCouldNot
		}
	}

	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		fmt.Fprintln(stderr, "statusgen verifyrun --in-container: `docker` is not on PATH — the container runner needs Docker (Docker Desktop on Windows, with the Linux-container backend, since the harness image is a Linux image)")
		return verifyrunExitCouldNot
	}

	// Resolve the host's git identity (#2097) before docker runs, so a value that
	// cannot cross intact is refused here rather than as could-not-attribute inside
	// the container.
	identity, err := hostGitIdentity(absRoot)
	if err != nil {
		fmt.Fprintln(stderr, "statusgen verifyrun --in-container: refusing to run —", err)
		return verifyrunExitCouldNot
	}

	inv := containerInvocation{
		pin:      pin,
		root:     absRoot,
		briefRel: briefRel,
		envFile:  envFile,
		inner:    buildInnerCommand(briefRel, check, dryRun, ci, timeout),
		uid:      hostUID(),
		gid:      hostGID(),
		identity: identity,
	}
	argv := composeDockerArgs(inv)

	// The printed command carries only the env-file PATH (a path is not a secret,
	// containers/secrets.md §2) — the file's CONTENTS are never rendered here or
	// anywhere.
	fmt.Fprintf(stdout, "statusgen verifyrun --in-container: %s\n", pin.ref())
	fmt.Fprintf(stdout, "docker %s\n", strings.Join(argv, " "))

	cmd := exec.Command(dockerPath, argv...)
	cmd.Env = dockerClientEnv(os.Environ(), identity)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// Propagate the inner verifyrun exit code verbatim: 0/1/2 mean the same
			// thing whether verifyrun ran on the host or in the container.
			return ee.ExitCode()
		}
		fmt.Fprintln(stderr, "statusgen verifyrun --in-container: docker run failed to start:", err)
		return verifyrunExitCouldNot
	}
	return verifyrunExitPass
}

// hostUID / hostGID return the invoking user's POSIX ids, or -1 on a platform
// (Windows) that has no POSIX uid — the signal composeDockerArgs uses to omit
// --user.
func hostUID() int { return osGetuid() }
func hostGID() int { return osGetgid() }

// osGetuid / osGetgid are indirected through package vars so a test can drive the
// Windows path (uid == -1 ⇒ no --user) on a POSIX host. os.Getuid already returns
// -1 on Windows.
var (
	osGetuid = os.Getuid
	osGetgid = os.Getgid
)
