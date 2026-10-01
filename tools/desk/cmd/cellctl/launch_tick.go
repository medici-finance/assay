package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// prepareTickLaunch changes only the execution mode of an already resolved
// interactive launch. Policy, provider, model, role, roots and working-directory
// arguments remain the caller's; no second configuration resolver lives here.
// The caller owns execution, timeout, process exclusion and cadence scheduling.
func prepareTickLaunch(harness string, argv, env []string, budget time.Duration) ([]string, []string, error) {
	if budget <= 0 || budget%time.Second != 0 {
		return nil, nil, fmt.Errorf("tick budget must be a positive whole number of seconds")
	}
	if len(argv) < 2 || argv[0] == "" || argv[len(argv)-1] == "" {
		return nil, nil, fmt.Errorf("tick launch requires an executable and final skill prompt")
	}
	var mode []string
	switch harness {
	case "codex":
		mode = []string{"exec"}
	case "claude":
		mode = []string{"--print", "--output-format", "text"}
	case "cursor":
		// Cursor print mode otherwise only proposes edits. Its supported --force
		// route honors explicit denials; cursorHeadlessPreflight checks that
		// contract before this branch can execute a real cadence launch.
		mode = []string{"--print", "--force", "--output-format", "text"}
	default:
		return nil, nil, fmt.Errorf("tick launch requires a supported noninteractive harness")
	}
	args := make([]string, 0, len(argv)+len(mode))
	args = append(args, argv[0])
	args = append(args, mode...)
	args = append(args, argv[1:len(argv)-1]...)
	if harness == "codex" {
		args = append(args, "-c", `shell_environment_policy.set.ASSAY_TICK="1"`, "-c",
			`shell_environment_policy.set.ASSAY_TICK_DEADLINE="`+strconv.FormatInt(int64(budget/time.Second), 10)+`"`)
	}
	args = append(args, argv[len(argv)-1]+"\nRun this skill in tick mode (--tick): exactly one bounded pass, print its tick summary line, and exit. Do not arm a wake or wait for another cadence.")

	// Do not inherit an outer caller's tick budget or conflicting spelling. Keep
	// credentials and every other prepared variable in memory, never in argv.
	envs := make([]string, 0, len(env)+2)
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		if strings.EqualFold(key, "ASSAY_TICK") || strings.EqualFold(key, "ASSAY_TICK_DEADLINE") {
			continue
		}
		envs = append(envs, kv)
	}
	envs = append(envs, "ASSAY_TICK=1", "ASSAY_TICK_DEADLINE="+strconv.FormatInt(int64(budget/time.Second), 10))
	return args, envs, nil
}
