# mfer-node
mfer-node is an Ethereum transaction simulator based on go-ethereum.

## What can mfer-node do
* Provide a standard web3 rpc like geth provides, so you can use a wallet to connect to it.
* Connect to most of Ethereum web3 rpc and fork the network.
* Detect missing state and re-fork at latest block heigh automaticlly.
* Local mempool to keep state mutation.
* Block pinning by adding a postfix `@height`
* Trace blocks by specific range (Optimised for L2).
* Auto merge batch request so you can even use a public RPC to get a good trace performance.

# Build Instructions

`go mod download`
`go build -o mfer-node ./cmd/mfer-node`

or use docker:

Build: `docker build -t local/mfer-node:latest  -f Dockerfile .`
Run: `docker run -d -p 8545:8545 -p 10545:10545 local/mfer-node mfer-node --upstream https://rpc.ankr.com/eth`

or via docker-compose: 
`docker-compose up -d`

# Usage

`mfer-node --help` to get all the available commands. 
