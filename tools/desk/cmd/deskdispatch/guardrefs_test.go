package main

import "testing"

func TestAllocationReferences(t *testing.T) {
	for name, src := range map[string]string{
		"os-process":      `package main; import "os"; func other() { os.StartProcess(command, argv, nil) }`,
		"syscall-process": `package main; import "syscall"; func other() { syscall.Exec(command, argv, nil) }`,
		"method-value":    `package main; type x struct{}; func (x) invoke(command string) { runCmd("", command, "add") }; var call = x{}.invoke`,
		"constant":        `package main; const command = "deskwt"; func other() { runCmd("", command, "add") }`,
		"variable":        `package main; var command = "deskwt"; func other() { runCmd("", command, "add") }`,
		"concatenated":    `package main; const command = "desk" + "wt"; func other() { runCmd("", command, "add") }`,
		"cross-file":      `package main; func other() { runCmd("", commandFromOtherFile, "add") }`,
		"slice":           `package main; func other() { argv := []string{"deskwt", "add"}; runCmd("", argv[0], argv[1:]...) }`,
		"alias":           `package main; func other(command string) { call := runCmd; call("", command, "add") }`,
		"wrapper":         `package main; func other(command string) { runCmd("", command, "add") }`,
		"method":          `package main; type example struct{}; func (example) createDispatchWorktree() { runCmd("", commandFromOtherFile, "add") }`,
		"initializer":     `package main; var result = runCmd("", commandFromOtherFile, "add")`,
		"closure":         `package main; var callback = func(command string) { runCmd("", command, "add") }`,
		"environment":     `package main; import "os"; func other() { runCmd("", os.Getenv("TOOL"), "add") }`,
		"exec-alias":      `package main; import proc "os/exec"; func other() { proc.Command(commandFromOtherFile, "add") }`,
		"runner-alias":    `package main; func other() { call := execCommand; call(commandFromOtherFile, "add") }`,
		"git-allocation":  `package main; func other() { runCmd("", "git", "worktree", "add", "--branch", "other") }`,
		"git-wrapper":     `package main; func other() { gitOut("", "worktree", "add", "--branch", "other") }`,
		"shell":           `package main; func other() { runCmd("", "sh", "-c", commandFromOtherFile) }`,
	} {
		t.Run(name, func(t *testing.T) {
			if bad := allocationViolations(t, "other.go", []byte(src)); len(bad) == 0 {
				t.Fatal("unproven allocation path admitted")
			}
		})
	}
}

func TestAllocationRelayBoundary(t *testing.T) {
	for _, tc := range []struct{ file, src string }{
		{"dispatch.go", `package main; func stepClaim(o dispatchOpts, auth claimAuth, script string) { args := []string{}; invoke := func() { runCmdEnv(o.root, auth.env, script, append(args, auth.args...)...) }; invoke() }`},
		{"reviewworktree.go", `package main; func createDispatchWorktree(o dispatchOpts, plan dispatchPlan) { args := []string{}; invoke := func() { runCmd(o.root, "deskwt", args...) }; invoke() }`},
		{"exec.go", `package main; import "github.com/medici-finance/assay/tools/desk/internal/deskkit"; func runCmdEnv() { invoke := func() { call := deskkit.ToolCall{Name: name, Args: args, Dir: dir, Env: env, Start: execCommand}; deskkit.Run(call) }; invoke() }`},
		{"dispatch.go", `package main; func stepClaim(o dispatchOpts, auth claimAuth, script string) { args := []string{}; if enabled { script := "deskwt"; runCmdEnv(o.root, auth.env, script, append(args, auth.args...)...) } }`},
		{"exec.go", `package main; func runCmd(dir, name string, args ...string) runResult { return runCmdEnv(dir, nil, replacement, args...) }`},
		{"other.go", `package main; func runCmd(dir, name string, args ...string) runResult { return runCmdEnv(dir, nil, name, args...) }`},
		{"dispatch.go", `package main; func stepClaim() { runCmdEnv(o.root, auth.env, command, append(args, auth.args...)...) }`},
		{"dispatch.go", `package main; type x struct{}; func (x) stepClaim() { runCmdEnv(o.root, auth.env, script, append(args, auth.args...)...) }`},
		{"repairadmission.go", `package main; type x struct{}; func (b *x) acquireLease() { runCmdEnv(b.o.root, b.auth.env, b.plan.claimTool, append([]string{"acquire", key, "--repo", b.repo}, b.auth.args...)...) }`},
		{"exec.go", `package main; import "github.com/medici-finance/assay/tools/desk/internal/deskkit"; func runCmdEnv() { call := deskkit.ToolCall{Name: name, Args: args, Dir: dir, Env: env, Start: replacement}; deskkit.Run(call) }`},
		{"exec.go", `package main; import "os/exec"; var another = exec.Command`},
		{"other.go", `package main; import tools "github.com/medici-finance/assay/tools/desk/internal/deskkit"; var execute = tools.Run`},
		{"other.go", `package main; import tools "github.com/medici-finance/assay/tools/desk/internal/deskkit"; func other() { tools.Run(tools.ToolCall{Name: command}) }`},
		{"other.go", `package main; import . "os/exec"; var invoke = Command`},
	} {
		if bad := allocationViolations(t, tc.file, []byte(tc.src)); len(bad) == 0 {
			t.Errorf("relay exception escaped exact boundary: %s", tc.src)
		}
	}
}

func TestAllocationHealthyReads(t *testing.T) {
	src := []byte(`package main; func reads() { runCmd("", "git", "rev-parse", "HEAD"); gitOut("", "rev-parse", "HEAD"); runCmd("", "deskroster", "set", "--repo", repo) }`)
	if bad := allocationViolations(t, "other.go", src); len(bad) != 0 {
		t.Fatalf("known non-allocation calls rejected: %v", bad)
	}
}
