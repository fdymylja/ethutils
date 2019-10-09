package mocks

import (
	"testing"
	"time"
)

func TestStreamer_Start(t *testing.T) {
	s := NewStreamer(t)
	exit := time.After(1 * time.Minute)
	go func() {
		for {
			select {
			case <-exit:
				return
			case tx := <-s.Transaction():
				t.Logf("%s", tx.Transaction.Hash().String())
			case <-s.Header():
			case err := <-s.Err():
				t.Fatal(err)
			}
		}
	}()
	s.Start()
	defer s.Stop()
	<-exit
}
