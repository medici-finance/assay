//go:build !linux

package main

import "runtime"

// rowReadSetToken is an executed row's read-set token off Linux: always the
// could-not-check token, never a read set.
func rowReadSetToken() string { return readSetOffLinuxToken(runtime.GOOS) }
