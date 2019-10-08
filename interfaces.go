// interfaces.go lists all the interfaces used by the package
package ethutils

import (
	"context"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

// Node defines behaviour of ethereum queried nodes
type Node interface {
	BlockPuller
	Close()
	SubscribeNewHead(ctx context.Context, headers chan<- *types.Header) (sub ethereum.Subscription, err error)
}

// BlockPuller defines the behaviour of the node used to query the ethereum network
type BlockPuller interface {
	// BlockByHash pulls a block from the ethereum network given its block hash
	BlockByHash(ctx context.Context, hash common.Hash) (block *types.Block, err error)
	// BlockByNumber pulls a block the ethereum network given its block number
	BlockByNumber(ctx context.Context, n *big.Int) (block *types.Block, err error)
}
