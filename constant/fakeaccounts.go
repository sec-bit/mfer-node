package constant

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	FAKE_ACCOUNT_0 = common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	FAKE_ACCOUNT_1 = common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	FAKE_ACCOUNT_2 = common.HexToAddress("0xcccccccccccccccccccccccccccccccccccccccc")
	FAKE_ACCOUNT_3 = common.HexToAddress("0xdddddddddddddddddddddddddddddddddddddddd")

	FAKE_ACCOUNT_RICH = common.BytesToAddress(crypto.Keccak256([]byte(time.Now().String())))
	FAKE_ACCOUNT_RAND = common.BytesToAddress(crypto.Keccak256([]byte(time.Now().String())))
	//  common.HexToAddress("0x0101010101010101010101010101010101010101")
)
