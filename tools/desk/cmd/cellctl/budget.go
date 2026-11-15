package main

// muharBudgetEnv is the one composition path for host, scrubbed, plan and
// Codex command environments. Unset inputs stay absent; explicit empty values
// reach muhar's invalid-pair refusal. No cell or host default is invented here.
func (c *Cell) muharBudgetEnv(env []string) []string {
	inputs := []struct {
		key, value string
		set        bool
	}{
		{"MUHAR_LEASE_DIR", c.Env.Get("MUHAR_LEASE_DIR"), c.Env.IsSet("MUHAR_LEASE_DIR")},
		{"MUHAR_TOTAL_SLOTS", c.Env.Get("MUHAR_TOTAL_SLOTS"), c.Env.IsSet("MUHAR_TOTAL_SLOTS")},
	}
	for _, input := range inputs {
		env = envUnset(env, input.key)
		if input.set {
			env = envSet(env, input.key, input.value)
		}
	}
	return env
}
