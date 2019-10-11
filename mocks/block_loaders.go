package mocks

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/fdymylja/ethutils/nodeop"
	"io/ioutil"
	"os"
	"sync"
	"time"
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

// loadBlock loads a block given a file and the target where the block should be saved TODO generalize with decoder interface
func loadBlock(wg *sync.WaitGroup, errs chan error, dir string, fileName string, blocks []*types.Block, writeAt int) {
	defer wg.Done()
	src := fmt.Sprintf("%s/%s", dir, fileName)
	f, err := os.Open(src)
	if err != nil {
		errs <- err
		return
	}
	defer f.Close()
	block := new(types.Block)
	err = rlp.Decode(f, block)
	if err != nil {
		errs <- err
	}
	blocks[writeAt] = block
}

// LoadBlocksFromDir load blocks content from a directory, blocks MUST be rlp encoded TODO generalize with decoder interface
func LoadBlocksFromDir(dir string, timeout time.Duration) BlockLoader {
	return func() (blocks []*types.Block, err error) {
		// wrap error
		defer func() {
			if err != nil {
				err = fmt.Errorf("LoadBlocksFromDir: %w", err)
			}
		}()
		// read dir
		results, err := ioutil.ReadDir(dir)
		if err != nil {
			return
		}
		// initialize sync values
		wg := new(sync.WaitGroup)
		errs := make(chan error, 1)
		blocks = make([]*types.Block, len(results), len(results))
		done := make(chan struct{})
		i := 0
		for _, result := range results {
			// if dir ignore
			if result.IsDir() {
				continue
			}
			// add a worker
			wg.Add(1)
			// start the work
			go loadBlock(wg, errs, dir, result.Name(), blocks, i)
			i++
		}
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
			return
		case <-time.After(timeout):
			err = fmt.Errorf("op timeout")
		case err = <-errs:
		}
		return
	}
}
