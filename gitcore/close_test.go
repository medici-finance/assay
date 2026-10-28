package gitcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

var closeCanary = errors.New("https://example.invalid/cleanup synthetic-auth payload-canary")
var primaryCloseTest = errors.New("primary transport failure")

type closeReceive struct {
	adv                                *packp.AdvRefs
	advertiseErr, receiveErr, closeErr error
	status                             string
	closes, calls                      int
	events                             *[]string
}

func (s *closeReceive) AdvertisedReferences() (*packp.AdvRefs, error) { return s.adv, s.advertiseErr }
func (s *closeReceive) AdvertisedReferencesContext(context.Context) (*packp.AdvRefs, error) {
	return s.AdvertisedReferences()
}
func (s *closeReceive) Close() error {
	s.closes++
	*s.events = append(*s.events, "session-close")
	return s.closeErr
}
func (s *closeReceive) ReceivePack(_ context.Context, req *packp.ReferenceUpdateRequest) (*packp.ReportStatus, error) {
	s.calls++
	if s.receiveErr != nil {
		return nil, s.receiveErr
	}
	if req.Packfile != nil {
		if _, err := io.Copy(io.Discard, req.Packfile); err != nil {
			return nil, err
		}
	}
	if req.Progress != nil {
		if _, err := io.WriteString(req.Progress, "remote refusal detail"); err != nil {
			return nil, err
		}
	}
	return &packp.ReportStatus{UnpackStatus: "ok", CommandStatuses: []*packp.CommandStatus{{ReferenceName: req.Commands[0].Name, Status: s.status}}}, nil
}

type closeClient struct {
	receive transport.ReceivePackSession
	upload  transport.UploadPackSession
}

func (c closeClient) NewReceivePackSession(*transport.Endpoint, transport.AuthMethod) (transport.ReceivePackSession, error) {
	return c.receive, nil
}
func (c closeClient) NewUploadPackSession(*transport.Endpoint, transport.AuthMethod) (transport.UploadPackSession, error) {
	return c.upload, nil
}
func closeAdv(t *testing.T, ref plumbing.ReferenceName, present bool) *packp.AdvRefs {
	t.Helper()
	adv := packp.NewAdvRefs()
	if err := adv.Capabilities.Set(capability.ReportStatus); err != nil {
		t.Fatal(err)
	}
	if err := adv.Capabilities.Set(capability.Sideband64k); err != nil {
		t.Fatal(err)
	}
	if present {
		adv.References[ref.String()] = plumbing.NewHash(strings.Repeat("1", 40))
	}
	return adv
}

// regression: #2395
func TestCloseKeepsCAS(t *testing.T) {
	ref := plumbing.ReferenceName("refs/dispatch/close-proof")
	objs, oid, err := MintClaimTag("close-proof", "tag body", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"push", "delete"} {
		for _, outcome := range []string{"applied", "rejected", "absent", "advertise-error", "receive-error"} {
			if op == "push" && outcome == "absent" {
				continue
			}
			t.Run(op+"/"+outcome, func(t *testing.T) {
				var events []string
				sess := &closeReceive{adv: closeAdv(t, ref, outcome != "absent"), closeErr: closeCanary, status: "ok", events: &events}
				if outcome == "rejected" {
					sess.status = "policy refused"
				}
				if outcome == "advertise-error" {
					sess.advertiseErr = primaryCloseTest
				}
				if outcome == "receive-error" {
					sess.receiveErr = primaryCloseTest
				}
				factory := func(*transport.Endpoint, *RefReceipt) (transport.Transport, *ackTransport, error) {
					return closeClient{receive: sess}, nil, nil
				}
				traces := 0
				var receipt RefReceipt
				trace := func(r RefReceipt) { traces++; receipt = r; events = append(events, "trace") }
				if op == "push" {
					v, err := pushRefUpdateVerdict(context.Background(), RefUpdate{URL: "fixture://example.invalid/repo", Ref: ref, New: oid, Objects: objs, Trace: trace}, factory)
					if strings.HasSuffix(outcome, "error") {
						if !errors.Is(err, primaryCloseTest) {
							t.Fatalf("primary error changed: %v", err)
						}
					} else if err != nil {
						t.Fatal(err)
					}
					if outcome == "applied" && (v.Result != RefUpdateApplied || receipt.Verdict != "applied") {
						t.Fatalf("acknowledged CAS changed: %+v %+v", v, receipt)
					}
					if outcome == "rejected" && (v.Result != RefUpdateRejected || v.Status != "policy refused" || v.Remote != "remote refusal detail" || receipt.Verdict != "rejected") {
						t.Fatalf("refusal changed: %+v %+v", v, receipt)
					}
				} else {
					v, err := deleteRefTrace(context.Background(), "fixture://example.invalid/repo", nil, ref, trace, factory)
					switch outcome {
					case "applied":
						if err != nil || v != DeleteDone || receipt.Verdict != "applied" {
							t.Fatalf("delete changed: %v %v %+v", v, err, receipt)
						}
					case "absent":
						if err != nil || v != DeleteAbsent || receipt.Verdict != "absent" {
							t.Fatalf("absence changed: %v %v %+v", v, err, receipt)
						}
					case "rejected":
						r, ok := err.(*RefRejectedError)
						if !ok || r.Ref != ref || r.Old != sess.adv.References[ref.String()] || r.Status != "policy refused" || r.Remote != "remote refusal detail" || receipt.Verdict != "rejected" {
							t.Fatalf("direct rejected type/fields changed: %T %v %+v", err, err, receipt)
						}
					default:
						if !errors.Is(err, primaryCloseTest) {
							t.Fatalf("primary delete error changed: %v", err)
						}
					}
				}
				if sess.closes != 1 || traces != 1 || !receipt.SessionCloseFailed || !reflect.DeepEqual(events, []string{"session-close", "trace"}) {
					t.Fatalf("close/receipt ordering changed: closes=%d traces=%d %+v %v", sess.closes, traces, receipt, events)
				}
				wantPhase := "acknowledge"
				if outcome == "absent" {
					wantPhase = "absent"
				}
				if outcome == "advertise-error" {
					wantPhase = "advertise"
				}
				if receipt.Phase != wantPhase {
					t.Fatalf("primary protocol phase changed: %s want %s", receipt.Phase, wantPhase)
				}
				wantCalls := 1
				if outcome == "absent" || outcome == "advertise-error" {
					wantCalls = 0
				}
				if sess.calls != wantCalls {
					t.Fatalf("unexpected retry/receive: %d want %d", sess.calls, wantCalls)
				}
				encoded, err := json.Marshal(receipt)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(encoded), "payload-canary") || strings.Contains(string(encoded), "synthetic-auth") {
					t.Fatalf("close error material leaked: %s", encoded)
				}
			})
		}
	}
}

type closeRead struct {
	io.Reader
	err    error
	calls  int
	events *[]string
}

func (r *closeRead) Close() error {
	r.calls++
	*r.events = append(*r.events, "response-close")
	return r.err
}

type closeUpload struct {
	adv                                            *packp.AdvRefs
	data                                           []byte
	advertiseErr, uploadErr, closeErr, responseErr error
	events                                         *[]string
	closes, calls                                  int
	body                                           *closeRead
}

func (s *closeUpload) AdvertisedReferences() (*packp.AdvRefs, error) { return s.adv, s.advertiseErr }
func (s *closeUpload) AdvertisedReferencesContext(context.Context) (*packp.AdvRefs, error) {
	return s.AdvertisedReferences()
}
func (s *closeUpload) Close() error {
	s.closes++
	*s.events = append(*s.events, "session-close")
	return s.closeErr
}
func (s *closeUpload) UploadPack(_ context.Context, req *packp.UploadPackRequest) (*packp.UploadPackResponse, error) {
	s.calls++
	if s.uploadErr != nil {
		return nil, s.uploadErr
	}
	s.body = &closeRead{Reader: bytes.NewReader(s.data), err: s.responseErr, events: s.events}
	return packp.NewUploadPackResponseWithPackfile(req, s.body), nil
}

// regression: #2395
func TestCloseKeepsPayload(t *testing.T) {
	objs, oid, err := MintClaimTag("close-proof", "tag body", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	var pack bytes.Buffer
	if _, err := packfile.NewEncoder(&pack, objs, false).Encode([]plumbing.Hash{oid, plumbing.NewHash(EmptyBlobHash)}, 10); err != nil {
		t.Fatal(err)
	}
	for _, outcome := range []string{"success", "advertise-error", "upload-error", "decode-error"} {
		for _, failed := range []bool{false, true} {
			for _, subscribe := range []bool{false, true} {
				name := outcome
				if failed {
					name += "/close-fails"
				} else {
					name += "/close-ok"
				}
				if subscribe {
					name += "/trace"
				} else {
					name += "/nil"
				}
				t.Run(name, func(t *testing.T) {
					var events []string
					sess := &closeUpload{adv: packp.NewAdvRefs(), data: append([]byte(nil), pack.Bytes()...), events: &events}
					if failed {
						sess.closeErr = closeCanary
						sess.responseErr = closeCanary
					}
					if outcome == "advertise-error" {
						sess.advertiseErr = primaryCloseTest
					}
					if outcome == "upload-error" {
						sess.uploadErr = primaryCloseTest
					}
					if outcome == "decode-error" {
						sess.data = []byte("not a pack")
					}
					traces := 0
					var receipt TagFetchReceipt
					var trace func(TagFetchReceipt)
					if subscribe {
						trace = func(r TagFetchReceipt) { traces++; receipt = r; events = append(events, "trace") }
					}
					payload, err := fetchTagPayload(context.Background(), "fixture://example.invalid/repo", nil, oid, trace, func(*transport.Endpoint) (transport.Transport, error) { return closeClient{upload: sess}, nil })
					if outcome == "success" {
						if err != nil || payload.Message != "tag body" || !payload.When.Equal(time.Unix(1, 0)) {
							t.Fatalf("payload tuple changed: %+v %v", payload, err)
						}
					} else if outcome != "decode-error" {
						if !errors.Is(err, primaryCloseTest) {
							t.Fatalf("primary payload error changed: %v", err)
						}
					} else if err == nil || !strings.Contains(err.Error(), "upload-pack decode") {
						t.Fatalf("decode outcome changed: %v", err)
					}
					if err != nil && strings.Contains(err.Error(), "payload-canary") {
						t.Fatalf("close error replaced primary: %v", err)
					}
					wantBody := outcome == "success" || outcome == "decode-error"
					expected := []string{}
					if wantBody {
						expected = append(expected, "response-close")
					}
					expected = append(expected, "session-close")
					if subscribe {
						expected = append(expected, "trace")
					}
					if sess.closes != 1 || !reflect.DeepEqual(events, expected) {
						t.Fatalf("historical close order/count changed: %v want %v", events, expected)
					}
					if wantBody && (sess.body == nil || sess.body.calls != 1) {
						t.Fatal("response outer close count changed")
					}
					if subscribe {
						if traces != 1 || receipt.SessionCloseFailed != failed || receipt.ResponseCloseFailed != (failed && wantBody) {
							t.Fatalf("optional diagnostic incorrect: %+v calls=%d", receipt, traces)
						}
						encoded, e := json.Marshal(receipt)
						if e != nil {
							t.Fatal(e)
						}
						if strings.Contains(string(encoded), "canary") {
							t.Fatalf("unbounded close error material in diagnostic %s", encoded)
						}
					} else if traces != 0 {
						t.Fatal("nil subscriber unexpectedly reported")
					}
					wantCalls := 1
					if outcome == "advertise-error" {
						wantCalls = 0
					}
					if sess.calls != wantCalls {
						t.Fatalf("upload retried: %d", sess.calls)
					}
				})
			}
		}
	}
}

// regression: #2395
func TestAckCloseIsOutcome(t *testing.T) {
	ref := plumbing.ReferenceName("refs/dispatch/close-proof")
	req := packp.NewReferenceUpdateRequest()
	if err := req.Capabilities.Set(capability.ReportStatus); err != nil {
		t.Fatal(err)
	}
	req.Commands = []*packp.Command{{Name: ref}}
	var events []string
	body := &closeRead{Reader: strings.NewReader(ackPkt("unpack ok\n") + ackPkt("ok "+ref.String()+"\n") + "0000"), err: closeCanary, events: &events}
	observed := &ackTransport{body: &ackBody{ReadCloser: body}}
	report := &packp.ReportStatus{UnpackStatus: "ok", CommandStatuses: []*packp.CommandStatus{{ReferenceName: ref, Status: "ok"}}}
	v, err := receiveClaim(context.Background(), stubReceive{report}, req, &bytes.Buffer{}, observed, &RefReceipt{})
	if err == nil || !strings.Contains(err.Error(), "response read failed") || body.calls != 1 {
		t.Fatalf("outcome-bearing close demoted: %+v %v close=%d", v, err, body.calls)
	}
	if strings.Contains(err.Error(), "payload-canary") {
		t.Fatal("raw close material escaped outcome error")
	}
}
