//go:build race

package main

// raceBuild reports whether this test binary is a race build. A race build's sync.Pool keeps
// fewer of the values put back into it, so code that pools its scratch state (regexp's
// matchers, for one) rebuilds it and allocates far more than an ordinary build does; an
// allocation ceiling read under it needs its own, wider bound.
const raceBuild = true
