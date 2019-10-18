package oldmocks

import (
	"testing"
	"time"
)

// Covers the working case
func TestLoadBlocksFromDir(t *testing.T) {
	dir := "../testfiles/blocks/"
	blockLoader := LoadBlocksFromDir(dir, 1*time.Minute)
	blocks, err := blockLoader()
	if err != nil {
		t.Fatal(err)
	}
	for _, block := range blocks {
		t.Log(block.Hash().String())
	}
}

// Covers the timeout case
func TestLoadBlocksFromDir2(t *testing.T) {
	dir := "../testfiles/blocks/"
	blockLoader := LoadBlocksFromDir(dir, 0)
	_, err := blockLoader()
	if err == nil {
		t.Fatal("error expected")
	}
	t.Log(err)
	// success
}

//
