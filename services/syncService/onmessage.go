package syncServices

import (
	"bytes"
	"log"
	"runtime/debug"

	"github.com/okuralabs/okura-node/account"
	"github.com/okuralabs/okura-node/blocks"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/message"
	"github.com/okuralabs/okura-node/services"
	nonceServices "github.com/okuralabs/okura-node/services/nonceService"
	"github.com/okuralabs/okura-node/services/transactionServices"
	"github.com/okuralabs/okura-node/statistics"
	"github.com/okuralabs/okura-node/tcpip"
	"github.com/okuralabs/okura-node/transactionsPool"
)

func OnMessage(addr [4]byte, m []byte) {

	h := common.GetHeight()
	if tcpip.IsIPBanned(addr, h, tcpip.SyncTopic) {
		return
	}
	//log.Println("New message nonce from:", addr)
	msg := message.TransactionsMessage{}
	//common.BlockMutex.Lock()
	//defer common.BlockMutex.Unlock()
	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			log.Println("recover (sync Msg)", r)
		}

	}()

	amsg, err := msg.GetFromBytes(m)
	if err != nil {
		panic(err)
	}

	isValid := message.CheckMessage(amsg)
	if isValid == false {
		log.Println("message is invalid")
		panic("message is invalid")
	}

	switch string(amsg.GetHead()) {
	case "hi": // getheader

		txn := amsg.(message.TransactionsMessage).GetTransactionsBytes()
		var topicip [6]byte
		var ip4 [4]byte
		if tcpip.GetPeersCount() < common.MaxPeersConnected {
			peers := txn[[2]byte{'P', 'P'}]
			peersConnectedNN := tcpip.GetPeersConnected(tcpip.NonceTopic)
			peersConnectedBB := tcpip.GetPeersConnected(tcpip.SyncTopic)
			peersConnectedTT := tcpip.GetPeersConnected(tcpip.TransactionTopic)

			for _, ip := range peers {
				copy(ip4[:], ip)
				copy(topicip[2:], ip)
				copy(topicip[:2], tcpip.NonceTopic[:])
				if bytes.Equal(ip4[:], addr[:]) {
					continue
				}
				if _, ok := peersConnectedNN[topicip]; !ok && !tcpip.IsIPBanned(ip4, h, tcpip.NonceTopic) {
					go nonceServices.StartSubscribingNonceMsg(ip4)
				}
				copy(topicip[:2], tcpip.SyncTopic[:])
				if _, ok := peersConnectedBB[topicip]; !ok && !tcpip.IsIPBanned(ip4, h, tcpip.SyncTopic) {
					go StartSubscribingSyncMsg(ip4)
				}
				copy(topicip[:2], tcpip.TransactionTopic[:])
				if _, ok := peersConnectedTT[topicip]; !ok && !tcpip.IsIPBanned(ip4, h, tcpip.TransactionTopic) {
					go transactionServices.StartSubscribingTransactionMsg(ip4)
				}
				if tcpip.GetPeersCount() > common.MaxPeersConnected {
					break
				}
			}
		}
		if h < 10 {
			common.IsSyncing.Store(true)
		}
		lastOtherHeight := common.GetInt64FromByte(txn[[2]byte{'L', 'H'}][0])
		common.SetHeightMax(lastOtherHeight)
		lastOtherBlockHashBytes := txn[[2]byte{'L', 'B'}][0]
		if lastOtherHeight == h {
			services.AdjustShiftInPastInReset(lastOtherHeight)
			lastBlockHashBytes, err := blocks.LoadHashOfBlock(h)
			if err != nil {
				panic(err)
			}
			if !bytes.Equal(lastOtherBlockHashBytes, lastBlockHashBytes) {
				SendGetHeaders(addr, lastOtherHeight)
			}
			common.IsSyncing.Store(false)
			return
		} else if lastOtherHeight < h {
			services.AdjustShiftInPastInReset(lastOtherHeight)
			common.IsSyncing.Store(false)
			return
		}
		// when others have longer chain
		SendGetHeaders(addr, lastOtherHeight)
		return
	case "sh":

		txn := amsg.(message.TransactionsMessage).GetTransactionsBytes()
		blcks := []blocks.Block{}
		indices := []int64{}
		for k, tx := range txn {
			for _, t := range tx {
				if k == [2]byte{'I', 'H'} {
					index := common.GetInt64FromByte(t)
					indices = append(indices, index)
				} else if k == [2]byte{'H', 'V'} {
					block := blocks.Block{
						BaseBlock:          blocks.BaseBlock{},
						TransactionsHashes: nil,
						BlockHash:          common.Hash{},
					}
					block, err := block.GetFromBytes(t)
					if err != nil {
						panic("cannot unmarshal header")
					}
					blcks = append(blcks, block)
				}
			}
		}
		hmax := common.GetHeightMax()
		if indices[len(indices)-1] <= h {
			log.Println("shorter other chain")
			return
		}
		if indices[0] > h {
			log.Println("too far blocks of other")
			return
		}
		// check blocks
		was := false
		incompleteTxn := false
		hashesMissingAll := [][]byte{}
		lastGoodBlock := indices[0]
		merkleTries := map[int64]*transactionsPool.MerkleTree{}
		log.Printf("Starting block verification for %d blocks", len(blcks))
		for i := 0; i < len(blcks); i++ {
			header := blcks[i].GetHeader()
			index := indices[i]
			log.Printf("Processing block %d/%d - Height: %d, Index: %d", i+1, len(blcks), header.Height, index)

			if index <= 0 {
				log.Printf("Skipping block with invalid index: %d", index)
				continue
			}
			block := blcks[i]
			oldBlock := blocks.Block{}
			if index <= h {
				hashOfMyBlockBytes, err := blocks.LoadHashOfBlock(index)
				if err != nil {
					log.Printf("ERROR: Failed to load block hash for index %d: %v", index, err)
					panic("cannot load block hash")
				}
				if bytes.Equal(block.BlockHash.GetBytes(), hashOfMyBlockBytes) {
					log.Printf("Block %d matches existing block, marking as lastGoodBlock", index)
					lastGoodBlock = index
					continue
				}
				log.Printf("Block hash mismatch at index %d - potential fork detected", index)
				defer services.AdjustShiftInPastInReset(hmax)
				common.ShiftToPastMutex.RLock()
				defer common.ShiftToPastMutex.RUnlock()
				services.ResetAccountsAndBlocksSync(index - common.ShiftToPastInReset)
				panic("potential fork detected")
			}
			if was {
				oldBlock = blcks[i-1]
				log.Printf("Using previous block from received blocks for index %d", index)
			} else {
				oldBlock, err = blocks.LoadBlock(index - 1)
				if err != nil {
					log.Printf("ERROR: Failed to load previous block for index %d: %v", index-1, err)
					panic("cannot load block")
				}
				was = true
				log.Printf("Loaded previous block from storage for index %d", index)
			}

			// Special logging for second block
			if index == 1 {
				log.Printf("=== Processing second block in sync service ===")
				log.Printf("Current height: %d", h)
				log.Printf("Second block hash: %x", block.BlockHash.GetBytes())
				log.Printf("Second block previous hash: %x", block.GetHeader().PreviousHash.GetBytes())
				log.Printf("Genesis block hash: %x", oldBlock.BlockHash.GetBytes())
				log.Printf("Is initial sync: %v", h == 0)
				log.Printf("Block verification path: %s", "sync")
				log.Printf("Block source: %s", func() string {
					if was {
						return "from received blocks"
					}
					return "from storage"
				}())

				// Check if block exists in storage
				storedBlock, err := blocks.LoadBlock(1)
				if err == nil {
					log.Printf("Second block already in storage - Hash: %x", storedBlock.BlockHash.GetBytes())
					log.Printf("Second block in storage previous hash: %x", storedBlock.GetHeader().PreviousHash.GetBytes())
					if !bytes.Equal(storedBlock.BlockHash.GetBytes(), block.BlockHash.GetBytes()) {
						log.Printf("WARNING: Second block hash mismatch between received and stored")
						log.Printf("Stored hash: %x", storedBlock.BlockHash.GetBytes())
						log.Printf("Received hash: %x", block.BlockHash.GetBytes())
					}
				} else {
					log.Printf("No second block found in storage")
				}
			}

			// Add detailed logging for block hash verification
			log.Printf("block %d hash: %x", index, block.BlockHash.GetBytes())
			log.Printf("Verifying block %d previous hash: %x", index, block.GetHeader().PreviousHash.GetBytes())
			log.Printf("Previous block %d hash: %x", index-1, oldBlock.BlockHash.GetBytes())
			log.Printf("Previous block %d previous hash: %x", index-1, oldBlock.GetHeader().PreviousHash.GetBytes())
			if !bytes.Equal(block.GetHeader().PreviousHash.GetBytes(), oldBlock.BlockHash.GetBytes()) {
				log.Printf("ERROR: Block %d previous hash mismatch - Expected: %x, Got: %x",
					index,
					oldBlock.BlockHash.GetBytes(),
					block.GetHeader().PreviousHash.GetBytes())
			}

			if header.Height != index {
				log.Printf("ERROR: Height mismatch - Block header height: %d, Expected index: %d", header.Height, index)
				defer services.AdjustShiftInPastInReset(hmax)
				common.ShiftToPastMutex.RLock()
				defer common.ShiftToPastMutex.RUnlock()
				services.ResetAccountsAndBlocksSync(index - common.ShiftToPastInReset)
				panic("not relevant height vs index")
			}

			log.Printf("Performing base block verification for block %d", index)
			merkleTrie, err := blocks.CheckBaseBlock(block, oldBlock)
			defer merkleTrie.Destroy()
			if err != nil {
				log.Printf("ERROR: Base block verification failed for block %d: %v", index, err)
				services.AdjustShiftInPastInReset(hmax)
				common.ShiftToPastMutex.RLock()
				defer common.ShiftToPastMutex.RUnlock()
				services.ResetAccountsAndBlocksSync(index - common.ShiftToPastInReset)
				panic(err)
			}
			merkleTries[index] = merkleTrie
			hashesMissing := blocks.IsAllTransactions(block)
			if len(hashesMissing) > 0 {
				log.Printf("Block %d is missing %d transactions", index, len(hashesMissing))
				hashesMissingAll = append(hashesMissingAll, hashesMissing...)
				incompleteTxn = true
			} else {
				log.Printf("Block %d has all transactions verified", index)
			}
			//common.IsSyncing.Store(true)
		}

		if incompleteTxn {
			log.Printf("Sync incomplete - requesting %d missing transactions from peer", len(hashesMissingAll))
			transactionServices.SendGT(addr, hashesMissingAll, "bt")
			log.Println("Incomplete transactions stored in DB")
			return
		}
		log.Println("Starting final block processing and fund transfers")
		common.IsSyncing.Store(true)
		common.BlockMutex.Lock()
		defer common.BlockMutex.Unlock()
		was = false
		for i := 0; i < len(blcks); i++ {
			block := blcks[i]
			index := indices[i]
			if block.GetHeader().Height <= lastGoodBlock {
				log.Printf("Skipping already verified block %d", index)
				continue
			}

			log.Printf("Processing final verification and fund transfer for block %d", index)
			oldBlock := blocks.Block{}
			if was == true {
				oldBlock = blcks[i-1]
			} else {
				oldBlock, err = blocks.LoadBlock(index - 1)
				if err != nil {
					log.Printf("ERROR: Failed to load previous block for index %d: %v", index-1, err)
					panic("cannot load block")
				}
				was = true
			}

			err := blocks.CheckBlockAndTransferFunds(&block, oldBlock, merkleTries[index])
			if err != nil {
				log.Printf("ERROR: Fund transfer failed for block %d: %v", index, err)
				hashesMissing := blocks.IsAllTransactions(block)
				if len(hashesMissing) > 0 {
					log.Printf("Detected %d missing transactions during fund transfer", len(hashesMissing))
					transactionServices.SendGT(addr, hashesMissing, "bt")
				}
				services.ResetAccountsAndBlocksSync(oldBlock.GetHeader().Height)
				return
			}

			log.Printf("Storing block %d", index)
			err = block.StoreBlock()
			if err != nil {
				log.Printf("ERROR: Failed to store block %d: %v", index, err)
				services.ResetAccountsAndBlocksSync(oldBlock.GetHeader().Height)
				return
			}

			log.Println("Sync New Block success -------------------------------------", block.GetHeader().Height)
			err = account.StoreAccounts(block.GetHeader().Height)
			if err != nil {
				log.Println(err)
			}

			err = account.StoreStakingAccounts(block.GetHeader().Height)
			if err != nil {
				log.Println(err)
			}
			common.SetHeight(block.GetHeader().Height)

			sm := statistics.GetStatsManager()
			sm.UpdateStatistics(block, oldBlock)

		}

	case "gh":

		txn := amsg.(message.TransactionsMessage).GetTransactionsBytes()

		bHeight := common.GetInt64FromByte(txn[[2]byte{'B', 'H'}][0])
		eHeight := common.GetInt64FromByte(txn[[2]byte{'E', 'H'}][0])
		SendHeaders(addr, bHeight, eHeight)
	default:
	}
}
