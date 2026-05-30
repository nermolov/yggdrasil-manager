package integration

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	code := m.Run()
	if buildTmpDir != "" {
		os.RemoveAll(buildTmpDir)
	}
	os.Exit(code)
}

func TestHelloWasm(t *testing.T) {
	runWasm(t, buildWasm(t, "hello-wasm"))
}

func TestTCPSelfConnect(t *testing.T) {
	runWasm(t, buildWasm(t, "tcp-self-connect"))
}
