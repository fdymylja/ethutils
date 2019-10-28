package stream

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/mocks"
	"github.com/fdymylja/ethutils/status"
	"log"
	"testing"
	"time"
)

// ExampleClient streams recv from ropsten network, default options used
func ExampleClient() {
	// init Streamer
	streamer := NewClientDefault("wss://ropsten.infura.io/ws")
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

func TestClient_ConnectClose(t *testing.T) {
	client := NewClientDefault("wss://ropsten.infura.io/ws")
	err := client.Connect()
	if err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
}

func TestClient_Connect(t *testing.T) {
	client := NewClientDefault("wss://ropsten.infura.io/ws")
	err := client.Connect()
	if err != nil {
		t.Fatal(err)
	}
	err = client.Connect()
	if !errors.Is(err, status.ErrAlreadyConnected) {
		t.Fatal("should return status.ErrAlreadyConnected")
	}
}

// Covers the case of Connect->Close->Connect
func TestClient_Reuse(t *testing.T) {
	client := NewClientDefault("wss://ropsten.infura.io/ws")
	err := client.Connect()
	if err != nil {
		t.Fatal(err)
	}
	err = client.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = client.Connect()
	if err != nil {
		t.Fatal(err)
	}
	err = client.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestClient_Close(t *testing.T) {
	s := NewClientDefault("")
	err := s.Close()
	if !errors.Is(err, status.ErrNotConnected) {
		t.Fatal("should return NotConnectedError")
	}
}

// test block
func TestClient_Block(t *testing.T) {
	options := &Options{
		NodeOpTimeout:      0,
		MaxRetries:         1,
		RetryWait:          0,
		WaitAfterHeader:    0,
		StreamBlocks:       true,
		StreamHeaders:      false,
		StreamTransactions: false,
	}
	node := mocks.NewNode()
	c := NewClient("", options)
	c.init()
	headers := make(chan *types.Header)
	sub, err := node.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		t.Fatal(err)
	}
	c.client = node
	// force loop
	go c.loop(headers, sub)
	// wait
	select {
	case <-c.Header():
		t.Fatal("unexpected header")
	case <-c.Transaction():
		t.Fatal("unexpected transaction")
	case <-c.Block():
	case err := <-c.Err():
		t.Fatalf("unexpected error: %s", err)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

// test Header
func TestClient_Header(t *testing.T) {
	options := &Options{
		NodeOpTimeout:      0,
		MaxRetries:         1,
		RetryWait:          0,
		WaitAfterHeader:    0,
		StreamBlocks:       false,
		StreamHeaders:      true,
		StreamTransactions: false,
	}
	node := mocks.NewNode()
	c := NewClient("", options)
	c.init()
	headers := make(chan *types.Header)
	sub, err := node.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		t.Fatal(err)
	}
	c.client = node
	// force loop
	go c.loop(headers, sub)
	// wait
	select {
	case <-c.Header():
	case <-c.Transaction():
		t.Fatal("unexpected transaction")
	case <-c.Block():
		t.Fatal("unexpected block")
	case err := <-c.Err():
		t.Fatalf("unexpected error: %s", err)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

// test tx
func TestClient_Transaction(t *testing.T) {
	options := &Options{
		NodeOpTimeout:      0,
		MaxRetries:         1,
		RetryWait:          0,
		WaitAfterHeader:    0,
		StreamBlocks:       false,
		StreamHeaders:      false,
		StreamTransactions: true,
	}
	node := mocks.NewNode()
	c := NewClient("", options)
	c.init()
	headers := make(chan *types.Header)
	sub, err := node.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		t.Fatal(err)
	}
	c.client = node
	// force loop
	go c.loop(headers, sub)
	// wait
	select {
	case <-c.Header():
		t.Fatal("unexpected header")
	case <-c.Transaction():
	case <-c.Block():
		t.Fatal("unexpected block")
	case err := <-c.Err():
		t.Fatalf("unexpected error: %s", err)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

// Err covers error forwarding coming from sub errors and its state change
func TestClient_Err(t *testing.T) {
	sub := mocks.NewSubscription()
	c := NewClientDefault("")
	c.init()
	c.connected = true
	go c.loop(nil, sub)
	testError := errors.New("test error")
	sub.SendError(testError)
	select {
	case err := <-c.Err():
		if !errors.Is(err, testError) {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
	select {
	case <-c.loopExit:
	default:
		t.Fatal("loop hasn't exited")
	}
}
