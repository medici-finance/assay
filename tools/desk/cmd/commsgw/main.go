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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return runContext(ctx, getenv)
}

// runContext serves until ctx ends or a listener fails. On every exit path it
// waits for the socket server to close its listener, so a clean stop leaves no
// socket behind to block the next start.
func runContext(ctx context.Context, getenv func(string) string) int {
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

	repo := cfg.Repo
	if repo == "" {
		repo = "medici-finance/assay"
	} // retain the existing standalone default
	filer := DeskfileIssueFiler{Repo: repo}

	// The outbound prose gate is consulted on every send (socket.go). Its
	// contained advisor is wired from the pinned decider runner entry (brief
	// 06) when one is configured; a configured-but-unsafe entry refuses to boot
	// here (containment never silently degrades), and an unconfigured one leaves
	// the valve off so every send holds (fail closed).
	gate, err := NewGate(getenv, cfg, filer)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitCodeOf(err)
	}

	agent := GatewayAgent{Root: cfg.QueueDir, Cell: cfg.Cell, Deps: deps, Emitter: NoOpInboxEmitter{}, Filer: filer}
	sock := SocketServer{Root: cfg.QueueDir, Cell: cfg.Cell, Deps: deps, Emitter: agent.Emitter, Filer: filer, Gate: gate, LocalOnly: cfg.LocalOnly}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sockCh := make(chan error, 1)
	a2aCh := make(chan error, 1)
	go func() { sockCh <- sock.ListenAndServeContext(ctx, cfg.Socket) }()
	if !cfg.LocalOnly {
		go func() { a2aCh <- ListenAndServeA2A(ctx, cfg, agent) }()
	}

	var failure error
	select {
	case <-ctx.Done():
		<-sockCh
		return 0
	case failure = <-sockCh:
		cancel()
	case failure = <-a2aCh:
		cancel()
		<-sockCh
	}
	if failure != nil && !errors.Is(failure, context.Canceled) {
		fmt.Fprintln(os.Stderr, failure)
		return 1
	}
	return 0
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
