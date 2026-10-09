package main

// packet_verifier_reads_test.go — a failed read never puts the reader's words in the packet.
//
// THE RISK. The packet is a file an agent is told to read first. An error from a read of the
// repository carries whatever the reader chose to say: a path on the machine, a system
// message, text taken from the repository itself. So the rule is that a failure's text goes
// to ONE place, the dispatcher's own `packet: NOT built` line on stderr, and only when no
// packet is written; where a packet IS written past a failed read, it states the omission
// in two fixed sentences of the tool's own.
//
// HOW IT IS CHECKED. Every read the provider makes goes through vpReader. Each test below
// makes exactly one of them fail with a recognisable text and runs the dispatcher's own
// entry, buildDispatchPacket, so what is read back is the file a verifier would be handed
// and the line an operator would see.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// vpFault is the text of the injected failure. Its parts are looked for one by one too.
const vpFault = "READ-FAULT-TEXT open /somewhere/private/objects/pack-1f: permission denied"

var vpFaultParts = []string{"READ-FAULT-TEXT", "/somewhere/private", "pack-1f", "permission denied"}

// vpFaulty is a real reader with one read site that fails.
type vpFaulty struct {
	vpReader
	site string
	open int // 1 for the provider's opening of the home, 2 for the re-check's
}

func (f vpFaulty) fails(site string) bool { return f.site == site }

func (f vpFaulty) HeadID() (string, error) {
	if (f.fails("head") && f.open == 1) || (f.fails("head again") && f.open == 2) {
		return "", errors.New(vpFault)
	}
	return f.vpReader.HeadID()
}

func (f vpFaulty) ParentHashes(rev string) ([]string, error) {
	if (f.fails("parents") && f.open == 1) || (f.fails("parents again") && f.open == 2) {
		return nil, errors.New(vpFault)
	}
	return f.vpReader.ParentHashes(rev)
}

func (f vpFaulty) FileAt(rev, path string) (string, error) {
	if f.fails("brief file") {
		return "", errors.New(vpFault)
	}
	return f.vpReader.FileAt(rev, path)
}

// PathHistory lets the real walk run to its end and then fails, as a history that stops
// short does: what was read before the gap is still listed.
func (f vpFaulty) PathHistory(rev, path string, visit func(gitcore.PathCommit) bool) error {
	if err := f.vpReader.PathHistory(rev, path, visit); err != nil {
		return err
	}
	if f.fails("history walk") {
		return errors.New(vpFault)
	}
	return nil
}

func (f vpFaulty) DiffNames(from, to string) ([]string, error) {
	if f.fails("changed paths") {
		return nil, errors.New(vpFault)
	}
	return f.vpReader.DiffNames(from, to)
}

// vpDispatchWithFault builds the fixture's packet through the dispatcher's entry with one
// read site failing. It returns the packet text ("" when none was written) and what stderr
// showed.
func vpDispatchWithFault(t *testing.T, site string) (text, stderr string) {
	t.Helper()
	root := vpRepo(t, vpBriefBefore+vpBriefAfter)
	orig, opens := vpOpen, 0
	vpOpen = func(home string) (vpReader, error) {
		opens++
		if (site == "open" && opens == 1) || (site == "open again" && opens == 2) {
			return nil, errors.New(vpFault)
		}
		r, err := orig(home)
		if err != nil {
			return nil, err
		}
		return vpFaulty{vpReader: r, site: site, open: opens}, nil
	}
	t.Cleanup(func() { vpOpen = orig })

	in := vpInput(root)
	dir := t.TempDir()
	in.o.promptFile = filepath.Join(dir, "assignment.md")
	var dest string
	stderr = captureStderr(t, func() { dest = buildDispatchPacket(in.o, in.plan, in.repo, in.home) })

	left, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	if dest == "" {
		if len(left) != 0 {
			t.Fatalf("%s: no packet was reported and the directory holds %q", site, left)
		}
		return "", stderr
	}
	body, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("%s: the reported packet cannot be read: %v", site, err)
	}
	return string(body), stderr
}

// TestVerifierPacketAFailedProviderReadIsOneStderrLineAndNoPacket: a read the provider
// cannot do without — opening the home, its commit, the brief, the re-check — ends in no
// packet at all, and the failure's text is on the one `NOT built` line and nowhere else.
func TestVerifierPacketAFailedProviderReadIsOneStderrLineAndNoPacket(t *testing.T) {
	for _, site := range []string{"open", "head", "parents", "brief file", "open again", "head again", "parents again"} {
		t.Run(site, func(t *testing.T) {
			text, stderr := vpDispatchWithFault(t, site)
			if text != "" {
				t.Fatalf("a packet was written past a failed read:\n%s", text)
			}
			lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
			if len(lines) != 1 || !strings.HasPrefix(lines[0], "deskdispatch: packet: NOT built — ") {
				t.Fatalf("stderr is not the one `NOT built` line:\n%s", stderr)
			}
			if strings.Count(lines[0], vpFault) != 1 {
				t.Errorf("control: the injected failure was not reached, or its text is not on the line: %s", lines[0])
			}
		})
	}
}

// TestVerifierPacketAFailedSectionReadIsStatedInTheToolsOwnWords: the two reads a packet
// survives — the history walk and a commit's changed paths — leave a packet that states
// each omission in a fixed sentence. Every such line is checked WHOLE: the fixed reason is
// the end of it, so nothing a reader said can follow.
func TestVerifierPacketAFailedSectionReadIsStatedInTheToolsOwnWords(t *testing.T) {
	const section = " (section \"Commits that changed or name this brief\") — size unknown — "
	const noParent = "the commit has no parent to compare against"
	for _, tc := range []struct {
		site, names, reason string
		items               int
	}{
		{"history walk", "`commits older than the 3 searched`", "the history could not be read further in this clone", 1},
		// The fixture has three commits to list; the first has no parent, so two are compared.
		{"changed paths", "`paths changed by ", "its first parent could not be read in this clone", 2},
	} {
		t.Run(tc.site, func(t *testing.T) {
			text, stderr := vpDispatchWithFault(t, tc.site)
			if text == "" {
				t.Fatalf("no packet was written; stderr:\n%s", stderr)
			}
			for _, part := range vpFaultParts {
				if strings.Contains(text, part) {
					t.Errorf("the packet holds %q from a failed read's own text", part)
				}
				if strings.Contains(stderr, part) {
					t.Errorf("stderr holds %q although a packet was written:\n%s", part, stderr)
				}
			}
			// Each omission is written twice by the shared builder: in the list at the top and
			// in its section.
			listed, inSection := 0, 0
			for _, l := range strings.Split(text, "\n") {
				if !strings.Contains(l, tc.names) || !strings.Contains(l, "size unknown") {
					continue
				}
				switch {
				case strings.HasPrefix(l, "- "+tc.names) && strings.HasSuffix(l, section+tc.reason):
					listed++
				case strings.HasPrefix(l, "_Omitted: "+tc.names) && strings.HasSuffix(l, section+tc.reason+"._"):
					inSection++
				case strings.HasSuffix(l, section+noParent) || strings.HasSuffix(l, section+noParent+"._"):
					// The fixture's first commit: another fixed sentence, and no read failed.
				default:
					t.Errorf("an omission line is not the tool's fixed sentence, whole:\n%s", l)
				}
			}
			if listed != tc.items || inSection != tc.items {
				t.Errorf("control: the failing read left %d listed and %d in-section omission line(s), want %d of each:\n%s",
					listed, inSection, tc.items, text)
			}
		})
	}
}
