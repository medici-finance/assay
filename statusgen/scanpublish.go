package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskcore/issueflow"
)

// scanpublish.go — the scan publishes recorded intake judgments.
//
// A judgment an intake session records (`scanloop land --handoff`) lives in a local log. The board
// schedules on two frontmatter keys of the placeholder, `handoff:` and `handoff-opened:`, and on the
// sibling companion file they name. Nothing wrote either, so publication was a hand edit. The scan is
// the placeholder's only writer, so the publication belongs here: given the log, it writes
// `<placeholder stem>.handoff.json` beside each placeholder and sets the two keys.
//
// Two things bound what the scan will write.
//
//   - Event. The scan reads open issues over the forge wire, which carries no update time, so it has
//     no live event time to compare a judgment against. It publishes the latest revision of the latest
//     event the LOG holds for the issue, never an older event, and it never republishes a judgment a
//     reopened issue's reactivation already stripped (the companion stays behind; its presence is the
//     mark that the judgment is spent). A judgment for an event the log has not yet seen is the
//     writer's to record; the board then holds the placeholder until it is.
//   - Text. A judgment carries three free-text fields (evidence, required_action, wake_condition).
//     Copying them into a committed file publishes them wherever that file is committed. The scan
//     therefore decides, through the one reader the package already has (onBehalfOfPublicForm), whether
//     the repository the file is committed to is public, and under a public root publishes a judgment
//     only when none of the three can carry prose. See scanRootPublic for how that repository is read.

// scanPublishOpts is what --scan-issues is told about publication. A zero value publishes nothing and
// leaves the scan exactly as it was.
type scanPublishOpts struct {
	// Judgments is the path of the judgment log (JSON lines, one intake-handoff-v1 record each).
	Judgments string
	// Target is the repository the scan's delta is committed to, as the drain resolved it: a name,
	// never a visibility value.
	Target string
}

// scanPublishResult is the count a publishing pass reports.
type scanPublishResult struct {
	Published   int
	Unchanged   int
	Unpublished int
}

// handoffTypedRefs are the shapes of a typed reference (spec/intake-exit-v1.md, "Typed refs") that a
// judgment's evidence may take. The desk tools hold the same shapes; the two modules share no code.
var handoffTypedRefs = []*regexp.Regexp{
	regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[0-9]{2,4}$`),                        // <stream>/<NN>
	regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+#[1-9][0-9]*$`), // owner/repo#N
	regexp.MustCompile(`^#[1-9][0-9]*$`),                                         // #N
	regexp.MustCompile(`^F-[A-Za-z0-9][A-Za-z0-9-]*$`),                           // F-<slug>
}

func isHandoffTypedRef(s string) bool {
	for _, re := range handoffTypedRefs {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// publicFormRefusal names the first field that keeps a judgment out of a public repository, or "".
// A public root takes a judgment only when its evidence is a typed reference and its required action
// and wake condition are empty (the verification block repeats the required action, so it is held to
// the same rule).
func publicFormRefusal(h issueflow.IntakeHandoff) string {
	switch {
	case !isHandoffTypedRef(h.Evidence):
		return "evidence is not a typed reference"
	case h.RequiredAction != "":
		return "required_action is free text"
	case h.WakeCondition != "":
		return "wake_condition is free text"
	case h.Verification != nil && h.Verification.RequiredAction != "":
		return "verification.required_action is free text"
	}
	return ""
}

// scanRootPublic reports whether the root's judgments must be treated as published to a public
// repository. It is true unless ALL of these hold: a target was supplied, the root's origin PUSH
// destination names that same repository, and the roster marks it private. Everything else (no
// target, an origin that cannot be read or does not parse as one repository, an origin that names a
// different repository, an unlisted or unmarked repository) answers public.
//
// The target is a declared name and the bytes are pushed to the origin, so the declaration alone is
// not trusted: the push destination is read the way deskpr reads it (`git remote get-url --push
// --all origin`, exactly one value). The visibility itself is not decided here; the target's name is
// handed, unchanged, to onBehalfOfPublicForm.
func scanRootPublic(root, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return onBehalfOfPublicForm("")
	}
	out, err := exec.Command("git", "-C", root, "remote", "get-url", "--push", "--all", "origin").Output()
	if err != nil {
		return true
	}
	dests := strings.Fields(strings.TrimSpace(string(out)))
	if len(dests) != 1 {
		return true
	}
	if dest := originOwnerRepo(dests[0]); dest == "" || !strings.EqualFold(dest, target) {
		return true
	}
	return onBehalfOfPublicForm(target)
}

// handoffLog is the decoded judgment log, grouped by item.
type handoffLog struct {
	byItem map[string][]issueflow.IntakeHandoff
	// bad lists records the scan could not use, each already worded for the notice.
	bad []string
}

// readHandoffLog reads the log at path. An absent file is not an error: it returns an empty log and
// absent=true. Any other read failure is returned.
func readHandoffLog(path string) (log handoffLog, absent bool, err error) {
	log.byItem = map[string][]issueflow.IntakeHandoff{}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return log, true, nil
	}
	if err != nil {
		return log, false, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 65536), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		h, derr := issueflow.DecodeIntakeHandoff([]byte(line))
		if derr != nil {
			log.bad = append(log.bad, fmt.Sprintf("judgment log line %d does not decode strictly", n))
			continue
		}
		if _, terr := time.Parse(time.RFC3339, h.Opened); terr != nil {
			log.bad = append(log.bad, fmt.Sprintf("judgment log line %d has no RFC 3339 event time", n))
			continue
		}
		key := strings.ToLower(h.Item)
		log.byItem[key] = append(log.byItem[key], h)
	}
	return log, false, sc.Err()
}

// latestHandoff picks the judgment to publish for one item: the highest revision recorded for the
// latest event. Records for older events are ignored.
func latestHandoff(rs []issueflow.IntakeHandoff) (issueflow.IntakeHandoff, bool) {
	var best issueflow.IntakeHandoff
	var bestAt time.Time
	found := false
	for _, r := range rs {
		at, err := time.Parse(time.RFC3339, r.Opened)
		if err != nil {
			continue
		}
		if !found || at.After(bestAt) || (at.Equal(bestAt) && r.Revision > best.Revision) {
			best, bestAt, found = r, at, true
		}
	}
	return best, found
}

// handoffCompanionName is the companion's file name for a placeholder file.
func handoffCompanionName(placeholderPath string) string {
	return placeholderNum(placeholderPath) + ".handoff.json"
}

// readCompanionIn reads and strictly decodes a companion inside dir without following a symlink out of it.
func readCompanionIn(dir, name string) (issueflow.IntakeHandoff, []byte, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return issueflow.IntakeHandoff{}, nil, err
	}
	defer r.Close()
	raw, err := r.ReadFile(name)
	if err != nil {
		return issueflow.IntakeHandoff{}, nil, err
	}
	h, err := issueflow.DecodeIntakeHandoff(raw)
	return h, raw, err
}

// setHandoffKeys returns the placeholder text with its `handoff:` and `handoff-opened:` keys set,
// both written last in the frontmatter. Existing values are replaced, so the result is the same text
// when the keys already say this.
func setHandoffKeys(content, name, opened string) string {
	lines := strings.Split(content, "\n")
	var out []string
	in, closed := false, false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !closed {
			if trimmed == "---" {
				if !in {
					in = true
				} else {
					out = append(out, "handoff: "+name, `handoff-opened: "`+opened+`"`)
					closed = true
				}
				out = append(out, line)
				continue
			}
			if in {
				key, _, _ := strings.Cut(trimmed, ":")
				switch strings.Trim(strings.TrimSpace(key), "\"'") {
				case "handoff", "handoff-opened":
					continue
				}
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// publishHandoffs is the publishing pass. It runs after the scan's own writes (or, with dryRun,
// instead of them: it then lists what it would write for the placeholders already on disk). It
// returns the counts, and an error only when the log exists and could not be read.
func publishHandoffs(root string, opts scanPublishOpts, dryRun bool, out io.Writer) (scanPublishResult, error) {
	var res scanPublishResult
	if opts.Judgments == "" {
		return res, nil
	}
	log, absent, err := readHandoffLog(opts.Judgments)
	if err != nil {
		return res, fmt.Errorf("cannot read the judgment log: %w", err)
	}
	if absent {
		fmt.Fprintln(out, "scan-issues: handoffs: no judgment log recorded — nothing to publish")
		return res, nil
	}
	var notices []string
	for _, b := range log.bad {
		notices = append(notices, "scan-issues: handoff unpublished — "+b)
		res.Unpublished++
	}

	public := scanRootPublic(root, opts.Target)
	dir := filepath.Join(root, "docs", "streams", scanStreamName)
	paths := placeholderFilePaths(&Stream{Dir: dir})
	for _, path := range paths {
		ph, ok, perr := parsePlaceholderFile(path)
		if perr != nil || !ok || ph.Status == "done" {
			continue
		}
		item := fmt.Sprintf("%s#%d", ph.Repo, ph.Issue)
		cand, found := latestHandoff(log.byItem[strings.ToLower(item)])
		if !found {
			continue
		}
		verdict, note := publishOne(dir, path, ph, cand, public, dryRun)
		switch verdict {
		case "published":
			res.Published++
			verb := "published"
			if dryRun {
				verb = "would publish"
			}
			fmt.Fprintf(out, "%s %s  (%s, revision %d)\n", verb, handoffCompanionName(path), item, cand.Revision)
		case "unchanged":
			res.Unchanged++
		default:
			res.Unpublished++
			notices = append(notices, fmt.Sprintf("scan-issues: handoff unpublished — %s: %s", item, note))
		}
	}
	emitNotices(notices)
	fmt.Fprintf(out, "scan-issues: handoffs: %d published, %d unchanged, %d unpublished\n", res.Published, res.Unchanged, res.Unpublished)
	return res, nil
}

// publishOne decides and applies one placeholder's publication. It returns "published", "unchanged"
// or "unpublished" with the reason for the last.
func publishOne(dir, path string, ph *Placeholder, cand issueflow.IntakeHandoff, public, dryRun bool) (string, string) {
	name := handoffCompanionName(path)
	candAt, _ := time.Parse(time.RFC3339, cand.Opened)

	if public {
		if why := publicFormRefusal(cand); why != "" {
			return "unpublished", why + "; the text stays local in a public repository"
		}
	}

	switch {
	case ph.Handoff != "" && ph.Handoff != name:
		return "unpublished", "the placeholder already points at another companion and is not replaced"
	case ph.Handoff == name:
		// Published before. A companion that cannot be read is rewritten; a readable one is only
		// replaced by a later event, or a later revision of the same event.
		cur, _, err := readCompanionIn(dir, name)
		if err == nil {
			curAt, terr := time.Parse(time.RFC3339, cur.Opened)
			switch {
			case terr == nil && curAt.After(candAt):
				return "unpublished", "the published judgment is for a later event than the log's latest"
			case terr == nil && curAt.Equal(candAt) && cur.Revision > cand.Revision:
				return "unpublished", "the published judgment is a later revision than the log's"
			case reflect.DeepEqual(cur, cand) && ph.HandoffOpened == cand.Opened:
				return "unchanged", ""
			}
		}
	default:
		// No pointer. A companion already beside the placeholder (or in its archive) for this event
		// or a later one means a reactivation stripped the pointer: that judgment is spent.
		for _, d := range []string{dir, filepath.Join(dir, archiveDirName)} {
			prev, _, err := readCompanionIn(d, name)
			if err != nil {
				continue
			}
			if prevAt, terr := time.Parse(time.RFC3339, prev.Opened); terr == nil && !prevAt.Before(candAt) {
				return "unpublished", "a reactivation already spent this event's judgment; record one for the new event"
			}
		}
	}

	if dryRun {
		return "published", ""
	}
	raw, err := json.Marshal(cand)
	if err != nil {
		return "unpublished", "the judgment could not be encoded"
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "unpublished", "the placeholder could not be read"
	}
	// Pointer first, companion second: a failure between the two leaves a pointer to a missing
	// companion, which the board holds and the next scan repairs. The reverse order would leave a
	// companion with no pointer, which reads as a spent judgment.
	if next := setHandoffKeys(string(body), name, cand.Opened); next != string(body) {
		if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
			return "unpublished", "the placeholder could not be written"
		}
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		return "unpublished", "the stream directory could not be opened"
	}
	defer r.Close()
	if err := r.WriteFile(name, append(raw, '\n'), 0o644); err != nil {
		return "unpublished", "the companion could not be written"
	}
	return "published", ""
}
