package benchmarks

import (
	"encoding/json"
	"math/rand"
	"os"
	"testing"
	"time"

	dbm "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/CosmWasm/wasmd/app"
	"github.com/CosmWasm/wasmd/x/wasm"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

const (
	defaultInitialBalance = 100000000000
	defaultGasWanted     = 500000
)

type (
	AppInfo struct {
		App          *app.WasmApp
		MinterKey    *secp256k1.PrivKey
		MinterAddr   sdk.AccAddress
		ContractAddr string
		Denom        string
		AccNum       uint64
		SeqNum       uint64
		TxConfig     client.TxConfig
	}

	balance struct {
		Address string `json:"address"`
		Amount  int64  `json:"amount"`
	}

	cw20InitMsg struct {
		Name            string    `json:"name"`
		Symbol          string    `json:"symbol"`
		Decimals        uint8     `json:"decimals"`
		InitialBalances []balance `json:"initial_balances"`
	}
)

func setupBaseApp(db dbm.DB, withGenesis bool) (*app.WasmApp, app.GenesisState) {
	wasmApp := app.NewWasmApp(
		log.NewTMLogger(log.NewSyncWriter(os.Stdout)),
		db,
		nil,
		true,
		wasm.EnableAllProposals,
		simtestutil.EmptyAppOptions{},
		nil,
	)

	if !withGenesis {
		return wasmApp, app.GenesisState{}
	}
	return wasmApp, app.NewDefaultGenesisState(wasmApp.AppCodec())
}

func SetupWithGenesisAccountsAndValSet(b testing.TB, db dbm.DB, genAccs []authtypes.GenesisAccount, balances ...banktypes.Balance) *app.WasmApp {
	wasmApp, genesisState := setupBaseApp(db, true)
	appCodec := wasmApp.AppCodec()

	// Setup validator set
	validator, valSet := createValidatorSet(b)
	
	// Initialize genesis state
	genesisState = initializeGenesisState(appCodec, genesisState, genAccs, validator, valSet, balances)

	// Initialize chain
	initializeChain(wasmApp, genesisState)

	return wasmApp
}

func createValidatorSet(b testing.TB) (*tmtypes.Validator, *tmtypes.ValidatorSet) {
	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(b, err)

	validator := tmtypes.NewValidator(pubKey, 1)
	return validator, tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})
}

func initializeGenesisState(
	appCodec codec.Codec,
	genesisState app.GenesisState,
	genAccs []authtypes.GenesisAccount,
	validator *tmtypes.Validator,
	valSet *tmtypes.ValidatorSet,
	balances []banktypes.Balance,
) app.GenesisState {
	// Auth genesis
	authGenesis := authtypes.NewGenesisState(authtypes.DefaultParams(), genAccs)
	genesisState[authtypes.ModuleName] = appCodec.MustMarshalJSON(authGenesis)

	// Staking genesis
	validators, delegations := createValidatorsAndDelegations(validator, valSet, genAccs[0])
	stakingGenesis := stakingtypes.NewGenesisState(stakingtypes.DefaultParams(), validators, delegations)
	genesisState[stakingtypes.ModuleName] = appCodec.MustMarshalJSON(stakingGenesis)

	// Bank genesis
	genesisState[banktypes.ModuleName] = appCodec.MustMarshalJSON(createBankGenesis(balances, validator))

	return genesisState
}

func createValidatorsAndDelegations(
	validator *tmtypes.Validator,
	valSet *tmtypes.ValidatorSet,
	firstAcc authtypes.GenesisAccount,
) ([]stakingtypes.Validator, []stakingtypes.Delegation) {
	bondAmt := sdk.DefaultPowerReduction
	validators := make([]stakingtypes.Validator, 0, len(valSet.Validators))
	delegations := make([]stakingtypes.Delegation, 0, len(valSet.Validators))

	pk, _ := cryptocodec.FromTmPubKeyInterface(validator.PubKey)
	pkAny, _ := codectypes.NewAnyWithValue(pk)
	
	validator := stakingtypes.Validator{
		OperatorAddress:   sdk.ValAddress(validator.Address).String(),
		ConsensusPubkey:   pkAny,
		Jailed:            false,
		Status:            stakingtypes.Bonded,
		Tokens:            bondAmt,
		DelegatorShares:   sdk.OneDec(),
		Description:       stakingtypes.Description{},
		UnbondingHeight:   int64(0),
		UnbondingTime:     time.Unix(0, 0).UTC(),
		Commission:        stakingtypes.NewCommission(sdk.ZeroDec(), sdk.ZeroDec(), sdk.ZeroDec()),
		MinSelfDelegation: sdk.ZeroInt(),
	}

	validators = append(validators, validator)
	delegations = append(delegations, stakingtypes.NewDelegation(
		firstAcc.GetAddress(),
		validator.Address.Bytes(),
		sdk.OneDec(),
	))

	return validators, delegations
}

func createBankGenesis(balances []banktypes.Balance, validator *tmtypes.Validator) *banktypes.GenesisState {
	bondAmt := sdk.DefaultPowerReduction
	
	// Add bonded amount to bonded pool module account
	balances = append(balances, banktypes.Balance{
		Address: authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(),
		Coins:   sdk.Coins{sdk.NewCoin(sdk.DefaultBondDenom, bondAmt)},
	})

	totalSupply := calculateTotalSupply(balances)
	return banktypes.NewGenesisState(
		banktypes.DefaultGenesisState().Params,
		balances,
		totalSupply,
		[]banktypes.Metadata{},
		nil,
	)
}

func calculateTotalSupply(balances []banktypes.Balance) sdk.Coins {
	totalSupply := sdk.NewCoins()
	for _, balance := range balances {
		totalSupply = totalSupply.Add(balance.Coins...)
	}
	return totalSupply
}

func InitializeWasmApp(b testing.TB, db dbm.DB, numAccounts int) AppInfo {
	minter := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(minter.PubKey().Address())
	denom := "uatom"

	genesisAccounts, balances := createGenesisAccountsAndBalances(addr, denom, numAccounts)
	wasmApp := SetupWithGenesisAccountsAndValSet(b, db, genesisAccounts, balances...)

	// Deploy and initialize contract
	contractAddr := deployAndInitializeContract(b, wasmApp, minter, addr, numAccounts)

	return AppInfo{
		App:          wasmApp,
		MinterKey:    minter,
		MinterAddr:   addr,
		ContractAddr: contractAddr,
		Denom:        denom,
		AccNum:       0,
		SeqNum:       2,
		TxConfig:     moduletestutil.MakeTestEncodingConfig().TxConfig,
	}
}

func createGenesisAccountsAndBalances(addr sdk.AccAddress, denom string, numAccounts int) ([]authtypes.GenesisAccount, []banktypes.Balance) {
	genAccs := make([]authtypes.GenesisAccount, numAccounts+1)
	bals := make([]banktypes.Balance, numAccounts+1)

	// Set up initial account
	genAccs[0] = &authtypes.BaseAccount{Address: addr.String()}
	bals[0] = banktypes.Balance{
		Address: addr.String(),
		Coins:   sdk.NewCoins(sdk.NewInt64Coin(denom, defaultInitialBalance)),
	}

	// Generate random accounts
	for i := 1; i <= numAccounts; i++ {
		acct := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
		genAccs[i] = &authtypes.BaseAccount{Address: acct}
		bals[i] = banktypes.Balance{
			Address: acct,
			Coins:   sdk.NewCoins(sdk.NewInt64Coin(denom, defaultInitialBalance)),
		}
	}

	return genAccs, bals
}

func deployAndInitializeContract(b testing.TB, wasmApp *app.WasmApp, minter *secp256k1.PrivKey, addr sdk.AccAddress, numAccounts int) string {
	height := int64(2)
	txConfig := moduletestutil.MakeTestEncodingConfig().TxConfig
	wasmApp.BeginBlock(abci.RequestBeginBlock{Header: tmproto.Header{Height: height, Time: time.Now()}})

	// Deploy contract
	codeID := deployContract(b, wasmApp, txConfig, minter, addr)
	
	// Initialize contract
	contractAddr := initializeContract(b, wasmApp, txConfig, minter, addr, codeID, numAccounts)

	wasmApp.EndBlock(abci.RequestEndBlock{Height: height})
	wasmApp.Commit()

	return contractAddr
}

func deployContract(b testing.TB, wasmApp *app.WasmApp, txConfig client.TxConfig, minter *secp256k1.PrivKey, addr sdk.AccAddress) uint64 {
	cw20Code, err := os.ReadFile("./testdata/cw20_base.wasm")
	require.NoError(b, err)

	storeMsg := wasmtypes.MsgStoreCode{
		Sender:       addr.String(),
		WASMByteCode: cw20Code,
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	storeTx, err := simtestutil.GenSignedMockTx(r, txConfig, []sdk.Msg{&storeMsg}, nil, 55123123, "", []uint64{0}, []uint64{0}, minter)
	require.NoError(b, err)

	_, _, err = wasmApp.SimDeliver(txConfig.TxEncoder(), storeTx)
	require.NoError(b, err)

	return 1 // First deployed contract gets ID 1
}

func GenSequenceOfTxs(b testing.TB, info *AppInfo, msgGen func(*AppInfo) ([]sdk.Msg, error), numToGenerate int) []sdk.Tx {
	fees := sdk.Coins{sdk.NewInt64Coin(info.Denom, 0)}
	txs := make([]sdk.Tx, numToGenerate)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < numToGenerate; i++ {
		msgs, err := msgGen(info)
		require.NoError(b, err)

		txs[i], err = simtestutil.GenSignedMockTx(
			r,
			info.TxConfig,
			msgs,
			fees,
			1234567,
			"",
			[]uint64{info.AccNum},
			[]uint64{info.SeqNum},
			info.MinterKey,
		)
		require.NoError(b, err)
		info.SeqNum++
	}

	return txs
}
