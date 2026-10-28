package deskkit

// verifyoutcomejoin.go — the PURE input-shape checks for the verify-outcome record's join keys
//: which PRs delivered the brief, which run produced the verdict.
//
// Everything here reads only the submitted record's own bytes. It makes no forge read, hashes
// nothing and writes nothing, so the writer can run it before admission: a malformed record
// causes no read the record names, and a caller that is not admitted learns nothing from a
// record that is merely well-formed. The forge-driven halves (dereference, register redaction)
// live in cmd/deskevidence, after admission.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	prRepoPartRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	prShaRe      = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	intLexemeRe  = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})$`)
	// dispatchRefRe is the dispatch-ref suffix grammar with a bounded prefix: 1-226
	// printable-ASCII bytes with no '@', whitespace or control byte, '@', a basic-form UTC
	// timestamp, '.', and 12 lowercase hex characters. 226 + the 30-byte suffix stays within
	// that brief's 256-byte string cap.
	dispatchRefRe = regexp.MustCompile(`^[\x21-\x3F\x41-\x7E]{1,226}@[0-9]{8}T[0-9]{6}Z\.[0-9a-f]{12}$`)
	sha256HexRe   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// DispatchRefShape reports whether ref matches the dispatch-ref dispatch_ref grammar.
func DispatchRefShape(ref string) bool { return dispatchRefRe.MatchString(ref) }

// foldKey maps a JSON object key to a canonical member of its Unicode simple-folding orbit, so
// two keys have the same fold exactly when strings.EqualFold says they match — the equivalence
// encoding/json uses when it matches a key to a struct field.
func foldKey(k string) string {
	var b strings.Builder
	for _, r := range k {
		min := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if f < min {
				min = f
			}
		}
		b.WriteRune(min)
	}
	return b.String()
}

// CheckRecordKeys refuses (exit 5, naming the key) a record in which any JSON object — the
// top level, a prs entry, a row_results entry, any other nested object — repeats a key or
// carries two keys that differ only in letter case. encoding/json keeps the LAST of a repeated
// key and matches a struct field case-insensitively, so such a record can resolve to one value
// in one reader and another in the next; this is the first half of the TARGET-TIE rule.
func CheckRecordKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(raw)))
	if err := scanKeys(dec); err != nil {
		return err
	}
	return nil
}

func scanKeys(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return Refused("refused: invalid --outcome-record JSON: " + err.Error())
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch d {
	case '{':
		seen := map[string]string{} // fold -> first spelling
		for dec.More() {
			kt, kerr := dec.Token()
			if kerr != nil {
				return Refused("refused: invalid --outcome-record JSON: " + kerr.Error())
			}
			key, _ := kt.(string)
			f := foldKey(key)
			if first, dup := seen[f]; dup {
				if first == key {
					return Refused(fmt.Sprintf("refused: duplicate key %q in --outcome-record (a record names each key once)", key))
				}
				return Refused(fmt.Sprintf("refused: keys %q and %q in --outcome-record differ only in letter case", first, key))
			}
			seen[f] = key
			if err := scanKeys(dec); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := scanKeys(dec); err != nil {
				return err
			}
		}
	}
	if _, err := dec.Token(); err != nil { // the closing delimiter
		return Refused("refused: invalid --outcome-record JSON: " + err.Error())
	}
	return nil
}

// JoinInput is the parsed, shape-checked join-key view of one submitted record.
type JoinInput struct {
	// Fields is the record's top-level object, key -> raw value.
	Fields map[string]json.RawMessage
	// PRs are the (clear, input-form) prs entries; PRsPresent is true for an explicit prs key,
	// including an explicit empty list.
	PRs        []DeliveredPR
	PRsPresent bool
	// DispatchRef is the clear input dispatch_ref, "" when absent.
	DispatchRef string
}

// joinTopKeys are the keys this brief adds; a lone key that matches one of them only in letter
// case is refused, so the redaction (which reads exact keys) can never miss what a struct
// decode would have matched.
var joinTopKeys = []string{
	"prs", "prs_unknown_reason", "verifier_tier", "started", "finished", "rows_passed", "rows_total",
	"rows", "row_results", "dispatch_ref", "dispatch_ref_sha256",
}

var prEntryKeys = []string{"repo", "number", "merge_sha", "withheld"}

func caseVariantOf(key string, known []string) (string, bool) {
	for _, k := range known {
		if key != k && strings.EqualFold(key, k) {
			return k, true
		}
	}
	return "", false
}

// ParseJoinInput runs every pure input-shape check of the join keys on one record and returns
// the parsed view. Each failure is exit 5 naming the field. It is meaningful on every record
// whatever its wake_schema or outcome: a record that carries none of the keys passes unchanged.
func ParseJoinInput(raw []byte) (JoinInput, error) {
	var in JoinInput
	if err := json.Unmarshal(bytes.TrimSpace(raw), &in.Fields); err != nil {
		return in, Refused("refused: invalid --outcome-record JSON: " + err.Error())
	}
	for k := range in.Fields {
		if want, bad := caseVariantOf(k, joinTopKeys); bad {
			return in, Refused(fmt.Sprintf("refused: key %q must be spelled %q", k, want))
		}
	}
	if _, has := in.Fields["dispatch_ref_sha256"]; has {
		return in, Refused("refused: dispatch_ref_sha256 is written by the writer only and is never valid input — supply dispatch_ref")
	}

	if v, has := in.Fields["prs"]; has {
		in.PRsPresent = true
		var entries []json.RawMessage
		if err := json.Unmarshal(v, &entries); err != nil || entries == nil {
			return in, Refused("refused: prs must be a JSON list of {repo, number, merge_sha}")
		}
		if len(entries) > 64 {
			return in, Refused("refused: prs lists more than 64 entries")
		}
		for i, e := range entries {
			pr, err := parsePREntry(i, e)
			if err != nil {
				return in, err
			}
			in.PRs = append(in.PRs, pr)
		}
	}
	if v, has := in.Fields["prs_unknown_reason"]; has {
		var reason string
		if err := json.Unmarshal(v, &reason); err != nil || !ValidPRsUnknownReason(reason) {
			return in, Refused("refused: prs_unknown_reason must be one of: " + PRsUnknownReasonNoPRDelivery)
		}
		if !in.PRsPresent || len(in.PRs) != 0 {
			return in, Refused("refused: prs_unknown_reason is valid only beside an explicit empty prs list")
		}
	} else if in.PRsPresent && len(in.PRs) == 0 {
		return in, Refused("refused: prs_unknown_reason is required when prs is an explicit empty list " +
			"(" + PRsUnknownReasonNoPRDelivery + ")")
	}

	if v, has := in.Fields["dispatch_ref"]; has {
		var ref string
		if err := json.Unmarshal(v, &ref); err != nil || !dispatchRefRe.MatchString(ref) {
			return in, Refused("refused: dispatch_ref must be <claim_key>@<YYYYMMDDTHHMMSSZ>.<12 lowercase hex> " +
				"(prefix 1-226 printable ASCII bytes, no '@')")
		}
		in.DispatchRef = ref
	}

	if err := checkRunMetadataShape(in.Fields); err != nil {
		return in, err
	}
	return in, nil
}

func parsePREntry(i int, raw json.RawMessage) (DeliveredPR, error) {
	at := func(f string) string { return "prs[" + strconv.Itoa(i) + "]." + f }
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return DeliveredPR{}, Refused("refused: " + fmt.Sprintf("prs[%d]", i) + " must be an object {repo, number, merge_sha}")
	}
	for k := range obj {
		if want, bad := caseVariantOf(k, prEntryKeys); bad {
			return DeliveredPR{}, Refused(fmt.Sprintf("refused: %s key %q must be spelled %q", fmt.Sprintf("prs[%d]", i), k, want))
		}
		switch k {
		case "repo", "number", "merge_sha":
		case "withheld":
			return DeliveredPR{}, Refused("refused: " + at("withheld") + " is written by the writer only and is never valid input")
		default:
			return DeliveredPR{}, Refused("refused: " + at(k) + " is not a prs field (repo, number, merge_sha)")
		}
	}
	var pr DeliveredPR
	var repo string
	if err := json.Unmarshal(obj["repo"], &repo); err != nil || !validPRRepo(repo) {
		return pr, Refused("refused: " + at("repo") + " must be <owner>/<name> (one '/', each part 1-100 bytes of [A-Za-z0-9._-], neither '.' nor '..')")
	}
	pr.Repo = repo
	n := strings.TrimSpace(string(obj["number"]))
	num, nerr := strconv.Atoi(n)
	if !intLexemeRe.MatchString(n) || nerr != nil || num < 1 {
		return pr, Refused("refused: " + at("number") + " must be a JSON integer >= 1")
	}
	pr.Number = num
	var sha string
	if err := json.Unmarshal(obj["merge_sha"], &sha); err != nil || !prShaRe.MatchString(sha) {
		return pr, Refused("refused: " + at("merge_sha") + " must be a full object name: exactly 40 or 64 lowercase hex characters")
	}
	pr.MergeSHA = sha
	return pr, nil
}

func validPRRepo(repo string) bool {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || strings.Contains(name, "/") {
		return false
	}
	for _, p := range []string{owner, name} {
		if p == "." || p == ".." || !prRepoPartRe.MatchString(p) {
			return false
		}
	}
	return true
}

// ValidPRRepo reports whether repo is <owner>/<name> in the grammar prs entries use.
func ValidPRRepo(repo string) bool { return validPRRepo(repo) }

// checkRunMetadataShape validates the run-metadata keys that are PRESENT, whatever the
// record's schema: types, the closed vocabularies, and the orderings.
func checkRunMetadataShape(f map[string]json.RawMessage) error {
	str := func(key string) (string, bool, error) {
		v, has := f[key]
		if !has {
			return "", false, nil
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return "", true, Refused("refused: " + key + " must be a JSON string")
		}
		return s, true, nil
	}
	count := func(key string) (int, bool, error) {
		v, has := f[key]
		if !has {
			return 0, false, nil
		}
		lex := strings.TrimSpace(string(v))
		n, err := strconv.Atoi(lex)
		if !intLexemeRe.MatchString(lex) || err != nil {
			return 0, true, Refused("refused: " + key + " must be a JSON integer >= 0")
		}
		return n, true, nil
	}

	if tier, has, err := str("verifier_tier"); err != nil {
		return err
	} else if has && !isDispatchTier(tier) {
		return Refused("refused: verifier_tier " + strconv.Quote(tier) + " is not one of " +
			strings.Join(DispatchTiers(), ", ") + " (the brief-frontmatter vocabulary, never a vendor model name)")
	}

	started, hasStarted, err := str("started")
	if err != nil {
		return err
	}
	finished, hasFinished, err := str("finished")
	if err != nil {
		return err
	}
	parse := func(key, v string) (time.Time, error) {
		t, perr := time.Parse(time.RFC3339, v)
		if perr != nil {
			return t, Refused("refused: " + key + " " + strconv.Quote(v) + " is not an RFC3339 timestamp")
		}
		return t, nil
	}
	var ts0, fin, st time.Time
	if hasStarted {
		if st, err = parse("started", started); err != nil {
			return err
		}
	}
	if hasFinished {
		if fin, err = parse("finished", finished); err != nil {
			return err
		}
	}
	if hasStarted && hasFinished && st.After(fin) {
		return Refused("refused: started " + started + " is after finished " + finished)
	}
	if hasStarted || hasFinished {
		tsStr, _, terr := str("ts")
		if terr != nil {
			return terr
		}
		var perr error
		if ts0, perr = time.Parse(time.RFC3339, tsStr); perr != nil {
			return Refused("refused: ts " + strconv.Quote(tsStr) + " is not an RFC3339 timestamp, so started/finished cannot be ordered against it")
		}
		if hasFinished && fin.After(ts0) {
			return Refused("refused: finished " + finished + " is after ts " + tsStr)
		}
		if hasStarted && st.After(ts0) {
			return Refused("refused: started " + started + " is after ts " + tsStr)
		}
	}

	passed, hasPassed, err := count("rows_passed")
	if err != nil {
		return err
	}
	total, hasTotal, err := count("rows_total")
	if err != nil {
		return err
	}
	if hasPassed && hasTotal && passed > total {
		return Refused(fmt.Sprintf("refused: rows_passed %d exceeds rows_total %d", passed, total))
	}

	heldRows := map[VerifyRowID]bool{}
	if v, has := f["rows"]; has {
		var rows []VerifyRowID
		if err := json.Unmarshal(v, &rows); err != nil {
			return Refused("refused: rows must contain positive integers or exact suffixed identities")
		}
		seen := map[VerifyRowID]bool{}
		for _, row := range rows {
			if seen[row] {
				return Refused("refused: rows repeats row " + string(row))
			}
			seen[row] = true
			heldRows[row] = true
		}
	}
	if hasPassed && hasTotal && passed > total-len(heldRows) {
		return Refused(fmt.Sprintf("refused: rows_passed %d exceeds non-held rows %d", passed, total-len(heldRows)))
	}
	if v, has := f["row_results"]; has {
		var entries []json.RawMessage
		if err := json.Unmarshal(v, &entries); err != nil || entries == nil {
			return Refused("refused: row_results must be a JSON list of {row, result}")
		}
		if !hasTotal {
			return Refused("refused: row_results needs rows_total (it covers ALL rows of the brief)")
		}
		if len(entries) != total {
			return Refused(fmt.Sprintf("refused: row_results lists %d rows but rows_total is %d — it covers ALL rows", len(entries), total))
		}
		seen := map[VerifyRowID]bool{}
		pass := 0
		for i, e := range entries {
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(e, &obj); err != nil || obj == nil {
				return Refused(fmt.Sprintf("refused: row_results[%d] must be an object {row, result}", i))
			}
			for k := range obj {
				if k != "row" && k != "result" {
					return Refused(fmt.Sprintf("refused: row_results[%d].%s is not a row_results field (row, result)", i, k))
				}
			}
			var row VerifyRowID
			if err := json.Unmarshal(obj["row"], &row); err != nil {
				return Refused(fmt.Sprintf("refused: row_results[%d].row must be a positive integer or exact suffixed identity", i))
			}
			if seen[row] {
				return Refused(fmt.Sprintf("refused: row_results[%d].row %s repeats", i, row))
			}
			seen[row] = true
			var res string
			if err := json.Unmarshal(obj["result"], &res); err != nil || !ValidRowResult(res) {
				return Refused(fmt.Sprintf("refused: row_results[%d].result must be one of pass, fail, could-not-run, deferred", i))
			}
			if res == RowPass {
				if heldRows[row] {
					return Refused("refused: held row " + string(row) + " also claims PASS")
				}
				pass++
			}
		}
		if hasPassed && pass != passed {
			return Refused(fmt.Sprintf("refused: row_results holds %d pass rows but rows_passed is %d", pass, passed))
		}
		for row := range heldRows {
			if !seen[row] {
				return Refused("refused: held row " + string(row) + " is absent from row_results inventory")
			}
		}
	} else if hasTotal {
		// Without exact results, only the declared contiguous numeric inventory
		// 1..total is known. Never infer a suffix or a foreign held identity.
		for row := range heldRows {
			n, err := strconv.Atoi(string(row))
			if err != nil || n > total {
				return Refused("refused: held row " + string(row) + " is outside count-only inventory 1..rows_total")
			}
		}
	}
	return nil
}

// CheckWakeV1RunFields refuses a verify-wake-v1 record that lacks one of the join keys every
// NEW such record carries — prs (or an explicit empty list beside prs_unknown_reason),
// verifier_tier, started, finished — naming the first missing key. It assumes the record has
// already passed ParseJoinInput, which validates the values and the prs / prs_unknown_reason
// pairing.
func CheckWakeV1RunFields(in JoinInput) error {
	if !in.PRsPresent {
		return Refused("refused: prs — a " + SchemaWakeV1 + " record lists the merged PRs that delivered the brief, " +
			"or `prs: []` with prs_unknown_reason")
	}
	for _, key := range []string{"verifier_tier", "started", "finished"} {
		var s string
		if v, has := in.Fields[key]; !has || json.Unmarshal(v, &s) != nil || strings.TrimSpace(s) == "" {
			return Refused("refused: " + key + " — a " + SchemaWakeV1 + " record carries the run's " + key)
		}
	}
	return nil
}

// ValidSHA256Hex reports whether s is a lowercase-hex sha256.
func ValidSHA256Hex(s string) bool { return sha256HexRe.MatchString(s) }
