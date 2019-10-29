package main

// genblocks generates blocks for mocks
import (
	"bytes"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fdymylja/ethutils/nodeop"
	"log"
	"os"
)

var genFile = `
// THIS IS A GENERATED FILE DO NOT EDIT!

import (
	"bytes"
	"encoding/hex"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

// mockBlocks is a slice of mockBlock
type mockBlocks []mockBlock

// MustDecode decodes all the blocks inside to *types.Block, if one fails it panics.
func (blocks mockBlocks) MustDecode() []*types.Block {
	b := make([]*types.Block, len(blocks))
	for i := range blocks {
		b[i] = blocks[i].MustDecode()
	}
	return b
}

// mockBlock represents a hex string of a block used for test purposes
type mockBlock string

// MustDecode decodes the hex string to a block, if it can't it panics
func (b mockBlock) MustDecode() *types.Block {
	block := new(types.Block)
	blockBytes, err := hex.DecodeString(string(b))
	if err != nil {
		panic(err)
	}
	r := bytes.NewReader(blockBytes)
	err = rlp.Decode(r, block)
	if err != nil {
		panic(err)
	}
	return block
}
`

func hexBlock(block *types.Block) string {
	buf := bytes.NewBuffer(nil)
	err := block.EncodeRLP(buf)
	if err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(buf.Bytes())
}

var endpoint = flag.String("endpoint", "wss://ropsten.infura.io/ws", "endpoint is the rpc endpoint")
var start = flag.Uint64("start", 6551146-100, "start is the starting block to start the download")
var final = flag.Uint64("final", 6551146, "final is the final block to download")
var timeout = flag.Duration("timeout", 0, "timeout is the maximum timeout for the block download")
var target = flag.String("dst", "mocks/testblocks/generated_blocks.go", "dst is the destination of the generated file")
var packageName = flag.String("package", "testblocks", "package is the name of the package given to the generated file")
var blocksSlice = "// BlocksSlice is the slice of all the blocks generated\nvar BlocksSlice = mockBlocks{"

// genblocks downloads blocks from start to final
func main() {
	flag.Parse()
	if *start >= *final {
		log.Fatal("starting block must be lower than final block")
	}
	client, err := ethclient.Dial(*endpoint)
	if err != nil {
		log.Fatalf("unable to connect client: %s", err)
	}
	// prepare context
	ctx := context.Background()
	if *timeout != 0 {
		ctx, _ = context.WithTimeout(ctx, *timeout)
	}
	// start downloading blocks
	blocks, errs := nodeop.DownloadBlocksCnc(ctx, client, *start, *final)
	for {
		select {
		case b, ok := <-blocks:
			if !ok {
				finalize()
				return
			}
			blockStr := fmt.Sprintf("// Block%d is an RLP encoded hex string of the content of block number %d\nvar Block%d mockBlock = \"%s\"", b.NumberU64(), b.NumberU64(), b.NumberU64(), hexBlock(b))
			blocksSlice = fmt.Sprintf(`%sBlock%d, `, blocksSlice, b.NumberU64())
			genFile = fmt.Sprintf("%s\n%s", genFile, blockStr)
		case err := <-errs:
			log.Fatal(err)
		}
	}
}

func finalize() {
	f, err := os.OpenFile(*target, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	blocksSlice += "}"
	log.Print(blocksSlice)
	pckg := fmt.Sprintf("package %s", *packageName)
	genFile = fmt.Sprintf("%s\n%s\n%s\n", pckg, genFile, blocksSlice)
	_, err = f.Write([]byte(genFile))
	if err != nil {
		log.Fatal(err)
	}

}
