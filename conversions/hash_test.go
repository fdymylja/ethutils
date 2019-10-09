package conversions

import (
	"bytes"
	"testing"
)

func TestHexToHash(t *testing.T) {
	result := []byte{236, 132, 241, 140, 97, 151, 161, 42, 187, 245, 209, 61, 187, 243, 212, 108, 24, 234, 22, 220, 29, 157, 184, 186, 230, 158, 28, 38, 214, 15, 203, 249}
	hash, err := HexToHash("0xec84f18c6197a12abbf5d13dbbf3d46c18ea16dc1d9db8bae69e1c26d60fcbf9")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, hash[:]) {
		t.Fatal("hash bytes are not equal")
	}
}
