//go:build linux

package deskkit

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// procBindChild marks the re-exec'd child of TestHotfixProcBindMount.
const procBindChild = "DESKKIT_PROCBIND_CHILD"

// regression: #6768
// TestHotfixProcBindMount: inside its own user and mount namespace a caller can bind another
// procfs file over /proc/self/uid_map and gid_map. Here that file is the thread's comm,
// rewritten to the identity text, so the filesystem type alone still reads as procfs. The
// gate must stay could-not-check. Skips where an unprivileged user namespace or the mount is
// unavailable.
func TestHotfixProcBindMount(t *testing.T) {
	if os.Getenv(procBindChild) == "1" {
		procBindChildRun()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestHotfixProcBindMount$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), procBindChild+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}
	out, err := cmd.CombinedOutput()
	s := string(out)
	switch {
	case strings.Contains(s, "PROCBIND: refused"):
		t.Log(strings.TrimSpace(s))
	case strings.Contains(s, "PROCBIND: unavailable"):
		t.Skipf("bind mount unavailable here: %s", s)
	case err != nil && !strings.Contains(s, "PROCBIND:"):
		t.Skipf("no unprivileged user and mount namespace here: %v %s", err, s)
	default:
		t.Fatalf("a procfs file bind-mounted over the id maps was not refused:\n%s", s)
	}
}

// procBindChildRun runs in the child: it plants the hostile mounts, then asks the gate.
func procBindChildRun() {
	runtime.LockOSThread()
	say := func(m string) { os.Stdout.WriteString("PROCBIND: " + m + "\n") }
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		say("unavailable: mount propagation: " + err.Error())
		return
	}
	comm := "/proc/thread-self/comm"
	if err := os.WriteFile(comm, []byte(identityIDMap), 0); err != nil {
		say("unavailable: write comm: " + err.Error())
		return
	}
	for _, p := range []string{procUIDMap, procGIDMap} {
		if err := syscall.Mount(comm, p, "", syscall.MS_BIND, ""); err != nil {
			say("unavailable: bind over " + p + ": " + err.Error())
			return
		}
	}
	if err := userNSIdentity(readProcFile, "linux"); err != nil {
		say("refused: " + err.Error())
		return
	}
	b, _ := readProcFile(procUIDMap)
	say("accepted: uid_map read as " + strings.TrimSpace(string(b)))
}
