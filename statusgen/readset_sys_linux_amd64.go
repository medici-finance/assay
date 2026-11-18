//go:build linux

package main

import "golang.org/x/sys/unix"

// auditArchNative is AUDIT_ARCH_X86_64: a stop from any other ABI (the i386
// compat entry, x32) is a call the table does not describe.
const auditArchNative = 0xc000003e

// sysTableArch is the amd64-only half of sysTable: the legacy path calls
// arm64 never had.
func sysTableArch() map[uint64]sysSpec {
	t := map[uint64]sysSpec{}
	for _, nr := range []uint64{
		unix.SYS_ARCH_PRCTL, unix.SYS_PIPE, unix.SYS_DUP2, unix.SYS_POLL, unix.SYS_SELECT, unix.SYS_EPOLL_WAIT,
		unix.SYS_EPOLL_CREATE, unix.SYS_ALARM, unix.SYS_PAUSE, unix.SYS_TIME, unix.SYS_GETPGRP,
		unix.SYS_INOTIFY_INIT, unix.SYS_EVENTFD, unix.SYS_SIGNALFD, unix.SYS_FORK, unix.SYS_VFORK,
	} {
		t[nr] = free()
	}
	t[unix.SYS_GETDENTS] = sysSpec{kind: skList}
	t[unix.SYS_OPEN] = at(skOpen, -1, 0, 1)
	t[unix.SYS_CREAT] = sysSpec{kind: skOpen, dirfd: -1, path: 0, flags: -2, dirfd2: -1, path2: -1}
	t[unix.SYS_STAT] = at(skStat, -1, 0, -1)
	t[unix.SYS_LSTAT] = at(skStat, -1, 0, -1)
	t[unix.SYS_ACCESS] = at(skStat, -1, 0, -1)
	t[unix.SYS_READLINK] = at(skStat, -1, 0, -1)
	t[unix.SYS_CHMOD] = at(skMutate, -1, 0, -1)
	t[unix.SYS_CHOWN] = at(skMutate, -1, 0, -1)
	t[unix.SYS_LCHOWN] = at(skMutate, -1, 0, -1)
	t[unix.SYS_UTIME] = at(skMutate, -1, 0, -1)
	t[unix.SYS_UTIMES] = at(skMutate, -1, 0, -1)
	t[unix.SYS_FUTIMESAT] = at(skMutate, 0, 1, -1)
	t[unix.SYS_UNLINK] = at(skMutate, -1, 0, -1)
	t[unix.SYS_RMDIR] = at(skMutate, -1, 0, -1)
	t[unix.SYS_MKDIR] = at(skCreate, -1, 0, -1)
	t[unix.SYS_MKNOD] = at(skCreate, -1, 0, -1)
	t[unix.SYS_LINK] = two(skLink, -1, 0, -1, 1)
	t[unix.SYS_RENAME] = two(skRename, -1, 0, -1, 1)
	t[unix.SYS_SYMLINK] = two(skSymlink, -1, -1, -1, 1)
	return t
}
