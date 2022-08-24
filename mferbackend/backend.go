package mferbackend

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/sec-bit/mfer-node/constant"
	"github.com/sec-bit/mfer-node/mferevm"
	"github.com/sec-bit/mfer-node/mfertxpool"
)

type MferBackend struct {
	EVM                 *mferevm.MferEVM
	TxPool              *mfertxpool.MferTxPool
	ImpersonatedAccount common.Address
	Randomized          bool
	Passthrough         bool
}

func NewMferBackend(e *mferevm.MferEVM, txPool *mfertxpool.MferTxPool, impersonatedAccount common.Address, randomize bool) *MferBackend {
	return &MferBackend{
		EVM:                 e,
		TxPool:              txPool,
		ImpersonatedAccount: impersonatedAccount,
		Randomized:          randomize,
	}
}

func (b *MferBackend) Accounts() []common.Address {
	walletAccounts := []common.Address{
		b.ImpersonatedAccount,
		constant.FAKE_ACCOUNT_0,
		constant.FAKE_ACCOUNT_1,
		constant.FAKE_ACCOUNT_2,
		constant.FAKE_ACCOUNT_3,
	}
	if b.Randomized {
		walletAccounts[0] = constant.FAKE_ACCOUNT_RAND
	}
	return walletAccounts

}
