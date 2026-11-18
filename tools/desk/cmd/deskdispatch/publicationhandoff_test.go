package main

import (
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The deskevidence handoff test runs this companion in a process of its own.
// It supplies one loopback forge and the same Git trees used later by the writer.
func TestPublicationHandoffDispatcher(t *testing.T) {
	var fixture []string
	for i, arg := range os.Args {
		if arg == "--" {
			fixture = os.Args[i+1:]
			break
		}
	}
	if len(fixture) == 0 {
		t.Skip("companion to TestDispatchPublicationHandoff")
	}
	if len(fixture) != 6 {
		t.Fatal("invalid local handoff fixture")
	}
	api, source, tracking, deliveryRepo, brief, prompt := fixture[0], fixture[1], fixture[2], fixture[3], fixture[4], fixture[5]
	endpoint, err := url.Parse(api)
	if err != nil || endpoint.Scheme != "http" || !net.ParseIP(endpoint.Hostname()).IsLoopback() {
		t.Fatal("handoff fixture requires a loopback forge")
	}
	const tracker = "example-org/tracker"
	isolateClaimTool(t, t.TempDir())
	withPortsOnPath(t, goDecisionBinary, goClaimBinary)
	deskkit.SetToolClass(deskkit.ClassCI)
	deskkit.SetGitHubCustodyMinter(func(string, deskkit.ForgeRepo) (string, string, error) { return ghStampToken, api, nil })
	oldExec := execCommand
	defer func() { execCommand = oldExec }()
	claimed, decided := "", ""
	execCommand = func(name string, args ...string) *exec.Cmd {
		switch filepath.Base(name) {
		case "git":
			return exec.Command(name, args...)
		case "deskwt":
			return exec.Command("printf", "%s\n", source)
		case goDecisionBinary, goClaimBinary:
			var repo string
			for i, arg := range args {
				if arg == "--repo" && i+1 < len(args) {
					repo = args[i+1]
				}
			}
			root := tracking
			if repo != tracker {
				root = source
			}
			if repo != tracker && repo != deliveryRepo {
				t.Errorf("unknown fixture repository %q", repo)
				return exec.Command("false")
			}
			if filepath.Base(name) == goDecisionBinary && args[0] == "ensure" {
				decided = repo
				return exec.Command("git", "-C", root, "show-ref", "--verify", "refs/fixture-decisions/4")
			}
			if args[0] == "acquire" {
				claimed = repo
				return exec.Command("git", "-C", root, "update-ref", "refs/fixture-claims/"+args[1], "HEAD")
			}
			return exec.Command("true")
		default:
			t.Errorf("unhandled dispatch child %s %v", name, args)
			return exec.Command("false")
		}
	}
	appendDispatchRecord = func(*deskkit.DispatchRecord) error { return nil }
	args := []string{"vg/01", "--kit", "verifier", "--model", "gpt-6-astra", "--tier", "strong", "--repo", deliveryRepo,
		"--root", source, "--claim-root", tracking, "--brief", filepath.Join(tracking, brief), "--gate-human", "--prompt-file", prompt}
	if err := cmdDispatch(args); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if decided != tracker || claimed != tracker {
		t.Fatalf("decision=%q claim=%q, want both in %q", decided, claimed, tracker)
	}
	b, err := os.ReadFile(emittedPathFor(prompt))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "deskevidence ") {
			if err := os.WriteFile(prompt+".command", []byte(line), 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("no publication command in dispatch assignment")
}
