package conversions

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/fdymylja/utils"
)

// HexToHash converts a hexadecimal string to common.Hash, must start with 0x prefix
func HexToHash(str string) (hash common.Hash, err error) {
	defer utils.WrapErrorP(&err)
	// convert string to bytes
	b, err := hexutil.Decode(str)
	if err != nil {
		return
	}
	// convert bytes to common.Hash
	hash = common.BytesToHash(b)
	return
}
