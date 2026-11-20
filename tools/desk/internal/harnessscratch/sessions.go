// Package harnessscratch reports, and on request removes, the per-session scratch
// directories a coding harness keeps under its own temp root (#7274).
//
// The claude harness lays its scratch out as <temp root>/<project slug>/<session id>,
// where <temp root> is the `claude-<uid>` directory under CLAUDE_CODE_TMPDIR (or the
// system default). Nothing removes a session's directory when the session ends, and the
// managed task scratch sweep never looks there, so ended sessions keep everything.
//
// Sessions is the one decision, scoped to one desk: it lists that desk's session
// directories, classifies each from a durable liveness record supplied by the caller, and
// holds any directory that contains Git data. A dry run and an apply run make the same decision; only an apply run removes, and
// it removes only directories whose session a durable record proves ended. A PID, a
// modification time or an age is never evidence here. The function is not scheduled by
// anything in this package.
package harnessscratch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// State is what a durable record says about one harness session.
type State int

const (
	// Unknown means no durable record speaks for the session, or it could not be read.
	// An unknown session is always kept.
	Unknown State = iota
	// Live means a durable record shows the session is still running.
	Live
	// Ended means a durable record shows the session stopped.
	Ended
)

func (s State) String() string {
	switch s {
	case Live:
		return "live"
	case Ended:
		return "ended"
	}
	return "unknown"
}

// Liveness reads a durable record (a roster beacon, a role lease) for one harness session
// id and returns its state and the reason, in words a report can carry.
type Liveness func(session string) (State, string)

// Actions a report row can carry.
const (
	ActionKeep      = "keep"
	ActionCandidate = "candidate" // dry run: would be removed by an apply run
	ActionRemoved   = "removed"
	ActionFailed    = "remove-failed"
)

// Entry is one row of the report.
type Entry struct {
	Path    string // absolute path of the session directory (or of the kept entry)
	Project string
	Session string
	State   string // live | ended | unknown
	Held    bool   // the directory holds Git data: it goes through deskwt, never a plain delete
	Action  string
	Reason  string
}

// walkLimit bounds the Git-data walk of one session directory. A directory with more
// entries is kept as could-not-check rather than judged on a prefix.
var walkLimit = 262144

var sessionID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Sessions lists ONE desk's session directories, <root>/<project>/<session id>, and
// classifies each. root is the harness temp root (the `claude-<uid>` directory) and project
// is the desk's own project slug. It never walks other projects under root: each desk
// cleans up its own sessions, so the pass is correct where the desk is the only thing on
// the filesystem, and a host carrying several desks calls it once per desk.
//
// With apply false nothing is removed; with apply true each candidate is checked again
// (liveness and Git data) right before it is removed through a root-confined handle, so a
// symlink cannot redirect the removal outside root. The returned error reports a project
// directory that could not be listed or a removal that failed; rows are returned either way.
// An absent project directory is an empty report, not an error.
func Sessions(root, project string, live Liveness, apply bool) ([]Entry, error) {
	if live == nil {
		return nil, errors.New("harness scratch: a liveness record reader is required")
	}
	if project == "" || project == "." || project == ".." || strings.ContainsAny(project, `/\`) {
		return nil, fmt.Errorf("harness scratch: %q is not one project slug", project)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("harness scratch: open %s: %w", root, err)
	}
	defer r.Close()
	fi, err := r.Lstat(project)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("harness scratch: inspect %s: %w", project, err)
	}
	if !fi.IsDir() {
		return []Entry{{Path: filepath.Join(root, project), Project: project, State: Unknown.String(), Action: ActionKeep, Reason: "not a project directory (links are never followed)"}}, nil
	}
	sessions, err := fs.ReadDir(r.FS(), project)
	if err != nil {
		return nil, fmt.Errorf("harness scratch: list %s: %w", project, err)
	}
	var out []Entry
	var errs []error
	for _, s := range sessions {
		e := classify(r, root, project, s, live)
		if apply && e.Action == ActionCandidate {
			if rerr := remove(r, project+"/"+s.Name(), live, &e); rerr != nil {
				errs = append(errs, rerr)
			}
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, errors.Join(errs...)
}

func classify(r *os.Root, root, project string, d fs.DirEntry, live Liveness) Entry {
	e := Entry{Path: filepath.Join(root, project, d.Name()), Project: project, Session: d.Name(), State: Unknown.String(), Action: ActionKeep}
	if !d.IsDir() {
		e.Reason = "not a session directory (links are never followed)"
		return e
	}
	if !sessionID.MatchString(d.Name()) {
		e.Reason = "name is not a harness session id"
		return e
	}
	st, why := live(d.Name())
	e.State, e.Reason = st.String(), why
	if st != Ended {
		return e
	}
	held, why, ok := gitData(r, project+"/"+d.Name())
	switch {
	case !ok:
		e.Reason += "; could-not-check Git data: " + why
	case held:
		e.Held = true
		e.Reason += "; held: " + why + " (route it through deskwt, never a plain delete)"
	default:
		e.Action = ActionCandidate
	}
	return e
}

// gitData walks one session directory without following links and reports whether it
// holds Git data: a `.git` file (a linked worktree registered with some repository) or a
// `.git` directory (a repository). ok is false when the walk failed or hit its bound.
func gitData(r *os.Root, rel string) (held bool, why string, ok bool) {
	n := 0
	err := fs.WalkDir(r.FS(), rel, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if n++; n > walkLimit {
			return fmt.Errorf("over the %d-entry walk limit", walkLimit)
		}
		if d.Name() != ".git" {
			return nil
		}
		at := strings.TrimPrefix(path.Dir(p), rel+"/")
		if d.Type().IsRegular() {
			why = "holds a registered git worktree at " + at
		} else {
			why = "holds a git repository at " + at
		}
		held = true
		return fs.SkipAll
	})
	if err != nil {
		return false, err.Error(), false
	}
	return held, why, true
}

func remove(r *os.Root, rel string, live Liveness, e *Entry) error {
	// Decide again at the moment of removal: a session can come back, and Git data can
	// appear, between planning and deleting.
	if st, why := live(e.Session); st != Ended {
		e.State, e.Action, e.Reason = st.String(), ActionKeep, why+" (re-read before removal)"
		return nil
	}
	if held, why, ok := gitData(r, rel); !ok || held {
		e.Action, e.Held = ActionKeep, held
		e.Reason += "; re-check before removal kept it: " + why
		return nil
	}
	if fi, err := r.Lstat(rel); err != nil || !fi.IsDir() {
		e.Action = ActionKeep
		e.Reason += "; no longer a real directory at removal"
		return nil
	}
	if err := r.RemoveAll(rel); err != nil {
		e.Action, e.Reason = ActionFailed, e.Reason+"; "+err.Error()
		return fmt.Errorf("harness scratch: remove %s: %w", e.Path, err)
	}
	e.Action = ActionRemoved
	return nil
}

// BeaconLiveness reads roster beacons (<rosterDir>/<session>.json, field `updated`). A
// beacon re-stamped inside window is live; one past it is ended. A missing beacon is
// UNKNOWN, not ended: beacons are keyed by the desk session name, which differs from the
// harness session id whenever DESK_SESSION is set, so absence proves nothing here.
func BeaconLiveness(rosterDir string, window time.Duration, now time.Time) Liveness {
	return func(session string) (State, string) {
		if !sessionID.MatchString(session) {
			return Unknown, "name is not a harness session id"
		}
		if rosterDir == "" {
			return Unknown, "could-not-check: no roster to read"
		}
		raw, err := os.ReadFile(filepath.Join(rosterDir, session+".json"))
		if errors.Is(err, fs.ErrNotExist) {
			return Unknown, "no roster beacon names this session; absence proves nothing"
		}
		if err != nil {
			return Unknown, "could-not-check: " + err.Error()
		}
		var b struct {
			Updated string `json:"updated"`
		}
		if json.Unmarshal(raw, &b) != nil {
			return Unknown, "could-not-check: roster beacon is not valid JSON"
		}
		at, err := time.Parse(time.RFC3339, strings.TrimSpace(b.Updated))
		if err != nil {
			return Unknown, "could-not-check: roster beacon has no readable update time"
		}
		age := now.Sub(at).Round(time.Second)
		if age > window {
			return Ended, fmt.Sprintf("roster beacon last updated %s ago, past the %s freshness window", age, window)
		}
		return Live, fmt.Sprintf("roster beacon updated %s ago", age)
	}
}
