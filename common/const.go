package common

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/okuralabs/okura-node/crypto/oqs"
	"golang.org/x/exp/rand"
	"log"
	"os"
	"strconv"
	"sync"
)

var (
	Decimals                       uint8   = 8
	MaxTotalSupply                 int64   = 230000000000000000
	InitSupply                     int64   = 23000000000000000
	RewardRatio                            = 1e-7
	ValidationTag                          = "validationTag"
	DifficultyMultiplier           int32   = 10
	BlockTimeInterval              float32 = 10 // 10 sec.
	DifficultyChange               float32 = 10
	MaxGasUsage                    int64   = 13700000 // circa 6.5k transactions in block
	MaxGasPrice                    int64   = 100000
	MaxTransactionsPerBlock        int16   = 5000 // on average 500 TPS
	MaxTransactionInPool                   = 10000
	MaxPeersConnected              int     = 6
	NumberOfHashesInBucket         int64   = 32
	NumberOfBlocksInBucket         int64   = 20
	MaxNumberOfTxBans              int     = 60480
	MinStakingForNode              int64   = 100000000000000
	MinStakingUser                 int64   = 100000000000 // should be 100000000000
	OraclesHeightDistance          int64   = 6            // one minute on average
	VotingHeightDistance           int64   = 60           // 60 => ten minute on average
	MaxTransactionDelay            int64   = 60480        // one week
	MaxTransactionInMultiSigPool   int64   = 60480        //one week
	ConnectionMaxTries                     = 2
	DefaultWalletHomePath                  = "/.okura/db/wallet/"
	DefaultBlockchainHomePath              = "/.okura/db/blockchain/"
	ConnectionsWithoutVerification         = [][]byte{[]byte("TRAN"), []byte("STAT"), []byte("ENCR"), []byte("DETS"), []byte("STAK"), []byte("ADEX")}
)

// db prefixes
var (
	BlocksDBPrefix                     = [2]byte{'B', 'I'}
	StatDBPrefix                       = [2]byte{'M', 'S'}
	BlockHeaderDBPrefix                = [2]byte{'H', 'B'}
	WalletDBPrefix                     = [2]byte{'W', '0'}
	PubKeyDBPrefix                     = [2]byte{'P', 'K'}
	PubKeyMarshalDBPrefix              = [2]byte{'P', 'M'}
	PubKeyMerkleTrieDBPrefix           = [2]byte{'M', 'K'}
	PubKeyRootHashMerkleTreeDBPrefix   = [2]byte{'R', 'K'}
	PubKeyBytesMerkleTrieDBPrefix      = [2]byte{'B', 'K'}
	BlockByHeightDBPrefix              = [2]byte{'B', 'H'}
	TransactionsHashesByHeightDBPrefix = [2]byte{'R', 'H'}
	MerkleTreeDBPrefix                 = [2]byte{'M', 'M'}
	MerkleNodeDBPrefix                 = [2]byte{'N', 'N'}
	RootHashMerkleTreeDBPrefix         = [2]byte{'R', 'R'}
	TransactionDBPrefix                = [2]byte{'T', 'T'}
	//StakingDBPrefix                    = [2]byte{'S', 'S'}
	TransactionPoolHashesDBPrefix    = [2]byte{'D', '0'}
	TransactionToSendHashesDBPrefix  = [2]byte{'E', '0'}
	TransactionSyncingHashesDBPrefix = [2]byte{'S', '0'}
	AccountsDBPrefix                 = [2]byte{'A', 'C'}
	StakingAccountsDBPrefix          = [2]byte{'S', 'A'}
	OutputLogsHashesDBPrefix         = [2]byte{'O', '0'}
	OutputLogDBPrefix                = [2]byte{'Z', '0'}
	OutputAddressesHashesDBPrefix    = [2]byte{'C', '0'}
	TokenDetailsDBPrefix             = [2]byte{'T', 'D'}
	DexAccountsDBPrefix              = [2]byte{'D', 'A'}
)

var chainID = int16(23)
var chainIDMutex = sync.Mutex{}
var nodeSignPrimary = true
var delegatedAccount Address
var rewardPercentage int16
var ShiftToPastInReset int64
var ShiftToPastMutex sync.RWMutex

func GetChainID() int16 {
	chainIDMutex.Lock()
	defer chainIDMutex.Unlock()
	return chainID
}

func SetChainID(chainid int16) {
	chainIDMutex.Lock()
	defer chainIDMutex.Unlock()
	chainID = chainid
}

func SetNodeSignPrimary(primary bool) {
	nodeSignPrimary = primary
}

func GetNodeSignPrimary(height int64) bool {
	if height == 0 {
		return true
	}
	if IsPaused() == false && IsPaused2() == false {
		if rand.Intn(2) == 0 {
			return true
		} else {
			return false
		}
	}
	if nodeSignPrimary && (IsPaused() == false) {
		return true
	}
	if (nodeSignPrimary == false) && (IsPaused2() == false) {
		return false
	}
	if IsPaused() == false {
		return true
	}
	if IsPaused2() == false {
		return false
	}
	return true
}

func GetDelegatedAccount() Address {
	return delegatedAccount
}

func GetMyRewardPercentage() int16 {
	return rewardPercentage
}

func init() {
	enc1 := oqs.NewConfigEnc1()
	fmt.Print(enc1.ToString())
	enc2 := oqs.NewConfigEnc2()
	fmt.Print(enc2.ToString())

	SetEncryption(enc1.SigName, enc1.PubKeyLength, enc1.PrivateKeyLength, enc1.SignatureLength, enc1.IsPaused, true)
	SetEncryption(enc2.SigName, enc2.PubKeyLength, enc2.PrivateKeyLength, enc2.SignatureLength, enc2.IsPaused, false)

	//log.SetOutput(io.Discard)
	ShiftToPastInReset = 1
	homePath, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	err = godotenv.Load(homePath + "/.okura/.env")
	if err != nil {
		log.Fatal("Error loading .env file", err)
	}
	da, err := strconv.Atoi(os.Getenv("DELEGATED_ACCOUNT"))
	if err != nil {
		log.Fatal("Error getting DELEGATED_ACCOUNT")
	}
	delegatedAccount = GetDelegatedAccountAddress(int16(da))

	//DefaultPercentageReward int16 = 500 // 50 %
	v, err := strconv.Atoi(os.Getenv("REWARD_PERCENTAGE"))
	if err != nil {
		log.Fatal("Error getting REWARD_PERCENTAGE")
	}
	rewardPercentage = int16(v)
	if rewardPercentage > 500 {
		log.Fatal("reward for operational account has to be less than 50%")
	}
}
