package deskkit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// hostLine is this host's per-platform pin-line name for artifact — the first
// candidate PlatformPin tries after the bare line (`.exe` form on Windows).
func hostLine(artifact string) string { return HostPlatformAssets(artifact)[0] }

func writePins(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, AssayVersionsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestPlatformPin_BarePreferred — when both shapes are present the bare line wins,
// even when the platform line carries a different tag.
func TestPlatformPin_BarePreferred(t *testing.T) {
	root := writePins(t, "statusgen v1.0.8 "+strings.Repeat("a", 64)+"\n"+
		hostLine("statusgen")+" v1.0.7 "+strings.Repeat("b", 64)+"\n")
	tag, sha, err := StatusgenPin(root)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.0.8" || sha != strings.Repeat("a", 64) {
		t.Errorf("bare line must win: got %s %s", tag, sha)
	}
}

// TestPlatformPin_FallsBackToHostPlatform — the adopter shape from
// docs/adopting-assay.md § install-statusgen: per-platform lines only. The host's
// own line is read; a bare-only reader (ArtifactPin) still refuses, unchanged.
func TestPlatformPin_FallsBackToHostPlatform(t *testing.T) {
	other := "statusgen-plan9-mips"
	root := writePins(t, "# per-platform only\n"+
		other+"  v1.0.8  "+strings.Repeat("c", 64)+"\n"+
		hostLine("statusgen")+"  v1.0.8  "+strings.Repeat("d", 64)+"  # this host\n")
	tag, sha, err := StatusgenPin(root)
	if err != nil {
		t.Fatalf("host-platform fallback: %v", err)
	}
	if tag != "v1.0.8" || sha != strings.Repeat("d", 64) {
		t.Errorf("got %s %s, want the host line's v1.0.8 %s", tag, sha, strings.Repeat("d", 64))
	}
	if _, _, err := ArtifactPin(root, "statusgen"); err == nil {
		t.Error("the strict bare reader must still refuse a per-platform-only file")
	}
}

// TestPlatformPin_MalformedBareIsNotSkipped — a malformed bare line is a refusal,
// never a hint to read the platform line instead.
func TestPlatformPin_MalformedBareIsNotSkipped(t *testing.T) {
	root := writePins(t, "statusgen only-a-tag\n"+
		hostLine("statusgen")+" v1.0.8 "+strings.Repeat("d", 64)+"\n")
	if _, _, err := StatusgenPin(root); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Errorf("malformed bare line must fail closed, got err=%v", err)
	}
	// A malformed platform line refuses too.
	root = writePins(t, hostLine("statusgen")+" only-a-tag\n")
	if _, _, err := StatusgenPin(root); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Errorf("malformed platform line must fail closed, got err=%v", err)
	}
}

// TestPlatformPin_NeitherShapeRefuses — no bare line and no line for THIS host
// (only some other platform's) is still "no statusgen pin", naming what it tried.
func TestPlatformPin_NeitherShapeRefuses(t *testing.T) {
	root := writePins(t, "statusgen-plan9-mips v1.0.8 "+strings.Repeat("c", 64)+"\n")
	_, _, err := StatusgenPin(root)
	if err == nil {
		t.Fatal("a file with no line for this host returned no error")
	}
	if !IsUnverifiable(err) || !strings.Contains(err.Error(), "no statusgen pin") ||
		!strings.Contains(err.Error(), hostLine("statusgen")) {
		t.Errorf("refusal must be Unverifiable and name the host line it looked for: %v", err)
	}
	if _, _, err := StatusgenPin(t.TempDir()); err == nil {
		t.Error("a missing pin file returned no error")
	}
}

func TestHostPlatformAssets(t *testing.T) {
	got := HostPlatformAssets("statusgen")
	base := "statusgen-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		if len(got) != 2 || got[0] != base+".exe" || got[1] != base {
			t.Errorf("windows candidates = %v", got)
		}
		return
	}
	if len(got) != 1 || got[0] != base {
		t.Errorf("candidates = %v, want [%s]", got, base)
	}
}

func TestComponentOf(t *testing.T) {
	for in, want := range map[string]string{
		"statusgen":                       "statusgen",
		"statusgen-darwin-arm64":          "statusgen",
		"statusgen-windows-amd64.exe":     "statusgen",
		"desk-tools-linux-amd64":          "desk-tools",
		"desk-tools-windows-arm64.tar.gz": "desk-tools",
		"desk-tools-source":               "desk-tools-source",
		"daily-harvest-linux-amd64":       "daily-harvest",
		"checksums.txt":                   "checksums.txt",
	} {
		if got := ComponentOf(in); got != want {
			t.Errorf("ComponentOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestComponentPinTag — the marker's component reader across line shapes.
func TestComponentPinTag(t *testing.T) {
	t.Run("bare wins", func(t *testing.T) {
		root := writePins(t, "statusgen v1.0.8 "+strings.Repeat("a", 64)+"\nstatusgen-linux-amd64 v1.0.7 "+strings.Repeat("b", 64)+"\n")
		tag, present, err := ComponentPinTag(root, "statusgen")
		if err != nil || !present || tag != "v1.0.8" {
			t.Errorf("got (%q,%v,%v)", tag, present, err)
		}
	})
	t.Run("platform lines agree", func(t *testing.T) {
		root := writePins(t, "statusgen-linux-amd64 v1.0.8 "+strings.Repeat("a", 64)+"  # ci\n"+
			"statusgen-windows-amd64.exe v1.0.8 "+strings.Repeat("b", 64)+"\n"+
			"desk-tools-linux-amd64 v1.0.7 "+strings.Repeat("c", 64)+"\n"+
			"desk-tools-windows-amd64.tar.gz v1.0.7 "+strings.Repeat("d", 64)+"\n"+
			"desk-tools-source v1.0.7 "+strings.Repeat("e", 40)+"\n")
		tag, present, err := ComponentPinTag(root, "statusgen")
		if err != nil || !present || tag != "v1.0.8" {
			t.Errorf("statusgen: got (%q,%v,%v)", tag, present, err)
		}
		tag, present, err = ComponentPinTag(root, "desk-tools")
		if err != nil || !present || tag != "v1.0.7" {
			t.Errorf("desk-tools (tarball + source lines must not confuse it): got (%q,%v,%v)", tag, present, err)
		}
	})
	t.Run("platform lines disagree", func(t *testing.T) {
		root := writePins(t, "statusgen-linux-amd64 v1.0.8 "+strings.Repeat("a", 64)+"\n"+
			"statusgen-darwin-arm64 v1.0.7 "+strings.Repeat("b", 64)+"\n")
		_, _, err := ComponentPinTag(root, "statusgen")
		if err == nil || !strings.Contains(err.Error(), "disagree") {
			t.Errorf("a half-moved bump must refuse, got %v", err)
		}
	})
	t.Run("absent is the distinct third state", func(t *testing.T) {
		root := writePins(t, "desk-tools-linux-amd64 v1.0.7 "+strings.Repeat("c", 64)+"\n")
		tag, present, err := ComponentPinTag(root, "qualgen")
		if err != nil || present || tag != "" {
			t.Errorf("got (%q,%v,%v), want (\"\", false, nil)", tag, present, err)
		}
	})
	t.Run("malformed refuses", func(t *testing.T) {
		root := writePins(t, "statusgen-linux-amd64 v1.0.8\n")
		if _, _, err := ComponentPinTag(root, "statusgen"); err == nil {
			t.Error("malformed platform line must refuse")
		}
	})
}
