package cellstop

import (
	"fmt"
	"os"
	"strings"
)

func processEnvironment(pid int) (map[string]string, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return nil, err
	}
	return relevantEnvironment(strings.Split(string(b), "\x00")), nil
}
