//go:build windows

package cellprocess

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsTree struct {
	job      windows.Handle
	assigned bool
}

func newProcessTree(cmd *exec.Cmd) (processTree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return nil, errors.Join(err, windows.CloseHandle(job))
	}
	// No user code can spawn a child before assignment to our job. Do not allow
	// breakaway. If containment cannot be established, terminate while suspended.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	return &windowsTree{job: job}, nil
}

func (t *windowsTree) started(p *os.Process) error {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		return fmt.Errorf("open suspended process: %w", err)
	}
	defer windows.CloseHandle(process)
	if err = windows.AssignProcessToJobObject(t.job, process); err != nil {
		return fmt.Errorf("assign suspended process to job: %w", err)
	}
	t.assigned = true
	// os/exec closes CreateProcess's primary-thread handle. The suspended process
	// has not executed user code, so its one initial thread is found by owner PID.
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("find suspended primary thread: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	var primary uint32
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != uint32(p.Pid) {
			continue
		}
		if primary != 0 {
			return errors.New("suspended primary thread identity is ambiguous")
		}
		primary = entry.ThreadID
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) || primary == 0 {
		return errors.New("suspended primary thread was not observed completely")
	}
	thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, primary)
	if err != nil {
		return fmt.Errorf("open suspended primary thread: %w", err)
	}
	previous, resumeErr := windows.ResumeThread(thread)
	closeErr := windows.CloseHandle(thread)
	if resumeErr != nil || closeErr != nil {
		return errors.Join(resumeErr, closeErr)
	}
	if previous != 1 {
		return errors.New("unexpected primary-thread suspension state")
	}
	return nil
}

func (t *windowsTree) kill(p *os.Process) error {
	if t.assigned {
		return windows.TerminateJobObject(t.job, 1)
	}
	// Assignment failed, so this process never resumed and cannot have children.
	if err := p.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

// Native JOBOBJECT_BASIC_ACCOUNTING_INFORMATION layout:
// https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_accounting_information
type jobAccounting struct {
	TotalUser, TotalKernel, PeriodUser, PeriodKernel int64
	PageFaults, Total, Active, Terminated            uint32
}

func (t *windowsTree) empty(*os.Process) (bool, error) {
	if !t.assigned {
		return true, nil // called only after Wait reaped the never-resumed child
	}
	var info jobAccounting
	if err := windows.QueryInformationJobObject(t.job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil); err != nil {
		return false, err
	}
	return info.Active == 0, nil
}
func (t *windowsTree) close() error { return windows.CloseHandle(t.job) }
