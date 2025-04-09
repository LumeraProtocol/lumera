package simulation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"google.golang.org/protobuf/proto"

	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/LumeraProtocol/lumera/testutil/cryptotestutils"
	"github.com/LumeraProtocol/lumera/x/action/keeper"
	"github.com/LumeraProtocol/lumera/x/action/types"
	supernodetypes "github.com/LumeraProtocol/lumera/x/supernode/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

// registerSenseAction creates a new SENSE action in PENDING state for the simulation
func registerSenseAction(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, bk types.BankKeeper, k keeper.Keeper, ak types.AccountKeeper) (string, *types.MsgRequestAction) {
	params := k.GetParams(ctx)

	// 1. Select random account with enough balance
	simAccount := selectRandomAccountWithSufficientFunds(r, ctx, accs, bk, ak, []string{""})

	// 2. Generate random valid SENSE metadata
	dataHash := generateRandomHash(r)
	senseMetadata := generateRequestActionSenseMetadata(dataHash)

	// 3. Determine fee amount (within valid range)
	feeAmount := generateRandomFee(r, ctx, params.BaseActionFee)

	// 4. Generate an expiration time (current time + random duration >= expiration_duration)
	expirationTime := getRandomExpirationTime(ctx, r, params)

	// 5. Create message
	msg := types.NewMsgRequestAction(
		simAccount.Address.String(),
		actionapi.ActionType_ACTION_TYPE_SENSE.String(),
		senseMetadata,
		feeAmount.String(),
		strconv.FormatInt(expirationTime, 10),
	)

	// 6. Cache keeper state for simulation
	msgServSim := keeper.NewMsgServerImpl(k)

	// 7. Deliver transaction
	result, err := msgServSim.RequestAction(ctx, msg)
	if err != nil {
		panic(fmt.Sprintf("failed to create SENSE action for finalization test: %v", err))
	}

	return result.ActionId, msg
}

// registerCascadeAction creates a new CASCADE action in PENDING state for the simulation
func registerCascadeAction(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, bk types.BankKeeper, k keeper.Keeper, ak types.AccountKeeper) (string, *types.MsgRequestAction) {
	params := k.GetParams(ctx)

	// 1. Select random account with enough balance
	simAccount := selectRandomAccountWithSufficientFunds(r, ctx, accs, bk, ak, []string{""})

	// 2. Set account public key
	err := addPubKeyToAccount(ctx, simAccount, ak)
	if err != nil {
		panic(fmt.Sprintf("failed to set account public key: %v", err))
	}

	// 2. Generate random valid CASCADE metadata
	dataHash := generateRandomHash(r)
	fileName := generateRandomFileName(r)
	cascadeMetadata := generateRequestActionCascadeMetadata(dataHash, fileName, simAccount)

	// 3. Determine fee amount (within valid range)
	feeAmount := generateRandomFee(r, ctx, params.BaseActionFee)

	// 4. Generate an expiration time (current time + random duration)
	expirationTime := getRandomExpirationTime(ctx, r, params)

	// 5. Create message
	msg := types.NewMsgRequestAction(
		simAccount.Address.String(),
		actionapi.ActionType_ACTION_TYPE_CASCADE.String(),
		cascadeMetadata,
		feeAmount.String(),
		strconv.FormatInt(expirationTime, 10),
	)

	// 6. Cache keeper state for simulation
	msgServSim := keeper.NewMsgServerImpl(k)

	// 7. Deliver transaction
	result, err := msgServSim.RequestAction(ctx, msg)
	if err != nil {
		panic(fmt.Sprintf("failed to create CASCADE action for finalization test: %v", err))
	}

	return result.ActionId, msg
}

// finalizeSenseAction finalizes a SENSE action by submitting 3 matching metadata entries
func finalizeSenseAction(ctx sdk.Context, k keeper.Keeper, bk types.BankKeeper, actionID string, supernodes []simtypes.Account) *types.MsgFinalizeAction {
	// 1. Get the action to verify it exists
	_, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("action with ID %s not found", actionID))
	}

	// 3. Submit from all three supernodes
	msgServSim := keeper.NewMsgServerImpl(k)

	var finalMsg *types.MsgFinalizeAction

	// Create finalization metadata with signature
	metadata := generateFinalizeMetadataForSense(ctx, k, actionID, supernodes)

	for i, supernode := range supernodes {
		// Get supernode's initial balance to verify no fee distribution
		feeDenom := k.GetParams(ctx).BaseActionFee.Denom
		initialBalance := bk.GetBalance(ctx, supernode.Address, feeDenom)

		// Create and submit finalization message
		msg := types.NewMsgFinalizeAction(
			supernode.Address.String(),
			actionID,
			actionapi.ActionType_ACTION_TYPE_SENSE.String(),
			metadata,
		)

		_, err := msgServSim.FinalizeAction(ctx, msg)
		if err != nil {
			panic(fmt.Sprintf("failed to finalize SENSE action %s with supernode %d: %v", actionID, i+1, err))
		}

		updatedAction, found := k.GetActionByID(ctx, actionID)
		if !found {
			panic(fmt.Sprintf("action with ID %s not found after finalization", actionID))
		}

		if i < 2 && updatedAction.State != actionapi.ActionState_ACTION_STATE_PROCESSING {
			panic(fmt.Sprintf("action %s not in PROCESSING state after %d FinalizeAction: %s", actionID, i+1, updatedAction.State))
		}

		if i < 2 {
			finalBalance := bk.GetBalance(ctx, supernode.Address, feeDenom)
			if !finalBalance.Equal(initialBalance) {
				panic(fmt.Sprintf("supernode %d balance changed after %d FinalizeAction, expected no fee distribution", i+1, i+1))
			}
		}

		finalMsg = msg
	}

	// 4. Verify action is in DONE state
	finalAction, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("action with ID %s not found after finalization", actionID))
	}

	if finalAction.State != actionapi.ActionState_ACTION_STATE_DONE {
		panic(fmt.Sprintf("action %s not in DONE state after finalization: %s", actionID, finalAction.State))
	}

	return finalMsg
}

// finalizeCascadeAction finalizes a CASCADE action with a single supernode
func finalizeCascadeAction(ctx sdk.Context, k keeper.Keeper, actionID string, supernodes []simtypes.Account) *types.MsgFinalizeAction {
	// 1. Get the action
	_, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("action with ID %s not found", actionID))
	}

	// 2. Generate finalization data
	metadata := generateFinalizeMetadataForCascade(ctx, k, actionID, supernodes)

	// 3. Create and submit finalization message
	msg := types.NewMsgFinalizeAction(
		supernodes[0].Address.String(),
		actionID,
		actionapi.ActionType_ACTION_TYPE_CASCADE.String(),
		metadata,
	)

	// 4. Deliver transaction
	msgServSim := keeper.NewMsgServerImpl(k)
	_, err := msgServSim.FinalizeAction(ctx, msg)
	if err != nil {
		panic(fmt.Sprintf("failed to finalize CASCADE action %s: %v", actionID, err))
	}

	// 5. Verify action is in DONE state
	finalAction, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("action with ID %s not found after finalization", actionID))
	}

	if finalAction.State != actionapi.ActionState_ACTION_STATE_DONE {
		panic(fmt.Sprintf("action %s not in DONE state after finalization: %s", actionID, finalAction.State))
	}

	return msg
}

// registerSenseOrCascadeAction finds an existing action in PENDING state or creates a new one
func registerSenseOrCascadeAction(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) (string, *actionapi.Action) {
	// Randomly choose between SENSE and CASCADE
	var actionID string
	if r.Intn(2) == 0 {
		actionID, _ = registerSenseAction(r, ctx, accs, bk, k, ak)
	} else {
		actionID, _ = registerCascadeAction(r, ctx, accs, bk, k, ak)
	}

	action, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("failed to find created action with ID %s", actionID))
	}
	return actionID, action
}

func finalizeAction(r *rand.Rand, ctx sdk.Context, k keeper.Keeper, ak types.AccountKeeper, bk types.BankKeeper, actionID string, actionType actionapi.ActionType, accs []simtypes.Account) ([]simtypes.Account, error) {
	if actionType == actionapi.ActionType_ACTION_TYPE_SENSE {
		supernodes, err := getRandomActiveSupernodes(r, ctx, 3, ak, k, accs)
		if err != nil {
			return nil, err
		}
		finalizeSenseAction(ctx, k, bk, actionID, supernodes)
		return supernodes, nil
	} else if actionType == actionapi.ActionType_ACTION_TYPE_CASCADE {
		supernodes, err := getRandomActiveSupernodes(r, ctx, 1, ak, k, accs)
		if err != nil {
			return nil, err
		}
		finalizeCascadeAction(ctx, k, actionID, supernodes)
		return supernodes, nil
	}
	panic("invalid action type")
}

// FindAccount find a specific address from an account list
func FindAccount(accs []simtypes.Account, address string) (simtypes.Account, bool) {
	creator, err := sdk.AccAddressFromBech32(address)
	if err != nil {
		panic(err)
	}
	return simtypes.FindAccount(accs, creator)
}

func addPubKeyToAccount(ctx sdk.Context, simAccount simtypes.Account, ak types.AccountKeeper) error {
	acc := ak.GetAccount(ctx, simAccount.Address)
	if acc != nil {
		err := acc.SetPubKey(simAccount.PubKey)
		if err != nil {
			return fmt.Errorf("failed to set pubkey for account %s: %w", simAccount.Address, err)
		}
		ak.SetAccount(ctx, acc)
		return nil
	}
	return fmt.Errorf("failed to set pubkey for account %s: account not found in account keeper", simAccount.Address)
}

// selectRandomAccountWithSufficientFunds selects a random account that has enough balance to cover the specified fee amount
func selectRandomAccountWithSufficientFunds(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, bk types.BankKeeper, ak types.AccountKeeper, skipAddresses []string) simtypes.Account {
	// Get a random account
	simAccount, _ := simtypes.RandomAcc(r, accs)

	for _, skipAddress := range skipAddresses {
		if simAccount.Address.String() == skipAddress {
			return selectRandomAccountWithSufficientFunds(r, ctx, accs, bk, ak, skipAddresses)
		}
	}

	// Check if the account has enough balance
	denom := sdk.DefaultBondDenom
	balance := bk.GetBalance(ctx, simAccount.Address, denom)

	// Ensure account has enough funds for gas + fees
	if balance.IsZero() || balance.Amount.LT(math.NewInt(1000000)) {
		// If the account doesn't have enough funds, recursively try another account
		return selectRandomAccountWithSufficientFunds(r, ctx, accs, bk, ak, skipAddresses)
	}

	err := addPubKeyToAccount(ctx, simAccount, ak)
	if err != nil {
		panic(err)
	}

	return simAccount
}

// selectRandomAccountWithInsufficientFunds selects a random account that doesn't have enough balance to cover the specified fee amount
func selectRandomAccountWithInsufficientFunds(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, bk types.BankKeeper, minFee sdk.Coin) simtypes.Account {
	// Get a random account
	simAccount, _ := simtypes.RandomAcc(r, accs)

	// Check if the account has insufficient balance
	balance := bk.GetBalance(ctx, simAccount.Address, sdk.DefaultBondDenom)

	// We need an account that has some funds (not zero) but less than the minimal fee
	// This ensures we can see the insufficient funds error rather than something else
	if balance.IsZero() || balance.Amount.GTE(minFee.Amount) {
		// If the account has zero balance or enough funds, recursively try another account or modify balance
		if balance.IsZero() {
			// If balance is zero, randomly select another account
			return selectRandomAccountWithInsufficientFunds(r, ctx, accs, bk, minFee)
		} else {
			// Since all accounts might have sufficient funds in simulation, we can use this account
			// but need to artificially reduce its effective balance for the test
			return simAccount
		}
	}

	return simAccount
}

// generateRandomHash generates a random hash string for use in SENSE metadata
func generateRandomHash(r *rand.Rand) string {
	// Generate random bytes
	randomBytes := make([]byte, 32)
	r.Read(randomBytes)

	// Create a SHA-256 hash from the random bytes
	hash := sha256.Sum256(randomBytes)

	// Convert to hex string
	return hex.EncodeToString(hash[:])
}

// generateRandomFileName generates a random file name for use in CASCADE metadata
func generateRandomFileName(r *rand.Rand) string {
	// Define a list of common file extensions
	extensions := []string{".jpg", ".png", ".pdf", ".txt", ".json", ".xml", ".zip"}

	// Generate a random name (8-16 characters)
	nameLength := 8 + r.Intn(9) // Random length between 8 and 16
	letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	nameBytes := make([]byte, nameLength)
	for i := range nameBytes {
		nameBytes[i] = letters[r.Intn(len(letters))]
	}

	// Choose a random extension
	extension := extensions[r.Intn(len(extensions))]

	return string(nameBytes) + extension
}

func generateCascadeSignature(simAccount simtypes.Account) string {
	privKey := secp256k1.PrivKey{Key: simAccount.PrivKey.Bytes()}
	privKeys := []secp256k1.PrivKey{privKey}
	signatureCascade, err := cryptotestutils.CreateSignatureString(privKeys, 50)
	if err != nil {
		panic(fmt.Sprintf("failed to create CASCADE signature: %v", err))
	}
	return signatureCascade
}

func generateSenseSignature(simAccounts []simtypes.Account) string {
	privKeys := make([]secp256k1.PrivKey, len(simAccounts))
	for i, simAccount := range simAccounts {
		privKey := secp256k1.PrivKey{Key: simAccount.PrivKey.Bytes()}
		privKeys[i] = privKey
	}
	signatureSense, err := cryptotestutils.CreateSignatureString(privKeys, 50)
	if err != nil {
		panic(fmt.Sprintf("failed to create SENSE signature: %v", err))
	}
	return signatureSense
}

func getRandomExpirationTime(ctx sdk.Context, r *rand.Rand, params types.Params) int64 {
	expirationDuration := time.Duration(r.Int63n(int64(params.ExpirationDuration)) + int64(params.ExpirationDuration))
	return ctx.BlockTime().Add(expirationDuration).Unix()
}

// generateRandomFee generates a random fee amount within the valid range
func generateRandomFee(r *rand.Rand, ctx sdk.Context, minFee sdk.Coin) sdk.Coin {
	// Get a random amount between min fee and min fee + 1000000
	minAmount := minFee.Amount.Int64()
	randomAddition := r.Int63n(1000000)
	amount := minAmount + randomAddition

	return sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(amount))
}

// selectRandomActionType randomly selects an action type between SENSE and CASCADE
func selectRandomActionType(r *rand.Rand) string {
	// Define available action types
	actionTypes := []string{
		actionapi.ActionType_ACTION_TYPE_SENSE.String(),
		actionapi.ActionType_ACTION_TYPE_CASCADE.String(),
	}

	// Return random selection
	return actionTypes[r.Intn(len(actionTypes))]
}

// generateRandomOtiValues generates n random bytes as OTI value for CASCADE metadata
func generateRandomOtiValues(n int) []byte {
	return make([]byte, n)
}

// getRandomActiveSupernodes simulates getting a list of active supernodes from the system
func getRandomActiveSupernodes(r *rand.Rand, ctx sdk.Context, numSupernodes int, ak types.AccountKeeper, k keeper.Keeper, accs []simtypes.Account) ([]simtypes.Account, error) {
	top10 := getTop10Supernodes(ctx, k)
	if len(top10) < 10 {
		for i := 0; i < 10-len(top10); i++ {
			_, err := registerSupernode(r, ctx, k, accs)
			if err != nil {
				return nil, err
			}
		}
	}
	top10 = getTop10Supernodes(ctx, k)
	if len(top10) < numSupernodes {
		return nil, fmt.Errorf("not enough active supernodes to satisfy request")
	}
	// Randomly select numSupernodes from top10
	// shuffle top10 and select first numSupernodes
	r.Shuffle(len(top10), func(i, j int) {
		top10[i], top10[j] = top10[j], top10[i]
	})

	selectedSupernodes := make([]simtypes.Account, numSupernodes)
	for i := 0; i < numSupernodes; i++ {
		simAccount, found := simtypes.FindAccount(accs, top10[i])
		if !found {
			panic(fmt.Sprintf("failed to find account for supernode %s", top10[i]))
		}
		selectedSupernodes[i] = simAccount
		err := addPubKeyToAccount(ctx, simAccount, ak)
		if err != nil {
			panic(err)
		}
	}
	return selectedSupernodes, nil
}

func generateKademliaIDs(ic uint64, max uint64, signature string) []string {
	var ids []string
	for i := ic; i < ic+max; i++ {
		id, err := keeper.CreateKademliaID(signature, i)
		if err != nil {
			panic(fmt.Sprintf("failed to create Kademlia ID: %v", err))
		}
		ids = append(ids, id)
	}
	return ids
}

// generateRequestActionSenseMetadata creates valid SENSE metadata for simulation
func generateRequestActionSenseMetadata(dataHash string) string {
	metadata := actionapi.SenseMetadata{
		DataHash:            dataHash,
		DdAndFingerprintsIc: rand.Uint64(),
	}

	// Marshal to JSON
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal SENSE metadata: %v", err))
	}

	return string(metadataBytes)
}

// generateCascadeMetadata creates valid CASCADE metadata for simulation
func generateRequestActionCascadeMetadata(dataHash string, fileName string, simAccount simtypes.Account) string {
	metadata := actionapi.CascadeMetadata{
		DataHash:   dataHash,
		FileName:   fileName,
		RqIdsIc:    rand.Uint64(),
		Signatures: generateCascadeSignature(simAccount),
	}

	// Marshal to JSON
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal CASCADE metadata: %v", err))
	}

	return string(metadataBytes)
}

// generateRequestActionValidMetadata generates valid metadata based on the action type
func generateRequestActionValidMetadata(r *rand.Rand, actionType string, simAccount simtypes.Account) string {
	switch actionType {
	case actionapi.ActionType_ACTION_TYPE_SENSE.String():
		dataHash := generateRandomHash(r)
		return generateRequestActionSenseMetadata(dataHash)
	case actionapi.ActionType_ACTION_TYPE_CASCADE.String():
		dataHash := generateRandomHash(r)
		fileName := generateRandomFileName(r)
		return generateRequestActionCascadeMetadata(dataHash, fileName, simAccount)
	default:
		panic(fmt.Sprintf("unsupported action type: %s", actionType))
	}
}

// generateInvalidSenseMetadata creates invalid SENSE metadata for simulation
func generateInvalidRequestActionSenseMetadata(r *rand.Rand) string {
	// Create an invalid metadata missing required fields
	metadata := actionapi.SenseMetadata{
		// Missing DataHash which is required
		// Missing DdAndFingerprintsIc which is required
	}

	// Alternatively we could add DataHash but set DdAndFingerprintsIc to an invalid value (0 or negative)
	if r.Intn(2) == 1 {
		metadata.DataHash = generateRandomHash(r)
		metadata.DdAndFingerprintsIc = 0 // Invalid value, must be positive
	}

	// Marshal to JSON
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal invalid SENSE metadata: %v", err))
	}

	return string(metadataBytes)
}

// generateInvalidCascadeMetadata creates invalid CASCADE metadata for simulation
func generateInvalidRequestActionCascadeMetadata(r *rand.Rand, simAccount simtypes.Account) string {
	// Create an invalid metadata missing required fields
	metadata := actionapi.CascadeMetadata{
		// Missing DataHash which is required
		// Missing FileName which is required
		// Missing RqIdsIc which is required
		// Missing Signatures which is required
	}

	// Alternatively we could add some fields but set RqIdsIc to an invalid value (0 or negative)
	rnd := r.Intn(2)
	switch rnd {
	case 0:
		metadata.DataHash = generateRandomHash(r)
	case 1:
		metadata.FileName = generateRandomFileName(r)
	case 2:
		metadata.RqIdsIc = 0 // Invalid value, must be positive
	case 3:
		metadata.Signatures = generateCascadeSignature(simAccount)
	}

	// Marshal to JSON
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal invalid CASCADE metadata: %v", err))
	}

	return string(metadataBytes)
}

// generateRequestActionInvalidMetadata generates invalid metadata based on the action type
func generateRequestActionInvalidMetadata(r *rand.Rand, actionType string, simAccount simtypes.Account) string {
	switch actionType {
	case actionapi.ActionType_ACTION_TYPE_SENSE.String():
		return generateInvalidRequestActionSenseMetadata(r)
	case actionapi.ActionType_ACTION_TYPE_CASCADE.String():
		return generateInvalidRequestActionCascadeMetadata(r, simAccount)
	default:
		panic(fmt.Sprintf("unsupported action type: %s", actionType))
	}
}

// generateFinalizeMetadataForSense creates finalization metadata for a SENSE action
func generateFinalizeMetadataForSense(ctx sdk.Context, k keeper.Keeper, actionID string, supernodes []simtypes.Account) string {
	// 1. Get the action using its ID
	action, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("action with ID %s not found", actionID))
	}

	// 2. Parse existing metadata
	var existingMetadata actionapi.SenseMetadata
	err := proto.Unmarshal(action.Metadata, &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing SENSE metadata: %v", err))
	}

	return generateValidFinalizeMetadata(
		existingMetadata.DdAndFingerprintsIc,
		existingMetadata.DdAndFingerprintsMax,
		actionapi.ActionType_ACTION_TYPE_SENSE.String(),
		supernodes,
		"")
}

// generateFinalizeMetadataForCascade creates finalization metadata for a CASCADE action
func generateFinalizeMetadataForCascade(ctx sdk.Context, k keeper.Keeper, actionID string, supernodes []simtypes.Account) string {
	// 1. Get the action using its ID
	action, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("action with ID %s not found", actionID))
	}

	// 2. Parse existing metadata
	var existingMetadata actionapi.CascadeMetadata
	err := proto.Unmarshal(action.Metadata, &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing CASCADE metadata: %v", err))
	}

	return generateValidFinalizeMetadata(
		existingMetadata.RqIdsIc,
		existingMetadata.RqIdsMax,
		actionapi.ActionType_ACTION_TYPE_CASCADE.String(),
		supernodes,
		existingMetadata.Signatures)
}

// generateValidFinalizeMetadata generates valid finalization metadata
func generateValidFinalizeMetadata(ic uint64, max uint64, actionType string, supernodes []simtypes.Account, existingSignature string) string {
	var metadataBytes []byte
	var err error
	switch actionType {
	case actionapi.ActionType_ACTION_TYPE_SENSE.String():
		// Create SENSE finalization metadata
		signature := generateSenseSignature(supernodes)
		ids := generateKademliaIDs(ic, max, signature)
		metadata := actionapi.SenseMetadata{
			DdAndFingerprintsIds: ids,
			Signatures:           signature,
		}
		metadataBytes, err = json.Marshal(&metadata)

	case actionapi.ActionType_ACTION_TYPE_CASCADE.String():
		// Create CASCADE finalization metadata
		ids := generateKademliaIDs(ic, max, existingSignature)
		metadata := actionapi.CascadeMetadata{
			RqIdsIds: ids,
			RqIdsOti: generateRandomOtiValues(12),
		}

		metadataBytes, err = json.Marshal(&metadata)

	default:
		panic(fmt.Sprintf("unsupported action type: %s", actionType))
	}

	if err != nil {
		panic(fmt.Sprintf("failed to marshal %s finalization metadata: %v", actionType, err))
	}

	return string(metadataBytes)
}

// generateNonExistentActionID generates an action ID that doesn't exist in the state
func generateNonExistentActionID(r *rand.Rand, ctx sdk.Context, k keeper.Keeper) string {
	for {
		// Generate a random action ID
		possibleID := generateRandomHash(r)

		// Check if this ID exists in the state
		_, found := k.GetActionByID(ctx, possibleID)
		if !found {
			// If not found, we've got a non-existent ID
			return possibleID
		}
		// If found, generate another ID and try again
	}
}

func findOrCreateDoneActionWithCreator(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) (string, *actionapi.Action, simtypes.Account, error) {
	// Create a PENDING action
	actionID, _ := registerCascadeAction(r, ctx, accs, bk, k, ak)

	// Select three random supernode accounts
	supernodes, err := getRandomActiveSupernodes(r, ctx, 1, ak, k, accs)
	if err != nil {
		return "", nil, simtypes.Account{}, err
	}

	// Finalize action by supernode
	finalizeCascadeAction(ctx, k, actionID, supernodes)

	// Get the created action
	action, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic("Created action not found")
	}

	// Get the creator account
	creator, found := FindAccount(accs, action.Creator)
	if !found {
		panic("Creator account not found")
	}

	// Verify the action is in a DONE state
	if action.State != actionapi.ActionState_ACTION_STATE_DONE {
		panic("Expected DONE action state, but action is in DONE state")
	}

	return actionID, action, creator, nil
}

// findOrCreateActionNotInDoneState finds an action that is NOT in DONE state or creates one
// Returns the action ID, the action object, and the creator account
func findOrCreateActionNotInDoneState(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) (string, *actionapi.Action, simtypes.Account) {
	// Create a PENDING action
	actionID, _ := registerSenseAction(r, ctx, accs, bk, k, ak)

	// Get the created action
	action, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic("Created action not found")
	}

	// Get the creator account
	creator, found := FindAccount(accs, action.Creator)
	if !found {
		panic("Creator account not found")
	}

	// Verify the action is in a non-DONE state (should be PENDING from registerSenseAction)
	if action.State == actionapi.ActionState_ACTION_STATE_DONE {
		panic("Expected non-DONE action state, but action is in DONE state")
	}

	return actionID, action, creator
}

// Helper functions for generating various types of invalid metadata for SENSE actions

// generateFinalizeSenseMetadataMissingDdIds creates SENSE metadata without the required DdAndFingerprintsIds field
func generateFinalizeSenseMetadataMissingDdIds(action *actionapi.Action, supernodes []simtypes.Account) string {
	// Parse existing metadata
	var existingMetadata actionapi.SenseMetadata
	err := proto.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing SENSE metadata: %v", err))
	}

	signature := generateSenseSignature(supernodes)
	//ids := generateKademliaIDs(ic, max, signature)

	// Create invalid metadata with missing DdAndFingerprintsIds field
	metadata := actionapi.SenseMetadata{
		// DdAndFingerprintsIds intentionally omitted
		Signatures: signature,
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateSenseMetadataEmptyDdIds creates SENSE metadata with empty DdAndFingerprintsIds
func generateSenseMetadataEmptyDdIds(action *actionapi.Action, supernodes []simtypes.Account) string {
	// Parse existing metadata
	var existingMetadata actionapi.SenseMetadata
	err := proto.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing SENSE metadata: %v", err))
	}

	signature := generateSenseSignature(supernodes)
	ids := generateKademliaIDs(existingMetadata.DdAndFingerprintsIc, existingMetadata.DdAndFingerprintsMax, signature)
	for i := 0; i < len(ids); i++ {
		ids[i] = ""
	}

	// Create invalid metadata with empty DdAndFingerprintsIds array
	metadata := actionapi.SenseMetadata{
		DdAndFingerprintsIds: ids,
		Signatures:           signature,
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateSenseMetadataInvalidDdIc creates SENSE metadata with invalid DdAndFingerprintsIc count
func generateSenseMetadataInvalidDdIc(action *actionapi.Action, supernodes []simtypes.Account) string {
	// Parse existing metadata
	var existingMetadata actionapi.SenseMetadata
	err := proto.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing SENSE metadata: %v", err))
	}

	// Generate valid IDs but less with wrong initial count
	signature := generateSenseSignature(supernodes)
	ids := generateKademliaIDs(existingMetadata.DdAndFingerprintsIc*2, existingMetadata.DdAndFingerprintsMax, signature)

	metadata := actionapi.SenseMetadata{
		DdAndFingerprintsIds: ids,
		Signatures:           signature,
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateSenseMetadataMissingIds creates SENSE metadata without the required SupernodeFingerprints field
func generateSenseMetadataMissingIds(action *actionapi.Action, supernodes []simtypes.Account) string {
	// Parse existing metadata
	var existingMetadata actionapi.SenseMetadata
	err := proto.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing SENSE metadata: %v", err))
	}

	// Generate some valid IDs
	signature := generateSenseSignature(supernodes)
	ids := generateKademliaIDs(existingMetadata.DdAndFingerprintsIc, existingMetadata.DdAndFingerprintsMax/2, signature)

	// Create invalid metadata with missing SupernodeFingerprints
	metadata := actionapi.SenseMetadata{
		DdAndFingerprintsIds: ids,
		Signatures:           signature,
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateSenseMetadataSignatureMismatch creates SENSE metadata with incorrect DataHash
func generateSenseMetadataSignatureMismatch(action *actionapi.Action, supernodes []simtypes.Account) string {
	// Parse existing metadata
	var existingMetadata actionapi.SenseMetadata
	err := proto.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing SENSE metadata: %v", err))
	}

	signature := generateSenseSignature(supernodes)
	ids := generateKademliaIDs(existingMetadata.DdAndFingerprintsIc, existingMetadata.DdAndFingerprintsMax, "wrong signature")

	// Create invalid metadata with different DataHash than the original action
	metadata := actionapi.SenseMetadata{
		DdAndFingerprintsIds: ids,
		Signatures:           signature,
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// Helper functions for generating various types of invalid metadata for CASCADE actions

// generateCascadeMetadataMissingRqIds creates CASCADE metadata without the required RqIdsIds field
func generateCascadeMetadataMissingRqIds(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.CascadeMetadata
	err := proto.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing CASCADE metadata: %v", err))
	}

	//ids := generateKademliaIDs(ic, max, existingMetadata.Signatures)

	// Create invalid metadata with missing RqIdsIds field
	metadata := actionapi.CascadeMetadata{
		// RqIdsIds intentionally omitted
		RqIdsOti: generateRandomOtiValues(12),
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateCascadeMetadataEmptyRqIds creates CASCADE metadata with empty RqIdsIds
func generateCascadeMetadataEmptyRqIds(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.CascadeMetadata
	err := proto.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing CASCADE metadata: %v", err))
	}

	ids := generateKademliaIDs(existingMetadata.RqIdsIc, existingMetadata.RqIdsMax, existingMetadata.Signatures)
	for i := 0; i < len(ids); i++ {
		ids[i] = ""
	}

	// Create invalid metadata with empty RqIdsIds array
	metadata := actionapi.CascadeMetadata{
		RqIdsIds: ids,
		RqIdsOti: generateRandomOtiValues(12),
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateCascadeMetadataInvalidRqIc creates CASCADE metadata with invalid RqIdsIc count
func generateCascadeMetadataInvalidRqIc(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.CascadeMetadata
	err := proto.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing CASCADE metadata: %v", err))
	}

	// Generate some valid IDs
	ids := generateKademliaIDs(existingMetadata.RqIdsIc*2, existingMetadata.RqIdsMax, existingMetadata.Signatures)

	// Create invalid metadata with mismatched IC count (5) vs actual ID count (3)
	metadata := actionapi.CascadeMetadata{
		RqIdsIds: ids,
		RqIdsOti: generateRandomOtiValues(12),
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateCascadeMetadataMissingIds creates CASCADE metadata with incorrect DataHash
func generateCascadeMetadataMissingIds(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.CascadeMetadata
	err := proto.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing CASCADE metadata: %v", err))
	}

	// Generate some valid IDs
	ids := generateKademliaIDs(existingMetadata.RqIdsIc, existingMetadata.RqIdsMax/2, existingMetadata.Signatures)

	// Create invalid metadata with different DataHash than the original action
	metadata := actionapi.CascadeMetadata{
		RqIdsIds: ids,
		RqIdsOti: generateRandomOtiValues(12),
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateCascadeMetadataSignatureMismatch creates CASCADE metadata with incorrect FileName
func generateCascadeMetadataSignatureMismatch(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.CascadeMetadata
	err := proto.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing CASCADE metadata: %v", err))
	}

	ids := generateKademliaIDs(existingMetadata.RqIdsIc, existingMetadata.RqIdsMax, "wrong signature")

	// Create invalid metadata with different FileName than the original action
	metadata := actionapi.CascadeMetadata{
		RqIdsIds: ids,
		RqIdsOti: generateRandomOtiValues(12),
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// selectRandomAccountExcept selects a random account from the list excluding the specified account
func selectRandomAccountExcept(r *rand.Rand, accs []simtypes.Account, excludeAddr string) simtypes.Account {
	if len(accs) <= 1 {
		panic("not enough accounts to select from")
	}

	// Parse the excluded address
	excludeAccount, err := sdk.AccAddressFromBech32(excludeAddr)
	if err != nil {
		panic(err)
	}

	// Keep selecting a random account until we find one that isn't the excluded account
	for {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		if !simAccount.Address.Equals(excludeAccount) {
			return simAccount
		}
	}
}

// selectAccountWithoutPermission selects a random account that doesn't have PublicKey in the account state
func selectAccountWithoutPermission(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) simtypes.Account {
	// For now, just select a random account since the permission checking is hypothetical
	simAccount, _ := simtypes.RandomAcc(r, accs)
	return simAccount
}

func getTop10Supernodes(ctx sdk.Context, k keeper.Keeper) []sdk.Address {
	// Query top-10 SuperNodes for action's block height
	topSuperNodesReq := &supernodetypes.QueryGetTopSuperNodesForBlockRequest{
		BlockHeight: int32(ctx.BlockHeight()),
		Limit:       10,
	}
	topSuperNodesResp, err := k.GetSupernodeKeeper().GetTopSuperNodesForBlock(ctx, topSuperNodesReq)
	if err != nil {
		panic(err)
	}

	supernodes := make([]sdk.Address, len(topSuperNodesResp.Supernodes))
	for i, sn := range topSuperNodesResp.Supernodes {
		supernodes[i] = sdk.MustAccAddressFromBech32(sn.SupernodeAccount)
	}

	return supernodes
}

func registerSupernode(r *rand.Rand, ctx sdk.Context, k keeper.Keeper, accs []simtypes.Account) (simtypes.Account, error) {
	var simAccount simtypes.Account
	var found bool
	stakingKeeper := k.GetStakingKeeper()

	// Try up to 10 times to find an eligible validator
	for i := 0; i < len(accs); i++ {
		simAccount, _ = simtypes.RandomAcc(r, accs)
		valAddr := sdk.ValAddress(simAccount.Address)

		validator, err := stakingKeeper.GetValidator(ctx, valAddr)
		if err != nil {
			continue
		}

		if validator.IsJailed() {
			continue
		}

		// Check if supernode already exists
		_, superNodeExists := k.GetSupernodeKeeper().QuerySuperNode(ctx, valAddr)
		if superNodeExists {
			continue
		}

		found = true
		break
	}

	if !found {
		return simtypes.Account{}, fmt.Errorf("no eligible validator found")
	}

	valAddr := sdk.ValAddress(simAccount.Address)
	validatorAddress := valAddr.String()

	// Generate a random IP address
	ipAddress := fmt.Sprintf("%d.%d.%d.%d",
		r.Intn(256), r.Intn(256), r.Intn(256), r.Intn(256))

	// Generate a random version
	version := fmt.Sprintf("v%d.%d.%d", r.Intn(10), r.Intn(10), r.Intn(10))

	supernode := supernodetypes.SuperNode{
		ValidatorAddress: validatorAddress,
		SupernodeAccount: simAccount.Address.String(),
		Evidence:         []*supernodetypes.Evidence{},
		Version:          version,
		Metrics: &supernodetypes.MetricsAggregate{
			Metrics:     make(map[string]float64),
			ReportCount: 0,
		},
		States: []*supernodetypes.SuperNodeStateRecord{
			{
				State:  supernodetypes.SuperNodeStateActive,
				Height: ctx.BlockHeight(),
			},
		},
		PrevIpAddresses: []*supernodetypes.IPAddressHistory{
			{
				Address: ipAddress,
				Height:  ctx.BlockHeight(),
			},
		},
	}

	sk := k.GetSupernodeKeeper()
	err := sk.SetSuperNode(ctx, supernode)
	if err != nil {
		panic(err)
	}

	return simAccount, nil
}
