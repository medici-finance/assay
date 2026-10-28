package container

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// fakeDocker puts a `docker` on PATH that prints msg to stderr and exits 1.
// No real container engine is touched.
func fakeDocker(t *testing.T, msg string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\necho '" + msg + "' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// TestContainerPermissionDeniedIsUnauthorized (the service contract Task 3): a
// `permission denied` from the engine's socket is ErrUnauthorized (retried
// slowly), never ErrUnreachable. A missing container stays ErrUnreachable.
func TestContainerPermissionDeniedIsUnauthorized(t *testing.T) {
	spec := &terminal.TargetSpec{Kind: "container", Container: "c-review"}

	fakeDocker(t, "permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock")
	_, err := NewDriver().Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if !errors.Is(err, terminal.ErrUnauthorized) || errors.Is(err, terminal.ErrUnreachable) {
		t.Fatalf("socket permission refusal: err = %v, want ErrUnauthorized (and not ErrUnreachable)", err)
	}

	fakeDocker(t, "Error: No such object: c-review")
	_, err = NewDriver().Attach(context.Background(), spec, terminal.Size{Cols: 80, Rows: 24})
	if !errors.Is(err, terminal.ErrUnreachable) || errors.Is(err, terminal.ErrUnauthorized) {
		t.Fatalf("missing container: err = %v, want ErrUnreachable (and not ErrUnauthorized)", err)
	}
}
