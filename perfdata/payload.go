package perfdata

// payload.go — the typed, bounded payload of each v1 kind and its validation.

import "encoding/json"

// Basis states how a value was obtained. Unavailable carries no value; a value is never
// absent under any other basis.
type Basis string

const (
	BasisMeasured    Basis = "measured"
	BasisProxy       Basis = "proxy"
	BasisEstimated   Basis = "estimated"
	BasisUnavailable Basis = "unavailable"
)

// MaxMs bounds every millisecond value (about 31 days).
const MaxMs = int64(31 * 24 * 3600 * 1000)

// MaxCount bounds every counter on one event.
const MaxCount = 1_000_000

// Measurement is a nullable millisecond quantity with an explicit basis. Proxy and
// estimated values name their method token.
type Measurement struct {
	Basis  Basis  `json:"basis"`
	Ms     *int64 `json:"ms,omitempty"`
	Method string `json:"method,omitempty"`
}

// Measured returns a measured value.
func Measured(ms int64) Measurement { return Measurement{Basis: BasisMeasured, Ms: &ms} }

// Unavailable returns the explicit missing value.
func Unavailable() Measurement { return Measurement{Basis: BasisUnavailable} }

// Known reports whether m carries a value.
func (m Measurement) Known() bool { return m.Basis != BasisUnavailable && m.Ms != nil }

func (m Measurement) validate(field string) error {
	switch m.Basis {
	case BasisUnavailable:
		if m.Ms != nil || m.Method != "" {
			return refuse(RefuseMeasurement, field, "unavailable carries no value")
		}
		return nil
	case BasisMeasured:
		if m.Method != "" {
			return refuse(RefuseSchema, field, "measured values carry no method")
		}
	case BasisProxy, BasisEstimated:
		if !tokenRE.MatchString(m.Method) {
			return refuse(RefuseMeasurement, field, "proxy and estimated values name a method token")
		}
	default:
		return refuse(RefuseSchema, field, "unknown basis")
	}
	if m.Ms == nil {
		return refuse(RefuseMeasurement, field, "a value with a basis cannot be missing")
	}
	if *m.Ms < 0 || *m.Ms > MaxMs {
		return refuse(RefuseMeasurement, field, "value out of range")
	}
	return nil
}

// Outcome is the bounded terminal outcome.
type Outcome string

const (
	OutcomeOK        Outcome = "ok"
	OutcomeError     Outcome = "error"
	OutcomeRefused   Outcome = "refused"
	OutcomeCancelled Outcome = "cancelled"
	OutcomeUnknown   Outcome = "unknown"
)

var outcomes = map[Outcome]bool{OutcomeOK: true, OutcomeError: true, OutcomeRefused: true, OutcomeCancelled: true, OutcomeUnknown: true}

// CPU is process CPU split into own (this process), descendant (its waited children)
// and inclusive (own+descendant as the OS reports it). Reports add only Own across
// processes; Inclusive is never added to a child's own CPU.
type CPU struct {
	Own        Measurement `json:"own"`
	Descendant Measurement `json:"descendant"`
	Inclusive  Measurement `json:"inclusive"`
}

func (c CPU) validate() error {
	for _, f := range []struct {
		n string
		m Measurement
	}{{"payload.cpu.own", c.Own}, {"payload.cpu.descendant", c.Descendant}, {"payload.cpu.inclusive", c.Inclusive}} {
		if err := f.m.validate(f.n); err != nil {
			return err
		}
	}
	if c.Own.Known() && c.Descendant.Known() && c.Inclusive.Known() && c.Own.Basis == BasisMeasured &&
		c.Descendant.Basis == BasisMeasured && c.Inclusive.Basis == BasisMeasured &&
		*c.Inclusive.Ms != *c.Own.Ms+*c.Descendant.Ms {
		return refuse(RefuseMeasurement, "payload.cpu.inclusive", "inclusive must equal own plus descendant")
	}
	if c.Own.Known() && c.Inclusive.Known() && *c.Inclusive.Ms < *c.Own.Ms {
		return refuse(RefuseMeasurement, "payload.cpu.inclusive", "inclusive cannot be below own")
	}
	return nil
}

// Children counts the child processes this process started.
type Children struct {
	Started        int `json:"started"`
	Uninstrumented int `json:"uninstrumented"`
}

// ToolCompletion is one tool invocation's own summary.
type ToolCompletion struct {
	Tool      string      `json:"tool"`
	Verb      string      `json:"verb,omitempty"`
	Outcome   Outcome     `json:"outcome"`
	StartedAt string      `json:"startedAt"`
	Wall      Measurement `json:"wall"`
	CPU       CPU         `json:"cpu"`
	Children  Children    `json:"children"`
}

// PhaseName is the closed phase vocabulary.
type PhaseName string

var phases = map[PhaseName]bool{
	"boot": true, "auth": true, "audit": true,
	"collect": true, "page": true, "enrich": true, "classify": true,
	"plan": true, "workspace": true, "source": true, "packet": true,
	"claim": true, "recheck": true, "child": true,
}

// Phase is one bounded phase interval inside an invocation.
type Phase struct {
	Phase     PhaseName   `json:"phase"`
	StartedAt string      `json:"startedAt"`
	Wall      Measurement `json:"wall"`
	CPUOwn    Measurement `json:"cpuOwn"`
}

// AttemptState is an attempt lifecycle transition.
type AttemptState string

const (
	AttemptStarted   AttemptState = "started"
	AttemptFinished  AttemptState = "finished"
	AttemptAbandoned AttemptState = "abandoned"
)

// Attempt is a workflow attempt lifecycle transition.
type Attempt struct {
	State   AttemptState `json:"state"`
	Outcome Outcome      `json:"outcome,omitempty"`
}

// UsageMode states whether a usage record is a delta or a cumulative snapshot.
type UsageMode string

const (
	UsageDelta      UsageMode = "delta"
	UsageCumulative UsageMode = "cumulative"
)

// Cost is a monetary amount in micro-units of Currency, with its pricing revision.
type Cost struct {
	MicroUnits      int64  `json:"microUnits"`
	Currency        string `json:"currency"`
	PricingRevision string `json:"pricingRevision"`
	Basis           Basis  `json:"basis"`
	Method          string `json:"method,omitempty"`
}

// Usage is model usage for the event's invocation. Nil token fields are unknown.
// A cumulative snapshot replaces the previous snapshot of the same series
// (invocation + model); it is never added to it.
type Usage struct {
	Mode             UsageMode `json:"mode"`
	Model            string    `json:"model"`
	InputTokens      *int64    `json:"inputTokens,omitempty"`
	OutputTokens     *int64    `json:"outputTokens,omitempty"`
	CacheReadTokens  *int64    `json:"cacheReadTokens,omitempty"`
	CacheWriteTokens *int64    `json:"cacheWriteTokens,omitempty"`
	Cost             *Cost     `json:"cost,omitempty"`
}

// Forge, EndpointClass, Origin and Scope bound an HTTP attempt group.
type (
	Forge         string
	EndpointClass string
	Origin        string
	Scope         string
	PointUnit     string
)

var (
	forges          = map[Forge]bool{"github": true, "gitlab": true, "none": true, "unknown": true}
	endpointClasses = map[EndpointClass]bool{"rest-read": true, "rest-write": true, "graphql": true, "token-mint": true, "local-service": true, "other": true}
	origins         = map[Origin]bool{"direct": true, "service-passthrough": true, "refresh": true, "poll": true, "repair": true, "unknown": true}
	pointUnits      = map[PointUnit]bool{"github-points": true, "gitlab-complexity": true}
)

// ScopeOwn is the only admitted request-counter scope: this process's own attempts.
const ScopeOwn Scope = "own"

// Points is GraphQL cost observation per attempt. Every attempt is in exactly one
// state, so unknown cost can never disappear into a total; sums are nil when no sample
// supports them.
type Points struct {
	Unit               PointUnit `json:"unit"`
	MeasuredSum        *int64    `json:"measuredSum,omitempty"`
	MeasuredSamples    int       `json:"measuredSamples"`
	EstimatedSum       *int64    `json:"estimatedSum,omitempty"`
	EstimatedSamples   int       `json:"estimatedSamples"`
	EstimateMethod     string    `json:"estimateMethod,omitempty"`
	UnknownSamples     int       `json:"unknownSamples"`
	UnsupportedSamples int       `json:"unsupportedSamples"`
	FailedSamples      int       `json:"failedSamples"`
}

// HTTPAttempts is a group of actual HTTP attempts by this process at one bounded
// forge/endpoint-class/origin.
type HTTPAttempts struct {
	Forge       Forge         `json:"forge"`
	Class       EndpointClass `json:"class"`
	Origin      Origin        `json:"origin"`
	Scope       Scope         `json:"scope"`
	Attempts    int           `json:"attempts"`
	Retries     int           `json:"retries"`
	NotModified int           `json:"notModified"`
	Failed      int           `json:"failed"`
	Duration    Distribution  `json:"duration"`
	Points      *Points       `json:"points,omitempty"`
}

// CallOutcomes counts logical service-call outcomes.
type CallOutcomes struct {
	OK        int `json:"ok"`
	Error     int `json:"error"`
	Timeout   int `json:"timeout"`
	Cancelled int `json:"cancelled"`
	Denied    int `json:"denied"`
}

// ServiceCall is the client view of calls to a query service: logical calls and
// transport attempts, never provider work.
type ServiceCall struct {
	Operation         string       `json:"operation"`
	LogicalCalls      int          `json:"logicalCalls"`
	TransportAttempts int          `json:"transportAttempts"`
	Retries           int          `json:"retries"`
	Outcomes          CallOutcomes `json:"outcomes"`
	Duration          Distribution `json:"duration"`
}

// CacheOutcomes counts handler response sources.
type CacheOutcomes struct {
	Hit         int `json:"hit"`
	Miss        int `json:"miss"`
	Stale       int `json:"stale"`
	Bypass      int `json:"bypass"`
	Unavailable int `json:"unavailable"`
}

// Coalescing counts coalesced leader and waiter requests.
type Coalescing struct {
	Leader int `json:"leader"`
	Waiter int `json:"waiter"`
}

// UpstreamCoverage states whether the handler's upstream seams were fully observed;
// only then can zero linked provider attempts mean a measured zero.
type UpstreamCoverage string

const (
	UpstreamObserved   UpstreamCoverage = "observed"
	UpstreamUnobserved UpstreamCoverage = "unobserved"
)

// Handler is the server view of handled requests.
type Handler struct {
	Operation  string           `json:"operation"`
	Requests   int              `json:"requests"`
	Cache      CacheOutcomes    `json:"cache"`
	Coalescing Coalescing       `json:"coalescing"`
	Upstream   UpstreamCoverage `json:"upstream"`
	Duration   Distribution     `json:"duration"`
}

// Health is collection health from the writer itself.
type Health struct {
	QueueFull    int64        `json:"queueFull"`
	WriteFailed  int64        `json:"writeFailed"`
	FlushTimeout int64        `json:"flushTimeout"`
	Refused      int64        `json:"refused"`
	WriteLatency Distribution `json:"writeLatency"`
}

// CorrectionReason is why an earlier event is corrected.
type CorrectionReason string

var correctionReasons = map[CorrectionReason]bool{"wrong-value": true, "wrong-basis": true, "wrong-attribution": true, "retracted": true}

// Correction excludes an earlier event of the same source from aggregates; the old
// record is retained. A replacement value is a new ordinary event.
type Correction struct {
	Corrects string           `json:"corrects"`
	Digest   string           `json:"digest"`
	Reason   CorrectionReason `json:"reason"`
}

// DecodePayload strictly decodes and validates the payload of kind k.
func DecodePayload(k Kind, raw json.RawMessage) (any, error) {
	var v interface{ validate() error }
	switch k {
	case KindTool:
		v = &ToolCompletion{}
	case KindPhase:
		v = &Phase{}
	case KindAttempt:
		v = &Attempt{}
	case KindUsage:
		v = &Usage{}
	case KindHTTP:
		v = &HTTPAttempts{}
	case KindCall:
		v = &ServiceCall{}
	case KindHandler:
		v = &Handler{}
	case KindHealth:
		v = &Health{}
	case KindCorrection:
		v = &Correction{}
	default:
		return nil, refuse(RefuseUnsupported, "kind", "unsupported kind")
	}
	if len(raw) == 0 {
		return nil, refuse(RefuseSchema, "payload", "payload is required")
	}
	if err := strictUnmarshal(raw, v); err != nil {
		r := AsRefusal(err)
		return nil, refuse(r.Class, "payload", r.Rule)
	}
	if err := v.validate(); err != nil {
		return nil, err
	}
	return v, nil
}

func count(field string, n int) error {
	if n < 0 || n > MaxCount {
		return refuse(RefuseMeasurement, field, "count out of range")
	}
	return nil
}

func (p *ToolCompletion) validate() error {
	if !tokenRE.MatchString(p.Tool) {
		return refuse(RefusePrivacy, "payload.tool", "tool is not a bounded vocabulary token")
	}
	if p.Verb != "" && !tokenRE.MatchString(p.Verb) {
		return refuse(RefusePrivacy, "payload.verb", "verb is not a bounded vocabulary token")
	}
	if !outcomes[p.Outcome] {
		return refuse(RefuseSchema, "payload.outcome", "unknown outcome")
	}
	if _, ok := ParseTime(p.StartedAt); !ok {
		return refuse(RefuseSchema, "payload.startedAt", "not a canonical UTC timestamp")
	}
	if err := p.Wall.validate("payload.wall"); err != nil {
		return err
	}
	if err := p.CPU.validate(); err != nil {
		return err
	}
	if err := count("payload.children.started", p.Children.Started); err != nil {
		return err
	}
	if err := count("payload.children.uninstrumented", p.Children.Uninstrumented); err != nil {
		return err
	}
	if p.Children.Uninstrumented > p.Children.Started {
		return refuse(RefuseMeasurement, "payload.children.uninstrumented", "more uninstrumented children than started")
	}
	return nil
}

func (p *Phase) validate() error {
	if !phases[p.Phase] {
		return refuse(RefusePrivacy, "payload.phase", "phase is not in the closed vocabulary")
	}
	if _, ok := ParseTime(p.StartedAt); !ok {
		return refuse(RefuseSchema, "payload.startedAt", "not a canonical UTC timestamp")
	}
	if err := p.Wall.validate("payload.wall"); err != nil {
		return err
	}
	return p.CPUOwn.validate("payload.cpuOwn")
}

func (p *Attempt) validate() error {
	switch p.State {
	case AttemptStarted:
		if p.Outcome != "" {
			return refuse(RefuseSchema, "payload.outcome", "a started attempt has no outcome")
		}
	case AttemptFinished, AttemptAbandoned:
		if !outcomes[p.Outcome] {
			return refuse(RefuseSchema, "payload.outcome", "unknown outcome")
		}
	default:
		return refuse(RefuseSchema, "payload.state", "unknown attempt state")
	}
	return nil
}

func (p *Usage) validate() error {
	if p.Mode != UsageDelta && p.Mode != UsageCumulative {
		return refuse(RefuseSchema, "payload.mode", "usage mode must be delta or cumulative")
	}
	if p.Model != "unavailable" && !revisionRE.MatchString(p.Model) {
		return refuse(RefusePrivacy, "payload.model", "model is not a bounded token")
	}
	for _, f := range []struct {
		n string
		v *int64
	}{{"payload.inputTokens", p.InputTokens}, {"payload.outputTokens", p.OutputTokens}, {"payload.cacheReadTokens", p.CacheReadTokens}, {"payload.cacheWriteTokens", p.CacheWriteTokens}} {
		if f.v != nil && (*f.v < 0 || *f.v > 1<<40) {
			return refuse(RefuseMeasurement, f.n, "token count out of range")
		}
	}
	if c := p.Cost; c != nil {
		if c.MicroUnits < 0 || !currencyRE.MatchString(c.Currency) || !revisionRE.MatchString(c.PricingRevision) {
			return refuse(RefuseMeasurement, "payload.cost", "cost needs amount, currency and pricing revision")
		}
		switch c.Basis {
		case BasisMeasured:
			if c.Method != "" {
				return refuse(RefuseSchema, "payload.cost.method", "measured cost carries no method")
			}
		case BasisEstimated:
			if !tokenRE.MatchString(c.Method) {
				return refuse(RefuseMeasurement, "payload.cost.method", "estimated cost names a method token")
			}
		default:
			return refuse(RefuseSchema, "payload.cost.basis", "cost basis must be measured or estimated")
		}
	}
	return nil
}

func (p *HTTPAttempts) validate() error {
	if !forges[p.Forge] {
		return refuse(RefuseSchema, "payload.forge", "unknown forge")
	}
	if !endpointClasses[p.Class] {
		return refuse(RefuseSchema, "payload.class", "unknown endpoint class")
	}
	if !origins[p.Origin] {
		return refuse(RefuseSchema, "payload.origin", "unknown origin")
	}
	if p.Scope != ScopeOwn {
		return refuse(RefuseMeasurement, "payload.scope", "request counters are own-process only; a parent never carries descendant attempts")
	}
	for _, f := range []struct {
		n string
		v int
	}{{"payload.attempts", p.Attempts}, {"payload.retries", p.Retries}, {"payload.notModified", p.NotModified}, {"payload.failed", p.Failed}} {
		if err := count(f.n, f.v); err != nil {
			return err
		}
		if f.v > p.Attempts {
			return refuse(RefuseMeasurement, f.n, "cannot exceed attempts")
		}
	}
	if err := p.Duration.validate("payload.duration"); err != nil {
		return err
	}
	if p.Duration.Count > int64(p.Attempts) {
		return refuse(RefuseMeasurement, "payload.duration", "more durations than attempts")
	}
	if p.Class != "graphql" {
		if p.Points != nil {
			return refuse(RefuseMeasurement, "payload.points", "points belong to graphql attempts only")
		}
		return nil
	}
	if p.Points == nil {
		return refuse(RefuseMeasurement, "payload.points", "graphql attempts state their point coverage, unknown included")
	}
	return p.Points.validate(p.Attempts)
}

func (p *Points) validate(attempts int) error {
	if !pointUnits[p.Unit] {
		return refuse(RefuseSchema, "payload.points.unit", "unknown point unit")
	}
	total := 0
	for _, n := range []int{p.MeasuredSamples, p.EstimatedSamples, p.UnknownSamples, p.UnsupportedSamples, p.FailedSamples} {
		if err := count("payload.points", n); err != nil {
			return err
		}
		total += n
	}
	if total != attempts {
		return refuse(RefuseMeasurement, "payload.points", "every attempt has exactly one point state; unknown is never dropped")
	}
	if (p.MeasuredSum == nil) != (p.MeasuredSamples == 0) {
		return refuse(RefuseMeasurement, "payload.points.measuredSum", "a measured sum needs measured samples and vice versa; unknown is not zero")
	}
	if (p.EstimatedSum == nil) != (p.EstimatedSamples == 0) {
		return refuse(RefuseMeasurement, "payload.points.estimatedSum", "an estimated sum needs estimated samples and vice versa")
	}
	if p.EstimatedSamples > 0 && !tokenRE.MatchString(p.EstimateMethod) {
		return refuse(RefuseMeasurement, "payload.points.estimateMethod", "estimates name a method token")
	}
	if p.EstimatedSamples == 0 && p.EstimateMethod != "" {
		return refuse(RefuseSchema, "payload.points.estimateMethod", "no estimate to describe")
	}
	for _, v := range []*int64{p.MeasuredSum, p.EstimatedSum} {
		if v != nil && (*v < 0 || *v > 1<<40) {
			return refuse(RefuseMeasurement, "payload.points", "point sum out of range")
		}
	}
	return nil
}

func (p *ServiceCall) validate() error {
	if !tokenRE.MatchString(p.Operation) {
		return refuse(RefusePrivacy, "payload.operation", "operation is not a bounded vocabulary token")
	}
	for _, f := range []struct {
		n string
		v int
	}{{"payload.logicalCalls", p.LogicalCalls}, {"payload.transportAttempts", p.TransportAttempts}, {"payload.retries", p.Retries}} {
		if err := count(f.n, f.v); err != nil {
			return err
		}
	}
	o := p.Outcomes
	if o.OK < 0 || o.Error < 0 || o.Timeout < 0 || o.Cancelled < 0 || o.Denied < 0 ||
		o.OK+o.Error+o.Timeout+o.Cancelled+o.Denied != p.LogicalCalls {
		return refuse(RefuseMeasurement, "payload.outcomes", "outcomes must account for every logical call")
	}
	if p.Retries > p.TransportAttempts {
		return refuse(RefuseMeasurement, "payload.retries", "cannot exceed transport attempts")
	}
	if err := p.Duration.validate("payload.duration"); err != nil {
		return err
	}
	if p.Duration.Count > int64(p.LogicalCalls) {
		return refuse(RefuseMeasurement, "payload.duration", "more durations than logical calls")
	}
	return nil
}

func (p *Handler) validate() error {
	if !tokenRE.MatchString(p.Operation) {
		return refuse(RefusePrivacy, "payload.operation", "operation is not a bounded vocabulary token")
	}
	if err := count("payload.requests", p.Requests); err != nil {
		return err
	}
	c := p.Cache
	if c.Hit < 0 || c.Miss < 0 || c.Stale < 0 || c.Bypass < 0 || c.Unavailable < 0 ||
		c.Hit+c.Miss+c.Stale+c.Bypass+c.Unavailable != p.Requests {
		return refuse(RefuseMeasurement, "payload.cache", "cache outcomes must account for every request")
	}
	if p.Coalescing.Leader < 0 || p.Coalescing.Waiter < 0 || p.Coalescing.Leader+p.Coalescing.Waiter > p.Requests {
		return refuse(RefuseMeasurement, "payload.coalescing", "coalescing counts exceed requests")
	}
	if p.Upstream != UpstreamObserved && p.Upstream != UpstreamUnobserved {
		return refuse(RefuseSchema, "payload.upstream", "unknown upstream coverage")
	}
	if err := p.Duration.validate("payload.duration"); err != nil {
		return err
	}
	if p.Duration.Count > int64(p.Requests) {
		return refuse(RefuseMeasurement, "payload.duration", "more durations than requests")
	}
	return nil
}

func (p *Health) validate() error {
	for _, v := range []int64{p.QueueFull, p.WriteFailed, p.FlushTimeout, p.Refused} {
		if v < 0 || v > 1<<40 {
			return refuse(RefuseMeasurement, "payload", "health counter out of range")
		}
	}
	return p.WriteLatency.validate("payload.writeLatency")
}

func (p *Correction) validate() error {
	if !eventIDRE.MatchString(p.Corrects) {
		return refuse(RefuseSchema, "payload.corrects", "not an event identity")
	}
	if !digestRE.MatchString(p.Digest) {
		return refuse(RefuseSchema, "payload.digest", "not a sha256 digest")
	}
	if !correctionReasons[p.Reason] {
		return refuse(RefuseSchema, "payload.reason", "unknown correction reason")
	}
	return nil
}
