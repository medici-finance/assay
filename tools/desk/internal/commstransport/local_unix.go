//go:build !windows

// Package commstransport owns the host-local gateway transport.
package commstransport

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

func Validate(address string) error {
	if !filepath.IsAbs(address) {
		return fmt.Errorf("comms socket must be an absolute path")
	}
	return nil
}

func Listen(address string) (net.Listener, error) {
	if err := Validate(address); err != nil {
		return nil, err
	}
	// The parent is the access boundary before chmod can protect the socket.
	fi, err := os.Lstat(filepath.Dir(address))
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() || fi.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("comms socket parent must be an owner-only directory")
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

func Dial(address string, timeout time.Duration) (net.Conn, error) {
	if err := Validate(address); err != nil {
		return nil, err
	}
	return net.DialTimeout("unix", address, timeout)
}
