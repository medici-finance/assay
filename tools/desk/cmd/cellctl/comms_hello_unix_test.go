//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
	"github.com/medici-finance/assay/tools/desk/internal/commstransport"
)

// Two independent session contexts use shipped client, gateway, drain and reader
// binaries. Only the model response is a deterministic offline fixture.
func TestTwoSessionsCommsHelloRoundTrip(t *testing.T) {
	root, err := os.MkdirTemp("", "ch-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	os.Mkdir(bin, 0700)
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	build := func(name, pkg string, testBinary bool) string {
		t.Helper()
		path := filepath.Join(bin, name)
		args := []string{"build", "-o", path, pkg}
		if testBinary {
			args = []string{"test", "-c", "-o", path, pkg}
		}
		cmd := exec.Command("go", args...)
		cmd.Dir = module
		if b, err := goquiet.Cmd(cmd).CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v %s", name, err, b)
		}
		return path
	}
	gateway := build("commsgw", "./cmd/commsgw", false)
	drain := build("commsloop", "./cmd/commsloop", false)
	client := build("deskcomms", "./cmd/deskcomms", false)
	native := build("fixture-codex", "./internal/cellreader", true)
	b, _ := os.ReadFile(native)
	sum := sha256.Sum256(b)
	workspace := filepath.Join(root, "reader")
	os.Mkdir(workspace, 0700)
	reader := []string{cellctlBinary(t), "comms-reader", "--codex", native, "--sha256", hex.EncodeToString(sum[:]), "--model", "fixture-model", "--effort", "low", "--home", root, "--workspace", workspace}
	argv, _ := json.Marshal(reader)
	c := &Cell{Name: "hello", Kind: "house", Dir: root, Ref: "hello", Env: envWith(map[string]string{"CELL": "hello", "CELL_KIND": "house", "HOME": root, "DESK_TOOLS_BIN": bin, "CELL_REPO_SLUG": "example/repo", "ROLES": strings.Join(knownRoles, " ")})}
	os.WriteFile(filepath.Join(root, "cell.env"), []byte("CELL=hello\nCELL_KIND=house\n"), 0600)
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", bin+":"+oldPath)
	if err = c.initComms(commsInitOptions{ReaderCmd: string(argv), Model: "fixture-model", Pin: "fixture-1", ReaderContained: true}); err != nil {
		t.Fatal(err)
	}
	cfg, err := c.loadDeskComms()
	if err != nil {
		t.Fatal(err)
	}
	base := []string{"DESK_LOOP=the-desk", "HOME=" + root, "PATH=" + bin + ":/usr/bin:/bin", "KUBECONFIG=/dev/null", "CELL_READER_FIXTURE=1", "CELL_READER_HELLO=1", "ASSAY_COMMS_GATEWAY_ENABLE=1", "ASSAY_COMMS_CELL=hello", "ASSAY_COMMS_REPO=example/repo", "ASSAY_COMMS_LOCAL_ONLY=1", "ASSAY_COMMS_QUEUE_DIR=" + cfg.QueueDir, "ASSAY_COMMS_SOCKET=" + cfg.Socket, "ASSAY_COMMS_TRUST_STORE=" + cfg.TrustStore, "ASSAY_RUNNER_DECIDER=" + string(cfg.Decider)}
	start := func(path, label string) *exec.Cmd {
		t.Helper()
		cmd := exec.Command(path)
		cmd.Dir = root
		cmd.Env = base
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		log, err := os.Create(filepath.Join(root, label+".log"))
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stdout = log
		cmd.Stderr = log
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				<-done
			}
			log.Close()
		})
		return cmd
	}
	start(gateway, "gateway")
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := commstransport.Dial(cfg.Socket, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			b, _ := os.ReadFile(filepath.Join(root, "gateway.log"))
			t.Fatalf("gateway unavailable: %s", b)
		}
		time.Sleep(20 * time.Millisecond)
	}
	session := func(role, id string) []string {
		return append(append([]string{}, base...), "DESK_CELL=hello", "DESK_ROLE="+role, "DESK_LOOP="+role, "DESK_SESSION="+id, "DESK_COMMS_GATEWAY="+cfg.Socket, "DESK_COMMS_KEY="+cfg.SigningKeys[role])
	}
	a, bSession := session("worker-desk", "hello-session-a"), session("pr-review-desk", "hello-session-b")
	call := func(env []string, input string, args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, client, args...)
		cmd.Dir = root
		cmd.Env = env
		cmd.Stdin = strings.NewReader(input)
		var out, errors bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errors
		if err := cmd.Run(); err != nil {
			t.Fatalf("client %v: %v %s", args, err, errors.String())
		}
		return out.String()
	}
	call(a, `{"kind":"informational","text":"hello from session A"}`, "send", "--to", "pr-review-desk", "--verb", "notify")
	call(bSession, `{"kind":"informational","text":"hello from session B"}`, "send", "--to", "worker-desk", "--verb", "notify")
	start(drain, "drain")
	type notice struct {
		ID      string
		From    struct{ Cell, Role string }
		Payload json.RawMessage
	}
	read := func(env []string, wantRole, wantText string) notice {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			raw := call(env, "", "poll", "--json")
			var notices []notice
			if err := json.Unmarshal([]byte(raw), &notices); err != nil {
				t.Fatal(err)
			}
			if len(notices) == 1 {
				n := notices[0]
				if n.From.Cell != "hello" || n.From.Role != wantRole || !strings.Contains(string(n.Payload), wantText) {
					t.Fatalf("wrong sender/payload: %+v", n)
				}
				return n
			}
			if time.Now().After(deadline) {
				b, _ := os.ReadFile(filepath.Join(root, "drain.log"))
				t.Fatalf("hello wasn't delivered: %s", b)
			}
			time.Sleep(30 * time.Millisecond)
		}
	}
	receivedA := read(a, "pr-review-desk", "hello from session B")
	receivedB := read(bSession, "worker-desk", "hello from session A")
	// Poll is repeatable; the sender does not see its own sent hello.
	if n := read(a, "pr-review-desk", "hello from session B"); n.ID != receivedA.ID {
		t.Fatal("poll consumed or changed hello")
	}
	call(a, "", "ack", receivedA.ID)
	call(bSession, "", "ack", receivedB.ID)
	for _, env := range [][]string{a, bSession} {
		if got := strings.TrimSpace(call(env, "", "poll", "--json")); got != "[]" {
			t.Fatal("ack did not clear mailbox", got)
		}
	}
	for _, item := range []struct{ Role, ID string }{{"worker-desk", receivedA.ID}, {"pr-review-desk", receivedB.ID}} {
		if ok, err := commsqueue.IsAcked(cfg.QueueDir, "hello", item.Role, item.ID); err != nil || !ok {
			t.Fatal("ack not durable", err)
		}
	}
	if held, err := commsqueue.ListHeld(cfg.QueueDir); err != nil || len(held) != 0 {
		t.Fatal("hello was quarantined", err)
	}
}
