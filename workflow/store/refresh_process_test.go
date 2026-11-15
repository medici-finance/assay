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
	if e = os.WriteFile(p.Acquired, b, 0600); e != nil {
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
				if json.Unmarshal(raw, &ticket) != nil {
					t.Fatal("ticket decode")
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
