package mferevm

import (
	"context"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestEVMExecute(t *testing.T) {
	mferEVM := NewMferEVM("http://tractor.local:8545", common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), "./keycache.txt", 50)
	mferEVM.Prepare()

	tx, _, _ := mferEVM.Conn.TransactionByHash(context.Background(), common.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))

	txs := make(types.Transactions, 2)
	txs[0] = tx
	txs[1] = tx

	mferEVM.ExecuteTxs(txs, nil, nil)
}

func TestGetBlockHeader(t *testing.T) {
	mferEVM := NewMferEVM("https://arb1.arbitrum.io/rpc", common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), "./cache.txt", 50)
	header := mferEVM.GetBlockHeader("0x124bb29")
	spew.Dump(header)
}
