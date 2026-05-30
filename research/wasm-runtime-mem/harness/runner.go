//go:build wasmmem

package harness

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Result holds the outcome of a single benchmark run.
type Result struct {
	Runtime     string
	Suite       int
	WASITarget  string
	Interpreter bool
	CBound      bool
	PeakRSSKiB  int64 // max(MaxrssKiB, VmHWMKiB)
	MaxrssKiB   int64 // from syscall.Wait4 Rusage.Maxrss
	VmHWMKiB    int64 // sampled from /proc/<pid>/status
	WallMs      int64
	OK          bool
	ErrMsg      string
	Stdout      string // trimmed
}

// MeasurePeakRSS runs bin with args, feeds stdin (may be nil), waits for exit,
// and returns peak memory measurements. It uses its own process group and samples
// /proc/<pid>/status VmHWM every 5ms concurrently with Wait4 Rusage.Maxrss.
//
// An error is returned only for hard failures (e.g. failed to start the process).
// A non-zero exit code sets OK=false and populates ErrMsg.
func MeasurePeakRSS(ctx context.Context, name string, suite int, wasiTarget string, interpreter, cbound bool, bin string, args []string, stdin io.Reader) (Result, error) {
	res := Result{
		Runtime:     name,
		Suite:       suite,
		WASITarget:  wasiTarget,
		Interpreter: interpreter,
		CBound:      cbound,
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf

	if stdin != nil {
		stdinPipe, err := cmd.StdinPipe()
		if err != nil {
			return res, fmt.Errorf("MeasurePeakRSS: StdinPipe: %w", err)
		}
		go func() {
			defer stdinPipe.Close()
			_, _ = io.Copy(stdinPipe, stdin)
		}()
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return res, fmt.Errorf("MeasurePeakRSS: start %q: %w", bin, err)
	}

	pid := cmd.Process.Pid

	// Poll /proc/<pid>/status for VmHWM every 5ms until the process exits.
	pollDone := make(chan int64, 1)
	pollCtx, pollCancel := context.WithCancel(context.Background())
	go func() {
		pollDone <- PollVmHWM(pollCtx, pid, 5*time.Millisecond)
	}()

	// Wait for the process and collect rusage.
	var rusage syscall.Rusage
	var wstatus syscall.WaitStatus
	_, err := syscall.Wait4(pid, &wstatus, 0, &rusage)

	// Stop the VmHWM poller and collect its peak.
	pollCancel()
	vmhwm := <-pollDone

	res.WallMs = time.Since(start).Milliseconds()
	res.VmHWMKiB = vmhwm

	if err != nil {
		return res, fmt.Errorf("MeasurePeakRSS: Wait4 pid %d: %w", pid, err)
	}

	// On Linux, Maxrss is in KiB.
	res.MaxrssKiB = rusage.Maxrss

	if res.MaxrssKiB > res.VmHWMKiB {
		res.PeakRSSKiB = res.MaxrssKiB
	} else {
		res.PeakRSSKiB = res.VmHWMKiB
	}

	res.Stdout = strings.TrimSpace(stdoutBuf.String())

	if wstatus.Exited() && wstatus.ExitStatus() == 0 {
		res.OK = true
	} else {
		res.OK = false
		if wstatus.Exited() {
			res.ErrMsg = fmt.Sprintf("exit status %d", wstatus.ExitStatus())
		} else if wstatus.Signaled() {
			res.ErrMsg = fmt.Sprintf("killed by signal %s", wstatus.Signal())
		} else {
			res.ErrMsg = "process did not exit cleanly"
		}
	}

	return res, nil
}
