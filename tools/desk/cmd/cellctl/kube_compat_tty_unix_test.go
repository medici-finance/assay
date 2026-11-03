//go:build darwin || linux

package main

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

func kubeTTY(t *testing.T, w *cliWorld, limit time.Duration, args ...string) (int, string) {
	t.Helper()
	master, slave, e := openPTY()
	if e != nil {
		t.Fatal(e)
	}
	defer master.Close()
	defer slave.Close()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, cellctlBinary(t), args...)
	cmd.Env = w.baseEnv()
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	collected := make(chan string, 1)
	go func() {
		var all strings.Builder
		buf := make([]byte, 4096)
		sent := false
		re := regexp.MustCompile(`confirm code: ([0-9a-f]{8})`)
		for {
			n, e := master.Read(buf)
			if n > 0 {
				all.Write(buf[:n])
				text := all.String()
				if !sent && strings.Contains(text, "Type the confirm code") {
					hit := re.FindStringSubmatch(text)
					if len(hit) == 2 {
						_, _ = master.Write([]byte(hit[1] + "\n"))
						sent = true
					}
				}
			}
			if e != nil {
				collected <- all.String()
				return
			}
		}
	}()
	e = cmd.Wait()
	_ = slave.Close()
	_ = master.Close()
	out := <-collected
	if e == nil {
		return 0, out
	}
	if ex, ok := e.(*exec.ExitError); ok {
		return ex.ExitCode(), out
	}
	return 1, out
}
