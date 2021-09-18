// Copyright (c) 2017-2018 The qitmeer developers

package miner

import (
	"encoding/hex"
	"fmt"
	"github.com/Qitmeer/qitmeer/common/hash"
	"github.com/Qitmeer/qitmeer/core/blockchain"
	"github.com/Qitmeer/qitmeer/core/json"
	"github.com/Qitmeer/qitmeer/core/types"
	"github.com/Qitmeer/qitmeer/core/types/pow"
	"github.com/Qitmeer/qitmeer/rpc"
	"github.com/Qitmeer/qitmeer/rpc/client/cmds"
)

func (m *Miner) APIs() []rpc.API {
	return []rpc.API{
		{
			NameSpace: cmds.DefaultServiceNameSpace,
			Service:   NewPublicMinerAPI(m),
			Public:    true,
		},
		{
			NameSpace: cmds.MinerNameSpace,
			Service:   NewPrivateMinerAPI(m),
			Public:    false,
		},
	}
}

type PublicMinerAPI struct {
	miner *Miner
}

func NewPublicMinerAPI(m *Miner) *PublicMinerAPI {
	pmAPI := &PublicMinerAPI{miner: m}
	return pmAPI
}

//func (api *PublicMinerAPI) GetBlockTemplate(request *mining.TemplateRequest) (interface{}, error){
func (api *PublicMinerAPI) GetBlockTemplate(capabilities []string, powType byte) (interface{}, error) {
	// Set the default mode and override it if supplied.
	mode := "template"
	request := json.TemplateRequest{Mode: mode, Capabilities: capabilities, PowType: powType}
	switch mode {
	case "template":
		return handleGetBlockTemplateRequest(api, &request)
	case "proposal":
		//TODO LL, will be added
		//return handleGetBlockTemplateProposal(s, request)
	}
	return nil, rpc.RpcInvalidError("Invalid mode")
}

//LL
// handleGetBlockTemplateRequest is a helper for handleGetBlockTemplate which
// deals with generating and returning block templates to the caller. In addition,
// it detects the capabilities reported by the caller
// in regards to whether or not it supports creating its own coinbase (the
// coinbasetxn and coinbasevalue capabilities) and modifies the returned block
// template accordingly.
func handleGetBlockTemplateRequest(api *PublicMinerAPI, request *json.TemplateRequest) (interface{}, error) {
	reply := make(chan *gbtResponse)
	err := api.miner.GBTMining(request, reply)
	if err != nil {
		return nil, err
	}
	resp := <-reply
	return resp.result, resp.err
}

//LL
//Attempts to submit new block to network.
//See https://en.bitcoin.it/wiki/BIP_0022 for full specification
func (api *PublicMinerAPI) SubmitBlock(hexBlock string) (interface{}, error) {
	// Deserialize the hexBlock.
	m := api.miner

	if len(hexBlock)%2 != 0 {
		hexBlock = "0" + hexBlock
	}
	serializedBlock, err := hex.DecodeString(hexBlock)

	if err != nil {
		return nil, rpc.RpcDecodeHexError(hexBlock)
	}
	block, err := types.NewBlockFromBytes(serializedBlock)
	if err != nil {
		return nil, rpc.RpcDeserializationError("Block decode failed: %s", err.Error())
	}

	// Because it's asynchronous, so you must ensure that all tips are referenced
	if len(block.Block().Transactions) <= 0 {
		return nil, fmt.Errorf("block is illegal")
	}
	height, err := blockchain.ExtractCoinbaseHeight(block.Block().Transactions[0])
	if err != nil {
		return nil, err
	}

	block.SetHeight(uint(height))
	return m.submitBlock(block)
}

// PrivateMinerAPI provides private RPC methods to control the miner.
type PrivateMinerAPI struct {
	miner *Miner
}

func NewPrivateMinerAPI(m *Miner) *PrivateMinerAPI {
	pmAPI := &PrivateMinerAPI{miner: m}
	return pmAPI
}

func (api *PrivateMinerAPI) Generate(numBlocks uint32, powType pow.PowType) ([]string, error) {
	// Respond with an error if the client is requesting 0 blocks to be generated.
	if numBlocks == 0 {
		return nil, rpc.RpcInternalError("Invalid number of blocks",
			"Configuration")
	}
	if numBlocks > 3000 {
		return nil, fmt.Errorf("error, more than 1000")
	}

	// Create a reply
	reply := make([]string, numBlocks)

	blockHashC := make(chan *hash.Hash)
	err := api.miner.CPUMiningGenerate(int(numBlocks), blockHashC, powType)
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < numBlocks; i++ {
		select {
		case blockHash := <-blockHashC:
			if blockHash == nil {
				break
			}
			// Mine the correct number of blocks, assigning the hex representation of the
			// hash of each one to its place in the reply.
			reply[i] = blockHash.String()
		}
	}
	return reply, nil
}
