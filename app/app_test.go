package app

import (
	"os"
	"testing"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmd/x/wasm"
)

var emptyWasmOpts = []wasm.Option{}

// TestWasmdExport ensures that exporting the app state and validators does not return errors.
func TestWasmdExport(t *testing.T) {
	db := dbm.NewMemDB()

	// Setup the app with custom options
	gapp := NewWasmAppWithCustomOptions(t, false, SetupOptions{
		Logger:  log.NewTMLogger(log.NewSyncWriter(os.Stdout)),
		DB:      db,
		AppOpts: simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
	})
	gapp.Commit()

	// Create a new app instance with the same database
	newGapp := NewWasmApp(
		log.NewTMLogger(log.NewSyncWriter(os.Stdout)), 
		db, 
		nil, 
		true, 
		wasm.EnableAllProposals, 
		simtestutil.NewAppOptionsWithFlagHome(t.TempDir()), 
		emptyWasmOpts,
	)

	// Export app state and validators and ensure no error occurs
	_, err := newGapp.ExportAppStateAndValidators(false, []string{}, nil)
	require.NoError(t, err, "ExportAppStateAndValidators should not return an error")
}

// TestBlockedAddrs verifies that blocked addresses are correctly set in the bank keeper.
func TestBlockedAddrs(t *testing.T) {
	gapp := Setup(t)

	for acc := range BlockedAddresses() {
		t.Run(acc, func(t *testing.T) {
			addr, err := sdk.AccAddressFromBech32(acc)
			if err != nil {
				// Fallback to getting module address if Bech32 decoding fails
				addr = gapp.AccountKeeper.GetModuleAddress(acc)
			}
			require.True(t, gapp.BankKeeper.BlockedAddr(addr), "Blocked addresses must be set correctly in the bank keeper")
		})
	}
}

// TestGetMaccPerms ensures that the module account permissions are correctly duplicated.
func TestGetMaccPerms(t *testing.T) {
	dup := GetMaccPerms()
	require.Equal(t, maccPerms, dup, "Duplicated module account permissions must match actual permissions")
}

// TestGetEnabledProposals verifies that the correct proposals are enabled based on configuration.
func TestGetEnabledProposals(t *testing.T) {
	testCases := map[string]struct {
		proposalsEnabled string
		specificEnabled  string
		expected         []wasm.ProposalType
	}{
		"all disabled": {
			proposalsEnabled: "false",
			expected:         wasm.DisableAllProposals,
		},
		"all enabled": {
			proposalsEnabled: "true",
			expected:         wasm.EnableAllProposals,
		},
		"some enabled": {
			proposalsEnabled: "okay",
			specificEnabled:  "StoreCode,InstantiateContract",
			expected: []wasm.ProposalType{
				wasm.ProposalTypeStoreCode,
				wasm.ProposalTypeInstantiateContract,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ProposalsEnabled = tc.proposalsEnabled
			EnableSpecificProposals = tc.specificEnabled
			proposals := GetEnabledProposals()

			assert.Equal(t, tc.expected, proposals)
		})
	}
}
