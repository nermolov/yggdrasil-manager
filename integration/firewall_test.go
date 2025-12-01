package integration

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"net"
	"testing"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/stretchr/testify/assert"
	"github.com/yggdrasil-network/yggdrasil-go/src/address"
)

const NODE_COUNT = 4

type Node struct {
	Namespace               string
	IPV6Address             string
	PrivateKey              ed25519.PrivateKey
	PublicKey               ed25519.PublicKey
	FilterAllowedPublicKeys []ed25519.PublicKey
}

// TestFirewallPacketDropping ensures that ping packets are dropped according to the firewall rules.
// Relies on `run-four-fully-connected-nodes.sh` to set up 4 network namespaces.
func TestFirewallPacketDropping(t *testing.T) {
	nodes := []Node{}

	// initial config generation
	for i := range NODE_COUNT {
		pubkey, privkey, err := ed25519.GenerateKey(nil)
		if err != nil {
			panic(err)
		}

		node := Node{
			Namespace:   fmt.Sprintf("node%d", i+1),
			PrivateKey:  privkey,
			PublicKey:   pubkey,
			IPV6Address: net.IP(address.AddrForKey(pubkey)[:]).String(),
		}
		nodes = append(nodes, node)
	}

	// add firewall rules
	for i := range NODE_COUNT {
		allowedKeys := []ed25519.PublicKey{}
		for j := range NODE_COUNT {
			if i == j {
				continue
			}
			// allow even indexed nodes to talk to each other, block odd indexed nodes
			if (i%2 == 0 && j%2 == 0) || (i%2 == 1 && j%2 == 1) {
				allowedKeys = append(allowedKeys, nodes[j].PublicKey)
			}
		}
		nodes[i].FilterAllowedPublicKeys = allowedKeys
	}

	// start each node
	for i := range NODE_COUNT {
		t.Logf("Starting node %s with allowed keys: %d", nodes[i].Namespace, len(nodes[i].FilterAllowedPublicKeys))

		config := map[string]any{
			"AdminListen": "none",
			"PrivateKey":  hex.EncodeToString(nodes[i].PrivateKey),
			"Manager": map[string]any{
				"FilterAllowedPublicKeys": func() []string {
					keysHex := []string{}
					for _, pk := range nodes[i].FilterAllowedPublicKeys {
						keysHex = append(keysHex, hex.EncodeToString(pk))
					}
					return keysHex
				}(),
			},
		}

		runYggdrasilNode(t, nodes[i].Namespace, config)
	}

	// wait for nodes to discover each other
	time.Sleep(3 * time.Second)

	// for each node, ping every other node
	t.Log("Pinging between nodes to test firewall rules")

	for sourceIdx := range nodes {
		for targetIdx := range nodes {
			if sourceIdx == targetIdx {
				continue
			}

			t.Logf("Setting network namespace to %s", nodes[sourceIdx].Namespace)
			setNetworkNamespace(nodes[sourceIdx].Namespace)

			pinger, err := probing.NewPinger(nodes[targetIdx].IPV6Address)

			if err != nil {
				t.Fatalf("Failed to create pinger from %s to %s: %v", nodes[sourceIdx].Namespace, nodes[targetIdx].Namespace, err)
			}

			pinger.SetPrivileged(true)
			pinger.Count = 1
			pinger.Timeout = 10 * time.Millisecond

			t.Logf("Pinging from %s (%s) to %s (%s)", nodes[sourceIdx].Namespace, nodes[sourceIdx].IPV6Address, nodes[targetIdx].Namespace, nodes[targetIdx].IPV6Address)

			err = pinger.Run()
			if err != nil {
				t.Fatalf("Could not run pinger from %s to %s: %v", nodes[sourceIdx].Namespace, nodes[targetIdx].Namespace, err)
			}

			stats := pinger.Statistics()
			t.Logf("Ping statistics from %s to %s: %+v", nodes[sourceIdx].Namespace, nodes[targetIdx].Namespace, stats)

			allowed := false
			for _, pk := range nodes[targetIdx].FilterAllowedPublicKeys {
				if pk.Equal(nodes[sourceIdx].PublicKey) {
					allowed = true
					break
				}
			}
			if allowed {
				assert.Equal(t, pinger.Count, stats.PacketsRecv, "All packets should be received from %s to %s", nodes[sourceIdx].Namespace, nodes[targetIdx].Namespace)
			} else {
				assert.Equal(t, 0, stats.PacketsRecv, "No packets should be received from %s to %s", nodes[sourceIdx].Namespace, nodes[targetIdx].Namespace)
			}
		}
	}
}
