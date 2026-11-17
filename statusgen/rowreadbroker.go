package main

// rowreadbroker.go — how an admitted Verify row reads forge state with the
// run's own read role, not one of its own.
//
// An admitted row carries no HOME, DESK_LOOP, GH_* or ASSAY_* variable of the
// invoking process (admittedRowEnv), so the `deskread` verb behind
// `statusgen rowread` could not resolve a role or find its key from inside the
// row. The run answers the row instead: runVerifyrun listens on a unix socket
// in the run's own scratch and names it to the rows in rowreadSocketVar. A
// `statusgen rowread` that finds the variable forwards its arguments over the
// socket, and the run executes the same verb (runRowreadWith: the same closed
// set of read kinds, the same target validation, the same fail-closed answer)
// in its own process, where deskread starts with the invoking environment
// less the forge token variables. The row receives only the verb's exit
// code, stdout and stderr. The socket accepts nothing but rowread arguments:
// it is the read-only lane, never a general credential.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// rowreadSocketVar names the run's row-read socket in an admitted row's
// environment.
const rowreadSocketVar = "STATUSGEN_ROWREAD_SOCKET"

// rowreadBrokerLimit bounds one request's bytes and rowreadBrokerWait one
// exchange, so a row cannot hold the run's listener open.
const (
	rowreadBrokerLimit = 64 << 10
	rowreadBrokerWait  = 3 * time.Minute
)

type rowreadRequest struct {
	Args []string `json:"args"`
}

type rowreadReply struct {
	Code   int    `json:"code"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// rowreadSocketPath is where the run's socket goes: in the run's scratch, or,
// when that path is longer than a unix socket address allows, in a short
// directory of its own (made here; the caller removes it).
func rowreadSocketPath(scratch string) (string, string, error) {
	if sock := filepath.Join(scratch, "r.sock"); len(sock) < 100 {
		return sock, "", nil
	}
	dir, err := os.MkdirTemp("/tmp", "vrr-")
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, "r.sock"), dir, nil
}

// serveRowread answers row-read requests at sock with readerEnv as the
// reader's environment until the returned stop is called.
func serveRowread(sock string, readerEnv []string) (func(), error) {
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(sock, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	reader := newDeskreadReader()
	reader.env = readerEnv
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				continue
			}
			go rowreadAnswer(conn, reader)
		}
	}()
	return func() { ln.Close(); <-done }, nil
}

// rowreadAnswer runs one forwarded request through the verb and replies.
func rowreadAnswer(conn net.Conn, reader deskreadReader) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(rowreadBrokerWait))
	var req rowreadRequest
	var reply rowreadReply
	if err := json.NewDecoder(io.LimitReader(conn, rowreadBrokerLimit)).Decode(&req); err != nil {
		reply = rowreadReply{Code: rowreadExitUsage, Stderr: "statusgen rowread: the run refused an unreadable request\n"}
	} else {
		var out, errb bytes.Buffer
		reply.Code = runRowreadWith(req.Args, reader, &out, &errb)
		reply.Stdout, reply.Stderr = out.String(), errb.String()
	}
	json.NewEncoder(conn).Encode(reply)
}

// rowreadForward sends a row's rowread arguments to the run's socket and
// relays the answer. A run that cannot be reached is a could-not-check, never
// an empty answer.
func rowreadForward(sock string, args []string, stdout, stderr io.Writer) int {
	conn, err := net.DialTimeout("unix", sock, 10*time.Second)
	if err != nil {
		fmt.Fprintf(stderr, "statusgen rowread: could-not-check: the run's row-read socket is unreachable: %v\n", err)
		return rowreadExitCouldNot
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(rowreadBrokerWait))
	if args == nil {
		args = []string{}
	}
	if err := json.NewEncoder(conn).Encode(rowreadRequest{Args: args}); err != nil {
		fmt.Fprintf(stderr, "statusgen rowread: could-not-check: sending to the run: %v\n", err)
		return rowreadExitCouldNot
	}
	var reply rowreadReply
	if err := json.NewDecoder(conn).Decode(&reply); err != nil {
		fmt.Fprintf(stderr, "statusgen rowread: could-not-check: no answer from the run: %v\n", err)
		return rowreadExitCouldNot
	}
	io.WriteString(stdout, reply.Stdout)
	io.WriteString(stderr, reply.Stderr)
	return reply.Code
}

// rowreadReaderEnv is the reader's environment for a brokered read: the
// invoking environment, which holds the run's read role, less the forge token
// variables and a cell's stored login, so the reader authenticates only with
// the credential it mints.
func rowreadReaderEnv(environ []string) []string {
	var env []string
	for _, kv := range environ {
		key, _, _ := strings.Cut(kv, "=")
		if admittedRowForgeToken(key) || strings.EqualFold(key, "CELLCTL_GH_AMBIENT") {
			continue
		}
		env = append(env, kv)
	}
	return env
}
