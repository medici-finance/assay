package main

// packet.go — the dispatch packet, kit-neutral half.
//
// A dispatch may hand its agent ONE read-ahead file: the material the agent would otherwise
// fetch call by call. What goes in it differs by kit; everything else — the header, the caps
// and omission list, the boundary lines around quoted text and the escaping inside them, the
// code span for a single-line value, the owner-only writer and the single assignment line —
// is internal/packet's and is the same for every kit.
//
// HOW A KIT GETS A PACKET. A provider is registered for the kit (registerPacketProvider, from
// an init in the provider's own file). dispatch() calls buildPacketFn ONCE, just before the
// prompt is assembled; the provider returns a packet.Spec (the head commit and an ordered
// list of named sections) and this file does the rest. A kit with no provider gets no packet
// and no message — adding a kit's packet is a new file, not an edit here.
//
// IT CAN NEVER FAIL A DISPATCH. The packet is a convenience: the claim is held, the worktree
// is cut and the agent can read everything at the source. So buildDispatchPacket returns a
// path or "", never an error. Any failure — no credential, a forge that will not answer, a
// head that moved, a directory that cannot be written, a provider that panics — is one line
// on stderr saying why, no `Packet:` line in the assignment, and the dispatch carries on. It
// is deliberately NOT one of the numbered dispatch steps.
//
// IT IS NOT BUILT ON --dry-run, which touches nothing.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/packet"
)

// packetInput is what a provider is given: the dispatch as resolved so far.
type packetInput struct {
	o    dispatchOpts
	plan dispatchPlan
	// repo is the repository the dispatched work belongs to (owner/name).
	repo string
	// home is the dispatched agent's worktree.
	home string
}

// packetProvider builds the packet.Spec for one dispatch of its kit: the head commit the
// packet describes and the ordered sections. It reads; it writes nothing. Returning
// errNoPacket means "this dispatch has nothing to build a packet from" and is silent; any
// other error is reported on stderr and the dispatch continues without a packet.
type packetProvider func(in packetInput) (packet.Spec, error)

// errNoPacket is a provider's way to decline quietly (a review dispatch with no change
// number has nothing to read yet).
var errNoPacket = errors.New("no packet for this dispatch")

var packetProviders = map[string]packetProvider{}

// registerPacketProvider binds a provider to a --kit value. It is called from init, so a
// second provider for one kit is a build defect and panics at start-up rather than letting
// the later file silently win.
func registerPacketProvider(kit string, p packetProvider) {
	key := packetKitKey(kit)
	if key == "" || p == nil {
		panic("deskdispatch: registerPacketProvider needs a kit and a provider")
	}
	if _, dup := packetProviders[key]; dup {
		panic("deskdispatch: two packet providers registered for kit " + key)
	}
	packetProviders[key] = p
}

func packetKitKey(kit string) string { return strings.ToLower(strings.TrimSpace(kit)) }

// buildPacketFn is the ONE call dispatch() makes. It is a seam so the full-run dispatch
// tests, which stand up a forge that knows only the routes they exercise, run without a
// packet unless a test asks for one.
var buildPacketFn = buildDispatchPacket

// packetNow and packetCacheDir are seams for the build time and for where a packet goes
// when the prompt is printed rather than written to a file.
var (
	packetNow      = time.Now
	packetCacheDir = os.UserCacheDir
)

// buildDispatchPacket builds and writes the packet for this dispatch and returns its
// absolute path, or "" when there is none. See the file comment: it never fails the caller.
func buildDispatchPacket(o dispatchOpts, plan dispatchPlan, repo, home string) (path string) {
	provider, ok := packetProviders[packetKitKey(o.kit)]
	if !ok {
		return ""
	}
	defer func() {
		if r := recover(); r != nil {
			path = ""
			packetSkipped(fmt.Sprintf("the builder stopped unexpectedly: %v", r))
		}
	}()
	spec, err := provider(packetInput{o: o, plan: plan, repo: repo, home: home})
	if errors.Is(err, errNoPacket) {
		return ""
	}
	if err != nil {
		packetSkipped(err.Error())
		return ""
	}
	if spec.Kit == "" {
		spec.Kit = packetKitKey(o.kit)
	}
	if spec.Item == "" {
		spec.Item = plan.claimKey
	}
	if spec.Built.IsZero() {
		spec.Built = packetNow()
	}
	dest, err := packetPathFor(o, plan)
	if err != nil {
		packetSkipped("no place to write it: " + err.Error())
		return ""
	}
	if _, err := packet.AssignmentLine(dest); err != nil {
		packetSkipped(err.Error())
		return ""
	}
	built, err := packet.Build(spec)
	if err != nil {
		packetSkipped(err.Error())
		return ""
	}
	if err := packet.Write(dest, built); err != nil {
		packetSkipped(fmt.Sprintf("could not write %s: %v", dest, err))
		return ""
	}
	o.say("packet: wrote %s (%d bytes, head %s, %d item(s) omitted and listed in it)",
		dest, len(built.Text), spec.Head, len(built.Omitted))
	return dest
}

// packetSkipped says, on stderr and regardless of --quiet, why this dispatch has no packet.
func packetSkipped(why string) {
	fmt.Fprintf(os.Stderr, "deskdispatch: packet: NOT built — %s. The assignment carries no `Packet:` line; "+
		"the dispatch continues and the agent reads at the source.\n", firstLine(why))
}

// packetMaxAge is how long a packet in the cache directory is kept. A packet is a snapshot
// for one dispatch; a week is far longer than any dispatch runs.
const packetMaxAge = 7 * 24 * time.Hour

const packetSuffix = ".packet.md"

// packetPathFor decides where the packet goes: BESIDE THE ASSIGNMENT.
//
//   - With --prompt-file, the assignment is a file, and the packet is its sibling:
//     "<prompt file without extension>.packet.md". Whoever owns the prompt file owns both.
//   - Without it the assignment is printed, so there is no file to sit beside. The packet
//     then goes in the user's cache directory ("<cache>/assay/packets/<claim key>.packet.md"),
//     an owner-only directory for regenerable files. It is NOT written into the agent's
//     worktree — an untracked file there would make a read-only review tree dirty — and not
//     into the configuration directory, which holds credentials an agent has no business
//     being pointed at. One file per claim key: a re-dispatch replaces it, and packets older
//     than a week are removed as new ones are written.
func packetPathFor(o dispatchOpts, plan dispatchPlan) (string, error) {
	if pf := strings.TrimSpace(o.promptFile); pf != "" {
		abs, err := filepath.Abs(pf)
		if err != nil {
			return "", err
		}
		return strings.TrimSuffix(abs, filepath.Ext(abs)) + packetSuffix, nil
	}
	cache, err := packetCacheDir()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(cache) {
		return "", fmt.Errorf("cache directory %q is not absolute", cache)
	}
	name := sanitizeSegment(plan.claimKey)
	if name == "" || strings.ContainsAny(name, `/\`) || !itemKeyRe.MatchString(name) {
		return "", fmt.Errorf("claim key %q does not make a file name", plan.claimKey)
	}
	dir := filepath.Join(cache, "assay", "packets")
	prunePackets(dir, packetNow().Add(-packetMaxAge))
	return filepath.Join(dir, name+packetSuffix), nil
}

// prunePackets removes this tool's own packets in dir last written before cutoff. Best
// effort and narrow: only regular files carrying the packet suffix, never a directory, never
// anything else that happens to be there.
func prunePackets(dir string, cutoff time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), packetSuffix) {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// writePacketLine adds the ONE line that tells the agent a packet exists:
// "Packet: <absolute path>". It is written for every kit from this single place, and only
// when a packet was actually written — no path, no line.
func writePacketLine(b *strings.Builder, plan dispatchPlan) {
	if plan.packetPath == "" {
		return
	}
	line, err := packet.AssignmentLine(plan.packetPath)
	if err != nil {
		return
	}
	b.WriteString("\n" + line + "\n")
}
