package gitcore

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type alternateCloseRead struct {
	io.Reader
	closeErr error
	closes   int
}

func (r *alternateCloseRead) Close() error { r.closes++; return r.closeErr }

// regression: #2395
func TestAlternatesClose(t *testing.T) {
	root := t.TempDir()
	objects := filepath.Join(root, "borrowed", "objects")
	if err := os.MkdirAll(objects, 0755); err != nil {
		t.Fatal(err)
	}
	payload := objects + "\n"
	baselineReader := &alternateCloseRead{Reader: strings.NewReader(payload)}
	baseline := readAlternatesWith(root, func(string) (io.ReadCloser, error) { return baselineReader, nil })
	if baselineReader.closes != 1 || baseline.fs == nil || baseline.diagnosis != "" {
		t.Fatalf("healthy alternate fixture: %+v closes=%d", baseline, baselineReader.closes)
	}
	failingReader := &alternateCloseRead{Reader: strings.NewReader(payload), closeErr: closeCanary}
	got := readAlternatesWith(root, func(string) (io.ReadCloser, error) { return failingReader, nil })
	if failingReader.closes != 1 || got.fs == nil || got.fs.Root() != baseline.fs.Root() || got.nativeReason != baseline.nativeReason {
		t.Fatalf("close changed valid filesystem/read capability: %+v closes=%d", got, failingReader.closes)
	}
	if got.diagnosis != "objects/info/alternates close failed" || strings.Contains(got.diagnosis, "canary") {
		t.Fatalf("unbounded/missing close diagnosis: %q", got.diagnosis)
	}
	for name, data := range map[string]string{"prior-path-error": "relative-unsupported\n" + payload, "scanner-error": strings.Repeat("x", bufio.MaxScanTokenSize+1)} {
		t.Run(name, func(t *testing.T) {
			r := &alternateCloseRead{Reader: strings.NewReader(data), closeErr: errors.New("private close canary")}
			observed := readAlternatesWith(root, func(string) (io.ReadCloser, error) { return r, nil })
			if r.closes != 1 || !strings.Contains(observed.diagnosis, "objects/info/alternates close failed") {
				t.Fatalf("close was lost: %+v closes=%d", observed, r.closes)
			}
			if name == "scanner-error" && !strings.Contains(observed.diagnosis, "could not be read to the end") {
				t.Fatalf("scanner bound/read error disappeared: %q", observed.diagnosis)
			}
			if name == "prior-path-error" && (!strings.Contains(observed.diagnosis, "relative entry") || observed.nativeReason == "" || observed.fs == nil) {
				t.Fatalf("prior path diagnosis/capability/fs changed: %+v", observed)
			}
		})
	}
}
