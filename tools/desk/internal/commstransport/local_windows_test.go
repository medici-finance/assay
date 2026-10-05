package commstransport

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestDialRefusesForeignPipeOwner(t *testing.T) {
	address := fmt.Sprintf(`\\.\pipe\assay-comms-owner-%d-%d`, os.Getpid(), time.Now().UnixNano())
	ln, err := Listen(address)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	c, err := Dial(address, time.Second)
	if err != nil {
		t.Fatalf("positive control: own pipe refused: %v", err)
	}
	c.Close()
	// Expecting a different account must refuse this server: the comparison,
	// not the dial, is what authenticates the endpoint.
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		t.Fatal(err)
	}
	if c, err := dialOwnedBy(address, time.Second, system); err == nil {
		c.Close()
		t.Fatal("dial accepted a pipe whose owner is not the expected account")
	}
}

func TestPipeSecurityNamesOwner(t *testing.T) {
	user, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	if got := pipeSecurity(user); !strings.HasPrefix(got, "O:"+user.String()+"D:P(") {
		t.Fatalf("pipe security does not name the current user as owner: %s", got)
	}
}
