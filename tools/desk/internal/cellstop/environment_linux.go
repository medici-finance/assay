package cellstop

import (
	"fmt"
	"os"
	"strings"
)

func processEnvironment(pid int) (map[string]string, error) {
	env, _, err := processEnvironmentRead(pid)
	return env, err
}

// processEnvironmentRead also reports whether any environment entry was readable.
func processEnvironmentRead(pid int) (map[string]string, bool, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return nil, false, err
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
