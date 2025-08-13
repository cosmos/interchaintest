package ethereum_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/cosmos/interchaintest/v10"
	"github.com/cosmos/interchaintest/v10/chain/ethereum"
	"github.com/cosmos/interchaintest/v10/chain/ethereum/geth"
	"github.com/cosmos/interchaintest/v10/ibc"
	"github.com/cosmos/interchaintest/v10/testreporter"
)

func TestGeth(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)

	// Log location
	f, err := interchaintest.CreateLogFile(fmt.Sprintf("%d.json", time.Now().Unix()))
	require.NoError(t, err)
	// Reporter/logs
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	// Get default ethereum chain config for geth
	gethConfig := geth.DefaultEthereumGethChainConfig("ethereum")

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{ChainConfig: gethConfig},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	ethereumChain := chains[0].(*geth.GethChain)

	ic := interchaintest.NewInterchain().
		AddChain(ethereumChain)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true, // Skip path creation, so we can have granular control over the process
	}))
	t.Logf("Interchain built")

	// Create and fund a user using GetAndFundTestUsers
	ethUserInitialAmount := ethereum.ETHER.MulRaw(1000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", ethUserInitialAmount, ethereumChain)
	ethUser := users[0]

	// Check balances of user
	balance, err := ethereumChain.GetBalance(ctx, ethUser.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, balance.Equal(ethUserInitialAmount))

	ethUser2, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, "user2", strings.Repeat("dog ", 23)+"fossil", ethUserInitialAmount, ethereumChain)
	require.NoError(t, err)

	balance, err = ethereumChain.GetBalance(ctx, ethUser2.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, balance.Equal(ethUserInitialAmount))

	txHash, err := ethereumChain.SendFundsWithNote(ctx, ethUser2.KeyName(), ibc.WalletAmount{
		Address: ethUser.FormattedAddress(),
		Amount:  ethUserInitialAmount.QuoRaw(10),
		Denom:   ethereumChain.Config().Denom,
	}, "memo")
	require.NoError(t, err)
	require.NotEmpty(t, txHash)

	balance, err = ethereumChain.GetBalance(ctx, ethUser.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, balance.Equal(ethUserInitialAmount.Add(ethUserInitialAmount.QuoRaw(10))))
}

type ContractOutput struct {
	TxHash string `json:"transactionHash"`
}
