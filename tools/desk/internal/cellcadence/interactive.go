package cellcadence

import (
	"context"
	"errors"
	"os"
	"time"
)

// RunInteractive holds the same durable ownership boundary as a cadence pass,
// without a tick deadline, restart delay, or model-summary requirement. The
// callback must return only after native child-tree cleanup, marking Uncertain
// if cleanup cannot be proved. A crash leaves Running persisted for recovery.
func (l *Lease) RunInteractive(ctx context.Context, cell, role string, pass func(context.Context) Result) error {
	if ctx == nil || cell == "" || role == "" || pass == nil {
		return errors.New("invalid interactive owner configuration")
	}
	l.mu.Lock()
	if l.file == nil || l.running {
		l.mu.Unlock()
		return errors.New("role lease closed or already running")
	}
	l.running = true
	l.mu.Unlock()
	defer func() { l.mu.Lock(); l.running = false; l.mu.Unlock() }()
	s, err := Read(l.dir)
	if err != nil && !errors.Is(err, ErrNoState) {
		return err
	}
	if err == nil {
		if s.Cell != cell || s.Role != role {
			return errors.New("checkpoint belongs to another cell or role")
		}
		if s.Running {
			return ErrUnfinished
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	sequence := s.Sequence + 1
	if sequence == 0 {
		return errors.New("role sequence exhausted")
	}
	now := time.Now().UTC()
	s = State{Schema: Schema, Mode: "interactive", Cell: cell, Role: role, PID: os.Getpid(), NextDue: now, Heartbeat: now, LastStart: now, Running: true, Sequence: sequence, Outcome: "running"}
	if err := write(l.dir, s); err != nil {
		return err
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := make(chan Result, 1)
	go func() { finished <- pass(child) }()
	beat := time.NewTicker(5 * time.Second)
	defer beat.Stop()
	var result Result
	var recordErr error
await:
	for {
		select {
		case result = <-finished:
			break await
		case <-beat.C:
			s.Heartbeat = time.Now().UTC()
			if err := write(l.dir, s); err != nil {
				recordErr = errors.Join(recordErr, err)
				cancel()
			}
		}
	}
	s.Heartbeat = time.Now().UTC()
	s.Outcome = "could-not-check"
	if result.Uncertain {
		return errors.Join(ErrUnfinished, result.Err, recordErr, write(l.dir, s))
	}
	s.Running = false
	s.LastFinish = s.Heartbeat
	s.NextDue = s.LastFinish // switching to cadence never inherits an interactive delay
	s.ExitCode = result.ExitCode
	if result.Err == nil && result.ExitCode == 0 && ctx.Err() == nil && recordErr == nil {
		s.Outcome = "ok"
	}
	return errors.Join(result.Err, ctx.Err(), recordErr, write(l.dir, s))
}
