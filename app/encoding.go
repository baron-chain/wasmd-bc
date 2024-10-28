package app

import (
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/CosmWasm/wasmd/app/params"
)

// MakeEncodingConfig creates and returns a new EncodingConfig with all modules registered.
func MakeEncodingConfig() params.EncodingConfig {
	encodingConfig := params.MakeEncodingConfig()

	// Register legacy amino codec and interfaces
	std.RegisterLegacyAminoCodec(encodingConfig.Amino)
	std.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	// Register module-specific codecs and interfaces
	ModuleBasics.RegisterLegacyAminoCodec(encodingConfig.Amino)
	ModuleBasics.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	return encodingConfig
}
