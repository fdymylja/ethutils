package nodeop

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/utils"
	"math/big"
)

type BlockPuller interface {
	BlockByHash(ctx context.Context, hash common.Hash) (block *types.Block, err error)
	BlockByNumber(ctx context.Context, n *big.Int) (block *types.Block, err error)
}

// DownloadBlock downloads a block given an EthClient and the block hash
func DownloadBlock(ctx context.Context, client BlockPuller, hash common.Hash) (block *types.Block, err error) {
	defer utils.WrapErrorP(&err)
	block, err = client.BlockByHash(ctx, hash)
	return
}

// DownloadBlockByNumber downloads a block given an EthClient and blocks number
func DownloadBlockByNumber(ctx context.Context, client BlockPuller, blockNumber uint64) (block *types.Block, err error) {
	defer utils.WrapErrorP(&err)
	bn := new(big.Int).SetUint64(blockNumber)
	block, err = client.BlockByNumber(ctx, bn)
	return
}
