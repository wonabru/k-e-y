package serverrpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/okuralabs/okura-node/account"
	"github.com/okuralabs/okura-node/blocks"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/core/stateDB"
	"github.com/okuralabs/okura-node/crypto/oqs"
	"github.com/okuralabs/okura-node/pubkeys"
	nonceServices "github.com/okuralabs/okura-node/services/nonceService"
	"github.com/okuralabs/okura-node/services/transactionServices"
	"github.com/okuralabs/okura-node/statistics"
	"github.com/okuralabs/okura-node/tcpip"
	"github.com/okuralabs/okura-node/transactionsDefinition"
	"github.com/okuralabs/okura-node/transactionsPool"
	"github.com/okuralabs/okura-node/wallet"
	"log"
	"net"
	"net/rpc"
	"strconv"
	"sync"
)

var listenerMutex sync.Mutex
var activeWallet *wallet.Wallet

type Listener []byte

func ListenRPC() {
	var address = "0.0.0.0:" + strconv.Itoa(tcpip.Ports[tcpip.RPCTopic])
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Error resolving TCP address: %v", err)
	}
	defer listener.Close()
	err = rpc.Register(new(Listener))
	if err != nil {
		log.Fatalf("Error registering RPC listener: %v", err)
	}
	log.Printf("RPC server listening on %s", address)
	rpc.Accept(listener)
}

func (l *Listener) Send(lineBeg []byte, reply *[]byte) error {
	listenerMutex.Lock()
	defer listenerMutex.Unlock()
	if len(lineBeg) < 4 {
		*reply = []byte("Error with message. Too small length calling server")
		return nil
	}
	line, left, err := common.BytesWithLenToBytes(lineBeg)
	if err != nil {
		*reply = []byte("wrong query")
		return nil
	}
	if len(line) < 4 {
		*reply = []byte("wrong query length")
		return nil
	}
	operation := string(line[0:4])
	verificationNeeded := true
	for _, noVerification := range common.ConnectionsWithoutVerification {
		if bytes.Equal([]byte(operation), noVerification) {
			verificationNeeded = false
			break
		}
	}
	byt := []byte{}
	signatureBytes := []byte{}

	if len(line) > 4 {
		byt = line[4:]
	}
	signatureBytes = left
	if verificationNeeded {
		if len(signatureBytes) == 0 {
			*reply = []byte("Invalid signature with length 0")
			return nil
		}
		activeWallet = wallet.GetActiveWallet()

		pubKey := activeWallet.PublicKey
		if signatureBytes[0] != 0 {
			pubKey = activeWallet.PublicKey2
		}

		if !wallet.Verify(common.BytesToLenAndBytes(line), signatureBytes, pubKey.GetBytes()) {
			*reply = []byte("Invalid signature")
			return nil
		}
	}
	switch operation {
	case "STAT":
		handleSTAT(byt, reply)
	case "WALL":
		handleWALL(byt, reply)
	case "TRAN":
		handleTRAN(byt, reply)
	case "CNCL":
		handleCNCL(byt, reply)
	case "VIEW":
		handleVIEW(byt, reply)
	case "ACCT":
		handleACCT(byt, reply)
	case "MINE":
		handleMINE(byt, reply)
	case "CHCK":
		handleCHECK(byt, reply)
	case "ENCR":
		handleENCR(byt, reply)
	case "VOTE":
		handleVOTE(byt, reply)
	//case "ACCS":
	//	handleACCS(byt, reply)
	case "DETS":
		handleDETS(byt, reply)
	case "STAK":
		handleSTAK(byt, reply)
	case "ADEX":
		handleADEX(byt, reply)
	case "LTKN":
		handleLTKN(byt, reply)
	case "GTBL":
		handleGTBL(byt, reply)
	case "ESCR":
		handleESCR(byt, reply)
	case "MULT":
		handleMULT(byt, reply)
	default:
		*reply = []byte("Invalid operation")
	}
	return nil
}

func handleWALL(line []byte, reply *[]byte) {
	log.Println(string(line))
	w := wallet.GetActiveWallet()
	r, err := json.Marshal(w)
	if err != nil {
		log.Println("Cannot marshal stat's struct")
		return
	}
	*reply = r
}

func handleCHECK(line []byte, reply *[]byte) {
	log.Println(string(line))
	w := wallet.GetActiveWallet()
	*reply = nil
	_, err := pubkeys.LoadPubKey(w.Address.GetBytes())
	if err != nil {
		*reply = []byte("Primary pubkey is not registered in blockchain. Please send transaction including primary PubKey to blockchain")

	}
	_, err = pubkeys.LoadPubKey(w.Address2.GetBytes())
	if err != nil {
		*reply = []byte("Secondary pubkey is not registered in blockchain. Please send transaction including secondary PubKey to blockchain")
	}
}

func handleESCR(line []byte, reply *[]byte) {
	log.Println(string(line))
	*reply = nil
	//primary := line[0] == 0
	//delay := common.GetInt64FromByte(line[1:9])

}

func handleMULT(line []byte, reply *[]byte) {
	log.Println(string(line))
	*reply = nil
}

func handleENCR(line []byte, reply *[]byte) {
	log.Println(string(line))
	*reply = nil

	enb1, err := oqs.GenerateBytesFromParams(common.SigName(), common.PubKeyLength(), common.PrivateKeyLength(), common.SignatureLength(), common.IsPaused())
	if err != nil {
		*reply = []byte(err.Error())
		return
	}
	enb := common.BytesToLenAndBytes(enb1)

	enb2, err := oqs.GenerateBytesFromParams(common.SigName2(), common.PubKeyLength2(), common.PrivateKeyLength2(), common.SignatureLength2(), common.IsPaused2())
	if err != nil {
		*reply = []byte(err.Error())
		return
	}
	enb = append(enb, common.BytesToLenAndBytes(enb2)...)
	*reply = enb
}

func handleMINE(line []byte, reply *[]byte) {
	ip := [4]byte{0, 0, 0, 0}
	if len(line) == 4 {
		copy(ip[:], line)
	}
	firstDel := common.GetDelegatedAccountAddress(1)
	if firstDel.GetHex() != common.GetDelegatedAccount().Hex() {
		nonceServices.InitNonceService()
		go nonceServices.StartSubscribingNonceMsgSelf()
		go nonceServices.StartSubscribingNonceMsg(tcpip.MyIP)
		if bytes.Equal(ip[:], []byte{0, 0, 0, 0}) == false {
			go nonceServices.StartSubscribingNonceMsg(ip)
		}
		*reply = []byte("Mining initiated")
	} else {
		*reply = []byte("First delegated account just automatically mines")
	}

}

func handleVOTE(line []byte, reply *[]byte) {
	enb1, line, err := common.BytesWithLenToBytes(line)
	if err != nil {
		*reply = []byte("cannot decode bytes 1")
		return
	}
	en1 := []byte{}
	if len(enb1) > 0 {
		config1, err := oqs.FromBytesToEncryptionConfig(enb1)
		if err != nil {
			*reply = []byte("cannot decode encryption from bytes 1")
			return
		}
		en1, _ = oqs.GenerateBytesFromParams(config1.SigName, config1.PubKeyLength, config1.PrivateKeyLength, config1.SignatureLength, config1.IsPaused)
	}
	enb2, left, err := common.BytesWithLenToBytes(line)
	if err != nil || len(left) > 0 {
		*reply = []byte("cannot decode bytes 2")
		return
	}
	en2 := []byte{}
	if len(enb2) > 0 {
		config2, err := oqs.FromBytesToEncryptionConfig(enb2)
		if err != nil {
			*reply = []byte("cannot decode encryption from bytes 2")
			return
		}
		en2, _ = oqs.GenerateBytesFromParams(config2.SigName, config2.PubKeyLength, config2.PrivateKeyLength, config2.SignatureLength, config2.IsPaused)
	}
	nonceServices.SetEncryptionData(en1, en2)
	*reply = []byte("Voting for new encryption is successful")
}

func handleGTBL(byt []byte, reply *[]byte) {
	if len(byt) == 2*common.AddressLength {
		addr := common.Address{}
		addr.Init(byt[:common.AddressLength])
		coin := common.Address{}
		coin.Init(byt[common.AddressLength : 2*common.AddressLength])
		inputs := stateDB.BalanceOfFunc
		ba := common.LeftPadBytes(addr.GetBytes(), 32)
		inputs = append(inputs, ba...)

		h := common.GetHeight()

		bl, err := blocks.LoadBlock(h)
		if err != nil {
			*reply = []byte(fmt.Sprint(err))
			return
		}

		output, _, _, _, _, err := blocks.GetViewFunctionReturns(coin, inputs, bl)
		if err != nil {
			*reply = []byte("Some error in SC query GTBL")
			return
		}
		*reply = common.Hex2Bytes(output)
	} else {
		*reply = []byte("Invalid query GTBL")
	}
}

func handleLTKN(line []byte, reply *[]byte) {
	blocks.StateMutex.RLock()
	accs := blocks.State.GetAllRegisteredTokens()
	blocks.StateMutex.RUnlock()
	if len(accs) > 0 {
		newAccs := map[string]stateDB.TokenInfo{}
		for k, v := range accs {
			newAccs[hex.EncodeToString(k[:])] = v
		}
		am, err := json.Marshal(newAccs)
		if err != nil {
			*reply = []byte(fmt.Sprint(err))
			return
		}
		*reply = am
	}
}

func handleADEX(byt []byte, reply *[]byte) {

	dexAcc := account.GetDexAccountByAddressBytes(byt[:common.AddressLength])
	marshal := dexAcc.Marshal()
	*reply = marshal
}

func handleVIEW(line []byte, reply *[]byte) {
	m := blocks.PasiveFunction{}

	err := json.Unmarshal(line, &m)
	if err != nil {
		*reply = []byte(fmt.Sprint(err))
		return
	}

	bl, err := blocks.LoadBlock(m.Height)
	if err != nil {
		*reply = []byte(fmt.Sprint(err))
		return
	}

	l, logs, _, _, _, err := blocks.GetViewFunctionReturns(m.Address, m.OptData, bl)
	if err != nil {
		*reply = []byte(fmt.Sprint(logs))
	}
	*reply, _ = hex.DecodeString(l)
}

func handleDETS(line []byte, reply *[]byte) {

	switch len(line) {
	case common.AddressLength:
		byt := [common.AddressLength]byte{}
		copy(byt[:], line)
		account.AccountsRWMutex.RLock()
		acc := account.Accounts.AllAccounts[byt]
		account.AccountsRWMutex.RUnlock()
		am := acc.Marshal()
		*reply = append([]byte("AC"), am...)
		break
	case common.HashLength:
		tx, err := transactionsDefinition.LoadFromDBPoolTx(common.TransactionDBPrefix[:], line)
		if err != nil {
			log.Println(err)
			*reply = []byte("TX")
			return
		}
		txb := tx.GetBytes()
		*reply = append([]byte("TX"), txb...)
		break
	case 8:
		height := common.GetInt64FromByte(line)
		block, err := blocks.LoadBlock(height)
		if err != nil {
			log.Println(err)
			*reply = []byte("BL")
			return
		}
		bb := block.GetBytes()
		*reply = append([]byte("BL"), bb...)
		break
	default:
		*reply = []byte("NO")
	}
}

func handleACCT(line []byte, reply *[]byte) {

	byt := [common.AddressLength]byte{}
	copy(byt[:], line[:common.AddressLength])
	account.AccountsRWMutex.RLock()
	acc := account.Accounts.AllAccounts[byt]
	defer account.AccountsRWMutex.RUnlock()
	am := acc.Marshal()
	*reply = am
}

func handleSTAK(line []byte, reply *[]byte) {

	byt := [common.AddressLength]byte{}
	copy(byt[:], line[:common.AddressLength])
	n := int(line[common.AddressLength])
	account.StakingRWMutex.RLock()
	acc := account.StakingAccounts[n].AllStakingAccounts[byt]
	locked, _ := account.GetLockedAmount(byt[:], common.GetHeight(), n)
	defer account.StakingRWMutex.RUnlock()
	am := acc.Marshal()
	*reply = append(am, common.GetByteInt64(locked)...)
}

//func handleACCS(line []byte, reply *[]byte) {
//
//	byt := [common.AddressLength]byte{}
//	copy(byt[:], line[:common.AddressLength])
//	for i:=0;i<256;i++ {
//		if common.ContainsKeyInMap(account.StakingAccounts[i].AllStakingAccounts, byt) {
//			acc := account.StakingAccounts[i].AllStakingAccounts[byt]
//			am := acc.Marshal()
//		}
//	}
//	*reply = am
//}

func handleTRAN(byt []byte, reply *[]byte) {

	*reply = []byte("transaction sent")
	transactionServices.OnMessage([4]byte{0, 0, 0, 0}, byt)

}

func handleCNCL(byt []byte, reply *[]byte) {

	*reply = []byte("hash is not 32 bytes")

	if len(byt) == common.HashLength {
		//TODO nice to have cancelling for any user not only owner of node
		if transactionsPool.PoolsTx.TransactionExists(byt) {
			tx := transactionsPool.PoolsTx.PopTransactionByHash(byt)
			if bytes.Equal(tx.TxParam.Sender.GetBytes(), activeWallet.MainAddress.GetBytes()) == false {
				transactionsPool.PoolsTx.AddTransaction(tx, tx.Hash)
				*reply = []byte("you are not the owner of transaction")
				return
			}
			transactionsPool.PoolsTx.BanTransactionByHash(byt)
		}
		if transactionsPool.PoolTxEscrow.TransactionExists(byt) {
			tx := transactionsPool.PoolTxEscrow.PopTransactionByHash(byt)
			if bytes.Equal(tx.TxParam.Sender.GetBytes(), activeWallet.MainAddress.GetBytes()) == false {
				transactionsPool.PoolTxEscrow.AddTransaction(tx, tx.Hash)
				*reply = []byte("you are not the owner of transaction")
				return
			}
			transactionsPool.PoolTxEscrow.BanTransactionByHash(byt)
		}
		if transactionsPool.PoolTxMultiSign.TransactionExists(byt) {
			tx := transactionsPool.PoolTxMultiSign.PopTransactionByHash(byt)
			if bytes.Equal(tx.TxParam.Sender.GetBytes(), activeWallet.MainAddress.GetBytes()) == false {
				transactionsPool.PoolTxMultiSign.AddTransaction(tx, tx.Hash)
				*reply = []byte("you are not the owner of transaction")
				return
			}
			transactionsPool.PoolTxMultiSign.BanTransactionByHash(byt)
		}
		//TODO to prune DB from bad transactions from time to time
		*reply = []byte("transaction cancelled")
		return
	}

}

func handleSTAT(byt []byte, reply *[]byte) {
	sm := statistics.GetStatsManager()
	msb, err := common.Marshal(sm.Stats, common.StatDBPrefix)
	if err != nil {
		log.Println(err)
		return
	}
	*reply = msb
}
