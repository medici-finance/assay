package main

import (
	"io"
	"os"
)

// stdout/stderr are package vars so tests can capture them.
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)
