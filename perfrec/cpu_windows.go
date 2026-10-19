//go:build windows

package perfrec

// CPUMillis cannot measure process CPU time on this build, and says so: ok=false, so the
// record OMITS cpuMs. A zero would read as "this ran in no CPU time at all", which is a
// measurement claim this platform arm cannot make.
func CPUMillis() (int64, bool) { return 0, false }

// cpuSampler is nil on this build: SystemProbe reports CPU coverage "unsupported", never zero.
var cpuSampler func() (int64, bool)
