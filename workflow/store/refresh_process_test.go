package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/refresh"
)

type refreshProcessPacket struct {
	Config                         Config
	Profile                        RefreshConfig
	Demand                         refresh.Demand
	Ready, Start, Acquired, Finish string
	Mixed                          bool
}

func TestPGRefreshProcess(t *testing.T) {
	path := os.Getenv("ASSAY_REFRESH_PROCESS_PACKET")
	if path == "" {
		return
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var p refreshProcessPacket
	if json.Unmarshal(raw, &p) != nil {
		t.Fatal("invalid process packet")
	}
	db, e := Open(context.Background(), p.Config)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	s, _ := db.Mirror(MirrorLimits{128, 1 << 20})
	g, e := db.Refresh(s, p.Profile, refreshTestAuthority{}, refreshTestClock{})
	if e != nil {
		t.Fatal(e)
	}
	r := refreshRig{mirrorRig: mirrorRig{s: s, db: db, config: p.Config}, g: g, cfg: p.Profile}
	if e = os.WriteFile(p.Ready, []byte("ready"), 0600); e != nil {
		t.Fatal(e)
	}
	waitPath(t, p.Start)
	ticket := acquireRefresh(t, r, p.Demand)
	b, e := json.Marshal(ticket)
	if e != nil {
		t.Fatal(e)
	}
	if e = publishFile(p.Acquired, b); e != nil {
		t.Fatal(e)
	}
	waitPath(t, p.Finish)
	if ticket.Owner {
		a, u := mirrorInput()
		u.Scope = p.Demand.Key.Scope
		u.Upsert[0].ID.Repository = u.Scope.Repository
		now := time.Now().UTC()
		u.Coverage.LastAttempted = now
		u.Coverage.LastComplete = now
		u.Coverage.LastInventory = now
		u.Coverage.ObservedUpTo = now
		u.Upsert[0].UpdatedAt = now
		u.Upsert[0].ObservedAt = now
		receipt := knownRefreshAttempt(t, r, ticket)
		if p.Mixed {
			for range 3 {
				receipt = knownRefreshAttempt(t, r, ticket)
			}
			receipt.HTTP.Value = 4
		}
		c := completionRefresh(r, ticket, &u)
		c.Attempts = receipt
		if e = g.Complete(context.Background(), ticket, s, a, c); e != nil {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	state, e := g.Wait(ctx, ticket)
	if e != nil || state.Pending {
		t.Fatalf("process wait: %+v %v", state, e)
	}
	if _, e = g.Leave(context.Background(), ticket, refresh.Outcome{}); e != nil {
		t.Fatal(e)
	}
	fmt.Println("REFRESH_PROCESS completed")
}

// publishFile makes b appear at path all at once. The parent waits for a
// content file with waitPath, which returns as soon as the path exists, so a
// plain os.WriteFile (create, then write) lets it read an empty or partial
// file. Writing a temp file in the same directory and renaming it over path
// means the path never exists with anything but the whole content.
func publishFile(path string, b []byte) error {
	tmp := path + ".tmp"
	if e := os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}

// regression: #6737
func TestPublishFileWhole(t *testing.T) {
	payload, e := json.Marshal(map[string]string{"pad": strings.Repeat("x", 4<<20)})
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	for i := range 50 {
		path := filepath.Join(dir, fmt.Sprint("ticket", i))
		done := make(chan error, 1)
		go func() { done <- publishFile(path, payload) }()
		// Spin without sleeping: the read must land inside any window in
		// which the path exists before its content is whole.
		for {
			if _, e := os.Stat(path); e == nil {
				break
			}
			select {
			case e := <-done:
				if e != nil {
					t.Fatal(e)
				}
				done <- nil
			default:
			}
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if !json.Valid(raw) {
			t.Fatalf("round %d: read %d of %d bytes", i, len(raw), len(payload))
		}
		if e = <-done; e != nil {
			t.Fatal(e)
		}
	}
}

func refreshProcessCommand(t *testing.T, p refreshProcessPacket) *exec.Cmd {
	t.Helper()
	raw, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "packet.json")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPGRefreshProcess$", "-test.v")
	cmd.Env = append(os.Environ(), "ASSAY_REFRESH_PROCESS_PACKET="+path)
	return cmd
}

// regression: F-refresh-process-ownership-lock-order
func TestPGRefreshGateProcesses(t *testing.T) {
	t.Run("expiry", checkRefreshExpiry)
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprint("mixed=", mixed), func(t *testing.T) {
			r := sqlRefresh(t)
			_, u := mirrorInput()
			dir := t.TempDir()
			start := filepath.Join(dir, "start")
			finish := filepath.Join(dir, "finish")
			cmds := make([]*exec.Cmd, 2)
			outputs := make([]strings.Builder, 2)
			for i := range cmds {
				d := r.demand(u.Scope, fmt.Sprint("caller-", string(rune('a'+i))))
				if mixed {
					d.Key.Scope.Repository.Path = fmt.Sprintf("example/project-%d", i)
				}
				p := refreshProcessPacket{Config: r.config, Profile: r.cfg, Demand: d, Ready: filepath.Join(dir, fmt.Sprint("ready", i)), Start: start, Acquired: filepath.Join(dir, fmt.Sprint("ticket", i)), Finish: finish, Mixed: mixed}
				cmds[i] = refreshProcessCommand(t, p)
				cmds[i].Stdout = &outputs[i]
				cmds[i].Stderr = &outputs[i]
				if e := cmds[i].Start(); e != nil {
					t.Fatal(e)
				}
			}
			for i := range cmds {
				waitPath(t, filepath.Join(dir, fmt.Sprint("ready", i)))
			}
			if e := os.WriteFile(start, nil, 0600); e != nil {
				t.Fatal(e)
			}
			owners := 0
			generation := ""
			for i := range cmds {
				path := filepath.Join(dir, fmt.Sprint("ticket", i))
				waitPath(t, path)
				raw, e := os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				var ticket refresh.Ticket
				if e := json.Unmarshal(raw, &ticket); e != nil {
					t.Fatalf("ticket decode (%d bytes): %v", len(raw), e)
				}
				if ticket.Owner {
					owners++
				}
				if !mixed && generation != "" && generation != ticket.Generation {
					t.Fatal("separate owner generations")
				}
				generation = ticket.Generation
			}
			expected := 1
			if mixed {
				expected = 2
			}
			if owners != expected {
				t.Fatalf("owners=%d want=%d", owners, expected)
			}
			if e := os.WriteFile(finish, nil, 0600); e != nil {
				t.Fatal(e)
			}
			for i, cmd := range cmds {
				if e := cmd.Wait(); e != nil {
					t.Fatalf("process failed: %v %s", e, outputs[i].String())
				}
				if !strings.Contains(outputs[i].String(), "REFRESH_PROCESS completed") {
					t.Fatal("missing process completion")
				}
			}
		})
	}
}
