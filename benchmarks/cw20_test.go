package benchmarks

// CW20 Message Types

type (
    // cw20InitMsg represents the instantiation message for a CW20 contract
    cw20InitMsg struct {
        Name            string    `json:"name"`              // Token name
        Symbol          string    `json:"symbol"`            // Token symbol
        Decimals        uint8     `json:"decimals"`         // Number of decimal places
        InitialBalances []balance `json:"initial_balances"` // Initial token distribution
    }

    // balance represents a token balance for a specific address
    balance struct {
        Address string `json:"address"`        // Account address holding tokens
        Amount  uint64 `json:"amount,string"`  // Token amount as string for JSON numerical compatibility
    }

    // cw20ExecMsg represents execution messages for a CW20 contract
    cw20ExecMsg struct {
        Transfer *transferMsg `json:"transfer,omitempty"` // Transfer execution, optional
    }

    // transferMsg represents a token transfer between addresses
    transferMsg struct {
        Recipient string `json:"recipient"`       // Recipient address
        Amount    uint64 `json:"amount,string"`   // Transfer amount as string for JSON numerical compatibility
    }
)
