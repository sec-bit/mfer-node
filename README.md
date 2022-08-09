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
