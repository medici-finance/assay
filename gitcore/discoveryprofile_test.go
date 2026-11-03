package gitcore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type discoveryInfo struct {
	dev uint64
	dir bool
}

func (i discoveryInfo) Name() string { return "fixture" }
func (i discoveryInfo) Size() int64  { return 0 }
func (i discoveryInfo) Mode() os.FileMode {
	if i.dir {
		return os.ModeDir | 0755
	}
	return 0644
}
func (i discoveryInfo) ModTime() time.Time { return time.Time{} }
func (i discoveryInfo) IsDir() bool        { return i.dir }
func (i discoveryInfo) Sys() any           { return i.dev }

// regression: #2395
func TestFilesystemProfile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	dir := filepath.Join(root, "middle", "cwd")
	boom := errors.New("synthetic stat failure")
	for _, c := range []struct {
		name                  string
		devices               []uint64
		unknown, fail, notdir bool
		wantNative            bool
	}{
		{name: "same", devices: []uint64{1, 1, 1}},
		{name: "boundary", devices: []uint64{1, 2, 2}, wantNative: true},
		{name: "same-endpoints-intermediate-boundary", devices: []uint64{1, 2, 1}, wantNative: true},
		{name: "unknown-platform", devices: []uint64{1, 1, 1}, unknown: true, wantNative: true},
		{name: "stat-error", devices: []uint64{1, 1, 1}, fail: true},
		{name: "not-directory", devices: []uint64{1, 1, 1}, notdir: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			paths := []string{dir, filepath.Dir(dir), root}
			calls := 0
			stat := func(path string) (os.FileInfo, error) {
				if calls >= len(paths) || path != paths[calls] {
					t.Fatalf("observation escaped resolved chain: %q at %d", path, calls)
				}
				n := calls
				calls++
				if c.fail && n == 1 {
					return nil, boom
				}
				return discoveryInfo{dev: c.devices[n], dir: !c.notdir || n != 0}, nil
			}
			device := func(info os.FileInfo) (uint64, bool) { return info.(discoveryInfo).dev, !c.unknown }
			why, err := filesystemReadReason(dir, root, stat, device)
			if c.fail {
				if !errors.Is(err, boom) || why != "" {
					t.Fatalf("stat failure demoted: %q %v", why, err)
				}
				return
			}
			if c.notdir {
				if err == nil || why != "" {
					t.Fatalf("non-directory was accepted: %q %v", why, err)
				}
				return
			}
			if err != nil || (why != "") != c.wantNative {
				t.Fatalf("profile=%q %v, nativewant=%v", why, err, c.wantNative)
			}
			if !c.wantNative && calls != 3 {
				t.Fatalf("incomplete same-filesystem observation: %d", calls)
			}
		})
	}
	if why, err := filesystemReadReason(dir, filepath.Join(t.TempDir(), "other"), os.Stat, platformDeviceIdentity); err == nil || why != "" {
		t.Fatalf("unrelated root accepted: %q %v", why, err)
	}
}

// regression: #2395
func TestPhysicalFilesystemProfile(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	why, err := NativeFilesystemReadReason(filepath.Join(physical, "nested"), physical)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := platformDeviceIdentity(info); ok && why != "" {
		t.Fatalf("same filesystem incorrectly native: %q", why)
	} else if !ok && !strings.Contains(why, "unavailable") {
		t.Fatalf("unknown device capability was guessed: %q", why)
	}
}
