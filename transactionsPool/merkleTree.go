package transactionsPool

import (
	"bytes"
	"fmt"
	"github.com/okuralabs/okura-node/logger"
	"sync"

	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/database"
)

type MerkleTree struct {
	Root     []MerkleNode
	TxHashes [][]byte
	DB       *database.BlockchainDB
}

type MerkleNode struct {
	Left  *MerkleNode
	Right *MerkleNode
	Data  []byte
}

var GlobalMerkleTree *MerkleTree
var globalMutex sync.RWMutex

func InitPermanentTrie() {
	merkleNodes, err := NewMerkleTree([][]byte{})
	if err != nil {
		logger.GetLogger().Fatal(err)
	}

	globalMutex.Lock()
	defer globalMutex.Unlock()

	GlobalMerkleTree = new(MerkleTree)
	GlobalMerkleTree.Root = merkleNodes

	// Use the global MainDB instance
	if database.MainDB == nil {
		logger.GetLogger().Fatal("MainDB is not initialized")
	}
	GlobalMerkleTree.DB = database.MainDB
}

func GetGlobalMerkleTree() *MerkleTree {
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	return GlobalMerkleTree
}

func NewMerkleTree(data [][]byte) ([]MerkleNode, error) {
	var nodes []MerkleNode
	for _, d := range data {
		node, err := NewMerkleNode(nil, nil, d)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, *node)
	}

	for len(nodes) > 1 {
		if len(nodes)%2 != 0 {
			nodes = append(nodes, nodes[len(nodes)-1])
		}
		var level []MerkleNode
		for i := 0; i < len(nodes); i += 2 {
			node, err := NewMerkleNode(&nodes[i], &nodes[i+1], nil)
			if err != nil {
				return nil, err
			}
			level = append(level, *node)
		}
		nodes = level
	}
	return nodes, nil
}

func NewMerkleNode(left, right *MerkleNode, data []byte) (*MerkleNode, error) {
	node := MerkleNode{}

	globalMutex.Lock()
	defer globalMutex.Unlock()

	if left == nil && right == nil {
		hash, err := common.CalcHashToByte(data)
		if err != nil {
			logger.GetLogger().Println("hash calculation fails")
			return nil, err
		}
		node.Data = hash[:]
	} else {
		prevHashes := append(left.Data, right.Data...)
		hash, err := common.CalcHashToByte(prevHashes)
		if err != nil {
			logger.GetLogger().Println("hash calculation fails")
			return nil, err
		}
		node.Data = hash[:]
	}
	node.Left = left
	node.Right = right
	return &node, nil
}

func (t *MerkleTree) IsTxHashInTree(hash []byte) bool {
	if t == nil {
		return false
	}
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	left, _ := t.Root[0].containsTxHash(0, hash)
	if left {
		return true
	}
	var right bool
	if len(t.Root) > 1 {
		right, _ = t.Root[1].containsTxHash(0, hash)
	}
	return right
}

func (n *MerkleNode) containsTxHash(index int64, hash []byte) (bool, int64) {
	if n == nil {
		return false, index - 1
	}

	globalMutex.RLock()
	defer globalMutex.RUnlock()

	index++
	if bytes.Equal(n.Data, hash) {
		return true, index - 1
	}

	left, indexLeft := n.Left.containsTxHash(index, hash)
	right, indexRight := n.Right.containsTxHash(index, hash)
	contrains := left || right
	ind := index - 1
	if left {
		ind = indexLeft
	}
	if right {
		ind = indexRight
	}
	return contrains, ind
}

func (t *MerkleTree) GetRootHash() []byte {
	if t == nil {
		return common.EmptyHash().GetBytes()
	}
	globalMutex.RLock()
	defer globalMutex.RUnlock()
	if len(t.Root) > 0 {
		return t.Root[0].Data
	}
	return common.EmptyHash().GetBytes()
}

func BuildMerkleTree(height int64, blockTransactionsHashes [][]byte, db *database.BlockchainDB) (*MerkleTree, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}

	merkleNodes, err := NewMerkleTree(blockTransactionsHashes)
	if err != nil {
		return nil, err
	}

	tree := new(MerkleTree)
	globalMutex.Lock()
	defer globalMutex.Unlock()

	tree.Root = merkleNodes
	tree.DB = db
	tree.TxHashes = blockTransactionsHashes

	return tree, nil
}

func (t *MerkleTree) StoreTree(height int64) error {
	if t == nil {
		return fmt.Errorf("merkle tree is nil")
	}

	globalMutex.RLock()
	merkleNodes := t.Root
	txHashes := t.TxHashes
	db := t.DB
	globalMutex.RUnlock()

	if db == nil {
		return fmt.Errorf("database is nil")
	}

	prefix := common.MerkleNodeDBPrefix[:]
	key := append(prefix, common.GetByteInt64(height)...)

	treeb, err := common.Marshal(merkleNodes, common.MerkleNodeDBPrefix)
	if err != nil {
		return err
	}
	err = db.Put(key, treeb)
	if err != nil {
		return err
	}

	prefix = common.RootHashMerkleTreeDBPrefix[:]
	key = append(prefix, common.GetByteInt64(height)...)
	err = db.Put(key, t.GetRootHash())
	if err != nil {
		return err
	}

	ret := []byte{}
	for _, a := range txHashes {
		ret = append(ret, a...)
	}

	prefix = common.TransactionsHashesByHeightDBPrefix[:]
	key = append(prefix, common.GetByteInt64(height)...)
	err = db.Put(key, ret)
	if err != nil {
		return err
	}

	return nil
}

func LoadTxHashes(height int64) ([][]byte, error) {
	if GlobalMerkleTree == nil {
		return nil, fmt.Errorf("global merkle tree is nil")
	}

	globalMutex.RLock()
	defer globalMutex.RUnlock()

	if GlobalMerkleTree.DB == nil {
		return nil, fmt.Errorf("database is nil")
	}

	prefix := common.TransactionsHashesByHeightDBPrefix[:]
	key := append(prefix, common.GetByteInt64(height)...)
	txbytes, err := GlobalMerkleTree.DB.Get(key)
	if err != nil {
		return nil, err
	}
	len_tx := common.HashLength
	ret := [][]byte{}
	for i := 0; i < len(txbytes)/len_tx; i++ {
		ret = append(ret, txbytes[len_tx*i:len_tx*(i+1)])
	}
	return ret, nil
}

func LoadHashMerkleTreeByHeight(height int64) ([]byte, error) {
	if GlobalMerkleTree == nil {
		return nil, fmt.Errorf("global merkle tree is nil")
	}

	globalMutex.RLock()
	defer globalMutex.RUnlock()

	if GlobalMerkleTree.DB == nil {
		return nil, fmt.Errorf("database is nil")
	}

	prefix := common.RootHashMerkleTreeDBPrefix[:]
	key := append(prefix, common.GetByteInt64(height)...)
	hash, err := GlobalMerkleTree.DB.Get(key)
	if err != nil {
		return nil, err
	}
	return hash, nil
}

func (t *MerkleTree) Destroy() {
	if t != nil {
		globalMutex.Lock()
		defer globalMutex.Unlock()
		t.Root = nil
		t.TxHashes = nil
		t.DB = nil
	}
}

func LoadTreeWithoutTxHashes(height int64) (*MerkleTree, error) {
	if GlobalMerkleTree == nil {
		return nil, fmt.Errorf("global merkle tree is nil")
	}

	globalMutex.RLock()
	defer globalMutex.RUnlock()

	if GlobalMerkleTree.DB == nil {
		return nil, fmt.Errorf("database is nil")
	}

	tree := new(MerkleTree)
	prefix := common.MerkleNodeDBPrefix[:]
	key := append(prefix, common.GetByteInt64(height)...)
	treeb, err := GlobalMerkleTree.DB.Get(key)
	if err != nil {
		return &MerkleTree{}, err
	}

	var merkleNodes []MerkleNode
	err = common.Unmarshal(treeb, common.MerkleNodeDBPrefix, &merkleNodes)
	if err != nil {
		return &MerkleTree{}, err
	}
	tree.Root = merkleNodes

	prefix = common.RootHashMerkleTreeDBPrefix[:]
	key = append(prefix, common.GetByteInt64(height)...)
	rootHash, err := GlobalMerkleTree.DB.Get(key)
	if err != nil || len(tree.Root) == 0 {
		return &MerkleTree{}, err
	}
	tree.Root[0].Data = rootHash

	return tree, nil
}

func FindTransactionInBlocks(targetHash []byte, height int64) (int64, error) {
	tree, err := LoadTreeWithoutTxHashes(height)
	if err != nil {
		return -1, err
	}
	defer tree.Destroy()

	if tree == nil {
		return -1, fmt.Errorf("merkle tree is nil")
	}

	if len(tree.Root) == 0 {
		return -1, fmt.Errorf("no merkle tree root hash")
	}

	left, hl := tree.Root[0].containsTxHash(0, targetHash)
	if left {
		return hl, nil
	}

	if len(tree.Root) > 1 {
		right, hr := tree.Root[1].containsTxHash(0, targetHash)
		if right {
			return hr, nil
		}
	}

	return -1, fmt.Errorf("tx hash not found")
}

func LastHeightStoredInMerleTrie() (int64, error) {
	if GlobalMerkleTree == nil {
		return -1, fmt.Errorf("global merkle tree is nil")
	}

	globalMutex.RLock()
	defer globalMutex.RUnlock()

	if GlobalMerkleTree.DB == nil {
		return -1, fmt.Errorf("database is nil")
	}

	i := int64(0)
	for {
		ib := common.GetByteInt64(i)
		prefix := append(common.RootHashMerkleTreeDBPrefix[:], ib...)
		isKey, err := GlobalMerkleTree.DB.IsKey(prefix)
		if err != nil {
			return i - 1, err
		}
		if !isKey {
			break
		}
		i++
	}
	return i - 1, nil
}

func RemoveMerkleTrieFromDB(height int64) error {
	if GlobalMerkleTree == nil {
		return fmt.Errorf("global merkle tree is nil")
	}

	globalMutex.RLock()
	defer globalMutex.RUnlock()

	if GlobalMerkleTree.DB == nil {
		return fmt.Errorf("database is nil")
	}

	hb := common.GetByteInt64(height)
	prefix := append(common.RootHashMerkleTreeDBPrefix[:], hb...)
	err := GlobalMerkleTree.DB.Delete(prefix)
	if err != nil {
		logger.GetLogger().Println("cannot remove root merkle trie hash", err)
		return err
	}

	prefix = append(common.MerkleNodeDBPrefix[:], hb...)
	err = GlobalMerkleTree.DB.Delete(prefix)
	if err != nil {
		logger.GetLogger().Println("cannot remove merkle trie node", err)
		return err
	}

	prefix = append(common.TransactionsHashesByHeightDBPrefix[:], hb...)
	err = GlobalMerkleTree.DB.Delete(prefix)
	if err != nil {
		logger.GetLogger().Println("cannot remove merkle trie transaction hashes", err)
		return err
	}
	return nil
}
