//go:build !houseprivate

package main

// rosterfixture_openstub_test.go — default (public / open-core) build. The
// fixture roster gets no product-config lines, so the shipped test tree carries
// no product-namespaced configuration. The house build supplies the real product
// fixtures behind the `houseprivate` build tag.

func darFixtureExtra() string { return "" }
