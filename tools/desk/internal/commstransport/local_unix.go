//go:build !windows

// Package commstransport owns the host-local gateway transport.
package commstransport

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func Validate(address string) error {
	if !filepath.IsAbs(address) {
		return fmt.Errorf("comms socket must be an absolute path")
	}
	return nil
}

// ownerOnlyParent is the access boundary on both ends of the socket: the
// parent must be a real directory, owned by this user, with no group or other
// bits. Nobody else can then bind, plant or replace the endpoint inside it.
func ownerOnlyParent(address string) error {
	dir := filepath.Dir(address)
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() || fi.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("comms socket parent must be an owner-only directory")
	}
	return ownedByCurrentUser(dir, fi)
}

func Listen(address string) (net.Listener, error) {
	if err := Validate(address); err != nil {
		return nil, err
	}
	// The parent is the access boundary before chmod can protect the socket.
	if err := ownerOnlyParent(address); err != nil {
		return nil, err
	}
	// Never unlink an existing listener. An occupied or stale socket needs
	// operator reconciliation; unlinking a live socket permits two gateways.
	ln, err := net.Listen("unix", address)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(address, 0600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

// RemoveStale removes a socket file that no gateway is serving. It refuses a
// path that is not a socket or not owned by this user, and a socket that still
// accepts a connection. Only a refused connection counts as stale; any other
// dial outcome leaves the file in place.
func RemoveStale(address string) error {
	if err := Validate(address); err != nil {
		return err
	}
	if err := ownerOnlyParent(address); err != nil {
		return err
	}
	fi, err := os.Lstat(address)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("comms endpoint exists and is not a socket")
	}
	if err := ownedByCurrentUser(address, fi); err != nil {
		return err
	}
	c, err := net.DialTimeout("unix", address, time.Second)
	if err == nil {
		c.Close()
		return fmt.Errorf("comms endpoint is held by a live gateway")
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("comms endpoint state could not be established: %w", err)
	}
	return os.Remove(address)
}

// Dial connects only to a socket this user owns, inside an owner-only
// directory this user owns: the client half of the boundary Listen sets.
func Dial(address string, timeout time.Duration) (net.Conn, error) {
	if err := Validate(address); err != nil {
		return nil, err
	}
	if err := ownerOnlyParent(address); err != nil {
		return nil, err
	}
	fi, err := os.Lstat(address)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return nil, fmt.Errorf("comms endpoint is not a socket")
	}
	if err := ownedByCurrentUser(address, fi); err != nil {
		return nil, err
	}
	return net.DialTimeout("unix", address, timeout)
}
