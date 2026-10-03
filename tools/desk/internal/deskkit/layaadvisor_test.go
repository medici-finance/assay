package deskkit

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
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
		time.Sleep(time.Second)
		os.Exit(0)
	}
	if req.Consultation.Detail == "malformed" {
		os.Stdout.WriteString("not json")
		os.Exit(0)
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
	p.RequestedBackend = req.Backend
	p.ActualBackend = "cpu"
	json.NewEncoder(os.Stdout).Encode(layaResponse{Prediction: p, FallbackReason: "device unavailable"})
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
func TestLayaAdvisorUnauthorizedFallback(t *testing.T) {
	a := layaFixture(t)
	a.Backend = "cuda"
	if _, err := a.Predict(context.Background(), Consultation{}); err == nil {
		t.Fatal("unauthorized fallback accepted")
	}
	a.AllowCPUFallback = true
	if _, err := a.Predict(context.Background(), Consultation{}); err != nil {
		t.Fatal(err)
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
		if time.Since(start) > 500*time.Millisecond {
			t.Fatal("process exceeded deadline envelope")
		}
	}
	a.MaxBytes = 1
	if _, err := a.Predict(context.Background(), Consultation{Context: "overflow"}); err == nil {
		t.Fatal("overflow accepted")
	}
}

func TestLayaPipeHold(t *testing.T) {
	if os.Getenv("HF_HUB_OFFLINE") == "1" {
		time.Sleep(time.Second)
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
		if time.Since(start) > 500*time.Millisecond {
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
