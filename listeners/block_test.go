package listeners

import (
	"context"
	"errors"
	"github.com/fdymylja/ethutils/mocks"
	"github.com/fdymylja/ethutils/mocks/testblocks"
	"github.com/fdymylja/ethutils/status"
	"testing"
	"time"
)

func TestBlock_Block(t *testing.T) {
	streamer := mocks.NewStreamer()
	mockBlock := testblocks.Block6551046.MustDecode()
	blockListener := NewBlock(streamer, mockBlock.NumberU64())
	streamer.SendBlock(mockBlock)
	select {
	case newBlock := <-blockListener.Block():
		if newBlock.NumberU64() != mockBlock.NumberU64() {
			t.Fatalf("blocks do not match")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

func TestBlock_Close(t *testing.T) {
	streamer := mocks.NewStreamer()
	blockListener := NewBlock(streamer, 60000)
	err := blockListener.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = <-blockListener.Err()
	if !errors.Is(err, status.ErrShutdown) {
		t.Fatalf("unexpected error: %s", err)
	}
	// try close again
	err = blockListener.Close()
	if !errors.Is(err, status.ErrClosed) {
		t.Fatalf("unexpected error: %s", err)
	}
}

// Covers the working Wait case, also covers the WaitContext case because Wait uses WaitContext
func TestBlock_Wait(t *testing.T) {
	streamer := mocks.NewStreamer()
	mockBlock := testblocks.Block6551046.MustDecode()
	blockListener := NewBlock(streamer, mockBlock.NumberU64())
	streamer.SendBlock(mockBlock)
	block, err := blockListener.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if block.NumberU64() != mockBlock.NumberU64() {
		t.Fatal("blocks do not match")
	}
}

func TestBlock_WaitContext(t *testing.T) {
	streamer := mocks.NewStreamer()
	mockBlock := testblocks.Block6551046.MustDecode()
	blockListener := NewBlock(streamer, mockBlock.NumberU64())
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err := blockListener.WaitContext(ctx)
	if err == nil {
		t.Fatal("error expected")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %s", err)
	}
}

// Cover the case in which the streamer forwards errors
func TestBlock_Wait2(t *testing.T) {
	streamer := mocks.NewStreamer()
	mockBlock := testblocks.Block6551046.MustDecode()
	blockListener := NewBlock(streamer, mockBlock.NumberU64())
	testErr := errors.New("test error")
	streamer.SendError(testErr)
	_, err := blockListener.Wait()
	if err == nil {
		t.Fatal("error expected")
	}
	if !errors.Is(err, testErr) {
		t.Fatalf("unexpected error: %s", err)
	}
}

// cover the case in which the listener drops the block
func TestBlock_Wait3(t *testing.T) {

}

func TestNewBlock(t *testing.T) {
	_ = NewBlock(mocks.NewStreamer(), 0)
}
