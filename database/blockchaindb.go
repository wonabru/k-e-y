package database

import (
	"github.com/okuralabs/okura-node/logger"
	"os"
	"path/filepath"
	"sync"

	"github.com/okuralabs/okura-node/common"
)

var (
	MainDB *BlockchainDB
)

func init() {
	// Get home directory
	homePath, err := os.UserHomeDir()
	if err != nil {
		logger.GetLogger().Fatal("failed to get home directory:", err)
	}

	// ONLY FOR TESTING
	os.RemoveAll(homePath + common.DefaultBlockchainHomePath)

	// Initialize the blockchain database
	db := &BlockchainDB{}
	blockchainPath := filepath.Join(homePath, common.DefaultBlockchainHomePath)
	pdb, err := db.InitPermanent(blockchainPath)
	if err != nil {
		logger.GetLogger().Fatal("failed to initialize blockchain database:", err)
	}
	MainDB = pdb
}

func CloseDB() error {
	var err error

	// Close the blockchain database
	if MainDB != nil {
		MainDB.mutex.Lock()
		(*MainDB).Close()
		MainDB.mutex.Unlock()
		MainDB = nil
	}

	return err
}

type InMemoryDBReader struct {
	db *BlockchainDB
}

func (r *InMemoryDBReader) Node(owner common.Hash, path []byte, hash common.Hash) ([]byte, error) {
	key := append(owner.Bytes(), path...)
	key = append(key, hash.Bytes()...)
	value, err := r.db.Get(key)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (r *InMemoryDBReader) Get(key []byte) ([]byte, error) {
	value, err := r.db.Get(key)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func GetDBPermanentInstance() *BlockchainDB {
	return &BlockchainDB{
		db:    (*MainDB).GetLdb(),
		mutex: sync.RWMutex{},
	}
}

func NewPermanentDB(dbPath string) *BlockchainDB {
	db := &BlockchainDB{}
	db.mutex = sync.RWMutex{}

	// Create a permanent database at the specified path
	permanent, err := db.InitPermanent(dbPath)
	if err != nil {
		logger.GetLogger().Printf("Failed to initialize permanent database at %s: %v", dbPath, err)
		return nil
	}
	return &BlockchainDB{
		db:    permanent.GetLdb(),
		mutex: sync.RWMutex{},
	}
}
