package blocks

import (
	"bytes"
	"fmt"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/crypto/oqs"
	"github.com/okuralabs/okura-node/pubkeys"
	"github.com/okuralabs/okura-node/wallet"
	"log"
)

type BaseHeader struct {
	PreviousHash     common.Hash      `json:"previous_hash"`
	Difficulty       int32            `json:"difficulty"`
	Height           int64            `json:"height"`
	DelegatedAccount common.Address   `json:"delegated_account"`
	OperatorAccount  common.Address   `json:"operator_account"`
	RootMerkleTree   common.Hash      `json:"root_merkle_tree"`
	Encryption1      []byte           `json:"encryption_1"`
	Encryption2      []byte           `json:"encryption_2"`
	Signature        common.Signature `json:"signature"`
	SignatureMessage []byte           `json:"signature_message"`
}

type BaseBlock struct {
	BaseHeader       BaseHeader  `json:"header"`
	BlockHeaderHash  common.Hash `json:"block_header_hash"`
	BlockTimeStamp   int64       `json:"block_time_stamp"`
	RewardPercentage int16       `json:"reward_percentage"`
	Supply           int64       `json:"supply"`
	PriceOracle      int64       `json:"price_oracle"`
	RandOracle       int64       `json:"rand_oracle"`
	PriceOracleData  []byte      `json:"price_oracle_data"`
	RandOracleData   []byte      `json:"rand_oracle_data"`
}

func FromBytesToEncryptionConfig(bb []byte, primary bool) (oqs.ConfigEnc, error) {
	if len(bb) == 0 {

		if !common.IsPaused() && primary {
			enc := oqs.CreateEncryptionScheme(common.SigName(), common.PubKeyLength(), common.PrivateKeyLength(), common.SignatureLength(), common.IsPaused())
			return enc, nil
		} else if !common.IsPaused2() && !primary {
			//TODO maybe one should make it better
			enc := oqs.CreateEncryptionScheme(common.SigName2(), common.PubKeyLength2(), common.PrivateKeyLength2(), common.SignatureLength2(), common.IsPaused2())
			return enc, nil
		} else {
			return oqs.ConfigEnc{}, fmt.Errorf("no valid encyption scheme")
		}
	}
	return oqs.FromBytesToEncryptionConfig(bb[:])
}

// GetString returns a string representation of BaseHeader.
func (b *BaseHeader) GetString() string {

	enc1String := ""
	enc1, err := FromBytesToEncryptionConfig(b.Encryption1[:], true)
	if err != nil {
		enc1String = fmt.Sprint(err)
	}
	enc1String = enc1.ToString()
	enc2String := ""
	enc2, err := FromBytesToEncryptionConfig(b.Encryption2[:], false)
	if err != nil {
		enc2String = fmt.Sprint(err)
	}
	enc2String = enc2.ToString()
	return fmt.Sprintf("PreviousHash: %s\nDifficulty: %d\nHeight: %d\nDelegatedAccount: %s\nOperatorAccount: %s\nRootMerkleTree: %s\nEncryption1: %s\nEncryption2: %s\nSignature: %s\nSignatureMessage: %x",
		b.PreviousHash.GetHex(), b.Difficulty, b.Height, b.DelegatedAccount.GetHex(), b.OperatorAccount.GetHex(), b.RootMerkleTree.GetHex(), enc1String, enc2String, b.Signature.GetHex(), b.SignatureMessage)
}

// GetString returns a string representation of BaseBlock.
func (b *BaseBlock) GetString() string {
	return fmt.Sprintf("Header: {%s}\nBlockHeaderHash: %s\nBlockTimeStamp: %d\nRewardPercentage: %d\nSupply: %d\nPriceOracle: %d\nRandOracle: %d\n",
		b.BaseHeader.GetString(), b.BlockHeaderHash.GetHex(), b.BlockTimeStamp, b.RewardPercentage, b.Supply, b.PriceOracle, b.RandOracle)
}

func (b *BaseHeader) GetBytesWithoutSignature() []byte {
	rb := b.PreviousHash.GetBytes()
	rb = append(rb, common.GetByteInt32(b.Difficulty)...)
	rb = append(rb, common.GetByteInt64(b.Height)...)
	rb = append(rb, b.DelegatedAccount.GetBytes()...)
	rb = append(rb, b.OperatorAccount.GetBytesWithPrimary()...)
	rb = append(rb, b.RootMerkleTree.GetBytes()...)
	rb = append(rb, common.BytesToLenAndBytes(b.Encryption1)...)
	rb = append(rb, common.BytesToLenAndBytes(b.Encryption2)...)
	return rb
}

func (b *BaseHeader) GetBytes() []byte {
	rb := b.PreviousHash.GetBytes()
	rb = append(rb, common.GetByteInt32(b.Difficulty)...)
	rb = append(rb, common.GetByteInt64(b.Height)...)
	rb = append(rb, b.DelegatedAccount.GetBytes()...)
	rb = append(rb, b.OperatorAccount.GetBytesWithPrimary()...)
	rb = append(rb, b.RootMerkleTree.GetBytes()...)

	rb = append(rb, common.BytesToLenAndBytes(b.Encryption1)...)
	rb = append(rb, common.BytesToLenAndBytes(b.Encryption2)...)

	rb = append(rb, common.BytesToLenAndBytes(b.SignatureMessage)...)
	rb = append(rb, common.BytesToLenAndBytes(b.Signature.GetBytes())...)
	//log.Println("block ", b.Height, " len bytes ", len(rb))
	return rb
}

func (bh *BaseHeader) Verify() bool {
	signatureBlockHeaderMessage := bh.GetBytesWithoutSignature()
	if !bytes.Equal(signatureBlockHeaderMessage, bh.SignatureMessage) {
		log.Println("signatures are different")
		return false
	}
	calcHash, err := common.CalcHashToByte(signatureBlockHeaderMessage)
	if err != nil {
		log.Println(err)
		return false
	}
	a := bh.OperatorAccount
	primary := common.GetNodeSignPrimary(bh.Height)
	pk, err := pubkeys.LoadPubKeyWithPrimary(a, primary)
	if err != nil {
		log.Println(err)
		return false
	}
	return wallet.Verify(calcHash, bh.Signature.GetBytes(), pk.GetBytes())
}

func (bh *BaseHeader) Sign(primary bool) (common.Signature, []byte, error) {
	signatureBlockHeaderMessage := bh.GetBytesWithoutSignature()
	calcHash, err := common.CalcHashToByte(signatureBlockHeaderMessage)
	if err != nil {
		return common.Signature{}, nil, err
	}
	w := wallet.GetActiveWallet()
	sign, err := w.Sign(calcHash, primary)
	if err != nil {
		return common.Signature{}, nil, err
	}
	return *sign, signatureBlockHeaderMessage, nil
}

func (bh *BaseHeader) GetFromBytes(b []byte) ([]byte, error) {
	if len(b) < 117+common.SignatureLength() && len(b) < 117+common.SignatureLength2() {
		return nil, fmt.Errorf("not enough bytes to decode BaseHeader")
	}
	//log.Println("block decompile len bytes ", len(b))

	bh.PreviousHash = common.GetHashFromBytes(b[:32])
	bh.Difficulty = common.GetInt32FromByte(b[32:36])
	bh.Height = common.GetInt64FromByte(b[36:44])
	address, err := common.BytesToAddress(b[44:64])
	if err != nil {
		return nil, err
	}
	bh.DelegatedAccount = address
	opAddress, err := common.BytesToAddress(b[64:85])
	if err != nil {
		return nil, err
	}
	bh.OperatorAccount = opAddress
	bh.RootMerkleTree = common.GetHashFromBytes(b[85:117])

	msgb, b, err := common.BytesWithLenToBytes(b[117:])
	if err != nil {
		return nil, err
	}
	bh.Encryption1 = msgb
	msgb, b, err = common.BytesWithLenToBytes(b[:])
	if err != nil {
		return nil, err
	}
	bh.Encryption2 = msgb

	msgb, b, err = common.BytesWithLenToBytes(b[:])
	if err != nil {
		return nil, err
	}
	bh.SignatureMessage = msgb
	sigBytes, b, err := common.BytesWithLenToBytes(b[:])
	if err != nil {
		return nil, err
	}
	sig, err := common.GetSignatureFromBytes(sigBytes, opAddress)
	if err != nil {
		return nil, err
	}
	bh.Signature = sig
	return b, nil
}

func (bb *BaseBlock) GetBytes() []byte {
	b := bb.BaseHeader.GetBytes()
	b = append(b, bb.BlockHeaderHash.GetBytes()...)
	b = append(b, common.GetByteInt64(bb.BlockTimeStamp)...)
	b = append(b, common.GetByteInt16(bb.RewardPercentage)...)
	b = append(b, common.GetByteInt64(bb.Supply)...)
	b = append(b, common.GetByteInt64(bb.PriceOracle)...)
	b = append(b, common.GetByteInt64(bb.RandOracle)...)
	b = append(b, common.BytesToLenAndBytes(bb.PriceOracleData)...)
	b = append(b, common.BytesToLenAndBytes(bb.RandOracleData)...)
	return b
}

func (bb *BaseBlock) GetFromBytes(b []byte) ([]byte, error) {
	if len(b) < 116+common.SignatureLength()+44+16 {
		return nil, fmt.Errorf("not enough bytes to decode BaseBlock")
	}
	b, err := bb.BaseHeader.GetFromBytes(b)
	if err != nil {
		return nil, err
	}
	bb.BlockHeaderHash = common.GetHashFromBytes(b[:32])
	bb.BlockTimeStamp = common.GetInt64FromByte(b[32:40])
	bb.RewardPercentage = common.GetInt16FromByte(b[40:42])
	bb.Supply = common.GetInt64FromByte(b[42:50])
	bb.PriceOracle = common.GetInt64FromByte(b[50:58])
	bb.RandOracle = common.GetInt64FromByte(b[58:66])
	bb.PriceOracleData, b, err = common.BytesWithLenToBytes(b[66:])
	if err != nil {
		return nil, err
	}
	bb.RandOracleData, b, err = common.BytesWithLenToBytes(b[:])
	if err != nil {
		return nil, err
	}
	return b[:], nil
}

func (b *BaseHeader) CalcHash() (common.Hash, error) {
	toByte, err := common.CalcHashToByte(b.GetBytes())
	if err != nil {
		return common.Hash{}, err
	}
	hash := common.GetHashFromBytes(toByte)
	return hash, nil
}
