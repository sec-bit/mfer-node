package mferbackend

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/sec-bit/mfer-safe/constant"
	"github.com/sec-bit/mfer-safe/mferevm"
	"github.com/sec-bit/mfer-safe/mfertxpool"
)

type MferBackend struct {
	EVM                 *mferevm.MferEVM
	TxPool              *mfertxpool.MferTxPool
	ImpersonatedAccount common.Address
}

func NewMferBackend(e *mferevm.MferEVM, txPool *mfertxpool.MferTxPool, impersonatedAccount common.Address) *MferBackend {
	return &MferBackend{
		EVM:                 e,
		TxPool:              txPool,
		ImpersonatedAccount: impersonatedAccount,
	}
}

func (b *MferBackend) Accounts() []common.Address {
	return []common.Address{
		b.ImpersonatedAccount,
		constant.FAKE_ACCOUNT_0,
		constant.FAKE_ACCOUNT_1,
		constant.FAKE_ACCOUNT_2,
		constant.FAKE_ACCOUNT_3,
	}
}
