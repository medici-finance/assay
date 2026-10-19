package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/custodytest"
)

func TestCommsManifestRoundTrip(t *testing.T) {
	for _, value := range []string{"", "/a path/comms.json", "/a'quoted/$literal.json", `C:\a path\reader's $file.json`} {
		t.Run(value, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "cell.env")
			if err := os.WriteFile(p, []byte("CELL=example\n"), 0600); err != nil {
				t.Fatal(err)
			}
			setEnvKey(p, "CELL_COMMS_CONFIG", value, false)
			for _, goos := range []string{"linux", "windows"} {
				e := envWith(map[string]string{"literal": "wrong"})
				if err := parseCellEnvFor(goos, e, p); err != nil || e.Get("CELL_COMMS_CONFIG") != value {
					t.Fatalf("%s round trip: got %q want %q err %v", goos, e.Get("CELL_COMMS_CONFIG"), value, err)
				}
			}
			e := effectiveCellEnv(p, []string{"CELL_COMMS_CONFIG=" + value})
			if e.Get("CELL_COMMS_CONFIG") != value {
				t.Fatalf("transaction resolution corrupted path: %q", e.Get("CELL_COMMS_CONFIG"))
			}
		})
	}
}

func TestCommsCommandQuoting(t *testing.T) {
	c := &Cell{Name: "example", Dir: filepath.Join(string(filepath.Separator), "cells with ' and $", "example")}
	for _, goos := range []string{"linux", "windows"} {
		for _, cockpit := range cockpitValues {
			if cockpit == "auto" {
				continue
			}
			sh := paneShellFor(goos, cockpit)
			got := c.commsCmdIn(sh, "/tools with ' and $/cellctl")
			want := sh.invoke(sh.quote("/tools with ' and $/cellctl")) + " --cells-root " + sh.quote(filepath.Dir(c.Dir)) + " comms 'example' run"
			if got != want || strings.Contains(got, "--cadence") {
				t.Fatalf("%s/%s: %s", goos, cockpit, got)
			}
		}
	}
}

func TestCommsStopFailure(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(map[bool]string{false: "storage-error", true: "children-uncertain"}[uncertain], func(t *testing.T) {
			c := &Cell{Name: "example", Dir: custodytest.PrivateTempDir(t)}
			l, err := cellcadence.Acquire(c.commsDir())
			if err != nil {
				t.Fatal(err)
			}
			started := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				err := l.RunInteractive(context.Background(), c.Name, "comms", func(context.Context) cellcadence.Result {
					close(started)
					for {
						if _, err := os.Lstat(filepath.Join(c.commsDir(), "STOP")); err == nil {
							break
						}
						time.Sleep(time.Millisecond)
					}
					return cellcadence.Result{Uncertain: uncertain, Err: errors.New("fixture shutdown failure"), ExitCode: 1}
				})
				l.Close()
				done <- err
			}()
			<-started
			if err := c.stopComms(); err == nil {
				t.Fatal("failed shutdown reported success")
			}
			if err := <-done; err == nil {
				t.Fatal("supervisor dropped failure")
			}
			s, err := c.commsState()
			if err != nil || s.Running != uncertain {
				t.Fatalf("checkpoint changed: %+v %v", s, err)
			}
		})
	}
}

func TestCommsCancelClassification(t *testing.T) {
	if !commsCancelOnly(errors.Join(context.Canceled, context.Canceled)) {
		t.Fatal("plain cancellation refused")
	}
	for _, err := range []error{errors.New("storage failure"), errors.Join(context.Canceled, errors.New("storage failure"))} {
		if commsCancelOnly(err) {
			t.Fatal("shutdown failure hidden by cancellation")
		}
	}
}

func TestCommsOrcaEnvelope(t *testing.T) {
	good := `{"ok":true,"result":{"terminal":{"handle":"t"}},"_meta":{"runtimeId":"r"}}`
	if _, err := readOrcaComms([]byte(good)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"{}", strings.Replace(good, "true", "false", 1), strings.Replace(good, `"handle":"t"`, `"handle":""`, 1), strings.Replace(good, `"runtimeId":"r"`, `"runtimeId":""`, 1), good + "{}"} {
		if _, err := readOrcaComms([]byte(bad)); err == nil {
			t.Fatalf("invalid identity accepted: %s", bad)
		}
	}
}

func TestCommsReservation(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	c := &Cell{Name: "example", Dir: custodytest.PrivateTempDir(t)}
	l, err := cellcadence.Acquire(c.commsLaunchDir())
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	p := filepath.Join(c.commsLaunchDir(), "surface.json")
	if err := os.WriteFile(p, []byte(`{"cockpit":"tmux","handle":"","label":"example-comms-pending"}`), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p)
	if err := c.startComms("tmux"); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("pending launch duplicated: %v", err)
	}
	after, _ := os.ReadFile(p)
	if string(after) != string(before) {
		t.Fatal("pending reservation changed")
	}
}

func TestCommsSurfaceRoundTrip(t *testing.T) {
	c := &Cell{Name: "example", Dir: custodytest.PrivateTempDir(t)}
	lease, err := c.acquireCommsLaunch()
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	f, err := os.OpenFile(filepath.Join(c.commsLaunchDir(), "surface.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, surface := range []*commsSurface{
		{Cockpit: "orca", Handle: "terminal-example", Label: "example-comms", RuntimeID: "runtime-example"},
		{Cockpit: "tmux", Handle: "@1", Label: "example-comms"},
	} {
		if err := c.writeCommsSurface(f, surface); err != nil {
			t.Fatal(err)
		}
		got, err := c.readCommsSurface()
		if err != nil || got == nil || *got != *surface {
			t.Fatalf("surface round trip: %+v %v", got, err)
		}
	}
}

// regression: #2790
func TestCommsLeaseContention(t *testing.T) {
	for _, release := range []bool{true, false} {
		t.Run(map[bool]string{true: "transient-probe", false: "persistent-owner"}[release], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := &Cell{Name: "example", Dir: custodytest.PrivateTempDir(t)}
				probe, err := cellcadence.Acquire(c.commsDir())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { probe.Close() })
				type result struct {
					lease *cellcadence.Lease
					err   error
				}
				done := make(chan result, 1)
				go func() {
					lease, err := c.acquireCommsLease()
					done <- result{lease, err}
				}()
				// The launch has reached its retry timer while the probe still
				// owns the lease; no host scheduling delay decides this collision.
				synctest.Wait()
				select {
				case r := <-done:
					if r.lease != nil {
						r.lease.Close()
					}
					t.Fatalf("launch did not wait for the ownership probe: %v", r.err)
				default:
				}
				if release {
					if err := probe.Close(); err != nil {
						t.Fatal(err)
					}
				}
				r := <-done
				if release {
					if r.err != nil {
						t.Fatalf("released probe still prevents startup: %v", r.err)
					}
					if r.lease == nil {
						t.Fatal("launch returned no lease")
					}
					r.lease.Close()
				} else {
					if !errors.Is(r.err, cellcadence.ErrBusy) || r.lease != nil {
						t.Fatalf("persistent owner was not refused: lease=%v err=%v", r.lease, r.err)
					}
					if other, err := cellcadence.Acquire(c.commsDir()); !errors.Is(err, cellcadence.ErrBusy) {
						if other != nil {
							other.Close()
						}
						t.Fatalf("persistent owner's lease changed: %v", err)
					}
				}
			})
		})
	}
}
