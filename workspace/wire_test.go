package workspace

import (
	"bytes"
	"encoding/hex"
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
	for _, name := range []string{"invalid-utf8.json.hex", "lone-high.json", "lone-low.json", "utf8-surrogate.json.hex"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", "wire", name))
			if err != nil {
				t.Fatal(err)
			}
			// .hex explicitly declares space-separated wire bytes, decoded before
			// the real production decoder sees them. No Unicode decoding occurs.
			if strings.HasSuffix(name, ".hex") {
				raw, err = hex.DecodeString(strings.Join(strings.Fields(string(raw)), ""))
				if err != nil {
					t.Fatal(err)
				}
			}
			_, err = DecodeRequest(raw)
			wantReason(t, err, ReasonMalformed)
		})
	}
	for _, pair := range [][2]string{
		{"replacement", `"/srv/\ufffd"`},
		{"pair", `"/srv/\ud83d\ude80"`},
		{"highest-pair", `"/srv/\uDBFF\uDFFF"`},
		{"lowest-pair", `"/srv/\ud800\udc00"`},
		{"below-surrogates", `"/srv/\ud7ff"`},
		{"above-surrogates", `"/srv/\ue000"`},
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
	// A high surrogate must be immediately followed by a low-surrogate escape. From
	// "\\dc00" on, each value pins one checkWire condition no earlier value reaches:
	// a backslash not starting \u, a low surrogate first, a second escape above
	// 0xdfff, a high surrogate followed by no backslash at all, and a second escape
	// just below 0xdc00. The positives above pin the range edges from the other side.
	for _, value := range []string{`"/x\ud800a"`, `"/x\ud800\ud800"`, `"/x\udc00\ud800"`, `"/x\ud800\\udc00"`, `"/x\ud800\u0041"`,
		`"/x\ud800\\dc00"`, `"/x\udc00\udc00"`, `"/x\ud800\ue000"`, `"/x\ud800xudc00"`, `"/x\ud800\udbff"`} {
		_, err := DecodeRequest(replaceWire(t, readFixture(t, "request-worker.json"), "root", []byte(value)))
		wantReason(t, err, ReasonMalformed)
	}
}

// regression: #3150
// The lexer indexes past each backslash on the strength of json.Valid alone, so a
// truncated escape must refuse as malformed, never panic, in every decoder.
func TestWireTruncated(t *testing.T) {
	for _, doc := range []string{`{"schema":"\`, `{"schema":"\u12`, `{"schema":"\ud800\`, `\`} {
		for _, dec := range []func([]byte) error{
			func(b []byte) error { _, err := DecodeRequest(b); return err },
			func(b []byte) error { _, err := DecodeRecord(b); return err },
			func(b []byte) error { _, err := DecodeResult(b); return err },
		} {
			wantReason(t, dec([]byte(doc)), ReasonMalformed)
		}
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
