package main

const envAssayTelemetry = "ASSAY_TELEMETRY"

// The effective Env already applies cell > machine defaults > process precedence.
// Forward only a bounded opt-in, including explicit off, never arbitrary file text.
func (c *Cell) telemetryValue() string {
	if c.Env.Get(envAssayTelemetry) == "1" {
		return "1"
	}
	return "0"
}

func (c *Cell) telemetryEnv(env []string) []string {
	return envSet(env, envAssayTelemetry, c.telemetryValue())
}

// The desk launcher owns the assigned function, independent of DESK_LOOP's authority.
func perfDeskFunction(role string) string {
	switch role {
	case "the-desk":
		return "coordinate"
	case "worker-desk":
		return "implement"
	case "pr-review-desk":
		return "review"
	case "verify-desk":
		return "verify"
	case "intake-desk":
		return "triage"
	}
	return "other"
}
