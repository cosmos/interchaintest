package cosmos

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/cosmos/interchaintest/v10/ibc"
)

// TokenFactoryCreateDenom creates a new tokenfactory token in the format 'factory/accountaddress/name'.
// This token will be viewable by standard bank balance queries and send functionality.
// Depending on the chain parameters, this may require a lot of gas (Juno, Osmosis) if the DenomCreationGasConsume param is enabled.
// If not, the default implementation cost 10,000,000 micro tokens (utoken) of the chain's native token.
func (tn *ChainNode) TokenFactoryCreateDenom(ctx context.Context, user ibc.Wallet, denomName string, gas uint64) (string, string, error) {
	cmd := []string{"tokenfactory", "create-denom", denomName}

	if gas != 0 {
		cmd = append(cmd, "--gas", strconv.FormatUint(gas, 10))
	}

	txHash, err := tn.ExecTx(ctx, user.KeyName(), cmd...)
	if err != nil {
		return "", "", err
	}

	return "factory/" + user.FormattedAddress() + "/" + denomName, txHash, nil
}

// TokenFactoryBurnDenom burns a tokenfactory denomination from the holders account.
func (tn *ChainNode) TokenFactoryBurnDenom(ctx context.Context, keyName, fullDenom string, amount uint64) (string, error) {
	coin := strconv.FormatUint(amount, 10) + fullDenom
	return tn.ExecTx(ctx, keyName,
		"tokenfactory", "burn", coin,
	)
}

// TokenFactoryBurnDenomFrom burns a tokenfactory denomination from any other users account.
// Only the admin of the token can perform this action.
func (tn *ChainNode) TokenFactoryBurnDenomFrom(ctx context.Context, keyName, fullDenom string, amount uint64, fromAddr string) (string, error) {
	return tn.ExecTx(ctx, keyName,
		"tokenfactory", "burn-from", fromAddr, convertToCoin(amount, fullDenom),
	)
}

// TokenFactoryChangeAdmin moves the admin of a tokenfactory token to a new address.
func (tn *ChainNode) TokenFactoryChangeAdmin(ctx context.Context, keyName, fullDenom, newAdmin string) (string, error) {
	return tn.ExecTx(ctx, keyName,
		"tokenfactory", "change-admin", fullDenom, newAdmin,
	)
}

// TokenFactoryForceTransferDenom force moves a token from 1 account to another.
// Only the admin of the token can perform this action.
func (tn *ChainNode) TokenFactoryForceTransferDenom(ctx context.Context, keyName, fullDenom string, amount uint64, fromAddr, toAddr string) (string, error) {
	return tn.ExecTx(ctx, keyName,
		"tokenfactory", "force-transfer", convertToCoin(amount, fullDenom), fromAddr, toAddr,
	)
}

// TokenFactoryMintDenom mints a tokenfactory denomination to the admins account.
// Only the admin of the token can perform this action.
func (tn *ChainNode) TokenFactoryMintDenom(ctx context.Context, keyName, fullDenom string, amount uint64) (string, error) {
	return tn.ExecTx(ctx, keyName,
		"tokenfactory", "mint", convertToCoin(amount, fullDenom),
	)
}

// TokenFactoryMintDenomTo mints a token to any external account.
// Only the admin of the token can perform this action.
func (tn *ChainNode) TokenFactoryMintDenomTo(ctx context.Context, keyName, fullDenom string, amount uint64, toAddr string) (string, error) {
	return tn.ExecTx(ctx, keyName,
		"tokenfactory", "mint-to", toAddr, convertToCoin(amount, fullDenom),
	)
}

// TokenFactoryMetadata sets the x/bank metadata for a tokenfactory token. This gives the token more detailed information to be queried
// by frontend UIs and other applications.
// Only the admin of the token can perform this action.
func (tn *ChainNode) TokenFactoryMetadata(ctx context.Context, keyName, fullDenom, ticker, description string, exponent uint64) (string, error) {
	return tn.ExecTx(ctx, keyName,
		"tokenfactory", "modify-metadata", fullDenom, ticker, description, strconv.FormatUint(exponent, 10),
	)
}

// TokenFactoryQueryAdmin returns the admin of a tokenfactory token.
func (c *CosmosChain) TokenFactoryQueryAdmin(ctx context.Context, fullDenom string) (*QueryDenomAuthorityMetadataResponse, error) {
	res := &QueryDenomAuthorityMetadataResponse{}
	stdout, stderr, err := c.GetFullNode().ExecQuery(ctx, "tokenfactory", "denom-authority-metadata", fullDenom)
	if err != nil {
		return nil, fmt.Errorf("failed to query tokenfactory denom-authority-metadata: %w\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	if err := json.Unmarshal(stdout, res); err != nil {
		return nil, err
	}

	return res, nil
}

// Deprecated: use TokenFactoryQueryAdmin instead.
func TokenFactoryGetAdmin(c *CosmosChain, ctx context.Context, fullDenom string) (*QueryDenomAuthorityMetadataResponse, error) {
	return c.TokenFactoryQueryAdmin(ctx, fullDenom)
}

func convertToCoin(amount uint64, denom string) string {
	return strconv.FormatUint(amount, 10) + denom
}
