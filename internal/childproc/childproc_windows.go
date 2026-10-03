//go:build windows

// Package childproc ties child processes to skillshare's lifetime on Windows,
// where terminating a process does not reach the processes it started.
package childproc

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// jobEnv carries the job handle and the parent PID to a copy started by ShareJob.
const jobEnv = "SKILLSHARE_CHILDPROC_JOB"

// x/sys/windows does not wrap IsProcessInJob.
var procIsProcessInJob = windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")

var (
	// job is the kill-on-close job set up by BindToProcess, or 0.
	job   windows.Handle
	jobMu sync.Mutex
)

// BindToProcess puts this process in a kill-on-close job, so its children
// (git, native CLIs) stop with it even when it is terminated. Children that
// must outlive it start through StartDetached. It does nothing inside a job
// that forbids breakaway, such as Task Scheduler's, because StartDetached could
// not work there; it reports whether the job was set up.
//
// The job allows breakaway only while StartDetached runs: the MSYS runtime
// behind Git for Windows' sh.exe breaks every child it starts away from a job
// that allows it, which would let hooks and ssh commands outlive skillshare.
func BindToProcess() bool {
	if adoptJob() {
		return true
	}
	if !breakawayAllowed() {
		return false
	}
	h, ok := assign(windows.CurrentProcess(), windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE)
	if ok {
		job = h
	}
	return ok
}

// ShareJob passes the job set up by BindToProcess to cmd, a copy of skillshare
// that runs in this process's place (the hidden-console relaunch), so the copy
// can start detached processes too. Call it before cmd.Start.
func ShareJob(cmd *exec.Cmd) {
	if job == 0 {
		return
	}
	self := windows.CurrentProcess()
	var h windows.Handle
	if err := windows.DuplicateHandle(self, job, self, &h, 0, true, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.AdditionalInheritedHandles = append(cmd.SysProcAttr.AdditionalInheritedHandles, syscall.Handle(h))
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%d:%d", jobEnv, h, os.Getpid()))
}

// adoptJob takes over a job passed by ShareJob. Holding its handle keeps the
// job open after the parent is gone, so it ends the job itself then: the parent
// waits for this process, so ending first means it was terminated.
func adoptJob() bool {
	v := os.Getenv(jobEnv)
	if v == "" {
		return false
	}
	os.Unsetenv(jobEnv)
	var h uintptr
	var ppid uint32
	if _, err := fmt.Sscanf(v, "%d:%d", &h, &ppid); err != nil {
		return false
	}
	parent, err := windows.OpenProcess(windows.SYNCHRONIZE, false, ppid)
	if err != nil {
		windows.CloseHandle(windows.Handle(h))
		return false
	}
	job = windows.Handle(h)
	go func() {
		windows.WaitForSingleObject(parent, windows.INFINITE)
		windows.TerminateJobObject(job, 1)
	}()
	return true
}

// BindChild puts one started child in a kill-on-close job held by this process,
// so terminating this process stops the child. The child's own children leave
// the job silently. It is the fallback for when BindToProcess was not possible.
func BindChild(pid int) {
	proc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(proc)
	assign(proc, windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE|windows.JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK)
}

// StartDetached starts a process that must outlive skillshare (a background
// server, a browser, an editor) outside the job set up by BindToProcess.
func StartDetached(cmd *exec.Cmd) error {
	if job == 0 {
		return cmd.Start()
	}
	jobMu.Lock()
	defer jobMu.Unlock()
	if !setLimits(job, windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE|windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK) {
		return cmd.Start()
	}
	defer setLimits(job, windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE)
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_BREAKAWAY_FROM_JOB
	return cmd.Start()
}

// breakawayAllowed reports whether children may leave the current job: true
// outside any job, or when the innermost job sets JOB_OBJECT_LIMIT_BREAKAWAY_OK.
func breakawayAllowed() bool {
	var inJob int32
	if r, _, _ := procIsProcessInJob.Call(uintptr(windows.CurrentProcess()), 0, uintptr(unsafe.Pointer(&inJob))); r == 0 {
		return false
	}
	if inJob == 0 {
		return true
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	if err := windows.QueryInformationJobObject(0, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil); err != nil {
		return false
	}
	return info.BasicLimitInformation.LimitFlags&windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK != 0
}

// assign creates a job with the given limits and puts proc in it. The job handle
// stays open for the rest of this process's life; closing it applies the limits.
func assign(proc windows.Handle, limits uint32) (windows.Handle, bool) {
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, false
	}
	if !setLimits(h, limits) {
		windows.CloseHandle(h)
		return 0, false
	}
	if err := windows.AssignProcessToJobObject(h, proc); err != nil {
		windows.CloseHandle(h)
		return 0, false
	}
	return h, true
}

func setLimits(h windows.Handle, limits uint32) bool {
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = limits
	_, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	return err == nil
}
