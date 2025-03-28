package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ory/dockertest/v3/docker"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const (
	flagFrom            = "from"
	flagHome            = "home"
	flagFees            = "fees"
	flagGas             = "gas"
	flagOutput          = "output"
	flagChainID         = "chain-id"
	flagSpendLimit      = "spend-limit"
	flagGasAdjustment   = "gas-adjustment"
	flagFeeGranter      = "fee-granter"
	flagBroadcastMode   = "broadcast-mode"
	flagKeyringBackend  = "keyring-backend"
	flagAllowedMessages = "allowed-messages"
)

type flagOption func(map[string]interface{})

// withKeyValue add a new flag to command

func withKeyValue(key string, value interface{}) flagOption {
	return func(o map[string]interface{}) {
		o[key] = value
	}
}

func applyOptions(chainID string, options []flagOption) map[string]interface{} {
	opts := map[string]interface{}{
		flagKeyringBackend: "test",
		flagOutput:         "json",
		flagGas:            "auto",
		flagFrom:           "alice",
		flagBroadcastMode:  "sync",
		flagGasAdjustment:  "1.5",
		flagChainID:        chainID,
		flagHome:           teritoriHomePath,
		flagFees:           standardFees.String(),
	}
	for _, apply := range options {
		apply(opts)
	}
	return opts
}

func (s *IntegrationTestSuite) execEncode(
	c *chain,
	txPath string,
	opt ...flagOption,
) string {
	opts := applyOptions(c.id, opt)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("%s - Executing teritorid encoding with %v", c.id, txPath)
	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		"encode",
		txPath,
	}
	for flag, value := range opts {
		teritoriCommand = append(teritoriCommand, fmt.Sprintf("--%s=%v", flag, value))
	}

	var encoded string
	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, 0, func(stdOut []byte, stdErr []byte) bool {
		if stdErr != nil {
			return false
		}
		encoded = strings.TrimSuffix(string(stdOut), "\n")
		return true
	})
	s.T().Logf("successfully encode with %v", txPath)
	return encoded
}

func (s *IntegrationTestSuite) execDecode(
	c *chain,
	txPath string,
	opt ...flagOption,
) string {
	opts := applyOptions(c.id, opt)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("%s - Executing teritorid decoding with %v", c.id, txPath)
	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		"decode",
		txPath,
	}
	for flag, value := range opts {
		teritoriCommand = append(teritoriCommand, fmt.Sprintf("--%s=%v", flag, value))
	}

	var decoded string
	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, 0, func(stdOut []byte, stdErr []byte) bool {
		if stdErr != nil {
			return false
		}
		decoded = strings.TrimSuffix(string(stdOut), "\n")
		return true
	})
	s.T().Logf("successfully decode %v", txPath)
	return decoded
}

func (s *IntegrationTestSuite) execVestingTx( //nolint:unused

	c *chain,
	method string,
	args []string,
	opt ...flagOption,
) {
	opts := applyOptions(c.id, opt)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("%s - Executing teritorid %s with %v", c.id, method, args)
	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		vestingtypes.ModuleName,
		method,
		"-y",
	}
	teritoriCommand = append(teritoriCommand, args...)

	for flag, value := range opts {
		teritoriCommand = append(teritoriCommand, fmt.Sprintf("--%s=%v", flag, value))
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, 0, s.defaultExecValidation(c, 0))
	s.T().Logf("successfully %s with %v", method, args)
}

func (s *IntegrationTestSuite) execCreatePeriodicVestingAccount( //nolint:unused

	c *chain,
	address,
	jsonPath string,
	opt ...flagOption,
) {
	s.T().Logf("Executing teritorid create periodic vesting account %s", c.id)
	s.execVestingTx(c, "create-periodic-vesting-account", []string{address, jsonPath}, opt...)
	s.T().Logf("successfully created periodic vesting account %s with %s", address, jsonPath)
}

func (s *IntegrationTestSuite) execUnjail(
	c *chain,
	opt ...flagOption,
) {
	opts := applyOptions(c.id, opt)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("Executing teritorid slashing unjail %s with options: %v", c.id, opt)
	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		slashingtypes.ModuleName,
		"unjail",
		"-y",
	}

	for flag, value := range opts {
		teritoriCommand = append(teritoriCommand, fmt.Sprintf("--%s=%v", flag, value))
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, 0, s.defaultExecValidation(c, 0))
	s.T().Logf("successfully unjail with options %v", opt)
}

func (s *IntegrationTestSuite) execFeeGrant(c *chain, valIdx int, granter, grantee, spendLimit string, opt ...flagOption) {
	opt = append(opt, withKeyValue(flagFrom, granter))
	opt = append(opt, withKeyValue(flagSpendLimit, spendLimit))
	opts := applyOptions(c.id, opt)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("granting %s fee from %s on chain %s", grantee, granter, c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		feegrant.ModuleName,
		"grant",
		granter,
		grantee,
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		fmt.Sprintf("--%s=%s", flags.FlagGas, "300000"), // default 200000 isn't enough
		"--keyring-backend=test",
		"--output=json",
		"-y",
	}
	for flag, value := range opts {
		teritoriCommand = append(teritoriCommand, fmt.Sprintf("--%s=%s", flag, value))
	}
	s.T().Logf("running feegrant on chain: %s - Tx %v", c.id, teritoriCommand)

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
}

func (s *IntegrationTestSuite) execFeeGrantRevoke(c *chain, valIdx int, granter, grantee string, opt ...flagOption) {
	opt = append(opt, withKeyValue(flagFrom, granter))
	opts := applyOptions(c.id, opt)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("revoking %s fee grant from %s on chain %s", grantee, granter, c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		feegrant.ModuleName,
		"revoke",
		granter,
		grantee,
		"-y",
	}
	for flag, value := range opts {
		teritoriCommand = append(teritoriCommand, fmt.Sprintf("--%s=%v", flag, value))
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
}

func (s *IntegrationTestSuite) execBankSend(
	c *chain,
	valIdx int,
	from,
	to,
	amt,
	fees string,
	expectErr bool,
	opt ...flagOption,
) {
	// TODO remove the hardcode opt after refactor, all methods should accept custom flags
	opt = append(opt, withKeyValue(flagFees, fees))
	opt = append(opt, withKeyValue(flagFrom, from))
	opts := applyOptions(c.id, opt)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("sending %s tokens from %s to %s on chain %s", amt, from, to, c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		banktypes.ModuleName,
		"send",
		from,
		to,
		amt,
		"-y",
	}
	for flag, value := range opts {
		teritoriCommand = append(teritoriCommand, fmt.Sprintf("--%s=%v", flag, value))
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.expectErrExecValidation(c, valIdx, expectErr))
}

func (s *IntegrationTestSuite) execBankMultiSend(
	c *chain,
	valIdx int,
	from string,
	to []string,
	amt string,
	fees string,
	expectErr bool,
	opt ...flagOption,
) {
	// TODO remove the hardcode opt after refactor, all methods should accept custom flags
	opt = append(opt, withKeyValue(flagFees, fees))
	opt = append(opt, withKeyValue(flagFrom, from))
	opts := applyOptions(c.id, opt)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("sending %s tokens from %s to %s on chain %s", amt, from, to, c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		banktypes.ModuleName,
		"multi-send",
		from,
	}

	teritoriCommand = append(teritoriCommand, to...)
	teritoriCommand = append(teritoriCommand, amt, "-y")

	for flag, value := range opts {
		teritoriCommand = append(teritoriCommand, fmt.Sprintf("--%s=%v", flag, value))
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.expectErrExecValidation(c, valIdx, expectErr))
}

func (s *IntegrationTestSuite) execDistributionFundCommunityPool(c *chain, valIdx int, from, amt, fees string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("Executing teritorid tx distribution fund-community-pool on chain %s", c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		distributiontypes.ModuleName,
		"fund-community-pool",
		amt,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, from),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		fmt.Sprintf("--%s=%s", flags.FlagFees, fees),
		"--keyring-backend=test",
		"--output=json",
		"-y",
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
	s.T().Logf("Successfully funded community pool")
}

func (s *IntegrationTestSuite) runGovExec(c *chain, valIdx int, submitterAddr, govCommand string, proposalFlags []string, fees string, validationFunc func([]byte, []byte) bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	validateResponse := s.defaultExecValidation(c, valIdx)
	if validationFunc != nil {
		validateResponse = validationFunc
	}

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		govtypes.ModuleName,
		govCommand,
	}

	generalFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagFrom, submitterAddr),
		fmt.Sprintf("--%s=%s", flags.FlagGas, "350000"), // default 200000 isn't enough
		fmt.Sprintf("--%s=%s", flags.FlagGasPrices, fees),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		"--keyring-backend=test",
		"--output=json",
		"-y",
	}

	teritoriCommand = concatFlags(teritoriCommand, proposalFlags, generalFlags)
	s.T().Logf("Executing teritorid tx gov %s on chain %s", govCommand, c.id)
	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, validateResponse)
	s.T().Logf("Successfully executed %s", govCommand)
}

// NOTE: Tx unused, left here for future reference
// func (s *IntegrationTestSuite) executeGKeysAddCommand(c *chain, valIdx int, name string, home string) string {
// 	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
// 	defer cancel()

// 	teritoriCommand := []string{
// 		teritoridBinary,
// 		keysCommand,
// 		"add",
// 		name,
// 		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
// 		"--keyring-backend=test",
// 		"--output=json",
// 	}

// 	var addrRecord AddressResponse
// 	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, func(stdOut []byte, stdErr []byte) bool {
// 		// Teritorid keys add by default returns payload to stdErr
// 		if err := json.Unmarshal(stdErr, &addrRecord); err != nil {
// 			return false
// 		}
// 		return strings.Contains(addrRecord.Address, "tori")
// 	})
// 	return addrRecord.Address
// }

// NOTE: Tx unused, left here for future reference
// func (s *IntegrationTestSuite) executeKeysList(c *chain, valIdx int, home string) { // nolint:U1000
// 	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
// 	defer cancel()

// 	teritoriCommand := []string{
// 		teritoridBinary,
// 		keysCommand,
// 		"list",
// 		"--keyring-backend=test",
// 		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
// 		"--output=json",
// 	}

// 	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, func([]byte, []byte) bool {
// 		return true
// 	})
// }

func (s *IntegrationTestSuite) execDelegate(c *chain, valIdx int, amount, valOperAddress, delegatorAddr, home, delegateFees string) { //nolint:unparam

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("Executing teritorid tx staking delegate %s", c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		stakingtypes.ModuleName,
		"delegate",
		valOperAddress,
		amount,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, delegatorAddr),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		fmt.Sprintf("--%s=%s", flags.FlagGasPrices, delegateFees),
		fmt.Sprintf("--%s=%s", flags.FlagGas, "250000"), // default 200_000 is not enough
		"--keyring-backend=test",
		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
		"--output=json",
		"-y",
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
	s.T().Logf("%s successfully delegated %s to %s", delegatorAddr, amount, valOperAddress)
}

func (s *IntegrationTestSuite) execUnbondDelegation(c *chain, valIdx int, amount, valOperAddress, delegatorAddr, home, delegateFees string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("Executing teritorid tx staking unbond %s", c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		stakingtypes.ModuleName,
		"unbond",
		valOperAddress,
		amount,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, delegatorAddr),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		fmt.Sprintf("--%s=%s", flags.FlagGasPrices, delegateFees),
		"--gas=250000", // default 200_000 is not enough; gas fees are higher when unbonding is done after LSM operations
		"--keyring-backend=test",
		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
		"--output=json",
		"-y",
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
	s.T().Logf("%s successfully undelegated %s to %s", delegatorAddr, amount, valOperAddress)
}

func (s *IntegrationTestSuite) execCancelUnbondingDelegation(c *chain, valIdx int, amount, valOperAddress, creationHeight, delegatorAddr, home, delegateFees string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("Executing teritorid tx staking cancel-unbond %s", c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		stakingtypes.ModuleName,
		"cancel-unbond",
		valOperAddress,
		amount,
		creationHeight,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, delegatorAddr),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		fmt.Sprintf("--%s=%s", flags.FlagGasPrices, delegateFees),
		"--keyring-backend=test",
		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
		"--output=json",
		"-y",
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
	s.T().Logf("%s successfully canceled unbonding %s to %s", delegatorAddr, amount, valOperAddress)
}

func (s *IntegrationTestSuite) execRedelegate(c *chain, valIdx int, amount, originalValOperAddress,
	newValOperAddress, delegatorAddr, home, delegateFees string,
) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("Executing teritorid tx staking redelegate %s", c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		stakingtypes.ModuleName,
		"redelegate",
		originalValOperAddress,
		newValOperAddress,
		amount,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, delegatorAddr),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		fmt.Sprintf("--%s=%s", flags.FlagGas, "350000"), // default 200000 isn't enough
		fmt.Sprintf("--%s=%s", flags.FlagGasPrices, delegateFees),
		"--keyring-backend=test",
		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
		"--output=json",
		"-y",
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
	s.T().Logf("%s successfully redelegated %s from %s to %s", delegatorAddr, amount, originalValOperAddress, newValOperAddress)
}

func (s *IntegrationTestSuite) getLatestBlockHeight(c *chain, valIdx int) int {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	type syncInfo struct {
		SyncInfo struct {
			LatestHeight string `json:"latest_block_height"`
		} `json:"SyncInfo"`
	}

	var currentHeight int
	teritoriCommand := []string{teritoridBinary, "status"}
	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, func(stdOut []byte, stdErr []byte) bool {
		var (
			err   error
			block syncInfo
		)
		s.Require().NoError(json.Unmarshal(stdErr, &block))
		currentHeight, err = strconv.Atoi(block.SyncInfo.LatestHeight)
		s.Require().NoError(err)
		return currentHeight > 0
	})
	return currentHeight
}

// func (s *IntegrationTestSuite) verifyBalanceChange(endpoint string, expectedAmount sdk.Coin, recipientAddress string) {
// 	s.Require().Eventually(
// 		func() bool {
// 			afterAtomBalance, err := getSpecificBalance(endpoint, recipientAddress, utoriDenom)
// 			s.Require().NoError(err)

// 			return afterAtomBalance.IsEqual(expectedAmount)
// 		},
// 		20*time.Second,
// 		5*time.Second,
// 	)
// }

func (s *IntegrationTestSuite) execSetWithdrawAddress(
	c *chain,
	valIdx int,
	fees,
	delegatorAddress,
	newWithdrawalAddress,
	homePath string,
) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("Setting distribution withdrawal address on chain %s for %s to %s", c.id, delegatorAddress, newWithdrawalAddress)
	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		distributiontypes.ModuleName,
		"set-withdraw-addr",
		newWithdrawalAddress,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, delegatorAddress),
		fmt.Sprintf("--%s=%s", flags.FlagFees, fees),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		fmt.Sprintf("--%s=%s", flags.FlagHome, homePath),
		"--keyring-backend=test",
		"--output=json",
		"-y",
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
	s.T().Logf("Successfully set new distribution withdrawal address for %s to %s", delegatorAddress, newWithdrawalAddress)
}

func (s *IntegrationTestSuite) execWithdrawReward(
	c *chain,
	valIdx int,
	delegatorAddress,
	validatorAddress,
	homePath string,
) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("Withdrawing distribution rewards on chain %s for delegator %s from %s validator", c.id, delegatorAddress, validatorAddress)
	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		distributiontypes.ModuleName,
		"withdraw-rewards",
		validatorAddress,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, delegatorAddress),
		fmt.Sprintf("--%s=%s", flags.FlagGasPrices, "300utori"),
		fmt.Sprintf("--%s=%s", flags.FlagGas, "auto"),
		fmt.Sprintf("--%s=%s", flags.FlagGasAdjustment, "1.5"),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		fmt.Sprintf("--%s=%s", flags.FlagHome, homePath),
		"--keyring-backend=test",
		"--output=json",
		"-y",
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
	s.T().Logf("Successfully withdrew distribution rewards for delegator %s from validator %s", delegatorAddress, validatorAddress)
}

func (s *IntegrationTestSuite) executeTeritoriTxCommand(ctx context.Context, c *chain, teritoriCommand []string, valIdx int, validation func([]byte, []byte) bool) {
	if validation == nil {
		validation = s.defaultExecValidation(s.chainA, 0)
	}
	var (
		outBuf bytes.Buffer
		errBuf bytes.Buffer
	)
	exec, err := s.dkrPool.Client.CreateExec(docker.CreateExecOptions{
		Context:      ctx,
		AttachStdout: true,
		AttachStderr: true,
		Container:    s.valResources[c.id][valIdx].Container.ID,
		User:         "root",
		Cmd:          teritoriCommand,
	})
	s.Require().NoError(err)

	err = s.dkrPool.Client.StartExec(exec.ID, docker.StartExecOptions{
		Context:      ctx,
		Detach:       false,
		OutputStream: &outBuf,
		ErrorStream:  &errBuf,
	})
	s.Require().NoError(err)

	stdOut := outBuf.Bytes()
	stdErr := errBuf.Bytes()
	if !validation(stdOut, stdErr) {
		s.Require().FailNowf("Exec validation failed", "stdout: %s, stderr: %s",
			string(stdOut), string(stdErr))
	}
}

func (s *IntegrationTestSuite) executeHermesCommand(ctx context.Context, hermesCmd []string) ([]byte, error) { //nolint:unparam
	var outBuf bytes.Buffer
	exec, err := s.dkrPool.Client.CreateExec(docker.CreateExecOptions{
		Context:      ctx,
		AttachStdout: true,
		AttachStderr: true,
		Container:    s.hermesResource.Container.ID,
		User:         "root",
		Cmd:          hermesCmd,
	})
	s.Require().NoError(err)

	err = s.dkrPool.Client.StartExec(exec.ID, docker.StartExecOptions{
		Context:      ctx,
		Detach:       false,
		OutputStream: &outBuf,
	})
	s.Require().NoError(err)

	// Check that the stdout output contains the expected status
	// and look for errors, e.g "insufficient fees"
	stdOut := []byte{}
	scanner := bufio.NewScanner(&outBuf)
	for scanner.Scan() {
		stdOut = scanner.Bytes()
		var out map[string]interface{}
		err = json.Unmarshal(stdOut, &out)
		s.Require().NoError(err)
		if err != nil {
			return nil, fmt.Errorf("hermes relayer command returned failed with error: %s", err)
		}
		// errors are catched by observing the logs level in the stderr output
		if lvl := out["level"]; lvl != nil && strings.ToLower(lvl.(string)) == "error" {
			errMsg := out["fields"].(map[string]interface{})["message"]
			return nil, fmt.Errorf("hermes relayer command failed: %s", errMsg)
		}
		if s := out["status"]; s != nil && s != "success" {
			return nil, fmt.Errorf("hermes relayer command returned failed with status: %s", s)
		}
	}

	return stdOut, nil
}

func (s *IntegrationTestSuite) expectErrExecValidation(chain *chain, valIdx int, expectErr bool) func([]byte, []byte) bool {
	return func(stdOut []byte, stdErr []byte) bool {
		var txResp sdk.TxResponse
		gotErr := cdc.UnmarshalJSON(stdOut, &txResp) != nil
		if gotErr {
			s.Require().True(expectErr)
		}

		endpoint := fmt.Sprintf("http://%s", s.valResources[chain.id][valIdx].GetHostPort("1317/tcp"))
		// wait for the tx to be committed on chain
		s.Require().Eventuallyf(
			func() bool {
				gotErr := queryTeritoriTx(endpoint, txResp.TxHash) != nil
				return gotErr == expectErr
			},
			time.Minute,
			5*time.Second,
			"stdOut: %s, stdErr: %s",
			string(stdOut), string(stdErr),
		)
		return true
	}
}

func (s *IntegrationTestSuite) defaultExecValidation(chain *chain, valIdx int) func([]byte, []byte) bool {
	return func(stdOut []byte, stdErr []byte) bool {
		var txResp sdk.TxResponse
		if err := cdc.UnmarshalJSON(stdOut, &txResp); err != nil {
			return false
		}
		if strings.Contains(txResp.String(), "code: 0") || txResp.Code == 0 {
			endpoint := fmt.Sprintf("http://%s", s.valResources[chain.id][valIdx].GetHostPort("1317/tcp"))
			s.Require().Eventually(
				func() bool {
					return queryTeritoriTx(endpoint, txResp.TxHash) == nil
				},
				time.Minute,
				5*time.Second,
				"stdOut: %s, stdErr: %s",
				string(stdOut), string(stdErr),
			)
			return true
		}
		return false
	}
}

func (s *IntegrationTestSuite) expectTxSubmitError(expectErrString string) func([]byte, []byte) bool {
	return func(stdOut []byte, stdErr []byte) bool {
		var txResp sdk.TxResponse
		if err := cdc.UnmarshalJSON(stdOut, &txResp); err != nil {
			return false
		}
		if strings.Contains(txResp.RawLog, expectErrString) {
			return true
		}
		return false
	}
}

func (s *IntegrationTestSuite) executeValidatorBond(c *chain, valIdx int, valOperAddress, delegatorAddr, home, delegateFees string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("Executing teritorid tx staking validator-bond %s", c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		stakingtypes.ModuleName,
		"validator-bond",
		valOperAddress,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, delegatorAddr),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		fmt.Sprintf("--%s=%s", flags.FlagGasPrices, delegateFees),
		"--keyring-backend=test",
		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
		"--output=json",
		"-y",
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
	s.T().Logf("%s successfully executed validator bond tx to %s", delegatorAddr, valOperAddress)
}

func (s *IntegrationTestSuite) executeTokenizeShares(c *chain, valIdx int, amount, valOperAddress, delegatorAddr, home, delegateFees string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("Executing teritorid tx staking tokenize-share %s", c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		stakingtypes.ModuleName,
		"tokenize-share",
		valOperAddress,
		amount,
		delegatorAddr,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, delegatorAddr),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		fmt.Sprintf("--%s=%s", flags.FlagGasPrices, delegateFees),
		fmt.Sprintf("--%s=%d", flags.FlagGas, 1000000),
		"--keyring-backend=test",
		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
		"--output=json",
		"-y",
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
	s.T().Logf("%s successfully executed tokenize share tx from %s", delegatorAddr, valOperAddress)
}

func (s *IntegrationTestSuite) executeRedeemShares(c *chain, valIdx int, amount, delegatorAddr, home, delegateFees string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("Executing teritorid tx staking redeem-tokens %s", c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		stakingtypes.ModuleName,
		"redeem-tokens",
		amount,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, delegatorAddr),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		fmt.Sprintf("--%s=%s", flags.FlagGasPrices, delegateFees),
		fmt.Sprintf("--%s=%d", flags.FlagGas, 1000000),
		"--keyring-backend=test",
		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
		"--output=json",
		"-y",
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
	s.T().Logf("%s successfully executed redeem share tx for %s", delegatorAddr, amount)
}

func (s *IntegrationTestSuite) executeTransferTokenizeShareRecord(c *chain, valIdx int, recordID, owner, newOwner, home, txFees string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	s.T().Logf("Executing teritorid tx staking transfer-tokenize-share-record %s", c.id)

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		stakingtypes.ModuleName,
		"transfer-tokenize-share-record",
		recordID,
		newOwner,
		fmt.Sprintf("--%s=%s", flags.FlagFrom, owner),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, c.id),
		fmt.Sprintf("--%s=%s", flags.FlagGasPrices, txFees),
		"--keyring-backend=test",
		fmt.Sprintf("--%s=%s", flags.FlagHome, home),
		"--output=json",
		"-y",
	}

	s.executeTeritoriTxCommand(ctx, c, teritoriCommand, valIdx, s.defaultExecValidation(c, valIdx))
	s.T().Logf("%s successfully executed transfer tokenize share record for %s", owner, recordID)
}

// signTxFileOnline signs a transaction file using the teritoricli tx sign command
// the from flag is used to specify the keyring account to sign the transaction
// the from account must be registered in the keyring and exist on chain (have a balance or be a genesis account)
func (s *IntegrationTestSuite) signTxFileOnline(chain *chain, valIdx int, from string, txFilePath string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	teritoriCommand := []string{
		teritoridBinary,
		txCommand,
		"sign",
		filepath.Join(teritoriHomePath, txFilePath),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, chain.id),
		fmt.Sprintf("--%s=%s", flags.FlagHome, teritoriHomePath),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, from),
		"--keyring-backend=test",
		"--output=json",
		"-y",
	}

	var output []byte
	var erroutput []byte
	captureOutput := func(stdout []byte, stderr []byte) bool {
		output = stdout
		erroutput = stderr
		return true
	}

	s.executeTeritoriTxCommand(ctx, chain, teritoriCommand, valIdx, captureOutput)
	if len(erroutput) > 0 {
		return nil, fmt.Errorf("failed to sign tx: %s", string(erroutput))
	}
	return output, nil
}

// broadcastTxFile broadcasts a signed transaction file using the teritoricli tx broadcast command
// the from flag is used to specify the keyring account to sign the transaction
// the from account must be registered in the keyring and exist on chain (have a balance or be a genesis account)
func (s *IntegrationTestSuite) broadcastTxFile(chain *chain, valIdx int, from string, txFilePath string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	broadcastTxCmd := []string{
		teritoridBinary,
		txCommand,
		"broadcast",
		filepath.Join(teritoriHomePath, txFilePath),
		fmt.Sprintf("--%s=%s", flags.FlagChainID, chain.id),
		fmt.Sprintf("--%s=%s", flags.FlagHome, teritoriHomePath),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, from),
		"--keyring-backend=test",
		"--output=json",
		"-y",
	}

	var output []byte
	var erroutput []byte
	captureOutput := func(stdout []byte, stderr []byte) bool {
		output = stdout
		erroutput = stderr
		return true
	}

	s.executeTeritoriTxCommand(ctx, chain, broadcastTxCmd, valIdx, captureOutput)
	if len(erroutput) > 0 {
		return nil, fmt.Errorf("failed to sign tx: %s", string(erroutput))
	}
	return output, nil
}
