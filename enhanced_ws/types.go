package geyser_enhanced_ws

// TransactionSubscribeFilter defines the filter parameters for transaction subscriptions.
type TransactionSubscribeFilter struct {
	Vote            *bool    `json:"vote,omitempty"`
	Failed          *bool    `json:"failed,omitempty"`
	Signature       *string  `json:"signature,omitempty"`
	AccountInclude  []string `json:"accountInclude,omitempty"`
	AccountExclude  []string `json:"accountExclude,omitempty"`
	AccountRequired []string `json:"accountRequired,omitempty"`
}

// TransactionSubscribeOptions defines the options for transaction subscriptions.
type TransactionSubscribeOptions struct {
	Commitment                     string `json:"commitment,omitempty"`
	Encoding                       string `json:"encoding,omitempty"`
	TransactionDetails             string `json:"transactionDetails,omitempty"`
	ShowRewards                    *bool  `json:"showRewards,omitempty"`
	MaxSupportedTransactionVersion *uint8 `json:"maxSupportedTransactionVersion,omitempty"`
}

// AccountSubscribeOptions defines the options for account subscriptions.
type AccountSubscribeOptions struct {
	Encoding   string `json:"encoding,omitempty"`
	Commitment string `json:"commitment,omitempty"`
}

// AccountNotification represents an account notification message.
type AccountNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Subscription int64 `json:"subscription"`
		Result       struct {
			Context struct {
				Slot uint64 `json:"slot"`
			} `json:"context"`
			Value struct {
				Lamports   uint64      `json:"lamports"`
				Data       interface{} `json:"data"` // Depending on encoding
				Owner      string      `json:"owner"`
				Executable bool        `json:"executable"`
				RentEpoch  uint64      `json:"rentEpoch"`
				Space      uint64      `json:"space"`
			} `json:"value"`
		} `json:"result"`
	} `json:"params"`
}

// TransactionNotification represents a transaction notification message.
type TransactionNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Subscription int64 `json:"subscription"`
		Result       struct {
			Transaction struct {
				Transaction []interface{} `json:"transaction"`
				Meta        struct {
					Err                  interface{}   `json:"err"`
					Status               interface{}   `json:"status"`
					Fee                  uint64        `json:"fee"`
					PreBalances          []uint64      `json:"preBalances"`
					PostBalances         []uint64      `json:"postBalances"`
					InnerInstructions    []interface{} `json:"innerInstructions"`
					LogMessages          []string      `json:"logMessages"`
					PreTokenBalances     []interface{} `json:"preTokenBalances"`
					PostTokenBalances    []interface{} `json:"postTokenBalances"`
					Rewards              interface{}   `json:"rewards"`
					LoadedAddresses      interface{}   `json:"loadedAddresses"`
					ComputeUnitsConsumed uint64        `json:"computeUnitsConsumed"`
				} `json:"meta"`
			} `json:"transaction"`
			Signature string `json:"signature"`
			Slot      uint64 `json:"slot"`
		} `json:"result"`
	} `json:"params"`
}
