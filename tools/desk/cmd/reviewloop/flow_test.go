package main

var actionTable = func() map[string]rule {
	out := map[string]rule{}
	for _, a := range KnownActions() {
		out[a], _ = LookupAction(a)
	}
	return out
}()
