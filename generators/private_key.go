package generators

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fdymylja/utils"
)

// PrivateKeys generates n *ecdsa.PrivateKeys
func PrivateKeys(n int) (keys []*ecdsa.PrivateKey, err error) {
	defer utils.WrapErrorP(&err)
	keys = make([]*ecdsa.PrivateKey, n)
	for i := range keys {
		privateKey := new(ecdsa.PrivateKey)
		privateKey, err = crypto.GenerateKey()
		if err != nil {
			return
		}
		keys[i] = privateKey
	}
	return
}
