package oldmocks

import (
	"testing"
	"time"
)

func TestStreamer_Start(t *testing.T) {
	s := NewStreamer(t)
	s.SetBlockTime(1 * time.Second)
	exit := time.After(30 * time.Second)
	go func() {
		for {
			select {
			case <-exit:
				return
			case tx := <-s.Transaction():
				t.Logf("%s", tx.Transaction.Hash().String())
			case <-s.Header():
			case <-s.Block():
			case err := <-s.Err():
				t.Fatal(err)
			}
		}
	}()
	s.Start()
	defer s.Stop()
	<-exit
}
