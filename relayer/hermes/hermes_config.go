package hermes

import (
	"fmt"
	"strconv"
	"strings"
)

// NewConfig returns a hermes Config with an entry for each of the provided ChainConfigs.
// The defaults were adapted from the sample config file found here: https://github.com/informalsystems/hermes/blob/master/config.toml
func NewConfig(chainConfigs ...ChainConfig) Config {
	return NewConfigWithPkTypes(nil, chainConfigs...)
}

// NewConfigWithPkTypes returns a hermes Config with an entry for each of the provided ChainConfigs,
// allowing custom PkType configuration per chain.
func NewConfigWithPkTypes(chainPkTypes map[string]string, chainConfigs ...ChainConfig) Config {
	var chains []Chain
	for _, hermesCfg := range chainConfigs {
		chainCfg := hermesCfg.cfg

		gasPricesStr, err := strconv.ParseFloat(strings.ReplaceAll(chainCfg.GasPrices, chainCfg.Denom, ""), 32)
		if err != nil {
			panic(err)
		}

		var chainType string
		var accountPrefix string
		var trustingPeriod string
		chainType = Cosmos
		accountPrefix = chainCfg.Bech32Prefix
		trustingPeriod = "14days"

		// Configure address type
		var addressType AddressType

		// Priority: custom PkType > eth_secp256k1 default > cosmos default
		customPkType := chainPkTypes[chainCfg.ChainID]

		switch {
		case customPkType != "":
			addressType = AddressType{
				Derivation: "ethermint",
				ProtoType: &ProtoType{
					PkType: customPkType,
				},
			}
		case chainCfg.SigningAlgorithm == "eth_secp256k1":
			addressType = AddressType{
				Derivation: "ethermint",
				ProtoType: &ProtoType{
					PkType: "/cosmos.evm.crypto.v1.ethsecp256k1.PubKey",
				},
			}
		default:
			addressType = AddressType{
				Derivation: "cosmos",
				ProtoType:  nil,
			}
		}

		chains = append(chains, Chain{
			ID:               chainCfg.ChainID,
			Type:             chainType,
			RPCAddr:          hermesCfg.rpcAddr,
			CCVConsumerChain: false,
			GrpcAddr:         fmt.Sprintf("http://%s", hermesCfg.grpcAddr),
			EventSource: EventSource{
				Mode:     "push",
				URL:      strings.ReplaceAll(fmt.Sprintf("%s/websocket", hermesCfg.rpcAddr), "http", "ws"),
				Interval: "100ms",
			},
			RPCTimeout:    "10s",
			TrustedNode:   true,
			AccountPrefix: accountPrefix,
			KeyName:       hermesCfg.keyName,
			KeyStoreType:  "Test",
			AddressType:   addressType,
			StorePrefix:   "ibc",
			DefaultGas:    200000,
			MaxGas:        400000,
			GasPrice: GasPrice{
				Price: gasPricesStr,
				Denom: chainCfg.Denom,
			},
			GasMultiplier:         chainCfg.GasAdjustment,
			MaxMsgNum:             30,
			MaxTxSize:             180000,
			MaxGrpcDecodingSize:   33554432,
			QueryPacketsChunkSize: 50,
			ClockDrift:            "5s",
			MaxBlockTime:          "30s",
			ClientRefreshRate:     "1/3",
			TrustingPeriod:        trustingPeriod,
			TrustThreshold: TrustThreshold{
				Numerator:   "1",
				Denominator: "3",
			},
			SequentialBatchTx: false,
			MemoPrefix:        "hermes",
		},
		)
	}

	return Config{
		Global: Global{
			LogLevel: "debug",
		},
		Mode: Mode{
			Clients: Clients{
				Enabled:      true,
				Refresh:      true,
				Misbehaviour: false,
			},
			Connections: Connections{
				Enabled: true,
			},
			Channels: Channels{
				Enabled: true,
			},
			Packets: Packets{
				Enabled:        true,
				ClearInterval:  100,
				ClearOnStart:   true,
				TxConfirmation: true,
			},
		},
		Rest: Rest{
			Enabled: false,
		},
		Telemetry: Telemetry{
			Enabled: false,
		},
		TracingServer: TracingServer{
			Enabled: false,
		},
		Chains: chains,
	}
}

// Chain type for Hermes, currently only `CosmosSdk` or `Namada` is supported.
const (
	Cosmos = "CosmosSdk"
	Namada = "Namada"
)

type Config struct {
	Global        Global        `toml:"global"`
	Mode          Mode          `toml:"mode"`
	Rest          Rest          `toml:"rest"`
	Telemetry     Telemetry     `toml:"telemetry"`
	TracingServer TracingServer `toml:"tracing_server"`
	Chains        []Chain       `toml:"chains"`
}

type Global struct {
	LogLevel string `toml:"log_level"`
}

type Clients struct {
	Enabled      bool `toml:"enabled"`
	Refresh      bool `toml:"refresh"`
	Misbehaviour bool `toml:"misbehaviour"`
}

type Connections struct {
	Enabled bool `toml:"enabled"`
}

type Channels struct {
	Enabled bool `toml:"enabled"`
}

type Packets struct {
	Enabled                       bool `toml:"enabled"`
	ClearInterval                 int  `toml:"clear_interval"`
	ClearOnStart                  bool `toml:"clear_on_start"`
	TxConfirmation                bool `toml:"tx_confirmation"`
	AutoRegisterCounterpartyPayee bool `toml:"auto_register_counterparty_payee"`
}

type Mode struct {
	Clients     Clients     `toml:"clients"`
	Connections Connections `toml:"connections"`
	Channels    Channels    `toml:"channels"`
	Packets     Packets     `toml:"packets"`
}

type Rest struct {
	Enabled bool   `toml:"enabled"`
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
}

type Telemetry struct {
	Enabled bool   `toml:"enabled"`
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
}

type TracingServer struct {
	Enabled bool `toml:"enabled"`
	Port    int  `toml:"port"`
}

type EventSource struct {
	Mode       string `toml:"mode"`
	URL        string `toml:"url,omitempty"`
	Interval   string `toml:"interval,omitempty"`
	BatchDelay string `toml:"batch_delay,omitempty"`
}

// AddressType represents the address_type configuration
// go-toml/v2 will automatically serialize this as an inline table.
type AddressType struct {
	Derivation string     `toml:"derivation"`
	ProtoType  *ProtoType `toml:"proto_type,omitempty,inline"`
}

type ProtoType struct {
	PkType string `toml:"pk_type"`
}

type GasPrice struct {
	Price float64 `toml:"price"`
	Denom string  `toml:"denom"`
}

type TrustThreshold struct {
	Numerator   string `toml:"numerator"`
	Denominator string `toml:"denominator"`
}

type Chain struct {
	ID                    string         `toml:"id"`
	Type                  string         `toml:"type"`
	RPCAddr               string         `toml:"rpc_addr"`
	GrpcAddr              string         `toml:"grpc_addr"`
	EventSource           EventSource    `toml:"event_source"`
	CCVConsumerChain      bool           `toml:"ccv_consumer_chain"`
	RPCTimeout            string         `toml:"rpc_timeout"`
	TrustedNode           bool           `toml:"trusted_node"`
	AccountPrefix         string         `toml:"account_prefix"`
	KeyName               string         `toml:"key_name"`
	KeyStoreType          string         `toml:"key_store_type"`
	AddressType           AddressType    `toml:"address_type,inline"` // Will be serialized as inline table
	StorePrefix           string         `toml:"store_prefix"`
	DefaultGas            int            `toml:"default_gas"`
	MaxGas                int            `toml:"max_gas"`
	GasPrice              GasPrice       `toml:"gas_price"`
	GasMultiplier         float64        `toml:"gas_multiplier"`
	MaxMsgNum             int            `toml:"max_msg_num"`
	MaxTxSize             int            `toml:"max_tx_size"`
	MaxGrpcDecodingSize   int            `toml:"max_grpc_decoding_size,omitempty"`
	QueryPacketsChunkSize int            `toml:"query_packets_chunk_size,omitempty"`
	ClockDrift            string         `toml:"clock_drift"`
	MaxBlockTime          string         `toml:"max_block_time"`
	ClientRefreshRate     string         `toml:"client_refresh_rate,omitempty"`
	TrustingPeriod        string         `toml:"trusting_period"`
	TrustThreshold        TrustThreshold `toml:"trust_threshold"`
	SequentialBatchTx     bool           `toml:"sequential_batch_tx"`
	MemoPrefix            string         `toml:"memo_prefix,omitempty"`
}
