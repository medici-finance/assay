package gitmaintenance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/medici-finance/assay/tools/desk/internal/cellprocess"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ErrRefDiagnostic preserves native stderr as a could-not-check result, even if
// the child exited successfully and a later ref read could infer its effect.
var ErrRefDiagnostic = errors.New("local ref transaction emitted diagnostic uncertainty")

var refProcess = cellprocess.Run

// deleteTempRef is a fixed offline reference transaction, the #2513 exception for
// go-git's missing compare-and-delete API. Unlike Run, it takes no verb or argv.
// Effective hook configuration is preserved. Maintenance callers must check the
// shared hook policy before this primitive; it never bypasses an existing hook.
// Ambient config injection is stripped and no transport operation is exposed.
func deleteTempRef(ctx context.Context, dir, name, expected string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ref := plumbing.ReferenceName(name)
	if !strings.HasPrefix(name, "refs/deskmerge/tmp/") || ref.Validate() != nil {
		return fmt.Errorf("CAS deletion requires an owned temporary reference")
	}
	h := plumbing.NewHash(expected)
	if h.IsZero() || h.String() != expected {
		return fmt.Errorf("CAS deletion requires a full expected object id")
	}
	if err := deskkit.Guard(); err != nil {
		return err
	}
	if err := CheckHooks(ctx, dir); err != nil {
		return err
	}
	argv := []string{"git", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "-c", "protocol.allow=never", "update-ref", "--no-deref", "-d", name, expected}
	env := append(refCASEnv(os.Environ()), "GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=")
	var stdout, stderr refOutput
	_, uncertain, err := refProcess(ctx, argv, env, dir, &stdout, &stderr)
	if uncertain {
		err = errors.Join(err, errCleanupUnknown)
	}
	if stderr.Len() != 0 {
		err = errors.Join(err, ErrRefDiagnostic)
	}
	if err != nil {
		return fmt.Errorf("temporary ref compare-and-delete: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

type refOutput struct{ bytes.Buffer }

func (b *refOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 64<<10 {
		return 0, errors.New("local ref transaction output exceeds budget")
	}
	return b.Buffer.Write(p)
}

// Preserve effective Git config scopes and platform runtime while rejecting
// parent command/config injection. No GIT_* input or transport selector survives.
func refCASEnv(parent []string) []string {
	var out []string
	for _, pair := range parent {
		key, _, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		switch strings.ToUpper(key) {
		case "PATH", "HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "SYSTEMROOT", "WINDIR", "XDG_CONFIG_HOME", "APPDATA", "USER", "LOGNAME", "SHELL", "TMPDIR", "TMP", "TEMP", "LANG", "LANGUAGE", "TZ":
			out = append(out, pair)
		}
	}
	return append(out, "GIT_TERMINAL_PROMPT=0")
}
