package perfrec

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// releaseV2 is a valid perf-v2 record: every roster counter is listed once, http measured.
func releaseV2() Record {
	counters := map[Counter]int64{CounterHTTP: 3}
	var un []Counter
	for _, c := range Counters() {
		if c != CounterHTTP {
			un = append(un, c)
		}
	}
	return Record{Schema: SchemaV2, TS: "2026-11-10T10:00:00Z", Tool: "deskpr", Verb: "create", Role: RoleNone,
		Boundary: BoundaryProcess, WallMs: 100, CPU: CPUUnsupported, Result: ResultOK, Phases: []PhaseStat{},
		Counters: counters, Unavailable: un, SourceSHA: "1a43034839", Install: strings.Repeat("a", 32)}
}

// TestV5CarriesReleaseTag: a perf-v2 record from a release build becomes perf-v5 carrying the
// stamped tag, and the line round-trips exactly — the v2 field set plus releaseTag, nothing else.
// regression: #6141
func TestV5CarriesReleaseTag(t *testing.T) {
	for _, tag := range []string{"v1.0.82", "v1.0.41+hotfix-0123456789ab", "statusgen/v0.8.0"} {
		r := WithRelease(releaseV2(), tag)
		if r.Schema != SchemaV5 || r.ReleaseTag != tag {
			t.Fatalf("WithRelease(%q) = schema %q tag %q; want %s carrying the tag", tag, r.Schema, r.ReleaseTag, SchemaV5)
		}
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("perf-v5 does not encode: %v", err)
		}
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(b, &keys); err != nil {
			t.Fatal(err)
		}
		if string(keys["releaseTag"]) != `"`+tag+`"` || len(keys) != 17 {
			t.Errorf("perf-v5 line %s: want the 16 perf-v2 keys plus releaseTag", b)
		}
		got, err := Decode(b)
		if err != nil || !reflect.DeepEqual(got, r) {
			t.Errorf("perf-v5 round trip = %+v, %v\nwant %+v", got, err, r)
		}
	}
}

// TestWithReleaseKeepsOthers: no tag is ever guessed. A dev or unstamped build, a sha, or any
// free text keeps the record perf-v2 with no tag; a v1, v3 or already-tagged record is unchanged.
func TestWithReleaseKeepsOthers(t *testing.T) {
	v2 := releaseV2()
	for _, tag := range []string{"", "dev", "1a43034839", "v1.0", "v1.0.82 extra", "v1.0.82\n", "../v1.0.82",
		"V1.0.82", "https://forge.example/v1.0.82", "v1.0.82+hotfix-xyz", "v1.0.82+hotfix-0123456789abc"} {
		if r := WithRelease(v2, tag); !reflect.DeepEqual(r, v2) {
			t.Errorf("WithRelease(%q) = schema %q tag %q; want the perf-v2 record unchanged", tag, r.Schema, r.ReleaseTag)
		}
	}
	v1 := New("deskpr", "create", RoleNone, 0, 0, "1a43034839", time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC))
	if r := WithRelease(v1, "v1.0.82"); !reflect.DeepEqual(r, v1) {
		t.Errorf("WithRelease on perf-v1 = %+v; perf-v1 is frozen and never tagged", r)
	}
	v3 := v2
	v3.Schema = SchemaV3
	if r := WithRelease(v3, "v1.0.82"); !reflect.DeepEqual(r, v3) {
		t.Errorf("WithRelease on perf-v3 = %+v; want it unchanged", r)
	}
	tagged := WithRelease(v2, "v1.0.82")
	if r := WithRelease(tagged, "v1.0.83"); !reflect.DeepEqual(r, tagged) {
		t.Errorf("WithRelease re-tagged a perf-v5 record: %+v", r)
	}
}

// TestV5StrictCodec: the tag lives in perf-v5 alone. No other schema writes or reads it, a
// perf-v5 line without a valid tag is refused, and unknown keys stay refused.
// regression: #6141
func TestV5StrictCodec(t *testing.T) {
	v2 := releaseV2()
	v2.ReleaseTag = "v1.0.82"
	if _, err := json.Marshal(v2); !errors.Is(err, ErrSchemaReuse) {
		t.Errorf("perf-v2 carrying releaseTag encoded (err %v); its field set is frozen", err)
	}
	v1 := New("deskpr", "create", RoleNone, 0, 0, "1a43034839", time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC))
	v1.ReleaseTag = "v1.0.82"
	if _, err := json.Marshal(v1); !errors.Is(err, ErrSchemaReuse) {
		t.Errorf("perf-v1 carrying releaseTag encoded (err %v); it is frozen", err)
	}
	bad := WithRelease(releaseV2(), "v1.0.82")
	bad.ReleaseTag = "free text"
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("perf-v5 with a free-text tag encoded")
	}
	legacy := WithRelease(releaseV2(), "v1.0.82")
	legacy.ForgeCalls = 1
	if _, err := json.Marshal(legacy); !errors.Is(err, ErrSchemaReuse) {
		t.Errorf("perf-v5 carrying a legacy counter encoded (err %v)", err)
	}

	good, err := json.Marshal(WithRelease(releaseV2(), "v1.0.82"))
	if err != nil {
		t.Fatal(err)
	}
	line := string(good)
	for name, l := range map[string]string{
		"no tag":        strings.Replace(line, `,"releaseTag":"v1.0.82"`, "", 1),
		"free text tag": strings.Replace(line, `"releaseTag":"v1.0.82"`, `"releaseTag":"see https://forge.example"`, 1),
		"unknown key":   strings.Replace(line, `"releaseTag"`, `"extra":1,"releaseTag"`, 1),
		"v2 with tag":   strings.Replace(line, `"schema":"perf-v5"`, `"schema":"perf-v2"`, 1),
		"v1 with tag": `{"schema":"perf-v1","ts":"2026-11-10T10:00:00Z","tool":"deskpr","verb":"create","role":"none",` +
			`"wallMs":1,"exit":0,"result":"ok","childProcs":0,"childWallMs":0,"forgeCalls":0,"sourceSHA":"1a43034839",` +
			`"install":"` + strings.Repeat("a", 32) + `","releaseTag":"v1.0.82"}`,
		"bad interval": strings.Replace(line, `"boundary":"process-init"`, `"boundary":"elsewhere"`, 1),
	} {
		if l == line {
			t.Fatalf("%s: the fixture edit did not apply to %s", name, line)
		}
		if r, err := Decode([]byte(l)); err == nil {
			t.Errorf("%s: decoded %+v; want the line refused", name, r)
		}
	}
}
