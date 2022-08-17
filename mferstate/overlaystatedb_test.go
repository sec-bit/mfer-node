package mferstate

import (
	"context"
	"log"
	"runtime"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sec-bit/mfer-node/utils"
)

const RPC_URL = "http://tractor.local:8545"

func TestDBDerive(t *testing.T) {
	ctx := context.Background()
	rpcClient, err := rpc.DialContext(ctx, RPC_URL)
	if err != nil {
		log.Panic(err)
	}

	conn := ethclient.NewClient(rpcClient)
	bh, err := conn.BlockNumber(ctx)
	if err != nil {
		log.Panic(err)
	}
	stateDB := NewOverlayStateDB(rpcClient, &bh, "./keycache.txt", 500)

	// vmCfg := vm.Config{}

	stateDB.InitState(true)
	// msg := types.NewMessage(constant.FAKE_ACCOUNT_0, &constant.FAKE_ACCOUNT_1, 0, big.NewInt(0), 999999999, big.NewInt(0), nil, nil, nil, nil, true)
	for j := 0; j < 10; j++ {
		state := stateDB.state
		runtime.GC()
		utils.PrintMemUsage("[before derive]")
		for i := 0; i < 10_000_000; i++ {
			state = state.Derive("test")

			// stateDB.SetCodeHash(msg.From(), common.Hash{})
			// evm := vm.NewEVM(a.vmContext, txContext, stateDB, a.chainConfig, vmCfg)

			// gasPool := new(core.GasPool).AddGas(math.MaxUint64)
			// result, err := core.ApplyMessage(evm, msg, gasPool)
			// state = state.parent
		}
		utils.PrintMemUsage("[after derive]")
		runtime.GC()
		// log.Printf("GC done.")
		// PrintMemUsage()
	}
	select {}
}
