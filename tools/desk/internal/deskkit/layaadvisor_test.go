package deskkit

import (
	"context"
	"encoding/json"
	"os"
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
	p.RequestedBackend = req.Backend
	p.ActualBackend = "cpu"
	json.NewEncoder(os.Stdout).Encode(layaResponse{Prediction: p, FallbackReason: "device unavailable"})
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
		if _, err := a.Predict(context.Background(), Consultation{Detail: detail}); err == nil {
			t.Fatalf("accepted %s", detail)
		}
	}
	a.MaxBytes = 1
	if _, err := a.Predict(context.Background(), Consultation{Context: "overflow"}); err == nil {
		t.Fatal("overflow accepted")
	}
}
