package listeners

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/mocks"
	"testing"
	"time"
)

func TestBlock_Block(t *testing.T) {
	recvAt := time.Now()
	streamer := mocks.NewStreamer()
	block := NewBlock(streamer, 0)
	streamer.SendBlock(&types.Block{
		ReceivedAt:   recvAt,
		ReceivedFrom: nil,
	})
	select {
	case block := <-block.Block():
		if block.ReceivedAt != recvAt {
			t.Fatalf("blocks do not match: expected %s, got %s", recvAt, block.ReceivedAt)
		}
	default:
		t.Fatal()
	}
}

func TestBlock_Close(t *testing.T) {

}

func TestBlock_Wait(t *testing.T) {

}

func TestBlock_WaitContext(t *testing.T) {

}

func TestNewBlock(t *testing.T) {
	_ = NewBlock(nil, 0)
}
