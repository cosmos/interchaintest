package cosmos_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/interchaintest/v10"
	"github.com/cosmos/interchaintest/v10/ibc"
	"github.com/cosmos/interchaintest/v10/relayer"
	"github.com/cosmos/interchaintest/v10/testreporter"
	"github.com/cosmos/interchaintest/v10/testutil"
)

func TestBlockDBMessagesView(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)

	const chainID0 = "c0"
	const chainID1 = "c1"
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{Name: testutil.TestSimd, Version: testutil.SimdVersion, ChainConfig: ibc.ChainConfig{ChainID: chainID0}, NumValidators: &numVals, NumFullNodes: &numFullNodes},
		{Name: testutil.TestSimd, Version: testutil.SimdVersion, ChainConfig: ibc.ChainConfig{ChainID: chainID1}, NumValidators: &numVals, NumFullNodes: &numFullNodes},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	gaia0, gaia1 := chains[0], chains[1]

	rf := interchaintest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t))
	r := rf.Build(t, client, network)

	ic := interchaintest.NewInterchain().
		AddChain(gaia0).
		AddChain(gaia1).
		AddRelayer(r, "r").
		AddLink(interchaintest.InterchainLink{
			Chain1:  gaia0,
			Chain2:  gaia1,
			Relayer: r,
		})

	dbDir := interchaintest.TempDir(t)
	dbPath := filepath.Join(dbDir, "blocks.db")

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()
	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: true,

		BlockDatabaseFile: dbPath,
	}))

	// The database should exist on disk,
	// but no transactions should have happened yet.
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Copy the busy timeout from the migration.
	// The journal_mode pragma should be persisted on disk, so we should not need to set that here.
	_, err = db.Exec(`PRAGMA busy_timeout = 3000`)
	require.NoError(t, err)

	var count int
	row := db.QueryRow(`SELECT COUNT(*) FROM v_cosmos_messages`)
	require.NoError(t, row.Scan(&count))
	require.Equal(t, 0, count)

	// Generate the path.
	// No transactions happen here.
	const pathName = "p"
	require.NoError(t, r.GeneratePath(ctx, eRep, chainID0, chainID1, pathName))

	t.Run("create clients", func(t *testing.T) {
		// Creating the clients will cause transactions.
		require.NoError(t, r.CreateClients(ctx, eRep, pathName, ibc.DefaultClientOpts()))

		// MsgCreateClient should match the opposite chain IDs.
		const qCreateClient = `SELECT
client_chain_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.client.v1.MsgCreateClient" AND chain_id = ?;`
		var clientChainID string
		require.NoError(t, db.QueryRow(qCreateClient, chainID0).Scan(&clientChainID))
		require.Equal(t, chainID1, clientChainID)

		require.NoError(t, db.QueryRow(qCreateClient, chainID1).Scan(&clientChainID))
		require.Equal(t, chainID0, clientChainID)
	})
	if t.Failed() {
		return
	}

	var gaia0ClientID, gaia0ConnID, gaia1ClientID, gaia1ConnID string
	t.Run("create connections", func(t *testing.T) {
		// The client isn't created immediately -- wait for two blocks to ensure the clients are ready.
		require.NoError(t, testutil.WaitForBlocks(ctx, 2, gaia0, gaia1))

		// Next, create the connections.
		require.NoError(t, r.CreateConnections(ctx, eRep, pathName))

		// Wait for another block before retrieving the connections and querying for them.
		require.NoError(t, testutil.WaitForBlocks(ctx, 1, gaia0, gaia1))

		conns, err := r.GetConnections(ctx, eRep, chainID0)
		require.NoError(t, err)

		// Collect the reported client IDs.
		gaia0ConnID = conns[0].ID
		gaia0ClientID = conns[0].ClientID
		gaia1ConnID = conns[0].Counterparty.ConnectionId
		gaia1ClientID = conns[0].Counterparty.ClientId

		// OpenInit happens on first chain.
		const qConnectionOpenInit = `SELECT
client_id, counterparty_client_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.connection.v1.MsgConnectionOpenInit" AND chain_id = ?
`
		var clientID, counterpartyClientID string
		require.NoError(t, db.QueryRow(qConnectionOpenInit, chainID0).Scan(&clientID, &counterpartyClientID))
		require.Equal(t, clientID, gaia0ClientID)
		require.Equal(t, counterpartyClientID, gaia1ClientID)

		// OpenTry happens on second chain.
		const qConnectionOpenTry = `SELECT
counterparty_client_id, counterparty_conn_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.connection.v1.MsgConnectionOpenTry" AND chain_id = ?
`
		var counterpartyConnID string
		require.NoError(t, db.QueryRow(qConnectionOpenTry, chainID1).Scan(&counterpartyClientID, &counterpartyConnID))
		require.Equal(t, counterpartyClientID, gaia0ClientID)
		require.Equal(t, counterpartyConnID, gaia0ConnID)

		// OpenAck happens on first chain again.
		const qConnectionOpenAck = `SELECT
conn_id, counterparty_conn_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.connection.v1.MsgConnectionOpenAck" AND chain_id = ?
`
		var connID string
		require.NoError(t, db.QueryRow(qConnectionOpenAck, chainID0).Scan(&connID, &counterpartyConnID))
		require.Equal(t, connID, gaia0ConnID)
		require.Equal(t, counterpartyConnID, gaia1ConnID)

		// OpenConfirm happens on second chain again.
		const qConnectionOpenConfirm = `SELECT
conn_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.connection.v1.MsgConnectionOpenConfirm" AND chain_id = ?
`
		require.NoError(t, db.QueryRow(qConnectionOpenConfirm, chainID1).Scan(&connID))
		require.Equal(t, connID, gaia0ConnID) // Not sure if this should be connection 0 or 1, as they are typically equal during this test.
	})
	if t.Failed() {
		return
	}

	const gaia0Port, gaia1Port = "transfer", "transfer" // Would be nice if these could differ.
	var gaia0ChannelID, gaia1ChannelID string
	t.Run("create channel", func(t *testing.T) {
		require.NoError(t, r.CreateChannel(ctx, eRep, pathName, ibc.CreateChannelOptions{
			SourcePortName: gaia0Port,
			DestPortName:   gaia1Port,
			Order:          ibc.Unordered,
			Version:        "ics20-1",
		}))

		// Wait for another block before retrieving the channels and querying for them.
		require.NoError(t, testutil.WaitForBlocks(ctx, 1, gaia0, gaia1))

		channels, err := r.GetChannels(ctx, eRep, chainID0)
		require.NoError(t, err)
		require.Len(t, channels, 1)

		gaia0ChannelID = channels[0].ChannelID
		gaia1ChannelID = channels[0].Counterparty.ChannelID

		// OpenInit happens on first chain.
		const qChannelOpenInit = `SELECT
port_id, counterparty_port_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.channel.v1.MsgChannelOpenInit" AND chain_id = ?
`
		var portID, counterpartyPortID string
		require.NoError(t, db.QueryRow(qChannelOpenInit, chainID0).Scan(&portID, &counterpartyPortID))
		require.Equal(t, gaia0Port, portID)
		require.Equal(t, gaia1Port, counterpartyPortID)

		// OpenTry happens on second chain.
		const qChannelOpenTry = `SELECT
port_id, counterparty_port_id, counterparty_channel_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.channel.v1.MsgChannelOpenTry" AND chain_id = ?
`
		var counterpartyChannelID string
		require.NoError(t, db.QueryRow(qChannelOpenTry, chainID1).Scan(&portID, &counterpartyPortID, &counterpartyChannelID))
		require.Equal(t, gaia1Port, portID)
		require.Equal(t, gaia0Port, counterpartyPortID)
		require.Equal(t, counterpartyChannelID, gaia0ChannelID)

		// OpenAck happens on first chain again.
		const qChannelOpenAck = `SELECT
port_id, channel_id, counterparty_channel_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.channel.v1.MsgChannelOpenAck" AND chain_id = ?
`
		var channelID string
		require.NoError(t, db.QueryRow(qChannelOpenAck, chainID0).Scan(&portID, &channelID, &counterpartyChannelID))
		require.Equal(t, gaia0Port, portID)
		require.Equal(t, channelID, gaia0ChannelID)
		require.Equal(t, counterpartyChannelID, gaia1ChannelID)

		// OpenConfirm happens on second chain again.
		const qChannelOpenConfirm = `SELECT
port_id, channel_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.channel.v1.MsgChannelOpenConfirm" AND chain_id = ?
`
		require.NoError(t, db.QueryRow(qChannelOpenConfirm, chainID1).Scan(&portID, &channelID))
		require.Equal(t, gaia1Port, portID)
		require.Equal(t, channelID, gaia1ChannelID)
	})
	if t.Failed() {
		return
	}

	t.Run("initiate transfer", func(t *testing.T) {
		// Build the faucet address for gaia1, so that gaia0 can send it a transfer.
		g1FaucetAddrBytes, err := gaia1.GetAddress(ctx, interchaintest.FaucetAccountKeyName)
		require.NoError(t, err)
		gaia1FaucetAddr, err := types.Bech32ifyAddressBytes(gaia1.Config().Bech32Prefix, g1FaucetAddrBytes)
		require.NoError(t, err)

		// Send the IBC transfer. Relayer isn't running, so this will just create a MsgTransfer.
		const txAmount = 13579 // Arbitrary amount that is easy to find in logs.
		transfer := ibc.WalletAmount{
			Address: gaia1FaucetAddr,
			Denom:   gaia0.Config().Denom,
			Amount:  math.NewInt(txAmount),
		}
		tx, err := gaia0.SendIBCTransfer(ctx, gaia0ChannelID, interchaintest.FaucetAccountKeyName, transfer, ibc.TransferOptions{})
		require.NoError(t, err)
		require.NoError(t, tx.Validate())

		const qMsgTransfer = `SELECT
port_id, channel_id
FROM v_cosmos_messages
WHERE type = "/ibc.applications.transfer.v1.MsgTransfer" AND chain_id = ?
`
		var portID, channelID string
		require.NoError(t, db.QueryRow(qMsgTransfer, chainID0).Scan(&portID, &channelID))
		require.Equal(t, gaia0Port, portID)
		require.Equal(t, channelID, gaia0ChannelID)
	})
	if t.Failed() {
		return
	}

	if !rf.Capabilities()[relayer.Flush] {
		t.Skip("cannot continue due to missing capability Flush")
	}

	t.Run("relay", func(t *testing.T) {
		require.NoError(t, r.Flush(ctx, eRep, pathName, gaia0ChannelID))
		require.NoError(t, testutil.WaitForBlocks(ctx, 5, gaia0))

		const qMsgRecvPacket = `SELECT
port_id, channel_id, counterparty_port_id, counterparty_channel_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.channel.v1.MsgRecvPacket" AND chain_id = ?
`

		var portID, channelID, counterpartyPortID, counterpartyChannelID string

		require.NoError(t, db.QueryRow(qMsgRecvPacket, chainID1).Scan(&portID, &channelID, &counterpartyPortID, &counterpartyChannelID))

		require.Equal(t, gaia0Port, portID)
		require.Equal(t, channelID, gaia0ChannelID)
		require.Equal(t, gaia1Port, counterpartyPortID)
		require.Equal(t, counterpartyChannelID, gaia1ChannelID)

		const qMsgAck = `SELECT
port_id, channel_id, counterparty_port_id, counterparty_channel_id
FROM v_cosmos_messages
WHERE type = "/ibc.core.channel.v1.MsgAcknowledgement" AND chain_id = ?
`
		require.NoError(t, db.QueryRow(qMsgAck, chainID0).Scan(&portID, &channelID, &counterpartyPortID, &counterpartyChannelID))

		require.Equal(t, gaia0Port, portID)
		require.Equal(t, channelID, gaia0ChannelID)
		require.Equal(t, gaia1Port, counterpartyPortID)
		require.Equal(t, counterpartyChannelID, gaia1ChannelID)
	})
}
