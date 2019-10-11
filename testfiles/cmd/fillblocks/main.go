package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fdymylja/ethutils/nodeop"
	"log"
	"os"
)

func main() {
	log.Print("starting")
	eth, err := ethclient.Dial("wss://ropsten.infura.io/ws/v3/38c930aee8474fbea8f3b33689faf8c9")
	if err != nil {
		log.Fatal(err)
	}
	// download block range
	blocks, err := nodeop.DownloadBlocksByRange(context.Background(), eth, 6551146-1000, 6551146)
	if err != nil {
		log.Fatal()
	}
	// save blocks to file
	for i, block := range blocks {
		log.Printf("Saving block n %d, hash: %s", i, block.Hash().String())
		f, err := os.OpenFile(fmt.Sprintf("testfiles/blocks/%d.block", i), os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatal(err)
		}
		err = block.EncodeRLP(f)
		if err != nil {
			log.Fatal(err)
		}
		err = f.Close()
		if err != nil {
			log.Fatal(err)
		}
	}
}
