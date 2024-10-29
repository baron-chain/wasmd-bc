package app

import (
	"encoding/json"
	"fmt"
	"log"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (app *WasmApp) ExportAppStateAndValidators(
	forZeroHeight bool,
	jailAllowedAddrs []string,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	ctx := app.NewContext(true, tmproto.Header{Height: app.LastBlockHeight()})
	height := app.LastBlockHeight() + 1

	if forZeroHeight {
		height = 0
		app.prepForZeroHeightGenesis(ctx, jailAllowedAddrs)
	}

	genState := app.ModuleManager.ExportGenesisForModules(ctx, app.appCodec, modulesToExport)
	appState, err := json.MarshalIndent(genState, "", "  ")
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	validators, err := staking.WriteValidators(ctx, app.StakingKeeper)
	return servertypes.ExportedApp{
		AppState:        appState,
		Validators:      validators,
		Height:          height,
		ConsensusParams: app.BaseApp.GetConsensusParams(ctx),
	}, err
}

func (app *WasmApp) prepForZeroHeightGenesis(ctx sdk.Context, jailAllowedAddrs []string) {
	allowedAddrsMap := buildAllowedAddressMap(jailAllowedAddrs)
	
	app.CrisisKeeper.AssertInvariants(ctx)
	
	app.handleDistributionState(ctx)
	
	// Preserve height for later restoration
	height := ctx.BlockHeight()
	ctx = ctx.WithBlockHeight(0)
	
	app.handleStakingState(ctx, allowedAddrsMap)
	
	app.handleSlashingState(ctx)
	
	// Restore original height
	ctx = ctx.WithBlockHeight(height)
}

func buildAllowedAddressMap(jailAllowedAddrs []string) map[string]bool {
	allowedAddrsMap := make(map[string]bool, len(jailAllowedAddrs))
	for _, addr := range jailAllowedAddrs {
		_, err := sdk.ValAddressFromBech32(addr)
		if err != nil {
			log.Fatal(err)
		}
		allowedAddrsMap[addr] = true
	}
	return allowedAddrsMap
}

func (app *WasmApp) handleDistributionState(ctx sdk.Context) {
	// Withdraw validator commissions
	app.StakingKeeper.IterateValidators(ctx, func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
		_, _ = app.DistrKeeper.WithdrawValidatorCommission(ctx, val.GetOperator())
		return false
	})

	// Withdraw delegator rewards
	app.withdrawAllDelegatorRewards(ctx)

	// Clear distribution state
	app.DistrKeeper.DeleteAllValidatorSlashEvents(ctx)
	app.DistrKeeper.DeleteAllValidatorHistoricalRewards(ctx)

	// Handle remaining validator rewards
	app.handleRemainingValidatorRewards(ctx)
}

func (app *WasmApp) withdrawAllDelegatorRewards(ctx sdk.Context) {
	dels := app.StakingKeeper.GetAllDelegations(ctx)
	for _, del := range dels {
		valAddr, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
		if err != nil {
			panic(err)
		}
		delAddr := sdk.MustAccAddressFromBech32(del.DelegatorAddress)
		
		if _, err = app.DistrKeeper.WithdrawDelegationRewards(ctx, delAddr, valAddr); err != nil {
			panic(err)
		}
	}
}

func (app *WasmApp) handleRemainingValidatorRewards(ctx sdk.Context) {
	app.StakingKeeper.IterateValidators(ctx, func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
		// Move outstanding rewards to community pool
		scraps := app.DistrKeeper.GetValidatorOutstandingRewardsCoins(ctx, val.GetOperator())
		feePool := app.DistrKeeper.GetFeePool(ctx)
		feePool.CommunityPool = feePool.CommunityPool.Add(scraps...)
		app.DistrKeeper.SetFeePool(ctx, feePool)

		if err := app.DistrKeeper.Hooks().AfterValidatorCreated(ctx, val.GetOperator()); err != nil {
			panic(err)
		}
		return false
	})
}

func (app *WasmApp) handleStakingState(ctx sdk.Context, allowedAddrsMap map[string]bool) {
	// Reset delegation state
	app.resetDelegationState(ctx)

	// Reset redelegations
	app.StakingKeeper.IterateRedelegations(ctx, func(_ int64, red stakingtypes.Redelegation) (stop bool) {
		for i := range red.Entries {
			red.Entries[i].CreationHeight = 0
		}
		app.StakingKeeper.SetRedelegation(ctx, red)
		return false
	})

	// Reset unbonding delegations
	app.StakingKeeper.IterateUnbondingDelegations(ctx, func(_ int64, ubd stakingtypes.UnbondingDelegation) (stop bool) {
		for i := range ubd.Entries {
			ubd.Entries[i].CreationHeight = 0
		}
		app.StakingKeeper.SetUnbondingDelegation(ctx, ubd)
		return false
	})

	app.resetValidatorState(ctx, allowedAddrsMap)
}

func (app *WasmApp) resetDelegationState(ctx sdk.Context) {
	dels := app.StakingKeeper.GetAllDelegations(ctx)
	for _, del := range dels {
		valAddr, err := sdk.ValAddressFromBech32(del.ValidatorAddress)
		if err != nil {
			panic(err)
		}
		delAddr := sdk.MustAccAddressFromBech32(del.DelegatorAddress)

		if err := app.DistrKeeper.Hooks().BeforeDelegationCreated(ctx, delAddr, valAddr); err != nil {
			panic(fmt.Errorf("error while incrementing period: %w", err))
		}

		if err := app.DistrKeeper.Hooks().AfterDelegationModified(ctx, delAddr, valAddr); err != nil {
			panic(fmt.Errorf("error while creating a new delegation period record: %w", err))
		}
	}
}

func (app *WasmApp) resetValidatorState(ctx sdk.Context, allowedAddrsMap map[string]bool) {
	store := ctx.KVStore(app.GetKey(stakingtypes.StoreKey))
	iter := sdk.KVStoreReversePrefixIterator(store, stakingtypes.ValidatorsKey)
	defer func() {
		if err := iter.Close(); err != nil {
			app.Logger().Error("error while closing the key-value store reverse prefix iterator: ", err)
		}
	}()

	for ; iter.Valid(); iter.Next() {
		addr := sdk.ValAddress(stakingtypes.AddressFromValidatorsKey(iter.Key()))
		validator, found := app.StakingKeeper.GetValidator(ctx, addr)
		if !found {
			panic("expected validator, not found")
		}

		validator.UnbondingHeight = 0
		if len(allowedAddrsMap) > 0 && !allowedAddrsMap[addr.String()] {
			validator.Jailed = true
		}

		app.StakingKeeper.SetValidator(ctx, validator)
	}

	if _, err := app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx); err != nil {
		log.Fatal(err)
	}
}

func (app *WasmApp) handleSlashingState(ctx sdk.Context) {
	app.SlashingKeeper.IterateValidatorSigningInfos(
		ctx,
		func(addr sdk.ConsAddress, info slashingtypes.ValidatorSigningInfo) (stop bool) {
			info.StartHeight = 0
			app.SlashingKeeper.SetValidatorSigningInfo(ctx, addr, info)
			return false
		},
	)
}
