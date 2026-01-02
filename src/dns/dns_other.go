//go:build !darwin

// Package dns provides platform-specific DNS management.
// This is the no-op variant for unsupported platforms.
package dns

import (
	"github.com/gologme/log"
	"github.com/yggdrasil-network/yggdrasil-go/src/core"
)

type DnsManager struct {
}

func New(core *core.Core, logger *log.Logger) *DnsManager {
	return &DnsManager{}
}

func (dns *DnsManager) ForceResolution() {
	// noop
}

func (dns *DnsManager) Cleanup() {
	// noop
}
