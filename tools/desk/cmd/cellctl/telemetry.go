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
