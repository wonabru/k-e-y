package main

import (
	"bytes"
	"fmt"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/crypto/oqs/rand"
	clientrpc "github.com/okuralabs/okura-node/rpc/client"
	"github.com/okuralabs/okura-node/services/transactionServices"
	"github.com/okuralabs/okura-node/statistics"
	"github.com/okuralabs/okura-node/transactionsDefinition"
	"github.com/okuralabs/okura-node/wallet"
	rand2 "math/rand"
	"sync"

	"log"
	"os"
	"time"
)

var mutex sync.Mutex
var MainWallet *wallet.Wallet

func main() {
	var ip string
	if len(os.Args) > 1 {
		ip = os.Args[1]
	} else {
		ip = "127.0.0.1"
	}
	go clientrpc.ConnectRPC(ip)
	wallet.InitActiveWallet(0, "a")
	MainWallet = wallet.GetActiveWallet()

	for range 10 {
		go sendTransactions(MainWallet)
		//time.Sleep(time.Millisecond * 1)
	}
	chanPeer := make(chan []byte)
	<-chanPeer
}

func SignMessage(line []byte) []byte {

	operation := string(line[0:4])
	verificationNeeded := true
	for _, noVerification := range common.ConnectionsWithoutVerification {
		if bytes.Equal([]byte(operation), noVerification) {
			verificationNeeded = false
			break
		}
	}
	if verificationNeeded {
		if MainWallet == nil || (!MainWallet.Check() || !MainWallet.Check2()) {
			log.Println("wallet not loaded yet")
			return line
		}
		if common.IsPaused() == false {
			// primary encryption used
			line = common.BytesToLenAndBytes(line)
			sign, err := MainWallet.Sign(line, true)
			if err != nil {
				log.Println(err)
				return line
			}
			line = append(line, sign.GetBytes()...)

		} else {
			// secondary encryption
			line = common.BytesToLenAndBytes(line)
			sign, err := MainWallet.Sign(line, false)
			if err != nil {
				log.Println(err)
				return line
			}
			line = append(line, sign.GetBytes()...)
		}
	} else {
		line = common.BytesToLenAndBytes(line)
	}
	return line
}

func SampleTransaction(w *wallet.Wallet) transactionsDefinition.Transaction {
	mutex.Lock()
	defer mutex.Unlock()
	sender := w.MainAddress
	recv := common.Address{}
	br := rand.RandomBytes(20)
	err := recv.Init(append([]byte{0}, br...))
	if err != nil {
		return transactionsDefinition.Transaction{}
	}
	amount := int64(rand2.Intn(1000000000))
	txdata := transactionsDefinition.TxData{
		Recipient: recv,
		Amount:    amount,
		OptData:   nil,
		Pubkey:    common.PubKey{}, //w.PublicKey, //
	}
	txParam := transactionsDefinition.TxParam{
		ChainID:     common.GetChainID(),
		Sender:      sender,
		SendingTime: common.GetCurrentTimeStampInSecond(),
		Nonce:       int16(rand2.Intn(65000)),
	}
	t := transactionsDefinition.Transaction{
		TxData:    txdata,
		TxParam:   txParam,
		Hash:      common.Hash{},
		Signature: common.Signature{},
		Height:    0,
		GasPrice:  0,
		GasUsage:  0,
	}

	clientrpc.InRPC <- SignMessage([]byte("STAT"))
	var reply []byte
	reply = <-clientrpc.OutRPC
	st := statistics.Stats{}
	err = common.Unmarshal(reply, common.StatDBPrefix, &st)
	if err != nil {
		return transactionsDefinition.Transaction{}
	}
	t.Height = st.Height

	err = t.CalcHashAndSet()
	if err != nil {
		log.Println("calc hash error", err)
	}
	err = t.Sign(w, w.PublicKey.Primary)
	if err != nil {
		log.Println("Signing error", err)
	}
	//s := rand.RandomBytes(common.SignatureLength)
	//sig := common.Signature{}
	//err = sig.Init(s, w.Address)
	//if err != nil {
	//	return transactionsDefinition.Transaction{}
	//}
	//t.Signature = sig
	return t
}

func sendTransactions(w *wallet.Wallet) {

	batchSize := 1
	count := int64(0)
	start := common.GetCurrentTimeStampInSecond()
	for range time.Tick(time.Millisecond * 100) {
		var txs []transactionsDefinition.Transaction
		for i := 0; i < batchSize; i++ {
			tx := SampleTransaction(w)
			txs = append(txs, tx)
			end := common.GetCurrentTimeStampInSecond()
			count++
			if count%1 == 0 && (end-start) > 0 {
				fmt.Println("tps=", count/(end-start), " count: ", count)
			}
		}
		m, err := transactionServices.GenerateTransactionMsg(txs, []byte("tx"), [2]byte{'T', 'T'})
		if err != nil {
			return
		}
		tmm := m.GetBytes()
		//count += int64(batchSize)
		clientrpc.InRPC <- SignMessage(append([]byte("TRAN"), tmm...))
		//log.Printf("send batch %d transactions", batchSize)
		<-clientrpc.OutRPC
		//log.Println("transactions sent")
	}
}
