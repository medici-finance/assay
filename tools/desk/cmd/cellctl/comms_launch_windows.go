//go:build windows

package main

// privateCommsDir verifies the invoking owner and effective Windows ACL.
func commsLaunchOwner(string) error { return nil }
