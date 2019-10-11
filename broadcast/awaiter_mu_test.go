package broadcast

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/fdymylja/ethutils/conversions"
	"github.com/fdymylja/ethutils/mocks"
	"testing"
	"time"
)

func mockStream(t testing.TB) *mocks.Streamer {
	s := mocks.NewStreamer(t)
	return s
}
func TestAwaiterMu(t *testing.T) {
	/*
		streamer := NewStreamDefault("")
	*/
}

func TestNewAwaiterMu(t *testing.T) {
	_ = NewAwaiter(mockStream(t))
}

func TestAwaiterMu_StreamerStarted(t *testing.T) {
	s := mockStream(t)
	s.SetBlockTime(1 * time.Second)
	s.SetBlockLoader(mocks.LoadBlocksFromDir("../testfiles/blocks/", 1*time.Minute))
	s.Start()
	defer s.Stop()
	a := NewAwaiter(s)
	defer a.Close()
	time.Sleep(5 * time.Second)
}

func TestAwaiterMu_Err(t *testing.T) {
	s := mockStream(t)
	a := NewAwaiter(s)
	testErr := errors.New("test error")
	s.InjectError(testErr)
	select {
	case err := <-a.Err():
		if !errors.Is(err, testErr) {
			t.Fatal("error cause does not match")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("error was not forwarded")
	}
}

func TestAwaiterMu_Close(t *testing.T) {

}

func TestAwaiterMu_NotifyTransaction(t *testing.T) {
	testTx := "0x4a474dc6eae9bd342bcedb784e2c604075364d9707b0d355ad956247dc1014c0"
	h, err := conversions.HexToHash(testTx)
	if err != nil {
		t.Fatal(err)
	}
	s := mockStream(t)
	a := NewAwaiter(s)
	s.SetBlockTime(1 * time.Second)
	s.Start()
	defer s.Stop()
	waiter := a.NotifyTransaction(h)
	select {
	case <-waiter.Err():
		t.Fatal("should not receive error")
	case <-waiter.Done():
		t.Fatal("should not receive waiterRemoved")
	case <-waiter.TransactionIncluded():

	}
}

func TestAwaiterMu_removeWaiter(t *testing.T) {
	s := mockStream(t)
	a := NewAwaiter(s)
	w := a.NotifyTransaction(common.Hash{})
	w.Stop()
	<-w.Done()
}
