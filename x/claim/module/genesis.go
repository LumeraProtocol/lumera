package claim

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"github.com/pastelnetwork/pastel/x/claim/keeper"
	"github.com/pastelnetwork/pastel/x/claim/types"
)

// InitGenesis initializes the module's state from a provided genesis state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	err := genState.Validate()
	if err != nil {
		panic(fmt.Sprintf("failed to validate genesis state: %s", err))
	}

	if err = k.SetParams(ctx, genState.Params); err != nil {
		panic(err)
	}

	if err := initModuleAccount(ctx, k); err != nil {
		panic(fmt.Sprintf("failed to initialize module account: %s", err))
	}

	// Only attempt to load CSV records if TotalClaimableAmount > 0
	if genState.TotalClaimableAmount > 0 {
		records, err := loadClaimRecordsFromCSV()
		if err != nil {
			if !os.IsNotExist(err) {
				panic(fmt.Sprintf("failed to load CSV: %s", err))
			}
			// If file doesn't exist and amount > 0, this is an error
			panic("CSV file not found but TotalClaimableAmount > 0")
		}

		totalCoins := math.NewInt(0)
		for _, record := range records {
			if err := k.SetClaimRecord(ctx, record); err != nil {
				panic(fmt.Sprintf("failed to set claim record: %s", err))
			}
			totalCoins = totalCoins.Add(record.Balance.AmountOf(types.DefaultDenom))
		}

		// Only check and mint coins if we have a positive total
		if totalCoins.IsPositive() {
			if totalCoins.Uint64() != genState.TotalClaimableAmount {
				panic(fmt.Sprintf("total coins in CSV (%s) does not match total claimable amount in genesis (%d)",
					totalCoins, genState.TotalClaimableAmount))
			}

			bankKeeper := k.GetBankKeeper()
			if err := bankKeeper.MintCoins(
				ctx,
				types.ModuleName,
				sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, totalCoins)),
			); err != nil {
				panic(fmt.Sprintf("failed to mint coins: %s", err))
			}
		}
	}
}

func initModuleAccount(ctx context.Context, k keeper.Keeper) error {
	accountKeeper := k.GetAccountKeeper()
	acc := accountKeeper.GetModuleAccount(ctx, types.ModuleName)
	if acc != nil {
		return nil // Module account already exists
	}

	moduleAcc := authtypes.NewEmptyModuleAccount(
		types.ModuleName,
		authtypes.Minter,
		authtypes.Burner,
	)

	accountKeeper.SetModuleAccount(ctx, moduleAcc)
	return nil
}

// ExportGenesis returns the module's exported genesis.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	genesis := types.DefaultGenesis()
	genesis.Params = k.GetParams(ctx)

	// this line is used by starport scaffolding # genesis/module/export

	return genesis
}

func loadClaimRecordsFromCSV() ([]types.ClaimRecord, error) {
	filepath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	records := make([]types.ClaimRecord, 0, len(rows)-1) // Pre-allocate with capacity
	for _, row := range rows {
		if len(row) < 2 { // Minimum required fields: address and balance
			panic(fmt.Sprintf("invalid CSV row: %v", row))
		}

		balance, ok := math.NewIntFromString(row[1]) // Balance is in second column
		if !ok {
			panic(fmt.Sprintf("invalid balance in CSV row: %v", row))
		}

		coin := sdk.NewCoin(types.DefaultDenom, balance)

		records = append(records, types.ClaimRecord{
			OldAddress: row[0], // Address is in first column
			Balance:    sdk.NewCoins(coin),
			Claimed:    false,
			ClaimTime:  0,
		})
	}

	return records, nil
}

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Define potential paths
	paths := []string{
		filepath.Join(homeDir, ".pastel", "config", "claims.csv"),
		filepath.Join(homeDir, "claims.csv"),
		"../../claims.csv",
		"../../../claims.csv",
	}

	// Check each path in order
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Return the default path with an error indicating file not found
	return paths[0], fmt.Errorf("claims file not found in any location")
}
