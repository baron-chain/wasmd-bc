/*
Package params defines the simulation parameters in Gaia.

It contains the default weights used for each transaction in the module's
simulation. These weights determine the probability of a transaction being simulated
for any given operation.

You can replace the default values of the weights by providing a `params.json` file
with the weights defined for each transaction operation:

	{
		"op_weight_msg_send": 60,
		"op_weight_msg_delegate": 100
	}

In the example above, the `MsgSend` has a 60% chance of being simulated, while the
`MsgDelegate` will always be simulated (100% chance).
*/
package params
