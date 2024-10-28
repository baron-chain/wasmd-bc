package app

import (
	"encoding/json"

	"github.com/cosmos/cosmos-sdk/codec"
)

// GenesisState represents the initial state of the blockchain, where each module's
// genesis data is stored as a raw JSON message, mapped by module name.
type GenesisState map[string]json.RawMessage

// NewDefaultGenesisState generates the default genesis state for the application
// by using the ModuleBasicManager to retrieve the default genesis for each module.
func NewDefaultGenesisState(cdc codec.JSONCodec) GenesisState {
	return ModuleBasics.DefaultGenesis(cdc)
}
