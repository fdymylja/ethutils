package interfaces

// interfaces.go lists all the interfaces used by the package

import (
	"context"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

// Node defines a subset of the RPCs that can be made to ethereum nodes
type Node interface {
	BlockPuller
	Close()
	SubscribeNewHead(ctx context.Context, headers chan<- *types.Header) (sub ethereum.Subscription, err error)
}

// BlockPuller defines the behaviour of the types used to query ethereum blocks
type BlockPuller interface {
	// BlockByHash pulls a block from the ethereum network given its block hash
	BlockByHash(ctx context.Context, hash common.Hash) (block *types.Block, err error)
	// BlockByNumber pulls a block the ethereum network given its block number
	BlockByNumber(ctx context.Context, n *big.Int) (block *types.Block, err error)
}

// TransactionPuller defines the behaviour of the types used to query ethereum transactions
type TransactionPuller interface {
	// TransactionByHash gets a transaction from its hash
	TransactionByHash(ctx context.Context, hash common.Hash) (tx *types.Transaction, isPending bool, err error)
}

// Streamer implements blocks, headers and transactions streaming coming from an ethereum node
type Streamer interface {
	Block() <-chan *types.Block       // Block returns new blocks coming from the ethereum network
	Header() <-chan *types.Header     // Header returns new headers coming from the ethereum network
	Transaction() <-chan *TxWithBlock // Transaction streams new transactions coming from the ethereum network
	Err() <-chan error                // Err returns errors coming from the underlying streaming client
	Close() error                     // Close allows to turn the streamer off
}
