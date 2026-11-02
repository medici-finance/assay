package main

// CI reads share a finite API budget. A rate-limit refusal is neither a clean
// check nor a permission failure. Retry only reads, with the same credential,
// after the provider's reset; never retry an ordinary auth/permission denial.
import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// One wait budget for the entire process, shared by HTTP and CLI reads. A job
// can cross one hourly quota reset, not wait forever as each endpoint fails.
const forgeReadMaxWait = 65 * time.Minute

var ciForgeReadWait = &forgeReadWait{left: forgeReadMaxWait}

type forgeReadWait struct {
	mu   sync.Mutex
	left time.Duration
}

func (b *forgeReadWait) reserve(d time.Duration) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if d <= 0 || d > b.left {
		return false
	}
	b.left -= d
	return true
}

// A Retry-After or an exhausted primary quota is positive evidence of rate
// limiting. A bare 403 (including permissions failures) never authorizes retry.
func forgeReadDelay(status int, h http.Header, now time.Time) time.Duration {
	if status != 403 && status != 429 {
		return 0
	}
	if raw := h.Get("Retry-After"); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 32); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
		if at, err := http.ParseTime(raw); err == nil && at.After(now) {
			d := at.Sub(now)
			if d > forgeReadMaxWait-time.Second {
				return 0
			}
			return d + time.Second
		}
		return 0
	}
	if h.Get("X-RateLimit-Remaining") != "0" {
		return 0
	}
	reset, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64)
	if err != nil || reset <= 0 {
		return 0
	}
	d := time.Unix(reset, 0).Sub(now)
	if d > forgeReadMaxWait-time.Second {
		return 0
	}
	d += time.Second
	if d <= 0 {
		return time.Second
	}
	return d
}

func waitForgeRead(ctx context.Context, d time.Duration) error {
	fmt.Fprintf(os.Stderr, "GitHub read rate-limited; retrying the same read in %s (CI wait budget applies)\n", d.Round(time.Second))
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type forgeReadDoer struct {
	inner  httpDoer
	budget *forgeReadWait
	now    func() time.Time
	wait   func(context.Context, time.Duration) error
}

func (d *forgeReadDoer) Do(req *http.Request) (*http.Response, error) {
	resp, err := d.inner.Do(req)
	if err != nil || req.Method != http.MethodGet {
		return resp, err
	}
	delay := forgeReadDelay(resp.StatusCode, resp.Header, d.now())
	if !d.budget.reserve(delay) {
		return resp, nil
	}
	// Close the refused response before waiting; retry only once. A second
	// failure goes to the caller unchanged and keeps its could-not-check state.
	resp.Body.Close()
	if err := d.wait(req.Context(), delay); err != nil {
		return nil, err
	}
	return d.inner.Do(req.Clone(req.Context()))
}

func ciForgeReadDoer(inner httpDoer) httpDoer {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return inner
	}
	return &forgeReadDoer{inner: inner, budget: ciForgeReadWait, now: time.Now, wait: waitForgeRead}
}

// corroboration's CLI reads use the same wait budget. gh hides headers unless
// --include is requested (which would change its JSON contract), so after a
// positive rate-limit error ask the same credential's uncharged /rate_limit
// endpoint for the read's REST or GraphQL reset. No token is minted or substituted here.
// makeCmd must construct a fresh, guarded read command for each attempt.
func forgeCLIRead(resource string, makeCmd func() *exec.Cmd) ([]byte, error) {
	return forgeCLIReadWith(resource, makeCmd, ciForgeReadWait, time.Now, waitForgeRead)
}

func forgeCLIReadWith(resource string, makeCmd func() *exec.Cmd, budget *forgeReadWait, now func() time.Time, wait func(context.Context, time.Duration) error) ([]byte, error) {
	cmd := makeCmd()
	if os.Getenv("GITHUB_ACTIONS") == "true" && cmd.Env == nil {
		return nil, fmt.Errorf("CI forge read requires a guarded credential environment")
	}
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	detail := forgeCLIFailure(cmd, err)
	if os.Getenv("GITHUB_ACTIONS") != "true" || !strings.Contains(strings.ToLower(detail), "rate limit") {
		return out, fmt.Errorf("%w: %s", err, detail)
	}
	host := os.Getenv("GH_HOST")
	if host != "" && host != "github.com" {
		return out, fmt.Errorf("%w: %s", err, detail)
	}
	probe := ghGuard(exec.Command("gh", "api", "rate_limit"))
	probe.Env, probe.Dir = cmd.Env, cmd.Dir
	raw, probeErr := probe.Output()
	if probeErr != nil {
		return out, fmt.Errorf("%w: %s (quota reset could not be read)", err, detail)
	}
	var limits struct {
		Resources map[string]struct {
			Remaining *int
			Reset     int64
		}
	}
	if resource != "core" && resource != "graphql" {
		return out, fmt.Errorf("%w: %s (unknown quota resource)", err, detail)
	}
	if json.Unmarshal(raw, &limits) != nil {
		return out, fmt.Errorf("%w: %s (quota response could not be parsed)", err, detail)
	}
	quota, ok := limits.Resources[resource]
	if !ok || quota.Remaining == nil || *quota.Remaining != 0 || quota.Reset <= 0 {
		return out, fmt.Errorf("%w: %s (no exhausted %s quota observed)", err, detail, resource)
	}
	h := http.Header{}
	h.Set("X-RateLimit-Remaining", "0")
	h.Set("X-RateLimit-Reset", strconv.FormatInt(quota.Reset, 10))
	delay := forgeReadDelay(403, h, now())
	if !budget.reserve(delay) {
		return out, fmt.Errorf("%w: %s (CI wait budget exhausted)", err, detail)
	}
	if waitErr := wait(context.Background(), delay); waitErr != nil {
		return nil, waitErr
	}
	retry := makeCmd()
	retry.Env = append([]string(nil), cmd.Env...)
	retry.Dir = cmd.Dir
	out, err = retry.Output()
	if err != nil {
		return out, fmt.Errorf("%w: %s", err, forgeCLIFailure(retry, err))
	}
	return out, nil
}

// Keep the provider's actual denial, which Output otherwise discards. Strip
// explicitly supplied credential values and bound output before diagnostics.
func forgeCLIFailure(cmd *exec.Cmd, err error) string {
	var raw string
	if e, ok := err.(*exec.ExitError); ok {
		raw = string(e.Stderr)
	}
	env := cmd.Env
	if env == nil {
		// Outside CI the guard retains implicit inheritance. Read only the CLI's
		// credential variables for redaction; never construct a child environment here.
		for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN", "CELLCTL_GH_AMBIENT"} {
			env = append(env, key+"="+os.Getenv(key))
		}
	}
	for _, kv := range env {
		key, value, ok := strings.Cut(kv, "=")
		if ok && value != "" && (strings.Contains(key, "TOKEN") || strings.Contains(key, "SECRET") || strings.Contains(key, "PASSWORD")) {
			raw = strings.ReplaceAll(raw, value, "[redacted]")
		}
	}
	raw = strings.TrimSpace(raw)
	if len(raw) > 4096 {
		raw = raw[:4096] + " [truncated]"
	}
	if raw == "" {
		return "forge read failed without provider diagnostics"
	}
	return raw
}
