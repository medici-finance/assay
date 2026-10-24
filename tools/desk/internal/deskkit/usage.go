package deskkit

// usage-v1 is a local, closed counts-only record. It deliberately has no transcript,
// prompt, model name or arbitrary metadata input. Enrollment skipping belongs to /27.
import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const UsageSchema = "usage-v1"
const UsageFile = "usage.jsonl"

var usageItemRE = regexp.MustCompile(`^(?:[a-z0-9][a-z0-9-]*/[0-9]{2,}|issue-[1-9][0-9]*)$`)

// UsageRecord contains only the run join, bounded categorical dimensions and counts.
type UsageRecord struct {
	Schema       string      `json:"schema"`
	TS           string      `json:"ts"`
	DispatchRef  string      `json:"dispatch_ref"`
	Repo         string      `json:"repo"`
	Item         string      `json:"item"`
	Tier         string      `json:"tier"`
	InputTokens  *VitalField `json:"input_tokens"`
	OutputTokens *VitalField `json:"output_tokens"`
	TotalTokens  *VitalField `json:"total_tokens"`
	CostMicros   *VitalField `json:"cost_micros"`
	WallSeconds  *VitalField `json:"wall_seconds"`
}

func (r UsageRecord) Counts() map[string]*VitalField {
	return map[string]*VitalField{"input_tokens": r.InputTokens, "output_tokens": r.OutputTokens, "total_tokens": r.TotalTokens, "cost_micros": r.CostMicros, "wall_seconds": r.WallSeconds}
}

// ParseUsageCount accepts only a reported nonnegative integer or an explicit failed read.
func ParseUsageCount(s string) (*VitalField, error) {
	if s == "could-not-check" {
		return CouldNotCheckVital(), nil
	}
	if s == "" || strings.Trim(s, "0123456789") != "" {
		return nil, Refused("usage count must be a nonnegative integer or could-not-check")
	}
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil {
		return nil, Refused("usage count exceeds int64")
	}
	return MeasuredInt(n), nil
}

func ValidateUsageRecord(r UsageRecord) error {
	if r.Schema != UsageSchema {
		return Refused("unsupported usage schema")
	}
	t, e := time.Parse(time.RFC3339, r.TS)
	if e != nil || t.Location() != time.UTC {
		return Refused("usage timestamp must be RFC3339 UTC")
	}
	key, _, ok := strings.Cut(r.DispatchRef, "@")
	if !ok || !ValidDispatchRef(key, r.DispatchRef) {
		return Refused("invalid usage dispatch_ref")
	}
	if !ValidDispatchRecordField("repo", r.Repo) {
		return Refused("invalid usage repo")
	}
	if len(r.Item) > 256 || !usageItemRE.MatchString(r.Item) {
		return Refused("invalid usage item")
	}
	good := false
	for _, tier := range DispatchTiers() {
		good = good || r.Tier == tier
	}
	if !good {
		return Refused("invalid usage tier")
	}
	for _, v := range r.Counts() {
		if v == nil {
			continue
		}
		if v.CouldNotCheck {
			if v.Value != nil {
				return Refused("ambiguous usage count")
			}
			continue
		}
		n, ok := v.Value.(int64)
		if !ok || n < 0 {
			return Refused("invalid measured usage count")
		}
	}
	return nil
}

// UnmarshalJSON is deliberately local to this record; VitalField also serves string/float vitals.
func (r *UsageRecord) UnmarshalJSON(b []byte) error {
	fields := map[string]json.RawMessage{}
	decoder := json.NewDecoder(bytes.NewReader(b))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return errors.New("usage record must be an object")
	}
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return errors.New("invalid usage key")
		}
		if _, duplicate := fields[key]; duplicate {
			return errors.New("duplicate usage field")
		}
		var raw json.RawMessage
		if err = decoder.Decode(&raw); err != nil {
			return err
		}
		fields[key] = raw
	}
	if _, err = decoder.Token(); err != nil {
		return err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return errors.New("invalid usage record suffix")
	}
	if len(fields) != 11 {
		return errors.New("usage-v1 requires exactly its eleven fields")
	}
	*r = UsageRecord{}
	for k, p := range map[string]*string{"schema": &r.Schema, "ts": &r.TS, "dispatch_ref": &r.DispatchRef, "repo": &r.Repo, "item": &r.Item, "tier": &r.Tier} {
		raw, ok := fields[k]
		if !ok || bytes.Equal(raw, []byte("null")) {
			return fmt.Errorf("missing usage %s", k)
		}
		if err := json.Unmarshal(raw, p); err != nil {
			return err
		}
		delete(fields, k)
	}
	counts := map[string]**VitalField{"input_tokens": &r.InputTokens, "output_tokens": &r.OutputTokens, "total_tokens": &r.TotalTokens, "cost_micros": &r.CostMicros, "wall_seconds": &r.WallSeconds}
	for k, p := range counts {
		raw, ok := fields[k]
		if !ok {
			return fmt.Errorf("missing usage %s", k)
		}
		delete(fields, k)
		if bytes.Equal(raw, []byte("null")) {
			continue
		}
		s := string(raw)
		if strings.HasPrefix(s, `"`) {
			if err := json.Unmarshal(raw, &s); err != nil {
				return err
			}
			if s != "could-not-check" {
				return errors.New("invalid string usage count")
			}
		}
		v, e := ParseUsageCount(s)
		if e != nil {
			return e
		}
		*p = v
	}
	if len(fields) != 0 {
		return errors.New("unknown usage field")
	}
	return ValidateUsageRecord(*r)
}

// UsageStoreDir resolves configuration before effects, including existing symlink ancestors.
func UsageStoreDir() (string, error) {
	cfg := EffectiveConfig()
	raw, env := os.LookupEnv(EnvUsageDir)
	if !env {
		raw = cfg.UsageDir
	}
	if raw == "" && env {
		return "", Refused("ASSAY_USAGE_DIR is empty")
	}
	if raw == "" {
		d, e := StateDir()
		if e != nil {
			return "", e
		}
		raw = filepath.Join(d, "usage")
	}
	if !filepath.IsAbs(raw) || strings.ContainsAny(raw, ",;\t\n\r\x00") {
		return "", Refused("ASSAY_USAGE_DIR must be one absolute path")
	}
	path := filepath.Clean(raw)
	ancestor := path
	var suffix []string
	for {
		_, e := os.Lstat(ancestor)
		if e == nil {
			break
		}
		if !errors.Is(e, os.ErrNotExist) {
			return "", Refused("cannot inspect usage directory")
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", Refused("cannot resolve usage directory")
		}
		suffix = append(suffix, filepath.Base(ancestor))
		ancestor = parent
	}
	real, e := filepath.EvalSymlinks(ancestor)
	if e != nil {
		return "", Refused("cannot resolve usage directory symlinks")
	}
	for i := len(suffix) - 1; i >= 0; i-- {
		real = filepath.Join(real, suffix[i])
	}
	for cur := real; ; cur = filepath.Dir(cur) {
		_, e := os.Lstat(filepath.Join(cur, ".git"))
		if e == nil {
			return "", Refused("usage store must be outside every git working tree")
		}
		if !errors.Is(e, os.ErrNotExist) {
			return "", Refused("cannot inspect git working tree boundary")
		}
		if filepath.Dir(cur) == cur {
			break
		}
	}
	return real, nil
}

// openUsageStore reuses the existing handle-checked, no-follow file opener. A
// directory check alone cannot stop a same-root symlink into a nested checkout.
func openUsageStore(create bool) (*os.File, error) {
	dir, e := UsageStoreDir()
	if e != nil {
		return nil, e
	}
	if create {
		if e = os.MkdirAll(dir, 0700); e != nil {
			return nil, e
		}
	}
	path := filepath.Join(dir, UsageFile)
	if st, err := os.Lstat(path); err == nil && !st.Mode().IsRegular() {
		return nil, Refused("usage store must be a regular, non-symlink file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return openRosterBeaconFile(path, create)
}

func AppendUsageRecord(r UsageRecord) error {
	r.Schema = UsageSchema
	if r.TS == "" {
		r.TS = time.Now().UTC().Format(time.RFC3339)
	}
	if e := ValidateUsageRecord(r); e != nil {
		return e
	}
	data, e := json.Marshal(r)
	if e != nil {
		return e
	}
	f, e := openUsageStore(true)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = TryLockExclusive(f); e != nil {
		return fmt.Errorf("usage store lock: %w", e)
	}
	defer UnlockFile(f)
	if e = f.Chmod(0600); e != nil {
		return e
	}
	if _, e = f.Seek(0, io.SeekEnd); e != nil {
		return e
	}
	_, e = f.Write(append(data, '\n'))
	return e
}

func ReadUsageRecords() ([]UsageRecord, error) {
	f, e := openUsageStore(false)
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	defer f.Close()
	if e = TryLockExclusive(f); e != nil {
		return nil, e
	}
	defer UnlockFile(f)
	var out []UsageRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4096), 64*1024)
	for line := 1; sc.Scan(); line++ {
		var r UsageRecord
		if e = json.Unmarshal(sc.Bytes(), &r); e != nil {
			return nil, fmt.Errorf("usage line %d: %w", line, e)
		}
		out = append(out, r)
	}
	if e = sc.Err(); e != nil {
		return nil, fmt.Errorf("usage line %d: %w", len(out)+1, e)
	}
	return out, nil
}
