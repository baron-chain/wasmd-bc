package app

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cosmossdk.io/math"
	dbm "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	bam "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/snapshots"
	pruningtypes "github.com/cosmos/cosmos-sdk/store/pruning/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmd/x/wasm"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

type (
	SetupOptions struct {
		Logger   log.Logger
		DB       *dbm.MemDB
		AppOpts  servertypes.AppOptions
		WasmOpts []wasm.Option
	}

	GenesisInitOptions struct {
		ValidatorSet      *tmtypes.ValidatorSet
		GenesisAccounts   []authtypes.GenesisAccount
		ChainID          string
		WasmOpts         []wasm.Option
		Balances         []banktypes.Balance
		ConsensusParams  *tmproto.ConsensusParams
		GenesisModifier  func(genesis GenesisState) GenesisState
	}
)

const defaultBondAmount = 100000000000000

func setupBase(t testing.TB, chainID string, withGenesis bool, invCheckPeriod uint, opts ...wasm.Option) (*WasmApp, GenesisState) {
	db := dbm.NewMemDB()
	nodeHome := t.TempDir()
	snapshotDir := filepath.Join(nodeHome, "data", "snapshots")

	snapshotDB, err := dbm.NewDB("metadata", dbm.GoLevelDBBackend, snapshotDir)
	require.NoError(t, err)
	t.Cleanup(func() { snapshotDB.Close() })

	snapshotStore, err := snapshots.NewStore(snapshotDB, snapshotDir)
	require.NoError(t, err)

	appOpts := makeAppOptions(nodeHome, invCheckPeriod)
	app := createApp(db, chainID, snapshotStore, appOpts, opts)

	if !withGenesis {
		return app, GenesisState{}
	}
	return app, NewDefaultGenesisState(app.AppCodec())
}

func createApp(db dbm.DB, chainID string, snapshotStore *snapshots.Store, appOpts servertypes.AppOptions, opts []wasm.Option) *WasmApp {
	return NewWasmApp(
		log.NewNopLogger(),
		db,
		nil,
		true,
		wasmtypes.EnableAllProposals,
		appOpts,
		opts,
		bam.SetChainID(chainID),
		bam.SetSnapshot(snapshotStore, snapshottypes.SnapshotOptions{KeepRecent: 2}),
	)
}

func makeAppOptions(nodeHome string, invCheckPeriod uint) servertypes.AppOptions {
	appOpts := make(simtestutil.AppOptionsMap, 0)
	appOpts[flags.FlagHome] = nodeHome
	appOpts[server.FlagInvCheckPeriod] = invCheckPeriod
	return appOpts
}

func Setup(t *testing.T, opts ...wasm.Option) *WasmApp {
	t.Helper()
	
	validator := createMockValidator(t)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})
	
	account, balance := createGenesisAccount()
	
	return SetupWithGenesisValSet(t, &GenesisInitOptions{
		ValidatorSet:    valSet,
		GenesisAccounts: []authtypes.GenesisAccount{account},
		ChainID:         "testing",
		WasmOpts:       opts,
		Balances:       []banktypes.Balance{balance},
	})
}

func SetupWithGenesisValSet(t *testing.T, options *GenesisInitOptions) *WasmApp {
	t.Helper()

	app, genesisState := setupBase(t, options.ChainID, true, 5, options.WasmOpts...)
	
	genesisState, err := initializeGenesisState(app.AppCodec(), genesisState, options)
	require.NoError(t, err)

	stateBytes, err := json.MarshalIndent(genesisState, "", " ")
	require.NoError(t, err)

	consensusParams := getConsensusParams(options.ConsensusParams)
	
	app.InitChain(createInitChainRequest(options.ChainID, consensusParams, stateBytes))
	initializeAppState(app, options.ChainID, options.ValidatorSet)

	return app
}

func initializeAppState(app *WasmApp, chainID string, valSet *tmtypes.ValidatorSet) {
	app.Commit()
	app.BeginBlock(abci.RequestBeginBlock{
		Header: tmproto.Header{
			ChainID:            chainID,
			Height:             app.LastBlockHeight() + 1,
			AppHash:            app.LastCommitID().Hash,
			Time:              time.Now().UTC(),
			ValidatorsHash:     valSet.Hash(),
			NextValidatorsHash: valSet.Hash(),
		},
	})
}

func createGenesisAccount() (authtypes.GenesisAccount, banktypes.Balance) {
	senderPrivKey := secp256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(defaultBondAmount))),
	}
	return acc, balance
}

func createMockValidator(t *testing.T) *tmtypes.Validator {
	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)
	return tmtypes.NewValidator(pubKey, 1)
}

func AddTestAddrsIncremental(app *WasmApp, ctx sdk.Context, accNum int, accAmt math.Int) []sdk.AccAddress {
	return addTestAddrs(app, ctx, accNum, accAmt, simtestutil.CreateIncrementalAccounts)
}

func addTestAddrs(app *WasmApp, ctx sdk.Context, accNum int, accAmt math.Int, strategy simtestutil.GenerateAccountStrategy) []sdk.AccAddress {
	addrs := strategy(accNum)
	initCoins := sdk.NewCoins(sdk.NewCoin(app.StakingKeeper.BondDenom(ctx), accAmt))

	for _, addr := range addrs {
		initAccountWithCoins(app, ctx, addr, initCoins)
	}
	return addrs
}

func initAccountWithCoins(app *WasmApp, ctx sdk.Context, addr sdk.AccAddress, coins sdk.Coins) {
	require.NoError(t, app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins))
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, coins))
}

func SignAndDeliverWithoutCommit(t *testing.T, txCfg client.TxConfig, app *bam.BaseApp, header tmproto.Header, msgs []sdk.Msg,
	chainID string, accNums, accSeqs []uint64, priv ...cryptotypes.PrivKey) (sdk.GasInfo, *sdk.Result, error) {
	
	tx, err := simtestutil.GenSignedMockTx(
		rand.New(rand.NewSource(time.Now().UnixNano())),
		txCfg,
		msgs,
		sdk.Coins{sdk.NewInt64Coin(sdk.DefaultBondDenom, 0)},
		simtestutil.DefaultGenTxGas,
		chainID,
		accNums,
		accSeqs,
		priv...,
	)
	require.NoError(t, err)

	return app.SimDeliver(txCfg.TxEncoder(), tx)
}

func GenesisStateWithValSet(codec codec.Codec, genesisState map[string]json.RawMessage,
	valSet *tmtypes.ValidatorSet, genAccs []authtypes.GenesisAccount, balances ...banktypes.Balance) (map[string]json.RawMessage, error) {

	authGenesis := createAuthGenesis(genAccs)
	genesisState[authtypes.ModuleName] = codec.MustMarshalJSON(authGenesis)

	validators, delegations := createValidatorsAndDelegations(valSet, genAccs)
	stakingGenesis := createStakingGenesis(validators, delegations)
	genesisState[stakingtypes.ModuleName] = codec.MustMarshalJSON(stakingGenesis)

	// Update total supply with bonded tokens
	bondedBalance := calculateBondedBalance(valSet)
	balances = append(balances, bondedBalance)

	bankGenesis := createBankGenesis(balances)
	genesisState[banktypes.ModuleName] = codec.MustMarshalJSON(bankGenesis)

	return genesisState, nil
}

func calculateBondedBalance(valSet *tmtypes.ValidatorSet) banktypes.Balance {
	bondAmt := sdk.DefaultPowerReduction.MulRaw(int64(len(valSet.Validators)))
	return banktypes.Balance{
		Address: authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(),
		Coins:   sdk.Coins{sdk.NewCoin(sdk.DefaultBondDenom, bondAmt)},
	}
}

func createValidatorsAndDelegations(valSet *tmtypes.ValidatorSet, genAccs []authtypes.GenesisAccount) ([]stakingtypes.Validator, []stakingtypes.Delegation) {
	validators := make([]stakingtypes.Validator, 0, len(valSet.Validators))
	delegations := make([]stakingtypes.Delegation, 0, len(valSet.Validators))

	for _, val := range valSet.Validators {
		validator, err := createValidator(val)
		if err != nil {
			panic(err)
		}
		validators = append(validators, validator)
		delegations = append(delegations, createDelegation(genAccs[0], val))
	}

	return validators, delegations
}

func createValidator(val *tmtypes.Validator) (stakingtypes.Validator, error) {
	pk, err := cryptocodec.FromTmPubKeyInterface(val.PubKey)
	if err != nil {
		return stakingtypes.Validator{}, fmt.Errorf("failed to convert pubkey: %w", err)
	}

	pkAny, err := codectypes.NewAnyWithValue(pk)
	if err != nil {
		return stakingtypes.Validator{}, fmt.Errorf("failed to create new any: %w", err)
	}

	return stakingtypes.Validator{
		OperatorAddress:   sdk.ValAddress(val.Address).String(),
		ConsensusPubkey:   pkAny,
		Jailed:            false,
		Status:            stakingtypes.Bonded,
		Tokens:            sdk.DefaultPowerReduction,
		DelegatorShares:   math.LegacyOneDec(),
		Description:       stakingtypes.Description{},
		UnbondingHeight:   int64(0),
		UnbondingTime:     time.Unix(0, 0).UTC(),
		Commission:        stakingtypes.NewCommission(math.LegacyZeroDec(), math.LegacyZeroDec(), math.LegacyZeroDec()),
		MinSelfDelegation: math.ZeroInt(),
	}, nil
}

func createDelegation(acc authtypes.GenesisAccount, val *tmtypes.Validator) stakingtypes.Delegation {
	return stakingtypes.NewDelegation(acc.GetAddress(), val.Address.Bytes(), math.LegacyOneDec())
}
