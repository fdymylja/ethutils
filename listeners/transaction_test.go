package listeners

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/mocks"
	"github.com/fdymylja/ethutils/mocks/testblocks"
	"github.com/fdymylja/ethutils/status"
	"testing"
	"time"
)

func TestNewTransaction(t *testing.T) {
	streamer := mocks.NewStreamer()
	_ = NewTransaction(streamer, common.Hash{})
}

func TestTransaction_Transaction(t *testing.T) {
	testBlock := testblocks.Block6550147.MustDecode()
	testTx := testBlock.Transactions()[0]
	testHash := testTx.Hash()
	streamer := mocks.NewStreamer()
	txListener := NewTransaction(streamer, testHash)
	streamer.SendTransaction(&interfaces.TxWithBlock{
		Transaction: testTx,
		BlockNumber: testBlock.NumberU64(),
		Timestamp:   testBlock.Time(),
	})
	select {
	case tx := <-txListener.Transaction():
		if tx.Transaction.Hash() != testHash {
			t.Fatal("hashes do not match")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
	<-txListener.signalClosed
	if !streamer.Terminated() {
		t.Fatal("streamer was not shutdown")
	}
}

func TestTransaction_Wait(t *testing.T) {
	testBlock := testblocks.Block6550147.MustDecode()
	testTx := testBlock.Transactions()[0]
	testHash := testTx.Hash()
	streamer := mocks.NewStreamer()
	txListener := NewTransaction(streamer, testHash)
	streamer.SendTransaction(&interfaces.TxWithBlock{
		Transaction: testTx,
		BlockNumber: testBlock.NumberU64(),
		Timestamp:   testBlock.Time(),
	})
	txRecv := make(chan *interfaces.TxWithBlock, 1)
	errRecv := make(chan error, 1)
	go func() {
		t, err := txListener.Wait()
		if err != nil {
			errRecv <- err
			return
		}
		txRecv <- t
	}()
	select {
	case tx := <-txRecv:
		if tx.Transaction.Hash() != testHash {
			t.Fatal("hashes do not match")
		}
	case err := <-errRecv:
		t.Fatalf("unexpected error: %s", err)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

// Cover wait block approval
func TestTransaction_Wait2(t *testing.T) {
	testBlock := testblocks.Block6550147.MustDecode()
	testTx := testBlock.Transactions()[0]
	testHash := testTx.Hash()
	streamer := mocks.NewStreamer()
	txListener := NewTransaction(streamer, testHash, 4)
	streamer.SendTransaction(&interfaces.TxWithBlock{
		Transaction: testTx,
		BlockNumber: testBlock.NumberU64(),
		Timestamp:   testBlock.Time(),
	})
	// tx was sent and received
	if txListener.tx == nil {
		t.Fatal("transaction should not be nil")
	}
	select {
	case <-txListener.Transaction():
		t.Fatal("transaction should not be finalized")
	case <-time.After(1 * time.Second): // pass
	}
	// send block that deems the transaction finalized
	streamer.SendHeader(testblocks.Block6550149.MustDecode().Header())
	streamer.SendBlock(testblocks.Block6550151.MustDecode())
	select {
	case tx := <-txListener.Transaction():
		if tx.Transaction.Hash() != testHash {
			t.Fatal("hashes do not match")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

func TestTransaction_Err(t *testing.T) {
	streamer := mocks.NewStreamer()
	txListener := NewTransaction(streamer, common.Hash{})
	testError := errors.New("test error")
	streamer.SendError(testError)
	select {
	case err := <-txListener.Err():
		if !errors.Is(err, testError) {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

func TestTransaction_Close(t *testing.T) {
	streamer := mocks.NewStreamer()
	txListener := NewTransaction(streamer, common.Hash{})
	err := txListener.Close()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	_, err = txListener.Wait()
	if !errors.Is(err, status.ErrShutdown) {
		t.Fatalf("unexpected error: %s", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err = txListener.WaitContext(ctx)
	if !errors.Is(err, status.ErrShutdown) {
		t.Fatalf("unexpected error: %s", err)
	}
	err = txListener.Close()
	if !errors.Is(err, status.ErrClosed) {
		t.Fatalf("unexpected error: %s", err)
	}
}

// Cover dead context
func TestTransaction_WaitContext(t *testing.T) {
	streamer := mocks.NewStreamer()
	txListener := NewTransaction(streamer, common.Hash{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := txListener.WaitContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %s", err)
	}
}
