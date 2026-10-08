// Package cellcache owns opt-in, per-cell Go cache storage. It never adopts an
// existing unmarked cache or clears a root while a managed consumer is active.
package cellcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const EnvPolicy = "ASSAY_GO_CACHE_POLICY"
const schema = "cell-go-cache-v1"

type Policy struct {
	Root   string `json:"root"`
	Domain string `json:"domain"`
	Budget int64  `json:"budget_bytes"`
	Floor  uint64 `json:"minimum_free_bytes"`
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
	sum := sha256.Sum256([]byte(canonical))
	p := &Policy{Root: filepath.Join(canonical, "go-cache"), Domain: hex.EncodeToString(sum[:]), Budget: 8 << 30, Floor: 10 << 30}
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
