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

func Listen(address string) (net.Listener, error) {
	if err := Validate(address); err != nil {
		return nil, err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	// go-winio rejects remote clients. The protected DACL admits only the
	// current account, matching the Unix owner-only socket boundary.
	return winio.ListenPipe(address, &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;" + user.User.Sid.String() + ")"})
}

func Dial(address string, timeout time.Duration) (net.Conn, error) {
	if err := Validate(address); err != nil {
		return nil, err
	}
	return winio.DialPipe(address, &timeout)
}
