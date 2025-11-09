package filter

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"

	"github.com/yggdrasil-network/yggdrasil-go/src/address"
)

type Filter struct {
	allowedAddresses []*address.Address
}

func NewFilter(allowedKeys []string) (*Filter, error) {
	f := new(Filter)

	f.allowedAddresses = make([]*address.Address, len(allowedKeys))
	for i, hexKey := range allowedKeys {
		keyBytes, err := hex.DecodeString(hexKey)
		if err != nil {
			panic(fmt.Errorf("invalid allowed public key hex: %w", err))
		}
		f.allowedAddresses[i] = address.AddrForKey(ed25519.PublicKey(keyBytes))
	}

	return f, nil
}

func (f *Filter) IsAllowed(ipAddr *address.Address) bool {
	allowed := false
	for _, a := range f.allowedAddresses {
		if *ipAddr == *a {
			allowed = true
			break
		}
	}
	return allowed
}
