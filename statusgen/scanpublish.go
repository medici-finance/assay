package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
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
//   - Text. Copying a judgment into a committed file publishes every field of it wherever that file
//     is committed. The scan therefore decides, through the one reader the package already has
//     (onBehalfOfPublicForm), whether the repository the file is committed to is public, and under a
//     public root publishes a judgment only when no field can carry prose (publicFormRefusal). See
//     scanRootPublic for how that repository is read.

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

// Closed shapes for the string fields a public root may carry. Each is bounded, so a field that
// passes is a reference or an opaque id and cannot work as a text channel.
var (
	pubItemRef   = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}/[A-Za-z0-9_.-]{1,64}#[1-9][0-9]{0,8}$`)
	pubSuccessor = regexp.MustCompile(`^(([A-Za-z0-9_.-]{1,64}/[A-Za-z0-9_.-]{1,64})?#[1-9][0-9]{0,8}|[a-z0-9][a-z0-9-]{0,39}/[0-9]{1,4})$`)
	pubSlug      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	pubOpaque    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,63}$`)
	pubVersion   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$`)
	pubRepoRef   = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}/[A-Za-z0-9_.-]{1,64}$`)
)

// pubEnum reports whether v is empty (when allowEmpty) or one of the listed values.
func pubEnum(v string, allowEmpty bool, vals ...string) bool {
	if v == "" {
		return allowEmpty
	}
	for _, x := range vals {
		if v == x {
			return true
		}
	}
	return false
}

// publicCoveredFields lists, by JSON path, every field of the judgment record that publicFormRefusal
// checks below, or admits because its type cannot carry text (a number, a bool, a time, a list of
// numbers), with the shape (leafShape) the check was written for. The list is compared with the
// record's type, not with a judgment's values: a field the record gains later is not on it, and a
// listed field whose type changes (a list of numbers becoming a list of strings, a map, an any) no
// longer matches its shape, so either way every judgment is refused under a public root, naming that
// field, until the list and the check below are brought up to date.
var publicCoveredFields = map[string]string{
	"schema": "string", "item": "string", "opened": "string", "revision": "int", "deciding_role": "string",
	"disposition": "string", "delivered": "bool", "route": "string", "successor": "string", "author_tier": "string",
	"external_role": "string", "required_action": "string", "wake_condition": "string", "evidence": "string",
	"verification.schema": "string", "verification.disposition": "string", "verification.deciding_role": "string",
	"verification.successor": "string", "verification.external_role": "string", "verification.required_action": "string",
	"verification.observed_at": "time", "verification.fresh_until": "time",
	"verification.source.repo": "string", "verification.source.brief": "string", "verification.source.source_version": "string",
	"verification.source.source_revision": "string", "verification.source.receipt_id": "string",
	"verification.source.case_id": "string", "verification.source.rows": "[]int", "verification.source.blocker_ref": "string",
}

// publicUncovered is the record's fields that publicCoveredFields does not list, or lists with
// another shape, read once.
var publicUncovered = sync.OnceValue(func() []string {
	return uncoveredFields(reflect.TypeOf(issueflow.IntakeHandoff{}), "", publicCoveredFields)
})

// uncoveredFields walks the fields encoding/json reads from t by JSON path, into nested structs,
// pointers to them and embedded structs (a time is a leaf), and returns each leaf path covered does
// not list, or lists with a shape other than the field's own. It skips exactly the fields
// encoding/json skips (an unexported field that is not an embedded struct, a "-" tag), so an
// embedded struct is walked whatever its tag: promoted when the tag names no field (none, or
// options only such as ",omitempty"), and nested under the tag's name when it names one.
func uncoveredFields(t reflect.Type, prefix string, covered map[string]string) []string {
	var out []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		nested := ft.Kind() == reflect.Struct && ft != reflect.TypeOf(time.Time{})
		tag := f.Tag.Get("json")
		if tag == "-" || (!f.IsExported() && !(f.Anonymous && ft.Kind() == reflect.Struct)) {
			continue
		}
		if name, _, _ := strings.Cut(tag, ","); f.Anonymous && nested && name == "" {
			// encoding/json promotes an embedded struct's fields, exported or not.
			out = append(out, uncoveredFields(ft, prefix, covered)...)
			continue
		}
		path := prefix + jsonFieldName(f)
		if nested {
			out = append(out, uncoveredFields(ft, path+".", covered)...)
			continue
		}
		if want, ok := covered[path]; !ok {
			out = append(out, path)
		} else if got := leafShape(ft); got != want {
			out = append(out, fmt.Sprintf("%s (shape %s, listed as %s)", path, got, want))
		}
	}
	return out
}

// leafShape names a leaf field's type by kind: "string" for any string type, "time" for a time,
// "[]" plus the element's shape for a list, "map[k]v" for a map, and the kind's name otherwise
// ("interface" for an any).
func leafShape(t reflect.Type) string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == reflect.TypeOf(time.Time{}) {
		return "time"
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		return "[]" + leafShape(t.Elem())
	case reflect.Map:
		return "map[" + leafShape(t.Key()) + "]" + leafShape(t.Elem())
	}
	return t.Kind().String()
}

// jsonFieldName is the name encoding/json gives f.
func jsonFieldName(f reflect.StructField) string {
	if n, _, _ := strings.Cut(f.Tag.Get("json"), ","); n != "" {
		return n
	}
	return f.Name
}

// publicFormRefusal names the first field that keeps a judgment out of a public repository, or "".
// The check is an ALLOWLIST over every field of the record: a public root takes a judgment only when
// each string field is a closed enum value, a bounded typed reference, an opaque id or an RFC 3339
// time, and the free-text fields (required action, wake condition, in the verification block too)
// are empty. A field the record gains later, or a listed field whose shape changes, is refused by
// default: until publicCoveredFields lists it with its shape, every judgment is refused, naming it.
func publicFormRefusal(h issueflow.IntakeHandoff) string {
	closed := func(field string) string { return field + " is not a closed shape" }
	if u := publicUncovered(); len(u) > 0 {
		return u[0] + " is not covered by the public allowlist"
	}
	if _, err := time.Parse(time.RFC3339, h.Opened); err != nil {
		return closed("opened")
	}
	switch {
	case !isHandoffTypedRef(h.Evidence) || len(h.Evidence) > 96:
		return "evidence is not a typed reference"
	case h.RequiredAction != "":
		return "required_action is free text"
	case h.WakeCondition != "":
		return "wake_condition is free text"
	case h.Schema != issueflow.IntakeHandoffSchema:
		return closed("schema")
	case !pubItemRef.MatchString(h.Item):
		return closed("item")
	case !pubSlug.MatchString(h.DecidingRole):
		return closed("deciding_role")
	case !pubEnum(string(h.Disposition), false, string(issueflow.Unresolved), string(issueflow.NeedsScoping), string(issueflow.Repair), string(issueflow.External), string(issueflow.Duplicate)):
		return closed("disposition")
	case !pubEnum(h.Route, true, "authoring", "repair", "qualification", "duplicate", "external"):
		return closed("route")
	case h.Successor != "" && !pubSuccessor.MatchString(h.Successor):
		return closed("successor")
	case !pubEnum(h.AuthorTier, true, "any", "strong"):
		return closed("author_tier")
	case !pubEnum(h.ExternalRole, true, "human", "operator", "brief-author", "verifier"):
		return closed("external_role")
	}
	if v := h.Verification; v != nil {
		src := v.Source
		switch {
		case v.RequiredAction != "":
			return "verification.required_action is free text"
		case !issueflow.SupportsHandoff(v.Schema):
			return closed("verification.schema")
		case !pubRepoRef.MatchString(src.Repo):
			return closed("verification.source.repo")
		case !pubSuccessor.MatchString(src.Brief):
			return closed("verification.source.brief")
		case !pubVersion.MatchString(src.SourceVersion):
			return closed("verification.source.source_version")
		case !pubOpaque.MatchString(src.SourceRevision):
			return closed("verification.source.source_revision")
		case !pubOpaque.MatchString(src.ReceiptID):
			return closed("verification.source.receipt_id")
		case !pubOpaque.MatchString(src.CaseID):
			return closed("verification.source.case_id")
		case !pubSuccessor.MatchString(src.BlockerRef):
			return closed("verification.source.blocker_ref")
		case len(src.Rows) > 64:
			return closed("verification.source.rows")
		case !pubEnum(string(v.Disposition), false, string(issueflow.Unresolved), string(issueflow.NeedsScoping), string(issueflow.Repair), string(issueflow.External), string(issueflow.Duplicate)):
			return closed("verification.disposition")
		case !pubSlug.MatchString(v.DecidingRole):
			return closed("verification.deciding_role")
		case v.Successor != "" && !pubSuccessor.MatchString(v.Successor):
			return closed("verification.successor")
		case !pubEnum(v.ExternalRole, true, "human", "operator", "brief-author", "verifier"):
			return closed("verification.external_role")
		}
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
	out, err := gitRemoteOriginURLs(root, "--push", "--all")
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
