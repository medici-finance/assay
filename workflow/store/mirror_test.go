package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/mirror/mirrortest"
	"github.com/pressly/goose/v3"
)

type mirrorRig struct {
	s      *MirrorStore
	db     *DB
	raw    *sql.DB
	config Config
}

func sqlMirror(t *testing.T) mirrorRig {
	t.Helper()
	_, c, raw := fixture(t)
	ctx := context.Background()
	if err := Migrate(ctx, c); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s, err := db.Mirror(MirrorLimits{128, 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	return mirrorRig{s, db, raw, c}
}
func newSQLMirror(t *testing.T) (*MirrorStore, *DB) { r := sqlMirror(t); return r.s, r.db }
func TestPGMirrorConformance(t *testing.T) {
	mirrortest.Check(t, func() mirror.Store { s, _ := newSQLMirror(t); return s })
}
func mirrorInput() (*mirrortest.TestAdmission, mirror.ScanUpdate) {
	a := &mirrortest.TestAdmission{Name: "synthetic-a", Allowed: true}
	scope := mirror.Scope{Repository: mirror.RepositoryID{Provider: mirror.GitHub, Host: "forge.example", Path: "example/project"}, Kind: mirror.Issues, AdmissionID: a.Name}
	now := time.Date(2026, 11, 3, 0, 0, 0, 123456789, time.UTC)
	return a, mirror.ScanUpdate{Scope: scope, Coverage: mirror.Coverage{State: mirror.Complete, LastAttempted: now, LastComplete: now, LastInventory: now, ObservedUpTo: now}, CompleteScan: true, FullInventory: true, Upsert: []mirror.Observation{{ID: mirror.RecordID{Repository: scope.Repository, Kind: "issue", Number: 1}, Title: "synthetic-canary", UpdatedAt: now, ObservedAt: now, InInventory: true}}}
}
func applyMirror(t *testing.T, s *MirrorStore, a mirror.Admission, u mirror.ScanUpdate) mirror.Snapshot {
	t.Helper()
	snap, err := s.Apply(context.Background(), a, u)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}
func readMirror(t *testing.T, s *MirrorStore, a mirror.Admission, scope mirror.Scope) mirror.Snapshot {
	t.Helper()
	snap, err := s.Read(context.Background(), a, scope)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}
func sameMirror(t *testing.T, want, got mirror.Snapshot) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("snapshot differs: want %#v got %#v", want, got)
	}
}
func rawRefused(t *testing.T, db *sql.DB, key []byte, body string, args ...any) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	values := append([]any{key}, args...)
	if _, err = tx.Exec(body, values...); err == nil {
		t.Fatal("invalid raw SQL transition accepted")
	}
}

// F-atomic: rollback and SQL proofs are exercised below the Go validator.
func TestPGMirrorAtomicCoverage(t *testing.T) {
	r := sqlMirror(t)
	a, u := mirrorInput()
	before := applyMirror(t, r.s, a, u)
	key, _ := scopeIndex(u.Scope, 1<<20)
	for name, sql := range map[string]string{
		"complete":   "UPDATE public.mirror_scopes SET version=version+1,state='complete',complete_scan=false,last_attempted=last_attempted+1,last_complete=last_complete+1 WHERE scope_key=$1",
		"inventory":  "UPDATE public.mirror_scopes SET version=version+1,full_inventory=false,last_attempted=last_attempted+1,last_complete=last_complete+1,last_inventory=last_inventory+1 WHERE scope_key=$1",
		"observed":   "UPDATE public.mirror_scopes SET version=version+1,state='partial',reason='truncated',complete_scan=false,last_attempted=last_attempted+1,observed_up_to=observed_up_to+1 WHERE scope_key=$1",
		"regression": "UPDATE public.mirror_scopes SET version=version+1,last_attempted=last_attempted-1,last_complete=last_complete-1,last_inventory=last_inventory-1,observed_up_to=observed_up_to-1 WHERE scope_key=$1",
	} {
		t.Run(name, func(t *testing.T) { rawRefused(t, r.raw, key, sql) })
	}
	tx, err := r.raw.Begin()
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec("UPDATE public.mirror_scopes SET version=version+1,state='partial',reason='truncated',complete_scan=false,full_inventory=false WHERE scope_key=$1", key)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := encodeFact(u.Upsert[0], 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec("INSERT INTO public.mirror_records(scope_key,record_kind,number,payload,in_inventory) VALUES($1,'issue',2,$2,false)", key, payload)
	if err == nil {
		t.Fatal("partial scan inserted an absence")
	}
	tx.Rollback()
	// A never-complete permission hold cannot recover with an SQL NULL inventory proof.
	hold := mirror.ScanUpdate{Scope: u.Scope, Coverage: mirror.Coverage{State: mirror.Unavailable, Reason: mirror.Permission, LastAttempted: u.Coverage.LastAttempted}, RetentionDecisionRequired: true}
	hold.Scope.AdmissionID = "empty-hold"
	ha := &mirrortest.TestAdmission{Name: hold.Scope.AdmissionID, Allowed: true}
	applyMirror(t, r.s, ha, hold)
	hk, _ := scopeIndex(hold.Scope, 1<<20)
	rawRefused(t, r.raw, hk, "UPDATE public.mirror_scopes SET version=version+1,state='complete',reason='',retention_required=false,complete_scan=true,full_inventory=true,last_attempted=last_attempted+1,last_complete=last_attempted+1 WHERE scope_key=$1")
	// A failure after the first upsert must roll back its row and the coverage CAS.
	_, err = r.raw.Exec(`CREATE FUNCTION public.fixture_fail_second() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.number=2 THEN RAISE EXCEPTION 'synthetic row failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fixture_fail_second BEFORE INSERT OR UPDATE ON public.mirror_records FOR EACH ROW EXECUTE FUNCTION public.fixture_fail_second()`)
	if err != nil {
		t.Fatal(err)
	}
	next := u
	next.ExpectedVersion = before.Version
	next.Upsert = append([]mirror.Observation(nil), u.Upsert...)
	next.Upsert[0].Title = "must-rollback"
	two := next.Upsert[0]
	two.ID.Number = 2
	next.Upsert = append(next.Upsert, two)
	next.Coverage.LastAttempted = next.Coverage.LastAttempted.Add(time.Second)
	next.Coverage.LastComplete = next.Coverage.LastAttempted
	next.Coverage.LastInventory = next.Coverage.LastAttempted
	if _, err = r.s.Apply(context.Background(), a, next); err == nil {
		t.Fatal("injected row failure accepted")
	}
	reopened, err := Open(context.Background(), r.config)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	s, _ := reopened.Mirror(r.s.limits)
	sameMirror(t, before, readMirror(t, s, a, u.Scope))
	// Existing facts may be absent only under a qualified full inventory; absence
	// is not closure or deletion, and a later observation can restore membership.
	partial := mirror.ScanUpdate{Scope: u.Scope, ExpectedVersion: before.Version, Coverage: before.Coverage, NotInInventory: []mirror.RecordID{u.Upsert[0].ID}}
	partial.Coverage.State = mirror.Partial
	partial.Coverage.Reason = mirror.Truncated
	if _, err = r.s.Apply(context.Background(), a, partial); !errors.Is(err, mirror.ErrInvalid) {
		t.Fatal("partial inventory marked an existing row absent")
	}
	full := partial
	full.CompleteScan = true
	full.FullInventory = true
	full.Coverage.State = mirror.Complete
	full.Coverage.Reason = ""
	absent := applyMirror(t, r.s, a, full)
	if len(absent.Records) != 1 || absent.Records[0].InInventory || absent.Records[0].State != before.Records[0].State || absent.Records[0].Title != before.Records[0].Title {
		t.Fatal("qualified absence deleted or closed a fact")
	}
	reobserved := u
	reobserved.ExpectedVersion = absent.Version
	if restored := applyMirror(t, r.s, a, reobserved); len(restored.Records) != 1 || !restored.Records[0].InInventory {
		t.Fatal("reobservation did not restore inventory membership")
	}
}

type processPacket struct {
	Config       Config
	Update       mirror.ScanUpdate
	Mode         string
	Ready, Start string
}

func TestPGMirrorProcess(t *testing.T) {
	path := os.Getenv("ASSAY_MIRROR_PROCESS_PACKET")
	if path == "" {
		return
	}
	packetBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var p processPacket
	if err = json.Unmarshal(packetBytes, &p); err != nil {
		t.Fatal(err)
	}
	db, err := Open(context.Background(), p.Config)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, _ := db.Mirror(MirrorLimits{128, 1 << 20})
	a := &mirrortest.TestAdmission{Name: p.Update.Scope.AdmissionID, Allowed: true}
	if p.Ready != "" {
		if err = os.WriteFile(p.Ready, []byte("ready"), 0600); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(15 * time.Second)
		for {
			if _, err = os.Stat(p.Start); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("process barrier timed out")
			}
			time.Sleep(time.Millisecond)
		}
	}
	if p.Mode == "read" {
		snap := readMirror(t, s, a, p.Update.Scope)
		if len(snap.Records) != 1 || snap.Records[0].Title != p.Update.Upsert[0].Title || snap.Version != p.Update.ExpectedVersion {
			t.Fatal("restart lost committed snapshot")
		}
		return
	}
	_, err = s.Apply(context.Background(), a, p.Update)
	if errors.Is(err, mirror.ErrConflict) {
		fmt.Println("MIRROR_RESULT conflict")
	} else if err != nil {
		t.Fatal(err)
	} else {
		fmt.Println("MIRROR_RESULT committed")
	}
}
func processCommand(t *testing.T, p processPacket) *exec.Cmd {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "packet.json")
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPGMirrorProcess$", "-test.v")
	cmd.Env = append(os.Environ(), "ASSAY_MIRROR_PROCESS_PACKET="+path)
	return cmd
}
func waitPath(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("process did not reach barrier")
		}
		time.Sleep(time.Millisecond)
	}
}

// F-cas: separate OS processes race both the absent and existing SQL version.
func TestPGMirrorConcurrentCAS(t *testing.T) {
	r := sqlMirror(t)
	a, u := mirrorInput()
	dir := t.TempDir()
	for round := range 2 {
		if round == 1 {
			u.ExpectedVersion = readMirror(t, r.s, a, u.Scope).Version
		}
		start := filepath.Join(dir, fmt.Sprint("start", round))
		commands := make([]*exec.Cmd, 2)
		outputs := make([]strings.Builder, 2)
		for i := range commands {
			candidate := u
			candidate.Upsert = append([]mirror.Observation(nil), u.Upsert...)
			candidate.Upsert[0].Title = fmt.Sprintf("winner-%d", i)
			ready := filepath.Join(dir, fmt.Sprintf("ready-%d-%d", round, i))
			commands[i] = processCommand(t, processPacket{Config: r.config, Update: candidate, Ready: ready, Start: start})
			commands[i].Stdout = &outputs[i]
			commands[i].Stderr = &outputs[i]
			if err := commands[i].Start(); err != nil {
				t.Fatal(err)
			}
		}
		for i := range commands {
			waitPath(t, filepath.Join(dir, fmt.Sprintf("ready-%d-%d", round, i)))
		}
		if err := os.WriteFile(start, nil, 0600); err != nil {
			t.Fatal(err)
		}
		committed, conflicts := 0, 0
		for i, c := range commands {
			if err := c.Wait(); err != nil {
				t.Fatalf("process: %v %s", err, outputs[i].String())
			}
			out := outputs[i].String()
			if strings.Contains(out, "MIRROR_RESULT committed") {
				committed++
			}
			if strings.Contains(out, "MIRROR_RESULT conflict") {
				conflicts++
			}
		}
		if committed != 1 || conflicts != 1 {
			t.Fatalf("CAS had %d commits and %d conflicts", committed, conflicts)
		}
		snap := readMirror(t, r.s, a, u.Scope)
		if snap.Version != fmt.Sprint(round+1) || len(snap.Records) != 1 {
			t.Fatal("losing process changed snapshot")
		}
	}
}

// F-rebuild: reopening and table loss never restore a stale completeness claim.
func TestPGMirrorRestartRebuild(t *testing.T) {
	r := sqlMirror(t)
	a, u := mirrorInput()
	snap := applyMirror(t, r.s, a, u)
	u.ExpectedVersion = snap.Version
	if out, err := processCommand(t, processPacket{Config: r.config, Update: u, Mode: "read"}).CombinedOutput(); err != nil {
		t.Fatalf("restart: %v %s", err, out)
	}
	if _, err := r.raw.Exec("DROP TABLE public.mirror_records,public.mirror_scopes CASCADE; DROP FUNCTION public.guard_mirror_record(),public.guard_mirror_scope()"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.s.Read(context.Background(), a, u.Scope); err == nil {
		t.Fatal("missing schema read as empty")
	}
	body, err := fs.ReadFile(migrationFS, "00002_mirror.sql")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, r.raw, fstest.MapFS{"00002_mirror.sql": &fstest.MapFile{Data: body}}, goose.WithDisableVersioning(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	reset := readMirror(t, r.s, a, u.Scope)
	if reset.Coverage.State != mirror.NeverRead || reset.Version != "" || len(reset.Records) != 0 {
		t.Fatal("rebuild revived old coverage")
	}
	key, _ := scopeIndex(u.Scope, 1<<20)
	rawRefused(t, r.raw, key, "INSERT INTO public.mirror_scopes(scope_key,provider,host,repository,kind,admission,version,state,last_attempted,last_complete) VALUES($1,'github',$2,$3,'issue',$4,1,'complete',1,1)", []byte(u.Scope.Repository.Host), []byte(u.Scope.Repository.Path), []byte(a.Name))
}

type repositoryAdmission struct {
	id         string
	repository mirror.RepositoryID
}

func (a repositoryAdmission) ID() string { return a.id }
func (a repositoryAdmission) Authorize(_ context.Context, repository mirror.RepositoryID) error {
	if repository != a.repository {
		return mirror.ErrPermission
	}
	return nil
}

type liveAdmission struct {
	id      string
	allowed atomic.Bool
	calls   atomic.Int64
}

func (a *liveAdmission) ID() string { return a.id }
func (a *liveAdmission) Authorize(context.Context, mirror.RepositoryID) error {
	a.calls.Add(1)
	if !a.allowed.Load() {
		return mirror.ErrPermission
	}
	return nil
}
func waitMirrorLock(t *testing.T, raw *sql.DB, fragment string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var n int
		err := raw.QueryRow("SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND position($1 in query)>0", fragment).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("database I/O did not reach lock barrier")
		}
		time.Sleep(time.Millisecond)
	}
}

// F-admission: independently revoke a capability while real database I/O waits.
func TestPGMirrorScopeIsolation(t *testing.T) {
	r := sqlMirror(t)
	a, u := mirrorInput()
	before := applyMirror(t, r.s, a, u)
	foreign := u.Scope.Repository
	foreign.Path = "foreign/project"
	for _, bad := range []mirror.Admission{repositoryAdmission{id: a.Name, repository: foreign}, nil, &mirrortest.TestAdmission{Name: "other", Allowed: true}, &mirrortest.TestAdmission{Name: a.Name, Allowed: false}} {
		snap, err := r.s.Read(context.Background(), bad, u.Scope)
		if !errors.Is(err, mirror.ErrPermission) || !reflect.DeepEqual(snap, mirror.Snapshot{}) {
			t.Fatal("unauthorized read disclosed rows")
		}
		if _, err = r.s.Apply(context.Background(), bad, u); !errors.Is(err, mirror.ErrPermission) {
			t.Fatal("unauthorized direct apply")
		}
	}
	for _, dimension := range []string{"admission", "provider", "host", "kind"} {
		other := u.Scope
		switch dimension {
		case "admission":
			other.AdmissionID = "synthetic-b"
		case "provider":
			other.Repository.Provider = mirror.GitLab
		case "host":
			other.Repository.Host = "second.example"
		case "kind":
			other.Kind = mirror.Changes
		}
		ad := &mirrortest.TestAdmission{Name: other.AdmissionID, Allowed: true}
		empty := readMirror(t, r.s, ad, other)
		if empty.Version != "" || len(empty.Records) != 0 {
			t.Fatal("scope partition leaked rows")
		}
		independent := u
		independent.Scope = other
		independent.Upsert = append([]mirror.Observation(nil), u.Upsert...)
		independent.Upsert[0].ID.Repository = other.Repository
		if other.Kind == mirror.Changes {
			independent.Upsert[0].ID.Kind = "pull-request"
			if other.Repository.Provider == mirror.GitLab {
				independent.Upsert[0].ID.Kind = "merge-request"
			}
		}
		independent.Upsert[0].Title = "separate-" + dimension
		separate := applyMirror(t, r.s, ad, independent)
		if separate.Version != "1" || separate.Records[0].Title != independent.Upsert[0].Title {
			t.Fatal("separate scope version/row lost")
		}
		sameMirror(t, before, readMirror(t, r.s, a, u.Scope))
	}
	live := &liveAdmission{id: a.Name}
	live.allowed.Store(true)
	tx, err := r.raw.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("LOCK public.mirror_records IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	type answer struct {
		snap mirror.Snapshot
		err  error
	}
	done := make(chan answer, 1)
	go func() { s, e := r.s.Read(context.Background(), live, u.Scope); done <- answer{s, e} }()
	waitMirrorLock(t, r.raw, "mirror_records")
	live.allowed.Store(false)
	tx.Rollback()
	got := <-done
	if !errors.Is(got.err, mirror.ErrPermission) || !reflect.DeepEqual(got.snap, mirror.Snapshot{}) {
		t.Fatal("revocation during read disclosed rows")
	}
	live.allowed.Store(true)
	tx, err = r.raw.Begin()
	if err != nil {
		t.Fatal(err)
	}
	key, _ := scopeIndex(u.Scope, 1<<20)
	if _, err = tx.Exec("SELECT number FROM public.mirror_records WHERE scope_key=$1 FOR UPDATE", key); err != nil {
		t.Fatal(err)
	}
	next := u
	next.ExpectedVersion = before.Version
	next.Upsert = append([]mirror.Observation(nil), u.Upsert...)
	next.Upsert[0].Title = "revoked-write"
	go func() { s, e := r.s.Apply(context.Background(), live, next); done <- answer{s, e} }()
	waitMirrorLock(t, r.raw, "INSERT INTO public.mirror_records")
	live.allowed.Store(false)
	tx.Rollback()
	got = <-done
	if !errors.Is(got.err, mirror.ErrPermission) {
		t.Fatal("revoked apply accepted")
	}
	sameMirror(t, before, readMirror(t, r.s, a, u.Scope))
}

// F-presence: SQL storage preserves source absence and exact timestamp instants.
func TestPGMirrorPresenceHistory(t *testing.T) {
	r := sqlMirror(t)
	a, u := mirrorInput()
	u.Coverage.LastAttempted = time.Date(2500, 1, 1, 0, 0, 0, 987654321, time.UTC)
	u.Coverage.LastComplete = u.Coverage.LastAttempted
	u.Coverage.LastInventory = u.Coverage.LastAttempted
	u.Coverage.ObservedUpTo = u.Coverage.LastAttempted
	u.Coverage.Validator = "opaque\x00validator"
	u.Coverage.Revision = "revision\x00"
	unknown := u.Upsert[0]
	unknown.Body = "\x00raw\xffbody"
	unknown.BodyKnown = false
	unknown.UpdatedAt = time.Date(1600, 1, 1, 0, 0, 0, 123456789, time.UTC)
	unknown.ObservedAt = u.Coverage.LastAttempted
	known := unknown
	known.ID.Number = 2
	known.Body = ""
	known.BodyKnown = true
	known.Labels = []string{}
	known.RequestedReviewers = []mirror.Actor{}
	known.Reviews.GitHub = []mirror.GitHubReview{}
	known.Reviews.GitLab = []mirror.GitLabApproval{}
	u.Upsert = []mirror.Observation{unknown, known}
	snap := applyMirror(t, r.s, a, u)
	got := readMirror(t, r.s, a, u.Scope)
	sameMirror(t, snap, got)
	if !reflect.DeepEqual(got.Records, u.Upsert) {
		t.Fatal("presence or exact timestamps lost")
	}
	if !got.Coverage.RetainedFrom.IsZero() {
		t.Fatal("old visible row fabricated history")
	}
	for _, provider := range []mirror.Provider{mirror.GitHub, mirror.GitLab} {
		changes := u
		changes.Scope.Kind = mirror.Changes
		changes.Scope.Repository.Provider = provider
		reviewed := known
		reviewed.ID.Repository = changes.Scope.Repository
		reviewed.ID.Number = 1
		reviewed.ID.Kind = "pull-request"
		reviewed.Labels = []string{"observed-label\x00"}
		reviewed.Author = mirror.Actor{ID: "author-id", Login: "author", Type: "User"}
		reviewed.RequestedReviewers = []mirror.Actor{{ID: "reviewer-id", Login: "reviewer", Type: "User"}}
		reviewed.Reviews = mirror.ReviewObservations{State: mirror.Partial}
		if provider == mirror.GitHub {
			reviewed.Reviews.GitHub = []mirror.GitHubReview{{ID: "review-id", Author: reviewed.Author, State: "APPROVED", Commit: "revision", SubmittedAt: u.Coverage.LastAttempted}}
		} else {
			reviewed.ID.Kind = "merge-request"
			reviewed.Reviews.GitLab = []mirror.GitLabApproval{{User: reviewed.Author, Approved: true, ObservedAt: u.Coverage.LastAttempted}}
		}
		changes.Upsert = []mirror.Observation{reviewed}
		applyMirror(t, r.s, a, changes)
		if observed := readMirror(t, r.s, a, changes.Scope); !reflect.DeepEqual(observed.Records, changes.Upsert) {
			t.Fatal("review provenance, actors or nested timestamps lost")
		}
	}
	payload, err := encodeFact(known, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	for _, corrupt := range [][]byte{append(append([]byte(nil), payload...), 0xff), append(append([]byte(nil), payload...), 1, 0), append([]byte{2}, payload[1:]...)} {
		if _, err = decodeFact(corrupt, 1<<20); !errors.Is(err, mirror.ErrInvalid) {
			t.Fatal("corrupt or trailing frame accepted")
		}
	}
	hold := mirror.ScanUpdate{Scope: u.Scope, ExpectedVersion: snap.Version, Coverage: snap.Coverage, RetentionDecisionRequired: true}
	hold.Coverage.LastAttempted = hold.Coverage.LastAttempted.Add(time.Second)
	hold.Coverage.State = mirror.Unavailable
	hold.Coverage.Reason = mirror.Permission
	held := applyMirror(t, r.s, a, hold)
	if len(held.Records) != 2 || held.Coverage.Reason != mirror.Permission {
		t.Fatal("status-only hold deleted facts")
	}
	q, err := mirror.QueryScope(context.Background(), r.s, a, mirror.Query{Scope: u.Scope, Now: hold.Coverage.LastAttempted, MaxAge: time.Hour})
	if err != nil || q.Available || len(q.Snapshot.Records) != 0 {
		t.Fatal("permission hold disclosed content")
	}
	invalid := hold
	invalid.ExpectedVersion = held.Version
	invalid.Coverage.RetainedFrom = unknown.UpdatedAt
	if _, err = r.s.Apply(context.Background(), a, invalid); !errors.Is(err, mirror.ErrInvalid) {
		t.Fatal("unqualified retention history accepted")
	}
	key, _ := scopeIndex(u.Scope, 1<<20)
	rawRefused(t, r.raw, key, "UPDATE public.mirror_scopes SET version=version+1,retained_from=1 WHERE scope_key=$1")
	bounded, _ := r.db.Mirror(MirrorLimits{1, 1 << 20})
	if _, err = bounded.Read(context.Background(), a, u.Scope); !errors.Is(err, mirror.ErrInvalid) {
		t.Fatal("record budget ignored")
	}
	small, _ := r.db.Mirror(MirrorLimits{128, 32})
	if _, err = small.Read(context.Background(), a, u.Scope); !errors.Is(err, mirror.ErrInvalid) {
		t.Fatal("byte budget ignored")
	}
}

// F-schema: compare the implemented public catalog and contract in both directions.
func TestPGMirrorSchemaBoundary(t *testing.T) {
	r := sqlMirror(t)
	raw, err := os.ReadFile("../../spec/coordination-store-v1.md")
	if err != nil {
		t.Fatal(err)
	}
	expected := contractColumns(string(raw))
	actual, err := schemaColumns(context.Background(), r.raw)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("contract/catalog mismatch: %v versus %v", expected, actual)
	}
	if _, err = r.raw.Exec("ALTER TABLE public.mirror_records DROP COLUMN payload"); err != nil {
		t.Fatal(err)
	}
	changed, err := schemaColumns(context.Background(), r.raw)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(expected, changed) {
		t.Fatal("missing mirror column accepted")
	}
	if _, err = r.raw.Exec("CREATE TABLE public.unexpected_mirror(id integer)"); err != nil {
		t.Fatal(err)
	}
	changed, err = schemaColumns(context.Background(), r.raw)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(expected, changed) {
		t.Fatal("extra mirror table accepted")
	}
	if reflect.DeepEqual(expected[:len(expected)-1], actual) {
		t.Fatal("missing contract entry accepted")
	}
	mod, err := os.ReadFile("../../forgeread/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"jackc/pgx", "pressly/goose", "sqlite", "database/sql"} {
		if strings.Contains(string(mod), forbidden) {
			t.Fatal("mirror kernel acquired persistence dependency")
		}
	}
	workflow, err := os.ReadFile("../../.github/workflows/assay-workflow.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"workflow/**", "forgeread/go.mod", "forgeread/mirror/**", "spec/coordination-store-v1.md", "docs/contracts.md", "tools/desk/internal/deskkit/citrigger_test.go"} {
		if strings.Count(string(workflow), input) != 2 {
			t.Fatalf("CI input is not live on both events: %s", input)
		}
	}
	TestPGFoundationBoundary(t)
}
