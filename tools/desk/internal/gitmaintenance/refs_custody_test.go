package gitmaintenance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/gitcore"
)

func diskReceipt(t *testing.T, tmp *TempRef) tempReceipt {
	t.Helper()
	body, err := os.ReadFile(tmp.path)
	if err != nil {
		t.Fatal(err)
	}
	var got tempReceipt
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

// regression: #2513
func TestTempPendingBeforeChild(t *testing.T) {
	root, old := refWorld(t)
	tmp := seedTemp(t, root, old)
	prev := refProcess
	t.Cleanup(func() { refProcess = prev })
	entered, release := make(chan tempReceipt, 1), make(chan struct{})
	defer close(release)
	calls := 0
	refProcess = func(ctx context.Context, _ []string, _ []string, _ string, _ io.Writer, _ io.Writer) (int, bool, error) {
		calls++
		entered <- diskReceipt(t, tmp)
		select {
		case <-release:
		case <-ctx.Done():
		}
		return 0, true, nil
	}
	type result struct {
		row TempRefResult
		err error
	}
	finished := make(chan result, 1)
	go func() {
		row, err := tmp.Complete(context.Background())
		finished <- result{row, err}
	}()
	select {
	case receipt := <-entered:
		if !receipt.Complete || !receipt.CleanupPending {
			t.Fatalf("child started before durable completed pending custody: %+v", receipt)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("CAS child did not start")
	}
	rows, err := RecoverTempRefs(context.Background(), root, 256)
	if err != nil || len(rows) != 1 || rows[0].State != "held" || !strings.Contains(rows[0].Reason, "another temporary") {
		t.Fatalf("live cleanup recovery=%+v,%v", rows, err)
	}
	if calls != 1 {
		t.Fatalf("overlapping CAS children: %d", calls)
	}
	// A duplicate producer completion must not rewrite a live child's pending
	// receipt, even when it owns an older pre-CAS in-memory snapshot.
	before, err := os.Stat(tmp.path)
	if err != nil {
		t.Fatal(err)
	}
	row, err := tmp.Complete(context.Background())
	after, statErr := os.Stat(tmp.path)
	if err != nil || row.State != "held" || !row.CleanupPending || statErr != nil || !os.SameFile(before, after) || !diskReceipt(t, tmp).CleanupPending {
		t.Fatalf("duplicate live completion changed custody=%+v,%v stat=%v", row, err, statErr)
	}
	assertRef(t, root, tmp.Name, old)
	// Release this fixture without closing the channel twice in the cleanup.
	release <- struct{}{}
	select {
	case got := <-finished:
		if got.row.State != "held" || !errors.Is(got.err, errCleanupUnknown) {
			t.Fatalf("uncertain completion=%+v,%v", got.row, got.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("CAS child did not finish")
	}
	if !diskReceipt(t, tmp).CleanupPending {
		t.Fatal("uncertain custody marker was cleared")
	}
}

// regression: #2513
func TestTempUncertainAbsent(t *testing.T) {
	root, old := refWorld(t)
	tmp := seedTemp(t, root, old)
	stale := tmp.receipt
	stale.Complete = true
	prev := refProcess
	t.Cleanup(func() { refProcess = prev })
	calls := 0
	refProcess = func(_ context.Context, _ []string, _ []string, _ string, _ io.Writer, _ io.Writer) (int, bool, error) {
		calls++
		refGit(t, root, "update-ref", "-d", tmp.Name)
		return 0, true, nil
	}
	row, err := tmp.Complete(context.Background())
	if row.State != "held" || !errors.Is(err, errCleanupUnknown) {
		t.Fatalf("absent uncertain completion=%+v,%v", row, err)
	}
	assertRef(t, root, tmp.Name, "")
	for _, inspect := range []bool{true, false} {
		var rows []TempRefResult
		if inspect {
			rows, err = InspectTempRefs(context.Background(), root)
		} else {
			rows, err = RecoverTempRefs(context.Background(), root, 256)
		}
		if err != nil || len(rows) != 1 || rows[0].State != "held" || !rows[0].CleanupPending || !strings.Contains(rows[0].Reason, "custody unconfirmed") {
			t.Fatalf("uncertain absent inspect=%v rows=%+v,%v", inspect, rows, err)
		}
	}
	// A caller holding an older eligible snapshot cannot erase the pending marker.
	row, err = cleanTempReceipt(context.Background(), root, tmp.path, stale, false)
	if err != nil || row.State != "held" || !row.CleanupPending || !diskReceipt(t, tmp).CleanupPending || calls != 1 {
		t.Fatalf("stale pending retry=%+v,%v calls=%d", row, err, calls)
	}
}

// regression: #2513
func TestTempCrashPending(t *testing.T) {
	root, old := refWorld(t)
	tmp := seedTemp(t, root, old)
	tmp.receipt.Complete = true
	tmp.receipt.CleanupPending = true
	if err := tmp.save(); err != nil {
		t.Fatal(err)
	}
	refGit(t, root, "update-ref", "-d", tmp.Name)
	rows, err := RecoverTempRefs(context.Background(), root, 256)
	if err != nil || len(rows) != 1 || rows[0].State != "held" || !rows[0].CleanupPending || !diskReceipt(t, tmp).CleanupPending {
		t.Fatalf("crashed coordinator receipt=%+v,%v", rows, err)
	}
}

// regression: #2513
func TestTempConfirmedErrorClears(t *testing.T) {
	for _, diagnostic := range []bool{true, false} {
		t.Run(map[bool]string{true: "diagnostic", false: "moved"}[diagnostic], func(t *testing.T) {
			root, old := refWorld(t)
			tmp := seedTemp(t, root, old)
			prev := refProcess
			t.Cleanup(func() { refProcess = prev })
			refProcess = func(_ context.Context, _ []string, _ []string, _ string, _ io.Writer, stderr io.Writer) (int, bool, error) {
				if diagnostic {
					refGit(t, root, "update-ref", "-d", tmp.Name)
					_, _ = stderr.Write([]byte("fixture warning"))
					return 0, false, nil
				}
				refGit(t, root, "update-ref", tmp.Name, "HEAD")
				return 1, false, errors.New("fixture CAS race")
			}
			row, err := tmp.Complete(context.Background())
			if row.State != "held" || err == nil || (diagnostic && !errors.Is(err, ErrRefDiagnostic)) {
				t.Fatalf("confirmed failed completion=%+v,%v", row, err)
			}
			if diskReceipt(t, tmp).CleanupPending {
				t.Fatal("confirmed stopped child left uncertain custody")
			}
			rows, err := RecoverTempRefs(context.Background(), root, 256)
			want := map[bool]string{true: "absent", false: "held"}[diagnostic]
			if err != nil || len(rows) != 1 || rows[0].State != want {
				t.Fatalf("confirmed retry=%+v,%v want=%s", rows, err, want)
			}
		})
	}
}

// regression: #2513
func TestTempBusyCompletion(t *testing.T) {
	root, old := refWorld(t)
	tmp := seedTemp(t, root, old)
	common, err := gitcore.CommonDir(root)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := OpenLock(filepath.Join(common, "assay-maintenance-temprefs.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := deskkit.TryLockExclusive(lock); err != nil {
		t.Fatal(err)
	}
	defer deskkit.UnlockFile(lock)
	row, err := tmp.Complete(context.Background())
	if err != nil || row.State != "held" || !diskReceipt(t, tmp).Complete || diskReceipt(t, tmp).CleanupPending {
		t.Fatalf("busy producer completion=%+v,%v", row, err)
	}
	if _, err := tmp.Observe(); err == nil {
		t.Fatal("completed producer overwrote its final receipt")
	}
	assertRef(t, root, tmp.Name, old)
	if err := deskkit.UnlockFile(lock); err != nil {
		t.Fatal(err)
	}
	if rows, err := RecoverTempRefs(context.Background(), root, tempRefLimit); err != nil || len(rows) != 1 || rows[0].State != "deleted" {
		t.Fatalf("busy completed recovery=%+v,%v", rows, err)
	}
}

// regression: #2513
func TestTempReceiptReread(t *testing.T) {
	root, old := refWorld(t)
	tmp := seedTemp(t, root, old)
	stale := tmp.receipt
	stale.Complete = true
	row, err := cleanTempReceipt(context.Background(), root, tmp.path, stale, false)
	if err != nil || row.State != "held" || diskReceipt(t, tmp).Complete {
		t.Fatalf("stale completion adopted=%+v,%v", row, err)
	}
	assertRef(t, root, tmp.Name, old)
}

// regression: #2513
func TestTempChangedObservation(t *testing.T) {
	root, old := refWorld(t)
	tmp := seedTemp(t, root, old)
	tree := refGit(t, root, "rev-parse", "HEAD^{tree}")
	moved := refGit(t, root, "commit-tree", tree, "-p", old, "-m", "unheld changed observation")
	refGit(t, root, "update-ref", tmp.Name, moved)
	fresh := tmp.receipt
	fresh.Hash = moved
	if err := saveTempReceipt(tmp.path, fresh); err != nil {
		t.Fatal(err)
	}
	row, err := tmp.Complete(context.Background())
	if row.State != "held" || err == nil || diskReceipt(t, tmp).Complete {
		t.Fatalf("changed producer observation adopted=%+v,%v", row, err)
	}
	assertRef(t, root, tmp.Name, moved)
}

// regression: #2513
func TestTempInspectOverflow(t *testing.T) {
	root, old := refWorld(t)
	tmp := seedTemp(t, root, old)
	for i := 0; i < tempRefLimit; i++ {
		name := filepath.Join(filepath.Dir(tmp.path), fmt.Sprintf("orphan-%03d", i))
		if err := os.WriteFile(name, []byte("unknown ownership\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := InspectTempRefs(context.Background(), root)
	if err == nil || len(rows) != 0 {
		t.Fatalf("overflow inventory returned partial clean state=%+v,%v", rows, err)
	}
	assertRef(t, root, tmp.Name, old)
	if diskReceipt(t, tmp).Complete {
		t.Fatal("inspection changed producer state")
	}
	common, err := gitcore.CommonDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(common, "assay-maintenance-temprefs.lock")); !os.IsNotExist(err) {
		t.Fatal("overflow inventory created write state", err)
	}
}
