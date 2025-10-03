package ibc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"cosmossdk.io/math"

	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	"github.com/cosmos/interchaintest/v10"
	"github.com/cosmos/interchaintest/v10/chain/cosmos"
	"github.com/cosmos/interchaintest/v10/ibc"
	"github.com/cosmos/interchaintest/v10/testreporter"
	"github.com/cosmos/interchaintest/v10/testutil"
)

const (
	CommitTimeout  = 1 * time.Second
	VotingPeriod   = "10s"
	TrustingPeriod = "60s"
	ExpiryWaitTime = 65 * time.Second
	VotingWaitTime = 15 * time.Second
)

func DefaultConfigToml() testutil.Toml {
	configToml := make(testutil.Toml)
	consensusToml := make(testutil.Toml)
	consensusToml["timeout_commit"] = CommitTimeout
	configToml["consensus"] = consensusToml
	return configToml
}

// Query the status of an IBC client using Cosmos SDK gRPC
func IBCClientStatus(ctx context.Context, chain *cosmos.CosmosChain, clientID string) (string, error) {
	grpcConn := chain.GetNode().GrpcConn
	if grpcConn == nil {
		return "", fmt.Errorf("failed to get gRPC connection: connection is nil")
	}

	clientQuery := clienttypes.NewQueryClient(grpcConn)
	resp, err := clientQuery.ClientStatus(ctx, &clienttypes.QueryClientStatusRequest{
		ClientId: clientID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to query client status: %w", err)
	}
	return resp.Status, nil
}

func TriggerClientExpiry(t *testing.T, ctx context.Context, eRep ibc.RelayerExecReporter, r ibc.Relayer) error {
	err := r.StopRelayer(ctx, eRep)
	require.NoError(t, err)

	// wait for trusting period to expire
	time.Sleep(ExpiryWaitTime)
	err = r.StartRelayer(ctx, eRep)
	require.NoError(t, err)

	return nil
}

func RecoverClient(t *testing.T, ctx context.Context, chain *cosmos.CosmosChain, eRep ibc.RelayerExecReporter, r ibc.Relayer, oldClientID string, newClientID string, user ibc.Wallet) error {

	status, err := IBCClientStatus(ctx, chain, newClientID)
	require.NoError(t, err)
	require.Equal(t, "Active", status)

	authority, err := chain.GetGovernanceAddress(ctx)
	require.NoError(t, err)

	recoverMessage := fmt.Sprintf(`{
  	    "@type": "/ibc.core.client.v1.MsgRecoverClient",
	    "subject_client_id": "07-tendermint-0",
	    "substitute_client_id": "07-tendermint-1",
	    "signer": "%s"
	}`, authority)

	// Submit proposal
	prop, err := chain.BuildProposal(nil, "Client Recovery Proposal", "Test Proposal", "ipfs://CID", "1000000000uatom", user.FormattedAddress(), false)
	require.NoError(t, err)
	prop.Messages = []json.RawMessage{json.RawMessage(recoverMessage)}
	result, err := chain.SubmitProposal(ctx, user.FormattedAddress(), prop)
	require.NoError(t, err)
	proposalId := result.ProposalID
	propID, err := strconv.ParseInt(proposalId, 10, 64)
	if err != nil {
		return err
	}
	// Pass proposal
	chain.VoteOnProposalAllValidators(ctx, uint64(propID), cosmos.ProposalVoteYes)
	require.NoError(t, err)
	time.Sleep(VotingWaitTime)

	status, err = IBCClientStatus(ctx, chain, oldClientID)
	require.NoError(t, err)
	require.Equal(t, "Active", status)

	return nil
}

// This tests an IBC client recovery after it expires.
func TestClientRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	DefaultGenesis := []cosmos.GenesisKV{
		// feemarket: set params and starting state
		cosmos.NewGenesisKV("app_state.feemarket.params.min_base_gas_price", "0.005"),
		cosmos.NewGenesisKV("app_state.feemarket.params.max_block_utilization", "50000000"),
		cosmos.NewGenesisKV("app_state.feemarket.state.base_gas_price", "0.005"),
		cosmos.NewGenesisKV("app_state.gov.params.voting_period", VotingPeriod),
	}

	// Chain Factory
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:    "gaia",
			Version: "v25.1.0",
			ChainConfig: ibc.ChainConfig{
				GasPrices:     "0.005uatom",
				ModifyGenesis: cosmos.ModifyGenesis(DefaultGenesis),
				Denom:         "uatom",
				ConfigFileOverrides: map[string]any{
					"config/config.toml": DefaultConfigToml(),
				},
			}},
		{
			Name:    "gaia",
			Version: "v25.1.0",
			ChainConfig: ibc.ChainConfig{
				GasPrices:     "0.005uatom",
				ModifyGenesis: cosmos.ModifyGenesis(DefaultGenesis),
				Denom:         "uatom",
				ConfigFileOverrides: map[string]any{
					"config/config.toml": DefaultConfigToml(),
				},
			}},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	gaia1, gaia2 := chains[0], chains[1]

	// Relayer Factory
	client, network := interchaintest.DockerSetup(t)
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.Hermes,
		zaptest.NewLogger(t),
	).Build(
		t, client, network)

	// Prep Interchain
	const ibcPath = "gaia-gaia-demo"
	clientOpts := ibc.CreateClientOptions{
		TrustingPeriod: TrustingPeriod,
	}
	ic := interchaintest.NewInterchain().
		AddChain(gaia1).
		AddChain(gaia2).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:           gaia1,
			Chain2:           gaia2,
			Relayer:          r,
			Path:             ibcPath,
			CreateClientOpts: clientOpts,
		})

	// Log location
	f, err := interchaintest.CreateLogFile(fmt.Sprintf("%d.json", time.Now().Unix()))
	require.NoError(t, err)
	// Reporter/logs
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: false,
	},
	),
	)

	// Create and Fund User Wallets
	fundAmount := math.NewInt(10_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "cosmos", fundAmount, gaia1, gaia2)
	gaia1User := users[0]
	gaia2User := users[1]

	{

		gaia1UserBalInitial, err := gaia1.GetBalance(ctx, gaia1User.FormattedAddress(), gaia1.Config().Denom)
		require.NoError(t, err)
		require.True(t, gaia1UserBalInitial.Equal(fundAmount))

		// Get Channel ID
		gaia1ChannelInfo, err := r.GetChannels(ctx, eRep, gaia1.Config().ChainID)
		require.NoError(t, err)
		gaia1ChannelID := gaia1ChannelInfo[0].ChannelID

		gaia2ChannelInfo, err := r.GetChannels(ctx, eRep, gaia2.Config().ChainID)
		require.NoError(t, err)
		gaia2ChannelID := gaia2ChannelInfo[0].ChannelID

		height, err := gaia2.Height(ctx)
		require.NoError(t, err)

		// Send Transaction
		amountToSend := math.NewInt(1_000_000)
		dstAddress := gaia2User.FormattedAddress()
		transfer := ibc.WalletAmount{
			Address: dstAddress,
			Denom:   gaia1.Config().Denom,
			Amount:  amountToSend,
		}
		tx, err := gaia1.SendIBCTransfer(ctx, gaia1ChannelID, gaia1User.KeyName(), transfer, ibc.TransferOptions{})
		require.NoError(t, err)
		require.NoError(t, tx.Validate())

		// relay MsgRecvPacket to gaia2, then MsgAcknowledgement back to gaia
		require.NoError(t, r.Flush(ctx, eRep, ibcPath, gaia1ChannelID))

		// test source wallet has decreased funds
		expectedBal := gaia1UserBalInitial.Sub(amountToSend)
		gaia1UserBalNew, err := gaia1.GetBalance(ctx, gaia1User.FormattedAddress(), gaia1.Config().Denom)
		require.NoError(t, err)
		require.True(t, gaia1UserBalNew.LTE(expectedBal))

		// Trace IBC Denom
		srcDenomTrace := transfertypes.NewDenom(gaia1.Config().Denom, transfertypes.NewHop("transfer", gaia2ChannelID))
		dstIbcDenom := srcDenomTrace.IBCDenom()

		// Test destination wallet has increased funds
		gaia2UserBalNew, err := gaia2.GetBalance(ctx, gaia2User.FormattedAddress(), dstIbcDenom)
		require.NoError(t, err)
		require.True(t, gaia2UserBalNew.Equal(amountToSend))

		// Validate light client
		chain := gaia2.(*cosmos.CosmosChain)
		reg := chain.Config().EncodingConfig.InterfaceRegistry
		msg, err := cosmos.PollForMessage[*clienttypes.MsgUpdateClient](ctx, chain, reg, height, height+10, nil)
		require.NoError(t, err)

		require.Equal(t, "07-tendermint-0", msg.ClientId)
		require.NotEmpty(t, msg.Signer)

	}

	// Make IBC clients expire
	err = TriggerClientExpiry(t, ctx, eRep, r)
	require.NoError(t, err)

	res, err := r.GetClients(ctx, eRep, gaia1.Config().ChainID)
	require.NoError(t, err)
	clientID1 := res[0].ClientID

	res, err = r.GetClients(ctx, eRep, gaia2.Config().ChainID)
	require.NoError(t, err)
	clientID2 := res[0].ClientID

	status, err := IBCClientStatus(ctx, gaia1.(*cosmos.CosmosChain), clientID1)
	require.NoError(t, err)
	require.Equal(t, "Expired", status)

	status, err = IBCClientStatus(ctx, gaia2.(*cosmos.CosmosChain), clientID2)
	require.NoError(t, err)
	require.Equal(t, "Expired", status)

	// Create substitute clients
	err = r.CreateClients(ctx, eRep, ibcPath, ibc.CreateClientOptions{
		TrustingPeriod: TrustingPeriod,
	})
	require.NoError(t, err)

	res, err = r.GetClients(ctx, eRep, gaia1.Config().ChainID)
	require.NoError(t, err)
	newClientID1 := res[len(res)-1].ClientID
	res, err = r.GetClients(ctx, eRep, gaia2.Config().ChainID)
	require.NoError(t, err)
	newClientID2 := res[len(res)-1].ClientID

	// Recover clients via governance proposal
	err = RecoverClient(t, ctx, gaia1.(*cosmos.CosmosChain), eRep, r, clientID1, newClientID1, gaia1User)
	require.NoError(t, err)

	err = RecoverClient(t, ctx, gaia2.(*cosmos.CosmosChain), eRep, r, clientID2, newClientID2, gaia2User)
	require.NoError(t, err)

	{
		gaia1UserBalInitial, err := gaia1.GetBalance(ctx, gaia1User.FormattedAddress(), gaia1.Config().Denom)
		require.NoError(t, err)

		// Get Channel ID
		gaia1ChannelInfo, err := r.GetChannels(ctx, eRep, gaia1.Config().ChainID)
		require.NoError(t, err)
		gaia1ChannelID := gaia1ChannelInfo[0].ChannelID

		gaia2ChannelInfo, err := r.GetChannels(ctx, eRep, gaia2.Config().ChainID)
		require.NoError(t, err)
		gaia2ChannelID := gaia2ChannelInfo[0].ChannelID

		// Trace IBC Denom
		srcDenomTrace := transfertypes.NewDenom(gaia1.Config().Denom, transfertypes.NewHop("transfer", gaia2ChannelID))
		dstIbcDenom := srcDenomTrace.IBCDenom()

		gaia2UserBalOld, err := gaia2.GetBalance(ctx, gaia2User.FormattedAddress(), dstIbcDenom)
		require.NoError(t, err)

		height, err := gaia2.Height(ctx)
		require.NoError(t, err)

		// Send Transaction
		amountToSend := math.NewInt(1_000_000)
		dstAddress := gaia2User.FormattedAddress()
		transfer := ibc.WalletAmount{
			Address: dstAddress,
			Denom:   gaia1.Config().Denom,
			Amount:  amountToSend,
		}
		tx, err := gaia1.SendIBCTransfer(ctx, gaia1ChannelID, gaia1User.KeyName(), transfer, ibc.TransferOptions{})
		require.NoError(t, err)
		require.NoError(t, tx.Validate())

		// relay MsgRecvPacket to gaia2, then MsgAcknowledgement back to gaia
		require.NoError(t, r.Flush(ctx, eRep, ibcPath, gaia1ChannelID))

		// test source wallet has decreased funds
		expectedBal := gaia1UserBalInitial.Sub(amountToSend)
		gaia1UserBalNew, err := gaia1.GetBalance(ctx, gaia1User.FormattedAddress(), gaia1.Config().Denom)
		require.NoError(t, err)
		require.True(t, gaia1UserBalNew.LTE(expectedBal))

		// Test destination wallet has increased funds
		gaia2UserBalNew, err := gaia2.GetBalance(ctx, gaia2User.FormattedAddress(), dstIbcDenom)
		require.NoError(t, err)
		expectedBalance := gaia2UserBalOld.Add(amountToSend)
		require.True(t, gaia2UserBalNew.Equal(expectedBalance))

		// Validate light client
		chain := gaia2.(*cosmos.CosmosChain)
		reg := chain.Config().EncodingConfig.InterfaceRegistry
		msg, err := cosmos.PollForMessage[*clienttypes.MsgUpdateClient](ctx, chain, reg, height, height+10, nil)
		require.NoError(t, err)

		require.Equal(t, "07-tendermint-0", msg.ClientId)
		require.NotEmpty(t, msg.Signer)

	}

}
