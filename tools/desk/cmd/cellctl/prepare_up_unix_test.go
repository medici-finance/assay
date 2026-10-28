//go:build darwin || linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func cellPrepUp(t *testing.T) {
	for _, n := range []int{1, 5} {
		t.Run(fmt.Sprintf("up%d", n), func(t *testing.T) {
			roles := strings.Join(upFiveRoles[:n], " ")
			w := newUpWorld(t, roles, 10*time.Millisecond)
			c := w.cell()
			hashes := retainedHistory(t, c, 10)
			began := time.Now()
			result := w.up(t, "--role", upFiveRoles[0])
			if n == 5 {
				result = w.up(t)
			}
			if result.Code != 0 {
				t.Fatalf("actual up exit=%d stderr=%s", result.Code, result.Stderr)
			}
			receipt, err := os.ReadFile(filepath.Join(c.Dir, "run", "scratch", "preparation.json"))
			must(t, err)
			again := w.up(t, "--role", upFiveRoles[0])
			if again.Code != 0 {
				t.Fatal(again.Stderr)
			}
			after, err := os.ReadFile(filepath.Join(c.Dir, "run", "scratch", "preparation.json"))
			must(t, err)
			if string(receipt) != string(after) {
				t.Fatal("attach repeated shared preparation")
			}
			assertHistory(t, hashes)
			t.Logf("actual-up roles=%d protected-files=500 samples=1 wall=%s", n, time.Since(began))
		})
	}
}

// A real built desk holds its production role lease while an actual up and a
// second direct desk race it. The fake harness records each actual start.
func cellConcurrentStarts(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.prepareLocalLaunch(t)
	path := filepath.Join(f.cellDir, "cell.env")
	base, err := os.ReadFile(path)
	must(t, err)
	must(t, os.WriteFile(path, append(base, []byte("\nROLES=worker-desk\n")...), 0600))
	marker, stop := filepath.Join(f.cellDir, "starts"), filepath.Join(f.cellDir, "release")
	script := "#!/bin/sh\ncase $1 in\n--version) echo '2.1.278 (stub)';exit 0;;\nplugin) exit 0;;\nesac\nprintf 'started\\n' >> " + bashQuote(marker) + "\nwhile [ ! -f " + bashQuote(stop) + " ]; do sleep 0.02; done\n"
	must(t, os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(script), 0700))
	must(t, os.WriteFile(filepath.Join(f.binDir, "tmux"), []byte("#!/bin/sh\nexit 0\n"), 0700))
	hashes := retainedHistory(t, &Cell{Dir: f.cellDir}, 10)
	done := make(chan runResult, 1)
	go func() {
		done <- f.run(t, []string{"CELLCTL_DESKWT=0"}, "desk", "example", "worker-desk", "--cadence", "off")
	}()
	t.Cleanup(func() { _ = os.WriteFile(stop, nil, 0600) })
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("actual desk did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	up := f.run(t, []string{"CELLCTL_DESKWT=0"}, "up", "example", "--role", "worker-desk", "--no-the-desk", "--cockpit", "tmux", "--no-attach")
	if up.code != 0 || !strings.Contains(up.stdout, "already-up") {
		t.Fatalf("up did not preserve live direct owner: %+v", up)
	}
	second := f.run(t, []string{"CELLCTL_DESKWT=0"}, "desk", "example", "worker-desk", "--cadence", "off")
	if second.code == 0 {
		t.Fatal("second direct start bypassed role ownership")
	}
	starts, err := os.ReadFile(marker)
	must(t, err)
	if string(starts) != "started\n" {
		t.Fatal("more than one actual harness started", string(starts))
	}
	must(t, os.WriteFile(stop, nil, 0600))
	select {
	case first := <-done:
		if first.code != 0 {
			t.Fatalf("first role failed: %+v", first)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first role did not finish")
	}
	down := f.run(t, nil, "down", "example", "--cockpit", "tmux", "--keep-deskd")
	if down.code != 0 {
		t.Fatalf("actual clean down failed: %+v", down)
	}
	assertHistory(t, hashes)
}
