package mocks

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func Test1(t *testing.T) {
	m := 10
	blocks := make([][]byte, m+1)
	for i := 0; i <= m; i++ {
		f, err := os.Open("../testfiles/blocks/1.block")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close() // yea ikr bad idea but i need to do things fast rn
		s, err := ioutil.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		blocks[i] = s
	}
	s, err := os.OpenFile("blocks.go", os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	stuff := fmt.Sprintf("blocks := %#v", blocks)
	_, err = s.Write([]byte(stuff))
	if err != nil {
		t.Fatal(err)
	}
	//
	// decode block
	/*
		block := new(types.Block)
		err = rlp.Decode(f, block)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%#v", block)
		b, err := json.Marshal(block)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s", b)
	*/
	// r := hex.NewDecoder(f)
}
