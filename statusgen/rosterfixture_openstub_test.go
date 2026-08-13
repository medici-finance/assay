package main

// rosterfixture_openstub_test.go — the fixture roster gets no product-config
// lines, so the test tree carries no product-namespaced configuration. This
// no-op hook is now the only implementation.

func darFixtureExtra() string { return "" }
