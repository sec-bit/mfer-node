package mferevm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/kataras/golog"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sec-bit/mfer-node/constant"
	"github.com/sec-bit/mfer-node/mfersigner"
	"github.com/sec-bit/mfer-node/mferstate"
)

type MferEVM struct {
	ctx        context.Context
	RpcClient  *rpc.Client
	Conn       *ethclient.Client
	SelfClient *rpc.Client
	SelfConn   *ethclient.Client

	StateDB             *mferstate.OverlayStateDB
	keyCacheFilePath    string
	maxKeyCache         uint64
	batchSize           int
	vmContext           vm.BlockContext
	gasPool             *core.GasPool
	chainConfig         *params.ChainConfig
	callMutex           *sync.RWMutex
	stateLock           *sync.RWMutex
	impersonatedAccount common.Address
	timeDelta           uint64
	blockNumberDelta    uint64
	tracer              vm.EVMLogger
	blockNumber         *uint64
	pinBlock            bool
	// specifiedBlockNumber *uint64
}

func NewMferEVM(rawurl string, impersonatedAccount common.Address, keyCacheFilePath string, maxKeyCache uint64, batchSize int) *MferEVM {
	mferEVM := &MferEVM{}
	splittedRawUrl := strings.Split(rawurl, "@")
	var specificBlock *uint64
	if len(splittedRawUrl) > 1 {
		bnStr := splittedRawUrl[len(splittedRawUrl)-1]
		bn, err := strconv.Atoi(bnStr)
		if err != nil {
			log.Panic(err)
		}
		bnU64 := uint64(bn)
		specificBlock = &bnU64
		lastIndex := strings.LastIndex(rawurl, "@"+bnStr)
		rawurl = rawurl[:lastIndex]
	}
	ctx := context.Background()
DIAL:
	RpcClient, err := rpc.DialContext(ctx, rawurl)
	if err != nil {
		golog.Errorf("Dial [%s] error: [%v] retrying", rawurl, err)
		time.Sleep(time.Second * 3)
		goto DIAL
	}
	mferEVM.ctx = ctx
	mferEVM.RpcClient = RpcClient
	mferEVM.Conn = ethclient.NewClient(RpcClient)
	mferEVM.callMutex = &sync.RWMutex{}
	mferEVM.stateLock = &sync.RWMutex{}
	mferEVM.impersonatedAccount = impersonatedAccount
	mferEVM.keyCacheFilePath = keyCacheFilePath
	mferEVM.maxKeyCache = maxKeyCache
	mferEVM.batchSize = batchSize
	mferEVM.blockNumber = new(uint64)
	if specificBlock != nil {
		mferEVM.SetBlockNumber(*specificBlock)
		mferEVM.pinBlock = true
		golog.Infof("Using specific block %d, auto update block context disabled", *specificBlock)
	} else {
		go mferEVM.updatePendingBN()
	}
	err = mferEVM.Prepare()
	if err != nil {
		golog.Errorf("Prepare error: %v", err)
		time.Sleep(time.Second)
		goto DIAL
	}
	golog.Infof("Using block %d", mferEVM.StateDB.StateBlockNumber())

	return mferEVM
}

func (a *MferEVM) StateLock() {
	a.stateLock.Lock()
}

func (a *MferEVM) StateUnlock() {
	a.stateLock.Unlock()
}

func (a *MferEVM) GetBlockHeader(blockNumber string) *types.Header {
	var raw json.RawMessage
	err := a.RpcClient.CallContext(a.ctx, &raw, "eth_getBlockByNumber", blockNumber, false)
	if err != nil {
		golog.Errorf("GetBlockHeader err: %v", err)
		return nil
	} else if len(raw) == 0 {
		golog.Errorf("GetBlockHeader: Block not found")
		return nil
	}
	// Decode header and transactions.
	var head types.Header
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil
	}

	return &head
}

// func (a *MferEVM) ResetState() {
// 	a.StateDB.InitState()
// }

func (a *MferEVM) ChainID() *big.Int {
	return a.chainConfig.ChainID
}

func (a *MferEVM) SetBlockNumber(bn uint64) {
	*a.blockNumber = bn
}

func (a *MferEVM) ResetToRoot() {
	a.StateDB.InitState(false, false)
	a.StateDB.InitFakeAccounts()
	a.gasPool = new(core.GasPool)
	a.gasPool.AddGas(a.vmContext.GasLimit)
}

func (a *MferEVM) AddGasPool() {
	a.gasPool = new(core.GasPool)
	a.gasPool.AddGas(a.vmContext.GasLimit)
}

func (a *MferEVM) Prepare() error {
	a.chainConfig = core.DefaultGenesisBlock().Config
	chainID, err := a.Conn.ChainID(a.ctx)
	if err != nil {
		return err
	}
	a.chainConfig.ChainID = chainID

	//avoid invalid opcode: SHR
	a.chainConfig.ByzantiumBlock = big.NewInt(0)
	a.chainConfig.ConstantinopleBlock = big.NewInt(0)

	getHash := func(bn uint64) common.Hash {
		blk, err := a.Conn.BlockByNumber(a.ctx, new(big.Int).SetUint64(bn))
		if err != nil {
			return common.Hash{}
		}
		return blk.Hash()
	}
	a.vmContext = vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		Coinbase:    common.HexToAddress("0xaabbccddaabbccddaabbccddaabbccddaabbccdd"),
		GetHash:     getHash,
		BaseFee:     big.NewInt(0),
		BlockNumber: big.NewInt(0),
		Time:        big.NewInt(0),
		Difficulty:  big.NewInt(0),
	}
	header := a.setVMContext()
	bn := header.Number.Uint64()
	a.SetBlockNumber(bn)
	if a.StateDB == nil {
		a.StateDB = mferstate.NewOverlayStateDB(a.RpcClient, a.blockNumber, a.keyCacheFilePath, a.maxKeyCache, a.batchSize)
	}
	a.StateDB.InitState(true, false)
	a.StateDB.InitFakeAccounts()
	a.AddGasPool()
	return nil
}

func (a *MferEVM) GetChainConfig() params.ChainConfig {
	return *a.chainConfig
}

func (a *MferEVM) SetTimeDelta(delta uint64) {
	a.timeDelta = delta
}

func (a *MferEVM) GetTimeDelta() uint64 {
	return a.timeDelta
}

func (a *MferEVM) SetBlockNumberDelta(delta uint64) {
	a.blockNumberDelta = delta
}

func (a *MferEVM) GetBlockNumberDelta() uint64 {
	return a.blockNumberDelta
}

func (a *MferEVM) setVMContext() (header *types.Header) {
	if a.pinBlock {
		golog.Debugf("pinblock: %v, bn: %d", a.pinBlock, *a.blockNumber)
		header = a.GetBlockHeader(fmt.Sprintf("0x%x", *a.blockNumber))
	} else {
		golog.Debugf("pinblock: %v, bn: latest", a.pinBlock)
		header = a.GetBlockHeader("latest")
	}
	if header == nil {
		return
	}

	a.vmContext.Coinbase = header.Coinbase // use real world coinbase to avoid simulation cheating
	a.vmContext.BlockNumber.SetInt64(int64(header.Number.Uint64() + 1 + a.blockNumberDelta))
	a.vmContext.Time.SetInt64(int64(header.Time + a.timeDelta))
	a.vmContext.Difficulty.Set(header.Difficulty)
	a.vmContext.GasLimit = header.GasLimit
	return
}

func (a *MferEVM) SetVMContextByBlockHeader(header *types.Header) {
	a.vmContext.BlockNumber.SetInt64(int64(header.Number.Uint64()))
	a.vmContext.Time.SetInt64(int64(header.Time + a.timeDelta))
	a.vmContext.Difficulty.Set(header.Difficulty)
	a.vmContext.GasLimit = header.GasLimit
}

func (a *MferEVM) GetVMContext() vm.BlockContext {
	return a.vmContext
}

func (a *MferEVM) updatePendingBN() {
	headerChan := make(chan *types.Header)
	ticker5Sec := time.NewTicker(time.Second * 5)
	tickerCheckMissingTireNode := time.NewTicker(time.Second * 10)

	sub, err := a.Conn.SubscribeNewHead(a.ctx, headerChan)
	if err != nil {
		golog.Warnf("subscribe err: %v, use poll instead", err)
	} else {
		ticker5Sec.Stop()
		go func() {
			for {
				<-sub.Err()
				golog.Errorf("sub err=%v, resubscribing", err)
			RESUB:
				sub, err = a.Conn.SubscribeNewHead(a.ctx, headerChan)
				if err != nil {
					golog.Errorf("sub err=%v, retrying", err)
					time.Sleep(time.Second)
					goto RESUB
				}
			}
		}()

	}
	for {
		select {
		case <-tickerCheckMissingTireNode.C:
			if a.StateDB == nil {
				continue
			}
			stateHeight := a.StateDB.StateBlockNumber()
			golog.Infof("Checking if height@%d(0x%02x) is missing", stateHeight, stateHeight)
			balance, err := a.Conn.BalanceAt(a.ctx, common.HexToAddress("0x0000000000000000000000000000000000000000"), big.NewInt(int64(stateHeight)))
			if err != nil {
				golog.Error(err)
			}
			shouldUpdateBN := false
			if err != nil && strings.Contains(err.Error(), "missing trie node") {
				golog.Warn("InitState (missing trie node)")
				shouldUpdateBN = true
			} else if balance.Sign() == 0 { //some node will not tell us missing trie node
				golog.Warn("InitState (0x0000...0000 balance is zero)")
				shouldUpdateBN = true
			}
			if shouldUpdateBN {
				// a.StateDB.InitState()
				// header := a.setVMContext()
				// a.SetBlockNumber(header.Number.Uint64())
				a.SelfClient.Call(nil, "mfer_reExecTxPool")
			}

		case <-ticker5Sec.C:
			a.setVMContext()
		case <-headerChan:
			a.setVMContext()
		}
		if a.StateDB == nil {
			continue
		}
		sizeStr := humanize.Bytes(uint64(a.StateDB.CacheSize()))
		golog.Infof("[Update] BN: %d, StateBlock: %d, Ts: %d, Diff: %d, GasLimit: %d, Cache: %s, RPCReq: %d",
			a.vmContext.BlockNumber, a.StateDB.StateBlockNumber(), a.vmContext.Time, a.vmContext.Difficulty, a.vmContext.GasLimit, sizeStr, a.StateDB.RPCRequestCount())
	}

}

var (
	rootHash  = crypto.Keccak256Hash([]byte("fake state root"))
	blockHash = crypto.Keccak256Hash([]byte("fake block hash"))
)

func (a *MferEVM) SetTracer(t vm.EVMLogger) {
	a.tracer = t
}

func (a *MferEVM) TxToMessage(tx *types.Transaction) types.Message {
	v, r, s := tx.RawSignatureValues()
	var signer types.Signer
	if v.Uint64() == 1 && bytes.Equal(s.Bytes(), constant.MFERSIGNER_S.Bytes()) && r != nil {
		signer = mfersigner.NewSigner(a.ChainID().Int64())
	} else {
		signer = types.NewLondonSigner(a.ChainID())
	}
	msg, _ := tx.AsMessage(signer, nil)
	return msg
}

// WarmUpCache is a specular method, it execute txs parallely to make batch getStorageAt request
func (a *MferEVM) WarmUpCache(txs types.Transactions, stateDB *mferstate.OverlayStateDB) {
	golog.Infof("Warming up %d txs", len(txs))
	start := time.Now()
	wg := sync.WaitGroup{}
	txCh := make(chan *types.Transaction, 100)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(db *mferstate.OverlayStateDB) {
			defer wg.Done()
			stateDB := db.Clone()
			for tx := range txCh {
				msg := a.TxToMessage(tx)
				gp := new(core.GasPool)
				gp.AddGas(math.MaxUint64)
				// stateDB.(*mferstate.OverlayStateDB).SetCodeHash(msg.From(), common.Hash{})
				txContext := core.NewEVMTxContext(msg)
				evm := vm.NewEVM(a.vmContext, txContext, stateDB, a.chainConfig, vm.Config{})
				stateDB.StartLogCollection(tx.Hash(), blockHash)
				core.ApplyMessage(evm, msg, gp)
			}
		}(stateDB)
	}
	for _, tx := range txs {
		txCh <- tx
	}
	close(txCh)
	wg.Wait()
	cacheSize := stateDB.CloneFromRoot().CacheSize()
	golog.Infof("Warmed up %d caches (consumes: %s)", cacheSize, time.Since(start))
}

func (a *MferEVM) ExecuteTxs(txs types.Transactions, stateDB *mferstate.OverlayStateDB, config *tracers.TraceConfig) (execResults []error) {
	execResults = make([]error, len(txs))
	var (
		gasUsed = uint64(0)
		txIndex = 0
	)
	for i, tx := range txs {
		// just try some txs
		if i < 100 {
			a.WarmUpCache(txs[i:], stateDB.Clone())
		}
		msg := a.TxToMessage(tx)
		gas, result := a.ExecuteMsg(stateDB, msg, tx.Hash(), i, config)
		gasUsed += gas
		execResults[i] = result
		txIndex++
	}

	return
}

func (a *MferEVM) ExecuteMsg(stateDB *mferstate.OverlayStateDB, msg types.Message, txHash common.Hash, txIndex int, config *tracers.TraceConfig) (gasUsed uint64, execResult error) {
	stateDB.SetCodeHash(msg.From(), common.Hash{})
	txContext := core.NewEVMTxContext(msg)
	snapshot := stateDB.Snapshot()
	var (
		tracer tracers.Tracer
		err    error
	)
	txctx := &tracers.Context{
		BlockHash: blockHash,
		TxHash:    txHash,
		TxIndex:   txIndex,
	}

	switch {
	case config != nil && config.Tracer != nil:
		// Define a meaningful timeout of a single transaction trace
		timeout := time.Second * 1
		if config.Timeout != nil {
			if timeout, err = time.ParseDuration(*config.Timeout); err != nil {
				return 0, err
			}
		}
		// Constuct the JavaScript tracer to execute with
		if tracer, err = tracers.New(*config.Tracer, txctx, config.TracerConfig); err != nil {
			return 0, err
		}
		// Handle timeouts and RPC cancellations
		deadlineCtx, cancel := context.WithTimeout(context.Background(), timeout)
		go func() {
			<-deadlineCtx.Done()
			if deadlineCtx.Err() == context.DeadlineExceeded {
				tracer.Stop(errors.New("execution timeout"))
			}
		}()
		defer cancel()

	case config == nil:
		tracer, err = tracers.New("callTracer", txctx, nil)
		if err != nil {
			log.Panic(err)
		}
	default:
		config.EnableMemory = true
		config.EnableReturnData = true
		config.DisableStorage = false
		tracer = logger.NewStructLogger(config.Config)
	}

	evm := vm.NewEVM(a.vmContext, txContext, stateDB, a.chainConfig, vm.Config{
		Debug:  true,
		Tracer: tracer,
	})

	stateDB.StartLogCollection(txHash, blockHash)
	msgResult, err := core.ApplyMessage(evm, msg, a.gasPool)
	if err != nil {
		golog.Errorf("rejected tx: %s, from: %s, err: %v", txHash.Hex(), msg.From(), err)
		stateDB.RevertToSnapshot(snapshot)
		return 0, err
	}
	var msgExecErr error
	if len(msgResult.Revert()) > 0 || msgResult.Err != nil {
		// spew.Dump(msgResult.Revert(), msgResult.Err)
		reason, errUnpack := abi.UnpackRevert(msgResult.Revert())
		msgExecErr = errors.New("execution reverted")
		if errUnpack == nil {
			msgExecErr = fmt.Errorf("execution reverted: %v", reason)
		}
		golog.Errorf("TxIdx: %d, Hash: %s, unwrapped: %v, err: %v", txIndex, txHash.Hex(), msgResult.Unwrap(), msgExecErr)
	}
	gasUsed = msgResult.UsedGas
	receipt := &types.Receipt{Type: types.LegacyTxType, PostState: rootHash.Bytes(), CumulativeGasUsed: gasUsed}
	if msgResult.Failed() {
		receipt.Status = types.ReceiptStatusFailed
	} else {
		receipt.Status = types.ReceiptStatusSuccessful
	}
	receipt.TxHash = txHash
	receipt.BlockHash = blockHash
	receipt.BlockNumber = a.vmContext.BlockNumber
	receipt.GasUsed = msgResult.UsedGas

	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(evm.TxContext.Origin, msg.Nonce())
	}

	traceResult, err := tracer.GetResult()
	if err != nil {
		golog.Error(err)
	}

	txExecutionLogs := stateDB.GetLogs(txHash)
	traceLogs := &types.Log{
		Address: common.HexToAddress("0x3fe75afe000000003fe75afe000000003fe75afe"),
		Topics:  []common.Hash{crypto.Keccak256Hash([]byte("TRACE"))},
		Data:    traceResult,
	}
	receipt.Logs = append(txExecutionLogs, traceLogs)
	receipt.TransactionIndex = uint(txIndex)
	stateDB.AddLog(traceLogs)
	stateDB.AddReceipt(txHash, receipt)
	return gasUsed, msgExecErr
}

func (a *MferEVM) DoCall(msg *types.Message, debug bool, stateDB *mferstate.OverlayStateDB) (*core.ExecutionResult, error) {
	txContext := core.NewEVMTxContext(msg)

	// a.callMutex.Lock()
	// log.Printf("DoCall clone from depth: %d", a.StateDB.GetOverlayDepth())
	// clonedDB := a.StateDB.Clone()

	vmCfg := vm.Config{
		Debug:  debug,
		Tracer: a.tracer,
	}

	stateDB.SetCodeHash(msg.From(), common.Hash{})
	evm := vm.NewEVM(a.vmContext, txContext, stateDB, a.chainConfig, vmCfg)

	gasPool := new(core.GasPool).AddGas(math.MaxUint64)
	result, err := core.ApplyMessage(evm, msg, gasPool)
	if err != nil {
		return result, fmt.Errorf("err: %w (supplied gas %d)", err, msg.Gas())
	}

	return result, nil
}
