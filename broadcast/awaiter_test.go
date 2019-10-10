package broadcast

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/fdymylja/ethutils/conversions"
	"github.com/fdymylja/ethutils/mocks"
	"github.com/fdymylja/ethutils/status"
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
	txID := "0x...." // put a recv you are waiting for
	// convert tx id to hash
	txHash, err := conversions.HexToHash(txID)
	if err != nil {
		panic(err)
	}
	// wait for tx
	resp := awaiter.Wait(txHash)
	if err != nil {
		panic(err)
	}
	tx := resp.Wait()
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
	defer s.Stop()
	a := NewAwaiter(s)
	defer a.Close()
	s.Start()
	defer s.Stop()
	resp := a.Wait(txHash)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", resp.Wait())
	return
}

// TestAwaiter_Wait2 covers the case in which Awaiter stops before finding the transaction we're looking for
func TestAwaiter_Wait2(t *testing.T) {
	tx := "0xf6e1096310e957ab868d3e7d38c44e4da8762eb903bf591daf58c8f20537a9a8"
	txHash, err := conversions.HexToHash(tx)
	if err != nil {
		t.Fatal(err)
	}
	s := mockStream(t)
	defer s.Stop()
	a := NewAwaiter(s)
	defer a.Close()
	resp := a.Wait(txHash)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(1 * time.Second)
		a.Close()
	}()
	select {
	case err := <-resp.Err():
		if !errors.Is(err, status.ErrShutdown) {
			t.Fatalf("wrong code path, it should exit with status.ErrClosed. Got: %s", err)
		}
	case k, ok := <-resp.Transaction():
		t.Fatal(k, ok)
	case <-time.After(10 * time.Second):

	}
}

// TestAwaiter_Wait3 covers the case in which the request is made but is not forwarded before Awaiter exits
func TestAwaiter_Wait3(t *testing.T) {
	s := mockStream(t)
	defer s.Stop()
	a := NewAwaiter(s)
	// block requests forever
	a.waitRequests = nil
	exit := make(chan struct{})
	errs := make(chan error, 1)
	go func() {
		resp := a.Wait(common.Hash{})
		errc := resp.Err()
		err := <-errc
		if !errors.Is(err, status.ErrClosed) {
			errs <- errors.New("error is not ErrClosed")
		}
		close(exit)
	}()
	time.Sleep(1 * time.Second)
	a.Close()
	select {
	case <-exit:
		// success
	case err := <-errs:
		t.Fatal(err)
	}
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

// TestAwaiter_onError covers the case in which the error sent to the parent is dropped before being forwarded
func TestAwaiter_onError(t *testing.T) {
	s := mockStream(t)
	defer s.Stop()
	a := NewAwaiter(s)
	// block requests forever
	a.errs = nil
	exit := make(chan struct{})
	go func() {
		a.onError(nil)
		close(exit)
	}()
	time.Sleep(1 * time.Second)
	a.Close()
	<-exit
}
