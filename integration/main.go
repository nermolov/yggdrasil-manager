package integration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"testing"

	"golang.org/x/sys/unix"
)

func setNetworkNamespace(nsName string) {
	nsPath := "/run/netns/" + nsName

	// Lock the OS thread to ensure consistent namespace operations
	runtime.LockOSThread()

	// Open the network namespace file
	fd, err := unix.Open(nsPath, unix.O_RDONLY, 0)
	if err != nil {
		panic(fmt.Errorf("failed to open netns: %v", err))
	}
	defer unix.Close(fd)

	// Set the network namespace
	err = unix.Setns(fd, unix.CLONE_NEWNET)
	if err != nil {
		panic(fmt.Errorf("failed to set netns: %v", err))
	}
}

func runYggdrasilNode(t *testing.T, namespace string, config map[string]any) {
	cmd := exec.CommandContext(t.Context(), "ip", "netns", "exec", namespace, "../yggdrasil", "-useconf")

	// log stdout/stderr with prefix
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}
	go func(ns string, rdr io.Reader) {
		scanner := bufio.NewScanner(rdr)
		for scanner.Scan() {
			t.Logf("[%s] %s", ns, scanner.Text())
		}
	}(namespace, stdout)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("Failed to get stderr pipe: %v", err)
	}

	go func(ns string, rdr io.Reader) {
		scanner := bufio.NewScanner(rdr)
		for scanner.Scan() {
			t.Logf("[%s] STDERR: %s", ns, scanner.Text())
		}
	}(namespace, stderr)

	// write config
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}
	go func() {
		defer stdin.Close()

		bs, err := json.Marshal(config)
		if err != nil {
			panic(fmt.Errorf("failed to marshal config: %v", err))
		}
		_, err = stdin.Write(bs)
		if err != nil {
			panic(fmt.Errorf("failed to write config to stdin: %v", err))
		}
	}()

	err = cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start node %s: %v", namespace, err)
	}
}
