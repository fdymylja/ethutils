package testblocks

import (
	"testing"
)

/*
func Test1(t *testing.T) {
	t.Log(os.Getwd())
	m := 100
	blocks := make(map[uint64]string)
	for i := 0; i <= m; i++ {
		f, err := os.Open(fmt.Sprintf("../../testfiles/blocks/%d.block", i))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close() // yea ikr bad idea but i need to do things fast rn
		s, err := ioutil.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		//
		block := new(types.Block)
		err = rlp.Decode(bytes.NewReader(s), block)
		if err != nil {
			t.Fatal(err)
		}
		//
		blockHex := hex.EncodeToString(s)
		blocks[block.NumberU64()] = blockHex
	}
	toPrint := ""
	for blockNumber, blockHex := range blocks {
		toPrint = fmt.Sprintf("%s\nvar Block%d mockBlock = \"%s\"", toPrint, blockNumber, blockHex)
	}
	f, err := os.OpenFile("blocks.go", os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(toPrint)
	if err != nil {
		t.Fatal(err)
	}
}
*/

func Test2(t *testing.T) {
	b := Block6550147.MustDecode()
	t.Log(b.NumberU64())
}
