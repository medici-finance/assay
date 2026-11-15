// This probe qualifies only local image/volume/socket primitives. It implements
// no caller authentication, signing, current authority or production profile.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

const (
	privateFile = "/probe/private/marker"
	socketFile  = "/probe/channel/current.sock"
)

func report(v any) {
	if json.NewEncoder(os.Stdout).Encode(v) != nil {
		os.Exit(1)
	}
}
func fail(stage string) {
	report(map[string]any{"ok": false, "stage": stage, "primitive_only": true})
	os.Exit(1)
}
func main() {
	if runtime.GOOS != "linux" || os.Getuid() != 501 || len(os.Args) != 2 {
		fail("profile")
	}
	switch os.Args[1] {
	case "serve":
		serve()
	case "client":
		client()
	case "sibling":
		sibling()
	default:
		fail("mode")
	}
}
func serve() {
	if os.WriteFile(privateFile, []byte("fixture"), 0600) != nil {
		fail("private-write")
	}
	b, err := os.ReadFile(privateFile)
	if err != nil || string(b) != "fixture" {
		fail("private-read")
	}
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketFile, Net: "unix"})
	if err != nil {
		fail("listen")
	}
	defer l.Close()
	if os.Chmod(socketFile, 0600) != nil {
		fail("socket-mode")
	}
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGTERM, os.Interrupt)
	defer signal.Stop(done)
	go func() { <-done; l.Close() }()
	report(map[string]any{"ok": true, "stage": "ready", "uid": os.Getuid(), "os": runtime.GOOS, "arch": runtime.GOARCH, "private_positive": true, "primitive_only": true})
	if l.SetDeadline(time.Now().Add(45*time.Second)) != nil {
		fail("deadline")
	}
	for {
		c, err := l.AcceptUnix()
		if err != nil {
			return
		}
		if c.SetDeadline(time.Now().Add(5*time.Second)) != nil {
			c.Close()
			fail("peer-deadline")
		}
		request := make([]byte, 5)
		_, e := io.ReadFull(c, request)
		if e == nil && string(request) == "probe" {
			_, e = c.Write([]byte("ready"))
		}
		c.Close()
		if e != nil {
			fail("peer")
		}
	}
}
func client() {
	if _, err := os.ReadFile(privateFile); !os.IsNotExist(err) {
		fail("private-excluded")
	}
	if err := os.WriteFile("/probe/channel/consumer-write", []byte("fixture"), 0600); !errors.Is(err, syscall.EROFS) {
		fail("readonly-volume")
	}
	c, err := net.DialTimeout("unix", socketFile, 5*time.Second)
	if err != nil {
		fail("connect")
	}
	defer c.Close()
	if c.SetDeadline(time.Now().Add(5*time.Second)) != nil {
		fail("deadline")
	}
	if _, err := c.Write([]byte("probe")); err != nil {
		fail("send")
	}
	response := make([]byte, 5)
	if _, err := io.ReadFull(c, response); err != nil || string(response) != "ready" {
		fail("reply")
	}
	report(map[string]any{"ok": true, "stage": "client", "uid": os.Getuid(), "private_excluded": true, "volume_write_denied": true, "socket_roundtrip": true, "primitive_only": true})
}
func sibling() {
	if _, err := os.ReadFile(privateFile); !os.IsNotExist(err) {
		fail("private-excluded")
	}
	c, err := net.DialTimeout("unix", socketFile, 2*time.Second)
	if err == nil {
		c.Close()
		fail("channel-excluded")
	}
	report(map[string]any{"ok": true, "stage": "sibling", "uid": os.Getuid(), "private_excluded": true, "channel_excluded": true, "primitive_only": true})
	fmt.Fprint(os.Stderr, "profile primitive probe only; no authority qualification\n")
}
