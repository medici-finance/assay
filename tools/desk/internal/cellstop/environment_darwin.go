package cellstop

import (
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/unix"
	"strings"
)

func processEnvironment(pid int) (map[string]string, error) {
	b, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return nil, err
	}
	if len(b) < 4 {
		return nil, fmt.Errorf("process arguments unavailable")
	}
	argc := int(binary.NativeEndian.Uint32(b[:4]))
	b = b[4:]
	// procargs2 stores executable, padding, argc NUL-terminated arguments, then env.
	skip := func() bool {
		for i, v := range b {
			if v == 0 {
				b = b[i+1:]
				return true
			}
		}
		return false
	}
	if !skip() {
		return nil, fmt.Errorf("process executable unavailable")
	}
	for len(b) > 0 && b[0] == 0 {
		b = b[1:]
	}
	for i := 0; i < argc; i++ {
		if !skip() {
			return nil, fmt.Errorf("process arguments truncated")
		}
	}
	return relevantEnvironment(strings.Split(string(b), "\x00")), nil
}
