//go:build windows

package deskkit

// processCPUMillis cannot measure process CPU time on this build, and says so: ok=false, so
// the record OMITS cpuMs. A zero would read as "this ran in no CPU time at all", which is a
// measurement claim this platform arm cannot make.
func processCPUMillis() (int64, bool) { return 0, false }
