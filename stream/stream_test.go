package stream

import (
	"errors"
	"github.com/fdymylja/ethutils/status"
	"log"
	"testing"
	"time"
)

// ExampleClient streams recv from ropsten network, default options used
func ExampleClient() {
	// init Streamer
	streamer := NewClientDefault("wss://ropsten.infura.io/ws/v3/38c930aee8474fbea8f3b33689faf8c9")
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
	client := NewClientDefault("wss://ropsten.infura.io/ws/v3/38c930aee8474fbea8f3b33689faf8c9")
	err := client.Connect()
	if err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
}

func TestClient_Connect(t *testing.T) {
	client := NewClientDefault("wss://ropsten.infura.io/ws/v3/38c930aee8474fbea8f3b33689faf8c9")
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
	client := NewClientDefault("wss://ropsten.infura.io/ws/v3/38c930aee8474fbea8f3b33689faf8c9")
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

var testError = errors.New("test error")

func TestClient_Close(t *testing.T) {
	s := NewClientDefault("")
	err := s.Close()
	if !errors.Is(err, status.ErrNotConnected) {
		t.Fatal("should return NotConnectedError")
	}
}
