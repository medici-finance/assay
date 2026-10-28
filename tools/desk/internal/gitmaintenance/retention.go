package gitmaintenance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/cellprocess"
)

// This read-only operation reports ordinary Git's effective recovery settings,
// including global/system/includes and the caller's worktree scope. Maintenance
// tasks keep their separate scrubbed environment and never expire any reflog.
func effectiveReflogRecovery(ctx context.Context, repo string) (map[string]string, error) {
	retention := map[string]string{
		"gc.reflogexpire":            "90 days (Git default)",
		"gc.reflogexpireunreachable": "30 days (Git default)",
	}
	argv := []string{"git", "-c", "protocol.allow=never", "-c", "gc.auto=0", "-c", "maintenance.auto=false",
		"config", "--null", "--get-regexp", `^gc(\..*)?\.reflogexpire(unreachable)?$`}
	env := append(refCASEnv(os.Environ()), "GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	var stdout, stderr boundedBuffer
	code, uncertain, err := cellprocess.Run(ctx, argv, env, repo, &stdout, &stderr)
	if uncertain {
		err = errors.Join(err, errCleanupUnknown)
	}
	// Git config's sole expected nonzero result is no matching key. Diagnostics
	// or uncertain child custody can never be mistaken for configured defaults.
	if code == 1 && stdout.Len() == 0 && stderr.Len() == 0 && !uncertain {
		return retention, nil
	}
	if err != nil {
		return nil, fmt.Errorf("maintenance: effective reflog recovery inventory: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	if stderr.Len() != 0 {
		return nil, fmt.Errorf("maintenance: effective reflog recovery inventory emitted uncertainty: %s", strings.TrimSpace(stderr.String()))
	}
	data := stdout.String()
	if data != "" && !strings.HasSuffix(data, "\x00") {
		return nil, errors.New("maintenance: incomplete effective reflog recovery inventory")
	}
	for _, entry := range strings.Split(data, "\x00") {
		if entry == "" {
			continue
		}
		key, value, _ := strings.Cut(entry, "\n")
		last := strings.LastIndexByte(key, '.')
		if last < 2 || !strings.EqualFold(key[:3], "gc.") {
			return nil, errors.New("maintenance: unexpected effective reflog recovery key")
		}
		field := strings.ToLower(key[last+1:])
		if field != "reflogexpire" && field != "reflogexpireunreachable" {
			return nil, errors.New("maintenance: unexpected effective reflog recovery field")
		}
		// Section and variable names are insensitive; subsection ref patterns
		// retain their case. Git emits configuration scopes in precedence order.
		key = "gc" + key[2:last+1] + field
		retention[key] = value
	}
	return retention, nil
}
