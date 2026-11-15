package cellcontainer

import (
	"slices"
	"strings"
)

// runtimeRestrictions are private expectations selected by a closed profile.
// There is no caller-facing switch that bypasses a runtime protection.
type runtimeRestrictions struct {
	pids, memory, cpu int64
	tmpfs, network    string
}

// isolationFailure is shared by the legacy role profile and authority profile.
// The legacy values and refusal order remain unchanged.
func isolationFailure(s *Inspection, w runtimeRestrictions) string {
	h := s.HostConfig
	switch {
	case !h.ReadonlyRootfs:
		return "root filesystem is writable"
	case h.Privileged:
		return "is privileged"
	case len(h.CapAdd) > 0:
		return "adds capabilities"
	case !slices.Equal(h.CapDrop, []string{"ALL"}):
		return "does not drop all capabilities"
	case len(h.Devices) > 0:
		return "has devices"
	case len(h.DeviceRequests) > 0:
		return "requests devices (such as GPUs)"
	case len(h.Binds) > 0:
		return "has unplanned bind mounts"
	case len(h.GroupAdd) > 0:
		return "adds groups"
	case len(h.PortBindings) > 0 || h.PublishAllPorts:
		return "publishes ports"
	case h.PidsLimit != w.pids:
		return "process limit differs"
	case h.Memory != w.memory:
		return "memory limit differs"
	case h.NanoCpus != w.cpu:
		return "CPU limit differs"
	case h.PidMode != "":
		return "shares a process namespace"
	case h.IpcMode == "host" || strings.HasPrefix(h.IpcMode, "container:"):
		return "shares an IPC namespace"
	case h.UsernsMode != "":
		return "user namespace mode differs"
	}
	if len(h.SecurityOpt) != 1 || (h.SecurityOpt[0] != "no-new-privileges" && h.SecurityOpt[0] != "no-new-privileges=true") {
		return "security options differ"
	}
	if len(h.Tmpfs) != 1 || h.Tmpfs["/tmp"] != w.tmpfs {
		return "temporary filesystem differs"
	}
	if h.NetworkMode != w.network || len(s.NetworkSettings.Networks) != 1 {
		return "network differs"
	}
	if _, ok := s.NetworkSettings.Networks[w.network]; !ok {
		return "network differs"
	}
	return ""
}

func mountFailure(s *Inspection, expected []Mount) string {
	mounts := map[string]Mount{}
	for _, m := range s.Mounts {
		if m.Type == "tmpfs" && m.Destination == "/tmp" {
			continue
		}
		source := m.Source
		if m.Type == "volume" {
			source = m.Name
		}
		if _, ok := mounts[m.Destination]; ok {
			return "duplicate mount target"
		}
		mounts[m.Destination] = Mount{m.Type, source, m.Destination, !m.RW}
	}
	if len(mounts) != len(expected) {
		return "mount count differs"
	}
	for _, m := range expected {
		if mounts[m.Target] != m {
			return "mount differs at " + m.Target
		}
	}
	return ""
}
