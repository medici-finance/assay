package runner

import "errors"

// Sentinel errors. Callers match with errors.Is; every refusal wraps one of
// these so a consumer can tell WHY a request or result was refused.
var (
	// ErrUnsupportedVersion: the request names a protocol version this
	// consumer does not implement.
	ErrUnsupportedVersion = errors.New("runner: unsupported protocol version")
	// ErrInvalidRequest: a required request field is absent or malformed.
	ErrInvalidRequest = errors.New("runner: invalid launch request")
	// ErrModeFields: the request's references do not match its execution mode.
	ErrModeFields = errors.New("runner: references do not match execution mode")
	// ErrUnsupportedMandatory: the request marks a capability or extension as
	// mandatory and the adapter (or the contract) does not provide it.
	ErrUnsupportedMandatory = errors.New("runner: unsupported mandatory capability or field")
	// ErrResumeUnsupported: the request asks to resume a session and the
	// adapter does not declare resume. A fresh session is never substituted.
	ErrResumeUnsupported = errors.New("runner: resume not supported by adapter")
	// ErrCredential: a request or result carries credential material.
	ErrCredential = errors.New("runner: credential material is not allowed in a packet or result")
	// ErrIdentityReuse: the invocation identity was already used for a
	// different request.
	ErrIdentityReuse = errors.New("runner: invocation identity reused with a different request")
	// ErrReconcileRequired: an earlier launch for this invocation, or for the
	// same authority, has an unknown or unfinished outcome; reconcile it before
	// launching anything else.
	ErrReconcileRequired = errors.New("runner: unknown or unfinished launch requires reconcile")
	// ErrUnknownInvocation: no such invocation is known to this client.
	ErrUnknownInvocation = errors.New("runner: unknown invocation")
	// ErrDefiniteFailure is returned by an adapter, wrapped, ONLY when it can
	// state that the launch definitely did not start. Any other Start error is
	// an unknown outcome.
	ErrDefiniteFailure = errors.New("runner: launch definitely failed")
	// ErrAckLost is the conventional adapter error for a launch whose
	// acknowledgment never arrived. It is an unknown outcome, not a failure.
	ErrAckLost = errors.New("runner: launch acknowledgment lost")
	// ErrModelFallback: the adapter ran a model other than the pinned one.
	ErrModelFallback = errors.New("runner: observed model differs from the pinned model")
	// ErrNotFinished: the invocation has no terminal observation to accept.
	ErrNotFinished = errors.New("runner: invocation has not finished")
	// ErrMalformedResult: the adapter's result is not well formed.
	ErrMalformedResult = errors.New("runner: malformed result")
	// ErrUnauthorizedTool: a tool request is outside the pinned profile.
	ErrUnauthorizedTool = errors.New("runner: tool request outside the pinned profile")
	// ErrFenced: the attempt's generation is not the current authority
	// generation, so its result cannot be accepted however well formed.
	ErrFenced = errors.New("runner: result is from a fenced authority generation")
	// ErrFenceUnavailable: the current generation could not be read. Fail closed.
	ErrFenceUnavailable = errors.New("runner: authority generation could not be checked")
	// ErrStateRegression: the adapter reported an invocation leaving a terminal
	// state (finished, failed, stopped), or a finished invocation with a
	// different result. The report is a contract violation and is not recorded.
	ErrStateRegression = errors.New("runner: adapter reported an invocation leaving a terminal state")
	// ErrModelUnreported: the finished observation names no model. An
	// unreported model is unknown, and unknown is not the pinned model.
	ErrModelUnreported = errors.New("runner: observed model was not reported")
	// ErrResultIdentity: the result echoes another invocation's identity.
	ErrResultIdentity = errors.New("runner: result echoes another invocation's identity")
	// ErrCancelFailed: the adapter returned an error for a cancel request, so
	// the request may not have been taken. The recorded state is unchanged.
	ErrCancelFailed = errors.New("runner: the adapter failed the cancel request")
)

// opaque carries an error from outside the contract (an adapter, a fence)
// behind one of the contract's sentinels. Its text is the sentinel's alone:
// whatever the outside error says, a caller that logs the refusal logs only
// the contract's own words. Unwrap keeps both in the chain, so errors.Is
// matches the sentinel and anything the outside error wraps (a definite
// failure, a lost acknowledgment, a context deadline).
//
// Build it with a composite literal whose sentinel is a package sentinel; the
// source guard TestNoPayloadInErrors checks every literal and that Error never
// reads cause.
type opaque struct {
	sentinel error
	cause    error
}

func (e *opaque) Error() string   { return e.sentinel.Error() }
func (e *opaque) Unwrap() []error { return []error{e.sentinel, e.cause} }
