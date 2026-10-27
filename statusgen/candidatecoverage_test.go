package main

// candidatecoverage_test.go — complete candidate coverage for derived-board/07's
// partial-delivery reconciliation (derived-board/10, #2703).
//
// A declaration list cannot prove its own completeness. These tests bind brief 07's
// candidate cohort to a separately collected forge inventory
// (testdata/witness-admission/candidate-inventory.json): every record of one dated
// state=all walk, matches and nonmatches alike, each bound by the SHA-256 of its kept
// matcher fields in the pinned serialization. Amendment A of the brief explains why the
// exact bytes are a private replay and the committed cohort carries synthetic stand-ins.
// Membership is derived through the existing seams (prRecordsFromPulls + canonicalBriefKey
// for the trailer path, backfillCandidates for the historical path), never from the
// delivery declarations, and joined to separately evidenced dispositions.

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/workref/trailers"
)

const (
	candBrief     = "derived-board/07"
	candStream    = "derived-board"
	candNum       = "07"
	candRepo      = "medici-finance/assay"
	candDir       = "testdata/witness-admission"
	candInvFile   = candDir + "/candidate-inventory.json"
	candCohortFil = candDir + "/candidate-cohort.json"
	candPerPage   = 100
)

// ---- pinned serialization (amendment A) -----------------------------------------------

type pinnedHead struct {
	SHA string `json:"sha"`
	Ref string `json:"ref"`
}

// pinnedRecord is the kept matcher fields of one pull, keys in the pinned order.
type pinnedRecord struct {
	Number         int        `json:"number"`
	State          string     `json:"state"`
	Draft          bool       `json:"draft"`
	Body           string     `json:"body"`
	MergedAt       string     `json:"merged_at"`
	MergeCommitSHA string     `json:"merge_commit_sha"`
	Head           pinnedHead `json:"head"`
}

type pinnedFile struct {
	Filename         string `json:"filename"`
	Status           string `json:"status"`
	PreviousFilename string `json:"previous_filename"`
}

// pinnedJSON is encoding/json with HTML escaping off, no indentation and no trailing newline.
func pinnedJSON(v any) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic(err)
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}

func candSHA(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func pinPull(p ghPull) pinnedRecord {
	return pinnedRecord{Number: p.Number, State: p.State, Draft: p.Draft, Body: p.Body, MergedAt: p.MergedAt,
		MergeCommitSHA: p.MergeCommitSHA, Head: pinnedHead{SHA: p.Head.SHA, Ref: p.Head.Ref}}
}

func (r pinnedRecord) pull() ghPull {
	p := ghPull{Number: r.Number, State: r.State, Draft: r.Draft, Body: r.Body, MergedAt: r.MergedAt, MergeCommitSHA: r.MergeCommitSHA}
	p.Head.SHA, p.Head.Ref = r.Head.SHA, r.Head.Ref
	return p
}

func keptSHA(p ghPull) string { return candSHA(pinnedJSON(pinPull(p))) }

func pinFiles(fs []prFile) []pinnedFile {
	out := []pinnedFile{}
	for _, f := range fs {
		out = append(out, pinnedFile(f))
	}
	return out
}

func filesSHA(fs []prFile) string { return candSHA(pinnedJSON(pinFiles(fs))) }

// ---- the committed inventory ----------------------------------------------------------

type candPage struct {
	Page          int    `json:"page"`
	Params        string `json:"params"`
	Count         int    `json:"count"`
	ResponseSHA   string `json:"response_sha256"`
	ResponseBytes int64  `json:"response_bytes"`
}

type candEntry struct {
	Page           int     `json:"page"`
	Position       int     `json:"position"`
	Number         int     `json:"number"`
	State          string  `json:"state"`
	Draft          bool    `json:"draft"`
	MergedAt       string  `json:"merged_at"`
	MergeCommitSHA string  `json:"merge_commit_sha"`
	HeadSHA        string  `json:"head_sha"`
	KeptSHA        string  `json:"kept_sha256"`
	FilesSHA       *string `json:"files_sha256"`
}

type candLimits struct {
	MaxPages         int    `json:"max_pages"`
	MaxResponseBytes int64  `json:"max_response_bytes"`
	MaxFilePages     int    `json:"max_file_pages_per_pr"`
	Retries          int    `json:"retries"`
	Redirects        string `json:"redirects"`
	TimeoutSeconds   int    `json:"client_timeout_seconds"`
}

type candTotals struct {
	Requests          int    `json:"requests"`
	Pages             int    `json:"pages"`
	Records           int    `json:"records"`
	ResponseBytes     int64  `json:"response_bytes"`
	BodyUTF8Bytes     int64  `json:"body_utf8_bytes"`
	FileListRequests  int    `json:"file_list_requests"`
	FileListPRs       int    `json:"file_list_prs"`
	FileListBytesNote string `json:"file_list_response_bytes"`
}

type candKnownAnswer struct {
	RecordResponse  string `json:"record_response_base64"`
	RecordPinned    string `json:"record_pinned"`
	RecordSHA       string `json:"record_sha256"`
	FilesResponse   string `json:"files_response_base64"`
	FilesPinned     string `json:"files_pinned"`
	FilesSHA        string `json:"files_sha256"`
	SyntheticNotice string `json:"synthetic"`
}

type candInventory struct {
	Schema       string     `json:"schema"`
	Repo         string     `json:"repo"`
	Brief        string     `json:"brief"`
	Endpoint     string     `json:"endpoint"`
	CollectedAt  string     `json:"collected_at"`
	CollectedEnd string     `json:"collected_end"`
	Cutoff       string     `json:"cutoff"`
	Limits       candLimits `json:"limits"`
	Totals       candTotals `json:"totals"`
	Pages        []candPage `json:"pages"`
	Terminal     struct {
		Page    int `json:"page"`
		Count   int `json:"count"`
		PerPage int `json:"per_page"`
	} `json:"terminal"`
	Records     []candEntry     `json:"records"`
	SnapshotSHA string          `json:"snapshot_sha256"`
	EligibleSHA string          `json:"eligible_sha256"`
	KnownAnswer candKnownAnswer `json:"known_answer"`
}

func candPageParams(page int) string {
	return fmt.Sprintf("state=all&per_page=%d&page=%d", candPerPage, page)
}

// candEligible reports whether mergedAt is a merge at or before cutoff. A merge time that
// does not parse is reported as an error, never guessed either way.
func candEligible(mergedAt, cutoff string) (bool, error) {
	if mergedAt == "" {
		return false, nil
	}
	m, err := time.Parse(time.RFC3339, mergedAt)
	if err != nil {
		return false, fmt.Errorf("merged_at %q does not parse: %v", mergedAt, err)
	}
	c, err := time.Parse(time.RFC3339, cutoff)
	if err != nil {
		return false, fmt.Errorf("cutoff %q does not parse: %v", cutoff, err)
	}
	return !m.After(c), nil
}

func candSnapshotDigest(rs []candEntry) string {
	var b strings.Builder
	for _, r := range rs {
		fmt.Fprintf(&b, "%d\t%d\t%d\t%s\n", r.Page, r.Position, r.Number, r.KeptSHA)
	}
	return candSHA([]byte(b.String()))
}

// candEligibleDigest is over number/kept pairs (eligible only), ascending by number.
func candEligibleDigest(pairs map[int]string) string {
	var ns []int
	for n := range pairs {
		ns = append(ns, n)
	}
	sort.Ints(ns)
	var b strings.Builder
	for _, n := range ns {
		fmt.Fprintf(&b, "%d\t%s\n", n, pairs[n])
	}
	return candSHA([]byte(b.String()))
}

// candEligibleKept is the inventory's eligible records, number -> kept_sha256.
func candEligibleKept(inv candInventory) (map[int]string, []string) {
	out := map[int]string{}
	var bad []string
	for _, r := range inv.Records {
		ok, err := candEligible(r.MergedAt, inv.Cutoff)
		if err != nil {
			bad = append(bad, fmt.Sprintf("record #%d: %v", r.Number, err))
			continue
		}
		if ok {
			out[r.Number] = r.KeptSHA
		}
	}
	return out, bad
}

// checkInventory audits the inventory's own completeness evidence. A non-empty result means
// the inventory cannot certify a complete walk.
func checkInventory(inv candInventory) []string {
	var p []string
	bad := func(f string, a ...any) { p = append(p, fmt.Sprintf(f, a...)) }
	if len(inv.Pages) == 0 {
		bad("the inventory records no pages")
		return p
	}
	if inv.Limits.MaxPages > 0 && len(inv.Pages) > inv.Limits.MaxPages {
		bad("%d pages exceed the %d-page collection limit", len(inv.Pages), inv.Limits.MaxPages)
	}
	counts := map[int]int{}
	var bytesSum int64
	for i, pg := range inv.Pages {
		if pg.Page != i+1 {
			bad("page %d is recorded at position %d: the page order is not contiguous", pg.Page, i+1)
		}
		if pg.Params != candPageParams(pg.Page) {
			bad("page %d was requested as %q, not the full-collection parameters", pg.Page, pg.Params)
		}
		last := i == len(inv.Pages)-1
		if !last && pg.Count != candPerPage {
			bad("page %d is short (%d records) but is not the last page", pg.Page, pg.Count)
		}
		if last && pg.Count >= candPerPage {
			bad("the last page %d is full (%d records): no terminal short page proves the walk ended", pg.Page, pg.Count)
		}
		if inv.Limits.MaxResponseBytes > 0 && pg.ResponseBytes > inv.Limits.MaxResponseBytes {
			bad("page %d is %d bytes, over the %d-byte response limit", pg.Page, pg.ResponseBytes, inv.Limits.MaxResponseBytes)
		}
		counts[pg.Page] = pg.Count
		bytesSum += pg.ResponseBytes
	}
	lastPg := inv.Pages[len(inv.Pages)-1]
	if inv.Terminal.Page != lastPg.Page || inv.Terminal.Count != lastPg.Count || inv.Terminal.PerPage != candPerPage {
		bad("the terminal receipt %+v does not match the last recorded page %d (%d records)", inv.Terminal, lastPg.Page, lastPg.Count)
	}
	seen := map[int]bool{}
	got := map[int]int{}
	prev := candEntry{}
	for _, r := range inv.Records {
		got[r.Page]++
		if r.Page < prev.Page || (r.Page == prev.Page && r.Position != prev.Position+1) || (r.Page > prev.Page && r.Position != 1) {
			bad("record #%d at page %d position %d breaks page-and-position order", r.Number, r.Page, r.Position)
		}
		prev = r
		if seen[r.Number] {
			bad("PR #%d appears twice in the inventory", r.Number)
		}
		seen[r.Number] = true
		if len(r.KeptSHA) != 64 {
			bad("record #%d has no kept_sha256", r.Number)
		}
	}
	for pg, n := range counts {
		if got[pg] != n {
			bad("page %d records %d entries but declares %d", pg, got[pg], n)
		}
	}
	for pg := range got {
		if _, ok := counts[pg]; !ok {
			bad("records cite page %d, which the page list does not record", pg)
		}
	}
	if d := candSnapshotDigest(inv.Records); d != inv.SnapshotSHA {
		bad("snapshot_sha256 %s does not match the per-record hashes (%s)", inv.SnapshotSHA, d)
	}
	el, why := candEligibleKept(inv)
	p = append(p, why...)
	if d := candEligibleDigest(el); d != inv.EligibleSHA {
		bad("eligible_sha256 %s does not match the eligible records (%s)", inv.EligibleSHA, d)
	}
	if inv.Totals.Records != len(inv.Records) || inv.Totals.Pages != len(inv.Pages) || inv.Totals.ResponseBytes != bytesSum {
		bad("the totals %+v do not match %d records, %d pages, %d page bytes", inv.Totals, len(inv.Records), len(inv.Pages), bytesSum)
	}
	return p
}

func candEntryOf(inv candInventory, n int) (candEntry, bool) {
	for _, r := range inv.Records {
		if r.Number == n {
			return r, true
		}
	}
	return candEntry{}, false
}

// ---- the committed cohort -------------------------------------------------------------

const candHeld = "held"

// candStandin is a SYNTHETIC stand-in for one member: its identity fields, its link-trailer
// lines verbatim, and the literal backfill form the matcher hit. It is not the forge record.
type candStandin struct {
	Synthetic      string            `json:"synthetic"`
	Number         int               `json:"number"`
	State          string            `json:"state"`
	Draft          bool              `json:"draft"`
	MergedAt       string            `json:"merged_at"`
	MergeCommitSHA string            `json:"merge_commit_sha"`
	HeadSHA        string            `json:"head_sha"`
	Trailers       []string          `json:"trailers"`
	Forms          map[string]string `json:"backfill_forms"`
}

type candMember struct {
	Number         int             `json:"number"`
	Disposition    string          `json:"disposition"`
	Reason         string          `json:"reason"`
	Paths          []string        `json:"paths"`
	MergeCommitSHA string          `json:"merge_commit_sha"`
	FilesSHA       string          `json:"files_sha256"`
	Standin        json.RawMessage `json:"standin"`
	Files          json.RawMessage `json:"files"`
}

type candCohort struct {
	Schema      string       `json:"schema"`
	Brief       string       `json:"brief"`
	Repo        string       `json:"repo"`
	Cutoff      string       `json:"cutoff"`
	InventorySH string       `json:"inventory_snapshot_sha256"`
	Members     []candMember `json:"members"`
}

func candIsHeld(raw json.RawMessage) bool {
	var s string
	return json.Unmarshal(raw, &s) == nil && s == candHeld
}

// standin returns the member's stand-in pull, or ok=false when it is held.
func (m candMember) standin() (ghPull, bool) {
	if candIsHeld(m.Standin) {
		return ghPull{}, false
	}
	var s candStandin
	if json.Unmarshal(m.Standin, &s) != nil || s.Number == 0 {
		return ghPull{}, false
	}
	return s.pull(), true
}

func (s candStandin) pull() ghPull {
	p := ghPull{Number: s.Number, State: s.State, Draft: s.Draft, MergedAt: s.MergedAt, MergeCommitSHA: s.MergeCommitSHA}
	p.Head.SHA = s.HeadSHA
	p.Head.Ref = fmt.Sprintf("standin-%d", s.Number)
	if f := s.Forms["ref"]; f != "" {
		p.Head.Ref = f
	}
	p.Body = s.Forms["body"] + "\n\n" + strings.Join(s.Trailers, "\n") + "\n"
	return p
}

// files returns the member's committed file list, or ok=false when it is held.
func (m candMember) files() ([]prFile, bool) {
	if candIsHeld(m.Files) {
		return nil, false
	}
	var fs []prFile
	if json.Unmarshal(m.Files, &fs) != nil || fs == nil {
		return nil, false
	}
	return fs, true
}

func candLoad[T any](path string) (T, error) {
	var v T
	b, err := os.ReadFile(path)
	if err != nil {
		return v, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return v, dec.Decode(&v)
}

func candStandins(c candCohort) []ghPull {
	var out []ghPull
	for _, m := range c.Members {
		if p, ok := m.standin(); ok {
			out = append(out, p)
		}
	}
	return out
}

// ---- membership and the coverage join --------------------------------------------------

// candidateMembership derives brief 07's candidates from pulls through the existing seams
// only: the `Brief:`-trailer path and the historical backfill path.
func candidateMembership(pulls []ghPull) map[int][]string {
	m := map[int][]string{}
	for _, r := range prRecordsFromPulls(pulls) {
		if r.State == prMerged && canonicalBriefKey(r.BriefRef) == candBrief {
			m[r.Number] = append(m[r.Number], "trailer")
		}
	}
	for _, p := range backfillCandidates(pulls, candStream, candNum) {
		m[p.Number] = append(m[p.Number], "backfill")
	}
	for n := range m {
		sort.Strings(m[n])
	}
	return m
}

type candResult struct {
	Failed    []string // checked-failed, each naming its candidate
	Unchecked []string // could-not-check, each naming why
}

func (r candResult) verdict() string {
	switch {
	case len(r.Failed) > 0:
		return "checked-failed"
	case len(r.Unchecked) > 0:
		return "could-not-check"
	}
	return "checked-clean"
}

// coverageJoin joins the membership derived from pulls to the separately evidenced
// dispositions and the current source declarations. Membership never comes from the
// declarations.
func coverageJoin(pulls []ghPull, members []candMember, d declaredPartials) candResult {
	var r candResult
	fail := func(f string, a ...any) { r.Failed = append(r.Failed, fmt.Sprintf(f, a...)) }
	held := func(f string, a ...any) { r.Unchecked = append(r.Unchecked, fmt.Sprintf(f, a...)) }
	r.Unchecked = append(r.Unchecked, d.unread...)
	byNum := map[int]ghPull{}
	for _, p := range pulls {
		byNum[p.Number] = p
	}
	mem := candidateMembership(pulls)
	disp := map[int]candMember{}
	for _, m := range members {
		if _, dup := disp[m.Number]; dup {
			fail("PR #%d has two dispositions", m.Number)
			continue
		}
		disp[m.Number] = m
	}
	var nums []int
	for n := range mem {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	for _, n := range nums {
		if _, ok := disp[n]; !ok {
			fail("candidate PR #%d (%s path) has no disposition", n, strings.Join(mem[n], "+"))
		}
	}
	members = append([]candMember(nil), members...) // never reorder the caller's cohort
	sort.Slice(members, func(i, j int) bool { return members[i].Number < members[j].Number })
	for _, m := range members {
		if _, ok := m.standin(); !ok {
			held("PR #%d: its stand-in is held, so its membership is could-not-check", m.Number)
			continue
		}
		paths, ok := mem[m.Number]
		if !ok {
			fail("disposition PR #%d names no recorded candidate", m.Number)
			continue
		}
		if !reflect.DeepEqual(paths, m.Paths) {
			fail("PR #%d: recorded paths %v contradict the disposition's %v", m.Number, paths, m.Paths)
		}
		files, have := m.files()
		switch {
		case !have:
			held("PR #%d: its file list is held, so its %s disposition is could-not-check", m.Number, m.Disposition)
			continue
		case filesSHA(files) != m.FilesSHA:
			fail("PR #%d: its committed file list does not hash to its files_sha256", m.Number)
		}
		if strings.TrimSpace(m.Reason) == "" {
			fail("PR #%d: the %s disposition gives no reason", m.Number, m.Disposition)
		}
		partial := d.partial(m.Number, candBrief)
		switch m.Disposition {
		case "partial":
			if !partial {
				fail("PR #%d is a partial delivery, but brief 07 declares no partial delivery for it", m.Number)
			}
		case "full":
			if partial {
				fail("PR #%d is a full delivery, but brief 07 declares it partial", m.Number)
			}
		case "authoring":
			if partial || !briefAuthoringOnly(candBrief, files) {
				fail("PR #%d: its file list is not an authoring-only shape for %s", m.Number, candBrief)
			}
		case "evidence-only":
			held("PR #%d: an Evidence-only verdict needs a patch read this fixture does not commit", m.Number)
		case "mention-only":
			p := byNum[m.Number]
			past, _ := backfillWalkPast(p.Body, candBrief)
			quoted := false
			lines := strings.Split(p.Body, "\n")
			for _, t := range trailers.Scan([]byte(p.Body)) {
				if t.Line >= 1 && t.Line <= len(lines) && strings.Contains(m.Reason, strings.TrimSpace(lines[t.Line-1])) {
					quoted = true
				}
			}
			if partial || !reflect.DeepEqual(m.Paths, []string{"backfill"}) || !past || !quoted {
				fail("PR #%d: a mention-only disposition needs a backfill-only match walked past by its own quoted trailer", m.Number)
			}
		default:
			fail("PR #%d has unsupported disposition %q", m.Number, m.Disposition)
		}
	}
	return r
}

// candidateCoverageJoin is the coverage join the tests call. It is a variable only so a
// fail-first run can disable it and watch the positive controls go red.
var candidateCoverageJoin = coverageJoin

// candidateCoverage is the whole verdict: the inventory's own completeness evidence, then
// the join. An inventory that cannot certify completeness is could-not-check.
func candidateCoverage(inv candInventory, c candCohort, pulls []ghPull, d declaredPartials) candResult {
	if p := checkInventory(inv); len(p) > 0 {
		return candResult{Unchecked: p}
	}
	if c.InventorySH != inv.SnapshotSHA {
		return candResult{Unchecked: []string{"the cohort is bound to a different inventory snapshot"}}
	}
	return candidateCoverageJoin(pulls, c.Members, d)
}

func candNames(msgs []string) string { return strings.Join(msgs, "\n") }

func candMustLoad(t interface {
	Helper()
	Fatal(...any)
}) (candInventory, candCohort) {
	t.Helper()
	inv, err := candLoad[candInventory](candInvFile)
	if err != nil {
		t.Fatal(err)
	}
	c, err := candLoad[candCohort](candCohortFil)
	if err != nil {
		t.Fatal(err)
	}
	return inv, c
}

// candWriteFile writes s to path, creating its directory.
func candWriteFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}


// candMembershipDiff names every disagreement between derived membership and the cohort.
func candMembershipDiff(mem map[int][]string, c candCohort) []string {
	var out []string
	rec := map[int]bool{}
	for _, m := range c.Members {
		rec[m.Number] = true
		got, ok := mem[m.Number]
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("cohort member PR #%d is not a candidate in these records", m.Number))
		case !reflect.DeepEqual(got, m.Paths):
			out = append(out, fmt.Sprintf("PR #%d: derived paths %v differ from the cohort's %v", m.Number, got, m.Paths))
		}
	}
	var extra []int
	for n := range mem {
		if !rec[n] {
			extra = append(extra, n)
		}
	}
	sort.Ints(extra)
	for _, n := range extra {
		out = append(out, fmt.Sprintf("PR #%d is a candidate (%s path) the cohort does not record", n, strings.Join(mem[n], "+")))
	}
	return out
}

// buildCandInventory decodes raw collection pages exactly as fetchAllPulls does
// (encoding/json into ghPull) and records every record's kept fields by hash.
func buildCandInventory(raw [][]byte, start, end, cutoff string) (candInventory, []ghPull, error) {
	inv := candInventory{Schema: "candidate-inventory/v1", Repo: candRepo, Brief: candBrief,
		Endpoint: "GET /repos/" + candRepo + "/pulls", CollectedAt: start, CollectedEnd: end, Cutoff: cutoff,
		Limits: candLimits{MaxPages: 100, MaxResponseBytes: 8 << 20, MaxFilePages: 30, Retries: 0, Redirects: "refused", TimeoutSeconds: 60}}
	var pulls []ghPull
	eligible := map[int]string{}
	for i, b := range raw {
		var batch []ghPull
		if err := json.Unmarshal(b, &batch); err != nil {
			return inv, nil, fmt.Errorf("page %d: %v", i+1, err)
		}
		inv.Pages = append(inv.Pages, candPage{Page: i + 1, Params: candPageParams(i + 1), Count: len(batch), ResponseSHA: candSHA(b), ResponseBytes: int64(len(b))})
		inv.Totals.ResponseBytes += int64(len(b))
		for j, p := range batch {
			k := keptSHA(p)
			inv.Records = append(inv.Records, candEntry{Page: i + 1, Position: j + 1, Number: p.Number, State: p.State, Draft: p.Draft,
				MergedAt: p.MergedAt, MergeCommitSHA: p.MergeCommitSHA, HeadSHA: p.Head.SHA, KeptSHA: k})
			inv.Totals.BodyUTF8Bytes += int64(len(p.Body))
			ok, err := candEligible(p.MergedAt, cutoff)
			if err != nil {
				return inv, nil, err
			}
			if ok {
				eligible[p.Number] = k
			}
			pulls = append(pulls, p)
		}
	}
	if len(inv.Pages) == 0 {
		return inv, nil, fmt.Errorf("no pages")
	}
	last := inv.Pages[len(inv.Pages)-1]
	inv.Terminal.Page, inv.Terminal.Count, inv.Terminal.PerPage = last.Page, last.Count, candPerPage
	inv.Totals.Pages, inv.Totals.Records, inv.Totals.Requests = len(inv.Pages), len(inv.Records), len(inv.Pages)
	inv.SnapshotSHA = candSnapshotDigest(inv.Records)
	inv.EligibleSHA = candEligibleDigest(eligible)
	return inv, pulls, nil
}

// ---- replay (amendment A): the one comparison rows 14 and 15 both drive --------------

type candReplayRecord struct {
	Page     int           `json:"page"`
	Position int           `json:"position"`
	Record   pinnedRecord  `json:"record"`
	Files    *[]pinnedFile `json:"files,omitempty"`
}

type candReplay struct {
	Schema  string             `json:"schema"`
	Kind    string             `json:"kind"`
	Records []candReplayRecord `json:"records"`
}

// candResolve resolves path's longest existing prefix through symlinks.
func candResolve(path string) (string, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	dir, rest := p, ""
	for {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(r, rest), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return p, nil
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		dir = parent
	}
}

// candInside reports whether path, symlinks resolved, lies inside root.
func candInside(path, root string) (bool, error) {
	r, err := candResolve(root)
	if err != nil {
		return false, err
	}
	p, err := candResolve(path)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(r, p)
	if err != nil {
		return false, err
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))), nil
}

func candUnpin(fs []pinnedFile) []prFile {
	out := []prFile{}
	for _, f := range fs {
		out = append(out, prFile(f))
	}
	return out
}

// compareReplay verifies a replay against the committed inventory and cohort, then runs
// flow over the projected pulls and the hash-verified file lists. Only a clean result
// prints the bare `REPLAY checked-clean` line.
func compareReplay(invPath, cohortPath, replayPath, worktree string, w io.Writer, flow func([]ghPull, map[int][]prFile) error) (string, []string) {
	cnc := func(m ...string) (string, []string) { return "could-not-check", m }
	if in, err := candInside(replayPath, worktree); err != nil || in {
		return cnc(fmt.Sprintf("the replay path resolves inside the checkout (or cannot be resolved: %v); replay bytes never live in the tree", err))
	}
	inv, err := candLoad[candInventory](invPath)
	if err != nil {
		return cnc("inventory: " + err.Error())
	}
	c, err := candLoad[candCohort](cohortPath)
	if err != nil {
		return cnc("cohort: " + err.Error())
	}
	if p := checkInventory(inv); len(p) > 0 {
		return cnc(p...)
	}
	if c.InventorySH != inv.SnapshotSHA {
		return cnc("the cohort is bound to a different inventory snapshot")
	}
	rp, err := candLoad[candReplay](replayPath)
	if err != nil {
		return cnc("replay: " + err.Error())
	}
	var msgs []string
	bad := func(f string, a ...any) { msgs = append(msgs, fmt.Sprintf(f, a...)) }
	invBy := map[int]candEntry{}
	for _, r := range inv.Records {
		invBy[r.Number] = r
	}
	eligible, why := candEligibleKept(inv)
	msgs = append(msgs, why...)
	seen := map[int]bool{}
	for _, r := range rp.Records {
		if seen[r.Record.Number] {
			bad("PR #%d appears twice in the replay", r.Record.Number)
		}
		seen[r.Record.Number] = true
	}
	kept := func(r candReplayRecord) string { return candSHA(pinnedJSON(r.Record)) }
	switch rp.Kind {
	case "captured":
		var entries []candEntry
		for i, r := range rp.Records {
			entries = append(entries, candEntry{Page: r.Page, Position: r.Position, Number: r.Record.Number, KeptSHA: kept(r)})
			if i >= len(inv.Records) {
				bad("PR #%d is an extra record in the captured replay", r.Record.Number)
				continue
			}
			e := inv.Records[i]
			if r.Record.Number != e.Number || r.Page != e.Page || r.Position != e.Position {
				bad("replay record %d is PR #%d at page %d position %d; the inventory has PR #%d at page %d position %d",
					i+1, r.Record.Number, r.Page, r.Position, e.Number, e.Page, e.Position)
				continue
			}
			if kept(r) != e.KeptSHA {
				bad("PR #%d: its replay record does not hash to its kept_sha256", e.Number)
			}
		}
		for i := len(rp.Records); i < len(inv.Records); i++ {
			bad("PR #%d is missing from the captured replay", inv.Records[i].Number)
		}
		if candSnapshotDigest(entries) != inv.SnapshotSHA {
			bad("the captured replay does not reproduce snapshot_sha256")
		}
	case "rebuilt":
		pairs := map[int]string{}
		for _, r := range rp.Records {
			k, ok := eligible[r.Record.Number]
			if !ok {
				bad("PR #%d in the rebuilt replay is not an eligible inventory record", r.Record.Number)
				continue
			}
			if kept(r) != k {
				bad("PR #%d: its replay record does not hash to its kept_sha256", r.Record.Number)
			}
			pairs[r.Record.Number] = kept(r)
		}
		var miss []int
		for n := range eligible {
			if !seen[n] {
				miss = append(miss, n)
			}
		}
		sort.Ints(miss)
		for _, n := range miss {
			bad("eligible PR #%d is missing from the rebuilt replay", n)
		}
		if candEligibleDigest(pairs) != inv.EligibleSHA {
			bad("the rebuilt replay does not reproduce eligible_sha256")
		}
	default:
		return cnc(fmt.Sprintf("the replay declares kind %q; only captured or rebuilt is accepted", rp.Kind))
	}
	files := map[int][]prFile{}
	for _, r := range rp.Records {
		n := r.Record.Number
		e, ok := invBy[n]
		if !ok {
			continue
		}
		if r.Files == nil {
			if e.FilesSHA != nil {
				bad("PR #%d: its file list is missing from the replay", n)
			}
			continue
		}
		if e.FilesSHA == nil {
			bad("PR #%d: the replay carries a file list for a record whose files_sha256 is null; it is never read unverified", n)
			continue
		}
		fs := candUnpin(*r.Files)
		if filesSHA(fs) != *e.FilesSHA {
			bad("PR #%d: its replay file list does not hash to files_sha256 (a substituted list)", n)
			continue
		}
		files[n] = fs
	}
	if len(msgs) > 0 {
		return cnc(msgs...)
	}
	var pulls []ghPull // the eligible records only: a later merge is outside the cohort
	for _, r := range rp.Records {
		if _, ok := eligible[r.Record.Number]; ok {
			pulls = append(pulls, r.Record.pull())
		}
	}
	if d := candMembershipDiff(candidateMembership(pulls), c); len(d) > 0 {
		return cnc(d...)
	}
	if flow != nil {
		if err := flow(pulls, files); err != nil {
			return cnc(err.Error())
		}
	}
	fmt.Fprintln(w, "REPLAY checked-clean")
	return "checked-clean", nil
}

// replayVerifier is the comparison the replay tests call. It is a variable only so a
// fail-first run can swap in an accept-all verifier and watch the controls go red.
var replayVerifier = compareReplay

// ---- recollect (row 11): two bounded walks + amendment A's file-list read -------------

type candBounds struct {
	MaxPages  int
	PerResp   int64
	Total     int64
	Requests  int
	Time      time.Duration
	FilePages int
}

var candDefaultBounds = candBounds{MaxPages: 100, PerResp: 8 << 20, Total: 512 << 20, Requests: 200, Time: 180 * time.Second, FilePages: 30}

// candDoer admits only the pulls-collection walk and, once enabled, the allowed PRs'
// file-list reads; it counts requests, decoded bytes and time against fixed bounds.
type candDoer struct {
	inner   httpDoer
	origin  string
	repo    string
	b       candBounds
	start   time.Time
	calls   int
	total   int64
	files   map[int]bool
	log     []string
	refused []string
}

func (d *candDoer) refuse(f string, a ...any) (*http.Response, error) {
	m := "refused: " + fmt.Sprintf(f, a...)
	d.refused = append(d.refused, m)
	return nil, errors.New(m)
}

func (d *candDoer) Do(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return d.refuse("method %s", req.Method)
	}
	if o := req.URL.Scheme + "://" + req.URL.Host; o != d.origin {
		return d.refuse("origin %s is not %s", o, d.origin)
	}
	if time.Since(d.start) > d.b.Time {
		return d.refuse("the %s time bound is spent", d.b.Time)
	}
	if d.calls >= d.b.Requests {
		return d.refuse("the %d-request bound is spent", d.b.Requests)
	}
	coll := "/repos/" + d.repo + "/pulls"
	p, q := req.URL.Path, req.URL.RawQuery
	ok := false
	switch {
	case p == coll:
		for pg := 1; pg <= d.b.MaxPages && !ok; pg++ {
			ok = q == candPageParams(pg)
		}
		if !ok {
			return d.refuse("collection request %q is outside the %d-page full-collection walk", q, d.b.MaxPages)
		}
	case strings.HasPrefix(p, coll+"/") && strings.HasSuffix(p, "/files"):
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(p, coll+"/"), "/files"))
		if err != nil || !d.files[n] {
			return d.refuse("file-list read %s is not an allowed eligible record with a recorded files_sha256", p)
		}
		for pg := 1; pg <= d.b.FilePages && !ok; pg++ {
			ok = q == fmt.Sprintf("per_page=100&page=%d", pg)
		}
		if !ok {
			return d.refuse("file-list request %q is outside the %d-page bound", q, d.b.FilePages)
		}
	default:
		return d.refuse("endpoint %s is neither the pulls collection nor a file-list read", p)
	}
	d.calls++
	d.log = append(d.log, "GET "+p+"?"+q)
	resp, err := d.inner.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body = &candCount{r: resp.Body, d: d}
	return resp, nil
}

// candCount counts decoded response bytes, per response and in aggregate.
type candCount struct {
	r io.ReadCloser
	d *candDoer
	n int64
}

func (c *candCount) Read(p []byte) (int, error) {
	k, err := c.r.Read(p)
	c.n += int64(k)
	c.d.total += int64(k)
	if c.n > c.d.b.PerResp {
		c.d.refused = append(c.d.refused, fmt.Sprintf("refused: a response exceeded the %d-byte decoded-response limit", c.d.b.PerResp))
		return k, errors.New("response over the decoded-response limit")
	}
	if c.d.total > c.d.b.Total {
		c.d.refused = append(c.d.refused, fmt.Sprintf("refused: responses exceeded the %d-byte aggregate limit", c.d.b.Total))
		return k, errors.New("responses over the aggregate limit")
	}
	return k, err
}

func (c *candCount) Close() error { return c.r.Close() }

type candRecollectOpts struct {
	W        io.Writer
	Getenv   func(string) string
	NewDoer  func() httpDoer
	Base     string
	Inv      string
	Cohort   string
	Worktree string
	Bounds   candBounds
}

type candRecollect struct {
	Skipped bool
	Verdict string
	Msgs    []string
	Doer    *candDoer
	Receipt string
}

// recollectCompare compares the fresh pre-cutoff records with the inventory and the
// cohort: a fresh eligible record the inventory lacks, or a membership contradiction, is
// checked-failed; a captured record now absent or changed is could-not-check.
func recollectCompare(inv candInventory, c candCohort, fresh map[int]ghPull) (failed, unchecked []string) {
	captured, why := candEligibleKept(inv)
	unchecked = append(unchecked, why...)
	var nums []int
	for n := range fresh {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	var pulls []ghPull
	for _, n := range nums {
		p := fresh[n]
		pulls = append(pulls, p)
		k, ok := captured[n]
		switch {
		case !ok:
			failed = append(failed, fmt.Sprintf("PR #%d merged at %s, before the cutoff, but the inventory does not record it", n, p.MergedAt))
		case keptSHA(p) != k:
			unchecked = append(unchecked, fmt.Sprintf("PR #%d: its kept fields changed since collection", n))
		}
	}
	var gone []int
	for n := range captured {
		if _, ok := fresh[n]; !ok {
			gone = append(gone, n)
		}
	}
	sort.Ints(gone)
	for _, n := range gone {
		unchecked = append(unchecked, fmt.Sprintf("inventory PR #%d is no longer returned by the forge", n))
	}
	failed = append(failed, candMembershipDiff(candidateMembership(pulls), c)...)
	return failed, unchecked
}

// candRecollectCompare is the comparison the recollect run calls. It is a variable only so
// a fail-first run can disable it and watch the omitted-page control go red.
var candRecollectCompare = recollectCompare

func candRecollectRun(o candRecollectOpts) (res candRecollect) {
	if o.Getenv("ASSAY_COHORT_RECHECK") != "1" {
		res.Skipped = true
		return res
	}
	cnc := func(m ...string) candRecollect {
		res.Verdict, res.Msgs = "could-not-check", m
		return res
	}
	inv, err := candLoad[candInventory](o.Inv)
	if err != nil {
		return cnc("inventory: " + err.Error())
	}
	c, err := candLoad[candCohort](o.Cohort)
	if err != nil {
		return cnc("cohort: " + err.Error())
	}
	if p := checkInventory(inv); len(p) > 0 {
		return cnc(p...)
	}
	if c.InventorySH != inv.SnapshotSHA {
		return cnc("the cohort is bound to a different inventory snapshot")
	}
	out := o.Getenv("ASSAY_COHORT_REPLAY_OUT")
	if out != "" {
		if in, err := candInside(out, o.Worktree); err != nil || in {
			return cnc("ASSAY_COHORT_REPLAY_OUT resolves inside the checkout (or cannot be resolved); refused before any contact")
		}
	}
	token := o.Getenv("GITHUB_TOKEN")
	if token == "" {
		return cnc("no GITHUB_TOKEN: the forge cannot be read, so coverage is could-not-check")
	}
	u, err := url.Parse(o.Base)
	if err != nil {
		return cnc("base: " + err.Error())
	}
	eligible, _ := candEligibleKept(inv)
	var fileNums []int
	for _, r := range inv.Records {
		if _, ok := eligible[r.Number]; ok && r.FilesSHA != nil {
			fileNums = append(fileNums, r.Number)
		}
	}
	sort.Ints(fileNums)
	fmt.Fprintf(o.W, "recollect: target GET %s/repos/%s/pulls?state=all&per_page=100&page=N (N <= %d), two independent walks\n", o.Base, candRepo, o.Bounds.MaxPages)
	fmt.Fprintf(o.W, "recollect: file lists GET %s/repos/%s/pulls/{n}/files?per_page=100&page=K (K <= %d), once each, for PRs %v\n", o.Base, candRepo, o.Bounds.FilePages, fileNums)
	fmt.Fprintf(o.W, "recollect: bounds %d requests, %d bytes per decoded response, %d bytes in total, %s, no retries, redirects refused\n",
		o.Bounds.Requests, o.Bounds.PerResp, o.Bounds.Total, o.Bounds.Time)
	d := &candDoer{inner: o.NewDoer(), origin: u.Scheme + "://" + u.Host, repo: candRepo, b: o.Bounds, start: time.Now()}
	res.Doer = d
	prev := fetchAllPullsMaxPages
	fetchAllPullsMaxPages = o.Bounds.MaxPages
	defer func() { fetchAllPullsMaxPages = prev }()
	cl := &ghClient{doer: d, base: o.Base, token: token}
	var walks [2]map[int]ghPull
	var order []ghPull
	var terminal [2]int
	for i := range walks {
		pulls, ok, why := cl.fetchAllPulls(candRepo)
		if len(d.refused) > 0 {
			return cnc(d.refused...)
		}
		if !ok {
			return cnc(fmt.Sprintf("walk %d: %s", i+1, why))
		}
		m := map[int]ghPull{}
		for _, p := range pulls {
			if _, dup := m[p.Number]; dup {
				return cnc(fmt.Sprintf("walk %d returned PR #%d twice", i+1, p.Number))
			}
			m[p.Number] = p
		}
		walks[i] = m
		terminal[i] = len(pulls) % candPerPage
		if i == 0 {
			order = pulls
		}
	}
	fresh := map[int]ghPull{}
	for n, p := range walks[0] {
		ok, err := candEligible(p.MergedAt, inv.Cutoff)
		if err != nil {
			return cnc(err.Error())
		}
		if ok {
			fresh[n] = p
		}
	}
	for n, p := range walks[1] {
		ok, err := candEligible(p.MergedAt, inv.Cutoff)
		if err != nil {
			return cnc(err.Error())
		}
		q, in := fresh[n]
		if ok != in || (ok && keptSHA(p) != keptSHA(q)) {
			return cnc(fmt.Sprintf("the two walks disagree on PR #%d", n))
		}
	}
	if len(fresh) != len(walksEligible(walks[1], inv.Cutoff)) {
		return cnc("the two walks disagree on the eligible set")
	}
	failed, unchecked := candRecollectCompare(inv, c, fresh)
	if len(failed) > 0 {
		res.Verdict, res.Msgs = "checked-failed", append(failed, unchecked...)
		return res
	}
	if len(unchecked) > 0 {
		return cnc(unchecked...)
	}
	d.files = map[int]bool{}
	for _, n := range fileNums {
		d.files[n] = true
	}
	lists := map[int][]prFile{}
	for _, n := range fileNums {
		fs, err := cl.ListPRFiles(candRepo, n)
		if len(d.refused) > 0 {
			return cnc(d.refused...)
		}
		if err != nil {
			return cnc(fmt.Sprintf("PR #%d file list: %v", n, err))
		}
		e, _ := candEntryOf(inv, n)
		if filesSHA(fs) != *e.FilesSHA {
			return cnc(fmt.Sprintf("PR #%d: its file list no longer hashes to files_sha256", n))
		}
		lists[n] = fs
	}
	invB, _ := os.ReadFile(o.Inv)
	cohB, _ := os.ReadFile(o.Cohort)
	rev, _ := exec.Command("git", "-C", o.Worktree, "rev-parse", "HEAD").Output()
	receipt := map[string]any{
		"source_revision":  strings.TrimSpace(string(rev)),
		"inventory_sha256": candSHA(invB), "cohort_sha256": candSHA(cohB),
		"requests": d.log, "terminal_page_counts": terminal, "elapsed": time.Since(d.start).String(),
		"decoded_bytes": d.total, "eligible_records": len(fresh), "file_lists": len(lists),
	}
	if f, err := os.CreateTemp("", "candidate-recollect-receipt-*.json"); err == nil {
		b, _ := json.MarshalIndent(receipt, "", "  ")
		_, _ = f.Write(b)
		_ = f.Close()
		res.Receipt = f.Name()
		fmt.Fprintf(o.W, "recollect: receipt %s\n", f.Name())
	}
	if out != "" {
		rp := candReplay{Schema: "candidate-replay/v1", Kind: "rebuilt"}
		for i, p := range order {
			if _, ok := fresh[p.Number]; !ok {
				continue
			}
			rec := candReplayRecord{Page: i/candPerPage + 1, Position: i%candPerPage + 1, Record: pinPull(p)}
			if fs, ok := lists[p.Number]; ok {
				pf := pinFiles(fs)
				rec.Files = &pf
			}
			rp.Records = append(rp.Records, rec)
		}
		if err := os.WriteFile(out, pinnedJSON(rp), 0o600); err != nil {
			return cnc("writing the rebuilt replay: " + err.Error())
		}
	}
	fmt.Fprintln(o.W, "RECOLLECT checked-clean")
	res.Verdict = "checked-clean"
	return res
}

func walksEligible(m map[int]ghPull, cutoff string) map[int]bool {
	out := map[int]bool{}
	for n, p := range m {
		if ok, _ := candEligible(p.MergedAt, cutoff); ok {
			out[n] = true
		}
	}
	return out
}

// ---- isolated source tree ---------------------------------------------------------------

const candBrief07 = "docs/streams/derived-board/brief-07-rollout-backfill.md"

func candProdHashes(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, p := range []string{candBrief07, "docs/streams/graph-repos.yaml", "docs/streams/derived-board/README.md"} {
		b, err := os.ReadFile(filepath.Join("..", p))
		if err != nil {
			t.Fatal(err)
		}
		out[p] = candSHA(b)
	}
	return out
}

// candIsoTree copies the real registry, brief 07 and its README row into a temporary tree,
// applying mut to brief 07's bytes. The production files are never written.
func candIsoTree(t *testing.T, mut func(string) string) string {
	t.Helper()
	root := t.TempDir()
	read := func(p string) string {
		b, err := os.ReadFile(filepath.Join("..", p))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	readme := read("docs/streams/derived-board/README.md")
	end := strings.Index(readme[4:], "\n---\n")
	if !strings.HasPrefix(readme, "---\n") || end < 0 {
		t.Fatal("README frontmatter")
	}
	var b strings.Builder
	b.WriteString(readme[:end+9])
	b.WriteString("\n# derived-board Stream\n\n## Briefs\n\n<!-- statusgen:briefs:begin -->\n")
	for _, l := range strings.Split(readme, "\n") {
		if strings.HasPrefix(l, "| # |") || strings.HasPrefix(l, "|---") || strings.HasPrefix(l, "| 07 |") {
			b.WriteString(l + "\n")
		}
	}
	b.WriteString("<!-- statusgen:briefs:end -->\n")
	brief := read(candBrief07)
	if mut != nil {
		m := mut(brief)
		if m == brief {
			t.Fatal("brief 07 mutation changed nothing")
		}
		brief = m
	}
	for p, s := range map[string]string{
		"docs/streams/graph-repos.yaml":        read("docs/streams/graph-repos.yaml"),
		"docs/streams/derived-board/README.md": b.String(),
		candBrief07:                            brief,
	} {
		if err := candWriteFile(filepath.Join(root, p), []byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func candDropDecl(n int) func(string) string {
	return func(s string) string {
		var out []string
		for _, l := range strings.Split(s, "\n") {
			if !strings.Contains(l, fmt.Sprintf(`{in: "assay#%d",`, n)) {
				out = append(out, l)
			}
		}
		return strings.Join(out, "\n")
	}
}

func candReplaceRef(from, to string) func(string) string {
	return func(s string) string { return strings.Replace(s, `in: "`+from+`"`, `in: "`+to+`"`, 1) }
}

func candFullDecl(n int) func(string) string {
	return func(s string) string {
		return strings.Replace(s, "\ndelivery:\n", fmt.Sprintf("\ndelivery:\n  - {in: \"assay#%d\", covers: full, note: \"SYNTHETIC full claim\"}\n", n), 1)
	}
}

func candReadme(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "docs/streams/derived-board/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func candServed(p ghPull) map[string]any {
	return map[string]any{"number": p.Number, "state": p.State, "draft": p.Draft, "body": p.Body,
		"merged_at": p.MergedAt, "merge_commit_sha": p.MergeCommitSHA, "head": map[string]any{"sha": p.Head.SHA, "ref": p.Head.Ref}}
}

func candServedPulls(ps []ghPull) []map[string]any {
	out := []map[string]any{}
	for _, p := range ps {
		out = append(out, candServed(p))
	}
	return out
}

func candFileMaps(fs []prFile) []map[string]any {
	out := []map[string]any{}
	for _, f := range fs {
		out = append(out, map[string]any{"filename": f.Filename, "status": f.Status, "previous_filename": f.PreviousFilename})
	}
	return out
}

// candServedFiles serves each member's committed list; a held list answers HTTP 500.
func candServedFiles(c candCohort) (map[int][]map[string]any, map[int]bool) {
	files, held := map[int][]map[string]any{}, map[int]bool{}
	for _, m := range c.Members {
		if fs, ok := m.files(); ok {
			files[m.Number] = candFileMaps(fs)
		} else {
			held[m.Number] = true
		}
	}
	return files, held
}

func candWithout(ps []ghPull, n int) []ghPull {
	var out []ghPull
	for _, p := range ps {
		if p.Number != n {
			out = append(out, p)
		}
	}
	return out
}

func candClone[T any](t *testing.T, v T) T {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func candCheckKnownAnswer(t *testing.T, ka candKnownAnswer) {
	t.Helper()
	rb, err := base64.StdEncoding.DecodeString(ka.RecordResponse)
	if err != nil {
		t.Fatal(err)
	}
	var p ghPull
	if err := json.Unmarshal(rb, &p); err != nil {
		t.Fatal(err)
	}
	if got := string(pinnedJSON(pinPull(p))); got != ka.RecordPinned || keptSHA(p) != ka.RecordSHA {
		t.Fatalf("known-answer record: got %s / %s", got, keptSHA(p))
	}
	if !strings.Contains(ka.RecordPinned, "<b>&amp;") || !strings.Contains(ka.RecordPinned, "\uFFFD") {
		t.Fatal("known-answer vector no longer exercises HTML escaping and invalid UTF-8")
	}
	fb, err := base64.StdEncoding.DecodeString(ka.FilesResponse)
	if err != nil {
		t.Fatal(err)
	}
	var fs []prFile
	if err := json.Unmarshal(fb, &fs); err != nil {
		t.Fatal(err)
	}
	if got := string(pinnedJSON(pinFiles(fs))); got != ka.FilesPinned || filesSHA(fs) != ka.FilesSHA {
		t.Fatalf("known-answer files: got %s", got)
	}
}

// ---- tests ------------------------------------------------------------------------------

// regression: #2703
func TestCandidateSnapshot(t *testing.T) {
	inv, c := candMustLoad(t)
	if p := checkInventory(inv); len(p) > 0 {
		t.Fatalf("committed inventory: %v", p)
	}
	candCheckKnownAnswer(t, inv.KnownAnswer)
	if c.InventorySH != inv.SnapshotSHA || c.Cutoff != inv.Cutoff || c.Brief != candBrief {
		t.Fatal("the cohort is not bound to this inventory")
	}
	for _, m := range c.Members {
		e, ok := candEntryOf(inv, m.Number)
		if !ok {
			t.Fatalf("member #%d is not in the inventory", m.Number)
		}
		if el, err := candEligible(e.MergedAt, inv.Cutoff); err != nil || !el {
			t.Fatalf("member #%d is not an eligible merge (%v)", m.Number, err)
		}
		if e.MergeCommitSHA != m.MergeCommitSHA || e.FilesSHA == nil || *e.FilesSHA != m.FilesSHA {
			t.Fatalf("member #%d: identity or files_sha256 differs from the inventory", m.Number)
		}
		if p, ok := m.standin(); ok {
			if p.State != e.State || p.Draft != e.Draft || p.MergedAt != e.MergedAt || p.MergeCommitSHA != e.MergeCommitSHA || p.Head.SHA != e.HeadSHA {
				t.Fatalf("member #%d: stand-in identity differs from the inventory", m.Number)
			}
		}
		if fs, ok := m.files(); ok && filesSHA(fs) != m.FilesSHA {
			t.Fatalf("member #%d: committed file list does not hash to files_sha256", m.Number)
		}
	}
	mem := candidateMembership(candStandins(c))
	if d := candMembershipDiff(mem, c); len(d) > 0 || mem[735] == nil {
		t.Fatalf("stand-ins do not replay the cohort: %v", d)
	}
	for name, f := range map[string]func(*candInventory){
		"dropped record":     func(v *candInventory) { v.Records = append(v.Records[:5:5], v.Records[6:]...) },
		"dropped page":       func(v *candInventory) { v.Pages = append(v.Pages[:1:1], v.Pages[2:]...) },
		"full terminal page": func(v *candInventory) { v.Pages[len(v.Pages)-1].Count = candPerPage; v.Terminal.Count = candPerPage },
		"capped":             func(v *candInventory) { v.Limits.MaxPages = len(v.Pages) - 1 },
		"corrupt kept":       func(v *candInventory) { v.Records[3].KeptSHA = strings.Repeat("0", 64) },
		"duplicate number":   func(v *candInventory) { v.Records[1].Number = v.Records[0].Number },
		"edited snapshot":    func(v *candInventory) { v.SnapshotSHA = strings.Repeat("1", 64) },
		"missing":            func(v *candInventory) { *v = candInventory{} },
	} {
		cp := candClone(t, inv)
		f(&cp)
		if r := candidateCoverage(cp, c, candStandins(c), declaredPartials{}); r.verdict() != "could-not-check" {
			t.Errorf("%s: an inventory that cannot certify completeness read %s", name, r.verdict())
		}
	}
	if _, err := candLoad[candInventory](filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("a missing inventory loaded")
	}
}

// regression: #2703
func TestCandidateCoverage(t *testing.T) {
	inv, c := candMustLoad(t)
	before := candProdHashes(t)
	pulls := candStandins(c)
	clean := candidateCoverage(inv, c, pulls, readDeclaredPartials(candIsoTree(t, nil), candRepo))
	if clean.verdict() != "checked-clean" {
		t.Fatalf("current source: %s %v %v", clean.verdict(), clean.Failed, clean.Unchecked)
	}
	r := candidateCoverage(inv, c, pulls, readDeclaredPartials(candIsoTree(t, candDropDecl(735)), candRepo))
	if r.verdict() != "checked-failed" || len(r.Failed) != 1 || !strings.Contains(r.Failed[0], "PR #735 ") {
		t.Fatalf("omitted #735 declaration: %s %v", r.verdict(), r.Failed)
	}
	// Membership never comes from the declarations: #735 is still a candidate.
	if candidateMembership(pulls)[735] == nil {
		t.Fatal("#735 left the membership")
	}
	if r := candidateCoverage(inv, c, pulls, readDeclaredPartials(candIsoTree(t, nil), candRepo)); r.verdict() != "checked-clean" {
		t.Fatalf("restored source: %v", r)
	}
	if got := candProdHashes(t); !reflect.DeepEqual(got, before) {
		t.Fatal("production source changed")
	}
}

func candPlant() ghPull {
	p := ghPull{Number: 700, State: "closed", Body: "SYNTHETIC older delivery.\n\nBrief: derived-board/07\n",
		MergedAt: "2026-08-01T00:00:00Z", MergeCommitSHA: strings.Repeat("7", 40)}
	p.Head.SHA, p.Head.Ref = strings.Repeat("8", 40), "feat/derived-board-07-prep"
	return p
}

// regression: #2703
func TestCandidatePlant(t *testing.T) {
	inv, c := candMustLoad(t)
	d := readDeclaredPartials(candIsoTree(t, nil), candRepo)
	if d.partial(700, candBrief) {
		t.Fatal("plant was predeclared")
	}
	pulls := append(candStandins(c), candPlant())
	r := candidateCoverage(inv, c, pulls, d)
	if r.verdict() != "checked-failed" || len(r.Failed) != 1 || !strings.Contains(r.Failed[0], "PR #700 ") {
		t.Fatalf("planted second older candidate: %s %v", r.verdict(), r.Failed)
	}
	// Independently of the five declared partials, the plant would derive implemented.
	prs := prsForCohort(prRecordsFromPulls(pulls), candBrief)
	markDeclaredPartials(prs, d, nil)
	cell := deriveSingle(t, BriefIdent{ID: candBrief}, LifecycleInput{LookedAt: true, PRs: prs})
	if cell.Cell != "implemented" || !strings.Contains(cell.MergedPR, "#700 ") {
		t.Fatalf("plant: %+v", cell)
	}
	if r := candidateCoverage(inv, c, candStandins(c), d); r.verdict() != "checked-clean" {
		t.Fatalf("without the plant: %v", r)
	}
}

// regression: #2703
func TestCandidateFlow(t *testing.T) {
	_, c := candMustLoad(t)
	before := candProdHashes(t)
	for _, tc := range []struct {
		name string
		mut  func(string) string
		impl bool
	}{{"current", nil, false}, {"omitted-735", candDropDecl(735), true}, {"restored", nil, false}} {
		for _, backfill := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/backfill=%v", tc.name, backfill), func(t *testing.T) {
				t.Setenv("GITHUB_TOKEN", "test-token")
				root := candIsoTree(t, tc.mut)
				commitFixture(t, root)
				readme := candReadme(t, root)
				files, held := candServedFiles(c)
				authoringServer(t, candServedPulls(candStandins(c)), files, held)
				src := &fakeFlipSource{}
				foldFlipSeam(t, src)
				args := []string{"--apply", "--root", root, "--repo", candRepo, "--json"}
				if backfill {
					args = append([]string{"--backfill"}, args...)
				}
				code, out := runReconcileCaptured(t, args...)
				if code != reconcileOK {
					t.Fatalf("exit %d: %s", code, out)
				}
				cell := cellOf(t, out, candBrief)
				ids := appliedIDs(t, out)
				if tc.impl {
					if cell.Cell != "implemented" || len(ids) != 1 || !strings.Contains(ids[0], "PR #735 ") {
						t.Fatalf("omitted declaration should surface the false implemented: %+v %v", cell, ids)
					}
					return
				}
				if cell.Cell != "in-progress" || cell.MergedPR != "" || len(ids) != 0 {
					t.Fatalf("partial-only cohort: %+v applied=%v", cell, ids)
				}
				if candReadme(t, root) != readme {
					t.Fatal("apply wrote the README")
				}
				if len(src.shapesSeen) != 0 {
					t.Fatalf("shape reads %v", src.shapesSeen)
				}
				streams, _, err := loadStreams(root)
				if err != nil {
					t.Fatal(err)
				}
				for _, s := range streams {
					s.Board = "generated"
				}
				ef, err := os.CreateTemp(t.TempDir(), "stderr")
				if err != nil {
					t.Fatal(err)
				}
				defer ef.Close()
				for _, n := range regenDriftNotices(root, streams, candRepo, false, "", ef) {
					if strings.Contains(n, "derived-board/07") || strings.Contains(n, "derived-board:07") {
						t.Fatalf("drift: %s", n)
					}
				}
			})
		}
	}
	if got := candProdHashes(t); !reflect.DeepEqual(got, before) {
		t.Fatal("production source changed")
	}
}

// regression: #2703
func TestCandidateFull(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	f := cohortLoad(t)
	r := cohortRecordAt(t, f, 1196)
	if len(r.Files) != 10 || !strings.Contains(r.Scope, "finishes those two deliverables") {
		t.Fatal("real full-delivery evidence absent")
	}
	if old := cohortRecordAt(t, f, 1062); !strings.Contains(old.Scope, "root resolution") || len(old.Files) != 35 {
		t.Fatal("prior delivery evidence absent")
	}
	reads := authoringServer(t, nil, map[int][]map[string]any{1196: candFileMaps(r.Files)}, nil)
	cl := newAuthoringClassifier(reconcileGHClient("test-token"), candRepo, nil)
	for i := 0; i < 2; i++ {
		if got, err := cl.nonDelivery(1196, "desk-tools/27"); err != nil || got {
			t.Fatalf("full delivery excluded=%v err=%v", got, err)
		}
	}
	if len(*reads) != 1 {
		t.Fatalf("file-list reads %v, want one cached read", *reads)
	}
	p := []PRRecord{
		{BriefRef: "desk-tools/27", Number: 1196, State: prMerged, MergeSHA: r.Pull.MergeCommitSHA},
		{BriefRef: "desk-tools/27", Number: 4000, State: prMerged, MergeSHA: "abcdef0", Partial: true},
	}
	b := BriefIdent{ID: "desk-tools/27", Version: 1}
	cell := deriveSingle(t, b, LifecycleInput{LookedAt: true, PRs: p, Witnesses: map[string]WitnessInfo{b.ID: {Version: 1, Passed: false}}})
	if cell.Cell != "implemented" || !strings.Contains(cell.MergedPR, "PR #1196 ") {
		t.Fatalf("full delivery under later partial and VERIFY FAIL: %+v", cell)
	}
	// A full claim beside a partial one is full: the reader drops the partial and the
	// coverage join names the contradiction.
	d := readDeclaredPartials(candIsoTree(t, candFullDecl(735)), candRepo)
	if d.partial(735, candBrief) || !d.partial(736, candBrief) {
		t.Fatal("full-over-partial history")
	}
	inv, c := candMustLoad(t)
	if res := candidateCoverage(inv, c, candStandins(c), d); res.verdict() != "checked-failed" || !strings.Contains(candNames(res.Failed), "PR #735 ") {
		t.Fatalf("full #735: %v", res)
	}
	t.Run("full-after-partial", TestPartialThenDelivery)
}

// regression: #2703
func TestCandidateUnknown(t *testing.T) {
	inv, c := candMustLoad(t)
	pulls := candStandins(c)
	d := readDeclaredPartials(candIsoTree(t, nil), candRepo)
	capped := candClone(t, inv)
	capped.Limits.MaxPages = len(capped.Pages) - 1
	if r := candidateCoverage(capped, c, pulls, d); r.verdict() != "could-not-check" {
		t.Fatalf("capped inventory: %v", r)
	}
	heldC := candClone(t, c)
	heldC.Members[0].Files = json.RawMessage(`"held"`)
	if r := candidateCoverage(inv, heldC, pulls, d); r.verdict() != "could-not-check" ||
		!strings.Contains(candNames(r.Unchecked), fmt.Sprintf("PR #%d:", heldC.Members[0].Number)) {
		t.Fatalf("held evidence: %v", r)
	}
	for name, mut := range map[string]func(string) string{
		"malformed": candReplaceRef("assay#735", "assay735"),
		"withheld":  candReplaceRef("assay#735", "pvt#735"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", "test-token")
			root := candIsoTree(t, mut)
			commitFixture(t, root)
			readme := candReadme(t, root)
			files, held := candServedFiles(c)
			authoringServer(t, candServedPulls(pulls), files, held)
			_, out := runReconcileCaptured(t, "--root", root, "--repo", candRepo, "--json")
			if cell := cellOf(t, out, candBrief); cell.Cell != "unknown" {
				t.Fatalf("cell %+v", cell)
			}
			if code, out := runReconcileCaptured(t, "--apply", "--root", root, "--repo", candRepo, "--json"); code != reconcileCouldNotCheck {
				t.Fatalf("apply exit %d, want could-not-check: %s", code, out)
			}
			if candReadme(t, root) != readme {
				t.Fatal("apply wrote through an unresolved declaration")
			}
			d := readDeclaredPartials(root, candRepo)
			if len(d.undecided) == 0 {
				t.Fatal("unresolved claim not disclosed")
			}
			for _, n := range []int{736, 945, 1010, 1194} {
				if !d.partial(n, candBrief) {
					t.Fatalf("direct reader dropped declared partial #%d", n)
				}
			}
			if r := candidateCoverage(inv, c, pulls, d); r.verdict() == "checked-clean" {
				t.Fatal("unresolved declaration read clean")
			}
		})
	}
}

// candMatch runs the real classifier over the committed lists served by authoringServer.
func candMatch(t *testing.T, id string, pulls []ghPull, files map[int][]map[string]any, fail map[int]bool, partials declaredPartials, src *fakeFlipSource) (ghPull, bool, string, []int) {
	t.Helper()
	t.Setenv("GITHUB_TOKEN", "test-token")
	reads := authoringServer(t, candServedPulls(pulls), files, fail)
	if src == nil {
		src = &fakeFlipSource{}
	}
	foldFlipSeam(t, src)
	cl := newAuthoringClassifier(reconcileGHClient("test-token"), candRepo, nil)
	cl.partials = partials
	stream, num, ok := briefStreamNum(id)
	if !ok {
		t.Fatalf("brief id %q", id)
	}
	m, ok, unread := matchBackfillPRChecked(pulls, stream, num, cl.nonDelivery)
	return m, ok, unread, *reads
}

func candRuleOff(t *testing.T) {
	prev := backfillWalkPast
	backfillWalkPast = func(string, string) (bool, string) { return false, "" }
	t.Cleanup(func() { backfillWalkPast = prev })
}

// regression: #2703
func TestCandidateMentionOnly(t *testing.T) {
	inv, c := candMustLoad(t)
	standins := candStandins(c)
	files, held := candServedFiles(c)
	if len(held) > 0 {
		t.Fatalf("held member lists %v: the backfill path is could-not-check", held)
	}
	partials := readDeclaredPartials(candIsoTree(t, nil), candRepo)
	if m, ok, unread, _ := candMatch(t, candBrief, standins, files, nil, partials, nil); ok || unread != "" {
		t.Fatalf("recorded cohort produced a witness #%d (unread %q)", m.Number, unread)
	}
	if n := len(backfillCandidates(standins, candStream, candNum)); n != len(c.Members) {
		t.Fatalf("backfill candidates %d", n)
	}
	mentions := 0
	for _, m := range c.Members {
		if m.Disposition != "mention-only" {
			continue
		}
		mentions++
		p, _ := m.standin()
		if past, why := backfillTrailerWalkPast(p.Body, candBrief); !past || why == "" {
			t.Fatalf("mention-only #%d is not walked past by its own trailer", m.Number)
		}
		if m.Number == 3794 && !strings.Contains(m.Reason, "Authors: derived-board/10") {
			t.Fatal("#3794 reason does not name its Authors: trailer")
		}
	}
	if mentions != 9 {
		t.Fatalf("mention-only members %d, want 9", mentions)
	}
	t.Run("rule-disabled", func(t *testing.T) {
		candRuleOff(t)
		if m, ok, _, _ := candMatch(t, candBrief, standins, files, nil, partials, nil); !ok || m.Number != 3945 {
			t.Fatalf("rule off, full cohort: #%d ok=%v", m.Number, ok)
		}
		if m, ok, _, _ := candMatch(t, candBrief, candWithout(standins, 3945), files, nil, partials, nil); !ok || m.Number != 3794 {
			t.Fatalf("rule off, without #3945: #%d ok=%v", m.Number, ok)
		}
	})
	t.Run("reconcile-backfill", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "test-token")
		root := candIsoTree(t, nil)
		commitFixture(t, root)
		readme := candReadme(t, root)
		authoringServer(t, candServedPulls(standins), files, held)
		code, out := runReconcileCaptured(t, "--backfill", "--apply", "--root", root, "--repo", candRepo, "--json")
		if cell := cellOf(t, out, candBrief); code != reconcileOK || cell.Cell != "in-progress" || cell.MergedPR != "" || len(appliedIDs(t, out)) != 0 {
			t.Fatalf("exit %d cell %+v", code, cell)
		}
		if candReadme(t, root) != readme {
			t.Fatal("status write")
		}
	})
	pull := func(n int, ref, body, merged string) ghPull {
		p := ghPull{Number: n, State: "closed", Body: body, MergedAt: merged, MergeCommitSHA: strings.Repeat("c", 40)}
		p.Head.SHA, p.Head.Ref = strings.Repeat("d", 40), ref
		return p
	}
	t.Run("trailerless-branch", func(t *testing.T) {
		p := pull(9001, "feat/derived-board-07-x", "", "2026-10-01T00:00:00Z")
		if m, ok, _, _ := candMatch(t, candBrief, []ghPull{p}, map[int][]map[string]any{9001: deliveryFiles}, nil, partials, nil); !ok || m.Number != 9001 {
			t.Fatal("trailerless branch candidate lost its witness")
		}
	})
	t.Run("declared-partial-only", func(t *testing.T) {
		p735 := standins[len(standins)-1]
		for _, s := range standins {
			if s.Number == 735 {
				p735 = s
			}
		}
		if _, ok, _, _ := candMatch(t, candBrief, []ghPull{p735}, files, nil, partials, nil); ok {
			t.Fatal("declared partial became the witness")
		}
		if m, ok, _, _ := candMatch(t, candBrief, []ghPull{p735}, files, nil, declaredPartials{}, nil); !ok || m.Number != 735 {
			t.Fatal("undeclared #735 is no longer the witness")
		}
	})
	t.Run("fenced-trailer", func(t *testing.T) {
		p := pull(9002, "x", "Notes on derived-board/07.\n\n```\nBrief: other/01\n```\n", "2026-10-01T00:00:00Z")
		if m, ok, _, _ := candMatch(t, candBrief, []ghPull{p}, map[int][]map[string]any{9002: deliveryFiles}, nil, partials, nil); !ok || m.Number != 9002 {
			t.Fatal("a fenced Brief: made the candidate trailer-era")
		}
	})
	t.Run("no-witness-brief", func(t *testing.T) {
		shape := reportedEvidence(t)
		nb := []ghPull{
			pull(503, "x503", "SYNTHETIC: authors nb/01.\n\nBrief: nb/01\n", "2026-10-03T00:00:00Z"),
			pull(502, "x502", "SYNTHETIC: part of nb/01.\n\nBrief: nb/01\n", "2026-10-02T00:00:00Z"),
			pull(501, "x501", "SYNTHETIC: Evidence for nb/01.\n", "2026-10-01T00:00:00Z"),
		}
		nbFiles := map[int][]map[string]any{
			503: {{"filename": "docs/streams/nb/brief-01-x.md", "status": "added"}, {"filename": "docs/streams/nb/README.md", "status": "modified"}},
			502: deliveryFiles,
			501: evidenceFiles(shape),
		}
		nbPartials := declaredPartials{byKey: map[string]map[int]bool{"nb/01": {502: true}}}
		for _, off := range []bool{false, true} {
			t.Run(fmt.Sprint("rule-off=", off), func(t *testing.T) {
				if off {
					candRuleOff(t)
				}
				src := &fakeFlipSource{shapes: map[int]prShape{501: shape}}
				if m, ok, unread, _ := candMatch(t, "nb/01", nb, nbFiles, nil, nbPartials, src); ok || unread != "" {
					t.Fatalf("synthetic brief gained witness #%d (unread %q)", m.Number, unread)
				}
			})
		}
	})
	t.Run("older-trailerless", func(t *testing.T) {
		old := pull(600, "feat/derived-board-07-old", "", "2026-09-01T00:00:00Z")
		all := append(append([]ghPull{}, standins...), old)
		fs := map[int][]map[string]any{600: deliveryFiles}
		for k, v := range files {
			fs[k] = v
		}
		if m, ok, _, _ := candMatch(t, candBrief, all, fs, nil, partials, nil); !ok || m.Number != 600 {
			t.Fatalf("older trailerless candidate not reached: #%d ok=%v", m.Number, ok)
		}
		if r := candidateCoverage(inv, c, all, partials); r.verdict() != "checked-failed" || !strings.Contains(candNames(r.Failed), "PR #600 ") {
			t.Fatalf("coverage did not name #600: %v", r)
		}
	})
	t.Run("unreadable", func(t *testing.T) {
		for _, n := range []int{3945, 1457} {
			_, ok, unread, reads := candMatch(t, candBrief, standins, files, map[int]bool{n: true}, partials, nil)
			if ok || !strings.Contains(unread, fmt.Sprintf("#%d", n)) {
				t.Fatalf("unreadable #%d: ok=%v unread=%q", n, ok, unread)
			}
			for _, r := range reads {
				if r < n {
					t.Fatalf("search continued past unreadable #%d to #%d", n, r)
				}
			}
		}
	})
}

// regression: #2703
func TestBackfillTrailerPredicate(t *testing.T) {
	other := prShape{Files: []string{"internal/x/x.go"}, BriefTrailers: 1, Briefs: []string{"other/01"}}
	if v, _ := attributeCandidate(other, "x/01"); v != candidateWalkPast {
		t.Fatal("attributeCandidate does not walk past another brief")
	}
	if past, _ := backfillTrailerWalkPast("Brief: other/01\n", "x/01"); !past {
		t.Fatal("backfill does not walk past another brief")
	}
	prev := linkTrailerVerdict
	linkTrailerVerdict = func(int, []string, string) (candidateVerdict, string) { return candidateCredit, "" }
	v, _ := attributeCandidate(other, "x/01")
	past, _ := backfillTrailerWalkPast("Brief: other/01\n", "x/01")
	linkTrailerVerdict = prev
	if v != candidateCredit || past {
		t.Fatal("the two readers do not share one predicate")
	}
	count, briefs, issues := parseLinkTrailers("Brief: a/01\nAuthors: b/02\nIssue: #5\n")
	if count != 2 || len(briefs) != 1 || issues != 1 {
		t.Fatalf("count=%d briefs=%v issues=%d", count, briefs, issues)
	}
	if why := bulkMigrationReason(prShape{Files: []string{"internal/x/x.go"}, BriefTrailers: count}); why != "" {
		t.Fatalf("two trailers plus Issue: tripped bulk: %s", why)
	}
	if why := bulkMigrationReason(prShape{Files: []string{"internal/x/x.go"}, BriefTrailers: 3}); why == "" {
		t.Fatal("three Brief:/Authors: trailers no longer trip bulk")
	}
	if _, _, issues := parseLinkTrailers("```\nIssue: #5\n```\n"); issues != 0 {
		t.Fatal("a fenced Issue: was counted")
	}
}

// candRepoRoot is the checkout this test runs in.
func candRepoRoot(t *testing.T) string {
	t.Helper()
	r, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// regression: #2703
func TestCandidateRecollect(t *testing.T) {
	res := candRecollectRun(candRecollectOpts{W: os.Stdout, Getenv: os.Getenv,
		NewDoer: func() httpDoer {
			return &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		},
		Base: githubAPIBase, Inv: candInvFile, Cohort: candCohortFil, Worktree: candRepoRoot(t), Bounds: candDefaultBounds})
	if res.Skipped {
		t.Skip("ASSAY_COHORT_RECHECK is not 1: no forge contact")
	}
	if res.Verdict != "checked-clean" {
		t.Fatalf("recollect %s:\n%s", res.Verdict, candNames(res.Msgs))
	}
}

// regression: #2703
func TestCandidateReplay(t *testing.T) {
	path := os.Getenv("ASSAY_COHORT_REPLAY")
	if path == "" {
		t.Skip("ASSAY_COHORT_REPLAY is not set: no replay is read")
	}
	_, c := candMustLoad(t)
	verdict, msgs := replayVerifier(candInvFile, candCohortFil, path, candRepoRoot(t), os.Stdout, func(pulls []ghPull, files map[int][]prFile) error {
		return candReplayFlow(t, c, pulls, files)
	})
	if verdict != "checked-clean" {
		t.Fatalf("replay %s:\n%s", verdict, candNames(msgs))
	}
}

// candReplayFlow drives row 13's backfill selection and row 4's reconcile over replay pulls.
func candReplayFlow(t *testing.T, c candCohort, pulls []ghPull, files map[int][]prFile) error {
	served := map[int][]map[string]any{}
	for n, fs := range files {
		served[n] = candFileMaps(fs)
	}
	partials := readDeclaredPartials(candIsoTree(t, nil), candRepo)
	m, ok, unread, reads := candMatch(t, candBrief, pulls, served, nil, partials, nil)
	for _, r := range reads {
		if _, verified := files[r]; !verified {
			return fmt.Errorf("the flow read PR #%d's file list, which has no verified files_sha256", r)
		}
	}
	if ok || unread != "" {
		return fmt.Errorf("replay backfill selection gave witness #%d (unread %q)", m.Number, unread)
	}
	for _, backfill := range []bool{false, true} {
		root := candIsoTree(t, nil)
		commitFixture(t, root)
		readme := candReadme(t, root)
		t.Setenv("GITHUB_TOKEN", "test-token")
		reads := authoringServer(t, candServedPulls(pulls), served, nil)
		args := []string{"--apply", "--root", root, "--repo", candRepo, "--json"}
		if backfill {
			args = append([]string{"--backfill"}, args...)
		}
		code, out := runReconcileCaptured(t, args...)
		if cell := cellOf(t, out, candBrief); code != reconcileOK || cell.Cell != "in-progress" || cell.MergedPR != "" || len(appliedIDs(t, out)) != 0 {
			return fmt.Errorf("replay reconcile (backfill=%v): exit %d cell %+v", backfill, code, cell)
		}
		if candReadme(t, root) != readme {
			return fmt.Errorf("replay reconcile wrote the README")
		}
		for _, r := range *reads {
			if _, verified := files[r]; !verified {
				return fmt.Errorf("the flow read PR #%d's file list, which has no verified files_sha256", r)
			}
		}
	}
	return nil
}

// ---- synthetic forge world for the hermetic controls (rows 12 and 15) -------------------

const candWorldCutoff = "2026-06-01T00:00:00Z"

// candWorldPulls is a SYNTHETIC 250-record repository, newest first: #150 carries brief 07's
// `Brief:` trailer (page 2), #20 names it only in its branch (the backfill path), every
// seventh record is open, and the rest merged before the cutoff.
func candWorldPulls() []ghPull {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var out []ghPull
	for n := 250; n >= 1; n-- {
		p := ghPull{Number: n, State: "closed", Body: fmt.Sprintf("SYNTHETIC record %d.\n", n),
			MergedAt: base.Add(time.Duration(n) * time.Hour).Format(time.RFC3339), MergeCommitSHA: fmt.Sprintf("%040x", n)}
		p.Head.SHA, p.Head.Ref = fmt.Sprintf("%040x", 1000+n), fmt.Sprintf("branch-%d", n)
		if n%7 == 0 {
			p.State, p.MergedAt, p.MergeCommitSHA = "open", "", ""
		}
		switch n {
		case 150:
			p.Body = "SYNTHETIC delivery.\n\nBrief: derived-board/07\n"
		case 20:
			p.Head.Ref = "feat/derived-board-07-x"
		}
		out = append(out, p)
	}
	return out
}

func candLater(n int) ghPull {
	p := ghPull{Number: n, State: "closed", Body: "SYNTHETIC later delivery.\n\nBrief: derived-board/07\n",
		MergedAt: "2026-07-01T00:00:00Z", MergeCommitSHA: fmt.Sprintf("%040x", n)}
	p.Head.SHA, p.Head.Ref = fmt.Sprintf("%040x", 1000+n), fmt.Sprintf("branch-%d", n)
	return p
}

func candWorldFiles() map[int][]prFile {
	return map[int][]prFile{
		150: {{Filename: "statusgen/synthetic.go", Status: "modified"}},
		20:  {{Filename: "statusgen/synthetic_old.go", Status: "added"}},
	}
}

func candPageBytes(ps []ghPull, page int) []byte {
	lo, hi := (page-1)*candPerPage, page*candPerPage
	if lo > len(ps) {
		lo = len(ps)
	}
	if hi > len(ps) {
		hi = len(ps)
	}
	b, err := json.Marshal(candServedPulls(ps[lo:hi]))
	if err != nil {
		panic(err)
	}
	return b
}

// candWriteWorld writes an inventory of ps and a cohort of its eligible candidates to dir.
func candWriteWorld(t *testing.T, dir string, ps []ghPull, files map[int][]prFile) (string, string, candInventory, candCohort) {
	t.Helper()
	var raw [][]byte
	for pg := 1; pg <= len(ps)/candPerPage+1; pg++ {
		raw = append(raw, candPageBytes(ps, pg))
	}
	inv, pulls, err := buildCandInventory(raw, "2026-06-01T00:00:00Z", "2026-06-01T00:00:10Z", candWorldCutoff)
	if err != nil {
		t.Fatal(err)
	}
	var eligible []ghPull
	for _, p := range pulls {
		if ok, _ := candEligible(p.MergedAt, candWorldCutoff); ok {
			eligible = append(eligible, p)
		}
	}
	mem := candidateMembership(eligible)
	c := candCohort{Schema: "candidate-cohort/v1", Brief: candBrief, Repo: candRepo, Cutoff: candWorldCutoff, InventorySH: inv.SnapshotSHA}
	var nums []int
	for n := range mem {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	for _, n := range nums {
		fs := files[n]
		sha := filesSHA(fs)
		for i := range inv.Records {
			if inv.Records[i].Number == n {
				inv.Records[i].FilesSHA = &sha
				c.Members = append(c.Members, candMember{Number: n, Disposition: "partial", Reason: "SYNTHETIC", Paths: mem[n],
					MergeCommitSHA: inv.Records[i].MergeCommitSHA, FilesSHA: sha, Standin: json.RawMessage(`"held"`), Files: pinnedJSON(pinFiles(fs))})
			}
		}
	}
	invPath, cohortPath := filepath.Join(dir, "inventory.json"), filepath.Join(dir, "cohort.json")
	if candWriteFile(invPath, pinnedJSON(inv)) != nil || candWriteFile(cohortPath, pinnedJSON(c)) != nil {
		t.Fatal("write world")
	}
	return invPath, cohortPath, inv, c
}

// candServe is an httptest forge serving a synthetic world.
type candServe struct {
	mu        sync.Mutex
	walk      func(walk int) []ghPull
	files     map[int][]prFile
	fullFiles bool
	gzip      bool
	fail      int // a collection page answering HTTP 500
	walks     int
	calls     int
	maxBody   int
	gzipped   int
}

func (s *candServe) start(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.calls++
		var body []byte
		switch p := r.URL.Path; {
		case p == "/repos/"+candRepo+"/pulls":
			pg, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if pg == 1 {
				s.walks++
			}
			if pg == s.fail {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			body = candPageBytes(s.walk(s.walks), pg)
		case strings.HasSuffix(p, "/files"):
			parts := strings.Split(strings.Trim(p, "/"), "/")
			n, _ := strconv.Atoi(parts[len(parts)-2])
			pg, _ := strconv.Atoi(r.URL.Query().Get("page"))
			fs := []map[string]any{}
			switch {
			case s.fullFiles:
				for i := 0; i < candPerPage; i++ {
					fs = append(fs, map[string]any{"filename": fmt.Sprintf("f%d-%d.go", pg, i), "status": "added"})
				}
			case pg == 1:
				fs = candFileMaps(s.files[n])
			}
			body, _ = json.Marshal(fs)
		default:
			http.NotFound(w, r)
			return
		}
		if len(body) > s.maxBody {
			s.maxBody = len(body)
		}
		if s.gzip && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			var z bytes.Buffer
			zw := gzip.NewWriter(&z)
			_, _ = zw.Write(body)
			_ = zw.Close()
			s.gzipped++
			w.Header().Set("Content-Encoding", "gzip")
			body = z.Bytes()
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func candEnv(m map[string]string, asked *[]string) func(string) string {
	return func(k string) string {
		if asked != nil {
			*asked = append(*asked, k)
		}
		return m[k]
	}
}

// regression: #2703
func TestRecollectControls(t *testing.T) {
	world := candWorldPulls()
	dir := t.TempDir()
	invPath, cohortPath, _, _ := candWriteWorld(t, dir, world, candWorldFiles())
	wt := candRepoRoot(t)
	env := map[string]string{"ASSAY_COHORT_RECHECK": "1", "GITHUB_TOKEN": "test-token"}
	type knobs struct {
		serve  func(*candServe)
		bounds func(*candBounds)
		env    map[string]string
		inv    string
		cohort string
	}
	run := func(t *testing.T, k knobs) (candRecollect, *candServe, string) {
		t.Helper()
		s := &candServe{walk: func(int) []ghPull { return world }, files: candWorldFiles()}
		if k.serve != nil {
			k.serve(s)
		}
		srv := s.start(t)
		b := candDefaultBounds
		if k.bounds != nil {
			k.bounds(&b)
		}
		e := map[string]string{}
		for kk, v := range env {
			e[kk] = v
		}
		for kk, v := range k.env {
			e[kk] = v
		}
		in, co := invPath, cohortPath
		if k.inv != "" {
			in, co = k.inv, k.cohort
		}
		var out bytes.Buffer
		res := candRecollectRun(candRecollectOpts{W: &out, Getenv: candEnv(e, nil), NewDoer: func() httpDoer { return srv.Client() },
			Base: srv.URL, Inv: in, Cohort: co, Worktree: wt, Bounds: b})
		return res, s, out.String()
	}
	cnc := func(t *testing.T, res candRecollect, out, want string) {
		t.Helper()
		if res.Verdict != "could-not-check" || !strings.Contains(candNames(res.Msgs), want) || strings.Contains(out, "RECOLLECT checked-clean") {
			t.Fatalf("want could-not-check naming %q, got %s:\n%s", want, res.Verdict, candNames(res.Msgs))
		}
	}

	t.Run("clean-and-rebuilt-replay", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "rebuilt.json")
		res, s, text := run(t, knobs{env: map[string]string{"ASSAY_COHORT_REPLAY_OUT": out}})
		if res.Verdict != "checked-clean" || !strings.Contains(text, "RECOLLECT checked-clean") || s.calls != 3*2+2 {
			t.Fatalf("%s calls=%d %v", res.Verdict, s.calls, res.Msgs)
		}
		var buf bytes.Buffer
		if v, msgs := compareReplay(invPath, cohortPath, out, wt, &buf, nil); v != "checked-clean" || !strings.Contains(buf.String(), "REPLAY checked-clean") {
			t.Fatalf("rebuilt replay: %s %v", v, msgs)
		}
	})
	t.Run("post-cutoff-merges-excluded", func(t *testing.T) {
		res, _, _ := run(t, knobs{serve: func(s *candServe) {
			s.walk = func(int) []ghPull { return append([]ghPull{candLater(252), candLater(251)}, world...) }
		}})
		if res.Verdict != "checked-clean" {
			t.Fatalf("a merge after the cutoff entered the cohort: %v", res.Msgs)
		}
	})
	t.Run("omitted-page", func(t *testing.T) {
		// A forged inventory without page 2, every receipt and digest recomputed so it is
		// self-consistent: only the independent walk can expose it.
		forged := append(append([]ghPull{}, world[:100]...), world[200:]...)
		fdir := t.TempDir()
		fi, fc, finv, _ := candWriteWorld(t, fdir, forged, candWorldFiles())
		if p := checkInventory(finv); len(p) > 0 {
			t.Fatalf("forged inventory is not self-consistent: %v", p)
		}
		res, _, _ := run(t, knobs{inv: fi, cohort: fc})
		if res.Verdict != "checked-failed" || !strings.Contains(candNames(res.Msgs), "PR #150 ") {
			t.Fatalf("omitted page: %s %v", res.Verdict, res.Msgs)
		}
		prev := candRecollectCompare
		candRecollectCompare = func(candInventory, candCohort, map[int]ghPull) ([]string, []string) { return nil, nil }
		res, _, _ = run(t, knobs{inv: fi, cohort: fc})
		candRecollectCompare = prev
		if res.Verdict != "checked-clean" {
			t.Fatalf("with the comparison disabled the forged inventory should pass (the control rests on it): %s", res.Verdict)
		}
	})
	t.Run("no-opt-in-zero-reads", func(t *testing.T) {
		var asked []string
		res := candRecollectRun(candRecollectOpts{W: io.Discard, Getenv: candEnv(map[string]string{"GITHUB_TOKEN": "test-token"}, &asked),
			NewDoer: func() httpDoer { t.Fatal("transport built without opt-in"); return nil }, Base: "http://127.0.0.1:1",
			Inv: invPath, Cohort: cohortPath, Worktree: wt, Bounds: candDefaultBounds})
		if !res.Skipped || !reflect.DeepEqual(asked, []string{"ASSAY_COHORT_RECHECK"}) {
			t.Fatalf("skipped=%v env reads %v", res.Skipped, asked)
		}
	})
	t.Run("missing-token", func(t *testing.T) {
		res, s, out := run(t, knobs{env: map[string]string{"GITHUB_TOKEN": ""}})
		cnc(t, res, out, "no GITHUB_TOKEN")
		if s.calls != 0 {
			t.Fatalf("%d requests without a token", s.calls)
		}
	})
	t.Run("replay-out-inside-checkout", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "link")
		if err := os.Symlink(wt, link); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{filepath.Join(wt, "statusgen", "replay.json"), filepath.Join(link, "statusgen", "replay.json")} {
			res, s, out := run(t, knobs{env: map[string]string{"ASSAY_COHORT_REPLAY_OUT": p}})
			cnc(t, res, out, "inside the checkout")
			if s.calls != 0 {
				t.Fatalf("%d requests before the refusal", s.calls)
			}
		}
	})
	t.Run("changed-old-body", func(t *testing.T) {
		res, _, out := run(t, knobs{serve: func(s *candServe) {
			w := candClone(t, world)
			w[100].Body += "edited\n" // #150
			s.walk = func(int) []ghPull { return w }
		}})
		cnc(t, res, out, "PR #150: its kept fields changed")
	})
	t.Run("changed-old-ref", func(t *testing.T) {
		res, _, out := run(t, knobs{serve: func(s *candServe) {
			w := candClone(t, world)
			w[220].Head.Ref = "renamed" // #30
			s.walk = func(int) []ghPull { return w }
		}})
		cnc(t, res, out, "PR #30: its kept fields changed")
	})
	t.Run("walks-disagree", func(t *testing.T) {
		res, _, out := run(t, knobs{serve: func(s *candServe) {
			w := candClone(t, world)
			w[150].Body += "edited\n" // #100
			s.walk = func(n int) []ghPull {
				if n == 2 {
					return w
				}
				return world
			}
		}})
		cnc(t, res, out, "the two walks disagree on PR #100")
	})
	t.Run("file-list-mismatch-writes-no-replay", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "rebuilt.json")
		res, _, text := run(t, knobs{env: map[string]string{"ASSAY_COHORT_REPLAY_OUT": out}, serve: func(s *candServe) {
			s.files = map[int][]prFile{150: candWorldFiles()[150], 20: {{Filename: "docs/other.md", Status: "added"}}}
		}})
		cnc(t, res, text, "PR #20: its file list no longer hashes")
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatal("a rebuilt replay was written before every file list matched")
		}
	})
	t.Run("file-list-at-forge-cap", func(t *testing.T) {
		res, _, out := run(t, knobs{serve: func(s *candServe) { s.fullFiles = true }})
		cnc(t, res, out, "3000-file cap")
	})
	t.Run("http-error", func(t *testing.T) {
		res, _, out := run(t, knobs{serve: func(s *candServe) { s.fail = 2 }})
		cnc(t, res, out, "HTTP 500")
	})
	clean, s0, _ := run(t, knobs{})
	if clean.Verdict != "checked-clean" {
		t.Fatalf("baseline: %v", clean.Msgs)
	}
	total, maxBody := clean.Doer.total, int64(s0.maxBody)
	for _, z := range []bool{false, true} {
		t.Run(fmt.Sprint("response-limit/gzip=", z), func(t *testing.T) {
			res, s, _ := run(t, knobs{serve: func(s *candServe) { s.gzip = z }, bounds: func(b *candBounds) { b.PerResp = maxBody }})
			if res.Verdict != "checked-clean" {
				t.Fatalf("at the limit: %v", res.Msgs)
			}
			res, s, out := run(t, knobs{serve: func(s *candServe) { s.gzip = z }, bounds: func(b *candBounds) { b.PerResp = maxBody - 1 }})
			cnc(t, res, out, "decoded-response limit")
			if z && s.gzipped == 0 {
				t.Fatal("no compressed response was served")
			}
		})
	}
	t.Run("aggregate-limit", func(t *testing.T) {
		if res, _, _ := run(t, knobs{bounds: func(b *candBounds) { b.Total = total }}); res.Verdict != "checked-clean" {
			t.Fatalf("at the limit: %v", res.Msgs)
		}
		res, _, out := run(t, knobs{bounds: func(b *candBounds) { b.Total = total - 1 }})
		cnc(t, res, out, "aggregate limit")
	})
	t.Run("request-bound", func(t *testing.T) {
		res, _, out := run(t, knobs{bounds: func(b *candBounds) { b.Requests = 3*2 + 1 }})
		cnc(t, res, out, "request bound")
	})
	t.Run("time-bound", func(t *testing.T) {
		res, _, out := run(t, knobs{bounds: func(b *candBounds) { b.Time = time.Nanosecond }})
		cnc(t, res, out, "time bound")
	})
	t.Run("page-bound", func(t *testing.T) {
		res, _, out := run(t, knobs{bounds: func(b *candBounds) { b.MaxPages = 2 }})
		cnc(t, res, out, "not provably complete")
	})
	t.Run("transport-refusals", func(t *testing.T) {
		s := &candServe{walk: func(int) []ghPull { return world }, files: candWorldFiles()}
		srv := s.start(t)
		d := &candDoer{inner: srv.Client(), origin: srv.URL, repo: candRepo, b: candDefaultBounds, start: time.Now(), files: map[int]bool{150: true}}
		for _, tc := range []struct{ method, url, want string }{
			{http.MethodGet, srv.URL + "/repos/" + candRepo + "/issues?state=all", "neither the pulls collection"},
			{http.MethodGet, srv.URL + "/repos/" + candRepo + "/pulls/150/reviews", "neither the pulls collection"},
			{http.MethodGet, srv.URL + "/repos/" + candRepo + "/pulls/21/files?per_page=100&page=1", "not an allowed eligible record"},
			{http.MethodGet, srv.URL + "/repos/" + candRepo + "/pulls/150/files?per_page=100&page=31", "outside the 30-page bound"},
			{http.MethodGet, srv.URL + "/repos/" + candRepo + "/pulls?state=open&per_page=100&page=1", "outside the 100-page"},
			{http.MethodGet, srv.URL + "/repos/" + candRepo + "/pulls?state=all&per_page=100&page=101", "outside the 100-page"},
			{http.MethodPost, srv.URL + "/repos/" + candRepo + "/pulls", "method POST"},
			{http.MethodGet, "http://127.0.0.2:9/repos/" + candRepo + "/pulls?state=all&per_page=100&page=1", "origin"},
		} {
			req, _ := http.NewRequest(tc.method, tc.url, nil)
			if _, err := d.Do(req); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s %s: %v", tc.method, tc.url, err)
			}
		}
		if s.calls != 0 {
			t.Fatalf("%d refused requests reached the forge", s.calls)
		}
		// A file list for an eligible record whose files_sha256 is null is never admitted:
		// candRecollectRun allows only records with a recorded hash.
		inv, _ := candLoad[candInventory](invPath)
		for _, r := range inv.Records {
			if r.FilesSHA != nil && r.Number != 150 && r.Number != 20 {
				t.Fatalf("record #%d carries files_sha256", r.Number)
			}
		}
	})
}

// candReplayOf renders an inventory world as a replay of kind; rebuilt keeps eligible records only.
func candReplayOf(t *testing.T, ps []ghPull, inv candInventory, files map[int][]prFile, kind string) candReplay {
	t.Helper()
	rp := candReplay{Schema: "candidate-replay/v1", Kind: kind}
	for i, p := range ps {
		e := inv.Records[i]
		if kind == "rebuilt" {
			if ok, _ := candEligible(p.MergedAt, inv.Cutoff); !ok {
				continue
			}
		}
		r := candReplayRecord{Page: e.Page, Position: e.Position, Record: pinPull(p)}
		if fs, ok := files[p.Number]; ok && e.FilesSHA != nil {
			pf := pinFiles(fs)
			r.Files = &pf
		}
		rp.Records = append(rp.Records, r)
	}
	return rp
}

// regression: #2703
func TestReplayBindingControls(t *testing.T) {
	inv0, _ := candMustLoad(t)
	candCheckKnownAnswer(t, inv0.KnownAnswer)
	world := candWorldPulls()
	files := candWorldFiles()
	dir := t.TempDir()
	invPath, cohortPath, inv, _ := candWriteWorld(t, dir, world, files)
	wt := candRepoRoot(t)
	check := func(t *testing.T, rp candReplay, invP string) (string, []string, string) {
		t.Helper()
		p := filepath.Join(t.TempDir(), "replay.json")
		if err := os.WriteFile(p, pinnedJSON(rp), 0o600); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		v, msgs := replayVerifier(invP, cohortPath, p, wt, &buf, nil)
		return v, msgs, buf.String()
	}
	for _, kind := range []string{"captured", "rebuilt"} {
		if v, msgs, out := check(t, candReplayOf(t, world, inv, files, kind), invPath); v != "checked-clean" || !strings.Contains(out, "REPLAY checked-clean") {
			t.Fatalf("unmutated %s pair: %s %v", kind, v, msgs)
		}
	}
	idx := func(rp candReplay, n int) int {
		for i, r := range rp.Records {
			if r.Record.Number == n {
				return i
			}
		}
		t.Fatalf("#%d not in replay", n)
		return -1
	}
	type mut struct {
		name, kind, want string
		f                func(*candReplay)
	}
	muts := []mut{
		{"body-byte", "captured", "PR #150: its replay record does not hash", func(r *candReplay) {
			b := []byte(r.Records[idx(*r, 150)].Record.Body)
			b[0] ^= 1
			r.Records[idx(*r, 150)].Record.Body = string(b)
		}},
		{"head-ref", "captured", "PR #20: its replay record does not hash", func(r *candReplay) { r.Records[idx(*r, 20)].Record.Head.Ref = "x" }},
		{"order-swap", "captured", "replay record 1 is PR #249", func(r *candReplay) {
			a, b := r.Records[0], r.Records[1]
			a.Page, a.Position, b.Page, b.Position = b.Page, b.Position, a.Page, a.Position
			r.Records[0], r.Records[1] = b, a
		}},
		{"extra-record", "captured", "PR #900 is an extra record", func(r *candReplay) {
			r.Records = append(r.Records, candReplayRecord{Page: 3, Position: 51, Record: pinPull(candLater(900))})
		}},
		{"dropped-record", "captured", "PR #1 is missing", func(r *candReplay) { r.Records = r.Records[:len(r.Records)-1] }},
		{"substituted-files", "captured", "PR #150: its replay file list does not hash", func(r *candReplay) {
			f := []pinnedFile{{Filename: "docs/x.md", Status: "added"}}
			r.Records[idx(*r, 150)].Files = &f
		}},
		{"rebuilt-missing-eligible", "rebuilt", "eligible PR #1 is missing", func(r *candReplay) { r.Records = r.Records[:len(r.Records)-1] }},
		{"rebuilt-ineligible", "rebuilt", "PR #7 in the rebuilt replay is not an eligible", func(r *candReplay) {
			r.Records = append(r.Records, candReplayRecord{Page: 3, Position: 44, Record: pinPull(world[243])})
		}},
		{"no-kind", "", "only captured or rebuilt", func(*candReplay) {}},
		{"dup-captured", "captured", "PR #150 appears twice", func(r *candReplay) { r.Records[idx(*r, 151)].Record.Number = 150 }},
		{"dup-rebuilt", "rebuilt", "PR #150 appears twice", func(r *candReplay) { r.Records = append(r.Records, r.Records[idx(*r, 150)]) }},
		{"rebuilt-missing-files", "rebuilt", "PR #20: its file list is missing", func(r *candReplay) { r.Records[idx(*r, 20)].Files = nil }},
		{"files-for-null-sha", "rebuilt", "PR #22: the replay carries a file list for a record whose files_sha256 is null", func(r *candReplay) {
			f := []pinnedFile{{Filename: "statusgen/a.go", Status: "added"}}
			r.Records[idx(*r, 22)].Files = &f
		}},
	}
	for _, m := range muts {
		t.Run(m.name, func(t *testing.T) {
			k := m.kind
			if k == "" {
				k = "captured"
			}
			rp := candReplayOf(t, world, inv, files, k)
			rp.Kind = m.kind
			m.f(&rp)
			v, msgs, out := check(t, rp, invPath)
			if v != "could-not-check" || !strings.Contains(candNames(msgs), m.want) || strings.Contains(out, "REPLAY") {
				t.Fatalf("want could-not-check naming %q, got %s:\n%s", m.want, v, candNames(msgs))
			}
		})
	}
	t.Run("rebuilt-checked-against-snapshot", func(t *testing.T) {
		// Every record, in collection order, under kind rebuilt: it reproduces snapshot_sha256
		// but not eligible_sha256, so a verifier that read the wrong digest would pass it.
		rp := candReplayOf(t, world, inv, files, "captured")
		rp.Kind = "rebuilt"
		v, msgs, _ := check(t, rp, invPath)
		if v != "could-not-check" || !strings.Contains(candNames(msgs), "is not an eligible inventory record") {
			t.Fatalf("%s %v", v, msgs)
		}
	})
	t.Run("snapshot-edited", func(t *testing.T) {
		ed := candClone(t, inv)
		ed.SnapshotSHA = strings.Repeat("0", 64)
		p := filepath.Join(t.TempDir(), "inv.json")
		if err := os.WriteFile(p, pinnedJSON(ed), 0o600); err != nil {
			t.Fatal(err)
		}
		v, msgs, _ := check(t, candReplayOf(t, world, ed, files, "captured"), p)
		if v != "could-not-check" || !strings.Contains(candNames(msgs), "snapshot_sha256") {
			t.Fatalf("%s %v", v, msgs)
		}
	})
	t.Run("symlink-into-checkout", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "link")
		if err := os.Symlink(filepath.Join(wt, "statusgen", candCohortFil), link); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if v, msgs := replayVerifier(invPath, cohortPath, link, wt, &buf, nil); v != "could-not-check" || !strings.Contains(candNames(msgs), "inside the checkout") {
			t.Fatalf("replay input via symlink: %s %v", v, msgs)
		}
		dlink := filepath.Join(t.TempDir(), "dlink")
		if err := os.Symlink(wt, dlink); err != nil {
			t.Fatal(err)
		}
		res := candRecollectRun(candRecollectOpts{W: io.Discard,
			Getenv:  candEnv(map[string]string{"ASSAY_COHORT_RECHECK": "1", "GITHUB_TOKEN": "x", "ASSAY_COHORT_REPLAY_OUT": filepath.Join(dlink, "out.json")}, nil),
			NewDoer: func() httpDoer { t.Fatal("transport built for a refused output path"); return nil },
			Base:    "http://127.0.0.1:1", Inv: invPath, Cohort: cohortPath, Worktree: wt, Bounds: candDefaultBounds})
		if res.Verdict != "could-not-check" || !strings.Contains(candNames(res.Msgs), "inside the checkout") {
			t.Fatalf("replay output via symlink: %v", res)
		}
	})
	t.Run("accept-all-verifier-goes-red", func(t *testing.T) {
		// The control rests on the comparison: an accept-all verifier passes a mutated replay.
		prev := replayVerifier
		replayVerifier = func(_, _, _, _ string, w io.Writer, _ func([]ghPull, map[int][]prFile) error) (string, []string) {
			fmt.Fprintln(w, "REPLAY checked-clean")
			return "checked-clean", nil
		}
		defer func() { replayVerifier = prev }()
		rp := candReplayOf(t, world, inv, files, "captured")
		rp.Records = rp.Records[1:]
		if v, _, _ := check(t, rp, invPath); v != "checked-clean" {
			t.Fatal("seam not wired")
		}
	})
}
