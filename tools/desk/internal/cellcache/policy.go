// Package cellcache owns opt-in, per-cell Go cache storage. It never adopts an
// existing unmarked cache or clears a root while a managed consumer is active.
package cellcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const EnvPolicy = "ASSAY_GO_CACHE_POLICY"
const schema = "cell-go-cache-v1"

type Policy struct {
	Root   string `json:"root"`
	Domain string `json:"domain"`
	Budget int64  `json:"budget_bytes"`
	Floor  uint64 `json:"minimum_free_bytes"`
	// BuildMaxAge is the age, in seconds, past which a compiler-cache entry
	// may be trimmed even while consumers hold records (see trimBuild). Zero
	// is a policy serialized before the field existed: it means the default,
	// and omitempty keeps such a policy's serialization byte-identical, so a
	// consumer launched by an older cellctl still matches its own policy.
	BuildMaxAge int64 `json:"build_max_age_seconds,omitempty"`
}

// DefaultBuildMaxAge is the compiled build-cache trim age.
const DefaultBuildMaxAge = 24 * time.Hour

// MinBuildMaxAge keeps every trim cutoff above the Go toolchain's own use
// stamp: cmd/go refreshes an entry's mtime when it reads one whose mtime is
// more than an hour old, so an entry a running build has used is never older
// than about an hour. Two hours leaves that refresh a full hour of margin.
const MinBuildMaxAge = 2 * time.Hour

// BuildAge is the effective trim age: the recorded one, or the default for a
// policy recorded before the field existed.
func (p Policy) BuildAge() time.Duration {
	if p.BuildMaxAge == 0 {
		return DefaultBuildMaxAge
	}
	return time.Duration(p.BuildMaxAge) * time.Second
}

// Resolve uses cell.env's existing resolver, before HOME changes. Ambient Go
// variables are deliberately not overrides. An override must name a new private
// root or a root already marked for this exact canonical cell directory.
func Resolve(cell string, get func(string) string) (*Policy, error) {
	mode := get("CELL_GO_CACHE")
	if mode == "" || mode == "off" {
		return nil, nil
	}
	if mode != "on" {
		return nil, fmt.Errorf("CELL_GO_CACHE must be on or off")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return nil, fmt.Errorf("managed Go cache requires macOS or Linux; this platform is unsupported")
	}
	canonical, err := filepath.EvalSymlinks(cell)
	if err != nil {
		return nil, err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return nil, err
	}
	p := &Policy{Root: filepath.Join(canonical, "go-cache"), Domain: DomainFor(canonical), Budget: 8 << 30, Floor: 10 << 30}
	if v := get("CELL_GO_CACHE_ROOT"); v != "" {
		p.Root = v
	}
	if v := get("CELL_GO_CACHE_BYTES"); v != "" {
		p.Budget, err = strconv.ParseInt(v, 10, 64)
		if err != nil || p.Budget <= 0 {
			return nil, fmt.Errorf("CELL_GO_CACHE_BYTES must be a positive byte count")
		}
	}
	if v := get("CELL_GO_CACHE_MIN_FREE"); v != "" {
		p.Floor, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("CELL_GO_CACHE_MIN_FREE must be a nonnegative byte count")
		}
	}
	p.BuildMaxAge = int64(DefaultBuildMaxAge / time.Second)
	if v := get("CELL_GO_CACHE_BUILD_MAX_AGE"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < MinBuildMaxAge || d%time.Second != 0 {
			return nil, fmt.Errorf("CELL_GO_CACHE_BUILD_MAX_AGE must be a whole-second duration of at least %s", MinBuildMaxAge)
		}
		p.BuildMaxAge = int64(d / time.Second)
	}
	return p, p.Validate()
}

func (p Policy) Validate() error {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return fmt.Errorf("managed Go cache requires macOS or Linux")
	}
	if !filepath.IsAbs(p.Root) || filepath.Clean(p.Root) != p.Root || filepath.Dir(p.Root) == p.Root {
		return fmt.Errorf("cache root must be a clean absolute directory")
	}
	b, err := hex.DecodeString(p.Domain)
	if err != nil || len(b) != sha256.Size || p.Budget <= 0 {
		return fmt.Errorf("invalid cache domain or budget")
	}
	// Zero is the pre-field default; anything else must clear Go's use stamp.
	// The upper bound keeps the seconds-to-Duration conversion from overflowing.
	if p.BuildMaxAge != 0 && (p.BuildMaxAge < int64(MinBuildMaxAge/time.Second) || p.BuildMaxAge > int64(math.MaxInt64/int64(time.Second))) {
		return fmt.Errorf("build cache trim age must be at least %s", MinBuildMaxAge)
	}
	return validatePath(p.Root)
}
func (p Policy) Env() []string {
	b, _ := json.Marshal(p)
	return []string{EnvPolicy + "=" + string(b), "GOCACHE=" + filepath.Join(p.Root, "build"), "GOMODCACHE=" + filepath.Join(p.Root, "mod"), "GOPATH=" + filepath.Join(p.Root, "gopath")}
}
func FromEnv(env []string) (*Policy, error) {
	value := ""
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if ok && k == EnvPolicy {
			value = v
		}
	}
	if value == "" {
		return nil, nil
	}
	var p Policy
	if err := json.Unmarshal([]byte(value), &p); err != nil {
		return nil, fmt.Errorf("invalid managed cache policy: %w", err)
	}
	return &p, p.Validate()
}

// Deferred is a retryable storage hold, never an empty queue. No claim or work
// is created by Check. Retry the same dispatch after space returns; the existing
// dispatch claim remains the duplicate-dispatch authority.
type Deferred struct{ Report Report }

func (d *Deferred) Error() string {
	return fmt.Sprintf("storage-deferred: %s; inspect cellctl cache <cell> status; free space or wait for consumers, then retry the same item (nothing claimed)", d.Report.Reason)
}
