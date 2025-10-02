package cosmos_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"cosmossdk.io/math"

	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"

	interchaintest "github.com/cosmos/interchaintest/v10"
	"github.com/cosmos/interchaintest/v10/chain/cosmos"
	"github.com/cosmos/interchaintest/v10/conformance"
	"github.com/cosmos/interchaintest/v10/ibc"
	"github.com/cosmos/interchaintest/v10/relayer"
	"github.com/cosmos/interchaintest/v10/testreporter"
	"github.com/cosmos/interchaintest/v10/testutil"
)

const (
	haltHeightDelta    = 10 // will propose upgrade this many blocks in the future
	blocksAfterUpgrade = 10
	votingPeriod       = "10s"
	maxDepositPeriod   = "10s"
)

func TestJunoUpgradeIBC(t *testing.T) {
	CosmosChainUpgradeIBCTest(t, "juno", "v6.0.0", "ghcr.io/strangelove-ventures/heighliner/juno", "v8.0.0", "multiverse")
}

func TestGaiaUpgradeIBC(t *testing.T) {
	CosmosChainUpgradeIBCTest(t, "gaia", "v24.0.0", "ghcr.io/cosmos/gaia", "v25.1.0", "v25.1.0")
}

func CosmosChainUpgradeIBCTest(t *testing.T, chainName, initialVersion, upgradeContainerRepo, upgradeVersion string, upgradeName string) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	var upgradeChainGenesis []cosmos.GenesisKV
	var gasPrices string
	if chainName == "juno" {
		// SDK v45 params for Juno genesis
		upgradeChainGenesis = []cosmos.GenesisKV{
			cosmos.NewGenesisKV("app_state.gov.voting_params.voting_period", votingPeriod),
			cosmos.NewGenesisKV("app_state.gov.deposit_params.max_deposit_period", maxDepositPeriod),
			cosmos.NewGenesisKV("app_state.gov.deposit_params.min_deposit.0.denom", "ujuno"),
		}
		gasPrices = "0ujuno"
	} else {
		// SDK > v45 params for Gaia genesis
		upgradeChainGenesis = []cosmos.GenesisKV{
			cosmos.NewGenesisKV("app_state.gov.params.voting_period", votingPeriod),
			cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", maxDepositPeriod),
			cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", "uatom"),
			// configure the feemarket module
			cosmos.NewGenesisKV("app_state.feemarket.params.enabled", false),
			cosmos.NewGenesisKV("app_state.feemarket.params.min_base_gas_price", "0.001"),
			cosmos.NewGenesisKV("app_state.feemarket.params.max_block_utilization", "50000000"),
			cosmos.NewGenesisKV("app_state.feemarket.state.base_gas_price", "0.001"),
		}
		gasPrices = "0.001uatom"
	}
	fixedChainGenesis := []cosmos.GenesisKV{
		cosmos.NewGenesisKV("app_state.gov.params.voting_period", votingPeriod),
		cosmos.NewGenesisKV("app_state.gov.params.max_deposit_period", maxDepositPeriod),
		cosmos.NewGenesisKV("app_state.gov.params.min_deposit.0.denom", "uatom"),
		// configure the feemarket module
		cosmos.NewGenesisKV("app_state.feemarket.params.enabled", false),
		cosmos.NewGenesisKV("app_state.feemarket.params.min_base_gas_price", "0.001"),
		cosmos.NewGenesisKV("app_state.feemarket.params.max_block_utilization", "50000000"),
		cosmos.NewGenesisKV("app_state.feemarket.state.base_gas_price", "0.001"),
	}
	fixedChainGasPrices := "0.001uatom"

	chains := interchaintest.CreateChainsWithChainSpecs(t, []*interchaintest.ChainSpec{
		{
			Name:      chainName,
			ChainName: chainName,
			Version:   initialVersion,
			ChainConfig: ibc.ChainConfig{
				GasPrices:     gasPrices,
				ModifyGenesis: cosmos.ModifyGenesis(upgradeChainGenesis),
			},
			NumValidators: &numValsOne,
			NumFullNodes:  &numFullNodesZero,
		},
		{
			Name:      "gaia",
			ChainName: "gaia-ibc",
			Version:   "v25.1.0",
			ChainConfig: ibc.ChainConfig{
				GasPrices:     fixedChainGasPrices,
				ModifyGenesis: cosmos.ModifyGenesis(fixedChainGenesis),
			},
			NumValidators: &numValsOne,
			NumFullNodes:  &numFullNodesZero,
		},
	})

	client, network := interchaintest.DockerSetup(t)

	chain, counterpartyChain := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	const (
		path        = "ibc-upgrade-test-path"
		relayerName = "relayer"
	)

	// Get a relayer instance
	rf := interchaintest.NewBuiltinRelayerFactory(
		ibc.Hermes,
		zaptest.NewLogger(t),
		relayer.StartupFlags("-b", "100"),
	)

	r := rf.Build(t, client, network)

	ic := interchaintest.NewInterchain().
		AddChain(chain).
		AddChain(counterpartyChain).
		AddRelayer(r, relayerName).
		AddLink(interchaintest.InterchainLink{
			Chain1:  chain,
			Chain2:  counterpartyChain,
			Relayer: r,
			Path:    path,
		})

	ctx := context.Background()

	rep := testreporter.NewNopReporter()

	require.NoError(t, ic.Build(ctx, rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation: false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	userFunds := math.NewInt(10_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain)
	chainUser := users[0]

	// test IBC conformance before chain upgrade
	conformance.TestChainPair(t, ctx, client, network, chain, counterpartyChain, rf, rep, r, path)

	height, err := chain.Height(ctx)
	require.NoError(t, err, "error fetching height before submit upgrade proposal")

	haltHeight := height + haltHeightDelta

	proposal := cosmos.SoftwareUpgradeProposal{
		Deposit:     "500000000" + chain.Config().Denom, // greater than min deposit
		Title:       "Chain Upgrade 1",
		Name:        upgradeName,
		Description: "First chain software upgrade",
		Height:      haltHeight,
	}

	upgradeTx, err := chain.UpgradeProposal(ctx, chainUser.KeyName(), proposal)
	require.NoError(t, err, "error submitting software upgrade proposal tx")

	propId, err := strconv.ParseUint(upgradeTx.ProposalID, 10, 64)
	require.NoError(t, err, "failed to convert proposal ID to uint64")

	err = chain.VoteOnProposalAllValidators(ctx, propId, cosmos.ProposalVoteYes)
	require.NoError(t, err, "failed to submit votes")

	_, err = cosmos.PollForProposalStatus(ctx, chain, height, height+haltHeightDelta, propId, govv1beta1.StatusPassed)
	require.NoError(t, err, "proposal status did not change to passed in expected number of blocks")

	height, err = chain.Height(ctx)
	require.NoError(t, err, "error fetching height before upgrade")

	timeoutCtx, timeoutCtxCancel := context.WithTimeout(ctx, time.Second*45)
	defer timeoutCtxCancel()

	// this should timeout due to chain halt at upgrade height.
	_ = testutil.WaitForBlocks(timeoutCtx, int(haltHeight-height)+1, chain)

	height, err = chain.Height(ctx)
	require.NoError(t, err, "error fetching height after chain should have halted")

	// make sure that chain is halted
	require.Equal(t, haltHeight, height, "height is not equal to halt height")

	// bring down nodes to prepare for upgrade
	err = chain.StopAllNodes(ctx)
	require.NoError(t, err, "error stopping node(s)")

	// upgrade version on all nodes
	chain.UpgradeVersion(ctx, client, upgradeContainerRepo, upgradeVersion)

	// start all nodes back up.
	// validators reach consensus on first block after upgrade height
	// and chain block production resumes.
	err = chain.StartAllNodes(ctx)
	require.NoError(t, err, "error starting upgraded node(s)")

	timeoutCtx, timeoutCtxCancel = context.WithTimeout(ctx, time.Second*45)
	defer timeoutCtxCancel()

	err = testutil.WaitForBlocks(timeoutCtx, int(blocksAfterUpgrade), chain)
	require.NoError(t, err, "chain did not produce blocks after upgrade")

	// test IBC conformance after chain upgrade on same path
	conformance.TestChainPair(t, ctx, client, network, chain, counterpartyChain, rf, rep, r, path)
}
