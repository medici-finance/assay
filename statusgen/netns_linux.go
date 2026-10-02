//go:build linux

package main

import (
	"net"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// runNetnsHelper is the in-namespace half of the check:ci sandbox (netns.go).
// args are [callerNetNS, argv0, argv...]. It brings lo up, proves isolation, and
// execs argv — or refuses. It returns only to refuse.
func runNetnsHelper(args []string) int {
	if len(args) < 2 {
		return netnsRefuse("the helper needs the caller's network namespace id and a command")
	}
	parentNS, argv := args[0], args[1:]

	if err := loopbackUp(); err != nil {
		return netnsRefuse("could not bring lo up inside the network namespace: %v", err)
	}
	own, _ := os.Readlink("/proc/self/ns/net")
	ifs, err := netnsSnapshot()
	if err != nil {
		return netnsRefuse("could not read the namespace's interfaces: %v", err)
	}
	if p := netnsIsolationProblem(own, parentNS, ifs); p != "" {
		return netnsRefuse("%s", p)
	}

	path, err := exec.LookPath(argv[0])
	if err != nil {
		return netnsRefuse("cannot resolve the row's shell %q: %v", argv[0], err)
	}
	err = syscall.Exec(path, argv, os.Environ())
	return netnsRefuse("exec of the row's shell failed: %v", err)
}

// loopbackUp sets IFF_UP on lo. It ALWAYS issues the set (never "already up,
// skip"): the kernel checks CAP_NET_ADMIN over the namespace before acting, so
// outside a namespace this process owns the call fails, and that failure is a
// refusal rather than a silent no-op.
func loopbackUp() error {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	ifr, err := unix.NewIfreq("lo")
	if err != nil {
		return err
	}
	if err := unix.IoctlIfreq(fd, unix.SIOCGIFFLAGS, ifr); err != nil {
		return err
	}
	ifr.SetUint16(ifr.Uint16() | unix.IFF_UP)
	return unix.IoctlIfreq(fd, unix.SIOCSIFFLAGS, ifr)
}

// netnsSnapshot reads the interfaces visible in the current network namespace.
func netnsSnapshot() ([]netnsIface, error) {
	list, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]netnsIface, 0, len(list))
	for _, ifc := range list {
		addrs, err := ifc.Addrs()
		if err != nil {
			return nil, err
		}
		out = append(out, netnsIface{
			Name:     ifc.Name,
			Loopback: ifc.Flags&net.FlagLoopback != 0,
			Up:       ifc.Flags&net.FlagUp != 0,
			Addrs:    len(addrs),
		})
	}
	return out, nil
}
