package interfaces

import "github.com/ethereum/go-ethereum/core/types"

// TODO move this type back to awaiter after turning it into an interface
// TxWithBlock represents the data structure forwarded by Stream on incoming transaction
type TxWithBlock struct {
	// Transaction is the transaction
	Transaction *types.Transaction
	// BlockNumber is the block number the tx was included in
	BlockNumber uint64
	// Timestamp is the timestamp of the block
	Timestamp uint64
}
