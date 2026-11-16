//go:build linux

package main

// auditArchNative is AUDIT_ARCH_AARCH64: a stop from any other ABI (the
// 32-bit compat entry) is a call the table does not describe.
const auditArchNative = 0xc00000b7

// sysTableArch is the arm64-only half of sysTable: arm64 carries only the
// *at forms, so it adds nothing.
func sysTableArch() map[uint64]sysSpec { return nil }
