package database

import (
	"errors"
	"fmt"
	"github.com/okuralabs/okura-node/logger"
	"os"
	"path/filepath"
	"sync"
	"time"

	commoneth "github.com/okuralabs/okura-node/common"
	"github.com/tecbot/gorocksdb"
)

type BlockchainDB struct {
	db    *gorocksdb.DB
	mutex sync.RWMutex
}

func (db *BlockchainDB) GetLdb() *gorocksdb.DB {
	if db == nil {
		return nil
	}
	return db.db
}

func (db *BlockchainDB) InitPermanent(dbPath string) (*BlockchainDB, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	var err error
	db.mutex.Lock()
	defer db.mutex.Unlock()

	// Create the database directory if it doesn't exist
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Check if the database directory exists
	if _, err := os.Stat(dbPath); err == nil {
		// If it exists, try to remove any stale lock files
		lockFile := filepath.Join(dbPath, "LOCK")
		if _, err := os.Stat(lockFile); err == nil {
			// Try to remove the lock file
			if err := os.Remove(lockFile); err != nil {
				logger.GetLogger().Printf("Warning: Could not remove stale lock file: %v", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to check database directory: %w", err)
	}

	opts := gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetErrorIfExists(false) // Don't error if database exists

	// Set max open files to prevent "too many open files" error
	opts.SetMaxOpenFiles(1000)

	// Set write buffer size and max write buffer number
	opts.SetWriteBufferSize(64 * 1024 * 1024) // 64MB
	opts.SetMaxWriteBufferNumber(3)

	db.db, err = gorocksdb.OpenDb(opts, dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return db, nil
}

func (db *BlockchainDB) InitInMemory() (*BlockchainDB, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	var err error
	db.mutex.Lock()
	defer db.mutex.Unlock()
	opts := gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetEnv(gorocksdb.NewMemEnv())
	db.db, err = gorocksdb.OpenDb(opts, "okura_node")
	if err != nil {
		logger.GetLogger().Fatal(err)
	}
	return db, nil
}

func (db *BlockchainDB) GetNode(hash commoneth.Hash) ([]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	return db.Get(hash[:])
}

func (d *BlockchainDB) Close() {
	logger.GetLogger().Println("Starting database closure...")

	// Create a channel to signal completion
	done := make(chan struct{})

	// Start the close operation in a goroutine
	go func() {
		// Try to acquire lock with timeout
		lockAcquired := make(chan bool, 1)
		go func() {
			d.mutex.Lock()
			lockAcquired <- true
		}()

		select {
		case <-lockAcquired:
			logger.GetLogger().Println("Acquired database mutex lock")
			defer d.mutex.Unlock()

			if d.db == nil {
				logger.GetLogger().Println("Database already closed")
				done <- struct{}{}
				return
			}

			logger.GetLogger().Println("Flushing pending writes...")
			fo := gorocksdb.NewDefaultFlushOptions()
			defer fo.Destroy()
			if err := d.db.Flush(fo); err != nil {
				logger.GetLogger().Printf("Error flushing database: %v", err)
			} else {
				logger.GetLogger().Println("Successfully flushed pending writes")
			}

			logger.GetLogger().Println("Closing database...")
			d.db.Close()
			logger.GetLogger().Println("Successfully closed database")

			d.db = nil
			logger.GetLogger().Println("Database closure completed successfully")
			done <- struct{}{}

		case <-time.After(1 * time.Second):
			logger.GetLogger().Println("Failed to acquire mutex lock, forcing cleanup")
			// Force cleanup without mutex
			if d.db != nil {
				d.db.Close()
				d.db = nil
			}
			done <- struct{}{}
		}
	}()

	// Wait for completion with timeout
	select {
	case <-done:
		logger.GetLogger().Println("Database closed normally")
	case <-time.After(2 * time.Second):
		logger.GetLogger().Println("Database closure timed out, forcing cleanup")
		// Last resort cleanup
		if d.db != nil {
			d.db.Close()
			d.db = nil
		}
	}
}

func (db *BlockchainDB) Put(k []byte, v []byte) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if len(k) == 0 {
		return errors.New("key cannot be empty")
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	wo := gorocksdb.NewDefaultWriteOptions()
	defer wo.Destroy()

	// Make a copy of the value to ensure it's not modified after the Put
	valueCopy := make([]byte, len(v))
	copy(valueCopy, v)

	err := db.db.Put(wo, k, valueCopy)
	if err != nil {
		return fmt.Errorf("failed to put key-value pair: %w", err)
	}
	return nil
}

func (db *BlockchainDB) LoadAllKeys(prefix []byte) ([][]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	if len(prefix) == 0 {
		return nil, errors.New("prefix cannot be empty")
	}
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	ro := gorocksdb.NewDefaultReadOptions()
	defer ro.Destroy()

	iter := db.db.NewIterator(ro)
	defer iter.Close()

	keys := [][]byte{}
	for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
		key := make([]byte, len(iter.Key().Data()))
		copy(key, iter.Key().Data())
		keys = append(keys, key)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (db *BlockchainDB) LoadAll(prefix []byte) ([][]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	if len(prefix) == 0 {
		return nil, errors.New("prefix cannot be empty")
	}
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	ro := gorocksdb.NewDefaultReadOptions()
	defer ro.Destroy()

	iter := db.db.NewIterator(ro)
	defer iter.Close()

	values := [][]byte{}
	for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
		value := make([]byte, len(iter.Value().Data()))
		copy(value, iter.Value().Data())
		values = append(values, value)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func (d *BlockchainDB) Get(key []byte) ([]byte, error) {
	if d == nil {
		return nil, fmt.Errorf("database is nil")
	}

	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if d.db == nil {
		return nil, fmt.Errorf("database is closed")
	}

	ro := gorocksdb.NewDefaultReadOptions()
	defer ro.Destroy()

	value, err := d.db.Get(ro, key)
	if err != nil {
		return nil, err
	}
	defer value.Free()

	if !value.Exists() {
		return nil, fmt.Errorf("key not found")
	}

	// Make a copy of the data to ensure it's not modified after the Get
	data := make([]byte, len(value.Data()))
	copy(data, value.Data())
	return data, nil
}

func (db *BlockchainDB) IsKey(key []byte) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database is nil")
	}
	if len(key) == 0 {
		return false, errors.New("key cannot be empty")
	}
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	ro := gorocksdb.NewDefaultReadOptions()
	defer ro.Destroy()
	value, err := db.db.Get(ro, key)
	if err != nil {
		return false, err
	}
	defer value.Free()
	return value.Exists(), nil
}

func (db *BlockchainDB) Delete(key []byte) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if len(key) == 0 {
		return errors.New("key cannot be empty")
	}
	db.mutex.RLock()
	defer db.mutex.RUnlock()
	wo := gorocksdb.NewDefaultWriteOptions()
	defer wo.Destroy()
	return db.db.Delete(wo, key)
}
