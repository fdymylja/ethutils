package broadcast

import (
	"errors"
	"github.com/fdymylja/ethutils/conversions"
	"github.com/fdymylja/ethutils/mocks"
	"log"
	"testing"
	"time"
)

func ExampleAwaiter() {
	// create a streamer
	streamer := NewStreamDefault("wss://ropsten.infura.io/ws/v3/38c930aee8474fbea8f3b33689faf8c9")
	err := streamer.Connect()
	if err != nil {
		panic(err)
	}
	exit := make(chan struct{})
	// create an awaiter
	awaiter := NewAwaiter(streamer)
	defer awaiter.Close()
	go func() {
		// listen for awaiter errors
		select {
		case err := <-awaiter.Err():
			panic(err)
		case <-exit:
			return
		}
	}()
	txID := "0x...." // put a transaction you are waiting for
	// convert tx id to hash
	txHash, err := conversions.HexToHash(txID)
	if err != nil {
		panic(err)
	}
	// wait for tx
	resp, err := awaiter.Wait(txHash)
	if err != nil {
		panic(err)
	}
	tx := <-resp
	log.Printf("txID: %s, block: %d, timestamp: %d", tx.Transaction.Hash().String(), tx.BlockNumber, tx.Timestamp)
	close(exit)
}

func mockStream(t testing.TB) *mocks.Streamer {
	s := mocks.NewStreamer(t)
	return s
}

func TestNewAwaiter(t *testing.T) {
	s := mockStream(t)
	_ = NewAwaiter(s)
	s.Start()
	defer s.Stop()
}

func TestAwaiter_Wait(t *testing.T) {
	//
	tx := "0xf6e1096310e957ab868d3e7d38c44e4da8762eb903bf591daf58c8f20537a9a7"
	txHash, err := conversions.HexToHash(tx)
	if err != nil {
		t.Fatal(err)
	}
	s := mockStream(t)
	a := NewAwaiter(s)
	defer a.Close()
	s.Start()
	defer s.Stop()
	resp, err := a.Wait(txHash)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", <-resp)
	return
}

func TestAwaiter_Wait2(t *testing.T) {
	//
	tx := "0xf6e1096310e957ab868d3e7d38c44e4da8762eb903bf591daf58c8f20537a9a8"
	txHash, err := conversions.HexToHash(tx)
	if err != nil {
		t.Fatal(err)
	}
	s := mockStream(t)
	a := NewAwaiter(s)
	defer a.Close()
	s.Start()
	defer s.Stop()
	resp, err := a.Wait(txHash)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.After(1 * time.Second)
		a.Close()
	}()
	k, ok := <-resp
	t.Log("exited", k, ok)
}

func TestAwaiter_Close(t *testing.T) {
	s := mockStream(t)
	a := NewAwaiter(s)
	a.Close()
	time.Sleep(1 * time.Second)
}

func TestAwaiter_Err(t *testing.T) {
	testErr := errors.New("test error")
	s := mockStream(t)
	a := NewAwaiter(s)
	s.InjectError(testErr)
	select {
	case err := <-a.Err():
		if !errors.Is(err, testErr) {
			t.Fatal("errors do not match")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("no error received")
	}
}
