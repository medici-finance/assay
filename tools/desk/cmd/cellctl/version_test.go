package main

import (
	"os"
	"runtime/debug"
	"strings"
	"testing"
)

const testRevision = "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b"

// fakeBuildInfo returns a build-info source carrying the given settings (key, value pairs).
func fakeBuildInfo(kv ...string) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) {
		info := &debug.BuildInfo{}
		for i := 0; i+1 < len(kv); i += 2 {
			info.Settings = append(info.Settings, debug.BuildSetting{Key: kv[i], Value: kv[i+1]})
		}
		return info, true
	}
}

func TestVersionString(t *testing.T) {
	noInfo := func() (*debug.BuildInfo, bool) { return nil, false }
	cases := []struct {
		name    string
		stamped string
		read    func() (*debug.BuildInfo, bool)
		want    string
	}{
		{"stamped tag wins over vcs", "v9.9.9-test",
			fakeBuildInfo("vcs.revision", testRevision, "vcs.modified", "true"), "v9.9.9-test"},
		{"stamped tag with no build info", "v1.2.3", noInfo, "v1.2.3"},
		{"dev + revision, clean", "dev",
			fakeBuildInfo("vcs.revision", testRevision, "vcs.modified", "false"), "dev-1a2b3c4d5e6f"},
		{"dev + revision, modified unset", "dev",
			fakeBuildInfo("vcs.revision", testRevision), "dev-1a2b3c4d5e6f"},
		{"dev + revision + modified", "dev",
			fakeBuildInfo("vcs.revision", testRevision, "vcs.modified", "true"), "dev-1a2b3c4d5e6f-dirty"},
		{"dev + no revision", "dev", fakeBuildInfo("vcs.modified", "true"), "dev"},
		{"dev + empty revision", "dev", fakeBuildInfo("vcs.revision", ""), "dev"},
		{"dev + no build info", "dev", noInfo, "dev"},
		{"dev + nil build info ok", "dev",
			func() (*debug.BuildInfo, bool) { return nil, true }, "dev"},
		{"dev + nil source", "dev", nil, "dev"},
		{"dev + short revision whole", "dev",
			fakeBuildInfo("vcs.revision", "abc123", "vcs.modified", "true"), "dev-abc123-dirty"},
		{"dev + exactly 12 hex", "dev",
			fakeBuildInfo("vcs.revision", "0123456789ab"), "dev-0123456789ab"},
		{"dev + sha256 revision", "dev",
			fakeBuildInfo("vcs.revision", strings.Repeat("f", 64)), "dev-ffffffffffff"},
		{"dev + non-hex revision", "dev",
			fakeBuildInfo("vcs.revision", "not-a-hash"), "dev"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionString(tc.stamped, tc.read); got != tc.want {
				t.Errorf("versionString(%q) = %q, want %q", tc.stamped, got, tc.want)
			}
		})
	}
}

// TestVersionVerbUsesBuildInfo pins the wiring: `--version` and `version` print what
// versionString derives from the build-info source, not the bare cellctlVersion var.
func TestVersionVerbUsesBuildInfo(t *testing.T) {
	oldRead, oldArgs, oldVer := readBuildInfo, os.Args, cellctlVersion
	t.Cleanup(func() { readBuildInfo, os.Args, cellctlVersion = oldRead, oldArgs, oldVer })
	readBuildInfo = fakeBuildInfo("vcs.revision", testRevision, "vcs.modified", "true")
	cellctlVersion = unstampedVersion
	for _, arg := range []string{"--version", "version"} {
		os.Args = []string{"cellctl", arg}
		var rc int
		out := captureStdout(t, func() { rc = run() })
		if rc != 0 || out != "dev-1a2b3c4d5e6f-dirty\n" {
			t.Errorf("cellctl %s: rc=%d out=%q, want rc=0 out=%q", arg, rc, out, "dev-1a2b3c4d5e6f-dirty\n")
		}
	}
	cellctlVersion = "v9.9.9-test"
	os.Args = []string{"cellctl", "--version"}
	if out := captureStdout(t, func() { run() }); out != "v9.9.9-test\n" {
		t.Errorf("stamped cellctl --version = %q, want %q", out, "v9.9.9-test\n")
	}
}
