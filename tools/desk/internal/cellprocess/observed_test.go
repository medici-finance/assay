package cellprocess

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestObservedLaunchFailure(t *testing.T) {
	argv, env := fixture(t, "sleep")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	called := false
	enrollment := errors.New("cannot persist child identity")
	code, uncertain, err := RunObserved(ctx, argv, env, t.TempDir(), io.Discard, io.Discard, func(pid int) error { called = pid > 0; return enrollment })
	if !called || code == 0 || uncertain || !errors.Is(err, enrollment) {
		t.Fatalf("unrecorded child not stopped: called=%t code=%d uncertain=%t err=%v", called, code, uncertain, err)
	}
}
