// Copyright (c) 2017-2018 The qitmeer developers

package merkle

import (
	"fmt"
	"github.com/Qitmeer/qitmeer/common/hash"
	"github.com/Qitmeer/qitmeer/core/types"
	"math"
)

//TODO refactoing the merkle root calculation to support abstract merkle node

func CalcMerkleRoot(txns []*types.Transaction) *hash.Hash {
	root := calcMerkleRoot(txns)
	return &root
}

// buildMerkleTreeStore creates a merkle tree from a slice of transactions,
// stores it using a linear array, and returns a slice of the backing array.  A
// linear array was chosen as opposed to an actual tree structure since it uses
// about half as much memory.  The following describes a merkle tree and how it
// is stored in a linear array.
//
// A merkle tree is a tree in which every non-leaf node is the hash of its
// children nodes.  A diagram depicting how this works for transactions
// where h(x) is a blake256 hash follows:
//
//	         root = h1234 = h(h12 + h34)
//	        /                           \
//	  h12 = h(h1 + h2)            h34 = h(h3 + h4)
//	   /            \              /            \
//	h1 = h(tx1)  h2 = h(tx2)    h3 = h(tx3)  h4 = h(tx4)
//
// The above stored as a linear array is as follows:
//
// 	[h1 h2 h3 h4 h12 h34 root]
//
// As the above shows, the merkle root is always the last element in the array.
//
// The number of inputs is not always a power of two which results in a
// balanced tree structure as above.  In that case, parent nodes with no
// children are also zero and parent nodes with only a single left node
// are calculated by concatenating the left node with itself before hashing.
// Since this function uses nodes that are pointers to the hashes, empty nodes
// will be nil.
func BuildMerkleTreeStore(transactions []*types.Tx, witness bool) []*hash.Hash {
	// If there's an empty stake tree, return totally zeroed out merkle tree root
	// only.
	if len(transactions) == 0 {
		merkles := make([]*hash.Hash, 1)
		merkles[0] = &hash.Hash{}
		return merkles
	}

	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := nextPowerOfTwo(len(transactions))
	arraySize := nextPoT*2 - 1
	merkles := make([]*hash.Hash, arraySize)

	// Create the base transaction hashes and populate the array with them.
	for i, tx := range transactions {
		switch {
		case witness && i == 0:
			merkles[i] = &hash.ZeroHash
		case witness:
			wSha := tx.Tx.TxHashFull()
			merkles[i] = &wSha
		default:
			txH := tx.Tx.TxHash()
			merkles[i] = &txH
		}
	}

	// Start the array offset after the last transaction and adjusted to the
	// next power of two.
	offset := nextPoT
	for i := 0; i < arraySize-1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case merkles[i] == nil:
			merkles[offset] = nil

		// When there is no right child, the parent is generated by
		// hashing the concatenation of the left child with itself.
		case merkles[i+1] == nil:
			newHash := HashMerkleBranches(merkles[i], merkles[i])
			merkles[offset] = newHash

		// The normal case sets the parent node to the hash of the
		// concatentation of the left and right children.
		default:
			newHash := HashMerkleBranches(merkles[i], merkles[i+1])
			merkles[offset] = newHash
		}
		offset++
	}

	return merkles
}

// calcMerkleRoot creates a merkle tree from the slice of transactions and
// returns the root of the tree.
func calcMerkleRoot(txns []*types.Transaction) hash.Hash {
	utilTxns := make([]*types.Tx, 0, len(txns))
	for _, tx := range txns {
		utilTxns = append(utilTxns, types.NewTx(tx))
	}
	merkles := BuildMerkleTreeStore(utilTxns, false)
	return *merkles[len(merkles)-1]
}

// HashMerkleBranches takes two hashes, treated as the left and right tree
// nodes, and returns the hash of their concatenation.  This is a helper
// function used to aid in the generation of a merkle tree.
func HashMerkleBranches(left *hash.Hash, right *hash.Hash) *hash.Hash {
	// Concatenate the left and right nodes.
	var h [hash.HashSize * 2]byte
	copy(h[:hash.HashSize], left[:])
	copy(h[hash.HashSize:], right[:])

	// TODO, add an abstract layer of hash func
	// TODO, double sha256 or other crypto hash
	newHash := hash.DoubleHashH(h[:])
	return &newHash
}

// nextPowerOfTwo returns the next highest power of two from a given number if
// it is not already a power of two.  This is a helper function used during the
// calculation of a merkle tree.
func nextPowerOfTwo(n int) int {
	// Return the number if it's already a power of 2.
	if n&(n-1) == 0 {
		return n
	}

	// Figure out and return the next power of two.
	exponent := uint(math.Log2(float64(n))) + 1
	return 1 << exponent // 2^exponent
}

// BuildParentsMerkleTreeStore creates a merkle tree from a slice of block parents,
// stores it using a linear array, and returns a slice of the backing array.  A
// linear array was chosen as opposed to an actual tree structure since it uses
// about half as much memory.  The following describes a merkle tree and how it
// is stored in a linear array.
//
// A merkle tree is a tree in which every non-leaf node is the hash of its
// children nodes.  A diagram depicting how this works for block parents
// where h(x) is a blake256 hash follows:
//
//	         root = h1234 = h(h12 + h34)
//	        /                           \
//	  h12 = h(h1 + h2)            h34 = h(h3 + h4)
//	   /            \              /            \
//	h1 = h(tx1)  h2 = h(tx2)    h3 = h(tx3)  h4 = h(tx4)
//
// The above stored as a linear array is as follows:
//
// 	[h1 h2 h3 h4 h12 h34 root]
//
// As the above shows, the merkle root is always the last element in the array.
//
// The number of inputs is not always a power of two which results in a
// balanced tree structure as above.  In that case, parent nodes with no
// children are also zero and parent nodes with only a single left node
// are calculated by concatenating the left node with itself before hashing.
// Since this function uses nodes that are pointers to the hashes, empty nodes
// will be nil.
func BuildParentsMerkleTreeStore(parents []*hash.Hash) []*hash.Hash {
	// If there's an empty stake tree, return totally zeroed out merkle tree root
	// only.
	if len(parents) == 0 {
		merkles := make([]*hash.Hash, 1)
		merkles[0] = &hash.Hash{}
		return merkles
	}

	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := nextPowerOfTwo(len(parents))
	arraySize := nextPoT*2 - 1
	merkles := make([]*hash.Hash, arraySize)

	// Populate the array with hashs.
	copy(merkles, parents)

	// Start the array offset after the last parent and adjusted to the
	// next power of two.
	offset := nextPoT
	for i := 0; i < arraySize-1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case merkles[i] == nil:
			merkles[offset] = nil

			// When there is no right child, the parent is generated by
			// hashing the concatenation of the left child with itself.
		case merkles[i+1] == nil:
			newHash := HashMerkleBranches(merkles[i], merkles[i])
			merkles[offset] = newHash

			// The normal case sets the parent node to the hash of the
			// concatentation of the left and right children.
		default:
			newHash := HashMerkleBranches(merkles[i], merkles[i+1])
			merkles[offset] = newHash
		}
		offset++
	}

	return merkles
}

func ValidateWitnessCommitment(blk *types.SerializedBlock) error {
	if len(blk.Transactions()) == 0 {
		str := "cannot validate witness commitment of block without " +
			"transactions"
		return fmt.Errorf(str)
	}

	coinbaseTx := blk.Transactions()[0]
	if len(coinbaseTx.Tx.TxIn) == 0 {
		return fmt.Errorf("transaction has no inputs")
	}

	witnessCommitment := coinbaseTx.Tx.TxIn[0].PreviousOut.Hash
	if witnessCommitment.IsEqual(&hash.ZeroHash) {
		return fmt.Errorf("Coinbase inputs has no witness commitment")
	}

	coinbase := coinbaseTx.Tx.TxIn[0].SignScript
	witnessMerkleTree := BuildMerkleTreeStore(blk.Transactions(), true)
	witnessMerkleRoot := witnessMerkleTree[len(witnessMerkleTree)-1]

	witnessPreimage := append(witnessMerkleRoot.Bytes(), coinbase...)
	computedCommitment := hash.DoubleHashH(witnessPreimage[:])

	if !computedCommitment.IsEqual(&witnessCommitment) {
		str := fmt.Sprintf("witness commitment does not match: "+
			"computed %s, coinbase includes %s", computedCommitment,
			witnessCommitment)
		return fmt.Errorf(str)
	}
	return nil
}

func BuildTokenBalanceMerkleTreeStore(balance []*hash.Hash) []*hash.Hash {
	// If there's an empty stake tree, return totally zeroed out merkle tree root
	// only.
	if len(balance) == 0 {
		merkles := make([]*hash.Hash, 1)
		merkles[0] = &hash.Hash{}
		return merkles
	}

	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := nextPowerOfTwo(len(balance))
	arraySize := nextPoT*2 - 1
	merkles := make([]*hash.Hash, arraySize)

	// Populate the array with hashs.
	copy(merkles, balance)

	// Start the array offset after the last parent and adjusted to the
	// next power of two.
	offset := nextPoT
	for i := 0; i < arraySize-1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case merkles[i] == nil:
			merkles[offset] = nil

			// When there is no right child, the parent is generated by
			// hashing the concatenation of the left child with itself.
		case merkles[i+1] == nil:
			newHash := HashMerkleBranches(merkles[i], merkles[i])
			merkles[offset] = newHash

			// The normal case sets the parent node to the hash of the
			// concatentation of the left and right children.
		default:
			newHash := HashMerkleBranches(merkles[i], merkles[i+1])
			merkles[offset] = newHash
		}
		offset++
	}

	return merkles
}
