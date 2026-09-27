// Package lib is TestCountsFixture's verbs-dimension decoy: a cmd/ subdirectory that is
// NOT package main, so it must not be counted as a verb.
package lib

// Helper exists only so this file is not empty.
func Helper() string { return "not a verb" }
