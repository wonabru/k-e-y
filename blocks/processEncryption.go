package blocks

import (
	"bytes"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/logger"
	"github.com/okuralabs/okura-node/voting"
	"sync"
)

var VoteChannel chan []byte
var VoteChannelMutex sync.Mutex

func init() {
	VoteChannel = make(chan []byte, 0)
	VoteChannelMutex = sync.Mutex{}
}

// ProcessBlockEncryption : store encryption
func ProcessBlockEncryption(block Block, lastBlock Block) error {
	if lastBlock.GetHeader().Height < 3 {
		return nil
	}
	if !bytes.Equal(block.BaseBlock.BaseHeader.Encryption1[:], lastBlock.BaseBlock.BaseHeader.Encryption1[:]) {
		enc1, err := FromBytesToEncryptionConfig(block.BaseBlock.BaseHeader.Encryption1[:], true)
		if err != nil {
			return err
		}

		logger.GetLogger().Println("new encryption: ", enc1.ToString())
		SetVoteEncryption(block.BaseBlock.BaseHeader.Encryption1[:], true)
		voting.ResetLastVoting()

		err = AddNewPubKeyToActiveWallet(enc1.SigName, true)
		if err != nil {
			return err
		}
	}

	if !bytes.Equal(block.BaseBlock.BaseHeader.Encryption2[:], lastBlock.BaseBlock.BaseHeader.Encryption2[:]) {
		enc2, err := FromBytesToEncryptionConfig(block.BaseBlock.BaseHeader.Encryption2[:], false)
		if err != nil {
			return err
		}

		SetVoteEncryption(block.BaseBlock.BaseHeader.Encryption2[:], false)
		voting.ResetLastVoting()
		err = AddNewPubKeyToActiveWallet(enc2.SigName, false)
		if err != nil {
			return err
		}

	}
	return nil
}

func SetVoteEncryption(enc []byte, primary bool) {
	enc1 := append([]byte{1}, enc...)
	if primary {
		enc1 = append([]byte{0}, enc...)
	}
	VoteChannelMutex.Lock()
	defer VoteChannelMutex.Unlock()
	VoteChannel <- enc1
	logger.GetLogger().Println(string(<-VoteChannel))
}

func SetEncryptionFromBlock(height int64) error {
	block, err := LoadBlock(height)
	if err != nil {
		return err
	}
	enc1, err := FromBytesToEncryptionConfig(block.BaseBlock.BaseHeader.Encryption1[:], true)
	if err != nil {
		return err
	}

	common.SetEncryption(enc1.SigName, enc1.PubKeyLength, enc1.PrivateKeyLength, enc1.SignatureLength, enc1.IsPaused, true)

	enc2, err := FromBytesToEncryptionConfig(block.BaseBlock.BaseHeader.Encryption2[:], false)
	if err != nil {
		return err
	}

	common.SetEncryption(enc2.SigName, enc2.PubKeyLength, enc2.PrivateKeyLength, enc2.SignatureLength, enc2.IsPaused, false)
	return nil
}

func SetEncryptionFromBytes(enc []byte, primary bool) error {

	enc1, err := FromBytesToEncryptionConfig(enc, primary)
	if err != nil {
		return err
	}
	common.SetEncryption(enc1.SigName, enc1.PubKeyLength, enc1.PrivateKeyLength, enc1.SignatureLength, enc1.IsPaused, primary)
	return nil
}
