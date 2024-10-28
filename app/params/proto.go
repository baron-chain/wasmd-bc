package params

import (
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
)

// MakeEncodingConfig creates an EncodingConfig for the application using Amino and Proto codecs.
func MakeEncodingConfig() EncodingConfig {
	// Initialize the legacy Amino codec (for backward compatibility)
	amino := codec.NewLegacyAmino()

	// Create a new interface registry for registering interface types
	interfaceRegistry := types.NewInterfaceRegistry()

	// Create a new protocol buffer-based marshaler
	marshaler := codec.NewProtoCodec(interfaceRegistry)

	// Set up the transaction configuration with default sign modes
	txCfg := tx.NewTxConfig(marshaler, tx.DefaultSignModes)

	// Return the EncodingConfig containing the codecs and transaction config
	return EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Marshaler:         marshaler,
		TxConfig:          txCfg,
		Amino:             amino,
	}
}
