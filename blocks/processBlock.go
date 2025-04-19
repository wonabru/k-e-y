package blocks

import (
	"bytes"
	"fmt"
	"log"

	"github.com/okuralabs/okura-node/account"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/crypto/oqs"
	"github.com/okuralabs/okura-node/database"
	"github.com/okuralabs/okura-node/oracles"
	"github.com/okuralabs/okura-node/transactionsDefinition"
	"github.com/okuralabs/okura-node/transactionsPool"
	"github.com/okuralabs/okura-node/voting"
)

func CheckBaseBlock(newBlock Block, lastBlock Block) (*transactionsPool.MerkleTree, error) {
	blockHeight := newBlock.GetHeader().Height
	if newBlock.GetBlockSupply() > common.MaxTotalSupply {
		return nil, fmt.Errorf("supply is too high")
	}

	if !bytes.Equal(lastBlock.BlockHash.GetBytes(), newBlock.GetHeader().PreviousHash.GetBytes()) {
		log.Println("lastBlock.BlockHash", lastBlock.BlockHash.GetHex(), newBlock.GetHeader().PreviousHash.GetHex())
		return nil, fmt.Errorf("last block hash not match to one stored in new block")
	}
	// needs to check block and process
	if newBlock.CheckProofOfSynergy() == false {
		return nil, fmt.Errorf("proof of synergy fails of block")
	}
	hash, err := newBlock.CalcBlockHash()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(hash.GetBytes(), newBlock.BlockHash.GetBytes()) {
		return nil, fmt.Errorf("wrong hash of block")
	}
	rootMerkleTrie := newBlock.GetHeader().RootMerkleTree
	txs := newBlock.TransactionsHashes
	txsBytes := make([][]byte, len(txs))
	for _, tx := range txs {
		hash := tx.GetBytes()
		txsBytes = append(txsBytes, hash)
	}
	merkleTrie, err := transactionsPool.BuildMerkleTree(blockHeight, txsBytes, transactionsPool.GlobalMerkleTree.DB)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(merkleTrie.GetRootHash(), rootMerkleTrie.GetBytes()) {
		return nil, fmt.Errorf("root merkleTrie hash check fails")
	}
	totalStaked := account.GetStakedInAllDelegatedAccounts()
	if !common.IsSyncing.Load() {

		if !oracles.VerifyPriceOracle(blockHeight, totalStaked, newBlock.BaseBlock.PriceOracle, newBlock.BaseBlock.PriceOracleData) {
			return nil, fmt.Errorf("price oracle check fails")
		}
		if !oracles.VerifyRandOracle(blockHeight, totalStaked, newBlock.BaseBlock.RandOracle, newBlock.BaseBlock.RandOracleData) {
			return nil, fmt.Errorf("rand oracle check fails")
		}
	}
	if len(newBlock.BaseBlock.BaseHeader.Encryption1[:]) == 0 || len(newBlock.BaseBlock.BaseHeader.Encryption2[:]) == 0 {
		return nil, fmt.Errorf("encryption opt data should be always present in block")
	}
	blockTime := newBlock.GetBlockTimeStamp()
	currTime := common.GetCurrentTimeStampInSecond()
	shouldCheck := !((currTime - blockTime) > int64(common.BlockTimeInterval)*common.VotingHeightDistance)
	if !common.IsSyncing.Load() && !bytes.Equal(newBlock.BaseBlock.BaseHeader.Encryption1[:], lastBlock.BaseBlock.BaseHeader.Encryption1[:]) {
		enc1, err := FromBytesToEncryptionConfig(newBlock.BaseBlock.BaseHeader.Encryption1[:], true)
		if err != nil {
			return nil, err
		}

		if enc1.SigName == common.SigName() && enc1.IsPaused == common.IsPaused() {
			//newBlock.BaseBlock.BaseHeader.Encryption1 = []byte{}
			log.Println("no need to change encryption, so leave encryption 1 empty")
		} else {
			if !oqs.VerifyEncConfig(enc1) {
				return nil, fmt.Errorf("encryption 1 verification fails")
			}
			if common.IsPaused() == false && common.SigName() != enc1.SigName {
				return nil, fmt.Errorf("you need to pause first to replace encryption, 1")
			}
			if enc1.IsPaused == true && common.IsPaused() == true && enc1.SigName == common.SigName() {
				return nil, fmt.Errorf("pausing fails, encryption is just puased, 1")
			}
			if enc1.SigName != common.SigName() && (enc1.IsPaused == true) {
				return nil, fmt.Errorf("new encryption has to be fully functional from beginning, so no paused, 1")
			}

			if shouldCheck && (enc1.SigName != common.SigName()) && !voting.VerifyEncryptionForReplacing(blockHeight, totalStaked, true) {
				return nil, fmt.Errorf("voting replacement encryption check fails, 1")
			}
			if shouldCheck && enc1.IsPaused == true && enc1.SigName == common.SigName() && !voting.VerifyEncryptionForPausing(blockHeight, totalStaked, true) {
				return nil, fmt.Errorf("voting pausing check fails, 1")
			}
			if enc1.SigName == common.SigName2() {
				return nil, fmt.Errorf("cannot exist 2 the same ecnryptions schemes, 1")
			}
		}
	}

	if !common.IsSyncing.Load() && !bytes.Equal(newBlock.BaseBlock.BaseHeader.Encryption2[:], lastBlock.BaseBlock.BaseHeader.Encryption2[:]) {
		enc2, err := FromBytesToEncryptionConfig(newBlock.BaseBlock.BaseHeader.Encryption2[:], false)
		if err != nil {
			return nil, err
		}
		if enc2.SigName == common.SigName2() && enc2.IsPaused == common.IsPaused2() {
			//newBlock.BaseBlock.BaseHeader.Encryption2 = []byte{}
			log.Println("no need to change encryption, so leave encryption 2 empty")
		} else {
			if !oqs.VerifyEncConfig(enc2) {
				return nil, fmt.Errorf("encryption 2 verification fails")
			}
			if common.IsPaused2() == false && common.SigName2() != enc2.SigName {
				return nil, fmt.Errorf("you need to pause first to replace encryption, 2")
			}
			if enc2.IsPaused == true && common.IsPaused2() == true && enc2.SigName == common.SigName2() {
				return nil, fmt.Errorf("pausing fails, encryption is just puased, 2")
			}
			if enc2.SigName != common.SigName2() && (enc2.IsPaused == true) {
				return nil, fmt.Errorf("new encryption has to be fully functional from beginning, so no paused, 2")
			}
			if shouldCheck && (enc2.SigName != common.SigName2()) && !voting.VerifyEncryptionForReplacing(blockHeight, totalStaked, false) {
				return nil, fmt.Errorf("voting replacement encryption check fails, 2")
			}
			if shouldCheck && enc2.IsPaused == true && enc2.SigName == common.SigName2() && !voting.VerifyEncryptionForPausing(blockHeight, totalStaked, false) {
				return nil, fmt.Errorf("voting pausing check fails, 2")
			}
			if enc2.SigName == common.SigName() {
				return nil, fmt.Errorf("cannot exist 2 the same ecnryptions schemes, 2")
			}
		}
	}
	return merkleTrie, nil
}

func IsAllTransactions(block Block) [][]byte {
	txs := block.TransactionsHashes
	hashes := [][]byte{}
	for _, tx := range txs {
		hash := tx.GetBytes()
		isKey := transactionsDefinition.CheckFromDBPoolTx(common.TransactionPoolHashesDBPrefix[:], hash)
		if isKey == false {
			hashes = append(hashes, hash)
		}
	}
	return hashes
}

func CheckBlockTransfers(block Block, lastBlock Block) (int64, int64, error) {
	txs := block.TransactionsHashes
	lastSupply := lastBlock.GetBlockSupply()
	accounts := map[[common.AddressLength]byte]account.Account{}
	stakingAccounts := map[[common.AddressLength]byte]account.StakingAccount{}
	totalFee := int64(0)
	for _, tx := range txs {
		hash := tx.GetBytes()
		poolTx, err := transactionsDefinition.LoadFromDBPoolTx(common.TransactionPoolHashesDBPrefix[:], hash)
		if err != nil {
			transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionPoolHashesDBPrefix[:], hash)
			if common.IsSyncing.Load() {
				poolTx, err = transactionsDefinition.LoadFromDBPoolTx(common.TransactionDBPrefix[:], hash)
				if err != nil {
					return 0, 0, err
				}
				//TODO
				//err = transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionDBPrefix[:], hash)
				//if err != nil {
				//	return 0, 0, err
				//}
				err = poolTx.StoreToDBPoolTx(common.TransactionPoolHashesDBPrefix[:])
				if err != nil {
					return 0, 0, err
				}
			} else {
				return 0, 0, err
			}
		}
		err = transactionsPool.CheckTransactionInDBAndInMarkleTrie(hash)
		if err != nil {
			return 0, 0, err
		}

		fee := poolTx.GasPrice * poolTx.GasUsage
		totalFee += fee
		amount := poolTx.TxData.Amount
		total_amount := fee + amount
		address := poolTx.GetSenderAddress()
		recipientAddress := poolTx.TxData.Recipient
		var n int
		if poolTx.GetLockedAmount() > 0 {
			n, err = account.IntDelegatedAccountFromAddress(poolTx.TxData.DelegatedAccountForLocking)
			if n <= 0 || n >= 256 || err != nil {
				return 0, 0, fmt.Errorf("DelegatedAccountForLocking must be a delegated account less than 256: CheckBlockTransfers")
			}
		} else {
			n, err = account.IntDelegatedAccountFromAddress(recipientAddress)
		}
		if err == nil && n < 512 { // delegated account
			stakingAcc := account.GetStakingAccountByAddressBytes(address.GetBytes(), n%256)
			if !bytes.Equal(stakingAcc.Address[:], address.GetBytes()) {

				log.Println("no account found in check block transfer: CheckBlockTransfers")
				copy(stakingAcc.Address[:], address.GetBytes())
				copy(stakingAcc.DelegatedAccount[:], recipientAddress.GetBytes())

			}
			if _, ok := stakingAccounts[stakingAcc.Address]; ok {
				stakingAcc = stakingAccounts[stakingAcc.Address]
			}
			stakingAcc.StakedBalance += amount
			stakingAcc.StakingRewards += fee // just using for fee in the local copy
			stakingAccounts[stakingAcc.Address] = stakingAcc
			ret := CheckStakingTransaction(poolTx, stakingAccounts[stakingAcc.Address].StakedBalance, stakingAccounts[stakingAcc.Address].StakingRewards)
			if ret == false {
				// remove bad transaction from pool
				transactionsPool.RemoveBadTransactionByHash(poolTx.Hash.GetBytes(), block.GetHeader().Height)
				return 0, 0, fmt.Errorf("staking transactions checking fails: CheckBlockTransfers")
			}
		}
		acc := account.GetAccountByAddressBytes(address.GetBytes())
		if !bytes.Equal(acc.Address[:], address.GetBytes()) {
			// remove bad transaction from pool
			transactionsPool.RemoveBadTransactionByHash(poolTx.Hash.GetBytes(), block.GetHeader().Height)
			return 0, 0, fmt.Errorf("no account found in check block transafer: CheckBlockTransfers")
		}
		if bytes.Equal(poolTx.TxParam.MultiSignTx.GetBytes(), ZerosHash) == false && (poolTx.TxData.Amount > 0 || len(poolTx.TxData.OptData) > 0 || poolTx.TxData.LockedAmount > 0 || poolTx.TxData.MultiSignNumber > 0) {
			transactionsPool.RemoveBadTransactionByHash(poolTx.Hash.GetBytes(), block.GetHeader().Height)
			return 0, 0, fmt.Errorf("transaction which confirms in multi signature account should have amount == 0, OptData = nil, LockedAmount = 0, MultiSignNumber = 0")
		}

		if _, ok := accounts[acc.Address]; ok {
			acc = accounts[acc.Address]
			acc.Balance -= total_amount
			accounts[acc.Address] = acc
		} else {
			acc.Balance -= total_amount
			accounts[acc.Address] = acc
		}
		if acc.Balance < 0 {
			// remove bad transaction from pool
			transactionsPool.RemoveBadTransactionByHash(poolTx.Hash.GetBytes(), block.GetHeader().Height)
			return 0, 0, fmt.Errorf("not enough funds on account: CheckBlockTransfers")
		}

	}
	reward := account.GetReward(lastSupply)
	lastSupply += reward
	if lastSupply != block.GetBlockSupply() {
		log.Println("lastSupply:", lastSupply, "block.GetBlockSupply()", block.GetBlockSupply())
		return 0, 0, fmt.Errorf("block supply checking fails: CheckBlockTransfers")
	}

	return reward, totalFee, nil
}

func ProcessBlockTransfers(block Block, reward int64) error {
	err := ProcessTransactionsEscrow(block.GetHeader().Height)
	if err != nil {
		log.Println("ProcessTransactionsEscrow: ", err)
	}

	txs := block.TransactionsHashes
	for _, tx := range txs {
		hash := tx.GetBytes()
		err := transactionsPool.CheckTransactionInDBAndInMarkleTrie(hash)
		if err != nil {
			return err
		}
		poolTx, err := transactionsDefinition.LoadFromDBPoolTx(common.TransactionPoolHashesDBPrefix[:], hash)
		if err != nil {
			return err
		}

		if poolTx.Height > block.GetHeader().Height {
			transactionsPool.RemoveBadTransactionByHash(poolTx.Hash.GetBytes(), block.GetHeader().Height)
			return fmt.Errorf("transaction height is wrong: ProcessBlockTransfers")
		}

		err = ProcessTransaction(poolTx, block.GetHeader().Height)
		if err != nil {
			// remove bad transaction from pool
			transactionsPool.RemoveBadTransactionByHash(poolTx.Hash.GetBytes(), block.GetHeader().Height)
			return err
		}
		err = ProcessTransactionsMultiSign(poolTx, block.GetHeader().Height)
		if err != nil {
			// remove bad transaction from pool
			transactionsPool.RemoveBadTransactionByHash(poolTx.Hash.GetBytes(), block.GetHeader().Height)
			return err
		}
	}
	addr := block.BaseBlock.BaseHeader.OperatorAccount.ByteValue
	n, err := account.IntDelegatedAccountFromAddress(block.BaseBlock.BaseHeader.DelegatedAccount)
	if err != nil || n < 1 || n > 255 {
		return fmt.Errorf("wrong delegated account in block: ProcessBlockTransfers")
	}
	staked, sum, _ := account.GetStakedInDelegatedAccount(n)
	if sum <= 0 {
		return fmt.Errorf("no staked amount in delegated account which was rewarded: ProcessBlockTransfers")
	}

	rewardPerc := block.GetRewardPercentage()
	if rewardPerc > 500 {
		return fmt.Errorf("reward has to be smaller than 50")
	}
	rewardOper := int64(float64(reward) * float64(rewardPerc) / 1000.0)

	err = account.Reward(addr[:], rewardOper, block.GetHeader().Height, n)
	if err != nil {
		return err
	}

	reward -= rewardOper
	rest := reward
	for _, acc := range staked {
		if acc.Balance > 0 {
			userReward := int64(float64(reward) * float64(acc.Balance) / sum)
			rest -= userReward // in the case when rounding lose some fraction of coins
			err := account.Reward(acc.Address[:], userReward, block.GetHeader().Height, n)
			if err != nil {
				return err
			}
		}
	}
	if rest > 0 {
		err := account.Reward(addr[:], rest, block.GetHeader().Height, n)
		if err != nil {
			return err
		}
	} else if rest < 0 {
		return fmt.Errorf("this shouldn't happen anytime: ProcessBlockTransfers")
	}

	return nil
}

func RemoveAllTransactionsRelatedToBlock(newBlock Block) {
	txs := newBlock.TransactionsHashes
	for _, tx := range txs {
		hash := tx.GetBytes()
		transactionsPool.PoolsTx.RemoveTransactionByHash(hash)
		transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionPoolHashesDBPrefix[:], hash)
	}
}

func EvaluateSmartContracts(bl *Block) bool {
	height := (*bl).GetHeader().Height
	if ok, logs, addresses, codes, _ := EvaluateSCForBlock(*bl); ok {
		StateMutex.Lock()
		State.SetSnapShotNum(height, State.Snapshot())
		StateMutex.Unlock()
		for th, a := range addresses {

			prefix := common.OutputLogsHashesDBPrefix[:]
			err := database.MainDB.Put(append(prefix, th[:]...), []byte(logs[th]))
			if err != nil {
				log.Println("Cannot store output logs")
				return false
			}

			aa := [common.AddressLength]byte{}
			copy(aa[:], a.GetBytes())
			prefix = common.OutputAddressesHashesDBPrefix[:]
			err = database.MainDB.Put(append(prefix, th[:]...), codes[aa])
			if err != nil {
				log.Println("Cannot store address codes")
				return false
			}
		}

	} else {
		log.Println("Evaluating Smart Contract fails")
		return false
	}
	return true
}

func CheckBlockAndTransferFunds(newBlock *Block, lastBlock Block, merkleTrie *transactionsPool.MerkleTree) error {

	defer RemoveAllTransactionsRelatedToBlock(*newBlock)
	n, err := account.IntDelegatedAccountFromAddress(newBlock.GetHeader().DelegatedAccount)
	if err != nil || n < 1 || n > 255 {
		return fmt.Errorf("wrong delegated account: CheckBlockAndTransferFunds")
	}
	opAccBlockAddr := newBlock.GetHeader().OperatorAccount
	if _, sumStaked, opAcc := account.GetStakedInDelegatedAccount(n); int64(sumStaked) < common.MinStakingForNode || !bytes.Equal(opAcc.Address[:], opAccBlockAddr.GetBytes()) {
		return fmt.Errorf("not enough staked coins to be a node or not valid operetional account: CheckBlockAndTransferFunds")
	}

	reward, totalFee, err := CheckBlockTransfers(*newBlock, lastBlock)
	if err != nil {
		return err
	}
	newBlock.BlockFee = totalFee + lastBlock.BlockFee

	if EvaluateSmartContracts(newBlock) == false {
		return fmt.Errorf("evaluation of smart contracts in block fails: CheckBlockAndTransferFunds")
	}

	staked, rewarded := GetSupplyInStakedAccounts()
	//coinsInDex := account.GetCoinLiquidityInDex()
	if GetSupplyInAccounts()+staked+rewarded+reward+lastBlock.BlockFee != newBlock.GetBlockSupply() {
		log.Println("GetSupplyInAccounts()", GetSupplyInAccounts())
		log.Println("staked:", staked)
		log.Println("rewarded", rewarded)
		log.Println("lastBlock.BlockFee", lastBlock.BlockFee)
		log.Println("GetSupplyInAccounts()+staked+rewarded+reward+lastBlock.BlockFee:", GetSupplyInAccounts()+staked+rewarded+reward+lastBlock.BlockFee, "newBlock.GetBlockSupply():", newBlock.GetBlockSupply())
		return fmt.Errorf("block supply checking fails vs account balances: CheckBlockAndTransferFunds")
	}
	hashes := newBlock.GetBlockTransactionsHashes()
	log.Println("Number of transactions in block: ", len(hashes))
	err = ProcessBlockPubKey(*newBlock)
	if err != nil {
		return err
	}
	head := newBlock.GetHeader()

	if head.Verify() == false {
		return fmt.Errorf("header fails to verify: CheckBlockAndTransferFunds")
	}

	err = merkleTrie.StoreTree(newBlock.GetHeader().Height)
	if err != nil {
		return err
	}
	err = ProcessBlockTransfers(*newBlock, reward)
	if err != nil {
		return err
	}
	for _, h := range hashes {
		tx, err := transactionsDefinition.LoadFromDBPoolTx(common.TransactionPoolHashesDBPrefix[:], h.GetBytes())
		if err != nil {
			log.Println(err)
			continue
		}
		err = tx.StoreToDBPoolTx(common.TransactionDBPrefix[:])
		if err != nil {
			return err
		}
		transactionsPool.PoolsTx.RemoveTransactionByHash(h.GetBytes())
		err = tx.RemoveFromDBPoolTx(common.TransactionPoolHashesDBPrefix[:])
		if err != nil {
			log.Println(err)
		}
	}
	err = ProcessBlockEncryption(*newBlock, lastBlock)
	if err != nil {
		log.Println("process block encryption fails", err)
	}
	return nil
}
