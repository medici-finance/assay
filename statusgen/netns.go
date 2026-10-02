package main

// netns — the check:ci network-off sandbox, with loopback UP (issue #1925).
//
// THE DEFECT. check:ci rows run network-off under `unshare --net
// --map-root-user`. A brand-new network namespace holds exactly one interface,
// `lo`, and the kernel creates it DOWN. So the sandbox was not merely
// "network-off": it had no loopback either, and every row whose test starts a
// local server (`httptest`, a fixture listener on 127.0.0.1) failed with
// `connect: network is unreachable` — a product-check `fail` manufactured by the
// sandbox, not by the work. The same rows pass under `docker --network none`,
// where loopback is up.
//
// THE FIX. The sandbox argv no longer hands the row's shell straight to
// `unshare`. It hands it to statusgen ITSELF, re-executed inside the new
// namespace under a hidden first argument (netnsHelperArg). That helper:
//
//  1. brings `lo` up (an ioctl — no dependency on `ip`/iproute2, which minimal
//     images such as the Debian golang image do not ship);
//  2. proves the namespace is actually isolated: it is NOT the caller's network
//     namespace (the caller passes its own `/proc/self/ns/net` id in), `lo` is
//     up, and no other interface is up or carries an address;
//  3. only then execs the row's shell.
//
// Any failure at 1 or 2 is a REFUSAL: the helper prints netnsRefusedMarker on
// stderr, exits netnsRefusedExit, and never runs the row. The caller reads that
// as could-not-run — never pass, never the row's own `fail`, and never a
// fallback to an un-sandboxed run. Loopback is the ONLY thing this change opens:
// isolation is the control, and the helper refuses rather than run a row in a
// namespace that can reach anything off-box.
//
// netnsWrapperArgv is the ONE place a network-namespace argv is assembled (the
// class guard TestNetnsWrapperOneChokePoint pins it), so no second call site can
// reintroduce a sandbox that skips the helper.

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// netnsHelperArg is the hidden first argument that turns a statusgen process
// into the in-namespace helper. It is not a user-facing subcommand. The helper
// runs its command only from a network namespace that differs from the caller
// id it is given, where it could bring lo up and no other interface is up or
// addressed; anywhere else — in particular in the caller's own namespace, the
// id networkOffWrapper always passes — it refuses. (Handed some OTHER id from
// inside a lo-only namespace it owns, such as the check:ci sandbox itself, it
// would run: that namespace really is isolated. The id is what makes "still in
// the caller's namespace" detectable, so callers must pass their own.)
const netnsHelperArg = "__verifyrun-netns"

// netnsRefusedExit is the helper's exit status when it refuses to run the row.
// 125 mirrors the convention of wrapper tools (`env`, `nice`, `timeout`) for
// "the wrapper itself failed" and is outside the 126/127 shell range.
const netnsRefusedExit = 125

// netnsRefusedMarker prefixes every refusal line the helper writes to stderr.
// The caller requires BOTH the exit status and this marker before reclassifying
// a run as could-not-run.
const netnsRefusedMarker = "verifyrun-netns: REFUSED: "

// netnsWrapperArgv assembles the sandbox argv prefix: a fresh network namespace
// (via an unprivileged user namespace) whose first process is the statusgen
// helper, told the caller's own network-namespace id so it can prove it is NOT
// still in it. The row's shell argv is appended after this prefix.
func netnsWrapperArgv(unsharePath, selfExe, parentNS string) []string {
	return []string{unsharePath, "--net", "--map-root-user", selfExe, netnsHelperArg, parentNS}
}

// maybeRunNetnsHelper dispatches the in-namespace helper when this process was
// started as one (os.Args[1] == netnsHelperArg). It never returns in that case:
// the helper either execs the row's shell or exits netnsRefusedExit. main() and
// the test binary's TestMain call it before anything else.
func maybeRunNetnsHelper() {
	if len(os.Args) >= 2 && os.Args[1] == netnsHelperArg {
		os.Exit(runNetnsHelper(os.Args[2:]))
	}
}

// netnsRefuse writes the refusal line and returns the refusal exit status.
func netnsRefuse(format string, a ...any) int {
	fmt.Fprintln(os.Stderr, netnsRefusedMarker+fmt.Sprintf(format, a...))
	return netnsRefusedExit
}

// netnsIface is the snapshot of one interface the isolation check reads.
type netnsIface struct {
	Name     string
	Loopback bool
	Up       bool
	Addrs    int
}

// netnsIsolationProblem decides, from a snapshot, whether the namespace the
// helper is in is fit to run a network-off row. It returns "" when it is, else
// the reason it is not. Pure, so every branch is tested on every host.
//
//   - own/parent: the helper's and the caller's `/proc/self/ns/net` ids. Equal
//     (or unknown) means no new namespace was entered: the row would run on the
//     caller's network.
//   - lo must be present and up — that is the point of the fix.
//   - every other interface must be down AND address-less. A new namespace can
//     legitimately contain kernel fallback tunnel devices (`sit0`, `tunl0`,
//     `ip6tnl0`, …) when those modules are loaded; they are created down with no
//     address and carry nothing, so they are tolerated. An interface that is up,
//     or holds an address, is a possible route off-box: refuse.
func netnsIsolationProblem(own, parent string, ifs []netnsIface) string {
	if own == "" || parent == "" {
		return fmt.Sprintf("cannot establish the network namespace identity (own=%q caller=%q)", own, parent)
	}
	if own == parent {
		return "still in the caller's network namespace (" + own + ") — no isolated namespace was entered"
	}
	loUp := false
	for _, ifc := range ifs {
		if ifc.Name == "lo" && ifc.Loopback {
			loUp = ifc.Up
			continue
		}
		if ifc.Up {
			return "interface " + ifc.Name + " is up inside the namespace — it is not isolated"
		}
		if ifc.Addrs > 0 {
			return fmt.Sprintf("interface %s carries %d address(es) inside the namespace — it is not isolated", ifc.Name, ifc.Addrs)
		}
	}
	if !loUp {
		return "lo is not up inside the namespace after bringing it up"
	}
	return ""
}

// classifyNetnsRefusal turns a helper refusal into could-not-run. A sandboxed
// run that exited netnsRefusedExit AND carries netnsRefusedMarker never ran the
// row: recording it as the row's own `fail` would blame the work for the
// sandbox, and recording anything else would be a verdict nobody produced.
func classifyNetnsRefusal(res runResult) runResult {
	if res.couldNotRun || res.exit != netnsRefusedExit {
		return res
	}
	i := bytes.Index(res.output, []byte(netnsRefusedMarker))
	if i < 0 {
		return res
	}
	line := string(res.output[i+len(netnsRefusedMarker):])
	if j := strings.IndexByte(line, '\n'); j >= 0 {
		line = line[:j]
	}
	res.couldNotRun = true
	res.reason = "the check:ci network-off sandbox refused to run the row: " + strings.TrimSpace(line) +
		" — no verdict was produced, and the row is never re-run outside the sandbox"
	return res
}
