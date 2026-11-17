//go:build race

package main

// raceBuild reports whether this test binary carries the race detector. Its instrumentation
// allocates on the measured goroutine, so an allocation ceiling read under it measures the
// detector, not the code.
const raceBuild = true
