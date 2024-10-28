package params

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
)

// EncodingConfig defines the encoding configuration for an app, 
// specifying the codecs and transaction configurations for both Protobuf and Amino.
type EncodingConfig struct {
	InterfaceRegistry types.InterfaceRegistry  // Registry for interface types used in Protobuf encoding
	Marshaler         codec.Codec             // Codec for marshaling and unmarshaling Protobuf messages
	TxConfig          client.TxConfig         // Transaction configuration for signing and encoding transactions
	Amino             *codec.LegacyAmino      // Legacy Amino codec for backward compatibility
}
