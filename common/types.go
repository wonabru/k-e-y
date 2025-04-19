package common

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/okuralabs/okura-node/crypto/blake2b"
	"log"
	"sync"
	"sync/atomic"
)

const (
	AddressLength   int = 20
	HashLength      int = 32
	ShortHashLength int = 8
)

type EncryptionConfig struct {
	mu               sync.RWMutex // Using RWMutex for more granular locking
	pubKeyLength     int32
	privateKeyLength int32
	signatureLength  int32
	sigName          string
	isPaused         int32 // Using int32 for atomic operations
	// Secondary config
	pubKeyLength2     int32
	privateKeyLength2 int32
	signatureLength2  int32
	sigName2          string
	isPaused2         int32
}

var encryptionConfigInstance *EncryptionConfig

func GetEncryptionConfigInstance() *EncryptionConfig {
	if encryptionConfigInstance == nil {
		// Initialize with a function to avoid init() if not needed immediately
		initEncryptionConfigInstance()
	}
	return encryptionConfigInstance
}

func initEncryptionConfigInstance() {
	encryptionConfigInstance = &EncryptionConfig{
		pubKeyLength:      897,
		privateKeyLength:  1281,
		signatureLength:   752,
		sigName:           "Falcon-512",
		isPaused:          0,
		pubKeyLength2:     5554,
		privateKeyLength2: 40,
		signatureLength2:  964,
		sigName2:          "MAYO-5",
		isPaused2:         0,
	}
}

func SetEncryption(sigName string, pubKeyLength, privateKeyLength, signatureLength int, isPaused, primary bool) {
	currEnc := GetEncryptionConfigInstance()
	if primary {
		if currEnc.sigName != sigName {

		}
	}
	GetEncryptionConfigInstance().SetEncryption(sigName, pubKeyLength, privateKeyLength, signatureLength, isPaused, primary)
}

func (ec *EncryptionConfig) SetEncryption(sigName string, pubKeyLength, privateKeyLength, signatureLength int, isPaused, primary bool) {

	ec.mu.Lock()
	defer ec.mu.Unlock()
	if primary {
		ec.sigName = sigName
		ec.pubKeyLength = int32(pubKeyLength)
		ec.privateKeyLength = int32(privateKeyLength)
		ec.signatureLength = int32(signatureLength)
		if isPaused {
			atomic.StoreInt32(&ec.isPaused, 1)
		} else {
			atomic.StoreInt32(&ec.isPaused, 0)
		}
	} else {
		ec.sigName2 = sigName
		ec.pubKeyLength2 = int32(pubKeyLength)
		ec.privateKeyLength2 = int32(privateKeyLength)
		ec.signatureLength2 = int32(signatureLength)
		if isPaused {
			atomic.StoreInt32(&ec.isPaused2, 1)
		} else {
			atomic.StoreInt32(&ec.isPaused2, 0)
		}
	}
}

func SigName() string {
	encryptionConfigInstance.mu.RLock()
	defer encryptionConfigInstance.mu.RUnlock()
	return encryptionConfigInstance.sigName
}

func SigName2() string {
	encryptionConfigInstance.mu.RLock()
	defer encryptionConfigInstance.mu.RUnlock()
	return encryptionConfigInstance.sigName2
}

func SetIsPaused(isPaused, primary bool) {
	if primary {
		if isPaused {
			atomic.StoreInt32(&encryptionConfigInstance.isPaused, 1)
		} else {
			atomic.StoreInt32(&encryptionConfigInstance.isPaused, 0)
		}
	} else {
		if isPaused {
			atomic.StoreInt32(&encryptionConfigInstance.isPaused2, 1)
		} else {
			atomic.StoreInt32(&encryptionConfigInstance.isPaused2, 0)
		}
	}
}

func IsPaused() bool {
	var paused *int32
	paused = &encryptionConfigInstance.isPaused
	return atomic.LoadInt32(paused) == 1
}

func IsPaused2() bool {
	var paused *int32
	paused = &encryptionConfigInstance.isPaused2
	return atomic.LoadInt32(paused) == 1
}

func PubKeyLength() int {
	encryptionConfigInstance.mu.RLock()
	defer encryptionConfigInstance.mu.RUnlock()
	return int(encryptionConfigInstance.pubKeyLength)
}

func PubKeyLength2() int {
	encryptionConfigInstance.mu.RLock()
	defer encryptionConfigInstance.mu.RUnlock()
	return int(encryptionConfigInstance.pubKeyLength2)
}

func PrivateKeyLength() int {
	encryptionConfigInstance.mu.RLock()
	defer encryptionConfigInstance.mu.RUnlock()
	return int(encryptionConfigInstance.privateKeyLength)
}

func PrivateKeyLength2() int {
	encryptionConfigInstance.mu.RLock()
	defer encryptionConfigInstance.mu.RUnlock()
	return int(encryptionConfigInstance.privateKeyLength2)
}

func SignatureLength() int {
	encryptionConfigInstance.mu.RLock()
	defer encryptionConfigInstance.mu.RUnlock()
	return int(encryptionConfigInstance.signatureLength)
}

func SignatureLength2() int {
	encryptionConfigInstance.mu.RLock()
	defer encryptionConfigInstance.mu.RUnlock()
	return int(encryptionConfigInstance.signatureLength2)
}

func (a PubKey) GetLength() int {
	if PubKeyLength() == PubKeyLength2() {
		log.Fatal("pubkey length in bytes cannot be equal")
	}
	return len(a.ByteValue)
}

func (p PrivKey) GetLength() int {
	return len(p.ByteValue)
}

func (s Signature) GetLength() int {
	return len(s.ByteValue)
}

func (a Address) GetLength() int {
	return AddressLength
}

func (a Hash) GetLength() int {
	return HashLength
}

func (a ShortHash) GetLength() int {
	return ShortHashLength
}

type Address struct {
	ByteValue [AddressLength]byte `json:"byte_value"`
	Primary   bool                `json:"primary"`
}

func (a *Address) Init(b []byte) error {
	if len(b) != AddressLength && len(b) != AddressLength+1 {
		return fmt.Errorf("error Address initialization with wrong length, should be %v", AddressLength)
	}
	if len(b) == AddressLength+1 {
		a.Primary = b[0] == 0
		b = b[1:]
	} else {
		a.Primary = true
	}
	copy(a.ByteValue[:], b[:])
	return nil
}

func BytesToAddress(b []byte) (Address, error) {
	var a Address
	err := a.Init(b)
	if err != nil {
		log.Println("Cannot init Address")
		return a, err
	}
	return a, nil
}

func PubKeyToAddress(pb []byte, primary bool) (Address, error) {
	hashBlake2b, err := blake2b.New160(nil)
	if err != nil {
		return Address{}, err
	}
	hashBlake2b.Write(pb[:])
	fb := []byte{0}
	if !primary {
		fb = []byte{1}
	}
	return BytesToAddress(append(fb, hashBlake2b.Sum(nil)...))
}

func (a *Address) GetBytes() []byte {
	return a.ByteValue[:]
}

func (a *Address) GetBytesWithPrimary() []byte {
	fb := []byte{0}
	if !a.Primary {
		fb = []byte{1}
	}
	return append(fb, a.ByteValue[:]...)
}

func (a *Address) GetHex() string {
	return hex.EncodeToString(a.ByteValue[:])
}

type PubKey struct {
	ByteValue   []byte  `json:"byte_value"`
	Address     Address `json:"address"`
	MainAddress Address `json:"mainAddress"`
	Primary     bool    `json:"primary"`
}

func (pk *PubKey) Init(b []byte, mainAddress Address) error {
	if len(b) != PubKeyLength() && len(b) != PubKeyLength2() && encryptionConfigInstance.isPaused == 0 && encryptionConfigInstance.isPaused2 == 0 {
		return fmt.Errorf("error Pubkey initialization with wrong length, should be %v, %v, got %v", PubKeyLength(), PubKeyLength2(), len(b))
	}
	if len(b) == PubKeyLength() {
		pk.Primary = true
	} else {
		pk.Primary = false
	}
	pk.ByteValue = b[:]
	addr, err := PubKeyToAddress(b[:], pk.Primary)
	if err != nil {
		return err
	}
	pk.Address = addr
	pk.MainAddress = mainAddress
	return nil
}

func (pk PubKey) GetBytes() []byte {
	return pk.ByteValue[:]
}

func (pk PubKey) GetHex() string {
	return hex.EncodeToString(pk.GetBytes())
}

func (pk PubKey) GetMainAddress() Address {
	return pk.MainAddress
}

func (pk PubKey) GetAddress() Address {
	return pk.Address
}

type PrivKey struct {
	ByteValue []byte  `json:"byte_value"`
	Address   Address `json:"address"`
	Primary   bool    `json:"primary"`
}

func (pk *PrivKey) Init(b []byte, address Address) error {

	if len(b) != PrivateKeyLength() && len(b) != PrivateKeyLength2() && encryptionConfigInstance.isPaused == 0 && encryptionConfigInstance.isPaused2 == 0 {
		return fmt.Errorf("error Private key initialization with wrong length, should be %v", pk.GetLength())
	}
	if len(b) == PrivateKeyLength() {
		pk.Primary = true
	} else {
		pk.Primary = false
	}
	pk.ByteValue = b[:]
	pk.Address = address
	return nil
}

func (pk PrivKey) GetBytes() []byte {
	return pk.ByteValue[:]
}

func (pk PrivKey) GetHex() string {
	return hex.EncodeToString(pk.GetBytes())
}

func (pk PrivKey) GetAddress() Address {
	return pk.Address
}

type Signature struct {
	ByteValue []byte  `json:"byte_value"`
	Address   Address `json:"address"`
	Primary   bool    `json:"primary"`
}

func (s *Signature) Init(b []byte, address Address) error {
	var primary bool
	if len(b) == 0 {
		return fmt.Errorf("error Signature initialization with wrong length, should be %v %v", SignatureLength(), len(b))
	}
	if b[0] == 0 {
		primary = true
	} else {
		primary = false
	}
	if primary && len(b) > SignatureLength()+1 {
		return fmt.Errorf("error Signature initialization with wrong length, should be %v %v", SignatureLength(), len(b))
	}
	if !primary && len(b) > SignatureLength2()+1 {
		return fmt.Errorf("error Signature 2 initialization with wrong length, should be %v %v", SignatureLength2(), len(b))
	}
	s.ByteValue = b[:]
	s.Address = address
	s.Primary = primary
	return nil
}

func (s Signature) GetBytes() []byte {
	return s.ByteValue[:]
}

func (s Signature) GetHex() string {
	return hex.EncodeToString(s.GetBytes())
}

func (s Signature) GetAddress() Address {
	return s.Address
}

type Hash [HashLength]byte
type ShortHash [ShortHashLength]byte

func (h *Hash) Set(b []byte) {
	copy(h[:], b[:])
}

func (h Hash) GetBytes() []byte {
	return h[:]
}

func (h Hash) GetHex() string {
	return hex.EncodeToString(h.GetBytes())
}
func (h *ShortHash) Set(b []byte) {
	copy(h[:], b[:])
}
func (h ShortHash) GetBytes() []byte {
	return h[:]
}

func (h ShortHash) GetHex() string {
	return hex.EncodeToString(h.GetBytes())
}

// GetByteInt32 converts an int32 value to a byte slice.
func GetByteInt32(u int32) []byte {
	tmp := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmp, uint32(u))
	return tmp
}

// GetByteInt16 converts an int16 value to a byte slice.
func GetByteInt16(u int16) []byte {
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, uint16(u))
	return tmp
}

// GetByteInt64 converts an int64 value to a byte slice.
func GetByteInt64(u int64) []byte {
	tmp := make([]byte, 8)
	binary.LittleEndian.PutUint64(tmp, uint64(u))
	return tmp
}

// GetByteInt64 converts an int64 value to a byte slice.
func GetByteUInt64(u uint64) []byte {
	tmp := make([]byte, 8)
	binary.LittleEndian.PutUint64(tmp, u)
	return tmp
}

// GetInt64FromByte converts a byte slice to an int64 value.
func GetInt64FromByte(bs []byte) int64 {
	return int64(binary.LittleEndian.Uint64(bs))
}

// GetInt32FromByte converts a byte slice to an int32 value.
func GetInt32FromByte(bs []byte) int32 {
	return int32(binary.LittleEndian.Uint32(bs))
}

// GetInt16FromByte converts a byte slice to an int16 value.
func GetInt16FromByte(bs []byte) int16 {
	return int16(binary.LittleEndian.Uint16(bs))
}

func EmptyHash() Hash {
	tmp := make([]byte, 32)
	h := Hash{}
	(&h).Set(tmp)
	return h
}

func EmptyAddress() Address {
	a := Address{}
	tmp := make([]byte, AddressLength+1)
	err := a.Init(tmp)
	if err != nil {
		return Address{}
	}
	return a
}

func EmptySignature() Signature {
	s := Signature{}
	tmp := make([]byte, SignatureLength()+1)
	s.Init(tmp, EmptyAddress())
	return s
}
