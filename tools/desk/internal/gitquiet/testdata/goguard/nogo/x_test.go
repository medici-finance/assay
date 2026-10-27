package nogo

import "testing"

func TestHome(t *testing.T) { t.Setenv("HOME", t.TempDir()) }
