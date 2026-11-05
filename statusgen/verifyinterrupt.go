package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
)

// Every Verify row runs in its OWN process group (killWholeProcessGroup), so a
// stop signal sent to statusgen's group — a terminal's Ctrl-C or hangup,
// timeout(1), a supervisor stopping the job — no longer reaches the row by
// itself. statusgen therefore catches those signals for the length of a run
// and turns them into a cancellation of every row's context: the row's group
// is killed exactly as at the row deadline, the row is could-not-run, no later
// row starts, and verifyrun writes no witness.

// runInterruptSignals are the stop signals a run forwards to its rows.
// syscall defines all three on every platform; Windows delivers only
// os.Interrupt, and the others never arrive there.
var runInterruptSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}

// runInterrupted is the cancellation cause a delivered stop signal records.
type runInterrupted struct{ sig os.Signal }

func (e runInterrupted) Error() string {
	switch e.sig {
	case os.Interrupt:
		return "interrupted by SIGINT"
	case syscall.SIGTERM:
		return "interrupted by SIGTERM"
	case syscall.SIGHUP:
		return "interrupted by SIGHUP"
	}
	return "interrupted by " + e.sig.String()
}

// watchRunInterrupt catches the stop signals until release is called and
// returns the context they cancel. A run sets it as its plan's stop context
// before any row starts and releases it when the run returns, so outside a
// run the signals keep their default effect.
//
// A signal that was IGNORED when statusgen started (nohup, a script's
// background job) is left ignored: signal.Notify would re-enable it and turn a
// run its caller deliberately shielded into an interrupted one. Only the first
// caught signal is handled; the watch then stops, so a second one takes its
// default effect, even while an escaped descendant still holds the row's pipe.
func watchRunInterrupt() (ctx context.Context, release func()) {
	ctx, cancel := context.WithCancelCause(context.Background())
	ch := make(chan os.Signal, 1)
	var watched []os.Signal
	for _, s := range runInterruptSignals {
		if !signal.Ignored(s) {
			watched = append(watched, s)
		}
	}
	// Notify with no signals would relay EVERY signal, so skip it outright.
	if len(watched) > 0 {
		signal.Notify(ch, watched...)
	}
	done := make(chan struct{})
	go func() {
		select {
		case s := <-ch:
			signal.Stop(ch)
			cancel(runInterrupted{sig: s})
		case <-done:
		}
	}()
	return ctx, func() {
		signal.Stop(ch)
		close(done)
		cancel(nil)
	}
}

// rowParent is the context every row deadline of this run derives from.
func (p shellPlan) rowParent() context.Context {
	if p.stop != nil {
		return p.stop
	}
	return context.Background()
}

// interruption names the stop signal that interrupted this run, or "" when
// none has. Only a delivered signal counts: the ordinary release at the end of
// a run cancels the context too, and that is not an interruption.
func (p shellPlan) interruption() string {
	if p.stop == nil {
		return ""
	}
	var ri runInterrupted
	if errors.As(context.Cause(p.stop), &ri) {
		return ri.Error()
	}
	return ""
}
