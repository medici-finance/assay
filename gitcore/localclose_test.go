package gitcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var errLocalRead = errors.New("primary local read")
var errLocalInputClose = errors.New("local input close")
var errLocalOutputClose = errors.New("local output close")

type localCloseReader struct {
	io.Reader
	readErr, closeErr error
	closes, bytesRead int
	events            *[]string
}

func (r *localCloseReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.bytesRead += n
	if r.readErr != nil {
		return n, r.readErr
	}
	return n, err
}
func (r *localCloseReader) Close() error {
	r.closes++
	if r.events != nil {
		*r.events = append(*r.events, "input-close")
	}
	return r.closeErr
}

type localCloseWriter struct {
	bytes.Buffer
	closeErr error
	closes   int
	events   *[]string
}

func (w *localCloseWriter) Close() error {
	w.closes++
	*w.events = append(*w.events, "output-close")
	return w.closeErr
}

type localCloseObject struct {
	plumbing.EncodedObject
	reader  io.ReadCloser
	openErr error
}

func (o localCloseObject) Reader() (io.ReadCloser, error) { return o.reader, o.openErr }
func localCloseFile(t *testing.T, reader io.ReadCloser, openErr error) *object.File {
	t.Helper()
	obj := &plumbing.MemoryObject{}
	obj.SetType(plumbing.BlobObject)
	blob, err := object.DecodeBlob(localCloseObject{EncodedObject: obj, reader: reader, openErr: openErr})
	if err != nil {
		t.Fatal(err)
	}
	return object.NewFile("leaf", 0, blob)
}

// regression: #2395
func TestLocalMaterialiseClose(t *testing.T) {
	for _, c := range []struct {
		name                                        string
		openErr, readErr, inputErr, outputErr, want error
	}{
		{name: "success"},
		{name: "reader-open-primary", openErr: errLocalRead, outputErr: errLocalOutputClose, want: errLocalRead},
		{name: "copy-primary", readErr: errLocalRead, inputErr: errLocalInputClose, outputErr: errLocalOutputClose, want: errLocalRead},
		{name: "output-close", outputErr: errLocalOutputClose, want: errLocalOutputClose},
		{name: "input-close", inputErr: errLocalInputClose, want: errLocalInputClose},
		{name: "both-close-output-precedence", inputErr: errLocalInputClose, outputErr: errLocalOutputClose, want: errLocalOutputClose},
	} {
		t.Run(c.name, func(t *testing.T) {
			events := []string{}
			rd := &localCloseReader{Reader: strings.NewReader("body"), readErr: c.readErr, closeErr: c.inputErr, events: &events}
			out := &localCloseWriter{closeErr: c.outputErr, events: &events}
			err := writeTreeFileContents("leaf", localCloseFile(t, rd, c.openErr), out)
			if c.want == nil {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				context := "write"
				if c.openErr != nil {
					context = "read"
				}
				want := fmt.Sprintf("gitcore: materialise: %s leaf: %v", context, c.want)
				if !errors.Is(err, c.want) || err.Error() != want {
					t.Fatalf("primary/close outcome changed: %v, want %s", err, want)
				}
				for _, secondary := range []error{errLocalInputClose, errLocalOutputClose} {
					if secondary != c.want && errors.Is(err, secondary) {
						t.Fatalf("secondary replaced/joined primary: %v", err)
					}
				}
			}
			wantEvents := []string{"input-close", "output-close"}
			wantInput := 1
			if c.openErr != nil {
				wantEvents = []string{"output-close"}
				wantInput = 0
			}
			if rd.closes != wantInput || out.closes != 1 || !reflect.DeepEqual(events, wantEvents) {
				t.Fatalf("close ownership/order changed: rd=%d out=%d %v", rd.closes, out.closes, events)
			}
			if c.openErr == nil && out.String() != "body" {
				t.Fatalf("copy output changed: %q", out.String())
			}
		})
	}
}

// regression: #2395
func TestBoundedBlobClose(t *testing.T) {
	for _, c := range []struct {
		name, data        string
		limit             int64
		readErr, closeErr error
		wantData          string
		wantLarge         bool
		wantErr           error
	}{
		{name: "success", data: "body", limit: 8, wantData: "body"},
		{name: "primary-plus-close", data: "body", limit: 8, readErr: errLocalRead, closeErr: errLocalInputClose, wantData: "body", wantErr: errLocalRead},
		{name: "sole-close", data: "body", limit: 8, closeErr: errLocalInputClose, wantData: "body", wantErr: errLocalInputClose},
		{name: "oversized-plus-read-and-close", data: strings.Repeat("x", 4096), limit: 3, readErr: errLocalRead, closeErr: errLocalInputClose, wantLarge: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			rd := &localCloseReader{Reader: strings.NewReader(c.data), readErr: c.readErr, closeErr: c.closeErr}
			raw, large, err := readBoundedBlob(rd, c.limit)
			if raw != c.wantData || large != c.wantLarge || (c.wantErr == nil) != (err == nil) || (c.wantErr != nil && !errors.Is(err, c.wantErr)) {
				t.Fatalf("tuple=(%q,%v,%v), want (%q,%v,%v)", raw, large, err, c.wantData, c.wantLarge, c.wantErr)
			}
			if c.wantErr == errLocalRead && err != errLocalRead {
				t.Fatalf("primary read error identity changed: %v", err)
			}
			if rd.closes != 1 || rd.bytesRead > int(c.limit+1) {
				t.Fatalf("close/bound changed: closes=%d bytes=%d limit=%d", rd.closes, rd.bytesRead, c.limit)
			}
		})
	}
}

// regression: #2395
func TestBareConfigClose(t *testing.T) {
	for _, c := range []struct {
		name, text        string
		readErr, closeErr error
		wantBare          bool
		wantClose         bool
	}{
		{name: "success", text: "[core]\n bare = true\n", wantBare: true},
		{name: "sole-close", text: "[core]\n bare = true\n", closeErr: errLocalInputClose, wantClose: true},
		{name: "decode-primary-plus-close", text: "[unterminated", closeErr: errLocalInputClose},
		{name: "read-primary-plus-close", readErr: errLocalRead, closeErr: errLocalInputClose},
	} {
		t.Run(c.name, func(t *testing.T) {
			rd := &localCloseReader{Reader: strings.NewReader(c.text), readErr: c.readErr, closeErr: c.closeErr}
			bare, err := readBareConfig("config", rd)
			if bare != c.wantBare || rd.closes != 1 {
				t.Fatalf("config tuple/close changed: %v %v closes=%d", bare, err, rd.closes)
			}
			if c.wantClose {
				if !errors.Is(err, errLocalInputClose) || err.Error() != "gitcore: close config: local input close" {
					t.Fatalf("sole config close lost: %v", err)
				}
				return
			}
			control := &localCloseReader{Reader: strings.NewReader(c.text), readErr: c.readErr}
			wantBare, wantErr := readBareConfig("config", control)
			if bare != wantBare || (err == nil) != (wantErr == nil) || reflect.TypeOf(err) != reflect.TypeOf(wantErr) || (err != nil && err.Error() != wantErr.Error()) {
				t.Fatalf("primary decode/read error changed: (%v,%T %v) vs (%v,%T %v)", bare, err, err, wantBare, wantErr, wantErr)
			}
			if errors.Is(err, errLocalInputClose) {
				t.Fatalf("cleanup replaced primary: %v", err)
			}
		})
	}
}
