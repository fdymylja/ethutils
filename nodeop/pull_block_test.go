package nodeop

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"testing"
)

func testEthClient() *ethclient.Client {
	endpoint := "wss://ropsten.infura.io/ws"
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		panic(err)
	}
	return client
}

func TestDownloadBlockByNumber(t *testing.T) {
	c := testEthClient()
	b, err := DownloadBlockByNumber(context.Background(), c, 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(hexutil.Encode(b.Hash().Bytes()))
}

func TestDownloadBlock(t *testing.T) {
	testHash := "0xb3074f936815a0425e674890d7db7b5e94f3a06dca5b22d291b55dcd02dde93e"
	c := testEthClient()
	_, err := DownloadBlock(context.Background(), c, common.BytesToHash(hexutil.MustDecode(testHash)))
	if err != nil {
		t.Fatal(err)
	}
}

func TestDownloadRangeByNumberRange(t *testing.T) {
	c := testEthClient()
	var start uint64
	var downloadNumb uint64 = 10
	blocks, err := DownloadBlocksByRange(context.Background(), c, start, start+downloadNumb)
	if err != nil {
		t.Fatal(err)
	}
	for i, block := range blocks {
		if block.Number().Uint64() != start+uint64(i) {
			t.Fatalf("block number mismatch: expected: %d got: %d", block.Number().Uint64(), start+uint64(i))
		}
		t.Log(block.NumberU64())
	}
}
