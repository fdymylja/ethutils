package nodeop

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fdymylja/ethutils/interfaces"
	"github.com/fdymylja/utils"
)

func PullTransaction(ctx context.Context, client interfaces.TransactionPuller, transactionHash common.Hash) (tx *types.Transaction, err error) {
	defer utils.WrapErrorP(&err)
	panic("TODO")
}
