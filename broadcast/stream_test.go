package broadcast

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/mocks"
	"log"
	"testing"
	"time"
)

// ExampleStream streams transaction from ropsten network, default options used
func ExampleNewStreamDefault() {
	// init streamer
	streamer := NewStreamDefault("wss://ropsten.infura.io/ws/v3/38c930aee8474fbea8f3b33689faf8c9")
	// connect it
	err := streamer.Connect()
	if err != nil {
		panic(err)
	}
	defer streamer.Close()
	exit := time.After(1 * time.Minute)
	for {
		select {
		case err := <-streamer.Err():
			panic(err)
		case tx := <-streamer.Transaction():
			log.Printf("recv at block %d tx: %s", tx.BlockNumber, tx.Transaction.Hash().String())
		case header := <-streamer.Header():
			log.Printf("recv header for block %d", header.Number.Uint64())
		case <-exit:
			return
		}
	}
}

// testMockStream swaps components of Stream with mock components
func testMockStream(s *Stream, t testing.TB) {
	testHeaders := mocks.NewEthereumNode(t)
	testHeaders.SetBlockTime(1 * time.Second)
	headers, sub := testHeaders.StartHeaders()
	// set ethClient to mock
	s.ethClient = testHeaders.Node()
	go s.listenBlockHeaders(headers, sub)
}

func TestStream_Connect(t *testing.T) {
	s := NewStreamDefault("wss://ropsten.infura.io/ws/v3/38c930aee8474fbea8f3b33689faf8c9")
	err := s.Connect()
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
}

func TestStream_Header(t *testing.T) {
	testHeaders := mocks.NewEthereumNode(t)
	headers, sub := testHeaders.StartHeaders()
	// init Stream
	s := NewStreamDefault("")
	// set ethClient to mock
	s.ethClient = testHeaders.Node()
	go s.listenBlockHeaders(headers, sub)
	select {
	case h := <-headers:
		t.Log(h.Number.Uint64())
	}
	_ = s.Close()
	return
}

func TestStream_Transaction(t *testing.T) {
	s := NewStreamDefault("")
	s.options.StreamBlocks = false
	testMockStream(s, t)
	select {
	case tx := <-s.Transaction():
		t.Log(tx.Transaction.Hash().String())
	}
}

func TestStream_Err(t *testing.T) {
	s := NewStreamDefault("")
	s.options.StreamBlocks = false
	s.options.StreamTransactions = true
	go s.listenBlockHeaders(nil, mocks.NewErrorSubscription(t, nil))
	// inject an error
	select {
	case err := <-s.Err():
		t.Log(err)
	}
}

func TestStream_downloadBlock(t *testing.T) {
	retries := 10
	s := NewStream("wss://ropsten.infura.io/ws/v3/38c930aee8474fbea8f3b33689faf8c9", &StreamOptions{
		NodeOpTimeout:      15 * time.Second,
		MaxRetries:         retries,
		RetryWait:          1 * time.Second,
		WaitAfterHeader:    0,
		StreamBlocks:       false,
		StreamTransactions: false,
	})
	err := s.Connect()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.downloadBlock(&types.Header{
		ParentHash:  common.Hash{},
		UncleHash:   common.Hash{},
		Coinbase:    common.Address{},
		Root:        common.Hash{},
		TxHash:      common.Hash{},
		ReceiptHash: common.Hash{},
		Bloom:       types.Bloom{},
		Difficulty:  nil,
		Number:      nil,
		GasLimit:    0,
		GasUsed:     0,
		Time:        0,
		Extra:       nil,
		MixDigest:   common.Hash{},
		Nonce:       types.BlockNonce{},
	})
	if err == nil {
		t.Fatal("should return an error")
	}
	t.Logf("failed because of: %s", err)
}
