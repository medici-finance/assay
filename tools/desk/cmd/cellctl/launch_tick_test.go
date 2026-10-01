package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPrepareTickLaunchPreservesResolvedLaunch(t *testing.T) {
	for _, tc := range []struct {
		harness string
		argv    []string
		mode    []string
	}{
		{"codex", []string{"/tools with spaces/codex", "-c", `model_provider="approved"`, "-c", `model_reasoning_effort="high"`, "--sandbox", "danger-full-access", "-C", "/work's tree & files", "-m", "pinned-model", `Invoke the "assay:worker-desk" skill now.`}, []string{"exec"}},
		{"claude", []string{"claude", "--effort", "high", "--settings", `{"availableModels":["pinned-model"]}`, "--name", "cell-role-session", "--model", "pinned-model", "/assay:pr-review-desk"}, []string{"--print", "--output-format", "text"}},
		{"cursor", []string{"agent", "--workspace", `C:\work's tree & files`, "--model", "pinned-model", `Invoke the "assay:verify-desk" skill now.`}, []string{"--print", "--output-format", "text"}},
	} {
		t.Run(tc.harness, func(t *testing.T) {
			env := []string{"PATH=/cell/shim:/approved/tools", "DESK_LOOP=worker-desk", "DESK_SESSION=cell-role-session", "DESK_ROOTS=example/repo=/work's tree & files", "ASSAY_COCKPIT=herdr", "CLAUDE_CONFIG_DIR=/cell/config", "ANTHROPIC_BASE_URL=https://provider.invalid", "ANTHROPIC_AUTH_TOKEN=synthetic-secret", "ASSAY_REPAIR_ADMISSION=on", "ASSAY_TICK=0", "assay_tick_deadline=999"}
			originalArgs, originalEnv := append([]string(nil), tc.argv...), append([]string(nil), env...)
			args, envs, err := prepareTickLaunch(tc.harness, tc.argv, env, 20*time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			prefix := append([]string{tc.argv[0]}, tc.mode...)
			prefix = append(prefix, tc.argv[1:len(tc.argv)-1]...)
			if !reflect.DeepEqual(args[:len(args)-1], prefix) {
				t.Fatalf("resolved argv changed: got %#v, want prefix %#v", args, prefix)
			}
			prompt := args[len(args)-1]
			if !strings.HasPrefix(prompt, tc.argv[len(tc.argv)-1]+"\n") || !strings.Contains(prompt, "--tick") || !strings.Contains(prompt, "exactly one bounded pass") {
				t.Fatalf("role prompt or tick contract lost: %q", prompt)
			}
			wantEnv := append(append([]string(nil), env[:len(env)-2]...), "ASSAY_TICK=1", "ASSAY_TICK_DEADLINE=1200")
			if !reflect.DeepEqual(envs, wantEnv) {
				t.Fatal("prepared environment changed beyond tick mode and budget")
			}
			if strings.Contains(strings.Join(args, "\n"), "synthetic-secret") {
				t.Fatal("credential entered argv")
			}
			args[0], envs[0] = "mutated", "mutated"
			if !reflect.DeepEqual(tc.argv, originalArgs) || !reflect.DeepEqual(env, originalEnv) {
				t.Fatal("tick preparation shares mutable slices with the original launch")
			}
		})
	}
}

func TestPrepareTickLaunchRefusesInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		harness string
		argv    []string
		budget  time.Duration
	}{
		{"other", []string{"tool", "skill"}, time.Minute},
		{"codex", nil, time.Minute},
		{"codex", []string{"codex"}, time.Minute},
		{"codex", []string{"", "skill"}, time.Minute},
		{"codex", []string{"codex", ""}, time.Minute},
		{"codex", []string{"codex", "skill"}, 0},
		{"codex", []string{"codex", "skill"}, -time.Second},
		{"codex", []string{"codex", "skill"}, 1500 * time.Millisecond},
	} {
		args, env, err := prepareTickLaunch(tc.harness, tc.argv, nil, tc.budget)
		if err == nil || args != nil || env != nil {
			t.Fatalf("accepted invalid tick launch: %#v", tc)
		}
	}
}
