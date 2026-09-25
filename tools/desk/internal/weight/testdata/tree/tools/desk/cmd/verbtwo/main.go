// Package main is TestCountsFixture's second verb. Its flag registrations exercise the
// …Var-form flags dimension (DurationVar) alongside a second plain form.
package main

import (
	"flag"
	"time"
)

var timeout time.Duration

func main() {
	fs := flag.NewFlagSet("verbtwo", flag.ContinueOnError)
	fs.DurationVar(&timeout, "timeout", 0, "the timeout flag — counted: …Var form, literal name")
	_ = fs.Duration("retry", 0, "the retry flag — counted: plain form, literal name")
}
