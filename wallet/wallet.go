package wallet

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/wonabru/bip39"
	"io"
	"log"
	"sync"

	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/crypto/oqs"
	"golang.org/x/crypto/sha3"
)

var globalMutex sync.RWMutex

type Wallet struct {
	password            string
	passwordBytes       []byte
	Iv                  []byte `json:"iv"`
	secretKey           common.PrivKey
	secretKey2          common.PrivKey
	EncryptedSecretKey  []byte         `json:"encrypted_secret_key"`
	EncryptedSecretKey2 []byte         `json:"encrypted_secret_key2"`
	PublicKey           common.PubKey  `json:"public_key"`
	PublicKey2          common.PubKey  `json:"public_key2"`
	Address             common.Address `json:"address"`
	Address2            common.Address `json:"address2"`
	MainAddress         common.Address `json:"main_address"`
	signer              oqs.Signature
	signer2             oqs.Signature
	HomePath            string `json:"home_path"`
	HomePath2           string `json:"home_path2"`
	HomePathOld         string `json:"home_path_old,omitempty"`
	HomePath2Old        string `json:"home_path2_old,omitempty"`
	WalletNumber        uint8  `json:"wallet_number"`
}

var activeWallet *Wallet

type AnyWallet interface {
	GetWallet() Wallet
}

func InitActiveWallet(walletNumber uint8, password string) {
	var err error
	activeWallet, err = Load(walletNumber, password)
	if err != nil {
		log.Println("wrong password")
		os.Exit(1)
	}
	if activeWallet == nil {
		log.Println("failed to load wallet")
		os.Exit(1)
	}
}

func (w *Wallet) SetPassword(password string) {
	(*w).password = password
	(*w).passwordBytes = passwordToByte(password)
}

func GetActiveWallet() *Wallet {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	return activeWallet
}

func (w *Wallet) ShowInfo() string {

	s := fmt.Sprintln("Length of public key:", w.PublicKey.GetLength())
	s += fmt.Sprintln("Beginning of public key:", w.PublicKey.GetHex()[:10])
	s += fmt.Sprintln("Address:", w.Address.GetHex())
	s += fmt.Sprintln("Length of private key:", w.GetSecretKey().GetLength())
	s += fmt.Sprintln("Length of public key 2:", w.PublicKey2.GetLength())
	s += fmt.Sprintln("Beginning of public key 2:", w.PublicKey2.GetHex()[:10])
	s += fmt.Sprintln("Address 2:", w.Address2.GetHex())
	s += fmt.Sprintln("Length of private key 2:", w.GetSecretKey2().GetLength())
	s += fmt.Sprintln("MainAddress:", w.MainAddress.GetHex())
	s += fmt.Sprintln("Wallet location", w.HomePath)
	s += fmt.Sprintln("Wallet location 2", w.HomePath2)
	s += fmt.Sprintln("Wallet Number", w.WalletNumber)
	fmt.Println(s)
	return s
}

func passwordToByte(password string) []byte {
	sh := make([]byte, 32)
	sha3.ShakeSum256(sh, []byte(password))
	return sh
}

func (w *Wallet) GetSigName(primary bool) string {
	if primary {
		return w.signer.Details().Name
	} else {
		return w.signer2.Details().Name
	}
}

func EmptyWallet(walletNumber uint8, sigName, sigName2 string) *Wallet {
	homePath, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	return &Wallet{
		password:      "",
		passwordBytes: nil,
		Iv:            nil,
		secretKey:     common.PrivKey{},
		secretKey2:    common.PrivKey{},
		PublicKey:     common.PubKey{},
		PublicKey2:    common.PubKey{},
		Address:       common.Address{},
		Address2:      common.Address{},
		MainAddress:   common.Address{},
		signer:        oqs.Signature{},
		signer2:       oqs.Signature{},
		HomePath:      homePath + common.DefaultWalletHomePath + strconv.Itoa(int(walletNumber)) + "/" + sigName,
		HomePath2:     homePath + common.DefaultWalletHomePath + strconv.Itoa(int(walletNumber)) + "/" + sigName2,
		HomePathOld:   homePath + common.DefaultWalletHomePath + strconv.Itoa(int(walletNumber)) + "/" + sigName,
		HomePath2Old:  homePath + common.DefaultWalletHomePath + strconv.Itoa(int(walletNumber)) + "/" + sigName2,
		WalletNumber:  walletNumber,
	}
}

func GenerateNewWallet(walletNumber uint8, password string) (*Wallet, error) {
	if len(password) < 1 {
		return nil, fmt.Errorf("password cannot be empty")
	}
	w := EmptyWallet(walletNumber, common.SigName(), common.SigName2())
	w.SetPassword(password)
	(*w).Iv = generateNewIv()
	var signer oqs.Signature
	var signer2 oqs.Signature
	//defer signer.Clean()

	// ignore potential errors everywhere
	err := signer.Init(common.SigName(), nil)
	if err != nil {
		return nil, err
	}
	pubKey, err := signer.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	mainAddress, err := common.PubKeyToAddress(pubKey, true)
	if err != nil {
		return nil, err
	}
	err = w.PublicKey.Init(pubKey, mainAddress)
	if err != nil {
		return nil, err
	}
	(*w).Address = w.PublicKey.GetAddress()
	(*w).MainAddress = (*w).Address
	err = w.secretKey.Init(signer.ExportSecretKey(), w.Address)
	if err != nil {
		return nil, err
	}
	(*w).signer = signer

	err = signer2.Init(common.SigName2(), nil)
	if err != nil {
		return nil, err
	}
	pubKey2, err := signer2.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	err = w.PublicKey2.Init(pubKey2, mainAddress)
	if err != nil {
		return nil, err
	}
	(*w).Address2 = w.PublicKey2.GetAddress()
	err = w.secretKey2.Init(signer2.ExportSecretKey(), w.Address2)
	if err != nil {
		return nil, err
	}
	(*w).signer2 = signer2

	fmt.Print(signer2.Details())
	return w, nil
}

func (w *Wallet) AddNewEncryptionToActiveWallet(sigName string, primary bool) error {

	if len(w.password) < 1 {
		return fmt.Errorf("password cannot be empty")
	}

	var signer oqs.Signature
	ew := EmptyWallet(w.WalletNumber, sigName, sigName)
	// ignore potential errors everywhere
	err := signer.Init(sigName, nil)
	if err != nil {
		return err
	}
	pubKey, err := signer.GenerateKeyPair()
	if err != nil {
		return err
	}
	mainAddress, err := common.PubKeyToAddress(pubKey, primary)
	if err != nil {
		return err
	}
	if primary {
		err = w.PublicKey.Init(pubKey, mainAddress)
		if err != nil {
			return err
		}
		(*w).Address = w.PublicKey.GetAddress()
		err = w.secretKey.Init(signer.ExportSecretKey(), w.Address)
		if err != nil {
			return err
		}
		(*w).signer = signer
		(*w).HomePath = ew.HomePath
	} else {
		err = w.PublicKey2.Init(pubKey, mainAddress)
		if err != nil {
			return err
		}
		(*w).Address2 = w.PublicKey2.GetAddress()
		err = w.secretKey2.Init(signer.ExportSecretKey(), w.Address2)
		if err != nil {
			return err
		}
		(*w).signer2 = signer
		(*w).HomePath2 = ew.HomePath2
	}

	log.Println(signer.Details())
	return nil
}

func generateNewIv() []byte {
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		panic(err)
	}
	return iv
}

func (w *Wallet) encrypt(v []byte) ([]byte, error) {
	cb, err := aes.NewCipher(w.passwordBytes)
	if err != nil {
		log.Println("Can not create AES function")
		return []byte{}, err
	}
	v = append([]byte("validationTag"), v...)
	ciphertext := make([]byte, aes.BlockSize+len(v))
	stream := cipher.NewCTR(cb, w.Iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], v)
	return ciphertext, nil
}

func (w *Wallet) decrypt(v []byte) ([]byte, error) {
	cb, err := aes.NewCipher(w.passwordBytes)
	if err != nil {
		log.Println("Can not create AES function")
		return []byte{}, err
	}

	plaintext := make([]byte, aes.BlockSize+len(v))
	stream := cipher.NewCTR(cb, w.Iv)
	stream.XORKeyStream(plaintext, v[aes.BlockSize:])
	if !bytes.Equal(plaintext[:len(common.ValidationTag)], []byte(common.ValidationTag)) {
		return nil, fmt.Errorf("wrong password")
	}
	return plaintext[len(common.ValidationTag):], nil
}

func (w *Wallet) GetMnemonicWords(primary bool) (string, error) {
	var secret []byte
	var secretLength int
	if primary {
		secret = w.GetSecretKey().GetBytes()
		secretLength = w.GetSecretKey().GetLength()
	} else {
		secret = w.GetSecretKey2().GetBytes()
		secretLength = w.GetSecretKey2().GetLength()
	}
	if secret == nil {
		return "", fmt.Errorf("you need load wallet first")
	}

	if secretLength > 64 {
		return "", fmt.Errorf("secret must be less than 64 bytes")
	}
	if secretLength < 64 {
		log.Println("not all mnemonic words are important. secret is less than 64 bytes")
		secretTmp := make([]byte, 64)
		copy(secretTmp, secret)
		secret = secretTmp[:]
	}
	mnemonic, _ := bip39.NewMnemonic(secret)

	secretKey, _ := bip39.MnemonicToByteArray(mnemonic)
	if !bytes.Equal(secretKey[:secretLength], secret[:secretLength]) {
		log.Println("Can not restore secret key from mnemonic")
		return "", fmt.Errorf("can not restore secret key from mnemonic")
	}
	return mnemonic, nil
}

func (w *Wallet) RestoreSecretKeyFromMnemonic(mnemonic string, primary bool) error {
	secretKey, err := bip39.MnemonicToByteArray(mnemonic)
	if err != nil {
		log.Println("Can not restore secret key")
		return err
	}
	var signer oqs.Signature
	if primary {
		if len(secretKey) < common.PrivateKeyLength() {
			return fmt.Errorf("not enough bytes for primary encryption private key")
		}
		err = w.secretKey.Init(secretKey[:common.PrivateKeyLength()], w.Address)
		if err != nil {
			return err
		}

		err = signer.Init(common.SigName(), w.secretKey.GetBytes())
		if err != nil {
			return err
		}
		(*w).signer = signer
	} else {
		if len(secretKey) < common.PrivateKeyLength2() {
			return fmt.Errorf("not enough bytes for secondary encryption private key")
		}
		err = w.secretKey2.Init(secretKey[:common.PrivateKeyLength2()], w.Address2)
		if err != nil {
			return err
		}
		err = signer.Init(common.SigName2(), w.secretKey2.GetBytes())
		if err != nil {
			return err
		}
		(*w).signer2 = signer
	}

	return nil
}

func (w *Wallet) Store(makeBackup bool) error {
	if w.GetSecretKey().GetBytes() == nil {
		return fmt.Errorf("you need load wallet first")
	}

	if makeBackup {
		// Get the next available backup number
		backupNum := 1
		backupPath := w.HomePathOld + "_backup" + fmt.Sprintf("%d", backupNum)
		for {
			if _, err := os.Stat(backupPath); os.IsNotExist(err) {
				break
			}
			backupNum++
			backupPath = w.HomePathOld + "_backup" + fmt.Sprintf("%d", backupNum)
		}
		err := os.Rename(w.HomePathOld, backupPath)
		if err != nil {
			return err
		}
	}
	globalMutex.Lock()
	defer globalMutex.Unlock()
	walletDB, err := leveldb.OpenFile(w.HomePath, nil)
	if err != nil {
		return err
	}
	if walletDB == nil {
		return fmt.Errorf("database is nil")
	}
	defer walletDB.Close()

	se, err := w.encrypt(w.secretKey.GetBytes())
	if err != nil {
		log.Println(err)
		return err
	}

	w2 := w
	(*w2).EncryptedSecretKey = make([]byte, len(se))
	copy((*w2).EncryptedSecretKey, se)

	if makeBackup {
		// Get the next available backup number
		backupNum := 1
		backupPath := w.HomePath2Old + "_backup" + fmt.Sprintf("%d", backupNum)
		for {
			if _, err := os.Stat(backupPath); os.IsNotExist(err) {
				break
			}
			backupNum++
			backupPath = w.HomePath2Old + "_backup" + fmt.Sprintf("%d", backupNum)
		}
		err = os.Rename(w.HomePath2Old, backupPath)
		if err != nil {
			return err
		}
	}
	walletDB2, err := leveldb.OpenFile(w.HomePath2, nil)
	if err != nil {
		return err
	}
	if walletDB2 == nil {
		return fmt.Errorf("database is nil")
	}
	defer walletDB2.Close()

	se, err = w.encrypt(w2.secretKey2.GetBytes())
	if err != nil {
		log.Println(err)
		return err
	}

	(*w2).EncryptedSecretKey2 = make([]byte, len(se))
	copy((*w2).EncryptedSecretKey2, se)

	wm, err := json.Marshal(&w2)
	if err != nil {
		log.Println(err)
		return err
	}
	prefix := common.WalletDBPrefix
	prefix[1] = w.WalletNumber
	err = walletDB.Put(prefix[:], wm, nil)
	if err != nil {
		return err
	}
	// Put a key-value pair into the database
	err = walletDB2.Put(prefix[:], wm, nil)
	if err != nil {
		return err
	}

	return nil
}

func Load(walletNumber uint8, password string) (*Wallet, error) {
	if len(password) == 0 {
		return nil, fmt.Errorf("password cannot be empty")
	}

	globalMutex.Lock()
	defer globalMutex.Unlock()

	w := EmptyWallet(walletNumber, common.SigName(), common.SigName2())
	if w == nil {
		return nil, fmt.Errorf("failed to create empty wallet")
	}

	homePath := w.HomePath
	homePath2 := w.HomePath2
	// Open the database with the provided options
	walletDB, err := leveldb.OpenFile(w.HomePath, nil)
	if err != nil {
		return nil, err
	}
	if walletDB == nil {
		return nil, fmt.Errorf("database is nil")
	}
	defer walletDB.Close()
	prefix := common.WalletDBPrefix
	prefix[1] = walletNumber
	value, err := walletDB.Get(prefix[:], nil)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(value, w)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	w.SetPassword(password)
	ds, err := w.decrypt(w.EncryptedSecretKey)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	err = w.secretKey.Init(ds[:common.PrivateKeyLength()], w.Address)
	if err != nil {
		return nil, err
	}
	var signer oqs.Signature
	err = signer.Init(common.SigName(), w.secretKey.GetBytes())
	if err != nil {
		return nil, err
	}
	(*w).signer = signer
	(*w).HomePath2 = homePath2
	walletDB2, err := leveldb.OpenFile(w.HomePath2, nil)
	if err != nil {
		return nil, err
	}
	if walletDB2 == nil {
		return nil, fmt.Errorf("database is nil")
	}

	defer walletDB2.Close()

	prefix = common.WalletDBPrefix
	prefix[1] = walletNumber
	value, err = walletDB2.Get(prefix[:], nil)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(value, w)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	w.MainAddress.Primary = true
	w.Address.Primary = true
	w.Address2.Primary = false
	w.PublicKey.Address.Primary = true
	w.PublicKey2.Address.Primary = false
	w.PublicKey.MainAddress.Primary = true
	w.PublicKey2.MainAddress.Primary = true
	w.SetPassword(password)
	ds, err = w.decrypt(w.EncryptedSecretKey2)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	err = w.secretKey2.Init(ds[:common.PrivateKeyLength2()], w.Address2)
	if err != nil {
		return nil, err
	}
	var signer2 oqs.Signature
	err = signer2.Init(common.SigName2(), w.secretKey2.GetBytes())
	if err != nil {
		return nil, err
	}
	(*w).signer2 = signer2
	(*w).HomePath = homePath

	log.Println("PubKey:", w.PublicKey.GetHex())
	log.Println("PubKey2:", w.PublicKey2.GetHex())
	log.Println("MainAddress:", w.MainAddress.GetHex())
	return w, err
}

func (w *Wallet) ChangePassword(password, newPassword string) error {
	if w.passwordBytes == nil {
		return fmt.Errorf("you need load wallet first")
	}
	if !bytes.Equal(passwordToByte(password), w.passwordBytes) {
		return fmt.Errorf("current password is not valid")
	}

	globalMutex.Lock()
	defer globalMutex.Unlock()

	w2 := &Wallet{
		password:      newPassword,
		passwordBytes: passwordToByte(newPassword),
		Iv:            w.Iv,
		secretKey:     w.secretKey,
		PublicKey:     w.PublicKey,
		Address:       w.Address,
		signer:        w.signer,
		secretKey2:    w.secretKey2,
		PublicKey2:    w.PublicKey2,
		Address2:      w.Address2,
		signer2:       w.signer2,
		MainAddress:   w.MainAddress,
		HomePath:      w.HomePath,
		HomePathOld:   w.HomePathOld,
		HomePath2:     w.HomePath2,
		HomePath2Old:  w.HomePath2Old,
		WalletNumber:  w.WalletNumber,
	}

	err := w2.Store(false)
	if err != nil {
		log.Println("Can not store new wallet")
		return err
	}
	_, err = Load(w2.WalletNumber, newPassword)
	if err != nil {
		return err
	}
	return nil
}

func (w *Wallet) Sign(data []byte, primary bool) (*common.Signature, error) {
	if len(data) > 0 {
		if primary {
			signature, err := w.signer.Sign(data)
			if err != nil {
				return nil, err
			}
			signature = append([]byte{0}, signature...)
			sig := &common.Signature{}
			err = sig.Init(signature, w.MainAddress)
			if err != nil {
				return nil, err
			}
			return sig, nil
		} else {
			signature2, err := w.signer2.Sign(data)
			if err != nil {
				return nil, err
			}
			signature2 = append([]byte{1}, signature2...)
			sig := &common.Signature{}
			err = sig.Init(signature2, w.MainAddress)
			if err != nil {
				return nil, err
			}
			return sig, nil
		}
	}
	return nil, fmt.Errorf("input data are empty")
}

func Verify(msg []byte, sig []byte, pubkey []byte) bool {
	var verifier oqs.Signature
	var err error
	primary := sig[0] == 0
	sig = sig[1:]
	if primary && !common.IsPaused() {
		err = verifier.Init(common.SigName(), nil)
		if err != nil {
			return false
		}
		if verifier.Details().LengthPublicKey == len(pubkey) {
			isVerified, err := verifier.Verify(msg, sig, pubkey)
			if err != nil {
				return false
			}
			return isVerified
		}
	}
	if !primary && !common.IsPaused2() {
		err = verifier.Init(common.SigName2(), nil)
		if err != nil {
			return false
		}
		if verifier.Details().LengthPublicKey == len(pubkey) {
			isVerified, err := verifier.Verify(msg, sig, pubkey)
			if err != nil {
				return false
			}
			return isVerified
		}
	}
	return false
}

func (w *Wallet) GetSecretKey() common.PrivKey {
	if w == nil {
		return common.PrivKey{}
	}
	return w.secretKey
}

func (w *Wallet) Check() bool {
	if (w != nil) && len(w.passwordBytes) > 0 && (len(w.GetSecretKey().GetBytes()) == w.GetSecretKey().GetLength()) {
		return true
	}
	return false
}

func (w *Wallet) GetSecretKey2() common.PrivKey {
	if w == nil {
		return common.PrivKey{}
	}
	return w.secretKey2
}

func (w *Wallet) Check2() bool {
	if (w != nil) && len(w.passwordBytes) > 0 && (len(w.GetSecretKey2().GetBytes()) == w.GetSecretKey2().GetLength()) {
		return true
	}
	return false
}
