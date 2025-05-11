package services

import (
	"github.com/okuralabs/okura-node/account"
	"github.com/okuralabs/okura-node/blocks"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/logger"
	"github.com/okuralabs/okura-node/transactionsPool"
)

func AdjustShiftInPastInReset(height int64) {
	common.ShiftToPastMutex.Lock()
	defer common.ShiftToPastMutex.Unlock()
	h := common.GetHeight()
	if height-h <= 0 {
		common.ShiftToPastInReset = 1
		return
	}
	common.ShiftToPastInReset *= 2
	if common.ShiftToPastInReset > height {
		common.ShiftToPastInReset = height
	}
	if common.ShiftToPastInReset < 1 {
		common.ShiftToPastInReset = 1
	}
}

func RevertVMToBlockHeight(height int64) bool {
	blocks.StateMutex.Lock()
	defer blocks.StateMutex.Unlock()
	lastNum := 0
	for h, n := range blocks.State.HeightToSnapShotNum {
		if h > height {
			continue
		}
		if n > lastNum {
			lastNum = n
		}
	}

	blocks.State.RevertToSnapshot(lastNum)
	return true
}

func ResetAccountsAndBlocksSync(height int64) {
	if height < 0 {
		logger.GetLogger().Println("try to reset from negative height")
		height = 0
		h := common.GetHeight()
		if h == 0 {
			common.IsSyncing.Store(true)
			return
		}
	}

	err := account.LoadAccounts(height)
	if err != nil {
		return
	}
	err = account.LoadStakingAccounts(height)
	if err != nil {
		return
	}

	ha, err := account.LastHeightStoredInAccounts()
	if err != nil {
		logger.GetLogger().Println(err)
	}
	hsa, err := account.LastHeightStoredInStakingAccounts()
	if err != nil {
		logger.GetLogger().Println(err)
	}
	hb, err := blocks.LastHeightStoredInBlocks()
	if err != nil {
		logger.GetLogger().Println(err)
	}
	err = blocks.SetEncryptionFromBlock(height)
	if err != nil {
		return
	}
	hd, err := account.LastHeightStoredInDexAccounts()
	if err != nil {
		logger.GetLogger().Println(err)
	}

	if RevertVMToBlockHeight(height) == false {
		logger.GetLogger().Println("reverting VM to height ", height, " fails.")
	}

	for i := hb; i > height; i-- {
		err := blocks.RemoveBlockFromDB(i)
		if err != nil {
			logger.GetLogger().Println(err)
		}
	}
	for i := ha; i > height; i-- {
		err := account.RemoveAccountsFromDB(i)
		if err != nil {
			logger.GetLogger().Println(err)
		}
	}
	for i := hsa; i > height; i-- {
		err := account.RemoveStakingAccountsFromDB(i)
		if err != nil {
			logger.GetLogger().Println(err)
		}
	}
	for i := hd; i > height; i-- {
		err := account.RemoveDexAccountsFromDB(i)
		if err != nil {
			logger.GetLogger().Println(err)
		}
	}

	hm, err := transactionsPool.LastHeightStoredInMerleTrie()
	if err != nil {
		logger.GetLogger().Println(err)
	}
	for i := hm; i > height; i-- {
		err := transactionsPool.RemoveMerkleTrieFromDB(i)
		if err != nil {
			logger.GetLogger().Println(err)
		}
	}

	common.SetHeight(height)

	common.IsSyncing.Store(true)
}
