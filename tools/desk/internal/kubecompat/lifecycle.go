package kubecompat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Size struct{ Width, Height uint16 }
type Terminal struct {
	In       io.Reader
	Out, Err io.Writer
	Sizes    <-chan Size
}
type Termination struct {
	ContainerID string
	PodUID      string
	LaunchID    string
	Clean       bool
	GroupEmpty  bool
	ExitCode    int32
}
type Snapshot struct {
	Busy        bool
	WorkUnknown bool

	StatefulSetUID string
	Replicas       int32
	PodPresent     bool
	Running        bool
	Pending        bool
	Identity       Identity
	Termination    *Termination
}

// Effects is the entire compatibility authority exposed to cellctl. There is no
// arbitrary resource/URL operation, exec, log, create, delete or credential API.
type Effects interface {
	Observe(context.Context, string) (Snapshot, error)
	Scale(context.Context, string, string, int32) error
	Attach(context.Context, string, Identity, Terminal) error
}
type Runtime struct {
	Profile         *Profile
	Digest, CellDir string
	Effects         Effects
	Poll            time.Duration
}

func (r *Runtime) ReceiptPath(role string) string {
	return filepath.Join(r.CellDir, "run", "kubecompat", role+".json")
}
func (r *Runtime) read(role string) (*Receipt, error) {
	v, e := ReadReceipt(r.ReceiptPath(role))
	if e != nil {
		return nil, e
	}
	if e = v.Identity.Valid(r.Profile, r.Digest, role); e != nil {
		return nil, e
	}
	return v, nil
}
func (r *Runtime) observe(ctx context.Context, role string) (Snapshot, error) {
	if _, ok := r.Profile.Roles[role]; !ok {
		return Snapshot{}, errors.New("role is not configured")
	}
	s, e := r.Effects.Observe(ctx, role)
	if e != nil {
		return s, errors.New("execution could not be measured; held for custodian reconciliation")
	}
	if s.StatefulSetUID == "" {
		return s, errors.New("workload UID missing; held")
	}
	if s.PodPresent && !s.Pending {
		if e = s.Identity.Valid(r.Profile, r.Digest, role); e != nil {
			return s, e
		}
		if s.Identity.StatefulSetUID != s.StatefulSetUID {
			return s, errors.New("workload owner changed; held")
		}
	}
	return s, nil
}
func (r *Runtime) Current(ctx context.Context, role string) (Snapshot, error) {
	s, e := r.observe(ctx, role)
	if e != nil {
		return s, e
	}
	v, e := r.read(role)
	if e != nil || v.State != "running" || !s.PodPresent || !s.Running {
		return s, errors.New("no measured current execution; held")
	}
	if e = v.Identity.Same(s.Identity); e != nil {
		return s, e
	}
	return s, nil
}
func (r *Runtime) Start(ctx context.Context, role string) (Identity, error) {
	s, e := r.observe(ctx, role)
	if e != nil {
		return Identity{}, e
	}
	v, re := r.read(role)
	if s.PodPresent {
		if re != nil || v.State != "running" || !s.Running {
			return Identity{}, errors.New("existing execution is unknown or not running; held")
		}
		if e = v.Identity.Same(s.Identity); e != nil {
			return Identity{}, e
		}
		return s.Identity, nil
	}
	if s.Replicas != 0 {
		return Identity{}, errors.New("workload has an unobserved execution; held")
	}
	if re != nil && !os.IsNotExist(re) {
		return Identity{}, errors.New("receipt is unreadable; held")
	}
	if v != nil && (v.State != "clean-stopped" || v.Identity.StatefulSetUID != s.StatefulSetUID) {
		return Identity{}, errors.New("previous execution has no exact clean stop proof; held")
	}
	// Missing receipt permits only the explicit cold-prepared profile. The independent
	// volume launcher still refuses absent, dirty or already consumed first-boot history.
	if e = r.Effects.Scale(ctx, role, s.StatefulSetUID, 1); e != nil {
		return Identity{}, errors.New("start outcome unknown; held, do not retry")
	}
	for {
		n, e := r.observe(ctx, role)
		if e != nil {
			return Identity{}, e
		}
		if n.StatefulSetUID != s.StatefulSetUID {
			return Identity{}, errors.New("workload identity changed during start; held")
		}
		if n.PodPresent && !n.Pending {
			if v != nil && n.Identity.PodUID == v.Identity.PodUID {
				return Identity{}, errors.New("same Pod cannot restart an execution; held")
			}
			if n.Running {
				if n.Identity.RestartCount != 0 {
					return Identity{}, errors.New("restarted container cannot be admitted as a new execution; held")
				}
				if e = WriteReceipt(r.ReceiptPath(role), Receipt{ProfileName, n.Identity, "running"}); e != nil {
					return Identity{}, e
				}
				return n.Identity, nil
			}
		}
		if e = r.wait(ctx); e != nil {
			return Identity{}, e
		}
	}
}
func (r *Runtime) Attach(ctx context.Context, role string, t Terminal) error {
	s, e := r.Current(ctx, role)
	if e != nil {
		return e
	}
	return r.Effects.Attach(ctx, role, s.Identity, t)
}
func (r *Runtime) wait(ctx context.Context) error {
	d := r.Poll
	if d <= 0 {
		d = 100 * time.Millisecond
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return errors.New("execution acknowledgement unavailable; held for custodian reconciliation")
	case <-timer.C:
		return nil
	}
}
func (r *Runtime) Stop(ctx context.Context, role string, custodianConfirmedIdle bool) error {
	if !custodianConfirmedIdle {
		return errors.New("stop held: custodian must confirm interactive idle and reconciled role claims/effects on the plan")
	}
	s, e := r.Current(ctx, role)
	if e != nil {
		return e
	}
	if s.Busy || s.WorkUnknown {
		return errors.New("role work is busy or unmeasured; reconcile before stop")
	}
	want := s.Identity
	// The input frame requests a bounded PID-1 stop. It does not shell, exec or run
	// another agent. A lost response cannot be translated to successful termination.
	frame := fmt.Sprintf("\x1dASSAY_STOP %s %s\n", want.PodUID, r.Profile.Roles[role].LaunchID)
	e = r.Effects.Attach(ctx, role, want, Terminal{In: strings.NewReader(frame), Out: io.Discard, Err: io.Discard})
	if e != nil {
		return errors.New("stop request outcome unknown; held")
	}
	for {
		n, e := r.observe(ctx, role)
		if e != nil {
			return e
		}
		if n.StatefulSetUID != s.StatefulSetUID || !n.PodPresent || n.Identity.PodUID != want.PodUID {
			return errors.New("lost or changed Pod while awaiting exact termination; held")
		}
		term := n.Termination
		if term != nil && term.ContainerID == want.ContainerID && term.PodUID == want.PodUID && term.LaunchID == r.Profile.Roles[role].LaunchID && term.Clean && term.GroupEmpty && term.ExitCode == 0 {
			// Both native exact-container termination and the launcher's group-empty record
			// are required. Ready/NotReady and accepted scaling are never this evidence.
			if e = WriteReceipt(r.ReceiptPath(role), Receipt{ProfileName, want, "clean-stopped"}); e != nil {
				return e
			}
			if e = r.Effects.Scale(ctx, role, s.StatefulSetUID, 0); e != nil {
				return errors.New("execution cleanly stopped but scale outcome unknown; held")
			}
			return nil
		}
		if n.Identity != want && (term == nil || term.ContainerID != want.ContainerID) {
			return errors.New("container identity changed without exact stop proof; held")
		}
		if e = r.wait(ctx); e != nil {
			return e
		}
	}
}
