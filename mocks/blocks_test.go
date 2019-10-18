package mocks

import (
	"os"
	"testing"
)

func Test1(t *testing.T) {
	f, err := os.Open("../testfiles/blocks/1.block")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

}
