package transactionServices

import (
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/message"
	"github.com/okuralabs/okura-node/tcpip"
	"github.com/okuralabs/okura-node/transactionsDefinition"
	"github.com/okuralabs/okura-node/transactionsPool"
	"log"
)

func OnMessage(addr [4]byte, m []byte) {

	if tcpip.IsIPBanned(addr) {
		return
	}
	//log.Println("New message nonce from:", addr)

	defer func() {
		if r := recover(); r != nil {
			//debug.PrintStack()
			log.Println("recover (nonce Msg)", r)
		}

	}()

	isValid, amsg := message.CheckValidMessage(m)
	if isValid == false {
		tcpip.BanIP(addr)
		return
	}

	switch string(amsg.GetHead()) {
	case "tx":
		msg := amsg.(message.TransactionsMessage)
		txn, err := msg.GetTransactionsFromBytes()
		if err != nil {
			return
		}
		if transactionsPool.PoolsTx.NumberOfTransactions() > common.MaxTransactionInPool {
			//log.Println("no more transactions can be accepted to the pool")
			return
		}
		// need to check transactions
		for _, v := range txn {
			for _, t := range v {
				//if t.Verify() {
				if transactionsPool.PoolsTx.TransactionExists(t.Hash.GetBytes()) {
					//log.Println("transaction just exists in Pool")
					continue
				}
				if transactionsDefinition.CheckFromDBPoolTx(common.TransactionDBPrefix[:], t.Hash.GetBytes()) {
					//log.Println("transaction just exists in DB")
					continue
				}

				isAdded := transactionsPool.PoolsTx.AddTransaction(t, t.Hash)
				if isAdded {
					err := t.StoreToDBPoolTx(common.TransactionPoolHashesDBPrefix[:])
					if err != nil {
						transactionsPool.PoolsTx.RemoveTransactionByHash(t.Hash.GetBytes())
						err := transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionPoolHashesDBPrefix[:], t.Hash.GetBytes())
						if err != nil {
							log.Println(err)
						}
						//err = transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionDBPrefix[:], t.Hash.GetBytes())
						//if err != nil {
						//	log.Println(err)
						//}
						log.Println(err)
						continue
					}
					//maybe we should not spread automatically transactions. Third party should care about it
					Spread(addr, m)
				}
			}
		}

	case "st":
		txn := amsg.(message.TransactionsMessage).GetTransactionsBytes()
		for topic, v := range txn {
			txs := []transactionsDefinition.Transaction{}
			for _, hs := range v {
				t, err := transactionsDefinition.LoadFromDBPoolTx(common.TransactionPoolHashesDBPrefix[:], hs)
				if err != nil {
					log.Println("cannot load transaction from pool", err)

					transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionPoolHashesDBPrefix[:], hs)
					continue

					//t, err = transactionsDefinition.LoadFromDBPoolTx(common.TransactionDBPrefix[:], hs)
					//if err != nil {
					//	transactionsPool.PoolsTx.RemoveTransactionByHash(hs)
					//	transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionDBPrefix[:], hs)
					//	//log.Println("cannot load transaction", err)
					//	continue
					//}
				}
				if len(t.GetBytes()) > 0 {
					txs = append(txs, t)
				}
			}
			transactionMsg, err := GenerateTransactionMsg(txs, []byte("tx"), topic)
			if err != nil {
				log.Println("cannot generate transaction msg", err)
			}
			Send(addr, transactionMsg.GetBytes())
		}
	case "bt":
		txn := amsg.(message.TransactionsMessage).GetTransactionsBytes()
		for topic, v := range txn {
			txs := []transactionsDefinition.Transaction{}
			for _, hs := range v {
				t, err := transactionsDefinition.LoadFromDBPoolTx(common.TransactionDBPrefix[:], hs)
				if err != nil {
					log.Println("cannot load transaction from DB", err)
					continue
					//transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionPoolHashesDBPrefix[:], hs)

					//t, err = transactionsDefinition.LoadFromDBPoolTx(common.TransactionDBPrefix[:], hs)
					//if err != nil {
					//	transactionsPool.PoolsTx.RemoveTransactionByHash(hs)
					//	transactionsDefinition.RemoveTransactionFromDBbyHash(common.TransactionDBPrefix[:], hs)
					//log.Println("cannot load transaction", err)
					//continue
					//}
				}
				if len(t.GetBytes()) > 0 {
					txs = append(txs, t)
				}
			}
			transactionMsg, err := GenerateTransactionMsg(txs, []byte("tx"), topic)
			if err != nil {
				log.Println("cannot generate transaction msg", err)
			}
			Send(addr, transactionMsg.GetBytes())
		}
	default:
	}
}
