package deskkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// LayaAdvisor is optional and never installed in default routing. Approval is
// supplied by the operator, never by the subprocess. No ambient credentials pass.
type LayaAdvisor struct {
	Command          []string
	Request          AssessmentRequest
	Approved         bool
	Backend          string
	AllowCPUFallback bool
	MaxBytes         int
	Timeout          time.Duration
}
type layaRequest struct {
	Request          AssessmentRequest `json:"request"`
	Consultation     Consultation      `json:"consultation"`
	Backend          string            `json:"backend"`
	AllowCPUFallback bool              `json:"allow_cpu_fallback"`
}
type layaDiagnostics struct {
	LatencySeconds float64  `json:"latency_seconds"`
	Omissions      []string `json:"omissions"`
	Precision      string   `json:"precision"`
	MemoryBytes    *int64   `json:"memory_bytes"`
}
type layaResponse struct {
	Diagnostics    *layaDiagnostics `json:"diagnostics,omitempty"`
	Prediction     Prediction       `json:"prediction"`
	FallbackReason string           `json:"fallback_reason"`
}
type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *boundedBuffer) Len() int      { return b.buffer.Len() }
func (b *boundedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, fmt.Errorf("laya: output byte limit")
	}
	return b.buffer.Write(p)
}
func (a LayaAdvisor) Predict(ctx context.Context, c Consultation) (Prediction, error) {
	fail := ConservativePrediction(a.Request)
	if !a.Approved {
		return fail, Refused("laya: activation requires owner approval")
	}
	if len(a.Command) == 0 || (a.Backend != "cpu" && a.Backend != "cuda") {
		return fail, Refused("laya: invalid process or backend")
	}
	limit := a.MaxBytes
	if limit <= 0 {
		limit = 64 * 1024
	}
	if len(c.Context)+len(c.Detail)+len(c.Prompt) > limit {
		return fail, Refused("laya: input byte limit")
	}
	input, err := json.Marshal(layaRequest{a.Request, c, a.Backend, a.AllowCPUFallback})
	if err != nil || len(input) > limit {
		return fail, Refused("laya: request byte limit")
	}
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.Command[0], a.Command[1:]...)
	// Bound inherited output-pipe completion too: CommandContext alone only
	// kills the direct process. Fail closed if pipes outlive process exit.
	cmd.WaitDelay = time.Millisecond
	cmd.Env = []string{"HF_HUB_OFFLINE=1", "TRANSFORMERS_OFFLINE=1", "KUBECONFIG=/dev/null"}
	cmd.Stdin = bytes.NewReader(input)
	out := &boundedBuffer{limit: limit}
	diagnostic := &boundedBuffer{limit: 4096}
	cmd.Stdout = out
	cmd.Stderr = diagnostic
	if err = cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fail, ctx.Err()
		}
		return fail, fmt.Errorf("laya: process failed: %w", err)
	}
	return a.decode(ctx, out.Bytes())
}

// decode shares the completion deadline; valid data never revives an expired call.
func (a LayaAdvisor) decode(ctx context.Context, output []byte) (Prediction, error) {
	fail := ConservativePrediction(a.Request)
	if err := layaDeadline(ctx); err != nil {
		return fail, err
	}
	var err error
	var response layaResponse
	dec := json.NewDecoder(bytes.NewReader(output))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&response); err != nil {
		return fail, fmt.Errorf("laya: malformed response: %w", err)
	}
	if dec.Decode(new(any)) != io.EOF {
		return fail, Refused("laya: trailing output")
	}
	p := response.Prediction
	if p.RequestedBackend != a.Backend {
		return fail, Refused("laya: requested backend mismatch")
	}
	if p.ActualBackend != a.Backend {
		if !(a.Backend == "cuda" && p.ActualBackend == "cpu" && a.AllowCPUFallback && response.FallbackReason != "") {
			return fail, Refused("laya: unauthorized fallback")
		}
	}
	if err = ValidatePrediction(a.Request, p); err != nil {
		return fail, err
	}
	// No calibration has been approved; local upstream action/confidence is shadow-only.
	if !p.Abstained || len(p.LabelProbabilities) != 0 {
		return fail, Refused("laya: no approved calibration")
	}
	if err := layaDeadline(ctx); err != nil {
		return fail, err
	}
	return p, nil
}
func (a LayaAdvisor) Advise(ctx context.Context, c Consultation) (Advice, error) {
	return (PredictionAdvisor{Request: a.Request, Predict: a.Predict}).Advise(ctx, c)
}

func layaDeadline(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}
