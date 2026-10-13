package deskfilestub

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(Run(m)) }

func TestStubRefuses(t *testing.T) { AssertStubbed(t) }
