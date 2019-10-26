package nodeop

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/mocks"
	"testing"
)

func testEthClient() interfaces.Node {
	node := mocks.NewNode()
	return node
}

func TestDownloadBlockByNumber(t *testing.T) {
	c := testEthClient()
	_, err := DownloadBlockByNumber(context.Background(), c, 6551046)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDownloadBlock(t *testing.T) {
	testHash := "0x952857460567205f35cc7d25b8d7782c5e335c3126271384c930d75224e7bc92"
	c := testEthClient()
	_, err := DownloadBlock(context.Background(), c, common.BytesToHash(hexutil.MustDecode(testHash)))
	if err != nil {
		t.Fatal(err)
	}
}

func TestDownloadRangeByNumberRange(t *testing.T) {
	c := testEthClient()
	var start uint64 = 6551046
	var downloadNumb uint64 = 10
	blocks, err := DownloadBlocksByRange(context.Background(), c, start, start+downloadNumb)
	if err != nil {
		t.Fatal(err)
	}
	for i, block := range blocks {
		if block.Number().Uint64() != start+uint64(i) {
			t.Fatalf("block number mismatch: expected: %d got: %d", block.Number().Uint64(), start+uint64(i))
		}
	}
}

func TestDownloadBlocksCnc(t *testing.T) {
	c := testEthClient()
	var start uint64 = 6551046
	var downloadNumb uint64 = 10
	blocks, errs := DownloadBlocksCnc(context.Background(), c, start, start+downloadNumb)
	for {
		select {
		case _, ok := <-blocks:
			if !ok {
				return
			}
		case err := <-errs:
			t.Fatal(err)
		}
	}
}
