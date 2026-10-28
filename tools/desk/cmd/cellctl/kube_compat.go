package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellop"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	kc "github.com/medici-finance/assay/tools/desk/internal/kubecompat"
	"github.com/medici-finance/assay/tools/desk/internal/kubecompat/native"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func (c *Cell) kubeSelected() bool {
	return c.Kind == "k8s" && c.Env.Get("CELL_KUBERNETES_CONFIG") != ""
}
func (c *Cell) kubeProfileE() (*kc.Profile, string, error) {
	p, d, e := kc.Load(c.Env.Get("CELL_KUBERNETES_CONFIG"), c.Name)
	if e != nil {
		return nil, "", e
	}
	if c.Harness != "claude" || c.Env.Get("CELL_PROVIDER") != "" || c.Env.Get("CELL_MODEL_POLICY") != "" || c.Env.Get("CELL_ROLE_CONTEXT") != "" || c.Env.Get("CELL_COCKPIT") != "" || (c.Env.Get("CELL_CADENCE") != "" && c.Env.Get("CELL_CADENCE") != "off") {
		return nil, "", errors.New("compatibility requires pinned Claude profile; host policies, role context, providers, cockpits and cadences unsupported")
	}
	for _, role := range c.Roles {
		if _, ok := p.Roles[role]; !ok {
			return nil, "", errors.New("unconfigured compatibility role")
		}
	}
	return p, d, nil
}
func (c *Cell) kubeRuntimeE(contact bool) (*kc.Runtime, error) {
	p, d, e := c.kubeProfileE()
	if e != nil {
		return nil, e
	}
	if _, e = native.Config(p); e != nil {
		return nil, e
	}
	rt := &kc.Runtime{Profile: p, Digest: d, CellDir: c.Dir}
	if contact {
		fmt.Fprintf(os.Stderr, "[kubernetes compatibility] target=%s tls=%s namespace=%s cell=%s profile=%s\n", p.Endpoint, p.TLSServerName, p.Namespace, p.Cell, kc.ProfileName)
		effect, e := native.New(p, d)
		if e != nil {
			return nil, e
		}
		rt.Effects = effect
	}
	return rt, nil
}
func (c *Cell) kubeRuntime(contact bool) *kc.Runtime {
	rt, e := c.kubeRuntimeE(contact)
	if e != nil {
		die("kubernetes compatibility: %v", e)
	}
	return rt
}
func (c *Cell) kubeOptions(o deskInputs) {
	if o.CfgIn != "" || o.Model != "" || o.Provider != "" || o.Harness != "" || o.Cockpit != "" || o.Persist || o.EndStaleLockHolder || o.CadenceFlag != "" || o.BudgetFlag != "" {
		die("kubernetes compatibility uses its pinned image/agent/model/role profile; host launch overrides are unsupported")
	}
}
func (c *Cell) kubeStart(role string) {
	if !valueIn(role, c.Roles) {
		die("role is not enabled in compatibility cell")
	}
	rt := c.kubeRuntime(c.Env.Get("DRY_RUN") != "1")
	if c.Env.Get("DRY_RUN") == "1" {
		fmt.Printf("[dry-run] compatibility start cell=%s role=%s target=%s image=%s; no API contact\n", c.Name, role, rt.Profile.Endpoint, rt.Profile.Roles[role].Image)
		return
	}
	kubeGuard(c, role)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	id, e := rt.Start(ctx, role)
	if e != nil {
		die("kubernetes compatibility start: %v", e)
	}
	fmt.Printf("%s: agent execution observed pod=%s container=%s; attach with cellctl attach %s %s\n", role, id.PodUID, id.ContainerID, c.Name, role)
}
func kubeAdmission(c *Cell, role string) error {
	if e := c.cadenceGuard(role); e != nil {
		return e
	}
	return kubeLocalQuiescence(c, role)
}
func kubeGuard(c *Cell, role string) {
	if e := kubeAdmission(c, role); e != nil {
		die("kubernetes compatibility admission: %v", e)
	}
}

func (c *Cell) kubePreconditionsE(rt *kc.Runtime, role string) ([]cellop.Precondition, error) {
	raw, e := os.ReadFile(rt.ReceiptPath(role))
	value := "absent"
	if e == nil {
		h := sha256.Sum256(raw)
		value = hex.EncodeToString(h[:])
	} else if !os.IsNotExist(e) {
		return nil, errors.New("compatibility receipt cannot be measured")
	}
	return []cellop.Precondition{{Key: "profile", Value: rt.Digest}, {Key: roleKey(role, "receipt"), Value: value}}, nil
}
func (c *Cell) kubePreconditions(rt *kc.Runtime, role string) []cellop.Precondition {
	v, e := c.kubePreconditionsE(rt, role)
	if e != nil {
		die("kubernetes compatibility: %v", e)
	}
	return v
}

func (c *Cell) kubeActions(rt *kc.Runtime, roles []string, verb string) []cellop.Action {
	actions := []cellop.Action{}
	for _, role := range roles {
		summary := "start only a proven cold/clean " + role + " or retain its exact current execution"
		if verb == "down" {
			summary = "custodian confirms " + role + " is interactively idle and all role claims/effects reconciled; stop every execution descendant and require exact container termination"
		}
		actions = append(actions, cellop.Action{ID: verb + "-" + role, Cell: c.Name, Summary: summary, Preconditions: c.kubePreconditions(rt, role)})
	}
	return actions
}
func (c *Cell) kubeVerb(verb string) cellopVerb {
	return cellopVerb{
		Verb: verb,
		Measure: func(c *Cell, a cellop.Action) ([]cellop.Precondition, error) {
			rt, e := c.kubeRuntimeE(false)
			if e != nil {
				return nil, e
			}
			role := strings.TrimPrefix(a.ID, verb+"-")
			if !valueIn(role, c.Roles) {
				return nil, errors.New("foreign compatibility action")
			}
			return c.kubePreconditionsE(rt, role)
		},
		Run: func(c *Cell, a cellop.Action) (string, error) {
			role := strings.TrimPrefix(a.ID, verb+"-")
			if verb == "up" {
				if e := kubeAdmission(c, role); e != nil {
					return "", e
				}
			}
			rt, e := c.kubeRuntimeE(true)
			if e != nil {
				return "", e
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if verb == "down" {
				if e = rt.Stop(ctx, role, true); e != nil {
					return "", e
				}
				return "exact container cleanly terminated; files retained, no conversation/process continuation promised", nil
			}
			_, e = rt.Start(ctx, role)
			return "current pinned agent execution observed", e
		},
	}
}

func (c *Cell) kubeUp(cmd *cobra.Command, v *cli.Values, cfg string) {
	c.kubeOptions(deskInputsFrom(v, cfg))
	if v.String("automate") != "" {
		die("kubernetes compatibility does not support host automation")
	}
	if cmd.Flags().Changed("apply") {
		cellopApply(cmd, c, c.kubeVerb("up"), v.String("apply"))
		return
	}
	roles := c.upTargets(v.Strings("role"), v.Bool("no-the-desk"))
	// Freeze the approved target and credentials once, before any role can start.
	// Later role controls are still rechecked and reported alongside earlier effects.
	rt, runtimeErr := c.kubeRuntimeE(!v.Bool("plan") && c.Env.Get("DRY_RUN") != "1")
	if runtimeErr != nil {
		r := cellop.NewResult("up", c.Name, time.Now())
		r.Finish(cellop.CouldNotCheck, cellop.ReasonUnmeasured, runtimeErr.Error(), time.Now())
		emitResult(cmd, r)
		exitWith(r.ExitCode())
		return
	}
	if v.Bool("plan") {
		cellopPlan(cmd, c, "up", c.kubeActions(rt, roles, "up"))
		return
	}

	r := cellop.NewResult("up", c.Name, time.Now())
	rows := []upRow{}
	for _, role := range roles {
		row := upRow{Role: role, State: upStateUp}
		if c.Env.Get("DRY_RUN") == "1" {
			row.State = upStateWouldStart
			row.Detail = "explicit compatibility profile validated; no API contact"
		} else {
			if e := kubeAdmission(c, role); e != nil {
				row.State = upStateFailed
				row.Detail = e.Error()
			} else {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				id, e := rt.Start(ctx, role)
				cancel()
				if e != nil {
					row.State = upStateFailed
					row.Detail = e.Error()
				} else {
					row.Detail = "pod=" + id.PodUID + " container=" + id.ContainerID + " (files retained; attach is presentation only)"
				}
			}
		}
		rows = append(rows, row)
	}
	r.Finish(cellop.OK, "", "", time.Now())
	withRoles(&r, rows)
	emitResult(cmd, r)
	if r.ExitCode() != 0 {
		exitWith(r.ExitCode())
	}
}
func (c *Cell) kubeDown(cmd *cobra.Command, roles []string) {
	if len(roles) == 0 {
		roles = c.Roles
	}
	for _, r := range roles {
		if !valueIn(r, c.Roles) {
			die("role is not enabled")
		}
	}
	rt := c.kubeRuntime(false)
	if c.Env.Get("DRY_RUN") == "1" {
		fmt.Println("[dry-run] compatibility down: would confirm custodian idle/reconciled work, stop execution group, observe exact container termination; no API contact")
		return
	}
	// Only the selected compatibility invocation gains this conditional gate; legacy
	// down keeps its existing terminal/off-terminal contract.
	markStateChanging(cmd)
	gateStateChanging(cmd, []string{c.Name})
	plan := cellopNewPlan(c, "down", c.kubeActions(rt, roles, "down"))
	cellopApply(cmd, c, c.kubeVerb("down"), plan.ID)
}
func (c *Cell) kubeStatus() {
	rt := c.kubeRuntime(c.Env.Get("DRY_RUN") != "1")
	for _, role := range c.Roles {
		if rt.Effects == nil {
			fmt.Printf("%s: unmeasured (offline profile validated; no API contact)\n", role)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		s, e := rt.Current(ctx, role)
		cancel()
		if e != nil {
			fmt.Printf("%s: held/unmeasured; %s\n", role, e)
		} else {
			fmt.Printf("%s: agent execution observed pod=%s container=%s restart=%d; retained files separate from conversation/process continuation\n", role, s.Identity.PodUID, s.Identity.ContainerID, s.Identity.RestartCount)
		}
	}
}
func attachCmd() *cobra.Command {
	return &cobra.Command{Use: "attach <cell> <role>", Short: "attach to an existing compatibility agent; never start or stop one", Long: "Presentation-only native Kubernetes attach. Reconnect requires the exact current execution tuple.\nDetach with Ctrl-] followed by detach and Enter. Closing this connection does not stop the agent.\nTerminal access and logs carry role credential custody; this is the trusted compatibility profile.", Annotations: map[string]string{leadingArgsKey: "2"}, Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		c := loadCell(args[0])
		if !c.kubeSelected() {
			die("attach requires CELL_KUBERNETES_CONFIG on a k8s cell")
		}
		if !valueIn(args[1], c.Roles) {
			die("role is not enabled")
		}
		rt := c.kubeRuntime(c.Env.Get("DRY_RUN") != "1")
		if rt.Effects == nil {
			fmt.Println("[dry-run] attach-only; no API contact or agent invocation")
			return nil
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var restore func()
		if term.IsTerminal(int(os.Stdin.Fd())) {
			state, e := term.MakeRaw(int(os.Stdin.Fd()))
			if e != nil {
				return e
			}
			restore = func() { _ = term.Restore(int(os.Stdin.Fd()), state) }
			defer restore()
		}
		sizes := make(chan kc.Size, 1)
		go func() {
			defer close(sizes)
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			last := kc.Size{}
			for {
				w, h, e := term.GetSize(int(os.Stdin.Fd()))
				if e == nil {
					next := kc.Size{Width: uint16(w), Height: uint16(h)}
					if next != last {
						select {
						case sizes <- next:
							last = next
						case <-ctx.Done():
							return
						}
					}
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
		e := rt.Attach(ctx, args[1], kc.Terminal{In: &kubeInput{source: os.Stdin, cancel: cancel}, Out: os.Stdout, Err: os.Stderr, Sizes: sizes})
		if ctx.Err() != nil {
			return nil
		}
		return e
	}}
}

// A client detach command is consumed locally. It never enters the supervisor's
// reserved stop protocol or sends a process signal to the agent.
type kubeInput struct {
	source  io.Reader
	cancel  context.CancelFunc
	pending []byte
	output  []byte
	detach  bool
	err     error
}

func (k *kubeInput) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(k.output) > 0 {
		n := copy(p, k.output)
		k.output = k.output[n:]
		return n, nil
	}
	if k.detach {
		k.cancel()
		return 0, context.Canceled
	}
	if k.err != nil {
		return 0, k.err
	}
	b := make([]byte, len(p))
	n, e := k.source.Read(b)
	k.err = e
	for _, c := range b[:n] {
		if len(k.pending) > 0 || c == 0x1d {
			k.pending = append(k.pending, c)
			if string(k.pending) == "\x1ddetach\r" || string(k.pending) == "\x1ddetach\n" {
				k.detach = true
				k.pending = nil
				break
			}
			if !strings.HasPrefix("\x1ddetach\r", string(k.pending)) && !strings.HasPrefix("\x1ddetach\n", string(k.pending)) {
				k.flushPending()
			}
			continue
		}
		k.output = append(k.output, c)
	}
	if e != nil {
		k.flushPending()
	}
	if len(k.output) > 0 || k.detach {
		return k.Read(p)
	}
	return 0, e
}

func (k *kubeInput) flushPending() {
	// Ctrl-] is reserved locally. Preserve unknown escape text, but never forward
	// the control byte that could enter the supervisor's confirmed-stop protocol.
	for _, b := range k.pending {
		if b != 0x1d {
			k.output = append(k.output, b)
		}
	}
	k.pending = nil
}

// Reuse the existing local lease/checkpoint inventory on migration. Selecting a
// remote backend does not erase unfinished local work or release a role claim.
func kubeLocalQuiescence(c *Cell, role string) error {
	busy, e := cellcadence.Probe(c.cadenceDir(role))
	if e != nil || busy {
		return errors.New("local role lease is busy or unmeasured; reconcile before migration")
	}
	prior, e := cellcadence.Read(c.cadenceDir(role))
	if e != nil && !errors.Is(e, cellcadence.ErrNoState) {
		return errors.New("local checkpoint unmeasured; reconcile before migration")
	}
	if e == nil && (prior.Running || prior.Cell != c.Name || prior.Role != role) {
		return errors.New("unfinished or foreign local checkpoint; reconcile before migration")
	}
	inventory := c.reconcileInventory([]string{role}, nil)
	if len(inventory.Orphans) > 0 {
		return errors.New("marked local role execution survives; reconcile before migration")
	}
	for _, entry := range inventory.Entries {
		if entry.Class != reconcileDead {
			return errors.New("local role execution is owned or unmeasured; reconcile before migration")
		}
	}
	return nil
}
