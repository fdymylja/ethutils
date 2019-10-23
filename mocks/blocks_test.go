package mocks

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"io/ioutil"
	"os"
	"testing"
)

func Test1(t *testing.T) {
	t.Log(os.Getwd())
	m := 10
	blocks := make(map[uint64]string)
	for i := 0; i <= m; i++ {
		f, err := os.Open(fmt.Sprintf("../testfiles/blocks/%d.block", i))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close() // yea ikr bad idea but i need to do things fast rn
		s, err := ioutil.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		//
		block := new(types.Block)
		err = rlp.Decode(bytes.NewReader(s), block)
		if err != nil {
			t.Fatal(err)
		}
		//
		blockHex := hex.EncodeToString(s)
		blocks[block.NumberU64()] = blockHex
	}
	for blockNumber, blockHex := range blocks {
		t.Logf("var Block%d mockBlock = \"%s\"", blockNumber, blockHex)
	}
}

func Test2(t *testing.T) {
	b := Block6550147.MustDecode()
	t.Log(b.NumberU64())
}
