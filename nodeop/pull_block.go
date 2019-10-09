package nodeop

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/utils"
	"math/big"
)

// DownloadBlock downloads a block given an EthClient and the block hash
func DownloadBlock(ctx context.Context, client interfaces.BlockPuller, hash common.Hash) (block *types.Block, err error) {
	defer utils.WrapErrorP(&err)
	block, err = client.BlockByHash(ctx, hash)
	return
}

// DownloadBlockByNumber downloads a block given an EthClient and blocks number
func DownloadBlockByNumber(ctx context.Context, client interfaces.BlockPuller, blockNumber uint64) (block *types.Block, err error) {
	defer utils.WrapErrorP(&err)
	bn := new(big.Int).SetUint64(blockNumber)
	block, err = client.BlockByNumber(ctx, bn)
	return
}

// DownloadBlocksByRange downloads blocks from a BlockPuller given a starting and an ending block number
func DownloadBlocksByRange(ctx context.Context, client interfaces.BlockPuller, blockStart, blockFinish uint64) (blocks []*types.Block, err error) {
	defer utils.WrapErrorP(&err)
	if blockStart > blockFinish {
		panic("starting block bigger than finishing block")
	}
	blocks = make([]*types.Block, blockFinish-blockStart+1)
	for i := 0; blockStart <= blockFinish; blockStart++ {
		blocks[i], err = DownloadBlockByNumber(ctx, client, blockStart)
		if err != nil {
			return
		}
		i++
	}
	return
}
