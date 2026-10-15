//go:build unix

package cellcadence

import "syscall"

const probeOpenFlags = syscall.O_NOFOLLOW | syscall.O_NONBLOCK
