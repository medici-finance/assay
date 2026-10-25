//go:build unix

package main

import "syscall"

// hotfixDropAttr runs a hotfix clone or build as the sudo-invoking user when cellctl runs as
// root, so no source code is ever compiled or fetched with root privileges.
func hotfixDropAttr(inv hotfixInvoker) *syscall.SysProcAttr {
	if !inv.Drop {
		return nil
	}
	return &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: inv.UID, Gid: inv.GID, Groups: []uint32{}}}
}
