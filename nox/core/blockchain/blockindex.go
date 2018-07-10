// Copyright (c) 2017-2018 The nox developers
package blockchain

import (
	"sync"
	"github.com/noxproject/nox/params"
	"github.com/noxproject/nox/database"
	"github.com/noxproject/nox/common/hash"
	"github.com/noxproject/nox/core/types"
)

// IndexManager provides a generic interface that the is called when blocks are
// connected and disconnected to and from the tip of the main chain for the
// purpose of supporting optional indexes.
type IndexManager interface {
	// Init is invoked during chain initialize in order to allow the index
	// manager to initialize itself and any indexes it is managing.  The
	// channel parameter specifies a channel the caller can close to signal
	// that the process should be interrupted.  It can be nil if that
	// behavior is not desired.
	Init(*BlockChain, <-chan struct{}) error

	// ConnectBlock is invoked when a new block has been connected to the
	// main chain.
	ConnectBlock(tx database.Tx, block *types.SerializedBlock, parent *types.SerializedBlock, utxoView *UtxoViewpoint) error

	// DisconnectBlock is invoked when a block has been disconnected from
	// the main chain.
	DisconnectBlock(tx database.Tx, block *types.SerializedBlock, parent *types.SerializedBlock,  utxoView *UtxoViewpoint) error
}

// blockIndex provides facilities for keeping track of an in-memory index of the
// block chain.  Although the name block chain suggests a single chain of
// blocks, it is actually a tree-shaped structure where any node can have
// multiple children.  However, there can only be one active branch which does
// indeed form a chain from the tip all the way back to the genesis block.
type blockIndex struct {
	// The following fields are set when the instance is created and can't
	// be changed afterwards, so there is no need to protect them with a
	// separate mutex.
	db          database.DB
	params      *params.Params

	sync.RWMutex
	index     map[hash.Hash]*blockNode
	chainTips map[uint64][]*blockNode
}

// newBlockIndex returns a new empty instance of a block index.  The index will
// be dynamically populated as block nodes are loaded from the database and
// manually added.
func newBlockIndex(db database.DB, par *params.Params) *blockIndex {
	return &blockIndex{
		db:          db,
		params:      par,
		index:       make(map[hash.Hash]*blockNode),
		chainTips:   make(map[uint64][]*blockNode),
	}
}

// lookupNode returns the block node identified by the provided hash.  It will
// return nil if there is no entry for the hash.
//
// This function MUST be called with the block index lock held (for reads).
func (bi *blockIndex) lookupNode(hash *hash.Hash) *blockNode {
	return bi.index[*hash]
}

// LookupNode returns the block node identified by the provided hash.  It will
// return nil if there is no entry for the hash.
//
// This function is safe for concurrent access.
func (bi *blockIndex) LookupNode(hash *hash.Hash) *blockNode {
	bi.RLock()
	node := bi.lookupNode(hash)
	bi.RUnlock()
	return node
}

// addNode adds the provided node to the block index.  Duplicate entries are not
// checked so it is up to caller to avoid adding them.
//
// This function MUST be called with the block index lock held (for writes).
func (bi *blockIndex) addNode(node *blockNode) {
	bi.index[node.hash] = node

	// Since the block index does not support nodes that do not connect to
	// an existing node (except the genesis block), all new nodes are either
	// extending an existing chain or are on a side chain, but in either
	// case, are a new chain tip.  In the case the node is extending a
	// chain, the parent is no longer a tip.
	bi.addChainTip(node)
	if node.parent != nil {
		bi.removeChainTip(node.parent)
	}
}

// AddNode adds the provided node to the block index.  Duplicate entries are not
// checked so it is up to caller to avoid adding them.
//
// This function is safe for concurrent access.
func (bi *blockIndex) AddNode(node *blockNode) {
	bi.Lock()
	bi.addNode(node)
	bi.Unlock()
}

// addChainTip adds the passed block node as a new chain tip.
//
// This function MUST be called with the block index lock held (for writes).
func (bi *blockIndex) addChainTip(tip *blockNode) {
	bi.chainTips[tip.height] = append(bi.chainTips[tip.height], tip)
}

// removeChainTip removes the passed block node from the available chain tips.
//
// This function MUST be called with the block index lock held (for writes).
func (bi *blockIndex) removeChainTip(tip *blockNode) {
	nodes := bi.chainTips[tip.height]
	for i, n := range nodes {
		if n == tip {
			copy(nodes[i:], nodes[i+1:])
			nodes[len(nodes)-1] = nil
			nodes = nodes[:len(nodes)-1]
			break
		}
	}

	// Either update the map entry for the height with the remaining nodes
	// or remove it altogether if there are no more nodes left.
	if len(nodes) == 0 {
		delete(bi.chainTips, tip.height)
	} else {
		bi.chainTips[tip.height] = nodes
	}
}

// HaveBlock returns whether or not the block index contains the provided hash.
//
// This function is safe for concurrent access.
func (bi *blockIndex) HaveBlock(hash *hash.Hash) bool {
	bi.RLock()
	_, hasBlock := bi.index[*hash]
	bi.RUnlock()
	return hasBlock
}

// NodeStatus returns the status associated with the provided node.
//
// This function is safe for concurrent access.
func (bi *blockIndex) NodeStatus(node *blockNode) blockStatus {
	bi.RLock()
	status := node.status
	bi.RUnlock()
	return status
}

// SetStatusFlags sets the provided status flags for the given block node
// regardless of their previous state.  It does not unset any flags.
//
// This function is safe for concurrent access.
func (bi *blockIndex) SetStatusFlags(node *blockNode, flags blockStatus) {
	bi.Lock()
	node.status |= flags
	bi.Unlock()
}

// UnsetStatusFlags unsets the provided status flags for the given block node
// regardless of their previous state.
//
// This function is safe for concurrent access.
func (bi *blockIndex) UnsetStatusFlags(node *blockNode, flags blockStatus) {
	bi.Lock()
	node.status &^= flags
	bi.Unlock()
}
