package main

import (
	"io"
	"log"
	"strconv"
	"strings"

	"github.com/okuralabs/okura-node/account"
	"github.com/okuralabs/okura-node/blocks"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/genesis"
	serverrpc "github.com/okuralabs/okura-node/rpc/server"
	nonceService "github.com/okuralabs/okura-node/services/nonceService"
	syncServices "github.com/okuralabs/okura-node/services/syncService"
	"github.com/okuralabs/okura-node/services/transactionServices"
	"github.com/okuralabs/okura-node/statistics"
	"github.com/okuralabs/okura-node/tcpip"
	"github.com/okuralabs/okura-node/transactionsPool"
	"github.com/okuralabs/okura-node/wallet"

	_ "net/http/pprof"
	"os"
	"time"
)

// MultiWriter implements io.Writer and writes to multiple writers
type MultiWriter struct {
	writers []io.Writer
}

func (t *MultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(p), nil
}
func main() {
	homePath, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	os.MkdirAll(homePath+common.DefaultLogsHomePath, 0744)
	// Create log file
	logFile, err := os.OpenFile(homePath+common.DefaultLogsHomePath+"mining.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()
	// Create multi writer
	mw := &MultiWriter{
		writers: []io.Writer{
			os.Stdout, // Console output
			logFile,   // File output
		},
	}
	// Create logger with multi writer
	log.New(mw, "", log.LstdFlags)
	// Set logger as the default logger
	log.SetOutput(mw)
	log.SetFlags(log.LstdFlags)
	// Now you can use log functions as usual
	log.Println("Application started")

	//fmt.Print("Enter password: ")
	//password, err := terminal.ReadPassword(0)
	//if err != nil {
	//	log.Fatal(err)
	//}
	// only for purpose of testing to start from beginning
	// Initialize wallet
	log.Println("Initializing wallet...")
	password := "a"
	wallet.InitActiveWallet(0, string(password))
	addrbytes := [common.AddressLength]byte{}
	copy(addrbytes[:], wallet.GetActiveWallet().Address.GetBytes())
	// Initialize accounts
	a := account.Account{
		Balance: 0,
		Address: addrbytes,
	}
	allAccounts := map[[20]byte]account.Account{}
	allAccounts[addrbytes] = a
	account.Accounts = account.AccountsType{AllAccounts: allAccounts}
	err = account.StoreAccounts(0)
	if err != nil {
		log.Fatal("Failed to store accounts:", err)
	}

	// Initialize DEX accounts
	log.Println("Initializing DEX accounts...")
	allDexAccounts := map[[20]byte]account.DexAccount{}
	account.DexAccounts = account.DexAccountsType{AllDexAccounts: allDexAccounts}
	err = account.StoreDexAccounts(0)
	if err != nil {
		log.Fatal("Failed to store DEX accounts:", err)
	}

	// Initialize staking accounts
	log.Println("Setting up staking accounts...")
	for i := 1; i < 256; i++ {
		del := common.GetDelegatedAccountAddress(int16(i))
		delbytes := [common.AddressLength]byte{}
		copy(delbytes[:], del.GetBytes())
		sa := account.StakingAccount{
			StakedBalance:    0,
			StakingRewards:   0,
			DelegatedAccount: delbytes,
			StakingDetails:   nil,
		}
		allStakingAccounts := map[[20]byte]account.StakingAccount{}
		allStakingAccounts[addrbytes] = sa
		account.StakingAccounts[i] = account.StakingAccountsType{AllStakingAccounts: allStakingAccounts}
	}
	err = account.StoreStakingAccounts(0)
	if err != nil {
		log.Fatal("Failed to store staking accounts:", err)
	}

	// Initialize transaction pool and merkle tree
	log.Println("Initializing transaction pool and merkle tree...")
	transactionsPool.InitPermanentTrie()
	defer transactionsPool.GlobalMerkleTree.Destroy()

	// Initialize statistics
	statistics.InitStatsManager()

	// Load accounts
	log.Println("Loading accounts...")
	err = account.LoadAccounts(-1)
	if err != nil {
		log.Fatal("Failed to load accounts:", err)
	}
	defer func() {
		log.Println("Storing accounts...")
		account.StoreAccounts(-1)
	}()

	// Load DEX accounts
	log.Println("Loading DEX accounts...")
	err = account.LoadDexAccounts(-1)
	if err != nil {
		log.Fatal("Failed to load DEX accounts:", err)
	}
	defer func() {
		log.Println("Storing DEX accounts...")
		account.StoreDexAccounts(-1)
	}()

	// Load staking accounts
	log.Println("Loading staking accounts...")
	err = account.LoadStakingAccounts(-1)
	if err != nil {
		log.Fatal("Failed to load staking accounts:", err)
	}
	defer func() {
		log.Println("Storing staking accounts...")
		account.StoreStakingAccounts(-1)
	}()

	// Initialize state database
	log.Println("Initializing state database...")
	blocks.InitStateDB()

	// Initialize genesis block
	log.Println("Initializing genesis block...")
	genesis.InitGenesis()

	// Initialize services
	log.Println("Initializing transaction service...")
	transactionServices.InitTransactionService()

	log.Println("Initializing sync service...")
	syncServices.InitSyncService()

	log.Println("Starting RPC server...")
	go serverrpc.ListenRPC()

	log.Println("Initializing nonce service...")
	nonceService.InitNonceService()
	go nonceService.StartSubscribingNonceMsgSelf()
	go nonceService.StartSubscribingNonceMsg(tcpip.MyIP)

	log.Println("Starting transaction and sync message subscriptions...")
	go transactionServices.StartSubscribingTransactionMsg(tcpip.MyIP)
	go syncServices.StartSubscribingSyncMsg(tcpip.MyIP)

	time.Sleep(time.Second)

	if len(os.Args) > 1 {
		log.Println("Processing command line arguments...")
		ips := strings.Split(os.Args[1], ".")
		if len(ips) != 4 {
			log.Println("Invalid IP address format")
			return
		}
		var ip [4]byte
		for i := 0; i < 4; i++ {
			num, err := strconv.Atoi(ips[i])
			if err != nil {
				log.Println("Invalid IP address segment:", ips[i])
				return
			}
			ip[i] = byte(num)
		}

		log.Println("Connecting to peer:", ip)
		go nonceService.StartSubscribingNonceMsg(ip)
		go syncServices.StartSubscribingSyncMsg(ip)
		go transactionServices.StartSubscribingTransactionMsg(ip)
	}

	time.Sleep(time.Second)

	log.Println("Starting peer discovery...")
	chanPeer := make(chan []byte)
	go tcpip.LookUpForNewPeersToConnect(chanPeer)
	topic := [2]byte{}
	ip := [4]byte{}

	log.Println("Entering main loop...")
QF:
	for {
		select {

		case topicip := <-chanPeer:
			copy(topic[:], topicip[:2])
			copy(ip[:], topicip[2:])
			log.Printf("Received peer message - Topic: %s, IP: %v", string(topic[:]), ip)

			if topic[0] == 'T' {
				log.Println("Starting transaction subscription for peer:", ip)
				go transactionServices.StartSubscribingTransactionMsg(ip)
			}
			if topic[0] == 'N' {
				log.Println("Starting nonce subscription for peer:", ip)
				go nonceService.StartSubscribingNonceMsg(ip)
			}
			if topic[0] == 'S' {
				log.Println("Starting self nonce subscription")
				go nonceService.StartSubscribingNonceMsgSelf()
			}
			if topic[0] == 'B' {
				log.Println("Starting sync subscription for peer:", ip)
				go syncServices.StartSubscribingSyncMsg(ip)
			}

		case <-tcpip.Quit:
			log.Println("Received quit signal, shutting down...")
			break QF
		}
	}

}
