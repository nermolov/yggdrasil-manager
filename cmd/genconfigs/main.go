package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	mconfig "github.com/nermolov/yggdrasil-manager/src/config"
	"github.com/yggdrasil-network/yggdrasil-go/src/config"
)

const PROTOCOL = "quic"

type configInput struct {
	Name                string `comment:"Unique name for the node, resulting config file will be named <name>.json"`
	PrivateKey          config.KeyBytes
	Listen              *configInputListen                `comment:"If set, the node will listen for incoming connections according to the provided options, and other nodes will be configured to connect to it"`
	MulticastInterfaces []config.MulticastInterfaceConfig `comment:"Multicast interface configurations for the node. If empty, the default platform-specific multicast configuration will be used."`
}

type configInputListen struct {
	Port       int    `comment:"Port to listen on"`
	PublicHost string `comment:"Public hostname or IP address that other nodes will use to connect to this node"`
	PublicPort int    `comment:"Public port that other nodes will use to connect to this node"`
}

type configOutput struct {
	*config.NodeConfig
	*mconfig.ManagerConfig
	// re-specify to set omitempty on marshal, allowing multicast auto-config at runtime
	MulticastInterfaces []config.MulticastInterfaceConfig `json:",omitempty"`
}

func main() {
	inputFile := flag.String("input", "", "config input json file path")
	outputDir := flag.String("output", "", "config output directory path")
	flag.Parse()
	if *inputFile == "" || *outputDir == "" {
		panic("input config file path and output directory path are required")
	}

	// read config input
	f, err := os.Open(*inputFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var inputConfigs []configInput
	if err := json.NewDecoder(f).Decode(&inputConfigs); err != nil {
		panic(err)
	}

	publicPeers := selectPublicPeers()

	for _, n := range inputConfigs {
		configOutput := configOutput{}
		configOutput.NodeConfig = config.GenerateConfig()
		configOutput.ManagerConfig = &mconfig.ManagerConfig{}

		// set private key
		configOutput.NodeConfig.PrivateKey = n.PrivateKey
		// if listener, configure listening
		if n.Listen != nil {
			configOutput.Listen = []string{fmt.Sprintf("%v://0.0.0.0:%v", PROTOCOL, n.Listen.Port)}
		} else { // else connect to all other listening nodes
			for _, on := range inputConfigs {
				if on.Listen != nil {
					configOutput.Peers = append(configOutput.Peers, fmt.Sprintf("%v://%v:%v", PROTOCOL, on.Listen.PublicHost, on.Listen.PublicPort))
				}
			}
		}
		// add public peers
		for _, peer := range publicPeers {
			configOutput.Peers = append(configOutput.Peers, peer)
		}
		// whitelist peers and connections
		for _, on := range inputConfigs {
			if n.Name == on.Name {
				continue
			}
			privateKey := ed25519.PrivateKey(on.PrivateKey)
			publicKey := privateKey.Public().(ed25519.PublicKey)

			configOutput.Manager.FilterAllowedPublicKeys = append(configOutput.Manager.FilterAllowedPublicKeys, hex.EncodeToString(publicKey))
			configOutput.NodeConfig.AllowedPublicKeys = configOutput.Manager.FilterAllowedPublicKeys
		}
		// set multicast interfaces
		configOutput.MulticastInterfaces = n.MulticastInterfaces

		// write config file to directory
		output, err := json.MarshalIndent(configOutput, "", "  ")
		if err != nil {
			panic(fmt.Sprintf("failed to marshal config output: %v\n", err))
		}
		outputPath := fmt.Sprintf("%s/%s.json", *outputDir, n.Name)
		if err := os.WriteFile(outputPath, output, 0644); err != nil {
			panic(fmt.Sprintf("failed to write config file %s: %v\n", outputPath, err))
		}
	}

	fmt.Printf("Configs written to %v\n", *outputDir)
}
