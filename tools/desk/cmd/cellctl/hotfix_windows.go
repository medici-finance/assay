//go:build windows

package main

import "syscall"

// hotfixDropAttr: the hotfix overlay is not supported on Windows (cmdHotfix refuses first).
func hotfixDropAttr(hotfixInvoker) *syscall.SysProcAttr { return nil }
