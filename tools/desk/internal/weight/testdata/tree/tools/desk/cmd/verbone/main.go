// Package main is TestCountsFixture's first verb: a cmd/ subdirectory with a package-main
// file, counted as one verb. Its flag registrations exercise the plain-form flags
// dimension and its non-literal-name decoy.
package main

import "flag"

func main() {
	fs := flag.NewFlagSet("verbone", flag.ContinueOnError)
	_ = fs.String("name", "", "the name flag — counted: plain form, literal name")

	dynamicName := computeFlagName()
	_ = fs.Bool(dynamicName, false, "decoy: non-literal name argument — NOT counted")
}

func computeFlagName() string { return "computed-at-runtime" }
