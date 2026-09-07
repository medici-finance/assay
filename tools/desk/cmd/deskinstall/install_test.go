package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sum returns the lowercase-hex sha256 of b, the exact shape a manifest pins.
func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// tarGz builds a minimal gzip-tarball carrying one regular file, as the
// desk-tools release asset does.
func tarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fixture builds a temp manifest + the asset bytes it pins, for platform
// windows-amd64. Returns the manifest path, the two asset byte blobs, and a
// Fetcher that serves them by URL. The returned statusgen bytes are what the
// manifest's sha256 was computed over — callers TAMPER them to drive the
// negative path.
func fixture(t *testing.T, tag string) (manifestPath string, statusgen, desktools []byte, fetch Fetcher) {
	t.Helper()
	statusgen = []byte("STATUSGEN-WINDOWS-AMD64-EXE-fixture-bytes\x00\x01\x02")
	deskInner := []byte("deskboard.exe fixture bytes")
	desktools = tarGz(t, "deskboard.exe", deskInner)

	sgAsset := "statusgen-windows-amd64.exe"
	dtAsset := "desk-tools-windows-amd64.tar.gz"
	sgSum := sum(statusgen)
	dtSum := sum(desktools)

	manifest := fmt.Sprintf(`schema: paired-versions-v1
plugin: "0.5.1"
statusgen:
  release_home: medici-finance/assay
  tag: %[1]s
  platforms:
    windows-amd64: %[2]s %[1]s %[3]s
desk-tools:
  release_home: medici-finance/assay
  tag: %[1]s
  platforms:
    windows-amd64: %[4]s %[1]s %[5]s
`, tag, sgAsset, sgSum, dtAsset, dtSum)

	dir := t.TempDir()
	manifestPath = filepath.Join(dir, "paired-versions.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	sgURL := fmt.Sprintf("https://github.com/medici-finance/assay/releases/download/%s/%s", tag, sgAsset)
	dtURL := fmt.Sprintf("https://github.com/medici-finance/assay/releases/download/%s/%s", tag, dtAsset)
	fetch = func(url string) ([]byte, error) {
		switch url {
		case sgURL:
			return statusgen, nil
		case dtURL:
			return desktools, nil
		}
		return nil, fmt.Errorf("unexpected url %q", url)
	}
	return manifestPath, statusgen, desktools, fetch
}

// TestWindowsInstallVerifiesCorrectHash — POSITIVE PATH (Verify row 4).
// A correctly-hashed download installs: both assets are placed and a success
// line naming the pinned version + verified sha256 is emitted.
func TestWindowsInstallVerifiesCorrectHash(t *testing.T) {
	const tag = "v0.26.0"
	manifest, _, _, fetch := fixture(t, tag)
	dest := t.TempDir()
	var out bytes.Buffer

	err := Install(Options{
		ManifestPath: manifest,
		DestDir:      dest,
		Platform:     "windows-amd64",
		Fetch:        fetch,
		Out:          &out,
	})
	if err != nil {
		t.Fatalf("expected clean install, got refusal: %v", err)
	}

	// statusgen.exe placed.
	if _, err := os.Stat(filepath.Join(dest, "statusgen-windows-amd64.exe")); err != nil {
		t.Errorf("statusgen not placed: %v", err)
	}
	// desk-tools tarball extracted (its inner binary placed).
	if _, err := os.Stat(filepath.Join(dest, "deskboard.exe")); err != nil {
		t.Errorf("desk-tools binary not extracted: %v", err)
	}
	// Success line names version + verified hash (brief 04/05 assert on this).
	got := out.String()
	if !strings.Contains(got, "installed statusgen "+tag+" sha256:") {
		t.Errorf("missing/short success line, got: %q", got)
	}
}

// TestWindowsInstallRefusesOnHashMismatch — NEGATIVE PATH / SECURITY ROW
// (Verify row 5). A byte-flipped binary whose sha256 does not match the pin is
// REFUSED (non-nil error naming the mismatch) and NOTHING is placed — the
// destination binary does not exist. Because verify is phase 1 (before ANY
// placement), a mismatch on one component leaves the WHOLE install empty.
func TestWindowsInstallRefusesOnHashMismatch(t *testing.T) {
	const tag = "v0.26.0"
	manifest, statusgen, desktools, _ := fixture(t, tag)
	dest := t.TempDir()

	// Tamper statusgen bytes AFTER the manifest pinned the original hash.
	tampered := append([]byte(nil), statusgen...)
	tampered[0] ^= 0xff

	sgURL := "https://github.com/medici-finance/assay/releases/download/" + tag + "/statusgen-windows-amd64.exe"
	dtURL := "https://github.com/medici-finance/assay/releases/download/" + tag + "/desk-tools-windows-amd64.tar.gz"
	tamperFetch := func(url string) ([]byte, error) {
		switch url {
		case sgURL:
			return tampered, nil
		case dtURL:
			return desktools, nil
		}
		return nil, fmt.Errorf("unexpected url %q", url)
	}

	err := Install(Options{
		ManifestPath: manifest,
		DestDir:      dest,
		Platform:     "windows-amd64",
		Fetch:        tamperFetch,
	})
	if err == nil {
		t.Fatal("SECURITY: tampered binary was accepted — install must REFUSE on hash mismatch")
	}
	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("refusal did not name the mismatch: %v", err)
	}
	// Nothing placed — the destination binary must not exist.
	if _, statErr := os.Stat(filepath.Join(dest, "statusgen-windows-amd64.exe")); !os.IsNotExist(statErr) {
		t.Errorf("REFUSED install still placed statusgen.exe (stat err=%v)", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dest, "deskboard.exe")); !os.IsNotExist(statErr) {
		t.Errorf("REFUSED install still placed a desk-tools binary (stat err=%v)", statErr)
	}
}

// TestWindowsInstallRefusesAbsentPin — the pin for a detected platform is absent:
// the installer REFUSES (never guesses), placing nothing.
func TestWindowsInstallRefusesAbsentPin(t *testing.T) {
	manifest, _, _, fetch := fixture(t, "v0.26.0")
	dest := t.TempDir()
	err := Install(Options{
		ManifestPath: manifest,
		DestDir:      dest,
		Platform:     "windows-arm64", // fixture only pins windows-amd64
		Fetch:        fetch,
	})
	if err == nil {
		t.Fatal("expected refusal for an unpinned platform, got clean install")
	}
	if !strings.Contains(err.Error(), "no pin line") {
		t.Errorf("refusal did not name the absent pin: %v", err)
	}
}
