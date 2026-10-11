package forgeread

import "context"

// OfflineReason is the reason an OfflineReader attaches to every repository.
const OfflineReason = "forge reads are offline — no forge read was attempted"

// OfflineReader answers could-not-check for every repository and touches nothing: no process, no
// network, no file. It is the safe default reader.
type OfflineReader struct {
	// Why overrides OfflineReason in the unavailable entries.
	Why string
}

// OpenIssues reports every requested repository as unavailable (ClassOffline) and returns an
// empty Issues map — never a map of empty slices, so an unread repository carries no answer.
func (o OfflineReader) OpenIssues(_ context.Context, repos []string) (Result, error) {
	why := o.Why
	if why == "" {
		why = OfflineReason
	}
	out := NewResult()
	for _, r := range Normalize(repos) {
		out.Unavailable = append(out.Unavailable, Unavailable{Repo: r, Reason: why, Class: ClassOffline})
	}
	return out, nil
}

// FrozenReader answers from a fixed, already-obtained Result — a recorded fixture or a snapshot
// taken earlier. It never reads anything, and it reconciles against the request on every call, so
// a repository the snapshot does not cover is unavailable (ClassMissing), not empty.
type FrozenReader struct {
	frozen Result
}

// NewFrozenReader copies r so later mutation by the caller cannot change the snapshot.
func NewFrozenReader(r Result) FrozenReader {
	cp := NewResult()
	for k, v := range r.Issues {
		cp.Issues[k] = append([]Issue{}, v...)
	}
	cp.Unavailable = append([]Unavailable(nil), r.Unavailable...)
	return FrozenReader{frozen: cp}
}

// OpenIssues reconciles the snapshot against the requested set.
func (f FrozenReader) OpenIssues(_ context.Context, repos []string) (Result, error) {
	return Reconcile(repos, f.frozen, "the frozen result does not cover this repository"), nil
}

var (
	_ Reader = OfflineReader{}
	_ Reader = FrozenReader{}
)
