package main

import (
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	_ "github.com/ethereum/go-ethereum/eth/tracers/js"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/kataras/golog"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sec-bit/mfer-node/mferbackend"
	"github.com/sec-bit/mfer-node/mferevm"
	"github.com/sec-bit/mfer-node/mfertxpool"
)

func defaultKeyCacheFilePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Panic(err)
	}
	cacheDir = path.Join(cacheDir, "MferSafe")
	err = os.MkdirAll(cacheDir, os.ModePerm)
	if err != nil {
		log.Panic(err)
	}
	fileName := "scratchPadKeyCache.txt"
	return path.Join(cacheDir, fileName)
}

const VERSION = "0.1.6"

func main() {
	account := flag.String("account", "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "impersonate account")
	rand := flag.Bool("rand", false, "randomize account")
	passthrough := flag.Bool("passthrough", true, "passthough call (forward call request to upstream, faster and less privacy)")
	upstreamURL := flag.String("upstream", "http://localhost:8545", "upstream node")
	listenURL := flag.String("listen", "127.0.0.1:10545", "web3provider bind address port")

	keyCacheFilePath := flag.String("keycache", defaultKeyCacheFilePath(), "state key cache file path")
	maxKeyCache := flag.Uint64("maxkeys", 100, "max keys stored")

	batchSize := flag.Int("batchsize", 100, "batch request size")
	logPath := flag.String("logpath", "./mfer-node.log", "path to log file")
	chainID := flag.Uint64("chainid", 0, "chainid override (0 for auto detect)")
	debugLevel := flag.String("debug", "info", "debug level")
	version := flag.Bool("version", false, "show version")
	flag.Parse()

	if *version {
		fmt.Println("mfer-node version:", VERSION)
		os.Exit(0)
	}
	pathToLog := *logPath
	// pathToLog += ".%Y%m%d%H%M.log"
	rl, err := rotatelogs.New(
		pathToLog,
		rotatelogs.WithMaxAge(time.Hour*72),
	)
	if err != nil {
		golog.Fatal(err)
	}
	if *logPath == "" {
		myLogger := log.New(os.Stdout, "", 0)
		golog.InstallStd(myLogger)
	}
	golog.SetOutput(rl)
	golog.SetTimeFormat("2006/01/02 15:04:05.000000")
	golog.SetLevel(*debugLevel)

	splittedListen := strings.Split(*listenURL, ":")
	listenAddr := splittedListen[0]
	listenPort, err := strconv.Atoi(splittedListen[1])
	if err != nil {
		golog.Fatal(err)
	}

	stack, err := node.New(&node.Config{
		Name: "mfer-safe",
		P2P: p2p.Config{
			NoDial:     true,
			ListenAddr: "",
		},
		HTTPHost:         listenAddr,
		HTTPPort:         listenPort,
		HTTPCors:         []string{"*"},
		HTTPVirtualHosts: []string{"*"},
	})
	if err != nil {
		log.Panic(err)
	}

	impersonatedAccount := common.HexToAddress(*account)
	mferEVM := mferevm.NewMferEVM(*upstreamURL, impersonatedAccount, *keyCacheFilePath, *maxKeyCache, *batchSize)
	txPool := mfertxpool.NewMferTxPool()
	b := mferbackend.NewMferBackend(mferEVM, txPool, impersonatedAccount, *rand)
	b.Passthrough = *passthrough
	if *chainID != 0 {
		b.OverrideChainID = big.NewInt(int64(*chainID))
	}
	stack.RegisterAPIs(mferbackend.GetEthAPIs(b))
	if err := stack.Start(); err != nil {
		log.Panic(err)
	}

	selfRPCClient, err := stack.Attach()
	if err != nil {
		log.Panic(err)
	}

	mferEVM.SelfClient = selfRPCClient
	mferEVM.SelfConn = ethclient.NewClient(selfRPCClient)

	select {}
}
