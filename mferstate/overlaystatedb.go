package mferstate

import (
	"bytes"
	"context"
	"log"
	"math/big"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/kataras/golog"
	"github.com/sec-bit/mfer-safe/constant"
	"github.com/sec-bit/mfer-safe/utils"
)

type OverlayStateDB struct {
	ctx  context.Context
	ec   *rpc.Client
	conn *ethclient.Client
	// block     int
	refundGas uint64
	state     *OverlayState
	stateBN   *uint64
}

func (db *OverlayStateDB) GetOverlayDepth() int64 {
	return db.state.deriveCnt
}

func NewOverlayStateDB(rpcClient *rpc.Client, blockNumber *uint64, keyCacheFilePath string, batchSize int) (db *OverlayStateDB) {
	db = &OverlayStateDB{
		ctx:       context.Background(),
		ec:        rpcClient,
		conn:      ethclient.NewClient(rpcClient),
		refundGas: 0,
		stateBN:   blockNumber,
	}
	state := NewOverlayState(db.ctx, db.ec, db.stateBN, keyCacheFilePath, batchSize).Derive("protect underlying") // protect underlying state
	db.state = state
	return db
}

func (db *OverlayStateDB) InitState(fetchNewState bool) {
	utils.PrintMemUsage("[before init]")
	tmpDB := db.state
	reason := "reset and protect underlying"
	for {
		if tmpDB.parent == nil {
			db.state = tmpDB

			golog.Infof("Resetting Scratchpad... BN: %d", *db.stateBN)
			if fetchNewState {
				db.state.resetScratchPad()
			}
			golog.Info(reason)
			// log.Printf("pre driveID: %d", db.state.deriveCnt)
			db.state = db.state.Derive(reason)
			// log.Printf("post driveID: %d", db.state.deriveCnt)
			break
		} else {
			// log.Printf("pop scratchPad from: %d", tmpDB.deriveCnt)
			tmpDB.txLogs = nil
			tmpDB.scratchPad = nil
			tmpDB.receipts = nil
			tmpDB = tmpDB.Parent()
		}
	}
	utils.PrintMemUsage("[current]")
}

func (db *OverlayStateDB) CreateAccount(account common.Address) {}

func (db *OverlayStateDB) SubBalance(account common.Address, delta *big.Int) {
	bal, err := db.state.get(account, GET_BALANCE, common.Hash{})
	if err != nil {
		log.Panic(err)
	}
	balB := new(big.Int).SetBytes(bal)
	post := balB.Sub(balB, delta)
	db.state.scratchPad[calcKey(BALANCE_KEY, account)] = post.Bytes()
}

func (db *OverlayStateDB) AddBalance(account common.Address, delta *big.Int) {
	bal, err := db.state.get(account, GET_BALANCE, common.Hash{})
	if err != nil {
		log.Panic(err)
	}
	balB := new(big.Int).SetBytes(bal)
	post := balB.Add(balB, delta)
	db.state.scratchPad[calcKey(BALANCE_KEY, account)] = post.Bytes()
}

func (db *OverlayStateDB) InitFakeAccounts() {
	db.AddBalance(constant.FAKE_ACCOUNT_0, new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1000)))
	db.AddBalance(constant.FAKE_ACCOUNT_1, new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1000)))
	db.AddBalance(constant.FAKE_ACCOUNT_2, new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1000)))
	db.AddBalance(constant.FAKE_ACCOUNT_3, new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1000)))
	db.AddBalance(constant.FAKE_ACCOUNT_RICH, new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1_000_000_000)))
}

func (db *OverlayStateDB) GetBalance(account common.Address) *big.Int {
	bal, err := db.state.get(account, GET_BALANCE, common.Hash{})
	if err != nil {
		log.Panic(err)
	}
	balB := new(big.Int).SetBytes(bal)
	return balB
}

func (db *OverlayStateDB) SetBalance(account common.Address, balance *big.Int) {
	db.state.scratchPad[calcKey(BALANCE_KEY, account)] = balance.Bytes()
}

func (db *OverlayStateDB) GetNonce(account common.Address) uint64 {
	nonce, err := db.state.get(account, GET_NONCE, common.Hash{})
	if err != nil {
		log.Panic(err)
	}
	nonceB := new(big.Int).SetBytes(nonce)
	return nonceB.Uint64()
}
func (db *OverlayStateDB) SetNonce(account common.Address, nonce uint64) {
	db.state.scratchPad[calcKey(NONCE_KEY, account)] = big.NewInt(int64(nonce)).Bytes()
}

func (db *OverlayStateDB) GetCodeHash(account common.Address) common.Hash {
	codehash, err := db.state.get(account, GET_CODEHASH, common.Hash{})
	if err != nil {
		log.Panic(err)
	}
	return common.BytesToHash(codehash)
}

func (db *OverlayStateDB) SetCodeHash(account common.Address, codeHash common.Hash) {
	db.state.scratchPad[calcKey(CODEHASH_KEY, account)] = codeHash.Bytes()
	if account.Hex() != (common.Address{}).Hex() {
		// log.Printf("SetCodeHash[depth:%d]: acc: %s key: %s, codehash: %s", db.state.deriveCnt, account.Hex(), calcKey( CODEHASH_KEY).Hex(), codeHash.Hex())
	}
}

func (db *OverlayStateDB) GetCode(account common.Address) []byte {
	code, err := db.state.get(account, GET_CODE, common.Hash{})
	if err != nil {
		log.Panic(err)
	}
	return code
}

func (db *OverlayStateDB) SetCode(account common.Address, code []byte) {
	db.state.scratchPad[calcKey(CODE_KEY, account)] = code
}

func (db *OverlayStateDB) GetCodeSize(account common.Address) int {
	code, err := db.state.get(account, GET_CODE, common.Hash{})
	if err != nil {
		log.Panic(err)
	}
	return len(code)
}

func (db *OverlayStateDB) AddRefund(delta uint64) { db.refundGas += delta }
func (db *OverlayStateDB) SubRefund(delta uint64) { db.refundGas -= delta }
func (db *OverlayStateDB) GetRefund() uint64      { return db.refundGas }

func (db *OverlayStateDB) GetCommittedState(account common.Address, key common.Hash) common.Hash {
	val, err := db.state.get(account, GET_STATE, key)
	if err != nil {
		log.Panic(err)
	}
	return common.BytesToHash(val)
}

func (db *OverlayStateDB) GetState(account common.Address, key common.Hash) common.Hash {
	v := db.GetCommittedState(account, key)
	// log.Printf("[R depth:%d, stateID:%02x] Acc: %s K: %s V: %s", db.state.deriveCnt, db.state.stateID, account.Hex(), key.Hex(), v.Hex())
	// log.Printf("Fetched: %s [%s] = %s", account.Hex(), key.Hex(), v.Hex())
	return v
}

func (db *OverlayStateDB) SetState(account common.Address, key common.Hash, value common.Hash) {
	// log.Printf("[W depth:%d stateID:%02x] Acc: %s K: %s V: %s", db.state.deriveCnt, db.state.stateID, account.Hex(), key.Hex(), value.Hex())
	db.state.scratchPad[calcStateKey(account, key)] = value.Bytes()
}

func (db *OverlayStateDB) Suicide(account common.Address) bool {
	db.state.scratchPad[calcKey(SUICIDE_KEY, account)] = []byte{0x01}
	return true
}

func (db *OverlayStateDB) HasSuicided(account common.Address) bool {
	if val, ok := db.state.scratchPad[calcKey(SUICIDE_KEY, account)]; ok {
		return bytes.Equal(val, []byte{0x01})
	}
	return false
}

func (db *OverlayStateDB) Exist(account common.Address) bool {
	return !db.Empty(account)
}

func (db *OverlayStateDB) Empty(account common.Address) bool {
	code := db.GetCode(account)
	nonce := db.GetNonce(account)
	balance := db.GetBalance(account)
	if len(code) == 0 && nonce == 0 && balance.Sign() == 0 {
		return true
	}
	return false
}

func (db *OverlayStateDB) PrepareAccessList(sender common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
}

func (db *OverlayStateDB) AddressInAccessList(addr common.Address) bool { return true }

func (db *OverlayStateDB) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return true, true
}

func (db *OverlayStateDB) AddAddressToAccessList(addr common.Address) { return }

func (db *OverlayStateDB) AddSlotToAccessList(addr common.Address, slot common.Hash) { return }

func (db *OverlayStateDB) RevertToSnapshot(revisionID int) {
	tmpState := db.state.Parent()
	golog.Infof("Rollbacking... revision: %d, currentID: %d", revisionID, tmpState.deriveCnt)
	for {
		if tmpState.deriveCnt+1 == int64(revisionID) {
			db.state = tmpState
			break
		} else {
			tmpState = tmpState.Parent()
		}
	}
}

func (db *OverlayStateDB) Snapshot() int {
	newOverlayState := db.state.Derive("snapshot")
	db.state = newOverlayState
	revisionID := int(newOverlayState.deriveCnt)
	return revisionID
}

func (db *OverlayStateDB) MergeTo(revisionID int) {
	currState, parentState := db.state, db.state.parent
	golog.Infof("Merging... target revisionID: %d, currentID: %d", revisionID, currState.deriveCnt)
	for {
		if currState.deriveCnt == int64(revisionID) {
			db.state = currState
			break
		}
		for k, v := range currState.scratchPad {
			parentState.scratchPad[k] = v
		}
		currState, parentState = parentState, parentState.parent
	}
}

func (db *OverlayStateDB) Clone() *OverlayStateDB {
	cpy := &OverlayStateDB{
		ctx:  db.ctx,
		ec:   db.ec,
		conn: db.conn,
		// block:     db.block,
		refundGas: 0,
		state:     db.state.Derive("clone"),
	}
	return cpy
}

func (db *OverlayStateDB) CloneFromRoot() *OverlayStateDB {
	cpy := &OverlayStateDB{
		ctx:  db.ctx,
		ec:   db.ec,
		conn: db.conn,
		// block:     db.block,
		refundGas: 0,
		state:     db.state.DeriveFromRoot(),
	}
	return cpy
}

func (db *OverlayStateDB) CacheSize() (size int) {
	root := db.state.getRootState()
	root.scratchPadMutex.RLock()
	defer root.scratchPadMutex.RUnlock()
	for k, v := range root.scratchPad {
		size += (len(k) + len(v))
	}
	return size
}

func (db *OverlayStateDB) RPCRequestCount() (cnt int64) {
	return db.state.getRootState().rpcCnt
}

func (db *OverlayStateDB) StateBlockNumber() (cnt uint64) {
	return *db.stateBN
}

func (db *OverlayStateDB) AddLog(vLog *types.Log) {
	golog.Debugf("StateID: %02x, AddLog: %s", db.state.stateID, spew.Sdump(vLog))
	db.state.txLogs[db.state.currentTxHash] = append(db.state.txLogs[db.state.currentTxHash], vLog)
}

func (db *OverlayStateDB) GetLogs(txHash common.Hash) []*types.Log {
	tmpStateDB := db.state
	logs := make([]*types.Log, 0)
	for {
		if tmpStateDB == nil {
			break
		}
		if tmpStateDB.txLogs[txHash] != nil {
			golog.Debugf("StateID: %02x, GetLogs: %s", db.state.stateID, spew.Sdump(tmpStateDB.txLogs[txHash]))
			logs = append(tmpStateDB.txLogs[txHash], logs...)
		}
		tmpStateDB = tmpStateDB.parent
	}
	return logs
}

func (db *OverlayStateDB) AddReceipt(txHash common.Hash, receipt *types.Receipt) {
	golog.Debugf("StateID: %02x, AddReceipt: %s", db.state.stateID, spew.Sdump(receipt))
	db.state.receipts[txHash] = receipt
}

func (db *OverlayStateDB) GetReceipt(txHash common.Hash) *types.Receipt {
	tmpStateDB := db.state
	for {
		if tmpStateDB.parent == nil {
			return nil
		}
		if receipt, ok := tmpStateDB.receipts[txHash]; ok {
			receipt.Logs = db.GetLogs(txHash)
			return receipt
		}
		tmpStateDB = tmpStateDB.parent
	}
}

func (db *OverlayStateDB) AddPreimage(common.Hash, []byte) {}

func (db *OverlayStateDB) ForEachStorage(account common.Address, callback func(common.Hash, common.Hash) bool) error {
	return nil
}

func (db *OverlayStateDB) StartLogCollection(txHash, blockHash common.Hash) {
	db.state.currentTxHash = txHash
	db.state.currentBlockHash = blockHash
}

func (db *OverlayStateDB) SetBatchSize(batchSize int) {
	db.state.getRootState().batchSize = batchSize
}
