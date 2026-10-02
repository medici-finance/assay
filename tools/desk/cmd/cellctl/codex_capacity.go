package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Codex counts spawned threads, excluding the primary thread. A roster pool of
// eight workers therefore requires eight, not nine. The override is per launch;
// it does not change the user's global Codex configuration or enable a disabled
// multi_agent feature. Resolve again for each bounded cadence pass.
// https://learn.chatgpt.com/docs/config-file/config-reference
func (c *Cell) codexCapacityArgs(role string) ([]string, error) {
	width, _, err := deskkit.ResolvedWidthInStateDir(role, c.Config, time.Now())
	if err != nil {
		return nil, fmt.Errorf("Codex capacity: %w", err)
	}
	return []string{"-c", "agents.max_concurrent_threads_per_session=" + strconv.Itoa(width)}, nil
}

// A standing supervisor can outlive a width's TTL or an operator's resize.
// Replace both spellings of the previous capacity override before each fresh
// process, preserving every unrelated model/policy flag and the final prompt.
func (c *Cell) refreshCodexCapacity(role string, argv []string) ([]string, error) {
	if len(argv) < 2 {
		return nil, fmt.Errorf("Codex capacity requires a launch and final prompt")
	}
	capacity, err := c.codexCapacityArgs(role)
	if err != nil {
		return nil, err
	}
	options := argv[:len(argv)-1]
	out := make([]string, 0, len(argv)+len(capacity))
	for i := 0; i < len(options); i++ {
		a := options[i]
		if (a == "-c" || a == "--config") && i+1 < len(options) {
			if codexCapacitySetting(options[i+1]) {
				i++
				continue
			}
			out = append(out, a, options[i+1])
			i++
			continue
		}
		if value, ok := strings.CutPrefix(a, "--config="); ok && codexCapacitySetting(value) {
			continue
		}
		if value, ok := strings.CutPrefix(a, "-c="); ok && codexCapacitySetting(value) {
			continue
		}
		if value, ok := strings.CutPrefix(a, "-c"); ok && codexCapacitySetting(value) {
			continue
		}
		out = append(out, a)
	}
	out = append(out, capacity...)
	return append(out, argv[len(argv)-1]), nil
}

func codexCapacitySetting(s string) bool {
	key, _, hasValue := strings.Cut(s, "=")
	key = strings.TrimSpace(key)
	return hasValue && (key == "agents.max_concurrent_threads_per_session" || key == "agents.max_threads")
}

// Ask the installed CLI for its effective feature state: defaults, profiles,
// project configuration and managed requirements belong to Codex's resolver.
// Reading a literal config.toml both misses default-on and can ignore a higher
// priority false. An unavailable/malformed probe refuses; it never enables it.
func (c *Cell) codexMultiAgentOn() bool { return c.codexMultiAgentOnAt(c.Repo) }

func (c *Cell) codexMultiAgentOnAt(dir string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "codex", "features", "list")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if c.Kind == "scrubbed" {
		cmd.Env = envSet(cmd.Env, "CODEX_HOME", filepath.Join(c.Home, ".codex"))
	}
	out, err := cmd.Output()
	return err == nil && codexMultiAgentFeatureEnabled(string(out))
}

func codexMultiAgentFeatureEnabled(output string) bool {
	found, enabled := false, false
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "multi_agent" {
			continue
		}
		if found || len(fields) != 3 || (fields[2] != "true" && fields[2] != "false") {
			return false
		}
		found, enabled = true, fields[2] == "true"
	}
	return found && enabled
}
