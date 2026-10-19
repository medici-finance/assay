//go:build unix

package perfrec

import "syscall"

// CPUMillis returns this process's user+system CPU time in milliseconds, read from
// getrusage(RUSAGE_SELF). ok=false when the call fails, in which case the record OMITS cpuMs
// rather than reporting a zero it did not measure.
func CPUMillis() (int64, bool) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0, false
	}
	user := int64(ru.Utime.Sec)*1000 + int64(ru.Utime.Usec)/1000
	sys := int64(ru.Stime.Sec)*1000 + int64(ru.Stime.Usec)/1000
	return user + sys, true
}
