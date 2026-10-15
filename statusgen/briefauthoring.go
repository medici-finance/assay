package main

// briefauthoring.go — keeping a briefs-AUTHORING PR from reading as the brief's DELIVERY
// in `statusgen reconcile`.
//
// THE DEFECT CLASS. Two readers turn a merged PR into the witness that a brief is
// `implemented`: the `Brief:` trailer fold (lifecycle.go, via ghfetch.go's ListPRs) and the
// declared `--backfill` branch/body match (reconcilebackfill.go's matchBackfillPR). A docs-only
// PR that only WROTE a brief names it in exactly those places, so either reader took it for the
// delivery and proposed flipping a row whose code was never built. The dispatcher already drew
// this line from the PR's changed files (deskkit.BriefAuthoringOnly); this file draws the same
// line for the two reconcile readers, so the class is closed in every reader that makes a PR a
// witness, not only the one that was reported.
//
// WHY A DUPLICATE AND NOT AN IMPORT. The classifier lives in
// tools/desk/internal/deskkit/briefauthoring.go, a separate Go module whose package is
// `internal/`, so there is no import path in either direction. statusgen's established answer is
// duplicate-the-reader and bind-the-two-with-a-shared-vector (riskpathtriggers.go,
// rosterconfig.go): testdata/brief_authoring_vectors.json is read by a coupling test in BOTH
// modules, so a drift in either copy reddens both.
//
// THE RULE (identical to the original). A PR is the AUTHORING of `<stream>/<NN>`, not its
// delivery, only when every file it touches is a stream board README, a brief file or a
// changelog fragment, AND it ADDS that brief's own file. Anything else, an empty list, or an
// unparseable brief id is "not authoring-only": the PR stays a witness, the status quo. The
// classification can only ever REMOVE a witness.
//
// THREE-STATE. A PR whose file list cannot be read, or cannot be proved complete, is neither
// answer. The brief derives `unknown` and the read is disclosed in `unread`, which refuses
// `--apply` (reconcile.go) — it is never guessed in either direction.
//
// COST. One file-list read per candidate PR, cached for the run. The trailer fold classifies
// only briefs whose board row is todo or in-progress (the only rows --apply can move); the
// backfill classifies only the merged pulls that match a todo-derived brief.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
)

// prFile is one entry of GET /pulls/{n}/files — the subset the classifier reads.
type prFile struct {
	Filename         string `json:"filename"`
	Status           string `json:"status"` // added | modified | removed | renamed | ...
	PreviousFilename string `json:"previous_filename"`
}

// splitBriefRef reduces a brief ref (`<stream>/<NN>` or a colon form whose last two segments are
// stream and number) to its stream and number — the reduction deskkit.SplitBriefTrailer performs.
func splitBriefRef(v string) (stream, nn string, ok bool) {
	return briefStreamNum(strings.TrimSpace(v))
}

// briefAuthoringOnly mirrors deskkit.BriefAuthoringOnly: it reports whether a change touching
// files only AUTHORED briefID rather than delivering it.
func briefAuthoringOnly(briefID string, files []prFile) bool {
	stream, nn, ok := splitBriefRef(briefID)
	if !ok || len(files) == 0 {
		return false
	}
	authored := false
	for _, f := range files {
		if !isAuthoringPath(f.Filename) {
			return false
		}
		if f.PreviousFilename != "" && !isAuthoringPath(f.PreviousFilename) {
			return false
		}
		if strings.EqualFold(strings.TrimSpace(f.Status), "added") && isBriefFileFor(f.Filename, stream, nn) {
			authored = true
		}
	}
	return authored
}

// cleanRepoPath normalises a forge-reported repo-relative path so a `./` prefix or a `..`
// segment cannot make a path outside docs/streams/ look like one inside it.
func cleanRepoPath(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if p == "" {
		return ""
	}
	return path.Clean(p)
}

// isAuthoringPath reports whether p is a file an authoring PR writes: a stream's board README,
// a brief file, or a changelog fragment directly under changelog/ (not its README).
func isAuthoringPath(p string) bool {
	p = cleanRepoPath(p)
	if p == "" || strings.HasPrefix(p, "/") || p == ".." || strings.HasPrefix(p, "../") {
		return false
	}
	dir, base := path.Split(p)
	if dir == "changelog/" {
		return strings.HasSuffix(base, ".md") && base != ".md" && !strings.EqualFold(base, "README.md")
	}
	stream, ok := strings.CutPrefix(dir, "docs/streams/")
	if !ok {
		return false
	}
	stream = strings.TrimSuffix(stream, "/")
	if stream == "" || strings.Contains(stream, "/") {
		return false
	}
	return base == "README.md" || isBriefFileName(base)
}

// isBriefFileName reports whether base names a brief file, `brief-<NN>-<slug>.md`, NN numeric.
func isBriefFileName(base string) bool {
	rest, ok := strings.CutPrefix(base, "brief-")
	if !ok {
		return false
	}
	nn, slug, ok := strings.Cut(rest, "-")
	if !ok || nn == "" || !strings.HasSuffix(slug, ".md") || slug == ".md" {
		return false
	}
	for _, c := range nn {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// isBriefFileFor reports whether p is brief <stream>/<NN>'s own file,
// `docs/streams/<stream>/brief-<NN>-*.md`.
func isBriefFileFor(p, stream, nn string) bool {
	dir, base := path.Split(cleanRepoPath(p))
	if !strings.EqualFold(dir, "docs/streams/"+stream+"/") {
		return false
	}
	prefix := "brief-" + nn + "-"
	return strings.HasPrefix(base, prefix) && strings.HasSuffix(base, ".md") && len(base) > len(prefix)+len(".md")
}

// ListPRFiles reads one PR's changed-file list (GET /pulls/{n}/files, REST only). GitHub stops
// listing at 3000 files, so a list that fills every page is not provably complete and is an
// error, never a partial answer: a short walk that happened to show only docs could hide the
// code that makes the PR a delivery.
func (c *ghClient) ListPRFiles(repo string, number int) ([]prFile, error) {
	const perPage = 100
	const maxPages = 30 // the forge's 3000-file cap
	var files []prFile
	for page := 1; page <= maxPages; page++ {
		url := fmt.Sprintf("%s/repos/%s/pulls/%d/files?per_page=%d&page=%d", c.base, repo, number, perPage, page)
		body, status, err := c.get(url)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("%s", httpReason(status, body))
		}
		var batch []prFile
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("HTTP 200 but response did not parse as a files list: %v", err)
		}
		files = append(files, batch...)
		if len(batch) < perPage {
			return files, nil
		}
	}
	return nil, fmt.Errorf("the file list reached the forge's %d-file cap, so it is not provably complete", perPage*maxPages)
}

// authoringClassifier answers "did PR n only AUTHOR brief b?" from the PR's changed files,
// reading each PR's file list at most once per run.
type authoringClassifier struct {
	client *ghClient
	repo   string
	stderr io.Writer
	mu     sync.Mutex // guards cache: prefetch fills it from several readers
	cache  map[int]filesResult
}

// authoringReadWorkers bounds the concurrent file-list reads a prefetch makes:
// enough to hide the per-request round trip, few enough to stay clear of the
// forge's secondary (concurrency) rate limit.
const authoringReadWorkers = 6

type filesResult struct {
	files []prFile
	err   error
}

func newAuthoringClassifier(client *ghClient, repo string, stderr io.Writer) *authoringClassifier {
	return &authoringClassifier{client: client, repo: repo, stderr: stderr, cache: map[int]filesResult{}}
}

// authoringOnly reports whether PR number only authored briefID. A non-nil error means the file
// list could not be read or proved complete: the answer is could-not-check.
func (a *authoringClassifier) authoringOnly(number int, briefID string) (bool, error) {
	r := a.files(number)
	if r.err != nil {
		return false, r.err
	}
	if briefAuthoringOnly(briefID, r.files) {
		if a.stderr != nil {
			fmt.Fprintf(a.stderr, "reconcile: NOTICE — PR #%d names %s but only AUTHORED it (it adds the brief's file and touches only "+
				"stream board READMEs, brief files and changelog fragments), so it is not a witness that the brief is implemented\n", number, briefID)
		}
		return true, nil
	}
	return false, nil
}

// files returns PR number's file-list read, reading it on the first ask.
func (a *authoringClassifier) files(number int) filesResult {
	if r, ok := a.cached(number); ok {
		return r
	}
	var r filesResult
	r.files, r.err = a.client.ListPRFiles(a.repo, number)
	a.mu.Lock()
	a.cache[number] = r
	a.mu.Unlock()
	return r
}

func (a *authoringClassifier) cached(number int) (filesResult, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, ok := a.cache[number]
	return r, ok
}

// prefetch reads every uncached PR in numbers, authoringReadWorkers at a time.
func (a *authoringClassifier) prefetch(numbers []int) {
	work := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < authoringReadWorkers && w < len(numbers); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range work {
				a.files(n)
			}
		}()
	}
	sent := map[int]bool{}
	for _, n := range numbers {
		if !sent[n] { // two workers on one PR would read its list twice
			sent[n] = true
			work <- n
		}
	}
	close(work)
	wg.Wait()
}

// prefetchBackfill reads, in parallel, EXACTLY the file lists the backfill
// match (matchBackfillPRChecked) will read one by one for briefs, so the
// sequential match that follows answers from the cache. It replays the match
// in waves: each wave reads, for every brief still unresolved, the next
// candidate the match would ask about — the newest not yet shown to be
// authoring-only — and stops a brief at its first delivery or unreadable list.
// The set of PRs read is therefore the set the sequential walk reads, never a
// speculative superset; only the waiting is overlapped.
func (a *authoringClassifier) prefetchBackfill(pulls []ghPull, briefs [][2]string) {
	type walk struct {
		id    string
		cands []ghPull
		next  int
	}
	var walks []*walk
	for _, b := range briefs {
		if cands := backfillCandidates(pulls, b[0], b[1]); len(cands) > 0 {
			walks = append(walks, &walk{id: b[0] + "/" + b[1], cands: cands})
		}
	}
	for {
		need := map[int]bool{}
		var wave []int
		for _, w := range walks {
			for w.next < len(w.cands) {
				n := w.cands[w.next].Number
				r, ok := a.cached(n)
				if !ok {
					if !need[n] {
						need[n] = true
						wave = append(wave, n)
					}
					break
				}
				if r.err != nil || !briefAuthoringOnly(w.id, r.files) {
					w.next = len(w.cands) // resolved: the match stops here
					break
				}
				w.next++ // authoring-only: the match asks about the next one
			}
		}
		if len(wave) == 0 {
			return
		}
		a.prefetch(wave)
	}
}

// unreadAuthoring is the disclosure for a PR whose authored-or-delivered question could not be
// answered.
func unreadAuthoring(number int, briefID string, err error) string {
	return fmt.Sprintf("could-not-check whether PR #%d authored or delivered %s (%v) — %s derives unknown, not a witnessed implemented",
		number, briefID, err, briefID)
}

// dropAuthoringPRs removes, from the `Brief:`-trailer PR records, each open or merged PR that
// only AUTHORED the brief its trailer names. inScope narrows which briefs are classified (nil =
// all), by canonical key. A PR the classifier could not read is KEPT (the status quo) but its
// brief is returned in undecided (by every ident id sharing the key), so the cell derives
// `unknown`, and the reason is returned in unread, which refuses --apply.
func dropAuthoringPRs(cl *authoringClassifier, prs []PRRecord, idents []BriefIdent, inScope func(key string) bool) (kept []PRRecord, undecided map[string]string, unread []string) {
	idsByKey := map[string][]string{}
	for _, b := range idents {
		k := canonicalBriefKey(b.ID)
		idsByKey[k] = append(idsByKey[k], b.ID)
	}
	undecided = map[string]string{}
	kept = make([]PRRecord, 0, len(prs))
	for _, pr := range prs {
		key := canonicalBriefKey(pr.BriefRef)
		if pr.State == prClosed || (inScope != nil && !inScope(key)) {
			kept = append(kept, pr)
			continue
		}
		only, err := cl.authoringOnly(pr.Number, key)
		switch {
		case err != nil:
			why := unreadAuthoring(pr.Number, key, err)
			unread = append(unread, why)
			for _, id := range idsByKey[key] {
				undecided[id] = why
			}
			kept = append(kept, pr)
		case only:
			// dropped: an authoring PR is not a witness
		default:
			kept = append(kept, pr)
		}
	}
	return kept, undecided, unread
}

// backfillAuthoring is the seam matchBackfillPRChecked asks "did PR n only author stream/num?"
// through. A nil seam classifies nothing (every match is a delivery — the behaviour before this
// file existed).
type backfillAuthoring func(number int, briefID string) (bool, error)

// matchBackfillPRChecked is matchBackfillPR with the authoring exemption: the highest-numbered
// MERGED pull that names stream/num AND did not only author it. Candidates are tried newest
// first, and the first delivery wins. A candidate whose file list could not be read ends the
// search unresolved (unread non-empty, ok false): a lower-numbered match is not taken in its
// place, because the unread PR may be the delivery.
func matchBackfillPRChecked(pulls []ghPull, stream, num string, authoring backfillAuthoring) (match ghPull, ok bool, unread string) {
	if authoring == nil {
		m, found := matchBackfillPR(pulls, stream, num)
		return m, found, ""
	}
	id := stream + "/" + num
	for _, p := range backfillCandidates(pulls, stream, num) {
		only, err := authoring(p.Number, id)
		if err != nil {
			return ghPull{}, false, unreadAuthoring(p.Number, id, err)
		}
		if !only {
			return p, true, ""
		}
	}
	return ghPull{}, false, ""
}

// backfillCandidates lists the MERGED pulls that name stream/num in a declared
// backfill form, newest first — the order matchBackfillPRChecked tries them in.
func backfillCandidates(pulls []ghPull, stream, num string) []ghPull {
	var cands []ghPull
	for _, p := range pulls {
		if p.MergedAt == "" {
			continue
		}
		if containsBackfillForm(p.Head.Ref, stream, num) || containsBackfillForm(p.Body, stream, num) {
			cands = append(cands, p)
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Number > cands[j].Number })
	return cands
}

// movableBriefKeys returns the canonical keys of the briefs whose board row is todo or
// in-progress — the only rows `reconcile --apply` can move to implemented. It is the scope of
// the trailer-fold classification, which keeps the file-list reads to the rows that matter.
// ok is false when the board could not be read, in which case the caller classifies everything.
func movableBriefKeys(boardRoot string) (keys map[string]bool, ok bool) {
	streams, _, err := loadStreams(boardRoot)
	if err != nil {
		return nil, false
	}
	keys = map[string]bool{}
	for _, s := range streams {
		for _, b := range s.Briefs {
			if b.Status == "todo" || b.Status == "in-progress" {
				keys[canonicalBriefKey(s.Name+"/"+b.Num)] = true
			}
		}
	}
	return keys, true
}

// admitPRWitnesses is the one place the `Brief:`-trailer PR set is narrowed to genuine
// witnesses. It builds the run's classifier, drops the authoring-only PRs from in.PRs (in
// place), and returns the classifier (shared with the --backfill match so a PR's files are read
// once), the briefs whose question could not be answered (to be merged into in.Undecided AFTER
// wireFoldInputs resets that map), and the disclosures. reconcile and regen's drift comparator
// both call it, so the two agree by construction. A run that could not look (in.LookedAt
// false) has no PRs to narrow.
func admitPRWitnesses(in *LifecycleInput, client *ghClient, repo, root string, stderr io.Writer) (*authoringClassifier, map[string]string, []string) {
	cl := newAuthoringClassifier(client, repo, stderr)
	if !in.LookedAt {
		return cl, nil, nil
	}
	var inScope func(string) bool
	if boardRoot, found := findBoardRoot(root); found {
		if keys, ok := movableBriefKeys(boardRoot); ok {
			inScope = func(key string) bool { return keys[key] }
		}
	}
	// dropAuthoringPRs reads one file list per in-scope open or merged PR;
	// read exactly those in parallel first, so its walk answers from the cache.
	var reads []int
	for _, pr := range in.PRs {
		if pr.State != prClosed && (inScope == nil || inScope(canonicalBriefKey(pr.BriefRef))) {
			reads = append(reads, pr.Number)
		}
	}
	cl.prefetch(reads)
	kept, undecided, unread := dropAuthoringPRs(cl, in.PRs, in.Briefs, inScope)
	in.PRs = kept
	for _, m := range unread {
		fmt.Fprintf(stderr, "reconcile: %s\n", m)
	}
	return cl, undecided, unread
}

// mergeUndecided adds the authoring-classification could-not-checks to in.Undecided without
// overwriting a reason the fold already gave.
func mergeUndecided(in *LifecycleInput, undecided map[string]string) {
	if len(undecided) == 0 {
		return
	}
	if in.Undecided == nil {
		in.Undecided = map[string]string{}
	}
	for id, why := range undecided {
		if _, taken := in.Undecided[id]; !taken {
			in.Undecided[id] = why
		}
	}
}
