package mocks

import (
	"context"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fdymylja/ethutils/nodeop"
)

// BlockLoader defines a function that loads ethereum blocks
type BlockLoader func() ([]*types.Block, error)

// ropstenLoadBlocks downloads blocks from blocks from ropsten ethereum network
func ropstenLoadBlocks(start, finish uint64) BlockLoader {
	return func() (blocks []*types.Block, err error) {
		c, err := ethclient.Dial("wss://ropsten.infura.io/ws/v3/38c930aee8474fbea8f3b33689faf8c9")
		if err != nil {
			panic(err)
		}
		blocks, err = nodeop.DownloadBlocksByRange(context.Background(), c, start, finish)
		return
	}
}