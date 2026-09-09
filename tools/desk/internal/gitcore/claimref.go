package gitcore

// claimref.go — the reusable go-git pieces the dispatch-claim ref transport is built from,
// next to List so a caller (cmd/deskclaim-ref) can mint a claim tag, place/advance/steal it
// with a SERVER-SIDE compare-and-swap, delete it, and read its payload — all in-process over
// git-smart-HTTP, with no forge CLI and no external git binary.
//
// WHY THESE THREE PRIMITIVES AND NOT Repo.Push. The high-level go-git Push cannot express
// "create this ref ONLY if it does not already exist, else fail" — RequireRemoteRefs=ZeroHash
// is rejected CLIENT-SIDE by go-git when the ref is absent, and a force push would clobber a
// live claim. A dispatch claim is a distributed lock: the create MUST be a server-side
// compare-and-swap (old=zero), and an advance/steal MUST be a compare-and-swap from the value
// the caller last read, so a claim stolen or advanced underneath the caller is rejected by the
// SERVER, not papered over. Only the plumbing receive-pack session carries an explicit `old`
// oid per command, so these functions drop to it. (Proven in a spike on github.com and
// gitlab.com, including the server-side CAS rejections, 7/7 green each.)

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/sideband"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/storage/memory"
)

// EmptyBlobHash is git's canonical empty-blob object id (the sha of a zero-length blob). A
// Go-minted claim tag targets it so a claim can be created with NO pre-fetch of the repo's
// default branch — the empty blob is pushed inline in the tag's packfile. This retires the
// script's hardcoded `refs/heads/main` target assumption (a GitLab wart: a repo whose default
// branch is not `main` had no `heads/main` to hang a tag on).
const EmptyBlobHash = "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"

// claimTagName is the tag NAME every dispatch-claim tag object carries. It matches the ref's
// own leaf (refs/dispatch/<id> -> tag name "dispatch/<id>") but is otherwise cosmetic: readers
// key on the message body, never the tag name.
func claimTagName(id string) string { return "dispatch/" + id }

// claimTaggerName / claimTaggerEmail are the FIXED tagger identity a Go-minted claim tag
// carries. Only tagger.When is ever read back (for the claim's age); the holder identity lives
// in the message body (owner=…), so these are a stable constant, never a real person and never
// a credential-derived value.
const (
	claimTaggerName  = "assay dispatch-claim"
	claimTaggerEmail = "dispatch-claim@assay.invalid"
)

// MintClaimTag builds an annotated tag object carrying message, stamped at when, targeting the
// empty blob — both objects held in a fresh in-memory store. It returns the store (the source
// of the objects a subsequent PushRefUpdate packs) and the tag's hash. The message body is the
// wire contract shared with the bash script (`dispatch-claim <id> owner=… state=… branch=…`),
// built by the caller and passed through verbatim.
func MintClaimTag(id, message string, when time.Time) (*memory.Storage, plumbing.Hash, error) {
	store := memory.NewStorage()

	blob := store.NewEncodedObject()
	blob.SetType(plumbing.BlobObject)
	blob.SetSize(0)
	// A zero-length blob: open and immediately close the writer so the object is a real,
	// empty blob (its hash is EmptyBlobHash).
	w, err := blob.Writer()
	if err != nil {
		return nil, plumbing.ZeroHash, fmt.Errorf("gitcore: mint claim tag: empty-blob writer: %w", err)
	}
	if cerr := w.Close(); cerr != nil {
		return nil, plumbing.ZeroHash, fmt.Errorf("gitcore: mint claim tag: empty-blob close: %w", cerr)
	}
	blobHash, err := store.SetEncodedObject(blob)
	if err != nil {
		return nil, plumbing.ZeroHash, fmt.Errorf("gitcore: mint claim tag: store empty blob: %w", err)
	}

	tag := &object.Tag{
		Name:       claimTagName(id),
		Tagger:     object.Signature{Name: claimTaggerName, Email: claimTaggerEmail, When: when.UTC()},
		Message:    message,
		TargetType: plumbing.BlobObject,
		Target:     blobHash,
	}
	obj := store.NewEncodedObject()
	if err := tag.Encode(obj); err != nil {
		return nil, plumbing.ZeroHash, fmt.Errorf("gitcore: mint claim tag: encode: %w", err)
	}
	tagHash, err := store.SetEncodedObject(obj)
	if err != nil {
		return nil, plumbing.ZeroHash, fmt.Errorf("gitcore: mint claim tag: store tag: %w", err)
	}
	return store, tagHash, nil
}

// RefUpdateResult is the OUTCOME of a compare-and-swap ref update that the SERVER accepted or
// refused. It is deliberately distinct from a transport error (returned separately): a refusal
// is a normal, expected answer ("the claim already exists" / "the old value no longer matches",
// i.e. a race the caller must act on), whereas a transport/auth/not-found error is
// could-not-check and fails closed.
type RefUpdateResult int

const (
	// RefUpdateApplied — the server accepted the update: the ref now holds New.
	RefUpdateApplied RefUpdateResult = iota
	// RefUpdateRejected — the server refused the update because the ref's current value did
	// not equal Old (a create whose ref already exists, or an advance/steal from a value that
	// has since changed). This is the compare-and-swap losing, NOT an error.
	RefUpdateRejected
)

// RefUpdate is one server-side compare-and-swap ref update. Old is the value the ref MUST
// currently hold for the update to apply (plumbing.ZeroHash means "must not exist" — a create);
// New is the value to set (plumbing.ZeroHash means "delete"). Objects supplies the objects New
// references (nil for a delete). URL and Auth are scoped to this one call, exactly as the other
// gitcore transport verbs: no remote alias, no insteadOf, no credential helper.
type RefUpdate struct {
	URL     string
	Auth    transport.AuthMethod
	Ref     plumbing.ReferenceName
	Old     plumbing.Hash
	New     plumbing.Hash
	Objects *memory.Storage
}

// PushRefUpdate performs the compare-and-swap ref update over a plumbing receive-pack session.
// It returns RefUpdateApplied/RefUpdateRejected with a nil error when the SERVER gave a verdict;
// a non-nil error is a transport/auth/not-found/protocol failure the caller reads as
// could-not-check (never as "free" or "applied").
func PushRefUpdate(ctx context.Context, u RefUpdate) (RefUpdateResult, error) {
	ep, err := transport.NewEndpoint(u.URL)
	if err != nil {
		return 0, fmt.Errorf("gitcore: ref-update endpoint: %w", err)
	}
	cli, err := client.NewClient(ep)
	if err != nil {
		return 0, fmt.Errorf("gitcore: ref-update client: %w", err)
	}
	sess, err := cli.NewReceivePackSession(ep, u.Auth)
	if err != nil {
		return 0, fmt.Errorf("gitcore: receive-pack session: %w", err)
	}
	defer sess.Close()

	adv, err := sess.AdvertisedReferencesContext(ctx)
	if err != nil {
		return 0, fmt.Errorf("gitcore: receive-pack advertise: %w", err)
	}

	req := packp.NewReferenceUpdateRequestFromCapabilities(adv.Capabilities)
	req.Commands = []*packp.Command{{Name: u.Ref, Old: u.Old, New: u.New}}
	if u.New != plumbing.ZeroHash {
		if u.Objects == nil {
			return 0, fmt.Errorf("gitcore: receive-pack: a non-delete update carries no object source")
		}
		var buf bytes.Buffer
		enc := packfile.NewEncoder(&buf, u.Objects, false)
		// Encode from the new tip AND the empty blob it targets so the pack is self-contained
		// even against a server that has never seen an empty-blob loose object.
		if _, eerr := enc.Encode([]plumbing.Hash{u.New, plumbing.NewHash(EmptyBlobHash)}, 10); eerr != nil {
			return 0, fmt.Errorf("gitcore: receive-pack pack encode: %w", eerr)
		}
		req.Packfile = io.NopCloser(&buf)
	}

	rs, err := sess.ReceivePack(ctx, req)
	// A report-status was decoded: the server gave a verdict. Prefer it over the aggregate
	// error ReceivePack also returns for a non-ok report, so a per-command "ng" (the CAS
	// losing) is classified as a rejection rather than an opaque error.
	if rs != nil {
		if rs.UnpackStatus != "" && rs.UnpackStatus != "ok" {
			return 0, fmt.Errorf("gitcore: receive-pack unpack error: %s", rs.UnpackStatus)
		}
		for _, cs := range rs.CommandStatuses {
			if cs.Status != "ok" {
				return RefUpdateRejected, nil
			}
		}
		return RefUpdateApplied, nil
	}
	if err != nil {
		return 0, fmt.Errorf("gitcore: receive-pack: %w", err)
	}
	// No report-status and no error: report-status was not negotiated, but the command was
	// sent and the session closed cleanly. Treat as applied (the local git transport takes
	// this path); the http transport always returns a report-status.
	return RefUpdateApplied, nil
}

// DeleteResult is the outcome of a ref delete.
type DeleteResult int

const (
	// DeleteDone — the ref existed and was deleted.
	DeleteDone DeleteResult = iota
	// DeleteAbsent — the ref did not exist (a release of an already-released claim: a no-op,
	// never an error).
	DeleteAbsent
)

// DeleteRef deletes Ref, reading its current value from the SAME receive-pack advertisement it
// then CAS-deletes against — so a delete "of whatever is there" is a single atomic session, and
// an absent ref is reported as DeleteAbsent rather than mis-read as an error. This is the
// release path's "delete if present, no-op if absent" without a separate read round trip.
func DeleteRef(ctx context.Context, url string, auth transport.AuthMethod, ref plumbing.ReferenceName) (DeleteResult, error) {
	ep, err := transport.NewEndpoint(url)
	if err != nil {
		return 0, fmt.Errorf("gitcore: delete-ref endpoint: %w", err)
	}
	cli, err := client.NewClient(ep)
	if err != nil {
		return 0, fmt.Errorf("gitcore: delete-ref client: %w", err)
	}
	sess, err := cli.NewReceivePackSession(ep, auth)
	if err != nil {
		return 0, fmt.Errorf("gitcore: receive-pack session: %w", err)
	}
	defer sess.Close()

	adv, err := sess.AdvertisedReferencesContext(ctx)
	if err != nil {
		return 0, fmt.Errorf("gitcore: receive-pack advertise: %w", err)
	}
	current, ok := adv.References[ref.String()]
	if !ok || current.IsZero() {
		return DeleteAbsent, nil
	}

	req := packp.NewReferenceUpdateRequestFromCapabilities(adv.Capabilities)
	req.Commands = []*packp.Command{{Name: ref, Old: current, New: plumbing.ZeroHash}}
	rs, err := sess.ReceivePack(ctx, req)
	if rs != nil {
		if rs.UnpackStatus != "" && rs.UnpackStatus != "ok" {
			return 0, fmt.Errorf("gitcore: receive-pack unpack error: %s", rs.UnpackStatus)
		}
		for _, cs := range rs.CommandStatuses {
			if cs.Status != "ok" {
				// The ref changed between advertise and delete — report as a transport-level
				// could-not-check; the caller re-reads rather than blindly assuming released.
				return 0, fmt.Errorf("gitcore: delete of %s rejected: %s", ref, cs.Status)
			}
		}
		return DeleteDone, nil
	}
	if err != nil {
		return 0, fmt.Errorf("gitcore: receive-pack: %w", err)
	}
	return DeleteDone, nil
}

// TagPayload is the message body and tagger timestamp read off a claim tag object.
type TagPayload struct {
	Message string
	When    time.Time
}

// FetchTagPayload reads the annotated tag object at Tag via a FILTERED upload-pack fetch
// (tree:0 + shallow depth 1), so the fetched pack is a handful of bytes regardless of whether
// the tag targets the empty blob (a Go-minted claim) or a commit on the default branch (a
// bash/REST-minted claim). Only the tag object itself is needed — its message and tagger date —
// so trees and blobs are filtered out and history is cut to one commit. Returns the decoded
// message and tagger time.
func FetchTagPayload(ctx context.Context, url string, auth transport.AuthMethod, tag plumbing.Hash) (TagPayload, error) {
	ep, err := transport.NewEndpoint(url)
	if err != nil {
		return TagPayload{}, fmt.Errorf("gitcore: tag-payload endpoint: %w", err)
	}
	cli, err := client.NewClient(ep)
	if err != nil {
		return TagPayload{}, fmt.Errorf("gitcore: tag-payload client: %w", err)
	}
	sess, err := cli.NewUploadPackSession(ep, auth)
	if err != nil {
		return TagPayload{}, fmt.Errorf("gitcore: upload-pack session: %w", err)
	}
	defer sess.Close()

	adv, err := sess.AdvertisedReferencesContext(ctx)
	if err != nil {
		return TagPayload{}, fmt.Errorf("gitcore: upload-pack advertise: %w", err)
	}

	req := packp.NewUploadPackRequestFromCapabilities(adv.Capabilities)
	req.Wants = []plumbing.Hash{tag}
	if adv.Capabilities.Supports(capability.Filter) {
		_ = req.Capabilities.Add(capability.Filter)
		req.Filter = packp.FilterTreeDepth(0)
	}
	if adv.Capabilities.Supports(capability.Shallow) {
		// A non-zero DepthCommits requires the shallow capability to be declared on the
		// request (UploadRequest.Validate) — the server advertised it, so declare it.
		_ = req.Capabilities.Add(capability.Shallow)
		req.Depth = packp.DepthCommits(1)
	}

	resp, err := sess.UploadPack(ctx, req)
	if err != nil {
		return TagPayload{}, fmt.Errorf("gitcore: upload-pack: %w", err)
	}
	defer resp.Close()

	store := memory.NewStorage()
	// When sideband was negotiated the packfile arrives multiplexed with progress; demux it
	// (mirroring go-git's own remote.fetchPack) before handing the raw pack to the parser.
	if err := packfile.UpdateObjectStorage(store, demuxPack(req.Capabilities, resp)); err != nil {
		return TagPayload{}, fmt.Errorf("gitcore: upload-pack decode: %w", err)
	}
	t, err := object.GetTag(store, tag)
	if err != nil {
		return TagPayload{}, fmt.Errorf("gitcore: read tag object %s: %w", tag, err)
	}
	return TagPayload{Message: t.Message, When: t.Tagger.When}, nil
}

// demuxPack unwraps the upload-pack response's packfile from its sideband framing when a
// sideband capability was negotiated, mirroring go-git's own remote.buildSidebandIfSupported.
func demuxPack(caps *capability.List, reader io.Reader) io.Reader {
	switch {
	case caps.Supports(capability.Sideband):
		return sideband.NewDemuxer(sideband.Sideband, reader)
	case caps.Supports(capability.Sideband64k):
		return sideband.NewDemuxer(sideband.Sideband64k, reader)
	default:
		return reader
	}
}
