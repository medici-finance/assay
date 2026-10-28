package deskkit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The fixture hangs far past the envelope, so a call that returns inside the envelope
// proves the deadline killed it; the margin keeps runner scheduling out of the result.
const (
	layaHang     = 10 * time.Second
	layaEnvelope = 3 * time.Second
)

// Reexec this test binary, not Python or any model, across the production seam.
func TestLayaProcess(t *testing.T) {
	if os.Getenv("HF_HUB_OFFLINE") != "1" {
		return
	}
	var req layaRequest
	if json.NewDecoder(os.Stdin).Decode(&req) != nil {
		os.Exit(2)
	}
	if req.Consultation.Detail == "timeout" {
		time.Sleep(layaHang)
		os.Exit(0)
	}
	if req.Consultation.Detail == "malformed" {
		os.Stdout.WriteString("not json")
		os.Exit(0)
	}
	if req.Consultation.Detail == "env" {
		// The child must see GPUs hidden and nothing inherited from the caller.
		if v, ok := os.LookupEnv("CUDA_VISIBLE_DEVICES"); !ok || v != "" || os.Getenv("LAYA_PARENT_MARKER") != "" {
			os.Exit(6)
		}
	}
	p := ConservativePrediction(req.Request)
	p.ShadowLabels = []string{"hold"}
	if strings.HasPrefix(req.Consultation.Detail, "pipes-") {
		exe, _ := os.Executable()
		child := exec.Command(exe, "-test.run=^TestLayaPipeHold$")
		child.Env = os.Environ()
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if child.Start() != nil {
			os.Exit(3)
		}
	}
	if req.Consultation.Detail == "non-abstaining" {
		p.Abstained = false
	}
	if req.Consultation.Detail == "stdout-over" {
		p.ProviderVersion = strings.Repeat("x", 2048)
	}
	if req.Consultation.Detail == "stderr-under" {
		os.Stderr.WriteString(strings.Repeat("x", 1024))
	}
	if req.Consultation.Detail == "stderr-over" {
		os.Stderr.WriteString(strings.Repeat("x", 4097))
	}
	// An honest child echoes the request; the Detail cases report a device switch.
	p.RequestedBackend, p.ActualBackend = req.Backend, req.Backend
	reason := ""
	switch req.Consultation.Detail {
	case "actual-cuda":
		p.ActualBackend = "cuda"
	case "actual-mps":
		p.ActualBackend = "mps"
	case "requested-other":
		p.RequestedBackend = "cuda"
	case "fallback-reason":
		reason = "device unavailable"
	}
	json.NewEncoder(os.Stdout).Encode(layaResponse{Prediction: p, FallbackReason: reason})
	if req.Consultation.Detail == "pipes-error" {
		os.Exit(4)
	}
	os.Exit(0)
}
func layaFixture(t *testing.T) LayaAdvisor {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return LayaAdvisor{Command: []string{exe, "-test.run=^TestLayaProcess$"}, Request: AssessmentRequest{Subject: "example-org/item", InputDigest: "input", SchemaDigest: "schema", Vocabulary: []string{"hold"}}, Approved: true, Backend: "cpu", Timeout: time.Second}
}
func TestLayaAdvisorRoundTrip(t *testing.T) {
	a := layaFixture(t)
	p, err := a.Predict(context.Background(), Consultation{Vocabulary: []string{"hold"}})
	if err != nil || !p.Abstained || p.ActualBackend != "cpu" {
		t.Fatalf("prediction=%+v err=%v", p, err)
	}
	advice, err := a.Advise(context.Background(), Consultation{})
	if err != nil || advice.Answer != "" {
		t.Fatalf("shadow advice escaped: %+v %v", advice, err)
	}
}

// layaRefused reports a typed boundary refusal, never a process failure or crash.
func layaRefused(err error) bool {
	var de *DeskError
	return errors.As(err, &de) && de.Code == ExitRefused
}

// Per the driver's ruling no device fallback is permitted: the child cannot report a
// non-CPU actual backend, a different requested backend, or a fallback reason.
func TestLayaAdvisorUnauthorizedFallback(t *testing.T) {
	a := layaFixture(t)
	if _, err := a.Predict(context.Background(), Consultation{}); err != nil {
		t.Fatalf("valid cpu control: %v", err)
	}
	for _, detail := range []string{"actual-cuda", "actual-mps", "requested-other", "fallback-reason"} {
		if _, err := a.Predict(context.Background(), Consultation{Detail: detail}); !layaRefused(err) {
			t.Errorf("%s accepted or not refused: %v", detail, err)
		}
	}
}

// The response check is a distinct lower layer: with the pre-launch backend check
// bypassed (the advisor itself names a non-CPU backend), a matching non-CPU record is
// still refused when decoded, whichever of the two backend fields carries it.
func TestLayaAdvisorDecodeCPUOnly(t *testing.T) {
	a := layaFixture(t)
	for _, backend := range []string{"cuda", "mps", "CPU", ""} {
		a.Backend = backend
		for _, fields := range [][2]string{{backend, backend}, {backend, "cpu"}, {"cpu", backend}} {
			p := ConservativePrediction(a.Request)
			p.RequestedBackend, p.ActualBackend = fields[0], fields[1]
			data, err := json.Marshal(layaResponse{Prediction: p})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.decode(context.Background(), data); !layaRefused(err) {
				t.Errorf("record requested=%q actual=%q accepted by decode: %v", fields[0], fields[1], err)
			}
		}
	}
}

// Per the driver's ruling the provider is CPU only. Every other value, including case
// and whitespace variants, an empty value, CUDA and MPS, is refused before a process
// starts. The command path does not exist, so a launch attempt would surface as a
// process failure rather than the refusal; the cpu control proves that it would.
func TestLayaAdvisorCPUOnly(t *testing.T) {
	a := layaFixture(t)
	a.Command = []string{filepath.Join(t.TempDir(), "never-launched")}
	if _, err := a.Predict(context.Background(), Consultation{}); err == nil || !strings.Contains(err.Error(), "process failed") {
		t.Fatalf("control: the cpu backend must reach launch: %v", err)
	}
	for _, backend := range []string{"", "cuda", "mps", "CPU", "Cpu", " cpu", "cpu ", "cpu\n", "cpu,cuda", "cuda:0", "auto", "rocm"} {
		a.Backend = backend
		_, err := a.Predict(context.Background(), Consultation{})
		if !layaRefused(err) || strings.Contains(err.Error(), "process failed") {
			t.Errorf("backend %q not refused before launch: %v", backend, err)
		}
	}
	a.Backend, a.Approved = "cpu", false
	if _, err := a.Predict(context.Background(), Consultation{}); !layaRefused(err) || strings.Contains(err.Error(), "process failed") {
		t.Errorf("unapproved call not refused before launch: %v", err)
	}
}

// The child sees CUDA devices hidden and inherits nothing from the caller, even when
// the caller's own environment selects a GPU.
func TestLayaAdvisorChildEnvironment(t *testing.T) {
	t.Setenv("CUDA_VISIBLE_DEVICES", "0")
	t.Setenv("LAYA_PARENT_MARKER", "inherited")
	a := layaFixture(t)
	if _, err := a.Predict(context.Background(), Consultation{Detail: "env"}); err != nil {
		t.Fatalf("child environment not fixed: %v", err)
	}
}

// Per the driver's ruling the provider stays disabled. No production file in this
// module other than its own may name LayaAdvisor, so nothing can construct or route to it.
func TestLayaAdvisorNotWired(t *testing.T) {
	root := filepath.Join("..", "..")
	own := filepath.Join(root, "internal", "deskkit", "layaadvisor.go")
	scanned := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || filepath.Clean(path) == own {
			return nil
		}
		scanned++
		src, err := os.ReadFile(path)
		if err != nil || !bytes.Contains(src, []byte("LayaAdvisor")) {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "LayaAdvisor" {
				t.Errorf("%s names LayaAdvisor; the provider must stay unwired", path)
			}
			return true
		})
		return nil
	})
	if err != nil || scanned == 0 {
		t.Fatalf("scan incomplete: %d files, %v", scanned, err)
	}
}
func TestLayaAdvisorLimits(t *testing.T) {
	a := layaFixture(t)
	a.Approved = false
	if _, err := a.Predict(context.Background(), Consultation{}); err == nil {
		t.Fatal("unapproved activation")
	}
	a.Approved = true
	for _, detail := range []string{"malformed", "timeout"} {
		a.Timeout = 50 * time.Millisecond
		start := time.Now()
		if _, err := a.Predict(context.Background(), Consultation{Detail: detail}); err == nil {
			t.Fatalf("accepted %s", detail)
		}
		if time.Since(start) > layaEnvelope {
			t.Fatalf("%s exceeded deadline envelope", detail)
		}
	}
	a.MaxBytes = 1
	if _, err := a.Predict(context.Background(), Consultation{Context: "overflow"}); err == nil {
		t.Fatal("overflow accepted")
	}
}

func TestLayaPipeHold(t *testing.T) {
	if os.Getenv("HF_HUB_OFFLINE") == "1" {
		time.Sleep(layaHang)
		os.Exit(0)
	}
}
func TestLayaAdvisorAbstention(t *testing.T) {
	a := layaFixture(t)
	for _, detail := range []string{"", "non-abstaining"} {
		p, err := a.Predict(context.Background(), Consultation{Detail: detail})
		if detail == "" {
			if err != nil || !p.Abstained {
				t.Fatalf("valid control: %+v %v", p, err)
			}
		} else if err == nil {
			t.Fatal("non-abstaining record accepted")
		}
	}
}
func TestLayaAdvisorPipeDeadline(t *testing.T) {
	a := layaFixture(t)
	a.Timeout = 100 * time.Millisecond
	for _, detail := range []string{"pipes-error", "pipes-valid"} {
		start := time.Now()
		_, err := a.Predict(context.Background(), Consultation{Detail: detail})
		if err == nil {
			t.Errorf("expired %s accepted", detail)
		}
		if time.Since(start) > layaEnvelope {
			t.Errorf("%s exceeded completion deadline", detail)
		}
	}
}
func TestLayaAdvisorOutputLimits(t *testing.T) {
	a := layaFixture(t)
	a.MaxBytes = 1024
	for _, detail := range []string{"", "stdout-over", "stderr-under", "stderr-over"} {
		_, err := a.Predict(context.Background(), Consultation{Detail: detail})
		over := strings.HasSuffix(detail, "over")
		if over && err == nil {
			t.Errorf("accepted %s", detail)
		}
		if !over && err != nil {
			t.Errorf("valid %s: %v", detail, err)
		}
	}
}

// Force io.Copy to select destination ReaderFrom if exposed, as os/exec does.
type layaReader struct{ io.Reader }

func TestLayaBufferCopyPaths(t *testing.T) {
	for _, size := range []int{16, 64} {
		for _, writerTo := range []bool{false, true} {
			b := &boundedBuffer{limit: 16}
			var r io.Reader = strings.NewReader(strings.Repeat("x", size))
			if !writerTo {
				r = layaReader{r}
			}
			_, err := io.Copy(b, r)
			if size <= 16 {
				if err != nil || b.Len() != size {
					t.Fatalf("valid copy: %v %d", err, b.Len())
				}
			} else if err == nil || b.Len() > 16 {
				t.Fatalf("over-limit copy: %v %d", err, b.Len())
			}
		}
	}
	typ := reflect.TypeOf(&boundedBuffer{})
	allowed := map[string]bool{"Write": true, "Len": true, "Bytes": true}
	for i := 0; i < typ.NumMethod(); i++ {
		if !allowed[typ.Method(i).Name] {
			t.Errorf("unchecked buffer method: %s", typ.Method(i).Name)
		}
	}
}

func TestLayaAdvisorExpiredResult(t *testing.T) {
	a := layaFixture(t)
	p := ConservativePrediction(a.Request)
	p.RequestedBackend, p.ActualBackend = "cpu", "cpu"
	data, err := json.Marshal(layaResponse{Prediction: p})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.decode(context.Background(), data); err != nil {
		t.Fatalf("valid data: %v", err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := a.decode(ctx, data); err == nil {
		t.Fatal("expired valid result accepted")
	}
}

// Expire specifically between entry and completion validation, without timer races.
type layaEndContext struct {
	context.Context
	calls int
}

func (c *layaEndContext) Err() error {
	c.calls++
	if c.calls > 1 {
		return context.DeadlineExceeded
	}
	return nil
}
func TestLayaAdvisorEndDeadline(t *testing.T) {
	a := layaFixture(t)
	p := ConservativePrediction(a.Request)
	p.RequestedBackend, p.ActualBackend = "cpu", "cpu"
	data, _ := json.Marshal(layaResponse{Prediction: p})
	ctx := &layaEndContext{Context: context.Background()}
	if _, err := a.decode(ctx, data); err == nil {
		t.Fatal("completion-expired result accepted")
	}
}
