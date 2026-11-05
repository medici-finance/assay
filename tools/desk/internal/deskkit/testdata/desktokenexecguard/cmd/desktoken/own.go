// Package main is a fixture: the minter's own package names itself, and the exec guard must
// not report it. It is never compiled (testdata).
package main

import "flag"

var fs = flag.NewFlagSet("desktoken", flag.ContinueOnError)
