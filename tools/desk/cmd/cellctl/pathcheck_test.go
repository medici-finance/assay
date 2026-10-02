package main

import (
	"strings"
	"testing"
)

func TestCellPathCheck(t *testing.T) {
	for _, group := range []string{"roots", "launcher"} {
		t.Run(group, func(t *testing.T) {
			for _, tc := range []struct{ name, path string }{
				{"backslash_unc", `\\srv\share\a`}, {"slash_unc", "//srv/share/a"}, {"mixed_sep_unc", `\/srv/share`},
				{"extended_unc", `\\?\UNC\srv\share\a`}, {"extended_drive", `\\?\C:\a`}, {"device_ns", `\\.\pipe\x`},
			} {
				t.Run(tc.name+"_refused", func(t *testing.T) {
					for _, os := range []string{"linux", "windows"} {
						err := cellPathCheck(os, tc.path)
						if err == nil || !strings.Contains(err.Error(), "UNC or device path") {
							t.Fatalf("%s %q: %v", os, tc.path, err)
						}
					}
				})
			}
		})
	}
	for _, tc := range []struct{ name, os, path string }{{"roots/posix_abs_accepted", "linux", "/a"}, {"roots/drive_letter_accepted", "windows", `C:\a`}, {"launcher/posix_exec_accepted", "linux", "/example/bin/run"}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := cellPathCheck(tc.os, tc.path); err != nil {
				t.Fatal(err)
			}
		})
	}
}
