package commstransport

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func Validate(address string) error {
	const prefix = `\\.\pipe\`
	if !strings.HasPrefix(address, prefix) || len(address) == len(prefix) || strings.ContainsAny(strings.TrimPrefix(address, prefix), `/\`) {
		return fmt.Errorf("comms endpoint must be a local named pipe (\\\\.\\pipe\\name)")
	}
	return nil
}

func currentUserSID() (*windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return user.User.Sid, nil
}

// pipeSecurity names the current user as both the owner and the only account
// in a protected DACL. The explicit owner is what Dial verifies; without it an
// elevated token would default the owner to the Administrators group.
func pipeSecurity(user *windows.SID) string {
	sid := user.String()
	return "O:" + sid + "D:P(A;;GA;;;" + sid + ")"
}

func Listen(address string) (net.Listener, error) {
	if err := Validate(address); err != nil {
		return nil, err
	}
	user, err := currentUserSID()
	if err != nil {
		return nil, err
	}
	// go-winio rejects remote clients. The protected DACL admits only the
	// current account, matching the Unix owner-only socket boundary.
	return winio.ListenPipe(address, &winio.PipeConfig{SecurityDescriptor: pipeSecurity(user)})
}

// RemoveStale is a no-op on Windows: a named pipe disappears with the last
// server handle, so no file is left behind by a gateway that exited.
func RemoveStale(address string) error {
	return Validate(address)
}

// Dial connects to the named pipe and then proves the server end belongs to
// the current user before any byte is written. Pipe names are host-global, so
// the server-side DACL alone does not stop another account from creating the
// name first.
func Dial(address string, timeout time.Duration) (net.Conn, error) {
	user, err := currentUserSID()
	if err != nil {
		return nil, err
	}
	return dialOwnedBy(address, timeout, user)
}

func dialOwnedBy(address string, timeout time.Duration, want *windows.SID) (net.Conn, error) {
	if err := Validate(address); err != nil {
		return nil, err
	}
	conn, err := winio.DialPipe(address, &timeout)
	if err != nil {
		return nil, err
	}
	if err := verifyPipeOwner(conn, want); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func verifyPipeOwner(conn net.Conn, want *windows.SID) error {
	f, ok := conn.(interface{ Fd() uintptr })
	if !ok {
		return fmt.Errorf("comms endpoint handle unavailable; refusing an unverified pipe")
	}
	sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_KERNEL_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("comms endpoint owner unreadable: %w", err)
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !owner.Equals(want) {
		return fmt.Errorf("comms endpoint is not owned by the current user")
	}
	return nil
}
