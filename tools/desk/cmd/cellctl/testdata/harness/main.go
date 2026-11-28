// This stand-in exercises the process protocol used by the cellctl feature and cadence tests.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "features" && os.Args[2] == "list" {
		features()
		return
	}
	for _, arg := range os.Args[1:] {
		if arg == "--cellctl-cadence-fixture" {
			cadence()
			return
		}
	}
	os.Exit(23)
}

func features() {
	if expected := os.Getenv("FEATURE_EXPECT_HOME"); expected != "" && os.Getenv("CODEX_HOME") != expected {
		os.Exit(24)
	}
	if expected := os.Getenv("FEATURE_EXPECT_CWD"); expected != "" {
		cwd, _ := os.Getwd()
		real, _ := filepath.EvalSymlinks(expected)
		if cwd != real {
			os.Exit(25)
		}
	}
	if os.Getenv("FEATURE_FAIL") == "1" {
		os.Exit(26)
	}
	_, _ = os.Stdout.WriteString(os.Getenv("FEATURE_OUTPUT"))
}

func cadence() {
	if os.Getenv("DESK_LOOP") != "worker-desk" || os.Getenv("DESK_ROOTS") != "example/repo=/a path" || os.Getenv("ASSAY_TICK") != "1" || os.Getenv("ASSAY_TICK_DEADLINE") != "1" || os.Getenv("ASSAY_COCKPIT") != "herdr" {
		os.Exit(23)
	}
	cwd, _ := os.Getwd()
	want, _ := filepath.EvalSymlinks(os.Getenv("FIXTURE_CWD"))
	if cwd != want {
		os.Exit(24)
	}
	final := "tick role=worker-desk outcome=noop swept=2 acted=0 filed=0 duration=1\n"
	if os.Getenv("FIXTURE_BAD_SUMMARY") == "1" {
		final = "completed without evidence\n"
	}
	if record := os.Getenv("FIXTURE_RECORD"); record != "" {
		f, err := os.OpenFile(record, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(25)
		}
		if _, err := fmt.Fprintln(f, "pass"); err != nil {
			os.Exit(25)
		}
		if err := f.Close(); err != nil {
			os.Exit(25)
		}
	}
	if os.Getenv("FIXTURE_CACHE_WRITE") == "1" {
		if os.WriteFile(filepath.Join(os.Getenv("GOCACHE"), "cadence-result"), []byte("compiled result"), 0600) != nil {
			os.Exit(27)
		}
	}
	for i, a := range os.Args[1:] {
		if a == "--output-last-message" && i+2 < len(os.Args) {
			if os.WriteFile(os.Args[i+2], []byte(final), 0600) != nil {
				os.Exit(26)
			}
			fmt.Println("progress after final file; not completion evidence")
			os.Exit(0)
		}
	}
	fmt.Print(final)
	os.Exit(0)
}
