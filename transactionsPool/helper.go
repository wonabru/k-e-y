package transactionsPool

import (
	"fmt"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/logger"
	"github.com/okuralabs/okura-node/transactionsDefinition"
)

func RemoveBadTransactionByHash(hash []byte, height int64) error {
	PoolsTx.RemoveTransactionByHash(hash)
	PoolTxEscrow.RemoveTransactionByHash(hash)
	PoolTxMultiSign.RemoveTransactionByHash(hash)
	err := transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionPoolHashesDBPrefix[:], hash)
	if err != nil {
		logger.GetLogger().Println(err)
	}
	//TODO
	err = transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionDBPrefix[:], hash)
	if err != nil {
		logger.GetLogger().Println(err)
	}
	err = CheckTransactionInDBAndInMarkleTrie(hash)
	if err == nil {
		logger.GetLogger().Println("transaction is in trie")
	}
	err = RemoveMerkleTrieFromDB(height)
	if err != nil {
		logger.GetLogger().Println(err)
	}
	PoolsTx.BanTransactionByHash(hash)
	PoolTxEscrow.BanTransactionByHash(hash)
	PoolTxMultiSign.BanTransactionByHash(hash)
	return nil
}

func CheckTransactionInDBAndInMarkleTrie(hash []byte) error {
	if transactionsDefinition.CheckFromDBPoolTx(common.TransactionDBPrefix[:], hash) {
		dbTx, err := transactionsDefinition.LoadFromDBPoolTx(common.TransactionDBPrefix[:], hash)
		if err != nil {
			//TODO
			//transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionDBPrefix[:], hash)
			return err
		}
		h := dbTx.Height
		txHeight, err := FindTransactionInBlocks(hash, h)
		if err != nil {
			return err
		}
		if txHeight < 0 {
			logger.GetLogger().Println("transaction not in merkle tree. removing transaction: checkTransactionInDBAndInMarkleTrie")
			//TODO
			//err = transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionDBPrefix[:], hash)
			//if err != nil {
			//	return err
			//}
		} else {
			return fmt.Errorf("transaction was previously added in chain: checkTransactionInDBAndInMarkleTrie")
		}
	}
	return nil
}
