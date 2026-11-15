package auth

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/celllaunch"
	"github.com/medici-finance/assay/tools/desk/internal/commstransport"
)

// OperatorCommand is accepted only on the operator-only control channel. It cannot
// carry signing material, an arbitrary mint payload, a role or a source selector.
// Registration observations must be freshly obtained by the protected launcher.
type OperatorCommand struct {
	Operation         ControlOperation
	ExpectedVersion   uint64
	LaunchID, GrantID string
	Observation       *celllaunch.ContainerObservation
	Lease             time.Duration
	Policy            *GrantScope
}

func (c *Controller) ServeControl(ctx context.Context) error {
	return c.serve(ctx, c.resources.Private, "control.sock", c.controlSlots, func(ctx context.Context, q wireRequest) wireResponse {
		if q.Schema != Schema || q.Operation != "control" || q.Probe != nil || ((q.Readback && q.Control != nil) || (!q.Readback && q.Control == nil)) {
			return heldResponse()
		}
		if q.Readback {
			r, e := c.ControlState(ctx)
			if e != nil {
				return heldResponse()
			}
			return wireResponse{Schema: Schema, Receipt: &r}
		}
		x := q.Control
		cmd := ControlCommand{Operation: x.Operation, ExpectedVersion: x.ExpectedVersion, GrantID: x.GrantID, Lease: x.Lease, Policy: x.Policy}
		if x.Operation == Register || x.Operation == Advance {
			c.mu.Lock()
			policy, ok := c.policies[x.LaunchID]
			c.mu.Unlock()
			if !ok || x.Observation == nil {
				return heldResponse()
			}
			permit, e := celllaunch.NewAuthorityPermit(policy.Launch)
			if e != nil {
				return heldResponse()
			}
			cmd.Launch = &LaunchBinding{ID: x.LaunchID, Permit: permit, Observation: *x.Observation}
		} else if x.LaunchID != "" || x.Observation != nil {
			return heldResponse()
		}
		r, e := c.ApplyControl(ctx, cmd)
		if e != nil {
			return heldResponse()
		}
		return wireResponse{Schema: Schema, Receipt: &r}
	})
}

// RequestControl is the explicit protected operator client. Query/grant paths
// never invoke it and the qualified query/caller profile lacks this volume.
func RequestControl(ctx context.Context, path string, max int, timeout time.Duration, command *OperatorCommand) (ControlReceipt, error) {
	if ctx == nil || !filepath.IsAbs(path) || !text(path) || max <= 0 || max > celllaunch.MaxRecordBytes || timeout <= 0 {
		return ControlReceipt{}, ErrConfig
	}
	call, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, e := commstransport.DialUnixContext(call, path, timeout)
	if e != nil {
		return ControlReceipt{}, ErrHeld
	}
	defer conn.Close()
	stop := context.AfterFunc(call, func() { conn.Close() })
	defer stop()
	deadline, _ := call.Deadline()
	if conn.SetDeadline(deadline) != nil {
		return ControlReceipt{}, ErrHeld
	}
	b, e := encode(wireRequest{Schema: Schema, Operation: "control", Control: command, Readback: command == nil}, max)
	if e != nil {
		return ControlReceipt{}, ErrHeld
	}
	if _, e = io.Copy(conn, bytes.NewReader(b)); e != nil {
		return ControlReceipt{}, ErrHeld
	}
	u, ok := conn.(*net.UnixConn)
	if !ok || u.CloseWrite() != nil {
		return ControlReceipt{}, ErrHeld
	}
	var r wireResponse
	if readFrame(conn, max, &r) != nil || r.Schema != Schema || r.Error != "" || r.Grant != nil || r.Proof != nil || r.Receipt == nil || call.Err() != nil {
		return ControlReceipt{}, ErrHeld
	}
	return *r.Receipt, nil
}

// VerifierConfigs is public policy/bootstrap data written only by the producer
// into the qualified read-only bootstrap view. It is not a current-state record.
type VerifierConfigs struct {
	Schema  string
	Entries map[string]VerifierConfig
}

// LoadVerifierConfig reads public policy only from an independently qualified
// bootstrap view. The returned record does not establish current authority.
func LoadVerifierConfig(root *os.Root, id string, max int) (VerifierConfig, error) {
	if root == nil || !name(id) || max <= 0 || max > celllaunch.MaxRecordBytes {
		return VerifierConfig{}, ErrConfig
	}
	st, e := root.Lstat("verifiers.json")
	if e != nil || !st.Mode().IsRegular() {
		return VerifierConfig{}, ErrHeld
	}
	f, e := root.Open("verifiers.json")
	if e != nil {
		return VerifierConfig{}, ErrHeld
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(st, opened) {
		return VerifierConfig{}, ErrHeld
	}
	var all VerifierConfigs
	if readFrame(f, max, &all) != nil || all.Schema != Schema {
		return VerifierConfig{}, ErrHeld
	}
	c, ok := all.Entries[id]
	if !ok {
		return VerifierConfig{}, ErrHeld
	}
	return c, nil
}

func (c *Controller) verifierConfigs(boot Bootstrap) VerifierConfigs {
	m := VerifierConfigs{Schema: Schema, Entries: map[string]VerifierConfig{}}
	for id, p := range c.policies {
		m.Entries[id] = VerifierConfig{Schema: Schema, Profile: Profile, Issuer: c.config.Issuer, Audience: c.config.Audience, Bootstrap: boot, Policy: p.Scope, Launch: p.Launch, MaxGrantLease: c.config.MaxGrantLease, ClockSkew: c.config.ClockSkew, CurrentReadLease: c.config.CurrentReadLease, MaxRevocationBound: c.config.MaxRevocationBound, MaxFrameBytes: c.config.MaxFrameBytes}
	}
	return m
}

func (c *Controller) publishVerifierConfigs(boot Bootstrap) error {
	b, e := encode(c.verifierConfigs(boot), c.config.MaxFrameBytes)
	if e != nil {
		return e
	}
	name := "verifiers-" + boot.Instance + ".tmp"
	f, e := c.resources.Bootstrap.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return ErrHeld
	}
	_, we := f.Write(b)
	se := f.Sync()
	ce := f.Close()
	if we != nil || se != nil || ce != nil {
		c.resources.Bootstrap.Remove(name)
		return ErrHeld
	}
	if c.resources.Bootstrap.Rename(name, "verifiers.json") != nil {
		c.resources.Bootstrap.Remove(name)
		return ErrHeld
	}
	return nil
}

func operationDigest(x OperatorCommand) string {
	b, _ := encode(x, celllaunch.MaxRecordBytes)
	return digest(b)
}
func controlCommandDigest(c ControlCommand) string {
	x := OperatorCommand{Operation: c.Operation, ExpectedVersion: c.ExpectedVersion, GrantID: c.GrantID, Lease: c.Lease, Policy: c.Policy}
	if c.Launch != nil {
		x.LaunchID = c.Launch.ID
		o := c.Launch.Observation
		x.Observation = &o
	}
	return operationDigest(x)
}
