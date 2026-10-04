//go:build windows

package deskkit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Windows readers can deny delete sharing, including the replacement operation.
// A bounded refusal must preserve the old committed file, release the writer
// lock, and clean its temporary; closing the reader permits a subsequent write.
func TestBeaconStoreWindowsSharingViolationPreservesCommittedFile(t *testing.T) {
	beaconStoreFixture(t)
	seed := `{"session":"shared","foreign":{"retained":true}}`
	path := beaconStoreSeed(t, "shared", seed)
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = windows.CloseHandle(handle)
		}
	}()
	start := time.Now()
	if _, err := AppendAck("shared", AckRecord{Restatement: "must remain unpublished"}); err == nil {
		t.Fatal("replacement succeeded through a handle that denies delete sharing")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("replacement wait was not bounded: %s", elapsed)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != seed {
		t.Fatalf("failed replacement changed original bytes: %q", data)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".roster-beacon-") {
			t.Errorf("failed replacement leaked temporary %s", entry.Name())
		}
	}
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	closed = true
	if _, err := AppendAck("shared", AckRecord{Restatement: "committed after reader closes"}); err != nil {
		t.Fatalf("write after reader closes: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("must remain unpublished")) {
		t.Fatal("refused transaction leaked into the next write")
	}
	if !bytes.Contains(data, []byte("committed after reader closes")) {
		t.Fatalf("subsequent transaction did not commit: %s", data)
	}
}
