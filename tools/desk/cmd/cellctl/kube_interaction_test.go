//go:build darwin || linux

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	kc "github.com/medici-finance/assay/tools/desk/internal/kubecompat"
	"golang.org/x/sys/unix"
)

type interactionClient struct {
	terminal *os.File
	cmd      *exec.Cmd
	done     chan error
	lines    chan string
}

func openInteractionClient(t *testing.T, w *cliWorld, role string) *interactionClient {
	t.Helper()
	m, s, err := openPTY()
	if err != nil {
		t.Fatal(err)
	}
	c := &interactionClient{terminal: m, done: make(chan error, 1), lines: make(chan string, 64)}
	c.size(t, 85, 30)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	c.cmd = exec.CommandContext(ctx, cellctlBinary(t), "attach", "example", role)
	c.cmd.Env = w.baseEnv()
	c.cmd.Stdin = s
	output, err := c.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = c.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	go func() {
		defer close(c.lines)
		scan := bufio.NewScanner(output)
		for scan.Scan() {
			c.lines <- strings.TrimRight(scan.Text(), "\r")
		}
	}()
	go func() {
		err := c.cmd.Wait()
		if err != nil {
			err = fmt.Errorf("attach client: %w", err)
		}
		c.done <- err
	}()
	t.Cleanup(func() { cancel(); _ = m.Close() })
	return c
}
func (c *interactionClient) size(t *testing.T, cols, rows uint16) {
	t.Helper()
	if err := unix.IoctlSetWinsize(int(c.terminal.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: cols, Row: rows}); err != nil {
		t.Fatal(err)
	}
}
func (c *interactionClient) send(t *testing.T, input string) {
	t.Helper()
	if _, err := c.terminal.WriteString(input); err != nil {
		t.Fatal(err)
	}
}
func (c *interactionClient) await(t *testing.T, command, want string) {
	t.Helper()
	// The tty can be opened before the native attach/resize negotiation is
	// complete. Only idempotent presentation queries are retried.
	c.send(t, command+"\n")
	deadline := time.NewTimer(6 * time.Second)
	defer deadline.Stop()
	retry := time.NewTicker(200 * time.Millisecond)
	defer retry.Stop()
	observedLines := 0
	for {
		select {
		case line, ok := <-c.lines:
			if !ok {
				t.Fatalf("%s: terminal closed after %d lines without the expected response", command, observedLines)
			}
			if line == want {
				return
			}
			observedLines++
		case <-retry.C:
			if command == "size" || command == "identity" {
				c.send(t, command+"\n")
			}
		case <-deadline.C:
			t.Fatalf("%s: expected response missing after %d lines", command, observedLines)
		}
	}
}
func (c *interactionClient) end(t *testing.T, kill bool) {
	t.Helper()
	if kill {
		if err := c.cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
	} else {
		c.send(t, "\x1ddetach\n")
	}
	select {
	case err := <-c.done:
		if (err != nil) != kill {
			t.Fatalf("operator disconnect: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("operator did not disconnect")
	}
}

type interactionCall struct {
	Kind, Python string
	PID          int
}

func readInteractionCalls(t *testing.T, f *interactionFixture) []interactionCall {
	t.Helper()
	var result []interactionCall
	for _, raw := range bytes.Split(bytes.TrimSpace(f.output(t, "repository/calls.jsonl")), []byte("\n")) {
		var event interactionCall
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatal("invalid interaction call record")
		}
		result = append(result, event)
	}
	return result
}
func interactionCounts(events []interactionCall, requests int) error {
	starts, calls := 0, 0
	for _, event := range events {
		switch event.Kind {
		case "agent":
			starts++
		case "model":
			calls++
		default:
			return fmt.Errorf("unexpected agent event kind")
		}
	}
	if starts != 1 {
		return fmt.Errorf("agent starts: got %d, want 1", starts)
	}
	if calls != requests {
		return fmt.Errorf("model calls: got %d, want %d", calls, requests)
	}
	return nil
}

func runInteraction(t *testing.T, role, mutation string) {
	t.Helper()
	w, server := kubeWorld(t)
	f := prepareInteraction(t, role, server.Profile.Roles[role], mutation)
	server.BindTerminal(role, f.binding, func(started string) {
		if started != role {
			t.Errorf("unexpected start: %s", started)
			return
		}
		if err := f.start(); err != nil {
			t.Error(err)
		}
	}, f.terminal)
	server.Save(t)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = f.docker(ctx, "rm", "-f", f.name)
	})
	up := func() {
		t.Helper()
		got := w.run(t, nil, "up", "example", "--role", role, "--json")
		if got.Code != 0 {
			t.Fatal("up", got)
		}
	}
	up()
	readRunning := func() kc.Identity {
		t.Helper()
		receipt, err := kc.ReadReceipt(filepath.Join(w.cellDir, "run", "kubecompat", role+".json"))
		if err != nil || receipt.State != "running" || receipt.Identity.Image != f.binding.Image {
			t.Fatal("missing candidate running receipt", receipt, err)
		}
		return receipt.Identity
	}
	before := readRunning()
	events := readInteractionCalls(t, f)
	if mutation == "duplicate-start" {
		if err := interactionCounts(events, 0); err == nil || !strings.Contains(err.Error(), "agent starts: got 2, want 1") {
			t.Fatal("duplicate start escaped", err)
		}
		return
	}
	if err := interactionCounts(events, 0); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([a-z]+[0-9]+)?$`).MatchString(events[0].Python) {
		t.Fatal("invalid fixture interpreter version")
	}
	pid := events[0].PID
	identity := fmt.Sprintf("IDENTITY %s %d", role, pid)
	client := openInteractionClient(t, w, role)
	client.await(t, "identity", identity)
	if mutation == "model-on-attach" {
		if err := interactionCounts(readInteractionCalls(t, f), 0); err == nil || !strings.Contains(err.Error(), "model calls: got 1, want 0") {
			t.Fatal("attach model call escaped", err)
		}
		client.end(t, false)
		return
	}
	client.await(t, "size", "SIZE 85x30")
	client.size(t, 117, 42)
	client.await(t, "size", "SIZE 117x42")
	input := f.output(t, "repository/input.txt")
	want := fmt.Sprintf("%s 2 %s", role, shaString(input))
	client.await(t, "work", "RESULT "+want)
	checkpoint := func() {
		t.Helper()
		if err := before.Same(readRunning()); err != nil {
			t.Fatal(err)
		}
		events := readInteractionCalls(t, f)
		if err := interactionCounts(events, 1); err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			if event.PID != pid {
				t.Fatalf("agent PID changed from %d to %d", pid, event.PID)
			}
		}
		if starts, _, _ := server.Counts(); starts != 1 {
			t.Fatal("unexpected scale/start count", starts)
		}
		if got := string(f.output(t, "repository/result.txt")); got != want+"\n" {
			t.Fatal("retained result differs from the expected synthetic work")
		}
	}
	checkpoint()
	client.end(t, false)
	up()
	checkpoint()
	client = openInteractionClient(t, w, role)
	client.await(t, "identity", identity)
	client.await(t, "read", "RETAINED "+want)
	client.end(t, true)
	checkpoint()
	client = openInteractionClient(t, w, role)
	client.await(t, "identity", identity)
	client.await(t, "read", "RETAINED "+want)
	client.end(t, false)
	checkpoint()
	revision, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	sourceStatus, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	runtimeID, err := f.docker(context.Background(), "inspect", "--format", "{{.Id}}", f.name)
	if err != nil {
		t.Fatal(err)
	}
	dockerVersion, err := f.docker(context.Background(), "version", "--format", "{{.Server.Version}}")
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"role": role,
		"cli_source": strings.TrimSpace(string(revision)), "cli_source_dirty": len(sourceStatus) != 0, "cli_sha256": shaString(fixtureFile(t, cellctlBinary(t))),
		"candidate_source": f.source, "identity": before, "profile_sha256": shaString(fixtureFile(t, server.ProfilePath)),
		"fixture_mount_owner": f.owner, "fixture_image": f.image, "fixture_container": runtimeID,
		"launcher_sha256": shaString(f.output(t, "desk-session.py")), "harness": "claude", "model": f.binding.Model,
		"agent_sha256": f.binding.AgentSHA256, "agent_version": f.binding.AgentVersion, "launch_id": f.binding.LaunchID, "pid": pid,
		"agent_starts": 1, "model_requests": 1, "result": want,
		"go": runtime.Version(), "docker": dockerVersion, "fixture_python": events[0].Python}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("INTERACTION_RECORD " + string(raw))
}

// regression: #7346
func TestKubeCompatCellctlInteraction(t *testing.T) {
	if os.Getenv("ASSAY_QUALIFY_INTERACTION") != "1" {
		t.Skip("opt-in offline fixture runtime")
	}
	for _, role := range kc.Roles {
		t.Run(role, func(t *testing.T) { runInteraction(t, role, "") })
	}
}

// regression: #7346
func TestKubeCompatInteractionCountControls(t *testing.T) {
	if os.Getenv("ASSAY_QUALIFY_INTERACTION") != "1" {
		t.Skip("opt-in offline fixture runtime")
	}
	for _, mutation := range []string{"duplicate-start", "model-on-attach"} {
		t.Run(mutation, func(t *testing.T) { runInteraction(t, "worker-desk", mutation) })
	}
}
