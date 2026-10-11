package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// countingReader serves n bytes and records how many were pulled from it.
type countingReader struct {
	left, read int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > c.left {
		n = c.left
	}
	for i := range p[:n] {
		p[i] = 'x'
	}
	c.left -= n
	c.read += n
	return int(n), nil
}

// TestReadBounded: a stream over the bound is refused, and no more than
// limit+1 bytes are pulled from it — the bound applies while reading, not
// after the whole response has been buffered.
func TestReadBounded(t *testing.T) {
	const limit = 1024

	t.Run("at the bound reads", func(t *testing.T) {
		b, err := readBounded(bytes.NewReader(bytes.Repeat([]byte("x"), limit)), limit)
		if err != nil || len(b) != limit {
			t.Fatalf("got %d bytes, err %v; want %d bytes, no error", len(b), err, limit)
		}
	})

	t.Run("over the bound refuses without buffering it", func(t *testing.T) {
		r := &countingReader{left: 64 * limit}
		b, err := readBounded(r, limit)
		if err == nil {
			t.Fatalf("SECURITY: a %d-byte stream passed a %d-byte bound", 64*limit, limit)
		}
		if b != nil {
			t.Errorf("refusal still returned %d bytes", len(b))
		}
		if !strings.Contains(err.Error(), "1024-byte bound") {
			t.Errorf("refusal %q does not name the bound", err)
		}
		if r.read > limit+1 {
			t.Errorf("pulled %d bytes from an oversized stream, want at most %d", r.read, limit+1)
		}
	})
}

// TestHTTPFetchBundleBounded: the production bundle downloader applies
// maxBundleBytes to the HTTP response itself.
func TestHTTPFetchBundleBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/big" {
			_, _ = w.Write(bytes.Repeat([]byte("x"), maxBundleBytes+10))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	if b, err := httpFetchBundle(srv.URL + "/small"); err != nil || string(b) != `{"ok":true}` {
		t.Fatalf("small bundle: got %q, %v", b, err)
	}
	b, err := httpFetchBundle(srv.URL + "/big")
	if err == nil {
		t.Fatalf("SECURITY: a %d-byte bundle response was accepted (bound %d)", len(b), maxBundleBytes)
	}
	if !strings.Contains(err.Error(), "-byte bound") {
		t.Errorf("refusal %q does not name the bound", err)
	}
}

// TestUnboundedReadSitesAllowList is the class guard for unbounded reads of a
// network response. Every io.ReadAll in this package's non-test source is
// listed here by file and count: readBounded's one (behind io.LimitReader) and
// httpGet's asset branch (unbounded on purpose: the sha256 pin decides those
// bytes). A new io.ReadAll anywhere in the package fails this test until a
// reviewer adds it to the list, which is the moment to ask whether it reads
// something a remote party controls.
func TestUnboundedReadSitesAllowList(t *testing.T) {
	allowed := map[string]int{"install.go": 2}

	got := map[string]int{}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(string(src), "io.ReadAll("); n > 0 {
			got[f] = n
		}
	}
	for f, n := range got {
		if allowed[f] != n {
			t.Errorf("%s has %d io.ReadAll call(s), allow-list says %d — read a network response through readBounded, or add the site here with a reason", f, n, allowed[f])
		}
	}
	for f, n := range allowed {
		if got[f] != n {
			t.Errorf("allow-list expects %d io.ReadAll call(s) in %s, found %d — the guard is stale", n, f, got[f])
		}
	}
}
