package mfersigner

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sec-bit/mfer-node/constant"
)

func NewSigner(chainID int64) *SetFromSigner {
	return &SetFromSigner{
		chainID: chainID,
	}
}

type SetFromSigner struct {
	chainID int64
}

func (signer *SetFromSigner) SignatureValues(tx *types.Transaction, sig []byte) (r, s, v *big.Int, err error) {
	return new(big.Int).SetBytes(sig), constant.MFERSIGNER_S, big.NewInt(1), nil
}
func (signer *SetFromSigner) Sender(tx *types.Transaction) (common.Address, error) {
	_, R, _ := tx.RawSignatureValues()
	return common.BigToAddress(R), nil
}

func (signer *SetFromSigner) ChainID() *big.Int {
	return big.NewInt(signer.chainID)
}

func (signer *SetFromSigner) Hash(tx *types.Transaction) common.Hash {
	return common.Hash{}
}

func (signer *SetFromSigner) Equal(signer2 types.Signer) bool {
	return true
}
