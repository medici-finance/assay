// Command commsgw is the per-cell message GATEWAY: the one chokepoint every
// inbound cell message crosses, cross-cell (A2A +
// mTLS, a2a.go) and within-cell (a loopback Unix socket, socket.go) alike, run
// through the same deterministic pre-check pipeline (precheck.go) before
// anything is queued for commsloop (the paired drain consumer, ../commsloop)
// to land.
//
// CONFIG-OFF DEFAULT. Every ASSAY_COMMS_* key in config.go is required; any
// one absent refuses to serve — see LoadConfig.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func main() {
	os.Exit(run(os.Getenv))
}

// run is main's testable body: it never calls os.Exit itself, so a test can
// assert on the returned code directly.
func run(getenv func(string) string) int {
	cfg, err := LoadConfig(getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitCodeOf(err)
	}

	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitCodeOf(err)
	}

	deps, err := NewPreCheckDeps(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitCodeOf(err)
	}

	filer := DeskfileIssueFiler{Repo: "medici-finance/assay"}
	agent := GatewayAgent{Root: cfg.QueueDir, Deps: deps, Emitter: NoOpInboxEmitter{}, Filer: filer}
	sock := SocketServer{Root: cfg.QueueDir, Deps: deps, Emitter: agent.Emitter, Filer: filer}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 2)
	go func() { errCh <- sock.ListenAndServe(cfg.Socket) }()
	go func() { errCh <- ListenAndServeA2A(ctx, cfg, agent) }()

	select {
	case <-ctx.Done():
		return 0
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
}

// exitCodeOf maps a deskkit typed error to its canonical exit code (0 ok, 3
// disabled, 5 refused, 6 unverifiable — exitcodes.go); any other error is a
// generic failure (1), never silently 0.
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var de *deskkit.DeskError
	if errors.As(err, &de) {
		return de.ExitCode()
	}
	return 1
}
