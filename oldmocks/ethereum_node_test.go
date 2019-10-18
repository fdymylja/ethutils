package oldmocks

// just imagine writing tests for testers
import (
	"testing"
	"time"
)

func TestEthereumNode_loadBlocks(t *testing.T) {
	c := &ethereumNode{test: t}
	c.loadBlocks(ropstenLoadBlocks(6532150, 6532170))
}

func TestEthereumNode_StartHeaders(t *testing.T) {
	c := NewEthereumNode(t)
	c.SetBlockTime(1 * time.Second)
	h, sub := c.StartHeaders()
	exit := time.After(1 * time.Second)
	for {
		select {
		case header := <-h:
			t.Log(header.Number.Uint64())
		case err := <-sub.Err():
			t.Logf("exiting: %s", err)
			c.Stop()
			time.Sleep(1 * time.Second)
			return
		case <-exit:
			sub.Unsubscribe()
		}
	}
}

func TestEthereumNode_BlockByNumber(t *testing.T) {

}
