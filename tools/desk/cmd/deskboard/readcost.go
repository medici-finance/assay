package main

// readcost.go — what one board sweep spends against the forge's read quota, and the
// conditional-request cache that keeps an unchanged board cheap (#2421).
//
// The problem it answers: an App installation's REST quota is 5000 calls an hour, and a
// watcher sweeps `actions` every few minutes. Before this, every sweep re-read every open
// PR in full and nothing was reused between sweeps, so the per-sweep cost grew with the
// number of open PRs and a busy board exhausted the quota.
//
// Three parts, all installed ONLY in this binary (deskkit.SetGitHubTransport in init — no
// other tool's transport changes):
//
//  1. boardTransport — every request a resolved GitHub backend sends passes through it.
//     A GET whose last 200 carried an ETag or Last-Modified is re-sent with If-None-Match /
//     If-Modified-Since; on a 304 the cached body is served back to the caller as the 200 it
//     was. GitHub does not charge an authorized 304 against the core quota. The cached body
//     is served ONLY after the forge itself has answered 304 for this exact validator — there
//     is no time-to-live and no "probably unchanged": a cache entry never answers a read the
//     forge did not just confirm. That makes the cache safe for every read, including the
//     check-state reads a stale "green" would come from.
//
//  2. sweepMeter — counts what the sweep sent: requests, how many came back 304, how many
//     per-PR reads were skipped (prfacts.go), by endpoint class, and the lowest
//     X-RateLimit-Remaining seen per rate-limit resource with its reset time. `actions`
//     reports it as Header.cost in the JSON and as one `deskboard: cost …` line on stderr.
//
//  3. the refuse-floor — while an `actions` sweep is running, a response reporting the
//     core or graphql quota below the floor (--rate-floor / DESKBOARD_RATE_FLOOR, default
//     500) stops the sweep: no further request is sent, and the verb exits 4 (rate-limited)
//     naming the reset time instead of printing a board. A sweep that stopped part-way never
//     prints, because a board missing the PRs it could not read would look complete.
//
// A cache file that is missing, unreadable, corrupt, or whose checksum or URL does not match
// is a miss — the request goes out unconditionally, exactly as if there were no cache. Writes
// are a temp file in the same directory plus a rename, so a concurrent deskboard run reads
// either the old whole file or the new whole file, never a torn one.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	// defaultRateFloor is the remaining-quota level below which an `actions` sweep stops.
	// 500 is 10% of an App installation's hourly REST quota: enough headroom for the
	// writes a verdict or a flip still needs after the board is read.
	defaultRateFloor = 500
	// rateFloorEnv overrides the default; --rate-floor overrides both. 0 disables the floor.
	rateFloorEnv = "DESKBOARD_RATE_FLOOR"
	// cacheBodyMax caps one cached response body. A larger body (a huge diff) is passed
	// through uncached rather than held on disk.
	cacheBodyMax = 4 << 20
	// cacheSchema versions the on-disk entry shape; any other value is a miss.
	cacheSchema = 1
	// cacheMaxAge prunes entries not used for this long (merged PRs' reads).
	cacheMaxAge = 7 * 24 * time.Hour
)

// rateFloor is the floor the next `actions` sweep enforces; run() sets it from the flag,
// the env var or the default on every invocation.
var rateFloor = defaultRateFloor

// boardCacheDir resolves the cache root: <desk-tools state dir>/deskboard-cache, beside the
// --delta snapshots. An unresolvable state dir disables the cache (every read a miss); it is
// never an error. A package var so a test can point it elsewhere.
var boardCacheDir = func() (string, error) {
	dir, err := deskkit.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "deskboard-cache"), nil
}

func init() {
	deskkit.SetGitHubTransport(func(base http.RoundTripper) http.RoundTripper {
		return &boardTransport{base: base}
	})
}

// ---------------------------------------------------------------------------
// meter
// ---------------------------------------------------------------------------

// classCost is one endpoint class's share of a sweep.
type classCost struct {
	Requests    int `json:"requests"`
	NotModified int `json:"notModified"`
}

// rateReading is the lowest remaining quota one rate-limit resource reported this sweep.
type rateReading struct {
	Resource  string `json:"resource"`
	Remaining int    `json:"remaining"`
	Limit     int    `json:"limit,omitempty"`
	Reset     string `json:"reset,omitempty"` // RFC3339 UTC; "" when the forge sent none
	reset     time.Time
}

// sweepCost is Header.cost on `actions`: what this sweep spent against the forge.
//
// Charged is Requests minus NotModified — the calls the quota was billed for (GraphQL
// calls bill the graphql resource, REST calls the core one; ByClass separates them).
// Skipped counts per-PR typed reads that were not sent at all because the PR's head, last
// update and check state matched the previous sweep (prfacts.go). Refused counts requests
// the floor stopped before they were sent. RateLimit is the lowest remaining quota each
// resource reported, with its reset time; FloorBreach names the resource that crossed the
// floor, and is set only when the sweep was stopped.
type sweepCost struct {
	Requests    int                  `json:"requests"`
	Charged     int                  `json:"charged"`
	NotModified int                  `json:"notModified"`
	Skipped     int                  `json:"skipped"`
	Refused     int                  `json:"refused,omitempty"`
	ByClass     map[string]classCost `json:"byClass"`
	RateLimit   []rateReading        `json:"rateLimit,omitempty"`
	Floor       int                  `json:"floor"`
	FloorBreach *rateReading         `json:"floorBreach,omitempty"`
}

// sweepMeter accumulates one sweep's cost. Requests arrive from the concurrent per-repo and
// per-PR workers, so every method takes the lock.
type sweepMeter struct {
	mu       sync.Mutex
	cost     sweepCost
	readings map[string]rateReading
	armed    bool // the floor is enforced only while an actions sweep runs
	breach   *rateReading
}

var boardMeter = &sweepMeter{}

// begin resets the meter and arms the floor for one sweep.
func (m *sweepMeter) begin(floor int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cost = sweepCost{ByClass: map[string]classCost{}, Floor: floor}
	m.readings = map[string]rateReading{}
	m.breach = nil
	m.armed = floor > 0
}

// end disarms the floor; the counts stay readable until the next begin.
func (m *sweepMeter) end() {
	m.mu.Lock()
	m.armed = false
	m.mu.Unlock()
}

func (m *sweepMeter) count(class string, notModified bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cost.ByClass == nil {
		m.cost.ByClass = map[string]classCost{}
	}
	c := m.cost.ByClass[class]
	c.Requests++
	m.cost.Requests++
	if notModified {
		c.NotModified++
		m.cost.NotModified++
	}
	m.cost.ByClass[class] = c
}

func (m *sweepMeter) skip(n int) {
	m.mu.Lock()
	m.cost.Skipped += n
	m.mu.Unlock()
}

// refusal returns the error a request gets once the floor is breached, counting it.
func (m *sweepMeter) refusal() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.armed || m.breach == nil {
		return nil
	}
	m.cost.Refused++
	return fmt.Errorf("deskboard: request not sent — %s quota is below the --rate-floor (%d < %d)",
		m.breach.Resource, m.breach.Remaining, m.cost.Floor)
}

// observe records a response's X-RateLimit-* headers and trips the floor when the core or
// graphql resource reports less remaining than the floor.
func (m *sweepMeter) observe(h http.Header) {
	rem, err := strconv.Atoi(strings.TrimSpace(h.Get("X-RateLimit-Remaining")))
	if err != nil {
		return
	}
	r := rateReading{Resource: strings.TrimSpace(h.Get("X-RateLimit-Resource")), Remaining: rem}
	if r.Resource == "" {
		r.Resource = "core"
	}
	if lim, lerr := strconv.Atoi(strings.TrimSpace(h.Get("X-RateLimit-Limit"))); lerr == nil {
		r.Limit = lim
	}
	if sec, serr := strconv.ParseInt(strings.TrimSpace(h.Get("X-RateLimit-Reset")), 10, 64); serr == nil && sec > 0 {
		r.reset = time.Unix(sec, 0).UTC()
		r.Reset = r.reset.Format(time.RFC3339)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readings == nil {
		m.readings = map[string]rateReading{}
	}
	if prev, ok := m.readings[r.Resource]; !ok || r.Remaining < prev.Remaining {
		m.readings[r.Resource] = r
	}
	if m.armed && m.breach == nil && m.cost.Floor > 0 && r.Remaining < m.cost.Floor &&
		(r.Resource == "core" || r.Resource == "graphql") {
		b := r
		m.breach = &b
	}
}

// snapshot returns a copy of the sweep's cost so far.
func (m *sweepMeter) snapshot() sweepCost {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.cost
	c.Charged = c.Requests - c.NotModified
	c.ByClass = make(map[string]classCost, len(m.cost.ByClass))
	for k, v := range m.cost.ByClass {
		c.ByClass[k] = v
	}
	c.RateLimit = nil
	for _, r := range m.readings {
		c.RateLimit = append(c.RateLimit, r)
	}
	sort.Slice(c.RateLimit, func(i, j int) bool { return c.RateLimit[i].Resource < c.RateLimit[j].Resource })
	if m.breach != nil {
		b := *m.breach
		c.FloorBreach = &b
	}
	return c
}

// line renders the one-line stderr summary.
func (c sweepCost) line() string {
	var b strings.Builder
	fmt.Fprintf(&b, "deskboard: cost requests=%d charged=%d notModified=%d skipped=%d",
		c.Requests, c.Charged, c.NotModified, c.Skipped)
	if c.Refused > 0 {
		fmt.Fprintf(&b, " refused=%d", c.Refused)
	}
	classes := make([]string, 0, len(c.ByClass))
	for k := range c.ByClass {
		classes = append(classes, k)
	}
	sort.Strings(classes)
	parts := make([]string, 0, len(classes))
	for _, k := range classes {
		v := c.ByClass[k]
		parts = append(parts, fmt.Sprintf("%s:%d/%d", k, v.Requests, v.NotModified))
	}
	fmt.Fprintf(&b, " byClass[requests/304]=%s", strings.Join(parts, ","))
	for _, r := range c.RateLimit {
		reset := r.Reset
		if reset == "" {
			reset = "unknown"
		}
		fmt.Fprintf(&b, " %s.remaining=%d reset=%s", r.Resource, r.Remaining, reset)
	}
	fmt.Fprintf(&b, " floor=%d", c.Floor)
	return b.String()
}

// floorError is the exit-4 refusal for a sweep the floor stopped. It names the resource,
// the reading, and the reset time, and carries the wait until reset as RetryAfter.
func (c sweepCost) floorError(now time.Time) error {
	b := c.FloorBreach
	reset := "an unknown time (the forge sent no X-RateLimit-Reset)"
	var wait time.Duration
	if b.Reset != "" {
		reset = b.Reset
		if t, err := time.Parse(time.RFC3339, b.Reset); err == nil && t.After(now) {
			wait = t.Sub(now)
		}
	}
	return deskkit.RateLimitedAfter(fmt.Sprintf(
		"deskboard: actions sweep STOPPED — the forge reported %d %s requests remaining, below the "+
			"refuse-floor of %d (--rate-floor / %s). No board is printed: a sweep that stopped "+
			"part-way would read as complete. The quota resets at %s; re-run after that.",
		b.Remaining, b.Resource, c.Floor, rateFloorEnv, reset), wait)
}

// resolveRateFloor applies --rate-floor over DESKBOARD_RATE_FLOOR over the default.
func resolveRateFloor(flagVal string) (int, error) {
	src, v := "--rate-floor", flagVal
	if v == "" {
		src, v = rateFloorEnv, strings.TrimSpace(os.Getenv(rateFloorEnv))
	}
	if v == "" {
		return defaultRateFloor, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, deskkit.Refused(fmt.Sprintf("deskboard: %s: invalid value %q: must be a non-negative integer (0 disables the floor)", src, v))
	}
	return n, nil
}

// endpointClass names the endpoint family a request path belongs to, for ByClass:
// "graphql", "repo", "pulls", "pulls.files", "commits", "commits.check-runs", "issues.events", …
func endpointClass(path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i, s := range segs {
		if s == "graphql" {
			return "graphql"
		}
		if s == "repos" && len(segs) >= i+3 {
			rest := segs[i+3:]
			if len(rest) == 0 {
				return "repo"
			}
			class := rest[0]
			// pulls/<n>/files, issues/<n>/events, commits/<sha>/check-runs, …: the
			// family plus the sub-resource; a bare id adds nothing.
			if len(rest) >= 3 {
				class += "." + rest[2]
			}
			return class
		}
	}
	if len(segs) > 0 && segs[0] != "" {
		return segs[0]
	}
	return "other"
}

// ---------------------------------------------------------------------------
// conditional-request transport + cache
// ---------------------------------------------------------------------------

type boardTransport struct{ base http.RoundTripper }

// cacheEntry is one cached 200: the validators the forge sent, the headers a replay needs
// (Content-Type, Link for paging), and the body with its checksum.
type cacheEntry struct {
	Schema       int               `json:"schema"`
	URL          string            `json:"url"`
	Accept       string            `json:"accept"`
	ETag         string            `json:"etag,omitempty"`
	LastModified string            `json:"lastModified,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Body         []byte            `json:"body"`
	BodySHA256   string            `json:"bodySha256"`
}

// replayHeaders are the response headers a cached 200 is replayed with.
var replayHeaders = []string{"Content-Type", "Link", "ETag", "Last-Modified"}

func (t *boardTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := boardMeter.refusal(); err != nil {
		return nil, err
	}
	class := endpointClass(req.URL.Path)
	cacheable := req.Method == http.MethodGet && req.Header.Get("If-None-Match") == "" &&
		req.Header.Get("If-Modified-Since") == "" && req.Header.Get("Range") == ""
	var (
		path  string
		entry *cacheEntry
	)
	if cacheable {
		path = cachePath(req)
		entry = loadCacheEntry(path, req)
	}
	out := req
	if entry != nil {
		out = req.Clone(req.Context())
		if entry.ETag != "" {
			out.Header.Set("If-None-Match", entry.ETag)
		}
		if entry.LastModified != "" {
			out.Header.Set("If-Modified-Since", entry.LastModified)
		}
	}
	resp, err := t.base.RoundTrip(out)
	if err != nil {
		boardMeter.count(class, false)
		return nil, err
	}
	boardMeter.observe(resp.Header)
	if resp.StatusCode == http.StatusNotModified && entry != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		boardMeter.count(class, true)
		touch(path)
		return entry.replay(req, resp), nil
	}
	boardMeter.count(class, false)
	if !cacheable || resp.StatusCode != http.StatusOK {
		return resp, nil
	}
	etag, lastMod := resp.Header.Get("ETag"), resp.Header.Get("Last-Modified")
	if etag == "" && lastMod == "" {
		if entry != nil {
			_ = os.Remove(path) // the old validator no longer describes this resource
		}
		return resp, nil
	}
	// Read the body to cache it, then hand the caller an identical body: what was read,
	// then whatever remains (an oversized body), then the original read error (a body cut
	// off mid-transfer must still fail the caller exactly as it would have).
	buf, rerr := io.ReadAll(io.LimitReader(resp.Body, cacheBodyMax+1))
	switch {
	case rerr != nil:
		resp.Body = readCloser{io.MultiReader(bytes.NewReader(buf), errReader{rerr}), resp.Body}
	case len(buf) > cacheBodyMax:
		resp.Body = readCloser{io.MultiReader(bytes.NewReader(buf), resp.Body), resp.Body}
	default:
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(buf))
		storeCacheEntry(path, req, resp.Header, buf)
	}
	return resp, nil
}

type readCloser struct {
	io.Reader
	io.Closer
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

// replay rebuilds the cached 200 for req, carrying the 304's own rate-limit and date
// headers so the caller sees the current quota, not the cached one.
func (e *cacheEntry) replay(req *http.Request, notModified *http.Response) *http.Response {
	h := http.Header{}
	for k, v := range e.Headers {
		h.Set(k, v)
	}
	for k, vs := range notModified.Header {
		if strings.HasPrefix(http.CanonicalHeaderKey(k), "X-Ratelimit-") || k == "Date" {
			h[k] = append([]string(nil), vs...)
		}
	}
	return &http.Response{
		Status: "200 OK", StatusCode: http.StatusOK,
		Proto: notModified.Proto, ProtoMajor: notModified.ProtoMajor, ProtoMinor: notModified.ProtoMinor,
		Header: h, Body: io.NopCloser(bytes.NewReader(e.Body)), ContentLength: int64(len(e.Body)),
		Request: req,
	}
}

// cachePath is the entry file for a request: one directory per repo (owner__name, or
// "_" for a request outside /repos/), one file per URL+Accept digest.
func cachePath(req *http.Request) string {
	root, err := boardCacheDir()
	if err != nil || root == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(req.URL.String() + "\x00" + req.Header.Get("Accept")))
	return filepath.Join(root, "http", repoDirOf(req.URL.Path), hex.EncodeToString(sum[:])+".json")
}

func repoDirOf(path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i, s := range segs {
		if s == "repos" && len(segs) >= i+3 && segs[i+1] != "" && segs[i+2] != "" {
			return safeName(segs[i+1]) + "__" + safeName(segs[i+2])
		}
	}
	return "_"
}

// safeName keeps a path segment to a filename-safe alphabet.
func safeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// loadCacheEntry returns the entry for req, or nil for ANY problem: missing, unreadable,
// unparseable, another schema, a different URL or Accept, no validator, or a body that does
// not match its checksum. A miss sends the request unconditionally.
func loadCacheEntry(path string, req *http.Request) *cacheEntry {
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var e cacheEntry
	if json.Unmarshal(raw, &e) != nil {
		return nil
	}
	if e.Schema != cacheSchema || e.URL != req.URL.String() || e.Accept != req.Header.Get("Accept") ||
		(e.ETag == "" && e.LastModified == "") {
		return nil
	}
	sum := sha256.Sum256(e.Body)
	if hex.EncodeToString(sum[:]) != e.BodySHA256 {
		return nil
	}
	return &e
}

func storeCacheEntry(path string, req *http.Request, h http.Header, body []byte) {
	if path == "" {
		return
	}
	sum := sha256.Sum256(body)
	e := cacheEntry{
		Schema: cacheSchema, URL: req.URL.String(), Accept: req.Header.Get("Accept"),
		ETag: h.Get("ETag"), LastModified: h.Get("Last-Modified"),
		Headers: map[string]string{}, Body: body, BodySHA256: hex.EncodeToString(sum[:]),
	}
	for _, k := range replayHeaders {
		if v := h.Get(k); v != "" {
			e.Headers[k] = v
		}
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return
	}
	_ = writeFileAtomic(path, raw)
}

// writeFileAtomic writes data to path via a uniquely named temp file in the same directory
// and a rename, so a concurrent reader sees the old whole file or the new whole file.
// Concurrent writers are last-writer-wins on the whole file.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(name)
		return errors.Join(werr, cerr)
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

func touch(path string) {
	if path != "" {
		now := time.Now()
		_ = os.Chtimes(path, now, now)
	}
}

// pruneBoardCache removes cache files not used for cacheMaxAge. Best effort: a failure
// leaves files behind, which only costs disk.
func pruneBoardCache(now time.Time) {
	root, err := boardCacheDir()
	if err != nil || root == "" {
		return
	}
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, werr error) error {
		if werr != nil || d.IsDir() {
			return nil
		}
		if info, ierr := d.Info(); ierr == nil && now.Sub(info.ModTime()) > cacheMaxAge {
			_ = os.Remove(p)
		}
		return nil
	})
}
