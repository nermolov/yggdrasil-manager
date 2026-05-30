//go:build wasmmem

package harness

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// SampleVmHWM reads /proc/<pid>/status and returns the VmHWM value in KiB.
// Returns 0 if the file cannot be read (process may have exited).
func SampleVmHWM(pid int) int64 {
	path := fmt.Sprintf("/proc/%d/status", pid)
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "VmHWM:") {
			continue
		}
		// Format: "VmHWM:   12345 kB"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		v, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return v
	}
	return 0
}

// PollVmHWM polls SampleVmHWM every interval until ctx is done, returning the peak.
func PollVmHWM(ctx context.Context, pid int, interval time.Duration) int64 {
	var peak int64
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// One final sample before returning.
			if v := SampleVmHWM(pid); v > peak {
				peak = v
			}
			return peak
		case <-ticker.C:
			if v := SampleVmHWM(pid); v > peak {
				peak = v
			}
		}
	}
}
