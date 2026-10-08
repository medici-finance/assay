package main

// verifysandbox — WHICH network-off sandbox a verifyrun run uses (issue #2350).
//
// A `check:ci` row is re-executed network-off, and that is enforced, never
// declared (rule 45). The default sandbox is `unshare --net --map-root-user`
// (networkOffWrapper, verifyrun.go): a fresh, empty network namespace entered
// through an unprivileged user namespace.
//
// THE GAP. A runner that is ALREADY inside a container started with
// `--network none`, under the container engine's DEFAULT seccomp profile, cannot
// use that wrapper: the default profile denies the user-namespace `unshare`. So
// every `check:ci` row there classifies could-not-run, even though the
// container's own network namespace already gives the same network-off
// guarantee. Granting the container a custom seccomp profile is deliberately not
// the route.
//
// THE MODE. `--sandbox=container-netns` tells verifyrun the run is inside such a
// container. Four properties hold it to the guarantee it replaces:
//
//  1. EXPLICIT ONLY. The mode is selected by the flag alone and is never inferred
//     from the environment: a guess that the process "looks containerised" is
//     exactly the declared-not-enforced hermeticity rule 45 forbids.
//  2. PRECHECKED. The unshare wrapper is skipped, so the run first checks that
//     the only network interface UP is loopback (loopbackOnlyPrecheck). Any other
//     interface up — or an interface list that cannot be read — refuses the run:
//     every row that would execute is recorded could-not-run, never pass.
//  3. WITNESSED. The witness Result cell carries `sandbox=<mode>` on every row a
//     sandbox stood behind, so a reader — and the witness gate, through
//     witnessSandboxOf — can tell a namespace-wrapped run from a
//     container-sandboxed one.
//  4. NEVER IN CI. The CI lane keeps the unshare path. The mode is refused when
//     verifyrun's CI-lane signals are present (ciLane).

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
)

// The --sandbox values, which are also the tokens the witness records. They are
// compile-time constants, never caller text, so the recorded mode forges nothing.
const (
	sandboxUnshare        = "unshare"         // namespace-wrapped: `unshare --net --map-root-user` + the netns helper
	sandboxContainerNetns = "container-netns" // container-sandboxed: an outer `--network none` container, loopback-only precheck
)

// netIface is the part of a network interface the precheck reads.
type netIface struct {
	Name     string
	Up       bool // the kernel's IFF_UP flag — NOT operstate
	Loopback bool // IFF_LOOPBACK
}

// listInterfacesFn is the interface source the precheck reads. It is
// systemInterfaces in production; it is a variable only so a test can plant an
// interface list on every host.
var listInterfacesFn = systemInterfaces

// systemInterfaces reads this process's network interfaces. net.Interfaces
// reports each interface's IFF_UP flag as net.FlagUp.
//
// Why the FLAG and not operstate: under `--network none`, `lo` reports operstate
// `unknown` even when it is up, and a DOWN `tunl0` is present. An operstate read
// misreads `lo`, and an "only `lo` exists" read fails on `tunl0`. The flag reads
// both correctly.
func systemInterfaces() ([]netIface, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]netIface, 0, len(ifs))
	for _, i := range ifs {
		out = append(out, netIface{
			Name:     i.Name,
			Up:       i.Flags&net.FlagUp != 0,
			Loopback: i.Flags&net.FlagLoopback != 0,
		})
	}
	return out, nil
}

// loopbackOnlyPrecheck passes only when no interface other than loopback has
// IFF_UP set. A down interface (tunl0 under `--network none`) is fine; an up
// one is a route off the box, and the run is refused with its name. A list that
// cannot be read is a refusal too: the precheck did not look, so it cannot pass.
func loopbackOnlyPrecheck(list func() ([]netIface, error)) (ok bool, why string) {
	ifs, err := list()
	if err != nil {
		return false, "the network interfaces could not be read (" + err.Error() + ")"
	}
	var up []string
	for _, i := range ifs {
		if i.Up && !i.Loopback {
			up = append(up, i.Name)
		}
	}
	if len(up) > 0 {
		return false, "non-loopback interface(s) up: " + strings.Join(up, ", ")
	}
	return true, ""
}

// containerNetnsRefusedNote is the could-not-run reason for every row of a run
// whose precheck refused.
func containerNetnsRefusedNote(why string) string {
	return "--sandbox=" + sandboxContainerNetns + " precheck refused the run: " + why +
		". This mode requires a container started with --network none, where loopback is the only interface up"
}

// ciLane reports whether this run is in the CI lane, using the two signals
// verifyrun already reads: the explicit --ci flag (the CI-context switch that
// skips env-bound rows) and GITHUB_ACTIONS=true (which executingRunner already
// reads to attribute a CI run). Either one is enough to refuse the container
// mode, so a CI job that forgot --ci still keeps the unshare path.
func ciLane(ciFlag bool) bool {
	return ciFlag || os.Getenv("GITHUB_ACTIONS") == "true"
}

// sandboxRefusal validates the --sandbox flag against the rest of the
// invocation. It returns "" when the run may proceed, else the refusal message.
func sandboxRefusal(mode string, ciFlag, inContainer bool) string {
	switch mode {
	case sandboxUnshare:
		return ""
	case sandboxContainerNetns:
	default:
		return fmt.Sprintf("unknown --sandbox %q — the values are %q (the default) and %q", mode, sandboxUnshare, sandboxContainerNetns)
	}
	if ciLane(ciFlag) {
		return "refusing --sandbox=" + sandboxContainerNetns + " in the CI lane (--ci or GITHUB_ACTIONS=true): CI re-executes check:ci rows under `unshare --net`, and this mode is for a runner already inside a --network none container"
	}
	if inContainer {
		return "refusing --sandbox=" + sandboxContainerNetns + " with --in-container: the harness container verifyrun starts is not a --network none container. Run verifyrun with this mode from inside the --network none container instead"
	}
	return ""
}

// witnessSandboxRe lifts the `sandbox=<mode>` token from a witness Result cell.
var witnessSandboxRe = regexp.MustCompile(`\bsandbox=(` + regexp.QuoteMeta(sandboxUnshare) + `|` + regexp.QuoteMeta(sandboxContainerNetns) + `)\b`)

// witnessSandboxOf reads which sandbox a witness row recorded: sandboxUnshare,
// sandboxContainerNetns, or "" when the row ran under no sandbox (or predates
// the field). It reads the Result cell positionally, like witnessStateOf, so a
// command that merely mentions `sandbox=` cannot impersonate the field.
func witnessSandboxOf(text string) string {
	cells := witnessCells(text)
	if cells == nil {
		return ""
	}
	result := cells[witnessCellResult]
	// The token precedes any could-not-run reason (` — …`), so only the head of
	// the cell is read: a reason that quotes `--sandbox=…` is not the field.
	if head, _, found := strings.Cut(result, " — "); found {
		result = head
	}
	m := witnessSandboxRe.FindStringSubmatch(result)
	if m == nil {
		return ""
	}
	return m[1]
}
