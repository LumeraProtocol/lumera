//go:build system_test

package system

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	claimtestutils "github.com/LumeraProtocol/lumera/x/claim/testutils"
	claimtypes "github.com/LumeraProtocol/lumera/x/claim/types"
)

func TestVestingLock(t *testing.T) {
	// Test params
	const (
		claimAmount = "1000000"
		tier        = "1" // 6 month vesting
	)

	// Init chain
	sut.ResetChain(t)

	// Generate Pastel account for claim
	pastelAccount, err := claimtestutils.GeneratePastelAddress()
	require.NoError(t, err)

	// Setup CSV with test address
	homedir, err := os.UserHomeDir()
	require.NoError(t, err)
	csvPath := homedir + "/claims.csv"
	err = os.WriteFile(csvPath, []byte(pastelAccount.Address+","+claimAmount+"\n"), 0644)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(csvPath) })

	// Configure genesis
	sut.ModifyGenesisJSON(t, func(genesis []byte) []byte {
		state := genesis
		var err error

		state, err = sjson.SetRawBytes(state, "app_state.claim.total_claimable_amount", []byte(claimAmount))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.claim.params.enable_claims", []byte("true"))
		require.NoError(t, err)

		endTime := time.Now().Add(1 * time.Hour).Unix()
		state, err = sjson.SetRawBytes(state, "app_state.claim.params.claim_end_time", []byte(fmt.Sprintf("%d", endTime)))
		require.NoError(t, err)

		return state
	})

	sut.StartChain(t)
	cli := NewLumeradCLI(t, sut, true)

	// Create test addresses
	senderKey := "vesting_sender"
	senderAddr := cli.AddKey(senderKey)
	validatorAddr := cli.GetKeyAddr("node0")

	// Fund sender account
	sendResp := cli.CustomCommand(
		"tx", "bank", "send",
		validatorAddr,
		senderAddr,
		"1000000stake",
		"--from", "node0",
	)
	RequireTxSuccess(t, sendResp)

	// Setup receiver
	receiverKey := "vesting_receiver"
	receiverAddr := cli.AddKey(receiverKey)
	sendReceiverResp := cli.CustomCommand(
		"tx", "bank", "send",
		validatorAddr,
		receiverAddr,
		"10000stake", // Gas money
		"--from", "node0",
	)
	RequireTxSuccess(t, sendReceiverResp)

	// Generate claim data
	testData, err := claimtestutils.GenerateClaimingTestData2(pastelAccount, senderAddr)
	require.NoError(t, err)

	// Verify pre-claim account type
	baseAccount := cli.GetAccount(senderAddr)
	require.NotNil(t, baseAccount, "account not found")
	baseAccountType := gjson.Get(baseAccount, "account.type").String()
	require.Equal(t, "/cosmos.auth.v1beta1.BaseAccount", baseAccountType, "account should be BaseAccount before claiming")

	// Execute claim
	t.Logf("Claiming from %s to %s", pastelAccount.Address, senderAddr)
	resp := cli.CustomCommand(
		"tx", "claim", "delayed-claim",
		testData.OldAddress,
		testData.NewAddress,
		testData.PubKey,
		testData.Signature,
		tier,
		"--from", senderKey,
	)
	RequireTxSuccess(t, resp)

	// Check vesting account creation
	vestingAccount := cli.GetAccount(senderAddr)
	vestingAccountType := gjson.Get(vestingAccount, "account.type").String()
	require.Equal(t, "/cosmos.vesting.v1beta1.DelayedVestingAccount", vestingAccountType, "Account should be DelayedVestingAccount")

	// Check vesting parameters
	vestingAmount := gjson.Get(vestingAccount, "account.value.base_vesting_account.original_vesting.0.amount").String()
	vestingDenom := gjson.Get(vestingAccount, "account.value.base_vesting_account.original_vesting.0.denom").String()
	endTimeUnix := gjson.Get(vestingAccount, "account.value.base_vesting_account.end_time").Int()

	require.Equal(t, claimAmount, vestingAmount, "Vesting amount should match claim")
	require.Equal(t, claimtypes.DefaultClaimsDenom, vestingDenom, "Correct denom")
	require.Greater(t, endTimeUnix, time.Now().Unix(), "End time should be future")

	// Verify vesting end time
	expectedEndTime := time.Now().AddDate(0, 6, 0).Unix()
	require.InDelta(t, expectedEndTime, endTimeUnix, 3600, "~6 month vesting period")

	// Try transfer during vesting period 🔒
	t.Logf("Attempting transfer: %s -> %s", senderAddr, receiverAddr)
	transferResp := cli.CustomCommand(
		"tx", "bank", "send",
		senderAddr,
		receiverAddr,
		"500000"+claimtypes.DefaultClaimsDenom,
		"--from", senderKey,
	)

	// Verify transfer fails
	RequireTxFailure(t, transferResp, "spendable balance 0ulume is smaller than 500000ulume")

	// Check receiver balance
	receiverBalance := cli.QueryBalance(receiverAddr, claimtypes.DefaultClaimsDenom)
	require.Equal(t, int64(0), receiverBalance, "Receiver should have 0 tokens")

	// Check sender balance
	claimedBalance := cli.QueryBalance(senderAddr, claimtypes.DefaultClaimsDenom)
	require.Equal(t, claimAmount, fmt.Sprintf("%d", claimedBalance), "Sender should have full amount")

	t.Log("Tokens confirmed locked in vesting account")
}

func TestDoubleClaimToVestingAccount(t *testing.T) {
	const (
		firstClaimAmount  = "1000000"
		secondClaimAmount = "500000"
		tier              = "1" // 6 month vesting
	)

	sut.ResetChain(t)

	// Generate test accounts
	firstPastelAccount, err := claimtestutils.GeneratePastelAddress()
	require.NoError(t, err)
	secondPastelAccount, err := claimtestutils.GeneratePastelAddress()
	require.NoError(t, err)

	// Setup CSV
	homedir, err := os.UserHomeDir()
	require.NoError(t, err)
	csvPath := homedir + "/claims.csv"
	csvContent := fmt.Sprintf("%s,%s\n%s,%s\n",
		firstPastelAccount.Address, firstClaimAmount,
		secondPastelAccount.Address, secondClaimAmount)
	err = os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(csvPath) })

	// Calculate total for both claims
	totalAmount := fmt.Sprintf("%d",
		parseInt(t, firstClaimAmount)+parseInt(t, secondClaimAmount))

	// Configure genesis
	sut.ModifyGenesisJSON(t, func(genesis []byte) []byte {
		state := genesis
		var err error

		state, err = sjson.SetRawBytes(state, "app_state.claim.total_claimable_amount", []byte(totalAmount))
		require.NoError(t, err)

		state, err = sjson.SetRawBytes(state, "app_state.claim.params.enable_claims", []byte("true"))
		require.NoError(t, err)

		endTime := time.Now().Add(1 * time.Hour).Unix()
		state, err = sjson.SetRawBytes(state, "app_state.claim.params.claim_end_time", []byte(fmt.Sprintf("%d", endTime)))
		require.NoError(t, err)

		return state
	})

	sut.StartChain(t)
	cli := NewLumeradCLI(t, sut, true)

	// Setup test account
	senderKey := "vesting_test_sender"
	senderAddr := cli.AddKey(senderKey)
	validatorAddr := cli.GetKeyAddr("node0")

	// Fund account
	sendResp := cli.CustomCommand(
		"tx", "bank", "send",
		validatorAddr,
		senderAddr,
		"1000000stake",
		"--from", "node0",
	)
	RequireTxSuccess(t, sendResp)

	// Generate first claim data
	firstTestData, err := claimtestutils.GenerateClaimingTestData2(firstPastelAccount, senderAddr)
	require.NoError(t, err)

	// Verify pre-claim account type
	baseAccount := cli.GetAccount(senderAddr)
	require.NotNil(t, baseAccount, "account not found")
	baseAccountType := gjson.Get(baseAccount, "account.type").String()
	require.Equal(t, "/cosmos.auth.v1beta1.BaseAccount", baseAccountType, "should be BaseAccount initially")

	// Execute first claim
	t.Logf("First claim: %s -> %s", firstPastelAccount.Address, senderAddr)
	resp := cli.CustomCommand(
		"tx", "claim", "delayed-claim",
		firstTestData.OldAddress,
		firstTestData.NewAddress,
		firstTestData.PubKey,
		firstTestData.Signature,
		tier,
		"--from", senderKey,
	)
	RequireTxSuccess(t, resp)

	// Check vesting account creation
	vestingAccount := cli.GetAccount(senderAddr)
	vestingAccountType := gjson.Get(vestingAccount, "account.type").String()
	require.Equal(t, "/cosmos.vesting.v1beta1.DelayedVestingAccount", vestingAccountType, "Should be DelayedVestingAccount after claim")

	// Verify vesting amount
	vestingAmount := gjson.Get(vestingAccount, "account.value.base_vesting_account.original_vesting.0.amount").String()
	require.Equal(t, firstClaimAmount, vestingAmount, "Vesting amount matches first claim")

	// Generate second claim data
	secondTestData, err := claimtestutils.GenerateClaimingTestData2(secondPastelAccount, senderAddr)
	require.NoError(t, err)

	// Attempt second claim
	t.Logf("Second claim attempt: %s -> %s", secondPastelAccount.Address, senderAddr)
	secondResp := cli.CustomCommand(
		"tx", "claim", "delayed-claim",
		secondTestData.OldAddress,
		secondTestData.NewAddress,
		secondTestData.PubKey,
		secondTestData.Signature,
		tier,
		"--from", senderKey,
	)

	// Verify second claim fails
	RequireTxFailure(t, secondResp, "destination address already has a non-base account")

	// Check balance
	balance := cli.QueryBalance(senderAddr, claimtypes.DefaultClaimsDenom)
	require.Equal(t, firstClaimAmount, fmt.Sprintf("%d", balance), "Balance should only contain first claim")

	// Verify second claim status
	secondClaimRecord := cli.CustomQuery(
		"q", "claim", "claim-record", secondPastelAccount.Address)
	claimed := gjson.Get(secondClaimRecord, "record.claimed").Bool()
	require.False(t, claimed, "Second claim should remain unclaimed")

	t.Log("Prevented double conversion to vesting account")
}

// Parse integer strings
func parseInt(t *testing.T, s string) int64 {
	var val int64
	_, err := fmt.Sscanf(s, "%d", &val)
	require.NoError(t, err, "Failed to parse integer")
	return val
}
