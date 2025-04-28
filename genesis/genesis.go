// Package genesis maintains access to the genesis file.
package genesis

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/okuralabs/okura-node/account"
	"github.com/okuralabs/okura-node/blocks"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/pubkeys"
	"github.com/okuralabs/okura-node/transactionsDefinition"
	"github.com/okuralabs/okura-node/transactionsPool"
	"github.com/okuralabs/okura-node/wallet"
	"log"
	"os"
	"strings"
)

type GenesisStaking struct {
	Account            string `json:"account"`
	Amount             int64  `json:"amount"`
	LockedAmount       int64  `json:"locked_amount"`
	ReleasedPerBlock   int64  `json:"released_per_block"`
	DelegatedAccount   int16  `json:"delegated_account"`
	OperationalAccount bool   `json:"operational_account"`
	PubKey             string `json:"pub_key"`
	PubKey2            string `json:"pub_key_2,omitempty"`
}

type GenesisTransactions struct {
	Account                 string `json:"account"`
	Amount                  int64  `json:"amount"`
	LockedAmount            int64  `json:"locked_amount"`
	ReleasedPerBlock        int64  `json:"released_per_block"`
	DelegatedAccount        int16  `json:"delegated_account"`
	PubKey                  string `json:"pub_key"`
	Signature               string `json:"signature"`
	EscrowTransactionsDelay int64  `json:"escrow_transactions_delay"`
	MultiSignNumber         uint8  `json:"multi_sign_number"`
	MultiSignAddresses      string `json:"multi_sign_addresses"`
}

// Genesis represents the genesis file.
type Genesis struct {
	Timestamp               int64                 `json:"date"`
	ChainID                 int16                 `json:"chain_id"`   // The chain id represents an unique id for this running instance.
	Difficulty              int32                 `json:"difficulty"` // How difficult it needs to be to solve the work problem.
	RewardRatio             float64               `json:"reward_ratio"`
	BlockTimeInterval       float32               `json:"block_time_interval"`
	MaxTotalSupply          int64                 `json:"max_total_supply"`
	InitSupply              int64                 `json:"init_supply"`
	DifficultyMultiplier    int32                 `json:"difficulty_multiplier"`
	DifficultyChange        float32               `json:"difficulty_change"`
	MaxGasUsage             int64                 `json:"max_gas_usage"`
	MaxGasPrice             int64                 `json:"max_gas_price"`
	MaxTransactionsPerBlock int16                 `json:"max_transactions_per_block"`
	MaxTransactionInPool    int                   `json:"max_transaction_in_pool"`
	MaxPeersConnected       int                   `json:"max_peers_connected"`
	NumberOfHashesInBucket  int64                 `json:"number_of_hashes_in_bucket"`
	NumberOfBlocksInBucket  int64                 `json:"number_of_blocks_in_bucket"`
	MinStakingForNode       int64                 `json:"min_staking_for_node"`
	MinStakingUser          int64                 `json:"min_staking_user"`
	OraclesHeightDistance   int64                 `json:"oracles_height_distance"`
	VotingHeightDistance    int64                 `json:"voting_height_distance"`
	StakedBalances          []GenesisStaking      `json:"staked_balances"`
	Transactions            []GenesisTransactions `json:"transactions"`
	Signature               string                `json:"signature"`
	OperatorPubKey          string                `json:"operator_pub_key"`
}

func storeGenesisPubKey(pubkeystr string, primary bool) common.PubKey {
	pubKeyOpBytes, err := hex.DecodeString(pubkeystr)
	if err != nil {
		log.Fatal("cannot decode address from string in genesis block")
	}
	addressOp1, err := common.PubKeyToAddress(pubKeyOpBytes[:], primary)
	if err != nil {
		log.Fatalf("cannot retrieve operator address from pub key in genesis block %v", err)
	}
	pubKeyOp1 := common.PubKey{}
	err = pubKeyOp1.Init(pubKeyOpBytes, addressOp1)
	if err != nil {
		log.Fatalf("cannot initialize operator pub key in genesis block %v", err)
	}
	err = blocks.StorePubKey(pubKeyOp1)
	if err != nil {
		log.Fatal("cannot store genesis operator pubkey", err)
	}
	err = blocks.StorePubKeyInPatriciaTrie(pubKeyOp1)
	if err != nil {
		log.Fatal("cannot store genesis pubkey in patricia trie", err)
	}
	return pubKeyOp1
}

func CreateBlockFromGenesis(genesis Genesis) blocks.Block {
	//myWallet := wallet.GetActiveWallet()
	//log.Println(myWallet.PublicKey.GetHex())
	initSupplyWithoutStaked := common.InitSupply
	for _, balance := range genesis.StakedBalances {
		initSupplyWithoutStaked -= balance.Amount
	}
	pkOp1 := storeGenesisPubKey(genesis.OperatorPubKey, true)
	addressOp1 := pkOp1.Address
	accDel1 := account.Accounts.AllAccounts[addressOp1.ByteValue]
	accDel1.Balance = initSupplyWithoutStaked
	accDel1.Address = addressOp1.ByteValue
	account.Accounts.AllAccounts[addressOp1.ByteValue] = accDel1

	walletNonce := int16(0)
	blockTransactionsHashesBytes := [][]byte{}
	blockTransactionsHashes := []common.Hash{}
	genesisTxs := []transactionsDefinition.Transaction{}
	for _, genTx := range genesis.Transactions {
		ab, err := hex.DecodeString(genTx.Account)
		if err != nil {
			log.Fatal("cannot decode address from string in genesis block")
		}
		a, err := common.BytesToAddress(ab)
		if err != nil {
			log.Fatal("cannot decode address from bytes in genesis block")
		}
		tx := GenesisTransaction(addressOp1, a, genTx, walletNonce, genesis.Timestamp)
		err = tx.CalcHashAndSet()
		if err != nil {
			log.Fatalf("cannot calculate hash of transaction in genesis block %v", err)
		}
		err = tx.StoreToDBPoolTx(common.TransactionPoolHashesDBPrefix[:])
		if err != nil {
			log.Fatalf("cannot store transaction of genesis block %v", err)
		}
		genesisTxs = append(genesisTxs, tx)
		blockTransactionsHashesBytes = append(blockTransactionsHashesBytes, tx.GetHash().GetBytes())
		blockTransactionsHashes = append(blockTransactionsHashes, tx.GetHash())
		walletNonce++
	}
	for _, stkTx := range genesis.StakedBalances {

		ab, err := hex.DecodeString(stkTx.Account)
		if err != nil {
			log.Fatal("cannot decode address from string in genesis block")
		}
		addrb := [common.AddressLength]byte{}
		copy(addrb[:], ab)
		delAddrb := [common.AddressLength]byte{}
		firstDel := common.GetDelegatedAccountAddress(stkTx.DelegatedAccount)
		copy(delAddrb[:], firstDel.GetBytes())
		sd := account.StakingDetail{
			Amount:      stkTx.Amount,
			Reward:      0,
			LastUpdated: genesis.Timestamp,
		}
		sds := map[int64][]account.StakingDetail{}
		sds[0] = []account.StakingDetail{sd}
		as := account.StakingAccount{
			StakedBalance:      stkTx.Amount,
			StakingRewards:     0,
			DelegatedAccount:   delAddrb,
			Address:            addrb,
			OperationalAccount: stkTx.OperationalAccount,
			LockedInitBlock:    []int64{0},
			LockedAmount:       []int64{stkTx.LockedAmount},
			ReleasePerBlock:    []int64{stkTx.ReleasedPerBlock},
			StakingDetails:     sds,
		}
		pk1 := storeGenesisPubKey(stkTx.PubKey, true)
		if stkTx.PubKey2 != "" {
			pk2 := storeGenesisPubKey(stkTx.PubKey2, false)
			err := pubkeys.AddPubKeyToAddress(pk2, pk1.Address)
			if err != nil {
				log.Fatal("cannot add secondary address")
			}
		}
		if err != nil {
			log.Fatal("cannot decode pubkey from string in genesis block")
		}
		account.StakingAccounts[stkTx.DelegatedAccount].AllStakingAccounts[addrb] = as
	}
	err := account.StoreStakingAccounts(0)
	if err != nil {
		log.Fatalln("cannot store staking accounts")
	}
	genesisMerkleTrie, err := transactionsPool.BuildMerkleTree(0, blockTransactionsHashesBytes, transactionsPool.GlobalMerkleTree.DB)
	if err != nil {
		log.Fatalf("cannot generate genesis merkleTrie %v", err)
	}
	defer genesisMerkleTrie.Destroy()

	err = genesisMerkleTrie.StoreTree(0)
	if err != nil {
		log.Fatalf("cannot store genesis merkleTrie %v", err)
	}
	rootHash := common.Hash{}
	rootHash.Set(genesisMerkleTrie.GetRootHash())

	bh := blocks.BaseHeader{
		PreviousHash:     common.EmptyHash(),
		Difficulty:       genesis.Difficulty,
		Height:           0,
		DelegatedAccount: common.GetDelegatedAccountAddress(1),
		OperatorAccount:  addressOp1,
		RootMerkleTree:   rootHash,
		Signature:        common.Signature{},
		SignatureMessage: []byte{},
	}
	signatureBlockHeaderMessage := bh.GetBytesWithoutSignature()
	bh.SignatureMessage = signatureBlockHeaderMessage
	hashb, err := common.CalcHashToByte(signatureBlockHeaderMessage)
	if err != nil {
		log.Fatalf("cannot calculate hash of genesis block header %v", err)
	}

	myWallet := wallet.GetActiveWallet()
	sign, err := myWallet.Sign(hashb, true)
	if err != nil {
		log.Fatalf("cannot sign genesis block header %v", err)
	}
	bh.Signature = *sign
	log.Println("Block Signature:", bh.Signature.GetHex())

	signature, err := common.GetSignatureFromString(genesis.Signature, addressOp1)
	if err != nil {
		log.Fatal(err)
	}
	bh.Signature = signature

	bhHash, err := bh.CalcHash()
	if err != nil {
		log.Fatalf("cannot calculate hash of genesis block header %v", err)
	}

	if bh.Verify() == false {
		log.Fatal("Block Header signature in genesis block fails to verify")
	}
	bb := blocks.BaseBlock{
		BaseHeader:       bh,
		BlockHeaderHash:  bhHash,
		BlockTimeStamp:   genesis.Timestamp,
		RewardPercentage: 0,
		Supply:           common.InitSupply + account.GetReward(common.InitSupply),
	}

	bl := blocks.Block{
		BaseBlock:          bb,
		TransactionsHashes: blockTransactionsHashes,
		BlockHash:          common.EmptyHash(),
	}
	hash, err := bl.CalcBlockHash()
	if err != nil {
		log.Fatalf("cannot calculate hash of genesis block %v", err)
	}
	bl.BlockHash = hash

	return bl
}

func GenesisTransaction(sender common.Address, recipient common.Address, genTx GenesisTransactions, walletNonce int16, timestamp int64) transactionsDefinition.Transaction {
	pkb, err := hex.DecodeString(genTx.PubKey)
	if err != nil {
		log.Fatal(err)
	}
	pk := common.PubKey{}
	err = pk.Init(pkb[:], recipient)
	if err != nil {
		log.Fatal(err)
	}
	err = blocks.StorePubKey(pk)
	if err != nil {
		log.Fatal(err)
	}

	msa := [][common.AddressLength]byte{}

	addrStr := strings.Split(genTx.MultiSignAddresses, ",")
	for _, as := range addrStr {
		if as == "" {
			continue
		}
		a, err := hex.DecodeString(as)
		if err != nil {
			log.Fatal(err)
		}
		ab := [common.AddressLength]byte{}
		copy(ab[:], a[:common.AddressLength])
		msa = append(msa, ab)
	}

	txdata := transactionsDefinition.TxData{
		Recipient:                  recipient,
		Amount:                     genTx.Amount,
		OptData:                    nil,
		LockedAmount:               genTx.LockedAmount,
		ReleasePerBlock:            genTx.ReleasedPerBlock,
		DelegatedAccountForLocking: common.GetDelegatedAccountAddress(genTx.DelegatedAccount),
		EscrowTransactionsDelay:    genTx.EscrowTransactionsDelay,
		MultiSignNumber:            genTx.MultiSignNumber,
		MultiSignAddresses:         msa,
	}
	txParam := transactionsDefinition.TxParam{
		ChainID:     common.GetChainID(),
		Sender:      sender,
		SendingTime: timestamp,
		Nonce:       walletNonce,
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

	err = t.CalcHashAndSet()
	if err != nil {
		log.Fatal("calc hash error", err)
	}

	signature, err := common.GetSignatureFromString(genTx.Signature, sender)

	if err != nil {
		log.Fatal(err)
	}
	t.Signature = signature

	if t.Verify() == false {
		myWallet := wallet.GetActiveWallet()
		log.Println(myWallet.PublicKey.GetHex())
		err = t.Sign(myWallet, true)
		if err != nil {
			log.Fatal("Signing error", err)
		}
		println(t.Signature.GetHex())
		log.Fatal("genesis transaction cannot be verified")
	}
	log.Println("transaction signature: ", t.Signature.GetHex())
	return t
}

// InitGenesis sets initial values written in genesis conf file
func InitGenesis() {
	pathhome, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	genesis, err := Load(pathhome + "/.okura/genesis/config/genesis.json")
	if err != nil {
		log.Fatal(err)
	}
	setInitParams(genesis)
	genesisBlock := CreateBlockFromGenesis(genesis)
	reward := account.GetReward(common.InitSupply)
	err = blocks.ProcessBlockTransfers(genesisBlock, reward)
	if err != nil {
		log.Fatalf("cannot process transactions in genesis block %v", err)
	}
	err = genesisBlock.StoreBlock()
	if err != nil {
		log.Fatal(err)
	}
	err = account.StoreAccounts(0)
	if err != nil {
		log.Fatal(err)
	}
	err = account.StoreStakingAccounts(0)
	if err != nil {
		log.Fatal(err)
	}

}

func setInitParams(genesisConfig Genesis) {
	common.SetHeight(0)
	common.SetChainID(genesisConfig.ChainID)

	common.BlockTimeInterval = genesisConfig.BlockTimeInterval
	common.RewardRatio = genesisConfig.RewardRatio
	common.BlockTimeInterval = genesisConfig.BlockTimeInterval
	common.MaxTotalSupply = genesisConfig.MaxTotalSupply
	common.InitSupply = genesisConfig.InitSupply
	common.DifficultyMultiplier = genesisConfig.DifficultyMultiplier
	common.DifficultyChange = genesisConfig.DifficultyChange
	common.MaxGasUsage = genesisConfig.MaxGasUsage
	common.MaxGasPrice = genesisConfig.MaxGasPrice
	common.MaxTransactionsPerBlock = genesisConfig.MaxTransactionsPerBlock
	common.MaxTransactionInPool = genesisConfig.MaxTransactionInPool
	common.MaxPeersConnected = genesisConfig.MaxPeersConnected
	common.NumberOfHashesInBucket = genesisConfig.NumberOfHashesInBucket
	common.NumberOfBlocksInBucket = genesisConfig.NumberOfBlocksInBucket
	common.MinStakingForNode = genesisConfig.MinStakingForNode
	common.MinStakingUser = genesisConfig.MinStakingUser
	common.OraclesHeightDistance = genesisConfig.OraclesHeightDistance
	common.VotingHeightDistance = genesisConfig.VotingHeightDistance
}

// Load opens and consumes the genesis file.
func Load(path string) (Genesis, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Genesis{}, err
	}

	var genesis Genesis
	err = json.Unmarshal(content, &genesis)
	if err != nil {
		return Genesis{}, err
	}

	mainWallet := wallet.GetActiveWallet()
	fmt.Println(mainWallet.MainAddress.GetHex())

	del1 := common.GetDelegatedAccountAddress(1)
	delegatedAccount := common.GetDelegatedAccount()
	if mainWallet.PublicKey.GetBytes() != nil &&
		genesis.OperatorPubKey[:100] != mainWallet.PublicKey.GetHex()[:100] &&
		delegatedAccount.GetHex() == del1.GetHex() {
		log.Println(mainWallet.PublicKey.GetHex())
		log.Fatal("Main Wallet address should be the same as in config genesis.json file")
	}
	return genesis, nil
}
