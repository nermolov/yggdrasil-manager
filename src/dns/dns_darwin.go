//go:build darwin

// Package dns provides platform-specific DNS management.
// This darwin variants implements a workaround to force IPv6 DNS resolution
// when the primary network on a macOS device does not support IPv6.
//
// macOS/iOS systems use the SystemConfiguration framework to determine the
// current usable/reachable network states, which also informs what address
// spaces DNS resolutions are returned for.
//
// To reduce usage of "expensive" interfaces when possible (e.g. a cell network that
// provides v6 when a user would prefer their v4-only Wi-Fi connection), the framework's
// "coupling" behavior requires that interfaces provide both v4 and v6 networking to be
// promoted to an active, reachable state. Due to this, the Yggdrasil v6-only network will
// never get promoted when a v4-only higher priority network exists, preventing DNS resolution.
// https://github.com/apple-oss-distributions/configd/blob/86ef71c041c9e08716c57f75e1093157f3078795/Plugins/IPMonitor/ip_plugin.c#L441-L466
//
// We can escape this "coupling" behavior by faking that our service uses a `gifX` tunnel device
// (instead of our `utunX`), which is explicitly opted out.
// https://github.com/apple-oss-distributions/configd/blob/86ef71c041c9e08716c57f75e1093157f3078795/Plugins/IPMonitor/ip_plugin.c#L7516
//
// Alternatively one could add a DNS Entity to your service which provides a DNS server,
// but that server must be reachable via the specified interface device and handle all
// DNS requests for the machine. Tailscale uses this approach, but that requires implementing
// a full DNS forwarder. Sticking with the simpler `gifX` hack until our DNS needs grow.
//
// Learn more:
// https://developer.apple.com/documentation/systemconfiguration
// https://developer.apple.com/library/archive/documentation/Networking/Conceptual/SystemConfigFrameworks/SC_Intro/SC_Intro.html#//apple_ref/doc/uid/TP40001065
package dns

import (
	"fmt"
	"os/exec"
	"strings"
	"text/template"

	"github.com/gologme/log"

	"github.com/google/uuid"
	"github.com/yggdrasil-network/yggdrasil-go/src/core"
)

const scutilUp = `d.init
d.add Addresses * {{.Address}}
d.add InterfaceName gif0
d.add Router {{.Address}}
set State:/Network/Service/{{.ServiceID}}/IPv6`

const scutilDown = `remove State:/Network/Service/{{.ServiceID}}/IPv6`

type DnsManager struct {
	core      *core.Core
	logger    *log.Logger
	serviceId string
}

func New(core *core.Core, logger *log.Logger) *DnsManager {
	return &DnsManager{
		core:      core,
		logger:    logger,
		serviceId: strings.ToUpper(uuid.New().String()),
	}
}

// ForceResolution forces IPv6 DNS records to resolve when the primary network is v4-only
func (dns *DnsManager) ForceResolution() {
	input := map[string]string{
		"ServiceID": dns.serviceId,
		"Address":   dns.core.Address().String(),
	}

	dns.logger.Printf("Forcing DNS resolution with SystemConfiguration network service %v", dns.serviceId)
	err := dns.runScutilCommand(scutilUp, input)
	if err != nil {
		dns.logger.Printf("Error forcing DNS resolution: %v", err)
	}
}

func (dns *DnsManager) Cleanup() {
	input := map[string]string{
		"ServiceID": dns.serviceId,
	}

	dns.logger.Printf("Cleaning up SystemConfiguration network service %v", dns.serviceId)
	err := dns.runScutilCommand(scutilDown, input)
	if err != nil {
		dns.logger.Errorf("Error cleaning up SystemConfiguration network service %v: %v", dns.serviceId, err)
	}
}

// runScutilCommand runs a `scutil` command with the templated stdin
// TODO: Replace with bindings to the SystemConfiguration API
// https://developer.apple.com/documentation/systemconfiguration
func (dns *DnsManager) runScutilCommand(tmplStr string, input map[string]string) error {
	tmpl, err := template.New("scutil").Parse(tmplStr)
	if err != nil {
		return err
	}

	var sb strings.Builder
	err = tmpl.Execute(&sb, input)
	if err != nil {
		return err
	}

	cmd := exec.Command("scutil")

	var stdout strings.Builder
	cmd.Stdout = &stdout

	// provide commands
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer stdin.Close()

		_, err = stdin.Write([]byte(sb.String()))
		if err != nil {
			panic(fmt.Errorf("failed to write stdin to scutil: %v", err))
		}
	}()

	if err := cmd.Run(); err != nil {
		return err
	}
	if stdout.Len() > 0 {
		dns.logger.Errorf("scutil does not accept input, output: %s\n", stdout.String())
	}

	return nil
}
