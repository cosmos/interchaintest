package thorchain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/icza/dyno"

	"github.com/cosmos/interchaintest/v10/ibc"
)

type GenesisKV struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

func NewGenesisKV(key string, value interface{}) GenesisKV {
	return GenesisKV{
		Key:   key,
		Value: value,
	}
}

func ModifyGenesis(genesisKV []GenesisKV) func(ibc.ChainConfig, []byte) ([]byte, error) {
	return func(chainConfig ibc.ChainConfig, genbz []byte) ([]byte, error) {
		g := make(map[string]interface{})
		if err := json.Unmarshal(genbz, &g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis file: %w", err)
		}

		for idx, values := range genesisKV {
			splitPath := strings.Split(values.Key, ".")

			path := make([]interface{}, len(splitPath))
			for i, component := range splitPath {
				if v, err := strconv.Atoi(component); err == nil {
					path[i] = v
				} else {
					path[i] = component
				}
			}

			if err := dyno.Set(g, values.Value, path...); err != nil {
				return nil, fmt.Errorf("failed to set key '%s' as '%+v' (index:%d) in genesis json: %w", values.Key, values.Value, idx, err)
			}
		}

		out, err := json.Marshal(g)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
		}
		return out, nil
	}
}
