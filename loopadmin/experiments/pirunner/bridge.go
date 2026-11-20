//go:build pirunner

// Package pirunner is an experiment, not a supported adapter: it evaluates
// whether one pinned Pi Durable harness, driven through a private JSON-lines
// bridge in a Node child process, can satisfy the runner contract and recover
// from real process kills. Every file carries the pirunner build tag, so the
// default build, vet and mutation gate never select it. It opens no listener,
// passes no credential to the child and makes no provider call: every model
// turn is the bridge's in-process faux provider. Fault controls are test-owned
// and never reachable from a managed profile.
package pirunner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// errBridgeGone: the child exited (or was killed) before answering. The
// operation's outcome is unknown, never a definite failure.
var errBridgeGone = errors.New("pirunner: bridge process gone before answering")

// msg is one decoded bridge line.
type msg = map[string]any

// bridge is one Node child running node/dist/bridge.js over a state directory.
type bridge struct {
	cmd  *exec.Cmd
	in   io.WriteCloser
	wmu  sync.Mutex
	pmu  sync.Mutex
	next int64
	pend map[int64]chan msg
	done chan struct{}

	// onCallback answers a callback; returning nil leaves it unanswered (the
	// lost-reply fault). onEvent records a counter event.
	onCallback func(op string, m msg) msg
	onEvent    func(name string, m msg)
	stderr     *bytes.Buffer
}

// startBridge spawns the child with a minimal environment: PATH only, plus an
// explicit offline KUBECONFIG and the optional trace switch. Nothing else from
// the parent environment (and so no credential) reaches the Node process.
func startBridge(node, script, dir string, onCallback func(string, msg) msg, onEvent func(string, msg)) (*bridge, error) {
	cmd := exec.Command(node, script, dir)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "KUBECONFIG=/dev/null", "NODE_NO_WARNINGS=1"}
	if os.Getenv("PIRUNNER_TRACE") == "1" {
		cmd.Env = append(cmd.Env, "PIRUNNER_TRACE=1")
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	b := &bridge{cmd: cmd, in: in, pend: map[int64]chan msg{}, done: make(chan struct{}), onCallback: onCallback, onEvent: onEvent, stderr: &bytes.Buffer{}}
	cmd.Stderr = &lockedWriter{w: b.stderr}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go b.read(out)
	return b, nil
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func (b *bridge) read(out io.Reader) {
	defer close(b.done)
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 64*1024), 16<<20)
	for sc.Scan() {
		var m msg
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		switch {
		case m["event"] != nil:
			if b.onEvent != nil {
				b.onEvent(fmt.Sprint(m["event"]), m)
			}
		case m["cb"] != nil:
			cb := m["cb"]
			go func() {
				ans := b.onCallback(fmt.Sprint(m["op"]), m)
				if ans == nil {
					return
				}
				ans["cb"] = cb
				_ = b.write(ans)
			}()
		default:
			id, _ := m["id"].(float64)
			b.pmu.Lock()
			ch := b.pend[int64(id)]
			delete(b.pend, int64(id))
			b.pmu.Unlock()
			if ch != nil {
				ch <- m
			}
		}
	}
}

func (b *bridge) write(m msg) error {
	line, err := json.Marshal(m)
	if err != nil {
		return err
	}
	b.wmu.Lock()
	defer b.wmu.Unlock()
	_, err = b.in.Write(append(line, '\n'))
	return err
}

// call sends one request and waits for its answer, the child's exit or ctx.
func (b *bridge) call(ctx context.Context, op string, fields msg) (msg, error) {
	ch := make(chan msg, 1)
	b.pmu.Lock()
	b.next++
	id := b.next
	b.pend[id] = ch
	b.pmu.Unlock()
	req := msg{"id": id, "op": op}
	for k, v := range fields {
		req[k] = v
	}
	if err := b.write(req); err != nil {
		return nil, errBridgeGone
	}
	select {
	case m := <-ch:
		return m, nil
	case <-b.done:
		return nil, errBridgeGone
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// kill SIGKILLs the child and returns only once the process has been reaped,
// so a successor never opens the same store while it still runs.
func (b *bridge) kill() error {
	_ = b.cmd.Process.Signal(syscall.SIGKILL)
	return b.wait()
}

func (b *bridge) wait() error {
	select {
	case <-b.done:
	case <-time.After(30 * time.Second):
		return errors.New("pirunner: bridge did not exit")
	}
	_ = b.cmd.Wait()
	if b.cmd.ProcessState == nil {
		return errors.New("pirunner: bridge exit not observed")
	}
	return nil
}

// close asks the child to close its stores and exit, then reaps it.
func (b *bridge) close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := b.call(ctx, "close", nil); err != nil && !errors.Is(err, errBridgeGone) {
		return b.kill()
	}
	return b.wait()
}
