package stream

import (
	"errors"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/ethutils/mocks"
	"github.com/fdymylja/ethutils/status"
	"testing"
	"time"
)

func TestNewMultiStream(t *testing.T) {
	_ = NewMultiStream(mocks.NewStreamer())
}

// Cover error forwarding and the fact that errors are only sent once
func TestMultiStream_Err(t *testing.T) {
	testError := errors.New("test error")
	streamer := mocks.NewStreamer()
	ms := NewMultiStream(streamer)
	defer ms.Close()
	listener, err := ms.NewListener()
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	streamer.SendError(testError)
	// extract error from listener
	select {
	case err := <-listener.Err():
		if !errors.Is(err, testError) {
			t.Fatalf("error does not match: got: %s, expected %s", err, testError)
		}
	case <-time.After(1 * time.Second): // give time to channels to forward error value
		t.Fatalf("no error received")
	}
	// extract error from MultiStream
	select {
	case err := <-ms.Err():
		if !errors.Is(err, testError) {
			t.Fatalf("error does not match: got: %s, expected %s", err, testError)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("no error received")
	}
	// try to forward another error to test if errs are forwarded once only
	streamer.SendError(testError)
	// extract error from listener
	select {
	case _, ok := <-listener.Err():
		if ok {
			t.Fatalf("channel should be closed")
			return
		}
	case <-time.After(1 * time.Second): // give time to channels to forward error value
	}
	// extract error from MultiStream
	select {
	case _, ok := <-ms.Err():
		if ok {
			t.Fatalf("channel should be closed")
			return
		}
	case <-time.After(1 * time.Second):
	}
}

// Cover functionality of NewListener func and the correct flow of information
// checks also for the correct removal of listeners during Close called from the children
func TestMultiStream_NewListener(t *testing.T) {
	testHeader := [32]byte{0xc3, 0xbd, 0x2d, 0x0, 0x74, 0x5c, 0x3, 0x4, 0x8a, 0x56, 0x16, 0x14, 0x6a, 0x96, 0xf5, 0xff, 0x78, 0xe5, 0x4e, 0xfb, 0x9e, 0x5b, 0x4, 0xaf, 0x20, 0x8c, 0xda, 0xff, 0x6f, 0x38, 0x30, 0xee}
	testBlock := [32]byte{0x1d, 0xcc, 0x4d, 0xe8, 0xde, 0xc7, 0x5d, 0x7a, 0xab, 0x85, 0xb5, 0x67, 0xb6, 0xcc, 0xd4, 0x1a, 0xd3, 0x12, 0x45, 0x1b, 0x94, 0x8a, 0x74, 0x13, 0xf0, 0xa1, 0x42, 0xfd, 0x40, 0xd4, 0x93, 0x47}
	testTransactionBlockNumber := uint64(1000)
	streamer := mocks.NewStreamer()
	ms := NewMultiStream(streamer)
	listener, err := ms.NewListener()
	if err != nil {
		t.Fatal(err)
	}
	streamer.SendHeader(new(types.Header))
	streamer.SendBlock(new(types.Block))
	streamer.SendTransaction(&interfaces.TxWithBlock{BlockNumber: testTransactionBlockNumber})
	select {
	case h := <-listener.Header():
		if h.Hash() != testHeader {
			t.Fatalf("hashes do not match: got: %#v, expected: %#v", h.Hash(), testHeader)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("no header received")
	}
	select {
	case block := <-listener.Block():
		if block.Hash() != testBlock {
			t.Fatalf("hashes do not match: got %#v, expected: %#v", block.Hash(), testBlock)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("no block received")
	}
	select {
	case tx := <-listener.Transaction():
		if tx.BlockNumber != testTransactionBlockNumber {
			t.Fatalf("transaction block number does not match got: %#v, expected: %#v", tx.BlockNumber, testTransactionBlockNumber)
		}
	}
	// generate new listener and check if it is removed correctly
	listener, err = ms.NewListener()
	if err != nil {
		t.Fatal(err)
	}
	err = listener.Close()
	if err != nil {
		t.Fatal(err)
	}
	_, ok := ms.listeners[listener.(msChildIF)]
	if ok {
		t.Fatal("listener not removed")
	}
}

func TestMultiStream_Close(t *testing.T) {
	streamer := mocks.NewStreamer()
	ms := NewMultiStream(streamer)
	listener, err := ms.NewListener()
	if err != nil {
		t.Fatal(err)
	}
	// close
	err = ms.Close()
	if err != nil {
		t.Fatal(err)
	}
	// check if error if status.Shutdown is sent to listener and MultiStream
	select {
	case err := <-ms.Err():
		if !errors.Is(err, status.ErrShutdown) {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("no error received")
	}
	select {
	case err := <-listener.Err():
		if !errors.Is(err, status.ErrShutdown) {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("no error received")
	}
	// check if creation of new streamers is allowed
	listener, err = ms.NewListener()
	if err == nil {
		t.Fatal("error expected")
	}
	// try to close the MultiStream again
	err = ms.Close()
	if !errors.Is(err, status.ErrClosed) {
		t.Fatalf("unexpected error: %s", err)
	}
	// see if parent streamer is closed
	if !streamer.Terminated() {
		t.Fatal("parent streamer is not closed")
	}
}
