package workspace

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// replaceWire changes one fixture's string token without decoding it first.
func replaceWire(t *testing.T, raw []byte, field string, token []byte) []byte {
	t.Helper()
	key := []byte(`"` + field + `":"`)
	start := bytes.Index(raw, key)
	if start < 0 {
		t.Fatalf("fixture has no %s string", field)
	}
	start += len(key) - 1
	end := bytes.IndexByte(raw[start+1:], '"') + start + 2
	if end <= start+1 {
		t.Fatal("fixture string is unterminated")
	}
	return append(append(append([]byte(nil), raw[:start]...), token...), raw[end:]...)
}

// regression: #3150
func TestWireUnicode(t *testing.T) {
	for _, name := range []string{"invalid-utf8", "lone-high", "lone-low", "utf8-surrogate"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", "wire", name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = DecodeRequest(raw)
			wantReason(t, err, ReasonMalformed)
		})
	}
	for _, pair := range [][2]string{
		{"replacement", `"/srv/\ufffd"`},
		{"pair", `"/srv/\ud83d\ude80"`},
		{"highest-pair", `"/srv/\uDBFF\uDFFF"`},
		{"escaped-backslash", `"/srv/\\ud800"`},
	} {
		t.Run(pair[0], func(t *testing.T) {
			raw := replaceWire(t, readFixture(t, "request-worker.json"), "root", []byte(pair[1]))
			q, err := DecodeRequest(raw)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := canonical(q)
			if err != nil {
				t.Fatal(err)
			}
			back, err := DecodeRequest(encoded)
			if err != nil || back.Cell.Root != q.Cell.Root || back.Digest() != q.Digest() {
				t.Fatalf("valid Unicode changed on canonical round trip: %v", err)
			}
			if pair[0] == "replacement" {
				literal := replaceWire(t, readFixture(t, "request-worker.json"), "root", []byte("\"/srv/�\""))
				other, err := DecodeRequest(literal)
				if err != nil || other.Cell.Root != "/srv/�" || other.Digest() != q.Digest() {
					t.Fatalf("valid U+FFFD was not preserved: %v", err)
				}
			}
		})
	}
	// A high surrogate must be immediately followed by a low-surrogate escape.
	for _, value := range []string{`"/x\ud800a"`, `"/x\ud800\ud800"`, `"/x\udc00\ud800"`, `"/x\ud800\\udc00"`, `"/x\ud800\u0041"`} {
		_, err := DecodeRequest(replaceWire(t, readFixture(t, "request-worker.json"), "root", []byte(value)))
		wantReason(t, err, ReasonMalformed)
	}
}

// regression: #3150
func TestWireOtherDecoders(t *testing.T) {
	for _, doc := range []string{"record-worker.json", "result-ok.json"} {
		for _, field := range []string{"root", "logical", "physical", "common_dir", "admin_dir"} {
			t.Run(doc+"/"+field, func(t *testing.T) {
				// A second public consumer and a different nested path from the reported request.
				raw := replaceWire(t, readFixture(t, doc), field, []byte(`"/second/\udfff"`))
				var err error
				if doc == "record-worker.json" {
					_, err = DecodeRecord(raw)
				} else {
					_, err = DecodeResult(raw)
				}
				wantReason(t, err, ReasonMalformed)
			})
		}
	}
}

// regression: #3150
func TestPathByteLimit(t *testing.T) {
	for _, n := range []int{4096, 4097} {
		path := "/" + strings.Repeat("é", 2047) + strings.Repeat("x", n-4095)
		if len(path) != n {
			t.Fatal("wrong byte boundary")
		}
		token, err := json.Marshal(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, doc := range []string{"request-worker.json", "record-worker.json", "result-ok.json"} {
			fields := []string{"root"}
			if doc != "request-worker.json" {
				fields = append(fields, "logical", "physical", "common_dir", "admin_dir")
			}
			for _, field := range fields {
				raw := replaceWire(t, readFixture(t, doc), field, token)
				switch doc {
				case "request-worker.json":
					_, err = DecodeRequest(raw)
				case "record-worker.json":
					_, err = DecodeRecord(raw)
				default:
					_, err = DecodeResult(raw)
				}
				if n == 4096 && err != nil {
					t.Errorf("%s/%s: 4096-byte path refused: %v", doc, field, err)
				}
				if n == 4097 {
					wantReason(t, err, ReasonInvalid)
				}
			}
		}
	}
}
