package app

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"runtime/debug"
	"strings"
	"testing"

	dbm "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	simcli "github.com/cosmos/cosmos-sdk/x/simulation/client/cli"
	"github.com/stretchr/testify/require"
	"github.com/CosmWasm/wasmd/x/wasm"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

const SimAppChainID = "simulation-app"

func init() {
	simcli.GetSimulatorFlags()
}

type StoreKeysPrefixes struct {
	A        storetypes.StoreKey
	B        storetypes.StoreKey
	Prefixes [][]byte
}

var (
	// Common test options
	defaultTestOptions = []func(*baseapp.BaseApp){
		baseapp.SetChainID(SimAppChainID),
	}

	storeKeyPrefixesConfig = getStoreKeyPrefixes()
)

func TestFullAppSimulation(t *testing.T) {
	config, db, appOpts, app := setupSimulationApp(t, "skipping application simulation")
	defer cleanupSimulation(t, db, appOpts[flags.FlagHome].(string))

	simResult, simParams, err := runSimulation(t, app, config)
	require.NoError(t, err)

	err = simtestutil.CheckExportSimulation(app, config, simParams)
	require.NoError(t, err)

	if config.Commit {
		simtestutil.PrintStats(db)
	}
}

func TestAppImportExport(t *testing.T) {
	config, db, appOpts, app := setupSimulationApp(t, "skipping application import/export simulation")
	defer cleanupSimulation(t, db, appOpts[flags.FlagHome].(string))

	// Run initial simulation
	_, simParams, err := runSimulation(t, app, config)
	require.NoError(t, err)
	require.NoError(t, simtestutil.CheckExportSimulation(app, config, simParams))

	if config.Commit {
		simtestutil.PrintStats(db)
	}

	// Export state
	exported, err := app.ExportAppStateAndValidators(false, []string{}, []string{})
	require.NoError(t, err)

	// Import into new instance
	newDB, newDir, newApp := setupNewAppInstance(t, config, exported, appOpts)
	defer func() {
		require.NoError(t, newDB.Close())
		require.NoError(t, os.RemoveAll(newDir))
	}()

	// Compare states
	compareStores(t, app, newApp)
}

func TestAppSimulationAfterImport(t *testing.T) {
	config, db, appOpts, app := setupSimulationApp(t, "skipping application simulation after import")
	defer cleanupSimulation(t, db, appOpts[flags.FlagHome].(string))

	// Run initial simulation
	stopEarly, simParams, err := runSimulation(t, app, config)
	require.NoError(t, err)
	require.NoError(t, simtestutil.CheckExportSimulation(app, config, simParams))

	if stopEarly {
		t.Log("can't export or import a zero-validator genesis, exiting test...")
		return
	}

	// Export state
	exported, err := app.ExportAppStateAndValidators(true, []string{}, []string{})
	require.NoError(t, err)

	// Import and simulate again
	newDB, newDir, newApp := setupNewAppInstance(t, config, exported, appOpts)
	defer func() {
		require.NoError(t, newDB.Close())
		require.NoError(t, os.RemoveAll(newDir))
	}()

	_, _, err = runSimulation(t, newApp, config)
	require.NoError(t, err)
}

func TestAppStateDeterminism(t *testing.T) {
	if !simcli.FlagEnabledValue {
		t.Skip("skipping application simulation")
	}

	config := getSimConfig()
	appOpts := getDefaultAppOptions(t)

	for seed := 0; seed < 3; seed++ {
		config.Seed = rand.Int63()
		appHashes := make([]json.RawMessage, 5)

		for attempt := 0; attempt < 5; attempt++ {
			app := createNewTestApp(t, config, appOpts)
			hash, err := simulateAndGetHash(t, app, config)
			require.NoError(t, err)

			appHashes[attempt] = hash
			if attempt > 0 {
				require.Equal(t, string(appHashes[0]), string(hash),
					"non-determinism in seed %d: attempt: %d", config.Seed, attempt+1)
			}
		}
	}
}

// Helper functions

func setupSimulationApp(t *testing.T, skipMsg string) (simtypes.Config, dbm.DB, simtestutil.AppOptionsMap, *WasmApp) {
	config := simcli.NewConfigFromFlags()
	config.ChainID = SimAppChainID

	db, dir, logger, skip, err := simtestutil.SetupSimulation(config, "leveldb-app-sim", "Simulation", simcli.FlagVerboseValue, simcli.FlagEnabledValue)
	if skip {
		t.Skip(skipMsg)
	}
	require.NoError(t, err, "simulation setup failed")

	appOpts := getBaseAppOptions(dir)
	app := NewWasmApp(logger, db, nil, true, wasm.EnableAllProposals, appOpts, emptyWasmOpts, append(defaultTestOptions, fauxMerkleModeOpt)...)
	require.Equal(t, "WasmApp", app.Name())

	return config, db, appOpts, app
}

func runSimulation(t *testing.T, app *WasmApp, config simtypes.Config) (bool, simtypes.Params, error) {
	return simulation.SimulateFromSeed(
		t,
		os.Stdout,
		app.BaseApp,
		simtestutil.AppStateFn(app.AppCodec(), app.SimulationManager(), app.DefaultGenesis()),
		simtypes.RandomAccounts,
		simtestutil.SimulationOperations(app, app.AppCodec(), config),
		BlockedAddresses(),
		config,
		app.AppCodec(),
	)
}

func setupNewAppInstance(t *testing.T, config simtypes.Config, exported servertypes.ExportedApp, appOpts simtestutil.AppOptionsMap) (dbm.DB, string, *WasmApp) {
	newDB, newDir, _, _, err := simtestutil.SetupSimulation(config, "leveldb-app-sim-2", "Simulation-2", simcli.FlagVerboseValue, simcli.FlagEnabledValue)
	require.NoError(t, err, "simulation setup failed")

	newApp := NewWasmApp(log.NewNopLogger(), newDB, nil, true, wasm.EnableAllProposals, appOpts, emptyWasmOpts, append(defaultTestOptions, fauxMerkleModeOpt)...)
	
	var genesisState GenesisState
	err = json.Unmarshal(exported.AppState, &genesisState)
	require.NoError(t, err)

	ctxB := newApp.NewContext(true, tmproto.Header{Height: app.LastBlockHeight()})
	newApp.ModuleManager.InitGenesis(ctxB, app.AppCodec(), genesisState)
	newApp.StoreConsensusParams(ctxB, exported.ConsensusParams)

	return newDB, newDir, newApp
}

func compareStores(t *testing.T, app, newApp *WasmApp) {
	ctxA := app.NewContext(true, tmproto.Header{Height: app.LastBlockHeight()})
	ctxB := newApp.NewContext(true, tmproto.Header{Height: app.LastBlockHeight()})

	for _, skp := range storeKeyPrefixesConfig {
		storeA := ctxA.KVStore(skp.A)
		storeB := ctxB.KVStore(skp.B)
		
		require.NotNil(t, storeA)
		require.NotNil(t, storeB)
		
		failedKVAs, failedKVBs := sdk.DiffKVStores(storeA, storeB, skp.Prefixes)
		require.Equal(t, len(failedKVAs), len(failedKVBs), "unequal sets of key-values to compare")
		require.Equal(t, 0, len(failedKVAs), simtestutil.GetSimulationLog(skp.A.Name(), app.SimulationManager().StoreDecoders, failedKVAs, failedKVBs))
	}
}

func cleanupSimulation(t *testing.T, db dbm.DB, dir string) {
	require.NoError(t, db.Close())
	require.NoError(t, os.RemoveAll(dir))
}

func getSimConfig() simtypes.Config {
	config := simcli.NewConfigFromFlags()
	config.InitialBlockHeight = 1
	config.ExportParamsPath = ""
	config.OnOperation = false
	config.AllInvariants = false
	config.ChainID = SimAppChainID
	return config
}

func getDefaultAppOptions(t *testing.T) simtestutil.AppOptionsMap {
	appOpts := make(simtestutil.AppOptionsMap, 0)
	appOpts[flags.FlagHome] = t.TempDir()
	appOpts[server.FlagInvCheckPeriod] = simcli.FlagPeriodValue
	return appOpts
}

func createNewTestApp(t *testing.T, config simtypes.Config, appOpts simtestutil.AppOptionsMap) *WasmApp {
	logger := getLogger()
	db := dbm.NewMemDB()
	return NewWasmApp(logger, db, nil, true, wasm.EnableAllProposals, appOpts, emptyWasmOpts, append(defaultTestOptions, interBlockCacheOpt())...)
}

func getLogger() log.Logger {
	if simcli.FlagVerboseValue {
		return log.TestingLogger()
	}
	return log.NewNopLogger()
}

func simulateAndGetHash(t *testing.T, app *WasmApp, config simtypes.Config) (json.RawMessage, error) {
	_, _, err := runSimulation(t, app, config)
	if err != nil {
		return nil, err
	}
	return app.LastCommitID().Hash, nil
}
