package firewall

import (
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/nermolov/yggdrasil-manager/src/firewall/testutils"
	probing "github.com/prometheus-community/pro-bing"
	"github.com/stretchr/testify/assert"
	"github.com/yggdrasil-network/yggdrasil-go/src/admin"
)

type Node struct {
	Namespace   string
	IPV6Address *string
}

// TestFirewallPacketDropping ensures that ping packets are dropped according to the firewall rules.
// Relies on `run-four-fully-connected-nodes.sh` to set up 4 nodes in network namespaces.
func TestFirewallPacketDropping(t *testing.T) {
	nodes := []Node{
		{Namespace: "node1"},
		{Namespace: "node2"},
		{Namespace: "node3"},
		{Namespace: "node4"},
	}

	// get the IPv6 addresses of the nodes
	t.Log("Getting IPv6 addresses of nodes")

	for i := range nodes {
		testutils.SetNetworkNamespace(nodes[i].Namespace)

		ipAddress, err := getYggdrasilIPAddress("127.0.0.1:9001")
		if err != nil {
			panic(err)
		}

		nodes[i].IPV6Address = &ipAddress

		t.Log(ipAddress)
	}

	// for each node, ping every other node
	t.Log("Pinging between nodes to test firewall rules")

	for i := range nodes {
		for j := range nodes {
			if i == j {
				continue
			}

			testutils.SetNetworkNamespace(nodes[i].Namespace)

			pinger, err := probing.NewPinger(*nodes[j].IPV6Address)

			if err != nil {
				t.Errorf("Failed to create pinger from %s to %s: %v", nodes[i].Namespace, nodes[j].Namespace, err)
				continue
			}

			pinger.SetPrivileged(true)
			pinger.Count = 1

			t.Logf("Pinging from %s (%s) to %s (%s)", nodes[i].Namespace, *nodes[i].IPV6Address, nodes[j].Namespace, *nodes[j].IPV6Address)

			err = pinger.Run()
			if err != nil {
				t.Errorf("Could not run pinger from %s to %s: %v", nodes[i].Namespace, nodes[j].Namespace, err)
			}

			stats := pinger.Statistics()
			t.Logf("Ping statistics from %s to %s: %+v", nodes[i].Namespace, nodes[j].Namespace, stats)

			assert.Equal(t, pinger.Count, stats.PacketsRecv, "All packets should be received from %s to %s", nodes[i].Namespace, nodes[j].Namespace)
		}
	}
}

func getYggdrasilIPAddress(endpoint string) (string, error) {
	// Connect to the TCP socket
	conn, err := net.Dial("tcp", endpoint)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	// Set up JSON encoder/decoder
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// Prepare the request
	send := &admin.AdminSocketRequest{
		Name: "getself",
	}
	args := map[string]string{}
	if send.Arguments, err = json.Marshal(args); err != nil {
		return "", err
	}

	// Send the request
	if err := encoder.Encode(&send); err != nil {
		return "", err
	}

	// Receive the response
	recv := &admin.AdminSocketResponse{}
	if err := decoder.Decode(&recv); err != nil {
		return "", err
	}

	// Check for errors
	if recv.Status == "error" {
		if recv.Error != "" {
			return "", errors.New(recv.Error)
		}
		return "", errors.New("admin socket returned an error")
	}

	// Parse the response
	var resp admin.GetSelfResponse
	if err := json.Unmarshal(recv.Response, &resp); err != nil {
		return "", err
	}

	return resp.IPAddress, nil
}
