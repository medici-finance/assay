package gitcore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/sideband"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// RefReceipt contains only request-local protocol metadata. It never contains a
// URL, credential, claim payload, response body, or arbitrary response headers.
// RequestID is an untrusted, bounded correlation header; callers must scrub it
// using their normal diagnostic scrubber before publication.
type RefReceipt struct {
	Ref, Old, New, Advertised    string
	PackSHA256                   string
	PackBytes, PackObjects       int
	Capabilities, Phase, Verdict string
	HTTPStatus                   int
	RequestID                    string
}

func (r *RefReceipt) pack(b []byte) {
	r.PackSHA256 = fmt.Sprintf("%x", sha256.Sum256(b))
	r.PackBytes = len(b)
	if len(b) >= 12 {
		r.PackObjects = int(binary.BigEndian.Uint32(b[8:12]))
	}
}

func (r *RefReceipt) finish(v RefUpdateVerdict, err error, trace func(RefReceipt)) {
	r.Verdict = "applied"
	if err != nil {
		r.Verdict = "unverifiable"
	}
	if v.Result == RefUpdateRejected {
		r.Verdict = "rejected"
	}
	if r.Phase == "absent" && err == nil {
		r.Verdict = "absent"
	}
	if trace != nil {
		trace(*r)
	}
}

const maxWireAck = 1024 * 1024
const maxAckData = 64 * 1024

// ackBody preserves reads/errors for go-git while retaining a bounded private
// copy. Overflow is not a truncated success: it makes the verdict unverifiable.
type ackBody struct {
	io.ReadCloser
	data     bytes.Buffer
	overflow bool
}

func (b *ackBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	keep := n
	if keep > maxWireAck-b.data.Len() {
		keep = maxWireAck - b.data.Len()
		b.overflow = true
	}
	b.data.Write(p[:keep])
	return n, err
}

type ackTransport struct {
	base    http.RoundTripper
	receipt *RefReceipt
	body    *ackBody
}

func (t *ackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := t.base.RoundTrip(req)
	if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/git-receive-pack") && res != nil {
		t.receipt.HTTPStatus = res.StatusCode
		t.receipt.RequestID = boundedID(res.Header.Get("X-GitHub-Request-Id"))
		if t.receipt.RequestID == "" {
			t.receipt.RequestID = boundedID(res.Header.Get("X-Request-Id"))
		}
		t.body = &ackBody{ReadCloser: res.Body}
		res.Body = t.body
	}
	return res, err
}

func boundedID(s string) string {
	if len(s) > 128 {
		return "invalid"
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune(":-_.", c)) {
			return "invalid"
		}
	}
	return s
}

func claimClient(ep *transport.Endpoint, r *RefReceipt) (transport.Transport, *ackTransport, error) {
	if ep.Protocol != "http" && ep.Protocol != "https" {
		c, err := client.NewClient(ep)
		return c, nil, err
	}
	// NewEndpoint(URL) has no custom TLS/proxy options. Fail explicitly if a
	// future caller supplies these: go-git's custom-option path requires a concrete
	// *http.Transport and must not silently bypass this observer.
	if ep.InsecureSkipTLS || len(ep.CaBundle) != 0 || len(ep.ClientCert) != 0 || len(ep.ClientKey) != 0 || ep.Proxy.URL != "" {
		return nil, nil, fmt.Errorf("gitcore: unsupported custom claim HTTP transport options")
	}
	observer := &ackTransport{base: http.DefaultTransport, receipt: r}
	// A private client preserves go-git's default initial-only redirect policy,
	// auth application and session behavior; global protocol registration is untouched.
	return githttp.NewClient(&http.Client{Transport: observer}), observer, nil
}

// receiveClaim is the only claim receive-pack sink. All transports need an
// exact decoded report. HTTP additionally retains the wire marker because
// go-git's decoder collapses `ng <ref> ok` into Status="ok". Native Git's raw
// marker is not exposed by that transport; its decoded report remains the limit.
func receiveClaim(ctx context.Context, sess transport.ReceivePackSession, req *packp.ReferenceUpdateRequest, remote *bytes.Buffer, observer *ackTransport, receipt *RefReceipt) (RefUpdateVerdict, error) {
	if !req.Capabilities.Supports(capability.ReportStatus) || len(req.Commands) != 1 || req.Commands[0] == nil {
		return RefUpdateVerdict{}, fmt.Errorf("gitcore: receive-pack requires one command and report-status")
	}
	var caps []string
	for _, c := range []capability.Capability{capability.ReportStatus, capability.Sideband64k, capability.Sideband, capability.Quiet} {
		if req.Capabilities.Supports(c) {
			caps = append(caps, string(c))
		}
	}
	receipt.Capabilities = strings.Join(caps, ",")
	receipt.Phase = "receive"
	rs, err := sess.ReceivePack(ctx, req)
	if observer != nil && observer.body != nil {
		// The decoder may stop at the inner flush. Finish the response within a
		// fixed byte budget so framing after it cannot disappear from validation.
		_, drainErr := io.Copy(io.Discard, io.LimitReader(observer.body, maxWireAck+1))
		closeErr := observer.body.Close()
		if drainErr != nil || closeErr != nil {
			// Still unverifiable, but the dependency's own error (a non-2xx
			// status closes the body before returning) names the cause.
			if err != nil {
				return RefUpdateVerdict{}, fmt.Errorf("gitcore: receive-pack: %w", withRemote(err, remote))
			}
			return RefUpdateVerdict{}, fmt.Errorf("gitcore: receive-pack response read failed")
		}
	}
	receipt.Phase = "acknowledge"
	if rs == nil || rs.UnpackStatus != "ok" || len(rs.CommandStatuses) != 1 || rs.CommandStatuses[0] == nil || rs.CommandStatuses[0].ReferenceName != req.Commands[0].Name || rs.CommandStatuses[0].Status == "" {
		if err != nil {
			return RefUpdateVerdict{}, fmt.Errorf("gitcore: receive-pack: %w", withRemote(err, remote))
		}
		return RefUpdateVerdict{}, fmt.Errorf("gitcore: receive-pack missing or mismatched acknowledgment")
	}
	status := rs.CommandStatuses[0].Status
	rejected := status != "ok"
	if observer != nil {
		marker, reason, wireErr := wireAck(observer.body, req)
		if wireErr != nil {
			return RefUpdateVerdict{}, wireErr
		}
		rejected = marker == "ng"
		if reason != status {
			return RefUpdateVerdict{}, fmt.Errorf("gitcore: receive-pack inconsistent acknowledgment")
		}
	}
	if rejected {
		return RefUpdateVerdict{Result: RefUpdateRejected, Status: strings.TrimSpace(status), Remote: remoteText(remote)}, nil
	}
	if err != nil {
		return RefUpdateVerdict{}, fmt.Errorf("gitcore: receive-pack: %w", withRemote(err, remote))
	}
	return RefUpdateVerdict{Result: RefUpdateApplied}, nil
}

func wireAck(body *ackBody, req *packp.ReferenceUpdateRequest) (string, string, error) {
	bad := func() (string, string, error) {
		return "", "", fmt.Errorf("gitcore: receive-pack invalid or incomplete wire acknowledgment")
	}
	if body == nil || body.overflow || !bytes.HasSuffix(body.data.Bytes(), []byte("0000")) {
		return bad()
	}
	var reader io.Reader = bytes.NewReader(body.data.Bytes())
	if req.Capabilities.Supports(capability.Sideband64k) || req.Capabilities.Supports(capability.Sideband) {
		outer := pktline.NewScanner(bytes.NewReader(body.data.Bytes()))
		flushed := false
		for outer.Scan() {
			if len(outer.Bytes()) == 0 {
				flushed = true
				break
			}
		}
		if !flushed || outer.Scan() || outer.Err() != nil {
			return bad()
		}
	}
	switch {
	case req.Capabilities.Supports(capability.Sideband64k):
		reader = sideband.NewDemuxer(sideband.Sideband64k, reader)
	case req.Capabilities.Supports(capability.Sideband):
		reader = sideband.NewDemuxer(sideband.Sideband, reader)
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxAckData+1))
	if err != nil || len(data) > maxAckData || !bytes.HasSuffix(data, []byte("0000")) {
		return bad()
	}
	scanner := pktline.NewScanner(bytes.NewReader(data))
	if !scanner.Scan() || string(scanner.Bytes()) != "unpack ok\n" {
		return bad()
	}
	if !scanner.Scan() {
		return bad()
	}
	line := strings.TrimSuffix(string(scanner.Bytes()), "\n")
	marker, rest, ok := strings.Cut(line, " ")
	if !ok {
		return bad()
	}
	ref, reason := rest, "ok"
	if marker == "ng" {
		ref, reason, ok = strings.Cut(rest, " ")
		if !ok || strings.TrimSpace(reason) == "" {
			return bad()
		}
	} else if marker != "ok" {
		return bad()
	}
	if ref != req.Commands[0].Name.String() {
		return bad()
	}
	if !scanner.Scan() || len(scanner.Bytes()) != 0 || scanner.Scan() || scanner.Err() != nil {
		return bad()
	}
	return marker, reason, nil
}
