//go:build !windows

package main

import "errors"

// createJunction exists only on windows; elsewhere a symlink never lacks a privilege, so the
// fallback that calls this is unreachable outside tests that inject their own primitive.
func createJunction(target, link string) error {
	return errors.New("directory junctions exist only on windows")
}
