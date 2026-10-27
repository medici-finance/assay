package deskkit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func usageFixture(t *testing.T) UsageRecord {
	t.Helper()
	return UsageRecord{Schema: UsageSchema, TS: "2026-10-06T14:15:02Z", DispatchRef: "assay--example--01@20261006T141502Z.3fa9c01b7d2e", Repo: "example-org/project", Item: "example/01", Tier: "strong"}
}
func usageHome(t *testing.T) string {
	t.Helper()
	withNoRoster(t)
	old := dirOverride
	dirOverride = t.TempDir()
	t.Cleanup(func() { dirOverride = old })
	t.Setenv(EnvUsageDir, filepath.Join(dirOverride, "usage"))
	return dirOverride
}

// regression: F-usage-store-in-repository
func TestUsageStore_RefusesGitWorkTree(t *testing.T) {
	state := usageHome(t)
	repo := t.TempDir()
	if e := os.Mkdir(filepath.Join(repo, ".git"), 0700); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{repo, filepath.Join(repo, "missing", "usage")} {
		t.Setenv(EnvUsageDir, path)
		if e := AppendUsageRecord(usageFixture(t)); e == nil {
			t.Fatal("accepted git worktree")
		}
		if _, e := os.Stat(filepath.Join(path, UsageFile)); !os.IsNotExist(e) {
			t.Fatalf("store written: %v", e)
		}
	}
	link := filepath.Join(t.TempDir(), "link")
	if e := os.Symlink(repo, link); e != nil {
		t.Fatal(e)
	}
	t.Setenv(EnvUsageDir, filepath.Join(link, "new", "usage"))
	if e := AppendUsageRecord(usageFixture(t)); e == nil {
		t.Fatal("accepted symlink into git worktree")
	}
	t.Setenv(EnvUsageDir, "")
	os.Unsetenv(EnvUsageDir)
	dir, e := UsageStoreDir()
	if e != nil || dir != func() string {
		d, err := filepath.EvalSymlinks(state)
		if err != nil {
			t.Fatal(err)
		}
		return filepath.Join(d, "usage")
	}() {
		t.Fatalf("default %q %v", dir, e)
	}
	// A file symlink inside the safe root must not redirect into a nested checkout.
	safe := filepath.Join(state, "redirect")
	nested := filepath.Join(safe, "repo")
	if e := os.MkdirAll(filepath.Join(nested, ".git"), 0700); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(nested, "counts.jsonl")
	if e := os.WriteFile(target, []byte("untouched"), 0644); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(filepath.Join("repo", "counts.jsonl"), filepath.Join(safe, UsageFile)); e != nil {
		t.Fatal(e)
	}
	t.Setenv(EnvUsageDir, safe)
	if e := AppendUsageRecord(usageFixture(t)); e == nil {
		t.Fatal("accepted file symlink into nested git worktree")
	}
	if _, e := ReadUsageRecords(); e == nil {
		t.Fatal("reader followed file symlink")
	}
	data, e := os.ReadFile(target)
	if e != nil || string(data) != "untouched" {
		t.Fatal("redirected write", e)
	}
	info, e := os.Stat(target)
	if e != nil || info.Mode().Perm() != 0644 {
		t.Fatal("redirected chmod", e)
	}
	// A configured roster path is applied rather than merely recognized.
	cfg := parseConfig(ClassReadOnly, "test", map[string]string{EnvUsageDir: filepath.Join(state, "configured")})
	if cfg.UsageDir != filepath.Join(state, "configured") {
		t.Fatal("roster usage key ignored")
	}
	t.Setenv(EnvUsageDir, filepath.Join(state, "outside"))
	if e := AppendUsageRecord(usageFixture(t)); e != nil {
		t.Fatal(e)
	}
	f, e := os.OpenFile(filepath.Join(state, "outside", UsageFile), os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = TryLockExclusive(f); e != nil {
		t.Fatal(e)
	}
	defer UnlockFile(f)
	if e = AppendUsageRecord(usageFixture(t)); e == nil {
		t.Fatal("append ignored an exclusive lock")
	}
}

// regression: F-usage-unknown-count-zero
func TestUsageRecord_ThreeState(t *testing.T) {
	usageHome(t)
	r := usageFixture(t)
	r.OutputTokens = CouldNotCheckVital()
	r.TotalTokens = MeasuredInt(0)
	b, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]json.RawMessage
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	if string(m["input_tokens"]) != "null" || string(m["output_tokens"]) != `"could-not-check"` || string(m["total_tokens"]) != "0" {
		t.Fatalf("three states %s", b)
	}
	if len(r.Counts()) != 5 {
		t.Fatal("count field drift")
	}
	var got UsageRecord
	if e = json.Unmarshal(b, &got); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(r, got) {
		t.Fatalf("round trip %+v", got)
	}
	for _, v := range []string{"-1", "1.5", "NaN", "", `{"message":"CANARY"}`, "9223372036854775808"} {
		if _, e := ParseUsageCount(v); e == nil {
			t.Fatalf("accepted %q", v)
		}
	}
	duplicate := bytes.Replace(b, []byte(`"input_tokens":`), []byte(`"input_tokens":-1,"input_tokens":`), 1)
	if e := json.Unmarshal(duplicate, &got); e == nil {
		t.Fatal("accepted duplicate count hiding a bad value")
	}
	for _, key := range []string{"input_tokens", "output_tokens", "total_tokens", "cost_micros", "wall_seconds"} {
		for _, value := range []string{`-1`, `1.5`, `"1"`, `{}`, `true`} {
			bad := bytes.Replace(b, append([]byte(`"`+key+`":`), m[key]...), []byte(`"`+key+`":`+value), 1)
			if e := json.Unmarshal(bad, &got); e == nil {
				t.Fatalf("accepted %s: %s", key, bad)
			}
		}
	}
	r.Tier = "vendor-model-name"
	if e := AppendUsageRecord(r); e == nil {
		t.Fatal("accepted model name as tier")
	}
	r = usageFixture(t)
	r.InputTokens = MeasuredString("text")
	if e := AppendUsageRecord(r); e == nil {
		t.Fatal("accepted arbitrary measured string")
	}
	r = usageFixture(t)
	if e := AppendUsageRecord(r); e != nil {
		t.Fatal(e)
	}
	d, _ := UsageStoreDir()
	info, e := os.Stat(filepath.Join(d, UsageFile))
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("file permissions %v %v", info, e)
	}
	rows, e := ReadUsageRecords()
	if e != nil || len(rows) != 1 || rows[0].InputTokens != nil {
		t.Fatalf("read %v %v", rows, e)
	}
	for _, tier := range DispatchTiers() {
		r.Tier = tier
		if e := ValidateUsageRecord(r); e != nil {
			t.Fatal(e)
		}
	}
	if strings.Contains(string(b), "CANARY") {
		t.Fatal("canary leaked")
	}
}
