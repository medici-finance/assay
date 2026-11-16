//go:build linux && (amd64 || arm64)

package main

import "golang.org/x/sys/unix"

// sysKind is how the tracer treats one system call (Task step 2). A call
// absent from sysTable is unclassified, and an unclassified call writes no
// read set: the table is an allowlist, never a denylist.
type sysKind uint8

const (
	skFree     sysKind = iota + 1 // takes no path and reaches no other process
	skOpen                        // open family: dirfd, path, flags
	skOpenat2                     // openat2: flags inside struct open_how
	skStat                        // stat/probe family; flags may carry AT_EMPTY_PATH
	skMutate                      // changes a path without reading it
	skCreate                      // creates the path (mkdir, mknod)
	skLink                        // link: the source is read, the target created
	skRename                      // rename: the source stat-ed, the target created
	skSymlink                     // symlink: the target path created
	skList                        // getdents on a descriptor
	skExec                        // execve / execveat
	skConnect                     // connect(fd, addr, len)
	skSendto                      // sendto with a destination address
	skSendmsg                     // sendmsg: msghdr.msg_name
	skSendmmsg                    // sendmmsg: each mmsghdr's msg_name
	skBind                        // bind: a unix path is created
	skClone                       // clone: flags in arg 0
	skClone3                      // clone3: flags inside struct clone_args
	skProc                        // reaches another process by pid
	skPidfdFd                     // pidfd_getfd: the process behind a pidfd
	skDeny                        // namespace, mount or root change
)

// sysSpec places one call's arguments. -1 means "not present": a missing
// dirfd is AT_FDCWD, a missing flags argument is zero.
type sysSpec struct {
	kind          sysKind
	dirfd, path   int8
	flags         int8
	dirfd2, path2 int8
	name          string // for a deny reason
}

const atFDCWD = -100

func free() sysSpec { return sysSpec{kind: skFree} }
func at(k sysKind, d, p, f int8) sysSpec {
	return sysSpec{kind: k, dirfd: d, path: p, flags: f, dirfd2: -1, path2: -1}
}
func two(k sysKind, d, p, d2, p2 int8) sysSpec {
	return sysSpec{kind: k, dirfd: d, path: p, flags: -1, dirfd2: d2, path2: p2}
}
func deny(name string) sysSpec { return sysSpec{kind: skDeny, name: name} }

// sysTable is the classification of every system call the tracer knows, for
// the calls both arm64 and amd64 carry. sysTableArch adds the per-ABI ones.
var sysTable = func() map[uint64]sysSpec {
	t := map[uint64]sysSpec{}
	for _, nr := range []uint64{
		unix.SYS_READ, unix.SYS_WRITE, unix.SYS_CLOSE, unix.SYS_CLOSE_RANGE, unix.SYS_FSTAT, unix.SYS_LSEEK,
		unix.SYS_MMAP, unix.SYS_MPROTECT, unix.SYS_MUNMAP, unix.SYS_BRK, unix.SYS_MREMAP, unix.SYS_MSYNC,
		unix.SYS_MINCORE, unix.SYS_MADVISE, unix.SYS_MLOCK, unix.SYS_MLOCK2, unix.SYS_MUNLOCK, unix.SYS_MLOCKALL,
		unix.SYS_MUNLOCKALL, unix.SYS_MEMBARRIER, unix.SYS_MBIND, unix.SYS_SET_MEMPOLICY, unix.SYS_GET_MEMPOLICY,
		unix.SYS_RT_SIGACTION, unix.SYS_RT_SIGPROCMASK, unix.SYS_RT_SIGRETURN, unix.SYS_RT_SIGPENDING,
		unix.SYS_RT_SIGTIMEDWAIT, unix.SYS_RT_SIGQUEUEINFO, unix.SYS_RT_SIGSUSPEND, unix.SYS_RT_TGSIGQUEUEINFO,
		unix.SYS_SIGALTSTACK, unix.SYS_RESTART_SYSCALL,
		unix.SYS_IOCTL, unix.SYS_PREAD64, unix.SYS_PWRITE64, unix.SYS_READV, unix.SYS_WRITEV, unix.SYS_PREADV,
		unix.SYS_PWRITEV, unix.SYS_PREADV2, unix.SYS_PWRITEV2, unix.SYS_SENDFILE, unix.SYS_SPLICE, unix.SYS_TEE,
		unix.SYS_VMSPLICE, unix.SYS_COPY_FILE_RANGE,
		unix.SYS_SCHED_YIELD, unix.SYS_SCHED_GETAFFINITY, unix.SYS_SCHED_SETAFFINITY, unix.SYS_SCHED_GETPARAM,
		unix.SYS_SCHED_SETPARAM, unix.SYS_SCHED_GETSCHEDULER, unix.SYS_SCHED_SETSCHEDULER, unix.SYS_SCHED_GETATTR,
		unix.SYS_SCHED_SETATTR, unix.SYS_SCHED_GET_PRIORITY_MAX, unix.SYS_SCHED_GET_PRIORITY_MIN,
		unix.SYS_SCHED_RR_GET_INTERVAL, unix.SYS_GETCPU, unix.SYS_GETPRIORITY, unix.SYS_SETPRIORITY,
		unix.SYS_IOPRIO_GET, unix.SYS_IOPRIO_SET,
		unix.SYS_DUP, unix.SYS_DUP3, unix.SYS_PIPE2, unix.SYS_FCNTL, unix.SYS_FLOCK, unix.SYS_FSYNC,
		unix.SYS_FDATASYNC, unix.SYS_SYNC_FILE_RANGE, unix.SYS_SYNCFS, unix.SYS_FTRUNCATE, unix.SYS_FALLOCATE,
		unix.SYS_FADVISE64, unix.SYS_READAHEAD, unix.SYS_FCHDIR, unix.SYS_FCHMOD, unix.SYS_FCHOWN,
		unix.SYS_FSTATFS, unix.SYS_FGETXATTR, unix.SYS_FLISTXATTR, unix.SYS_FSETXATTR, unix.SYS_FREMOVEXATTR,
		unix.SYS_GETCWD, unix.SYS_UMASK,
		unix.SYS_NANOSLEEP, unix.SYS_CLOCK_NANOSLEEP, unix.SYS_CLOCK_GETTIME, unix.SYS_CLOCK_GETRES,
		unix.SYS_GETTIMEOFDAY, unix.SYS_GETITIMER, unix.SYS_SETITIMER, unix.SYS_TIMES,
		unix.SYS_TIMER_CREATE, unix.SYS_TIMER_SETTIME, unix.SYS_TIMER_GETTIME, unix.SYS_TIMER_GETOVERRUN,
		unix.SYS_TIMER_DELETE, unix.SYS_TIMERFD_CREATE, unix.SYS_TIMERFD_SETTIME, unix.SYS_TIMERFD_GETTIME,
		unix.SYS_EVENTFD2, unix.SYS_SIGNALFD4, unix.SYS_EPOLL_CREATE1, unix.SYS_EPOLL_CTL, unix.SYS_EPOLL_PWAIT,
		unix.SYS_EPOLL_PWAIT2, unix.SYS_PPOLL, unix.SYS_PSELECT6, unix.SYS_INOTIFY_INIT1, unix.SYS_MEMFD_CREATE,
		unix.SYS_GETPID, unix.SYS_GETPPID, unix.SYS_GETTID, unix.SYS_GETUID, unix.SYS_GETEUID, unix.SYS_GETGID,
		unix.SYS_GETEGID, unix.SYS_GETRESUID, unix.SYS_GETRESGID, unix.SYS_GETGROUPS, unix.SYS_GETPGID,
		unix.SYS_SETPGID, unix.SYS_GETSID, unix.SYS_SETSID, unix.SYS_SETUID, unix.SYS_SETGID, unix.SYS_SETREUID,
		unix.SYS_SETREGID, unix.SYS_SETRESUID, unix.SYS_SETRESGID, unix.SYS_SETFSUID, unix.SYS_SETFSGID,
		unix.SYS_SETGROUPS, unix.SYS_CAPGET, unix.SYS_CAPSET, unix.SYS_PRCTL, unix.SYS_PERSONALITY,
		unix.SYS_SECCOMP,
		unix.SYS_GETRLIMIT, unix.SYS_SETRLIMIT, unix.SYS_PRLIMIT64, unix.SYS_GETRUSAGE, unix.SYS_SYSINFO,
		unix.SYS_UNAME, unix.SYS_GETRANDOM,
		unix.SYS_EXIT, unix.SYS_EXIT_GROUP, unix.SYS_WAIT4, unix.SYS_WAITID, unix.SYS_KILL, unix.SYS_TKILL,
		unix.SYS_TGKILL, unix.SYS_PIDFD_SEND_SIGNAL,
		unix.SYS_SET_TID_ADDRESS, unix.SYS_SET_ROBUST_LIST, unix.SYS_GET_ROBUST_LIST, unix.SYS_FUTEX,
		unix.SYS_FUTEX_WAITV, unix.SYS_RSEQ,
		unix.SYS_SOCKET, unix.SYS_SOCKETPAIR, unix.SYS_LISTEN, unix.SYS_ACCEPT, unix.SYS_ACCEPT4,
		unix.SYS_GETSOCKNAME, unix.SYS_GETPEERNAME, unix.SYS_SETSOCKOPT, unix.SYS_GETSOCKOPT, unix.SYS_SHUTDOWN,
		unix.SYS_RECVFROM, unix.SYS_RECVMSG, unix.SYS_RECVMMSG,
		unix.SYS_IO_SETUP, unix.SYS_IO_DESTROY, unix.SYS_IO_SUBMIT, unix.SYS_IO_CANCEL, unix.SYS_IO_GETEVENTS,
		unix.SYS_IO_PGETEVENTS,
	} {
		t[nr] = free()
	}
	t[unix.SYS_OPENAT] = at(skOpen, 0, 1, 2)
	t[unix.SYS_OPENAT2] = at(skOpenat2, 0, 1, 2)
	t[unix.SYS_NEWFSTATAT] = at(skStat, 0, 1, 3)
	t[unix.SYS_STATX] = at(skStat, 0, 1, 2)
	t[unix.SYS_FACCESSAT] = at(skStat, 0, 1, -1)
	t[unix.SYS_FACCESSAT2] = at(skStat, 0, 1, -1)
	t[unix.SYS_READLINKAT] = at(skStat, 0, 1, -1)
	t[unix.SYS_CHDIR] = at(skStat, -1, 0, -1)
	t[unix.SYS_STATFS] = at(skStat, -1, 0, -1)
	t[unix.SYS_GETXATTR] = at(skStat, -1, 0, -1)
	t[unix.SYS_LGETXATTR] = at(skStat, -1, 0, -1)
	t[unix.SYS_LISTXATTR] = at(skStat, -1, 0, -1)
	t[unix.SYS_LLISTXATTR] = at(skStat, -1, 0, -1)
	t[unix.SYS_SETXATTR] = at(skMutate, -1, 0, -1)
	t[unix.SYS_LSETXATTR] = at(skMutate, -1, 0, -1)
	t[unix.SYS_REMOVEXATTR] = at(skMutate, -1, 0, -1)
	t[unix.SYS_LREMOVEXATTR] = at(skMutate, -1, 0, -1)
	t[unix.SYS_TRUNCATE] = at(skMutate, -1, 0, -1)
	t[unix.SYS_FCHMODAT] = at(skMutate, 0, 1, -1)
	t[unix.SYS_FCHMODAT2] = at(skMutate, 0, 1, 3)
	t[unix.SYS_FCHOWNAT] = at(skMutate, 0, 1, 4)
	t[unix.SYS_UTIMENSAT] = at(skMutate, 0, 1, 3)
	t[unix.SYS_UNLINKAT] = at(skMutate, 0, 1, -1)
	t[unix.SYS_MKDIRAT] = at(skCreate, 0, 1, -1)
	t[unix.SYS_MKNODAT] = at(skCreate, 0, 1, -1)
	t[unix.SYS_LINKAT] = two(skLink, 0, 1, 2, 3)
	t[unix.SYS_RENAMEAT] = two(skRename, 0, 1, 2, 3)
	t[unix.SYS_RENAMEAT2] = two(skRename, 0, 1, 2, 3)
	t[unix.SYS_SYMLINKAT] = two(skSymlink, -1, -1, 1, 2)
	t[unix.SYS_GETDENTS64] = sysSpec{kind: skList}
	t[unix.SYS_EXECVE] = at(skExec, -1, 0, -1)
	t[unix.SYS_EXECVEAT] = at(skExec, 0, 1, 4)
	t[unix.SYS_CONNECT] = sysSpec{kind: skConnect}
	t[unix.SYS_SENDTO] = sysSpec{kind: skSendto}
	t[unix.SYS_SENDMSG] = sysSpec{kind: skSendmsg}
	t[unix.SYS_SENDMMSG] = sysSpec{kind: skSendmmsg}
	t[unix.SYS_BIND] = sysSpec{kind: skBind}
	t[unix.SYS_CLONE] = sysSpec{kind: skClone}
	t[unix.SYS_CLONE3] = sysSpec{kind: skClone3}
	t[unix.SYS_PTRACE] = sysSpec{kind: skProc, dirfd: 1, path: -1, name: "ptrace"}
	t[unix.SYS_PROCESS_VM_READV] = sysSpec{kind: skProc, dirfd: 0, path: -1, name: "process_vm_readv"}
	t[unix.SYS_PROCESS_VM_WRITEV] = sysSpec{kind: skProc, dirfd: 0, path: -1, name: "process_vm_writev"}
	t[unix.SYS_PIDFD_OPEN] = sysSpec{kind: skProc, dirfd: 0, path: -1, name: "pidfd_open"}
	t[unix.SYS_KCMP] = sysSpec{kind: skProc, dirfd: 0, path: 1, name: "kcmp"}
	t[unix.SYS_PIDFD_GETFD] = sysSpec{kind: skPidfdFd, name: "pidfd_getfd"}
	for nr, name := range map[uint64]string{
		unix.SYS_UNSHARE: "unshare", unix.SYS_SETNS: "setns", unix.SYS_MOUNT: "mount", unix.SYS_UMOUNT2: "umount2",
		unix.SYS_PIVOT_ROOT: "pivot_root", unix.SYS_CHROOT: "chroot", unix.SYS_FSOPEN: "fsopen",
		unix.SYS_FSMOUNT: "fsmount", unix.SYS_FSPICK: "fspick", unix.SYS_FSCONFIG: "fsconfig",
		unix.SYS_OPEN_TREE: "open_tree", unix.SYS_MOVE_MOUNT: "move_mount", unix.SYS_MOUNT_SETATTR: "mount_setattr",
		// A file handle opens an inode with no path the recorder can name.
		unix.SYS_OPEN_BY_HANDLE_AT: "open_by_handle_at",
	} {
		t[nr] = deny(name)
	}
	for nr, s := range sysTableArch() {
		t[nr] = s
	}
	return t
}()

// tracerArch is true: the system-call table covers this architecture.
const tracerArch = true
