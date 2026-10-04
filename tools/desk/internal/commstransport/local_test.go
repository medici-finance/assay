package commstransport

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLocalTransportRoundTripAndExclusiveListen(t *testing.T) {
	dir, err := os.MkdirTemp("", "comms-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	address := filepath.Join(dir, "gw.sock")
	if runtime.GOOS == "windows" {
		address = fmt.Sprintf(`\\.\pipe\assay-comms-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	}
	ln, err := Listen(address)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if other, err := Listen(address); err == nil {
		other.Close()
		t.Fatal("second gateway took the same endpoint")
	}
	done := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err == nil {
			defer c.Close()
			_, err = io.CopyN(c, c, 4)
		}
		done <- err
	}()
	c, err := Dial(address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second))
	if _, err = c.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err = io.ReadFull(c, buf); err != nil || string(buf) != "ping" {
		t.Fatalf("read %q: %v", buf, err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRemoteEndpointRefused(t *testing.T) {
	for _, address := range []string{"", "relative.sock", "tcp://127.0.0.1:1234", `\\remote\pipe\comms`} {
		if err := Validate(address); err == nil {
			t.Fatalf("accepted nonlocal endpoint %q", address)
		}
	}
}
