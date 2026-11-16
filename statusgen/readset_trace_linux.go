//go:build linux && (amd64 || arm64)

package main

// The Linux read-set tracer (build-less-brittle/15, Task step 2). It runs a
// row's shell under ptrace from the first instruction, follows every child
// process and thread, and stops at every system call (PTRACE_SYSCALL, a
// superset of the brief's seccomp-trapped set: a call on the path-free list
// is stopped and passed, never classified). Each call is classified by
// sysTable; a call the table does not hold, a lost process or any default-
// deny event ends with no read set.

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// tracedRun is the outcome of one traced row.
type tracedRun struct {
	output   []byte
	exit     int  // the shell's exit status; -1 when it did not exit normally
	exited   bool // the shell exited (as opposed to a signal)
	err      error
	startErr error // the tracee never started: the host refused the tracer
	log      traceLog
}

// pendingPath is one path argument captured at syscall entry.
type pendingPath struct {
	lexical string
	existed bool // the path existed at entry (for O_CREAT)
	ok      bool
}

// pendingCall is a call between its entry and exit stops.
type pendingCall struct {
	spec   sysSpec
	nr     uint64
	args   [6]uint64
	p1, p2 pendingPath
	addr   []sockAddr
}

type tproc struct {
	tgid    int
	exe     string
	pending *pendingCall
	started bool
}

type tracer struct {
	leader   int
	armAfter int // exec events before the row's own shell
	execs    int
	procs    map[int]*tproc
	log      traceLog
	unixBind map[string]bool   // abstract and path unix names bound in this row
	goTool   func(string) bool // reports whether an executable is $GOROOT/bin/go
	mu       sync.Mutex        // guards procs for the deadline killer
}

func (t *tracer) armed() bool { return t.execs >= t.armAfter }

func (t *tracer) deny(why string) {
	if t.log.Denied == "" && t.armed() {
		t.log.Denied = why
	}
}

// ptraceOptions follow every child from its first instruction, report execs,
// mark syscall stops, and kill every tracee if statusgen dies.
const ptraceOptions = unix.PTRACE_O_TRACESYSGOOD | unix.PTRACE_O_TRACECLONE | unix.PTRACE_O_TRACEFORK |
	unix.PTRACE_O_TRACEVFORK | unix.PTRACE_O_TRACEEXEC | unix.PTRACE_O_EXITKILL

// runTraced starts cmd under the tracer and drives it to completion. cmd's
// Stdout/Stderr are replaced by one pipe. It never calls cmd.Wait: the tracer
// reaps every tracee itself.
func runTraced(ctx context.Context, cmd *exec.Cmd, armAfter int, goTool func(string) bool) tracedRun {
	done := make(chan tracedRun, 1)
	go func() {
		// Every ptrace request must come from the thread that started the
		// tracee; the goroutine keeps that thread until it returns.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		done <- traceOnThread(ctx, cmd, armAfter, goTool)
	}()
	return <-done
}

func traceOnThread(ctx context.Context, cmd *exec.Cmd, armAfter int, goTool func(string) bool) tracedRun {
	pr, pw, err := os.Pipe()
	if err != nil {
		return tracedRun{exit: -1, startErr: err}
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Ptrace = true
	cmd.SysProcAttr.Setpgid = true
	cmd.Cancel = nil
	var out bytes.Buffer
	copied := make(chan struct{})
	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return tracedRun{exit: -1, startErr: err}
	}
	pw.Close()
	go func() {
		_, _ = io.Copy(&out, pr)
		pr.Close()
		close(copied)
	}()
	t := &tracer{leader: cmd.Process.Pid, armAfter: armAfter, goTool: goTool, procs: map[int]*tproc{}, unixBind: map[string]bool{},
		log: traceLog{Created: map[string]bool{}, Pids: map[int]bool{}}}
	stopKill := context.AfterFunc(ctx, func() {
		_ = syscall.Kill(-t.leader, syscall.SIGKILL)
		t.mu.Lock()
		for tid := range t.procs {
			_ = syscall.Kill(tid, syscall.SIGKILL)
		}
		t.mu.Unlock()
	})
	exit, exited := t.loop()
	stopKill()
	<-copied
	_ = cmd.Process.Release()
	return tracedRun{output: out.Bytes(), exit: exit, exited: exited, log: t.log}
}

// loop is the wait loop. It returns the leader's exit status.
func (t *tracer) loop() (int, bool) {
	exit, exited := -1, false
	first := true
	for {
		var ws syscall.WaitStatus
		tid, err := syscall.Wait4(-1, &ws, syscall.WALL|wNoThread, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			if err != syscall.ECHILD {
				t.deny("the tracer lost a process")
			}
			return exit, exited
		}
		if ws.Exited() || ws.Signaled() {
			t.mu.Lock()
			delete(t.procs, tid)
			t.mu.Unlock()
			if tid == t.leader {
				if ws.Exited() {
					exit, exited = ws.ExitStatus(), true
				}
			}
			continue
		}
		if !ws.Stopped() {
			continue
		}
		p := t.proc(tid)
		sig := ws.StopSignal()
		switch {
		case first && tid == t.leader && sig == syscall.SIGTRAP:
			// The PTRACE_TRACEME stop after the first exec.
			first = false
			p.started = true
			if err := unix.PtraceSetOptions(tid, ptraceOptions); err != nil {
				t.deny("the tracer could not set its options")
			}
			p.exe = readExe(tid)
			t.resume(tid, 0)
		case sig == syscall.SIGTRAP|0x80:
			t.syscallStop(tid, p)
			t.resume(tid, 0)
		case sig == syscall.SIGTRAP && ws.TrapCause() > 0:
			t.event(tid, p, ws.TrapCause())
			t.resume(tid, 0)
		case sig == syscall.SIGSTOP && !p.started:
			// A new child's first stop.
			p.started = true
			t.resume(tid, 0)
		default:
			p.started = true
			t.resume(tid, int(sig))
		}
	}
}

func (t *tracer) resume(tid, sig int) { _ = unix.PtraceSyscall(tid, sig) }

func (t *tracer) proc(tid int) *tproc {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.procs[tid]
	if !ok {
		p = &tproc{tgid: tgidOf(tid), exe: readExe(tid)}
		t.procs[tid] = p
	}
	t.log.Pids[p.tgid] = true
	return p
}

func tgidOf(tid int) int {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", tid))
	if err != nil {
		return tid
	}
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, "Tgid:"); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				return n
			}
		}
	}
	return tid
}

func readExe(tid int) string {
	s, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", tid))
	if err != nil {
		return ""
	}
	return s
}

// event handles a ptrace event stop.
func (t *tracer) event(tid int, p *tproc, cause int) {
	switch cause {
	case unix.PTRACE_EVENT_CLONE, unix.PTRACE_EVENT_FORK, unix.PTRACE_EVENT_VFORK:
		msg, err := unix.PtraceGetEventMsg(tid)
		if err != nil {
			t.deny("the tracer lost a process")
			return
		}
		child := t.proc(int(msg))
		if child.exe == "" {
			child.exe = p.exe
		}
	case unix.PTRACE_EVENT_EXEC:
		former, _ := unix.PtraceGetEventMsg(tid)
		var call *pendingCall
		if int(former) != tid {
			t.mu.Lock()
			if fp := t.procs[int(former)]; fp != nil {
				call = fp.pending
			}
			delete(t.procs, int(former))
			t.mu.Unlock()
		} else {
			call = p.pending
		}
		p.pending = nil
		wasArmed := t.armed()
		t.execs++
		p.exe = readExe(tid)
		if !wasArmed {
			return
		}
		if call != nil && call.p1.ok {
			t.record(p, accExec, call.p1.lexical, false)
		}
		if p.exe != "" {
			t.record(p, accExec, p.exe, false)
		}
		t.execEnv(tid, p)
	}
}

// execEnv captures what an exec carries: a go-run entry for the go tool and
// any GIT_DIR / GIT_COMMON_DIR the new image sees.
func (t *tracer) execEnv(tid int, p *tproc) {
	env := map[string]string{}
	if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", tid)); err == nil {
		for _, kv := range strings.Split(string(b), "\x00") {
			if k, v, ok := strings.Cut(kv, "="); ok {
				env[k] = v
			}
		}
	}
	cwd, _ := os.Readlink(fmt.Sprintf("/proc/%d/cwd", tid))
	for _, k := range []string{"GIT_DIR", "GIT_COMMON_DIR"} {
		if v := env[k]; v != "" {
			if !filepath.IsAbs(v) {
				v = filepath.Join(cwd, v)
			}
			t.log.GitDirs = append(t.log.GitDirs, resolveBest(filepath.Clean(v)))
		}
	}
	if t.goTool != nil && t.goTool(p.exe) {
		b, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", tid))
		args := strings.Split(strings.TrimSuffix(string(b), "\x00"), "\x00")
		t.log.GoRuns = append(t.log.GoRuns, traceGoRun{Cwd: resolveBest(cwd), Args: args, Env: env})
	}
}

// syscallInfo is struct ptrace_syscall_info.
type syscallInfo struct {
	op    uint8
	arch  uint32
	nr    uint64
	args  [6]uint64
	rval  int64
	isErr bool
}

func getSyscallInfo(tid int) (syscallInfo, error) {
	var buf [88]byte
	_, _, e := unix.Syscall6(unix.SYS_PTRACE, unix.PTRACE_GET_SYSCALL_INFO, uintptr(tid), uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])), 0, 0)
	if e != 0 {
		return syscallInfo{}, e
	}
	le := binary.LittleEndian
	si := syscallInfo{op: buf[0], arch: le.Uint32(buf[4:])}
	switch si.op {
	case unix.PTRACE_SYSCALL_INFO_ENTRY, unix.PTRACE_SYSCALL_INFO_SECCOMP:
		si.nr = le.Uint64(buf[24:])
		for i := range si.args {
			si.args[i] = le.Uint64(buf[32+8*i:])
		}
	case unix.PTRACE_SYSCALL_INFO_EXIT:
		si.rval = int64(le.Uint64(buf[24:]))
		si.isErr = buf[32] != 0
	}
	return si, nil
}

// syscallStop handles one syscall entry or exit stop.
func (t *tracer) syscallStop(tid int, p *tproc) {
	si, err := getSyscallInfo(tid)
	if err == syscall.ESRCH {
		// The thread was killed while stopped (another thread's exit_group):
		// it runs no further, so its call reaches nothing.
		p.pending = nil
		return
	}
	if err != nil {
		t.deny("the tracer could not read a system call")
		return
	}
	switch si.op {
	case unix.PTRACE_SYSCALL_INFO_ENTRY:
		p.pending = nil
		if !t.armed() {
			return
		}
		if si.arch != auditArchNative {
			t.deny("a system call from another ABI")
			return
		}
		spec, ok := sysTable[si.nr]
		if !ok {
			t.deny("an unclassified system call (" + strconv.FormatUint(si.nr, 10) + ")")
			return
		}
		if spec.kind == skFree {
			return
		}
		p.pending = t.entry(tid, p, spec, si)
	case unix.PTRACE_SYSCALL_INFO_EXIT:
		call := p.pending
		p.pending = nil
		if call == nil || !t.armed() {
			return
		}
		t.exit(tid, p, call, si.rval)
	}
}

func argInt(a uint64) int { return int(int32(uint32(a))) }

// entry captures a call's arguments while the tracee is stopped at entry.
func (t *tracer) entry(tid int, p *tproc, spec sysSpec, si syscallInfo) *pendingCall {
	c := &pendingCall{spec: spec, nr: si.nr, args: si.args}
	a := si.args
	switch spec.kind {
	case skDeny:
		t.deny("a call the recorder does not trace (" + spec.name + ")")
		return nil
	case skProc:
		for _, i := range []int8{spec.dirfd, spec.path} {
			if i >= 0 && !t.traced(argInt(a[i])) {
				t.deny("a call reaching into another process (" + spec.name + ")")
				return nil
			}
		}
		return nil
	case skPidfdFd:
		if !t.traced(pidfdTarget(p.tgid, argInt(a[0]))) {
			t.deny("a call reaching into another process (pidfd_getfd)")
		}
		return nil
	case skClone:
		if a[0]&nsFlags != 0 {
			t.deny("a namespace, mount or root change (clone)")
		}
		return nil
	case skClone3:
		b := readMem(tid, uintptr(a[0]), 8)
		if len(b) < 8 || binary.LittleEndian.Uint64(b)&nsFlags != 0 {
			t.deny("a namespace, mount or root change (clone3)")
		}
		return nil
	case skBind:
		c.addr = []sockAddr{readSockAddr(tid, uintptr(a[1]), int(a[2]))}
		return c
	case skConnect, skSendto, skSendmsg, skSendmmsg:
		// An outbound address is judged at entry: the attempt itself is the
		// event, whether or not it then succeeds.
		var addrs []sockAddr
		switch spec.kind {
		case skConnect:
			addrs = []sockAddr{readSockAddr(tid, uintptr(a[1]), int(a[2]))}
		case skSendto:
			if a[4] != 0 {
				addrs = []sockAddr{readSockAddr(tid, uintptr(a[4]), int(a[5]))}
			}
		case skSendmsg:
			if sa, ok := msgName(tid, uintptr(a[1])); ok {
				addrs = []sockAddr{sa}
			}
		case skSendmmsg:
			n := int(a[2])
			if n > 1024 {
				n = 1024
			}
			for i := 0; i < n; i++ {
				// struct mmsghdr is a 56-byte msghdr, a length and padding.
				if sa, ok := msgName(tid, uintptr(a[1])+uintptr(64*i)); ok {
					addrs = append(addrs, sa)
				}
			}
		}
		for _, sa := range addrs {
			if why := t.connectOK(tid, sa); why != "" {
				t.deny(why)
				break
			}
		}
		return nil
	case skList:
		c.p1 = t.fdPath(tid, p, argInt(a[0]))
		return c
	}
	flags := uint64(0)
	if spec.flags >= 0 {
		flags = a[spec.flags]
	}
	if spec.kind == skOpenat2 {
		b := readMem(tid, uintptr(a[2]), 8)
		if len(b) == 8 {
			flags = binary.LittleEndian.Uint64(b)
		}
	}
	if spec.path >= 0 {
		if a[spec.path] == 0 {
			// A NULL path (utimensat) acts on the descriptor.
			if spec.kind == skMutate {
				return nil
			}
		}
		raw := readString(tid, uintptr(a[spec.path]))
		if raw == "" && flags&unix.AT_EMPTY_PATH != 0 && spec.dirfd >= 0 {
			if spec.kind != skExec {
				return nil // fstat-like on a descriptor already recorded
			}
			c.p1 = t.fdPath(tid, p, argInt(a[spec.dirfd]))
		} else {
			c.p1 = t.lexical(tid, p, spec.dirfd, a, raw)
		}
	}
	if spec.path2 >= 0 {
		c.p2 = t.lexical(tid, p, spec.dirfd2, a, readString(tid, uintptr(a[spec.path2])))
	}
	for _, pp := range []*pendingPath{&c.p1, &c.p2} {
		if pp.ok {
			_, err := os.Lstat(pp.lexical)
			pp.existed = err == nil
		}
	}
	return c
}

const nsFlags = unix.CLONE_NEWNS | unix.CLONE_NEWUTS | unix.CLONE_NEWIPC | unix.CLONE_NEWUSER | unix.CLONE_NEWPID |
	unix.CLONE_NEWNET | unix.CLONE_NEWCGROUP | unix.CLONE_NEWTIME

func (t *tracer) traced(pid int) bool {
	if pid <= 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.log.Pids[pid] {
		return true
	}
	_, ok := t.procs[pid]
	return ok
}

// exit finishes a call at its exit stop, now that its result is known.
func (t *tracer) exit(tid int, p *tproc, c *pendingCall, rval int64) {
	failed := rval < 0 && rval > -4096
	errno := syscall.Errno(0)
	if failed {
		errno = syscall.Errno(-rval)
	}
	missing := errno == syscall.ENOENT || errno == syscall.ENOTDIR
	switch c.spec.kind {
	case skBind:
		if !failed && len(c.addr) == 1 && c.addr[0].unix != "" {
			name := c.addr[0].unix
			if !strings.HasPrefix(name, "@") {
				name = t.absFrom(tid, name)
				t.log.Created[resolveBest(name)] = true
			}
			t.unixBind[name] = true
		}
	case skList:
		if c.p1.ok && !failed {
			t.record(p, accList, c.p1.lexical, false)
		}
	case skOpen, skOpenat2:
		flags := uint64(0)
		if c.spec.flags >= 0 {
			flags = c.args[c.spec.flags]
		} else if c.spec.flags == -2 {
			flags = unix.O_CREAT | unix.O_WRONLY | unix.O_TRUNC
		}
		if c.spec.kind == skOpenat2 {
			flags = 0
			if b := readMem(tid, uintptr(c.args[2]), 8); len(b) == 8 {
				flags = binary.LittleEndian.Uint64(b)
			}
		}
		if !c.p1.ok {
			return
		}
		if failed {
			t.record(p, accStat, c.p1.lexical, missing)
			return
		}
		if flags&unix.O_CREAT != 0 && !c.p1.existed {
			t.log.Created[resolveBest(c.p1.lexical)] = true
			return
		}
		if flags&unix.O_ACCMODE == unix.O_WRONLY {
			t.record(p, accStat, c.p1.lexical, false)
			return
		}
		t.record(p, accRead, c.p1.lexical, false)
	case skStat, skMutate:
		if c.p1.ok {
			t.record(p, accStat, c.p1.lexical, failed && missing)
		}
	case skCreate:
		if !c.p1.ok {
			return
		}
		if !failed {
			t.log.Created[resolveBest(c.p1.lexical)] = true
		} else {
			t.record(p, accStat, c.p1.lexical, missing)
		}
	case skLink, skRename:
		if c.p1.ok {
			kind := accStat
			if c.spec.kind == skLink {
				kind = accRead
			}
			t.record(p, kind, c.p1.lexical, failed && missing)
		}
		fallthrough
	case skSymlink:
		if c.p2.ok {
			if !failed {
				t.log.Created[resolveBest(c.p2.lexical)] = true
			} else {
				t.record(p, accStat, c.p2.lexical, missing)
			}
		}
	case skExec:
		// A successful exec is recorded at its PTRACE_EVENT_EXEC stop; a
		// failed one is a probe.
		if failed && c.p1.ok {
			t.record(p, accStat, c.p1.lexical, missing)
		}
	}
}

// record resolves a lexical path and appends the access.
func (t *tracer) record(p *tproc, kind traceAccess, lexical string, absent bool) {
	if lexical == "" {
		return
	}
	resolved := resolveBest(lexical)
	t.log.Events = append(t.log.Events, traceEvent{Kind: kind, Path: resolved, Links: linksOn(lexical), Absent: absent, Exe: p.exe})
}

// lexical turns a path argument into an absolute lexical path, resolving it
// against the tracee's directory descriptor or working directory and turning
// /proc/self, /dev/fd and /proc/<pid>/fd links into their targets.
func (t *tracer) lexical(tid int, p *tproc, dirfdIdx int8, a [6]uint64, raw string) pendingPath {
	if raw == "" {
		return pendingPath{}
	}
	if !filepath.IsAbs(raw) {
		dirfd := atFDCWD
		if dirfdIdx >= 0 {
			dirfd = argInt(a[dirfdIdx])
		}
		var base string
		if dirfd == atFDCWD {
			base, _ = os.Readlink(fmt.Sprintf("/proc/%d/cwd", tid))
		} else {
			bp := t.fdPath(tid, p, dirfd)
			if !bp.ok {
				return pendingPath{}
			}
			base = bp.lexical
		}
		if !filepath.IsAbs(base) {
			return pendingPath{}
		}
		raw = filepath.Join(base, raw)
	}
	l := filepath.Clean(raw)
	l = procMagic(tid, p.tgid, l)
	if l == "" {
		return pendingPath{}
	}
	return pendingPath{lexical: l, ok: true}
}

// absFrom is name resolved against the tracee's working directory.
func (t *tracer) absFrom(tid int, name string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	cwd, _ := os.Readlink(fmt.Sprintf("/proc/%d/cwd", tid))
	return filepath.Join(cwd, name)
}

// fdPath is the absolute path a tracee descriptor names, or !ok for a pipe,
// socket or other non-file.
func (t *tracer) fdPath(tid int, p *tproc, fd int) pendingPath {
	s, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%d", tid, fd))
	if err != nil || !filepath.IsAbs(s) {
		return pendingPath{}
	}
	return pendingPath{lexical: strings.TrimSuffix(s, " (deleted)"), ok: true}
}

// procMagic rewrites the /proc and /dev/fd aliases a tracee sees to what they
// name for it. A descriptor naming a non-file returns "".
func procMagic(tid, tgid int, l string) string {
	rewrite := func(prefix, with string) {
		if l == prefix || strings.HasPrefix(l, prefix+"/") {
			l = with + strings.TrimPrefix(l, prefix)
		}
	}
	rewrite("/proc/self", fmt.Sprintf("/proc/%d", tgid))
	rewrite("/proc/thread-self", fmt.Sprintf("/proc/%d/task/%d", tgid, tid))
	rewrite("/dev/fd", fmt.Sprintf("/proc/%d/fd", tgid))
	rewrite("/dev/stdin", fmt.Sprintf("/proc/%d/fd/0", tgid))
	rewrite("/dev/stdout", fmt.Sprintf("/proc/%d/fd/1", tgid))
	rewrite("/dev/stderr", fmt.Sprintf("/proc/%d/fd/2", tgid))
	parts := strings.Split(strings.TrimPrefix(l, "/"), "/")
	if len(parts) >= 3 && parts[0] == "proc" {
		if _, err := strconv.Atoi(parts[1]); err == nil {
			n := 0
			switch {
			case parts[2] == "fd" && len(parts) >= 4:
				n = 4
			case parts[2] == "cwd" || parts[2] == "root" || parts[2] == "exe":
				n = 3
			}
			if n > 0 {
				target, err := os.Readlink("/" + strings.Join(parts[:n], "/"))
				if err != nil || !filepath.IsAbs(target) {
					if err != nil {
						return l
					}
					return ""
				}
				return filepath.Join(append([]string{target}, parts[n:]...)...)
			}
		}
	}
	return l
}

// linksOn lists the symbolic links on a lexical path's own components.
func linksOn(lexical string) []string {
	var out []string
	cur := "/"
	for _, part := range strings.Split(strings.TrimPrefix(lexical, "/"), "/") {
		if part == "" {
			continue
		}
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		if err != nil {
			break
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			out = append(out, cur)
			r, err := filepath.EvalSymlinks(cur)
			if err != nil {
				break
			}
			cur = r
		}
	}
	return out
}

func readMem(tid int, addr uintptr, n int) []byte {
	if addr == 0 || n <= 0 {
		return nil
	}
	f, err := os.Open(fmt.Sprintf("/proc/%d/mem", tid))
	if err != nil {
		return nil
	}
	defer f.Close()
	b := make([]byte, n)
	m, _ := f.ReadAt(b, int64(addr))
	return b[:m]
}

// readString reads a NUL-terminated string (at most PATH_MAX bytes).
func readString(tid int, addr uintptr) string {
	if addr == 0 {
		return ""
	}
	f, err := os.Open(fmt.Sprintf("/proc/%d/mem", tid))
	if err != nil {
		return ""
	}
	defer f.Close()
	var out []byte
	buf := make([]byte, 256)
	for len(out) < 4096 {
		m, _ := f.ReadAt(buf, int64(addr)+int64(len(out)))
		if m == 0 {
			break
		}
		if i := bytes.IndexByte(buf[:m], 0); i >= 0 {
			return string(append(out, buf[:i]...))
		}
		out = append(out, buf[:m]...)
	}
	return string(out)
}

// sockAddr is a decoded socket address.
type sockAddr struct {
	family uint16
	ip     net.IP
	port   int
	unix   string // a path, or "@name" for an abstract socket
	bad    bool
}

func readSockAddr(tid int, addr uintptr, n int) sockAddr {
	if n > 128 {
		n = 128
	}
	b := readMem(tid, addr, n)
	if len(b) < 2 {
		return sockAddr{bad: true}
	}
	sa := sockAddr{family: binary.LittleEndian.Uint16(b)}
	switch sa.family {
	case unix.AF_UNIX:
		name := b[2:]
		if len(name) > 0 && name[0] == 0 {
			sa.unix = "@" + string(bytes.TrimRight(name[1:], "\x00"))
		} else if i := bytes.IndexByte(name, 0); i >= 0 {
			sa.unix = string(name[:i])
		} else {
			sa.unix = string(name)
		}
	case unix.AF_INET:
		if len(b) < 8 {
			return sockAddr{bad: true}
		}
		sa.port = int(binary.BigEndian.Uint16(b[2:]))
		sa.ip = net.IP(append([]byte(nil), b[4:8]...))
	case unix.AF_INET6:
		if len(b) < 24 {
			return sockAddr{bad: true}
		}
		sa.port = int(binary.BigEndian.Uint16(b[2:]))
		sa.ip = net.IP(append([]byte(nil), b[8:24]...))
	}
	return sa
}

// msgName decodes a msghdr's destination address, ok false when it has none.
func msgName(tid int, msg uintptr) (sockAddr, bool) {
	b := readMem(tid, msg, 12)
	if len(b) < 12 {
		return sockAddr{bad: true}, true
	}
	ptr, n := binary.LittleEndian.Uint64(b), binary.LittleEndian.Uint32(b[8:])
	if ptr == 0 || n == 0 {
		return sockAddr{}, false
	}
	return readSockAddr(tid, uintptr(ptr), int(n)), true
}

// connectOK decides one outbound address: loopback or a local socket only, and
// only to a listener a traced process holds.
func (t *tracer) connectOK(tid int, sa sockAddr) string {
	switch {
	case sa.bad:
		return "a connect the tracer could not decode"
	case sa.family == unix.AF_UNSPEC || sa.family == unix.AF_NETLINK:
		return ""
	case sa.family == unix.AF_UNIX:
		name := sa.unix
		if !strings.HasPrefix(name, "@") {
			name = resolveBest(t.absFrom(tid, name))
		}
		if t.unixBind[name] || t.unixBind[sa.unix] {
			return ""
		}
		if !strings.HasPrefix(name, "@") {
			// A path naming nothing reaches no one (glibc probes the nscd
			// socket this way): the attempt exchanges nothing.
			if _, err := os.Lstat(name); errors.Is(err, os.ErrNotExist) {
				return ""
			}
		}
		return "a connect to a local socket no traced process listens on"
	case sa.family == unix.AF_INET || sa.family == unix.AF_INET6:
		if !sa.ip.IsLoopback() {
			return "a connect to a non-loopback address"
		}
		if t.listenerTraced(tid, sa.port) {
			return ""
		}
		return "a connect to a local socket no traced process listens on"
	}
	return "a connect to an unclassified address family"
}

// listenerTraced reports whether a socket bound to the loopback port is held
// by a traced process.
func (t *tracer) listenerTraced(tgid, port int) bool {
	inodes := map[string]bool{}
	for _, f := range []string{"tcp", "tcp6", "udp", "udp6"} {
		b, err := os.ReadFile(fmt.Sprintf("/proc/%d/net/%s", tgid, f))
		if err != nil {
			continue
		}
		for _, l := range strings.Split(string(b), "\n")[1:] {
			fs := strings.Fields(l)
			if len(fs) < 10 {
				continue
			}
			_, ps, ok := strings.Cut(fs[1], ":")
			if !ok {
				continue
			}
			n, err := strconv.ParseInt(ps, 16, 32)
			if err != nil || int(n) != port {
				continue
			}
			if strings.HasPrefix(f, "tcp") && fs[3] != "0A" {
				continue
			}
			inodes["socket:["+fs[9]+"]"] = true
		}
	}
	if len(inodes) == 0 {
		return false
	}
	t.mu.Lock()
	pids := make([]int, 0, len(t.log.Pids))
	for pid := range t.log.Pids {
		pids = append(pids, pid)
	}
	t.mu.Unlock()
	for _, pid := range pids {
		ents, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
		if err != nil {
			continue
		}
		for _, e := range ents {
			if s, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", pid, e.Name())); err == nil && inodes[s] {
				return true
			}
		}
	}
	return false
}

// pidfdTarget is the pid behind a tracee's pidfd, or 0.
func pidfdTarget(tgid, fd int) int {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/fdinfo/%d", tgid, fd))
	if err != nil {
		return 0
	}
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, "Pid:"); ok {
			n, _ := strconv.Atoi(strings.TrimSpace(v))
			return n
		}
	}
	return 0
}

// wNoThread (__WNOTHREAD) keeps the wait loop to this thread's own children
// and tracees, so a concurrent exec elsewhere in statusgen is never reaped
// here.
const wNoThread = 0x20000000
