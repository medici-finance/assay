package gitmaintenance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/cellprocess"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var errCleanupUnknown = errors.New("maintenance: process-tree cleanup could not be confirmed")

// This is the issue #2513 native-format owner, not a general Git escape hatch.
// Git's maintenance machinery handles concurrent pack readers, MIDX selection
// and native ref locks; go-git has no equivalent incremental maintenance API.
// The closed operation switch accepts no flags, refspecs, transport or command
// fragments from callers. GC, prefetch, pruning and unreachable-object expiry
// are absent; MIDX expiry removes only redundant packs whose objects are retained.
func native(ctx context.Context, common, operation string) ([]byte, error) {
	if operation == "incremental-repack" {
		// Match Git's incremental task in the same order, with an explicit capped
		// batch instead of its uncapped automatic second-largest-pack heuristic.
		for _, step := range []string{"midx-write", "midx-verify", "midx-expire", "midx-repack"} {
			if _, err := native(ctx, common, step); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	switch operation {
	case "pack-refs", "commit-graph", "loose-objects", "midx-write", "midx-expire", "midx-repack":
		if err := deskkit.Guard(); err != nil {
			return nil, err
		}
		if err := checkCommonHooks(ctx, common); err != nil {
			return nil, err
		}
	}
	var operationArgs []string
	switch operation {
	case "config":
		operationArgs = []string{"config", "--local", "--no-includes", "--null", "--list"}
	case "worktree-config":
		operationArgs = []string{"config", "--file=" + filepath.Join(common, "config.worktree"), "--no-includes", "--null", "--list"}
	case "objects":
		operationArgs = []string{"count-objects", "-v"}
	case "refs":
		operationArgs = []string{"for-each-ref", "--format=%(refname)", "--sort=refname"}
	case "worktrees":
		operationArgs = []string{"worktree", "list", "--porcelain", "-z"}
	case "pack-refs":
		operationArgs = []string{"pack-refs", "--all"}
	case "commit-graph":
		operationArgs = []string{"commit-graph", "write", "--reachable", "--no-progress"}
	case "loose-objects":
		// Manual maintenance (no --auto, --schedule or --detach) runs in the
		// foreground in Git before and after the 2.47 detach-option change.
		// autoDetach is additionally pinned false below; avoid requiring a newer
		// Git solely for its --no-detach spelling.
		operationArgs = []string{"maintenance", "run", "--task=loose-objects"}
	case "midx-write":
		operationArgs = []string{"multi-pack-index", "write", "--no-progress"}
	case "midx-expire":
		operationArgs = []string{"multi-pack-index", "expire", "--no-progress"}
	case "midx-verify":
		operationArgs = []string{"multi-pack-index", "verify", "--no-progress"}
	case "midx-repack":
		batch, err := packBatch(ctx, common)
		if err != nil {
			return nil, err
		}
		operationArgs = []string{"multi-pack-index", "repack", "--no-progress", "--batch-size=" + strconv.FormatInt(batch, 10)}
	default:
		return nil, fmt.Errorf("maintenance: unaudited native operation %q", operation)
	}
	args := []string{"--no-pager", "--no-replace-objects", "--git-dir=" + common}
	// Command-line config outranks every file. Task selection is explicit; neither
	// local maintenance.strategy nor .enabled values can add a network task.
	for _, value := range []string{
		"core.fsmonitor=false", "core.alternateRefsCommand=",
		"gc.auto=0", "gc.autoDetach=false", "maintenance.auto=false", "maintenance.autoDetach=false",
		"maintenance.loose-objects.batchSize=1000",
		"pack.threads=1", "pack.window=10", "pack.depth=10", "pack.windowMemory=64m", "pack.deltaCacheSize=32m",
		"pack.indexVersion=2", "pack.writeReverseIndex=false", "repack.writeBitmaps=false",
		"core.multiPackIndex=true", "core.commitGraph=true", "commitGraph.maxNewFilters=0",
		"credential.helper=", "core.askPass=", "protocol.allow=never", "color.ui=false",
	} {
		args = append(args, "-c", value)
	}
	args = append(args, operationArgs...)
	var stdout, stderr boundedBuffer
	_, uncertain, err := cellprocess.Run(ctx, append([]string{"git"}, args...), maintenanceEnv(os.Environ()), filepath.Dir(common), &stdout, &stderr)
	if uncertain {
		err = errors.Join(err, errCleanupUnknown)
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("maintenance: %s exceeded its time budget: %w", operation, errors.Join(ctx.Err(), err))
		}
		return nil, fmt.Errorf("maintenance: %s: %w: %s", operation, err, strings.TrimSpace(stderr.String()))
	}
	if stderr.Len() != 0 {
		return nil, fmt.Errorf("maintenance: %s emitted uncertainty: %s", operation, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// The threshold is positive and capped at 64 MiB. Small repositories still
// consolidate at least two small packs: a fixed 64 MiB threshold would never
// compact a repository whose total packed data is smaller than that threshold.
// Git's batch threshold is a selection heuristic, not a hard output byte limit;
// the coordinator's object-store, memory and process-time bounds also apply.
func packBatch(ctx context.Context, common string) (int64, error) {
	entries, err := readPackEntries(ctx, common)
	if err != nil {
		return 0, err
	}
	var largest, second int64
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if !strings.HasSuffix(entry.Name(), ".pack") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return 0, err
		}
		n := info.Size()
		if n > largest {
			second = largest
			largest = n
		} else if n > second {
			second = n
		}
	}
	batch := second + 1
	if batch > 64<<20 {
		batch = 64 << 20
	}
	return batch, nil
}

// Preserve only executable lookup, platform runtime and temporary-directory
// necessities. HOME/global configs and every parent GIT_* override are dropped.
// Protocol denial and no-lazy-fetch are separate backstops, not an allowlist of
// network helpers whose future expansion could silently permit a fetch.
func maintenanceEnv(parent []string) []string {
	var out []string
	for _, pair := range parent {
		key, _, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		switch strings.ToUpper(key) {
		case "PATH", "SYSTEMROOT", "WINDIR", "TMPDIR", "TMP", "TEMP":
			out = append(out, pair)
		}
	}
	return append(out, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1",
		"GIT_ALLOW_PROTOCOL=", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
}

// Inventories and error output are bounded independently of Git's work budget.
type boundedBuffer struct{ bytes.Buffer }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 4<<20 {
		return 0, errors.New("maintenance: native output exceeds inventory budget")
	}
	return b.Buffer.Write(p)
}
