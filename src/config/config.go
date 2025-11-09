package config

import (
	"errors"

	"github.com/hjson/hjson-go/v4"
)

type ManagerConfig struct {
	Manager managerConfigOptions `comment:"yggdrasil-manager specific configuration options."`
}

type managerConfigOptions struct {
	FilterAllowedPublicKeys []string `comment:"List of peer public keys to allow ipv6 traffic to/from on the tunnel. Traffic can still be routed for nodes not included in this list."`
}

func (mcfg *ManagerConfig) UnmarshalHJSON(data []byte) error {
	if err := hjson.Unmarshal(data, mcfg); err != nil {
		return err
	}
	return mcfg.postprocessConfig()
}

func (mcfg *ManagerConfig) postprocessConfig() error {
	if len(mcfg.Manager.FilterAllowedPublicKeys) == 0 {
		return errors.New("Manager.FilterAllowedPublicKeys is a required field")
	}
	return nil
}
