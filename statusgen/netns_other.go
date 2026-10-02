//go:build !linux

package main

import "runtime"

// runNetnsHelper off Linux: there is no `unshare --net` namespace to be inside
// of, so the helper can only refuse. networkOffWrapper never builds the sandbox
// off Linux; this exists so a stray invocation fails closed, not open.
func runNetnsHelper(args []string) int {
	return netnsRefuse("the network-off sandbox is Linux-only and this host is %s", runtime.GOOS)
}
