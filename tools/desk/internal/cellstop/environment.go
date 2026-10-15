//go:build darwin || linux

package cellstop

import "strings"

func relevantEnvironment(entries []string) map[string]string {
	out := map[string]string{}
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		switch key {
		case "GOCACHE", "GOMODCACHE", "GOPATH", "DESK_SESSION", "DESK_ROOTS", "HOME", "ASSAY_CONFIG_HOME", "PWD",
			EnvCell, EnvCellDir, EnvCellRole:
			out[key] = value
		}
	}
	return out
}
