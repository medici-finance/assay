package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type cohortDisposition struct {
	Brief          string
	Number         int
	Covers         string
	BriefPath      string `json:"brief_path"`
	BriefSHA       string `json:"brief_sha256"`
	Obligations    string
	ObligationsSHA string `json:"obligations_sha256"`
}
type cohortRecord struct {
	MatcherInput bool `json:"matcher_input"`
	Pull         ghPull
	URL          string
	Files        []prFile
	MergePaths   []string `json:"merge_paths"`
	PathsSHA     string   `json:"merge_paths_sha256"`
	Scope        string   `json:"scope_excerpt"`
	BodySHA      string   `json:"body_sha256"`
}
type cohortFixture struct {
	Repair     struct{ Base, Delivery, Before, After string } `json:"delivery_repair"`
	Source     string
	BoardBase  string `json:"board_base"`
	BoardHead  string `json:"board_head"`
	BoardDiff  string `json:"board_diff"`
	DiffSHA    string `json:"board_diff_sha256"`
	BoardPairs []struct {
		Brief   string `json:"brief_excerpt"`
		Witness string `json:"witness_excerpt"`
	} `json:"board_body_pairs"`
	Reviews      []struct{ URL, Head, Table string }
	Dispositions []cohortDisposition
	Records      []cohortRecord
	Evidence     cohortRecord `json:"evidence_counterexample"`
	Collection   struct {
		Count, Cutoff      int
		Complete           bool
		Unread             []string
		Trailer, Potential []int
		AsOf               string `json:"as_of"`
	}
}

func cohortLoad(t *testing.T) cohortFixture {
	t.Helper()
	var f cohortFixture
	b, e := os.ReadFile("testdata/witness-admission/board-cohort.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	return f
}
func cohortHash(s string) string        { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }
func cohortKey(id string, n int) string { return fmt.Sprintf("%s#%d", canonicalBriefKey(id), n) }
func cohortRecordAt(t *testing.T, f cohortFixture, n int) cohortRecord {
	t.Helper()
	for _, r := range f.Records {
		if r.Pull.Number == n {
			return r
		}
	}
	t.Fatalf("missing PR #%d", n)
	return cohortRecord{}
}
func cohortPulls(f cohortFixture) []ghPull {
	var p []ghPull
	for _, r := range f.Records {
		if r.MatcherInput {
			p = append(p, r.Pull)
		}
	}
	return p
}

// Membership is extracted from the pinned tree delta and independently joined
// to the board producer's witness excerpts and BOTH review tables. No delivery
// declaration or expected disposition supplies membership.
func cohortMembers(f cohortFixture) (map[string]int, error) {
	rows := map[string]bool{}
	stream := ""
	for _, l := range strings.Split(f.BoardDiff, "\n") {
		if strings.HasPrefix(l, "+++ b/docs/streams/") {
			stream = strings.TrimSuffix(strings.TrimPrefix(l, "+++ b/docs/streams/"), "/README.md")
		}
		if strings.HasPrefix(l, "+| ") {
			cells := strings.Split(l, "|")
			if len(cells) < 7 {
				return nil, fmt.Errorf("bad board row")
			}
			id := stream + "/" + strings.TrimSpace(cells[1])
			if rows[id] {
				return nil, fmt.Errorf("duplicate board %s", id)
			}
			rows[id] = true
		}
	}
	members := map[string]int{}
	rx := regexp.MustCompile(`PR #(\d+) `)
	for _, p := range f.BoardPairs {
		id := canonicalBriefKey(p.Brief)
		m := rx.FindStringSubmatch(p.Witness)
		if len(m) != 2 {
			return nil, fmt.Errorf("missing witness %s", id)
		}
		n, _ := strconv.Atoi(m[1])
		if !rows[id] || members[id] != 0 {
			return nil, fmt.Errorf("extra/duplicate pair %s#%d", id, n)
		}
		members[id] = n
	}
	for id := range rows {
		if members[id] == 0 {
			return nil, fmt.Errorf("missing board witness %s", id)
		}
	}
	if len(members) != 10 {
		return nil, fmt.Errorf("board members=%d, want 10", len(members))
	}
	if len(f.Reviews) != 2 {
		return nil, fmt.Errorf("review manifests=%d", len(f.Reviews))
	}
	for _, r := range f.Reviews {
		if r.Head != f.BoardHead {
			return nil, fmt.Errorf("review head mismatch %s", r.URL)
		}
		seen := map[string]int{}
		for _, l := range strings.Split(strings.TrimSpace(r.Table), "\n") {
			c := strings.Split(l, "|")
			if len(c) < 4 {
				return nil, fmt.Errorf("bad review row")
			}
			id := strings.TrimSpace(c[1])
			n, e := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(c[2]), "#"))
			if e != nil || seen[id] != 0 || members[id] != n {
				return nil, fmt.Errorf("review mismatch %s#%d", id, n)
			}
			seen[id] = n
		}
		for id, n := range members {
			if seen[id] != n {
				return nil, fmt.Errorf("review missing %s#%d", id, n)
			}
		}
	}
	return members, nil
}

// This is an evidence identity join, not a prose classifier. An additional
// admitted matcher candidate has no default disposition, even if older than
// a known partial. Production admission remains the existing owner.
func cohortCoverage(f cohortFixture, prs []PRRecord, d declaredPartials) error {
	members, e := cohortMembers(f)
	if e != nil {
		return e
	}
	expected := map[string]cohortDisposition{}
	for _, v := range f.Dispositions {
		k := cohortKey(v.Brief, v.Number)
		if _, ok := expected[k]; ok {
			return fmt.Errorf("duplicate disposition %s", k)
		}
		if members[v.Brief] != v.Number {
			return fmt.Errorf("extra disposition %s", k)
		}
		expected[k] = v
	}
	for id, n := range members {
		if _, ok := expected[cohortKey(id, n)]; !ok {
			return fmt.Errorf("missing disposition %s", cohortKey(id, n))
		}
	}
	seen := map[string]bool{}
	for _, pr := range prs {
		id := canonicalBriefKey(pr.BriefRef)
		if members[id] == 0 || pr.State != prMerged {
			continue
		}
		k := cohortKey(id, pr.Number)
		v, ok := expected[k]
		if !ok {
			return fmt.Errorf("unresolved candidate %s", k)
		}
		if seen[k] {
			return fmt.Errorf("duplicate candidate %s", k)
		}
		seen[k] = true
		if v.Covers != "full" && v.Covers != "partial" {
			return fmt.Errorf("unknown disposition %s", k)
		}
		if d.partial(pr.Number, id) != (v.Covers == "partial") {
			return fmt.Errorf("source disposition mismatch %s", k)
		}
		found := false
		for _, r := range f.Records {
			if r.Pull.Number == pr.Number {
				found = true
				paths := make([]string, 0, len(r.Files))
				for _, file := range r.Files {
					paths = append(paths, file.Filename)
				}
				sort.Strings(paths)
				if r.Pull.MergeCommitSHA != pr.MergeSHA || len(r.Files) == 0 || !reflect.DeepEqual(paths, r.MergePaths) || cohortHash(strings.Join(paths, "\n")+"\n") != r.PathsSHA || len(r.BodySHA) != 64 || r.Scope == "" || r.URL != fmt.Sprintf("https://github.com/medici-finance/assay/pull/%d", pr.Number) {
					return fmt.Errorf("provenance mismatch %s", k)
				}
			}
		}
		if !found {
			return fmt.Errorf("missing evidence %s", k)
		}
	}
	for k := range expected {
		if !seen[k] {
			return fmt.Errorf("missing candidate %s", k)
		}
	}
	return nil
}
func cohortWrite(t *testing.T, p, s string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(s), 0644); e != nil {
		t.Fatal(e)
	}
}
func cohortRead(t *testing.T, p string) string {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func cohortRoot(t *testing.T, f cohortFixture) string {
	t.Helper()
	root := t.TempDir()
	template := cohortRead(t, "testdata/designgate/docs/streams/dg/brief-03-model-scoped.md")
	cohortWrite(t, filepath.Join(root, "docs/streams/graph-repos.yaml"), "schema: graph-repos-v1\ncell: assay\nrepos:\n  assay: {cell: assay, repo: medici-finance/assay}\n")
	boards := map[string]string{}
	for _, v := range f.Dispositions {
		s, n, _ := briefStreamNum(v.Brief)
		raw := strings.ReplaceAll(template, "dg/03", v.Brief)
		name := filepath.Base(v.BriefPath)
		if v.Brief == "statusgen/26" {
			raw = cohortRead(t, filepath.Join("..", v.BriefPath))
			name = filepath.Base(v.BriefPath)
		}
		cohortWrite(t, filepath.Join(root, "docs/streams", s, name), raw)
		if boards[s] == "" {
			boards[s] = "---\nstream: " + s + "\nstatus: active\nboard: generated\n---\n\n# Cohort\n\n| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|---|---|---|---|---|---|\n"
		}
		boards[s] += fmt.Sprintf("| %s | [Cohort](%s) | 0 | S | in-progress | — | — |\n", n, name)
	}
	for s, b := range boards {
		cohortWrite(t, filepath.Join(root, "docs/streams", s, "README.md"), b)
	}
	return root
}
func cohortServer(t *testing.T, f cohortFixture, fail map[int]bool) *[]int {
	var pulls []map[string]any
	files := map[int][]map[string]any{}
	for _, r := range f.Records {
		if !r.MatcherInput {
			continue
		}
		b, _ := json.Marshal(r.Pull)
		var p map[string]any
		_ = json.Unmarshal(b, &p)
		pulls = append(pulls, p)
		for _, v := range r.Files {
			files[r.Pull.Number] = append(files[r.Pull.Number], map[string]any{"filename": v.Filename, "status": v.Status})
		}
	}
	return authoringServer(t, pulls, files, fail)
}
func cohortInput(t *testing.T, f cohortFixture, root string) (LifecycleInput, *authoringClassifier, []string) {
	t.Helper()
	ids, e := reconcileBriefIdents(root)
	if e != nil {
		t.Fatal(e)
	}
	in := LifecycleInput{Briefs: ids, LookedAt: true, PRs: prRecordsFromPulls(cohortPulls(f))}
	cl, u, unread := admitPRWitnesses(&in, reconcileGHClient("test-token"), "medici-finance/assay", root, io.Discard)
	mergeUndecided(&in, u)
	return in, cl, unread
}
func cohortAssert(t *testing.T, f cohortFixture, cells []BriefCell) {
	t.Helper()
	for _, v := range f.Dispositions {
		found := false
		for _, c := range cells {
			if canonicalBriefKey(c.ID) != v.Brief {
				continue
			}
			found = true
			want := "implemented"
			if v.Covers == "partial" {
				want = "in-progress"
			}
			if c.Cell != want {
				t.Errorf("%s#%d: got %s want %s", v.Brief, v.Number, c.Cell, want)
			}
			if v.Covers == "partial" && c.MergedPR != "" {
				t.Errorf("partial witness %s", c.MergedPR)
			}
			if v.Covers == "full" && !strings.Contains(c.MergedPR, fmt.Sprintf("PR #%d (merged %s)", v.Number, shortSHA(cohortRecordAt(t, f, v.Number).Pull.MergeCommitSHA))) {
				t.Errorf("wrong full provenance %+v", c)
			}
		}
		if !found {
			t.Errorf("missing cell %s", v.Brief)
		}
	}
}

// regression: #2703
func TestBoardCohortSource(t *testing.T) {
	f := cohortLoad(t)
	if cohortHash(f.BoardDiff) != f.DiffSHA {
		t.Fatal("board diff digest mismatch")
	}
	members, e := cohortMembers(f)
	if e != nil {
		t.Fatal(e)
	}
	if len(members) != 10 {
		t.Fatal(members)
	}
	d := readDeclaredPartials("..", "medici-finance/assay")
	prs := prRecordsFromPulls(cohortPulls(f))
	t.Run("coverage", func(t *testing.T) {
		if e := cohortCoverage(f, prs, d); e != nil {
			t.Fatal(e)
		}
	})
	for _, v := range f.Dispositions {
		if cohortHash(v.Obligations) != v.ObligationsSHA || v.Obligations == "" {
			t.Fatalf("obligations integrity %s#%d", v.Brief, v.Number)
		}
	}
	var trailer, potential []int
	for _, p := range prsForCohort(prs, "statusgen/26") {
		if p.State == prMerged {
			trailer = append(trailer, p.Number)
		}
	}
	for _, p := range backfillCandidates(cohortPulls(f), "statusgen", "26") {
		potential = append(potential, p.Number)
	}
	if !reflect.DeepEqual(trailer, f.Collection.Trailer) || !reflect.DeepEqual(potential, f.Collection.Potential) || !f.Collection.Complete || len(f.Collection.Unread) != 0 || f.Collection.Count < len(f.Records) || f.Collection.Cutoff < 3803 || f.Collection.AsOf == "" {
		t.Fatal("pickup candidate receipt mismatch")
	}
	markDeclaredPartials(prs, d, nil)
	c := deriveSingle(t, BriefIdent{ID: "statusgen/26"}, LifecycleInput{LookedAt: true, PRs: prsForCohort(prs, "statusgen/26")})
	if c.Cell != "in-progress" || c.MergedPR != "" {
		t.Fatalf("statusgen/26#3759 false promotion: %+v", c)
	}
	for _, kind := range []string{"missing", "extra", "duplicate", "mismatch"} {
		t.Run(kind, func(t *testing.T) {
			m := cohortLoad(t)
			switch kind {
			case "missing":
				m.BoardPairs = m.BoardPairs[1:]
			case "extra":
				m.BoardPairs[0].Brief = "assay:assay:extra:99"
			case "duplicate":
				m.BoardPairs = append(m.BoardPairs, m.BoardPairs[0])
			case "mismatch":
				m.BoardPairs[0].Witness = "[PR #1 (merged abcdef0)]"
			}
			if _, e := cohortMembers(m); e == nil {
				t.Fatal("membership mutation escaped")
			}
		})
	}
}
func prsForCohort(prs []PRRecord, id string) []PRRecord {
	var out []PRRecord
	for _, p := range prs {
		if canonicalBriefKey(p.BriefRef) == id {
			out = append(out, p)
		}
	}
	return out
}

// regression: #2703
func TestBoardCohortFlow(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	f := cohortLoad(t)
	root := cohortRoot(t, f)
	commitFixture(t, root)
	cohortServer(t, f, nil)
	in, cl, unread := cohortInput(t, f, root)
	if len(unread) != 0 {
		t.Fatal(unread)
	}
	cells := DeriveLifecycle(in)
	cohortAssert(t, f, cells)
	for _, c := range cells {
		if canonicalBriefKey(c.ID) == "statusgen/26" {
			a, b := backfillBase(c, in)
			if a || b {
				t.Fatal("partial trailer enters fallback")
			}
		}
	}
	folded, u := applyReconcileBackfillWith(cells, in, cohortPulls(f), true, nil, cl.nonDelivery)
	if len(u) != 0 || !reflect.DeepEqual(cells, folded) {
		t.Fatalf("ineligible fallback changed cells: %v", u)
	}
	code, out := runReconcileCaptured(t, "--backfill", "--apply", "--root", root, "--repo", "medici-finance/assay", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d: %s", code, out)
	}
	var res reconcileResult
	if e := json.Unmarshal([]byte(out), &res); e != nil {
		t.Fatal(e)
	}
	cohortAssert(t, f, res.Briefs)
	if len(res.Applied) != 9 {
		t.Fatalf("applied=%d, want nine full rows", len(res.Applied))
	}
	for _, a := range res.Applied {
		if canonicalBriefKey(a.ID) == "statusgen/26" {
			t.Fatal("partial written")
		}
		if strings.Replace(a.RowBefore, "| in-progress |", "| implemented |", 1) != a.RowAfter {
			t.Fatal("protected cell changed")
		}
	}
	// Reset only isolated board rows, then drive the actual drift consumer.
	root = cohortRoot(t, f)
	streams, _, e := loadStreams(root)
	if e != nil {
		t.Fatal(e)
	}
	ef, e := os.CreateTemp(t.TempDir(), "stderr")
	if e != nil {
		t.Fatal(e)
	}
	defer ef.Close()
	notices := regenDriftNotices(root, streams, "medici-finance/assay", false, "", ef)
	seen := map[string]bool{}
	for _, n := range notices {
		if strings.Contains(n, "statusgen/26") {
			t.Fatal(n)
		}
		for _, v := range f.Dispositions {
			if strings.Contains(n, v.Brief) && strings.Contains(n, fmt.Sprintf("PR #%d ", v.Number)) {
				seen[v.Brief] = true
			}
		}
	}
	if len(seen) != 9 {
		t.Fatalf("full drift witnesses=%v; notices=%v", seen, notices)
	}
	// Separate legitimate historical fixture: do not remove the real 3759 trailer.
	t.Run("historical", TestPartialBackfillSkips)
}

// regression: #2703
func TestBoardCohortPlant(t *testing.T) {
	f := cohortLoad(t)
	d := readDeclaredPartials("..", "medici-finance/assay")
	prs := prRecordsFromPulls(cohortPulls(f))
	plant := cohortRecordAt(t, f, 3171).Pull
	plant.Number = 97
	plant.MergedAt = "2026-01-01T00:00:00Z"
	prs = append(prs, prRecordsFromPulls([]ghPull{plant})...)
	if d.partial(97, "apps-installer/10") {
		t.Fatal("plant was predeclared")
	}
	markDeclaredPartials(prs, d, nil)
	if known := prsForCohort(prs, "statusgen/26"); len(known) != 1 || !known[0].Partial {
		t.Fatal("newer known partial was not excluded")
	}
	e := cohortCoverage(f, prs, d)
	if e == nil || !strings.Contains(e.Error(), "apps-installer/10#97") {
		t.Fatalf("coverage accepted older unresolved second site: %v", e)
	}
	t.Log(e)
	// Independently bypass the coverage join: the production reader still holds
	// the real declared partial, while an undeclared copy reproduces promotion.
	p := prsForCohort(prs, "statusgen/26")
	markDeclaredPartials(p, d, nil)
	c := deriveSingle(t, BriefIdent{ID: "statusgen/26"}, LifecycleInput{LookedAt: true, PRs: p})
	if c.Cell != "in-progress" || c.MergedPR != "" {
		t.Fatal(c)
	}
	root := cohortRoot(t, f)
	path := filepath.Join(root, cohortPath(f, "statusgen/26"))
	raw := cohortRead(t, path)
	cohortWrite(t, path, cohortOmit(raw))
	d = readDeclaredPartials(root, "medici-finance/assay")
	p = prsForCohort(prRecordsFromPulls(cohortPulls(f)), "statusgen/26")
	markDeclaredPartials(p, d, nil)
	c = deriveSingle(t, BriefIdent{ID: "statusgen/26"}, LifecycleInput{LookedAt: true, PRs: p})
	if c.Cell != "implemented" || !strings.Contains(c.MergedPR, "PR #3759 ") {
		t.Fatalf("omission no longer reproduces: %+v", c)
	}
}
func cohortPath(f cohortFixture, id string) string {
	for _, d := range f.Dispositions {
		if d.Brief == id {
			return d.BriefPath
		}
	}
	return ""
}
func cohortOmit(s string) string {
	s = strings.Replace(s, "  - {in: \"assay#3759\", covers: partial, note: \"sub-split (a); snapshot reuse, workloads and Verify 2 remain owed\"}\n", "", 1)
	// Remove an empty delivery block, but retain later entries regardless of order.
	return regexp.MustCompile(`(?m)^delivery:\n([^ \t\n])`).ReplaceAllString(s, "$1")
}

// Only the immutable delivery snapshots carry the metadata-only byte proof.
// Live brief maintenance is tested separately through the production readers.
func cohortRepairEvidence(f cohortFixture) error {
	if f.Repair.Base != f.Source || f.Repair.Delivery != "92315c7fa0f0b1e9a5e66a3c6dfdabc8a1d0cde7" {
		return fmt.Errorf("delivery snapshot revision mismatch")
	}
	for _, v := range f.Dispositions {
		if v.Brief != "statusgen/26" {
			continue
		}
		if cohortHash(f.Repair.Before) != v.BriefSHA || !strings.Contains(f.Repair.Before, v.Obligations) {
			return fmt.Errorf("delivery snapshot obligations mismatch")
		}
		normalized := strings.Replace(cohortOmit(f.Repair.After), "version: 3\n", "version: 2\n", 1)
		if normalized != f.Repair.Before {
			return fmt.Errorf("delivery-time repair changed bytes outside declaration/version")
		}
		return nil
	}
	return fmt.Errorf("missing statusgen/26 delivery evidence")
}

// regression: #2703
func TestBoardCohortFloors(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	f := cohortLoad(t)
	for _, kind := range []string{"malformed", "unresolved", "files", "provenance"} {
		t.Run(kind, func(t *testing.T) {
			root := cohortRoot(t, f)
			path := filepath.Join(root, cohortPath(f, "statusgen/26"))
			raw := cohortRead(t, path)
			fail := map[int]bool{}
			switch kind {
			case "malformed":
				cohortWrite(t, path, strings.Replace(raw, "assay#3759", "#3759", 1))
			case "unresolved":
				cohortWrite(t, path, strings.Replace(raw, "assay#3759", "unknown#3759", 1))
			case "files":
				fail[1196] = true
			}
			cohortServer(t, f, fail)
			in, _, unread := cohortInput(t, f, root)
			if kind == "provenance" {
				prs := prRecordsFromPulls(cohortPulls(f))
				for i := range prs {
					if prs[i].Number == 1196 {
						prs[i].MergeSHA = "wrong"
					}
				}
				e := cohortCoverage(f, prs, readDeclaredPartials(root, "medici-finance/assay"))
				if e == nil || !strings.Contains(e.Error(), "desk-tools/27#1196") {
					t.Fatalf("provenance mutation escaped: %v", e)
				}
				return
			}
			if len(unread) == 0 || reconcileWriteRefusal(in, "", unread) == "" {
				t.Fatal("missing refusal")
			}
			unknown := false
			for _, c := range DeriveLifecycle(in) {
				if c.Cell == "unknown" {
					unknown = true
				}
			}
			if !unknown {
				t.Fatal("unread input not unknown")
			}
			before := cohortBoards(t, root)
			code, _ := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "medici-finance/assay", "--json")
			if code != reconcileCouldNotCheck || !reflect.DeepEqual(before, cohortBoards(t, root)) {
				t.Fatalf("refusal writes or exit=%d", code)
			}
		})
	}
	// The delivery-time byte invariant belongs to immutable evidence, not the
	// living brief. Current declarations still run through the real readers.
	t.Run("delivery-snapshot", func(t *testing.T) {
		if err := cohortRepairEvidence(f); err != nil {
			t.Fatal(err)
		}
		changed := f
		changed.Repair.After += "\nrewritten delivery-time obligation\n"
		if cohortRepairEvidence(changed) == nil {
			t.Fatal("changed delivery-time evidence accepted")
		}
	})
	t.Run("later-maintenance", func(t *testing.T) {
		root := cohortRoot(t, f)
		path := filepath.Join(root, cohortPath(f, "statusgen/26"))
		raw := cohortRead(t, path)
		// Later comment, version, Evidence and another partial entry are legitimate
		// maintenance. None changes the recorded #3759 delivery's disposition.
		raw = regexp.MustCompile(`(?m)^version: [0-9]+$`).ReplaceAllString(raw, "version: 100")
		raw = strings.Replace(raw, "delivery:\n", "delivery:\n  - {in: \"assay#4002\", covers: partial, note: \"later synthetic slice\"}\n", 1)
		raw = strings.Replace(raw, "## Evidence\n", "## Evidence\n\nLater independent evidence is recorded here.\n", 1)
		cohortWrite(t, path, raw+"\n<!-- Later maintenance note. -->\n")
		d := readDeclaredPartials(root, "medici-finance/assay")
		if !d.partial(3759, "statusgen/26") || !d.partial(4002, "statusgen/26") {
			t.Fatal("maintenance lost a partial declaration")
		}
		prs := prRecordsFromPulls(cohortPulls(f))
		if err := cohortCoverage(f, prs, d); err != nil {
			t.Fatal(err)
		}
		cohortServer(t, f, nil)
		in, _, unread := cohortInput(t, f, root)
		if len(unread) != 0 {
			t.Fatal(unread)
		}
		cohortAssert(t, f, DeriveLifecycle(in))
	})
	t.Run("protected", TestApplyReconcileWrites_DoneRowImmutable)
	t.Run("stamp", TestApplyReconcileWrites_NeverTouchesHumanStamp)
	t.Run("evidence-unknown", TestEvidenceReadUnknown)
}
func cohortBoards(t *testing.T, root string) map[string]string {
	t.Helper()
	m := map[string]string{}
	paths, e := filepath.Glob(filepath.Join(root, "docs/streams/*/README.md"))
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range paths {
		m[p] = cohortRead(t, p)
	}
	return m
}

// regression: #2703
func TestBoardCohortFull(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	f := cohortLoad(t)
	root := cohortRoot(t, f)
	shape := reportedEvidence(t)
	evidence := f.Evidence
	evidence.MatcherInput = true
	for _, path := range shape.Files {
		evidence.Files = append(evidence.Files, prFile{Filename: path, Status: "modified"})
	}
	f.Records = append(f.Records, evidence)
	reads := cohortServer(t, f, nil)
	foldFlipSeam(t, &fakeFlipSource{shapes: map[int]prShape{2295: shape}})
	in, cl, u := cohortInput(t, f, root)
	if len(u) != 0 {
		t.Fatal(u)
	}
	cohortAssert(t, f, DeriveLifecycle(in))
	for twice := 0; twice < 2; twice++ {
		for _, v := range f.Dispositions {
			got, e := cl.nonDelivery(v.Number, v.Brief)
			if e != nil || got != (v.Covers == "partial") {
				t.Fatalf("%s#%d: excluded=%v err=%v", v.Brief, v.Number, got, e)
			}
		}
	}
	sort.Ints(*reads)
	if len(*reads) != 10 {
		t.Fatalf("cached reads=%v, want nine full file lists plus Evidence-only", *reads)
	}
	r := cohortRecordAt(t, f, 1196)
	if len(r.Files) != 10 || !strings.Contains(r.Scope, "finishes those two deliverables") {
		t.Fatal("real full-delivery evidence absent")
	}
	old := cohortRecordAt(t, f, 1062)
	if !strings.Contains(old.Scope, "root resolution") || len(old.Files) != 35 {
		t.Fatal("prior delivery evidence absent")
	}
	for _, p := range in.PRs {
		if p.Number == 2295 {
			t.Fatal("Evidence-only PR survived admission")
		}
	}
	// Actual full record admitted through its real file list; later partial is
	// synthetic, while the Evidence-only counterexample retains its real shape.
	p := prsForCohort(in.PRs, "desk-tools/27")
	p = append(p, PRRecord{BriefRef: "desk-tools/27", Number: 4000, State: prMerged, MergeSHA: "abcdef0", Partial: true})
	b := BriefIdent{ID: "desk-tools/27", Version: 1}
	c := deriveSingle(t, b, LifecycleInput{LookedAt: true, PRs: p, Witnesses: map[string]WitnessInfo{b.ID: {Version: 1, Passed: false}}})
	if c.Cell != "implemented" || !strings.Contains(c.MergedPR, "PR #1196 ") {
		t.Fatal(c)
	}
	t.Run("evidence", TestEvidenceAdmission)
	t.Run("authoring", TestAuthoringPRIsNoWitness)
	t.Run("full-after-partial", TestPartialThenDelivery)
}
