package stream

import (
	"bytes"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/status"
	"testing"
	"time"
)

type mockStoppableStreamer struct {
	removed  bool
	streamer *stoppableStreamer
}

func (m *mockStoppableStreamer) removeListener(childIF msChildIF) {
	m.removed = true
}

func newMockStoppableStreamer() *mockStoppableStreamer {
	return &mockStoppableStreamer{}
}

func (m *mockStoppableStreamer) sendBlock(block *types.Block) {
	m.streamer.sendBlock(block)
}

func (m *mockStoppableStreamer) sendError(err error) {
	m.streamer.sendError(err)
}

func (m *mockStoppableStreamer) sendTx(tx *interfaces.TxWithBlock) {
	m.streamer.sendTransaction(tx)
}

func (m *mockStoppableStreamer) sendHeader(header *types.Header) {
	m.streamer.sendHeader(header)
}

func TestStoppableStreamer_Block(t *testing.T) {
	producer := newMockStoppableStreamer()
	streamer := newStoppableStreamer(producer)
	producer.streamer = streamer
	producer.sendBlock(new(types.Block))
	select {
	case block := <-streamer.Block():
		blockHash := block.Hash()
		if !bytes.Equal(blockHash[:], []byte{0x1d, 0xcc, 0x4d, 0xe8, 0xde, 0xc7, 0x5d, 0x7a, 0xab, 0x85, 0xb5, 0x67, 0xb6, 0xcc, 0xd4, 0x1a, 0xd3, 0x12, 0x45, 0x1b, 0x94, 0x8a, 0x74, 0x13, 0xf0, 0xa1, 0x42, 0xfd, 0x40, 0xd4, 0x93, 0x47}) {
			t.Fatalf("block hash expected: %s, got: %#v", []byte{}, blockHash[:])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("block send timeout")
	}
}

func TestStoppableStreamer_Err(t *testing.T) {
	producer := newMockStoppableStreamer()
	streamer := newStoppableStreamer(producer)
	producer.streamer = streamer
	testError := errors.New("testError")
	producer.sendError(testError)
	select {
	case err := <-streamer.Err():
		if !errors.Is(err, testError) {
			t.Fatal("errors do not match")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("error send timeout")
	}
}

func TestStoppableStreamer_Close(t *testing.T) {
	producer := newMockStoppableStreamer()
	streamer := newStoppableStreamer(producer)
	producer.streamer = streamer
	closeDone := make(chan struct{})
	go func() {
		_ = streamer.Close()
		close(closeDone)
	}()
	select {
	case err := <-streamer.Err():
		if !errors.Is(err, status.ErrShutdown) {
			t.Fatalf("expected status.ErrShutdown, got: %s", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("error send timeout")
	}
	<-closeDone
	if !producer.removed {
		t.Fatal("streamer was not set to nil")
	}
}

func TestStoppableStreamer_Header(t *testing.T) {
	producer := newMockStoppableStreamer()
	streamer := newStoppableStreamer(producer)
	producer.streamer = streamer
	producer.sendHeader(&types.Header{
		ParentHash: common.Hash{},
	})
	select {
	case h := <-streamer.Header():
		if h.Hash() != [32]byte{0xc3, 0xbd, 0x2d, 0x0, 0x74, 0x5c, 0x3, 0x4, 0x8a, 0x56, 0x16, 0x14, 0x6a, 0x96, 0xf5, 0xff, 0x78, 0xe5, 0x4e, 0xfb, 0x9e, 0x5b, 0x4, 0xaf, 0x20, 0x8c, 0xda, 0xff, 0x6f, 0x38, 0x30, 0xee} {
			headerHash := h.Hash()
			t.Fatalf("hashes do not match: expected %#v, got: %#v", common.Hash{}, headerHash[:])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("header send timeout")
	}
}

func TestStoppableStreamer_Transaction(t *testing.T) {
	producer := newMockStoppableStreamer()
	streamer := newStoppableStreamer(producer)
	producer.streamer = streamer
	producer.sendTx(&interfaces.TxWithBlock{
		Transaction: nil,
		BlockNumber: 1,
		Timestamp:   1,
	})
	select {
	case tx := <-streamer.Transaction():
		if tx.BlockNumber != 1 {
			t.Fatal("wrong block forwarded")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("transaction send timeout")
	}
}

// Cover the case in which the Streamer is stopped while there are send ops
func TestStoppableStreamer_Close2(t *testing.T) {
	producer := newMockStoppableStreamer()
	streamer := newStoppableStreamer(producer)
	producer.streamer = streamer
	producer.sendTx(nil)
	producer.sendHeader(nil)
	producer.sendBlock(nil)
	go func() {
		time.Sleep(1 * time.Second)
		streamer.Close()
	}()
	// wait close
	select {
	case err := <-streamer.Err():
		if !errors.Is(err, status.ErrShutdown) {
			t.Fatalf("expected status.ErrShutdown, got: %s", err)
		}
	}
	// see if goroutines dropped the msg
	select {
	case <-streamer.Block():
		t.Fatal("received block")
	case <-streamer.Transaction():
		t.Fatal("received tx")
	case <-streamer.Header():
		t.Fatal("received header")
	default:

	}
	// see if in case streamer is not alive the send ops are blocking or not
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
		case <-time.After(1 * time.Second):
			panic("ops are blocking")
		}
	}()
	streamer.sendBlock(new(types.Block))
	streamer.sendHeader(new(types.Header))
	streamer.sendTransaction(new(interfaces.TxWithBlock))
	close(done)
}
