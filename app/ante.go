package app

import (
	errorsmod "cosmossdk.io/errors"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	ibcante "github.com/cosmos/ibc-go/v7/modules/core/ante"
	"github.com/cosmos/ibc-go/v7/modules/core/keeper"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmTypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

// HandlerOptions extends the SDK's AnteHandler options by requiring additional fields for IBC and Wasm modules.
type HandlerOptions struct {
	ante.HandlerOptions

	IBCKeeper         *keeper.Keeper
	WasmConfig        *wasmTypes.WasmConfig
	TXCounterStoreKey storetypes.StoreKey
}

// NewAnteHandler creates a new AnteHandler for the application using the provided HandlerOptions.
func NewAnteHandler(options HandlerOptions) (sdk.AnteHandler, error) {
	// Check required dependencies
	if options.AccountKeeper == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "account keeper is required for AnteHandler")
	}
	if options.BankKeeper == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "bank keeper is required for AnteHandler")
	}
	if options.SignModeHandler == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "sign mode handler is required for AnteHandler")
	}
	if options.WasmConfig == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "wasm config is required for AnteHandler")
	}
	if options.TXCounterStoreKey == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrLogic, "tx counter key is required for AnteHandler")
	}

	// Create the sequence of AnteDecorators
	anteDecorators := []sdk.AnteDecorator{
		ante.NewSetUpContextDecorator(), // Must be called first
		wasmkeeper.NewLimitSimulationGasDecorator(options.WasmConfig.SimulationGasLimit), // Enforce gas limits early
		wasmkeeper.NewCountTXDecorator(options.TXCounterStoreKey), // Track transaction counts
		ante.NewExtensionOptionsDecorator(options.ExtensionOptionChecker), // Process extension options
		ante.NewValidateBasicDecorator(), // Basic validation
		ante.NewTxTimeoutHeightDecorator(), // Timeout height check
		ante.NewValidateMemoDecorator(options.AccountKeeper), // Validate transaction memo
		ante.NewConsumeGasForTxSizeDecorator(options.AccountKeeper), // Consume gas based on tx size
		ante.NewDeductFeeDecorator(options.AccountKeeper, options.BankKeeper, options.FeegrantKeeper, options.TxFeeChecker), // Deduct fees
		ante.NewSetPubKeyDecorator(options.AccountKeeper), // Set the public key for the transaction
		ante.NewValidateSigCountDecorator(options.AccountKeeper), // Validate signature count
		ante.NewSigGasConsumeDecorator(options.AccountKeeper, options.SigGasConsumer), // Consume gas for signatures
		ante.NewSigVerificationDecorator(options.AccountKeeper, options.SignModeHandler), // Verify signatures
		ante.NewIncrementSequenceDecorator(options.AccountKeeper), // Increment the sequence number
		ibcante.NewRedundantRelayDecorator(options.IBCKeeper), // Handle redundant relays for IBC
	}

	// Chain the AnteDecorators and return the handler
	return sdk.ChainAnteDecorators(anteDecorators...), nil
}
