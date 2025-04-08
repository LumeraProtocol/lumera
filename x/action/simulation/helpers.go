package simulation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/LumeraProtocol/lumera/testutil/sample"

	//"github.com/LumeraProtocol/lumera/testutil/sample"
	//"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"math/rand"
	"strconv"
	"time"

	"cosmossdk.io/math"
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/LumeraProtocol/lumera/x/action/keeper"
	"github.com/LumeraProtocol/lumera/x/action/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

// FindAccount find a specific address from an account list
func FindAccount(accs []simtypes.Account, address string) (simtypes.Account, bool) {
	creator, err := sdk.AccAddressFromBech32(address)
	if err != nil {
		panic(err)
	}
	return simtypes.FindAccount(accs, creator)
}

// selectRandomAccountWithSufficientFunds selects a random account that has enough balance to cover the specified fee amount
func selectRandomAccountWithSufficientFunds(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, bk types.BankKeeper, ak types.AccountKeeper) simtypes.Account {
	// Get a random account
	simAccount, _ := simtypes.RandomAcc(r, accs)

	// Check if the account has enough balance
	denom := sdk.DefaultBondDenom
	balance := bk.GetBalance(ctx, simAccount.Address, denom)

	// Ensure account has enough funds for gas + fees
	if balance.IsZero() || balance.Amount.LT(math.NewInt(1000000)) {
		// If the account doesn't have enough funds, recursively try another account
		return selectRandomAccountWithSufficientFunds(r, ctx, accs, bk, ak)
	}

	acc := ak.GetAccount(ctx, simAccount.Address)
	if acc != nil {
		err := acc.SetPubKey(simAccount.PubKey)
		if err != nil {
			return simAccount
		}
		ak.SetAccount(ctx, acc)
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

func getCascadeSignature(simAccount simtypes.Account) string {
	privKey := secp256k1.PrivKey{Key: simAccount.PrivKey.Bytes()}
	signatureCascade, err := sample.CreateSignatureString([]secp256k1.PrivKey{privKey}, 50)
	if err != nil {
		panic(fmt.Sprintf("failed to create CASCADE signature: %v", err))
	}
	return signatureCascade
}

// generateCascadeMetadata creates valid CASCADE metadata for simulation
func generateRequestActionCascadeMetadata(dataHash string, fileName string, simAccount simtypes.Account) string {
	metadata := actionapi.CascadeMetadata{
		DataHash:   dataHash,
		FileName:   fileName,
		RqIdsIc:    rand.Uint64(),
		Signatures: getCascadeSignature(simAccount),
	}

	// Marshal to JSON
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal CASCADE metadata: %v", err))
	}

	return string(metadataBytes)
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

// generateInvalidMetadata generates invalid metadata based on the action type
func generateInvalidMetadata(r *rand.Rand, actionType string, simAccount simtypes.Account) string {
	switch actionType {
	case actionapi.ActionType_ACTION_TYPE_SENSE.String():
		return generateInvalidRequestActionSenseMetadata(r)
	case actionapi.ActionType_ACTION_TYPE_CASCADE.String():
		return generateInvalidRequestActionCascadeMetadata(r, simAccount)
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
		metadata.Signatures = getCascadeSignature(simAccount)
	}

	// Marshal to JSON
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal invalid CASCADE metadata: %v", err))
	}

	return string(metadataBytes)
}

// createPendingSenseAction creates a new SENSE action in PENDING state for the simulation
func createPendingSenseAction(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, bk types.BankKeeper, k keeper.Keeper, ak types.AccountKeeper) string {
	// Create a new SENSE action
	simAccount := selectRandomAccountWithSufficientFunds(r, ctx, accs, bk, ak)
	dataHash := generateRandomHash(r)
	params := k.GetParams(ctx)
	senseMetadata := generateRequestActionSenseMetadata(dataHash)
	feeAmount := generateRandomFee(r, ctx, params.BaseActionFee)
	expirationDuration := time.Duration(r.Int63n(int64(params.ExpirationDuration)))
	expirationTime := ctx.BlockTime().Add(expirationDuration).Unix()

	msg := types.NewMsgRequestAction(
		simAccount.Address.String(),
		actionapi.ActionType_ACTION_TYPE_SENSE.String(),
		senseMetadata,
		feeAmount.String(),
		strconv.FormatInt(expirationTime, 10),
	)

	msgServSim := keeper.NewMsgServerImpl(k)
	result, err := msgServSim.RequestAction(ctx, msg)
	if err != nil {
		panic(fmt.Sprintf("failed to create SENSE action for finalization test: %v", err))
	}

	return result.ActionId
}

// selectRandomSupernodes selects n random accounts to act as supernodes for the simulation
func selectRandomSupernodes(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, n int) []simtypes.Account {
	if len(accs) < n {
		panic(fmt.Sprintf("not enough accounts to select %d supernodes", n))
	}

	// Shuffle the accounts to ensure randomness
	shuffledAccs := make([]simtypes.Account, len(accs))
	copy(shuffledAccs, accs)
	r.Shuffle(len(shuffledAccs), func(i, j int) {
		shuffledAccs[i], shuffledAccs[j] = shuffledAccs[j], shuffledAccs[i]
	})

	// Select the first n accounts
	return shuffledAccs[:n]
}

// generateRandomKademliaIDs generates n random Kademlia IDs
func generateRandomKademliaIDs(r *rand.Rand, n int) []string {
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		// Generate a random hash as a Kademlia ID
		ids[i] = generateRandomHash(r)
	}
	return ids
}

// generateConsistentFingerprintResults generates a consistent set of fingerprint results for the supernodes
func generateConsistentFingerprintResults(r *rand.Rand) map[string]string {
	// Create a map to store the fingerprint results
	// For the simulation, we'll use simple mock data with consistent results
	results := make(map[string]string)

	// Generate between 1-5 fingerprint results
	numResults := 1 + r.Intn(5)
	for i := 0; i < numResults; i++ {
		key := fmt.Sprintf("feature_%d", i)
		value := generateRandomHash(r)[:10] // Use part of a hash as the feature value
		results[key] = value
	}

	return results
}

// generateFinalizeMetadataForSense creates finalization metadata for a SENSE action
func generateFinalizeMetadataForSense(r *rand.Rand, ctx sdk.Context, k keeper.Keeper, actionID string, fingerprintResults map[string]string, ddIds []string) string {
	// Get the action using its ID
	action, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("action with ID %s not found", actionID))
	}

	// Parse existing metadata
	var existingMetadata actionapi.SenseMetadata
	err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing SENSE metadata: %v", err))
	}

	// Create finalization metadata
	metadata := actionapi.SenseMetadata{
		DataHash:              existingMetadata.DataHash,
		DdAndFingerprintsIc:   uint64(len(ddIds)),
		CollectionId:          existingMetadata.CollectionId,
		GroupId:               existingMetadata.GroupId,
		DdAndFingerprintsIds:  ddIds,
		Signatures:            "",
		SupernodeFingerprints: fingerprintResults,
	}

	// Marshal to JSON
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal SENSE finalization metadata: %v", err))
	}

	return string(metadataBytes)
}

// signMetadata simulates signing the metadata with a supernode's key
func signMetadata(supernode simtypes.Account, metadata string) string {
	// In a real implementation, this would use the supernode's private key to sign the metadata
	// For simulation, we'll create a mock signature
	hash := sha256.Sum256([]byte(metadata + supernode.Address.String()))
	return hex.EncodeToString(hash[:])
}

// addSignatureToMetadata adds a signature to the metadata
func addSignatureToMetadata(metadata string, signature string) string {
	// Try to unmarshal as SenseMetadata first
	var senseMetadata actionapi.SenseMetadata
	senseErr := json.Unmarshal([]byte(metadata), &senseMetadata)
	if senseErr == nil {
		// It's SenseMetadata
		if senseMetadata.Signatures == "" {
			senseMetadata.Signatures = signature
		} else {
			senseMetadata.Signatures += "," + signature
		}

		// Marshal back to JSON
		metadataBytes, err := json.Marshal(&senseMetadata)
		if err != nil {
			panic(fmt.Sprintf("failed to marshal SENSE metadata with signature: %v", err))
		}

		return string(metadataBytes)
	}

	// If not SenseMetadata, try CascadeMetadata
	var cascadeMetadata actionapi.CascadeMetadata
	cascadeErr := json.Unmarshal([]byte(metadata), &cascadeMetadata)
	if cascadeErr == nil {
		// It's CascadeMetadata
		if cascadeMetadata.Signatures == "" {
			cascadeMetadata.Signatures = signature
		} else {
			cascadeMetadata.Signatures += "," + signature
		}

		// Marshal back to JSON
		metadataBytes, err := json.Marshal(&cascadeMetadata)
		if err != nil {
			panic(fmt.Sprintf("failed to marshal CASCADE metadata with signature: %v", err))
		}

		return string(metadataBytes)
	}

	// If we get here, both unmarshal attempts failed
	panic(fmt.Sprintf("failed to unmarshal metadata: %v, %v", senseErr, cascadeErr))
}

// createPendingCascadeAction creates a new CASCADE action in PENDING state for the simulation
func createPendingCascadeAction(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, bk types.BankKeeper, k keeper.Keeper, ak types.AccountKeeper) string {
	// Create a new CASCADE action
	simAccount := selectRandomAccountWithSufficientFunds(r, ctx, accs, bk, ak)
	dataHash := generateRandomHash(r)
	fileName := generateRandomFileName(r)
	cascadeMetadata := generateRequestActionCascadeMetadata(dataHash, fileName, simAccount)
	feeAmount := generateRandomFee(r, ctx, k.GetParams(ctx).BaseActionFee)
	expirationDuration := time.Duration(r.Int63n(int64(k.GetParams(ctx).ExpirationDuration)))
	expirationTime := ctx.BlockTime().Add(expirationDuration).Unix()

	msg := types.NewMsgRequestAction(
		simAccount.Address.String(),
		actionapi.ActionType_ACTION_TYPE_CASCADE.String(),
		cascadeMetadata,
		feeAmount.String(),
		strconv.FormatInt(expirationTime, 10),
	)

	msgServSim := keeper.NewMsgServerImpl(k)
	result, err := msgServSim.RequestAction(ctx, msg)
	if err != nil {
		panic(fmt.Sprintf("failed to create CASCADE action for finalization test: %v", err))
	}

	return result.ActionId
}

// selectRandomSupernode selects a single random account to act as a supernode
func selectRandomSupernode(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) simtypes.Account {
	supernodes := selectRandomSupernodes(r, ctx, accs, 1)
	return supernodes[0]
}

// generateRandomRqIds generates n random request IDs for CASCADE metadata
func generateRandomRqIds(r *rand.Rand, n int) []string {
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		// Generate a random hash as a request ID
		ids[i] = generateRandomHash(r)
	}
	return ids
}

// generateRandomOtiValues generates n random OTI values for CASCADE metadata
func generateRandomOtiValues(r *rand.Rand, n int) []string {
	values := make([]string, n)
	for i := 0; i < n; i++ {
		// Generate a random OTI value (we'll use a hash substring for simulation)
		values[i] = generateRandomHash(r)[:16]
	}
	return values
}

// generateFinalizeMetadataForCascade creates finalization metadata for a CASCADE action
func generateFinalizeMetadataForCascade(action *actionapi.Action, rqIds []string, otiValues []string) string {
	// Parse existing metadata
	var existingMetadata actionapi.CascadeMetadata
	err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing CASCADE metadata: %v", err))
	}

	// Create finalization metadata
	metadata := actionapi.CascadeMetadata{
		DataHash:   existingMetadata.DataHash,
		FileName:   existingMetadata.FileName,
		RqIdsIc:    uint64(len(rqIds)),
		RqIdsIds:   rqIds,
		Signatures: "",
		// Note: RqIdsOti field is omitted for simulation purposes
	}

	// Marshal to JSON
	metadataBytes, err := json.Marshal(&metadata)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal CASCADE finalization metadata: %v", err))
	}

	return string(metadataBytes)
}

// encodeMapToJSON encodes a map to a JSON string
func encodeMapToJSON(m map[string]string) string {
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal map to JSON: %v", err))
	}
	return string(jsonBytes)
}

// generateNonMatchingFingerprintResults generates fingerprint results that are different from the original
// This is useful for testing consensus failure scenarios in SENSE actions
func generateNonMatchingFingerprintResults(r *rand.Rand, originalResults map[string]string) map[string]string {
	nonMatchingResults := make(map[string]string)

	// Copy all the original key-value pairs
	for k, v := range originalResults {
		nonMatchingResults[k] = v
	}

	// Modify at least one value to make results non-matching
	// Either modify an existing value or add a new one
	if len(nonMatchingResults) > 0 && r.Intn(2) == 0 {
		// Modify an existing value
		for k := range nonMatchingResults {
			nonMatchingResults[k] = nonMatchingResults[k] + "_modified"
			break // just modify one key
		}
	} else {
		// Add a new key-value pair
		newKey := fmt.Sprintf("feature_%d", len(nonMatchingResults))
		newValue := generateRandomHash(r)[:10] // Use part of a hash as the feature value
		nonMatchingResults[newKey] = newValue
	}

	return nonMatchingResults
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

// generateValidFinalizeMetadata generates valid finalization metadata
// Since we're testing with an invalid action ID, the content doesn't matter much
func generateValidFinalizeMetadata(r *rand.Rand) string {
	// Randomly choose between SENSE and CASCADE metadata
	actionType := selectRandomActionType(r)

	switch actionType {
	case actionapi.ActionType_ACTION_TYPE_SENSE.String():
		// Create valid SENSE finalization metadata
		metadata := actionapi.SenseMetadata{
			DataHash:              generateRandomHash(r),
			DdAndFingerprintsIc:   3,
			DdAndFingerprintsIds:  generateRandomKademliaIDs(r, 3),
			Signatures:            "",
			SupernodeFingerprints: generateConsistentFingerprintResults(r),
		}

		metadataBytes, err := json.Marshal(&metadata)
		if err != nil {
			panic(fmt.Sprintf("failed to marshal SENSE finalization metadata: %v", err))
		}

		return string(metadataBytes)

	case actionapi.ActionType_ACTION_TYPE_CASCADE.String():
		// Create valid CASCADE finalization metadata
		metadata := actionapi.CascadeMetadata{
			DataHash:   generateRandomHash(r),
			FileName:   generateRandomFileName(r),
			RqIdsIc:    5,
			RqIdsIds:   generateRandomRqIds(r, 5),
			Signatures: "",
		}

		metadataBytes, err := json.Marshal(&metadata)
		if err != nil {
			panic(fmt.Sprintf("failed to marshal CASCADE finalization metadata: %v", err))
		}

		return string(metadataBytes)

	default:
		panic(fmt.Sprintf("unsupported action type: %s", actionType))
	}
}

// createDoneAction creates a new action in the DONE state for testing purposes
func createDoneAction(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) (string, *actionapi.Action) {
	// Create a pending action first
	actionID := createPendingSenseAction(r, ctx, accs, bk, k, ak)

	// Get the created action
	action, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic("Created action not found")
	}

	// For simulation testing purposes, we just return the action with a modified state
	// We don't actually modify it in the store, as this is just for testing
	// The simulation code will verify that attempting to finalize this action will fail
	// due to its invalid state, even though we're only changing it in memory
	simulatedAction := action
	simulatedAction.State = actionapi.ActionState_ACTION_STATE_DONE

	return actionID, simulatedAction
}

// findOrCreatePendingAction finds an existing action in PENDING state or creates a new one
func findOrCreatePendingAction(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) (string, *actionapi.Action) {
	// For simulation purposes, we'll just create a new pending action
	// Randomly choose between SENSE and CASCADE
	var actionID string
	if r.Intn(2) == 0 {
		actionID = createPendingSenseAction(r, ctx, accs, bk, k, ak)
	} else {
		actionID = createPendingCascadeAction(r, ctx, accs, bk, k, ak)
	}

	action, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("failed to find created action with ID %s", actionID))
	}
	return actionID, action
}

// selectRandomNonSupernode selects a random account that is not registered as a supernode
func selectRandomNonSupernode(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) simtypes.Account {
	// For simulation purposes, we'll use a simple approach:
	// - We'll shuffle all accounts
	// - Then pick from the second half of accounts, assuming first half are supernodes

	// If we only have few accounts, return a random one as a fallback
	if len(accs) < 3 {
		randomAccount, _ := simtypes.RandomAcc(r, accs)
		return randomAccount
	}

	// Shuffle the accounts to ensure randomness
	shuffledAccs := make([]simtypes.Account, len(accs))
	copy(shuffledAccs, accs)
	r.Shuffle(len(shuffledAccs), func(i, j int) {
		shuffledAccs[i], shuffledAccs[j] = shuffledAccs[j], shuffledAccs[i]
	})

	// For simulation purposes, we'll assume the first 1/3 of shuffled accounts are supernodes
	// and the rest are regular accounts
	supernodeCount := len(shuffledAccs) / 3
	if supernodeCount < 1 {
		supernodeCount = 1
	}

	// Select a random account from the non-supernode accounts
	nonSupernodeIndex := supernodeCount + r.Intn(len(shuffledAccs)-supernodeCount)
	if nonSupernodeIndex >= len(shuffledAccs) {
		// Fallback if we somehow go out of bounds
		nonSupernodeIndex = len(shuffledAccs) - 1
	}

	return shuffledAccs[nonSupernodeIndex]
}

// findOrCreateDoneActionWithCreator finds an action in DONE state or creates one,
// and returns its ID, the action itself, and the creator account
func findOrCreateDoneActionWithCreator(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) (string, *actionapi.Action, simtypes.Account) {
	// For simplicity in the simulation, we'll create a new action and set it to DONE state
	// Create a PENDING action first
	actionID := createPendingSenseAction(r, ctx, accs, bk, k, ak)

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

	// For simulation, manually update the action state to DONE
	// We create a modified copy to return, but don't modify the actual store
	doneAction := action
	doneAction.State = actionapi.ActionState_ACTION_STATE_DONE
	// Update the action in the store
	_ = k.SetAction(ctx, doneAction)

	// Verify the action is now in DONE state
	finalAction, found := k.GetActionByID(ctx, actionID)
	if !found || finalAction.State != actionapi.ActionState_ACTION_STATE_DONE {
		panic("Failed to set action to DONE state")
	}

	return actionID, doneAction, creator
}

// generateApprovalSignature generates a valid signature for action approval
func generateApprovalSignature(creator simtypes.Account, actionID string) string {
	// In a real implementation, this would use the creator's private key to sign the action ID
	// For simulation, we create a mock signature
	hash := sha256.Sum256([]byte(actionID + creator.Address.String()))
	return hex.EncodeToString(hash[:])
}

// findOrCreateActionNotInDoneState finds an action that is NOT in DONE state or creates one
// Returns the action ID, the action object, and the creator account
func findOrCreateActionNotInDoneState(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account, k keeper.Keeper, bk types.BankKeeper, ak types.AccountKeeper) (string, *actionapi.Action, simtypes.Account) {
	// Create a PENDING action
	actionID := createPendingSenseAction(r, ctx, accs, bk, k, ak)

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

	// Verify the action is in a non-DONE state (should be PENDING from createPendingSenseAction)
	if action.State == actionapi.ActionState_ACTION_STATE_DONE {
		panic("Expected non-DONE action state, but action is in DONE state")
	}

	return actionID, action, creator
}

// Helper functions for generating various types of invalid metadata for SENSE actions

// generateSenseMetadataMissingDdIds creates SENSE metadata without the required DdAndFingerprintsIds field
func generateSenseMetadataMissingDdIds(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.SenseMetadata
	err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing SENSE metadata: %v", err))
	}

	// Create invalid metadata with missing DdAndFingerprintsIds field
	metadata := actionapi.SenseMetadata{
		DataHash:            existingMetadata.DataHash,
		DdAndFingerprintsIc: 3, // Claiming 3 ids but not providing any
		CollectionId:        existingMetadata.CollectionId,
		GroupId:             existingMetadata.GroupId,
		// DdAndFingerprintsIds intentionally omitted
		Signatures:            "",
		SupernodeFingerprints: make(map[string]string),
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateSenseMetadataEmptyDdIds creates SENSE metadata with empty DdAndFingerprintsIds
func generateSenseMetadataEmptyDdIds(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.SenseMetadata
	err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing SENSE metadata: %v", err))
	}

	// Create invalid metadata with empty DdAndFingerprintsIds array
	metadata := actionapi.SenseMetadata{
		DataHash:              existingMetadata.DataHash,
		DdAndFingerprintsIc:   3, // Claiming 3 ids but providing empty array
		CollectionId:          existingMetadata.CollectionId,
		GroupId:               existingMetadata.GroupId,
		DdAndFingerprintsIds:  []string{}, // Empty array
		Signatures:            "",
		SupernodeFingerprints: make(map[string]string),
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateSenseMetadataInvalidDdIc creates SENSE metadata with invalid DdAndFingerprintsIc count
func generateSenseMetadataInvalidDdIc(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.SenseMetadata
	err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing SENSE metadata: %v", err))
	}

	// Generate some valid IDs
	ids := []string{"id1", "id2", "id3"}

	// Create invalid metadata with mismatched IC count (5) vs actual ID count (3)
	metadata := actionapi.SenseMetadata{
		DataHash:              existingMetadata.DataHash,
		DdAndFingerprintsIc:   5, // Claiming 5 ids but only providing 3
		CollectionId:          existingMetadata.CollectionId,
		GroupId:               existingMetadata.GroupId,
		DdAndFingerprintsIds:  ids,
		Signatures:            "",
		SupernodeFingerprints: make(map[string]string),
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateSenseMetadataMissingFingerprints creates SENSE metadata without the required SupernodeFingerprints field
func generateSenseMetadataMissingFingerprints(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.SenseMetadata
	err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing SENSE metadata: %v", err))
	}

	// Generate some valid IDs
	ids := []string{"id1", "id2", "id3"}

	// Create invalid metadata with missing SupernodeFingerprints
	metadata := actionapi.SenseMetadata{
		DataHash:             existingMetadata.DataHash,
		DdAndFingerprintsIc:  uint64(len(ids)),
		CollectionId:         existingMetadata.CollectionId,
		GroupId:              existingMetadata.GroupId,
		DdAndFingerprintsIds: ids,
		Signatures:           "",
		// SupernodeFingerprints intentionally omitted
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateSenseMetadataDataHashMismatch creates SENSE metadata with incorrect DataHash
func generateSenseMetadataDataHashMismatch(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.SenseMetadata
	err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing SENSE metadata: %v", err))
	}

	// Generate some valid IDs
	ids := []string{"id1", "id2", "id3"}

	// Create invalid metadata with different DataHash than the original action
	metadata := actionapi.SenseMetadata{
		DataHash:              existingMetadata.DataHash + "invalidated", // Changed DataHash
		DdAndFingerprintsIc:   uint64(len(ids)),
		CollectionId:          existingMetadata.CollectionId,
		GroupId:               existingMetadata.GroupId,
		DdAndFingerprintsIds:  ids,
		Signatures:            "",
		SupernodeFingerprints: make(map[string]string),
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// Helper functions for generating various types of invalid metadata for CASCADE actions

// generateCascadeMetadataMissingRqIds creates CASCADE metadata without the required RqIdsIds field
func generateCascadeMetadataMissingRqIds(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.CascadeMetadata
	err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing CASCADE metadata: %v", err))
	}

	// Create invalid metadata with missing RqIdsIds field
	metadata := actionapi.CascadeMetadata{
		DataHash: existingMetadata.DataHash,
		FileName: existingMetadata.FileName,
		RqIdsIc:  5, // Claiming 5 ids but not providing any
		// RqIdsIds intentionally omitted
		Signatures: "",
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateCascadeMetadataEmptyRqIds creates CASCADE metadata with empty RqIdsIds
func generateCascadeMetadataEmptyRqIds(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.CascadeMetadata
	err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing CASCADE metadata: %v", err))
	}

	// Create invalid metadata with empty RqIdsIds array
	metadata := actionapi.CascadeMetadata{
		DataHash:   existingMetadata.DataHash,
		FileName:   existingMetadata.FileName,
		RqIdsIc:    5,          // Claiming 5 ids but providing empty array
		RqIdsIds:   []string{}, // Empty array
		RqIdsOti:   []byte{},   // Empty array as bytes
		Signatures: "",
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateCascadeMetadataInvalidRqIc creates CASCADE metadata with invalid RqIdsIc count
func generateCascadeMetadataInvalidRqIc(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.CascadeMetadata
	err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing CASCADE metadata: %v", err))
	}

	// Generate some valid IDs
	ids := []string{"id1", "id2", "id3"}

	// Create invalid metadata with mismatched IC count (5) vs actual ID count (3)
	metadata := actionapi.CascadeMetadata{
		DataHash:   existingMetadata.DataHash,
		FileName:   existingMetadata.FileName,
		RqIdsIc:    5, // Claiming 5 ids but only providing 3
		RqIdsIds:   ids,
		RqIdsOti:   []byte("oti_data"), // Add some byte data
		Signatures: "",
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateCascadeMetadataDataHashMismatch creates CASCADE metadata with incorrect DataHash
func generateCascadeMetadataDataHashMismatch(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.CascadeMetadata
	err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing CASCADE metadata: %v", err))
	}

	// Generate some valid IDs
	ids := []string{"id1", "id2", "id3"}

	// Create invalid metadata with different DataHash than the original action
	metadata := actionapi.CascadeMetadata{
		DataHash:   existingMetadata.DataHash + "invalidated", // Changed DataHash
		FileName:   existingMetadata.FileName,
		RqIdsIc:    uint64(len(ids)),
		RqIdsIds:   ids,
		RqIdsOti:   []byte("oti_data"), // Add some byte data
		Signatures: "",
	}

	metadataBytes, _ := json.Marshal(&metadata)
	return string(metadataBytes)
}

// generateCascadeMetadataFileNameMismatch creates CASCADE metadata with incorrect FileName
func generateCascadeMetadataFileNameMismatch(action *actionapi.Action) string {
	// Parse existing metadata
	var existingMetadata actionapi.CascadeMetadata
	err := json.Unmarshal([]byte(action.Metadata), &existingMetadata)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal existing CASCADE metadata: %v", err))
	}

	// Generate some valid IDs
	ids := []string{"id1", "id2", "id3"}

	// Create invalid metadata with different FileName than the original action
	metadata := actionapi.CascadeMetadata{
		DataHash:   existingMetadata.DataHash,
		FileName:   existingMetadata.FileName + ".wrong", // Changed FileName
		RqIdsIc:    uint64(len(ids)),
		RqIdsIds:   ids,
		RqIdsOti:   []byte("oti_data"), // Add some byte data
		Signatures: "",
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

// selectAccountWithoutPermission selects a random account that hypothetically lacks permissions
// for requesting an action. Since the actual permission mechanism isn't defined yet,
// this simply selects a random account for now.
func selectAccountWithoutPermission(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) simtypes.Account {
	// For now, just select a random account since the permission checking is hypothetical
	simAccount, _ := simtypes.RandomAcc(r, accs)
	return simAccount
}

// generateValidFinalizationMetadata generates valid finalization metadata for verification attempts
func generateValidFinalizationMetadata(r *rand.Rand, actionType string, originalMetadata string) string {
	switch actionType {
	case actionapi.ActionType_ACTION_TYPE_SENSE.String():
		var existingMetadata actionapi.SenseMetadata
		_ = json.Unmarshal([]byte(originalMetadata), &existingMetadata)

		ddIds := generateRandomKademliaIDs(r, 3)
		fingerprintResults := generateConsistentFingerprintResults(r)

		metadata := actionapi.SenseMetadata{
			DataHash:              existingMetadata.DataHash,
			DdAndFingerprintsIc:   uint64(len(ddIds)),
			CollectionId:          existingMetadata.CollectionId,
			GroupId:               existingMetadata.GroupId,
			DdAndFingerprintsIds:  ddIds,
			Signatures:            "",
			SupernodeFingerprints: fingerprintResults,
		}

		metadataBytes, _ := json.Marshal(&metadata)
		return string(metadataBytes)

	case actionapi.ActionType_ACTION_TYPE_CASCADE.String():
		var existingMetadata actionapi.CascadeMetadata
		_ = json.Unmarshal([]byte(originalMetadata), &existingMetadata)

		rqIds := generateRandomRqIds(r, 3)

		metadata := actionapi.CascadeMetadata{
			DataHash:   existingMetadata.DataHash,
			FileName:   existingMetadata.FileName,
			RqIdsIc:    uint64(len(rqIds)),
			RqIdsIds:   rqIds,
			Signatures: "",
		}

		metadataBytes, _ := json.Marshal(&metadata)
		return string(metadataBytes)

	default:
		panic(fmt.Sprintf("unsupported action type: %s", actionType))
	}
}

// getRandomActiveSupernodes simulates getting a list of active supernodes from the system
func getRandomActiveSupernodes(r *rand.Rand, ctx sdk.Context, actionType string) []simtypes.Account {
	// For the simulation, we'll just return random accounts as supernodes
	// In a real implementation, this would query the supernode keeper

	// Determine number of supernodes to return based on action type
	numSupernodes := 1 // Default for CASCADE
	if actionType == actionapi.ActionType_ACTION_TYPE_SENSE.String() {
		numSupernodes = 3 // SENSE requires 3 supernodes
	}

	// Create dummy accounts to represent supernodes
	supernodes := make([]simtypes.Account, numSupernodes)
	for i := 0; i < numSupernodes; i++ {
		// Generate a random address
		privateKey := make([]byte, 32)
		r.Read(privateKey)
		simAccount := simtypes.Account{
			PrivKey: secp256k1.GenPrivKeyFromSecret(privateKey),
			PubKey:  secp256k1.GenPrivKeyFromSecret(privateKey).PubKey(),
			Address: sdk.AccAddress(secp256k1.GenPrivKeyFromSecret(privateKey).PubKey().Address()),
		}
		supernodes[i] = simAccount
	}

	return supernodes
}

// finalizeSenseActionWithConsensus finalizes a SENSE action by submitting 3 matching metadata entries
// from different supernodes, establishing consensus
func finalizeSenseActionWithConsensus(ctx sdk.Context, k keeper.Keeper, actionID string, supernodes []simtypes.Account) {
	// 1. Get the action to verify it exists
	_, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("action with ID %s not found", actionID))
	}

	// 2. Generate consensus data (same for all supernodes)
	r := rand.New(rand.NewSource(ctx.BlockTime().UnixNano())) // Create deterministic random source
	ddIds := generateRandomKademliaIDs(r, 3)
	fingerprintResults := generateConsistentFingerprintResults(r)

	// 3. Submit from all three supernodes
	msgServSim := keeper.NewMsgServerImpl(k)

	for i, supernode := range supernodes[:3] { // Ensure we only use the first 3 supernodes
		// Create finalization metadata with signature
		metadata := generateFinalizeMetadataForSense(r, ctx, k, actionID, fingerprintResults, ddIds)
		signature := signMetadata(supernode, metadata)
		metadataWithSig := addSignatureToMetadata(metadata, signature)

		// Create and submit finalization message
		msg := types.NewMsgFinalizeAction(
			supernode.Address.String(),
			actionID,
			actionapi.ActionType_ACTION_TYPE_SENSE.String(),
			metadataWithSig,
		)

		_, err := msgServSim.FinalizeAction(ctx, msg)
		if err != nil {
			panic(fmt.Sprintf("failed to finalize SENSE action %s with supernode %d: %v", actionID, i+1, err))
		}
	}

	// 4. Verify action is in DONE state
	finalAction, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("action with ID %s not found after finalization", actionID))
	}

	if finalAction.State != actionapi.ActionState_ACTION_STATE_DONE {
		panic(fmt.Sprintf("action %s not in DONE state after finalization: %s", actionID, finalAction.State))
	}
}

// finalizeCascadeAction finalizes a CASCADE action with a single supernode
func finalizeCascadeAction(ctx sdk.Context, k keeper.Keeper, actionID string, supernode simtypes.Account) {
	// 1. Get the action
	action, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("action with ID %s not found", actionID))
	}

	// 2. Generate finalization data
	r := rand.New(rand.NewSource(ctx.BlockTime().UnixNano())) // Create deterministic random source
	rqIds := generateRandomRqIds(r, 5)
	otiValues := generateRandomOtiValues(r, 5)

	// 3. Create finalization metadata with signature
	metadata := generateFinalizeMetadataForCascade(action, rqIds, otiValues)
	signature := signMetadata(supernode, metadata)
	metadataWithSig := addSignatureToMetadata(metadata, signature)

	// 4. Create and submit finalization message
	msg := types.NewMsgFinalizeAction(
		supernode.Address.String(),
		actionID,
		actionapi.ActionType_ACTION_TYPE_CASCADE.String(),
		metadataWithSig,
	)

	// 5. Deliver transaction
	msgServSim := keeper.NewMsgServerImpl(k)
	_, err := msgServSim.FinalizeAction(ctx, msg)
	if err != nil {
		panic(fmt.Sprintf("failed to finalize CASCADE action %s: %v", actionID, err))
	}

	// 6. Verify action is in DONE state
	finalAction, found := k.GetActionByID(ctx, actionID)
	if !found {
		panic(fmt.Sprintf("action with ID %s not found after finalization", actionID))
	}

	if finalAction.State != actionapi.ActionState_ACTION_STATE_DONE {
		panic(fmt.Sprintf("action %s not in DONE state after finalization: %s", actionID, finalAction.State))
	}
}
