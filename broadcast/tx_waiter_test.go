package broadcast

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"testing"
	"time"
)

func newMockTxProducer(t testing.TB) *mockTxProducer {
	return &mockTxProducer{
		t:             t,
		shutdown:      make(chan struct{}),
		waiterStopped: make(chan struct{}),
	}
}

type mockTxProducer struct {
	t             testing.TB
	shutdown      chan struct{}
	waiterStopped chan struct{}
}

func (m *mockTxProducer) close() {
	close(m.shutdown)
}

func (m *mockTxProducer) shuttingDown() <-chan struct{} {
	return m.shutdown
}

func (m *mockTxProducer) sendTx(tx common.Hash, target *TxWaiter) {
	target.sendResponse(&interfaces.TxWithBlock{
		Transaction: &types.Transaction{},
		BlockNumber: 0,
		Timestamp:   0,
	})

}

func (m mockTxProducer) removeTxWaiter(w *TxWaiter) {
	close(m.waiterStopped)
}

func TestNewTxWaiter(t *testing.T) {
	producer := newMockTxProducer(t)
	_ = NewTxWaiter(common.Hash{}, producer)
}

// Cover the case in which the txProducer is closed
func TestTxWaiter_Err(t *testing.T) {
	producer := newMockTxProducer(t)
	waiter := NewTxWaiter(common.Hash{}, producer)
	go func() {
		time.Sleep(10 * time.Millisecond)
		producer.close()
	}()
	select {
	case <-waiter.Err():
		// success
	case <-waiter.Done():
		t.Fatal("Done should not be called")
	case <-waiter.TransactionIncluded():
		t.Fatal("TransactionIncluded should not be called")
	}

}

// Cover the case in which the tx is forwarded
func TestTxWaiter_TransactionIncluded(t *testing.T) {
	producer := newMockTxProducer(t)
	waiter := NewTxWaiter((&types.Transaction{}).Hash(), producer)
	go func() {
		time.Sleep(10 * time.Millisecond)
		producer.sendTx(common.Hash{}, waiter)
	}()
	select {
	case <-waiter.Err():
		t.Fatal("Err should not be called")
	case <-waiter.Done():
		t.Fatal("Done should not be called")
	case <-waiter.TransactionIncluded():
		// success
	}
}

// Cover the case in which the waiter is stopped
func TestTxWaiter_Done(t *testing.T) {
	producer := newMockTxProducer(t)
	waiter := NewTxWaiter(common.Hash{}, producer)
	go func() {
		waiter.Stop()
	}()
	select {
	case <-waiter.Err():
		t.Fatal("Err should not be called")
	case <-waiter.Done():
		// success
	case <-waiter.TransactionIncluded():
		t.Fatal("Done should not be called")

	}
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("timeout, waiter did not stop properly")
	case <-producer.waiterStopped:
		// success
	}
}

// Cover the case in which we see the hash... yeah I know rly dumb.
func TestTxWaiter_Hash(t *testing.T) {
	waiter := &TxWaiter{hash: common.Hash{}}
	if waiter.Hash() != (common.Hash{}) {
		t.Fatal("fatality")
	}
}
