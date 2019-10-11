# ethutils
ethutils is a library that implements a series of ethereum utilities

###broadcast
####Stream 
Stream can be used to stream block contents, headers and transactions

example:
```go
// ExampleStream streams recv from ropsten network, default options used
func ExampleNewStreamDefault() {
	// init Streamer
	streamer := broadcast.NewStreamDefault("wss://ropsten.infura.io/ws/v3/38c930aee8474fbea8f3b33689faf8c9")
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
```

