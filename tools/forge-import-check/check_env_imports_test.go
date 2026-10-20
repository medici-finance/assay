package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #2743
func TestEnvImportRows(t *testing.T) {
	helper, err := os.ReadFile("check_env_imports.go")
	if err != nil {
		t.Fatal(err)
	}
	quote := string(rune(96))
	cases := []struct {
		name, literal, alias, prefix, suffix, expr string
		want                                       string
	}{
		{"plain", "\"os\"", "", "", ".go", "os.Getenv(\"BOUNDARY_FIXTURE\")", "1"},
		{"alias", "\"os\"", "envx ", "", ".go", "envx.Getenv(\"BOUNDARY_FIXTURE\")", "1"},
		{"dot", "\"os\"", ". ", "", ".go", "Getenv(\"BOUNDARY_FIXTURE\")", "1"},
		{"raw_os", quote + "os" + quote, "", "", ".go", "os.Getenv(\"BOUNDARY_FIXTURE\")", "1"},
		{"raw_syscall", quote + "syscall" + quote, "", "", ".go", "syscall.Environ()", "1"},
		{"hex_os", "\"\\x6f\\x73\"", "", "", ".go", "os.Getenv(\"BOUNDARY_FIXTURE\")", "1"},
		{"octal_syscall", "\"\\163\\171\\163\\143\\141\\154\\154\"", "", "", ".go", "syscall.Environ()", "1"},
		{"unicode_os", "\"\\u006f\\U00000073\"", "", "", ".go", "os.Getenv(\"BOUNDARY_FIXTURE\")", "1"},
		{"build_tag", quote + "os" + quote, "envx ", "//go:build boundary_fixture\n\n", ".go", "envx.Getenv(\"BOUNDARY_FIXTURE\")", "1"},
		{"test_excluded", quote + "os" + quote, "", "", "_test.go", "os.Getenv(\"BOUNDARY_FIXTURE\")", "0"},
		{"clean", "\"sort\"", "", "", ".go", "sort.IntsAreSorted(nil)", "0"},
	}
	for _, row := range []struct{ brief, number string }{
		{"brief-37-scoped-forge-mirror-with-persisted-freshness-coverage-and-polling-repair.md", "18"},
		{"brief-38-forge-event-adapter-feedback-observations-and-durable-reevaluation-demand.md", "13"},
	} {
		data, err := os.ReadFile(filepath.Join("..", "..", "docs", "streams", "forge-neutral", row.brief))
		if err != nil {
			t.Fatal(err)
		}
		var command string
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "| "+row.number+" |") {
				parts := strings.Split(line, quote)
				if len(parts) < 3 {
					t.Fatal("row has no command")
				}
				command = parts[1]
			}
		}
		fields := strings.Fields(command)
		if len(fields) != 7 || strings.Join(fields[:5], " ") != "GOWORK=off GOPROXY=off GOTOOLCHAIN=local go run" {
			t.Fatalf("unrecognized or absent row command: %q", command)
		}
		for _, tc := range cases {
			t.Run(row.number+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				write := func(path, content string) {
					t.Helper()
					path = filepath.Join(root, path)
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				write(fields[5], string(helper))
				write("go.mod", "module fixture.invalid/imports\n\ngo 1.25.0\n")
				target := fields[6]
				write(target+"/base.go", "package fixture\n")
				source := tc.prefix + "package fixture\nimport " + tc.alias + tc.literal + "\nfunc readFixture() { _ = " + tc.expr + " }\n"
				write(target+"/plant"+tc.suffix, source)
				env := append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local", "GOSUMDB=off")
				// Compile each synthetic fixture, including the normally excluded
				// build-tag case. No test or environment-read function is called.
				build := exec.Command("go", "test", "-run", "^$", "-timeout=20s", "-tags=boundary_fixture", "./"+target)
				build.Dir, build.Env = root, env
				if output, err := build.CombinedOutput(); err != nil {
					t.Fatalf("fixture is not compiler-valid: %v\n%s", err, output)
				}
				cmd := exec.Command(fields[3], fields[4:]...)
				cmd.Dir, cmd.Env = root, env
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				err := cmd.Run()
				if strings.TrimSpace(stdout.String()) != tc.want || (err == nil) != (tc.want == "0") {
					t.Fatalf("exact row %s: count=%q err=%v want=%s; %s", row.number, stdout.String(), err, tc.want, stderr.String())
				}
			})
		}
	}
}

// regression: #2743
func TestEnvImportFailures(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, errs bytes.Buffer
	run := func(target string, want int) {
		t.Helper()
		out.Reset()
		errs.Reset()
		if got := check([]string{target}, &out, &errs); got != want || out.Len() != 0 {
			t.Fatalf("%s: exit=%d stdout=%q stderr=%q; want %d and no count", target, got, out.String(), errs.String(), want)
		}
	}
	run("../escape", 2)
	run("forgeread/mirror", 2)
	if err := os.MkdirAll("forgeread/mirror", 0o755); err != nil {
		t.Fatal(err)
	}
	run("forgeread/mirror", 2)
	if err := os.WriteFile("forgeread/mirror/bad.go", []byte("package fixture\nfunc broken("), 0o644); err != nil {
		t.Fatal(err)
	}
	run("forgeread/mirror", 2)
	if err := os.Remove("forgeread/mirror/bad.go"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), "forgeread/mirror/link"); err != nil {
		t.Fatal(err)
	}
	run("forgeread/mirror", 2)
	if err := os.Symlink(t.TempDir(), "forgeread/events"); err != nil {
		t.Fatal(err)
	}
	run("forgeread/events", 2)
	t.Chdir(t.TempDir())
	if err := os.Symlink(t.TempDir(), "forgeread"); err != nil {
		t.Fatal(err)
	}
	run("forgeread/mirror", 2)
}
