package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
)

func (tn *ChainNode) GetKeyInConsumerChain(ctx context.Context, consumer *CosmosChain) (string, error) {
	valConsBz, _, err := tn.ExecBin(ctx, "tendermint", "show-address")
	if err != nil {
		return "", err
	}
	valCons := strings.TrimSpace(string(valConsBz))
	consumerID, err := tn.GetConsumerChainByChainID(ctx, consumer.Config().ChainID)
	if err != nil {
		return "", err
	}

	stdout, _, err := tn.ExecQuery(ctx, "provider", "validator-consumer-key", consumerID, valCons)
	if err != nil {
		return "", err
	}
	key := gjson.GetBytes(stdout, "consumer_address").String()
	if key == "" {
		parsed, err := sdk.ConsAddressFromBech32(valCons)
		if err != nil {
			return "", err
		}
		key, err = bech32.ConvertAndEncode(consumer.Config().Bech32Prefix+"valcons", parsed)
		if err != nil {
			return "", err
		}
	}

	return key, nil
}

func (tn *ChainNode) GetConsumerChainByChainID(ctx context.Context, chainID string) (string, error) {
	if tn.HasCommand(ctx, "tx", "provider", "create-consumer") {
		chains, err := tn.ListConsumerChains(ctx)
		if err != nil {
			return "", err
		}
		for _, chain := range chains.Chains {
			if chain.ChainID == chainID {
				return chain.ConsumerID, nil
			}
		}
		return "", fmt.Errorf("chain %s not found", chainID)
	} else {
		return chainID, nil
	}
}

func (tn *ChainNode) ListConsumerChains(ctx context.Context) (ListConsumerChainsResponse, error) {
	queryRes, _, err := tn.ExecQuery(
		ctx,
		"provider", "list-consumer-chains",
	)
	if err != nil {
		return ListConsumerChainsResponse{}, err
	}

	var queryResponse ListConsumerChainsResponse
	err = json.Unmarshal(queryRes, &queryResponse)
	if err != nil {
		return ListConsumerChainsResponse{}, err
	}

	return queryResponse, nil
}

func (tn *ChainNode) GetConsumerChainSpawnTime(ctx context.Context, chainID string) (time.Time, error) {
	if tn.HasCommand(ctx, "tx", "provider", "create-consumer") {
		consumerID, err := tn.GetConsumerChainByChainID(ctx, chainID)
		if err != nil {
			return time.Time{}, err
		}
		consumerChain, _, err := tn.ExecQuery(ctx, "provider", "consumer-chain", consumerID)
		if err != nil {
			return time.Time{}, err
		}
		spawnTime := gjson.GetBytes(consumerChain, "init_params.spawn_time").Time()
		return spawnTime, nil
	} else {
		proposals, _, err := tn.ExecQuery(ctx, "gov", "proposals")
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to query proposed chains: %w", err)
		}
		spawnTime := gjson.GetBytes(proposals, fmt.Sprintf("proposals.#(messages.0.content.chain_id==%q).messages.0.content.spawn_time", chainID)).Time()
		return spawnTime, nil
	}
}

type ListConsumerChainsResponse struct {
	Chains     []ConsumerChain `json:"chains"`
	Pagination Pagination      `json:"pagination"`
}

type ConsumerChain struct {
	ChainID            string   `json:"chain_id"`
	ClientID           string   `json:"client_id"`
	TopN               int      `json:"top_N"`
	MinPowerInTopN     string   `json:"min_power_in_top_N"`
	ValidatorsPowerCap int      `json:"validators_power_cap"`
	ValidatorSetCap    int      `json:"validator_set_cap"`
	Allowlist          []string `json:"allowlist"`
	Denylist           []string `json:"denylist"`
	Phase              string   `json:"phase"`
	Metadata           Metadata `json:"metadata"`
	MinStake           string   `json:"min_stake"`
	AllowInactiveVals  bool     `json:"allow_inactive_vals"`
	ConsumerID         string   `json:"consumer_id"`
}

type Pagination struct {
	NextKey interface{} `json:"next_key"`
	Total   string      `json:"total"`
}

type Metadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Metadata    string `json:"metadata"`
}
