package services

import (
	"bytes"
	"fmt"
	"github.com/okuralabs/okura-node/account"
	"github.com/okuralabs/okura-node/blocks"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/crypto/oqs"
	"github.com/okuralabs/okura-node/message"
	"github.com/okuralabs/okura-node/oracles"
	"github.com/okuralabs/okura-node/tcpip"
	"github.com/okuralabs/okura-node/transactionsDefinition"
	"github.com/okuralabs/okura-node/transactionsPool"
	"github.com/okuralabs/okura-node/wallet"
	"log"
	"sync"
)

var (
	SendChanNonce     chan []byte
	SendChanSelfNonce chan []byte
	SendMutexNonce    sync.RWMutex
	SendChanTx        chan []byte
	SendMutexTx       sync.RWMutex
	SendChanSync      chan []byte
	SendMutexSync     sync.RWMutex
)

func CreateBlockFromNonceMessage(nonceTx []transactionsDefinition.Transaction,
	lastBlock blocks.Block,
	merkleTrie *transactionsPool.MerkleTree,
	txs []common.Hash) (blocks.Block, error) {

	encryption1 := []byte{}
	encryption2 := []byte{}
	b := []byte{}
	var err error
	myWallet := wallet.GetActiveWallet()
	heightTransaction := nonceTx[0].GetHeight()
	//totalFee := int64(0)
	for _, at := range nonceTx {
		heightLastBlocktransaction := common.GetInt64FromByte(at.GetData().GetOptData()[:8])
		hashLastBlocktransaction := at.GetData().GetOptData()[8:40]
		if !bytes.Equal(hashLastBlocktransaction, lastBlock.GetBlockHash().GetBytes()) {
			ha, err := blocks.LoadHashOfBlock(heightTransaction - 2)
			if err != nil {
				return blocks.Block{}, err
			}
			return blocks.Block{}, fmt.Errorf("last block hash and nonce hash do not match %v %v", ha, hashLastBlocktransaction)
		}
		if heightTransaction != heightLastBlocktransaction+1 {
			return blocks.Block{}, fmt.Errorf("last block height and nonce height do not match")
		}
		encryption1, b, err = common.BytesWithLenToBytes(at.GetData().GetOptData()[56:])
		if err != nil {
			return blocks.Block{}, err
		}
		encryption2, b, err = common.BytesWithLenToBytes(b[:])
		if err != nil {
			return blocks.Block{}, err
		}
		if len(encryption1) == 0 {
			encryption1, err = oqs.GenerateBytesFromParams(common.SigName(), common.PubKeyLength(), common.PrivateKeyLength(), common.SignatureLength(), common.IsPaused())
			if err != nil {
				return blocks.Block{}, err
			}
		}
		if len(encryption2) == 0 {
			encryption2, err = oqs.GenerateBytesFromParams(common.SigName2(), common.PubKeyLength2(), common.PrivateKeyLength2(), common.SignatureLength2(), common.IsPaused2())
			if err != nil {
				return blocks.Block{}, err
			}
		}
	}

	reward := account.GetReward(lastBlock.GetBlockSupply())
	supply := lastBlock.GetBlockSupply() + reward

	sendingTimeTransaction := nonceTx[0].GetParam().SendingTime
	ti := sendingTimeTransaction - lastBlock.GetBlockTimeStamp()
	bblock := lastBlock.GetBaseBlock()
	diff := blocks.AdjustDifficulty(bblock.BaseHeader.Difficulty, ti)
	sendingTimeMessage := common.GetByteInt64(nonceTx[0].GetParam().SendingTime)
	rootMerkleTrie := common.Hash{}
	rootMerkleTrie.Set(merkleTrie.GetRootHash())
	bh := blocks.BaseHeader{
		PreviousHash:     lastBlock.GetBlockHash(),
		Difficulty:       diff,
		Height:           heightTransaction,
		DelegatedAccount: common.GetDelegatedAccount(),
		OperatorAccount:  myWallet.Address,
		RootMerkleTree:   rootMerkleTrie,
		Encryption1:      encryption1,
		Encryption2:      encryption2,
		Signature:        common.Signature{},
		SignatureMessage: sendingTimeMessage,
	}
	sign, signatureBlockHeaderMessage, err := bh.Sign(common.GetNodeSignPrimary(heightTransaction))
	if err != nil {
		return blocks.Block{}, err
	}
	bh.Signature = sign
	bh.SignatureMessage = signatureBlockHeaderMessage
	bhHash, err := bh.CalcHash()
	if err != nil {
		return blocks.Block{}, err
	}
	totalStaked := account.GetStakedInAllDelegatedAccounts()
	priceOracle, priceOracleData, err := oracles.CalculatePriceOracle(heightTransaction, totalStaked)
	if err != nil {
		log.Println("could not establish price oracle", err)
	}
	randOracle, randOracleData, err := oracles.CalculateRandOracle(heightTransaction, totalStaked)
	if err != nil {
		log.Println("could not establish rand oracle", err)
	}
	bb := blocks.BaseBlock{
		BaseHeader:       bh,
		BlockHeaderHash:  bhHash,
		BlockTimeStamp:   common.GetCurrentTimeStampInSecond(),
		RewardPercentage: common.GetMyRewardPercentage(),
		Supply:           supply,
		PriceOracle:      priceOracle,
		RandOracle:       randOracle,
		PriceOracleData:  priceOracleData,
		RandOracleData:   randOracleData,
	}

	bl := blocks.Block{
		BaseBlock:          bb,
		TransactionsHashes: txs,
		BlockHash:          common.Hash{},
	}
	hash, err := bl.CalcBlockHash()
	if err != nil {
		return blocks.Block{}, err
	}
	bl.BlockHash = hash

	return bl, nil
}

func GenerateBlockMessage(bl blocks.Block) message.TransactionsMessage {

	bm := message.BaseMessage{
		Head:    []byte("bl"),
		ChainID: common.GetChainID(),
	}
	txm := [2]byte{}
	copy(txm[:], append([]byte("N"), 0))
	atm := message.TransactionsMessage{
		BaseMessage:       bm,
		TransactionsBytes: map[[2]byte][][]byte{},
	}
	atm.TransactionsBytes[txm] = [][]byte{bl.GetBytes()}

	return atm
}

func SendNonce(ip [4]byte, nb []byte) {
	nb = append(ip[:], nb...)
	SendMutexNonce.Lock()
	SendChanNonce <- nb
	SendMutexNonce.Unlock()
}

func BroadcastBlock(bl blocks.Block) {
	atm := GenerateBlockMessage(bl)
	nb := atm.GetBytes()
	var ip [4]byte
	var peers = tcpip.GetPeersConnected(tcpip.NonceTopic)
	for topicip, _ := range peers {
		copy(ip[:], topicip[2:])
		SendNonce(ip, nb)
	}
}
