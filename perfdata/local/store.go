// Package local is the bounded, segmented, append-only local store of
// performance-event-v1 records under a caller-resolved state directory:
//
//	<stateDir>/performance-events/
//	  manifest.json        store layout and schema versions
//	  source.json          random source identity, epoch and last reserved sequence
//	  aliases-e<N>.json    private alias map of epoch N (keyed digests, never raw locators)
//	  coverage.json        retirements, failed-write ranges, unclean ends, epoch resets
//	  lock                 the writer lock (flock, bounded wait)
//	  run/                 one live marker per open writer
//	  segments/            seg-<firstSeq>-e<epoch>-<YYYYMMDD>.jsonl, one canonical event per line
//	  distributions/       day-<YYYY-MM-DD>.json rollups (only with distribution retention)
//
// The tree is distinct from audit.jsonl, dispatch-records.jsonl and perf/. Directories
// are 0700 and files 0600; metadata is replaced atomically by rename. Appends never
// fsync on the work path, so a crash can lose the tail of the last write: the reader
// reports that as a partial tail or an unexplained gap, never as complete coverage.
package local

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/perfdata"
)

// DirName is the store directory under the caller's state directory.
const DirName = "performance-events"

// StoreSchema identifies the on-disk layout.
const StoreSchema = "performance-store-v1"

// ErrOff is returned by Open when collection is not armed; nothing was created.
var ErrOff = errors.New("perfdata/local: collection is off")

var errLockTimeout = errors.New("perfdata/local: store lock wait exceeded")

// Root returns the store directory of a state directory.
func Root(stateDir string) string { return filepath.Join(stateDir, DirName) }

// Options are writer options. Now and the unexported fault hook exist for tests.
type Options struct {
	Build    perfdata.Build
	Now      func() time.Time
	LockWait time.Duration

	// fault, when set, is called before each effect ("append", "meta", "segment")
	// and its error replaces the effect's result. Tests use it for full disk and slow I/O.
	fault func(op string) error
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now().UTC()
	}
	return time.Now().UTC()
}

// Store is one writer handle. It is safe for use by one goroutine at a time per
// method; Write serializes internally. Several Stores (and processes) may share a tree.
type Store struct {
	root    string
	policy  perfdata.Policy
	profile perfdata.Profile
	opts    Options

	mu       sync.Mutex
	liveMu   sync.Mutex
	lockF    *os.File
	liveF    *os.File
	livePath string
	aliases  *aliasFile
	aliasSt  fileStamp
}

type sourceFile struct {
	Schema        string `json:"schema"`
	Source        string `json:"source"`
	Epoch         uint32 `json:"epoch"`
	LastSeq       uint64 `json:"lastSeq"`
	AliasesMinted bool   `json:"aliasesMinted"`
}

type lostRange struct {
	Source string            `json:"source"`
	Range  perfdata.SeqRange `json:"range"`
}

type coverageFile struct {
	Schema      string                `json:"schema"`
	Retired     []retired             `json:"retired"`
	Lost        []lostRange           `json:"lost"`
	Drops       perfdata.Drops        `json:"drops"`
	Unclean     int                   `json:"unclean"`
	EpochResets []perfdata.EpochReset `json:"epochResets"`
}

type retired struct {
	Source string `json:"source,omitempty"`
	perfdata.Retirement
}

type manifest struct {
	Schema        string `json:"schema"`
	EventSchema   string `json:"eventSchema"`
	BatchSchema   string `json:"batchSchema"`
	RollupSchema  string `json:"rollupSchema"`
	SegmentFormat string `json:"segmentFormat"`
}

const (
	sourceSchema   = "performance-source-v1"
	coverageSchema = "performance-coverage-v1"
	aliasSchema    = "performance-aliases-v1"
)

// Open arms a writer on stateDir. It creates nothing and returns ErrOff unless c is
// armed; the policy must admit the consented profile. Open recovers source identity,
// detects writers that ended uncleanly and applies retirement.
func Open(stateDir string, c perfdata.Consent, policy perfdata.Policy, o Options) (*Store, error) {
	if !c.Armed {
		return nil, ErrOff
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if !policy.Admits(c.Profile) {
		return nil, fmt.Errorf("perfdata/local: profile %q not admitted by policy %q", c.Profile, policy.Revision)
	}
	if policy.SegmentBytes == 0 {
		policy.SegmentBytes = perfdata.DefaultSegmentBytes
	}
	if o.LockWait == 0 {
		o.LockWait = 2 * time.Second
	}
	if o.Build.Version == "" {
		o.Build = perfdata.Build{Version: "dev", OS: "unknown", Arch: "unknown"}
	}
	s := &Store{root: Root(stateDir), policy: policy, profile: c.Profile, opts: o}
	for _, d := range []string{s.root, filepath.Join(s.root, "segments"), filepath.Join(s.root, "run"), filepath.Join(s.root, "distributions")} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, err
		}
		_ = os.Chmod(d, 0o700)
	}
	lf, err := os.OpenFile(filepath.Join(s.root, "lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	s.lockF = lf
	if err := s.openLive(); err != nil {
		lf.Close()
		return nil, err
	}
	err = s.locked(func() error {
		if err := s.ensureManifest(); err != nil {
			return err
		}
		cov := s.loadCoverage()
		s.reapLive(&cov)
		if _, err := s.loadSource(&cov, true); err != nil {
			return err
		}
		if err := s.retire(&cov); err != nil {
			return err
		}
		return s.saveCoverage(cov)
	})
	if err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// Profile is the store's consented profile.
func (s *Store) Profile() perfdata.Profile { return s.profile }

// Policy is the store's selected policy.
func (s *Store) Policy() perfdata.Policy { return s.policy }

// Close releases the writer and removes its live marker (a clean end).
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.liveMu.Lock()
	if s.liveF != nil {
		unlockFile(s.liveF)
		s.liveF.Close()
		_ = os.Remove(s.livePath)
		s.liveF = nil
	}
	s.liveMu.Unlock()
	if s.lockF != nil {
		s.lockF.Close()
		s.lockF = nil
	}
	return nil
}

// abandon releases the live marker WITHOUT removing it, so the next writer reports
// an unclean end. Used when a flush budget expires.
func (s *Store) abandon() {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	if s.liveF != nil {
		unlockFile(s.liveF)
		s.liveF.Close()
		s.liveF = nil
	}
}

func (s *Store) fault(op string) error {
	if s.opts.fault != nil {
		return s.opts.fault(op)
	}
	return nil
}

func (s *Store) locked(fn func() error) error {
	if err := lockFile(s.lockF, s.opts.LockWait); err != nil {
		return err
	}
	defer unlockFile(s.lockF)
	return fn()
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Store) openLive() error {
	s.livePath = filepath.Join(s.root, "run", "live-"+randomHex(8)+".lock")
	f, err := os.OpenFile(s.livePath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if liveMarkersSupported && !tryLockFile(f) {
		f.Close()
		return errors.New("perfdata/local: cannot hold live marker")
	}
	s.liveF = f
	return nil
}

// reapLive counts and removes live markers whose writer is gone (unclean ends).
func (s *Store) reapLive(cov *coverageFile) {
	if !liveMarkersSupported {
		return
	}
	ents, _ := os.ReadDir(filepath.Join(s.root, "run"))
	for _, e := range ents {
		p := filepath.Join(s.root, "run", e.Name())
		if p == s.livePath || !strings.HasPrefix(e.Name(), "live-") {
			continue
		}
		f, err := os.OpenFile(p, os.O_RDWR, 0)
		if err != nil {
			continue
		}
		if tryLockFile(f) {
			cov.Unclean++
			_ = os.Remove(p)
			unlockFile(f)
		}
		f.Close()
	}
}

// writeAtomic replaces path with data via a same-directory temporary file and rename.
func (s *Store) writeAtomic(path string, data []byte) error {
	if err := s.fault("meta"); err != nil {
		return err
	}
	tmp := path + ".tmp-" + randomHex(6)
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func (s *Store) writeJSON(name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return s.writeAtomic(filepath.Join(s.root, name), append(b, '\n'))
}

func (s *Store) ensureManifest() error {
	p := filepath.Join(s.root, "manifest.json")
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	return s.writeJSON("manifest.json", manifest{
		Schema: StoreSchema, EventSchema: perfdata.EventSchema, BatchSchema: perfdata.BatchSchema,
		RollupSchema: perfdata.RollupSchema, SegmentFormat: "jsonl-canonical",
	})
}

func readCoverage(root string) (coverageFile, bool) {
	cov := coverageFile{Schema: coverageSchema}
	b, err := os.ReadFile(filepath.Join(root, "coverage.json"))
	if errors.Is(err, os.ErrNotExist) {
		return cov, true
	}
	if err != nil || json.Unmarshal(b, &cov) != nil || cov.Schema != coverageSchema {
		return coverageFile{Schema: coverageSchema}, false
	}
	return cov, true
}

func (s *Store) loadCoverage() coverageFile {
	cov, _ := readCoverage(s.root)
	return cov
}

func (s *Store) saveCoverage(cov coverageFile) error { return s.writeJSON("coverage.json", cov) }

func readSource(root string) (sourceFile, error) {
	var sf sourceFile
	b, err := os.ReadFile(filepath.Join(root, "source.json"))
	if err != nil {
		return sf, err
	}
	if json.Unmarshal(b, &sf) != nil || sf.Schema != sourceSchema || !perfdata.IsSource(sf.Source) || sf.Epoch == 0 {
		return sourceFile{}, errors.New("corrupt source metadata")
	}
	return sf, nil
}

// loadSource returns the current source metadata, recovering it when lost or behind.
// A lost source file is rebuilt from the newest segment with a new epoch; a lost alias
// map starts a new epoch so old and new aliases can never be joined; a metadata file
// behind the segments is moved forward so a sequence is never reused.
func (s *Store) loadSource(cov *coverageFile, scan bool) (sourceFile, error) {
	now := perfdata.FormatTime(s.opts.now())
	sf, err := readSource(s.root)
	changed := false
	if err != nil {
		src, epoch, maxSeq, _, found := s.scanIdentity()
		if found {
			sf = sourceFile{Schema: sourceSchema, Source: src, Epoch: epoch + 1, LastSeq: maxSeq}
			cov.EpochResets = append(cov.EpochResets, perfdata.EpochReset{Epoch: sf.Epoch, Reason: perfdata.ResetSourceLost, At: now})
		} else {
			sf = sourceFile{Schema: sourceSchema, Source: perfdata.NewSource(), Epoch: 1}
			if !errors.Is(err, os.ErrNotExist) {
				cov.EpochResets = append(cov.EpochResets, perfdata.EpochReset{Epoch: 1, Reason: perfdata.ResetSourceLost, At: now})
			}
		}
		changed = true
	} else if scan {
		if src, epoch, maxSeq, first, found := s.scanIdentity(); found && src == sf.Source && maxSeq > sf.LastSeq {
			// Appends to an open segment advance source.json lazily, so metadata
			// behind the newest segment's own range is normal and only caught up.
			// Metadata behind where that segment began was restored or rewritten:
			// a new epoch makes the discontinuity visible.
			if first > sf.LastSeq+1 {
				sf.Epoch = max(sf.Epoch, epoch) + 1
				sf.AliasesMinted = false
				cov.EpochResets = append(cov.EpochResets, perfdata.EpochReset{Epoch: sf.Epoch, Reason: perfdata.ResetSeqBehind, At: now})
			}
			sf.LastSeq = maxSeq
			changed = true
		}
	}
	if sf.AliasesMinted {
		if _, err := os.Stat(s.aliasPath(sf.Epoch)); err != nil {
			sf.Epoch++
			sf.AliasesMinted = false
			cov.EpochResets = append(cov.EpochResets, perfdata.EpochReset{Epoch: sf.Epoch, Reason: perfdata.ResetAliasMapLost, At: now})
			changed = true
		}
	}
	if changed {
		if err := s.writeJSON("source.json", sf); err != nil {
			return sf, err
		}
		if !scan {
			if err := s.saveCoverage(*cov); err != nil {
				return sf, err
			}
		}
	}
	return sf, nil
}

// segment is one segment file name, parsed.
type segment struct {
	name     string
	firstSeq uint64
	epoch    uint32
	day      string // YYYY-MM-DD
}

func segmentName(firstSeq uint64, epoch uint32, day string) string {
	return fmt.Sprintf("seg-%020d-e%d-%s.jsonl", firstSeq, epoch, strings.ReplaceAll(day, "-", ""))
}

func parseSegment(name string) (segment, bool) {
	if !strings.HasPrefix(name, "seg-") || !strings.HasSuffix(name, ".jsonl") {
		return segment{}, false
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(name, "seg-"), ".jsonl"), "-")
	if len(parts) != 3 || len(parts[0]) != 20 || !strings.HasPrefix(parts[1], "e") || len(parts[2]) != 8 {
		return segment{}, false
	}
	seq, err1 := strconv.ParseUint(parts[0], 10, 64)
	ep, err2 := strconv.ParseUint(parts[1][1:], 10, 32)
	if err1 != nil || err2 != nil {
		return segment{}, false
	}
	d := parts[2]
	return segment{name: name, firstSeq: seq, epoch: uint32(ep), day: d[:4] + "-" + d[4:6] + "-" + d[6:]}, true
}

func listSegments(root string) []segment {
	ents, _ := os.ReadDir(filepath.Join(root, "segments"))
	var out []segment
	for _, e := range ents {
		if sg, ok := parseSegment(e.Name()); ok {
			out = append(out, sg)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// scanIdentity finds the source, newest epoch and highest sequence in the segments,
// and the first sequence of the newest segment holding a decodable record.
func (s *Store) scanIdentity() (src string, epoch uint32, maxSeq, first uint64, found bool) {
	segs := listSegments(s.root)
	for i := len(segs) - 1; i >= 0; i-- {
		b, err := os.ReadFile(filepath.Join(s.root, "segments", segs[i].name))
		if err != nil {
			continue
		}
		for _, line := range bytes.Split(b, []byte("\n")) {
			e, err := perfdata.DecodeEvent(line)
			if err != nil {
				continue
			}
			if !found || e.Seq > maxSeq {
				src, maxSeq, found = e.Source, e.Seq, true
			}
			epoch = max(epoch, e.Epoch)
		}
		if found {
			return src, epoch, maxSeq, segs[i].firstSeq, true
		}
	}
	return
}

// segTail describes the newest segment: whether one exists, its size, whether its
// last line is incomplete, and the highest sequence of src among its last lines.
type segTail struct {
	seg     segment
	ok      bool
	size    int64
	partial bool
	lastSeq uint64
}

// newestTail reads only the end of the newest segment. Appends are serialized by the
// store lock, so the highest sequence of a segment sits in its last complete lines.
func (s *Store) newestTail(src string) segTail {
	segs := listSegments(s.root)
	if len(segs) == 0 {
		return segTail{}
	}
	t := segTail{seg: segs[len(segs)-1], ok: true}
	f, err := os.Open(filepath.Join(s.root, "segments", t.seg.name))
	if err != nil {
		t.partial = true // unreadable: never append to it
		return t
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		t.partial = true
		return t
	}
	t.size = st.Size()
	if t.size == 0 {
		return t
	}
	n := min(t.size, int64(2*perfdata.MaxEventBytes+2))
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, t.size-n); err != nil {
		t.partial = true
		return t
	}
	t.partial = buf[n-1] != '\n'
	decoded := false
	for _, line := range bytes.Split(buf, []byte("\n")) {
		if e, err := perfdata.DecodeEvent(line); err == nil && e.Source == src {
			t.lastSeq, decoded = max(t.lastSeq, e.Seq), true
		}
	}
	if !decoded && n < t.size {
		if b, err := os.ReadFile(f.Name()); err == nil {
			for _, line := range bytes.Split(b, []byte("\n")) {
				if e, err := perfdata.DecodeEvent(line); err == nil && e.Source == src {
					t.lastSeq = max(t.lastSeq, e.Seq)
				}
			}
		}
	}
	return t
}

// aliasFile is the private mapping of one epoch: HMAC(secret, locator key) → alias.
// Raw locators are never written; the secret never leaves this file.
type aliasFile struct {
	Schema string            `json:"schema"`
	Epoch  uint32            `json:"epoch"`
	Secret string            `json:"secret"`
	Map    map[string]string `json:"map"`
	dirty  bool
}

type fileStamp struct {
	epoch uint32
	size  int64
	mod   time.Time
}

func (s *Store) aliasPath(epoch uint32) string {
	return filepath.Join(s.root, fmt.Sprintf("aliases-e%d.json", epoch))
}

func (s *Store) loadAliases(epoch uint32) (*aliasFile, error) {
	p := s.aliasPath(epoch)
	st, err := os.Stat(p)
	if errors.Is(err, os.ErrNotExist) {
		af := &aliasFile{Schema: aliasSchema, Epoch: epoch, Secret: randomHex(32), Map: map[string]string{}}
		s.aliases, s.aliasSt = af, fileStamp{epoch: epoch}
		return af, nil
	}
	if err != nil {
		return nil, err
	}
	stamp := fileStamp{epoch: epoch, size: st.Size(), mod: st.ModTime()}
	if s.aliases != nil && s.aliasSt == stamp && !s.aliases.dirty {
		return s.aliases, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var af aliasFile
	if json.Unmarshal(b, &af) != nil || af.Schema != aliasSchema || af.Epoch != epoch || len(af.Secret) != 64 {
		return nil, errors.New("corrupt alias map")
	}
	if af.Map == nil {
		af.Map = map[string]string{}
	}
	s.aliases, s.aliasSt = &af, stamp
	return &af, nil
}

// Alias implements perfdata.AliasMapper over the loaded epoch map.
func (af *aliasFile) Alias(key string) (string, error) {
	sec, _ := hex.DecodeString(af.Secret)
	m := hmac.New(sha256.New, sec)
	m.Write([]byte(key))
	k := hex.EncodeToString(m.Sum(nil))
	if a, ok := af.Map[k]; ok {
		return a, nil
	}
	a := perfdata.NewAlias()
	af.Map[k] = a
	af.dirty = true
	return a, nil
}

// Refused is one refused draft.
type Refused struct {
	Index   int               `json:"index"`
	Refusal *perfdata.Refusal `json:"refusal"`
}

// WriteResult is the outcome of one Write.
type WriteResult struct {
	Lines   []json.RawMessage `json:"lines"`
	Refused []Refused         `json:"refused"`
}

// Write seals and appends drafts as one locked unit: no other writer (goroutine or
// process) can interleave or reuse a sequence. The next sequence is the larger of
// source.json's reservation and the newest segment's last record; source.json is
// rewritten (before the append) only when a segment is started or aliases are minted,
// and after a failed append so the failed range is never reused. With partial=false
// any refused draft refuses the whole call and nothing is written.
func (s *Store) Write(drafts []perfdata.Draft, partial bool) (WriteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res WriteResult
	if s.lockF == nil {
		return res, errors.New("perfdata/local: store closed")
	}
	err := s.locked(func() error {
		cov := coverageFile{}
		covLoaded := false
		loadCov := func() *coverageFile {
			if !covLoaded {
				cov, covLoaded = s.loadCoverage(), true
			}
			return &cov
		}
		sf, err := readSource(s.root)
		if err != nil {
			if sf, err = s.loadSource(loadCov(), false); err != nil {
				return err
			}
		}
		withCtx := false
		for _, d := range drafts {
			withCtx = withCtx || d.Ctx != nil
		}
		if withCtx && sf.AliasesMinted {
			if _, err := os.Stat(s.aliasPath(sf.Epoch)); err != nil {
				if sf, err = s.loadSource(loadCov(), false); err != nil { // alias map lost: new epoch
					return err
				}
			}
		}
		tail := s.newestTail(sf.Source)
		sf.LastSeq = max(sf.LastSeq, tail.lastSeq)
		var af *aliasFile
		recorded := s.opts.now()
		var lines [][]byte
		size := 0
		for i, d := range drafts {
			var ctx *perfdata.Context
			if d.Ctx != nil {
				if af == nil {
					if af, err = s.loadAliases(sf.Epoch); err != nil {
						return err
					}
				}
				ctx, err = perfdata.Translate(s.profile, d.Ctx, af)
			}
			var line []byte
			if err == nil {
				var e perfdata.Event
				id := perfdata.Identity{Source: sf.Source, Epoch: sf.Epoch, Seq: sf.LastSeq + uint64(len(lines)) + 1}
				e, line, err = perfdata.Seal(d, id, s.profile, s.policy.Revision, s.opts.Build, ctx, recorded)
				if err == nil {
					err = s.policy.CheckEvent(e)
				}
			}
			if err != nil {
				res.Refused = append(res.Refused, Refused{Index: i, Refusal: perfdata.AsRefusal(err)})
				err = nil
				continue
			}
			lines = append(lines, line)
			size += len(line) + 1
		}
		if len(lines) == 0 || (!partial && len(res.Refused) > 0) {
			if af != nil && af.dirty {
				s.aliases = nil // discard unsaved aliases
			}
			return nil
		}
		reserve := false
		if af != nil && af.dirty {
			if err := s.writeJSON(filepath.Base(s.aliasPath(sf.Epoch)), af); err != nil {
				s.aliases = nil
				return err
			}
			af.dirty = false
			if st, err := os.Stat(s.aliasPath(sf.Epoch)); err == nil {
				s.aliasSt = fileStamp{epoch: sf.Epoch, size: st.Size(), mod: st.ModTime()}
			}
			sf.AliasesMinted, reserve = true, true
		}
		day := recorded.Format("2006-01-02")
		seg := ""
		if tail.ok && !tail.partial && tail.seg.epoch == sf.Epoch && tail.seg.day == day &&
			(tail.size == 0 || tail.size+int64(size) <= s.policy.SegmentBytes) {
			seg = tail.seg.name
		}
		first := sf.LastSeq + 1
		sf.LastSeq += uint64(len(lines))
		if seg == "" {
			if err := s.fault("segment"); err != nil {
				return s.lost(loadCov(), sf, first, err)
			}
			seg, reserve = segmentName(first, sf.Epoch, day), true
		}
		if reserve {
			if err := s.writeJSON("source.json", sf); err != nil {
				return err // nothing reserved, nothing written
			}
		}
		if err := s.append(seg, lines); err != nil {
			return s.lost(loadCov(), sf, first, err)
		}
		for _, l := range lines {
			res.Lines = append(res.Lines, json.RawMessage(l))
		}
		if seg != tail.seg.name && s.totalBytes() > s.policy.ByteCap {
			c := loadCov()
			if err := s.retire(c); err == nil {
				_ = s.saveCoverage(*c)
			}
		}
		return nil
	})
	return res, err
}

// lost reserves and records a range whose append failed, so it is never reused. Both
// writes may fail too (a full disk); the gap then stays unexplained.
func (s *Store) lost(c *coverageFile, sf sourceFile, first uint64, err error) error {
	_ = s.writeJSON("source.json", sf)
	c.Lost = append(c.Lost, lostRange{Source: sf.Source, Range: perfdata.SeqRange{From: first, To: sf.LastSeq, Reason: perfdata.GapWriteFailed}})
	_ = s.saveCoverage(*c)
	return err
}

func (s *Store) append(name string, lines [][]byte) error {
	if err := s.fault("append"); err != nil {
		return err
	}
	var buf bytes.Buffer
	for _, l := range lines {
		buf.Write(l)
		buf.WriteByte('\n')
	}
	f, err := os.OpenFile(filepath.Join(s.root, "segments", name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(buf.Bytes())
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

func (s *Store) totalBytes() int64 {
	var n int64
	for _, sg := range listSegments(s.root) {
		if st, err := os.Stat(filepath.Join(s.root, "segments", sg.name)); err == nil {
			n += st.Size()
		}
	}
	return n
}
