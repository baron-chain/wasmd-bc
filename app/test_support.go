package app

import (
	"github.com/cosmos/cosmos-sdk/baseapp"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	ibckeeper "github.com/cosmos/ibc-go/v7/modules/core/keeper"

	"github.com/CosmWasm/wasmd/x/wasm"
)

// GetIBCKeeper returns the IBC keeper instance.
func (app *WasmApp) GetIBCKeeper() *ibckeeper.Keeper {
	return app.IBCKeeper
}

// GetScopedIBCKeeper returns the scoped IBC keeper instance for capability management.
func (app *WasmApp) GetScopedIBCKeeper() capabilitykeeper.ScopedKeeper {
	return app.ScopedIBCKeeper
}

// GetBaseApp returns the BaseApp instance.
func (app *WasmApp) GetBaseApp() *baseapp.BaseApp {
	return app.BaseApp
}

// GetBankKeeper returns the Bank keeper instance.
func (app *WasmApp) GetBankKeeper() bankkeeper.Keeper {
	return app.BankKeeper
}

// GetStakingKeeper returns the Staking keeper instance.
func (app *WasmApp) GetStakingKeeper() *stakingkeeper.Keeper {
	return app.StakingKeeper
}

// GetAccountKeeper returns the Account keeper instance.
func (app *WasmApp) GetAccountKeeper() authkeeper.AccountKeeper {
	return app.AccountKeeper
}

// GetWasmKeeper returns the Wasm keeper instance.
func (app *WasmApp) GetWasmKeeper() wasm.Keeper {
	return app.WasmKeeper
}
