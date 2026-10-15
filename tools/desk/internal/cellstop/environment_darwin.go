package cellstop

import (
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/unix"
	"strings"
)

func processEnvironment(pid int) (map[string]string, error) {
	env, _, err := processEnvironmentRead(pid)
	return env, err
}

// processEnvironmentRead also reports whether the environment was actually readable: macOS
// returns the arguments of its platform binaries with the environment withheld, which is not
// the same thing as an empty environment.
func processEnvironmentRead(pid int) (map[string]string, bool, error) {
	b, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return nil, false, err
	}
	if len(b) < 4 {
		return nil, false, fmt.Errorf("process arguments unavailable")
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
		return nil, false, fmt.Errorf("process executable unavailable")
	}
	for len(b) > 0 && b[0] == 0 {
		b = b[1:]
	}
	for i := 0; i < argc; i++ {
		if !skip() {
			return nil, false, fmt.Errorf("process arguments truncated")
		}
	}
	entries := strings.Split(string(b), "\x00")
	readable := false
	for _, e := range entries {
		if strings.Contains(e, "=") {
			readable = true
			break
		}
	}
	return relevantEnvironment(entries), readable, nil
}
