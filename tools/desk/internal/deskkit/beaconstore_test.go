package deskkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func beaconStoreFixture(t *testing.T) string {
	t.Helper()
	previous := dirOverride
	dirOverride = t.TempDir()
	t.Cleanup(func() { dirOverride = previous })
	return dirOverride
}

func beaconStoreSeed(t *testing.T, session, contents string) string {
	t.Helper()
	path, err := AckBeaconPath(session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Subprocesses share only fixture files, not Go mutexes or memory. The test binary
// itself is the helper, so these scenarios can execute natively on Windows too.
func TestBeaconStoreProcessHelper(t *testing.T) {
	mode := os.Getenv("ASSAY_BEACON_TEST_MODE")
	if mode == "" {
		return
	}
	dirOverride = os.Getenv("ASSAY_BEACON_TEST_DIR")
	if dirOverride == "" {
		t.Fatal("missing isolated helper directory")
	}
	session := "concurrent"
	actor := os.Getenv("ASSAY_BEACON_TEST_ACTOR")
	if mode == "hold" {
		_, err := MutateRosterBeacon(session, func(obj map[string]json.RawMessage) (BeaconAction, error) {
			obj["uncommitted"] = json.RawMessage(`true`)
			if err := os.WriteFile(filepath.Join(dirOverride, "entered"), []byte("ready"), 0600); err != nil {
				return BeaconKeep, err
			}
			// The parent kills this process while it owns the sidecar lock.
			for {
				time.Sleep(time.Second)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	for i := 0; i < 24; i++ {
		var err error
		switch mode {
		case "ack":
			_, err = AppendAck(session, AckRecord{Role: "worker-desk", Restatement: fmt.Sprintf("actor-%s-%d", actor, i)})
		case "vitals":
			_, err = MergeResourceVitals(session, ResourceVitals{Tokens: MeasuredInt(int64(i)), Model: MeasuredString("test-model")})
		case "update":
			_, err = MutateRosterBeacon(session, func(obj map[string]json.RawMessage) (BeaconAction, error) {
				var count int
				if raw := obj["updates"]; len(raw) != 0 {
					if err := json.Unmarshal(raw, &count); err != nil {
						return BeaconKeep, err
					}
				}
				// Widen overlap: without the cross-process lock two update actors read
				// the same count and overwrite each other's increment.
				time.Sleep(2 * time.Millisecond)
				obj["updates"] = json.RawMessage(strconv.Itoa(count + 1))
				return BeaconWrite, nil
			})
		default:
			t.Fatalf("unknown helper mode %q", mode)
		}
		if err != nil {
			t.Fatalf("%s iteration %d: %v", mode, i, err)
		}
	}
}

func beaconStoreProcess(t *testing.T, dir, mode, actor string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestBeaconStoreProcessHelper$", "-test.timeout=30s")
	cmd.Env = append(os.Environ(), "ASSAY_BEACON_TEST_DIR="+dir, "ASSAY_BEACON_TEST_MODE="+mode, "ASSAY_BEACON_TEST_ACTOR="+actor)
	return cmd
}

func TestBeaconStoreConcurrentProcessesPreserveFields(t *testing.T) {
	dir := beaconStoreFixture(t)
	// A large foreign field makes truncating publication visible to the reader,
	// which deliberately takes no lock.
	foreign := strings.Repeat("unowned-data-", 8192)
	seed, _ := json.Marshal(map[string]any{"session": "concurrent", "foreign": foreign, "updates": 0})
	path := beaconStoreSeed(t, "concurrent", string(seed))
	type result struct {
		mode   string
		output []byte
		err    error
	}
	results := make(chan result, 8)
	for actor, mode := range []string{"ack", "ack", "ack", "ack", "vitals", "vitals", "update", "update"} {
		cmd := beaconStoreProcess(t, dir, mode, strconv.Itoa(actor))
		go func() { out, err := cmd.CombinedOutput(); results <- result{mode, out, err} }()
	}
	finished, reads := 0, 0
	var readErr error
	for finished < 8 {
		select {
		case result := <-results:
			finished++
			if result.err != nil {
				t.Errorf("%s subprocess: %v\n%s", result.mode, result.err, result.output)
			}
		default:
			data, err := os.ReadFile(path)
			if err == nil && !json.Valid(data) {
				err = fmt.Errorf("reader saw incomplete JSON (%d bytes)", len(data))
			}
			if err != nil && readErr == nil {
				readErr = err
			}
			reads++
			time.Sleep(time.Millisecond)
		}
	}
	if readErr != nil {
		t.Error(readErr)
	}
	if reads == 0 {
		t.Fatal("reader never observed concurrent writers")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var obj struct {
		Foreign  string      `json:"foreign"`
		Acks     []AckRecord `json:"acks"`
		Updates  int         `json:"updates"`
		Resource struct {
			Tokens int    `json:"tokens"`
			Model  string `json:"model"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	if obj.Foreign != foreign {
		t.Error("unknown field changed or disappeared")
	}
	if obj.Updates != 48 {
		t.Errorf("updates=%d, want 48; read/modify/write lost updates", obj.Updates)
	}
	if len(obj.Acks) != 96 {
		t.Errorf("acks=%d, want 96; concurrent receipts were lost", len(obj.Acks))
	}
	seen := make(map[string]bool)
	for _, ack := range obj.Acks {
		if seen[ack.Restatement] {
			t.Errorf("duplicate receipt %q", ack.Restatement)
		}
		seen[ack.Restatement] = true
	}
	for actor := 0; actor < 4; actor++ {
		for i := 0; i < 24; i++ {
			key := fmt.Sprintf("actor-%d-%d", actor, i)
			if !seen[key] {
				t.Errorf("missing receipt %q", key)
			}
		}
	}
	if obj.Resource.Tokens != 23 || obj.Resource.Model != "test-model" {
		t.Errorf("resource update lost: %+v", obj.Resource)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("beacon mode=%o, want 600", info.Mode().Perm())
		}
	}
}

func TestBeaconStoreKilledWriterLeavesCommittedJSONAndReleasesLock(t *testing.T) {
	dir := beaconStoreFixture(t)
	seed := `{"session":"concurrent","foreign":{"retained":true}}`
	path := beaconStoreSeed(t, "concurrent", seed)
	cmd := beaconStoreProcess(t, dir, "hold", "0")
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "entered")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("writer did not enter callback: %s", &output)
		}
		time.Sleep(5 * time.Millisecond)
	}
	// The contender must wait while the owner has modified its private object but
	// not published it. This assertion kills a no-lock mutant.
	wrote := make(chan error, 1)
	go func() {
		_, err := AppendAck("concurrent", AckRecord{Role: "worker-desk", Restatement: "after owner exits"})
		wrote <- err
	}()
	select {
	case err := <-wrote:
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("contender completed while owner held transaction: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != seed {
		t.Errorf("unpublished callback changed committed bytes: %q", before)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("expected killed helper to fail")
	}
	select {
	case err := <-wrote:
		if err != nil {
			t.Fatalf("lock not released after owner died: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("writer remained blocked after lock owner's death")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["uncommitted"]; ok {
		t.Fatal("killed callback leaked its uncommitted field")
	}
	if len(obj["foreign"]) == 0 || len(obj["acks"]) == 0 {
		t.Fatalf("committed fields or subsequent receipt missing: %s", data)
	}
}

func TestBeaconStoreMalformedStatePreserved(t *testing.T) {
	beaconStoreFixture(t)
	for name, contents := range map[string]string{"truncated": `{"acks":[`, "empty": "", "whitespace": "  \n", "null": "null", "array": "[]", "scalar": "42", "duplicate": `{"acks":[],"acks":[]}`, "trailing": `{} {}`} {
		t.Run(name, func(t *testing.T) {
			path := beaconStoreSeed(t, name, contents)
			called := false
			if _, err := MutateRosterBeacon(name, func(obj map[string]json.RawMessage) (BeaconAction, error) { called = true; return BeaconWrite, nil }); err == nil {
				t.Error("malformed state accepted")
			}
			if called {
				t.Error("callback invoked with malformed existing state")
			}
			if _, err := AppendAck(name, AckRecord{Restatement: "receipt"}); err == nil {
				t.Error("receipt writer accepted malformed state")
			}
			if _, err := MergeResourceVitals(name, ResourceVitals{}); err == nil {
				t.Error("resource writer accepted malformed state")
			}
			if _, err := LastAckWithin(name, time.Hour, time.Now()); err == nil {
				t.Error("receipt reader treated malformed state as absent")
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != contents {
				t.Errorf("existing bytes changed: %q", got)
			}
		})
	}
}

func TestBeaconStoreAbortKeepDeleteAndEncodeFailure(t *testing.T) {
	beaconStoreFixture(t)
	seed := `{"foreign":"retain"}`
	path := beaconStoreSeed(t, "transaction", seed)
	sentinel := errors.New("callback refused")
	_, err := MutateRosterBeacon("transaction", func(obj map[string]json.RawMessage) (BeaconAction, error) {
		obj["foreign"] = json.RawMessage(`"changed"`)
		return BeaconWrite, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("callback error lost: %v", err)
	}
	for name, mutate := range map[string]func(map[string]json.RawMessage) (BeaconAction, error){
		"keep": func(obj map[string]json.RawMessage) (BeaconAction, error) {
			obj["foreign"] = json.RawMessage(`"changed"`)
			return BeaconKeep, nil
		},
		"bad encoding": func(obj map[string]json.RawMessage) (BeaconAction, error) {
			obj["bad"] = json.RawMessage(`{`)
			return BeaconWrite, nil
		},
	} {
		_, err := MutateRosterBeacon("transaction", mutate)
		if name == "keep" && err != nil {
			t.Fatal(err)
		}
		if name == "bad encoding" && err == nil {
			t.Fatal("bad raw JSON was published")
		}
		got, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(got) != seed {
			t.Errorf("%s changed committed bytes: %q", name, got)
		}
	}
	if _, err := MutateRosterBeacon("transaction", func(map[string]json.RawMessage) (BeaconAction, error) { return BeaconDelete, nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("delete did not remove beacon: %v", err)
	}
	// Unlinking the lock file splits concurrent writers between distinct inodes.
	if _, err := os.Stat(path + ".lock"); err != nil {
		t.Fatalf("stable sidecar disappeared: %v", err)
	}
}

func TestBeaconStoreLockTimeoutDoesNotRunCallback(t *testing.T) {
	beaconStoreFixture(t)
	path := beaconStoreSeed(t, "busy", `{"foreign":"keep"}`)
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := TryLockExclusive(lock); err != nil {
		t.Fatal(err)
	}
	defer UnlockFile(lock)
	called := false
	start := time.Now()
	_, err = MutateRosterBeacon("busy", func(map[string]json.RawMessage) (BeaconAction, error) { called = true; return BeaconWrite, nil })
	if err == nil || called {
		t.Fatalf("contended transaction err=%v callback=%t", err, called)
	}
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Errorf("lock wait not bounded: %s", elapsed)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"foreign":"keep"}` {
		t.Fatalf("contended writer changed bytes: %q", got)
	}
}

func TestBeaconStorePortableSessionNames(t *testing.T) {
	dir := beaconStoreFixture(t)
	for _, session := range []string{"", ".", "..", "../escape", "a/b", `a\b`, "stream:ads", "trailing.", "trailing ", "NUL", "con.txt", "COM1", "LPT9.data", "COM¹", "CONIN$", "bad\x00name", "bad?name", "bad*name"} {
		called := false
		if _, err := MutateRosterBeacon(session, func(map[string]json.RawMessage) (BeaconAction, error) { called = true; return BeaconWrite, nil }); err == nil || called {
			t.Errorf("unsafe session %q: err=%v callback=%t", session, err, called)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("refused names created filesystem entries: %v", entries)
	}
	for _, session := range []string{"worker-desk-123", "session_42", "COM12", "COM0", "company"} {
		if _, err := MutateRosterBeacon(session, func(map[string]json.RawMessage) (BeaconAction, error) { return BeaconWrite, nil }); err != nil {
			t.Errorf("valid session %q: %v", session, err)
		}
	}
}

// The directory-as-destination fault reaches replacement after the temporary
// file has been written and synced. Failure must not delete the destination or
// leave a half-written beacon-shaped file behind.
func TestBeaconStorePublicationFailureCleansTemporary(t *testing.T) {
	dir := t.TempDir()
	destination := filepath.Join(dir, "destination.json")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(destination, "retained")
	if err := os.WriteFile(marker, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeRosterBeacon(destination, []byte(`{"valid":true}`)); err == nil {
		t.Fatal("replacement of directory unexpectedly succeeded")
	}
	got, err := os.ReadFile(marker)
	if err != nil || string(got) != "unchanged" {
		t.Fatalf("destination changed: %q %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "destination.json" {
		t.Fatalf("temporary file leaked after publication failure: %v", entries)
	}
}
